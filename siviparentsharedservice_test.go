package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func siviSharedTestPointer[T any](value T) *T { return &value }

func sharedTextEdits(t *testing.T, original *SIVIParentProjection) []SIVIParentCellEdit {
	t.Helper()
	edits := []SIVIParentCellEdit{}
	for _, column := range []string{"SiteSurveyor", "FieldNumber", "Location", "NtsMapSheet", "UTMZone", "PlotRepresenting", "SiteSeries", "MapUnit", "SiteNotes"} {
		text := "  X  "
		if column == "UTMZone" {
			text = "01"
		} else if column == "SiteNotes" {
			text = "first\r\nsecond\n" + strings.Repeat("m", 100000)
		}
		edit := siviParentEdit(t, original, siviParentSharedSourceBinding(column),
			ProjectMetadataCell{Storage: "text", Text: &text})
		edit.Column = column
		edits = append(edits, edit)
	}
	return edits
}

func sharedNumericEdits(t *testing.T, original *SIVIParentProjection) []SIVIParentCellEdit {
	t.Helper()
	edits := []SIVIParentCellEdit{}
	for _, field := range []struct {
		column string
		value  ProjectMetadataCell
	}{
		{"UTMEasting", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(-123456.78912345)}},
		{"UTMNorthing", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(-7654321.123456789)}},
		{"SlopeGradient", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(-101.125)}},
		{"LocationAccuracy", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-32768")}},
		{"Elevation", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("32767")}},
		{"Aspect", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-360")}},
		{"StandAge", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("32767")}},
		{"StartDate", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-32768")}},
		{"Latitude", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(-49.1234567890123)}},
		{"Longitude", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(123.9876543210987)}},
	} {
		edits = append(edits, siviParentEdit(t, original, field.column, field.value))
	}
	return edits
}

func sharedServiceFixture(t *testing.T) (*SIVIParentSharedService, ProjectState, *SIVIParentProjection, SIVIParentSharedWrite) {
	t.Helper()
	contexts, state, db, original, _ := siviParentWriteFixture(t, true, 3)
	contexts.siviParentReviewEnabled = true
	service, err := NewSIVIParentSharedService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET rowid=901 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	original, err = contexts.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []SIVIParentCellEdit{}
	for _, column := range []string{"AirPhotoNum", "XCoord", "YCoord", "StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss", "VegNotes"} {
		value := ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(3.25)}
		if column == "AirPhotoNum" || column == "VegNotes" {
			value = ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("  Literal  ")}
		}
		edits = append(edits, siviParentEdit(t, original, column, value))
	}
	return service, state, original, SIVIParentSharedWrite{original, edits}
}
func TestSIVIParentSharedTextTypedRawUnicodeIngestion(t *testing.T) {
	_, _, original, _ := sharedServiceFixture(t)
	request := SIVIParentSharedWrite{original, sharedTextEdits(t, original)}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"  X  "`, strconv.Quote(original.ContextID)} {
		for _, escaped := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\udfff\ud800"`} {
			bad := strings.Replace(string(wire), marker, escaped, 1)
			if bad == string(wire) {
				t.Fatal("raw ingestion marker missing")
			}
			decoded := request
			err := json.Unmarshal([]byte(bad), &decoded)
			if err == nil {
				t.Fatal("typed SDK DTO repaired malformed raw Unicode", marker, escaped)
			}
			if escaped == `"\ud800"` || escaped == `"\udfff"` {
				kind := "high"
				if escaped == `"\udfff"` {
					kind = "low"
				}
				fragment := "SIVI shared-field transport: profile lifecycle JSON: metadata draft Unicode: unpaired " +
					kind + " UTF-16 surrogate in quality JSON string"
				if !strings.Contains(err.Error(), fragment) {
					t.Fatal("raw ingestion receipt lost its diagnostic contract", err)
				}
			}
			if !reflect.DeepEqual(decoded, request) {
				t.Fatal("failed ingestion partially replaced the request")
			}
		}
	}
	var decoded SIVIParentSharedWrite
	valid := strings.Replace(string(wire), `"  X  "`, `"\ud83d\ude00"`, 1)
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil || *decoded.Edits[0].Value.Text != "\U0001f600" {
		t.Fatal("valid raw surrogate pair rejected or repaired", err)
	}
	if _, err := planSIVIParentShared(context.Background(), original, decoded.Edits); err != nil {
		t.Fatal("accepted raw Unicode did not pass scoped text planning", err)
	}
}

