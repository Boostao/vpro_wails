package lifecycle

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
	service := &UserLogService{contexts: contexts}
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
	path := filepath.Join("frontend", "src", "userLogWire.json")
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
