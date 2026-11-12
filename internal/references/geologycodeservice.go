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

var GeologyCodeDatabase = resources.GeologyCodesDB

var geologyCodeProvenanceJSON = resources.GeologyCodesProvenanceJSON

type GeologyCodeChoice listcatalog.Choice

type GeologyCodeService struct {
	Mu         sync.RWMutex `json:"-" yaml:"-"`
	closed     bool
	Path       string                      `json:"-" yaml:"-"`
	Provenance listcatalog.Provenance      `json:"-" yaml:"-"`
	Cache      *sqlutil.ListCatalogueCache `json:"-" yaml:"-"`
}

func NewGeologyCodeService(dataDir string) (*GeologyCodeService, error) {
	path, provenance, cache, err := sqlutil.OpenListCatalogue(dataDir, "geology-codes.db", "geology code",
		GeologyCodeDatabase, geologyCodeProvenanceJSON, listcatalog.GeologyProfile())
	if err != nil {
		return nil, err
	}
	return &GeologyCodeService{Path: path, Provenance: provenance, Cache: cache}, nil
}

func (s *GeologyCodeService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.Cache.Rows = nil
	return nil
}

func (s *GeologyCodeService) ListBedrockChoices(ctx context.Context) ([]GeologyCodeChoice, error) {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return nil, err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return nil, errors.New("geology code catalogue is closed")
	}
	rows, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.GeologyProfile(), "geology code", "BedrockType", false)
	if err != nil {
		return nil, err
	}

	result := make([]GeologyCodeChoice, 0, len(rows))
	for _, source := range rows {
		row := GeologyCodeChoice(source)
		if row.Code == nil {
			row.Diagnostic = "Geology code is NULL"
		} else if err := sqlutil.ValidateSiteCodeText("Item", row.Code, 4); err != nil {
			row.Diagnostic = err.Error()
		} else {
			row.Selectable = true
		}
		result = append(result, row)
	}
	return result, nil
}

func (s *GeologyCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return errors.New("geology code catalogue is closed")
	}
	_, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.GeologyProfile(), "geology code", "", true)
	return err
}
