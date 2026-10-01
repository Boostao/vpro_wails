package main

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

func poolFixture(t *testing.T) (*ContextService, ProjectState, *FS882Header) {
	t.Helper()
	service, state := contextServiceFixture(t)
	page, err := service.projects.ListPlots(0, 1)
	if err != nil || len(page.Plots) != 1 {
		t.Fatalf("plot fixture: %+v %v", page, err)
	}
	header, err := service.GetPlot(context.Background(), state.ContextID, page.Plots[0].PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, header
}

func TestPlotPoolWarmReadsReuseOwnedHandleAndPreserveFiles(t *testing.T) {
	service, state, original := poolFixture(t)
	owner, pool := service.projects.sqlite, service.projects.sqlite.projectDB
	if pool == nil {
		t.Fatal("first scoped read did not initialise the owned project pool")
	}
	before := databaseBytes(t, owner.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
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
		if owner.projectDB != pool {
			t.Fatal("operation replaced or closed the session pool")
		}
	}
	stats := pool.Stats()
	if stats.MaxOpenConnections != 2 || stats.OpenConnections != 1 || stats.InUse != 0 || stats.Idle != 1 {
		t.Fatalf("serial warm workload did not reuse one idle connection: %+v", stats)
	}
	after := databaseBytes(t, owner.attachments)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("readonly plot workload changed project/support bytes")
	}
	current, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, current) {
		t.Fatal("warm reads changed configuration")
	}
}

func TestPlotPoolEveryConnectionKeepsForeignKeysAndBusyPolicy(t *testing.T) {
	service, _, _ := poolFixture(t)
	pool := service.projects.sqlite.projectDB
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
	service, state, header := poolFixture(t)
	pool := service.projects.sqlite.projectDB
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
	if service.projects.sqlite.projectDB != pool || pool.Stats().OpenConnections > 2 || pool.Stats().InUse != 0 {
		t.Fatal("concurrent reads lost ownership or exceeded the pool bound")
	}
}

func TestPlotPoolFailedPublicationRetainsPoolAndSwitchClosesIt(t *testing.T) {
	service, state, header := poolFixture(t)
	owner, pool := service.projects.sqlite, service.projects.sqlite.projectDB
	preferences := service.projects.preferences
	replace := preferences.replace
	preferences.replace = func(string, string) error { return errors.New("pool publication failure") }
	if _, err := switchSQLiteTestSU(t, service.projects, "Sample"); err == nil || !strings.Contains(err.Error(), "publication failure") {
		t.Fatalf("failed publication accepted: %v", err)
	}
	preferences.replace = replace
	if service.projects.sqlite != owner || owner.projectDB != pool {
		t.Fatal("failed publication retired the original pool")
	}
	if _, err := service.GetPlot(context.Background(), state.ContextID, header.PlotNumber); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err == nil || owner.projectDB != nil {
		t.Fatal("successful switch left old project pool open")
	}
	if _, err := service.GetPlot(context.Background(), next.ContextID, header.PlotNumber); err != nil {
		t.Fatal(err)
	}
	newPool := service.projects.sqlite.projectDB
	if newPool == pool || newPool == nil {
		t.Fatal("new context reused a retired project pool")
	}
	if err := service.projects.closeSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if err := newPool.Ping(); err == nil {
		t.Fatal("shutdown did not close its project pool")
	}
}

func TestPlotPoolUnscopedReadBorrowBlocksSwitchAndReleasesOnce(t *testing.T) {
	service, state, _ := poolFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}

	if service.projects.operationMu.TryLock() {
		service.projects.operationMu.Unlock()
		release()
		t.Fatal("unscoped read borrow did not protect pool lifetime")
	}
	switched := make(chan error, 1)
	go func() {
		_, err := service.SwitchContext(state.ContextID, contextSelection(state))
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
	service, state, original := poolFixture(t)
	pool := service.projects.sqlite.projectDB
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
	if service.projects.sqlite.projectDB != pool {
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
	preferences, err := openDesktopConfig(data, config)
	if err != nil {
		b.Fatal(err)
	}
	projects, err := newSQLiteProjectService(data, config, preferences)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := projects.closeSQLiteContext(); err != nil {
			b.Error(err)
		}
	})
	legacyProjects, err := NewProjectService(data)
	if err != nil {
		b.Fatal(err)
	}
	legacy := NewPlotService(legacyProjects)
	plots, err := newPlotServiceWithPreferences(projects)
	if err != nil {
		b.Fatal(err)
	}
	scoped, err := NewContextService(projects, plots)
	if err != nil {
		b.Fatal(err)
	}
	page, err := projects.ListPlots(0, 1)
	if err != nil || len(page.Plots) != 1 {
		b.Fatal("benchmark plot is unavailable", err)
	}
	plot := page.Plots[0].PlotNumber
	for name, read := range map[string]func() (*FS882Header, error){
		"legacy-per-operation": func() (*FS882Header, error) { return legacy.GetPlot(plot) },
		"context-owned-pool":   func() (*FS882Header, error) { return scoped.GetPlot(context.Background(), projects.contextID, plot) },
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
