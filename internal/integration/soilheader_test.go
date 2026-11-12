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

func TestSoilPhysicalAllWriteRoutes(t *testing.T) {
	s, _ := vprotest.HeaderFixture(t)
	if err := os.WriteFile(filepath.Join(s.Projects.Root, "soil-codes.db"), []byte("unavailable catalogue"), 0600); err != nil {
		t.Fatal(err)
	}
	values := []*string{nil, vprotest.QualityString("1234"), vprotest.QualityString("Z9"), vprotest.QualityString("q'X4"),
		vprotest.QualityString("aB"), vprotest.QualityString(" "), vprotest.QualityString(" a "), vprotest.QualityString("\r\nx"),
		vprotest.QualityString("e\u0301"), vprotest.QualityString("a\U0001F600b"), vprotest.QualityString("\U0001F600\U0001F600"),
		vprotest.QualityString("\uFFFD"), vprotest.QualityString(`\u`)}
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		for i, value := range values {
			for route, create := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
				h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("SOIL-%s-%s-%d", field.Name, route, i)}
				setSiteCode(&h, field.Name, value)
				if err := create(h); err != nil {
					t.Fatalf("%s %s: %v", field.Name, route, err)
				}
				for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
					setSiteCode(&h, field.Name, vprotest.QualityString("AB"))
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					setSiteCode(&h, field.Name, value)
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(*got, h) {
						t.Fatalf("raw/nullable/partner roundtrip: %#v %v", got, err)
					}
				}
			}
		}
	}
}

func TestSoilPhysicalRejectsEveryWriteRoute(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SOIL-invalid"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, field := range vprocore.SoilHeaderFields(h) {
		for _, value := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff}), string([]byte{0xed, 0xa0, 0x80})} {
			next := h
			setSiteCode(&next, field.Name, &value)
			for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.Name) {
					t.Fatalf("invalid %s accepted: %v", field.Name, err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "SOIL-invalid-new"
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("invalid create accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
	}
}

func TestSoilPhysicalHistoricalOmissionNoopAndIndependence(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SOIL-historical"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	for _, historical := range []string{"", "historical-too-long", "aB", "ZZZZ", string([]byte{0xff})} {
		if _, err := db.Exec(`UPDATE Sample_Env SET SoilClassGroup=?,SoilClassSubGroup=? WHERE PlotNumber=?`, historical, historical, h.PlotNumber); err != nil {
			t.Fatal(err)
		}
		loaded, err := s.GetPlot(h.PlotNumber)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TRIGGER soil_unchanged BEFORE UPDATE OF SoilClassGroup,SoilClassSubGroup ON Sample_Env BEGIN SELECT RAISE(ABORT,'untouched soil rewritten'); END`); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := update(*loaded); err != nil {
				t.Fatal(err)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
		loaded.FieldNotes = vprotest.QualityString("unrelated")
		if err := s.UpdatePlot(*loaded); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetPlot(h.PlotNumber)
		if err != nil || !vprocore.SameSiteCode(got.SoilClassGroup, &historical) || !vprocore.SameSiteCode(got.SoilClassSubGroup, &historical) {
			t.Fatal("historical bytes repaired or partners changed")
		}
		if _, err := db.Exec(`DROP TRIGGER soil_unchanged`); err != nil {
			t.Fatal(err)
		}
		loaded.SoilClassGroup = nil
		if err := s.UpdatePlot(*loaded); err != nil {
			t.Fatal(err)
		}
		got, err = s.GetPlot(h.PlotNumber)
		if err != nil || got.SoilClassGroup != nil || !vprocore.SameSiteCode(got.SoilClassSubGroup, &historical) {
			t.Fatal("clear changed untouched subgroup")
		}
		loaded.SoilClassSubGroup = nil
		if err := s.UpdatePlot(*loaded); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSoilPhysicalRawJSONEveryOccurrence(t *testing.T) {
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		for _, key := range []string{field.Property, field.Name, strings.ToUpper(field.Property), field.Property[:1] + `\u` + fmt.Sprintf("%04x", field.Property[1]) + field.Property[2:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, `"\ud800\ud800"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.Property + `":"ok"}`, `{"` + field.Property + `":"ok","` + key + `":` + raw + `}`} {
					h := vprocore.FS882Header{PlotNumber: "untouched"}
					before := h
					if err := json.Unmarshal([]byte(data), &h); err == nil || !reflect.DeepEqual(h, before) {
						t.Fatal("malformed input repaired or destination mutated")
					}
				}
			}
			for _, raw := range []string{`null`, `"\ufffd"`, `"\ud83d\ude00"`, `"a\ud83d\ude00b"`, `"O'"`, `"e\u0301"`, `" \n"`, `"\\u"`} {
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &h); err != nil {
					t.Fatal(err)
				}
			}
			for _, raw := range []string{`7`, `true`, `[]`, `{}`} {
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+raw+`}`), &h); err == nil {
					t.Fatal("invalid field type accepted")
				}
			}
		}
	}
}

