package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type SIVIParentJoinReview struct {
	ContextID, Project, Plot, Scope, Diagnostic string
	Verified                                    bool
	EnvRowIDs, AdminRowIDs                      []string
}

func siviASCIIPlotKey(value string) (string, bool) {
	if len(value) == 0 || len(value) > 7 {
		return "", false
	}
	key := strings.TrimRight(value, " ")
	if key == "" {
		return "", false
	}
	for _, char := range key {
		if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return "", false
		}
	}
	return strings.ToUpper(key), true
}

func readOwnedSIVIParentJoin(ctx context.Context, plots *PlotService, contextID, plot string) (*SIVIParentJoinReview, error) {
	return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*SIVIParentJoinReview, error) {
		return readSIVIParentJoin(ctx, tx, "project", contextID, owner.selection.Project, plot)
	})
}

func readSIVIParentJoin(ctx context.Context, tx *sql.Tx, alias, contextID, project, plot string) (*SIVIParentJoinReview, error) {
	result := &SIVIParentJoinReview{ContextID: contextID, Project: project, Plot: plot,
		Scope: "DAO-General1033-ASCII-alphanumeric-TEXT7", EnvRowIDs: []string{}, AdminRowIDs: []string{}}
	requested, supported := siviASCIIPlotKey(plot)
	if !supported {
		result.Diagnostic = "Plot identity is outside the independently probed ASCII TEXT7 join domain."
		return result, nil
	}
	unavailable := false
	literal := []int{0, 0}
	for i, suffix := range []string{"_Env", "_Admin"} {
		table, column := project+suffix, "PlotNumber"
		if i == 1 {
			column = "Plot"
		}
		if err := validateSIVIPhysicalSchema(ctx, tx, alias, table, "join review"); err != nil {
			return nil, err
		}
		keyColumn := quoteHeaderIdentifier(column)
		query := "SELECT rowid,typeof(" + keyColumn + "),CAST(" + keyColumn + " AS BLOB) FROM " + quoteHeaderIdentifier(alias) + "." +
			quoteHeaderIdentifier(table) + " ORDER BY rowid"
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var storage string
			var raw []byte
			if err := rows.Scan(&id, &storage, &raw); err != nil {
				return nil, errors.Join(err, rows.Close())
			}
			if storage == "null" {
				continue
			}
			if storage != "text" {
				unavailable = true
				continue
			}
			cell, err := projectMetadataCell(storage, raw)
			if err != nil {
				return nil, errors.Join(err, rows.Close())
			}
			if _, err := metadataCellValue(cell); err != nil {
				return nil, errors.Join(err, rows.Close())
			}
			key, valid := siviASCIIPlotKey(*cell.Text)
			if !valid {
				unavailable = true
				continue
			}
			if key != requested {
				continue
			}
			idText := strconv.FormatInt(id, 10)
			if i == 0 {
				result.EnvRowIDs = append(result.EnvRowIDs, idText)
			} else {
				result.AdminRowIDs = append(result.AdminRowIDs, idText)
			}
			if *cell.Text == plot {
				literal[i]++
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
	}
	result.Verified = !unavailable && literal[0] == 1 && literal[1] == 1 &&
		len(result.EnvRowIDs) == 1 && len(result.AdminRowIDs) == 1
	switch {
	case unavailable:
		result.Diagnostic = "A physical join identity has unsupported storage, Unicode, punctuation, empty text or length; source membership is not certified."
	case !result.Verified:
		result.Diagnostic = fmt.Sprintf("Source comparison finds %d Env and %d Admin rows; one exact unambiguous physical pair is required.", len(result.EnvRowIDs), len(result.AdminRowIDs))
	default:
		result.Diagnostic = "One exact physical pair; case/trailing-space aliases are absent in the independently probed ASCII domain. This is not write authorization."
	}
	return result, nil
}
