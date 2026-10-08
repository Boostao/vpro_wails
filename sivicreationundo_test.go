package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func siviCreationUndoFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, siviCreationUndoRequest, *siviCreationResult) {
	t.Helper()
	service, state, db, creation := siviCreationFixture(t, external, strength)
	creation.RequestID = "00000000-0000-4000-8000-000000000001"
	created, err := service.createSIVIVegetation(context.Background(), state.ContextID, creation)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, creation.Plot, creation.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db, siviCreationUndoRequest{RequestID: "00000000-0000-4000-8000-000000000011",
		ContextID: state.ContextID, Project: creation.Project, Plot: creation.Plot, HistoryID: creation.RequestID,
		Action: AuditRestoreRetain, Expected: review.Expected}, created
}

func assertSIVICreationUndoRefused(t *testing.T, service *ContextService, contextID string, request siviCreationUndoRequest, lookup bool) {
	t.Helper()
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.undoSIVICreation(context.Background(), contextID, request); result != nil || err == nil {
		t.Fatal("unsafe Undo accepted", result, err)
	}
	if lookup {
		if result, err := service.lookupSIVICreationUndoReceipt(context.Background(), contextID, request); result != nil || err == nil {
			t.Fatal("unsafe Undo receipt accepted", result, err)
		}
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVICreationUndoRetainPruneStrengthsAliasesReceipt(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, strength := range []int{0, 1, 2, 3} {
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
				t.Run(fmt.Sprintf("%t/%d/%s", external, strength, action), func(t *testing.T) {
					service, state, db, request, created := siviCreationUndoFixture(t, external, strength)
					request.Action = action
					if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"='sAmPlE_vEg' WHERE ID=?`, created.ID); err != nil {
						t.Fatal(err)
					}
					beforeVeg, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					beforeAudit, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Audit", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					var originalHistory string
					if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory" WHERE RequestID=?`, request.HistoryID).Scan(&originalHistory); err != nil {
						t.Fatal(err)
					}
					files := databaseBytes(t, service.projects.sqlite.attachments)
					review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, request.Plot, request.HistoryID)
					if err != nil || review.Expected != request.Expected || !reflect.DeepEqual(review.Committed, created.Committed) || len(review.Columns) != 44 {
						t.Fatal("immutable all44 review differs", review, err)
					}
					assertProfileSUFiles(t, service, files)
					result, err := service.undoSIVICreation(context.Background(), state.ContextID, request)
					if err != nil || result == nil || !result.DidCommit || result.Replayed || result.Cancelled ||
						result.RemovedRows != 1 || result.UndoID != request.RequestID || result.Expected != request.Expected ||
						result.Actor == "" || result.AuditStrength != strength || !reflect.DeepEqual(result.Committed, created.Committed) ||
						!reflect.DeepEqual(result.AuditsBefore, review.AuditsBefore) || result.Action != action {
						t.Fatal("complete Undo receipt differs", result, err)
					}
					if _, err := time.Parse("2006-01-02 15:04:05", result.EditWhen); err != nil {
						t.Fatal(err)
					}
					expectedVeg := ProjectMetadataTable{Columns: beforeVeg.Columns, Rows: []ProjectMetadataRow{}}
					for _, row := range beforeVeg.Rows {
						if row.RowID != created.RowID {
							expectedVeg.Rows = append(expectedVeg.Rows, row)
						}
					}
					actualVeg, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
					if err != nil || !reflect.DeepEqual(actualVeg, expectedVeg) {
						t.Fatal("Undo removed unrelated vegetation", err)
					}
					var creation siviCreationHistory
					if err := json.Unmarshal([]byte(originalHistory), &creation); err != nil {
						t.Fatal(err)
					}
					planned, err := planSIVIDeletionRestorationAudits(beforeAudit, creation.Audits, action)
					actualAudit, readErr := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Audit", "", nil, "")
					if err != nil || readErr != nil || !reflect.DeepEqual(planned, actualAudit) {
						t.Fatal("Undo changed unrelated audits or literal aliases", err, readErr)
					}
					pruned := 0
					if action == AuditRestorePrune {
						pruned = len(creation.Audits)
					}
					if result.PrunedAuditRows != pruned || len(result.AuditsAfter) != len(creation.Audits)-pruned {
						t.Fatal("typed audit counts differ")
					}
					for _, row := range result.AuditsAfter {
						index, _ := siteUnitTransferColumns(actualAudit, "Restore")
						if !reflect.DeepEqual(row.Cells[index["Restore"]], siviCreationInteger("-1")) {
							t.Fatal("retained Access BOOLEAN storage is not -1")
						}
					}
					var preserved string
					if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory" WHERE RequestID=?`, request.HistoryID).Scan(&preserved); err != nil || preserved != originalHistory {
						t.Fatal("immutable creation history changed", err)
					}
					var reservation int
					if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=?`, created.ID).Scan(&reservation); err != nil || reservation != 1 {
						t.Fatal("permanent reservation released", reservation, err)
					}
					expected := *result
					expected.DidCommit, expected.Replayed = false, true
					files = databaseBytes(t, service.projects.sqlite.attachments)
					for i := 0; i < 2; i++ {
						receipt, err := service.lookupSIVICreationUndoReceipt(context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(receipt, &expected) {
							t.Fatal("read-only lost acknowledgement receipt differs", receipt, err)
						}
						retry, err := service.undoSIVICreation(context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(retry, &expected) {
							t.Fatal("retry replay differs", retry, err)
						}
						assertProfileSUFiles(t, service, files)
					}
					if _, err := service.lookupSIVICreationReceipt(context.Background(), state.ContextID, created.Request); err == nil {
						t.Fatal("accepted creation Lookup eligibility was weakened after Undo")
					}
					other := request
					other.RequestID = "00000000-0000-4000-8000-000000000012"
					assertSIVICreationUndoRefused(t, service, state.ContextID, other, false)
					if review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, request.Plot, request.HistoryID); review != nil || err == nil {
						t.Fatal("consumed creation review reopened", review, err)
					}
					assertSIVICreationUndoWire(t, result, []string{"requestId", "contextId", "project", "plot", "historyId", "expected", "undoId",
						"form", "rowId", "id", "action", "actor", "auditStrength", "editWhen", "columns", "committed", "creation", "request",
						"auditColumns", "auditsBefore", "auditsAfter", "cancelled", "removedRows", "prunedAuditRows", "didCommit", "replayed"})
					assertSIVICreationUndoWire(t, review, []string{"contextId", "project", "plot", "historyId", "expected", "form", "rowId", "id",
						"columns", "committed", "creation", "auditColumns", "auditsBefore"})
				})
			}
		}
	}
}

func assertSIVICreationUndoWire(t *testing.T, value any, keys []string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(raw, &properties); err != nil || len(properties) != len(keys) {
		t.Fatal("wire property count differs", string(raw), err)
	}
	for _, key := range keys {
		if properties[key] == nil {
			t.Fatal("missing explicit lowerCamel property", key)
		}
	}
}

func TestSIVICreationUndoStrictTransport(t *testing.T) {
	request := siviCreationUndoRequest{RequestID: "abcdef00-0000-4000-8000-000000000011", ContextID: "owner",
		Project: "Sample", Plot: "108050", HistoryID: "abcdef00-0000-4000-8000-000000000001",
		Action: AuditRestoreRetain, Expected: strings.Repeat("a", 64)}
	raw, _ := json.Marshal(request)
	text := string(raw)
	var decoded siviCreationUndoRequest
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded != request {
		t.Fatal(err)
	}
	assertSIVICreationUndoWire(t, request, []string{"requestId", "contextId", "project", "plot", "historyId", "action", "expected"})
	var properties map[string]json.RawMessage
	_ = json.Unmarshal(raw, &properties)
	for name := range properties {
		copy := map[string]json.RawMessage{}
		for key, value := range properties {
			copy[key] = value
		}
		delete(copy, name)
		missing, _ := json.Marshal(copy)
		copy[name] = json.RawMessage("null")
		null, _ := json.Marshal(copy)
		for _, invalid := range []string{string(missing), string(null),
			strings.Replace(text, `"`+name+`":`, `"`+name+`":`+string(properties[name])+`,"`+name+`":`, 1),
			strings.Replace(text, `"`+name+`":`, `"`+strings.ToUpper(name)+`":`, 1)} {
			if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
				t.Fatal("missing/null/duplicate/recased property accepted", invalid)
			}
		}
	}
	for _, invalid := range []string{
		`[]`, `null`, text + `{}`, text[:len(text)-1] + `,"extra":1}`,
		strings.Replace(text, `"owner"`, `"\ud800"`, 1), strings.Replace(text, `"owner"`, `"\udc00"`, 1),
		strings.Replace(text, `"owner"`, "\"\xff\"", 1), strings.Replace(text, `"owner"`, `""`, 1),
		strings.Replace(text, `"owner"`, `123`, 1),
		strings.Replace(text, request.RequestID, strings.ToUpper(request.RequestID), 1),
		strings.Replace(text, request.RequestID, request.HistoryID, 1),
		strings.Replace(text, request.RequestID, "abcdef00-0000-3000-8000-000000000011", 1),
		strings.Replace(text, request.Expected, strings.ToUpper(request.Expected), 1),
		strings.Replace(text, request.Expected, strings.Repeat("g", 64), 1),
		strings.Replace(text, request.Expected, request.Expected[:63], 1),
		strings.Replace(text, `"retain"`, `"Retain"`, 1), strings.Replace(text, `"retain"`, `" retain "`, 1),
		strings.Replace(text, `"108050"`, `"12345678"`, 1),
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("malformed transport accepted", invalid)
		}
	}
}

func TestSIVICreationUndoCancelUnresolvedOwnershipCancellation(t *testing.T) {
	service, state, _, request, _ := siviCreationUndoFixture(t, false, 3)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVICreationUndoReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
		t.Fatal("missing history is not unresolved", receipt, err)
	}
	request.Action = AuditRestoreCancel
	for i := 0; i < 2; i++ {
		result, err := service.undoSIVICreation(context.Background(), state.ContextID, request)
		if err != nil || result == nil || !result.Cancelled || result.DidCommit || result.Replayed || result.RemovedRows != 0 || result.UndoID != "" {
			t.Fatal("cancel consumed or durably succeeded", result, err)
		}
		if receipt, err := service.lookupSIVICreationUndoReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
			t.Fatal("cancel created receipt", receipt, err)
		}
		assertProfileSUFiles(t, service, files)
	}
	request.Action = AuditRestoreRetain
	for _, mutate := range []func(*siviCreationUndoRequest){
		func(r *siviCreationUndoRequest) { r.ContextID = "stale" },
		func(r *siviCreationUndoRequest) { r.Project = "Other" },
		func(r *siviCreationUndoRequest) { r.Plot = "999999" },
		func(r *siviCreationUndoRequest) { r.HistoryID = "00000000-0000-4000-8000-000000000099" },
		func(r *siviCreationUndoRequest) { r.Expected = strings.Repeat("0", 64) },
		func(r *siviCreationUndoRequest) { r.RequestID = r.HistoryID },
	} {
		invalid := request
		mutate(&invalid)
		assertSIVICreationUndoRefused(t, service, state.ContextID, invalid, false)
	}
	var nilService *ContextService
	if result, err := nilService.undoSIVICreation(context.Background(), state.ContextID, request); result != nil || err == nil {
		t.Fatal("nil service accepted")
	}
	if result, err := service.undoSIVICreation(nil, state.ContextID, request); result != nil || err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.undoSIVICreation(ctx, state.ContextID, request); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled writer acknowledged", result, err)
	}
	if result, err := service.lookupSIVICreationUndoReceipt(ctx, state.ContextID, request); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled lookup acknowledged", result, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Millisecond)
	result, err := service.lookupSIVICreationUndoReceipt(ctx, state.ContextID, request)
	cancel()
	owner.mu.Unlock()
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("owned snapshot lease ignored cancellation", result, err)
	}
	assertProfileSUFiles(t, service, files)
	if result, err := service.undoSIVICreation(context.Background(), state.ContextID, request); result == nil || err != nil {
		t.Fatal("read-only cancel consumed creation", result, err)
	}
}

func TestSIVICreationUndoEveryRawCellDrift(t *testing.T) {
	service, state, db, request, created := siviCreationUndoFixture(t, false, 3)
	for i, column := range created.Columns {
		t.Run(column.Name, func(t *testing.T) {
			value := any("Changed Literal")
			if column.Name == "Flag" {
				value = int64(-1)
			}
			if _, err := db.Exec(`UPDATE Sample_Veg SET `+quoteHeaderIdentifier(column.Name)+`=? WHERE rowid=?`, value, created.RowID); err != nil {
				t.Fatal(err)
			}
			assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
			original, err := metadataCellValue(created.Committed.Cells[i])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Veg SET `+quoteHeaderIdentifier(column.Name)+`=? WHERE rowid=?`, original, created.RowID); err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("corrected current raw cells could not retry", err)
	}
}

