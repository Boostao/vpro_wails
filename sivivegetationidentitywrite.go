package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"
)

const siviIdentityHistoryTable = "__VPRO_SIVIIdentityHistory"
const siviIdentityHistorySQL = `CREATE TABLE "__VPRO_SIVIIdentityHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`
const siviIdentityLedger = "__VPRO_ChildIdentity"

type siviIdentityHistory struct {
	Project, Plot   string
	Extended        bool
	Before          ProjectMetadataTable
	Edits           []siviHeightEdit
	Committed       ProjectMetadataTable
	ReturnOccupants []siviIdentityOccupant
}

func withSIVIIdentityWriter[T any](ctx context.Context, service *ContextService, contextID, plot, operation string,
	apply func(*PlotService, *sqliteContext, *sql.Tx) (*T, bool, error)) (*T, error) {
	return withContextPlotRequest(ctx, service, contextID, func(plots *PlotService) (*T, error) {
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		var result *T
		committed := false
		err := owner.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return err
			}
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			if err := siviHeightParents(ctx, tx, owner.selection.Project, plot); err != nil {
				return err
			}
			var changed bool
			result, changed, err = apply(plots, owner, tx)
			if err != nil {
				return err
			}
			if !changed {
				return ctx.Err()
			}
			if err := owner.validateMetadataWriterFiles(); err != nil {
				return err
			}
			if err := siviHeightContextParent(ctx, owner, plot); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			committed = true
			return nil
		})
		if err != nil {
			if committed {
				return nil, fmt.Errorf("SIVI identity %s committed but cleanup failed; reload before retrying: %w", operation, err)
			}
			return nil, errors.Join(err, ctx.Err())
		}
		return result, nil
	})
}

func (s *ContextService) writeSIVIIdentities(ctx context.Context, contextID, plot string, extended bool, original []siviVegetationProjection, edits []siviHeightEdit) (*siviHeightWriteResult, error) {
	return withSIVIIdentityWriter(ctx, s, contextID, plot, "edit",
		func(_ *PlotService, owner *sqliteContext, tx *sql.Tx) (*siviHeightWriteResult, bool, error) {
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			project := owner.selection.Project
			veg := before[project+"_Veg"]
			observed, err := projectSIVIVegetation(ctx, plot, extended, veg)
			if err != nil {
				return nil, false, err
			}
			if !reflect.DeepEqual(original, observed) {
				return nil, false, errors.New("SIVI original source rows changed; cancel and reload")
			}
			reserved, err := siviIdentityReservedIDs(project, before)
			if err != nil {
				return nil, false, err
			}
			assignments, err := planSIVIIdentityEdits(ctx, plot, extended, veg, edits, reserved)
			if err != nil {
				return nil, false, err
			}
			result := &siviHeightWriteResult{}
			if len(assignments) == 0 {
				return result, false, nil
			}
			planned, err := applySIVIIdentityPlan(veg, assignments)
			if err != nil {
				return nil, false, err
			}
			event := siviIdentityHistory{Project: project, Plot: plot, Extended: extended,
				Before:    ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{}},
				Committed: ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{}}, Edits: []siviHeightEdit{}}
			event.ReturnOccupants, err = siviIdentityReturnOccupants(veg, assignments)
			if err != nil {
				return nil, false, err
			}
			owned := map[string]bool{}
			for _, assignment := range assignments {
				owned[assignment.RowID] = true
				for _, edit := range edits {
					if edit.RowID == assignment.RowID {
						event.Edits = append(event.Edits, edit)
					}
				}
			}
			for i, row := range veg.Rows {
				if owned[row.RowID] {
					event.Before.Rows = append(event.Before.Rows, row)
					event.Committed.Rows = append(event.Committed.Rows, planned.Rows[i])
				}
			}
			baseline, ledger, err := reserveSIVIIdentityPlan(ctx, tx, project, before, assignments)
			if err != nil {
				return nil, false, err
			}
			if err := writeSIVIIdentityAssignments(ctx, tx, project, plot, assignments); err != nil {
				return nil, false, err
			}
			expected := map[string]ProjectMetadataTable{project + "_Veg": planned, siviIdentityLedger: ledger}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(baseline, after, expected, ""); err != nil {
				return nil, false, fmt.Errorf("SIVI identity transaction differs from its complete unaudited plan: %w", err)
			}
			proposal, err := json.Marshal(event)
			if err != nil {
				return nil, false, err
			}
			if err := appendTechnicalProvenance(ctx, tx, siviIdentityHistoryTable, siviIdentityHistorySQL, string(proposal), "SIVI identity"); err != nil {
				return nil, false, err
			}
			var id int64
			if err := tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM "`+siviIdentityHistoryTable+`"`).Scan(&id); err != nil {
				return nil, false, err
			}
			after, err = environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(baseline, after, expected, siviIdentityHistoryTable); err != nil {
				return nil, false, err
			}
			result.ChangedCells, result.HistoryID = len(assignments), strconv.FormatInt(id, 10)
			return result, true, nil
		})
}

func writeSIVIIdentityAssignments(ctx context.Context, tx *sql.Tx, project, plot string, assignments []siviHeightAssignment) error {
	table := quoteHeaderIdentifier(project + "_Veg")
	for _, assignment := range assignments {
		if assignment.After.Storage == "integer" {
			var collision bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE ID=? AND rowid<>?)`,
				assignment.Value, assignment.RowID).Scan(&collision); err != nil {
				return err
			}
			if collision {
				return errors.New("SIVI new ID collides with physical SQLite identity storage")
			}
		}
		if err := updateSIVIPhysicalIdentity(ctx, tx, project, plot, assignment); err != nil {
			return err
		}
	}
	return nil
}

