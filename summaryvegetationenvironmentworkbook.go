package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type summaryEnvironmentalWorkbookInput struct {
	FirstRow  int                                        `json:"firstRow"`
	English   bool                                       `json:"english"`
	FieldList ProjectMetadataTable                       `json:"fieldList"`
	Units     []summaryVegetationUnit                    `json:"units"`
	Rows      []SummaryVegetationEnvironmentalNumericRow `json:"rows"`
}

type summaryEnvironmentalWorkbook struct {
	Bytes  []byte
	Header summaryEnvironmentalHeaderPlan
}

// This private inspection preserves typed values, not Excel automation coercion.
func prepareSummaryEnvironmentalWorkbook(ctx context.Context, input summaryEnvironmentalWorkbookInput) (result summaryEnvironmentalWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Summary environmental workbook requires a context")
	}
	plan, err := prepareSummaryEnvironmentalHeader(ctx, input.FirstRow, input.English, input.FieldList, input.Units, input.Rows)
	if err != nil {
		return result, err
	}
	if err := reportWorkbookSourceValue(reflect.ValueOf(input), func(cell ProjectMetadataCell) error {
		_, err := metadataCellValue(cell)
		return err
	}, "Summary environmental"); err != nil {
		return result, fmt.Errorf("Summary environmental workbook provenance: %w", err)
	}
	book := excelize.NewFile()
	defer func() {
		resultErr = errors.Join(resultErr, book.Close(), ctx.Err())
		if resultErr != nil {
			result = summaryEnvironmentalWorkbook{}
		}
	}()
	const sheet = "Summary Environment"
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return result, err
	}
	if err := book.SetDocProps(&excelize.DocProperties{Title: "Summary environmental inspection", Creator: "VPRO",
		Created: "2000-01-01T00:00:00Z", Modified: "2000-01-01T00:00:00Z"}); err != nil {
		return result, err
	}
	style, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 8},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return result, err
	}
	heading, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 8, Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E5E7EB"}},
		Alignment: &excelize.Alignment{WrapText: true, Horizontal: "center"}})
	if err != nil {
		return result, err
	}
	last, err := excelize.CoordinatesToCellName(plan.LastValueColumn, max(input.FirstRow-1, plan.LastDataRow))
	if err != nil {
		return result, err
	}
	if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", input.FirstRow-1), last, style); err != nil {
		return result, err
	}
	lastColumn, err := excelize.ColumnNumberToName(plan.LastValueColumn)
	if err != nil {
		return result, err
	}
	for _, width := range []struct {
		first, last string
		value       float64
	}{{"A", "A", 18}, {"B", "B", 32}, {"C", lastColumn, 12}} {
		if err := book.SetColWidth(sheet, width.first, width.last, width.value); err != nil {
			return result, err
		}
	}
	for _, placed := range plan.Cells {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		address, err := excelize.CoordinatesToCellName(placed.Column, placed.Row)
		if err != nil {
			return result, err
		}
		if placed.Value.Kind == "cell" {
			cell := placed.Value.Cell
			switch cell.Storage {
			case "text":
				if err := setReportWorkbookText(book, sheet, address, *cell.Text); err != nil {
					return result, fmt.Errorf("Summary environmental workbook %s: %w", address, err)
				}
			case "integer":
				value, err := strconv.ParseInt(*cell.Integer, 10, 64)
				if err != nil {
					return result, err
				}
				if value < -999999999999999 || value > 999999999999999 {
					return result, fmt.Errorf("Summary environmental workbook %s integer exceeds Excel's fifteen-digit precision; no rounding", address)
				}
				if err := book.SetCellInt(sheet, address, value); err != nil {
					return result, err
				}
			}
		}
		if placed.Row == input.FirstRow-1 {
			if err := book.SetCellStyle(sheet, address, address, heading); err != nil {
				return result, err
			}
		}
	}
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	source, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	encoded := hex.EncodeToString(source)
	for offset, row := 0, 1; offset < len(encoded); offset, row = offset+30000, row+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if row > 1048576 {
			return result, errors.New("Summary environmental workbook provenance exceeds Excel's row limit")
		}
		if err := book.SetCellStr("_VPRO_Source", "A"+strconv.Itoa(row), encoded[offset:min(offset+30000, len(encoded))]); err != nil {
			return result, err
		}
	}
	book.SetActiveSheet(0)
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
	return summaryEnvironmentalWorkbook{Bytes: bytes.Clone(buffer.Bytes()), Header: plan}, nil
}
