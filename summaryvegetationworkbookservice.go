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

const summaryVegetationWorkbookFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_WORKBOOK"
const summaryVegetationWorkbookDraftScope = "Explicit read-only normal-SU layer inspection; literal XLSX, not original NewShortVeg orchestration, saved callbacks, ancillary sheets or formatting/printing parity"

type SummaryVegetationWorkbookReview struct {
	Preview            SummaryVegetationPreview                       `json:"preview"`
	Environment        *SummaryVegetationEnvironmentalLifeformPreview `json:"environment"`
	Options            SummaryVegetationWorkbookOptions               `json:"options"`
	Scope              string                                         `json:"scope"`
	Sheet              string                                         `json:"sheet"`
	FirstVegetationRow int                                            `json:"firstVegetationRow"`
	ApprovalHash       string                                         `json:"approvalHash"`
	WorkbookSHA256     string                                         `json:"workbookSHA256"`
	Bytes              int                                            `json:"bytes"`
}

type SummaryVegetationWorkbookService struct {
	source  *SummaryVegetationEnvironmentalLifeformService
	enabled bool
}

type summaryVegetationWorkbookHooks struct {
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

func NewSummaryVegetationWorkbookService(source *SummaryVegetationEnvironmentalLifeformService,
	lookup func(string) (string, bool)) (*SummaryVegetationWorkbookService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(summaryVegetationWorkbookFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationWorkbookService{source: source, enabled: enabled}, nil
}

func (s *SummaryVegetationWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Summary draft workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary draft workbook is disabled in this session")
	}
	if s.source == nil || s.source.environment == nil || s.source.environment.report == nil ||
		!s.source.environment.report.enabled {
		return errors.New("Summary draft workbook requires enabled Summary Vegetation")
	}
	contexts := s.source.environment.report.contexts
	if contexts == nil || contexts.projects == nil || contexts.projects.sqlite == nil ||
		contexts.projects.preferences == nil || contexts.plots == nil {
		return errors.New("Summary draft workbook owned context is unavailable")
	}
	return nil
}

func requireSummaryWorkbookUnfilteredScope(ctx context.Context, config *desktopConfig) error {
	if err := acquireMutexLease(ctx, &config.mu); err != nil {
		return err
	}
	defer config.mu.Unlock()
	values, err := config.readLocked()
	if err != nil {
		return err
	}
	enforce, err := longVegetationConfigBool(values, "DataQualityFilterEnforce")
	if err != nil {
		return err
	}
	if enforce {
		return errors.New("Summary draft workbook quality enforcement is saved but unqualified; no unfiltered workbook returned")
	}
	return ctx.Err()
}

func (s *SummaryVegetationWorkbookService) readSource(ctx context.Context, plots *PlotService, owner *sqliteContext,
	tx *sql.Tx, contextID string, options SummaryVegetationWorkbookOptions) (summaryVegetationWorkbookSource, error) {
	if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.projects.preferences); err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	source, err := readSummaryVegetationWorkbookSource(ctx, s.source, plots, owner, tx, contextID, options)
	if err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.projects.preferences); err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	return source, nil
}

func summaryVegetationWorkbookApproval(source summaryVegetationWorkbookSource, workbook summaryVegetationWorkbook,
	options SummaryVegetationWorkbookOptions) (string, error) {
	data, err := json.Marshal(struct {
		Source   summaryVegetationWorkbookSource
		Options  SummaryVegetationWorkbookOptions
		Scope    string
		Sheet    string
		FirstRow int
		Workbook []byte
	}{source, options, summaryVegetationWorkbookDraftScope, workbook.Sheet, workbook.FirstVegetationRow, workbook.Bytes})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte("VPRO Summary draft workbook approval v2\x00"), data...))
	return hex.EncodeToString(digest[:]), nil
}

func (s *SummaryVegetationWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*SummaryVegetationWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var options SummaryVegetationWorkbookOptions
	if err := json.Unmarshal([]byte(requestJSON), &options); err != nil {
		return nil, err
	}
	contexts := s.source.environment.report.contexts
	return withContextPlotRequest(ctx, contexts, contextID, func(plots *PlotService) (*SummaryVegetationWorkbookReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(owner *sqliteContext, tx *sql.Tx) (*SummaryVegetationWorkbookReview, error) {
			source, err := s.readSource(ctx, plots, owner, tx, contextID, options)
			if err != nil {
				return nil, err
			}
			workbook, err := prepareSummaryVegetationWorkbook(ctx, source.Input)
			if err != nil {
				return nil, err
			}
			if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.projects.preferences); err != nil {
				return nil, err
			}
			approval, err := summaryVegetationWorkbookApproval(source, workbook, options)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			review := &SummaryVegetationWorkbookReview{
				Preview: source.Input.Vegetation, Options: options, Scope: summaryVegetationWorkbookDraftScope,
				Sheet: workbook.Sheet, FirstVegetationRow: workbook.FirstVegetationRow,
				ApprovalHash: approval, WorkbookSHA256: hex.EncodeToString(hash[:]), Bytes: len(workbook.Bytes),
			}
			if environment := source.Input.Environment; environment != nil {
				review.Environment = &SummaryVegetationEnvironmentalLifeformPreview{
					Vegetation: review.Preview, FieldList: environment.FieldList, Rows: environment.Rows,
					SourcePath: owner.attachments["VPro64"], EnvSourcePath: owner.attachments["project"],
					MasterSourcePath: owner.attachments["VLists"], Source: summaryEnvironmentalLifeformSource,
				}
			}
			return review, nil
		})
	})
}

func (s *SummaryVegetationWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, summaryVegetationWorkbookHooks{})
}

func (s *SummaryVegetationWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string,
	hooks summaryVegetationWorkbookHooks) (*VegetationWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request summaryVegetationWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Summary draft workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Summary draft workbook requires the exact reviewed approval hash")
	}
	contexts := s.source.environment.report.contexts
	return withContextPlotRequest(ctx, contexts, contextID, func(plots *PlotService) (*VegetationWorkbookOutcome, error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		read := func() (summaryVegetationWorkbookSource, error) {
			return withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (summaryVegetationWorkbookSource, error) {
				return s.readSource(ctx, plots, owner, tx, contextID, request.Options)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareSummaryVegetationWorkbook(ctx, expected.Input)
		if err != nil {
			return nil, err
		}
		approval, err := summaryVegetationWorkbookApproval(expected, workbook, request.Options)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Summary draft workbook physical source/scope/options differ; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Summary draft workbook physical source/metadata differ; reload review")
			}
			return nil
		}
		result, publishErr := publishArtifactChecked(ctx, request.Destination, artifactPublicationFormat{
			label: "Summary draft workbook", artifact: "XLSX", commitLabel: "workbook",
			stagePattern: ".vpro-summary-draft-workbook-*",
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
					return errors.New("staged Summary draft workbook differs from reviewed bytes")
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
		receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "Summary draft workbook")
		return &VegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
