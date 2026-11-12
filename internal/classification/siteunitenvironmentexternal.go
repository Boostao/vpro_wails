package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
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
	"sort"
	"strconv"
	"strings"
)

type externalSUEnvironmentSource struct {
	Review    SiteUnitEnvironmentReview
	Selection vprocore.DesktopSelection
	User      string
	Files     map[string]os.FileInfo
	Paths     map[string]string
	Witness   map[string]map[string]vprocore.ProjectMetadataTable
}

func externalSUEnvironmentFiles(owner *vprocore.SqliteContext) (map[string]os.FileInfo, error) {
	if err := owner.ValidateMetadataWriterFiles(); err != nil {
		return nil, err
	}
	if owner.Selection.SU == "None" || owner.AttachmentInfo["su"] == nil ||
		os.SameFile(owner.AttachmentInfo["su"], owner.AttachmentInfo["project"]) {
		return nil, errors.New("external SU/environment transfer requires a selected separate physical SU file")
	}
	files := make(map[string]os.FileInfo)
	for _, role := range []string{"project", "su", "VLists", "VUser"} {
		path, original := owner.Attachments[role], owner.AttachmentInfo[role]
		if path == "" || original == nil {
			return nil, fmt.Errorf("external SU/environment transfer requires the owned %s file", role)
		}
		handle, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("external SU/environment %s identity unavailable: %w", role, err)
		}
		info, statErr := handle.Stat()
		if err := errors.Join(statErr, handle.Close()); err != nil {
			return nil, fmt.Errorf("external SU/environment %s identity capture failed: %w", role, err)
		}
		if !os.SameFile(info, original) {
			return nil, fmt.Errorf("external SU/environment %s file was replaced; reopen the context", role)
		}
		files[role] = info
	}
	return files, nil
}

func compareExternalSUEnvironmentFiles(expected, observed map[string]os.FileInfo) error {
	if len(expected) != 4 || len(observed) != 4 {
		return errors.New("external SU/environment transfer has incomplete physical file provenance")
	}
	for _, role := range []string{"project", "su", "VLists", "VUser"} {
		if expected[role] == nil || observed[role] == nil || !os.SameFile(expected[role], observed[role]) {
			return fmt.Errorf("external SU/environment reviewed %s file identity changed", role)
		}
	}
	return nil
}

