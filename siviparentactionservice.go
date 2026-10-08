package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const siviParentActionEditingFeatureEnvironment = "VPRO_SIVI_PARENT_ACTION_EDITING"

type SIVIParentActionInput struct {
	ContextID string              `json:"contextId"`
	ControlID string              `json:"controlId"`
	Table     string              `json:"table"`
	RowID     string              `json:"rowId"`
	Expected  ProjectMetadataCell `json:"expected"`
	Option    *int                `json:"option"`
}

type SIVIParentActionWrite struct {
	Original *SIVIParentProjection   `json:"original"`
	Actions  []SIVIParentActionInput `json:"actions"`
}

type SIVIParentActionSaveResult struct {
	ChangedCells          int
	HistoryID             string
	SourceRefreshRequired bool
}

func (input *SIVIParentActionInput) UnmarshalJSON(data []byte) error {
	type plain SIVIParentActionInput
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "contextId", "controlId", "table", "rowId", "expected"); err != nil {
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
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "actions"); err != nil {
		return fmt.Errorf("SIVI parent action-write transport: %w", err)
	}
	*request = SIVIParentActionWrite(decoded)
	return nil
}

func siviParentActionEditingFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siviParentActionEditingFeatureEnvironment, lookup)
}

func (s *ContextService) requireSIVIParentActionEditing(ctx context.Context) error {
	if err := s.requireSIVIParentEditing(ctx); err != nil {
		return err
	}
	if !s.siviParentActionEditingEnabled {
		return errors.New("SIVI source action editing requires enabled parent actions in this session")
	}
	return nil
}

func (s *ContextService) GetSIVIParentActionOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := s.requireSIVIParentActionEditing(ctx); err != nil {
		return nil, err
	}
	return s.readSIVIParent(ctx, contextID, plot)
}

func (s *ContextService) SaveSIVIParentActions(ctx context.Context, contextID, plot string, request SIVIParentActionWrite) (*SIVIParentActionSaveResult, error) {
	if err := s.requireSIVIParentActionEditing(ctx); err != nil {
		return nil, err
	}
	actions := make([]siviParentActionEdit, len(request.Actions))
	for index, input := range request.Actions {
		actions[index] = siviParentActionEdit{input.ContextID, input.ControlID, input.Table, input.RowID, input.Expected, input.Option}
	}
	result, err := s.writeSIVIParentActions(ctx, contextID, plot, request.Original, actions)
	if err != nil {
		return nil, err
	}
	return &SIVIParentActionSaveResult{result.ChangedCells, result.HistoryID, result.SourceRefreshRequired}, nil
}

func (s *ContextService) RestoreSIVIParentActions(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.requireSIVIParentActionEditing(ctx); err != nil {
		return nil, err
	}
	return s.restoreSIVIParentActions(ctx, contextID, plot, historyID, action)
}
