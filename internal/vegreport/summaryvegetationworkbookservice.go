package vegreport

import (
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
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const summaryVegetationWorkbookDraftScope = "Explicit read-only normal-SU layer inspection; literal XLSX, not original NewShortVeg orchestration, saved callbacks, ancillary sheets or formatting/printing parity"
const summaryVegetationAncillaryWorkbookScope = "Explicit read-only normal-SU layer inspection with SU Table and optional ReportSummary; literal XLSX, not original NewShortVeg preflight/callbacks or formatting/printing parity"
const summaryVegetationEffectiveReferencesScope = "Explicit read-only normal-SU layer inspection with projected attribute/common/guide references; literal comparison and XLSX, not original Report callbacks, Access collation or formatting/printing parity"
const summaryVegetationEffectiveAncillaryScope = "Explicit read-only normal-SU layer inspection with projected attribute/common/guide references, SU Table and optional ReportSummary; literal comparison and XLSX, not original Report callbacks, Access collation or formatting/printing parity"
const summaryVegetationGroupedWorkbookScope = "Explicit read-only normal-SU Strata/Lifeform/None inspection; typed XLSX, not grouped common/guide references, ancillary worksheets, original Report callbacks or formatting/printing parity"
const summaryVegetationStrataReferencesScope = "Explicit read-only normal-SU Strata effective-reference inspection; typed XLSX, not Lifeform/None common names, grouped ancillary worksheets, original Report callbacks or formatting/printing parity"
const summaryVegetationGroupedAttributesScope = "Explicit read-only normal-SU Lifeform/None effective-attribute inspection; typed XLSX, no common/guide names, grouped ancillary worksheets, original Report callbacks or formatting/printing parity"
const summaryVegetationGroupedAncillaryScope = "Explicit read-only normal-SU Strata/Lifeform/None inspection with optional effective references, SU Table and optional ReportSummary; typed XLSX, no Lifeform/None common names, original Report callbacks or formatting/printing parity"
const summaryVegetationConstancyScope = "Explicit read-only normal-SU Constancy inspection with numeric lifeform and raw-species grouping; typed XLSX with optional environment/ancillary composition, adapted raw-code tie ordering, not original Report callbacks or formatting/printing parity"
const summaryVegetationUnanimousCommonScope = "Explicit read-only normal-SU Lifeform/None unanimous common-name inspection; typed XLSX with optional environment/ancillary composition, source First empty-to-NULL output and explicit mixed-name refusal, not original Report/guide callbacks, Access collation or formatting/printing parity"

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
	source *SummaryVegetationEnvironmentalLifeformService
	now    func() time.Time
}

type summaryVegetationWorkbookHooks struct {
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func NewSummaryVegetationWorkbookService(source *SummaryVegetationEnvironmentalLifeformService) (*SummaryVegetationWorkbookService, error) {
	return &SummaryVegetationWorkbookService{source: source, now: time.Now}, nil
}

func (s *SummaryVegetationWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Summary draft workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.source == nil || s.source.environment == nil || s.source.environment.report == nil {
		return errors.New("Summary draft workbook requires Summary Vegetation")
	}
	contexts := s.source.environment.report.contexts
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Sqlite == nil ||
		contexts.Projects.Preferences == nil || contexts.Plots == nil {
		return errors.New("Summary draft workbook owned context is unavailable")
	}
	return nil
}

func requireSummaryWorkbookUnfilteredScope(ctx context.Context, config *vprocore.DesktopConfig) error {
	if err := sqlutil.AcquireMutexLease(ctx, &config.Mu); err != nil {
		return err
	}
	defer config.Mu.Unlock()
	values, err := config.ReadLocked()
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

func (s *SummaryVegetationWorkbookService) readSource(ctx context.Context, plots *vprocore.PlotService, owner *vprocore.SqliteContext,
	tx *sql.Tx, contextID string, options SummaryVegetationWorkbookOptions) (summaryVegetationWorkbookSource, error) {
	return s.readSourceAt(ctx, plots, owner, tx, contextID, options, "")
}

func (s *SummaryVegetationWorkbookService) readSourceAt(ctx context.Context, plots *vprocore.PlotService, owner *vprocore.SqliteContext,
	tx *sql.Tx, contextID string, options SummaryVegetationWorkbookOptions, createdAt string) (summaryVegetationWorkbookSource, error) {
	if err := validateSummaryWorkbookInspectionOptions(options); err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	if options.Ancillary != nil {
		if err := validateSummaryWorkbookCreatedAt(createdAt); err != nil {
			return summaryVegetationWorkbookSource{}, err
		}
	} else if createdAt != "" {
		return summaryVegetationWorkbookSource{}, errors.New("Summary draft workbook creation time requires explicit ancillary mode")
	}
	if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.Projects.Preferences); err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	source, err := readSummaryVegetationWorkbookSource(ctx, s.source, plots, owner, tx, contextID, options)
	if err != nil {
		return summaryVegetationWorkbookSource{}, err
	}
	if options.Ancillary != nil {
		additional := summaryVegetationWorkbookAncillarySource{CreatedAt: createdAt}
		if options.Ancillary.CreateSummary {
			var retainedEnv *vprocore.ProjectMetadataTable
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
	if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.Projects.Preferences); err != nil {
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
	if options.References != nil {
		domain = "VPRO Summary effective references workbook approval v1\x00"
	}
	if options.GroupAndOrder != nil {
		domain = "VPRO Summary grouped workbook approval v1\x00"
		if options.References != nil {
			domain = "VPRO Summary Strata effective references workbook approval v1\x00"
			if *options.GroupAndOrder != 12 {
				domain = "VPRO Summary grouped attributes workbook approval v1\x00"
			}
		}
		if options.Ancillary != nil {
			domain = "VPRO Summary grouped ancillary workbook approval v1\x00"
		}
	}
	if options.Constancy {
		domain = "VPRO Summary Constancy workbook approval v1\x00"
	}
	if options.UnanimousCommon {
		domain = "VPRO Summary unanimous common-name workbook approval v1\x00"
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
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationWorkbookReview, error) {
			createdAt := ""
			if options.Ancillary != nil {
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
			if err := requireSummaryWorkbookUnfilteredScope(ctx, plots.Projects.Preferences); err != nil {
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
					SourcePath: owner.Attachments["VPro64"], EnvSourcePath: owner.Attachments["project"],
					MasterSourcePath: owner.Attachments["VLists"], Source: summaryEnvironmentalLifeformSource,
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
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
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*VegetationWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (summaryVegetationWorkbookSource, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (summaryVegetationWorkbookSource, error) {
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
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "Summary draft workbook", Artifact: "XLSX", CommitLabel: "workbook",
			StagePattern: ".vpro-summary-draft-workbook-*",
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
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "Summary draft workbook")
		return &VegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
