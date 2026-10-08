package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func tableCSVBundleFixture(t *testing.T) tableCSVDocument {
	t.Helper()
	empty := ""
	text := "  duplicate\r\n\x00\U0001f332  "
	negativeZero := math.Copysign(0, -1)
	integer := "9223372036854775807"
	blob := "00ff"
	descriptions := []TableCSVDescription{
		{"-9223372036854775808", ProjectMetadataCell{Storage: "null"}},
		{"9007199254740993", ProjectMetadataCell{Storage: "null"}},
		{"2", ProjectMetadataCell{Storage: "text", Text: &empty}},
		{"3", ProjectMetadataCell{Storage: "text", Text: &empty}},
		{"4", ProjectMetadataCell{Storage: "text", Text: &text}},
		{"5", ProjectMetadataCell{Storage: "text", Text: &text}},
		{"6", ProjectMetadataCell{Storage: "real", Real: &negativeZero}},
		{"7", ProjectMetadataCell{Storage: "integer", Integer: &integer}},
		{"8", ProjectMetadataCell{Storage: "blob", BlobHex: &blob}},
		{"9", ProjectMetadataCell{Storage: "blob", BlobHex: &empty}},
	}
	document, err := encodeTableCSV(context.Background(), "Literal_\U0001f332", tableCSVFixture(), descriptions)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

type tableCSVBundleTestMember struct {
	name   string
	data   []byte
	method uint16
	mode   os.FileMode
}

func tableCSVBundleTestArchive(t *testing.T, members ...tableCSVBundleTestMember) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, member := range members {
		header := &zip.FileHeader{Name: member.name, Method: member.method}
		if member.mode != 0 {
			header.SetMode(member.mode)
		}
		destination, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := destination.Write(member.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), buffer.Bytes()...)
}

