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
	"unicode/utf8"
)

type ProjectMetadataCreate struct {
	PlotNumber string               `json:"plotNumber"`
	ProjectID  *string              `json:"projectId"`
	Original   ProjectMetadataTable `json:"original"`
}

func (request *ProjectMetadataCreate) UnmarshalJSON(data []byte) error {
	if err := validateMetadataDraftJSON(data); err != nil {
		return err
	}
	type plain ProjectMetadataCreate
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var decoded plain
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range []string{"plotNumber", "projectId", "original"} {
		if _, present := properties[name]; !present {
			return fmt.Errorf("blank metadata proposal requires explicit %s", name)
		}
	}
	*request = ProjectMetadataCreate(decoded)
	return nil
}

func (s *ContextService) CreateBlankProjectMetadata(ctx context.Context, contextID string, request ProjectMetadataCreate) (ProjectMetadataRow, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataRow, error) {
		return plots.createBlankProjectMetadata(request)
	})
}

func validateBlankMetadataSchema(columns []ProjectMetadataColumn) error {
	if len(columns) != len(projectMetadataFields)+5 {
		return errors.New("blank metadata requires the complete source75-column schema; no unknown field defaults were inferred")
	}
	seen := map[string]bool{}
	for _, column := range columns {
		_, ordinary := projectMetadataFields[column.Name]
		managed := column.Name == "ID" || column.Name == "ProjectID" || column.Name == "AllSpecs" ||
			column.Name == "TableOfLists" || column.Name == "DateLastEdited"
		if seen[column.Name] || !ordinary && !managed {
			return fmt.Errorf("blank metadata source column %q is unknown or repeated", column.Name)
		}
		seen[column.Name] = true
	}
	return nil
}

