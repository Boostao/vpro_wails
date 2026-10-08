package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

const siviProjectAssignmentHistoryTable = "__VPRO_SIVIProjectAssignmentHistory"
const siviProjectAssignmentHistorySQL = `CREATE TABLE "__VPRO_SIVIProjectAssignmentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviProjectAssignmentHistoryDomain = siviParentHistoryDomain{
	siviProjectAssignmentHistoryTable, siviProjectAssignmentHistorySQL, "SIVI ProjectID assignment", siviProjectAssignmentHistoryAssignments,
}

type siviProjectMetadataSchemaObject struct {
	Type, Name, Table string
	SQL               *string
}

type siviProjectAssignmentHistory struct {
	Selection    siviProjectSelection
	Choices      SIVIProjectChoices
	Columns      []ProjectMetadataColumn
	Schema       []siviProjectMetadataSchemaObject
	SchemaSHA256 string
}

func siviProjectAssignmentSchemaDigest(columns []ProjectMetadataColumn, schema []siviProjectMetadataSchemaObject) (string, error) {
	data, err := json.Marshal(struct {
		Columns []ProjectMetadataColumn
		Schema  []siviProjectMetadataSchemaObject
	}{columns, schema})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func siviProjectAssignmentAvailability(ctx context.Context, tx *sql.Tx, source int) (bool, string, error) {
	if source == 1 {
		return true, "Env metadata is guarded in the project transaction; Save independently rechecks membership and ownership.", nil
	}
	if source != 2 {
		return false, "", errors.New("SIVI ProjectID assignment requires source Env (1) or Master (2)")
	}
	var journal string
	if err := tx.QueryRowContext(ctx, `PRAGMA "VMetaData".journal_mode`).Scan(&journal); err != nil {
		return false, "", err
	}
	// A read-only WAL snapshot does not exclude a concurrent metadata commit.
	switch strings.ToLower(journal) {
	case "delete", "truncate", "persist":
		return true, "Observed master rollback-journal locking; Save independently rechecks metadata, membership and ownership.", nil
	default:
		return false, "SIVI ProjectID master requires rollback-journal read locks through commit", nil
	}
}

func readSIVIProjectAssignmentEvidence(ctx context.Context, owner *sqliteContext, tx *sql.Tx, contextID string, selection siviProjectSelection) (*siviProjectAssignmentHistory, error) {
	available, diagnostic, err := siviProjectAssignmentAvailability(ctx, tx, selection.SourceOption)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, errors.New(diagnostic)
	}
	choices, err := readSIVIProjectChoicesAtAlias(ctx, owner, tx, contextID, selection.SourceOption, "main")
	if err != nil {
		return nil, err
	}
	alias := "main"
	if selection.SourceOption == 2 {
		alias = "VMetaData"
	}
	table, err := readSQLiteStorageRows(ctx, tx, alias, choices.Table, "", nil, "")
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT type,name,tbl_name,sql FROM `+quoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE tbl_name COLLATE BINARY=? ORDER BY type COLLATE BINARY,name COLLATE BINARY`, choices.Table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schema := []siviProjectMetadataSchemaObject{}
	for rows.Next() {
		var object siviProjectMetadataSchemaObject
		if err := rows.Scan(&object.Type, &object.Name, &object.Table, &object.SQL); err != nil {
			return nil, err
		}
		schema = append(schema, object)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	digest, err := siviProjectAssignmentSchemaDigest(table.Columns, schema)
	if err != nil {
		return nil, err
	}
	return &siviProjectAssignmentHistory{selection, *choices, table.Columns, schema, digest}, nil
}

func (s *ContextService) writeSIVIProjectAssignment(ctx context.Context, contextID, plot string, original *siviParentProjection, selection siviProjectSelection) (*siviParentWriteResult, error) {
	return s.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, original, siviProjectAssignmentHistoryDomain, nil,
		s.siviProjectAssignmentWriteHooks(ctx, contextID, plot, selection))
}

func (s *ContextService) siviProjectAssignmentWriteHooks(ctx context.Context, contextID, plot string, selection siviProjectSelection) siviParentWriteHooks {
	var evidence *siviProjectAssignmentHistory
	var owner *sqliteContext
	var preferences *desktopConfig
	checkSource := func() error {
		values, err := preferences.readLocked()
		if err != nil {
			return err
		}
		source, err := configInt(values, "Current", "ProjectIdSource", 1, 2)
		if err != nil {
			return err
		}
		if source != selection.SourceOption {
			return errors.New("SIVI ProjectID preference changed; reload before writing")
		}
		return nil
	}
	return siviParentWriteHooks{
		prepare: func(conn *sql.Conn) (func(), error) {
			// The context request and owner leases precede the preference lease.
			// No snapshot/update call may reacquire that preference mutex.
			owner, preferences = s.projects.sqlite, s.projects.preferences
			if err := acquireMutexLease(ctx, &preferences.mu); err != nil {
				return nil, err
			}
			release := preferences.mu.Unlock
			if err := checkSource(); err != nil {
				release()
				return nil, err
			}
			if selection.SourceOption == 2 {
				if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "VMetaData"`, sqliteFileURI(owner.attachments["VMetaData"], "ro")); err != nil {
					release()
					return nil, err
				}
			}
			return release, nil
		},
		plan: func(tx *sql.Tx, observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			var err error
			evidence, err = readSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, selection)
			if err != nil {
				return nil, err
			}
			env := ProjectMetadataTable{Columns: observed.EnvColumns, Rows: []ProjectMetadataRow{observed.Rows[0].Env}}
			admin := ProjectMetadataTable{Columns: observed.AdminColumns, Rows: []ProjectMetadataRow{observed.Rows[0].Admin}}
			planned, err := planSIVIProjectAssignment(ctx, contextID, observed.Project, plot, env, admin, evidence.Choices, selection)
			if err != nil {
				return nil, err
			}
			return planned.Assignments, nil
		},
		verify: func(tx *sql.Tx) error {
			if err := checkSource(); err != nil {
				return err
			}
			fresh, err := readSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, selection)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(evidence, fresh) {
				return errors.New("SIVI ProjectID metadata or schema changed inside the assignment transaction")
			}
			return nil
		},
		decorate: func(event *siviParentHistory) { event.ProjectAssignment = evidence },
	}
}

