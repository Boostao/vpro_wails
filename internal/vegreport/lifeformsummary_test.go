package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func lifeformTable(names []string, rows ...[]vprocore.ProjectMetadataCell) vprocore.ProjectMetadataTable {
	table := vprocore.ProjectMetadataTable{Rows: []vprocore.ProjectMetadataRow{}}
	for _, name := range names {
		table.Columns = append(table.Columns, vprocore.ProjectMetadataColumn{Name: name})
	}
	for i, cells := range rows {
		table.Rows = append(table.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(i + 1), Cells: cells})
	}
	return table
}

func lifeformFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	names := []string{"PlotNumber", "Species"}
	for i := 1; i <= 10; i++ {
		names = append(names, fmt.Sprintf("Cover%d", i))
	}
	veg := lifeformTable(names)
	add := func(plot, species string, cover vprocore.ProjectMetadataCell) {
		cells := []vprocore.ProjectMetadataCell{vprotest.MetadataText(plot), vprotest.MetadataText(species), cover}
		for len(cells) < len(names) {
			cells = append(cells, vprocore.ProjectMetadataCell{Storage: "null"})
		}
		veg.Rows = append(veg.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(veg.Rows) + 1), Cells: cells})
	}
	add("P", "A", vprotest.MetadataInteger("10"))
	add("P", "A", vprotest.MetadataInteger("20"))
	add("P", "B", vprocore.ProjectMetadataCell{Storage: "null"})
	su := lifeformTable([]string{"PlotNumber", "SiteUnit"},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("U")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("U")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("V")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("orphan"), vprotest.MetadataText("Empty")},
		[]vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("Zero")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), {Storage: "null"}})
	spec := lifeformTable([]string{"Code", "Lifeform", "Codetype"},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("1"), vprotest.MetadataText("U")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("1"), vprotest.MetadataText("u")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("B"), vprotest.MetadataInteger("2"), vprotest.MetadataText("")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("3"), vprotest.MetadataText("S")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("4"), vprotest.MetadataText("s")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("5"), {Storage: "null"}},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("a"), vprotest.MetadataInteger("6"), vprotest.MetadataText("U")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("13"), vprotest.MetadataText("U")},
		[]vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger("14"), vprotest.MetadataText("U")})
	catalogue := lifeformTable([]string{"Lifeform", "LifeformTXT", "Definition", "ShortName"})
	for _, form := range []int{2, 1, 0, 12, 13, 14, -1} {
		catalogue.Rows = append(catalogue.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(catalogue.Rows) + 1),
			Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataInteger(fmt.Sprint(form)), vprotest.MetadataText(fmt.Sprint(form)),
				{Storage: "null"}, vprotest.MetadataText("")}})
	}
	return veg, su, spec, catalogue
}

func TestLifeformSummaryGlobalEntryMultiplicityCountsAndNullMeans(t *testing.T) {
	veg, su, spec, catalogue := lifeformFixture()
	report, err := planLifeformSummary(context.Background(), "Project", "Selected", veg, su, spec, catalogue)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Entries) != 12 || len(report.Catalogue) != 6 || len(report.Units) != 5 {
		t.Fatal("global insertion/SU/definition fanout lost", report)
	}
	u, v, empty, zero, nullUnit := report.Units[0], report.Units[1], report.Units[2], report.Units[3], report.Units[4]
	if u.NPlots != 2 || u.Occurrences != 24 || u.UniqueSpecies != 2 || v.NPlots != 1 || v.Occurrences != 12 ||
		v.UniqueSpecies != 2 || empty.NPlots != 1 || empty.Occurrences != 0 || zero.NPlots != 0 ||
		nullUnit.NPlots != 1 || nullUnit.Occurrences != 0 {
		t.Fatal("physical denominator or plot-only rejoin changed", report.Units)
	}
	if *u.Rows[1].MeanCover != 1.6 || *v.Rows[1].MeanCover != 1.6 ||
		u.Rows[1].CoverCount != 16 || v.Rows[1].CoverCount != 8 || *u.Rows[1].Presence != .5 ||
		*u.Rows[0].MeanCover != 0 || u.Rows[0].CoverCount != 8 || u.Rows[0].PlotGroups != 1 {
		t.Fatal("source Count/Sum semantics lost", u, v)
	}
	for _, unit := range []LifeformSummaryUnit{empty, zero, nullUnit} {
		for _, row := range unit.Rows {
			if row.MeanCover != nil || row.CoverCount != 0 || row.PlotGroups != 0 ||
				unit.NPlots == 0 && row.Presence != nil || unit.NPlots != 0 && (row.Presence == nil || *row.Presence != 0) {
				t.Fatal("NULL sum/zero denominator silently replaced", unit)
			}
		}
	}
	for _, entry := range report.Entries {
		if entry.Lifeform != 1 && entry.Lifeform != 2 || entry.Species == "a" {
			t.Fatal("SQL NULL, S/s, binary identity or 0..12 inclusion changed", entry)
		}
	}
	if report.Catalogue[0].Lifeform != 2 || report.Catalogue[5].Lifeform != -1 ||
		report.Catalogue[0].Definition.Storage != "null" || *report.Catalogue[0].ShortName.Text != "" {
		t.Fatal("physical catalogue order or NULL/empty metadata lost")
	}
	again, err := planLifeformSummary(context.Background(), "Project", "Selected", veg, su, spec, catalogue)
	if err != nil || !reflect.DeepEqual(again, report) {
		t.Fatal("preview is nondeterministic", err)
	}
	*report.Memberships[0].PlotNumber.Text = "changed"
	if *su.Rows[0].Cells[0].Text != "P" {
		t.Fatal("report aliases source storage")
	}
}

