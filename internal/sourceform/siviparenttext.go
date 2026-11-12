package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"fmt"
)

func PlanSIVIParentText(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable, edits []SiviParentScalarEdit) ([]SiviParentScalarAssignment, error) {
	owners := map[string]string{"SV_PolygonNumber": "Env", "SV_CanopyComposition": "Env"}
	return planSIVIParentCells(ctx, contextID, project, plot, env, admin, edits, owners, ValidateSIVIParentText)
}

func ValidateSIVIParentText(column string, value vprocore.ProjectMetadataCell) error {
	if value.Storage != "null" && value.Storage != "text" {
		return fmt.Errorf("SIVI %s requires explicit NULL or nonempty TEXT storage", column)
	}
	maximum := 25
	if column == "SV_CanopyComposition" {
		maximum = 50
	}
	return sqlutil.ValidateSiteCodeText(column, value.Text, maximum)
}
