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
)

type LongVegetationNamingPreferenceValues struct {
	Mode          int    `json:"mode"`
	SelectedField string `json:"selectedField"`
}

func (v *LongVegetationNamingPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationNamingPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation Show preferences", "mode", "selectedField"); err != nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation Show preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationNamingPreferencesService(contexts *vprocore.ContextService) *LongVegetationNamingPreferencesService {
	return &LongVegetationNamingPreferencesService{contexts: contexts}
}

func (s *LongVegetationNamingPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation Show preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation Show preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationNamingPreferencesService) availability() LongVegetationNamingAvailability {
	return LongVegetationNamingAvailability{None: true, English: true, Code: true}
}

func (s *LongVegetationNamingPreferencesService) GetLongVegetationNamingPreferences(ctx context.Context, contextID string) (*LongVegetationNamingPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationNamingPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationNamingPreferencesReview, error) {
			values, err := ReadLongVegetationNamingPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationNamingPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			if request.Expected != request.Proposed && !s.availability().allows(request.Proposed.Mode) {
				return struct{}{}, fmt.Errorf("Long Vegetation Show choice %d is unavailable on this control; saved naming values were not reset", request.Proposed.Mode)
			}
			update, saveErr := CompareAndSetLongVegetationNamingPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
