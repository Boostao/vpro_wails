package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestVegetationPhysicalDomainsAndRawJSON(t *testing.T) {
	for _, field := range childFields["Veg"] {
		t.Run(field.column, func(t *testing.T) {
			s, db := childFixture(t)
			record := childRecord("Veg", "CHILD1", 0, false)
			member := record.FieldByName(field.member)
			scalar := member.Type()
			if scalar.Kind() == reflect.Pointer {
				scalar = scalar.Elem()
			}
			var invalid []any
			switch scalar.Kind() {
			case reflect.String:
				maximum, found := vegetationTextMaximum[field.column]
				if !found {
					t.Fatal("missing source text bound")
				}
				invalid = []any{string([]byte{255}), strings.Repeat("x", maximum+1),
					strings.Repeat("x", maximum-1) + "\U0001F332"}
				exact := strings.Repeat("x", maximum)
				if maximum >= 2 {
					exact = strings.Repeat("x", maximum-2) + "\U0001F332"
				}
				for _, text := range []string{exact, "", " "} {
					if err := validateVegetationField(field.column, text); err != nil {
						t.Fatal("physical text bound or invented normalization:", err)
					}
				}
				if field.column != "Species" {
					if err := validateVegetationField(field.column, nil); err != nil {
						t.Fatal(err)
					}
				} else if err := validateVegetationField(field.column, nil); err == nil {
					t.Fatal("required Species accepted NULL")
				}
				property := childJSONKey(record, field)
				folded := strings.ReplaceAll(property, "s", "\u017f")
				for _, raw := range []string{
					`"` + property + `":"\ud800"`,
					`"` + property + `":"bad` + string([]byte{255}) + `"`,
					`"` + property + `":"\udfff","` + property + `":"valid"`,
					`"` + folded + `":"\ud800","` + property + `":"valid"`,
				} {
					if err := json.Unmarshal([]byte(`{`+raw+`}`), &VegRecord{}); err == nil {
						t.Fatal("malformed raw text repaired before rejection")
					}
				}
			case reflect.Float64:
				invalid = []any{math.MaxFloat32 * 2.0, -math.MaxFloat32 * 2.0, math.NaN(), math.Inf(1)}
				for _, number := range []float64{-math.MaxFloat32, math.MaxFloat32, -1.1234567890123, 0, 101} {
					if err := validateVegetationField(field.column, number); err != nil {
						t.Fatal("invented physical range/rounding:", err)
					}
				}
			case reflect.Int:
				minimum, maximum := -32768, 32767
				if field.column == "LL" || field.column == "PV" {
					minimum, maximum = math.MinInt32, math.MaxInt32
				}
				invalid = []any{minimum - 1, maximum + 1}
				for _, number := range []int{minimum, maximum, -1, 0, 101} {
					if err := validateVegetationField(field.column, number); err != nil {
						t.Fatal("incorrect source integer range:", err)
					}
				}
			default:
				t.Fatal("unclassified mapped physical type")
			}
			for _, entry := range invalid {
				if member.Kind() == reflect.Pointer {
					pointer := reflect.New(scalar)
					pointer.Elem().Set(reflect.ValueOf(entry))
					member.Set(pointer)
				} else {
					member.Set(reflect.ValueOf(entry))
				}
				before := substrateSnapshot(t, db)
				if err := s.SaveVegRecord(record.Interface().(VegRecord)); err == nil {
					t.Fatalf("create accepted invalid Veg.%s: %v", field.column, entry)
				}
				assertSubstrateSnapshot(t, db, before)
			}
		})
	}
}

func TestVegetationHistoricalOmissionAndFreshRestoreAliases(t *testing.T) {
	for _, field := range childFields["Veg"] {
		t.Run(field.column, func(t *testing.T) {
			s, db := childFixture(t)
			if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD1", Species: "RAW"}); err != nil {
				t.Fatal(err)
			}
			id := childID(t, db, "Veg", "CHILD1")
			var historical, invalid any
			switch {
			case vegetationSingleColumns[field.column]:
				historical, invalid = math.MaxFloat32*2.0, math.MaxFloat32*3.0
			case vegetationIntegerColumns[field.column]:
				historical, invalid = 32768, 32769
			case field.column == "LL" || field.column == "PV":
				historical, invalid = int(math.MaxInt32)+1, int(math.MaxInt32)+2
			default:
				maximum := vegetationTextMaximum[field.column]
				historical, invalid = strings.Repeat("h", maximum+1), strings.Repeat("i", maximum+2)
			}
			if _, err := db.Exec(`UPDATE Sample_Veg SET "`+field.column+`"=? WHERE ID=?`, historical, id); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TRIGGER forbid_veg_history BEFORE UPDATE OF "` + field.column +
				`" ON Sample_Veg BEGIN SELECT RAISE(ABORT,'unchanged history must be omitted'); END`); err != nil {
				t.Fatal(err)
			}
			record := loadedChildRecord(t, s, "Veg", "CHILD1")
			changed := "Layer"
			if field.column == changed {
				changed = "Collected"
			}
			record.FieldByName(changed).Set(reflect.ValueOf(qualityString("r")))
			for _, update := range []bool{false, true} {
				if err := saveVegetationRecord(s, record.Interface().(VegRecord), update); err != nil {
					t.Fatal("unchanged historical assignment:", err)
				}
			}
			member := record.FieldByName(field.member)
			if member.Kind() == reflect.Pointer {
				pointer := reflect.New(member.Type().Elem())
				pointer.Elem().Set(reflect.ValueOf(invalid))
				member.Set(pointer)
			} else {
				member.Set(reflect.ValueOf(invalid))
			}
			before := substrateSnapshot(t, db)
			for _, update := range []bool{false, true} {
				if err := saveVegetationRecord(s, record.Interface().(VegRecord), update); err == nil {
					t.Fatal("existing-row route accepted fresh invalid value")
				}
				assertSubstrateSnapshot(t, db, before)
			}
			for _, alias := range []string{"_Veg", "Sample_Veg", "_veg", "sAmPlE_VEG"} {
				rowID := restoreAuditFixture(t, db, "CHILD1", alias, strings.ToLower(field.column), &id, invalid, historical)
				before := substrateSnapshot(t, db)
				if err := s.SetAuditRestoreSelection("CHILD1", []string{rowID}); err == nil {
					t.Fatal("invalid fresh restoration selected")
				}
				if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{rowID}, AuditRestoreRetain); err == nil {
					t.Fatal("invalid fresh restoration accepted")
				}
				assertSubstrateSnapshot(t, db, before)
			}
		})
	}
}

func saveVegetationRecord(s *PlotService, record VegRecord, update bool) error {
	if update {
		return s.UpdateVegRecord(record)
	}
	return s.SaveVegRecord(record)
}
