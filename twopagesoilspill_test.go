package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageSoilWriteLargeMemoCacheSpill(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			t.Run(fmt.Sprintf("%s/external-%v", form, external), func(t *testing.T) {
				service, state, db, request := soilWriteFixture(t, external, form, 3)
				column := "Comment"
				if form == "SoilMineralXL" {
					column = "Comments"
				}
				memo := strings.Repeat("\U0001f600", 270000)
				request.Edits = []twoPageSoilEdit{soilPlanEdit(request.Original, column, metadataText(memo))}
				beforeAudits := soilWriteCount(t, db, "Sample_Audit")
				result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
				if err != nil || result == nil || !result.DidCommit || len(result.History.Audits) != 1 {
					t.Fatalf("large source Memo/audit/history must commit without reading behind its own spill lock: %v", err)
				}
				if soilWriteCount(t, db, "Sample_Audit") != beforeAudits+1 ||
					soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
					t.Fatal("large Memo committed extra or missing provenance")
				}
				reopened, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, request.Original.Plot, form)
				if err != nil || !reflect.DeepEqual(reopened.Records, result.History.Committed) {
					t.Fatal("large committed physical original differs", err)
				}
				if recovered, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request); err != nil ||
					recovered == nil || !recovered.Replayed || !reflect.DeepEqual(recovered.History, result.History) {
					t.Fatal("large readonly recovery lost exact committed history", err)
				}
			})
		}
	}
}

func TestTwoPageSoilWriteFinalScopeUsesUncommittedParentRows(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, scope := range []string{"Env", "Admin", "SU"} {
			if external && scope == "SU" {
				continue
			}
			t.Run(fmt.Sprintf("%s/external-%v", scope, external), func(t *testing.T) {
				service, state, db, request := soilWriteFixture(t, external, "SoilHumusXL", 3)
				priorAudits := soilWriteCount(t, db, "Sample_Audit")
				query := `DELETE FROM "Sample_Env" WHERE PlotNumber='108050'`
				if scope == "Admin" {
					query = `DELETE FROM "Sample_Admin" WHERE Plot='108050'`
				} else if scope == "SU" {
					query = `DELETE FROM "Report_SU" WHERE PlotNumber='108050'`
				}
				result, err := service.writeTwoPageSoilObserved(context.Background(), state.ContextID, request,
					func(tx *sql.Tx) error {
						_, err := tx.Exec(query)
						return err
					}, nil)
				if err == nil || result != nil || !strings.Contains(err.Error(), "outside the selected context") {
					t.Fatal("final writer scope ignored its own uncommitted parent change", result, err)
				}
				original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, request.Original.Plot, request.Original.Form)
				if err != nil || !reflect.DeepEqual(original, request.Original) ||
					soilWriteCount(t, db, "Sample_Audit") != priorAudits {
					t.Fatal("rejected final parent change did not roll back soil/parent/audits", err)
				}
			})
		}
	}
}

func TestTwoPageSoilWriteExternalSUReservedThroughCommit(t *testing.T) {
	service, state, _, request := soilWriteFixture(t, true, "SoilMineralXL", 3)
	su, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["su"], "rw")+"&_busy_timeout=20")
	if err != nil {
		t.Fatal(err)
	}
	defer su.Close()
	result, err := service.writeTwoPageSoilObserved(context.Background(), state.ContextID, request,
		func(tx *sql.Tx) error {
			_, err := su.Exec(`DELETE FROM "Report_SU" WHERE PlotNumber='108050'`)
			if err == nil || !strings.Contains(err.Error(), "locked") {
				return fmt.Errorf("independent SU removal escaped through-commit read reservation: %v", err)
			}
			return nil
		}, nil)
	if err != nil || result == nil || !result.DidCommit {
		t.Fatal("reserved external SU soil write failed", result, err)
	}
	var count int
	if err := su.QueryRow(`SELECT COUNT(*) FROM "Report_SU" WHERE PlotNumber='108050'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("independent writer removed selected SU membership", count, err)
	}
}

func TestTwoPageSoilWriteExternalSUWALRefusalAndCorrection(t *testing.T) {
	service, state, db, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	su, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["su"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer su.Close()
	var mode string
	if err := su.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("external SU WAL fixture failed", mode, err)
	}
	priorAudits := soilWriteCount(t, db, "Sample_Audit")
	result, err := service.writeTwoPageSoil(context.Background(), state.ContextID, request)
	if err == nil || result != nil || !strings.Contains(err.Error(), "two-page soil selected SU observations require rollback-journal") {
		t.Fatal("WAL external SU was mistaken for stable through-commit membership", result, err)
	}
	if err := su.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatal("external SU journal correction failed", mode, err)
	}
	original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, request.Original.Plot, request.Original.Form)
	if err != nil || !reflect.DeepEqual(original, request.Original) || soilWriteCount(t, db, "Sample_Audit") != priorAudits {
		t.Fatal("external SU refusal changed source/audits", err)
	}
	result, err = service.writeTwoPageSoil(context.Background(), state.ContextID, request)
	if err != nil || result == nil || !result.DidCommit {
		t.Fatal("explicit corrected source journal could not retry", result, err)
	}
}

func TestTwoPageSoilWriteLargeMemoCancellationRollsBackAndRetries(t *testing.T) {
	service, state, db, request := soilWriteFixture(t, true, "SoilMineralXL", 3)
	request.Edits = []twoPageSoilEdit{soilPlanEdit(request.Original, "Comments", metadataText(strings.Repeat("large memo", 150000)))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	priorAudits := soilWriteCount(t, db, "Sample_Audit")
	result, err := service.writeTwoPageSoilObserved(ctx, state.ContextID, request, func(tx *sql.Tx) error {
		cancel()
		return nil
	}, nil)
	if !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled spilled transaction committed", result, err)
	}
	original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, request.Original.Plot, request.Original.Form)
	if err != nil || !reflect.DeepEqual(original, request.Original) || soilWriteCount(t, db, "Sample_Audit") != priorAudits {
		t.Fatal("cancelled spilled source/audits were not restored", err)
	}
	recovered, err := service.lookupTwoPageSoilReceipt(context.Background(), state.ContextID, request)
	if err != nil || recovered != nil {
		t.Fatal("cancelled spilled history survived rollback", recovered, err)
	}
	result, err = service.writeTwoPageSoil(context.Background(), state.ContextID, request)
	if err != nil || result == nil || !result.DidCommit || soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
		t.Fatal("explicit uncommitted large request retry failed", result, err)
	}
}
