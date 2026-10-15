package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/reports/subzonematrix"
	"github.com/xuri/excelize/v2"
)

type subzoneMatrixSpillContext struct {
	context.Context
	parent  string
	seen    bool
	onSpill func(string) error
	failure error
	checks  int
}

func (ctx *subzoneMatrixSpillContext) Err() error {
	if ctx.failure != nil {
		return ctx.failure
	}
	if ctx.seen {
		return ctx.Context.Err()
	}
	ctx.checks++
	if ctx.checks%256 != 0 {
		return ctx.Context.Err()
	}
	directories, err := os.ReadDir(ctx.parent)
	if err != nil {
		ctx.failure = err
		return err
	}
	for _, directory := range directories {
		path := filepath.Join(ctx.parent, directory.Name())
		files, err := os.ReadDir(path)
		if err != nil {
			ctx.failure = err
			return err
		}
		for _, file := range files {
			opened, err := os.Open(filepath.Join(path, file.Name()))
			if err != nil {
				ctx.failure = err
				return err
			}
			info, statErr := opened.Stat()
			if err := errors.Join(statErr, opened.Close()); err != nil {
				ctx.failure = err
				return err
			}
			if info.Size() >= excelize.StreamChunkSize {
				ctx.seen = true
				if ctx.onSpill != nil {
					ctx.failure = ctx.onSpill(filepath.Join(path, file.Name()))
				}
				return ctx.failure
			}
		}
	}
	return ctx.Context.Err()
}

func subzoneMatrixMillionCellFixture(t *testing.T) subzoneMatrixReview {
	t.Helper()
	source := subzoneMatrixReview{
		ContextID: "large-report", Hierarchy: "Sample", HierarchyPath: "hierarchy.db", LabelsPath: "labels.db",
		HierarchyOriginal: ProjectMetadataTable{Columns: []ProjectMetadataColumn{
			{Name: "ID", DeclaredType: "INTEGER"}, {Name: "Name", DeclaredType: "TEXT"},
			{Name: "Parent", DeclaredType: "INTEGER"}, {Name: "Level", DeclaredType: "INTEGER"},
		}},
		LabelsOriginal: ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "MyUnit", DeclaredType: "TEXT"}}},
	}
	add := func(id int, name string, parent ProjectMetadataCell, level string) {
		source.HierarchyOriginal.Rows = append(source.HierarchyOriginal.Rows, ProjectMetadataRow{
			RowID: fmt.Sprint(id), Cells: []ProjectMetadataCell{metadataInteger(fmt.Sprint(id)), metadataText(name), parent, metadataInteger(level)},
		})
	}
	for column := 0; column < 250; column++ {
		label := fmt.Sprintf("U%08d1", column)
		source.LabelsOriginal.Rows = append(source.LabelsOriginal.Rows, ProjectMetadataRow{
			RowID: fmt.Sprint(column + 1), Cells: []ProjectMetadataCell{metadataText(label)},
		})
		if column >= 20 {
			add(100000+column, label+"01", metadataInteger("1"), "11")
		}
	}
	for row := 1; row <= 4000; row++ {
		add(row, fmt.Sprintf("Association %04d", row), ProjectMetadataCell{Storage: "null"}, "10")
		for column := 0; column < 20; column++ {
			add(4000+(row-1)*20+column+1,
				fmt.Sprintf("U%08d1", column)+strings.Repeat("x", 256)+fmt.Sprintf("%04d", row),
				metadataInteger(fmt.Sprint(row)), "11")
		}
	}
	rows, labels, err := subzoneMatrixInputs(context.Background(), source.HierarchyOriginal, source.LabelsOriginal)
	if err != nil {
		t.Fatal(err)
	}
	source.Matrix, err = subzonematrix.Plan(context.Background(), rows, labels)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Matrix.Rows) != 4000 || len(source.Matrix.Columns) != 250 {
		t.Fatal("fixture does not exercise exactly one million pivot cells")
	}
	return source
}

func TestSubzoneMatrixWorkbookMillionCellsSpillAndCancellation(t *testing.T) {
	source := subzoneMatrixMillionCellFixture(t)
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel-%t", cancel), func(t *testing.T) {
			parent := t.TempDir()
			ctx := &subzoneMatrixSpillContext{Context: context.Background(), parent: parent}
			if cancel {
				ctx.onSpill = func(string) error { return context.Canceled }
			}
			result, err := prepareSubzoneMatrixWorkbookInDirectory(ctx, source, parent)
			if !ctx.seen {
				t.Fatal("worksheet never spilled at the actual Excelize threshold", err)
			}
			if cancel {
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
					t.Fatal("in-flight cancellation returned partial bytes", err)
				}
			} else {
				if err != nil || len(result.Bytes) == 0 {
					t.Fatal("million-cell preparation failed", err)
				}
				archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, entry := range archive.File {
					if entry.Name != "xl/worksheets/sheet1.xml" {
						continue
					}
					found = true
					reader, err := entry.Open()
					if err != nil {
						t.Fatal(err)
					}
					decoder := xml.NewDecoder(reader)
					rows := 0
					lastRow := ""
					for {
						token, err := decoder.Token()
						if errors.Is(err, io.EOF) {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
						if start, ok := token.(xml.StartElement); ok && start.Name.Local == "row" {
							rows++
							for _, attribute := range start.Attr {
								if attribute.Name.Local == "r" {
									lastRow = attribute.Value
								}
							}
						}
					}
					if err := reader.Close(); err != nil {
						t.Fatal(err)
					}
					if rows != 4001 || lastRow != "4001" {
						t.Fatal("spilled worksheet was truncated", rows, lastRow)
					}
				}
				if !found {
					t.Fatal("spilled worksheet missing from output")
				}
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatal("owned stream staging survived preparation", entries, err)
			}
		})
	}
}

