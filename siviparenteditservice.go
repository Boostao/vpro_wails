package main

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
	if err := decodeProfileLifecycleJSON(data, &decoded, "contextId", "table", "rowId", "column", "expected", "value"); err != nil {
		return fmt.Errorf("SIVI parent direct-cell transport: %w", err)
	}
	*edit = SIVIParentCellEdit(decoded)
	return nil
}

func (request *SIVIParentDirectWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentDirectWrite
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "scalars", "options", "text", "categorical"); err != nil {
		return fmt.Errorf("SIVI parent direct-write transport: %w", err)
	}
	*request = SIVIParentDirectWrite(decoded)
	return nil
}

func siviParentEditingFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siviParentEditingFeatureEnvironment, lookup)
}

func (s *ContextService) requireSIVIParentEditing(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.siviParentReviewEnabled || !s.siviParentEditingEnabled {
		return errors.New("SIVI directly bound parent editing requires enabled parent review and editing in this session")
	}
	return nil
}

func (s *ContextService) SaveSIVIParentDirect(ctx context.Context, contextID, plot string, request SIVIParentDirectWrite) (*SIVIParentWriteResult, error) {
	if err := s.requireSIVIParentEditing(ctx); err != nil {
		return nil, err
	}
	return s.writeSIVIParentDirect(ctx, contextID, plot, request.Original,
		siviParentDirectEdits{request.Scalars, request.Options, request.Text, request.Categorical})
}

func (s *ContextService) GetSIVIParentDirectOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := s.requireSIVIParentEditing(ctx); err != nil {
		return nil, err
	}
	return s.readSIVIParent(ctx, contextID, plot)
}

func (s *ContextService) RestoreSIVIParentDirect(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.requireSIVIParentEditing(ctx); err != nil {
		return nil, err
	}
	return s.restoreSIVIParentDirect(ctx, contextID, plot, historyID, action)
}
