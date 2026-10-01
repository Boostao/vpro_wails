package listcatalog

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	_ "github.com/mattn/go-sqlite3"
)

func fixture(t *testing.T) ([]byte, Provenance) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "site-codes-fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	p := Provenance{Version: 1, Exporter: Exporter, SourceSHA256: SourceSHA256,
		SnapshotSHA256: SnapshotSHA256, DatabaseSHA256: strings.Repeat("0", 64),
		TypedCellsSHA256: strings.Repeat("0", 64), Rows: 139, Cells: 1390,
		SourceTable: "USysTableOfLists", SQL: SourceSQL, SourceSchema: ExpectedSchema(),
		ItemOrderEncoding: ItemOrderEncoding}
	rows, err := SnapshotRows(data, p)
	if err != nil {
		t.Fatal(err)
	}
	p.TypedCellsSHA256, err = TypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	return data, p
}

func memoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func nativeCells(row Choice) []Cell {
	texts := map[string]*string{"ListName": row.ListName, "ListFilter": row.ListFilter,
		"Item": row.Code, "ItemDescription": row.Description, "FieldUsedIn": row.FieldUsedIn,
		"ValidateLoops": row.ValidateLoops, "Note": row.Note}
	var cells []Cell
	for _, field := range ExpectedSchema() {
		cell := Cell{Name: field.Name, DAOType: field.DAOType, IsNull: true}
		if field.DAOType == 10 {
			cell.Value = texts[field.Name]
			if cell.Value != nil {
				cell.Utf16CodeUnits = utf16.Encode([]rune(*cell.Value))
			}
		} else if field.DAOType == 7 && row.ItemOrder != nil {
			value := strconv.FormatFloat(*row.ItemOrder, 'g', -1, 64)
			cell.Value = &value
		} else if field.DAOType == 1 {
			value := row.Flag
			if field.Name == "Validate" {
				value = row.Validate
			}
			if value != nil {
				text := "False"
				if *value {
					text = "True"
				}
				cell.Value = &text
			}
		}
		cell.IsNull = cell.Value == nil
		cells = append(cells, cell)
	}
	return cells
}

func TestFixtureAllCellsRoundTrip(t *testing.T) {
	data, p := fixture(t)
	db := memoryDB(t)
	if err := ImportSnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDatabase(db, p); err != nil {
		t.Fatal(err)
	}
	rows, err := ReadChoices(db, "")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := SnapshotRows(data, p)
	if err != nil || !reflect.DeepEqual(rows, expected) {
		t.Fatalf("all metadata rows differ: %v", err)
	}
	var native nativeSnapshot
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	compared, duplicates := 0, map[string]int{}
	for _, row := range rows {
		source := native.Lists[*row.ListName].Rows[mustOrdinal(row.RowID)-1].Cells
		restored := nativeCells(row)
		for i, cell := range source {
			got := restored[i]
			if cell.DAOType == 7 && !cell.IsNull {
				want, _ := strconv.ParseFloat(*cell.Value, 64)
				actual, _ := strconv.ParseFloat(*got.Value, 64)
				if math.Float64bits(want) != math.Float64bits(actual) {
					t.Fatalf("Double bits differ at %s/%s", row.RowID, cell.Name)
				}
			} else if !reflect.DeepEqual(cell, got) {
				t.Fatalf("cell differs at %s/%s: %#v / %#v", row.RowID, cell.Name, cell, got)
			}
			compared++
		}
		if row.Selectable || row.Diagnostic != "" {
			t.Fatal("reader assigned selection policy")
		}
		if row.Code != nil {
			duplicates[*row.Code]++
		}
	}
	if compared != 1390 || duplicates["M.f"] != 2 || duplicates["M.s"] != 2 {
		t.Fatalf("cells/duplicates: %d, %v", compared, duplicates)
	}
	for list, count := range map[string]int{"Exposure": 12, "SiteDisturbance": 127} {
		subset, err := ReadChoices(db, list)
		if err != nil || len(subset) != count {
			t.Fatalf("%s: %d, %v", list, len(subset), err)
		}
		for i, row := range subset {
			if row.RowID != strconv.Itoa(i+1) {
				t.Fatal("lost source ordinal")
			}
		}
	}
	if _, err := ReadChoices(db, "unverified"); err == nil {
		t.Fatal("unknown list accepted")
	}
}

