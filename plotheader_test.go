package main

import (
	"database/sql"
	"math"
	"reflect"
	"strings"
	"testing"
)

func headerFixture(t *testing.T) (*PlotService, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	projects, err := NewProjectServiceWithConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewPlotService(projects)
	service.SetAuditStrength(3)
	service.SetCurrentUser("HeaderTester")
	db, _, err := service.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return service, db
}

func fullHeader(plot string) FS882Header {
	h := FS882Header{PlotNumber: plot}
	v := reflect.ValueOf(&h).Elem()
	for i := 1; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Pointer {
			continue
		}

		value := reflect.New(field.Type().Elem())
		if value.Elem().Kind() == reflect.Int {
			value.Elem().SetInt(int64(300 + i))
			if v.Type().Field(i).Name == "StartDate" {
				value.Elem().SetInt(2026)
			}
		} else if value.Elem().Kind() == reflect.Float64 {
			value.Elem().SetFloat(float64(300+i) + 0.25)
		} else if value.Elem().Kind() == reflect.Bool {
			value.Elem().SetBool(true)
		} else {
			text := "value-'quoted'-" + v.Type().Field(i).Tag.Get("json")
			if v.Type().Field(i).Name == "Date" {
				text = "2026-09-29"
			}
			switch v.Type().Field(i).Name {
			case "Zone":
				text = "BG"
			case "SubZone":
				text = "xh1"
			case "SiteSeries":
				text = "01"
			case "SitePlotQuality":
				text = "Good"
			case "VegPlotQuality":
				text = "Fair"
			case "SoilPlotQuality":
				text = "Poor"
			case "SiteDisturbance1", "SiteDisturbance2", "SiteDisturbance3":
				text = "manual"
			case "Exposure1", "Exposure2":
				text = "AT"
			case "FSRegionDistrict":
				text = "RCB.DCC"
			case "Ecosection":
				text = "ALR"
			case "SoilClassGroup":
				text = "1234"
			case "SoilClassSubGroup":
				text = "Z9"
			case "BedrockGeology1", "BedrockGeology2", "BedrockGeology3":
				text = "AB"
			}
			if isParentCodeProperty(v.Type().Field(i).Tag.Get("json")) {
				text = "X"
			}
			value.Elem().SetString(text)
		}
		field.Set(value)
	}
	return h
}

func auditCount(t *testing.T, db *sql.DB, plot string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber = ?`, plot).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPlotHeader_StrictCreateUpdate(t *testing.T) {
	service, db := headerFixture(t)
	h := fullHeader("HDRMODE")
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before, err := service.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	audits := auditCount(t, db, h.PlotNumber)
	replacement := FS882Header{PlotNumber: h.PlotNumber}
	if err := service.CreatePlot(replacement); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate create did not fail: %v", err)
	}
	after, err := service.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(before, after) || auditCount(t, db, h.PlotNumber) != audits {
		t.Fatalf("duplicate create changed existing data or audit: %#v %v", after, err)
	}
	missing := FS882Header{PlotNumber: "HDRGONE"}
	if err := service.UpdatePlot(missing); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("missing update did not fail: %v", err)
	}
	for _, table := range []string{"Env", "Admin"} {
		key := "PlotNumber"
		if table == "Admin" {
			key = "Plot"
		}
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_`+table+` WHERE `+key+` = ?`, missing.PlotNumber).Scan(&count); err != nil || count != 0 {
			t.Fatalf("missing update created %s row: %d %v", table, count, err)
		}
	}
}

func TestPlotHeader_NonFiniteCoordinates(t *testing.T) {
	service, db := headerFixture(t)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, mapping := range headerFields {
			field := mapping.member
			member, _ := reflect.TypeOf(FS882Header{}).FieldByName(field)
			if member.Type.Kind() != reflect.Pointer || member.Type.Elem().Kind() != reflect.Float64 {
				continue
			}
			h := FS882Header{PlotNumber: "HDRNAN"}
			reflect.ValueOf(&h).Elem().FieldByName(field).Set(reflect.ValueOf(&value))
			if err := service.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "finite") {
				t.Fatalf("%s accepted %v: %v", field, value, err)
			}
		}
	}
	if auditCount(t, db, "HDRNAN") != 0 {
		t.Fatal("invalid coordinates created audit rows")
	}
}

