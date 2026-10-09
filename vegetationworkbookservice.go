package main

import (
	"context"
	"errors"
)

type VegetationWorkbookOptions struct {
	QuickReport        bool `json:"quickReport"`
	SpaceBetweenGroups bool `json:"spaceBetweenGroups"`
	ReportSummary      bool `json:"reportSummary"`
}

type VegetationWorkbookSummary struct {
	CreatedDate        string               `json:"createdDate"`
	EnvironmentRows    int                  `json:"environmentRows"`
	SelectedPlotRows   int                  `json:"selectedPlotRows"`
	SpeciesVersion     ProjectMetadataCell  `json:"speciesVersion"`
	VersionStatus      string               `json:"versionStatus"`
	VersionDefinitions ProjectMetadataTable `json:"versionDefinitions"`
}

type VegetationWorkbookSheet struct {
	Unit ProjectMetadataCell `json:"unit"`
	Name string              `json:"name"`
}

type VegetationWorkbookSkippedUnit struct {
	Unit   ProjectMetadataCell `json:"unit"`
	Reason string              `json:"reason"`
}

type VegetationWorkbookReview struct {
	Preview        LongVegetationPreview           `json:"preview"`
	Options        VegetationWorkbookOptions       `json:"options"`
	Summary        *VegetationWorkbookSummary      `json:"summary"`
	Scope          string                          `json:"scope"`
	CreatedDate    string                          `json:"createdDate"`
	Sheets         []VegetationWorkbookSheet       `json:"sheets"`
	SkippedUnits   []VegetationWorkbookSkippedUnit `json:"skippedUnits"`
	ApprovalHash   string                          `json:"approvalHash"`
	WorkbookSHA256 string                          `json:"workbookSHA256"`
	Bytes          int                             `json:"bytes"`
}

type VegetationWorkbookOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

type VegetationWorkbookService struct {
	private *vegetationWorkbookService
}

func NewVegetationWorkbookService(contexts *ContextService, lookup func(string) (string, bool)) (*VegetationWorkbookService, error) {
	service, err := newVegetationWorkbookService(contexts, lookup)
	if err != nil {
		return nil, err
	}
	return &VegetationWorkbookService{private: service}, nil
}

func (s *VegetationWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookReview, error) {
	if s == nil || s.private == nil {
		return nil, errors.New("Long Vegetation workbook service is unavailable")
	}
	review, err := s.private.GetReview(ctx, contextID, requestJSON)
	if err != nil {
		return nil, err
	}
	result := &VegetationWorkbookReview{
		Preview: review.Preview, Scope: review.Scope, CreatedDate: review.CreatedDate,
		Options:      VegetationWorkbookOptions{review.Layout.QuickReport, review.Layout.SpaceBetweenGroups, review.Layout.ReportSummary},
		Sheets:       make([]VegetationWorkbookSheet, 0, len(review.Sheets)),
		SkippedUnits: make([]VegetationWorkbookSkippedUnit, 0, len(review.SkippedUnits)),
		ApprovalHash: review.ApprovalHash, WorkbookSHA256: review.WorkbookSHA256, Bytes: review.Bytes,
	}
	if summary := review.Layout.Summary; summary != nil {
		result.Summary = &VegetationWorkbookSummary{
			summary.CreatedDate, summary.EnvironmentRows, summary.SelectedPlotRows,
			cloneSiteUnitCell(summary.SpeciesVersion), summary.VersionStatus, summary.VersionDefinitions,
		}
	}
	for _, sheet := range review.Sheets {
		result.Sheets = append(result.Sheets, VegetationWorkbookSheet{cloneSiteUnitCell(sheet.Unit), sheet.Name})
	}
	for _, skipped := range review.SkippedUnits {
		result.SkippedUnits = append(result.SkippedUnits, VegetationWorkbookSkippedUnit{cloneSiteUnitCell(skipped.Unit), skipped.Reason})
	}
	return result, nil
}

func (s *VegetationWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	if s == nil || s.private == nil {
		return nil, errors.New("Long Vegetation workbook service is unavailable")
	}
	outcome, err := s.private.ExportReviewed(ctx, contextID, requestJSON)
	if err != nil {
		return nil, err
	}
	return &VegetationWorkbookOutcome{
		outcome.Status, outcome.RequestedDestination, outcome.Path, outcome.SHA256, outcome.ErrorMessage,
	}, nil
}
