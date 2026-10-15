package main

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

func projectSummaryVegetationAspect(ctx context.Context, env ProjectMetadataTable) (ProjectMetadataTable, error) {
	columns, err := siteUnitTransferColumns(env, "PlotNumber", "Aspect")
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	result := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn{}, env.Columns...), Rows: []ProjectMetadataRow{}}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, err
		}
		cells := make([]ProjectMetadataCell, len(row.Cells))
		for i, cell := range row.Cells {
			cells[i] = cloneSiteUnitCell(cell)
		}
		aspect := row.Cells[columns["Aspect"]]
		if aspect.Storage != "null" {
			value, err := metadataCellValue(aspect)
			number, integer := value.(int64)
			if err != nil || !integer || number < -32768 || number > 32767 {
				return ProjectMetadataTable{}, fmt.Errorf("environmental Aspect row %s requires original signed16 INTEGER storage", row.RowID)
			}
			label := summaryVegetationAspectText(number)
			cells[columns["Aspect"]] = ProjectMetadataCell{Storage: "text", Text: &label}
		}
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result, nil
}

func prepareSummaryVegetationBGCUnit(ctx context.Context, siteUnit *string, env, su ProjectMetadataTable) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) {
		return ProjectMetadataCell{}, fmt.Errorf("environmental BGC concatenation literal source preparation: %w", err)
	}
	if siteUnit == nil {
		return formatSummaryVegetationEnvironmentalCategories(ctx, "SVBioGeoClimaticUnit", nil, nil, 0)
	}
	if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: siteUnit}); err != nil {
		return fail(err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := siteUnitTransferColumns(env, "PlotNumber", "Zone", "SubZone")
	if err != nil {
		return fail(err)
	}
	memberships := map[string]int64{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		unit, err := vegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return fail(fmt.Errorf("SU row %s SiteUnit: %w", row.RowID, err))
		}
		plot, err := vegetationReportIdentity(row.Cells[sc["PlotNumber"]])
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
			values[i], err = vegetationReportIdentity(row.Cells[ec[field]])
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
