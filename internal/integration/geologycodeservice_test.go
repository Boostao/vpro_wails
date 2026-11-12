package integration

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
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

func TestGeologyCatalogue870CellsAnd13FieldShape(t *testing.T) {
	service, err := references.NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	fixture, err := os.ReadFile(filepath.Join("resources", "geology-codes-fixture.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := listcatalog.GeologySnapshotRows(fixture, service.Provenance)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := service.ListBedrockChoices(context.Background())
	if err != nil || len(rows) != 87 {
		t.Fatal("catalogue count", err)
	}
	var actual []listcatalog.Choice
	selectable := 0
	for i, row := range rows {
		if row.RowID != strconv.Itoa(i+1) || row.ListName == nil || *row.ListName != "BedrockType" {
			t.Fatal("ordinal/casing differs")
		}
		if row.Selectable {
			selectable++
			if row.Diagnostic != "" {
				t.Fatal("selectable diagnostic")
			}
		} else if row.Diagnostic == "" {
			t.Fatal("unselectable lacks diagnostic")
		}
		row.Selectable, row.Diagnostic = false, ""
		actual = append(actual, listcatalog.Choice(row))
	}
	if selectable != 86 || rows[0].Code == nil || *rows[0].Code != "" || rows[0].Selectable ||
		!reflect.DeepEqual(actual, expected) {
		t.Fatal("native metadata/blank row changed")
	}
	hash, err := listcatalog.TypedHash(actual)
	if err != nil || hash != service.Provenance.TypedCellsSHA256 {
		t.Fatal("storage fidelity hash changed")
	}
	geology, soil := reflect.TypeOf(references.GeologyCodeChoice{}), reflect.TypeOf(references.SoilCodeChoice{})
	if geology.NumField() != 13 {
		t.Fatal("DTO field count")
	}
	for i := 0; i < 13; i++ {
		if !reflect.DeepEqual(geology.Field(i), soil.Field(i)) {
			t.Fatal("DTO type/tag mismatch")
		}
	}
	encoded, err := json.Marshal(references.GeologyCodeChoice{})
	var shape map[string]any
	if err != nil || json.Unmarshal(encoded, &shape) != nil || len(shape) != 13 ||
		shape["code"] != nil || shape["diagnostic"] != "" {
		t.Fatal("nullable JSON shape differs")
	}
}

func TestGeologyCatalogueNonoverwriteReadonlyWarmRetryAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geology-codes.db")
	corrupt := []byte("existing corrupt catalogue")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := references.NewGeologyCodeService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatal("existing catalogue replaced/accepted", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, corrupt) {
		t.Fatal("existing bytes changed")
	}
	s, err := references.NewGeologyCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := vprotest.ReadonlyCatalogueFixture(t, s.Path).Exec(`UPDATE SiteCodeChoices SET Item='changed'`); err == nil {
		t.Fatal("reference writable")
	}
	if _, err := s.ListBedrockChoices(context.Background()); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(s.Path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("broken"), 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListBedrockChoices(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum") || rows != nil {
		t.Fatal("checksum failure hidden", err)
	}
	file, err = os.OpenFile(s.Path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt(references.GeologyCodeDatabase, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListBedrockChoices(context.Background()); err != nil || len(rows) != 87 {
		t.Fatal("exact restoration warm retry failed", err)
	}
	old := s.Provenance
	s.Provenance.TypedCellsSHA256 = strings.Repeat("0", 64)
	if _, err := s.ListBedrockChoices(context.Background()); err == nil {
		t.Fatal("stored fidelity failure hidden")
	}
	s.Provenance = old
	s.Provenance.Rows--
	if _, err := s.ListBedrockChoices(context.Background()); err == nil {
		t.Fatal("invalid provenance hidden")
	}
	s.Provenance = old
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := s.ListBedrockChoices(context.Background()); err != nil && !strings.Contains(err.Error(), "closed") {
				t.Error(err)
			}
		}()
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	group.Wait()
	if _, err := s.ListBedrockChoices(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed catalogue succeeded")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := references.NewGeologyCodeService(filepath.Dir(s.Path))
	if err != nil {
		t.Fatal("restart", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(s.Path)
	if err != nil || !bytes.Equal(data, references.GeologyCodeDatabase) {
		t.Fatal("readonly service changed bytes")
	}
}
