package projectops

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// ValidateProjectName enforces canonical VPro project naming rules:
// ^[A-Za-z][A-Za-z0-9_]{0,30}$ and reserves None, Sample, and any name containing "master".
func ValidateProjectName(raw string) (string, error) {
	if !utf8.ValidString(raw) || strings.ContainsRune(raw, 0) {
		return "", errors.New("project name must be valid UTF-8 without NUL bytes")
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw {
		return "", errors.New("project name must not be empty or contain leading/trailing whitespace")
	}
	if !vprocore.ProjectNamePattern.MatchString(trimmed) {
		return "", errors.New("project name must start with a letter and contain only ASCII letters, digits, or underscores (max 31 chars)")
	}
	lower := strings.ToLower(trimmed)
	if lower == "none" || lower == "sample" || strings.Contains(lower, "master") {
		return "", fmt.Errorf("project name %q is reserved (None, Sample, and master names are not permitted)", trimmed)
	}
	return trimmed, nil
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func validateUTF16Max(label, val string, maxUnits int) error {
	if !utf8.ValidString(val) || strings.ContainsRune(val, 0) {
		return fmt.Errorf("%s contains invalid UTF-8 or NUL bytes", label)
	}
	if maxUnits > 0 && utf16Len(val) > maxUnits {
		return fmt.Errorf("%s exceeds %d UTF-16 code units", label, maxUnits)
	}
	return nil
}

// NarrowSingleCover narrows a float64 cover input to IEEE-754 float32 BEFORE evaluating < 100.
func NarrowSingleCover(value float64) (float32, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || math.Abs(value) > math.MaxFloat32 {
		return 0, false
	}
	narrowed := float32(value)
	if narrowed < 0 || narrowed >= 100 {
		return narrowed, false
	}
	return narrowed, true
}

func resolveNewProjectDestination(projectsRoot, projectName, requestedDest string, label string) (string, os.FileInfo, error) {
	dest := strings.TrimSpace(requestedDest)
	if dest == "" {
		dest = filepath.Join(projectsRoot, projectName+".db")
	} else if !strings.EqualFold(filepath.Ext(dest), ".db") {
		dest += ".db"
	}
	return vprocore.FreshSQLiteDestination(dest, label)
}

type contextSnapshotInfo struct {
	Root        string
	Project     string
	ProjectPath string
	SU          string
	SUPath      string
	VListsPath  string
	VPro64Path  string
	CurrentUser string
	AuditLevel  int
}

func inspectContextSnapshot(ctx context.Context, s *vprocore.ContextService, contextID string) (contextSnapshotInfo, error) {
	if s == nil {
		return contextSnapshotInfo{}, errors.New("project context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (contextSnapshotInfo, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return contextSnapshotInfo{}, err
		}
		defer owner.Mu.Unlock()
		if owner.Conn == nil {
			return contextSnapshotInfo{}, errors.New("project context is closed")
		}
		user := strings.TrimSpace(plots.CurrentUser)
		if user == "" {
			user = "VProUser"
		}
		return contextSnapshotInfo{
			Root:        plots.Projects.Root,
			Project:     owner.Selection.Project,
			ProjectPath: owner.Selection.ProjectPath,
			SU:          owner.Selection.SU,
			SUPath:      owner.Selection.SUPath,
			VListsPath:  owner.Attachments["VLists"],
			VPro64Path:  owner.Attachments["VPro64"],
			CurrentUser: user,
			AuditLevel:  plots.AuditStrength,
		}, nil
	})
}

func computeFileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// readVListsVersionStamps reads USysAllSpecs and USysTableOfLists version descriptions
// from VLists.db _table_metadata along with the short SHA-256 digest of VLists.db.
func readVListsVersionStamps(ctx context.Context, vlistsPath string) (allSpecsStamp, tableOfListsStamp, shortDigest string, err error) {
	if vlistsPath == "" {
		return "Unknown", "Unknown", "", nil
	}
	fullSHA, shaErr := computeFileSHA256(vlistsPath)
	if shaErr != nil {
		return "", "", "", fmt.Errorf("hash VLists.db: %w", shaErr)
	}
	if len(fullSHA) >= 12 {
		shortDigest = fullSHA[:12]
	} else {
		shortDigest = fullSHA
	}
	db, openErr := sqlutil.OpenReadOnlyContext(ctx, vlistsPath)
	if openErr != nil {
		return "", "", "", fmt.Errorf("open VLists.db: %w", openErr)
	}
	defer db.Close()

	readDesc := func(tableName string) string {
		var desc sql.NullString
		qErr := db.QueryRowContext(ctx, `SELECT description FROM "_table_metadata" WHERE table_name = ?`, tableName).Scan(&desc)
		if qErr != nil || !desc.Valid || strings.TrimSpace(desc.String) == "" {
			return "Unknown"
		}
		return strings.TrimSpace(desc.String)
	}
	allSpecsBase := readDesc("USysAllSpecs")
	listsBase := readDesc("USysTableOfLists")
	if shortDigest != "" {
		allSpecsStamp = fmt.Sprintf("%s [sha256:%s]", allSpecsBase, shortDigest)
		tableOfListsStamp = fmt.Sprintf("%s [sha256:%s]", listsBase, shortDigest)
	} else {
		allSpecsStamp = allSpecsBase
		tableOfListsStamp = listsBase
	}
	if utf16Len(allSpecsStamp) > 255 {
		allSpecsStamp = allSpecsBase
	}
	if utf16Len(tableOfListsStamp) > 255 {
		tableOfListsStamp = listsBase
	}
	return allSpecsStamp, tableOfListsStamp, shortDigest, nil
}

type rawSQLiteCell struct {
	storage string
	intVal  int64
	realVal float64
	bytes   []byte
}

func (c rawSQLiteCell) sqlParam() any {
	switch c.storage {
	case "null":
		return nil
	case "integer":
		return c.intVal
	case "real":
		return c.realVal
	case "text":
		return string(c.bytes)
	case "blob":
		return append([]byte(nil), c.bytes...)
	default:
		return nil
	}
}

type sqliteQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func readExactTableRows(ctx context.Context, db sqliteQueryer, qualifiedTable string, whereClause string, args ...any) ([]string, [][]rawSQLiteCell, error) {
	colRows, err := db.QueryContext(ctx, "SELECT * FROM "+qualifiedTable+" LIMIT 0")
	if err != nil {
		return nil, nil, err
	}
	cols, colErr := colRows.Columns()
	_ = colRows.Close()
	if colErr != nil {
		return nil, nil, colErr
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("table %s has no columns", qualifiedTable)
	}
	selectExprs := make([]string, 0, len(cols)*2)
	for _, col := range cols {
		q := vprocore.QuoteHeaderIdentifier(col)
		selectExprs = append(selectExprs,
			"typeof("+q+")",
			"CASE typeof("+q+") WHEN 'integer' THEN CAST("+q+" AS INTEGER) WHEN 'real' THEN CAST("+q+" AS REAL) ELSE CAST("+q+" AS BLOB) END",
		)
	}
	query := "SELECT " + strings.Join(selectExprs, ", ") + " FROM " + qualifiedTable
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out [][]rawSQLiteCell
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		storages := make([]string, len(cols))
		vals := make([]any, len(cols))
		dest := make([]any, 0, len(cols)*2)
		for i := range cols {
			dest = append(dest, &storages[i], &vals[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		rowCells := make([]rawSQLiteCell, len(cols))
		for i := range cols {
			cell := rawSQLiteCell{storage: storages[i]}
			switch storages[i] {
			case "null":
			case "integer":
				if v, ok := vals[i].(int64); ok {
					cell.intVal = v
				}
			case "real":
				if v, ok := vals[i].(float64); ok {
					cell.realVal = v
				}
			case "text", "blob":
				switch b := vals[i].(type) {
				case []byte:
					if storages[i] == "text" && !utf8.Valid(b) {
						return nil, nil, fmt.Errorf("table %s column %s contains malformed UTF-8", qualifiedTable, cols[i])
					}
					cell.bytes = append([]byte(nil), b...)
				case string:
					if storages[i] == "text" && !utf8.ValidString(b) {
						return nil, nil, fmt.Errorf("table %s column %s contains malformed UTF-8", qualifiedTable, cols[i])
					}
					cell.bytes = []byte(b)
				}
			default:
				return nil, nil, fmt.Errorf("unsupported SQLite storage class %q in %s.%s", storages[i], qualifiedTable, cols[i])
			}
			rowCells[i] = cell
		}
		out = append(out, rowCells)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, nil
}

func insertExactTableRows(ctx context.Context, tx *sql.Tx, targetTable string, cols []string, rows [][]rawSQLiteCell) error {
	if len(rows) == 0 {
		return nil
	}
	quotedCols := make([]string, len(cols))
	marks := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = vprocore.QuoteHeaderIdentifier(c)
		marks[i] = "?"
	}
	stmtSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		vprocore.QuoteHeaderIdentifier(targetTable),
		strings.Join(quotedCols, ", "),
		strings.Join(marks, ", "),
	)
	stmt, err := tx.PrepareContext(ctx, stmtSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		args := make([]any, len(cols))
		for i := range cols {
			args[i] = r[i].sqlParam()
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return err
		}
	}
	return nil
}

func ensureChildIdentityTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL,
		"ID" INTEGER NOT NULL,
		PRIMARY KEY ("ChildTable", "ID")
	)`)
	return err
}

func reserveChildID(ctx context.Context, tx *sql.Tx, childTable string, id int64) error {
	quoted := vprocore.QuoteHeaderIdentifier(childTable)
	_, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES (?, ?)
		ON CONFLICT ("ChildTable", "ID") DO NOTHING`, quoted, id)
	return err
}

