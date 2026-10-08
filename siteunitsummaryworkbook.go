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
	"strings"

	"github.com/xuri/excelize/v2"
)

type siteUnitSummaryWorkbook struct {
	Bytes  []byte
	Sheets []vegetationWorkbookSheet
}

func validateSiteUnitSummaryWorkbook(ctx context.Context, preview SiteUnitSummaryPreview) ([]vegetationWorkbookSheet, error) {
	report := preview.Report
	if preview.ContextID == "" || preview.ProjectPath == "" || preview.SUPath == "" ||
		report.Project == "" || report.SU == "" || report.SU == "None" || report.SU == "USysSuTableDynamic" ||
		(report.Method != 1 && report.Method != 2) || report.QuerySource != string(siteUnitDetailSelectedSU) ||
		!reflect.DeepEqual(report.Fields, siteUnitSummaryFields()) || len(report.Units) == 0 || len(report.Memberships) == 0 {
		return nil, errors.New("Summary Environment workbook requires a complete owned normal-SU preview, explicit method and original 39-field schema")
	}
	if err := lifeformWorkbookSourceValue(reflect.ValueOf(preview)); err != nil {
		return nil, err
	}
	members := map[string]SiteUnitSummaryMembership{}
	counts := map[string]int{}
	for i, member := range report.Memberships {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if member.RowID == "" || i > 0 && report.Memberships[i-1].RowID >= member.RowID || member.JoinedRows < 0 {
			return nil, errors.New("Summary Environment requires ordered distinct original membership IDs and nonnegative physical counts")
		}
		plot, unit := member.PlotNumber.Text, member.SiteUnit.Text
		valid := false
		switch member.Status {
		case "null-unit":
			valid = unit == nil && member.JoinedRows == 0
		case "null-plot":
			valid = unit != nil && plot == nil && member.JoinedRows == 0
		case "missing-env", "missing-admin":
			valid = unit != nil && plot != nil && member.JoinedRows == 0
		case "joined":
			valid = unit != nil && plot != nil && member.JoinedRows > 0
		}
		if !valid {
			return nil, errors.New("Summary Environment membership status disagrees with typed identities or physical counts")
		}
		members[member.RowID] = member
	}
	sheets := make([]vegetationWorkbookSheet, 0, len(report.Units))
	envOwners, adminOwners, nameOwners := map[string]string{}, map[string]string{}, map[string]string{}
	joinedPlots := map[string]bool{}
	for i, unit := range report.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if i > 0 && report.Units[i-1].Code <= unit.Code || len(unit.Values) != 39 ||
			len(unit.Plots) == 0 || unit.NameCandidates == nil {
			return nil, errors.New("Summary Environment requires descending distinct literal units, physical plots and complete values/name evidence")
		}
		if unit.Values[8] != unit.Values[9] {
			return nil, errors.New("Summary Environment requires the source's repeated SiteDisturbance2 summaries without independent replacement")
		}
		name, err := environmentWorkbookSheetName(unit.Code, i)
		if err != nil {
			return nil, err
		}
		for _, sheet := range sheets {
			if strings.EqualFold(sheet.Name, name) {
				// Refusal replaces the source's silent skipping, never suffixes or omitted units.
				return nil, errors.New("Summary Environment source worksheet sanitization/truncation creates a name collision; safe adaptation refuses all output, no suffix or omitted sheet")
			}
		}
		sheets = append(sheets, vegetationWorkbookSheet{Unit: ProjectMetadataCell{Storage: "text", Text: &unit.Code}, Name: name})
		nameRows := make([]ProjectMetadataRow, 0, len(unit.NameCandidates))
		for j, candidate := range unit.NameCandidates {
			if candidate.RowID == "" || j > 0 && unit.NameCandidates[j-1].RowID >= candidate.RowID {
				return nil, errors.New("Summary Environment requires ordered distinct original name candidate IDs")
			}
			if _, found := nameOwners[candidate.RowID]; found {
				return nil, errors.New("Summary Environment name candidate belongs to more than one unit")
			}
			nameOwners[candidate.RowID] = unit.Code
			nameRows = append(nameRows, ProjectMetadataRow{RowID: candidate.RowID, Cells: []ProjectMetadataCell{candidate.Value}})
		}
		names, err := resolveReportUnitNames(ctx, nameRows, 0, "Summary Environment workbook")
		if err != nil {
			return nil, err
		}
		if names.Status != unit.NameStatus || !reflect.DeepEqual(names.LongName, unit.LongName) ||
			(names.Status != "unique" && names.Status != "missing") {
			return nil, errors.New("Summary Environment long name disagrees with original candidates or is unresolved; no inferred First")
		}
		var previous [4]string
		for j, plot := range unit.Plots {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			member, found := members[plot.SURowID]
			key := [4]string{plot.PlotNumber, plot.SURowID, plot.EnvRowID, plot.AdminRowID}
			ordered := false
			for k := range key {
				if key[k] != previous[k] {
					ordered = key[k] > previous[k]
					break
				}
			}
			if !found || member.Status != "joined" || *member.SiteUnit.Text != unit.Code ||
				*member.PlotNumber.Text != plot.PlotNumber || plot.EnvRowID == "" || plot.AdminRowID == "" ||
				j > 0 && !ordered {
				return nil, errors.New("Summary Environment plot lacks original membership ownership, distinct physical keys or literal source order")
			}
			for k, owners := range []map[string]string{envOwners, adminOwners} {
				id := key[k+2]
				if owner, found := owners[id]; found && owner != plot.PlotNumber {
					return nil, errors.New("Summary Environment physical source row has inconsistent plot identity")
				}
				owners[id] = plot.PlotNumber
			}
			previous = key
			counts[plot.SURowID]++
			joinedPlots[plot.PlotNumber] = true
		}
	}
	// Membership counts retain duplicate SU/Env/Admin weights, not distinct plots.
	envCounts, adminCounts := map[string]int{}, map[string]int{}
	for _, plot := range envOwners {
		envCounts[plot]++
	}
	for _, plot := range adminOwners {
		adminCounts[plot]++
	}
	for _, member := range report.Memberships {
		if counts[member.RowID] != member.JoinedRows ||
			(member.PlotNumber.Text != nil && member.SiteUnit.Text != nil &&
				member.Status != "joined" && joinedPlots[*member.PlotNumber.Text]) {
			return nil, errors.New("Summary Environment physical plot evidence disagrees with membership counts/exclusions; no omitted rows")
		}
		if member.Status == "joined" {
			plot := *member.PlotNumber.Text
			if member.JoinedRows/envCounts[plot] != adminCounts[plot] || member.JoinedRows%envCounts[plot] != 0 {
				return nil, errors.New("Summary Environment lacks the complete physical Env/Admin join for an original membership")
			}
		}
	}
	return sheets, ctx.Err()
}

