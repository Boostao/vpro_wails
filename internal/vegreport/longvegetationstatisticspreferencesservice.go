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

type LongVegetationStatisticsPreferenceValues struct {
	AvgType int `json:"avgType"`
	OrderBy int `json:"orderBy"`
}

func (v *LongVegetationStatisticsPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationStatisticsPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preferences", "avgType", "orderBy"); err != nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationStatisticsPreferencesService(contexts *vprocore.ContextService) *LongVegetationStatisticsPreferencesService {
	return &LongVegetationStatisticsPreferencesService{contexts: contexts}
}

func (s *LongVegetationStatisticsPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation statistics preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation statistics preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationStatisticsPreferencesService) GetLongVegetationStatisticsPreferences(ctx context.Context, contextID string) (*LongVegetationStatisticsPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationStatisticsPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationStatisticsPreferencesReview, error) {
			values, err := ReadLongVegetationStatisticsPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationStatisticsPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values}, nil
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetLongVegetationStatisticsPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
