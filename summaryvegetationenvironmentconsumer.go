package main

import (
	"context"
	"errors"
	"fmt"
)

type SummaryVegetationEnvironmentalRow struct {
	Field       ProjectMetadataRow    `json:"field"`
	LabelColumn int                   `json:"labelColumn"`
	Values      []ProjectMetadataCell `json:"values"`
}

func planSummaryVegetationEnvironmentalRows(ctx context.Context, fieldList ProjectMetadataTable,
	units []summaryVegetationUnit, env, su ProjectMetadataTable) ([]SummaryVegetationEnvironmentalRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		func(function string, argument ProjectMetadataCell) error {
			_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
			return err
		},
		func(function string, argument ProjectMetadataCell, unit string) (ProjectMetadataCell, error) {
			return prepareSummaryEnvironmentalSelectedCell(ctx, function, argument, &unit, env, su)
		},
		func(field ProjectMetadataRow, label int, values []ProjectMetadataCell) SummaryVegetationEnvironmentalRow {
			return SummaryVegetationEnvironmentalRow{Field: field, LabelColumn: label, Values: values}
		})
}

func prepareSummaryEnvironmentalSelectedCell(ctx context.Context, function string, argument ProjectMetadataCell,
	unit *string, env, su ProjectMetadataTable) (ProjectMetadataCell, error) {
	dynamic, number, err := summaryEnvironmentalSelectedFunction(function, argument)
	if err != nil {
		return ProjectMetadataCell{}, err
	}
	if function == "SVnPlots" {
		return prepareSummaryVegetationEnvironmentalPlotCount(ctx, unit, su)
	}
	if dynamic {
		return prepareSummaryVegetationEnvironmentalDynamicCategory(ctx, function, number, unit, env, su)
	}
	return prepareSummaryVegetationEnvironmentalCategories(ctx, function, unit, env, su)
}

func planSummaryEnvironmentalSelectedRows[V, R any](ctx context.Context, fieldList ProjectMetadataTable,
	units []summaryVegetationUnit, env, su ProjectMetadataTable,
	validate func(string, ProjectMetadataCell) error,
	prepare func(string, ProjectMetadataCell, string) (V, error),
	build func(ProjectMetadataRow, int, []V) R) ([]R, error) {
	fail := func(err error) ([]R, error) {
		return nil, fmt.Errorf("Summary environmental selected-field matrix: %w", err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(units) == 0 || len(units) > 250 {
		return fail(errors.New("requires an available nonempty prepared SV1 unit frame within source250-group limit"))
	}
	seen := map[string]bool{}
	for _, unit := range units {
		if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: &unit.Code}); err != nil {
			return fail(err)
		}
		if seen[unit.Code] {
			return fail(errors.New("prepared SV1 unit frame has duplicate literal unit identities"))
		}
		seen[unit.Code] = true
	}
	if err := validateSummaryVegetationFieldListWitness(ctx, fieldList); err != nil {
		return fail(err)
	}
	index, err := indexSummaryVegetationFieldList(ctx, fieldList)
	if err != nil {
		return fail(err)
	}
	rows := make(map[string]ProjectMetadataRow, len(fieldList.Rows))
	for _, row := range fieldList.Rows {
		rows[row.RowID] = row
	}
	result := make([]R, 0, len(index.Selected))
	for _, id := range index.Selected {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		row := rows[id]
		function, err := vegetationReportIdentity(row.Cells[5])
		if err != nil || function == nil {
			return fail(fmt.Errorf("field-list row %s requires an original TEXT FunctionName", id))
		}
		if err := validate(*function, row.Cells[6]); err != nil {
			return fail(fmt.Errorf("field-list row %s: %w", id, err))
		}
		for _, position := range []int{1, 2} {
			if _, err := vegetationReportIdentity(row.Cells[position]); err != nil {
				return fail(fmt.Errorf("field-list row %s caption: %w", id, err))
			}
		}
		label := 2
		values := make([]V, 0, len(units))
		if row.Cells[2].Text != nil && *row.Cells[2].Text == "n Plots" {
			label = 3
		}
		for _, unit := range units {
			value, err := prepare(*function, row.Cells[6], unit.Code)
			if err != nil {
				return fail(fmt.Errorf("field-list row %s unit %q: %w", id, unit.Code, err))
			}
			values = append(values, value)
		}
		result = append(result, build(cloneSummaryVegetationFieldListRow(row), label, values))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
