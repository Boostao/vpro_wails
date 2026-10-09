package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ProjectMetadataColumn struct {
	Name         string `json:"name"`
	DeclaredType string `json:"declaredType"`
}

type ProjectMetadataCell struct {
	Storage string   `json:"storage"`
	Text    *string  `json:"text"`
	Integer *string  `json:"integer"`
	Real    *float64 `json:"real"`
	BlobHex *string  `json:"blobHex"`
}

type ProjectMetadataRow struct {
	RowID string                `json:"rowId"`
	Cells []ProjectMetadataCell `json:"cells"`
}

type ProjectMetadataTable struct {
	Columns []ProjectMetadataColumn `json:"columns"`
	Rows    []ProjectMetadataRow    `json:"rows"`
}

type ProjectMetadataReview struct {
	Project         string               `json:"project"`
	PlotNumber      string               `json:"plotNumber"`
	ProjectID       *string              `json:"projectId"`
	ProjectRecords  ProjectMetadataTable `json:"projectRecords"`
	MasterTemplates ProjectMetadataTable `json:"masterTemplates"`
}

func (s *ContextService) ReviewProjectMetadata(ctx context.Context, contextID, plot string) (ProjectMetadataReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataReview, error) {
		if plot == "" || !utf8.ValidString(plot) {
			return ProjectMetadataReview{}, errors.New("metadata review requires a literal valid plot identity")
		}
		c := plots.projects.sqlite
		env := `"project".` + quoteHeaderIdentifier(c.selection.Project+"_Env")
		rows, err := c.conn.QueryContext(ctx, `SELECT CAST(ProjectID AS BLOB),typeof(ProjectID) FROM `+env+
			` WHERE PlotNumber COLLATE BINARY=? AND EXISTS(SELECT 1 FROM USysEnv WHERE PlotNumber COLLATE BINARY=?)`, plot, plot)
		if err != nil {
			return ProjectMetadataReview{}, fmt.Errorf("metadata parent scope unavailable: %w", err)
		}
		var projectID *string
		count := 0
		for rows.Next() {
			var raw []byte
			var storage string
			if err := rows.Scan(&raw, &storage); err != nil {
				rows.Close()
				return ProjectMetadataReview{}, err
			}
			if storage != "text" && storage != "null" || !utf8.Valid(raw) {
				rows.Close()
				return ProjectMetadataReview{}, errors.New("metadata parent ProjectID has unsupported historical storage; no identity was repaired")
			}
			if storage == "text" {
				value := string(raw)
				projectID = &value
			}
			count++
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return ProjectMetadataReview{}, fmt.Errorf("metadata parent read failed: %w", err)
		}
		if count != 1 {
			return ProjectMetadataReview{}, errors.New("metadata parent is unavailable or ambiguous in the selected context")
		}
		project, err := readProjectMetadataTable(ctx, c, "project", c.selection.Project+"_Metadata", projectID, true)
		if err != nil {
			return ProjectMetadataReview{}, fmt.Errorf("project metadata review unavailable: %w", err)
		}
		master, err := readProjectMetadataTable(ctx, c, "VMetaData", "ProjectMetaData", projectID, false)
		if err != nil {
			return ProjectMetadataReview{}, fmt.Errorf("master metadata review unavailable: %w", err)
		}
		return ProjectMetadataReview{c.selection.Project, plot, projectID, project, master}, nil
	})
}

func readProjectMetadataTable(ctx context.Context, c *sqliteContext, alias, table string, projectID *string, project bool) (ProjectMetadataTable, error) {
	required := []string{"ProjectID"}
	if project {
		required = append(required, "ID")
	}
	if err := c.requireColumns(ctx, alias, table, required); err != nil {
		return ProjectMetadataTable{}, err
	}
	return readProjectMetadataRows(ctx, c.conn, alias, table, projectID, project)
}

type projectMetadataQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readProjectMetadataRows(ctx context.Context, db projectMetadataQueryer, alias, table string, projectID *string, project bool) (ProjectMetadataTable, error) {
	result, err := readSQLiteStorageRows(ctx, db, alias, table, "ProjectID", projectID, "")
	if err != nil || !project {
		return result, err
	}
	idIndex := -1
	for index, column := range result.Columns {
		if column.Name == "ID" {
			idIndex = index
		}
	}
	if idIndex < 0 {
		return ProjectMetadataTable{}, errors.New("project metadata requires its exact physical ID column")
	}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		identity := row.Cells[idIndex]
		if identity.Storage != "integer" || identity.Integer == nil {
			return ProjectMetadataTable{}, errors.New("project metadata has an unsupported physical ID")
		}
		id, err := strconv.ParseInt(*identity.Integer, 10, 32)
		if err != nil || seen[*identity.Integer] {
			return ProjectMetadataTable{}, errors.New("project metadata has an invalid or ambiguous signed32 ID")
		}
		seen[strconv.FormatInt(id, 10)] = true
	}
	return result, nil
}

// Filter/order identifiers are internal literals; values remain SQL parameters.
func readSQLiteStorageRows(ctx context.Context, db projectMetadataQueryer, alias, table, filterColumn string, filterValue *string, orderColumn string) (ProjectMetadataTable, error) {
	return readSQLiteStorageRowsScoped(ctx, db, alias, table, filterColumn, filterValue, orderColumn, false)
}

