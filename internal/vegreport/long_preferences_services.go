package vegreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// 1. Long Vegetation Report Title Preferences

type LongVegetationPreferenceValues struct {
	Title string `json:"title"`
}

func (v *LongVegetationPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation preferences", "title"); err != nil {
		return err
	}
	*v = LongVegetationPreferenceValues(decoded)
	return nil
}

type LongVegetationPreferencesRequest struct {
	Expected LongVegetationPreferenceValues `json:"expected"`
	Proposed LongVegetationPreferenceValues `json:"proposed"`
}

func (v *LongVegetationPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationPreferencesRequest(decoded)
	return nil
}

type LongVegetationPreferencesReview struct {
	ContextID   string                         `json:"contextId"`
	Project     string                         `json:"project"`
	ProjectPath string                         `json:"projectPath"`
	SU          string                         `json:"su"`
	SUPath      string                         `json:"suPath"`
	Values      LongVegetationPreferenceValues `json:"values"`
}

type LongVegetationPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationPreferencesService(contexts *vprocore.ContextService) *LongVegetationPreferencesService {
	return &LongVegetationPreferencesService{contexts: contexts}
}

func (s *LongVegetationPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationPreferencesService) GetLongVegetationPreferences(ctx context.Context, contextID string) (*LongVegetationPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationPreferencesReview, error) {
			title, err := ReadLongVegetationPreference(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(title); err != nil {
				return nil, err
			}
			return &LongVegetationPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: LongVegetationPreferenceValues{Title: title},
			}, nil
		})
}

func (s *LongVegetationPreferencesService) SaveLongVegetationPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationPreferencesOutcome {
	out := LongVegetationPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation", nil,
		func(plots *vprocore.PlotService, request LongVegetationPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationPreference(plots.Projects.Preferences, ctx, request.Expected.Title, request.Proposed.Title)
		}))
	return &out
}

// 2. Long Vegetation Statistics Preferences

type LongVegetationStatisticsPreferenceValues struct {
	AvgType int `json:"avgType"`
	OrderBy int `json:"orderBy"`
}

func (v *LongVegetationStatisticsPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationStatisticsPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preferences", "avgType", "orderBy"); err != nil {
		return err
	}
	*v = LongVegetationStatisticsPreferenceValues(decoded)
	return nil
}

type LongVegetationStatisticsPreferencesRequest struct {
	Expected LongVegetationStatisticsPreferenceValues `json:"expected"`
	Proposed LongVegetationStatisticsPreferenceValues `json:"proposed"`
}

func (v *LongVegetationStatisticsPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationStatisticsPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation statistics preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationStatisticsPreferencesRequest(decoded)
	return nil
}

type LongVegetationStatisticsPreferencesReview struct {
	ContextID   string                                   `json:"contextId"`
	Project     string                                   `json:"project"`
	ProjectPath string                                   `json:"projectPath"`
	SU          string                                   `json:"su"`
	SUPath      string                                   `json:"suPath"`
	Values      LongVegetationStatisticsPreferenceValues `json:"values"`
}

type LongVegetationStatisticsPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationStatisticsPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationStatisticsPreferencesService(contexts *vprocore.ContextService) *LongVegetationStatisticsPreferencesService {
	return &LongVegetationStatisticsPreferencesService{contexts: contexts}
}

func (s *LongVegetationStatisticsPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationStatisticsPreferencesService) GetLongVegetationStatisticsPreferences(ctx context.Context, contextID string) (*LongVegetationStatisticsPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation statistics",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationStatisticsPreferencesReview, error) {
			values, err := ReadLongVegetationStatisticsPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationStatisticsPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values,
			}, nil
		})
}

