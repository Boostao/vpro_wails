package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
)

type SiteUnitEnvironmentChange struct {
	PlotNumber string              `json:"plotNumber"`
	EnvRowID   string              `json:"envRowId"`
	AdminRowID string              `json:"adminRowId"`
	SURowID    string              `json:"suRowId"`
	Field      string              `json:"field"`
	Origin     string              `json:"origin"`
	Before     ProjectMetadataCell `json:"before"`
	After      ProjectMetadataCell `json:"after"`
}

func planSiteUnitEnvironment(env, admin, su, master, personal ProjectMetadataTable) ([]SiteUnitEnvironmentChange, error) {
	type indexedTable struct {
		columns map[string]int
		rows    map[string][]ProjectMetadataRow
	}
	index := func(table ProjectMetadataTable, identity string, required ...string) (indexedTable, error) {
		columns, err := siteUnitTransferColumns(table, append([]string{identity}, required...)...)
		if err != nil {
			return indexedTable{}, err
		}
		rows, err := siteUnitTransferIndex(table, columns[identity])
		return indexedTable{columns, rows}, err
	}
	environments, err := index(env, "PlotNumber", "Flag")
	if err != nil {
		return nil, fmt.Errorf("SU/environment plot source: %w", err)
	}
	parents, err := index(admin, "Plot", "UserSiteUnit", "SiteUnitShortName", "SiteUnitLongName")
	if err != nil {
		return nil, fmt.Errorf("SU/environment Admin target: %w", err)
	}
	units, err := index(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("SU/environment selected SU: %w", err)
	}
	masters, err := index(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return nil, fmt.Errorf("SU/environment master definitions: %w", err)
	}
	users, err := index(personal, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return nil, fmt.Errorf("SU/environment personal definitions: %w", err)
	}
	changes := []SiteUnitEnvironmentChange{}
	for _, source := range su.Rows {
		identity := source.Cells[units.columns["PlotNumber"]]
		if identity.Storage == "null" {
			continue
		}
		plot := *identity.Text
		envRows, adminRows := environments.rows[plot], parents.rows[plot]
		if len(envRows) == 0 || len(adminRows) == 0 {
			continue
		}
		if len(envRows) != 1 || len(adminRows) != 1 || len(units.rows[plot]) != 1 {
			return nil, errors.New("SU/environment transfer has ambiguous physical plot links")
		}
		values := map[string]ProjectMetadataCell{"UserSiteUnit": source.Cells[units.columns["SiteUnit"]]}
		origins := map[string]string{"UserSiteUnit": "selected SU"}
		if _, err := metadataCellValue(values["UserSiteUnit"]); err != nil {
			return nil, err
		}
		if code := values["UserSiteUnit"].Text; code != nil {
			matches, overrides := masters.rows[*code], users.rows[*code]
			if len(matches) > 1 || len(overrides) > 1 {
				return nil, errors.New("SU/environment transfer has duplicate matching master or personal definitions; no definition was chosen")
			}
			definitions, definitionColumns, origin := matches, masters.columns, "master"
			if len(overrides) == 1 {
				definitions, definitionColumns, origin = overrides, users.columns, "personal override"
			}
			if len(definitions) == 1 {
				values["SiteUnitShortName"] = definitions[0].Cells[definitionColumns["SiteSeries"]]
				values["SiteUnitLongName"] = definitions[0].Cells[definitionColumns["SiteSeriesLongName"]]
				origins["SiteUnitShortName"], origins["SiteUnitLongName"] = origin, origin
			}
		}
		for _, field := range []string{"UserSiteUnit", "SiteUnitShortName", "SiteUnitLongName"} {
			after, assigned := values[field]
			if !assigned {
				continue
			}
			before := adminRows[0].Cells[parents.columns[field]]
			for _, cell := range []ProjectMetadataCell{before, after} {
				if _, err := metadataCellValue(cell); err != nil {
					return nil, err
				}
			}
			if reflect.DeepEqual(before, after) {
				continue
			}
			flag := envRows[0].Cells[environments.columns["Flag"]]
			if _, err := metadataCellValue(flag); err != nil {
				return nil, err
			}
			if flag.Storage != "null" && !(flag.Storage == "integer" && flag.Integer != nil && *flag.Integer == "0") {
				return nil, errors.New("SU/environment transfer cannot change a locked or unverifiably unlocked plot")
			}
			bound := 100
			if field == "SiteUnitShortName" {
				bound = 50
			}
			if before.Storage != "null" && before.Storage != "text" || after.Storage != "null" && after.Storage != "text" {
				return nil, fmt.Errorf("SU/environment transfer cannot assign unsupported historical %s storage", field)
			}
			if after.Text != nil && (strings.ContainsRune(*after.Text, 0) || len(utf16.Encode([]rune(*after.Text))) > bound) {
				return nil, fmt.Errorf("SU/environment %s requires new text within %d UTF-16 units and without NUL", field, bound)
			}
			changes = append(changes, SiteUnitEnvironmentChange{plot, envRows[0].RowID, adminRows[0].RowID,
				source.RowID, field, origins[field], cloneSiteUnitCell(before), cloneSiteUnitCell(after)})
		}
	}
	return changes, nil
}

