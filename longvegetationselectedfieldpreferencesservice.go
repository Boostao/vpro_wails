package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const longVegetationSelectedFieldPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES"
const longVegetationSelectedFieldDefinitionsSource = "configured-owned-imported-SQLite VLists.USysAllSpecs physical columns; source Value List suggestions are not mandatory membership"

func longVegetationSelectedFieldPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationSelectedFieldPreferencesFeatureEnvironment, lookup)
}

type LongVegetationSelectedFieldPreferenceValues struct {
	Mode          int    `json:"mode"`
	SelectedField string `json:"selectedField"`
}

func (v *LongVegetationSelectedFieldPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSelectedFieldPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation selected field preferences", "mode", "selectedField"); err != nil {
		return err
	}
	if !longVegetationNamingMode(decoded.Mode) {
		return errors.New("Long Vegetation selected field requires source mode 0, 1, 2 or 4")
	}
	*v = LongVegetationSelectedFieldPreferenceValues(decoded)
	return nil
}

type LongVegetationSelectedFieldPreferencesRequest struct {
	Expected LongVegetationSelectedFieldPreferenceValues `json:"expected"`
	Proposed LongVegetationSelectedFieldPreferenceValues `json:"proposed"`
}

func (v *LongVegetationSelectedFieldPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSelectedFieldPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation selected field preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationSelectedFieldPreferencesRequest(decoded)
	return nil
}

type LongVegetationSelectedFieldPreferencesReview struct {
	ContextID   string                                      `json:"contextId"`
	Project     string                                      `json:"project"`
	ProjectPath string                                      `json:"projectPath"`
	SU          string                                      `json:"su"`
	SUPath      string                                      `json:"suPath"`
	Values      LongVegetationSelectedFieldPreferenceValues `json:"values"`
	Source      string                                      `json:"source"`
	Definitions []ProjectMetadataColumn                     `json:"definitions"`
}

type LongVegetationSelectedFieldPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationSelectedFieldPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationSelectedFieldPreferencesService(contexts *ContextService, enabled bool) *LongVegetationSelectedFieldPreferencesService {
	return &LongVegetationSelectedFieldPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationSelectedFieldPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation selected field preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation selected field preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation selected field preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func readLongVegetationSelectedFieldDefinitions(ctx context.Context, tx *sql.Tx) ([]ProjectMetadataColumn, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "VLists".sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
		"USysAllSpecs").Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, errors.New("Long Vegetation selected field definitions require original physical VLists.USysAllSpecs")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, "VLists", "USysAllSpecs", "Long Vegetation selected field definitions"); err != nil {
		return nil, fmt.Errorf("Long Vegetation selected field definitions unavailable: %w", err)
	}
	return readSQLiteStorageColumns(ctx, tx, "VLists", "USysAllSpecs")
}

func (s *LongVegetationSelectedFieldPreferencesService) GetLongVegetationSelectedFieldPreferences(ctx context.Context, contextID string) (*LongVegetationSelectedFieldPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationSelectedFieldPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationSelectedFieldPreferencesReview, error) {
			definitions, err := readLongVegetationSelectedFieldDefinitions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := plots.projects.preferences.readLongVegetationSelectedFieldPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSelectedFieldPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: values, Source: longVegetationSelectedFieldDefinitionsSource, Definitions: definitions}, nil
		})
	})
}

// Persistence receipts survive request cleanup errors.
func (s *LongVegetationSelectedFieldPreferencesService) SaveLongVegetationSelectedFieldPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSelectedFieldPreferencesOutcome {
	outcome := &LongVegetationSelectedFieldPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationSelectedFieldPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*sqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, saveErr := plots.projects.preferences.compareAndSetLongVegetationSelectedFieldPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
