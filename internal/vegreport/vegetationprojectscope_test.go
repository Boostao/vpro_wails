package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func projectScopeRequestJSON(t *testing.T, revision, kind string) string {
	t.Helper()
	raw, err := json.Marshal(vegetationScopeRequest{revision, kind})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func projectScopeAuthority(t *testing.T, service *LongVegetationReportScopeService, contextID string) (vprocore.VegetationProjectScopeEntry, error) {
	t.Helper()
	return vprocore.WithContextPlotRequest(context.Background(), service.contexts, contextID, func(plots *vprocore.PlotService) (vprocore.VegetationProjectScopeEntry, error) {
		return vprocore.WithOwnedPreferenceSnapshot(context.Background(), plots, vprocore.PublicationReadSnapshotHooks{},
			func(owner *vprocore.SqliteContext, tx *sql.Tx) (vprocore.VegetationProjectScopeEntry, error) {
				config, err := plots.Projects.Preferences.Snapshot()
				if err != nil {
					return vprocore.VegetationProjectScopeEntry{}, err
				}
				return ApprovedProjectScope(service.contexts, context.Background(), contextID, owner, tx, config)
			})
	})
}

func TestProjectScopeExclusiveAuthorityRetainsIndependentDraftsAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state, _ := projectOwnedSourceFixture(t, external, false)
		selected := NewLongVegetationSelectedUnitSelectionService(contexts, true)
		scope := NewLongVegetationReportScopeService(contexts, true)
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		initial, err := scope.GetLongVegetationReportScope(ctx, state.ContextID)
		if err != nil || initial.SourceKind != vprocore.VegetationSelectedListScope || initial.Revision != "0" ||
			initial.Fields == nil || initial.Build != nil || initial.BuildCurrent {
			t.Fatal("session inferred historical or None project intent", initial, err)
		}
		draft, err := selected.SetLongVegetationSelectedUnitFields(ctx, state.ContextID,
			selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := selected.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision); err != nil {
			t.Fatal(err)
		}
		active, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, "0", vprocore.VegetationProjectFieldsScope))
		if err != nil || active.Revision != "1" || active.SourceKind != vprocore.VegetationProjectFieldsScope || active.Build != nil {
			t.Fatal("explicit project scope install", active, err)
		}
		inactive := contexts.SelectedUnitSelections.Read(state.ContextID)
		if inactive.Revision != 3 || inactive.Build != nil || !reflect.DeepEqual(inactive.Fields, []string{"PlotNumber"}) {
			t.Fatal("scope switch lost selected draft or retained selected approval", inactive)
		}
		if _, err := selectedUnitAuthority(t, selected, state.ContextID); err == nil {
			t.Fatal("selected approval survived competing project intent")
		}
		if _, err := PreviewLongVegetation(contexts, ctx, state.ContextID); err == nil {
			t.Fatal("legacy preview silently selected a list under project intent")
		}
		unitPrefs := NewLongVegetationUnitPreferencesService(contexts, true)
		request := `{"expected":{"unitGroups":1},"proposed":{"unitGroups":1}}`
		if outcome := unitPrefs.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request); outcome.Committed || outcome.Changed || outcome.ErrorMessage == "" {
			t.Fatal("legacy preference consumer used project authority", outcome)
		}
		if _, err := scope.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision); err == nil {
			t.Fatal("empty project field draft fabricated Build")
		}
		active, err = scope.SetLongVegetationProjectFields(ctx, state.ContextID,
			selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
		if err != nil || active.Revision != "2" {
			t.Fatal(active, err)
		}
		active, err = scope.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision)
		if err != nil || active.Revision != "3" || !active.BuildCurrent || active.Build == nil ||
			len(active.Build.ProjectTuples) != 5 || len(active.Build.ApprovalHash) != 64 {
			t.Fatal("project Build lost original physical tuples", active, err)
		}
		approved, err := projectScopeAuthority(t, scope, state.ContextID)
		if err != nil || approved.SourceKind != vprocore.VegetationProjectFieldsScope || approved.Revision != 3 {
			t.Fatal("project authority lacks explicit source/revision", approved, err)
		}
		for _, row := range active.Build.ProjectTuples {
			if row.AdminRowID == nil || row.EnvRowID == "" || row.PlotNumber.Text == nil || row.SiteUnit.Text == nil {
				t.Fatal("original source2 tuple fabricated missing provenance", row)
			}
		}
		*active.Build.ProjectTuples[0].SiteUnit.Text = "mutated"
		active.Fields[0] = "mutated"
		current, err := scope.GetLongVegetationReportScope(ctx, state.ContextID)
		if err != nil || !current.BuildCurrent || current.Fields[0] != "PlotNumber" ||
			*current.Build.ProjectTuples[0].SiteUnit.Text != "108050" {
			t.Fatal("scope DTO aliases internal reviewed state", current, err)
		}
		same, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, current.Revision, vprocore.VegetationProjectFieldsScope))
		if err != nil || !reflect.DeepEqual(same.Build, current.Build) || same.Revision != current.Revision {
			t.Fatal("unchanged explicit scope erased reviewed Build", same, err)
		}
		back, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, current.Revision, vprocore.VegetationSelectedListScope))
		if err != nil || back.Revision != "4" || back.Build != nil || back.Fields[0] != "PlotNumber" {
			t.Fatal("switch back discarded independent project draft or kept approval", back, err)
		}
		if _, err := projectScopeAuthority(t, scope, state.ContextID); err == nil {
			t.Fatal("retired project Build revived under selected intent")
		}
		selectedDraft, err := selected.GetLongVegetationSelectedUnitSelection(ctx, state.ContextID)
		if err != nil || selectedDraft.Build != nil || selectedDraft.Revision != "4" || selectedDraft.Fields[0] != "PlotNumber" {
			t.Fatal("selected draft did not survive scope switch independently", selectedDraft, err)
		}
		if _, err := selected.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, selectedDraft.Revision); err != nil {
			t.Fatal("explicit selected rebuild unavailable after scope recovery", err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("scope intent/Build wrote YAML", err)
		}
	}
}

