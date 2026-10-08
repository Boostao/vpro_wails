package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type tableCSVLeaseContext struct {
	context.Context
	doneCalls atomic.Int32
	queued    chan struct{}
}

func (ctx *tableCSVLeaseContext) Done() <-chan struct{} {
	// Three fast read leases precede the snapshot mutex's check and wait.
	if ctx.doneCalls.Add(1) == 5 {
		close(ctx.queued)
	}
	return ctx.Context.Done()
}

func tableCSVProjectWriter(t *testing.T, service *ContextService, statements string) {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.selection.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(statements); err != nil {
		t.Fatal(err)
	}
}

func TestOwnedTableCSVPhysicalProjectScopeAndDescriptions(t *testing.T) {
	service, state := contextServiceFixture(t)
	tableCSVProjectWriter(t, service, `DROP TABLE _table_metadata;
		CREATE TABLE _table_metadata(table_name TEXT,description TEXT);
		INSERT INTO _table_metadata VALUES
		('Sample_Other',NULL),('Sample_Other',''),('Sample_Other',''),
		('Sample_Other','  literal description  '),('OtherProject_Other','not owned');`)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
	if err != nil {
		t.Fatal(err)
	}
	if result.ContextID != state.ContextID || result.Project != "Sample" || result.ProjectPath != state.ProjectPath ||
		!result.DescriptionMetadataPresent || result.Document.Manifest.Table != "Sample_Other" {
		t.Fatal("table review lost actual owner/project/path/metadata availability:", result)
	}
	descriptions := result.Document.Manifest.Descriptions
	if len(descriptions) != 4 || descriptions[0].Value.Storage != "null" ||
		descriptions[1].Value.Storage != "text" || *descriptions[1].Value.Text != "" ||
		*descriptions[2].Value.Text != "" || *descriptions[3].Value.Text != "  literal description  " ||
		descriptions[1].RowID == descriptions[2].RowID {
		t.Fatal("physical NULL/empty/duplicate Description candidates changed:", descriptions)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Other", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTableCSV(context.Background(), result.Document)
	if err != nil || !reflect.DeepEqual(decoded, want) {
		t.Fatal("owned review filtered or changed whole physical table:", err)
	}
	result.Document.Manifest.Columns[0].Name = "caller change"
	*descriptions[3].Value.Text = "caller change"
	again, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
	if err != nil || again.Document.Manifest.Columns[0].Name == "caller change" ||
		*again.Document.Manifest.Descriptions[3].Value.Text != "  literal description  " {
		t.Fatal("owned review aliases an earlier caller's manifest:", err)
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !reflect.DeepEqual(config, after) {
		t.Fatal("read-only table review changed YAML:", err)
	}
}

func TestOwnedTableCSVMetadataAbsenceEmptyAndMalformedRemainDistinct(t *testing.T) {
	service, state := contextServiceFixture(t)
	for _, test := range []struct {
		name, sql string
		present   bool
		fail      bool
	}{
		{"absent", `DROP TABLE _table_metadata`, false, false},
		{"present empty", `CREATE TABLE _table_metadata(table_name TEXT,description TEXT)`, true, false},
		{"malformed", `DROP TABLE _table_metadata; CREATE TABLE _table_metadata(table_name TEXT)`, false, true},
		{"view", `DROP TABLE _table_metadata; CREATE VIEW _table_metadata AS SELECT 'Sample_Other' AS table_name,NULL AS description`, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tableCSVProjectWriter(t, service, test.sql)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			result, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
			if test.fail {
				if err == nil || !reflect.DeepEqual(result, ownedTableCSVReview{}) {
					t.Fatal("invalid physical Description metadata returned success/partial result:", err)
				}
			} else if err != nil || result.DescriptionMetadataPresent != test.present ||
				result.Document.Manifest.Descriptions == nil || len(result.Document.Manifest.Descriptions) != 0 {
				t.Fatal("absent and present-empty metadata were conflated:", err)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}

func TestOwnedTableCSVRejectsForeignDerivedAndStaleReadsAndRetriesCancelledLease(t *testing.T) {
	service, state := contextServiceFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, table := range []string{"", "sample_Other", "OtherProject_Other", "_table_metadata", "USysEnv",
		"Sample_Profile", "VLists.USysAllSpecs", "Sample_Other; DELETE FROM Sample_Env"} {
		result, err := service.readProjectTableCSV(context.Background(), state.ContextID, table)
		if err == nil || !reflect.DeepEqual(result, ownedTableCSVReview{}) {
			t.Fatal("foreign/derived/unavailable table returned success/partial document:", table, err)
		}
	}
	if result, err := service.readProjectTableCSV(context.Background(), "stale", "Sample_Other"); err == nil || !reflect.DeepEqual(result, ownedTableCSVReview{}) {
		t.Fatal("stale context returned a document")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.readProjectTableCSV(cancelled, state.ContextID, "Sample_Other"); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, ownedTableCSVReview{}) {
		t.Fatal("cancelled context returned success/partial document:", err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	waiting, cancelWaiting := context.WithCancel(context.Background())
	defer cancelWaiting()
	observed := &tableCSVLeaseContext{Context: waiting, queued: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		result, err := service.readProjectTableCSV(observed, state.ContextID, "Sample_Other")
		if !reflect.DeepEqual(result, ownedTableCSVReview{}) {
			done <- errors.New("cancelled queued lease returned a partial document")
			return
		}
		done <- err
	}()
	select {
	case <-observed.queued:
		cancelWaiting()
	case <-time.After(3 * time.Second):
		owner.mu.Unlock()
		t.Fatal("table review never reached the held snapshot mutex")
	}
	select {
	case err := <-done:
		owner.mu.Unlock()
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued snapshot cancellation lost context error:", err)
		}
	case <-time.After(3 * time.Second):
		owner.mu.Unlock()
		t.Fatal("cancelled table review waited indefinitely for the snapshot mutex")
	}
	if _, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other"); err != nil {
		t.Fatal("cancelled read damaged the pinned context; retry failed:", err)
	}
	assertProfileSUFiles(t, service, before)
	tableCSVProjectWriter(t, service, `ALTER TABLE Sample_Other RENAME TO Archived_Other;
		CREATE VIEW Sample_Other AS SELECT * FROM Archived_Other;`)
	if result, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other"); err == nil || !strings.Contains(err.Error(), "physical table") || !reflect.DeepEqual(result, ownedTableCSVReview{}) {
		t.Fatal("derived view impersonated the original physical core table:", err)
	}
}
