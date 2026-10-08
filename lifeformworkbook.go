package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type lifeformWorkbookLayout struct {
	Details [6]bool
}

type lifeformWorkbook struct {
	Bytes  []byte
	Sheets []vegetationWorkbookSheet
}

// Validate provenance before JSON can replace malformed Unicode or typed cells.
func lifeformWorkbookSourceValue(value reflect.Value) error {
	if value.Type() == reflect.TypeOf(ProjectMetadataCell{}) {
		cell := value.Interface().(ProjectMetadataCell)
		if _, err := vegetationReportIdentity(cell); err != nil {
			return err
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if err := lifeformWorkbookSourceValue(value.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if err := lifeformWorkbookSourceValue(value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if !value.IsNil() {
			return lifeformWorkbookSourceValue(value.Elem())
		}
	case reflect.String:
		return environmentWorkbookText(value.String())
	case reflect.Float64:
		if math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0) {
			return errors.New("Lifeform workbook requires finite source numbers")
		}
	}
	return nil
}

func validateLifeformWorkbook(ctx context.Context, lifeform LifeformSummaryPreview, attributes SpeciesAttributeSummaryPreview) error {
	lf, at := lifeform.Report, attributes.Report
	if lifeform.ContextID == "" || lifeform.ProjectPath == "" || lifeform.SUPath == "" ||
		lifeform.ContextID != attributes.ContextID || lifeform.ProjectPath != attributes.ProjectPath ||
		lifeform.SUPath != attributes.SUPath || lf.Project == "" || lf.SU == "" ||
		lf.Project != at.Project || lf.SU != at.SU || lf.SU == "None" || lf.SU == "USysSuTableDynamic" ||
		lf.QuerySource != "normal-su-global-entrydat-plot-rejoin" ||
		lf.Ordering != "physical-catalogue-rowid-and-first-membership" ||
		at.QuerySource != "normal-su-raw-veg-attribute-join" ||
		!reflect.DeepEqual(at.Definitions, speciesAttributeDefinitions()) ||
		!reflect.DeepEqual(lf.Memberships, at.Memberships) ||
		len(lf.Units) == 0 || len(lf.Units) != len(at.Units) || len(lf.Catalogue) == 0 {
		return errors.New("Lifeform workbook requires complete matching owned normal-SU previews and source schemas")
	}
	for _, source := range []any{lifeform, attributes} {
		if err := lifeformWorkbookSourceValue(reflect.ValueOf(source)); err != nil {
			return err
		}
	}
	members := map[string]VegetationReportMembership{}
	unitOrder := []string{}
	unitRows := map[string][]string{}
	unitPlots := map[string]int{}
	for _, member := range lf.Memberships {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, found := members[member.RowID]; found || member.RowID == "" {
			return errors.New("Lifeform workbook requires distinct nonempty original membership row IDs")
		}
		members[member.RowID] = member
		key := vegetationTextKey(member.SiteUnit)
		if _, found := unitRows[key]; !found {
			unitOrder = append(unitOrder, key)
		}
		unitRows[key] = append(unitRows[key], member.RowID)
		if member.PlotNumber.Text != nil {
			unitPlots[key]++
		}
	}
	if len(unitOrder) != len(lf.Units) {
		return errors.New("Lifeform workbook cannot omit or invent source units")
	}
	catalogueIDs, forms := map[string]bool{}, map[int]bool{}
	for _, cat := range lf.Catalogue {
		if cat.RowID == "" || catalogueIDs[cat.RowID] || forms[cat.Lifeform] || cat.Lifeform == 13 ||
			int64(cat.Lifeform) < -999999999999999 || int64(cat.Lifeform) > 999999999999999 {
			return errors.New("Lifeform workbook requires complete unique original catalogue keys excluding 13")
		}
		catalogueIDs[cat.RowID], forms[cat.Lifeform] = true, true
	}
	validCount := func(n int) bool { return n >= 0 && int64(n) <= 999999999999999 }
	for i, unit := range lf.Units {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := vegetationTextKey(unit.Code)
		attributeUnit := at.Units[i]
		if key != unitOrder[i] || !reflect.DeepEqual(unit.Code, attributeUnit.Code) ||
			unit.NPlots != unitPlots[key] || unit.NPlots != attributeUnit.NPlots ||
			!reflect.DeepEqual(unit.SURowIDs, unitRows[key]) ||
			!reflect.DeepEqual(unit.SURowIDs, attributeUnit.SURowIDs) ||
			!validCount(unit.UniqueSpecies) || !validCount(unit.Occurrences) ||
			unit.UniqueSpecies > unit.Occurrences || len(unit.Rows) != len(lf.Catalogue) ||
			len(unit.Rows) > 1048576-18 || len(attributeUnit.Rows) != 6 {
			return errors.New("Lifeform workbook unit ownership, physical counts or complete row shape mismatch")
		}
		for j, row := range unit.Rows {
			cat := lf.Catalogue[j]
			if row.CatalogueRowID != cat.RowID || row.Lifeform != cat.Lifeform ||
				!validCount(row.PlotGroups) || !validCount(row.CoverCount) ||
				row.PlotGroups > unit.NPlots || row.CoverCount < row.PlotGroups ||
				row.CoverCount > unit.Occurrences ||
				(row.PlotGroups == 0) != (row.CoverCount == 0) ||
				(unit.NPlots == 0) != (row.Presence == nil) ||
				(row.PlotGroups == 0 || unit.NPlots == 0) != (row.MeanCover == nil) {
				return errors.New("Lifeform workbook catalogue association or typed counts/NULL ratios mismatch")
			}
			if row.Presence != nil && *row.Presence != float64(row.PlotGroups)/float64(unit.NPlots) {
				return errors.New("Lifeform workbook presence disagrees with its physical source denominator")
			}
		}
		for j, row := range attributeUnit.Rows {
			def := at.Definitions[j]
			if row.Field != def.Field || len(row.Categories) != len(def.Categories) ||
				!validCount(row.PlotOccurrences) || row.PlotOccurrences > unit.NPlots ||
				row.Count == nil && row.PlotOccurrences != 0 ||
				row.Count != nil && (!validCount(*row.Count) || *row.Count == 0 || *row.Count < row.PlotOccurrences) {
				return errors.New("Lifeform workbook requires all six original attribute counts and category shapes")
			}
			total := 0
			for _, count := range row.Categories {
				if count == nil {
					continue
				}
				if row.Count == nil || !validCount(*count) || *count == 0 || *count > *row.Count-total {
					return errors.New("Lifeform workbook has invalid nullable attribute category counts")
				}
				total += *count
			}
		}
	}
	for _, entry := range lf.Entries {
		member, found := members[entry.InsertSURowID]
		if !found || entry.SpeciesRowID == "" || member.PlotNumber.Text == nil ||
			entry.PlotNumber != *member.PlotNumber.Text || !reflect.DeepEqual(entry.ProjectID, member.SiteUnit) ||
			entry.Lifeform < 0 || entry.Lifeform > 12 || entry.Cover != float64(float32(entry.Cover)) {
			return errors.New("Lifeform workbook entry lacks original insertion ownership or SINGLE assignment evidence")
		}
	}
	for _, match := range at.Matches {
		member, found := members[match.SURowID]
		if !found || member.SiteUnit.Text == nil || member.PlotNumber.Text == nil ||
			match.PlotNumber != *member.PlotNumber.Text || match.VegRowID == "" ||
			match.AttributeRowID == "" || len(match.Values) != 6 {
			return errors.New("Lifeform workbook attribute match lacks original membership or complete typed values")
		}
	}
	return ctx.Err()
}

func prepareLifeformSummaryWorkbook(ctx context.Context, lifeform LifeformSummaryPreview, attributes SpeciesAttributeSummaryPreview, layout lifeformWorkbookLayout) (result lifeformWorkbook, resultErr error) {
	if ctx == nil {
		return result, errors.New("Lifeform workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := validateLifeformWorkbook(ctx, lifeform, attributes); err != nil {
		return result, err
	}
	book := excelize.NewFile()
	defer func() {
		resultErr = errors.Join(resultErr, book.Close(), ctx.Err())
		if resultErr != nil {
			result = lifeformWorkbook{}
		}
	}()
	if err := book.SetDocProps(&excelize.DocProperties{Title: "Lifeform Summary", Creator: "VPRO",
		Created: "2000-01-01T00:00:00Z", Modified: "2000-01-01T00:00:00Z"}); err != nil {
		return result, err
	}
	bold, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return result, err
	}
	format := "0.0%"
	percent, err := book.NewStyle(&excelize.Style{CustomNumFmt: &format})
	if err != nil {
		return result, err
	}
	text := func(sheet, cell, value string) error {
		if err := environmentWorkbookText(value); err != nil {
			return err
		}
		return book.SetCellStr(sheet, cell, value)
	}
	labels := [6]string{"SRank", "Wetland_Ind", "WeedStatus", "RedBlueList", "ASMR", "Climate"}
	detailLabels := [6]string{"nSRankDetail", "nWetland_Ind", "nWeedStatusDetail", "nRedBlueListDetail", "nASMRDetail", "nClimateDetail"}
	for i, unit := range lifeform.Report.Units {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		code := ""
		if unit.Code.Text != nil {
			code = *unit.Code.Text
		}
		// NULL takes the source empty-name mapping at its own index. The typed
		// sheet mapping/provenance retains NULL versus actual empty text.
		name, err := environmentWorkbookSheetName(code, i)
		if err != nil {
			return result, err
		}
		for _, previous := range result.Sheets {
			if strings.EqualFold(previous.Name, name) {
				return result, errors.New("Lifeform source worksheet names collide; no suffix or overwrite")
			}
		}
		if i == 0 {
			err = book.SetSheetName("Sheet1", name)
		} else {
			_, err = book.NewSheet(name)
		}
		if err != nil {
			return result, err
		}
		result.Sheets = append(result.Sheets, vegetationWorkbookSheet{cloneSiteUnitCell(unit.Code), name})
		for row, caption := range []string{"Project: " + lifeform.Report.Project,
			"Site Unit Table: " + lifeform.Report.SU, "Unit: " + name,
			"nPlots: " + strconv.Itoa(unit.NPlots),
			"Number of unique species: " + strconv.Itoa(unit.UniqueSpecies),
			"Total species occurences : " + strconv.Itoa(unit.Occurrences)} {
			if err := text(name, fmt.Sprintf("A%d", row+1), caption); err != nil {
				return result, err
			}
		}
		if err := book.SetCellStyle(name, "A1", "A6", bold); err != nil {
			return result, err
		}
		for _, header := range []struct{ col, caption string }{
			{"B", "^Count"}, {"C", "^Number of plot occurences"},
		} {
			if err := text(name, header.col+"8", header.caption); err != nil {
				return result, err
			}
			if err := book.MergeCell(name, header.col+"8", header.col+"9"); err != nil {
				return result, err
			}
		}
		if err := book.SetCellStyle(name, "B8", "C9", bold); err != nil {
			return result, err
		}
		detailRow := 8
		for j, row := range attributes.Report.Units[i].Rows {
			if err := text(name, fmt.Sprintf("A%d", j+10), labels[j]); err != nil {
				return result, err
			}
			if row.Count != nil {
				if err := book.SetCellInt(name, fmt.Sprintf("B%d", j+10), int64(*row.Count)); err != nil {
					return result, err
				}
			}
			if err := book.SetCellInt(name, fmt.Sprintf("C%d", j+10), int64(row.PlotOccurrences)); err != nil {
				return result, err
			}
			if !layout.Details[j] {
				continue
			}
			if err := text(name, fmt.Sprintf("E%d", detailRow), "SiteUnit"); err != nil {
				return result, err
			}
			if err := text(name, fmt.Sprintf("E%d", detailRow+1), detailLabels[j]); err != nil {
				return result, err
			}
			for k, category := range attributes.Report.Definitions[j].Categories {
				header, err := excelize.CoordinatesToCellName(k+6, detailRow)
				if err != nil {
					return result, err
				}
				if err := text(name, header, category); err != nil {
					return result, err
				}
				if row.Categories[k] != nil {
					cell, err := excelize.CoordinatesToCellName(k+6, detailRow+1)
					if err != nil {
						return result, err
					}
					if err := book.SetCellInt(name, cell, int64(*row.Categories[k])); err != nil {
						return result, err
					}
				}
			}
			last, _ := excelize.CoordinatesToCellName(len(row.Categories)+5, detailRow)
			if err := book.SetCellStyle(name, fmt.Sprintf("E%d", detailRow), last, bold); err != nil {
				return result, err
			}
			detailRow += 4
		}
		for j, caption := range []string{"Lifeform", "Presence", "Mean Cover"} {
			cell, _ := excelize.CoordinatesToCellName(j+1, 18)
			if err := text(name, cell, caption); err != nil {
				return result, err
			}
		}
		if err := book.SetCellStyle(name, "A18", "C18", bold); err != nil {
			return result, err
		}
		last := max(31, 18+len(unit.Rows))
		if err := book.SetCellStyle(name, "B19", fmt.Sprintf("C%d", last), percent); err != nil {
			return result, err
		}
		for j, row := range unit.Rows {
			if err := book.SetCellInt(name, fmt.Sprintf("A%d", j+19), int64(row.Lifeform)); err != nil {
				return result, err
			}
			for k, value := range []*float64{row.Presence, row.MeanCover} {
				if value == nil {
					continue
				}
				cell, _ := excelize.CoordinatesToCellName(k+2, j+19)
				if err := book.SetCellFloat(name, cell, *value, -1, 64); err != nil {
					return result, err
				}
			}
		}
	}
	if _, err := book.NewSheet("_VPRO_Source"); err != nil {
		return result, err
	}
	source, err := json.Marshal(struct {
		Lifeform   LifeformSummaryPreview
		Attributes SpeciesAttributeSummaryPreview
		Layout     lifeformWorkbookLayout
	}{lifeform, attributes, layout})
	if err != nil {
		return result, err
	}
	encoded := hex.EncodeToString(source)
	for offset, row := 0, 1; offset < len(encoded); offset, row = offset+30000, row+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if row > 1048576 {
			return result, errors.New("Lifeform lossless source metadata exceeds Excel's row limit")
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
	result.Bytes = bytes.Clone(buffer.Bytes())
	return result, nil
}
