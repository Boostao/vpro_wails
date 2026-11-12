package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

const SiviHeightHistoryTable = "__VPRO_SIVIHeightHistory"
const SiviHeightHistorySQL = `CREATE TABLE "__VPRO_SIVIHeightHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

type SIVIHeightWriteResult struct {
	ChangedCells int
	HistoryID    string
}

type siviHeightWriteResult = SIVIHeightWriteResult

type siviHeightHistoryChange struct {
	RowID, Column string
	Before, After vprocore.ProjectMetadataCell
	Audit         vprocore.AuditEntry
}

type SiviHeightHistory struct {
	Project, Plot string
	Columns       []vprocore.ProjectMetadataColumn
	Committed     []vprocore.ProjectMetadataRow
	Changes       []siviHeightHistoryChange
}

type SiviVegetationWritePolicy struct {
	Kind, HistoryTable, HistorySQL string
	Columns                        []string
	Plan                           func(context.Context, string, bool, vprocore.ProjectMetadataTable, []SiviHeightEdit) ([]SiviHeightAssignment, error)
}

type SiviVegetationWriteHooks struct {
	Prepare  func(context.Context, *sql.Conn, *vprocore.SqliteContext) error                       `json:"-" yaml:"-"`
	Validate func(context.Context, *sql.Tx, *vprocore.SqliteContext, []SiviHeightAssignment) error `json:"-" yaml:"-"`
}

func SiviHeightWritePolicy() SiviVegetationWritePolicy {
	return SiviVegetationWritePolicy{"height", SiviHeightHistoryTable, SiviHeightHistorySQL,
		[]string{"HeightA", "HeightB", "Height6"}, PlanSIVIHeightEdits}
}

func WriteSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []SiviVegetationProjection, edits []SiviHeightEdit) (*siviHeightWriteResult, error) {
	return WriteSIVIVegetationCells(s, ctx, contextID, plot, extended, original, edits, SiviHeightWritePolicy())
}

func WriteSIVIVegetationCells(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []SiviVegetationProjection, edits []SiviHeightEdit, policy SiviVegetationWritePolicy) (*siviHeightWriteResult, error) {
	return WriteSIVIVegetationCellsWithHooks(s, ctx, contextID, plot, extended, original, edits, policy, SiviVegetationWriteHooks{})
}

func WriteSIVIVegetationCellsWithHooks(s *vprocore.ContextService, ctx context.Context, contextID, plot string, extended bool, original []SiviVegetationProjection, edits []SiviHeightEdit, policy SiviVegetationWritePolicy, hooks SiviVegetationWriteHooks) (*siviHeightWriteResult, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*siviHeightWriteResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return nil, err
		}
		if err := vprocore.ValidateChildPhysicalText("SIVI audit user", plots.CurrentUser, 100); err != nil {
			return nil, err
		}
		owner := plots.Projects.Sqlite
		result := &siviHeightWriteResult{}
		committed := false
		err := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			if err := SiviHeightContextParent(ctx, owner, plot); err != nil {
				return err
			}
			if hooks.Prepare != nil {
				if err := hooks.Prepare(ctx, conn, owner); err != nil {
					return err
				}
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
			project := owner.Selection.Project
			if err := siviHeightParents(ctx, tx, project, plot); err != nil {
				return err
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			veg := before[project+"_Veg"]
			observed, err := ProjectSIVIVegetation(ctx, plot, extended, veg)
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
			if hooks.Validate != nil {
				if err := hooks.Validate(ctx, tx, owner, assignments); err != nil {
					return err
				}
			}
			planned, changes, records, err := applySIVIHeightAssignments(ctx, tx, project, plot, veg, assignments, plots.CurrentUser, plots.AuditStrength)
			if err != nil {
				return err
			}
			audit, err := AppendSIVIPlannedAudits(before[project+"_Audit"], records)
			if err != nil {
				return err
			}
			expected := map[string]vprocore.ProjectMetadataTable{project + "_Veg": planned, project + "_Audit": audit}
			after, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := vprocore.VerifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("SIVI transaction differs from its complete plan: %w", err)
			}
			if len(changes) != 0 {
				history := SiviHeightHistory{Project: project, Plot: plot, Columns: planned.Columns, Changes: changes}
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
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, policy.HistoryTable, policy.HistorySQL, string(proposal), "SIVI "+policy.Kind); err != nil {
					return err
				}
				var id int64
				if err := tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM `+vprocore.QuoteHeaderIdentifier(policy.HistoryTable)).Scan(&id); err != nil {
					return err
				}
				result.HistoryID = strconv.FormatInt(id, 10)
				after, err = vprocore.EnvironmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				if err := vprocore.VerifySiteUnitTransferTables(before, after, expected, policy.HistoryTable); err != nil {
					return fmt.Errorf("SIVI provenance transaction differs from its plan: %w", err)
				}
			}
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return err
			}
			if err := SiviHeightContextParent(ctx, owner, plot); err != nil {
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

func SiviHeightContextParent(ctx context.Context, owner *vprocore.SqliteContext, plot string) error {
	var member bool
	if err := owner.Conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, plot).
		Scan(&member); err != nil || !member {
		return errors.Join(err, errors.New("SIVI parent is outside the selected context"))
	}
	return nil
}

