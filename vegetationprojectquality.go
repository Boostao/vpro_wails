package main

import "context"

// QCLV still owns original selected-SU witnesses. Only qualified plot existence
// prunes a complete project source2 Build; its fanout never weights that Build.
func planLongVegetationQualifiedProjectSelectedUnits(ctx context.Context,
	env, admin, su ProjectMetadataTable, names []string,
	quality vegetationQualitySelection) ([]vegetationProjectSelectedUnitOccurrence, error) {
	projectRows, err := planLongVegetationProjectSelectedUnits(ctx, env, admin, names)
	if err != nil {
		return nil, err
	}
	witnesses, err := planLongVegetationSelectedUnits(ctx, env, admin, su, names, &quality)
	if err != nil {
		return nil, err
	}
	qualified := map[string]bool{}
	for _, row := range witnesses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if row.PlotNumber.Text != nil {
			qualified[*row.PlotNumber.Text] = true
		}
	}
	result := []vegetationProjectSelectedUnitOccurrence{}
	for _, row := range projectRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if qualified[*row.PlotNumber.Text] {
			result = append(result, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
