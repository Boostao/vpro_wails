package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func projectEntryFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	veg, su, layers := vegetationReportFixture(t)
	su.Rows = su.Rows[:7]
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Unit", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "101", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("U")}},
			{RowID: "102", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("V")}},
			{RowID: "103", Cells: []ProjectMetadataCell{metadataText("P2"), ProjectMetadataCell{Storage: "null"}}},
			{RowID: "104", Cells: []ProjectMetadataCell{metadataText("P3"), metadataText("orphan")}},
			{RowID: "105", Cells: []ProjectMetadataCell{metadataText(""), metadataText("")}},
			{RowID: "106", Cells: []ProjectMetadataCell{ProjectMetadataCell{Storage: "null"}, metadataText("null plot")}},
		}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "201", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "202", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "203", Cells: []ProjectMetadataCell{metadataText("P2")}},
			{RowID: "204", Cells: []ProjectMetadataCell{metadataText("P3")}},
			{RowID: "205", Cells: []ProjectMetadataCell{metadataText("")}},
		}}
	return veg, su, layers, env, admin
}

func TestProjectEntryPreparationIndependentMAX(t *testing.T) {
	veg, su, layers, env, admin := projectEntryFixture(t)
	before, err := json.Marshal([]ProjectMetadataTable{veg, su, layers, env, admin})
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantUnits := []vegetationProjectSelectedUnitOccurrence{
		{"101", "201", metadataText("P1"), metadataText("U")},
		{"101", "202", metadataText("P1"), metadataText("U")},
		{"102", "201", metadataText("P1"), metadataText("V")},
		{"102", "202", metadataText("P1"), metadataText("V")},
		{"103", "203", metadataText("P2"), ProjectMetadataCell{Storage: "null"}},
		{"104", "204", metadataText("P3"), metadataText("orphan")},
		{"105", "205", metadataText(""), metadataText("")},
	}
	if got.Project != "Project" || got.SourceSU != "Selected" || !reflect.DeepEqual(got.SelectedUnits, wantUnits) ||
		len(got.Entries.ReducedRows) != 5 || len(got.Entries.Observations) != 14 || len(got.Entries.Layers) != 13 {
		t.Fatal("project denominator tuples must not multiply entry MAX/layer insertion", got)
	}
	type reducedWitness struct {
		plot    string
		species ProjectMetadataCell
		ids     []string
		covers  map[int]ProjectMetadataCell
	}
	real := func(value float64) ProjectMetadataCell { return ProjectMetadataCell{Storage: "real", Real: &value} }
	wantRows := []reducedWitness{
		{"", metadataText(""), []string{"6"}, map[int]ProjectMetadataCell{9: real(-5)}},
		{"P1", ProjectMetadataCell{Storage: "null"}, []string{"7"}, map[int]ProjectMetadataCell{2: real(0)}},
		{"P1", metadataText(""), []string{"9"}, map[int]ProjectMetadataCell{2: real(1)}},
		{"P1", metadataText("A"), []string{"1", "2"}, map[int]ProjectMetadataCell{
			0: metadataInteger("-2"), 1: metadataInteger("0"), 5: real(4), 6: real(0), 7: real(-1),
			10: real(8), 11: real(9), 12: real(10), 13: real(111), 14: real(33)}},
		{"P2", metadataText("A"), []string{"3", "4"}, map[int]ProjectMetadataCell{
			0: metadataInteger("9007199254740993"), 5: real(-2), 7: real(120)}},
	}
	if !reflect.DeepEqual(got.Entries.CoverColumns, []string{"Cover1", "Cover2", "Cover3", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c",
		"Cover6", "Cover7", "Cover8", "Cover9", "Cover10", "TotalA", "TotalB"}) {
		t.Fatal("original cover order differs", got.Entries.CoverColumns)
	}
	for i, want := range wantRows {
		row := got.Entries.ReducedRows[i]
		if row.PlotNumber != want.plot || !reflect.DeepEqual(row.Species, want.species) || !reflect.DeepEqual(row.SourceRowIDs, want.ids) || len(row.Covers) != 15 {
			t.Fatal("independent reduced original row differs", i, row, want)
		}
		for col, actual := range row.Covers {
			expected := ProjectMetadataCell{Storage: "null"}
			if value, ok := want.covers[col]; ok {
				expected = value
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatal("independent MAX/storage/NULL cover differs", i, col, actual, expected)
			}
		}
	}
	after, err := json.Marshal([]ProjectMetadataTable{veg, su, layers, env, admin})
	if err != nil || string(before) != string(after) {
		t.Fatal("entry projection changed inputs", err)
	}
	*got.Entries.ReducedRows[3].Species.Text = "changed"
	*got.Entries.ReducedRows[3].Covers[0].Integer = "999"
	got.Entries.ReducedRows[3].SourceRowIDs[0] = "changed"
	*got.SelectedUnits[0].PlotNumber.Text = "changed"
	got.Entries.CoverColumns[0] = "changed"
	got.Entries.Layers[0].Layer = "changed"
	for _, row := range got.Entries.Observations {
		if row.PlotNumber == "P1" && row.Species.Text != nil && *row.Species.Text == "A" && row.Layer == "1" {
			if *row.Cover.Integer != "-2" {
				t.Fatal("observations alias reduced cover")
			}
		}
	}
	after, err = json.Marshal([]ProjectMetadataTable{veg, su, layers, env, admin})
	if err != nil || string(before) != string(after) || *got.SelectedUnits[1].PlotNumber.Text != "P1" {
		t.Fatal("entries alias physical inputs or original tuple peers", err)
	}
}

