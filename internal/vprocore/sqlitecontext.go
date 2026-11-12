package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type SqliteContext struct {
	Mu             sync.Mutex `json:"-" yaml:"-"`
	db             *sql.DB
	Conn           *sql.Conn                   `json:"-" yaml:"-"`
	ProjectReadDB  *sql.DB                     `json:"-" yaml:"-"`
	ProjectDB      *sql.DB                     `json:"-" yaml:"-"`
	Selection      DesktopSelection            `json:"-" yaml:"-"`
	Attachments    map[string]string           `json:"-" yaml:"-"`
	AttachmentInfo map[string]os.FileInfo      `json:"-" yaml:"-"`
	Descriptions   map[string][]map[string]any `json:"-" yaml:"-"`
	Profile        *PlotProfileSource          `json:"-" yaml:"-"`
	profileRole    string
	ProfileWrite   bool `json:"-" yaml:"-"`
}

// database/sql discards a pinned SQLite connection when a transaction's context
// is cancelled. Keep cleanup caller-owned; every query still uses the request
// context, and callers must check it before committing the read snapshot.
func (c *SqliteContext) BeginReadSnapshot(ctx context.Context) (*sql.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tx, err := c.Conn.BeginTx(context.WithoutCancel(ctx), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, tx.Rollback())
	}
	return tx, nil
}

func (c *SqliteContext) projectReadDatabase(ctx context.Context) (*sql.DB, error) {
	if err := sqlutil.AcquireMutexLease(ctx, &c.Mu); err != nil {
		return nil, err
	}
	defer c.Mu.Unlock()
	if c.Conn == nil {
		return nil, errors.New("project context is closed")
	}
	if c.ProjectDB != nil {
		return c.ProjectDB, nil
	}
	if c.ProjectReadDB != nil {
		return c.ProjectReadDB, nil
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(c.Selection.ProjectPath, "ro")+"&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	c.ProjectReadDB = db
	return db, nil
}

func (c *SqliteContext) ProjectDatabase(ctx context.Context) (*sql.DB, error) {
	if err := sqlutil.AcquireMutexLease(ctx, &c.Mu); err != nil {
		return nil, err
	}
	defer c.Mu.Unlock()
	if c.Conn == nil {
		return nil, errors.New("project context is closed")
	}
	if c.ProjectDB != nil {
		return c.ProjectDB, nil
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(c.Selection.ProjectPath, "rw")+"&_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	var journalMode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL;").Scan(&journalMode); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	if !strings.EqualFold(journalMode, "wal") {
		return nil, errors.Join(fmt.Errorf("expected SQLite WAL journal mode, got %q", journalMode), db.Close())
	}
	c.ProjectDB = db
	return db, nil
}

func ExistingDatabasePath(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("database path must be an explicit absolute path: %q", path)
	}
	absolute, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve database %q: %w", path, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("database path is not a regular file: %s", path)
	}
	return absolute, nil
}

