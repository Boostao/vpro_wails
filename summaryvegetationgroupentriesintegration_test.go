package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func summaryGroupedRawTables(t *testing.T) (veg, su, master, personal, layers ProjectMetadataTable) {
	t.Helper()
	_, prepared := summaryRawGroupFixture(t)
	_, su, _, master, personal, _ = summaryLayerFixture()
	veg = ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows: []ProjectMetadataRow{}}
	for _, name := range prepared.CoverColumns {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: name})
	}
	for i, row := range prepared.ReducedRows[:3] {
		cells := []ProjectMetadataCell{metadataText(row.PlotNumber), cloneSiteUnitCell(row.Species)}
		for _, cover := range row.Covers {
			cells = append(cells, cloneSiteUnitCell(cover))
		}
		veg.Rows = append(veg.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
	}
	// A lower duplicate must not replace the MAX Cover1 or erase Cover2.
	duplicate := ProjectMetadataRow{RowID: "4", Cells: []ProjectMetadataCell{}}
	for _, cell := range veg.Rows[1].Cells {
		duplicate.Cells = append(duplicate.Cells, cloneSiteUnitCell(cell))
	}
	duplicate.Cells[2], duplicate.Cells[3] = strataReal(1), ProjectMetadataCell{Storage: "null"}
	veg.Rows = append(veg.Rows, duplicate)
	layers = summaryGroupedLayers()
	layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "LayerText"})
	for i := range layers.Rows {
		layers.Rows[i].Cells = append(layers.Rows[i].Cells, cloneSiteUnitCell(layers.Rows[i].Cells[0]))
	}
	return
}

func TestSummaryVegetationGroupedFullSourceMAXAndFanout(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	for _, fanout := range []bool{false, true} {
		for _, grouping := range []int{12, 13, 14} {
			for _, average := range []int{1, 2} {
				veg, su, master, personal, layers := summaryGroupedRawTables(t)
				if fanout {
					personal = lifeformReferenceFixture(lifeformReferenceRow("X", "Alpha", metadataInteger("1"), metadataText("alternate"), metadataText("U")))
				}
				got, err := prepareSummaryVegetationGrouped(context.Background(), "Sample", "Selected", veg, su, master, personal, layers, average, grouping)
				if err != nil {
					t.Fatal(grouping, fanout, average, err)
				}
				a, b, order := "12.00", "5.00", 12.0
				group := metadataText("A")
				if grouping == 13 || grouping == 14 {
					a, order = "14.00", 14
					group = metadataText("01")
					if grouping == 14 {
						group = null
					}
				}
				if average == 2 {
					a, order = "16.00", 16
					if grouping != 12 {
						a, order = "18.67", 18.67
					}
				}
				if fanout {
					if average == 1 {
						a, b, order = "24.00", "10.00", 24
						if grouping == 13 {
							a, b, order = "56.00", "20.00", 56
						} else if grouping == 14 {
							a, order = "28.00", 28
						}
					} else if grouping == 13 {
						a, b, order = "37.33", "10.00", 37.33
					}
				}
				expected := [][]ProjectMetadataCell{{group, metadataText("Alpha"), strataReal(order), metadataText(a), metadataText(b)}}
				if grouping == 12 {
					a, order = "2.00", 2
					if average == 2 || fanout {
						a, order = "4.00", 4
					}
					expected = append(expected, []ProjectMetadataCell{metadataText("B"), metadataText("Alpha"), strataReal(order), metadataText(a), null})
				}
				assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, expected)
				if got.Report.Units[0].NumPlots != 4 || got.Report.Units[1].NumPlots != 1 {
					t.Fatal("raw duplicates/reference fanout altered denominator")
				}
				sawDuplicate := false
				for _, row := range got.Report.Rows {
					for _, witness := range row.Witnesses {
						if witness.ConversionLayerID != "" || len(witness.RawVegRowIDs) == 0 {
							t.Fatal("invented conversion or lost physical raw provenance", witness)
						}
						if reflect.DeepEqual(witness.RawVegRowIDs, []string{"2", "4"}) {
							sawDuplicate = true
						}
					}
				}
				if !sawDuplicate {
					t.Fatal("MAX provenance lost lower duplicate source row")
				}
			}
		}
	}
}

func TestSummaryVegetationGroupedFullSourceCancellationAndDetachment(t *testing.T) {
	for _, grouping := range []int{12, 13, 14} {
		veg, su, master, personal, layers := summaryGroupedRawTables(t)
		before, err := json.Marshal([]ProjectMetadataTable{veg, su, master, personal, layers})
		if err != nil {
			t.Fatal(err)
		}
		probe := &projectUnitCancellationContext{Context: context.Background()}
		got, err := prepareSummaryVegetationGrouped(probe, "Sample", "Selected", veg, su, master, personal, layers, 1, grouping)
		if err != nil {
			t.Fatal(err)
		}
		*got.Report.Rows[0].Covers[0].Text = "changed"
		got.Report.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
		after, err := json.Marshal([]ProjectMetadataTable{veg, su, master, personal, layers})
		if err != nil || string(before) != string(after) {
			t.Fatal("full grouped report aliases source", err)
		}
		for point := 1; point <= probe.calls; point++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
			got, err := prepareSummaryVegetationGrouped(ctx, "Sample", "Selected", veg, su, master, personal, layers, 1, grouping)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
				t.Fatal("cancelled full preparation leaked result", grouping, point, err)
			}
		}
		veg.Rows[0].Cells[2] = strataReal(1e100)
		got, err = prepareSummaryVegetationGrouped(context.Background(), "Sample", "Selected", veg, su, master, personal, layers, 1, grouping)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("unrepresentable raw SINGLE accepted")
		}
		veg.Rows[0].Cells[2] = strataReal(10)
		if _, err := prepareSummaryVegetationGrouped(context.Background(), "Sample", "Selected", veg, su, master, personal, layers, 1, grouping); err != nil {
			t.Fatal("raw correction retry failed", err)
		}
	}
}

func TestSummaryVegetationGroupedLiteralThreeCharacterCodeAndProjection(t *testing.T) {
	raw, su, stage, master, personal, _ := summaryLayerFixture()
	layers := summaryGroupedLayers()
	layers.Rows[0].Cells[3] = metadataText("001")
	for i := range stage.Rows {
		stage.Rows[i].Cells[1] = metadataText("001")
	}
	got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, layers, 1, 13)
	if err != nil || got.Report.Rows[0].Layer.Text == nil || *got.Report.Rows[0].Layer.Text != "001" ||
		!reflect.DeepEqual(got.Report.Rows[0].Order, strataReal(10)) {
		t.Fatal("literal three-character LifeForm or numeric Order lost", got, err)
	}
	for _, value := range []ProjectMetadataCell{metadataText("AA"), metadataText(string([]byte{0xff})), metadataInteger("1")} {
		altered := summaryGroupedLayers()
		altered.Rows[0].Cells[2] = value
		got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, altered, 1, 12)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("invalid projected Strata accepted", got, err)
		}
	}
	layers = summaryGroupedLayers()
	layers.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "null"}
	for i := range stage.Rows {
		stage.Rows[i].Cells[1] = metadataText("1")
	}
	got, err = planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, layers, 1, 12)
	if err != nil || got.Report.Rows[0].Layer.Storage != "null" {
		t.Fatal("nullable projected group lost", got, err)
	}
}
