package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const longVegetationEnforcementPreferencesFeatureEnvironment = "VPRO_LONG_VEGETATION_ENFORCEMENT_PREFERENCES"

func longVegetationEnforcementPreferencesFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(longVegetationEnforcementPreferencesFeatureEnvironment, lookup)
}

type LongVegetationEnforcementPreferenceValues struct {
	Enforce    bool `json:"enforce"`
	UnitGroups int  `json:"unitGroups"`
}

func (v *LongVegetationEnforcementPreferenceValues) UnmarshalJSON(data []byte) error {
	type plain LongVegetationEnforcementPreferenceValues
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation enforcement preferences", "enforce", "unitGroups"); err != nil {
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
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation enforcement preference save", "expected", "proposed"); err != nil {
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
	Printers    PrinterObservation                        `json:"printers"`
}

type LongVegetationEnforcementPreferencesOutcome struct {
	ContextID    string              `json:"contextId"`
	Changed      bool                `json:"changed"`
	Committed    bool                `json:"committed"`
	ErrorMessage string              `json:"errorMessage"`
	Printers     *PrinterObservation `json:"printers"`
}

type LongVegetationEnforcementPreferencesService struct {
	contexts *ContextService
	enabled  bool
	snapshot publicationReadSnapshotHooks
	printers func(context.Context) (PrinterObservation, error)
}

func NewLongVegetationEnforcementPreferencesService(contexts *ContextService, enabled bool) *LongVegetationEnforcementPreferencesService {
	return &LongVegetationEnforcementPreferencesService{contexts: contexts, enabled: enabled, printers: observePrinters}
}

func (s *LongVegetationEnforcementPreferencesService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation enforcement preferences require a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation enforcement preferences are disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil || s.contexts.plots == nil || s.printers == nil {
		return errors.New("Long Vegetation enforcement preference context or printer observer is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func (s *LongVegetationEnforcementPreferencesService) GetLongVegetationEnforcementPreferences(ctx context.Context, contextID string) (*LongVegetationEnforcementPreferencesReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationEnforcementPreferencesReview, error) {
		result, err := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, _ *sql.Tx) (*LongVegetationEnforcementPreferencesReview, error) {
			values, err := plots.projects.preferences.readLongVegetationEnforcementPreferences(ctx)
			if err != nil {
				return nil, err
			}
			return &LongVegetationEnforcementPreferencesReview{ContextID: contextID, Project: owner.selection.Project,
				ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath, Values: values}, nil
		})
		if err != nil {
			return nil, err
		}
		result.Printers, err = s.printers(ctx)
		if err != nil {
			return nil, fmt.Errorf("Long Vegetation enforcement printer observation unavailable; no values applied: %w", err)
		}
		if err := validatePrinterObservation(ctx, result.Printers); err != nil {
			return nil, err
		}
		return result, nil
	})
}

// A failed host refresh cannot erase a committed paired preference receipt.
func (s *LongVegetationEnforcementPreferencesService) SaveLongVegetationEnforcementPreferences(ctx context.Context, contextID, requestJSON string) *LongVegetationEnforcementPreferencesOutcome {
	outcome := &LongVegetationEnforcementPreferencesOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request LongVegetationEnforcementPreferencesRequest
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
			update, saveErr := plots.projects.preferences.compareAndSetLongVegetationEnforcementPreferences(ctx, request.Expected, request.Proposed)
			outcome.Changed, outcome.Committed = update.Changed, update.Committed
			if saveErr != nil {
				return struct{}{}, saveErr
			}
			printers, err := s.printers(ctx)
			if err == nil {
				err = validatePrinterObservation(ctx, printers)
			}
			if err != nil {
				if update.Committed {
					return struct{}{}, fmt.Errorf("Long Vegetation enforcement preferences were committed; do not replay the change; printer refresh unavailable: %w", err)
				}
				return struct{}{}, fmt.Errorf("Long Vegetation enforcement preferences unchanged; printer refresh unavailable: %w", err)
			}
			outcome.Printers = &printers
			return struct{}{}, nil
		})
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
