package integration

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func sharedReferenceEdits(t *testing.T, service *sourceform.SIVIParentSharedService, state vprocore.ProjectState, original *sourceform.SIVIParentProjection) []sourceform.SIVIParentCellEdit {
	t.Helper()
	references, err := service.GetReferences(context.Background(), state.ContextID, "108050", vprocore.ProjectMetadataCell{Storage: "null"})
	if err != nil {
		t.Fatal(err)
	}
	edits := []sourceform.SIVIParentCellEdit{}
	for i, field := range sourceform.SiviParentSharedReferenceFields {
		value := strings.Repeat("X", field.Maximum)
		if field.Required {
			found := false
			for _, choice := range references.Fields[i].Choices {
				if choice.Selectable && choice.Code != nil {
					value = *choice.Code
					found = true
					break
				}
			}
			if !found {
				t.Fatal("required catalogue unavailable", field.Column, references.Fields[i])
			}
		}
		edits = append(edits, sftest.SiviParentEdit(t, original, field.Column, vprocore.ProjectMetadataCell{Storage: "text", Text: &value}))
	}
	return edits
}

func TestSIVIParentSharedReferencePhysicalPolicies(t *testing.T) {
	service, _, original := sftest.SharedReferenceFixture(t)
	if len(sourceform.SiviParentSharedReferenceFields) != 28 || len(sourceform.SiviParentSharedOwners) != 32 {
		t.Fatal("scope count changed")
	}
	ctx := context.Background()
	for _, field := range sourceform.SiviParentSharedReferenceFields {
		bound := strings.Repeat("\U0001f600", field.Maximum/2) + strings.Repeat("x", field.Maximum%2)
		for _, value := range []vprocore.ProjectMetadataCell{{Storage: "null"}, {Storage: "text", Text: &bound}} {
			edit := sftest.SiviParentEdit(t, original, field.Column, value)
			if _, err := sourceform.PlanSIVIParentSharedScope(ctx, original, []sourceform.SIVIParentCellEdit{edit}, true); err != nil {
				t.Fatal("valid reference text/null rejected", field.Column, err)
			}
		}
		for _, value := range []vprocore.ProjectMetadataCell{
			{Storage: "text", Text: vprotest.SiviSharedTestPointer(bound + "x")},
			{Storage: "text", Text: vprotest.SiviSharedTestPointer("")},
			{Storage: "text", Text: vprotest.SiviSharedTestPointer(string([]byte{0xff}))},
			{Storage: "integer", Integer: vprotest.SiviSharedTestPointer("1")},
		} {
			if _, err := sourceform.PlanSIVIParentSharedScope(ctx, original, []sourceform.SIVIParentCellEdit{sftest.SiviParentEdit(t, original, field.Column, value)}, true); err == nil {
				t.Fatal("invalid physical reference accepted", field.Column)
			}
		}
	}
	_ = service
}

