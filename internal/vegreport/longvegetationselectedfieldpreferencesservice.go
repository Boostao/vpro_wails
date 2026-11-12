package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const longVegetationSelectedFieldPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES"
const longVegetationSelectedFieldDefinitionsSource = "configured-owned-imported-SQLite VLists.USysAllSpecs physical columns; source Value List suggestions are not mandatory membership"

func LongVegetationSelectedFieldPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(longVegetationSelectedFieldPreferencesFeatureEnvironment, lookup)
}

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
	enabled  bool
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewLongVegetationSelectedFieldPreferencesService(contexts *vprocore.ContextService, enabled bool) *LongVegetationSelectedFieldPreferencesService {
	return &LongVegetationSelectedFieldPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationSelectedFieldPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation selected field preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation selected field preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation selected field preference context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
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
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationSelectedFieldPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationSelectedFieldPreferencesReview, error) {
			definitions, err := readLongVegetationSelectedFieldDefinitions(ctx, tx)
			if err != nil {
				return nil, err
			}
			values, err := ReadLongVegetationSelectedFieldPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationSelectedFieldPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Source: longVegetationSelectedFieldDefinitionsSource, Definitions: definitions}, nil
		})
	})
}

// Persistence receipts survive request cleanup errors.
func (s *LongVegetationSelectedFieldPreferencesService) SaveLongVegetationSelectedFieldPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationSelectedFieldPreferencesOutcome {
	outcome := &LongVegetationSelectedFieldPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationSelectedFieldPreferencesRequest
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
			update, saveErr := CompareAndSetLongVegetationSelectedFieldPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, saveErr
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
