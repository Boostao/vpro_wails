package vegreport

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

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type vegetationProjectWorkbook struct {
	Preview  LongVegetationProjectPreview
	Workbook vegetationWorkbook
}

type vegetationProjectWorkbookEvidence struct {
	Scope   LongVegetationReportScope
	Summary *vegetationProjectWorkbookSummary
}

func prepareLongVegetationProjectWorkbook(ctx context.Context, source vegetationLayerReport,
	scope LongVegetationReportScope, settings LongVegetationSettings, layout vegetationWorkbookLayout) (vegetationProjectWorkbook, error) {
	return prepareLongVegetationProjectWorkbookWithSummary(ctx, source, scope, settings, layout, nil)
}

func prepareLongVegetationProjectWorkbookWithSummary(ctx context.Context, source vegetationLayerReport,
	scope LongVegetationReportScope, settings LongVegetationSettings, layout vegetationWorkbookLayout,
	summary *vegetationProjectWorkbookSummary) (vegetationProjectWorkbook, error) {
	fail := func(err error) (vegetationProjectWorkbook, error) { return vegetationProjectWorkbook{}, err }
	if ctx == nil {
		return fail(errors.New("project workbook requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if scope.ContextID == "" || scope.Project == "" || scope.ProjectPath == "" || scope.SU == "" ||
		(scope.SU == "None") != (scope.SUPath == "") ||
		source.Project != scope.Project || source.SU != scope.SU || !scope.BuildCurrent || scope.Build == nil {
		return fail(errors.New("project workbook requires complete actual owner paths and current original Build authority"))
	}
	for _, value := range []string{scope.ContextID, scope.Project, scope.ProjectPath, scope.SU, scope.SUPath} {
		if err := reportcore.EnvironmentWorkbookText(value); err != nil {
			return fail(err)
		}
	}
	if _, err := selectedUnitRevision(scope.Revision); err != nil {
		return fail(err)
	}
	hash, err := hex.DecodeString(scope.Build.ApprovalHash)
	if err != nil || len(hash) != 32 || hex.EncodeToString(hash) != scope.Build.ApprovalHash {
		return fail(errors.New("project workbook requires its exact lowercase SHA256 Build approval"))
	}
	switch scope.SourceKind {
	case vprocore.VegetationProjectFieldsScope:
		if len(scope.Fields) == 0 || settings.UnitGrouping != "selected-fields" || !reflect.DeepEqual(settings.UnitFields, scope.Fields) {
			return fail(errors.New("project workbook settings lost their explicit selected field order"))
		}
	case vprocore.VegetationProjectAllScope:
		if scope.SU != "None" || settings.UnitGrouping != "all-plots" || settings.UnitFields != nil {
			return fail(errors.New("project All plots workbook requires actual None ownership without active field settings"))
		}
	default:
		return fail(errors.New("project workbook requires an explicit original project source"))
	}
	if layout.Summary != nil || layout.ReportSummary != (summary != nil) {
		return fail(errors.New("project workbook summary requires explicit distinct original project evidence; selected-list membership summaries are unavailable"))
	}
	report, err := longVegetationProjectTransport(ctx, source, scope.SourceKind)
	if err != nil {
		return fail(err)
	}
	if (settings.Quality != nil) != (report.Quality != nil) {
		return fail(errors.New("project workbook settings and original-SU quality provenance disagree"))
	}
	inputs := make([]vegetationProjectUnitInput, len(scope.Build.ProjectTuples))
	for i, row := range scope.Build.ProjectTuples {
		inputs[i] = vegetationProjectUnitInput{row.EnvRowID, row.AdminRowID, row.PlotNumber, row.SiteUnit}
	}
	validated, err := validateProjectUnitInputs(ctx, inputs)
	if err != nil || len(validated) == 0 {
		return fail(errors.Join(errors.New("project workbook requires complete original Build tuples"), err))
	}
	if (validated[0].AdminRowID != nil) != (scope.SourceKind == vprocore.VegetationProjectFieldsScope) {
		return fail(errors.New("project workbook Build tuples contradict original source kind"))
	}
	plots := map[string]bool{}
	if report.Quality != nil {
		for _, occurrence := range report.Quality.Occurrences {
			plots[reportcore.VegetationTextKey(occurrence.PlotNumber)] = true
		}
	}
	expected := make([]vprocore.LongVegetationProjectUnit, 0, len(validated))
	full := make([]vprocore.LongVegetationProjectUnit, len(validated))
	for i, row := range validated {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		full[i] = vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: row.AdminRowID, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit}
		if report.Quality == nil || plots[reportcore.VegetationTextKey(row.PlotNumber)] {
			expected = append(expected, full[i])
		}
	}
	if !reflect.DeepEqual(expected, report.ProjectUnits) {
		return fail(errors.New("project workbook output differs from complete Build tuples or exact original-SU qualified-plot subset"))
	}
	if summary != nil && (summary.SourceKind != scope.SourceKind || !reflect.DeepEqual(summary.ProjectTuples, report.ProjectUnits)) {
		return fail(errors.New("project workbook summary differs from its current original report tuples"))
	}
	scope.Fields = append([]string{}, scope.Fields...)
	scope.Build = &vprocore.LongVegetationProjectBuild{ApprovalHash: scope.Build.ApprovalHash, ProjectTuples: full}
	settings.UnitFields = append([]string(nil), settings.UnitFields...)
	settings.Quality = sqlutil.CloneCatalogueValue(settings.Quality)
	preview := LongVegetationPreview{ContextID: scope.ContextID, ProjectPath: scope.ProjectPath, SUPath: scope.SUPath,
		Settings: settings, Report: report}
	workbook, err := renderLongVegetationWorkbook(ctx, preview, layout, &vegetationProjectWorkbookEvidence{Scope: scope, Summary: summary})
	if err != nil {
		return fail(err)
	}
	return vegetationProjectWorkbook{Preview: LongVegetationProjectPreview{Scope: scope, Preview: preview}, Workbook: workbook}, nil
}

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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "project workbook review", "revision", "buildApprovalHash", "scope"); err != nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "project workbook publication",
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(decoded.Destination); err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(decoded.Destination), ".xlsx") {
		return errors.New("project workbook requires an explicit .xlsx destination")
	}
	*request = vegetationProjectWorkbookExportRequest(decoded)
	return nil
}