func siviHeightParents(ctx context.Context, tx *sql.Tx, project, plot string) error {
	if err := vprocore.ChildParent(tx, project, plot); err != nil {
		return err
	}
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		var count int
		column := vprocore.QuoteHeaderIdentifier(parent.column)
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(project+"_"+parent.table)+
			` WHERE typeof(`+column+`)='text' AND `+column+` COLLATE BINARY IS ?`, plot).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("SIVI editing requires unique original literal text parents")
		}
	}
	return nil
}

func applySIVIHeightAssignments(ctx context.Context, tx *sql.Tx, project, plot string, veg vprocore.ProjectMetadataTable, assignments []SiviHeightAssignment, user string, strength int) (vprocore.ProjectMetadataTable, []siviHeightHistoryChange, []vprocore.AuditEntry, error) {
	columns, err := vprocore.SiteUnitTransferColumns(veg, "ID", "Species", "PlotNumber")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, nil, nil, err
	}
	planned := vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: append([]vprocore.ProjectMetadataRow{}, veg.Rows...)}
	changes := []siviHeightHistoryChange{}
	records := []vprocore.AuditEntry{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, assignment := range assignments {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, nil, nil, err
		}
		for i, row := range planned.Rows {
			if row.RowID != assignment.RowID {
				continue
			}
			idCell := row.Cells[columns["ID"]]
			if idCell.Storage != "integer" || idCell.Integer == nil {
				return vprocore.ProjectMetadataTable{}, nil, nil, errors.New("SIVI writes require the original signed32 application ID")
			}
			id, err := strconv.ParseInt(*idCell.Integer, 10, 32)
			if err != nil || id < math.MinInt32 || id > math.MaxInt32 {
				return vprocore.ProjectMetadataTable{}, nil, nil, errors.New("SIVI writes require the original signed32 application ID")
			}
			table := vprocore.QuoteHeaderIdentifier(project + "_Veg")
			if err := vprocore.RequireChildIdentity(tx, table, "Veg", plot, id); err != nil {
				return vprocore.ProjectMetadataTable{}, nil, nil, err
			}
			before, err := vprocore.MetadataCellValue(assignment.Before)
			if err != nil {
				return vprocore.ProjectMetadataTable{}, nil, nil, err
			}
			if assignment.Before.Storage == "blob" {
				return vprocore.ProjectMetadataTable{}, nil, nil, errors.New("historical SIVI blob replacement requires separately defined audit restoration; omit it unchanged")
			}
			updated, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+vprocore.QuoteHeaderIdentifier(assignment.Column)+
				`=? WHERE rowid=? AND typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ? AND ID=?`,
				assignment.Value, assignment.RowID, plot, id)
			if err != nil {
				return vprocore.ProjectMetadataTable{}, nil, nil, err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return vprocore.ProjectMetadataTable{}, nil, nil, errors.Join(err, errors.New("SIVI height update did not affect exactly one physical row"))
			}
			planned.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
			planned.Rows[i].Cells[columns[assignment.Column]] = vprocore.CloneSiteUnitCell(assignment.After)
			audits, err := vprocore.AuditChildFieldsTracked(tx, project, "Veg", plot, id,
				[]vprocore.ChildField{{Column: assignment.Column}}, []any{before}, []any{assignment.Value}, user, strength, when)
			if err != nil {
				return vprocore.ProjectMetadataTable{}, nil, nil, err
			}
			for _, audit := range audits {
				ids, err := vprocore.AuditRowIDs([]string{audit.RowID})
				if err != nil {
					return vprocore.ProjectMetadataTable{}, nil, nil, err
				}
				inserted, err := vprocore.SelectedAuditEntries(tx, project, plot, ids)
				if err != nil {
					return vprocore.ProjectMetadataTable{}, nil, nil, err
				}
				record := inserted[0]
				if record.Table != "_Veg" || record.EditField != assignment.Column ||
					record.ID == nil || *record.ID != id || record.User != user || record.EditWhen != when ||
					record.Restore || record.Flag || !vprocore.MetadataAuditTextEqual(record.BeforeEdit, before) ||
					!vprocore.MetadataAuditTextEqual(record.AfterEdit, assignment.Value) {
					return vprocore.ProjectMetadataTable{}, nil, nil, errors.New("SIVI inserted audit differs from its typed assignment")
				}
				changes = append(changes, siviHeightHistoryChange{assignment.RowID, assignment.Column,
					vprocore.CloneSiteUnitCell(assignment.Before), vprocore.CloneSiteUnitCell(assignment.After), record})
				records = append(records, record)
			}
		}
	}
	return planned, changes, records, nil
}

