package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageSoilClientSharedReceiptWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageSoilReceiptWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Name    string                 `json:"name"`
		Receipt twoPageSoilWriteResult `json:"receipt"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatal("shared receipt fixture lost one of four audit policies")
	}
	for strength, entry := range entries {
		history := entry.Receipt.History
		if err := validateTwoPageSoilHistory(context.Background(), history); err != nil {
			t.Fatal(entry.Name, err)
		}
		if history.AuditStrength != strength || len(history.Audits) != strength ||
			history.Actor != "Fixture actor" || history.EditWhen != "2026-10-08 12:34:56" ||
			!entry.Receipt.DidCommit || entry.Receipt.Replayed {
			t.Fatal("shared receipt lost independent provenance/count/commit expectations")
		}
		if history.Committed.Rows[0].Cells[2].Storage != "text" ||
			*history.Committed.Rows[0].Cells[2].Text != "LF" ||
			history.Committed.Rows[0].Cells[3].Storage != "real" ||
			*history.Committed.Rows[0].Cells[3].Real != 5 ||
			history.Committed.Rows[0].Cells[4].Storage != "null" ||
			history.Committed.Rows[1].Cells[0].Storage != "null" {
			t.Fatal("shared receipt lost complete mixed committed plan or NULL peer")
		}
		encoded, err := json.Marshal(entry.Receipt)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip twoPageSoilWriteResult
		if err := json.Unmarshal(encoded, &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, entry.Receipt) {
			t.Fatal("actual Go receipt serialization changed strict client wire", err)
		}
		if strength == 1 {
			history := roundTrip.History
			history.Request.Original.Records.Columns[3].DeclaredType = ""
			id := "5"
			history.Request.Original.Records.Rows[0].Cells[3] = ProjectMetadataCell{Storage: "integer", Integer: &id}
			history.Request.Edits[1].Expected = ProjectMetadataCell{Storage: "integer", Integer: &id}
			history.Committed.Columns[3].DeclaredType = ""
			history.Audits[0].BeforeEdit = &id
			if err := validateTwoPageSoilHistory(context.Background(), history); err != nil {
				t.Fatal("legitimate typed storage transition was omitted despite identical audit text", err)
			}
			blob, before, after := "ff00", "[255 0]", "LF"
			history.Request.Original.Records.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "blob", BlobHex: &blob}
			history.Request.Edits[0].Expected = history.Request.Original.Records.Rows[0].Cells[2]
			audit := history.Audits[0]
			audit.RowID, audit.EditField = "-1", "Horizon"
			audit.BeforeEdit, audit.AfterEdit = &before, &after
			history.Audits = append([]AuditEntry{audit}, history.Audits...)
			if err := validateTwoPageSoilHistory(context.Background(), history); err != nil {
				t.Fatal("historical BLOB byte-slice audit changed its exact source representation", err)
			}
		}
	}
}
