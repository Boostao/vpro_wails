package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	PlotNumber string                       `json:"plotNumber"`
	EnvRowID   string                       `json:"envRowId"`
	AdminRowID string                       `json:"adminRowId"`
	SURowID    string                       `json:"suRowId"`
	Before     vprocore.ProjectMetadataCell `json:"before"`
	After      vprocore.ProjectMetadataCell `json:"after"`
}

func planEnvironmentSiteUnits(env, admin, su vprocore.ProjectMetadataTable) ([]EnvironmentSiteUnitChange, error) {
	envColumns, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("environment transfer source: %w", err)
	}
	adminColumns, err := vprocore.SiteUnitTransferColumns(admin, "Plot", "UserSiteUnit")
	if err != nil {
		return nil, fmt.Errorf("environment transfer admin link: %w", err)
	}
	suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("environment transfer SU target: %w", err)
	}
	envRows, err := vprocore.SiteUnitTransferIndex(env, envColumns["PlotNumber"])
	if err != nil {
		return nil, err
	}
	adminRows, err := vprocore.SiteUnitTransferIndex(admin, adminColumns["Plot"])
	if err != nil {
		return nil, err
	}
	suRows, err := vprocore.SiteUnitTransferIndex(su, suColumns["PlotNumber"])
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
		for _, cell := range []vprocore.ProjectMetadataCell{before, after} {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
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
		changes = append(changes, EnvironmentSiteUnitChange{
			plot, sources[0].RowID, parents[0].RowID, target.RowID, vprocore.CloneSiteUnitCell(before), vprocore.CloneSiteUnitCell(after)})
	}
	return changes, nil
}

type EnvironmentSiteUnitReview struct {
	ContextID   string                        `json:"contextId"`
	Project     string                        `json:"project"`
	SU          string                        `json:"su"`
	Path        string                        `json:"path"`
	Env         vprocore.ProjectMetadataTable `json:"env"`
	Admin       vprocore.ProjectMetadataTable `json:"admin"`
	Target      vprocore.ProjectMetadataTable `json:"target"`
	Changes     []EnvironmentSiteUnitChange   `json:"changes"`
	HistoryHash string                        `json:"historyHash"`
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
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "review", "confirmed"); err != nil {
		return err
	}
	var nested struct {
		Review json.RawMessage `json:"review"`
	}
	if err := json.Unmarshal(data, &nested); err != nil {
		return err
	}
	if err := vprocore.DecodeProfileLifecycleJSON(nested.Review, &decoded.Review,
		"contextId", "project", "su", "path", "env", "admin", "target", "changes", "historyHash"); err != nil {
		return err
	}
	*request = EnvironmentSiteUnitTransfer(decoded)
	return nil
}

func ValidateEnvironmentSiteUnitOwner(c *vprocore.SqliteContext) error {
	if err := c.ValidateMetadataWriterFiles(); err != nil {
		return err
	}
	if c.Selection.SU == "None" || c.AttachmentInfo["su"] == nil ||
		!os.SameFile(c.AttachmentInfo["project"], c.AttachmentInfo["su"]) {
		return errors.New("environment/SU transfer requires an explicitly selected SU in the owned project file; arbitrary-file writes are unavailable")
	}
	return nil
}

func readEnvironmentSiteUnitReview(ctx context.Context, tx *sql.Tx, c *vprocore.SqliteContext, contextID string) (EnvironmentSiteUnitReview, error) {
	if err := ValidateEnvironmentSiteUnitOwner(c); err != nil {
		return EnvironmentSiteUnitReview{}, err
	}
	review := EnvironmentSiteUnitReview{ContextID: contextID, Project: c.Selection.Project,
		SU: c.Selection.SU, Path: c.Selection.ProjectPath}
	for _, item := range []struct {
		name string
		dest *vprocore.ProjectMetadataTable
	}{{review.Project + "_Env", &review.Env}, {review.Project + "_Admin", &review.Admin}, {review.SU + "_SU", &review.Target}} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, item.name).
			Scan(&count); err != nil || count != 1 {
			return EnvironmentSiteUnitReview{}, errors.Join(err, errors.New("environment/SU transfer requires original physical tables, not substituted views"))
		}
		table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", item.name, "", nil, "")
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
		if err := vprocore.VerifyTechnicalProvenance(ctx, tx, environmentSiteUnitHistory, environmentSiteUnitHistorySQL, "environment/SU transfer"); err != nil {
			return EnvironmentSiteUnitReview{}, err
		}
		history, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", environmentSiteUnitHistory, "", nil, "")
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

