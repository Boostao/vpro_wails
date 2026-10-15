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

func TestTwoPageEnvironmentTableProductionWire(t *testing.T) {
	service, state, db := twoPageEnvironmentTableFixture(t, true)
	read := func(target string, offset, limit int) *TwoPageEnvironmentTableReview {
		t.Helper()
		if err := service.contexts.projects.preferences.update("Current", map[string]any{"DataFormName": target}); err != nil {
			t.Fatal(err)
		}
		beforeData := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		beforeConfig, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		afterConfig, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(beforeConfig, afterConfig) ||
			!reflect.DeepEqual(beforeData, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
			t.Fatal("production wire review changed configuration/data", err)
		}
		got.ContextID, got.ProjectPath = "wire-owned", `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Default     *TwoPageEnvironmentTableReview `json:"default"`
		XL          *TwoPageEnvironmentTableReview `json:"xl"`
		Normal      *TwoPageEnvironmentTableReview `json:"normal"`
		Chars       *TwoPageEnvironmentTableReview `json:"chars"`
		Page        *TwoPageEnvironmentTableReview `json:"page"`
		Empty       *TwoPageEnvironmentTableReview `json:"empty"`
		None        *TwoPageEnvironmentTableReview `json:"none"`
		PagingFirst *TwoPageEnvironmentTableReview `json:"pagingFirst"`
		PagingNext  *TwoPageEnvironmentTableReview `json:"pagingNext"`
	}
	wire := wireFixture{
		Default: read("FS882-6x4", 0, 50), XL: read("FS882-6x4XL", 0, 50),
		Normal: read("FS882-8x6XL", 0, 50), Chars: read("FS882-8x6XL-CHARS", 0, 50),
		Page: read("FS882-6x4", 4, 2), Empty: read("FS882-6x4", 13, 50),
	}
	if _, err := db.Exec(`WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<19)
		INSERT INTO Sample_Admin(Plot) SELECT 'A' FROM n`); err != nil {
		t.Fatal(err)
	}
	wire.PagingFirst, wire.PagingNext = read("FS882-6x4", 0, 50), read("FS882-6x4", 50, 50)
	if wire.PagingFirst.TotalRows != 70 || len(wire.PagingFirst.Rows) != 50 || len(wire.PagingNext.Rows) != 20 {
		t.Fatal("independent environment paging expectation changed")
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	var err error
	state, err = service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	wire.None = read("FS882-6x4", 0, 50)
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "twoPageEnvironmentTableWire.json")
	if os.Getenv("VPRO_UPDATE_TWO_PAGE_ENVIRONMENT_TABLE_WIRE") == "true" {
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
		t.Fatal("environment production fixture differs from independent owned service responses")
	}
}
