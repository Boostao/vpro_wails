package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageSoilClientStrictGoSharedWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageSoilWire.json"))
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
		Request twoPageSoilRequest `json:"request"`
		PeerIDs []struct {
			ID       string          `json:"id"`
			Storage  string          `json:"storage"`
			Value    json.RawMessage `json:"value"`
			Collides bool            `json:"collides"`
		} `json:"peerIDs"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	original := fixture.Request.Original
	if original.ContextID != fixture.Owner.ContextID || original.Project != fixture.Owner.Project ||
		original.Plot != fixture.Owner.Plot || original.Form != fixture.Owner.Form {
		t.Fatal("shared fixture owner differs from strict original")
	}
	got, err := planTwoPageSoilEdits(context.Background(), original, fixture.Request.Edits)
	want := []twoPageSoilAssignment{
		{"-7", "Horizon", -2147483648, ProjectMetadataCell{Storage: "null"}, metadataText("LF"), "LF"},
		{"-7", "UpperDepth", -2147483648, siviReal(-0.25), siviReal(5), float64(5)},
		{"-7", "Comment", -2147483648, metadataText(""), ProjectMetadataCell{Storage: "null"}, nil},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("shared client request does not have independent expected Go assignments: %v, %v", got, err)
	}
	encoded, err := json.Marshal(fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip twoPageSoilRequest
	if err := json.Unmarshal(encoded, &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, fixture.Request) {
		t.Fatal("shared strict request repaired or lost source data", err)
	}
	if original.Records.Rows[1].RowID != "9223372036854775807" || original.Records.Rows[1].Cells[0].Storage != "null" ||
		original.Records.Columns[1].DeclaredType != "TEXT" || original.Records.Rows[0].Cells[5].BlobHex == nil ||
		*original.Records.Rows[0].Cells[5].BlobHex != "ff00" {
		t.Fatal("shared wire lost declaredType, physical identity, NULL peer or unmapped bytes")
	}
	for _, peer := range fixture.PeerIDs {
		t.Run(peer.ID+"/"+peer.Storage+"/"+string(peer.Value), func(t *testing.T) {
			cell := ProjectMetadataCell{Storage: peer.Storage}
			switch peer.Storage {
			case "integer":
				if err := json.Unmarshal(peer.Value, &cell.Integer); err != nil {
					t.Fatal(err)
				}
			case "real":
				if err := json.Unmarshal(peer.Value, &cell.Real); err != nil {
					t.Fatal(err)
				}
			case "text":
				if err := json.Unmarshal(peer.Value, &cell.Text); err != nil {
					t.Fatal(err)
				}
			case "blob":
				if err := json.Unmarshal(peer.Value, &cell.BlobHex); err != nil {
					t.Fatal(err)
				}
			case "null":
				if string(peer.Value) != "null" {
					t.Fatal("NULL peer fixture is not NULL")
				}
			default:
				t.Fatal("unknown peer fixture storage")
			}
			if siviIdentityCellMatches(cell, peer.ID) != peer.Collides {
				t.Fatal("independent expected peer collision differs from Go identity ownership")
			}
			var candidate twoPageSoilRequest
			if err := json.Unmarshal(encoded, &candidate); err != nil {
				t.Fatal(err)
			}
			candidate.Original.Records.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "integer", Integer: &peer.ID}
			candidate.Original.Records.Rows[1].Cells[0] = cell
			plan, err := planTwoPageSoilEdits(context.Background(), candidate.Original, candidate.Edits)
			if peer.Collides && err == nil {
				t.Fatal("Go planner accepted a duplicate numeric identity", plan)
			}
			if !peer.Collides && (err != nil || len(plan) != 3) {
				t.Fatal("Go planner refused an independently noncolliding peer", err)
			}
		})
	}
}
