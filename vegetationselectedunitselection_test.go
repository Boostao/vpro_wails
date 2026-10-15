package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func selectedUnitDraftJSON(t *testing.T, revision string, fields []string) string {
	t.Helper()
	raw, err := json.Marshal(selectedUnitSelectionDraft{Revision: revision, Fields: fields})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func selectedUnitAuthority(t *testing.T, service *LongVegetationSelectedUnitSelectionService, contextID string) ([]string, error) {
	t.Helper()
	return withContextPlotRequest(context.Background(), service.contexts, contextID, func(plots *PlotService) ([]string, error) {
		return withOwnedPreferenceSnapshot(context.Background(), plots, publicationReadSnapshotHooks{},
			func(owner *sqliteContext, tx *sql.Tx) ([]string, error) {
				config, err := plots.projects.preferences.snapshot()
				if err != nil {
					return nil, err
				}
				return service.contexts.approvedSelectedUnitFields(context.Background(), contextID, owner, tx, config)
			})
	})
}

func TestSelectedUnitSessionDraftBuildIsReadOnlyDetachedAndContextOwned(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			private, state := vegetationWorkbookServiceFixture(t, external)
			service := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
			before := databaseBytes(t, private.contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(private.contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			initial, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
			if err != nil || initial.Revision != "0" || initial.Fields == nil || len(initial.Fields) != 0 ||
				initial.Build != nil || initial.BuildCurrent || len(initial.Definitions) == 0 {
				t.Fatal("session must not infer exported defaults or historical field selection", initial, err)
			}
			if _, err := selectedUnitAuthority(t, service, state.ContextID); err == nil {
				t.Fatal("empty session invented selection authority")
			}
			draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
			if err != nil || draft.Revision != "1" || draft.Build != nil {
				t.Fatal("explicit field draft", draft, err)
			}
			built, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, "1")
			if err != nil || built.Revision != "2" || !built.BuildCurrent || built.Build == nil ||
				len(built.Build.ApprovalHash) != 64 || len(built.Build.Occurrences) != 3 {
				t.Fatal("Build lost original nullable witnesses or revision", built, err)
			}
			if _, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, "1"); err == nil {
				t.Fatal("completed Build permitted late same-revision overwrite")
			}
			fields, err := selectedUnitAuthority(t, service, state.ContextID)
			if err != nil || !reflect.DeepEqual(fields, []string{"PlotNumber"}) {
				t.Fatal("owned complete source authority", fields, err)
			}
			fields[0] = "changed"
			built.Fields[0] = "changed"
			*built.Build.Occurrences[0].SiteUnit.Text = "changed"
			current, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
			if err != nil || !current.BuildCurrent || current.Fields[0] != "PlotNumber" ||
				*current.Build.Occurrences[0].SiteUnit.Text != "108050" {
				t.Fatal("transport aliases immutable session Build", current, err)
			}
			for _, revision := range []string{"0", "01", "-1", "", "9223372036854775808"} {
				if _, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, revision, []string{})); err == nil {
					t.Fatal("malformed/stale draft revision accepted", revision)
				}
			}
			if _, err := service.GetLongVegetationSelectedUnitSelection(ctx, "stale"); err == nil {
				t.Fatal("stale context read accepted")
			}
			other, err := NewContextService(private.contexts.projects, private.contexts.plots)
			if err != nil {
				t.Fatal(err)
			}
			fresh := NewLongVegetationSelectedUnitSelectionService(other, true)
			restarted, err := fresh.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
			if err != nil || restarted.Build != nil || len(restarted.Fields) != 0 {
				t.Fatal("new session inherited field authority", restarted, err)
			}
			invalid, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "2", []string{"absent"}))
			if err != nil || invalid.Revision != "3" || invalid.Build != nil || invalid.BuildCurrent {
				t.Fatal("draft replacement did not invalidate Build", invalid, err)
			}
			if _, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, "3"); err == nil {
				t.Fatal("absent physical field built successfully")
			}
			if _, err := selectedUnitAuthority(t, service, state.ContextID); err == nil {
				t.Fatal("invalid draft retained prior report authority")
			}
			assertProfileSUFiles(t, private.contexts, before)
			after, err := os.ReadFile(private.contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("session draft/Build changed LVUnitGroups or YAML", err)
			}
		})
	}
}