func TestProjectScopeNoneContextOriginalEnvAndSource2RemainSeparate(t *testing.T) {
	contexts, state, _ := projectOwnedSourceFixture(t, true, true)
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	active, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "0", vprocore.VegetationProjectAllScope))
	if err != nil {
		t.Fatal(err)
	}
	built, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision)
	if err != nil || !built.BuildCurrent || built.SUPath != "" || built.SU != "None" ||
		len(built.Build.ProjectTuples) != 5 || built.SourceKind != vprocore.VegetationProjectAllScope {
		t.Fatal("None context fabricated SU path/source3 tuples", built, err)
	}
	for _, row := range built.Build.ProjectTuples {
		if row.AdminRowID != nil || *row.SiteUnit.Text != "All plots" {
			t.Fatal("Env-only source3 invented Admin witnesses", row)
		}
	}
	if built.Build.ProjectTuples[3].PlotNumber.Storage != "null" {
		t.Fatal("original nullable Env tuple omitted")
	}
	active, err = service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, built.Revision, vprocore.VegetationProjectFieldsScope))
	if err != nil {
		t.Fatal(err)
	}
	active, err = service.SetLongVegetationProjectFields(ctx, state.ContextID,
		selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	joined, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision)
	if err != nil || len(joined.Build.ProjectTuples) != 5 || joined.Build.ProjectTuples[3].AdminRowID == nil ||
		joined.Build.ApprovalHash == built.Build.ApprovalHash {
		t.Fatal("no-SU source2 guessed Env-only scope or fabricated source authority", joined, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	if err := contexts.Projects.Preferences.Update("ReportOptions", map[string]any{"DataQualityFilterEnforceLV": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, joined.Revision); err == nil {
		t.Fatal("no-SU source2 invented QCLV authority")
	}
}

func TestProjectScopeBuildSourceConfigurationDriftAndExactRecovery(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, true)
	contexts := private.contexts
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	active, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "0", vprocore.VegetationProjectFieldsScope))
	if err != nil {
		t.Fatal(err)
	}
	active, err = service.SetLongVegetationProjectFields(ctx, state.ContextID,
		selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
	if err != nil {
		t.Fatal(err)
	}
	built, err := service.BuildLongVegetationProjectScope(ctx, state.ContextID, active.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"project", "su"} {
		db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(contexts.Projects.Sqlite.Attachments[role], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		table, column := "Sample_Admin", "OfficeNotes"
		if role == "su" {
			table, column = "Report_SU", "SiteUnit"
		}
		var original any
		if err := db.QueryRow(`SELECT ` + vprocore.QuoteHeaderIdentifier(column) + ` FROM ` + vprocore.QuoteHeaderIdentifier(table) + ` WHERE rowid=1`).Scan(&original); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE ` + vprocore.QuoteHeaderIdentifier(table) + ` SET ` + vprocore.QuoteHeaderIdentifier(column) + `='changed' WHERE rowid=1`); err != nil {
			t.Fatal(err)
		}
		current, err := service.GetLongVegetationReportScope(ctx, state.ContextID)
		if err != nil || current.BuildCurrent || !reflect.DeepEqual(current.Build, built.Build) {
			t.Fatal("complete unrelated original source drift erased receipt or kept approval", role, current, err)
		}
		if _, err := projectScopeAuthority(t, service, state.ContextID); err == nil {
			t.Fatal("unrelated original source drift accepted", role)
		}
		if _, err := db.Exec(`UPDATE `+vprocore.QuoteHeaderIdentifier(table)+` SET `+vprocore.QuoteHeaderIdentifier(column)+`=? WHERE rowid=1`, original); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := projectScopeAuthority(t, service, state.ContextID); err != nil {
			t.Fatal("exact original source recovery failed", role, err)
		}
	}
	values, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	original := values["ReportOptions"].(map[string]any)["LVReportTitle"]
	if err := contexts.Projects.Preferences.Update("ReportOptions", map[string]any{"LVReportTitle": "changed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := projectScopeAuthority(t, service, state.ContextID); err == nil {
		t.Fatal("complete config drift accepted")
	}
	if err := contexts.Projects.Preferences.Update("ReportOptions", map[string]any{"LVReportTitle": original}); err != nil {
		t.Fatal(err)
	}
	if _, err := projectScopeAuthority(t, service, state.ContextID); err != nil {
		t.Fatal("exact config recovery failed", err)
	}
}

func TestProjectScopeStrictCASAndFailedInstallsAreAtomic(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"revision":"0","sourceKind":"project"}`, `{"revision":"0","sourceKind":null}`,
		`{"revision":"0","sourceKind":"selected-list","sourceKind":"project-all-plots"}`,
		`{"revision":"0","sourceKind":"selected-list","\u0073ourceKind":"project-all-plots"}`,
		`{"revision":"0","sourceKind":"selected-list","extra":true}`,
		`{"revision":"0","sourceKind":"\ud800"}`,
	} {
		var request vegetationScopeRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("strict scope wire accepted invalid/ambiguous request", raw)
		}
	}
	private, state := vegetationWorkbookServiceFixture(t, false)
	service := NewLongVegetationReportScopeService(private.contexts, true)
	ctx := context.Background()
	for _, revision := range []string{"", "00", "-1", "1", "9223372036854775808"} {
		if _, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
			projectScopeRequestJSON(t, revision, vprocore.VegetationProjectFieldsScope)); err == nil {
			t.Fatal("malformed or stale revision accepted", revision)
		}
	}
	if _, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "0", vprocore.VegetationProjectAllScope)); err == nil {
		t.Fatal("source3 without original None context accepted")
	}
	service.enabled = false
	if _, err := service.GetLongVegetationReportScope(ctx, state.ContextID); err == nil {
		t.Fatal("disabled service inferred intent")
	}
	service.enabled = true
	owner := private.contexts.Projects.Sqlite
	owner.Mu.Lock()
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	_, err := service.SetLongVegetationReportScope(short, state.ContextID,
		projectScopeRequestJSON(t, "0", vprocore.VegetationProjectFieldsScope))
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || private.contexts.ProjectReportScopes.Read(state.ContextID).Revision != 0 {
		t.Fatal("cancelled waiting scope switch changed authority", err)
	}
	ledger := &private.contexts.ProjectReportScopes
	ledger.Mu.Lock()
	ledger.Entries = map[string]vprocore.VegetationProjectScopeEntry{state.ContextID: {Revision: math.MaxInt64, SourceKind: vprocore.VegetationSelectedListScope}}
	ledger.Mu.Unlock()
	if _, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, "9223372036854775807", vprocore.VegetationProjectFieldsScope)); err == nil {
		t.Fatal("scope revision overflow accepted")
	}
	if private.contexts.SelectedUnitSelections.Read(state.ContextID).Revision != 0 {
		t.Fatal("failed exhausted switch partially invalidated selected draft")
	}
}

