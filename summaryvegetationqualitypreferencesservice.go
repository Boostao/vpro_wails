package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

const summaryVegetationQualityPreferencesFeatureEnvironment = "VPRO_SUMMARY_VEGETATION_QUALITY_PREFERENCES"
const summaryVegetationQualitySuggestionsSource = "configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite and BEC_Use; ItemOrder then physical rowid; suggestions only, not DataQuality qualification"

func summaryVegetationQualityPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(summaryVegetationQualityPreferencesFeatureEnvironment, lookup)
}

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
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Vegetation quality preferences",
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
	if err := decodeStrictRequiredJSON(data, &decoded, "Summary Vegetation quality preference Save", "expected", "proposed"); err != nil {
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
	QualityDefinitions ProjectMetadataTable                     `json:"qualityDefinitions"`
	BECDefinitions     ProjectMetadataTable                     `json:"becDefinitions"`
}

type SummaryVegetationQualityPreferencesOutcome struct {
	ContextID    string `json:"contextId"`
	Changed      bool   `json:"changed"`
	Committed    bool   `json:"committed"`
	ErrorMessage string `json:"errorMessage"`
}

type SummaryVegetationQualityPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
}

func NewSummaryVegetationQualityPreferencesService(contexts *ContextService, enabled bool) *SummaryVegetationQualityPreferencesService {
	return &SummaryVegetationQualityPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *SummaryVegetationQualityPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary Vegetation quality preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Summary Vegetation quality preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Summary Vegetation quality preference context service is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func readSummaryVegetationBECSuggestions(ctx context.Context, tx *sql.Tx) (ProjectMetadataTable, error) {
	if err := validateSIVIPhysicalSchema(ctx, tx, "VLists", "USysTableOfLists", "Summary Vegetation BEC minimum suggestions"); err != nil {
		return ProjectMetadataTable{}, err
	}
	table, err := readSQLiteLiteralTextRows(ctx, tx, "VLists", "USysTableOfLists", "ListName", "BEC_Use", "ItemOrder")
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	if !reflect.DeepEqual(table.Columns, siviParentReferenceColumns) {
		return ProjectMetadataTable{}, errors.New("Summary Vegetation BEC minimum suggestions require the exact ten-column imported schema")
	}
	return table, nil
}

func (s *SummaryVegetationQualityPreferencesService) GetSummaryVegetationQualityPreferences(ctx context.Context, contextID string) (*SummaryVegetationQualityPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*SummaryVegetationQualityPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*SummaryVegetationQualityPreferencesReview, error) {
			// Both forms use the identical PlotQualitySite suggestion scope.
			quality, err := readLongVegetationQualityMinimumSuggestions(ctx, tx)
			if err != nil {
				return nil, fmt.Errorf("Summary Vegetation quality suggestions: %w", err)
			}
			bec, err := readSummaryVegetationBECSuggestions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := plots.projects.preferences.readSummaryVegetationQualityPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationQualityPreferencesReview{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: SummaryVegetationQualityPreferenceValues(values),
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
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*sqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			update, saveErr := plots.projects.preferences.compareAndSetSummaryVegetationQualityPreferences(ctx,
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
