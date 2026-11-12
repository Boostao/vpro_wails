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

func TestGeologyPhysicalWriteRoutesAndIndependentRawValues(t *testing.T) {
	for _, field := range vprocore.GeologyHeaderFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, _ := vprotest.HeaderFixture(t)
			if err := os.WriteFile(filepath.Join(s.Projects.Root, "geology-codes.db"), []byte("unavailable"), 0600); err != nil {
				t.Fatal(err)
			}
			values := []*string{nil, vprotest.QualityString("ZZZZ"), vprotest.QualityString("aB"), vprotest.QualityString("q'X4"),
				vprotest.QualityString(" "), vprotest.QualityString(" a "), vprotest.QualityString("\r\nx"), vprotest.QualityString("e\u0301"),
				vprotest.QualityString("A\u00e9\u4e2dB"), vprotest.QualityString("a\U0001F600b"), vprotest.QualityString("\U0001F600\U0001F600"),
				vprotest.QualityString("\uFFFD"), vprotest.QualityString(`\u`)}
			for route, create := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
				for i, value := range values {
					h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("GEO-%s-%s-%d", field.Name, route, i),
						BedrockGeology1: vprotest.QualityString("AB"), BedrockGeology2: vprotest.QualityString("CD"), BedrockGeology3: vprotest.QualityString("EF")}
					setSiteCode(&h, field.Name, value)
					if err := create(h); err != nil {
						t.Fatal(err)
					}
					for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSiteCode(&h, field.Name, vprotest.QualityString("XY"))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSiteCode(&h, field.Name, value)
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						got, err := s.GetPlot(h.PlotNumber)
						if err != nil || !reflect.DeepEqual(*got, h) {
							t.Fatal("raw text/NULL/independent partners changed", err)
						}
					}
				}
			}
		})
	}
}

func TestGeologyPhysicalRejectsWritesAndRawJSONEveryOccurrence(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "GEO-invalid"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, field := range vprocore.GeologyHeaderFields(h) {
		for _, invalid := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff}), string([]byte{0xed, 0xa0, 0x80})} {
			next := h
			setSiteCode(&next, field.Name, &invalid)
			for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.Name) {
					t.Fatal("invalid update accepted", err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "GEO-new"
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("invalid create accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
		for _, key := range []string{field.Property, field.Name, strings.ToUpper(field.Property), `\u0062` + field.Property[1:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.Property + `":"ok"}`, `{"` + field.Property + `":"ok","` + key + `":` + raw + `}`} {
					target := h
					if err := json.Unmarshal([]byte(data), &target); err == nil || !reflect.DeepEqual(target, h) {
						t.Fatal("malformed token repaired/hidden or destination changed")
					}
				}
			}
			for _, raw := range []string{`null`, `"aB"`, `"\ufffd"`, `"a\ud83d\ude00b"`, `"O'"`, `"e\u0301"`, `" \n"`} {
				var target vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &target); err != nil {
					t.Fatal(err)
				}
			}
			for _, raw := range []string{`7`, `true`, `[]`, `{}`} {
				var target vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &target); err == nil {
					t.Fatal("invalid nullable string type accepted")
				}
			}
		}
		for _, raw := range []string{`""`, `"12345"`, `"abc\ud83d\ude00"`} {
			var target vprocore.FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"GEO-json","`+field.Property+`":`+raw+`}`), &target); err != nil {
				t.Fatal(err)
			}
			if err := s.SavePlot(target); err == nil {
				t.Fatal("JSON bypassed storage boundary")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestGeologyPhysicalHistoricalOmissionAndSchemaPending(t *testing.T) {
	for _, field := range vprocore.GeologyHeaderFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "GEO-history"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			column := vprocore.QuoteHeaderIdentifier(field.Name)
			for _, historical := range []string{"", "historical-too-long", "aB", "ZZZZ", string([]byte{0xff})} {
				if _, err := db.Exec(`UPDATE Sample_Env SET `+column+`=? WHERE PlotNumber=?`, historical, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				loaded, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER geo_untouched BEFORE UPDATE OF ` + column + ` ON Sample_Env BEGIN SELECT RAISE(ABORT,'unchanged geology assigned'); END`); err != nil {
					t.Fatal(err)
				}
				before := vprotest.SubstrateSnapshot(t, db)
				for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
					loaded.FieldNotes = vprotest.QualityString("unrelated")
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					before = vprotest.SubstrateSnapshot(t, db)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.Name).Interface(), &historical) {
					t.Fatal("historical bytes changed", err)
				}
				if _, err := db.Exec(`DROP TRIGGER geo_untouched`); err != nil {
					t.Fatal(err)
				}
				if historical == "" || historical == "historical-too-long" {
					data, err := json.Marshal(loaded)
					if err != nil || json.Unmarshal(data, &h) != nil || s.SavePlot(h) != nil {
						t.Fatal("unchanged historical JSON failed", err)
					}
				}
				setSiteCode(loaded, field.Name, nil)
				if err := s.UpdatePlot(*loaded); err != nil {
					t.Fatal("historical NULL clear failed", err)
				}
			}
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetHeaderCapabilities()
			if err != nil || caps[field.Property] {
				t.Fatal("missing schema capability enabled")
			}
			h = vprocore.FS882Header{PlotNumber: h.PlotNumber, FieldNotes: vprotest.QualityString("schema independent")}
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("schema pending blocked unrelated save", err)
			}
			setSiteCode(&h, field.Name, vprotest.QualityString("AB"))
			before := vprotest.SubstrateSnapshot(t, db)
			if err := s.UpdatePlot(h); err == nil {
				t.Fatal("missing column write accepted")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		})
	}
}