func TestSIVIParentSharedSixRawReferenceDefinitionsOwnershipAndRetry(t *testing.T) {
	service, state, _ := sftest.SharedReferenceFixture(t)
	ctx := context.Background()
	zone := vprocore.ProjectMetadataCell{Storage: "null"}
	before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
	references, err := service.GetReferences(ctx, state.ContextID, "108050", zone)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{"MoistureRegime": 10, "NutrientRegime": 7, "MesoSlopePosition": 9, "SurfaceShape": 4, "StructuralStage": 21, "SuccessionalStatus": 10}
	for _, field := range references.Fields {
		if count, raw := counts[field.Column]; raw {
			if !field.Available || len(field.Definitions.Rows) != count || !reflect.DeepEqual(field.Definitions.Columns, sourceform.SiviParentReferenceColumns) ||
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
	vprotest.AssertProfileSUFiles(t, service.Contexts, before)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Contexts.Projects.Sqlite.Attachments["VLists"], "rw"))
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
	var raw sourceform.SIVIParentSharedReference
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
	for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, original := sftest.SharedReferenceFixture(t)
			ctx := context.Background()
			edits := sharedReferenceEdits(t, service, state, original)
			before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
			required := 0
			for i, field := range sourceform.SiviParentSharedReferenceFields {
				if !field.Required {
					continue
				}

				required++
				bad := append([]sourceform.SIVIParentCellEdit(nil), edits...)
				bad[i].Value = vprocore.ProjectMetadataCell{Storage: "text", Text: vprotest.SiviSharedTestPointer("?")}
				if _, err := service.Save(ctx, state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: bad}); err == nil {
					t.Fatal("nonmember required field accepted", field.Column)
				}
				vprotest.AssertProfileSUFiles(t, service.Contexts, before)
			}
			if required != 4 {
				t.Fatal("membership policy widened")
			}
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TRIGGER shared_ref_abort BEFORE INSERT ON Sample_Audit
				WHEN NEW.EditField='SuccessionalStatus' BEGIN SELECT RAISE(ABORT,'reference audit failure'); END`); err != nil {
				t.Fatal(err)
			}
			before = vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
			if _, err := service.Save(ctx, state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: edits}); err == nil {
				t.Fatal("reference audit failure committed")
			}
			vprotest.AssertProfileSUFiles(t, service.Contexts, before)
			if _, err := db.Exec(`DROP TRIGGER shared_ref_abort`); err != nil {
				t.Fatal(err)
			}
			result, err := service.Save(ctx, state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: edits})
			if err != nil || result.ChangedCells != 28 {
				t.Fatal("28-field atomic reference Save failed", result, err)
			}
			if _, err := service.Save(ctx, state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: edits}); err == nil {
				t.Fatal("reference stale collision accepted")
			}
			service.References = sourceform.SiviParentSharedReferenceReaders{}
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
	rows []referencespkg.ParentCodeChoice
}

func (stub sharedReferenceParentStub) ListChoices(context.Context, string) ([]referencespkg.ParentCodeChoice, error) {
	return stub.rows, nil
}

func TestSIVIParentSharedBorrowedValidityAndEmptyAvailability(t *testing.T) {
	service, state, original := sftest.SharedReferenceFixture(t)
	code, list := "Q", "SoilDrainage"
	service.References.Parent = sharedReferenceParentStub{[]referencespkg.ParentCodeChoice{
		{RowID: "1", ListName: &list, Code: &code, Selectable: false, Diagnostic: "source definition invalid"},
	}}
	references, err := service.GetReferences(context.Background(), state.ContextID, "108050", vprocore.ProjectMetadataCell{Storage: "null"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range references.Fields {
		if field.Column == list && (!field.Available || field.Choices[0].Selectable ||
			field.Choices[0].Diagnostic != "source definition invalid" || len(field.Definitions.Rows[0].Cells) != 10) {
			t.Fatal("borrowed validity or metadata was overwritten", field)
		}
	}
	edit := sftest.SiviParentEdit(t, original, list, vprocore.ProjectMetadataCell{Storage: "text", Text: &code})
	if _, err := service.Save(context.Background(), state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: []sourceform.SIVIParentCellEdit{edit}}); err == nil {
		t.Fatal("unselectable borrowed Item authorized a write")
	}
	service.References.Parent = sharedReferenceParentStub{}
	references, err = service.GetReferences(context.Background(), state.ContextID, "108050", vprocore.ProjectMetadataCell{Storage: "null"})
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
	service, state, original := sftest.SharedReferenceFixture(t)
	edits := sharedReferenceEdits(t, service, state, original)
	edits = append(edits, sharedTextEdits(t, original)...)
	edits = append(edits, sharedNumericEdits(t, original)...)
	for _, column := range []string{"AirPhotoNum", "XCoord", "YCoord", "StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss", "VegNotes"} {
		value := vprocore.ProjectMetadataCell{Storage: "real", Real: vprotest.SiviSharedTestPointer(3.25)}
		if column == "AirPhotoNum" || column == "VegNotes" {
			value = vprocore.ProjectMetadataCell{Storage: "text", Text: vprotest.SiviSharedTestPointer("  Literal  ")}
		}
		edits = append(edits, sftest.SiviParentEdit(t, original, column, value))
	}
	for _, column := range []string{"HumusThickness", "SeepageDepth", "RootingDepth", "RootRestrictingDepth"} {
		value := vprocore.ProjectMetadataCell{Storage: "integer", Integer: vprotest.SiviSharedTestPointer("-32768")}
		if column == "HumusThickness" {
			value = vprocore.ProjectMetadataCell{Storage: "real", Real: vprotest.SiviSharedTestPointer(12.5)}
		}
		edits = append(edits, sftest.SiviParentEdit(t, original, column, value))
	}
	edits = append(edits, sftest.SiviParentEdit(t, original, "Date",
		vprocore.ProjectMetadataCell{Storage: "text", Text: vprotest.SiviSharedTestPointer("9999-12-31 23:59:59.123456789")}))
	if len(edits) != 60 || original.Rows[0].Env.RowID == original.Rows[0].Admin.RowID {
		t.Fatal("mixed-scope fixture does not cover 60 fields and distinct owners")
	}
	request := sourceform.SIVIParentSharedWrite{Original: original, Edits: edits}
	before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
	for _, column := range []string{"HumusThickness", "Exposure1"} {
		wrong := append([]sourceform.SIVIParentCellEdit(nil), edits...)
		for i := range wrong {
			if wrong[i].Column == column {
				wrong[i].RowID = original.Rows[0].Env.RowID
				if column == "Exposure1" {
					wrong[i].RowID = original.Rows[0].Admin.RowID
				}
			}
		}
		if _, err := service.Save(context.Background(), state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: wrong}); err == nil {
			t.Fatal("cross-table physical identity accepted", column)
		}
		vprotest.AssertProfileSUFiles(t, service.Contexts, before)
	}
	result, err := service.Save(context.Background(), state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != 60 {
		t.Fatal("mixed 60-field transaction failed", result, err)
	}
	if _, err := service.Restore(context.Background(), state.ContextID, "108050", result.HistoryID, vprocore.AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("60-field typed restoration changed originals", err)
	}
}

func TestSIVIParentSharedReferenceHistoricalNoOpNullableAndMissingCatalogues(t *testing.T) {
	for _, action := range []vprocore.AuditRestoreAction{vprocore.AuditRestoreRetain, vprocore.AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, _ := sftest.SharedReferenceFixture(t)
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`UPDATE Sample_Env SET Exposure1=X'00FF',Exposure2='',SoilDrainage='LONG-HISTORICAL',SuccessionalStatus='???' WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			service.References = sourceform.SiviParentSharedReferenceReaders{}
			original, err := service.GetOriginal(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			unchanged, nullable := []sourceform.SIVIParentCellEdit{}, []sourceform.SIVIParentCellEdit{}
			for _, field := range sourceform.SiviParentSharedReferenceFields {
				if field.Required {
					edit := sftest.SiviParentEdit(t, original, field.Column, vprocore.ProjectMetadataCell{Storage: "null"})
					if edit.Expected.Storage != "blob" {
						nullable = append(nullable, edit)
					}
					edit.Value = edit.Expected
					unchanged = append(unchanged, edit)
				}
			}
			before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
			result, err := service.Save(context.Background(), state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: unchanged})
			if err != nil || result.ChangedCells != 0 {
				t.Fatal("historical required no-op was revalidated", result, err)
			}
			vprotest.AssertProfileSUFiles(t, service.Contexts, before)
			blob := sftest.SiviParentEdit(t, original, "Exposure1", vprocore.ProjectMetadataCell{Storage: "null"})
			if _, err := service.Save(context.Background(), state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: []sourceform.SIVIParentCellEdit{blob}}); err == nil {
				t.Fatal("historical BLOB replacement bypassed lossless audit guard")
			}
			vprotest.AssertProfileSUFiles(t, service.Contexts, before)
			result, err = service.Save(context.Background(), state.ContextID, "108050", sourceform.SIVIParentSharedWrite{Original: original, Edits: nullable})
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

