package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

type EnvironmentWorkbookSheet struct {
	Unit string `json:"unit"`
	Name string `json:"name"`
}

type environmentWorkbook struct {
	Bytes  []byte
	Sheets []EnvironmentWorkbookSheet
}

func environmentWorkbookText(value string) error {
	if !utf8.ValidString(value) || len(utf16.Encode([]rune(value))) > 32767 {
		return errors.New("workbook text requires valid Unicode within Excel's 32767 UTF-16 units; no truncation")
	}
	for _, r := range value {
		if r != '\t' && r != '\n' && r != '\r' && (r < 0x20 || r == 0xfffe || r == 0xffff) {
			return errors.New("workbook text contains an unsupported XML character; no replacement")
		}
	}
	return nil
}

func environmentWorkbookSheetName(code string, index int) (string, error) {
	if err := environmentWorkbookText(code); err != nil {
		return "", err
	}
	name := code
	if name == "" {
		name = fmt.Sprintf("NoName%d", index)
	}
	units := utf16.Encode([]rune(name))
	if len(units) > 31 {
		if units[30] >= 0xd800 && units[30] <= 0xdbff {
			return "", errors.New("source 31-unit worksheet truncation would split Unicode; choose a valid unit name explicitly")
		}
		name = string(utf16.Decode(units[:31]))
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:/\[]*?`, r) {
			return '-'
		}
		return r
	}, name)
	if strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") || strings.EqualFold(name, "_VPRO_Source") {
		return "", errors.New("worksheet name conflicts with Excel/provenance rules; no implicit identity change")
	}
	return name, nil
}

func prepareLongEnvironmentWorkbook(ctx context.Context, report EnvironmentReport) (result environmentWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Long Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if len(report.Units) == 0 || !reflect.DeepEqual(report.Fields, longEnvironmentFields()) {
		return result, errors.New("Long Environment workbook requires the complete original 72-field report and at least one unit")
	}
	for _, value := range []string{report.Title, report.Project, report.SU} {
		if err := environmentWorkbookText(value); err != nil {
			return result, err
		}
	}
	book := excelize.NewFile()
	defer func() {
		resultErr = errors.Join(resultErr, book.Close())
		if resultErr != nil {
			result = environmentWorkbook{}
		}
	}()
	if err := book.SetDocProps(&excelize.DocProperties{Title: report.Title, Creator: "VPRO",
		Created: "2000-01-01T00:00:00Z", Modified: "2000-01-01T00:00:00Z"}); err != nil {
		return result, err
	}
	heading, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E5E7EB"}}})
	if err != nil {
		return result, err
	}
	setText := func(sheet, address, value string) error {
		if err := environmentWorkbookText(value); err != nil {
			return err
		}
		return book.SetCellStr(sheet, address, value)
	}
	codes := map[string]bool{}
	for index, unit := range report.Units {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if codes[unit.Code] {
			return result, errors.New("Long Environment workbook requires distinct original unit identities")
		}
		codes[unit.Code] = true
		if len(unit.Plots) > 16383 {
			return result, errors.New("Long Environment unit exceeds Excel's 16384-column limit; no omitted plots")
		}
		name, err := environmentWorkbookSheetName(unit.Code, index)
		if err != nil {
			return result, err
		}
		for _, previous := range result.Sheets {
			if strings.EqualFold(previous.Name, name) {
				return result, errors.New("source worksheet sanitization/truncation creates a name collision; no suffix or overwrite")
			}
		}
		if index == 0 {
			err = book.SetSheetName("Sheet1", name)
		} else {
			_, err = book.NewSheet(name)
		}
		if err != nil {
			return result, err
		}
		result.Sheets = append(result.Sheets, EnvironmentWorkbookSheet{unit.Code, name})
		longName := ""
		if unit.LongName != nil {
			longName = *unit.LongName
		} else if unit.NameStatus == "conflicting" || unit.NameStatus == "unsupported_storage" {
			return result, errors.New("Long Environment unit has unresolved long-name candidates; no inferred First")
		}
		for row, value := range []string{report.Title, "Environment Table", "Site Unit - " + unit.Code, longName} {
			if err := setText(name, fmt.Sprintf("A%d", row+1), value); err != nil {
				return result, err
			}
		}
		for field, definition := range report.Fields {
			address := fmt.Sprintf("A%d", field+5)
			if err := setText(name, address, definition.Label); err != nil {
				return result, err
			}
			if definition.Heading {
				last, err := excelize.CoordinatesToCellName(len(unit.Plots)+1, field+5)
				if err != nil {
					return result, err
				}
				if err := book.SetCellStyle(name, address, last, heading); err != nil {
					return result, err
				}
			}
		}
		for column, plot := range unit.Plots {
			if len(plot.Values) != len(report.Fields) {
				return result, errors.New("Long Environment plot lacks complete typed cells")
			}
			for field, cell := range plot.Values {
				if err := ctx.Err(); err != nil {
					return result, err
				}
				value, err := metadataCellValue(cell)
				if err != nil {
					return result, err
				}
				address, err := excelize.CoordinatesToCellName(column+2, field+5)
				if err != nil {
					return result, err
				}
				switch value := value.(type) {
				case nil:
				case string:
					err = setText(name, address, value)
				case int64:
					if value < -999999999999999 || value > 999999999999999 {
						return result, errors.New("historical INTEGER exceeds Excel's 15-digit numeric precision; no rounding or string repair")
					}
					err = book.SetCellInt(name, address, value)
				case float64:
					err = book.SetCellFloat(name, address, value, -1, 64)
				default:
					return result, errors.New("Long Environment workbook cannot represent original blob storage")
				}
				if err != nil {
					return result, err
				}
			}
		}
		if err := book.SetColWidth(name, "A", "A", 32); err != nil {
			return result, err
		}
		if len(unit.Plots) != 0 {
			last, err := excelize.ColumnNumberToName(len(unit.Plots) + 1)
			if err != nil {
				return result, err
			}
			if err := book.SetColWidth(name, "B", last, 14); err != nil {
				return result, err
			}
		}
		if err := book.SetPanes(name, &excelize.Panes{Freeze: true, XSplit: 1, YSplit: 5, TopLeftCell: "B6", ActivePane: "bottomRight"}); err != nil {
			return result, err
		}
		orientation, fitWidth, fitHeight, fit := "portrait", 0, 1, true
		if err := book.SetPageLayout(name, &excelize.PageLayoutOptions{
			Orientation: &orientation, FitToWidth: &fitWidth, FitToHeight: &fitHeight,
		}); err != nil {
			return result, err
		}
		if err := book.SetSheetProps(name, &excelize.SheetPropsOptions{FitToPage: &fit}); err != nil {
			return result, err
		}
		side, half := 0.55, 0.5
		if err := book.SetPageMargins(name, &excelize.PageLayoutMarginsOptions{
			Left: &side, Right: &side, Top: &half, Bottom: &half, Header: &half, Footer: &half,
		}); err != nil {
			return result, err
		}
		if err := book.SetHeaderFooter(name, &excelize.HeaderFooterOptions{
			OddHeader: "&RPage &P of &N", OddFooter: "&R&D",
		}); err != nil {
			return result, err
		}
	}
	ordered := append([]EnvironmentWorkbookSheet(nil), result.Sheets...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return strings.ToUpper(ordered[i].Name) < strings.ToUpper(ordered[j].Name)
	})
	for i := len(ordered) - 2; i >= 0; i-- {
		if err := book.MoveSheet(ordered[i].Name, ordered[i+1].Name); err != nil {
			return result, err
		}
	}
	book.SetActiveSheet(0)
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	source, err := json.Marshal(report)
	if err != nil {
		return result, err
	}
	// Chunk the lossless typed report as hex: visible workbook numeric/blank
	// conventions must never become a replacement for original storage metadata.
	hexSource := fmt.Sprintf("%x", source)
	for offset, row := 0, 1; offset < len(hexSource); offset, row = offset+30000, row+1 {
		end := min(offset+30000, len(hexSource))
		if err := book.SetCellStr("_VPRO_Source", "A"+strconv.Itoa(row), hexSource[offset:end]); err != nil {
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
	result.Bytes = bytes.Clone(buffer.Bytes())
	return result, nil
}
