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
	"reflect"
	"strconv"
	"strings"
	"time"
)

func RestoreSIVIHeights(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return RestoreSIVIVegetationCells(s, ctx, contextID, plot, historyID, action, SiviHeightWritePolicy())
}

func RestoreSIVIVegetationCells(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction, policy SiviVegetationWritePolicy) (*vprocore.AuditRestoreResult, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*vprocore.AuditRestoreResult, error) {
		if action != vprocore.AuditRestoreCancel && action != vprocore.AuditRestoreRetain && action != vprocore.AuditRestorePrune {
			return nil, errors.New("SIVI restoration requires cancel, retain or prune")
		}
		if action == vprocore.AuditRestoreCancel {
			return &vprocore.AuditRestoreResult{Cancelled: true}, nil
		}
		ids, err := vprocore.AuditRowIDs([]string{historyID})
		if err != nil {
			return nil, err
		}
		owner := plots.Projects.Sqlite
		result := &vprocore.AuditRestoreResult{}
		committed := false
		err = owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			if err := SiviHeightContextParent(ctx, owner, plot); err != nil {
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
			if err := vprocore.VerifyTechnicalProvenance(ctx, tx, policy.HistoryTable, policy.HistorySQL, "SIVI "+policy.Kind); err != nil {
				return err
			}
			var proposal string
			var restored *string
			if err := tx.QueryRowContext(ctx, `SELECT Proposal,Restored FROM `+vprocore.QuoteHeaderIdentifier(policy.HistoryTable)+` WHERE ID=?`, ids[0]).
				Scan(&proposal, &restored); err != nil {
				return err
			}
			if restored != nil {
				return fmt.Errorf("SIVI %s history was already restored; do not replay", policy.Kind)
			}
			if err := vprocore.ValidateMetadataDraftJSON([]byte(proposal)); err != nil {
				return err
			}
			decoder := json.NewDecoder(strings.NewReader(proposal))
			decoder.DisallowUnknownFields()
			var event SiviHeightHistory
			if err := decoder.Decode(&event); err != nil {
				return err
			}
			project := owner.Selection.Project
			if event.Project != project || event.Plot != plot || len(event.Changes) == 0 || len(event.Committed) == 0 {
				return errors.New("typed SIVI history is incomplete or belongs to another project/plot")
			}
			if err := siviHeightParents(ctx, tx, project, plot); err != nil {
				return err
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			veg := before[project+"_Veg"]
			if !reflect.DeepEqual(event.Columns, veg.Columns) {
				return errors.New("SIVI vegetation schema changed; restoration is unavailable")
			}
			columns, err := vprocore.SiteUnitTransferColumns(veg, append([]string{"ID", "Species", "PlotNumber"}, policy.Columns...)...)
			if err != nil {
				return err
			}
			rows := map[string]int{}
			for i, row := range veg.Rows {
				rows[row.RowID] = i
			}
			owned := map[string]bool{}
			for _, row := range event.Committed {
				i, present := rows[row.RowID]
				if !present || owned[row.RowID] || !reflect.DeepEqual(row, veg.Rows[i]) {
					return errors.New("SIVI physical row changed since the typed edit; no restoration or pruning")
				}
				owned[row.RowID] = true
			}
			planned := vprocore.ProjectMetadataTable{Columns: veg.Columns, Rows: append([]vprocore.ProjectMetadataRow{}, veg.Rows...)}
			audit := before[project+"_Audit"]
			auditColumns, err := vprocore.SiteUnitTransferColumns(audit, "Restore")
			if err != nil {
				return err
			}
			audit.Rows = append([]vprocore.ProjectMetadataRow{}, audit.Rows...)
			seen, seenAudits := map[[2]string]bool{}, map[string]bool{}
			allowed := map[string]bool{}
			for _, column := range policy.Columns {
				allowed[column] = true
			}
			for _, change := range event.Changes {
				if err := ctx.Err(); err != nil {
					return err
				}
				key := [2]string{change.RowID, change.Column}
				if !owned[change.RowID] || seen[key] || seenAudits[change.Audit.RowID] ||
					!allowed[change.Column] {
					return errors.New("typed SIVI restoration has unavailable or repeated cells/audits")
				}
				seen[key], seenAudits[change.Audit.RowID] = true, true
				i, column := rows[change.RowID], columns[change.Column]
				if !reflect.DeepEqual(veg.Rows[i].Cells[column], change.After) {
					return errors.New("SIVI committed cell differs from its typed audit")
				}
				value, err := vprocore.MetadataCellValue(change.Before)
				if err != nil || change.Before.Storage == "blob" {
					return errors.Join(err, errors.New("typed SIVI restoration contains unsupported original storage"))
				}
				after, err := vprocore.MetadataCellValue(change.After)
				if err != nil {
					return err
				}
				if !vprocore.MetadataAuditTextEqual(change.Audit.BeforeEdit, value) || !vprocore.MetadataAuditTextEqual(change.Audit.AfterEdit, after) ||
					change.Audit.EditField != change.Column || change.Audit.ID == nil ||
					veg.Rows[i].Cells[columns["ID"]].Integer == nil ||
					*veg.Rows[i].Cells[columns["ID"]].Integer != strconv.FormatInt(*change.Audit.ID, 10) ||
					veg.Rows[i].Cells[columns["PlotNumber"]].Storage != "text" ||
					veg.Rows[i].Cells[columns["PlotNumber"]].Text == nil || *veg.Rows[i].Cells[columns["PlotNumber"]].Text != plot {
					return errors.New("SIVI typed audit differs from its original physical identity/value")
				}
				ids, err := vprocore.AuditRowIDs([]string{change.Audit.RowID})
				if err != nil {
					return err
				}
				actual, err := vprocore.SelectedAuditEntries(tx, project, plot, ids)
				if err != nil {
					return err
				}
				normalize := func(entry vprocore.AuditEntry) (vprocore.AuditEntry, error) {
					if !strings.EqualFold(entry.Table, "_Veg") && !strings.EqualFold(entry.Table, project+"_Veg") {
						return vprocore.AuditEntry{}, errors.New("SIVI audit has an unavailable restoration table alias")
					}
					entry.Table = "_Veg"
					return entry, nil
				}
				observed, err := normalize(actual[0])
				if err != nil {
					return err
				}
				record, err := normalize(change.Audit)
				if err != nil || !reflect.DeepEqual(observed, record) {
					return errors.Join(err, errors.New("SIVI audit changed; no restoration or pruning"))
				}
				if err := VerifySIVIPhysicalAudit(before[project+"_Audit"], record, project); err != nil {
					return err
				}
				table := vprocore.QuoteHeaderIdentifier(project + "_Veg")
				if err := vprocore.RequireChildIdentity(tx, table, "Veg", plot, *change.Audit.ID); err != nil {
					return err
				}
				updated, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+vprocore.QuoteHeaderIdentifier(change.Column)+
					`=? WHERE rowid=? AND typeof(PlotNumber)='text' AND PlotNumber COLLATE BINARY IS ? AND ID=?`,
					value, change.RowID, plot, *change.Audit.ID)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SIVI restoration did not affect exactly one physical row"))
				}
				planned.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, planned.Rows[i].Cells...)
				planned.Rows[i].Cells[column] = vprocore.CloneSiteUnitCell(change.Before)
				query := `UPDATE ` + vprocore.QuoteHeaderIdentifier(project+"_Audit") + ` SET Restore=-1 WHERE rowid=?`
				if action == vprocore.AuditRestorePrune {
					query = `DELETE FROM ` + vprocore.QuoteHeaderIdentifier(project+"_Audit") + ` WHERE rowid=?`
				}
				changed, err := tx.ExecContext(ctx, query, ids[0])
				if err != nil {
					return err
				}
				if count, err := changed.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SIVI audit restoration did not affect exactly one row"))
				}
				for j, row := range audit.Rows {
					if row.RowID != change.Audit.RowID {
						continue
					}
					if action == vprocore.AuditRestorePrune {
						audit.Rows = append(audit.Rows[:j], audit.Rows[j+1:]...)
						result.PrunedAuditRows++
					} else {
						truth := "-1"
						audit.Rows[j].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
						audit.Rows[j].Cells[auditColumns["Restore"]] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &truth}
					}
					break
				}
				result.RestoredRows++
			}
			when := time.Now().UTC().Format(time.RFC3339Nano)
			updated, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(policy.HistoryTable)+` SET Restored=? WHERE ID=? AND Restored IS NULL`, when, ids[0])
			if err != nil {
				return err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("SIVI history restoration did not mark exactly one event"))
			}
			history := before[policy.HistoryTable]
			historyColumns, err := vprocore.SiteUnitTransferColumns(history, "ID", "Restored")
			if err != nil {
				return err
			}
			history.Rows = append([]vprocore.ProjectMetadataRow{}, history.Rows...)
			for i, row := range history.Rows {
				if row.Cells[historyColumns["ID"]].Integer != nil && *row.Cells[historyColumns["ID"]].Integer == historyID {
					history.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
					history.Rows[i].Cells[historyColumns["Restored"]] = vprocore.ProjectMetadataCell{Storage: "text", Text: &when}
				}
			}
			after, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := vprocore.VerifySiteUnitTransferTables(before, after, map[string]vprocore.ProjectMetadataTable{
				project + "_Veg": planned, project + "_Audit": audit, policy.HistoryTable: history,
			}, ""); err != nil {
				return fmt.Errorf("SIVI restoration differs from its complete plan: %w", err)
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
			return nil
		})
		if err != nil {
			if committed {
				return nil, fmt.Errorf("SIVI restoration committed but cleanup failed; do not replay: %w", err)
			}
			return nil, errors.Join(err, ctx.Err())
		}
		return result, nil
	})
}

