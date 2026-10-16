package main

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestReportWorkbookSourcePoliciesRemainDistinct(t *testing.T) {
	canonical := func(cell ProjectMetadataCell) error {
		_, err := metadataCellValue(cell)
		return err
	}
	integer, real, blob := "12", 1.5, "00ff"
	for _, cell := range []ProjectMetadataCell{
		{Storage: "integer", Integer: &integer}, {Storage: "real", Real: &real}, {Storage: "blob", BlobHex: &blob},
	} {
		if err := reportWorkbookSourceValue(reflect.ValueOf(cell), canonical, "Summary environmental"); err != nil {
			t.Fatal("canonical typed provenance rejected", cell, err)
		}
		if err := lifeformWorkbookSourceValue(reflect.ValueOf(cell)); err == nil {
			t.Fatal("existing text/NULL-only Lifeform source policy was broadened", cell)
		}
	}
	for _, cell := range []ProjectMetadataCell{{Storage: "null"}, metadataText("001")} {
		if err := lifeformWorkbookSourceValue(reflect.ValueOf(cell)); err != nil {
			t.Fatal("existing identity provenance narrowed", err)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		input := struct{ Values []float64 }{[]float64{value}}
		err := lifeformWorkbookSourceValue(reflect.ValueOf(input))
		if err == nil || err.Error() != "Lifeform workbook requires finite source numbers" {
			t.Fatal("existing finite-number error changed", err)
		}
	}
	malformed := integer
	cell := ProjectMetadataCell{Storage: "null", Integer: &malformed}
	if err := reportWorkbookSourceValue(reflect.ValueOf(cell), canonical, "Summary environmental"); err == nil {
		t.Fatal("mixed typed payload accepted")
	}
}

func TestReportWorkbookLiteralTextReopenedAndFailures(t *testing.T) {
	book := excelize.NewFile()
	defer func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	}()
	for address, value := range map[string]string{"A1": "_x0001_", "A2": "=1+1", "A3": "1 - 2 - 3", "A4": "0001"} {
		if err := setReportWorkbookText(book, "Sheet1", address, value); err != nil {
			t.Fatal(err)
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := excelize.OpenReader(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	for address, expected := range map[string]string{"A1": "_x0001_", "A2": "=1+1", "A3": "1 - 2 - 3", "A4": "0001"} {
		actual, err := reopened.GetCellValue("Sheet1", address, excelize.Options{RawCellValue: true})
		formula, formulaErr := reopened.GetCellFormula("Sheet1", address)
		if err != nil || formulaErr != nil || actual != expected || formula != "" {
			t.Fatal("shared literal writer changed reopened source", address, actual, err, formulaErr)
		}
	}
	for _, test := range []struct{ sheet, address, value string }{
		{"Sheet1", "B1", "_xD800_"}, {"Sheet1", "B1", string([]byte{0xff})},
		{"Sheet1", "B1", "\x00"}, {"missing", "B1", "valid"}, {"Sheet1", "invalid-cell", "valid"},
	} {
		if err := setReportWorkbookText(book, test.sheet, test.address, test.value); err == nil {
			t.Fatal("invalid text/destination was silently accepted", test.address)
		}
	}
}
