package envreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func validateLongEnvironmentTitle(title string) error {
	if !utf8.ValidString(title) || strings.ContainsRune(title, 0) {
		return errors.New("Long Environment title requires complete Unicode without NUL; no repair or normalization")
	}
	return nil
}

func ReadLongEnvironmentPreference(s *vprocore.DesktopConfig, ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("Long Environment preferences require a context")
	}
	if s == nil {
		return "", errors.New("Long Environment preference configuration is unavailable")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &s.Mu); err != nil {
		return "", err
	}
	defer s.Mu.Unlock()
	values, err := s.ReadLocked()
	if err != nil {
		return "", err
	}
	title, err := vprocore.ConfigString(values, "ReportOptions", "LEReportTitle")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return title, nil
}

func CompareAndSetLongEnvironmentPreference(s *vprocore.DesktopConfig, ctx context.Context, expected, proposed string) (vprocore.ConfigStringPreferenceUpdate, error) {
	if ctx == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Environment preference changes require a context")
	}
	if s == nil {
		return vprocore.ConfigStringPreferenceUpdate{}, errors.New("Long Environment preference configuration is unavailable")
	}
	if expected != proposed {
		if err := validateLongEnvironmentTitle(proposed); err != nil {
			return vprocore.ConfigStringPreferenceUpdate{}, err
		}
	}
	return s.CompareAndSetReportStrings(ctx, "Long Environment preferences", []vprocore.ReportStringPreferenceChange{
		{Key: "LEReportTitle", Expected: expected, Proposed: proposed},
	})
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
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongEnvironmentPreferencesService(contexts *vprocore.ContextService) *LongEnvironmentPreferencesService {
	return &LongEnvironmentPreferencesService{contexts: contexts}
}

func (s *LongEnvironmentPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongEnvironmentPreferencesService) GetLongEnvironmentPreferences(ctx context.Context, contextID string) (*LongEnvironmentPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Environment",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongEnvironmentPreferencesReview, error) {
			title, err := ReadLongEnvironmentPreference(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongEnvironmentPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: LongEnvironmentPreferenceValues{Title: title},
			}, nil
		})
}

func (s *LongEnvironmentPreferencesService) SaveLongEnvironmentPreferences(ctx context.Context, contextID, requestJSON string) *LongEnvironmentPreferencesOutcome {
	out := LongEnvironmentPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Environment", nil,
		func(plots *vprocore.PlotService, request LongEnvironmentPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongEnvironmentPreference(plots.Projects.Preferences, ctx, request.Expected.Title, request.Proposed.Title)
		}))
	return &out
}

type SiteUnitSummaryPreferenceValues struct {
	Method       int `json:"method"`
	SiteUnitType int `json:"siteUnitType"`
}

func (values *SiteUnitSummaryPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Environment preferences", "method", "siteUnitType"); err != nil {
		return err
	}
	*values = SiteUnitSummaryPreferenceValues(decoded)
	return nil
}

func CompareAndSetSiteUnitSummaryPreferences(s *vprocore.DesktopConfig, ctx context.Context,
	expected, proposed SiteUnitSummaryPreferenceValues) (vprocore.ReportPreferenceUpdate, error) {
	if proposed.Method != 1 && proposed.Method != 2 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	if proposed.SiteUnitType != 1 {
		return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Environment preference changes support only normal SU; hierarchy/field workflows are unavailable")
	}
	return vprocore.CompareAndSetReportPreferences(s, ctx, "Summary Environment preferences", []vprocore.ReportPreferenceChange[int]{
		{Key: "SEOptValueMethod", Expected: expected.Method, Proposed: proposed.Method},
		{Key: "SESuType", Expected: expected.SiteUnitType, Proposed: proposed.SiteUnitType},
	}, func(values vprocore.ConfigValues, key string) (int, error) {
		switch key {
		case "SEOptValueMethod":
			return vprocore.ConfigInt(values, "ReportOptions", key, 1, 2)
		case "SESuType":
			return vprocore.ConfigInt(values, "ReportOptions", key, 1, 3)
		default:
			return 0, fmt.Errorf("unsupported Summary Environment preference %s", key)
		}
	})
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
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSiteUnitSummaryPreferencesService(contexts *vprocore.ContextService) *SiteUnitSummaryPreferencesService {
	return &SiteUnitSummaryPreferencesService{contexts: contexts}
}

func (s *SiteUnitSummaryPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *SiteUnitSummaryPreferencesService) GetSiteUnitSummaryPreferences(ctx context.Context, contextID string) (*SiteUnitSummaryOptions, error) {
	h := s.helper()
	if err := vprocore.RequireReportPreferenceContext(ctx, h, contextID, "Summary Environment"); err != nil {
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
	out := SiteUnitSummaryPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Summary Environment",
		func(owner *vprocore.SqliteContext, _ *sql.Tx, _ SiteUnitSummaryPreferencesRequest) error {
			if owner.Selection.SU == "None" {
				return errors.New("Summary Environment preferences require an explicitly selected normal SU")
			}
			return nil
		},
		func(plots *vprocore.PlotService, request SiteUnitSummaryPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetSiteUnitSummaryPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}
