package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	if !confirmed || !vprocore.ProjectNamePattern.MatchString(name) ||
		strings.EqualFold(name, "None") || strings.EqualFold(name, "Sample") ||
		strings.Contains(strings.ToLower(name), "master") {
		return errors.New("confirm a literal desktop family name (letter first, ASCII letters/digits/underscore, at most31 characters); None, Sample and master names are reserved")
	}
	return nil
}

type PlotProfileFileReview struct {
	Template       vprocore.ProjectMetadataTable `json:"template"`
	MetadataAbsent bool                          `json:"metadataAbsent"`
}

type PlotProfileFileCreation struct {
	Review    PlotProfileFileReview `json:"review"`
	Name      string                `json:"name"`
	Path      string                `json:"path"`
	Confirmed bool                  `json:"confirmed"`
}

type PlotProfileFileCreated struct {
	Source    vprocore.PlotProfileSource `json:"source"`
	Table     string                     `json:"table"`
	RuleCount int                        `json:"ruleCount"`
}

func (review *PlotProfileFileReview) UnmarshalJSON(data []byte) error {
	type plain PlotProfileFileReview
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "template", "metadataAbsent"); err != nil {
		return err
	}
	*review = PlotProfileFileReview(decoded)
	return nil
}

func (request *PlotProfileFileCreation) UnmarshalJSON(data []byte) error {
	type plain PlotProfileFileCreation
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "review", "name", "path", "confirmed"); err != nil {
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
	template, err := vprocore.ReadSQLiteStorageRows(ctx, conn, "VPro64", "USysProfileTable", "", nil, "")
	if err != nil {
		return PlotProfileFileReview{}, err
	}
	if len(template.Rows) != 0 {
		return PlotProfileFileReview{}, errors.New("profile creation requires the original empty template; legacy population is unavailable")
	}
	metadata, err := vprocore.ProfileMetadataColumns(ctx, conn, "VPro64")
	if err != nil {
		return PlotProfileFileReview{}, err
	}
	if metadata != nil {
		return PlotProfileFileReview{}, errors.New("blank profile creation requires the retained template's absent description metadata; present metadata is not silently discarded")
	}
	return PlotProfileFileReview{Template: template, MetadataAbsent: true}, nil
}

func ReviewPlotProfileFileCreation(s *vprocore.ContextService, ctx context.Context, contextID string) (PlotProfileFileReview, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (PlotProfileFileReview, error) {
		c := plots.Projects.Sqlite
		if err := vprocore.ProfileOwnedFiles(c); err != nil {
			return PlotProfileFileReview{}, err
		}
		review, err := readPlotProfileFileReview(ctx, c.Conn)
		if err != nil {
			return PlotProfileFileReview{}, err
		}
		return review, vprocore.ProfileOwnedFiles(c)
	})
}

func CreatePlotProfileFile(s *vprocore.ContextService, ctx context.Context, contextID string, request PlotProfileFileCreation) (_ PlotProfileFileCreated, resultErr error) {
	if err := validateNewProfileName(request.Name, request.Confirmed); err != nil {
		return PlotProfileFileCreated{}, err
	}
	path, parent, err := vprocore.FreshSQLiteDestination(request.Path, "Profile")
	if err != nil {
		return PlotProfileFileCreated{}, err
	}
	temporary, published := "", false
	defer func() {
		if temporary != "" {
			resultErr = errors.Join(resultErr, os.Remove(temporary))
			for _, suffix := range []string{"-journal", "-wal", "-shm"} {
				if err := os.Remove(temporary + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
					resultErr = errors.Join(resultErr, err)
				}
			}
		}
		if published && resultErr != nil {
			resultErr = fmt.Errorf("Profile file published, but cleanup failed; do not replay creation: %w", resultErr)
		}
	}()
	_, err = vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return struct{}{}, err
		}
		fresh, err := readPlotProfileFileReview(ctx, owner.Conn)
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
		observed, err := readPlotProfileFileReview(ctx, owner.Conn)
		if err != nil {
			return struct{}{}, err
		}
		if !reflect.DeepEqual(fresh, observed) {
			return struct{}{}, errors.New("profile source template changed during staging; no file published")
		}
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
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
	return PlotProfileFileCreated{Source: vprocore.PlotProfileSource{Name: request.Name, Path: path}, Table: request.Name + "_Profile", RuleCount: 0}, nil
}

func writePlotProfileFile(ctx context.Context, path string, request PlotProfileFileCreation) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	tx, err := db.Begin()
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
	if err := ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func createBlankProfileTable(ctx context.Context, tx *sql.Tx, name string, template vprocore.ProjectMetadataTable, proposal string) error {
	var collision int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name=? COLLATE NOCASE`, name+"_Profile").Scan(&collision); err != nil {
		return err
	}
	if collision != 0 {
		return errors.New("profile destination name already belongs to a physical SQLite object; no replacement")
	}
	if _, err := tx.ExecContext(ctx, strings.Replace(originalProfileTemplateSQL, vprocore.QuoteHeaderIdentifier("USysProfileTable"),
		vprocore.QuoteHeaderIdentifier(name+"_Profile"), 1)); err != nil {
		return err
	}
	if err := vprocore.AppendCreationProvenance(ctx, tx, "__VPRO_ProfileCreationHistory", profileCreationHistorySQL, proposal); err != nil {
		return err
	}
	stored, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", name+"_Profile", "", nil, "")
	if err != nil || !reflect.DeepEqual(stored, template) {
		return errors.Join(err, errors.New("new profile schema/empty rules differ from the independently reviewed template"))
	}
	return nil
}
