package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mattn/go-sqlite3"
)

func externalSUEnvironmentFixture(t *testing.T) (*ContextService, ProjectState, *externalSUEnvironmentSource) {
	t.Helper()
	service, state, _ := siteUnitEnvironmentFixture(t)
	path := filepath.Join(t.TempDir(), "selected ' # \u00e9.db")
	bytes, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := db.Exec(`ALTER TABLE Sample_SU RENAME TO External_SU;
		UPDATE _table_metadata SET table_name='External_SU' WHERE table_name='Sample_SU'`)
	if err := errors.Join(writeErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "External", path
	state, err = service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
	if err != nil || review == nil || len(review.Review.Changes) != 4 {
		t.Fatal("external reverse review unavailable", review, err)
	}
	return service, state, review
}

func TestExternalSUEnvironmentCommitPreservationAndReplay(t *testing.T) {
	service, state, approved := externalSUEnvironmentFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	if _, err := service.ReviewSiteUnitEnvironment(t.Context(), state.ContextID); err == nil {
		t.Fatal("legacy reverse API acquired external authority")
	}
	result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved)
	if err != nil || result == nil || result.ChangedRows != 2 || result.ChangedCells != 4 || result.HistoryID != "1" {
		t.Fatal("external transfer failed exact commit", result, err)
	}
	for role, original := range before {
		if os.SameFile(owner.attachmentInfo[role], owner.attachmentInfo["project"]) {
			continue
		}
		actual, err := os.ReadFile(owner.attachments[role])
		if err != nil || !reflect.DeepEqual(original, actual) {
			t.Fatal("external transfer changed support or external source", role, err)
		}
	}
	var unit, short, long string
	if err := owner.conn.QueryRowContext(t.Context(),
		`SELECT UserSiteUnit,SiteUnitShortName,SiteUnitLongName FROM project.Sample_Admin WHERE Plot='8229723'`).
		Scan(&unit, &short, &long); err != nil || unit != "  Review ' unit  " || short != unit || long != "Personal long" {
		t.Fatal("external selection or personal precedence changed", unit, short, long, err)
	}
	fresh, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
	if err != nil || fresh == nil || len(fresh.Review.Changes) != 0 || fresh.Review.HistoryHash == "" {
		t.Fatal("external committed review unavailable", fresh, err)
	}
	after := databaseBytes(t, owner.attachments)
	if result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved); err == nil || result != nil {
		t.Fatal("external completed proposal replay accepted", result, err)
	}
	assertProfileSUFiles(t, service, after)
}

func TestExternalSUEnvironmentDriftAndAtomicRollback(t *testing.T) {
	for _, variant := range []string{"source", "master", "personal", "target", "metadata", "schema", "locked", "abort", "late-other", "history-schema"} {
		t.Run(variant, func(t *testing.T) {
			service, state, approved := externalSUEnvironmentFixture(t)
			owner := service.projects.sqlite
			role, statement := "project", ""
			switch variant {
			case "source":
				role, statement = "su", `UPDATE External_SU SET SiteUnit='Drift' WHERE PlotNumber='8229723'`
			case "master":
				role, statement = "VLists", `UPDATE MasterSiteUnitList SET SiteSeriesLongName='Drift' WHERE SiteSeries='  Review '' unit  '`
			case "personal":
				role, statement = "VUser", `UPDATE UserSiteUnitList SET SiteSeriesLongName='Drift' WHERE SiteSeries='  Review '' unit  '`
			case "target":
				statement = `UPDATE Sample_Admin SET SiteUnitLongName='Drift' WHERE Plot='8229723'`
			case "metadata":
				role, statement = "su", `UPDATE _table_metadata SET description='Drift'`
			case "schema":
				role, statement = "su", `CREATE TABLE Unreviewed(X)`
			case "locked":
				statement = `UPDATE Sample_Env SET Flag=-1 WHERE PlotNumber='8229723'`
			case "abort":
				statement = `CREATE TRIGGER external_guard BEFORE UPDATE ON Sample_Admin
					WHEN OLD.Plot='8428935' BEGIN SELECT RAISE(ABORT,'Independent rejection'); END`
			case "late-other":
				statement = `CREATE TRIGGER external_guard AFTER UPDATE ON Sample_Admin
					BEGIN UPDATE Sample_Env SET EnteredBy='Unreviewed' WHERE PlotNumber='8229723'; END`
			case "history-schema":
				statement = `CREATE TABLE __VPRO_SUEnvironmentHistory(ID INTEGER,Created TEXT,Proposal TEXT)`
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			if variant == "abort" || variant == "late-other" {
				approved, err = service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
			}
			before := databaseBytes(t, owner.attachments)
			if result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved); err == nil || result != nil {
				t.Fatal("external drift/unreviewed write accepted", result, err)
			}
			assertProfileSUFiles(t, service, before)
			if variant == "history-schema" {
				if result, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID); err == nil || result != nil {
					t.Fatal("readonly external review accepted invalid history", result, err)
				}
			}
			if variant == "abort" || variant == "late-other" {
				if _, err := db.Exec(`DROP TRIGGER external_guard`); err != nil {
					t.Fatal(err)
				}
				approved, err = service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				if result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved); err != nil || result == nil || result.ChangedCells != 4 {
					t.Fatal("external correction/retry failed", result, err)
				}
			}
		})
	}
}

