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
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (LongVegetationProjectUnitPreferences, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (LongVegetationProjectUnitPreferences, error) {
			config, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return LongVegetationProjectUnitPreferences{}, err
			}
			entry, err := ApprovedProjectScope(s.contexts, ctx, contextID, owner, tx, config)
			if err != nil {
				return LongVegetationProjectUnitPreferences{}, err
			}
			scope := projectScopeDTO(contextID, owner, entry)
			if request.Revision != scope.Revision || request.ApprovalHash != entry.Build.ApprovalHash {
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
					ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
					SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
					Values: LongVegetationUnitPreferenceValues{UnitGroups: group},
					Availability: LongVegetationUnitAvailability{
						SelectedFields: entry.SourceKind == vprocore.VegetationProjectFieldsScope,
						None:           entry.SourceKind == vprocore.VegetationProjectAllScope,
					},
				},
			}, nil
		})
	})
}
