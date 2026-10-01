package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParentCodeDescriptorLimitsAndExclusions(t *testing.T) {
	want := map[string]int{
		"CoarseFragLith1": 12, "CoarseFragLith2": 12, "CoarseFragLith3": 12,
		"FloodingRegimeDur": 2, "FloodingRegimeFreq": 7, "GeoMorProSubSurf": 3, "GeoMorProSurf": 3,
		"HumusForm": 4, "HumusFormPhase": 50, "HydroGeoSubSystem": 2, "HydroGeoSystem": 3, "RealmClass": 5,
		"RootRestrictingType": 1, "RootZoneParticleSize": 6, "SurfaceExpSubSurf": 3, "SurfaceExpSurf": 3,
		"SurficialMaterialSubSurf": 6, "SurficialMaterialSurf": 6, "TerrainTextureSubSurf": 3,
		"TerrainTextureSurf": 3, "WaterSource": 5,
	}
	if len(parentCodeFields(FS882Header{})) != len(want) {
		t.Fatal("descriptor count")
	}
	for _, field := range parentCodeFields(FS882Header{}) {
		member, exists := reflect.TypeOf(FS882Header{}).FieldByName(field.name)
		if !exists || member.Type != reflect.TypeOf((*string)(nil)) || member.Tag.Get("json") != field.property ||
			field.table != "Env" || field.maximum != want[field.name] {
			t.Fatal("source typed descriptor/DDL mismatch", field.name)
		}
		delete(want, field.name)
	}
	if len(want) != 0 || isParentCodeProperty("becSiteUnit") || isParentCodeProperty("bedrockGeology1") || isParentCodeProperty("soilDrainage") {
		t.Fatal("excluded or source-held field guarded")
	}
}

func TestParentCodeAllFieldSaveRoutesAndUTF16Bounds(t *testing.T) {
	for _, field := range parentCodeFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			if err := os.WriteFile(filepath.Join(s.projects.root, "parent-codes.db"), []byte("unavailable catalogue"), 0600); err != nil {
				t.Fatal(err)
			}
			maximum := strings.Repeat("x", field.maximum)
			values := []*string{nil, qualityString("X"), qualityString(" "), &maximum}
			if field.maximum >= 2 {
				values = append(values, qualityString("e\u0301"), qualityString("\U0001F600"))
			}
			if field.maximum >= 4 {
				values = append(values, qualityString("q'X4"), qualityString("a\U0001F600b"), qualityString("\r\nx"))
			}
			for i, value := range values {
				for route, create := range map[string]func(FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
					h := FS882Header{PlotNumber: fmt.Sprintf("P-%s-%s-%d", field.name, route, i)}
					setSiteCode(&h, field.name, value)
					if err := create(h); err != nil {
						t.Fatal("independent manual create", err)
					}
					for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSiteCode(&h, field.name, qualityString("z"))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSiteCode(&h, field.name, value)
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						got, err := s.GetPlot(h.PlotNumber)
						if err != nil || !reflect.DeepEqual(*got, h) {
							t.Fatal("raw case/NULL/partners changed", err)
						}
					}
				}
			}
			h := FS882Header{PlotNumber: "P-invalid"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			for _, invalid := range []string{"", maximum + "x", strings.Repeat("x", field.maximum-1) + "\U0001F600", string([]byte{0xff})} {
				next := h
				setSiteCode(&next, field.name, &invalid)
				for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(next); err == nil || !strings.Contains(err.Error(), field.name) {
						t.Fatal("invalid update accepted", err)
					}
					assertSubstrateSnapshot(t, db, before)
				}
				next.PlotNumber = "P-invalid-new"
				for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
					if err := create(next); err == nil {
						t.Fatal("invalid create accepted")
					}
					assertSubstrateSnapshot(t, db, before)
				}
			}
		})
	}
}

