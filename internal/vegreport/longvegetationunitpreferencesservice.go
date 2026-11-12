package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
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

const longVegetationUnitPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_UNIT_PREFERENCES"

func LongVegetationUnitPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return vprocore.SiviFeature(longVegetationUnitPreferencesFeatureEnvironment, lookup)
}

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
	enabled               bool
	snapshot              vprocore.PublicationReadSnapshotHooks
	beforeSelectedUnitCAS func()
}

func NewLongVegetationUnitPreferencesService(contexts *vprocore.ContextService, enabled bool) *LongVegetationUnitPreferencesService {
	return &LongVegetationUnitPreferencesService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationUnitPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation report unit preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation report unit preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation report unit preference context service is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return err
	}
	return s.contexts.RequireSelectedListReportScope(contextID)
}

func (s *LongVegetationUnitPreferencesService) availability(selectedFields bool) LongVegetationUnitAvailability {
	return LongVegetationUnitAvailability{SiteUnitTable: true, SelectedFields: selectedFields, None: s.contexts.LongVegetationNoneUnitsEnabled}
}

func (s *LongVegetationUnitPreferencesService) GetLongVegetationUnitPreferences(ctx context.Context, contextID string) (*LongVegetationUnitPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationUnitPreferencesReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationUnitPreferencesReview, error) {
			values, err := ReadLongVegetationUnitPreferences(plots.Projects.Preferences, ctx)
			if err != nil {
				return nil, err
			}
			selectedFields, err := SelectedUnitAvailability(s.contexts, ctx, plots, tx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationUnitPreferencesReview{ContextID: contextID, Project: owner.Selection.Project,
				ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
				Values: values, Availability: s.availability(selectedFields)}, nil
		})
	})
}

func (s *LongVegetationUnitPreferencesService) SaveLongVegetationUnitPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationUnitPreferencesOutcome {
	outcome := &LongVegetationUnitPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationUnitPreferencesRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
				return struct{}{}, err
			}
			if request.Expected.UnitGroups != 2 && request.Proposed.UnitGroups == 2 {
				if !s.contexts.LongVegetationSelectedUnitsEnabled {
					return struct{}{}, errors.New("Long Vegetation selected report units are disabled in this session")
				}
				return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (struct{}, error) {
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
					outcome.Changed, outcome.Committed = update.Changed, update.Committed
					return struct{}{}, err
				})
			}
			_, readErr := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			owner := plots.Projects.Sqlite
			if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
				return struct{}{}, err
			}
			defer owner.Mu.Unlock()
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return struct{}{}, err
			}
			if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
				return struct{}{}, err
			}
			if request.Expected.UnitGroups != request.Proposed.UnitGroups && !s.availability(false).allows(request.Proposed.UnitGroups) {
				return struct{}{}, fmt.Errorf("Long Vegetation report unit choice %d is unavailable in this session; saved LVUnitGroups was not reset", request.Proposed.UnitGroups)
			}
			update, err := CompareAndSetLongVegetationUnitPreferences(plots.Projects.Preferences, ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
