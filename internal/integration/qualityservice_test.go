package integration

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
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

func qualityTestProvenance(t *testing.T) references.QualityProvenance {
	t.Helper()
	var p references.QualityProvenance
	if err := references.DecodeQualityJSON(references.QualityProvenanceJSON, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func qualitySourceChoices() []references.PlotQualityChoice {
	codes := []string{"NA", "Excellent", "Good", "Fair", "Poor"}
	descriptions := []string{"null", "Excellent - FS882 Full detail", "Good - FS882 Partial or Lower quality",
		"Fair - Ground Call / SIVI", "Poor - Visuals, Notes, Low quality"}
	notes := []string{"", "E", "G", "F", "P"}
	result := make([]references.PlotQualityChoice, 5)
	for i := range result {
		order, flag := float64(i+1), false
		result[i] = references.PlotQualityChoice{RowID: strconv.Itoa(i + 1), Code: vprotest.QualityString(codes[i]),
			ListName: vprotest.QualityString("PlotQualitySite"), ListFilter: vprotest.QualityString(""), ItemOrder: &order,
			Description: vprotest.QualityString(descriptions[i]), FieldUsedIn: vprotest.QualityString(""),
			Validate: &flag, Note: vprotest.QualityString(notes[i]), Flag: &flag, Selectable: true}
	}
	return result
}

func TestQualityCatalogueAll50CellsAndProvenance(t *testing.T) {
	dir := t.TempDir()
	service, err := references.NewQualityService(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	got, err := service.ListPlotQualityChoices(context.Background())
	if err != nil || !reflect.DeepEqual(got, qualitySourceChoices()) {
		t.Fatalf("native ten-column/five-row catalogue mismatch: %#v %v", got, err)
	}
	p := qualityTestProvenance(t)
	if err := references.ValidateQualityProvenance(p); err != nil {
		t.Fatal(err)
	}
	if sqlutil.BecHash(references.QualityDatabase) != p.DatabaseSHA256 {
		t.Fatal("packaged database checksum mismatch")
	}
	hash, err := references.QualityTypedHash(got)
	if err != nil || hash != p.TypedCellsSHA256 || p.Cells != 50 {
		t.Fatalf("all typed cells mismatch: %s %v", hash, err)
	}
	if _, err := service.Db.Exec(`UPDATE PlotQualityChoices SET Item='changed'`); err == nil {
		t.Fatal("catalogue is writable")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListPlotQualityChoices(context.Background()); err == nil {
		t.Fatal("closed service silently succeeded")
	}
	reopened, err := references.NewQualityService(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	again, err := reopened.ListPlotQualityChoices(context.Background())
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
	if _, err := references.NewQualityService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
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
	if _, err := references.NewQualityService(blocker); err == nil {
		t.Fatal("invalid data directory accepted")
	}
	service, err := references.NewQualityService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = service.ListPlotQualityChoices(context.Background()) // Close races deliberately; either rows or explicit closed error.
			}
		}()
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func qualitySyntheticFixture(t *testing.T, rows []references.PlotQualityChoice) ([]byte, references.QualityProvenance) {
	t.Helper()
	p := qualityTestProvenance(t)
	var snapshot references.QualityNativeSnapshot
	snapshot.Source.Before, snapshot.Source.After = strings.ToUpper(p.SourceSHA256), strings.ToUpper(p.SourceSHA256)
	snapshot.SourceTable, snapshot.Filter, snapshot.SQL = p.SourceTable, p.Filter, p.SQL
	snapshot.ReadOnly, snapshot.RowCount = true, len(rows)
	for _, field := range references.QualityExpectedSchema() {
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
			values[2] = vprotest.QualityString(strconv.FormatFloat(*row.ItemOrder, 'g', -1, 64))
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
		var cells []references.QualityNativeCell
		for i, field := range references.QualityExpectedSchema() {
			cell := references.QualityNativeCell{Name: field.Name, DAOType: field.DAOType, IsNull: values[i] == nil, Value: values[i]}
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
	p.SnapshotSHA256 = sqlutil.BecHash(data)
	p.TypedCellsSHA256, err = references.QualityTypedHash(rows)
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
	want[0] = references.PlotQualityChoice{RowID: "1"}
	fraction, negativeZero, flag := math.Nextafter(1.2345678901234567, 2), math.Copysign(0, -1), true
	want[1].ItemOrder, want[1].Validate, want[1].Flag = &fraction, &flag, nil
	want[2].ItemOrder = &negativeZero
	want[3].Code = vprotest.QualityString(strings.Repeat("x", 16))
	want[4].Code = vprotest.QualityString("Good") // Duplicate Item remains a separate ordinal row.
	data, p := qualitySyntheticFixture(t, want)
	db := qualityImportDB(t)
	if err := references.ImportQualitySnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	got, err := references.QualityChoices(db)
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		references.QualityChoiceStatus(&want[i])
	}
	if !reflect.DeepEqual(got, want) || math.Float64bits(*got[2].ItemOrder) != math.Float64bits(negativeZero) {
		t.Fatalf("NULL/bool/exact Double/source ordinal changed: %#v", got)
	}
	if got[0].Selectable || got[0].Diagnostic == "" || got[3].Selectable || got[3].Diagnostic == "" {
		t.Fatal("unselectable source row has no explicit diagnostic")
	}
	if err := references.ImportQualitySnapshot(db, data, p); err == nil {
		t.Fatal("import collision overwrote catalogue")
	}
	again, _ := references.QualityChoices(db)
	if !reflect.DeepEqual(again, got) {
		t.Fatal("failed repeat import changed rows")
	}
}

func TestQualityImportProvenanceAndTamperingFailClosed(t *testing.T) {
	data, p := qualitySyntheticFixture(t, qualitySourceChoices())
	for _, change := range []func(*references.QualityProvenance){
		func(p *references.QualityProvenance) { p.SourceSHA256 = "bad" },
		func(p *references.QualityProvenance) { p.SnapshotSHA256 = strings.Repeat("0", 64) },
		func(p *references.QualityProvenance) { p.TypedCellsSHA256 = strings.Repeat("0", 64) },
		func(p *references.QualityProvenance) { p.Exporter = "unknown" },
		func(p *references.QualityProvenance) { p.SourceSchema[0].Name = "Invented" },
	} {
		bad := p
		bad.SourceSchema = append([]references.BecSourceColumn(nil), p.SourceSchema...)
		change(&bad)
		db := qualityImportDB(t)
		if err := references.ImportQualitySnapshot(db, data, bad); err == nil {
			t.Fatal("bad provenance accepted")
		}
		var tables int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('PlotQualityChoices','QualityProvenance')`).Scan(&tables); err != nil || tables != 0 {
			t.Fatal("rejected import left partial schema")
		}
	}

	db := qualityImportDB(t)
	if err := references.ImportQualitySnapshot(db, data, p); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`UPDATE PlotQualityChoices SET Note='wrong' WHERE RowID='1'`,
		`UPDATE QualityProvenance SET SourceSHA256='wrong'`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		if err := references.ValidateQualityDatabase(db, p); err == nil {
			t.Fatal("database tampering was accepted")
		}
	}
}

func TestQualityEmbeddedAndInstalledChecksumsFailClosed(t *testing.T) {
	oldDB, oldProvenance := references.QualityDatabase, references.QualityProvenanceJSON
	t.Cleanup(func() { references.QualityDatabase, references.QualityProvenanceJSON = oldDB, oldProvenance })
	references.QualityDatabase = append(append([]byte(nil), oldDB...), 0)
	if _, err := references.NewQualityService(t.TempDir()); err == nil || !strings.Contains(err.Error(), "embedded") {
		t.Fatalf("embedded corruption accepted: %v", err)
	}

	references.QualityDatabase = oldDB
	references.QualityProvenanceJSON = []byte(`{"unknown":true}`)
	if _, err := references.NewQualityService(t.TempDir()); err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("invalid provenance accepted: %v", err)
	}
	references.QualityProvenanceJSON = oldProvenance
	dir := t.TempDir()
	service, err := references.NewQualityService(dir)
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
	if _, err := references.NewQualityService(dir); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("installed corruption accepted: %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("corrupt installed catalogue was silently overwritten")
	}
}

func TestQualityImportRejectsMalformedSourceCells(t *testing.T) {
	data, original := qualitySyntheticFixture(t, qualitySourceChoices())
	for _, mutate := range []func(*references.QualityNativeSnapshot){
		func(s *references.QualityNativeSnapshot) { s.ReadOnly = false },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][0].DAOType = 7 },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][3].Name = "ScientificName" },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][3].Utf16CodeUnits = []uint16{0xD800} },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][2].Value = vprotest.QualityString("NaN") },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][7].Value = vprotest.QualityString("1") },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][6].IsNull = false },
		func(s *references.QualityNativeSnapshot) { s.Rows[0][2].Utf16CodeUnits = []uint16{} },
		func(s *references.QualityNativeSnapshot) { s.Source.After = strings.Repeat("0", 64) },
	} {
		var snapshot references.QualityNativeSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			t.Fatal(err)
		}
		mutate(&snapshot)
		bad, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		p := original
		p.SnapshotSHA256 = sqlutil.BecHash(bad)
		db := qualityImportDB(t)
		if err := references.ImportQualitySnapshot(db, bad, p); err == nil {
			t.Fatal("malformed source cell accepted")
		}
	}
}
