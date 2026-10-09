package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type pictureRestorationRequest struct {
	RequestID string                 `json:"requestId"`
	ContextID string                 `json:"contextId"`
	HistoryID string                 `json:"historyId"`
	Deletion  pictureDeletionRequest `json:"deletion"`
	Peers     []ProjectMetadataRow   `json:"peers"`
}

func (request *pictureRestorationRequest) UnmarshalJSON(data []byte) error {
	const operation = "picture restoration"
	names := []string{"requestId", "contextId", "historyId", "deletion", "peers"}
	properties, err := sourceChildJSONObject(data, operation, names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, names...); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["peers"], func(raw json.RawMessage) error {
		return sourceChildRowJSON(raw, operation+" peer")
	}); err != nil {
		return err
	}
	type plain pictureRestorationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if err := validatePictureRestoration(pictureRestorationRequest(decoded)); err != nil {
		return err
	}
	*request = pictureRestorationRequest(decoded)
	return nil
}

func validatePictureRestoration(request pictureRestorationRequest) error {
	if _, _, err := preparePictureDeletion(request.Deletion); err != nil {
		return err
	}
	if request.ContextID == "" || strings.ContainsRune(request.ContextID, 0) {
		return errors.New("picture restoration requires an explicit operation context")
	}
	if err := validateChildPhysicalText("Picture restoration context", request.ContextID, 100); err != nil {
		return err
	}
	historyID, err := strconv.ParseInt(request.HistoryID, 10, 64)
	if err != nil || historyID < 1 || strconv.FormatInt(historyID, 10) != request.HistoryID ||
		!siviDeletionUUID.MatchString(request.RequestID) || request.RequestID == request.Deletion.RequestID || request.Peers == nil {
		return errors.New("picture restoration requires distinct exact UUID, history identity and explicit complete peers")
	}
	seen := map[string]bool{}
	for _, peer := range request.Peers {
		rowID, err := strconv.ParseInt(peer.RowID, 10, 64)
		if err != nil || strconv.FormatInt(rowID, 10) != peer.RowID || seen[peer.RowID] ||
			peer.RowID == request.Deletion.Original.RowID || len(peer.Cells) != len(request.Deletion.Columns) {
			return errors.New("picture restoration requires complete distinct physical peers excluding the deleted rowid")
		}
		seen[peer.RowID] = true
		for _, cell := range peer.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return fmt.Errorf("picture restoration peer: %w", err)
			}
		}
		parent := peer.Cells[3]
		if parent.Storage != "text" || parent.Text == nil || *parent.Text != request.Deletion.PlotNumber {
			return errors.New("picture restoration peer is outside the literal parent")
		}
	}
	return nil
}

type pictureRestorationResult struct {
	Request       pictureRestorationRequest `json:"request"`
	Deletion      pictureDeletionResult     `json:"deletion"`
	Actor         string                    `json:"actor"`
	AuditStrength int                       `json:"auditStrength"`
	RestoredWhen  string                    `json:"restoredWhen"`
	DidCommit     bool                      `json:"didCommit"`
	Replayed      bool                      `json:"replayed"`
}

func (result *pictureRestorationResult) UnmarshalJSON(data []byte) error {
	const operation = "picture restoration history"
	names := []string{"request", "deletion", "actor", "auditStrength", "restoredWhen", "didCommit", "replayed"}
	properties, err := sourceChildJSONObject(data, operation, names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, names...); err != nil {
		return err
	}
	type plain pictureRestorationResult
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*result = pictureRestorationResult(decoded)
	return nil
}

func decodePictureRestoration(raw []byte, deletion pictureDeletionResult) (pictureRestorationResult, error) {
	var receipt pictureRestorationResult
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return receipt, err
	}
	when, err := time.Parse(time.RFC3339Nano, receipt.RestoredWhen)
	if err != nil || when.UTC().Format(time.RFC3339Nano) != receipt.RestoredWhen || !receipt.DidCommit || receipt.Replayed ||
		receipt.Request.HistoryID != deletion.HistoryID || !reflect.DeepEqual(receipt.Request.Deletion, deletion.Request) ||
		!reflect.DeepEqual(receipt.Deletion, deletion) || receipt.Actor == "" || receipt.AuditStrength < 0 || receipt.AuditStrength > 3 {
		return pictureRestorationResult{}, errors.Join(err, errors.New("picture restoration acknowledgement has incompatible provenance"))
	}
	if err := validateChildPhysicalText("Picture restoration history actor", receipt.Actor, 255); err != nil {
		return pictureRestorationResult{}, err
	}
	if strings.ContainsRune(receipt.Actor, 0) {
		return pictureRestorationResult{}, errors.New("picture restoration history actor contains NUL")
	}
	return receipt, nil
}

