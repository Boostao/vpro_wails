package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tableCSVFixture() ProjectMetadataTable {
	text := "  literal,\"quoted\"\r\nline\n\x00 \U0001f332  "
	integer := "-9223372036854775808"
	blob := "00ff0d0a"
	empty := ""
	real := math.Copysign(0, -1)
	return ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"Nullable", "TEXT"}, {"Empty", "TEXT"}, {"Text", "MEMO"},
			{"Integer", "LONG"}, {"Real", "DOUBLE"}, {"Blob", "OLE"}},
		Rows: []ProjectMetadataRow{{RowID: "9007199254740993", Cells: []ProjectMetadataCell{
			{Storage: "null"}, {Storage: "text", Text: &empty}, {Storage: "text", Text: &text},
			{Storage: "integer", Integer: &integer}, {Storage: "real", Real: &real}, {Storage: "blob", BlobHex: &blob},
		}}},
	}
}

func tableCSVClone(t *testing.T, document tableCSVDocument) tableCSVDocument {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var cloned tableCSVDocument
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func tableCSVReplaceData(document *tableCSVDocument, data string) {
	document.Data = []byte(data)
	hash := sha256.Sum256(document.Data)
	document.Manifest.SHA256 = hex.EncodeToString(hash[:])
}

func TestTableCSVRoundtripExactStorageAndIndependentDescriptions(t *testing.T) {
	table := tableCSVFixture()
	empty := ""
	descriptions := []TableCSVDescription{{"1", ProjectMetadataCell{Storage: "null"}},
		{"2", ProjectMetadataCell{Storage: "text", Text: &empty}}, {"3", ProjectMetadataCell{Storage: "text", Text: &empty}}}
	document, err := encodeTableCSV(context.Background(), "Sample_Env", table, descriptions)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(document.Data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"", `""`, `"  literal,\"quoted\"\r\nline\n\u0000 \U0001f332  "`, "-9223372036854775808", "-0", "00ff0d0a"}
	// Marshal emits the literal non-ASCII rune, not its Go escape spelling.
	want[2] = strings.ReplaceAll(want[2], `\U0001f332`, "\U0001f332")
	if len(records) != 2 || !reflect.DeepEqual(records[1], want) {
		t.Fatalf("CSV fields differ from independent literal expectation: %#v", records)
	}
	if document.Manifest.Version != 1 || document.Manifest.Table != "Sample_Env" ||
		len(document.Manifest.Descriptions) != 3 || document.Manifest.Descriptions[0].Value.Storage != "null" ||
		document.Manifest.Descriptions[1].Value.Storage != "text" || *document.Manifest.Descriptions[1].Value.Text != "" {
		t.Fatal("NULL/empty/duplicate Description candidates were collapsed")
	}
	decoded, err := decodeTableCSV(context.Background(), document)
	if err != nil || !reflect.DeepEqual(decoded, table) {
		t.Fatal("typed roundtrip differs:", decoded, err)
	}
	if math.Float64bits(*decoded.Rows[0].Cells[4].Real) != math.Float64bits(*table.Rows[0].Cells[4].Real) {
		t.Fatal("negative real zero lost its sign")
	}
	*table.Rows[0].Cells[2].Text = "caller changed"
	table.Columns[0].Name = "caller renamed"
	*descriptions[1].Value.Text = "caller changed description"
	if document.Manifest.Columns[0].Name != "Nullable" || *document.Manifest.Descriptions[1].Value.Text != "" {
		t.Fatal("encoded manifest aliases caller-owned schema/description values")
	}
	document.Manifest.Columns[0].Name = "later manifest change"
	document.Manifest.RowIDs[0] = "1"
	if decoded.Columns[0].Name != "Nullable" || decoded.Rows[0].RowID != "9007199254740993" {
		t.Fatal("decoded table aliases its manifest")
	}
}

func TestTableCSVEmptyTablesAndSQLitePhysicalRoundtrip(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE "Sample_Other" ("Name" TEXT,"Value");
		INSERT INTO "Sample_Other"(rowid,"Name","Value") VALUES
		(-9007199254740993,NULL,NULL),(2,'',-1),(3,'  Literal  ',9007199254740993),
		(4,'Comma,quote"',3.125),(5,'Blob',X'0001ff');`); err != nil {
		t.Fatal(err)
	}
	table, err := readSQLiteStorageRows(ctx, db, "main", "Sample_Other", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	document, err := encodeTableCSV(ctx, "Sample_Other", table, []TableCSVDescription{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTableCSV(ctx, document)
	if err != nil || !reflect.DeepEqual(decoded, table) {
		t.Fatal("independent SQLite physical rows lost storage/identity/order:", err)
	}
	if _, err := db.Exec(`DELETE FROM Sample_Other`); err != nil {
		t.Fatal(err)
	}
	empty := ProjectMetadataTable{Columns: table.Columns, Rows: []ProjectMetadataRow{}}
	document, err = encodeTableCSV(ctx, "Sample_Other", empty, []TableCSVDescription{})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = decodeTableCSV(ctx, document)
	if err != nil || !reflect.DeepEqual(decoded, empty) {
		t.Fatal("empty physical table did not roundtrip:", err)
	}
}

func TestTableCSVSingleColumnEmptyRecordsRemainPhysicalRows(t *testing.T) {
	empty, integer := "", "7"
	table := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"Value", ""}},
		Rows: []ProjectMetadataRow{
			{"-1", []ProjectMetadataCell{{Storage: "null"}}},
			{"2", []ProjectMetadataCell{{Storage: "blob", BlobHex: &empty}}},
			{"3", []ProjectMetadataCell{{Storage: "text", Text: &empty}}},
			{"4", []ProjectMetadataCell{{Storage: "integer", Integer: &integer}}},
			{"5", []ProjectMetadataCell{{Storage: "null"}}},
		},
	}
	document, err := encodeTableCSV(context.Background(), "PhysicalValues", table, []TableCSVDescription{})
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(document.Data)).ReadAll()
	if err != nil || !reflect.DeepEqual(records, [][]string{{"Value"}, {""}, {""}, {`""`}, {"7"}, {""}}) {
		t.Fatal("single-column empty records disappeared or reordered:", records, err)
	}
	decoded, err := decodeTableCSV(context.Background(), document)
	if err != nil || !reflect.DeepEqual(decoded, table) {
		t.Fatal("single-column NULL/empty-BLOB/text physical rows failed roundtrip:", decoded, err)
	}
}

func TestTableCSVRejectsCorruptionAmbiguityMalformedUnicodeAndPartialResults(t *testing.T) {
	document, err := encodeTableCSV(context.Background(), "Sample_Env", tableCSVFixture(), []TableCSVDescription{})
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*tableCSVDocument){
		"version":          func(d *tableCSVDocument) { d.Manifest.Version = 2 },
		"checksum":         func(d *tableCSVDocument) { d.Data = append(d.Data, 'x') },
		"nil descriptions": func(d *tableCSVDocument) { d.Manifest.Descriptions = nil },
		"columns":          func(d *tableCSVDocument) { d.Manifest.Columns[1].Name = d.Manifest.Columns[0].Name },
		"missing row ids":  func(d *tableCSVDocument) { d.Manifest.RowIDs = nil },
		"nonliteral id":    func(d *tableCSVDocument) { d.Manifest.RowIDs[0] = "+1" },
		"nil storage":      func(d *tableCSVDocument) { d.Manifest.Storage[0] = nil },
		"unknown storage":  func(d *tableCSVDocument) { d.Manifest.Storage[0][0] = "boolean" },
		"header drift": func(d *tableCSVDocument) {
			tableCSVReplaceData(d, strings.Replace(string(d.Data), "Nullable", "Wrong", 1))
		},
		"extra record": func(d *tableCSVDocument) { tableCSVReplaceData(d, string(d.Data)+",,,,,\n") },
		"missing record": func(d *tableCSVDocument) {
			tableCSVReplaceData(d, strings.SplitN(string(d.Data), "\n", 2)[0]+"\n")
		},
		"invalid utf8": func(d *tableCSVDocument) { tableCSVReplaceData(d, string(d.Data)+string([]byte{0xff})) },
		"invalid surrogate": func(d *tableCSVDocument) {
			records, _ := csv.NewReader(bytes.NewReader(d.Data)).ReadAll()
			records[1][2] = `"\ud800"`
			var output bytes.Buffer
			writer := csv.NewWriter(&output)
			if err := writer.WriteAll(records); err != nil {
				t.Fatal(err)
			}
			tableCSVReplaceData(d, output.String())
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			bad := tableCSVClone(t, document)
			mutate(&bad)
			result, err := decodeTableCSV(context.Background(), bad)
			if err == nil || !reflect.DeepEqual(result, ProjectMetadataTable{}) {
				t.Fatal("invalid interchange returned success/partial rows:", result, err)
			}
		})
	}
	for _, cell := range []ProjectMetadataCell{
		{Storage: "real", Real: func() *float64 { value := math.NaN(); return &value }()},
		{Storage: "integer", Integer: func() *string { value := "+1"; return &value }()},
		{Storage: "text", Text: func() *string { value := string([]byte{0xff}); return &value }()},
		{Storage: "null", Text: func() *string { value := ""; return &value }()},
	} {
		table := tableCSVFixture()
		table.Rows[0].Cells[0] = cell
		result, err := encodeTableCSV(context.Background(), "Sample_Env", table, []TableCSVDescription{})
		if err == nil || !reflect.DeepEqual(result, tableCSVDocument{}) {
			t.Fatal("malformed stored cell returned success/partial document:", err)
		}
	}
}

type tableCSVTestContext struct {
	context.Context
	calls int
	after int
}

func (ctx *tableCSVTestContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.after {
		return context.Canceled
	}
	return nil
}

func TestTableCSVCancellationReturnsNoPartialData(t *testing.T) {
	table := tableCSVFixture()
	second := table.Rows[0]
	second.RowID = "2"
	table.Rows = append(table.Rows, second)
	document, err := encodeTableCSV(context.Background(), "Sample_Env", table, []TableCSVDescription{})
	if err != nil {
		t.Fatal(err)
	}
	for _, after := range []int{1, 2, 3, 4} {
		result, err := encodeTableCSV(&tableCSVTestContext{Context: context.Background(), after: after}, "Sample_Env", table, []TableCSVDescription{})
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, tableCSVDocument{}) {
			t.Fatal("cancelled encode returned success/partial document:", err)
		}
		decoded, err := decodeTableCSV(&tableCSVTestContext{Context: context.Background(), after: after}, document)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(decoded, ProjectMetadataTable{}) {
			t.Fatal("cancelled decode returned success/partial table:", err)
		}
	}
}
