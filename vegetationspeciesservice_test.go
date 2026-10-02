package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func TestVegetationSpeciesCanonicalContextUsesReadonlyFamilyNotProjectWriter(t *testing.T) {
	service, state := contextServiceFixture(t)
	options, err := service.ListVegetationSpecies(context.Background(), state.ContextID, "SubVegAXL_BC")
	if err != nil || len(options) == 0 || options[0].Code == nil {
		t.Fatal("canonical source list unavailable:", err)
	}
	files := map[string][]byte{}
	for _, path := range service.projects.sqlite.attachments {
		if path == state.ProjectPath {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = content
	}
	if err := service.CreatePlot(state.ContextID, FS882Header{PlotNumber: "SPECIES1"}); err != nil {
		t.Fatal(err)
	}
	service.plots.SetAuditStrength(3)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('SPECIES1','RAW',-123,0)`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	audits := auditCount(t, db, "SPECIES1")
	if err := service.UpdateVegetationSpecies(state.ContextID, "SPECIES1", []VegetationSpeciesUpdate{
		{ID: -123, Form: "SubVegAXL_BC", Expected: "RAW", Value: *options[0].Code},
	}); err != nil {
		t.Fatal("writer must validate against the owned canonical family:", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE ID=-123`).Scan(&stored); err != nil || stored != *options[0].Code {
		t.Fatal("exact selected source code differs:", stored, err)
	}
	if auditCount(t, db, "SPECIES1") != audits+1 {
		t.Fatal("species selection audit differs")
	}
	after := substrateSnapshot(t, db)
	for table, rows := range before {
		if table != "Veg" && table != "Audit" && rows != after[table] {
			t.Fatal("canonical selection changed unrelated project table:", table)
		}
	}
	for path, content := range files {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, content) {
			t.Fatal("selection changed reference/support bytes:", path, err)
		}
	}
}

func TestVegetationSpeciesExplicitRawTransport(t *testing.T) {
	for _, payload := range []string{
		`{}`, `{"id":0,"form":"SubVegAXL_BC","value":"A"}`,
		`{"id":null,"form":"SubVegAXL_BC","expected":"RAW","value":"A"}`,
		`{"id":0,"form":"SubVegAXL_BC","expected":null,"value":"A"}`,
		`{"id":0,"form":null,"expected":"RAW","value":"A"}`,
		`{"id":0,"form":"SubVegAXL_BC","expected":"RAW","value":null}`,
		`{"id":0.5,"form":"SubVegAXL_BC","expected":"RAW","value":"A"}`,
		`{"id":0,"form":"SubVegAXL_BC","expected":"\ud800","value":"A"}`,
		`{"id":0,"form":"SubVegAXL_BC","expected":"RAW","value":"A","VALUE":"\udfff"}`,
	} {
		if err := json.Unmarshal([]byte(payload), &VegetationSpeciesUpdate{}); err == nil {
			t.Fatal("invalid species draft accepted:", payload)
		}
	}
	var update VegetationSpeciesUpdate
	if err := json.Unmarshal([]byte(`{"id":0,"form":"SubVegAXL_BC","expected":"","value":"  RAW' "}`), &update); err != nil ||
		update.ID != 0 || update.Expected != "" || update.Value != "  RAW' " {
		t.Fatal("explicit zero, empty history or literal text lost:", update, err)
	}
}

