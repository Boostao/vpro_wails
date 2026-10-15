package main

import (
	"context"
	"fmt"
	"strconv"
	"unicode/utf8"
)

type OrdinaryAuditReadEntry struct {
	RowID      string  `json:"rowId"`
	Project    string  `json:"project"`
	User       *string `json:"user"`
	PlotNumber string  `json:"plotNumber"`
	Table      string  `json:"table"`
	EditField  string  `json:"editField"`
	EditWhen   *string `json:"editWhen"`
	BeforeEdit *string `json:"beforeEdit"`
	AfterEdit  *string `json:"afterEdit"`
	Restore    bool    `json:"restore"`
	Flag       bool    `json:"flag"`
	ID         *int64  `json:"id"`
}

func (s *ContextService) ListAuditReadEntries(ctx context.Context, contextID, plot string) ([]OrdinaryAuditReadEntry, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]OrdinaryAuditReadEntry, error) {
		return plots.ListAuditReadEntries(plot)
	})
}

func (s *PlotService) ListAuditReadEntries(plot string) ([]OrdinaryAuditReadEntry, error) {
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer release()
	rows, err := queryAuditEntries(s.readDB(db), project, `"PlotNumber" = ?`, plot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []OrdinaryAuditReadEntry{}
	for rows.Next() {
		var entry OrdinaryAuditReadEntry
		var id int64
		if err := rows.Scan(&id, &entry.Project, &entry.User, &entry.PlotNumber,
			&entry.Table, &entry.EditField, &entry.EditWhen, &entry.BeforeEdit,
			&entry.AfterEdit, &entry.Restore, &entry.Flag, &entry.ID); err != nil {
			return nil, fmt.Errorf("invalid ordinary audit row: %w", err)
		}
		entry.RowID = strconv.FormatInt(id, 10)
		if entry.Project != project {
			return nil, fmt.Errorf("audit rowId %s has foreign project ownership", entry.RowID)
		}
		texts := []string{entry.Project, entry.PlotNumber, entry.Table, entry.EditField}
		for _, value := range []*string{entry.User, entry.EditWhen, entry.BeforeEdit, entry.AfterEdit} {
			if value != nil {
				texts = append(texts, *value)
			}
		}
		for _, value := range texts {
			if !utf8.ValidString(value) {
				return nil, fmt.Errorf("ordinary audit rowId %s contains malformed UTF-8; no text repaired", entry.RowID)
			}
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
