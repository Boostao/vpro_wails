package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const pictureMetadataHistoryTable = "__VPRO_PictureMetadataHistory"
const pictureMetadataHistorySQL = `CREATE TABLE "__VPRO_PictureMetadataHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

type pictureMetadataEdit struct {
	RequestID string                  `json:"requestId"`
	ID        int64                   `json:"id"`
	Columns   []ProjectMetadataColumn `json:"columns"`
	Original  ProjectMetadataRow      `json:"original"`
	Changes   []ProjectMetadataChange `json:"changes"`
}

func (request *pictureMetadataEdit) UnmarshalJSON(data []byte) error {
	const operation = "picture metadata edit"
	properties, err := sourceChildJSONObject(data, operation, []string{"requestId", "id", "columns", "original", "changes"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, "requestId", "id", "columns", "original", "changes"); err != nil {
		return err
	}
	if err := sourceChildRowJSON(properties["original"], operation); err != nil {
		return err
	}
	if err := pictureMetadataSchemaAndChangesJSON(properties, operation); err != nil {
		return err
	}
	type plain pictureMetadataEdit
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if _, _, err := preparePictureMetadataEdit(pictureMetadataEdit(decoded)); err != nil {
		return err
	}
	*request = pictureMetadataEdit(decoded)
	return nil
}

func pictureMetadataSchemaAndChangesJSON(properties map[string]json.RawMessage, operation string) error {
	if err := sourceChildArrayJSON(properties["columns"], func(raw json.RawMessage) error {
		p, err := sourceChildJSONObject(raw, operation+" column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(p, operation+" column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["changes"], func(raw json.RawMessage) error {
		p, err := sourceChildJSONObject(raw, operation+" change", []string{"column", "value"}, nil)
		if err != nil {
			return err
		}
		if err := sourceChildJSONNonNull(p, operation+" change", "column", "value"); err != nil {
			return err
		}
		return sourceChildCellJSON(p["value"], operation)
	}); err != nil {
		return err
	}
	return nil
}

type pictureMetadataWriteResult struct {
	RequestID       string                  `json:"requestId"`
	ContextID       string                  `json:"contextId"`
	Project         string                  `json:"project"`
	PlotNumber      string                  `json:"plotNumber"`
	RequestedSource string                  `json:"requestedSource"`
	Source          string                  `json:"source"`
	OwnedFiles      map[string]string       `json:"ownedFiles"`
	Actor           string                  `json:"actor"`
	AuditStrength   int                     `json:"auditStrength"`
	ID              int64                   `json:"id"`
	Columns         []ProjectMetadataColumn `json:"columns"`
	Original        ProjectMetadataRow      `json:"original"`
	Committed       ProjectMetadataRow      `json:"committed"`
	Changes         []ProjectMetadataChange `json:"changes"`
	EditWhen        string                  `json:"editWhen"`
	HistoryID       string                  `json:"historyId"`
	DidCommit       bool                    `json:"didCommit"`
	Replayed        bool                    `json:"replayed"`
}

type pictureMetadataHistory struct {
	Request pictureMetadataEdit        `json:"request"`
	Result  pictureMetadataWriteResult `json:"result"`
}

func (history *pictureMetadataHistory) UnmarshalJSON(data []byte) error {
	const operation = "picture metadata history"
	properties, err := sourceChildJSONObject(data, operation, []string{"request", "result"}, nil)
	if err != nil {
		return err
	}
	names := []string{"requestId", "contextId", "project", "plotNumber", "requestedSource", "source", "ownedFiles",
		"actor", "auditStrength", "id", "columns", "original", "committed", "changes", "editWhen", "historyId", "didCommit", "replayed"}
	result, err := sourceChildJSONObject(properties["result"], operation+" result", names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(result, operation, names...); err != nil {
		return err
	}
	for _, name := range []string{"original", "committed"} {
		if err := sourceChildRowJSON(result[name], operation); err != nil {
			return err
		}
	}
	if err := pictureMetadataSchemaAndChangesJSON(result, operation); err != nil {
		return err
	}
	type plain pictureMetadataHistory
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*history = pictureMetadataHistory(decoded)
	return nil
}

type pictureMetadataCommittedError struct{ cause error }

func (err *pictureMetadataCommittedError) Error() string {
	return "picture metadata committed, but cleanup failed; retain the receipt and reload before retrying: " + err.cause.Error()
}

func (err *pictureMetadataCommittedError) Unwrap() error { return err.cause }

func preparePictureMetadataEdit(request pictureMetadataEdit) (int64, []ProjectMetadataChange, error) {
	if request.RequestID == "" || strings.ContainsRune(request.RequestID, 0) {
		return 0, nil, errors.New("picture edit requires an explicit nonempty request identity")
	}
	if err := validateChildPhysicalText("Picture request identity", request.RequestID, 255); err != nil {
		return 0, nil, err
	}
	if request.ID < -2147483648 || request.ID > 2147483647 {
		return 0, nil, errors.New("picture edit requires an exact signed32 ID")
	}
	rowID, err := strconv.ParseInt(request.Original.RowID, 10, 64)
	if err != nil || strconv.FormatInt(rowID, 10) != request.Original.RowID {
		return 0, nil, errors.New("picture edit requires an exact signed64 physical rowid")
	}
	names := []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"}
	if len(request.Columns) != len(names) || len(request.Original.Cells) != len(names) || len(request.Changes) == 0 {
		return 0, nil, errors.New("picture edit requires five source columns/cells and explicit changes")
	}
	for i, column := range request.Columns {
		if column.Name != names[i] {
			return 0, nil, errors.New("picture edit requires the exact physical source column order")
		}
		if _, err := metadataCellValue(request.Original.Cells[i]); err != nil {
			return 0, nil, fmt.Errorf("picture original %s: %w", column.Name, err)
		}
	}
	id := request.Original.Cells[0]
	if id.Storage != "integer" || id.Integer == nil || *id.Integer != strconv.FormatInt(request.ID, 10) {
		return 0, nil, errors.New("picture selected ID must be the reviewed signed32 integer, not a repaired historical value")
	}
	seen := map[string]bool{}
	changes := []ProjectMetadataChange{}
	for _, change := range request.Changes {
		i := 1
		if change.Column == "PicName" {
			i = 2
		} else if change.Column != "PicDir" {
			return 0, nil, fmt.Errorf("picture field %q is not writable", change.Column)
		}
		if seen[change.Column] {
			return 0, nil, errors.New("picture edit repeats a field")
		}
		seen[change.Column] = true
		if _, err := metadataCellValue(change.Value); err != nil {
			return 0, nil, err
		}
		if reflect.DeepEqual(change.Value, request.Original.Cells[i]) {
			continue
		}
		if change.Value.Storage != "null" && change.Value.Storage != "text" {
			return 0, nil, errors.New("new picture values require nullable text")
		}
		if change.Value.Text != nil {
			if err := validateChildPhysicalText("Picture."+change.Column, *change.Value.Text, 255); err != nil {
				return 0, nil, err
			}
			if strings.ContainsRune(*change.Value.Text, 0) || change.Column == "PicName" && *change.Value.Text == "" {
				return 0, nil, errors.New("new picture text must not contain NUL; PicName must not be empty")
			}
		}
		changes = append(changes, change)
	}
	return rowID, changes, nil
}

func pictureMetadataOwnedFiles(plots *PlotService, owner *sqliteContext, source *ownedPictureLibrary) (map[string]string, error) {
	if err := source.checkFile(); err != nil {
		return nil, err
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	files := map[string]string{}
	for role, path := range owner.attachments {
		files["attachment:"+role] = path
	}
	for role, path := range plots.projects.supportPaths {
		files["support:"+role] = path
	}
	for role, path := range map[string]string{
		"project": owner.selection.ProjectPath, "siteUnit": owner.selection.SUPath, "hierarchy": owner.selection.HierarchyPath,
	} {
		files["selection:"+role] = path
	}
	if owner.profile != nil {
		files["profile"] = owner.profile.Path
	}
	for role, path := range files {
		if path == "" {
			delete(files, role)
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("picture writer owned %s unavailable: %w", role, err)
		}
		if os.SameFile(info, source.info) {
			return nil, fmt.Errorf("picture library aliases context-owned %s; no write is authorized", role)
		}
	}
	return files, nil
}

func pictureMetadataSelectedRow(ctx context.Context, tx *sql.Tx, plot string, request pictureMetadataEdit) (ProjectMetadataTable, ProjectMetadataRow, error) {
	records, err := readPictureLibraryRows(ctx, tx, plot)
	if err != nil {
		return records, ProjectMetadataRow{}, err
	}
	if !reflect.DeepEqual(records.Columns, request.Columns) {
		return records, ProjectMetadataRow{}, errors.New("picture schema changed; reload before saving")
	}
	var count int
	// Noninteger numeric aliases cannot establish a unique source Long identity.
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tblVPics WHERE ID=? OR
		(typeof(ID) IN ('text','real','blob') AND CAST(ID AS NUMERIC)=?)`, request.ID, request.ID).Scan(&count); err != nil {
		return records, ProjectMetadataRow{}, err
	}
	if count != 1 {
		return records, ProjectMetadataRow{}, errors.New("picture ID is missing or ambiguous across the library")
	}
	for _, row := range records.Rows {
		if row.RowID == request.Original.RowID && row.Cells[0].Storage == "integer" &&
			row.Cells[0].Integer != nil && *row.Cells[0].Integer == strconv.FormatInt(request.ID, 10) {
			return records, row, nil
		}
	}
	return records, ProjectMetadataRow{}, errors.New("picture physical row/ID is outside the literal parent")
}

