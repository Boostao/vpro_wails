package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const googleEarthPreferencesFeatureEnvironment = "VPRO_GOOGLE_EARTH_PREFERENCES"

func GoogleEarthPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(googleEarthPreferencesFeatureEnvironment, lookup)
}

type GoogleEarthPreferenceValues struct {
	Title            string `json:"title"`
	DescriptionField string `json:"descriptionField"`
}

func (v *GoogleEarthPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthPreferenceValues
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preferences", "title", "descriptionField"); err != nil {
		return err
	}
	*v = GoogleEarthPreferenceValues(decoded)
	return nil
}

type GoogleEarthPreferencesRequest struct {
	Expected GoogleEarthPreferenceValues `json:"expected"`
	Proposed GoogleEarthPreferenceValues `json:"proposed"`
}

func (v *GoogleEarthPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = GoogleEarthPreferencesRequest(decoded)
	return nil
}

type GoogleEarthPreferencesReview struct {
	ContextID   string                      `json:"contextId"`
	Project     string                      `json:"project"`
	ProjectPath string                      `json:"projectPath"`
	SU          string                      `json:"su"`
	SUPath      string                      `json:"suPath"`
	Values      GoogleEarthPreferenceValues `json:"values"`
}

type GoogleEarthPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type GoogleEarthPreferencesService struct {
	contexts *vprocore.ContextService
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewGoogleEarthPreferencesService(contexts *vprocore.ContextService, enabled bool) *GoogleEarthPreferencesService {
	return &GoogleEarthPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *GoogleEarthPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Google Earth preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Google Earth preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Google Earth preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *GoogleEarthPreferencesService) GetGoogleEarthPreferences(ctx context.Context, contextID string) (*GoogleEarthPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*GoogleEarthPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*GoogleEarthPreferencesReview, error) {
			values, err := ReadGoogleEarthPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(values.PlaceName, values.DescriptionField); err != nil {
				return nil, err
			}
			return &GoogleEarthPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: GoogleEarthPreferenceValues{Title: values.PlaceName, DescriptionField: values.DescriptionField}}, nil
		})
	})
}

// Save always returns a typed receipt, including post-commit failures. An RPC error
// would discard the committed bit and make an unsafe replay appear appropriate.
func (s *GoogleEarthPreferencesService) SaveGoogleEarthPreferences(ctx context.Context, contextID, requestJSON string) *GoogleEarthPreferencesOutcome {
	outcome := &GoogleEarthPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request GoogleEarthPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			// Complete the physical read and its cleanup before the config commit.
			// Unchanged historical/unavailable fields do not need new membership.
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (struct{}, error) {
				if request.Proposed.DescriptionField == request.Expected.DescriptionField {
					return struct{}{}, nil
				}
				table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
				if err != nil {
					return struct{}{}, err
				}
				for _, column := range table.Columns {
					if column.Name == request.Proposed.DescriptionField {
						return struct{}{}, nil
					}
				}
				return struct{}{}, errors.New("new Google Earth description field is not a current owned physical Env field")
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetGoogleEarthPreferences(plots.Projects.Preferences, ctx,
				GoogleEarthPreferences{PlaceName: request.Expected.Title, DescriptionField: request.Expected.DescriptionField},
				GoogleEarthPreferences{PlaceName: request.Proposed.Title, DescriptionField: request.Proposed.DescriptionField})
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
