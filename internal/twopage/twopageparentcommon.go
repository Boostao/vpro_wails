package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

type twoPageParentCommonField struct {
	Owner, Domain string `json:"-" yaml:"-"`
}

var TwoPageParentCommonFields = map[string]twoPageParentCommonField{
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

func validateTwoPageParentCommonField(column string, value vprocore.ProjectMetadataCell) error {
	field, exists := TwoPageParentCommonFields[column]
	if !exists {
		return fmt.Errorf("two-page common field %q is outside its source scope", column)
	}
	switch field.Domain {
	case "scalar":
		return sourceform.ValidateSIVIParentScalar(column, value)
	case "option":
		return sourceform.ValidateSIVIParentOption(column, value)
	case "text":
		return sourceform.ValidateSIVIParentText(column, value)
	case "categorical":
		return sourceform.ValidateSIVIParentCategorical(column, value)
	case "plot-type":
		if value.Storage != "null" && value.Storage != "text" {
			return errors.New("two-page PlotType requires nullable TEXT storage")
		}
		return sqlutil.ValidateSiteCodeText(column, value.Text, 10)
	default:
		return fmt.Errorf("two-page %s has an unavailable common-field domain", column)
	}
}

func PlanTwoPageParentCommonProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("two-page common-field editing requires one owned physical pair and explicit edits")
	}
	parent, err := ProjectTwoPageParent(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(TwoPageParentCommonFields))
	for column, field := range TwoPageParentCommonFields {
		owners[column] = field.Owner
	}
	result, err := sourceform.PlanProjectedParentCells(ctx, original.ContextID, original.Project, parent, edits, owners, nil, validateTwoPageParentCommonField)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field planning: %w", err)
	}
	return result, nil
}

func TwoPageParentCommonHistory(form string) (sourceform.SiviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Common", "two-page common fields", PlanTwoPageParentCommonProjection)
}

func WriteTwoPageParentCommon(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) (*sourceform.SiviParentWriteResult, error) {
	history, err := TwoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, history,
		func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
			assignments, err := PlanTwoPageParentCommonProjection(ctx, observed, edits)
			if err != nil {
				return nil, err
			}
			if err := sourceform.ValidateSourceParentBooleanAssignments(assignments); err != nil {
				return nil, err
			}
			return assignments, nil
		}, sourceform.SiviParentWriteHooks{Read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page common-field writing: %w", err)
	}
	return result, nil
}

func RestoreTwoPageParentCommon(s *vprocore.ContextService, ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	history, err := TwoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.RestoreSourceParentHistory(s, ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field restoration: %w", err)
	}
	return result, nil
}
