package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	Choices                                  vprocore.ProjectMetadataTable
}

type siviProjectChoices = SIVIProjectChoices

func projectSIVIProjectChoices(ctx context.Context, original vprocore.ProjectMetadataTable) (vprocore.ProjectMetadataTable, error) {
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	columns, err := vprocore.SiteUnitTransferColumns(original, "ProjectID", "ProjectTitle")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("SIVI ProjectID choice schema: %w", err)
	}
	for name := range columns {
		if strings.EqualFold(name, "rowid") || strings.EqualFold(name, "_rowid_") || strings.EqualFold(name, "oid") {
			return vprocore.ProjectMetadataTable{}, errors.New("SIVI ProjectID choices require unshadowed physical row identities")
		}
	}
	result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, name := range []string{"ProjectID", "ProjectTitle"} {
		result.Columns = append(result.Columns, original.Columns[columns[name]])
	}
	for _, row := range original.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, err
		}
		projected := vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: []vprocore.ProjectMetadataCell{}}
		for _, name := range []string{"ProjectID", "ProjectTitle"} {
			cell := row.Cells[columns[name]]
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return vprocore.ProjectMetadataTable{}, fmt.Errorf("SIVI ProjectID choice row %s: %w", row.RowID, err)
			}
			projected.Cells = append(projected.Cells, vprocore.CloneSiteUnitCell(cell))
		}
		result.Rows = append(result.Rows, projected)
	}
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	return result, nil
}

func ReadSIVIProjectChoices(s *vprocore.ContextService, ctx context.Context, contextID string) (*siviProjectChoices, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviProjectChoices, error) {
		return readOwnedSIVIProjectChoices(ctx, plots, contextID)
	})
}

func readOwnedSIVIProjectChoices(ctx context.Context, plots *vprocore.PlotService, contextID string) (*siviProjectChoices, error) {
	values, err := plots.Projects.Preferences.Snapshot()
	if err != nil {
		return nil, err
	}
	source, err := vprocore.ConfigInt(values, "Current", "ProjectIdSource", 1, 2)
	if err != nil {
		return nil, fmt.Errorf("SIVI ProjectID source unavailable: %w", err)
	}
	return ReadOwnedSIVIProjectChoicesAtSource(ctx, plots, contextID, source)
}

func ReadOwnedSIVIProjectChoicesAtSource(ctx context.Context, plots *vprocore.PlotService, contextID string, source int) (*SIVIProjectChoices, error) {
	if source != 1 && source != 2 {
		return nil, errors.New("SIVI ProjectID source must be Env (1) or Master (2)")
	}
	return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SIVIProjectChoices, error) {
		return readSIVIProjectChoicesSnapshot(ctx, owner, tx, contextID, source)
	})
}

func readSIVIProjectChoicesSnapshot(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID string, source int) (*SIVIProjectChoices, error) {
	return ReadSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, source, "project")
}

func ReadSIVIProjectChoicesAtAlias(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID string, source int, projectAlias string) (*SIVIProjectChoices, error) {
	if source != 1 && source != 2 {
		return nil, errors.New("SIVI ProjectID source must be Env (1) or Master (2)")
	}
	result := &siviProjectChoices{ContextID: contextID, Project: owner.Selection.Project, SourceOption: source,
		Source: "Env", Alias: "project", Table: owner.Selection.Project + "_Metadata"}
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
	matches, err := tx.QueryContext(ctx, `SELECT name FROM `+vprocore.QuoteHeaderIdentifier(sqlAlias)+
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
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, sqlAlias, result.Table, "ProjectID choices"); err != nil {
		return nil, err
	}
	original, err := vprocore.ReadSQLiteStorageRows(ctx, tx, sqlAlias, result.Table, "", nil, "")
	if err != nil {
		return nil, err
	}
	result.Choices, err = projectSIVIProjectChoices(ctx, original)
	if err != nil {
		return nil, err
	}
	return result, nil
}
