package main

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

func validateSIVIParentHistoryOriginal(ctx context.Context, event siviParentHistory, project, plot string) (*siviParentProjection, error) {
	return validateSourceParentHistoryOriginal(ctx, event, project, plot, projectSIVIParent)
}

func validateSourceParentHistoryOriginal(ctx context.Context, event siviParentHistory, project, plot string, projectParent sourceParentProjector) (*siviParentProjection, error) {
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
		ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}},
		ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil || !reflect.DeepEqual(rebuilt, original) {
		return nil, errors.Join(err, errors.New("typed SIVI original projection differs from its physical rows"))
	}
	return original, nil
}

func siviParentHistoryAssignments(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
	if event.ProjectAssignment != nil || event.EntryPlan != nil {
		return nil, errors.New("ProjectID assignment history is not direct parent history")
	}
	original, err := validateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	edits := siviParentDirectEdits{}
	for _, change := range event.Changes {
		edit := siviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After}
		if _, scalar := siviParentScalarOwners[change.Column]; scalar {
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
	assignments, err := planSIVIParentDirectEdits(ctx, original, edits)
	if err != nil || len(assignments) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("typed SIVI parent history contains repeated or unchanged assignments"))
	}
	return assignments, nil
}

func (s *ContextService) restoreSIVIParentDirect(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreSIVIParentHistory(ctx, contextID, plot, historyID, action, siviParentDirectHistory)
}

func (s *ContextService) restoreSIVIParentHistory(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction, historyDomain siviParentHistoryDomain) (*AuditRestoreResult, error) {
	return s.restoreSourceParentHistory(ctx, contextID, plot, historyID, action, historyDomain, readSIVIParentForWrite)
}

func (s *ContextService) restoreSourceParentHistory(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction, historyDomain siviParentHistoryDomain, read func(context.Context, *sql.Tx, *sqliteContext, string, string, string) (*siviParentProjection, error)) (*AuditRestoreResult, error) {
	return s.restoreSourceParentHistoryAuthorized(ctx, contextID, plot, historyID, action, historyDomain, read, nil)
}

