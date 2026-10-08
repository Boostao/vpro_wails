package main

import (
	"context"
	"fmt"
)

func planSIVIParentCategorical(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	owners := map[string]string{"SnowCoverregime": "Env", "SV_RootZoneTexture": "Env", "SV_AhorizonType": "Env"}
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, owners, func(column string, value ProjectMetadataCell) error {
		if value.Storage != "null" && value.Storage != "text" {
			return fmt.Errorf("SIVI %s requires explicit NULL or TEXT storage", column)
		}
		if value.Text != nil && *value.Text == "" {
			return nil
		}
		maximum := 1
		switch column {
		case "SV_RootZoneTexture":
			maximum = 100
		case "SV_AhorizonType":
			maximum = 5
		}
		return validateSiteCodeText(column, value.Text, maximum)
	})
}
