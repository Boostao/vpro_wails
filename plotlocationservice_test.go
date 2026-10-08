package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlotLocationReviewFeatureIndependentLiteralOptIn(t *testing.T) {
	for _, test := range []struct {
		name, value               string
		present, enabled, invalid bool
	}{
		{"unset", "", false, false, false},
		{"true", "true", true, true, false},
		{"false", "false", true, false, false},
		{"empty", "", true, false, true},
		{"uppercase", "TRUE", true, false, true},
		{"number", "1", true, false, true},
		{"padded", " true ", true, false, true},
		{"newline", "true\n", true, false, true},
		{"negative", "False", true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			enabled, err := plotLocationReviewFeature(func(name string) (string, bool) {
				calls++
				if name != "VPRO_PLOT_LOCATION_REVIEW" {
					t.Fatal("unrelated feature lookup:", name)
				}
				return test.value, test.present
			})
			if calls != 1 || enabled != test.enabled || (err != nil) != test.invalid {
				t.Fatal("literal opt-in changed:", test, enabled, err, calls)
			}
			if err != nil && !strings.Contains(err.Error(), "VPRO_PLOT_LOCATION_REVIEW") {
				t.Fatal("invalid setting lost feature identity:", err)
			}
		})
	}
	enabled, err := plotLocationReviewFeature(func(name string) (string, bool) {
		if name == "VPRO_PLOT_LOCATION_REVIEW" {
			return "", false
		}
		return "true", true
	})
	if enabled || err != nil {
		t.Fatal("unrelated flags enabled location review:", err)
	}
}

func assertPlotLocationFacadeReadOnly(t *testing.T, service *ContextService) func() {
	t.Helper()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	contextID := service.projects.contextID
	return func() {
		t.Helper()
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(config, after) || service.projects.contextID != contextID {
			t.Fatal("location facade changed configuration/context:", err)
		}
	}
}

func TestPlotLocationReviewFacadeDefaultOffAndIndependentGate(t *testing.T) {
	service, state := contextServiceFixture(t)
	check := assertPlotLocationFacadeReadOnly(t, service)
	defer check()
	for _, flag := range []*bool{
		&service.siviHeightEnabled, &service.siviParentReviewEnabled,
		&service.siviParentEditingEnabled, &service.siviParentActionEditingEnabled,
		&service.siviProjectAssignmentEnabled, &service.tableCSVReviewEnabled,
	} {
		*flag = true
		if got, err := service.GetPlotLocationReview(context.Background(), state.ContextID); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatal("unrelated opt-in enabled location review:", got, err)
		}
	}
	service.plotLocationReviewEnabled = false
	if got, err := service.GetPlotLocationReview(context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("explicit false enabled location review:", got, err)
	}
}

func TestPlotLocationReviewFacadeExactDTOParityJSONAndDetachedValues(t *testing.T) {
	service, state := contextServiceFixture(t)
	check := assertPlotLocationFacadeReadOnly(t, service)
	defer check()
	service.plotLocationReviewEnabled = true
	want, err := service.readPlotLocations(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetPlotLocationReview(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(got, want) || len(got.Report.Rows) == 0 {
		t.Fatal("public facade changed private location review:", got, err)
	}
	assertKeys := func(value any, want []string) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		if !reflect.DeepEqual(keys, want) {
			t.Fatal("public JSON field contract changed:", keys, want)
		}
	}
	assertKeys(got, []string{"contextId", "projectPath", "report", "suPath"})
	assertKeys(got.Report, []string{"fields", "project", "rows", "su"})
	assertKeys(got.Report.Rows[0], []string{"adminRowId", "envRowId", "membershipRowIds", "storedLongitude", "values"})
	got.Report.Fields[0].Key = "caller"
	got.Report.Rows[0].EnvRowID = "caller"
	*got.Report.Rows[0].Values[0].Text = "caller"
	got.Report.Rows[0].StoredLongitude = ProjectMetadataCell{Storage: "null"}
	got.Report.Rows[0].MembershipRowIDs = append(got.Report.Rows[0].MembershipRowIDs, "caller")
	again, err := service.GetPlotLocationReview(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("caller aliases later public location snapshot:", err)
	}
	if got, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Env"); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("location opt-in enabled table CSV:", err)
	}
	if got, err := service.GetSIVIParentOriginal(context.Background(), state.ContextID, "108050"); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("location opt-in enabled SIVI:", err)
	}
}

func TestPlotLocationReviewFacadeStaleOwnershipCancellationRetryAndNilErrors(t *testing.T) {
	service, state := contextServiceFixture(t)
	service.plotLocationReviewEnabled = true
	check := assertPlotLocationFacadeReadOnly(t, service)
	defer check()
	if got, err := service.GetPlotLocationReview(context.Background(), "stale"); got != nil || err == nil {
		t.Fatal("stale context returned locations:", got, err)
	}
	owner := service.projects.sqlite
	prior := owner.attachmentInfo["project"]
	owner.attachmentInfo["project"] = owner.attachmentInfo["VPro64"]
	got, err := service.GetPlotLocationReview(context.Background(), state.ContextID)
	owner.attachmentInfo["project"] = prior
	if got != nil || err == nil {
		t.Fatal("foreign owner returned locations:", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.GetPlotLocationReview(ctx, state.ContextID); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("pre-request cancellation returned locations:", got, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	queued := &tableCSVLeaseContext{Context: ctx, queued: make(chan struct{})}
	owner.mu.Lock()
	locked := true
	defer func() {
		if locked {
			owner.mu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		value, err := service.GetPlotLocationReview(queued, state.ContextID)
		if value != nil {
			done <- errors.New("cancelled facade returned partial locations")
			return
		}
		done <- err
	}()
	select {
	case <-queued.queued:
	case <-time.After(5 * time.Second):
		t.Fatal("facade did not reach held snapshot mutex")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued facade cancellation lost:", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled facade retained leases")
	}
	owner.mu.Unlock()
	locked = false
	if got, err := service.GetPlotLocationReview(context.Background(), state.ContextID); got == nil || err != nil {
		t.Fatal("facade retry discarded pinned context:", err)
	}
}

func TestPlotLocationReviewFacadeRejectsPhysicalViewWithNil(t *testing.T) {
	service, state := contextServiceFixture(t)
	service.plotLocationReviewEnabled = true
	mutateContextFixture(t, state.ProjectPath, `DROP TABLE Sample_Admin; CREATE VIEW Sample_Admin AS SELECT PlotNumber AS Plot FROM Sample_Env`)
	check := assertPlotLocationFacadeReadOnly(t, service)
	defer check()
	if got, err := service.GetPlotLocationReview(context.Background(), state.ContextID); got != nil || err == nil || !strings.Contains(err.Error(), "physical table") {
		t.Fatal("physical view impostor returned locations:", got, err)
	}
}
