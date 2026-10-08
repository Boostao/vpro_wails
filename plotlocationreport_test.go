package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func locationTestTables() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	for _, field := range plotLocationFields() {
		env.Columns = append(env.Columns, ProjectMetadataColumn{Name: field.Key, DeclaredType: "VARCHAR"})
	}
	null := ProjectMetadataCell{Storage: "null"}
	env.Rows = []ProjectMetadataRow{
		{"-9007199254740993", []ProjectMetadataCell{metadataText("000001"), metadataText("  Zone  "), metadataText(""), metadataText("01"), null, siviReal(49.25), siviReal(123.75), metadataInteger("9007199254740993")}},
		{"2", []ProjectMetadataCell{metadataText("000002"), null, null, null, metadataInteger("0"), siviReal(math.Copysign(0, -1)), siviReal(-123.75), null}},
		{"3", []ProjectMetadataCell{metadataText("000003"), null, null, null, null, null, siviReal(130), null}},
		{"4", []ProjectMetadataCell{metadataText("000004"), null, null, null, null, siviReal(50), null, null}},
		{"5", []ProjectMetadataCell{metadataText("NOADMIN"), null, null, null, null, siviReal(50), siviReal(130), null}},
	}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "VARCHAR"}}, Rows: []ProjectMetadataRow{}}
	for i := 0; i < 4; i++ {
		admin.Rows = append(admin.Rows, ProjectMetadataRow{strconv.Itoa(i + 1), []ProjectMetadataCell{cloneSiteUnitCell(env.Rows[i].Cells[0])}})
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "VARCHAR"}, {Name: "SiteUnit", DeclaredType: "VARCHAR"}},
		Rows: []ProjectMetadataRow{
			{"1", []ProjectMetadataCell{metadataText("000001"), null}},
			{"2", []ProjectMetadataCell{metadataText("000001"), metadataText("")}},
			{"3", []ProjectMetadataCell{metadataText("000001"), metadataText("different classification")}},
			{"4", []ProjectMetadataCell{null, metadataText("unused")}},
		}}
	return env, admin, su
}

func TestPlotLocationsSourceEightFieldsSignNullsScopeAndDetachedValues(t *testing.T) {
	env, admin, su := locationTestTables()
	report, err := planPlotLocations(context.Background(), "Project", "None", env, admin, su)
	if err != nil || len(report.Rows) != 2 || len(report.Fields) != 8 {
		t.Fatal("source project-only scope", report, err)
	}
	labels := []string{}
	for _, field := range report.Fields {
		labels = append(labels, field.Label)
	}
	if !reflect.DeepEqual(labels, []string{"Plot Number", "Zone", "Subzone", "Site Series", "Accuracy", "Latitude", "Longitude", "Elevation"}) {
		t.Fatal("source worksheet order/labels changed", labels)
	}
	first, second := report.Rows[0], report.Rows[1]
	if first.EnvRowID != "-9007199254740993" || first.AdminRowID != "1" || len(first.MembershipRowIDs) != 0 ||
		*first.Values[0].Text != "000001" || *first.Values[1].Text != "  Zone  " || *first.Values[2].Text != "" ||
		*first.Values[3].Text != "01" || first.Values[4].Storage != "null" || *first.Values[7].Integer != "9007199254740993" ||
		*first.StoredLongitude.Real != 123.75 || *first.Values[6].Real != -123.75 ||
		*second.StoredLongitude.Real != -123.75 || *second.Values[6].Real != 123.75 || !math.Signbit(*second.Values[5].Real) {
		t.Fatal("source longitude negation, leading zeros, NULL/empty or physical precision changed", report)
	}
	*report.Rows[0].Values[1].Text = "caller"
	*report.Rows[0].StoredLongitude.Real = 1
	report.Fields[0].Label = "caller"
	if *env.Rows[0].Cells[1].Text != "  Zone  " || *env.Rows[0].Cells[6].Real != 123.75 {
		t.Fatal("report shares original cell pointers")
	}
	selected, err := planPlotLocations(context.Background(), "Project", "Subset", env, admin, su)
	if err != nil || len(selected.Rows) != 1 ||
		!reflect.DeepEqual(selected.Rows[0].MembershipRowIDs, []string{"1", "2", "3"}) {
		t.Fatal("selected membership duplicates/classification NULL/empty altered one report row", selected, err)
	}
}

