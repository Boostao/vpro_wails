package listcatalog

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func soilFixture(t *testing.T) ([]byte, Provenance, []Choice) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "soil-codes-fixture.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	profile := SoilProfile()
	p := Provenance{Version: 1, Exporter: profile.Exporter, SourceSHA256: profile.SourceSHA256,
		SnapshotSHA256: profile.SnapshotSHA256, DatabaseSHA256: strings.Repeat("0", 64), TypedCellsSHA256: strings.Repeat("0", 64),
		Rows: 101, Cells: 1010, SourceTable: "USysTableOfLists", SQL: profile.SQL, SourceSchema: ExpectedSchema(), ItemOrderEncoding: ItemOrderEncoding}
	rows, err := SoilSnapshotRows(data, p)
	if err != nil {
		t.Fatal(err)
	}
	p.TypedCellsSHA256, err = TypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	return data, p, rows
}

func TestSoilDAO1010CellsAndStorageRoundtrip(t *testing.T) {
	_, p, expected := soilFixture(t)
	db := memoryDB(t)
	if err := ImportRows(db, expected, p, SoilProfile()); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadChoicesFor(db, "", SoilProfile())
	if err != nil || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("1010 DAO cells changed: %v", err)
	}
	for _, list := range SoilProfile().Lists {
		rows, err := ReadChoicesFor(db, list.Name, SoilProfile())
		if err != nil || len(rows) != list.Rows {
			t.Fatal(list.Name, err)
		}
		if rows[0].Code == nil || *rows[0].Code != "" {
			t.Fatal("native empty row lost/converted to NULL")
		}
	}
	if _, err := ReadChoicesFor(db, "SoilClassSubGroup", SoilProfile()); err == nil {
		t.Fatal("invented casing alias accepted")
	}
}

func TestSoilSnapshotFailClosedShapeCellsAndQueries(t *testing.T) {
	data, p, _ := soilFixture(t)
	if _, err := SoilSnapshotRows(append(append([]byte{}, data...), 0), p); err == nil {
		t.Fatal("checksum failure accepted")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ before, after string }{
		{`"Readonly":true`, `"Readonly":false`},
		{`"NativeGUIStarted":false`, `"NativeGUIStarted":true`},
		{`"DAOHandlesClosed":true`, `"DAOHandlesClosed":false`},
		{`"Count":39`, `"Count":38`},
		{`[ListName]='SoilClassSubgroup'`, `[ListName]='SoilClassSubGroup'`},
		{SoilNativeTypedCellsSHA256, strings.Repeat("0", 64)},
	} {
		t.Run(change.before, func(t *testing.T) {
			changed := bytes.Replace(raw, []byte(change.before), []byte(change.after), 1)
			if bytes.Equal(changed, raw) {
				t.Fatal("test mutation absent")
			}
			var out bytes.Buffer
			writer := gzip.NewWriter(&out)
			if _, err := writer.Write(changed); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := decodeSoilSnapshot(out.Bytes(), p); err == nil {
				t.Fatal("source shape/query/cell mutation accepted")
			}
		})
	}
}

func TestSoilStorageNullableDuplicateFractionalSignedZeroAndRollback(t *testing.T) {
	_, p, rows := soilFixture(t)
	testDAOStorage(t, SoilProfile(), p, rows)
}

func testDAOStorage(t *testing.T, profile Profile, p Provenance, rows []Choice) {
	t.Helper()
	duplicate, empty := "same", ""
	rows[1].Code, rows[2].Code = &duplicate, &duplicate
	rows[1].Note, rows[2].Note = &empty, nil
	rows[3].Code, rows[3].ItemOrder, rows[4].Validate = nil, nil, nil
	flag := true
	rows[4].Flag = &flag
	zero, fraction := math.Copysign(0, -1), 2.5
	rows[1].ItemOrder, rows[2].ItemOrder = &zero, &fraction
	rows[4].ItemOrder = &fraction
	rows[1].Flag, rows[2].Flag = nil, new(bool)
	var err error
	p.TypedCellsSHA256, err = TypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	db := memoryDB(t)
	if err := ImportRows(db, rows, p, profile); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadChoicesFor(db, "", profile)
	if err != nil || !reflect.DeepEqual(actual, rows) || math.Float64bits(*actual[1].ItemOrder) != math.Float64bits(zero) {
		t.Fatal("typed nullable/duplicate/double loss", err)
	}
	for _, kind := range []string{"identity", "member", "nonfinite"} {
		bad := append([]Choice{}, rows...)
		switch kind {
		case "identity":
			bad[2].RowID = bad[1].RowID
		case "member":
			bad[2].ListName = &duplicate
		default:
			value := math.Inf(1)
			bad[2].ItemOrder = &value
		}
		badP := p
		badP.TypedCellsSHA256, err = TypedHash(bad)
		if err != nil {
			if kind != "nonfinite" {
				t.Fatal(err)
			}
			continue
		}
		other := memoryDB(t)
		if err := ImportRows(other, bad, badP, profile); err == nil {
			t.Fatal("invalid import accepted")
		}
		var count int
		if err := other.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('SiteCodeChoices','SiteCodeProvenance')`).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed import not rolled back")
		}
	}
}

func TestSoilScalarAndNativeHashEncoding(t *testing.T) {
	value, clr, bits := "0", "System.Double", "0000000000000080"
	cell := daoNativeCell{CLRType: &clr, DAOType: 7, IEEE754LittleEndianHex: &bits, Value: &value}
	if err := validateDAOScalar(cell); err != nil {
		t.Fatal("signed-zero native bits rejected", err)
	}
	for _, bad := range []daoNativeCell{{DAOType: 7, Value: &value}, {IsNull: true, CLRType: &clr}, {DAOType: 7, Value: &value, CLRType: &clr, IEEE754LittleEndianHex: &value}} {
		if err := validateDAOScalar(bad); err == nil {
			t.Fatal("invalid scalar accepted")
		}
	}
	if _, err := daoNativeHash([][]daoNativeCell{{cell}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(Choice{})
	if err != nil || !bytes.Contains(encoded, []byte(`"diagnostic":""`)) {
		t.Fatal("DTO defaults changed")
	}
}
