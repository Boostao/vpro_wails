package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func personalSpeciesFixture(t *testing.T) (*ContextService, ProjectState, *sql.DB) {
	t.Helper()
	service, state := contextServiceFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VUser"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return service, state, db
}

func TestPersonalSpeciesTransportPhysicalBoundsAndNullMetadata(t *testing.T) {
	valid := `{"entered":"zznew01","scientificName":null,"lifeform":null,"englishName":""}`
	var request PersonalSpeciesDefinitionRequest
	if err := json.Unmarshal([]byte(valid), &request); err != nil {
		t.Fatal(err)
	}
	proposed, err := preparePersonalSpecies(request)
	if err != nil || *proposed.Code != "ZZNEW01" || proposed.ScientificName != nil ||
		proposed.Lifeform != nil || proposed.EnglishName == nil || *proposed.EnglishName != "" || proposed.CodeType != nil {
		t.Fatalf("source UCase/NULL/empty/default distinction lost: %+v %v", proposed, err)
	}
	for _, body := range []string{
		`{"entered":"ZZNEW01","scientificName":null,"lifeform":null}`,
		`{"entered":null,"scientificName":null,"lifeform":null,"englishName":null}`,
		`{"entered":"ZZNEW01","scientificName":null,"lifeform":1.5,"englishName":null}`,
		`{"entered":"ZZNEW01","scientificName":"\ud800","lifeform":null,"englishName":null}`,
		`{"entered":"ZZNEW01","scientificName":null,"lifeform":null,"englishName":"\udfff"}`,
		`{"entered":"ZZNEW01","scientificName":null,"lifeform":null,"englishName":null,"Codetype":"u"}`,
		`{"entered":"ZZNEW01","scientificName":null,"lifeform":null,"englishName":null,"path":"caller.db"}`,
		`{"Entered":"ZZNEW01","scientificName":null,"lifeform":null,"englishName":null}`,
	} {
		if err := json.Unmarshal([]byte(body), &request); err == nil {
			t.Fatalf("accepted malformed/implicit/caller-owned metadata: %s", body)
		}
	}
	exact := strings.Repeat("\U0001f332", 127) + "x"
	overlong := exact + "x"
	badUTF8 := string([]byte{0xff})
	tooLarge := 32768
	for _, request := range []PersonalSpeciesDefinitionRequest{
		{Entered: ""}, {Entered: "TOOLONG09"}, {Entered: "\u00e9new"},
		{Entered: "ZZNEW01", ScientificName: &overlong},
		{Entered: "ZZNEW01", EnglishName: &overlong},
		{Entered: "ZZNEW01", ScientificName: &badUTF8},
		{Entered: "ZZNEW01", Lifeform: &tooLarge},
	} {
		if _, err := preparePersonalSpecies(request); err == nil {
			t.Fatalf("accepted invalid source input: %+v", request)
		}
	}
	request = PersonalSpeciesDefinitionRequest{Entered: " zzraw ", ScientificName: &exact}
	if proposed, err := preparePersonalSpecies(request); err != nil || *proposed.Code != " ZZRAW " || *proposed.ScientificName != exact {
		t.Fatalf("source UCase silently trimmed or changed metadata: %+v %v", proposed, err)
	}
}