func TestGeologyPhysicalAtomicWriteAuditAndStrictIdentity(t *testing.T) {
	for _, create := range []bool{false, true} {
		for _, trigger := range []string{
			`CREATE TRIGGER geo_reject BEFORE INSERT ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
			`CREATE TRIGGER geo_reject BEFORE UPDATE OF BedrockGeology3 ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
			`CREATE TRIGGER geo_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='BedrockGeology3' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
		} {
			if create && strings.Contains(trigger, "BEFORE UPDATE") || !create && strings.Contains(trigger, "BEFORE INSERT ON Sample_Env") {
				continue
			}
			t.Run(fmt.Sprintf("%t/%s", create, trigger), func(t *testing.T) {
				s, db := vprotest.HeaderFixture(t)
				h := vprocore.FS882Header{PlotNumber: "GEO-atomic", BedrockGeology1: vprotest.QualityString("AB"), BedrockGeology2: vprotest.QualityString("CD"), BedrockGeology3: vprotest.QualityString("EF")}
				if !create {
					s.SetAuditStrength(0)
					if err := s.CreatePlot(h); err != nil {
						t.Fatal(err)
					}
				}
				s.SetAuditStrength(3)
				next := h
				next.BedrockGeology1, next.BedrockGeology2, next.BedrockGeology3 = vprotest.QualityString("aB"), vprotest.QualityString("q'X4"), vprotest.QualityString("ZZZZ")
				next.OfficeNotes = vprotest.QualityString("admin partner")
				if _, err := db.Exec(trigger); err != nil {
					t.Fatal(err)
				}
				before := vprotest.SubstrateSnapshot(t, db)
				save := s.UpdatePlot
				if create {
					save = s.CreatePlot
				}
				if err := save(next); err == nil {
					t.Fatal("failed transaction accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
				if _, err := db.Exec(`DROP TRIGGER geo_reject`); err != nil {
					t.Fatal(err)
				}
				if err := save(next); err != nil {
					t.Fatal("retry failed", err)
				}
				before = vprotest.SubstrateSnapshot(t, db)
				if err := s.CreatePlot(next); err == nil {
					t.Fatal("collision accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
				next.PlotNumber = "GEO-absent"
				if err := s.UpdatePlot(next); err == nil {
					t.Fatal("stale identity accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			})
		}
	}
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "GEO-audit", BedrockGeology1: vprotest.QualityString("AB")}
	s.SetAuditStrength(0)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	for _, value := range []*string{vprotest.QualityString("ab"), vprotest.QualityString("q'X4"), nil} {
		h.BedrockGeology1 = value
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SavePlot(h); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ListAuditEntries(h.PlotNumber)
	if err != nil || len(entries) != 3 {
		t.Fatal("phantom/noop or missing mapped audits", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND EditField='BedrockGeology1' AND BeforeEdit='AB' AND AfterEdit='ab'`, h.PlotNumber).Scan(&count); err != nil || count != 1 {
		t.Fatal("exact case audit lost")
	}
	for _, entry := range entries {
		if entry.Table != "_Env" || entry.EditField != "BedrockGeology1" || entry.ID != nil || entry.BeforeEdit == nil && entry.AfterEdit == nil {
			t.Fatal("unmapped/phantom audit")
		}
	}
}
