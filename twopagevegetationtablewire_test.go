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

func TestTwoPageVegetationTableProductionWire(t *testing.T) {
	service, state, _ := twoPageVegetationTableFixture(t, true)
	check := assertPlotLocationFacadeReadOnly(t, service.contexts)
	read := func(form string, offset, limit int) *TwoPageVegetationTableReview {
		t.Helper()
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", form, offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		got.ContextID, got.ProjectPath = "wire-owned", `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Normal      *TwoPageVegetationTableReview `json:"normal"`
		Chars       *TwoPageVegetationTableReview `json:"chars"`
		Page        *TwoPageVegetationTableReview `json:"page"`
		Empty       *TwoPageVegetationTableReview `json:"empty"`
		None        *TwoPageVegetationTableReview `json:"none"`
		PagingFirst *TwoPageVegetationTableReview `json:"pagingFirst"`
		PagingNext  *TwoPageVegetationTableReview `json:"pagingNext"`
	}
	wire := wireFixture{
		Normal: read("FS882-8x6XL", 0, 50),
		Chars:  read("FS882-8x6XL-CHARS", 0, 50),
		Page:   read("FS882-8x6XL", 2, 2),
		Empty:  read("FS882-8x6XL", 10, 50),
	}
	check()
	mutateContextFixture(t, state.SUPath, `WITH RECURSIVE n(i) AS (VALUES(1) UNION ALL SELECT i+1 FROM n WHERE i<15)
		INSERT INTO Report_SU(PlotNumber) SELECT 'A' FROM n`)
	pagingCheck := assertPlotLocationFacadeReadOnly(t, service.contexts)
	wire.PagingFirst = read("FS882-8x6XL", 0, 50)
	wire.PagingNext = read("FS882-8x6XL", 50, 50)
	pagingCheck()
	if wire.PagingFirst.TotalRows != 70 || len(wire.PagingFirst.Rows) != 50 || len(wire.PagingNext.Rows) != 20 {
		t.Fatal("independent paging fixture expectation changed")
	}
	paths := make(map[string]string)
	for _, path := range service.contexts.projects.sqlite.attachments {
		paths[path] = path
	}
	before := databaseBytes(t, paths)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	state, err := service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	wire.None = read("FS882-8x6XL", 0, 50)
	if !reflect.DeepEqual(before, databaseBytes(t, paths)) {
		t.Fatal("None review changed an original database")
	}
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "twoPageVegetationTableWire.json")
	if os.Getenv("VPRO_UPDATE_TWO_PAGE_VEGETATION_TABLE_WIRE") == "true" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
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
		t.Fatal("production fixture differs from independent owned service responses")
	}
}