func TestSIVICreationUndoDriftAuditSchemaIdentity(t *testing.T) {
	for _, mutation := range []string{
		`DELETE FROM Sample_Veg WHERE rowid=%s`,
		`UPDATE Sample_Veg SET rowid=-10 WHERE rowid=%s`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID) SELECT 'outside','A',ID FROM Sample_Veg WHERE rowid=%s`,
		`ALTER TABLE Sample_Veg ADD COLUMN Changed TEXT`,
		`ALTER TABLE Sample_Audit ADD COLUMN Changed TEXT`,
		`DELETE FROM "__VPRO_ChildIdentity"`,
		`UPDATE "__VPRO_ChildIdentity" SET ChildTable='"Other_Veg"'`,
		`DELETE FROM Sample_Audit WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET BeforeEdit='changed' WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET Restore=1 WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET Flag=-1 WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET "Table"='_Veg ' WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET "Table"='Other_Veg' WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET Project='sample' WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Audit SET AfterEdit='changed' WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=%s)`,
		`UPDATE Sample_Admin SET Plot='outside' WHERE Plot='108050'`,
		`UPDATE Sample_Env SET PlotNumber='outside' WHERE PlotNumber='108050'`,
		`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=Proposal||' '`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state, db, request, created := siviCreationUndoFixture(t, false, 3)
			if _, err := db.Exec(strings.ReplaceAll(mutation, "%s", created.RowID)); err != nil {
				t.Fatal(err)
			}
			assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
		})
	}
}

func TestSIVICreationUndoRollbackTriggersAndRetry(t *testing.T) {
	service, state, db, request, _ := siviCreationUndoFixture(t, true, 3)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_undo BEFORE DELETE ON Sample_Veg BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER fail_undo AFTER DELETE ON Sample_Veg BEGIN DELETE FROM Sample_Admin; END`,
		`CREATE TRIGGER fail_undo AFTER DELETE ON Sample_Veg BEGIN DELETE FROM "__VPRO_ChildIdentity"; END`,
		`CREATE TRIGGER fail_undo AFTER DELETE ON Sample_Veg BEGIN UPDATE "__VPRO_SIVICreationHistory" SET Proposal=Proposal||' '; END`,
		`CREATE TRIGGER fail_undo BEFORE UPDATE OF Restore ON Sample_Audit BEGIN SELECT RAISE(ABORT,'retention failure'); END`,
		`CREATE TRIGGER fail_undo AFTER UPDATE OF Restore ON Sample_Audit BEGIN UPDATE Sample_Audit SET AfterEdit='changed' WHERE rowid=NEW.rowid; END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
		if _, err := db.Exec(`DROP TRIGGER fail_undo`); err != nil {
			t.Fatal(err)
		}
	}
	request.Action = AuditRestorePrune
	if _, err := db.Exec(`CREATE TRIGGER fail_undo AFTER DELETE ON Sample_Audit BEGIN DELETE FROM "__VPRO_ChildIdentity"; END`); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER fail_undo`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.undoSIVICreation(context.Background(), state.ContextID, request); result == nil || err != nil {
		t.Fatal("rolled-back Undo could not retry", result, err)
	}
}

