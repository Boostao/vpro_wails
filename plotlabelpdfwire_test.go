package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlotLabelPDFStrictClientWireRoundTrip(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "plotLabelPDFWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Review   PlotLabelPDFExportReview   `json:"review"`
		Request  PlotLabelPDFExportRequest  `json:"request"`
		Outcome  PlotLabelPDFExportOutcome  `json:"outcome"`
		Recovery PlotLabelPDFAttemptOutcome `json:"recovery"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var original, returned any
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &returned); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, returned) {
		t.Fatal("client fixture differs from the strict production wire")
	}
	if wire.Review.PDFSHA256 != wire.Outcome.SHA256 || wire.Request.SessionID != wire.Review.SessionID ||
		wire.Recovery.Outcome == nil || *wire.Recovery.Outcome != wire.Outcome {
		t.Fatal("client fixture contradicts its approved owner/receipt")
	}
}
