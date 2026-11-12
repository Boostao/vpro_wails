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

var RegionCodeDatabase = resources.RegionCodesDB

var RegionCodeProvenanceJSON = resources.RegionCodesProvenanceJSON

type RegionCodeChoice listcatalog.Choice

type RegionCodeService struct {
	Mu         sync.RWMutex `json:"-" yaml:"-"`
	closed     bool
	Path       string                      `json:"-" yaml:"-"`
	Provenance listcatalog.Provenance      `json:"-" yaml:"-"`
	Cache      *sqlutil.ListCatalogueCache `json:"-" yaml:"-"`
}

func NewRegionCodeService(dataDir string) (*RegionCodeService, error) {
	path, p, cache, err := sqlutil.OpenListCatalogue(dataDir, "region-codes.db", "region code",
		RegionCodeDatabase, RegionCodeProvenanceJSON, listcatalog.RegionProfile())
	if err != nil {
		return nil, err
	}
	return &RegionCodeService{Path: path, Provenance: p, Cache: cache}, nil
}

func (s *RegionCodeService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.Cache.Rows = nil
	return nil
}

func (s *RegionCodeService) listChoices(ctx context.Context, list string, limit int) ([]RegionCodeChoice, error) {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return nil, err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return nil, errors.New("region code catalogue is closed")
	}
	rows, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.RegionProfile(), "region code", list, false)
	if err != nil {
		return nil, err
	}
	result := make([]RegionCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := RegionCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Region code is NULL"
		} else if err := sqlutil.ValidateSiteCodeText("Item", row.Code, limit); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *RegionCodeService) ListRegionChoices(ctx context.Context) ([]RegionCodeChoice, error) {
	return s.listChoices(ctx, "Region", 7)
}

func (s *RegionCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return errors.New("region code catalogue is closed")
	}
	_, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.RegionProfile(), "region code", "", true)
	return err
}

func (s *RegionCodeService) ListEcosectionChoices(ctx context.Context) ([]RegionCodeChoice, error) {
	return s.listChoices(ctx, "Ecosection", 3)
}
