package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

const siviProjectAssignmentFeatureEnvironment = "VPRO_SIVI_PROJECT_ASSIGNMENT"

type SIVIProjectAssignmentOriginal struct {
	Original             *SIVIParentProjection
	Choices              *SIVIProjectChoices
	AssignmentAvailable  bool
	AssignmentDiagnostic string
}

type SIVIProjectSelection struct {
	ContextID        string                  `json:"contextId"`
	ControlID        string                  `json:"controlId"`
	Table            string                  `json:"table"`
	RowID            string                  `json:"rowId"`
	Expected         ProjectMetadataCell     `json:"expected"`
	SourceOption     int                     `json:"sourceOption"`
	MetadataAlias    string                  `json:"metadataAlias"`
	MetadataTable    string                  `json:"metadataTable"`
	MetadataColumns  []ProjectMetadataColumn `json:"metadataColumns"`
	MetadataOriginal ProjectMetadataRow      `json:"metadataOriginal"`
}

type SIVIProjectAssignmentWrite struct {
	Original  *SIVIParentProjection `json:"original"`
	Selection SIVIProjectSelection  `json:"selection"`
}

func (input *SIVIProjectSelection) UnmarshalJSON(data []byte) error {
	type plain SIVIProjectSelection
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "contextId", "controlId", "table", "rowId",
		"expected", "sourceOption", "metadataAlias", "metadataTable", "metadataColumns", "metadataOriginal"); err != nil {
		return fmt.Errorf("SIVI ProjectID selection transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if err := decodeProfileLifecycleJSON(properties["metadataOriginal"], &decoded.MetadataOriginal, "rowId", "cells"); err != nil {
		return fmt.Errorf("SIVI ProjectID metadata row transport: %w", err)
	}
	var columns []json.RawMessage
	if err := json.Unmarshal(properties["metadataColumns"], &columns); err != nil {
		return err
	}
	for index, raw := range columns {
		if err := decodeProfileLifecycleJSON(raw, &decoded.MetadataColumns[index], "name", "declaredType"); err != nil {
			return fmt.Errorf("SIVI ProjectID metadata column transport: %w", err)
		}
	}
	*input = SIVIProjectSelection(decoded)
	return nil
}

func (request *SIVIProjectAssignmentWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIProjectAssignmentWrite
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "selection"); err != nil {
		return fmt.Errorf("SIVI ProjectID assignment transport: %w", err)
	}
	*request = SIVIProjectAssignmentWrite(decoded)
	return nil
}

func (input SIVIProjectSelection) private() siviProjectSelection {
	return siviProjectSelection{
		ContextID: input.ContextID, ControlID: input.ControlID, Table: input.Table, RowID: input.RowID,
		Expected: input.Expected, SourceOption: input.SourceOption, MetadataAlias: input.MetadataAlias,
		MetadataTable: input.MetadataTable, MetadataColumns: input.MetadataColumns, MetadataOriginal: input.MetadataOriginal,
	}
}

func siviProjectAssignmentFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(siviProjectAssignmentFeatureEnvironment, lookup)
}

func (s *ContextService) requireSIVIProjectAssignment(ctx context.Context) error {
	if err := s.requireSIVIParentEditing(ctx); err != nil {
		return err
	}
	if !s.siviProjectAssignmentEnabled {
		return errors.New("SIVI ProjectID assignment requires enabled assignment editing in this session")
	}
	return nil
}

func (s *ContextService) GetSIVIProjectAssignmentOriginal(ctx context.Context, contextID, plot string) (*SIVIProjectAssignmentOriginal, error) {
	if err := s.requireSIVIProjectAssignment(ctx); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*SIVIProjectAssignmentOriginal, error) {
		values, err := plots.projects.preferences.snapshot()
		if err != nil {
			return nil, err
		}
		source, err := configInt(values, "Current", "ProjectIdSource", 1, 2)
		if err != nil {
			return nil, fmt.Errorf("SIVI ProjectID assignment source unavailable: %w", err)
		}
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*SIVIProjectAssignmentOriginal, error) {
			return readSIVIProjectAssignmentSnapshot(ctx, owner, tx, contextID, plot, source)
		})
	})
}

func (s *ContextService) SaveSIVIProjectAssignment(ctx context.Context, contextID, plot string, request SIVIProjectAssignmentWrite) (*SIVIParentWriteResult, error) {
	if err := s.requireSIVIProjectAssignment(ctx); err != nil {
		return nil, err
	}
	return s.writeSIVIProjectAssignment(ctx, contextID, plot, request.Original, request.Selection.private())
}

func (s *ContextService) RestoreSIVIProjectAssignment(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.requireSIVIProjectAssignment(ctx); err != nil {
		return nil, err
	}
	return s.restoreSIVIProjectAssignment(ctx, contextID, plot, historyID, action)
}
