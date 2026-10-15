package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

const plotLabelPDFExportEnvironment = "VPRO_PLOT_LABEL_PDF_EXPORT"

type PlotLabelPDFExportService struct {
	contexts  *ContextService
	enabled   bool
	sessionID string
	runtime   plotLabelPDFRuntime
	mu        sync.Mutex
	attempts  map[string]*plotLabelPDFAttempt
}

func NewPlotLabelPDFExportService(contexts *ContextService, enabled bool) *PlotLabelPDFExportService {
	return &PlotLabelPDFExportService{contexts: contexts, enabled: enabled, sessionID: rand.Text()}
}

type PlotLabelPDFReviewRequest struct {
	PlotNumber string `json:"plotNumber"`
}

func (r *PlotLabelPDFReviewRequest) UnmarshalJSON(data []byte) error {
	type plain PlotLabelPDFReviewRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "label PDF review", "plotNumber"); err != nil {
		return err
	}
	*r = PlotLabelPDFReviewRequest(decoded)
	return nil
}

type PlotLabelPDFExportRequest struct {
	PlotNumber   string `json:"plotNumber"`
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
	RequestID    string `json:"requestId"`
	SessionID    string `json:"sessionId"`
}

func (r *PlotLabelPDFExportRequest) UnmarshalJSON(data []byte) error {
	type plain PlotLabelPDFExportRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "label PDF export", "plotNumber", "approvalHash", "destination", "requestId", "sessionId"); err != nil {
		return err
	}
	if !siviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("label PDF export requires a stable lowercase UUIDv4")
	}
	hash, err := hex.DecodeString(decoded.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != decoded.ApprovalHash {
		return errors.New("label PDF export requires an exact lowercase hexadecimal source approval")
	}
	if decoded.Destination == "" {
		return errors.New("label PDF export requires an explicit destination")
	}
	if decoded.SessionID == "" || len(decoded.SessionID) > 128 {
		return errors.New("label PDF export requires its reviewed service owner")
	}
	*r = PlotLabelPDFExportRequest(decoded)
	return nil
}

type PlotLabelPDFExportReview struct {
	Preview      PlotLabelRegionalPreview `json:"preview"`
	Project      string                   `json:"project"`
	ProjectPath  string                   `json:"projectPath"`
	SU           string                   `json:"su"`
	SUPath       string                   `json:"suPath"`
	ApprovalHash string                   `json:"approvalHash"`
	PDFSHA256    string                   `json:"pdfSHA256"`
	Layout       string                   `json:"layout"`
	SessionID    string                   `json:"sessionId"`
}

type PlotLabelPDFExportOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

func (s *PlotLabelPDFExportService) authorize(ctx context.Context, write bool) error {
	if ctx == nil {
		return errors.New("label PDF service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("label PDF service requires its owned SQLite context")
	}
	if s.sessionID == "" {
		return errors.New("label PDF service owner is unavailable")
	}
	if write && !s.enabled {
		return errors.New("label PDF export is disabled in this session")
	}
	return nil
}

func plotLabelPDFApprovalHash(source ownedPlotLabelPDFPreparation) (string, error) {
	source, err := snapshotPlotLabelPDFPreparation(source)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(source)
	if err != nil {
		return "", fmt.Errorf("label PDF source approval: %w", err)
	}
	hash := sha256.Sum256(append([]byte("VPRO label PDF source approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *PlotLabelPDFExportService) GetReview(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFExportReview, error) {
	if err := s.authorize(ctx, true); err != nil {
		return nil, err
	}
	var request PlotLabelPDFReviewRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	source, err := s.contexts.readPlotLabelPDFPreparation(ctx, contextID, request.PlotNumber, s.runtime)
	if err != nil {
		return nil, err
	}
	approval, err := plotLabelPDFApprovalHash(*source)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(source.Bytes)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &PlotLabelPDFExportReview{
		Preview: source.Preview, Project: source.Selection.Project, ProjectPath: source.Selection.ProjectPath,
		SU: source.Selection.SU, SUPath: source.Selection.SUPath, ApprovalHash: approval,
		PDFSHA256: hex.EncodeToString(hash[:]), Layout: "portable-letter-single-fixed-ascii-v1", SessionID: s.sessionID,
	}, nil
}

func (s *PlotLabelPDFExportService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*PlotLabelPDFExportOutcome, error) {
	if err := s.authorize(ctx, true); err != nil {
		return nil, err
	}
	return s.exportReviewed(ctx, contextID, requestJSON, plotLabelPDFPublicationHooks{runtime: s.runtime})
}

func (s *PlotLabelPDFExportService) exportReviewed(ctx context.Context, contextID, requestJSON string, hooks plotLabelPDFPublicationHooks) (outcome *PlotLabelPDFExportOutcome, resultErr error) {
	if err := s.authorize(ctx, true); err != nil {
		return nil, err
	}
	var request PlotLabelPDFExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	execute, prior, err := s.beginAttempt(ctx, contextID, request)
	if err != nil || !execute {
		return prior, err
	}
	defer func() {
		if outcome == nil {
			outcome = plotLabelPDFOutcome(request.Destination, artifactPublication{}, resultErr)
		}
		s.finishAttempt(request.RequestID, *outcome)
	}()
	source, err := s.contexts.readPlotLabelPDFPreparation(ctx, contextID, request.PlotNumber, hooks.runtime)
	if err != nil {
		return nil, err
	}
	approval, err := plotLabelPDFApprovalHash(*source)
	if err != nil {
		return nil, err
	}
	if approval != request.ApprovalHash {
		return nil, errors.New("label PDF source/date/fonts/owner differs; prepare a new review")
	}
	result, publicationErr := s.contexts.publishOwnedPlotLabelPDF(ctx, contextID, *source, request.Destination, hooks)
	return plotLabelPDFOutcome(request.Destination, result, publicationErr), nil
}

func plotLabelPDFOutcome(destination string, result artifactPublication, err error) *PlotLabelPDFExportOutcome {
	receipt := artifactPublicationOutcome(destination, result, err, "label PDF")
	return &PlotLabelPDFExportOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}
}
