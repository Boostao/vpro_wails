package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
)

const longVegetationQualityMinimumPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES"
const longVegetationQualityMinimumSuggestionsSource = "configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite; ItemOrder then physical rowid; suggestions only"

func longVegetationQualityMinimumPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationQualityMinimumPreferencesFeatureEnvironment, lookup)
}

type LongVegetationQualityMinimumPreferenceValues struct {
	SiteMinimum string `json:"siteMinimum"`
	VegMinimum  string `json:"vegMinimum"`
	SoilMinimum string `json:"soilMinimum"`
}

func (v *LongVegetationQualityMinimumPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityMinimumPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality minimum preferences",
		"siteMinimum", "vegMinimum", "soilMinimum"); err != nil {
		return err
	}
	*v = LongVegetationQualityMinimumPreferenceValues(decoded)
	return nil
}

type LongVegetationQualityMinimumPreferencesRequest struct {
	Expected LongVegetationQualityMinimumPreferenceValues `json:"expected"`
	Proposed LongVegetationQualityMinimumPreferenceValues `json:"proposed"`
}

func (v *LongVegetationQualityMinimumPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityMinimumPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality minimum preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationQualityMinimumPreferencesRequest(decoded)
	return nil
}

type LongVegetationQualityMinimumPreferencesReview struct {
	ContextID   string                                       `json:"contextId"`
	Project     string                                       `json:"project"`
	ProjectPath string                                       `json:"projectPath"`
	SU          string                                       `json:"su"`
	SUPath      string                                       `json:"suPath"`
	Values      LongVegetationQualityMinimumPreferenceValues `json:"values"`
	Source      string                                       `json:"source"`
	Definitions ProjectMetadataTable                         `json:"definitions"`
}

type LongVegetationQualityMinimumPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationQualityMinimumPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationQualityMinimumPreferencesService(contexts *ContextService, enabled bool) *LongVegetationQualityMinimumPreferencesService {
	return &LongVegetationQualityMinimumPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationQualityMinimumPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation quality minimum preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation quality minimum preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation quality minimum preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func readLongVegetationQualityMinimumSuggestions(ctx context.Context, tx *sql.Tx) (ProjectMetadataTable, error) {
	if err := validateSIVIPhysicalSchema(ctx, tx, "VLists", "USysTableOfLists", "Long Vegetation quality minimum suggestions"); err != nil {
		return ProjectMetadataTable{}, err
	}
	table, err := readSQLiteLiteralTextRows(ctx, tx, "VLists", "USysTableOfLists", "ListName", "PlotQualitySite", "ItemOrder")
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	if !reflect.DeepEqual(table.Columns, siviParentReferenceColumns) {
		return ProjectMetadataTable{}, errors.New("Long Vegetation quality minimum suggestions require the exact ten-column imported schema")
	}
	return table, nil
}

func (s *LongVegetationQualityMinimumPreferencesService) GetLongVegetationQualityMinimumPreferences(ctx context.Context, contextID string) (*LongVegetationQualityMinimumPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationQualityMinimumPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationQualityMinimumPreferencesReview, error) {
			definitions, err := readLongVegetationQualityMinimumSuggestions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := plots.projects.preferences.readLongVegetationQualityMinimumPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityMinimumPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: values, Source: longVegetationQualityMinimumSuggestionsSource, Definitions: definitions}, nil
		})
	})
}

// Persistence receipts survive request cleanup errors.
func (s *LongVegetationQualityMinimumPreferencesService) SaveLongVegetationQualityMinimumPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityMinimumPreferencesOutcome {
	outcome := &LongVegetationQualityMinimumPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationQualityMinimumPreferencesRequest
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
			update, saveErr := plots.projects.preferences.compareAndSetLongVegetationQualityMinimumPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
