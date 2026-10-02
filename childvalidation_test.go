package main

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func newSoilChildRecord(kind string) reflect.Value {
	if kind == "Humus" {
		return reflect.ValueOf(&HumusRecord{PlotNumber: "CHILD1"}).Elem()
	}
	return reflect.ValueOf(&MineralRecord{PlotNumber: "CHILD1"}).Elem()
}

func saveSoilChildRecord(s *PlotService, record any, update bool) error {
	switch entry := record.(type) {
	case HumusRecord:
		if update {
			return s.UpdateHumusRecord(entry)
		}
		return s.SaveHumusRecord(entry)
	case MineralRecord:
		if update {
			return s.UpdateMineralRecord(entry)
		}
		return s.SaveMineralRecord(entry)
	default:
		panic("unexpected soil record type")
	}
}

func TestSoilChildPhysicalDomainsAndRawJSON(t *testing.T) {
	for _, kind := range []string{"Humus", "Mineral"} {
		t.Run(kind, func(t *testing.T) {
			for _, field := range childFields[kind] {
				t.Run(field.column, func(t *testing.T) {
					s, db := childFixture(t)
					record := newSoilChildRecord(kind)
					member := record.FieldByName(field.member)
					var invalid []any
					switch {
					case member.Type().Elem().Kind() == reflect.String:
						maximum, found := soilChildTextMaximum[kind][field.column]
						if !found {
							t.Fatal("missing source text domain")
						}
						invalid = []any{string([]byte{255})}
						if maximum > 0 {
							invalid = append(invalid, strings.Repeat("x", maximum+1), strings.Repeat("x", maximum-1)+"\U0001F332")
							exact := strings.Repeat("x", maximum)
							if maximum >= 2 {
								exact = strings.Repeat("x", maximum-2) + "\U0001F332"
							}
							if err := validateSoilChildField(kind, field.column, exact); err != nil {
								t.Fatal(err)
							}
						} else if err := validateSoilChildField(kind, field.column, strings.Repeat("\U0001F332", 4096)); err != nil {
							t.Fatal("invented memo bound:", err)
						}
						for _, valid := range []any{nil, "", " "} {
							if err := validateSoilChildField(kind, field.column, valid); err != nil {
								t.Fatal("invented text normalization/domain:", err)
							}
						}
						property := childJSONKey(record, field)
						for _, raw := range []string{
							`"\ud800"`, `"\udfff"`, `"bad` + string([]byte{255}) + `"`,
							`"\ud800","` + property + `":"valid"`,
						} {
							payload := []byte(`{"` + property + `":` + raw + `}`)
							if err := json.Unmarshal(payload, record.Addr().Interface()); err == nil {
								t.Fatalf("raw malformed Unicode repaired for %q", property)
							}
						}
					case member.Type().Elem().Kind() == reflect.Float64:
						invalid = []any{math.MaxFloat32 * 2.0, -math.MaxFloat32 * 2.0, math.Inf(1), math.Inf(-1), math.NaN()}
						for _, number := range []float64{-math.MaxFloat32, math.MaxFloat32, -1.1234567890123, 0, 101} {
							if err := validateSoilChildField(kind, field.column, number); err != nil {
								t.Fatal("invented numeric domain or rounding:", err)
							}
						}
					case member.Type().Elem().Kind() == reflect.Int:
						invalid = []any{-32769, 32768}
						for _, number := range []int{-32768, 32767, -1, 0, 101} {
							if err := validateSoilChildField(kind, field.column, number); err != nil {
								t.Fatal("invented integer domain:", err)
							}
						}
					default:
						t.Fatal("unclassified source domain")
					}
					for _, entry := range invalid {
						pointer := reflect.New(member.Type().Elem())
						pointer.Elem().Set(reflect.ValueOf(entry))
						member.Set(pointer)
						before := substrateSnapshot(t, db)
						if err := saveSoilChildRecord(s, record.Interface(), false); err == nil {
							t.Fatalf("create accepted invalid %s.%s: %v", kind, field.column, entry)
						}
						assertSubstrateSnapshot(t, db, before)
					}
				})
			}
		})
	}
}

