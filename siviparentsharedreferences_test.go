package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sharedReferenceFixture(t *testing.T) (*SIVIParentSharedService, ProjectState, *SIVIParentProjection) {
	t.Helper()
	service, state, original, _ := sharedServiceFixture(t)
	dir := t.TempDir()
	region, err := NewRegionCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	site, err := NewSiteCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := NewParentCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	geology, err := NewGeologyCodeService(dir)
	if err != nil {
		t.Fatal(err)
	}
	bec, err := NewBECService(dir)
	if err != nil {
		t.Fatal(err)
	}
	service.references = siviParentSharedReferenceReaders{region: region, site: site, parent: parent, geology: geology, bec: bec}
	t.Cleanup(func() {
		for _, close := range []func() error{region.Close, site.Close, parent.Close, geology.Close, bec.Close} {
			if err := close(); err != nil {
				t.Error(err)
			}
		}
	})
	return service, state, original
}

func sharedReferenceEdits(t *testing.T, service *SIVIParentSharedService, state ProjectState, original *SIVIParentProjection) []SIVIParentCellEdit {
	t.Helper()
	references, err := service.GetReferences(context.Background(), state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
	if err != nil {
		t.Fatal(err)
	}
	edits := []SIVIParentCellEdit{}
	for i, field := range siviParentSharedReferenceFields {
		value := strings.Repeat("X", field.maximum)
		if field.required {
			found := false
			for _, choice := range references.Fields[i].Choices {
				if choice.Selectable && choice.Code != nil {
					value = *choice.Code
					found = true
					break
				}
			}
			if !found {
				t.Fatal("required catalogue unavailable", field.column, references.Fields[i])
			}
		}
		edits = append(edits, siviParentEdit(t, original, field.column, ProjectMetadataCell{Storage: "text", Text: &value}))
	}
	return edits
}

func TestSIVIParentSharedReferenceGatesAndPhysicalPolicies(t *testing.T) {
	service, state, original := sharedReferenceFixture(t)
	if len(siviParentSharedReferenceFields) != 28 || len(siviParentSharedOwners) != 32 {
		t.Fatal("scope count changed")
	}
	ctx := context.Background()
	edits := sharedReferenceEdits(t, service, state, original)
	service.referencesEnabled = false
	if _, err := service.GetReferences(ctx, state.ContextID, "108050", ProjectMetadataCell{Storage: "null"}); err == nil {
		t.Fatal("reference gate bypassed")
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, edits}); err == nil {
		t.Fatal("disabled reference write accepted")
	}
	assertProfileSUFiles(t, service.contexts, before)
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("reference gate changed accepted originals", err)
	}
	for _, field := range siviParentSharedReferenceFields {
		bound := strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("x", field.maximum%2)
		for _, value := range []ProjectMetadataCell{{Storage: "null"}, {Storage: "text", Text: &bound}} {
			edit := siviParentEdit(t, original, field.column, value)
			if _, err := planSIVIParentSharedScope(ctx, original, []SIVIParentCellEdit{edit}, true); err != nil {
				t.Fatal("valid reference text/null rejected", field.column, err)
			}
		}
		for _, value := range []ProjectMetadataCell{
			{Storage: "text", Text: siviSharedTestPointer(bound + "x")},
			{Storage: "text", Text: siviSharedTestPointer("")},
			{Storage: "text", Text: siviSharedTestPointer(string([]byte{0xff}))},
			{Storage: "integer", Integer: siviSharedTestPointer("1")},
		} {
			if _, err := planSIVIParentSharedScope(ctx, original, []SIVIParentCellEdit{siviParentEdit(t, original, field.column, value)}, true); err == nil {
				t.Fatal("invalid physical reference accepted", field.column)
			}
		}
	}
	if _, err := NewSIVIParentSharedService(service.contexts, func(key string) (string, bool) {
		if key == siviParentSharedReferenceEnvironment {
			return "invalid", true
		}
		return "true", true
	}); err == nil {
		t.Fatal("invalid runtime gate silently repaired")
	}
}

