package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const pictureDeletionHistoryTable = "__VPRO_PictureDeletionHistory"
const pictureDeletionHistorySQL = `CREATE TABLE "__VPRO_PictureDeletionHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`
const pictureDeletionIdentityTable = "__VPRO_PictureDeletionIdentity"
const pictureDeletionIdentitySQL = `CREATE TABLE "__VPRO_PictureDeletionIdentity"(ID INTEGER PRIMARY KEY,RequestID TEXT NOT NULL)`

type pictureDeletionRequest struct {
	RequestID  string                  `json:"requestId"`
	ContextID  string                  `json:"contextId"`
	Project    string                  `json:"project"`
	PlotNumber string                  `json:"plotNumber"`
	Columns    []ProjectMetadataColumn `json:"columns"`
	Original   ProjectMetadataRow      `json:"original"`
}

func (request *pictureDeletionRequest) UnmarshalJSON(data []byte) error {
	const operation = "picture deletion"
	names := []string{"requestId", "contextId", "project", "plotNumber", "columns", "original"}
	properties, err := sourceChildJSONObject(data, operation, names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, names...); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["columns"], func(raw json.RawMessage) error {
		column, err := sourceChildJSONObject(raw, operation+" column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(column, operation+" column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildRowJSON(properties["original"], operation); err != nil {
		return err
	}
	type plain pictureDeletionRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if _, _, err := preparePictureDeletion(pictureDeletionRequest(decoded)); err != nil {
		return err
	}
	*request = pictureDeletionRequest(decoded)
	return nil
}

type pictureDeletionResult struct {
	Request         pictureDeletionRequest `json:"request"`
	ID              int64                  `json:"id"`
	RequestedSource string                 `json:"requestedSource"`
	Source          string                 `json:"source"`
	OwnedFiles      map[string]string      `json:"ownedFiles"`
	Actor           string                 `json:"actor"`
	AuditStrength   int                    `json:"auditStrength"`
	DeletedWhen     string                 `json:"deletedWhen"`
	HistoryID       string                 `json:"historyId"`
	DidCommit       bool                   `json:"didCommit"`
	Replayed        bool                   `json:"replayed"`
}

func preparePictureDeletion(request pictureDeletionRequest) (int64, int64, error) {
	if !siviDeletionUUID.MatchString(request.RequestID) || !projectNamePattern.MatchString(request.Project) ||
		request.ContextID == "" || request.PlotNumber == "" || !utf8.ValidString(request.PlotNumber) || strings.ContainsRune(request.PlotNumber, 0) {
		return 0, 0, errors.New("picture deletion requires an exact UUID, context, project and literal parent")
	}
	if err := validateChildPhysicalText("Picture deletion context", request.ContextID, 100); err != nil {
		return 0, 0, err
	}
	rowID, err := strconv.ParseInt(request.Original.RowID, 10, 64)
	if err != nil || strconv.FormatInt(rowID, 10) != request.Original.RowID {
		return 0, 0, errors.New("picture deletion requires an exact signed64 physical rowid")
	}
	names := []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"}
	if len(request.Columns) != len(names) || len(request.Original.Cells) != len(names) {
		return 0, 0, errors.New("picture deletion requires all five source columns and cells")
	}
	for i, column := range request.Columns {
		if column.Name != names[i] || !utf8.ValidString(column.DeclaredType) || strings.ContainsRune(column.DeclaredType, 0) {
			return 0, 0, errors.New("picture deletion requires the exact source schema")
		}
		if _, err := metadataCellValue(request.Original.Cells[i]); err != nil {
			return 0, 0, fmt.Errorf("picture deletion original %s: %w", column.Name, err)
		}
	}
	id, parent := request.Original.Cells[0], request.Original.Cells[3]
	if id.Storage != "integer" || id.Integer == nil {
		return 0, 0, errors.New("picture deletion NULL or noninteger identity is unavailable; no identity repair")
	}
	number, err := strconv.ParseInt(*id.Integer, 10, 32)
	if err != nil || strconv.FormatInt(number, 10) != *id.Integer {
		return 0, 0, errors.New("picture deletion requires an exact existing signed32 identity")
	}
	if parent.Storage != "text" || parent.Text == nil || *parent.Text != request.PlotNumber {
		return 0, 0, errors.New("picture deletion original is outside the literal parent")
	}
	return rowID, number, nil
}

type pictureDeletionCommittedError struct{ cause error }

func (err *pictureDeletionCommittedError) Error() string {
	return "picture deletion committed, but cleanup failed; retain the receipt and recover before retrying: " + err.cause.Error()
}
func (err *pictureDeletionCommittedError) Unwrap() error { return err.cause }

func pictureDeletionAuthority(ctx context.Context, plots *PlotService, owner *sqliteContext, contextID string,
	source *ownedPictureLibrary, request pictureDeletionRequest) (pictureDeletionResult, error) {
	if request.ContextID != contextID || request.Project != owner.selection.Project {
		return pictureDeletionResult{}, errors.New("picture deletion reviewed owner changed")
	}
	files, err := pictureMetadataOwnedFiles(plots, owner, source)
	if err != nil {
		return pictureDeletionResult{}, err
	}
	if plots.currentUser == "" || strings.ContainsRune(plots.currentUser, 0) || plots.auditStrength < 0 || plots.auditStrength > 3 {
		return pictureDeletionResult{}, errors.New("picture deletion requires an explicit valid actor and audit policy")
	}
	if err := validateChildPhysicalText("Picture deletion actor", plots.currentUser, 255); err != nil {
		return pictureDeletionResult{}, err
	}
	return pictureDeletionResult{Request: request, RequestedSource: source.requestedPath, Source: source.path,
		OwnedFiles: files, Actor: plots.currentUser, AuditStrength: plots.auditStrength}, ctx.Err()
}

type pictureDeletionHooks struct {
	afterLibraryCommit func(*sql.Tx) error
}

func (s *ContextService) deletePicture(ctx context.Context, contextID string, source *ownedPictureLibrary, request pictureDeletionRequest) (pictureDeletionResult, error) {
	return s.deletePictureWithHooks(ctx, contextID, source, request, pictureDeletionHooks{})
}

func (s *ContextService) deletePictureWithHooks(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureDeletionRequest, hooks pictureDeletionHooks) (pictureDeletionResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return pictureDeletionResult{}, errors.New("picture deletion requires an owned service and request context")
	}
	if _, _, err := preparePictureDeletion(request); err != nil {
		return pictureDeletionResult{}, err
	}
	var receipt pictureDeletionResult
	result, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (pictureDeletionResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (pictureDeletionResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.PlotNumber); err != nil {
				return pictureDeletionResult{}, err
			}
			authority, err := pictureDeletionAuthority(ctx, plots, owner, contextID, source, request)
			if err != nil {
				return pictureDeletionResult{}, err
			}
			return withPictureMetadataParentGuards(ctx, owner, request.PlotNumber, func(freshParent func() error) (pictureDeletionResult, error) {
				receipt, err = writePictureDeletion(ctx, source, request, authority, func() error {
					if err := freshParent(); err != nil {
						return err
					}
					_, err := pictureDeletionAuthority(ctx, plots, owner, contextID, source, request)
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
		return receipt, &pictureDeletionCommittedError{cause: err}
	}
	return result, err
}

func (s *ContextService) lookupPictureDeletionReceipt(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureDeletionRequest) (*pictureDeletionResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return nil, errors.New("picture deletion recovery requires an owned service and request context")
	}
	if _, _, err := preparePictureDeletion(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*pictureDeletionResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*pictureDeletionResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.PlotNumber); err != nil {
				return nil, err
			}
			authority, err := pictureDeletionAuthority(ctx, plots, owner, contextID, source, request)
			if err != nil {
				return nil, err
			}
			return readPictureDeletionReceipt(ctx, source, request, authority)
		})
	})
}

