package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longEnvironmentPreferencesFeatureEnvironment = "VPRO_LONG_ENVIRONMENT_PREFERENCES"

func longEnvironmentPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longEnvironmentPreferencesFeatureEnvironment, lookup)
}

type LongEnvironmentPreferenceValues struct {
	Title string `json:"title"`
}

func (v *LongEnvironmentPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentPreferenceValues
	var decoded plain
	if err := decodeGoogleEarthKMLJSON(data, &decoded, "Long Environment preferences", "title"); err != nil {
		return err
	}
	*v = LongEnvironmentPreferenceValues(decoded)
	return nil
}

type LongEnvironmentPreferencesRequest struct {
	Expected LongEnvironmentPreferenceValues `json:"expected"`
	Proposed LongEnvironmentPreferenceValues `json:"proposed"`
}

func (v *LongEnvironmentPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentPreferencesRequest
	var decoded plain
	if err := decodeGoogleEarthKMLJSON(data, &decoded, "Long Environment preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongEnvironmentPreferencesRequest(decoded)
	return nil
}

type LongEnvironmentPreferencesReview struct {
	ContextID   string                          `json:"contextId"`
	Project     string                          `json:"project"`
	ProjectPath string                          `json:"projectPath"`
	SU          string                          `json:"su"`
	SUPath      string                          `json:"suPath"`
	Values      LongEnvironmentPreferenceValues `json:"values"`
}

type LongEnvironmentPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongEnvironmentPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongEnvironmentPreferencesService(contexts *ContextService, enabled bool) *LongEnvironmentPreferencesService {
	return &LongEnvironmentPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongEnvironmentPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Environment preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Environment preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Environment preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongEnvironmentPreferencesService) GetLongEnvironmentPreferences(ctx context.Context, contextID string) (*LongEnvironmentPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongEnvironmentPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongEnvironmentPreferencesReview, error) {
			title, err := plots.projects.preferences.readLongEnvironmentPreference(ctx)
			if err != nil {
				return nil, err
			}
			if err := validateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongEnvironmentPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: LongEnvironmentPreferenceValues{Title: title}}, nil
		})
	})
}

// Keep the committed receipt outside callbacks: post-commit cancellation must not
// become an RPC error which loses the irreversible result.
func (s *LongEnvironmentPreferencesService) SaveLongEnvironmentPreferences(ctx context.Context, contextID, requestJSON string) *LongEnvironmentPreferencesOutcome {
	outcome := &LongEnvironmentPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongEnvironmentPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			// Verify live physical ownership and finish cleanup before YAML CAS.
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*sqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := plots.projects.preferences.compareAndSetLongEnvironmentPreference(ctx, request.Expected.Title, request.Proposed.Title)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