func (s *LongVegetationStatisticsPreferencesService) SaveLongVegetationStatisticsPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationStatisticsPreferencesOutcome {
	out := LongVegetationStatisticsPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation statistics", nil,
		func(plots *vprocore.PlotService, request LongVegetationStatisticsPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationStatisticsPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 3. Long Vegetation Switch Preferences

type LongVegetationSwitchPreferenceValues struct {
	QuickReport         bool `json:"quickReport"`
	SpaceBetweenGroups  bool `json:"spaceBetweenGroups"`
	ReportSummary       bool `json:"reportSummary"`
	ConstantSpeciesList bool `json:"constantSpeciesList"`
}

func (v *LongVegetationSwitchPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSwitchPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preferences",
		"quickReport", "spaceBetweenGroups", "reportSummary", "constantSpeciesList"); err != nil {
		return err
	}
	*v = LongVegetationSwitchPreferenceValues(decoded)
	return nil
}

type LongVegetationSwitchPreferencesRequest struct {
	Expected LongVegetationSwitchPreferenceValues `json:"expected"`
	Proposed LongVegetationSwitchPreferenceValues `json:"proposed"`
}

func (v *LongVegetationSwitchPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSwitchPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation switch preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationSwitchPreferencesRequest(decoded)
	return nil
}

type LongVegetationSwitchPreferencesReview struct {
	ContextID   string                               `json:"contextId"`
	Project     string                               `json:"project"`
	ProjectPath string                               `json:"projectPath"`
	SU          string                               `json:"su"`
	SUPath      string                               `json:"suPath"`
	Values      LongVegetationSwitchPreferenceValues `json:"values"`
}

type LongVegetationSwitchPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationSwitchPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationSwitchPreferencesService(contexts *vprocore.ContextService) *LongVegetationSwitchPreferencesService {
	return &LongVegetationSwitchPreferencesService{contexts: contexts}
}

func (s *LongVegetationSwitchPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationSwitchPreferencesService) GetLongVegetationSwitchPreferences(ctx context.Context, contextID string) (*LongVegetationSwitchPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation switch",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationSwitchPreferencesReview, error) {
			values, err := ReadLongVegetationSwitchPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSwitchPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values,
			}, nil
		})
}

func (s *LongVegetationSwitchPreferencesService) SaveLongVegetationSwitchPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSwitchPreferencesOutcome {
	out := LongVegetationSwitchPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation switch", nil,
		func(plots *vprocore.PlotService, request LongVegetationSwitchPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationSwitchPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 4. Long Vegetation Threshold Preferences

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

func (s *LongVegetationThresholdPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationThresholdPreferencesService) GetLongVegetationThresholdPreferences(ctx context.Context, contextID string) (*LongVegetationThresholdPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation threshold",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationThresholdPreferencesReview, error) {
			values, err := ReadLongVegetationThresholdPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationThresholdPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values,
			}, nil
		})
}

func (s *LongVegetationThresholdPreferencesService) SaveLongVegetationThresholdPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationThresholdPreferencesOutcome {
	out := LongVegetationThresholdPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation threshold", nil,
		func(plots *vprocore.PlotService, request LongVegetationThresholdPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationThresholdPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 5. Long Vegetation Grouping Preferences

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

func (s *LongVegetationGroupPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationGroupPreferencesService) availability() LongVegetationGroupAvailability {
	return LongVegetationGroupAvailability{Layer: true, Strata: true, Lifeform: true, None: true}
}

func (s *LongVegetationGroupPreferencesService) GetLongVegetationGroupPreferences(ctx context.Context, contextID string) (*LongVegetationGroupPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation grouping",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationGroupPreferencesReview, error) {
			values, err := ReadLongVegetationGroupPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationGroupPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values, Availability: s.availability(),
			}, nil
		})
}

