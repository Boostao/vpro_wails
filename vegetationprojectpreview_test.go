package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func projectReadJSON(t *testing.T, scope *LongVegetationReportScope) string {
	t.Helper()
	raw, err := json.Marshal(vegetationProjectReadRequest{scope.Revision, scope.Build.ApprovalHash})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func projectPreviewFixture(t *testing.T, external, none, all bool) (*LongVegetationReportScopeService, ProjectState, *LongVegetationReportScope) {
	t.Helper()
	contexts, state, _ := projectOwnedSourceFixture(t, external, none)
	service := NewLongVegetationReportScopeService(contexts, true)
	ctx := context.Background()
	kind, group := vegetationProjectFieldsScope, 2
	if all {
		kind, group = vegetationProjectAllScope, 3
	}
	scope, err := service.SetLongVegetationReportScope(ctx, state.ContextID, projectScopeRequestJSON(t, "0", kind))
	if err != nil {
		t.Fatal(err)
	}
	if !all {
		scope, err = service.SetLongVegetationProjectFields(ctx, state.ContextID, selectedUnitDraftJSON(t, scope.Revision, []string{"PlotNumber"}))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": group}); err != nil {
		t.Fatal(err)
	}
	scope, err = service.BuildLongVegetationProjectScope(ctx, state.ContextID, scope.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, scope
}

func TestProjectOwnedPreviewRequiresExactApprovedIntentAndSavedMode(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, mode := range []struct{ none, all bool }{{false, false}, {true, false}, {true, true}} {
			service, state, scope := projectPreviewFixture(t, external, mode.none, mode.all)
			contexts := service.contexts
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			request := projectReadJSON(t, scope)
			options, err := service.GetLongVegetationProjectOptions(context.Background(), state.ContextID, request)
			if err != nil || !options.Scope.BuildCurrent || !reflect.DeepEqual(options.Scope, *scope) ||
				options.Options.SU != state.ActiveSU || options.Options.SUPath != state.SUPath {
				t.Fatal("project options lost exact reviewed intent/path ownership", options, err)
			}
			preview, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, request)
			if err != nil || !reflect.DeepEqual(preview.Scope, *scope) || preview.Preview.Report.SourceKind != scope.SourceKind ||
				!reflect.DeepEqual(preview.Preview.Settings, options.Options.Settings) ||
				len(preview.Preview.Report.ProjectUnits) != 5 || len(preview.Preview.Report.Units) == 0 {
				t.Fatal("owned project preview lost atomic source/scope/typed calculation", preview, err)
			}
			if mode.all {
				if preview.Preview.Settings.UnitGrouping != "all-plots" || preview.Preview.Report.Units[0].NumPlots != 5 ||
					preview.Preview.Report.ProjectUnits[3].PlotNumber.Storage != "null" {
					t.Fatal("Env-only original NULL denominator lost", preview)
				}
			} else if preview.Preview.Settings.UnitGrouping != "selected-fields" ||
				!reflect.DeepEqual(preview.Preview.Settings.UnitFields, []string{"PlotNumber"}) {
				t.Fatal("project field ordering omitted from settings", preview)
			}
			for _, unit := range preview.Preview.Report.Units {
				if len(unit.MembershipIDs) != 0 || len(unit.ProjectUnitIndexes) != unit.NumPlots {
					t.Fatal("project preview invented physical SU rows", unit)
				}
			}
			again, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, request)
			if err != nil || !reflect.DeepEqual(again, preview) {
				t.Fatal("repeated unchanged owned project preview differs", err)
			}
			assertProfileSUFiles(t, contexts, before)
			after, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("readonly project preview changed configuration", err)
			}
		}
	}
}

func TestProjectOwnedPreviewRefusesAmbiguousRequestAndLostBuildAuthority(t *testing.T) {
	service, state, scope := projectPreviewFixture(t, true, false, false)
	ctx := context.Background()
	for _, raw := range []string{
		`{}`, `{"revision":"01","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"0","approvalHash":"` + strings.ToUpper(scope.Build.ApprovalHash) + `"}`,
		`{"revision":"0","approvalHash":"bad"}`, `{"revision":"0","approvalHash":null}`,
		`{"revision":"0","revision":"1","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"0","\u0072evision":"1","approvalHash":"` + scope.Build.ApprovalHash + `"}`,
		`{"revision":"0","approvalHash":"` + scope.Build.ApprovalHash + `","extra":true}`,
	} {
		var request vegetationProjectReadRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("invalid or ambiguous project read request accepted", raw)
		}
	}
	request := projectReadJSON(t, scope)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if output, err := service.PreviewLongVegetationProject(cancelled, state.ContextID, request); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(output, LongVegetationProjectPreview{}) {
		t.Fatal("cancelled owned project preview returned output", output, err)
	}
	if _, err := service.PreviewLongVegetationProject(ctx, state.ContextID, request); err != nil {
		t.Fatal("cancellation lost pinned source/lease", err)
	}
	wrong := *scope
	wrong.Revision = "0"
	if output, err := service.PreviewLongVegetationProject(ctx, state.ContextID, projectReadJSON(t, &wrong)); err == nil || !reflect.DeepEqual(output, LongVegetationProjectPreview{}) {
		t.Fatal("old scope revision authorized current project output", output, err)
	}
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVReportTitle": "drift"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewLongVegetationProject(ctx, state.ContextID, request); err == nil {
		t.Fatal("config drift ignored exact project Build")
	}
}

func TestProjectOwnedPreviewNeverTreatsBuildNormalizationAsSavedPreference(t *testing.T) {
	service, state, scope := projectPreviewFixture(t, false, true, false)
	if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": 1}); err != nil {
		t.Fatal(err)
	}
	request := projectReadJSON(t, scope)
	for _, run := range []func() error{
		func() error {
			_, err := service.GetLongVegetationProjectOptions(context.Background(), state.ContextID, request)
			return err
		},
		func() error {
			_, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, request)
			return err
		},
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "saved report unit choice") {
			t.Fatal("planned Build unit choice became an unsaved effective report preference", err)
		}
	}
}

func TestProjectOwnedPreviewReadCleanupFailureCannotReturnSuccessfulOutput(t *testing.T) {
	service, state, scope := projectPreviewFixture(t, false, true, true)
	before := service.contexts.projectReportScopes.read(state.ContextID)
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("project preview snapshot cleanup failure") }
	result, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, projectReadJSON(t, scope))
	if err == nil || !reflect.DeepEqual(result, LongVegetationProjectPreview{}) ||
		!reflect.DeepEqual(before, service.contexts.projectReportScopes.read(state.ContextID)) {
		t.Fatal("failed completed snapshot published project output or changed authority", result, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if _, err := service.PreviewLongVegetationProject(context.Background(), state.ContextID, projectReadJSON(t, scope)); err != nil {
		t.Fatal("failed cleanup lost attached original project reads", err)
	}
}
