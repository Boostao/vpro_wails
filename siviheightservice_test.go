package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIHeightFeatureRequiresExplicitLiteralOptIn(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{
		{"", false, false, false},
		{"true", true, true, false},
		{"false", true, false, false},
		{"", true, false, true},
		{"TRUE", true, false, true},
		{"1", true, false, true},
		{" true ", true, false, true},
	} {
		enabled, err := siviHeightFeature(func(name string) (string, bool) {
			if name != "VPRO_SIVI_HEIGHT_EDITING" {
				t.Fatal("unexpected feature setting", name)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
}

func TestSIVIHeightFacadeDefaultDeniesWithoutDatabaseChanges(t *testing.T) {
	service, state, _, original, edits := siviWriteFixture(t, true, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.GetSIVIVegetation(context.Background(), state.ContextID, "108050", false); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("default reader was enabled", result, err)
	}
	if result, err := service.SaveSIVIHeights(context.Background(), state.ContextID, "108050", false, original, edits); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("default writer was enabled", result, err)
	}
	if result, err := service.RestoreSIVIHeights(context.Background(), state.ContextID, "108050", "1", AuditRestoreCancel); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("default restorer was enabled", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIHeightFacadeTransportAndOwnedRoundTrip(t *testing.T) {
	service, state, db, original, edits := siviWriteFixture(t, true, 3)
	service.siviHeightEnabled = true
	review, err := service.GetSIVIVegetation(context.Background(), state.ContextID, "108050", false)
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
	var decoded []SIVIVegetationProjection
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(review, decoded) {
		t.Fatal("tagged original transport changed", err)
	}
	editJSON, err := json.Marshal(edits)
	if err != nil {
		t.Fatal(err)
	}
	var transported []SIVIHeightEdit
	if err := json.Unmarshal(editJSON, &transported); err != nil || !reflect.DeepEqual(edits, transported) {
		t.Fatal("strict edit transport changed", err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.GetSIVIVegetation(cancelled, state.ContextID, "108050", false); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("read cancellation lost", result, err)
	}
	if result, err := service.SaveSIVIHeights(cancelled, state.ContextID, "108050", false, decoded, transported); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("write cancellation lost", result, err)
	}
	if result, err := service.RestoreSIVIHeights(cancelled, state.ContextID, "108050", "1", AuditRestorePrune); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("restore cancellation lost", result, err)
	}
	if result, err := service.SaveSIVIHeights(context.Background(), "stale", "108050", false, decoded, transported); result != nil || err == nil {
		t.Fatal("stale public context accepted", result, err)
	}
	assertProfileSUFiles(t, service, before)
	result, err := service.SaveSIVIHeights(context.Background(), state.ContextID, "108050", false, decoded, transported)
	if err != nil || result == nil || result.ChangedCells != 3 || result.HistoryID == "" {
		t.Fatal(result, err)
	}
	restored, err := service.RestoreSIVIHeights(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestorePrune)
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
		var edit SIVIHeightEdit
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
		var projection SIVIVegetationProjection
		if err := json.Unmarshal([]byte(raw), &projection); err == nil {
			t.Fatal("original projection accepted malformed transport", raw)
		}
	}
}
