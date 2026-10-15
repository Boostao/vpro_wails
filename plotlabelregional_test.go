package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func plotLabelRegionalFixture(t *testing.T) (*PlotLabelRegionalService, ProjectState) {
	t.Helper()
	contexts, state := contextServiceFixture(t)
	t.Setenv(plotLabelRegionalDateEnvironment, "true")
	service, err := NewPlotLabelRegionalService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, contexts.projects.sqlite.attachments["project"],
		`INSERT INTO Sample_Env(PlotNumber,Zone,SubZone,SiteSeries,PlotRepresenting,ProjectID)
VALUES('LB001','CWH','dm','01','  source representing  ',' literal ID ')`)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["project"],
		`INSERT INTO Sample_Admin(Plot) VALUES('LB001')`)
	service.now = func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.Local) }
	service.format = func(time.Time) (plotLabelNativeDate, error) {
		return plotLabelNativeDate{"Sunday, October 11, 2026", "en-US", "dddd, MMMM d, yyyy"}, nil
	}
	return service, state
}

func TestPlotLabelRegionalReadStrictWireAndNoWrites(t *testing.T) {
	service, state := plotLabelRegionalFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetPreview(t.Context(), state.ContextID, "LB001")
	if err != nil || got == nil {
		t.Fatal(got, err)
	}
	if got.ContextID != state.ContextID || got.PlotNumber != "LB001" || got.Header.PlotNumber != "LB001" ||
		*got.Header.Zone != "CWH" || *got.Header.SubZone != "dm" || *got.Header.SiteSeries != "01" ||
		*got.Header.ProjectID != " literal ID " || *got.Header.PlotRepresenting != "  source representing  " ||
		got.LabelDate != "Sunday, October 11, 2026" || got.ISODate != "2026-10-11" ||
		got.Locale != "en-US" || got.Pattern != "dddd, MMMM d, yyyy" || got.Source != "windows-user-long-date" {
		t.Fatal("independent label source/date values changed", got)
	}
	got.ContextID = "label-wire"
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PlotLabelRegionalPreview
	decoder := json.NewDecoder(bytes.NewReader(wire))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(got, &decoded) {
		t.Fatal("strict regional label wire changed", err)
	}
	if path := os.Getenv("VPRO_PLOT_LABEL_REGIONAL_WIRE_PATH"); path != "" {
		if err := os.WriteFile(path, append(wire, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	afterConfig, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(config, afterConfig) ||
		!reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
		t.Fatal("regional label read wrote data/config", err)
	}
}

func TestPlotLabelRegionalGateContextAndExplicitDateErrors(t *testing.T) {
	service, state := plotLabelRegionalFixture(t)
	t.Setenv(plotLabelRegionalDateEnvironment, "false")
	disabled, err := NewPlotLabelRegionalService(service.contexts)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := disabled.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || result != nil {
		t.Fatal("backend-off date returned partial success", result, err)
	}
	t.Setenv(plotLabelRegionalDateEnvironment, "invalid")
	if _, err := NewPlotLabelRegionalService(service.contexts); err == nil {
		t.Fatal("malformed feature flag accepted")
	}
	for _, owner := range []string{"", "foreign"} {
		if result, err := service.GetPreview(t.Context(), owner, "LB001"); err == nil || result != nil {
			t.Fatal("foreign owner returned a label", result, err)
		}
	}
	for _, plot := range []string{"", "missing", "LB001 ", "lb001", "\x00", "\xff", "12345678"} {
		if result, err := service.GetPreview(t.Context(), state.ContextID, plot); err == nil || result != nil {
			t.Fatal("invalid/literal nonmember returned a label", plot, result, err)
		}
	}
	for _, metadata := range []plotLabelNativeDate{
		{"", "en-US", "pattern"}, {"date", "", "pattern"}, {"date", "en-US", ""},
		{"\xff", "en-US", "pattern"}, {"date", "en-US", "\x00"}, {"date", "en-US", strings.Repeat("x", 4096)},
	} {
		service.format = func(time.Time) (plotLabelNativeDate, error) { return metadata, nil }
		if result, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || result != nil {
			t.Fatal("malformed native date returned partial label", result, err)
		}
	}
	service.format = func(time.Time) (plotLabelNativeDate, error) {
		return plotLabelNativeDate{}, errors.New("native calendar unavailable")
	}
	if result, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || result != nil ||
		!strings.Contains(err.Error(), "native calendar unavailable") {
		t.Fatal("native error silently fell back", result, err)
	}
}

func TestPlotLabelRegionalMalformedStorageCorrectionAndNullEmpty(t *testing.T) {
	service, state := plotLabelRegionalFixture(t)
	path := service.contexts.projects.sqlite.attachments["project"]
	for _, assignment := range []string{
		"Zone=NULL", "Zone='12345'", "Zone=CAST(X'FF' AS TEXT)", "Zone=X'434857'",
		"ProjectID=CAST(X'FF' AS TEXT)", "PlotRepresenting=CAST(X'FF' AS TEXT)",
		"ProjectID='" + strings.Repeat("x", 51) + "'", "SubZone='" + strings.Repeat("x", 46) + "',SiteSeries=''",
	} {
		mutateContextFixture(t, path, "UPDATE Sample_Env SET "+assignment+" WHERE PlotNumber='LB001'")
		if result, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || result != nil {
			t.Fatal("invalid source was repaired before JSON", assignment, result, err)
		}
		mutateContextFixture(t, path,
			`UPDATE Sample_Env SET Zone='CWH',ProjectID=' literal ID ',PlotRepresenting='  source representing  ',SubZone='dm',SiteSeries='01' WHERE PlotNumber='LB001'`)
		if result, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err != nil || result == nil {
			t.Fatal("source correction did not retry", result, err)
		}
	}
	mutateContextFixture(t, path, `UPDATE Sample_Env SET Zone='',SubZone=NULL,SiteSeries=NULL,ProjectID='',PlotRepresenting=NULL WHERE PlotNumber='LB001'`)
	got, err := service.GetPreview(t.Context(), state.ContextID, "LB001")
	if err != nil || got == nil || got.Header.Zone == nil || *got.Header.Zone != "" ||
		got.Header.ProjectID == nil || *got.Header.ProjectID != "" || got.Header.PlotRepresenting != nil ||
		got.Header.SubZone != nil || got.Header.SiteSeries != nil {
		t.Fatal("valid empty/NULL distinctions changed", got, err)
	}
}

func TestPlotLabelRegionalCancellationLeaseAndRetry(t *testing.T) {
	service, state := plotLabelRegionalFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	service.format = func(time.Time) (plotLabelNativeDate, error) {
		close(entered)
		<-release
		return plotLabelNativeDate{"Sunday, October 11, 2026", "en-US", "dddd, MMMM d, yyyy"}, nil
	}
	done := make(chan error, 1)
	go func() {
		result, err := service.GetPreview(ctx, state.ContextID, "LB001")
		if result != nil {
			done <- errors.New("cancelled date published a partial label")
			return
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("native date phase was not reached")
	}
	if service.contexts.projects.operationMu.TryLock() {
		service.contexts.projects.operationMu.Unlock()
		t.Fatal("native date phase lost its owning context lease")
	}
	cancel()
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation did not discard the native date", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled regional label did not release its lease")
	}
	service.format = func(time.Time) (plotLabelNativeDate, error) {
		return plotLabelNativeDate{"Sunday, October 11, 2026", "en-US", "dddd, MMMM d, yyyy"}, nil
	}
	if got, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err != nil || got == nil {
		t.Fatal("read failed after in-flight cancellation", got, err)
	}
}
