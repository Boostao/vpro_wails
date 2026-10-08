package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviSharedTestPointer[T any](value T) *T { return &value }

func sharedServiceFixture(t *testing.T) (*SIVIParentSharedService, ProjectState, *SIVIParentProjection, SIVIParentSharedWrite) {
	t.Helper()
	contexts, state, _, original, _ := siviParentWriteFixture(t, true, 3)
	contexts.siviParentReviewEnabled = true
	service, err := NewSIVIParentSharedService(contexts, func(string) (string, bool) { return "true", true })
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

func TestSIVIParentSharedGateAndStrictTransport(t *testing.T) {
	service, state, _, request := sharedServiceFixture(t)
	for _, value := range []string{"", "TRUE", "1", " true "} {
		if _, err := NewSIVIParentSharedService(service.contexts, func(string) (string, bool) { return value, true }); err == nil {
			t.Fatal("coerced shared editor gate", value)
		}
	}
	disabled, err := NewSIVIParentSharedService(service.contexts, func(name string) (string, bool) {
		if name != siviParentSharedFeatureEnvironment {
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
