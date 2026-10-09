package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIIdentityServiceIndependentGateAndUnavailableContext(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", " true", "1"} {
		service, err := NewSIVIIdentityService(&ContextService{}, func(name string) (string, bool) {
			if name != siviIdentityFeatureEnvironment {
				t.Fatal("identity borrowed predecessor editing authority", name)
			}
			return flag, flag != ""
		})
		if flag != "" && flag != "false" && flag != "true" {
			if err == nil || service != nil {
				t.Fatal("malformed identity gate silently defaulted", service, err)
			}
			continue
		}
		if err != nil || service.enabled != (flag == "true") {
			t.Fatal("identity flag is not independently default-off", service, err)
		}
		if flag != "true" {
			if result, err := service.GetOriginal(context.Background(), "owned", "P", false); result != nil || err == nil {
				t.Fatal("disabled identity source read succeeded", result, err)
			}
			if result, err := service.SaveReviewed(context.Background(), "owned", "P", false, `{}`); result != nil || err == nil {
				t.Fatal("disabled identity save succeeded", result, err)
			}
			if result, err := service.RestoreReviewed(context.Background(), "owned", "P", "1", AuditRestoreCancel); result != nil || err == nil {
				t.Fatal("disabled identity restoration cancel borrowed authority", result, err)
			}
		}
	}
	if service, err := NewSIVIIdentityService(nil, func(string) (string, bool) { return "true", true }); service != nil || err == nil {
		t.Fatal("identity constructed without context ownership", service, err)
	}
	if service, err := NewSIVIIdentityService(&ContextService{}, nil); service != nil || err == nil {
		t.Fatal("identity constructed without explicit lookup", service, err)
	}
	var missing *SIVIIdentityService
	if result, err := missing.GetOriginal(context.Background(), "owned", "P", false); result != nil || err == nil {
		t.Fatal("nil service produced an original", result, err)
	}
	if err := missing.require(nil); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := missing.require(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation was replaced by a success-shaped availability result", err)
	}
}

func TestSIVIIdentityServiceStrictWireSaveAndRestoration(t *testing.T) {
	contexts, state, db, original, heights := siviWriteFixture(t, false, 0)
	service, err := NewSIVIIdentityService(contexts, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	wire, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(wire, original) {
		t.Fatal("facade lost actual source originals", wire, err)
	}
	request := SIVIIdentityWrite{Original: original, Edits: []SIVIHeightEdit{
		{heights[0].RowID, "SubVegA-SIVI_BC", "ID", metadataInteger("10000001"), metadataInteger("-1")},
	}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{
		`{}`, `{"original":null,"edits":[]}`, `{"original":[],"edits":null}`,
		strings.Replace(string(data), `"edits":`, `"unknown":true,"edits":`, 1),
		strings.Replace(string(data), `"column":"ID"`, `"column":"Species"`, 1),
		strings.Replace(string(data), `"form":"SubVegA-SIVI_BC"`, `"form":"\ud800"`, 1),
		strings.Replace(string(data), `"value":`, `"value":null,"value":`, 1),
		strings.Replace(string(data), `"storage":`, `"storage":"null","storage":`, 1),
		strings.Replace(string(data), `"storage":`, `"Storage":`, 1),
		strings.Replace(string(data), `"Form":`, `"Form":"SubVegD-SIVI","Form":`, 1),
		strings.Replace(string(data), `"rowId":`, `"rowId":"1","rowId":`, 1),
		strings.Replace(string(data), `"cells":`, `"unknown":true,"cells":`, 1),
		strings.Replace(string(data), `"blobHex":null`, `"blobHex":null,"unknown":true`, 1),
	} {
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, malformed); err == nil || result != nil {
			t.Fatal("malformed/foreign identity wire was accepted", malformed, result, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	audit := heightTableSnapshot(t, db, "Sample_Audit")
	result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, string(data))
	if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
		t.Fatal("facade did not preserve strength0 identity history", result, err)
	}
	if got := heightTableSnapshot(t, db, "Sample_Audit"); got != audit {
		t.Fatal("identity facade granted ID field audits")
	}
	if restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune); err != nil ||
		restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 0 {
		t.Fatal("facade changed typed unaudited restoration semantics", restored, err)
	}
}
