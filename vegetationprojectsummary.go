package main

import (
	"context"
	"errors"
	"reflect"
)

type vegetationProjectWorkbookSummary struct {
	Header         vegetationWorkbookSummaryHeader
	SourceKind     string
	SourcePlotRows int
	ProjectTuples  []LongVegetationProjectUnit
}

func prepareVegetationProjectWorkbookSummary(ctx context.Context, createdDate string,
	env, admin, descriptions ProjectMetadataTable, scope LongVegetationReportScope,
	source vegetationLayerReport) (vegetationProjectWorkbookSummary, error) {
	fail := func(err error) (vegetationProjectWorkbookSummary, error) {
		return vegetationProjectWorkbookSummary{}, err
	}
	header, err := prepareVegetationWorkbookSummaryHeader(ctx, createdDate, env, descriptions)
	if err != nil {
		return fail(err)
	}
	if !scope.BuildCurrent || scope.Build == nil || scope.Project != source.Project || scope.SU != source.SU {
		return fail(errors.New("project summary requires its current independent original scope Build"))
	}
	report, err := longVegetationProjectTransport(ctx, source, scope.SourceKind)
	if err != nil {
		return fail(err)
	}
	full := []LongVegetationProjectUnit{}
	switch scope.SourceKind {
	case vegetationProjectFieldsScope:
		rows, err := planLongVegetationProjectSelectedUnits(ctx, env, admin, scope.Fields)
		if err != nil {
			return fail(err)
		}
		for _, row := range rows {
			id := row.AdminRowID
			full = append(full, LongVegetationProjectUnit{row.EnvRowID, &id, row.PlotNumber, row.SiteUnit})
		}
	case vegetationProjectAllScope:
		rows, err := planLongVegetationProjectNoneUnits(ctx, env)
		if err != nil {
			return fail(err)
		}
		for _, row := range rows {
			full = append(full, LongVegetationProjectUnit{row.EnvRowID, nil, row.PlotNumber, row.SiteUnit})
		}
	default:
		return fail(errors.New("project summary requires explicit original source2 or source3"))
	}
	if !reflect.DeepEqual(full, scope.Build.ProjectTuples) {
		return fail(errors.New("project summary physical field projection differs from complete original Build tuples"))
	}
	plots := map[string]bool{}
	if report.Quality != nil {
		for _, occurrence := range report.Quality.Occurrences {
			plots[vegetationTextKey(occurrence.PlotNumber)] = true
		}
	}
	result := vegetationProjectWorkbookSummary{Header: header, SourceKind: scope.SourceKind, ProjectTuples: []LongVegetationProjectUnit{}}
	for _, row := range full {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if report.Quality != nil && !plots[vegetationTextKey(row.PlotNumber)] {
			continue
		}
		row.AdminRowID = cloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
		result.ProjectTuples = append(result.ProjectTuples, row)
		if row.PlotNumber.Storage != "null" {
			result.SourcePlotRows++
		}
	}
	if !reflect.DeepEqual(result.ProjectTuples, report.ProjectUnits) {
		return fail(errors.New("project summary output differs from its exact original-SU qualified-plot subset"))
	}
	return result, nil
}
