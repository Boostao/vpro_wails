package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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

type SiviParentDirectEdits struct {
	Scalars, Options, Text, Categorical []SiviParentScalarEdit
}

type SIVIParentWriteResult struct {
	ChangedCells int
	HistoryID    string
}

type SiviParentWriteResult = SIVIParentWriteResult

type SiviParentHistoryChange struct {
	Table, RowID, Column string
	Before, After        vprocore.ProjectMetadataCell
	Audit                vprocore.AuditEntry
}

type SiviParentHistory struct {
	Original, Committed *SiviParentProjection
	Changes             []SiviParentHistoryChange
	ProjectAssignment   *SiviProjectAssignmentHistory `json:",omitempty"`
	EntryPlan           []SiviParentScalarEdit        `json:",omitempty"`
}

type SiviParentWriteHooks struct {
	Authorize func(*vprocore.PlotService) error                                                                              `json:"-" yaml:"-"`
	Read      func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*SiviParentProjection, error) `json:"-" yaml:"-"`
	Prepare   func(*sql.Conn) (func(), error)                                                                                `json:"-" yaml:"-"`
	Verify    func(*sql.Tx) error                                                                                            `json:"-" yaml:"-"`
	Plan      func(*sql.Tx, *SiviParentProjection) ([]SiviParentScalarAssignment, error)                                     `json:"-" yaml:"-"`
	Decorate  func(*SiviParentHistory)                                                                                       `json:"-" yaml:"-"`
}

type SiviParentHistoryDomain struct {
	Table, Schema, Label string                                                                                         `json:"-" yaml:"-"`
	Assignments          func(context.Context, SiviParentHistory, string, string) ([]SiviParentScalarAssignment, error) `json:"-" yaml:"-"`
}

var SiviParentDirectHistory = SiviParentHistoryDomain{
	siviParentHistoryTable, siviParentHistorySQL, "SIVI parent", SiviParentHistoryAssignments,
}

