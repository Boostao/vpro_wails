package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSiteUnitDetailOwnedScopeProjectExternalAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			before := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
			if err != nil || result.ContextID != state.ContextID || result.ProjectPath != state.ProjectPath ||
				result.SUPath != state.SUPath || result.Scope.Project != state.ActiveProject || result.Scope.SU != state.ActiveSU ||
				result.Scope.QuerySource != siteUnitDetailSelectedSU || len(result.Scope.Units) != 1 ||
				result.Scope.Units[0].Code != "  Unit 'quoted'  " || len(result.Scope.Units[0].Plots) != 1 ||
				result.Scope.Units[0].Plots[0].PlotNumber != "108050" || len(result.Scope.Memberships) != 3 {
				t.Fatal("owned joined scope differs", result, err)
			}
			again, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("repeat read differs", again, err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("scope read changed preferences", err)
			}
		})
	}
}

func TestSiteUnitDetailOwnedScopeCancellationCleanupAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	sentinel := errors.New("owned scope cleanup failed")
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return sentinel }},
		{rollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		result, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
			t.Fatal("cleanup error retained success-shaped scope", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, err := service.readSiteUnitDetailNormalSUScope(ctx, state.ContextID, 1,
		publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("late cancellation retained completed scope", result, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err = service.readSiteUnitDetailNormalSUScope(ctx, state.ContextID, 1, publicationReadSnapshotHooks{})
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("owned lease wait ignored deadline", err)
	}
	result, err = service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	if err != nil || len(result.Scope.Units) != 1 {
		t.Fatal("clean retry failed after cleanup/cancellation", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSiteUnitDetailOwnedScopeStaleNoneAndPhysicalTableGuard(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil {
		t.Fatal("stale context supplied scope")
	}
	if result, err := service.readSiteUnitDetailNormalSUScope(context.Background(), next.ContextID, 1, publicationReadSnapshotHooks{}); err == nil ||
		!strings.Contains(err.Error(), "selected normal SU") || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("None guessed whole-project scope", result, err)
	}
	service, state = reportServiceFixture(t, false)
	owner := service.projects.sqlite
	original := owner.attachments["project"]
	owner.attachments["project"] = original + ".unowned"
	if result, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil ||
		!reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("unowned path supplied scope", result, err)
	}
	owner.attachments["project"] = original
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("DROP TABLE Report_SU; CREATE VIEW Report_SU AS SELECT '108050' AS PlotNumber,'invented' AS SiteUnit")
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.readSiteUnitDetailNormalSUScope(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{}); err == nil ||
		!reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("view substituted for physical selected SU", result, err)
	}
}
