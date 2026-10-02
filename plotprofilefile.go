package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const originalProfileTemplateSQL = `CREATE TABLE "USysProfileTable" (
  "Order" SMALLINT,
  "Table" VARCHAR,
  "Field" VARCHAR,
  "Operator" VARCHAR,
  "Layer" VARCHAR,
  "Species" VARCHAR,
  "Criteria" VARCHAR,
  "Operation" VARCHAR,
  "PlotCount" INTEGER
)`

const profileCreationHistorySQL = `CREATE TABLE "__VPRO_ProfileCreationHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`

func validateNewProfileName(name string, confirmed bool) error {
	if !confirmed || !projectNamePattern.MatchString(name) ||
		strings.EqualFold(name, "None") || strings.EqualFold(name, "Sample") ||
		strings.Contains(strings.ToLower(name), "master") {
		return errors.New("confirm a literal desktop family name (letter first, ASCII letters/digits/underscore, at most31 characters); None, Sample and master names are reserved")
	}
	return nil
}

type PlotProfileFileReview struct {
	Template       ProjectMetadataTable `json:"template"`
	MetadataAbsent bool                 `json:"metadataAbsent"`
}

type PlotProfileFileCreation struct {
	Review    PlotProfileFileReview `json:"review"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Confirmed bool                  `json:"confirmed"`
}

type PlotProfileFileCreated struct {
	Source    PlotProfileSource `json:"source"`
	Table     string            `json:"table"`
	RuleCount int               `json:"ruleCount"`
}

func (review *PlotProfileFileReview) UnmarshalJSON(data []byte) error {
	type plain PlotProfileFileReview
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "template", "metadataAbsent"); err != nil {
		return err
	}
	*review = PlotProfileFileReview(decoded)
	return nil
}

func (request *PlotProfileFileCreation) UnmarshalJSON(data []byte) error {
	type plain PlotProfileFileCreation
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "review", "name", "path", "confirmed"); err != nil {
		return err
	}
	*request = PlotProfileFileCreation(decoded)
	return nil
}

func readPlotProfileFileReview(ctx context.Context, conn *sql.Conn) (PlotProfileFileReview, error) {
	var definition string
	if err := conn.QueryRowContext(ctx, `SELECT sql FROM VPro64.sqlite_master
		WHERE type='table' AND name COLLATE BINARY='USysProfileTable'`).Scan(&definition); err != nil {
		return PlotProfileFileReview{}, fmt.Errorf("original physical blank profile template unavailable: %w", err)
	}
	if strings.Join(strings.Fields(definition), " ") != strings.Join(strings.Fields(originalProfileTemplateSQL), " ") {
		return PlotProfileFileReview{}, errors.New("blank profile template constraints differ from the retained original; no inferred schema adaptation")
	}
	var extras int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM VPro64.sqlite_master
		WHERE tbl_name COLLATE BINARY='USysProfileTable' AND type<>'table'`).Scan(&extras); err != nil {
		return PlotProfileFileReview{}, err
	}
	if extras != 0 {
		return PlotProfileFileReview{}, errors.New("blank profile template has unreviewed indexes/triggers; creation is unavailable")
	}
	template, err := readSQLiteStorageRows(ctx, conn, "VPro64", "USysProfileTable", "", nil, "")
	if err != nil {
		return PlotProfileFileReview{}, err
	}
	if len(template.Rows) != 0 {
		return PlotProfileFileReview{}, errors.New("profile creation requires the original empty template; legacy population is unavailable")
	}
	metadata, err := profileMetadataColumns(ctx, conn, "VPro64")
	if err != nil {
		return PlotProfileFileReview{}, err
	}
	if metadata != nil {
		return PlotProfileFileReview{}, errors.New("blank profile creation requires the retained template's absent description metadata; present metadata is not silently discarded")
	}
	return PlotProfileFileReview{Template: template, MetadataAbsent: true}, nil
}

func (s *ContextService) ReviewPlotProfileFileCreation(ctx context.Context, contextID string) (PlotProfileFileReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (PlotProfileFileReview, error) {
		c := plots.projects.sqlite
		if err := profileOwnedFiles(c); err != nil {
			return PlotProfileFileReview{}, err
		}
		review, err := readPlotProfileFileReview(ctx, c.conn)
		if err != nil {
			return PlotProfileFileReview{}, err
		}
		return review, profileOwnedFiles(c)
	})
}

