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
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type vegetationProjectWorkbookReadRequest struct {
	Revision          string `json:"revision"`
	BuildApprovalHash string `json:"buildApprovalHash"`
	Scope             string `json:"scope"`
}

func validateProjectWorkbookReadRequest(revision, hash, scope string) error {
	if scope != "unlumped" {
		return errors.New("project workbook requires explicit unlumped report scope")
	}
	return validateProjectReadRequest(vegetationProjectReadRequest{Revision: revision, ApprovalHash: hash})
}

func (request *vegetationProjectWorkbookReadRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationProjectWorkbookReadRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "project workbook review", "revision", "buildApprovalHash", "scope"); err != nil {
		return err
	}
	if err := validateProjectWorkbookReadRequest(decoded.Revision, decoded.BuildApprovalHash, decoded.Scope); err != nil {
		return err
	}
	*request = vegetationProjectWorkbookReadRequest(decoded)
	return nil
}

type vegetationProjectWorkbookExportRequest struct {
	Revision          string `json:"revision"`
	BuildApprovalHash string `json:"buildApprovalHash"`
	Scope             string `json:"scope"`
	CreatedDate       string `json:"createdDate"`
	ApprovalHash      string `json:"approvalHash"`
	Destination       string `json:"destination"`
}

func (request *vegetationProjectWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationProjectWorkbookExportRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "project workbook publication",
		"revision", "buildApprovalHash", "scope", "createdDate", "approvalHash", "destination"); err != nil {
		return err
	}
	if err := validateProjectWorkbookReadRequest(decoded.Revision, decoded.BuildApprovalHash, decoded.Scope); err != nil {
		return err
	}
	hash, err := hex.DecodeString(decoded.ApprovalHash)
	if err != nil || len(hash) != 32 || hex.EncodeToString(hash) != decoded.ApprovalHash {
		return errors.New("project workbook requires its exact lowercase reviewed workbook approval")
	}
	date, err := time.Parse("2006-01-02", decoded.CreatedDate)
	if err != nil || date.Format("2006-01-02") != decoded.CreatedDate || date.Year() < 100 {
		return errors.New("project workbook requires its exact reviewed ISO creation date")
	}
	if err := validateGoogleEarthReviewStrings(decoded.Destination); err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(decoded.Destination), ".xlsx") {
		return errors.New("project workbook requires an explicit .xlsx destination")
	}
	*request = vegetationProjectWorkbookExportRequest(decoded)
	return nil
}

type VegetationProjectWorkbookSummary struct {
	CreatedDate        string                      `json:"createdDate"`
	EnvironmentRows    int                         `json:"environmentRows"`
	SourcePlotRows     int                         `json:"sourcePlotRows"`
	SourceKind         string                      `json:"sourceKind"`
	ProjectTuples      []LongVegetationProjectUnit `json:"projectTuples"`
	SpeciesVersion     ProjectMetadataCell         `json:"speciesVersion"`
	VersionStatus      string                      `json:"versionStatus"`
	VersionDefinitions ProjectMetadataTable        `json:"versionDefinitions"`
}

type LongVegetationProjectWorkbookReview struct {
	Scope          LongVegetationReportScope         `json:"scope"`
	Preview        LongVegetationPreview             `json:"preview"`
	Options        VegetationWorkbookOptions         `json:"options"`
	Summary        *VegetationProjectWorkbookSummary `json:"summary"`
	ReportScope    string                            `json:"reportScope"`
	CreatedDate    string                            `json:"createdDate"`
	Sheets         []VegetationWorkbookSheet         `json:"sheets"`
	SkippedUnits   []VegetationWorkbookSkippedUnit   `json:"skippedUnits"`
	ApprovalHash   string                            `json:"approvalHash"`
	WorkbookSHA256 string                            `json:"workbookSHA256"`
	Bytes          int                               `json:"bytes"`
}

type vegetationProjectWorkbookSource struct {
	Scope       LongVegetationReportScope
	Report      vegetationLayerReport
	Settings    LongVegetationSettings
	Layout      vegetationWorkbookLayout
	Summary     *vegetationProjectWorkbookSummary
	Config      configValues
	Tables      []ProjectMetadataTable
	ReportScope string
	CreatedDate string
}

type LongVegetationProjectWorkbookService struct {
	scope    *LongVegetationReportScopeService
	enabled  bool
	now      func() time.Time
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationProjectWorkbookService(contexts *ContextService, projectEnabled, workbookEnabled bool) *LongVegetationProjectWorkbookService {
	return &LongVegetationProjectWorkbookService{
		scope: NewLongVegetationReportScopeService(contexts, projectEnabled), enabled: workbookEnabled, now: time.Now,
	}
}

func (s *LongVegetationProjectWorkbookService) require(ctx context.Context, contextID string) error {
	if s == nil || !s.enabled || s.scope == nil {
		return errors.New("project workbook publication is disabled in this session")
	}
	return s.scope.require(ctx, contextID)
}

func (s *LongVegetationProjectWorkbookService) readSource(ctx context.Context, plots *PlotService, tx *sql.Tx,
	contextID string, request vegetationProjectWorkbookReadRequest, createdDate string) (vegetationProjectWorkbookSource, error) {
	fail := func(err error) (vegetationProjectWorkbookSource, error) {
		return vegetationProjectWorkbookSource{}, err
	}
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return fail(err)
	}
	owned, options, err := s.scope.readProjectOptionsForConfig(ctx, plots, tx, contextID,
		vegetationProjectReadRequest{Revision: request.Revision, ApprovalHash: request.BuildApprovalHash}, values)
	if err != nil {
		return fail(err)
	}
	layout, err := decodeVegetationWorkbookLayout(values)
	if err != nil {
		return fail(err)
	}
	fields := owned.Scope.Fields
	if owned.Scope.SourceKind == vegetationProjectAllScope {
		fields = nil
	}
	source, err := readLongVegetationProjectSource(ctx, plots.projects.sqlite, tx, fields, options)
	if err != nil {
		return fail(err)
	}
	result := vegetationProjectWorkbookSource{Scope: owned.Scope, Report: source.Report, Settings: owned.Options.Settings,
		Layout: layout, Config: values, Tables: source.Tables, ReportScope: request.Scope, CreatedDate: createdDate}
	if layout.ReportSummary {
		descriptions, err := readProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
		if err != nil {
			return fail(err)
		}
		var admin ProjectMetadataTable
		if owned.Scope.SourceKind == vegetationProjectFieldsScope {
			admin = source.Tables[5]
		}
		summary, err := prepareVegetationProjectWorkbookSummary(ctx, createdDate, source.Tables[1], admin, descriptions, owned.Scope, source.Report)
		if err != nil {
			return fail(err)
		}
		result.Summary = &summary
		result.Tables = append(result.Tables, descriptions)
	}
	return result, nil
}