func TestSIVIParentSharedSixRawReferenceDefinitionsOwnershipAndRetry(t *testing.T) {
	service, state, _ := sharedReferenceFixture(t)
	ctx := context.Background()
	zone := ProjectMetadataCell{Storage: "null"}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	references, err := service.GetReferences(ctx, state.ContextID, "108050", zone)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"MoistureRegime": 10, "NutrientRegime": 7, "MesoSlopePosition": 9, "SurfaceShape": 4, "StructuralStage": 21, "SuccessionalStatus": 10}
	for _, field := range references.Fields {
		if count, raw := counts[field.Column]; raw {
			if !field.Available || len(field.Definitions.Rows) != count || !reflect.DeepEqual(field.Definitions.Columns, siviParentReferenceColumns) ||
				!strings.Contains(field.Source, "not frozen DAO") {
				t.Fatal("six-list raw/provenance boundary changed", field)
			}
			if field.Definitions.Rows[0].Cells[3].Storage != "null" {
				t.Fatal("NULL Item was collapsed", field.Column)
			}
		}
		if field.Column == "SubZone" && (!field.Available || len(field.Choices) != 0) {
			t.Fatal("NULL Zone meant all SubZones")
		}
	}
	assertProfileSUFiles(t, service.contexts, before)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='MoistureRegime';
		INSERT INTO USysTableOfLists(rowid,ListName,ListFilter,ItemOrder,Item,ItemDescription,Validate,Flag) VALUES
		(900001,'MoistureRegime',NULL,NULL,NULL,'Null',1,-1),
		(900002,'MoistureRegime','',2,'','',NULL,0),
		(900003,'MoistureRegime',NULL,3,'ZZZ','first',1,0),
		(900004,'MoistureRegime',NULL,3,'ZZZ','second',1,0),
		(900005,'MoistureRegime',NULL,4,'LONG','too long',0,0);`); err != nil {
		t.Fatal(err)
	}
	references, err = service.GetReferences(ctx, state.ContextID, "108050", zone)
	if err != nil {
		t.Fatal(err)
	}
	var raw SIVIParentSharedReference
	for _, field := range references.Fields {
		if field.Column == "MoistureRegime" {
			raw = field
		}
	}
	if !raw.Available || len(raw.Definitions.Rows) != 5 || len(raw.Choices) != 5 ||
		raw.Definitions.Rows[0].RowID != "900001" || *raw.Definitions.Rows[0].Cells[7].Integer != "1" ||
		raw.Definitions.Rows[0].Cells[1].Storage != "null" || *raw.Definitions.Rows[1].Cells[1].Text != "" ||
		raw.Choices[0].Selectable || raw.Choices[1].Selectable || !raw.Choices[2].Selectable || !raw.Choices[3].Selectable || raw.Choices[4].Selectable {
		t.Fatal("raw NULL/empty/duplicates/BOOLEAN/validity repaired", raw)
	}
	if _, err := db.Exec(`ALTER TABLE USysTableOfLists ADD COLUMN Extra INTEGER GENERATED ALWAYS AS (42) VIRTUAL`); err != nil {
		t.Fatal(err)
	}
	references, err = service.GetReferences(ctx, state.ContextID, "108050", zone)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range references.Fields {
		if field.Column == "MoistureRegime" && (field.Available || !strings.Contains(field.Diagnostic, "unshadowed")) {
			t.Fatal("generated schema accepted", field)
		}
	}
	if _, err := db.Exec(`ALTER TABLE USysTableOfLists DROP COLUMN Extra`); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.GetReferences(cancelled, state.ContextID, "108050", zone); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel ignored", err)
	}
	if _, err := service.GetReferences(ctx, "stale", "108050", zone); err == nil {
		t.Fatal("stale reference owner accepted")
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	blocked, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = service.GetReferences(blocked, state.ContextID, "108050", zone)
	stop()
	if _, rollbackErr := conn.ExecContext(ctx, "ROLLBACK"); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("blocked read ignored cancellation", err)
	}
	if _, err := service.GetReferences(ctx, state.ContextID, "108050", zone); err != nil {
		t.Fatal("cancelled reference snapshot broke retry", err)
	}
}

func TestSIVIParentSharedTwentyEightReferencesAtomicMembershipAndAuthoritativeRestore(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, original := sharedReferenceFixture(t)
			ctx := context.Background()
			edits := sharedReferenceEdits(t, service, state, original)
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			required := 0
			for i, field := range siviParentSharedReferenceFields {
				if !field.required {
					continue
				}

				required++
				bad := append([]SIVIParentCellEdit(nil), edits...)
				bad[i].Value = ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("?")}
				if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, bad}); err == nil {
					t.Fatal("nonmember required field accepted", field.column)
				}
				assertProfileSUFiles(t, service.contexts, before)
			}
			if required != 4 {
				t.Fatal("membership policy widened")
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TRIGGER shared_ref_abort BEFORE INSERT ON Sample_Audit
				WHEN NEW.EditField='SuccessionalStatus' BEGIN SELECT RAISE(ABORT,'reference audit failure'); END`); err != nil {
				t.Fatal(err)
			}
			before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, edits}); err == nil {
				t.Fatal("reference audit failure committed")
			}
			assertProfileSUFiles(t, service.contexts, before)
			if _, err := db.Exec(`DROP TRIGGER shared_ref_abort`); err != nil {
				t.Fatal(err)
			}
			result, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, edits})
			if err != nil || result.ChangedCells != 28 {
				t.Fatal("28-field atomic reference Save failed", result, err)
			}
			if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, edits}); err == nil {
				t.Fatal("reference stale collision accepted")
			}
			service.references = siviParentSharedReferenceReaders{}
			service.referencesEnabled = false
			if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, action); err != nil {
				t.Fatal("historical restore required current catalogues/gate", err)
			}
			fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(original, fresh) {
				t.Fatal("reference restore changed historical cells", err)
			}
		})
	}
}

