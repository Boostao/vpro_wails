package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
)

type projectPlotProfilePresencePreview struct {
	Project     string   `json:"project"`
	Table       string   `json:"table"`
	SU          string   `json:"su"`
	TotalPlots  int      `json:"totalPlots"`
	PlotNumbers []string `json:"plotNumbers"`
}

func (s *ContextService) previewProjectPlotProfilePresence(ctx context.Context, contextID string, request ProjectPlotProfileRunRequest) (projectPlotProfilePresencePreview, error) {
	return s.previewProjectPlotProfilePresenceObserved(ctx, contextID, request, nil)
}

func (s *ContextService) previewProjectPlotProfilePresenceObserved(ctx context.Context, contextID string, request ProjectPlotProfileRunRequest,
	observe func(*sqliteContext, *sql.Tx) error) (projectPlotProfilePresencePreview, error) {
	if ctx == nil {
		return projectPlotProfilePresencePreview{}, errors.New("species-only profile requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return projectPlotProfilePresencePreview{}, err
	}
	if request.Subvarieties {
		return projectPlotProfilePresencePreview{}, errors.New("species-only subvariety preparation remains unavailable; no global Lump state was inherited")
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result projectPlotProfilePresencePreview, resultErr error) {
		owner := plots.projects.sqlite
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		job, err := newSQLiteContext(ctx, owner.selection, plots.projects.supportPaths, owner.profile)
		if err != nil {
			return result, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, job.Close())
			if resultErr != nil {
				result = projectPlotProfilePresencePreview{}
			}
		}()
		if err := registerProfileFunctions(job.conn); err != nil {
			return result, err
		}
		tx, err := job.beginReadSnapshot(ctx)
		if err != nil {
			return result, err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
			if resultErr != nil {
				result = projectPlotProfilePresencePreview{}
			}
		}()
		role, table, err := job.profileLocation()
		if err != nil {
			return result, err
		}
		fresh, err := readSQLiteStorageRows(ctx, tx, role, table, "", nil, "Order")
		if err != nil {
			return result, err
		}
		if !reflect.DeepEqual(request.OriginalRules, fresh) {
			return result, errors.New("species-only profile rules/schema/counts changed since review; reload before preview")
		}
		plan, err := compilePlotProfilePresence(fresh)
		if err != nil {
			return result, err
		}
		needsLump := false
		for _, selector := range plan.selectors {
			needsLump = needsLump || selector.table == "Lump"
		}
		if needsLump && request.ProjectLump == nil {
			return result, errors.New("species-only profile requires explicit project-local Lump review before preview")
		}
		lumpRelation := ""
		if request.ProjectLump != nil {
			lumpTable := job.selection.Project + "_Lump"
			lump, err := readSQLiteStorageRows(ctx, tx, "project", lumpTable, "", nil, "")
			if err != nil {
				return result, err
			}
			if !reflect.DeepEqual(*request.ProjectLump, lump) {
				return result, errors.New("species-only project Lump definitions/schema changed since review; review again")
			}
			lumpRelation = `"project".` + quoteHeaderIdentifier(lumpTable)
		}
		if observe != nil {
			if err := observe(job, tx); err != nil {
				return result, err
			}
		}
		presence, err := executePlotProfilePresence(ctx, tx, plan, lumpRelation)
		if err != nil {
			return result, err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := profileOwnedFiles(job); err != nil {
			return result, err
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return projectPlotProfilePresencePreview{
			Project: job.selection.Project, Table: table, SU: job.selection.SU,
			TotalPlots: presence.totalPlots, PlotNumbers: presence.plotNumbers,
		}, nil
	})
}
