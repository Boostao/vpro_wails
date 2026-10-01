package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

func parentCatalogueExpected(t *testing.T, p listcatalog.Provenance) []listcatalog.Choice {
	t.Helper()
	var inputs [][]byte
	for _, name := range []string{"parent-codes-fixture.json", "parent-codes-source-provenance.json",
		"parent-codes-fixture-manifest.json", "geology-codes-fixture.json.gz"} {
		data, err := os.ReadFile(filepath.Join("resources", name))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, data)
	}
	rows, err := listcatalog.ParentSnapshotRows(inputs[0], inputs[1], inputs[2], inputs[3], p)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestParentCatalogueAll16Lists3470CellsAnd13Properties(t *testing.T) {
	s, err := NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var actual []listcatalog.Choice
	selectable := 0
	for _, list := range listcatalog.ParentProfile().Lists {
		maximum, exists := parentCodeListMaximum(list.Name)
		if !exists || maximum <= 0 {
			t.Fatal("registry missing bound", list.Name)
		}
		rows, err := s.ListChoices(list.Name)
		if err != nil || len(rows) != list.Rows {
			t.Fatal(list.Name, err)
		}
		for i, row := range rows {
			if row.RowID != strconv.Itoa(i+1) || row.ListName == nil || *row.ListName != list.Name {
				t.Fatal("ordinal/casing lost")
			}
			if row.Selectable {
				selectable++
				if row.Diagnostic != "" {
					t.Fatal("selectable diagnostic")
				}
			} else if row.Diagnostic == "" {
				t.Fatal("NULL/empty code lacks diagnostic")
			}
			row.Selectable, row.Diagnostic = false, ""
			actual = append(actual, listcatalog.Choice(row))
		}
	}
	if selectable != 330 || !reflect.DeepEqual(actual, parentCatalogueExpected(t, s.provenance)) {
		t.Fatal("NULL/empty/full 3470 metadata cells changed")
	}
	hash, err := listcatalog.TypedHash(actual)
	if err != nil || hash != s.provenance.TypedCellsSHA256 {
		t.Fatal("typed metadata hash changed")
	}
	parent, soil := reflect.TypeOf(ParentCodeChoice{}), reflect.TypeOf(SoilCodeChoice{})
	if parent.NumField() != 13 {
		t.Fatal("DTO count")
	}
	for i := 0; i < 13; i++ {
		if !reflect.DeepEqual(parent.Field(i), soil.Field(i)) {
			t.Fatal("DTO shape/types/tags changed")
		}
	}
	for _, unsupported := range []string{"", "bedrocktype", "HydroGeoSystem", "SoilClassGroup", "BECSiteUnit", "WaterSource' OR 1=1"} {
		if rows, err := s.ListChoices(unsupported); err == nil || rows != nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatal("unsupported list hidden", unsupported, err)
		}
	}
}

func TestParentCatalogueChoiceDiagnosticsPreserveCells(t *testing.T) {
	for _, list := range listcatalog.ParentProfile().Lists {
		maximum, exists := parentCodeListMaximum(list.Name)
		if !exists {
			t.Fatal("unsupported profile list")
		}
		for _, code := range []*string{nil, qualityString(""), qualityString(strings.Repeat("x", maximum+1)),
			qualityString(string([]byte{0xff})), qualityString("X")} {
			source := listcatalog.Choice{RowID: "exact", Code: code, ListName: qualityString(list.Name)}
			row := parentCodeChoice(source, maximum)
			valid := code != nil && *code == "X"
			if row.Selectable != valid || (row.Diagnostic == "") != valid ||
				!reflect.DeepEqual(row.Code, code) || row.RowID != source.RowID {
				t.Fatal("code repaired/truncated or diagnostics wrong", list.Name)
			}
		}
	}
}

func TestParentCatalogueNonoverwriteChecksumWarmRetryAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent-codes.db")
	corrupt := []byte("owned existing corrupt catalogue")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewParentCodeService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatal("existing catalogue accepted/replaced", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, corrupt) {
		t.Fatal("existing bytes changed")
	}
	s, err := NewParentCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := readonlyCatalogueFixture(t, s.path).Exec(`UPDATE SiteCodeChoices SET Item='changed'`); err == nil {
		t.Fatal("reference writable")
	}
	if _, err := s.ListChoices("HumusForm"); err != nil {
		t.Fatal(err)
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
	for _, list := range listcatalog.ParentProfile().Lists {
		if rows, err := s.ListChoices(list.Name); err == nil || rows != nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatal("cached catalogue hid corruption", err)
		}
	}
	file, err = os.OpenFile(s.path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt(parentCodeDatabase, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, list := range listcatalog.ParentProfile().Lists {
		if rows, err := s.ListChoices(list.Name); err != nil || len(rows) != list.Rows {
			t.Fatal("exact byte restoration retry failed", err)
		}
	}
	old := s.provenance
	s.provenance.TypedCellsSHA256 = strings.Repeat("0", 64)
	if _, err := s.ListChoices("WaterSource"); err == nil {
		t.Fatal("bad stored fidelity hidden")
	}
	s.provenance = old
	s.provenance.Rows--
	if _, err := s.ListChoices("WaterSource"); err == nil {
		t.Fatal("bad provenance count hidden")
	}
	s.provenance = old
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListChoices("WaterSource"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed catalogue succeeded")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewParentCodeService(filepath.Dir(s.path))
	if err != nil {
		t.Fatal("restart", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(s.path)
	if err != nil || !bytes.Equal(data, parentCodeDatabase) {
		t.Fatal("readonly/restart changed bytes")
	}
}
