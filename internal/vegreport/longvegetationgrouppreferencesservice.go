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

type LongVegetationGroupPreferenceValues struct {
	GroupBy int `json:"groupBy"`
}

func (v *LongVegetationGroupPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationGroupPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation grouping preferences", "groupBy"); err != nil {
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation grouping preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationGroupPreferencesService(contexts *vprocore.ContextService) *LongVegetationGroupPreferencesService {
	return &LongVegetationGroupPreferencesService{contexts: contexts}
}

func (s *LongVegetationGroupPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation grouping preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation grouping preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationGroupPreferencesService) availability() LongVegetationGroupAvailability {
	return LongVegetationGroupAvailability{Layer: true, Strata: true, Lifeform: true, None: true}
}

func (s *LongVegetationGroupPreferencesService) GetLongVegetationGroupPreferences(ctx context.Context, contextID string) (*LongVegetationGroupPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationGroupPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationGroupPreferencesReview, error) {
			values, err := ReadLongVegetationGroupPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationGroupPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			if request.Expected.GroupBy != request.Proposed.GroupBy && !s.availability().allows(request.Proposed.GroupBy) {
				return struct{}{}, fmt.Errorf("Long Vegetation grouping choice %d is unavailable; saved LVGroupBy was not reset", request.Proposed.GroupBy)
			}
			update, err := CompareAndSetLongVegetationGroupPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
