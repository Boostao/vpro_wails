package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLabelledReadErrorsPreserveCancellationAndRealFailures(t *testing.T) {
	for _, reason := range []error{context.Canceled, context.DeadlineExceeded} {
		if actual := labelledReadError("catalogue", fmt.Errorf("validation: %w", reason)); actual != reason {
			t.Fatal("transport cancellation acknowledgement acquired a domain prefix", actual)
		}
	}
	corruption := errors.New("checksum mismatch")
	actual := labelledReadError("catalogue", corruption)
	if !errors.Is(actual, corruption) || actual.Error() != "catalogue: checksum mismatch" {
		t.Fatal("real failure lost its context", actual)
	}
	closed := errors.New("close failed")
	combined := errors.Join(labelledReadError("catalogue", context.Canceled), closed)
	if !errors.Is(combined, closed) || combined.Error() == "context canceled" {
		t.Fatal("cleanup failure was suppressed as expected cancellation", combined)
	}
}

func TestStateDiscoveryCancellationAndRecovery(t *testing.T) {
	service, _, _, _ := sqliteServiceFixture(t)
	original, err := service.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := []func(context.Context) error{
		func(ctx context.Context) error { _, err := service.GetState(ctx); return err },
		func(ctx context.Context) error { _, _, err := service.discoverContext(ctx); return err },
		func(ctx context.Context) error {
			_, err := inspectFileContext(ctx, original.ProjectPath)
			return err
		},
		func(ctx context.Context) error {
			_, err := service.discoverSUsContext(ctx, ProjectInfo{Path: original.ProjectPath})
			return err
		},
		func(ctx context.Context) error {
			_, err := discoverHierarchiesContext(ctx, filepath.Join(service.root, "projects"))
			return err
		},
	}
	for index, read := range reads {
		if err := read(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("state helper %d did not cancel: %v", index, err)
		}
	}
	service.mu.Lock()
	waiting, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err = service.GetState(waiting)
	stop()
	service.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("state read retained queued selection lock", err)
	}
	recovered, err := service.GetState(context.Background())
	if err != nil || !reflect.DeepEqual(recovered, original) {
		t.Fatal("state did not recover unchanged", recovered, err)
	}
}

func TestStateDiscoveryInterruptsMetadataSQLWithoutDiagnosticFallback(t *testing.T) {
	service, _, _, _ := sqliteServiceFixture(t)
	original, err := service.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// A non-Sample file exercises the diagnostic branch, which must not swallow cancellation.
	path := filepath.Join(service.root, "projects", "ZCancel.db")
	content, err := os.ReadFile(original.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE _table_metadata RENAME TO _table_metadata_CancelProof;
 CREATE VIEW _table_metadata AS SELECT * FROM _table_metadata_CancelProof
 WHERE (WITH RECURSIVE work(x) AS
 (SELECT 1 UNION ALL SELECT x+1 FROM work WHERE x<1000000000)
 SELECT sum(x) FROM work)>0`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	state, err := service.GetState(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(state, ProjectState{}) {
		t.Fatal("cancelled discovery became a successful partial state/diagnostic", state, err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("metadata cancellation retained its SQL/selection lease")
	}
	if _, err := db.Exec(`DROP VIEW _table_metadata;
 ALTER TABLE _table_metadata_CancelProof RENAME TO _table_metadata;
 ALTER TABLE Sample_Env RENAME TO InspectionOnly_Env`); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.GetState(context.Background())
	if err != nil || recovered.ContextID != original.ContextID || recovered.ActiveProject != original.ActiveProject {
		t.Fatal("state could not recover", recovered, err)
	}
}
