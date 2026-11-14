package main

import (
	"context"
	"errors"

	datasheetpkg "github.com/boostao/vpro-wails/internal/datasheet"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type DatasheetColumnMeta = datasheetpkg.DatasheetColumnMeta
type DatasheetTableInfo = datasheetpkg.DatasheetTableInfo
type DatasheetSortSpec = datasheetpkg.DatasheetSortSpec
type DatasheetFilterSpec = datasheetpkg.DatasheetFilterSpec
type DatasheetPageRequest = datasheetpkg.DatasheetPageRequest
type DatasheetRow = datasheetpkg.DatasheetRow
type DatasheetPage = datasheetpkg.DatasheetPage
type DatasheetCellEdit = datasheetpkg.DatasheetCellEdit
type DatasheetEditRequest = datasheetpkg.DatasheetEditRequest
type DatasheetEditResult = datasheetpkg.DatasheetEditResult
type DatasheetFindReplaceRequest = datasheetpkg.DatasheetFindReplaceRequest
type DatasheetFindReplaceItem = datasheetpkg.DatasheetFindReplaceItem
type DatasheetFindReplacePreview = datasheetpkg.DatasheetFindReplacePreview

type DatasheetService struct {
	contexts         *vprocorepkg.ContextService
	environmentTable *datasheetpkg.TwoPageEnvironmentTableService
	vegetationTable  *datasheetpkg.TwoPageVegetationTableService
}

func NewDatasheetService(environmentTable *datasheetpkg.TwoPageEnvironmentTableService, vegetationTable *datasheetpkg.TwoPageVegetationTableService) *DatasheetService {
	var contexts *vprocorepkg.ContextService
	if environmentTable != nil {
		contexts = environmentTable.Contexts
	}
	return &DatasheetService{
		contexts:         contexts,
		environmentTable: environmentTable,
		vegetationTable:  vegetationTable,
	}
}

func buildDatasheetService(contexts *ContextService) (*DatasheetService, error) {
	core := unwrapContextService(contexts)
	envTable, err := datasheetpkg.NewTwoPageEnvironmentTableService(core)
	if err != nil {
		return nil, err
	}
	vegTable, err := datasheetpkg.NewTwoPageVegetationTableService(core)
	if err != nil {
		return nil, err
	}
	return NewDatasheetService(envTable, vegTable), nil
}

func (s *DatasheetService) setContext(contexts *ContextService) {
	if s == nil {
		return
	}
	s.contexts = unwrapContextService(contexts)
}

func (s *DatasheetService) datasheetContext() (*vprocorepkg.ContextService, error) {
	if s == nil {
		return nil, errors.New("datasheet service is unavailable")
	}
	if s.contexts != nil {
		return s.contexts, nil
	}
	if s.environmentTable != nil && s.environmentTable.Contexts != nil {
		return s.environmentTable.Contexts, nil
	}
	return nil, errors.New("datasheet context service is unavailable")
}

func (s *DatasheetService) GetEnvironmentTableReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageEnvironmentTableReview, error) {
	if s == nil || s.environmentTable == nil {
		return nil, errors.New("environment table service is unavailable")
	}
	return s.environmentTable.GetReview(ctx, contextID, plot, offset, limit)
}

func (s *DatasheetService) GetVegetationTableReview(ctx context.Context, contextID, plot, form string, offset, limit int) (*TwoPageVegetationTableReview, error) {
	if s == nil || s.vegetationTable == nil {
		return nil, errors.New("vegetation table service is unavailable")
	}
	return s.vegetationTable.GetReview(ctx, contextID, plot, form, offset, limit)
}

func (s *DatasheetService) ListDatasheetTables(ctx context.Context, contextID string) ([]DatasheetTableInfo, error) {
	contexts, err := s.datasheetContext()
	if err != nil {
		return nil, err
	}
	return datasheetpkg.ListDatasheetTables(ctx, contexts, contextID)
}

func (s *DatasheetService) GetDatasheetPage(ctx context.Context, contextID string, request DatasheetPageRequest) (*DatasheetPage, error) {
	contexts, err := s.datasheetContext()
	if err != nil {
		return nil, err
	}
	return datasheetpkg.GetDatasheetPage(ctx, contexts, contextID, request)
}

func (s *DatasheetService) PreviewDatasheetFindReplace(ctx context.Context, contextID string, request DatasheetFindReplaceRequest) (*DatasheetFindReplacePreview, error) {
	contexts, err := s.datasheetContext()
	if err != nil {
		return nil, err
	}
	return datasheetpkg.PreviewDatasheetFindReplace(ctx, contexts, contextID, request)
}

func (s *DatasheetService) ApplyDatasheetEdits(ctx context.Context, contextID string, request DatasheetEditRequest) (*DatasheetEditResult, error) {
	contexts, err := s.datasheetContext()
	if err != nil {
		return nil, err
	}
	return datasheetpkg.ApplyDatasheetEdits(ctx, contexts, contextID, request)
}
