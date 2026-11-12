package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

func summaryVegetationAspectText(value int64) string {
	switch {
	case value == 999:
		return "Level"
	case value < 0 || value > 360:
		return "???"
	case value <= 11 || value >= 349:
		return "N"
	case value <= 33:
		return "NNE"
	case value <= 56:
		return "NE"
	case value <= 78:
		return "ENE"
	case value <= 101:
		return "E"
	case value <= 123:
		return "ESE"
	case value <= 146:
		return "SE"
	case value <= 168:
		return "SSE"
	case value <= 191:
		return "S"
	case value <= 213:
		return "SSW"
	case value <= 236:
		return "SW"
	case value <= 258:
		return "WSW"
	case value <= 281:
		return "W"
	case value <= 303:
		return "WNW"
	case value <= 326:
		return "NW"
	default:
		return "NNW"
	}
}

func projectSummaryVegetationAspect(ctx context.Context, env vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataTable, error) {
	columns, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", "Aspect")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	result := vprocore.ProjectMetadataTable{Columns: append([]vprocore.ProjectMetadataColumn{}, env.Columns...), Rows: []vprocore.ProjectMetadataRow{}}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, err
		}
		cells := make([]vprocore.ProjectMetadataCell, len(row.Cells))
		for i, cell := range row.Cells {
			cells[i] = vprocore.CloneSiteUnitCell(cell)
		}
		aspect := row.Cells[columns["Aspect"]]
		if aspect.Storage != "null" {
			value, err := vprocore.MetadataCellValue(aspect)
			number, integer := value.(int64)
			if err != nil || !integer || number < -32768 || number > 32767 {
				return vprocore.ProjectMetadataTable{}, fmt.Errorf("environmental Aspect row %s requires original signed16 INTEGER storage", row.RowID)
			}
			label := summaryVegetationAspectText(number)
			cells[columns["Aspect"]] = vprocore.ProjectMetadataCell{Storage: "text", Text: &label}
		}
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result, nil
}

func prepareSummaryVegetationBGCUnit(ctx context.Context, siteUnit *string, env, su vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) {
		return vprocore.ProjectMetadataCell{}, fmt.Errorf("environmental BGC concatenation literal source preparation: %w", err)
	}
	if siteUnit == nil {
		return formatSummaryVegetationEnvironmentalCategories(ctx, "SVBioGeoClimaticUnit", nil, nil, 0)
	}
	if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", "Zone", "SubZone")
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s PlotNumber: %w", row.RowID, err))
		}
		if unit != nil && *unit == *siteUnit && plot != nil {
			memberships[*plot]++
		}
	}
	counts := map[string]int64{}
	admitted := map[string]bool{}
	var nulls int64
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		values := make([]*string, 3)
		for i, field := range []string{"PlotNumber", "Zone", "SubZone"} {
			values[i], err = vprocore.VegetationReportIdentity(row.Cells[ec[field]])
			if err != nil {
				return fail(fmt.Errorf("Env row %s %s: %w", row.RowID, field, err))
			}
		}
		plot, zone, subzone := values[0], values[1], values[2]
		if plot == nil || memberships[*plot] == 0 {
			continue
		}
		weight := memberships[*plot]
		if zone == nil && subzone == nil {
			if weight > 2147483647-nulls {
				return fail(errors.New("joined NULL count exceeds source Long range"))
			}
			nulls += weight
			continue
		}
		label := ""
		if zone != nil {
			label += *zone
		}
		if subzone != nil {
			label += *subzone
		}
		if weight > 2147483647-counts[label] {
			return fail(errors.New("joined category count exceeds source Long range"))
		}
		counts[label] += weight
		if zone != nil {
			admitted[label] = true
		}
	}
	if len(admitted) > 32766 {
		return fail(errors.New("original large Integer category-loop boundary is unavailable"))
	}
	labels := make([]string, 0, len(admitted))
	for label := range admitted {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	categories := make([]summaryVegetationEnvironmentalCategory, 0, len(labels))
	for _, label := range labels {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		categories = append(categories, summaryVegetationEnvironmentalCategory{label: label, count: counts[label]})
	}
	return formatSummaryVegetationEnvironmentalCategories(ctx, "SVBioGeoClimaticUnit", siteUnit, categories, nulls)
}