func TestSIVIParentSharedTextPhysicalBoundsAndLiteralBinding(t *testing.T) {
	_, _, original, _ := sharedServiceFixture(t)
	ctx := context.Background()
	expectedBounds := map[string]int{"SiteSurveyor": 30, "FieldNumber": 50, "Location": 255,
		"NtsMapSheet": 8, "UTMZone": 2, "PlotRepresenting": 255, "SiteSeries": 5, "MapUnit": 15, "SiteNotes": 0}
	if !reflect.DeepEqual(siviParentSharedTextBounds, expectedBounds) {
		t.Fatal("source physical bounds changed")
	}
	for _, edit := range sharedTextEdits(t, original) {
		maximum := expectedBounds[edit.Column]
		bound := strings.Repeat("\U0001f600", maximum/2) + strings.Repeat("x", maximum%2)
		if maximum == 0 {
			bound = "line\r\n" + strings.Repeat("\U0001f600", 100000)
		}
		for _, value := range []ProjectMetadataCell{
			{Storage: "null"}, {Storage: "text", Text: &bound},
			{Storage: "text", Text: siviSharedTestPointer("01")},
		} {
			edit.Value = value
			assignments, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{edit})
			if err != nil {
				t.Fatal("valid text/null rejected", edit.Column, err)
			}
			if len(assignments) != 0 && assignments[0].Column != edit.Column {
				t.Fatal("source alias leaked into physical assignment", assignments)
			}
		}
		bad := []ProjectMetadataCell{
			{Storage: "text", Text: siviSharedTestPointer("")},
			{Storage: "text", Text: siviSharedTestPointer(string([]byte{0xff}))},
			{Storage: "integer", Integer: siviSharedTestPointer("1")},
			{Storage: "real", Real: siviSharedTestPointer(1.0)},
		}
		if maximum > 0 {
			bad = append(bad, ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer(bound + "x")})
		}
		for _, value := range bad {
			edit.Value = value
			if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{edit}); err == nil {
				t.Fatal("text validator fallthrough accepted changed invalid storage", edit.Column, value)
			}
		}
	}
	notes := sharedTextEdits(t, original)[8]
	for _, column := range []string{"siteNotes", "SITENOTES", "sitesurveyor"} {
		notes.Column = column
		if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{notes}); err == nil {
			t.Fatal("blanket recasing/source alias accepted as public column", column)
		}
	}
	notes = sharedTextEdits(t, original)[8]
	if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{notes, notes}); err == nil {
		t.Fatal("repeated canonical Notes target accepted")
	}
	for _, binding := range original.Bindings {
		if binding.Binding == "siteNotes" {
			original.EnvColumns[binding.Column].Name = "siteNotes"
		}
	}
	if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{notes}); err == nil {
		t.Fatal("guessed physical Notes casing accepted")
	}
}

func TestSIVIParentSharedThirtyTwoAtomicAuditAndRestoration(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, original, request := sharedServiceFixture(t)
			request.Edits = append(request.Edits, sharedSoilEdits(t, original)...)
			request.Edits = append(request.Edits, sharedTextEdits(t, original)...)
			request.Edits = append(request.Edits, sharedNumericEdits(t, original)...)
			request.Edits = append(request.Edits, siviParentEdit(t, original, "Date",
				ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("9999-12-31 23:59:59.999999999")}))
			ctx := context.Background()
			prior, err := service.contexts.ListAuditEntries(ctx, state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Save(ctx, state.ContextID, "108050", request)
			if err != nil || result.ChangedCells != 32 || result.HistoryID == "" {
				t.Fatal("thirty-two-field transaction failed", result, err)
			}
			fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range request.Edits {
				if !reflect.DeepEqual(siviParentCell(t, fresh, siviParentSharedSourceBinding(edit.Column)), edit.Value) {
					t.Fatal("literal physical storage changed", edit.Column)
				}
			}
			audits, err := service.contexts.ListAuditEntries(ctx, state.ContextID, "108050")
			if err != nil || len(audits) != len(prior)+32 {
				t.Fatal("thirty-two audits mismatch", err)
			}
			notesAudit := false
			for _, audit := range audits {
				if audit.EditField == "siteNotes" {
					t.Fatal("source binding leaked into physical audit identity")
				}
				notesAudit = notesAudit || audit.EditField == "SiteNotes"
			}
			if !notesAudit {
				t.Fatal("physical SiteNotes audit missing")
			}
			restored, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action)
			if err != nil || restored.RestoredRows != 32 {
				t.Fatal("typed thirty-two-field restoration failed", restored, err)
			}
			fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(original, fresh) {
				t.Fatal("restoration lost raw original storage", err)
			}
			if action == AuditRestorePrune {
				audits, err = service.contexts.ListAuditEntries(ctx, state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(prior, audits) {
					t.Fatal("typed prune did not restore original audits", err)
				}
			}
		})
	}
}

