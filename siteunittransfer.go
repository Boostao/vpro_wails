package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
)

type EnvironmentSiteUnitChange struct {
	PlotNumber string              `json:"plotNumber"`
	EnvRowID   string              `json:"envRowId"`
	AdminRowID string              `json:"adminRowId"`
	SURowID    string              `json:"suRowId"`
	Before     ProjectMetadataCell `json:"before"`
	After      ProjectMetadataCell `json:"after"`
}

func planEnvironmentSiteUnits(env, admin, su ProjectMetadataTable) ([]EnvironmentSiteUnitChange, error) {
	envColumns, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("environment transfer source: %w", err)
	}
	adminColumns, err := siteUnitTransferColumns(admin, "Plot", "UserSiteUnit")
	if err != nil {
		return nil, fmt.Errorf("environment transfer admin link: %w", err)
	}
	suColumns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("environment transfer SU target: %w", err)
	}
	index := func(table ProjectMetadataTable, column int) (map[string][]ProjectMetadataRow, error) {
		result := map[string][]ProjectMetadataRow{}
		for _, row := range table.Rows {
			cell := row.Cells[column]
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
			if cell.Storage == "null" {
				continue
			}
			if cell.Storage != "text" {
				return nil, errors.New("transfer plot identities require original text storage; no coercion")
			}
			result[*cell.Text] = append(result[*cell.Text], row)
		}
		return result, nil
	}
	envRows, err := index(env, envColumns["PlotNumber"])
	if err != nil {
		return nil, err
	}
	adminRows, err := index(admin, adminColumns["Plot"])
	if err != nil {
		return nil, err
	}
	suRows, err := index(su, suColumns["PlotNumber"])
	if err != nil {
		return nil, err
	}
	changes := []EnvironmentSiteUnitChange{}
	for _, target := range su.Rows {
		identity := target.Cells[suColumns["PlotNumber"]]
		if identity.Storage == "null" {
			continue
		}
		plot := *identity.Text
		sources, parents := envRows[plot], adminRows[plot]
		if len(sources) == 0 || len(parents) == 0 {
			continue
		}
		if len(sources) != 1 || len(parents) != 1 || len(suRows[plot]) != 1 {
			return nil, errors.New("environment/SU transfer has ambiguous physical plot links; DISTINCT cannot authorize a write")
		}
		before := target.Cells[suColumns["SiteUnit"]]
		after := parents[0].Cells[adminColumns["UserSiteUnit"]]
		for _, cell := range []ProjectMetadataCell{before, after} {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if reflect.DeepEqual(before, after) {
			continue
		}
		if before.Storage != "null" && before.Storage != "text" || after.Storage != "null" && after.Storage != "text" {
			return nil, errors.New("environment/SU transfer cannot assign unsupported historical SiteUnit storage")
		}
		if after.Text != nil && (strings.ContainsRune(*after.Text, 0) || len(utf16.Encode([]rune(*after.Text))) > 255) {
			return nil, errors.New("environment/SU transfer requires new SiteUnit text within the source 255 UTF-16-unit bound and without NUL")
		}
		clone := func(cell ProjectMetadataCell) ProjectMetadataCell {
			if cell.Text != nil {
				text := *cell.Text
				cell.Text = &text
			}
			return cell
		}
		changes = append(changes, EnvironmentSiteUnitChange{
			plot, sources[0].RowID, parents[0].RowID, target.RowID, clone(before), clone(after)})
	}
	return changes, nil
}

type EnvironmentSiteUnitReview struct {
	ContextID   string                      `json:"contextId"`
	Project     string                      `json:"project"`
	SU          string                      `json:"su"`
	Path        string                      `json:"path"`
	Env         ProjectMetadataTable        `json:"env"`
	Admin       ProjectMetadataTable        `json:"admin"`
	Target      ProjectMetadataTable        `json:"target"`
	Changes     []EnvironmentSiteUnitChange `json:"changes"`
	HistoryHash string                      `json:"historyHash"`
}

type EnvironmentSiteUnitTransfer struct {
	Review    EnvironmentSiteUnitReview `json:"review"`
	Confirmed bool                      `json:"confirmed"`
}

type EnvironmentSiteUnitResult struct {
	ChangedRows int    `json:"changedRows"`
	HistoryID   string `json:"historyId"`
}

