package main

import (
	"context"
	"errors"
	"fmt"
)

type SIVICHARSParentEntryWrite struct {
	Original *SIVIParentProjection `json:"original"`
	Edits    []SIVIParentCellEdit  `json:"edits"`
}

func (request *SIVICHARSParentEntryWrite) UnmarshalJSON(data []byte) error {
	type plain SIVICHARSParentEntryWrite
	var decoded plain
	if err := validateSourceParentEntryJSONProperties(data, "CHARS parent entry"); err != nil {
		return fmt.Errorf("CHARS parent entry transport: %w", err)
	}
	if err := decodeStrictRequiredJSON(data, &decoded, "CHARS parent entry", "original", "edits"); err != nil {
		return fmt.Errorf("CHARS parent entry transport: %w", err)
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
	return s.contexts.writeSIVICHARSParentOwnedEntry(ctx, contextID, plot, request.Original, request.Edits, s.readers)
}

func (s *SIVICHARSParentEntryService) Restore(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVICHARSParentEntry(ctx, contextID, plot, historyID, action)
}
