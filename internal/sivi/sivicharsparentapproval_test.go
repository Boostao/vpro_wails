package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSIVICHARSApprovalRequiresExactPhysicalProjectAndReferenceFiles(t *testing.T) {
	service, state, _ := charsEntryServiceFixture(t)
	path := service.contexts.Projects.Sqlite.Attachments["VLists"]
	copyPath := filepath.Join(t.TempDir(), "byte identical # reference.db")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "identical-copy", "wrong-project", "valid"} {
		t.Run(kind, func(t *testing.T) {
			_, err := vprocore.WithContextPlotRequest(context.Background(), service.contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
				owner := plots.Projects.Sqlite
				hooks := sourceform.SiviParentWriteHooks{Prepare: func(conn *sql.Conn) (func(), error) {
					if kind != "missing" {
						selected := path
						if kind == "identical-copy" {
							selected = copyPath
						}
						if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS chars_entry_refs`, sqlutil.SqliteFileURI(selected, "ro")); err != nil {
							return nil, err
						}
					}
					return func() {}, nil
				}}
				err := sourceform.WithSIVIParentWriteTransaction(context.Background(), owner, "CHARS reference identity witness", func(tx *sql.Tx, _ string) error {
					observedOwner := &vprocore.SqliteContext{AttachmentInfo: map[string]os.FileInfo{}}
					for role, info := range owner.AttachmentInfo {
						observedOwner.AttachmentInfo[role] = info
					}
					if kind == "wrong-project" {
						observedOwner.AttachmentInfo["project"] = owner.AttachmentInfo["VLists"]
					}
					err := verifySIVICHARSReferenceOwner(context.Background(), tx, observedOwner)
					if kind == "valid" && err != nil || kind != "valid" && err == nil {
						t.Fatal("physical alias ownership result incorrect", kind, err)
					}
					return nil
				}, hooks)
				return true, err
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSIVICHARSApprovalHoldsActualFamilyReadLockUntilTransactionEnds(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	edits := []sourceform.SiviParentScalarEdit{charsMandatoryEdit(t, review, "SuccessionalStatus")}
	refPath := service.contexts.Projects.Sqlite.Attachments["VLists"]
	competing, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(refPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer competing.Close()
	conn, err := competing.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "PRAGMA busy_timeout=0"); err != nil {
		t.Fatal(err)
	}
	var rowID int64
	var description sql.NullString
	if err := conn.QueryRowContext(context.Background(), `SELECT rowid,ItemDescription FROM USysTableOfLists WHERE ListName='SuccessionalStatus' LIMIT 1`).Scan(&rowID, &description); err != nil {
		t.Fatal(err)
	}
	_, err = vprocore.WithContextPlotRequest(context.Background(), service.contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		owner := plots.Projects.Sqlite
		hooks := sourceform.SiviParentWriteHooks{Prepare: func(conn *sql.Conn) (func(), error) {
			if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS chars_entry_refs`, sqlutil.SqliteFileURI(refPath, "ro")); err != nil {
				return nil, err
			}
			return func() {}, nil
		}}
		err := sourceform.WithSIVIParentWriteTransaction(context.Background(), owner, "CHARS family lock witness", func(tx *sql.Tx, suAlias string) error {
			parent, err := readSIVICHARSParentForWrite(context.Background(), tx, owner, suAlias, state.ContextID, "108050")
			if err != nil {
				return err
			}
			assignments, err := planSIVICHARSParentWritableEntry(context.Background(), parent, edits)
			if err != nil {
				return err
			}
			if err := approveSIVICHARSRequiredReferences(context.Background(), tx, owner, service.readers, assignments); err != nil {
				return err
			}
			_, err = conn.ExecContext(context.Background(), `UPDATE USysTableOfLists SET ItemDescription='competing' WHERE rowid=?`, rowID)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
				t.Fatal("mandatory catalogue drift committed before parent transaction ended", err)
			}
			return nil
		}, hooks)
		return true, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `UPDATE USysTableOfLists SET ItemDescription='after release' WHERE rowid=?`, rowID); err != nil {
		t.Fatal("completed approval leaked its family read lock", err)
	}
	if _, err := conn.ExecContext(context.Background(), `UPDATE USysTableOfLists SET ItemDescription=? WHERE rowid=?`, description, rowID); err != nil {
		t.Fatal(err)
	}
}

func TestSIVICHARSOwnedEntryConcurrentReviewedSavesCommitExactlyOnce(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	request := SIVICHARSParentEntryWrite{Original: review.Original, Edits: []sourceform.SIVIParentCellEdit{
		charsMandatoryEdit(t, review, "SuccessionalStatus"),
	}}

	start := make(chan struct{})
	results := make(chan *sourceform.SIVIParentWriteResult, 2)
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, err := service.Save(context.Background(), state.ContextID, "108050", request)
			if err != nil {
				failures <- err
			} else {
				results <- result
			}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(failures)
	success, refused := 0, 0
	for result := range results {
		if result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
			t.Fatal("colliding Save returned incomplete success", result)
		}
		success++
	}
	for failure := range failures {
		if errors.Is(failure, context.Canceled) || !strings.Contains(failure.Error(), "changed") {
			t.Fatal("colliding Save failed outside original source CAS", failure)
		}
		refused++
	}
	if success != 1 || refused != 1 {
		t.Fatal("colliding reviewed Saves did not commit exactly once", success, refused)
	}
}
