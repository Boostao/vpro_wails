package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type ProjectMetadataChange struct {
	Column string              `json:"column"`
	Value  ProjectMetadataCell `json:"value"`
}

type ProjectMetadataEdit struct {
	PlotNumber         string                  `json:"plotNumber"`
	ProjectID          *string                 `json:"projectId"`
	ID                 int64                   `json:"id"`
	Columns            []ProjectMetadataColumn `json:"columns"`
	Original           ProjectMetadataRow      `json:"original"`
	Changes            []ProjectMetadataChange `json:"changes"`
	StandardPopulation string                  `json:"standardPopulation"`
}

func (request *ProjectMetadataEdit) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("metadata draft JSON contains malformed UTF-8")
	}
	tokens := json.NewDecoder(bytes.NewReader(data))
	for {
		start := tokens.InputOffset()
		token, err := tokens.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if _, text := token.(string); text {
			raw := bytes.TrimSpace(data[start:tokens.InputOffset()])
			for len(raw) > 0 && (raw[0] == ',' || raw[0] == ':') {
				raw = bytes.TrimSpace(raw[1:])
			}
			if err := validateQualityJSONToken(raw); err != nil {
				return fmt.Errorf("metadata draft Unicode: %w", err)
			}
		}
	}
	type plain ProjectMetadataEdit
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
	for _, name := range []string{"plotNumber", "projectId", "id", "columns", "original", "changes", "standardPopulation"} {
		if _, present := properties[name]; !present {
			return fmt.Errorf("metadata draft requires explicit %s", name)
		}
		if name == "id" && bytes.Equal(bytes.TrimSpace(properties[name]), []byte("null")) {
			return errors.New("metadata physical ID cannot be NULL or defaulted")
		}
	}
	*request = ProjectMetadataEdit(decoded)
	return nil
}

func (c *sqliteContext) validateMetadataWriterFiles() error {
	projectInfo := c.attachmentInfo["project"]
	if projectInfo == nil || c.attachmentInfo["VLists"] == nil {
		return errors.New("metadata editing requires identified project and reference files")
	}
	for role, info := range c.attachmentInfo {
		current, err := os.Stat(c.attachments[role])
		if err != nil {
			return fmt.Errorf("metadata %s file unavailable: %w", role, err)
		}
		if !os.SameFile(current, info) {
			return fmt.Errorf("metadata %s file identity changed; reopen the context", role)
		}
		if strings.HasPrefix(role, "V") && os.SameFile(projectInfo, info) {
			return fmt.Errorf("metadata project file is also owned by %s; support-file writes are unavailable", role)
		}
	}
	return nil
}

func (c *sqliteContext) withMetadataWriter(ctx context.Context, operation func(*sql.Conn) error) (resultErr error) {
	if err := acquireMutexLease(ctx, &c.mu); err != nil {
		return err
	}
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("metadata context is closed")
	}
	if err := c.validateMetadataWriterFiles(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(c.attachments["project"], "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "reference"`, sqliteFileURI(c.attachments["VLists"], "ro")); err != nil {
		return fmt.Errorf("metadata reference attachment unavailable: %w", err)
	}
	return operation(conn)
}

func metadataReferenceVersion(ctx context.Context, tx *sql.Tx, table string) (ProjectMetadataCell, error) {
	rows, err := tx.QueryContext(ctx, `SELECT typeof(description),CAST(description AS BLOB)
		FROM reference._table_metadata WHERE table_name COLLATE BINARY=?`, table)
	if err != nil {
		return ProjectMetadataCell{}, fmt.Errorf("metadata version description unavailable: %w", err)
	}
	defer rows.Close()
	count := 0
	var result ProjectMetadataCell
	for rows.Next() {
		var storage string
		var raw []byte
		if err := rows.Scan(&storage, &raw); err != nil {
			return ProjectMetadataCell{}, err
		}
		if storage != "text" {
			return ProjectMetadataCell{}, errors.New("metadata version requires an explicit text table-object description")
		}
		result, err = projectMetadataCell(storage, raw)
		if err != nil {
			return ProjectMetadataCell{}, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return ProjectMetadataCell{}, err
	}
	if count != 1 {
		return ProjectMetadataCell{}, errors.New("metadata version description is missing or ambiguous; no Unknown fallback was written")
	}
	if err := validateChildPhysicalText("Metadata version", *result.Text, 255); err != nil {
		return ProjectMetadataCell{}, err
	}
	return result, nil
}

func metadataParentObservation(ctx context.Context, tx *sql.Tx, project, plot string, projectID *string) error {
	if err := childParent(tx, project, plot); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(project+"_Env")+
		` WHERE PlotNumber COLLATE BINARY=? AND typeof(ProjectID)='text' AND ProjectID COLLATE BINARY IS ?`, plot, projectID).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return errors.New("metadata parent ProjectID changed; reload before saving")
	}
	return nil
}

