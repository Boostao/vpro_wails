package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const pictureCreationHistoryTable = "__VPRO_PictureCreationHistory"
const pictureCreationHistorySQL = `CREATE TABLE "__VPRO_PictureCreationHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`
const pictureCreationIdentityTable = "__VPRO_PictureCreationIdentity"
const pictureCreationIdentitySQL = `CREATE TABLE "__VPRO_PictureCreationIdentity"(ID INTEGER PRIMARY KEY,RequestID TEXT NOT NULL)`

// Private, unregistered draft boundary: a selected path is storage provenance,
// not an image-read grant or proof of a native chooser selection.
type pictureCreationRequest struct {
	ContextID    string `json:"contextId"`
	Project      string `json:"project"`
	PlotNumber   string `json:"plotNumber"`
	RequestID    string `json:"requestId"`
	SelectedPath string `json:"selectedPath"`
}

func (request *pictureCreationRequest) UnmarshalJSON(data []byte) error {
	names := []string{"contextId", "project", "plotNumber", "requestId", "selectedPath"}
	properties, err := sourceChildJSONObject(data, "picture creation", names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "picture creation", names...); err != nil {
		return err
	}
	type plain pictureCreationRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if _, _, err := preparePictureCreation(pictureCreationRequest(decoded)); err != nil {
		return err
	}
	*request = pictureCreationRequest(decoded)
	return nil
}

type pictureCreationResult struct {
	RequestID       string                  `json:"requestId"`
	ContextID       string                  `json:"contextId"`
	Project         string                  `json:"project"`
	PlotNumber      string                  `json:"plotNumber"`
	SelectedPath    string                  `json:"selectedPath"`
	RequestedSource string                  `json:"requestedSource"`
	Source          string                  `json:"source"`
	OwnedFiles      map[string]string       `json:"ownedFiles"`
	Actor           string                  `json:"actor"`
	AuditStrength   int                     `json:"auditStrength"`
	ID              int64                   `json:"id"`
	RowID           string                  `json:"rowId"`
	Columns         []ProjectMetadataColumn `json:"columns"`
	Committed       ProjectMetadataRow      `json:"committed"`
	CreatedWhen     string                  `json:"createdWhen"`
	HistoryID       string                  `json:"historyId"`
	DidCommit       bool                    `json:"didCommit"`
	Replayed        bool                    `json:"replayed"`
}

type pictureCreationHistory struct {
	Request pictureCreationRequest `json:"request"`
	Result  pictureCreationResult  `json:"result"`
}