func pictureRestorationAuthority(ctx context.Context, plots *PlotService, owner *sqliteContext, contextID string,
	source *ownedPictureLibrary, deletion pictureDeletionRequest) (pictureDeletionResult, error) {
	current := deletion
	current.ContextID = contextID
	authority, err := pictureDeletionAuthority(ctx, plots, owner, contextID, source, current)
	authority.Request = deletion
	return authority, err
}

func comparePictureRestorationProvenance(event, authority pictureDeletionResult) error {
	if !reflect.DeepEqual(event.Request, authority.Request) || event.Source != authority.Source ||
		event.RequestedSource != authority.RequestedSource || !reflect.DeepEqual(event.OwnedFiles, authority.OwnedFiles) {
		return errors.New("picture restoration immutable deletion/source ownership changed")
	}
	return nil
}

type pictureRestorationCommittedError struct{ cause error }

func (err *pictureRestorationCommittedError) Error() string {
	return "picture restoration committed, but cleanup failed; retain the receipt and recover before retrying: " + err.cause.Error()
}
func (err *pictureRestorationCommittedError) Unwrap() error { return err.cause }

func (s *ContextService) restorePicture(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureRestorationRequest) (pictureRestorationResult, error) {
	return s.restorePictureWithHooks(ctx, contextID, source, request, pictureDeletionHooks{})
}

func (s *ContextService) restorePictureWithHooks(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureRestorationRequest, hooks pictureDeletionHooks) (pictureRestorationResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return pictureRestorationResult{}, errors.New("picture restoration requires an owned service and request context")
	}
	if err := validatePictureRestoration(request); err != nil {
		return pictureRestorationResult{}, err
	}
	if request.ContextID != contextID {
		return pictureRestorationResult{}, errors.New("picture restoration reviewed operation context changed")
	}
	var receipt pictureRestorationResult
	result, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (pictureRestorationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (pictureRestorationResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.Deletion.PlotNumber); err != nil {
				return pictureRestorationResult{}, err
			}
			authority, err := pictureRestorationAuthority(ctx, plots, owner, contextID, source, request.Deletion)
			if err != nil {
				return pictureRestorationResult{}, err
			}
			return withPictureMetadataParentGuards(ctx, owner, request.Deletion.PlotNumber, func(freshParent func() error) (pictureRestorationResult, error) {
				receipt, err = writePictureRestoration(ctx, source, request, authority, func() error {
					if err := freshParent(); err != nil {
						return err
					}
					_, err := pictureRestorationAuthority(ctx, plots, owner, contextID, source, request.Deletion)
					return err
				})
				if receipt.DidCommit && !receipt.Replayed && hooks.afterLibraryCommit != nil {
					err = errors.Join(err, hooks.afterLibraryCommit(parent))
				}
				return receipt, err
			})
		})
	})
	err = errors.Join(err, ctx.Err())
	if err != nil && receipt.DidCommit {
		return receipt, &pictureRestorationCommittedError{cause: err}
	}
	return result, err
}

