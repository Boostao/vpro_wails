package main

import (
	"context"
	_ "embed"
	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

//go:embed resources/geology-codes.db
var geologyCodeDatabase []byte

//go:embed resources/geology-codes-provenance.json
var geologyCodeProvenanceJSON []byte

type GeologyCodeChoice listcatalog.Choice

type GeologyCodeService struct {
	mu         sync.RWMutex
	closed     bool
	path       string
	provenance listcatalog.Provenance
	cache      *listCatalogueCache
}

func NewGeologyCodeService(dataDir string) (*GeologyCodeService, error) {
	path, provenance, cache, err := openListCatalogue(dataDir, "geology-codes.db", "geology code",
		geologyCodeDatabase, geologyCodeProvenanceJSON, listcatalog.GeologyProfile())
	if err != nil {
		return nil, err
	}
	return &GeologyCodeService{path: path, provenance: provenance, cache: cache}, nil
}

func (s *GeologyCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cache.rows = nil
	return nil
}

func (s *GeologyCodeService) ListBedrockChoices(ctx context.Context) ([]GeologyCodeChoice, error) {
	if err := acquireContextLock(ctx, s.mu.TryLock, s.mu.Lock, s.mu.Unlock); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("geology code catalogue is closed")
	}
	rows, err := s.cache.choicesContext(ctx, s.path, s.provenance, listcatalog.GeologyProfile(), "geology code", "BedrockType", false)
	if err != nil {
		return nil, err
	}

	result := make([]GeologyCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := GeologyCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Geology code is NULL"
		} else if err := validateSiteCodeText("Item", row.Code, 4); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *GeologyCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := acquireContextLock(ctx, s.mu.TryLock, s.mu.Lock, s.mu.Unlock); err != nil {
		return err
	}
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("geology code catalogue is closed")
	}
	_, err := s.cache.choicesContext(ctx, s.path, s.provenance, listcatalog.GeologyProfile(), "geology code", "", true)
	return err
}