func TestPersonalSpeciesDefinitionOnlyPreservesFamilyAndSourceDefaults(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	if _, err := db.Exec(`CREATE TABLE _table_metadata(table_name TEXT,description TEXT);
		INSERT INTO _table_metadata VALUES ('USysUserSpp',NULL),('USysUserSpp',''),('USysUserSpp','Original personal definitions')`); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{"project": state.ProjectPath, "config": filepath.Join(service.projects.config, "config.yml")}
	for role, path := range service.projects.supportPaths {
		if role != "VUser" {
			paths[role] = path
		}
	}
	beforeFiles := databaseBytes(t, paths)
	beforeTables := map[string]string{}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, table := range append(tables, "sqlite_master") {
		beforeTables[table] = heightTableSnapshot(t, db, table)
	}
	empty := ""
	name := "  Source name \U0001f332  "
	proposed, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID,
		PersonalSpeciesDefinitionRequest{Entered: "zznew01", ScientificName: &name, EnglishName: &empty})
	if err != nil || proposed.Code == nil || *proposed.Code != "ZZNEW01" || proposed.CodeType != nil {
		t.Fatalf("definition creation failed: %+v %v", proposed, err)
	}
	definitions, err := service.ListVegetationSpeciesUsers(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "zznew01"})
	if err != nil || len(definitions) != 1 || definitions[0].ScientificName == nil || *definitions[0].ScientificName != name ||
		definitions[0].EnglishName == nil || *definitions[0].EnglishName != "" || definitions[0].Lifeform != nil || definitions[0].CodeType != nil {
		t.Fatalf("readonly coordinator did not observe exact nullable definition: %+v %v", definitions, err)
	}
	var report float64
	var number, codeType any
	if err := db.QueryRow(`SELECT Report,SppNumber,Codetype FROM USysUserSpp WHERE Code='ZZNEW01'`).Scan(&report, &number, &codeType); err != nil ||
		report != 1 || number != nil || codeType != nil {
		t.Fatalf("hidden source defaults changed: %v %v %v %v", report, number, codeType, err)
	}
	var field, after string
	var plot, before any
	if err := db.QueryRow(`SELECT EditField,AfterEdit,PlotNumber,BeforeEdit FROM USysAuditTrail
		WHERE rowid=(SELECT MAX(rowid) FROM USysAuditTrail)`).Scan(&field, &after, &plot, &before); err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(after), &snapshot); err != nil || field != "CreateRecord" || plot != nil || before != nil ||
		snapshot["Code"] != "ZZNEW01" || snapshot["LifeForm"] != nil || snapshot["ScientificName"] != name ||
		snapshot["EnglishName"] != "" || snapshot["Report"] != float64(1) || snapshot["SppNumber"] != nil || snapshot["Codetype"] != nil {
		t.Fatalf("explicit definition-only audit differs: %s %v", after, err)
	}
	for table, before := range beforeTables {
		after := heightTableSnapshot(t, db, table)
		if table == "USysUserSpp" || table == "USysAuditTrail" {
			var original, actual []json.RawMessage
			if err := json.Unmarshal([]byte(before), &original); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(after), &actual); err != nil || len(actual) != len(original)+1 {
				t.Fatalf("%s expected exactly one new record", table)
			}
			for i := range original {
				if !bytes.Equal(original[i], actual[i]) {
					t.Fatalf("%s changed historical row %d", table, i)
				}
			}
		} else if before != after {
			t.Fatalf("unrelated user table/schema changed: %s", table)
		}
	}
	for role, before := range beforeFiles {
		actual, err := os.ReadFile(paths[role])
		if err != nil || !bytes.Equal(before, actual) {
			t.Fatalf("definition creation changed %s bytes", role)
		}
	}
}