func AppendSIVIPlannedAudits(original vprocore.ProjectMetadataTable, records []vprocore.AuditEntry) (vprocore.ProjectMetadataTable, error) {
	columns, err := vprocore.SiteUnitTransferColumns(original, "Project", "User", "PlotNumber", "Table", "EditField",
		"EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID")
	if err != nil || len(columns) != 11 {
		return vprocore.ProjectMetadataTable{}, errors.Join(err, errors.New("SIVI requires the original eleven-column audit schema"))
	}
	planned := vprocore.ProjectMetadataTable{Columns: original.Columns, Rows: append([]vprocore.ProjectMetadataRow{}, original.Rows...)}
	for _, record := range records {
		zero := "0"
		values := map[string]vprocore.ProjectMetadataCell{
			"Project": {Storage: "text", Text: &record.Project}, "User": {Storage: "text", Text: &record.User},
			"PlotNumber": {Storage: "text", Text: &record.PlotNumber}, "Table": {Storage: "text", Text: &record.Table},
			"EditField": {Storage: "text", Text: &record.EditField}, "EditWhen": {Storage: "text", Text: &record.EditWhen},
			"BeforeEdit": {Storage: "null"}, "AfterEdit": {Storage: "null"},
			"Restore": {Storage: "integer", Integer: &zero}, "Flag": {Storage: "integer", Integer: &zero},
			"ID": {Storage: "null"},
		}
		if record.ID != nil {
			id := strconv.FormatInt(*record.ID, 10)
			values["ID"] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &id}
		}
		if record.BeforeEdit != nil {
			values["BeforeEdit"] = vprocore.ProjectMetadataCell{Storage: "text", Text: record.BeforeEdit}
		}
		if record.AfterEdit != nil {
			values["AfterEdit"] = vprocore.ProjectMetadataCell{Storage: "text", Text: record.AfterEdit}
		}
		row := vprocore.ProjectMetadataRow{RowID: record.RowID, Cells: make([]vprocore.ProjectMetadataCell, len(columns))}
		for name, i := range columns {
			row.Cells[i] = vprocore.CloneSiteUnitCell(values[name])
		}
		planned.Rows = append(planned.Rows, row)
	}
	return planned, nil
}
