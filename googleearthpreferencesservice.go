package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const googleEarthPreferencesFeatureEnvironment = "VPRO_GOOGLE_EARTH_PREFERENCES"

func googleEarthPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(googleEarthPreferencesFeatureEnvironment, lookup)
}

type GoogleEarthPreferenceValues struct {
	Title            string `json:"title"`
	DescriptionField string `json:"descriptionField"`
}

func (v *GoogleEarthPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthPreferenceValues
	var decoded plain
	if err := decodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preferences", "title", "descriptionField"); err != nil {
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
	if err := decodeGoogleEarthKMLJSON(data, &decoded, "Google Earth preference save", "expected", "proposed"); err != nil {
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
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewGoogleEarthPreferencesService(contexts *ContextService, enabled bool) *GoogleEarthPreferencesService {
	return &GoogleEarthPreferencesService{contexts: contexts, enabled: enabled}
}

func withOwnedPreferenceSnapshot[T any](ctx context.Context, plots *PlotService, hooks publicationReadSnapshotHooks,
	read func(*sqliteContext, *sql.Tx) (T, error)) (T, error) {
	owner := plots.projects.sqlite
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		var zero T
		return zero, err
	}
	defer owner.mu.Unlock()
	return withPublicationReadSnapshot(ctx, owner, hooks, func(tx *sql.Tx) (T, error) { return read(owner, tx) })
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
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Google Earth preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *GoogleEarthPreferencesService) GetGoogleEarthPreferences(ctx context.Context, contextID string) (*GoogleEarthPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*GoogleEarthPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*GoogleEarthPreferencesReview, error) {
			values, err := plots.projects.preferences.readGoogleEarthPreferences(ctx)
			if err != nil {
				return nil, err
			}
			if err := validateGoogleEarthReviewStrings(values.PlaceName, values.DescriptionField); err != nil {
				return nil, err
			}
			return &GoogleEarthPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
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
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			// Complete the physical read and its cleanup before the config commit.
			// Unchanged historical/unavailable fields do not need new membership.
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (struct{}, error) {
				if request.Proposed.DescriptionField == request.Expected.DescriptionField {
					return struct{}{}, nil
				}
				table, err := readPhysicalLocationTable(ctx, tx, "project", owner.selection.Project+"_Env")
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
			update, err := plots.projects.preferences.compareAndSetGoogleEarthPreferences(ctx,
				googleEarthPreferences{PlaceName: request.Expected.Title, DescriptionField: request.Expected.DescriptionField},
				googleEarthPreferences{PlaceName: request.Proposed.Title, DescriptionField: request.Proposed.DescriptionField})
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
