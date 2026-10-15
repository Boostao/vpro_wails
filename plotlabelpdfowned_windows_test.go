package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func plotLabelPDFOwnedFixture(t *testing.T) (*ContextService, ProjectState, plotLabelPDFRuntime) {
	t.Helper()
	service, state := plotLabelRegionalFixture(t)
	mutateContextFixture(t, state.ProjectPath, `ALTER TABLE Sample_Admin ADD COLUMN UnusedLabelWitness TEXT`)
	fonts := plotLabelPDFWindowsTestFonts(t)
	return service.contexts, state, plotLabelPDFRuntime{
		now: service.now, format: service.format,
		fonts: func(context.Context) (plotLabelPDFFonts, error) { return fonts, nil },
	}
}

func TestOwnedPlotLabelPDFReadPublicationAndNoDatabaseWrites(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	again, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil || !reflect.DeepEqual(expected, again) {
		t.Fatalf("source review retry: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "label # owned.pdf")
	if evidence := os.Getenv("VPRO_PRIVATE_LABEL_PDF_PUBLICATION_EVIDENCE"); evidence != "" {
		destination = filepath.Join(evidence, "published.pdf")
	}
	result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, plotLabelPDFPublicationHooks{runtime: runtime})
	if err != nil || !result.Published || result.Path == "" || result.SHA256 == "" {
		t.Fatalf("publication: %+v, %v", result, err)
	}
	requestedInfo, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	publishedInfo, err := os.Stat(result.Path)
	if err != nil || !os.SameFile(requestedInfo, publishedInfo) {
		t.Fatalf("normalized published destination identity: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, expected.Bytes) {
		t.Fatalf("published content: %v", err)
	}
	collision, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, plotLabelPDFPublicationHooks{runtime: runtime})
	if err == nil || collision.Published {
		t.Fatalf("existing destination overwritten: %+v, %v", collision, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestOwnedPlotLabelPDFRejectsSourceDrift(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Env SET PlotRepresenting='changed' WHERE PlotNumber='LB001'`,
		`UPDATE Sample_Admin SET UnusedLabelWitness='unrelated but physical changed' WHERE Plot='LB001'`,
		`ALTER TABLE Sample_Admin RENAME TO OriginalLabelAdmin;
CREATE TABLE Sample_Admin AS SELECT * FROM OriginalLabelAdmin;
INSERT INTO Sample_Admin SELECT * FROM Sample_Admin WHERE Plot='LB001'`,
		`UPDATE Sample_Env SET ProjectID=CAST(X'FF' AS TEXT) WHERE PlotNumber='LB001'`,
	} {
		contexts, state, runtime := plotLabelPDFOwnedFixture(t)
		expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
		if err != nil {
			t.Fatal(err)
		}
		mutateContextFixture(t, state.ProjectPath, mutation)
		destination := filepath.Join(t.TempDir(), "refused.pdf")
		result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, plotLabelPDFPublicationHooks{runtime: runtime})
		if err == nil || result.Published {
			t.Fatalf("source drift published: %+v, %v", result, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("refused destination exists: %v", err)
		}
	}
}

func TestOwnedPlotLabelPDFPrelinkAndPostcommitResult(t *testing.T) {
	for _, phase := range []string{"precommit", "published"} {
		contexts, state, runtime := plotLabelPDFOwnedFixture(t)
		expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "timing.pdf")
		hooks := plotLabelPDFPublicationHooks{
			runtime: runtime,
			observe: func(actual string) error {
				if actual == phase {
					mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET PlotRepresenting='drift' WHERE PlotNumber='LB001'`)
				}
				return nil
			},
		}
		result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, hooks)
		if err == nil || result.Published != (phase == "published") {
			t.Fatalf("%s result: %+v, %v", phase, result, err)
		}
		if result.Published {
			data, readErr := os.ReadFile(destination)
			if readErr != nil || !bytes.Equal(data, expected.Bytes) || result.SHA256 == "" ||
				!strings.Contains(err.Error(), "do not replay") {
				t.Fatalf("committed result lost: %+v, %v, %v", result, err, readErr)
			}
		}
	}
}

func TestOwnedPlotLabelPDFCleanupErrorsKeepPublication(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "cleanup.pdf")
	observations := 0
	hooks := plotLabelPDFPublicationHooks{
		runtime: runtime,
		snapshot: publicationReadSnapshotHooks{rollbackRead: func(*sql.Tx) error {
			observations++
			if observations == 3 {
				return errors.New("planned postcommit snapshot cleanup fault")
			}
			return nil
		}},
	}
	result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, hooks)
	if !result.Published || result.SHA256 == "" || err == nil || !strings.Contains(err.Error(), "cleanup fault") {
		t.Fatalf("postcommit cleanup zeroed result: %+v, %v", result, err)
	}
}

func TestOwnedPlotLabelPDFDateDriftAndCancellation(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
	if err != nil {
		t.Fatal(err)
	}
	changed := runtime
	changed.now = func() time.Time { return time.Date(2026, 10, 12, 12, 0, 0, 0, time.Local) }
	changed.format = func(time.Time) (plotLabelNativeDate, error) {
		return plotLabelNativeDate{"Monday, October 12, 2026", "en-US", "dddd, MMMM d, yyyy"}, nil
	}
	result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, filepath.Join(t.TempDir(), "stale-date.pdf"), plotLabelPDFPublicationHooks{runtime: changed})
	if err == nil || result.Published {
		t.Fatalf("stale date accepted: %+v, %v", result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	hooks := plotLabelPDFPublicationHooks{runtime: runtime, observe: func(phase string) error {
		if phase == "precommit" {
			cancel()
		}
		return nil
	}}
	result, err = contexts.publishOwnedPlotLabelPDF(ctx, state.ContextID, *expected, filepath.Join(t.TempDir(), "cancel.pdf"), hooks)
	cancel()
	if !errors.Is(err, context.Canceled) || result.Published {
		t.Fatalf("precommit cancellation: %+v, %v", result, err)
	}
}
