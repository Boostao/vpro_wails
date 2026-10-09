package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const vegetationWriteUUID = "b0cbfa00-1234-4123-8123-000000000001"

func vegetationWriteFixture(t *testing.T, external bool, form string, strength int) (*ContextService, ProjectState, *sql.DB, twoPageVegetationRequest) {
	t.Helper()
	service, state, db := twoPageVegetationReadFixture(t, external, "TEXT")
	if err := service.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	service.plots.mu.Lock()
	service.plots.currentUser = "vegetation-review"
	service.plots.mu.Unlock()
	if _, err := db.Exec(`INSERT INTO Sample_Veg(rowid,ID,PlotNumber,Species,Cover1,Cover2,cover3,
		TotalA,Cover4,Cover5,TotalB,Cover5a,Cover5b,Cover5c,Cover6,Cover7,Cover8,Cover9,Collected,Unmapped) VALUES
		(-7,42,'108050','TEST',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL,X'FF00'),
		(3,42,'108050','PEER',0,0,0,0,0,0,0,0,0,0,0,0,0,0,'V',X'00FF'),
		(8,NULL,'108050','NULLID',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL,NULL),
		(9,42,'other','OUTSIDE',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	column := "Cover1"
	if form == "SubVegCXL" {
		column = "Cover6"
	} else if form == "SubVegDXL" {
		column = "Cover7"
	}
	request := twoPageVegetationRequest{vegetationWriteUUID, original, []twoPageVegetationEdit{
		vegetationPlanEdit(original, column, siviReal(5)),
		vegetationPlanEdit(original, "Collected", metadataText("C")),
	}}
	return service, state, db, request
}

func TestTwoPageVegetationWriteAtomicStrengthsAndReadonlyRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		for strength, auditCount := range []int{0, 1, 2, 3} {
			t.Run(fmt.Sprintf("external-%v/strength-%d", external, strength), func(t *testing.T) {
				service, state, db, request := vegetationWriteFixture(t, external, "SubVegAXL_BC", strength)
				request.Edits = append(request.Edits[:1], vegetationPlanEdit(request.Original, "Cover2", ProjectMetadataCell{Storage: "null"}), request.Edits[1])
				before := databaseBytes(t, service.projects.sqlite.attachments)
				if result, err := service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, request); err != nil || result != nil {
					t.Fatalf("absent history is unresolved, not a commit certificate: %+v %v", result, err)
				}
				if !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
					t.Fatal("unresolved lookup created data/schema")
				}
				priorAudits := soilWriteCount(t, db, "Sample_Audit")
				result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil || result == nil || !result.DidCommit || result.Replayed || len(result.History.Audits) != auditCount ||
					!reflect.DeepEqual(result.History.Request, request) {
					t.Fatalf("complete mixed audit provenance = %+v %v", result, err)
				}
				if soilWriteCount(t, db, "Sample_Audit") != priorAudits+auditCount || soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
					t.Fatal("physical source audit count or always-durable history differs")
				}
				current, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", request.Original.Form)
				if err != nil || !reflect.DeepEqual(current.Records, result.History.Committed) {
					t.Fatalf("complete reopened committed rows differ: %+v %v", current, err)
				}
				for _, rowID := range []string{"3", "8"} {
					for i, column := range request.Original.Records.Columns {
						if !reflect.DeepEqual(*vegetationPlanCell(current, rowID, column.Name), *vegetationPlanCell(request.Original, rowID, column.Name)) {
							t.Fatalf("duplicate-ID/NULL peer cell %s index%d changed", rowID, i)
						}
					}
				}
				var outside string
				if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE rowid=9`).Scan(&outside); err != nil || outside != "OUTSIDE" {
					t.Fatal("global duplicate ID peer changed", outside, err)
				}
				for _, audit := range result.History.Audits {
					if audit.ID == nil || *audit.ID != 42 || audit.Table != "_Veg" || audit.User != "vegetation-review" {
						t.Fatal("nonunique signed logical audit identity/provenance changed")
					}
				}
				before = databaseBytes(t, service.projects.sqlite.attachments)
				replay, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil || replay == nil || replay.DidCommit || !replay.Replayed || !reflect.DeepEqual(replay.History, result.History) {
					t.Fatalf("once-only replay differs: %+v %v", replay, err)
				}
				recovery, err := service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, request)
				if err != nil || recovery == nil || recovery.DidCommit || !recovery.Replayed || !reflect.DeepEqual(recovery.History, result.History) ||
					!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
					t.Fatalf("readonly recovery changed physical storage: %+v %v", recovery, err)
				}
				if _, err := db.Exec(`UPDATE Sample_Veg SET Species='LATER' WHERE rowid=-7`); err != nil {
					t.Fatal(err)
				}
				before = databaseBytes(t, service.projects.sqlite.attachments)
				recovery, err = service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, request)
				if err != nil || recovery == nil || !reflect.DeepEqual(recovery.History, result.History) ||
					!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
					t.Fatalf("later drift erased receipt or got repaired: %+v %v", recovery, err)
				}
			})
		}
	}
}