func (s *ContextService) CreatePlotProfileFile(ctx context.Context, contextID string, request PlotProfileFileCreation) (_ PlotProfileFileCreated, resultErr error) {
	if err := validateNewProfileName(request.Name, request.Confirmed); err != nil {
		return PlotProfileFileCreated{}, err
	}
	path, parent, err := freshSQLiteDestination(request.Path, "Profile")
	if err != nil {
		return PlotProfileFileCreated{}, err
	}
	temporary, published := "", false
	defer func() {
		if temporary != "" {
			resultErr = errors.Join(resultErr, os.Remove(temporary))
		}
		if published && resultErr != nil {
			resultErr = fmt.Errorf("Profile file published, but cleanup failed; do not replay creation: %w", resultErr)
		}
	}()
	_, err = withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (struct{}, error) {
		owner := plots.projects.sqlite
		if err := profileOwnedFiles(owner); err != nil {
			return struct{}{}, err
		}
		fresh, err := readPlotProfileFileReview(ctx, owner.conn)
		if err != nil {
			return struct{}{}, err
		}
		if !reflect.DeepEqual(fresh, request.Review) {
			return struct{}{}, errors.New("profile template/metadata changed since review; no file published")
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".vpro-profile-file-*.db")
		if err != nil {
			return struct{}{}, err
		}
		temporary = file.Name()
		if err := file.Close(); err != nil {
			return struct{}{}, err
		}
		if err := writePlotProfileFile(ctx, temporary, request); err != nil {
			return struct{}{}, err
		}
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		observed, err := readPlotProfileFileReview(ctx, owner.conn)
		if err != nil {
			return struct{}{}, err
		}
		if !reflect.DeepEqual(fresh, observed) {
			return struct{}{}, errors.New("profile source template changed during staging; no file published")
		}
		if err := profileOwnedFiles(owner); err != nil {
			return struct{}{}, err
		}
		current, err := os.Stat(filepath.Dir(path))
		if err != nil || !os.SameFile(parent, current) {
			return struct{}{}, errors.Join(err, errors.New("profile destination directory identity changed; no file published"))
		}
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		if err := os.Link(temporary, path); err != nil {
			return struct{}{}, fmt.Errorf("profile publication requires atomic no-replace linking; destination unchanged: %w", err)
		}
		published = true
		return struct{}{}, nil
	})
	if err != nil {
		return PlotProfileFileCreated{}, err
	}
	return PlotProfileFileCreated{Source: PlotProfileSource{request.Name, path}, Table: request.Name + "_Profile", RuleCount: 0}, nil
}

func writePlotProfileFile(ctx context.Context, path string, request PlotProfileFileCreation) (resultErr error) {
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	proposal, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if err := createBlankProfileTable(ctx, tx, request.Name, request.Review.Template, string(proposal)); err != nil {
		return err
	}
	var metadata bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='_table_metadata')`).Scan(&metadata); err != nil || metadata {
		return errors.Join(err, errors.New("new profile synthesized description metadata"))
	}
	return tx.Commit()
}

func createBlankProfileTable(ctx context.Context, tx *sql.Tx, name string, template ProjectMetadataTable, proposal string) error {
	var collision int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name=? COLLATE NOCASE`, name+"_Profile").Scan(&collision); err != nil {
		return err
	}
	if collision != 0 {
		return errors.New("profile destination name already belongs to a physical SQLite object; no replacement")
	}
	if _, err := tx.ExecContext(ctx, strings.Replace(originalProfileTemplateSQL, quoteHeaderIdentifier("USysProfileTable"),
		quoteHeaderIdentifier(name+"_Profile"), 1)); err != nil {
		return err
	}
	if err := appendCreationProvenance(ctx, tx, "__VPRO_ProfileCreationHistory", profileCreationHistorySQL, proposal); err != nil {
		return err
	}
	stored, err := readSQLiteStorageRows(ctx, tx, "main", name+"_Profile", "", nil, "")
	if err != nil || !reflect.DeepEqual(stored, template) {
		return errors.Join(err, errors.New("new profile schema/empty rules differ from the independently reviewed template"))
	}
	return nil
}

func appendCreationProvenance(ctx context.Context, tx *sql.Tx, table, expectedDefinition, proposal string) error {
	if _, err := tx.ExecContext(ctx, strings.Replace(expectedDefinition, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)); err != nil {
		return err
	}
	var definition string
	if err := tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master
		WHERE type='table' AND name COLLATE BINARY=?`, table).Scan(&definition); err != nil {
		return fmt.Errorf("physical profile creation provenance unavailable: %w", err)
	}
	if strings.Join(strings.Fields(definition), " ") != strings.Join(strings.Fields(expectedDefinition), " ") {
		return errors.New("profile creation provenance schema differs; no existing history overwritten")
	}
	var extras int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master
		WHERE tbl_name COLLATE NOCASE=? AND type<>'table'`, table).Scan(&extras); err != nil {
		return err
	}
	if extras != 0 {
		return errors.New("profile creation provenance has unreviewed indexes/triggers; no history appended")
	}
	when := time.Now().UTC().Format(time.RFC3339Nano)
	inserted, err := tx.ExecContext(ctx, `INSERT INTO `+quoteHeaderIdentifier(table)+`(Created,Proposal) VALUES(?,?)`, when, proposal)
	if err != nil {
		return err
	}
	if count, err := inserted.RowsAffected(); err != nil || count != 1 {
		return errors.Join(err, errors.New("profile creation provenance did not insert exactly one event"))
	}
	id, err := inserted.LastInsertId()
	if err != nil {
		return err
	}
	var observed, observedWhen string
	if err := tx.QueryRowContext(ctx, `SELECT Created,Proposal FROM `+quoteHeaderIdentifier(table)+` WHERE ID=?`, id).
		Scan(&observedWhen, &observed); err != nil || observed != proposal || observedWhen != when {
		return errors.Join(err, errors.New("new profile technical history differs from its complete proposal"))
	}
	return nil
}
