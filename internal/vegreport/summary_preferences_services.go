package vegreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// 1. Summary Vegetation Core Preferences

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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation preferences",
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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation preference Save", "expected", "proposed"); err != nil {
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
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationPreferencesService(contexts *vprocore.ContextService) *SummaryVegetationPreferencesService {
	return &SummaryVegetationPreferencesService{contexts: contexts}
}

func (s *SummaryVegetationPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func summaryVegetationPreferenceSelectionAvailable(value SummaryVegetationPreferenceValues) bool {
	return !((value.GroupBy == 2 || value.GroupBy == 3) && value.Display == 2)
}

func (s *SummaryVegetationPreferencesService) GetSummaryVegetationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Summary Vegetation",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*SummaryVegetationPreferencesReview, error) {
			values, err := ReadSummaryVegetationPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			dto := SummaryVegetationPreferenceValues(values)
			return &SummaryVegetationPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: dto, PreviewEnabled: true, SelectionAvailable: summaryVegetationPreferenceSelectionAvailable(dto),
			}, nil
		})
}

func (s *SummaryVegetationPreferencesService) SaveSummaryVegetationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPreferencesOutcome {
	out := SummaryVegetationPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Summary Vegetation", nil,
		func(plots *vprocore.PlotService, request SummaryVegetationPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			if request.Expected != request.Proposed && !summaryVegetationPreferenceSelectionAvailable(request.Proposed) {
				return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation selection is unavailable in this session; saved values were not reset")
			}
			return CompareAndSetSummaryVegetationPreferences(plots.Projects.Preferences, ctx,
				summaryVegetationPreferenceValues(request.Expected), summaryVegetationPreferenceValues(request.Proposed))
		}))
	return &out
}

// 2. Summary Vegetation Quality Preferences

const summaryVegetationQualitySuggestionsSource = "configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite and BEC_Use; ItemOrder then physical rowid; suggestions only, not DataQuality qualification"

type SummaryVegetationQualityPreferenceValues struct {
	Enforce          bool   `json:"enforce"`
	SiteMinimum      string `json:"siteMinimum"`
	VegMinimum       string `json:"vegMinimum"`
	SoilMinimum      string `json:"soilMinimum"`
	BECMinimum       string `json:"becMinimum"`
	SiteIncludeNulls bool   `json:"siteIncludeNulls"`
	VegIncludeNulls  bool   `json:"vegIncludeNulls"`
	SoilIncludeNulls bool   `json:"soilIncludeNulls"`
	BECIncludeNulls  bool   `json:"becIncludeNulls"`
}

func (v *SummaryVegetationQualityPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationQualityPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation quality preferences",
		"enforce", "siteMinimum", "vegMinimum", "soilMinimum", "becMinimum",
		"siteIncludeNulls", "vegIncludeNulls", "soilIncludeNulls", "becIncludeNulls"); err != nil {
		return err
	}
	*v = SummaryVegetationQualityPreferenceValues(decoded)
	return nil
}

type SummaryVegetationQualityPreferencesRequest struct {
	Expected SummaryVegetationQualityPreferenceValues `json:"expected"`
	Proposed SummaryVegetationQualityPreferenceValues `json:"proposed"`
}

func (v *SummaryVegetationQualityPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationQualityPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation quality preference Save", "expected", "proposed"); err != nil {
		return err
	}
	*v = SummaryVegetationQualityPreferencesRequest(decoded)
	return nil
}

type SummaryVegetationQualityPreferencesReview struct {
	ContextID          string                                   `json:"contextId"`
	Project            string                                   `json:"project"`
	ProjectPath        string                                   `json:"projectPath"`
	SU                 string                                   `json:"su"`
	SUPath             string                                   `json:"suPath"`
	Values             SummaryVegetationQualityPreferenceValues `json:"values"`
	Source             string                                   `json:"source"`
	QualityDefinitions vprocore.ProjectMetadataTable            `json:"qualityDefinitions"`
	BECDefinitions     vprocore.ProjectMetadataTable            `json:"becDefinitions"`
}

type SummaryVegetationQualityPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type SummaryVegetationQualityPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationQualityPreferencesService(contexts *vprocore.ContextService) *SummaryVegetationQualityPreferencesService {
	return &SummaryVegetationQualityPreferencesService{contexts: contexts}
}

func (s *SummaryVegetationQualityPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func readSummaryVegetationBECSuggestions(ctx context.Context, tx *sql.Tx) (vprocore.ProjectMetadataTable, error) {
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "VLists", "USysTableOfLists", "Summary Vegetation BEC minimum suggestions"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	table, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "VLists", "USysTableOfLists", "ListName", "BEC_Use", "ItemOrder")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	if !reflect.DeepEqual(table.Columns, sourceform.SiviParentReferenceColumns) {
		return vprocore.ProjectMetadataTable{}, errors.New("Summary Vegetation BEC minimum suggestions require the exact ten-column imported schema")
	}
	return table, nil
}

func (s *SummaryVegetationQualityPreferencesService) GetSummaryVegetationQualityPreferences(ctx context.Context, contextID string) (*SummaryVegetationQualityPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Summary Vegetation quality",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationQualityPreferencesReview, error) {
			quality, err := readLongVegetationQualityMinimumSuggestions(ctx, tx)
			if err != nil {
				return nil, fmt.Errorf("Summary Vegetation quality suggestions: %w", err)
			}
			bec, err := readSummaryVegetationBECSuggestions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := ReadSummaryVegetationQualityPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationQualityPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: SummaryVegetationQualityPreferenceValues(values),
				Source: summaryVegetationQualitySuggestionsSource, QualityDefinitions: quality, BECDefinitions: bec,
			}, nil
		})
}

