package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryEnvironmentalCountPreparationFixture() vprocore.ProjectMetadataTable {
	cell := func(text *string) vprocore.ProjectMetadataCell {
		if text == nil {
			return vprocore.ProjectMetadataCell{Storage: "null"}
		}
		return vprocore.ProjectMetadataCell{Storage: "text", Text: text}
	}
	p, a, lower, empty, zero, spaced := "P", "A", "a", "", "zero", " A "
	rows := [][2]*string{{&p, &a}, {&p, &a}, {nil, &a}, {&p, &lower}, {nil, &zero},
		{&empty, &empty}, {&p, &spaced}, {&p, nil}}
	table := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for i, row := range rows {
		table.Rows = append(table.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []vprocore.ProjectMetadataCell{cell(row[0]), cell(row[1])}})
	}
	return table
}

func TestSummaryEnvironmentalCountPreparationLiteralScope(t *testing.T) {
	table := summaryEnvironmentalCountPreparationFixture()
	before := cloneSummaryVegetationFieldListTable(table)
	for _, tc := range []struct {
		unit string
		want string
	}{
		{"A", "2"}, {"a", "1"}, {" A ", "1"}, {"", "1"}, {"zero", "0"},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			cell, err := prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &tc.unit, table)
			if err != nil || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{Storage: "integer", Integer: &tc.want}) {
				t.Fatalf("got %#v %v; want %s", cell, err, tc.want)
			}
		})
	}
	missing := "missing"
	cell, err := prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &missing, table)
	if err == nil || !strings.Contains(err.Error(), "error3021") || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("missing result %#v %v", cell, err)
	}
	cell, err = prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), nil, vprocore.ProjectMetadataTable{})
	if err == nil || !strings.Contains(err.Error(), "error94") || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("NULL pre-schema result %#v %v", cell, err)
	}
	if !reflect.DeepEqual(table, before) {
		t.Fatal("caller physical data changed")
	}
}

func TestSummaryEnvironmentalCountPreparationRefusalCorrection(t *testing.T) {
	unit := "A"
	for _, tc := range []struct {
		name   string
		change func(*vprocore.ProjectMetadataTable)
	}{
		{"missing column", func(table *vprocore.ProjectMetadataTable) { table.Columns[0].Name = "missing" }},
		{"duplicate column", func(table *vprocore.ProjectMetadataTable) { table.Columns[0].Name = "SiteUnit" }},
		{"duplicate row", func(table *vprocore.ProjectMetadataTable) { table.Rows[1].RowID = table.Rows[0].RowID }},
		{"partial row", func(table *vprocore.ProjectMetadataTable) { table.Rows[0].Cells = table.Rows[0].Cells[:1] }},
		{"noncanonical row", func(table *vprocore.ProjectMetadataTable) { table.Rows[0].RowID = "01" }},
		{"malformed text", func(table *vprocore.ProjectMetadataTable) {
			value := string([]byte{0xff})
			table.Rows[7].Cells[0] = vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
		}},
		{"integer plot", func(table *vprocore.ProjectMetadataTable) {
			value := "1"
			table.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
		}},
		{"blob unit", func(table *vprocore.ProjectMetadataTable) {
			value := "41"
			table.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "blob", BlobHex: &value}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := summaryEnvironmentalCountPreparationFixture()
			tc.change(&table)
			cell, err := prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, table)
			if err == nil || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
				t.Fatalf("invalid result %#v %v", cell, err)
			}
			cell, err = prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, summaryEnvironmentalCountPreparationFixture())
			if err != nil || cell.Integer == nil || *cell.Integer != "2" {
				t.Fatalf("corrected result %#v %v", cell, err)
			}
		})
	}
	bad := string([]byte{0xff})
	cell, err := prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &bad, summaryEnvironmentalCountPreparationFixture())
	if err == nil || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("malformed scope result %#v %v", cell, err)
	}
	cell, err = prepareSummaryVegetationEnvironmentalPlotCount(nil, &unit, summaryEnvironmentalCountPreparationFixture())
	if err == nil || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("nil context %#v %v", cell, err)
	}
}

func TestSummaryEnvironmentalCountPreparationBoundariesCancel(t *testing.T) {
	unit, plot := "A", "P"
	table := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteUnit"}, {Name: "PlotNumber"}}}
	for i := 1; i <= 32767; i++ {
		table.Rows = append(table.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(i), Cells: []vprocore.ProjectMetadataCell{
			{Storage: "text", Text: &unit}, {Storage: "text", Text: &plot},
		}})
	}
	cell, err := prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, table)
	if err != nil || cell.Integer == nil || *cell.Integer != "32767" {
		t.Fatalf("32767 result %#v %v", cell, err)
	}
	table.Rows = append(table.Rows, vprocore.ProjectMetadataRow{RowID: "32768", Cells: table.Rows[0].Cells})
	cell, err = prepareSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, table)
	if err == nil || !strings.Contains(err.Error(), "overflow6") || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("32768 result %#v %v", cell, err)
	}
	base, cancel := context.WithCancel(context.Background())
	ctx := &summaryEnvironmentalCategoryCancelContext{Context: base, cancel: cancel, at: 4}
	cell, err = prepareSummaryVegetationEnvironmentalPlotCount(ctx, &unit, table)
	cancel()
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, vprocore.ProjectMetadataCell{}) {
		t.Fatalf("inflight cancellation %#v %v", cell, err)
	}
}
