package main

import (
	"context"
	"errors"
	"fmt"
)

type twoPageParentCommonField struct {
	owner, domain string
}

var twoPageParentCommonFields = map[string]twoPageParentCommonField{
	"SV_StandHeight":        {"Env", "scalar"},
	"SV_AhorizonDepth":      {"Env", "scalar"},
	"SV_GleyingMottlingCM":  {"Env", "scalar"},
	"SV_PercentCoarseFrags": {"Env", "scalar"},
	"SV_SoilDepth":          {"Env", "scalar"},
	"StrataCoverTotal":      {"Admin", "scalar"},
	"SV_FloodPlain":         {"Env", "scalar"},
	"SV_StandAgeEstMeas":    {"Env", "option"},
	"SV_StandHeightEstMeas": {"Env", "option"},
	"SV_PolygonNumber":      {"Env", "text"},
	"SV_CanopyComposition":  {"Env", "text"},
	"SV_RootZoneTexture":    {"Env", "categorical"},
	"SV_AhorizonType":       {"Env", "categorical"},
	"PlotType":              {"Admin", "plot-type"},
}

func validateTwoPageParentCommonField(column string, value ProjectMetadataCell) error {
	field, exists := twoPageParentCommonFields[column]
	if !exists {
		return fmt.Errorf("two-page common field %q is outside its source scope", column)
	}
	switch field.domain {
	case "scalar":
		return validateSIVIParentScalar(column, value)
	case "option":
		return validateSIVIParentOption(column, value)
	case "text":
		return validateSIVIParentText(column, value)
	case "categorical":
		return validateSIVIParentCategorical(column, value)
	case "plot-type":
		if value.Storage != "null" && value.Storage != "text" {
			return errors.New("two-page PlotType requires nullable TEXT storage")
		}
		return validateSiteCodeText(column, value.Text, 10)
	default:
		return fmt.Errorf("two-page %s has an unavailable common-field domain", column)
	}
}

func planTwoPageParentCommonProjection(ctx context.Context, original *siviParentProjection, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("two-page common-field editing requires one owned physical pair and explicit edits")
	}
	parent, err := projectTwoPageParent(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}},
		ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(twoPageParentCommonFields))
	for column, field := range twoPageParentCommonFields {
		owners[column] = field.owner
	}
	result, err := planProjectedParentCells(ctx, original.ContextID, original.Project, parent, edits, owners, nil, validateTwoPageParentCommonField)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field planning: %w", err)
	}
	return result, nil
}

func twoPageParentCommonHistory(form string) (siviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Common", "two-page common fields", planTwoPageParentCommonProjection)
}

func (s *ContextService) writeTwoPageParentCommon(ctx context.Context, contextID, plot, form string, original *siviParentProjection, edits []siviParentScalarEdit) (*siviParentWriteResult, error) {
	history, err := twoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := s.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, original, history,
		func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			return planTwoPageParentCommonProjection(ctx, observed, edits)
		}, siviParentWriteHooks{read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page common-field writing: %w", err)
	}
	return result, nil
}

func (s *ContextService) restoreTwoPageParentCommon(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	history, err := twoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := twoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := s.restoreSourceParentHistory(ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field restoration: %w", err)
	}
	return result, nil
}
