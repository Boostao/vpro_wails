package main

import (
	_ "embed"
	"errors"
	"fmt"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

//go:embed resources/parent-codes.db
var parentCodeDatabase []byte

//go:embed resources/parent-codes-provenance.json
var parentCodeProvenanceJSON []byte

type ParentCodeChoice listcatalog.Choice

type ParentCodeService struct {
	mu         sync.RWMutex
	closed     bool
	path       string
	provenance listcatalog.Provenance
	cache      *listCatalogueCache
}

func NewParentCodeService(dataDir string) (*ParentCodeService, error) {
	path, provenance, cache, err := openListCatalogue(dataDir, "parent-codes.db", "parent code",
		parentCodeDatabase, parentCodeProvenanceJSON, listcatalog.ParentProfile())
	if err != nil {
		return nil, err
	}
	return &ParentCodeService{path: path, provenance: provenance, cache: cache}, nil
}

func (s *ParentCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cache.rows = nil
	return nil
}

func (s *ParentCodeService) ListChoices(listName string) ([]ParentCodeChoice, error) {
	maximum, supported := parentCodeListMaximum(listName)
	if !supported {
		return nil, fmt.Errorf("unsupported parent code list %q", listName)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("parent code catalogue is closed")
	}
	rows, err := s.cache.choices(s.path, s.provenance, listcatalog.ParentProfile(), "parent code", listName, false)
	if err != nil {
		return nil, fmt.Errorf("parent code catalogue: %w", err)
	}
	result := make([]ParentCodeChoice, 0, len(rows))
	for _, source := range rows {
		result = append(result, parentCodeChoice(source, maximum))
	}
	return result, nil
}

func (s *ParentCodeService) ReloadCatalogue() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("parent code catalogue is closed")
	}
	_, err := s.cache.choices(s.path, s.provenance, listcatalog.ParentProfile(), "parent code", "", true)
	return err
}

func parentCodeChoice(source listcatalog.Choice, maximum int) ParentCodeChoice {
	row := ParentCodeChoice(source)
	row.Selectable, row.Diagnostic = false, ""
	if row.Code == nil {
		row.Diagnostic = "Parent code is NULL"
	} else if err := validateSiteCodeText("Item", row.Code, maximum); err != nil {
		row.Diagnostic = err.Error()
	} else {
		row.Selectable = true
	}
	return row
}
