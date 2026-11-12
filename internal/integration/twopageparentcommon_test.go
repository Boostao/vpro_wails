package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func twoPageCommonEdits(t *testing.T, parent *sourceform.SiviParentProjection) []sourceform.SiviParentScalarEdit {
	t.Helper()
	values := vprotest.TwoPageCommonValues()
	edits := []sourceform.SiviParentScalarEdit{}
	for _, binding := range parent.Bindings {
		if _, exists := values[binding.Binding]; !exists {
			continue
		}
		value := values[binding.Binding]
		edits = append(edits, sftest.SiviParentEdit(t, parent, binding.Binding, value))
		delete(values, binding.Binding)
	}
	return edits
}

func TestTwoPageParentCommonSourceScopeAndExactDomains(t *testing.T) {
	env, admin := vprotest.SiviParentTables(t)
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		parent, err := twopage.ProjectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin)
		if err != nil {
			t.Fatal(err)
		}
		for _, binding := range parent.Bindings {
			if _, common := twopage.TwoPageParentCommonFields[binding.Binding]; common {
				row := &parent.Rows[0].Env
				if binding.Table == parent.AdminTable {
					row = &parent.Rows[0].Admin
				}
				row.Cells[binding.Column] = vprocore.ProjectMetadataCell{Storage: "null"}
			}
		}
		edits := twoPageCommonEdits(t, parent)
		want := 13
		if strings.HasSuffix(form, "-CHARS") {
			want = 14
		}
		got, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, edits)
		if err != nil || len(got) != want || len(edits) != want {
			t.Fatal("source scope inferred a missing height option or duplicated the age assignment", form, got, err)
		}
		for _, assignment := range got {
			if !reflect.DeepEqual(assignment.After, vprotest.TwoPageCommonValues()[assignment.Column]) {
				t.Fatal("common-field literal storage changed", assignment)
			}
		}
		for _, column := range []string{"GIS_BGC_VER", "SV_FullCruiseCard", "PlotNumber"} {
			foreign := sftest.SiviParentEdit(t, parent, column, vprotest.MetadataText("1"))
			if got, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, append(edits, foreign)); err == nil || got != nil {
				t.Fatal("foreign tail leaked a partial common plan", column, got, err)
			}
		}
		if got, err := twopage.PlanTwoPageParentExtraProjection(context.Background(), parent, edits); err == nil || got != nil {
			t.Fatal("additional-field facade widened into common fields", got, err)
		}
	}
}

func TestTwoPageParentCommonTypedThresholdsAndHistoricalOmission(t *testing.T) {
	for column, policy := range twopage.TwoPageParentCommonFields {
		env, admin := vprotest.SiviParentTables(t)
		parent, err := twopage.ProjectTwoPageParent(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL-CHARS", env, admin)
		if err != nil {
			t.Fatal(err)
		}
		row := &parent.Rows[0].Env
		columns := parent.EnvColumns
		if policy.Owner == "Admin" {
			row, columns = &parent.Rows[0].Admin, parent.AdminColumns
		}
		for index, physical := range columns {
			if physical.Name == column {
				row.Cells[index] = vprocore.ProjectMetadataCell{Storage: "null"}
			}
		}
		edit := sftest.SiviParentEdit(t, parent, column, vprotest.TwoPageCommonValues()[column])
		valid, invalid := []vprocore.ProjectMetadataCell{{Storage: "null"}}, []vprocore.ProjectMetadataCell{}
		switch policy.Domain {
		case "scalar":
			if column == "SV_FloodPlain" {
				valid = append(valid, vprotest.MetadataInteger("0"), vprotest.MetadataInteger("-1"))
				invalid = append(invalid, vprotest.MetadataInteger("1"), vprotest.SiviReal(-1), vprotest.MetadataText("-1"))
			} else {
				valid = append(valid, vprotest.SiviReal(-math.MaxFloat32), vprotest.SiviReal(math.MaxFloat32))
				invalid = append(invalid, vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))), vprotest.SiviReal(math.NaN()), vprotest.MetadataInteger("1"))
			}
		case "option":
			valid = append(valid, vprotest.MetadataText("1"), vprotest.MetadataText("2"))
			invalid = append(invalid, vprotest.MetadataText(""), vprotest.MetadataText("01"), vprotest.MetadataText("3"), vprotest.MetadataInteger("1"))
		case "text", "categorical", "plot-type":
			maximum := map[string]int{"SV_PolygonNumber": 25, "SV_CanopyComposition": 50,
				"SV_RootZoneTexture": 100, "SV_AhorizonType": 5, "PlotType": 10}[column]
			valid = append(valid, vprotest.MetadataText(strings.Repeat("x", maximum)),
				vprotest.MetadataText(strings.Repeat("\U0001f600", maximum/2)+strings.Repeat("x", maximum%2)))
			invalid = append(invalid, vprotest.MetadataText(strings.Repeat("x", maximum+1)),
				vprotest.MetadataText(strings.Repeat("\U0001f600", maximum/2+1)), vprotest.MetadataText(string([]byte{0xff})), vprotest.MetadataInteger("1"))
			if policy.Domain == "categorical" {
				valid = append(valid, vprotest.MetadataText(""))
			} else {
				invalid = append(invalid, vprotest.MetadataText(""))
			}
		}
		for _, value := range valid {
			edit.Value = value
			got, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil {
				t.Fatal("valid common domain rejected", column, value, got, err)
			}
		}
		for _, value := range invalid {
			edit.Value = value
			if got, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
				t.Fatal("invalid common value became a plan", column, value, got, err)
			}
		}
		historical := vprotest.MetadataText(strings.Repeat("historical", 30))
		binding := sftest.SiviParentEdit(t, parent, column, historical)
		for index, physical := range columns {
			if physical.Name == column {
				row.Cells[index] = historical
			}
		}
		binding.Expected = historical
		got, err := twopage.PlanTwoPageParentCommonProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{binding})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("historical invalid common storage reassigned", column, got, err)
		}
	}
}

