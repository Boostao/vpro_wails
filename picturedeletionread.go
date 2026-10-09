package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strconv"
)

type PictureDeletionHistoryEvent struct {
	Deletion    pictureDeletionResult     `json:"deletion"`
	Restoration *pictureRestorationResult `json:"restoration"`
}

type PictureDeletionHistoryList struct {
	ContextID  string                        `json:"contextId"`
	Project    string                        `json:"project"`
	PlotNumber string                        `json:"plotNumber"`
	Events     []PictureDeletionHistoryEvent `json:"events"`
}

type PictureRestorationReview struct {
	ContextID  string                `json:"contextId"`
	Project    string                `json:"project"`
	PlotNumber string                `json:"plotNumber"`
	HistoryID  string                `json:"historyId"`
	Deletion   pictureDeletionResult `json:"deletion"`
	Peers      []ProjectMetadataRow  `json:"peers"`
}

func pictureDeletionEvents(ctx context.Context, tx *sql.Tx, source *ownedPictureLibrary, plots *PlotService,
	owner *sqliteContext, contextID, plot string) (PictureDeletionHistoryList, error) {
	history, _, err := readPictureDeletionHistory(ctx, tx, source)
	if err != nil {
		return PictureDeletionHistoryList{}, err
	}
	list := PictureDeletionHistoryList{ContextID: contextID, Project: owner.selection.Project,
		PlotNumber: plot, Events: []PictureDeletionHistoryEvent{}}
	for _, deletion := range history {
		if deletion.Request.Project != list.Project || deletion.Request.PlotNumber != plot {
			continue
		}
		authority, err := pictureRestorationAuthority(ctx, plots, owner, contextID, source, deletion.Request)
		if err != nil {
			return list, err
		}
		if err := comparePictureRestorationProvenance(deletion, authority); err != nil {
			return list, err
		}
		event := PictureDeletionHistoryEvent{Deletion: deletion}
		var restored sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT Restored FROM "__VPRO_PictureDeletionHistory" WHERE ID=?`, deletion.HistoryID).Scan(&restored); err != nil {
			return list, err
		}
		if restored.Valid {
			receipt, err := decodePictureRestoration([]byte(restored.String), deletion)
			if err != nil {
				return list, err
			}
			event.Restoration = &receipt
		}
		list.Events = append(list.Events, event)
	}
	return list, nil
}

func (s *ContextService) readPictureDeletionEvents(ctx context.Context, contextID string, source *ownedPictureLibrary,
	plot string) (*PictureDeletionHistoryList, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*PictureDeletionHistoryList, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*PictureDeletionHistoryList, error) {
			if err := requirePictureParent(ctx, owner, parent, plot); err != nil {
				return nil, err
			}
			return withPictureHistoryReadSnapshot(ctx, source, func(tx *sql.Tx) (*PictureDeletionHistoryList, error) {
				list, err := pictureDeletionEvents(ctx, tx, source, plots, owner, contextID, plot)
				if err != nil {
					return nil, err
				}
				return &list, nil
			})
		})
	})
}

func (s *ContextService) reviewPictureRestoration(ctx context.Context, contextID string, source *ownedPictureLibrary,
	plot, historyID string) (*PictureRestorationReview, error) {
	id, err := strconv.ParseInt(historyID, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != historyID {
		return nil, errors.New("picture restoration review requires an exact positive history identity")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*PictureRestorationReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*PictureRestorationReview, error) {
			if err := requirePictureParent(ctx, owner, parent, plot); err != nil {
				return nil, err
			}
			return withPictureHistoryReadSnapshot(ctx, source, func(tx *sql.Tx) (*PictureRestorationReview, error) {
				if err := pictureDeletionWriterSchema(ctx, tx); err != nil {
					return nil, err
				}
				list, err := pictureDeletionEvents(ctx, tx, source, plots, owner, contextID, plot)
				if err != nil {
					return nil, err
				}
				for _, event := range list.Events {
					if event.Deletion.HistoryID != historyID {
						continue
					}
					if event.Restoration != nil {
						return nil, errors.New("picture deletion is already restored; retain its historical receipt")
					}
					records, err := readPictureLibraryRows(ctx, tx, plot)
					if err != nil {
						return nil, err
					}
					if !reflect.DeepEqual(records.Columns, event.Deletion.Request.Columns) {
						return nil, errors.New("picture restoration review schema differs from the historical deletion")
					}
					rowID, pictureID, err := preparePictureDeletion(event.Deletion.Request)
					if err != nil {
						return nil, err
					}
					collisions, err := pictureRestorationCollisionCount(ctx, tx, rowID, pictureID)
					if err != nil {
						return nil, err
					}
					if collisions != 0 {
						return nil, errors.New("picture restoration review refuses an occupied rowid or identity; no replacement")
					}
					return &PictureRestorationReview{ContextID: contextID, Project: list.Project, PlotNumber: plot,
						HistoryID: historyID, Deletion: event.Deletion, Peers: records.Rows}, nil
				}
				return nil, errors.New("picture restoration history is outside the owned literal parent")
			})
		})
	})
}
