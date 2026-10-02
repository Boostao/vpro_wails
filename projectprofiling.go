package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-sqlite3"
)

type ProjectPlotProfileRunRequest struct {
	OriginalRules ProjectMetadataTable  `json:"originalRules"`
	ProjectLump   *ProjectMetadataTable `json:"projectLump"`
	Subvarieties  bool                  `json:"subvarieties"`
}

type ProjectPlotProfileStep struct {
	RowID     string `json:"rowId"`
	Order     int16  `json:"order"`
	Operation string `json:"operation"`
	PlotCount int    `json:"plotCount"`
	Remaining int    `json:"remaining"`
}

type ProjectPlotProfileResult struct {
	Project     string                   `json:"project"`
	Table       string                   `json:"table"`
	SU          string                   `json:"su"`
	TotalPlots  int                      `json:"totalPlots"`
	PlotNumbers []string                 `json:"plotNumbers"`
	Steps       []ProjectPlotProfileStep `json:"steps"`
}

func (s *ContextService) ReviewProjectPlotProfileLump(ctx context.Context, contextID string) (ProjectMetadataTable, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (ProjectMetadataTable, error) {
		return readProjectProfileLump(ctx, plots.projects.sqlite)
	})
}

func readProjectProfileLump(ctx context.Context, c *sqliteContext) (ProjectMetadataTable, error) {
	table := c.selection.Project + "_Lump"
	if err := c.requireColumns(ctx, "project", table, []string{"LumpCode", "SppCode", "Use"}); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("explicit project-local lump table unavailable: %w", err)
	}
	return readSQLiteStorageRows(ctx, c.conn, "project", table, "", nil, "")
}

func profileOwnedFiles(c *sqliteContext) error {
	for role, path := range c.attachments {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !os.SameFile(info, c.attachmentInfo[role]) {
			return fmt.Errorf("profile %s file identity changed; reload the context", role)
		}
	}
	return nil
}