func (s *LongVegetationGroupPreferencesService) SaveLongVegetationGroupPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationGroupPreferencesOutcome {
	out := LongVegetationGroupPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation grouping", nil,
		func(plots *vprocore.PlotService, request LongVegetationGroupPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			if request.Expected.GroupBy != request.Proposed.GroupBy && !s.availability().allows(request.Proposed.GroupBy) {
				return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Long Vegetation grouping choice %d is unavailable; saved LVGroupBy was not reset", request.Proposed.GroupBy)
			}
			return CompareAndSetLongVegetationGroupPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 6. Long Vegetation Quality Include Nulls Preferences

type LongVegetationQualityNullPreferenceValues struct {
	SiteIncludeNulls bool `json:"siteIncludeNulls"`
	VegIncludeNulls  bool `json:"vegIncludeNulls"`
	SoilIncludeNulls bool `json:"soilIncludeNulls"`
}

func (v *LongVegetationQualityNullPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preferences",
		"siteIncludeNulls", "vegIncludeNulls", "soilIncludeNulls"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferenceValues(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesRequest struct {
	Expected LongVegetationQualityNullPreferenceValues `json:"expected"`
	Proposed LongVegetationQualityNullPreferenceValues `json:"proposed"`
}

func (v *LongVegetationQualityNullPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityNullPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality Include Nulls preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationQualityNullPreferencesRequest(decoded)
	return nil
}

type LongVegetationQualityNullPreferencesReview struct {
	ContextID   string                                    `json:"contextId"`
	Project     string                                    `json:"project"`
	ProjectPath string                                    `json:"projectPath"`
	SU          string                                    `json:"su"`
	SUPath      string                                    `json:"suPath"`
	Values      LongVegetationQualityNullPreferenceValues `json:"values"`
}

type LongVegetationQualityNullPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationQualityNullPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationQualityNullPreferencesService(contexts *vprocore.ContextService) *LongVegetationQualityNullPreferencesService {
	return &LongVegetationQualityNullPreferencesService{contexts: contexts}
}

func (s *LongVegetationQualityNullPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationQualityNullPreferencesService) GetLongVegetationQualityNullPreferences(ctx context.Context, contextID string) (*LongVegetationQualityNullPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation quality Include Nulls",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationQualityNullPreferencesReview, error) {
			values, err := ReadLongVegetationQualityNullPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityNullPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values,
			}, nil
		})
}

func (s *LongVegetationQualityNullPreferencesService) SaveLongVegetationQualityNullPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityNullPreferencesOutcome {
	out := LongVegetationQualityNullPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation quality Include Nulls", nil,
		func(plots *vprocore.PlotService, request LongVegetationQualityNullPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationQualityNullPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 7. Long Vegetation Quality Minimum Preferences

const longVegetationQualityMinimumSuggestionsSource = "configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite; ItemOrder then physical rowid; suggestions only"

type LongVegetationQualityMinimumPreferenceValues struct {
	SiteMinimum string `json:"siteMinimum"`
	VegMinimum  string `json:"vegMinimum"`
	SoilMinimum string `json:"soilMinimum"`
}

func (v *LongVegetationQualityMinimumPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityMinimumPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality minimum preferences",
		"siteMinimum", "vegMinimum", "soilMinimum"); err != nil {
		return err
	}
	*v = LongVegetationQualityMinimumPreferenceValues(decoded)
	return nil
}

type LongVegetationQualityMinimumPreferencesRequest struct {
	Expected LongVegetationQualityMinimumPreferenceValues `json:"expected"`
	Proposed LongVegetationQualityMinimumPreferenceValues `json:"proposed"`
}

func (v *LongVegetationQualityMinimumPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationQualityMinimumPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation quality minimum preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationQualityMinimumPreferencesRequest(decoded)
	return nil
}

type LongVegetationQualityMinimumPreferencesReview struct {
	ContextID   string                                       `json:"contextId"`
	Project     string                                       `json:"project"`
	ProjectPath string                                       `json:"projectPath"`
	SU          string                                       `json:"su"`
	SUPath      string                                       `json:"suPath"`
	Values      LongVegetationQualityMinimumPreferenceValues `json:"values"`
	Source      string                                       `json:"source"`
	Definitions vprocore.ProjectMetadataTable                `json:"definitions"`
}

type LongVegetationQualityMinimumPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationQualityMinimumPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationQualityMinimumPreferencesService(contexts *vprocore.ContextService) *LongVegetationQualityMinimumPreferencesService {
	return &LongVegetationQualityMinimumPreferencesService{contexts: contexts}
}

func (s *LongVegetationQualityMinimumPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func readLongVegetationQualityMinimumSuggestions(ctx context.Context, tx *sql.Tx) (vprocore.ProjectMetadataTable, error) {
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "VLists", "USysTableOfLists", "Long Vegetation quality minimum suggestions"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	table, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "VLists", "USysTableOfLists", "ListName", "PlotQualitySite", "ItemOrder")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if !reflect.DeepEqual(table.Columns, sourceform.SiviParentReferenceColumns) {
		return vprocore.ProjectMetadataTable{}, errors.New("Long Vegetation quality minimum suggestions require the exact ten-column imported schema")
	}
	return table, nil
}

func (s *LongVegetationQualityMinimumPreferencesService) GetLongVegetationQualityMinimumPreferences(ctx context.Context, contextID string) (*LongVegetationQualityMinimumPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation quality minimum",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationQualityMinimumPreferencesReview, error) {
			definitions, err := readLongVegetationQualityMinimumSuggestions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := ReadLongVegetationQualityMinimumPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityMinimumPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Source: longVegetationQualityMinimumSuggestionsSource, Definitions: definitions,
			}, nil
		})
}