func TestSIVIParentSharedReferencePreservesAcceptedThirtyTwo(t *testing.T) {
	service, state, original, request := sftest.SharedServiceFixture(t)
	ctx := context.Background()
	fresh, err := service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("reference support changed full physical originals", err)
	}
	for _, field := range sourceform.SiviParentSharedReferenceFields {
		_ = sftest.SiviParentEdit(t, fresh, field.Column, vprocore.ProjectMetadataCell{Storage: "null"})
	}
	references, err := service.GetReferences(ctx, state.ContextID, "108050", vprocore.ProjectMetadataCell{Storage: "null"})
	if err != nil || len(references.Fields) != 28 {
		t.Fatal("runtime reference read failed", err)
	}
	result, err := service.Save(ctx, state.ContextID, "108050", request)
	if err != nil || result.ChangedCells != len(request.Edits) {
		t.Fatal("reference support disabled accepted 32-field Save", result, err)
	}
	if _, err := service.Restore(ctx, state.ContextID, "108050", result.HistoryID, vprocore.AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	fresh, err = service.GetOriginal(ctx, state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(original, fresh) {
		t.Fatal("reference support changed accepted originals during restoration", err)
	}
}

func TestSIVIParentSharedRawReferenceSchemaGuards(t *testing.T) {
	service, state, _ := sftest.SharedReferenceFixture(t)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Contexts.Projects.Sqlite.Attachments["VLists"], "rw"))
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
		references, readErr := service.GetReferences(context.Background(), state.ContextID, "108050", vprocore.ProjectMetadataCell{Storage: "null"})
		if _, err := db.Exec(mutation.restore); err != nil {
			t.Fatal(err)
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		unavailable := 0
		for i, field := range references.Fields {
			if sourceform.SiviParentSharedReferenceFields[i].Reader == "family" {
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
