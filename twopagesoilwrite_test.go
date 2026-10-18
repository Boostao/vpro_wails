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
)

const soilWriteUUID = "f4b1aa00-1234-4123-8123-000000000001"

func soilWriteFixture(t *testing.T, external bool, form string, strength int) (*ContextService, ProjectState, *sql.DB, twoPageSoilRequest) {
	t.Helper()
	service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	if err := service.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	service.plots.mu.Lock()
	service.plots.currentUser = "soil-review"
	service.plots.mu.Unlock()
	table := "Sample_Humus"
	if form == "SoilMineralXL" {
		table = "Sample_Mineral"
	}
	for _, column := range soilPlanSourceFields[form] {
		if column == "Horizon" || column == "UpperDepth" || column == "Comment" {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE ` + quoteHeaderIdentifier(table) + ` ADD COLUMN ` + quoteHeaderIdentifier(column)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO ` + quoteHeaderIdentifier(table) + `(rowid,ID,PlotNumber,Horizon,UpperDepth,Unmapped)
		VALUES(-7,0,'108050','H',-0.25,X'FF00'),(2,NULL,'108050','historical overlength',NULL,X'00FF')`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	comment := "Comment"
	if form == "SoilMineralXL" {
		comment = "Comments"
	}
	request := twoPageSoilRequest{RequestID: soilWriteUUID, Original: original, Edits: []twoPageSoilEdit{
		soilPlanEdit(original, "Horizon", ProjectMetadataCell{Storage: "null"}),
		soilPlanEdit(original, "UpperDepth", siviReal(5)),
		soilPlanEdit(original, comment, metadataText("")),
	}}
	return service, state, db, request
}

func soilWriteCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + quoteHeaderIdentifier(table)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestTwoPageSoilWriteAtomicAuditStrengthsAndRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			for strength, auditCount := range []int{0, 1, 2, 3} {
				t.Run(fmt.Sprintf("%s/external-%v/strength-%d", form, external, strength), func(t *testing.T) {
					service, state, db, request := soilWriteFixture(t, external, form, strength)
					before := databaseBytes(t, service.projects.sqlite.attachments)
					if result, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err != nil || result != nil {
						t.Fatalf("uncommitted receipt must remain unresolved: %v, %v", result, err)
					}
					if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
						t.Fatal("unresolved lookup wrote data/schema")
					}
					priorAudits := soilWriteCount(t, db, "Sample_Audit")
					result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
					if err != nil || result == nil || !result.DidCommit || result.Replayed ||
						len(result.History.Audits) != auditCount || !reflect.DeepEqual(result.History.Request, request) {
						t.Fatalf("complete mixed typed receipt = %#v, %v", result, err)
					}
					if soilWriteCount(t, db, "Sample_Audit") != priorAudits+auditCount ||
						soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
						t.Fatal("audit policy or always-durable technical history differs")
					}
					reopened, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", form)
					if err != nil || !reflect.DeepEqual(reopened.Records, result.History.Committed) {
						t.Fatalf("physical committed rows/NULL/empty/unmapped cells differ: %v, %v", reopened, err)
					}
					for _, audit := range result.History.Audits {
						if audit.ID == nil || *audit.ID != 0 || audit.User != "soil-review" || audit.Restore || audit.Flag {
							t.Fatal("soil audit invented a logical identity or flags")
						}
						switch audit.EditField {
						case "Horizon":
							if audit.BeforeEdit == nil || *audit.BeforeEdit != "H" || audit.AfterEdit != nil {
								t.Fatal("deletion audit did not preserve NULL")
							}
						case "UpperDepth":
							if audit.BeforeEdit == nil || *audit.BeforeEdit != "-0.25" || audit.AfterEdit == nil || *audit.AfterEdit != "5" {
								t.Fatal("numeric edit audit differs")
							}
						case "Comment", "Comments":
							if audit.BeforeEdit != nil || audit.AfterEdit == nil || *audit.AfterEdit != "" {
								t.Fatal("addition audit collapsed empty to NULL")
							}
						default:
							t.Fatal("unrequested field audit")
						}
					}
					before = databaseBytes(t, service.projects.sqlite.attachments)
					for _, lookup := range []bool{false, true} {
						var replay *twoPageSoilWriteResult
						if lookup {
							replay, err = service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request)
						} else {
							replay, err = service.writeTwoPageSoil(context.Background(), state.ContextID, request)
						}
						if err != nil || replay == nil || replay.DidCommit || !replay.Replayed || !reflect.DeepEqual(replay.History, result.History) {
							t.Fatalf("exact request recovery/replay differs: %v, %v", replay, err)
						}
					}
					if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
						t.Fatal("receipt recovery/replay mutated physical source/history")
					}
					if _, err := db.Exec(`UPDATE ` + quoteHeaderIdentifier(request.Original.Table) + ` SET Horizon='later' WHERE rowid=-7`); err != nil {
						t.Fatal(err)
					}
					before = databaseBytes(t, service.projects.sqlite.attachments)
					recovered, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request)
					if err != nil || recovered == nil || !reflect.DeepEqual(recovered.History, result.History) {
						t.Fatal("later source drift erased truthful irreversible provenance", recovered, err)
					}
					if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
						t.Fatal("historical recovery repaired later source drift")
					}
				})
			}
		}
	}
}

