package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExternalSUEnvironmentActualLateCancellationAndCleanupReceipt(t *testing.T) {
	for _, committedCleanup := range []bool{false, true} {
		t.Run(map[bool]string{false: "late-cancel", true: "committed-cleanup"}[committedCleanup], func(t *testing.T) {
			service, state, approved := externalSUEnvironmentFixture(t)
			owner := service.projects.sqlite
			before := databaseBytes(t, owner.attachments)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cleanupFailure := errors.New("independent cleanup failure")
			hooks := externalSUEnvironmentWriteHooks{}
			if committedCleanup {
				hooks.afterCleanup = func() error { return cleanupFailure }
			} else {
				hooks.beforeCommit = func() error {
					cancel()
					return nil
				}
			}
			result, err := service.transferExternalSUEnvironmentWithHooks(ctx, state.ContextID, approved, hooks)
			if !committedCleanup {
				if result != nil || !errors.Is(err, context.Canceled) {
					t.Fatal("late cancellation lost rollback", result, err)
				}
				assertProfileSUFiles(t, service, before)
			} else {
				if result == nil || result.ChangedCells != 4 || result.HistoryID != "1" || !errors.Is(err, cleanupFailure) {
					t.Fatal("cleanup error erased committed receipt", result, err)
				}
				var records int
				if err := owner.conn.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM project.__VPRO_SUEnvironmentHistory`).Scan(&records); err != nil || records != 1 {
					t.Fatal("committed cleanup error lost physical history", records, err)
				}
				after := databaseBytes(t, owner.attachments)
				if replay, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved); replay != nil || err == nil {
					t.Fatal("committed cleanup error permitted replay", replay, err)
				}
				assertProfileSUFiles(t, service, after)
			}
			fresh, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
			if err != nil || fresh == nil {
				t.Fatal("cleanup/cancellation damaged owned reader", fresh, err)
			}
		})
	}
}

func TestExternalSUEnvironmentOpenedIdentityReplacementAndAliases(t *testing.T) {
	for _, role := range []string{"project", "su", "VLists", "VUser"} {
		t.Run(role, func(t *testing.T) {
			paths := make(map[string]string)
			infos := make(map[string]os.FileInfo)
			for _, name := range []string{"project", "su", "VLists", "VUser"} {
				paths[name] = filepath.Join(t.TempDir(), name+".db")
				if err := os.WriteFile(paths[name], []byte(name), 0600); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(paths[name])
				if err != nil {
					t.Fatal(err)
				}
				infos[name] = info
			}
			owner := &sqliteContext{attachments: paths, attachmentInfo: infos, selection: desktopSelection{SU: "External"}}
			expected, err := externalSUEnvironmentFiles(owner)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(paths[role], paths[role]+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths[role], []byte(role), 0600); err != nil {
				t.Fatal(err)
			}
			if observed, err := externalSUEnvironmentFiles(owner); err == nil || observed != nil {
				t.Fatal("byte-identical replaced physical owner accepted", observed, err)
			}
			observed := make(map[string]os.FileInfo)
			for name, info := range expected {
				observed[name] = info
			}
			info, err := os.Stat(paths[role])
			if err != nil {
				t.Fatal(err)
			}
			observed[role] = info
			if err := compareExternalSUEnvironmentFiles(expected, observed); err == nil {
				t.Fatal("reviewed opened-handle identity did not freeze replacement")
			}
		})
	}
}

func TestExternalSUEnvironmentRefusesSameFileAndSupportTarget(t *testing.T) {
	service, state, _ := siteUnitEnvironmentFixture(t)
	if result, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID); err == nil || result != nil {
		t.Fatal("external successor changed same-file authority", result, err)
	}
	owner := service.projects.sqlite
	paths := make(map[string]string)
	infos := make(map[string]os.FileInfo)
	for role, path := range owner.attachments {
		paths[role], infos[role] = path, owner.attachmentInfo[role]
	}
	paths["project"], infos["project"] = paths["VLists"], infos["VLists"]
	aliased := &sqliteContext{attachments: paths, attachmentInfo: infos, selection: owner.selection}
	if result, err := externalSUEnvironmentFiles(aliased); err == nil || result != nil {
		t.Fatal("support file granted project mutation authority", result, err)
	}
}

func TestExternalSUEnvironmentWholeRowsDescriptionsAndProvenance(t *testing.T) {
	service, state, approved := externalSUEnvironmentFixture(t)
	owner := service.projects.sqlite
	db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["su"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// The selected code is unchanged: a nonselected peer still belongs to the witness.
	if _, err := db.Exec(`UPDATE External_SU SET PlotNumber='Different foreign peer' WHERE PlotNumber='Foreign'`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, owner.attachments)
	if result, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, approved); err == nil || result != nil {
		t.Fatal("complete source peer drift was ignored", result, err)
	}
	assertProfileSUFiles(t, service, before)
	fresh, err := service.reviewExternalSUEnvironment(t.Context(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.transferExternalSUEnvironment(t.Context(), state.ContextID, fresh); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := owner.conn.QueryRowContext(t.Context(), `SELECT Proposal FROM project.__VPRO_SUEnvironmentHistory WHERE ID=1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Request   SiteUnitEnvironmentTransfer
		Selection desktopSelection
		Witness   map[string]map[string]ProjectMetadataTable
		User      string
	}
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || !reflect.DeepEqual(saved.Request.Review, fresh.Review) ||
		!reflect.DeepEqual(saved.Witness, fresh.Witness) || saved.Selection != fresh.Selection || saved.User != fresh.User {
		t.Fatal("external complete typed plan/provenance was not retained", err)
	}
}
