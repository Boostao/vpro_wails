package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOwnedPlotLabelPDFExternalLiteralSnapshots(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	path := filepath.Join(t.TempDir(), "external '# 日本.db")
	data, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(state)
	selection.ProjectPath = path
	next, err := contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	plan, err := contexts.readPlotLabelPDFPreparation(t.Context(), next.ContextID, "LB001", runtime)
	if err != nil || plan.Selection.ProjectPath != next.ProjectPath {
		t.Fatalf("external physical source: %v", err)
	}
	for _, plot := range []string{"lb001", "LB001 ", "00001"} {
		if got, err := contexts.readPlotLabelPDFPreparation(t.Context(), next.ContextID, plot, runtime); err == nil || got != nil {
			t.Fatalf("foreign literal %q admitted: %v", plot, err)
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestOwnedPlotLabelPDFFullPeerSurvivesAutomaticRowidReuse(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Admin WHERE Plot='LB001';
INSERT INTO Sample_Admin(Plot,UnusedLabelWitness) VALUES('LB001','replacement peer')`)
	observed, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil || observed.Source[1].Rows[0].RowID != expected.Source[1].Rows[0].RowID ||
		!bytes.Equal(observed.Bytes, expected.Bytes) {
		t.Fatalf("required same-rowid/same-PDF replacement witness absent: %v", err)
	}
	result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected,
		filepath.Join(t.TempDir(), "replaced.pdf"), plotLabelPDFPublicationHooks{runtime: runtime})
	if err == nil || result.Published {
		t.Fatalf("replacement peer admitted: %+v, %v", result, err)
	}
}

func TestOwnedPlotLabelPDFFontAndPreparedArtifactGuards(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	changedRuntime := runtime
	changedRuntime.fonts = func(ctx context.Context) (plotLabelPDFFonts, error) {
		fonts, err := runtime.fonts(ctx)
		fonts.regular = append(bytes.Clone(fonts.regular), 0)
		return fonts, err
	}
	for _, variant := range []string{"font", "artifact", "header", "owner", "selection"} {
		plan, err := snapshotPlotLabelPDFPreparation(*expected)
		if err != nil {
			t.Fatal(err)
		}
		active := runtime
		switch variant {
		case "font":
			active = changedRuntime
		case "artifact":
			plan.Bytes = []byte("%PDF-forged")
		case "header":
			*plan.Preview.Header.ProjectID = "forged"
		case "owner":
			plan.Preview.ContextID = "foreign"
		case "selection":
			plan.Selection.SU = "foreign"
		}
		result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, plan,
			filepath.Join(t.TempDir(), variant+".pdf"), plotLabelPDFPublicationHooks{runtime: active})
		if err == nil || result.Published {
			t.Fatalf("%s guard: %+v, %v", variant, result, err)
		}
	}
}

func TestOwnedPlotLabelPDFInFlightCancellationRetainsCoordinator(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	before, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	active := runtime
	active.fonts = func(ctx context.Context) (plotLabelPDFFonts, error) {
		cancel()
		return runtime.fonts(ctx)
	}
	result, err := contexts.readPlotLabelPDFPreparation(ctx, state.ContextID, "LB001", active)
	cancel()
	if !errors.Is(err, context.Canceled) || result != nil {
		t.Fatalf("in-flight source cancellation: %v", err)
	}
	retry, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil || !reflect.DeepEqual(before, retry) {
		t.Fatalf("cancelled snapshot discarded attached coordinator: %v", err)
	}
}