func readPictureDeletionReceipt(ctx context.Context, source *ownedPictureLibrary, request pictureDeletionRequest,
	authority pictureDeletionResult) (*pictureDeletionResult, error) {
	return withPictureHistoryReadSnapshot(ctx, source, func(tx *sql.Tx) (*pictureDeletionResult, error) {
		history, _, err := readPictureDeletionHistory(ctx, tx, source)
		if err != nil {
			return nil, err
		}
		for _, event := range history {
			if event.Request.RequestID != request.RequestID {
				continue
			}
			if err := comparePictureDeletionProvenance(event, authority); err != nil {
				return nil, err
			}
			event.Replayed = true
			return &event, nil
		}
		return nil, nil
	})
}

func comparePictureDeletionProvenance(event, authority pictureDeletionResult) error {
	if !reflect.DeepEqual(event.Request, authority.Request) || event.Actor != authority.Actor ||
		event.AuditStrength != authority.AuditStrength || event.Source != authority.Source ||
		event.RequestedSource != authority.RequestedSource || !reflect.DeepEqual(event.OwnedFiles, authority.OwnedFiles) {
		return errors.New("picture deletion request identity collides with incompatible provenance")
	}
	return nil
}

func writePictureDeletion(ctx context.Context, source *ownedPictureLibrary, request pictureDeletionRequest,
	authority pictureDeletionResult, validate func() error) (result pictureDeletionResult, resultErr error) {
	rowID, id, err := preparePictureDeletion(request)
	if err != nil {
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

	history, reservations, err := readPictureDeletionHistory(ctx, tx, source)
	if err != nil {
		return result, err
	}
	for _, event := range history {
		if event.Request.RequestID != request.RequestID {
			continue
		}
		if err := comparePictureDeletionProvenance(event, authority); err != nil {
			return result, err
		}
		if err := errors.Join(validate(), source.checkFile(), ctx.Err()); err != nil {
			return result, err
		}
		event.Replayed = true
		return event, nil
	}
	before, selected, err := pictureMetadataSelectedRow(ctx, tx, request.PlotNumber,
		pictureMetadataEdit{ID: id, Columns: request.Columns, Original: request.Original})
	if err != nil {
		return result, fmt.Errorf("picture deletion target: %w", err)
	}
	if !reflect.DeepEqual(selected, request.Original) {
		return result, errors.New("picture deletion complete physical row changed; reload before deleting")
	}
	for _, definition := range []string{pictureDeletionHistorySQL, pictureDeletionIdentitySQL} {
		if _, err := tx.ExecContext(ctx, strings.Replace(definition, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)); err != nil {
			return result, err
		}
	}
	if _, reserved := reservations[id]; !reserved {
		if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_PictureDeletionIdentity"(ID,RequestID) VALUES(?,?)`, id, request.RequestID); err != nil {
			return result, err
		}
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM tblVPics WHERE rowid=? AND typeof(ID)='integer' AND ID=?
		AND typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB)`, rowID, id, request.PlotNumber)
	if err != nil {
		return result, err
	}
	count, err := deleted.RowsAffected()
	if err != nil || count != 1 {
		return result, errors.Join(err, errors.New("picture deletion expected exactly one physical row"))
	}
	expected := ProjectMetadataTable{Columns: before.Columns, Rows: []ProjectMetadataRow{}}
	for _, row := range before.Rows {
		if row.RowID != request.Original.RowID {
			expected.Rows = append(expected.Rows, row)
		}
	}
	after, err := readPictureLibraryRows(ctx, tx, request.PlotNumber)
	if err != nil || !reflect.DeepEqual(after, expected) {
		return result, errors.Join(err, errors.New("picture deletion differs from the complete physical plan"))
	}
	receipt := authority
	receipt.ID, receipt.DidCommit = id, true
	receipt.DeletedWhen = time.Now().UTC().Format(time.RFC3339Nano)
	var historyID int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ID),0) FROM "__VPRO_PictureDeletionHistory"`).Scan(&historyID); err != nil {
		return result, err
	}
	if historyID == math.MaxInt64 {
		return result, errors.New("picture deletion history identity space exhausted")
	}
	receipt.HistoryID = strconv.FormatInt(historyID+1, 10)
	proposal, err := json.Marshal(receipt)
	if err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_PictureDeletionHistory"(ID,Created,Proposal) VALUES(?,?,?)`,
		historyID+1, receipt.DeletedWhen, string(proposal)); err != nil {
		return result, err
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