func PlanSIVIParentDirectEdits(ctx context.Context, original *SiviParentProjection, edits SiviParentDirectEdits) ([]SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 ||
		len(edits.Scalars)+len(edits.Options)+len(edits.Text)+len(edits.Categorical) == 0 {
		return nil, errors.New("SIVI parent writing requires one physical pair and explicit direct-field edits")
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	assignments := []SiviParentScalarAssignment{}
	for _, domain := range []struct {
		edits []SiviParentScalarEdit
		plan  func(context.Context, string, string, string, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, []SiviParentScalarEdit) ([]SiviParentScalarAssignment, error)
	}{
		{edits.Scalars, PlanSIVIParentScalars}, {edits.Options, PlanSIVIParentOptions},
		{edits.Text, PlanSIVIParentText}, {edits.Categorical, PlanSIVIParentCategorical},
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

func SourceParentWriterSUAlias(owner *vprocore.SqliteContext) string {
	if owner.Selection.SU == "None" {
		return ""
	}
	if owner.Attachments["su"] == owner.Attachments["project"] {
		return "main"
	}
	return "sivi_su"
}

func SiviParentWriterSU(ctx context.Context, conn *sql.Conn, owner *vprocore.SqliteContext) (string, error) {
	alias := SourceParentWriterSUAlias(owner)
	if alias != "sivi_su" {
		return alias, nil
	}
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "sivi_su"`, sqlutil.SqliteFileURI(owner.Attachments["su"], "ro")); err != nil {
		return "", fmt.Errorf("SIVI selected SU attachment unavailable: %w", err)
	}
	return "sivi_su", nil
}

type SourceParentProjector func(context.Context, string, string, string, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) (*SiviParentProjection, error)

func ReadSIVIParentForWrite(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, suAlias, contextID, plot string) (*SiviParentProjection, error) {
	return ReadSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, ProjectSIVIParent)
}

func ReadSourceParentForWrite(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, suAlias, contextID, plot string, projectParent SourceParentProjector) (*SiviParentProjection, error) {
	project := owner.Selection.Project
	join, err := readSIVIParentJoin(ctx, tx, "main", contextID, project, plot)
	if err != nil {
		return nil, err
	}
	if !join.Verified {
		return nil, fmt.Errorf("SIVI parent writing requires certified physical source membership: %s", join.Diagnostic)
	}
	if suAlias != "" {
		table := owner.Selection.SU + "_SU"
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(suAlias)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, errors.New("SIVI parent writing requires the selected physical SU table")
		}
		if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, suAlias, table, "selected SU membership"); err != nil {
			return nil, err
		}
		var member bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+vprocore.QuoteHeaderIdentifier(suAlias)+"."+
			vprocore.QuoteHeaderIdentifier(table)+` WHERE typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ?)`, plot).
			Scan(&member); err != nil {
			return nil, err
		}
		if !member {
			return nil, errors.New("SIVI parent is outside the selected SU context")
		}
	}
	env, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", project+"_Env", "PlotNumber", &plot, "")
	if err != nil {
		return nil, err
	}
	admin, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", project+"_Admin", "Plot", &plot, "")
	if err != nil {
		return nil, err
	}
	parent, err := projectParent(ctx, contextID, project, plot, env, admin)
	if err != nil {
		return nil, err
	}
	if len(parent.Rows) != 1 || parent.Rows[0].Env.RowID != join.EnvRowIDs[0] || parent.Rows[0].Admin.RowID != join.AdminRowIDs[0] {
		return nil, errors.New("SIVI projection differs from the certified physical source pair")
	}
	return parent, nil
}

func WithSIVIParentWriteTransaction(ctx context.Context, owner *vprocore.SqliteContext, purpose string, operation func(*sql.Tx, string) error, hooks ...SiviParentWriteHooks) error {
	committed := false
	err := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
		for _, hook := range hooks {
			if hook.Prepare != nil {
				release, err := hook.Prepare(conn)
				if err != nil {
					return err
				}
				defer release()
			}
		}
		suAlias, err := SiviParentWriterSU(ctx, conn, owner)
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
		for _, hook := range hooks {
			if hook.Verify != nil {
				if err := hook.Verify(tx); err != nil {
					return err
				}
			}
		}
		if err := owner.ValidateMetadataWriterFiles(); err != nil {
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

func WriteSIVIParentDirect(s *vprocore.ContextService, ctx context.Context, contextID, plot string, original *SiviParentProjection, edits SiviParentDirectEdits) (*SiviParentWriteResult, error) {
	return WriteSIVIParentPlanned(s, ctx, contextID, plot, original, SiviParentDirectHistory, func(observed *SiviParentProjection) ([]SiviParentScalarAssignment, error) {
		assignments, err := PlanSIVIParentDirectEdits(ctx, observed, edits)
		if err != nil {
			return nil, err
		}
		if err := ValidateSourceParentBooleanAssignments(assignments); err != nil {
			return nil, fmt.Errorf("SIVI direct entry: %w", err)
		}
		return assignments, nil
	})
}

func WriteSIVIParentPlanned(s *vprocore.ContextService, ctx context.Context, contextID, plot string, original *SiviParentProjection, history SiviParentHistoryDomain, plan func(*SiviParentProjection) ([]SiviParentScalarAssignment, error)) (*SiviParentWriteResult, error) {
	return WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, history, plan, SiviParentWriteHooks{})
}

func WriteSIVIParentPlannedWithHooks(s *vprocore.ContextService, ctx context.Context, contextID, plot string, original *SiviParentProjection, history SiviParentHistoryDomain, plan func(*SiviParentProjection) ([]SiviParentScalarAssignment, error), hooks SiviParentWriteHooks) (*SiviParentWriteResult, error) {
	read := ReadSIVIParentForWrite
	if hooks.Read != nil {
		read = hooks.Read
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiviParentWriteResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return nil, err
		}
		if hooks.Authorize != nil {
			if err := hooks.Authorize(plots); err != nil {
				return nil, err
			}
		}
		if history.Table == siviProjectAssignmentHistoryTable && plots.CurrentUser == "" {
			return nil, errors.New("SIVI parent writing requires an explicit audit user")
		}
		if err := vprocore.ValidateChildPhysicalText("SIVI parent audit user", plots.CurrentUser, 100); err != nil {
			return nil, err
		}
		owner := plots.Projects.Sqlite
		result := &SiviParentWriteResult{}
		err := WithSIVIParentWriteTransaction(ctx, owner, "parent edit", func(tx *sql.Tx, suAlias string) error {
			observed, err := read(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(original, observed) {
				return errors.New("SIVI original physical parents changed; undo and reload before writing")
			}
			var assignments []SiviParentScalarAssignment
			if hooks.Plan != nil {
				assignments, err = hooks.Plan(tx, observed)
			} else {
				assignments, err = plan(observed)
			}
			if err != nil {
				return err
			}
			if len(assignments) == 0 {
				return ctx.Err()
			}
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "main", observed.Project+"_Audit", "parent audits"); err != nil {
				return err
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			expected, changes, records, err := ApplySIVIParentAssignments(ctx, tx, observed, before, assignments, plots.CurrentUser, plots.AuditStrength)
			if err != nil {
				return err
			}
			expected[observed.Project+"_Audit"], err = AppendSIVIPlannedAudits(before[observed.Project+"_Audit"], records)
			if err != nil {
				return err
			}
			after, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := vprocore.VerifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("SIVI parent transaction differs from its complete plan: %w", err)
			}
			fresh, err := read(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			if len(changes) != 0 {
				event := SiviParentHistory{Original: observed, Committed: fresh, Changes: changes}
				if hooks.Decorate != nil {
					hooks.Decorate(&event)
				}
				proposal, err := json.Marshal(event)
				if err != nil {
					return err
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, history.Table, history.Schema, string(proposal), history.Label); err != nil {
					return err
				}
				var id int64
				if err := tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM `+vprocore.QuoteHeaderIdentifier(history.Table)).Scan(&id); err != nil {
					return err
				}
				result.HistoryID = strconv.FormatInt(id, 10)
				after, err = vprocore.EnvironmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				if err := vprocore.VerifySiteUnitTransferTables(before, after, expected, history.Table); err != nil {
					return fmt.Errorf("SIVI parent provenance differs from its complete plan: %w", err)
				}
			}
			result.ChangedCells = len(assignments)
			return nil
		}, hooks)
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}