const environmentSiteUnitHistory = "__VPRO_EnvironmentSUHistory"
const environmentSiteUnitHistorySQL = `CREATE TABLE "__VPRO_EnvironmentSUHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

func (request *EnvironmentSiteUnitTransfer) UnmarshalJSON(data []byte) error {
	type plain EnvironmentSiteUnitTransfer
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "confirmed"); err != nil {
		return err
	}
	var nested struct {
		Review json.RawMessage `json:"review"`
	}
	if err := json.Unmarshal(data, &nested); err != nil {
		return err
	}
	if err := decodeProfileLifecycleJSON(nested.Review, &decoded.Review,
		"contextId", "project", "su", "path", "env", "admin", "target", "changes", "historyHash"); err != nil {
		return err
	}
	*request = EnvironmentSiteUnitTransfer(decoded)
	return nil
}

func (c *sqliteContext) validateEnvironmentSiteUnitOwner() error {
	if err := c.validateMetadataWriterFiles(); err != nil {
		return err
	}
	if c.selection.SU == "None" || c.attachmentInfo["su"] == nil ||
		!os.SameFile(c.attachmentInfo["project"], c.attachmentInfo["su"]) {
		return errors.New("environment/SU transfer requires an explicitly selected SU in the owned project file; arbitrary-file writes are unavailable")
	}
	return nil
}

func readEnvironmentSiteUnitReview(ctx context.Context, tx *sql.Tx, c *sqliteContext, contextID string) (EnvironmentSiteUnitReview, error) {
	if err := c.validateEnvironmentSiteUnitOwner(); err != nil {
		return EnvironmentSiteUnitReview{}, err
	}
	review := EnvironmentSiteUnitReview{ContextID: contextID, Project: c.selection.Project,
		SU: c.selection.SU, Path: c.selection.ProjectPath}
	for _, item := range []struct {
		name string
		dest *ProjectMetadataTable
	}{{review.Project + "_Env", &review.Env}, {review.Project + "_Admin", &review.Admin}, {review.SU + "_SU", &review.Target}} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, item.name).
			Scan(&count); err != nil || count != 1 {
			return EnvironmentSiteUnitReview{}, errors.Join(err, errors.New("environment/SU transfer requires original physical tables, not substituted views"))
		}
		table, err := readSQLiteStorageRows(ctx, tx, "main", item.name, "", nil, "")
		if err != nil {
			return EnvironmentSiteUnitReview{}, err
		}
		*item.dest = table
	}
	changes, err := planEnvironmentSiteUnits(review.Env, review.Admin, review.Target)
	if err != nil {
		return EnvironmentSiteUnitReview{}, err
	}
	review.Changes = changes
	var historyObjects int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE=?`, environmentSiteUnitHistory).
		Scan(&historyObjects); err != nil {
		return EnvironmentSiteUnitReview{}, err
	}
	if historyObjects != 0 {
		if err := verifyTechnicalProvenance(ctx, tx, environmentSiteUnitHistory, environmentSiteUnitHistorySQL, "environment/SU transfer"); err != nil {
			return EnvironmentSiteUnitReview{}, err
		}
		history, err := readSQLiteStorageRows(ctx, tx, "main", environmentSiteUnitHistory, "", nil, "")
		if err != nil {
			return EnvironmentSiteUnitReview{}, err
		}
		raw, err := json.Marshal(history)
		if err != nil {
			return EnvironmentSiteUnitReview{}, err
		}
		review.HistoryHash = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return review, nil
}

func (s *ContextService) ReviewEnvironmentSiteUnits(ctx context.Context, contextID string) (EnvironmentSiteUnitReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (EnvironmentSiteUnitReview, error) {
		c := plots.projects.sqlite
		var review EnvironmentSiteUnitReview
		err := c.withMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
			tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, tx.Rollback()) }()
			review, err = readEnvironmentSiteUnitReview(ctx, tx, c, contextID)
			return err
		})
		return review, err
	})
}

func environmentSiteUnitTables(ctx context.Context, tx *sql.Tx) (map[string]ProjectMetadataTable, error) {
	schema, err := readSQLiteStorageRows(ctx, tx, "main", "sqlite_master", "", nil, "")
	if err != nil {
		return nil, err
	}
	tables := map[string]ProjectMetadataTable{"sqlite_master": schema}
	for _, row := range schema.Rows {
		if row.Cells[0].Text == nil || *row.Cells[0].Text != "table" {
			continue
		}
		name := *row.Cells[1].Text
		table, err := readSQLiteStorageRows(ctx, tx, "main", name, "", nil, "")
		if err != nil {
			return nil, err
		}
		tables[name] = table
	}
	return tables, nil
}