func TestProjectScopeStaleBuildAndSelectedBuildCannotInstallAcrossScopeSwitch(t *testing.T) {
	for _, selectedBuild := range []bool{false, true} {
		private, state := vegetationWorkbookServiceFixture(t, true)
		scope := NewLongVegetationReportScopeService(private.contexts, true)
		selected := NewLongVegetationSelectedUnitSelectionService(private.contexts, true)
		ctx := context.Background()
		var buildErr error
		if selectedBuild {
			draft, err := selected.SetLongVegetationSelectedUnitFields(ctx, state.ContextID,
				selectedUnitDraftJSON(t, "0", []string{"PlotNumber"}))
			if err != nil {
				t.Fatal(err)
			}

			selected.beforeInstall = func() {
				if _, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
					projectScopeRequestJSON(t, "0", vprocore.VegetationProjectFieldsScope)); err != nil {
					t.Fatal(err)
				}
			}
			_, buildErr = selected.BuildLongVegetationSelectedUnitSelection(ctx, state.ContextID, draft.Revision)
		} else {
			active, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
				projectScopeRequestJSON(t, "0", vprocore.VegetationProjectFieldsScope))
			if err != nil {
				t.Fatal(err)
			}
			draft, err := scope.SetLongVegetationProjectFields(ctx, state.ContextID,
				selectedUnitDraftJSON(t, active.Revision, []string{"PlotNumber"}))
			if err != nil {
				t.Fatal(err)
			}
			scope.beforeInstall = func() {
				if _, err := scope.SetLongVegetationReportScope(ctx, state.ContextID,
					projectScopeRequestJSON(t, draft.Revision, vprocore.VegetationSelectedListScope)); err != nil {
					t.Fatal(err)
				}
			}
			_, buildErr = scope.BuildLongVegetationProjectScope(ctx, state.ContextID, draft.Revision)
		}
		if buildErr == nil || private.contexts.SelectedUnitSelections.Read(state.ContextID).Build != nil ||
			private.contexts.ProjectReportScopes.Read(state.ContextID).Build != nil {
			t.Fatal("stale opposite-scope Build installed competing approval", selectedBuild, buildErr)
		}
	}
}
