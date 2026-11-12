package main

import (
	"context"

	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
	"github.com/boostao/vpro-wails/resources"
)

var siteCodeDatabase = resources.SiteCodesDB

var siteCodeProvenanceJSON = resources.SiteCodesProvenanceJSON

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
		siteCodeDatabase, siteCodeProvenanceJSON, listcatalog.SiteBundledProfile2026())
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

func (s *SiteCodeService) listChoices(ctx context.Context, list string) ([]SiteCodeChoice, error) {
	if err := acquireContextLock(ctx, s.mu.TryLock, s.mu.Lock, s.mu.Unlock); err != nil {
		return nil, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("site code catalogue is closed")
	}
	rows, err := s.cache.choicesContext(ctx, s.path, s.provenance, listcatalog.SiteBundledProfile2026(), "site code", list, false)
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

func (s *SiteCodeService) ListSiteDisturbanceChoices(ctx context.Context) ([]SiteCodeChoice, error) {
	return s.listChoices(ctx, "SiteDisturbance")
}

func (s *SiteCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := acquireContextLock(ctx, s.mu.TryLock, s.mu.Lock, s.mu.Unlock); err != nil {
		return err
	}
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("site code catalogue is closed")
	}
	_, err := s.cache.choicesContext(ctx, s.path, s.provenance, listcatalog.SiteBundledProfile2026(), "site code", "", true)
	return err
}

func (s *SiteCodeService) ListExposureChoices(ctx context.Context) ([]SiteCodeChoice, error) {
	return s.listChoices(ctx, "Exposure")
}

// Legacy constructors remain guarded without retaining an unowned DB handle.
