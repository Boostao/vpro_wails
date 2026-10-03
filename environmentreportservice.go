package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type LongEnvironmentRequest struct {
	Title string `json:"title"`
}

func (request *LongEnvironmentRequest) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentRequest
	var value plain
	if err := decodeProfileLifecycleJSON(data, &value, "title"); err != nil {
		return err
	}
	*request = LongEnvironmentRequest(value)
	return nil
}

type LongEnvironmentOptions struct {
	ContextID string `json:"contextId"`
	Title     string `json:"title"`
}

type LongEnvironmentPreview struct {
	ContextID   string            `json:"contextId"`
	ProjectPath string            `json:"projectPath"`
	SUPath      string            `json:"suPath"`
	Report      EnvironmentReport `json:"report"`
}

func (s *ContextService) GetLongEnvironmentOptions(ctx context.Context, contextID string) (LongEnvironmentOptions, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (LongEnvironmentOptions, error) {
		values, err := plots.projects.preferences.snapshot()
		if err != nil {
			return LongEnvironmentOptions{}, err
		}
		title, err := configString(values, "ReportOptions", "LEReportTitle")
		if err != nil {
			return LongEnvironmentOptions{}, err
		}
		if strings.ContainsRune(title, 0) {
			return LongEnvironmentOptions{}, errors.New("ReportOptions.LEReportTitle contains NUL; correct the configuration explicitly")
		}
		return LongEnvironmentOptions{contextID, title}, nil
	})
}

func (s *ContextService) PreviewLongEnvironment(ctx context.Context, contextID string, request LongEnvironmentRequest) (LongEnvironmentPreview, error) {
	if !utf8.ValidString(request.Title) || strings.ContainsRune(request.Title, 0) {
		return LongEnvironmentPreview{}, errors.New("Long Environment title requires complete Unicode without NUL; no repair or normalization")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result LongEnvironmentPreview, resultErr error) {
		owner := plots.projects.sqlite
		if owner.selection.SU == "None" {
			return result, errors.New("Long Environment requires an explicitly selected SU; project-only plots are not an implicit scope")
		}
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return result, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		tx, err := owner.conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return result, err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
			if resultErr != nil {
				result = LongEnvironmentPreview{}
			}
		}()
		tables := make([]ProjectMetadataTable, 4)
		for i, source := range []struct{ role, table string }{
			{"project", owner.selection.Project + "_Env"},
			{"project", owner.selection.Project + "_Admin"},
			{"su", owner.selection.SU + "_SU"},
			{"VLists", "MasterSiteUnitList"},
		} {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
				`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
				return result, err
			}
			if count != 1 {
				return result, fmt.Errorf("Long Environment requires original physical table %s.%s", source.role, source.table)
			}
			tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
			if err != nil {
				return result, err
			}
		}
		report, err := planLongEnvironment(ctx, owner.selection.Project, owner.selection.SU, request.Title, tables[0], tables[1], tables[2], tables[3])
		if err != nil {
			return result, err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return LongEnvironmentPreview{contextID, owner.selection.ProjectPath, owner.selection.SUPath, report}, nil
	})
}
