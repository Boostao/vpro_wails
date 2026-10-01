package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

//go:embed resources/site-codes.db
var siteCodeDatabase []byte

//go:embed resources/site-codes-provenance.json
var siteCodeProvenanceJSON []byte

type SiteCodeChoice listcatalog.Choice

type SiteCodeService struct {
	mu         sync.RWMutex
	db         *sql.DB
	path       string
	provenance listcatalog.Provenance
}

func NewSiteCodeService(dataDir string) (*SiteCodeService, error) {
	db, target, provenance, err := openListCatalogue(dataDir, "site-codes.db", "site code",
		siteCodeDatabase, siteCodeProvenanceJSON, listcatalog.SiteProfile())
	if err != nil {
		return nil, err
	}
	return &SiteCodeService{db: db, path: target, provenance: provenance}, nil
}

func checkSiteCodeChecksum(path string, provenance listcatalog.Provenance) error {
	return checkListCatalogueChecksum(path, provenance, "site code")
}

func (s *SiteCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *SiteCodeService) listChoices(list string) ([]SiteCodeChoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("site code catalogue is closed")
	}
	if err := checkSiteCodeChecksum(s.path, s.provenance); err != nil {
		return nil, err
	}
	if err := listcatalog.ValidateDatabase(s.db, s.provenance); err != nil {
		return nil, err
	}
	rows, err := listcatalog.ReadChoices(s.db, list)
	if err != nil {
		return nil, err
	}
	result := make([]SiteCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := SiteCodeChoice(source)
		row.Selectable = false
		row.Diagnostic = ""
		switch {
		case row.ListName == nil || *row.ListName != list:
			row.Diagnostic = "Site code row belongs to a different list"
		case row.Code == nil:
			row.Diagnostic = "Site code is NULL"
		default:
			limit := 8
			if list == "Exposure" {
				limit = 2
			}
			if err := validateSiteCodeText("Item", row.Code, limit); err != nil {
				row.Diagnostic = err.Error()
			} else {
				row.Selectable = true
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *SiteCodeService) ListSiteDisturbanceChoices() ([]SiteCodeChoice, error) {
	return s.listChoices("SiteDisturbance")
}

func (s *SiteCodeService) ListExposureChoices() ([]SiteCodeChoice, error) {
	return s.listChoices("Exposure")
}

func (s *PlotService) exposureChoices() ([]SiteCodeChoice, error) {
	if s.siteCodesError != nil {
		return nil, s.siteCodesError
	}
	if s.siteCodes != nil {
		return s.siteCodes.ListExposureChoices()
	}
	// Legacy constructors remain guarded without retaining an unowned DB handle.
	catalogue, err := NewSiteCodeService(s.projects.root)
	if err != nil {
		return nil, err
	}
	rows, readErr := catalogue.ListExposureChoices()
	return rows, errors.Join(readErr, catalogue.Close())
}
