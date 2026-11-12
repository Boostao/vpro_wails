package main

import (
	"context"
	"errors"
)

type ImportExportService struct {
	contexts          *ContextService
	tableCSVArchive   *TableCSVArchiveService
	plotLabelRegional *PlotLabelRegionalService
	plotLabelPDF      *PlotLabelPDFExportService
}

func NewImportExportService(
	contexts *ContextService,
	tableCSVArchive *TableCSVArchiveService,
	plotLabelRegional *PlotLabelRegionalService,
	plotLabelPDF *PlotLabelPDFExportService,
) *ImportExportService {
	return &ImportExportService{
		contexts:          contexts,
		tableCSVArchive:   tableCSVArchive,
		plotLabelRegional: plotLabelRegional,
		plotLabelPDF:      plotLabelPDF,
	}
}

func (s *ImportExportService) ChooseAccessDatabaseFile(ctx context.Context, contextID string) (string, error) {
	if s == nil || s.contexts == nil {
		return "", errors.New("import/export context is unavailable")
	}
	return s.contexts.ChooseAccessDatabaseFile(ctx, contextID)
}

func (s *ImportExportService) InspectAccessDatabase(ctx context.Context, contextID, sourcePath string) (AccessDatabaseInspection, error) {
	if s == nil || s.contexts == nil {
		return AccessDatabaseInspection{}, errors.New("import/export context is unavailable")
	}
	return s.contexts.InspectAccessDatabase(ctx, contextID, sourcePath)
}

func (s *ImportExportService) ImportAccessProject(ctx context.Context, contextID string, request AccessProjectImportRequest) (AccessProjectImportResult, error) {
	if s == nil || s.contexts == nil {
		return AccessProjectImportResult{}, errors.New("import/export context is unavailable")
	}
	return s.contexts.ImportAccessProject(ctx, contextID, request)
}

func (s *ImportExportService) GetProjectTableCSVReview(ctx context.Context, contextID, table string) (*ProjectTableCSVReview, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("import/export context is unavailable")
	}
	return s.contexts.GetProjectTableCSVReview(ctx, contextID, table)
}

func (s *ImportExportService) GetTableCSVArchiveReview(ctx context.Context, contextID, requestJSON string) (*TableCSVArchiveReview, error) {
	if s == nil || s.tableCSVArchive == nil {
		return nil, errors.New("table CSV archive service is unavailable")
	}
	return s.tableCSVArchive.GetTableCSVArchiveReview(ctx, contextID, requestJSON)
}

func (s *ImportExportService) ExportReviewedTableCSVArchive(ctx context.Context, contextID, requestJSON string) (*TableCSVArchiveOutcome, error) {
	if s == nil || s.tableCSVArchive == nil {
		return nil, errors.New("table CSV archive service is unavailable")
	}
	return s.tableCSVArchive.ExportReviewedTableCSVArchive(ctx, contextID, requestJSON)
}

func (s *ImportExportService) GetPlotLabelRegionalPreview(ctx context.Context, contextID, plot string) (*PlotLabelRegionalPreview, error) {
	if s == nil || s.plotLabelRegional == nil {
		return nil, errors.New("plot label regional service is unavailable")
	}
	return s.plotLabelRegional.GetPreview(ctx, contextID, plot)
}

func (s *ImportExportService) GetPlotLabelPDFReview(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFExportReview, error) {
	if s == nil || s.plotLabelPDF == nil {
		return nil, errors.New("plot label PDF export service is unavailable")
	}
	return s.plotLabelPDF.GetReview(ctx, contextID, requestJSON)
}

func (s *ImportExportService) ExportReviewedPlotLabelPDF(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFExportOutcome, error) {
	if s == nil || s.plotLabelPDF == nil {
		return nil, errors.New("plot label PDF export service is unavailable")
	}
	return s.plotLabelPDF.ExportReviewed(ctx, contextID, requestJSON)
}

func (s *ImportExportService) RecoverPlotLabelPDFOutcome(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFAttemptOutcome, error) {
	if s == nil || s.plotLabelPDF == nil {
		return nil, errors.New("plot label PDF export service is unavailable")
	}
	return s.plotLabelPDF.RecoverOutcome(ctx, contextID, requestJSON)
}
