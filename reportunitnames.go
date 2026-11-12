package main

import (
	"context"

	"fmt"
)

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