func (s *LongVegetationQualityMinimumPreferencesService) SaveLongVegetationQualityMinimumPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityMinimumPreferencesOutcome {
	out := LongVegetationQualityMinimumPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation quality minimum", nil,
		func(plots *vprocore.PlotService, request LongVegetationQualityMinimumPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationQualityMinimumPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 8. Long Vegetation Naming (Show) Preferences

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

func (s *LongVegetationNamingPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationNamingPreferencesService) availability() LongVegetationNamingAvailability {
	return LongVegetationNamingAvailability{None: true, English: true, Code: true}
}

func (s *LongVegetationNamingPreferencesService) GetLongVegetationNamingPreferences(ctx context.Context, contextID string) (*LongVegetationNamingPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation Show",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationNamingPreferencesReview, error) {
			values, err := ReadLongVegetationNamingPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationNamingPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values, Availability: s.availability(),
			}, nil
		})
}

func (s *LongVegetationNamingPreferencesService) SaveLongVegetationNamingPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationNamingPreferencesOutcome {
	out := LongVegetationNamingPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation Show", nil,
		func(plots *vprocore.PlotService, request LongVegetationNamingPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			if request.Expected != request.Proposed && !s.availability().allows(request.Proposed.Mode) {
				return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Long Vegetation Show choice %d is unavailable on this control; saved naming values were not reset", request.Proposed.Mode)
			}
			return CompareAndSetLongVegetationNamingPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 9. Long Vegetation Selected Field Preferences

const longVegetationSelectedFieldDefinitionsSource = "configured-owned-imported-SQLite VLists.USysAllSpecs physical columns; source Value List suggestions are not mandatory membership"

type LongVegetationSelectedFieldPreferenceValues struct {
	Mode          int    `json:"mode"`
	SelectedField string `json:"selectedField"`
}

func (v *LongVegetationSelectedFieldPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSelectedFieldPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation selected field preferences", "mode", "selectedField"); err != nil {
		return err
	}
	if !longVegetationNamingMode(decoded.Mode) {
		return errors.New("Long Vegetation selected field requires source mode 0, 1, 2 or 4")
	}
	*v = LongVegetationSelectedFieldPreferenceValues(decoded)
	return nil
}

type LongVegetationSelectedFieldPreferencesRequest struct {
	Expected LongVegetationSelectedFieldPreferenceValues `json:"expected"`
	Proposed LongVegetationSelectedFieldPreferenceValues `json:"proposed"`
}

func (v *LongVegetationSelectedFieldPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationSelectedFieldPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation selected field preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationSelectedFieldPreferencesRequest(decoded)
	return nil
}

type LongVegetationSelectedFieldPreferencesReview struct {
	ContextID   string                                      `json:"contextId"`
	Project     string                                      `json:"project"`
	ProjectPath string                                      `json:"projectPath"`
	SU          string                                      `json:"su"`
	SUPath      string                                      `json:"suPath"`
	Values      LongVegetationSelectedFieldPreferenceValues `json:"values"`
	Source      string                                      `json:"source"`
	Definitions []vprocore.ProjectMetadataColumn            `json:"definitions"`
}

type LongVegetationSelectedFieldPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationSelectedFieldPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationSelectedFieldPreferencesService(contexts *vprocore.ContextService) *LongVegetationSelectedFieldPreferencesService {
	return &LongVegetationSelectedFieldPreferencesService{contexts: contexts}
}

func (s *LongVegetationSelectedFieldPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func readLongVegetationSelectedFieldDefinitions(ctx context.Context, tx *sql.Tx) ([]vprocore.ProjectMetadataColumn, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "VLists".sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
		"USysAllSpecs").Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, errors.New("Long Vegetation selected field definitions require original physical VLists.USysAllSpecs")
	}
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "VLists", "USysAllSpecs", "Long Vegetation selected field definitions"); err != nil {
		return nil, fmt.Errorf("Long Vegetation selected field definitions unavailable: %w", err)
	}
	return vprocore.ReadSQLiteStorageColumns(ctx, tx, "VLists", "USysAllSpecs")
}