func TestTwoPageSoilWriteAllSourceBindings(t *testing.T) {
	integers := map[string]bool{"vonPost": true, "ASP": true, "PercentCoarseFragsGravel": true,
		"PercentCoarseFragsCobbles": true, "PercentCoarseFragsStones": true, "PercentCoarseFragsTotal": true}
	reals := map[string]bool{"UpperDepth": true, "LowerDepth": true, "HumusFormpH": true, "MineralFormpH": true}
	for form, fields := range soilPlanSourceFields {
		t.Run(form, func(t *testing.T) {
			service, state, _, request := soilWriteFixture(t, true, form, 3)
			request.Edits = nil
			for _, column := range fields {
				value := metadataText("X")
				if integers[column] {
					value = soilPlanInteger("12")
				} else if reals[column] {
					value = siviReal(2.5)
				}
				request.Edits = append(request.Edits, soilPlanEdit(request.Original, column, value))
			}
			result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
			if err != nil || result == nil || len(result.History.Audits) != len(fields) {
				t.Fatalf("all visible soil bindings did not commit/audit: %v, %v", result, err)
			}
			reopened, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", form)
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range request.Edits {
				for i, column := range reopened.Records.Columns {
					if column.Name == edit.Column && !reflect.DeepEqual(reopened.Records.Rows[0].Cells[i], edit.Value) {
						t.Fatalf("independent physical cell %s differs", edit.Column)
					}
				}
			}
		})
	}
}

func TestTwoPageSoilWriteRollbackRefusalsAndRetry(t *testing.T) {
	for name, fault := range map[string]string{
		"global integer peer":  `INSERT INTO Sample_Humus(ID,PlotNumber,Horizon) VALUES(0,'other','peer')`,
		"global text alias":    `INSERT INTO Sample_Humus(ID,PlotNumber,Horizon) VALUES('00','other','peer')`,
		"replaced original":    `DELETE FROM Sample_Humus WHERE rowid=-7; INSERT INTO Sample_Humus(rowid,ID,PlotNumber,Horizon) VALUES(-7,0,'108050','replacement')`,
		"complete peer drift":  `UPDATE Sample_Humus SET Unmapped=X'F0' WHERE rowid=2`,
		"parent schema":        `ALTER TABLE Sample_Admin ADD COLUMN Generated TEXT GENERATED ALWAYS AS ('drift') VIRTUAL`,
		"soil identity shadow": `ALTER TABLE Sample_Humus ADD COLUMN oid TEXT`,
		"trigger side effect":  `CREATE TRIGGER SoilDrift AFTER UPDATE ON Sample_Humus BEGIN UPDATE Sample_Humus SET Unmapped=X'F0' WHERE rowid=2; END`,
		"trigger audit error":  `CREATE TRIGGER SoilAuditFailure BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'planned soil audit failure'); END`,
		"history failure": `CREATE TABLE "__VPRO_TwoPageSoilHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
			CREATE TRIGGER SoilHistoryFailure BEFORE INSERT ON "__VPRO_TwoPageSoilHistory" BEGIN SELECT RAISE(ABORT,'planned soil history failure'); END`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, db, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
			if _, err := db.Exec(fault); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request); err == nil || result != nil {
				t.Fatalf("unsafe/drifting source produced receipt: %v, %v", result, err)
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
				t.Fatal("refused/rolled back soil write changed source/audit/history bytes")
			}
			switch name {
			case "trigger audit error", "trigger side effect", "history failure":
				trigger := map[string]string{"trigger audit error": "SoilAuditFailure", "trigger side effect": "SoilDrift", "history failure": "SoilHistoryFailure"}[name]
				if _, err := db.Exec(`DROP TRIGGER ` + quoteHeaderIdentifier(trigger)); err != nil {
					t.Fatal(err)
				}
				if result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request); err != nil || result == nil || !result.DidCommit {
					t.Fatalf("corrected transaction did not retry exactly once: %v, %v", result, err)
				}
			}
		})
	}
}

