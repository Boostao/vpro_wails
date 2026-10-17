package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"
)

type summaryVegetationAncillaryTables struct {
	Env, UnitNames, Descriptions ProjectMetadataTable
}

type summaryVegetationAncillaryInput struct {
	CreatedAt string
	SU, Veg   ProjectMetadataTable
	Tables    summaryVegetationAncillaryTables
}

type summaryVegetationAncillaryMembership struct {
	SourceRowID          string
	PlotNumber, SiteUnit ProjectMetadataCell
}

type summaryVegetationAncillaryUnit struct {
	Code     string
	NumPlots int64
	LongName ProjectMetadataCell
}

type summaryVegetationAncillaryPlan struct {
	CreatedAt                 string
	EnvironmentRows           int
	EnvironmentCountAvailable bool
	SelectedPlotRows          int
	SpeciesVersion            ProjectMetadataCell
	VersionStatus             string
	Memberships               []summaryVegetationAncillaryMembership
	Units                     []summaryVegetationAncillaryUnit
}

func readSummaryVegetationAncillaryTables(ctx context.Context, owner *sqliteContext,
	tx *sql.Tx) (summaryVegetationAncillaryTables, error) {
	fail := func(err error) (summaryVegetationAncillaryTables, error) {
		return summaryVegetationAncillaryTables{}, err
	}
	if ctx == nil || owner == nil || tx == nil || owner.selection.Project == "" {
		return fail(errors.New("Summary ancillary input requires its owned project read snapshot"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var result summaryVegetationAncillaryTables
	for _, source := range []struct {
		role, name string
		target     *ProjectMetadataTable
	}{
		{"project", owner.selection.Project + "_Env", &result.Env},
		{"VLists", "MasterSiteUnitList", &result.UnitNames},
	} {
		if err := validateSIVIPhysicalSchema(ctx, tx, source.role, source.name, "Summary ancillary input"); err != nil {
			return fail(err)
		}
		table, err := readPhysicalLocationTable(ctx, tx, source.role, source.name)
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary %s.%s: %w", source.role, source.name, err))
		}
		*source.target = table
	}
	descriptions, err := readProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary AllSpecs Description: %w", err))
	}
	result.Descriptions = descriptions
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

// This literal read model does not run the original master/temporary-table mutations.
func prepareSummaryVegetationAncillary(ctx context.Context,
	input summaryVegetationAncillaryInput) (summaryVegetationAncillaryPlan, error) {
	fail := func(err error) (summaryVegetationAncillaryPlan, error) {
		return summaryVegetationAncillaryPlan{}, err
	}
	if ctx == nil {
		return fail(errors.New("Summary ancillary preparation requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	created, err := time.Parse(time.RFC3339, input.CreatedAt)
	if err != nil || created.Year() < 100 || created.Format(time.RFC3339) != input.CreatedAt {
		return fail(errors.New("Summary ancillary preparation requires an explicit canonical reviewed RFC3339 time"))
	}
	if err := reportWorkbookSourceValue(reflect.ValueOf(input), func(cell ProjectMetadataCell) error {
		_, err := metadataCellValue(cell)
		return err
	}, "Summary ancillary"); err != nil {
		return fail(err)
	}
	su, err := siteUnitTransferColumns(input.SU, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary SU: %w", err))
	}
	veg, err := siteUnitTransferColumns(input.Veg, "PlotNumber")
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary Veg: %w", err))
	}
	names, err := siteUnitTransferColumns(input.Tables.UnitNames, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary master units: %w", err))
	}
	if len(input.Tables.Env.Columns) == 0 {
		return fail(errors.New("Summary ancillary count requires the complete physical Env schema; unavailable is not empty"))
	}
	header, err := prepareVegetationWorkbookSummaryHeader(ctx, created.Format("2006-01-02"),
		input.Tables.Env, input.Tables.Descriptions)
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary version/count inputs: %w", err))
	}
	result := summaryVegetationAncillaryPlan{
		CreatedAt: input.CreatedAt, EnvironmentRows: header.EnvironmentRows,
		EnvironmentCountAvailable: header.EnvironmentRows != 0,
		SpeciesVersion:            cloneSiteUnitCell(header.SpeciesVersion), VersionStatus: header.VersionStatus,
		Memberships: []summaryVegetationAncillaryMembership{}, Units: []summaryVegetationAncillaryUnit{},
	}
	plots := map[string]bool{}
	for _, row := range input.Veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vegetationReportIdentity(row.Cells[veg["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary Veg row%s: %w", row.RowID, err))
		}
		if plot != nil {
			plots[*plot] = true
		}
	}
	selected := map[string]bool{}
	counts := map[string]int64{}
	for _, row := range input.SU.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[su["PlotNumber"]], row.Cells[su["SiteUnit"]]
		p, err := vegetationReportIdentity(plot)
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary SU row%s PlotNumber: %w", row.RowID, err))
		}
		u, err := vegetationReportIdentity(unit)
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary SU row%s SiteUnit: %w", row.RowID, err))
		}
		result.Memberships = append(result.Memberships, summaryVegetationAncillaryMembership{
			SourceRowID: row.RowID, PlotNumber: cloneSiteUnitCell(plot), SiteUnit: cloneSiteUnitCell(unit),
		})
		if p != nil {
			result.SelectedPlotRows++
			if u != nil {
				counts[*u]++
				if plots[*p] {
					selected[*u] = true
				}
			}
		}
	}
	units := map[string]summaryVegetationAncillaryUnit{}
	for _, row := range input.Tables.UnitNames.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		code, err := vegetationReportIdentity(row.Cells[names["SiteSeries"]])
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary master unit row%s: %w", row.RowID, err))
		}
		longName := row.Cells[names["SiteSeriesLongName"]]
		if _, err := vegetationReportIdentity(longName); err != nil {
			return fail(fmt.Errorf("Summary ancillary master unit row%s long name: %w", row.RowID, err))
		}
		if code == nil || !selected[*code] {
			continue
		}
		unit, present := units[*code]
		if present && !metadataCellEqual(unit.LongName, longName) {
			return fail(fmt.Errorf("Summary ancillary unit %q has ambiguous source First long names; no representative inferred", *code))
		}
		if counts[*code] > math.MaxInt64-unit.NumPlots {
			return fail(errors.New("Summary ancillary joined plot count exceeds signed64"))
		}
		units[*code] = summaryVegetationAncillaryUnit{*code, unit.NumPlots + counts[*code], cloneSiteUnitCell(longName)}
	}
	codes := make([]string, 0, len(units))
	for code := range units {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		result.Units = append(result.Units, units[code])
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
