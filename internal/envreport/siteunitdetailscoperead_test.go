package envreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
			service, state := vprotest.ReportServiceFixture(t, external)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			result, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || result.ContextID != state.ContextID || result.ProjectPath != state.ProjectPath ||
				result.SUPath != state.SUPath || result.Scope.Project != state.ActiveProject || result.Scope.SU != state.ActiveSU ||
				result.Scope.QuerySource != siteUnitDetailSelectedSU || len(result.Scope.Units) != 1 ||
				result.Scope.Units[0].Code != "  Unit 'quoted'  " || len(result.Scope.Units[0].Plots) != 1 ||
				result.Scope.Units[0].Plots[0].PlotNumber != "108050" || len(result.Scope.Memberships) != 3 {
				t.Fatal("owned joined scope differs", result, err)
			}
			again, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("repeat read differs", again, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("scope read changed preferences", err)
			}
		})
	}
}

func TestSiteUnitDetailOwnedScopeCancellationCleanupAndRetry(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	sentinel := errors.New("owned scope cleanup failed")
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return sentinel }},
		{RollbackRead: func(*sql.Tx) error { return sentinel }},
	} {
		result, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, hooks)
		if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
			t.Fatal("cleanup error retained success-shaped scope", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, err := ReadSiteUnitDetailNormalSUScope(service, ctx, state.ContextID, 1,
		vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("late cancellation retained completed scope", result, err)
	}
	owner := service.Projects.Sqlite
	owner.Mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	_, err = ReadSiteUnitDetailNormalSUScope(service, ctx, state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("owned lease wait ignored deadline", err)
	}
	result, err = ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || len(result.Scope.Units) != 1 {
		t.Fatal("clean retry failed after cleanup/cancellation", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSiteUnitDetailOwnedScopeStaleNoneAndPhysicalTableGuard(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil {
		t.Fatal("stale context supplied scope")
	}
	if result, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), next.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil ||
		!strings.Contains(err.Error(), "selected normal SU") || !reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("None guessed whole-project scope", result, err)
	}
	service, state = vprotest.ReportServiceFixture(t, false)
	owner := service.Projects.Sqlite
	original := owner.Attachments["project"]
	owner.Attachments["project"] = original + ".unowned"
	if result, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil ||
		!reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("unowned path supplied scope", result, err)
	}
	owner.Attachments["project"] = original
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("DROP TABLE Report_SU; CREATE VIEW Report_SU AS SELECT '108050' AS PlotNumber,'invented' AS SiteUnit")
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	if result, err := ReadSiteUnitDetailNormalSUScope(service, context.Background(), state.ContextID, 1, vprocore.PublicationReadSnapshotHooks{}); err == nil ||
		!reflect.DeepEqual(result, siteUnitDetailOwnedScope{}) {
		t.Fatal("view substituted for physical selected SU", result, err)
	}
}
