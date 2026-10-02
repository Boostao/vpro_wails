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
	if !request.Confirmed || !projectNamePattern.MatchString(request.Name) ||
		strings.EqualFold(request.Name, "None") || strings.EqualFold(request.Name, "Sample") ||
		strings.Contains(strings.ToLower(request.Name), "master") {
		return PlotProfileFileCreated{}, errors.New("confirm a literal desktop family name (letter first, ASCII letters/digits/underscore, at most31 characters); None, Sample and master names are reserved")
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
	definition := strings.Replace(originalProfileTemplateSQL, quoteHeaderIdentifier("USysProfileTable"),
		quoteHeaderIdentifier(request.Name+"_Profile"), 1)
	if _, err := tx.ExecContext(ctx, definition+`;
		CREATE TABLE "__VPRO_ProfileCreationHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
		return err
	}
	proposal, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ProfileCreationHistory"(Created,Proposal) VALUES(?,?)`,
		time.Now().UTC().Format(time.RFC3339Nano), string(proposal)); err != nil {
		return err
	}
	stored, err := readSQLiteStorageRows(ctx, tx, "main", request.Name+"_Profile", "", nil, "")
	if err != nil || !reflect.DeepEqual(stored, request.Review.Template) {
		return errors.Join(err, errors.New("new profile schema/empty rules differ from the independently reviewed template"))
	}
	var observed string
	if err := tx.QueryRowContext(ctx, `SELECT Proposal FROM "__VPRO_ProfileCreationHistory" WHERE ID=1`).Scan(&observed); err != nil || observed != string(proposal) {
		return errors.Join(err, errors.New("new profile technical history differs from its complete proposal"))
	}
	var metadata bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='_table_metadata')`).Scan(&metadata); err != nil || metadata {
		return errors.Join(err, errors.New("new profile synthesized description metadata"))
	}
	return tx.Commit()
}
