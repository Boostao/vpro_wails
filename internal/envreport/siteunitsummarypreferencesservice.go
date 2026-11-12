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

const siteUnitSummaryPreferencesFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_PREFERENCES"

func SiteUnitSummaryPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(siteUnitSummaryPreferencesFeatureEnvironment, lookup)
}

type SiteUnitSummaryPreferencesRequest struct {
	Expected SiteUnitSummaryPreferenceValues `json:"expected"`
	Proposed SiteUnitSummaryPreferenceValues `json:"proposed"`
}

func (request *SiteUnitSummaryPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Environment preference save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSiteUnitSummaryPreferencesService(contexts *vprocore.ContextService, enabled bool) *SiteUnitSummaryPreferencesService {
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
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Summary Environment preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *SiteUnitSummaryPreferencesService) GetSiteUnitSummaryPreferences(ctx context.Context, contextID string) (*SiteUnitSummaryOptions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	options, err := ReadSiteUnitSummaryOptions(s.contexts, ctx, contextID, s.snapshot)
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
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (struct{}, error) {
				if owner.Selection.SU == "None" {
					return struct{}{}, errors.New("Summary Environment preferences require an explicitly selected normal SU")
				}
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, err := CompareAndSetSiteUnitSummaryPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
