package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func summaryFieldListWriterFixture(t *testing.T) (*SummaryVegetationFieldListService, SummaryVegetationFieldListRequest, string) {
	t.Helper()
	service, id := summaryFieldListOwnedFixture(t)
	summaryFieldListWriterExec(t, service, `CREATE TABLE WriterSourceAudit (Entry TEXT);
		INSERT INTO WriterSourceAudit VALUES ('source audit untouched')`)
	review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	request := SummaryVegetationFieldListRequest{RequestID: "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a854",
		Review: *review, Actions: []SummaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "row", RowID: "2"}}}
	return service, request, summaryFieldListWriterJSON(t, request)
}

func summaryFieldListWriterJSON(t *testing.T, request SummaryVegetationFieldListRequest) string {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func summaryFieldListWriterExec(t *testing.T, service *SummaryVegetationFieldListService, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["VPro64"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(t.Context(), statement)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
}

func summaryFieldListWriterReopen(t *testing.T, service *SummaryVegetationFieldListService) ProjectMetadataTable {
	t.Helper()
	db, err := openReadOnly(service.contexts.projects.sqlite.attachments["VPro64"])
	if err != nil {
		t.Fatal(err)
	}
	table, err := readSQLiteStorageRows(t.Context(), db, "main", summaryVegetationFieldListTable, "", nil, "")
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	return table
}

func summaryFieldListWriterCount(t *testing.T, service *SummaryVegetationFieldListService) int {
	t.Helper()
	db, err := openReadOnly(service.contexts.projects.sqlite.attachments["VPro64"])
	if err != nil {
		t.Fatal(err)
	}
	var present, count int
	err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE name=?`, summaryFieldListReceiptTable).Scan(&present)
	if err == nil && present != 0 {
		err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM "__VPRO_SummaryFieldListReceipt"`).Scan(&count)
	}
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	if present == 0 {
		return -1
	}
	return count
}

