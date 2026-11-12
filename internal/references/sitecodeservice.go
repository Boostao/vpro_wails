package references

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"

	"errors"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
	"github.com/boostao/vpro-wails/resources"
)

var siteCodeDatabase = resources.SiteCodesDB

var SiteCodeProvenanceJSON = resources.SiteCodesProvenanceJSON

type SiteCodeChoice listcatalog.Choice

type SiteCodeService struct {
	Mu         sync.RWMutex `json:"-" yaml:"-"`
	closed     bool
	Path       string `json:"-" yaml:"-"`
	provenance listcatalog.Provenance
	Cache      *sqlutil.ListCatalogueCache `json:"-" yaml:"-"`
}

func NewSiteCodeService(dataDir string) (*SiteCodeService, error) {
	target, provenance, cache, err := sqlutil.OpenListCatalogue(dataDir, "site-codes.db", "site code",
		siteCodeDatabase, SiteCodeProvenanceJSON, listcatalog.SiteBundledProfile2026())
	if err != nil {
		return nil, err
	}
	return &SiteCodeService{Path: target, provenance: provenance, Cache: cache}, nil
}

func checkSiteCodeChecksum(path string, provenance listcatalog.Provenance) error {
	return sqlutil.CheckListCatalogueChecksum(path, provenance, "site code")
}

func (s *SiteCodeService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.Cache.Rows = nil
	return nil
}

func (s *SiteCodeService) listChoices(ctx context.Context, list string) ([]SiteCodeChoice, error) {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return nil, err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return nil, errors.New("site code catalogue is closed")
	}
	rows, err := s.Cache.ChoicesContext(ctx, s.Path, s.provenance, listcatalog.SiteBundledProfile2026(), "site code", list, false)
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
			if err := sqlutil.ValidateSiteCodeText("Item", row.Code, limit); err != nil {
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
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return errors.New("site code catalogue is closed")
	}
	_, err := s.Cache.ChoicesContext(ctx, s.Path, s.provenance, listcatalog.SiteBundledProfile2026(), "site code", "", true)
	return err
}

func (s *SiteCodeService) ListExposureChoices(ctx context.Context) ([]SiteCodeChoice, error) {
	return s.listChoices(ctx, "Exposure")
}

// Legacy constructors remain guarded without retaining an unowned DB handle.