func TestSIVICreationUndoOneWinnerConcurrentRetry(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(fmt.Sprint(same), func(t *testing.T) {
			service, state, db, request, _ := siviCreationUndoFixture(t, true, 0)
			var wg sync.WaitGroup
			results := make(chan *siviCreationUndoResult, 2)
			errs := make(chan error, 2)
			for i := 0; i < 2; i++ {
				candidate := request
				if !same && i == 1 {
					candidate.RequestID = "00000000-0000-4000-8000-000000000012"
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					result, err := service.undoSIVICreation(context.Background(), state.ContextID, candidate)
					results <- result
					errs <- err
				}()
			}
			wg.Wait()
			close(results)
			close(errs)
			commits, replayed, failures := 0, 0, 0
			for result := range results {
				if result != nil && result.DidCommit {
					commits++
				}
				if result != nil && result.Replayed {
					replayed++
				}
			}
			for err := range errs {
				if err != nil {
					failures++
				}
			}
			if commits != 1 || same && (replayed != 1 || failures != 0) || !same && (replayed != 0 || failures != 1) {
				t.Fatal("one-winner consumption differs", commits, replayed, failures)
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_SIVICreationUndoHistory"`).Scan(&count); err != nil || count != 1 {
				t.Fatal("zero-audit consumption not durable", count, err)
			}
		})
	}
}

