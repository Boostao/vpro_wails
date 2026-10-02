package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestVegetationNumbersSourceFieldsAtomicRollbackAndHistoricalOmission(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,Cover6,Cover7)
		VALUES ('CHILD1','RAW',0,0,0,0),('CHILD1','RAW',-9,0,0,0),('CHILD1','RAW',-10,NULL,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	number := func(value float64) *float64 { return &value }
	makeUpdate := func(id int, form, property string, expected, value *float64) VegetationNumberUpdate {
		return VegetationNumberUpdate{id, map[string]*float64{property: value},
			map[string]*float64{property: expected}, map[string]string{property: form}}
	}
	for form, properties := range vegetationNumberForms {
		for property := range properties {
			var stored *float64
			if err := db.QueryRow(`SELECT ` + quoteHeaderIdentifier(property) + ` FROM Sample_Veg WHERE ID=0`).Scan(&stored); err != nil {
				t.Fatal(err)
			}
			if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{
				makeUpdate(0, form, property, stored, number(-1.234567890123)),
			}); err != nil {
				t.Fatalf("%s.%s: %v", form, property, err)
			}
		}
	}
	if auditCount(t, db, "CHILD1") != 17 {
		t.Fatal("source partners generated phantom numeric audits")
	}
	before := substrateSnapshot(t, db)
	for _, update := range []VegetationNumberUpdate{
		makeUpdate(0, "SubVegCXL", "cover1", number(-1.234567890123), number(2)),
		makeUpdate(0, "SubVegDXL", "height6", number(-1.234567890123), number(2)),
		makeUpdate(0, "SubVegAXL", "cover1", number(-1.234567890123), number(2)),
		makeUpdate(-10, "SubVegCXL", "cover6", nil, number(0)),
		makeUpdate(-10, "SubVegDXL", "cover7", nil, number(0)),
		makeUpdate(-10, "SubVegAhtXL", "height1", nil, number(0)),
		makeUpdate(0, "SubVegAXL_BC", "cover1", number(0), number(2)),
		makeUpdate(0, "SubVegDXL", "cover7", number(-1.234567890123), number(100)),
		makeUpdate(0, "SubVegAhtXL", "height1", number(-1.234567890123), number(math.Inf(1))),
		makeUpdate(2147483648, "SubVegAXL_BC", "cover1", nil, number(2)),
	} {
		if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{update}); err == nil {
			t.Fatal("unavailable source/identity/value accepted:", update)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	first := makeUpdate(0, "SubVegAXL_BC", "cover1", number(-1.234567890123), number(99.999))
	second := makeUpdate(-9, "SubVegDXL", "cover9", nil, number(-3.5))
	if _, err := db.Exec(`CREATE TRIGGER fail_number_audit BEFORE INSERT ON Sample_Audit WHEN NEW.ID=-9
		BEGIN SELECT RAISE(ABORT,'injected numeric audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{first, second}); err == nil {
		t.Fatal("partially committed numeric batch")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_number_audit`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{first, second}); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{first, first}); err == nil {
		t.Fatal("duplicate identities accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover8=120 WHERE ID=-9;
		CREATE TRIGGER prevent_historical_number BEFORE UPDATE OF Cover8 ON Sample_Veg
		WHEN OLD.ID=-9 BEGIN SELECT RAISE(ABORT,'historical field reassigned'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{
		{ID: -9, Values: map[string]*float64{"cover8": number(120), "cover9": nil},
			Expected: map[string]*float64{"cover8": number(120), "cover9": number(-3.5)},
			Forms:    map[string]string{"cover8": "SubVegDXL", "cover9": "SubVegDXL"}},
	}); err != nil {
		t.Fatal("historical omission or nullable correction failed:", err)
	}
}

func TestVegetationNumbersExplicitTransportAndMatchingKeys(t *testing.T) {
	s, db := childFixture(t)
	before := substrateSnapshot(t, db)
	for _, payload := range []string{
		`{"values":{"cover1":1},"expected":{"cover1":null},"forms":{"cover1":"SubVegAXL_BC"}}`,
		`{"id":null,"values":{"cover1":1},"expected":{"cover1":null},"forms":{"cover1":"SubVegAXL_BC"}}`,
		`{"id":1.5,"values":{"cover1":1},"expected":{"cover1":null},"forms":{"cover1":"SubVegAXL_BC"}}`,
	} {
		var update VegetationNumberUpdate
		if err := json.Unmarshal([]byte(payload), &update); err == nil {
			t.Fatal("implicit/invalid identity accepted:", payload)
		}
	}
	for _, payload := range []string{
		`{"id":0,"values":{},"expected":{},"forms":{}}`,
		`{"id":0,"values":{"cover1":1},"expected":{},"forms":{"cover1":"SubVegAXL_BC"}}`,
		`{"id":0,"values":{"cover1":1},"expected":{"cover1":null},"forms":null}`,
		`{"id":0,"values":{"cover1":1},"expected":{"cover1":null},"forms":{"cover2":"SubVegAXL_BC"}}`,
		`{"id":0,"values":{"cover1":1},"expected":{"height1":null},"forms":{"cover1":"SubVegAXL_BC"}}`,
	} {
		var update VegetationNumberUpdate
		if err := json.Unmarshal([]byte(payload), &update); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{update}); err == nil {
			t.Fatal("nonmatching source transport accepted:", payload)
		}
		assertSubstrateSnapshot(t, db, before)
	}
}
