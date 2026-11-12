package main

import (
	"context"
	"errors"
)

type DatasheetService struct {
	environmentTable *TwoPageEnvironmentTableService
	vegetationTable  *TwoPageVegetationTableService
}

func NewDatasheetService(environmentTable *TwoPageEnvironmentTableService, vegetationTable *TwoPageVegetationTableService) *DatasheetService {
	return &DatasheetService{
		environmentTable: environmentTable,
		vegetationTable:  vegetationTable,
	}
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
