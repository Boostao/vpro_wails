package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func speciesCodeCheckFixture(t *testing.T) (*ContextService, ProjectState, string) {
	t.Helper()
	service, state := contextServiceFixture(t)
	for _, plot := range []string{"CHECK1", "CHECK2"} {
		if err := service.CreatePlot(state.ContextID, FS882Header{PlotNumber: plot}); err != nil {
			t.Fatal(err)
		}
		service.plots.SetAuditStrength(3)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`ALTER TABLE Sample_Veg RENAME TO CheckSourceVeg;
		CREATE TABLE Sample_Veg AS SELECT * FROM CheckSourceVeg;
		DROP TABLE CheckSourceVeg`); err != nil {
		t.Fatal("nullable disposable vegetation variant:", err)
	}
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Height1)
		VALUES ('CHECK1','BADONE',-100,1),('CHECK2','BADTWO',-101,2),('CHECK2',NULL,-102,3)`); err != nil {
		t.Fatal(err)
	}
	options, err := service.ListVegetationSpecies(context.Background(), state.ContextID, "SubVegAhtXL")
	if err != nil || len(options) == 0 || options[0].Code == nil {
		t.Fatal("canonical fixture target unavailable:", err)
	}
	return service, state, *options[0].Code
}

func TestSpeciesCodeCheckScopedReadonlyReviewAndLiteralMetadata(t *testing.T) {
	service, state, target := speciesCodeCheckFixture(t)
	files := map[string][]byte{}
	for _, path := range service.projects.sqlite.attachments {
		var err error
		files[path], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	review, err := service.ReviewSpeciesCodes(context.Background(), state.ContextID)
	if err != nil || review.Project != "Sample" || review.SU != state.ActiveSU {
		t.Fatal("context-selected scope differs:", review, err)
	}
	found := map[int]SpeciesCodeCheckRow{}
	for _, row := range review.Rows {
		found[row.ID] = row
	}
	for _, id := range []int{-100, -101} {
		if found[id].Code == nil || found[id].Status != "unlisted" {
			t.Fatal("height-only unlisted source omitted:", found[id])
		}
	}
	if found[-102].Code != nil || found[-102].Status != "missing" {
		t.Fatal("NULL code distinction lost:", found[-102])
	}
	options, err := service.LookupSpeciesCodeCheckTarget(context.Background(), state.ContextID, VegetationSpeciesLookup{target})
	if err != nil || len(options) == 0 || options[0].Code == nil || *options[0].Code != target {
		t.Fatal("literal target metadata unavailable:", options, err)
	}
	wrongCase, err := service.LookupSpeciesCodeCheckTarget(context.Background(), state.ContextID, VegetationSpeciesLookup{strings.ToLower(target)})
	if err != nil || len(wrongCase) != 0 {
		t.Fatal("case was implicitly repaired:", wrongCase, err)
	}
	for path, before := range files {
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("readonly check changed database bytes:", path, err)
		}
	}
	if _, err := service.ReviewSpeciesCodes(context.Background(), "stale"); err == nil {
		t.Fatal("stale review accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ReviewSpeciesCodes(cancelled, state.ContextID); err == nil {
		t.Fatal("cancelled review returned success")
	}
}

func TestSpeciesCodeCheckPreservesTemporaryDefinitionsAndDuplicateNullableMetadata(t *testing.T) {
	service, state, _ := speciesCodeCheckFixture(t)
	var target string
	if err := service.projects.sqlite.conn.QueryRowContext(context.Background(),
		`SELECT Code FROM USysAllSpecs WHERE Code IS NOT NULL AND Code<>'' ORDER BY Code COLLATE BINARY LIMIT 1`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	userPath := service.projects.supportPaths["VUser"]
	userDB, err := sql.Open("sqlite3", sqliteFileURI(userPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer userDB.Close()
	if _, err := userDB.Exec(`INSERT INTO USysUserSpp(Code,ScientificName,EnglishName,LifeForm,Codetype)
			VALUES ('ZZSEN999',NULL,'',999,NULL);
			INSERT INTO USysUserSpp(Code,ScientificName,EnglishName,LifeForm,Codetype)
			VALUES (?,NULL,'',NULL,'') ON CONFLICT(Code) DO UPDATE
			SET ScientificName=NULL,EnglishName='',LifeForm=NULL,Codetype=''`, target); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(userPath)
	if err != nil {
		t.Fatal(err)
	}
	options, err := service.LookupSpeciesCodeCheckTarget(context.Background(), state.ContextID, VegetationSpeciesLookup{target})
	if err != nil || len(options) < 2 {
		t.Fatal("duplicate master/user definitions collapsed:", options, err)
	}
	foundMaster, foundUser := false, false
	for _, option := range options {
		if option.Code == nil || *option.Code != target {
			t.Fatal("literal target changed:", option)
		}
		if option.Source == "master" {
			foundMaster = true
		}
		if option.Source == "user" {
			foundUser = true
			if option.ScientificName != nil || option.Lifeform != nil || option.EnglishName == nil ||
				*option.EnglishName != "" || option.CodeType == nil || *option.CodeType != "" {
				t.Fatal("NULL/empty user metadata was repaired or collapsed:", option)
			}
		}
	}
	if !foundMaster || !foundUser {
		t.Fatal("physical master/user provenance lost")
	}
	original := "BADONE"
	if err := service.SaveSpeciesCodeCheck(state.ContextID, []SpeciesCodeCheckUpdate{{-100, "CHECK1", &original, target}}); err != nil {
		t.Fatal(err)
	}
	sentinel, err := service.LookupSpeciesCodeCheckTarget(context.Background(), state.ContextID, VegetationSpeciesLookup{"ZZSEN999"})
	if err != nil || len(sentinel) != 1 || sentinel[0].Lifeform == nil || *sentinel[0].Lifeform != 999 {
		t.Fatal("existing LifeForm999 definition was deleted or hidden:", sentinel, err)
	}
	after, err := os.ReadFile(userPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("project check/save changed temporary or nullable user definitions")
	}
}

func TestSpeciesCodeCheckMultiPlotAtomicAuditRollbackObservationAndRetry(t *testing.T) {
	service, state, target := speciesCodeCheckFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before := substrateSnapshot(t, db)
	first, second := "BADONE", "BADTWO"
	updates := []SpeciesCodeCheckUpdate{{-100, "CHECK1", &first, target}, {-101, "CHECK2", &second, target}}
	support := map[string][]byte{}
	for _, path := range service.projects.sqlite.attachments {
		if path == state.ProjectPath {
			continue
		}
		support[path], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, trigger := range []string{
		`CREATE TRIGGER fail_check BEFORE INSERT ON Sample_Audit WHEN NEW.ID=-101 BEGIN SELECT RAISE(ABORT,'second plot audit failure'); END`,
		`CREATE TRIGGER fail_check AFTER UPDATE OF Species ON Sample_Veg WHEN NEW.ID=-101 BEGIN UPDATE Sample_Veg SET Species='DRIFT' WHERE ID=NEW.ID; END`,
		`CREATE TRIGGER fail_check AFTER INSERT ON Sample_Audit WHEN NEW.ID=-101 BEGIN UPDATE Sample_Veg SET Species='DRIFT' WHERE ID=-100; END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		if err := service.SaveSpeciesCodeCheck(state.ContextID, updates); err == nil {
			t.Fatal("audit failure/stored drift did not abort the entire multi-plot transaction")
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER fail_check`); err != nil {
			t.Fatal(err)
		}
	}
	for _, invalid := range [][]SpeciesCodeCheckUpdate{
		nil,
		{{-100, "CHECK1", &first, "ZBADCK01"}},
		{{-100, "CHECK2", &first, target}},
		{{-100, "CHECK1", &second, target}},
		{updates[0], updates[0]},
		{{-102, "CHECK2", &first, target}},
		{{-100, "CHECK1", &first, first}},
		{{2147483648, "CHECK1", &first, target}},
	} {
		if err := service.SaveSpeciesCodeCheck(state.ContextID, invalid); err == nil {
			t.Fatal("invalid check assignment accepted:", invalid)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if err := service.SaveSpeciesCodeCheck("stale", updates); err == nil {
		t.Fatal("stale write accepted")
	}
	if err := service.SaveSpeciesCodeCheck(state.ContextID, updates); err != nil {
		t.Fatal("multi-plot retry failed:", err)
	}
	for _, plot := range []string{"CHECK1", "CHECK2"} {
		if auditCount(t, db, plot) != 1 {
			t.Fatal("retry did not commit one audit per independently changed plot:", plot)
		}
	}
	if err := service.SaveSpeciesCodeCheck(state.ContextID, []SpeciesCodeCheckUpdate{{-102, "CHECK2", nil, target}}); err != nil {
		t.Fatal("explicit NULL original could not be corrected:", err)
	}
	for path, content := range support {
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, content) {
			t.Fatal("code check changed user/reference/metadata bytes:", path, err)
		}
	}
}

func TestSpeciesCodeCheckSelectedWorkingUnitCannotBroadenReplacement(t *testing.T) {
	service, state, target := speciesCodeCheckFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`INSERT INTO Sample_SU(PlotNumber,SiteUnit) VALUES ('CHECK1','Code check scope')`); err != nil {
		t.Fatal(err)
	}
	release()
	selected := contextSelection(state)
	selected.SU, selected.SUPath = "Sample", state.ProjectPath
	current, err := service.SwitchContext(state.ContextID, selected)
	if err != nil {
		t.Fatal(err)
	}
	db, _, release, err = service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	review, err := service.ReviewSpeciesCodes(context.Background(), current.ContextID)
	if err != nil || review.SU != "Sample" {
		t.Fatal("working-unit scope unavailable:", review, err)
	}
	found := false
	for _, row := range review.Rows {
		if row.ID == -100 {
			found = true
		}
		if row.ID == -101 || row.ID == -102 {
			t.Fatal("code check escaped selected working unit:", row)
		}
	}
	if !found {
		t.Fatal("selected working unit lost its source row")
	}
	before := substrateSnapshot(t, db)
	second := "BADTWO"
	if err := service.SaveSpeciesCodeCheck(current.ContextID, []SpeciesCodeCheckUpdate{{-101, "CHECK2", &second, target}}); err == nil {
		t.Fatal("replacement broadened to an excluded project plot")
	}
	assertSubstrateSnapshot(t, db, before)
	first := "BADONE"
	if err := service.SaveSpeciesCodeCheck(current.ContextID, []SpeciesCodeCheckUpdate{{-100, "CHECK1", &first, target}}); err != nil {
		t.Fatal("selected working-unit replacement failed:", err)
	}
}

func TestSpeciesCodeCheckRejectsAmbiguousAndMalformedHistoricalStorageWithoutRepair(t *testing.T) {
	for _, mutation := range []string{
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID) VALUES ('CHECK2','DUPLICATE',-100)`,
		`UPDATE Sample_Veg SET Species=x'FF' WHERE ID=-100`,
		`UPDATE Sample_Veg SET ID=0.5 WHERE ID=-100`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state, _ := speciesCodeCheckFixture(t)
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if _, err := service.ReviewSpeciesCodes(context.Background(), state.ContextID); err == nil {
				t.Fatal("ambiguous/raw-invalid source became a successful or repaired review")
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
}

func TestSpeciesCodeCheckStrictRawTransport(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"id":0,"plotNumber":"CHECK1","value":"A"}`,
		`{"id":null,"plotNumber":"CHECK1","expected":null,"value":"A"}`,
		`{"id":0,"plotNumber":null,"expected":null,"value":"A"}`,
		`{"id":0,"plotNumber":"CHECK1","expected":null,"value":null}`,
		`{"id":0.5,"plotNumber":"CHECK1","expected":null,"value":"A"}`,
		`{"id":0,"plotNumber":"CHECK1","expected":"\ud800","value":"A"}`,
		`{"id":0,"plotNumber":"CHECK1","expected":null,"value":"\udfff"}`,
		`{"id":0,"plotNumber":"CHECK1","expected":null,"value":"A","Value":"B"}`,
		`{"id":0,"plotNumber":"CHECK1","expected":null,"value":"A","scope":"all"}`,
	} {
		if err := json.Unmarshal([]byte(payload), &SpeciesCodeCheckUpdate{}); err == nil {
			t.Fatal("malformed or caller-selected scope accepted:", payload)
		}
	}
	var update SpeciesCodeCheckUpdate
	if err := json.Unmarshal([]byte(`{"id":0,"plotNumber":"CHECK1","expected":null,"value":" A' "}`), &update); err != nil ||
		update.Expected != nil || update.Value != " A' " {
		t.Fatal("NULL/literal transport changed:", update, err)
	}
}
