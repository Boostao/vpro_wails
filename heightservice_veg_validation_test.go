package main

import (
	"math"
	"reflect"
	"testing"
)

func setHeightVegProperty(record *VegRecord, property string, value *float64) {
	reflected := reflect.ValueOf(record).Elem()
	for _, field := range childFields["Veg"] {
		if childJSONKey(reflected, field) == property {
			reflected.FieldByName(field.member).Set(reflect.ValueOf(value))
			return
		}
	}
	panic("unknown test vegetation property")
}

func loadHeightVeg(t *testing.T, service *PlotService) VegRecord {
	t.Helper()
	rows, err := service.ListVegRecords("CHILD1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("load height vegetation row: %d %v", len(rows), err)
	}
	return rows[0]
}

func TestHeight_VegWritePathsRejectNewInvalidNumerics(t *testing.T) {
	for _, path := range []string{"create", "update", "legacy-save"} {
		t.Run(path, func(t *testing.T) {
			s, db := childFixture(t)
			s.SetAuditStrength(3)
			var baseline VegRecord
			if path != "create" {
				seedHeight(t, db, "CHILD1", -7)
				baseline = loadHeightVeg(t, s)
			}
			before := heightTableSnapshot(t, db, "Sample_Veg")
			audits := heightTableSnapshot(t, db, "Sample_Audit")
			for property := range heightProperties {
				invalid := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.5e38, -3.5e38}
				if property[:1] != "h" {
					invalid = append(invalid, 100, 125)
				}
				for _, number := range invalid {
					record := baseline
					if path == "create" {
						record = VegRecord{PlotNumber: "CHILD1", Species: "PSEUMEN"}
					}
					setHeightVegProperty(&record, property, heightFloat(number))
					var err error
					if path == "update" {
						err = s.UpdateVegRecord(record)
					} else {
						err = s.SaveVegRecord(record)
					}
					if err == nil {
						t.Fatalf("%s accepted %s=%v", path, property, number)
					}
				}
			}
			if heightTableSnapshot(t, db, "Sample_Veg") != before ||
				heightTableSnapshot(t, db, "Sample_Audit") != audits {
				t.Fatal("rejected vegetation writes changed data/audit")
			}
			var ledger int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='__VPRO_ChildIdentity'`).Scan(&ledger); err != nil || ledger != 0 {
				t.Fatalf("failed creation/update left reservations: %d %v", ledger, err)
			}
		})
	}
}

func TestHeight_VegHistoricalInvalidPreserveClearAndActualOldRead(t *testing.T) {
	for _, path := range []string{"update-zero", "legacy-save"} {
		t.Run(path, func(t *testing.T) {
			s, db := childFixture(t)
			s.SetAuditStrength(3)
			id := 0
			if path == "legacy-save" {
				id = -7
			}
			seedHeight(t, db, "CHILD1", id)
			if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=125,TotalA=100,Height1=1e40 WHERE PlotNumber='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			save := func(record VegRecord) error {
				if path == "update-zero" {
					return s.UpdateVegRecord(record)
				}
				return s.SaveVegRecord(record)
			}
			record := loadHeightVeg(t, s)
			if err := save(record); err != nil {
				t.Fatalf("finite historical invalid value not preserved: %v", err)
			}
			if auditCount(t, db, "CHILD1") != 0 {
				t.Fatal("unchanged invalid data was audited")
			}
			// The stale payload cannot claim its historical invalid field is
			// unchanged after the actual transaction read becomes valid.
			if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=20 WHERE PlotNumber='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			if err := save(record); err == nil {
				t.Fatal("stale DTO reintroduced invalid cover125")
			}
			record = loadHeightVeg(t, s)
			record.Cover1, record.TotalA, record.Height1 = nil, nil, nil
			if err := save(record); err != nil {
				t.Fatalf("historical invalid clearing failed: %v", err)
			}
			for _, field := range []string{"Cover1", "TotalA", "Height1"} {
				if value := storedHeight(t, db, "CHILD1", id, field); value != nil {
					t.Fatalf("%s not cleared", field)
				}
			}
			if auditCount(t, db, "CHILD1") != 3 {
				t.Fatal("clearing audits missing")
			}
		})
	}
}

func TestHeight_VegSharedBoundaryPreservesPrecisionAndLegacyFields(t *testing.T) {
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 0)
	record := loadHeightVeg(t, s)
	record.Cover1 = heightFloat(99.99)
	record.TotalA = heightFloat(-125)
	record.Height1 = heightFloat(1.234567890123)
	record.Height2 = heightFloat(-math.MaxFloat32)
	record.Height3 = heightFloat(math.Copysign(0, -1))
	// Covers outside the verified surface retain their existing finite-only
	// semantics; the new domain must not leak into these legacy attributes.
	record.Cover7 = heightFloat(1000)
	record.Cover10 = heightFloat(1e40)
	if err := s.UpdateVegRecord(record); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]float64{
		"Cover1": 99.99, "TotalA": -125, "Height1": 1.234567890123,
		"Height2": -math.MaxFloat32, "Height3": 0, "Cover7": 1000, "Cover10": 1e40,
	} {
		if got := storedHeight(t, db, "CHILD1", 0, field); got == nil || *got != want {
			t.Fatalf("%s precision/domain changed", field)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_normal_height_audit BEFORE INSERT ON Sample_Audit
		BEGIN SELECT RAISE(ABORT,'normal height audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	record = loadHeightVeg(t, s)
	record.Height1 = heightFloat(2.75)
	before := heightTableSnapshot(t, db, "Sample_Veg")
	audits := heightTableSnapshot(t, db, "Sample_Audit")
	if err := s.UpdateVegRecord(record); err == nil {
		t.Fatal("failing audit accepted normal update")
	}
	if before != heightTableSnapshot(t, db, "Sample_Veg") || audits != heightTableSnapshot(t, db, "Sample_Audit") {
		t.Fatal("normal update failure did not roll back")
	}
}