func TestSIVICreationUndoReplayTamperPrunedRowIDReuseAndReopen(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		for _, mutation := range []string{
			`INSERT INTO Sample_Veg(rowid,PlotNumber,Species,ID) VALUES(%s,'outside','A',-99)`,
			`INSERT INTO Sample_Veg(PlotNumber,Species,ID) VALUES('outside','A',%d)`,
			`DELETE FROM "__VPRO_ChildIdentity"`,
			`UPDATE "__VPRO_SIVICreationUndoHistory" SET Proposal=Proposal||' '`,
			`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=Proposal||' '`,
		} {
			t.Run(string(action)+mutation, func(t *testing.T) {
				service, state, db, request, created := siviCreationUndoFixture(t, false, 3)
				request.Action = action
				if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
					t.Fatal(err)
				}
				query := strings.ReplaceAll(mutation, "%s", created.RowID)
				query = strings.ReplaceAll(query, "%d", fmt.Sprint(created.ID))
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
				assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
			})
		}
		service, state, db, request, _ := siviCreationUndoFixture(t, false, 3)
		request.Action = action
		result, err := service.undoSIVICreation(context.Background(), state.ContextID, request)
		if err != nil {
			t.Fatal(err)
		}
		if action == AuditRestoreRetain {
			if _, err := db.Exec(`UPDATE Sample_Audit SET Restore=1 WHERE rowid=?`, result.AuditsBefore[0].RowID); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := db.Exec(`INSERT INTO Sample_Audit(rowid,Project,"Table",ID) VALUES(?,'Other','_Env',99)`, result.AuditsBefore[0].RowID); err != nil {
				t.Fatal(err)
			}
		}
		assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
	}
	service, state, _, request, _ := siviCreationUndoFixture(t, true, 0)
	if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	reopened, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVICreationUndoReceipt(context.Background(), reopened.ContextID, request); receipt == nil || err != nil || receipt.ContextID != reopened.ContextID || !receipt.Replayed {
		t.Fatal("new current owner lost immutable request receipt", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
	request.Expected = strings.Repeat("0", 64)
	assertSIVICreationUndoRefused(t, service, reopened.ContextID, request, true)
}

func TestSIVICreationUndoMembershipsAndHistoricalNullAuthority(t *testing.T) {
	for _, form := range []string{"SubVegA-SIVI", "SubVegA-SIVI_BC", "SubVegC-SIVI", "SubVegD-SIVI"} {
		for _, orphan := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", form, orphan), func(t *testing.T) {
				service, state, db, creation := siviCreationFixture(t, false, 3)
				creation.RequestID, creation.Form = "00000000-0000-4000-8000-000000000001", form
				switch form {
				case "SubVegA-SIVI":
					creation.Covers = []siviCreationCover{{"Cover5a", siviReal(0)}}
				case "SubVegC-SIVI":
					creation.Species, creation.Covers = "C", []siviCreationCover{{"Cover6", siviReal(0)}}
				case "SubVegD-SIVI":
					creation.Species, creation.Covers = "D", []siviCreationCover{{"Cover9", siviReal(0)}}
				}
				created, err := service.createSIVIVegetation(context.Background(), state.ContextID, creation)
				if err != nil {
					t.Fatal(err)
				}
				if orphan {
					var raw string
					if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory"`).Scan(&raw); err != nil {
						t.Fatal(err)
					}
					var history siviCreationHistory
					if err := json.Unmarshal([]byte(raw), &history); err != nil {
						t.Fatal(err)
					}
					columns, _ := siteUnitTransferColumns(ProjectMetadataTable{Columns: history.Columns}, siviCreationColumns...)
					for _, cover := range history.Request.Covers {
						history.Committed.Cells[columns[cover.Column]] = ProjectMetadataCell{Storage: "null"}
						if _, err := db.Exec(`UPDATE Sample_Veg SET `+quoteHeaderIdentifier(cover.Column)+`=NULL WHERE rowid=?`, created.RowID); err != nil {
							t.Fatal(err)
						}
					}
					if review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, creation.Plot, creation.RequestID); review != nil || err == nil {
						t.Fatal("cleared current covers became creation authority", review, err)
					}
					// Install a coherent historical empty-cover event, not a writer
					// feature or a mutation of evidence by the Undo kernel.
					history.Request.Covers = []siviCreationCover{}
					retained := []AuditEntry{}
					for _, audit := range history.Audits {
						if audit.EditField == "Species" {
							retained = append(retained, audit)
						} else if _, err := db.Exec(`DELETE FROM Sample_Audit WHERE rowid=?`, audit.RowID); err != nil {
							t.Fatal(err)
						}
					}
					history.Audits = retained
					updated, _ := json.Marshal(history)
					if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, string(updated)); err != nil {
						t.Fatal(err)
					}
					projected, err := projectSIVIVegetation(context.Background(), creation.Plot, true,
						ProjectMetadataTable{Columns: history.Columns, Rows: []ProjectMetadataRow{history.Committed}})
					if err != nil {
						t.Fatal(err)
					}
					for _, group := range projected {
						if len(group.Rows) != 0 {
							t.Fatal("historical NULL creation incorrectly fabricated membership")
						}
					}
				}
				review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, creation.Plot, creation.RequestID)
				if err != nil {
					t.Fatal(err)
				}
				request := siviCreationUndoRequest{RequestID: "00000000-0000-4000-8000-000000000011", ContextID: state.ContextID,
					Project: creation.Project, Plot: creation.Plot, HistoryID: creation.RequestID, Expected: review.Expected, Action: AuditRestoreRetain}
				if result, err := service.undoSIVICreation(context.Background(), state.ContextID, request); result == nil || err != nil || result.RemovedRows != 1 {
					t.Fatal("complete source/historical orphan authority failed", result, err)
				}
			})
		}
	}
}

