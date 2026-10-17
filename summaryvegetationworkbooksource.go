package main

import (
	"context"
	"database/sql"
	"errors"
)

type summaryVegetationWorkbookOptions = SummaryVegetationWorkbookOptions

type summaryVegetationWorkbookSourcePath struct {
	Role string
	Path string
}

type summaryVegetationWorkbookSource struct {
	Input       summaryVegetationWorkbookInput
	Raw         summaryVegetationSnapshotPreview
	Environment summaryEnvironmentalPhysicalInput
	Paths       []summaryVegetationWorkbookSourcePath
	Ancillary   *summaryVegetationWorkbookAncillarySource `json:"Ancillary,omitempty"`
}

type summaryVegetationWorkbookAncillarySource struct {
	CreatedAt string
	Tables    summaryVegetationAncillaryTables
}

func readSummaryVegetationWorkbookSource(ctx context.Context, service *SummaryVegetationEnvironmentalLifeformService,
	plots *PlotService, owner *sqliteContext, tx *sql.Tx, contextID string,
	options summaryVegetationWorkbookOptions) (summaryVegetationWorkbookSource, error) {
	fail := func(err error) (summaryVegetationWorkbookSource, error) {
		return summaryVegetationWorkbookSource{}, err
	}
	if ctx == nil {
		return fail(errors.New("Summary workbook source requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if service == nil || service.environment == nil || service.environment.report == nil ||
		!service.environment.report.enabled {
		return fail(errors.New("Summary workbook source requires enabled Summary Vegetation"))
	}
	environment := service.environment
	if options.IncludeEnv && (!service.enabled || !environment.enabled || !environment.numericEnabled) {
		return fail(errors.New("Summary workbook environmental header requires enabled environmental, numeric and lifeform inspection"))
	}
	if plots == nil || plots.projects == nil || owner == nil || plots.projects.sqlite != owner || tx == nil {
		return fail(errors.New("Summary workbook source requires the owned plot context and read transaction"))
	}
	contexts := environment.report.contexts
	if contexts == nil || contexts.projects == nil || contexts.projects.sqlite != owner ||
		!plots.contextScoped || contextID == "" || plots.projects.contextID != contextID {
		return fail(errors.New("Summary workbook source requires its current scoped report context"))
	}
	if err := validateGoogleEarthReviewStrings(contextID, options.Title); err != nil {
		return fail(err)
	}
	if options.ShowClass != 0 && options.ShowClass != 1 && options.ShowClass != 10 && options.ShowClass != 20 {
		return fail(errors.New("Summary workbook requires supported retained ShowClass0,1,10 or20"))
	}
	calculationClass := options.ShowClass
	if options.Display != 4 && options.Display != 5 {
		// These source displays retain Show but do not consume it; keep the legacy calculator policy unchanged.
		calculationClass = 0
	}
	if err := validateSummaryVegetationDisplayOptions(ctx, options.Average, options.Display, calculationClass); err != nil {
		return fail(err)
	}
	grouping := 11
	if options.GroupAndOrder != nil {
		if options.References != nil || options.Ancillary != nil {
			return fail(errors.New("Summary grouped workbook effective references and ancillary worksheets are not qualified; choose Layer or clear those modes"))
		}
		grouping = *options.GroupAndOrder
		if err := validateSummaryVegetationGroupedOptions(ctx, options.Average, grouping, options.Display, calculationClass); err != nil {
			return fail(err)
		}
	}
	raw, err := environment.report.readSnapshotWithReferences(ctx, contextID, plots, owner, tx,
		options.Average, options.Display, calculationClass, grouping, options.References)
	if err != nil {
		return fail(err)
	}
	result := summaryVegetationWorkbookSource{Raw: raw,
		Input: summaryVegetationWorkbookInput{Title: options.Title, Vegetation: raw.Preview},
		Paths: []summaryVegetationWorkbookSourcePath{}}
	for _, role := range []string{"project", "su", "VLists", "VUser", "VPro64"} {
		path, ok := owner.attachments[role]
		if !ok || path == "" {
			return fail(errors.New("Summary workbook source is missing a required owned attachment path"))
		}
		result.Paths = append(result.Paths, summaryVegetationWorkbookSourcePath{role, path})
	}
	if options.IncludeEnv {
		result.Environment, err = readSummaryEnvironmentalTables(ctx, owner, tx, validateSummaryEnvironmentalLifeformSelectedFunction)
		if err != nil {
			return fail(err)
		}
		rows, err := planSummaryEnvironmentalSnapshotRows(ctx, raw, result.Environment.FieldList, result.Environment.Env)
		if err != nil {
			return fail(err)
		}
		result.Input.Environment = &summaryEnvironmentalWorkbookInput{
			FirstRow: 5, FieldList: result.Environment.FieldList,
			Units: summaryEnvironmentalPreviewUnits(raw.Preview), Rows: rows,
			English: raw.Preview.References != nil && raw.Preview.References.ShowCommon,
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