func (history *pictureCreationHistory) UnmarshalJSON(data []byte) error {
	properties, err := sourceChildJSONObject(data, "picture creation history", []string{"request", "result"}, nil)
	if err != nil {
		return err
	}
	names := []string{"requestId", "contextId", "project", "plotNumber", "selectedPath", "requestedSource", "source",
		"ownedFiles", "actor", "auditStrength", "id", "rowId", "columns", "committed", "createdWhen", "historyId", "didCommit", "replayed"}
	result, err := sourceChildJSONObject(properties["result"], "picture creation receipt", names, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(result, "picture creation receipt", names...); err != nil {
		return err
	}
	if err := sourceChildRowJSON(result["committed"], "picture creation receipt"); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(result["columns"], func(raw json.RawMessage) error {
		p, err := sourceChildJSONObject(raw, "picture creation column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(p, "picture creation column", "name", "declaredType")
	}); err != nil {
		return err
	}
	// The owned-file map is open-keyed, but decoded-key collisions are not.
	var files map[string]json.RawMessage
	if err := json.Unmarshal(result["ownedFiles"], &files); err != nil {
		return err
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	if _, err := sourceChildJSONObject(result["ownedFiles"], "picture creation owned files", keys, nil); err != nil {
		return err
	}
	type plain pictureCreationHistory
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*history = pictureCreationHistory(decoded)
	return nil
}

func preparePictureCreation(request pictureCreationRequest) (string, string, error) {
	if !siviDeletionUUID.MatchString(request.RequestID) {
		return "", "", errors.New("picture creation requires a lowercase UUIDv4 requestId")
	}
	if request.ContextID == "" || !projectNamePattern.MatchString(request.Project) || request.PlotNumber == "" {
		return "", "", errors.New("picture creation requires an exact reviewed context, project and parent")
	}
	for _, field := range []struct {
		name  string
		value string
		limit int
	}{{"context", request.ContextID, 255}, {"project", request.Project, 255}, {"parent", request.PlotNumber, 7}} {
		if err := validateChildPhysicalText("Picture creation "+field.name, field.value, field.limit); err != nil {
			return "", "", err
		}
		if strings.ContainsRune(field.value, 0) {
			return "", "", errors.New("picture creation identity contains NUL")
		}
	}
	return splitPictureCreationPath(request.SelectedPath)
}

func splitPictureCreationPath(path string) (string, string, error) {
	refuse := func() (string, string, error) {
		return "", "", errors.New("picture creation requires an unambiguous literal fully qualified Windows file path")
	}
	if runtime.GOOS != "windows" || !utf8.ValidString(path) || strings.ContainsAny(path, "\x00/") {
		return refuse()
	}
	var components []string
	if len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1:3] == `:\` {
		components = strings.Split(path[3:], `\`)
	} else if strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, `\\?\`) && !strings.HasPrefix(path, `\\.\`) {
		components = strings.Split(path[2:], `\`)
		if len(components) < 3 {
			return refuse()
		}
	} else {
		return refuse()
	}
	for _, component := range components {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `<>:"|?*`) ||
			strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return refuse()
		}
		for _, r := range component {
			if r < 32 {
				return refuse()
			}
		}
		stem := strings.ToUpper(strings.TrimRight(strings.SplitN(component, ".", 2)[0], " "))
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
			(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) &&
				(strings.ContainsRune("123456789", rune(stem[3])))) {
			return refuse()
		}
		// Superscript device digits are multi-byte; Windows treats them as aliases.
		for _, prefix := range []string{"COM", "LPT"} {
			for _, digit := range []string{"¹", "²", "³"} {
				if stem == prefix+digit {
					return refuse()
				}
			}
		}
	}
	if !filepath.IsAbs(path) {
		return refuse()
	}
	// Split, unlike Clean/Dir, retains the selected literal prefix and separator.
	dir, name := filepath.Split(path)
	if name == "" || !strings.HasSuffix(dir, `\`) || dir+name != path {
		return refuse()
	}
	if err := validateChildPhysicalText("Picture.PicDir", dir, 255); err != nil {
		return "", "", err
	}
	if err := validateChildPhysicalText("Picture.PicName", name, 255); err != nil {
		return "", "", err
	}
	return dir, name, nil
}

func pictureCreationCells(id int64, dir, name, plot string) []ProjectMetadataCell {
	raw := strconv.FormatInt(id, 10)
	return []ProjectMetadataCell{
		{Storage: "integer", Integer: &raw}, {Storage: "text", Text: &dir},
		{Storage: "text", Text: &name}, {Storage: "text", Text: &plot}, {Storage: "null"},
	}
}

func pictureCreationAuthority(ctx context.Context, plots *PlotService, owner *sqliteContext, contextID string,
	source *ownedPictureLibrary, request pictureCreationRequest) (pictureCreationResult, error) {
	if request.ContextID != contextID || request.Project != owner.selection.Project {
		return pictureCreationResult{}, errors.New("picture creation reviewed owner changed")
	}
	files, err := pictureMetadataOwnedFiles(plots, owner, source)
	if err != nil {
		return pictureCreationResult{}, err
	}
	if plots.currentUser == "" || strings.ContainsRune(plots.currentUser, 0) || plots.auditStrength < 0 || plots.auditStrength > 3 {
		return pictureCreationResult{}, errors.New("picture creation requires an explicit valid actor and audit configuration")
	}
	if err := validateChildPhysicalText("Picture creation actor", plots.currentUser, 255); err != nil {
		return pictureCreationResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return pictureCreationResult{}, err
	}
	return pictureCreationResult{
		RequestID: request.RequestID, ContextID: contextID, Project: request.Project, PlotNumber: request.PlotNumber,
		SelectedPath: request.SelectedPath, RequestedSource: source.requestedPath, Source: source.path,
		OwnedFiles: files, Actor: plots.currentUser, AuditStrength: plots.auditStrength,
	}, nil
}

type pictureCreationHooks struct {
	afterLibraryCommit func(*sql.Tx) error
}

func (s *ContextService) createPicture(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureCreationRequest) (pictureCreationResult, error) {
	return s.createPictureWithHooks(ctx, contextID, source, request, pictureCreationHooks{})
}

func (s *ContextService) createPictureWithHooks(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureCreationRequest, hooks pictureCreationHooks) (pictureCreationResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return pictureCreationResult{}, errors.New("picture creation requires an owned context service and request context")
	}
	if _, _, err := preparePictureCreation(request); err != nil {
		return pictureCreationResult{}, err
	}
	var receipt pictureCreationResult
	result, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (pictureCreationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (pictureCreationResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.PlotNumber); err != nil {
				return pictureCreationResult{}, err
			}
			authority, err := pictureCreationAuthority(ctx, plots, owner, contextID, source, request)
			if err != nil {
				return pictureCreationResult{}, err
			}
			return withPictureMetadataParentGuards(ctx, owner, request.PlotNumber, func(freshParent func() error) (pictureCreationResult, error) {
				receipt, err = writePictureCreation(ctx, source, request, authority, func() error {
					if err := freshParent(); err != nil {
						return err
					}
					_, err := pictureCreationAuthority(ctx, plots, owner, contextID, source, request)
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
		return receipt, &pictureMetadataCommittedError{cause: err}
	}
	return result, err
}

func (s *ContextService) lookupPictureCreationReceipt(ctx context.Context, contextID string, source *ownedPictureLibrary,
	request pictureCreationRequest) (*pictureCreationResult, error) {
	if ctx == nil || s == nil || s.projects == nil || s.plots == nil {
		return nil, errors.New("picture creation receipt requires an owned context service and request context")
	}
	if _, _, err := preparePictureCreation(request); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*pictureCreationResult, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, parent *sql.Tx) (*pictureCreationResult, error) {
			if err := requirePictureParent(ctx, owner, parent, request.PlotNumber); err != nil {
				return nil, err
			}
			authority, err := pictureCreationAuthority(ctx, plots, owner, contextID, source, request)
			if err != nil {
				return nil, err
			}
			return readPictureCreationReceipt(ctx, source, request, authority)
		})
	})
}

func pictureCreationSnapshot(ctx context.Context, tx *sql.Tx, plot string) (ProjectMetadataTable, error) {
	if err := pictureMetadataWriterSchema(ctx, tx); err != nil {
		return ProjectMetadataTable{}, err
	}
	if _, err := readPictureLibraryRows(ctx, tx, plot); err != nil {
		return ProjectMetadataTable{}, err
	}
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE name COLLATE BINARY='tblVPics' AND type='table'`).Scan(&definition); err != nil {
		return ProjectMetadataTable{}, err
	}
	upper := strings.ToUpper(definition)
	if strings.Contains(upper, "VIRTUAL TABLE") || strings.Contains(upper, "WITHOUT ROWID") {
		return ProjectMetadataTable{}, errors.New("picture creation requires an ordinary rowid table")
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA table_xinfo(tblVPics)`)
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	for rows.Next() {
		var ordinal, required, primary, hidden int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&ordinal, &name, &kind, &required, &defaultValue, &primary, &hidden); err != nil {
			return ProjectMetadataTable{}, errors.Join(err, rows.Close())
		}
		if defaultValue.Valid {
			return ProjectMetadataTable{}, errors.Join(errors.New("picture creation refuses unreviewed SQLite source defaults"), rows.Close())
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return ProjectMetadataTable{}, err
	}
	return readSQLiteStorageRows(ctx, tx, "main", "tblVPics", "", nil, "")
}

// Positive signed32 allocation is a desktop safety adaptation, not Access's
// random GenUniqueID sequence. DISTINCT ensures equivalent numeric aliases
// reserve one identity; no scoped parent filter can hide a library collision.
func allocatePictureCreationID(ctx context.Context, tx *sql.Tx, limit int64, source *ownedPictureLibrary) (int64, error) {
	if limit < 1 || limit > math.MaxInt32 {
		return 0, errors.New("invalid picture allocation bound")
	}
	var deletionTables int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE IN (?,?)`,
		pictureDeletionHistoryTable, pictureDeletionIdentityTable).Scan(&deletionTables); err != nil {
		return 0, err
	}
	deleted := ""
	if deletionTables != 0 {
		if source == nil {
			return 0, errors.New("picture allocation requires the owned deletion history source")
		}
		if _, _, err := readPictureDeletionHistory(ctx, tx, source); err != nil {
			return 0, err
		}
		deleted = ` UNION SELECT ID FROM "__VPRO_PictureDeletionIdentity"`
	}
	rows, err := tx.QueryContext(ctx, `SELECT ID FROM (
		SELECT CAST(CAST(ID AS NUMERIC) AS INTEGER) AS ID FROM tblVPics WHERE ID IS NOT NULL
		UNION SELECT ID FROM "__VPRO_PictureCreationIdentity"`+deleted+`)
		WHERE ID>0 AND ID<=? ORDER BY ID`, limit)
	if err != nil {
		return 0, err
	}
	id, allocationErr := availableChildID(rows, limit)
	return id, errors.Join(allocationErr, rows.Close())
}