func metadataSelectedRow(ctx context.Context, tx *sql.Tx, project string, request ProjectMetadataEdit) (ProjectMetadataTable, ProjectMetadataRow, error) {
	table := quoteHeaderIdentifier(project + "_Metadata")
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE ID=?`, request.ID).Scan(&count); err != nil {
		return ProjectMetadataTable{}, ProjectMetadataRow{}, err
	}
	if count != 1 {
		return ProjectMetadataTable{}, ProjectMetadataRow{}, errors.New("metadata physical ID is missing or ambiguous across the project table")
	}
	result, err := readProjectMetadataRows(ctx, tx, "main", project+"_Metadata", request.ProjectID, true)
	if err != nil {
		return result, ProjectMetadataRow{}, err
	}
	if !reflect.DeepEqual(result.Columns, request.Columns) {
		return result, ProjectMetadataRow{}, errors.New("metadata schema changed; reload before saving")
	}
	idIndex, projectIndex := -1, -1
	for i, column := range result.Columns {
		if column.Name == "ID" {
			idIndex = i
		}
		if column.Name == "ProjectID" {
			projectIndex = i
		}
	}
	for _, row := range result.Rows {
		if row.RowID == request.Original.RowID && row.Cells[idIndex].Integer != nil &&
			*row.Cells[idIndex].Integer == strconv.FormatInt(request.ID, 10) {
			if projectIndex < 0 || row.Cells[projectIndex].Storage != "text" ||
				row.Cells[projectIndex].Text == nil || *row.Cells[projectIndex].Text != *request.ProjectID {
				return result, ProjectMetadataRow{}, errors.New("metadata stored ProjectID does not preserve the reviewed literal text identity")
			}
			return result, row, nil
		}
	}
	return result, ProjectMetadataRow{}, errors.New("metadata selected physical row is no longer owned by the reviewed ProjectID")
}

type metadataCommittedError struct{ cause error }

func (err *metadataCommittedError) Error() string {
	return "metadata edit committed, but writer cleanup failed; reload before retrying: " + err.cause.Error()
}

func (err *metadataCommittedError) Unwrap() error { return err.cause }

func (s *ContextService) SaveProjectMetadata(ctx context.Context, contextID string, request ProjectMetadataEdit) error {
	_, err := withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		return struct{}{}, plots.saveProjectMetadata(request)
	})
	return err
}

func (s *PlotService) saveProjectMetadata(request ProjectMetadataEdit) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	if request.PlotNumber == "" || !utf8.ValidString(request.PlotNumber) || request.ProjectID == nil ||
		*request.ProjectID == "" || !utf8.ValidString(*request.ProjectID) ||
		request.ID < -2147483648 || request.ID > 2147483647 {
		return errors.New("metadata editing requires a literal parent/ProjectID and exact signed32 physical ID")
	}
	rowID, err := strconv.ParseInt(request.Original.RowID, 10, 64)
	if err != nil || strconv.FormatInt(rowID, 10) != request.Original.RowID {
		return errors.New("metadata editing requires the exact reviewed physical rowid")
	}
	changes := make([]projectMetadataChange, len(request.Changes))
	for i, change := range request.Changes {
		changes[i] = projectMetadataChange{change.Column, change.Value}
	}
	assignments, err := prepareProjectMetadataChanges(request.Columns, request.Original, changes)
	if err != nil {
		return err
	}
	if request.StandardPopulation != "" && request.StandardPopulation != "keep" {
		return errors.New("metadata source-standard population is unavailable; only an explicit keep-other-fields decision is supported")
	}
	for _, assignment := range assignments {
		if assignment.column == "EcosysCollectionStandard" && assignment.after.Text != nil {
			standard := *assignment.after.Text
			if (len(standard) >= 4 && strings.EqualFold(standard[:4], "DEIF") ||
				len(standard) >= 3 && strings.EqualFold(standard[:3], "DTE") || strings.EqualFold(standard, "LMH25")) &&
				request.StandardPopulation != "keep" {
				return errors.New("metadata collection standard requires an explicit keep-other-fields decision; source-default population is not automatic")
			}
		}
	}
	c := s.projects.sqlite
	ctx := s.operationContext()
	user, strength := s.currentUser, s.auditStrength
	if err := validateChildPhysicalText("Metadata audit user", user, 255); err != nil {
		return err
	}
	committed := false
	err = c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
		var member bool
		if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
			request.PlotNumber).Scan(&member); err != nil || !member {
			return errors.Join(errors.New("metadata parent is outside the selected context"), err)
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
		project := c.selection.Project
		if err := metadataParentObservation(ctx, tx, project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		current, row, err := metadataSelectedRow(ctx, tx, project, request)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(row, request.Original) {
			return errors.New("metadata original row changed; reload before saving")
		}
		if len(assignments) == 0 {
			return nil
		}
		index := map[string]int{}
		for i, column := range current.Columns {
			index[column.Name] = i
		}
		planned := ProjectMetadataRow{RowID: row.RowID, Cells: append([]ProjectMetadataCell{}, row.Cells...)}
		for _, assignment := range assignments {
			field := projectMetadataFields[assignment.column]
			if field.limitToList && assignment.after.Text != nil {
				var registered bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reference.USysTableOfLists
					WHERE ListName COLLATE BINARY=? AND typeof(Note)='text' AND Note COLLATE BINARY=?)`,
					field.referenceList, *assignment.after.Text).Scan(&registered); err != nil {
					return fmt.Errorf("metadata quality reference unavailable: %w", err)
				}
				if !registered {
					return fmt.Errorf("metadata.%s requires a literal registered Note from %s", assignment.column, field.referenceList)
				}
			}
			planned.Cells[index[assignment.column]] = assignment.after
		}
		when := time.Now().Format("2006-01-02 15:04:05")
		stamps := map[string]ProjectMetadataCell{"DateLastEdited": {Storage: "text", Text: &when}}
		for column, table := range map[string]string{"AllSpecs": "USysAllSpecs", "TableOfLists": "USysTableOfLists"} {
			value, err := metadataReferenceVersion(ctx, tx, table)
			if err != nil {
				return err
			}
			stamps[column] = value
		}
		for column, value := range stamps {
			i, present := index[column]
			if !present {
				return fmt.Errorf("metadata source stamp %s is unavailable", column)
			}
			planned.Cells[i] = value
		}
		var fields []childField
		var before, after, args []any
		var sets []string
		for i, column := range current.Columns {
			if reflect.DeepEqual(row.Cells[i], planned.Cells[i]) {
				continue
			}
			old, err := metadataCellValue(row.Cells[i])
			if err != nil {
				return fmt.Errorf("metadata original %s cannot be audited: %w", column.Name, err)
			}
			if row.Cells[i].Storage == "blob" {
				return fmt.Errorf("metadata historical blob %s requires separately defined audit restoration; omit it unchanged", column.Name)
			}
			value, err := metadataCellValue(planned.Cells[i])
			if err != nil {
				return err
			}
			fields = append(fields, childField{column: column.Name})
			before, after = append(before, old), append(after, value)
			sets, args = append(sets, quoteHeaderIdentifier(column.Name)+"=?"), append(args, value)
		}
		table := quoteHeaderIdentifier(project + "_Metadata")
		result, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+strings.Join(sets, ",")+
			` WHERE rowid=? AND ID=? AND ProjectID COLLATE BINARY IS ?`, append(args, rowID, request.ID, request.ProjectID)...)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return errors.Join(fmt.Errorf("metadata mutation expected one row, found %d", affected), err)
		}
		if err := auditChildFields(tx, project, "Metadata", request.PlotNumber, request.ID, fields, before, after, user, strength, when); err != nil {
			return err
		}
		if err := metadataParentObservation(ctx, tx, project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		_, observed, err := metadataSelectedRow(ctx, tx, project, request)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(observed, planned) {
			return errors.New("metadata final stored row differs from the complete plan; mutation and audits rolled back")
		}
		if err := c.validateMetadataWriterFiles(); err != nil {
			return err
		}
		if err := c.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
			request.PlotNumber).Scan(&member); err != nil || !member {
			return errors.Join(errors.New("metadata parent left the selected context before commit"), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	})
	if err != nil && committed {
		return &metadataCommittedError{cause: err}
	}
	return err
}
