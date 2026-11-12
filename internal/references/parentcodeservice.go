package references

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"

	"errors"
	"fmt"
	"sync"

	"github.com/boostao/vpro-wails/internal/listcatalog"
	"github.com/boostao/vpro-wails/resources"
)

var ParentCodeDatabase = resources.ParentCodesDB

var parentCodeProvenanceJSON = resources.ParentCodesProvenanceJSON

type ParentCodeChoice listcatalog.Choice

type ParentCodeService struct {
	Mu         sync.RWMutex `json:"-" yaml:"-"`
	closed     bool
	Path       string                      `json:"-" yaml:"-"`
	Provenance listcatalog.Provenance      `json:"-" yaml:"-"`
	Cache      *sqlutil.ListCatalogueCache `json:"-" yaml:"-"`
}

func NewParentCodeService(dataDir string) (*ParentCodeService, error) {
	path, provenance, cache, err := sqlutil.OpenListCatalogue(dataDir, "parent-codes.db", "parent code",
		ParentCodeDatabase, parentCodeProvenanceJSON, listcatalog.ParentBundledProfile2026())
	if err != nil {
		return nil, err
	}
	return &ParentCodeService{Path: path, Provenance: provenance, Cache: cache}, nil
}

func (s *ParentCodeService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.Cache.Rows = nil
	return nil
}

func (s *ParentCodeService) ListChoices(ctx context.Context, listName string) ([]ParentCodeChoice, error) {
	maximum, supported := sqlutil.ParentCodeListMaximum(listName)
	if !supported {
		return nil, fmt.Errorf("unsupported parent code list %q", listName)
	}
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return nil, err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return nil, errors.New("parent code catalogue is closed")
	}
	rows, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.ParentBundledProfile2026(), "parent code", listName, false)
	if err != nil {
		return nil, fmt.Errorf("parent code catalogue: %w", err)
	}
	result := make([]ParentCodeChoice, 0, len(rows))
	for _, source := range rows {
		result = append(result, ParentCodeChoiceInternal(source, maximum))
	}
	return result, nil
}

func (s *ParentCodeService) ReloadCatalogue(ctx context.Context) error {
	if err := sqlutil.AcquireContextLock(ctx, s.Mu.TryLock, s.Mu.Lock, s.Mu.Unlock); err != nil {
		return err
	}
	defer s.Mu.Unlock()
	if s.closed {
		return errors.New("parent code catalogue is closed")
	}
	_, err := s.Cache.ChoicesContext(ctx, s.Path, s.Provenance, listcatalog.ParentBundledProfile2026(), "parent code", "", true)
	return err
}

func ParentCodeChoiceInternal(source listcatalog.Choice, maximum int) ParentCodeChoice {
	row := ParentCodeChoice(source)
	row.Selectable, row.Diagnostic = false, ""
	if row.Code == nil {
		row.Diagnostic = "Parent code is NULL"
	} else if err := sqlutil.ValidateSiteCodeText("Item", row.Code, maximum); err != nil {
		row.Diagnostic = err.Error()
	} else {
		row.Selectable = true
	}
	return row
}
