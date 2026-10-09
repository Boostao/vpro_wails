package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPictureLifecycleClientFixtureStrictGoRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("frontend", "src", "pictureLifecycleWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Deletion               pictureDeletionResult `json:"deletion"`
		RestorationRequestID   string                `json:"restorationRequestId"`
		RestoringActor         string                `json:"restoringActor"`
		RestoringAuditStrength int                   `json:"restoringAuditStrength"`
		RestoredWhen           string                `json:"restoredWhen"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal("client fixture differs from strict production wire", err)
	}
	wire, err := json.Marshal(fixture.Deletion)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct{ Deletion json.RawMessage }
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	var before, after any
	if err := json.Unmarshal(envelope.Deletion, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire, &after); err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("production serialization changed the shared deletion wire", before, after, err)
	}
	restoration := pictureRestorationResult{Request: pictureRestorationRequest{
		RequestID: fixture.RestorationRequestID, ContextID: "later-owned", HistoryID: fixture.Deletion.HistoryID,
		Deletion: fixture.Deletion.Request, Peers: []ProjectMetadataRow{},
	}, Deletion: fixture.Deletion, Actor: fixture.RestoringActor, AuditStrength: fixture.RestoringAuditStrength,
		RestoredWhen: fixture.RestoredWhen, DidCommit: true}
	wire, err = json.Marshal(restoration)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodePictureRestoration(wire, fixture.Deletion)
	if err != nil || !reflect.DeepEqual(decoded, restoration) {
		t.Fatal("current-context fixture restoration did not round-trip", decoded, err)
	}
}