func (s *ContextService) RunProjectPlotProfile(ctx context.Context, contextID string, request ProjectPlotProfileRunRequest) (ProjectPlotProfileResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result ProjectPlotProfileResult, resultErr error) {
		owner := plots.projects.sqlite
		if err := profileOwnedFiles(owner); err != nil {
			return result, err
		}
		job, err := newSQLiteContext(ctx, owner.selection, plots.projects.supportPaths)
		if err != nil {
			return result, err
		}
		defer func() {
			resultErr = errors.Join(resultErr, job.Close())
			if resultErr != nil {
				result = ProjectPlotProfileResult{}
			}
		}()
		if err := registerProfileFunctions(job.conn); err != nil {
			return result, err
		}
		tx, err := job.conn.BeginTx(ctx, nil)
		if err != nil {
			return result, err
		}
		defer func() {
			err := tx.Rollback()
			if !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
			if resultErr != nil {
				result = ProjectPlotProfileResult{}
			}
		}()
		table := job.selection.Project + "_Profile"
		fresh, err := readSQLiteStorageRows(ctx, tx, "project", table, "", nil, "Order")
		if err != nil {
			return result, err
		}
		if !reflect.DeepEqual(request.OriginalRules, fresh) {
			return result, errors.New("profile rules/schema/counts changed since review; reload before running")
		}
		env, err := readSQLiteStorageColumns(ctx, tx, "temp", "USysEnv")
		if err != nil {
			return result, err
		}
		rules, err := compilePlotProfileRules(fresh, env)
		if err != nil {
			return result, err
		}
		needsLump := false
		for _, rule := range rules {
			needsLump = needsLump || rule.table == "Lump"
		}
		if needsLump && request.ProjectLump == nil {
			return result, errors.New("explicitly review and select the project-local lump table before running Lump rules")
		}
		if request.ProjectLump != nil {
			lump, err := readSQLiteStorageRows(ctx, tx, "project", job.selection.Project+"_Lump", "", nil, "")
			if err != nil {
				return result, err
			}
			if !reflect.DeepEqual(*request.ProjectLump, lump) {
				return result, errors.New("project lump definitions/schema changed since review; review again")
			}
		}
		lumpRelation := `"project".` + quoteHeaderIdentifier(job.selection.Project+"_Lump")
		if request.Subvarieties {
			if request.ProjectLump == nil {
				return result, errors.New("subvariety combination requires explicit project-lump review; source stale global state is not inherited")
			}
			lumpRelation, err = createProfileCombinedLump(ctx, tx, lumpRelation)
			if err != nil {
				return result, err
			}
		}
		scope, err := queryProfilePlots(ctx, tx, "SELECT PlotNumber FROM USysEnv", nil)
		if err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE VProProfileRunPlots(PlotNumber TEXT COLLATE BINARY PRIMARY KEY)`); err != nil {
			return result, err
		}
		result = ProjectPlotProfileResult{Project: job.selection.Project, Table: table, SU: job.selection.SU,
			TotalPlots: len(scope), PlotNumbers: []string{}, Steps: []ProjectPlotProfileStep{}}
		for _, rule := range rules {
			if err := ctx.Err(); err != nil {
				return ProjectPlotProfileResult{}, err
			}
			query, args := profileMatchQuery(rule, env, lumpRelation)
			matches, err := queryProfilePlots(ctx, tx, query, args)
			if err != nil {
				return ProjectPlotProfileResult{}, fmt.Errorf("profile row %s (Order %d) failed; no stored data changed: %w", rule.rowID, rule.order, err)
			}
			if err := applyProfileStep(ctx, tx, rule, matches); err != nil {
				return ProjectPlotProfileResult{}, err
			}
			var remaining int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM VProProfileRunPlots`).Scan(&remaining); err != nil {
				return ProjectPlotProfileResult{}, err
			}
			var count int
			if err := tx.QueryRowContext(ctx, `SELECT changes()`).Scan(&count); err != nil {
				return ProjectPlotProfileResult{}, err
			}
			if rule.operation == "Common plots" {
				count = remaining
			}
			result.Steps = append(result.Steps, ProjectPlotProfileStep{rule.rowID, rule.order, rule.operation, count, remaining})
		}
		result.PlotNumbers, err = queryProfilePlots(ctx, tx, `SELECT PlotNumber FROM VProProfileRunPlots ORDER BY PlotNumber COLLATE BINARY`, nil)
		if err != nil {
			return ProjectPlotProfileResult{}, err
		}
		if err := profileOwnedFiles(owner); err != nil {
			return ProjectPlotProfileResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return ProjectPlotProfileResult{}, err
		}
		return result, nil
	})
}

