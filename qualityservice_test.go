package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"
)

func qualityString(value string) *string { return &value }

func qualityTestProvenance(t *testing.T) qualityProvenance {
	t.Helper()
	var p qualityProvenance
	if err := decodeQualityJSON(qualityProvenanceJSON, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func qualitySourceChoices() []PlotQualityChoice {
	codes := []string{"NA", "Excellent", "Good", "Fair", "Poor"}
	descriptions := []string{"null", "Excellent - FS882 Full detail", "Good - FS882 Partial or Lower quality",
		"Fair - Ground Call / SIVI", "Poor - Visuals, Notes, Low quality"}
	notes := []string{"", "E", "G", "F", "P"}
	result := make([]PlotQualityChoice, 5)
	for i := range result {
		order, flag := float64(i+1), false
		result[i] = PlotQualityChoice{RowID: strconv.Itoa(i + 1), Code: qualityString(codes[i]),
			ListName: qualityString("PlotQualitySite"), ListFilter: qualityString(""), ItemOrder: &order,
			Description: qualityString(descriptions[i]), FieldUsedIn: qualityString(""),
			Validate: &flag, Note: qualityString(notes[i]), Flag: &flag, Selectable: true}
	}
	return result
}

func TestQualityCatalogueAll50CellsAndProvenance(t *testing.T) {
	dir := t.TempDir()
	service, err := NewQualityService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	got, err := service.ListPlotQualityChoices()
	if err != nil || !reflect.DeepEqual(got, qualitySourceChoices()) {
		t.Fatalf("native ten-column/five-row catalogue mismatch: %#v %v", got, err)
	}
	p := qualityTestProvenance(t)
	if err := validateQualityProvenance(p); err != nil {
		t.Fatal(err)
	}
	if becHash(qualityDatabase) != p.DatabaseSHA256 {
		t.Fatal("packaged database checksum mismatch")
	}
	hash, err := qualityTypedHash(got)
	if err != nil || hash != p.TypedCellsSHA256 || p.Cells != 50 {
		t.Fatalf("all typed cells mismatch: %s %v", hash, err)
	}
	if _, err := service.db.Exec(`UPDATE PlotQualityChoices SET Item='changed'`); err == nil {
		t.Fatal("catalogue is writable")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListPlotQualityChoices(); err == nil {
		t.Fatal("closed service silently succeeded")
	}
	reopened, err := NewQualityService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	again, err := reopened.ListPlotQualityChoices()
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("reopen changed catalogue: %v", err)
	}
}

func TestQualityCatalogueNonoverwritingCollisionAndConcurrency(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "plot-quality-site.db")
	bad := []byte("preexisting user file")
	if err := os.WriteFile(target, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewQualityService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("collision did not fail closed: %v", err)
	}
	actual, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(actual, bad) {
		t.Fatal("existing user file overwritten")
	}
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewQualityService(blocker); err == nil {
		t.Fatal("invalid data directory accepted")
	}
	service, err := NewQualityService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = service.ListPlotQualityChoices() // Close races deliberately; either rows or explicit closed error.
			}
		}()
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func qualitySyntheticFixture(t *testing.T, rows []PlotQualityChoice) ([]byte, qualityProvenance) {
	t.Helper()
	p := qualityTestProvenance(t)
	var snapshot qualityNativeSnapshot
	snapshot.Source.Before, snapshot.Source.After = strings.ToUpper(p.SourceSHA256), strings.ToUpper(p.SourceSHA256)
	snapshot.SourceTable, snapshot.Filter, snapshot.SQL = p.SourceTable, p.Filter, p.SQL
	snapshot.ReadOnly, snapshot.RowCount = true, len(rows)
	for _, field := range qualityExpectedSchema() {
		snapshot.Schema.Fields = append(snapshot.Schema.Fields, struct {
			Name string
			Type int
			Size int
		}{field.Name, field.DAOType, field.Size})
	}
	for _, row := range rows {
		values := []*string{row.ListName, row.ListFilter, nil, row.Code, row.Description,
			row.FieldUsedIn, row.ValidateLoops, nil, row.Note, nil}
		if row.ItemOrder != nil {
			values[2] = qualityString(strconv.FormatFloat(*row.ItemOrder, 'g', -1, 64))
		}
		for i, value := range []*bool{row.Validate, row.Flag} {
			if value != nil {
				text := "False"
				if *value {
					text = "True"
				}
				values[[]int{7, 9}[i]] = &text
			}
		}
		var cells []qualityNativeCell
		for i, field := range qualityExpectedSchema() {
			cell := qualityNativeCell{Name: field.Name, DAOType: field.DAOType, IsNull: values[i] == nil, Value: values[i]}
			if field.DAOType == 10 && cell.Value != nil {
				cell.Utf16CodeUnits = utf16.Encode([]rune(*cell.Value))
			}
			cells = append(cells, cell)
		}
		snapshot.Rows = append(snapshot.Rows, cells)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	p.SnapshotSHA256 = becHash(data)
	p.TypedCellsSHA256, err = qualityTypedHash(rows)
	if err != nil {
		t.Fatal(err)
	}
	return data, p
}

func qualityImportDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "quality.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestQualityImportNullBooleanIEEE64AndSourceOrder(t *testing.T) {
	want := qualitySourceChoices()
	want[0] = PlotQualityChoice{RowID: "1"}
	fraction, negativeZero, flag := math.Nextafter(1.2345678901234567, 2), math.Copysign(0, -1), true
	want[1].ItemOrder, want[1].Validate, want[1].Flag = &fraction, &flag, nil
	want[2].ItemOrder = &negativeZero
	want[3].Code = qualityString(strings.Repeat("x", 16))
	want[4].Code = qualityString("Good") // Duplicate Item remains a separate ordinal row.
	data, p := qualitySyntheticFixture(t, want)
	db := qualityImportDB(t)
	if err := importQualitySnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	got, err := qualityChoices(db)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		qualityChoiceStatus(&want[i])
	}
	if !reflect.DeepEqual(got, want) || math.Float64bits(*got[2].ItemOrder) != math.Float64bits(negativeZero) {
		t.Fatalf("NULL/bool/exact Double/source ordinal changed: %#v", got)
	}
	if got[0].Selectable || got[0].Diagnostic == "" || got[3].Selectable || got[3].Diagnostic == "" {
		t.Fatal("unselectable source row has no explicit diagnostic")
	}
	if err := importQualitySnapshot(db, data, p); err == nil {
		t.Fatal("import collision overwrote catalogue")
	}
	again, _ := qualityChoices(db)
	if !reflect.DeepEqual(again, got) {
		t.Fatal("failed repeat import changed rows")
	}
}

