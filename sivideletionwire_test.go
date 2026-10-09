package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSIVIDeletionAuditRealProducerFormat(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("resources", "sivi-audit-number-format.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Real float64 `json:"real"`
		Text string  `json:"text"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 8 {
		t.Fatal("complete shared audit number-format fixture was not loaded")
	}
	for _, value := range cases {
		if actual := headerAuditValue(value.Real); actual != value.Text {
			t.Errorf("real %v producer text = %v; frontend fixture requires %q", value.Real, actual, value.Text)
		}
	}
}
