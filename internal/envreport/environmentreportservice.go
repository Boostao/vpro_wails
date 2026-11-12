package envreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type LongEnvironmentRequest struct {
	Title string `json:"title"`
}

func (request *LongEnvironmentRequest) UnmarshalJSON(data []byte) error {
	type plain LongEnvironmentRequest
	var value plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &value, "title"); err != nil {
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
	ContextID   string                       `json:"contextId"`
	ProjectPath string                       `json:"projectPath"`
	SUPath      string                       `json:"suPath"`
	Report      reportcore.EnvironmentReport `json:"report"`
}

func GetLongEnvironmentOptions(s *vprocore.ContextService, ctx context.Context, contextID string) (LongEnvironmentOptions, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (LongEnvironmentOptions, error) {
		title, err := ReadLongEnvironmentPreference(plots.Projects.Preferences, ctx)
		if err != nil {
			return LongEnvironmentOptions{}, err
		}
		if strings.ContainsRune(title, 0) {
			return LongEnvironmentOptions{}, errors.New("ReportOptions.LEReportTitle contains NUL; correct the configuration explicitly")
		}
		return LongEnvironmentOptions{contextID, title}, nil
	})
}

func PreviewLongEnvironment(s *vprocore.ContextService, ctx context.Context, contextID string, request LongEnvironmentRequest) (LongEnvironmentPreview, error) {
	if err := validateLongEnvironmentTitle(request.Title); err != nil {
		return LongEnvironmentPreview{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (result LongEnvironmentPreview, resultErr error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.SU == "None" {
			return result, errors.New("Long Environment requires an explicitly selected SU; project-only plots are not an implicit scope")
		}
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return result, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return result, err
		}
		tx, err := owner.BeginReadSnapshot(ctx)
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
		tables := make([]vprocore.ProjectMetadataTable, 4)
		for i, source := range []struct{ role, table string }{
			{"project", owner.Selection.Project + "_Env"},
			{"project", owner.Selection.Project + "_Admin"},
			{"su", owner.Selection.SU + "_SU"},
			{"VLists", "MasterSiteUnitList"},
		} {
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
				`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
				return result, err
			}
			if count != 1 {
				return result, fmt.Errorf("Long Environment requires original physical table %s.%s", source.role, source.table)
			}
			tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
			if err != nil {
				return result, err
			}
		}
		report, err := reportcore.PlanLongEnvironment(ctx, owner.Selection.Project, owner.Selection.SU, request.Title, tables[0], tables[1], tables[2], tables[3])
		if err != nil {
			return result, err
		}
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return result, err
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return LongEnvironmentPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, report}, nil
	})
}