func TestSIVICreationUndoSHAValidCoherentEvidenceChange(t *testing.T) {
	service, state, db, request, _ := siviCreationUndoFixture(t, false, 0)
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory"`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	siviCreationUndoAssertCoherentChange(t, service, state, db, request, raw)
}

func TestSIVICreationUndoSourceAuditDuplicateOccupancy(t *testing.T) {
	service, state, db, request, _ := siviCreationUndoFixture(t, false, 3)
	if _, err := db.Exec(`INSERT INTO Sample_Audit SELECT * FROM Sample_Audit WHERE ID=(SELECT ID FROM Sample_Veg WHERE rowid=(SELECT MAX(rowid) FROM Sample_Veg))`); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
}

func TestSIVICreationUndoCanonicalHistoryCorruption(t *testing.T) {
	for _, mutate := range []func(*siviCreationHistory){
		func(h *siviCreationHistory) { h.Result.RequestID = "different" },
		func(h *siviCreationHistory) { h.Result.HistoryID = "different" },
		func(h *siviCreationHistory) { h.Result.ID = 0 },
		func(h *siviCreationHistory) { h.Result.ID = 2147483648 },
		func(h *siviCreationHistory) { h.Request.Plot = "outside" },
		func(h *siviCreationHistory) { h.Request.Covers[0].Value = siviReal(1) },
		func(h *siviCreationHistory) { h.Columns[0].Name = "NotPlotNumber" },
		func(h *siviCreationHistory) { h.Columns = h.Columns[:43] },
		func(h *siviCreationHistory) { h.Original.Cells[0] = siviCreationText("tampered") },
		func(h *siviCreationHistory) { h.Committed.Cells[0] = siviCreationText("tampered") },
		func(h *siviCreationHistory) { h.Actor = "" },
		func(h *siviCreationHistory) { h.When = "2026-10-07T21:00:00Z" },
		func(h *siviCreationHistory) { h.AuditStrength = 4 },
		func(h *siviCreationHistory) { h.Audits = nil },
		func(h *siviCreationHistory) { h.Audits = h.Audits[:1] },
		func(h *siviCreationHistory) { h.Audits = append(h.Audits, h.Audits[0]) },
		func(h *siviCreationHistory) { h.Audits[0].Table = "Sample_Veg" },
		func(h *siviCreationHistory) { h.Audits[0].Restore = true },
		func(h *siviCreationHistory) { h.Audits[0].BeforeEdit = h.Audits[0].AfterEdit },
		func(h *siviCreationHistory) { h.Audits[0].RowID = "01" },
		func(h *siviCreationHistory) { h.Audits[0].ID = nil },
	} {
		t.Run(fmt.Sprintf("%p", mutate), func(t *testing.T) {
			service, state, db, request, _ := siviCreationUndoFixture(t, false, 3)
			var raw string
			if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory"`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var history siviCreationHistory
			if err := json.Unmarshal([]byte(raw), &history); err != nil {
				t.Fatal(err)
			}
			mutate(&history)
			corrupt, _ := json.Marshal(history)
			if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, string(corrupt)); err != nil {
				t.Fatal(err)
			}
			if review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, request.Plot, request.HistoryID); review != nil || err == nil {
				t.Fatal("corrupt immutable history review accepted", review, err)
			}
			assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
		})
	}
}