func tableCSVBundleTestManifest(t *testing.T, document tableCSVDocument) []byte {
	t.Helper()
	raw, err := json.Marshal(document.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func tableCSVBundleAssertRejected(t *testing.T, data []byte, budget int64) {
	t.Helper()
	document, err := decodeTableCSVBundle(context.Background(), data, budget)
	if err == nil || err.Error() == "" || !reflect.DeepEqual(document, tableCSVDocument{}) {
		t.Fatalf("expected explicit error and zero output, got %#v, %v", document, err)
	}
}

func TestTableCSVBundleDeterministicIndependentZIPAndExactRoundtrip(t *testing.T) {
	ctx := context.Background()
	document := tableCSVBundleFixture(t)
	encoded, err := encodeTableCSVBundle(ctx, document)
	if err != nil {
		t.Fatal(err)
	}
	second, err := encodeTableCSVBundle(ctx, document)
	if err != nil || !bytes.Equal(encoded, second) {
		t.Fatal("bundle bytes are not deterministic:", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil || len(archive.File) != 2 {
		t.Fatal("independent ZIP reader:", err)
	}
	wantNames := []string{"table.csv", "manifest.json"}
	wantBytes := [][]byte{document.Data, tableCSVBundleTestManifest(t, document)}
	for index, member := range archive.File {
		if member.Name != wantNames[index] || member.Method != zip.Store ||
			member.Flags != 0 || !member.Mode().IsRegular() || member.Mode().Perm() != 0600 ||
			member.CompressedSize64 != uint64(len(wantBytes[index])) ||
			member.UncompressedSize64 != uint64(len(wantBytes[index])) ||
			member.ModifiedDate != 33 || member.ModifiedTime != 0 ||
			member.ReaderVersion != 20 || len(member.Extra) != 0 || member.Comment != "" {
			t.Fatalf("unexpected independent member metadata: %#v", member.FileHeader)
		}
		source, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(source)
		closeErr := source.Close()
		if err != nil || closeErr != nil || !bytes.Equal(data, wantBytes[index]) {
			t.Fatal("independent member content differs:", err, closeErr)
		}
	}
	decoded, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
	if err != nil || !reflect.DeepEqual(decoded, document) {
		t.Fatal("exact document roundtrip:", err)
	}
	table, err := decodeTableCSV(ctx, decoded)
	if err != nil || !reflect.DeepEqual(table, tableCSVFixture()) {
		t.Fatal("typed table roundtrip:", err)
	}
	if !math.Signbit(*table.Rows[0].Cells[4].Real) ||
		!math.Signbit(*decoded.Manifest.Descriptions[6].Value.Real) {
		t.Fatal("real negative zero lost its sign")
	}
	tableCSVBundleAssertRejected(t, encoded, int64(len(encoded)-1))
	tableCSVBundleAssertRejected(t, encoded, 0)
	tableCSVBundleAssertRejected(t, encoded, -1)
}

func TestTableCSVBundleDetachedInputsOutputsAndConventionalDescriptorZIP(t *testing.T) {
	ctx := context.Background()
	document := tableCSVBundleFixture(t)
	original := tableCSVClone(t, document)
	encoded, err := encodeTableCSVBundle(ctx, document)
	if err != nil {
		t.Fatal(err)
	}
	document.Data[0] = 'X'
	document.Manifest.Columns[0].Name = "mutated"
	document.Manifest.Storage[0][0] = "blob"
	document.Manifest.RowIDs[0] = "0"
	*document.Manifest.Descriptions[4].Value.Text = "mutated"
	document.Manifest.Descriptions[0].RowID = "0"
	decoded, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
	if err != nil || !reflect.DeepEqual(decoded, original) {
		t.Fatal("encode retained caller-owned inputs:", err)
	}
	encodedBefore := append([]byte(nil), encoded...)
	decoded.Data[0] = 'X'
	decoded.Manifest.Columns[0].Name = "mutated"
	decoded.Manifest.Storage[0][0] = "blob"
	*decoded.Manifest.Descriptions[4].Value.Text = "mutated"
	again, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
	if err != nil || !reflect.DeepEqual(again, original) || !bytes.Equal(encoded, encodedBefore) {
		t.Fatal("decoded outputs alias archive or another decode:", err)
	}
	for index := range encoded {
		encoded[index] = 0
	}
	if !reflect.DeepEqual(again, original) {
		t.Fatal("decoded bytes retained input archive")
	}
	// Independently produced Store archives use data descriptors and either order.
	independent := tableCSVBundleTestArchive(t,
		tableCSVBundleTestMember{"manifest.json", tableCSVBundleTestManifest(t, original), zip.Store, 0},
		tableCSVBundleTestMember{"table.csv", original.Data, zip.Store, 0})
	decoded, err = decodeTableCSVBundle(ctx, independent, int64(len(independent)))
	if err != nil || !reflect.DeepEqual(decoded, original) {
		t.Fatal("conventional ZIP data descriptors did not roundtrip:", err)
	}
}

func TestTableCSVBundleEmptyTableAndSingleColumnNULL(t *testing.T) {
	empty := ""
	for _, rows := range [][]ProjectMetadataRow{
		{},
		{{RowID: "-9007199254740993", Cells: []ProjectMetadataCell{{Storage: "null"}}}},
		{
			{RowID: "1", Cells: []ProjectMetadataCell{{Storage: "null"}}},
			{RowID: "2", Cells: []ProjectMetadataCell{{Storage: "blob", BlobHex: &empty}}},
			{RowID: "3", Cells: []ProjectMetadataCell{{Storage: "text", Text: &empty}}},
		},
	} {
		table := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Only", DeclaredType: ""}}, Rows: rows}
		document, err := encodeTableCSV(context.Background(), "Physical", table, []TableCSVDescription{})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := encodeTableCSVBundle(context.Background(), document)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeTableCSVBundle(context.Background(), encoded, int64(len(encoded)))
		if err != nil || !reflect.DeepEqual(decoded, document) {
			t.Fatal("empty table/single column NULL lost:", err)
		}
		got, err := decodeTableCSV(context.Background(), decoded)
		if err != nil || !reflect.DeepEqual(got, table) {
			t.Fatal("physical row lost:", err)
		}
	}
}

func TestTableCSVBundleCheckedAggregateZIP64Budget(t *testing.T) {
	for _, sizes := range [][2]uint64{
		{1, math.MaxUint64},
		{uint64(math.MaxInt64), 1},
	} {
		var output bytes.Buffer
		writer := zip.NewWriter(&output)
		for index, name := range []string{"table.csv", "manifest.json"} {
			if _, err := writer.CreateRaw(&zip.FileHeader{Name: name, Method: zip.Store,
				CompressedSize64: sizes[index], UncompressedSize64: sizes[index]}); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		tableCSVBundleAssertRejected(t, output.Bytes(), math.MaxInt64)
	}
}

func TestTableCSVBundleRejectsMemberSetAndUnsupportedHeaders(t *testing.T) {
	document := tableCSVBundleFixture(t)
	csv := tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0}
	manifest := tableCSVBundleTestMember{"manifest.json", tableCSVBundleTestManifest(t, document), zip.Store, 0}
	cases := map[string][]tableCSVBundleTestMember{
		"missing":   {csv},
		"extra":     {csv, manifest, {"extra", nil, zip.Store, 0}},
		"duplicate": {csv, csv},
		"unknown":   {csv, {"Manifest.json", manifest.data, zip.Store, 0}},
		"traversal": {csv, {"../manifest.json", manifest.data, zip.Store, 0}},
		"directory": {csv, {"manifest.json/", nil, zip.Store, os.ModeDir | 0700}},
		"symlink":   {csv, {"manifest.json", manifest.data, zip.Store, os.ModeSymlink | 0600}},
		"pipe":      {csv, {"manifest.json", manifest.data, zip.Store, os.ModeNamedPipe | 0600}},
		"deflate":   {{csv.name, csv.data, zip.Deflate, 0}, manifest},
	}
	for name, members := range cases {
		t.Run(name, func(t *testing.T) {
			data := tableCSVBundleTestArchive(t, members...)
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
		})
	}
	encoded, err := encodeTableCSVBundle(context.Background(), document)
	if err != nil {
		t.Fatal(err)
	}
	central := bytes.Index(encoded, []byte{'P', 'K', 1, 2})
	archive, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	dataOffset, err := archive.File[0].DataOffset()
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func([]byte){
		"local signature": func(b []byte) { b[0] = 0 },
		"local method":    func(b []byte) { binary.LittleEndian.PutUint16(b[8:], zip.Deflate) },
		"local encrypted": func(b []byte) { binary.LittleEndian.PutUint16(b[6:], 1) },
		"local name":      func(b []byte) { b[30] = 'X' },
		"local size":      func(b []byte) { b[18]++ },
		"local crc":       func(b []byte) { b[14]++ },
		"central crc":     func(b []byte) { b[central+16]++ },
		"payload crc":     func(b []byte) { b[dataOffset] ^= 1 },
		"encryption": func(b []byte) {
			binary.LittleEndian.PutUint16(b[6:], 1)
			binary.LittleEndian.PutUint16(b[central+8:], 1)
		},
		"strong encryption": func(b []byte) {
			binary.LittleEndian.PutUint16(b[6:], 0x40)
			binary.LittleEndian.PutUint16(b[central+8:], 0x40)
		},
		"unsupported method": func(b []byte) {
			binary.LittleEndian.PutUint16(b[8:], 99)
			binary.LittleEndian.PutUint16(b[central+10:], 99)
		},
		"aggregate oversized": func(b []byte) {
			size := uint32(len(b))
			binary.LittleEndian.PutUint32(b[central+20:], size)
			binary.LittleEndian.PutUint32(b[central+24:], size)
		},
		"Store size mismatch": func(b []byte) { b[central+20]++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			data := append([]byte(nil), encoded...)
			mutate(data)
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
		})
	}
	tableCSVBundleAssertRejected(t, []byte("not zip"), 100)
	tableCSVBundleAssertRejected(t, encoded[:len(encoded)-1], int64(len(encoded)))
	descriptorZIP := tableCSVBundleTestArchive(t, csv, manifest)
	descriptor := bytes.Index(descriptorZIP, []byte{'P', 'K', 7, 8})
	for _, offset := range []int{4, 8, 12} {
		data := append([]byte(nil), descriptorZIP...)
		data[descriptor+offset]++
		tableCSVBundleAssertRejected(t, data, int64(len(data)))
	}
}

func TestTableCSVBundleRejectsManifestRepairAmbiguityAndCSVMismatch(t *testing.T) {
	document := tableCSVBundleFixture(t)
	raw := string(tableCSVBundleTestManifest(t, document))
	mutations := map[string]string{
		"bad syntax":          "{",
		"trailing object":     raw + "{}",
		"trailing scalar":     raw + " true",
		"not object":          "[]",
		"root unknown":        strings.Replace(raw, `"version":`, `"extra":0,"version":`, 1),
		"root duplicate":      strings.Replace(raw, `"version":`, `"version":1,"version":`, 1),
		"escaped duplicate":   strings.Replace(raw, `"version":`, `"\u0076ersion":1,"version":`, 1),
		"case alias":          strings.Replace(raw, `"version":`, `"Version":`, 1),
		"missing version":     strings.Replace(raw, `"version":1,`, "", 1),
		"column unknown":      strings.Replace(raw, `"name":`, `"other":0,"name":`, 1),
		"column duplicate":    strings.Replace(raw, `"declaredType":`, `"declaredType":"X","declaredType":`, 1),
		"missing type":        strings.Replace(raw, `,"declaredType":"TEXT"`, "", 1),
		"description unknown": strings.Replace(raw, `"rowId":`, `"other":0,"rowId":`, 1),
		"description duplicate": strings.Replace(raw, `"rowId":`,
			`"rowId":"1","rowId":`, 1),
		"cell unknown":         strings.Replace(raw, `"storage":"null"`, `"other":0,"storage":"null"`, 1),
		"cell duplicate":       strings.Replace(raw, `"text":null`, `"text":null,"text":null`, 1),
		"cell missing payload": strings.Replace(raw, `"text":null,`, "", 1),
		"cell duplicate real": strings.Replace(raw, `"real":-0`,
			`"real":1,"real":-0`, 1),
		"columns null":     strings.Replace(raw, `"columns":[`, `"columns":null,"unused":[`, 1),
		"wrong field type": strings.Replace(raw, `"version":1`, `"version":"1"`, 1),
		"invalid utf8":     strings.Replace(raw, "Literal_", "Literal_"+string([]byte{0xff}), 1),
		"root surrogate":   strings.Replace(raw, "Literal_", `\ud800`, 1),
		"column surrogate": strings.Replace(raw, `"Nullable"`, `"\udc00"`, 1),
		"key surrogate":    strings.Replace(raw, `"version"`, `"\ud800"`, 1),
		"nested surrogate": strings.Replace(raw, `"text":""`, `"text":"\ud800"`, 1),
		"mismatched pair":  strings.Replace(raw, `"text":""`, `"text":"\ud800\u0041"`, 1),
		"truncated escape": strings.Replace(raw, `"text":""`, `"text":"\u00"`, 1),
		"checksum":         strings.Replace(raw, document.Manifest.SHA256, strings.Repeat("0", 64), 1),
		"manifest version": strings.Replace(raw, `"version":1`, `"version":2`, 1),
	}
	mutations["storage mismatch"] = strings.Replace(raw, `"storage":[["null"`, `"storage":[["text"`, 1)
	for name, manifest := range mutations {
		t.Run(name, func(t *testing.T) {
			data := tableCSVBundleTestArchive(t,
				tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
				tableCSVBundleTestMember{"manifest.json", []byte(manifest), zip.Store, 0})
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
		})
	}
	for name, csv := range map[string][]byte{
		"checksum": append(append([]byte(nil), document.Data...), 'x'),
		"header":   []byte(strings.Replace(string(document.Data), "Nullable", "Wrong", 1)),
	} {
		t.Run("csv "+name, func(t *testing.T) {
			bad := tableCSVClone(t, document)
			if name == "header" {
				tableCSVReplaceData(&bad, string(csv))
			}
			data := tableCSVBundleTestArchive(t,
				tableCSVBundleTestMember{"table.csv", csv, zip.Store, 0},
				tableCSVBundleTestMember{"manifest.json", tableCSVBundleTestManifest(t, bad), zip.Store, 0})
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
			if encoded, err := encodeTableCSVBundle(context.Background(), bad); name == "header" && (err == nil || encoded != nil) {
				t.Fatal("encode accepted an invalid document")
			}
		})
	}
	// Valid paired escapes retain astral text; literal backslash-u is not repaired.
	valid := strings.Replace(raw, "\U0001f332", `\ud83c\udf32`, -1)
	data := tableCSVBundleTestArchive(t,
		tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
		tableCSVBundleTestMember{"manifest.json", []byte(valid), zip.Store, 0})
	decoded, err := decodeTableCSVBundle(context.Background(), data, int64(len(data)))
	if err != nil || !reflect.DeepEqual(decoded, document) {
		t.Fatal("valid surrogate pair lost:", err)
	}
}

func TestTableCSVBundleEncodeRequiresAlreadyValidatedDocument(t *testing.T) {
	document := tableCSVBundleFixture(t)
	for name, mutate := range map[string]func(*tableCSVDocument){
		"checksum": func(d *tableCSVDocument) { d.Data[0] = 'X' },
		"nil descriptions": func(d *tableCSVDocument) {
			d.Manifest.Descriptions = nil
		},
		"invalid description Unicode": func(d *tableCSVDocument) {
			*d.Manifest.Descriptions[4].Value.Text = string([]byte{0xff})
		},
		"nonfinite description": func(d *tableCSVDocument) {
			*d.Manifest.Descriptions[6].Value.Real = math.Inf(1)
		},
		"invalid column Unicode": func(d *tableCSVDocument) {
			d.Manifest.Columns[0].DeclaredType = string([]byte{0xff})
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := tableCSVClone(t, document)
			mutate(&bad)
			result, err := encodeTableCSVBundle(context.Background(), bad)
			if err == nil || result != nil {
				t.Fatal("invalid existing document produced a bundle:", err)
			}
		})
	}
}

func TestTableCSVBundleDeclaredTypeRequiresStringPreservesExplicitEmpty(t *testing.T) {
	document := tableCSVBundleFixture(t)
	raw := string(tableCSVBundleTestManifest(t, document))
	for _, value := range []string{"null", "0", "false", "[]", "{}"} {
		t.Run(value, func(t *testing.T) {
			manifest := strings.Replace(raw, `"declaredType":"TEXT"`, `"declaredType":`+value, 1)
			data := tableCSVBundleTestArchive(t,
				tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
				tableCSVBundleTestMember{"manifest.json", []byte(manifest), zip.Store, 0})
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
		})
	}
	document.Manifest.Columns[0].DeclaredType = ""
	manifest := strings.Replace(raw, `"declaredType":"TEXT"`, `"declaredType":""`, 1)
	data := tableCSVBundleTestArchive(t,
		tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
		tableCSVBundleTestMember{"manifest.json", []byte(manifest), zip.Store, 0})
	decoded, err := decodeTableCSVBundle(context.Background(), data, int64(len(data)))
	if err != nil || !reflect.DeepEqual(decoded, document) {
		t.Fatal("explicit empty declared type was not preserved:", err)
	}
	encoded, err := encodeTableCSVBundle(context.Background(), decoded)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	source, err := archive.File[1].Open()
	if err != nil {
		t.Fatal(err)
	}
	encodedManifest, err := io.ReadAll(source)
	closeErr := source.Close()
	if err != nil || closeErr != nil || !bytes.Equal(encodedManifest, tableCSVBundleTestManifest(t, document)) {
		t.Fatal("re-encoded manifest changed explicit empty declared type:", err, closeErr)
	}
}

func TestTableCSVBundleOtherScalarNullsCannotBecomeValidMetadata(t *testing.T) {
	document := tableCSVBundleFixture(t)
	raw := string(tableCSVBundleTestManifest(t, document))
	for name, manifest := range map[string]string{
		"version":     strings.Replace(raw, `"version":1`, `"version":null`, 1),
		"table":       strings.Replace(raw, `"table":"Literal_🌲"`, `"table":null`, 1),
		"sha256":      strings.Replace(raw, `"`+document.Manifest.SHA256+`"`, `null`, 1),
		"column name": strings.Replace(raw, `"name":"Nullable"`, `"name":null`, 1),
		"row identity": strings.Replace(raw, `"rowIds":["9007199254740993"]`,
			`"rowIds":[null]`, 1),
		"storage tag": strings.Replace(raw, `"storage":[["null"`, `"storage":[[null`, 1),
		"description identity": strings.Replace(raw, `"rowId":"-9223372036854775808"`,
			`"rowId":null`, 1),
		"cell storage": strings.Replace(raw, `"storage":"null"`, `"storage":null`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if manifest == raw {
				t.Fatal("scalar NULL mutation did not match the fixture")
			}
			data := tableCSVBundleTestArchive(t,
				tableCSVBundleTestMember{"table.csv", document.Data, zip.Store, 0},
				tableCSVBundleTestMember{"manifest.json", []byte(manifest), zip.Store, 0})
			tableCSVBundleAssertRejected(t, data, int64(len(data)))
		})
	}
}

type tableCSVBundleMutationContext struct {
	context.Context
	calls  int
	at     int
	mutate func()
}

func (ctx *tableCSVBundleMutationContext) Err() error {
	ctx.calls++
	if ctx.calls == ctx.at {
		ctx.mutate()
	}
	return ctx.Context.Err()
}

func TestTableCSVBundleSnapshotsBeforeEveryContextCheckpoint(t *testing.T) {
	original := tableCSVBundleFixture(t)
	encodeCount := &tableCSVBundleMutationContext{Context: context.Background()}
	want, err := encodeTableCSVBundle(encodeCount, original)
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= encodeCount.calls; at++ {
		document := tableCSVClone(t, original)
		ctx := &tableCSVBundleMutationContext{Context: context.Background(), at: at, mutate: func() {
			document.Data[0] = 'X'
			document.Manifest.Columns[0].Name = "changed"
			document.Manifest.RowIDs[0] = "0"
			document.Manifest.Storage[0][0] = "text"
			document.Manifest.Descriptions[0].RowID = "0"
			*document.Manifest.Descriptions[4].Value.Text = "changed"
			*document.Manifest.Descriptions[6].Value.Real = 1
			*document.Manifest.Descriptions[7].Value.Integer = "0"
			*document.Manifest.Descriptions[8].Value.BlobHex = "abcd"
			*document.Manifest.Descriptions[9].Value.BlobHex = "ff"
		}}
		got, err := encodeTableCSVBundle(ctx, document)
		if err != nil || !bytes.Equal(got, want) || ctx.calls < at {
			t.Fatalf("encode alias mutation at checkpoint %d changed the snapshot: %v", at, err)
		}
	}
	decodeCount := &tableCSVBundleMutationContext{Context: context.Background()}
	if _, err := decodeTableCSVBundle(decodeCount, want, int64(len(want))); err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= decodeCount.calls; at++ {
		encoded := append([]byte(nil), want...)
		ctx := &tableCSVBundleMutationContext{Context: context.Background(), at: at, mutate: func() {
			for index := range encoded {
				encoded[index] = 0
			}
		}}
		got, err := decodeTableCSVBundle(ctx, encoded, int64(len(encoded)))
		if err != nil || !reflect.DeepEqual(got, original) || ctx.calls < at {
			t.Fatalf("decode alias mutation at checkpoint %d changed the snapshot: %v", at, err)
		}
	}
}

func TestTableCSVBundleSnapshotDoesNotRepairMalformedCallerText(t *testing.T) {
	document := tableCSVBundleFixture(t)
	*document.Manifest.Descriptions[4].Value.Text = string([]byte{0xff})
	ctx := &tableCSVBundleMutationContext{Context: context.Background(), at: 1, mutate: func() {
		*document.Manifest.Descriptions[4].Value.Text = "repaired later"
	}}
	got, err := encodeTableCSVBundle(ctx, document)
	if err == nil || got != nil {
		t.Fatal("snapshot repaired malformed original text or validated a later alias mutation:", err)
	}
}

type tableCSVBundleCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
	at     int
}

