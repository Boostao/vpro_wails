package main

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
	read := func(offset, limit int) *TwoPageProfilingReview {
		t.Helper()
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(config, after) ||
			!reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
			t.Fatal("profiling wire changed data/config/history", err)
		}
		got.ContextID, got.ProjectPath, got.Source.Path = "wire-owned", `C:\fixture\Sample.db`, `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Default     *TwoPageProfilingReview `json:"default"`
		Page        *TwoPageProfilingReview `json:"page"`
		End         *TwoPageProfilingReview `json:"end"`
		None        *TwoPageProfilingReview `json:"none"`
		PagingFirst *TwoPageProfilingReview `json:"pagingFirst"`
		PagingNext  *TwoPageProfilingReview `json:"pagingNext"`
		Absent      *TwoPageProfilingReview `json:"absent"`
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
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	var err error
	state, err = service.contexts.SwitchContext(state.ContextID, selection)
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
	path := filepath.Join("frontend", "src", "twoPageProfilingWire.json")
	if os.Getenv("VPRO_UPDATE_TWO_PAGE_PROFILING_WIRE") == "true" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(path)
	if err != nil {
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
