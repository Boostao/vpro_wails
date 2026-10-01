package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
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
	db         *sql.DB
	path       string
	provenance listcatalog.Provenance
}

func NewSoilCodeService(dataDir string) (*SoilCodeService, error) {
	db, path, provenance, err := openListCatalogue(dataDir, "soil-codes.db", "soil code",
		soilCodeDatabase, soilCodeProvenanceJSON, listcatalog.SoilProfile())
	if err != nil {
		return nil, err
	}
	return &SoilCodeService{db: db, path: path, provenance: provenance}, nil
}

func (s *SoilCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *SoilCodeService) listChoices(list string) ([]SoilCodeChoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("soil code catalogue is closed")
	}
	if err := checkListCatalogueChecksum(s.path, s.provenance, "soil code"); err != nil {
		return nil, err
	}
	if err := listcatalog.ValidateDatabaseFor(s.db, s.provenance, listcatalog.SoilProfile()); err != nil {
		return nil, fmt.Errorf("soil code catalogue: %w", err)
	}
	rows, err := listcatalog.ReadChoicesFor(s.db, list, listcatalog.SoilProfile())
	if err != nil {
		return nil, fmt.Errorf("soil code catalogue: %w", err)
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

func (s *SoilCodeService) ListSubgroupChoices() ([]SoilCodeChoice, error) {
	return s.listChoices("SoilClassSubgroup")
}
