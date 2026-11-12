package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageVegetationWriteRejectsSourceCover100WithoutHistory(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		t.Run(form, func(t *testing.T) {
			service, state, db, request := vegetationWriteFixture(t, false, form, 3)
			valid := vprocore.CloneSiteUnitCell(request.Edits[0].Value)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			for _, invalid := range []float64{99.999999, math.Nextafter(100, math.Inf(-1)), 100, 101} {
				request.Edits[0].Value = vprotest.SiviReal(invalid)
				result, err := WriteTwoPageVegetation(service, context.Background(), state.ContextID, request)
				if err == nil || result != nil || !strings.Contains(err.Error(), "below 100 or NULL") ||
					!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatalf("source >=100 refusal committed data/audits/technical history: %+v %v", result, err)
				}
			}
			for _, invalid := range []float64{0.1, 1e-46, -1e-46} {
				request.Edits[0].Value = vprotest.SiviReal(invalid)
				result, err := WriteTwoPageVegetation(service, context.Background(), state.ContextID, request)
				if err == nil || result != nil ||
					!(strings.Contains(err.Error(), "canonical source SINGLE") || strings.Contains(err.Error(), "negative-zero")) ||
					!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatalf("noncanonical/negative-underflow transport changed storage: %+v %v", result, err)
				}
			}
			priorAudits := soilWriteCount(t, db, "Sample_Audit")
			request.Edits[0].Value = valid
			result, err := WriteTwoPageVegetation(service, context.Background(), state.ContextID, request)
			if err != nil || result == nil || !result.DidCommit || result.Replayed ||
				soilWriteCount(t, db, "Sample_Audit") != priorAudits+2 ||
				soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
				t.Fatalf("private uncommitted request correction did not commit exactly once: %+v %v", result, err)
			}

			before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			replay, err := WriteTwoPageVegetation(service, context.Background(), state.ContextID, request)
			if err != nil || replay == nil || replay.DidCommit || !replay.Replayed ||
				!reflect.DeepEqual(replay.History, result.History) ||
				!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
				t.Fatalf("corrected request exact replay changed storage/provenance: %+v %v", replay, err)
			}
		})
	}
}

func TestTwoPageVegetationWritePreservesHistoricalCoverBoundsInReceipt(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		for _, historical := range []float64{0.1, 99.999999, 1e-46, -1e-46, 100, 101} {
			t.Run(fmt.Sprintf("%s/%g", form, historical), func(t *testing.T) {
				service, state, db, request := vegetationWriteFixture(t, false, form, 3)
				column := request.Edits[0].Column
				if _, err := db.Exec("UPDATE Sample_Veg SET "+vprocore.QuoteHeaderIdentifier(column)+"=? WHERE rowid=-7", historical); err != nil {
					t.Fatal(err)
				}
				original, err := ReadTwoPageVegetationOriginal(service, context.Background(), state.ContextID, "108050", form)
				if err != nil {
					t.Fatal(err)
				}
				request.Original = original
				request.Edits = []twoPageVegetationEdit{
					vegetationPlanEdit(original, column, vprotest.SiviReal(historical)),
					vegetationPlanEdit(original, "Collected", vprotest.MetadataText("C")),
				}
				result, err := WriteTwoPageVegetation(service, context.Background(), state.ContextID, request)
				if err != nil || result == nil || !result.DidCommit || len(result.History.Audits) != 1 ||
					!twoPageVegetationCellEqual(*vegetationPlanCell(
						&twoPageVegetationOriginal{Records: result.History.Committed}, "-7", column), vprotest.SiviReal(historical)) {
					t.Fatalf("unchanged historical cover was rejected/rewritten or audited: %+v %v", result, err)
				}
				before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				receipt, err := LookupTwoPageVegetationReceipt(service, context.Background(), state.ContextID, request)
				if err != nil || receipt == nil || !receipt.Replayed ||
					!reflect.DeepEqual(receipt.History, result.History) ||
					!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatalf("unchanged out-of-rule cover invalidated readonly receipt: %+v %v", receipt, err)
				}
			})
		}
	}
}