func summaryFieldListWriterAssertPreserved(t *testing.T, service *SummaryVegetationFieldListService, before map[string][]byte, config []byte) {
	t.Helper()
	after := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	delete(after, "VPro64")
	delete(before, "VPro64")
	if !reflect.DeepEqual(after, before) {
		t.Fatal("coordinator changed another attachment or its source audit")
	}
	got, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(got, config) {
		t.Fatal("coordinator changed configuration", err)
	}
	db, err := openReadOnly(service.contexts.projects.sqlite.attachments["VPro64"])
	if err != nil {
		t.Fatal(err)
	}
	var audit string
	err = db.QueryRowContext(t.Context(), `SELECT Entry FROM WriterSourceAudit`).Scan(&audit)
	var invented int
	if err == nil {
		err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE name='USysAuditTrail'`).Scan(&invented)
	}
	if err := errors.Join(err, db.Close()); err != nil || audit != "source audit untouched" || invented != 0 {
		t.Fatal("source audit changed/invented", audit, invented, err)
	}
}

func TestSummaryVegetationFieldListWriterCommitReplayIndependentSevenCells(t *testing.T) {
	service, request, raw := summaryFieldListWriterFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	immediateObserved := false
	outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{
		beforeCommit: func() error {
			db, err := sql.Open("sqlite3", sqliteFileURI(request.Review.SourcePath, "rw")+"&_busy_timeout=0")
			if err != nil {
				return err
			}
			_, writeErr := db.ExecContext(t.Context(), `UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE rowid=1`)
			immediateObserved = writeErr != nil && strings.Contains(writeErr.Error(), "locked")
			return errors.Join(db.Close(), func() error {
				if !immediateObserved {
					return errors.New("independent writer did not observe immediate transaction lock")
				}
				return nil
			}())
		},
	})
	if err != nil || !outcome.Committed || !outcome.Changed || !outcome.Verified || outcome.Replayed || !immediateObserved {
		t.Fatal("owned commit failed", outcome, err)
	}
	got := summaryFieldListWriterReopen(t, service)
	want := [][]any{
		{int64(1), "Env", "duplicate", "A", int64(-1), "sum", "F1"},
		{int64(1), "Env", "duplicate", "A", int64(0), "sum", "F1"},
		{int64(-1), "Admin", "hidden", "", int64(-1), nil, "H"},
		{nil, nil, "NULL order", nil, int64(-1), nil, nil},
		{int64(2), "Env", nil, nil, int64(-1), nil, "F2"},
	}
	if len(got.Rows) != len(want) {
		t.Fatal("physical row count differs")
	}
	for i, row := range got.Rows {
		if !reflect.DeepEqual(row.Cells, summaryFieldListExpected(t, want[i])) {
			t.Fatal("independent seven source cells differ", i, row)
		}
	}
	if !reflect.DeepEqual(got, outcome.Plan.Proposed) || !reflect.DeepEqual(outcome.Plan.Original, request.Review.Physical) ||
		len(outcome.Plan.Assignments) != 2 || summaryFieldListWriterCount(t, service) != 1 {
		t.Fatal("complete plan or atomic receipt differs", outcome)
	}
	summaryFieldListWriterAssertPreserved(t, service, before, config)
	committedBytes := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	replay, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{})
	if err != nil || !replay.Committed || !replay.Replayed || !replay.Verified || !reflect.DeepEqual(replay.Plan, outcome.Plan) ||
		!reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), committedBytes) {
		t.Fatal("stable replay mutated or failed independent verification", replay, err)
	}
	request.Actions = []SummaryVegetationFieldListAction{{Kind: "none"}}
	if collision, err := commitSummaryVegetationFieldList(t.Context(), service, summaryFieldListWriterJSON(t, request),
		summaryFieldListWriterHooks{}); err == nil || collision.Committed {
		t.Fatal("UUID collision accepted", collision, err)
	}
	if !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), committedBytes) {
		t.Fatal("collision changed source or receipt")
	}
}

func TestSummaryVegetationFieldListWriterNoopCreatesNothing(t *testing.T) {
	for _, actions := range [][]SummaryVegetationFieldListAction{
		{}, {{Kind: "row", RowID: "1"}}, {{Kind: "all", Selected: true}, {Kind: "none"}, {Kind: "row", RowID: "3", Selected: true}},
	} {
		// Hidden row checkbox remains unavailable; only the first two are genuine no-ops.
		service, request, _ := summaryFieldListWriterFixture(t)
		request.Actions = actions
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		outcome, err := commitSummaryVegetationFieldList(t.Context(), service, summaryFieldListWriterJSON(t, request), summaryFieldListWriterHooks{})
		if len(actions) < 2 {
			if err != nil || outcome.Committed || outcome.Changed || !outcome.Verified {
				t.Fatal("no-op failed", outcome, err)
			}
		} else if err == nil {
			t.Fatal("hidden row was enabled")
		}
		if summaryFieldListWriterCount(t, service) != -1 || !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
			t.Fatal("no-op/refused action created technical history or wrote")
		}
	}
}

func TestSummaryVegetationFieldListWriterGlobalNoneExistingAuditAndNoop(t *testing.T) {
	service, request, _ := summaryFieldListWriterFixture(t)
	summaryFieldListWriterExec(t, service, `CREATE TABLE USysAuditTrail (Sentinel TEXT NOT NULL);
		INSERT INTO USysAuditTrail VALUES ('historical source audit')`)
	request.Actions = []SummaryVegetationFieldListAction{{Kind: "none"}}
	outcome, err := commitSummaryVegetationFieldList(t.Context(), service, summaryFieldListWriterJSON(t, request), summaryFieldListWriterHooks{})
	if err != nil || !outcome.Committed || !outcome.Verified || len(outcome.Plan.Assignments) != 3 {
		t.Fatal("global none did not include hidden and NULL source rows", outcome, err)
	}
	want := cloneSummaryVegetationFieldListTable(request.Review.Physical)
	zero := "0"
	for i := range want.Rows {
		want.Rows[i].Cells[4] = ProjectMetadataCell{Storage: "integer", Integer: &zero}
	}
	if !reflect.DeepEqual(summaryFieldListWriterReopen(t, service), want) {
		t.Fatal("global none changed more than physical Report or omitted hidden rows")
	}
	db, err := openReadOnly(request.Review.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var audit string
	var count int
	err = db.QueryRowContext(t.Context(), `SELECT Sentinel FROM USysAuditTrail`).Scan(&audit)
	if err == nil {
		err = db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM USysAuditTrail`).Scan(&count)
	}
	if err := errors.Join(err, db.Close()); err != nil || audit != "historical source audit" || count != 1 {
		t.Fatal("technical receipt became a phantom source option audit", audit, count, err)
	}
	request.Review.Physical = want
	request.RequestID = "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a855"
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	noop, err := commitSummaryVegetationFieldList(t.Context(), service, summaryFieldListWriterJSON(t, request), summaryFieldListWriterHooks{})
	if err != nil || noop.Committed || !noop.Verified || noop.Changed ||
		summaryFieldListWriterCount(t, service) != 1 ||
		!reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
		t.Fatal("global none no-op persisted receipt or rewrote source", noop, err)
	}
}

