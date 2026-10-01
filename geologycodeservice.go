package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
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
	db         *sql.DB
	path       string
	provenance listcatalog.Provenance
}

func NewGeologyCodeService(dataDir string) (*GeologyCodeService, error) {
	db, path, provenance, err := openListCatalogue(dataDir, "geology-codes.db", "geology code",
		geologyCodeDatabase, geologyCodeProvenanceJSON, listcatalog.GeologyProfile())
	if err != nil {
		return nil, err
	}
	return &GeologyCodeService{db: db, path: path, provenance: provenance}, nil
}

func (s *GeologyCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *GeologyCodeService) ListBedrockChoices() ([]GeologyCodeChoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("geology code catalogue is closed")
	}
	if err := checkListCatalogueChecksum(s.path, s.provenance, "geology code"); err != nil {
		return nil, err
	}
	if err := listcatalog.ValidateDatabaseFor(s.db, s.provenance, listcatalog.GeologyProfile()); err != nil {
		return nil, fmt.Errorf("geology code catalogue: %w", err)
	}
	rows, err := listcatalog.ReadChoicesFor(s.db, "BedrockType", listcatalog.GeologyProfile())
	if err != nil {
		return nil, fmt.Errorf("geology code catalogue: %w", err)
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