// allocateSafeChildID seeds __VPRO_ChildIdentity with all existing positive IDs in childTable
// and Audit before calling vprocore.AllocateChildID so deleted IDs are never reused.
func allocateSafeChildID(ctx context.Context, tx *sql.Tx, project, suffix string) (int64, error) {
	if err := ensureChildIdentityTable(ctx, tx); err != nil {
		return 0, err
	}
	childTable := project + "_" + suffix
	quotedChild := vprocore.QuoteHeaderIdentifier(childTable)
	quotedAudit := vprocore.QuoteHeaderIdentifier(project + "_Audit")

	rows, err := tx.QueryContext(ctx, `SELECT "ID" FROM `+quotedChild+` WHERE typeof("ID")='integer' AND "ID" > 0
		UNION
		SELECT "ID" FROM `+quotedAudit+` WHERE "Table" COLLATE NOCASE IN (?, ?) AND typeof("ID")='integer' AND "ID" > 0
		UNION
		SELECT "ID" FROM "__VPRO_ChildIdentity" WHERE "ChildTable" IN (?, ?) AND "ID" > 0`,
		"_"+suffix, childTable, quotedChild, childTable)
	if err != nil {
		return 0, err
	}
	var existing []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		existing = append(existing, id)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return 0, err
	}
	for _, id := range existing {
		if err := reserveChildID(ctx, tx, childTable, id); err != nil {
			return 0, err
		}
	}
	nextID, err := vprocore.AllocateChildID(tx, quotedChild)
	if err != nil {
		return 0, err
	}
	if err := reserveChildID(ctx, tx, childTable, nextID); err != nil {
		return 0, err
	}
	return nextID, nil
}