func updateSIVIPhysicalIdentity(ctx context.Context, tx *sql.Tx, project, plot string, assignment siviHeightAssignment) error {
	old, err := metadataCellValue(assignment.Before)
	if err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(project+"_Veg")+
		` SET ID=? WHERE rowid=? AND typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ? AND typeof(ID)=? AND ID IS ?`,
		assignment.Value, assignment.RowID, plot, assignment.Before.Storage, old)
	if err != nil {
		return err
	}
	if count, err := updated.RowsAffected(); err != nil || count != 1 {
		return errors.Join(err, errors.New("SIVI ID assignment did not affect exactly one original physical row"))
	}
	return nil
}

func reserveSIVIIdentityPlan(ctx context.Context, tx *sql.Tx, project string, before map[string]ProjectMetadataTable, assignments []siviHeightAssignment) (map[string]ProjectMetadataTable, ProjectMetadataTable, error) {
	baseline := before
	ledger, present := before[siviIdentityLedger]
	if !present {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE "__VPRO_ChildIdentity" (
			"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID"))`); err != nil {
			return nil, ProjectMetadataTable{}, err
		}
		created, err := environmentSiteUnitTables(ctx, tx)
		if err != nil {
			return nil, ProjectMetadataTable{}, err
		}
		if err := verifySiteUnitTransferTables(before, created, nil, siviIdentityLedger); err != nil {
			return nil, ProjectMetadataTable{}, err
		}
		ledger, baseline = created[siviIdentityLedger], created
		if len(ledger.Rows) != 0 {
			return nil, ProjectMetadataTable{}, errors.New("new SIVI identity reservation ledger is not empty")
		}
	}
	if !reflect.DeepEqual(ledger.Columns, []ProjectMetadataColumn{{Name: "ChildTable", DeclaredType: "TEXT"}, {Name: "ID", DeclaredType: "INTEGER"}}) {
		return nil, ProjectMetadataTable{}, errors.New("SIVI identity reservation schema differs from the existing child ledger")
	}
	expected := ProjectMetadataTable{Columns: ledger.Columns, Rows: append([]ProjectMetadataRow{}, ledger.Rows...)}
	table := quoteHeaderIdentifier(project + "_Veg")
	ids := map[string]bool{}
	for _, assignment := range assignments {
		for _, cell := range []ProjectMetadataCell{assignment.Before, assignment.After} {
			if cell.Storage == "integer" && cell.Integer != nil {
				ids[*cell.Integer] = true
			}
		}
	}
	for _, row := range ledger.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == table && row.Cells[1].Integer != nil {
			delete(ids, *row.Cells[1].Integer)
		}
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	for _, id := range ordered {
		inserted, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" (ChildTable,ID) VALUES (?,?)`, table, id)
		if err != nil {
			return nil, ProjectMetadataTable{}, err
		}
		if count, err := inserted.RowsAffected(); err != nil || count != 1 {
			return nil, ProjectMetadataTable{}, errors.Join(err, errors.New("SIVI identity reservation did not insert exactly one entry"))
		}
		rowID, err := inserted.LastInsertId()
		if err != nil {
			return nil, ProjectMetadataTable{}, err
		}
		owner, number := table, id
		expected.Rows = append(expected.Rows, ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10), Cells: []ProjectMetadataCell{
			{Storage: "text", Text: &owner}, {Storage: "integer", Integer: &number},
		}})
	}
	sort.Slice(expected.Rows, func(i, j int) bool {
		a, _ := strconv.ParseInt(expected.Rows[i].RowID, 10, 64)
		b, _ := strconv.ParseInt(expected.Rows[j].RowID, 10, 64)
		return a < b
	})
	return baseline, expected, nil
}

