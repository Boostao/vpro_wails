package main

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func userLogFixture(t *testing.T) (*ContextService, ProjectState) {
	t.Helper()
	service, state := contextServiceFixture(t)
	mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], `
		DELETE FROM USysUserLog;
		INSERT INTO USysUserLog VALUES
		('A','2024-01-02 03:04:05',NULL,'PC'),
		('',NULL,'',NULL),
		(NULL,0,'2024-01-03 04:05:06',X'00FF'),
		('A','2024-01-02 03:04:05',NULL,'PC'),
		('historical username longer than fifty UTF16 code units',1.125,NULL,'M');`)
	return service, state
}

func TestUserLogOriginalFiltersPagingAndTaggedStorage(t *testing.T) {
	service, state := userLogFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, item := range []struct {
		mode, offset, limit, total int
		ids                        []string
	}{
		{1, 0, 200, 5, []string{"1", "2", "3", "4", "5"}},
		{2, 0, 200, 3, []string{"1", "4", "5"}},
		{1, 1, 2, 5, []string{"2", "3"}},
		{2, 1, 1, 3, []string{"4"}},
		{1, math.MaxInt, 200, 5, []string{}},
	} {
		result, err := service.reviewUserLog(context.Background(), state.ContextID, item.mode, item.offset, item.limit)
		if err != nil {
			t.Fatal(err)
		}
		var ids = []string{}
		for _, row := range result.Table.Rows {
			ids = append(ids, row.RowID)
		}
		if result.ContextID != state.ContextID || result.SourcePath != service.projects.sqlite.attachments["VPro64"] ||
			result.Mode != item.mode || result.Offset != item.offset || result.Limit != item.limit ||
			result.Total != item.total || !reflect.DeepEqual(ids, item.ids) || len(result.Table.Columns) != 4 {
			t.Fatal("source filter/page/provenance differs", result, item)
		}
	}
	all, err := service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	rows := all.Table.Rows
	if *rows[0].Cells[1].Text != "2024-01-02 03:04:05" || rows[0].Cells[2].Storage != "null" ||
		*rows[1].Cells[0].Text != "" || rows[1].Cells[1].Storage != "null" || *rows[1].Cells[2].Text != "" ||
		rows[2].Cells[0].Storage != "null" || *rows[2].Cells[1].Integer != "0" ||
		*rows[2].Cells[3].BlobHex != "00ff" || *rows[4].Cells[1].Real != 1.125 ||
		len(*rows[4].Cells[0].Text) <= 50 || !reflect.DeepEqual(rows[0].Cells, rows[3].Cells) {
		t.Fatal("historical storage, NULL/empty or duplicate physical rows were changed", rows)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("readonly user log changed the database family")
	}
}

func TestUserLogInvalidRequestsAndStaleContext(t *testing.T) {
	service, state := userLogFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, item := range []struct{ mode, offset, limit int }{
		{0, 0, 1}, {3, 0, 1}, {-1, 0, 1}, {1, -1, 1}, {1, 0, 0}, {1, 0, 201},
	} {
		result, err := service.reviewUserLog(context.Background(), state.ContextID, item.mode, item.offset, item.limit)
		if err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("invalid request returned a successful/partial review", item, result, err)
		}
	}
	for _, id := range []string{"", "stale", state.ContextID + " "} {
		result, err := service.reviewUserLog(context.Background(), id, 1, 0, 1)
		if err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("stale identity returned a review", id, result, err)
		}
	}
	if _, err := service.reviewUserLog(nil, state.ContextID, 1, 0, 1); err == nil {
		t.Fatal("nil request context accepted")
	}
	var absent *ContextService
	if _, err := absent.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1); err == nil {
		t.Fatal("absent service accepted")
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("invalid requests changed the database family")
	}
	current, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1); err == nil {
		t.Fatal("retired context still owns the log")
	}
	if _, err := service.reviewUserLog(context.Background(), current.ContextID, 1, 0, 1); err != nil {
		t.Fatal("new owned context cannot read its log", err)
	}
}

func TestUserLogMalformedTextCorrectionAndSchemaRefusal(t *testing.T) {
	service, state := userLogFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	mutateContextFixture(t, path, `UPDATE USysUserLog SET User=CAST(X'FF' AS TEXT) WHERE rowid=1`)
	for _, mode := range []int{1, 2} {
		result, err := service.reviewUserLog(context.Background(), state.ContextID, mode, 0, 1)
		if err == nil || !strings.Contains(err.Error(), "malformed Unicode") || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("malformed text was repaired or hidden behind a partial page", result, err)
		}
	}
	mutateContextFixture(t, path, `UPDATE USysUserLog SET User='A' WHERE rowid=1`)
	if _, err := service.reviewUserLog(context.Background(), state.ContextID, 2, 0, 1); err != nil {
		t.Fatal("explicit correction could not retry", err)
	}
	for _, query := range []string{
		`ALTER TABLE USysUserLog ADD COLUMN Extra TEXT`,
		`ALTER TABLE USysUserLog DROP COLUMN Extra; ALTER TABLE USysUserLog RENAME COLUMN OutTime TO outtime`,
		`DROP TABLE USysUserLog`,
		`CREATE VIEW USysUserLog AS SELECT 'A' AS User, NULL AS InTime, NULL AS OutTime, 'PC' AS LocalMachine`,
	} {
		mutateContextFixture(t, path, query)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		result, err := service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1)
		if err == nil || !reflect.DeepEqual(result, UserLogReview{}) ||
			!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
			t.Fatal("unsupported/missing schema was repaired or accepted", result, err)
		}
	}
}

