package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func assertSIVIDeletionRefused(t *testing.T, service *vprocore.ContextService, contextID string, request sourceform.SiviDeletionRequest, lookup bool) {
	t.Helper()
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), contextID, request); receipt != nil || err == nil {
		t.Fatal("invalid/conflicting deletion mutated or replayed", receipt, err)
	}
	if lookup {
		if receipt, err := sourceform.LookupSIVIDeletionReceipt(service, context.Background(), contextID, request); receipt != nil || err == nil {
			t.Fatal("invalid/conflicting receipt was resolved", receipt, err)
		}
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIDeletionAllFormsReceiptAuditsAndReadOnlyReplay(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, strength := range []int{0, 1, 2, 3} {
			for _, form := range []string{"SubVegA-SIVI", "SubVegA-SIVI_BC", "SubVegC-SIVI", "SubVegD-SIVI"} {
				t.Run(fmt.Sprintf("%t/%d/%s", external, strength, form), func(t *testing.T) {
					service, state, db, request := sftest.SiviDeletionFixture(t, external, strength)
					if _, err := db.Exec(`UPDATE Sample_Veg SET Cover5a=0,Cover7=0,Flag=1,Other2=' old overlength Historical literal ',Collected='bad'
						WHERE rowid=?`, request.Original.RowID); err != nil {
						t.Fatal(err)
					}
					original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, request.Plot, form, request.Original.RowID)
					if err != nil {
						t.Fatal(err)
					}
					request.Form, request.Columns, request.Original = form, original.Columns, original.Original
					before, err := vprocore.ReadSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
					if err != nil {
						t.Fatal(err)
					}
					result, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request)
					if err != nil || result == nil || !result.DidCommit || result.Replayed {
						t.Fatal("bounded deletion failed", result, err)
					}
					if result.RequestID != request.RequestID || result.HistoryID != request.RequestID ||
						result.ContextID != state.ContextID || result.Project != request.Project || result.Plot != request.Plot ||
						result.Form != request.Form || result.RowID != request.Original.RowID || result.ID != 10000001 ||
						!reflect.DeepEqual(result.Columns, request.Columns) || !reflect.DeepEqual(result.Original, request.Original) ||
						!reflect.DeepEqual(result.Request, request) || result.Actor == "" || result.AuditStrength != strength ||
						len(result.Columns) != 44 || len(result.Original.Cells) != 44 {
						t.Fatal("receipt lost full typed row, ownership, or provenance", result)
					}
					if _, err := time.Parse("2006-01-02 15:04:05", result.EditWhen); err != nil {
						t.Fatal(err)
					}
					raw, err := json.Marshal(result)
					if err != nil {
						t.Fatal(err)
					}
					var properties map[string]json.RawMessage
					if err := json.Unmarshal(raw, &properties); err != nil {
						t.Fatal(err)
					}
					keys := []string{"requestId", "contextId", "project", "plot", "form", "rowId", "id", "historyId", "actor",
						"auditStrength", "editWhen", "columns", "original", "request", "audits", "didCommit", "replayed"}
					if len(properties) != len(keys) {
						t.Fatal("receipt shape differs or invents a committed NULL row", string(raw))
					}
					for _, key := range keys {
						if properties[key] == nil {
							t.Fatal("missing lowerCamel receipt field", key)
						}
					}
					expectedAudits := 0
					for i, column := range request.Columns {
						if column.Name != "ID" && column.Name != "PlotNumber" && request.Original.Cells[i].Storage != "null" && strength == 3 {
							expectedAudits++
						}
					}
					if len(result.Audits) != expectedAudits {
						t.Fatal("deletion audit strength/completeness differs", len(result.Audits), expectedAudits)
					}
					for _, audit := range result.Audits {
						if audit.EditField == "ID" || audit.EditField == "PlotNumber" || audit.AfterEdit != nil {
							t.Fatal("identity/parent audit or fake replacement", audit)
						}
						if audit.EditField == "Flag" && (audit.BeforeEdit == nil || *audit.BeforeEdit != "-1") {
							t.Fatal("Access true=-1 was not explicitly normalized for audit", audit)
						}
						if audit.EditField == "Other2" && (audit.BeforeEdit == nil || *audit.BeforeEdit != " old overlength Historical literal ") {
							t.Fatal("historical invalid literal was trimmed/repaired", audit)
						}
					}
					after, err := vprocore.ReadSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
					if err != nil || len(after.Rows) != len(before.Rows)-1 {
						t.Fatal("not exactly one physical deletion", err)
					}
					for _, row := range before.Rows {
						if row.RowID == request.Original.RowID {
							continue
						}
						found := false
						for _, remaining := range after.Rows {
							found = found || reflect.DeepEqual(row, remaining)
						}
						if !found {
							t.Fatal("unreviewed vegetation changed", row)
						}
					}
					var reserved int
					if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=10000001`).Scan(&reserved); err != nil || reserved != 1 {
						t.Fatal("deletion lost permanent identity reservation", reserved, err)
					}
					bytes := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
					expected := *result
					expected.DidCommit, expected.Replayed = false, true
					for i := 0; i < 2; i++ {
						replay, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(replay, &expected) {
							t.Fatal("matching request replay differs", replay, err)
						}
						lookup, err := sourceform.LookupSIVIDeletionReceipt(service, context.Background(), state.ContextID, request)
						if err != nil || !reflect.DeepEqual(lookup, &expected) {
							t.Fatal("read-only durable receipt differs", lookup, err)
						}
						vprotest.AssertProfileSUFiles(t, service, bytes)
					}
					conflict := request
					conflict.Form = "SubVegA-SIVI"
					if conflict.Form == request.Form {
						conflict.Form = "SubVegA-SIVI_BC"
					}
					assertSIVIDeletionRefused(t, service, state.ContextID, conflict, true)
				})
			}
		}
	}
}

func TestSIVIDeletionFull44AndSchemaRejectWithoutWrites(t *testing.T) {
	service, state, _, request := sftest.SiviDeletionFixture(t, false, 3)
	for i, column := range request.Columns {
		t.Run(column.Name, func(t *testing.T) {
			draft := request
			draft.Original.Cells = append([]vprocore.ProjectMetadataCell{}, request.Original.Cells...)
			if draft.Original.Cells[i].Storage == "null" {
				draft.Original.Cells[i] = vprotest.MetadataText("changed")
			} else {
				draft.Original.Cells[i] = vprocore.ProjectMetadataCell{Storage: "null"}
			}
			assertSIVIDeletionRefused(t, service, state.ContextID, draft, false)
			draft = request
			draft.Columns = append([]vprocore.ProjectMetadataColumn{}, request.Columns...)
			draft.Columns[i].DeclaredType = "CHANGED"
			assertSIVIDeletionRefused(t, service, state.ContextID, draft, false)
		})
	}
	for _, mutate := range []func(*sourceform.SiviDeletionRequest){
		func(r *sourceform.SiviDeletionRequest) { r.ContextID = "old-owner" },
		func(r *sourceform.SiviDeletionRequest) { r.Project = "Other" },
		func(r *sourceform.SiviDeletionRequest) { r.Plot = "999999" },
		func(r *sourceform.SiviDeletionRequest) { r.Form = "SubVegA-XL" },
		func(r *sourceform.SiviDeletionRequest) { r.Original.RowID = "-9223372036854775808" },
		func(r *sourceform.SiviDeletionRequest) { r.Columns = r.Columns[:43] },
		func(r *sourceform.SiviDeletionRequest) { r.Original.Cells = r.Original.Cells[:43] },
	} {
		draft := request
		mutate(&draft)
		assertSIVIDeletionRefused(t, service, state.ContextID, draft, false)
	}
}

func TestSIVIDeletionFreshStorageParentAndLogicalIDRejections(t *testing.T) {
	for _, change := range []string{
		`UPDATE Sample_Veg SET HeightA=999 WHERE ID=10000001`,
		`UPDATE Sample_Veg SET Flag=-1 WHERE ID=10000001`,
		`UPDATE Sample_Veg SET PlotNumber='outside' WHERE ID=10000001`,
		`UPDATE Sample_Veg SET Cover1=NULL,Cover6=NULL WHERE ID=10000001`,
		`UPDATE Sample_Veg SET Other1=X'00' WHERE ID=10000001`,
		`UPDATE Sample_Veg SET ID=NULL WHERE ID=10000001`,
		`UPDATE Sample_Veg SET ID=2147483648 WHERE ID=10000001`,
		`UPDATE Sample_Veg SET ID='bad' WHERE ID=10000001`,
		`UPDATE Sample_Veg SET ID=1.5 WHERE ID=10000001`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','DUP',10000001,0)`,
		`UPDATE Sample_Admin SET Plot='outside' WHERE Plot='108050'`,
		`DELETE FROM Sample_Admin WHERE Plot='108050'`,
		`UPDATE Sample_Env SET PlotNumber='outside' WHERE PlotNumber='108050'`,
		`ALTER TABLE Sample_Veg ADD COLUMN Unexpected TEXT`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
			if original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, request.Plot, request.Form, request.Original.RowID); err == nil &&
				reflect.DeepEqual(original.Original, request.Original) {
				t.Fatal("read returned stale unchanged original", original)
			}
		})
	}
}

