package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLongVegetationWorkbookLifeformIntegerCellsNullGapsAndStrictStorage(t *testing.T) {
	preview := vegetationWorkbookFixture(t)
	preview.Settings.Grouping = "lifeform"
	preview.Report.Units = preview.Report.Units[:1]
	row := preview.Report.Units[0].Rows[0]
	groups := []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("-32768"), vprotest.MetadataInteger("-32768"),
		vprotest.MetadataInteger("0"), vprotest.MetadataInteger("32767"), {Storage: "null"}, vprotest.MetadataInteger("2")}
	preview.Report.Units[0].Rows = nil
	for _, group := range groups {
		next := row
		next.Layer = group
		preview.Report.Units[0].Rows = append(preview.Report.Units[0].Rows, next)
	}
	result, err := prepareLongVegetationWorkbook(context.Background(), preview,
		vegetationWorkbookLayout{QuickReport: true, SpaceBetweenGroups: true})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := book.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	sheet := result.Sheets[0].Name
	// Source starts scanning at row7; NULL stops subsequent gap insertion.
	for _, cell := range []struct{ address, value string }{
		{"A5", "-32768"}, {"A6", "-32768"}, {"A7", ""}, {"A8", "0"},
		{"A9", ""}, {"A10", "32767"}, {"A11", ""}, {"A12", "2"},
	} {
		value, err := book.GetCellValue(sheet, cell.address)
		if err != nil || value != cell.value {
			t.Fatal(cell, value, err)
		}
	}
	for _, address := range []string{"A5", "A6", "A8", "A10", "A12"} {
		kind, err := book.GetCellType(sheet, address)
		if err != nil || kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
			t.Fatal("Lifeform code was not numeric", address, kind, err)
		}
		id, err := book.GetCellStyle(sheet, address)
		if err != nil {
			t.Fatal(err)
		}
		style, err := book.GetStyle(id)
		if err != nil || style.NumFmt != 1 {
			t.Fatal("Lifeform code lost source integer format", address, style, err)
		}
	}
	for _, invalid := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("2"), vprotest.StrataReal(2), vprotest.MetadataInteger("32768"),
		vprotest.MetadataInteger("-32769"), {Storage: "integer"}, {Storage: "null", Integer: vprotest.MetadataInteger("1").Integer}} {
		preview.Report.Units[0].Rows[0].Layer = invalid
		refused, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{QuickReport: true})
		if err == nil || len(refused.Bytes) != 0 {
			t.Fatal("invalid Lifeform storage emitted workbook", invalid, err)
		}
	}
	preview.Report.Units[0].Rows[0].Layer = vprotest.MetadataInteger("2")
	preview.Report.Units[0].Rows[0].Species = vprotest.MetadataInteger("2")
	if refused, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{QuickReport: true}); err == nil || len(refused.Bytes) != 0 {
		t.Fatal("numeric Lifeform widened species identity acceptance", err)
	}
}
