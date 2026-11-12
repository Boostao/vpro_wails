package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	if len(vprocore.ParentCodeFields(vprocore.FS882Header{})) != len(want) {
		t.Fatal("descriptor count")
	}
	for _, field := range vprocore.ParentCodeFields(vprocore.FS882Header{}) {
		member, exists := reflect.TypeOf(vprocore.FS882Header{}).FieldByName(field.Name)
		if !exists || member.Type != reflect.TypeOf((*string)(nil)) || member.Tag.Get("json") != field.Property ||
			field.Table != "Env" || field.Maximum != want[field.Name] {
			t.Fatal("source typed descriptor/DDL mismatch", field.Name)
		}
		delete(want, field.Name)
	}
	if len(want) != 0 || vprocore.IsParentCodeProperty("becSiteUnit") || vprocore.IsParentCodeProperty("bedrockGeology1") || vprocore.IsParentCodeProperty("soilDrainage") {
		t.Fatal("excluded or source-held field guarded")
	}
}

func TestParentCodeAllFieldSaveRoutesAndUTF16Bounds(t *testing.T) {
	for _, field := range vprocore.ParentCodeFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			if err := os.WriteFile(filepath.Join(s.Projects.Root, "parent-codes.db"), []byte("unavailable catalogue"), 0600); err != nil {
				t.Fatal(err)
			}
			maximum := strings.Repeat("x", field.Maximum)
			values := []*string{nil, vprotest.QualityString("X"), vprotest.QualityString(" "), &maximum}
			if field.Maximum >= 2 {
				values = append(values, vprotest.QualityString("e\u0301"), vprotest.QualityString("\U0001F600"))
			}
			if field.Maximum >= 4 {
				values = append(values, vprotest.QualityString("q'X4"), vprotest.QualityString("a\U0001F600b"), vprotest.QualityString("\r\nx"))
			}
			for i, value := range values {
				for route, create := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
					h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("P-%s-%s-%d", field.Name, route, i)}
					setSiteCode(&h, field.Name, value)
					if err := create(h); err != nil {
						t.Fatal("independent manual create", err)
					}
					for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSiteCode(&h, field.Name, vprotest.QualityString("z"))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSiteCode(&h, field.Name, value)
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
			h := vprocore.FS882Header{PlotNumber: "P-invalid"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			for _, invalid := range []string{"", maximum + "x", strings.Repeat("x", field.Maximum-1) + "\U0001F600", string([]byte{0xff})} {
				next := h
				setSiteCode(&next, field.Name, &invalid)
				for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(next); err == nil || !strings.Contains(err.Error(), field.Name) {
						t.Fatal("invalid update accepted", err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
				}
				next.PlotNumber = "P-invalid-new"
				for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
					if err := create(next); err == nil {
						t.Fatal("invalid create accepted")
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
				}
			}
		})
	}
}

