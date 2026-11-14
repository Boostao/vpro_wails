package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func vegetationFacade(t *testing.T, contexts *vprocore.ContextService) *TwoPageVegetationService {
	t.Helper()
	service, err := NewTwoPageVegetationService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func vegetationAttemptJSON(t *testing.T, request twoPageVegetationRequest) string {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestTwoPageVegetationServiceCommitAndReadonlyRecovery(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		t.Run(form, func(t *testing.T) {
			contexts, state, db, request := vegetationWriteFixture(t, false, form, 3)
			raw := vegetationAttemptJSON(t, request)
			before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
			readonly := vegetationFacade(t, contexts)
			if result, err := readonly.GetOriginal(context.Background(), state.ContextID, "108050", form); result == nil || err != nil {
				t.Fatal("physical original refused", result, err)
			}
			if result, err := readonly.GetSpeciesReferences(context.Background(), state.ContextID, "108050", form); result == nil || err != nil {
				t.Fatal("physical Species definitions refused", result, err)
			}
			if result, err := readonly.RecoverReceipt(context.Background(), state.ContextID, raw); result != nil || err != nil {
				t.Fatal("missing history is not unresolved nil", result, err)
			}
			if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
				t.Fatal("readonly reads created source/schema/history")
			}
			enabled := vegetationFacade(t, contexts)
			priorAudits := soilWriteCount(t, db, "Sample_Audit")
			result, err := enabled.Save(context.Background(), state.ContextID, raw)
			if err != nil || result == nil || !result.DidCommit || result.Replayed ||
				soilWriteCount(t, db, "Sample_Audit") != priorAudits+2 || soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
				t.Fatal("facade did not retain exact atomic writer result", result, err)
			}
			before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
			recovered, err := readonly.RecoverReceipt(context.Background(), state.ContextID, raw)
			if err != nil || recovered == nil || recovered.DidCommit || !recovered.Replayed ||
				!reflect.DeepEqual(recovered.History, result.History) ||
				!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
				t.Fatal("readonly recovery lost provenance or wrote data", recovered, err)
			}
		})
	}
}

func TestTwoPageVegetationServiceStrictTransportAndExplicitErrors(t *testing.T) {
	contexts, state, _, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
	if result, err := NewTwoPageVegetationService(nil); result != nil || err == nil {
		t.Fatal("nil owner accepted", result, err)
	}
	service := vegetationFacade(t, contexts)
	raw := vegetationAttemptJSON(t, request)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, invalid := range []string{
		"", "null", `{}`, strings.Replace(raw, `"requestId":`, `"unexpected":`, 1),
		strings.Replace(raw, `"requestId":`, `"requestId":"b0cbfa00-1234-4123-8123-000000000009","requestId":`, 1),
		strings.Replace(raw, `"requestId":`, `"\u0072equestId":"b0cbfa00-1234-4123-8123-000000000009","requestId":`, 1),
	} {
		if result, err := service.Save(context.Background(), state.ContextID, invalid); result != nil || err == nil {
			t.Fatal("malformed/duplicate/escaped transport granted writer", result, err)
		}
		if result, err := service.RecoverOutcome(context.Background(), state.ContextID, invalid); result != nil || err == nil {
			t.Fatal("malformed transport granted certificate", result, err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, cancelled} {
		if result, err := service.Save(ctx, state.ContextID, raw); result != nil || err == nil ||
			ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("nil/cancelled facade writer granted", result, err)
		}
		if result, err := service.RecoverOutcome(ctx, state.ContextID, raw); result != nil || err == nil {
			t.Fatal("nil/cancelled facade recovery granted", result, err)
		}
	}
	for _, facade := range []*TwoPageVegetationService{nil, {}} {
		if result, err := facade.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
			t.Fatal("unowned facade accepted writer", result, err)
		}
	}
	if len(contexts.TwoPageVegetationAttempts.Requests) != 0 ||
		!reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("strict/pre-cancel denial created an attempt or changed physical data")
	}
}

func TestTwoPageVegetationServiceActualMainRegistration(t *testing.T) {
	mainData, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	plotEntryData, err := os.ReadFile("domainservices_plotentry.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"buildPlotEntryService(contextService, plots, refs)",
		"application.NewService(plotEntryService)",
	} {
		if strings.Count(string(mainData), required) != 1 {
			t.Fatal("actual main registration missing/duplicated", required)
		}
	}
	for _, required := range []string{
		"twopagepkg.NewTwoPageVegetationService(core)",
		"twoPageVegetation:            twoPageVegetation",
	} {
		if strings.Count(string(plotEntryData), required) != 1 {
			t.Fatal("actual PlotEntryService builder registration missing/duplicated", required)
		}
	}
}
