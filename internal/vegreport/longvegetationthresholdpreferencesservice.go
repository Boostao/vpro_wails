package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type LongVegetationThresholdPreferenceValues struct {
	PresenceGreaterThan  float64 `json:"presenceGreaterThan"`
	MeanCoverGreaterThan float64 `json:"meanCoverGreaterThan"`
}

func (v *LongVegetationThresholdPreferenceValues) UnmarshalJSON(data []byte) error {
	var decoded struct {
		PresenceGreaterThan  json.RawMessage `json:"presenceGreaterThan"`
		MeanCoverGreaterThan json.RawMessage `json:"meanCoverGreaterThan"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation threshold preferences",
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation threshold preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationThresholdPreferencesService(contexts *vprocore.ContextService) *LongVegetationThresholdPreferencesService {
	return &LongVegetationThresholdPreferencesService{contexts: contexts}
}

func (s *LongVegetationThresholdPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation threshold preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation threshold preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationThresholdPreferencesService) GetLongVegetationThresholdPreferences(ctx context.Context, contextID string) (*LongVegetationThresholdPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationThresholdPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationThresholdPreferencesReview, error) {
			values, err := ReadLongVegetationThresholdPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationThresholdPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values}, nil
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetLongVegetationThresholdPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
