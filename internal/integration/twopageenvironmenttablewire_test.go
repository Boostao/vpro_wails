package integration

import (
	"github.com/boostao/vpro-wails/internal/datasheet"
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

func TestTwoPageEnvironmentTableProductionWire(t *testing.T) {
	service, state, db := twoPageEnvironmentTableFixture(t, true)
	read := func(target string, offset, limit int) *datasheet.TwoPageEnvironmentTableReview {
		t.Helper()
		if err := service.Contexts.Projects.Preferences.Update("Current", map[string]any{"DataFormName": target}); err != nil {
			t.Fatal(err)
		}
		beforeData := vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)
		beforeConfig, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		afterConfig, err := os.ReadFile(service.Contexts.Projects.Preferences.Path)
		if err != nil || !reflect.DeepEqual(beforeConfig, afterConfig) ||
			!reflect.DeepEqual(beforeData, vprotest.DatabaseBytes(t, service.Contexts.Projects.Sqlite.Attachments)) {
			t.Fatal("production wire review changed configuration/data", err)
		}
		got.ContextID, got.ProjectPath = "wire-owned", `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Default     *datasheet.TwoPageEnvironmentTableReview `json:"default"`
		XL          *datasheet.TwoPageEnvironmentTableReview `json:"xl"`
		Normal      *datasheet.TwoPageEnvironmentTableReview `json:"normal"`
		Chars       *datasheet.TwoPageEnvironmentTableReview `json:"chars"`
		Page        *datasheet.TwoPageEnvironmentTableReview `json:"page"`
		Empty       *datasheet.TwoPageEnvironmentTableReview `json:"empty"`
		None        *datasheet.TwoPageEnvironmentTableReview `json:"none"`
		PagingFirst *datasheet.TwoPageEnvironmentTableReview `json:"pagingFirst"`
		PagingNext  *datasheet.TwoPageEnvironmentTableReview `json:"pagingNext"`
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
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	var err error
	state, err = service.Contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	wire.None = read("FS882-6x4", 0, 50)
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "twoPageEnvironmentTableWire.json")
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
		t.Fatal("environment production fixture differs from independent owned service responses")
	}
}
