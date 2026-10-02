package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestSourceVegetationCreationRollbackAllocationAndNoGuessedValues(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`CREATE TABLE USysAllSpecs(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		CREATE TABLE USysUserSpp(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		INSERT INTO USysAllSpecs VALUES ('A',NULL,3,NULL,'u'),('A','duplicate',3,'','U'),('C',NULL,12,NULL,'X'),('D',NULL,9,NULL,'U');
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('CHILD1','RAW',0,0);
		CREATE TRIGGER fail_creation_audit BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Cover1'
		BEGIN SELECT RAISE(ABORT,'injected source creation audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	zero, negative, height, tooLarge := 0.0, -3.125, -7.123456789, 100.0
	before := substrateSnapshot(t, db)
	for _, request := range []VegetationCreationRequest{
		{Form: "wrong", Species: "A", Values: map[string]*float64{"cover1": &zero}},
		{Form: "SubVegAXL_BC", Species: "A"},
		{Form: "SubVegAXL_BC", Species: "A", Values: map[string]*float64{"cover1": nil}},
		{Form: "SubVegChtXL", Species: "C", Values: map[string]*float64{"height6": &height}},
		{Form: "SubVegAXL_BC", Species: "C", Values: map[string]*float64{"cover1": &zero}},
		{Form: "SubVegAXL_BC", Species: "a", Values: map[string]*float64{"cover1": &zero}},
		{Form: "SubVegAXL_BC", Species: "UNKNOWN", Values: map[string]*float64{"cover1": &zero}},
		{Form: "SubVegAXL_BC", Species: "A", Values: map[string]*float64{"cover6": &zero}},
		{Form: "SubVegAXL_BC", Species: "A", Values: map[string]*float64{"cover1": &tooLarge}},
		{Form: "SubVegAXL_BC", Species: "A", Values: map[string]*float64{"cover1": &negative}},
	} {
		if id, err := s.CreateSourceVegetation("CHILD1", request); err == nil || id != 0 {
			t.Fatal("rejected creation returned success/identity:", request, id, err)
		}
		assertSubstrateSnapshot(t, db, before)
		var ledger int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='__VPRO_ChildIdentity'`).Scan(&ledger); err != nil || ledger != 0 {
			t.Fatal("rejected creation committed the identity ledger:", ledger, err)
		}
	}
	if _, err := db.Exec(`DROP TRIGGER fail_creation_audit`); err != nil {
		t.Fatal(err)
	}
	request := VegetationCreationRequest{Form: "SubVegAXL_BC", Species: "A", Values: map[string]*float64{"cover1": &negative}}
	for _, assignment := range []string{`Species='D'`, `Cover1=4`} {
		if _, err := db.Exec(`CREATE TRIGGER change_planned_creation AFTER INSERT ON Sample_Veg
			WHEN NEW.ID>0 BEGIN UPDATE Sample_Veg SET ` + assignment + ` WHERE rowid=NEW.rowid; END`); err != nil {
			t.Fatal(err)
		}
		if id, err := s.CreateSourceVegetation("CHILD1", request); err == nil || id != 0 {
			t.Fatal("independently stored species/number drift escaped creation guard:", id, err)
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER change_planned_creation`); err != nil {
			t.Fatal(err)
		}
	}
	id, err := s.CreateSourceVegetation("CHILD1", request)
	if err != nil || id != 1 {
		t.Fatal("retained creation retry failed:", id, err)
	}
	var layer, otherCover, recordedCover any
	if err := db.QueryRow(`SELECT Layer,Cover2,Cover1 FROM Sample_Veg WHERE ID=?`, id).Scan(&layer, &otherCover, &recordedCover); err != nil ||
		layer != nil || otherCover != nil || recordedCover != negative {
		t.Fatal("creation guessed Layer/covers or changed precision:", layer, otherCover, recordedCover, err)
	}
	if auditCount(t, db, "CHILD1") != 2 {
		t.Fatal("creation must audit only explicit species/cover")
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	review, err := inspectVegetationDeletion(context.Background(), tx, `"Sample_Veg"`, "CHILD1", request.Form, id)
	tx.Rollback()
	if err != nil || s.DeleteReviewedVegetation("CHILD1", VegetationDeletionRequest{ID: id, Form: request.Form, Expected: review.review.Expected}) != nil {
		t.Fatal("creation/deletion reservation setup failed:", err)
	}
	id, err = s.CreateSourceVegetation("CHILD1", request)
	if err != nil || id != 2 {
		t.Fatal("creation reused deleted/reserved identity:", id, err)
	}
	id, err = s.CreateSourceVegetation("CHILD1", VegetationCreationRequest{Form: "SubVegAhtXL", Species: "A", Values: map[string]*float64{"height1": &height}})
	if err != nil || id != 3 {
		t.Fatal("explicit source height-only creation failed:", id, err)
	}
	var cover any
	var gotHeight float64
	if err := db.QueryRow(`SELECT Cover1,Height1 FROM Sample_Veg WHERE ID=?`, id).Scan(&cover, &gotHeight); err != nil || cover != nil || gotHeight != height {
		t.Fatal("height-only creation inferred cover or rounded height:", cover, gotHeight, err)
	}
}

func TestSourceVegetationCreationCanonicalFamilyAndScopedContext(t *testing.T) {
	service, state := contextServiceFixture(t)
	options, err := service.ListVegetationSpecies(context.Background(), state.ContextID, "SubVegAXL_BC")
	if err != nil || len(options) == 0 || options[0].Code == nil {
		t.Fatal("canonical source list unavailable:", err)
	}
	files := map[string][]byte{}
	for _, path := range service.projects.sqlite.attachments {
		if path != state.ProjectPath {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files[path] = data
		}
	}
	if err := service.CreatePlot(state.ContextID, FS882Header{PlotNumber: "NEW1"}); err != nil {
		t.Fatal(err)
	}
	service.plots.SetAuditStrength(3)
	number := -6.875
	request := VegetationCreationRequest{Form: "SubVegAXL_BC", Species: *options[0].Code, Values: map[string]*float64{"cover2": &number}}
	id, err := service.CreateSourceVegetation(state.ContextID, "NEW1", request)
	if err != nil || id <= 0 || id > math.MaxInt32 {
		t.Fatal("scoped canonical creation failed:", id, err)
	}
	if _, err := service.CreateSourceVegetation("retired", "NEW1", request); err == nil {
		t.Fatal("retired context creation accepted")
	}
	if _, err := service.plots.CreateSourceVegetation("NEW1", request); err == nil {
		t.Fatal("legacy unscoped creation bypassed active context")
	}
	records, err := service.ListVegRecords(context.Background(), state.ContextID, "NEW1")
	if err != nil || len(records) != 1 || records[0].ID != id || records[0].Species != request.Species || records[0].Cover2 == nil || *records[0].Cover2 != number {
		t.Fatal("created canonical record differs:", records, err)
	}
	for path, original := range files {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("creation changed canonical reference/user/metadata bytes:", path, err)
		}
	}
}

func TestSourceVegetationCreationStrictRawTransport(t *testing.T) {
	for _, payload := range []string{
		`{}`, `{"form":"SubVegAXL_BC","species":"A"}`, `{"form":"SubVegAXL_BC","species":null,"values":{}}`,
		`{"form":"SubVegAXL_BC","species":"A","values":null}`,
		`{"form":"SubVegAXL_BC","species":"\ud800","values":{"cover1":0}}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"id":0}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"plotNumber":"NEW1"}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"Values":{}}`,
	} {
		if err := json.Unmarshal([]byte(payload), &VegetationCreationRequest{}); err == nil {
			t.Fatal("invalid/raw-repaired/identity-bearing creation accepted:", payload)
		}
	}
	var request VegetationCreationRequest
	if err := json.Unmarshal([]byte(`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0,"cover2":null}}`), &request); err != nil ||
		request.Values["cover1"] == nil || *request.Values["cover1"] != 0 || request.Values["cover2"] != nil {
		t.Fatal("explicit zero/NULL transport lost:", request, err)
	}
}
