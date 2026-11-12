package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPlotPoolWarmReadsReuseOwnedHandleAndPreserveFiles(t *testing.T) {
	seedBytes, err := vprocore.SampleFiles.ReadFile("resources/Sample.db")
	if err != nil {
		t.Fatal(err)
	}
	service, state, original := vprotest.PoolFixture(t)
	owner, pool := service.Projects.Sqlite, service.Projects.Sqlite.ProjectReadDB
	if pool == nil {
		t.Fatal("first scoped read did not initialise the owned read-only project pool")
	}
	if owner.ProjectDB != nil {
		t.Fatal("read-only GetPlot unexpectedly initialised writable WAL project pool")
	}
	vprotest.AssertNoWALSidecars(t, state.ProjectPath)
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	if !bytes.Equal(before["project"], seedBytes) {
		t.Fatal("initial GetPlot mutated Sample.db bytes")
	}
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		header, err := service.GetPlot(context.Background(), state.ContextID, original.PlotNumber)
		if err != nil || !reflect.DeepEqual(header, original) {
			t.Fatalf("warm header changed: %v", err)
		}
		if _, err := service.GetHeaderCapabilities(context.Background(), state.ContextID); err != nil {
			t.Fatal(err)
		}
		if owner.ProjectReadDB != pool || owner.ProjectDB != nil {
			t.Fatal("operation replaced read pool or opened writable pool")
		}
	}
	stats := pool.Stats()
	if stats.MaxOpenConnections != 2 || stats.OpenConnections != 1 || stats.InUse != 0 || stats.Idle != 1 {
		t.Fatalf("serial warm workload did not reuse one idle connection: %+v", stats)
	}
	vprotest.AssertNoWALSidecars(t, state.ProjectPath)
	after := vprotest.DatabaseBytes(t, owner.Attachments)
	if !reflect.DeepEqual(before, after) || !bytes.Equal(after["project"], seedBytes) {
		t.Fatal("readonly plot workload changed project/support bytes")
	}
	current, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(config, current) {
		t.Fatal("warm reads changed configuration")
	}
	// Verify non-coordinator fallback in getActiveReadDB also opens mode=ro without file/WAL mutation.
	legacyDir := t.TempDir()
	legacyProjects, err := vprocore.NewProjectServiceWithConfig(legacyDir, legacyDir)
	if err != nil {
		t.Fatal(err)
	}
	legacyState, err := legacyProjects.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := legacyProjects.ProjectFile(vprocore.CompatibleProject(legacyState.Projects, legacyState.ActiveProject))
	legacyBefore, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	legacyPlots := vprocore.NewPlotService(legacyProjects)
	if _, err := legacyPlots.GetHeaderCapabilities(); err != nil {
		t.Fatal(err)
	}
	if got, err := legacyPlots.GetPlot(original.PlotNumber); err != nil || !reflect.DeepEqual(got, original) {
		t.Fatalf("legacy read-only GetPlot mismatch: %v", err)
	}
	vprotest.AssertNoWALSidecars(t, legacyPath)
	legacyAfter, err := os.ReadFile(legacyPath)
	if err != nil || !bytes.Equal(legacyBefore, legacyAfter) {
		t.Fatal("legacy non-coordinator GetPlot/GetHeaderCapabilities mutated project bytes")
	}
}

func TestPlotPoolEveryConnectionKeepsForeignKeysAndBusyPolicy(t *testing.T) {
	service, _, _ := vprotest.PoolFixture(t)
	pool := service.Projects.Sqlite.ProjectReadDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for _, connection := range []*sql.Conn{first, second} {
		var foreignKeys, timeout int
		if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 || timeout != 5000 {
			t.Fatalf("connection policy lost: foreign_keys=%d busy_timeout=%d", foreignKeys, timeout)
		}
	}
	if stats := pool.Stats(); stats.OpenConnections != 2 || stats.MaxOpenConnections != 2 {
		t.Fatalf("pool connection bound differs: %+v", stats)
	}
}

func TestPlotPoolConcurrentReadsKeepOneOwnerAndBoundedConnections(t *testing.T) {
	service, state, header := vprotest.PoolFixture(t)
	pool := service.Projects.Sqlite.ProjectReadDB
	results := make(chan error, 24)
	var wait sync.WaitGroup
	for i := 0; i < cap(results); i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < 10; i++ {
				if _, err := service.GetPlot(context.Background(), state.ContextID, header.PlotNumber); err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}()
	}
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	vprotest.AssertNoWALSidecars(t, state.ProjectPath)
	if service.Projects.Sqlite.ProjectReadDB != pool || service.Projects.Sqlite.ProjectDB != nil || pool.Stats().OpenConnections > 2 || pool.Stats().InUse != 0 {
		t.Fatal("concurrent reads lost ownership, opened writable pool, or exceeded the pool bound")
	}
}