func TestSIVICreationUndoTechnicalHistorySchemaAndConsumption(t *testing.T) {
	for _, mutation := range []string{
		`CREATE INDEX extra_history ON "__VPRO_SIVICreationHistory"(Created)`,
		`ALTER TABLE "__VPRO_SIVICreationHistory" ADD COLUMN Extra TEXT`,
		`CREATE TRIGGER extra_history AFTER UPDATE ON "__VPRO_SIVICreationHistory" BEGIN SELECT 1; END`,
	} {
		service, state, db, request, _ := siviCreationUndoFixture(t, false, 0)
		if _, err := db.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
	}
	for _, mutation := range []string{
		`CREATE INDEX extra_undo ON "__VPRO_SIVICreationUndoHistory"(Created)`,
		`CREATE TRIGGER extra_undo AFTER INSERT ON "__VPRO_SIVICreationUndoHistory" BEGIN SELECT 1; END`,
		`UPDATE "__VPRO_SIVICreationUndoHistory" SET CreationID='00000000-0000-4000-8000-000000000099'`,
		`UPDATE "__VPRO_SIVICreationUndoHistory" SET Created='different'`,
	} {
		service, state, db, request, _ := siviCreationUndoFixture(t, false, 0)
		if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(mutation); err != nil {
			t.Fatal(err)
		}
		assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
	}
	service, state, db, request, _ := siviCreationUndoFixture(t, false, 0)
	// A valid alternate UUID cannot create a second event for a reserved logical ID.
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory"`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var creation siviCreationHistory
	_ = json.Unmarshal([]byte(raw), &creation)
	creation.Request.RequestID = "00000000-0000-4000-8000-000000000002"
	creation.Result.RequestID, creation.Result.HistoryID = creation.Request.RequestID, creation.Request.RequestID
	duplicate, _ := json.Marshal(creation)
	if _, err := db.Exec(`INSERT INTO "__VPRO_SIVICreationHistory" VALUES(?,?,?)`, creation.Request.RequestID, creation.When, string(duplicate)); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationUndoRefused(t, service, state.ContextID, request, true)
}

