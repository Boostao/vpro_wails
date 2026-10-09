package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTwoPageVegetationClientStrictGoSharedWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Owner struct {
			ContextID string `json:"contextId"`
			Project   string `json:"project"`
			Plot      string `json:"plot"`
			Form      string `json:"form"`
		} `json:"owner"`
		Request       twoPageVegetationRequest `json:"request"`
		ColumnAliases []struct {
			Left  string `json:"left"`
			Right string `json:"right"`
			Equal bool   `json:"equal"`
		} `json:"columnAliases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal("shared frontend fixture differs from strict production request wire", err)
	}
	for _, witness := range fixture.ColumnAliases {
		if strings.EqualFold(witness.Left, witness.Right) != witness.Equal {
			t.Fatal("independent identifier collision witness differs from strict Go schema", witness)
		}
	}
	original := fixture.Request.Original
	if original.ContextID != fixture.Owner.ContextID || original.Project != fixture.Owner.Project ||
		original.Plot != fixture.Owner.Plot || original.Form != fixture.Owner.Form {
		t.Fatal("shared request owner differs from source original")
	}
	plan, err := planTwoPageVegetationEdits(context.Background(), original, fixture.Request.Edits)
	want := []twoPageVegetationAssignment{
		{"-7", "Cover1", 42, siviReal(0), siviReal(5), float64(5)},
		{"-7", "Collected", 42, metadataText(""), ProjectMetadataCell{Storage: "null"}, nil},
		{"-7", "Cover5c", 42, metadataInteger("0"), siviReal(99.5), float64(99.5)},
	}
	if err != nil || len(plan) != len(want) {
		t.Fatal("independent source assignment count differs", plan, err)
	}
	for i, assignment := range plan {
		if assignment.RowID != want[i].RowID || assignment.Column != want[i].Column || assignment.ID != 42 ||
			!twoPageVegetationCellEqual(assignment.Before, want[i].Before) ||
			!twoPageVegetationCellEqual(assignment.After, want[i].After) || assignment.Value != want[i].Value {
			t.Fatal("shared request differs from independent expected assignment", assignment, want[i])
		}
	}
	if original.Records.Rows[1].RowID != "9223372036854775807" ||
		original.Records.Rows[1].Cells[0].Storage != "null" ||
		!math.Signbit(*original.Records.Rows[1].Cells[4].Real) ||
		*original.Records.Rows[0].Cells[19].BlobHex != "ff00" {
		t.Fatal("wire lost physical signed64, NULL ID, signed-zero witness or unmapped bytes")
	}
	encoded, err := json.Marshal(fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip twoPageVegetationRequest
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal("production JSON round trip refused typed request", err)
	}
	reencoded, err := json.Marshal(roundTrip)
	if err != nil || !bytes.Equal(encoded, reencoded) {
		t.Fatal("production JSON round trip changed typed request or REAL bits", err)
	}
}
