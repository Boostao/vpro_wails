package sivi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
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
	ContextID string                           `json:"contextId"`
	Project   string                           `json:"project"`
	Plot      string                           `json:"plot"`
	Columns   []vprocore.ProjectMetadataColumn `json:"columns"`
	Rows      []vprocore.ProjectMetadataRow    `json:"rows"`
}

func ReadSIVIDeletionTargets(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*siviDeletionTargets, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion targets require an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" {
		return nil, errors.New("SIVI deletion targets require an explicit literal parent")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI deletion targets parent", plot, 7); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviDeletionTargets, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*siviDeletionTargets, error) {
			project := owner.Selection.Project
			if err := sourceform.SiviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := sourceform.SiviSpeciesReadParents(ctx, tx, project, plot); err != nil {
				return nil, err
			}
			var physical int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`,
				project+"_Veg").Scan(&physical); err != nil || physical != 1 {
				return nil, errors.Join(err, errors.New("SIVI deletion targets require the exact physical vegetation table"))
			}
			table, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "project", project+"_Veg", "PlotNumber", plot, "")
			if err != nil {
				return nil, err
			}
			columns, err := vprocore.SiteUnitTransferColumns(table, sourceform.SiviCreationColumns...)
			if err != nil || len(columns) != len(sourceform.SiviCreationColumns) {
				return nil, errors.Join(err, errors.New("SIVI deletion targets require the exact all44 physical schema and complete typed rows"))
			}
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			result := &siviDeletionTargets{ContextID: contextID, Project: project, Plot: plot, Columns: table.Columns, Rows: table.Rows}
			return result, sourceform.SiviHeightContextParent(ctx, owner, plot)
		})
	})
}

func ReadSIVIDeletionHistory(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*siviDeletionHistoryList, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion history requires an owned context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plot == "" {
		return nil, errors.New("SIVI deletion history requires an explicit literal parent")
	}
	if err := vprocore.ValidateChildPhysicalText("SIVI deletion history parent", plot, 7); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviDeletionHistoryList, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*siviDeletionHistoryList, error) {
			project := owner.Selection.Project
			if err := sourceform.SiviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := sourceform.SiviSpeciesReadParents(ctx, tx, project, plot); err != nil {
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
			_, present := tables[sourceform.SiviDeletionHistoryTable]
			result := &siviDeletionHistoryList{ContextID: contextID, Project: project, Plot: plot,
				HistoryPresent: present, Events: []siviDeletionHistoryEvent{}}
			for _, deletion := range deletions {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if deletion.Request.Project != project || deletion.Request.Plot != plot {
					continue
				}
				columns, err := vprocore.SiteUnitTransferColumns(vprocore.ProjectMetadataTable{Columns: deletion.Columns}, "Species")
				if err != nil {
					return nil, err
				}
				cell := deletion.Original.Cells[columns["Species"]]
				var species *string
				switch cell.Storage {
				case "null":
				case "text":
					species = vprocore.CloneSiteUnitCell(cell).Text
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
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			return result, sourceform.SiviHeightContextParent(ctx, owner, plot)
		})
	})
}

type SIVIDeletionOriginal = sourceform.SiviDeletionOriginal
type SIVIDeletionResult = sourceform.SiviDeletionResult
type SIVIDeletionRestorationReview = siviDeletionRestorationReview
type SIVIDeletionRestorationResult = siviDeletionRestorationResult
type SIVIDeletionHistoryEvent = siviDeletionHistoryEvent
type SIVIDeletionHistoryList = siviDeletionHistoryList
type SIVIDeletionTargets = siviDeletionTargets

type SIVIDeletionService struct {
	contexts *vprocore.ContextService
}

func NewSIVIDeletionService(contexts *vprocore.ContextService) (*SIVIDeletionService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI deletion requires explicit context ownership")
	}
	return &SIVIDeletionService{contexts: contexts}, nil
}

func (s *SIVIDeletionService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("SIVI deletion requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI deletion requires explicit context ownership")
	}
	return nil
}

func (s *SIVIDeletionService) GetOriginal(ctx context.Context, contextID, plot, form, rowID string) (*SIVIDeletionOriginal, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIDeletionOriginal(s.contexts, ctx, contextID, plot, form, rowID)
}

func (s *SIVIDeletionService) Delete(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request sourceform.SiviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.DeleteSIVIVegetation(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	var request sourceform.SiviDeletionRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.LookupSIVIDeletionReceipt(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) ReviewRestoration(ctx context.Context, contextID, plot, historyID string) (*SIVIDeletionRestorationReview, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReviewSIVIDeletionRestoration(s.contexts, ctx, contextID, plot, historyID)
}

func (s *SIVIDeletionService) Restore(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return RestoreSIVIDeletion(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) LookupRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	var request siviDeletionRestorationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupSIVIDeletionRestorationReceipt(s.contexts, ctx, contextID, request)
}

func (s *SIVIDeletionService) GetHistory(ctx context.Context, contextID, plot string) (*SIVIDeletionHistoryList, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return ReadSIVIDeletionHistory(s.contexts, ctx, contextID, plot)
}

func (s *SIVIDeletionService) GetTargets(ctx context.Context, contextID, plot string) (*SIVIDeletionTargets, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadSIVIDeletionTargets(s.contexts, ctx, contextID, plot)
}