func TestSIVIDeletionRollbackCancellationAndRetry(t *testing.T) {
	service, state, db, request := sftest.SiviDeletionFixture(t, true, 3)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_deletion AFTER DELETE ON Sample_Veg BEGIN UPDATE Sample_Admin SET Plot='bad'; END`,
		`CREATE TRIGGER fail_deletion AFTER DELETE ON Sample_Veg BEGIN UPDATE Sample_Veg SET Species='BAD'; END`,
		`CREATE TRIGGER fail_deletion AFTER INSERT ON Sample_Audit BEGIN UPDATE Sample_Audit SET BeforeEdit='WRONG' WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_deletion BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
		`CREATE TRIGGER fail_deletion BEFORE DELETE ON Sample_Veg BEGIN SELECT RAISE(IGNORE); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
		if _, err := db.Exec(`DROP TRIGGER fail_deletion`); err != nil {
			t.Fatal(err)
		}
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := sourceform.DeleteSIVIVegetation(service, ctx, state.ContextID, request); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request succeeded", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	result, err := sourceform.DeleteSIVIVegetation(service, ctx, state.ContextID, request)
	cancel()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("writer wait did not honor cancellation", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	ctx, cancel = context.WithCancel(context.Background())
	staged := &vprotest.SiviCreationJournalCancellation{Context: ctx, Journal: service.Projects.Sqlite.Attachments["project"] + "-journal", Cancel: cancel}
	result, err = sourceform.DeleteSIVIVegetation(service, staged, state.ContextID, request)
	cancel()
	if result != nil || !errors.Is(err, context.Canceled) || !staged.Staged {
		t.Fatal("staged mutation cancellation not exercised", result, err, staged.Staged)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if result, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request); result == nil || err != nil {
		t.Fatal("rollback blocked exact retry", result, err)
	}
}

func TestSIVIDeletionSignedLogicalIDsAndBooleanRawEvidence(t *testing.T) {
	for _, id := range []int64{-2147483648, -1, 0, 2147483647} {
		service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
		if _, err := db.Exec(`UPDATE Sample_Veg SET ID=?,Flag=7 WHERE rowid=?`, id, request.Original.RowID); err != nil {
			t.Fatal(err)
		}
		original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, request.Plot, request.Form, request.Original.RowID)
		if err != nil {
			t.Fatal(err)
		}
		request.Original = original.Original
		receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request)
		if err != nil || receipt == nil || receipt.ID != id {
			t.Fatal("signed32 existing identity lost", receipt, err)
		}
		for i, column := range receipt.Columns {
			if column.Name == "Flag" && !reflect.DeepEqual(receipt.Original.Cells[i], vprotest.MetadataInteger("7")) {
				t.Fatal("BOOL audit normalization changed raw typed history")
			}
		}
		var reserved int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=?`, id).Scan(&reserved); err != nil || reserved != 1 {
			t.Fatal("signed ID not reserved", reserved, err)
		}
	}
	for _, declared := range []string{"BOOL", "BOOLEAN", "BIT"} {
		value, err := sourceform.SiviDeletionAuditValue(vprocore.ProjectMetadataColumn{Name: "Other1", DeclaredType: declared}, vprotest.MetadataInteger("1"))
		if err != nil || value != int64(-1) {
			t.Fatal("scalar declared BOOL not normalized", value, err)
		}
	}
}

