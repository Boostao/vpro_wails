package main

import (
	"context"
	"fmt"
)

type twoPageParentExtraField struct {
	owner, domain string
	maximum       int
}

var twoPageParentExtraFields = map[string]twoPageParentExtraField{
	"SV_WaterTableCM":        {"Env", "single", 0},
	"SV_FullCruiseCard":      {"Env", "text", 50},
	"ActiveLayerDepth":       {"Env", "single", 0},
	"PlotSize":               {"Admin", "single", 0},
	"ProvinceStateTerritory": {"Admin", "text", 255},
	"SiteUnitLongName":       {"Admin", "text", 100},
	"GIS_BGC":                {"Admin", "text", 255},
	"GIS_BGC_VER":            {"Admin", "integer", 0},
	"BEC_Use":                {"Admin", "text", 255},
}

func planTwoPageParentExtraFields(ctx context.Context, contextID, project, plot, form string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	parent, err := projectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(twoPageParentExtraFields))
	for column, field := range twoPageParentExtraFields {
		owners[column] = field.owner
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("two-page additional-field planning requires explicit edits")
	}
	result, err := planProjectedParentCells(ctx, contextID, project, parent, edits, owners, nil, validateTwoPageParentExtraField)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field planning: %w", err)
	}
	return result, nil
}

func validateTwoPageParentExtraField(column string, value ProjectMetadataCell) error {
	field, exists := twoPageParentExtraFields[column]
	if !exists {
		return fmt.Errorf("two-page field %q is outside additional-field source scope", column)
	}
	if value.Storage == "null" {
		return nil
	}
	switch field.domain {
	case "single":
		if value.Storage != "real" || value.Real == nil {
			return fmt.Errorf("two-page %s requires nullable SINGLE real storage", column)
		}
		return validateSingleRangeChange(column, nil, value.Real)
	case "integer":
		return validateSIVIParentSharedNumber(column, "integer", value)
	case "text":
		if value.Storage != "text" || value.Text == nil {
			return fmt.Errorf("two-page %s requires nullable text storage", column)
		}
		return validateSiteCodeText(column, value.Text, field.maximum)
	default:
		return fmt.Errorf("two-page %s has an unavailable source domain", column)
	}
}