func TestTwoPageVegetationWriteAllBindingsAndSourceDeparture(t *testing.T) {
	for _, test := range []struct {
		form   string
		fields []string
	}{
		{"SubVegAXL_BC", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Collected"}},
		{"SubVegAXL", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c", "Collected"}},
		{"SubVegCXL", []string{"Cover6", "Collected"}},
		{"SubVegDXL", []string{"Cover7", "Cover8", "Cover9", "Collected"}},
	} {
		t.Run(test.form, func(t *testing.T) {
			service, state, _, request := vegetationWriteFixture(t, true, test.form, 3)
			request.Edits = nil
			for _, column := range test.fields {
				value := siviReal(1.25)
				if column == "Collected" {
					value = metadataText("")
				} else if test.form == "SubVegCXL" {
					value = ProjectMetadataCell{Storage: "null"}
				}
				request.Edits = append(request.Edits, vegetationPlanEdit(request.Original, column, value))
			}
			result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
			if err != nil || result == nil || len(result.History.Audits) != len(test.fields) {
				t.Fatalf("source-bound writes differ: %+v %v", result, err)
			}
			current, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", test.form)
			if err != nil || !reflect.DeepEqual(current.Records, result.History.Committed) {
				t.Fatalf("reopened source cells differ: %+v %v", current, err)
			}
			if test.form == "SubVegCXL" && !reflect.DeepEqual(current.SourceRowIDs, []string{"3", "8"}) {
				t.Fatal("cleared final cover was deleted or retained in source membership", current.SourceRowIDs)
			}
		})
	}
}

func TestTwoPageVegetationWriteRollbackCancelRetryAndCommittedErrors(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprint("cancel-", cancelRequest), func(t *testing.T) {
			service, state, _, request := vegetationWriteFixture(t, true, "SubVegAXL_BC", 3)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			before := databaseBytes(t, service.projects.sqlite.attachments)
			planned := errors.New("planned source transaction refusal")
			result, err := service.writeTwoPageVegetationObserved(ctx, state.ContextID, request, func(*sql.Tx) error {
				if cancelRequest {
					cancel()
					return nil
				}
				return planned
			}, nil)
			if result != nil || err == nil || !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatalf("rollback/cancel escaped atomic boundary: %+v %v", result, err)
			}
			if cancelRequest && !errors.Is(err, context.Canceled) || !cancelRequest && !errors.Is(err, planned) {
				t.Fatal("operation-specific error lost", err)
			}
			committedError := errors.New("planned postcommit completion failure")
			result, err = service.writeTwoPageVegetationObserved(context.Background(), state.ContextID, request, nil,
				func(*twoPageVegetationWriteResult) error { return committedError })
			if result == nil || !result.DidCommit || !errors.Is(err, committedError) {
				t.Fatalf("committed error lost truthful irreversible result: %+v %v", result, err)
			}
			recovered, err := service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, request)
			if err != nil || recovered == nil || !reflect.DeepEqual(recovered.History, result.History) {
				t.Fatalf("committed error could not recover without replay: %+v %v", recovered, err)
			}
		})
	}
}

