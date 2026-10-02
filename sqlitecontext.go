package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type sqliteContext struct {
	mu             sync.Mutex
	db             *sql.DB
	conn           *sql.Conn
	projectDB      *sql.DB
	selection      desktopSelection
	attachments    map[string]string
	attachmentInfo map[string]os.FileInfo
	descriptions   map[string][]map[string]any
	profile        *PlotProfileSource
	profileRole    string
}

func sqliteFileURI(path, mode string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	address := url.URL{Scheme: "file", Path: clean}
	query := url.Values{"mode": {mode}}
	address.RawQuery = query.Encode()
	return address.String()
}

func (c *sqliteContext) projectDatabase(ctx context.Context) (*sql.DB, error) {
	if err := acquireMutexLease(ctx, &c.mu); err != nil {
		return nil, err
	}
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, errors.New("project context is closed")
	}
	if c.projectDB != nil {
		return c.projectDB, nil
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(c.selection.ProjectPath, "rw")+"&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	c.projectDB = db
	return db, nil
}

func existingDatabasePath(path string) (string, error) {
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

func newSQLiteContext(ctx context.Context, selection desktopSelection, support map[string]string, profile ...*PlotProfileSource) (_ *sqliteContext, resultErr error) {
	if len(profile) > 1 {
		return nil, errors.New("one explicit plot-profile source may be selected")
	}
	for _, name := range []string{selection.Project, selection.SU, selection.Hierarchy} {
		if !projectNamePattern.MatchString(name) {
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
	candidate := &sqliteContext{db: db, conn: conn, selection: selection,
		attachments: map[string]string{}, attachmentInfo: map[string]os.FileInfo{}, descriptions: map[string][]map[string]any{}}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, candidate.Close())
		}
	}()
	for _, seed := range databaseFamilySeeds {
		path, present := support[seed.name]
		if !present {
			return nil, fmt.Errorf("required support role %s is not configured", seed.name)
		}
		if _, err := candidate.attach(ctx, seed.name, path); err != nil {
			return nil, err
		}
		for _, table := range seed.tables {
			if err := candidate.requireTable(ctx, seed.name, table); err != nil {
				return nil, err
			}
		}
	}
	if len(support) != len(databaseFamilySeeds) {
		return nil, errors.New("unknown support database role")
	}
	projectPath, err := candidate.attach(ctx, "project", selection.ProjectPath)
	if err != nil {
		return nil, err
	}
	candidate.selection.ProjectPath = projectPath
	if err := candidate.validateProject(ctx); err != nil {
		return nil, err
	}
	if selection.SU != "None" {
		path, err := candidate.attach(ctx, "su", selection.SUPath)
		if err != nil {
			return nil, err
		}
		candidate.selection.SUPath = path
		if err := candidate.requireTable(ctx, "su", selection.SU+"_SU"); err != nil {
			return nil, err
		}
		if err := candidate.requireColumns(ctx, "su", selection.SU+"_SU", []string{"PlotNumber", "SiteUnit"}); err != nil {
			return nil, err
		}
		database, err := openReadOnly(path)
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
		path, err := candidate.attach(ctx, "hierarchy", selection.HierarchyPath)
		if err != nil {
			return nil, err
		}
		candidate.selection.HierarchyPath = path
		if err := candidate.requireTable(ctx, "hierarchy", selection.Hierarchy+"_Hierarchy"); err != nil {
			return nil, err
		}
		if err := candidate.requireColumns(ctx, "hierarchy", selection.Hierarchy+"_Hierarchy", []string{"ID", "Name", "Parent", "Level"}); err != nil {
			return nil, err
		}
		database, err := openReadOnly(path)
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

func (c *sqliteContext) attach(ctx context.Context, alias, path string) (string, error) {
	path, err := existingDatabasePath(path)
	if err != nil {
		return "", fmt.Errorf("attach %s: %w", alias, err)
	}
	if _, exists := c.attachments[alias]; exists {
		return "", fmt.Errorf("attachment alias %q is already owned", alias)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("identify attachment %s: %w", alias, err)
	}
	if _, err := c.conn.ExecContext(ctx, "ATTACH DATABASE ? AS "+quoteHeaderIdentifier(alias), sqliteFileURI(path, "ro")); err != nil {
		return "", fmt.Errorf("attach %s: %w", alias, err)
	}
	c.attachments[alias] = path
	c.attachmentInfo[alias] = info
	rows, err := c.tableDescriptions(ctx, alias)
	if err != nil {
		return "", fmt.Errorf("read %s table descriptions: %w", alias, err)
	}
	c.descriptions[alias] = rows
	return path, nil
}

func (c *sqliteContext) requireTable(ctx context.Context, alias, table string) error {
	var exists bool
	if err := c.conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+quoteHeaderIdentifier(alias)+".sqlite_master WHERE type='table' AND name=?)", table).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s.%s is missing a required physical table", alias, table)
	}
	return nil
}

