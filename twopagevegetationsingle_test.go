package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTwoPageVegetationSingleCalibrationTypedAssignments(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationSingle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var calibration struct {
		Cases []struct {
			Raw            string
			NativeAccepted bool
			NativeValue    *float64
			SingleHex      *string
			ClientAccepted bool
		}
	}
	if err := json.Unmarshal(raw, &calibration); err != nil || len(calibration.Cases) != 8 {
		t.Fatal("source SINGLE calibration must retain all eight observed cases", err)
	}
	for form, fields := range map[string][]string{
		"SubVegAXL_BC": {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"},
		"SubVegAXL":    {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"},
		"SubVegCXL":    {"Cover6"},
		"SubVegDXL":    {"Cover7", "Cover8", "Cover9"},
	} {
		original := twoPageVegetationPlanFixture(t, form)
		for _, column := range fields {
			if column == "Collected" {
				continue
			}
			for _, observed := range calibration.Cases {
				t.Run(form+"/"+column+"/"+observed.Raw, func(t *testing.T) {
					*vegetationPlanCell(original, "-7", column) = siviReal(1)
					value, err := strconv.ParseFloat(observed.Raw, 64)
					if err != nil {
						t.Fatal(err)
					}
					edit := vegetationPlanEdit(original, column, siviReal(value))
					plan, err := planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{edit})
					rawCanonical := observed.ClientAccepted && value == *observed.NativeValue
					if (err == nil) != rawCanonical || err != nil && plan != nil {
						t.Fatalf("raw input is not canonical typed transport: %+v %v", plan, err)
					}
					if observed.NativeAccepted {
						if observed.NativeValue == nil || observed.SingleHex == nil {
							t.Fatal("accepted source case lacks independently observed storage")
						}
						bits := make([]byte, 4)
						binary.LittleEndian.PutUint32(bits, math.Float32bits(float32(*observed.NativeValue)))
						if hex.EncodeToString(bits) != *observed.SingleHex {
							t.Fatal("native scalar and independently observed SINGLE bytes disagree")
						}
						edit.Value = siviReal(*observed.NativeValue)
						plan, err = planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{edit})
						if (err == nil) != observed.ClientAccepted || err != nil && plan != nil {
							t.Fatalf("canonical source assignment / SQLite signed-zero adaptation: %+v %v", plan, err)
						}
						if observed.ClientAccepted && (len(plan) != 1 || !twoPageVegetationCellEqual(plan[0].After, edit.Value)) {
							t.Fatal("planner silently altered canonical typed transport", plan)
						}
					}
				})
			}
		}
	}
}

func TestTwoPageVegetationWriteCanonicalSingleStorage(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		for _, canonical := range []float64{0.10000000149011612, 99.99999237060547, 0} {
			t.Run(fmt.Sprintf("%s/%g", form, canonical), func(t *testing.T) {
				service, state, db, request := vegetationWriteFixture(t, false, form, 3)
				column := request.Edits[0].Column
				if _, err := db.Exec("UPDATE Sample_Veg SET " + quoteHeaderIdentifier(column) + "=1 WHERE rowid=-7"); err != nil {
					t.Fatal(err)
				}
				var err error
				request.Original, err = service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
				if err != nil {
					t.Fatal(err)
				}
				request.Edits = []twoPageVegetationEdit{vegetationPlanEdit(request.Original, column, siviReal(canonical))}
				priorAudits := soilWriteCount(t, db, "Sample_Audit")
				result, err := service.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil || result == nil || !result.DidCommit ||
					soilWriteCount(t, db, "Sample_Audit") != priorAudits+1 ||
					soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
					t.Fatalf("canonical SINGLE did not commit exactly one audited field: %+v %v", result, err)
				}
				reopened, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
				if err != nil || !twoPageVegetationCellEqual(*vegetationPlanCell(reopened, "-7", column), siviReal(canonical)) ||
					!twoPageVegetationTableEqual(reopened.Records, result.History.Committed) {
					t.Fatalf("reopened physical REAL bits differ from exact canonical request: %+v %v", reopened, err)
				}
				for _, rowID := range []string{"3", "8"} {
					for _, peerColumn := range request.Original.Records.Columns {
						if !twoPageVegetationCellEqual(*vegetationPlanCell(reopened, rowID, peerColumn.Name),
							*vegetationPlanCell(request.Original, rowID, peerColumn.Name)) {
							t.Fatalf("canonical write changed peer %s.%s", rowID, peerColumn.Name)
						}
					}
				}
			})
		}
	}
}

func TestTwoPageVegetationLegacySingleReceiptRecovery(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		for _, legacy := range []float64{0.1, 99.999999, 1e-46, -1e-46} {
			t.Run(fmt.Sprintf("%s/%g", form, legacy), func(t *testing.T) {
				contexts, state, db, request := vegetationWriteFixture(t, false, form, 3)
				result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil {
					t.Fatal(err)
				}
				history := result.History
				column := request.Edits[0].Column
				history.Request.Edits[0].Value = siviReal(legacy)
				*vegetationPlanCell(&twoPageVegetationOriginal{Records: history.Committed}, "-7", column) = siviReal(legacy)
				auditValue := fmt.Sprint(legacy)
				history.Audits[0].AfterEdit = &auditValue
				if err := validateTwoPageVegetationHistory(context.Background(), history); err != nil {
					t.Fatal("precalibration history was reinterpreted with new assignment rules", err)
				}
				proposal, err := json.Marshal(history)
				if err != nil {
					t.Fatal(err)
				}
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = tx.Rollback() })
				for _, update := range []struct {
					query string
					args  []any
				}{
					{"UPDATE Sample_Veg SET " + quoteHeaderIdentifier(column) + "=? WHERE rowid=-7", []any{legacy}},
					{"UPDATE Sample_Audit SET AfterEdit=? WHERE rowid=?", []any{auditValue, history.Audits[0].RowID}},
					{"UPDATE " + quoteHeaderIdentifier(twoPageVegetationHistoryTable) + " SET Proposal=? WHERE RequestID=?", []any{string(proposal), request.RequestID}},
				} {
					if _, err := tx.Exec(update.query, update.args...); err != nil {
						t.Fatal(err)
					}
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
				request = history.Request
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
				raw := vegetationAttemptJSON(t, request)
				outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
				if err != nil || outcome == nil || outcome.Status != "committed" || outcome.RequestJSON != raw ||
					outcome.Receipt == nil || !reflect.DeepEqual(outcome.Receipt.History, history) ||
					!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
					t.Fatalf("legacy exact request/audit/peer recovery changed or became unavailable: %+v %v", outcome, err)
				}
				replay, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil || replay == nil || replay.DidCommit || !replay.Replayed ||
					!reflect.DeepEqual(replay.History, history) ||
					!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
					t.Fatalf("legacy committed UUID must replay before new-assignment validation: %+v %v", replay, err)
				}
				request.RequestID = "b0cbfa00-1234-4123-8123-000000000002"
				request.Original, err = contexts.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
				if err != nil {
					t.Fatal(err)
				}
				request.Edits = []twoPageVegetationEdit{vegetationPlanEdit(request.Original, column, siviReal(0.2))}
				if result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request); err == nil || result != nil ||
					!strings.Contains(err.Error(), "canonical source SINGLE") ||
					!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
					t.Fatalf("historical decoding must not authorize a new noncanonical write: %+v %v", result, err)
				}
			})
		}
	}
}
