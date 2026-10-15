package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type LongVegetationProjectUnitPreferences struct {
	Scope       LongVegetationReportScope           `json:"scope"`
	Preferences LongVegetationUnitPreferencesReview `json:"preferences"`
}

func (s *LongVegetationReportScopeService) GetLongVegetationProjectUnitPreferences(ctx context.Context,
	contextID, requestJSON string) (LongVegetationProjectUnitPreferences, error) {
	if err := s.require(ctx, contextID); err != nil {
		return LongVegetationProjectUnitPreferences{}, err
	}
	var request vegetationProjectReadRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return LongVegetationProjectUnitPreferences{}, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (LongVegetationProjectUnitPreferences, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (LongVegetationProjectUnitPreferences, error) {
			config, err := plots.projects.preferences.snapshot()
			if err != nil {
				return LongVegetationProjectUnitPreferences{}, err
			}
			entry, err := s.contexts.approvedProjectScope(ctx, contextID, owner, tx, config)
			if err != nil {
				return LongVegetationProjectUnitPreferences{}, err
			}
			scope := projectScopeDTO(contextID, owner, entry)
			if request.Revision != scope.Revision || request.ApprovalHash != entry.build.ApprovalHash {
				return LongVegetationProjectUnitPreferences{}, errors.New("project unit preference review scope/revision/Build changed; reload before Save")
			}
			group, err := longVegetationUnitOption(config, "LVUnitGroups")
			if err != nil {
				return LongVegetationProjectUnitPreferences{}, err
			}
			scope.BuildCurrent = true
			return LongVegetationProjectUnitPreferences{
				Scope: *scope,
				Preferences: LongVegetationUnitPreferencesReview{
					ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
					SU: owner.selection.SU, SUPath: owner.selection.SUPath,
					Values: LongVegetationUnitPreferenceValues{UnitGroups: group},
					Availability: LongVegetationUnitAvailability{
						SelectedFields: entry.sourceKind == vegetationProjectFieldsScope,
						None:           entry.sourceKind == vegetationProjectAllScope,
					},
				},
			}, nil
		})
	})
}
