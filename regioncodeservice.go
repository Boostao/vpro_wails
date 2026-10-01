package main

import (
	"database/sql"
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
	db         *sql.DB
	path       string
	provenance listcatalog.Provenance
}

func NewRegionCodeService(dataDir string) (*RegionCodeService, error) {
	db, path, p, err := openListCatalogue(dataDir, "region-codes.db", "region code",
		regionCodeDatabase, regionCodeProvenanceJSON, listcatalog.RegionProfile())
	if err != nil {
		return nil, err
	}
	return &RegionCodeService{db: db, path: path, provenance: p}, nil
}

func (s *RegionCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *RegionCodeService) listChoices(list string, limit int) ([]RegionCodeChoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("region code catalogue is closed")
	}
	if err := checkListCatalogueChecksum(s.path, s.provenance, "region code"); err != nil {
		return nil, err
	}
	if err := listcatalog.ValidateDatabaseFor(s.db, s.provenance, listcatalog.RegionProfile()); err != nil {
		return nil, err
	}
	rows, err := listcatalog.ReadChoicesFor(s.db, list, listcatalog.RegionProfile())
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

func (s *RegionCodeService) ListEcosectionChoices() ([]RegionCodeChoice, error) {
	return s.listChoices("Ecosection", 3)
}
