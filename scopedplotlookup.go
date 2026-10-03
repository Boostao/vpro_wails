package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

type ScopedPlotLookup struct {
	PlotNumber string `json:"plotNumber"`
}

func (request *ScopedPlotLookup) UnmarshalJSON(data []byte) error {
	type plain ScopedPlotLookup
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "plotNumber"); err != nil {
		return err
	}
	*request = ScopedPlotLookup(decoded)
	return nil
}

type ScopedPlotLookupResult struct {
	ContextID string      `json:"contextId"`
	Plot      PlotSummary `json:"plot"`
}

func (s *ContextService) LookupScopedPlot(ctx context.Context, contextID string, request ScopedPlotLookup) (ScopedPlotLookupResult, error) {
	if request.PlotNumber == "" || !utf8.ValidString(request.PlotNumber) || strings.ContainsRune(request.PlotNumber, 0) {
		return ScopedPlotLookupResult{}, errors.New("find requires a nonempty literal plot number with complete Unicode and no NUL; no normalization")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result ScopedPlotLookupResult, resultErr error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return result, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		tx, err := owner.conn.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
			if resultErr != nil {
				result = ScopedPlotLookupResult{}
			}
		}()
		var env, admin int
		project := owner.selection.Project
		if err := tx.QueryRowContext(ctx, `SELECT
			(SELECT COUNT(*) FROM project.`+quoteHeaderIdentifier(project+"_Env")+` WHERE PlotNumber COLLATE BINARY=?),
			(SELECT COUNT(*) FROM project.`+quoteHeaderIdentifier(project+"_Admin")+` WHERE Plot COLLATE BINARY=?)`,
			request.PlotNumber, request.PlotNumber).Scan(&env, &admin); err != nil {
			return result, err
		}
		if env != 1 || admin != 1 {
			return result, errors.New("literal plot requires exactly one original Env and Admin record; missing or ambiguous identity cannot be opened")
		}
		if err := tx.QueryRowContext(ctx, `SELECT PlotNumber,FieldNumber,PlotRepresenting,Zone,SubZone,SiteSeries
			FROM USysEnv WHERE PlotNumber COLLATE BINARY=?`, request.PlotNumber).
			Scan(&result.Plot.PlotNumber, &result.Plot.FieldNumber, &result.Plot.PlotRepresenting,
				&result.Plot.Zone, &result.Plot.SubZone, &result.Plot.SiteSeries); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return result, errors.New("literal plot is outside the current project/SU recordset; current scope is unchanged")
			}
			return result, err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		result.ContextID = contextID
		return result, nil
	})
}