func TestTwoPageVegetationWriteSnapshotAndHistoryRefusals(t *testing.T) {
	for _, drift := range []string{"peer", "schema", "history-view", "history-case", "parent"} {
		t.Run(drift, func(t *testing.T) {
			service, state, db, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
			queries := map[string]string{
				"peer":         `UPDATE Sample_Veg SET Species='REPLACED' WHERE rowid=3`,
				"schema":       `ALTER TABLE Sample_Veg ADD COLUMN Extra`,
				"history-view": `CREATE VIEW "__VPRO_TwoPageVegetationHistory" AS SELECT 1 RequestID,2 Created,3 Proposal`,
				"history-case": `CREATE TABLE "__vpro_twopagevegetationhistory"(RequestID TEXT,Created TEXT,Proposal TEXT)`,
				"parent":       `DELETE FROM Sample_Admin WHERE Plot='108050'`,
			}
			if _, err := db.Exec(queries[drift]); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request); err == nil || result != nil ||
				!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatalf("source drift/history impersonation accepted or repaired: %+v %v", result, err)
			}
		})
	}
	service, state, db, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
	if result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request); err != nil || result == nil {
		t.Fatal(result, err)
	}
	different := request
	different.Edits = append([]twoPageVegetationEdit{}, request.Edits...)
	different.Edits[0].Value = siviReal(6)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, lookup := range []bool{false, true} {
		var result *twoPageVegetationWriteResult
		var err error
		if lookup {
			result, err = service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, different)
		} else {
			result, err = service.writeTwoPageVegetation(context.Background(), state.ContextID, different)
		}
		if err == nil || result != nil || !strings.Contains(err.Error(), "UUID belongs") ||
			!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
			t.Fatalf("UUID collision became a replay/fallback: %+v %v", result, err)
		}
	}
	if _, err := db.Exec(`UPDATE "__VPRO_TwoPageVegetationHistory" SET Proposal='{}'`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, request); err == nil || result != nil ||
		!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatalf("malformed durable history was silently recovered/repaired: %+v %v", result, err)
	}
}

func TestTwoPageVegetationWriteConcurrentSameUUID(t *testing.T) {
	service, state, db, request := vegetationWriteFixture(t, true, "SubVegAXL_BC", 3)
	var wait sync.WaitGroup
	wait.Add(2)
	results := make(chan *twoPageVegetationWriteResult, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wait.Done()
			result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
			results <- result
			failures <- err
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	commits, replays := 0, 0
	for result := range results {
		if result == nil {
			t.Fatal("concurrent request lost result")
		}
		if result.DidCommit {
			commits++
		}
		if result.Replayed {
			replays++
		}
	}
	if commits != 1 || replays != 1 || soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
		t.Fatal("stable UUID did not commit exactly once", commits, replays)
	}
}

func TestTwoPageVegetationRequestStrictBoundary(t *testing.T) {
	service, _, _, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded twoPageVegetationRequest
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("strict full request roundtrip failed", err)
	}
	for _, invalid := range []string{
		strings.Replace(string(wire), `"requestId":`, `"requestId":"`+vegetationWriteUUID+`","requestId":`, 1),
		strings.Replace(string(wire), `"sourceRowIds":`, `"sourceRowIds":[],"\u0073ourceRowIds":`, 1),
		strings.Replace(string(wire), `"declaredType":`, `"type":"wrong","declaredType":`, 1),
		strings.Replace(string(wire), `"PEER"`, `"\ud800"`, 1),
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("strict request allowed ambiguous transport", invalid)
		}
	}

	detached, err := detachTwoPageVegetationRequest(context.Background(), service, request)
	if err != nil {
		t.Fatal(err)
	}
	*vegetationPlanCell(detached.Original, "3", "Species").Text = "DETACHED"
	if *vegetationPlanCell(request.Original, "3", "Species").Text != "PEER" {
		t.Fatal("transport detachment retained mutable original peer aliases")
	}
}

func TestTwoPageVegetationWriteExternalSUReservationAndWALRetry(t *testing.T) {
	for _, wal := range []bool{false, true} {
		t.Run(fmt.Sprint("wal-", wal), func(t *testing.T) {
			service, state, db, request := vegetationWriteFixture(t, true, "SubVegAXL_BC", 3)
			su, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["su"], "rw")+"&_busy_timeout=20")
			if err != nil {
				t.Fatal(err)
			}
			defer su.Close()
			if wal {
				var mode string
				if err := su.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
					t.Fatal("external WAL fixture setup failed", mode, err)
				}
				priorAudits := soilWriteCount(t, db, "Sample_Audit")
				result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if result != nil || err == nil || !strings.Contains(err.Error(), "selected SU observations require rollback-journal") ||
					soilWriteCount(t, db, "Sample_Audit") != priorAudits {
					t.Fatalf("unstable external SU membership allowed mutation: %+v %v", result, err)
				}
				if err := su.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil || mode != "delete" {
					t.Fatal("external journal correction failed", mode, err)
				}
			}
			blocked := false
			result, err := service.writeTwoPageVegetationObserved(context.Background(), state.ContextID, request, func(*sql.Tx) error {
				_, err := su.Exec(`DELETE FROM Report_SU WHERE PlotNumber='108050'`)
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
					return fmt.Errorf("external SU was not reserved through commit: %v", err)
				}
				blocked = true
				return nil
			}, nil)
			if err != nil || result == nil || !result.DidCommit || !blocked {
				t.Fatalf("external read reservation or corrected retry failed: %+v %v", result, err)
			}
		})
	}
}

