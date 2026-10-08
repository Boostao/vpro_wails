package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
)

const siviHeightHistoryTable = "__VPRO_SIVIHeightHistory"
const siviHeightHistorySQL = `CREATE TABLE "__VPRO_SIVIHeightHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

type SIVIHeightWriteResult struct {
	ChangedCells int
	HistoryID    string
}

type siviHeightWriteResult = SIVIHeightWriteResult

type siviHeightHistoryChange struct {
	RowID, Column string
	Before, After ProjectMetadataCell
	Audit         AuditEntry
}

type siviHeightHistory struct {
	Project, Plot string
	Columns       []ProjectMetadataColumn
	Committed     []ProjectMetadataRow
	Changes       []siviHeightHistoryChange
}

type siviVegetationWritePolicy struct {
	Kind, HistoryTable, HistorySQL string
	Columns                        []string
	Plan                           func(context.Context, string, bool, ProjectMetadataTable, []siviHeightEdit) ([]siviHeightAssignment, error)
}

func siviHeightWritePolicy() siviVegetationWritePolicy {
	return siviVegetationWritePolicy{"height", siviHeightHistoryTable, siviHeightHistorySQL,
		[]string{"HeightA", "HeightB", "Height6"}, planSIVIHeightEdits}
}

func (s *ContextService) writeSIVIHeights(ctx context.Context, contextID, plot string, extended bool, original []siviVegetationProjection, edits []siviHeightEdit) (*siviHeightWriteResult, error) {
	return s.writeSIVIVegetationCells(ctx, contextID, plot, extended, original, edits, siviHeightWritePolicy())
}

func (s *ContextService) writeSIVIVegetationCells(ctx context.Context, contextID, plot string, extended bool, original []siviVegetationProjection, edits []siviHeightEdit, policy siviVegetationWritePolicy) (*siviHeightWriteResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*siviHeightWriteResult, error) {
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		if err := validateChildPhysicalText("SIVI audit user", plots.currentUser, 100); err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		result := &siviHeightWriteResult{}
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
			project := owner.selection.Project
			if err := siviHeightParents(ctx, tx, project, plot); err != nil {
				return err
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			veg := before[project+"_Veg"]
			observed, err := projectSIVIVegetation(ctx, plot, extended, veg)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(original, observed) {
				return errors.New("SIVI original source rows changed; cancel and reload")
			}
			assignments, err := policy.Plan(ctx, plot, extended, veg, edits)
			if err != nil {
				return err
			}
			if len(assignments) == 0 {
				return ctx.Err()
			}
			planned, changes, records, err := applySIVIHeightAssignments(ctx, tx, project, plot, veg, assignments, plots.currentUser, plots.auditStrength)
			if err != nil {
				return err
			}
			audit, err := appendSIVIPlannedAudits(before[project+"_Audit"], records)
			if err != nil {
				return err
			}
			expected := map[string]ProjectMetadataTable{project + "_Veg": planned, project + "_Audit": audit}
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("SIVI transaction differs from its complete plan: %w", err)
			}
			if len(changes) != 0 {
				history := siviHeightHistory{Project: project, Plot: plot, Columns: planned.Columns, Changes: changes}
				seen := map[string]bool{}
				for _, change := range changes {
					for _, row := range planned.Rows {
						if row.RowID == change.RowID && !seen[row.RowID] {
							history.Committed = append(history.Committed, row)
							seen[row.RowID] = true
						}
					}
				}
				proposal, err := json.Marshal(history)
				if err != nil {
					return err
				}
				if err := appendTechnicalProvenance(ctx, tx, policy.HistoryTable, policy.HistorySQL, string(proposal), "SIVI "+policy.Kind); err != nil {
					return err
				}
				var id int64
				if err := tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM `+quoteHeaderIdentifier(policy.HistoryTable)).Scan(&id); err != nil {
					return err
				}
				result.HistoryID = strconv.FormatInt(id, 10)
				after, err = environmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				if err := verifySiteUnitTransferTables(before, after, expected, policy.HistoryTable); err != nil {
					return fmt.Errorf("SIVI provenance transaction differs from its plan: %w", err)
				}
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
			result.ChangedCells = len(assignments)
			return nil
		})
		if err != nil {
			if committed {
				return nil, fmt.Errorf("SIVI %s edit committed but cleanup failed; reload before retrying: %w", policy.Kind, err)
			}
			return nil, errors.Join(err, ctx.Err())
		}
		return result, nil
	})
}

func siviHeightContextParent(ctx context.Context, owner *sqliteContext, plot string) error {
	var member bool
	if err := owner.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, plot).
		Scan(&member); err != nil || !member {
		return errors.Join(err, errors.New("SIVI parent is outside the selected context"))
	}
	return nil
}

func siviHeightParents(ctx context.Context, tx *sql.Tx, project, plot string) error {
	if err := childParent(tx, project, plot); err != nil {
		return err
	}
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		var count int
		column := quoteHeaderIdentifier(parent.column)
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(project+"_"+parent.table)+
			` WHERE typeof(`+column+`)='text' AND `+column+` COLLATE BINARY IS ?`, plot).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("SIVI editing requires unique original literal text parents")
		}
	}
	return nil
}