func (c *sqliteContext) columns(ctx context.Context, alias, table string) ([]string, error) {
	rows, err := c.conn.QueryContext(ctx, "PRAGMA "+quoteHeaderIdentifier(alias)+".table_info("+quoteHeaderIdentifier(table)+")")
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

func (c *sqliteContext) requireColumns(ctx context.Context, alias, table string, required []string) error {
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

func (c *sqliteContext) tableDescriptions(ctx context.Context, alias string) ([]map[string]any, error) {
	var exists bool
	if err := c.conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+quoteHeaderIdentifier(alias)+".sqlite_master WHERE type='table' AND name='_table_metadata')").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	rows, err := c.conn.QueryContext(ctx, "SELECT * FROM "+quoteHeaderIdentifier(alias)+"._table_metadata")
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

func (c *sqliteContext) validateProject(ctx context.Context) error {
	for _, suffix := range coreTables {
		if err := c.requireTable(ctx, "project", c.selection.Project+"_"+suffix); err != nil {
			return err
		}
	}
	var versions []any
	for _, row := range c.descriptions["project"] {
		if row["table_name"] == c.selection.Project+"_Env" {
			versions = append(versions, row["description"])
		}
	}
	if len(versions) != 1 || versions[0] != "VP08" {
		return fmt.Errorf("project %s requires one unambiguous VP08 Env description; found %v (VP05-07 conversion is unavailable)", c.selection.Project, versions)
	}
	required := map[string][]string{
		"Admin": {"Plot"},
		"Audit": {"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID"},
		"Humus": {"ID", "PlotNumber"}, "Mineral": {"ID", "PlotNumber"},
		"Other": {"ID", "PlotNumber"}, "Metadata": {"ID", "ProjectID"},
	}
	for _, field := range headerFields {
		if field.table != "" && field.column != "" {
			required[field.table] = append(required[field.table], field.column)
		}
	}
	for table, fields := range required {
		if err := c.requireColumns(ctx, "project", c.selection.Project+"_"+table, fields); err != nil {
			return err
		}
	}
	return nil
}

func (c *sqliteContext) createView(ctx context.Context, name, query string) error {
	if _, err := c.conn.ExecContext(ctx, "CREATE TEMP VIEW "+quoteHeaderIdentifier(name)+" AS "+query); err != nil {
		return fmt.Errorf("create temporary %s: %w", name, err)
	}
	rows, err := c.conn.QueryContext(ctx, "SELECT * FROM "+quoteHeaderIdentifier(name)+" LIMIT 0")
	if err != nil {
		return fmt.Errorf("validate temporary %s: %w", name, err)
	}
	return rows.Close()
}

func (c *sqliteContext) createViews(ctx context.Context) error {
	relation := func(alias, table string) string {
		return quoteHeaderIdentifier(alias) + "." + quoteHeaderIdentifier(table)
	}
	project := func(suffix string) string { return relation("project", c.selection.Project+"_"+suffix) }
	env := "SELECT DISTINCT env.*, admin.* FROM " + project("Env") + " AS env INNER JOIN " + project("Admin") + " AS admin ON env.PlotNumber=admin.Plot"
	if err := c.createView(ctx, "Filtered_Env", env); err != nil {
		return err
	}
	if c.selection.SU != "None" {
		env += " WHERE EXISTS(SELECT 1 FROM " + relation("su", c.selection.SU+"_SU") + " AS su WHERE su.PlotNumber=env.PlotNumber)"
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
		if err := c.requireColumns(ctx, "project", c.selection.Project+"_Veg", append(fields, entry.filters...)); err != nil {
			return err
		}
		for i, field := range fields {
			fields[i] = quoteHeaderIdentifier(field)
		}
		filters := make([]string, len(entry.filters))
		for i, field := range entry.filters {
			filters[i] = quoteHeaderIdentifier(field) + " IS NOT NULL"
		}
		if err := c.createView(ctx, entry.view, "SELECT DISTINCT "+strings.Join(fields, ",")+" FROM "+project("Veg")+" WHERE "+strings.Join(filters, " OR ")); err != nil {
			return err
		}
	}
	for _, seed := range databaseFamilySeeds {
		if seed.name == "VPro64" {
			continue
		}
		for _, table := range seed.tables {
			view := table
			if table == "MasterSiteUnitList" {
				view = "USysMasterSiteUnitList"
			} else if table == "UserSiteUnitList" {
				view = "USysUserSiteUnitList"
			}
			if err := c.createView(ctx, view, "SELECT * FROM "+relation(seed.name, table)); err != nil {
				return err
			}
		}
	}
	if err := c.requireColumns(ctx, "VLists", "USysAllSpecs", []string{"EnglishName", "CombinedEnglishName"}); err != nil {
		return err
	}
	for _, name := range []string{"MasterSiteUnitList", "MasterUnitList_Hierarchy"} {
		if err := c.createView(ctx, name, "SELECT * FROM USysMasterSiteUnitList UNION SELECT * FROM USysUserSiteUnitList"); err != nil {
			return err
		}
	}
	if c.selection.SU != "None" {
		if err := c.createView(ctx, "USysSU", "SELECT * FROM "+relation("su", c.selection.SU+"_SU")); err != nil {
			return err
		}
	}
	if c.selection.Hierarchy != "None" {
		if err := c.createView(ctx, "USysHierarchy", "SELECT * FROM "+relation("hierarchy", c.selection.Hierarchy+"_Hierarchy")); err != nil {
			return err
		}
	}
	return nil
}

func (c *sqliteContext) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	var projectErr error
	if c.projectDB != nil {
		projectErr = c.projectDB.Close()
	}
	err := errors.Join(projectErr, c.conn.Close(), c.db.Close())
	c.conn, c.db = nil, nil
	c.projectDB = nil
	return err
}