func NewSQLiteContext(ctx context.Context, selection DesktopSelection, support map[string]string, profile ...*PlotProfileSource) (_ *SqliteContext, resultErr error) {
	if len(profile) > 1 {
		return nil, errors.New("one explicit plot-profile source may be selected")
	}
	for _, name := range []string{selection.Project, selection.SU, selection.Hierarchy} {
		if !ProjectNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid context family name %q", name)
		}
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	candidate := &SqliteContext{db: db, Conn: conn, Selection: selection,
		Attachments: map[string]string{}, AttachmentInfo: map[string]os.FileInfo{}, Descriptions: map[string][]map[string]any{}}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, candidate.Close())
		}
	}()
	for _, seed := range DatabaseFamilySeeds {
		path, present := support[seed.Name]
		if !present {
			return nil, fmt.Errorf("required support role %s is not configured", seed.Name)
		}
		if _, err := candidate.Attach(ctx, seed.Name, path); err != nil {
			return nil, err
		}
		for _, table := range seed.tables {
			if err := candidate.requireTable(ctx, seed.Name, table); err != nil {
				return nil, err
			}
		}
	}
	if len(support) != len(DatabaseFamilySeeds) {
		return nil, errors.New("unknown support database role")
	}
	projectPath, err := candidate.Attach(ctx, "project", selection.ProjectPath)
	if err != nil {
		return nil, err
	}
	candidate.Selection.ProjectPath = projectPath
	if err := candidate.validateProject(ctx); err != nil {
		return nil, err
	}
	if selection.SU != "None" {
		path, err := candidate.Attach(ctx, "su", selection.SUPath)
		if err != nil {
			return nil, err
		}
		candidate.Selection.SUPath = path
		if err := candidate.requireTable(ctx, "su", selection.SU+"_SU"); err != nil {
			return nil, err
		}
		if err := candidate.RequireColumns(ctx, "su", selection.SU+"_SU", []string{"PlotNumber", "SiteUnit"}); err != nil {
			return nil, err
		}
		database, err := sqlutil.OpenReadOnly(path)
		if err != nil {
			return nil, err
		}
		err = errors.Join(workingUnitSUAuthorized(database, selection.SU), database.Close())
		if err != nil {
			return nil, err
		}
	} else if selection.SUPath != "" {
		return nil, errors.New("SUPath must be empty when no SU is selected")
	}
	if selection.Hierarchy != "None" {
		path, err := candidate.Attach(ctx, "hierarchy", selection.HierarchyPath)
		if err != nil {
			return nil, err
		}
		candidate.Selection.HierarchyPath = path
		if err := candidate.requireTable(ctx, "hierarchy", selection.Hierarchy+"_Hierarchy"); err != nil {
			return nil, err
		}
		if err := candidate.RequireColumns(ctx, "hierarchy", selection.Hierarchy+"_Hierarchy", []string{"ID", "Name", "Parent", "Level"}); err != nil {
			return nil, err
		}
		database, err := sqlutil.OpenReadOnly(path)
		if err != nil {
			return nil, err
		}
		valid, checkErr := hierarchyCompatible(database, selection.Hierarchy+"_Hierarchy")
		checkErr = errors.Join(checkErr, database.Close())
		if checkErr != nil {
			return nil, checkErr
		}
		if !valid {
			return nil, errors.New("hierarchy requires integer ID/Parent/Level and text Name")
		}
	} else if selection.HierarchyPath != "" {
		return nil, errors.New("HierarchyPath must be empty when no hierarchy is selected")
	}
	if err := candidate.createViews(ctx); err != nil {
		return nil, err
	}
	if len(profile) == 1 {
		if err := candidate.attachProfile(ctx, profile[0]); err != nil {
			return nil, err
		}
	}
	return candidate, nil
}

func (c *SqliteContext) Attach(ctx context.Context, alias, path string) (string, error) {
	path, err := ExistingDatabasePath(path)
	if err != nil {
		return "", fmt.Errorf("attach %s: %w", alias, err)
	}
	if _, exists := c.Attachments[alias]; exists {
		return "", fmt.Errorf("attachment alias %q is already owned", alias)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("identify attachment %s: %w", alias, err)
	}
	info, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return "", fmt.Errorf("capture attachment %s identity: %w", alias, err)
	}
	if _, err := c.Conn.ExecContext(ctx, "ATTACH DATABASE ? AS "+QuoteHeaderIdentifier(alias), sqlutil.SqliteFileURI(path, "ro")); err != nil {
		return "", fmt.Errorf("attach %s: %w", alias, err)
	}
	current, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("verify attached %s identity: %w", alias, err)
	}
	if !os.SameFile(info, current) {
		return "", fmt.Errorf("attachment %s identity changed during attach", alias)
	}
	c.Attachments[alias] = path
	c.AttachmentInfo[alias] = info
	rows, err := c.tableDescriptions(ctx, alias)
	if err != nil {
		return "", fmt.Errorf("read %s table descriptions: %w", alias, err)
	}
	c.Descriptions[alias] = rows
	return path, nil
}