func TestPlotHeader_AllFieldsRoundTripAndAudit(t *testing.T) {
	s, db := headerFixture(t)
	caps, err := s.GetHeaderCapabilities()
	if err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeOf(FS882Header{})
	if len(caps) != typ.NumField() {
		t.Fatalf("capabilities %d, DTO fields %d", len(caps), typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		key := typ.Field(i).Tag.Get("json")
		supported, exists := caps[key]
		if !exists || supported != (key != "locked") {
			t.Errorf("capability %s: %v (exists %v)", key, supported, exists)
		}
	}
	h := fullHeader("HDRALL")
	if err := s.SavePlot(h); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, h) {
		t.Fatalf("roundtrip: got %#v, error %v", got, err)
	}
	// Independently assert persisted canonical aliases, not just symmetric
	// Save/Get behavior using the same mapping allowlist.
	wantColumns := map[string]any{
		"ProjectID": *h.ProjectID, "SiteSurveyor": *h.Surveyor, "Date": *h.Date,
		"FieldNumber": *h.FieldNo, "Location": *h.GeneralLocation, "NtsMapSheet": *h.MapSheet,
		"UTMZone": *h.UTMZone, "UTMEasting": *h.Easting, "UTMNorthing": *h.Northing,
		"LocationAccuracy": *h.Accuracy, "PlotRepresenting": *h.PlotRepresenting,
		"SiteSeries": *h.SiteSeries, "TransDistrib": *h.Transition, "MapUnit": *h.MapUnit,
		"MoistureRegime": *h.MoistureRegime, "NutrientRegime": *h.NutrientRegime,
		"SuccessionalStatus": *h.Successional, "StructuralStage": *h.StructuralStage,
		"StandAge": *h.StandAge, "Elevation": *h.Elevation, "SlopeGradient": *h.Slope,
		"Aspect": *h.Aspect, "MesoSlopePosition": *h.MesoSlopePos, "SurfaceShape": *h.SurfaceShape,
		"SurfaceTopographyType": *h.MicrotopType, "SurfaceTopographySize": *h.MicrotopSize,
		"SiteNotes": *h.FieldNotes, "OfficeNotes": *h.OfficeNotes, "StartDate": *h.StartDate,
	}
	for column, want := range wantColumns {
		table := "Sample_Env"
		key := "PlotNumber"
		if column == "OfficeNotes" || column == "StartDate" {
			table, key = "Sample_Admin", "Plot"
		}
		var actual string
		query := `SELECT CAST(` + quoteHeaderIdentifier(column) + ` AS TEXT) FROM ` +
			quoteHeaderIdentifier(table) + ` WHERE ` + quoteHeaderIdentifier(key) + ` = ?`
		number := 0.0
		numeric := true
		switch want := want.(type) {
		case int:
			number = float64(want)
		case float64:
			number = want
		default:
			numeric = false
		}
		if numeric {
			var actualNumber float64
			if err := db.QueryRow(query, h.PlotNumber).Scan(&actualNumber); err != nil {
				t.Fatal(err)
			}
			if actualNumber != number {
				t.Errorf("%s.%s = %v, want %v", table, column, actualNumber, number)
			}
			continue
		}
		if err := db.QueryRow(query, h.PlotNumber).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		if actual != headerAuditValue(want) {
			t.Errorf("%s.%s = %q, want %v", table, column, actual, want)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET Temporary = 'not header data' WHERE PlotNumber = ?`, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	fieldCount := typ.NumField() - 2 // identity and UI-only lock
	if count := auditCount(t, db, h.PlotNumber); count != fieldCount {
		t.Fatalf("initial audit count %d, want %d", count, fieldCount)
	}
	entries, err := s.ListAuditEntries(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.BeforeEdit != nil || entry.AfterEdit == nil || entry.User != "HeaderTester" ||
			entry.EditField == "PlotNumber" || entry.EditField == "Plot" {
			t.Fatalf("invalid initial audit: %#v", entry)
		}
	}
	if err := s.SavePlot(*got); err != nil {
		t.Fatal(err)
	}
	if count := auditCount(t, db, h.PlotNumber); count != fieldCount {
		t.Fatalf("unchanged save audited: %d", count)
	}
	var unmanaged string
	if err := db.QueryRow(`SELECT Temporary FROM Sample_Env WHERE PlotNumber = ?`, h.PlotNumber).Scan(&unmanaged); err != nil || unmanaged != "not header data" {
		t.Fatalf("unmanaged column changed: %q %v", unmanaged, err)
	}
	// Update every supported string/number, including Admin fields.
	updated := fullHeader(h.PlotNumber)
	v := reflect.ValueOf(&updated).Elem()
	for i := 1; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Pointer {
			continue
		}
		if f.Elem().Kind() == reflect.Int {
			f.Elem().SetInt(f.Elem().Int() + 1)
		} else if f.Elem().Kind() == reflect.Float64 {
			f.Elem().SetFloat(f.Elem().Float() + 0.125)
		} else if f.Elem().Kind() == reflect.Bool {
			f.Elem().SetBool(!f.Elem().Bool())
		} else {
			f.Elem().SetString(f.Elem().String() + "-updated")
			switch v.Type().Field(i).Name {
			case "Zone":
				f.Elem().SetString("CWH")
			case "SubZone":
				f.Elem().SetString("vh2")
			case "SiteSeries":
				f.Elem().SetString("Wm05")
			case "SiteDisturbance1", "SiteDisturbance2", "SiteDisturbance3":
				f.Elem().SetString("changed")
			case "Exposure1", "Exposure2":
				f.Elem().SetString("NA")
			case "FSRegionDistrict":
				f.Elem().SetString("RCB.DMH")
			case "Ecosection":
				f.Elem().SetString("BAU")
			case "SoilClassGroup":
				f.Elem().SetString("5678")
			case "SoilClassSubGroup":
				f.Elem().SetString("Q2")
			case "BedrockGeology1", "BedrockGeology2", "BedrockGeology3":
				f.Elem().SetString("aB")
			}
			if isParentCodeProperty(v.Type().Field(i).Tag.Get("json")) {
				f.Elem().SetString("x")
			}
		}
	}
	if err := s.SavePlot(updated); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, updated) {
		t.Fatalf("updated roundtrip: %#v, %v", got, err)
	}
	if count := auditCount(t, db, h.PlotNumber); count != 2*fieldCount {
		t.Fatalf("update audit count: %d", count)
	}
	if err := s.SavePlot(FS882Header{PlotNumber: h.PlotNumber}); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, FS882Header{PlotNumber: h.PlotNumber}) {
		t.Fatalf("clear roundtrip: %#v, %v", got, err)
	}
	var cleared int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber = ? AND BeforeEdit IS NOT NULL AND AfterEdit IS NULL`, h.PlotNumber).Scan(&cleared); err != nil {
		t.Fatal(err)
	}
	if cleared != fieldCount || auditCount(t, db, h.PlotNumber) != 3*fieldCount {
		t.Fatalf("clear-to-null audit count: %d", cleared)
	}
}

func TestPlotHeader_AuditStrengths(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(string(rune('0'+strength)), func(t *testing.T) {
			s, db := headerFixture(t)
			if err := s.SetAuditStrength(strength); err != nil {
				t.Fatal(err)
			}
			h := FS882Header{PlotNumber: "HDRSTR"}
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			n := 0
			text := "" // non-null zero and empty text are values, not SQL NULL
			h.Elevation, h.OfficeNotes = &n, &text
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			want := 0
			if strength >= 2 {
				want += 2
			}
			if count := auditCount(t, db, h.PlotNumber); count != want {
				t.Fatalf("NULL->value audit count %d, want %d", count, want)
			}
			nextN, nextText := 12, "office"
			h.Elevation, h.OfficeNotes = &nextN, &nextText
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if strength >= 1 {
				want += 2
			}
			if count := auditCount(t, db, h.PlotNumber); count != want {
				t.Fatalf("value->value audit count %d, want %d", count, want)
			}
			h.Elevation, h.OfficeNotes = nil, nil
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if strength == 3 {
				want += 2
			}
			if count := auditCount(t, db, h.PlotNumber); count != want {
				t.Fatalf("value->NULL audit count %d, want %d", count, want)
			}
		})
	}
}

