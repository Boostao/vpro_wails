package main

import (
	"context"
	"fmt"
)

func summaryEnvironmentalLifeformArgument(argument ProjectMetadataCell) (string, error) {
	if _, err := metadataCellValue(argument); err != nil {
		return "", fmt.Errorf("lifeform environmental FieldNumber: %w", err)
	}
	if argument.Storage != "text" || argument.Text == nil {
		return "", fmt.Errorf("SVLifeformCover requires an original literal TEXT FieldNumber1 through13")
	}
	switch *argument.Text {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13":
		return *argument.Text, nil
	default:
		return "", fmt.Errorf("SVLifeformCover has an unavailable literal FieldNumber %q", *argument.Text)
	}
}

func planSummaryVegetationEnvironmentalLifeformRows(ctx context.Context, fieldList ProjectMetadataTable,
	units []summaryVegetationUnit, env, su, quick, master ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalLifeformRowsWithMasterInteger(ctx, fieldList, units, env, su, quick, master,
		vegetationLifeformInteger)
}

func planSummaryEnvironmentalLifeformRowsWithMasterInteger(ctx context.Context, fieldList ProjectMetadataTable,
	units []summaryVegetationUnit, env, su, quick, master ProjectMetadataTable,
	readInteger func(ProjectMetadataCell) (*int, error)) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		func(function string, argument ProjectMetadataCell) error {
			if function == "SVLifeformCover" {
				_, err := summaryEnvironmentalLifeformArgument(argument)
				return err
			}
			return validateSummaryEnvironmentalNumericSelectedFunction(function, argument)
		},
		func(function string, argument ProjectMetadataCell, unit string) (SummaryVegetationEnvironmentalValue, error) {
			if function == "SVLifeformCover" {
				form, err := summaryEnvironmentalLifeformArgument(argument)
				if err != nil {
					return SummaryVegetationEnvironmentalValue{}, err
				}
				cell, err := prepareSummaryEnvironmentalLifeformWithMasterInteger(ctx, form, &unit, quick, master, su, readInteger)
				if err != nil {
					return SummaryVegetationEnvironmentalValue{}, err
				}
				return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
			}
			return prepareSummaryEnvironmentalNumericSelectedValue(ctx, function, argument, unit, env, su)
		},
		func(field ProjectMetadataRow, label int, values []SummaryVegetationEnvironmentalValue) SummaryVegetationEnvironmentalNumericRow {
			return SummaryVegetationEnvironmentalNumericRow{Field: field, LabelColumn: label, Values: values}
		})
}
