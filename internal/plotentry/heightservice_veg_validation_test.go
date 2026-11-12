package plotentry

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"math"
	"reflect"
	"testing"
)

func setHeightVegProperty(record *vprocore.VegRecord, property string, value *float64) {
	reflected := reflect.ValueOf(record).Elem()
	for _, field := range vprocore.ChildFields["Veg"] {
		if vprocore.ChildJSONKey(reflected, field) == property {
			reflected.FieldByName(field.Member).Set(reflect.ValueOf(value))
			return
		}
	}
	panic("unknown test vegetation property")
}

func loadHeightVeg(t *testing.T, service *vprocore.PlotService) vprocore.VegRecord {
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
			s, db := vprotest.ChildFixture(t)
			s.SetAuditStrength(3)
			var baseline vprocore.VegRecord
			if path != "create" {
				vprotest.SeedHeight(t, db, "CHILD1", -7)
				baseline = loadHeightVeg(t, s)
			}
			before := vprotest.HeightTableSnapshot(t, db, "Sample_Veg")
			audits := vprotest.HeightTableSnapshot(t, db, "Sample_Audit")
			for property := range vprocore.HeightProperties {
				invalid := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.5e38, -3.5e38}
				if property[:1] != "h" {
					invalid = append(invalid, 100, 125)
				}
				for _, number := range invalid {
					record := baseline
					if path == "create" {
						record = vprocore.VegRecord{PlotNumber: "CHILD1", Species: "PSEUMEN"}
					}
					setHeightVegProperty(&record, property, vprotest.HeightFloat(number))
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
			if vprotest.HeightTableSnapshot(t, db, "Sample_Veg") != before ||
				vprotest.HeightTableSnapshot(t, db, "Sample_Audit") != audits {
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
			s, db := vprotest.ChildFixture(t)
			s.SetAuditStrength(3)
			id := 0
			if path == "legacy-save" {
				id = -7
			}
			vprotest.SeedHeight(t, db, "CHILD1", id)
			if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=125,TotalA=100,Height1=1e40 WHERE PlotNumber='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			save := func(record vprocore.VegRecord) error {
				if path == "update-zero" {
					return s.UpdateVegRecord(record)
				}
				return s.SaveVegRecord(record)
			}
			record := loadHeightVeg(t, s)
			if err := save(record); err != nil {
				t.Fatalf("finite historical invalid value not preserved: %v", err)
			}
			if vprotest.AuditCount(t, db, "CHILD1") != 0 {
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
				if value := vprotest.StoredHeight(t, db, "CHILD1", id, field); value != nil {
					t.Fatalf("%s not cleared", field)
				}
			}
			if vprotest.AuditCount(t, db, "CHILD1") != 3 {
				t.Fatal("clearing audits missing")
			}
		})
	}
}

func TestHeight_VegSharedBoundaryPreservesPrecisionAndHistoricalFields(t *testing.T) {
	s, db := vprotest.ChildFixture(t)
	vprotest.SeedHeight(t, db, "CHILD1", 0)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover10=1e40 WHERE PlotNumber='CHILD1';
		CREATE TRIGGER forbid_historical_cover10 BEFORE UPDATE OF Cover10 ON Sample_Veg
		BEGIN SELECT RAISE(ABORT,'unchanged history must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	record := loadHeightVeg(t, s)
	record.Cover1 = vprotest.HeightFloat(99.99)
	record.TotalA = vprotest.HeightFloat(-125)
	record.Height1 = vprotest.HeightFloat(1.234567890123)
	record.Height2 = vprotest.HeightFloat(-math.MaxFloat32)
	record.Height3 = vprotest.HeightFloat(math.Copysign(0, -1))
	// Physical Single bounds do not introduce the height grid's <100 rule here.
	// An unchanged historical overflow is omitted, not reassigned or repaired.
	record.Cover7 = vprotest.HeightFloat(1000)
	if err := s.UpdateVegRecord(record); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]float64{
		"Cover1": 99.99, "TotalA": -125, "Height1": 1.234567890123,
		"Height2": -math.MaxFloat32, "Height3": 0, "Cover7": 1000, "Cover10": 1e40,
	} {
		if got := vprotest.StoredHeight(t, db, "CHILD1", 0, field); got == nil || *got != want {
			t.Fatalf("%s precision/domain changed", field)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_normal_height_audit BEFORE INSERT ON Sample_Audit
		BEGIN SELECT RAISE(ABORT,'normal height audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	record = loadHeightVeg(t, s)
	record.Height1 = vprotest.HeightFloat(2.75)
	before := vprotest.HeightTableSnapshot(t, db, "Sample_Veg")
	audits := vprotest.HeightTableSnapshot(t, db, "Sample_Audit")
	if err := s.UpdateVegRecord(record); err == nil {
		t.Fatal("failing audit accepted normal update")
	}
	if before != vprotest.HeightTableSnapshot(t, db, "Sample_Veg") || audits != vprotest.HeightTableSnapshot(t, db, "Sample_Audit") {
		t.Fatal("normal update failure did not roll back")
	}
}