func TestTwoPageParentCommonOwnedAuditsRestorationAndScopeIsolation(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			service, state, db, parent, _ := twoPageWriteFixture(t, form, external)
			before := vprotest.TwoPageWriteTables(t, db)
			edits := twoPageCommonEdits(t, parent)
			written, err := twopage.WriteTwoPageParentCommon(service, context.Background(), state.ContextID, "108050", form, parent, edits)
			if err != nil || written == nil || written.ChangedCells != len(edits) || written.HistoryID == "" {
				t.Fatal("common transaction did not commit exact scope", form, external, written, err)
			}
			history, err := twopage.TwoPageParentCommonHistory(form)
			if err != nil {
				t.Fatal(err)
			}
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			var event sourceform.SiviParentHistory
			if err := json.Unmarshal([]byte(proposal), &event); err != nil || len(event.Changes) != len(edits) || !reflect.DeepEqual(event.Original, parent) {
				t.Fatal("common history lost exact original or assignment scope", err)
			}
			fresh, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range edits {
				if !reflect.DeepEqual(sftest.SiviParentCell(t, fresh, edit.Column), edit.Value) {
					t.Fatal("common write storage differs", edit.Column)
				}
			}
			bytes := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if got, err := twopage.RestoreTwoPageParentExtra(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
				t.Fatal("additional history consumed a common event", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service, bytes)
			action := vprocore.AuditRestorePrune
			if external {
				action = vprocore.AuditRestoreRetain
			}
			restored, err := twopage.RestoreTwoPageParentCommon(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != len(edits) || restored.CleanedVegRows != 0 {
				t.Fatal("common restoration widened scope", restored, err)
			}
			reloaded, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
			if err != nil || !reflect.DeepEqual(reloaded, parent) {
				t.Fatal("common restoration differs from original", err)
			}
			after := vprotest.TwoPageWriteTables(t, db)
			for table, original := range before {
				if table != "sqlite_master" && (table != "Sample_Audit" || action == vprocore.AuditRestorePrune) && !reflect.DeepEqual(original, after[table]) {
					t.Fatal("common restoration changed an original table", table)
				}
			}
			ownedAudits := map[string]bool{}
			for _, change := range event.Changes {
				var count, flag int
				if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, change.Audit.RowID).Scan(&count, &flag); err != nil ||
					(action == vprocore.AuditRestoreRetain && (count != 1 || flag != -1)) || (action == vprocore.AuditRestorePrune && count != 0) {
					t.Fatal("owned common audit retained/pruned incorrectly", count, flag, err)
				}
				ownedAudits[change.Audit.RowID] = true
			}
			audits := after["Sample_Audit"]
			rows := []vprocore.ProjectMetadataRow{}
			for _, row := range audits.Rows {
				if !ownedAudits[row.RowID] {
					rows = append(rows, row)
				}
			}
			audits.Rows = rows
			if !reflect.DeepEqual(before["Sample_Audit"], audits) {
				t.Fatal("common restoration changed pre-existing audits")
			}
			bytes = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if got, err := twopage.RestoreTwoPageParentCommon(service, context.Background(), state.ContextID, "108050", form, written.HistoryID, action); err == nil || got != nil {
				t.Fatal("consumed common history replayed", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service, bytes)
		}
	}
}

func TestTwoPageParentCommonRollbackCancellationAndCollision(t *testing.T) {
	for _, failure := range []string{"audit", "stale", "cancel", "foreign-tail"} {
		service, state, db, parent, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", true)
		edits := twoPageCommonEdits(t, parent)
		ctx := context.Background()
		switch failure {
		case "audit":
			if _, err := db.Exec(`CREATE TRIGGER common_audit_failure BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='PlotType' BEGIN SELECT RAISE(ABORT,'common late audit failure'); END`); err != nil {
				t.Fatal(err)
			}
		case "stale":
			if _, err := db.Exec(`UPDATE Sample_Admin SET PlotType='independent' WHERE Plot='108050'`); err != nil {
				t.Fatal(err)
			}
		case "cancel":
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			ctx = cancelled
		case "foreign-tail":
			edits = append(edits, sftest.SiviParentEdit(t, parent, "GIS_BGC", vprotest.MetadataText("forbidden")))
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		got, err := twopage.WriteTwoPageParentCommon(service, ctx, state.ContextID, "108050", parent.Form, parent, edits)
		if err == nil || got != nil || failure == "cancel" && !errors.Is(err, context.Canceled) {
			t.Fatal("common failure became success", failure, got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
}