func pictureMetadataWriterSchema(ctx context.Context, tx *sql.Tx) error {
	var triggers int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND tbl_name COLLATE NOCASE='tblVPics'`).Scan(&triggers); err != nil {
		return err
	}
	if triggers != 0 {
		return errors.New("picture writer refuses unreviewed source triggers")
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_list(tblVPics)`)
	if err != nil {
		return err
	}
	hasForeignKeys := rows.Next()
	err = errors.Join(rows.Err(), rows.Close())
	if hasForeignKeys {
		err = errors.Join(err, errors.New("picture writer refuses unreviewed source foreign keys"))
	}
	return err
}

type pictureMetadataParentGuard struct {
	path string
	info os.FileInfo
	db   *sql.DB
	tx   *sql.Tx
}

// Writer reservations prevent independent parent/SU edits under WAL as well as
// rollback journals. These transactions only observe and roll back; they are
// not a distributed commit with the picture-library mutation.
func withPictureMetadataParentGuards[T any](ctx context.Context, owner *sqliteContext, plot string,
	operation func(func() error) (T, error)) (result T, resultErr error) {
	var guards []*pictureMetadataParentGuard
	roles := map[string]*pictureMetadataParentGuard{}
	defer func() {
		for i := len(guards) - 1; i >= 0; i-- {
			guard := guards[i]
			if guard.tx != nil {
				if err := guard.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}
			if guard.db != nil {
				resultErr = errors.Join(resultErr, guard.db.Close())
			}
		}
	}()
	wanted := []string{"project"}
	if owner.selection.SU != "None" {
		wanted = append(wanted, "su")
	}
	for _, role := range wanted {
		path, info := owner.attachments[role], owner.attachmentInfo[role]
		if path == "" || info == nil {
			return result, fmt.Errorf("picture parent guard requires the owned %s file", role)
		}
		for _, guard := range guards {
			if os.SameFile(info, guard.info) {
				roles[role] = guard
				break
			}
		}
		if roles[role] == nil {
			guard := &pictureMetadataParentGuard{path: path, info: info}
			guards = append(guards, guard)
			roles[role] = guard
		}
	}
	sort.Slice(guards, func(i, j int) bool { return strings.ToLower(guards[i].path) < strings.ToLower(guards[j].path) })
	for _, guard := range guards {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		current, err := os.Stat(guard.path)
		if err != nil || !os.SameFile(current, guard.info) {
			return result, errors.Join(err, errors.New("picture parent file identity changed before reservation"))
		}
		guard.db, err = sql.Open("sqlite3", sqliteFileURI(guard.path, "rw")+"&_busy_timeout=5000&_txlock=immediate")
		if err != nil {
			return result, err
		}
		guard.db.SetMaxOpenConns(1)
		// Cancellation must not asynchronously release a parent reservation while
		// the library commit is in progress. Cleanup remains caller-owned.
		guard.tx, err = guard.db.BeginTx(context.WithoutCancel(ctx), nil)
		if err != nil {
			return result, err
		}
	}
	validate := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return err
		}
		var member bool
		project := owner.selection.Project
		query := `SELECT EXISTS(SELECT 1 FROM ` + quoteHeaderIdentifier(project+"_Env") + ` AS env
			INNER JOIN ` + quoteHeaderIdentifier(project+"_Admin") + ` AS admin ON env.PlotNumber=admin.Plot
			WHERE typeof(env.PlotNumber)='text' AND CAST(env.PlotNumber AS BLOB)=CAST(? AS BLOB))`
		if err := roles["project"].tx.QueryRowContext(ctx, query, plot).Scan(&member); err != nil {
			return fmt.Errorf("fresh picture parent scope unavailable: %w", err)
		}
		if !member {
			return errors.New("picture parent left the independently reserved project scope")
		}
		if owner.selection.SU != "None" {
			query := `SELECT EXISTS(SELECT 1 FROM ` + quoteHeaderIdentifier(owner.selection.SU+"_SU") +
				` WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB))`
			if err := roles["su"].tx.QueryRowContext(ctx, query, plot).Scan(&member); err != nil {
				return fmt.Errorf("fresh picture SU scope unavailable: %w", err)
			}
			if !member {
				return errors.New("picture parent left the independently reserved literal SU scope")
			}
		}
		return nil
	}
	if err := validate(); err != nil {
		return result, err
	}
	return operation(validate)
}

