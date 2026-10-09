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

const vegetationWorkbookFeatureEnvironment = "VPRO_LONG_VEGETATION_WORKBOOK"

type vegetationWorkbookReviewRequest struct {
	Scope string `json:"scope"`
}

func (request *vegetationWorkbookReviewRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationWorkbookReviewRequest
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "scope"); err != nil {
		return err
	}
	*request = vegetationWorkbookReviewRequest(decoded)
	return nil
}

type vegetationWorkbookExportRequest struct {
	Scope        string `json:"scope"`
	CreatedDate  string `json:"createdDate"`
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
}

func (request *vegetationWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationWorkbookExportRequest
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "scope", "createdDate", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = vegetationWorkbookExportRequest(decoded)
	return nil
}

type vegetationWorkbookReview struct {
	Preview        LongVegetationPreview
	Layout         vegetationWorkbookLayout
	Scope          string
	CreatedDate    string
	Sheets         []vegetationWorkbookSheet
	SkippedUnits   []vegetationWorkbookSkippedUnit
	ApprovalHash   string
	WorkbookSHA256 string
	Bytes          int
}

type vegetationWorkbookOutcome struct {
	Status               string
	RequestedDestination string
	Path                 string
	SHA256               string
	ErrorMessage         string
}

type vegetationWorkbookSource struct {
	Preview     LongVegetationPreview
	Layout      vegetationWorkbookLayout
	Config      configValues
	Tables      []ProjectMetadataTable
	Scope       string
	CreatedDate string
}

type vegetationWorkbookService struct {
	contexts *ContextService
	enabled  bool
	now      func() time.Time
}

type vegetationWorkbookHooks struct {
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

func newVegetationWorkbookService(contexts *ContextService, lookup func(string) (string, bool)) (*vegetationWorkbookService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(vegetationWorkbookFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &vegetationWorkbookService{contexts: contexts, enabled: enabled, now: time.Now}, nil
}

func (s *vegetationWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Long Vegetation workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation workbook is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil ||
		s.contexts.projects.preferences == nil || s.contexts.plots == nil || s.now == nil {
		return errors.New("Long Vegetation workbook owned context/configuration is unavailable")
	}
	return nil
}

func decodeVegetationWorkbookLayout(values configValues) (vegetationWorkbookLayout, error) {
	fail := func(err error) (vegetationWorkbookLayout, error) { return vegetationWorkbookLayout{}, err }
	combine, err := longVegetationConfigBool(values, "LVUseSppCodesOnly")
	if err != nil {
		return fail(err)
	}
	if combine {
		return fail(errors.New("Long Vegetation combined variants/lumping remain unavailable; retained preference was not reset"))
	}
	var layout vegetationWorkbookLayout
	for _, field := range []struct {
		key   string
		value *bool
	}{
		{"LVQuickReport", &layout.QuickReport}, {"LVSpaceBetweenGroups", &layout.SpaceBetweenGroups},
		{"LVReportSummary", &layout.ReportSummary},
	} {
		*field.value, err = longVegetationConfigBool(values, field.key)
		if err != nil {
			return fail(err)
		}
	}
	return layout, nil
}

func (s *vegetationWorkbookService) readSource(ctx context.Context, plots *PlotService, tx *sql.Tx, contextID, scope, createdDate string) (vegetationWorkbookSource, error) {
	fail := func(err error) (vegetationWorkbookSource, error) { return vegetationWorkbookSource{}, err }
	if scope != "unlumped" {
		return fail(errors.New("Long Vegetation workbook requires explicit unlumped scope; no inferred current lump selection"))
	}
	date, err := time.Parse("2006-01-02", createdDate)
	if err != nil || date.Format("2006-01-02") != createdDate || date.Year() < 100 {
		return fail(errors.New("Long Vegetation workbook requires its exact reviewed ISO creation date"))
	}
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return fail(err)
	}
	options, err := decodeLongVegetationOptions(values)
	if err != nil {
		return fail(err)
	}
	if err := s.contexts.checkLongVegetationGrouping(options); err != nil {
		return fail(err)
	}
	layout, err := decodeVegetationWorkbookLayout(values)
	if err != nil {
		return fail(err)
	}
	owner := plots.projects.sqlite
	source, err := readLongVegetationReportSource(ctx, owner, tx, options)
	if err != nil {
		return fail(err)
	}
	report, err := longVegetationTransport(ctx, source.Report)
	if err != nil {
		return fail(err)
	}
	if layout.ReportSummary {
		table := owner.selection.Project + "_Env"
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(errors.New("Long Vegetation summary requires original physical Env table"))
		}
		env, err := readSQLiteStorageRows(ctx, tx, "project", table, "", nil, "")
		if err != nil {
			return fail(err)
		}
		descriptions, err := readProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
		if err != nil {
			return fail(err)
		}
		summary, err := prepareVegetationWorkbookSummary(ctx, createdDate, env, source.Tables[1], descriptions)
		if err != nil {
			return fail(err)
		}
		if report.Quality != nil {
			summary, err = vegetationWorkbookQualitySummary(ctx, summary, *report.Quality)
			if err != nil {
				return fail(err)
			}
		}
		layout.Summary = &summary
		source.Tables = append(source.Tables, env, descriptions)
	}
	preview := LongVegetationPreview{
		ContextID: contextID, ProjectPath: owner.selection.ProjectPath, SUPath: owner.selection.SUPath,
		Settings: vegetationSettings(options), Report: report,
	}
	return vegetationWorkbookSource{preview, layout, values, source.Tables, scope, createdDate}, nil
}

