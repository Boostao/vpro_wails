package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import "context"

type SummaryVegetationEnvironmentalNumericRow struct {
	Field       vprocore.ProjectMetadataRow           `json:"field"`
	LabelColumn int                                   `json:"labelColumn"`
	Values      []SummaryVegetationEnvironmentalValue `json:"values"`
}

func summaryEnvironmentalIsNumeric(function string) bool {
	switch function {
	case "SVSeepage", "SVRootingDepth", "SVStandAge", "SVElevation", "SVSlope", "SVSubstrate", "SVStrataCover":
		return true
	default:
		return false
	}
}

func validateSummaryEnvironmentalNumericSelectedFunction(function string, argument vprocore.ProjectMetadataCell) error {
	if summaryEnvironmentalIsNumeric(function) {
		_, err := summaryEnvironmentalNumericArgument(function, argument)
		return err
	}
	_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
	return err
}

func prepareSummaryEnvironmentalNumericSelectedValue(ctx context.Context, function string,
	argument vprocore.ProjectMetadataCell, unit string, env, su vprocore.ProjectMetadataTable) (SummaryVegetationEnvironmentalValue, error) {
	if summaryEnvironmentalIsNumeric(function) {
		return prepareSummaryVegetationEnvironmentalNumericValue(ctx, function, argument, &unit, env, su)
	}
	cell, err := prepareSummaryEnvironmentalSelectedCell(ctx, function, argument, &unit, env, su)
	if err != nil {
		return SummaryVegetationEnvironmentalValue{}, err
	}
	return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
}

func planSummaryVegetationEnvironmentalNumericRows(ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su vprocore.ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		validateSummaryEnvironmentalNumericSelectedFunction,
		func(function string, argument vprocore.ProjectMetadataCell, unit string) (SummaryVegetationEnvironmentalValue, error) {
			return prepareSummaryEnvironmentalNumericSelectedValue(ctx, function, argument, unit, env, su)
		},
		func(field vprocore.ProjectMetadataRow, label int, values []SummaryVegetationEnvironmentalValue) SummaryVegetationEnvironmentalNumericRow {
			return SummaryVegetationEnvironmentalNumericRow{Field: field, LabelColumn: label, Values: values}
		})
}