func (s *ContextService) editPictureMetadata(ctx context.Context, contextID, plot string, source *ownedPictureLibrary, request pictureMetadataEdit) (pictureMetadataWriteResult, error) {
	rowID, changes, err := preparePictureMetadataEdit(request)
	if err != nil {
		return pictureMetadataWriteResult{}, err
	}
	var receipt pictureMetadataWriteResult
	result, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (pictureMetadataWriteResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (pictureMetadataWriteResult, error) {
			if err := requirePictureParent(ctx, owner, parent, plot); err != nil {
				return pictureMetadataWriteResult{}, err
			}
			files, err := pictureMetadataOwnedFiles(plots, owner, source)
			if err != nil {
				return pictureMetadataWriteResult{}, err
			}
			if plots.currentUser == "" {
				return pictureMetadataWriteResult{}, errors.New("picture audit requires an explicit actor")
			}
			if strings.ContainsRune(plots.currentUser, 0) {
				return pictureMetadataWriteResult{}, errors.New("picture audit actor must not contain NUL")
			}
			if err := validateChildPhysicalText("Picture audit actor", plots.currentUser, 255); err != nil {
				return pictureMetadataWriteResult{}, err
			}
			return withPictureMetadataParentGuards(ctx, owner, plot, func(freshParent func() error) (pictureMetadataWriteResult, error) {
				receipt, err = writePictureMetadata(ctx, source, request, rowID, changes, pictureMetadataWriteResult{
					RequestID: request.RequestID, ContextID: contextID, Project: owner.selection.Project, PlotNumber: plot,
					RequestedSource: source.requestedPath, Source: source.path, OwnedFiles: files, Actor: plots.currentUser,
					AuditStrength: plots.auditStrength, ID: request.ID, Columns: request.Columns, Original: request.Original,
				}, func() error {
					if err := freshParent(); err != nil {
						return err
					}
					_, err := pictureMetadataOwnedFiles(plots, owner, source)
					return err
				})
				return receipt, err
			})
		})
	})
	if err != nil && receipt.DidCommit {
		return receipt, &pictureMetadataCommittedError{cause: err}
	}
	return result, errors.Join(err, ctx.Err())
}

