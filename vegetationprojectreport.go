package main

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

type vegetationProjectUnitInput struct {
	EnvRowID   string
	AdminRowID *string
	PlotNumber ProjectMetadataCell
	SiteUnit   ProjectMetadataCell
}

func validateProjectUnitInputs(ctx context.Context, inputs []vegetationProjectUnitInput) ([]vegetationProjectUnitInput, error) {
	seen := map[struct{ env, admin string }]bool{}
	identity := selectedUnitIdentityReader()
	result := make([]vegetationProjectUnitInput, 0, len(inputs))
	var selected *bool
	for _, row := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hasAdmin := row.AdminRowID != nil
		if selected != nil && *selected != hasAdmin {
			return nil, errors.New("project report units cannot mix Env-only and Env/Admin provenance")
		}
		selected = &hasAdmin
		key := struct{ env, admin string }{env: row.EnvRowID}
		ids := []string{row.EnvRowID}
		if hasAdmin {
			key.admin = *row.AdminRowID
			ids = append(ids, *row.AdminRowID)
			if row.PlotNumber.Storage != "text" {
				return nil, errors.New("project selected unit requires an original non-NULL joined plot")
			}
		} else if row.SiteUnit.Storage != "text" || row.SiteUnit.Text == nil || *row.SiteUnit.Text != "All plots" {
			return nil, errors.New("project None unit requires its source All plots label")
		}
		for _, id := range ids {
			value, err := strconv.ParseInt(id, 10, 64)
			if err != nil || strconv.FormatInt(value, 10) != id {
				return nil, errors.New("project report units require exact original physical signed64 identities")
			}
		}
		if seen[key] {
			return nil, errors.New("project report units repeat an original physical tuple")
		}
		seen[key] = true
		if _, err := identity(row.PlotNumber); err != nil {
			return nil, err
		}
		label, err := vegetationReportIdentity(row.SiteUnit)
		if err != nil {
			return nil, err
		}
		if label != nil {
			if err := validateChildPhysicalText("project report unit label", *label, 255); err != nil {
				return nil, err
			}
		}
		row.AdminRowID = cloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
		result = append(result, row)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func projectReportPreparation(ctx context.Context, project vegetationProjectPreparation) (VegetationReportPreparation, error) {
	if ctx == nil {
		return VegetationReportPreparation{}, errors.New("project report kernel requires a context")
	}
	if err := ctx.Err(); err != nil {
		return VegetationReportPreparation{}, err
	}
	if project.Project == "" || project.SourceSU == "" || !utf8.ValidString(project.Project+project.SourceSU) ||
		strings.ContainsRune(project.Project+project.SourceSU, 0) ||
		(project.SelectedUnits == nil) == (project.NoneUnits == nil) ||
		project.NoneUnits != nil && project.SourceSU != "None" {
		return VegetationReportPreparation{}, errors.New("project report kernel requires one explicit exclusive original unit source")
	}
	inputs := []vegetationProjectUnitInput{}
	for _, row := range project.SelectedUnits {
		if err := ctx.Err(); err != nil {
			return VegetationReportPreparation{}, err
		}
		admin := row.AdminRowID
		inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, &admin, row.PlotNumber, row.SiteUnit})
	}
	for _, row := range project.NoneUnits {
		if err := ctx.Err(); err != nil {
			return VegetationReportPreparation{}, err
		}
		inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, nil, row.PlotNumber, row.SiteUnit})
	}
	validated, err := validateProjectUnitInputs(ctx, inputs)
	if err != nil {
		return VegetationReportPreparation{}, err
	}
	return VegetationReportPreparation{Project: project.Project, SU: project.SourceSU,
		CoverColumns: project.Entries.CoverColumns, Layers: project.Entries.Layers,
		ReducedRows: project.Entries.ReducedRows, Observations: project.Entries.Observations,
		Memberships: []VegetationReportMembership{}, projectUnitInputs: validated}, nil
}
