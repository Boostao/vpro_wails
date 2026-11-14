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

type LongVegetationQualityNullPreferenceValues struct {
	SiteIncludeNulls bool `json:"siteIncludeNulls"`
	VegIncludeNulls  bool `json:"vegIncludeNulls"`
	SoilIncludeNulls bool `json:"soilIncludeNulls"`
}

func (v *LongVegetationQualityNullPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preferences",
		"siteIncludeNulls", "vegIncludeNulls", "soilIncludeNulls"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferenceValues(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesRequest struct {
	Expected LongVegetationQualityNullPreferenceValues `json:"expected"`
	Proposed LongVegetationQualityNullPreferenceValues `json:"proposed"`
}

func (v *LongVegetationQualityNullPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferencesRequest(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesReview struct {
	ContextID   string                                    `json:"contextId"`
	Project     string                                    `json:"project"`
	ProjectPath string                                    `json:"projectPath"`
	SU          string                                    `json:"su"`
	SUPath      string                                    `json:"suPath"`
	Values      LongVegetationQualityNullPreferenceValues `json:"values"`
}

type LongVegetationQualityNullPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationQualityNullPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationQualityNullPreferencesService(contexts *vprocore.ContextService) *LongVegetationQualityNullPreferencesService {
	return &LongVegetationQualityNullPreferencesService{contexts: contexts}
}

func (s *LongVegetationQualityNullPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation quality Include Nulls preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation quality Include Nulls preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationQualityNullPreferencesService) GetLongVegetationQualityNullPreferences(ctx context.Context, contextID string) (*LongVegetationQualityNullPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationQualityNullPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationQualityNullPreferencesReview, error) {
			values, err := ReadLongVegetationQualityNullPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityNullPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *LongVegetationQualityNullPreferencesService) SaveLongVegetationQualityNullPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityNullPreferencesOutcome {
	outcome := &LongVegetationQualityNullPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationQualityNullPreferencesRequest
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
			update, err := CompareAndSetLongVegetationQualityNullPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