func subzoneMatrixWorkbookFixture(t *testing.T) subzoneMatrixReview {
	t.Helper()
	service, state := subzoneMatrixFixture(t)
	result, err := service.readSubzoneMatrix(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSubzoneMatrixWorkbookLiteralLayoutAndTypedProvenance(t *testing.T) {
	source := subzoneMatrixWorkbookFixture(t)
	before, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepareSubzoneMatrixWorkbook(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	}()
	if !reflect.DeepEqual(book.GetSheetList(), []string{subzoneMatrixWorksheet, "_VPRO_Source"}) {
		t.Fatal("unexpected source worksheet names", book.GetSheetList())
	}
	for address, expected := range map[string]string{
		"A1": "", "B1": "AAAAAAAAA", "C1": "BBBBBBBBB", "D1": "ZZZZZZZZZ",
		"A2": "", "B2": "", "A3": "Association Alpha", "B3": "01", "C3": "02",
		"A4": "Association Beta", "B4": "03",
	} {
		actual, err := book.GetCellValue(subzoneMatrixWorksheet, address)
		if err != nil || actual != expected {
			t.Fatal(address, actual, expected, err)
		}
		formula, err := book.GetCellFormula(subzoneMatrixWorksheet, address)
		if err != nil || formula != "" {
			t.Fatal("literal source became a formula", address, formula, err)
		}
	}
	cellType, err := book.GetCellType(subzoneMatrixWorksheet, "B3")
	if err != nil || cellType != excelize.CellTypeInlineString {
		t.Fatal("leading-zero source text became numeric", cellType, err)
	}
	for address, expected := range map[string]struct {
		rotation int
		bold     bool
	}{"B1": {90, false}, "B3": {0, true}, "D2": {0, true}} {
		id, err := book.GetCellStyle(subzoneMatrixWorksheet, address)
		if err != nil {
			t.Fatal(err)
		}
		style, err := book.GetStyle(id)
		if err != nil || style.Alignment == nil || style.Alignment.TextRotation != expected.rotation ||
			style.Alignment.Vertical != "bottom" || (style.Font != nil && style.Font.Bold) != expected.bold {
			t.Fatal("source-inspired layout differs", address, style, err)
		}
		if expected.bold && style.Alignment.Horizontal != "center" {
			t.Fatal("body alignment differs", style)
		}
	}
	panes, err := book.GetPanes(subzoneMatrixWorksheet)
	if err != nil || !panes.Freeze || panes.XSplit != 1 || panes.YSplit != 1 || panes.TopLeftCell != "B2" {
		t.Fatal("B2 freeze differs", panes, err)
	}
	visible, err := book.GetSheetVisible("_VPRO_Source")
	if err != nil || visible {
		t.Fatal("typed provenance sheet unexpectedly visible", err)
	}
	rows, err := book.GetRows("_VPRO_Source")
	if err != nil || len(rows) == 0 {
		t.Fatal("typed provenance missing", err)
	}
	var encoded strings.Builder
	for _, row := range rows {
		if len(row) != 1 || len(row[0]) > 30000 {
			t.Fatal("invalid provenance chunk")
		}
		encoded.WriteString(row[0])
	}
	raw, err := hex.DecodeString(encoded.String())
	if err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		ContextID, Hierarchy, HierarchyPath, LabelsPath, OriginalSourceSHA256 string
		Comparison, Ordering                                                  string
		Matrix                                                                subzonematrix.Matrix
	}
	if err := json.Unmarshal(raw, &provenance); err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(struct {
		Hierarchy, Labels ProjectMetadataTable
	}{source.HierarchyOriginal, source.LabelsOriginal})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(original)
	if provenance.ContextID != source.ContextID || provenance.Hierarchy != source.Hierarchy ||
		provenance.HierarchyPath != source.HierarchyPath || provenance.LabelsPath != source.LabelsPath ||
		provenance.OriginalSourceSHA256 != hex.EncodeToString(digest[:]) || result.SourceSHA256 != provenance.OriginalSourceSHA256 ||
		!reflect.DeepEqual(provenance.Matrix, source.Matrix) || !strings.Contains(provenance.Comparison, "no general locale") {
		t.Fatal("typed NULL/empty matrix or source scope/hash differs", provenance)
	}
	after, err := json.Marshal(source)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("workbook preparation mutated observed source", err)
	}
}