func TestSummaryVegetationFieldListWriterLiteralProvenance(t *testing.T) {
	for _, field := range []string{"context", "project", "projectPath", "su", "suPath", "sourcePath", "source"} {
		t.Run(field, func(t *testing.T) {
			service, request, _ := summaryFieldListWriterFixture(t)
			switch field {
			case "context":
				request.Review.ContextID += "-stale"
			case "project":
				request.Review.Project += "-other"
			case "projectPath":
				request.Review.ProjectPath += "-other"
			case "su":
				request.Review.SU = "Other"
				request.Review.SUPath = request.Review.ProjectPath
			case "suPath":
				request.Review.SUPath = request.Review.ProjectPath
			case "sourcePath":
				request.Review.SourcePath = service.contexts.projects.sqlite.attachments["VLists"]
			case "source":
				request.Review.Source += "-other"
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, summaryFieldListWriterJSON(t, request),
				summaryFieldListWriterHooks{}); err == nil || outcome.Committed {
				t.Fatal("unowned literal provenance accepted", outcome, err)
			}
			if !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
				t.Fatal("provenance refusal wrote")
			}
		})
	}
}

func TestSummaryVegetationFieldListWriterReceiptInsertionAndTriggerDriftRollback(t *testing.T) {
	for _, mode := range []string{"insertion", "assignmentDrift", "receiptDrift", "receiptPlan"} {
		t.Run(mode, func(t *testing.T) {
			service, request, raw := summaryFieldListWriterFixture(t)
			statement := `CREATE TABLE ` + summaryFieldListReceiptSchema[len("CREATE TABLE "):] + `;`
			switch mode {
			case "insertion":
				statement += `CREATE TRIGGER WriterReject BEFORE INSERT ON "__VPRO_SummaryFieldListReceipt" BEGIN SELECT RAISE(ABORT,'receipt denied'); END`
			case "assignmentDrift":
				statement += `CREATE TRIGGER WriterReject AFTER UPDATE OF Report ON USysShortVegSummaryFieldList BEGIN UPDATE USysShortVegSummaryFieldList SET Label='drift' WHERE rowid=NEW.rowid; END`
			case "receiptDrift":
				statement += `CREATE TRIGGER WriterReject AFTER INSERT ON "__VPRO_SummaryFieldListReceipt" BEGIN UPDATE USysShortVegSummaryFieldList SET Label='drift' WHERE rowid=1; END`
			case "receiptPlan":
				statement += `CREATE TRIGGER WriterReject AFTER INSERT ON "__VPRO_SummaryFieldListReceipt" BEGIN UPDATE "__VPRO_SummaryFieldListReceipt" SET Plan='{}' WHERE RequestID=NEW.RequestID; END`
			}
			summaryFieldListWriterExec(t, service, statement)
			if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err == nil || outcome.Committed {
				t.Fatal("receipt failure/drift committed", outcome, err)
			}
			if !reflect.DeepEqual(summaryFieldListWriterReopen(t, service), request.Review.Physical) ||
				summaryFieldListWriterCount(t, service) != 0 {
				t.Fatal("rollback failed all seven source cells/history")
			}
			summaryFieldListWriterExec(t, service, `DROP TRIGGER WriterReject`)
			if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
				t.Fatal("corrected retry failed", outcome, err)
			}
		})
	}
}

func TestSummaryVegetationFieldListWriterInFlightMutationCancellationAndRetry(t *testing.T) {
	service, request, raw := summaryFieldListWriterFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan struct {
		outcome summaryFieldListWriterOutcome
		err     error
	}, 1)
	go func() {
		outcome, err := commitSummaryVegetationFieldList(ctx, service, raw, summaryFieldListWriterHooks{
			configure: func(conn *sql.Conn) error {
				if err := conn.Raw(func(driver any) error {
					return driver.(*sqlite3.SQLiteConn).RegisterFunc("writerHold", func() int64 {
						close(entered)
						<-release
						return 1
					}, false)
				}); err != nil {
					return err
				}
				_, err := conn.ExecContext(ctx, `CREATE TEMP TRIGGER WriterHold AFTER UPDATE OF Report ON VPro64.USysShortVegSummaryFieldList
					WHEN NEW.rowid=1 BEGIN SELECT writerHold(); END`)
				return err
			},
		})
		finished <- struct {
			outcome summaryFieldListWriterOutcome
			err     error
		}{outcome, err}
	}()
	select {
	case <-entered:
		cancel()
		close(release)
	case result := <-finished:
		t.Fatal("failed before actual mutation trigger", result)
	case <-time.After(10 * time.Second):
		cancel()
		close(release)
		t.Fatal("mutation trigger was not reached")
	}
	select {
	case result := <-finished:
		if result.outcome.Committed || !errors.Is(result.err, context.Canceled) {
			t.Fatal("actual mutation cancellation accepted", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled disposable writer did not clean up")
	}
	if !reflect.DeepEqual(summaryFieldListWriterReopen(t, service), request.Review.Physical) ||
		summaryFieldListWriterCount(t, service) != -1 {
		t.Fatal("cancelled statement/receipt escaped rollback")
	}
	if _, err := service.GetSummaryVegetationFieldList(t.Context(), request.Review.ContextID); err != nil {
		t.Fatal("cancellation discarded pinned read coordinator", err)
	}
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
		t.Fatal("retry after in-flight cancellation failed", outcome, err)
	}
}