func prepareSiteUnitSummaryWorkbook(ctx context.Context, preview SiteUnitSummaryPreview) (result siteUnitSummaryWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Summary Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	sheets, err := validateSiteUnitSummaryWorkbook(ctx, preview)
	if err != nil {
		return result, err
	}
	book := excelize.NewFile()
	defer func() {
		resultErr = errors.Join(resultErr, book.Close(), ctx.Err())
		if resultErr != nil {
			result = siteUnitSummaryWorkbook{}
		}
	}()
	if err := book.SetDocProps(&excelize.DocProperties{Title: "Summary Environment", Creator: "VPRO",
		Created: "2000-01-01T00:00:00Z", Modified: "2000-01-01T00:00:00Z"}); err != nil {
		return result, err
	}
	bold, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return result, err
	}
	text := func(sheet, cell, value string) error {
		// Excelize's plain shared-string writer does not preserve literal OOXML escapes.
		// A single unformatted rich-text run uses its checked escape-aware writer.
		if strings.Contains(value, "_x") {
			if err := book.SetCellRichText(sheet, cell, []excelize.RichTextRun{{Text: value}}); err != nil {
				return err
			}
			actual, err := book.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
			if err != nil {
				return err
			}
			if actual != value {
				return errors.New("Summary Environment literal text cannot roundtrip through Excel's escape representation; no replacement or partial workbook")
			}
			return nil
		}
		return book.SetCellStr(sheet, cell, value)
	}
	method := "Quantitative values summarized as: Min---Mean---Max"
	if preview.Report.Method == 2 {
		method = "Quantitative values summarized as: Interquartile 25% - 50% - 75%"
	}
	rows := [...]int{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27,
		30, 31, 32, 33, 34, 35, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49}
	for i, unit := range preview.Report.Units {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		name := sheets[i].Name
		if i == 0 {
			err = book.SetSheetName("Sheet1", name)
		} else {
			_, err = book.NewSheet(name)
		}
		if err != nil {
			return result, err
		}
		longName := ""
		if unit.LongName != nil {
			longName = *unit.LongName
		}
		for j, value := range []string{unit.Code, longName, "Plots in unit: " + strconv.Itoa(len(unit.Plots)), method} {
			if err := text(name, fmt.Sprintf("A%d", j+1), value); err != nil {
				return result, err
			}
		}
		if err := book.SetCellStyle(name, "A1", "A3", bold); err != nil {
			return result, err
		}
		for j, heading := range []string{"SITE", "VEGETATION", "SOILS"} {
			cell := fmt.Sprintf("A%d", [...]int{6, 29, 37}[j])
			if err := text(name, cell, heading); err != nil {
				return result, err
			}
			if err := book.SetCellStyle(name, cell, cell, bold); err != nil {
				return result, err
			}
		}
		for j, field := range preview.Report.Fields {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if err := text(name, fmt.Sprintf("A%d", rows[j]), field.Label); err != nil {
				return result, err
			}
			// Helper-produced summaries remain literal strings, including numeric-looking text.
			if err := text(name, fmt.Sprintf("B%d", rows[j]), unit.Values[j]); err != nil {
				return result, err
			}
		}
		if err := book.SetColWidth(name, "A", "A", 34); err != nil {
			return result, err
		}
		if err := book.SetColWidth(name, "B", "B", 60); err != nil {
			return result, err
		}
	}
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	source, err := json.Marshal(preview)
	if err != nil {
		return result, err
	}
	encoded := hex.EncodeToString(source)
	for offset, row := 0, 1; offset < len(encoded); offset, row = offset+30000, row+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if row > 1048576 {
			return result, errors.New("Summary Environment lossless source metadata exceeds Excel's row limit")
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
	result.Bytes, result.Sheets = bytes.Clone(buffer.Bytes()), sheets
	return result, nil
}