func TestPlotLocationsNumericNegationPreservesHistoricalValuesAndRejectsCoercion(t *testing.T) {
	for _, test := range []struct {
		cell ProjectMetadataCell
		want string
	}{
		{siviReal(0), "-0"}, {siviReal(math.Copysign(0, -1)), "0"},
		{siviReal(-200), "200"}, {metadataInteger("9007199254740993"), "-9007199254740993"},
		{metadataInteger("-9223372036854775807"), "9223372036854775807"},
	} {
		got, err := negateLocationLongitude(test.cell)
		if err != nil {
			t.Fatal(err)
		}
		actual := ""
		if got.Storage == "integer" {
			actual = *got.Integer
		} else {
			actual = strconv.FormatFloat(*got.Real, 'g', -1, 64)
		}
		if actual != test.want {
			t.Fatal("numeric value repaired/rounded", test, got)
		}
	}
	for _, cell := range []ProjectMetadataCell{metadataText("123"), {Storage: "blob", BlobHex: qualityString("00")},
		metadataInteger("-9223372036854775808"), siviReal(math.NaN()), siviReal(math.Inf(1))} {
		if _, err := negateLocationLongitude(cell); err == nil {
			t.Fatal("unsupported or overflowing coordinate coerced", cell)
		}
	}
	env, admin, su := locationTestTables()
	env.Rows[1].Cells[5] = siviReal(100)
	env.Rows[1].Cells[6] = siviReal(-200)
	got, err := planPlotLocations(context.Background(), "Project", "None", env, admin, su)
	if err != nil || *got.Rows[1].Values[5].Real != 100 || *got.Rows[1].Values[6].Real != 200 {
		t.Fatal("historical ranges silently clamped/dropped", got, err)
	}
	zero, err := negateLocationLongitude(siviReal(0))
	if err != nil {
		t.Fatal(err)
	}
	if !math.Signbit(*zero.Real) {
		t.Fatal("source zero sign was repaired")
	}
}

func TestPlotLocationsMalformedAmbiguousAndCancelledInputYieldsNoPartialReport(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable)
	}{
		{"missing field", func(env, _, _ *ProjectMetadataTable) { env.Columns[7].Name = "not Elevation" }},
		{"partial row", func(env, _, _ *ProjectMetadataTable) { env.Rows[1].Cells = env.Rows[1].Cells[:7] }},
		{"duplicate row ID", func(env, _, _ *ProjectMetadataTable) { env.Rows[1].RowID = env.Rows[0].RowID }},
		{"duplicate parent", func(_, admin, _ *ProjectMetadataTable) {
			admin.Rows = append(admin.Rows, ProjectMetadataRow{"99", []ProjectMetadataCell{metadataText("000001")}})
		}},
		{"duplicate env", func(env, _, _ *ProjectMetadataTable) { env.Rows[1].Cells[0] = metadataText("000001") }},
		{"numeric identity", func(env, _, _ *ProjectMetadataTable) { env.Rows[0].Cells[0] = metadataInteger("1") }},
		{"late text longitude", func(env, _, _ *ProjectMetadataTable) { env.Rows[1].Cells[6] = metadataText("123") }},
		{"late text latitude", func(env, _, _ *ProjectMetadataTable) { env.Rows[1].Cells[5] = metadataText("49") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, admin, su := locationTestTables()
			test.mutate(&env, &admin, &su)
			got, err := planPlotLocations(context.Background(), "Project", "None", env, admin, su)
			if got != nil || err == nil {
				t.Fatal("unsupported inputs returned partial locations", got, err)
			}
		})
	}
	env, admin, su := locationTestTables()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planPlotLocations(ctx, "Project", "None", env, admin, su); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled projection returned rows", got, err)
	}
	if got, err := planPlotLocations(context.Background(), "bad name", "None", env, admin, su); got != nil || err == nil || !strings.Contains(err.Error(), "literal") {
		t.Fatal("implicit family identity accepted", got, err)
	}
}