func (s *ContextService) restoreSIVIIdentities(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if action != AuditRestoreCancel && action != AuditRestoreRetain && action != AuditRestorePrune {
		return nil, errors.New("SIVI identity restoration requires cancel, retain or prune")
	}
	if action == AuditRestoreCancel {
		return &AuditRestoreResult{Cancelled: true}, nil
	}
	ids, err := auditRowIDs([]string{historyID})
	if err != nil {
		return nil, err
	}
	return withSIVIIdentityWriter(ctx, s, contextID, plot, "restoration",
		func(_ *PlotService, owner *sqliteContext, tx *sql.Tx) (*AuditRestoreResult, bool, error) {
			if err := verifyTechnicalProvenance(ctx, tx, siviIdentityHistoryTable, siviIdentityHistorySQL, "SIVI identity"); err != nil {
				return nil, false, err
			}
			var proposal string
			var restored *string
			if err := tx.QueryRowContext(ctx, `SELECT Proposal,Restored FROM "`+siviIdentityHistoryTable+`" WHERE ID=?`, ids[0]).Scan(&proposal, &restored); err != nil {
				return nil, false, err
			}
			if restored != nil {
				return nil, false, errors.New("SIVI identity history was already restored; do not replay")
			}
			var event siviIdentityHistory
			if err := decodeProfileLifecycleJSON([]byte(proposal), &event, "Project", "Plot", "Extended", "Before", "Edits", "Committed", "ReturnOccupants"); err != nil {
				return nil, false, err
			}
			project := owner.selection.Project
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			veg := before[project+"_Veg"]
			assignments, err := planSIVIIdentityRestoration(ctx, project, plot, veg, event)
			if err != nil {
				return nil, false, err
			}
			planned, err := applySIVIIdentityPlan(veg, assignments)
			if err != nil {
				return nil, false, err
			}
			for _, assignment := range assignments {
				if err := updateSIVIPhysicalIdentity(ctx, tx, project, plot, assignment); err != nil {
					return nil, false, err
				}
			}
			when := time.Now().UTC().Format(time.RFC3339Nano)
			updated, err := tx.ExecContext(ctx, `UPDATE "`+siviIdentityHistoryTable+`" SET Restored=? WHERE ID=? AND Restored IS NULL`, when, ids[0])
			if err != nil {
				return nil, false, err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return nil, false, errors.Join(err, errors.New("SIVI identity restoration did not mark exactly one history event"))
			}
			history := before[siviIdentityHistoryTable]
			historyColumns, err := siteUnitTransferColumns(history, "ID", "Restored")
			if err != nil {
				return nil, false, err
			}
			history.Rows = append([]ProjectMetadataRow{}, history.Rows...)
			for i, row := range history.Rows {
				if row.Cells[historyColumns["ID"]].Integer != nil && *row.Cells[historyColumns["ID"]].Integer == strconv.FormatInt(ids[0], 10) {
					history.Rows[i].Cells = append([]ProjectMetadataCell{}, row.Cells...)
					history.Rows[i].Cells[historyColumns["Restored"]] = ProjectMetadataCell{Storage: "text", Text: &when}
				}
			}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return nil, false, err
			}
			if err := verifySiteUnitTransferTables(before, after, map[string]ProjectMetadataTable{
				project + "_Veg": planned, siviIdentityHistoryTable: history,
			}, ""); err != nil {
				return nil, false, err
			}
			// Neither action prunes source audits: ID was explicitly excluded.
			return &AuditRestoreResult{RestoredRows: len(assignments)}, true, nil
		})
}

func planSIVIIdentityRestoration(ctx context.Context, project, plot string, veg ProjectMetadataTable, event siviIdentityHistory) ([]siviHeightAssignment, error) {
	if event.Project != project || event.Plot != plot || len(event.Before.Rows) == 0 ||
		!reflect.DeepEqual(event.Before.Columns, veg.Columns) {
		return nil, errors.New("SIVI identity history belongs to another owner or changed schema")
	}
	assignments, err := planSIVIIdentityEdits(ctx, plot, event.Extended, event.Before, event.Edits, nil)
	if err != nil || len(assignments) != len(event.Before.Rows) {
		return nil, errors.Join(err, errors.New("SIVI identity history has incomplete or repeated original assignments"))
	}
	committed, err := applySIVIIdentityPlan(event.Before, assignments)
	if err != nil || !reflect.DeepEqual(committed, event.Committed) {
		return nil, errors.Join(err, errors.New("SIVI identity history changed non-ID fields or committed values"))
	}
	if _, err := siteUnitTransferColumns(veg, "ID"); err != nil {
		return nil, err
	}
	for _, row := range committed.Rows {
		matches := false
		for _, current := range veg.Rows {
			if current.RowID == row.RowID {
				matches = reflect.DeepEqual(row, current)
				break
			}
		}
		if !matches {
			return nil, errors.New("SIVI physical row changed since the identity edit; no restoration or pruning")
		}
	}
	occupants, err := siviIdentityReturnOccupants(veg, assignments)
	if err != nil || !reflect.DeepEqual(occupants, event.ReturnOccupants) {
		return nil, errors.Join(err, errors.New("SIVI original ID occupancy changed; historical duplicates cannot be safely restored"))
	}
	reversed := make([]siviHeightAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		value, err := metadataCellValue(assignment.Before)
		if err != nil {
			return nil, err
		}
		reversed = append(reversed, siviHeightAssignment{
			assignment.RowID, "ID", cloneSiteUnitCell(assignment.After), cloneSiteUnitCell(assignment.Before), value,
		})
	}
	return reversed, ctx.Err()
}