func TestProjectEntryQualityScopesReductionWithoutReweighting(t *testing.T) {
	veg, su, layers, env, admin := projectEntryFixture(t)
	q := vegetationQualitySelection{Occurrences: []vegetationQualityOccurrence{
		{Membership: VegetationReportMembership{RowID: "1", PlotNumber: metadataText("P1"), SiteUnit: metadataText("U")},
			EnvRowID: "101", AdminRowID: "201"},
	}}
	q.Occurrences = append(q.Occurrences, q.Occurrences[0], q.Occurrences[0])
	got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, &q)
	if err != nil || len(got.SelectedUnits) != 4 || len(got.Entries.ReducedRows) != 3 || len(got.Entries.Observations) != 10 {
		t.Fatal("quality existence must scope entries once, not multiply original MAX", got, err)
	}
	for _, row := range got.Entries.ReducedRows {
		if row.PlotNumber != "P1" {
			t.Fatal("outside qualified plot entered reduction", row)
		}
	}
	q.Occurrences = nil
	got, err = prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, &q)
	if err != nil || got.SelectedUnits == nil || len(got.SelectedUnits) != 0 ||
		got.Entries.ReducedRows == nil || len(got.Entries.ReducedRows) != 0 || got.Entries.Observations == nil || len(got.Entries.Observations) != 0 {
		t.Fatal("empty quality must not widen scope or lose explicit empty shape", got, err)
	}
}

func TestProjectEntryStorageTieAndLayerMultiplicity(t *testing.T) {
	var first vegetationReportEntries
	for reverse := 0; reverse < 2; reverse++ {
		veg, su, layers, env, admin := projectEntryFixture(t)
		zero, negativeZero := 0.0, math.Copysign(0, -1)
		veg.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &negativeZero}
		veg.Rows[1].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &zero}
		if reverse == 1 {
			veg.Rows[0], veg.Rows[1] = veg.Rows[1], veg.Rows[0]
		}
		layers.Rows = append(layers.Rows, ProjectMetadataRow{RowID: "15", Cells: []ProjectMetadataCell{metadataText("5a")}})
		got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, nil)
		if err != nil || len(got.Entries.Observations) != 16 || len(got.Entries.Layers) != 14 {
			t.Fatal("original duplicate LayerCode rows collapsed", got, err)
		}
		cell := got.Entries.ReducedRows[3].Covers[0]
		if cell.Storage != "real" || *cell.Real != 0 || math.Signbit(*cell.Real) {
			t.Fatal("binary64 signed-zero tie changed", cell)
		}
		if reverse == 0 {
			first = got.Entries
		} else if !reflect.DeepEqual(first, got.Entries) {
			t.Fatal("original physical row ordering changed MAX output", first, got.Entries)
		}
	}
}

