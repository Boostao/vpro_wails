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
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type vegetationWorkbookReviewRequest struct {
	Scope string `json:"scope"`
}

func (request *vegetationWorkbookReviewRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationWorkbookReviewRequest
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "scope"); err != nil {
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
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "scope", "createdDate", "approvalHash", "destination"); err != nil {
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
	Sheets         []reportcore.VegetationWorkbookSheetInternal
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
	Config      vprocore.ConfigValues
	Tables      []vprocore.ProjectMetadataTable
	Scope       string
	CreatedDate string
}

type vegetationWorkbookService struct {
	contexts *vprocore.ContextService
	now      func() time.Time
}

type vegetationWorkbookHooks struct {
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func newVegetationWorkbookService(contexts *vprocore.ContextService) (*vegetationWorkbookService, error) {
	return &vegetationWorkbookService{contexts: contexts, now: time.Now}, nil
}

func (s *vegetationWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Long Vegetation workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil ||
		s.contexts.Projects.Preferences == nil || s.contexts.Plots == nil || s.now == nil {
		return errors.New("Long Vegetation workbook owned context/configuration is unavailable")
	}
	return nil
}

func decodeVegetationWorkbookLayout(values vprocore.ConfigValues) (vegetationWorkbookLayout, error) {
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

func (s *vegetationWorkbookService) readSource(ctx context.Context, plots *vprocore.PlotService, tx *sql.Tx, contextID, scope, createdDate string) (vegetationWorkbookSource, error) {
	fail := func(err error) (vegetationWorkbookSource, error) { return vegetationWorkbookSource{}, err }
	if scope != "unlumped" {
		return fail(errors.New("Long Vegetation workbook requires explicit unlumped scope; no inferred current lump selection"))
	}
	date, err := time.Parse("2006-01-02", createdDate)
	if err != nil || date.Format("2006-01-02") != createdDate || date.Year() < 100 {
		return fail(errors.New("Long Vegetation workbook requires its exact reviewed ISO creation date"))
	}
	values, err := plots.Projects.Preferences.Snapshot()
	if err != nil {
		return fail(err)
	}
	options, err := DecodeOwnedLongVegetationOptions(s.contexts, ctx, plots, tx, values)
	if err != nil {
		return fail(err)
	}
	if err := CheckLongVegetationGrouping(s.contexts, options); err != nil {
		return fail(err)
	}
	layout, err := decodeVegetationWorkbookLayout(values)
	if err != nil {
		return fail(err)
	}
	owner := plots.Projects.Sqlite
	source, err := readLongVegetationReportSource(ctx, owner, tx, options)
	if err != nil {
		return fail(err)
	}
	report, err := longVegetationTransport(ctx, source.Report)
	if err != nil {
		return fail(err)
	}
	if layout.ReportSummary {
		table := owner.Selection.Project + "_Env"
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(errors.New("Long Vegetation summary requires original physical Env table"))
		}
		env, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", table, "", nil, "")
		if err != nil {
			return fail(err)
		}
		descriptions, err := vprocore.ReadProfileDescriptions(ctx, tx, "VLists", "USysAllSpecs")
		if err != nil {
			return fail(err)
		}
		summary, err := prepareVegetationWorkbookSummary(ctx, createdDate, env, source.Tables[1], descriptions)
		if err != nil {
			return fail(err)
		}
		if options.SelectedUnitFields != nil {
			summary, err = vegetationWorkbookSelectedUnitSummary(ctx, summary, source.Tables[5], source.Tables[6],
				source.Tables[1], options.SelectedUnitFields, report)
			if err != nil {
				return fail(err)
			}
		} else if options.NoneUnits {
			summary, err = vegetationWorkbookNoneUnitSummary(ctx, summary, env, report)
			if err != nil {
				return fail(err)
			}
		} else if report.Quality != nil {
			summary, err = vegetationWorkbookQualitySummary(ctx, summary, *report.Quality)
			if err != nil {
				return fail(err)
			}
		}
		layout.Summary = &summary
		source.Tables = append(source.Tables, env, descriptions)
	}
	preview := LongVegetationPreview{
		ContextID: contextID, ProjectPath: owner.Selection.ProjectPath, SUPath: owner.Selection.SUPath,
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Scope); err != nil {
		return nil, err
	}
	createdDate := s.now().Format("2006-01-02")
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*vegetationWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(_ *vprocore.SqliteContext, tx *sql.Tx) (*vegetationWorkbookReview, error) {
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Scope, request.CreatedDate, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Long Vegetation workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Long Vegetation workbook requires the exact reviewed approval hash")
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*vegetationWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (vegetationWorkbookSource, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (vegetationWorkbookSource, error) {
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
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "Long Vegetation workbook", Artifact: "XLSX", CommitLabel: "workbook", StagePattern: ".vpro-vegetation-workbook-*",
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
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "Long Vegetation workbook")
		return &vegetationWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
