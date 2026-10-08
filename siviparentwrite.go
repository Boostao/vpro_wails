package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

const siviParentHistoryTable = "__VPRO_SIVIParentHistory"
const siviParentHistorySQL = `CREATE TABLE "__VPRO_SIVIParentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

type siviParentDirectEdits struct {
	Scalars, Options, Text, Categorical []siviParentScalarEdit
}

type SIVIParentWriteResult struct {
	ChangedCells int
	HistoryID    string
}

type siviParentWriteResult = SIVIParentWriteResult

type siviParentHistoryChange struct {
	Table, RowID, Column string
	Before, After        ProjectMetadataCell
	Audit                AuditEntry
}

type siviParentHistory struct {
	Original, Committed *siviParentProjection
	Changes             []siviParentHistoryChange
}

type siviParentHistoryDomain struct {
	table, schema, label string
	assignments          func(context.Context, siviParentHistory, string, string) ([]siviParentScalarAssignment, error)
}

var siviParentDirectHistory = siviParentHistoryDomain{
	siviParentHistoryTable, siviParentHistorySQL, "SIVI parent", siviParentHistoryAssignments,
}

func planSIVIParentDirectEdits(ctx context.Context, original *siviParentProjection, edits siviParentDirectEdits) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 ||
		len(edits.Scalars)+len(edits.Options)+len(edits.Text)+len(edits.Categorical) == 0 {
		return nil, errors.New("SIVI parent writing requires one physical pair and explicit direct-field edits")
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	assignments := []siviParentScalarAssignment{}
	for _, domain := range []struct {
		edits []siviParentScalarEdit
		plan  func(context.Context, string, string, string, ProjectMetadataTable, ProjectMetadataTable, []siviParentScalarEdit) ([]siviParentScalarAssignment, error)
	}{
		{edits.Scalars, planSIVIParentScalars}, {edits.Options, planSIVIParentOptions},
		{edits.Text, planSIVIParentText}, {edits.Categorical, planSIVIParentCategorical},
	} {
		if len(domain.edits) == 0 {
			continue
		}
		planned, err := domain.plan(ctx, original.ContextID, original.Project, original.Plot, env, admin, domain.edits)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, planned...)
	}
	return assignments, nil
}

func siviParentWriterSU(ctx context.Context, conn *sql.Conn, owner *sqliteContext) (string, error) {
	if owner.selection.SU == "None" {
		return "", nil
	}
	if owner.attachments["su"] == owner.attachments["project"] {
		return "main", nil
	}
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "sivi_su"`, sqliteFileURI(owner.attachments["su"], "ro")); err != nil {
		return "", fmt.Errorf("SIVI selected SU attachment unavailable: %w", err)
	}
	return "sivi_su", nil
}

func readSIVIParentForWrite(ctx context.Context, tx *sql.Tx, owner *sqliteContext, suAlias, contextID, plot string) (*siviParentProjection, error) {
	project := owner.selection.Project
	join, err := readSIVIParentJoin(ctx, tx, "main", contextID, project, plot)
	if err != nil {
		return nil, err
	}
	if !join.Verified {
		return nil, fmt.Errorf("SIVI parent writing requires certified physical source membership: %s", join.Diagnostic)
	}
	if suAlias != "" {
		table := owner.selection.SU + "_SU"
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(suAlias)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, errors.New("SIVI parent writing requires the selected physical SU table")
		}
		if err := validateSIVIPhysicalSchema(ctx, tx, suAlias, table, "selected SU membership"); err != nil {
			return nil, err
		}
		var member bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+quoteHeaderIdentifier(suAlias)+"."+
			quoteHeaderIdentifier(table)+` WHERE typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ?)`, plot).
			Scan(&member); err != nil {
			return nil, err
		}
		if !member {
			return nil, errors.New("SIVI parent is outside the selected SU context")
		}
	}
	env, err := readSQLiteStorageRows(ctx, tx, "main", project+"_Env", "PlotNumber", &plot, "")
	if err != nil {
		return nil, err
	}
	admin, err := readSQLiteStorageRows(ctx, tx, "main", project+"_Admin", "Plot", &plot, "")
	if err != nil {
		return nil, err
	}
	parent, err := projectSIVIParent(ctx, contextID, project, plot, env, admin)
	if err != nil {
		return nil, err
	}
	if len(parent.Rows) != 1 || parent.Rows[0].Env.RowID != join.EnvRowIDs[0] || parent.Rows[0].Admin.RowID != join.AdminRowIDs[0] {
		return nil, errors.New("SIVI projection differs from the certified physical source pair")
	}
	return parent, nil
}

func withSIVIParentWriteTransaction(ctx context.Context, owner *sqliteContext, purpose string, operation func(*sql.Tx, string) error) error {
	committed := false
	err := owner.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
		suAlias, err := siviParentWriterSU(ctx, conn, owner)
		if err != nil {
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
		if err := operation(tx, suAlias); err != nil {
			return err
		}
		if err := owner.validateMetadataWriterFiles(); err != nil {
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
			return fmt.Errorf("SIVI %s committed but cleanup failed; reload before retrying: %w", purpose, err)
		}
		return errors.Join(err, ctx.Err())
	}
	return nil
}

func (s *ContextService) writeSIVIParentDirect(ctx context.Context, contextID, plot string, original *siviParentProjection, edits siviParentDirectEdits) (*siviParentWriteResult, error) {
	return s.writeSIVIParentPlanned(ctx, contextID, plot, original, siviParentDirectHistory, func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
		return planSIVIParentDirectEdits(ctx, observed, edits)
	})
}

