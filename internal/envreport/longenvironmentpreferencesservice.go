package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const longEnvironmentPreferencesFeatureEnvironment = "VPRO_LONG_ENVIRONMENT_PREFERENCES"

func LongEnvironmentPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(longEnvironmentPreferencesFeatureEnvironment, lookup)
}

type LongEnvironmentPreferenceValues struct {
	Title string `json:"title"`
}

func (v *LongEnvironmentPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentPreferenceValues
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Long Environment preferences", "title"); err != nil {
		return err
	}
	*v = LongEnvironmentPreferenceValues(decoded)
	return nil
}

type LongEnvironmentPreferencesRequest struct {
	Expected LongEnvironmentPreferenceValues `json:"expected"`
	Proposed LongEnvironmentPreferenceValues `json:"proposed"`
}

func (v *LongEnvironmentPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Long Environment preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongEnvironmentPreferencesRequest(decoded)
	return nil
}

type LongEnvironmentPreferencesReview struct {
	ContextID   string                          `json:"contextId"`
	Project     string                          `json:"project"`
	ProjectPath string                          `json:"projectPath"`
	SU          string                          `json:"su"`
	SUPath      string                          `json:"suPath"`
	Values      LongEnvironmentPreferenceValues `json:"values"`
}

type LongEnvironmentPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongEnvironmentPreferencesService struct {
	contexts *vprocore.ContextService
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongEnvironmentPreferencesService(contexts *vprocore.ContextService, enabled bool) *LongEnvironmentPreferencesService {
	return &LongEnvironmentPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongEnvironmentPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Environment preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Environment preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Environment preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *LongEnvironmentPreferencesService) GetLongEnvironmentPreferences(ctx context.Context, contextID string) (*LongEnvironmentPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongEnvironmentPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongEnvironmentPreferencesReview, error) {
			title, err := ReadLongEnvironmentPreference(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongEnvironmentPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: LongEnvironmentPreferenceValues{Title: title}}, nil
		})
	})
}

// Keep the committed receipt outside callbacks: post-commit cancellation must not
// become an RPC error which loses the irreversible result.
func (s *LongEnvironmentPreferencesService) SaveLongEnvironmentPreferences(ctx context.Context, contextID, requestJSON string) *LongEnvironmentPreferencesOutcome {
	outcome := &LongEnvironmentPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongEnvironmentPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			// Verify live physical ownership and finish cleanup before YAML CAS.
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetLongEnvironmentPreference(plots.Projects.Preferences, ctx, request.Expected.Title, request.Proposed.Title)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