func TestParentCodeRawJSONEveryOccurrenceAndPhysicalBoundary(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	for _, field := range vprocore.ParentCodeFields(vprocore.FS882Header{}) {
		for _, key := range []string{field.Property, field.Name, strings.ToUpper(field.Property),
			fmt.Sprintf(`\u%04x`, field.Property[0]) + field.Property[1:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, input := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.Property + `":"X"}`, `{"` + field.Property + `":"X","` + key + `":` + raw + `}`} {
					h := vprocore.FS882Header{PlotNumber: "untouched"}
					before := h
					if err := json.Unmarshal([]byte(input), &h); err == nil || !reflect.DeepEqual(h, before) {
						t.Fatal("malformed token repaired/hidden")
					}
				}
			}
			for _, raw := range []string{`null`, `"X"`, `"a\ud83d\ude00b"`, `"e\u0301"`} {
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &h); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, raw := range []string{`""`, `"` + strings.Repeat("x", field.Maximum+1) + `"`,
			`"` + strings.Repeat("x", field.Maximum-1) + `\ud83d\ude00"`} {
			var h vprocore.FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"P-json","`+field.Property+`":`+raw+`}`), &h); err != nil {
				t.Fatal(err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			if err := s.SavePlot(h); err == nil {
				t.Fatal("JSON bypassed physical guard")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestParentCodeAtomicWritesExactAuditAndIdentity(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "P-atomic", CoarseFragLith1: vprotest.QualityString("AB"), HumusFormPhase: vprotest.QualityString("phase")}
	s.SetAuditStrength(0)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	next := h
	next.CoarseFragLith1, next.HumusFormPhase, next.OfficeNotes = vprotest.QualityString("rawFULLitem1"), vprotest.QualityString("phase q'X4"), vprotest.QualityString("partner")
	for _, trigger := range []string{
		`CREATE TRIGGER parent_reject BEFORE UPDATE OF HumusFormPhase ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER parent_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'admin rejected'); END`,
		`CREATE TRIGGER parent_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='HumusFormPhase' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		if err := s.UpdatePlot(next); err == nil {
			t.Fatal("failed transaction accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER parent_reject`); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal("raw full item retry failed", err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	if err := s.CreatePlot(next); err == nil {
		t.Fatal("collision accepted")
	}
	next.PlotNumber = "P-absent"
	if err := s.UpdatePlot(next); err == nil {
		t.Fatal("stale identity accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	next.PlotNumber = h.PlotNumber
	next.CoarseFragLith1 = vprotest.QualityString("RAWfullITEM1")
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	next.CoarseFragLith1 = nil
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	count := vprotest.AuditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(next); err != nil || vprotest.AuditCount(t, db, h.PlotNumber) != count {
		t.Fatal("noop audit")
	}
	var exact, phantom int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND EditField='CoarseFragLith1' AND BeforeEdit='rawFULLitem1' AND AfterEdit='RAWfullITEM1'`, h.PlotNumber).Scan(&exact); err != nil || exact != 1 {
		t.Fatal("case/full item audit not exact")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND BeforeEdit IS NULL AND AfterEdit IS NULL`, h.PlotNumber).Scan(&phantom); err != nil || phantom != 0 {
		t.Fatal("phantom audits")
	}
	newHeader := vprocore.FS882Header{PlotNumber: "P-create-fail", CoarseFragLith1: vprotest.QualityString("rawFULLitem1"), HumusFormPhase: vprotest.QualityString("phase")}
	if _, err := db.Exec(`CREATE TRIGGER parent_create_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='HumusFormPhase' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if err := s.CreatePlot(newHeader); err == nil {
		t.Fatal("create audit failure accepted")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestParentCodeHistoricalOmissionSchemaAndClear(t *testing.T) {
	for _, field := range vprocore.ParentCodeFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "P-history"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}

			column := vprocore.QuoteHeaderIdentifier(field.Name)
			for _, historical := range []string{"", strings.Repeat("a", field.Maximum+1), "z", string([]byte{0xff})} {
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
				before := vprotest.SubstrateSnapshot(t, db)
				for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
					loaded.OfficeNotes = vprotest.QualityString("independent Admin")
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					before = vprotest.SubstrateSnapshot(t, db)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.Name).Interface(), &historical) {
					t.Fatal("historical bytes repaired")
				}
				if _, err := db.Exec(`DROP TRIGGER parent_untouched`); err != nil {
					t.Fatal(err)
				}
				setSiteCode(loaded, field.Name, nil)
				if err := s.UpdatePlot(*loaded); err != nil {
					t.Fatal("clear historical value", err)
				}
			}
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetHeaderCapabilities()
			if err != nil || caps[field.Property] {
				t.Fatal("schema capability pending")
			}
			h.FieldNotes = vprotest.QualityString("independent")
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("missing schema blocked unrelated save", err)
			}
			setSiteCode(&h, field.Name, vprotest.QualityString("X"))
			before := vprotest.SubstrateSnapshot(t, db)
			if err := s.UpdatePlot(h); err == nil {
				t.Fatal("missing schema write accepted")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		})
	}
}