func TestSelectedUnitSessionFullPhysicalDriftAndExplicitRecovery(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	service := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`ALTER TABLE Sample_Admin ADD COLUMN Unselected TEXT; UPDATE Sample_Admin SET Unselected='before';
		INSERT INTO Sample_Admin(rowid,Plot,Unselected) VALUES(999,'unjoined','before')`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	built, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET Unselected='after'`); err != nil {
		t.Fatal(err)
	}
	drift, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
	if err != nil || drift.BuildCurrent || drift.Build.ApprovalHash != built.Build.ApprovalHash {
		t.Fatal("unselected physical peer fields were omitted from source witness or receipt erased", drift, err)
	}
	if _, err := selectedUnitAuthority(t, service, state.ContextID); err == nil {
		t.Fatal("complete physical peer drift did not refuse authority")
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET Unselected='before'`); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
	if err != nil || !recovered.BuildCurrent || recovered.Build.ApprovalHash != built.Build.ApprovalHash {
		t.Fatal("exact physical recovery failed", recovered, err)
	}
	if _, err := selectedUnitAuthority(t, service, state.ContextID); err != nil {
		t.Fatal("exact reviewed-source retry refused", err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Admin WHERE rowid=999;
		INSERT INTO Sample_Admin(rowid,Plot,Unselected) VALUES(999,'unjoined','replacement')`); err != nil {
		t.Fatal(err)
	}
	if _, err := selectedUnitAuthority(t, service, state.ContextID); err == nil {
		t.Fatal("same-rowid same-plot physical replacement bypassed full source witness")
	}
	if _, err := db.Exec(`DELETE FROM Sample_Admin WHERE rowid=999;
		INSERT INTO Sample_Admin(rowid,Plot,Unselected) VALUES(999,'unjoined','before')`); err != nil {
		t.Fatal(err)
	}
	if _, err := selectedUnitAuthority(t, service, state.ContextID); err != nil {
		t.Fatal("exact physical replacement recovery refused", err)
	}
}

func TestSelectedUnitSessionBuildRejectsConcurrentDraftInvalidation(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	service := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
	ctx := context.Background()
	draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	service.snapshot.commitRead = func(*sql.Tx) error {
		ledger := &private.contexts.selectedUnitSelections
		ledger.mu.Lock()
		ledger.entries[state.ContextID] = selectedUnitSelectionEntry{revision: 2, fields: []string{}}
		ledger.mu.Unlock()
		return nil
	}
	if result, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision); result != nil ||
		err == nil || !strings.Contains(err.Error(), "during Build") {
		t.Fatal("late source Build replaced a newer explicit draft", result, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	current, err := service.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
	if err != nil || current.Revision != "2" || len(current.Fields) != 0 || current.Build != nil {
		t.Fatal("invalidated draft/approval was overwritten", current, err)
	}
}

func TestSelectedUnitSessionApprovalBindsFieldOrderCompleteContextAndConfiguration(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	_, err := withContextPlotRequest(context.Background(), private.contexts, state.ContextID, func(plots *PlotService) (struct{}, error) {
		return withOwnedPreferenceSnapshot(context.Background(), plots, publicationReadSnapshotHooks{},
			func(owner *sqliteContext, tx *sql.Tx) (struct{}, error) {
				env, admin, su := selectedUnitFixture()
				su.Rows = su.Rows[:6]
				tables := [3]ProjectMetadataTable{env, admin, su}
				config := longVegetationOptionsFixture(t)
				changed := longVegetationOptionsFixture(t)
				changed["ReportOptions"].(map[string]any)["LVReportTitle"] = "changed"
				first, err := buildSelectedUnitSelection(context.Background(), state.ContextID, owner, tables, []string{"Zone", "SubZone"}, config)
				if err != nil {
					return struct{}{}, err
				}
				for _, change := range []struct {
					context string
					fields  []string
					config  configValues
				}{
					{state.ContextID, []string{"SubZone", "Zone"}, config},
					{"other-context", []string{"Zone", "SubZone"}, config},
					{state.ContextID, []string{"Zone", "SubZone"}, changed},
				} {
					next, err := buildSelectedUnitSelection(context.Background(), change.context, owner, tables, change.fields, change.config)
					if err != nil {
						return struct{}{}, err
					}
					if next.ApprovalHash == first.ApprovalHash {
						t.Fatal("Build approval omitted ordered selection/context/configuration", change)
					}
				}
				return struct{}{}, nil
			})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSelectedUnitSessionRejectsStrictRequestsAndRetainsDraftOnReadCleanupFailure(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	service := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
	ctx := context.Background()
	for _, raw := range []string{
		`{}`, `{"revision":"0","fields":null}`, `{"revision":"0","fields":["PlotNumber"],"extra":1}`,
		`{"revision":"0","revision":"0","fields":["PlotNumber"]}`,
		`{"revision":"0","fields":[],"\u0066ields":["PlotNumber"]}`,
		`{"revision":"0","fields":["\ud800"]}`, `{"revision":0,"fields":[]}`,
	} {
		if _, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, raw); err == nil {
			t.Fatal("strict field request accepted malformed wire", raw)
		}

	}
	disabled := NewLongVegetationSelectedUnitSelectionService(private.contexts, false)
	if _, err := disabled.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID); err == nil {
		t.Fatal("selection gate ignored")
	}
	draft, err := service.SetLongVegetationSelectedUnitFields(ctx, state.ContextID, selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	service.snapshot.commitRead = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected completed snapshot cleanup failure")
	}
	if result, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision); err == nil || result != nil {
		t.Fatal("failed snapshot installed successful Build", result, err)
	}
	entry := private.contexts.selectedUnitSelections.read(state.ContextID)
	if entry.build != nil || strconv.FormatInt(entry.revision, 10) != draft.Revision {
		t.Fatal("cleanup failure changed draft or approval", entry)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	cancelled, cancel := context.WithCancel(ctx)
	service.snapshot.commitRead = func(tx *sql.Tx) error {
		cancel()
		return tx.Commit()
	}
	if result, err := service.BuildLongVegetationSelectedUnitSelection(cancelled, state.ContextID, draft.Revision); err == nil || result != nil {
		t.Fatal("late cancelled Build installed approval", result, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if result, err := service.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision); err != nil || !result.BuildCurrent {
		t.Fatal("failed/cancelled Build prevented explicit retry", result, err)
	}
	if _, err := service.GetLongVegetationSelectedUnitSelection(nil, state.ContextID); err == nil ||
		!strings.Contains(err.Error(), "context") {
		t.Fatal("nil context accepted", err)
	}
}
