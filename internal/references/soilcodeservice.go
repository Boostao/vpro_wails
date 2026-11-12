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

var SoilCodeDatabase = resources.SoilCodesDB

var SoilCodeProvenanceJSON = resources.SoilCodesProvenanceJSON

type SoilCodeChoice listcatalog.Choice

type SoilCodeService struct {
	Mu         sync.RWMutex `json:"-" yaml:"-"`
	closed     bool
	Path       string                      `json:"-" yaml:"-"`
	Provenance listcatalog.Provenance      `json:"-" yaml:"-"`
	Cache      *sqlutil.ListCatalogueCache `json:"-" yaml:"-"`
}

func NewSoilCodeService(dataDir string) (*SoilCodeService, error) {
	path, provenance, cache, err := sqlutil.OpenListCatalogue(dataDir, "soil-codes.db", "soil code",
		SoilCodeDatabase, SoilCodeProvenanceJSON, listcatalog.SoilBundledProfile2026())
	if err != nil {
		return nil, err
	}
	return &SoilCodeService{Path: path, Provenance: provenance, Cache: cache}, nil
}

func (s *SoilCodeService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.Cache.Rows = nil
	return nil
}

func (s *SoilCodeService) listChoices(ctx context.Context, list string) ([]SoilCodeChoice, error) {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return nil, err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return nil, errors.New("soil code catalogue is closed")
	}
	rows, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.SoilBundledProfile2026(), "soil code", list, false)
	if err != nil {
		return nil, err
	}
	result := make([]SoilCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := SoilCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Soil code is NULL"
		} else if err := sqlutil.ValidateSiteCodeText("Item", row.Code, 4); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *SoilCodeService) ListGreatGroupChoices(ctx context.Context) ([]SoilCodeChoice, error) {
	return s.listChoices(ctx, "SoilClassGroup")
}

func (s *SoilCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return errors.New("soil code catalogue is closed")
	}
	_, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.SoilBundledProfile2026(), "soil code", "", true)
	return err
}

func (s *SoilCodeService) ListSubgroupChoices(ctx context.Context) ([]SoilCodeChoice, error) {
	return s.listChoices(ctx, "SoilClassSubgroup")
}
