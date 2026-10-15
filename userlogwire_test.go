package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUserLogSharedActualGoWire(t *testing.T) {
	contexts, state := userLogFixture(t)
	service := &UserLogService{contexts: contexts, enabled: true}
	wires := map[string]UserLogReview{}
	for _, item := range []struct {
		name                string
		mode, offset, limit int
	}{
		{"all", 1, 0, 50}, {"current", 2, 0, 50},
		{"page", 1, 1, 2}, {"end", 2, 10, 50},
	} {
		value, err := service.GetReview(context.Background(), state.ContextID, item.mode, item.offset, item.limit)
		if err != nil {
			t.Fatal(err)
		}
		value.ContextID = "owned-context"
		value.SourcePath = `C:\fixtures\saved '# log\VPro64.db`
		wires[item.name] = value
	}
	encoded, err := json.MarshalIndent(wires, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("VPRO_USER_LOG_WIRE_PATH")
	if path == "" {
		path = filepath.Join("frontend", "src", "userLogWire.json")
	}
	if os.Getenv("VPRO_GENERATE_USER_LOG_WIRE") == "true" {
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
		t.Fatal("shared frontend fixture differs from the actual Go service wire")
	}
}