func TestSubzoneMatrixWorkbookRefusalsAndLiteralFormulaText(t *testing.T) {
	base := subzoneMatrixWorkbookFixture(t)
	for _, test := range []struct {
		name   string
		change func(*subzoneMatrixReview)
		want   string
	}{
		{"missing-scope", func(source *subzoneMatrixReview) { source.ContextID = "" }, "scope"},
		{"None", func(source *subzoneMatrixReview) { source.Hierarchy = "None" }, "selected hierarchy"},
		{"altered-result", func(source *subzoneMatrixReview) { source.Matrix.Columns[0] = "changed" }, "original plan"},
		{"unused-malformed-text", func(source *subzoneMatrixReview) {
			source.HierarchyOriginal.Rows[0].Cells[4] = metadataText("\xff")
		}, "malformed Unicode"},
		{"malformed-schema", func(source *subzoneMatrixReview) {
			source.HierarchyOriginal.Columns[4].DeclaredType = "\xff"
		}, "malformed Unicode"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var source subzoneMatrixReview
			if err := json.Unmarshal(raw, &source); err != nil {
				t.Fatal(err)
			}
			test.change(&source)
			result, err := prepareSubzoneMatrixWorkbook(context.Background(), source)
			if err == nil || !strings.Contains(err.Error(), test.want) || !reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
				t.Fatal("invalid result/scope was published as bytes", result.SourceSHA256, err)
			}
		})
	}
	for _, text := range []string{"=1+1", "", "é🌲", "a\r\nb"} {
		source := subzoneMatrixWorkbookFixture(t)
		indices, err := siteUnitTransferColumns(source.HierarchyOriginal, "Name")
		if err != nil {
			t.Fatal(err)
		}
		source.HierarchyOriginal.Rows[2].Cells[indices["Name"]] = metadataText("AAAAAAAAA1" + text)
		rows, labels, err := subzoneMatrixInputs(context.Background(), source.HierarchyOriginal, source.LabelsOriginal)
		if err != nil {
			t.Fatal(err)
		}
		source.Matrix, err = subzonematrix.Plan(context.Background(), rows, labels)
		if err != nil {
			t.Fatal(err)
		}
		result, err := prepareSubzoneMatrixWorkbook(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
		if err != nil {
			t.Fatal(err)
		}
		value, valueErr := book.GetCellValue(subzoneMatrixWorksheet, "B3")
		formula, formulaErr := book.GetCellFormula(subzoneMatrixWorksheet, "B3")
		closeErr := book.Close()
		if valueErr != nil || formulaErr != nil || closeErr != nil || value != text || formula != "" {
			t.Fatal("literal suffix was normalized/evaluated", text, value, formula, valueErr, formulaErr, closeErr)
		}
		for _, text := range []string{"bad\x01text", strings.Repeat("x", 32768)} {
			source := subzoneMatrixWorkbookFixture(t)
			indices, err := siteUnitTransferColumns(source.HierarchyOriginal, "Name")
			if err != nil {
				t.Fatal(err)
			}
			source.HierarchyOriginal.Rows[2].Cells[indices["Name"]] = metadataText("AAAAAAAAA1" + text)
			rows, labels, err := subzoneMatrixInputs(context.Background(), source.HierarchyOriginal, source.LabelsOriginal)
			if err != nil {
				t.Fatal(err)
			}
			source.Matrix, err = subzonematrix.Plan(context.Background(), rows, labels)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := prepareSubzoneMatrixWorkbook(context.Background(), source); err == nil ||
				!reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
				t.Fatal("invalid XML/overlength text produced workbook bytes", err)
			}
		}
	}
}

func TestSubzoneMatrixWorkbookCancellationAndNoLabels(t *testing.T) {
	source := subzoneMatrixWorkbookFixture(t)
	if result, err := prepareSubzoneMatrixWorkbook(nil, source); err == nil || !reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
		t.Fatal("nil workbook context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := prepareSubzoneMatrixWorkbook(ctx, source); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
		t.Fatal("cancelled workbook returned bytes", err)
	}
	source.LabelsOriginal.Rows = []ProjectMetadataRow{}
	rows, labels, err := subzoneMatrixInputs(context.Background(), source.HierarchyOriginal, source.LabelsOriginal)
	if err != nil {
		t.Fatal(err)
	}
	source.Matrix, err = subzonematrix.Plan(context.Background(), rows, labels)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := prepareSubzoneMatrixWorkbook(context.Background(), source); err == nil ||
		!strings.Contains(err.Error(), "Nothing to report") || !reflect.DeepEqual(result, subzoneMatrixWorkbook{}) {
		t.Fatal("source no-label report emitted a workbook", err)
	}
}
