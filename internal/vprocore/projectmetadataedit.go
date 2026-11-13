package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

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

func ValidateMetadataDraftJSON(data []byte) error {
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
			if err := sqlutil.ValidateQualityJSONToken(raw); err != nil {
				return fmt.Errorf("metadata draft Unicode: %w", err)
			}
		}
	}
	return nil
}

func (request *ProjectMetadataEdit) UnmarshalJSON(data []byte) error {
	if err := ValidateMetadataDraftJSON(data); err != nil {
		return err
	}
	type plain ProjectMetadataEdit
	var decoded plain
	if err := sqlutil.DecodeStrictJSON(data, &decoded); err != nil {
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

func (c *SqliteContext) ValidateMetadataWriterFiles() error {
	projectInfo := c.AttachmentInfo["project"]
	if projectInfo == nil || c.AttachmentInfo["VLists"] == nil {
		return errors.New("metadata editing requires identified project and reference files")
	}
	for role, info := range c.AttachmentInfo {
		current, err := os.Stat(c.Attachments[role])
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

func (c *SqliteContext) WithMetadataWriter(ctx context.Context, operation func(*sql.Conn) error) (resultErr error) {
	return c.WithSelectedFileWriter(ctx, "project", c.ValidateMetadataWriterFiles, operation)
}

func (c *SqliteContext) WithSelectedFileWriter(ctx context.Context, role string, validate func() error,
	operation func(*sql.Conn) error) (resultErr error) {
	if err := sqlutil.AcquireMutexLease(ctx, &c.Mu); err != nil {
		return err
	}
	defer c.Mu.Unlock()
	if c.Conn == nil {
		return errors.New("metadata context is closed")
	}
	if err := validate(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(c.Attachments[role], "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
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
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "reference"`, sqlutil.SqliteFileURI(c.Attachments["VLists"], "ro")); err != nil {
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
		result, err = ProjectMetadataCellInternal(storage, raw)
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
	if err := ValidateChildPhysicalText("Metadata version", *result.Text, 255); err != nil {
		return ProjectMetadataCell{}, err
	}
	return result, nil
}

func MetadataParentObservation(ctx context.Context, tx *sql.Tx, project, plot string, projectID *string) error {
	if err := ChildParent(tx, project, plot); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(project+"_Env")+
		` WHERE PlotNumber COLLATE BINARY=? AND typeof(ProjectID)='text' AND ProjectID COLLATE BINARY IS ?`, plot, projectID).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return errors.New("metadata parent ProjectID changed; reload before saving")
	}
	return nil
}

func MetadataSelectedRow(ctx context.Context, tx *sql.Tx, project string, request ProjectMetadataEdit) (ProjectMetadataTable, ProjectMetadataRow, error) {
	table := QuoteHeaderIdentifier(project + "_Metadata")
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE ID=?`, request.ID).Scan(&count); err != nil {
		return ProjectMetadataTable{}, ProjectMetadataRow{}, err
	}
	if count != 1 {
		return ProjectMetadataTable{}, ProjectMetadataRow{}, errors.New("metadata physical ID is missing or ambiguous across the project table")
	}
	result, err := ReadProjectMetadataRows(ctx, tx, "main", project+"_Metadata", request.ProjectID, true)
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

type MetadataCommittedError struct {
	Cause error `json:"-" yaml:"-"`
}

func (err *MetadataCommittedError) Error() string {
	return "metadata edit committed, but writer cleanup failed; reload before retrying: " + err.Cause.Error()
}

func (err *MetadataCommittedError) Unwrap() error { return err.Cause }

func (s *ContextService) SaveProjectMetadata(ctx context.Context, contextID string, request ProjectMetadataEdit) error {
	_, err := WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		return struct{}{}, plots.saveProjectMetadata(request)
	})
	return err
}

func (s *PlotService) saveProjectMetadata(request ProjectMetadataEdit) error {
	if err := s.RequireContextEdit(); err != nil {
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
	changes := make([]ProjectMetadataChangeInternal, len(request.Changes))
	for i, change := range request.Changes {
		changes[i] = ProjectMetadataChangeInternal{change.Column, change.Value}
	}
	assignments, err := PrepareProjectMetadataChanges(request.Columns, request.Original, changes)
	if err != nil {
		return err
	}
	if request.StandardPopulation != "" && request.StandardPopulation != "keep" && request.StandardPopulation != "populate" {
		return errors.New("metadata source-standard decision must explicitly keep current drafts or confirm population")
	}
	recognizedStandardChanged := false
	for _, assignment := range assignments {
		if assignment.Column == "EcosysCollectionStandard" && assignment.after.Text != nil {
			standard := *assignment.after.Text
			recognized := len(standard) >= 4 && strings.EqualFold(standard[:4], "DEIF") ||
				len(standard) >= 3 && strings.EqualFold(standard[:3], "DTE") || strings.EqualFold(standard, "LMH25")
			if recognized && request.StandardPopulation == "" {
				return errors.New("metadata collection standard requires an explicit keep-current-drafts or confirmed population decision; source-default population is not automatic")
			}
			recognizedStandardChanged = recognizedStandardChanged || recognized
		}
	}
	if request.StandardPopulation == "populate" && !recognizedStandardChanged {
		return errors.New("metadata confirmed population requires a changed recognized collection standard")
	}
	c := s.Projects.Sqlite
	ctx := s.OperationContext()
	user, strength := s.CurrentUser, s.AuditStrength
	if err := ValidateChildPhysicalText("Metadata audit user", user, 255); err != nil {
		return err
	}
	committed := false
	err = c.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
		var member bool
		if err := c.Conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
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
		project := c.Selection.Project
		if err := MetadataParentObservation(ctx, tx, project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		current, row, err := MetadataSelectedRow(ctx, tx, project, request)
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
			field := ProjectMetadataFields[assignment.Column]
			if field.LimitToList && assignment.after.Text != nil {
				var registered bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reference.USysTableOfLists
					WHERE ListName COLLATE BINARY=? AND typeof(Note)='text' AND Note COLLATE BINARY=?)`,
					field.ReferenceList, *assignment.after.Text).Scan(&registered); err != nil {
					return fmt.Errorf("metadata quality reference unavailable: %w", err)
				}
				if !registered {
					return fmt.Errorf("metadata.%s requires a literal registered Note from %s", assignment.Column, field.ReferenceList)
				}
			}
			planned.Cells[index[assignment.Column]] = assignment.after
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
		var fields []ChildField
		var before, after, args []any
		var sets []string
		for i, column := range current.Columns {
			if reflect.DeepEqual(row.Cells[i], planned.Cells[i]) {
				continue
			}
			old, err := MetadataCellValue(row.Cells[i])
			if err != nil {
				return fmt.Errorf("metadata original %s cannot be audited: %w", column.Name, err)
			}
			if row.Cells[i].Storage == "blob" {
				return fmt.Errorf("metadata historical blob %s requires separately defined audit restoration; omit it unchanged", column.Name)
			}
			value, err := MetadataCellValue(planned.Cells[i])
			if err != nil {
				return err
			}
			fields = append(fields, ChildField{Column: column.Name})
			before, after = append(before, old), append(after, value)
			sets, args = append(sets, QuoteHeaderIdentifier(column.Name)+"=?"), append(args, value)
		}
		table := QuoteHeaderIdentifier(project + "_Metadata")
		result, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+strings.Join(sets, ",")+
			` WHERE rowid=? AND ID=? AND ProjectID COLLATE BINARY IS ?`, append(args, rowID, request.ID, request.ProjectID)...)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return errors.Join(fmt.Errorf("metadata mutation expected one row, found %d", affected), err)
		}
		audits, err := AuditChildFieldsTracked(tx, project, "Metadata", request.PlotNumber, request.ID, fields, before, after, user, strength, when)
		if err != nil {
			return err
		}
		if err := MetadataParentObservation(ctx, tx, project, request.PlotNumber, request.ProjectID); err != nil {
			return err
		}
		_, observed, err := MetadataSelectedRow(ctx, tx, project, request)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(observed, planned) {
			return errors.New("metadata final stored row differs from the complete plan; mutation and audits rolled back")
		}
		if err := appendProjectMetadataEditHistory(ctx, tx, ProjectMetadataEditHistory{
			Project: project, PlotNumber: request.PlotNumber, ProjectID: request.ProjectID, ID: request.ID,
			User: user, EditWhen: when, Columns: current.Columns, Original: row, Committed: observed, Audits: audits,
		}); err != nil {
			return err
		}
		if err := c.ValidateMetadataWriterFiles(); err != nil {
			return err
		}
		if err := c.Conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`,
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
		return &MetadataCommittedError{Cause: err}
	}
	return err
}
