package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryEnvironmentalWorkbookFixture(t *testing.T) summaryEnvironmentalWorkbookInput {
	t.Helper()
	fields, units, rows := summaryEnvironmentalHeaderFixture(t, true)
	return summaryEnvironmentalWorkbookInput{FirstRow: 5, FieldList: fields, Units: units, Rows: rows}
}

func openSummaryEnvironmentalWorkbook(t *testing.T, result summaryEnvironmentalWorkbook) *excelize.File {
	t.Helper()
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	})
	return book
}

func summaryEnvironmentalWorkbookSource(t *testing.T, book *excelize.File) []byte {
	t.Helper()
	var encoded strings.Builder
	for row := 1; ; row++ {
		value, err := book.GetCellValue("_VPRO_Source", "A"+strconv.Itoa(row))
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		if len(value) > 30000 || len(value)%2 != 0 {
			t.Fatal("provenance chunk exceeds its exact hex boundary")
		}
		encoded.WriteString(value)
	}
	raw, err := hex.DecodeString(encoded.String())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSummaryEnvironmentalWorkbookPlacementTagsAndDeterminism(t *testing.T) {
	for _, english := range []bool{false, true} {
		t.Run(fmt.Sprint(english), func(t *testing.T) {
			input := summaryEnvironmentalWorkbookFixture(t)
			input.English = english
			before, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			result, err := prepareSummaryEnvironmentalWorkbook(t.Context(), input)
			if err != nil || len(result.Bytes) == 0 || result.Header.NextRow != 11 || len(result.Header.Fields) != 4 {
				t.Fatal("complete private header workbook unavailable", err)
			}
			book := openSummaryEnvironmentalWorkbook(t, result)
			first, second := "D", "E"
			if english {
				first, second = "E", "F"
			}
			for _, expected := range []struct{ address, value string }{
				{first + "4", "A"}, {second + "4", "B"},
				{"A5", "Env"}, {"B5", "Null order"}, {"B6", "Negative order"},
				{"C7", "n Plots"}, {"C8", "N Plots"},
				{first + "5", ""}, {second + "5", ""},
				{first + "6", "0007"}, {second + "6", "0007"},
				{first + "7", "0"}, {second + "7", "0"},
				{first + "8", ""}, {second + "8", ""},
			} {
				value, err := book.GetCellValue("Summary Environment", expected.address, excelize.Options{RawCellValue: true})
				if err != nil || value != expected.value {
					t.Fatal("independent placement/value differs", expected, value, err)
				}
				formula, err := book.GetCellFormula("Summary Environment", expected.address)
				if err != nil || formula != "" {
					t.Fatal("literal cell acquired a formula", expected.address, formula, err)
				}
			}
			for _, address := range []string{first + "6", first + "8"} {
				kind, err := book.GetCellType("Summary Environment", address)
				if err != nil || kind != excelize.CellTypeSharedString {
					t.Fatal("leading zero or blank text lost its text type", address, kind, err)
				}
			}
			kind, err := book.GetCellType("Summary Environment", first+"7")
			if err != nil || kind != excelize.CellTypeUnset {
				t.Fatal("numeric zero became text or a Boolean", kind, err)
			}
			raw := summaryEnvironmentalWorkbookSource(t, book)
			var restored summaryEnvironmentalWorkbookInput
			if !bytes.Equal(raw, before) || json.Unmarshal(raw, &restored) != nil || !reflect.DeepEqual(restored, input) {
				t.Fatal("Empty, NULL, zero, blank text or complete hidden field provenance changed")
			}
			after, _ := json.Marshal(input)
			repeated, err := prepareSummaryEnvironmentalWorkbook(t.Context(), input)
			if err != nil || !bytes.Equal(result.Bytes, repeated.Bytes) || !bytes.Equal(before, after) {
				t.Fatal("preparation changed input or nondeterministic workbook bytes", err)
			}
			archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
			if err != nil {
				t.Fatal(err)
			}
			hidden := false
			for _, file := range archive.File {
				if file.Name != "xl/workbook.xml" {
					continue
				}
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				content, readErr := io.ReadAll(reader)
				if err := errors.Join(readErr, reader.Close()); err != nil {
					t.Fatal(err)
				}
				hidden = strings.Contains(string(content), `name="_VPRO_Source"`) && strings.Contains(string(content), `state="veryHidden"`)
			}
			if !hidden {
				t.Fatal("typed provenance is not very hidden")
			}
		})
	}
}

func TestSummaryEnvironmentalWorkbookLiteralStringsAndPrecision(t *testing.T) {
	values := []string{"=SUM(A1:A2)", "+1", "-2", "@name", "001", "1E20", "1 - 2 - 3", "1/2",
		"_x0001_", "_x005F_x0001_ _xFFFF_", " x \r\n\t\U0001f603 ", "<>&", strings.Repeat("\U0001f603", 8000)}
	for _, value := range values {
		t.Run(value[:min(len(value), 24)], func(t *testing.T) {
			input := summaryEnvironmentalWorkbookFixture(t)
			input.Rows[1].Values[0].Cell.Text = &value
			result, err := prepareSummaryEnvironmentalWorkbook(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			book := openSummaryEnvironmentalWorkbook(t, result)
			actual, err := book.GetCellValue("Summary Environment", "D6", excelize.Options{RawCellValue: true})
			formula, formulaErr := book.GetCellFormula("Summary Environment", "D6")
			if err != nil || formulaErr != nil || actual != value || formula != "" {
				t.Fatal("reopened literal text was coerced/repaired", actual, err, formulaErr)
			}
			if !bytes.Contains(summaryEnvironmentalWorkbookSource(t, book), []byte(`"firstRow":5`)) {
				t.Fatal("complete input provenance unavailable")
			}
		})
	}
	for _, value := range []string{"999999999999999", "-999999999999999"} {
		input := summaryEnvironmentalWorkbookFixture(t)
		input.Rows[2].Values[0].Cell.Integer = &value
		result, err := prepareSummaryEnvironmentalWorkbook(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		book := openSummaryEnvironmentalWorkbook(t, result)
		actual, err := book.GetCellValue("Summary Environment", "D7", excelize.Options{RawCellValue: true})
		if err != nil || actual != value {
			t.Fatal("fifteen-digit integer boundary changed", actual, err)
		}
	}
}

func TestSummaryEnvironmentalWorkbookRefusesWithoutPartialBytes(t *testing.T) {
	cases := []struct {
		name string
		edit func(*summaryEnvironmentalWorkbookInput)
	}{
		{"integer-positive-precision", func(p *summaryEnvironmentalWorkbookInput) {
			value := "1000000000000000"
			p.Rows[2].Values[0].Cell.Integer = &value
		}},
		{"integer-negative-precision", func(p *summaryEnvironmentalWorkbookInput) {
			value := "-1000000000000000"
			p.Rows[2].Values[0].Cell.Integer = &value
		}},
		{"unrepresentable-escape", func(p *summaryEnvironmentalWorkbookInput) { value := "_xD800_"; p.Rows[1].Values[0].Cell.Text = &value }},
		{"malformed-unicode", func(p *summaryEnvironmentalWorkbookInput) {
			value := string([]byte{0xff})
			p.Rows[1].Values[0].Cell.Text = &value
		}},
		{"overlength-text", func(p *summaryEnvironmentalWorkbookInput) {
			value := strings.Repeat("\U0001f603", 16384)
			p.Rows[1].Values[0].Cell.Text = &value
		}},
		{"hidden-invalid-source", func(p *summaryEnvironmentalWorkbookInput) {
			p.FieldList.Rows[4].Cells[2].Text = metadataText(string([]byte{0xff})).Text
		}},
		{"partial-matrix", func(p *summaryEnvironmentalWorkbookInput) { p.Rows = p.Rows[:3] }},
		{"changed-unit-count", func(p *summaryEnvironmentalWorkbookInput) { p.Units = p.Units[:1] }},
		{"invalid-tag", func(p *summaryEnvironmentalWorkbookInput) {
			p.Rows[0].Values[0] = SummaryVegetationEnvironmentalValue{Kind: "empty", Cell: &ProjectMetadataCell{Storage: "null"}}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := summaryEnvironmentalWorkbookFixture(t)
			test.edit(&input)
			result, err := prepareSummaryEnvironmentalWorkbook(t.Context(), input)
			if err == nil || !reflect.DeepEqual(result, summaryEnvironmentalWorkbook{}) {
				t.Fatal("invalid input returned success or partial output", err)
			}
		})
	}
	for _, ctx := range []context.Context{nil, &summaryHeaderCancellingContext{Context: context.Background(), remaining: 10}} {
		result, err := prepareSummaryEnvironmentalWorkbook(ctx, summaryEnvironmentalWorkbookFixture(t))
		if err == nil || !reflect.DeepEqual(result, summaryEnvironmentalWorkbook{}) {
			t.Fatal("missing/cancelled context returned partial output", err)
		}
		if ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation identity lost", err)
		}
	}
}

func TestSummaryEnvironmentalWorkbookCancellationAfterPlacementAndSerialization(t *testing.T) {
	input := summaryEnvironmentalWorkbookFixture(t)
	headerContext := &summaryHeaderCancellingContext{Context: context.Background(), remaining: 1000}
	if _, err := prepareSummaryEnvironmentalHeader(headerContext, input.FirstRow, input.English, input.FieldList, input.Units, input.Rows); err != nil {
		t.Fatal(err)
	}
	headerChecks := 1000 - headerContext.remaining
	completeContext := &summaryHeaderCancellingContext{Context: context.Background(), remaining: 1000}
	if result, err := prepareSummaryEnvironmentalWorkbook(completeContext, input); err != nil || len(result.Bytes) == 0 {
		t.Fatal("uncancelled calibration failed", err)
	}
	completeChecks := 1000 - completeContext.remaining
	if completeChecks <= headerChecks+3 {
		t.Fatal("render and post-serialization cancellation checkpoints are not separate")
	}
	for _, at := range []int{headerChecks + 2, completeChecks - 1, completeChecks} {
		ctx := &summaryHeaderCancellingContext{Context: context.Background(), remaining: at}
		result, err := prepareSummaryEnvironmentalWorkbook(ctx, input)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryEnvironmentalWorkbook{}) {
			t.Fatal("cancellation during placement/after serialization/cleanup returned partial bytes", at, err)
		}
	}
}