func TestSIVIDeletionSharedProjectsAndInternalForeignCorruption(t *testing.T) {
	service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
	for _, suffix := range vprocore.CoreTables {
		if _, err := db.Exec(`CREATE TABLE ` + vprocore.QuoteHeaderIdentifier("Second_"+suffix) + ` AS SELECT * FROM ` + vprocore.QuoteHeaderIdentifier("Sample_"+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO _table_metadata(table_name,description) VALUES('Second_Env','VP08')`); err != nil {
		t.Fatal(err)
	}
	first, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	selection := vprotest.ContextSelection(state)
	selection.Project = "Second"
	secondState, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), secondState.ContextID, request.Plot, request.Form, request.Original.RowID)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := request
	secondRequest.RequestID, secondRequest.ContextID, secondRequest.Project = "00000000-0000-4000-8000-000000000002", secondState.ContextID, "Second"
	secondRequest.Columns, secondRequest.Original = original.Columns, original.Original
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if receipt, err := sourceform.LookupSIVIDeletionReceipt(service, context.Background(), secondState.ContextID, secondRequest); receipt != nil || err != nil {
		t.Fatal("foreign valid history blocked unresolved lookup", receipt, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	second, err := sourceform.DeleteSIVIVegetation(service, context.Background(), secondState.ContextID, secondRequest)
	if err != nil || second == nil || second.ID != first.ID {
		t.Fatal("shared-library distinct project deletion failed", second, err)
	}
	cross := secondRequest
	cross.RequestID = request.RequestID
	assertSIVIDeletionRefused(t, service, secondState.ContextID, cross, true)
	reopened, err := service.SwitchContext(secondState.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if receipt, err := sourceform.LookupSIVIDeletionReceipt(service, context.Background(), reopened.ContextID, request); receipt == nil || err != nil {
		t.Fatal("second project's history blocked first receipt", receipt, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	cross = request
	cross.RequestID = secondRequest.RequestID
	assertSIVIDeletionRefused(t, service, reopened.ContextID, cross, true)
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, secondRequest.RequestID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var history sourceform.SiviDeletionHistory
	if err := json.Unmarshal([]byte(proposal), &history); err != nil {
		t.Fatal(err)
	}
	history.Result.Project = "Sample"
	broken, err := json.Marshal(history)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=? WHERE RequestID=?`, string(broken), secondRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRefused(t, service, reopened.ContextID, request, true)
}

func TestSIVIDeletionReceiptCorruptionRefusedWithoutRepair(t *testing.T) {
	for _, change := range []string{
		`DELETE FROM "__VPRO_ChildIdentity"`,
		`DELETE FROM Sample_Audit WHERE ID=10000001`,
		`UPDATE Sample_Audit SET BeforeEdit='changed' WHERE ID=10000001`,
		`UPDATE "__VPRO_SIVIDeletionHistory" SET Created='invalid'`,
		`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=Proposal||' '`,
		`UPDATE "__VPRO_SIVIDeletionHistory" SET RequestID='00000000-0000-4000-8000-000000000099'`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','returned',10000001,0)`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
			if receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request); receipt == nil || err != nil {
				t.Fatal(receipt, err)
			}
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			assertSIVIDeletionRefused(t, service, state.ContextID, request, true)
		})
	}
}