func TestBundledCatalogue(t *testing.T) {
	database := filepath.Join("..", "..", "resources", "site-codes.db")
	provenance := filepath.Join("..", "..", "resources", "site-codes-provenance.json")
	data, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(provenance)
	if err != nil {
		t.Fatal(err)
	}
	p, err := DecodeProvenance(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if checksum(data) != p.DatabaseSHA256 {
		t.Fatal("bundled database checksum mismatch")
	}
	db, err := sql.Open("sqlite3", "file:"+database+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ValidateDatabase(db, p); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM SiteCodeChoices`); err == nil {
		t.Fatal("bundled validator connection is not read-only")
	}
}

func mustOrdinal(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func TestDecodeSyntheticAndInvalidCells(t *testing.T) {
	text, truth := "A\U0001F332", true
	negativeZero := math.Copysign(0, -1)
	row := Choice{RowID: "synthetic", Code: &text, ItemOrder: &negativeZero, Validate: &truth}
	cells := nativeCells(row)
	got, err := DecodeCells(row.RowID, cells)
	if err != nil || !reflect.DeepEqual(got, row) || !bytes.Equal(OrderBytes(got.ItemOrder), []byte{128, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("synthetic negative zero/Unicode: %+v %v", got, err)
	}
	if OrderBytes(nil) != nil {
		t.Fatal("NULL Double lost")
	}
	for _, value := range []string{"True", "-1", "False", "0"} {
		cells[7].Value = &value
		result, err := DecodeCells("bool", cells)
		if err != nil || *result.Validate != (value == "True" || value == "-1") {
			t.Fatalf("DAO Boolean %s: %v", value, err)
		}
	}
	fraction := -1.375
	row.ItemOrder = &fraction
	cells = nativeCells(row)
	got, err = DecodeCells("synthetic", cells)
	if err != nil || *got.ItemOrder != fraction {
		t.Fatalf("fractional Double: %v", err)
	}
	bad := map[string]func([]Cell) []Cell{
		"count":         func(c []Cell) []Cell { return c[:9] },
		"name":          func(c []Cell) []Cell { c[0].Name = "ID"; return c },
		"type":          func(c []Cell) []Cell { c[0].DAOType = 12; return c },
		"null":          func(c []Cell) []Cell { c[0].IsNull = false; return c },
		"null utf16":    func(c []Cell) []Cell { c[0].Utf16CodeUnits = []uint16{}; return c },
		"surrogate":     func(c []Cell) []Cell { c[3].Utf16CodeUnits = []uint16{0xD800}; return c },
		"missing utf16": func(c []Cell) []Cell { c[3].Utf16CodeUnits = nil; return c },
		"numeric utf16": func(c []Cell) []Cell { c[2].Utf16CodeUnits = []uint16{}; return c },
	}
	for _, value := range []string{"NaN", "+Inf", "-Inf", "1e999", "bogus"} {
		v := value
		bad["Double "+v] = func(c []Cell) []Cell { c[2].Value = &v; return c }
	}
	for _, value := range []string{"1", "true", "FALSE", "2", ""} {
		v := value
		bad["Boolean "+v] = func(c []Cell) []Cell { c[7].Value = &v; return c }
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCells("bad", mutate(nativeCells(row))); err == nil {
				t.Fatal("malformed cells accepted")
			}
		})
	}
	for _, value := range []string{strings.Repeat("x", 256), string([]byte{0xff})} {
		row.Code = &value
		if _, err := DecodeCells("bad", nativeCells(row)); err == nil {
			t.Fatal("invalid/oversize text accepted")
		}
	}
	nan := math.NaN()
	if _, err := TypedHash([]Choice{{ItemOrder: &nan}}); err == nil {
		t.Fatal("hash accepted nonfinite Double")
	}
}

func TestProvenanceAndSnapshotFailures(t *testing.T) {
	data, p := fixture(t)
	encoded, _ := json.Marshal(p)
	if decoded, err := DecodeProvenance(encoded); err != nil || !reflect.DeepEqual(decoded, p) {
		t.Fatalf("provenance roundtrip: %v", err)
	}
	for _, malformed := range [][]byte{[]byte(`{}`), append(encoded, []byte(` {}`)...), []byte(`{"unknown":1}`), []byte(`null`)} {
		if _, err := DecodeProvenance(malformed); err == nil {
			t.Fatal("malformed provenance accepted")
		}
	}
	mutations := map[string]func(*Provenance){
		"version":    func(p *Provenance) { p.Version++ },
		"exporter":   func(p *Provenance) { p.Exporter = "unknown" },
		"source":     func(p *Provenance) { p.SourceSHA256 = strings.Repeat("0", 64) },
		"snapshot":   func(p *Provenance) { p.SnapshotSHA256 = strings.Repeat("0", 64) },
		"db hash":    func(p *Provenance) { p.DatabaseSHA256 = "x" },
		"typed hash": func(p *Provenance) { p.TypedCellsSHA256 = strings.ToUpper(p.TypedCellsSHA256) },
		"rows":       func(p *Provenance) { p.Rows-- },
		"cells":      func(p *Provenance) { p.Cells-- },
		"table":      func(p *Provenance) { p.SourceTable = "Other" },
		"sql":        func(p *Provenance) { p.SQL += ";" },
		"schema":     func(p *Provenance) { p.SourceSchema = nil },
		"order":      func(p *Provenance) { p.ItemOrderEncoding = "REAL" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := p
			mutate(&copy)
			if err := ValidateProvenance(copy); err == nil {
				t.Fatal("invalid provenance accepted")
			}
		})
	}
	if _, err := SnapshotRows(append(append([]byte{}, data...), ' '), p); err == nil {
		t.Fatal("changed snapshot accepted")
	}
	if _, err := SnapshotRows([]byte(`{`), p); err == nil {
		t.Fatal("invalid snapshot accepted")
	}
	snapshotMutations := map[string]func(*nativeSnapshot){
		"readonly":  func(s *nativeSnapshot) { s.ReadOnly = false },
		"unchanged": func(s *nativeSnapshot) { s.Unchanged.Equal = false },
		"query":     func(s *nativeSnapshot) { s.SQL = "" },
		"schema":    func(s *nativeSnapshot) { s.Schema.Fields[0].Type = 12 },
		"source":    func(s *nativeSnapshot) { s.Source.After = "" },
		"count":     func(s *nativeSnapshot) { s.CellCount-- },
		"list":      func(s *nativeSnapshot) { delete(s.Lists, "Exposure") },
		"ordinal":   func(s *nativeSnapshot) { s.Lists["Exposure"].Rows[0].Ordinal = "01" },
		"cell":      func(s *nativeSnapshot) { s.Lists["Exposure"].Rows[0].Cells[0].DAOType = 12 },
		"membership": func(s *nativeSnapshot) {
			cell := &s.Lists["Exposure"].Rows[0].Cells[0]
			text := "SiteDisturbance"
			cell.Value = &text
			cell.Utf16CodeUnits = utf16.Encode([]rune(text))
		},
	}
	for name, mutate := range snapshotMutations {
		t.Run("snapshot "+name, func(t *testing.T) {
			var s nativeSnapshot
			json.Unmarshal(data, &s)
			mutate(&s)
			encoded, _ := json.Marshal(s)
			if _, err := decodeSnapshot(encoded, p); err == nil {
				t.Fatal("invalid native snapshot structure accepted")
			}
		})
	}
}

func TestImportCollisionRollbackAndTampering(t *testing.T) {
	data, p := fixture(t)
	for _, existing := range []string{"SiteCodeChoices", "SiteCodeProvenance"} {
		t.Run(existing, func(t *testing.T) {
			db := memoryDB(t)
			if _, err := db.Exec(`CREATE TABLE ` + existing + `(sentinel TEXT); INSERT INTO ` + existing + ` VALUES ('unchanged')`); err != nil {
				t.Fatal(err)
			}
			if err := ImportSnapshot(db, data, p); err == nil {
				t.Fatal("collision accepted")
			}
			var tables int
			db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table'`).Scan(&tables)
			var sentinel string
			db.QueryRow(`SELECT sentinel FROM ` + existing).Scan(&sentinel)
			if tables != 1 || sentinel != "unchanged" {
				t.Fatal("collision altered existing database or leaked table")
			}
		})
	}
	for _, mutation := range []string{
		`UPDATE SiteCodeChoices SET Item='changed' WHERE ListName='Exposure' AND RowID='1'`,
		`UPDATE SiteCodeChoices SET RowID='01' WHERE ListName='Exposure' AND RowID='1'`,
		`DELETE FROM SiteCodeChoices WHERE ListName='Exposure' AND RowID='1'`,
		`UPDATE SiteCodeProvenance SET Exporter='unknown'`,
		`INSERT INTO SiteCodeProvenance SELECT * FROM SiteCodeProvenance`,
		`PRAGMA ignore_check_constraints=ON; UPDATE SiteCodeChoices SET Validate=-1 WHERE ListName='Exposure' AND RowID='1'`,
		`PRAGMA ignore_check_constraints=ON; UPDATE SiteCodeChoices SET ItemOrder='12345678' WHERE ListName='Exposure' AND RowID='1'`,
		`UPDATE SiteCodeChoices SET ItemOrder=X'7ff0000000000000' WHERE ListName='Exposure' AND RowID='1'`,
	} {
		t.Run(mutation, func(t *testing.T) {
			db := memoryDB(t)
			if err := ImportSnapshot(db, data, p); err != nil {
				t.Fatal(err)
			}
			if err := ImportSnapshot(db, data, p); err == nil {
				t.Fatal("second import accepted")
			}
			if err := ValidateDatabase(db, p); err != nil {
				t.Fatalf("second import damaged catalogue: %v", err)
			}
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			if err := ValidateDatabase(db, p); err == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
	bad := p
	bad.TypedCellsSHA256 = strings.Repeat("0", 64)
	db := memoryDB(t)
	if err := ImportSnapshot(db, data, bad); err == nil {
		t.Fatal("typed checksum mismatch accepted")
	}
	var tables int
	db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table'`).Scan(&tables)
	if tables != 0 {
		t.Fatal("bad import changed database")
	}
}

func TestSyntheticDatabaseDoubleAndNulls(t *testing.T) {
	db := memoryDB(t)
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatal(err)
	}
	values := []*float64{nil, new(float64), new(float64)}
	*values[1] = math.Copysign(0, -1)
	*values[2] = -2.75
	for i, value := range values {
		if _, err := db.Exec(`INSERT INTO SiteCodeChoices(RowID,ListName,ItemOrder) VALUES (?,'Exposure',?)`, strconv.Itoa(i+1), OrderBytes(value)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := ReadChoices(db, "Exposure")
	if err != nil || len(rows) != 3 {
		t.Fatalf("read synthetic rows: %v", err)
	}
	for i, row := range rows {
		if !bytes.Equal(OrderBytes(row.ItemOrder), OrderBytes(values[i])) {
			t.Fatal("synthetic Double bits lost")
		}
		if row.Code != nil || row.Description != nil || row.Validate != nil || row.Flag != nil {
			t.Fatal("NULL metadata lost")
		}
	}
	positiveZero := 0.0
	a, _ := TypedHash([]Choice{{ItemOrder: &positiveZero}})
	b, _ := TypedHash([]Choice{{ItemOrder: values[1]}})
	c, _ := TypedHash([]Choice{{}})
	if a == b || a == c || b == c {
		t.Fatal("typed hash conflates zero, negative zero or NULL")
	}
}
