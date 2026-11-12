package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"os"

	"reflect"
	"testing"
	"time"
)

func TestLongEnvironmentPreviewOwnedPhysicalScopeAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := vprotest.ReportServiceFixture(t, external)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			options, err := GetLongEnvironmentOptions(service, context.Background(), state.ContextID)
			if err != nil || options.ContextID != state.ContextID || options.Title != "Long Evironment Report" {
				t.Fatal("original YAML default unavailable", options, err)
			}
			preview, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{"  Custom title  "})
			if err != nil || preview.ContextID != state.ContextID || preview.ProjectPath != state.ProjectPath || preview.SUPath != state.SUPath {
				t.Fatal("owned readonly preview failed", preview, err)
			}
			if len(preview.Report.Fields) != 72 || len(preview.Report.Units) != 2 || preview.Report.Title != "  Custom title  " ||
				preview.Report.Units[0].Code != "  Unit 'quoted'  " || len(preview.Report.Units[0].Plots) != 1 ||
				preview.Report.Units[0].Plots[0].PlotNumber != "108050" || preview.Report.Units[1].Plots[0].Status != "missing_env_and_admin" {
				t.Fatal("selected physical SU/orphan scope differs", preview)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("preview/options changed configuration", err)
			}
		})
	}
}

func TestLongEnvironmentPreviewCancellationStalenessAndRetry(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner := service.Projects.Sqlite
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	preview, err := PreviewLongEnvironment(service, ctx, state.ContextID, LongEnvironmentRequest{"Title"})
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(preview, LongEnvironmentPreview{}) {
		t.Fatal("cancelled read publishes output or loses cancellation", preview, err)
	}
	if _, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal("cancelled report leaked lease", err)
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err == nil {
		t.Fatal("stale preview published")
	}
	if _, err := GetLongEnvironmentOptions(service, context.Background(), state.ContextID); err == nil {
		t.Fatal("stale options published")
	}
	if _, err := PreviewLongEnvironment(service, context.Background(), next.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal(err)
	}
}

func TestLongEnvironmentPreviewStrictTransportAndConfiguredTitle(t *testing.T) {
	for _, raw := range []string{`{}`, `{"title":null}`, `{"title":"\ud800"}`, `{"title":"ok","quality":"Poor"}`} {
		var request LongEnvironmentRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("incomplete/repaired/unsupported transport accepted", raw)
		}
	}
	service, state := vprotest.ReportServiceFixture(t, false)
	for _, title := range []string{"\x00", string([]byte{0xff})} {
		if _, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{title}); err == nil {
			t.Fatal("invalid title repaired", title)
		}
	}
	if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LEReportTitle": "  Remembered title  "}); err != nil {
		t.Fatal(err)
	}
	options, err := GetLongEnvironmentOptions(service, context.Background(), state.ContextID)
	if err != nil || options.Title != "  Remembered title  " {
		t.Fatal("existing YAML title discarded", options, err)
	}
	if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LEReportTitle": 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := GetLongEnvironmentOptions(service, context.Background(), state.ContextID); err == nil {
		t.Fatal("invalid configured title silently defaulted")
	}
}

func TestLongEnvironmentPreviewRequiresSelectedAndUnchangedOwnedFiles(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewLongEnvironment(service, context.Background(), next.ContextID, LongEnvironmentRequest{"Title"}); err == nil {
		t.Fatal("no SU silently became all project plots")
	}
	service, state = vprotest.ReportServiceFixture(t, true)
	owner := service.Projects.Sqlite
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	if preview, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err == nil ||
		!reflect.DeepEqual(preview, LongEnvironmentPreview{}) {
		t.Fatal("changed file ownership published report", preview, err)
	}
	owner.AttachmentInfo["su"] = original
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := PreviewLongEnvironment(service, context.Background(), state.ContextID, LongEnvironmentRequest{"Title"}); err != nil {
		t.Fatal("rejected ownership read leaked lease", err)
	}
}
