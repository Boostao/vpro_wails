package main

import (
	"context"
	"fmt"
)

func planSIVIParentText(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	owners := map[string]string{"SV_PolygonNumber": "Env", "SV_CanopyComposition": "Env"}
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, owners, func(column string, value ProjectMetadataCell) error {
		if value.Storage != "null" && value.Storage != "text" {
			return fmt.Errorf("SIVI %s requires explicit NULL or nonempty TEXT storage", column)
		}
		maximum := 25
		if column == "SV_CanopyComposition" {
			maximum = 50
		}
		return validateSiteCodeText(column, value.Text, maximum)
	})
}