func (s *ContextService) writeSIVIParentPlanned(ctx context.Context, contextID, plot string, original *siviParentProjection, history siviParentHistoryDomain, plan func(*siviParentProjection) ([]siviParentScalarAssignment, error)) (*siviParentWriteResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviParentWriteResult, error) {
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		if err := validateChildPhysicalText("SIVI parent audit user", plots.currentUser, 100); err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		result := &siviParentWriteResult{}
		err := withSIVIParentWriteTransaction(ctx, owner, "parent edit", func(tx *sql.Tx, suAlias string) error {
			observed, err := readSIVIParentForWrite(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(original, observed) {
				return errors.New("SIVI original physical parents changed; undo and reload before writing")
			}
			assignments, err := plan(observed)
			if err != nil {
				return err
			}
			if len(assignments) == 0 {
				return ctx.Err()
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "main", observed.Project+"_Audit", "parent audits"); err != nil {
				return err
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			expected, changes, records, err := applySIVIParentAssignments(ctx, tx, observed, before, assignments, plots.currentUser, plots.auditStrength)
			if err != nil {
				return err
			}
			expected[observed.Project+"_Audit"], err = appendSIVIPlannedAudits(before[observed.Project+"_Audit"], records)
			if err != nil {
				return err
			}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("SIVI parent transaction differs from its complete plan: %w", err)
			}
			fresh, err := readSIVIParentForWrite(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			if len(changes) != 0 {
				proposal, err := json.Marshal(siviParentHistory{Original: observed, Committed: fresh, Changes: changes})
				if err != nil {
					return err
				}
				if err := appendTechnicalProvenance(ctx, tx, history.table, history.schema, string(proposal), history.label); err != nil {
					return err
				}
				var id int64
				if err := tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM `+quoteHeaderIdentifier(history.table)).Scan(&id); err != nil {
					return err
				}
				result.HistoryID = strconv.FormatInt(id, 10)
				after, err = environmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				if err := verifySiteUnitTransferTables(before, after, expected, history.table); err != nil {
					return fmt.Errorf("SIVI parent provenance differs from its complete plan: %w", err)
				}
			}
			result.ChangedCells = len(assignments)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func applySIVIParentAssignments(ctx context.Context, tx *sql.Tx, original *siviParentProjection, before map[string]ProjectMetadataTable, assignments []siviParentScalarAssignment, user string, strength int) (map[string]ProjectMetadataTable, []siviParentHistoryChange, []AuditEntry, error) {
	expected := map[string]ProjectMetadataTable{}
	changes, records := []siviParentHistoryChange{}, []AuditEntry{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, assignment := range assignments {
		suffix, identity := "Env", "PlotNumber"
		if assignment.Table == original.AdminTable {
			suffix, identity = "Admin", "Plot"
		}
		prior, err := metadataCellValue(assignment.Before)
		if err != nil {
			return nil, nil, nil, err
		}
		if assignment.Before.Storage == "blob" {
			return nil, nil, nil, errors.New("historical SIVI parent BLOB replacement has no source audit representation; omit it unchanged")
		}
		changed, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(assignment.Table)+
			` SET `+quoteHeaderIdentifier(assignment.Column)+`=? WHERE rowid=? AND typeof(`+quoteHeaderIdentifier(identity)+
			`)='text' AND `+quoteHeaderIdentifier(identity)+` COLLATE BINARY IS ?`, assignment.Value, assignment.RowID, original.Plot)
		if err != nil {
			return nil, nil, nil, err
		}
		if count, err := changed.RowsAffected(); err != nil || count != 1 {
			return nil, nil, nil, errors.Join(err, errors.New("SIVI parent edit did not affect exactly one physical row"))
		}
		planned, present := expected[assignment.Table]
		if !present {
			planned = before[assignment.Table]
			planned.Rows = append([]ProjectMetadataRow{}, planned.Rows...)
		}
		columns, err := siteUnitTransferColumns(planned, assignment.Column)
		if err != nil {
			return nil, nil, nil, err
		}
		for i, row := range planned.Rows {
			if row.RowID == assignment.RowID {
				planned.Rows[i].Cells = append([]ProjectMetadataCell{}, row.Cells...)
				planned.Rows[i].Cells[columns[assignment.Column]] = cloneSiteUnitCell(assignment.After)
			}
		}
		expected[assignment.Table] = planned
		if !auditHeaderChange(prior, assignment.Value, strength) {
			continue
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO `+quoteHeaderIdentifier(original.Project+"_Audit")+
			` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,NULL)`,
			original.Project, user, original.Plot, "_"+suffix, assignment.Column, when, headerAuditValue(prior), headerAuditValue(assignment.Value))
		if err != nil {
			return nil, nil, nil, err
		}
		id, err := inserted.LastInsertId()
		if err != nil {
			return nil, nil, nil, err
		}
		actual, err := selectedAuditEntries(tx, original.Project, original.Plot, []int64{id})
		if err != nil {
			return nil, nil, nil, err
		}
		record := actual[0]
		if record.Table != "_"+suffix || record.ID != nil || record.EditField != assignment.Column ||
			record.User != user || record.EditWhen != when || record.Restore || record.Flag ||
			!metadataAuditTextEqual(record.BeforeEdit, prior) || !metadataAuditTextEqual(record.AfterEdit, assignment.Value) {
			return nil, nil, nil, errors.New("SIVI parent inserted audit differs from its typed assignment")
		}
		changes = append(changes, siviParentHistoryChange{assignment.Table, assignment.RowID, assignment.Column,
			cloneSiteUnitCell(assignment.Before), cloneSiteUnitCell(assignment.After), record})
		records = append(records, record)
	}
	return expected, changes, records, nil
}