func ApplySIVIParentAssignments(ctx context.Context, tx *sql.Tx, original *SiviParentProjection, before map[string]vprocore.ProjectMetadataTable, assignments []SiviParentScalarAssignment, user string, strength int) (map[string]vprocore.ProjectMetadataTable, []SiviParentHistoryChange, []vprocore.AuditEntry, error) {
	expected := map[string]vprocore.ProjectMetadataTable{}
	changes, records := []SiviParentHistoryChange{}, []vprocore.AuditEntry{}
	when := time.Now().Format("2006-01-02 15:04:05")
	for _, assignment := range assignments {
		suffix, identity := "Env", "PlotNumber"
		if assignment.Table == original.AdminTable {
			suffix, identity = "Admin", "Plot"
		}
		prior, err := vprocore.MetadataCellValue(assignment.Before)
		if err != nil {
			return nil, nil, nil, err
		}
		if assignment.Before.Storage == "blob" {
			return nil, nil, nil, errors.New("historical SIVI parent BLOB replacement has no source audit representation; omit it unchanged")
		}
		changed, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(assignment.Table)+
			` SET `+vprocore.QuoteHeaderIdentifier(assignment.Column)+`=? WHERE rowid=? AND typeof(`+vprocore.QuoteHeaderIdentifier(identity)+
			`)='text' AND `+vprocore.QuoteHeaderIdentifier(identity)+` COLLATE BINARY IS ?`, assignment.Value, assignment.RowID, original.Plot)
		if err != nil {
			return nil, nil, nil, err
		}
		if count, err := changed.RowsAffected(); err != nil || count != 1 {
			return nil, nil, nil, errors.Join(err, errors.New("SIVI parent edit did not affect exactly one physical row"))
		}
		planned, present := expected[assignment.Table]
		if !present {
			planned = before[assignment.Table]
			planned.Rows = append([]vprocore.ProjectMetadataRow{}, planned.Rows...)
		}
		columns, err := vprocore.SiteUnitTransferColumns(planned, assignment.Column)
		if err != nil {
			return nil, nil, nil, err
		}
		for i, row := range planned.Rows {
			if row.RowID == assignment.RowID {
				planned.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
				planned.Rows[i].Cells[columns[assignment.Column]] = vprocore.CloneSiteUnitCell(assignment.After)
			}
		}
		expected[assignment.Table] = planned
		if !vprocore.AuditHeaderChange(prior, assignment.Value, strength) {
			continue
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO `+vprocore.QuoteHeaderIdentifier(original.Project+"_Audit")+
			` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,NULL)`,
			original.Project, user, original.Plot, "_"+suffix, assignment.Column, when, vprocore.HeaderAuditValue(prior), vprocore.HeaderAuditValue(assignment.Value))
		if err != nil {
			return nil, nil, nil, err
		}
		id, err := inserted.LastInsertId()
		if err != nil {
			return nil, nil, nil, err
		}
		actual, err := vprocore.SelectedAuditEntries(tx, original.Project, original.Plot, []int64{id})
		if err != nil {
			return nil, nil, nil, err
		}
		record := actual[0]
		if record.Table != "_"+suffix || record.ID != nil || record.EditField != assignment.Column ||
			record.User != user || record.EditWhen != when || record.Restore || record.Flag ||
			!vprocore.MetadataAuditTextEqual(record.BeforeEdit, prior) || !vprocore.MetadataAuditTextEqual(record.AfterEdit, assignment.Value) {
			return nil, nil, nil, errors.New("SIVI parent inserted audit differs from its typed assignment")
		}
		changes = append(changes, SiviParentHistoryChange{assignment.Table, assignment.RowID, assignment.Column,
			vprocore.CloneSiteUnitCell(assignment.Before), vprocore.CloneSiteUnitCell(assignment.After), record})
		records = append(records, record)
	}
	return expected, changes, records, nil
}
