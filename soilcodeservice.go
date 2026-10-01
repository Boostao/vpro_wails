package main

import (
	_ "embed"
	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

//go:embed resources/soil-codes.db
var soilCodeDatabase []byte

//go:embed resources/soil-codes-provenance.json
var soilCodeProvenanceJSON []byte

type SoilCodeChoice listcatalog.Choice

type SoilCodeService struct {
	mu         sync.RWMutex
	closed     bool
	path       string
	provenance listcatalog.Provenance
	cache      *listCatalogueCache
}

func NewSoilCodeService(dataDir string) (*SoilCodeService, error) {
	path, provenance, cache, err := openListCatalogue(dataDir, "soil-codes.db", "soil code",
		soilCodeDatabase, soilCodeProvenanceJSON, listcatalog.SoilProfile())
	if err != nil {
		return nil, err
	}
	return &SoilCodeService{path: path, provenance: provenance, cache: cache}, nil
}

func (s *SoilCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cache.rows = nil
	return nil
}

func (s *SoilCodeService) listChoices(list string) ([]SoilCodeChoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("soil code catalogue is closed")
	}
	rows, err := s.cache.choices(s.path, s.provenance, listcatalog.SoilProfile(), "soil code", list, false)
	if err != nil {
		return nil, err
	}
	result := make([]SoilCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := SoilCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Soil code is NULL"
		} else if err := validateSiteCodeText("Item", row.Code, 4); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *SoilCodeService) ListGreatGroupChoices() ([]SoilCodeChoice, error) {
	return s.listChoices("SoilClassGroup")
}

func (s *SoilCodeService) ReloadCatalogue() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("soil code catalogue is closed")
	}
	_, err := s.cache.choices(s.path, s.provenance, listcatalog.SoilProfile(), "soil code", "", true)
	return err
}

func (s *SoilCodeService) ListSubgroupChoices() ([]SoilCodeChoice, error) {
	return s.listChoices("SoilClassSubgroup")
}
