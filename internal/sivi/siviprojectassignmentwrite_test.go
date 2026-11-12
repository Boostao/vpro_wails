package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"

	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func siviProjectAssignmentStoredEvent(t *testing.T, db *sql.DB, id string) sourceform.SiviParentHistory {
	t.Helper()
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIProjectAssignmentHistory" WHERE ID=?`, id).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var event sourceform.SiviParentHistory
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
					service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, external, source, strength, "old")
					support := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
					for role, path := range service.Projects.Sqlite.Attachments {
						if path == state.ProjectPath {
							delete(support, role)
						}
					}
					config, err := os.ReadFile(service.Projects.Preferences.Path)
					if err != nil {
						t.Fatal(err)
					}
					prior, err := service.ListAuditEntries(context.Background(), state.ContextID, "108050")
					if err != nil {
						t.Fatal(err)
					}
					written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
					if err != nil || written == nil || written.ChangedCells != 1 || (written.HistoryID == "") != (strength == 0) {
						t.Fatal("assignment/source/strength failed", written, err)
					}
					fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
					if err != nil || !reflect.DeepEqual(sftest.SiviParentCell(t, fresh, "ProjectID"), selection.MetadataOriginal.Cells[0]) {
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
							record.User != service.Plots.CurrentUser || record.Project != "Sample" ||
							record.PlotNumber != "108050" || record.Restore || record.Flag ||
							!vprocore.MetadataAuditTextEqual(record.BeforeEdit, "old") ||
							!vprocore.MetadataAuditTextEqual(record.AfterEdit, "  Existing 🌱  ") {
							t.Fatal("assignment audit differs from raw storage", record)
						}
						if _, err := sourceform.SiviParentHistoryAssignments(context.Background(), event, "Sample", "108050"); err == nil {
							t.Fatal("direct validator accepted assignment history")
						}
						if _, err := siviParentActionHistoryAssignments(context.Background(), event, "Sample", "108050"); err == nil {
							t.Fatal("action validator accepted assignment history")
						}
						if !external && source == 1 && strength == 3 {
							for _, domain := range []sourceform.SiviParentHistoryDomain{sourceform.SiviParentDirectHistory, siviParentActionHistory} {
								if _, err := db.Exec(domain.Schema + `; INSERT INTO ` + vprocore.QuoteHeaderIdentifier(domain.Table) +
									`(ID,Created,Proposal) SELECT ID,Created,Proposal FROM "__VPRO_SIVIProjectAssignmentHistory"`); err != nil {
									t.Fatal(err)
								}
							}
						}
						if got, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err == nil || got != nil {
							t.Fatal("direct namespace restored assignment")
						}
						if got, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err == nil || got != nil {
							t.Fatal("action namespace restored assignment")
						}
						if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='sAmPlE_EnV' WHERE rowid=?`, record.RowID); err != nil {
							t.Fatal(err)
						}
						before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
						cancelled, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreCancel)
						if err != nil || cancelled == nil || !cancelled.Cancelled {
							t.Fatal("restoration cancel failed", cancelled, err)
						}
						vprotest.AssertProfileSUFiles(t, service, before)
						action := vprocore.AuditRestoreRetain
						if strength == 2 {
							action = vprocore.AuditRestorePrune
						}
						restored, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, action)
						if err != nil || restored == nil || restored.RestoredRows != 1 ||
							restored.PrunedAuditRows != map[bool]int{true: 1, false: 0}[action == vprocore.AuditRestorePrune] {
							t.Fatal("typed restoration failed", restored, err)
						}
						fresh, err = sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
						if err != nil || !reflect.DeepEqual(fresh, parent) {
							t.Fatal("restoration normalized original", fresh, err)
						}
						var count, truth int
						if err := db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(Restore),0) FROM Sample_Audit WHERE rowid=?`, record.RowID).Scan(&count, &truth); err != nil ||
							(action == vprocore.AuditRestoreRetain && (count != 1 || truth != -1)) ||
							(action == vprocore.AuditRestorePrune && count != 0) {
							t.Fatal("audit prune/Access true=-1 changed", count, truth, err)
						}
						before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
						if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, action); err == nil || got != nil {
							t.Fatal("restoration replayed", got, err)
						}
						vprotest.AssertProfileSUFiles(t, service, before)
					}
					vprotest.AssertProfileSUFiles(t, service, support)
					after, err := os.ReadFile(service.Projects.Preferences.Path)
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
			service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, before)
			if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, before); err != nil {
				t.Fatal(err)
			}
			parent, selection = sftest.SiviProjectAssignmentWriteSelection(t, service, state)
			bytes := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.ChangedCells != 0 || written.HistoryID != "" {
				t.Fatal("same value from physical duplicate created history/audit", written, err)
			}
			vprotest.AssertProfileSUFiles(t, service, bytes)
			if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID='new'`); err != nil {
				t.Fatal(err)
			}
			_, selection = sftest.SiviProjectAssignmentWriteSelection(t, service, state)
			written, err = sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.HistoryID == "" {
				t.Fatal("valid historical correction failed", written, err)
			}
			if _, err := db.Exec(`DELETE FROM Sample_Metadata`); err != nil {
				t.Fatal(err)
			}
			if err := service.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
				t.Fatal(err)
			}
			if _, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err != nil {
				t.Fatal("restoration inferred/required a current metadata definition", err)
			}
			fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
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
		service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 2, strength, nil)
		written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
		if err != nil || written == nil || written.ChangedCells != 1 || (written.HistoryID == "") != (strength < 2) {
			t.Fatal("NULL addition audit strengths differ", strength, written, err)
		}
		if strength < 2 {
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", "1", vprocore.AuditRestorePrune); err == nil || got != nil {
				t.Fatal("unaudited assignment was restored/pruned")
			}
			vprotest.AssertProfileSUFiles(t, service, before)
		}
	}
}

func TestSIVIProjectAssignmentWriterExactUTF16UnavailableAndDrift(t *testing.T) {
	service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	for _, value := range []any{nil, "", strings.Repeat("🌱", 15) + "x", string([]byte{0xff}), []byte{0xff}} {
		if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, value); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		current := selection
		switch value := value.(type) {
		case nil:
			current.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}}
		case string:
			current.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{vprotest.MetadataText(value), {Storage: "null"}}
		case []byte:
			hex := "ff"
			current.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{{Storage: "blob", BlobHex: &hex}, {Storage: "null"}}
		}
		if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, current); err == nil || got != nil {
			t.Fatal("drift/unavailable choice wrote", value, got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectID=?`, strings.Repeat("🌱", 15)); err != nil {
		t.Fatal(err)
	}
	_, selection = sftest.SiviProjectAssignmentWriteSelection(t, service, state)
	for _, test := range []struct{ change, undo string }{
		{`UPDATE Sample_Env SET FieldNumber='drift' WHERE PlotNumber='108050'`, `UPDATE Sample_Env SET FieldNumber=NULL WHERE PlotNumber='108050'`},
		{`UPDATE Sample_Metadata SET ProjectTitle='' WHERE ID=2`, `UPDATE Sample_Metadata SET ProjectTitle=NULL WHERE ID=2`},
		{`ALTER TABLE Sample_Env ADD COLUMN Drift TEXT`, ""},
	} {
		if _, err := db.Exec(test.change); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
			t.Fatal("whole parent/metadata/schema drift wrote", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		if test.undo != "" {
			if _, err := db.Exec(test.undo); err != nil {
				t.Fatal(err)
			}
		}
	}
	parent, selection = sftest.SiviProjectAssignmentWriteSelection(t, service, state)
	if err := service.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
		t.Fatal("changed preference authorized old selection")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if err := service.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": 1}); err != nil {
		t.Fatal(err)
	}
	written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("exact30 UTF16 assignment failed", written, err)
	}
}

