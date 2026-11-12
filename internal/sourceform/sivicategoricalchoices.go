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

type siviCategoricalChoices struct {
	ContextID, Project, ControlID, ControlName, Binding string
	SourceType, Alias, Table, ListName, Membership      string
	Ordering                                            string
	SourceDistinctRow                                   bool
	Definitions                                         vprocore.ProjectMetadataTable
	Values                                              []vprocore.ProjectMetadataCell
}

func siviCategoricalSource(binding string) (*siviCategoricalChoices, error) {
	result := &siviCategoricalChoices{
		Binding: binding, SourceType: "Table/Query", Alias: "VLists", Table: "USysTableOfLists",
		Membership: "literal-binary-list-name", Ordering: "sqlite-ItemOrder-then-physical-rowid",
		Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Values:      []vprocore.ProjectMetadataCell{},
	}
	switch binding {
	case "SnowCoverregime":
		result.ControlName, result.ListName = "SnowCoverregime", "SnowCoverRegime"
	case "SV_RootZoneTexture":
		result.ControlName, result.ListName, result.SourceDistinctRow = "RootZoneTexture", "SoilTexture", true
	case "SV_AhorizonType":
		result.ControlName, result.SourceType = "AhorizonType", "Value List"
		result.Alias, result.Table, result.Membership, result.Ordering = "", "", "source-value-list", "source-order"
		for _, value := range []string{"Ah", "Ae", ""} {
			result.Values = append(result.Values, vprocore.ProjectMetadataCell{Storage: "text", Text: &value})
		}
	default:
		return nil, fmt.Errorf("SIVI categorical binding %q is outside the source scope", binding)
	}
	bindings, err := SiviParentSourceBindings()
	if err != nil {
		return nil, err
	}
	for _, source := range bindings {
		if source.Binding == binding && !source.Implicit {
			result.ControlID = source.ControlID
			return result, nil
		}
	}
	return nil, fmt.Errorf("SIVI categorical binding %q has no direct source control", binding)
}

func ReadSIVICategoricalChoices(s *vprocore.ContextService, ctx context.Context, contextID, binding string) (*siviCategoricalChoices, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviCategoricalChoices, error) {
		return readOwnedSIVICategoricalChoices(ctx, plots, contextID, binding)
	})
}

func readOwnedSIVICategoricalChoices(ctx context.Context, plots *vprocore.PlotService, contextID, binding string) (result *siviCategoricalChoices, resultErr error) {
	result, err := siviCategoricalSource(binding)
	if err != nil {
		return nil, err
	}
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
	result.ContextID, result.Project = contextID, owner.Selection.Project
	if result.SourceType == "Table/Query" {
		if err := tx.QueryRowContext(ctx, `SELECT name FROM VLists.sqlite_master
			WHERE type='table' AND name COLLATE NOCASE=?`, result.Table).Scan(&result.Table); err != nil {
			return nil, fmt.Errorf("SIVI categorical source requires physical VLists table: %w", err)
		}
		if err := validateSIVICategoricalSchema(ctx, tx, result.Alias, result.Table); err != nil {
			return nil, err
		}
		result.Definitions, err = vprocore.ReadSQLiteStorageRows(ctx, tx, result.Alias, result.Table, "ListName", &result.ListName, "ItemOrder")
		if err != nil {
			return nil, err
		}
		_, err = vprocore.SiteUnitTransferColumns(result.Definitions, "ListName", "Item", "ItemDescription", "ItemOrder")
		if err != nil {
			return nil, fmt.Errorf("SIVI categorical physical definitions: %w", err)
		}
		if len(result.Definitions.Rows) == 0 {
			return nil, fmt.Errorf("SIVI categorical list %q is unavailable", result.ListName)
		}
		for _, row := range result.Definitions.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for _, cell := range row.Cells {
				if _, err := vprocore.MetadataCellValue(cell); err != nil {
					return nil, fmt.Errorf("SIVI categorical physical row %s: %w", row.RowID, err)
				}
			}
		}
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

func validateSIVICategoricalSchema(ctx context.Context, tx *sql.Tx, alias, table string) error {
	return vprocore.ValidateSIVIPhysicalSchema(ctx, tx, alias, table, "categorical definitions")
}
