package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const longVegetationGroupPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_GROUP_PREFERENCES"

func longVegetationGroupPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationGroupPreferencesFeatureEnvironment, lookup)
}

type LongVegetationGroupPreferenceValues struct {
	GroupBy int `json:"groupBy"`
}

func (v *LongVegetationGroupPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationGroupPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation grouping preferences", "groupBy"); err != nil {
		return err
	}
	if decoded.GroupBy < 1 || decoded.GroupBy > 4 {
		return errors.New("Long Vegetation Group by requires source choice 1, 2, 3 or 4")
	}
	*v = LongVegetationGroupPreferenceValues(decoded)
	return nil
}

type LongVegetationGroupPreferencesRequest struct {
	Expected LongVegetationGroupPreferenceValues `json:"expected"`
	Proposed LongVegetationGroupPreferenceValues `json:"proposed"`
}

func (v *LongVegetationGroupPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationGroupPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation grouping preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationGroupPreferencesRequest(decoded)
	return nil
}

type LongVegetationGroupAvailability struct {
	Layer    bool `json:"layer"`
	Strata   bool `json:"strata"`
	Lifeform bool `json:"lifeform"`
	None     bool `json:"none"`
}

func (a LongVegetationGroupAvailability) allows(group int) bool {
	switch group {
	case 1:
		return a.Layer
	case 2:
		return a.Strata
	case 3:
		return a.Lifeform
	case 4:
		return a.None
	default:
		return false
	}
}

type LongVegetationGroupPreferencesReview struct {
	ContextID    string                              `json:"contextId"`
	Project      string                              `json:"project"`
	ProjectPath  string                              `json:"projectPath"`
	SU           string                              `json:"su"`
	SUPath       string                              `json:"suPath"`
	Values       LongVegetationGroupPreferenceValues `json:"values"`
	Availability LongVegetationGroupAvailability     `json:"availability"`
}

type LongVegetationGroupPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationGroupPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewLongVegetationGroupPreferencesService(contexts *ContextService, enabled bool) *LongVegetationGroupPreferencesService {
	return &LongVegetationGroupPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationGroupPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation grouping preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation grouping preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation grouping preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationGroupPreferencesService) availability() LongVegetationGroupAvailability {
	return LongVegetationGroupAvailability{Layer: true, Strata: s.contexts.longVegetationStrataEnabled,
		Lifeform: s.contexts.longVegetationLifeformEnabled, None: s.contexts.longVegetationNoneEnabled}
}

func (s *LongVegetationGroupPreferencesService) GetLongVegetationGroupPreferences(ctx context.Context, contextID string) (*LongVegetationGroupPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationGroupPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationGroupPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationGroupPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationGroupPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: values, Availability: s.availability()}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *LongVegetationGroupPreferencesService) SaveLongVegetationGroupPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationGroupPreferencesOutcome {
	outcome := &LongVegetationGroupPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationGroupPreferencesRequest
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
			if request.Expected.GroupBy != request.Proposed.GroupBy && !s.availability().allows(request.Proposed.GroupBy) {
				return struct{}{}, fmt.Errorf("Long Vegetation grouping choice %d is disabled in this session; saved LVGroupBy was not reset", request.Proposed.GroupBy)
			}
			update, err := plots.projects.preferences.compareAndSetLongVegetationGroupPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
