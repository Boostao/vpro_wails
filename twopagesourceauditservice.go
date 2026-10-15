package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

const twoPageSourceAuditReviewEnvironment = "VPRO_TWO_PAGE_SOURCE_AUDIT_REVIEW"

type TwoPageSourceAuditOriginal struct {
	ContextID    string               `json:"contextId"`
	Project      string               `json:"project"`
	Plot         string               `json:"plot"`
	Form         string               `json:"form"`
	RecordSource string               `json:"recordSource"`
	Table        string               `json:"table"`
	Records      ProjectMetadataTable `json:"records"`
}

type TwoPageSourceAuditService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageSourceAuditService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageSourceAuditService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page source audit requires owned contexts and explicit feature lookup")
	}
	review, err := siviFeature(twoPageSourceAuditReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageSourceAuditService{contexts: contexts, review: review}, nil
}

func (s *TwoPageSourceAuditService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSourceAuditOriginal, error) {
	if ctx == nil {
		return nil, errors.New("two-page source audit requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.review {
		return nil, errors.New("two-page source audit reads require independently enabled audit review")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page source audit requires an exact valid literal parent")
	}
	if form != "FS882-8x6XL" && form != "FS882-8x6XL-CHARS" {
		return nil, fmt.Errorf("two-page source audit does not support source form %q", form)
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageSourceAuditOriginal, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageSourceAuditOriginal, error) {
			project := owner.selection.Project
			table := project + "_Audit"
			if err := requireTwoPageChildReadScope(ctx, tx, project, table, plot, "audit"); err != nil {
				return nil, err
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page source audit"); err != nil {
				return nil, err
			}
			columns, err := readSQLiteStorageColumns(ctx, tx, "project", table)
			if err != nil {
				return nil, err
			}
			for _, name := range []string{"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID"} {
				found := false
				for _, column := range columns {
					found = found || column.Name == name
				}
				if !found {
					return nil, fmt.Errorf("two-page source audit requires exact source column %q; no binding was repaired", name)
				}
			}
			records, err := readSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "EditWhen")
			if err != nil {
				return nil, fmt.Errorf("two-page source audit storage: %w", err)
			}
			for _, row := range records.Rows {
				for index, column := range records.Columns {
					if column.Name == "Project" && (row.Cells[index].Storage != "text" || row.Cells[index].Text == nil || *row.Cells[index].Text != project) {
						return nil, fmt.Errorf("two-page source audit row %s has foreign or nonliteral project ownership", row.RowID)
					}
				}
			}
			return &TwoPageSourceAuditOriginal{ContextID: contextID, Project: project, Plot: plot, Form: form,
				RecordSource: "UsysAuditTrail", Table: table, Records: records}, nil
		})
	})
}
