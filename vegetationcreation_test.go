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
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"decision":null}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"Decision":"keep"}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"decision":"keep","entered":"\ud800"}`,
		`{"form":"SubVegAXL_BC","species":"A","values":{"cover1":0},"decision":"replace","entered":"OLD","selected":"\udfff"}`,
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

func TestSourceVegetationCreationExplicitDecisionsRollbackAndRetry(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`CREATE TABLE USysAllSpecs(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT,OldCode TEXT);
				CREATE TABLE USysUserSpp(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
				INSERT INTO USysAllSpecs VALUES ('new_a',NULL,99,NULL,'S','olddup'),('NEW_B','duplicate',3,'','U','olddup'),
				(NULL,NULL,NULL,NULL,NULL,'Personal');
				INSERT INTO USysUserSpp VALUES ('Personal',NULL,NULL,'',NULL),('olddup',NULL,1,NULL,'U');
				INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES ('CHILD1','RAW',0,0)`); err != nil {
		t.Fatal(err)
	}
	text := func(value string) *string { return &value }
	zero := 0.0
	request := func(species, decision string, entered, selected *string) VegetationCreationRequest {
		return VegetationCreationRequest{Form: "SubVegAXL_BC", Species: species, Decision: decision,
			Entered: entered, Selected: selected, Values: map[string]*float64{"cover1": &zero}}
	}
	before := substrateSnapshot(t, db)
	for _, proposed := range []VegetationCreationRequest{
		request("NEW_A", "", text("olddup"), nil),
		request("NEW_A", "automatic", text("olddup"), text("new_a")),
		request("NEW_A", "replace", nil, text("new_a")),
		request("NEW_A", "replace", text("olddup"), nil),
		request("new_a", "replace", text("olddup"), text("new_a")),
		request("NEW_A", "replace", text("olddup"), text("NEW_A")),
		request("NEW_A", "replace", text("wrong"), text("new_a")),
		request("NEW_A", "replace", text(" olddup "), text("new_a")),
		request("NEW_A", "replace", text("é"), text("new_a")),
		request("OLDDUP", "keep", text("olddup"), text("new_a")),
		request("PERSONAL", "keep", text("Personal"), nil),
		request("UNKNOWN", "keep", text("unknown"), nil),
		request("OLDDUP", "user", text("olddup"), text("olddup")),
		request("PERSONAL", "user", text("wrong"), text("Personal")),
		request("PERSONAL", "user", text("Personal"), text("PERSONAL")),
	} {
		if id, err := s.CreateSourceVegetation("CHILD1", proposed); err == nil || id != 0 {
			t.Fatal("invalid, stale or precedence-bypassing creation decision accepted:", proposed, id, err)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	replace := request("NEW_A", "replace", text("OLDdup"), text("new_a"))
	keep := request("OLDDUP", "keep", text("olddup"), nil)
	user := request("PERSONAL", "user", text("personal"), text("Personal"))
	for _, mutation := range []string{
		`DELETE FROM USysAllSpecs WHERE Code='new_a'`,
		`UPDATE USysAllSpecs SET Code=NULL WHERE OldCode='olddup'`,
		`DELETE FROM USysUserSpp WHERE Code='Personal'`,
		`INSERT INTO USysAllSpecs VALUES ('MASTER',NULL,NULL,NULL,NULL,'personal')`,
	} {
		if _, err := db.Exec(`CREATE TRIGGER reference_drift AFTER INSERT ON Sample_Veg BEGIN ` + mutation + `; END`); err != nil {
			t.Fatal(err)
		}
		proposed := user
		if mutation == `DELETE FROM USysAllSpecs WHERE Code='new_a'` {
			proposed = replace
		} else if mutation == `UPDATE USysAllSpecs SET Code=NULL WHERE OldCode='olddup'` {
			proposed = keep
		}
		if id, err := s.CreateSourceVegetation("CHILD1", proposed); err == nil || id != 0 {
			t.Fatal("stored source reference drift escaped transactional validation:", id, err)
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER reference_drift`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_creation_decision BEFORE INSERT ON Sample_Audit
				WHEN NEW.EditField='Cover1' BEGIN SELECT RAISE(ABORT,'injected decision audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, proposed := range []VegetationCreationRequest{replace, keep, user} {
		if id, err := s.CreateSourceVegetation("CHILD1", proposed); err == nil || id != 0 {
			t.Fatal("failed audit committed a creation decision:", id, err)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_creation_decision`); err != nil {
		t.Fatal(err)
	}
	for i, proposed := range []VegetationCreationRequest{replace, keep, user} {
		id, err := s.CreateSourceVegetation("CHILD1", proposed)
		if err != nil || id != int64(i+1) {
			t.Fatal("retained explicit decision retry/identity failed:", proposed, id, err)
		}
		var species string
		var layer any
		if err := db.QueryRow(`SELECT Species,Layer FROM Sample_Veg WHERE ID=?`, id).Scan(&species, &layer); err != nil ||
			species != proposed.Species || layer != nil {
			t.Fatal("source UCase or NULL Layer differs:", species, layer, err)
		}
	}
	if auditCount(t, db, "CHILD1") != 6 {
		t.Fatal("each creation must commit exactly its species/explicit cover audits")
	}
	var userCount int
	if err := db.QueryRow(`SELECT count(*) FROM USysUserSpp`).Scan(&userCount); err != nil || userCount != 2 {
		t.Fatal("project creation wrote personal definitions:", userCount, err)
	}
}

func TestSourceVegetationCreationReusesSeparatelySavedNullablePersonalDefinition(t *testing.T) {
	service, state := contextServiceFixture(t)
	if _, err := service.CreatePersonalSpeciesDefinition(context.Background(), state.ContextID,
		PersonalSpeciesDefinitionRequest{Entered: "zcreate"}); err != nil {
		t.Fatal(err)
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
	if err := service.CreatePlot(state.ContextID, FS882Header{PlotNumber: "NEWUSER"}); err != nil {
		t.Fatal(err)
	}
	zero, entered, selected := 0.0, "zcreate", "ZCREATE"
	request := VegetationCreationRequest{Form: "SubVegCXL", Species: selected, Decision: "user",
		Entered: &entered, Selected: &selected, Values: map[string]*float64{"cover6": &zero}}
	id, err := service.CreateSourceVegetation(state.ContextID, "NEWUSER", request)
	if err != nil || id <= 0 {
		t.Fatal("nullable non-dropdown personal code could not be explicitly reused:", id, err)
	}
	records, err := service.ListVegRecords(context.Background(), state.ContextID, "NEWUSER")
	if err != nil || len(records) != 1 || records[0].Species != selected || records[0].Layer != nil {
		t.Fatal("separate project creation differs:", records, err)
	}
	for path, original := range files {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("project creation altered separately saved user/support bytes:", path, err)
		}
	}
}
