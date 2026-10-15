package main

import (
	"context"
	"errors"
)

type SummaryVegetationEnvironmentalNumericPreview struct {
	Vegetation    SummaryVegetationPreview                   `json:"vegetation"`
	FieldList     ProjectMetadataTable                       `json:"fieldList"`
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
	if s == nil || !s.numericEnabled {
		return nil, errors.New("Summary numeric environmental preview is disabled in this session")
	}
	return previewSummaryEnvironmentalSnapshot(ctx, s, contextID, average, display, showClass,
		func(function string, argument ProjectMetadataCell) error {
			if summaryEnvironmentalIsNumeric(function) {
				_, err := summaryEnvironmentalNumericArgument(function, argument)
				return err
			}
			_, _, err := summaryEnvironmentalSelectedFunction(function, argument)
			return err
		},
		func(source SummaryVegetationPreview, fields, env, su ProjectMetadataTable, sourcePath, envPath string) (*SummaryVegetationEnvironmentalNumericPreview, error) {
			rows, err := planSummaryVegetationEnvironmentalNumericRows(ctx, fields, summaryEnvironmentalPreviewUnits(source), env, su)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationEnvironmentalNumericPreview{Vegetation: source, FieldList: fields, Rows: rows,
				SourcePath: sourcePath, EnvSourcePath: envPath, Source: summaryEnvironmentalNumericSource}, nil
		})
}
