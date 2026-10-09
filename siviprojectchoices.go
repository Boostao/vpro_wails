package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type SIVIProjectChoices struct {
	ContextID, Project, Source, Alias, Table string
	SourceOption                             int
	Choices                                  ProjectMetadataTable
}

type siviProjectChoices = SIVIProjectChoices

func projectSIVIProjectChoices(ctx context.Context, original ProjectMetadataTable) (ProjectMetadataTable, error) {
	if err := ctx.Err(); err != nil {
		return ProjectMetadataTable{}, err
	}
	columns, err := siteUnitTransferColumns(original, "ProjectID", "ProjectTitle")
	if err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("SIVI ProjectID choice schema: %w", err)
	}
	for name := range columns {
		if strings.EqualFold(name, "rowid") || strings.EqualFold(name, "_rowid_") || strings.EqualFold(name, "oid") {
			return ProjectMetadataTable{}, errors.New("SIVI ProjectID choices require unshadowed physical row identities")
		}
	}
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	for _, name := range []string{"ProjectID", "ProjectTitle"} {
		result.Columns = append(result.Columns, original.Columns[columns[name]])
	}
	for _, row := range original.Rows {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, err
		}
		projected := ProjectMetadataRow{RowID: row.RowID, Cells: []ProjectMetadataCell{}}
		for _, name := range []string{"ProjectID", "ProjectTitle"} {
			cell := row.Cells[columns[name]]
			if _, err := metadataCellValue(cell); err != nil {
				return ProjectMetadataTable{}, fmt.Errorf("SIVI ProjectID choice row %s: %w", row.RowID, err)
			}
			projected.Cells = append(projected.Cells, cloneSiteUnitCell(cell))
		}
		result.Rows = append(result.Rows, projected)
	}
	if err := ctx.Err(); err != nil {
		return ProjectMetadataTable{}, err
	}
	return result, nil
}

func (s *ContextService) readSIVIProjectChoices(ctx context.Context, contextID string) (*siviProjectChoices, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviProjectChoices, error) {
		return readOwnedSIVIProjectChoices(ctx, plots, contextID)
	})
}

func readOwnedSIVIProjectChoices(ctx context.Context, plots *PlotService, contextID string) (*siviProjectChoices, error) {
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return nil, err
	}
	source, err := configInt(values, "Current", "ProjectIdSource", 1, 2)
	if err != nil {
		return nil, fmt.Errorf("SIVI ProjectID source unavailable: %w", err)
	}
	return readOwnedSIVIProjectChoicesAtSource(ctx, plots, contextID, source)
}

func readOwnedSIVIProjectChoicesAtSource(ctx context.Context, plots *PlotService, contextID string, source int) (*SIVIProjectChoices, error) {
	if source != 1 && source != 2 {
		return nil, errors.New("SIVI ProjectID source must be Env (1) or Master (2)")
	}
	return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*SIVIProjectChoices, error) {
		return readSIVIProjectChoicesSnapshot(ctx, owner, tx, contextID, source)
	})
}

func readSIVIProjectChoicesSnapshot(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID string, source int) (*SIVIProjectChoices, error) {
	return readSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, source, "project")
}

func readSIVIProjectChoicesAtAlias(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID string, source int, projectAlias string) (*SIVIProjectChoices, error) {
	if source != 1 && source != 2 {
		return nil, errors.New("SIVI ProjectID source must be Env (1) or Master (2)")
	}
	result := &siviProjectChoices{ContextID: contextID, Project: owner.selection.Project, SourceOption: source,
		Source: "Env", Alias: "project", Table: owner.selection.Project + "_Metadata"}
	collation := "BINARY"
	if source == 2 {
		result.Source, result.Alias, result.Table = "Master", "VMetaData", "ProjectMetadata"
		collation = "NOCASE"
	}
	sqlAlias := projectAlias
	if source == 2 {
		sqlAlias = result.Alias
	}
	// Access identifiers are case-insensitive; retain the master's actual physical
	// spelling, never synthesize a view or silently select the other source.
	matches, err := tx.QueryContext(ctx, `SELECT name FROM `+quoteHeaderIdentifier(sqlAlias)+
		`.sqlite_master WHERE type='table' AND name COLLATE `+collation+`=?`, result.Table)
	if err != nil {
		return nil, err
	}
	count := 0
	for matches.Next() {
		if err := matches.Scan(&result.Table); err != nil {
			matches.Close()
			return nil, err
		}
		count++
	}
	if err := errors.Join(matches.Err(), matches.Close()); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("SIVI ProjectID source %s requires one original physical metadata table", result.Source)
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, sqlAlias, result.Table, "ProjectID choices"); err != nil {
		return nil, err
	}
	original, err := readSQLiteStorageRows(ctx, tx, sqlAlias, result.Table, "", nil, "")
	if err != nil {
		return nil, err
	}
	result.Choices, err = projectSIVIProjectChoices(ctx, original)
	if err != nil {
		return nil, err
	}
	return result, nil
}
