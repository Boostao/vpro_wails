package main

import (
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
	closed     bool
	path       string
	provenance listcatalog.Provenance
	cache      *listCatalogueCache
}

func NewSiteCodeService(dataDir string) (*SiteCodeService, error) {
	target, provenance, cache, err := openListCatalogue(dataDir, "site-codes.db", "site code",
		siteCodeDatabase, siteCodeProvenanceJSON, listcatalog.SiteProfile())
	if err != nil {
		return nil, err
	}
	return &SiteCodeService{path: target, provenance: provenance, cache: cache}, nil
}

func checkSiteCodeChecksum(path string, provenance listcatalog.Provenance) error {
	return checkListCatalogueChecksum(path, provenance, "site code")
}

func (s *SiteCodeService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.cache.rows = nil
	return nil
}

func (s *SiteCodeService) listChoices(list string) ([]SiteCodeChoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("site code catalogue is closed")
	}
	rows, err := s.cache.choices(s.path, s.provenance, listcatalog.SiteProfile(), "site code", list, false)
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

func (s *SiteCodeService) ReloadCatalogue() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("site code catalogue is closed")
	}
	_, err := s.cache.choices(s.path, s.provenance, listcatalog.SiteProfile(), "site code", "", true)
	return err
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
