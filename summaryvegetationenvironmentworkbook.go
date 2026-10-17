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

type summaryWorkbookLayout struct {
	Sheet, Title                      string
	LastRow, LastColumn               int
	SeparatorRow, ItalicFromRow       int
	RealOrderFromRow, RealOrderColumn int
	HeaderRows                        []int
	Cells                             []summaryEnvironmentalHeaderCell
}

// This private inspection preserves typed values, not Excel automation coercion.
func prepareSummaryEnvironmentalWorkbook(ctx context.Context, input summaryEnvironmentalWorkbookInput) (summaryEnvironmentalWorkbook, error) {
	if ctx == nil {
		return summaryEnvironmentalWorkbook{}, errors.New("Summary environmental workbook requires a context")
	}
	plan, err := prepareSummaryEnvironmentalHeader(ctx, input.FirstRow, input.English, input.FieldList, input.Units, input.Rows)
	if err != nil {
		return summaryEnvironmentalWorkbook{}, err
	}
	data, err := renderSummaryWorkbook(ctx, input, summaryWorkbookLayout{
		Sheet: "Summary Environment", Title: "Summary environmental inspection",
		LastRow: max(input.FirstRow-1, plan.LastDataRow), LastColumn: plan.LastValueColumn,
		HeaderRows: []int{input.FirstRow - 1}, Cells: plan.Cells,
	})
	if err != nil {
		return summaryEnvironmentalWorkbook{}, err
	}
	return summaryEnvironmentalWorkbook{Bytes: data, Header: plan}, nil
}

func renderSummaryWorkbook[T any](ctx context.Context, input T, layout summaryWorkbookLayout) (result []byte, resultErr error) {
	return renderSummaryWorkbookWithSheets(ctx, input, layout, nil)
}

func renderSummaryWorkbookWithSheets(ctx context.Context, input any, layout summaryWorkbookLayout,
	writeSheets func(*excelize.File) error) (result []byte, resultErr error) {
	if ctx == nil {
		return nil, errors.New("Summary workbook rendering requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if layout.LastRow < 1 || layout.LastRow > 32767 || layout.LastColumn < 1 || layout.LastColumn > 254 {
		return nil, errors.New("Summary workbook layout exceeds the qualified source frame")
	}
	if layout.RealOrderFromRow < 0 || layout.RealOrderFromRow > layout.LastRow ||
		(layout.RealOrderFromRow == 0) != (layout.RealOrderColumn == 0) ||
		layout.RealOrderFromRow > 0 && (layout.RealOrderColumn < 3 || layout.RealOrderColumn > 4 ||
			layout.RealOrderColumn > layout.LastColumn) {
		return nil, errors.New("Summary workbook real Order range is outside the qualified frame")
	}
	if err := reportWorkbookSourceValue(reflect.ValueOf(input), func(cell ProjectMetadataCell) error {
		_, err := metadataCellValue(cell)
		return err
	}, "Summary environmental"); err != nil {
		return nil, fmt.Errorf("Summary workbook provenance: %w", err)
	}
	seen := map[[2]int]bool{}
	for _, placed := range layout.Cells {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := [2]int{placed.Row, placed.Column}
		if placed.Row < 1 || placed.Row > layout.LastRow || placed.Column < 1 || placed.Column > layout.LastColumn || seen[key] {
			return nil, errors.New("Summary workbook contains overlapping or out-of-frame cells")
		}
		seen[key] = true
		switch placed.Value.Kind {
		case "empty":
			if placed.Value.Cell != nil {
				return nil, errors.New("Summary workbook Empty must not contain a typed cell")
			}
		case "cell":
			if placed.Value.Cell == nil {
				return nil, errors.New("Summary workbook ordinary value requires a typed cell")
			}
			if _, err := metadataCellValue(*placed.Value.Cell); err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("Summary workbook requires an exact Empty or ordinary value tag")
		}
	}
	book := excelize.NewFile()
	defer func() {
		resultErr = errors.Join(resultErr, book.Close(), ctx.Err())
		if resultErr != nil {
			result = nil
		}
	}()
	sheet := layout.Sheet
	if err := book.SetSheetName("Sheet1", sheet); err != nil {
		return result, err
	}
	if err := book.SetDocProps(&excelize.DocProperties{Title: layout.Title, Creator: "VPRO",
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
	italic, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 8, Italic: true},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"}})
	if err != nil {
		return result, err
	}
	last, err := excelize.CoordinatesToCellName(layout.LastColumn, layout.LastRow)
	if err != nil {
		return result, err
	}
	if err := book.SetCellStyle(sheet, "A1", last, style); err != nil {
		return result, err
	}
	lastColumn, err := excelize.ColumnNumberToName(layout.LastColumn)
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
	if layout.SeparatorRow != 0 {
		if layout.SeparatorRow < 1 || layout.SeparatorRow > layout.LastRow {
			return result, errors.New("Summary workbook separator is outside the frame")
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("A%d", layout.SeparatorRow), fmt.Sprintf("%s%d", lastColumn, layout.SeparatorRow), heading); err != nil {
			return result, err
		}
	}
	for _, placed := range layout.Cells {
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
			case "null":
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
			case "real":
				if layout.RealOrderFromRow == 0 || placed.Row < layout.RealOrderFromRow || placed.Column != layout.RealOrderColumn {
					return result, fmt.Errorf("Summary workbook %s real values require the explicit grouped Order range", address)
				}
				if err := book.SetCellFloat(sheet, address, *cell.Real, -1, 64); err != nil {
					return result, err
				}
			default:
				return result, fmt.Errorf("Summary workbook %s has unsupported visible storage %q", address, cell.Storage)
			}
		}
		if layout.ItalicFromRow > 0 && placed.Row >= layout.ItalicFromRow && placed.Column == 2 {
			if err := book.SetCellStyle(sheet, address, address, italic); err != nil {
				return result, err
			}
		}
		for _, row := range layout.HeaderRows {
			if placed.Row == row {
				if err := book.SetCellStyle(sheet, address, address, heading); err != nil {
					return result, err
				}
			}
		}
	}
	if writeSheets != nil {
		if err := writeSheets(book); err != nil {
			return result, err
		}
		if err := ctx.Err(); err != nil {
			return result, err
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
	return bytes.Clone(buffer.Bytes()), nil
}
