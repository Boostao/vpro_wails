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

func summaryGroupedLayers() ProjectMetadataTable {
	return ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "Strata"}, {Name: "LayerCode"},
	}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{metadataText("1"), metadataText("A1"), metadataText("A"), metadataText("01")}},
		{RowID: "2", Cells: []ProjectMetadataCell{metadataText("2"), metadataText("A2"), metadataText("A"), metadataText("02")}},
		{RowID: "3", Cells: []ProjectMetadataCell{metadataText("4"), metadataText("B1"), metadataText("B"), metadataText("04")}},
	}}
}

func TestSummaryVegetationGroupedStageSourceQueries(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "summaryVegetationGroupSource.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Grouping int     `json:"grouping"`
			Average  int     `json:"average"`
			Display  int     `json:"display"`
			Staged   [][]any `json:"stagedRows"`
			Fields   []struct {
				Name string `json:"name"`
				Type int    `json:"type"`
				Size int    `json:"size"`
			} `json:"expectedFields"`
			Rows [][]any `json:"expectedRows"`
		} `json:"stagedQueryCases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil || len(fixture.Cases) != 6 {
		t.Fatal("independent source fixture coverage lost", err)
	}
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
					t.Fatal("source fixture scalar type lost")
				}
			}
			stage.Rows = append(stage.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
		}
		got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, summaryGroupedLayers(), test.Average, test.Grouping)
		if err != nil {
			t.Fatal(test.Grouping, test.Average, err)
		}
		for i, field := range []summaryVegetationGroupField{got.GroupField, got.OrderField} {
			index := i * 2
			expected := test.Fields[index]
			if field.Name != expected.Name || field.DAOType != expected.Type || field.Size != expected.Size {
				t.Fatal("DAO group/Order metadata differs", field, expected)
			}
		}
		rows := [][]any{}
		for _, row := range got.Report.Rows {
			values := []any{}
			cells := append([]ProjectMetadataCell{row.Layer, row.Species, row.Order}, row.Covers...)
			for _, cell := range cells {
				var value any
				switch cell.Storage {
				case "text":
					value = *cell.Text
				case "real":
					value = *cell.Real
				case "null":
				default:
					t.Fatal("unexpected projected storage", cell)
				}
				values = append(values, value)
			}
			rows = append(rows, values)
		}
		if !reflect.DeepEqual(got.Report.Columns, []string{"A", "B"}) || !reflect.DeepEqual(rows, test.Rows) {
			t.Fatal("independent grouping rows or numeric Order differ", test.Grouping, test.Average, rows, test.Rows)
		}
	}
}

func TestSummaryVegetationGroupedStageGuardsCancellationAndOwnership(t *testing.T) {
	for _, grouping := range []int{12, 13, 14} {
		raw, su, stage, master, personal, _ := summaryLayerFixture()
		layers := summaryGroupedLayers()
		if grouping == 13 {
			for i := range stage.Rows {
				stage.Rows[i].Cells[1] = metadataText("01")
			}
		}
		before := cloneSiteUnitCell(stage.Rows[0].Cells[3])
		got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, layers, 1, grouping)
		if err != nil {
			t.Fatal(err)
		}
		*got.Report.Rows[0].Covers[0].Text = "changed"
		if !reflect.DeepEqual(before, stage.Rows[0].Cells[3]) {
			t.Fatal("group output aliases stage")
		}
		probe := &projectUnitCancellationContext{Context: context.Background()}
		if _, err := planSummaryVegetationGroupedStage(probe, raw, su, stage, master, personal, layers, 2, grouping); err != nil {
			t.Fatal(err)
		}
		for point := 1; point <= probe.calls; point++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
			got, err := planSummaryVegetationGroupedStage(ctx, raw, su, stage, master, personal, layers, 2, grouping)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
				t.Fatal("cancelled group leaked output", grouping, point, got, err)
			}
		}
		for _, bad := range []string{"duplicate", "overlength", "malformed"} {
			altered := summaryGroupedLayers()
			if bad == "duplicate" {
				altered.Rows = append(altered.Rows, altered.Rows[0])
			} else {
				index, limit := 0, 2
				if grouping == 13 {
					index, limit = 3, 3
				}
				text := strings.Repeat("x", limit+1)
				if bad == "malformed" {
					text = string([]byte{0xff})
				}
				altered.Rows[0].Cells[index] = metadataText(text)
			}
			got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, altered, 1, grouping)
			if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
				t.Fatal("invalid literal mapping accepted", grouping, bad, got, err)
			}
		}
		for i := range stage.Rows {
			stage.Rows[i].Cells[3] = ProjectMetadataCell{Storage: "null"}
		}
		got, err = planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, layers, 1, grouping)
		if err != nil || len(got.Report.Rows) != 0 || !reflect.DeepEqual(got.Report.Columns, []string{"A", "B"}) {
			t.Fatal("allNULL groups confused with absent pivot columns", got, err)
		}
	}
	raw, su, stage, master, personal, _ := summaryLayerFixture()
	for _, test := range [][2]int{{0, 12}, {1, 11}, {1, 15}} {
		got, err := planSummaryVegetationGroupedStage(context.Background(), raw, su, stage, master, personal, summaryGroupedLayers(), test[0], test[1])
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("unsupported grouping/calculation accepted", got, err)
		}
	}
	if _, err := planSummaryVegetationGroupedStage(nil, raw, su, stage, master, personal, summaryGroupedLayers(), 1, 12); err == nil {
		t.Fatal("nil context accepted")
	}
}