func TestProjectEntryPreparationRefusalsAndCancellation(t *testing.T) {
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable){
		func(v, s, l *ProjectMetadataTable) { v.Columns = v.Columns[:2] },
		func(v, s, l *ProjectMetadataTable) { v.Columns[2].Name = v.Columns[3].Name },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].RowID = "01" },
		func(v, s, l *ProjectMetadataTable) { v.Rows[1].RowID = "1" },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells = nil },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[1] = metadataInteger("7") },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataText("bad") },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataInteger("9223372036854775808") },
		func(v, s, l *ProjectMetadataTable) { l.Rows[0].Cells[0] = metadataText("A") },
		func(v, s, l *ProjectMetadataTable) { l.Rows[0].Cells = nil },
		func(v, s, l *ProjectMetadataTable) { s.Columns = nil },
	} {
		veg, su, layers, env, admin := projectEntryFixture(t)
		mutate(&veg, &su, &layers)
		got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, nil)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectPreparation{}) {
			t.Fatal("invalid project entry inputs produced partial success", got, err)
		}
	}
	veg, su, layers, env, admin := projectEntryFixture(t)
	for _, name := range []string{"", "\x00", "\xff"} {
		if got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", name, veg, layers, env, admin, su, []string{"Unit"}, nil); err == nil || !reflect.DeepEqual(got, vegetationProjectPreparation{}) {
			t.Fatal("invalid original SU selection accepted", name, got, err)
		}

	}
	for _, name := range []string{"", "\x00", "\xff"} {
		if got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), name, "Selected", veg, layers, env, admin, su, []string{"Unit"}, nil); err == nil || !reflect.DeepEqual(got, vegetationProjectPreparation{}) {
			t.Fatal("invalid project identity accepted", name, got, err)
		}
	}
	run := func(ctx context.Context) (bool, error) {
		got, err := prepareLongVegetationProjectSelectedUnits(ctx, "Project", "Selected", veg, layers, env, admin, su, []string{"Unit"}, nil)
		return reflect.DeepEqual(got, vegetationProjectPreparation{}), err
	}
	if empty, err := run(nil); err == nil || !empty {
		t.Fatal("nil context accepted", empty, err)
	}
	probe := &projectUnitCancellationContext{Context: context.Background()}
	if empty, err := run(probe); err != nil || empty {
		t.Fatal(probe, err)
	}
	for step := 1; step <= probe.calls; step++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: step}
		if empty, err := run(ctx); !errors.Is(err, context.Canceled) || !empty {
			t.Fatal("mid-projection/reduction cancellation returned partial entries", step, empty, err)
		}
	}
}

func TestProjectEntryNoSUCallerUsesMAX(t *testing.T) {
	veg, _, layers, env, admin := projectEntryFixture(t)
	got, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "None",
		veg, layers, env, admin, ProjectMetadataTable{}, []string{"Unit"}, nil)
	if err != nil || got.SourceSU != "None" || len(got.SelectedUnits) != 7 || got.NoneUnits != nil ||
		len(got.Entries.ReducedRows) != 5 || len(got.Entries.Observations) != 14 {
		t.Fatal("actual caller passes TempReportVeg, not raw vegetation or fabricated SU", got, err)
	}
	row := got.Entries.ReducedRows[3]
	if row.PlotNumber != "P1" || *row.Species.Text != "A" || !reflect.DeepEqual(row.SourceRowIDs, []string{"1", "2"}) ||
		*row.Covers[0].Integer != "-2" || *row.Covers[5].Real != 4 {
		t.Fatal("no-SU source2 must still reduce duplicate raw covers by MAX", row)
	}
	_, su, _, _, _ := projectEntryFixture(t)
	for _, test := range []struct {
		su      ProjectMetadataTable
		quality *vegetationQualitySelection
	}{
		{su, nil},
		{ProjectMetadataTable{}, &vegetationQualitySelection{}},
		{su, &vegetationQualitySelection{}},
	} {
		rows, err := prepareLongVegetationProjectSelectedUnits(context.Background(), "Project", "None",
			veg, layers, env, admin, test.su, []string{"Unit"}, test.quality)
		if err == nil || !reflect.DeepEqual(rows, vegetationProjectPreparation{}) {
			t.Fatal("None scope invented physical SU or Env-only QCLV authority", rows, err)
		}
	}
}

