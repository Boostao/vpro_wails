package main

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestCollectedSourceCyclesAndTransport(t *testing.T) {
	text := func(value string) *string { return &value }
	for _, initial := range []*string{nil, text("C"), text("V"), text("c"), text("v"), text("\uFF23"), text("\uFF43"), text("\uFF36"), text("\uFF56"), text(""), text("X"), text("historical")} {
		for clicks := 1; clicks <= 3; clicks++ {
			want := initial
			for range clicks {
				switch otherNullable(want) {
				case nil:
					want = text("C")
				case "C", "c", "\uFF23", "\uFF43":
					want = text("V")
				case "V", "v", "\uFF36", "\uFF56":
					want = nil
				}
			}
			if !reflect.DeepEqual(collectedAfterClicks(initial, clicks), want) {
				t.Fatalf("cycle %v/%d", otherNullable(initial), clicks)
			}
		}
	}
	for _, payload := range []string{
		`{}`, `{"id":0,"clicks":1}`, `{"id":null,"expected":null,"clicks":1}`,
		`{"id":0.5,"expected":null,"clicks":1}`, `{"id":0,"expected":1,"clicks":1}`,
		`{"id":0,"expected":null,"clicks":null}`, `{"id":0,"expected":null,"clicks":1.5}`,
		`{"id":0,"expected":"\ud800","clicks":1}`,
		`{"id":0,"expected":"C","EXPECTED":"\udfff","clicks":1}`,
	} {
		if err := json.Unmarshal([]byte(payload), &CollectedRecordUpdate{}); err == nil {
			t.Fatal("invalid transport accepted:", payload)
		}
	}
	var update CollectedRecordUpdate
	if err := json.Unmarshal([]byte(`{"id":0,"expected":null,"clicks":1}`), &update); err != nil || update.Expected != nil || update.Clicks != 1 {
		t.Fatal("explicit zero/NULL was lost:", update, err)
	}
}

func TestCollectedAtomicStaleRollbackOwnershipAndHistoricalOmission(t *testing.T) {
	s, db := childFixture(t)
	malformed, historical, visual := string([]byte{0xff}), "historical", "V"
	s.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,Species,ID,Cover1,Cover6,Cover7) VALUES
		('CHILD1','RAW',0,0,0,0),('CHILD1','RAW',-9,0,0,0)`); err != nil {
		t.Fatal(err)
	}
	updates := []CollectedRecordUpdate{{ID: 0, Clicks: 1}, {ID: -9, Clicks: 2}}
	before := substrateSnapshot(t, db)
	if _, err := db.Exec(`CREATE TRIGGER fail_collected BEFORE INSERT ON Sample_Audit
		WHEN NEW.ID=-9 BEGIN SELECT RAISE(ABORT,'injected collected audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollectedRecords("CHILD1", updates); err == nil {
		t.Fatal("partial multirow save accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_collected`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollectedRecords("CHILD1", updates); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, db, "CHILD1") != 2 {
		t.Fatal("cycle generated intermediate or missing audits")
	}
	before = substrateSnapshot(t, db)
	for _, batch := range [][]CollectedRecordUpdate{
		updates, nil, {{ID: 0}}, {{ID: 0, Clicks: 4}}, {{ID: 0, Clicks: -1}},
		{{ID: 999, Clicks: 1}}, {{ID: math.MaxInt32 + 1, Clicks: 1}},
		{{ID: -9, Clicks: 1}, {ID: -9, Clicks: 1}},
		{{ID: 0, Expected: &malformed, Clicks: 1}},
	} {
		if err := s.UpdateCollectedRecords("CHILD1", batch); err == nil {
			t.Fatal("invalid/stale patch accepted:", batch)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if err := s.UpdateCollectedRecords("CHILD2", updates); err == nil {
		t.Fatal("foreign identities accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Collected='historical' WHERE ID=0;
		CREATE TRIGGER preserve_collected BEFORE UPDATE OF Collected ON Sample_Veg WHEN OLD.ID=0
		BEGIN SELECT RAISE(ABORT,'historical assignment must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollectedRecords("CHILD1", []CollectedRecordUpdate{
		{ID: 0, Expected: &historical, Clicks: 1},
		{ID: -9, Expected: &visual, Clicks: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, db, "CHILD1") != 3 {
		t.Fatal("historical no-op or NULL transition audit differs")
	}
	before = substrateSnapshot(t, db)
	if err := s.UpdateCollectedRecords("CHILD1", []CollectedRecordUpdate{{ID: -9, Clicks: 3}}); err != nil {
		t.Fatal(err)
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DELETE FROM Sample_Env WHERE PlotNumber='CHILD1'`); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	if err := s.UpdateCollectedRecords("CHILD1", []CollectedRecordUpdate{{ID: -9, Clicks: 1}}); err == nil {
		t.Fatal("missing parent accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}
