package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func otherPatch(id int, property string, before, after *string) OtherRecordUpdate {
	return OtherRecordUpdate{ID: id, Text: map[string]OtherTextUpdate{property: {Expected: before, Value: after}}}
}

func TestOtherPhysicalTextSaveRoutesAndRawJSON(t *testing.T) {
	s, db := childFixture(t)
	for column, maximum := range otherTextMaximum {
		member := reflect.ValueOf(&OtherRecord{}).Elem().FieldByName(column)
		property := childJSONKey(reflect.ValueOf(OtherRecord{}), childField{column, column})
		for _, invalid := range []string{strings.Repeat("x", maximum+1), strings.Repeat("x", maximum-1) + "\U0001F332", string([]byte{255})} {
			record := OtherRecord{PlotNumber: "CHILD1"}
			member = reflect.ValueOf(&record).Elem().FieldByName(column)
			member.Set(reflect.ValueOf(&invalid))
			before := substrateSnapshot(t, db)
			if err := s.SaveOtherRecord(record); err == nil {
				t.Fatalf("create accepted invalid %s", column)
			}
			assertSubstrateSnapshot(t, db, before)
		}
		for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"bad` + string([]byte{255}) + `"`, `"\ud800","` + property + `":"valid"`} {
			if err := json.Unmarshal([]byte(`{"`+property+`":`+raw+`}`), &OtherRecord{}); err == nil {
				t.Fatalf("raw repaired %s: %q", property, raw)
			}
		}
		exact := strings.Repeat("x", maximum-2) + "\U0001F332"
		if err := validateOtherField(column, exact); err != nil {
			t.Fatal(err)
		}
		if err := validateOtherField(column, ""); err != nil {
			t.Fatalf("physical guard invented zero-length prohibition: %v", err)
		}
	}
	for _, payload := range []string{
		`{}`, `{"id":null}`, `{"id":1.5}`, `{"id":0,"text":{"dataName":{}}}`,
		`{"id":0,"flags":{"userFlag1":{"value":null}}}`,
		`{"id":0,"text":{"dataName":{"expected":null,"value":"\ud800","value":"valid"}}}`,
		`{"id":0,"flags":{"userFlag1":{"expected":null,"value":-1}}}`,
	} {
		if err := json.Unmarshal([]byte(payload), &OtherRecordUpdate{}); err == nil {
			t.Fatalf("malformed patch accepted: %s", payload)
		}
		if err := json.Unmarshal([]byte(`{"ſitePlotQuality":"\ud800"}`), &FS882Header{}); err == nil {
			t.Fatal("Unicode identifier recasing bypassed the preserved header token guard")
		}
	}
}

func TestOtherHistoricalOmissionAndFreshRestoration(t *testing.T) {
	s, db := childFixture(t)
	if err := s.SaveOtherRecord(OtherRecord{PlotNumber: "CHILD1"}); err != nil {
		t.Fatal(err)
	}
	id := childID(t, db, "Other", "CHILD1")
	historical := strings.Repeat("h", 51)
	if _, err := db.Exec(`UPDATE Sample_Other SET DataName=? WHERE ID=?`, historical, id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER forbid_historical_assignment BEFORE UPDATE OF DataName ON Sample_Other
		BEGIN SELECT RAISE(ABORT,'historical column must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	record := loadedChildRecord(t, s, "Other", "CHILD1").Interface().(OtherRecord)
	record.DataItem = qualityString(" raw' item ")
	for _, save := range []func(OtherRecord) error{s.SaveOtherRecord, s.UpdateOtherRecord} {
		if err := save(record); err != nil {
			t.Fatalf("unchanged historical assignment: %v", err)
		}
	}
	invalid := strings.Repeat("x", 52)
	record.DataName = &invalid
	before := substrateSnapshot(t, db)
	if err := s.UpdateOtherRecord(record); err == nil {
		t.Fatal("update accepted new overflow")
	}
	assertSubstrateSnapshot(t, db, before)
	for _, alias := range []string{"_Other", "Sample_Other", "_oThEr", "sAmPlE_oThEr"} {
		rowID := restoreAuditFixture(t, db, "CHILD1", alias, "dataNAME", &id, invalid, historical)
		before = substrateSnapshot(t, db)
		if err := s.SetAuditRestoreSelection("CHILD1", []string{rowID}); err == nil {
			t.Fatal("invalid fresh restoration selected")
		}
		if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{rowID}, AuditRestoreRetain); err == nil {
			t.Fatal("invalid fresh restoration accepted")
		}
		assertSubstrateSnapshot(t, db, before)
	}
}

func TestOtherAtomicPatchesNullFlagsConflictRollbackRetry(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	for range 2 {
		if err := s.SaveOtherRecord(OtherRecord{PlotNumber: "CHILD1"}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.ListOtherRecords("CHILD1")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	clearChildAudit(t, db)
	yes := true
	patch := otherPatch(int(rows[0].ID), "dataName", nil, qualityString(" raw' name "))
	patch.Flags = map[string]OtherFlagUpdate{"userFlag1": {Expected: nil, Value: &yes}}
	second := otherPatch(int(rows[1].ID), "userItem3", nil, qualityString("raw second"))
	before := substrateSnapshot(t, db)
	if _, err := db.Exec(`CREATE TRIGGER fail_other_audit BEFORE INSERT ON Sample_Audit
		WHEN NEW."Table"='_Other' AND NEW.EditField='UserItem3'
		BEGIN SELECT RAISE(ABORT,'injected Other audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateOtherRecords("CHILD1", []OtherRecordUpdate{patch, second}); err == nil {
		t.Fatal("audit failure accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_other_audit`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateOtherRecords("CHILD1", []OtherRecordUpdate{patch, second}); err != nil {
		t.Fatal(err)
	}
	assertRestoreField(t, db, "Other", "UserFlag1", "CHILD1", &rows[0].ID, true)
	if auditCount(t, db, "CHILD1") != 3 {
		t.Fatal("incorrect exact audit count")
	}
	before = substrateSnapshot(t, db)
	for _, updates := range [][]OtherRecordUpdate{
		{patch}, {second, second}, {otherPatch(99999, "dataName", nil, nil)},
		{otherPatch(int(rows[0].ID), "unknown", nil, nil)}, {{ID: int(rows[0].ID)}},
	} {
		if err := s.UpdateOtherRecords("CHILD1", updates); err == nil {
			t.Fatal("invalid/stale patch accepted")
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if err := s.UpdateOtherRecords("CHILD2", []OtherRecordUpdate{second}); err == nil {
		t.Fatal("foreign plot accepted")
	}
	clear := OtherRecordUpdate{ID: int(rows[0].ID), Flags: map[string]OtherFlagUpdate{"userFlag1": {Expected: &yes, Value: nil}}}
	if err := s.UpdateOtherRecords("CHILD1", []OtherRecordUpdate{clear}); err != nil {
		t.Fatal(err)
	}
	assertRestoreField(t, db, "Other", "UserFlag1", "CHILD1", &rows[0].ID, nil)
}
