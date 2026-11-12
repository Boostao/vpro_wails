package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
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

func summaryLayerFixture() (raw, su, stage, master, personal, layers vprocore.ProjectMetadataTable) {
	raw, su = summaryUnitTables([]vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("P2"), vprotest.MetadataText("P3")},
		[][2]vprocore.ProjectMetadataCell{
			{vprotest.MetadataText("P1"), vprotest.MetadataText("A")}, {vprotest.MetadataText("P1"), vprotest.MetadataText("A")},
			{vprotest.MetadataText("P2"), vprotest.MetadataText("A")}, {vprotest.MetadataText("P5"), vprotest.MetadataText("A")},
			{vprotest.MetadataText("P3"), vprotest.MetadataText("B")}, {{Storage: "null"}, vprotest.MetadataText("A")},
			{vprotest.MetadataText("P9"), {Storage: "null"}}, {vprotest.MetadataText("P6"), vprotest.MetadataText("Orphan")},
		})
	stage = vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "Plotnumber"}, {Name: "Layer"}, {Name: "Species"}, {Name: "Cover"},
	}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, row := range []struct {
		plot, layer, species string
		cover                float64
	}{{"P1", "1", "X", 10}, {"P2", "1", "X", 20}, {"P3", "1", "X", 5}, {"P2", "1", "S", 2}} {
		stage.Rows = append(stage.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(stage.Rows) + 1), Cells: []vprocore.ProjectMetadataCell{
			vprotest.MetadataText(row.plot), vprotest.MetadataText(row.layer), vprotest.MetadataText(row.species), vprotest.StrataReal(row.cover),
		}})
	}
	master = sftest.LifeformReferenceFixture(
		vprotest.LifeformReferenceRow("X", "Alpha", vprotest.MetadataInteger("1"), vprotest.MetadataText("x"), vprotest.MetadataText("U")),
		vprotest.LifeformReferenceRow("S", "Excluded", vprotest.MetadataInteger("2"), vprotest.MetadataText("s"), vprotest.MetadataText("s")))
	personal = sftest.LifeformReferenceFixture()
	layers = vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "LayerText"}, {Name: "Layer1234567"}, {Name: "Layer"},
	}, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.MetadataText("1"), vprotest.MetadataText("A1")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("10"), vprotest.MetadataText("10"), {Storage: "null"}}},
	}}
	return
}

func assertSummaryLayerRows(t *testing.T, result summaryVegetationLayerReport, columns []string, expected [][]vprocore.ProjectMetadataCell) {
	t.Helper()
	if !reflect.DeepEqual(result.Columns, columns) || len(result.Rows) != len(expected) {
		t.Fatal("source pivot names or final row count changed", result, expected)
	}
	for i, row := range result.Rows {
		cells := []vprocore.ProjectMetadataCell{row.Layer, row.Species, row.Order}
		cells = append(cells, row.Covers...)
		if !reflect.DeepEqual(cells, expected[i]) {
			t.Fatal("independent source scalar/NULL/shape witness differs", i, cells, expected[i])
		}
	}
}