func TestSIVICreationUndoNewAllocationKeepsReservationAndNonUUIDHistory(t *testing.T) {
	service, state, db, request, created := siviCreationUndoFixture(t, false, 0)
	request.Action = AuditRestorePrune
	if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	next := created.Request
	next.RequestID = "accepted-preexisting-non-uuid"
	result, err := service.createSIVIVegetation(context.Background(), state.ContextID, next)
	if err != nil || result == nil || result.ID <= created.ID {
		t.Fatal("new creation reused permanent ID", result, err)
	}
	// Physical rowIDs may be reused: receipt lookup must refuse without replay.
	if _, err := db.Exec(`UPDATE Sample_Veg SET rowid=-1234567 WHERE rowid=?`, result.RowID); err != nil {
		t.Fatal(err)
	}
	// The unrelated event must remain internally coherent to be valid authority.
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory" WHERE RequestID=?`, next.RequestID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var history siviCreationHistory
	_ = json.Unmarshal([]byte(raw), &history)
	history.Result.RowID, history.Committed.RowID = "-1234567", "-1234567"
	changed, _ := json.Marshal(history)
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=? WHERE RequestID=?`, string(changed), next.RequestID); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVICreationUndoReceipt(context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal("valid unrelated accepted non-UUID history blocked Undo facts", receipt, err)
	}
	assertProfileSUFiles(t, service, files)
}
func siviCreationUndoAssertCoherentChange(t *testing.T, service *ContextService, state ProjectState, db *sql.DB, request siviCreationUndoRequest, raw string) {
	t.Helper()
	var history siviCreationHistory
	if err := json.Unmarshal([]byte(raw), &history); err != nil {
		t.Fatal(err)
	}
	history.Actor = "Coherently Changed Actor"
	changed, _ := json.Marshal(history)
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, string(changed)); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
	review, err := service.reviewSIVICreationUndo(context.Background(), state.ContextID, request.Plot, request.HistoryID)
	if err != nil || review.Expected == request.Expected {
		t.Fatal("SHA failed to bind valid changed evidence", review, err)
	}
	request.Expected = review.Expected
	history.Actor = "Second Coherent Actor"
	drift, _ := json.Marshal(history)
	trigger := `CREATE TRIGGER drift_evidence AFTER DELETE ON Sample_Veg BEGIN UPDATE "__VPRO_SIVICreationHistory" SET Proposal='` +
		strings.ReplaceAll(string(drift), "'", "''") + `'; END`
	if _, err := db.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationUndoRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER drift_evidence`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.undoSIVICreation(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("SHA drift rollback prevented valid retry", err)
	}
}