func ReviewEnvironmentSiteUnits(s *vprocore.ContextService, ctx context.Context, contextID string) (EnvironmentSiteUnitReview, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (EnvironmentSiteUnitReview, error) {
		c := plots.Projects.Sqlite
		var review EnvironmentSiteUnitReview
		err := c.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
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

func TransferEnvironmentSiteUnits(s *vprocore.ContextService, ctx context.Context, contextID string, request EnvironmentSiteUnitTransfer) (*EnvironmentSiteUnitResult, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*EnvironmentSiteUnitResult, error) {
		if !request.Confirmed || request.Review.ContextID != contextID || len(request.Review.Changes) == 0 {
			return nil, errors.New("environment/SU transfer requires explicit confirmation of a nonempty current-context review")
		}
		c := plots.Projects.Sqlite
		result := &EnvironmentSiteUnitResult{}
		committed := false
		err := c.WithMetadataWriter(ctx, func(conn *sql.Conn) (resultErr error) {
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
			original, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			target := original[fresh.SU+"_SU"]
			expected := vprocore.ProjectMetadataTable{Columns: target.Columns, Rows: append([]vprocore.ProjectMetadataRow(nil), target.Rows...)}
			columns, err := vprocore.SiteUnitTransferColumns(target, "SiteUnit")
			if err != nil {
				return err
			}
			for _, change := range fresh.Changes {
				value, err := vprocore.MetadataCellValue(change.After)
				if err != nil {
					return err
				}
				rowID, err := strconv.ParseInt(change.SURowID, 10, 64)
				if err != nil {
					return err
				}
				updated, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(fresh.SU+"_SU")+
					` SET SiteUnit=? WHERE rowid=? AND PlotNumber COLLATE BINARY=?`, value, rowID, change.PlotNumber)
				if err != nil {
					return err
				}
				if count, err := updated.RowsAffected(); err != nil || count != 1 {
					return errors.Join(err, errors.New("environment/SU transfer lost its sole physical target"))
				}
				for i, row := range expected.Rows {
					if row.RowID == change.SURowID {
						cells := append([]vprocore.ProjectMetadataCell(nil), row.Cells...)
						cells[columns["SiteUnit"]] = change.After
						expected.Rows[i].Cells = cells
					}
				}
				result.ChangedRows++
			}
			proposal, err := json.Marshal(struct {
				Request EnvironmentSiteUnitTransfer `json:"request"`
				User    string                      `json:"user"`
			}{request, plots.CurrentUser})
			if err != nil {
				return err
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, environmentSiteUnitHistory, environmentSiteUnitHistorySQL,
				string(proposal), "environment/SU transfer"); err != nil {
				return err
			}
			observed, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
			if err != nil {
				return err
			}
			if err := vprocore.VerifySiteUnitTransferTables(original, observed,
				map[string]vprocore.ProjectMetadataTable{fresh.SU + "_SU": expected}, environmentSiteUnitHistory); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, `SELECT CAST(ID AS TEXT) FROM __VPRO_EnvironmentSUHistory ORDER BY ID DESC LIMIT 1`).
				Scan(&result.HistoryID); err != nil {
				return err
			}
			if id, err := strconv.ParseInt(result.HistoryID, 10, 64); err != nil || id < 1 {
				return errors.Join(err, errors.New("environment/SU transfer could not seal a positive exact history identity"))
			}
			if err := ValidateEnvironmentSiteUnitOwner(c); err != nil {
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