func TestPlotHeader_UnsupportedSchemaAndValues(t *testing.T) {
	s, db := headerFixture(t)
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'view'`)
	if err != nil {
		t.Fatal(err)
	}
	var views []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		views = append(views, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		if _, err := db.Exec(`DROP VIEW ` + quoteHeaderIdentifier(view)); err != nil {
			t.Fatal(err)
		}
	}
	// English-looking aliases and lock columns are NOT evidence-backed mappings.
	if _, err := db.Exec(`DROP TABLE Sample_Env; DROP TABLE Sample_Admin;
		CREATE TABLE Sample_Env (PlotNumber TEXT PRIMARY KEY, Elevation INTEGER, MapSheet TEXT, Locked INTEGER, UserSiteUnit TEXT);
		CREATE TABLE Sample_Admin (Plot TEXT UNIQUE, StartDate INTEGER, SoilNotes TEXT, SpeciesListComplete BOOLEAN)`); err != nil {
		t.Fatal(err)
	}
	caps, err := s.GetHeaderCapabilities()
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range headerFields {
		want := field.property == "plotNumber" || field.property == "elevation" || field.property == "startDate"
		if caps[field.property] != want {
			t.Errorf("%s capability %v, want %v", field.property, caps[field.property], want)
		}
		if want {
			continue
		}
		h := FS882Header{PlotNumber: "HDRUNS"}
		f := reflect.ValueOf(&h).Elem().FieldByName(field.member)
		f.Set(reflect.ValueOf(fullHeader(h.PlotNumber)).FieldByName(field.member))
		if field.property == "locked" {
			f.SetBool(true)
		}
		if err := s.SavePlot(h); err == nil || !strings.Contains(err.Error(), field.property) {
			t.Errorf("nondefault unsupported %s: %v", field.property, err)
		}
		if count := auditCount(t, db, h.PlotNumber); count != 0 {
			t.Fatal("rejected input left audit writes")
		}
	}
	h := FS882Header{PlotNumber: "HDRUNS"}
	empty, zero := "", 0
	zeroFloat := 0.0
	for _, unsupported := range []FS882Header{
		{PlotNumber: h.PlotNumber, MapSheet: &empty},
		{PlotNumber: h.PlotNumber, Accuracy: &zero},
		{PlotNumber: h.PlotNumber, Easting: &zeroFloat},
		{PlotNumber: h.PlotNumber, Northing: &zeroFloat},
		{PlotNumber: h.PlotNumber, Slope: &zeroFloat},
		{PlotNumber: h.PlotNumber, SpeciesListComplete: boolPointer(false)},
		{PlotNumber: h.PlotNumber, UpdatedFromCards: boolPointer(false)},
	} {
		if err := s.SavePlot(unsupported); err == nil {
			t.Fatalf("explicit unsupported zero/empty value was silently discarded: %#v", unsupported)
		}
	}
	if err := s.SavePlot(h); err != nil {
		t.Fatalf("unsupported nil/default values: %v", err)
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, FS882Header{PlotNumber: h.PlotNumber}) {
		t.Fatalf("unsupported values must remain nil/default: %#v %v", got, err)
	}
	if _, err := db.Exec(`ALTER TABLE Sample_Admin RENAME COLUMN Plot TO UnknownIdentity`); err != nil {
		t.Fatal(err)
	}
	caps, err = s.GetHeaderCapabilities()
	if err != nil || caps["plotNumber"] {
		t.Fatalf("missing identity capability: %#v %v", caps, err)
	}
	if err := s.SavePlot(FS882Header{PlotNumber: "HDRBAD"}); err == nil {
		t.Fatal("save allowed without verified Admin identity")
	}
	if _, err := s.GetPlot("HDRUNS"); err == nil {
		t.Fatal("read allowed without verified Admin identity")
	}
}

func TestPlotHeader_Rollback(t *testing.T) {
	for _, failure := range []string{"collision", "missing-admin", "admin-insert", "admin-update", "audit-insert"} {
		t.Run(failure, func(t *testing.T) {
			s, db := headerFixture(t)
			h := fullHeader("HDRFAIL")
			if failure == "admin-update" {
				if err := s.SavePlot(h); err != nil {
					t.Fatal(err)
				}

				if _, err := db.Exec(`CREATE TRIGGER fail_admin BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT, 'admin update failure'); END`); err != nil {
					t.Fatal(err)
				}
				h.GeneralLocation = nil
			} else {
				query := ""
				switch failure {
				case "collision":
					query = `PRAGMA foreign_keys=OFF; INSERT INTO Sample_Admin (Plot,OfficeNotes) VALUES ('HDRFAIL','existing'); PRAGMA foreign_keys=ON`
				case "missing-admin":
					query = `INSERT INTO Sample_Env (PlotNumber,Location) VALUES ('HDRFAIL','existing')`
				case "admin-insert":
					query = `CREATE TRIGGER fail_admin BEFORE INSERT ON Sample_Admin BEGIN SELECT RAISE(ABORT, 'admin insert failure'); END`
				case "audit-insert":
					query = `CREATE TRIGGER fail_audit BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT, 'audit insert failure'); END`
				}
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			beforeAudit := auditCount(t, db, h.PlotNumber)
			if err := s.SavePlot(h); err == nil {
				t.Fatal("expected failure")
			}
			var envCount, adminCount int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Env WHERE PlotNumber = ?`, h.PlotNumber).Scan(&envCount); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Admin WHERE Plot = ?`, h.PlotNumber).Scan(&adminCount); err != nil {
				t.Fatal(err)
			}
			wantEnv, wantAdmin := 0, 0
			switch failure {
			case "collision":
				wantAdmin = 1
			case "missing-admin":
				wantEnv = 1
			case "admin-update":
				wantEnv, wantAdmin = 1, 1
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(*got, fullHeader(h.PlotNumber)) {
					t.Fatalf("partial header persisted: %#v %v", got, err)
				}
			}
			if envCount != wantEnv || adminCount != wantAdmin || auditCount(t, db, h.PlotNumber) != beforeAudit {
				t.Fatalf("partial save: Env %d Admin %d", envCount, adminCount)
			}
		})
	}
}
