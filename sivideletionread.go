package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

type siviDeletionOriginal struct {
	ContextID string                  `json:"contextId"`
	Project   string                  `json:"project"`
	Plot      string                  `json:"plot"`
	Form      string                  `json:"form"`
	Columns   []ProjectMetadataColumn `json:"columns"`
	Original  ProjectMetadataRow      `json:"original"`
}

func siviDeletionSource(ctx context.Context, project, plot, form, rowID string, veg ProjectMetadataTable) (*siviDeletionOriginal, error) {
	if err := validateSIVIDeletionScope(project, plot, form, rowID); err != nil {
		return nil, err
	}
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil || len(columns) != len(siviCreationColumns) {
		return nil, errors.Join(err, errors.New("SIVI deletion requires the complete all44 physical schema"))
	}
	groups, err := projectSIVIVegetation(ctx, plot, form == "SubVegA-SIVI", veg)
	if err != nil {
		return nil, err
	}
	member := false
	for _, group := range groups {
		if group.Form == form {
			for _, row := range group.Rows {
				member = member || row.RowID == rowID
			}
		}
	}
	if !member {
		return nil, errors.New("SIVI deletion row is unavailable in the exact source form and parent")
	}
	for _, row := range veg.Rows {
		if row.RowID != rowID {
			continue
		}
		original := &siviDeletionOriginal{Project: project, Plot: plot, Form: form, Columns: veg.Columns, Original: row}
		id, err := validateSIVIDeletionOriginal(*original)
		if err != nil {
			return nil, err
		}
		occupants := 0
		for _, candidate := range veg.Rows {
			parent := candidate.Cells[columns["PlotNumber"]]
			if parent.Text != nil && *parent.Text == plot &&
				siviIdentityCellMatches(candidate.Cells[columns["ID"]], strconv.FormatInt(id, 10)) {
				occupants++
			}
		}
		if occupants != 1 {
			return nil, errors.New("SIVI deletion logical ID is ambiguous in its parent; source audits cannot distinguish rows")
		}
		return original, nil
	}
	return nil, fmt.Errorf("SIVI deletion physical row %s disappeared", rowID)
}

func (s *ContextService) readSIVIDeletionOriginal(ctx context.Context, contextID, plot, form, rowID string) (*siviDeletionOriginal, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion original requires an owned context")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionOriginal, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionOriginal, error) {
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, owner.selection.Project, plot); err != nil {
				return nil, err
			}
			veg, err := readSQLiteStorageRows(ctx, tx, "project", owner.selection.Project+"_Veg", "", nil, "")
			if err != nil {
				return nil, err
			}
			original, err := siviDeletionSource(ctx, owner.selection.Project, plot, form, rowID, veg)
			if err != nil {
				return nil, err
			}
			original.ContextID = contextID
			return original, siviHeightContextParent(ctx, owner, plot)
		})
	})
}

func (s *ContextService) lookupSIVIDeletionReceipt(ctx context.Context, contextID string, request siviDeletionRequest) (*siviDeletionResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion receipt requires an owned context")
	}
	if err := validateSIVIDeletionRequest(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionResult, error) {
			if request.Project != owner.selection.Project {
				return nil, errors.New("SIVI deletion receipt belongs to another project")
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
			schema, err := readSQLiteStorageRows(ctx, tx, "project", "sqlite_master", "", nil, "")
			if err != nil {
				return nil, err
			}
			present := map[string]bool{}
			for _, row := range schema.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" && row.Cells[1].Text != nil {
					present[*row.Cells[1].Text] = true
				}
			}
			if !present[siviDeletionHistoryTable] {
				return nil, nil
			}
			tables := map[string]ProjectMetadataTable{}
			for _, name := range []string{siviDeletionHistoryTable, request.Project + "_Veg", request.Project + "_Audit", siviIdentityLedger} {
				if !present[name] {
					continue
				}
				table, err := readSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
				if err != nil {
					return nil, err
				}
				tables[name] = table
			}
			history, err := readSIVIDeletionReplay(ctx, request, tables)
			if err != nil || history == nil {
				return nil, err
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := siviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return siviDeletionReceipt(*history, contextID, false), nil
		})
	})
}

func sameSIVIDeletionRequest(a, b siviDeletionRequest) bool {
	a.ContextID, b.ContextID = "", ""
	return reflect.DeepEqual(a, b)
}