func TestSIVIDeletionSourcePhysicalOrderIsNotImplicitCanonicalOrder(t *testing.T) {
	_, _, _, request := sftest.SiviDeletionFixture(t, false, 3)
	request.Columns = append([]vprocore.ProjectMetadataColumn{}, request.Columns...)
	request.Original.Cells = append([]vprocore.ProjectMetadataCell{}, request.Original.Cells...)
	for i, j := 0, len(request.Columns)-1; i < j; i, j = i+1, j-1 {
		request.Columns[i], request.Columns[j] = request.Columns[j], request.Columns[i]
		request.Original.Cells[i], request.Original.Cells[j] = request.Original.Cells[j], request.Original.Cells[i]
	}
	original, err := sourceform.SiviDeletionSource(context.Background(), request.Project, request.Plot, request.Form, request.Original.RowID,
		vprocore.ProjectMetadataTable{Columns: request.Columns, Rows: []vprocore.ProjectMetadataRow{request.Original}})
	if err != nil || !reflect.DeepEqual(original.Columns, request.Columns) || !reflect.DeepEqual(original.Original, request.Original) {
		t.Fatal("full44 original reordered physical fields", original, err)
	}
	id, err := sourceform.ValidateSIVIDeletionOriginal(*original)
	if err != nil || strconv.FormatInt(id, 10) != "10000001" {
		t.Fatal("logical identity was inferred from a fixed ordinal", id, err)
	}
}

func TestSIVIDeletionAll44RawHistoricalCellsAndSigned64PhysicalRows(t *testing.T) {
	for _, rowID := range []string{"-9223372036854775808", "9223372036854775807"} {
		service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
		if _, err := db.Exec(`UPDATE Sample_Veg SET rowid=? WHERE rowid=?`, rowID, request.Original.RowID); err != nil {
			t.Fatal(err)
		}
		for _, column := range request.Columns {
			if column.Name == "ID" || column.Name == "PlotNumber" || column.Name == "Flag" {
				continue
			}
			if _, err := db.Exec(`UPDATE Sample_Veg SET `+vprocore.QuoteHeaderIdentifier(column.Name)+`=? WHERE rowid=?`,
				" historical Overlength Invalid literal ", rowID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`UPDATE Sample_Veg SET Flag=1 WHERE rowid=?`, rowID); err != nil {
			t.Fatal(err)
		}
		original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, request.Plot, request.Form, rowID)
		if err != nil {
			t.Fatal(err)
		}
		request.Original = original.Original
		receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request)
		if err != nil || receipt == nil || receipt.RowID != rowID || receipt.ID != 10000001 ||
			len(receipt.Audits) != 42 || !reflect.DeepEqual(receipt.Original, original.Original) {
			t.Fatal("all44 historical row or signed64 physical identity was coerced/lost", receipt, err)
		}
	}
}

