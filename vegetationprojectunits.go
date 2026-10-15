package main

import (
	"context"
	"errors"
	"fmt"
)

type vegetationProjectSelectedUnitOccurrence struct {
	EnvRowID   string
	AdminRowID string
	PlotNumber ProjectMetadataCell
	SiteUnit   ProjectMetadataCell
}

type vegetationProjectNoneUnitOccurrence struct {
	EnvRowID   string
	PlotNumber ProjectMetadataCell
	SiteUnit   ProjectMetadataCell
}

// Project source2 uses UsysEnv's original Env INNER JOIN Admin tuples, not SU
// memberships. Project source3 below instead preserves each original Env row.
func planLongVegetationProjectSelectedUnits(ctx context.Context, env, admin ProjectMetadataTable,
	names []string) ([]vegetationProjectSelectedUnitOccurrence, error) {
	if ctx == nil {
		return nil, errors.New("project selected report units require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ec, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("project selected report units Env: %w", err)
	}
	ac, err := siteUnitTransferColumns(admin, "Plot")
	if err != nil {
		return nil, fmt.Errorf("project selected report units Admin: %w", err)
	}
	fields, err := selectedUnitFields(env, admin, names)
	if err != nil {
		return nil, fmt.Errorf("project selected report units: %w", err)
	}
	type parent struct {
		row    ProjectMetadataRow
		plot   *string
		values []*string
	}
	identity := selectedUnitIdentityReader()
	read := func(table ProjectMetadataTable, plotIndex int, isAdmin bool) ([]parent, error) {
		parents := make([]parent, 0, len(table.Rows))
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			plot, err := identity(row.Cells[plotIndex])
			if err != nil {
				return nil, fmt.Errorf("physical row %s project plot: %w", row.RowID, err)
			}
			p := parent{row: row, plot: plot, values: make([]*string, len(fields))}
			for i, field := range fields {
				if field.admin != isAdmin {
					continue
				}
				p.values[i], err = selectedUnitScalar(row.Cells[field.index], field.kind)
				if err != nil {
					return nil, fmt.Errorf("physical row %s project field %s: %w", row.RowID, table.Columns[field.index].Name, err)
				}
			}
			parents = append(parents, p)
		}
		return parents, nil
	}
	envRows, err := read(env, ec["PlotNumber"], false)
	if err != nil {
		return nil, err
	}
	adminRows, err := read(admin, ac["Plot"], true)
	if err != nil {
		return nil, err
	}
	adminPlots := map[string][]parent{}
	for _, row := range adminRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if row.plot != nil {
			adminPlots[*row.plot] = append(adminPlots[*row.plot], row)
		}
	}
	result := []vegetationProjectSelectedUnitOccurrence{}
	for _, e := range envRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.plot == nil {
			continue
		}
		for _, a := range adminPlots[*e.plot] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			values := make([]*string, len(fields))
			for i, field := range fields {
				values[i] = e.values[i]
				if field.admin {
					values[i] = a.values[i]
				}
			}
			label, err := selectedUnitLabel(values)
			if err != nil {
				return nil, fmt.Errorf("project selected report unit Env %s/Admin %s: %w", e.row.RowID, a.row.RowID, err)
			}
			result = append(result, vegetationProjectSelectedUnitOccurrence{
				EnvRowID: e.row.RowID, AdminRowID: a.row.RowID,
				PlotNumber: cloneSiteUnitCell(e.row.Cells[ec["PlotNumber"]]), SiteUnit: label,
			})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func planLongVegetationProjectNoneUnits(ctx context.Context, env ProjectMetadataTable) ([]vegetationProjectNoneUnitOccurrence, error) {
	if ctx == nil {
		return nil, errors.New("project None report units require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	columns, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("project None report units Env: %w", err)
	}
	identity := selectedUnitIdentityReader()
	result := make([]vegetationProjectNoneUnitOccurrence, 0, len(env.Rows))
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot := row.Cells[columns["PlotNumber"]]
		if _, err := identity(plot); err != nil {
			return nil, fmt.Errorf("physical row %s project None plot: %w", row.RowID, err)
		}
		label := "All plots"
		result = append(result, vegetationProjectNoneUnitOccurrence{
			EnvRowID: row.RowID, PlotNumber: cloneSiteUnitCell(plot),
			SiteUnit: ProjectMetadataCell{Storage: "text", Text: &label},
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
