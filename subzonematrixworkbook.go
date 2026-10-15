package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/reports/subzonematrix"
	"github.com/xuri/excelize/v2"
)

const subzoneMatrixWorksheet = "qrySubzoneMatrix3"

type subzoneMatrixWorkbook struct {
	Bytes        []byte
	SourceSHA256 string
}

func subzoneMatrixOriginalSHA256(source subzoneMatrixReview) (string, error) {
	original, err := json.Marshal(struct {
		Hierarchy, Labels ProjectMetadataTable
	}{source.HierarchyOriginal, source.LabelsOriginal})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(original)
	return hex.EncodeToString(digest[:]), nil
}

func prepareSubzoneMatrixWorkbook(ctx context.Context, source subzoneMatrixReview) (result subzoneMatrixWorkbook, resultErr error) {
	return prepareSubzoneMatrixWorkbookInDirectory(ctx, source, "")
}

func prepareSubzoneMatrixWorkbookInDirectory(ctx context.Context, source subzoneMatrixReview, parent string) (result subzoneMatrixWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Subzone Matrix workbook requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for _, scope := range []string{source.ContextID, source.Hierarchy, source.HierarchyPath, source.LabelsPath} {
		if scope == "" {
			return result, errors.New("Subzone Matrix workbook requires complete observed source scope")
		}
		if err := environmentWorkbookText(scope); err != nil {
			return result, err
		}
	}
	if source.Hierarchy == "None" {
		return result, errors.New("Subzone Matrix workbook requires a selected hierarchy")
	}
	rows, labels, err := subzoneMatrixInputs(ctx, source.HierarchyOriginal, source.LabelsOriginal)
	if err != nil {
		return result, err
	}
	expected, err := subzonematrix.Plan(ctx, rows, labels)
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(expected, source.Matrix) {
		return result, errors.New("Subzone Matrix workbook differs from its complete original plan")
	}
	if len(expected.Rows) == 0 {
		return result, errors.New("Nothing to report. Ensure that your units are contained in USysSubzoneMatrixLabels")
	}
	widths := make([]float64, len(expected.Columns)+1)
	for i := range widths {
		widths[i] = 8
	}
	measure := func(column int, text *string) error {
		if text == nil {
			return nil
		}
		if err := environmentWorkbookText(*text); err != nil {
			return err
		}
		widths[column] = max(widths[column], min(float64(len([]rune(*text)))+2, 80))
		return nil
	}
	for _, row := range expected.Rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := measure(0, row.Name); err != nil {
			return result, err
		}
		for i, value := range row.Values {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if err := measure(i+1, value); err != nil {
				return result, err
			}
		}
	}
	for _, table := range []ProjectMetadataTable{source.HierarchyOriginal, source.LabelsOriginal} {
		for _, column := range table.Columns {
			if !utf8.ValidString(column.Name) || !utf8.ValidString(column.DeclaredType) {
				return result, errors.New("Subzone Matrix original schema contains malformed Unicode")
			}
		}
		for _, row := range table.Rows {
			for _, cell := range row.Cells {
				if err := ctx.Err(); err != nil {
					return result, err
				}
				if _, err := metadataCellValue(cell); err != nil {
					return result, err
				}
			}
		}
	}
	sourceSHA256, err := subzoneMatrixOriginalSHA256(source)
	if err != nil {
		return result, err
	}
	provenance, err := json.Marshal(struct {
		ContextID, Hierarchy, HierarchyPath, LabelsPath, OriginalSourceSHA256 string
		Comparison, Ordering                                                  string
		Matrix                                                                subzonematrix.Matrix
	}{source.ContextID, source.Hierarchy, source.HierarchyPath, source.LabelsPath,
		sourceSHA256, "controlled-ASCII-case/trailing-space; no general locale",
		"technical-ASCII-key-order/NULL-first", expected})
	if err != nil {
		return result, err
	}
	directory, err := os.MkdirTemp(parent, "vpro-subzone-matrix-")
	if err != nil {
		return result, fmt.Errorf("create Subzone Matrix worksheet staging: %w", err)
	}
	book := excelize.NewFile(excelize.Options{TmpDir: directory})
	defer func() {
		resultErr = errors.Join(resultErr, book.Close())
		if err := os.RemoveAll(directory); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("clean up Subzone Matrix worksheet staging %q: %w", directory, err))
		}
		if resultErr != nil {
			result = subzoneMatrixWorkbook{}
		}
	}()
	if err := book.SetDocProps(&excelize.DocProperties{
		Title: "Subzone/Association Matrix", Creator: "VPRO",
		Created: "2000-01-01T00:00:00Z", Modified: "2000-01-01T00:00:00Z",
	}); err != nil {
		return result, err
	}
	if err := book.SetSheetName("Sheet1", subzoneMatrixWorksheet); err != nil {
		return result, err
	}
	heading, err := book.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{TextRotation: 90, Vertical: "bottom"}})
	if err != nil {
		return result, err
	}
	body, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "bottom"}})
	if err != nil {
		return result, err
	}
	stream, err := book.NewStreamWriter(subzoneMatrixWorksheet)
	if err != nil {
		return result, err
	}
	for i, width := range widths {
		if err := stream.SetColWidth(i+1, i+1, width); err != nil {
			return result, err
		}
	}
	if err := stream.SetPanes(&excelize.Panes{Freeze: true, XSplit: 1, YSplit: 1, TopLeftCell: "B2", ActivePane: "bottomRight"}); err != nil {
		return result, err
	}
	header := make([]any, len(widths))
	for i, caption := range expected.Columns {
		header[i+1] = excelize.Cell{StyleID: heading, Value: caption}
	}
	if err := stream.SetRow("A1", header, excelize.RowOpts{Height: 105}); err != nil {
		return result, err
	}
	for i, row := range expected.Rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		values := make([]any, len(widths))
		if row.Name != nil {
			values[0] = *row.Name
		}
		for column, value := range row.Values {
			cell := excelize.Cell{StyleID: body}
			if value != nil {
				cell.Value = *value
			}
			values[column+1] = cell
		}
		if err := stream.SetRow("A"+strconv.Itoa(i+2), values); err != nil {
			return result, err
		}
	}
	if err := stream.Flush(); err != nil {
		return result, err
	}
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	encoded := hex.EncodeToString(provenance)
	for offset, row := 0, 1; offset < len(encoded); offset, row = offset+30000, row+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := book.SetCellStr("_VPRO_Source", fmt.Sprintf("A%d", row), encoded[offset:min(offset+30000, len(encoded))]); err != nil {
			return result, err
		}
	}
	if err := book.SetSheetVisible("_VPRO_Source", false, true); err != nil {
		return result, err
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return subzoneMatrixWorkbook{bytes.Clone(buffer.Bytes()), sourceSHA256}, nil
}
