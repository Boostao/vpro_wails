package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func siviProjectionFixture() ProjectMetadataTable {
	real := func(value float64) ProjectMetadataCell { return ProjectMetadataCell{Storage: "real", Real: &value} }
	names := []string{"ID", "PlotNumber", "Species", "Cover1", "Cover2", "Cover3", "TotalA", "HeightA",
		"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "HeightB", "Collected",
		"Cover6", "Height6", "Cover7", "Cover8", "Cover9"}
	table := ProjectMetadataTable{}
	for _, name := range names {
		table.Columns = append(table.Columns, ProjectMetadataColumn{Name: name})
	}
	for _, values := range []map[string]ProjectMetadataCell{
		{"Cover1": real(0), "HeightA": real(2), "HeightB": metadataText("")},
		{"Cover5a": real(-1), "HeightB": metadataText("  historical words  ")},
		{"HeightA": real(5), "HeightB": metadataText("height-only")},
		{"Cover6": real(0), "Height6": real(-3)},
		{"Height6": real(3)},
		{"Cover9": real(0)},
		{"Cover5c": real(120), "Cover6": real(2), "Cover7": real(-1), "HeightB": metadataInteger("1")},
		{"PlotNumber": metadataText("Other"), "Cover1": real(4)},
		{"PlotNumber": {Storage: "null"}, "Cover1": real(4)},
	} {
		row := ProjectMetadataRow{RowID: fmt.Sprint(len(table.Rows) + 1)}
		for _, name := range names {
			cell := ProjectMetadataCell{Storage: "null"}
			switch name {
			case "ID":
				cell = metadataInteger("0")
			case "PlotNumber":
				cell = metadataText("P")
			case "Species":
				cell = metadataText("S")
			}
			if value, present := values[name]; present {
				cell = value
			}
			row.Cells = append(row.Cells, cell)
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}

func TestSIVIProjectionPreservesPhysicalRowsAndCoverMembership(t *testing.T) {
	table := siviProjectionFixture()
	for _, extended := range []bool{false, true} {
		got, err := projectSIVIVegetation(context.Background(), "P", extended, table)
		if err != nil || len(got) != 3 {
			t.Fatal(got, err)
		}
		form := "SubVegA-SIVI_BC"
		if extended {
			form = "SubVegA-SIVI"
		}
		if got[0].Form != form || got[0].Query != "USysVegA" || len(got[0].Columns) != 16 ||
			got[1].Form != "SubVegC-SIVI" || got[2].Form != "SubVegD-SIVI" {
			t.Fatal("source identity/query changed", got)
		}
		for i, expected := range [][]string{{"1", "2", "7"}, {"4", "7"}, {"6", "7"}} {
			ids := []string{}
			for _, row := range got[i].Rows {
				ids = append(ids, row.RowID)
				if *row.Cells[0].Integer != "0" {
					t.Fatal("physical application ID was replaced with rowid")
				}
			}
			if !reflect.DeepEqual(ids, expected) {
				t.Fatal("zero/negative/extended-only membership lost or height-only rows invented", i, ids)
			}
		}
		b := -1
		for i, column := range got[0].Columns {
			if column == "HeightB" {
				b = i
			}
		}
		if b < 0 || *got[0].Rows[0].Cells[b].Text != "" ||
			*got[0].Rows[1].Cells[b].Text != "  historical words  " ||
			got[0].Rows[2].Cells[b].Storage != "integer" {
			t.Fatal("HeightB empty/literal/historical invalid storage was coerced", got[0])
		}
	}
}

func TestSIVIProjectionClonesRawCellsAndRetainsNULLHeightB(t *testing.T) {
	table := siviProjectionFixture()
	got, err := projectSIVIVegetation(context.Background(), "P", false, table)
	if err != nil {
		t.Fatal(err)
	}
	got[0].Columns[0] = "changed"
	got[0].Rows[0].RowID = "changed"
	*got[0].Rows[0].Cells[2].Text = "changed"
	*got[0].Rows[0].Cells[7].Real = 99
	*got[0].Rows[1].Cells[14].Text = "changed"
	again, err := projectSIVIVegetation(context.Background(), "P", true, table)
	if err != nil || again[0].Columns[0] != "ID" || again[0].Rows[0].RowID != "1" ||
		*again[0].Rows[0].Cells[2].Text != "S" || *again[0].Rows[0].Cells[7].Real != 2 ||
		*again[0].Rows[1].Cells[14].Text != "  historical words  " {
		t.Fatal("SIVI source/output aliases", again, err)
	}
	table.Rows[0].Cells[14] = ProjectMetadataCell{Storage: "null"}
	again, err = projectSIVIVegetation(context.Background(), "P", false, table)
	if err != nil || again[0].Rows[0].Cells[14].Storage != "null" {
		t.Fatal("HeightB NULL repaired to empty text", again, err)
	}
}

func TestSIVIProjectionErrorsCancellationAndLiteralPlotScope(t *testing.T) {
	for _, change := range []func(*ProjectMetadataTable){
		func(t *ProjectMetadataTable) { t.Columns = t.Columns[:len(t.Columns)-1] },
		func(t *ProjectMetadataTable) { t.Columns[0].Name = "PlotNumber" },
		func(t *ProjectMetadataTable) { t.Rows[0].Cells = nil },
		func(t *ProjectMetadataTable) { t.Rows[1].RowID = t.Rows[0].RowID },
		func(t *ProjectMetadataTable) { t.Rows[0].Cells[1] = metadataInteger("1") },
	} {
		table := siviProjectionFixture()
		change(&table)
		got, err := projectSIVIVegetation(context.Background(), "P", false, table)
		if err == nil || got != nil {
			t.Fatal("malformed snapshot returned partial output", got, err)
		}
	}
	for _, plot := range []string{"P\x00", "\xff"} {
		if got, err := projectSIVIVegetation(context.Background(), plot, false, siviProjectionFixture()); err == nil || got != nil {
			t.Fatal("malformed literal plot accepted", got, err)
		}
	}
	for _, remaining := range []int{1, 6, 20} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		got, err := projectSIVIVegetation(ctx, "P", false, siviProjectionFixture())
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled snapshot returned partial output", got, err)
		}
	}
	got, err := projectSIVIVegetation(context.Background(), "p", false, siviProjectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range got {
		if len(group.Rows) != 0 {
			t.Fatal("literal typed plot scope was silently recased")
		}
	}
}