func TestSIVIParentSharedTextHistoricalOmissionsRollbackAndRetry(t *testing.T) {
	service, state, _, _ := sharedServiceFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Env SET SiteSurveyor=?,FieldNumber='',Location=X'00ff',MapUnit=777,SiteNotes=''
			WHERE PlotNumber='108050'`, strings.Repeat("x", 40)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	original, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []SIVIParentCellEdit{
		siviParentEdit(t, original, "HumusThickness", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(2.5)}),
		siviParentEdit(t, original, "UTMZone", ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("01")}),
	}
	notes := siviParentEdit(t, original, "siteNotes", ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("line\r\n  q'X  ")})
	notes.Column = "SiteNotes"
	edits = append(edits, notes)
	for _, column := range []string{"SiteSurveyor", "FieldNumber", "Location", "MapUnit"} {
		edits = append(edits, siviParentEdit(t, original, column, siviParentCell(t, original, column)))
	}
	if _, err := db.Exec(`CREATE TRIGGER shared_text_abort BEFORE INSERT ON Sample_Audit
			WHEN NEW.EditField='SiteNotes' BEGIN SELECT RAISE(ABORT,'text audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	request := SIVIParentSharedWrite{original, edits}
	if _, err := service.Save(ctx, state.ContextID, "108050", request); err == nil {
		t.Fatal("mixed Admin/text failed audit committed")
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`DROP TRIGGER shared_text_abort`); err != nil {
		t.Fatal(err)
	}
	wrong := request
	wrong.Edits = append([]SIVIParentCellEdit(nil), edits...)
	wrong.Edits[2].RowID = original.Rows[0].Admin.RowID
	if _, err := service.Save(ctx, state.ContextID, "108050", wrong); err == nil {
		t.Fatal("Notes accepted Admin physical row")
	}
	result, err := service.Save(ctx, state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 3 {
		t.Fatal("retry normalized omitted historical storage", result, err)
	}
	if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("historical mixed originals not restored", err)
	}
}
func TestSIVIParentSharedGateAndStrictTransport(t *testing.T) {
	service, state, _, request := sharedServiceFixture(t)
	for _, value := range []string{"", "TRUE", "1", " true "} {
		if _, err := NewSIVIParentSharedService(service.contexts, func(string) (string, bool) { return value, true }); err == nil {
			t.Fatal("coerced shared editor gate", value)
		}
	}
	disabled, err := NewSIVIParentSharedService(service.contexts, func(name string) (string, bool) {
		if name != siviParentSharedFeatureEnvironment && name != siviParentSharedReferenceEnvironment {
			t.Fatal("wrong gate", name)
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := disabled.Save(context.Background(), state.ContextID, "108050", request); err == nil {
		t.Fatal("default enabled write")
	}
	service.contexts.siviParentReviewEnabled = false
	if _, err := service.GetOriginal(context.Background(), state.ContextID, "108050"); err == nil {
		t.Fatal("bypassed parent review")
	}
	assertProfileSUFiles(t, service.contexts, before)
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SIVIParentSharedWrite
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("transport lost raw originals", err)
	}
	for _, bad := range []string{
		string(wire) + "{}",
		strings.Replace(string(wire), `"edits":`, `"actions":[],"edits":`, 1),
		strings.Replace(string(wire), `"  Literal  "`, `"\ud800"`, 1),
		strings.Replace(string(wire), `"  Literal  "`, "\""+string([]byte{0xff})+"\"", 1),
		`{"original":null,"edits":[]}`, `{"edits":[]}`, `{"original":{},"edits":null}`,
	} {
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil {
			t.Fatal("unsafe transport decoded", bad[:min(len(bad), 80)])
		}
	}
}

func TestSIVIParentSharedOwnedWriteAuditRestoreAndReplay(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, original, request := sharedServiceFixture(t)
			ctx := context.Background()
			priorAudits, err := service.contexts.ListAuditEntries(ctx, state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Save(ctx, state.ContextID, "108050", request)
			if err != nil || result.ChangedCells != 8 || result.HistoryID == "" {
				t.Fatal("eight-field transaction failed", result, err)
			}
			fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range request.Edits {
				if !reflect.DeepEqual(siviParentCell(t, fresh, edit.Column), edit.Value) {
					t.Fatal("literal storage lost", edit.Column)
				}
			}
			audits, err := service.contexts.ListAuditEntries(ctx, state.ContextID, "108050")
			if err != nil || len(audits) != len(priorAudits)+8 {
				t.Fatal("wrong audits", len(audits), err)
			}
			if _, err := service.Save(ctx, state.ContextID, "108050", request); err == nil {
				t.Fatal("stale Save replayed")
			}
			restored, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action)
			if err != nil || restored.RestoredRows != 8 || restored.CleanedVegRows != 0 {
				t.Fatal("typed restoration failed", restored, err)
			}
			fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(original, fresh) {
				t.Fatal("raw originals not restored", err)
			}
			if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action); err == nil {
				t.Fatal("restoration replayed")
			}
		})
	}
}

