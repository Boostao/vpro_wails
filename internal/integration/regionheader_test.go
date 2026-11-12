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

func TestRegionAllWriteRoutesRawNullableBounds(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	if err := os.WriteFile(filepath.Join(s.Projects.Root, "region-codes.db"), []byte("unavailable lookup"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, field := range vprocore.RegionHeaderFields(vprocore.FS882Header{}) {
		values := []*string{nil, vprotest.QualityString("zz"), vprotest.QualityString("O'"), vprotest.QualityString("aB"),
			vprotest.QualityString(" "), vprotest.QualityString("\uFFFD"), vprotest.QualityString(strings.Repeat("x", field.Maximum)),
			vprotest.QualityString(strings.Repeat("x", field.Maximum-2) + "\U0001F600")}
		if field.Maximum == 7 {
			values = append(values, vprotest.QualityString(`\ud800`))
		}
		for i, value := range values {
			for route, create := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "save": s.SavePlot} {
				h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("R-%s-%s-%d", field.Name, route, i)}
				setSiteCode(&h, field.Name, value)
				if err := create(h); err != nil {
					t.Fatal(err)
				}
				for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
					setSiteCode(&h, field.Name, vprotest.QualityString("NA"))
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					setSiteCode(&h, field.Name, value)
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(*got, h) {
						t.Fatalf("raw/nullable roundtrip: %#v %v", got, err)
					}
				}
			}
		}
	}
	h := vprocore.FS882Header{PlotNumber: "R-stale", FSRegionDistrict: vprotest.QualityString("raw")}
	before := vprotest.SubstrateSnapshot(t, db)
	if err := s.UpdatePlot(h); err == nil {
		t.Fatal("missing update created row")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	h.FSRegionDistrict = vprotest.QualityString("other")
	if err := s.CreatePlot(h); err == nil {
		t.Fatal("create collision overwrote row")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestRegionInvalidAllWriteRoutesAndHistoricalOmission(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "R-invalid"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, field := range vprocore.RegionHeaderFields(h) {
		for _, value := range []string{"", strings.Repeat("x", field.Maximum+1), strings.Repeat("x", field.Maximum-1) + "\U0001F600", string([]byte{0xff})} {
			next := h
			setSiteCode(&next, field.Name, &value)
			for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.Name) {
					t.Fatalf("invalid field accepted: %v", err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "R-invalid-new"
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("invalid create accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
	}
	for _, historical := range []string{"", "historical-too-long"} {
		if _, err := db.Exec(`UPDATE Sample_Env SET FSRegionDistrict=?,Ecosection=? WHERE PlotNumber=?`, historical, historical, h.PlotNumber); err != nil {
			t.Fatal(err)
		}
		loaded, err := s.GetPlot(h.PlotNumber)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TRIGGER region_unchanged BEFORE UPDATE OF FSRegionDistrict,Ecosection ON Sample_Env BEGIN SELECT RAISE(ABORT,'historical rewritten'); END`); err != nil {
			t.Fatal(err)
		}
		loaded.FieldNotes = vprotest.QualityString("unrelated")
		for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := update(*loaded); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(`DROP TRIGGER region_unchanged`); err != nil {
			t.Fatal(err)
		}
		loaded.FSRegionDistrict, loaded.Ecosection = nil, nil
		if err := s.UpdatePlot(*loaded); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET Ecosection=? WHERE PlotNumber=?`, string([]byte{0xff}), h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	before = vprotest.SubstrateSnapshot(t, db)
	if err := s.SavePlot(*loaded); err == nil {
		t.Fatal("historical malformed Unicode grandfathered")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestRegionJSONEveryOccurrenceAndUnicode(t *testing.T) {
	for _, field := range vprocore.RegionHeaderFields(vprocore.FS882Header{}) {
		for _, key := range []string{field.Property, field.Name, strings.ToUpper(field.Property), field.Property[:1] + `\u` + fmt.Sprintf("%04x", field.Property[1]) + field.Property[2:]} {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`, `"\ud800\\udc00"`, `"\ud800\ud800"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{`{"` + key + `":` + raw + `}`, `{"` + key + `":` + raw + `,"` + field.Property + `":"ok"}`, `{"` + field.Property + `":"ok","` + key + `":` + raw + `}`} {
					h := vprocore.FS882Header{PlotNumber: "untouched"}
					before := h
					if err := json.Unmarshal([]byte(data), &h); err == nil || !reflect.DeepEqual(h, before) {
						t.Fatal("malformed Unicode repaired/destination mutated")
					}
				}
			}
			for _, token := range []string{`null`, `"\ufffd"`, `"\ud83d\ude00"`, `"\\ud800"`, `"O'"`} {
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+token+`}`), &h); err != nil {
					t.Fatal(err)
				}
			}
			for _, token := range []string{`7`, `true`, `[]`, `{}`} {
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+token+`}`), &h); err == nil {
					t.Fatal("invalid JSON field type accepted")
				}
			}
		}
	}
}

func TestRegionExactAuditStrengthsAndAtomicRollback(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	for _, field := range vprocore.RegionHeaderFields(vprocore.FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("R-audit-%s-%d", field.Name, strength)}
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
				if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber=? AND "Table"='_Env' AND EditField=?`, h.PlotNumber, field.Name).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != expected {
					t.Fatalf("strength %d phase %d: %d want %d", strength, phase, count, expected)
				}
			}
			if strength >= 1 {
				var before, after string
				if err := db.QueryRow(`SELECT BeforeEdit,AfterEdit FROM Sample_Audit WHERE PlotNumber=? AND EditField=? AND BeforeEdit='AB'`, h.PlotNumber, field.Name).Scan(&before, &after); err != nil || before != "AB" || after != "ab" {
					t.Fatalf("case audit mismatch %v", err)
				}
			}
		}
	}
	s.SetAuditStrength(3)
	for _, trigger := range []string{
		`CREATE TRIGGER region_reject BEFORE UPDATE OF FSRegionDistrict ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER region_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Ecosection' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("R-rollback-%d", len(trigger))}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		h.FSRegionDistrict, h.Ecosection, h.OfficeNotes = vprotest.QualityString("O'raw"), vprotest.QualityString("ab"), vprotest.QualityString("partner")
		if err := s.UpdatePlot(h); err == nil {
			t.Fatal("trigger failure accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER region_reject`); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdatePlot(h); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRegionSchemaMissingRejectsWithoutMutation(t *testing.T) {
	for _, field := range vprocore.RegionHeaderFields(vprocore.FS882Header{}) {
		s, db := vprotest.HeaderFixture(t)
		h := vprocore.FS882Header{PlotNumber: "R-schema"}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}

		if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + vprocore.QuoteHeaderIdentifier(field.Name)); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		setSiteCode(&h, field.Name, vprotest.QualityString("raw"))
		if err := s.UpdatePlot(h); err == nil {
			t.Fatal("missing schema accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
}
func TestRegionCreateDataAndAuditRollbackRetry(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER region_create_reject BEFORE INSERT ON Sample_Env BEGIN SELECT RAISE(ABORT,'data rejected'); END`,
		`CREATE TRIGGER region_create_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Ecosection' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		s, db := vprotest.HeaderFixture(t)
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := vprotest.SubstrateSnapshot(t, db)
		h := vprocore.FS882Header{PlotNumber: "R-create-rollback", FSRegionDistrict: vprotest.QualityString("o'raw"), Ecosection: vprotest.QualityString("aB")}
		if err := s.CreatePlot(h); err == nil {
			t.Fatal("create data/history failure accepted")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
		if _, err := db.Exec(`DROP TRIGGER region_create_reject`); err != nil {
			t.Fatal(err)
		}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) {
			t.Fatal("create retry changed raw values")
		}
	}
}