// A nil receipt means no matching durable event was found, not proof that a
// save failed. Callers must retain their draft and unknown barrier in that case.
func (s *ContextService) lookupPictureMetadataReceipt(ctx context.Context, contextID, plot string, source *ownedPictureLibrary,
	request pictureMetadataEdit) (*pictureMetadataWriteResult, error) {
	if _, _, err := preparePictureMetadataEdit(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*pictureMetadataWriteResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*pictureMetadataWriteResult, error) {
			if err := requirePictureParent(ctx, owner, parent, plot); err != nil {
				return nil, err
			}
			files, err := pictureMetadataOwnedFiles(plots, owner, source)
			if err != nil {
				return nil, err
			}
			return readPictureMetadataReceipt(ctx, source, request, pictureMetadataWriteResult{
				ContextID: contextID, Project: owner.selection.Project, PlotNumber: plot,
				RequestedSource: source.requestedPath, Source: source.path, OwnedFiles: files,
				Actor: plots.currentUser, AuditStrength: plots.auditStrength,
			})
		})
	})
}

func readPictureMetadataReceipt(ctx context.Context, source *ownedPictureLibrary, request pictureMetadataEdit,
	authority pictureMetadataWriteResult) (result *pictureMetadataWriteResult, resultErr error) {
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro")+"&_query_only=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, db.Close())
		if resultErr != nil {
			result = nil
		}
	}()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	result, err = replayPictureMetadata(ctx, tx, request, authority)
	if err != nil {
		return nil, err
	}
	if err := source.checkFile(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, tx.Commit()
}

