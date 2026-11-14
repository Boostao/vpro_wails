package vegreport

import (
	"context"
	"errors"
	"fmt"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

func summaryEnvironmentalLifeformArgument(argument vprocore.ProjectMetadataCell) (string, error) {
	if _, err := vprocore.MetadataCellValue(argument); err != nil {
		return "", fmt.Errorf("lifeform environmental FieldNumber: %w", err)
	}
	if argument.Storage != "text" || argument.Text == nil {
		return "", fmt.Errorf("SVLifeformCover requires an original literal TEXT FieldNumber1 through13")
	}
	switch *argument.Text {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13":
		return *argument.Text, nil
	default:
		return "", fmt.Errorf("SVLifeformCover has an unavailable literal FieldNumber %q", *argument.Text)
	}
}

func planSummaryVegetationEnvironmentalLifeformRows(ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su, quick, master vprocore.ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalLifeformRowsWithMasterInteger(ctx, fieldList, units, env, su, quick, master,
		vprocore.VegetationLifeformInteger)
}

func planSummaryEnvironmentalLifeformRowsWithMasterInteger(ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su, quick, master vprocore.ProjectMetadataTable,
	readInteger func(vprocore.ProjectMetadataCell) (*int, error)) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		func(function string, argument vprocore.ProjectMetadataCell) error {
			if function == "SVLifeformCover" {
				_, err := summaryEnvironmentalLifeformArgument(argument)
				return err
			}
			return validateSummaryEnvironmentalNumericSelectedFunction(function, argument)
		},
		func(function string, argument vprocore.ProjectMetadataCell, unit string) (SummaryVegetationEnvironmentalValue, error) {
			if function == "SVLifeformCover" {
				form, err := summaryEnvironmentalLifeformArgument(argument)
				if err != nil {
					return SummaryVegetationEnvironmentalValue{}, err
				}
				cell, err := prepareSummaryEnvironmentalLifeformWithMasterInteger(ctx, form, &unit, quick, master, su, readInteger)
				if err != nil {
					return SummaryVegetationEnvironmentalValue{}, err
				}
				return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
			}
			return prepareSummaryEnvironmentalNumericSelectedValue(ctx, function, argument, unit, env, su)
		},
		func(field vprocore.ProjectMetadataRow, label int, values []SummaryVegetationEnvironmentalValue) SummaryVegetationEnvironmentalNumericRow {
			return SummaryVegetationEnvironmentalNumericRow{Field: field, LabelColumn: label, Values: values}
		})
}

type SummaryVegetationEnvironmentalLifeformPreview struct {
	Vegetation       SummaryVegetationPreview                   `json:"vegetation"`
	FieldList        vprocore.ProjectMetadataTable              `json:"fieldList"`
	Rows             []SummaryVegetationEnvironmentalNumericRow `json:"rows"`
	SourcePath       string                                     `json:"sourcePath"`
	EnvSourcePath    string                                     `json:"envSourcePath"`
	MasterSourcePath string                                     `json:"masterSourcePath"`
	Source           string                                     `json:"source"`
}

type SummaryVegetationEnvironmentalLifeformService struct {
	environment *SummaryVegetationEnvironmentalService
}

const summaryEnvironmentalLifeformSource = "one owned SQLite snapshot: raw Summary units/Veg/master, physical SU counts, selected field-list and needed Env; source-qualified QuickVeg/lifeform arguments1 through13; literal scope adaptation; explicit numeric Empty; no formatted workbook"

func NewSummaryVegetationEnvironmentalLifeformService(environment *SummaryVegetationEnvironmentalService) (*SummaryVegetationEnvironmentalLifeformService, error) {
	return &SummaryVegetationEnvironmentalLifeformService{environment: environment}, nil
}