func TestParentCodeRawJSONEveryOccurrenceAndPhysicalBoundary(t *testing.T) {
	s, db := headerFixture(t)
	for _, field := range parentCodeFields(FS882Header{}) {
		for _, key := range []string{field.property, field.name, strings.ToUpper(field.property),
			fmt.Sprintf(`\u%04x`, field.property[0]) + field.property[1:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, input := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.property + `":"X"}`, `{"` + field.property + `":"X","` + key + `":` + raw + `}`} {
					h := FS882Header{PlotNumber: "untouched"}
					before := h
					if err := json.Unmarshal([]byte(input), &h); err == nil || !reflect.DeepEqual(h, before) {
						t.Fatal("malformed token repaired/hidden")
					}
				}
			}
			for _, raw := range []string{`null`, `"X"`, `"a\ud83d\ude00b"`, `"e\u0301"`} {
				var h FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &h); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, raw := range []string{`""`, `"` + strings.Repeat("x", field.maximum+1) + `"`,
			`"` + strings.Repeat("x", field.maximum-1) + `\ud83d\ude00"`} {
			var h FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"P-json","`+field.property+`":`+raw+`}`), &h); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if err := s.SavePlot(h); err == nil {
				t.Fatal("JSON bypassed physical guard")
			}
			assertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestParentCodeAtomicWritesExactAuditAndIdentity(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "P-atomic", CoarseFragLith1: qualityString("AB"), HumusFormPhase: qualityString("phase")}
	s.SetAuditStrength(0)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	next := h
	next.CoarseFragLith1, next.HumusFormPhase, next.OfficeNotes = qualityString("rawFULLitem1"), qualityString("phase q'X4"), qualityString("partner")
	for _, trigger := range []string{
		`CREATE TRIGGER parent_reject BEFORE UPDATE OF HumusFormPhase ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER parent_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'admin rejected'); END`,
		`CREATE TRIGGER parent_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='HumusFormPhase' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := substrateSnapshot(t, db)
		if err := s.UpdatePlot(next); err == nil {
			t.Fatal("failed transaction accepted")
		}
		assertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER parent_reject`); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal("raw full item retry failed", err)
	}
	before := substrateSnapshot(t, db)
	if err := s.CreatePlot(next); err == nil {
		t.Fatal("collision accepted")
	}
	next.PlotNumber = "P-absent"
	if err := s.UpdatePlot(next); err == nil {
		t.Fatal("stale identity accepted")
	}
	assertSubstrateSnapshot(t, db, before)
	next.PlotNumber = h.PlotNumber
	next.CoarseFragLith1 = qualityString("RAWfullITEM1")
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	next.CoarseFragLith1 = nil
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	count := auditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(next); err != nil || auditCount(t, db, h.PlotNumber) != count {
		t.Fatal("noop audit")
	}
	var exact, phantom int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND EditField='CoarseFragLith1' AND BeforeEdit='rawFULLitem1' AND AfterEdit='RAWfullITEM1'`, h.PlotNumber).Scan(&exact); err != nil || exact != 1 {
		t.Fatal("case/full item audit not exact")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND BeforeEdit IS NULL AND AfterEdit IS NULL`, h.PlotNumber).Scan(&phantom); err != nil || phantom != 0 {
		t.Fatal("phantom audits")
	}
	newHeader := FS882Header{PlotNumber: "P-create-fail", CoarseFragLith1: qualityString("rawFULLitem1"), HumusFormPhase: qualityString("phase")}
	if _, err := db.Exec(`CREATE TRIGGER parent_create_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='HumusFormPhase' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before = substrateSnapshot(t, db)
	if err := s.CreatePlot(newHeader); err == nil {
		t.Fatal("create audit failure accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestParentCodeHistoricalOmissionSchemaAndClear(t *testing.T) {
	for _, field := range parentCodeFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "P-history"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}

			column := quoteHeaderIdentifier(field.name)
			for _, historical := range []string{"", strings.Repeat("a", field.maximum+1), "z", string([]byte{0xff})} {
				if _, err := db.Exec(`UPDATE Sample_Env SET `+column+`=? WHERE PlotNumber=?`, historical, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				loaded, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER parent_untouched BEFORE UPDATE OF ` + column + ` ON Sample_Env BEGIN SELECT RAISE(ABORT,'unchanged parent field assigned'); END`); err != nil {
					t.Fatal(err)
				}
				before := substrateSnapshot(t, db)
				for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					assertSubstrateSnapshot(t, db, before)
					loaded.OfficeNotes = qualityString("independent Admin")
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					before = substrateSnapshot(t, db)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), &historical) {
					t.Fatal("historical bytes repaired")
				}
				if _, err := db.Exec(`DROP TRIGGER parent_untouched`); err != nil {
					t.Fatal(err)
				}
				setSiteCode(loaded, field.name, nil)
				if err := s.UpdatePlot(*loaded); err != nil {
					t.Fatal("clear historical value", err)
				}
			}
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetHeaderCapabilities()
			if err != nil || caps[field.property] {
				t.Fatal("schema capability pending")
			}
			h.FieldNotes = qualityString("independent")
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("missing schema blocked unrelated save", err)
			}
			setSiteCode(&h, field.name, qualityString("X"))
			before := substrateSnapshot(t, db)
			if err := s.UpdatePlot(h); err == nil {
				t.Fatal("missing schema write accepted")
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
}
