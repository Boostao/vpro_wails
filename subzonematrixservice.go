package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const subzoneMatrixFeatureEnvironment = "VPRO_SUBZONE_MATRIX_WORKBOOK"

type SubzoneMatrixRow struct {
	Name   *string   `json:"name"`
	Values []*string `json:"values"`
}

type SubzoneMatrixPreview struct {
	ContextID     string             `json:"contextId"`
	Hierarchy     string             `json:"hierarchy"`
	HierarchyPath string             `json:"hierarchyPath"`
	LabelsPath    string             `json:"labelsPath"`
	SourceSHA256  string             `json:"sourceSHA256"`
	Columns       []string           `json:"columns"`
	Rows          []SubzoneMatrixRow `json:"rows"`
	Total         int                `json:"total"`
	Offset        int                `json:"offset"`
	Limit         int                `json:"limit"`
}

type SubzoneMatrixWorkbookReview struct {
	Preview        SubzoneMatrixPreview `json:"preview"`
	ApprovalHash   string               `json:"approvalHash"`
	WorkbookSHA256 string               `json:"workbookSHA256"`
	Bytes          int                  `json:"bytes"`
}

type SubzoneMatrixWorkbookOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

type subzoneMatrixPageRequest struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

func (request *subzoneMatrixPageRequest) UnmarshalJSON(data []byte) error {
	type plain subzoneMatrixPageRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Subzone Matrix preview", "offset", "limit"); err != nil {
		return err
	}
	if decoded.Offset < 0 || decoded.Limit < 1 || decoded.Limit > 100 {
		return errors.New("Subzone Matrix preview requires offset >= 0 and limit 1..100")
	}
	*request = subzoneMatrixPageRequest(decoded)
	return nil
}

type subzoneMatrixExportRequest struct {
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
}

func (request *subzoneMatrixExportRequest) UnmarshalJSON(data []byte) error {
	type plain subzoneMatrixExportRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Subzone Matrix export", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = subzoneMatrixExportRequest(decoded)
	return nil
}

type SubzoneMatrixService struct {
	contexts *ContextService
	enabled  bool
}

type subzoneMatrixPublicationHooks struct {
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

func NewSubzoneMatrixService(contexts *ContextService, lookup func(string) (string, bool)) (*SubzoneMatrixService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(subzoneMatrixFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SubzoneMatrixService{contexts: contexts, enabled: enabled}, nil
}

func (s *SubzoneMatrixService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Subzone Matrix requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Subzone Matrix is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Subzone Matrix owned context is unavailable")
	}
	return nil
}

func subzoneMatrixPage(source subzoneMatrixReview, offset, limit int) (SubzoneMatrixPreview, error) {
	digest, err := subzoneMatrixOriginalSHA256(source)
	if err != nil {
		return SubzoneMatrixPreview{}, err
	}
	preview := SubzoneMatrixPreview{
		ContextID: source.ContextID, Hierarchy: source.Hierarchy, HierarchyPath: source.HierarchyPath,
		LabelsPath: source.LabelsPath, SourceSHA256: digest, Columns: append([]string{}, source.Matrix.Columns...),
		Rows: make([]SubzoneMatrixRow, 0), Total: len(source.Matrix.Rows), Offset: offset, Limit: limit,
	}
	start := min(offset, len(source.Matrix.Rows))
	for _, row := range source.Matrix.Rows[start : start+min(limit, len(source.Matrix.Rows)-start)] {
		values := make([]*string, len(row.Values))
		for i, value := range row.Values {
			if value != nil {
				text := *value
				values[i] = &text
			}
		}
		var name *string
		if row.Name != nil {
			text := *row.Name
			name = &text
		}
		preview.Rows = append(preview.Rows, SubzoneMatrixRow{name, values})
	}
	return preview, nil
}

func (s *SubzoneMatrixService) GetPreview(ctx context.Context, contextID, requestJSON string) (*SubzoneMatrixPreview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request subzoneMatrixPageRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	source, err := s.contexts.readSubzoneMatrix(ctx, contextID)
	if err != nil {
		return nil, err
	}
	preview, err := subzoneMatrixPage(source, request.Offset, request.Limit)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &preview, nil
}

func subzoneMatrixApproval(source subzoneMatrixReview, workbook subzoneMatrixWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		ContextID, Hierarchy, HierarchyPath, LabelsPath, SourceSHA256, WorkbookSHA256 string
	}{source.ContextID, source.Hierarchy, source.HierarchyPath, source.LabelsPath, workbook.SourceSHA256,
		fmt.Sprintf("%x", sha256.Sum256(workbook.Bytes))})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO Subzone Matrix XLSX approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *SubzoneMatrixService) GetReview(ctx context.Context, contextID string) (*SubzoneMatrixWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	source, err := s.contexts.readSubzoneMatrix(ctx, contextID)
	if err != nil {
		return nil, err
	}
	workbook, err := prepareSubzoneMatrixWorkbook(ctx, source)
	if err != nil {
		return nil, err
	}
	approval, err := subzoneMatrixApproval(source, workbook)
	if err != nil {
		return nil, err
	}
	preview, err := subzoneMatrixPage(source, 0, 25)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SubzoneMatrixWorkbookReview{preview, approval, fmt.Sprintf("%x", sha256.Sum256(workbook.Bytes)), len(workbook.Bytes)}, nil
}

func (s *SubzoneMatrixService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*SubzoneMatrixWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, subzoneMatrixPublicationHooks{})
}

func (s *SubzoneMatrixService) exportReviewed(ctx context.Context, contextID, requestJSON string, hooks subzoneMatrixPublicationHooks) (*SubzoneMatrixWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request subzoneMatrixExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Subzone Matrix requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Subzone Matrix requires the exact reviewed approval hash")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SubzoneMatrixWorkbookOutcome, error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		read := func() (subzoneMatrixReview, error) {
			return withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (subzoneMatrixReview, error) {
				return readSubzoneMatrixSnapshot(ctx, owner, tx, contextID)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareSubzoneMatrixWorkbook(ctx, expected)
		if err != nil {
			return nil, err
		}
		approval, err := subzoneMatrixApproval(expected, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Subzone Matrix scope/complete original source/format differs; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Subzone Matrix complete original source changed; reload review")
			}
			return nil
		}
		result, publishErr := publishArtifactChecked(ctx, request.Destination, artifactPublicationFormat{
			label: "Subzone Matrix workbook", artifact: "XLSX", commitLabel: "workbook",
			stagePattern: ".vpro-subzone-matrix-*",
			encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(workbook.Bytes), nil
			},
			validate: func(ctx context.Context, actual []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !bytes.Equal(actual, workbook.Bytes) {
					return errors.New("staged Subzone Matrix workbook differs from reviewed bytes")
				}
				return nil
			},
		}, validateSource, hooks.publication)
		if result.Published {
			publishErr = errors.Join(publishErr, validateSource())
			if publishErr != nil {
				publishErr = fmt.Errorf("Workbook published; do not replay: %w", publishErr)
			}
		}
		receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "Subzone Matrix workbook")
		return &SubzoneMatrixWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
