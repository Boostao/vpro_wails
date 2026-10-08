package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

type reportUnitNames struct {
	LongName   *string
	Status     string
	Candidates []EnvironmentReportName
}

func resolveReportUnitNames(ctx context.Context, rows []ProjectMetadataRow, column int, label string) (reportUnitNames, error) {
	result := reportUnitNames{Status: "missing", Candidates: []EnvironmentReportName{}}
	names, unsupported := map[string]bool{}, false
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return reportUnitNames{}, err
		}
		if column < 0 || column >= len(row.Cells) {
			return reportUnitNames{}, errors.New("report unit name column is outside the original reference row")
		}
		cell := row.Cells[column]
		if _, err := metadataCellValue(cell); err != nil {
			return reportUnitNames{}, fmt.Errorf("%s master name value: %w", label, err)
		}
		result.Candidates = append(result.Candidates, EnvironmentReportName{row.RowID, cloneSiteUnitCell(cell)})
		switch cell.Storage {
		case "text":
			names[*cell.Text] = true
		case "null":
		default:
			unsupported = true
		}
	}
	sort.Slice(result.Candidates, func(i, j int) bool { return result.Candidates[i].RowID < result.Candidates[j].RowID })
	switch {
	case unsupported:
		result.Status = "unsupported_storage"
	case len(names) > 1:
		result.Status = "conflicting"
	case len(names) == 1:
		result.Status = "unique"
		name := sortedEnvironmentReportKeys(names)[0]
		result.LongName = &name
	}
	if err := ctx.Err(); err != nil {
		return reportUnitNames{}, err
	}
	return result, nil
}

func addLongVegetationUnitNames(ctx context.Context, report *vegetationLayerReport, master ProjectMetadataTable) error {
	columns, err := siteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fmt.Errorf("Long Vegetation master name schema: %w", err)
	}
	rows, err := siteUnitTransferIndex(master, columns["SiteSeries"])
	if err != nil {
		return err
	}
	for i := range report.Units {
		if err := ctx.Err(); err != nil {
			return err
		}
		unit := &report.Units[i]
		code, err := vegetationReportIdentity(unit.Code)
		if err != nil {
			return err
		}
		unit.NameCandidates = []EnvironmentReportName{}
		if code == nil {
			empty := ""
			unit.LongName, unit.NameStatus = &empty, "unassigned"
			continue
		}
		names, err := resolveReportUnitNames(ctx, rows[*code], columns["SiteSeriesLongName"], "Long Vegetation")
		if err != nil {
			return err
		}
		unit.LongName, unit.NameStatus, unit.NameCandidates = names.LongName, names.Status, names.Candidates
		if names.Status != "unique" {
			report.Diagnostics = append(report.Diagnostics, vegetationLayerDiagnostic{
				Code: "unit_name_" + names.Status, Identity: vegetationTextKey(unit.Code), Count: len(names.Candidates),
			})
		}
	}
	return ctx.Err()
}
