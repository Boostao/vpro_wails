package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
)

type ownedTableCSVReview struct {
	ContextID                  string
	Project                    string
	ProjectPath                string
	DescriptionMetadataPresent bool
	Document                   tableCSVDocument
}

func (s *ContextService) readProjectTableCSV(ctx context.Context, contextID, table string) (ownedTableCSVReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ownedTableCSVReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (ownedTableCSVReview, error) {
			var zero ownedTableCSVReview
			names := make([]string, len(coreTables))
			for index, suffix := range coreTables {
				names[index] = owner.selection.Project + "_" + suffix
			}
			if !slices.Contains(names, table) {
				return zero, errors.New("table CSV review requires one literal core table from the owned project; arbitrary SQL, support tables and views are unavailable")
			}
			var kind string
			if err := tx.QueryRowContext(ctx, `SELECT type FROM project.sqlite_master WHERE name COLLATE BINARY=?`, table).Scan(&kind); err != nil {
				return zero, fmt.Errorf("table CSV physical source unavailable: %w", err)
			}
			if kind != "table" {
				return zero, errors.New("table CSV source must be an original physical table, not a view")
			}
			physical, err := readSQLiteStorageRows(ctx, tx, "project", table, "", nil, "")
			if err != nil {
				return zero, fmt.Errorf("table CSV physical rows unavailable: %w", err)
			}
			metadata, err := readProfileDescriptions(ctx, tx, "project", table)
			if err != nil {
				return zero, fmt.Errorf("table CSV Description metadata unavailable: %w", err)
			}
			descriptions := []TableCSVDescription{}
			descriptionIndex := -1
			for index, column := range metadata.Columns {
				if column.Name == "description" {
					descriptionIndex = index
				}
			}
			for _, row := range metadata.Rows {
				if descriptionIndex < 0 || descriptionIndex >= len(row.Cells) {
					return zero, errors.New("table CSV Description binding is unavailable; no candidate was inferred")
				}
				descriptions = append(descriptions, TableCSVDescription{row.RowID, row.Cells[descriptionIndex]})
			}
			document, err := encodeTableCSV(ctx, table, physical, descriptions)
			if err != nil {
				return zero, err
			}
			return ownedTableCSVReview{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				DescriptionMetadataPresent: len(metadata.Columns) > 0, Document: document,
			}, nil
		})
	})
}