func vegetationWorkbookQualitySummary(ctx context.Context, summary vegetationWorkbookSummary, quality LongVegetationQualitySelection) (vegetationWorkbookSummary, error) {
	fail := func(err error) (vegetationWorkbookSummary, error) { return vegetationWorkbookSummary{}, err }
	physical := map[string]vegetationWorkbookMembership{}
	for _, member := range summary.Memberships {
		physical[member.RowID] = member
	}
	if len(quality.Occurrences) == 0 || len(quality.Occurrences) > 1048572 {
		return fail(errors.New("Long Vegetation summary requires nonempty bounded filtered SU occurrence evidence"))
	}
	summary.Memberships = make([]vegetationWorkbookMembership, 0, len(quality.Occurrences))
	summary.SelectedPlotRows = 0
	for _, occurrence := range quality.Occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		member, ok := physical[occurrence.MembershipID]
		if !ok || !reflect.DeepEqual(member.PlotNumber, occurrence.PlotNumber) || !reflect.DeepEqual(member.SiteUnit, occurrence.SiteUnit) ||
			occurrence.PlotNumber.Storage != "text" {
			return fail(errors.New("Long Vegetation summary quality occurrence differs from original physical SU membership"))
		}
		member.Quality = &occurrence
		summary.Memberships = append(summary.Memberships, member)
		summary.SelectedPlotRows++
	}
	return summary, nil
}

func vegetationWorkbookApproval(source vegetationWorkbookSource, workbook vegetationWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		Source   vegetationWorkbookSource
		Workbook vegetationWorkbook
	}{source, workbook})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO Long Vegetation workbook approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *vegetationWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*vegetationWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request vegetationWorkbookReviewRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.Scope); err != nil {
		return nil, err
	}
	createdDate := s.now().Format("2006-01-02")
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*vegetationWorkbookReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{}, func(_ *sqliteContext, tx *sql.Tx) (*vegetationWorkbookReview, error) {
			source, err := s.readSource(ctx, plots, tx, contextID, request.Scope, createdDate)
			if err != nil {
				return nil, err
			}
			workbook, err := prepareLongVegetationWorkbook(ctx, source.Preview, source.Layout)
			if err != nil {
				return nil, err
			}
			approval, err := vegetationWorkbookApproval(source, workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			return &vegetationWorkbookReview{source.Preview, source.Layout, source.Scope, source.CreatedDate,
				workbook.Sheets, workbook.SkippedUnits, approval, hex.EncodeToString(hash[:]), len(workbook.Bytes)}, nil
		})
	})
}

func (s *vegetationWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*vegetationWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, vegetationWorkbookHooks{})
}

func (s *vegetationWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string, hooks vegetationWorkbookHooks) (*vegetationWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request vegetationWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.Scope, request.CreatedDate, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Long Vegetation workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Long Vegetation workbook requires the exact reviewed approval hash")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*vegetationWorkbookOutcome, error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		read := func() (vegetationWorkbookSource, error) {
			return withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (vegetationWorkbookSource, error) {
				return s.readSource(ctx, plots, tx, contextID, request.Scope, request.CreatedDate)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareLongVegetationWorkbook(ctx, expected.Preview, expected.Layout)
		if err != nil {
			return nil, err
		}
		approval, err := vegetationWorkbookApproval(expected, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Long Vegetation workbook source/configuration/scope/date differs; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Long Vegetation workbook raw source/configuration/metadata differs; reload review")
			}
			return nil
		}
		result, publishErr := publishArtifactChecked(ctx, request.Destination, artifactPublicationFormat{
			label: "Long Vegetation workbook", artifact: "XLSX", commitLabel: "workbook", stagePattern: ".vpro-vegetation-workbook-*",
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
					return errors.New("staged workbook differs from reviewed bytes")
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
		receipt := artifactPublicationOutcome(request.Destination, result, publishErr, "Long Vegetation workbook")
		return &vegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
