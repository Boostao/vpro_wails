package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func twoPageCommonValues() map[string]ProjectMetadataCell {
	return map[string]ProjectMetadataCell{
		"SV_StandHeight": siviReal(-3.25), "SV_AhorizonDepth": siviReal(0.1),
		"SV_GleyingMottlingCM": siviReal(25), "SV_PercentCoarseFrags": siviReal(101),
		"SV_SoilDepth": siviReal(0), "StrataCoverTotal": siviReal(125),
		"SV_FloodPlain": metadataInteger("-1"), "SV_StandAgeEstMeas": metadataText("2"),
		"SV_StandHeightEstMeas": metadataText("2"), "SV_PolygonNumber": metadataText("  MiXeD  "),
		"SV_CanopyComposition": metadataText("e\u0301 Canopy"), "SV_RootZoneTexture": metadataText(""),
		"SV_AhorizonType": metadataText(" Ah "), "PlotType": metadataText("  Custom  "),
	}
}

func twoPageCommonEdits(t *testing.T, parent *siviParentProjection) []siviParentScalarEdit {
	t.Helper()
	values := twoPageCommonValues()
	edits := []siviParentScalarEdit{}
	for _, binding := range parent.Bindings {
		if _, exists := values[binding.Binding]; !exists {
			continue
		}
		value := values[binding.Binding]
		edits = append(edits, siviParentEdit(t, parent, binding.Binding, value))
		delete(values, binding.Binding)
	}
	return edits
}

func TestTwoPageParentCommonSourceScopeAndExactDomains(t *testing.T) {
	env, admin := siviParentTables(t)
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		parent, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin)
		if err != nil {
			t.Fatal(err)
		}
		for _, binding := range parent.Bindings {
			if _, common := twoPageParentCommonFields[binding.Binding]; common {
				row := &parent.Rows[0].Env
				if binding.Table == parent.AdminTable {
					row = &parent.Rows[0].Admin
				}
				row.Cells[binding.Column] = ProjectMetadataCell{Storage: "null"}
			}
		}
		edits := twoPageCommonEdits(t, parent)
		want := 13
		if strings.HasSuffix(form, "-CHARS") {
			want = 14
		}
		got, err := planTwoPageParentCommonProjection(context.Background(), parent, edits)
		if err != nil || len(got) != want || len(edits) != want {
			t.Fatal("source scope inferred a missing height option or duplicated the age assignment", form, got, err)
		}
		for _, assignment := range got {
			if !reflect.DeepEqual(assignment.After, twoPageCommonValues()[assignment.Column]) {
				t.Fatal("common-field literal storage changed", assignment)
			}
		}
		for _, column := range []string{"GIS_BGC_VER", "SV_FullCruiseCard", "PlotNumber"} {
			foreign := siviParentEdit(t, parent, column, metadataText("1"))
			if got, err := planTwoPageParentCommonProjection(context.Background(), parent, append(edits, foreign)); err == nil || got != nil {
				t.Fatal("foreign tail leaked a partial common plan", column, got, err)
			}
		}
		if got, err := planTwoPageParentExtraProjection(context.Background(), parent, edits); err == nil || got != nil {
			t.Fatal("additional-field facade widened into common fields", got, err)
		}
	}
}

func TestTwoPageParentCommonTypedThresholdsAndHistoricalOmission(t *testing.T) {
	for column, policy := range twoPageParentCommonFields {
		env, admin := siviParentTables(t)
		parent, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL-CHARS", env, admin)
		if err != nil {
			t.Fatal(err)
		}
		row := &parent.Rows[0].Env
		columns := parent.EnvColumns
		if policy.owner == "Admin" {
			row, columns = &parent.Rows[0].Admin, parent.AdminColumns
		}
		for index, physical := range columns {
			if physical.Name == column {
				row.Cells[index] = ProjectMetadataCell{Storage: "null"}
			}
		}
		edit := siviParentEdit(t, parent, column, twoPageCommonValues()[column])
		valid, invalid := []ProjectMetadataCell{{Storage: "null"}}, []ProjectMetadataCell{}
		switch policy.domain {
		case "scalar":
			if column == "SV_FloodPlain" {
				valid = append(valid, metadataInteger("0"), metadataInteger("-1"))
				invalid = append(invalid, metadataInteger("1"), siviReal(-1), metadataText("-1"))
			} else {
				valid = append(valid, siviReal(-math.MaxFloat32), siviReal(math.MaxFloat32))
				invalid = append(invalid, siviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))), siviReal(math.NaN()), metadataInteger("1"))
			}
		case "option":
			valid = append(valid, metadataText("1"), metadataText("2"))
			invalid = append(invalid, metadataText(""), metadataText("01"), metadataText("3"), metadataInteger("1"))
		case "text", "categorical", "plot-type":
			maximum := map[string]int{"SV_PolygonNumber": 25, "SV_CanopyComposition": 50,
				"SV_RootZoneTexture": 100, "SV_AhorizonType": 5, "PlotType": 10}[column]
			valid = append(valid, metadataText(strings.Repeat("x", maximum)),
				metadataText(strings.Repeat("\U0001f600", maximum/2)+strings.Repeat("x", maximum%2)))
			invalid = append(invalid, metadataText(strings.Repeat("x", maximum+1)),
				metadataText(strings.Repeat("\U0001f600", maximum/2+1)), metadataText(string([]byte{0xff})), metadataInteger("1"))
			if policy.domain == "categorical" {
				valid = append(valid, metadataText(""))
			} else {
				invalid = append(invalid, metadataText(""))
			}
		}
		for _, value := range valid {
			edit.Value = value
			got, err := planTwoPageParentCommonProjection(context.Background(), parent, []siviParentScalarEdit{edit})
			if err != nil || got == nil {
				t.Fatal("valid common domain rejected", column, value, got, err)
			}
		}
		for _, value := range invalid {
			edit.Value = value
			if got, err := planTwoPageParentCommonProjection(context.Background(), parent, []siviParentScalarEdit{edit}); err == nil || got != nil {
				t.Fatal("invalid common value became a plan", column, value, got, err)
			}
		}
		historical := metadataText(strings.Repeat("historical", 30))
		binding := siviParentEdit(t, parent, column, historical)
		for index, physical := range columns {
			if physical.Name == column {
				row.Cells[index] = historical
			}
		}
		binding.Expected = historical
		got, err := planTwoPageParentCommonProjection(context.Background(), parent, []siviParentScalarEdit{binding})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("historical invalid common storage reassigned", column, got, err)
		}
	}
}

