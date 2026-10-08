package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const tableCSVArchiveFeatureEnvironment = "VPRO_TABLE_CSV_ARCHIVE_EXPORT"

func tableCSVArchiveFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(tableCSVArchiveFeatureEnvironment, lookup)
}

type TableCSVArchiveService struct {
	contexts *ContextService
	enabled  bool
}

func NewTableCSVArchiveService(contexts *ContextService, enabled bool) *TableCSVArchiveService {
	return &TableCSVArchiveService{contexts: contexts, enabled: enabled}
}

func (s *TableCSVArchiveService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("table CSV archive requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("table CSV archive export is disabled in this session")
	}
	if s.contexts == nil {
		return errors.New("table CSV archive context service is unavailable")
	}
	return nil
}

type TableCSVArchiveReviewRequest struct {
	Table string `json:"table"`
}

func (request *TableCSVArchiveReviewRequest) UnmarshalJSON(data []byte) error {
	type plain TableCSVArchiveReviewRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "table CSV archive review", "table"); err != nil {
		return err
	}
	*request = TableCSVArchiveReviewRequest(decoded)
	return nil
}

type TableCSVArchiveExportRequest struct {
	Table        string `json:"table"`
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
}

func (request *TableCSVArchiveExportRequest) UnmarshalJSON(data []byte) error {
	type plain TableCSVArchiveExportRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "table CSV archive export", "table", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = TableCSVArchiveExportRequest(decoded)
	return nil
}

type TableCSVArchiveReview struct {
	Review        ProjectTableCSVReview `json:"review"`
	ApprovalHash  string                `json:"approvalHash"`
	ArchiveSHA256 string                `json:"archiveSHA256"`
	ByteCount     int                   `json:"byteCount"`
	Format        string                `json:"format"`
	Version       int                   `json:"version"`
}

type TableCSVArchiveOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

func validateTableCSVArchiveStrings(values ...string) error {
	for _, value := range values {
		if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return errors.New("table CSV archive requires explicit nonempty valid Unicode identities and destination without NUL; no text repaired")
		}
	}
	return nil
}

func tableCSVArchiveApprovalHash(ctx context.Context, review ownedTableCSVReview) (string, error) {
	review.Document = snapshotTableCSVBundleDocument(review.Document)
	if err := validateTableCSVArchiveStrings(review.ContextID, review.Project, review.ProjectPath); err != nil {
		return "", err
	}
	if err := validateOwnedTableCSVArchive(ctx, ownedTableCSVArchive{review.DescriptionMetadataPresent, review.Document}); err != nil {
		return "", err
	}
	data, err := json.Marshal(review)
	if err != nil {
		return "", fmt.Errorf("table CSV archive source approval: %w", err)
	}
	digest := sha256.Sum256(append([]byte("VPRO owned table CSV approval v1\x00"), data...))
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest[:]), nil
}

func (s *TableCSVArchiveService) GetTableCSVArchiveReview(ctx context.Context, contextID, requestJSON string) (*TableCSVArchiveReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request TableCSVArchiveReviewRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateTableCSVArchiveStrings(contextID, request.Table); err != nil {
		return nil, err
	}
	source, err := s.contexts.readProjectTableCSV(ctx, contextID, request.Table)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeOwnedTableCSVArchive(ctx, ownedTableCSVArchive{source.DescriptionMetadataPresent, source.Document})
	if err != nil {
		return nil, err
	}
	approval, err := tableCSVArchiveApprovalHash(ctx, source)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &TableCSVArchiveReview{
		Review: ProjectTableCSVReview{
			ContextID: source.ContextID, Project: source.Project, ProjectPath: source.ProjectPath,
			DescriptionMetadataPresent: source.DescriptionMetadataPresent,
			Manifest:                   source.Document.Manifest, CSV: string(source.Document.Data),
		},
		ApprovalHash: approval, ArchiveSHA256: hex.EncodeToString(digest[:]),
		ByteCount: len(encoded), Format: ownedTableCSVArchiveFormat, Version: 1,
	}, nil
}

func (s *TableCSVArchiveService) ExportReviewedTableCSVArchive(ctx context.Context, contextID, requestJSON string) (*TableCSVArchiveOutcome, error) {
	return s.exportReviewedTableCSVArchive(ctx, contextID, requestJSON, tableCSVOwnedPublicationHooks{})
}

func (s *TableCSVArchiveService) exportReviewedTableCSVArchive(ctx context.Context, contextID, requestJSON string, hooks tableCSVOwnedPublicationHooks) (*TableCSVArchiveOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request TableCSVArchiveExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateTableCSVArchiveStrings(contextID, request.Table, request.Destination); err != nil {
		return nil, err
	}
	if len(request.ApprovalHash) != 64 || strings.ToLower(request.ApprovalHash) != request.ApprovalHash {
		return nil, errors.New("table CSV archive requires the exact reviewed source approval hash")
	}
	if _, err := hex.DecodeString(request.ApprovalHash); err != nil {
		return nil, errors.New("table CSV archive requires a hexadecimal source approval hash")
	}
	source, err := s.contexts.readProjectTableCSV(ctx, contextID, request.Table)
	if err != nil {
		return nil, err
	}
	approval, err := tableCSVArchiveApprovalHash(ctx, source)
	if err != nil {
		return nil, err
	}
	if approval != request.ApprovalHash {
		return nil, errors.New("table CSV archive owned source/schema/rows/metadata differs; prepare a new review before publication")
	}
	result, publishErr := s.contexts.publishOwnedProjectTableCSVArchiveWithHooks(ctx, contextID, source, request.Destination, hooks)
	receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "table CSV archive")
	return &TableCSVArchiveOutcome{
		Status: receipt.Status, RequestedDestination: receipt.RequestedDestination,
		Path: receipt.Path, SHA256: receipt.SHA256, ErrorMessage: receipt.ErrorMessage,
	}, nil
}
