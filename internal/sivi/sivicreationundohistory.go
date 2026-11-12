package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"sort"
)

type siviCreationUndoHistoryEvent struct {
	HistoryID         string  `json:"historyId"`
	Form              string  `json:"form"`
	RowID             string  `json:"rowId"`
	ID                int64   `json:"id"`
	Species           string  `json:"species"`
	Actor             string  `json:"actor"`
	EditWhen          string  `json:"editWhen"`
	Undone            bool    `json:"undone"`
	Consumed          bool    `json:"consumed"`
	ReviewAvailable   bool    `json:"reviewAvailable"`
	UnavailableReason *string `json:"unavailableReason"`
}

type siviCreationUndoHistoryList struct {
	ContextID      string                         `json:"contextId"`
	Project        string                         `json:"project"`
	Plot           string                         `json:"plot"`
	HistoryPresent bool                           `json:"historyPresent"`
	Events         []siviCreationUndoHistoryEvent `json:"events"`
}

func ReadSIVICreationUndoHistory(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*siviCreationUndoHistoryList, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI creation Undo history requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" {
		return nil, errors.New("SIVI creation Undo history requires an explicit literal parent")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI creation Undo history parent", plot, 7); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviCreationUndoHistoryList, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*siviCreationUndoHistoryList, error) {
			project := owner.Selection.Project
			if err := sourceform.SiviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := sourceform.SiviSpeciesReadParents(ctx, tx, project, plot); err != nil {
				return nil, err
			}
			tables, err := readSIVICreationUndoTables(ctx, tx, project)
			if err != nil {
				return nil, err
			}
			creations, err := readSIVICreationUndoCreations(ctx, tables)
			if err != nil {
				return nil, err
			}
			undos, err := readSIVICreationUndos(ctx, tables, creations)
			if err != nil {
				return nil, err
			}
			consumed := map[string]bool{}
			for _, undo := range undos {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				consumed[undo.Request.HistoryID] = true
			}
			_, present := tables[sourceform.SiviCreationHistoryTable]
			result := &siviCreationUndoHistoryList{ContextID: contextID, Project: project, Plot: plot,
				HistoryPresent: present, Events: []siviCreationUndoHistoryEvent{}}
			for _, creation := range creations {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if creation.Request.Project != project || creation.Request.Plot != plot {
					continue
				}
				undone := consumed[creation.Result.HistoryID]
				available := !undone && vprocore.SiviDeletionUUID.MatchString(creation.Result.HistoryID)
				var reason *string
				if !available {
					message := "Historical creation was already consumed by an Undo."
					if !vprocore.SiviDeletionUUID.MatchString(creation.Result.HistoryID) {
						message = "Historical creation has a legacy non-UUID identity; typed Undo requires a reviewed UUID history."
					}
					reason = &message
				}
				result.Events = append(result.Events, siviCreationUndoHistoryEvent{
					HistoryID: creation.Result.HistoryID, Form: creation.Request.Form, RowID: creation.Result.RowID,
					ID: creation.Result.ID, Species: creation.Request.Species, Actor: creation.Actor, EditWhen: creation.When,
					Undone: undone, Consumed: undone, ReviewAvailable: available, UnavailableReason: reason,
				})
			}
			sort.Slice(result.Events, func(i, j int) bool {
				if result.Events[i].EditWhen != result.Events[j].EditWhen {
					return result.Events[i].EditWhen > result.Events[j].EditWhen
				}
				return result.Events[i].HistoryID < result.Events[j].HistoryID
			})
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			return result, sourceform.SiviHeightContextParent(ctx, owner, plot)
		})
	})
}
