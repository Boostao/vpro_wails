package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const summaryVegetationEnvironmentalFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_ENVIRONMENT"
const summaryVegetationEnvironmentalNumericFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC"

type SummaryVegetationEnvironmentalPreview struct {
	Vegetation    SummaryVegetationPreview            `json:"vegetation"`
	FieldList     vprocore.ProjectMetadataTable       `json:"fieldList"`
	Rows          []SummaryVegetationEnvironmentalRow `json:"rows"`
	SourcePath    string                              `json:"sourcePath"`
	EnvSourcePath string                              `json:"envSourcePath"`
	Source        string                              `json:"source"`
}

type SummaryVegetationEnvironmentalService struct {
	report         *SummaryVegetationService
	enabled        bool
	numericEnabled bool
	snapshot       vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationEnvironmentalService(report *SummaryVegetationService, lookup func(string) (string, bool)) (*SummaryVegetationEnvironmentalService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := vprocore.SiviFeature(summaryVegetationEnvironmentalFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	numericEnabled, err := vprocore.SiviFeature(summaryVegetationEnvironmentalNumericFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationEnvironmentalService{report: report, enabled: enabled, numericEnabled: numericEnabled}, nil
}

func (s *SummaryVegetationEnvironmentalService) PreviewEnvironmental(ctx context.Context, contextID string,
	average, display, showClass int) (*SummaryVegetationEnvironmentalPreview, error) {
	return previewSummaryEnvironmentalSnapshot(ctx, s, contextID, average, display, showClass,
		func(function string, argument vprocore.ProjectMetadataCell) error {
			_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
			return err
		},
		func(source SummaryVegetationPreview, fields, env, su vprocore.ProjectMetadataTable, sourcePath, envPath string) (*SummaryVegetationEnvironmentalPreview, error) {
			rows, err := planSummaryVegetationEnvironmentalRows(ctx, fields, summaryEnvironmentalPreviewUnits(source), env, su)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalPreview{Vegetation: source, FieldList: fields, Rows: rows,
				SourcePath: sourcePath, EnvSourcePath: envPath,
				Source: "one owned SQLite snapshot: raw Summary units, physical SU counts, selected field-list and needed Env; literal scope adaptation; no formatted workbook"}, nil
		})
}

func summaryEnvironmentalPreviewUnits(source SummaryVegetationPreview) []summaryVegetationUnit {
	units := make([]summaryVegetationUnit, 0, len(source.Units))
	for _, unit := range source.Units {
		units = append(units, summaryVegetationUnit{Code: unit.Code, NumPlots: unit.NumPlots})
	}
	return units
}

func previewSummaryEnvironmentalSnapshot[R any](ctx context.Context, s *SummaryVegetationEnvironmentalService,
	contextID string, average, display, showClass int,
	validate func(string, vprocore.ProjectMetadataCell) error,
	build func(SummaryVegetationPreview, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, string, string) (R, error)) (R, error) {
	return previewSummaryEnvironmentalRawSnapshot(ctx, s, contextID, average, display, showClass, validate,
		func(source summaryVegetationSnapshotPreview, fields, env vprocore.ProjectMetadataTable, sourcePath, envPath, masterPath string) (R, error) {
			return build(source.Preview, fields, env, source.SU, sourcePath, envPath)
		})
}

func previewSummaryEnvironmentalRawSnapshot[R any](ctx context.Context, s *SummaryVegetationEnvironmentalService,
	contextID string, average, display, showClass int,
	validate func(string, vprocore.ProjectMetadataCell) error,
	build func(summaryVegetationSnapshotPreview, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, string, string, string) (R, error)) (R, error) {
	var zero R
	if ctx == nil {
		return zero, errors.New("Summary environmental preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if s == nil || !s.enabled {
		return zero, errors.New("Summary environmental preview is disabled in this session")
	}
	if s.report == nil || !s.report.Enabled {
		return zero, errors.New("Summary environmental preview requires enabled Summary Vegetation")
	}
	contexts := s.report.contexts
	if contexts == nil || contexts.Projects == nil || contexts.Projects.Sqlite == nil || contexts.Plots == nil {
		return zero, errors.New("Summary environmental preview context service unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return zero, err
	}
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return zero, err
	}
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (R, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (R, error) {
			source, err := s.report.readSnapshot(ctx, contextID, plots, owner, tx, average, display, showClass, 11)
			if err != nil {
				return zero, err
			}
			physical, err := readSummaryEnvironmentalTables(ctx, owner, tx, validate)
			if err != nil {
				return zero, err
			}
			return build(source, physical.FieldList, physical.Env, owner.Attachments["VPro64"], owner.Attachments["project"], owner.Attachments["VLists"])
		})
	})
}

type summaryEnvironmentalPhysicalInput struct {
	FieldList vprocore.ProjectMetadataTable
	Env       vprocore.ProjectMetadataTable
}

func readSummaryEnvironmentalTables(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	validate func(string, vprocore.ProjectMetadataCell) error) (summaryEnvironmentalPhysicalInput, error) {
	fail := func(err error) (summaryEnvironmentalPhysicalInput, error) {
		return summaryEnvironmentalPhysicalInput{}, err
	}
	fields, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return fail(err)
	}
	if err := validateSummaryVegetationFieldListWitness(ctx, fields); err != nil {
		return fail(err)
	}
	index, err := indexSummaryVegetationFieldList(ctx, fields)
	if err != nil {
		return fail(err)
	}
	selected := make(map[string]bool, len(index.Selected))
	for _, id := range index.Selected {
		selected[id] = true
	}
	needsEnv := false
	for _, row := range fields.Rows {
		if !selected[row.RowID] {
			continue
		}
		function, err := vprocore.VegetationReportIdentity(row.Cells[5])
		if err != nil {
			return fail(fmt.Errorf("Summary environmental selected row %s FunctionName: %w", row.RowID, err))
		}
		if function != nil {
			if err := validate(*function, row.Cells[6]); err != nil {
				return fail(fmt.Errorf("Summary environmental selected row %s: %w", row.RowID, err))
			}
			if *function != "SVnPlots" && *function != "SVLifeformCover" {
				needsEnv = true
			}
		}
	}
	var env vprocore.ProjectMetadataTable
	if needsEnv {
		name := owner.Selection.Project + "_Env"
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(fmt.Errorf("Summary environmental preview requires original physical project.%s table", name))
		}
		if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "project", name, "Summary environmental preview"); err != nil {
			return fail(err)
		}
		env, err = vprocore.ReadSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
		if err != nil {
			return fail(fmt.Errorf("Summary environmental physical Env read: %w", err))
		}
	}
	return summaryEnvironmentalPhysicalInput{FieldList: fields, Env: env}, nil
}