func TestSIVIProjectAssignmentWriterTriggersSchemaRollbackAndRetry(t *testing.T) {
	service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
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
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
			t.Fatal("trigger/provenance drift committed", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.undo); err != nil {
			t.Fatal(err)
		}
	}
	hooks := sourceform.SiviProjectAssignmentWriteHooks(service, context.Background(), state.ContextID, "108050", selection)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	err := sourceform.WithSIVIParentWriteTransaction(context.Background(), service.Projects.Sqlite, "schema guard test", func(tx *sql.Tx, suAlias string) error {
		if _, err := hooks.Plan(tx, parent); err != nil {
			return err
		}
		_, err := tx.Exec(`CREATE INDEX AssignmentUnexpectedIndex ON Sample_Metadata(ProjectTitle)`)
		return err
	}, hooks)
	if err == nil {
		t.Fatal("metadata schema changed between plan and commit")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err != nil || got == nil || got.ChangedCells != 1 {
		t.Fatal("rollback poisoned assignment retry", got, err)
	}
}

func TestSIVIProjectAssignmentWriterCancellationCollisionAndPreferenceLease(t *testing.T) {
	service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.WriteSIVIProjectAssignment(service, ctx, state.ContextID, "108050", parent, selection); !errors.Is(err, context.Canceled) || got != nil {
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
	got, writeErr := sourceform.WriteSIVIProjectAssignment(service, ctx, state.ContextID, "108050", parent, selection)
	cancel()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(writeErr, context.DeadlineExceeded) || got != nil {
		t.Fatal("collision ignored cancellation", got, writeErr)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	hooks := sourceform.SiviProjectAssignmentWriteHooks(service, context.Background(), state.ContextID, "108050", selection)
	err = sourceform.WithSIVIParentWriteTransaction(context.Background(), service.Projects.Sqlite, "preference guard test", func(tx *sql.Tx, alias string) error {
		if _, err := hooks.Plan(tx, parent); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()
		failed := make(chan error, 1)
		go func() { failed <- service.Projects.Preferences.CompareAndSetProjectIDSource(ctx, 2, 1) }()
		if err := <-failed; !errors.Is(err, context.DeadlineExceeded) {
			return errors.New("source preference switched during pinned metadata validation")
		}
		master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw")+"&_busy_timeout=50")
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
	vprotest.AssertProfileSUFiles(t, service, before)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
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
	service, state, db, _, _ := sftest.SiviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	if _, err := db.Exec(`CREATE TABLE Assignment_SU(PlotNumber TEXT,SiteUnit TEXT);
			INSERT INTO Assignment_SU VALUES('108050','unit')`); err != nil {
		t.Fatal(err)
	}
	selected := vprotest.ContextSelection(state)
	selected.SU, selected.SUPath = "Assignment", state.ProjectPath
	var err error
	state, err = service.SwitchContext(state.ContextID, selected)
	if err != nil {
		t.Fatal(err)
	}
	parent, selection := sftest.SiviProjectAssignmentWriteSelection(t, service, state)
	owner := service.Projects.Sqlite
	if _, err := db.Exec(`DELETE FROM Assignment_SU`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil || got != nil {
		t.Fatal("assignment wrote outside selected SU")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO Assignment_SU VALUES('108050','unit')`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, owner.Attachments)
	for _, role := range []string{"project", "VMetaData", "VLists", "su"} {
		info := owner.AttachmentInfo[role]
		foreign := owner.AttachmentInfo["VLists"]
		if role == "VLists" {
			foreign = owner.AttachmentInfo["project"]
		}
		owner.AttachmentInfo[role] = foreign
		got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
		owner.AttachmentInfo[role] = info
		if err == nil || got != nil {
			t.Fatal("assignment ignored independently replaced attached file", role)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	for _, user := range []string{"", strings.Repeat("x", 101), string([]byte{0xff})} {
		prior := service.Plots.CurrentUser
		service.Plots.CurrentUser = user
		got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
		service.Plots.CurrentUser = prior
		if err == nil || got != nil {
			t.Fatal("assignment accepted invalid audit user", user)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), "stale", "108050", parent, selection); err == nil || got != nil {
		t.Fatal("assignment accepted stale context")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("ownership/SU correction failed retry", written, err)
	}
}

func TestSIVIProjectAssignmentHistoryRejectsDirectAndActionEvents(t *testing.T) {
	service, state, db, parent, edits := sftest.SiviParentWriteFixture(t, false, 3)
	direct, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, "108050", parent, edits)
	if err != nil {
		t.Fatal(err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentHistory" WHERE ID=?`, direct.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(sourceform.SiviProjectAssignmentHistorySQL+`; INSERT INTO "__VPRO_SIVIProjectAssignmentHistory"(ID,Created,Proposal)
			VALUES(1,'copied direct',?)`, proposal); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", "1", vprocore.AuditRestoreRetain); err == nil || got != nil {
		t.Fatal("assignment namespace restored a copied direct event")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", direct.HistoryID, vprocore.AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050';
			UPDATE Sample_Admin SET PlotType='legacy' WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	parent, err = sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	action, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, "108050", parent, sftest.SiviActionProposalEdits(parent))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentActionHistory" WHERE ID=?`, action.HistoryID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=1`, proposal); err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", "1", vprocore.AuditRestorePrune); err == nil || got != nil {
		t.Fatal("assignment namespace restored a copied action event")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}
func TestSIVIProjectAssignmentRestorationMalformedHistoryAndUnauditedDeletion(t *testing.T) {
	service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, strings.Repeat("x", 31))
	written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil {
		t.Fatal(err)
	}
	event := siviProjectAssignmentStoredEvent(t, db, written.HistoryID)
	original, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for index, mutation := range []func(*sourceform.SiviParentHistory){
		func(e *sourceform.SiviParentHistory) { e.ProjectAssignment = nil },
		func(e *sourceform.SiviParentHistory) { e.ProjectAssignment.Selection.SourceOption = 2 },
		func(e *sourceform.SiviParentHistory) { e.ProjectAssignment.Selection.MetadataColumns = nil },
		func(e *sourceform.SiviParentHistory) {
			e.ProjectAssignment.Selection.MetadataColumns[0].DeclaredType += "_changed"
		},
		func(e *sourceform.SiviParentHistory) {
			e.ProjectAssignment.Selection.MetadataOriginal.Cells[1] = vprotest.MetadataText("")
		},
		func(e *sourceform.SiviParentHistory) { e.ProjectAssignment.Schema = nil },
		func(e *sourceform.SiviParentHistory) {
			value := "malformed schema"
			e.ProjectAssignment.Schema[0].SQL = &value
		},
		func(e *sourceform.SiviParentHistory) { e.ProjectAssignment.Columns[0].DeclaredType = "BLOB" },
		func(e *sourceform.SiviParentHistory) { e.Changes[0].Column = "SV_PolygonNumber" },
		func(e *sourceform.SiviParentHistory) { e.Changes = append(e.Changes, e.Changes[0]) },
		func(e *sourceform.SiviParentHistory) {
			e.Committed.Rows[0].Env.Cells[0] = vprotest.MetadataText("other")
		},
		func(e *sourceform.SiviParentHistory) { value := "forged"; e.Changes[0].Audit.AfterEdit = &value },
	} {
		var candidate sourceform.SiviParentHistory
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
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("malformed assignment provenance restored/pruned", index, got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
	}
	for _, proposal := range []string{string(original) + "{}",
		strings.Replace(string(original), `"ProjectAssignment":`, `"Unknown":true,"ProjectAssignment":`, 1),
		strings.Replace(string(original), `"ProjectID"`, `"\ud800"`, 1)} {
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIProjectAssignmentHistory" SET Proposal=? WHERE ID=?`, proposal, written.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err == nil || got != nil {
			t.Fatal("malformed raw JSON history restored", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
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
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); err == nil || got != nil {
			t.Fatal("restoration overwrote unaudited edits/deleted audit", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		if drift.undo != "" {
			if _, err := db.Exec(drift.undo); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestSIVIProjectAssignmentMasterWALUnavailableAndRollbackJournalRetry(t *testing.T) {
	service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL fixture unavailable", mode, err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil ||
		got != nil || !strings.Contains(err.Error(), "rollback-journal") {
		t.Fatal("read-only WAL snapshot authorized unguarded metadata", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if err := master.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatal("rollback journal recovery failed", mode, err)
	}
	if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err != nil || got == nil {
		t.Fatal("safe metadata retry failed", got, err)
	}
}

func TestSIVIProjectAssignmentRestorationTriggerRollbackCancellationAndRetry(t *testing.T) {
	service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, ctx, state.ContextID, "108050", written.HistoryID, vprocore.AuditRestorePrune); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled restoration wrote", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	for _, test := range []struct{ change, undo string }{
		{`CREATE TRIGGER AssignmentRestoreAudit AFTER UPDATE OF Restore ON Sample_Audit
					BEGIN UPDATE Sample_Env SET FieldNumber='BAD' WHERE PlotNumber='108050'; END`, `DROP TRIGGER AssignmentRestoreAudit`},
		{`CREATE TRIGGER AssignmentRestoreHistory AFTER UPDATE OF Restored ON "__VPRO_SIVIProjectAssignmentHistory"
					BEGIN UPDATE Sample_Admin SET PlotType='BAD' WHERE Plot='108050'; END`, `DROP TRIGGER AssignmentRestoreHistory`},
	} {
		if _, err := db.Exec(test.change); err != nil {
			t.Fatal(err)
		}
		before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err == nil || got != nil {
			t.Fatal("restoration trigger committed partial parents/audit/history", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.undo); err != nil {
			t.Fatal(err)
		}
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), next.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("restoration rollback/reopened owner retry failed", restored, err)
	}
	fresh, err := sourceform.ReadSIVIParent(service, context.Background(), next.ContextID, "108050")
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
			service, state, db, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, true, source, 3, "old")
			path := state.ProjectPath
			table := "Sample_Metadata"
			if source == 2 {
				path, table = service.Projects.SupportPaths["VMetaData"], "ProjectMetadata"
			}
			sourceDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer sourceDB.Close()
			prior := vprocore.QuoteHeaderIdentifier(table + "_prior")
			physical := vprocore.QuoteHeaderIdentifier(table)
			if _, err := sourceDB.Exec(`ALTER TABLE ` + physical + ` RENAME TO ` + prior +
				`; CREATE TABLE ` + physical + `(ProjectID TEXT,ProjectTitle VARCHAR,ID INTEGER);
				INSERT INTO ` + physical + `(rowid,ProjectID,ProjectTitle) SELECT rowid,ProjectID,ProjectTitle FROM ` + prior +
				`; DROP TABLE ` + prior); err != nil {
				t.Fatal(err)
			}
			choices, err := sourceform.ReadSIVIProjectChoices(service, context.Background(), state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			var row vprocore.ProjectMetadataRow
			for _, candidate := range choices.Choices.Rows {
				if candidate.RowID == selection.MetadataOriginal.RowID {
					row = candidate
				}
			}
			if !reflect.DeepEqual(row, selection.MetadataOriginal) ||
				reflect.DeepEqual(choices.Choices.Columns, selection.MetadataColumns) {
				t.Fatal("schema-only drift fixture changed the raw selected ID/title/row", choices)
			}
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if got, err := sourceform.PrepareSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", selection); err == nil || got != nil {
				t.Fatal("preview treated row equality as expected column validation", got, err)
			}
			if got, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection); err == nil ||
				got != nil || !strings.Contains(err.Error(), "column names or declared types changed") {
				t.Fatal("writer authorized stale expected choice-column provenance", got, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			selection.MetadataColumns = append([]vprocore.ProjectMetadataColumn{}, choices.Choices.Columns...)
			written, err := sourceform.WriteSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", parent, selection)
			if err != nil || written == nil || written.ChangedCells != 1 {
				t.Fatal("explicitly refreshed choice schema failed retry", written, err)
			}
			event := siviProjectAssignmentStoredEvent(t, db, written.HistoryID)
			if !reflect.DeepEqual(event.ProjectAssignment.Selection.MetadataColumns, choices.Choices.Columns) {
				t.Fatal("history lost actual expected column names/declared types", event)
			}
			if got, err := sourceform.CtxRestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, "108050", written.HistoryID, vprocore.AuditRestoreRetain); err != nil ||
				got == nil || got.RestoredRows != 1 {
				t.Fatal("expected schema provenance failed typed restoration", got, err)
			}
		})
	}
}