func TestVegetationSpeciesAtomicMembershipSourceRowsAndHistoricalOmission(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`CREATE TABLE USysAllSpecs(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		CREATE TABLE USysUserSpp(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		INSERT INTO USysAllSpecs VALUES
		('A','A-name',3,NULL,'u'),('A','duplicate',3,'','U'),
		('C',NULL,12,NULL,'X'),('D',NULL,9,NULL,'U'),('SKIP',NULL,3,NULL,'s');
		INSERT INTO USysUserSpp VALUES ('USER',NULL,4,NULL,'U');
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,Cover6,Cover7) VALUES
		('CHILD1','RAW',0,0,0,0),('CHILD1','RAW',-9,0,0,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Height1) VALUES ('CHILD1','RAW',-10,0)`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, form := range []string{"SubVegAXL_BC", "SubVegAhtXL", "SubVegCXL", "SubVegChtXL", "SubVegDXL"} {
		code := "A"
		if strings.Contains(form, "VegC") {
			code = "C"
		} else if strings.Contains(form, "VegD") {
			code = "D"
		}
		if err := s.UpdateVegetationSpecies("CHILD1", []VegetationSpeciesUpdate{{ID: 0, Form: form, Expected: "RAW", Value: code}}); err != nil {
			t.Fatal(form, err)
		}
		if _, err := db.Exec(`UPDATE Sample_Veg SET Species='RAW' WHERE ID=0; DELETE FROM Sample_Audit WHERE PlotNumber='CHILD1'`); err != nil {
			t.Fatal(err)
		}
	}
	for _, batch := range [][]VegetationSpeciesUpdate{
		nil,
		{{ID: 0, Form: "SubVegAXL", Expected: "RAW", Value: "A"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "C"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "a"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: " A"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "SKIP"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: ""}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "123456789"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: string([]byte{0xff})}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "stale", Value: "A"}},
		{{ID: -10, Form: "SubVegAXL_BC", Expected: "RAW", Value: "A"}},
		{{ID: -10, Form: "SubVegChtXL", Expected: "RAW", Value: "C"}},
		{{ID: math.MaxInt32 + 1, Form: "SubVegAXL_BC", Expected: "RAW", Value: "A"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "A"}, {ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "USER"}},
		{{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "A"}, {ID: -9, Form: "SubVegCXL", Expected: "RAW", Value: "A"}},
	} {
		if err := s.UpdateVegetationSpecies("CHILD1", batch); err == nil {
			t.Fatal("invalid source selection accepted:", batch)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	updates := []VegetationSpeciesUpdate{
		{ID: 0, Form: "SubVegAXL_BC", Expected: "RAW", Value: "A"},
		{ID: -9, Form: "SubVegAXL_BC", Expected: "RAW", Value: "USER"},
		{ID: -10, Form: "SubVegAhtXL", Expected: "RAW", Value: "A"},
	}
	if err := s.UpdateVegetationSpecies("CHILD2", updates); err == nil {
		t.Fatal("foreign plot accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`CREATE TRIGGER fail_species BEFORE INSERT ON Sample_Audit
		WHEN NEW.ID=-9 BEGIN SELECT RAISE(ABORT,'injected species audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationSpecies("CHILD1", updates); err == nil {
		t.Fatal("partial species save accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_species`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationSpecies("CHILD1", updates); err != nil {
		t.Fatal("retry:", err)
	}
	if auditCount(t, db, "CHILD1") != 3 {
		t.Fatal("species audit count differs")
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Species='historical invalid' WHERE ID=0;
		CREATE TRIGGER preserve_species BEFORE UPDATE OF Species ON Sample_Veg WHEN OLD.ID=0
		BEGIN SELECT RAISE(ABORT,'historical assignment must be omitted'); END;
		DROP TABLE USysUserSpp`); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	if err := s.UpdateVegetationSpecies("CHILD1", []VegetationSpeciesUpdate{
		{ID: 0, Form: "SubVegAXL_BC", Expected: "historical invalid", Value: "historical invalid"},
	}); err != nil {
		t.Fatal("unchanged historical species should not need a reference assignment:", err)
	}
	assertSubstrateSnapshot(t, db, before)
	if err := s.UpdateVegetationSpecies("CHILD1", []VegetationSpeciesUpdate{
		{ID: -9, Form: "SubVegAXL_BC", Expected: "USER", Value: "A"},
	}); err == nil || !strings.Contains(err.Error(), "references unavailable") {
		t.Fatal("reference failure was not explicit:", err)
	}
	assertSubstrateSnapshot(t, db, before)
}