func readSQLiteLiteralTextRows(ctx context.Context, db projectMetadataQueryer, alias, table, filterColumn, filterValue, orderColumn string) (ProjectMetadataTable, error) {
	if filterColumn == "" {
		return ProjectMetadataTable{}, errors.New("literal text storage scope requires an explicit column")
	}
	return readSQLiteStorageRowsScoped(ctx, db, alias, table, filterColumn, &filterValue, orderColumn, true)
}

func readSQLiteStorageRowsScoped(ctx context.Context, db projectMetadataQueryer, alias, table, filterColumn string, filterValue *string, orderColumn string, literalText bool) (ProjectMetadataTable, error) {
	columns, err := readSQLiteStorageColumns(ctx, db, alias, table)
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	result := ProjectMetadataTable{Columns: columns, Rows: []ProjectMetadataRow{}}
	fields := []string{"rowid"}
	for _, column := range result.Columns {
		name := quoteHeaderIdentifier(column.Name)
		fields = append(fields, "typeof("+name+")", "CASE typeof("+name+
			") WHEN 'integer' THEN CAST("+name+" AS INTEGER) WHEN 'real' THEN CAST("+name+
			" AS REAL) ELSE CAST("+name+" AS BLOB) END")
	}
	relation := quoteHeaderIdentifier(alias) + "." + quoteHeaderIdentifier(table)
	query := "SELECT " + strings.Join(fields, ",") + " FROM " + relation
	var arguments []any
	if filterColumn != "" {
		column := quoteHeaderIdentifier(filterColumn)
		if literalText {
			query += " WHERE typeof(" + column + ")='text' AND CAST(" + column + " AS BLOB)=CAST(? AS BLOB)"
		} else {
			query += " WHERE " + column + " COLLATE BINARY IS ?"
		}
		arguments = append(arguments, filterValue)
	}
	query += " ORDER BY "
	if orderColumn != "" {
		query += quoteHeaderIdentifier(orderColumn) + ","
	}
	query += "rowid"
	rows, err := db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var rowID int64
		storage := make([]string, len(result.Columns))
		values := make([]any, len(result.Columns))
		destinations := []any{&rowID}
		for i := range values {
			destinations = append(destinations, &storage[i], &values[i])
		}
		if err := rows.Scan(destinations...); err != nil {
			return ProjectMetadataTable{}, err
		}
		row := ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10), Cells: []ProjectMetadataCell{}}
		for i, value := range values {
			cell, err := projectMetadataCell(storage[i], value)
			if err != nil {
				return ProjectMetadataTable{}, fmt.Errorf("row %d column %s: %w", rowID, result.Columns[i].Name, err)
			}
			row.Cells = append(row.Cells, cell)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return ProjectMetadataTable{}, err
	}
	return result, nil
}

func readSQLiteStorageColumns(ctx context.Context, db projectMetadataQueryer, alias, table string) ([]ProjectMetadataColumn, error) {
	schema, err := db.QueryContext(ctx, "PRAGMA "+quoteHeaderIdentifier(alias)+".table_info("+quoteHeaderIdentifier(table)+")")
	if err != nil {
		return nil, err
	}
	columns := []ProjectMetadataColumn{}
	for schema.Next() {
		var index, required, primary int
		var name, kind string
		var defaultValue sql.NullString
		if err := schema.Scan(&index, &name, &kind, &required, &defaultValue, &primary); err != nil {
			schema.Close()
			return nil, err
		}
		if !utf8.ValidString(name) || !utf8.ValidString(kind) {
			schema.Close()
			return nil, errors.New("metadata schema contains malformed Unicode")
		}
		columns = append(columns, ProjectMetadataColumn{name, kind})
	}
	if err := errors.Join(schema.Err(), schema.Close()); err != nil {
		return nil, err
	}
	return columns, nil
}

func projectMetadataCell(storage string, value any) (ProjectMetadataCell, error) {
	cell := ProjectMetadataCell{Storage: storage}
	switch storage {
	case "null":
		if value != nil {
			return cell, errors.New("metadata NULL storage disagrees with its value")
		}
	case "integer":
		number, valid := value.(int64)
		if !valid {
			return cell, errors.New("metadata integer storage could not be preserved")
		}
		raw := strconv.FormatInt(number, 10)
		cell.Integer = &raw
	case "real":
		number, valid := value.(float64)
		if !valid || math.IsNaN(number) || math.IsInf(number, 0) {
			return cell, errors.New("metadata real storage is unavailable without conversion")
		}
		cell.Real = &number
	case "text", "blob":
		raw, valid := value.([]byte)
		if !valid {
			return cell, errors.New("metadata byte storage could not be preserved")
		}
		if storage == "blob" {
			value := hex.EncodeToString(raw)
			cell.BlobHex = &value
		} else {
			if !utf8.Valid(raw) {
				return cell, errors.New("metadata text contains malformed Unicode; it was not repaired")
			}
			value := string(raw)
			cell.Text = &value
		}
	default:
		return cell, fmt.Errorf("unsupported metadata storage %q", storage)
	}
	return cell, nil
}
