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
	"reflect"
)

const longVegetationQualityMinimumPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES"
const longVegetationQualityMinimumSuggestionsSource = "configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite; ItemOrder then physical rowid; suggestions only"

func LongVegetationQualityMinimumPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(longVegetationQualityMinimumPreferencesFeatureEnvironment, lookup)
}

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
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationQualityMinimumPreferencesService(contexts *vprocore.ContextService, enabled bool) *LongVegetationQualityMinimumPreferencesService {
	return &LongVegetationQualityMinimumPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationQualityMinimumPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation quality minimum preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation quality minimum preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation quality minimum preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
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
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationQualityMinimumPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationQualityMinimumPreferencesReview, error) {
			definitions, err := readLongVegetationQualityMinimumSuggestions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := ReadLongVegetationQualityMinimumPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationQualityMinimumPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Source: longVegetationQualityMinimumSuggestionsSource, Definitions: definitions}, nil
		})
	})
}

// Persistence receipts survive request cleanup errors.
func (s *LongVegetationQualityMinimumPreferencesService) SaveLongVegetationQualityMinimumPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationQualityMinimumPreferencesOutcome {
	outcome := &LongVegetationQualityMinimumPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationQualityMinimumPreferencesRequest
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
			update, saveErr := CompareAndSetLongVegetationQualityMinimumPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
