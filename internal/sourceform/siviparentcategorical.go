package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
)

func PlanSIVIParentCategorical(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable, edits []SiviParentScalarEdit) ([]SiviParentScalarAssignment, error) {
	owners := map[string]string{"SnowCoverregime": "Env", "SV_RootZoneTexture": "Env", "SV_AhorizonType": "Env"}
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, owners, ValidateSIVIParentCategorical)
}

func ValidateSIVIParentCategorical(column string, value vprocore.ProjectMetadataCell) error {
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
	return sqlutil.ValidateSiteCodeText(column, value.Text, maximum)
}
