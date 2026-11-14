package vegreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type vegetationProjectUnitRequest struct {
	Revision     string                             `json:"revision"`
	ApprovalHash string                             `json:"approvalHash"`
	Expected     LongVegetationUnitPreferenceValues `json:"expected"`
	Proposed     LongVegetationUnitPreferenceValues `json:"proposed"`
}

func (request *vegetationProjectUnitRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationProjectUnitRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "project report unit save", "revision", "approvalHash", "expected", "proposed"); err != nil {
		return err
	}
	raw, err := json.Marshal(vegetationProjectReadRequest{decoded.Revision, decoded.ApprovalHash})
	if err != nil {
		return err
	}
	var read vegetationProjectReadRequest
	if err := json.Unmarshal(raw, &read); err != nil {
		return err
	}
	*request = vegetationProjectUnitRequest(decoded)
	return nil
}

type LongVegetationProjectUnitOutcome struct {
	SourceKind string                               `json:"sourceKind"`
	Revision   string                               `json:"revision"`
	Outcome    LongVegetationUnitPreferencesOutcome `json:"outcome"`
}

func (s *LongVegetationReportScopeService) SaveLongVegetationProjectUnits(ctx context.Context, contextID, requestJSON string) LongVegetationProjectUnitOutcome {
	return s.saveProjectUnits(ctx, contextID, requestJSON, nil)
}

func (s *LongVegetationReportScopeService) saveProjectUnits(ctx context.Context, contextID, requestJSON string,
	beforeCAS func()) LongVegetationProjectUnitOutcome {
	outcome := LongVegetationProjectUnitOutcome{Outcome: LongVegetationUnitPreferencesOutcome{ContextID: contextID}}
	err := s.require(ctx, contextID)
	var request vegetationProjectUnitRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		outcome.Revision = request.Revision
		_, err = vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			type candidate struct {
				scope  vprocore.VegetationProjectScopeEntry
				config vprocore.ConfigValues
			}
			read := func(owner *vprocore.SqliteContext, tx *sql.Tx) (candidate, error) {
				config, err := plots.Projects.Preferences.Snapshot()
				if err != nil {
					return candidate{}, err
				}
				scope, err := ApprovedProjectScope(s.contexts, ctx, contextID, owner, tx, config)
				if err != nil {
					return candidate{}, err
				}
				if request.Revision != projectScopeDTO(contextID, owner, scope).Revision ||
					request.ApprovalHash != scope.Build.ApprovalHash {
					return candidate{}, errors.New("project unit save scope/revision/Build changed; reload before Save")
				}
				desired := 2
				if scope.SourceKind == vprocore.VegetationProjectAllScope {
					desired = 3
				}
				if request.Proposed.UnitGroups != desired {
					return candidate{}, errors.New("project unit Save cannot change source intent; switch the explicit report scope first")
				}
				planned, err := selectedUnitConfigForChoice(config, desired)
				return candidate{scope, planned}, err
			}
			prepared, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, read)
			if err != nil {
				return struct{}{}, err
			}
			owner := plots.Projects.Sqlite
			if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
				return struct{}{}, err
			}
			defer owner.Mu.Unlock()
			final, err := vprocore.WithPublicationReadSnapshot(ctx, owner, s.snapshot, func(tx *sql.Tx) (candidate, error) {
				return read(owner, tx)
			})
			if err != nil {
				return struct{}{}, err
			}
			if !reflect.DeepEqual(prepared, final) {
				return struct{}{}, errors.New("project unit source/configuration/scope changed before Save")
			}
			outcome.SourceKind = final.scope.SourceKind
			if beforeCAS != nil {
				beforeCAS()
			}
			update, err := CompareAndSetReviewedLongVegetationUnitPreferences(plots.Projects.Preferences, ctx,
				request.Expected, request.Proposed, func(current vprocore.ConfigValues) error {
					planned, err := selectedUnitConfigForChoice(current, request.Proposed.UnitGroups)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(final.config, planned) {
						return errors.New("project unit configuration changed since Build; rebuild before Save")
					}
					return nil
				})
			outcome.Outcome.Changed, outcome.Outcome.Committed = update.Changed, update.Committed
			return struct{}{}, err
		})
	}
	if err != nil {
		outcome.Outcome.ErrorMessage = err.Error()
	}
	return outcome
}

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