func TestProjectEntryNoneOriginalEnvAndMAX(t *testing.T) {
	veg, _, layers, env, _ := projectEntryFixture(t)
	before, err := json.Marshal([]ProjectMetadataTable{veg, layers, env})
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareLongVegetationProjectNoneUnits(context.Background(), "Project", veg, layers, env)
	want := []vegetationProjectNoneUnitOccurrence{
		{"101", metadataText("P1"), metadataText("All plots")},
		{"102", metadataText("P1"), metadataText("All plots")},
		{"103", metadataText("P2"), metadataText("All plots")},
		{"104", metadataText("P3"), metadataText("All plots")},
		{"105", metadataText(""), metadataText("All plots")},
		{"106", ProjectMetadataCell{Storage: "null"}, metadataText("All plots")},
	}
	if err != nil || got.Project != "Project" || got.SourceSU != "None" || got.SelectedUnits != nil ||
		!reflect.DeepEqual(got.NoneUnits, want) || len(got.Entries.ReducedRows) != 5 || len(got.Entries.Observations) != 14 {
		t.Fatal("None keeps each original Env denominator tuple and separate MAX entries", got, err)
	}
	row := got.Entries.ReducedRows[3]
	if !reflect.DeepEqual(row.SourceRowIDs, []string{"1", "2"}) || *row.Covers[0].Integer != "-2" || *row.Covers[5].Real != 4 {
		t.Fatal("None did not use caller-owned TempReportVeg MAX", row)
	}
	after, err := json.Marshal([]ProjectMetadataTable{veg, layers, env})
	if err != nil || string(before) != string(after) {
		t.Fatal("None preparation mutated physical inputs", err)
	}
	*got.NoneUnits[0].PlotNumber.Text = "changed"
	*got.NoneUnits[0].SiteUnit.Text = "changed"
	*got.Entries.ReducedRows[3].Covers[0].Integer = "999"
	after, err = json.Marshal([]ProjectMetadataTable{veg, layers, env})
	if err != nil || string(before) != string(after) || *got.NoneUnits[1].PlotNumber.Text != "P1" ||
		*got.NoneUnits[1].SiteUnit.Text != "All plots" {
		t.Fatal("None result aliases originals or peers", err)
	}
	env.Rows = nil
	got, err = prepareLongVegetationProjectNoneUnits(context.Background(), "Project", veg, layers, env)
	if err != nil || got.NoneUnits == nil || len(got.NoneUnits) != 0 || got.Entries.ReducedRows == nil ||
		len(got.Entries.ReducedRows) != 0 || got.Entries.Observations == nil || len(got.Entries.Observations) != 0 {
		t.Fatal("empty Env scope invented raw/project fallback entries", got, err)
	}
}

func TestProjectEntryNoneCancellationAndMalformedInputs(t *testing.T) {
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable){
		func(v, l, e *ProjectMetadataTable) { e.Rows[1].RowID = "101" },
		func(v, l, e *ProjectMetadataTable) { e.Rows[0].Cells = nil },
		func(v, l, e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataInteger("7") },
		func(v, l, e *ProjectMetadataTable) { v.Columns = v.Columns[:2] },
		func(v, l, e *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataText("bad") },
		func(v, l, e *ProjectMetadataTable) { l.Rows[0].Cells[0] = metadataText("A") },
	} {
		veg, _, layers, env, _ := projectEntryFixture(t)
		mutate(&veg, &layers, &env)
		got, err := prepareLongVegetationProjectNoneUnits(context.Background(), "Project", veg, layers, env)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectPreparation{}) {
			t.Fatal("malformed None preparation produced partial output", got, err)
		}
	}
	for _, selected := range []bool{false, true} {
		veg, _, layers, env, admin := projectEntryFixture(t)
		run := func(ctx context.Context) (bool, error) {
			var got vegetationProjectPreparation
			var err error
			if selected {
				got, err = prepareLongVegetationProjectSelectedUnits(ctx, "Project", "None", veg, layers, env, admin,
					ProjectMetadataTable{}, []string{"Unit"}, nil)
			} else {
				got, err = prepareLongVegetationProjectNoneUnits(ctx, "Project", veg, layers, env)
			}
			return reflect.DeepEqual(got, vegetationProjectPreparation{}), err
		}
		if empty, err := run(nil); err == nil || !empty {
			t.Fatal("nil context accepted", selected, empty, err)
		}
		probe := &projectUnitCancellationContext{Context: context.Background()}
		if empty, err := run(probe); err != nil || empty {
			t.Fatal("successful no-SU probe failed", selected, probe, err)
		}
		for step := 1; step <= probe.calls; step++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: step}
			if empty, err := run(ctx); !errors.Is(err, context.Canceled) || !empty {
				t.Fatal("no-SU cancellation leaked entries", selected, step, empty, err)
			}
		}
	}
}
