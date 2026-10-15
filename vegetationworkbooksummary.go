package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
)

type vegetationWorkbookSummary struct {
	CreatedDate        string
	EnvironmentRows    int
	SelectedPlotRows   int
	SpeciesVersion     ProjectMetadataCell
	VersionStatus      string
	VersionDefinitions ProjectMetadataTable
	Memberships        []vegetationWorkbookMembership
}

type vegetationWorkbookMembership struct {
	RowID        string
	PlotNumber   ProjectMetadataCell
	SiteUnit     ProjectMetadataCell
	Quality      *LongVegetationQualityOccurrence
	NoneUnit     *LongVegetationNoneUnitOccurrence
	SelectedUnit *LongVegetationSelectedUnitOccurrence
}

type vegetationWorkbookSummaryHeader struct {
	CreatedDate        string
	EnvironmentRows    int
	SpeciesVersion     ProjectMetadataCell
	VersionStatus      string
	VersionDefinitions ProjectMetadataTable
}

func prepareVegetationWorkbookSummaryHeader(ctx context.Context, createdDate string, env, descriptions ProjectMetadataTable) (vegetationWorkbookSummaryHeader, error) {
	fail := func(err error) (vegetationWorkbookSummaryHeader, error) {
		return vegetationWorkbookSummaryHeader{}, err
	}
	if ctx == nil {
		return fail(errors.New("Long Vegetation summary requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	date, err := time.Parse("2006-01-02", createdDate)
	if err != nil || date.Format("2006-01-02") != createdDate || date.Year() < 100 {
		return fail(errors.New("Long Vegetation summary requires an explicit reviewed ISO calendar date"))
	}
	if _, err := siteUnitTransferColumns(env); err != nil {
		return fail(err)
	}
	unknownVersion := "Unknown"
	result := vegetationWorkbookSummaryHeader{
		CreatedDate: createdDate, EnvironmentRows: len(env.Rows), SpeciesVersion: ProjectMetadataCell{Storage: "text", Text: &unknownVersion},
		VersionStatus: "missing-definition", VersionDefinitions: descriptions,
	}
	if len(descriptions.Columns) == 0 {
		result.VersionStatus = "metadata-unavailable"
	}
	if len(descriptions.Rows) > 1 {
		return fail(errors.New("Long Vegetation AllSpecs version has duplicate definitions; no inferred Description"))
	}
	if len(descriptions.Columns) > 0 {
		if _, err := siteUnitTransferColumns(descriptions, "table_name", "description"); err != nil {
			return fail(err)
		}
	} else if len(descriptions.Rows) > 0 {
		return fail(errors.New("Long Vegetation AllSpecs Description rows lack metadata schema"))
	}
	if len(descriptions.Rows) == 1 {
		index, err := siteUnitTransferColumns(descriptions, "table_name", "description")
		if err != nil {
			return fail(err)
		}
		table, err := vegetationReportIdentity(descriptions.Rows[0].Cells[index["table_name"]])
		if err != nil || table == nil || *table != "USysAllSpecs" {
			return fail(errors.New("Long Vegetation version requires literal USysAllSpecs Description ownership"))
		}
		cell := descriptions.Rows[0].Cells[index["description"]]
		value, err := metadataCellValue(cell)
		if err != nil {
			return fail(err)
		}
		switch value.(type) {
		case nil:
			result.SpeciesVersion, result.VersionStatus = cloneSiteUnitCell(cell), "null"
		case string:
			result.SpeciesVersion, result.VersionStatus = cloneSiteUnitCell(cell), "literal"
		default:
			return fail(fmt.Errorf("Long Vegetation AllSpecs Description has unsupported %s storage", cell.Storage))
		}
	}
	if descriptions.Columns != nil {
		result.VersionDefinitions.Columns = append([]ProjectMetadataColumn{}, descriptions.Columns...)
	}
	if descriptions.Rows != nil {
		result.VersionDefinitions.Rows = make([]ProjectMetadataRow, len(descriptions.Rows))
		for i, row := range descriptions.Rows {
			cells := make([]ProjectMetadataCell, len(row.Cells))
			for j, cell := range row.Cells {
				cells[j] = cloneSiteUnitCell(cell)
			}
			result.VersionDefinitions.Rows[i] = ProjectMetadataRow{RowID: row.RowID, Cells: cells}
		}
	}
	return result, nil
}

func prepareVegetationWorkbookSummary(ctx context.Context, createdDate string, env, su, descriptions ProjectMetadataTable) (vegetationWorkbookSummary, error) {
	fail := func(err error) (vegetationWorkbookSummary, error) { return vegetationWorkbookSummary{}, err }
	header, err := prepareVegetationWorkbookSummaryHeader(ctx, createdDate, env, descriptions)
	if err != nil {
		return fail(err)
	}
	columns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	result := vegetationWorkbookSummary{
		CreatedDate: header.CreatedDate, EnvironmentRows: header.EnvironmentRows,
		SpeciesVersion: header.SpeciesVersion, VersionStatus: header.VersionStatus, VersionDefinitions: header.VersionDefinitions,
		Memberships: []vegetationWorkbookMembership{},
	}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[columns["PlotNumber"]], row.Cells[columns["SiteUnit"]]
		for _, cell := range []ProjectMetadataCell{plot, unit} {
			if _, err := vegetationReportIdentity(cell); err != nil {
				return fail(err)
			}
		}
		if plot.Storage != "null" {
			result.SelectedPlotRows++
		}
		result.Memberships = append(result.Memberships, vegetationWorkbookMembership{
			RowID: row.RowID, PlotNumber: cloneSiteUnitCell(plot), SiteUnit: cloneSiteUnitCell(unit),
		})
	}
	return result, nil
}

type vegetationWorkbookSummaryRow struct {
	PlotNumber ProjectMetadataCell
	SiteUnit   ProjectMetadataCell
}

func writeVegetationWorkbookSummary(ctx context.Context, book *excelize.File, preview LongVegetationPreview, summary vegetationWorkbookSummary) error {
	rows := make([]vegetationWorkbookSummaryRow, len(summary.Memberships))
	for i, member := range summary.Memberships {
		rows[i] = vegetationWorkbookSummaryRow{member.PlotNumber, member.SiteUnit}
	}
	header := vegetationWorkbookSummaryHeader{summary.CreatedDate, summary.EnvironmentRows, summary.SpeciesVersion,
		summary.VersionStatus, summary.VersionDefinitions}
	return writeVegetationWorkbookSummaryData(ctx, book, preview, header, summary.SelectedPlotRows, rows)
}

func writeVegetationProjectWorkbookSummary(ctx context.Context, book *excelize.File, preview LongVegetationPreview, summary vegetationProjectWorkbookSummary) error {
	rows := make([]vegetationWorkbookSummaryRow, len(summary.ProjectTuples))
	count := 0
	for i, tuple := range summary.ProjectTuples {
		rows[i] = vegetationWorkbookSummaryRow{tuple.PlotNumber, tuple.SiteUnit}
		if tuple.PlotNumber.Storage != "null" {
			count++
		}
	}
	if count != summary.SourcePlotRows {
		return errors.New("project summary non-NULL plot count differs from its original tuples")
	}
	if err := writeVegetationWorkbookSummaryData(ctx, book, preview, summary.Header, summary.SourcePlotRows, rows); err != nil {
		return err
	}
	return book.SetCellStr("ReportSummary", "D3", "Original project report tuples (desktop adaptation)")
}

func writeVegetationWorkbookSummaryData(ctx context.Context, book *excelize.File, preview LongVegetationPreview,
	header vegetationWorkbookSummaryHeader, plotRows int, rows []vegetationWorkbookSummaryRow) error {
	const sheet = "ReportSummary"
	if _, err := book.NewSheet(sheet); err != nil {
		return err
	}
	setText := func(address, value string) error {
		if err := environmentWorkbookText(value); err != nil {
			return err
		}
		return book.SetCellStr(sheet, address, value)
	}
	setIdentity := func(address string, cell ProjectMetadataCell) error {
		text, err := vegetationReportIdentity(cell)
		if err != nil {
			return err
		}
		if text == nil {
			return nil
		}
		return setText(address, *text)
	}
	date, err := time.Parse("2006-01-02", header.CreatedDate)
	if err != nil || date.Format("2006-01-02") != header.CreatedDate || date.Year() < 100 || header.EnvironmentRows < 0 || plotRows < 0 ||
		plotRows > len(rows) || len(rows) > 1048572 {
		return errors.New("Long Vegetation summary requires valid reviewed date, counts and bounded memberships")
	}
	settings := preview.Settings
	unitTable := preview.Report.SU + "_SU"
	if settings.UnitGrouping == "all-plots" || settings.UnitGrouping == "selected-fields" {
		unitTable = "USysSuTableDynamic_SU"
	}
	group := map[string]string{"layer": "Layer", "strata": "Strata", "lifeform": "Lifeform", "none": "None"}[settings.Grouping]
	order := map[string]string{"species": "Species", "presence": "Presence"}[settings.Order]
	for _, entry := range []struct{ address, value string }{
		{"A1", preview.Report.Title}, {"A2", "Date created: " + header.CreatedDate},
		{"A4", "Vegetation table:"}, {"B4", preview.Report.Project + "_Env"},
		{"A5", "Site unit table:"}, {"B5", unitTable},
		{"A6", "Lumping table:"}, {"B6", "None_Lump"},
		{"A7", "Data grouped by:"}, {"B7", group}, {"A8", "Data ordered by:"}, {"B8", order},
		{"A9", "Presence > than:"}, {"A10", "Mean cover > than:"},
		{"A11", "Number of plots in database"}, {"A12", "Number of plots in site unit table"},
		{"A13", "AllSpecs version:"}, {"D4", "Plot Number"}, {"E4", "Site Unit"},
	} {
		if err := setText(entry.address, entry.value); err != nil {
			return err
		}
	}
	for _, entry := range []struct {
		address string
		value   float64
	}{{"B9", settings.PresenceGreaterThan / 100}, {"B10", settings.MeanCoverGreaterThan}} {
		if err := book.SetCellFloat(sheet, entry.address, entry.value, -1, 64); err != nil {
			return err
		}
	}
	for _, entry := range []struct {
		address string
		value   int
	}{{"B11", header.EnvironmentRows}, {"B12", plotRows}} {
		if err := book.SetCellInt(sheet, entry.address, int64(entry.value)); err != nil {
			return err
		}
	}
	if err := setIdentity("B13", header.SpeciesVersion); err != nil {
		return err
	}
	for index, member := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		address := strconv.Itoa(index + 5)
		if err := setIdentity("D"+address, member.PlotNumber); err != nil {
			return err
		}
		if err := setIdentity("E"+address, member.SiteUnit); err != nil {
			return err
		}
	}
	if settings.Grouping == "lifeform" {
		for _, entry := range []struct{ address, value string }{{"G4", "Code"}, {"H4", "Description"}} {
			if err := setText(entry.address, entry.value); err != nil {
				return err
			}
		}
		for index, label := range []string{
			"genus-level and mixed", "coniferous tree", "broad-leaved tree", "evergreen shrub",
			"deciduous shrub", "fern or fern-ally", "graminoid", "forb", "parasite or saprophyte",
			"moss", "hepatic", "lichen", "dwarf woody plant", "macro alga",
		} {
			if err := book.SetCellInt(sheet, "G"+strconv.Itoa(index+5), int64(index)); err != nil {
				return err
			}
			if err := setText("H"+strconv.Itoa(index+5), label); err != nil {
				return err
			}
		}
	}
	for _, entry := range []struct {
		start, end string
		width      float64
	}{{"A", "A", 29}, {"B", "B", 32}, {"D", "E", 24}, {"G", "G", 10}, {"H", "H", 32}} {
		if err := book.SetColWidth(sheet, entry.start, entry.end, entry.width); err != nil {
			return fmt.Errorf("Long Vegetation summary layout: %w", err)
		}
	}
	return ctx.Err()
}
