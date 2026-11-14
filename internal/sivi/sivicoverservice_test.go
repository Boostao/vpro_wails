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

func siviCoverServiceFixture(t *testing.T, contexts *vprocore.ContextService, _ string, _ bool) *SIVICoverService {
	t.Helper()
	service, err := NewSIVICoverService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func siviCoverRequestJSON(t *testing.T, original []sourceform.SiviVegetationProjection, edit sourceform.SiviHeightEdit) string {
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

func TestSIVICoverServiceOwnerGuards(t *testing.T) {
	contexts, _, _, _, _ := sftest.SiviWriteFixture(t, false, 3)
	defaultCover := siviCoverServiceFixture(t, contexts, "", false)
	if defaultCover == nil {
		t.Fatal("default cover service nil")
	}
	if service, err := NewSIVICoverService(nil); err == nil || service != nil {
		t.Fatal("cover service lacks ownership", service, err)
	}
}

func TestSIVICoverServiceValidOwnedWireAndHeightIsolation(t *testing.T) {
	contexts, state, db, original, heights := sftest.SiviWriteFixture(t, true, 3)
	service := siviCoverServiceFixture(t, contexts, "true", true)
	read, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(read, original) {
		t.Fatal("public source differs from private reviewed projection", read, err)
	}
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	saved, err := service.SaveReviewed(context.Background(), state.ContextID, "108050", false, siviCoverRequestJSON(t, original, edit))
	if err != nil || saved == nil || saved.ChangedCells != 1 || saved.HistoryID == "" {
		t.Fatal("valid strict cover wire failed", saved, err)
	}
	var cover, height float64
	if err := db.QueryRow(`SELECT Cover1,HeightA FROM Sample_Veg WHERE ID=10000001`).Scan(&cover, &height); err != nil ||
		cover != 3 || height != 2 {
		t.Fatal("cover facade widened physical assignment", cover, height, err)
	}
	restored, err := service.RestoreReviewed(context.Background(), state.ContextID, "108050", saved.HistoryID, vprocore.AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
		t.Fatal("public isolated cover restoration failed", restored, err)
	}
	again, err := service.GetOriginal(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("public cover restoration did not return exact source", again, err)
	}
}

func TestSIVICoverServiceRejectsRawMalformedTransportWithoutWrites(t *testing.T) {
	contexts, state, _, original, heights := sftest.SiviWriteFixture(t, false, 3)
	service := siviCoverServiceFixture(t, contexts, "true", true)
	edit := sourceform.SiviHeightEdit{heights[0].RowID, "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	valid := siviCoverRequestJSON(t, original, edit)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
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
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	if result, err := service.GetOriginal(context.Background(), "foreign", "108050", false); err == nil || result != nil {
		t.Fatal("cover facade read foreign ownership", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.SaveReviewed(ctx, state.ContextID, "108050", false, valid); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled public cover Save returned authority", result, err)
	}
	vprotest.AssertProfileSUFiles(t, contexts, before)
}
