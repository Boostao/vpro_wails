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
	"testing"
)

func TestSIVIHeightFacadeTransportAndOwnedRoundTrip(t *testing.T) {
	service, state, db, original, edits := sftest.SiviWriteFixture(t, true, 3)
	review, err := GetSIVIVegetation(service, context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(review, original) {
		t.Fatal("public source read differs", review, err)
	}
	raw, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	var transport []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &transport); err != nil {
		t.Fatal(err)
	}
	if len(transport) != 3 {
		t.Fatal("source groups lost", string(raw))
	}
	for _, group := range transport {
		if len(group) != 4 || group["Form"] == nil || group["Query"] == nil || group["Columns"] == nil || group["Rows"] == nil {
			t.Fatal("public JSON shape drifted", string(raw))
		}
	}
	var decoded []sourceform.SIVIVegetationProjection
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(review, decoded) {
		t.Fatal("tagged original transport changed", err)
	}
	editJSON, err := json.Marshal(edits)
	if err != nil {
		t.Fatal(err)
	}
	var transported []sourceform.SIVIHeightEdit
	if err := json.Unmarshal(editJSON, &transported); err != nil || !reflect.DeepEqual(edits, transported) {
		t.Fatal("strict edit transport changed", err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := GetSIVIVegetation(service, cancelled, state.ContextID, "108050", false); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("read cancellation lost", result, err)
	}
	if result, err := SaveSIVIHeights(service, cancelled, state.ContextID, "108050", false, decoded, transported); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("write cancellation lost", result, err)
	}
	if result, err := RestoreSIVIHeights(service, cancelled, state.ContextID, "108050", "1", vprocore.AuditRestorePrune); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("restore cancellation lost", result, err)
	}
	if result, err := SaveSIVIHeights(service, context.Background(), "stale", "108050", false, decoded, transported); result != nil || err == nil {
		t.Fatal("stale public context accepted", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	result, err := SaveSIVIHeights(service, context.Background(), state.ContextID, "108050", false, decoded, transported)
	if err != nil || result == nil || result.ChangedCells != 3 || result.HistoryID == "" {
		t.Fatal(result, err)
	}
	restored, err := RestoreSIVIHeights(service, context.Background(), state.ContextID, "108050", result.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 3 || restored.PrunedAuditRows != 3 {
		t.Fatal(restored, err)
	}
	var a, c float64
	var b string
	if err := db.QueryRow(`SELECT HeightA,HeightB,Height6 FROM Sample_Veg WHERE ID=10000001`).Scan(&a, &b, &c); err != nil || a != 2 || b != "before" || c != -3 {
		t.Fatal("public typed restoration failed", a, b, c, err)
	}
}

func TestSIVIHeightExportedDraftRetainsStrictJSONDecoder(t *testing.T) {
	for _, raw := range []string{
		`{"rowId":"1","form":"SubVegA-SIVI_BC","column":"HeightB","expected":{"storage":"null"},"value":{"storage":"text","text":"\ud800"}}`,
		`{"rowId":"1","form":"SubVegA-SIVI_BC","column":"HeightB","expected":{"storage":"null"},"value":null}`,
		`{"rowId":"1","form":"SubVegA-SIVI_BC","column":"HeightB","expected":{"storage":"null"},"value":{"storage":"null"},"extra":true}`,
	} {
		var edit sourceform.SIVIHeightEdit
		if err := json.Unmarshal([]byte(raw), &edit); err == nil {
			t.Fatal("exported draft accepted malformed transport", raw)
		}
	}
}

func TestSIVIHeightOriginalProjectionRejectsRawUnicodeAndIncompleteJSON(t *testing.T) {
	for _, raw := range []string{
		`{"Form":"SubVegA-SIVI_BC","Query":"USysVegA","Columns":["HeightB"],"Rows":[{"rowId":"1","cells":[{"storage":"text","text":"\ud800"}]}]}`,
		`{"Form":"SubVegA-SIVI_BC","Query":"USysVegA","Columns":null,"Rows":[]}`,
		`{"Form":"SubVegA-SIVI_BC","Query":"USysVegA","Columns":[]}`,
		`{"Form":"SubVegA-SIVI_BC","Query":"USysVegA","Columns":[],"Rows":[],"extra":true}`,
	} {
		var projection sourceform.SIVIVegetationProjection
		if err := json.Unmarshal([]byte(raw), &projection); err == nil {
			t.Fatal("original projection accepted malformed transport", raw)
		}
	}
}
