package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestVegetationAttributesAtomicExpectedPatchesAndPhysicalDomains(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	for _, id := range []int{0, -9} {
		if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,Species,ID) VALUES ('CHILD1','RAW',?)`, id); err != nil {
			t.Fatal(err)
		}
	}
	var updates []VegetationAttributeUpdate
	for _, id := range []int{0, -9} {
		update := VegetationAttributeUpdate{ID: id, Values: map[string]*float64{}, Expected: map[string]*float64{}}
		for property := range vegetationAttributeProperties {
			value := -32768.0
			if property == "ll" || property == "pv" {
				value = -2147483648
			}
			update.Values[property], update.Expected[property] = &value, nil
		}
		updates = append(updates, update)
	}
	before := substrateSnapshot(t, db)
	if _, err := db.Exec(`CREATE TRIGGER fail_attribute_batch BEFORE INSERT ON Sample_Audit
		WHEN NEW.ID=-9 BEGIN SELECT RAISE(ABORT,'injected attribute audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationAttributes("CHILD1", updates); err == nil {
		t.Fatal("partial multirow commit accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_attribute_batch`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegetationAttributes("CHILD1", updates); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, db, "CHILD1") != 24 {
		t.Fatal("incorrect exact audit count")
	}
	before = substrateSnapshot(t, db)
	if err := s.UpdateVegetationAttributes("CHILD1", updates); err == nil {
		t.Fatal("stale attributes accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	for _, update := range updates {
		for property, value := range update.Values {
			update.Expected[property], update.Values[property] = value, nil
		}
	}
	if err := s.UpdateVegetationAttributes("CHILD1", updates); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, db, "CHILD1") != 48 {
		t.Fatal("incorrect explicit NULL count")
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET AF=32768,LL=2147483648 WHERE ID=0;
		CREATE TRIGGER forbid_attribute_history BEFORE UPDATE OF AF,LL ON Sample_Veg
		BEGIN SELECT RAISE(ABORT,'history assignment must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	supplied := VegetationAttributeUpdate{ID: 0, Values: map[string]*float64{
		"af": heightFloat(32768), "ll": heightFloat(2147483648), "dc": heightFloat(1),
	}, Expected: map[string]*float64{"af": heightFloat(32768), "ll": heightFloat(2147483648), "dc": nil}}
	if err := s.UpdateVegetationAttributes("CHILD1", []VegetationAttributeUpdate{supplied}); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	for _, batch := range [][]VegetationAttributeUpdate{
		nil, {{}}, {supplied, supplied},
		{{ID: 0, Values: map[string]*float64{"height1": nil}, Expected: map[string]*float64{"height1": nil}}},
		{{ID: 0, Values: map[string]*float64{"af": heightFloat(32769)}, Expected: map[string]*float64{"af": heightFloat(32768)}}},
		{{ID: 0, Values: map[string]*float64{"ll": heightFloat(2147483649)}, Expected: map[string]*float64{"ll": heightFloat(2147483648)}}},
		{{ID: -9, Values: map[string]*float64{"dc": heightFloat(1.5)}, Expected: map[string]*float64{"dc": nil}}},
		{{ID: -9, Values: map[string]*float64{"dc": nil}, Expected: map[string]*float64{"af": nil}}},
		{{ID: 999, Values: map[string]*float64{"dc": nil}, Expected: map[string]*float64{"dc": nil}}},
	} {
		if err := s.UpdateVegetationAttributes("CHILD1", batch); err == nil {
			t.Fatal("invalid attribute patch accepted:", batch)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if err := s.UpdateVegetationAttributes("CHILD2", updates); err == nil {
		t.Fatal("foreign identities accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestVegetationAttributeTransportAndCanonicalSuggestions(t *testing.T) {
	for _, payload := range []string{`{}`, `{"id":null}`, `{"id":0.5}`, `{"id":0,"values":{"af":"1"}}`} {
		if err := json.Unmarshal([]byte(payload), &VegetationAttributeUpdate{}); err == nil {
			t.Fatal("invalid identity/value transport accepted:", payload)
		}
	}
	var update VegetationAttributeUpdate
	if err := json.Unmarshal([]byte(`{"id":0,"values":{"af":null},"expected":{"af":null}}`), &update); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(update.Values, map[string]*float64{"af": nil}) {
		t.Fatal("explicit NULL was lost")
	}
	service, state := contextServiceFixture(t)
	rows, err := service.ListVegetationAttributeSuggestions(context.Background(), state.ContextID)
	if err != nil || len(rows) != 145 {
		t.Fatalf("canonical full metadata: count=%d %v", len(rows), err)
	}
	last := ""
	for _, row := range rows {
		if row.ListName != nil && *row.ListName == "phenologyCodeVeg" && row.Item != nil {
			if *row.Item < last {
				t.Fatal("PV source Item ordering replaced by ItemOrder")
			}
			last = *row.Item
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListVegetationAttributeSuggestions(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled attribute reference read accepted:", err)
	}
}