func TestPersonalSpeciesCollisionRollbackAndRetry(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	checkUnchanged := func(beforeDefinitions, beforeAudit string) {
		t.Helper()
		if heightTableSnapshot(t, db, "USysUserSpp") != beforeDefinitions || heightTableSnapshot(t, db, "USysAuditTrail") != beforeAudit {
			t.Fatal("rejected user definition changed records/history")
		}
	}
	beforeDefinitions, beforeAudit := heightTableSnapshot(t, db, "USysUserSpp"), heightTableSnapshot(t, db, "USysAuditTrail")
	for _, code := range []string{"PSEUMEN", "pseumen"} {
		if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: code}); err == nil {
			t.Fatal("master code collision accepted")
		}
		checkUnchanged(beforeDefinitions, beforeAudit)
	}
	mutateContextFixture(t, service.projects.supportPaths["VLists"],
		`INSERT INTO USysAllSpecs(Code,OldCode) VALUES ('ZZMSTR01','zzalias')`)
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZALIAS"}); err == nil {
		t.Fatal("master old-code precedence ignored")
	}
	checkUnchanged(beforeDefinitions, beforeAudit)
	for _, statement := range []string{
		`CREATE TRIGGER reject_user_audit BEFORE INSERT ON USysAuditTrail BEGIN SELECT RAISE(ABORT,'audit blocked'); END`,
		`CREATE TRIGGER change_user_insert AFTER INSERT ON USysUserSpp BEGIN UPDATE USysUserSpp SET ScientificName='drift' WHERE Code=NEW.Code; END`,
		`CREATE TRIGGER change_user_audit AFTER INSERT ON USysAuditTrail BEGIN UPDATE USysUserSpp SET Report=2 WHERE Code='ZZNEW01'; END`,
		`CREATE TRIGGER change_audit_snapshot AFTER INSERT ON USysAuditTrail BEGIN UPDATE USysAuditTrail SET AfterEdit='drift' WHERE rowid=NEW.rowid; END`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZNEW01"}); err == nil {
			t.Fatal("audit failure/stored metadata drift accepted")
		}
		checkUnchanged(beforeDefinitions, beforeAudit)
		name := strings.Fields(statement)[2]
		if _, err := db.Exec("DROP TRIGGER " + quoteHeaderIdentifier(name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZNEW01"}); err != nil {
		t.Fatal(err)
	}
	beforeDefinitions, beforeAudit = heightTableSnapshot(t, db, "USysUserSpp"), heightTableSnapshot(t, db, "USysAuditTrail")
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "zznew01"}); err == nil {
		t.Fatal("case-equivalent personal duplicate accepted")
	}
	checkUnchanged(beforeDefinitions, beforeAudit)
}

func TestPersonalSpeciesUnusableNullAliasDoesNotBecomeAUsableDefinition(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	mutateContextFixture(t, service.projects.supportPaths["VLists"],
		`INSERT INTO USysAllSpecs(Code,OldCode) VALUES (NULL,'zznil01'),('','zzempty')`)
	aliases, err := service.ListVegetationSpeciesAliases(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "ZZNIL01"})
	if err != nil || len(aliases) != 1 || aliases[0].Code != nil {
		t.Fatalf("NULL alias evidence changed: %+v %v", aliases, err)
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID,
		PersonalSpeciesDefinitionRequest{Entered: "ZZNIL01"}); err != nil {
		t.Fatalf("unusable NULL alias blocked the source personal path: %v", err)
	}
	beforeDefinitions, beforeAudit := heightTableSnapshot(t, db, "USysUserSpp"), heightTableSnapshot(t, db, "USysAuditTrail")
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID,
		PersonalSpeciesDefinitionRequest{Entered: "ZZEMPTY"}); err == nil {
		t.Fatal("empty but non-NULL alias was silently treated as NULL")
	}
	if heightTableSnapshot(t, db, "USysUserSpp") != beforeDefinitions || heightTableSnapshot(t, db, "USysAuditTrail") != beforeAudit {
		t.Fatal("rejected non-NULL alias changed records")
	}
}

