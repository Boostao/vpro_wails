package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type summaryGroupRawFixture struct {
	CoverColumns []string `json:"rawCoverColumns"`
	Inputs       []struct {
		Plot    string    `json:"plotNumber"`
		Species string    `json:"species"`
		Bits    []*string `json:"coverSingleBits"`
	} `json:"rawInputs"`
	References map[string][][]any `json:"rawReferenceFanout"`
	Cases      []struct {
		Builder  string `json:"builder"`
		Scenario string `json:"scenario"`
		Fields   []struct {
			Name string `json:"name"`
			Type int    `json:"type"`
			Size int    `json:"size"`
		} `json:"expectedFields"`
		Rows []struct {
			Values []any  `json:"values"`
			Bits   string `json:"singleBits"`
		} `json:"expectedRows"`
	} `json:"rawBuilderCases"`
}

func summaryRawGroupFixture(t *testing.T) (summaryGroupRawFixture, VegetationReportPreparation) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "summaryVegetationGroupSource.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture summaryGroupRawFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	prepared := VegetationReportPreparation{Project: "Sample", SU: "Selected",
		CoverColumns: fixture.CoverColumns, ReducedRows: []VegetationReportReducedRow{}}
	for i, input := range fixture.Inputs {
		row := VegetationReportReducedRow{PlotNumber: input.Plot, Species: metadataText(input.Species),
			SourceRowIDs: []string{siteUnitNumericString(float64(i + 1))}, Covers: []ProjectMetadataCell{}}
		for _, bits := range input.Bits {
			cell := ProjectMetadataCell{Storage: "null"}
			if bits != nil {
				bytes, err := hex.DecodeString(*bits)
				if err != nil || len(bytes) != 4 {
					t.Fatal("independent input SINGLE bits lost", bits, err)
				}
				value := float64(math.Float32frombits(binary.LittleEndian.Uint32(bytes)))
				cell = strataReal(value)
			}
			row.Covers = append(row.Covers, cell)
		}
		prepared.ReducedRows = append(prepared.ReducedRows, row)
	}
	return fixture, prepared
}

func summaryRawGroupReferences(t *testing.T, rows [][]any) ProjectMetadataTable {
	t.Helper()
	table := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Code"}, {Name: "EnglishName"}, {Name: "Lifeform"}, {Name: "ScientificName"}, {Name: "Codetype"},
	}, Rows: []ProjectMetadataRow{}}
	for i, row := range rows {
		cells := []ProjectMetadataCell{}
		for _, value := range row {
			switch value := value.(type) {
			case string:
				cells = append(cells, metadataText(value))
			case float64:
				integer := siteUnitNumericString(value)
				cells = append(cells, ProjectMetadataCell{Storage: "integer", Integer: &integer})
			case nil:
				cells = append(cells, ProjectMetadataCell{Storage: "null"})
			default:
				t.Fatal("reference source type lost")
			}
		}
		table.Rows = append(table.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
	}
	return table
}

func TestSummaryVegetationGroupedRawOriginalBuilders(t *testing.T) {
	fixture, prepared := summaryRawGroupFixture(t)
	if len(fixture.Cases) != 6 || len(prepared.ReducedRows) != 10 {
		t.Fatal("independent raw source cases lost")
	}
	count := 0
	for _, test := range fixture.Cases {
		t.Run(test.Builder+"/"+test.Scenario, func(t *testing.T) {
			grouping := map[string]int{"BuildStrataTable": 12, "BuildLifeFormTable": 13, "ConvertProVeg2V2Null": 14}[test.Builder]
			union := summaryRawGroupReferences(t, fixture.References[test.Scenario])
			got, err := prepareSummaryVegetationGroupedEntries(context.Background(), prepared, union, grouping)
			if err != nil {
				t.Fatal(err)
			}
			typeNames := map[int]string{10: "TEXT", 6: "SINGLE"}
			if len(test.Fields) != len(got.Columns) {
				t.Fatal("source field count lost")
			}
			for i, field := range test.Fields {
				expected := typeNames[field.Type]
				if field.Type == 10 {
					expected += "(" + siteUnitNumericString(float64(field.Size)) + ")"
				}
				if got.Columns[i].Name != field.Name || got.Columns[i].DeclaredType != expected {
					t.Fatal("source field metadata differs", got.Columns[i], field)
				}
			}
			sort.Slice(got.Rows, func(i, j int) bool {
				a, b := got.Rows[i].Cells, got.Rows[j].Cells
				for _, index := range []int{0, 1, 2} {
					x, y := vegetationTextKey(a[index]), vegetationTextKey(b[index])
					if x != y {
						return x < y
					}
				}
				return false
			})
			if len(got.Rows) != len(test.Rows) {
				t.Fatal("source row count differs", len(got.Rows), len(test.Rows))
			}
			for i, row := range got.Rows {
				values := []any{}
				for _, cell := range row.Cells {
					switch cell.Storage {
					case "text":
						values = append(values, *cell.Text)
					case "real":
						values = append(values, *cell.Real)
					case "null":
						values = append(values, nil)
					default:
						t.Fatal("source entry scalar type lost")
					}
				}
				bytes := make([]byte, 4)
				binary.LittleEndian.PutUint32(bytes, math.Float32bits(float32(*row.Cells[3].Real)))
				if !reflect.DeepEqual(values, test.Rows[i].Values) || hex.EncodeToString(bytes) != test.Rows[i].Bits {
					t.Fatal("independent raw row/SINGLE bits differ", values, test.Rows[i])
				}
			}
			count += len(got.Rows)
		})
	}
	if count != 58 {
		t.Fatal("source 58-row coverage lost", count)
	}
}

