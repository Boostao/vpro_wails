package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/reports/breakreport"
	"github.com/mattn/go-sqlite3"
)

func breakReportFixture(t *testing.T) (*ContextService, ProjectState) {
	t.Helper()
	service, state := contextServiceFixture(t)
	mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], `
		DELETE FROM USysBreakTable;
		INSERT INTO USysBreakTable VALUES
		(1,'R1','A','P1'),(2,'R1','A','P2'),(3,NULL,NULL,'P3'),
		(4,'',NULL,'P4'),(5,NULL,'','P5'),(6,'','','P6'),
		(7,'=1+1','001','P7'),(8,'R2','B','P8');`)
	return service, state
}

func TestBreakReportLiteralDistinctPagingAndWitness(t *testing.T) {
	service, state := breakReportFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	before := databaseBytes(t, service.projects.sqlite.attachments)
	text := func(value string) *string { return &value }
	expected := []breakreport.Pair{{}, {Reference: text("")}, {BreakCode: text("")},
		{Reference: text(""), BreakCode: text("")},
		{Reference: text("=1+1"), BreakCode: text("001")},
		{Reference: text("R1"), BreakCode: text("A")},
		{Reference: text("R2"), BreakCode: text("B")}}
	full, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100)
	if err != nil || !reflect.DeepEqual(full.Rows, expected) || full.Total != 7 ||
		full.ContextID != state.ContextID || full.SourcePath != path || len(full.SourceSHA256) != 64 {
		t.Fatal("source DISTINCT/literal/NULL/empty/provenance differs", full, err)
	}
	for _, page := range []struct{ offset, limit, count int }{{1, 2, 2}, {6, 100, 1}, {7, 1, 0}, {math.MaxInt, 100, 0}} {
		result, err := service.reviewBreakReport(context.Background(), state.ContextID, page.offset, page.limit)
		if err != nil || result.Total != 7 || result.Offset != page.offset || result.Limit != page.limit ||
			result.Rows == nil || len(result.Rows) != page.count || result.SourceSHA256 != full.SourceSHA256 {
			t.Fatal("bounded page differs", page, result, err)
		}
		if page.count > 0 && !reflect.DeepEqual(result.Rows, expected[page.offset:page.offset+page.count]) {
			t.Fatal("page selected different source pairs", page, result)
		}
	}
	*full.Rows[5].Reference = "caller changed"
	retry, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100)
	if err != nil || !reflect.DeepEqual(retry.Rows, expected) {
		t.Fatal("caller mutation contaminated subsequent read", retry, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("readonly report changed database family")
	}
	mutateContextFixture(t, path, `UPDATE USysBreakTable SET "Order"=900,PlotNumber='ignored' WHERE rowid=1`)
	changed, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100)
	if err != nil || !reflect.DeepEqual(changed.Rows, expected) || changed.SourceSHA256 == retry.SourceSHA256 {
		t.Fatal("ignored physical source fields were dropped from witness", changed, err)
	}
}

func TestBreakReportRequestsContextsAndPhysicalOwner(t *testing.T) {
	service, state := breakReportFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, request := range []struct{ offset, limit int }{{-1, 1}, {0, 0}, {0, 101}} {
		result, err := service.reviewBreakReport(context.Background(), state.ContextID, request.offset, request.limit)
		if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) {
			t.Fatal("invalid request succeeded", result, err)
		}
	}
	for _, id := range []string{"", "foreign", state.ContextID + " "} {
		result, err := service.reviewBreakReport(context.Background(), id, 0, 1)
		if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) {
			t.Fatal("foreign context succeeded", result, err)
		}
	}
	if result, err := service.reviewBreakReport(nil, state.ContextID, 0, 1); err == nil || !reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("nil context succeeded", result, err)
	}
	var absent *ContextService
	if _, err := absent.reviewBreakReport(context.Background(), state.ContextID, 0, 1); err == nil {
		t.Fatal("absent service succeeded")
	}
	owner := service.projects.sqlite
	original := owner.attachments["VPro64"]
	replacement := filepath.Join(t.TempDir(), "same bytes # foreign.db")
	if err := os.WriteFile(replacement, before["VPro64"], 0600); err != nil {
		t.Fatal(err)
	}
	owner.attachments["VPro64"] = replacement
	result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1)
	owner.attachments["VPro64"] = original
	if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("byte-identical foreign source supplied ownership", result, err)
	}
	current, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1); err == nil {
		t.Fatal("retired context retained ownership")
	}
	if _, err := service.reviewBreakReport(context.Background(), current.ContextID, 0, 1); err != nil {
		t.Fatal("current owner cannot retry", err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatal("request/ownership refusal changed database family")
	}
}