type sharedReferenceParentStub struct {
	rows []ParentCodeChoice
}

func (stub sharedReferenceParentStub) ListChoices(context.Context, string) ([]ParentCodeChoice, error) {
	return stub.rows, nil
}

func TestSIVIParentSharedBorrowedValidityAndEmptyAvailability(t *testing.T) {
	service, state, original := sharedReferenceFixture(t)
	code, list := "Q", "SoilDrainage"
	service.references.parent = sharedReferenceParentStub{[]ParentCodeChoice{
		{RowID: "1", ListName: &list, Code: &code, Selectable: false, Diagnostic: "source definition invalid"},
	}}
	references, err := service.GetReferences(context.Background(), state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range references.Fields {
		if field.Column == list && (!field.Available || field.Choices[0].Selectable ||
			field.Choices[0].Diagnostic != "source definition invalid" || len(field.Definitions.Rows[0].Cells) != 10) {
			t.Fatal("borrowed validity or metadata was overwritten", field)
		}
	}
	edit := siviParentEdit(t, original, list, ProjectMetadataCell{Storage: "text", Text: &code})
	if _, err := service.Save(context.Background(), state.ContextID, "108050", SIVIParentSharedWrite{original, []SIVIParentCellEdit{edit}}); err == nil {
		t.Fatal("unselectable borrowed Item authorized a write")
	}
	service.references.parent = sharedReferenceParentStub{}
	references, err = service.GetReferences(context.Background(), state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range references.Fields {
		if field.Column == list && (field.Available || !strings.Contains(field.Diagnostic, "empty")) {
			t.Fatal("empty borrowed catalogue reported available", field)
		}
	}
}

func TestSIVIParentSharedAllSixtyMixedAtomicAndRestoration(t *testing.T) {
	service, state, original := sharedReferenceFixture(t)
	edits := sharedReferenceEdits(t, service, state, original)
	edits = append(edits, sharedTextEdits(t, original)...)
	edits = append(edits, sharedNumericEdits(t, original)...)
	for _, column := range []string{"AirPhotoNum", "XCoord", "YCoord", "StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss", "VegNotes"} {
		value := ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(3.25)}
		if column == "AirPhotoNum" || column == "VegNotes" {
			value = ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("  Literal  ")}
		}
		edits = append(edits, siviParentEdit(t, original, column, value))
	}
	for _, column := range []string{"HumusThickness", "SeepageDepth", "RootingDepth", "RootRestrictingDepth"} {
		value := ProjectMetadataCell{Storage: "integer", Integer: siviSharedTestPointer("-32768")}
		if column == "HumusThickness" {
			value = ProjectMetadataCell{Storage: "real", Real: siviSharedTestPointer(12.5)}
		}
		edits = append(edits, siviParentEdit(t, original, column, value))
	}
	edits = append(edits, siviParentEdit(t, original, "Date",
		ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("9999-12-31 23:59:59.123456789")}))
	if len(edits) != 60 || original.Rows[0].Env.RowID == original.Rows[0].Admin.RowID {
		t.Fatal("mixed-scope fixture does not cover 60 fields and distinct owners")
	}
	request := SIVIParentSharedWrite{original, edits}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for _, column := range []string{"HumusThickness", "Exposure1"} {
		wrong := append([]SIVIParentCellEdit(nil), edits...)
		for i := range wrong {
			if wrong[i].Column == column {
				wrong[i].RowID = original.Rows[0].Env.RowID
				if column == "Exposure1" {
					wrong[i].RowID = original.Rows[0].Admin.RowID
				}
			}
		}
		if _, err := service.Save(context.Background(), state.ContextID, "108050", SIVIParentSharedWrite{original, wrong}); err == nil {
			t.Fatal("cross-table physical identity accepted", column)
		}
		assertProfileSUFiles(t, service.contexts, before)
	}
	result, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 60 {
		t.Fatal("mixed 60-field transaction failed", result, err)
	}
	if _, err := service.Restore(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("60-field typed restoration changed originals", err)
	}
}

