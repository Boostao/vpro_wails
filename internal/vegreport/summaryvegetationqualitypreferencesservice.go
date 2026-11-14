package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

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

func (s *SummaryVegetationQualityPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary Vegetation quality preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Summary Vegetation quality preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
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
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationQualityPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationQualityPreferencesReview, error) {
			// Both forms use the identical PlotQualitySite suggestion scope.
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
	})
}

func (s *SummaryVegetationQualityPreferencesService) SaveSummaryVegetationQualityPreferences(ctx context.Context, contextID, requestJSON string) *SummaryVegetationQualityPreferencesOutcome {
	outcome := &SummaryVegetationQualityPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SummaryVegetationQualityPreferencesRequest
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
			update, saveErr := CompareAndSetSummaryVegetationQualityPreferences(plots.Projects.Preferences, ctx,
				summaryVegetationQualityPreferenceValues(request.Expected), summaryVegetationQualityPreferenceValues(request.Proposed))
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
