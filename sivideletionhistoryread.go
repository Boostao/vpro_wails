package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

type siviDeletionHistoryEvent struct {
	HistoryID string  `json:"historyId"`
	Form      string  `json:"form"`
	RowID     string  `json:"rowId"`
	ID        int64   `json:"id"`
	Species   *string `json:"species"`
	Actor     string  `json:"actor"`
	EditWhen  string  `json:"editWhen"`
	Restored  bool    `json:"restored"`
	Consumed  bool    `json:"consumed"`
}

type siviDeletionHistoryList struct {
	ContextID      string                     `json:"contextId"`
	Project        string                     `json:"project"`
	Plot           string                     `json:"plot"`
	HistoryPresent bool                       `json:"historyPresent"`
	Events         []siviDeletionHistoryEvent `json:"events"`
}

type siviDeletionTargets struct {
	ContextID string                  `json:"contextId"`
	Project   string                  `json:"project"`
	Plot      string                  `json:"plot"`
	Columns   []ProjectMetadataColumn `json:"columns"`
	Rows      []ProjectMetadataRow    `json:"rows"`
}

func (s *ContextService) readSIVIDeletionTargets(ctx context.Context, contextID, plot string) (*siviDeletionTargets, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion targets require an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" {
		return nil, errors.New("SIVI deletion targets require an explicit literal parent")
	}
	if err := validateChildPhysicalText("SIVI deletion targets parent", plot, 7); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionTargets, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionTargets, error) {
			project := owner.selection.Project
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, project, plot); err != nil {
				return nil, err
			}
			var physical int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
				project+"_Veg").Scan(&physical); err != nil || physical != 1 {
				return nil, errors.Join(err, errors.New("SIVI deletion targets require the exact physical vegetation table"))
			}
			table, err := readSQLiteLiteralTextRows(ctx, tx, "project", project+"_Veg", "PlotNumber", plot, "")
			if err != nil {
				return nil, err
			}
			columns, err := siteUnitTransferColumns(table, siviCreationColumns...)
			if err != nil || len(columns) != len(siviCreationColumns) {
				return nil, errors.Join(err, errors.New("SIVI deletion targets require the exact all44 physical schema and complete typed rows"))
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			result := &siviDeletionTargets{ContextID: contextID, Project: project, Plot: plot, Columns: table.Columns, Rows: table.Rows}
			return result, siviHeightContextParent(ctx, owner, plot)
		})
	})
}

func (s *ContextService) readSIVIDeletionHistory(ctx context.Context, contextID, plot string) (*siviDeletionHistoryList, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion history requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" {
		return nil, errors.New("SIVI deletion history requires an explicit literal parent")
	}
	if err := validateChildPhysicalText("SIVI deletion history parent", plot, 7); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviDeletionHistoryList, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*siviDeletionHistoryList, error) {
			project := owner.selection.Project
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := siviSpeciesReadParents(ctx, tx, project, plot); err != nil {
				return nil, err
			}
			tables, err := readSIVIDeletionRestorationTables(ctx, tx, project)
			if err != nil {
				return nil, err
			}
			deletions, err := readSIVIDeletionRestorationDeletions(ctx, tables)
			if err != nil {
				return nil, err
			}
			restorations, err := readSIVIDeletionRestorations(ctx, tables, deletions)
			if err != nil {
				return nil, err
			}
			consumed := map[string]bool{}
			for _, restoration := range restorations {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				consumed[restoration.Request.HistoryID] = true
			}
			_, present := tables[siviDeletionHistoryTable]
			result := &siviDeletionHistoryList{ContextID: contextID, Project: project, Plot: plot,
				HistoryPresent: present, Events: []siviDeletionHistoryEvent{}}
			for _, deletion := range deletions {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if deletion.Request.Project != project || deletion.Request.Plot != plot {
					continue
				}
				columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: deletion.Columns}, "Species")
				if err != nil {
					return nil, err
				}
				cell := deletion.Original.Cells[columns["Species"]]
				var species *string
				switch cell.Storage {
				case "null":
				case "text":
					species = cloneSiteUnitCell(cell).Text
				default:
					return nil, fmt.Errorf("SIVI deletion history %s has unsupported non-text historical Species storage", deletion.Result.HistoryID)
				}
				restored := consumed[deletion.Result.HistoryID]
				result.Events = append(result.Events, siviDeletionHistoryEvent{
					HistoryID: deletion.Result.HistoryID, Form: deletion.Request.Form, RowID: deletion.Result.RowID,
					ID: deletion.Result.ID, Species: species, Actor: deletion.Actor, EditWhen: deletion.When,
					Restored: restored, Consumed: restored,
				})
			}
			sort.Slice(result.Events, func(i, j int) bool {
				if result.Events[i].EditWhen != result.Events[j].EditWhen {
					return result.Events[i].EditWhen > result.Events[j].EditWhen
				}
				return result.Events[i].HistoryID < result.Events[j].HistoryID
			})
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			return result, siviHeightContextParent(ctx, owner, plot)
		})
	})
}