type SiteUnitEnvironmentReview struct {
	ContextID   string                      `json:"contextId"`
	Project     string                      `json:"project"`
	SU          string                      `json:"su"`
	Path        string                      `json:"path"`
	Env         ProjectMetadataTable        `json:"env"`
	Admin       ProjectMetadataTable        `json:"admin"`
	Source      ProjectMetadataTable        `json:"source"`
	Master      ProjectMetadataTable        `json:"master"`
	Personal    ProjectMetadataTable        `json:"personal"`
	Changes     []SiteUnitEnvironmentChange `json:"changes"`
	HistoryHash string                      `json:"historyHash"`
}

type SiteUnitEnvironmentTransfer struct {
	Review    SiteUnitEnvironmentReview `json:"review"`
	Confirmed bool                      `json:"confirmed"`
}

type SiteUnitEnvironmentResult struct {
	ChangedRows  int    `json:"changedRows"`
	ChangedCells int    `json:"changedCells"`
	HistoryID    string `json:"historyId"`
}

const siteUnitEnvironmentHistory = "__VPRO_SUEnvironmentHistory"
const siteUnitEnvironmentHistorySQL = `CREATE TABLE "__VPRO_SUEnvironmentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

func (request *SiteUnitEnvironmentTransfer) UnmarshalJSON(data []byte) error {
	type plain SiteUnitEnvironmentTransfer
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
		"contextId", "project", "su", "path", "env", "admin", "source", "master", "personal", "changes", "historyHash"); err != nil {
		return err
	}
	*request = SiteUnitEnvironmentTransfer(decoded)
	return nil
}

func readSiteUnitEnvironmentReview(ctx context.Context, tx *sql.Tx, c *sqliteContext, contextID string) (SiteUnitEnvironmentReview, error) {
	if err := c.validateEnvironmentSiteUnitOwner(); err != nil {
		return SiteUnitEnvironmentReview{}, err
	}
	review := SiteUnitEnvironmentReview{ContextID: contextID, Project: c.selection.Project,
		SU: c.selection.SU, Path: c.selection.ProjectPath}
	for _, item := range []struct {
		alias, name string
		dest        *ProjectMetadataTable
	}{{"main", review.Project + "_Env", &review.Env}, {"main", review.Project + "_Admin", &review.Admin},
		{"main", review.SU + "_SU", &review.Source}, {"reference", "MasterSiteUnitList", &review.Master},
		{"personal", "UserSiteUnitList", &review.Personal}} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(item.alias)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, item.name).Scan(&count); err != nil || count != 1 {
			return SiteUnitEnvironmentReview{}, errors.Join(err, errors.New("SU/environment review requires original physical source tables"))
		}
		table, err := readSQLiteStorageRows(ctx, tx, item.alias, item.name, "", nil, "")
		if err != nil {
			return SiteUnitEnvironmentReview{}, err
		}
		*item.dest = table
	}
	changes, err := planSiteUnitEnvironment(review.Env, review.Admin, review.Source, review.Master, review.Personal)
	if err != nil {
		return SiteUnitEnvironmentReview{}, err
	}
	review.Changes = changes
	var objects int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name COLLATE NOCASE=?`, siteUnitEnvironmentHistory).Scan(&objects); err != nil {
		return SiteUnitEnvironmentReview{}, err
	}
	if objects != 0 {
		if err := verifyTechnicalProvenance(ctx, tx, siteUnitEnvironmentHistory, siteUnitEnvironmentHistorySQL, "SU/environment transfer"); err != nil {
			return SiteUnitEnvironmentReview{}, err
		}
		table, err := readSQLiteStorageRows(ctx, tx, "main", siteUnitEnvironmentHistory, "", nil, "")
		if err != nil {
			return SiteUnitEnvironmentReview{}, err
		}
		raw, err := json.Marshal(table)
		if err != nil {
			return SiteUnitEnvironmentReview{}, err
		}
		review.HistoryHash = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return review, nil
}

