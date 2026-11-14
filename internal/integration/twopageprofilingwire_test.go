package integration

import (
	"github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageProfilingProductionWire(t *testing.T) {
	service, state, db := twoPageProfilingFixture(t, true, false)
	read := func(offset, limit int) *classification.TwoPageProfilingReview {
		t.Helper()
		before := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
		if err != nil || !reflect.DeepEqual(config, after) ||
			!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)) {
			t.Fatal("profiling wire changed data/config/history", err)
		}
		got.ContextID, got.ProjectPath, got.Source.Path = "wire-owned", `C:\fixture\Sample.db`, `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Default     *classification.TwoPageProfilingReview `json:"default"`
		Page        *classification.TwoPageProfilingReview `json:"page"`
		End         *classification.TwoPageProfilingReview `json:"end"`
		None        *classification.TwoPageProfilingReview `json:"none"`
		PagingFirst *classification.TwoPageProfilingReview `json:"pagingFirst"`
		PagingNext  *classification.TwoPageProfilingReview `json:"pagingNext"`
		Absent      *classification.TwoPageProfilingReview `json:"absent"`
	}
	wire := wireFixture{Default: read(0, 50), Page: read(1, 1), End: read(100000, 50)}
	if _, err := db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<66)
		INSERT INTO Sample_Profile("Order") SELECT i FROM n`); err != nil {
		t.Fatal(err)
	}
	wire.PagingFirst, wire.PagingNext = read(0, 50), read(50, 50)
	if wire.PagingFirst.TotalRows != 70 || len(wire.PagingFirst.ProfileOriginal.Rows) != 50 ||
		len(wire.PagingNext.ProfileOriginal.Rows) != 20 {
		t.Fatal("independent50/20 profile page expectations changed")
	}
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	var err error
	state, err = service.Contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	wire.None = read(0, 50)
	if _, err := db.Exec(`DROP TABLE _table_metadata`); err != nil {
		t.Fatal(err)
	}
	wire.Absent = read(0, 50)
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "tests", "fixtures", "twoPageProfilingWire.json")
	fixture, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(append(encoded, '\n'), fixture) {
		t.Fatal(err)
	}
	var decoded wireFixture
	decoder := json.NewDecoder(bytes.NewReader(fixture))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, wire) {
		t.Fatal("profiling wire differs from actual owned Go responses")
	}
}
