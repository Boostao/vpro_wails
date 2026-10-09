package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

type twoPageSoilOriginal struct {
	ContextID    string               `json:"contextId"`
	Project      string               `json:"project"`
	Plot         string               `json:"plot"`
	Form         string               `json:"form"`
	RecordSource string               `json:"recordSource"`
	Table        string               `json:"table"`
	Records      ProjectMetadataTable `json:"records"`
}

func (s *ContextService) readTwoPageSoilOriginal(ctx context.Context, contextID, plot, form string) (*twoPageSoilOriginal, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page soil original requires an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page soil original requires an exact valid literal parent")
	}
	var kind, source string
	switch form {
	case "SoilHumusXL":
		kind, source = "Humus", "UsysHumus"
	case "SoilMineralXL":
		kind, source = "Mineral", "UsysMineral"
	default:
		return nil, fmt.Errorf("two-page soil original does not support source form %q", form)
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*twoPageSoilOriginal, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*twoPageSoilOriginal, error) {
			project := owner.selection.Project
			table := project + "_" + kind
			if err := requireTwoPageSoilReadScope(ctx, tx, project, table, plot); err != nil {
				return nil, err
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page soil originals"); err != nil {
				return nil, fmt.Errorf("two-page soil original %s schema: %w", kind, err)
			}
			records, err := readSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page soil original %s storage: %w", kind, err)
			}
			return &twoPageSoilOriginal{ContextID: contextID, Project: project, Plot: plot, Form: form,
				RecordSource: source, Table: table, Records: records}, nil
		})
	})
}

func requireTwoPageSoilReadScope(ctx context.Context, tx *sql.Tx, project, table, plot string) error {
	return requireTwoPageChildReadScope(ctx, tx, project, table, plot, "soil")
}