func withSiteUnitEnvironmentWriter(ctx context.Context, c *sqliteContext, operation func(*sql.Conn) error) error {
	return c.withMetadataWriter(ctx, func(conn *sql.Conn) error {
		if c.attachmentInfo["VUser"] == nil {
			return errors.New("SU/environment personal definitions require the owned VUser file")
		}
		if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS personal`, sqliteFileURI(c.attachments["VUser"], "ro")); err != nil {
			return fmt.Errorf("SU/environment personal definitions unavailable: %w", err)
		}
		return operation(conn)
	})
}

func (s *ContextService) ReviewSiteUnitEnvironment(ctx context.Context, contextID string) (SiteUnitEnvironmentReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (SiteUnitEnvironmentReview, error) {
		c := plots.projects.sqlite
		var review SiteUnitEnvironmentReview
		err := withSiteUnitEnvironmentWriter(ctx, c, func(conn *sql.Conn) (resultErr error) {
			tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, tx.Rollback()) }()
			review, err = readSiteUnitEnvironmentReview(ctx, tx, c, contextID)
			return err
		})
		return review, err
	})
}

func (s *ContextService) TransferSiteUnitEnvironment(ctx context.Context, contextID string, request SiteUnitEnvironmentTransfer) (*SiteUnitEnvironmentResult, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*SiteUnitEnvironmentResult, error) {
		if !request.Confirmed || request.Review.ContextID != contextID || len(request.Review.Changes) == 0 {
			return nil, errors.New("SU/environment transfer requires explicit confirmation of a nonempty current-context review")
		}
		c := plots.projects.sqlite
		result, committed := &SiteUnitEnvironmentResult{}, false
		err := withSiteUnitEnvironmentWriter(ctx, c, func(conn *sql.Conn) (resultErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, err)
				}
			}()
			fresh, err := readSiteUnitEnvironmentReview(ctx, tx, c, contextID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(fresh, request.Review) {
				return errors.New("SU/environment source, definitions, target or history changed since review; no transfer applied")
			}
			original, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			expected := ProjectMetadataTable{Columns: fresh.Admin.Columns, Rows: append([]ProjectMetadataRow(nil), fresh.Admin.Rows...)}
			columns, err := siteUnitTransferColumns(expected, "UserSiteUnit", "SiteUnitShortName", "SiteUnitLongName")
			if err != nil {
				return err
			}
			rows := map[string]bool{}
			for _, change := range fresh.Changes {
				value, err := metadataCellValue(change.After)
				if err != nil {
					return err
				}
				rowID, err := strconv.ParseInt(change.AdminRowID, 10, 64)
				if err != nil {
					return err
				}
				update, err := tx.ExecContext(ctx, `UPDATE `+quoteHeaderIdentifier(fresh.Project+"_Admin")+
					` SET `+quoteHeaderIdentifier(change.Field)+`=? WHERE rowid=? AND Plot COLLATE BINARY=?`, value, rowID, change.PlotNumber)
				if err != nil {
					return err
				}
				if count, err := update.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("SU/environment transfer lost its sole physical Admin target"))
				}
				for i, row := range expected.Rows {
					if row.RowID == change.AdminRowID {
						cells := append([]ProjectMetadataCell(nil), row.Cells...)
						cells[columns[change.Field]] = change.After
						expected.Rows[i].Cells = cells
					}
				}
				rows[change.AdminRowID] = true
				result.ChangedCells++
			}
			result.ChangedRows = len(rows)
			proposal, err := json.Marshal(struct {
				Request SiteUnitEnvironmentTransfer `json:"request"`
				User    string                      `json:"user"`
			}{request, plots.currentUser})
			if err != nil {
				return err
			}
			if err := appendTechnicalProvenance(ctx, tx, siteUnitEnvironmentHistory, siteUnitEnvironmentHistorySQL, string(proposal), "SU/environment transfer"); err != nil {
				return err
			}
			observed, err := environmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := verifySiteUnitTransferTables(original, observed,
				map[string]ProjectMetadataTable{fresh.Project + "_Admin": expected}, siteUnitEnvironmentHistory); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, `SELECT CAST(ID AS TEXT) FROM __VPRO_SUEnvironmentHistory ORDER BY ID DESC LIMIT 1`).Scan(&result.HistoryID); err != nil {
				return err
			}
			if id, err := strconv.ParseInt(result.HistoryID, 10, 64); err != nil || id < 1 {
				return errors.Join(err, errors.New("SU/environment transfer could not seal an exact positive history identity"))
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
				return nil, fmt.Errorf("SU/environment transfer committed, but cleanup failed; do not replay: %w", err)
			}
			return nil, err
		}
		return result, nil
	})
}
