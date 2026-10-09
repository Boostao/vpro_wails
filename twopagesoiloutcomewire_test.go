package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageSoilClientSharedOutcomeWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageSoilOutcomeWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var outcomes []TwoPageSoilAttemptOutcome
	if err := json.Unmarshal(raw, &outcomes); err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 4 {
		t.Fatal("shared actual-Go outcome fixture lost one of four statuses")
	}
	for index, status := range []string{"running", "unknown", "notCommitted", "committed"} {
		outcome := outcomes[index]
		var request twoPageSoilRequest
		if err := json.Unmarshal([]byte(outcome.RequestJSON), &request); err != nil {
			t.Fatal("echoed wire is not a complete strict-Go source request", err)
		}
		if outcome.Status != status || outcome.ContextID != request.Original.ContextID ||
			outcome.RequestID != request.RequestID || outcome.Reason != "Fixture "+status {
			t.Fatal("shared outcome lost independent exact wire expectations", outcome)
		}
		if status == "committed" {
			if outcome.Receipt == nil || outcome.Receipt.DidCommit || !outcome.Receipt.Replayed ||
				!reflect.DeepEqual(outcome.Receipt.History.Request, request) {
				t.Fatal("shared committed outcome lost read-only complete request provenance")
			}
			if err := validateTwoPageSoilHistory(context.Background(), outcome.Receipt.History); err != nil {
				t.Fatal(err)
			}
		} else if outcome.Receipt != nil {
			t.Fatal("noncommitted status invented a committed source receipt")
		}
		encoded, err := json.Marshal(outcome)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip TwoPageSoilAttemptOutcome
		if err := json.Unmarshal(encoded, &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, outcome) {
			t.Fatal("actual Go outcome serialization changed client wire", err)
		}
	}
}
