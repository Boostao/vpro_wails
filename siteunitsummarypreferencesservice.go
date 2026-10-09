package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const siteUnitSummaryPreferencesFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_PREFERENCES"

func siteUnitSummaryPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siteUnitSummaryPreferencesFeatureEnvironment, lookup)
}

type SiteUnitSummaryPreferencesRequest struct {
	Expected SiteUnitSummaryPreferenceValues `json:"expected"`
	Proposed SiteUnitSummaryPreferenceValues `json:"proposed"`
}

func (request *SiteUnitSummaryPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryPreferencesRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Environment preference save", "expected", "proposed"); err != nil {
		return err
	}
	*request = SiteUnitSummaryPreferencesRequest(decoded)
	return nil
}

type SiteUnitSummaryPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type SiteUnitSummaryPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewSiteUnitSummaryPreferencesService(contexts *ContextService, enabled bool) *SiteUnitSummaryPreferencesService {
	return &SiteUnitSummaryPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *SiteUnitSummaryPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary Environment preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary Environment preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Summary Environment preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *SiteUnitSummaryPreferencesService) GetSiteUnitSummaryPreferences(ctx context.Context, contextID string) (*SiteUnitSummaryOptions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	options, err := s.contexts.readSiteUnitSummaryOptions(ctx, contextID, s.snapshot)
	if err != nil {
		return nil, err
	}
	return &options, nil
}

func (s *SiteUnitSummaryPreferencesService) SaveSiteUnitSummaryPreferences(ctx context.Context,
	contextID, requestJSON string) *SiteUnitSummaryPreferencesOutcome {
	outcome := &SiteUnitSummaryPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SiteUnitSummaryPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (struct{}, error) {
				if owner.selection.SU == "None" {
					return struct{}{}, errors.New("Summary Environment preferences require an explicitly selected normal SU")
				}
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := plots.projects.preferences.compareAndSetSiteUnitSummaryPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
