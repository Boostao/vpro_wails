package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type siviCategoricalChoices struct {
	ContextID, Project, ControlID, ControlName, Binding string
	SourceType, Alias, Table, ListName, Membership      string
	Ordering                                            string
	SourceDistinctRow                                   bool
	Definitions                                         ProjectMetadataTable
	Values                                              []ProjectMetadataCell
}

func siviCategoricalSource(binding string) (*siviCategoricalChoices, error) {
	result := &siviCategoricalChoices{
		Binding: binding, SourceType: "Table/Query", Alias: "VLists", Table: "USysTableOfLists",
		Membership: "literal-binary-list-name", Ordering: "sqlite-ItemOrder-then-physical-rowid",
		Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Values:      []ProjectMetadataCell{},
	}
	switch binding {
	case "SnowCoverregime":
		result.ControlName, result.ListName = "SnowCoverregime", "SnowCoverRegime"
	case "SV_RootZoneTexture":
		result.ControlName, result.ListName, result.SourceDistinctRow = "RootZoneTexture", "SoilTexture", true
	case "SV_AhorizonType":
		result.ControlName, result.SourceType = "AhorizonType", "Value List"
		result.Alias, result.Table, result.Membership, result.Ordering = "", "", "source-value-list", "source-order"
		for _, value := range []string{"Ah", "Ae", ""} {
			result.Values = append(result.Values, ProjectMetadataCell{Storage: "text", Text: &value})
		}
	default:
		return nil, fmt.Errorf("SIVI categorical binding %q is outside the source scope", binding)
	}
	bindings, err := siviParentSourceBindings()
	if err != nil {
		return nil, err
	}
	for _, source := range bindings {
		if source.Binding == binding && !source.Implicit {
			result.ControlID = source.ControlID
			return result, nil
		}
	}
	return nil, fmt.Errorf("SIVI categorical binding %q has no direct source control", binding)
}

func (s *ContextService) readSIVICategoricalChoices(ctx context.Context, contextID, binding string) (*siviCategoricalChoices, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviCategoricalChoices, error) {
		return readOwnedSIVICategoricalChoices(ctx, plots, contextID, binding)
	})
}

func readOwnedSIVICategoricalChoices(ctx context.Context, plots *PlotService, contextID, binding string) (result *siviCategoricalChoices, resultErr error) {
	result, err := siviCategoricalSource(binding)
	if err != nil {
		return nil, err
	}
	owner := plots.projects.sqlite
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return nil, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	tx, err := owner.beginReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = nil
		}
	}()
	result.ContextID, result.Project = contextID, owner.selection.Project
	if result.SourceType == "Table/Query" {
		if err := tx.QueryRowContext(ctx, `SELECT name FROM VLists.sqlite_master
			WHERE type='table' AND name COLLATE NOCASE=?`, result.Table).Scan(&result.Table); err != nil {
			return nil, fmt.Errorf("SIVI categorical source requires physical VLists table: %w", err)
		}
		if err := validateSIVICategoricalSchema(ctx, tx, result.Alias, result.Table); err != nil {
			return nil, err
		}
		result.Definitions, err = readSQLiteStorageRows(ctx, tx, result.Alias, result.Table, "ListName", &result.ListName, "ItemOrder")
		if err != nil {
			return nil, err
		}
		_, err = siteUnitTransferColumns(result.Definitions, "ListName", "Item", "ItemDescription", "ItemOrder")
		if err != nil {
			return nil, fmt.Errorf("SIVI categorical physical definitions: %w", err)
		}
		if len(result.Definitions.Rows) == 0 {
			return nil, fmt.Errorf("SIVI categorical list %q is unavailable", result.ListName)
		}
		for _, row := range result.Definitions.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for _, cell := range row.Cells {
				if _, err := metadataCellValue(cell); err != nil {
					return nil, fmt.Errorf("SIVI categorical physical row %s: %w", row.RowID, err)
				}
			}
		}
	}
	if err := profileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func validateSIVICategoricalSchema(ctx context.Context, tx *sql.Tx, alias, table string) error {
	return validateSIVIPhysicalSchema(ctx, tx, alias, table, "categorical definitions")
}

func validateSIVIPhysicalSchema(ctx context.Context, tx *sql.Tx, alias, table, scope string) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA "+quoteHeaderIdentifier(alias)+".table_xinfo("+quoteHeaderIdentifier(table)+")")
	if err != nil {
		return err
	}
	for rows.Next() {
		var position, required, primary, hidden int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&position, &name, &kind, &required, &defaultValue, &primary, &hidden); err != nil {
			return errors.Join(err, rows.Close())
		}
		if hidden != 0 || strings.EqualFold(name, "rowid") || strings.EqualFold(name, "_rowid_") || strings.EqualFold(name, "oid") {
			return errors.Join(fmt.Errorf("SIVI %s require unshadowed physical identities and ordinary columns", scope), rows.Close())
		}
	}
	return errors.Join(rows.Err(), rows.Close())
}
