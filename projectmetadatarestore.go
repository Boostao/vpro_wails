package main

import (
	"bytes"
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

type ProjectMetadataRestoreReview struct {
	ContextID  string                  `json:"contextId"`
	HistoryID  string                  `json:"historyId"`
	Project    string                  `json:"project"`
	PlotNumber string                  `json:"plotNumber"`
	ProjectID  *string                 `json:"projectId"`
	ID         int64                   `json:"id"`
	Columns    []ProjectMetadataColumn `json:"columns"`
	Current    ProjectMetadataRow      `json:"current"`
	Restored   ProjectMetadataRow      `json:"restored"`
	Audits     []AuditEntry            `json:"audits"`
}

type ProjectMetadataRestore struct {
	Review    ProjectMetadataRestoreReview `json:"review"`
	Action    AuditRestoreAction           `json:"action"`
	Confirmed bool                         `json:"confirmed"`
}

type ProjectMetadataRestoreEvent struct {
	HistoryID string   `json:"historyId"`
	Created   string   `json:"created"`
	RowID     string   `json:"rowId"`
	ID        int64    `json:"id"`
	Fields    []string `json:"fields"`
	Restored  bool     `json:"restored"`
}

type ProjectMetadataRestoreHistory struct {
	HistoryPresent bool                          `json:"historyPresent"`
	Events         []ProjectMetadataRestoreEvent `json:"events"`
}

func (request *ProjectMetadataRestore) UnmarshalJSON(data []byte) error {
	if err := validateMetadataDraftJSON(data); err != nil {
		return err
	}
	type plain ProjectMetadataRestore
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value plain
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range []string{"review", "action", "confirmed"} {
		if raw, present := properties[name]; !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("metadata restoration requires explicit non-NULL %s", name)
		}
	}
	*request = ProjectMetadataRestore(value)
	return nil
}

func readProjectMetadataEditHistory(ctx context.Context, tx *sql.Tx, historyID string) (projectMetadataEditHistory, error) {
	var event projectMetadataEditHistory
	ids, err := auditRowIDs([]string{historyID})
	if err != nil {
		return event, err
	}
	if err := verifyTechnicalProvenance(ctx, tx, projectMetadataEditHistoryTable, projectMetadataEditHistorySQL, "metadata edit"); err != nil {
		return event, err
	}
	var raw string
	var restored *string
	if err := tx.QueryRowContext(ctx, `SELECT Proposal,Restored FROM __VPRO_MetadataEditHistory WHERE ID=?`, ids[0]).Scan(&raw, &restored); err != nil {
		return event, fmt.Errorf("typed metadata edit history unavailable; legacy plaintext audits cannot be inferred: %w", err)
	}
	if restored != nil {
		return event, errors.New("metadata edit was already restored; its completed transaction cannot be replayed")
	}
	return decodeProjectMetadataEditHistory(raw)
}

