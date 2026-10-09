package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longVegetationSwitchPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_SWITCH_PREFERENCES"

func longVegetationSwitchPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationSwitchPreferencesFeatureEnvironment, lookup)
}

type LongVegetationSwitchPreferenceValues struct {
	QuickReport         bool `json:"quickReport"`
	SpaceBetweenGroups  bool `json:"spaceBetweenGroups"`
	ReportSummary       bool `json:"reportSummary"`
	ConstantSpeciesList bool `json:"constantSpeciesList"`
}

func (v *LongVegetationSwitchPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSwitchPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preferences",
		"quickReport", "spaceBetweenGroups", "reportSummary", "constantSpeciesList"); err != nil {
		return err
	}
	*v = LongVegetationSwitchPreferenceValues(decoded)
	return nil
}

type LongVegetationSwitchPreferencesRequest struct {
	Expected LongVegetationSwitchPreferenceValues `json:"expected"`
	Proposed LongVegetationSwitchPreferenceValues `json:"proposed"`
}

func (v *LongVegetationSwitchPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSwitchPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationSwitchPreferencesRequest(decoded)
	return nil
}

type LongVegetationSwitchPreferencesReview struct {
	ContextID   string                               `json:"contextId"`
	Project     string                               `json:"project"`
	ProjectPath string                               `json:"projectPath"`
	SU          string                               `json:"su"`
	SUPath      string                               `json:"suPath"`
	Values      LongVegetationSwitchPreferenceValues `json:"values"`
}

type LongVegetationSwitchPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationSwitchPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationSwitchPreferencesService(contexts *ContextService, enabled bool) *LongVegetationSwitchPreferencesService {
	return &LongVegetationSwitchPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationSwitchPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation switch preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation switch preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation switch preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationSwitchPreferencesService) GetLongVegetationSwitchPreferences(ctx context.Context, contextID string) (*LongVegetationSwitchPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationSwitchPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationSwitchPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationSwitchPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSwitchPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: values}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *LongVegetationSwitchPreferencesService) SaveLongVegetationSwitchPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSwitchPreferencesOutcome {
	outcome := &LongVegetationSwitchPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationSwitchPreferencesRequest
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
			update, err := plots.projects.preferences.compareAndSetLongVegetationSwitchPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
