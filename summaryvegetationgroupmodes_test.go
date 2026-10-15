package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryVegetationGroupedAdditionalSourceOutcomes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "summaryVegetationGroupAdditionalSource.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Grouping       int     `json:"grouping"`
			Display        int     `json:"display"`
			Average        int     `json:"average"`
			AverageIgnored bool    `json:"averageIgnored"`
			ShowClass      int     `json:"showClass"`
			Staged         [][]any `json:"stagedRows"`
			Fields         []struct {
				Name string `json:"name"`
				Type int    `json:"type"`
				Size int    `json:"size"`
			} `json:"expectedFields"`
			Rows        [][]any         `json:"expectedRows"`
			SourceError json.RawMessage `json:"sourceError"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) != 36 {
		t.Fatal("original additional source coverage lost", err)
	}
	refusals := 0
	for _, test := range fixture.Cases {
		raw, su, stage, master, personal, _ := summaryLayerFixture()
		stage.Rows = nil
		for i, row := range test.Staged {
			cells := []ProjectMetadataCell{}
			for _, item := range row {
				switch value := item.(type) {
				case nil:
					cells = append(cells, ProjectMetadataCell{Storage: "null"})
				case string:
					cells = append(cells, metadataText(value))
				case float64:
					cells = append(cells, strataReal(value))
				default:
					t.Fatal("source input scalar lost")
				}
			}
			stage.Rows = append(stage.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
		}
		averages := []int{test.Average}
		if test.AverageIgnored {
			averages = []int{1, 2}
		}
		for _, average := range averages {
			got, err := planSummaryVegetationGroupedStageOptions(context.Background(), raw, su, stage, master, personal,
				summaryGroupedLayers(), average, test.Grouping, test.Display, test.ShowClass)
			if string(test.SourceError) != "null" {
				if err == nil || !strings.Contains(err.Error(), "CDbl composite criteria type mismatch") ||
					!reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
					t.Fatal("original grouped source error silently repaired", test.Grouping, test.Display, got, err)
				}
				refusals++
				continue
			}
			if err != nil {
				t.Fatal(test.Grouping, test.Display, test.ShowClass, average, err)
			}
			for i, field := range []summaryVegetationGroupField{got.GroupField, got.OrderField} {
				expected := test.Fields[i*2]
				if field.Name != expected.Name || field.DAOType != expected.Type || field.Size != expected.Size {
					t.Fatal("original grouped DAO metadata lost", field, expected)
				}
			}
			rows := [][]any{}
			for _, row := range got.Report.Rows {
				values := []any{}
				cells := append([]ProjectMetadataCell{row.Layer, row.Species, row.Order}, row.Covers...)
				for _, cell := range cells {
					switch cell.Storage {
					case "null":
						values = append(values, nil)
					case "text":
						values = append(values, *cell.Text)
					default:
						t.Fatal("unexpected original grouped display scalar", cell)
					}
				}
				rows = append(rows, values)
			}
			if !reflect.DeepEqual(rows, test.Rows) || !reflect.DeepEqual(got.Report.Columns, []string{"A", "B"}) {
				t.Fatal("independent source grouped display rows differ", test.Grouping, test.Display, test.ShowClass, rows, test.Rows)
			}
		}
	}
	if refusals != 4 {
		t.Fatal("four original Group12/13 Display2 failures lost", refusals)
	}
}

func TestSummaryVegetationGroupedAdditionalRawCancellationAndNulls(t *testing.T) {
	for _, grouping := range []int{12, 13, 14} {
		for _, mode := range [][2]int{{1, 0}, {3, 0}, {4, 20}, {5, 10}, {2, 0}} {
			if grouping != 14 && mode[0] == 2 {
				continue
			}
			veg, su, master, personal, layers := summaryGroupedRawTables(t)
			probe := &projectUnitCancellationContext{Context: context.Background()}
			got, err := prepareSummaryVegetationGroupedOptions(probe, "Sample", "Selected", veg, su, master, personal,
				layers, 1, grouping, mode[0], mode[1])
			if err != nil {
				t.Fatal(grouping, mode, err)
			}
			if got.OrderField.DAOType != 10 || got.OrderField.Size != 0 {
				t.Fatal("class late override or prefix Order is not TEXT")
			}
			for point := 1; point <= probe.calls; point++ {
				ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
				got, err := prepareSummaryVegetationGroupedOptions(ctx, "Sample", "Selected", veg, su, master, personal,
					layers, 1, grouping, mode[0], mode[1])
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
					t.Fatal("cancelled additional preparation leaked output", grouping, mode, point, err)
				}
			}
			raw, members, stage, refs, user, _ := summaryLayerFixture()
			for i := range stage.Rows {
				stage.Rows[i].Cells[3] = ProjectMetadataCell{Storage: "null"}
			}
			got, err = planSummaryVegetationGroupedStageOptions(context.Background(), raw, members, stage, refs, user,
				summaryGroupedLayers(), 1, grouping, mode[0], mode[1])
			if mode[0] == 1 || mode[0] == 3 {
				if err == nil || !strings.Contains(err.Error(), "all-NULL staged group") ||
					!reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
					t.Fatal("allNULL original failure lost", got, err)
				}
			} else if err != nil || len(got.Report.Rows) != 0 || !reflect.DeepEqual(got.Report.Columns, []string{"A", "B"}) {
				t.Fatal("allNULL class/None2 output differs", got, err)
			}
		}
	}
}