func TestSIVIParentSharedReferenceHistoricalNoOpNullableAndMissingCatalogues(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, _ := sharedReferenceFixture(t)
			db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`UPDATE Sample_Env SET Exposure1=X'00FF',Exposure2='',SoilDrainage='LONG-HISTORICAL',SuccessionalStatus='???' WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			service.references = siviParentSharedReferenceReaders{}
			original, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			unchanged, nullable := []SIVIParentCellEdit{}, []SIVIParentCellEdit{}
			for _, field := range siviParentSharedReferenceFields {
				if field.required {
					edit := siviParentEdit(t, original, field.column, ProjectMetadataCell{Storage: "null"})
					if edit.Expected.Storage != "blob" {
						nullable = append(nullable, edit)
					}
					edit.Value = edit.Expected
					unchanged = append(unchanged, edit)
				}
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			result, err := service.Save(context.Background(), state.ContextID, "108050", SIVIParentSharedWrite{original, unchanged})
			if err != nil || result.ChangedCells != 0 {
				t.Fatal("historical required no-op was revalidated", result, err)
			}
			assertProfileSUFiles(t, service.contexts, before)
			blob := siviParentEdit(t, original, "Exposure1", ProjectMetadataCell{Storage: "null"})
			if _, err := service.Save(context.Background(), state.ContextID, "108050", SIVIParentSharedWrite{original, []SIVIParentCellEdit{blob}}); err == nil {
				t.Fatal("historical BLOB replacement bypassed lossless audit guard")
			}
			assertProfileSUFiles(t, service.contexts, before)
			result, err = service.Save(context.Background(), state.ContextID, "108050", SIVIParentSharedWrite{original, nullable})
			if err != nil || result.ChangedCells != 3 {
				t.Fatal("explicit required NULL depended on catalogue availability", result, err)
			}

			if _, err := service.Restore(context.Background(), state.ContextID, "108050", result.HistoryID, action); err != nil {
				t.Fatal(err)
			}
			fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(original, fresh) {
				t.Fatal("historical nontext/empty/nonmember restoration was normalized", err)
			}
		})
	}
}

func TestSIVIParentSharedIndependentRuntimeGates(t *testing.T) {
	service, state, _ := sharedReferenceFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for _, review := range []bool{false, true} {
		for _, shared := range []bool{false, true} {
			for _, reference := range []bool{false, true} {
				service.contexts.siviParentReviewEnabled = review
				gated, err := NewSIVIParentSharedService(service.contexts, func(name string) (string, bool) {
					enabled := shared
					if name == siviParentSharedReferenceEnvironment {
						enabled = reference
					}
					return fmt.Sprint(enabled), true
				})
				if err != nil {
					t.Fatal(err)
				}
				_, err = gated.GetOriginal(context.Background(), state.ContextID, "108050")
				if (err == nil) != (review && shared) {
					t.Fatal("reference gate changed the accepted original gate", review, shared, reference, err)
				}
				_, err = gated.GetReferences(context.Background(), state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
				if (err == nil) != (review && shared && reference) {
					t.Fatal("reference gate was not independent", review, shared, reference, err)
				}
			}
		}
	}
	service.contexts.siviParentReviewEnabled = true
	omitted, err := NewSIVIParentSharedService(service.contexts, func(name string) (string, bool) {
		return "true", name == siviParentSharedFeatureEnvironment
	})
	if err != nil || omitted.referencesEnabled || !omitted.enabled {
		t.Fatal("omitted reference gate did not preserve default 32", err)
	}
	assertProfileSUFiles(t, service.contexts, before)
}

func TestSIVIParentSharedReferenceRuntimeGatePreservesAcceptedThirtyTwo(t *testing.T) {
	service, state, original, request := sharedServiceFixture(t)
	ctx := context.Background()
	for _, enabled := range []bool{false, true} {
		service.referencesEnabled = enabled
		fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
		if err != nil || !reflect.DeepEqual(original, fresh) {
			t.Fatal("reference runtime gate changed full physical originals", enabled, err)
		}
		for _, field := range siviParentSharedReferenceFields {
			_ = siviParentEdit(t, fresh, field.column, ProjectMetadataCell{Storage: "null"})
		}
		references, err := service.GetReferences(ctx, state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
		if enabled {
			if err != nil || len(references.Fields) != 28 {
				t.Fatal("enabled runtime reference read failed", err)
			}
		} else {
			if err == nil || references != nil || !strings.Contains(err.Error(), "independently disabled") {
				t.Fatal("disabled reference runtime lacked explicit denial", err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			edit := siviParentEdit(t, original, "RealmClass", ProjectMetadataCell{Storage: "text", Text: siviSharedTestPointer("free")})
			if _, err := service.Save(ctx, state.ContextID, "108050", SIVIParentSharedWrite{original, []SIVIParentCellEdit{edit}}); err == nil {
				t.Fatal("reference runtime denial fell back to a broader writer")
			}
			assertProfileSUFiles(t, service.contexts, before)
		}
		result, err := service.Save(ctx, state.ContextID, "108050", request)
		if err != nil || result.ChangedCells != len(request.Edits) {
			t.Fatal("reference runtime gate disabled accepted 32-field Save", enabled, result, err)
		}
		if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil {
			t.Fatal(err)
		}
		fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
		if err != nil || !reflect.DeepEqual(original, fresh) {
			t.Fatal("reference runtime gate changed accepted originals during restoration", enabled, err)
		}
	}
}

func TestSIVIParentSharedRawReferenceSchemaGuards(t *testing.T) {
	service, state, _ := sharedReferenceFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, mutation := range []struct{ change, restore string }{
		{`ALTER TABLE USysTableOfLists ADD COLUMN rowid INTEGER`, `ALTER TABLE USysTableOfLists DROP COLUMN rowid`},
		{`ALTER TABLE USysTableOfLists RENAME COLUMN Flag TO Flags`, `ALTER TABLE USysTableOfLists RENAME COLUMN Flags TO Flag`},
		{`ALTER TABLE USysTableOfLists RENAME TO MissingDefinitions`, `ALTER TABLE MissingDefinitions RENAME TO USysTableOfLists`},
	} {
		if _, err := db.Exec(mutation.change); err != nil {
			t.Fatal(err)
		}
		references, readErr := service.GetReferences(context.Background(), state.ContextID, "108050", ProjectMetadataCell{Storage: "null"})
		if _, err := db.Exec(mutation.restore); err != nil {
			t.Fatal(err)
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		unavailable := 0
		for i, field := range references.Fields {
			if siviParentSharedReferenceFields[i].reader == "family" {
				if field.Available || field.Diagnostic == "" || len(field.Choices) != 0 {
					t.Fatal("raw schema was silently repaired or replaced by legacy Lists", mutation.change, field)
				}
				unavailable++
			}
		}
		if unavailable != 6 {
			t.Fatal("schema guard did not cover exactly six raw-family fields", unavailable)
		}
	}
}