func (s *ContextService) TransferEnvironmentSiteUnits(ctx context.Context, contextID string, request EnvironmentSiteUnitTransfer) (*EnvironmentSiteUnitResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*EnvironmentSiteUnitResult, error) {
		if !request.Confirmed || request.Review.ContextID != contextID || len(request.Review.Changes) == 0 {
			return nil, errors.New("environment/SU transfer requires explicit confirmation of a nonempty current-context review")
		}
		c := plots.projects.sqlite
		result := &EnvironmentSiteUnitResult{}
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
			fresh, err := readEnvironmentSiteUnitReview(ctx, tx, c, contextID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(fresh, request.Review) {
				return errors.New("environment/SU source, target or physical links changed since review; no transfer applied")
			}
			original, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			target := original[fresh.SU+"_SU"]
			expected := ProjectMetadataTable{Columns: target.Columns, Rows: append([]ProjectMetadataRow(nil), target.Rows...)}
			columns, err := siteUnitTransferColumns(target, "SiteUnit")
			if err != nil {
				return err
			}
			for _, change := range fresh.Changes {
				value, err := metadataCellValue(change.After)
				if err != nil {
					return err
				}
				rowID, err := strconv.ParseInt(change.SURowID, 10, 64)
				if err != nil {
					return err
				}
				updated, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(fresh.SU+"_SU")+
					` SET SiteUnit=? WHERE rowid=? AND PlotNumber COLLATE BINARY=?`, value, rowID, change.PlotNumber)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("environment/SU transfer lost its sole physical target"))
				}
				for i, row := range expected.Rows {
					if row.RowID == change.SURowID {
						cells := append([]ProjectMetadataCell(nil), row.Cells...)
						cells[columns["SiteUnit"]] = change.After
						expected.Rows[i].Cells = cells
					}
				}
				result.ChangedRows++
			}
			proposal, err := json.Marshal(struct {
				Request EnvironmentSiteUnitTransfer `json:"request"`
				User    string                      `json:"user"`
			}{request, plots.currentUser})
			if err != nil {
				return err
			}
			if err := appendTechnicalProvenance(ctx, tx, environmentSiteUnitHistory, environmentSiteUnitHistorySQL,
				string(proposal), "environment/SU transfer"); err != nil {
				return err
			}
			observed, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			for name, before := range original {
				switch name {
				case fresh.SU + "_SU":
					before = expected
				case environmentSiteUnitHistory:
					after := observed[name]
					if !reflect.DeepEqual(after.Columns, before.Columns) || len(after.Rows) != len(before.Rows)+1 ||
						!reflect.DeepEqual(after.Rows[:len(before.Rows)], before.Rows) {
						return errors.New("original environment/SU history changed; transfer rolled back")
					}
					continue
				case "sqlite_master":
					filter := func(schema ProjectMetadataTable) ProjectMetadataTable {
						schema.Rows = append([]ProjectMetadataRow(nil), schema.Rows...)
						rows := schema.Rows[:0]
						for _, row := range schema.Rows {
							if row.Cells[2].Text == nil || *row.Cells[2].Text != environmentSiteUnitHistory {
								rows = append(rows, row)
							}
						}
						schema.Rows = rows
						return schema
					}
					before = filter(before)
					if !reflect.DeepEqual(filter(observed[name]), before) {
						return errors.New("original project schema changed during environment/SU transfer; all changes rolled back")
					}
					continue
				}
				if !reflect.DeepEqual(observed[name], before) {
					return fmt.Errorf("project table %s differs from the reviewed environment/SU plan; all changes rolled back", name)
				}
			}
			if len(observed) != len(original) && !(len(observed) == len(original)+1 && original[environmentSiteUnitHistory].Columns == nil) {
				return errors.New("environment/SU transfer introduced unexpected project tables; all changes rolled back")
			}
			if err := tx.QueryRowContext(ctx, `SELECT CAST(ID AS TEXT) FROM __VPRO_EnvironmentSUHistory ORDER BY ID DESC LIMIT 1`).
				Scan(&result.HistoryID); err != nil {
				return err
			}
			if id, err := strconv.ParseInt(result.HistoryID, 10, 64); err != nil || id < 1 {
				return errors.Join(err, errors.New("environment/SU transfer could not seal a positive exact history identity"))
			}
			if err := c.validateEnvironmentSiteUnitOwner(); err != nil {
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
				return nil, fmt.Errorf("environment/SU transfer committed, but cleanup failed; do not replay: %w", err)
			}
			return nil, err
		}
		return result, nil
	})
}

func siteUnitTransferColumns(table ProjectMetadataTable, required ...string) (map[string]int, error) {
	index := map[string]int{}
	for i, column := range table.Columns {
		if _, duplicate := index[column.Name]; duplicate || column.Name == "" {
			return nil, errors.New("transfer schema has missing or ambiguous column identities")
		}
		index[column.Name] = i
	}
	for _, name := range required {
		if _, present := index[name]; !present {
			return nil, fmt.Errorf("transfer requires original %s column", name)
		}
	}
	seen := map[string]bool{}
	for _, row := range table.Rows {
		id, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != row.RowID || seen[row.RowID] || len(row.Cells) != len(table.Columns) {
			return nil, errors.New("transfer requires complete rows with distinct exact signed64 physical identities")
		}
		seen[row.RowID] = true
	}
	return index, nil
}