func TestTwoPageSoilWriteCancellationAndCommittedCompletionError(t *testing.T) {
	service, state, db, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeTwoPageSoil(ctx, state.ContextID, request); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("precommit cancellation succeeded", result, err)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("precommit cancellation changed data")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	completionError := errors.New("planned completion observer error")
	result, err := service.writeTwoPageSoilObserved(ctx, state.ContextID, request, nil, func(receipt *twoPageSoilWriteResult) error {
		var depth float64
		if err := db.QueryRow(`SELECT UpperDepth FROM Sample_Humus WHERE rowid=-7`).Scan(&depth); err != nil || depth != 5 {
			t.Fatalf("observer ran before independently visible commit: %v, %v", depth, err)
		}
		if !receipt.DidCommit || soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
			t.Fatal("observer lost irreversible receipt")
		}
		cancel()
		return errors.Join(completionError, ctx.Err())
	})
	if result == nil || !result.DidCommit || !errors.Is(err, completionError) || !errors.Is(err, context.Canceled) ||
		!strings.Contains(err.Error(), "committed but completion failed") {
		t.Fatal("committed result was erased or error hidden", result, err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	recovered, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request)
	if err != nil || recovered == nil || !reflect.DeepEqual(recovered.History, result.History) {
		t.Fatal("postcommit cancellation lost owned coordinator/receipt", recovered, err)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("postcommit recovery replayed mutation")
	}
}