func findPictureRestoration(ctx context.Context, tx *sql.Tx, source *ownedPictureLibrary, request pictureRestorationRequest,
	authority pictureDeletionResult) (pictureDeletionResult, *pictureRestorationResult, error) {
	history, _, err := readPictureDeletionHistory(ctx, tx, source)
	if err != nil {
		return pictureDeletionResult{}, nil, err
	}
	var deletion *pictureDeletionResult
	for _, event := range history {
		if event.Request.RequestID == request.RequestID {
			return pictureDeletionResult{}, nil, errors.New("picture restoration UUID collides with deletion history")
		}
		if event.HistoryID == request.HistoryID {
			copy := event
			deletion = &copy
		}
		var raw sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT Restored FROM "__VPRO_PictureDeletionHistory" WHERE ID=?`, event.HistoryID).Scan(&raw); err != nil {
			return pictureDeletionResult{}, nil, err
		}
		if raw.Valid {
			restored, err := decodePictureRestoration([]byte(raw.String), event)
			if err != nil {
				return pictureDeletionResult{}, nil, err
			}
			if restored.Request.RequestID == request.RequestID {
				if !reflect.DeepEqual(restored.Request, request) {
					return pictureDeletionResult{}, nil, errors.New("picture restoration UUID collides with another reviewed plan")
				}
				if err := comparePictureRestorationProvenance(event, authority); err != nil {
					return pictureDeletionResult{}, nil, err
				}
				restored.Replayed = true
				return event, &restored, nil
			}
			if event.HistoryID == request.HistoryID {
				return pictureDeletionResult{}, nil, errors.New("picture deletion was already restored by a different request")
			}
		}
	}
	if deletion == nil {
		return pictureDeletionResult{}, nil, errors.New("picture restoration deletion history is missing")
	}
	if err := comparePictureRestorationProvenance(*deletion, authority); err != nil {
		return pictureDeletionResult{}, nil, err
	}
	return *deletion, nil, nil
}

func writePictureRestoration(ctx context.Context, source *ownedPictureLibrary, request pictureRestorationRequest,
	authority pictureDeletionResult, validate func() error) (result pictureRestorationResult, resultErr error) {
	if err := validatePictureRestoration(request); err != nil {
		return result, err
	}
	if err := errors.Join(ctx.Err(), source.checkFile(), validate()); err != nil {
		return result, err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw")+"&_busy_timeout=5000&_txlock=immediate&_foreign_keys=on")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := pictureDeletionWriterSchema(ctx, tx); err != nil {
		return result, err
	}
	deletion, replay, err := findPictureRestoration(ctx, tx, source, request, authority)
	if err != nil {
		return result, err
	}
	if replay != nil {
		return *replay, errors.Join(validate(), source.checkFile(), ctx.Err())
	}
	before, err := readPictureLibraryRows(ctx, tx, request.Deletion.PlotNumber)
	if err != nil || !reflect.DeepEqual(before.Columns, request.Deletion.Columns) || !reflect.DeepEqual(before.Rows, request.Peers) {
		return result, errors.Join(err, errors.New("picture restoration complete physical peers/schema changed; reload the review"))
	}
	rowID, id, err := preparePictureDeletion(request.Deletion)
	if err != nil {
		return result, err
	}
	collisions, err := pictureRestorationCollisionCount(ctx, tx, rowID, id)
	if err != nil {
		return result, err
	}
	if collisions != 0 {
		return result, errors.New("picture restoration rowid or identity is occupied; no replacement or identity repair")
	}
	values := []any{rowID}
	for _, cell := range request.Deletion.Original.Cells {
		value, err := metadataCellValue(cell)
		if err != nil {
			return result, err
		}
		values = append(values, value)
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) VALUES(?,?,?,?,?,?)`, values...)
	if err != nil {
		return result, err
	}
	count, err := inserted.RowsAffected()
	if err != nil || count != 1 {
		return result, errors.Join(err, errors.New("picture restoration expected exactly one inserted physical row"))
	}
	expected := append([]ProjectMetadataRow{}, request.Peers...)
	expected = append(expected, request.Deletion.Original)
	sort.Slice(expected, func(i, j int) bool {
		left, _ := strconv.ParseInt(expected[i].RowID, 10, 64)
		right, _ := strconv.ParseInt(expected[j].RowID, 10, 64)
		return left < right
	})
	after, err := readPictureLibraryRows(ctx, tx, request.Deletion.PlotNumber)
	if err != nil || !reflect.DeepEqual(after, ProjectMetadataTable{Columns: before.Columns, Rows: expected}) {
		return result, errors.Join(err, errors.New("picture restoration differs from the complete typed physical plan"))
	}
	receipt := pictureRestorationResult{Request: request, Deletion: deletion, Actor: authority.Actor,
		AuditStrength: authority.AuditStrength, RestoredWhen: time.Now().UTC().Format(time.RFC3339Nano), DidCommit: true}
	proposal, err := json.Marshal(receipt)
	if err != nil {
		return result, err
	}
	updated, err := tx.ExecContext(ctx, `UPDATE "__VPRO_PictureDeletionHistory" SET Restored=? WHERE ID=? AND Restored IS NULL`,
		string(proposal), request.HistoryID)
	if err != nil {
		return result, err
	}
	count, err = updated.RowsAffected()
	if err != nil || count != 1 {
		return result, errors.Join(err, errors.New("picture restoration expected one unconsumed history acknowledgement"))
	}
	if _, _, err := readPictureDeletionHistory(ctx, tx, source); err != nil {
		return result, err
	}
	if err := errors.Join(validate(), source.checkFile(), ctx.Err()); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return receipt, nil
}