func (c *SqliteContext) requireTable(ctx context.Context, alias, table string) error {
	var exists bool
	if err := c.Conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+QuoteHeaderIdentifier(alias)+".sqlite_master WHERE type='table' AND name=?)", table).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s.%s is missing a required physical table", alias, table)
	}
	return nil
}

func (c *SqliteContext) columns(ctx context.Context, alias, table string) ([]string, error) {
	rows, err := c.Conn.QueryContext(ctx, "PRAGMA "+QuoteHeaderIdentifier(alias)+".table_info("+QuoteHeaderIdentifier(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&index, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (c *SqliteContext) RequireColumns(ctx context.Context, alias, table string, required []string) error {
	names, err := c.columns(ctx, alias, table)
	if err != nil {
		return err
	}
	for _, name := range required {
		if !slices.Contains(names, name) {
			return fmt.Errorf("%s.%s is missing required column %s", alias, table, name)
		}
	}
	return nil
}

func (c *SqliteContext) tableDescriptions(ctx context.Context, alias string) ([]map[string]any, error) {
	var exists bool
	if err := c.Conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+QuoteHeaderIdentifier(alias)+".sqlite_master WHERE type='table' AND name='_table_metadata')").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	rows, err := c.Conn.QueryContext(ctx, "SELECT * FROM "+QuoteHeaderIdentifier(alias)+"._table_metadata")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(names, "table_name") || !slices.Contains(names, "description") {
		return nil, errors.New("_table_metadata must contain table_name and description")
	}
	var result []map[string]any
	for rows.Next() {
		values, pointers := make([]any, len(names)), make([]any, len(names))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, name := range names {
			row[name] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (c *SqliteContext) validateProject(ctx context.Context) error {
	for _, suffix := range CoreTables {
		if err := c.requireTable(ctx, "project", c.Selection.Project+"_"+suffix); err != nil {
			return err
		}
	}
	var versions []any
	for _, row := range c.Descriptions["project"] {
		if row["table_name"] == c.Selection.Project+"_Env" {
			versions = append(versions, row["description"])
		}
	}
	if len(versions) != 1 || versions[0] != "VP08" {
		return fmt.Errorf("project %s requires one unambiguous VP08 Env description; found %v (VP05-07 conversion is unavailable)", c.Selection.Project, versions)
	}
	required := map[string][]string{
		"Admin": {"Plot"},
		"Audit": {"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID"},
		"Humus": {"ID", "PlotNumber"}, "Mineral": {"ID", "PlotNumber"},
		"Other": {"ID", "PlotNumber"}, "Metadata": {"ID", "ProjectID"},
	}
	for _, field := range HeaderFields {
		if field.Table != "" && field.Column != "" {
			required[field.Table] = append(required[field.Table], field.Column)
		}
	}
	for table, fields := range required {
		if err := c.RequireColumns(ctx, "project", c.Selection.Project+"_"+table, fields); err != nil {
			return err
		}
	}
	return nil
}

func (c *SqliteContext) createView(ctx context.Context, name, query string) error {
	if _, err := c.Conn.ExecContext(ctx, "CREATE TEMP VIEW "+QuoteHeaderIdentifier(name)+" AS "+query); err != nil {
		return fmt.Errorf("create temporary %s: %w", name, err)
	}
	rows, err := c.Conn.QueryContext(ctx, "SELECT * FROM "+QuoteHeaderIdentifier(name)+" LIMIT 0")
	if err != nil {
		return fmt.Errorf("validate temporary %s: %w", name, err)
	}
	return rows.Close()
}

func (c *SqliteContext) createViews(ctx context.Context) error {
	relation := func(alias, table string) string {
		return QuoteHeaderIdentifier(alias) + "." + QuoteHeaderIdentifier(table)
	}
	project := func(suffix string) string { return relation("project", c.Selection.Project+"_"+suffix) }
	env := "SELECT DISTINCT env.*, admin.* FROM " + project("Env") + " AS env INNER JOIN " + project("Admin") + " AS admin ON env.PlotNumber=admin.Plot"
	if err := c.createView(ctx, "Filtered_Env", env); err != nil {
		return err
	}
	if c.Selection.SU != "None" {
		env += " WHERE EXISTS(SELECT 1 FROM " + relation("su", c.Selection.SU+"_SU") + " AS su WHERE su.PlotNumber=env.PlotNumber)"
	}
	if err := c.createView(ctx, "USysEnv", env); err != nil {
		return err
	}
	for _, entry := range []struct{ view, suffix string }{
		{"USysVeg", "Veg"}, {"USysHumus", "Humus"}, {"USysMineral", "Mineral"},
		{"USysAuditTrail", "Audit"}, {"USysOther", "Other"}, {"USysMetadata", "Metadata"},
	} {
		if err := c.createView(ctx, entry.view, "SELECT DISTINCT * FROM "+project(entry.suffix)); err != nil {
			return err
		}
	}
	for _, entry := range []struct {
		view            string
		fields, filters []string
	}{
		{"USysVegA", []string{"Cover1", "Cover2", "Cover3", "TotalA", "HeightA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "HeightB", "Collected"},
			[]string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}},
		{"USysVegB", []string{"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "Collected"},
			[]string{"Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}},
		{"USysVegC", []string{"Cover6", "Height6", "Collected"}, []string{"Cover6"}},
		{"USysVegD", []string{"Cover7", "Cover8", "Cover9", "Collected"}, []string{"Cover7", "Cover8", "Cover9"}},
	} {
		fields := append([]string{"ID", "PlotNumber", "Species"}, entry.fields...)
		if err := c.RequireColumns(ctx, "project", c.Selection.Project+"_Veg", append(fields, entry.filters...)); err != nil {
			return err
		}
		for i, field := range fields {
			fields[i] = QuoteHeaderIdentifier(field)
		}
		filters := make([]string, len(entry.filters))
		for i, field := range entry.filters {
			filters[i] = QuoteHeaderIdentifier(field) + " IS NOT NULL"
		}
		if err := c.createView(ctx, entry.view, "SELECT DISTINCT "+strings.Join(fields, ",")+" FROM "+project("Veg")+" WHERE "+strings.Join(filters, " OR ")); err != nil {
			return err
		}
	}
	for _, seed := range DatabaseFamilySeeds {
		if seed.Name == "VPro64" {
			continue
		}
		for _, table := range seed.tables {
			view := table
			if table == "MasterSiteUnitList" {
				view = "USysMasterSiteUnitList"
			} else if table == "UserSiteUnitList" {
				view = "USysUserSiteUnitList"
			}
			if err := c.createView(ctx, view, "SELECT * FROM "+relation(seed.Name, table)); err != nil {
				return err
			}
		}
	}
	if err := c.RequireColumns(ctx, "VLists", "USysAllSpecs", []string{"EnglishName", "CombinedEnglishName"}); err != nil {
		return err
	}
	for _, name := range []string{"MasterSiteUnitList", "MasterUnitList_Hierarchy"} {
		if err := c.createView(ctx, name, "SELECT * FROM USysMasterSiteUnitList UNION SELECT * FROM USysUserSiteUnitList"); err != nil {
			return err
		}
	}
	if c.Selection.SU != "None" {
		if err := c.createView(ctx, "USysSU", "SELECT * FROM "+relation("su", c.Selection.SU+"_SU")); err != nil {
			return err
		}
	}
	if c.Selection.Hierarchy != "None" {
		if err := c.createView(ctx, "USysHierarchy", "SELECT * FROM "+relation("hierarchy", c.Selection.Hierarchy+"_Hierarchy")); err != nil {
			return err
		}
	}
	return nil
}

func (c *SqliteContext) Close() error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.Conn == nil {
		return nil
	}
	connErr := errors.Join(c.Conn.Close(), c.db.Close())
	var readErr error
	if c.ProjectReadDB != nil {
		readErr = c.ProjectReadDB.Close()
	}
	var projectErr error
	if c.ProjectDB != nil {
		projectErr = c.ProjectDB.Close()
	}
	err := errors.Join(connErr, readErr, projectErr)
	c.Conn, c.db = nil, nil
	c.ProjectReadDB = nil
	c.ProjectDB = nil
	return err
}