func TestSoilPhysicalRawJSONStorageBoundsAndHistorical(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		for _, raw := range []string{`""`, `"12345"`, `"abc\ud83d\ude00"`} {
			var h vprocore.FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"SOIL-json","`+field.Property+`":`+raw+`}`), &h); err != nil {
				t.Fatal(err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			if err := s.CreatePlot(h); err == nil {
				t.Fatal("raw JSON invalid physical create accepted")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
		h := vprocore.FS882Header{PlotNumber: "SOIL-json-" + field.Name}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		for _, historical := range []string{"", "long-old-value"} {
			if _, err := db.Exec(`UPDATE Sample_Env SET `+vprocore.QuoteHeaderIdentifier(field.Name)+`=? WHERE PlotNumber=?`, historical, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(map[string]any{"plotNumber": h.PlotNumber, field.Property: historical})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &h); err != nil {
				t.Fatal(err)
			}
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("unchanged historical JSON rejected", err)
			}
			var next vprocore.FS882Header
			if err := json.Unmarshal([]byte(`{"plotNumber":"`+h.PlotNumber+`","`+field.Property+`":"12345"}`), &next); err != nil {
				t.Fatal(err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			if err := s.UpdatePlot(next); err == nil {
				t.Fatal("new overlength JSON update accepted")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
	}
}

func TestSoilPhysicalSchemaPendingCollisionAndRollback(t *testing.T) {
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		s, db := vprotest.HeaderFixture(t)
		h := vprocore.FS882Header{PlotNumber: "SOIL-schema"}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + vprocore.QuoteHeaderIdentifier(field.Name)); err != nil {
			t.Fatal(err)
		}
		caps, err := s.GetHeaderCapabilities()
		if err != nil || caps[field.Property] {
			t.Fatal("missing schema capability enabled")
		}
		h.FieldNotes = vprotest.QualityString("independent")
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal("unrelated save blocked by pending soil schema", err)
		}
		setSiteCode(&h, field.Name, vprotest.QualityString("1234"))
		before := vprotest.SubstrateSnapshot(t, db)
		if err := s.UpdatePlot(h); err == nil {
			t.Fatal("missing soil column accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SOIL-strict", SoilClassGroup: vprotest.QualityString("1234"), SoilClassSubGroup: vprotest.QualityString("Z9")}
	before := vprotest.SubstrateSnapshot(t, db)
	if err := s.UpdatePlot(h); err == nil {
		t.Fatal("missing update created parent")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if err := s.CreatePlot(h); err == nil {
		t.Fatal("collision overwrote parent")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	for _, trigger := range []string{
		`CREATE TRIGGER soil_reject BEFORE UPDATE OF SoilClassSubGroup ON Sample_Env BEGIN SELECT RAISE(ABORT,'soil data rejected'); END`,
		`CREATE TRIGGER soil_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='SoilClassSubGroup' BEGIN SELECT RAISE(ABORT,'soil audit rejected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		next := h
		next.SoilClassGroup, next.SoilClassSubGroup, next.OfficeNotes = vprotest.QualityString("aB"), vprotest.QualityString("q'X4"), vprotest.QualityString("partner")
		before = vprotest.SubstrateSnapshot(t, db)
		if err := s.UpdatePlot(next); err == nil {
			t.Fatal("failed transaction accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER soil_reject`); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdatePlot(next); err != nil {
			t.Fatal(err)
		}
		h = next
		h.SoilClassGroup, h.SoilClassSubGroup = vprotest.QualityString("1234"), vprotest.QualityString("Z9")
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSoilPhysicalExactAuditNoPhantoms(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SOIL-audit", SoilClassGroup: vprotest.QualityString("DBC"), SoilClassSubGroup: vprotest.QualityString("CA")}
	s.SetAuditStrength(0)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	s.SetAuditStrength(3)
	for _, change := range []struct {
		member string
		value  *string
	}{
		{"SoilClassGroup", vprotest.QualityString("1234")},
		{"SoilClassSubGroup", vprotest.QualityString("Z9")},
		{"SoilClassGroup", nil},
		{"SoilClassSubGroup", vprotest.QualityString("z9")},
	} {
		setSiteCode(&h, change.member, change.value)
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal(err)
	}
	entries, err := s.ListAuditEntries(h.PlotNumber)
	if err != nil || len(entries) != 4 {
		t.Fatalf("expected only four mapped changes, got %#v %v", entries, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND BeforeEdit IS NULL AND AfterEdit IS NULL`, h.PlotNumber).Scan(&count); err != nil || count != 0 {
		t.Fatal("phantom NULL/NULL audit")
	}
	for _, entry := range entries {
		if entry.Table != "_Env" || (entry.EditField != "SoilClassGroup" && entry.EditField != "SoilClassSubGroup") || entry.ID != nil {
			t.Fatalf("unmapped/child audit %#v", entry)
		}
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND EditField='SoilClassSubGroup' AND BeforeEdit='Z9' AND AfterEdit='z9'`, h.PlotNumber).Scan(&count); err != nil || count != 1 {
		t.Fatal("case-sensitive exact audit lost")
	}
}

func TestSoilPhysicalCreateRollbackAndAuditStrengths(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER soil_create_reject BEFORE INSERT ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER soil_create_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='SoilClassSubGroup' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		s, db := vprotest.HeaderFixture(t)
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		h := vprocore.FS882Header{PlotNumber: "SOIL-create", SoilClassGroup: vprotest.QualityString("1234"), SoilClassSubGroup: vprotest.QualityString("Z9")}
		if err := s.CreatePlot(h); err == nil {
			t.Fatal("failed create accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER soil_create_reject`); err != nil {
			t.Fatal(err)
		}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
	}
	s, db := vprotest.HeaderFixture(t)
	for _, field := range vprocore.SoilHeaderFields(vprocore.FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("SOIL-strength-%s-%d", field.Name, strength)}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			expected := 0
			for phase, value := range []*string{vprotest.QualityString("AB"), vprotest.QualityString("ab"), vprotest.QualityString("ab"), nil} {
				setSiteCode(&h, field.Name, value)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if phase == 0 && strength >= 2 || phase == 1 && strength >= 1 || phase == 3 && strength >= 3 {
					expected++
				}
				var count int
				if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND EditField=?`, h.PlotNumber, field.Name).Scan(&count); err != nil || count != expected {
					t.Fatalf("strength %d phase %d count %d expected %d: %v", strength, phase, count, expected, err)
				}
			}
		}
	}
}