func TestPlotPoolFailedPublicationRetainsPoolAndSwitchClosesIt(t *testing.T) {
	service, state, header := vprotest.PoolFixture(t)
	owner, pool := service.Projects.Sqlite, service.Projects.Sqlite.ProjectReadDB
	preferences := service.Projects.Preferences
	replace := preferences.Replace
	preferences.Replace = func(string, string) error { return errors.New("pool publication failure") }
	if _, err := switchSQLiteTestSU(t, service.Projects, "Sample"); err == nil || !strings.Contains(err.Error(), "publication failure") {
		t.Fatalf("failed publication accepted: %v", err)
	}
	preferences.Replace = replace
	if service.Projects.Sqlite != owner || owner.ProjectReadDB != pool {
		t.Fatal("failed publication retired the original pool")
	}
	if _, err := service.GetPlot(context.Background(), state.ContextID, header.PlotNumber); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err == nil || owner.ProjectReadDB != nil {
		t.Fatal("successful switch left old project pool open")
	}
	if _, err := service.GetPlot(context.Background(), next.ContextID, header.PlotNumber); err != nil {
		t.Fatal(err)
	}
	newPool := service.Projects.Sqlite.ProjectReadDB
	if newPool == pool || newPool == nil {
		t.Fatal("new context reused a retired project pool")
	}
	if err := service.Projects.CloseSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if err := newPool.Ping(); err == nil {
		t.Fatal("shutdown did not close its project pool")
	}
}

func TestPlotPoolUnscopedReadBorrowBlocksSwitchAndReleasesOnce(t *testing.T) {
	service, state, _ := vprotest.PoolFixture(t)
	db, _, release, err := service.Plots.GetActiveReadDB()
	if err != nil {
		t.Fatal(err)
	}

	if service.Projects.OperationMu.TryLock() {
		service.Projects.OperationMu.Unlock()
		release()
		t.Fatal("unscoped read borrow did not protect pool lifetime")
	}
	switched := make(chan error, 1)
	go func() {
		_, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
		switched <- err
	}()
	release()
	release()
	select {
	case err := <-switched:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("released read borrow blocked switching")
	}
	if err := db.Ping(); err == nil {
		t.Fatal("released old pool remained open after switching")
	}
}

func TestPlotPoolAuditFailureRollsBackAndRetryKeepsTheSamePool(t *testing.T) {
	service, state, original := vprotest.PoolFixture(t)
	pool, err := service.Projects.Sqlite.ProjectDatabase(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var before int
	if err := pool.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(`CREATE TRIGGER PoolAuditFailure BEFORE INSERT ON Sample_Audit
			BEGIN SELECT RAISE(ABORT,'pool audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	changed := *original
	value := "session pool retry"
	changed.PlotRepresenting = &value
	if err := service.UpdatePlot(state.ContextID, changed); err == nil || !strings.Contains(err.Error(), "pool audit failure") {
		t.Fatalf("injected failure did not reject save: %v", err)
	}
	actual, err := service.GetPlot(context.Background(), state.ContextID, original.PlotNumber)
	if err != nil || !reflect.DeepEqual(actual, original) {
		t.Fatalf("failed pooled save changed parent data: %v", err)
	}
	var after int
	if err := pool.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&after); err != nil || before != after {
		t.Fatal("failed pooled save changed history", err)
	}
	if _, err := pool.Exec(`DROP TRIGGER PoolAuditFailure`); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlot(state.ContextID, changed); err != nil {
		t.Fatal(err)
	}
	actual, err = service.GetPlot(context.Background(), state.ContextID, original.PlotNumber)
	if err != nil || actual.PlotRepresenting == nil || *actual.PlotRepresenting != value {
		t.Fatal("pooled retry did not save the intended value", err)
	}
	if err := pool.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&after); err != nil || after != before+1 {
		t.Fatal("pooled retry did not add exactly one audit", err)
	}
	if service.Projects.Sqlite.ProjectDB != pool {
		t.Fatal("failed save/retry replaced its session pool")
	}
}

func BenchmarkPlotPoolReads(b *testing.B) {
	root := b.TempDir()
	data, config := filepath.Join(root, "data"), filepath.Join(root, "config")
	for _, path := range []string{data, config} {
		if err := os.Mkdir(path, 0700); err != nil {
			b.Fatal(err)
		}
	}
	preferences, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		b.Fatal(err)
	}
	projects, err := vprocore.NewSQLiteProjectService(data, config, preferences)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := projects.CloseSQLiteContext(); err != nil {
			b.Error(err)
		}
	})
	legacyProjects, err := vprocore.NewProjectService(data)
	if err != nil {
		b.Fatal(err)
	}
	legacy := vprocore.NewPlotService(legacyProjects)
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		b.Fatal(err)
	}
	scoped, err := vprocore.NewContextService(projects, plots)
	if err != nil {
		b.Fatal(err)
	}
	page, err := projects.ListPlots(context.Background(), 0, 1)
	if err != nil || len(page.Plots) != 1 {
		b.Fatal("benchmark plot is unavailable", err)
	}
	plot := page.Plots[0].PlotNumber
	for name, read := range map[string]func() (*vprocore.FS882Header, error){
		"legacy-per-operation": func() (*vprocore.FS882Header, error) { return legacy.GetPlot(plot) },
		"context-owned-pool": func() (*vprocore.FS882Header, error) {
			return scoped.GetPlot(context.Background(), projects.ContextID, plot)
		},
	} {
		b.Run(name, func(b *testing.B) {
			if _, err := read(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := read(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
