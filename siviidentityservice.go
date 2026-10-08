package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const siviIdentityFeatureEnvironment = "VPRO_SIVI_IDENTITY_EDITING"

type SIVIIdentityWrite struct {
	Original []SIVIVegetationProjection `json:"original"`
	Edits    []SIVIHeightEdit           `json:"edits"`
}

func (request *SIVIIdentityWrite) UnmarshalJSON(data []byte) error {
	const operation = "SIVI identity"
	properties, err := sourceChildJSONObject(data, operation, []string{"original", "edits"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, "original", "edits"); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["original"], func(raw json.RawMessage) error {
		group, err := sourceChildJSONObject(raw, operation, []string{"Form", "Query", "Columns", "Rows"}, nil)
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(group, operation, "Form", "Query", "Columns", "Rows"); err != nil {
			return err
		}
		return sourceChildRowsJSON(group["Rows"], operation)
	}); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["edits"], func(raw json.RawMessage) error {
		edit, err := sourceChildJSONObject(raw, operation, []string{"rowId", "form", "column", "expected", "value"}, nil)
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(edit, operation, "rowId", "form", "column", "expected", "value"); err != nil {
			return err
		}
		for _, name := range []string{"expected", "value"} {
			if err := sourceChildCellJSON(edit[name], operation); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	type plain SIVIIdentityWrite
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "SIVI identity Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI identity transport: %w", err)
	}
	*request = SIVIIdentityWrite(decoded)
	return nil
}

type SIVIIdentityService struct {
	contexts *ContextService
	enabled  bool
}

func NewSIVIIdentityService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVIIdentityService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI identity service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(siviIdentityFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVIIdentityService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVIIdentityService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI identity service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return errors.New("SIVI identity editing is disabled in this session")
	}
	return nil
}

func (s *SIVIIdentityService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIVegetation(ctx, contextID, plot, extended)
}

func (s *SIVIIdentityService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVIIdentityWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 {
		return nil, errors.New("SIVI identity Save requires three reviewed source groups")
	}
	return s.contexts.writeSIVIIdentities(ctx, contextID, plot, extended, request.Original, request.Edits)
}

func (s *SIVIIdentityService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVIIdentities(ctx, contextID, plot, historyID, action)
}
