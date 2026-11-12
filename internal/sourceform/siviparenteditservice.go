package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
)

const siviParentEditingFeatureEnvironment = "VPRO_SIVI_PARENT_EDITING"

type SIVIParentDirectWrite struct {
	Original    *SIVIParentProjection `json:"original"`
	Scalars     []SIVIParentCellEdit  `json:"scalars"`
	Options     []SIVIParentCellEdit  `json:"options"`
	Text        []SIVIParentCellEdit  `json:"text"`
	Categorical []SIVIParentCellEdit  `json:"categorical"`
}

func (edit *SIVIParentCellEdit) UnmarshalJSON(data []byte) error {
	type plain SIVIParentCellEdit
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "contextId", "table", "rowId", "column", "expected", "value"); err != nil {
		return fmt.Errorf("SIVI parent direct-cell transport: %w", err)
	}
	*edit = SIVIParentCellEdit(decoded)
	return nil
}

func (request *SIVIParentDirectWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentDirectWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "scalars", "options", "text", "categorical"); err != nil {
		return fmt.Errorf("SIVI parent direct-write transport: %w", err)
	}
	*request = SIVIParentDirectWrite(decoded)
	return nil
}

func SiviParentEditingFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(siviParentEditingFeatureEnvironment, lookup)
}

func RequireSIVIParentEditing(s *vprocore.ContextService, ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.SiviParentReviewEnabled || !s.SiviParentEditingEnabled {
		return errors.New("SIVI directly bound parent editing requires enabled parent review and editing in this session")
	}
	return nil
}

func SaveSIVIParentDirect(s *vprocore.ContextService, ctx context.Context, contextID, plot string, request SIVIParentDirectWrite) (*SIVIParentWriteResult, error) {
	if err := RequireSIVIParentEditing(s, ctx); err != nil {
		return nil, err
	}
	return WriteSIVIParentDirect(s, ctx, contextID, plot, request.Original,
		SiviParentDirectEdits{request.Scalars, request.Options, request.Text, request.Categorical})
}

func GetSIVIParentDirectOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := RequireSIVIParentEditing(s, ctx); err != nil {
		return nil, err
	}
	return ReadSIVIParent(s, ctx, contextID, plot)
}

func RestoreSIVIParentDirect(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := RequireSIVIParentEditing(s, ctx); err != nil {
		return nil, err
	}
	return CtxRestoreSIVIParentDirect(s, ctx, contextID, plot, historyID, action)
}
