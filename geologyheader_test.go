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

func TestGeologyPhysicalWriteRoutesAndIndependentRawValues(t *testing.T) {
	for _, field := range geologyHeaderFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, _ := headerFixture(t)
			if err := os.WriteFile(filepath.Join(s.projects.root, "geology-codes.db"), []byte("unavailable"), 0600); err != nil {
				t.Fatal(err)
			}
			values := []*string{nil, qualityString("ZZZZ"), qualityString("aB"), qualityString("q'X4"),
				qualityString(" "), qualityString(" a "), qualityString("\r\nx"), qualityString("e\u0301"),
				qualityString("A\u00e9\u4e2dB"), qualityString("a\U0001F600b"), qualityString("\U0001F600\U0001F600"),
				qualityString("\uFFFD"), qualityString(`\u`)}
			for route, create := range map[string]func(FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
				for i, value := range values {
					h := FS882Header{PlotNumber: fmt.Sprintf("GEO-%s-%s-%d", field.name, route, i),
						BedrockGeology1: qualityString("AB"), BedrockGeology2: qualityString("CD"), BedrockGeology3: qualityString("EF")}
					setSiteCode(&h, field.name, value)
					if err := create(h); err != nil {
						t.Fatal(err)
					}
					for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSiteCode(&h, field.name, qualityString("XY"))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSiteCode(&h, field.name, value)
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
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "GEO-invalid"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, field := range geologyHeaderFields(h) {
		for _, invalid := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff}), string([]byte{0xed, 0xa0, 0x80})} {
			next := h
			setSiteCode(&next, field.name, &invalid)
			for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.name) {
					t.Fatal("invalid update accepted", err)
				}
				assertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "GEO-new"
			for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("invalid create accepted")
				}
				assertSubstrateSnapshot(t, db, before)
			}
		}
		for _, key := range []string{field.property, field.name, strings.ToUpper(field.property), `\u0062` + field.property[1:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.property + `":"ok"}`, `{"` + field.property + `":"ok","` + key + `":` + raw + `}`} {
					target := h
					if err := json.Unmarshal([]byte(data), &target); err == nil || !reflect.DeepEqual(target, h) {
						t.Fatal("malformed token repaired/hidden or destination changed")
					}
				}
			}
			for _, raw := range []string{`null`, `"aB"`, `"\ufffd"`, `"a\ud83d\ude00b"`, `"O'"`, `"e\u0301"`, `" \n"`} {
				var target FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &target); err != nil {
					t.Fatal(err)
				}
			}
			for _, raw := range []string{`7`, `true`, `[]`, `{}`} {
				var target FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &target); err == nil {
					t.Fatal("invalid nullable string type accepted")
				}
			}
		}
		for _, raw := range []string{`""`, `"12345"`, `"abc\ud83d\ude00"`} {
			var target FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"GEO-json","`+field.property+`":`+raw+`}`), &target); err != nil {
				t.Fatal(err)
			}
			if err := s.SavePlot(target); err == nil {
				t.Fatal("JSON bypassed storage boundary")
			}
			assertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestGeologyPhysicalHistoricalOmissionAndSchemaPending(t *testing.T) {
	for _, field := range geologyHeaderFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "GEO-history"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			column := quoteHeaderIdentifier(field.name)
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
				before := substrateSnapshot(t, db)
				for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					assertSubstrateSnapshot(t, db, before)
					loaded.FieldNotes = qualityString("unrelated")
					if err := update(*loaded); err != nil {
						t.Fatal(err)
					}
					before = substrateSnapshot(t, db)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), &historical) {
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
				setSiteCode(loaded, field.name, nil)
				if err := s.UpdatePlot(*loaded); err != nil {
					t.Fatal("historical NULL clear failed", err)
				}
			}
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + column); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetHeaderCapabilities()
			if err != nil || caps[field.property] {
				t.Fatal("missing schema capability enabled")
			}
			h = FS882Header{PlotNumber: h.PlotNumber, FieldNotes: qualityString("schema independent")}
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("schema pending blocked unrelated save", err)
			}
			setSiteCode(&h, field.name, qualityString("AB"))
			before := substrateSnapshot(t, db)
			if err := s.UpdatePlot(h); err == nil {
				t.Fatal("missing column write accepted")
			}
			assertSubstrateSnapshot(t, db, before)
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
				s, db := headerFixture(t)
				h := FS882Header{PlotNumber: "GEO-atomic", BedrockGeology1: qualityString("AB"), BedrockGeology2: qualityString("CD"), BedrockGeology3: qualityString("EF")}
				if !create {
					s.SetAuditStrength(0)
					if err := s.CreatePlot(h); err != nil {
						t.Fatal(err)
					}
				}
				s.SetAuditStrength(3)
				next := h
				next.BedrockGeology1, next.BedrockGeology2, next.BedrockGeology3 = qualityString("aB"), qualityString("q'X4"), qualityString("ZZZZ")
				next.OfficeNotes = qualityString("admin partner")
				if _, err := db.Exec(trigger); err != nil {
					t.Fatal(err)
				}
				before := substrateSnapshot(t, db)
				save := s.UpdatePlot
				if create {
					save = s.CreatePlot
				}
				if err := save(next); err == nil {
					t.Fatal("failed transaction accepted")
				}
				assertSubstrateSnapshot(t, db, before)
				if _, err := db.Exec(`DROP TRIGGER geo_reject`); err != nil {
					t.Fatal(err)
				}
				if err := save(next); err != nil {
					t.Fatal("retry failed", err)
				}
				before = substrateSnapshot(t, db)
				if err := s.CreatePlot(next); err == nil {
					t.Fatal("collision accepted")
				}
				assertSubstrateSnapshot(t, db, before)
				next.PlotNumber = "GEO-absent"
				if err := s.UpdatePlot(next); err == nil {
					t.Fatal("stale identity accepted")
				}
				assertSubstrateSnapshot(t, db, before)
			})
		}
	}
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "GEO-audit", BedrockGeology1: qualityString("AB")}
	s.SetAuditStrength(0)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	for _, value := range []*string{qualityString("ab"), qualityString("q'X4"), nil} {
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
