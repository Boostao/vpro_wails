package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviCoverServiceFixture(t *testing.T, contexts *ContextService, value string, present bool) *SIVICoverService {
	t.Helper()
	service, err := NewSIVICoverService(contexts, func(name string) (string, bool) {
		if name != siviCoverFeatureEnvironment {
			t.Fatal("cover gate queried another feature", name)
		}
		return value, present
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func siviCoverRequestJSON(t *testing.T, original []siviVegetationProjection, edit siviHeightEdit) string {
	t.Helper()
	request := SIVICoverWrite{Original: original, Edits: []SIVICoverEdit{
		{edit.RowID, edit.Form, edit.Column, edit.Expected, edit.Value},
	}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSIVICoverServiceIndependentDefaultOffAndStrictFeature(t *testing.T) {
	contexts, state, _, original, heights := siviWriteFixture(t, false, 3)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	valid := siviCoverRequestJSON(t, original, edit)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, flag := range []struct {
		value   string
		present bool
	}{{"", false}, {"false", true}} {
		service := siviCoverServiceFixture(t, contexts, flag.value, flag.present)
		if result, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false); err == nil || result != nil {
			t.Fatal("disabled cover read leaked source", result, err)
		}
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, valid); err == nil || result != nil {
			t.Fatal("disabled cover service wrote valid transport", result, err)
		}
		if result, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", "1", AuditRestoreCancel); err == nil || result != nil {
			t.Fatal("disabled cover restoration returned success", result, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	for _, value := range []string{"", "TRUE", "1", " true ", "yes"} {
		if service, err := NewSIVICoverService(contexts, func(string) (string, bool) { return value, true }); err == nil || service != nil {
			t.Fatal("malformed cover gate accepted", value, service, err)
		}
	}
	if service, err := NewSIVICoverService(nil, func(string) (string, bool) { return "true", true }); err == nil || service != nil {
		t.Fatal("cover service lacks ownership", service, err)
	}
	if service, err := NewSIVICoverService(contexts, nil); err == nil || service != nil {
		t.Fatal("cover service lacks explicit gate authority", service, err)
	}
}

func TestSIVICoverServiceValidOwnedWireAndHeightIsolation(t *testing.T) {
	contexts, state, db, original, heights := siviWriteFixture(t, true, 3)
	service := siviCoverServiceFixture(t, contexts, "true", true)
	read, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(read, original) {
		t.Fatal("public source differs from private reviewed projection", read, err)
	}
	if result, err := contexts.GetSIVIVegetation(context.Background(), state.ContextID, "108050", false); err == nil || result != nil {
		t.Fatal("cover gate implicitly enabled height facade", result, err)
	}
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCoverRequestJSON(t, original, edit))
	if err != nil || saved == nil || saved.ChangedCells != 1 || saved.HistoryID == "" {
		t.Fatal("valid strict cover wire failed", saved, err)
	}
	var cover, height float64
	if err := db.QueryRow(`SELECT Cover1,HeightA FROM Sample_Veg WHERE ID=10000001`).Scan(&cover, &height); err != nil ||
		cover != 3 || height != 2 {
		t.Fatal("cover facade widened physical assignment", cover, height, err)
	}
	restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
		t.Fatal("public isolated cover restoration failed", restored, err)
	}
	again, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("public cover restoration did not return exact source", again, err)
	}
}

func TestSIVICoverServiceRejectsRawMalformedTransportWithoutWrites(t *testing.T) {
	contexts, state, _, original, heights := siviWriteFixture(t, false, 3)
	service := siviCoverServiceFixture(t, contexts, "true", true)
	edit := siviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	valid := siviCoverRequestJSON(t, original, edit)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, body := range []string{
		"{}", "null", "[]", valid + "{}",
		strings.Replace(valid, `"original":`, `"Original":`, 1),
		strings.Replace(valid, `"edits":`, `"extra":false,"edits":`, 1),
		strings.Replace(valid, `"edits":`, `"edits":[],"edits":`, 1),
		strings.Replace(valid, `"rowId":`, `"extra":false,"rowId":`, 1),
		strings.Replace(valid, `"column":"Cover1"`, `"column":"HeightA","column":"Cover1"`, 1),
		strings.Replace(valid, `"column":"Cover1"`, `"column":"HeightA"`, 1),
		strings.Replace(valid, `"rowId":"`+edit.RowID+`"`, `"rowId":"\ud800"`, 1),
		strings.Replace(valid, `"rowId":"`+edit.RowID+`"`, `"rowId":"\udc00"`, 1),
		strings.Replace(valid, `"value":{"storage":"real","text":null,"integer":null,"real":3,"blobHex":null}`,
			`"value":{"storage":"text","text":"\ud800","integer":null,"real":null,"blobHex":null}`, 1),
	} {
		if body == valid {
			t.Fatal("malformed transport selector did not change the request")
		}
		if result, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, body); err == nil || result != nil {
			t.Fatal("malformed/raw repaired cover transport was accepted", body, result, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	if result, err := service.GetOriginal(context.Background(), "foreign", "108050", false); err == nil || result != nil {
		t.Fatal("cover facade read foreign ownership", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, valid); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled public cover Save returned authority", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
}
