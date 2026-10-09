package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func workingUnitServiceFixture(t *testing.T) (*WorkingUnitService, *ProjectService, *sql.DB) {
	t.Helper()
	root := becTestDir(t)
	config := filepath.Join(root, "config")
	projects, err := NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	service, err := NewWorkingUnitService(projects, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	return service, projects, db
}

func workingUnitCodes(rows []WorkingUnitChoice) []string {
	result := []string{}
	for _, row := range rows {
		if row.Code != nil {
			result = append(result, *row.Code)
		}
	}
	return result
}

func TestWorkingUnitFrozenMasterChoicesAndReadOnly(t *testing.T) {
	service, _, _ := workingUnitServiceFixture(t)
	p := workingUnitProvenanceFixture(t)
	if p.DatabaseSHA256 != "94475866a0732284d4b4cdbc8e1dd977777b2b312cf35b6b9594d6496a5f7de3" ||
		p.TypedCellsSHA256 != "b50c467ba2ebb8b3bff153f55371bd2dca0a5fd6b0e9be29f10c3031a485b432" ||
		p.MasterRows+p.UserRows != 3508 || len(p.SourceCopies) != 5 {
		t.Fatal("native reference integrity boundary changed")
	}
	rows, err := service.GetMasterWorkingUnitChoices(context.Background())
	if err != nil || len(rows) != 3504 {
		t.Fatalf("master catalogue %d: %v", len(rows), err)
	}
	same, err := service.GetWorkingUnitChoices(context.Background(), "master")
	if err != nil || !reflect.DeepEqual(rows, same) {
		t.Fatalf("master APIs disagree: %v", err)
	}
	data, _, snapshot := workingUnitSnapshotFixture(t)
	if len(data) == 0 {
		t.Fatal("empty snapshot")
	}
	first, last := false, false
	codes := map[string]int{}
	negative, positive := false, false
	for _, row := range rows {
		ordinal, err := strconv.Atoi(row.RowID)
		if err != nil || ordinal < 1 || ordinal > 3504 || row.Origin != "master" || row.SourceID == nil {
			t.Fatalf("inexact raw identity: %#v", row)
		}
		source := snapshot.Tables["MasterSiteUnitList"][ordinal-1]
		if !reflect.DeepEqual(row.SourceID, source.ID) || !reflect.DeepEqual(row.Code, source.SiteSeries) ||
			!reflect.DeepEqual(row.Description, source.SiteSeriesLongName) ||
			!reflect.DeepEqual(row.ScientificName, source.SiteSeriesScientificName) ||
			!reflect.DeepEqual(row.Level, source.Level) || !row.Selectable || row.Diagnostic != "" {
			t.Fatalf("raw source metadata changed at ordinal %s", row.RowID)
		}
		first, last = first || row.RowID == "1", last || row.RowID == "3504"
		id, err := strconv.ParseInt(*row.SourceID, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		negative, positive = negative || id < 0, positive || id > 0
		codes[*row.Code]++
	}
	duplicates := 0
	for _, count := range codes {
		if count > 1 {
			duplicates++
		}
	}
	if !first || !last || !negative || !positive || duplicates != 34 {
		t.Fatalf("source identities/duplicates lost: first=%v last=%v signs=%v/%v duplicates=%d", first, last, negative, positive, duplicates)
	}
	if _, err := service.db.Exec(`DELETE FROM WorkingUnits`); err == nil {
		t.Fatal("frozen connection permits reference writes")
	}
	encoded, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"rowId", "sourceId"} {
		if len(payload[key]) == 0 || payload[key][0] != '"' {
			t.Fatalf("identity %s is not an exact JSON string", key)
		}
		if string(payload["diagnostic"]) != `""` {
			t.Fatalf("selectable diagnostic must be a non-null empty JSON string: %s", encoded)
		}
	}
}

func TestWorkingUnitMasterNullableDuplicateAndLevelShape(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(workingUnitSchema + `
 INSERT INTO WorkingUnits VALUES
 ('master','1','-2147483648','001',NULL,NULL,11),
 ('master','2','2147483647','001','different definition','Sci',11),
 ('master','3','0',NULL,NULL,NULL,11),
 ('master','4','-1','',NULL,NULL,11),
 ('master','5','1','other level',NULL,NULL,12),
 ('user','1','-2','USER ONLY',NULL,NULL,11);`); err != nil {
		t.Fatal(err)
	}
	service := &WorkingUnitService{db: db}
	rows, err := service.GetMasterWorkingUnitChoices(context.Background())
	if err != nil || len(rows) != 4 {
		t.Fatalf("raw Level11 shape: %#v %v", rows, err)
	}
	selectable := 0
	for _, row := range rows {
		if row.Selectable {
			selectable++
			if row.Code == nil || *row.Code != "001" || row.Diagnostic != "" {
				t.Fatalf("raw leading-zero code lost: %#v", row)
			}
		} else if row.Diagnostic == "" {
			t.Fatal("NULL/empty reference hidden without diagnostic")
		}
	}
	if selectable != 2 {
		t.Fatal("duplicate metadata collapsed or NULL/empty reference selectable")
	}
	if _, err := db.Exec(`ALTER TABLE WorkingUnits DROP COLUMN ScientificName`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetMasterWorkingUnitChoices(context.Background()); err == nil {
		t.Fatal("missing catalogue schema silently accepted")
	}
}

func TestWorkingUnitProjectAndWholeSUChoicesNoWrites(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	if _, err := db.Exec(`
 DELETE FROM Sample_Env; DELETE FROM Sample_Admin; DELETE FROM Sample_Audit;
 INSERT INTO Sample_Env(PlotNumber) VALUES('P1'),('P2'),('P3'),('P4'),('P5'),('P6'),('P7');
 INSERT INTO Sample_Admin(Plot,UserSiteUnit) VALUES
 ('P1','001'),('P2','001'),('P3','Case'),('P4','case'),('P5','O''Brien'),('P6',NULL),('P7',''),('UNJOINED','excluded');
 CREATE TABLE Working_SU(PlotNumber TEXT,SiteUnit TEXT);
 INSERT INTO Working_SU VALUES('P1','001'),('OTHER PROJECT','off-project'),(NULL,'NULL partner'),('P2','001'),('P3',NULL),('P4','O''Brien');
 CREATE TABLE _vpro_su_policy(table_name TEXT PRIMARY KEY,kind TEXT);
 INSERT INTO _vpro_su_policy VALUES('Working_SU','working');`); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.SelectSU("Working"); err != nil {
		t.Fatal(err)
	}
	projectFile := filepath.Join(projects.root, "projects", "Sample.db")
	before, err := os.ReadFile(projectFile)
	if err != nil {
		t.Fatal(err)
	}
	env, err := service.GetWorkingUnitChoices(context.Background(), "env")
	if err != nil || !reflect.DeepEqual(workingUnitCodes(env), []string{"", "001", "Case", "case", "O'Brien"}) {
		t.Fatalf("joined raw env values: %#v %v", env, err)
	}
	if env[0].Selectable || env[0].Diagnostic == "" {
		t.Fatal("historical empty value was silently selectable")
	}
	su, err := service.GetWorkingUnitChoices(context.Background(), "su")
	if err != nil || !reflect.DeepEqual(workingUnitCodes(su), []string{"001", "NULL partner", "O'Brien", "off-project"}) {
		t.Fatalf("SU must include entire authorized table: %#v %v", su, err)
	}
	for _, row := range append(env, su...) {
		if row.SourceID != nil || row.Description != nil || row.ScientificName != nil || row.Level != nil {
			t.Fatal("project choices invent reference metadata/IDs")
		}
	}
	for _, mode := range []string{"env", "master", "su"} {
		if state, err := service.SetWorkingUnitMode(mode); err != nil || state.Mode != mode || state.Warning != nil {
			t.Fatalf("mode %s: %#v %v", mode, state, err)
		}
	}
	after, err := os.ReadFile(projectFile)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("read/mode API mutated project or audit: %v", err)
	}
}

func TestWorkingUnitContextSchemaAndClosedGuards(t *testing.T) {
	service, projects, db := workingUnitServiceFixture(t)
	if _, err := service.GetWorkingUnitChoices(context.Background(), "su"); err == nil {
		t.Fatal("SU mode without active table silently returned master/empty")
	}
	for _, invalid := range []string{"", "Env", "unknown", `master'; DELETE FROM Sample_Admin;--`} {
		if _, err := service.GetWorkingUnitChoices(context.Background(), invalid); err == nil {
			t.Fatal("invalid source mode accepted")
		}
		if _, err := service.SetWorkingUnitMode(invalid); err == nil {
			t.Fatal("invalid preference mode accepted")
		}
	}
	if _, err := db.Exec(`
 CREATE TABLE Private_SU(PlotNumber TEXT,SiteUnit TEXT);
 CREATE TABLE _vpro_su_policy(table_name TEXT PRIMARY KEY,kind TEXT);
 INSERT INTO _vpro_su_policy VALUES('Private_SU','master');`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Private", "Missing", `Bad";DELETE`, ""} {
		projects.mu.Lock()
		projects.activeSU = name
		projects.mu.Unlock()
		if _, err := service.GetWorkingUnitChoices(context.Background(), "su"); err == nil {
			t.Fatalf("unauthorized/stale context %q accepted", name)
		}
		if _, err := service.GetWorkingUnitMode(); err == nil {
			t.Fatalf("unauthorized/stale preference context %q accepted", name)
		}
	}
	projects.mu.Lock()
	projects.activeSU, projects.active = "None", "Missing"
	projects.mu.Unlock()
	if _, err := service.GetWorkingUnitChoices(context.Background(), "env"); err == nil {
		t.Fatal("stale project context accepted")
	}
	projects.mu.Lock()
	projects.active = "Sample"
	projects.mu.Unlock()
	if _, err := db.Exec(`ALTER TABLE Sample_Admin DROP COLUMN UserSiteUnit`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetWorkingUnitChoices(context.Background(), "env"); err == nil {
		t.Fatal("unsupported Admin schema silently accepted")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetMasterWorkingUnitChoices(context.Background()); err == nil {
		t.Fatal("closed master lookup succeeded")
	}
	if _, err := service.GetWorkingUnitMode(); err == nil {
		t.Fatal("closed preference read succeeded")
	}
	if _, err := service.SetWorkingUnitMode("env"); err == nil {
		t.Fatal("closed preference write succeeded")
	}
}

func TestWorkingUnitCopiedCatalogueNeverOverwritesExistingFiles(t *testing.T) {
	root := becTestDir(t)
	projects, err := NewProjectServiceWithConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"vlists.db", "bec.db", "working-unit.db"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("unrecognized user reference"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewWorkingUnitService(projects, root); err == nil || !strings.Contains(err.Error(), "not replaced") {
		t.Fatalf("unknown existing catalogue accepted/overwritten: %v", err)
	}
	for _, name := range []string{"vlists.db", "bec.db", "working-unit.db"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(data) != "unrecognized user reference" {
			t.Fatalf("existing %s changed: %v", name, err)
		}
	}
}