func TestSummaryVegetationLayerSourceJoins(t *testing.T) {
	for _, scenario := range []string{"canonical-layer", "same-name-fanout", "different-name-fanout", "null-layer"} {
		for _, average := range []int{1, 2} {
			raw, su, stage, master, personal, layers := summaryLayerFixture()
			switch scenario {
			case "same-name-fanout":
				personal = sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("X", "Alpha", vprotest.MetadataInteger("1"), vprotest.MetadataText("alternate"), vprotest.MetadataText("U")))
			case "different-name-fanout":
				personal = sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("X", "Beta", vprotest.MetadataInteger("1"), vprotest.MetadataText("alternate"), vprotest.MetadataText("U")))
			case "null-layer":
				stage.Rows = append(stage.Rows,
					vprocore.ProjectMetadataRow{RowID: "5", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("10"), vprotest.MetadataText("X"), vprotest.StrataReal(4)}},
					vprocore.ProjectMetadataRow{RowID: "6", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2"), vprotest.MetadataText("Z"), vprotest.MetadataText("X"), vprotest.StrataReal(8)}})
			}
			a, s := "10.00", "0.50"
			if average == 2 {
				a, s = "13.33", "2.00"
			}
			alpha := []vprocore.ProjectMetadataCell{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("5.00"), vprotest.MetadataText(a), vprotest.MetadataText("5.00")}
			synonym := []vprocore.ProjectMetadataCell{vprotest.MetadataText("A1"), vprotest.MetadataText("S"), vprotest.MetadataText(s), vprotest.MetadataText(s), {Storage: "null"}}
			expected := [][]vprocore.ProjectMetadataCell{alpha, synonym}
			if scenario == "same-name-fanout" {
				a, b, order := "20.00", "10.00", "20.00"
				if average == 2 {
					a, b, order = "13.33", "5.00", "5.00"
				}
				expected = [][]vprocore.ProjectMetadataCell{{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText(order), vprotest.MetadataText(a), vprotest.MetadataText(b)}, synonym}
			} else if scenario == "different-name-fanout" {
				expected = [][]vprocore.ProjectMetadataCell{alpha, {vprotest.MetadataText("A1"), vprotest.MetadataText("Beta"), vprotest.MetadataText("5.00"), vprotest.MetadataText(a), vprotest.MetadataText("5.00")}, synonym}
			} else if scenario == "null-layer" {
				value := "4.00"
				if average == 2 {
					value = "5.33"
				}
				expected = append([][]vprocore.ProjectMetadataCell{{{Storage: "null"}, vprotest.MetadataText("Alpha"), vprotest.MetadataText(value), vprotest.MetadataText(value), {Storage: "null"}}}, expected...)
			}
			result, err := planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, average)
			if err != nil {
				t.Fatal(scenario, average, err)
			}
			assertSummaryLayerRows(t, result, []string{"A", "B"}, expected)
			if len(result.Units) != 2 || result.Units[0].NumPlots != 4 || result.Units[1].NumPlots != 1 {
				t.Fatal("joins multiplied denominator or raw orphan admission", result.Units)
			}
			index, weight := 0, 4
			if scenario == "null-layer" {
				index = 1
			}
			if scenario == "same-name-fanout" {
				weight = 8
			}
			if len(result.Rows[index].Witnesses) != weight {
				t.Fatal("SU/reference source join multiplicity lost", scenario, result.Rows[index].Witnesses)
			}
		}
	}
}

func TestSummaryVegetationLayerSourcePivotTyping(t *testing.T) {
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	for _, scenario := range []string{"floor1", "floor2", "mixed1", "mixed2", "null", "mixed-null", "final-empty-B"} {
		raw, su, stage, master, personal, layers := summaryLayerFixture()
		mode := 1
		if strings.HasSuffix(scenario, "2") {
			mode = 2
		}
		for i := range stage.Rows {
			stage.Rows[i].Cells[3] = vprotest.StrataReal(0)
			if scenario == "null" || scenario == "mixed-null" {
				stage.Rows[i].Cells[3] = null
			}
		}
		if strings.HasPrefix(scenario, "mixed") {
			stage.Rows[2].Cells[3] = vprotest.StrataReal(5)
		}
		expected := [][]vprocore.ProjectMetadataCell{
			{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("0.01"), vprotest.MetadataText("0.01"), vprotest.MetadataText("0.01")},
			{vprotest.MetadataText("A1"), vprotest.MetadataText("S"), vprotest.MetadataText("0.01"), vprotest.MetadataText("0.01"), null},
		}
		columns := []string{"A", "B"}
		switch {
		case scenario == "null":
			expected = [][]vprocore.ProjectMetadataCell{}
		case scenario == "mixed-null":
			expected = [][]vprocore.ProjectMetadataCell{{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("5.00"), null, vprotest.MetadataText("5.00")}}
		case strings.HasPrefix(scenario, "mixed"):
			expected[0] = []vprocore.ProjectMetadataCell{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("5.00"), vprotest.MetadataText("0.01"), vprotest.MetadataText("5.00")}
		case scenario == "final-empty-B":
			stage.Rows = append(stage.Rows[:2], stage.Rows[3])
			for i := range stage.Rows {
				stage.Rows[i].Cells[3] = vprotest.StrataReal(10)
			}
			columns = []string{"A"}
			expected = [][]vprocore.ProjectMetadataCell{
				{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("7.50"), vprotest.MetadataText("7.50")},
				{vprotest.MetadataText("A1"), vprotest.MetadataText("S"), vprotest.MetadataText("2.50"), vprotest.MetadataText("2.50")},
			}
		}
		result, err := planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, mode)
		if err != nil {
			t.Fatal(scenario, err)
		}
		assertSummaryLayerRows(t, result, columns, expected)
	}
}

