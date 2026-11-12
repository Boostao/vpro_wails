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

func TestSoilCatalogueAll1010CellsAndExact13FieldShape(t *testing.T) {
	service, err := references.NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	fixture, err := os.ReadFile(filepath.Join("resources", "soil-codes-fixture.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := listcatalog.DecodeProvenanceFor(references.SoilCodeProvenanceJSON, listcatalog.SoilBundledProfile2026())
	if err != nil {
		t.Fatal(err)
	}
	expected, _, err := listcatalog.SoilBundled2026Rows(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var actual []listcatalog.Choice
	for _, test := range []struct {
		list  string
		count int
		read  func(context.Context) ([]references.SoilCodeChoice, error)
	}{
		{"SoilClassGroup", 39, service.ListGreatGroupChoices}, {"SoilClassSubgroup", 62, service.ListSubgroupChoices},
	} {
		rows, err := test.read(context.Background())
		if err != nil || len(rows) != test.count {
			t.Fatalf("%s: %d %v", test.list, len(rows), err)
		}
		selectable := 0
		for i, row := range rows {
			if row.RowID != strconv.Itoa(i+1) || row.ListName == nil || *row.ListName != test.list {
				t.Fatal("native ordinal/member changed")
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
		if selectable != test.count-1 || rows[0].Code == nil || *rows[0].Code != "" || rows[0].Selectable {
			t.Fatal("empty native row lost/selectable")
		}
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("all1010 cells differ")
	}
	hash, err := listcatalog.TypedHash(actual)
	if err != nil || hash != p.TypedCellsSHA256 {
		t.Fatal("storage fidelity hash changed")
	}
	soil, region := reflect.TypeOf(references.SoilCodeChoice{}), reflect.TypeOf(references.RegionCodeChoice{})
	if soil.NumField() != 13 {
		t.Fatal("DTO field count")
	}
	for i := 0; i < 13; i++ {
		if !reflect.DeepEqual(soil.Field(i), region.Field(i)) {
			t.Fatal("exact13-field DTO mismatch")
		}
	}
	encoded, err := json.Marshal(references.SoilCodeChoice{})
	var shape map[string]any
	if err != nil || json.Unmarshal(encoded, &shape) != nil || len(shape) != 13 || shape["diagnostic"] != "" || shape["code"] != nil {
		t.Fatal("nullable public JSON shape mismatch")
	}
}

func TestSoilCatalogueReadonlyNonoverwriteCorruptionWarmRetryRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "soil-codes.db")
	corrupt := []byte("owned corrupt catalogue")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := references.NewSoilCodeService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatal("existing corrupt catalogue accepted/replaced")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, corrupt) {
		t.Fatal("existing bytes changed")
	}
	service, err := references.NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := vprotest.ReadonlyCatalogueFixture(t, service.Path).Exec(`UPDATE SiteCodeChoices SET Item='changed'`); err == nil {
		t.Fatal("reference writable")
	}
	file, err := os.OpenFile(service.Path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("broken"), 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(context.Context) ([]references.SoilCodeChoice, error){service.ListGreatGroupChoices, service.ListSubgroupChoices} {
		if _, err := read(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatal("corruption hidden", err)
		}
	}
	file, err = os.OpenFile(service.Path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt(references.SoilCodeDatabase, 0); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(context.Context) ([]references.SoilCodeChoice, error){service.ListGreatGroupChoices, service.ListSubgroupChoices} {
		if rows, err := read(context.Background()); err != nil || len(rows) == 0 {
			t.Fatal("exact byte restoration warm retry failed", err)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListGreatGroupChoices(context.Background()); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("closed list success")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := references.NewSoilCodeService(filepath.Dir(service.Path))
	if err != nil {
		t.Fatal("restart", err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(service.Path)
	if err != nil || !bytes.Equal(data, references.SoilCodeDatabase) {
		t.Fatal("readonly/restart changed packaged bytes")
	}
}

func TestSoilCataloguePerCallProvenanceFailureAndConcurrentClose(t *testing.T) {
	service, err := references.NewSoilCodeService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	old := service.Provenance
	service.Provenance.TypedCellsSHA256 = strings.Repeat("0", 64)
	if _, err := service.ListSubgroupChoices(context.Background()); err == nil {
		t.Fatal("bad stored fidelity hidden")
	}
	service.Provenance = old
	service.Provenance.Rows--
	if _, err := service.ListGreatGroupChoices(context.Background()); err == nil {
		t.Fatal("bad count hidden")
	}
	service.Provenance = old
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 3; j++ {
				if _, err := service.ListSubgroupChoices(context.Background()); err != nil && !strings.Contains(err.Error(), "closed") {
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
