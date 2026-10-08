package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func speciesAttributeFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	text, null := metadataText, ProjectMetadataCell{Storage: "null"}
	veg := lifeformTable([]string{"PlotNumber", "Species", "Cover1"},
		[]ProjectMetadataCell{text("P"), text("A"), text("not a cover")},
		[]ProjectMetadataCell{text("P"), text("A"), null},
		[]ProjectMetadataCell{text("Q"), text("A"), metadataInteger("999")},
		[]ProjectMetadataCell{text("P"), text("B"), null},
		[]ProjectMetadataCell{text("p"), text("A"), null},
		[]ProjectMetadataCell{text("P"), text("a"), null},
		[]ProjectMetadataCell{null, text("A"), null},
		[]ProjectMetadataCell{text(""), text("A"), null})
	su := lifeformTable([]string{"PlotNumber", "SiteUnit"},
		[]ProjectMetadataCell{text("P"), text("U")},
		[]ProjectMetadataCell{text("P"), text("U")},
		[]ProjectMetadataCell{text("P"), text("V")},
		[]ProjectMetadataCell{text("Q"), text("U")},
		[]ProjectMetadataCell{text("orphan"), text("O")},
		[]ProjectMetadataCell{null, text("N")},
		[]ProjectMetadataCell{text("P"), null},
		[]ProjectMetadataCell{text("P"), text("")},
		[]ProjectMetadataCell{text(""), text("E")})
	names := []string{"Code", "Codetype"}
	for _, definition := range speciesAttributeDefinitions() {
		names = append(names, definition.Field)
	}
	attributes := lifeformTable(names)
	add := func(code, kind ProjectMetadataCell, values ...string) {
		cells := []ProjectMetadataCell{code, kind}
		for _, value := range values {
			cells = append(cells, text(value))
		}
		for len(cells) < len(names) {
			cells = append(cells, null)
		}
		attributes.Rows = append(attributes.Rows, ProjectMetadataRow{RowID: fmt.Sprint(len(attributes.Rows) + 1), Cells: cells})
	}
	add(text("A"), text("u"), "S1", "1", "I", "R", "0", "6")
	add(text("A"), text(""), "S1", "0", "", "b", "7", "1")
	add(text("A"), null, "S1", "1", "I", "R", "0", "6")
	add(text("A"), text("S"), "S1", "1", "I", "R", "0", "6")
	add(text("A"), text("s"), "S1", "1", "I", "R", "0", "6")
	add(text("A"), text("u"))
	add(text("B"), text("U"), "S2S3", "4", "P", "B", "6", "0")
	add(null, text("u"), "S1", "1", "I", "R", "0", "6")
	return veg, su, attributes
}

func TestSpeciesAttributeSummaryRawPhysicalCountsAndLiteralPivots(t *testing.T) {
	veg, su, attributes := speciesAttributeFixture()
	report, err := planSpeciesAttributeSummary(context.Background(), "Project", "Selected", veg, su, attributes)
	if err != nil || len(report.Units) != 7 || len(report.Matches) != 34 {
		t.Fatal("physical joins changed", len(report.Units), len(report.Matches), err)
	}
	unit := report.Units[0]
	if unit.Code.Text == nil || *unit.Code.Text != "U" || unit.NPlots != 3 ||
		!reflect.DeepEqual(unit.SURowIDs, []string{"1", "2", "4"}) {
		t.Fatal("physical unit denominator changed", unit)
	}
	for _, row := range unit.Rows {
		if row.Count == nil || *row.Count != 12 || row.PlotOccurrences != 2 {
			t.Fatal("deduplicated raw Veg, SU or attribute definitions", row)
		}
	}
	if *unit.Rows[0].Categories[0] != 10 || *unit.Rows[0].Categories[2] != 2 ||
		unit.Rows[0].Categories[1] != nil || *unit.Rows[1].Categories[0] != 5 ||
		*unit.Rows[1].Categories[3] != 2 || *unit.Rows[3].Categories[1] != 2 {
		t.Fatal("fixed source pivots, NULL or case identity changed", unit.Rows)
	}
	for _, index := range []int{2, 3, 4} {
		for _, row := range report.Units[index].Rows {
			if row.Count != nil || row.PlotOccurrences != 0 {
				t.Fatal("orphan/NULL identity created observations", index, row)
			}
		}
	}
	again, err := planSpeciesAttributeSummary(context.Background(), "Project", "Selected", veg, su, attributes)
	if err != nil || !reflect.DeepEqual(report, again) {
		t.Fatal("deterministic read differs", err)
	}
	*report.Matches[0].Values[0].Text = "detached"
	*report.Memberships[0].SiteUnit.Text = "detached"
	*report.Units[0].Code.Text = "detached"
	if *attributes.Rows[0].Cells[2].Text != "S1" || *su.Rows[0].Cells[1].Text != "U" ||
		*again.Matches[0].Values[0].Text != "S1" {
		t.Fatal("report cells alias source/previous result")
	}
}

