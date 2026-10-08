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

const siteUnitSummaryWorkbookFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_WORKBOOK"

type siteUnitSummaryWorkbookRequest struct {
	Method int `json:"method"`
}

func (request *siteUnitSummaryWorkbookRequest) UnmarshalJSON(data []byte) error {
	type plain siteUnitSummaryWorkbookRequest
	var value plain
	if err := decodeStrictRequiredJSON(data, &value, "Summary Environment workbook review", "method"); err != nil {
		return err
	}
	*request = siteUnitSummaryWorkbookRequest(value)
	return nil
}

type siteUnitSummaryWorkbookExportRequest struct {
	Method       int    `json:"method"`
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
}

func (request *siteUnitSummaryWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	type plain siteUnitSummaryWorkbookExportRequest
	var value plain
	if err := decodeStrictRequiredJSON(data, &value, "Summary Environment workbook export", "method", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = siteUnitSummaryWorkbookExportRequest(value)
	return nil
}

type SiteUnitSummaryWorkbookScope struct {
	SavedMethod    int `json:"savedMethod"`
	SiteUnitType   int `json:"siteUnitType"`
	OrderBy        int `json:"orderBy"`
	IncludeSpecies int `json:"includeSpecies"`
}

type SiteUnitSummaryWorkbookReview struct {
	Preview        SiteUnitSummaryPreview       `json:"preview"`
	Scope          SiteUnitSummaryWorkbookScope `json:"scope"`
	Sheets         []VegetationWorkbookSheet    `json:"sheets"`
	ApprovalHash   string                       `json:"approvalHash"`
	WorkbookSHA256 string                       `json:"workbookSHA256"`
	Bytes          int                          `json:"bytes"`
}

type SiteUnitSummaryWorkbookOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

type SiteUnitSummaryWorkbookService struct {
	contexts *ContextService
	enabled  bool
}

type siteUnitSummaryWorkbookHooks struct {
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

type siteUnitSummaryWorkbookSource struct {
	Input siteUnitSummaryWorkbookInput
	Scope SiteUnitSummaryWorkbookScope
}

func NewSiteUnitSummaryWorkbookService(contexts *ContextService, lookup func(string) (string, bool)) (*SiteUnitSummaryWorkbookService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(siteUnitSummaryWorkbookFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SiteUnitSummaryWorkbookService{contexts, enabled}, nil
}

func (s *SiteUnitSummaryWorkbookService) authorize(ctx context.Context, method int) error {
	if ctx == nil {
		return errors.New("Summary Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary Environment workbook is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil ||
		s.contexts.projects.preferences == nil {
		return errors.New("Summary Environment workbook owned context/configuration is unavailable")
	}
	if method != 1 && method != 2 {
		return errors.New("Summary Environment workbook requires explicit Mean (1) or Interquartile (2)")
	}
	return nil
}

func readSiteUnitSummaryWorkbookScope(ctx context.Context, config *desktopConfig) (SiteUnitSummaryWorkbookScope, error) {
	if err := acquireMutexLease(ctx, &config.mu); err != nil {
		return SiteUnitSummaryWorkbookScope{}, err
	}
	defer config.mu.Unlock()
	values, err := config.readLocked()
	if err != nil {
		return SiteUnitSummaryWorkbookScope{}, err
	}
	scope := SiteUnitSummaryWorkbookScope{}
	for _, field := range []struct {
		key      string
		min, max int
		target   *int
	}{
		{"SEOptValueMethod", 1, 2, &scope.SavedMethod},
		{"SESuType", 1, 3, &scope.SiteUnitType},
		{"SEOrderBy", 1, 3, &scope.OrderBy},
		{"SEIncludeSppSummary", 0, 1, &scope.IncludeSpecies},
	} {
		*field.target, err = configInt(values, "ReportOptions", field.key, field.min, field.max)
		if err != nil {
			return SiteUnitSummaryWorkbookScope{}, err
		}
	}
	if scope.SiteUnitType != 1 || scope.OrderBy == 2 || scope.IncludeSpecies != 0 {
		return SiteUnitSummaryWorkbookScope{}, errors.New("Saved Summary Environment hierarchy/field-derived/lifeform/species scope is unavailable; no options were reset")
	}
	return scope, ctx.Err()
}

func readSiteUnitSummaryWorkbookPublicationSource(ctx context.Context, plots *PlotService, owner *sqliteContext,
	tx *sql.Tx, contextID string, method int) (siteUnitSummaryWorkbookSource, error) {
	scope, err := readSiteUnitSummaryWorkbookScope(ctx, plots.projects.preferences)
	if err != nil {
		return siteUnitSummaryWorkbookSource{}, err
	}
	input, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, method)
	if err != nil {
		return siteUnitSummaryWorkbookSource{}, err
	}
	return siteUnitSummaryWorkbookSource{input, scope}, nil
}

func siteUnitSummaryWorkbookApproval(source siteUnitSummaryWorkbookSource, workbook siteUnitSummaryWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		Source   siteUnitSummaryWorkbookSource
		Sheets   []vegetationWorkbookSheet
		Workbook []byte
	}{source, workbook.Sheets, workbook.Bytes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO Summary Environment workbook approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *SiteUnitSummaryWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookReview, error) {
	var request siteUnitSummaryWorkbookRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, request.Method); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SiteUnitSummaryWorkbookReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(owner *sqliteContext, tx *sql.Tx) (*SiteUnitSummaryWorkbookReview, error) {
			source, err := readSiteUnitSummaryWorkbookPublicationSource(ctx, plots, owner, tx, contextID, request.Method)
			if err != nil {
				return nil, err
			}
			workbook, err := prepareSiteUnitSummaryWorkbook(ctx, source.Input.Preview)
			if err != nil {
				return nil, err
			}
			scope, err := readSiteUnitSummaryWorkbookScope(ctx, plots.projects.preferences)
			if err != nil || scope != source.Scope {
				return nil, errors.Join(err, errors.New("Summary Environment saved scope changed during preparation; reload review"))
			}
			approval, err := siteUnitSummaryWorkbookApproval(source, workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			sheets := make([]VegetationWorkbookSheet, 0, len(workbook.Sheets))
			for _, sheet := range workbook.Sheets {
				sheets = append(sheets, VegetationWorkbookSheet{cloneSiteUnitCell(sheet.Unit), sheet.Name})
			}
			return &SiteUnitSummaryWorkbookReview{source.Input.Preview, source.Scope, sheets, approval, hex.EncodeToString(hash[:]), len(workbook.Bytes)}, nil
		})
	})
}

func (s *SiteUnitSummaryWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*SiteUnitSummaryWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, siteUnitSummaryWorkbookHooks{})
}