func TestTwoPageParentCommonOwnedAuditsRestorationAndScopeIsolation(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, external := range []bool{false, true} {
			service, state, db, parent, _ := twoPageWriteFixture(t, form, external)
			before := twoPageWriteTables(t, db)
			edits := twoPageCommonEdits(t, parent)
			written, err := service.writeTwoPageParentCommon(context.Background(), state.ContextID, "108050", form, parent, edits)
			if err != nil || written == nil || written.ChangedCells != len(edits) || written.HistoryID == "" {
				t.Fatal("common transaction did not commit exact scope", form, external, written, err)
			}
			history, err := twoPageParentCommonHistory(form)
			if err != nil {
				t.Fatal(err)
			}
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM `+quoteHeaderIdentifier(history.table)+` WHERE ID=?`, written.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			var event siviParentHistory
			if err := json.Unmarshal([]byte(proposal), &event); err != nil || len(event.Changes) != len(edits) || !reflect.DeepEqual(event.Original, parent) {
				t.Fatal("common history lost exact original or assignment scope", err)
			}
			fresh, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range edits {
				if !reflect.DeepEqual(siviParentCell(t, fresh, edit.Column), edit.Value) {
					t.Fatal("common write storage differs", edit.Column)
				}
			}
			bytes := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.restoreTwoPageParentExtra(context.Background(), state.ContextID, "108050", form, written.HistoryID, AuditRestorePrune); err == nil || got != nil {
				t.Fatal("additional history consumed a common event", got, err)
			}
			assertProfileSUFiles(t, service, bytes)
			action := AuditRestorePrune
			if external {
				action = AuditRestoreRetain
			}
			restored, err := service.restoreTwoPageParentCommon(context.Background(), state.ContextID, "108050", form, written.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != len(edits) || restored.CleanedVegRows != 0 {
				t.Fatal("common restoration widened scope", restored, err)
			}
			reloaded, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
			if err != nil || !reflect.DeepEqual(reloaded, parent) {
				t.Fatal("common restoration differs from original", err)
			}
			after := twoPageWriteTables(t, db)
			for table, original := range before {
				if table != "sqlite_master" && (table != "Sample_Audit" || action == AuditRestorePrune) && !reflect.DeepEqual(original, after[table]) {
					t.Fatal("common restoration changed an original table", table)
				}
			}
			ownedAudits := map[string]bool{}
			for _, change := range event.Changes {
				var count, flag int
				if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, change.Audit.RowID).Scan(&count, &flag); err != nil ||
					(action == AuditRestoreRetain && (count != 1 || flag != -1)) || (action == AuditRestorePrune && count != 0) {
					t.Fatal("owned common audit retained/pruned incorrectly", count, flag, err)
				}
				ownedAudits[change.Audit.RowID] = true
			}
			audits := after["Sample_Audit"]
			rows := []ProjectMetadataRow{}
			for _, row := range audits.Rows {
				if !ownedAudits[row.RowID] {
					rows = append(rows, row)
				}
			}
			audits.Rows = rows
			if !reflect.DeepEqual(before["Sample_Audit"], audits) {
				t.Fatal("common restoration changed pre-existing audits")
			}
			bytes = databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.restoreTwoPageParentCommon(context.Background(), state.ContextID, "108050", form, written.HistoryID, action); err == nil || got != nil {
				t.Fatal("consumed common history replayed", got, err)
			}
			assertProfileSUFiles(t, service, bytes)
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
			edits = append(edits, siviParentEdit(t, parent, "GIS_BGC", metadataText("forbidden")))
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		got, err := service.writeTwoPageParentCommon(ctx, state.ContextID, "108050", parent.Form, parent, edits)
		if err == nil || got != nil || failure == "cancel" && !errors.Is(err, context.Canceled) {
			t.Fatal("common failure became success", failure, got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
}
