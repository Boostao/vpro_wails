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

func TestTwoPageProjectMetadataProductionWire(t *testing.T) {
	service, state, db := twoPageProjectMetadataFixture(t, true)
	read := func(offset, limit int) *TwoPageProjectMetadataReview {
		t.Helper()
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		configBefore, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", offset, limit)
		if err != nil {
			t.Fatal(err)
		}
		configAfter, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(configBefore, configAfter) ||
			!reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
			t.Fatal("production wire changed data/config/history", err)
		}
		got.ContextID, got.ProjectPath = "wire-owned", `C:\fixture\Sample.db`
		if got.SU != "None" {
			got.SUPath = `C:\fixture\Report.db`
		}
		return got
	}
	type wireFixture struct {
		Default     *TwoPageProjectMetadataReview `json:"default"`
		Page        *TwoPageProjectMetadataReview `json:"page"`
		End         *TwoPageProjectMetadataReview `json:"end"`
		EmptyID     *TwoPageProjectMetadataReview `json:"emptyId"`
		None        *TwoPageProjectMetadataReview `json:"none"`
		PagingFirst *TwoPageProjectMetadataReview `json:"pagingFirst"`
		PagingNext  *TwoPageProjectMetadataReview `json:"pagingNext"`
	}
	wire := wireFixture{Default: read(0, 50), Page: read(1, 1), End: read(100000, 50)}
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	wire.EmptyID = read(0, 50)
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='Alpha' WHERE PlotNumber='108050';
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<67)
		INSERT INTO Sample_Metadata(ProjectID,ID) SELECT 'ALPHA ',1000+i FROM n`); err != nil {
		t.Fatal(err)
	}
	wire.PagingFirst, wire.PagingNext = read(0, 50), read(50, 50)
	if wire.PagingFirst.TotalRows != 70 || len(wire.PagingFirst.MetadataOriginal.Rows) != 50 ||
		len(wire.PagingNext.MetadataOriginal.Rows) != 20 {
		t.Fatal("independent50/20 metadata page expectations changed")
	}
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	var err error
	state, err = service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	wire.None = read(0, 50)
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "twoPageProjectMetadataWire.json")
	if os.Getenv("VPRO_UPDATE_TWO_PAGE_PROJECT_METADATA_WIRE") == "true" {
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
		t.Fatal("metadata wire differs from actual independently owned Go responses")
	}
}