func TestLifeformSummaryEveryInsertedLifeformAndCatalogueOnlyExcludes13(t *testing.T) {
	veg, su, _, _ := lifeformFixture()
	spec := lifeformTable([]string{"Code", "Lifeform", "Codetype"})
	catalogue := lifeformTable([]string{"Lifeform", "LifeformTXT", "Definition", "ShortName"})
	for i := 0; i <= 14; i++ {
		spec.Rows = append(spec.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(i + 1),
			Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataInteger(fmt.Sprint(i)), vprotest.MetadataText("X")}})
		catalogue.Rows = append(catalogue.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(i + 1),
			Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataInteger(fmt.Sprint(i)), vprotest.MetadataText(""), {Storage: "null"}, {Storage: "null"}}})
	}
	report, err := planLifeformSummary(context.Background(), "Project", "Selected", veg, su, spec, catalogue)
	if err != nil || len(report.Entries) != 52 || len(report.Catalogue) != 14 {
		t.Fatal("insert/report code domains conflated", report, err)
	}
	for _, entry := range report.Entries {
		if entry.Lifeform < 0 || entry.Lifeform > 12 {
			t.Fatal(entry)
		}
	}
	if report.Units[0].Rows[13].Lifeform != 14 || report.Units[0].Rows[13].MeanCover != nil {
		t.Fatal("uninserted catalogue lifeform should retain NULL mean")
	}
}

func TestLifeformSummarySingleAssignmentPrecisionAndInvalidHistoricalText(t *testing.T) {
	for _, value := range []float64{0.1, 16777217, math.SmallestNonzeroFloat32 / 2, -16777217} {
		veg, su, spec, catalogue := lifeformFixture()
		veg.Rows = veg.Rows[:1]
		veg.Rows[0].Cells[2] = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
		report, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue)
		if err != nil || report.Entries[0].Cover != float64(float32(value)) {
			t.Fatal("SINGLE assignment was skipped or rounded after aggregation", value, report, err)
		}

	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable){
		func(v, _, _ *vprocore.ProjectMetadataTable) {
			v.Rows[0].Cells[2] = vprotest.MetadataText("invalid number")
		},
		func(v, _, _ *vprocore.ProjectMetadataTable) {
			n := math.MaxFloat64
			v.Rows[0].Cells[2] = vprocore.ProjectMetadataCell{Storage: "real", Real: &n}
		},
		func(v, s, _ *vprocore.ProjectMetadataTable) {
			v.Rows[0].Cells[0] = vprotest.MetadataText("123456789")
			s.Rows[0].Cells[0] = vprotest.MetadataText("123456789")
		},
		func(v, _, r *vprocore.ProjectMetadataTable) {
			v.Rows[0].Cells[1] = vprotest.MetadataText("123456789")
			r.Rows[0].Cells[0] = vprotest.MetadataText("123456789")
		},
		func(_, s, _ *vprocore.ProjectMetadataTable) {
			s.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("😀", 11))
		},
		func(v, _, _ *vprocore.ProjectMetadataTable) {
			v.Rows[0].Cells[1] = vprotest.MetadataText(string([]byte{0xff}))
		},
	} {
		veg, su, spec, catalogue := lifeformFixture()
		mutate(&veg, &su, &spec)
		if got, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue); err == nil ||
			!reflect.DeepEqual(got, LifeformSummaryReport{}) {
			t.Fatal("invalid historical input was repaired", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	veg, su, spec, catalogue := lifeformFixture()
	if _, err := planLifeformSummary(ctx, "P", "S", veg, su, spec, catalogue); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	catalogue.Rows = append(catalogue.Rows, catalogue.Rows[0])
	if _, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue); err == nil {
		t.Fatal("duplicate physical catalogue PK silently grouped")
	}
}

