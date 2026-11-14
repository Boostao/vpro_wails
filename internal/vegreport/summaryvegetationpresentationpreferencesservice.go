package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

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

func (s *SummaryVegetationPresentationPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary Vegetation presentation preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Summary Vegetation presentation preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *SummaryVegetationPresentationPreferencesService) GetSummaryVegetationPresentationPreferences(ctx context.Context, contextID string) (*SummaryVegetationPresentationPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationPresentationPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, _ *sql.Tx) (*SummaryVegetationPresentationPreferencesReview, error) {
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
	})
}

func (s *SummaryVegetationPresentationPreferencesService) SaveSummaryVegetationPresentationPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationPresentationPreferencesOutcome {
	outcome := &SummaryVegetationPresentationPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SummaryVegetationPresentationPreferencesRequest
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
			if request.Expected.HierarchyHeader != request.Proposed.HierarchyHeader {
				return struct{}{}, errors.New("Summary Vegetation hierarchy-header editing requires unavailable hierarchy report scope; unchanged historical settings are retained")
			}
			update, saveErr := CompareAndSetSummaryVegetationPresentationPreferences(plots.Projects.Preferences, ctx,
				summaryVegetationPresentationPreferenceValues(request.Expected), summaryVegetationPresentationPreferenceValues(request.Proposed),
				request.ConstancyApplied)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