func VerifySIVIPhysicalAudit(table vprocore.ProjectMetadataTable, record vprocore.AuditEntry, project string) error {
	return verifySIVIPhysicalTableAudit(table, record, project, "_Veg")
}

func verifySIVIPhysicalTableAudit(table vprocore.ProjectMetadataTable, record vprocore.AuditEntry, project, suffix string) error {
	expected, err := AppendSIVIPlannedAudits(vprocore.ProjectMetadataTable{Columns: table.Columns}, []vprocore.AuditEntry{record})
	if err != nil {
		return err
	}
	columns, err := vprocore.SiteUnitTransferColumns(table, "Table")
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		if row.RowID != record.RowID {
			continue
		}
		alias := row.Cells[columns["Table"]]
		if alias.Storage != "text" || alias.Text == nil ||
			(!strings.EqualFold(*alias.Text, suffix) && !strings.EqualFold(*alias.Text, project+suffix)) {
			return errors.New("SIVI physical audit has an unsupported table alias/storage")
		}
		row.Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
		row.Cells[columns["Table"]] = expected.Rows[0].Cells[columns["Table"]]
		if !reflect.DeepEqual(row, expected.Rows[0]) {
			return errors.New("SIVI physical audit storage changed; no restoration or pruning")
		}
		return nil
	}
	return errors.New("SIVI physical audit is no longer available")
}
