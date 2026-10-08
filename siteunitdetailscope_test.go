package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func siteUnitDetailScopeFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("P1")}},
		{"2", []ProjectMetadataCell{metadataText("P1")}},
		{"3", []ProjectMetadataCell{metadataText("P2")}},
		{"4", []ProjectMetadataCell{metadataText("")}},
	}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot"}}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("P1")}},
		{"2", []ProjectMetadataCell{metadataText("P1")}},
		{"3", []ProjectMetadataCell{metadataText("P3")}},
		{"4", []ProjectMetadataCell{metadataText("")}},
	}}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("P1"), metadataText("U")}},
		{"2", []ProjectMetadataCell{metadataText("P1"), metadataText("U")}},
		{"3", []ProjectMetadataCell{metadataText("P2"), metadataText("U")}},
		{"4", []ProjectMetadataCell{metadataText("P3"), metadataText("V")}},
		{"5", []ProjectMetadataCell{{Storage: "null"}, metadataText("W")}},
		{"6", []ProjectMetadataCell{metadataText("P1"), {Storage: "null"}}},
		{"7", []ProjectMetadataCell{metadataText(""), metadataText("")}},
	}}
	return env, admin, su
}

func TestSiteUnitDetailScopePhysicalWeightsAndIndependentSQLite(t *testing.T) {
	env, admin, su := siteUnitDetailScopeFixture()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, schema := range []string{"CREATE TABLE env(plot TEXT)", "CREATE TABLE admin(plot TEXT)", "CREATE TABLE su(plot TEXT, unit TEXT)"} {
		if _, err := db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	for name, table := range map[string]ProjectMetadataTable{"env": env, "admin": admin, "su": su} {
		for _, row := range table.Rows {
			values := []any{row.RowID}
			for _, cell := range row.Cells {
				value, err := metadataCellValue(cell)
				if err != nil {
					t.Fatal(err)
				}
				values = append(values, value)
			}
			query := "INSERT INTO " + name + "(rowid,plot) VALUES(?,?)"
			if name == "su" {
				query = "INSERT INTO su(rowid,plot,unit) VALUES(?,?,?)"
			}
			if _, err := db.Exec(query, values...); err != nil {
				t.Fatal(err)
			}
		}
	}
	expected := map[string][]siteUnitDetailJoinedPlot{}
	rows, err := db.Query(`SELECT s.unit,e.plot,CAST(s.rowid AS TEXT),CAST(e.rowid AS TEXT),CAST(a.rowid AS TEXT)
		FROM su s JOIN env e ON e.plot=s.plot JOIN admin a ON a.plot=e.plot WHERE s.unit IS NOT NULL
		ORDER BY s.unit DESC,e.plot,CAST(s.rowid AS TEXT),CAST(e.rowid AS TEXT),CAST(a.rowid AS TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var unit string
		var plot siteUnitDetailJoinedPlot
		if err := rows.Scan(&unit, &plot.PlotNumber, &plot.SURowID, &plot.EnvRowID, &plot.AdminRowID); err != nil {
			t.Fatal(err)
		}
		expected[unit] = append(expected[unit], plot)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	for _, source := range []siteUnitDetailQuerySource{siteUnitDetailProjectJoin, siteUnitDetailSelectedSU} {
		result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", source, 9, env, admin, su)
		if err != nil || len(result.Units) != 2 || result.Units[0].Code != "U" || len(result.Units[0].Plots) != 8 ||
			result.Units[1].Code != "" || len(result.Units[1].Plots) != 1 || result.QuerySource != source {
			t.Fatal("exact duplicate-weighted/empty-unit threshold differs", result, err)
		}
		for _, unit := range result.Units {
			if !reflect.DeepEqual(unit.Plots, expected[unit.Code]) {
				t.Fatal("independent SQLite physical join differs", unit, expected[unit.Code])
			}
		}
		statuses := []string{"joined", "joined", "missing-admin", "missing-env", "null-plot", "null-unit", "joined"}
		for i, member := range result.Memberships {
			if member.Status != statuses[i] {
				t.Fatal("excluded membership lost its explicit reason", member)
			}
		}
		refused, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", source, 8, env, admin, su)
		if err == nil || !reflect.DeepEqual(refused, siteUnitDetailScope{}) {
			t.Fatal("one-row-over-budget published partial output", refused, err)
		}
	}
}

func TestSiteUnitDetailScopeLiteralsPermutationAndNoAliases(t *testing.T) {
	env, admin, su := siteUnitDetailScopeFixture()
	*su.Rows[0].Cells[1].Text = " O'Brien\r\n\x00\u6811 "
	original, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 9, env, admin, su)
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(0); seed < 8; seed++ {
		random := rand.New(rand.NewSource(seed))
		for _, table := range []*ProjectMetadataTable{&env, &admin, &su} {
			random.Shuffle(len(table.Rows), func(i, j int) { table.Rows[i], table.Rows[j] = table.Rows[j], table.Rows[i] })
		}
		result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 9, env, admin, su)
		if err != nil || !reflect.DeepEqual(result, original) {
			t.Fatal("physical input order changed preparation", result, err)
		}
	}
	*original.Memberships[0].SiteUnit.Text = "mutated output"
	result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 9, env, admin, su)
	if err != nil || *result.Memberships[0].SiteUnit.Text != " O'Brien\r\n\x00\u6811 " {
		t.Fatal("returned membership aliased input", result, err)
	}
}

func TestSiteUnitDetailScopeErrorsCancellationAndEmptyScope(t *testing.T) {
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable){
		func(e, a, s *ProjectMetadataTable) { e.Columns[0].Name = "wrong" },
		func(e, a, s *ProjectMetadataTable) { a.Rows[0].RowID = "01" },
		func(e, a, s *ProjectMetadataTable) { s.Rows[1].RowID = s.Rows[0].RowID },
		func(e, a, s *ProjectMetadataTable) { s.Rows[0].Cells = s.Rows[0].Cells[:1] },
		func(e, a, s *ProjectMetadataTable) { s.Rows[0].Cells[1] = metadataInteger("1") },
		func(e, a, s *ProjectMetadataTable) { a.Rows[0].Cells[0] = metadataInteger("1") },
	} {
		env, admin, su := siteUnitDetailScopeFixture()
		mutate(&env, &admin, &su)
		result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 9, env, admin, su)
		if err == nil || !reflect.DeepEqual(result, siteUnitDetailScope{}) {
			t.Fatal("malformed boundary published partial success", result, err)
		}
	}
	env, admin, su := siteUnitDetailScopeFixture()
	for remaining := 1; remaining <= 15; remaining++ {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		result, err := prepareSiteUnitDetailScope(ctx, "Project", "Selected", siteUnitDetailProjectJoin, 9, env, admin, su)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, siteUnitDetailScope{}) {
			t.Fatal("cancelled partial preparation escaped", remaining, result, err)
		}
	}
	for _, source := range []siteUnitDetailQuerySource{"", "unknown-filter", "hierarchy", "field-derived"} {
		if result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", source, 9, env, admin, su); err == nil ||
			!reflect.DeepEqual(result, siteUnitDetailScope{}) {
			t.Fatal("unknown provenance guessed", source, result, err)
		}
	}
	su.Rows = nil
	result, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 1, env, admin, su)
	if err != nil || result.Units == nil || result.Memberships == nil || len(result.Units) != 0 || len(result.Memberships) != 0 {
		t.Fatal("empty scope differs from failure", result, err)
	}
	if _, err := prepareSiteUnitDetailScope(context.Background(), "Project", "None", siteUnitDetailProjectJoin, 1, env, admin, su); err == nil {
		t.Fatal("unselected SU supplied authority")
	}
	if _, err := prepareSiteUnitDetailScope(context.Background(), "Project", "Selected", siteUnitDetailProjectJoin, 0, env, admin, su); err == nil {
		t.Fatal("zero budget silently defaulted")
	}
}
