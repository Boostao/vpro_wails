package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func lifeformPreparedFixture() VegetationReportPreparation {
	result := VegetationReportPreparation{Project: "Project", SU: "Selected", CoverColumns: longVegetationCoverColumns(),
		Memberships: []VegetationReportMembership{
			{RowID: "1", PlotNumber: metadataText("P"), SiteUnit: metadataText("U")},
			{RowID: "2", PlotNumber: metadataText("P"), SiteUnit: metadataText("U")},
			{RowID: "3", PlotNumber: metadataText("P"), SiteUnit: ProjectMetadataCell{Storage: "null"}},
			{RowID: "4", PlotNumber: metadataText("Q"), SiteUnit: ProjectMetadataCell{Storage: "null"}},
		}}
	covers := make([]ProjectMetadataCell, len(result.CoverColumns))
	for i := range covers {
		covers[i] = ProjectMetadataCell{Storage: "null"}
	}
	result.ReducedRows = []VegetationReportReducedRow{{PlotNumber: "P", Species: metadataText("A"), Covers: covers}}
	return result
}

func lifeformSetCover(prepared *VegetationReportPreparation, row int, name string, cell ProjectMetadataCell) {
	for i, column := range prepared.CoverColumns {
		if name == column {
			prepared.ReducedRows[row].Covers[i] = cell
			return
		}
	}
	panic("unknown fixture cover")
}

func lifeformMasterFixture() ProjectMetadataTable {
	return lifeformReferenceFixture(lifeformReferenceRow("A", "Scientific A", metadataInteger("2"), metadataText("English A"), metadataText("U")))
}

func lifeformLayerFixture(codes ...string) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}}}
	for i, code := range codes {
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: []ProjectMetadataCell{metadataText(code)}})
	}
	return result
}

func lifeformUnionFixture(t *testing.T, master, personal ProjectMetadataTable) ProjectMetadataTable {
	t.Helper()
	result, err := prepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil {
		t.Fatal(err)
	}
	return result.Table
}

func TestLongVegetationLifeformCoverSelectionAndThreeSingleAssignments(t *testing.T) {
	master := lifeformMasterFixture()
	for _, name := range longVegetationCoverColumns() {
		prepared := lifeformPreparedFixture()
		lifeformSetCover(&prepared, 0, name, strataReal(3))
		result, err := prepareLongVegetationLifeforms(context.Background(), prepared, master)
		if err != nil || len(result.Observations) != 1 {
			t.Fatal(result, err)
		}
		want := 3.0
		if name == "Cover10" || name == "TotalA" || name == "TotalB" {
			want = 0
		}
		if *result.Observations[0].Cover.Real != want {
			t.Fatal("active source cover inclusion differs", name, result)
		}
	}
	for _, test := range []struct {
		a, b, want float64
	}{
		{16777217, -16777216, 0},   // First SINGLE assignment before exact addition.
		{1, math.Ldexp(1, -24), 1}, // SetTo99's parameter rounds a halfway sum.
		{99.9, 0, float64(float32(99.9))},
		{float64(math.Nextafter32(float32(99.9), 0)), 0, float64(math.Nextafter32(float32(99.9), 0))},
		{99.90001, 0, float64(float32(99.9))},
		{-1000, 0, -1000}, // No invented lower cap.
	} {
		prepared := lifeformPreparedFixture()
		lifeformSetCover(&prepared, 0, "Cover1", strataReal(test.a))
		lifeformSetCover(&prepared, 0, "Cover2", strataReal(test.b))
		result, err := prepareLongVegetationLifeforms(context.Background(), prepared, master)
		if err != nil || *result.Observations[0].Cover.Real != test.want {
			t.Fatal("SINGLE or 99.9 boundary differs", test, result, err)
		}
	}
	// Distinct five-column rows with the same insertion lifeform double the
	// Variant cap value before the last SINGLE assignment.
	master.Rows = append(master.Rows, ProjectMetadataRow{RowID: "2",
		Cells: lifeformReferenceRow("A", "Other", metadataInteger("2"), metadataText("Other"), metadataText("U"))})
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(120))
	result, err := prepareLongVegetationLifeforms(context.Background(), prepared, master)
	if err != nil || *result.Observations[0].Cover.Real != float64(float32(199.8)) {
		t.Fatal("Variant cap or final grouped SINGLE assignment lost", result, err)
	}
}

