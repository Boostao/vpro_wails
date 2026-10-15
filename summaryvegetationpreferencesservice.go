package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const summaryVegetationPreferencesFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_PREFERENCES"

func summaryVegetationPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(summaryVegetationPreferencesFeatureEnvironment, lookup)
}

type SummaryVegetationPreferenceValues struct {
	GroupBy    int     `json:"groupBy"`
	Average    int     `json:"average"`
	Display    int     `json:"display"`
	ShowClass  int     `json:"showClass"`
	ValueLimit float64 `json:"valueLimit"`
}

func (v *SummaryVegetationPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Vegetation preferences",
		"groupBy", "average", "display", "showClass", "valueLimit"); err != nil {
		return err
	}
	if err := validateSummaryVegetationPreferenceValues(summaryVegetationPreferenceValues(decoded)); err != nil {
		return err
	}
	*v = SummaryVegetationPreferenceValues(decoded)
	return nil
}

type SummaryVegetationPreferencesRequest struct {
	Expected SummaryVegetationPreferenceValues `json:"expected"`
	Proposed SummaryVegetationPreferenceValues `json:"proposed"`
}

func (v *SummaryVegetationPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Vegetation preference Save", "expected", "proposed"); err != nil {
		return err
	}
	*v = SummaryVegetationPreferencesRequest(decoded)
	return nil
}

type SummaryVegetationPreferencesReview struct {
	ContextID          string                            `json:"contextId"`
	Project            string                            `json:"project"`
	ProjectPath        string                            `json:"projectPath"`
	SU                 string                            `json:"su"`
	SUPath             string                            `json:"suPath"`
	Values             SummaryVegetationPreferenceValues `json:"values"`
	PreviewEnabled     bool                              `json:"previewEnabled"`
	SelectionAvailable bool                              `json:"selectionAvailable"`
}

type SummaryVegetationPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type SummaryVegetationPreferencesService struct {
	contexts       *ContextService
	enabled        bool
	previewEnabled bool
	snapshot       publicationReadSnapshotHooks
}

func NewSummaryVegetationPreferencesService(contexts *ContextService, enabled, previewEnabled bool) *SummaryVegetationPreferencesService {
	return &SummaryVegetationPreferencesService{contexts: contexts, enabled: enabled, previewEnabled: previewEnabled}
}

func (s *SummaryVegetationPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary Vegetation preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary Vegetation preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Summary Vegetation preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func summaryVegetationPreferenceSelectionAvailable(value SummaryVegetationPreferenceValues) bool {
	return !((value.GroupBy == 2 || value.GroupBy == 3) && value.Display == 2)
}

func (s *SummaryVegetationPreferencesService) GetSummaryVegetationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SummaryVegetationPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, _ *sql.Tx) (*SummaryVegetationPreferencesReview, error) {
			values, err := plots.projects.preferences.readSummaryVegetationPreferences(ctx)
			if err != nil {
				return nil, err
			}
			dto := SummaryVegetationPreferenceValues(values)
			return &SummaryVegetationPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
				Values: dto, PreviewEnabled: s.previewEnabled, SelectionAvailable: summaryVegetationPreferenceSelectionAvailable(dto)}, nil
		})
	})
}

// Keep irreversible persistence outside request cleanup's zero-on-error result.
func (s *SummaryVegetationPreferencesService) SaveSummaryVegetationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPreferencesOutcome {
	outcome := &SummaryVegetationPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SummaryVegetationPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*sqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			if request.Expected != request.Proposed && (!s.previewEnabled || !summaryVegetationPreferenceSelectionAvailable(request.Proposed)) {
				return struct{}{}, errors.New("Summary Vegetation selection is unavailable in this session; saved values were not reset")
			}
			update, saveErr := plots.projects.preferences.compareAndSetSummaryVegetationPreferences(ctx,
				summaryVegetationPreferenceValues(request.Expected), summaryVegetationPreferenceValues(request.Proposed))
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
