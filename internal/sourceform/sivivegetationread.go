package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func ReadSIVIVegetation(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool) ([]SiviVegetationProjection, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) ([]SiviVegetationProjection, error) {
		return readOwnedSIVIVegetation(ctx, plots, plot, extended)
	})
}

func readOwnedSIVIVegetation(ctx context.Context, plots *vprocore.PlotService, plot string, extended bool) (result []SiviVegetationProjection, resultErr error) {
	owner := plots.Projects.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return nil, err
	}
	defer owner.Mu.Unlock()
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return nil, err
	}
	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = nil
		}
	}()
	table := owner.Selection.Project + "_Veg"
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("SIVI requires original physical project table %q", table)
	}
	veg, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", table, "PlotNumber", &plot, "")
	if err != nil {
		return nil, err
	}
	result, err = ProjectSIVIVegetation(ctx, plot, extended, veg)
	if err != nil {
		return nil, err
	}
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