func TestSIVIParentSharedRejectsUnsafeTargetsAndPreservesHistoricalNoops(t *testing.T) {
	service, state, original, request := sharedServiceFixture(t)
	ctx := context.Background()
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for _, mutate := range []func(*SIVIParentSharedWrite){
		func(r *SIVIParentSharedWrite) { r.Edits[0].Column = "ProjectID" },
		func(r *SIVIParentSharedWrite) { r.Edits[0].Column = "PlotType" },
		func(r *SIVIParentSharedWrite) { r.Edits[0].ContextID = "stale" },
		func(r *SIVIParentSharedWrite) { r.Edits[0].RowID = "999" },
		func(r *SIVIParentSharedWrite) { r.Edits[0].Table = "Sample_Admin" },
		func(r *SIVIParentSharedWrite) { r.Edits = append(r.Edits, r.Edits[0]) },
		func(r *SIVIParentSharedWrite) { r.Edits[0].Value.Text = siviSharedTestPointer(strings.Repeat("x", 21)) },
		func(r *SIVIParentSharedWrite) { r.Edits[1].Value.Real = siviSharedTestPointer(1e39) },
	} {
		copy := request
		copy.Edits = append([]SIVIParentCellEdit{}, request.Edits...)
		mutate(&copy)
		if _, err := service.Save(ctx, state.ContextID, "108050", copy); err == nil {
			t.Fatal("unsafe assignment accepted")
		}

		assertProfileSUFiles(t, service.contexts, before)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Save(cancelled, state.ContextID, "108050", request); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation", err)
	}
	for _, binding := range original.Bindings {
		if binding.Binding == "AirPhotoNum" {
			original.Rows[0].Env.Cells[binding.Column] = ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer(strings.Repeat("x", 40))}
		}
	}
	noop := siviParentEdit(t, original, "AirPhotoNum", siviParentCell(t, original, "AirPhotoNum"))
	assignments, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{noop})
	if err != nil || len(assignments) != 0 {
		t.Fatal("historical invalid noop was validated/assigned", assignments, err)
	}
}