func TestTwoPageVegetationWriteUnavailableOwnersAndNoChange(t *testing.T) {
	for _, refusal := range []string{"stale-context", "forged-context", "actor", "policy", "no-change"} {
		t.Run(refusal, func(t *testing.T) {
			service, state, _, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
			contextID := state.ContextID
			switch refusal {
			case "stale-context":
				contextID += "-stale"
			case "forged-context":
				request.Original.ContextID += "-forged"
				for i := range request.Edits {
					request.Edits[i].ContextID = request.Original.ContextID
				}
			case "actor":
				service.plots.mu.Lock()
				service.plots.currentUser = ""
				service.plots.mu.Unlock()
			case "policy":
				service.plots.mu.Lock()
				service.plots.auditStrength = -1
				service.plots.mu.Unlock()
			case "no-change":
				for i := range request.Edits {
					request.Edits[i].Value = cloneSiteUnitCell(request.Edits[i].Expected)
				}
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if result, err := service.writeTwoPageVegetation(context.Background(), contextID, request); err == nil || result != nil ||
				!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
				t.Fatalf("unavailable owner or unchanged plan created mutation/provenance: %+v %v", result, err)
			}
		})
	}
}

func TestTwoPageVegetationWriteLosslessAffinity(t *testing.T) {
	for _, affinity := range []string{"REAL", "INTEGER"} {
		t.Run(affinity, func(t *testing.T) {
			service, state, db, _ := vegetationWriteFixture(t, false, "SubVegCXL", 3)
			if _, err := db.Exec(`DROP TABLE Sample_Veg; CREATE TABLE Sample_Veg(
					ID,PlotNumber TEXT,Species TEXT,Cover6 ` + affinity + `,Collected TEXT);
					INSERT INTO Sample_Veg(rowid,ID,PlotNumber,Species,Cover6) VALUES(-7,42,'108050','TEST',0)`); err != nil {
				t.Fatal(err)
			}
			original, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", "SubVegCXL")
			if err != nil {
				t.Fatal(err)
			}
			request := twoPageVegetationRequest{vegetationWriteUUID, original, []twoPageVegetationEdit{
				vegetationPlanEdit(original, "Cover6", siviReal(5)),
			}}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
			if affinity == "INTEGER" {
				if result != nil || err == nil || !strings.Contains(err.Error(), "complete plan") ||
					!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
					t.Fatalf("affinity normalization masqueraded as exact typed commit: %+v %v", result, err)
				}
			} else if err != nil || result == nil || !result.DidCommit {
				t.Fatalf("REAL source storage failed exact typed write: %+v %v", result, err)
			}
		})
	}
}

func TestTwoPageVegetationWriteExactRealBitsAndUUIDWitness(t *testing.T) {
	service, state, db, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover4=CAST(0 AS REAL) WHERE rowid=-7`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", request.Original.Form)
	if err != nil {
		t.Fatal(err)
	}
	request.Original = original
	request.Edits = []twoPageVegetationEdit{vegetationPlanEdit(original, "Cover1", siviReal(5))}
	for _, forgedOriginal := range []bool{false, true} {
		wrong := request
		wrong.Original = cloneVegetationPlanOriginal(t, request.Original)
		wrong.Edits = append([]twoPageVegetationEdit{}, request.Edits...)
		if forgedOriginal {
			*vegetationPlanCell(wrong.Original, "-7", "Cover4") = siviReal(math.Copysign(0, -1))
		} else {
			wrong.Edits[0].Value = siviReal(math.Copysign(0, -1))
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, wrong); result != nil || err == nil ||
			!reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
			t.Fatalf("negative-zero value/original witness was silently normalized: %+v %v", result, err)
		}
	}
	if result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request); err != nil || result == nil {
		t.Fatal(result, err)
	}
	wrong := request
	wrong.Original = cloneVegetationPlanOriginal(t, request.Original)
	*vegetationPlanCell(wrong.Original, "-7", "Cover4") = siviReal(math.Copysign(0, -1))
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.lookupTwoPageVegetationReceipt(context.Background(), state.ContextID, wrong); result != nil || err == nil ||
		!strings.Contains(err.Error(), "UUID belongs") || !reflect.DeepEqual(before, databaseBytes(t, service.projects.sqlite.attachments)) {
		t.Fatalf("different real-bit UUID witness was mistaken for exact replay: %+v %v", result, err)
	}
}
