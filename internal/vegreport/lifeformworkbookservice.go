package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

const LifeformWorkbookFeatureEnvironment = "VPRO_LIFEFORM_WORKBOOK"

type lifeformWorkbookRequest struct {
	Details [6]bool
}

type lifeformWorkbookExportRequest struct {
	Details      [6]bool
	ApprovalHash string
	Destination  string
}

func decodeLifeformWorkbookDetails(values []json.RawMessage) ([6]bool, error) {
	var details [6]bool
	if len(values) != len(details) {
		return details, errors.New("Lifeform workbook requires exactly six explicit detail booleans in source order")
	}
	for i, value := range values {
		switch string(bytes.TrimSpace(value)) {
		case "true":
			details[i] = true
		case "false":
		default:
			return [6]bool{}, errors.New("Lifeform workbook detail options require literal true/false; NULL is not false")
		}
	}
	return details, nil
}

func (request *lifeformWorkbookRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Details []json.RawMessage `json:"details"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &raw, "Lifeform workbook review", "details"); err != nil {
		return err
	}
	details, err := decodeLifeformWorkbookDetails(raw.Details)
	if err != nil {
		return err
	}
	request.Details = details
	return nil
}

func (request *lifeformWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Details      []json.RawMessage `json:"details"`
		ApprovalHash string            `json:"approvalHash"`
		Destination  string            `json:"destination"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &raw, "Lifeform workbook export", "details", "approvalHash", "destination"); err != nil {
		return err
	}
	details, err := decodeLifeformWorkbookDetails(raw.Details)
	if err != nil {
		return err
	}
	*request = lifeformWorkbookExportRequest{details, raw.ApprovalHash, raw.Destination}
	return nil
}

type LifeformWorkbookReview struct {
	Lifeform       LifeformSummaryPreview               `json:"lifeform"`
	Attributes     SpeciesAttributeSummaryPreview       `json:"attributes"`
	Details        []bool                               `json:"details"`
	Sheets         []reportcore.VegetationWorkbookSheet `json:"sheets"`
	ApprovalHash   string                               `json:"approvalHash"`
	WorkbookSHA256 string                               `json:"workbookSHA256"`
	Bytes          int                                  `json:"bytes"`
}

type LifeformWorkbookOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

type LifeformWorkbookService struct {
	contexts *vprocore.ContextService
	enabled  bool
}

type lifeformWorkbookHooks struct {
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func NewLifeformWorkbookService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*LifeformWorkbookService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := vprocore.SiviFeature(LifeformWorkbookFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &LifeformWorkbookService{contexts: contexts, enabled: enabled}, nil
}

func (s *LifeformWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Lifeform workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Lifeform workbook is disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Lifeform workbook owned context is unavailable")
	}
	return nil
}

func lifeformWorkbookApproval(input lifeformWorkbookInput, layout lifeformWorkbookLayout, workbook lifeformWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		Source   lifeformWorkbookInput
		Layout   lifeformWorkbookLayout
		Sheets   []reportcore.VegetationWorkbookSheetInternal
		Workbook []byte
	}{input, layout, workbook.Sheets, workbook.Bytes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO Lifeform Summary workbook approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *LifeformWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*LifeformWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request lifeformWorkbookRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LifeformWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LifeformWorkbookReview, error) {
			input, err := readLifeformWorkbookSource(ctx, owner, tx, contextID)
			if err != nil {
				return nil, err
			}
			layout := lifeformWorkbookLayout{Details: request.Details}
			workbook, err := prepareLifeformSummaryWorkbook(ctx, input.Lifeform, input.Attributes, layout)
			if err != nil {
				return nil, err
			}
			approval, err := lifeformWorkbookApproval(input, layout, workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			sheets := make([]reportcore.VegetationWorkbookSheet, 0, len(workbook.Sheets))
			for _, sheet := range workbook.Sheets {
				sheets = append(sheets, reportcore.VegetationWorkbookSheet{Unit: vprocore.CloneSiteUnitCell(sheet.Unit), Name: sheet.Name})
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return &LifeformWorkbookReview{input.Lifeform, input.Attributes, append([]bool(nil), request.Details[:]...),
				sheets, approval, hex.EncodeToString(hash[:]), len(workbook.Bytes)}, nil
		})
	})
}

func (s *LifeformWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*LifeformWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, lifeformWorkbookHooks{})
}

func (s *LifeformWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string, hooks lifeformWorkbookHooks) (*LifeformWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request lifeformWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Lifeform workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Lifeform workbook requires the exact reviewed approval hash")
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LifeformWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (lifeformWorkbookInput, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (lifeformWorkbookInput, error) {
				return readLifeformWorkbookSource(ctx, owner, tx, contextID)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		layout := lifeformWorkbookLayout{Details: request.Details}
		workbook, err := prepareLifeformSummaryWorkbook(ctx, expected.Lifeform, expected.Attributes, layout)
		if err != nil {
			return nil, err
		}
		approval, err := lifeformWorkbookApproval(expected, layout, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Lifeform workbook source/scope/physical rows/detail options differ; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Lifeform workbook raw source/physical rows/metadata differ; reload review")
			}
			return nil
		}
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "Lifeform Summary workbook", Artifact: "XLSX", CommitLabel: "workbook",
			StagePattern: ".vpro-lifeform-workbook-*",
			Encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(workbook.Bytes), nil
			},
			Validate: func(ctx context.Context, actual []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !bytes.Equal(actual, workbook.Bytes) {
					return errors.New("staged Lifeform workbook differs from reviewed bytes")
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
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "Lifeform Summary workbook")
		return &LifeformWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
