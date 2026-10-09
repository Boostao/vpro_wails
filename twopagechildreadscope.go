package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func requireTwoPageChildReadScope(ctx context.Context, tx *sql.Tx, project, table, plot, child string) error {
	for _, required := range []struct{ table, column string }{
		{project + "_Env", "PlotNumber"}, {project + "_Admin", "Plot"}, {table, "PlotNumber"},
	} {
		var physical int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master
			WHERE type='table' AND name COLLATE BINARY=?`, required.table).Scan(&physical); err != nil {
			return fmt.Errorf("two-page %s original physical schema: %w", child, err)
		}
		if physical != 1 {
			return fmt.Errorf("two-page %s original requires original physical project table %q", child, required.table)
		}
		columns, err := readSQLiteStorageColumns(ctx, tx, "project", required.table)
		if err != nil {
			return fmt.Errorf("two-page %s original physical parent schema: %w", child, err)
		}
		found := false
		for _, column := range columns {
			if strings.EqualFold(column.Name, required.column) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("two-page %s original %q is missing required physical parent column %q", child, required.table, required.column)
		}
	}
	var selected bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv
		WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB))`, plot).Scan(&selected); err != nil {
		return fmt.Errorf("two-page %s selected parent: %w", child, err)
	}
	if !selected {
		return fmt.Errorf("two-page %s parent is outside the selected context", child)
	}
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		var count int
		column := quoteHeaderIdentifier(parent.column)
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.`+quoteHeaderIdentifier(project+"_"+parent.table)+
			` WHERE typeof(`+column+`)='text' AND CAST(`+column+` AS BLOB)=CAST(? AS BLOB)`, plot).Scan(&count); err != nil {
			return fmt.Errorf("two-page %s original %s parent: %w", child, parent.table, err)
		}
		if count != 1 {
			return fmt.Errorf("two-page %s original requires one literal physical %s parent", child, parent.table)
		}
	}
	return nil
}
