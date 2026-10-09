package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func soilFacade(t *testing.T, contexts *ContextService, values map[string]string) *TwoPageSoilService {
	t.Helper()
	service, err := NewTwoPageSoilService(contexts, func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestTwoPageSoilServiceIndependentGatesAndNoWrites(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range []map[string]string{
		{},
		{twoPageSoilReviewEnvironment: "false", twoPageSoilWritingEnvironment: "false"},
		{twoPageSoilWritingEnvironment: "true"},
		{twoPageSoilReviewEnvironment: "false", twoPageSoilWritingEnvironment: "true"},
	} {
		service := soilFacade(t, contexts, values)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		if original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || original != nil {
			t.Fatal("disabled review exposed originals", original, err)
		}
		if references, err := service.GetReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err == nil || references != nil {
			t.Fatal("disabled review exposed source references", references, err)
		}
		if result, err := service.Save(context.Background(), state.ContextID, string(raw)); err == nil || result != nil {
			t.Fatal("writing alone bypassed review ownership", result, err)
		}
		if result, err := service.RecoverReceipt(context.Background(), state.ContextID, string(raw)); err == nil || result != nil {
			t.Fatal("disabled review exposed receipt authority", result, err)
		}
		if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
			t.Fatal("default-off gate checks changed source/schema")
		}
	}
	review := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
	if original, err := review.GetOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err != nil ||
		!reflect.DeepEqual(original, request.Original) {
		t.Fatal("independent read-only review changed raw original", original, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if references, err := review.GetReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err != nil ||
		references == nil || len(references.Fields) != 6 {
		t.Fatal("read-only review did not return source references", references, err)
	}
	if result, err := review.Save(context.Background(), state.ContextID, string(raw)); err == nil || result != nil {
		t.Fatal("read-only review granted mutation", result, err)
	}
	if result, err := review.RecoverReceipt(context.Background(), state.ContextID, string(raw)); err != nil || result != nil {
		t.Fatal("uncommitted read-only recovery invented receipt", result, err)
	}
	if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("read-only facade created data or history")
	}
}

func TestTwoPageSoilServiceActualSaveAndDisabledWritingRecovery(t *testing.T) {
	contexts, state, db, request := soilWriteFixture(t, false, "SoilMineralXL", 1)
	enabled := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	original, err := enabled.GetOriginal(context.Background(), state.ContextID, "108050", "SoilMineralXL")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wire), `"declaredType":`) || strings.Contains(string(wire), `"type":`) ||
		!strings.Contains(string(wire), `"storage":"blob"`) || !strings.Contains(string(wire), `"storage":"null"`) {
		t.Fatal("production original wire lost metadata names or raw tagged cells", string(wire))
	}
	request.Original = original
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := enabled.Save(context.Background(), state.ContextID, string(raw))
	if err != nil || accepted == nil || !accepted.DidCommit || len(accepted.History.Audits) != 1 ||
		soilWriteCount(t, db, twoPageSoilHistoryTable) != 1 {
		t.Fatal("facade did not preserve mixed audited/unaudited commit", accepted, err)
	}
	wire, err = json.Marshal(accepted)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip TwoPageSoilWriteResult
	if err := json.Unmarshal(wire, &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, *accepted) {
		t.Fatal("exported strict result does not round-trip real production wire", err)
	}
	readonly := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "false"})
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	recovered, err := readonly.RecoverReceipt(context.Background(), state.ContextID, string(raw))
	if err != nil || recovered == nil || recovered.DidCommit || !recovered.Replayed || !reflect.DeepEqual(recovered.History, accepted.History) {
		t.Fatal("turning writing off erased read-only irreversible recovery", recovered, err)
	}
	if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("read-only recovery replayed mutation")
	}
}

func TestTwoPageSoilServiceExplicitErrorsAndStrictTransport(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	lookup := func(string) (string, bool) { return "", false }
	if service, err := NewTwoPageSoilService(nil, lookup); err == nil || service != nil {
		t.Fatal("nil owned contexts accepted")
	}
	if service, err := NewTwoPageSoilService(contexts, nil); err == nil || service != nil {
		t.Fatal("implicit feature lookup accepted")
	}
	for _, name := range []string{twoPageSoilReviewEnvironment, twoPageSoilWritingEnvironment} {
		for _, invalid := range []string{"", "1", "True", " true", "false ", "yes"} {
			if service, err := NewTwoPageSoilService(contexts, func(key string) (string, bool) {
				return invalid, key == name
			}); err == nil || service != nil {
				t.Fatalf("malformed feature %s=%q accepted", name, invalid)
			}
		}
	}
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true", twoPageSoilWritingEnvironment: "true"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.Save(ctx, state.ContextID, "invalid JSON"); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled context was replaced by transport failure", result, err)
	}
	for _, facade := range []*TwoPageSoilService{nil, {}, service} {
		if result, err := facade.Save(nil, state.ContextID, "invalid JSON"); err == nil || result != nil {
			t.Fatal("nil context accepted")
		}
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, scope := range []struct{ contextID, plot, form string }{
		{state.ContextID + "-stale", "108050", "SoilHumusXL"},
		{state.ContextID, "108050 ", "SoilHumusXL"},
		{state.ContextID, "108050", "SoilHumusXL "},
	} {
		if original, err := service.GetOriginal(context.Background(), scope.contextID, scope.plot, scope.form); err == nil || original != nil {
			t.Fatal("invalid facade original scope accepted", original, err)
		}
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Replace(string(raw), `"requestId":`, `"unknown":`, 1),
		strings.Replace(string(raw), `"project":"Sample"`, `"project":"Sample","\u0070roject":"Other"`, 1),
		strings.Replace(string(raw), `"text":"108050"`, `"text":"\ud800"`, 1),
		strings.Replace(string(raw), `"rowId":"-7"`, `"rowId":"-7","rowId":"2"`, 1),
		`null`, `{}`, `"not a request"`,
	} {
		for _, recover := range []bool{false, true} {
			var result *TwoPageSoilWriteResult
			var err error
			if recover {
				result, err = service.RecoverReceipt(context.Background(), state.ContextID, invalid)
			} else {
				result, err = service.Save(context.Background(), state.ContextID, invalid)
			}
			if err == nil || result != nil {
				t.Fatal("strict facade request repaired or silently accepted", result, err)
			}
		}
	}
	if after := databaseBytes(t, contexts.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("invalid facade guards/transport changed source/audit/history")
	}
}