func TestBreakReportPhysicalSchemaRefusal(t *testing.T) {
	for _, query := range []string{
		`ALTER TABLE USysBreakTable DROP COLUMN PlotNumber`,
		`ALTER TABLE USysBreakTable RENAME COLUMN Reference TO reference`,
		`ALTER TABLE USysBreakTable ADD COLUMN Extra TEXT`,
		`ALTER TABLE USysBreakTable ADD COLUMN Extra TEXT GENERATED ALWAYS AS (Reference) VIRTUAL`,
		`ALTER TABLE USysBreakTable ADD COLUMN rowid INTEGER`,
		`DROP TABLE USysBreakTable; CREATE VIEW USysBreakTable AS SELECT 1 AS "Order",'R' AS Reference,'A' AS BreakCode,'P1' AS PlotNumber`,
		`DROP TABLE USysBreakTable; CREATE TABLE USysBreakTable("Order" INTEGER PRIMARY KEY,Reference TEXT,BreakCode TEXT,PlotNumber TEXT) WITHOUT ROWID;
		 INSERT INTO USysBreakTable VALUES(1,'R','A','P1')`,
	} {
		t.Run(query, func(t *testing.T) {
			service, state := breakReportFixture(t)
			mutateContextFixture(t, service.projects.sqlite.attachments["VPro64"], query)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1)
			if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) ||
				!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatal("unsupported physical source was accepted/repaired", result, err)
			}
		})
	}
}

func TestBreakReportProjectedStorageAndCorrection(t *testing.T) {
	for _, item := range []struct {
		affinity string
		valid    bool
	}{{"TEXT", true}, {"BLOB", true}, {"", true}, {"INTEGER", false}, {"NUMERIC", false}, {"REAL", false}} {
		for _, field := range []string{"Reference", "BreakCode"} {
			t.Run(item.affinity+field, func(t *testing.T) {
				service, state := breakReportFixture(t)
				path := service.projects.sqlite.attachments["VPro64"]
				referenceType, codeType := "TEXT", "TEXT"
				if field == "Reference" {
					referenceType = item.affinity
				} else {
					codeType = item.affinity
				}
				mutateContextFixture(t, path, `DROP TABLE USysBreakTable; CREATE TABLE USysBreakTable("Order" INTEGER,Reference `+
					referenceType+`,BreakCode `+codeType+`,PlotNumber TEXT); INSERT INTO USysBreakTable VALUES(1,'001','002','P1')`)
				result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1)
				if item.valid {
					if err != nil || result.Total != 1 || *result.Rows[0].Reference != "001" || *result.Rows[0].BreakCode != "002" {
						t.Fatal("literal leading-zero text changed", result, err)
					}
				} else if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) || !strings.Contains(err.Error(), field) {
					t.Fatal("numeric affinity invented source text", result, err)
				}
			})
		}
	}
	service, state := breakReportFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	for _, assignment := range []string{
		`Reference=CAST(X'FF' AS TEXT)`, `BreakCode=CAST(X'FF' AS TEXT)`, `PlotNumber=CAST(X'FF' AS TEXT)`,
		`Reference=X'52'`, `BreakCode=X'41'`, `Reference='r1'`,
		`Reference=printf('%0260d',0)`, `BreakCode=printf('%0260d',0)`,
	} {
		mutateContextFixture(t, path, `UPDATE USysBreakTable SET `+assignment+` WHERE rowid=1`)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		result, err := service.reviewBreakReport(context.Background(), state.ContextID, 6, 1)
		if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) ||
			!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
			t.Fatal("invalid off-page original returned partial success", assignment, result, err)
		}
		mutateContextFixture(t, path, `UPDATE USysBreakTable SET Reference='R1',BreakCode='A',PlotNumber='P1' WHERE rowid=1`)
		if _, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1); err != nil {
			t.Fatal("explicit source correction cannot retry", assignment, err)
		}
	}
}

