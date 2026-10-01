package main

import (
	"database/sql"
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
	db         *sql.DB
	path       string
	provenance listcatalog.Provenance
}

func NewParentCodeService(dataDir string) (*ParentCodeService, error) {
	db, path, provenance, err := openListCatalogue(dataDir, "parent-codes.db", "parent code",
		parentCodeDatabase, parentCodeProvenanceJSON, listcatalog.ParentProfile())
	if err != nil {
		return nil, err
	}
	return &ParentCodeService{db: db, path: path, provenance: provenance}, nil
}

func (s *ParentCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *ParentCodeService) ListChoices(listName string) ([]ParentCodeChoice, error) {
	maximum, supported := parentCodeListMaximum(listName)
	if !supported {
		return nil, fmt.Errorf("unsupported parent code list %q", listName)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("parent code catalogue is closed")
	}
	if err := checkListCatalogueChecksum(s.path, s.provenance, "parent code"); err != nil {
		return nil, err
	}
	if err := listcatalog.ValidateDatabaseFor(s.db, s.provenance, listcatalog.ParentProfile()); err != nil {
		return nil, fmt.Errorf("parent code catalogue: %w", err)
	}
	rows, err := listcatalog.ReadChoicesFor(s.db, listName, listcatalog.ParentProfile())
	if err != nil {
		return nil, fmt.Errorf("parent code catalogue: %w", err)
	}
	result := make([]ParentCodeChoice, 0, len(rows))
	for _, source := range rows {
		result = append(result, parentCodeChoice(source, maximum))
	}
	return result, nil
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