func TestPersonalSpeciesContextOwnershipCancellationAndConcurrentCollision(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	request := PersonalSpeciesDefinitionRequest{Entered: "ZZNEW01"}
	beforeDefinitions, beforeAudit := heightTableSnapshot(t, db, "USysUserSpp"), heightTableSnapshot(t, db, "USysAuditTrail")
	if _, err := service.plots.createPersonalSpeciesDefinition(request); err == nil {
		t.Fatal("legacy unscoped definition mutation accepted")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.createPersonalSpeciesDefinition(ctx, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write accepted: %v", err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	queued, stop := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := service.createPersonalSpeciesDefinition(queued, state.ContextID, request)
		result <- err
	}()
	stop()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Errorf("queued user write did not cancel: %v", err)
	}
	owner.mu.Unlock()
	original := owner.attachmentInfo["VLists"]
	owner.attachmentInfo["VLists"] = owner.attachmentInfo["VUser"]
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("changed physical reference identity accepted")
	}
	owner.attachmentInfo["VLists"] = original
	if _, err := owner.attach(context.Background(), "shared-user", service.projects.supportPaths["VUser"]); err != nil {
		t.Fatal(err)
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, request); err == nil ||
		!strings.Contains(err.Error(), "shared-role") {
		t.Fatalf("actual duplicate readonly user attachment did not reject writes: %v", err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DETACH DATABASE "shared-user"`); err != nil {
		t.Fatal(err)
	}
	delete(owner.attachments, "shared-user")
	delete(owner.attachmentInfo, "shared-user")
	delete(owner.descriptions, "shared-user")
	if heightTableSnapshot(t, db, "USysUserSpp") != beforeDefinitions || heightTableSnapshot(t, db, "USysAuditTrail") != beforeAudit {
		t.Fatal("context/cancellation/ownership rejection changed records")
	}
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, request)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !strings.Contains(err.Error(), "already has") {
			t.Errorf("concurrent definition failed unexpectedly: %v", err)
		}
	}
	if accepted != 1 {
		t.Fatalf("concurrent duplicate creation accepted %d times", accepted)
	}
	current, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZNEW02"}); err == nil {
		t.Fatal("stale definition writer accepted")
	}
	if err := service.projects.closeSQLiteContext(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), current.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZNEW02"}); err == nil {
		t.Fatal("closed definition writer accepted")
	}
}

func TestPersonalSpeciesWriterReadonlyReferenceAndExistingWAL(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("disposable WAL setup failed: %s %v", mode, err)
	}
	reference := service.projects.supportPaths["VLists"]
	before, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	owner := service.projects.sqlite
	err = owner.withPersonalSpeciesWriter(context.Background(), func(conn *sql.Conn) error {
		var foreignKeys int
		if err := conn.QueryRowContext(context.Background(), `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
			t.Fatalf("user writer foreign keys unavailable: %d %v", foreignKeys, err)
		}
		_, err := conn.ExecContext(context.Background(), `INSERT INTO reference.USysAllSpecs(Code) VALUES ('ZZNOREF')`)
		return err
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Fatalf("reference attachment accepted mutation: %v", err)
	}
	if _, err := service.createPersonalSpeciesDefinition(context.Background(), state.ContextID,
		PersonalSpeciesDefinitionRequest{Entered: "ZZWAL01"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("user definition changed existing journal mode: %s %v", mode, err)
	}
	after, err := os.ReadFile(reference)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("readonly reference bytes changed")
	}
}

func TestPersonalSpeciesCancelBlockedTransactionWithoutDelayedWrite(t *testing.T) {
	service, state, db := personalSpeciesFixture(t)
	beforeDefinitions, beforeAudit := heightTableSnapshot(t, db, "USysUserSpp"), heightTableSnapshot(t, db, "USysAuditTrail")
	lock, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err := lock.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	defer lock.ExecContext(context.Background(), `ROLLBACK`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := service.createPersonalSpeciesDefinition(ctx, state.ContextID, PersonalSpeciesDefinitionRequest{Entered: "ZZCAN01"})
		result <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for owner := service.projects.sqlite; ; {
		if !owner.mu.TryLock() {
			break
		}
		owner.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("user writer did not acquire its operation lease")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	if _, err := lock.ExecContext(context.Background(), `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked user write did not report cancellation: %v", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("canceled user transaction remained blocked")
	}
	if heightTableSnapshot(t, db, "USysUserSpp") != beforeDefinitions || heightTableSnapshot(t, db, "USysAuditTrail") != beforeAudit {
		t.Fatal("canceled transaction committed a delayed definition/audit")
	}
}