func (ctx *tableCSVBundleCancelContext) Err() error {
	ctx.calls++
	if ctx.at > 0 && ctx.calls == ctx.at {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func tableCSVBundleCancellation(at int) *tableCSVBundleCancelContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &tableCSVBundleCancelContext{Context: ctx, cancel: cancel, at: at}
}

func TestTableCSVBundleCancellationAtEveryCheckpointReturnsZero(t *testing.T) {
	document := tableCSVBundleFixture(t)
	// Larger than the bounded reader chunk, with cancellation within member I/O.
	text := strings.Repeat("literal\r\n\x00\U0001f332", 3000)
	document.Manifest.Descriptions[4].Value.Text = &text
	encodeCtx := tableCSVBundleCancellation(0)
	data, err := encodeTableCSVBundle(encodeCtx, document)
	encodeCtx.cancel()
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= encodeCtx.calls; at++ {
		ctx := tableCSVBundleCancellation(at)
		result, err := encodeTableCSVBundle(ctx, document)
		ctx.cancel()
		if !errors.Is(err, context.Canceled) || result != nil {
			t.Fatalf("encode cancellation checkpoint %d produced partial success: %v", at, err)
		}
	}
	decodeCtx := tableCSVBundleCancellation(0)
	_, err = decodeTableCSVBundle(decodeCtx, data, int64(len(data)))
	decodeCtx.cancel()
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= decodeCtx.calls; at++ {
		ctx := tableCSVBundleCancellation(at)
		result, err := decodeTableCSVBundle(ctx, data, int64(len(data)))
		ctx.cancel()
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, tableCSVDocument{}) {
			t.Fatalf("decode cancellation checkpoint %d produced partial success: %v", at, err)
		}
	}
}
