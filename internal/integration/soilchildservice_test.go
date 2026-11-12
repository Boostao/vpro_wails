package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func soilTextPatch(kind string, id int, property string, expected, value *string) vprocore.SoilRecordUpdate {
	return vprocore.SoilRecordUpdate{Kind: kind, ID: id, Text: map[string]vprocore.SoilTextUpdate{property: {Expected: expected, Value: value}}}
}

func TestSoilAtomicExpectedPatchesDomainsAndNull(t *testing.T) {
	s, db := vprotest.ChildFixture(t)
	s.SetAuditStrength(3)
	var updates []vprocore.SoilRecordUpdate
	for _, kind := range []string{"Humus", "Mineral"} {
		if _, err := db.Exec(`INSERT INTO "Sample_` + kind + `" (PlotNumber,ID) VALUES ('CHILD1',0)`); err != nil {
			t.Fatal(err)
		}
		update := vprocore.SoilRecordUpdate{Kind: kind, ID: 0, Text: map[string]vprocore.SoilTextUpdate{}, Numbers: map[string]vprocore.SoilNumberUpdate{}}
		for _, field := range vprocore.ChildFields[kind] {
			property := vprocore.ChildJSONKey(vprocore.ChildPatchShape(kind), field)
			if maximum, text := vprocore.SoilChildTextMaximum[kind][field.Column]; text {
				value := " raw' \U0001F332 "
				if maximum > 0 {
					value = strings.Repeat("x", maximum)
				}
				update.Text[property] = vprocore.SoilTextUpdate{Value: &value}
			} else {
				value := -1.1234567890123
				if vprocore.SoilChildIntegerColumns[kind][field.Column] {
					value = -32768
				}
				update.Numbers[property] = vprocore.SoilNumberUpdate{Value: &value}
			}
		}
		updates = append(updates, update)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	if _, err := db.Exec(`CREATE TRIGGER fail_soil_batch BEFORE INSERT ON Sample_Audit
		WHEN NEW."Table"='_Mineral' BEGIN SELECT RAISE(ABORT,'injected soil audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSoilRecords("CHILD1", updates); err == nil {
		t.Fatal("cross-table audit failure accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_soil_batch`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSoilRecords("CHILD1", updates); err != nil {
		t.Fatal(err)
	}
	if vprotest.AuditCount(t, db, "CHILD1") != 30 {
		t.Fatal("incorrect exact batch audit count")
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if err := s.UpdateSoilRecords("CHILD1", updates); err == nil {
		t.Fatal("stale cross-table batch accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	for _, update := range updates {
		for property, change := range update.Text {
			update.Text[property] = vprocore.SoilTextUpdate{Expected: change.Value}
		}
		for property, change := range update.Numbers {
			update.Numbers[property] = vprocore.SoilNumberUpdate{Expected: change.Value}
		}
	}
	if err := s.UpdateSoilRecords("CHILD1", updates); err != nil {
		t.Fatal(err)
	}
	if vprotest.AuditCount(t, db, "CHILD1") != 60 {
		t.Fatal("incorrect NULL audit count")
	}
	for _, kind := range []string{"Humus", "Mineral"} {
		record := loadedChildRecord(t, s, kind, "CHILD1")
		for _, field := range vprocore.ChildFields[kind] {
			if !record.FieldByName(field.Member).IsNil() {
				t.Fatalf("NULL clearing failed for %s.%s", kind, field.Column)
			}
		}
	}
}

func TestSoilPatchIdentityOwnershipTypesAndHistoricalOmission(t *testing.T) {
	s, db := vprotest.ChildFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Humus (PlotNumber,ID,Horizon,UpperDepth) VALUES ('CHILD1',-9,?,?)`,
		strings.Repeat("h", 9), math.MaxFloat32*2.0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER forbid_soil_history BEFORE UPDATE OF Horizon,UpperDepth ON Sample_Humus
		BEGIN SELECT RAISE(ABORT,'unchanged history must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	horizon, depth := strings.Repeat("h", 9), math.MaxFloat32*2.0
	update := soilTextPatch("Humus", -9, "rootsSize", nil, vprotest.QualityString("raw"))
	update.Text["horizon"] = vprocore.SoilTextUpdate{Expected: &horizon, Value: &horizon}
	update.Numbers = map[string]vprocore.SoilNumberUpdate{"upperDepth": {Expected: &depth, Value: &depth}}
	if err := s.UpdateSoilRecords("CHILD1", []vprocore.SoilRecordUpdate{update}); err != nil {
		t.Fatal(err)
	}
	if vprotest.AuditCount(t, db, "CHILD1") != 1 {
		t.Fatal("unchanged historical patch emitted audit")
	}
	clear := soilTextPatch("Humus", -9, "rootsSize", vprotest.QualityString("raw"), nil)
	before := vprotest.SubstrateSnapshot(t, db)
	number := 1.5
	for _, changes := range [][]vprocore.SoilRecordUpdate{
		nil, {{}}, {clear, clear}, {soilTextPatch("Other", -9, "dataName", nil, nil)},
		{soilTextPatch("Humus", 999, "rootsSize", nil, nil)},
		{soilTextPatch("Humus", -9, "unknown", nil, nil)},
		{soilTextPatch("Humus", -9, "vonPost", nil, nil)},
		{clear, {Kind: "Mineral", ID: 0, Text: map[string]vprocore.SoilTextUpdate{"horizon": {}}}},
		{{Kind: "Humus", ID: -9, Numbers: map[string]vprocore.SoilNumberUpdate{"vonPost": {Value: &number}}}},
		{{Kind: "Humus", ID: -9, Text: clear.Text, Numbers: map[string]vprocore.SoilNumberUpdate{"rootsSize": {}}}},
	} {
		if err := s.UpdateSoilRecords("CHILD1", changes); err == nil {
			t.Fatalf("invalid batch accepted: %+v", changes)
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
	if err := s.UpdateSoilRecords("CHILD2", []vprocore.SoilRecordUpdate{clear}); err == nil {
		t.Fatal("foreign plot identity accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if err := s.UpdateSoilRecords("CHILD1", []vprocore.SoilRecordUpdate{clear}); err != nil {
		t.Fatal(err)
	}
}

func TestSoilPatchExplicitTransportAndRawUnicode(t *testing.T) {
	for _, payload := range []string{
		`{}`, `{"id":null,"kind":"Humus"}`, `{"id":0}`, `{"id":0,"kind":null}`,
		`{"id":0,"kind":"Humus","text":{"horizon":{}}}`,
		`{"id":0,"kind":"Humus","numbers":{"vonPost":{"value":1}}}`,
		`{"id":0,"kind":"Humus","numbers":{"upperDepth":{"value":"1","expected":null}}}`,
		`{"id":0,"kind":"Humus","text":{"horizon":{"value":"\ud800","value":"valid","expected":null}}}`,
		`{"id":0,"kind":"Humus","text":{"horizon":{"value":null,"expected":"\udfff"}}}`,
	} {
		if err := json.Unmarshal([]byte(payload), &vprocore.SoilRecordUpdate{}); err == nil {
			t.Fatalf("invalid transport accepted: %s", payload)
		}
	}
	var update vprocore.SoilRecordUpdate
	if err := json.Unmarshal([]byte(`{"kind":"Humus","id":0,"text":{"horizon":{"value":null,"expected":null}}}`), &update); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update.Text["horizon"], vprocore.SoilTextUpdate{}) {
		t.Fatal("explicit NULL distinction lost")
	}
}