func applySIVIHeightAssignments(ctx context.Context, tx *sql.Tx, project, plot string, veg ProjectMetadataTable, assignments []siviHeightAssignment, user string, strength int) (ProjectMetadataTable, []siviHeightHistoryChange, []AuditEntry, error) {
	columns, err := siteUnitTransferColumns(veg, "ID", "Species", "PlotNumber")
	if err != nil {
		return ProjectMetadataTable{}, nil, nil, err
	}
	planned := ProjectMetadataTable{Columns: veg.Columns, Rows: append([]ProjectMetadataRow{}, veg.Rows...)}
	changes := []siviHeightHistoryChange{}
	records := []AuditEntry{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return ProjectMetadataTable{}, nil, nil, err
		}
		for i, row := range planned.Rows {
			if row.RowID != assignment.RowID {
				continue
			}
			idCell := row.Cells[columns["ID"]]
			if idCell.Storage != "integer" || idCell.Integer == nil {
				return ProjectMetadataTable{}, nil, nil, errors.New("SIVI writes require the original signed32 application ID")
			}
			id, err := strconv.ParseInt(*idCell.Integer, 10, 32)
			if err != nil || id < math.MinInt32 || id > math.MaxInt32 {
				return ProjectMetadataTable{}, nil, nil, errors.New("SIVI writes require the original signed32 application ID")
			}
			table := quoteHeaderIdentifier(project + "_Veg")
			if err := requireChildIdentity(tx, table, "Veg", plot, id); err != nil {
				return ProjectMetadataTable{}, nil, nil, err
			}
			before, err := metadataCellValue(assignment.Before)
			if err != nil {
				return ProjectMetadataTable{}, nil, nil, err
			}
			if assignment.Before.Storage == "blob" {
				return ProjectMetadataTable{}, nil, nil, errors.New("historical SIVI blob replacement requires separately defined audit restoration; omit it unchanged")
			}
			updated, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+quoteHeaderIdentifier(assignment.Column)+
				`=? WHERE rowid=? AND typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ? AND ID=?`,
				assignment.Value, assignment.RowID, plot, id)
			if err != nil {
				return ProjectMetadataTable{}, nil, nil, err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return ProjectMetadataTable{}, nil, nil, errors.Join(err, errors.New("SIVI height update did not affect exactly one physical row"))
			}
			planned.Rows[i].Cells = append([]ProjectMetadataCell{}, row.Cells...)
			planned.Rows[i].Cells[columns[assignment.Column]] = cloneSiteUnitCell(assignment.After)
			audits, err := auditChildFieldsTracked(tx, project, "Veg", plot, id,
				[]childField{{column: assignment.Column}}, []any{before}, []any{assignment.Value}, user, strength, when)
			if err != nil {
				return ProjectMetadataTable{}, nil, nil, err
			}
			for _, audit := range audits {
				ids, err := auditRowIDs([]string{audit.RowID})
				if err != nil {
					return ProjectMetadataTable{}, nil, nil, err
				}
				inserted, err := selectedAuditEntries(tx, project, plot, ids)
				if err != nil {
					return ProjectMetadataTable{}, nil, nil, err
				}
				record := inserted[0]
				if record.Table != "_Veg" || record.EditField != assignment.Column ||
					record.ID == nil || *record.ID != id || record.User != user || record.EditWhen != when ||
					record.Restore || record.Flag || !metadataAuditTextEqual(record.BeforeEdit, before) ||
					!metadataAuditTextEqual(record.AfterEdit, assignment.Value) {
					return ProjectMetadataTable{}, nil, nil, errors.New("SIVI inserted audit differs from its typed assignment")
				}
				changes = append(changes, siviHeightHistoryChange{assignment.RowID, assignment.Column,
					cloneSiteUnitCell(assignment.Before), cloneSiteUnitCell(assignment.After), record})
				records = append(records, record)
			}
		}
	}
	return planned, changes, records, nil
}

func appendSIVIPlannedAudits(original ProjectMetadataTable, records []AuditEntry) (ProjectMetadataTable, error) {
	columns, err := siteUnitTransferColumns(original, "Project", "User", "PlotNumber", "Table", "EditField",
		"EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID")
	if err != nil || len(columns) != 11 {
		return ProjectMetadataTable{}, errors.Join(err, errors.New("SIVI requires the original eleven-column audit schema"))
	}
	planned := ProjectMetadataTable{Columns: original.Columns, Rows: append([]ProjectMetadataRow{}, original.Rows...)}
	for _, record := range records {
		zero := "0"
		values := map[string]ProjectMetadataCell{
			"Project": {Storage: "text", Text: &record.Project}, "User": {Storage: "text", Text: &record.User},
			"PlotNumber": {Storage: "text", Text: &record.PlotNumber}, "Table": {Storage: "text", Text: &record.Table},
			"EditField": {Storage: "text", Text: &record.EditField}, "EditWhen": {Storage: "text", Text: &record.EditWhen},
			"BeforeEdit": {Storage: "null"}, "AfterEdit": {Storage: "null"},
			"Restore": {Storage: "integer", Integer: &zero}, "Flag": {Storage: "integer", Integer: &zero},
			"ID": {Storage: "null"},
		}
		if record.ID != nil {
			id := strconv.FormatInt(*record.ID, 10)
			values["ID"] = ProjectMetadataCell{Storage: "integer", Integer: &id}
		}
		if record.BeforeEdit != nil {
			values["BeforeEdit"] = ProjectMetadataCell{Storage: "text", Text: record.BeforeEdit}
		}
		if record.AfterEdit != nil {
			values["AfterEdit"] = ProjectMetadataCell{Storage: "text", Text: record.AfterEdit}
		}
		row := ProjectMetadataRow{RowID: record.RowID, Cells: make([]ProjectMetadataCell, len(columns))}
		for name, i := range columns {
			row.Cells[i] = cloneSiteUnitCell(values[name])
		}
		planned.Rows = append(planned.Rows, row)
	}
	return planned, nil
}