func TestSpeciesAttributeSummaryIndependentSQLiteQueries(t *testing.T) {
	veg, su, attributes := speciesAttributeFixture()
	report, err := planSpeciesAttributeSummary(context.Background(), "Project", "Selected", veg, su, attributes)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for name, table := range map[string]ProjectMetadataTable{"Veg": veg, "SU": su, "Attributes": attributes} {
		columns := ""
		holders := ""
		for i, column := range table.Columns {
			if i != 0 {
				columns += ","
				holders += ","
			}
			columns += quoteHeaderIdentifier(column.Name)
			holders += "?"
		}
		if _, err := db.Exec("CREATE TABLE " + name + "(" + columns + ")"); err != nil {
			t.Fatal(err)
		}
		for _, row := range table.Rows {
			args := []any{}
			for _, cell := range row.Cells {
				value, err := metadataCellValue(cell)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, value)
			}
			if _, err := db.Exec("INSERT INTO "+name+" VALUES("+holders+")", args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, unit := range report.Units {
		var code any
		if unit.Code.Text != nil {
			code = *unit.Code.Text
		}
		for i, definition := range report.Definitions {
			field := "a." + quoteHeaderIdentifier(definition.Field)
			from := " FROM SU s JOIN Veg v ON s.PlotNumber COLLATE BINARY=v.PlotNumber" +
				" JOIN Attributes a ON v.Species COLLATE BINARY=a.Code" +
				" WHERE s.SiteUnit COLLATE BINARY=? AND a.Codetype COLLATE BINARY NOT IN ('S','s') AND " + field + " IS NOT NULL"
			var total sql.NullInt64
			var plots int
			if err := db.QueryRow("SELECT SUM(n) FROM (SELECT COUNT(s.PlotNumber) n"+from+" GROUP BY "+field+")", code).Scan(&total); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT COUNT(DISTINCT v.PlotNumber)"+from, code).Scan(&plots); err != nil {
				t.Fatal(err)
			}
			row := unit.Rows[i]
			if total.Valid != (row.Count != nil) || total.Valid && total.Int64 != int64(*row.Count) || plots != row.PlotOccurrences {
				t.Fatal("independent raw SQL total/plot query differs", unit.Code, definition.Field, total, plots, row)
			}
			for j, literal := range definition.Categories {
				var count sql.NullInt64
				if err := db.QueryRow("SELECT SUM(n) FROM (SELECT COUNT(s.PlotNumber) n"+from+
					" AND "+field+" COLLATE BINARY=? GROUP BY "+field+")", code, literal).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count.Valid != (row.Categories[j] != nil) || count.Valid && count.Int64 != int64(*row.Categories[j]) {
					t.Fatal("independent fixed pivot differs", definition.Field, literal, count, row.Categories[j])
				}
			}
		}
	}
}

func TestSpeciesAttributeSummaryCancellationAndSchemaRefusal(t *testing.T) {
	veg, su, attributes := speciesAttributeFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, request := range []context.Context{nil, ctx} {
		report, err := planSpeciesAttributeSummary(request, "P", "SU", veg, su, attributes)
		if err == nil || !reflect.DeepEqual(report, SpeciesAttributeSummaryReport{}) ||
			request != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("invalid request published", report, err)
		}
	}
	for _, scope := range []string{"None", "USysSuTableDynamic"} {
		if _, err := planSpeciesAttributeSummary(context.Background(), "P", scope, veg, su, attributes); err == nil {
			t.Fatal("unavailable scope became normal SU", scope)
		}
	}
	for _, mutate := range []func(*ProjectMetadataTable){
		func(table *ProjectMetadataTable) { table.Columns[2].Name = "WrongRank" },
		func(table *ProjectMetadataTable) { table.Rows[1].RowID = table.Rows[0].RowID },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells = table.Rows[0].Cells[:1] },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[2] = metadataInteger("1") },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[0] = metadataInteger("1") },
	} {
		_, _, candidate := speciesAttributeFixture()
		mutate(&candidate)
		report, err := planSpeciesAttributeSummary(context.Background(), "P", "SU", veg, su, candidate)
		if err == nil || !reflect.DeepEqual(report, SpeciesAttributeSummaryReport{}) {
			t.Fatal("bad physical source published", report, err)
		}
	}
}
