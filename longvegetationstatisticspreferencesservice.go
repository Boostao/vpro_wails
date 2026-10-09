package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longVegetationStatisticsPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_STATISTICS_PREFERENCES"

func longVegetationStatisticsPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationStatisticsPreferencesFeatureEnvironment, lookup)
}

type LongVegetationStatisticsPreferenceValues struct {
	AvgType int `json:"avgType"`
	OrderBy int `json:"orderBy"`
}

func (v *LongVegetationStatisticsPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationStatisticsPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preferences", "avgType", "orderBy"); err != nil {
		return err
	}
	*v = LongVegetationStatisticsPreferenceValues(decoded)
	return nil
}

type LongVegetationStatisticsPreferencesRequest struct {
	Expected LongVegetationStatisticsPreferenceValues `json:"expected"`
	Proposed LongVegetationStatisticsPreferenceValues `json:"proposed"`
}

func (v *LongVegetationStatisticsPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationStatisticsPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationStatisticsPreferencesRequest(decoded)
	return nil
}

type LongVegetationStatisticsPreferencesReview struct {
	ContextID   string                                   `json:"contextId"`
	Project     string                                   `json:"project"`
	ProjectPath string                                   `json:"projectPath"`
	SU          string                                   `json:"su"`
	SUPath      string                                   `json:"suPath"`
	Values      LongVegetationStatisticsPreferenceValues `json:"values"`
}

type LongVegetationStatisticsPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationStatisticsPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationStatisticsPreferencesService(contexts *ContextService, enabled bool) *LongVegetationStatisticsPreferencesService {
	return &LongVegetationStatisticsPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationStatisticsPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation statistics preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation statistics preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation statistics preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationStatisticsPreferencesService) GetLongVegetationStatisticsPreferences(ctx context.Context, contextID string) (*LongVegetationStatisticsPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationStatisticsPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationStatisticsPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationStatisticsPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationStatisticsPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: values}, nil
		})
	})
}

// Preserve irreversible persistence even when request cleanup reports an error.
func (s *LongVegetationStatisticsPreferencesService) SaveLongVegetationStatisticsPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationStatisticsPreferencesOutcome {
	outcome := &LongVegetationStatisticsPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationStatisticsPreferencesRequest
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
			update, err := plots.projects.preferences.compareAndSetLongVegetationStatisticsPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