func TestTwoPageSoilWriteInFlightCancellationRollsBackAndRetries(t *testing.T) {
	service, state, db, request := soilWriteFixture(t, true, "SoilMineralXL", 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	observed := false
	result, err := service.writeTwoPageSoilObserved(ctx, state.ContextID, request, func(tx *sql.Tx) error {
		observed = true
		var staged, independent float64
		if err := tx.QueryRowContext(ctx, `SELECT UpperDepth FROM Sample_Mineral WHERE rowid=-7`).Scan(&staged); err != nil || staged != 5 {
			t.Fatalf("cancellation did not reach actual staged mutation: %v, %v", staged, err)
		}
		if err := db.QueryRow(`SELECT UpperDepth FROM Sample_Mineral WHERE rowid=-7`).Scan(&independent); err != nil || independent != -0.25 {
			t.Fatalf("uncommitted mutation leaked to independent reader: %v, %v", independent, err)
		}
		var pending int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "__VPRO_TwoPageSoilHistory"`).Scan(&pending); err != nil || pending != 1 {
			t.Fatal("cancellation did not include staged technical provenance", pending, err)
		}
		cancel()
		return ctx.Err()
	}, nil)
	if !observed || !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("in-flight cancellation returned a committed receipt", result, err)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("in-flight cancellation left data/audit/history writes")
	}
	if receipt, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err != nil || receipt != nil {
		t.Fatal("cancelled request falsely became committed or lost coordinator", receipt, err)
	}
	if receipt, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request); err != nil || receipt == nil || !receipt.DidCommit ||
		soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
		t.Fatal("cancelled source plan did not retry once", receipt, err)
	}
}

func TestTwoPageSoilWriteReceiptCorruptionAndCorrection(t *testing.T) {
	service, state, db, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	accepted, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	var original string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_TwoPageSoilHistory" WHERE RequestID=?`, request.RequestID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(twoPageSoilHistory) twoPageSoilHistory{
		func(h twoPageSoilHistory) twoPageSoilHistory { h.Actor = ""; return h },
		func(h twoPageSoilHistory) twoPageSoilHistory { h.AuditStrength = 0; return h },
		func(h twoPageSoilHistory) twoPageSoilHistory {
			h.Committed.Rows[0].Cells[0] = soilPlanInteger("1")
			return h
		},
		func(h twoPageSoilHistory) twoPageSoilHistory { h.Audits[0].EditField = "Unmapped"; return h },
		func(h twoPageSoilHistory) twoPageSoilHistory {
			h.Request.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
			return h
		},
	} {
		var history twoPageSoilHistory
		if err := json.Unmarshal([]byte(original), &history); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(mutate(history))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_TwoPageSoilHistory" SET Proposal=? WHERE RequestID=?`, string(raw), request.RequestID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if receipt, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err == nil || receipt != nil {
			t.Fatal("corrupt receipt was silently accepted", receipt, err)
		}
		if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
			t.Fatal("corrupt receipt lookup repaired storage")
		}
	}
	if _, err := db.Exec(`UPDATE "__VPRO_TwoPageSoilHistory" SET Proposal=? WHERE RequestID=?`, original, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err != nil || receipt == nil ||
		!reflect.DeepEqual(receipt.History, accepted.History) {
		t.Fatal("exact receipt correction did not recover provenance", receipt, err)
	}
}

func TestTwoPageSoilWriteRefusesHistoryViewsAndShadows(t *testing.T) {
	for _, schema := range []string{
		`CREATE VIEW "__VPRO_TwoPageSoilHistory" AS SELECT '' RequestID,'' Created,'' Proposal`,
		`CREATE TABLE "__vpro_twopagesoilhistory"(RequestID TEXT,Created TEXT,Proposal TEXT)`,
		`CREATE TABLE "__VPRO_TwoPageSoilHistory"(RequestID TEXT,Created TEXT,Proposal TEXT,oid TEXT)`,
		`CREATE TABLE "__VPRO_TwoPageSoilHistory"(RequestID TEXT,Created TEXT,Proposal TEXT,Extra TEXT GENERATED ALWAYS AS ('hidden') VIRTUAL)`,
	} {
		t.Run(schema, func(t *testing.T) {
			service, state, db, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
			if _, err := db.Exec(schema); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if receipt, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err == nil || receipt != nil {
				t.Fatal("unavailable history produced unresolved/success-shaped receipt", receipt, err)
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
				t.Fatal("history refusal changed source storage")
			}
		})
	}
}

func TestTwoPageSoilWriteConcurrentUUIDAndPayloadCollision(t *testing.T) {
	for _, sameUUID := range []bool{false, true} {
		t.Run(fmt.Sprint(sameUUID), func(t *testing.T) {
			service, state, db, first := soilWriteFixture(t, false, "SoilHumusXL", 3)
			second := first
			if !sameUUID {
				second.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
			}
			type outcome struct {
				result *twoPageSoilWriteResult
				err    error
			}
			start := make(chan struct{})
			results := make(chan outcome, 2)
			var ready sync.WaitGroup
			ready.Add(2)
			for _, request := range []twoPageSoilRequest{first, second} {
				go func(request twoPageSoilRequest) {
					ready.Done()
					<-start
					result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
					results <- outcome{result, err}
				}(request)
			}
			ready.Wait()
			close(start)
			commits, replays, failures := 0, 0, 0
			for i := 0; i < 2; i++ {
				got := <-results
				if got.err != nil {
					failures++
					if got.result != nil || !strings.Contains(got.err.Error(), "complete original/schema changed") {
						t.Fatal("concurrent stale source failed for wrong reason", got)
					}
				} else if got.result.DidCommit {
					commits++
				} else if got.result.Replayed {
					replays++
				}
			}
			if commits != 1 || sameUUID && (replays != 1 || failures != 0) || !sameUUID && (replays != 0 || failures != 1) ||
				soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
				t.Fatalf("concurrent commit/replay/failure = %d/%d/%d", commits, replays, failures)
			}
			if sameUUID {
				conflict := first
				conflict.Edits = append([]twoPageSoilEdit{}, first.Edits...)
				conflict.Edits[1].Value = siviReal(6)
				before := databaseBytes(t, service.projects.sqlite.attachments)
				for _, lookup := range []bool{false, true} {
					var result *twoPageSoilWriteResult
					var err error
					if lookup {
						result, err = service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, conflict)
					} else {
						result, err = service.writeTwoPageSoil(context.Background(), state.ContextID, conflict)
					}
					if err == nil || result != nil || !strings.Contains(err.Error(), "UUID belongs") {
						t.Fatal("different payload reused committed UUID", result, err)
					}
				}
				if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
					t.Fatal("UUID collision changed committed data")
				}
			}
		})
	}
}

