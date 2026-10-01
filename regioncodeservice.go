package main

import (
	_ "embed"
	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

//go:embed resources/region-codes.db
var regionCodeDatabase []byte

//go:embed resources/region-codes-provenance.json
var regionCodeProvenanceJSON []byte

type RegionCodeChoice listcatalog.Choice

type RegionCodeService struct {
	mu         sync.RWMutex
	closed     bool
	path       string
	provenance listcatalog.Provenance
	cache      *listCatalogueCache
}

func NewRegionCodeService(dataDir string) (*RegionCodeService, error) {
	path, p, cache, err := openListCatalogue(dataDir, "region-codes.db", "region code",
		regionCodeDatabase, regionCodeProvenanceJSON, listcatalog.RegionProfile())
	if err != nil {
		return nil, err
	}
	return &RegionCodeService{path: path, provenance: p, cache: cache}, nil
}

func (s *RegionCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cache.rows = nil
	return nil
}

func (s *RegionCodeService) listChoices(list string, limit int) ([]RegionCodeChoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("region code catalogue is closed")
	}
	rows, err := s.cache.choices(s.path, s.provenance, listcatalog.RegionProfile(), "region code", list, false)
	if err != nil {
		return nil, err
	}
	result := make([]RegionCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := RegionCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Region code is NULL"
		} else if err := validateSiteCodeText("Item", row.Code, limit); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *RegionCodeService) ListRegionChoices() ([]RegionCodeChoice, error) {
	return s.listChoices("Region", 7)
}

func (s *RegionCodeService) ReloadCatalogue() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("region code catalogue is closed")
	}
	_, err := s.cache.choices(s.path, s.provenance, listcatalog.RegionProfile(), "region code", "", true)
	return err
}

func (s *RegionCodeService) ListEcosectionChoices() ([]RegionCodeChoice, error) {
	return s.listChoices("Ecosection", 3)
}