func TestLongVegetationLifeformSelectedScopeMaxBeforeConversion(t *testing.T) {
	veg := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}}
	for _, name := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: name})
	}
	for i, plot := range []string{"P", "P", "unselected"} {
		cells := []ProjectMetadataCell{metadataText(plot), metadataText("A")}
		for _, name := range longVegetationCoverColumns() {
			cell := ProjectMetadataCell{Storage: "null"}
			if i == 0 && name == "Cover1" {
				cell = strataReal(6)
			}
			if i == 1 && name == "Cover2" {
				cell = strataReal(7)
			}
			if i == 2 && name == "Cover1" {
				cell = strataReal(90)
			}
			cells = append(cells, cell)
		}
		veg.Rows = append(veg.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P"), metadataText("U")}}}}
	prepared, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su,
		ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "LayerText"}}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := planLongVegetationLifeforms(context.Background(), prepared, lifeformMasterFixture(), lifeformMasterFixture(),
		lifeformLayerFixture("02"), layerTestOptions())
	if err != nil || len(report.Units) != 1 || len(report.Units[0].Rows) != 1 ||
		*report.Units[0].Rows[0].MeanCover != 13 || *report.Units[0].Rows[0].Presence != 1 {
		t.Fatal("scope or per-cover MAX before conversion differs", report, err)
	}
}

func TestLongVegetationLifeformFormatterAndMissingDefinitions(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	for _, test := range []struct {
		cell ProjectMetadataCell
		want string
	}{{metadataInteger("0"), "00"}, {metadataInteger("2"), "02"}, {metadataInteger("123"), "123"},
		{metadataInteger("-2"), "-02"}, {metadataInteger("-32768"), "-32768"}} {
		layer, err := vegetationLifeformInsertionLayer(test.cell)
		if err != nil || layer.Text == nil || *layer.Text != test.want {
			t.Fatal(test, layer, err)
		}
	}
	layer, err := vegetationLifeformInsertionLayer(null)
	if err != nil || layer.Storage != "null" {
		t.Fatal("NULL formatter invented '?' key", layer, err)
	}
	layer, err = vegetationLifeformNull2Question(metadataText(""))
	if err != nil || *layer.Text != "?" {
		t.Fatal("empty comparison lost", layer, err)
	}
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(1))
	result, err := prepareLongVegetationLifeforms(context.Background(), prepared, lifeformReferenceFixture())
	if err != nil || result.Observations[0].Layer.Storage != "null" || *result.Observations[0].Cover.Real != 1 {
		t.Fatal("LEFT JOIN missing definition dropped observation", result, err)
	}
}

func TestLongVegetationLifeformSeparateMasterLayerAndPhysicalFanout(t *testing.T) {
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(4))
	master := lifeformMasterFixture()
	personal := lifeformReferenceFixture(lifeformReferenceRow("A", "Personal A", metadataInteger("3"), metadataText("Personal"), metadataText("U")))
	union := lifeformUnionFixture(t, master, personal)
	options := layerTestOptions()
	options.ConstantSpeciesList = false
	options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -100
	report, err := planLongVegetationLifeforms(context.Background(), prepared, master, union, lifeformLayerFixture("02", "02", "03"), options)
	if err != nil || len(report.Units) != 2 {
		t.Fatal(report, err)
	}
	for _, unit := range report.Units {
		if unit.NumPlots != 2 || len(unit.Rows) != 1 {
			t.Fatal("physical duplicate/unassigned denominator lost", unit)
		}
		row := unit.Rows[0]
		wantMean, wantPivot := 6.0, 12.0
		if unit.Code.Text != nil {
			wantMean, wantPivot = 12, 24
		}
		if row.Layer.Integer == nil || *row.Layer.Integer != "2" || *row.Species.Text != "Scientific A" ||
			*row.MeanCover != wantMean || *row.Presence != 0.5 || *row.Plots[0].Cover != wantPivot {
			t.Fatal("insertion lifeform confused with later master or LayerCode fanout", row)
		}
	}
}

