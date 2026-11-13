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
	"strings"
	"time"
)

func ValidateSIVIParentHistoryOriginal(ctx context.Context, event SiviParentHistory, project, plot string) (*SiviParentProjection, error) {
	return ValidateSourceParentHistoryOriginal(ctx, event, project, plot, ProjectSIVIParent)
}

func ValidateSourceParentHistoryOriginal(ctx context.Context, event SiviParentHistory, project, plot string, projectParent SourceParentProjector) (*SiviParentProjection, error) {
	if event.Original == nil || event.Committed == nil || len(event.Changes) == 0 ||
		event.Original.ContextID == "" || event.Original.ContextID != event.Committed.ContextID ||
		event.Original.Project != project || event.Original.Plot != plot ||
		event.Committed.Project != project || event.Committed.Plot != plot ||
		!reflect.DeepEqual(event.Original.EnvColumns, event.Committed.EnvColumns) ||
		!reflect.DeepEqual(event.Original.AdminColumns, event.Committed.AdminColumns) {
		return nil, errors.New("typed SIVI parent history is incomplete or belongs to another physical owner")
	}
	original := event.Original
	if len(original.Rows) != 1 || len(event.Committed.Rows) != 1 ||
		original.Rows[0].Env.RowID != event.Committed.Rows[0].Env.RowID ||
		original.Rows[0].Admin.RowID != event.Committed.Rows[0].Admin.RowID {
		return nil, errors.New("typed SIVI parent history changed physical identities")
	}
	rebuilt, err := projectParent(ctx, original.ContextID, project, plot,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return nil, errors.Join(err, errors.New("typed SIVI original projection differs from its physical rows"))
	}
	return original, nil
}

func SiviParentHistoryAssignments(ctx context.Context, event SiviParentHistory, project, plot string) ([]SiviParentScalarAssignment, error) {
	if event.ProjectAssignment != nil || event.EntryPlan != nil {
		return nil, errors.New("ProjectID assignment history is not direct parent history")
	}
	original, err := ValidateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	edits := SiviParentDirectEdits{}
	for _, change := range event.Changes {
		edit := SiviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After}
		if _, scalar := SiviParentScalarOwners[change.Column]; scalar {
			edits.Scalars = append(edits.Scalars, edit)
			continue
		}
		switch change.Column {
		case "SV_StandAgeEstMeas", "SV_StandHeightEstMeas":
			edits.Options = append(edits.Options, edit)
		case "SV_PolygonNumber", "SV_CanopyComposition":
			edits.Text = append(edits.Text, edit)
		case "SnowCoverregime", "SV_RootZoneTexture", "SV_AhorizonType":
			edits.Categorical = append(edits.Categorical, edit)
		default:
			return nil, errors.New("typed SIVI parent history contains an unavailable callback or identity target")
		}
	}
	assignments, err := PlanSIVIParentDirectEdits(ctx, original, edits)
	if err != nil || len(assignments) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("typed SIVI parent history contains repeated or unchanged assignments"))
	}
	return assignments, nil
}

func CtxRestoreSIVIParentDirect(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return RestoreSIVIParentHistory(s, ctx, contextID, plot, historyID, action, SiviParentDirectHistory)
}

func RestoreSIVIParentHistory(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction, historyDomain SiviParentHistoryDomain) (*vprocore.AuditRestoreResult, error) {
	return RestoreSourceParentHistory(s, ctx, contextID, plot, historyID, action, historyDomain, ReadSIVIParentForWrite)
}

func RestoreSourceParentHistory(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction, historyDomain SiviParentHistoryDomain, read func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*SiviParentProjection, error)) (*vprocore.AuditRestoreResult, error) {
	return RestoreSourceParentHistoryAuthorized(s, ctx, contextID, plot, historyID, action, historyDomain, read, nil)
}

func RestoreSourceParentHistoryAuthorized(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction, historyDomain SiviParentHistoryDomain, read func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*SiviParentProjection, error), authorize func(*vprocore.PlotService) error) (*vprocore.AuditRestoreResult, error) {
	return RestoreSourceParentHistoryCompared(s, ctx, contextID, plot, historyID, action, historyDomain, read, authorize,
		func(left, right *SiviParentProjection) bool { return reflect.DeepEqual(left, right) })
}