func summaryRawLayerFixture() (raw, su, master, personal, layers vprocore.ProjectMetadataTable) {
	raw, su, stage, master, personal, layers := summaryLayerFixture()
	raw.Columns = []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}
	for _, column := range longVegetationCoverColumns() {
		raw.Columns = append(raw.Columns, vprocore.ProjectMetadataColumn{Name: column})
	}
	raw.Rows = nil
	for _, staged := range stage.Rows {
		cells := []vprocore.ProjectMetadataCell{vprocore.CloneSiteUnitCell(staged.Cells[0]), vprocore.CloneSiteUnitCell(staged.Cells[2])}
		for i := range longVegetationCoverColumns() {
			cell := vprocore.ProjectMetadataCell{Storage: "null"}
			if i == 0 {
				cell = vprocore.CloneSiteUnitCell(staged.Cells[3])
			}
			cells = append(cells, cell)
		}
		raw.Rows = append(raw.Rows, vprocore.ProjectMetadataRow{RowID: staged.RowID, Cells: cells})
	}
	return raw, su, master, personal, layers
}

func TestSummaryVegetationLayerRawPreparationAndSourceOrigins(t *testing.T) {
	raw, su, master, personal, layers := summaryRawLayerFixture()
	duplicate := vprocore.ProjectMetadataRow{RowID: "5", Cells: append([]vprocore.ProjectMetadataCell{}, raw.Rows[0].Cells...)}
	duplicate.Cells[2] = vprotest.StrataReal(9)
	raw.Rows = append(raw.Rows, duplicate)
	result, err := prepareSummaryVegetationLayer(context.Background(), "Project", "Selected", raw, su, master, personal, layers, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertSummaryLayerRows(t, result, []string{"A", "B"}, [][]vprocore.ProjectMetadataCell{
		{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("5.00"), vprotest.MetadataText("10.00"), vprotest.MetadataText("5.00")},
		{vprotest.MetadataText("A1"), vprotest.MetadataText("S"), vprotest.MetadataText("0.50"), vprotest.MetadataText("0.50"), {Storage: "null"}},
	})
	found := 0
	for _, witness := range result.Rows[0].Witnesses {
		if reflect.DeepEqual(witness.RawVegRowIDs, []string{"1", "5"}) {
			if witness.ConversionLayerID != "1" || witness.LayerMapID != "1" {
				t.Fatal("conversion and caption-join layer identities lost", witness)
			}
			found++
		}

	}
	if found != 2 {
		t.Fatal("MAX raw source origins or duplicate SU join weight lost", result.Rows[0].Witnesses)
	}
	for i := range raw.Rows {
		raw.Rows[i].Cells[2] = vprotest.StrataReal(0.1)
	}
	result, err = prepareSummaryVegetationLayer(context.Background(), "Project", "Selected", raw, su, master, personal, layers, 2)
	if err != nil || result.Rows[0].Covers[0].Text == nil || *result.Rows[0].Covers[0].Text != "0.10" ||
		*result.Rows[0].Witnesses[0].Cover.Real != 0.10000000149011612 {
		t.Fatal("raw DOUBLE was not staged to original SINGLE before aggregation", result, err)
	}
	for i := range raw.Rows {
		raw.Rows[i].Cells[2] = vprocore.ProjectMetadataCell{Storage: "null"}
	}
	result, err = prepareSummaryVegetationLayer(context.Background(), "Project", "Selected", raw, su, master, personal, layers, 1)
	if err != nil || len(result.Units) != 2 || result.Columns == nil || len(result.Columns) != 0 || len(result.Rows) != 0 {
		t.Fatal("raw NULL covers must admit units but not create stage/pivot columns", result, err)
	}
}

func TestSummaryVegetationLayerRawBoundaryAndCancellation(t *testing.T) {
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable){
		func(raw, su *vprocore.ProjectMetadataTable) {
			raw.Rows[0].Cells[0] = vprotest.MetadataText("12345678")
			su.Rows[0].Cells[0], su.Rows[1].Cells[0] = vprotest.MetadataText("12345678"), vprotest.MetadataText("12345678")
		},
		func(raw, _ *vprocore.ProjectMetadataTable) {
			raw.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
		},
		func(raw, _ *vprocore.ProjectMetadataTable) { raw.Rows[0].Cells[1] = vprotest.MetadataText("") },
		func(raw, _ *vprocore.ProjectMetadataTable) { raw.Rows[0].Cells[1] = vprotest.MetadataText("123456789") },
		func(raw, _ *vprocore.ProjectMetadataTable) {
			raw.Rows[0].Cells[len(raw.Rows[0].Cells)-1] = vprotest.StrataReal(math.MaxFloat64)
		},
	} {
		raw, su, master, personal, layers := summaryRawLayerFixture()
		mutate(&raw, &su)
		result, err := prepareSummaryVegetationLayer(context.Background(), "Project", "Selected", raw, su, master, personal, layers, 1)
		if err == nil || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("TempReportVeg required text/SINGLE constraints returned partial report", result, err)
		}
	}
	raw, su, master, personal, layers := summaryRawLayerFixture()
	probe := &projectUnitCancellationContext{Context: context.Background()}
	result, err := prepareSummaryVegetationLayer(probe, "Project", "Selected", raw, su, master, personal, layers, 1)
	if err != nil || len(result.Rows) != 2 {
		t.Fatal(result, err)
	}
	result.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
	if raw.Rows[0].RowID != "1" {
		t.Fatal("raw origins alias source")
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		result, err := prepareSummaryVegetationLayer(ctx, "Project", "Selected", raw, su, master, personal, layers, 1)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("cancelled raw preparation or origin association leaked data", point, result, err)
		}
	}
	if _, err := prepareSummaryVegetationLayer(nil, "Project", "Selected", raw, su, master, personal, layers, 1); err == nil {
		t.Fatal("nil preparation context accepted")
	}
	if _, err := prepareSummaryVegetationLayer(context.Background(), strings.Repeat("x", 21), "Selected", raw, su, master, personal, layers, 1); err == nil {
		t.Fatal("overlength ProjectID silently truncated")
	}
}

