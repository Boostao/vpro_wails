package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	Env, UnitNames, Descriptions vprocore.ProjectMetadataTable
}

type summaryVegetationAncillaryInput struct {
	CreatedAt string
	SU, Veg   vprocore.ProjectMetadataTable
	Tables    summaryVegetationAncillaryTables
}

type summaryVegetationAncillaryMembership struct {
	SourceRowID          string
	PlotNumber, SiteUnit vprocore.ProjectMetadataCell
}

type summaryVegetationAncillaryUnit struct {
	Code     string
	NumPlots int64
	LongName vprocore.ProjectMetadataCell
}

type summaryVegetationAncillaryPlan struct {
	CreatedAt                 string
	EnvironmentRows           int
	EnvironmentCountAvailable bool
	SelectedPlotRows          int
	SpeciesVersion            vprocore.ProjectMetadataCell
	VersionStatus             string
	Memberships               []summaryVegetationAncillaryMembership
	Units                     []summaryVegetationAncillaryUnit
}

func readSummaryVegetationAncillaryTables(ctx context.Context, owner *vprocore.SqliteContext,
	tx *sql.Tx) (summaryVegetationAncillaryTables, error) {
	return readSummaryVegetationAncillaryTablesWithEnv(ctx, owner, tx, nil)
}

func readSummaryVegetationAncillaryTablesWithEnv(ctx context.Context, owner *vprocore.SqliteContext,
	tx *sql.Tx, retainedEnv *vprocore.ProjectMetadataTable) (summaryVegetationAncillaryTables, error) {
	fail := func(err error) (summaryVegetationAncillaryTables, error) {
		return summaryVegetationAncillaryTables{}, err
	}
	if ctx == nil || owner == nil || tx == nil || owner.Selection.Project == "" {
		return fail(errors.New("Summary ancillary input requires its owned project read snapshot"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	var result summaryVegetationAncillaryTables
	if retainedEnv != nil {
		if len(retainedEnv.Columns) == 0 {
			return fail(errors.New("Summary ancillary input requires a complete retained physical Env schema"))
		}
		result.Env = *retainedEnv
	}
	for _, source := range []struct {
		role, name string
		target     *vprocore.ProjectMetadataTable
	}{
		{"project", owner.Selection.Project + "_Env", &result.Env},
		{"VLists", "MasterSiteUnitList", &result.UnitNames},
	} {
		if source.role == "project" && retainedEnv != nil {
			continue
		}
		if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, source.role, source.name, "Summary ancillary input"); err != nil {
			return fail(err)
		}
		table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, source.role, source.name)
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary %s.%s: %w", source.role, source.name, err))
		}
		*source.target = table
	}
	descriptions, err := vprocore.ReadProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
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
	if err := reportcore.ReportWorkbookSourceValue(reflect.ValueOf(input), func(cell vprocore.ProjectMetadataCell) error {
		_, err := vprocore.MetadataCellValue(cell)
		return err
	}, "Summary ancillary"); err != nil {
		return fail(err)
	}
	memberships, selectedPlotRows, err := prepareSummaryAncillaryMemberships(ctx, input.SU)
	if err != nil {
		return fail(err)
	}
	veg, err := vprocore.SiteUnitTransferColumns(input.Veg, "PlotNumber")
	if err != nil {
		return fail(fmt.Errorf("Summary ancillary Veg: %w", err))
	}
	names, err := vprocore.SiteUnitTransferColumns(input.Tables.UnitNames, "SiteSeries", "SiteSeriesLongName")
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
		SpeciesVersion:            vprocore.CloneSiteUnitCell(header.SpeciesVersion), VersionStatus: header.VersionStatus,
		Memberships: memberships, SelectedPlotRows: selectedPlotRows, Units: []summaryVegetationAncillaryUnit{},
	}
	plots := map[string]bool{}
	for _, row := range input.Veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[veg["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary Veg row%s: %w", row.RowID, err))
		}
		if plot != nil {
			plots[*plot] = true
		}
	}
	selected := map[string]bool{}
	counts := map[string]int64{}
	for _, member := range memberships {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		p, u := member.PlotNumber.Text, member.SiteUnit.Text
		if p != nil {
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
		code, err := vprocore.VegetationReportIdentity(row.Cells[names["SiteSeries"]])
		if err != nil {
			return fail(fmt.Errorf("Summary ancillary master unit row%s: %w", row.RowID, err))
		}
		longName := row.Cells[names["SiteSeriesLongName"]]
		if _, err := vprocore.VegetationReportIdentity(longName); err != nil {
			return fail(fmt.Errorf("Summary ancillary master unit row%s long name: %w", row.RowID, err))
		}
		if code == nil || !selected[*code] {
			continue
		}
		unit, present := units[*code]
		if present && !vprocore.MetadataCellEqual(unit.LongName, longName) {
			return fail(fmt.Errorf("Summary ancillary unit %q has ambiguous source First long names; no representative inferred", *code))
		}
		if counts[*code] > math.MaxInt64-unit.NumPlots {
			return fail(errors.New("Summary ancillary joined plot count exceeds signed64"))
		}
		units[*code] = summaryVegetationAncillaryUnit{*code, unit.NumPlots + counts[*code], vprocore.CloneSiteUnitCell(longName)}
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

func prepareSummaryAncillaryMemberships(ctx context.Context, table vprocore.ProjectMetadataTable) ([]summaryVegetationAncillaryMembership, int, error) {
	if ctx == nil {
		return nil, 0, errors.New("Summary ancillary membership preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	columns, err := vprocore.SiteUnitTransferColumns(table, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, 0, fmt.Errorf("Summary ancillary SU: %w", err)
	}
	result, count := []summaryVegetationAncillaryMembership{}, 0
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		plot, unit := row.Cells[columns["PlotNumber"]], row.Cells[columns["SiteUnit"]]
		for _, field := range []struct {
			name string
			cell vprocore.ProjectMetadataCell
		}{{"PlotNumber", plot}, {"SiteUnit", unit}} {
			if _, err := vprocore.MetadataCellValue(field.cell); err != nil {
				return nil, 0, fmt.Errorf("Summary ancillary SU row%s %s: %w", row.RowID, field.name, err)
			}
			if _, err := vprocore.VegetationReportIdentity(field.cell); err != nil {
				return nil, 0, fmt.Errorf("Summary ancillary SU row%s %s: %w", row.RowID, field.name, err)
			}
		}
		result = append(result, summaryVegetationAncillaryMembership{
			SourceRowID: row.RowID, PlotNumber: vprocore.CloneSiteUnitCell(plot), SiteUnit: vprocore.CloneSiteUnitCell(unit),
		})
		if plot.Text != nil {
			count++
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return result, count, nil
}
