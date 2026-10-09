package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func vegetationFacade(t *testing.T, contexts *ContextService, values map[string]string) *TwoPageVegetationService {
	t.Helper()
	service, err := NewTwoPageVegetationService(contexts, func(name string) (string, bool) {
		if name != twoPageVegetationReviewEnvironment && name != twoPageVegetationWritingEnvironment {
			t.Fatal("vegetation facade consulted another workflow's gate", name)
		}
		value, found := values[name]
		return value, found
	})
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

func TestTwoPageVegetationServiceIndependentGatesAndReadonlyRecovery(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
		t.Run(form, func(t *testing.T) {
			contexts, state, db, request := vegetationWriteFixture(t, false, form, 3)
			raw := vegetationAttemptJSON(t, request)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			for _, values := range []map[string]string{
				{}, {twoPageVegetationReviewEnvironment: "false"}, {twoPageVegetationWritingEnvironment: "true"},
				{twoPageVegetationReviewEnvironment: "false", twoPageVegetationWritingEnvironment: "true"},
			} {
				facade := vegetationFacade(t, contexts, values)
				if result, err := facade.GetOriginal(context.Background(), state.ContextID, "108050", form); result != nil || err == nil {
					t.Fatal("disabled physical original granted", result, err)
				}
				if result, err := facade.GetSpeciesReferences(context.Background(), state.ContextID, "108050", form); result != nil || err == nil {
					t.Fatal("disabled Species references granted", result, err)
				}
				if result, err := facade.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
					t.Fatal("disabled writer granted", result, err)
				}
				if result, err := facade.RecoverOutcome(context.Background(), state.ContextID, raw); result != nil || err == nil {
					t.Fatal("disabled recovery granted", result, err)
				}
			}
			readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
			if result, err := readonly.GetOriginal(context.Background(), state.ContextID, "108050", form); result == nil || err != nil {
				t.Fatal("review-only physical original refused", result, err)
			}
			if result, err := readonly.GetSpeciesReferences(context.Background(), state.ContextID, "108050", form); result == nil || err != nil {
				t.Fatal("review-only physical Species definitions refused", result, err)
			}
			if result, err := readonly.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
				t.Fatal("review-only grant allowed mutation", result, err)
			}
			if result, err := readonly.RecoverReceipt(context.Background(), state.ContextID, raw); result != nil || err != nil {
				t.Fatal("missing history is not unresolved nil", result, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("flags/readonly reads created source/schema/history")
			}
			enabled := vegetationFacade(t, contexts, map[string]string{
				twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
			})
			priorAudits := soilWriteCount(t, db, "Sample_Audit")
			result, err := enabled.Save(context.Background(), state.ContextID, raw)
			if err != nil || result == nil || !result.DidCommit || result.Replayed ||
				soilWriteCount(t, db, "Sample_Audit") != priorAudits+2 || soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
				t.Fatal("facade did not retain exact atomic writer result", result, err)
			}
			before = databaseBytes(t, contexts.projects.sqlite.attachments)
			recovered, err := readonly.RecoverReceipt(context.Background(), state.ContextID, raw)
			if err != nil || recovered == nil || recovered.DidCommit || !recovered.Replayed ||
				!reflect.DeepEqual(recovered.History, result.History) ||
				!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("writing-disabled recovery lost provenance or wrote data", recovered, err)
			}
		})
	}
}

func TestTwoPageVegetationServiceStrictTransportAndExplicitErrors(t *testing.T) {
	contexts, state, _, request := vegetationWriteFixture(t, false, "SubVegAXL_BC", 3)
	lookup := func(string) (string, bool) { return "", false }
	if result, err := NewTwoPageVegetationService(nil, lookup); result != nil || err == nil {
		t.Fatal("nil owner accepted", result, err)
	}
	if result, err := NewTwoPageVegetationService(contexts, nil); result != nil || err == nil {
		t.Fatal("nil gate lookup accepted", result, err)
	}
	for _, name := range []string{twoPageVegetationReviewEnvironment, twoPageVegetationWritingEnvironment} {
		if result, err := NewTwoPageVegetationService(contexts, func(key string) (string, bool) {
			return "invalid", key == name
		}); result != nil || err == nil {
			t.Fatal("invalid feature value silently accepted", name, result, err)
		}
	}
	service := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	raw := vegetationAttemptJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
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
	if len(contexts.twoPageVegetationAttempts.requests) != 0 ||
		!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("strict/pre-cancel denial created an attempt or changed physical data")
	}
}

func TestTwoPageVegetationServiceActualMainRegistration(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"NewTwoPageVegetationService(contextService, os.LookupEnv)",
		"application.NewService(twoPageVegetation)",
	} {
		if strings.Count(string(data), required) != 1 {
			t.Fatal("actual main registration missing/duplicated", required)
		}
	}
}
