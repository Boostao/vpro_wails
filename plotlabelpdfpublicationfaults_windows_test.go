package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnedPlotLabelPDFStageFailuresRetainNoDestination(t *testing.T) {
	for _, fault := range []string{"short-write", "cleanup-after-commit", "corrupt-stage"} {
		contexts, state, runtime := plotLabelPDFOwnedFixture(t)
		expected, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), fault+".pdf")
		hooks := plotLabelPDFPublicationHooks{runtime: runtime}
		switch fault {
		case "short-write":
			hooks.publication.write = func(file *os.File, data []byte) (int, error) {
				return file.Write(data[:len(data)-1])
			}
		case "cleanup-after-commit":
			hooks.publication.remove = func(path string) error {
				if err := os.Remove(path); err != nil {
					return err
				}
				return errors.New("planned committed stage cleanup fault")
			}
		case "corrupt-stage":
			hooks.publication.observe = func(phase, stage string) error {
				if phase == "staged" {
					return os.WriteFile(stage, []byte("%PDF-not-the-approved-output"), 0600)
				}
				return nil
			}
		}
		result, err := contexts.publishOwnedPlotLabelPDF(t.Context(), state.ContextID, *expected, destination, hooks)
		if err == nil || result.Published != (fault == "cleanup-after-commit") {
			t.Fatalf("%s: %+v, %v", fault, result, err)
		}
		if fault == "short-write" && !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("short write hidden: %v", err)
		}
		if result.Published {
			if result.SHA256 == "" || !strings.Contains(err.Error(), "do not replay") {
				t.Fatalf("committed cleanup receipt lost: %+v, %v", result, err)
			}
		} else if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed stage left destination: %v", err)
		}
	}
}

func TestOwnedPlotLabelPDFTypedNullEmptyAndStorageRefusals(t *testing.T) {
	contexts, state, runtime := plotLabelPDFOwnedFixture(t)
	for _, value := range []string{"NULL", "''"} {
		mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET PlotRepresenting=`+value+`,ProjectID=`+value+` WHERE PlotNumber='LB001'`)
		plan, err := contexts.readPlotLabelPDFPreparation(t.Context(), state.ContextID, "LB001", runtime)
		if err != nil || (plan.Preview.Header.ProjectID == nil) != (value == "NULL") {
			t.Fatalf("typed metadata %s: %v", value, err)
		}
	}
	for _, mutation := range []string{
		`UPDATE Sample_Env SET Zone=NULL WHERE PlotNumber='LB001'`,
		`UPDATE Sample_Env SET Zone=CAST(X'4348' AS BLOB) WHERE PlotNumber='LB001'`,
		`UPDATE Sample_Env SET Zone='CWH',ProjectID=CAST(X'FF' AS TEXT) WHERE PlotNumber='LB001'`,
	} {
		mutateContextFixture(t, state.ProjectPath, mutation)
		if plan, err := contexts.readPlotLabelPDFPreparation(context.Background(), state.ContextID, "LB001", runtime); err == nil || plan != nil {
			t.Fatalf("invalid original storage admitted: %v", err)
		}
	}
}
