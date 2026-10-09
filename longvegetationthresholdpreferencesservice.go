package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const longVegetationThresholdPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_THRESHOLD_PREFERENCES"

func longVegetationThresholdPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationThresholdPreferencesFeatureEnvironment, lookup)
}

type LongVegetationThresholdPreferenceValues struct {
	PresenceGreaterThan  float64 `json:"presenceGreaterThan"`
	MeanCoverGreaterThan float64 `json:"meanCoverGreaterThan"`
}

func (v *LongVegetationThresholdPreferenceValues) UnmarshalJSON(data []byte) error {
	var decoded struct {
		PresenceGreaterThan  json.RawMessage `json:"presenceGreaterThan"`
		MeanCoverGreaterThan json.RawMessage `json:"meanCoverGreaterThan"`
	}
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation threshold preferences",
		"presenceGreaterThan", "meanCoverGreaterThan"); err != nil {
		return err
	}
	presence, err := decodeLongVegetationThresholdNumber(decoded.PresenceGreaterThan, "presence")
	if err != nil {
		return err
	}
	cover, err := decodeLongVegetationThresholdNumber(decoded.MeanCoverGreaterThan, "mean cover")
	if err != nil {
		return err
	}
	*v = LongVegetationThresholdPreferenceValues{PresenceGreaterThan: presence, MeanCoverGreaterThan: cover}
	return nil
}

func decodeLongVegetationThresholdNumber(data []byte, field string) (float64, error) {
	var decoded *float64
	if err := json.Unmarshal(data, &decoded); err != nil {
		return 0, fmt.Errorf("Long Vegetation %s threshold requires a finite JSON number: %w", field, err)
	}
	if decoded == nil {
		return 0, fmt.Errorf("Long Vegetation %s threshold requires a finite JSON number, not NULL", field)
	}
	value := *decoded
	mantissa := strings.FieldsFunc(string(data), func(r rune) bool { return r == 'e' || r == 'E' })[0]
	if value == 0 && strings.ContainsAny(mantissa, "123456789") {
		return 0, fmt.Errorf("Long Vegetation %s threshold underflows the finite DOUBLE domain", field)
	}
	return value, nil
}

type LongVegetationThresholdPreferencesRequest struct {
	Expected LongVegetationThresholdPreferenceValues `json:"expected"`
	Proposed LongVegetationThresholdPreferenceValues `json:"proposed"`
}

func (v *LongVegetationThresholdPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationThresholdPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation threshold preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationThresholdPreferencesRequest(decoded)
	return nil
}

type LongVegetationThresholdPreferencesReview struct {
	ContextID   string                                  `json:"contextId"`
	Project     string                                  `json:"project"`
	ProjectPath string                                  `json:"projectPath"`
	SU          string                                  `json:"su"`
	SUPath      string                                  `json:"suPath"`
	Values      LongVegetationThresholdPreferenceValues `json:"values"`
}

type LongVegetationThresholdPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationThresholdPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationThresholdPreferencesService(contexts *ContextService, enabled bool) *LongVegetationThresholdPreferencesService {
	return &LongVegetationThresholdPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationThresholdPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation threshold preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation threshold preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation threshold preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationThresholdPreferencesService) GetLongVegetationThresholdPreferences(ctx context.Context, contextID string) (*LongVegetationThresholdPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationThresholdPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationThresholdPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationThresholdPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationThresholdPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: values}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *LongVegetationThresholdPreferencesService) SaveLongVegetationThresholdPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationThresholdPreferencesOutcome {
	outcome := &LongVegetationThresholdPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationThresholdPreferencesRequest
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
			update, err := plots.projects.preferences.compareAndSetLongVegetationThresholdPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
