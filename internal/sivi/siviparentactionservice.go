package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type SIVIParentActionInput struct {
	ContextID string                       `json:"contextId"`
	ControlID string                       `json:"controlId"`
	Table     string                       `json:"table"`
	RowID     string                       `json:"rowId"`
	Expected  vprocore.ProjectMetadataCell `json:"expected"`
	Option    *int                         `json:"option"`
}

type SIVIParentActionWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Actions  []SIVIParentActionInput          `json:"actions"`
}

type SIVIParentActionSaveResult struct {
	ChangedCells          int
	HistoryID             string
	SourceRefreshRequired bool
}

func (input *SIVIParentActionInput) UnmarshalJSON(data []byte) error {
	type plain SIVIParentActionInput
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "contextId", "controlId", "table", "rowId", "expected"); err != nil {
		return fmt.Errorf("SIVI parent action transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if _, present := properties["option"]; !present {
		return errors.New("SIVI parent action transport requires an explicit option, including NULL")
	}
	*input = SIVIParentActionInput(decoded)
	return nil
}

func (request *SIVIParentActionWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentActionWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "actions"); err != nil {
		return fmt.Errorf("SIVI parent action-write transport: %w", err)
	}
	*request = SIVIParentActionWrite(decoded)
	return nil
}

func RequireSIVIParentActionEditing(s *vprocore.ContextService, ctx context.Context) error {
	return sourceform.RequireSIVIParentEditing(s, ctx)
}

func GetSIVIParentActionOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentProjection, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIParent(s, ctx, contextID, plot)
}

func SaveSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot string, request SIVIParentActionWrite) (*SIVIParentActionSaveResult, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	actions := make([]sourceform.SiviParentActionEdit, len(request.Actions))
	for index, input := range request.Actions {
		actions[index] = sourceform.SiviParentActionEdit{ContextID: input.ContextID, ControlID: input.ControlID, Table: input.Table, RowID: input.RowID, Expected: input.Expected, Option: input.Option}
	}
	result, err := WriteSIVIParentActions(s, ctx, contextID, plot, request.Original, actions)
	if err != nil {
		return nil, err
	}
	return &SIVIParentActionSaveResult{result.ChangedCells, result.HistoryID, result.SourceRefreshRequired}, nil
}

func RestoreSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	return CtxRestoreSIVIParentActions(s, ctx, contextID, plot, historyID, action)
}