func TestLongVegetationLifeformEnglishConstantFanoutNullKeysAndOrder(t *testing.T) {
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(4))
	master := lifeformMasterFixture()
	master.Rows = append(master.Rows,
		ProjectMetadataRow{RowID: "2", Cells: lifeformReferenceRow("A", "Scientific A", metadataInteger("2"), metadataText("Other English"), metadataText("X"))},
		ProjectMetadataRow{RowID: "3", Cells: lifeformReferenceRow("A", "Scientific A", metadataInteger("10"), metadataText("English A"), metadataText("U"))},
		ProjectMetadataRow{RowID: "4", Cells: lifeformReferenceRow("A", "Scientific A", ProjectMetadataCell{Storage: "null"}, metadataText("Null lifeform"), metadataText("U"))})
	union := lifeformUnionFixture(t, master, lifeformReferenceFixture())
	for _, order := range []string{"species", "presence"} {
		options := layerTestOptions()
		options.Order = order
		report, err := planLongVegetationLifeforms(context.Background(), prepared, master, union, lifeformLayerFixture(), options)
		if err != nil {
			t.Fatal(err)
		}
		for _, unit := range report.Units {
			if len(unit.Rows) != 6 || unit.Rows[0].Layer.Storage != "null" || unit.Rows[0].Presence != nil ||
				*unit.Rows[1].Layer.Integer != "2" || *unit.Rows[5].Layer.Integer != "10" {
				t.Fatal("constant nullable keys, English fanout or numeric 13/23 ordering lost", unit)
			}
			names := map[string]bool{}
			for _, row := range unit.Rows[1:5] {
				names[*row.EnglishName.Text+"/"+*row.MatchedName.Text] = true
			}
			if len(names) != 4 {
				t.Fatal("English names incorrectly matched in constant join", names)
			}
		}
	}
}