func TestSIVIDeletionUnavailableLogicalStorageAndBlobsOnOriginalRead(t *testing.T) {
	for _, update := range []string{
		`ID=NULL`, `ID='bad'`, `ID=2147483648`, `ID=-2147483649`, `ID=1.5`,
		`Other2=X'FF'`, `Flag='bad'`,
	} {
		service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
		if _, err := db.Exec(`UPDATE Sample_Veg SET `+update+` WHERE rowid=?`, request.Original.RowID); err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		if original, err := sourceform.ReadSIVIDeletionOriginal(service, context.Background(), state.ContextID, request.Plot, request.Form, request.Original.RowID); original != nil || err == nil {
			t.Fatal("unavailable storage was implicitly repaired or accepted", update, original, err)
		}
		assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
		vprotest.AssertProfileSUFiles(t, service, before)
	}
}

func TestSIVIDeletionTechnicalTableRollbackAndMissingReceipt(t *testing.T) {
	service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
	if _, err := db.Exec(`CREATE TABLE "__VPRO_SIVIDeletionHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
		CREATE TRIGGER fail_history AFTER INSERT ON "__VPRO_SIVIDeletionHistory" BEGIN DELETE FROM Sample_Admin; END`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER fail_history;
		CREATE TABLE "__VPRO_ChildIdentity"(ChildTable TEXT NOT NULL,ID INTEGER NOT NULL,PRIMARY KEY(ChildTable,ID));
		CREATE TRIGGER fail_ledger AFTER INSERT ON "__VPRO_ChildIdentity" BEGIN DELETE FROM Sample_Admin; END`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
	if _, err := db.Exec(`DROP TRIGGER fail_ledger`); err != nil {
		t.Fatal(err)
	}
	if receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal("technical rollback blocked retry", receipt, err)
	}
	if _, err := db.Exec(`DROP TABLE "__VPRO_SIVIDeletionHistory"`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if receipt, err := sourceform.LookupSIVIDeletionReceipt(service, context.Background(), state.ContextID, request); receipt != nil || err != nil {
		t.Fatal("missing receipt recreated history or inferred committed result", receipt, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	assertSIVIDeletionRefused(t, service, state.ContextID, request, false)
}

func TestSIVIDeletionCanonicalHistoryInternalAuthority(t *testing.T) {
	service, state, db, request := sftest.SiviDeletionFixture(t, false, 3)
	if receipt, err := sourceform.DeleteSIVIVegetation(service, context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal(receipt, err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory"`).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []func(*sourceform.SiviDeletionHistory){
		func(h *sourceform.SiviDeletionHistory) { h.Request.Project = "Foreign" },
		func(h *sourceform.SiviDeletionHistory) { h.Result.ID++ },
		func(h *sourceform.SiviDeletionHistory) { h.Result.RowID = "-1" },
		func(h *sourceform.SiviDeletionHistory) { h.Result.HistoryID = "00000000-0000-4000-8000-000000000099" },
		func(h *sourceform.SiviDeletionHistory) { h.Actor = "" },
		func(h *sourceform.SiviDeletionHistory) { h.AuditStrength = 4 },
		func(h *sourceform.SiviDeletionHistory) { h.Columns[0].DeclaredType = "CHANGED" },
		func(h *sourceform.SiviDeletionHistory) { h.Original.Cells[0] = vprotest.MetadataText("other") },
		func(h *sourceform.SiviDeletionHistory) { h.Audits = h.Audits[:len(h.Audits)-1] },
		func(h *sourceform.SiviDeletionHistory) { h.Audits[0].EditField = "ID" },
		func(h *sourceform.SiviDeletionHistory) { h.Audits[0].BeforeEdit = nil },
		func(h *sourceform.SiviDeletionHistory) { h.Audits[0].AfterEdit = h.Audits[0].BeforeEdit },
		func(h *sourceform.SiviDeletionHistory) { h.Audits[0].Restore = true },
	} {
		var history sourceform.SiviDeletionHistory
		if err := json.Unmarshal([]byte(proposal), &history); err != nil {
			t.Fatal(err)
		}
		corrupt(&history)
		broken, err := json.Marshal(history)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=?`, string(broken)); err != nil {
			t.Fatal(err)
		}
		assertSIVIDeletionRefused(t, service, state.ContextID, request, true)
	}
}