func TestExternalSUEnvironmentCancelledAndUnapprovedPreserveFiles(t *testing.T) {
	service, state, approved := externalSUEnvironmentFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := service.reviewExternalSUEnvironment(ctx, state.ContextID); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled external read accepted", result, err)
	}
	if result, err := service.transferExternalSUEnvironment(ctx, state.ContextID, approved); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled external transfer accepted", result, err)
	}
	if result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, nil); err == nil || result != nil {
		t.Fatal("unapproved external transfer accepted", result, err)
	}
	assertProfileSUFiles(t, service, before)
	if result, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID); err != nil || result == nil {
		t.Fatal("cancelled external read damaged coordinator", result, err)
	}
}

func TestExternalSUEnvironmentReservationsAndCancellation(t *testing.T) {
	for _, journal := range []string{"DELETE", "WAL"} {
		for _, cancelOperation := range []bool{false, true} {
			t.Run(journal+map[bool]string{false: "/release", true: "/cancel"}[cancelOperation], func(t *testing.T) {
				service, _, approved := externalSUEnvironmentFixture(t)
				owner := service.projects.sqlite
				for _, role := range []string{"project", "su", "VLists", "VUser"} {
					db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
					if err != nil {
						t.Fatal(err)
					}
					var mode string
					err = db.QueryRow(`PRAGMA journal_mode=` + journal).Scan(&mode)
					if err := errors.Join(err, db.Close()); err != nil {
						t.Fatal(err)
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				before := databaseBytes(t, owner.attachments)
				err := owner.withMetadataWriter(ctx, func(conn *sql.Conn) error {
					committed, err := withExternalSUEnvironmentTransactions(ctx, owner, conn, approved.Files, externalSUEnvironmentWriteHooks{},
						func(project *sql.Tx, sources map[string]*sql.Tx) error {
							for _, role := range []string{"project", "su", "VLists", "VUser"} {
								db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw")+
									"&_busy_timeout=1&_txlock=immediate")
								if err != nil {
									return err
								}
								tx, beginErr := db.BeginTx(t.Context(), nil)
								if beginErr == nil {
									t.Error("independent writer acquired reserved source", role)
									beginErr = tx.Rollback()
								}
								if closeErr := db.Close(); closeErr != nil {
									return closeErr
								}
								if beginErr == nil {
									return errors.New("source reservation did not reject writer")
								}
								var locked sqlite3.Error
								if !errors.As(beginErr, &locked) || locked.Code != sqlite3.ErrBusy && locked.Code != sqlite3.ErrLocked {
									return beginErr
								}
							}
							if cancelOperation {
								if _, err := project.ExecContext(ctx, `UPDATE Sample_Admin SET UserSiteUnit='Must rollback' WHERE Plot='8229723'`); err != nil {
									return err
								}
								cancel()
							}
							return nil
						})
					if cancelOperation {
						if committed || !errors.Is(err, context.Canceled) {
							t.Fatal("cancelled reservation committed or hid cancellation", committed, err)
						}
					} else if !committed || err != nil {
						t.Fatal("reserved observation could not finish", committed, err)
					}
					return err
				})
				if cancelOperation && !errors.Is(err, context.Canceled) || !cancelOperation && err != nil {
					t.Fatal(err)
				}
				assertProfileSUFiles(t, service, before)
				for _, role := range []string{"project", "su", "VLists", "VUser"} {
					db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw")+
						"&_busy_timeout=1&_txlock=immediate")
					if err != nil {
						t.Fatal(err)
					}
					tx, err := db.BeginTx(t.Context(), nil)
					if err != nil {
						t.Fatal("reservation leaked after cleanup", role, err)
					}
					if err := errors.Join(tx.Rollback(), db.Close()); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