func (s *SummaryVegetationEnvironmentalLifeformService) PreviewEnvironmentalLifeform(ctx context.Context,
	contextID string, average, display, showClass int) (*SummaryVegetationEnvironmentalLifeformPreview, error) {
	if ctx == nil {
		return nil, errors.New("Summary lifeform environmental preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.environment == nil {
		return nil, errors.New("Summary lifeform environmental preview service is unavailable")
	}
	return previewSummaryEnvironmentalRawSnapshot(ctx, s.environment, contextID, average, display, showClass,
		validateSummaryEnvironmentalLifeformSelectedFunction,
		func(source summaryVegetationSnapshotPreview, fields, env vprocore.ProjectMetadataTable,
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

func validateSummaryEnvironmentalLifeformSelectedFunction(function string, argument vprocore.ProjectMetadataCell) error {
	if function == "SVLifeformCover" {
		_, err := summaryEnvironmentalLifeformArgument(argument)
		return err
	}
	return validateSummaryEnvironmentalNumericSelectedFunction(function, argument)
}

func planSummaryEnvironmentalSnapshotRows(ctx context.Context, source summaryVegetationSnapshotPreview,
	fields, env vprocore.ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
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
	var quick vprocore.ProjectMetadataTable
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

type SummaryVegetationEnvironmentalNumericRow struct {
	Field       vprocore.ProjectMetadataRow           `json:"field"`
	LabelColumn int                                   `json:"labelColumn"`
	Values      []SummaryVegetationEnvironmentalValue `json:"values"`
}

func summaryEnvironmentalIsNumeric(function string) bool {
	switch function {
	case "SVSeepage", "SVRootingDepth", "SVStandAge", "SVElevation", "SVSlope", "SVSubstrate", "SVStrataCover":
		return true
	default:
		return false
	}
}

func validateSummaryEnvironmentalNumericSelectedFunction(function string, argument vprocore.ProjectMetadataCell) error {
	if summaryEnvironmentalIsNumeric(function) {
		_, err := summaryEnvironmentalNumericArgument(function, argument)
		return err
	}
	_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
	return err
}

func prepareSummaryEnvironmentalNumericSelectedValue(ctx context.Context, function string,
	argument vprocore.ProjectMetadataCell, unit string, env, su vprocore.ProjectMetadataTable) (SummaryVegetationEnvironmentalValue, error) {
	if summaryEnvironmentalIsNumeric(function) {
		return prepareSummaryVegetationEnvironmentalNumericValue(ctx, function, argument, &unit, env, su)
	}
	cell, err := prepareSummaryEnvironmentalSelectedCell(ctx, function, argument, &unit, env, su)
	if err != nil {
		return SummaryVegetationEnvironmentalValue{}, err
	}
	return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
}

func planSummaryVegetationEnvironmentalNumericRows(ctx context.Context, fieldList vprocore.ProjectMetadataTable,
	units []summaryVegetationUnit, env, su vprocore.ProjectMetadataTable) ([]SummaryVegetationEnvironmentalNumericRow, error) {
	return planSummaryEnvironmentalSelectedRows(ctx, fieldList, units, env, su,
		validateSummaryEnvironmentalNumericSelectedFunction,
		func(function string, argument vprocore.ProjectMetadataCell, unit string) (SummaryVegetationEnvironmentalValue, error) {
			return prepareSummaryEnvironmentalNumericSelectedValue(ctx, function, argument, unit, env, su)
		},
		func(field vprocore.ProjectMetadataRow, label int, values []SummaryVegetationEnvironmentalValue) SummaryVegetationEnvironmentalNumericRow {
			return SummaryVegetationEnvironmentalNumericRow{Field: field, LabelColumn: label, Values: values}
		})
}

type SummaryVegetationEnvironmentalNumericPreview struct {
	Vegetation    SummaryVegetationPreview                   `json:"vegetation"`
	FieldList     vprocore.ProjectMetadataTable              `json:"fieldList"`
	Rows          []SummaryVegetationEnvironmentalNumericRow `json:"rows"`
	SourcePath    string                                     `json:"sourcePath"`
	EnvSourcePath string                                     `json:"envSourcePath"`
	Source        string                                     `json:"source"`
}

const summaryEnvironmentalNumericSource = "one owned SQLite snapshot: raw Summary units, physical SU counts, selected field-list and needed Env; literal scope adaptation; explicit numeric Empty; no formatted workbook"

func (s *SummaryVegetationEnvironmentalService) PreviewEnvironmentalNumeric(ctx context.Context, contextID string,
	average, display, showClass int) (*SummaryVegetationEnvironmentalNumericPreview, error) {
	if ctx == nil {
		return nil, errors.New("Summary numeric environmental preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("Summary numeric environmental preview service is unavailable")
	}
	return previewSummaryEnvironmentalSnapshot(ctx, s, contextID, average, display, showClass,
		func(function string, argument vprocore.ProjectMetadataCell) error {
			if summaryEnvironmentalIsNumeric(function) {
				_, err := summaryEnvironmentalNumericArgument(function, argument)
				return err
			}
			_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
			return err
		},
		func(source SummaryVegetationPreview, fields, env, su vprocore.ProjectMetadataTable, sourcePath, envPath string) (*SummaryVegetationEnvironmentalNumericPreview, error) {
			rows, err := planSummaryVegetationEnvironmentalNumericRows(ctx, fields, summaryEnvironmentalPreviewUnits(source), env, su)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalNumericPreview{Vegetation: source, FieldList: fields, Rows: rows,
				SourcePath: sourcePath, EnvSourcePath: envPath, Source: summaryEnvironmentalNumericSource}, nil
		})
}

type SummaryVegetationEnvironmentalValue struct {
	Kind string                        `json:"kind"`
	Cell *vprocore.ProjectMetadataCell `json:"cell"`
}

func summaryEnvironmentalNumericArgument(function string, argument vprocore.ProjectMetadataCell) (string, error) {
	if _, err := vprocore.MetadataCellValue(argument); err != nil {
		return "", fmt.Errorf("numeric environmental FieldNumber: %w", err)
	}
	switch function {
	case "SVSubstrate", "SVStrataCover":
		text, err := vprocore.VegetationReportIdentity(argument)
		if err != nil || text == nil {
			return "", fmt.Errorf("function %q requires a calibrated literal TEXT FieldNumber suffix", function)
		}
		switch function {
		case "SVSubstrate":
			switch *text {
			case "DecWood", "BedRock", "Rocks", "MineralSoil", "OrganicMatter", "Water":
				return *text, nil
			}
		case "SVStrataCover":
			switch *text {
			case "Tree", "Shrub", "Herb", "Moss":
				return *text, nil
			}
		}
		return "", fmt.Errorf("function %q has an unavailable literal FieldNumber suffix %q", function, *text)
	case "SVSeepage", "SVRootingDepth", "SVStandAge", "SVElevation", "SVSlope":
		if argument.Storage != "null" {
			return "", fmt.Errorf("function %q requires an original NULL FieldNumber", function)
		}
		return "", nil
	default:
		return "", fmt.Errorf("function %q has no calibrated numeric environmental primitive", function)
	}
}

func prepareSummaryVegetationEnvironmentalNumericValue(ctx context.Context, function string,
	argument vprocore.ProjectMetadataCell, siteUnit *string, env, su vprocore.ProjectMetadataTable) (SummaryVegetationEnvironmentalValue, error) {
	fail := func(err error) (SummaryVegetationEnvironmentalValue, error) {
		return SummaryVegetationEnvironmentalValue{}, fmt.Errorf("numeric environmental value %s: %w", function, err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	suffix, err := summaryEnvironmentalNumericArgument(function, argument)
	if err != nil {
		return fail(err)
	}
	var result summaryVegetationEnvironmentalDepthResult
	switch function {
	case "SVStandAge", "SVElevation":
		cell, err := prepareSummaryVegetationEnvironmentalAgeElevation(ctx, function, siteUnit, env, su)
		if err != nil {
			return fail(err)
		}
		return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &cell}, nil
	case "SVSeepage", "SVRootingDepth":
		result, err = prepareSummaryVegetationEnvironmentalDepth(ctx, function, siteUnit, env, su)
	case "SVSlope":
		result, err = prepareSummaryVegetationEnvironmentalSlope(ctx, siteUnit, env, su)
	case "SVSubstrate", "SVStrataCover":
		result, err = prepareSummaryVegetationEnvironmentalCover(ctx, function, suffix, siteUnit, env, su)
	}
	if err != nil {
		return fail(err)
	}
	if result.Empty {
		return SummaryVegetationEnvironmentalValue{Kind: "empty"}, nil
	}
	return SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &vprocore.ProjectMetadataCell{Storage: "text", Text: &result.Text}}, nil
}
