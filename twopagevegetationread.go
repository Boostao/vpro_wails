package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type twoPageVegetationOriginal struct {
	ContextID    string               `json:"contextId"`
	Project      string               `json:"project"`
	Plot         string               `json:"plot"`
	Form         string               `json:"form"`
	RecordSource string               `json:"recordSource"`
	Table        string               `json:"table"`
	Records      ProjectMetadataTable `json:"records"`
	SourceRowIDs []string             `json:"sourceRowIds"`
}

func twoPageVegetationSource(form string) (string, []string, error) {
	switch form {
	case "SubVegAXL_BC", "SubVegAXL":
		return "USysVegA", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"}, nil
	case "SubVegCXL":
		return "USysVegC", []string{"Cover6"}, nil
	case "SubVegDXL":
		return "USysVegD", []string{"Cover7", "Cover8", "Cover9"}, nil
	default:
		return "", nil, fmt.Errorf("two-page vegetation original does not support source form %q", form)
	}
}

func (s *ContextService) readTwoPageVegetationOriginal(ctx context.Context, contextID, plot, form string) (*twoPageVegetationOriginal, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page vegetation original requires an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page vegetation original requires an exact valid literal parent")
	}
	source, covers, err := twoPageVegetationSource(form)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageVegetationOriginal, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageVegetationOriginal, error) {
			project := owner.selection.Project
			table := project + "_Veg"
			if err := requireTwoPageChildReadScope(ctx, tx, project, table, plot, "vegetation"); err != nil {
				return nil, err
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page vegetation originals"); err != nil {
				return nil, fmt.Errorf("two-page vegetation original schema: %w", err)
			}
			records, err := readSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation original storage: %w", err)
			}
			sourceRows, err := twoPageVegetationSourceRows(records, source, covers)
			if err != nil {
				return nil, err
			}
			return &twoPageVegetationOriginal{ContextID: contextID, Project: project, Plot: plot, Form: form,
				RecordSource: source, Table: table, Records: records, SourceRowIDs: sourceRows}, nil
		})
	})
}

func twoPageVegetationSourceRows(records ProjectMetadataTable, source string, covers []string) ([]string, error) {
	indices := make([]int, 0, len(covers))
	for _, cover := range covers {
		index, matches := -1, 0
		for i, column := range records.Columns {
			if strings.EqualFold(column.Name, cover) {
				index, matches = i, matches+1
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("two-page vegetation source %s requires one physical cover column %q", source, cover)
		}
		indices = append(indices, index)
	}
	sourceRows := []string{}
	for _, row := range records.Rows {
		if len(row.Cells) != len(records.Columns) {
			return nil, errors.New("two-page vegetation source requires complete physical rows")
		}
		for _, index := range indices {
			if row.Cells[index].Storage != "null" {
				sourceRows = append(sourceRows, row.RowID)
				break
			}
		}
	}
	return sourceRows, nil
}
