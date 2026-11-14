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

type LongVegetationPreferenceValues struct {
	Title string `json:"title"`
}

func (v *LongVegetationPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation preferences", "title"); err != nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationPreferencesService(contexts *vprocore.ContextService) *LongVegetationPreferencesService {
	return &LongVegetationPreferencesService{contexts: contexts}
}

func (s *LongVegetationPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationPreferencesService) GetLongVegetationPreferences(ctx context.Context, contextID string) (*LongVegetationPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationPreferencesReview, error) {
			title, err := ReadLongVegetationPreference(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongVegetationPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetLongVegetationPreference(plots.Projects.Preferences, ctx, request.Expected.Title, request.Proposed.Title)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
