package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type LongVegetationSwitchPreferenceValues struct {
	QuickReport         bool `json:"quickReport"`
	SpaceBetweenGroups  bool `json:"spaceBetweenGroups"`
	ReportSummary       bool `json:"reportSummary"`
	ConstantSpeciesList bool `json:"constantSpeciesList"`
}

func (v *LongVegetationSwitchPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSwitchPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preferences",
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationSwitchPreferencesService(contexts *vprocore.ContextService) *LongVegetationSwitchPreferencesService {
	return &LongVegetationSwitchPreferencesService{contexts: contexts}
}

func (s *LongVegetationSwitchPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation switch preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation switch preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationSwitchPreferencesService) GetLongVegetationSwitchPreferences(ctx context.Context, contextID string) (*LongVegetationSwitchPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationSwitchPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationSwitchPreferencesReview, error) {
			values, err := ReadLongVegetationSwitchPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSwitchPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values}, nil
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetLongVegetationSwitchPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