func (s *LongVegetationSelectedFieldPreferencesService) GetLongVegetationSelectedFieldPreferences(ctx context.Context, contextID string) (*LongVegetationSelectedFieldPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation selected field",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationSelectedFieldPreferencesReview, error) {
			definitions, err := readLongVegetationSelectedFieldDefinitions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := ReadLongVegetationSelectedFieldPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSelectedFieldPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Source: longVegetationSelectedFieldDefinitionsSource, Definitions: definitions,
			}, nil
		})
}

func (s *LongVegetationSelectedFieldPreferencesService) SaveLongVegetationSelectedFieldPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSelectedFieldPreferencesOutcome {
	out := LongVegetationSelectedFieldPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation selected field", nil,
		func(plots *vprocore.PlotService, request LongVegetationSelectedFieldPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetLongVegetationSelectedFieldPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}

// 10. Long Vegetation Enforcement Preferences

type LongVegetationEnforcementPreferenceValues struct {
	Enforce    bool `json:"enforce"`
	UnitGroups int  `json:"unitGroups"`
}

func (v *LongVegetationEnforcementPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationEnforcementPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation enforcement preferences", "enforce", "unitGroups"); err != nil {
		return err
	}
	if decoded.UnitGroups < 1 || decoded.UnitGroups > 3 {
		return errors.New("Long Vegetation enforcement requires source unit-group 1, 2 or 3")
	}
	*v = LongVegetationEnforcementPreferenceValues(decoded)
	return nil
}

type LongVegetationEnforcementPreferencesRequest struct {
	Expected LongVegetationEnforcementPreferenceValues `json:"expected"`
	Proposed LongVegetationEnforcementPreferenceValues `json:"proposed"`
}

func (v *LongVegetationEnforcementPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationEnforcementPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation enforcement preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationEnforcementPreferencesRequest(decoded)
	return nil
}

type LongVegetationEnforcementPreferencesReview struct {
	ContextID   string                                    `json:"contextId"`
	Project     string                                    `json:"project"`
	ProjectPath string                                    `json:"projectPath"`
	SU          string                                    `json:"su"`
	SUPath      string                                    `json:"suPath"`
	Values      LongVegetationEnforcementPreferenceValues `json:"values"`
	Printers    vprocore.PrinterObservation               `json:"printers"`
}

type LongVegetationEnforcementPreferencesOutcome struct {
	ContextID    string                       `json:"contextId"`
	Changed      bool                         `json:"changed"`
	Committed    bool                         `json:"committed"`
	ErrorMessage string                       `json:"errorMessage"`
	Printers     *vprocore.PrinterObservation `json:"printers"`
}

type LongVegetationEnforcementPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
	printers func(context.Context) (vprocore.PrinterObservation, error)
}

func NewLongVegetationEnforcementPreferencesService(contexts *vprocore.ContextService) *LongVegetationEnforcementPreferencesService {
	return &LongVegetationEnforcementPreferencesService{contexts: contexts, printers: vprocore.ObservePrinters}
}

