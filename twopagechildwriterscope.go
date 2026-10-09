package main

import (
	"context"
	"database/sql"
	"fmt"
)

func twoPageChildWriterSelectedParent(ctx context.Context, tx *sql.Tx, owner *sqliteContext, suAlias, plot, child string) error {
	project := owner.selection.Project
	query := `SELECT EXISTS(SELECT 1 FROM main.` + quoteHeaderIdentifier(project+"_Env") +
		` AS e INNER JOIN main.` + quoteHeaderIdentifier(project+"_Admin") + ` AS a ON e.PlotNumber=a.Plot
		WHERE typeof(e.PlotNumber)='text' AND CAST(e.PlotNumber AS BLOB)=CAST(? AS BLOB)`
	if suAlias != "" {
		query += ` AND EXISTS(SELECT 1 FROM ` + quoteHeaderIdentifier(suAlias) + "." +
			quoteHeaderIdentifier(owner.selection.SU+"_SU") + ` AS su WHERE su.PlotNumber=e.PlotNumber)`
	}
	query += `)`
	var selected bool
	if err := tx.QueryRowContext(ctx, query, plot).Scan(&selected); err != nil {
		return fmt.Errorf("two-page %s selected parent: %w", child, err)
	}
	if !selected {
		return fmt.Errorf("two-page %s parent is outside the selected context", child)
	}
	return nil
}