func TestSummaryVegetationGroupedRawOwnershipGuardsAndCancellation(t *testing.T) {
	for _, grouping := range []int{12, 13, 14} {
		fixture, prepared := summaryRawGroupFixture(t)
		union := summaryRawGroupReferences(t, fixture.References["fanout"])
		before, err := json.Marshal(prepared)
		if err != nil {
			t.Fatal(err)
		}
		got, err := prepareSummaryVegetationGroupedEntries(context.Background(), prepared, union, grouping)
		if err != nil {
			t.Fatal(err)
		}
		*got.Rows[0].Cells[2].Text = "changed"
		*got.Rows[0].Cells[3].Real = 7
		after, err := json.Marshal(prepared)
		if err != nil || string(before) != string(after) {
			t.Fatal("entry output aliases input", err)
		}
		probe := &projectUnitCancellationContext{Context: context.Background()}
		if _, err := prepareSummaryVegetationGroupedEntries(probe, prepared, union, grouping); err != nil {
			t.Fatal(err)
		}
		for point := 1; point <= probe.calls; point++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
			got, err := prepareSummaryVegetationGroupedEntries(ctx, prepared, union, grouping)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, ProjectMetadataTable{}) {
				t.Fatal("cancelled entries leaked output", grouping, point, err)
			}
		}
		for _, bad := range []string{"plot", "species", "malformed", "duplicate", "provenance", "covers", "excluded-overflow", "project", "columns"} {
			_, altered := summaryRawGroupFixture(t)
			switch bad {
			case "plot":
				altered.ReducedRows[0].PlotNumber = "12345678"
			case "species":
				altered.ReducedRows[0].Species = metadataText("123456789")
			case "malformed":
				altered.ReducedRows[0].Species = metadataText(string([]byte{0xff}))
			case "duplicate":
				altered.ReducedRows = append(altered.ReducedRows, altered.ReducedRows[0])
			case "provenance":
				altered.ReducedRows[0].SourceRowIDs = nil
			case "covers":
				altered.ReducedRows[0].Covers = nil
			case "excluded-overflow":
				altered.ReducedRows[0].Covers[12] = strataReal(math.MaxFloat64)
			case "project":
				altered.Project = strings.Repeat("P", 21)
			case "columns":
				altered.CoverColumns[0] = "cover1"
			}
			got, err := prepareSummaryVegetationGroupedEntries(context.Background(), altered, union, grouping)
			if err == nil || !reflect.DeepEqual(got, ProjectMetadataTable{}) {
				t.Fatal("invalid raw staging accepted", grouping, bad, got, err)
			}
			if _, err := prepareSummaryVegetationGroupedEntries(context.Background(), prepared, union, grouping); err != nil {
				t.Fatal("valid correction failed", err)
			}
		}
	}
	fixture, prepared := summaryRawGroupFixture(t)
	union := summaryRawGroupReferences(t, fixture.References["ordinary"])
	if _, err := prepareSummaryVegetationGroupedEntries(nil, prepared, union, 12); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := prepareSummaryVegetationGroupedEntries(context.Background(), prepared, union, 11); err == nil {
		t.Fatal("unverified grouping accepted")
	}
}