func decodeProjectMetadataEditHistory(raw string) (projectMetadataEditHistory, error) {
	var event projectMetadataEditHistory
	if err := validateMetadataDraftJSON([]byte(raw)); err != nil {
		return event, fmt.Errorf("typed metadata history Unicode/JSON invalid: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return event, fmt.Errorf("typed metadata history invalid: %w", err)
	}
	if len(event.Columns) == 0 || len(event.Original.Cells) != len(event.Columns) ||
		len(event.Committed.Cells) != len(event.Columns) || len(event.Audits) == 0 ||
		len(event.Audits) != len(event.Records) || event.Original.RowID != event.Committed.RowID {
		return event, errors.New("typed metadata history has incomplete schema, row or audit identities")
	}
	return event, nil
}

func metadataRestoreReview(ctx context.Context, tx *sql.Tx, project, plot, contextID, historyID string) (ProjectMetadataRestoreReview, error) {
	var review ProjectMetadataRestoreReview
	event, err := readProjectMetadataEditHistory(ctx, tx, historyID)
	if err != nil {
		return review, err
	}
	if event.Project != project || event.PlotNumber != plot || event.ProjectID == nil || *event.ProjectID == "" {
		return review, errors.New("typed metadata history is not owned by the selected parent/project")
	}
	if err := childParent(tx, project, plot); err != nil {
		return review, err
	}
	if err := metadataParentObservation(ctx, tx, project, plot, event.ProjectID); err != nil {
		return review, err
	}
	request := ProjectMetadataEdit{PlotNumber: plot, ProjectID: event.ProjectID, ID: event.ID,
		Columns: event.Columns, Original: event.Committed}
	table, current, err := metadataSelectedRow(ctx, tx, project, request)
	if err != nil {
		return review, err
	}
	if !reflect.DeepEqual(current, event.Committed) {
		return review, errors.New("metadata row changed since the typed edit; no restoration or history pruning")
	}
	rowIDs := make([]string, len(event.Audits))
	for i, audit := range event.Audits {
		rowIDs[i] = audit.RowID
	}
	ids, err := auditRowIDs(rowIDs)
	if err != nil {
		return review, err
	}
	records, err := selectedAuditEntries(tx, project, plot, ids)
	if err != nil {
		return review, err
	}
	restored := ProjectMetadataRow{RowID: current.RowID, Cells: append([]ProjectMetadataCell{}, current.Cells...)}
	index := map[string]int{}
	for i, column := range table.Columns {
		if _, exists := index[column.Name]; column.Name == "" || exists {
			return review, errors.New("metadata restoration requires unambiguous physical columns")
		}
		index[column.Name] = i
		if _, err := metadataCellValue(event.Original.Cells[i]); err != nil {
			return review, fmt.Errorf("typed metadata original %s invalid: %w", column.Name, err)
		}
	}
	seen := map[string]bool{}
	for i, record := range records {
		link := event.Audits[i]
		column, present := index[link.Column]
		if !present || seen[link.Column] || record.EditField != link.Column || record.ID == nil || *record.ID != event.ID {
			return review, errors.New("typed metadata restoration has ambiguous audit/field ownership")
		}
		seen[link.Column] = true
		expected := event.Records[i]
		if expected.Table != "_Metadata" || expected.User != event.User || expected.EditWhen != event.EditWhen ||
			expected.Restore || expected.Flag {
			return review, errors.New("typed metadata provenance does not preserve the original canonical audit observation")
		}
		if !strings.EqualFold(record.Table, "_Metadata") && !strings.EqualFold(record.Table, project+"_Metadata") {
			return review, errors.New("typed metadata audit table is not an allowed original/project-qualified alias")
		}
		normalized := record
		normalized.Table = expected.Table
		if !reflect.DeepEqual(normalized, expected) {
			return review, fmt.Errorf("metadata audit row %s changed since its typed mutation", record.RowID)
		}
		before, err := metadataCellValue(event.Original.Cells[column])
		if err != nil {
			return review, err
		}
		after, err := metadataCellValue(event.Committed.Cells[column])
		if err != nil {
			return review, err
		}
		if reflect.DeepEqual(event.Original.Cells[column], event.Committed.Cells[column]) ||
			!metadataAuditTextEqual(record.BeforeEdit, before) || !metadataAuditTextEqual(record.AfterEdit, after) {
			return review, errors.New("typed metadata audit does not match its exact storage-class change")
		}
		if err := validateMetadataRestorationValue(ctx, tx, link.Column, event.Original.Cells[column]); err != nil {
			return review, err
		}
		restored.Cells[column] = event.Original.Cells[column]
	}
	return ProjectMetadataRestoreReview{contextID, historyID, project, plot, event.ProjectID, event.ID,
		table.Columns, current, restored, records}, nil
}

func validateMetadataRestorationValue(ctx context.Context, tx *sql.Tx, column string, cell ProjectMetadataCell) error {
	field, editable := projectMetadataFields[column]
	if editable {
		if _, err := validateProjectMetadataAssignment(column, cell); err != nil {
			return err
		}
		if field.limitToList && cell.Text != nil {
			var registered bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reference.USysTableOfLists
				WHERE ListName COLLATE BINARY=? AND typeof(Note)='text' AND Note COLLATE BINARY=?)`,
				field.referenceList, *cell.Text).Scan(&registered); err != nil {
				return err
			}
			if !registered {
				return fmt.Errorf("metadata restoration %s requires its literal registered reference Note", column)
			}
		}
		return nil
	}
	switch column {
	case "AllSpecs", "TableOfLists", "DateLastEdited":
		value, err := metadataCellValue(cell)
		if err != nil {
			return err
		}
		if cell.Storage != "text" && cell.Storage != "null" {
			return fmt.Errorf("metadata stamp restoration %s requires its original nullable text storage", column)
		}
		return validateChildPhysicalText("Metadata."+column, value, 255)
	default:
		return fmt.Errorf("metadata restoration cannot assign identity/unsupported column %s", column)
	}
}

func (s *ContextService) ReviewProjectMetadataRestoration(ctx context.Context, contextID, plot, historyID string) (ProjectMetadataRestoreReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataRestoreReview, error) {
		var review ProjectMetadataRestoreReview
		c := plots.projects.sqlite
		err := c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			var member bool
			if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, plot).Scan(&member); err != nil || !member {
				return errors.Join(errors.New("metadata restoration parent is outside the selected context"), err)
			}
			review, err = metadataRestoreReview(ctx, tx, c.selection.Project, plot, contextID, historyID)
			return err
		})
		if err != nil {
			return ProjectMetadataRestoreReview{}, err
		}
		return review, nil
	})
}

func (s *ContextService) RestoreProjectMetadata(ctx context.Context, contextID string, request ProjectMetadataRestore) (*AuditRestoreResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*AuditRestoreResult, error) {
		if request.Action == AuditRestoreCancel {
			return &AuditRestoreResult{Cancelled: true}, nil
		}
		if !request.Confirmed || request.Review.ContextID != contextID ||
			request.Action != AuditRestoreRetain && request.Action != AuditRestorePrune {
			return nil, errors.New("metadata restoration requires explicit current-context review and retain/prune confirmation")
		}
		c := plots.projects.sqlite
		result := &AuditRestoreResult{}
		committed := false
		err := c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			var member bool
			if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
				request.Review.PlotNumber).Scan(&member); err != nil || !member {
				return errors.Join(errors.New("metadata restoration parent is outside the selected context"), err)
			}
			current, err := metadataRestoreReview(ctx, tx, c.selection.Project, request.Review.PlotNumber, contextID, request.Review.HistoryID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(current, request.Review) {
				return errors.New("metadata restoration review changed; no assignments or history pruning")
			}
			var sets []string
			var args []any
			for i, column := range current.Columns {
				if reflect.DeepEqual(current.Current.Cells[i], current.Restored.Cells[i]) {
					continue
				}
				value, err := metadataCellValue(current.Restored.Cells[i])
				if err != nil {
					return err
				}
				sets, args = append(sets, quoteHeaderIdentifier(column.Name)+"=?"), append(args, value)
				result.RestoredRows++
			}
			rowID, err := strconv.ParseInt(current.Current.RowID, 10, 64)
			if err != nil || strconv.FormatInt(rowID, 10) != current.Current.RowID || len(sets) == 0 {
				return errors.New("metadata restoration requires an exact physical rowid and actual audited changes")
			}
			updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(current.Project+"_Metadata")+
				` SET `+strings.Join(sets, ",")+` WHERE rowid=? AND ID=? AND ProjectID COLLATE BINARY IS ?`,
				append(args, rowID, current.ID, current.ProjectID)...)
			if err != nil {
				return err
			}
			if count, err := updated.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("metadata restoration lost its sole physical target"))
			}
			auditIDs := make([]string, len(current.Audits))
			for i, audit := range current.Audits {
				auditIDs[i] = audit.RowID
			}
			ids, err := auditRowIDs(auditIDs)
			if err != nil {
				return err
			}
			observedAudits, err := selectedAuditEntries(tx, current.Project, current.PlotNumber, ids)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(observedAudits, current.Audits) {
				return errors.New("metadata restoration changed its proven audit rows; mutation and pruning rolled back")
			}
			if request.Action == AuditRestorePrune {
				for _, audit := range current.Audits {
					deleted, err := tx.ExecContext(ctx, `DELETE FROM `+quoteHeaderIdentifier(current.Project+"_Audit")+
						` WHERE rowid=? AND Project COLLATE BINARY=? AND PlotNumber COLLATE BINARY=?`, audit.RowID, current.Project, current.PlotNumber)
					if err != nil {
						return err
					}
					if count, err := deleted.RowsAffected(); err != nil || count != 1 {
						return errors.Join(err, errors.New("metadata restoration lost a proven audit row"))
					}
					result.PrunedAuditRows++
				}
			}
			_, observed, err := metadataSelectedRow(ctx, tx, current.Project, ProjectMetadataEdit{
				PlotNumber: current.PlotNumber, ProjectID: current.ProjectID, ID: current.ID,
				Columns: current.Columns, Original: current.Current})
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(observed, current.Restored) {
				return errors.New("metadata restoration differs from its complete typed plan after audit pruning; all changes rolled back")
			}
			if err := metadataParentObservation(ctx, tx, current.Project, current.PlotNumber, current.ProjectID); err != nil {
				return err
			}
			if err := c.validateMetadataWriterFiles(); err != nil {
				return err
			}
			if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
				current.PlotNumber).Scan(&member); err != nil || !member {
				return errors.Join(errors.New("metadata restoration parent left the selected context before commit"), err)
			}
			receipt, err := json.Marshal(struct {
				Review ProjectMetadataRestoreReview `json:"review"`
				Action AuditRestoreAction           `json:"action"`
				User   string                       `json:"user"`
				When   string                       `json:"when"`
			}{current, request.Action, plots.currentUser, time.Now().UTC().Format(time.RFC3339Nano)})
			if err != nil {
				return err
			}
			recorded, err := tx.ExecContext(ctx, `UPDATE __VPRO_MetadataEditHistory SET Restored=? WHERE ID=? AND Restored IS NULL`,
				string(receipt), current.HistoryID)
			if err != nil {
				return err
			}
			if count, err := recorded.RowsAffected(); err != nil || count != 1 {
				return errors.Join(err, errors.New("metadata restoration could not seal its sole no-replay receipt"))
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			committed = true
			return nil
		})
		if err != nil {
			if committed {
				return nil, &metadataCommittedError{cause: err}
			}
			return nil, err
		}
		return result, nil
	})
}

func (s *ContextService) ListProjectMetadataRestoreHistory(ctx context.Context, contextID, plot string) (ProjectMetadataRestoreHistory, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataRestoreHistory, error) {
		result := ProjectMetadataRestoreHistory{Events: []ProjectMetadataRestoreEvent{}}
		c := plots.projects.sqlite
		err := c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			var member bool
			if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, plot).Scan(&member); err != nil || !member {
				return errors.Join(errors.New("metadata history parent is outside the selected context"), err)
			}
			tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE=?`,
				projectMetadataEditHistoryTable).Scan(&count); err != nil {
				return err
			}
			if count == 0 {
				return nil
			}
			if err := verifyTechnicalProvenance(ctx, tx, projectMetadataEditHistoryTable, projectMetadataEditHistorySQL, "metadata edit"); err != nil {
				return err
			}
			result.HistoryPresent = true
			rows, err := tx.QueryContext(ctx, `SELECT ID,Created,Proposal,Restored FROM __VPRO_MetadataEditHistory ORDER BY ID DESC`)
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
			for rows.Next() {
				var id int64
				var created, raw string
				var restored *string
				if err := rows.Scan(&id, &created, &raw, &restored); err != nil {
					return err
				}
				event, err := decodeProjectMetadataEditHistory(raw)
				if err != nil {
					return err
				}
				if event.Project != c.selection.Project || event.PlotNumber != plot {
					continue
				}
				fields := make([]string, len(event.Audits))
				for i, audit := range event.Audits {
					fields[i] = audit.Column
				}
				result.Events = append(result.Events, ProjectMetadataRestoreEvent{
					strconv.FormatInt(id, 10), created, event.Committed.RowID, event.ID, fields, restored != nil})
			}
			return rows.Err()
		})
		if err != nil {
			return ProjectMetadataRestoreHistory{}, err
		}
		return result, nil
	})
}