func writePictureMetadata(ctx context.Context, source *ownedPictureLibrary, request pictureMetadataEdit, rowID int64,
	changes []ProjectMetadataChange, receipt pictureMetadataWriteResult, validate func() error) (result pictureMetadataWriteResult, resultErr error) {
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	if err := validate(); err != nil {
		return result, err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := pictureMetadataWriterSchema(ctx, tx); err != nil {
		return result, err
	}
	replayed, err := replayPictureMetadata(ctx, tx, request, receipt)
	if err != nil {
		return result, err
	}
	if replayed != nil {
		if err := validate(); err != nil {
			return result, err
		}
		return *replayed, ctx.Err()
	}
	records, row, err := pictureMetadataSelectedRow(ctx, tx, receipt.PlotNumber, request)
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(row, request.Original) {
		return result, errors.New("picture original row changed; reload before saving")
	}
	receipt.Committed = row
	receipt.Changes = changes
	if len(changes) == 0 {
		if err := validate(); err != nil {
			return result, err
		}
		return receipt, ctx.Err()
	}
	planned := ProjectMetadataRow{RowID: row.RowID, Cells: append([]ProjectMetadataCell{}, row.Cells...)}
	var sets []string
	var arguments []any
	for _, change := range changes {
		i := 1
		if change.Column == "PicName" {
			i = 2
		}
		planned.Cells[i] = change.Value
		value, err := metadataCellValue(change.Value)
		if err != nil {
			return result, err
		}
		sets, arguments = append(sets, quoteHeaderIdentifier(change.Column)+"=?"), append(arguments, value)
	}
	mutation, err := tx.ExecContext(ctx, `UPDATE tblVPics SET `+strings.Join(sets, ",")+
		` WHERE rowid=? AND typeof(ID)='integer' AND ID=? AND typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB)`,
		append(arguments, rowID, request.ID, receipt.PlotNumber)...)
	if err != nil {
		return result, err
	}
	if count, err := mutation.RowsAffected(); err != nil || count != 1 {
		return result, errors.Join(err, fmt.Errorf("picture mutation expected one row, found %d", count))
	}
	after, observed, err := pictureMetadataSelectedRow(ctx, tx, receipt.PlotNumber, request)
	if err != nil {
		return result, err
	}
	expected := ProjectMetadataTable{Columns: records.Columns, Rows: append([]ProjectMetadataRow{}, records.Rows...)}
	for i := range expected.Rows {
		if expected.Rows[i].RowID == row.RowID {
			expected.Rows[i] = planned
		}
	}
	if !reflect.DeepEqual(observed, planned) || !reflect.DeepEqual(after, expected) {
		return result, errors.New("picture committed storage differs from the complete plan; rolled back")
	}
	receipt.Committed = observed
	receipt.EditWhen = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.DidCommit = true
	history, err := json.Marshal(pictureMetadataHistory{request, receipt})
	if err != nil {
		return result, err
	}
	if err := appendTechnicalProvenance(ctx, tx, pictureMetadataHistoryTable, pictureMetadataHistorySQL, string(history), "picture metadata"); err != nil {
		return result, err
	}
	var historyID int64
	if err := tx.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&historyID); err != nil {
		return result, err
	}
	receipt.HistoryID = strconv.FormatInt(historyID, 10)
	if err := validate(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return receipt, nil
}

func replayPictureMetadata(ctx context.Context, tx *sql.Tx, request pictureMetadataEdit, authority pictureMetadataWriteResult) (*pictureMetadataWriteResult, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE BINARY=?`, pictureMetadataHistoryTable).Scan(&exists); err != nil || exists == 0 {
		return nil, err
	}
	if err := verifyTechnicalProvenance(ctx, tx, pictureMetadataHistoryTable, pictureMetadataHistorySQL, "picture metadata"); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT ID,Proposal,Restored FROM "__VPRO_PictureMetadataHistory" ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	var found *pictureMetadataWriteResult
	for rows.Next() {
		var id int64
		var proposal string
		var restored sql.NullString
		if err := rows.Scan(&id, &proposal, &restored); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		var history pictureMetadataHistory
		if err := json.Unmarshal([]byte(proposal), &history); err != nil {
			return nil, errors.Join(errors.New("picture history is malformed; no receipt inferred"), err, rows.Close())
		}
		if history.Request.RequestID != request.RequestID {
			continue
		}
		result := history.Result
		_, changes, planErr := preparePictureMetadataEdit(history.Request)
		planned := ProjectMetadataRow{RowID: request.Original.RowID, Cells: append([]ProjectMetadataCell{}, request.Original.Cells...)}
		for _, change := range changes {
			i := 1
			if change.Column == "PicName" {
				i = 2
			}
			planned.Cells[i] = change.Value
		}
		_, whenErr := time.Parse(time.RFC3339Nano, result.EditWhen)
		if found != nil || restored.Valid || !reflect.DeepEqual(history.Request, request) || result.RequestID != request.RequestID ||
			result.ContextID != authority.ContextID || result.Project != authority.Project || result.PlotNumber != authority.PlotNumber ||
			result.Source != authority.Source || result.RequestedSource != authority.RequestedSource ||
			result.Actor != authority.Actor || result.AuditStrength != authority.AuditStrength ||
			!reflect.DeepEqual(result.OwnedFiles, authority.OwnedFiles) || !result.DidCommit || result.Replayed ||
			result.HistoryID != "" || planErr != nil || whenErr != nil || len(changes) == 0 ||
			!reflect.DeepEqual(result.Changes, changes) || !reflect.DeepEqual(result.Committed, planned) ||
			result.ID != request.ID || !reflect.DeepEqual(result.Original, request.Original) || !reflect.DeepEqual(result.Columns, request.Columns) {
			return nil, errors.Join(errors.New("picture request identity collides with different or unavailable committed provenance"), rows.Close())
		}
		result.HistoryID, result.Replayed = strconv.FormatInt(id, 10), true
		found = &result
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if found != nil {
		_, observed, err := pictureMetadataSelectedRow(ctx, tx, authority.PlotNumber, request)
		if err != nil || !reflect.DeepEqual(observed, found.Committed) {
			return nil, errors.Join(errors.New("picture receipt no longer matches stored row; reload without repeating the write"), err)
		}
	}
	return found, nil
}