func reserveMetadataIdentities(ctx context.Context, tx *sql.Tx, project string) error {
	table := quoteHeaderIdentifier(project + "_Metadata")
	rows, err := tx.QueryContext(ctx, `SELECT typeof(ID),ID FROM `+table+`
		UNION ALL SELECT typeof(ID),ID FROM `+quoteHeaderIdentifier(project+"_Audit")+`
		WHERE "Table" COLLATE NOCASE IN (?,?)`, "_Metadata", project+"_Metadata")
	if err != nil {
		return err
	}
	ids := map[int64]bool{}
	for rows.Next() {
		var storage string
		var value sql.NullInt64
		if err := rows.Scan(&storage, &value); err != nil {
			rows.Close()
			return err
		}
		if storage != "integer" || !value.Valid || value.Int64 < -2147483648 || value.Int64 > 2147483647 {
			rows.Close()
			return errors.New("metadata stored/audited identity cannot be safely reserved; creation made no changes")
		}
		ids[value.Int64] = true
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID"))`); err != nil {
		return err
	}
	for id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable","ID") VALUES (?,?)
			ON CONFLICT ("ChildTable","ID") DO NOTHING`, table, id); err != nil {
			return err
		}
	}
	return nil
}

func metadataCreationParents(ctx context.Context, tx *sql.Tx, project, plot string) ([]string, error) {
	snapshot := []string{}
	for _, kind := range []string{"Env", "Admin"} {
		table := quoteHeaderIdentifier(project + "_" + kind)
		rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
		if err != nil {
			return nil, err
		}
		fields := []string{"CAST(rowid AS TEXT)"}
		for rows.Next() {
			var cid, required, primary int
			var name, declared string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &declared, &required, &defaultValue, &primary); err != nil {
				rows.Close()
				return nil, err
			}
			snapshot = append(snapshot, table, name, declared)
			column := quoteHeaderIdentifier(name)
			fields = append(fields, "typeof("+column+")", "hex(CAST("+column+" AS BLOB))")
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
		values, targets := make([]string, len(fields)), make([]any, len(fields))
		for i := range values {
			targets[i] = &values[i]
		}
		identity := "PlotNumber"
		if kind == "Admin" {
			identity = "Plot"
		}
		if err := tx.QueryRowContext(ctx, "SELECT "+strings.Join(fields, ",")+" FROM "+table+
			" WHERE "+quoteHeaderIdentifier(identity)+" COLLATE BINARY=?", plot).Scan(targets...); err != nil {
			return nil, err
		}
		snapshot = append(snapshot, values...)
	}
	return snapshot, nil
}

func (s *PlotService) createBlankProjectMetadata(request ProjectMetadataCreate) (created ProjectMetadataRow, resultErr error) {
	return s.createProjectMetadata(request, nil)
}

func (s *PlotService) createProjectMetadata(request ProjectMetadataCreate, template *ProjectMetadataTemplateCreate) (created ProjectMetadataRow, resultErr error) {
	if err := s.requireContextEdit(); err != nil {
		return created, err
	}
	if request.PlotNumber == "" || !utf8.ValidString(request.PlotNumber) || request.ProjectID == nil || *request.ProjectID == "" {
		return created, errors.New("blank metadata requires the selected plot's existing literal ProjectID; parent identity is never inferred or changed")
	}
	if err := validateChildPhysicalText("Metadata.ProjectID", *request.ProjectID, 20); err != nil {
		return created, err
	}
	if request.Original.Rows == nil || len(request.Original.Rows) != 0 {
		return created, errors.New("blank metadata requires an explicit empty matching-record review; existing candidates must be selected instead")
	}
	if err := validateBlankMetadataSchema(request.Original.Columns); err != nil {
		return created, err
	}
	c, ctx := s.projects.sqlite, s.operationContext()
	user, strength := s.currentUser, s.auditStrength
	if err := validateChildPhysicalText("Metadata audit user", user, 255); err != nil {
		return created, err
	}
	committed := false
	resultErr = c.withMetadataWriter(ctx, func(conn *sql.Conn) (err error) {
		var member bool
		if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, request.PlotNumber).Scan(&member); err != nil || !member {
			return errors.Join(errors.New("blank metadata parent is outside the selected context"), err)
		}
		if template != nil {
			if c.attachmentInfo["VMetaData"] == nil {
				return errors.New("template creation requires the identified readonly master metadata database")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "metadataMaster"`,
				sqliteFileURI(c.attachments["VMetaData"], "ro")); err != nil {
				return fmt.Errorf("readonly metadata master attachment unavailable: %w", err)
			}
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() {
			if rollback := tx.Rollback(); rollback != nil && !errors.Is(rollback, sql.ErrTxDone) {
				err = errors.Join(err, rollback)
			}
		}()
		if err := metadataParentObservation(ctx, tx, c.selection.Project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		parents, err := metadataCreationParents(ctx, tx, c.selection.Project, request.PlotNumber)
		if err != nil {
			return err
		}
		current, err := readProjectMetadataRows(ctx, tx, "main", c.selection.Project+"_Metadata", request.ProjectID, true)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, request.Original) {
			return errors.New("blank metadata schema/candidates changed; reload before creation")
		}
		var templateValues []ProjectMetadataChange
		if template != nil {
			templateValues, err = metadataTemplatePlan(ctx, tx, *template)
			if err != nil {
				return err
			}
		}
		if err := reserveMetadataIdentities(ctx, tx, c.selection.Project); err != nil {
			return err
		}
		table := quoteHeaderIdentifier(c.selection.Project + "_Metadata")
		id, err := allocateChildID(tx, table)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable","ID") VALUES (?,?)`, table, id); err != nil {
			return err
		}
		cells := make([]ProjectMetadataCell, len(current.Columns))
		names, values := make([]string, len(cells)), make([]any, len(cells))
		proposed := map[string]ProjectMetadataCell{}
		for _, change := range templateValues {
			proposed[change.Column] = change.Value
		}
		for i, column := range current.Columns {
			names[i] = quoteHeaderIdentifier(column.Name)
			cells[i].Storage = "null"
			if column.Name == "ID" {
				exact := strconv.FormatInt(id, 10)
				cells[i] = ProjectMetadataCell{Storage: "integer", Integer: &exact}
				values[i] = id
			} else if column.Name == "ProjectID" {
				cells[i] = ProjectMetadataCell{Storage: "text", Text: request.ProjectID}
				values[i] = *request.ProjectID
			} else if value, present := proposed[column.Name]; present {
				cells[i] = value
				values[i], err = validateProjectMetadataAssignment(column.Name, value)
				if err != nil {
					return err
				}
			}
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO `+table+` (`+strings.Join(names, ",")+`) VALUES (`+
			strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")+`)`, values...)
		if err != nil {
			return err
		}
		rowID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		planned := ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10), Cells: cells}
		when := time.Now().Format("2006-01-02 15:04:05")
		if strength > 0 {
			snapshot, err := json.Marshal(ProjectMetadataTable{Columns: current.Columns, Rows: []ProjectMetadataRow{planned}})
			if err != nil {
				return err
			}
			audit, err := tx.ExecContext(ctx, `INSERT INTO `+quoteHeaderIdentifier(c.selection.Project+"_Audit")+
				` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID")
				VALUES (?,?,?,'_Metadata','CreateRecord',?,NULL,?,0,0,?)`, c.selection.Project, user,
				request.PlotNumber, when, string(snapshot), id)
			if err != nil {
				return err
			}
			auditID, err := audit.LastInsertId()
			if err != nil {
				return err
			}
			var valid bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+quoteHeaderIdentifier(c.selection.Project+"_Audit")+
				` WHERE rowid=? AND Project COLLATE BINARY=? AND User COLLATE BINARY=? AND PlotNumber COLLATE BINARY=?
				AND "Table"='_Metadata' AND EditField='CreateRecord' AND EditWhen=? AND BeforeEdit IS NULL
				AND typeof(AfterEdit)='text' AND AfterEdit COLLATE BINARY=? AND Restore=0 AND Flag=0 AND ID=?)`,
				auditID, c.selection.Project, user, request.PlotNumber, when, string(snapshot), id).Scan(&valid); err != nil || !valid {
				return errors.Join(errors.New("blank metadata creation audit differs from the complete plan"), err)
			}
		}
		observed, err := readProjectMetadataRows(ctx, tx, "main", c.selection.Project+"_Metadata", request.ProjectID, true)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(observed, ProjectMetadataTable{Columns: current.Columns, Rows: []ProjectMetadataRow{planned}}) {
			return errors.New("blank metadata final row/schema/candidates differ from the explicit plan; creation and audit rolled back")
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE ID=?`, id).Scan(&count); err != nil || count != 1 {
			return errors.Join(errors.New("blank metadata allocated identity became ambiguous"), err)
		}
		if err := metadataParentObservation(ctx, tx, c.selection.Project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		finalParents, err := metadataCreationParents(ctx, tx, c.selection.Project, request.PlotNumber)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(parents, finalParents) {
			return errors.New("blank metadata creation changed parent storage/schema/scope; the complete transaction rolled back")
		}
		if template != nil {
			finalValues, err := metadataTemplatePlan(ctx, tx, *template)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(templateValues, finalValues) {
				return errors.New("metadata template assignments changed; creation and audit rolled back")
			}
		}
		if err := c.validateMetadataWriterFiles(); err != nil {
			return err
		}
		if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, request.PlotNumber).Scan(&member); err != nil || !member {
			return errors.Join(errors.New("blank metadata parent left selected context"), err)
		}
		var reserved bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM "__VPRO_ChildIdentity" WHERE ChildTable=? AND ID=?)`, table, id).Scan(&reserved); err != nil || !reserved {
			return errors.Join(errors.New("blank metadata identity reservation changed"), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		created = planned
		return nil
	})
	if resultErr != nil && committed {
		resultErr = &metadataCommittedError{cause: resultErr}
	}
	return created, resultErr
}