func TestSIVIParentSharedAuditFailureRollsBackAndRetryRechecksOwnership(t *testing.T) {
	service, state, original, request := sharedServiceFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER shared_audit_abort BEFORE INSERT ON Sample_Audit
		BEGIN SELECT RAISE(ABORT,'shared audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil {
		t.Fatal("audit failure committed")
	}
	assertProfileSUFiles(t, service.contexts, before)
	fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("failed Save changed data", err)
	}
	if _, err := db.Exec(`DROP TRIGGER shared_audit_abort`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(context.Background(), "stale", "108050", request); err == nil {
		t.Fatal("stale context accepted")
	}
	assertProfileSUFiles(t, service.contexts, before)
	result, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 8 {
		t.Fatal("confirmed rollback could not retry", result, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET SiteSurveyor='concurrent owner' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Restore(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err == nil {
		t.Fatal("restoration overwrote unrelated concurrent changes")
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func sharedSoilEdits(t *testing.T, original *SIVIParentProjection) []SIVIParentCellEdit {
	t.Helper()
	edits := []SIVIParentCellEdit{siviParentEdit(t, original, "HumusThickness",
		ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(2.5)})}
	for i, column := range []string{"SeepageDepth", "RootingDepth", "RootRestrictingDepth"} {
		value := []string{"-32768", "32767", "0"}[i]
		edits = append(edits, siviParentEdit(t, original, column,
			ProjectMetadataCell{Storage: "integer", Integer: &value}))
	}
	return edits
}

func TestSIVIParentSharedSoilMixedTablesAndTypedRestoration(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, original, request := sharedServiceFixture(t)
			request.Edits = append(request.Edits, sharedSoilEdits(t, original)...)
			wire, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var decoded SIVIParentSharedWrite
			if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
				t.Fatal("mixed raw transport failed", err)
			}
			before, err := service.contexts.ListAuditEntries(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Save(context.Background(), state.ContextID, "108050", decoded)
			if err != nil || result.ChangedCells != 12 || result.HistoryID == "" {
				t.Fatal("mixed twelve-field Save failed", result, err)
			}
			fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range decoded.Edits {
				if !reflect.DeepEqual(siviParentCell(t, fresh, edit.Column), edit.Value) {
					t.Fatal("mixed physical cell mismatch", edit.Column)
				}
			}
			audits, err := service.contexts.ListAuditEntries(context.Background(), state.ContextID, "108050")
			if err != nil || len(audits) != len(before)+12 {
				t.Fatal("mixed audits mismatch", err)
			}
			foundAdmin := false
			for _, audit := range audits {
				if audit.EditField == "HumusThickness" && audit.Table == "_Admin" {
					foundAdmin = true
				}
			}
			if !foundAdmin {
				t.Fatal("HumusThickness audit targeted Env")
			}
			restored, err := service.Restore(context.Background(), state.ContextID, "108050", result.HistoryID, action)
			if err != nil || restored.RestoredRows != 12 || restored.CleanedVegRows != 0 {
				t.Fatal("mixed typed restoration failed", restored, err)
			}
			fresh, err = service.GetOriginal(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(fresh, original) {
				t.Fatal("mixed restoration changed original storage", err)
			}
			if action == AuditRestorePrune {
				audits, err = service.contexts.ListAuditEntries(context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(audits, before) {
					t.Fatal("prune did not restore original audit rows", err)
				}
			}
		})
	}
}

func TestSIVIParentSharedSoilCanonicalDomainsAndCrossTableCollisions(t *testing.T) {
	service, state, original, _ := sharedServiceFixture(t)
	ctx := context.Background()
	for _, value := range []ProjectMetadataCell{
		{Storage: "real", Real: siviSharedTestPointer(1e39)},
		{Storage: "integer", Integer: siviSharedTestPointer("3")},
		{Storage: "text", Text: siviSharedTestPointer("3")},
	} {
		if _, err := planSIVIParentShared(ctx, original,
			[]SIVIParentCellEdit{siviParentEdit(t, original, "HumusThickness", value)}); err == nil {
			t.Fatal("invalid Admin Single accepted", value)
		}
	}
	for _, column := range []string{"SeepageDepth", "RootingDepth", "RootRestrictingDepth"} {
		for _, value := range []ProjectMetadataCell{
			{Storage: "null"},
			{Storage: "integer", Integer: siviSharedTestPointer("-32768")},
			{Storage: "integer", Integer: siviSharedTestPointer("32767")},
			{Storage: "integer", Integer: siviSharedTestPointer("0")},
		} {
			if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{siviParentEdit(t, original, column, value)}); err != nil {
				t.Fatal("valid integer domain rejected", column, value, err)
			}
		}
		for _, value := range []ProjectMetadataCell{
			{Storage: "integer", Integer: siviSharedTestPointer("-32769")},
			{Storage: "integer", Integer: siviSharedTestPointer("32768")},
			{Storage: "integer", Integer: siviSharedTestPointer("9007199254740993")},
			{Storage: "integer", Integer: siviSharedTestPointer("01")},
			{Storage: "real", Real: siviSharedTestPointer(3.0)},
			{Storage: "text", Text: siviSharedTestPointer("3")},
		} {
			if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{siviParentEdit(t, original, column, value)}); err == nil {
				t.Fatal("noncanonical/out-of-range integer accepted", column, value)
			}
		}
	}
	for _, column := range []string{"HumusThickness", "RootingDepth"} {
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		for _, swap := range []bool{false, true} {
			edits := sharedSoilEdits(t, original)
			for i := range edits {
				if edits[i].Column != column {
					continue
				}
				if column == "HumusThickness" {
					edits[i].RowID = original.Rows[0].Env.RowID
					if swap {
						edits[i].Table = original.EnvTable
					}
				} else {
					edits[i].RowID = original.Rows[0].Admin.RowID
					if swap {
						edits[i].Table = original.AdminTable
					}
				}
			}
			if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, edits}); err == nil {
				t.Fatal("swapped table/row identity accepted", column)
			}
			assertProfileSUFiles(t, service.contexts, before)
		}
	}
}