func TestQualityImportProvenanceAndTamperingFailClosed(t *testing.T) {
	data, p := qualitySyntheticFixture(t, qualitySourceChoices())
	for _, change := range []func(*qualityProvenance){
		func(p *qualityProvenance) { p.SourceSHA256 = "bad" },
		func(p *qualityProvenance) { p.SnapshotSHA256 = strings.Repeat("0", 64) },
		func(p *qualityProvenance) { p.TypedCellsSHA256 = strings.Repeat("0", 64) },
		func(p *qualityProvenance) { p.Exporter = "unknown" },
		func(p *qualityProvenance) { p.SourceSchema[0].Name = "Invented" },
	} {
		bad := p
		bad.SourceSchema = append([]becSourceColumn(nil), p.SourceSchema...)
		change(&bad)
		db := qualityImportDB(t)
		if err := importQualitySnapshot(db, data, bad); err == nil {
			t.Fatal("bad provenance accepted")
		}
		var tables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('PlotQualityChoices','QualityProvenance')`).Scan(&tables); err != nil || tables != 0 {
			t.Fatal("rejected import left partial schema")
		}
	}

	db := qualityImportDB(t)
	if err := importQualitySnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE PlotQualityChoices SET Note='wrong' WHERE RowID='1'`,
		`UPDATE QualityProvenance SET SourceSHA256='wrong'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		if err := validateQualityDatabase(db, p); err == nil {
			t.Fatal("database tampering was accepted")
		}
	}
}

func TestQualityEmbeddedAndInstalledChecksumsFailClosed(t *testing.T) {
	oldDB, oldProvenance := qualityDatabase, qualityProvenanceJSON
	t.Cleanup(func() { qualityDatabase, qualityProvenanceJSON = oldDB, oldProvenance })
	qualityDatabase = append(append([]byte(nil), oldDB...), 0)
	if _, err := NewQualityService(t.TempDir()); err == nil || !strings.Contains(err.Error(), "embedded") {
		t.Fatalf("embedded corruption accepted: %v", err)
	}

	qualityDatabase = oldDB
	qualityProvenanceJSON = []byte(`{"unknown":true}`)
	if _, err := NewQualityService(t.TempDir()); err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("invalid provenance accepted: %v", err)
	}
	qualityProvenanceJSON = oldProvenance
	dir := t.TempDir()
	service, err := NewQualityService(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "plot-quality-site.db")
	db, err := sql.Open("sqlite3", target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE PlotQualityChoices SET Item='X' WHERE RowID='1'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewQualityService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("installed corruption accepted: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("corrupt installed catalogue was silently overwritten")
	}
}

func TestQualityImportRejectsMalformedSourceCells(t *testing.T) {
	data, original := qualitySyntheticFixture(t, qualitySourceChoices())
	for _, mutate := range []func(*qualityNativeSnapshot){
		func(s *qualityNativeSnapshot) { s.ReadOnly = false },
		func(s *qualityNativeSnapshot) { s.Rows[0][0].DAOType = 7 },
		func(s *qualityNativeSnapshot) { s.Rows[0][3].Name = "ScientificName" },
		func(s *qualityNativeSnapshot) { s.Rows[0][3].Utf16CodeUnits = []uint16{0xD800} },
		func(s *qualityNativeSnapshot) { s.Rows[0][2].Value = qualityString("NaN") },
		func(s *qualityNativeSnapshot) { s.Rows[0][7].Value = qualityString("1") },
		func(s *qualityNativeSnapshot) { s.Rows[0][6].IsNull = false },
		func(s *qualityNativeSnapshot) { s.Rows[0][2].Utf16CodeUnits = []uint16{} },
		func(s *qualityNativeSnapshot) { s.Source.After = strings.Repeat("0", 64) },
	} {
		var snapshot qualityNativeSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			t.Fatal(err)
		}
		mutate(&snapshot)
		bad, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		p := original
		p.SnapshotSHA256 = becHash(bad)
		db := qualityImportDB(t)
		if err := importQualitySnapshot(db, bad, p); err == nil {
			t.Fatal("malformed source cell accepted")
		}
	}
}
