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
	"time"
)

const summaryVegetationWorkbookFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_WORKBOOK"
const summaryVegetationWorkbookDraftScope = "Explicit read-only normal-SU layer inspection; literal XLSX, not original NewShortVeg orchestration, saved callbacks, ancillary sheets or formatting/printing parity"
const summaryVegetationAncillaryWorkbookFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK"
const summaryVegetationAncillaryWorkbookScope = "Explicit read-only normal-SU layer inspection with SU Table and optional ReportSummary; literal XLSX, not original NewShortVeg preflight/callbacks or formatting/printing parity"

type SummaryVegetationWorkbookAncillaryUnit struct {
	Code     string  `json:"code"`
	NumPlots int64   `json:"numPlots"`
	LongName *string `json:"longName"`
}

type SummaryVegetationWorkbookReportSummary struct {
	EnvironmentRows           int                                      `json:"environmentRows"`
	EnvironmentCountAvailable bool                                     `json:"environmentCountAvailable"`
	SelectedPlotRows          int                                      `json:"selectedPlotRows"`
	Version                   string                                   `json:"version"`
	VersionStatus             string                                   `json:"versionStatus"`
	Units                     []SummaryVegetationWorkbookAncillaryUnit `json:"units"`
}

type SummaryVegetationWorkbookAncillaryReview struct {
	CreatedAt      string                                  `json:"createdAt"`
	MembershipRows int                                     `json:"membershipRows"`
	Sheets         []string                                `json:"sheets"`
	Summary        *SummaryVegetationWorkbookReportSummary `json:"summary"`
}

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
	Ancillary          *SummaryVegetationWorkbookAncillaryReview      `json:"ancillary,omitempty"`
}

type SummaryVegetationWorkbookService struct {
	source           *SummaryVegetationEnvironmentalLifeformService
	enabled          bool
	ancillaryEnabled bool
	now              func() time.Time
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
	return &SummaryVegetationWorkbookService{source: source, enabled: enabled, now: time.Now}, nil
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
	return s.readSourceAt(ctx, plots, owner, tx, contextID, options, "")
}

func (s *SummaryVegetationWorkbookService) readSourceAt(ctx context.Context, plots *PlotService, owner *sqliteContext,
	tx *sql.Tx, contextID string, options SummaryVegetationWorkbookOptions, createdAt string) (summaryVegetationWorkbookSource, error) {
	if options.Ancillary != nil {
		if !s.ancillaryEnabled {
			return summaryVegetationWorkbookSource{}, errors.New("Summary ancillary workbook is disabled in this session")
		}
		if err := validateSummaryWorkbookCreatedAt(createdAt); err != nil {
			return summaryVegetationWorkbookSource{}, err
		}
	} else if createdAt != "" {
		return summaryVegetationWorkbookSource{}, errors.New("Summary draft workbook creation time requires explicit ancillary mode")
	}
	if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.projects.preferences); err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	source, err := readSummaryVegetationWorkbookSource(ctx, s.source, plots, owner, tx, contextID, options)
	if err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	if options.Ancillary != nil {
		additional := summaryVegetationWorkbookAncillarySource{CreatedAt: createdAt}
		if options.Ancillary.CreateSummary {
			var retainedEnv *ProjectMetadataTable
			if options.IncludeEnv {
				retainedEnv = &source.Environment.Env
			}
			additional.Tables, err = readSummaryVegetationAncillaryTablesWithEnv(ctx, owner, tx, retainedEnv)
			if err != nil {
				return summaryVegetationWorkbookSource{}, err
			}
		}
		source.Ancillary = &additional
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
	}{source, options, summaryWorkbookScope(options), workbook.Sheet, workbook.FirstVegetationRow, workbook.Bytes})
	if err != nil {
		return "", err
	}
	domain := "VPRO Summary draft workbook approval v2\x00"
	if options.Ancillary != nil {
		domain = "VPRO Summary ancillary workbook approval v1\x00"
	}
	digest := sha256.Sum256(append([]byte(domain), data...))
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
			createdAt := ""
			if options.Ancillary != nil {
				if !s.ancillaryEnabled {
					return nil, errors.New("Summary ancillary workbook is disabled in this session")
				}
				if s.now == nil {
					return nil, errors.New("Summary ancillary workbook review clock is unavailable")
				}
				createdAt = s.now().UTC().Format(time.RFC3339)
			}
			source, err := s.readSourceAt(ctx, plots, owner, tx, contextID, options, createdAt)
			if err != nil {
				return nil, err
			}
			workbook, err := prepareSummaryWorkbookSource(ctx, source, options)
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
				Preview: source.Input.Vegetation, Options: options, Scope: summaryWorkbookScope(options),
				Sheet: workbook.Sheet, FirstVegetationRow: workbook.FirstVegetationRow,
				ApprovalHash: approval, WorkbookSHA256: hex.EncodeToString(hash[:]), Bytes: len(workbook.Bytes),
			}
			if source.Ancillary != nil {
				review.Ancillary, err = summaryWorkbookAncillaryReview(source, workbook, options)
				if err != nil {
					return nil, err
				}
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
				return s.readSourceAt(ctx, plots, owner, tx, contextID, request.Options, request.CreatedAt)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareSummaryWorkbookSource(ctx, expected, request.Options)
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