func queryProfilePlots(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, query string, args []any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plots, seen := []string{}, map[string]bool{}
	for rows.Next() {
		var raw any
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		value, valid := raw.(string)
		if !valid || value == "" || !utf8.ValidString(value) {
			return nil, errors.New("profile scope contains unsupported/invalid plot identity; no identity was repaired")
		}
		if seen[value] {
			return nil, errors.New("profile query has ambiguous duplicate plot identities")
		}
		seen[value] = true
		plots = append(plots, value)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	return plots, nil
}

func profileDriverNull(value any) bool {
	// go-sqlite3's generic function argument represents SQL NULL as a nil []byte.
	blob, isBlob := value.([]byte)
	return value == nil || isBlob && blob == nil
}

func profileNumeric(value any) (any, error) {
	if profileDriverNull(value) {
		return nil, nil
	}
	var number float64
	switch value := value.(type) {
	case int64:
		number = float64(value)
	case float64:
		number = value
	default:
		return nil, errors.New("profile numeric storage is not numeric; historical values were not coerced")
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, errors.New("profile numeric value is not finite")
	}
	return number, nil
}

func profileMax(values ...any) (float64, error) {
	var maximum float32
	for _, value := range values {
		raw, err := profileNumeric(value)
		if err != nil {
			return 0, err
		}
		if profileDriverNull(raw) {
			continue
		}
		number := raw.(float64)
		if number > float64(maximum) {
			maximum = float32(number)
			if math.IsInf(float64(maximum), 0) {
				return 0, errors.New("source MadMax Single overflow")
			}
		}
	}
	return float64(maximum), nil
}

func registerProfileFunctions(conn *sql.Conn) error {
	return conn.Raw(func(raw any) error {
		driver, valid := raw.(*sqlite3.SQLiteConn)
		if !valid {
			return errors.New("profile runner requires the owned SQLite driver")
		}
		for _, function := range []struct {
			name string
			run  any
		}{
			{"vpro_profile_number", profileNumeric}, {"vpro_profile_max", profileMax},
			{"vpro_profile_text", func(raw any) (any, error) {
				if profileDriverNull(raw) {
					return nil, nil
				}
				value, valid := raw.(string)
				if !valid || !profileASCII(value) {
					return nil, errors.New("non-ASCII/non-text comparison remains unavailable; no stored value was converted")
				}
				return value, nil
			}},
			{"vpro_profile_bool", func(raw any) (any, error) {
				if profileDriverNull(raw) {
					return nil, nil
				}
				value, valid := raw.(int64)
				if !valid || value != -1 && value != 0 && value != 1 {
					return nil, errors.New("unsupported historical BOOLEAN storage")
				}
				if value == 1 {
					value = -1
				}
				return value, nil
			}},
		} {
			if err := driver.RegisterFunc(function.name, function.run, true); err != nil {
				return err
			}
		}
		return nil
	})
}

func createProfileCombinedLump(ctx context.Context, tx *sql.Tx, lump string) (string, error) {
	query := `CREATE TEMP VIEW VProProfileCombinedLump AS
		SELECT DISTINCT substr(vpro_profile_text(s.Code),1,length(s.Code)-1) AS LumpCode,s.Code AS SppCode
		FROM "VLists".USysAllSpecs s
		WHERE substr(vpro_profile_text(s.Code),-1)<'9'
		AND NOT EXISTS(SELECT 1 FROM ` + lump + ` l WHERE vpro_profile_bool(l."Use")=-1
			AND vpro_profile_text(l.SppCode) COLLATE NOCASE=vpro_profile_text(s.Code) COLLATE NOCASE)
		UNION SELECT DISTINCT LumpCode,SppCode FROM ` + lump + ` WHERE vpro_profile_bool("Use")=-1`
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return "", err
	}
	return "VProProfileCombinedLump", nil
}

