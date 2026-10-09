package listcatalog

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func geologyFixture(t *testing.T) ([]byte, Provenance, []Choice) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "geology-codes-fixture.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(filepath.Join("..", "..", "resources", "geology-codes-provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodeProvenanceFor(encoded, GeologyProfile())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := GeologySnapshotRows(data, p)
	if err != nil {
		t.Fatal(err)
	}
	return data, p, rows
}

func TestGeologyDAO870CellsAndStorageRoundtrip(t *testing.T) {
	_, p, expected := geologyFixture(t)
	db := memoryDB(t)
	if err := ImportRows(db, expected, p, GeologyProfile()); err != nil {
		t.Fatal(err)
	}
	actual, err := ReadChoicesFor(db, "BedrockType", GeologyProfile())
	if err != nil || len(actual) != 87 || !reflect.DeepEqual(actual, expected) {
		t.Fatal("870 cells/native order differ", err)
	}
	hash, err := TypedHash(actual)
	if err != nil || hash != p.TypedCellsSHA256 {
		t.Fatal("normalized typed hash differs", err)
	}
	if actual[0].Code == nil || *actual[0].Code != "" || actual[0].ValidateLoops != nil {
		t.Fatal("NULL/empty metadata lost")
	}
	if _, err := ReadChoicesFor(db, "bedrocktype", GeologyProfile()); err == nil {
		t.Fatal("invented list casing alias accepted")
	}
}

func TestGeologySnapshotRejectsSourceSchemaQueryCellsAndHashes(t *testing.T) {
	data, p, _ := geologyFixture(t)
	if _, err := GeologySnapshotRows(append(append([]byte{}, data...), 0), p); err == nil {
		t.Fatal("checksum failure accepted")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var decoded bytes.Buffer
	if _, err := decoded.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ before, after string }{
		{`"Readonly":true`, `"Readonly":false`},
		{`"NativeGUIStarted":false`, `"NativeGUIStarted":true`},
		{`"DAOHandlesClosed":true`, `"DAOHandlesClosed":false`},
		{`"Count":87`, `"Count":86`},
		{`[ListName]='BedrockType'`, `[ListName]='bedrocktype'`},
		{`"Size":8`, `"Size":4`},
		{`"code":""`, `"code":null`},
		{GeologyNativeTypedCellsSHA256, strings.Repeat("0", 64)},
		{SourceSHA256, strings.Repeat("0", 64)},
		{`652409eb30260d3793bc1246ebebb480cd405be2b526bf377e739d13ff9ccb47`, strings.Repeat("0", 64)},
		{`6299e1fabbe1a72efcc667dd541b4a996e1a95b2ba831f83182504de763f2221`, strings.Repeat("0", 64)},
	} {
		t.Run(change.before, func(t *testing.T) {
			changed := bytes.Replace(decoded.Bytes(), []byte(change.before), []byte(change.after), 1)
			if bytes.Equal(changed, decoded.Bytes()) {
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
			if _, err := decodeGeologySnapshot(out.Bytes(), p); err == nil {
				t.Fatal("source/query/cell/hash mutation accepted")
			}
		})
	}
}

func TestGeologyStorageNullableDuplicatesFractionalSignedZeroAndRollback(t *testing.T) {
	_, p, rows := geologyFixture(t)
	testDAOStorage(t, GeologyProfile(), p, rows)
}