func siviProjectAssignmentHistoryAssignments(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
	original, err := validateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	evidence := event.ProjectAssignment
	if evidence == nil || len(event.Changes) != 1 {
		return nil, errors.New("typed ProjectID history requires one audited assignment and its original source evidence")
	}
	digest, err := siviProjectAssignmentSchemaDigest(evidence.Columns, evidence.Schema)
	if err != nil || digest != evidence.SchemaSHA256 {
		return nil, errors.Join(err, errors.New("typed ProjectID history metadata schema evidence is incomplete or changed"))
	}
	change := event.Changes[0]
	if change.Column != "ProjectID" || change.Table != original.EnvTable ||
		change.RowID != original.Rows[0].Env.RowID || evidence.Selection.ContextID != original.ContextID ||
		!reflect.DeepEqual(change.Before, evidence.Selection.Expected) ||
		len(evidence.Schema) == 0 {
		return nil, errors.New("typed ProjectID history changed its source or parent identity")
	}
	tableObject := false
	for _, object := range evidence.Schema {
		if object.Table != evidence.Choices.Table || object.Name == "" ||
			(object.Type != "table" && object.Type != "index" && object.Type != "trigger") {
			return nil, errors.New("typed ProjectID history contains malformed metadata schema provenance")
		}
		if object.Type == "table" {
			if tableObject || object.Name != object.Table || object.SQL == nil || *object.SQL == "" {
				return nil, errors.New("typed ProjectID history requires one physical metadata schema")
			}
			tableObject = true
		}
	}
	projected, err := projectSIVIProjectChoices(ctx, ProjectMetadataTable{Columns: evidence.Columns})
	if err != nil || !tableObject || !reflect.DeepEqual(projected.Columns, evidence.Choices.Choices.Columns) {
		return nil, errors.Join(err, errors.New("typed ProjectID history schema differs from its projected choices"))
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	planned, err := planSIVIProjectAssignment(ctx, original.ContextID, project, plot, env, admin, evidence.Choices, evidence.Selection)
	if err != nil || len(planned.Assignments) != 1 || !reflect.DeepEqual(planned.Assignments[0].After, change.After) ||
		planned.Assignments[0].Table != change.Table || planned.Assignments[0].RowID != change.RowID {
		return nil, errors.Join(err, errors.New("typed ProjectID history differs from its physical choice assignment"))
	}
	columns, err := siteUnitTransferColumns(env, "ProjectID")
	if err != nil {
		return nil, err
	}
	env.Rows = append([]ProjectMetadataRow{}, env.Rows...)
	env.Rows[0].Cells = append([]ProjectMetadataCell{}, env.Rows[0].Cells...)
	env.Rows[0].Cells[columns["ProjectID"]] = cloneSiteUnitCell(change.After)
	committed, err := projectSIVIParent(ctx, original.ContextID, project, plot, env, admin)
	if err != nil || !reflect.DeepEqual(committed, event.Committed) {
		return nil, errors.Join(err, errors.New("typed ProjectID history changed unrelated parent values"))
	}
	return planned.Assignments, nil
}

func (s *ContextService) restoreSIVIProjectAssignment(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreSIVIParentHistory(ctx, contextID, plot, historyID, action, siviProjectAssignmentHistoryDomain)
}
