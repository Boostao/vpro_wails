package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExternalSUEnvironmentProductionClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "externalSUEnvironmentWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review   ExternalSUEnvironmentReview         `json:"review"`
		Request  ExternalSUEnvironmentRequest        `json:"request"`
		Outcome  ExternalSUEnvironmentOutcome        `json:"outcome"`
		Recovery ExternalSUEnvironmentAttemptOutcome `json:"recovery"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		t.Fatal("client fixture does not match strict production DTOs", err)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var original, actual any
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil || !reflect.DeepEqual(original, actual) {
		t.Fatal("production Go wire lost or invented client fixture fields", err)
	}
	if wire.Review.Review.ContextID != wire.Recovery.ContextID || wire.Request.RequestID != wire.Recovery.RequestID ||
		wire.Request.ApprovalHash != wire.Review.ApprovalHash || wire.Request.SessionID != wire.Review.SessionID ||
		wire.Outcome.Receipt == nil || wire.Outcome.Receipt.ChangedCells != 3 || wire.Outcome.Receipt.ChangedRows != 1 ||
		wire.Review.ProjectPath == wire.Review.SUPath || len(wire.Review.Review.Personal.Rows) != 0 {
		t.Fatal("synthetic exact external owner/reference/receipt contract changed")
	}
}