func TestSIVIParentSharedDateTransportAndCalendar(t *testing.T) {
	_, _, original, _ := sharedServiceFixture(t)
	ctx := context.Background()
	valid := []string{"0100-01-01 00:00:00", "9999-12-31 23:59:59.999999999",
		"2000-02-29 12:34:56", "2026-03-08 02:30:00.100000000", "2026-11-01 01:30:00.123456789"}
	for size := 1; size <= 9; size++ {
		valid = append(valid, "2024-02-29 00:00:00."+strings.Repeat("0", size))
	}
	for _, value := range valid {
		edit := siviParentEdit(t, original, "Date", ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer(value)})
		request := SIVIParentSharedWrite{original, []SIVIParentCellEdit{edit}}
		wire, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SIVIParentSharedWrite
		if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(request, decoded) {
			t.Fatal("Date raw transport normalized", err)
		}
		for _, escaped := range []string{`"\ud800"`, `"\udfff"`} {
			bad := strings.Replace(string(wire), strconv.Quote(value), escaped, 1)
			if err := json.Unmarshal([]byte(bad), &decoded); err == nil ||
				!strings.Contains(err.Error(), "metadata draft Unicode: unpaired") {
				t.Fatal("Date raw SDK Unicode was repaired before rejection", err)
			}
		}
		assignments, err := planSIVIParentShared(ctx, original, decoded.Edits)
		if err != nil || len(assignments) != 1 || assignments[0].Value != value {
			t.Fatal("Date calendar/fraction normalized or rejected", value, assignments, err)
		}
	}
	for _, value := range []ProjectMetadataCell{
		{Storage: "integer", Integer: siviSharedTestPointer("45292")},
		{Storage: "real", Real: siviSharedTestPointer(45292.125)},
		{Storage: "text", Text: siviSharedTestPointer("")},
		{Storage: "text", Text: siviSharedTestPointer("0099-12-31 23:59:59")},
		{Storage: "text", Text: siviSharedTestPointer("1900-02-29 12:00:00")},
		{Storage: "text", Text: siviSharedTestPointer("2026-01-01")},
		{Storage: "text", Text: siviSharedTestPointer("2026-01-01T00:00:00")},
		{Storage: "text", Text: siviSharedTestPointer("2026-01-01 00:00:00Z")},
		{Storage: "text", Text: siviSharedTestPointer("2026-01-01 00:00:00.1234567890")},
		{Storage: "text", Text: siviSharedTestPointer(" 2026-01-01 00:00:00")},
		{Storage: "text", Text: siviSharedTestPointer("2026-01-01 00:00:00\xed\xa0\x80")},
	} {
		if _, err := planSIVIParentShared(ctx, original,
			[]SIVIParentCellEdit{siviParentEdit(t, original, "Date", value)}); err == nil {
			t.Fatal("malformed/new nontext Date accepted", value)
		}
	}
	if _, err := planSIVIParentShared(ctx, original,
		[]SIVIParentCellEdit{siviParentEdit(t, original, "Date", ProjectMetadataCell{Storage: "null"})}); err != nil {
		t.Fatal("explicit Date NULL rejected", err)
	}
}

func TestSIVIParentSharedDateLegacyCorrectionNullAndAuthoritativeRestore(t *testing.T) {
	for _, historical := range []any{"localized old date", "", "2024-02-29", int64(45292), 45292.125, nil} {
		for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
			t.Run(fmt.Sprintf("%T-%v-%s", historical, historical, action), func(t *testing.T) {
				service, state, _, _ := sharedServiceFixture(t)
				db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				if _, err := db.Exec(`UPDATE Sample_Env SET Date=? WHERE PlotNumber='108050'`, historical); err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				original, err := service.GetOriginal(ctx, state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				noop := siviParentEdit(t, original, "Date", siviParentCell(t, original, "Date"))
				result, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, []SIVIParentCellEdit{noop}})
				if err != nil || result.ChangedCells != 0 || result.HistoryID != "" {
					t.Fatal("historical Date noop parsed/assigned", result, err)
				}
				date := siviParentEdit(t, original, "Date",
					ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("2026-03-08 02:30:00.100000000")})
				year := siviParentEdit(t, original, "StartDate",
					ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-32768")})
				result, err = service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, []SIVIParentCellEdit{date, year}})
				if err != nil || result.ChangedCells != 2 {
					t.Fatal("mixed Date/year correction failed", result, err)
				}
				fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(siviParentCell(t, fresh, "Date"), date.Value) {
					t.Fatal("Date wallclock/fraction lost", err)
				}
				if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action); err != nil {
					t.Fatal("authoritative historical Date restoration rejected", err)
				}
				fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(original, fresh) {
					t.Fatal("historical Date storage normalized", err)
				}
				if historical != nil {
					result, err = service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{fresh,
						[]SIVIParentCellEdit{siviParentEdit(t, fresh, "Date", ProjectMetadataCell{Storage: "null"})}})
					if err != nil || result.ChangedCells != 1 {
						t.Fatal("nullable Date Save failed", result, err)
					}
					cleared, err := service.GetOriginal(ctx, state.ContextID, "108050")
					if err != nil || siviParentCell(t, cleared, "Date").Storage != "null" {
						t.Fatal("explicit Date NULL did not persist", err)
					}
					if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action); err != nil {
						t.Fatal("Date NULL history restoration failed", err)
					}
					fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
					if err != nil || !reflect.DeepEqual(original, fresh) {
						t.Fatal("Date NULL restoration lost raw original", err)
					}
				}
			})
		}
	}
}

