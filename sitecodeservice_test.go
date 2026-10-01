package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func TestSiteCodeCatalogueFrozenCellsDTOOrderAndDuplicates(t *testing.T) {
	service, err := NewSiteCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	p, err := listcatalog.DecodeProvenance(siteCodeProvenanceJSON)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("resources", "site-codes-fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := listcatalog.SnapshotRows(data, p)
	if err != nil {
		t.Fatal(err)
	}
	var actual []listcatalog.Choice
	duplicates := map[string][]SiteCodeChoice{}
	for _, list := range []string{"Exposure", "SiteDisturbance"} {
		var rows []SiteCodeChoice
		if list == "Exposure" {
			rows, err = service.ListExposureChoices()
		} else {
			rows, err = service.ListSiteDisturbanceChoices()
		}
		if err != nil {
			t.Fatal(err)
		}
		wantCount, wantSelectable := 127, 126
		if list == "Exposure" {
			wantCount, wantSelectable = 12, 11
		}
		if len(rows) != wantCount {
			t.Fatalf("%s: %d rows", list, len(rows))
		}
		selectable := 0
		for i, row := range rows {
			if row.RowID != strconv.Itoa(i+1) || row.ListName == nil || *row.ListName != list {
				t.Fatal("source ordinal/list identity changed")
			}
			if row.Selectable {
				selectable++
				if row.Diagnostic != "" {
					t.Fatal("selectable code has diagnostic")
				}
			} else if row.Diagnostic == "" {
				t.Fatal("unsupported code has no diagnostic")
			}
			if list == "Exposure" && (row.Note == nil || *row.Note != "") {
				t.Fatal("Exposure empty Note became NULL")
			}
			if list == "SiteDisturbance" && row.Note != nil {
				t.Fatal("disturbance NULL Note became empty")
			}
			if row.Code != nil && (*row.Code == "M.f" || *row.Code == "M.s") {
				duplicates[*row.Code] = append(duplicates[*row.Code], row)
			}
			row.Selectable, row.Diagnostic = false, ""
			actual = append(actual, listcatalog.Choice(row))
		}
		if selectable != wantSelectable {
			t.Fatalf("%s selectable %d, want %d", list, selectable, wantSelectable)
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("1390 typed metadata cells changed")
	}
	hash, err := listcatalog.TypedHash(actual)
	if err != nil || hash != p.TypedCellsSHA256 {
		t.Fatalf("typed checksum %s: %v", hash, err)
	}
	for _, code := range []string{"M.f", "M.s"} {
		rows := duplicates[code]
		if len(rows) != 2 || rows[0].RowID == rows[1].RowID || reflect.DeepEqual(rows[0].Description, rows[1].Description) {
			t.Fatalf("%s duplicate rows collapsed: %#v", code, rows)
		}
	}
	if reflect.TypeOf(SiteCodeChoice{}).NumField() != reflect.TypeOf(PlotQualityChoice{}).NumField() {
		t.Fatal("choice DTO shape changed")
	}
	for i := 0; i < reflect.TypeOf(SiteCodeChoice{}).NumField(); i++ {
		if !reflect.DeepEqual(reflect.TypeOf(SiteCodeChoice{}).Field(i), reflect.TypeOf(PlotQualityChoice{}).Field(i)) {
			t.Fatal("choice DTO fields/tags differ from quality")
		}
	}
	var shape map[string]any
	encoded, err := json.Marshal(SiteCodeChoice(actual[0]))
	if err != nil || json.Unmarshal(encoded, &shape) != nil || len(shape) != 13 {
		t.Fatal("choice JSON display shape changed")
	}
	if _, exists := shape["scientific"]; exists {
		t.Fatal("invented scientific metadata")
	}
}

func TestSiteCodeCatalogueFailClosedNonoverwritingAndReadonly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site-codes.db")
	if err := os.WriteFile(path, []byte("corrupt existing catalogue"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSiteCodeService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("corrupt catalogue accepted: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "corrupt existing catalogue" {
		t.Fatal("existing corrupt catalogue overwritten")
	}
	service, err := NewSiteCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.db.Exec(`UPDATE SiteCodeChoices SET Item='zz' WHERE ListName='Exposure'`); err == nil {
		t.Fatal("catalogue connection writable")
	}
	// Even an already-open catalogue cannot hide file corruption behind cached choices.
	file, err := os.OpenFile(service.path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteAt([]byte("broken"), 0)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("corruption probe: %v %v", writeErr, closeErr)
	}
	for _, list := range []func() ([]SiteCodeChoice, error){service.ListExposureChoices, service.ListSiteDisturbanceChoices} {
		if _, err := list(); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("open catalogue silently trusted corrupt file: %v", err)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListExposureChoices(); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed catalogue silently succeeded")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSiteCodeCatalogueConcurrentReadsAndClose(t *testing.T) {
	service, err := NewSiteCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 4; j++ {
				if _, err := service.ListExposureChoices(); err != nil && !strings.Contains(err.Error(), "closed") {
					t.Error(err)
				}
			}
		}()
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	group.Wait()
}