func TestLifeformSummaryIgnoresUnrelatedCoversAndUsesExplicitCasePolicy(t *testing.T) {
	veg, su, spec, catalogue := lifeformFixture()
	base, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Cover5a", "Cover5b", "Cover5c", "TotalA", "TotalB"} {
		veg.Columns = append(veg.Columns, vprocore.ProjectMetadataColumn{Name: name})
		for i := range veg.Rows {
			veg.Rows[i].Cells = append(veg.Rows[i].Cells, vprotest.MetadataText("unrelated invalid historical cover"))
		}
	}
	got, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue)
	if err != nil || !reflect.DeepEqual(got, base) {
		t.Fatal("source ten-cover projection inherited strata or layer fields", got, err)
	}
	for _, codeType := range []string{"ſ", " S", "s "} {
		spec.Rows = spec.Rows[:1]
		spec.Rows[0].Cells[2] = vprotest.MetadataText(codeType)
		got, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue)
		if err != nil || len(got.Entries) != 4 {
			t.Fatal("nonliteral CodeType silently normalized or Unicode-folded", codeType, got, err)
		}
	}
}

func TestLifeformSummarySingleRoundingAtBothSourceAssignments(t *testing.T) {
	veg, su, spec, catalogue := lifeformFixture()
	veg.Rows = veg.Rows[:1]
	veg.Rows[0].Cells[2] = vprotest.MetadataInteger("16777217")
	veg.Rows[0].Cells[3] = vprotest.MetadataInteger("1")
	report, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue)
	if err != nil || report.Entries[0].Cover != 16777216 {
		t.Fatal("MAX assignment to copied SINGLE schema must precede Nz sum and final SINGLE assignment", report, err)
	}
	n := math.MaxFloat64
	veg.Rows[0].Cells[2] = vprocore.ProjectMetadataCell{Storage: "real", Real: &n}
	spec.Rows = nil
	if _, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue); err == nil {
		t.Fatal("unmatched species bypassed the earlier copied SINGLE assignment")
	}
}

func TestLifeformSummaryRefusesMissingPhysicalSchemaAndDynamicScope(t *testing.T) {
	for _, which := range []string{"Veg", "SU", "USysAllSpecs", "LifeformCodes"} {
		veg, su, spec, catalogue := lifeformFixture()
		var table *vprocore.ProjectMetadataTable
		switch which {
		case "Veg":
			table = &veg
		case "SU":
			table = &su
		case "USysAllSpecs":
			table = &spec
		case "LifeformCodes":
			table = &catalogue
		}
		table.Columns[0].Name = "WrongPhysicalBinding"
		if got, err := planLifeformSummary(context.Background(), "P", "S", veg, su, spec, catalogue); err == nil ||
			!reflect.DeepEqual(got, LifeformSummaryReport{}) {
			t.Fatal("missing physical schema was guessed", which, got, err)
		}
	}
	veg, su, spec, catalogue := lifeformFixture()
	if got, err := planLifeformSummary(context.Background(), "P", "USysSuTableDynamic", veg, su, spec, catalogue); err == nil ||
		!strings.Contains(err.Error(), "hierarchy/dynamic-break scope is unavailable") ||
		!reflect.DeepEqual(got, LifeformSummaryReport{}) {
		t.Fatal("hierarchy scope used normal-SU planner", got, err)
	}
}