func pictureCreationTechnicalState(ctx context.Context, tx *sql.Tx) (bool, error) {
	var history, ledger int
	for table, target := range map[string]*int{pictureCreationHistoryTable: &history, pictureCreationIdentityTable: &ledger} {
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE=?`, table).Scan(target); err != nil {
			return false, err
		}
	}
	if history == 0 && ledger == 0 {
		return false, nil
	}
	if history != 1 || ledger != 1 {
		return false, errors.New("picture creation history/identity pair is missing or ambiguous")
	}
	if err := verifyTechnicalProvenance(ctx, tx, pictureCreationHistoryTable, pictureCreationHistorySQL, "picture creation"); err != nil {
		return false, err
	}
	return true, verifyTechnicalProvenance(ctx, tx, pictureCreationIdentityTable, pictureCreationIdentitySQL, "picture creation identities")
}

func validatePictureCreationHistory(event pictureCreationHistory, id int64, created string, source *ownedPictureLibrary) error {
	r, request := event.Result, event.Request
	dir, name, err := preparePictureCreation(request)
	if err != nil {
		return err
	}
	rowID, rowErr := strconv.ParseInt(r.RowID, 10, 64)
	_, whenErr := time.Parse(time.RFC3339Nano, r.CreatedWhen)
	if id <= 0 || r.HistoryID != strconv.FormatInt(id, 10) || created != r.CreatedWhen || whenErr != nil ||
		r.RequestID != request.RequestID || r.ContextID != request.ContextID || r.Project != request.Project ||
		r.PlotNumber != request.PlotNumber || r.SelectedPath != request.SelectedPath ||
		r.ID < 1 || r.ID > math.MaxInt32 || rowErr != nil || strconv.FormatInt(rowID, 10) != r.RowID ||
		r.Committed.RowID != r.RowID || !reflect.DeepEqual(r.Committed.Cells, pictureCreationCells(r.ID, dir, name, request.PlotNumber)) ||
		!r.DidCommit || r.Replayed || r.Source != source.path || r.RequestedSource != source.requestedPath ||
		len(r.Columns) != 5 || len(r.OwnedFiles) == 0 || r.Actor == "" || strings.ContainsRune(r.Actor, 0) ||
		r.AuditStrength < 0 || r.AuditStrength > 3 {
		return errors.New("picture creation history contains inconsistent immutable provenance")
	}
	if err := validateChildPhysicalText("Picture creation history actor", r.Actor, 255); err != nil {
		return err
	}
	for i, name := range []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"} {
		if r.Columns[i].Name != name || !utf8.ValidString(r.Columns[i].DeclaredType) || strings.ContainsRune(r.Columns[i].DeclaredType, 0) {
			return errors.New("picture creation history has malformed columns")
		}
	}
	for key, value := range r.OwnedFiles {
		if key == "" || value == "" || !utf8.ValidString(key) || !utf8.ValidString(value) ||
			strings.ContainsRune(key, 0) || strings.ContainsRune(value, 0) {
			return errors.New("picture creation history has malformed owned files")
		}
	}
	return nil
}

func replayPictureCreation(ctx context.Context, tx *sql.Tx, source *ownedPictureLibrary, request pictureCreationRequest,
	authority pictureCreationResult, snapshot ProjectMetadataTable) (*pictureCreationResult, error) {
	exists, err := pictureCreationTechnicalState(ctx, tx)
	if err != nil || !exists {
		return nil, err
	}
	ledger := map[int64]string{}
	rows, err := tx.QueryContext(ctx, `SELECT ID,typeof(RequestID),CAST(RequestID AS BLOB) FROM "__VPRO_PictureCreationIdentity" ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	identities := map[string]bool{}
	for rows.Next() {
		var id int64
		var storage string
		var raw []byte
		if err := rows.Scan(&id, &storage, &raw); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		uuid := string(raw)
		if id < 1 || id > math.MaxInt32 || storage != "text" || !siviDeletionUUID.MatchString(uuid) || identities[uuid] {
			return nil, errors.Join(errors.New("picture creation identity ledger is corrupt"), rows.Close())
		}
		ledger[id], identities[uuid] = uuid, true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT ID,typeof(Created),CAST(Created AS BLOB),typeof(Proposal),CAST(Proposal AS BLOB),typeof(Restored)
		FROM "__VPRO_PictureCreationHistory" ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	var found *pictureCreationResult
	seen := map[int64]bool{}
	for rows.Next() {
		var id int64
		var createdType, proposalType, restoredType string
		var created, proposal []byte
		if err := rows.Scan(&id, &createdType, &created, &proposalType, &proposal, &restoredType); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		var event pictureCreationHistory
		err := json.Unmarshal(proposal, &event)
		if err == nil {
			err = validatePictureCreationHistory(event, id, string(created), source)
		}
		if err != nil || createdType != "text" || proposalType != "text" || restoredType != "null" ||
			ledger[event.Result.ID] != event.Request.RequestID || seen[event.Result.ID] {
			return nil, errors.Join(errors.New("picture creation global history is corrupt; no receipt inferred"), err, rows.Close())
		}
		seen[event.Result.ID] = true
		if event.Request.RequestID != request.RequestID {
			continue
		}
		r := event.Result
		if found != nil || event.Request != request || r.ContextID != authority.ContextID || r.Project != authority.Project ||
			r.PlotNumber != authority.PlotNumber || r.Source != authority.Source || r.RequestedSource != authority.RequestedSource ||
			r.Actor != authority.Actor || r.AuditStrength != authority.AuditStrength || !reflect.DeepEqual(r.OwnedFiles, authority.OwnedFiles) ||
			!reflect.DeepEqual(r.Columns, snapshot.Columns) {
			return nil, errors.Join(errors.New("picture creation request identity collides with incompatible provenance"), rows.Close())
		}
		r.Replayed = true
		found = &r
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if len(seen) != len(ledger) {
		return nil, errors.New("picture creation ledger has a reservation without durable history")
	}
	if found == nil {
		return nil, nil
	}
	var matches int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tblVPics WHERE ID=? OR
		(typeof(ID) IN ('text','real','blob') AND CAST(ID AS NUMERIC)=?)`, found.ID, found.ID).Scan(&matches); err != nil {
		return nil, err
	}
	var physical bool
	for _, row := range snapshot.Rows {
		if reflect.DeepEqual(row, found.Committed) {
			physical = true
		}
	}
	if matches != 1 || !physical {
		return nil, errors.New("picture creation receipt no longer matches the exact physical row; no write replayed")
	}
	return found, nil
}

func writePictureCreation(ctx context.Context, source *ownedPictureLibrary, request pictureCreationRequest,
	authority pictureCreationResult, validate func() error) (result pictureCreationResult, resultErr error) {
	dir, name, err := preparePictureCreation(request)
	if err != nil {
		return result, err
	}
	if err := source.checkFile(); err != nil {
		return result, err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw")+"&_busy_timeout=5000&_txlock=immediate&_foreign_keys=on")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	if err := validate(); err != nil {
		return result, err
	}
	// Caller-owned cancellation prevents asynchronous rollback during commit.
	tx, err := db.BeginTx(context.WithoutCancel(ctx), nil)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	before, err := pictureCreationSnapshot(ctx, tx, request.PlotNumber)
	if err != nil {
		return result, err
	}
	replayed, err := replayPictureCreation(ctx, tx, source, request, authority, before)
	if err != nil {
		return result, err
	}
	if replayed != nil {
		if err := errors.Join(validate(), source.checkFile(), ctx.Err()); err != nil {
			return result, err
		}
		return *replayed, nil
	}
	for _, definition := range []string{pictureCreationHistorySQL, pictureCreationIdentitySQL} {
		if _, err := tx.ExecContext(ctx, strings.Replace(definition, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)); err != nil {
			return result, err
		}
	}
	if _, err := pictureCreationTechnicalState(ctx, tx); err != nil {
		return result, err
	}
	id, err := allocatePictureCreationID(ctx, tx, math.MaxInt32, source)
	if err != nil {
		return result, err
	}
	var aliases int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tblVPics WHERE ID=? OR CAST(ID AS NUMERIC)=?`, id, id).Scan(&aliases); err != nil {
		return result, err
	}
	if aliases != 0 {
		return result, errors.New("picture allocator cannot prove a collision-free source identity")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_PictureCreationIdentity"(ID,RequestID) VALUES(?,?)`, id, request.RequestID); err != nil {
		return result, err
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO tblVPics(ID,PicDir,PicName,PlotNumber,PicComment) VALUES(?,?,?,?,NULL)`,
		id, dir, name, request.PlotNumber)
	if err != nil {
		return result, err
	}
	count, err := inserted.RowsAffected()
	if err != nil || count != 1 {
		return result, errors.Join(err, errors.New("picture creation expected exactly one appended source row"))
	}
	rowID, err := inserted.LastInsertId()
	if err != nil {
		return result, err
	}
	planned := ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10), Cells: pictureCreationCells(id, dir, name, request.PlotNumber)}
	expected := ProjectMetadataTable{Columns: before.Columns, Rows: append(append([]ProjectMetadataRow{}, before.Rows...), planned)}
	sort.Slice(expected.Rows, func(i, j int) bool {
		a, _ := strconv.ParseInt(expected.Rows[i].RowID, 10, 64)
		b, _ := strconv.ParseInt(expected.Rows[j].RowID, 10, 64)
		return a < b
	})
	after, err := pictureCreationSnapshot(ctx, tx, request.PlotNumber)
	if err != nil || !reflect.DeepEqual(after, expected) {
		return result, errors.Join(err, errors.New("picture creation storage differs from the complete library append plan"))
	}
	receipt := authority
	receipt.ID, receipt.RowID, receipt.Columns, receipt.Committed = id, planned.RowID, before.Columns, planned
	receipt.CreatedWhen, receipt.DidCommit = time.Now().UTC().Format(time.RFC3339Nano), true
	var historyID int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(ID),0) FROM "__VPRO_PictureCreationHistory"`).Scan(&historyID); err != nil {
		return result, err
	}
	if historyID == math.MaxInt64 {
		return result, errors.New("picture creation history identity space exhausted")
	}
	historyID++
	receipt.HistoryID = strconv.FormatInt(historyID, 10)
	proposal, err := json.Marshal(pictureCreationHistory{request, receipt})
	if err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_PictureCreationHistory"(ID,Created,Proposal) VALUES(?,?,?)`,
		historyID, receipt.CreatedWhen, string(proposal)); err != nil {
		return result, err
	}
	verified, err := replayPictureCreation(ctx, tx, source, request, authority, after)
	if err != nil || verified == nil {
		return result, errors.Join(err, errors.New("picture creation durable provenance did not verify"))
	}
	if err := errors.Join(validate(), source.checkFile(), ctx.Err()); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return receipt, nil
}

func readPictureCreationReceipt(ctx context.Context, source *ownedPictureLibrary, request pictureCreationRequest,
	authority pictureCreationResult) (result *pictureCreationResult, resultErr error) {
	if err := source.checkFile(); err != nil {
		return nil, err
	}
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
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
			result = nil
		}
	}()
	snapshot, err := pictureCreationSnapshot(ctx, tx, request.PlotNumber)
	if err != nil {
		return nil, err
	}
	result, err = replayPictureCreation(ctx, tx, source, request, authority, snapshot)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(source.checkFile(), ctx.Err()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("picture creation receipt snapshot: %w", err)
	}
	return result, nil
}