func projectWorkbookApproval(source vegetationProjectWorkbookSource, workbook vegetationWorkbook) (string, error) {
	raw, err := json.Marshal(struct {
		Source vegetationProjectWorkbookSource
		Bytes  []byte
	}{source, workbook.Bytes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO original-project Long Vegetation workbook approval v1\x00"), raw...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *LongVegetationProjectWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*LongVegetationProjectWorkbookReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	var request vegetationProjectWorkbookReadRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	createdDate := s.now().Format("2006-01-02")
	return withContextPlotRequest(ctx, s.scope.contexts, contextID, func(plots *PlotService) (*LongVegetationProjectWorkbookReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(_ *sqliteContext, tx *sql.Tx) (*LongVegetationProjectWorkbookReview, error) {
			source, err := s.readSource(ctx, plots, tx, contextID, request, createdDate)
			if err != nil {
				return nil, err
			}
			prepared, err := prepareLongVegetationProjectWorkbookWithSummary(ctx, source.Report, source.Scope, source.Settings, source.Layout, source.Summary)
			if err != nil {
				return nil, err
			}
			approval, err := projectWorkbookApproval(source, prepared.Workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(prepared.Workbook.Bytes)
			result := &LongVegetationProjectWorkbookReview{Scope: prepared.Preview.Scope, Preview: prepared.Preview.Preview,
				Options:     VegetationWorkbookOptions{source.Layout.QuickReport, source.Layout.SpaceBetweenGroups, source.Layout.ReportSummary},
				ReportScope: source.ReportScope, CreatedDate: source.CreatedDate, ApprovalHash: approval,
				WorkbookSHA256: hex.EncodeToString(hash[:]), Bytes: len(prepared.Workbook.Bytes),
				Sheets: []VegetationWorkbookSheet{}, SkippedUnits: []VegetationWorkbookSkippedUnit{}}
			if summary := source.Summary; summary != nil {
				result.Summary = &VegetationProjectWorkbookSummary{summary.Header.CreatedDate, summary.Header.EnvironmentRows,
					summary.SourcePlotRows, summary.SourceKind, summary.ProjectTuples, summary.Header.SpeciesVersion,
					summary.Header.VersionStatus, summary.Header.VersionDefinitions}
			}
			for _, sheet := range prepared.Workbook.Sheets {
				result.Sheets = append(result.Sheets, VegetationWorkbookSheet{sheet.Unit, sheet.Name})
			}
			for _, skipped := range prepared.Workbook.SkippedUnits {
				result.SkippedUnits = append(result.SkippedUnits, VegetationWorkbookSkippedUnit{skipped.Unit, skipped.Reason})
			}
			return result, nil
		})
	})
}

func (s *LongVegetationProjectWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*VegetationWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, vegetationWorkbookHooks{})
}

func (s *LongVegetationProjectWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string,
	hooks vegetationWorkbookHooks) (*VegetationWorkbookOutcome, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	var request vegetationProjectWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.scope.contexts, contextID, func(plots *PlotService) (*VegetationWorkbookOutcome, error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		read := func() (vegetationProjectWorkbookSource, error) {
			return withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (vegetationProjectWorkbookSource, error) {
				return s.readSource(ctx, plots, tx, contextID,
					vegetationProjectWorkbookReadRequest{request.Revision, request.BuildApprovalHash, request.Scope}, request.CreatedDate)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		prepared, err := prepareLongVegetationProjectWorkbookWithSummary(ctx, expected.Report, expected.Scope, expected.Settings, expected.Layout, expected.Summary)
		if err != nil {
			return nil, err
		}
		approval, err := projectWorkbookApproval(expected, prepared.Workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("project workbook raw source/configuration/scope/date changed; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("project workbook complete physical/configuration/metadata witness changed; reload review")
			}
			return nil
		}
		result, publishErr := publishArtifactChecked(ctx, request.Destination, artifactPublicationFormat{
			label: "project Long Vegetation workbook", artifact: "XLSX", commitLabel: "workbook", stagePattern: ".vpro-project-vegetation-workbook-*",
			encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(prepared.Workbook.Bytes), nil
			},
			validate: func(ctx context.Context, actual []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !bytes.Equal(actual, prepared.Workbook.Bytes) {
					return errors.New("staged project workbook differs from exact reviewed bytes")
				}
				return nil
			},
		}, validateSource, hooks.publication)
		if result.Published {
			publishErr = errors.Join(publishErr, validateSource())
			if publishErr != nil {
				publishErr = fmt.Errorf("Project workbook published; do not replay: %w", publishErr)
			}
		}
		receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "project Long Vegetation workbook")
		return &VegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
