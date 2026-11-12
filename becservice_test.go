package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func becTestDir(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(".", ".bec-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	return root
}

func becFixture(t *testing.T) *BECService {
	t.Helper()
	s, err := NewBECService(becTestDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func becString(value string) *string { return &value }

func TestBECFrozenCatalogueAndLookups(t *testing.T) {
	s := becFixture(t)
	status, err := s.GetBECCatalogueStatus(context.Background())
	if err != nil || status.Version != 2 || status.ZoneRows != 331 || status.SiteSeriesRows != 8900 ||
		status.DuplicateKeys != 2253 || status.UnselectableRows != 67 ||
		status.SourceSHA256 != "7b19bd03e0a7cf0a9df453118fe42d8fecd9433f9c85634df3e556e6632161ba" ||
		status.SnapshotSHA256 != "568c964b10f7ea83201a793bbe2c050fa6df45641a4640fab8a55a602adca814" ||
		becHash(becDatabase) != "6f106e2af85b47869780f4ce0645ee9c4dafff2442059677bfe9cfa061f91b0e" {
		t.Fatalf("catalogue status %#v: %v", status, err)
	}
	zones, err := s.ListBECZones(context.Background())
	if err != nil || len(zones) != 17 {
		t.Fatalf("zones %d: %v", len(zones), err)
	}
	all, err := s.ListBECSubZones(context.Background(), nil)
	if err != nil || len(all) != 331 {
		t.Fatalf("all subzones %d: %v", len(all), err)
	}
	upper, err := s.ListBECSubZones(context.Background(), becString("BG"))
	if err != nil || len(upper) == 0 {
		t.Fatalf("BG subzones: %v", err)
	}
	lower, err := s.ListBECSubZones(context.Background(), becString("bg"))
	if err != nil || !reflect.DeepEqual(upper, lower) {
		t.Fatalf("ASCII lookup changed raw subzones: %v", err)
	}
	series, err := s.ListBECSiteSeries(context.Background(), becString("BG"), becString("xh1"))
	if err != nil || len(series) != 58 {
		t.Fatalf("BG xh1 rows %d: %v", len(series), err)
	}
	mixed, err := s.ListBECSiteSeries(context.Background(), becString("bg"), becString("XH1"))
	if err != nil || !reflect.DeepEqual(series, mixed) {
		t.Fatalf("ASCII lookup changed raw definitions: %v", err)
	}
	var leadingZero, v13Unit, duplicate bool
	codes := map[string]int{}
	for _, row := range series {
		if row.SiteSeries == nil || row.SourceID == nil || !row.Selectable || row.Diagnostic != "" {
			t.Fatalf("incomplete raw definition: %#v", row)
		}
		codes[*row.SiteSeries]++
		leadingZero = leadingZero || *row.SiteSeries == "01"
		v13Unit = v13Unit || *row.SiteSeries == "101"
		duplicate = duplicate || codes[*row.SiteSeries] > 1
	}
	if !leadingZero || !v13Unit || !duplicate {
		t.Fatalf("lost historical leading zeros, v13 101 unit, or duplicates: %#v", codes)
	}
	v13Haida, err := s.ListBECSiteSeries(context.Background(), becString("CWH"), becString("vh3"))
	if err != nil || len(v13Haida) == 0 {
		t.Fatalf("missing BECdb v13 CWH vh3 units: %d %v", len(v13Haida), err)
	}
	emptyCodes, err := s.ListBECSiteSeries(context.Background(), becString("CMA"), becString("wh"))
	if err != nil {
		t.Fatal(err)
	}
	var unselectable, selectableCMA int
	for _, row := range emptyCodes {
		if row.SiteSeries != nil && *row.SiteSeries == "" {
			unselectable++
			if row.Selectable || row.Diagnostic == "" {
				t.Fatalf("empty source code not marked: %#v", row)
			}
		} else if row.Selectable {
			selectableCMA++
		}
	}
	if unselectable == 0 || selectableCMA == 0 {
		t.Fatalf("CMA wh missing preserved unselectable (%d) or v13 selectable (%d) rows", unselectable, selectableCMA)
	}
	for _, identity := range []struct {
		source, zone, subZone, code string
	}{
		{"64", "ESSF", "xv1", "07"},
		{"6698", "BG", "xh1", "101"},
		{"1593", "SWB", "mk", "09"},
	} {
		rows, err := s.ListBECSiteSeries(context.Background(), &identity.zone, &identity.subZone)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.SourceID != nil && *row.SourceID == identity.source && row.SiteSeries != nil && *row.SiteSeries == identity.code {
				found = true
			}
		}
		if !found {
			t.Fatalf("source row %s (%s %s /%s) missing", identity.source, identity.zone, identity.subZone, identity.code)
		}
	}
	for _, code := range []string{"All", "ZZ", "O'Q"} {
		rows, err := s.ListBECSubZones(context.Background(), &code)
		if err != nil || len(rows) != 0 || rows == nil {
			t.Fatalf("literal/no-match subzones %q: %#v %v", code, rows, err)
		}
	}
	for _, pair := range [][2]*string{
		{nil, nil}, {nil, becString("xh1")}, {becString("BG"), nil},
		{becString("ZZ"), becString("xh1")}, {becString("BG"), becString("O'Q")}, {becString("All"), becString("xh1")},
	} {
		rows, err := s.ListBECSiteSeries(context.Background(), pair[0], pair[1])
		if err != nil || len(rows) != 0 || rows == nil {
			t.Fatalf("NULL/no-match series: %#v %v", rows, err)
		}
	}
	for _, code := range []string{"", "ABCDE"} {
		if _, err := s.ListBECSubZones(context.Background(), &code); err == nil {
			t.Fatalf("invalid Zone %q accepted", code)
		}
	}
	for _, code := range []string{"", "123456789"} {
		if _, err := s.ListBECSiteSeries(context.Background(), becString("BG"), &code); err == nil {
			t.Fatalf("invalid SubZone %q accepted", code)
		}
	}
	if _, err := s.db.Exec(`DELETE FROM BECZoneList`); err == nil {
		t.Fatal("catalogue connection is writable")
	}
}

func TestBECExistingFileAndCloseGuards(t *testing.T) {
	root := becTestDir(t)
	existingReference := []byte("existing user vlists file")
	if err := os.WriteFile(filepath.Join(root, "vlists.db"), existingReference, 0644); err != nil {
		t.Fatal(err)
	}
	s, err := NewBECService(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListBECZones(context.Background()); err == nil {
		t.Fatal("closed lookup succeeded")
	}
	if _, err := s.ListBECSubZones(context.Background(), nil); err == nil {
		t.Fatal("closed subzone lookup succeeded")
	}
	if _, err := s.ListBECSiteSeries(context.Background(), nil, nil); err == nil {
		t.Fatal("closed NULL lookup succeeded")
	}
	if _, err := s.GetBECCatalogueStatus(context.Background()); err == nil {
		t.Fatal("closed status succeeded")
	}
	data, _ := os.ReadFile(filepath.Join(root, "vlists.db"))
	if string(data) != string(existingReference) {
		t.Fatal("existing ReferenceService data replaced")
	}
	s, err = NewBECService(root)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	corrupt := []byte("unknown historical bec.db")
	if err := os.WriteFile(filepath.Join(root, "bec.db"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBECService(root); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("unknown user catalogue not rejected: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(root, "bec.db"))
	if string(data) != string(corrupt) {
		t.Fatal("unknown user catalogue silently overwritten")
	}
}

func becRoundTripSnapshot(t *testing.T) (becSnapshot, becProvenance) {
	t.Helper()
	s := becFixture(t)
	var provenance becProvenance
	if err := json.Unmarshal(becProvenanceJSON, &provenance); err != nil {
		t.Fatal(err)
	}
	zones, series := becExpectedSchema()
	snapshot := becSnapshot{Version: provenance.Version, Exporter: provenance.Exporter,
		SourceAccessSHA256: provenance.SourceSHA256, ZoneSchema: zones, SeriesSchema: series}
	rows, err := s.db.Query(`SELECT RowID,Zone,SubZone,ZoneDescription,Description FROM BECZoneList ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var row BECSubZone
		if err := rows.Scan(&row.RowID, &row.Zone, &row.SubZone, &row.ZoneDescription, &row.Description); err != nil {
			t.Fatal(err)
		}
		snapshot.Zones = append(snapshot.Zones, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT ` + becSeriesColumns + ` FROM BECSiteSeries ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var row BECSiteSeries
		if err := rows.Scan(becSeriesScan(&row)...); err != nil {
			t.Fatal(err)
		}
		snapshot.SiteSeries = append(snapshot.SiteSeries, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return snapshot, provenance
}

func becMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestBECImportAllMetadataAndNullableSignedIdentities(t *testing.T) {
	snapshot, provenance := becRoundTripSnapshot(t)
	snapshot.SiteSeries[0].SourceID = becString("-1.25")
	snapshot.SiteSeries[0].OriginalSourceID = becString("0")
	snapshot.SiteSeries[0].Flag = nil
	snapshot.SiteSeries[0].ReferenceID = nil
	snapshot.SiteSeries[0].AddedDate = nil
	snapshot.SiteSeries[0].Comments = nil
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	provenance.SnapshotSHA256 = becHash(data)
	db := becMemory(t)
	if err := importBECSnapshot(db, data, provenance); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT ` + becSeriesColumns + ` FROM BECSiteSeries ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	for rows.Next() {
		var got BECSiteSeries
		if err := rows.Scan(becSeriesScan(&got)...); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, snapshot.SiteSeries[i]) {
			t.Fatalf("raw metadata changed at %d: %#v / %#v", i, got, snapshot.SiteSeries[i])
		}
		i++
	}
	if err := rows.Err(); err != nil || i != len(snapshot.SiteSeries) {
		t.Fatalf("imported rows %d: %v", i, err)
	}
	rows.Close()
	firstRow := snapshot.SiteSeries[0]
	lastRow := snapshot.SiteSeries[len(snapshot.SiteSeries)-1]
	// Ensure firstRow has non-empty Zone & SubZone for lookup test; pick first non-empty SubZone row if RowID 1 has empty SubZone
	targetFirst := firstRow
	for _, cand := range snapshot.SiteSeries {
		if cand.Zone != nil && *cand.Zone != "" && cand.SubZone != nil && *cand.SubZone != "" {
			targetFirst = cand
			break
		}
	}
	if _, err := db.Exec(`UPDATE BECSiteSeries SET SiteSeries=NULL WHERE RowID=?;
 UPDATE BECSiteSeries SET SiteSeries='' WHERE RowID=?`, targetFirst.RowID, lastRow.RowID); err != nil {
		t.Fatal(err)
	}
	service := &BECService{db: db}
	for _, pair := range [][2]string{{*targetFirst.Zone, *targetFirst.SubZone}, {*lastRow.Zone, *lastRow.SubZone}} {
		rows, err := service.ListBECSiteSeries(context.Background(), &pair[0], &pair[1])
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range rows {
			if row.RowID == targetFirst.RowID || row.RowID == lastRow.RowID {
				found = true
				if row.Selectable || row.Diagnostic == "" {
					t.Fatalf("NULL/empty code not explicitly unselectable: %#v", row)
				}
			}
		}
		if !found {
			t.Fatal("NULL/empty code was filtered out")
		}
	}
}

func TestBECV1SnapshotValidationPreserved(t *testing.T) {
	zones, series := becExpectedSchema()
	v1Snap := becSnapshot{
		Version:            1,
		Exporter:           becExporterV1,
		SourceAccessSHA256: strings.Repeat("a", 64),
		ZoneSchema:         zones,
		SeriesSchema:       series,
		Zones:              []BECSubZone{{RowID: "1", Zone: becString("BG"), SubZone: becString("xh1"), ZoneDescription: becString("Bunchgrass"), Description: becString("Okanagan Very Dry Hot")}},
		SiteSeries:         []BECSiteSeries{{RowID: "1", SourceID: becString("64"), Zone: becString("BG"), SubZone: becString("xh1"), SiteSeries: becString("01"), AddedDate: becString("2000-10-11T00:00:00")}},
	}
	raw, err := json.Marshal(v1Snap)
	if err != nil {
		t.Fatal(err)
	}
	p := becProvenance{
		Version:        1,
		Exporter:       becExporterV1,
		SourceSHA256:   strings.Repeat("a", 64),
		SnapshotSHA256: becHash(raw),
		ZoneRows:       1,
		SiteSeriesRows: 1,
		DuplicateKeys:  0,
	}
	db := becMemory(t)
	if err := importBECSnapshot(db, raw, p); err != nil {
		t.Fatalf("v1 snapshot import failed: %v", err)
	}
	status, err := validateBECDatabase(db, p)
	if err != nil || status.Version != 1 || status.ZoneRows != 1 || status.SiteSeriesRows != 1 {
		t.Fatalf("v1 validation mismatch: %+v %v", status, err)
	}
}

func TestBECImportGuardsAndRollback(t *testing.T) {
	snapshot, provenance := becRoundTripSnapshot(t)
	for _, scenario := range []string{"checksum", "schema", "identity", "count", "source", "duplicate-count", "invalid-double", "date"} {
		t.Run(scenario, func(t *testing.T) {
			copyData, _ := json.Marshal(snapshot)
			var copySnapshot becSnapshot
			json.Unmarshal(copyData, &copySnapshot)
			p := provenance
			switch scenario {
			case "schema":
				copySnapshot.SeriesSchema[18].DAOType = 8
			case "identity":
				copySnapshot.SiteSeries[1].RowID = "1"
			case "count":
				copySnapshot.Zones = copySnapshot.Zones[1:]
			case "source":
				copySnapshot.SourceAccessSHA256 = strings.Repeat("0", 64)
			case "duplicate-count":
				p.DuplicateKeys++
			case "invalid-double":
				copySnapshot.SiteSeries[0].SourceID = becString("NaN")
			case "date":
				copySnapshot.SiteSeries[0].AddedDate = becString("10/11/2000")
			}
			data, _ := json.Marshal(copySnapshot)
			p.SnapshotSHA256 = becHash(data)
			if scenario == "checksum" {
				p.SnapshotSHA256 = strings.Repeat("0", 64)
			}
			db := becMemory(t)
			if err := importBECSnapshot(db, data, p); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed import left schema/data: %d %v", count, err)
			}
		})
	}
}

func TestBECFreezeNativeSnapshot(t *testing.T) {
	source := os.Getenv("BEC_NATIVE_SNAPSHOT")
	destination := os.Getenv("BEC_FREEZE_DESTINATION")
	if source == "" || destination == "" {
		t.Skip("one-time, explicitly requested native snapshot import")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	var provenance becProvenance
	if err := json.Unmarshal(becProvenanceJSON, &provenance); err != nil {
		t.Fatal(err)
	}
	if existing, err := os.Stat(destination); err == nil {
		if existing.Size() != 0 {
			t.Fatal("refusing to overwrite a nonempty frozen database")
		}
		if err := os.Remove(destination); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite3", destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := importBECSnapshot(db, data, provenance); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("imported native snapshot %s to %s", provenance.SnapshotSHA256, destination)
}