func TestSummaryVegetationFieldListWriterPersistedReceiptStrictness(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE "__VPRO_SummaryFieldListReceipt" SET Plan='{}'`,
		`UPDATE "__VPRO_SummaryFieldListReceipt" SET Request=Request || ' '`,
		`UPDATE "__VPRO_SummaryFieldListReceipt" SET Request=CAST(Request AS BLOB)`,
		`UPDATE "__VPRO_SummaryFieldListReceipt" SET Plan=CAST(X'FF' AS TEXT)`,
		`PRAGMA ignore_check_constraints=ON; UPDATE "__VPRO_SummaryFieldListReceipt" SET Version=2`,
		`ALTER TABLE "__VPRO_SummaryFieldListReceipt" ADD COLUMN Unexpected TEXT`,
		`INSERT INTO "__VPRO_SummaryFieldListReceipt" SELECT CAST(RequestID AS BLOB),Version,Request,Plan FROM "__VPRO_SummaryFieldListReceipt"`,
		`UPDATE "__VPRO_SummaryFieldListReceipt" SET RequestID=CAST(RequestID AS BLOB)`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, _, raw := summaryFieldListWriterFixture(t)
			if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
				t.Fatal(outcome, err)
			}
			summaryFieldListWriterExec(t, service, mutation)
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err == nil ||
				outcome.Verified || outcome.Committed {
				t.Fatal("malformed receipt became success-shaped fallback", outcome, err)
			}
			if !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
				t.Fatal("invalid receipt refusal mutated")
			}
		})
	}
}

func TestSummaryVegetationFieldListWriterCancellationBeforeCommitRemovesNewReceiptTable(t *testing.T) {
	service, request, raw := summaryFieldListWriterFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	outcome, err := commitSummaryVegetationFieldList(ctx, service, raw, summaryFieldListWriterHooks{
		beforeCommit: func() error { cancel(); return nil },
	})
	if !errors.Is(err, context.Canceled) || outcome.Committed || outcome.Verified {
		t.Fatal("cancellation before commit escaped explicit guard", outcome, err)
	}
	if !reflect.DeepEqual(summaryFieldListWriterReopen(t, service), request.Review.Physical) ||
		summaryFieldListWriterCount(t, service) != -1 {
		t.Fatal("cancelled new-table transaction changed source or left technical table")
	}
	if retry, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !retry.Committed {
		t.Fatal("cancelled before-commit retry failed", retry, err)
	}
}

func TestSummaryVegetationFieldListWriterReplayReportsHistoricalCommitOnCurrentDrift(t *testing.T) {
	service, _, raw := summaryFieldListWriterFixture(t)
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
		t.Fatal(outcome, err)
	}
	summaryFieldListWriterExec(t, service, `UPDATE USysShortVegSummaryFieldList SET Label='external change' WHERE rowid=1`)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{})
	if err == nil || !outcome.Committed || !outcome.Replayed || outcome.Verified || outcome.Plan == nil {
		t.Fatal("replay drift was silently accepted or erased historical commit", outcome, err)
	}
	if !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
		t.Fatal("replay drift overwrote external source")
	}
}

func TestSummaryVegetationFieldListWriterOpenedOwnershipAliasAndRetry(t *testing.T) {
	service, _, raw := summaryFieldListWriterFixture(t)
	owner := service.contexts.projects.sqlite
	path, info := owner.attachments["VLists"], owner.attachmentInfo["VLists"]
	for _, mode := range []string{"path", "identity", "shared", "missing"} {
		switch mode {
		case "path":
			owner.attachments["VLists"] = owner.attachments["VPro64"]
		case "identity":
			owner.attachmentInfo["VLists"] = owner.attachmentInfo["VPro64"]
		case "shared":
			owner.attachments["VLists"] = owner.attachments["VPro64"]
			owner.attachmentInfo["VLists"] = owner.attachmentInfo["VPro64"]
		case "missing":
			owner.attachmentInfo["VLists"] = nil
		}
		outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{})
		owner.attachments["VLists"], owner.attachmentInfo["VLists"] = path, info
		if err == nil || outcome.Committed {
			t.Fatal("unowned/shared attachment accepted", mode, outcome, err)
		}
	}
	if summaryFieldListWriterCount(t, service) != -1 {
		t.Fatal("ownership refusal wrote receipt")
	}
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
		t.Fatal("exact ownership restoration retry failed", outcome, err)
	}
}

func TestSummaryVegetationFieldListWriterPhysicalReplacementRefusalRetry(t *testing.T) {
	service, _, raw := summaryFieldListWriterFixture(t)
	owner := service.contexts.projects.sqlite
	path := owner.attachments["VPro64"]
	bytesBefore, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup := path + ".writer-original"
	// Release only the fixture's pinned reader attachment so Windows permits the
	// physical replacement. The owner retains its original opened identity.
	if _, err := owner.conn.ExecContext(t.Context(), `DETACH DATABASE VPro64`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := owner.conn.ExecContext(t.Context(), `ATTACH DATABASE ? AS VPro64`, sqliteFileURI(path, "ro")); err != nil {
			t.Error("restore pinned reader alias", err)
		}
	}()
	if err := os.Rename(path, backup); err != nil {
		t.Fatal("fixture reader release did not allow physical replacement", err)
	}
	restored := false
	defer func() {
		if restored {
			return
		}
		if err := errors.Join(os.Remove(path), os.Rename(backup, path)); err != nil {
			t.Error("restore original physical fixture", err)
		}
	}()
	if err := os.WriteFile(path, bytesBefore, 0600); err != nil {
		t.Fatal(err)
	}
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err == nil || outcome.Committed {
		t.Fatal("byte-identical physical replacement accepted", outcome, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	restored = true
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil || !outcome.Committed {
		t.Fatal("restored physical identity retry failed", outcome, err)
	}
}

func TestSummaryVegetationFieldListWriterPostcommitCleanupRetainsReceipt(t *testing.T) {
	service, _, raw := summaryFieldListWriterFixture(t)
	cleanup := errors.New("owned writer cleanup witness")
	outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{
		afterClose: func() error { return cleanup },
	})
	if !errors.Is(err, cleanup) || !outcome.Committed || !outcome.Changed || !outcome.Verified || outcome.Plan == nil ||
		summaryFieldListWriterCount(t, service) != 1 {
		t.Fatal("postcommit cleanup zeroed irreversible receipt", outcome, err)
	}
	if replay, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err != nil ||
		!replay.Replayed || !replay.Committed || !replay.Verified {
		t.Fatal("postcommit cleanup replay failed", replay, err)
	}
}

func TestSummaryVegetationFieldListWriterQueuedSwitchLeaseReleased(t *testing.T) {
	service, request, raw := summaryFieldListWriterFixture(t)
	state, err := service.contexts.projects.GetState(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	finished, switched := make(chan error, 1), make(chan error, 1)
	go func() {
		outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{
			beforeCommit: func() error { close(entered); <-release; return nil },
		})
		if err == nil && !outcome.Committed {
			err = errors.New("writer failed to commit before lease release")
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		close(release)
		t.Fatal("writer never acquired lease")
	}
	if service.contexts.projects.operationMu.TryLock() {
		service.contexts.projects.operationMu.Unlock()
		close(release)
		t.Fatal("writer lost operation lease")
	}
	if service.contexts.projects.sqlite.mu.TryLock() {
		service.contexts.projects.sqlite.mu.Unlock()
		close(release)
		t.Fatal("writer lost owner mutex")
	}
	go func() {
		_, err := service.contexts.SwitchContext(request.Review.ContextID, contextSelection(state))
		switched <- err
	}()
	select {
	case err := <-switched:
		close(release)
		t.Fatal("switch escaped writer lease", err)
	case <-time.After(40 * time.Millisecond):
	}
	close(release)
	for _, ch := range []chan error{finished, switched} {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatal("writer/switch failed", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("writer lease deadlocked queued switch")
		}
	}
	if outcome, err := commitSummaryVegetationFieldList(t.Context(), service, raw, summaryFieldListWriterHooks{}); err == nil || outcome.Committed {
		t.Fatal("old context replay bypassed switched ownership", outcome, err)
	}
}
