package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
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

func TestSIVIIdentityServiceUnavailableContext(t *testing.T) {
	if service, err := NewSIVIIdentityService(nil); service != nil || err == nil {
		t.Fatal("identity constructed without context ownership", service, err)
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
	contexts, state, db, original, heights := sftest.SiviWriteFixture(t, false, 0)
	service, err := NewSIVIIdentityService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(wire, original) {
		t.Fatal("facade lost actual source originals", wire, err)
	}
	request := SIVIIdentityWrite{Original: original, Edits: []sourceform.SIVIHeightEdit{
		{RowID: heights[0].RowID, Form: "SubVegA-SIVI_BC", Column: "ID", Expected: vprotest.MetadataInteger("10000001"), Value: vprotest.MetadataInteger("-1")},
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
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, malformed); err == nil || result != nil {
			t.Fatal("malformed/foreign identity wire was accepted", malformed, result, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	audit := vprotest.HeightTableSnapshot(t, db, "Sample_Audit")
	result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, string(data))
	if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
		t.Fatal("facade did not preserve strength0 identity history", result, err)
	}
	if got := vprotest.HeightTableSnapshot(t, db, "Sample_Audit"); got != audit {
		t.Fatal("identity facade granted ID field audits")
	}
	if restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", result.HistoryID, vprocore.AuditRestorePrune); err != nil ||
		restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 0 {
		t.Fatal("facade changed typed unaudited restoration semantics", restored, err)
	}
}