func TestLongVegetationLifeformErrorsAndCancellationAreAtomic(t *testing.T) {
	master := lifeformMasterFixture()
	for _, bad := range []ProjectMetadataCell{metadataText("12"), strataReal(math.MaxFloat64), {Storage: "null", Text: new(string)}} {
		prepared := lifeformPreparedFixture()
		lifeformSetCover(&prepared, 0, "Cover1", bad)
		report, err := planLongVegetationLifeforms(context.Background(), prepared, master, master, lifeformLayerFixture(), layerTestOptions())
		if err == nil || !reflect.DeepEqual(report, vegetationLayerReport{}) {
			t.Fatal("invalid cover produced partial output", report, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := planLongVegetationLifeforms(ctx, lifeformPreparedFixture(), master, master, lifeformLayerFixture(), layerTestOptions())
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal(report, err)
	}
	for remaining := 1; remaining <= 25; remaining++ {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		report, err := planLongVegetationLifeforms(ctx, lifeformPreparedFixture(), master, master, lifeformLayerFixture(), layerTestOptions())
		if err != nil && (!errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, vegetationLayerReport{})) {
			t.Fatal("mid-plan cancellation returned partial output", remaining, report, err)
		}
	}
	for _, form := range []ProjectMetadataCell{metadataText("2"), strataReal(2), metadataInteger("32768"), metadataInteger("1000")} {
		bad := lifeformReferenceFixture(lifeformReferenceRow("A", "A", form, metadataText("A"), metadataText("U")))
		report, err := planLongVegetationLifeforms(context.Background(), lifeformPreparedFixture(), bad, bad, lifeformLayerFixture(), layerTestOptions())
		if err == nil || !reflect.DeepEqual(report, vegetationLayerReport{}) {
			t.Fatal("invalid source INTEGER or insertion TEXT(3) accepted", form, report, err)
		}
	}
}

func TestLongVegetationLifeformMissingPersonalMasterAndNoInventedConstantMatch(t *testing.T) {
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(7))
	personal := lifeformReferenceFixture(lifeformReferenceRow("A", "Personal", metadataInteger("3"), metadataText("Personal name"), metadataText("U")))
	master := lifeformReferenceFixture()
	union := lifeformUnionFixture(t, master, personal)
	options := layerTestOptions()
	report, err := planLongVegetationLifeforms(context.Background(), prepared, master, union, lifeformLayerFixture("03", "03"), options)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		if len(unit.Rows) != 1 || unit.Rows[0].Layer.Storage != "null" || *unit.Rows[0].Species.Text != "A" ||
			unit.Rows[0].MeanCover != nil || unit.Rows[0].EnglishName.Storage != "null" {
			t.Fatal("personal insertion definition leaked into separate master join or NULL constant matched", unit)
		}
	}
	options.ConstantSpeciesList = false
	options.MeanCoverGreaterThan, options.PresenceGreaterThan = -1, -1
	report, err = planLongVegetationLifeforms(context.Background(), prepared, master, union, lifeformLayerFixture("03", "03"), options)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		want := 7.0
		if unit.Code.Text != nil {
			want = 14
		}
		if *unit.Rows[0].MeanCover != want || unit.Rows[0].Layer.Storage != "null" {
			t.Fatal("missing master LEFT JOIN or LayerCode multiplicity lost", unit)
		}
	}
	// A NULL CodeType master is excluded from USysAllSpecies but is retained
	// by the later report's (<>'S' OR Is Null) predicate.
	master = lifeformReferenceFixture(lifeformReferenceRow("A", "", metadataInteger("4"), metadataText(""), ProjectMetadataCell{Storage: "null"}))
	union = lifeformUnionFixture(t, master, lifeformReferenceFixture())
	report, err = planLongVegetationLifeforms(context.Background(), prepared, master, union, lifeformLayerFixture("04", "04"), options)
	if err != nil || *report.Units[0].Rows[0].Layer.Integer != "4" || *report.Units[0].Rows[0].Species.Text != "" ||
		*report.Units[0].Rows[0].MeanCover != 3.5 {
		t.Fatal("UNION NULL exclusion vs later master NULL retention or NULL insertion Layer equality differs", report, err)
	}
}

func TestLongVegetationLifeformSpeciesAndPresenceOrdering13And23(t *testing.T) {
	prepared := lifeformPreparedFixture()
	lifeformSetCover(&prepared, 0, "Cover1", strataReal(5))
	prepared.Memberships = []VegetationReportMembership{
		{RowID: "1", PlotNumber: metadataText("P"), SiteUnit: metadataText("U")},
		{RowID: "2", PlotNumber: metadataText("Q"), SiteUnit: metadataText("U")},
		{RowID: "3", PlotNumber: metadataText("R"), SiteUnit: metadataText("U")},
	}
	for _, plot := range []string{"Q", "R"} {
		row := prepared.ReducedRows[0]
		row.PlotNumber = plot
		if plot == "R" {
			row.Species = metadataText("B")
		}
		prepared.ReducedRows = append(prepared.ReducedRows, row)
	}
	master := lifeformReferenceFixture(
		lifeformReferenceRow("A", "Z name", metadataInteger("2"), metadataText(""), metadataText("U")),
		lifeformReferenceRow("B", "A name", metadataInteger("2"), metadataText(""), metadataText("U")))
	for _, test := range []struct{ order, first string }{{"species", "A name"}, {"presence", "Z name"}} {
		options := layerTestOptions()
		options.ConstantSpeciesList, options.Order = false, test.order
		options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -1
		report, err := planLongVegetationLifeforms(context.Background(), prepared, master, master, lifeformLayerFixture("02"), options)
		if err != nil || len(report.Units) != 1 || len(report.Units[0].Rows) != 2 ||
			*report.Units[0].Rows[0].Species.Text != test.first {
			t.Fatal("13/23 source ordering differs", test, report, err)
		}
		for _, row := range report.Units[0].Rows {
			want := 1.0 / 3
			if *row.Species.Text == "Z name" {
				want = 2.0 / 3
			}
			if *row.Presence != want {
				t.Fatal("source grouped plot presence differs", row)
			}
		}
	}
}