func TestSIVIParentSharedDateOwnershipCollisionAtomicRollbackAndRetry(t *testing.T) {
	service, state, original, _ := sharedServiceFixture(t)
	ctx := context.Background()
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	date := siviParentEdit(t, original, "Date",
		ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("0100-01-01 00:00:00.000000000")})
	year := siviParentEdit(t, original, "StartDate",
		ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-32768")})
	request := SIVIParentSharedWrite{original, []SIVIParentCellEdit{year, date}}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for _, bad := range []SIVIParentCellEdit{
		siviParentEdit(t, original, "Date", ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("invalid")}),
		{ContextID: date.ContextID, Table: original.AdminTable, RowID: date.RowID, Column: "Date", Expected: date.Expected, Value: date.Value},
		{ContextID: date.ContextID, Table: date.Table, RowID: original.Rows[0].Admin.RowID, Column: "Date", Expected: date.Expected, Value: date.Value},
	} {
		if _, err := service.Save(ctx, state.ContextID, "108050",
			SIVIParentSharedWrite{original, []SIVIParentCellEdit{year, bad}}); err == nil {
			t.Fatal("mixed Date/year malformed or foreign target accepted")
		}
		assertProfileSUFiles(t, service.contexts, before)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Save(cancelled, state.ContextID, "108050", request); !errors.Is(err, context.Canceled) {
		t.Fatal("Date Save ignored cancellation", err)
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`CREATE TRIGGER shared_date_abort BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='Date' BEGIN SELECT RAISE(ABORT,'Date audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", request); err == nil {
		t.Fatal("Date audit failure committed mixed Admin/Env writes")
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`DROP TRIGGER shared_date_abort;
		UPDATE Sample_Env SET Date='collision' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", request); err == nil {
		t.Fatal("Date original collision accepted")
	}
	assertProfileSUFiles(t, service.contexts, before)
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	request = SIVIParentSharedWrite{fresh, []SIVIParentCellEdit{
		siviParentEdit(t, fresh, "StartDate", year.Value), siviParentEdit(t, fresh, "Date", date.Value),
	}}
	result, err := service.Save(ctx, state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 2 {
		t.Fatal("Date retry failed", result, err)
	}
	if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal("Date retry history restoration failed", err)
	}
	restored, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(fresh, restored) {
		t.Fatal("Date collision baseline was not restored", err)
	}
}

func TestSIVIParentSharedNumericDomainsAndOwnership(t *testing.T) {
	service, state, original, _ := sharedServiceFixture(t)
	ctx := context.Background()
	for _, edit := range sharedNumericEdits(t, original) {
		kind := siviParentSharedNumberKinds[edit.Column]
		valid := []ProjectMetadataCell{{Storage: "null"}}
		bad := []ProjectMetadataCell{{Storage: "text", Text: siviSharedTestPointer("3")}}
		if kind == "integer" {
			for _, value := range []string{"-32768", "32767", "0"} {
				valid = append(valid, ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer(value)})
			}
			for _, value := range []string{"-32769", "32768", "01", "9007199254740993"} {
				bad = append(bad, ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer(value)})
			}
			bad = append(bad, ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(3.0)})
		} else {
			limit := float64(math.MaxFloat32)
			if kind == "latitude" {
				limit = 90
			} else if kind == "longitude" {
				limit = 180
			}
			for _, value := range []float64{-limit, limit, 0, -1.1234567890123} {
				valid = append(valid, ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(value)})
			}
			for _, value := range []float64{math.Nextafter(limit, math.Inf(1)), -math.Nextafter(limit, math.Inf(1)), math.NaN(), math.Inf(1)} {
				bad = append(bad, ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(value)})
			}
			bad = append(bad, ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("3")})
		}
		for _, value := range valid {
			edit.Value = value
			assignments, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{edit})
			if err != nil {
				t.Fatal("valid physical/safety numeric domain rejected", edit.Column, value, err)
			}
			if len(assignments) == 1 && !reflect.DeepEqual(assignments[0].After, value) {
				t.Fatal("numeric precision/storage normalized", edit.Column)
			}
		}
		for _, value := range bad {
			edit.Value = value
			if _, err := planSIVIParentShared(ctx, original, []SIVIParentCellEdit{edit}); err == nil {
				t.Fatal("invalid numeric domain accepted", edit.Column, value)
			}
		}
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	edits := sharedNumericEdits(t, original)
	for _, column := range []string{"StartDate", "Latitude"} {
		for _, change := range []string{"row", "table", "context", "duplicate"} {
			bad := append([]SIVIParentCellEdit(nil), edits...)
			for i := range bad {
				if bad[i].Column != column {
					continue
				}
				switch change {
				case "row":
					bad[i].RowID = original.Rows[0].Admin.RowID
					if column == "StartDate" {
						bad[i].RowID = original.Rows[0].Env.RowID
					}
				case "table":
					bad[i].Table = original.AdminTable
					if column == "StartDate" {
						bad[i].Table = original.EnvTable
					}
				case "context":
					bad[i].ContextID = "foreign"
				case "duplicate":
					bad = append(bad, bad[i])
				}
				break
			}
			if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, bad}); err == nil {
				t.Fatal("numeric identity/collision accepted", column, change)
			}
			assertProfileSUFiles(t, service.contexts, before)
		}
	}
}

