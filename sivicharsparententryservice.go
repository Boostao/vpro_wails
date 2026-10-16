package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type SIVICHARSParentEntryWrite struct {
	Original *SIVIParentProjection   `json:"original"`
	Edits    []SIVIParentCellEdit    `json:"edits"`
	Actions  []SIVIParentActionInput `json:"actions,omitempty"`
}

func (request *SIVICHARSParentEntryWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICHARSParentEntryWrite
	var decoded plain
	if err := validateSourceParentEntryJSONProperties(data, "CHARS parent entry"); err != nil {
		return fmt.Errorf("CHARS parent entry transport: %w", err)
	}
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("CHARS parent entry transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("CHARS parent entry properties: %w", err)
	}
	for key, value := range properties {
		switch key {
		case "original", "edits":
		case "actions":
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return errors.New("CHARS parent entry actions must be omitted or an explicit array")
			}
			var actions []json.RawMessage
			if err := json.Unmarshal(value, &actions); err != nil {
				return fmt.Errorf("CHARS parent entry action array: %w", err)
			}
			for _, raw := range actions {
				var action SIVIParentActionInput
				if err := decodeStrictRequiredJSON(raw, &action, "CHARS source action",
					"contextId", "controlId", "table", "rowId", "expected", "option"); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("CHARS parent entry has an unknown property %q", key)
		}
	}
	*request = SIVICHARSParentEntryWrite(decoded)
	return nil
}

type SIVICHARSParentEntryService struct {
	contexts *ContextService
	readers  siviParentSharedReferenceReaders
	enabled  bool
}

func newSIVICHARSParentEntryService(contexts *ContextService, readers siviParentSharedReferenceReaders,
	enabled bool) (*SIVICHARSParentEntryService, error) {
	if contexts == nil {
		return nil, errors.New("CHARS parent entry requires the owned context service")
	}
	return &SIVICHARSParentEntryService{contexts, readers, enabled}, nil
}

func (s *SIVICHARSParentEntryService) require(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.enabled {
		return errors.New("CHARS parent entry is independently disabled in this session")
	}
	return nil
}

func (s *SIVICHARSParentEntryService) Review(ctx context.Context, contextID, plot string) (*SIVICHARSParentEntryReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVICHARSParentReferences(ctx, contextID, plot, s.readers)
}

func (s *SIVICHARSParentEntryService) Save(ctx context.Context, contextID, plot string, request SIVICHARSParentEntryWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	actions := make([]siviParentActionEdit, len(request.Actions))
	for index, input := range request.Actions {
		actions[index] = siviParentActionEdit{input.ContextID, input.ControlID, input.Table, input.RowID, input.Expected, input.Option}
	}
	return s.contexts.writeSIVICHARSParentOwnedProposal(ctx, contextID, plot, request.Original, request.Edits, actions, s.readers)
}

func (s *SIVICHARSParentEntryService) Restore(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVICHARSParentEntry(ctx, contextID, plot, historyID, action)
}