func TestSoilChildHistoricalOmissionUpdatesAndRestorationAliases(t *testing.T) {
	for _, kind := range []string{"Humus", "Mineral"} {
		for _, field := range childFields[kind] {
			t.Run(kind+"/"+field.column, func(t *testing.T) {
				s, db := childFixture(t)
				if err := saveSoilChildRecord(s, newSoilChildRecord(kind).Interface(), false); err != nil {
					t.Fatal(err)
				}
				id := childID(t, db, kind, "CHILD1")
				var historical, invalid any
				switch {
				case soilChildSingleColumns[kind][field.column]:
					historical, invalid = math.MaxFloat32*2.0, math.MaxFloat32*3.0
				case soilChildIntegerColumns[kind][field.column]:
					historical, invalid = 32768, 32769
				default:
					maximum := soilChildTextMaximum[kind][field.column]
					if maximum == 0 {
						historical, invalid = string([]byte{255}), string([]byte{254})
					} else {
						historical, invalid = strings.Repeat("h", maximum+1), strings.Repeat("i", maximum+2)
					}
				}
				if _, err := db.Exec(`UPDATE "Sample_`+kind+`" SET "`+field.column+`"=? WHERE ID=?`, historical, id); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER forbid_historical_assignment BEFORE UPDATE OF "` + field.column +
					`" ON "Sample_` + kind + `" BEGIN SELECT RAISE(ABORT,'historical assignment must be omitted'); END`); err != nil {
					t.Fatal(err)
				}
				record := loadedChildRecord(t, s, kind, "CHILD1")
				changed := "Horizon"
				if field.column == changed {
					changed = "RootsSize"
				}
				record.FieldByName(changed).Set(reflect.ValueOf(qualityString("raw")))
				for _, update := range []bool{false, true} {
					if err := saveSoilChildRecord(s, record.Interface(), update); err != nil {
						t.Fatal("unchanged historical assignment:", err)
					}
				}
				member := record.FieldByName(field.member)
				pointer := reflect.New(member.Type().Elem())
				pointer.Elem().Set(reflect.ValueOf(invalid))
				member.Set(pointer)
				before := substrateSnapshot(t, db)
				for _, update := range []bool{false, true} {
					if err := saveSoilChildRecord(s, record.Interface(), update); err == nil {
						t.Fatal("existing-row route accepted new invalid value")
					}
					assertSubstrateSnapshot(t, db, before)
				}
				for _, alias := range []string{"_" + kind, "Sample_" + kind, "_" + strings.ToLower(kind), "sAmPlE_" + strings.ToUpper(kind)} {
					rowID := restoreAuditFixture(t, db, "CHILD1", alias, strings.ToLower(field.column), &id,
						invalid, historical)
					before = substrateSnapshot(t, db)
					if err := s.SetAuditRestoreSelection("CHILD1", []string{rowID}); err == nil {
						t.Fatal("invalid restoration selected")
					}
					if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{rowID}, AuditRestoreRetain); err == nil {
						t.Fatal("invalid fresh restoration accepted")
					}
					assertSubstrateSnapshot(t, db, before)
				}
			})
		}
	}
}

func TestChildUnicodeFoldedTextAndExplicitNullPresence(t *testing.T) {
	for _, kind := range []string{"Humus", "Mineral"} {
		record := newSoilChildRecord(kind)
		for property := range soilChildJSONTextNames(kind, record.Interface()) {
			folded := strings.ReplaceAll(property, "s", "\u017f")
			payload := []byte(`{"` + folded + `":"\ud800","` + property + `":"valid"}`)
			if err := json.Unmarshal(payload, record.Addr().Interface()); err == nil {
				t.Fatal("Unicode identifier folding bypassed raw token guard")
			}
		}
		if err := json.Unmarshal([]byte(`{"rootſSize":null}`), record.Addr().Interface()); err != nil {
			t.Fatal(err)
		}
		if !childPresence(record.Interface())["rootssize"] {
			t.Fatal("explicit folded NULL was treated as omission")
		}
	}
}