func (s *LongVegetationEnforcementPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil || s.printers == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *LongVegetationEnforcementPreferencesService) GetLongVegetationEnforcementPreferences(ctx context.Context, contextID string) (*LongVegetationEnforcementPreferencesReview, error) {
	h := s.helper()
	return vprocore.WithReportPreferencePlot(ctx, h, contextID, "Long Vegetation enforcement", func(plots *vprocore.PlotService) (*LongVegetationEnforcementPreferencesReview, error) {
		result, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, h.Snapshot, func(owner *vprocore.SqliteContext, _ *sql.Tx) (*LongVegetationEnforcementPreferencesReview, error) {
			values, err := ReadLongVegetationEnforcementPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationEnforcementPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: values,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		result.Printers, err = s.printers(ctx)
		if err != nil {
			return nil, fmt.Errorf("Long Vegetation enforcement printer observation unavailable; no values applied: %w", err)
		}
		if err := vprocore.ValidatePrinterObservation(ctx, result.Printers); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (s *LongVegetationEnforcementPreferencesService) SaveLongVegetationEnforcementPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationEnforcementPreferencesOutcome {
	var observedPrinters *vprocore.PrinterObservation
	receipt := vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Long Vegetation enforcement", nil,
		func(plots *vprocore.PlotService, request LongVegetationEnforcementPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			update, saveErr := CompareAndSetLongVegetationEnforcementPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			if saveErr != nil {
				return update, saveErr
			}
			printers, err := s.printers(ctx)
			if err == nil {
				err = vprocore.ValidatePrinterObservation(ctx, printers)
			}
			if err != nil {
				if update.Committed {
					return update, fmt.Errorf("Long Vegetation enforcement preferences were committed; do not replay the change; printer refresh unavailable: %w", err)
				}
				return update, fmt.Errorf("Long Vegetation enforcement preferences unchanged; printer refresh unavailable: %w", err)
			}
			observedPrinters = &printers
			return update, nil
		})
	return &LongVegetationEnforcementPreferencesOutcome{
		ContextID: receipt.ContextID, Changed: receipt.Changed, Committed: receipt.Committed,
		ErrorMessage: receipt.ErrorMessage, Printers: observedPrinters,
	}
}

// 11. Long Vegetation Report Unit Preferences

type LongVegetationUnitPreferenceValues struct {
	UnitGroups int `json:"unitGroups"`
}

func (v *LongVegetationUnitPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationUnitPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation report unit preferences", "unitGroups"); err != nil {
		return err
	}
	if decoded.UnitGroups < 1 || decoded.UnitGroups > 3 {
		return errors.New("Long Vegetation report units require source choice 1, 2 or 3")
	}
	*v = LongVegetationUnitPreferenceValues(decoded)
	return nil
}

type LongVegetationUnitPreferencesRequest struct {
	Expected LongVegetationUnitPreferenceValues `json:"expected"`
	Proposed LongVegetationUnitPreferenceValues `json:"proposed"`
}

func (v *LongVegetationUnitPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain LongVegetationUnitPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation report unit preference save", "expected", "proposed"); err != nil {
		return err
	}
	*v = LongVegetationUnitPreferencesRequest(decoded)
	return nil
}

type LongVegetationUnitAvailability struct {
	SiteUnitTable  bool `json:"siteUnitTable"`
	SelectedFields bool `json:"selectedFields"`
	None           bool `json:"none"`
}

func (a LongVegetationUnitAvailability) allows(group int) bool {
	switch group {
	case 1:
		return a.SiteUnitTable
	case 2:
		return a.SelectedFields
	case 3:
		return a.None
	default:
		return false
	}
}

type LongVegetationUnitPreferencesReview struct {
	ContextID    string                             `json:"contextId"`
	Project      string                             `json:"project"`
	ProjectPath  string                             `json:"projectPath"`
	SU           string                             `json:"su"`
	SUPath       string                             `json:"suPath"`
	Values       LongVegetationUnitPreferenceValues `json:"values"`
	Availability LongVegetationUnitAvailability     `json:"availability"`
}

type LongVegetationUnitPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type LongVegetationUnitPreferencesService struct {
	contexts              *vprocore.ContextService
	snapshot              vprocore.PublicationReadSnapshotHooks
	beforeSelectedUnitCAS func()
}

func NewLongVegetationUnitPreferencesService(contexts *vprocore.ContextService) *LongVegetationUnitPreferencesService {
	return &LongVegetationUnitPreferencesService{contexts: contexts}
}

func (s *LongVegetationUnitPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	var checkScope func(string) error
	if s.contexts != nil {
		checkScope = s.contexts.RequireSelectedListReportScope
	}
	return vprocore.ReportPreferenceHelper{
		Contexts:   s.contexts,
		Snapshot:   s.snapshot,
		CheckScope: checkScope,
	}
}

func (s *LongVegetationUnitPreferencesService) availability(selectedFields bool) LongVegetationUnitAvailability {
	return LongVegetationUnitAvailability{SiteUnitTable: true, SelectedFields: selectedFields, None: true}
}

func (s *LongVegetationUnitPreferencesService) GetLongVegetationUnitPreferences(ctx context.Context, contextID string) (*LongVegetationUnitPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Long Vegetation report unit",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationUnitPreferencesReview, error) {
			values, err := ReadLongVegetationUnitPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			selectedFields, err := SelectedUnitAvailability(s.contexts, ctx, plots, tx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationUnitPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Availability: s.availability(selectedFields),
			}, nil
		})
}