func profileMatchQuery(rule plotProfileRule, env []ProjectMetadataColumn, lump string) (string, []any) {
	if rule.table == "Env" {
		field := quoteHeaderIdentifier(rule.field)
		if rule.envNull {
			return `SELECT PlotNumber FROM USysEnv WHERE ` + field + ` IS NULL`, nil
		}
		switch rule.criterion.(type) {
		case string:
			field = "vpro_profile_text(" + field + ") COLLATE NOCASE"
		default:
			boolean := false
			for _, column := range env {
				boolean = boolean || column.Name == rule.field && strings.Contains(strings.ToUpper(column.DeclaredType), "BOOL")
			}
			if boolean {
				field = "vpro_profile_bool(" + field + ")"
			} else {
				field = "vpro_profile_number(" + field + ")"
			}
		}
		operator, value := rule.operator, rule.criterion
		if operator == "Like" || operator == "Not Like" {
			// Only the source ASCII * and ? subset is accepted by the planner.
			field = "lower(" + field + ")"
			operator = "GLOB"
			if rule.operator == "Not Like" {
				operator = "NOT GLOB"
			}
			value = strings.ToLower(value.(string))
		}
		return `SELECT PlotNumber FROM USysEnv WHERE ` + field + " " + operator + " ?", []any{value}
	}
	columns := []string{}
	for _, cover := range profileCovers {
		expression := "vpro_profile_number(v." + quoteHeaderIdentifier(cover) + ")"
		if rule.table == "Lump" {
			expression = "SUM(" + expression + ")"
		}
		columns = append(columns, expression+" AS "+quoteHeaderIdentifier(cover))
	}
	from := `USysEnv e INNER JOIN USysVeg v ON e.PlotNumber COLLATE BINARY=v.PlotNumber COLLATE BINARY`
	selector := "vpro_profile_text(v.Species) COLLATE NOCASE"
	if rule.table == "Lump" {
		from += " INNER JOIN " + lump + " l ON vpro_profile_text(v.Species) COLLATE NOCASE=vpro_profile_text(l.SppCode) COLLATE NOCASE"
		selector = "vpro_profile_text(l.LumpCode) COLLATE NOCASE"
	}
	base := "SELECT e.PlotNumber," + strings.Join(columns, ",") + " FROM " + from + " WHERE " + selector + "=?"
	if rule.table == "Lump" {
		base += " GROUP BY e.PlotNumber"
	}
	expression := profileCoverExpression(rule.layer, false)
	query := "WITH covered AS (" + base + ") SELECT DISTINCT PlotNumber FROM covered WHERE " + expression + " " + rule.operator + " ?"
	args := []any{rule.species, rule.criterion}
	if rule.operation == "Add plots" && rule.operator == "<" {
		if rule.layer == "Any" || rule.layer == "SumAll" {
			// Source absence is based on species membership, not an invented zero cover.
			pool := `SELECT DISTINCT e.PlotNumber FROM ` + from
			query += " UNION SELECT PlotNumber FROM (" + pool + ") pool WHERE NOT EXISTS(SELECT 1 FROM covered WHERE covered.PlotNumber COLLATE BINARY=pool.PlotNumber COLLATE BINARY)"
		} else {
			absence := profileCoverExpression(rule.layer, true)
			pool := `SELECT DISTINCT e.PlotNumber FROM USysEnv e INNER JOIN USysVeg v ON e.PlotNumber COLLATE BINARY=v.PlotNumber COLLATE BINARY`
			positive := "SELECT PlotNumber FROM covered GROUP BY PlotNumber HAVING SUM(" + absence + ")>0"
			query += " UNION SELECT PlotNumber FROM (" + pool + ") pool WHERE NOT EXISTS(SELECT 1 FROM (" + positive + ") p WHERE p.PlotNumber COLLATE BINARY=pool.PlotNumber COLLATE BINARY)"
		}
	}
	if rule.operation == "Subtract plots" && rule.table == "Veg" {
		// The source removes already-selected plots without the species for every operator.
		query += " UNION SELECT PlotNumber FROM VProProfileRunPlots r WHERE NOT EXISTS(SELECT 1 FROM covered WHERE covered.PlotNumber COLLATE BINARY=r.PlotNumber COLLATE BINARY)"
	} else if rule.operation == "Subtract plots" && rule.operator == "<" {
		query += " UNION SELECT PlotNumber FROM VProProfileRunPlots r WHERE NOT EXISTS(SELECT 1 FROM covered WHERE covered.PlotNumber COLLATE BINARY=r.PlotNumber COLLATE BINARY)"
	}
	return query, args
}

func profileCoverExpression(layer string, absence bool) string {
	columns := profileLayerColumns(layer, absence)
	if layer == "Any" && !absence {
		return "vpro_profile_max(" + strings.Join(columns, ",") + ")"
	}
	if len(columns) == 1 && !absence {
		return quoteHeaderIdentifier(columns[0])
	}
	parts := []string{}
	for _, column := range columns {
		parts = append(parts, "COALESCE("+quoteHeaderIdentifier(column)+",0)")
	}
	return strings.Join(parts, "+")
}

func applyProfileStep(ctx context.Context, tx *sql.Tx, rule plotProfileRule, matches []string) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE VProProfileMatches(PlotNumber TEXT COLLATE BINARY PRIMARY KEY)`); err != nil {
		return err
	}
	for _, plot := range matches {
		if _, err := tx.ExecContext(ctx, `INSERT INTO VProProfileMatches VALUES(?)`, plot); err != nil {
			return err
		}
	}
	query := `INSERT INTO VProProfileRunPlots SELECT PlotNumber FROM VProProfileMatches WHERE PlotNumber NOT IN (SELECT PlotNumber FROM VProProfileRunPlots)`
	if rule.operation == "Subtract plots" {
		query = `DELETE FROM VProProfileRunPlots WHERE PlotNumber IN (SELECT PlotNumber FROM VProProfileMatches)`
	}
	if rule.operation == "Common plots" {
		query = `DELETE FROM VProProfileRunPlots WHERE PlotNumber NOT IN (SELECT PlotNumber FROM VProProfileMatches)`
	}
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DROP TABLE VProProfileMatches`)
	return err
}
