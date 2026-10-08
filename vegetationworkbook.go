package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type vegetationWorkbookLayout struct {
	QuickReport        bool
	SpaceBetweenGroups bool
	ReportSummary      bool
	Summary            *vegetationWorkbookSummary
}

type vegetationWorkbookSheet struct {
	Unit ProjectMetadataCell
	Name string
}

type vegetationWorkbookSkippedUnit struct {
	Unit   ProjectMetadataCell
	Reason string
}

type vegetationWorkbook struct {
	Bytes        []byte
	Sheets       []vegetationWorkbookSheet
	SkippedUnits []vegetationWorkbookSkippedUnit
}

func prepareLongVegetationWorkbook(ctx context.Context, preview LongVegetationPreview, layout vegetationWorkbookLayout) (result vegetationWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Long Vegetation workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if layout.ReportSummary != (layout.Summary != nil) {
		return result, errors.New("Long Vegetation ReportSummary requires captured summary evidence; no omitted requested summary")
	}
	settings, report := preview.Settings, preview.Report
	if preview.ContextID == "" || preview.ProjectPath == "" || preview.SUPath == "" ||
		report.Project == "" || report.SU == "" || settings.Title != report.Title || len(report.Units) == 0 ||
		settings.ShowEnglishName && settings.ShowSpeciesCode ||
		settings.Average != "all-plots" && settings.Average != "observations" ||
		settings.Order != "species" && settings.Order != "presence" ||
		math.IsNaN(settings.PresenceGreaterThan) || math.IsInf(settings.PresenceGreaterThan, 0) ||
		math.IsNaN(settings.MeanCoverGreaterThan) || math.IsInf(settings.MeanCoverGreaterThan, 0) {
		return result, errors.New("Long Vegetation workbook requires complete owned report/settings identity")
	}
	groupLabel := "Layer"
	switch settings.Grouping {
	case "layer", "none":
	case "lifeform":
		groupLabel = "Lifeform"
	case "strata":
		groupLabel = "Strata"
	default:
		return result, errors.New("Long Vegetation workbook grouping is unavailable")
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
			result = vegetationWorkbook{}
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
	percentFormat, coverFormat := "0.0%", "0.0"
	percent, err := book.NewStyle(&excelize.Style{CustomNumFmt: &percentFormat})
	if err != nil {
		return result, err
	}
	cover, err := book.NewStyle(&excelize.Style{CustomNumFmt: &coverFormat})
	if err != nil {
		return result, err
	}
	setText := func(sheet, address, value string) error {
		if err := environmentWorkbookText(value); err != nil {
			return err
		}
		return book.SetCellStr(sheet, address, value)
	}
	setCell := func(sheet, address string, cell ProjectMetadataCell) error {
		value, err := metadataCellValue(cell)
		if err != nil {
			return err
		}
		if value == nil {
			return nil
		}
		text, ok := value.(string)
		if !ok {
			return errors.New("Long Vegetation workbook identities require original text/NULL storage")
		}
		return setText(sheet, address, text)
	}
	setNumber := func(sheet, address string, value *float64, style int) error {
		if value == nil {
			return nil
		}
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			return errors.New("Long Vegetation workbook requires finite typed numeric results")
		}
		if err := book.SetCellFloat(sheet, address, *value, -1, 64); err != nil {
			return err
		}
		return book.SetCellStyle(sheet, address, address, style)
	}
	identities := map[string]bool{}
	for index, unit := range report.Units {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		code, err := vegetationReportIdentity(unit.Code)
		if err != nil {
			return result, err
		}
		identity := vegetationTextKey(unit.Code)
		if identities[identity] || unit.NumPlots < 0 {
			return result, errors.New("Long Vegetation workbook requires distinct unit identities and valid plot counts")
		}
		identities[identity] = true
		if unit.NumPlots == 0 {
			result.SkippedUnits = append(result.SkippedUnits, vegetationWorkbookSkippedUnit{
				cloneSiteUnitCell(unit.Code), "source NumPlots < 1",
			})
			continue
		}
		if unit.NumPlots > 250 {
			return result, errors.New("Long Vegetation source limits one unit to 250 plots; split the selected data explicitly")
		}
		if len(unit.Rows) == 0 {
			return result, errors.New("Long Vegetation workbook lacks row/pivot evidence for a selected unit; empty-row output remains unavailable")
		}
		shortName := "Unassigned"
		if code != nil {
			shortName = *code
			if shortName == "" {
				shortName = fmt.Sprintf("NoName%d", index)
			}
		}
		name, err := environmentWorkbookSheetName(shortName, index)
		if err != nil {
			return result, err
		}
		if layout.ReportSummary && strings.EqualFold(name, "ReportSummary") {
			return result, errors.New("Long Vegetation worksheet names collide; no suffix or overwrite")
		}
		for _, previous := range result.Sheets {
			if strings.EqualFold(previous.Name, name) {
				return result, errors.New("Long Vegetation worksheet names collide; no suffix or overwrite")
			}
		}
		if len(result.Sheets) == 0 {
			err = book.SetSheetName("Sheet1", name)
		} else {
			_, err = book.NewSheet(name)
		}
		if layout.Summary != nil {
			if err := writeVegetationWorkbookSummary(ctx, book, preview, *layout.Summary); err != nil {
				return result, err
			}
		}
		if err != nil {
			return result, err
		}
		result.Sheets = append(result.Sheets, vegetationWorkbookSheet{cloneSiteUnitCell(unit.Code), name})
		unitTitle := shortName
		if unit.NameStatus == "conflicting" || unit.NameStatus == "unsupported_storage" {
			return result, errors.New("Long Vegetation workbook refuses unresolved original unit names")
		}
		if unit.LongName != nil && *unit.LongName != "" {
			unitTitle += " - [" + *unit.LongName + "]"
		}
		for row, value := range []string{report.Title, unitTitle, " "} {
			if err := setText(name, "A"+strconv.Itoa(row+1), value); err != nil {
				return result, err
			}
		}
		plots := map[string]bool{}
		for _, row := range unit.Rows {
			seen := map[string]bool{}
			for _, plot := range row.Plots {
				if seen[plot.PlotNumber] {
					return result, errors.New("Long Vegetation row repeats a pivot plot")
				}
				seen[plot.PlotNumber], plots[plot.PlotNumber] = true, true
			}
		}
		plotColumns := make([]string, 0, len(plots))
		for plot := range plots {
			plotColumns = append(plotColumns, plot)
		}
		sort.Strings(plotColumns)
		labels := []string{groupLabel, "Spp"}
		if settings.ShowEnglishName {
			labels = append(labels, "English Name")
		} else if settings.ShowSpeciesCode {
			labels = append(labels, "Code")
		}
		presenceColumn, meanColumn := len(labels)+1, len(labels)+2
		labels = append(labels, "P", "MC")
		firstPlotColumn := len(labels) + 1
		labels = append(labels, plotColumns...)
		if len(labels) > 16384 {
			return result, errors.New("Long Vegetation workbook exceeds Excel's column limit")
		}
		for column, label := range labels {
			address, err := excelize.CoordinatesToCellName(column+1, 4)
			if err != nil {
				return result, err
			}
			if err := setText(name, address, label); err != nil {
				return result, err
			}
		}
		lastColumn, err := excelize.ColumnNumberToName(len(labels))
		if err != nil {
			return result, err
		}
		if err := book.SetCellStyle(name, "A4", lastColumn+"4", heading); err != nil {
			return result, err
		}
		outputRow, previousGroup := 5, ""
		// Source starts at worksheet row 7 and stops at the next blank group.
		gapScans, scanGaps := 0, true
		for index, row := range unit.Rows {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			group := ""
			if row.Layer.Text != nil {
				group = *row.Layer.Text
			}
			if layout.SpaceBetweenGroups && index >= 2 && scanGaps && gapScans < 1000 {
				if group != previousGroup {
					outputRow++
				}
				gapScans++
				if index+1 < len(unit.Rows) {
					next := unit.Rows[index+1].Layer
					if next.Storage == "null" || next.Text != nil && *next.Text == "" {
						scanGaps = false
					}
				}
			}
			previousGroup = group
			if outputRow > 1048576 {
				return result, errors.New("Long Vegetation workbook exceeds Excel's row limit")
			}
			if err := setCell(name, "A"+strconv.Itoa(outputRow), row.Layer); err != nil {
				return result, err
			}
			if err := setCell(name, "B"+strconv.Itoa(outputRow), row.Species); err != nil {
				return result, err
			}
			if settings.ShowEnglishName || settings.ShowSpeciesCode {
				if err := setCell(name, "C"+strconv.Itoa(outputRow), row.EnglishName); err != nil {
					return result, err
				}
			}
			for _, metric := range []struct {
				column int
				value  *float64
				style  int
			}{{presenceColumn, row.Presence, percent}, {meanColumn, row.MeanCover, cover}} {
				address, err := excelize.CoordinatesToCellName(metric.column, outputRow)
				if err != nil {
					return result, err
				}
				if err := setNumber(name, address, metric.value, metric.style); err != nil {
					return result, err
				}
			}
			for _, plot := range row.Plots {
				column := sort.SearchStrings(plotColumns, plot.PlotNumber) + firstPlotColumn
				address, err := excelize.CoordinatesToCellName(column, outputRow)
				if err != nil {
					return result, err
				}
				if err := setNumber(name, address, plot.Cover, cover); err != nil {
					return result, err
				}
			}
			outputRow++
		}
		if err := book.SetColWidth(name, "A", lastColumn, 14); err != nil {
			return result, err
		}
		if err := book.SetColWidth(name, "B", "B", 42); err != nil {
			return result, err
		}
		firstPlot, err := excelize.CoordinatesToCellName(firstPlotColumn, 5)
		if err != nil {
			return result, err
		}
		if err := book.SetPanes(name, &excelize.Panes{Freeze: true, XSplit: firstPlotColumn - 1,
			YSplit: 4, TopLeftCell: firstPlot, ActivePane: "bottomRight"}); err != nil {
			return result, err
		}
		if !layout.QuickReport {
			orientation, side, top := "landscape", .75, .5
			if err := book.SetPageLayout(name, &excelize.PageLayoutOptions{Orientation: &orientation}); err != nil {
				return result, err
			}
			if err := book.SetPageMargins(name, &excelize.PageLayoutMarginsOptions{
				Left: &side, Right: &side, Top: &top, Bottom: &side,
			}); err != nil {
				return result, err
			}
			if err := book.SetHeaderFooter(name, &excelize.HeaderFooterOptions{OddFooter: "&LPage &P of &N&R&D"}); err != nil {
				return result, err
			}
		}
	}
	if len(result.Sheets) == 0 {
		return result, errors.New("There isn't any data to form a Long Vegetation workbook")
	}
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	for _, sheet := range result.Sheets {
		if err := book.MoveSheet(sheet.Name, "_VPRO_Source"); err != nil {
			return result, err
		}
	}
	if layout.Summary != nil {
		if err := book.MoveSheet("ReportSummary", "_VPRO_Source"); err != nil {
			return result, err
		}
	}
	book.SetActiveSheet(0)
	source, err := json.Marshal(struct {
		Preview LongVegetationPreview
		Layout  vegetationWorkbookLayout
	}{preview, layout})
	if err != nil {
		return result, err
	}
	encoded := hex.EncodeToString(source)
	for offset, row := 0, 1; offset < len(encoded); offset, row = offset+30000, row+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if row > 1048576 {
			return result, errors.New("Long Vegetation lossless source metadata exceeds Excel's row limit")
		}
		if err := book.SetCellStr("_VPRO_Source", "A"+strconv.Itoa(row), encoded[offset:min(offset+30000, len(encoded))]); err != nil {
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