func TestUserLogExactPhysicalRowBoundAndEmptyTable(t *testing.T) {
	service, state := userLogFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	mutateContextFixture(t, path, `DELETE FROM USysUserLog`)
	empty, err := service.reviewUserLog(context.Background(), state.ContextID, 2, 0, 200)
	if err != nil || empty.Total != 0 || empty.Table.Rows == nil || len(empty.Table.Columns) != 4 {
		t.Fatal("valid empty table unavailable", empty, err)
	}
	mutateContextFixture(t, path, `WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000)
		INSERT INTO USysUserLog(User) SELECT 'synthetic' FROM n`)
	exact, err := service.reviewUserLog(context.Background(), state.ContextID, 2, 9999, 200)
	if err != nil || exact.Total != 10000 || len(exact.Table.Rows) != 1 || exact.Table.Rows[0].RowID != "10000" {
		t.Fatal("exact physical bound was not accepted", exact, err)
	}
	mutateContextFixture(t, path, `INSERT INTO USysUserLog(User) VALUES ('one beyond')`)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, mode := range []int{1, 2} {
		result, err := service.reviewUserLog(context.Background(), state.ContextID, mode, 0, 1)
		if err == nil || !strings.Contains(err.Error(), "no rows were truncated") || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("over-bound source was truncated", result, err)
		}
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("bound refusal changed the database family")
	}
}

func TestUserLogCancellationRetryAndPhysicalOwnership(t *testing.T) {
	service, state := userLogFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.reviewUserLog(ctx, state.ContextID, 1, 0, 1); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, UserLogReview{}) {
		t.Fatal("cancelled request returned a review", result, err)
	}

	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.reviewUserLog(ctx, state.ContextID, 1, 0, 1)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, UserLogReview{}) {
		t.Fatal("queued snapshot acquisition did not cancel", result, err)
	}
	if _, err := service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1); err != nil {
		t.Fatal("cancelled acquisition lost the coordinator", err)
	}
	original := owner.attachments["VPro64"]
	replacement := filepath.Join(t.TempDir(), "byte-identical different log.db")
	if err := os.WriteFile(replacement, before["VPro64"], 0600); err != nil {
		t.Fatal(err)
	}
	owner.attachments["VPro64"] = replacement
	result, err = service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1)
	owner.attachments["VPro64"] = original
	if err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
		t.Fatal("byte-identical unrelated log supplied ownership", result, err)
	}
	if _, err := service.reviewUserLog(context.Background(), state.ContextID, 1, 0, 1); err != nil {
		t.Fatal("restored owner cannot retry", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("cancellation/ownership refusal changed the database family")
	}
}

func TestUserLogInFlightCountCancellationAndRetry(t *testing.T) {
	service, state := userLogFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	observed := false
	setAuthorizer := func(authorize func(int, string, string, string) int) {
		t.Helper()
		if err := owner.conn.Raw(func(raw any) error {
			driver, valid := raw.(*sqlite3.SQLiteConn)
			if !valid {
				return errors.New("user log cancellation requires the actual SQLite coordinator")
			}
			driver.RegisterAuthorizer(authorize)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	setAuthorizer(func(action int, table, column, database string) int {
		if !observed && action == sqlite3.SQLITE_READ && table == "USysUserLog" && database == "VPro64" {
			observed = true
			close(entered)
			<-release
		}
		return sqlite3.SQLITE_OK
	})
	defer setAuthorizer(nil)
	go func() {
		defer close(finished)
		select {
		case <-entered:
		case <-time.After(10 * time.Second):
		}
		cancel()
		close(release)
	}()
	result, err := service.reviewUserLog(ctx, state.ContextID, 1, 0, 1)
	<-finished
	select {
	case <-entered:
	default:
		t.Fatal("cancellation did not enter the actual owned log query")
	}
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, UserLogReview{}) {
		t.Fatal("in-flight cancelled count returned a partial/success-shaped review", result, err)
	}
	setAuthorizer(nil)
	result, err = service.reviewUserLog(context.Background(), state.ContextID, 2, 0, 200)
	if err != nil || result.Total != 3 {
		t.Fatal("in-flight cancellation lost attachments/views or blocked retry", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("in-flight cancellation/retry changed the database family")
	}
}