func TestSummaryVegetationLayerStrictErrorsAndCancellation(t *testing.T) {
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable){
		func(stage, _, _ *vprocore.ProjectMetadataTable) { stage.Rows[0].Cells[3] = vprotest.StrataReal(0.1) },
		func(stage, _, _ *vprocore.ProjectMetadataTable) {
			stage.Rows[0].Cells[3] = vprotest.StrataReal(math.Inf(1))
		},
		func(stage, _, _ *vprocore.ProjectMetadataTable) { stage.Rows[0].Cells[0] = vprotest.MetadataText("") },
		func(stage, _, _ *vprocore.ProjectMetadataTable) {
			stage.Rows[0].Cells[0] = vprotest.MetadataText("123456789")
		},
		func(stage, _, _ *vprocore.ProjectMetadataTable) {
			stage.Rows[0].Cells[0] = vprotest.MetadataText("😀😀😀😀x")
		},
		func(stage, _, _ *vprocore.ProjectMetadataTable) { stage.Rows[0].Cells[2] = vprotest.MetadataText("") },
		func(stage, _, _ *vprocore.ProjectMetadataTable) {
			stage.Rows[0].Cells[2] = vprotest.MetadataText(string([]byte{255}))
		},
		func(stage, _, _ *vprocore.ProjectMetadataTable) { stage.Rows[1].RowID = stage.Rows[0].RowID },
		func(stage, _, _ *vprocore.ProjectMetadataTable) { stage.Columns[0].Name = "plotnumber" },
		func(_, _, layers *vprocore.ProjectMetadataTable) {
			layers.Rows[1].Cells[1] = vprotest.MetadataText("1")
		},
		func(_, _, layers *vprocore.ProjectMetadataTable) {
			layers.Rows[0].Cells[2] = vprotest.MetadataInteger("1")
		},
		func(_, master, _ *vprocore.ProjectMetadataTable) {
			master.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 256))
		},
		func(stage, _, _ *vprocore.ProjectMetadataTable) {
			stage.Rows[0].Cells[3] = vprotest.StrataReal(math.Ldexp(1, 100))
			stage.Rows[1].Cells[3] = vprotest.StrataReal(1)
		},
	} {
		raw, su, stage, master, personal, layers := summaryLayerFixture()
		mutate(&stage, &master, &layers)
		result, err := planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, 1)
		if err == nil || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("strict stage/reference/layer or unsafe arithmetic returned partial report", result, err)
		}
	}
	raw, su, stage, master, personal, layers := summaryLayerFixture()
	probe := &projectUnitCancellationContext{Context: context.Background()}
	result, err := planSummaryVegetationLayer(probe, raw, su, stage, master, personal, layers, 1)
	if err != nil || len(result.Rows) != 2 {
		t.Fatal(result, err)
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		result, err := planSummaryVegetationLayer(ctx, raw, su, stage, master, personal, layers, 1)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("cancelled join/pivot leaked partial data", point, result, err)
		}
	}
	for _, mode := range []int{0, 3} {
		if _, err := planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, mode); err == nil {
			t.Fatal("unsupported mode guessed")
		}
	}
	if _, err := planSummaryVegetationLayer(nil, raw, su, stage, master, personal, layers, 1); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestSummaryVegetationLayerUnionNullFallbackAndOwnership(t *testing.T) {
	raw, su, stage, master, personal, layers := summaryLayerFixture()
	personal = sftest.LifeformReferenceFixture(master.Rows[0].Cells)
	result, err := planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, 1)
	if err != nil || len(result.Rows[0].Witnesses) != 4 || len(result.Rows[0].Witnesses[0].ReferenceOrigins) != 2 {
		t.Fatal("exact UNION duplicate should retain origins without multiplying cover", result, err)
	}
	*result.Rows[0].Layer.Text = "changed"
	*result.Rows[0].Species.Text = "changed"
	*result.Rows[0].Covers[0].Text = "changed"
	*result.Rows[0].Witnesses[0].Cover.Real = 99
	result.Rows[0].Witnesses[0].ReferenceOrigins[0].Table = "changed"
	result.Units[0].MembershipIDs[0] = "changed"
	if *layers.Rows[0].Cells[2].Text != "A1" || *master.Rows[0].Cells[1].Text != "Alpha" || *stage.Rows[0].Cells[3].Real != 10 {
		t.Fatal("result mutated source data")
	}
	master.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
	personal = sftest.LifeformReferenceFixture()
	stage.Rows[0].Cells[2] = vprocore.ProjectMetadataCell{Storage: "null"}
	stage.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
	result, err = planSummaryVegetationLayer(context.Background(), raw, su, stage, master, personal, layers, 1)
	if err != nil || result.Rows[0].Layer.Storage != "null" || result.Rows[0].Species.Storage != "null" ||
		*result.Rows[0].Covers[0].Text != "5.00" || *result.Rows[2].Species.Text != "X" {
		t.Fatal("NULL layer/species grouping or scientific-name NULL fallback repaired", result, err)
	}
}