type VegetationProjectWorkbookSummary struct {
	CreatedDate        string                               `json:"createdDate"`
	EnvironmentRows    int                                  `json:"environmentRows"`
	SourcePlotRows     int                                  `json:"sourcePlotRows"`
	SourceKind         string                               `json:"sourceKind"`
	ProjectTuples      []vprocore.LongVegetationProjectUnit `json:"projectTuples"`
	SpeciesVersion     vprocore.ProjectMetadataCell         `json:"speciesVersion"`
	VersionStatus      string                               `json:"versionStatus"`
	VersionDefinitions vprocore.ProjectMetadataTable        `json:"versionDefinitions"`
}

type LongVegetationProjectWorkbookReview struct {
	Scope          LongVegetationReportScope            `json:"scope"`
	Preview        LongVegetationPreview                `json:"preview"`
	Options        VegetationWorkbookOptions            `json:"options"`
	Summary        *VegetationProjectWorkbookSummary    `json:"summary"`
	ReportScope    string                               `json:"reportScope"`
	CreatedDate    string                               `json:"createdDate"`
	Sheets         []reportcore.VegetationWorkbookSheet `json:"sheets"`
	SkippedUnits   []VegetationWorkbookSkippedUnit      `json:"skippedUnits"`
	ApprovalHash   string                               `json:"approvalHash"`
	WorkbookSHA256 string                               `json:"workbookSHA256"`
	Bytes          int                                  `json:"bytes"`
}

type vegetationProjectWorkbookSource struct {
	Scope       LongVegetationReportScope
	Report      vegetationLayerReport
	Settings    LongVegetationSettings
	Layout      vegetationWorkbookLayout
	Summary     *vegetationProjectWorkbookSummary
	Config      vprocore.ConfigValues
	Tables      []vprocore.ProjectMetadataTable
	ReportScope string
	CreatedDate string
}

type LongVegetationProjectWorkbookService struct {
	scope    *LongVegetationReportScopeService
	now      func() time.Time
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationProjectWorkbookService(contexts *vprocore.ContextService) *LongVegetationProjectWorkbookService {
	return &LongVegetationProjectWorkbookService{
		scope: NewLongVegetationReportScopeService(contexts), now: time.Now,
	}
}

func (s *LongVegetationProjectWorkbookService) require(ctx context.Context, contextID string) error {
	if s == nil || s.scope == nil {
		return errors.New("project workbook service is unavailable")
	}
	return s.scope.require(ctx, contextID)
}

func (s *LongVegetationProjectWorkbookService) readSource(ctx context.Context, plots *vprocore.PlotService, tx *sql.Tx,
	contextID string, request vegetationProjectWorkbookReadRequest, createdDate string) (vegetationProjectWorkbookSource, error) {
	fail := func(err error) (vegetationProjectWorkbookSource, error) {
		return vegetationProjectWorkbookSource{}, err
	}
	values, err := plots.Projects.Preferences.Snapshot()
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
	if owned.Scope.SourceKind == vprocore.VegetationProjectAllScope {
		fields = nil
	}
	source, err := readLongVegetationProjectSource(ctx, plots.Projects.Sqlite, tx, fields, options)
	if err != nil {
		return fail(err)
	}
	result := vegetationProjectWorkbookSource{Scope: owned.Scope, Report: source.Report, Settings: owned.Options.Settings,
		Layout: layout, Config: values, Tables: source.Tables, ReportScope: request.Scope, CreatedDate: createdDate}
	if layout.ReportSummary {
		descriptions, err := vprocore.ReadProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
		if err != nil {
			return fail(err)
		}
		var admin vprocore.ProjectMetadataTable
		if owned.Scope.SourceKind == vprocore.VegetationProjectFieldsScope {
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
	return vprocore.WithContextPlotRequest(ctx, s.scope.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationProjectWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(_ *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationProjectWorkbookReview, error) {
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
				Sheets: []reportcore.VegetationWorkbookSheet{}, SkippedUnits: []VegetationWorkbookSkippedUnit{}}
			if summary := source.Summary; summary != nil {
				result.Summary = &VegetationProjectWorkbookSummary{summary.Header.CreatedDate, summary.Header.EnvironmentRows,
					summary.SourcePlotRows, summary.SourceKind, summary.ProjectTuples, summary.Header.SpeciesVersion,
					summary.Header.VersionStatus, summary.Header.VersionDefinitions}
			}
			for _, sheet := range prepared.Workbook.Sheets {
				result.Sheets = append(result.Sheets, reportcore.VegetationWorkbookSheet{Unit: sheet.Unit, Name: sheet.Name})
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
	return vprocore.WithContextPlotRequest(ctx, s.scope.contexts, contextID, func(plots *vprocore.PlotService) (*VegetationWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (vegetationProjectWorkbookSource, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (vegetationProjectWorkbookSource, error) {
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
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "project Long Vegetation workbook", Artifact: "XLSX", CommitLabel: "workbook", StagePattern: ".vpro-project-vegetation-workbook-*",
			Encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(prepared.Workbook.Bytes), nil
			},
			Validate: func(ctx context.Context, actual []byte) error {
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
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "project Long Vegetation workbook")
		return &VegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
