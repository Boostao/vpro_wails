package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func soilFacade(t *testing.T, contexts *vprocore.ContextService) *TwoPageSoilService {
	t.Helper()
	service, err := NewTwoPageSoilService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestTwoPageSoilServiceReadOnlyOriginalAndReferencesNoWrites(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, true, "SoilHumusXL", 3)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	review := soilFacade(t, contexts)
	if original, err := review.GetOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err != nil ||
		!reflect.DeepEqual(original, request.Original) {
		t.Fatal("read-only review changed raw original", original, err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	if references, err := review.GetReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); err != nil ||
		references == nil || len(references.Fields) != 6 {
		t.Fatal("read-only review did not return source references", references, err)
	}
	if result, err := review.RecoverReceipt(context.Background(), state.ContextID, string(raw)); err != nil || result != nil {
		t.Fatal("uncommitted read-only recovery invented receipt", result, err)
	}
	if after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("read-only facade created data or history")
	}
}

func TestTwoPageSoilServiceActualSaveAndReadOnlyRecovery(t *testing.T) {
	contexts, state, db, request := soilWriteFixture(t, false, "SoilMineralXL", 1)
	enabled := soilFacade(t, contexts)
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
	readonly := soilFacade(t, contexts)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	recovered, err := readonly.RecoverReceipt(context.Background(), state.ContextID, string(raw))
	if err != nil || recovered == nil || recovered.DidCommit || !recovered.Replayed || !reflect.DeepEqual(recovered.History, accepted.History) {
		t.Fatal("read-only irreversible recovery failed", recovered, err)
	}
	if after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("read-only recovery replayed mutation")
	}
}

func TestTwoPageSoilServiceExplicitErrorsAndStrictTransport(t *testing.T) {
	contexts, state, _, request := soilWriteFixture(t, false, "SoilHumusXL", 3)
	if service, err := NewTwoPageSoilService(nil); err == nil || service != nil {
		t.Fatal("nil owned contexts accepted")
	}
	service := soilFacade(t, contexts)
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
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
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
	if after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("invalid facade guards/transport changed source/audit/history")
	}
}