func readExternalSUEnvironmentTable(ctx context.Context, tx *sql.Tx, alias, name string) (vprocore.ProjectMetadataTable, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(alias)+
		`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, name).Scan(&count); err != nil || count != 1 {
		return vprocore.ProjectMetadataTable{}, errors.Join(err, fmt.Errorf("external SU/environment requires physical %s.%s", alias, name))
	}
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, alias, name, "external SU/environment source"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	return vprocore.ReadSQLiteStorageRows(ctx, tx, alias, name, "", nil, "")
}

func externalSUEnvironmentWitness(ctx context.Context, tx *sql.Tx, alias, name string) (map[string]vprocore.ProjectMetadataTable, error) {
	table, err := readExternalSUEnvironmentTable(ctx, tx, alias, name)
	if err != nil {
		return nil, err
	}
	schema, err := vprocore.ReadSQLiteStorageRows(ctx, tx, alias, "sqlite_master", "", nil, "")
	if err != nil {
		return nil, err
	}
	witness := map[string]vprocore.ProjectMetadataTable{name: table, "sqlite_master": schema}
	for _, row := range schema.Rows {
		if row.Cells[0].Text != nil && *row.Cells[0].Text == "table" &&
			row.Cells[1].Text != nil && *row.Cells[1].Text == "_table_metadata" {
			metadata, err := vprocore.ReadSQLiteStorageRows(ctx, tx, alias, "_table_metadata", "", nil, "")
			if err != nil {
				return nil, err
			}
			witness["_table_metadata"] = metadata
		}
	}
	return witness, nil
}

func readExternalSUEnvironmentSource(ctx context.Context, owner *vprocore.SqliteContext, contextID, user string,
	projectTx *sql.Tx, sourceTx map[string]*sql.Tx, aliases map[string]string) (*externalSUEnvironmentSource, error) {
	files, err := externalSUEnvironmentFiles(owner)
	if err != nil {
		return nil, err
	}
	source := &externalSUEnvironmentSource{
		Review: SiteUnitEnvironmentReview{ContextID: contextID, Project: owner.Selection.Project,
			SU: owner.Selection.SU, Path: owner.Selection.ProjectPath},
		Selection: owner.Selection, User: user, Files: files, Paths: make(map[string]string),
		Witness: make(map[string]map[string]vprocore.ProjectMetadataTable),
	}
	for role := range files {
		source.Paths[role] = owner.Attachments[role]
	}
	for _, item := range []struct {
		role, name string
		dest       *vprocore.ProjectMetadataTable
	}{
		{"project", source.Review.Project + "_Env", &source.Review.Env},
		{"project", source.Review.Project + "_Admin", &source.Review.Admin},
		{"su", source.Review.SU + "_SU", &source.Review.Source},
		{"VLists", "MasterSiteUnitList", &source.Review.Master},
		{"VUser", "UserSiteUnitList", &source.Review.Personal},
	} {
		tx := sourceTx[item.role]
		if item.role == "project" {
			tx = projectTx
		}
		if tx == nil || aliases[item.role] == "" {
			return nil, errors.New("external SU/environment source has no owned transaction")
		}
		table, err := readExternalSUEnvironmentTable(ctx, tx, aliases[item.role], item.name)
		if err != nil {
			return nil, err
		}
		*item.dest = table
		source.Witness[item.role], err = externalSUEnvironmentWitness(ctx, tx, aliases[item.role], item.name)
		if err != nil {
			return nil, err
		}
	}
	source.Review.Changes, err = planSiteUnitEnvironment(source.Review.Env, source.Review.Admin,
		source.Review.Source, source.Review.Master, source.Review.Personal)
	if err != nil {
		return nil, err
	}
	var objects int
	if err := projectTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(aliases["project"])+
		`.sqlite_master WHERE name COLLATE NOCASE=?`, siteUnitEnvironmentHistory).Scan(&objects); err != nil {
		return nil, err
	}
	if objects != 0 {
		if err := vprocore.VerifyTechnicalProvenanceIn(ctx, projectTx, aliases["project"], siteUnitEnvironmentHistory,
			siteUnitEnvironmentHistorySQL, "external SU/environment transfer"); err != nil {
			return nil, err
		}
		table, err := vprocore.ReadSQLiteStorageRows(ctx, projectTx, aliases["project"], siteUnitEnvironmentHistory, "", nil, "")
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(table)
		if err != nil {
			return nil, err
		}
		source.Review.HistoryHash = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return source, nil
}

func ReviewExternalSUEnvironment(s *vprocore.ContextService, ctx context.Context, contextID string) (*externalSUEnvironmentSource, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*externalSUEnvironmentSource, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		return vprocore.WithPublicationReadSnapshot(ctx, owner, vprocore.PublicationReadSnapshotHooks{}, func(tx *sql.Tx) (*externalSUEnvironmentSource, error) {
			return readExternalSUEnvironmentSource(ctx, owner, contextID, plots.CurrentUser, tx,
				map[string]*sql.Tx{"su": tx, "VLists": tx, "VUser": tx},
				map[string]string{"project": "project", "su": "su", "VLists": "VLists", "VUser": "VUser"})
		})
	})
}

type externalSUEnvironmentReservation struct {
	path  string
	info  os.FileInfo
	roles []string
	db    *sql.DB
	tx    *sql.Tx
}

type externalSUEnvironmentWriteHooks struct {
	beforeCommit func() error
	afterCleanup func() error
}

func withExternalSUEnvironmentTransactions(ctx context.Context, owner *vprocore.SqliteContext, project *sql.Conn,
	files map[string]os.FileInfo, hooks externalSUEnvironmentWriteHooks,
	operation func(*sql.Tx, map[string]*sql.Tx) error) (committed bool, resultErr error) {
	var reservations []*externalSUEnvironmentReservation
	defer func() {
		for i := len(reservations) - 1; i >= 0; i-- {
			item := reservations[i]
			if item.tx != nil {
				if err := item.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, fmt.Errorf("external SU/environment source rollback: %w", err))
				}
			}
			if item.db != nil {
				resultErr = errors.Join(resultErr, item.db.Close())
			}
		}
		if hooks.afterCleanup != nil {
			resultErr = errors.Join(resultErr, hooks.afterCleanup())
		}
	}()
	for _, role := range []string{"project", "su", "VLists", "VUser"} {
		if files[role] == nil {
			return false, errors.New("external SU/environment source reservation has incomplete identities")
		}
		var existing *externalSUEnvironmentReservation
		for _, item := range reservations {
			if os.SameFile(files[role], item.info) {
				existing = item
				break
			}
		}
		if existing == nil {
			existing = &externalSUEnvironmentReservation{path: owner.Attachments[role], info: files[role]}
			reservations = append(reservations, existing)
		}
		existing.roles = append(existing.roles, role)
	}
	sort.Slice(reservations, func(i, j int) bool {
		return strings.ToLower(reservations[i].path) < strings.ToLower(reservations[j].path)
	})
	transactions := make(map[string]*sql.Tx)
	for _, item := range reservations {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		var err error
		if item.roles[0] == "project" {
			item.tx, err = project.BeginTx(context.WithoutCancel(ctx), nil)
		} else {
			item.db, err = sql.Open("sqlite3", sqlutil.SqliteFileURI(item.path, "rw")+"&_busy_timeout=5000&_txlock=immediate")
			if err != nil {
				return false, err
			}
			item.db.SetMaxOpenConns(1)
			item.tx, err = item.db.BeginTx(context.WithoutCancel(ctx), nil)
		}
		if err != nil {
			return false, fmt.Errorf("external SU/environment source reservation unavailable: %w", err)
		}
		for _, role := range item.roles {
			transactions[role] = item.tx
		}
	}
	if err := operation(transactions["project"], transactions); err != nil {
		return false, err
	}
	if hooks.beforeCommit != nil {
		if err := hooks.beforeCommit(); err != nil {
			return false, err
		}
	}
	fresh, err := externalSUEnvironmentFiles(owner)
	if err != nil {
		return false, err
	}
	if err := compareExternalSUEnvironmentFiles(files, fresh); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := transactions["project"].Commit(); err != nil {
		return false, &externalSUEnvironmentCommitUncertainError{err}
	}
	return true, nil
}

func TransferExternalSUEnvironment(s *vprocore.ContextService, ctx context.Context, contextID string,
	approved *externalSUEnvironmentSource) (*SiteUnitEnvironmentResult, error) {
	return TransferExternalSUEnvironmentWithHooks(s, ctx, contextID, approved, externalSUEnvironmentWriteHooks{})
}

func TransferExternalSUEnvironmentWithHooks(s *vprocore.ContextService, ctx context.Context, contextID string,
	approved *externalSUEnvironmentSource, hooks externalSUEnvironmentWriteHooks) (*SiteUnitEnvironmentResult, error) {
	return TransferExternalSUEnvironmentOwned(s, ctx, contextID, approved, nil, hooks)
}

func TransferExternalSUEnvironmentOwned(s *vprocore.ContextService, ctx context.Context, contextID string,
	approved *externalSUEnvironmentSource, attempt *externalSUEnvironmentAttemptIdentity,
	hooks externalSUEnvironmentWriteHooks) (*SiteUnitEnvironmentResult, error) {
	result, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*SiteUnitEnvironmentResult, error) {
		if approved == nil || approved.Review.ContextID != contextID || len(approved.Review.Changes) == 0 {
			return nil, errors.New("external SU/environment transfer requires a nonempty current-context approved plan")
		}
		if attempt != nil {
			if err := attempt.validate(); err != nil {
				return nil, err
			}
		}
		owner := plots.Projects.Sqlite
		result := &SiteUnitEnvironmentResult{}
		committed := false
		err := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) error {
			files, err := externalSUEnvironmentFiles(owner)
			if err != nil {
				return err
			}
			if err := compareExternalSUEnvironmentFiles(approved.Files, files); err != nil {
				return err
			}
			committed, err = withExternalSUEnvironmentTransactions(ctx, owner, conn, files, hooks, func(tx *sql.Tx, transactions map[string]*sql.Tx) error {
				fresh, err := readExternalSUEnvironmentSource(ctx, owner, contextID, plots.CurrentUser, tx, transactions,
					map[string]string{"project": "main", "su": "main", "VLists": "main", "VUser": "main"})
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(fresh.Review, approved.Review) || !reflect.DeepEqual(fresh.Witness, approved.Witness) ||
					!reflect.DeepEqual(fresh.Paths, approved.Paths) || fresh.Selection != approved.Selection || fresh.User != approved.User {
					return errors.New("external SU/environment source, owner, definitions, target or history changed; no transfer applied")
				}
				original, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				expected := vprocore.ProjectMetadataTable{Columns: fresh.Review.Admin.Columns, Rows: append([]vprocore.ProjectMetadataRow(nil), fresh.Review.Admin.Rows...)}
				columns, err := vprocore.SiteUnitTransferColumns(expected, "UserSiteUnit", "SiteUnitShortName", "SiteUnitLongName")
				if err != nil {
					return err
				}
				rows := make(map[string]bool)
				for _, change := range fresh.Review.Changes {
					value, err := vprocore.MetadataCellValue(change.After)
					if err != nil {
						return err
					}
					rowID, err := strconv.ParseInt(change.AdminRowID, 10, 64)
					if err != nil {
						return err
					}
					update, err := tx.ExecContext(ctx, `UPDATE `+vprocore.QuoteHeaderIdentifier(fresh.Review.Project+"_Admin")+
						` SET `+vprocore.QuoteHeaderIdentifier(change.Field)+`=? WHERE rowid=? AND typeof(Plot)='text' AND CAST(Plot AS BLOB)=CAST(? AS BLOB)`,
						value, rowID, change.PlotNumber)
					if err != nil {
						return err
					}
					if count, err := update.RowsAffected(); err != nil || count != 1 {
						return errors.Join(err, errors.New("external SU/environment transfer lost its sole literal physical Admin target"))
					}
					for i, row := range expected.Rows {
						if row.RowID == change.AdminRowID {
							expected.Rows[i].Cells = append([]vprocore.ProjectMetadataCell(nil), row.Cells...)
							expected.Rows[i].Cells[columns[change.Field]] = change.After
						}
					}
					rows[change.AdminRowID] = true
					result.ChangedCells++
				}
				result.ChangedRows = len(rows)
				proposal, err := externalSUEnvironmentProposal(fresh, attempt)
				if err != nil {
					return err
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, siteUnitEnvironmentHistory, siteUnitEnvironmentHistorySQL,
					string(proposal), "external SU/environment transfer"); err != nil {
					return err
				}
				observed, err := vprocore.EnvironmentSiteUnitTables(ctx, tx)
				if err != nil {
					return err
				}
				if err := vprocore.VerifySiteUnitTransferTables(original, observed,
					map[string]vprocore.ProjectMetadataTable{fresh.Review.Project + "_Admin": expected}, siteUnitEnvironmentHistory); err != nil {
					return err
				}
				if err := tx.QueryRowContext(ctx, `SELECT CAST(ID AS TEXT) FROM __VPRO_SUEnvironmentHistory ORDER BY ID DESC LIMIT 1`).
					Scan(&result.HistoryID); err != nil {
					return err
				}
				if id, err := strconv.ParseInt(result.HistoryID, 10, 64); err != nil || id < 1 {
					return errors.Join(err, errors.New("external SU/environment transfer lost its exact positive history identity"))
				}
				return nil
			})
			return err
		})
		if err != nil {
			if committed {
				return result, fmt.Errorf("external SU/environment transfer committed but cleanup failed; never replay: %w", err)
			}
			return nil, err
		}
		return result, nil
	})
	if result == nil && err != nil {
		var uncertain *externalSUEnvironmentCommitUncertainError
		if !errors.As(err, &uncertain) {
			return nil, &externalSUEnvironmentNotCommittedError{err}
		}
	}
	return result, err
}
