package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

func PreviewProjectPlotProfilePresence(s *vprocore.ContextService, ctx context.Context, contextID string, request ProjectPlotProfileRunRequest) (projectPlotProfilePresencePreview, error) {
	return PreviewProjectPlotProfilePresenceObserved(s, ctx, contextID, request, nil)
}

func PreviewProjectPlotProfilePresenceObserved(s *vprocore.ContextService, ctx context.Context, contextID string, request ProjectPlotProfileRunRequest,
	observe func(*vprocore.SqliteContext, *sql.Tx) error) (projectPlotProfilePresencePreview, error) {
	if ctx == nil {
		return projectPlotProfilePresencePreview{}, errors.New("species-only profile requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return projectPlotProfilePresencePreview{}, err
	}
	if request.Subvarieties {
		return projectPlotProfilePresencePreview{}, errors.New("species-only subvariety preparation remains unavailable; no global Lump state was inherited")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (result projectPlotProfilePresencePreview, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return result, err
		}
		job, err := vprocore.NewSQLiteContext(ctx, owner.Selection, plots.Projects.SupportPaths, owner.Profile)
		if err != nil {
			return result, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, job.Close())
			if resultErr != nil {
				result = projectPlotProfilePresencePreview{}
			}
		}()
		if err := registerProfileFunctions(job.Conn); err != nil {
			return result, err
		}
		tx, err := job.BeginReadSnapshot(ctx)
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
		role, table, err := job.ProfileLocation()
		if err != nil {
			return result, err
		}
		fresh, err := vprocore.ReadSQLiteStorageRows(ctx, tx, role, table, "", nil, "Order")
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
			lumpTable := job.Selection.Project + "_Lump"
			lump, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", lumpTable, "", nil, "")
			if err != nil {
				return result, err
			}
			if !reflect.DeepEqual(*request.ProjectLump, lump) {
				return result, errors.New("species-only project Lump definitions/schema changed since review; review again")
			}
			lumpRelation = `"project".` + vprocore.QuoteHeaderIdentifier(lumpTable)
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
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := vprocore.ProfileOwnedFiles(job); err != nil {
			return result, err
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return projectPlotProfilePresencePreview{
			Project: job.Selection.Project, Table: table, SU: job.Selection.SU,
			TotalPlots: presence.totalPlots, PlotNumbers: presence.plotNumbers,
		}, nil
	})
}