func TestSIVIParentSharedNumericHistoricalRollbackRetryAndCancellation(t *testing.T) {
	service, state, _, _ := sharedServiceFixture(t)
	ctx := context.Background()
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Env SET Latitude=400,Longitude=-400,SlopeGradient=1e39,
		UTMEasting='historic invalid',Aspect=3.25,LocationAccuracy=40000,StandAge=X'00ff'
		WHERE PlotNumber='108050'; UPDATE Sample_Admin SET StartDate=40000 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := readSQLiteStorageRows(ctx, db, "main", "Sample_Metadata", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	noops := []SIVIParentCellEdit{}
	for _, edit := range sharedNumericEdits(t, original) {
		edit.Value = cloneSiteUnitCell(edit.Expected)
		noops = append(noops, edit)
	}
	result, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, noops})
	if err != nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("historical numeric noops assigned/audited", result, err)
	}
	edits := []SIVIParentCellEdit{
		siviParentEdit(t, original, "StartDate", ProjectMetadataCell{Storage: "null"}),
		siviParentEdit(t, original, "Latitude", ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(-90.0)}),
	}
	for _, noop := range noops {
		if noop.Column != "StartDate" && noop.Column != "Latitude" {
			edits = append(edits, noop)
		}
	}
	request := SIVIParentSharedWrite{original, edits}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(cancelled, state.ContextID, "108050", request); !errors.Is(err, context.Canceled) {
		t.Fatal("numeric Save ignored cancellation", err)
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`CREATE TRIGGER shared_numeric_abort BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='Latitude' BEGIN SELECT RAISE(ABORT,'geographic audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", request); err == nil {
		t.Fatal("mixed Admin/geographic audit failure committed")
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`DROP TRIGGER shared_numeric_abort`); err != nil {
		t.Fatal(err)
	}
	result, err = service.Save(ctx, state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 2 {
		t.Fatal("numeric retry normalized historical omissions", result, err)
	}
	currentMetadata, err := readSQLiteStorageRows(ctx, db, "main", "Sample_Metadata", "", nil, "")
	if err != nil || !reflect.DeepEqual(metadata, currentMetadata) {
		t.Fatal("Admin year changed metadata", err)
	}
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"SV_StandAgeEstMeas", "SV_StandHeightEstMeas"} {
		if !reflect.DeepEqual(siviParentCell(t, fresh, column), siviParentCell(t, original, column)) {
			t.Fatal("numeric Save changed source options", column)
		}
	}
	if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal("numeric typed restoration failed", err)
	}
	fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("numeric typed restoration normalized originals", err)
	}
}

func TestSIVIParentSharedSoilHistoricalOmissionsRollbackRetryAndDrift(t *testing.T) {
	service, state, _, _ := sharedServiceFixture(t)
	ctx := context.Background()
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Admin SET HumusThickness=1e39 WHERE Plot='108050';
		UPDATE Sample_Env SET SeepageDepth=40000,RootingDepth=2.5,RootRestrictingDepth='historical invalid' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	noops := []SIVIParentCellEdit{}
	for _, column := range []string{"HumusThickness", "SeepageDepth", "RootingDepth", "RootRestrictingDepth"} {
		noops = append(noops, siviParentEdit(t, original, column, siviParentCell(t, original, column)))
	}
	result, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, noops})
	if err != nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("historical noops assigned/audited", result, err)
	}
	mixed := []SIVIParentCellEdit{
		siviParentEdit(t, original, "HumusThickness", ProjectMetadataCell{Storage: "null"}),
		siviParentEdit(t, original, "RootingDepth", ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("5")}),
		noops[1], noops[3],
	}
	if _, err := db.Exec(`CREATE TRIGGER soil_second_audit_abort BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='RootingDepth' BEGIN SELECT RAISE(ABORT,'second table audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, mixed}); err == nil {
		t.Fatal("mixed audit failure committed")
	}
	assertProfileSUFiles(t, service.contexts, before)
	if _, err := db.Exec(`DROP TRIGGER soil_second_audit_abort`); err != nil {
		t.Fatal(err)
	}
	result, err = service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, mixed})
	if err != nil || result.ChangedCells != 2 {
		t.Fatal("mixed retry failed or assigned historical noops", result, err)
	}
	if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal("historical raw restoration failed", err)
	}
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("historical storage was normalized", err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET HumusThickness=3 WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, mixed}); err == nil {
		t.Fatal("concurrent Admin change overwritten")
	}
	assertProfileSUFiles(t, service.contexts, before)
}