func TestBreakReportPhysicalAndOriginalByteBounds(t *testing.T) {
	service, state := breakReportFixture(t)
	path := service.projects.sqlite.attachments["VPro64"]
	mutateContextFixture(t, path, `DELETE FROM USysBreakTable`)
	empty, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100)
	if err != nil || empty.Total != 0 || empty.Rows == nil {
		t.Fatal("empty physical source unavailable", empty, err)
	}
	mutateContextFixture(t, path, `WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000)
		INSERT INTO USysBreakTable(Reference,BreakCode) SELECT 'R','A' FROM n`)
	exact, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100)
	if err != nil || exact.Total != 1 {
		t.Fatal("exact physical row bound refused", exact, err)
	}
	mutateContextFixture(t, path, `INSERT INTO USysBreakTable(Reference,BreakCode) VALUES('R','A')`)
	if result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1); err == nil ||
		!strings.Contains(err.Error(), "10000-row") || !reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("duplicate physical rows evaded input bound", result, err)
	}
	mutateContextFixture(t, path, `DELETE FROM USysBreakTable; INSERT INTO USysBreakTable(Reference,BreakCode) VALUES('R','A')`)
	for _, size := range []int{breakReportMaxOriginalBytes - 2, breakReportMaxOriginalBytes - 1} {
		mutateContextFixture(t, path, fmt.Sprintf(`UPDATE USysBreakTable SET PlotNumber=zeroblob(%d)`, size))
		result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 1)
		if size == breakReportMaxOriginalBytes-2 {
			if err != nil || result.Total != 1 {
				t.Fatal("exact complete-original byte bound refused", result, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "8MiB") || !reflect.DeepEqual(result, BreakReportReview{}) {
			t.Fatal("ignored original field evaded byte bound", result, err)
		}
	}
}

func TestBreakReportInFlightCancellationCoordinatorRetry(t *testing.T) {
	service, state := breakReportFixture(t)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.reviewBreakReport(ctx, state.ContextID, 0, 1); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("pre-cancel succeeded", result, err)
	}
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.reviewBreakReport(ctx, state.ContextID, 0, 1)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("queued snapshot cancellation failed", result, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	observed := false
	setAuthorizer := func(authorize func(int, string, string, string) int) {
		t.Helper()
		if err := owner.conn.Raw(func(raw any) error {
			driver, valid := raw.(*sqlite3.SQLiteConn)
			if !valid {
				return errors.New("PC-ORD Break Report cancellation requires the actual SQLite coordinator")
			}
			driver.RegisterAuthorizer(authorize)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	setAuthorizer(func(action int, table, column, database string) int {
		if !observed && action == sqlite3.SQLITE_READ && table == "USysBreakTable" && database == "VPro64" {
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
	result, err = service.reviewBreakReport(ctx, state.ContextID, 0, 1)
	<-finished
	select {
	case <-entered:
	default:
		t.Fatal("in-flight cancellation never entered the actual source query")
	}
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("in-flight cancelled query returned success/partial data", result, err)
	}
	setAuthorizer(nil)
	if result, err := service.reviewBreakReport(context.Background(), state.ContextID, 0, 100); err != nil || result.Total != 7 {
		t.Fatal("cancelled read lost coordinator/attachments or blocked retry", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("cancelled reads changed physical database family")
	}
}
