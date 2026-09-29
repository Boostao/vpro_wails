package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

type Coordinator struct {
	database *sql.DB
	conn     *sql.Conn
}

func NewCoordinator(ctx context.Context, path, project string) (_ *Coordinator, err error) {
	if !projectNamePattern.MatchString(project) {
		return nil, errors.New("invalid VPRO project name")
	}
	database, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	defer func() {
		if err != nil {
			database.Close()
		}
	}()
	conn, err := database.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()
	var installed bool
	err = conn.QueryRowContext(ctx, "SELECT installed FROM duckdb_extensions() WHERE extension_name = 'sqlite_scanner'").Scan(&installed)
	if err != nil {
		return nil, fmt.Errorf("inspect DuckDB sqlite_scanner extension: %w", err)
	}
	if !installed {
		return nil, errors.New("DuckDB sqlite_scanner is not installed for this DuckDB version; provision it explicitly before offline use")
	}
	if _, err = conn.ExecContext(ctx, "LOAD sqlite_scanner"); err != nil {
		return nil, fmt.Errorf("load cached sqlite_scanner: %w", err)
	}
	alias := "vpro_project_" + strings.ToLower(project)
	quotedPath := "'" + strings.ReplaceAll(path, "'", "''") + "'"
	if _, err = conn.ExecContext(ctx, `ATTACH `+quotedPath+` AS "`+alias+`" (TYPE sqlite, READ_ONLY)`); err != nil {
		return nil, fmt.Errorf("attach SQLite project: %w", err)
	}
	if err = createProjectViews(ctx, conn, alias, project); err != nil {
		return nil, err
	}
	return &Coordinator{database: database, conn: conn}, nil
}

func createProjectViews(ctx context.Context, conn *sql.Conn, alias, project string) error {
	relation := func(suffix string) string {
		return `"` + alias + `"."` + project + `_` + suffix + `"`
	}
	veg := relation("Veg")
	viewSQL := []struct{ name, query string }{
		{"USysEnv", `SELECT DISTINCT env.*, admin.* FROM ` + relation("Env") + ` AS env INNER JOIN ` + relation("Admin") + ` AS admin ON env."PlotNumber" = admin."Plot"`},
		{"USysVeg", `SELECT DISTINCT * FROM ` + veg},
		{"USysHumus", `SELECT DISTINCT * FROM ` + relation("Humus")},
		{"USysMineral", `SELECT DISTINCT * FROM ` + relation("Mineral")},
		{"USysAuditTrail", `SELECT DISTINCT * FROM ` + relation("Audit") + ` ORDER BY "EditWhen"`},
		{"USysOther", `SELECT DISTINCT * FROM ` + relation("Other")},
		{"USysMetadata", `SELECT DISTINCT * FROM ` + relation("Metadata")},
	}
	columns, err := conn.QueryContext(ctx, "SELECT * FROM "+veg+" LIMIT 0")
	if err != nil {
		return err
	}
	fields, err := columns.Columns()
	columns.Close()
	if err != nil {
		return err
	}
	succession := slices.Contains(fields, "SuccessionYear")
	shared := `"ID", "PlotNumber", "Species"`
	if succession {
		shared = `"ID", "PlotNumber", "SuccessionYear", "Species"`
	}
	coverA := `"Cover1", "Cover2", "Cover3", "TotalA", "HeightA", "Cover4", "Cover5", "TotalB", "HeightB", "Collected"`
	coverB := `"Cover4", "Cover5", "TotalB", "Collected"`
	filterA := `"Cover1" IS NOT NULL OR "Cover2" IS NOT NULL OR "Cover3" IS NOT NULL OR "TotalA" IS NOT NULL OR "Cover4" IS NOT NULL OR "Cover5" IS NOT NULL OR "TotalB" IS NOT NULL`
	filterB := `"Cover4" IS NOT NULL OR "Cover5" IS NOT NULL OR "TotalB" IS NOT NULL`
	coverC := `"Cover6", "Collected"`
	coverD := `"Cover7", "Collected"`
	filterD := `"Cover7" IS NOT NULL`
	if !succession {
		coverA = `"Cover1", "Cover2", "Cover3", "TotalA", "HeightA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "HeightB", "Collected"`
		coverB = `"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "Collected"`
		filterA += ` OR "Cover5a" IS NOT NULL OR "Cover5b" IS NOT NULL OR "Cover5c" IS NOT NULL`
		filterB += ` OR "Cover5a" IS NOT NULL OR "Cover5b" IS NOT NULL OR "Cover5c" IS NOT NULL`
		coverC = `"Cover6", "Height6", "Collected"`
		coverD = `"Cover7", "Cover8", "Cover9", "Collected"`
		filterD = `"Cover7" IS NOT NULL OR "Cover8" IS NOT NULL OR "Cover9" IS NOT NULL`
	}
	viewSQL = append(viewSQL,
		struct{ name, query string }{"USysVegA", `SELECT DISTINCT ` + shared + `, ` + coverA + ` FROM ` + veg + ` WHERE ` + filterA},
		struct{ name, query string }{"USysVegB", `SELECT DISTINCT ` + shared + `, ` + coverB + ` FROM ` + veg + ` WHERE ` + filterB},
		struct{ name, query string }{"USysVegC", `SELECT DISTINCT ` + shared + `, ` + coverC + ` FROM ` + veg + ` WHERE "Cover6" IS NOT NULL`},
		struct{ name, query string }{"USysVegD", `SELECT DISTINCT ` + shared + `, ` + coverD + ` FROM ` + veg + ` WHERE ` + filterD},
	)
	for _, view := range viewSQL {
		if _, err := conn.ExecContext(ctx, `CREATE TEMP VIEW "`+view.name+`" AS `+view.query); err != nil {
			return fmt.Errorf("create %s: %w", view.name, err)
		}
	}
	return nil
}

func (coordinator *Coordinator) CountPlots(ctx context.Context) (int, error) {
	var count int
	err := coordinator.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM "USysEnv"`).Scan(&count)
	return count, err
}

func (coordinator *Coordinator) Close() error {
	connectionErr := coordinator.conn.Close()
	databaseErr := coordinator.database.Close()
	return errors.Join(connectionErr, databaseErr)
}