func (s *ContextService) restoreSourceParentHistoryAuthorized(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction, historyDomain siviParentHistoryDomain, read func(context.Context, *sql.Tx, *sqliteContext, string, string, string) (*siviParentProjection, error), authorize func(*PlotService) error) (*AuditRestoreResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*AuditRestoreResult, error) {
		if action != AuditRestoreCancel && action != AuditRestoreRetain && action != AuditRestorePrune {
			return nil, errors.New("SIVI parent restoration requires cancel, retain or prune")
		}
		if action == AuditRestoreCancel {
			return &AuditRestoreResult{Cancelled: true}, nil
		}
		if err := plots.requireContextEdit(); err != nil {
			return nil, err
		}
		if authorize != nil {
			if err := authorize(plots); err != nil {
				return nil, err
			}
		}
		ids, err := auditRowIDs([]string{historyID})
		if err != nil {
			return nil, err
		}
		owner := plots.projects.sqlite
		result := &AuditRestoreResult{}
		err = withSIVIParentWriteTransaction(ctx, owner, "parent restoration", func(tx *sql.Tx, suAlias string) error {
			if err := verifyTechnicalProvenance(ctx, tx, historyDomain.table, historyDomain.schema, historyDomain.label); err != nil {
				return err
			}
			var proposal string
			var restored *string
			if err := tx.QueryRowContext(ctx, `SELECT Proposal,Restored FROM `+quoteHeaderIdentifier(historyDomain.table)+` WHERE ID=?`, ids[0]).
				Scan(&proposal, &restored); err != nil {
				return err
			}
			if restored != nil {
				return errors.New("SIVI parent history was already restored; do not replay")
			}
			if !json.Valid([]byte(proposal)) {
				return errors.New("typed SIVI parent history requires one complete JSON value")
			}
			if err := validateMetadataDraftJSON([]byte(proposal)); err != nil {
				return err
			}
			decoder := json.NewDecoder(strings.NewReader(proposal))
			decoder.DisallowUnknownFields()
			var event siviParentHistory
			if err := decoder.Decode(&event); err != nil {
				return err
			}
			project := owner.selection.Project
			if event.EntryPlan != nil && !twoPageEntryProjectHistoryTable(historyDomain.table) {
				return errors.New("complete-entry source plans belong only to their exact entry history")
			}
			if historyDomain.table != siviProjectAssignmentHistoryTable && !twoPageEntryProjectHistoryTable(historyDomain.table) && event.ProjectAssignment != nil {
				return errors.New("ProjectID assignment provenance belongs only to its isolated history")
			}
			if _, err := historyDomain.assignments(ctx, event, project, plot); err != nil {
				return err
			}
			fresh, err := read(ctx, tx, owner, suAlias, contextID, plot)
			if err != nil {
				return err
			}
			expectedParent := *event.Committed
			expectedParent.ContextID = contextID
			if !reflect.DeepEqual(fresh, &expectedParent) {
				return errors.New("SIVI physical parents changed since the typed edit; no restoration or pruning")
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "main", project+"_Audit", "parent restoration audits"); err != nil {
				return err
			}
			before, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			expected := map[string]ProjectMetadataTable{}
			audit := before[project+"_Audit"]
			audit.Rows = append([]ProjectMetadataRow{}, audit.Rows...)
			auditColumns, err := siteUnitTransferColumns(audit, "Restore")
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
					planned.Rows = append([]ProjectMetadataRow{}, planned.Rows...)
				}
				columns, err := siteUnitTransferColumns(planned, change.Column)
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
				value, err := metadataCellValue(change.Before)
				if err != nil || change.Before.Storage == "blob" {
					return errors.Join(err, errors.New("typed SIVI parent restoration contains unsupported original storage"))
				}
				after, err := metadataCellValue(change.After)
				if err != nil {
					return err
				}
				record := change.Audit
				if seenAudits[record.RowID] || record.ID != nil || record.EditField != change.Column ||
					record.Restore || record.Flag || record.Project != project || record.PlotNumber != plot ||
					!metadataAuditTextEqual(record.BeforeEdit, value) || !metadataAuditTextEqual(record.AfterEdit, after) {
					return errors.New("typed SIVI parent audit differs from its physical identity/value")
				}
				seenAudits[record.RowID] = true
				auditIDs, err := auditRowIDs([]string{record.RowID})
				if err != nil {
					return err
				}
				actual, err := selectedAuditEntries(tx, project, plot, auditIDs)
				if err != nil {
					return err
				}
				normalize := func(entry AuditEntry) (AuditEntry, error) {
					if !strings.EqualFold(entry.Table, suffix) && !strings.EqualFold(entry.Table, project+suffix) {
						return AuditEntry{}, errors.New("SIVI parent audit has an unavailable restoration table alias")
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
				updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(change.Table)+
					` SET `+quoteHeaderIdentifier(change.Column)+`=? WHERE rowid=? AND typeof(`+quoteHeaderIdentifier(identity)+
					`)='text' AND `+quoteHeaderIdentifier(identity)+` COLLATE BINARY IS ?`, value, change.RowID, plot)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SIVI parent restoration did not affect exactly one physical row"))
				}
				planned.Rows[rowIndex].Cells = append([]ProjectMetadataCell{}, planned.Rows[rowIndex].Cells...)
				planned.Rows[rowIndex].Cells[columns[change.Column]] = cloneSiteUnitCell(change.Before)
				expected[change.Table] = planned
				query := `UPDATE ` + quoteHeaderIdentifier(project+"_Audit") + ` SET Restore=-1 WHERE rowid=?`
				if action == AuditRestorePrune {
					query = `DELETE FROM ` + quoteHeaderIdentifier(project+"_Audit") + ` WHERE rowid=?`
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
					if action == AuditRestorePrune {
						audit.Rows = append(audit.Rows[:i], audit.Rows[i+1:]...)
						result.PrunedAuditRows++
					} else {
						audit.Rows[i].Cells = append([]ProjectMetadataCell{}, row.Cells...)
						truth := "-1"
						audit.Rows[i].Cells[auditColumns["Restore"]] = ProjectMetadataCell{Storage: "integer", Integer: &truth}
					}
					break
				}
				result.RestoredRows++
			}
			expected[project+"_Audit"] = audit
			when := time.Now().UTC().Format(time.RFC3339Nano)
			updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(historyDomain.table)+` SET Restored=? WHERE ID=? AND Restored IS NULL`, when, ids[0])
			if err != nil {
				return err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("SIVI parent history restoration did not affect exactly one event"))
			}
			history := before[historyDomain.table]
			history.Rows = append([]ProjectMetadataRow{}, history.Rows...)
			historyColumns, err := siteUnitTransferColumns(history, "Restored")
			if err != nil {
				return err
			}
			for i, row := range history.Rows {
				if row.RowID == historyID {
					history.Rows[i].Cells = append([]ProjectMetadataCell{}, row.Cells...)
					history.Rows[i].Cells[historyColumns["Restored"]] = ProjectMetadataCell{Storage: "text", Text: &when}
				}
			}
			expected[historyDomain.table] = history
			after, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(before, after, expected, ""); err != nil {
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