func (s *SiteUnitSummaryWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string,
	hooks siteUnitSummaryWorkbookHooks) (*SiteUnitSummaryWorkbookOutcome, error) {
	var request siteUnitSummaryWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, request.Method); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Summary Environment workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Summary Environment workbook requires the exact reviewed approval hash")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SiteUnitSummaryWorkbookOutcome, error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		read := func() (siteUnitSummaryWorkbookSource, error) {
			return withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (siteUnitSummaryWorkbookSource, error) {
				return readSiteUnitSummaryWorkbookPublicationSource(ctx, plots, owner, tx, contextID, request.Method)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareSiteUnitSummaryWorkbook(ctx, expected.Input.Preview)
		if err != nil {
			return nil, err
		}
		approval, err := siteUnitSummaryWorkbookApproval(expected, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Summary Environment workbook source/method/saved scope/physical rows differ; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Summary Environment workbook raw source/saved scope/metadata differ; reload review")
			}
			return nil
		}
		result, publishErr := publishArtifactChecked(ctx, request.Destination, artifactPublicationFormat{
			label: "Summary Environment workbook", artifact: "XLSX", commitLabel: "workbook",
			stagePattern: ".vpro-summary-environment-workbook-*",
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
					return errors.New("staged Summary Environment workbook differs from reviewed bytes")
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
		receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "Summary Environment workbook")
		return &SiteUnitSummaryWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
