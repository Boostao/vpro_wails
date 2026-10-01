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

	"context"
	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func TestRegionCatalogueAll1640CellsShapeAndNativeOrder(t *testing.T) {
	s, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	data, err := os.ReadFile(filepath.Join("resources", "region-codes-fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := listcatalog.DecodeProvenanceFor(regionCodeProvenanceJSON, listcatalog.RegionProfile())
	if err != nil {
		t.Fatal(err)
	}
	expected, err := listcatalog.RegionSnapshotRows(data, p)
	if err != nil {
		t.Fatal(err)
	}
	var actual []listcatalog.Choice
	for _, list := range []string{"Ecosection", "Region"} {
		read := s.ListEcosectionChoices
		count := 137
		if list == "Region" {
			read = s.ListRegionChoices
			count = 27
		}
		rows, err := read(context.Background())
		if err != nil || len(rows) != count {
			t.Fatalf("%s: %d rows %v", list, len(rows), err)
		}
		selectable := 0
		for i, row := range rows {
			if row.RowID != strconv.Itoa(i+1) || row.ListName == nil || *row.ListName != list || row.Note != nil {
				t.Fatal("ordinal/case/NULL metadata changed")
			}
			if row.Selectable {
				selectable++
				if row.Diagnostic != "" {
					t.Fatal("selectable row diagnostic")
				}
			} else if row.Diagnostic == "" {
				t.Fatal("unselectable row lacks diagnostic")
			}
			row.Selectable, row.Diagnostic = false, ""
			actual = append(actual, listcatalog.Choice(row))
		}
		if selectable != count-1 || rows[0].Code != nil {
			t.Fatal("NULL row lost or selectable")
		}
		if list == "Ecosection" {
			for i, want := range []string{"ALR", "BAU", "BBT", "BOV"} {
				if rows[i+1].Code == nil || *rows[i+1].Code != want {
					t.Fatal("unsorted native query order changed")
				}
			}
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("1640 typed cells changed")
	}
	hash, err := listcatalog.TypedHash(actual)
	if err != nil || hash != p.TypedCellsSHA256 {
		t.Fatal("typed metadata checksum mismatch")
	}
	region, site := reflect.TypeOf(RegionCodeChoice{}), reflect.TypeOf(SiteCodeChoice{})
	if region.NumField() != 13 {
		t.Fatal("choice field count")
	}
	for i := 0; i < 13; i++ {
		if !reflect.DeepEqual(region.Field(i), site.Field(i)) {
			t.Fatal("choice JSON/types differ")
		}
	}
	encoded, err := json.Marshal(RegionCodeChoice{})
	var shape map[string]any
	if err != nil || json.Unmarshal(encoded, &shape) != nil || len(shape) != 13 || shape["diagnostic"] != "" {
		t.Fatal("non-null diagnostic/JSON shape changed")
	}
}

func TestRegionCatalogueReadonlyNonoverwritingCorruptionAndRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "region-codes.db")
	if err := os.WriteFile(path, []byte("owned corrupt catalogue"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRegionCodeService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatal("existing catalogue overwritten/accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "owned corrupt catalogue" {
		t.Fatal("owned bytes replaced")
	}
	s, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := readonlyCatalogueFixture(t, s.path).Exec(`UPDATE SiteCodeChoices SET Item='zz'`); err == nil {
		t.Fatal("read-only catalogue writable")
	}
	file, err := os.OpenFile(s.path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("broken"), 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(context.Context) ([]RegionCodeChoice, error){s.ListRegionChoices, s.ListEcosectionChoices} {
		if _, err := read(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("corruption hidden: %v", err)
		}
	}
	file, err = os.OpenFile(s.path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt(regionCodeDatabase, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListEcosectionChoices(context.Background()); err != nil || len(rows) != 137 {
		t.Fatalf("restored bytes retry failed %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListRegionChoices(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed catalogue succeeded")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRegionCatalogueConcurrentReadCloseAndProvenance(t *testing.T) {
	s, err := NewRegionCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := s.provenance
	p.Rows--
	if err := listcatalog.ValidateDatabaseFor(readonlyCatalogueFixture(t, s.path), p, listcatalog.RegionProfile()); err == nil {
		t.Fatal("invalid row provenance accepted")
	}
	p = s.provenance
	p.TypedCellsSHA256 = strings.Repeat("0", 64)
	if err := listcatalog.ValidateDatabaseFor(readonlyCatalogueFixture(t, s.path), p, listcatalog.RegionProfile()); err == nil {
		t.Fatal("invalid stored provenance accepted")
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 3; j++ {
				if _, err := s.ListRegionChoices(context.Background()); err != nil && !strings.Contains(err.Error(), "closed") {
					t.Error(err)
				}
			}
		}()
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	group.Wait()
}
