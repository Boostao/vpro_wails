package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longVegetationPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_PREFERENCES"

func longVegetationPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationPreferencesFeatureEnvironment, lookup)
}

type LongVegetationPreferenceValues struct {
	Title string `json:"title"`
}

func (v *LongVegetationPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation preferences", "title"); err != nil {
		return err
	}
	*v = LongVegetationPreferenceValues(decoded)
	return nil
}

type LongVegetationPreferencesRequest struct {
	Expected LongVegetationPreferenceValues `json:"expected"`
	Proposed LongVegetationPreferenceValues `json:"proposed"`
}

func (v *LongVegetationPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationPreferencesRequest(decoded)
	return nil
}

type LongVegetationPreferencesReview struct {
	ContextID   string                         `json:"contextId"`
	Project     string                         `json:"project"`
	ProjectPath string                         `json:"projectPath"`
	SU          string                         `json:"su"`
	SUPath      string                         `json:"suPath"`
	Values      LongVegetationPreferenceValues `json:"values"`
}

type LongVegetationPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationPreferencesService(contexts *ContextService, enabled bool) *LongVegetationPreferencesService {
	return &LongVegetationPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationPreferencesService) GetLongVegetationPreferences(ctx context.Context, contextID string) (*LongVegetationPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationPreferencesReview, error) {
			title, err := plots.projects.preferences.readLongVegetationPreference(ctx)
			if err != nil {
				return nil, err
			}
			if err := validateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongVegetationPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: LongVegetationPreferenceValues{Title: title}}, nil
		})
	})
}

// Retain the committed result even if cleanup or cancellation reports an error.
func (s *LongVegetationPreferencesService) SaveLongVegetationPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationPreferencesOutcome {
	outcome := &LongVegetationPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationPreferencesRequest
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
			update, err := plots.projects.preferences.compareAndSetLongVegetationPreference(ctx, request.Expected.Title, request.Proposed.Title)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
