package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

const longVegetationUnitPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_UNIT_PREFERENCES"

func longVegetationUnitPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationUnitPreferencesFeatureEnvironment, lookup)
}

type LongVegetationUnitPreferenceValues struct {
	UnitGroups int `json:"unitGroups"`
}

func (v *LongVegetationUnitPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationUnitPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation report unit preferences", "unitGroups"); err != nil {
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
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation report unit preference save", "expected", "proposed"); err != nil {
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
	contexts              *ContextService
	enabled               bool
	snapshot              publicationReadSnapshotHooks
	beforeSelectedUnitCAS func()
}

func NewLongVegetationUnitPreferencesService(contexts *ContextService, enabled bool) *LongVegetationUnitPreferencesService {
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
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation report unit preference context service is unavailable")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return err
	}
	return s.contexts.requireSelectedListReportScope(contextID)
}

func (s *LongVegetationUnitPreferencesService) availability(selectedFields bool) LongVegetationUnitAvailability {
	return LongVegetationUnitAvailability{SiteUnitTable: true, SelectedFields: selectedFields, None: s.contexts.longVegetationNoneUnitsEnabled}
}

func (s *LongVegetationUnitPreferencesService) GetLongVegetationUnitPreferences(ctx context.Context, contextID string) (*LongVegetationUnitPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationUnitPreferencesReview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationUnitPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationUnitPreferences(ctx)
			if err != nil {
				return nil, err
			}
			selectedFields, err := s.contexts.selectedUnitAvailability(ctx, plots, tx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationUnitPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
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
		_, err = withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (struct{}, error) {
			if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
				return struct{}{}, err
			}
			if request.Expected.UnitGroups != 2 && request.Proposed.UnitGroups == 2 {
				if !s.contexts.longVegetationSelectedUnitsEnabled {
					return struct{}{}, errors.New("Long Vegetation selected report units are disabled in this session")
				}
				return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (struct{}, error) {
					if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
						return struct{}{}, err
					}
					ledger := &s.contexts.selectedUnitSelections
					ledger.mu.Lock()
					defer ledger.mu.Unlock()
					values, err := plots.projects.preferences.snapshot()
					if err != nil {
						return struct{}{}, err
					}
					if _, err := validateSelectedUnitApproval(ctx, contextID, owner, tx, values, ledger.entries[contextID]); err != nil {
						return struct{}{}, err
					}
					planned, err := selectedUnitConfigForChoice(values, 2)
					if err != nil {
						return struct{}{}, err
					}
					if s.beforeSelectedUnitCAS != nil {
						s.beforeSelectedUnitCAS()
					}
					update, err := plots.projects.preferences.compareAndSetReviewedLongVegetationUnitPreferences(ctx, request.Expected, request.Proposed, func(current configValues) error {
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
			_, readErr := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(*sqliteContext, *sql.Tx) (struct{}, error) {
				return struct{}{}, nil
			})
			if readErr != nil {
				return struct{}{}, readErr
			}
			owner := plots.projects.sqlite
			if err := acquireMutexLease(ctx, &owner.mu); err != nil {
				return struct{}{}, err
			}
			defer owner.mu.Unlock()
			if err := profileOwnedFiles(owner); err != nil {
				return struct{}{}, err
			}
			if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
				return struct{}{}, err
			}
			if request.Expected.UnitGroups != request.Proposed.UnitGroups && !s.availability(false).allows(request.Proposed.UnitGroups) {
				return struct{}{}, fmt.Errorf("Long Vegetation report unit choice %d is unavailable in this session; saved LVUnitGroups was not reset", request.Proposed.UnitGroups)
			}
			update, err := plots.projects.preferences.compareAndSetLongVegetationUnitPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
