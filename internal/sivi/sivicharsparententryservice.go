package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type SIVICHARSParentEntryWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Edits    []sourceform.SIVIParentCellEdit  `json:"edits"`
	Actions  []SIVIParentActionInput          `json:"actions,omitempty"`
}

func (request *SIVICHARSParentEntryWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICHARSParentEntryWrite
	var decoded plain
	if err := vprocore.ValidateSourceParentEntryJSONProperties(data, "CHARS parent entry"); err != nil {
		return fmt.Errorf("CHARS parent entry transport: %w", err)
	}
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
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
				if err := vprocore.DecodeStrictRequiredJSON(raw, &action, "CHARS source action",
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
	contexts *vprocore.ContextService
	readers  sourceform.SiviParentSharedReferenceReaders
	enabled  bool
}

func NewSIVICHARSParentEntryService(contexts *vprocore.ContextService, readers sourceform.SiviParentSharedReferenceReaders,
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
	return ReadSIVICHARSParentReferences(s.contexts, ctx, contextID, plot, s.readers)
}

func (s *SIVICHARSParentEntryService) Save(ctx context.Context, contextID, plot string, request SIVICHARSParentEntryWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	actions := make([]sourceform.SiviParentActionEdit, len(request.Actions))
	for index, input := range request.Actions {
		actions[index] = sourceform.SiviParentActionEdit{ContextID: input.ContextID, ControlID: input.ControlID, Table: input.Table, RowID: input.RowID, Expected: input.Expected, Option: input.Option}
	}
	return WriteSIVICHARSParentOwnedProposal(s.contexts, ctx, contextID, plot, request.Original, request.Edits, actions, s.readers)
}

func (s *SIVICHARSParentEntryService) Restore(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return RestoreSIVICHARSParentEntry(s.contexts, ctx, contextID, plot, historyID, action)
}