func RestoreSourceParentHistoryCompared(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction, historyDomain SiviParentHistoryDomain, read func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*SiviParentProjection, error), authorize func(*vprocore.PlotService) error, equal func(*SiviParentProjection, *SiviParentProjection) bool) (*vprocore.AuditRestoreResult, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*vprocore.AuditRestoreResult, error) {
		if action != vprocore.AuditRestoreCancel && action != vprocore.AuditRestoreRetain && action != vprocore.AuditRestorePrune {
			return nil, errors.New("SIVI parent restoration requires cancel, retain or prune")
		}
		if action == vprocore.AuditRestoreCancel {
			return &vprocore.AuditRestoreResult{Cancelled: true}, nil
		}
		if err := plots.RequireContextEdit(); err != nil {
			return nil, err
		}
		if authorize != nil {
			if err := authorize(plots); err != nil {
				return nil, err
			}
		}
		ids, err := vprocore.AuditRowIDs([]string{historyID})
		if err != nil {
			return nil, err
		}
		owner := plots.Projects.Sqlite
		result := &vprocore.AuditRestoreResult{}
		err = WithSIVIParentWriteTransaction(ctx, owner, "parent restoration", func(tx *sql.Tx, suAlias string) error {
			if err := vprocore.VerifyTechnicalProvenance(ctx, tx, historyDomain.Table, historyDomain.Schema, historyDomain.Label); err != nil {
				return err
			}
			var proposal string
			var restored *string
			if err := tx.QueryRowContext(ctx, `SELECT Proposal,Restored FROM `+vprocore.QuoteHeaderIdentifier(historyDomain.Table)+` WHERE ID=?`, ids[0]).
				Scan(&proposal, &restored); err != nil {
				return err
			}
			if restored != nil {
				return errors.New("SIVI parent history was already restored; do not replay")
			}
			if !json.Valid([]byte(proposal)) {
				return errors.New("typed SIVI parent history requires one complete JSON value")
			}
			if err := vprocore.ValidateMetadataDraftJSON([]byte(proposal)); err != nil {
				return err
			}
			decoder := json.NewDecoder(strings.NewReader(proposal))
			decoder.DisallowUnknownFields()
			var event SiviParentHistory
			if err := decoder.Decode(&event); err != nil {
				return err
			}
			project := owner.Selection.Project
			if event.EntryPlan != nil && !TwoPageEntryProjectHistoryTable(historyDomain.Table) {
				return errors.New("complete-entry source plans belong only to their exact entry history")
			}
			if historyDomain.Table != siviProjectAssignmentHistoryTable && !TwoPageEntryProjectHistoryTable(historyDomain.Table) && event.ProjectAssignment != nil {
				return errors.New("ProjectID assignment provenance belongs only to its isolated history")
			}
			if _, err := historyDomain.Assignments(ctx, event, project, plot); err != nil {
				return err
			}
			fresh, err := read(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			expectedParent := *event.Committed
			expectedParent.ContextID = contextID
			if !equal(fresh, &expectedParent) {
				return errors.New("SIVI physical parents changed since the typed edit; no restoration or pruning")
			}
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "main", project+"_Audit", "parent restoration audits"); err != nil {
				return err
			}
			before, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			expected := map[string]vprocore.ProjectMetadataTable{}
			audit := before[project+"_Audit"]
			audit.Rows = append([]vprocore.ProjectMetadataRow{}, audit.Rows...)
			auditColumns, err := vprocore.SiteUnitTransferColumns(audit, "Restore")
			if err != nil {
				return err
			}
			seenAudits := map[string]bool{}
			for _, change := range event.Changes {
				if err := ctx.Err(); err != nil {
					return err
				}
				suffix, identity := "_Env", "PlotNumber"
				if change.Table == project+"_Admin" {
					suffix, identity = "_Admin", "Plot"
				}
				planned, present := expected[change.Table]
				if !present {
					planned = before[change.Table]
					planned.Rows = append([]vprocore.ProjectMetadataRow{}, planned.Rows...)
				}
				columns, err := vprocore.SiteUnitTransferColumns(planned, change.Column)
				if err != nil {
					return err
				}
				rowIndex := -1
				for i, row := range planned.Rows {
					if row.RowID == change.RowID {
						rowIndex = i
					}
				}
				if rowIndex < 0 || !reflect.DeepEqual(planned.Rows[rowIndex].Cells[columns[change.Column]], change.After) {
					return errors.New("SIVI committed parent cell differs from its typed audit")
				}
				value, err := vprocore.MetadataCellValue(change.Before)
				if err != nil || change.Before.Storage == "blob" {
					return errors.Join(err, errors.New("typed SIVI parent restoration contains unsupported original storage"))
				}
				after, err := vprocore.MetadataCellValue(change.After)
				if err != nil {
					return err
				}
				record := change.Audit
				if seenAudits[record.RowID] || record.ID != nil || record.EditField != change.Column ||
					record.Restore || record.Flag || record.Project != project || record.PlotNumber != plot ||
					!vprocore.MetadataAuditTextEqual(record.BeforeEdit, value) || !vprocore.MetadataAuditTextEqual(record.AfterEdit, after) {
					return errors.New("typed SIVI parent audit differs from its physical identity/value")
				}
				seenAudits[record.RowID] = true
				auditIDs, err := vprocore.AuditRowIDs([]string{record.RowID})
				if err != nil {
					return err
				}
				actual, err := vprocore.SelectedAuditEntries(tx, project, plot, auditIDs)
				if err != nil {
					return err
				}
				normalize := func(entry vprocore.AuditEntry) (vprocore.AuditEntry, error) {
					if !strings.EqualFold(entry.Table, suffix) && !strings.EqualFold(entry.Table, project+suffix) {
						return vprocore.AuditEntry{}, errors.New("SIVI parent audit has an unavailable restoration table alias")
					}
					entry.Table = suffix
					return entry, nil
				}
				observed, err := normalize(actual[0])
				if err != nil {
					return err
				}
				record, err = normalize(record)
				if err != nil || !reflect.DeepEqual(observed, record) {
					return errors.Join(err, errors.New("SIVI parent audit changed; no restoration or pruning"))
				}
				if err := verifySIVIPhysicalTableAudit(before[project+"_Audit"], record, project, suffix); err != nil {
					return err
				}
				updated, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(change.Table)+
					` SET `+vprocore.QuoteHeaderIdentifier(change.Column)+`=? WHERE rowid=? AND typeof(`+vprocore.QuoteHeaderIdentifier(identity)+
					`)='text' AND `+vprocore.QuoteHeaderIdentifier(identity)+` COLLATE BINARY IS ?`, value, change.RowID, plot)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SIVI parent restoration did not affect exactly one physical row"))
				}
				planned.Rows[rowIndex].Cells = append([]vprocore.ProjectMetadataCell{}, planned.Rows[rowIndex].Cells...)
				planned.Rows[rowIndex].Cells[columns[change.Column]] = vprocore.CloneSiteUnitCell(change.Before)
				expected[change.Table] = planned
				query := `UPDATE ` + vprocore.QuoteHeaderIdentifier(project+"_Audit") + ` SET Restore=-1 WHERE rowid=?`
				if action == vprocore.AuditRestorePrune {
					query = `DELETE FROM ` + vprocore.QuoteHeaderIdentifier(project+"_Audit") + ` WHERE rowid=?`
				}
				changed, err := tx.ExecContext(ctx, query, auditIDs[0])
				if err != nil {
					return err
				}
				if count, err := changed.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SIVI parent audit restoration did not affect exactly one row"))
				}
				for i, row := range audit.Rows {
					if row.RowID != record.RowID {
						continue
					}
					if action == vprocore.AuditRestorePrune {
						audit.Rows = append(audit.Rows[:i], audit.Rows[i+1:]...)
						result.PrunedAuditRows++
					} else {
						audit.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
						truth := "-1"
						audit.Rows[i].Cells[auditColumns["Restore"]] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &truth}
					}
					break
				}
				result.RestoredRows++
			}
			expected[project+"_Audit"] = audit
			when := time.Now().UTC().Format(time.RFC3339Nano)
			updated, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(historyDomain.Table)+` SET Restored=? WHERE ID=? AND Restored IS NULL`, when, ids[0])
			if err != nil {
				return err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("SIVI parent history restoration did not affect exactly one event"))
			}
			history := before[historyDomain.Table]
			history.Rows = append([]vprocore.ProjectMetadataRow{}, history.Rows...)
			historyColumns, err := vprocore.SiteUnitTransferColumns(history, "Restored")
			if err != nil {
				return err
			}
			for i, row := range history.Rows {
				if row.RowID == historyID {
					history.Rows[i].Cells = append([]vprocore.ProjectMetadataCell{}, row.Cells...)
					history.Rows[i].Cells[historyColumns["Restored"]] = vprocore.ProjectMetadataCell{Storage: "text", Text: &when}
				}
			}
			expected[historyDomain.Table] = history
			after, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := vprocore.VerifySiteUnitTransferTables(before, after, expected, ""); err != nil {
				return fmt.Errorf("SIVI parent restoration differs from its complete plan: %w", err)
			}
			if _, err := read(ctx, tx, owner, suAlias, contextID, plot); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
}
