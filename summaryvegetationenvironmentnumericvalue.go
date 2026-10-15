package main

import (
	"context"
	"errors"
	"fmt"
)

type SummaryVegetationEnvironmentalValue struct {
	Kind string               `json:"kind"`
	Cell *ProjectMetadataCell `json:"cell"`
}

func summaryEnvironmentalNumericArgument(function string, argument ProjectMetadataCell) (string, error) {
	if _, err := metadataCellValue(argument); err != nil {
		return "", fmt.Errorf("numeric environmental FieldNumber: %w", err)
	}
	switch function {
	case "SVSubstrate", "SVStrataCover":
		text, err := vegetationReportIdentity(argument)
		if err != nil || text == nil {
			return "", fmt.Errorf("function %q requires a calibrated literal TEXT FieldNumber suffix", function)
		}
		switch function {
		case "SVSubstrate":
			switch *text {
			case "DecWood", "BedRock", "Rocks", "MineralSoil", "OrganicMatter", "Water":
				return *text, nil
			}
		case "SVStrataCover":
			switch *text {
			case "Tree", "Shrub", "Herb", "Moss":
				return *text, nil
			}
		}
		return "", fmt.Errorf("function %q has an unavailable literal FieldNumber suffix %q", function, *text)
	case "SVSeepage", "SVRootingDepth", "SVStandAge", "SVElevation", "SVSlope":
		if argument.Storage != "null" {
			return "", fmt.Errorf("function %q requires an original NULL FieldNumber", function)
		}
		return "", nil
	default:
		return "", fmt.Errorf("function %q has no calibrated numeric environmental primitive", function)
	}
}

func prepareSummaryVegetationEnvironmentalNumericValue(ctx context.Context, function string,
	argument ProjectMetadataCell, siteUnit *string, env, su ProjectMetadataTable) (SummaryVegetationEnvironmentalValue, error) {
	fail := func(err error) (SummaryVegetationEnvironmentalValue, error) {
		return SummaryVegetationEnvironmentalValue{}, fmt.Errorf("numeric environmental value %s: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	suffix, err := summaryEnvironmentalNumericArgument(function, argument)
	if err != nil {
		return fail(err)
	}
	var result summaryVegetationEnvironmentalDepthResult
	switch function {
	case "SVStandAge", "SVElevation":
		cell, err := prepareSummaryVegetationEnvironmentalAgeElevation(ctx, function, siteUnit, env, su)
		if err != nil {
			return fail(err)
		}
		return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
	case "SVSeepage", "SVRootingDepth":
		result, err = prepareSummaryVegetationEnvironmentalDepth(ctx, function, siteUnit, env, su)
	case "SVSlope":
		result, err = prepareSummaryVegetationEnvironmentalSlope(ctx, siteUnit, env, su)
	case "SVSubstrate", "SVStrataCover":
		result, err = prepareSummaryVegetationEnvironmentalCover(ctx, function, suffix, siteUnit, env, su)
	}
	if err != nil {
		return fail(err)
	}
	if result.Empty {
		return SummaryVegetationEnvironmentalValue{Kind: "empty"}, nil
	}
	return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &ProjectMetadataCell{Storage: "text", Text: &result.Text}}, nil
}
