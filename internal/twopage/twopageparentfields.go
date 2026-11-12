package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
)

func planTwoPageParentExtraFields(ctx context.Context, contextID, project, plot, form string, env, admin vprocore.ProjectMetadataTable, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	parent, err := ProjectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(sourceform.TwoPageParentExtraFields))
	for column, field := range sourceform.TwoPageParentExtraFields {
		owners[column] = field.Owner
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("two-page additional-field planning requires explicit edits")
	}
	result, err := sourceform.PlanProjectedParentCells(ctx, contextID, project, parent, edits, owners, nil, validateTwoPageParentExtraField)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field planning: %w", err)
	}
	return result, nil
}

func validateTwoPageParentExtraField(column string, value vprocore.ProjectMetadataCell) error {
	field, exists := sourceform.TwoPageParentExtraFields[column]
	if !exists {
		return fmt.Errorf("two-page field %q is outside additional-field source scope", column)
	}
	if value.Storage == "null" {
		return nil
	}
	switch field.Domain {
	case "single":
		if value.Storage != "real" || value.Real == nil {
			return fmt.Errorf("two-page %s requires nullable SINGLE real storage", column)
		}
		return vprocore.ValidateSingleRangeChange(column, nil, value.Real)
	case "integer":
		return sourceform.ValidateSIVIParentSharedNumber(column, "integer", value)
	case "text":
		if value.Storage != "text" || value.Text == nil {
			return fmt.Errorf("two-page %s requires nullable text storage", column)
		}
		return sqlutil.ValidateSiteCodeText(column, value.Text, field.Maximum)
	default:
		return fmt.Errorf("two-page %s has an unavailable source domain", column)
	}
}
