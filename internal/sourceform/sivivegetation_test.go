package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"

	"reflect"
	"testing"
)

func TestSIVIProjectionPreservesPhysicalRowsAndCoverMembership(t *testing.T) {
	table := vprotest.SiviProjectionFixture()
	for _, extended := range []bool{false, true} {
		got, err := ProjectSIVIVegetation(context.Background(), "P", extended, table)
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
	table := vprotest.SiviProjectionFixture()
	got, err := ProjectSIVIVegetation(context.Background(), "P", false, table)
	if err != nil {
		t.Fatal(err)
	}
	got[0].Columns[0] = "changed"
	got[0].Rows[0].RowID = "changed"
	*got[0].Rows[0].Cells[2].Text = "changed"
	*got[0].Rows[0].Cells[7].Real = 99
	*got[0].Rows[1].Cells[14].Text = "changed"
	again, err := ProjectSIVIVegetation(context.Background(), "P", true, table)
	if err != nil || again[0].Columns[0] != "ID" || again[0].Rows[0].RowID != "1" ||
		*again[0].Rows[0].Cells[2].Text != "S" || *again[0].Rows[0].Cells[7].Real != 2 ||
		*again[0].Rows[1].Cells[14].Text != "  historical words  " {
		t.Fatal("SIVI source/output aliases", again, err)
	}
	table.Rows[0].Cells[14] = vprocore.ProjectMetadataCell{Storage: "null"}
	again, err = ProjectSIVIVegetation(context.Background(), "P", false, table)
	if err != nil || again[0].Rows[0].Cells[14].Storage != "null" {
		t.Fatal("HeightB NULL repaired to empty text", again, err)
	}
}

func TestSIVIProjectionErrorsCancellationAndLiteralPlotScope(t *testing.T) {
	for _, change := range []func(*vprocore.ProjectMetadataTable){
		func(t *vprocore.ProjectMetadataTable) { t.Columns = t.Columns[:len(t.Columns)-1] },
		func(t *vprocore.ProjectMetadataTable) { t.Columns[0].Name = "PlotNumber" },
		func(t *vprocore.ProjectMetadataTable) { t.Rows[0].Cells = nil },
		func(t *vprocore.ProjectMetadataTable) { t.Rows[1].RowID = t.Rows[0].RowID },
		func(t *vprocore.ProjectMetadataTable) { t.Rows[0].Cells[1] = vprotest.MetadataInteger("1") },
	} {
		table := vprotest.SiviProjectionFixture()
		change(&table)
		got, err := ProjectSIVIVegetation(context.Background(), "P", false, table)
		if err == nil || got != nil {
			t.Fatal("malformed snapshot returned partial output", got, err)
		}
	}
	for _, plot := range []string{"P\x00", "\xff"} {
		if got, err := ProjectSIVIVegetation(context.Background(), plot, false, vprotest.SiviProjectionFixture()); err == nil || got != nil {
			t.Fatal("malformed literal plot accepted", got, err)
		}
	}
	for _, remaining := range []int{1, 6, 20} {
		ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: remaining}
		got, err := ProjectSIVIVegetation(ctx, "P", false, vprotest.SiviProjectionFixture())
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled snapshot returned partial output", got, err)
		}
	}
	got, err := ProjectSIVIVegetation(context.Background(), "p", false, vprotest.SiviProjectionFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range got {
		if len(group.Rows) != 0 {
			t.Fatal("literal typed plot scope was silently recased")
		}
	}
}
