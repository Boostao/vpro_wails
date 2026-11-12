package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"

	"fmt"
)

func addLongVegetationUnitNames(ctx context.Context, report *vegetationLayerReport, master vprocore.ProjectMetadataTable) error {
	columns, err := vprocore.SiteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fmt.Errorf("Long Vegetation master name schema: %w", err)
	}
	rows, err := vprocore.SiteUnitTransferIndex(master, columns["SiteSeries"])
	if err != nil {
		return err
	}
	for i := range report.Units {
		if err := ctx.Err(); err != nil {
			return err
		}
		unit := &report.Units[i]
		code, err := vprocore.VegetationReportIdentity(unit.Code)
		if err != nil {
			return err
		}
		unit.NameCandidates = []vprocore.EnvironmentReportName{}
		if code == nil {
			empty := ""
			unit.LongName, unit.NameStatus = &empty, "unassigned"
			continue
		}
		names, err := reportcore.ResolveReportUnitNames(ctx, rows[*code], columns["SiteSeriesLongName"], "Long Vegetation")
		if err != nil {
			return err
		}
		unit.LongName, unit.NameStatus, unit.NameCandidates = names.LongName, names.Status, names.Candidates
		if names.Status != "unique" {
			report.Diagnostics = append(report.Diagnostics, vegetationLayerDiagnostic{
				Code: "unit_name_" + names.Status, Identity: reportcore.VegetationTextKey(unit.Code), Count: len(names.Candidates),
			})
		}
	}
	return ctx.Err()
}
