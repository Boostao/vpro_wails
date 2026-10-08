package main

import (
	"context"
	"errors"
	"fmt"
)

const siviParentSharedFeatureEnvironment = "VPRO_SIVI_PARENT_SHARED_EDITING"
const siviParentSharedHistoryTable = "__VPRO_SIVIParentSharedHistory"
const siviParentSharedHistorySQL = `CREATE TABLE "__VPRO_SIVIParentSharedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviParentSharedOwners = map[string]string{
	"AirPhotoNum": "Env", "XCoord": "Env", "YCoord": "Env",
	"VegNotes":        "Env",
	"StrataCoverTree": "Env", "StrataCoverShrub": "Env",
	"StrataCoverHerb": "Env", "StrataCoverMoss": "Env",
}

type SIVIParentSharedWrite struct {
	Original *SIVIParentProjection `json:"original"`
	Edits    []SIVIParentCellEdit  `json:"edits"`
}

func (request *SIVIParentSharedWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentSharedWrite
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("SIVI shared-field transport: %w", err)
	}
	*request = SIVIParentSharedWrite(decoded)
	return nil
}

// Registration is intentionally separate from the XL facade: shared storage
// does not authorize XL callbacks or its whole-header Save.
type SIVIParentSharedService struct {
	contexts *ContextService
	enabled  bool
}

func NewSIVIParentSharedService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVIParentSharedService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI shared-field service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(siviParentSharedFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVIParentSharedService{contexts: contexts, enabled: enabled}, nil
}

func (s *SIVIParentSharedService) require(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled || !s.contexts.siviParentReviewEnabled {
		return errors.New("SIVI shared-field editing requires independently enabled parent review and shared-field editing")
	}
	return nil
}

func (s *SIVIParentSharedService) GetOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIParent(ctx, contextID, plot)
}

func planSIVIParentShared(ctx context.Context, original *siviParentProjection, edits []SIVIParentCellEdit) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("SIVI shared-field editing requires one owned physical parent pair")
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	return planSIVIParentCells(ctx, original.ContextID, original.Project, original.Plot, env, admin, edits,
		siviParentSharedOwners, func(column string, cell ProjectMetadataCell) error {
			value, err := metadataCellValue(cell)
			if err != nil {
				return err
			}
			return validateOrdinaryRestoreValue("Env", column, value)
		})
}

var siviParentSharedHistory = siviParentHistoryDomain{
	siviParentSharedHistoryTable, siviParentSharedHistorySQL, "SIVI shared fields",
	func(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
		if event.ProjectAssignment != nil {
			return nil, errors.New("ProjectID history is not shared-field history")
		}
		original, err := validateSIVIParentHistoryOriginal(ctx, event, project, plot)
		if err != nil {
			return nil, err
		}
		edits := make([]SIVIParentCellEdit, 0, len(event.Changes))
		for _, change := range event.Changes {
			edits = append(edits, SIVIParentCellEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
		}
		assignments, err := planSIVIParentShared(ctx, original, edits)
		if err != nil || len(assignments) != len(event.Changes) {
			return nil, errors.Join(err, errors.New("typed shared-field history contains repeated or unchanged assignments"))
		}
		return assignments, nil
	},
}

func (s *SIVIParentSharedService) Save(ctx context.Context, contextID, plot string, request SIVIParentSharedWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.writeSIVIParentPlanned(ctx, contextID, plot, request.Original, siviParentSharedHistory,
		func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			return planSIVIParentShared(ctx, observed, request.Edits)
		})
}

func (s *SIVIParentSharedService) Restore(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVIParentHistory(ctx, contextID, plot, historyID, action, siviParentSharedHistory)
}
