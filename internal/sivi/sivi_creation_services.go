package sivi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type SIVICreationResult = sourceform.SiviCreationResult

type SIVICreationService struct {
	contexts *vprocore.ContextService
}

func NewSIVICreationService(contexts *vprocore.ContextService) (*SIVICreationService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI creation requires explicit context ownership")
	}
	return &SIVICreationService{contexts: contexts}, nil
}

func (s *SIVICreationService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI creation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI creation requires explicit context ownership")
	}
	return nil
}

func (s *SIVICreationService) GetReferences(ctx context.Context, contextID, plot string) (sourceform.SIVISpeciesReferences, error) {
	if err := s.require(ctx); err != nil {
		return sourceform.SIVISpeciesReferences{}, err
	}
	return sourceform.GetSIVISpeciesReferences(s.contexts, ctx, contextID, plot)
}

func (s *SIVICreationService) Create(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request sourceform.SiviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.CreateSIVIVegetation(s.contexts, ctx, contextID, request)
}

func (s *SIVICreationService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request sourceform.SiviCreationRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return sourceform.LookupSIVICreationReceipt(s.contexts, ctx, contextID, request)
}

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

type SIVICreationUndoReview = siviCreationUndoReview
type SIVICreationUndoResult = siviCreationUndoResult
type SIVICreationUndoHistoryEvent = siviCreationUndoHistoryEvent
type SIVICreationUndoHistoryList = siviCreationUndoHistoryList

type SIVICreationUndoService struct {
	contexts *vprocore.ContextService
}

func NewSIVICreationUndoService(contexts *vprocore.ContextService) (*SIVICreationUndoService, error) {
	if contexts == nil {
		return nil, errors.New("SIVI creation Undo requires explicit context ownership")
	}
	return &SIVICreationUndoService{contexts: contexts}, nil
}

func (s *SIVICreationUndoService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI creation Undo requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("SIVI creation Undo requires explicit context ownership")
	}
	return nil
}

func (s *SIVICreationUndoService) GetHistory(ctx context.Context, contextID, plot string) (*SIVICreationUndoHistoryList, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReadSIVICreationUndoHistory(s.contexts, ctx, contextID, plot)
}

func (s *SIVICreationUndoService) Review(ctx context.Context, contextID, plot, historyID string) (*SIVICreationUndoReview, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReviewSIVICreationUndo(s.contexts, ctx, contextID, plot, historyID)
}

func (s *SIVICreationUndoService) Undo(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return UndoSIVICreation(s.contexts, ctx, contextID, request)
}

func (s *SIVICreationUndoService) LookupReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request siviCreationUndoRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	return LookupSIVICreationUndoReceipt(s.contexts, ctx, contextID, request)
}
