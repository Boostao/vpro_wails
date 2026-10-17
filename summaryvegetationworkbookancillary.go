package main

import (
	"context"
	"errors"
	"time"
)

func validateSummaryWorkbookCreatedAt(value string) error {
	created, err := time.Parse(time.RFC3339, value)
	if err != nil || created.Year() < 100 || created.UTC().Format(time.RFC3339) != value {
		return errors.New("Summary ancillary workbook requires its explicit canonical reviewed UTC RFC3339 time")
	}
	return nil
}

func summaryWorkbookScope(options SummaryVegetationWorkbookOptions) string {
	if options.GroupAndOrder != nil {
		return summaryVegetationGroupedWorkbookScope
	}
	if options.References != nil {
		if options.Ancillary != nil {
			return summaryVegetationEffectiveAncillaryScope
		}
		return summaryVegetationEffectiveReferencesScope
	}
	if options.Ancillary != nil {
		return summaryVegetationAncillaryWorkbookScope
	}
	return summaryVegetationWorkbookDraftScope
}

func prepareSummaryWorkbookSource(ctx context.Context, source summaryVegetationWorkbookSource,
	options SummaryVegetationWorkbookOptions) (summaryVegetationWorkbook, error) {
	if options.GroupAndOrder != nil && (options.References != nil || options.Ancillary != nil ||
		source.Input.Vegetation.GroupAndOrder != *options.GroupAndOrder || source.Raw.Preview.GroupAndOrder != *options.GroupAndOrder) {
		return summaryVegetationWorkbook{}, errors.New("Summary grouped workbook requires its exact supported source/options without references or ancillary worksheets")
	}
	if options.Ancillary == nil {
		if source.Ancillary != nil {
			return summaryVegetationWorkbook{}, errors.New("Summary draft workbook has unexpected ancillary source authority")
		}
		if options.References != nil || options.GroupAndOrder != nil {
			return prepareSummaryVegetationWorkbookWithSheets(ctx, source.Input, struct {
				Source  summaryVegetationWorkbookSource
				Options SummaryVegetationWorkbookOptions
				Scope   string
			}{source, options, summaryWorkbookScope(options)}, nil)
		}
		return prepareSummaryVegetationWorkbook(ctx, source.Input)
	}
	if source.Ancillary == nil {
		return summaryVegetationWorkbook{}, errors.New("Summary ancillary workbook requires its complete reviewed source authority")
	}
	additional := source.Ancillary
	// The composer retains these tables and time separately; do not duplicate them in provenance.
	source.Ancillary = nil
	result, err := prepareSummaryVegetationAncillaryWorkbook(ctx, summaryVegetationAncillaryWorkbookInput{
		Source: source, Options: options, Tables: additional.Tables,
		CreatedAt: additional.CreatedAt, CreateSummary: options.Ancillary.CreateSummary,
	})
	if err != nil {
		return summaryVegetationWorkbook{}, err
	}
	if result.Ancillary != nil {
		for _, unit := range result.Ancillary.Units {
			if unit.NumPlots > 9007199254740991 {
				return summaryVegetationWorkbook{}, errors.New("Summary ancillary reviewed unit count exceeds the lossless client integer boundary")
			}
		}
	}
	return result, nil
}

func summaryWorkbookAncillaryReview(source summaryVegetationWorkbookSource, workbook summaryVegetationWorkbook,
	options SummaryVegetationWorkbookOptions) (*SummaryVegetationWorkbookAncillaryReview, error) {
	if source.Ancillary == nil || options.Ancillary == nil ||
		(workbook.Ancillary != nil) != options.Ancillary.CreateSummary {
		return nil, errors.New("Summary ancillary workbook review differs from its composed source plan")
	}
	result := &SummaryVegetationWorkbookAncillaryReview{
		CreatedAt: source.Ancillary.CreatedAt, MembershipRows: len(source.Raw.SU.Rows),
		Sheets: []string{workbook.Sheet, "SU Table"},
	}
	if plan := workbook.Ancillary; plan != nil {
		result.Sheets = append(result.Sheets, "ReportSummary")
		summary := &SummaryVegetationWorkbookReportSummary{
			EnvironmentRows: plan.EnvironmentRows, EnvironmentCountAvailable: plan.EnvironmentCountAvailable,
			SelectedPlotRows: plan.SelectedPlotRows, Version: "Unknown", VersionStatus: plan.VersionStatus,
			Units: []SummaryVegetationWorkbookAncillaryUnit{},
		}
		if plan.SpeciesVersion.Text != nil {
			summary.Version = *plan.SpeciesVersion.Text
		}
		for _, unit := range plan.Units {
			var longName *string
			if unit.LongName.Text != nil {
				value := *unit.LongName.Text
				longName = &value
			}
			summary.Units = append(summary.Units, SummaryVegetationWorkbookAncillaryUnit{
				Code: unit.Code, NumPlots: unit.NumPlots, LongName: longName,
			})
		}
		result.Summary = summary
	}
	return result, nil
}
