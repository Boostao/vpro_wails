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
	"sync"
	"testing"
	"time"
)

func siviProjectAssignmentWriteFixture(t *testing.T, external bool, source, strength int, before any) (*ContextService, ProjectState, *sql.DB, *siviParentProjection, siviProjectSelection) {
	t.Helper()
	service, state := contextServiceFixture(t)
	if external {
		path := filepath.Join(t.TempDir(), "external # assignment.db")
		data, err := os.ReadFile(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		selected := contextSelection(state)
		selected.ProjectPath = path
		state, err = service.SwitchContext(state.ContextID, selected)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := service.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID=? WHERE PlotNumber='108050';
		DELETE FROM Sample_Metadata; INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle)
		VALUES(1,'  Existing 🌱  ',''),(2,'  Existing 🌱  ',NULL)`, before); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, service.projects.supportPaths["VMetaData"], `DELETE FROM ProjectMetadata;
		INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES('other',''),('  Existing 🌱  ',NULL)`)
	parent, selection := siviProjectAssignmentWriteSelection(t, service, state)
	return service, state, db, parent, selection
}

func siviProjectAssignmentWriteSelection(t *testing.T, service *ContextService, state ProjectState) (*siviParentProjection, siviProjectSelection) {
	t.Helper()
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	choices, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return parent, siviProjectSelection{
		ContextID: state.ContextID, ControlID: "form:frmSIVIsite/ProjectID", Table: parent.EnvTable,
		RowID: parent.Rows[0].Env.RowID, Expected: siviParentCell(t, parent, "ProjectID"),
		SourceOption: choices.SourceOption, MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns:  append([]ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: choices.Choices.Rows[len(choices.Choices.Rows)-1],
	}
}

func siviProjectAssignmentStoredEvent(t *testing.T, db *sql.DB, id string) siviParentHistory {
	t.Helper()
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIProjectAssignmentHistory" WHERE ID=?`, id).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var event siviParentHistory
	if err := json.Unmarshal([]byte(proposal), &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestSIVIProjectAssignmentWriterOwnedSourcesStrengthAndTypedRestoration(t *testing.T) {
	for _, external := range []bool{false, true} {
		for source := 1; source <= 2; source++ {
			for strength := 0; strength <= 3; strength++ {
				t.Run(strconv.FormatBool(external)+"/"+strconv.Itoa(source)+"/"+strconv.Itoa(strength), func(t *testing.T) {
					service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, external, source, strength, "old")
					support := databaseBytes(t, service.projects.sqlite.attachments)
					for role, path := range service.projects.sqlite.attachments {
						if path == state.ProjectPath {
							delete(support, role)
						}
					}
					config, err := os.ReadFile(service.projects.preferences.path)
					if err != nil {
						t.Fatal(err)
					}
					prior, err := service.ListAuditEntries(context.Background(), state.ContextID, "108050")
					if err != nil {
						t.Fatal(err)
					}
					written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
					if err != nil || written == nil || written.ChangedCells != 1 || (written.HistoryID == "") != (strength == 0) {
						t.Fatal("assignment/source/strength failed", written, err)
					}
					fresh, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
					if err != nil || !reflect.DeepEqual(siviParentCell(t, fresh, "ProjectID"), selection.MetadataOriginal.Cells[0]) {
						t.Fatal("assignment normalized its raw ID", fresh, err)
					}
					audits, err := service.ListAuditEntries(context.Background(), state.ContextID, "108050")
					if err != nil || len(audits) != len(prior)+map[bool]int{true: 1, false: 0}[strength > 0] {
						t.Fatal("assignment created phantom audits", audits, err)
					}
					if strength > 0 {
						event := siviProjectAssignmentStoredEvent(t, db, written.HistoryID)
						if !reflect.DeepEqual(event.Original, parent) || !reflect.DeepEqual(event.Committed, fresh) ||
							!reflect.DeepEqual(event.ProjectAssignment.Selection, selection) ||
							event.ProjectAssignment.Choices.Alias != selection.MetadataAlias ||
							event.ProjectAssignment.Selection.MetadataOriginal.Cells[1].Storage != "null" {
							t.Fatal("history lost full raw parent/source/duplicate provenance", event)
						}
						record := event.Changes[0].Audit
						if record.Table != "_Env" || record.EditField != "ProjectID" || record.ID != nil ||
							record.User != service.plots.currentUser || record.Project != "Sample" ||
							record.PlotNumber != "108050" || record.Restore || record.Flag ||
							!metadataAuditTextEqual(record.BeforeEdit, "old") ||
							!metadataAuditTextEqual(record.AfterEdit, "  Existing 🌱  ") {
							t.Fatal("assignment audit differs from raw storage", record)
						}
						if _, err := siviParentHistoryAssignments(context.Background(), event, "Sample", "108050"); err == nil {
							t.Fatal("direct validator accepted assignment history")
						}
						if _, err := siviParentActionHistoryAssignments(context.Background(), event, "Sample", "108050"); err == nil {
							t.Fatal("action validator accepted assignment history")
						}
						if !external && source == 1 && strength == 3 {
							for _, domain := range []siviParentHistoryDomain{siviParentDirectHistory, siviParentActionHistory} {
								if _, err := db.Exec(domain.schema + `; INSERT INTO ` + quoteHeaderIdentifier(domain.table) +
									`(ID,Created,Proposal) SELECT ID,Created,Proposal FROM "__VPRO_SIVIProjectAssignmentHistory"`); err != nil {
									t.Fatal(err)
								}
							}
						}
						if got, err := service.restoreSIVIParentDirect(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err == nil || got != nil {
							t.Fatal("direct namespace restored assignment")
						}
						if got, err := service.restoreSIVIParentActions(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err == nil || got != nil {
							t.Fatal("action namespace restored assignment")
						}
						if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='sAmPlE_EnV' WHERE rowid=?`, record.RowID); err != nil {
							t.Fatal(err)
						}
						before := databaseBytes(t, service.projects.sqlite.attachments)
						cancelled, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreCancel)
						if err != nil || cancelled == nil || !cancelled.Cancelled {
							t.Fatal("restoration cancel failed", cancelled, err)
						}
						assertProfileSUFiles(t, service, before)
						action := AuditRestoreRetain
						if strength == 2 {
							action = AuditRestorePrune
						}
						restored, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, action)
						if err != nil || restored == nil || restored.RestoredRows != 1 ||
							restored.PrunedAuditRows != map[bool]int{true: 1, false: 0}[action == AuditRestorePrune] {
							t.Fatal("typed restoration failed", restored, err)
						}
						fresh, err = service.readSIVIParent(context.Background(), state.ContextID, "108050")
						if err != nil || !reflect.DeepEqual(fresh, parent) {
							t.Fatal("restoration normalized original", fresh, err)
						}
						var count, truth int
						if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, record.RowID).Scan(&count, &truth); err != nil ||
							(action == AuditRestoreRetain && (count != 1 || truth != -1)) ||
							(action == AuditRestorePrune && count != 0) {
							t.Fatal("audit prune/Access true=-1 changed", count, truth, err)
						}
						before = databaseBytes(t, service.projects.sqlite.attachments)
						if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, action); err == nil || got != nil {
							t.Fatal("restoration replayed", got, err)
						}
						assertProfileSUFiles(t, service, before)
					}
					assertProfileSUFiles(t, service, support)
					after, err := os.ReadFile(service.projects.preferences.path)
					if err != nil || !reflect.DeepEqual(config, after) {
						t.Fatal("assignment changed preferences", err)
					}
				})
			}
		}
	}
}

func TestSIVIProjectAssignmentWriterHistoricalNoopsAndUnauditedPreservation(t *testing.T) {
	for index, before := range []any{nil, "", strings.Repeat("🌱", 16), int64(2)} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, before)
			if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, before); err != nil {
				t.Fatal(err)
			}
			parent, selection = siviProjectAssignmentWriteSelection(t, service, state)
			bytes := databaseBytes(t, service.projects.sqlite.attachments)
			written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.ChangedCells != 0 || written.HistoryID != "" {
				t.Fatal("same value from physical duplicate created history/audit", written, err)
			}
			assertProfileSUFiles(t, service, bytes)
			if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID='new'`); err != nil {
				t.Fatal(err)
			}
			_, selection = siviProjectAssignmentWriteSelection(t, service, state)
			written, err = service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.HistoryID == "" {
				t.Fatal("valid historical correction failed", written, err)
			}
			if _, err := db.Exec(`DELETE FROM Sample_Metadata`); err != nil {
				t.Fatal(err)
			}
			if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
				t.Fatal(err)
			}
			if _, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err != nil {
				t.Fatal("restoration inferred/required a current metadata definition", err)
			}
			fresh, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(fresh, parent) {
				t.Fatal("restoration changed historical original storage", fresh, err)
			}
			var remaining int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Metadata`).Scan(&remaining); err != nil || remaining != 0 {
				t.Fatal("restoration recreated metadata", remaining, err)
			}
		})
	}
	for strength := 0; strength <= 3; strength++ {
		service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 2, strength, nil)
		written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
		if err != nil || written == nil || written.ChangedCells != 1 || (written.HistoryID == "") != (strength < 2) {
			t.Fatal("NULL addition audit strengths differ", strength, written, err)
		}
		if strength < 2 {
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", "1", AuditRestorePrune); err == nil || got != nil {
				t.Fatal("unaudited assignment was restored/pruned")
			}
			assertProfileSUFiles(t, service, before)
		}
	}
}

func TestSIVIProjectAssignmentWriterExactUTF16UnavailableAndDrift(t *testing.T) {
	service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	for _, value := range []any{nil, "", strings.Repeat("🌱", 15) + "x", string([]byte{0xff}), []byte{0xff}} {
		if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, value); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		current := selection
		switch value := value.(type) {
		case nil:
			current.MetadataOriginal.Cells = []ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}}
		case string:
			current.MetadataOriginal.Cells = []ProjectMetadataCell{metadataText(value), {Storage: "null"}}
		case []byte:
			hex := "ff"
			current.MetadataOriginal.Cells = []ProjectMetadataCell{{Storage: "blob", BlobHex: &hex}, {Storage: "null"}}
		}
		if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, current); err == nil || got != nil {
			t.Fatal("drift/unavailable choice wrote", value, got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, strings.Repeat("🌱", 15)); err != nil {
		t.Fatal(err)
	}
	_, selection = siviProjectAssignmentWriteSelection(t, service, state)
	for _, test := range []struct{ change, undo string }{
		{`UPDATE Sample_Env SET FieldNumber='drift' WHERE PlotNumber='108050'`, `UPDATE Sample_Env SET FieldNumber=NULL WHERE PlotNumber='108050'`},
		{`UPDATE Sample_Metadata SET ProjectTitle='' WHERE ID=2`, `UPDATE Sample_Metadata SET ProjectTitle=NULL WHERE ID=2`},
		{`ALTER TABLE Sample_Env ADD COLUMN Drift TEXT`, ""},
	} {
		if _, err := db.Exec(test.change); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
			t.Fatal("whole parent/metadata/schema drift wrote", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if test.undo != "" {
			if _, err := db.Exec(test.undo); err != nil {
				t.Fatal(err)
			}
		}
	}
	parent, selection = siviProjectAssignmentWriteSelection(t, service, state)
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
		t.Fatal("changed preference authorized old selection")
	}
	assertProfileSUFiles(t, service, before)
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 1}); err != nil {
		t.Fatal(err)
	}
	written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("exact30 UTF16 assignment failed", written, err)
	}
}

func TestSIVIProjectAssignmentWriterTriggersSchemaRollbackAndRetry(t *testing.T) {
	service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	for _, test := range []struct{ change, undo string }{
		{`CREATE TRIGGER AssignmentParent AFTER UPDATE OF ProjectID ON Sample_Env BEGIN UPDATE Sample_Admin SET PlotType='BAD' WHERE Plot='108050'; END`, `DROP TRIGGER AssignmentParent`},
		{`CREATE TRIGGER AssignmentMetadata AFTER UPDATE OF ProjectID ON Sample_Env BEGIN UPDATE Sample_Metadata SET ProjectTitle='BAD'; END`, `DROP TRIGGER AssignmentMetadata`},
		{`CREATE TRIGGER AssignmentAudit AFTER INSERT ON Sample_Audit BEGIN UPDATE Sample_Audit SET Flag=-1 WHERE rowid=NEW.rowid; END`, `DROP TRIGGER AssignmentAudit`},
		{`CREATE TABLE "__VPRO_SIVIProjectAssignmentHistory"(ID INTEGER PRIMARY KEY,Created TEXT,Proposal TEXT,Restored TEXT)`, `DROP TABLE "__VPRO_SIVIProjectAssignmentHistory"`},
		{`CREATE TABLE "__VPRO_SIVIProjectAssignmentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT);
			CREATE TRIGGER AssignmentHistory AFTER INSERT ON "__VPRO_SIVIProjectAssignmentHistory" BEGIN UPDATE Sample_Env SET FieldNumber='BAD' WHERE PlotNumber='108050'; END`,
			`DROP TRIGGER AssignmentHistory; DROP TABLE "__VPRO_SIVIProjectAssignmentHistory"`},
	} {
		if _, err := db.Exec(test.change); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
			t.Fatal("trigger/provenance drift committed", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.undo); err != nil {
			t.Fatal(err)
		}
	}
	hooks := service.siviProjectAssignmentWriteHooks(context.Background(), state.ContextID, "108050", selection)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	err := withSIVIParentWriteTransaction(context.Background(), service.projects.sqlite, "schema guard test", func(tx *sql.Tx, suAlias string) error {
		if _, err := hooks.plan(tx, parent); err != nil {
			return err
		}
		_, err := tx.Exec(`CREATE INDEX AssignmentUnexpectedIndex ON Sample_Metadata(ProjectTitle)`)
		return err
	}, hooks)
	if err == nil {
		t.Fatal("metadata schema changed between plan and commit")
	}
	assertProfileSUFiles(t, service, before)
	if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err != nil || got == nil || got.ChangedCells != 1 {
		t.Fatal("rollback poisoned assignment retry", got, err)
	}
}

func TestSIVIProjectAssignmentWriterCancellationCollisionAndPreferenceLease(t *testing.T) {
	service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.writeSIVIProjectAssignment(ctx, state.ContextID, "108050", parent, selection); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("pre-cancelled assignment wrote", got, err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	got, writeErr := service.writeSIVIProjectAssignment(ctx, state.ContextID, "108050", parent, selection)
	cancel()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(writeErr, context.DeadlineExceeded) || got != nil {
		t.Fatal("collision ignored cancellation", got, writeErr)
	}
	assertProfileSUFiles(t, service, before)
	hooks := service.siviProjectAssignmentWriteHooks(context.Background(), state.ContextID, "108050", selection)
	err = withSIVIParentWriteTransaction(context.Background(), service.projects.sqlite, "preference guard test", func(tx *sql.Tx, alias string) error {
		if _, err := hooks.plan(tx, parent); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()
		failed := make(chan error, 1)
		go func() { failed <- service.projects.preferences.compareAndSetProjectIDSource(ctx, 2, 1) }()
		if err := <-failed; !errors.Is(err, context.DeadlineExceeded) {
			return errors.New("source preference switched during pinned metadata validation")
		}
		master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw")+"&_busy_timeout=50")
		if err != nil {
			return err
		}
		defer master.Close()
		if _, err := master.Exec(`UPDATE ProjectMetadata SET ProjectTitle='concurrent drift'`); err == nil {
			return errors.New("master metadata write escaped the transaction-held read lock")
		}
		return nil
	}, hooks)
	if err != nil {
		t.Fatal("preference/metadata source guard failed", err)
	}
	assertProfileSUFiles(t, service, before)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "parents changed") {
			t.Fatal("collision returned unexpected failure", err)
		}
	}
	if success != 1 {
		t.Fatal("assignment collision elected multiple writers", success)
	}
}

func TestSIVIProjectAssignmentWriterOwnershipUserAndSelectedSU(t *testing.T) {
	service, state, db, _, _ := siviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	if _, err := db.Exec(`CREATE TABLE Assignment_SU(PlotNumber TEXT,SiteUnit TEXT);
			INSERT INTO Assignment_SU VALUES('108050','unit')`); err != nil {
		t.Fatal(err)
	}
	selected := contextSelection(state)
	selected.SU, selected.SUPath = "Assignment", state.ProjectPath
	var err error
	state, err = service.SwitchContext(state.ContextID, selected)
	if err != nil {
		t.Fatal(err)
	}
	parent, selection := siviProjectAssignmentWriteSelection(t, service, state)
	owner := service.projects.sqlite
	if _, err := db.Exec(`DELETE FROM Assignment_SU`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, owner.attachments)
	if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
		t.Fatal("assignment wrote outside selected SU")
	}
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO Assignment_SU VALUES('108050','unit')`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, owner.attachments)
	for _, role := range []string{"project", "VMetaData", "VLists", "su"} {
		info := owner.attachmentInfo[role]
		foreign := owner.attachmentInfo["VLists"]
		if role == "VLists" {
			foreign = owner.attachmentInfo["project"]
		}
		owner.attachmentInfo[role] = foreign
		got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
		owner.attachmentInfo[role] = info
		if err == nil || got != nil {
			t.Fatal("assignment ignored independently replaced attached file", role)
		}
		assertProfileSUFiles(t, service, before)
	}
	for _, user := range []string{"", strings.Repeat("x", 101), string([]byte{0xff})} {
		prior := service.plots.currentUser
		service.plots.currentUser = user
		got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
		service.plots.currentUser = prior
		if err == nil || got != nil {
			t.Fatal("assignment accepted invalid audit user", user)
		}
		assertProfileSUFiles(t, service, before)
	}
	if got, err := service.writeSIVIProjectAssignment(context.Background(), "stale", "108050", parent, selection); err == nil || got != nil {
		t.Fatal("assignment accepted stale context")
	}
	assertProfileSUFiles(t, service, before)
	written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("ownership/SU correction failed retry", written, err)
	}
}

func TestSIVIProjectAssignmentHistoryRejectsDirectAndActionEvents(t *testing.T) {
	service, state, db, parent, edits := siviParentWriteFixture(t, false, 3)
	direct, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, edits)
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentHistory" WHERE ID=?`, direct.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(siviProjectAssignmentHistorySQL+`; INSERT INTO "__VPRO_SIVIProjectAssignmentHistory"(ID,Created,Proposal)
			VALUES(1,'copied direct',?)`, proposal); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", "1", AuditRestoreRetain); err == nil || got != nil {
		t.Fatal("assignment namespace restored a copied direct event")
	}
	assertProfileSUFiles(t, service, before)
	if _, err := service.restoreSIVIParentDirect(context.Background(), state.ContextID, "108050", direct.HistoryID, AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050';
			UPDATE Sample_Admin SET PlotType='legacy' WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	parent, err = service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	action, err := service.writeSIVIParentActions(context.Background(), state.ContextID, "108050", parent, siviActionProposalEdits(parent))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentActionHistory" WHERE ID=?`, action.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=1`, proposal); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", "1", AuditRestorePrune); err == nil || got != nil {
		t.Fatal("assignment namespace restored a copied action event")
	}
	assertProfileSUFiles(t, service, before)
}
func TestSIVIProjectAssignmentRestorationMalformedHistoryAndUnauditedDeletion(t *testing.T) {
	service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, strings.Repeat("x", 31))
	written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil {
		t.Fatal(err)
	}
	event := siviProjectAssignmentStoredEvent(t, db, written.HistoryID)
	original, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for index, mutation := range []func(*siviParentHistory){
		func(e *siviParentHistory) { e.ProjectAssignment = nil },
		func(e *siviParentHistory) { e.ProjectAssignment.Selection.SourceOption = 2 },
		func(e *siviParentHistory) { e.ProjectAssignment.Selection.MetadataColumns = nil },
		func(e *siviParentHistory) {
			e.ProjectAssignment.Selection.MetadataColumns[0].DeclaredType += "_changed"
		},
		func(e *siviParentHistory) { e.ProjectAssignment.Selection.MetadataOriginal.Cells[1] = metadataText("") },
		func(e *siviParentHistory) { e.ProjectAssignment.Schema = nil },
		func(e *siviParentHistory) { value := "malformed schema"; e.ProjectAssignment.Schema[0].SQL = &value },
		func(e *siviParentHistory) { e.ProjectAssignment.Columns[0].DeclaredType = "BLOB" },
		func(e *siviParentHistory) { e.Changes[0].Column = "SV_PolygonNumber" },
		func(e *siviParentHistory) { e.Changes = append(e.Changes, e.Changes[0]) },
		func(e *siviParentHistory) { e.Committed.Rows[0].Env.Cells[0] = metadataText("other") },
		func(e *siviParentHistory) { value := "forged"; e.Changes[0].Audit.AfterEdit = &value },
	} {
		var candidate siviParentHistory
		if err := json.Unmarshal(original, &candidate); err != nil {
			t.Fatal(err)
		}
		mutation(&candidate)
		data, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=?`, string(data), written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestorePrune); err == nil || got != nil {
			t.Fatal("malformed assignment provenance restored/pruned", index, got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	for _, proposal := range []string{string(original) + "{}",
		strings.Replace(string(original), `"ProjectAssignment":`, `"Unknown":true,"ProjectAssignment":`, 1),
		strings.Replace(string(original), `"ProjectID"`, `"\ud800"`, 1)} {
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=?`, proposal, written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err == nil || got != nil {
			t.Fatal("malformed raw JSON history restored", got, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=?`, string(original), written.HistoryID); err != nil {
		t.Fatal(err)
	}
	for _, drift := range []struct{ change, undo string }{
		{`UPDATE Sample_Env SET FieldNumber='unaudited' WHERE PlotNumber='108050'`, `UPDATE Sample_Env SET FieldNumber=NULL WHERE PlotNumber='108050'`},
		{`DELETE FROM Sample_Audit WHERE rowid=` + event.Changes[0].Audit.RowID, ""},
	} {
		if _, err := db.Exec(drift.change); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestorePrune); err == nil || got != nil {
			t.Fatal("restoration overwrote unaudited edits/deleted audit", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if drift.undo != "" {
			if _, err := db.Exec(drift.undo); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSIVIProjectAssignmentMasterWALUnavailableAndRollbackJournalRetry(t *testing.T) {
	service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL fixture unavailable", mode, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil ||
		got != nil || !strings.Contains(err.Error(), "rollback-journal") {
		t.Fatal("read-only WAL snapshot authorized unguarded metadata", got, err)
	}
	assertProfileSUFiles(t, service, before)
	if err := master.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatal("rollback journal recovery failed", mode, err)
	}
	if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err != nil || got == nil {
		t.Fatal("safe metadata retry failed", got, err)
	}
}

func TestSIVIProjectAssignmentRestorationTriggerRollbackCancellationAndRetry(t *testing.T) {
	service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.restoreSIVIProjectAssignment(ctx, state.ContextID, "108050", written.HistoryID, AuditRestorePrune); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled restoration wrote", got, err)
	}
	assertProfileSUFiles(t, service, before)
	for _, test := range []struct{ change, undo string }{
		{`CREATE TRIGGER AssignmentRestoreAudit AFTER UPDATE OF Restore ON Sample_Audit
					BEGIN UPDATE Sample_Env SET FieldNumber='BAD' WHERE PlotNumber='108050'; END`, `DROP TRIGGER AssignmentRestoreAudit`},
		{`CREATE TRIGGER AssignmentRestoreHistory AFTER UPDATE OF Restored ON "__VPRO_SIVIProjectAssignmentHistory"
					BEGIN UPDATE Sample_Admin SET PlotType='BAD' WHERE Plot='108050'; END`, `DROP TRIGGER AssignmentRestoreHistory`},
	} {
		if _, err := db.Exec(test.change); err != nil {
			t.Fatal(err)
		}
		before = databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err == nil || got != nil {
			t.Fatal("restoration trigger committed partial parents/audit/history", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.undo); err != nil {
			t.Fatal(err)
		}
	}
	next, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.restoreSIVIProjectAssignment(context.Background(), next.ContextID, "108050", written.HistoryID, AuditRestoreRetain)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("restoration rollback/reopened owner retry failed", restored, err)
	}
	fresh, err := service.readSIVIParent(context.Background(), next.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	parent.ContextID = next.ContextID
	if !reflect.DeepEqual(fresh, parent) {
		t.Fatal("reopened restoration changed raw original", fresh, parent)
	}
}

func TestSIVIProjectAssignmentWriterExpectedChoiceSchemaDriftAndRetry(t *testing.T) {
	for _, source := range []int{1, 2} {
		t.Run(strconv.Itoa(source), func(t *testing.T) {
			service, state, db, parent, selection := siviProjectAssignmentWriteFixture(t, true, source, 3, "old")
			path := state.ProjectPath
			table := "Sample_Metadata"
			if source == 2 {
				path, table = service.projects.supportPaths["VMetaData"], "ProjectMetadata"
			}
			sourceDB, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer sourceDB.Close()
			prior := quoteHeaderIdentifier(table + "_prior")
			physical := quoteHeaderIdentifier(table)
			if _, err := sourceDB.Exec(`ALTER TABLE ` + physical + ` RENAME TO ` + prior +
				`; CREATE TABLE ` + physical + `(ProjectID TEXT,ProjectTitle VARCHAR,ID INTEGER);
				INSERT INTO ` + physical + `(rowid,ProjectID,ProjectTitle) SELECT rowid,ProjectID,ProjectTitle FROM ` + prior +
				`; DROP TABLE ` + prior); err != nil {
				t.Fatal(err)
			}
			choices, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			var row ProjectMetadataRow
			for _, candidate := range choices.Choices.Rows {
				if candidate.RowID == selection.MetadataOriginal.RowID {
					row = candidate
				}
			}
			if !reflect.DeepEqual(row, selection.MetadataOriginal) ||
				reflect.DeepEqual(choices.Choices.Columns, selection.MetadataColumns) {
				t.Fatal("schema-only drift fixture changed the raw selected ID/title/row", choices)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", selection); err == nil || got != nil {
				t.Fatal("preview treated row equality as expected column validation", got, err)
			}
			if got, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection); err == nil ||
				got != nil || !strings.Contains(err.Error(), "column names or declared types changed") {
				t.Fatal("writer authorized stale expected choice-column provenance", got, err)
			}
			assertProfileSUFiles(t, service, before)
			selection.MetadataColumns = append([]ProjectMetadataColumn{}, choices.Choices.Columns...)
			written, err := service.writeSIVIProjectAssignment(context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.ChangedCells != 1 {
				t.Fatal("explicitly refreshed choice schema failed retry", written, err)
			}
			event := siviProjectAssignmentStoredEvent(t, db, written.HistoryID)
			if !reflect.DeepEqual(event.ProjectAssignment.Selection.MetadataColumns, choices.Choices.Columns) {
				t.Fatal("history lost actual expected column names/declared types", event)
			}
			if got, err := service.restoreSIVIProjectAssignment(context.Background(), state.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err != nil ||
				got == nil || got.RestoredRows != 1 {
				t.Fatal("expected schema provenance failed typed restoration", got, err)
			}
		})
	}
}
