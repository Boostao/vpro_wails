package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBreakReportSharedActualGoWire(t *testing.T) {
	contexts, state := breakReportFixture(t)
	service := &BreakReportService{contexts: contexts, enabled: true}
	wires := map[string]BreakReportReview{}
	for _, item := range []struct {
		name          string
		offset, limit int
	}{{"first", 0, 50}, {"page", 5, 1}, {"end", 100, 50}} {
		value, err := service.GetReview(context.Background(), state.ContextID, item.offset, item.limit)
		if err != nil {
			t.Fatal(err)
		}
		value.ContextID = "break-context"
		value.SourcePath = `C:\fixtures\saved '# codes\VPro64.db`
		wires[item.name] = value
	}
	mutateContextFixture(t, contexts.projects.sqlite.attachments["VPro64"], `DELETE FROM USysBreakTable`)
	empty, err := service.GetReview(context.Background(), state.ContextID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	empty.ContextID, empty.SourcePath = wires["first"].ContextID, wires["first"].SourcePath
	wires["empty"] = empty
	encoded, err := json.MarshalIndent(wires, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "breakReportWire.json")
	if os.Getenv("VPRO_GENERATE_BREAK_REPORT_WIRE") == "true" {
		output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := output.Write(encoded); err != nil {
			output.Close()
			t.Fatal(err)
		}
		if err := output.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("shared frontend fixture differs from actual Go break-report wire")
	}
}
