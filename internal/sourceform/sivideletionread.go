package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

type SiviDeletionOriginal struct {
	ContextID string                           `json:"contextId"`
	Project   string                           `json:"project"`
	Plot      string                           `json:"plot"`
	Form      string                           `json:"form"`
	Columns   []vprocore.ProjectMetadataColumn `json:"columns"`
	Original  vprocore.ProjectMetadataRow      `json:"original"`
}

func SiviDeletionSource(ctx context.Context, project, plot, form, rowID string, veg vprocore.ProjectMetadataTable) (*SiviDeletionOriginal, error) {
	if err := ValidateSIVIDeletionScope(project, plot, form, rowID); err != nil {
		return nil, err
	}
	columns, err := vprocore.SiteUnitTransferColumns(veg, SiviCreationColumns...)
	if err != nil || len(columns) != len(SiviCreationColumns) {
		return nil, errors.Join(err, errors.New("SIVI deletion requires the complete all44 physical schema"))
	}
	groups, err := ProjectSIVIVegetation(ctx, plot, form == "SubVegA-SIVI", veg)
	if err != nil {
		return nil, err
	}
	member := false
	for _, group := range groups {
		if group.Form == form {
			for _, row := range group.Rows {
				member = member || row.RowID == rowID
			}
		}
	}
	if !member {
		return nil, errors.New("SIVI deletion row is unavailable in the exact source form and parent")
	}
	for _, row := range veg.Rows {
		if row.RowID != rowID {
			continue
		}
		original := &SiviDeletionOriginal{Project: project, Plot: plot, Form: form, Columns: veg.Columns, Original: row}
		id, err := ValidateSIVIDeletionOriginal(*original)
		if err != nil {
			return nil, err
		}
		occupants := 0
		for _, candidate := range veg.Rows {
			parent := candidate.Cells[columns["PlotNumber"]]
			if parent.Text != nil && *parent.Text == plot &&
				SiviIdentityCellMatches(candidate.Cells[columns["ID"]], strconv.FormatInt(id, 10)) {
				occupants++
			}
		}
		if occupants != 1 {
			return nil, errors.New("SIVI deletion logical ID is ambiguous in its parent; source audits cannot distinguish rows")
		}
		return original, nil
	}
	return nil, fmt.Errorf("SIVI deletion physical row %s disappeared", rowID)
}

func ReadSIVIDeletionOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot, form, rowID string) (*SiviDeletionOriginal, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion original requires an owned context")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviDeletionOriginal, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SiviDeletionOriginal, error) {
			if err := SiviHeightContextParent(ctx, owner, plot); err != nil {
				return nil, err
			}
			if err := SiviSpeciesReadParents(ctx, tx, owner.Selection.Project, plot); err != nil {
				return nil, err
			}
			veg, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", owner.Selection.Project+"_Veg", "", nil, "")
			if err != nil {
				return nil, err
			}
			original, err := SiviDeletionSource(ctx, owner.Selection.Project, plot, form, rowID, veg)
			if err != nil {
				return nil, err
			}
			original.ContextID = contextID
			return original, SiviHeightContextParent(ctx, owner, plot)
		})
	})
}

func LookupSIVIDeletionReceipt(s *vprocore.ContextService, ctx context.Context, contextID string, request SiviDeletionRequest) (*SiviDeletionResult, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("SIVI deletion receipt requires an owned context")
	}
	if err := ValidateSIVIDeletionRequest(request); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviDeletionResult, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SiviDeletionResult, error) {
			if request.Project != owner.Selection.Project {
				return nil, errors.New("SIVI deletion receipt belongs to another project")
			}
			if err := SiviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			if err := SiviSpeciesReadParents(ctx, tx, request.Project, request.Plot); err != nil {
				return nil, err
			}
			schema, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", "sqlite_master", "", nil, "")
			if err != nil {
				return nil, err
			}
			present := map[string]bool{}
			for _, row := range schema.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" && row.Cells[1].Text != nil {
					present[*row.Cells[1].Text] = true
				}
			}
			if !present[SiviDeletionHistoryTable] {
				return nil, nil
			}
			tables := map[string]vprocore.ProjectMetadataTable{}
			for _, name := range []string{SiviDeletionHistoryTable, request.Project + "_Veg", request.Project + "_Audit", SiviIdentityLedger} {
				if !present[name] {
					continue
				}
				table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "project", name, "", nil, "")
				if err != nil {
					return nil, err
				}
				tables[name] = table
			}
			history, err := readSIVIDeletionReplay(ctx, request, tables)
			if err != nil || history == nil {
				return nil, err
			}
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return nil, err
			}
			if err := SiviHeightContextParent(ctx, owner, request.Plot); err != nil {
				return nil, err
			}
			return SiviDeletionReceipt(*history, contextID, false), nil
		})
	})
}

func sameSIVIDeletionRequest(a, b SiviDeletionRequest) bool {
	a.ContextID, b.ContextID = "", ""
	return reflect.DeepEqual(a, b)
}
