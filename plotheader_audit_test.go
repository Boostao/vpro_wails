package main

import (
	"encoding/json"
	"testing"
)

func TestPlotHeader_AuditRowIdentity(t *testing.T) {
	s, db := headerFixture(t)
	if err := s.SavePlot(FS882Header{PlotNumber: "HDRKEY"}); err != nil {
		t.Fatal(err)
	}
	for _, rowID := range []int64{9007199254740993, 9007199254740994} {
		if _, err := db.Exec(`INSERT INTO Sample_Audit
			(rowid, Project, User, PlotNumber, "Table", EditField, EditWhen, BeforeEdit, AfterEdit, Restore, Flag, ID)
			VALUES (?, 'Sample', 'Tester', 'HDRKEY', '_Env', 'Elevation', '2026-09-29 21:00:00', '1', '2', 0, 0, 42)`, rowID); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := s.ListAuditEntries("HDRKEY")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("audit entries: %d", len(entries))
	}
	want := map[string]bool{"9007199254740993": true, "9007199254740994": true}
	for _, entry := range entries {
		if !want[entry.RowID] {
			t.Fatalf("duplicate or inaccurate rowId: %q", entry.RowID)
		}
		delete(want, entry.RowID)
		if entry.ID == nil || *entry.ID != 42 {
			t.Fatalf("child record ID changed: %v", entry.ID)
		}
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["rowId"] != entry.RowID {
			t.Fatalf("rowId must be an exact JSON string: %s", data)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing row identifiers: %v", want)
	}
}
