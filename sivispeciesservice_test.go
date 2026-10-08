package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSIVISpeciesFrontendReferenceWireFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("frontend", "src", "siviSpeciesReferenceFixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var references SIVISpeciesReferences
	if err := json.Unmarshal(data, &references); err != nil {
		t.Fatal("frontend fixture is not the strict production reference wire format", err)
	}
	encoded, err := json.Marshal(references)
	if err != nil {
		t.Fatal(err)
	}
	var fixture, wire any
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fixture, wire) {
		t.Fatal("production reference serialization changed the frontend wire fixture")
	}
	if references.Master.Rows[0].RowID != "9223372036854775807" ||
		references.Master.Rows[1].RowID != "-9223372036854775808" {
		t.Fatal("reference fixture lost signed64 physical identities")
	}
}

func siviSpeciesServiceFixture(t *testing.T, extended bool, strength int) (*SIVISpeciesService, *ContextService, ProjectState, *sql.DB, SIVISpeciesWrite) {
	t.Helper()
	contexts, state, db, _, _ := siviWriteFixture(t, extended, strength)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `DELETE FROM USysAllSpecs;
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode) VALUES
		('A',NULL,3,'','U',NULL),('A','duplicate',3,NULL,'U',NULL),
		('C',NULL,12,NULL,'x',NULL),('D',NULL,9,NULL,'u',NULL),
		('new_a',NULL,99,NULL,'S','olddup'),('NEW_B','',3,NULL,'U','olddup'),
		('é😀😀😀',NULL,1,NULL,'X',NULL),('éé😀😀😀',NULL,1,NULL,'X',NULL)`)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["VUser"], `DELETE FROM USysUserSpp;
		INSERT INTO USysUserSpp(Code,ScientificName,LifeForm,EnglishName,Codetype) VALUES
		('Personal',NULL,99,'','S'),('olddup',NULL,1,NULL,'U')`)
	if _, err := db.Exec(`UPDATE Sample_Veg SET rowid=9223372036854775807 WHERE ID=10000001;
		INSERT INTO Sample_Veg(rowid,PlotNumber,Species,ID,Cover6,Collected)
		VALUES (-9223372036854775808,'108050','RAW',10000002,0,'C');
		INSERT INTO Sample_Veg(rowid,PlotNumber,Species,ID,Cover7,Collected)
		VALUES (-9223372036854775807,'108050','RAW',10000003,0,'V')`); err != nil {
		t.Fatal(err)
	}
	service, err := NewSIVISpeciesService(contexts, func(name string) (string, bool) {
		if name != siviSpeciesFeatureEnvironment {
			t.Fatal("Species consulted a predecessor gate", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", extended)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := service.GetReferences(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []SIVISpeciesEdit{}
	for i, id := range []string{"10000001", "10000002", "10000003"} {
		for _, row := range original[i].Rows {
			if row.Cells[0].Integer != nil && *row.Cells[0].Integer == id {
				edits = append(edits, SIVISpeciesEdit{RowID: row.RowID, Form: original[i].Form, Expected: row.Cells[2], Value: []string{"A", "C", "D"}[i]})
			}
		}
	}
	if len(edits) != 3 {
		t.Fatal("controlled Species rows missing", edits)
	}
	return service, contexts, state, db, SIVISpeciesWrite{original, refs, edits}
}

func siviSpeciesRequestJSON(t *testing.T, request SIVISpeciesWrite) string {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSIVISpeciesServiceIndependentGateNilCancellationAndReferences(t *testing.T) {
	service, contexts, state, _, request := siviSpeciesServiceFixture(t, false, 3)
	valid := siviSpeciesRequestJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if len(request.References.Master.Rows) != 8 || len(request.References.Personal.Rows) != 2 ||
		request.References.Master.Rows[0].Cells[1].Storage != "null" ||
		*request.References.Master.Rows[0].Cells[3].Text != "" {
		t.Fatal("references lost duplicate definitions or nullable metadata", request.References)
	}
	contexts.siviHeightEnabled = true
	for _, present := range []bool{false, true} {
		disabled, err := NewSIVISpeciesService(contexts, func(name string) (string, bool) {
			if name != siviSpeciesFeatureEnvironment {
				t.Fatal("foreign gate", name)
			}
			return "false", present
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, err := disabled.GetOriginal(context.Background(), state.ContextID, "108050", false); got != nil || err == nil {
			t.Fatal("disabled original succeeded", got, err)
		}
		if got, err := disabled.GetReferences(context.Background(), state.ContextID, "108050"); got.ContextID != "" || err == nil {
			t.Fatal("disabled reference succeeded", got, err)
		}
		if got, err := disabled.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); got != nil || err == nil {
			t.Fatal("disabled Save succeeded", got, err)
		}
		if got, err := disabled.RestoreReviewed(context.Background(), state.ContextID, "108050", "1", AuditRestorePrune); got != nil || err == nil {
			t.Fatal("disabled Restore succeeded", got, err)
		}
	}
	for _, value := range []string{"", "TRUE", "1", " true ", "yes"} {
		if got, err := NewSIVISpeciesService(contexts, func(string) (string, bool) { return value, true }); got != nil || err == nil {
			t.Fatal("invalid literal gate", value, got, err)
		}
	}
	if got, err := NewSIVISpeciesService(nil, func(string) (string, bool) { return "true", true }); got != nil || err == nil {
		t.Fatal("nil owner", got, err)
	}
	if got, err := NewSIVISpeciesService(contexts, nil); got != nil || err == nil {
		t.Fatal("nil lookup", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, target := range []*SIVISpeciesService{nil, {}, service} {
		for _, ctx := range []context.Context{nil, ctx} {
			if got, err := target.GetOriginal(ctx, state.ContextID, "108050", false); got != nil || err == nil {
				t.Fatal("nil/canceled original", got, err)
			}
			if got, err := target.GetReferences(ctx, state.ContextID, "108050"); got.ContextID != "" || err == nil {
				t.Fatal("nil/canceled references", got, err)
			}
			if got, err := target.SaveReviewed(ctx, state.ContextID, "108050", false, valid); got != nil || err == nil {
				t.Fatal("nil/canceled Save", got, err)
			}
			if got, err := target.RestoreReviewed(ctx, state.ContextID, "108050", "1", AuditRestoreRetain); got != nil || err == nil {
				t.Fatal("nil/canceled Restore", got, err)
			}
		}
	}
	for _, plot := range []string{"foreign", "", "108050 "} {
		if got, err := service.GetReferences(context.Background(), state.ContextID, plot); err == nil || got.ContextID != "" {
			t.Fatal("foreign parent reference success", got, err)
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVISpeciesServiceAtomicWritesStrengthsRestorationAliasesAndReplay(t *testing.T) {
	for _, extended := range []bool{false, true} {
		for strength := 0; strength <= 3; strength++ {
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
				t.Run(strconv.FormatBool(extended)+"/"+strconv.Itoa(strength)+"/"+string(action), func(t *testing.T) {
					service, contexts, state, db, request := siviSpeciesServiceFixture(t, extended, strength)
					unrelated := databaseBytes(t, contexts.projects.sqlite.attachments)
					for role, path := range contexts.projects.sqlite.attachments {
						if path == contexts.projects.sqlite.attachments["project"] {
							delete(unrelated, role)
						}
					}
					result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", extended, siviSpeciesRequestJSON(t, request))
					if err != nil || result == nil || result.ChangedCells != 3 || (result.HistoryID == "") != (strength == 0) {
						t.Fatal("atomic Species mutation differs", result, err)
					}
					for i, want := range []string{"A", "C", "D"} {
						var value string
						if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || value != want {
							t.Fatal(i, value, want, err)
						}
					}
					var count int
					expected := []int{0, 3, 3, 3}[strength]
					if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE EditField='Species'`).Scan(&count); err != nil || count != expected {
						t.Fatal("Species audit count", count, expected, err)
					}
					for _, table := range []string{siviHeightHistoryTable, siviCoverHistoryTable, siviCombinedHistoryTable, siviCollectedHistoryTable} {
						if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, table).Scan(&count); err != nil || count != 0 {
							t.Fatal("Species touched predecessor history", table, count, err)
						}
					}
					assertProfileSUFiles(t, contexts, unrelated)
					if strength == 0 {
						return
					}
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='Sample_Veg' WHERE EditField='Species'`); err != nil {
						t.Fatal(err)
					}
					beforeCancel := heightTableSnapshot(t, db, "Sample_Veg")
					canceled, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreCancel)
					if err != nil || canceled == nil || heightTableSnapshot(t, db, "Sample_Veg") != beforeCancel {
						t.Fatal("restore cancel wrote", canceled, err)
					}
					restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", result.HistoryID, action)
					if err != nil || restored == nil || restored.RestoredRows != 3 || restored.CleanedVegRows != 0 {
						t.Fatal("typed Species restore differs", restored, err)
					}
					for i := 0; i < 3; i++ {
						var value string
						if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE ID=?`, 10000001+i).Scan(&value); err != nil || value != "RAW" {
							t.Fatal("restored Species", value, err)
						}
					}
					if replay, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", result.HistoryID, action); err == nil || replay != nil {
						t.Fatal("history replay accepted", replay, err)
					}
					assertProfileSUFiles(t, contexts, unrelated)
				})
			}
		}
	}
}

func TestSIVISpeciesServicePreservesCallerFormDecisionsAndNoop(t *testing.T) {
	service, contexts, state, db, request := siviSpeciesServiceFixture(t, false, 3)
	// The controlled A row also belongs to C. Its caller form, not the first
	// projection containing its rowid, determines the permitted species list.
	edit := request.Edits[0]
	edit.Form, edit.Value = "SubVegC-SIVI", "C"
	request.Edits = []SIVISpeciesEdit{edit}
	result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request))
	if err != nil || result == nil || result.ChangedCells != 1 {
		t.Fatal("overlapping physical row lost caller form", result, err)
	}
	for _, decision := range []string{"keep", "replace", "user"} {
		original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
		if err != nil {
			t.Fatal(err)
		}
		request.Original = original
		edit.Expected = metadataText("C")
		for _, row := range original[1].Rows {
			if row.RowID == edit.RowID {
				edit.Expected = row.Cells[2]
			}
		}
		text := func(s string) *string { return &s }
		edit.Decision, edit.Entered, edit.Selected = decision, text("olddup"), nil
		switch decision {
		case "keep":
			edit.Value = "OLDDUP"
		case "replace":
			edit.Value, edit.Selected = "NEW_A", text("new_a")
		case "user":
			edit.Value, edit.Entered, edit.Selected = "PERSONAL", text("personal"), text("Personal")
		}
		request.Edits = []SIVISpeciesEdit{edit}
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request)); err != nil || result.ChangedCells != 1 {
			t.Fatal("source NotInList decision failed", decision, result, err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Species='historical overlength invalid' WHERE ID=10000001`); err != nil {
		t.Fatal(err)
	}
	request.Original, err = service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range request.Original[1].Rows {
		if row.RowID == edit.RowID {
			edit.Expected = row.Cells[2]
		}
	}
	edit.Value, edit.Decision, edit.Entered, edit.Selected = *edit.Expected.Text, "", nil, nil
	request.Edits = []SIVISpeciesEdit{edit}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request)); err != nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("unchanged historical invalid Species was validated/written", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	edit.Value = "éé😀😀😀"
	edit.Form = request.Original[0].Form
	request.Edits = []SIVISpeciesEdit{edit}
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request)); err != nil || result.ChangedCells != 1 {
		t.Fatal("exact non-ASCII listed code failed", result, err)
	}
}

func TestSIVISpeciesServiceRejectsSourceCASReferencesAndMalformedBatches(t *testing.T) {
	service, contexts, state, db, request := siviSpeciesServiceFixture(t, false, 3)
	valid := siviSpeciesRequestJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, mutate := range []func(*SIVISpeciesWrite){
		func(r *SIVISpeciesWrite) { r.Original = r.Original[:2] },
		func(r *SIVISpeciesWrite) { r.Edits = nil },
		func(r *SIVISpeciesWrite) { r.Edits[0].RowID = "10000001" },
		func(r *SIVISpeciesWrite) { r.Edits[0].RowID = "+9223372036854775807" },
		func(r *SIVISpeciesWrite) { r.Edits[0].Form = "SubVegA-SIVI" },
		func(r *SIVISpeciesWrite) { r.Edits[0].Expected = metadataText("stale") },
		func(r *SIVISpeciesWrite) { r.Edits[0].Value = "" },
		func(r *SIVISpeciesWrite) { r.Edits[0].Value = "é😀😀😀😀" },
		func(r *SIVISpeciesWrite) { r.Edits[0].Value = "PERSONAL" },
		func(r *SIVISpeciesWrite) { r.Edits[0].Value = "C" },
		func(r *SIVISpeciesWrite) { r.References.ContextID = "foreign" },
		func(r *SIVISpeciesWrite) { r.References.Project = "foreign" },
		func(r *SIVISpeciesWrite) { r.References.Plot = "foreign" },
		func(r *SIVISpeciesWrite) { r.References.Master.Rows[1].Cells[1] = metadataText("changed") },
		func(r *SIVISpeciesWrite) { r.References.Personal.Rows = r.References.Personal.Rows[:1] },
		func(r *SIVISpeciesWrite) {
			duplicate := r.Edits[0]
			duplicate.Form, duplicate.Value = "SubVegC-SIVI", "C"
			r.Edits = append(r.Edits, duplicate)
		},
	} {
		var bad SIVISpeciesWrite
		if err := json.Unmarshal([]byte(valid), &bad); err != nil {
			t.Fatal(err)
		}
		mutate(&bad)
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, bad)); err == nil || result != nil {
			t.Fatal("stale/foreign/malformed Save accepted", bad, result, err)
		}
	}
	assertProfileSUFiles(t, contexts, before)
	for _, change := range []struct{ apply, undo string }{
		{`UPDATE Sample_Veg SET Species='drift' WHERE ID=10000001`, `UPDATE Sample_Veg SET Species='RAW' WHERE ID=10000001`},
		{`UPDATE Sample_Veg SET Cover1=NULL,Cover6=NULL WHERE ID=10000001`, `UPDATE Sample_Veg SET Cover1=0,Cover6=0 WHERE ID=10000001`},
		{`UPDATE Sample_Veg SET PlotNumber='foreign' WHERE ID=10000001`, `UPDATE Sample_Veg SET PlotNumber='108050' WHERE ID=10000001`},
	} {
		if _, err := db.Exec(change.apply); err != nil {
			t.Fatal(err)
		}
		pre := heightTableSnapshot(t, db, "Sample_Veg")
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
			t.Fatal("source drift Save accepted", result, err)
		}
		if heightTableSnapshot(t, db, "Sample_Veg") != pre {
			t.Fatal("rejected source Save wrote")
		}
		if _, err := db.Exec(change.undo); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{"VLists", "VUser"} {
		table := "USysAllSpecs"
		if role == "VUser" {
			table = "USysUserSpp"
		}
		path := contexts.projects.sqlite.attachments[role]
		mutateContextFixture(t, path, `UPDATE `+table+` SET EnglishName='unused reference drift' WHERE Code='`+map[string]string{"VLists": "D", "VUser": "Personal"}[role]+`'`)
		pre := heightTableSnapshot(t, db, "Sample_Veg")
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
			t.Fatal("unused reference drift accepted", role, result, err)
		}
		if heightTableSnapshot(t, db, "Sample_Veg") != pre {
			t.Fatal("reference drift caused partial writes")
		}
	}
	request.References, _ = service.GetReferences(context.Background(), state.ContextID, "108050")
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request)); err != nil || result.ChangedCells != 3 {
		t.Fatal("reload retry failed", result, err)
	}
}

func TestSIVISpeciesServiceRollbackHistoryCollisionRetryAndReadOwnership(t *testing.T) {
	service, contexts, state, db, request := siviSpeciesServiceFixture(t, false, 3)
	valid := siviSpeciesRequestJSON(t, request)
	for _, trigger := range []struct{ setup, cleanup string }{
		{`CREATE TRIGGER species_abort BEFORE UPDATE OF Species ON Sample_Veg WHEN OLD.ID=10000002 BEGIN SELECT RAISE(ABORT,'blocked Species'); END`, `DROP TRIGGER species_abort`},
		{`CREATE TRIGGER species_drift AFTER UPDATE OF Species ON Sample_Veg BEGIN UPDATE Sample_Veg SET Collected='C' WHERE ID=10000003; END`, `DROP TRIGGER species_drift`},
		{`CREATE TABLE "__VPRO_SIVISpeciesHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT);
		 INSERT INTO "__VPRO_SIVISpeciesHistory" VALUES(9223372036854775807,'occupied','foreign',NULL)`, `DROP TABLE "__VPRO_SIVISpeciesHistory"`},
	} {
		if _, err := db.Exec(trigger.setup); err != nil {
			t.Fatal(err)
		}
		pre := heightTableSnapshot(t, db, "Sample_Veg")
		audit := heightTableSnapshot(t, db, "Sample_Audit")
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
			t.Fatal("rollback/collision Save accepted", result, err)
		}
		if pre != heightTableSnapshot(t, db, "Sample_Veg") || audit != heightTableSnapshot(t, db, "Sample_Audit") {
			t.Fatal("rejected Species Save retained partial writes")
		}
		if _, err := db.Exec(trigger.cleanup); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{"VLists", "VUser"} {
		path := contexts.projects.sqlite.attachments[role]
		info := contexts.projects.sqlite.attachmentInfo[role]
		contexts.projects.sqlite.attachmentInfo[role] = contexts.projects.sqlite.attachmentInfo["project"]
		if got, err := service.GetReferences(context.Background(), state.ContextID, "108050"); err == nil || got.ContextID != "" {
			t.Fatal("wrong owner returned partial references", got, err)
		}
		if got, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || got != nil {
			t.Fatal("wrong owner Save succeeded", got, err)
		}
		contexts.projects.sqlite.attachmentInfo[role] = info
		if observed, err := os.Stat(path); err != nil || !os.SameFile(info, observed) {
			t.Fatal("test changed owned fixture", err)
		}
	}
	personalPath := contexts.projects.sqlite.attachments["VUser"]
	mutateContextFixture(t, personalPath, `ALTER TABLE USysUserSpp RENAME COLUMN LifeForm TO MissingLifeform`)
	if got, err := service.GetReferences(context.Background(), state.ContextID, "108050"); err == nil || got.ContextID != "" || !reflect.DeepEqual(got.Master, ProjectMetadataTable{}) {
		t.Fatal("malformed personal schema returned partial master definitions", got, err)
	}
	mutateContextFixture(t, personalPath, `ALTER TABLE USysUserSpp RENAME COLUMN MissingLifeform TO LifeForm`)
	if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err != nil || result.ChangedCells != 3 {
		t.Fatal("rollback/ownership retry failed", result, err)
	}
}

func TestSIVISpeciesServiceOwnedWaitCancellationAndReadonlyUserAlias(t *testing.T) {
	service, contexts, state, _, request := siviSpeciesServiceFixture(t, false, 3)
	owner := contexts.projects.sqlite
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	owner.mu.Lock()
	got, err := service.GetReferences(ctx, state.ContextID, "108050")
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || got.ContextID != "" {
		t.Fatal("owned reference wait ignored cancellation", got, err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	owner.mu.Lock()
	result, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, siviSpeciesRequestJSON(t, request))
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || result != nil {
		t.Fatal("owned writer wait ignored cancellation", result, err)
	}
	err = owner.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS "sivi_species_user"`, sqliteFileURI(owner.attachments["VUser"], "ro")); err != nil {
			return err
		}
		_, err := conn.ExecContext(context.Background(), `INSERT INTO sivi_species_user.USysUserSpp(Code) VALUES('FORBID')`)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "readonly") {
			return errors.New("personal reference alias was writable")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
