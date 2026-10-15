package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const longVegetationNamingPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_NAMING_PREFERENCES"

func longVegetationNamingPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationNamingPreferencesFeatureEnvironment, lookup)
}

type LongVegetationNamingPreferenceValues struct {
	Mode          int    `json:"mode"`
	SelectedField string `json:"selectedField"`
}

func (v *LongVegetationNamingPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationNamingPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation Show preferences", "mode", "selectedField"); err != nil {
		return err
	}
	if !longVegetationNamingMode(decoded.Mode) {
		return errors.New("Long Vegetation Show requires source mode 0, 1, 2 or historical selected-field mode 4")
	}
	*v = LongVegetationNamingPreferenceValues(decoded)
	return nil
}

type LongVegetationNamingPreferencesRequest struct {
	Expected LongVegetationNamingPreferenceValues `json:"expected"`
	Proposed LongVegetationNamingPreferenceValues `json:"proposed"`
}

func (v *LongVegetationNamingPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationNamingPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation Show preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationNamingPreferencesRequest(decoded)
	return nil
}

type LongVegetationNamingAvailability struct {
	None          bool `json:"none"`
	English       bool `json:"english"`
	Code          bool `json:"code"`
	SelectedField bool `json:"selectedField"`
}

func (a LongVegetationNamingAvailability) allows(mode int) bool {
	switch mode {
	case 0:
		return a.None
	case 1:
		return a.English
	case 2:
		return a.Code
	default:
		return false
	}
}

type LongVegetationNamingPreferencesReview struct {
	ContextID    string                               `json:"contextId"`
	Project      string                               `json:"project"`
	ProjectPath  string                               `json:"projectPath"`
	SU           string                               `json:"su"`
	SUPath       string                               `json:"suPath"`
	Values       LongVegetationNamingPreferenceValues `json:"values"`
	Availability LongVegetationNamingAvailability     `json:"availability"`
}

type LongVegetationNamingPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationNamingPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationNamingPreferencesService(contexts *ContextService, enabled bool) *LongVegetationNamingPreferencesService {
	return &LongVegetationNamingPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationNamingPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation Show preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation Show preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation Show preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationNamingPreferencesService) availability() LongVegetationNamingAvailability {
	return LongVegetationNamingAvailability{None: true, English: true, Code: s.contexts.longVegetationCodeEnabled}
}

func (s *LongVegetationNamingPreferencesService) GetLongVegetationNamingPreferences(ctx context.Context, contextID string) (*LongVegetationNamingPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationNamingPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, _ *sql.Tx) (*LongVegetationNamingPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationNamingPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationNamingPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: values, Availability: s.availability()}, nil
		})
	})
}

// Persistence receipts survive request cleanup errors.
func (s *LongVegetationNamingPreferencesService) SaveLongVegetationNamingPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationNamingPreferencesOutcome {
	outcome := &LongVegetationNamingPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationNamingPreferencesRequest
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
			if request.Expected != request.Proposed && !s.availability().allows(request.Proposed.Mode) {
				return struct{}{}, fmt.Errorf("Long Vegetation Show choice %d is disabled in this session; saved naming values were not reset", request.Proposed.Mode)
			}
			update, saveErr := plots.projects.preferences.compareAndSetLongVegetationNamingPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
