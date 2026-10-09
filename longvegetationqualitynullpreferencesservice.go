package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longVegetationQualityNullPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_QUALITY_NULL_PREFERENCES"

func longVegetationQualityNullPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationQualityNullPreferencesFeatureEnvironment, lookup)
}

type LongVegetationQualityNullPreferenceValues struct {
	SiteIncludeNulls bool `json:"siteIncludeNulls"`
	VegIncludeNulls  bool `json:"vegIncludeNulls"`
	SoilIncludeNulls bool `json:"soilIncludeNulls"`
}

func (v *LongVegetationQualityNullPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preferences",
		"siteIncludeNulls", "vegIncludeNulls", "soilIncludeNulls"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferenceValues(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesRequest struct {
	Expected LongVegetationQualityNullPreferenceValues `json:"expected"`
	Proposed LongVegetationQualityNullPreferenceValues `json:"proposed"`
}

func (v *LongVegetationQualityNullPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferencesRequest(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesReview struct {
	ContextID   string                                    `json:"contextId"`
	Project     string                                    `json:"project"`
	ProjectPath string                                    `json:"projectPath"`
	SU          string                                    `json:"su"`
	SUPath      string                                    `json:"suPath"`
	Values      LongVegetationQualityNullPreferenceValues `json:"values"`
}

type LongVegetationQualityNullPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationQualityNullPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationQualityNullPreferencesService(contexts *ContextService, enabled bool) *LongVegetationQualityNullPreferencesService {
	return &LongVegetationQualityNullPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationQualityNullPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation quality Include Nulls preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation quality Include Nulls preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation quality Include Nulls preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationQualityNullPreferencesService) GetLongVegetationQualityNullPreferences(ctx context.Context, contextID string) (*LongVegetationQualityNullPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationQualityNullPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationQualityNullPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationQualityNullPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityNullPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: values}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *LongVegetationQualityNullPreferencesService) SaveLongVegetationQualityNullPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityNullPreferencesOutcome {
	outcome := &LongVegetationQualityNullPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationQualityNullPreferencesRequest
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
			update, err := plots.projects.preferences.compareAndSetLongVegetationQualityNullPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
