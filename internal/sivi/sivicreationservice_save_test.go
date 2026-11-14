package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestSIVICreationServiceOwnedReferencesCreateReceiptAndReplay(t *testing.T) {
	_, contexts, state, _, original := siviSpeciesServiceFixture(t, false, 3)
	service, err := NewSIVICreationService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := service.GetReferences(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(refs, original.References) {
		t.Fatalf("independent owned references changed: %v", err)
	}
	zero := 0.0
	request := sourceform.SiviCreationRequest{
		RequestID: "00000000-0000-4000-8000-000000000099", ContextID: state.ContextID,
		Project: "Sample", Plot: "108050", Form: "SubVegA-SIVI_BC", Species: "A",
		Covers: []sourceform.SiviCreationCover{{Column: "Cover1", Value: vprocore.ProjectMetadataCell{Storage: "real", Real: &zero}}},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	missing, err := service.LookupReceipt(context.Background(), state.ContextID, string(raw))
	if err != nil || missing != nil {
		t.Fatalf("uncommitted request inferred a receipt: %+v %v", missing, err)
	}
	if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("missing receipt lookup changed an owned file")
	}
	result, err := service.Create(context.Background(), state.ContextID, string(raw))
	if err != nil || result == nil {
		t.Fatalf("owned creation failed: %+v %v", result, err)
	}
	if !result.DidCommit || result.Replayed || result.RequestID != request.RequestID ||
		result.ContextID != state.ContextID || result.Project != "Sample" || result.Plot != "108050" ||
		result.RowID == "" || result.HistoryID == "" || len(result.Columns) != 44 || len(result.Committed.Cells) != 44 {
		t.Fatalf("incomplete source-owned creation receipt: %+v", result)
	}
	committed := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, operation := range []func() (*SIVICreationResult, error){
		func() (*SIVICreationResult, error) {
			return service.LookupReceipt(context.Background(), state.ContextID, string(raw))
		},
		func() (*SIVICreationResult, error) {
			return service.Create(context.Background(), state.ContextID, string(raw))
		},
	} {
		replayed, err := operation()
		if err != nil || replayed == nil || replayed.DidCommit || !replayed.Replayed ||
			replayed.ID != result.ID || replayed.RowID != result.RowID || replayed.HistoryID != result.HistoryID ||
			!reflect.DeepEqual(replayed.Committed, result.Committed) {
			t.Fatalf("verified receipt differs: %+v %v", replayed, err)
		}
		if !reflect.DeepEqual(committed, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
			t.Fatal("durable lookup/replay changed an owned file")
		}
	}
}
