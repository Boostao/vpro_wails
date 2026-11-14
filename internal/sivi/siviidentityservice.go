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

type SIVIIdentityWrite struct {
	Original []sourceform.SIVIVegetationProjection `json:"original"`
	Edits    []sourceform.SIVIHeightEdit           `json:"edits"`
}

func (request *SIVIIdentityWrite) UnmarshalJSON(data []byte) error {
	const operation = "SIVI identity"
	properties, err := sourceform.SourceChildJSONObject(data, operation, []string{"original", "edits"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, operation, "original", "edits"); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["original"], func(raw json.RawMessage) error {
		group, err := sourceform.SourceChildJSONObject(raw, operation, []string{"Form", "Query", "Columns", "Rows"}, nil)
		if err != nil {
			return err
		}
		if err := sourceform.SourceChildJSONNonNull(group, operation, "Form", "Query", "Columns", "Rows"); err != nil {
			return err
		}
		return sourceform.SourceChildRowsJSON(group["Rows"], operation)
	}); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["edits"], func(raw json.RawMessage) error {
		edit, err := sourceform.SourceChildJSONObject(raw, operation, []string{"rowId", "form", "column", "expected", "value"}, nil)
		if err != nil {
			return err
		}
		if err := sourceform.SourceChildJSONNonNull(edit, operation, "rowId", "form", "column", "expected", "value"); err != nil {
			return err
		}
		for _, name := range []string{"expected", "value"} {
			if err := sourceform.SourceChildCellJSON(edit[name], operation); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	type plain SIVIIdentityWrite
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "SIVI identity Save", "original", "edits"); err != nil {
		return fmt.Errorf("SIVI identity transport: %w", err)
	}
	*request = SIVIIdentityWrite(decoded)
	return nil
}

type SIVIIdentityService struct {
	contexts *vprocore.ContextService
}

func NewSIVIIdentityService(contexts *vprocore.ContextService) (*SIVIIdentityService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI identity service requires context ownership")
	}
	return &SIVIIdentityService{contexts: contexts}, nil
}

func (s *SIVIIdentityService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI identity service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI identity service requires context ownership")
	}
	return nil
}

func (s *SIVIIdentityService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]sourceform.SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func (s *SIVIIdentityService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*sourceform.SIVIHeightWriteResult, error) {
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
	return sourceform.WriteSIVIIdentities(s.contexts, ctx, contextID, plot, extended, request.Original, request.Edits)
}

func (s *SIVIIdentityService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return sourceform.RestoreSIVIIdentities(s.contexts, ctx, contextID, plot, historyID, action)
}