func TestTwoPageSoilWriteRefusesAffinityNormalization(t *testing.T) {
	service, state, db := twoPageSoilReadFixture(t, false, "TEXT")
	if _, err := db.Exec(`DROP TABLE Sample_Humus;
		CREATE TABLE Sample_Humus(ID,PlotNumber TEXT,UpperDepth INTEGER,Horizon TEXT,Comment TEXT,Unmapped BLOB);
		INSERT INTO Sample_Humus(rowid,ID,PlotNumber,UpperDepth,Horizon,Unmapped)
		VALUES(-7,0,'108050',-0.25,'H',X'FF00')`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL")
	if err != nil {
		t.Fatal(err)
	}
	request := twoPageSoilRequest{RequestID: soilWriteUUID, Original: original, Edits: []twoPageSoilEdit{soilPlanEdit(original, "UpperDepth", siviReal(5))}}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request); err == nil || receipt != nil ||
		!strings.Contains(err.Error(), "transaction differs from its complete plan") {
		t.Fatal("INTEGER affinity silently repaired reviewed REAL storage", receipt, err)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("affinity normalization refusal changed source/audits/history")
	}
}

func TestTwoPageSoilWriteActualContextRotationAndReceiptProvenance(t *testing.T) {
	service, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	accepted, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	selection := service.projects.sqlite.selection
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
		SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath})
	if err != nil || next.ContextID == state.ContextID {
		t.Fatal("actual current owner did not rotate", next, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err == nil || receipt != nil {
		t.Fatal("stale owner retained read authority", receipt, err)
	}
	recovered, err := service.lookupTwoPageSoilReceipt(context.Background(), next.ContextID, request)
	if err != nil || recovered == nil || !reflect.DeepEqual(recovered.History, accepted.History) ||
		recovered.History.Request.Original.ContextID != state.ContextID {
		t.Fatal("new read owner erased immutable original context provenance", recovered, err)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("current-owner recovery changed source/history")
	}
}

func TestTwoPageSoilWriteStrictRequestAndInvalidGoValues(t *testing.T) {
	service, state, _, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded twoPageSoilRequest
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("complete original strict wire does not round trip", err)
	}
	valid := string(raw)
	for _, invalid := range []string{
		strings.Replace(valid, `"requestId":`, `"unknown":`, 1),
		strings.Replace(valid, `"project":"Sample"`, `"project":"Sample","\u0070roject":"Other"`, 1),
		strings.Replace(valid, `"declaredType":""`, `"declaredType":"","declaredType":"TEXT"`, 1),
		strings.Replace(valid, `"rowId":"-7"`, `"rowId":"-7","rowId":"2"`, 1),
		strings.Replace(valid, `"columns":[`, `"columns":null,"unknown":[`, 1),
		strings.Replace(valid, `"integer":"0"`, `"integer":"0","integer":"1"`, 1),
		strings.Replace(valid, `"text":"108050"`, `"text":"\ud800"`, 1),
		strings.Replace(valid, soilWriteUUID, "not-a-uuid", 1),
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatalf("malformed/duplicate complete request accepted: %q", invalid)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, mutate := range []func(*twoPageSoilRequest){
		func(r *twoPageSoilRequest) { r.RequestID = "invalid" },
		func(r *twoPageSoilRequest) { r.Original.Records.Columns[0].DeclaredType = string([]byte{0xff}) },
		func(r *twoPageSoilRequest) { r.Edits[0].Value = metadataText(string([]byte{0xff})) },
		func(r *twoPageSoilRequest) {
			r.Original.ContextID += "-stale"
			for i := range r.Edits {
				r.Edits[i].ContextID = r.Original.ContextID
			}
		},
		func(r *twoPageSoilRequest) {
			r.Edits = []twoPageSoilEdit{soilPlanEdit(r.Original, "Horizon", metadataText("H"))}
		},
	} {
		var input twoPageSoilRequest
		if err := json.Unmarshal(raw, &input); err != nil {
			t.Fatal(err)
		}
		mutate(&input)
		if result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, input); err == nil || result != nil {
			t.Fatal("invalid/direct-Go/noop input returned committed receipt", result, err)
		}
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("invalid raw/direct-Go inputs changed source")
	}
}
