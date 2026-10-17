package main

import (
	"context"
	"errors"
	"os"
)

const summaryVegetationEnvironmentalLifeformFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM"

type SummaryVegetationEnvironmentalLifeformPreview struct {
	Vegetation       SummaryVegetationPreview                   `json:"vegetation"`
	FieldList        ProjectMetadataTable                       `json:"fieldList"`
	Rows             []SummaryVegetationEnvironmentalNumericRow `json:"rows"`
	SourcePath       string                                     `json:"sourcePath"`
	EnvSourcePath    string                                     `json:"envSourcePath"`
	MasterSourcePath string                                     `json:"masterSourcePath"`
	Source           string                                     `json:"source"`
}

type SummaryVegetationEnvironmentalLifeformService struct {
	environment *SummaryVegetationEnvironmentalService
	enabled     bool
}

const summaryEnvironmentalLifeformSource = "one owned SQLite snapshot: raw Summary units/Veg/master, physical SU counts, selected field-list and needed Env; source-qualified QuickVeg/lifeform arguments1 through13; literal scope adaptation; explicit numeric Empty; no formatted workbook"

func NewSummaryVegetationEnvironmentalLifeformService(environment *SummaryVegetationEnvironmentalService,
	lookup func(string) (string, bool)) (*SummaryVegetationEnvironmentalLifeformService, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	enabled, err := siviFeature(summaryVegetationEnvironmentalLifeformFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationEnvironmentalLifeformService{environment: environment, enabled: enabled}, nil
}

func (s *SummaryVegetationEnvironmentalLifeformService) PreviewEnvironmentalLifeform(ctx context.Context,
	contextID string, average, display, showClass int) (*SummaryVegetationEnvironmentalLifeformPreview, error) {
	if ctx == nil {
		return nil, errors.New("Summary lifeform environmental preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || !s.enabled {
		return nil, errors.New("Summary lifeform environmental preview is disabled in this session")
	}
	if s.environment == nil || !s.environment.numericEnabled {
		return nil, errors.New("Summary lifeform environmental preview requires enabled numeric environmental inspection")
	}
	return previewSummaryEnvironmentalRawSnapshot(ctx, s.environment, contextID, average, display, showClass,
		validateSummaryEnvironmentalLifeformSelectedFunction,
		func(source summaryVegetationSnapshotPreview, fields, env ProjectMetadataTable,
			sourcePath, envPath, masterPath string) (*SummaryVegetationEnvironmentalLifeformPreview, error) {
			rows, err := planSummaryEnvironmentalSnapshotRows(ctx, source, fields, env)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalLifeformPreview{
				Vegetation: source.Preview, FieldList: fields, Rows: rows,
				SourcePath: sourcePath, EnvSourcePath: envPath, MasterSourcePath: masterPath,
				Source: summaryEnvironmentalLifeformSource,
			}, nil
		})
}

func validateSummaryEnvironmentalLifeformSelectedFunction(function string, argument ProjectMetadataCell) error {
	if function == "SVLifeformCover" {
		_, err := summaryEnvironmentalLifeformArgument(argument)
		return err
	}
	return validateSummaryEnvironmentalNumericSelectedFunction(function, argument)
}

func planSummaryEnvironmentalSnapshotRows(ctx context.Context, source summaryVegetationSnapshotPreview,
	fields, env ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	index, err := indexSummaryVegetationFieldList(ctx, fields)
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool, len(index.Selected))
	for _, id := range index.Selected {
		selected[id] = true
	}
	needsQuick := false
	for _, row := range fields.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if selected[row.RowID] && row.Cells[5].Text != nil && *row.Cells[5].Text == "SVLifeformCover" {
			needsQuick = true
		}
	}
	var quick ProjectMetadataTable
	if needsQuick {
		quick, err = prepareSummaryEnvironmentalQuickVeg(ctx, source.Preview.Project, source.Preview.SU, source.Veg, source.SU)
		if err != nil {
			return nil, err
		}
	}
	if source.Preview.References != nil {
		// Derived attributes have NULL Lifeform; only retained physical master rows can contribute.
		master, err := summaryEffectiveReferenceEnvironmentMaster(ctx, source.Master)
		if err != nil {
			return nil, err
		}
		return planSummaryEnvironmentalLifeformRowsWithMasterInteger(ctx, fields,
			summaryEnvironmentalPreviewUnits(source.Preview), env, source.SU, quick, master, summaryVegetationMasterLongInteger)
	}
	return planSummaryVegetationEnvironmentalLifeformRows(ctx, fields,
		summaryEnvironmentalPreviewUnits(source.Preview), env, source.SU, quick, source.Master)
}