func (s *SummaryVegetationQualityPreferencesService) SaveSummaryVegetationQualityPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationQualityPreferencesOutcome {
	out := SummaryVegetationQualityPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Summary Vegetation quality", nil,
		func(plots *vprocore.PlotService, request SummaryVegetationQualityPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			return CompareAndSetSummaryVegetationQualityPreferences(plots.Projects.Preferences, ctx,
				summaryVegetationQualityPreferenceValues(request.Expected), summaryVegetationQualityPreferenceValues(request.Proposed))
		}))
	return &out
}

// 3. Summary Vegetation Presentation Preferences

const summaryVegetationPresentationPreferencesSource = "owned native YAML saved Summary presentation; explicit Save adaptation; constancy-on forces Group by3/Display1/common names off without clearing limit; no guide renames, field-list writes or report-consumer parity"

type SummaryVegetationPresentationPreferenceValues struct {
	Title           string  `json:"title"`
	ShowCommon      bool    `json:"showCommon"`
	ShowGuide       bool    `json:"showGuide"`
	HierarchyHeader bool    `json:"hierarchyHeader"`
	IncludeEnv      bool    `json:"includeEnv"`
	CreateSummary   bool    `json:"createSummary"`
	CombineVariants bool    `json:"combineVariants"`
	Constancy       bool    `json:"constancy"`
	GroupBy         int     `json:"groupBy"`
	Display         int     `json:"display"`
	ValueLimit      float64 `json:"valueLimit"`
}

func (v *SummaryVegetationPresentationPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationPresentationPreferenceValues
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation presentation preferences",
		"title", "showCommon", "showGuide", "hierarchyHeader", "includeEnv", "createSummary",
		"combineVariants", "constancy", "groupBy", "display", "valueLimit"); err != nil {
		return err
	}
	*v = SummaryVegetationPresentationPreferenceValues(decoded)
	return nil
}

type SummaryVegetationPresentationPreferencesRequest struct {
	Expected         SummaryVegetationPresentationPreferenceValues `json:"expected"`
	Proposed         SummaryVegetationPresentationPreferenceValues `json:"proposed"`
	ConstancyApplied bool                                          `json:"constancyApplied"`
}

func (v *SummaryVegetationPresentationPreferencesRequest) UnmarshalJSON(data []byte) error {
	type plain SummaryVegetationPresentationPreferencesRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Summary Vegetation presentation preference Save",
		"expected", "proposed", "constancyApplied"); err != nil {
		return err
	}
	*v = SummaryVegetationPresentationPreferencesRequest(decoded)
	return nil
}

type SummaryVegetationPresentationPreferencesReview struct {
	ContextID   string                                        `json:"contextId"`
	Project     string                                        `json:"project"`
	ProjectPath string                                        `json:"projectPath"`
	SU          string                                        `json:"su"`
	SUPath      string                                        `json:"suPath"`
	Values      SummaryVegetationPresentationPreferenceValues `json:"values"`
	Source      string                                        `json:"source"`
}

type SummaryVegetationPresentationPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type SummaryVegetationPresentationPreferencesService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationPresentationPreferencesService(contexts *vprocore.ContextService) *SummaryVegetationPresentationPreferencesService {
	return &SummaryVegetationPresentationPreferencesService{contexts: contexts}
}

func (s *SummaryVegetationPresentationPreferencesService) helper() vprocore.ReportPreferenceHelper {
	if s == nil {
		return vprocore.ReportPreferenceHelper{}
	}
	return vprocore.ReportPreferenceHelper{Contexts: s.contexts, Snapshot: s.snapshot}
}

func (s *SummaryVegetationPresentationPreferencesService) GetSummaryVegetationPresentationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPresentationPreferencesReview, error) {
	return vprocore.GetOwnedReportPreferences(ctx, s.helper(), contextID, "Summary Vegetation presentation",
		func(plots *vprocore.PlotService, owner *vprocore.SqliteContext, _ *sql.Tx) (*SummaryVegetationPresentationPreferencesReview, error) {
			values, err := ReadSummaryVegetationPresentationPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationPresentationPreferencesReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Values: SummaryVegetationPresentationPreferenceValues(values),
				Source: summaryVegetationPresentationPreferencesSource,
			}, nil
		})
}

func (s *SummaryVegetationPresentationPreferencesService) SaveSummaryVegetationPresentationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPresentationPreferencesOutcome {
	out := SummaryVegetationPresentationPreferencesOutcome(vprocore.SaveOwnedReportPreferences(ctx, s.helper(), contextID, requestJSON, "Summary Vegetation presentation", nil,
		func(plots *vprocore.PlotService, request SummaryVegetationPresentationPreferencesRequest) (vprocore.ReportPreferenceUpdate, error) {
			if request.Expected.HierarchyHeader != request.Proposed.HierarchyHeader {
				return vprocore.ReportPreferenceUpdate{}, errors.New("Summary Vegetation hierarchy-header editing requires unavailable hierarchy report scope; unchanged historical settings are retained")
			}
			return CompareAndSetSummaryVegetationPresentationPreferences(plots.Projects.Preferences, ctx,
				summaryVegetationPresentationPreferenceValues(request.Expected), summaryVegetationPresentationPreferenceValues(request.Proposed),
				request.ConstancyApplied)
		}))
	return &out
}