func (s *LongVegetationUnitPreferencesService) SaveLongVegetationUnitPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationUnitPreferencesOutcome {
	h := s.helper()
	out := LongVegetationUnitPreferencesOutcome(vprocore.RunReportPreferenceSave(ctx, h, contextID, requestJSON, "Long Vegetation report unit",
		func(plots *vprocore.PlotService, request LongVegetationUnitPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
				return vprocore.ReportPreferenceUpdate{}, err
			}
			if request.Expected.UnitGroups != 2 && request.Proposed.UnitGroups == 2 {
				var captured vprocore.ReportPreferenceUpdate
				_, snapErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, h.Snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (struct{}, error) {
					if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
						return struct{}{}, err
					}
					ledger := &s.contexts.SelectedUnitSelections
					ledger.Mu.Lock()
					defer ledger.Mu.Unlock()
					values, err := plots.Projects.Preferences.Snapshot()
					if err != nil {
						return struct{}{}, err
					}
					if _, err := validateSelectedUnitApproval(ctx, contextID, owner, tx, values, ledger.Entries[contextID]); err != nil {
						return struct{}{}, err
					}
					planned, err := selectedUnitConfigForChoice(values, 2)
					if err != nil {
						return struct{}{}, err
					}
					if s.beforeSelectedUnitCAS != nil {
						s.beforeSelectedUnitCAS()
					}
					update, err := CompareAndSetReviewedLongVegetationUnitPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed, func(current vprocore.ConfigValues) error {
						currentPlan, err := selectedUnitConfigForChoice(current, 2)
						if err != nil {
							return err
						}
						if !reflect.DeepEqual(planned, currentPlan) {
							return errors.New("selected report unit configuration changed since Build; rebuild explicitly before Save")
						}
						return nil
					})
					captured = update
					return struct{}{}, err
				})
				return captured, snapErr
			}
			if _, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, h.Snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			}); readErr != nil {
				return vprocore.ReportPreferenceUpdate{}, readErr
			}
			owner := plots.Projects.Sqlite
			if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
				return vprocore.ReportPreferenceUpdate{}, err
			}
			defer owner.Mu.Unlock()
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return vprocore.ReportPreferenceUpdate{}, err
			}
			if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
				return vprocore.ReportPreferenceUpdate{}, err
			}
			if request.Expected.UnitGroups != request.Proposed.UnitGroups && !s.availability(false).allows(request.Proposed.UnitGroups) {
				return vprocore.ReportPreferenceUpdate{}, fmt.Errorf("Long Vegetation report unit choice %d is unavailable in this session; saved LVUnitGroups was not reset", request.Proposed.UnitGroups)
			}
			return CompareAndSetLongVegetationUnitPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
		}))
	return &out
}
