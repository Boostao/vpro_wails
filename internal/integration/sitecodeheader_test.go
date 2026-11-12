package integration

import (
	"github.com/boostao/vpro-wails/internal/references"
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

func setSiteCode(h *vprocore.FS882Header, member string, value *string) {
	reflect.ValueOf(h).Elem().FieldByName(member).Set(reflect.ValueOf(value))
}

func TestSiteCodeAllFieldsWritePathsAndBoundaries(t *testing.T) {
	s, _ := vprotest.HeaderFixture(t)
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		values := []*string{nil, vprotest.QualityString("AT"), vprotest.QualityString("X")}
		if field.Maximum == 8 {
			values = append(values, vprotest.QualityString("zz"), vprotest.QualityString("O'"), vprotest.QualityString("rawCase"),
				vprotest.QualityString(strings.Repeat("x", 8)), vprotest.QualityString(strings.Repeat("x", 6)+"\U0001F600"),
				vprotest.QualityString("\uFFFD"), vprotest.QualityString(`\ud800`), vprotest.QualityString("M.f"), vprotest.QualityString("M.s"))
		}
		for i, value := range values {
			for route, create := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "compat": s.SavePlot} {
				h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("%s-%s-%d", field.Name, route, i)}
				setSiteCode(&h, field.Name, value)
				if err := create(h); err != nil {
					t.Fatalf("valid %s: %v", field.Name, err)
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
						t.Fatalf("raw/NULL value or partner changed: %#v %v", got, err)
					}
				}
				setSiteCode(&h, field.Name, nil)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestSiteCodeInvalidEveryWritePathAndExposureStrictCanonical(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "CODEINVALID"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, field := range vprocore.SiteCodeHeaderFields(h) {
		invalids := []string{"", strings.Repeat("x", field.Maximum+1),
			strings.Repeat("x", field.Maximum-1) + "\U0001F600", string([]byte{0xFF})}
		if field.Maximum == 2 {
			invalids = append(invalids, "at", "At", "a", "A", "zz", "O'", "Atmosphere-related effects", "\U0001F600", "\uFFFD")
		}
		for _, value := range invalids {
			next := h
			setSiteCode(&next, field.Name, &value)
			for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.Name) {
					t.Fatalf("%s invalid %q accepted: %v", field.Name, value, err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "NEWINVALID"
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatalf("invalid %s create accepted", field.Name)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
	}
}

func TestSiteCodeHistoricalOmissionAndCatalogueFailureIsolation(t *testing.T) {
	for _, failure := range []string{"corrupt-default", "closed-injected", "startup-error"} {
		t.Run(failure, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "CODEHIST"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Env SET SiteDisturbance1='123456789',SiteDisturbance2='',SiteDisturbance3='raw',
 Exposure1='historical',Exposure2='zz' WHERE PlotNumber=?`, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			loaded, err := s.GetPlot(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "corrupt-default":
				if err := os.WriteFile(filepath.Join(s.Projects.Root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			case "closed-injected":
				s.SiteCodes, err = references.NewSiteCodeService(s.Projects.Root)
				if err != nil || s.SiteCodes.Close() != nil {
					t.Fatal("close fixture failed")
				}
			case "startup-error":
				s.SiteCodesError = fmt.Errorf("catalogue initialization failure")
			}
			if _, err := db.Exec(`CREATE TRIGGER codes_no_rewrite BEFORE UPDATE OF
 SiteDisturbance1,SiteDisturbance2,SiteDisturbance3,Exposure1,Exposure2 ON Sample_Env
 BEGIN SELECT RAISE(ABORT,'unchanged code rewritten'); END`); err != nil {
				t.Fatal(err)
			}
			next := *loaded
			next.OfficeNotes = vprotest.QualityString("unrelated Admin edit")
			next.FieldNotes = vprotest.QualityString("unrelated Env edit")
			for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err != nil {
					t.Fatalf("unrelated historical save required catalogue or rewrote code: %v", err)
				}
			}
			if _, err := db.Exec(`DROP TRIGGER codes_no_rewrite`); err != nil {
				t.Fatal(err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			next.Exposure1 = vprotest.QualityString("AT")
			if err := s.UpdatePlot(next); err == nil || !strings.Contains(err.Error(), "catalogue") {
				t.Fatalf("new Exposure bypassed unavailable catalogue: %v", err)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
			next.Exposure1 = nil
			next.Exposure2 = nil
			next.SiteDisturbance1 = nil
			next.SiteDisturbance2 = nil
			if err := s.UpdatePlot(next); err != nil {
				t.Fatalf("NULL clear required catalogue: %v", err)
			}
			next.SiteDisturbance3 = vprotest.QualityString("unknown")
			if err := s.UpdatePlot(next); err != nil {
				t.Fatalf("disturbance manual value required catalogue: %v", err)
			}
			got, err := s.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(*got, next) {
				t.Fatal("historical code or valid clear normalized")
			}
		})
	}
}

func TestSiteCodeJSONAllOccurrencesCaseFoldEscapedKeysAndUnicode(t *testing.T) {
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		keys := []string{field.Property, field.Name, strings.ToUpper(field.Property),
			field.Property[:1] + `\u` + fmt.Sprintf("%04x", field.Property[1]) + field.Property[2:]}
		for _, key := range keys {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`,
				`"\ud800\\udc00"`, `"\ud800\ud800"`, `"\ud800\udc0"`, `"\ud"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{
					`{"` + key + `":` + raw + `}`,
					`{"` + key + `":` + raw + `,"` + field.Property + `":"AT"}`,
					`{"` + field.Property + `":"AT","` + key + `":` + raw + `}`,
				} {
					h := vprocore.FS882Header{PlotNumber: "unchanged"}
					before := h
					if err := json.Unmarshal([]byte(data), &h); err == nil || !reflect.DeepEqual(h, before) {
						t.Fatalf("malformed/duplicate %s repaired or destination mutated: %s", key, data)
					}
				}
			}
			for _, text := range []string{`\ud800`, "\uFFFD", "\U0001F600", "O'"} {
				token, err := json.Marshal(text)
				if err != nil {
					t.Fatal(err)
				}
				var h vprocore.FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+string(token)+`}`), &h); err != nil {
					t.Fatal(err)
				}
				got := reflect.ValueOf(h).FieldByName(field.Name).Interface().(*string)
				if got == nil || *got != text {
					t.Fatal("valid Unicode/literal backslash normalized")
				}
			}
		}
		var h vprocore.FS882Header
		if err := json.Unmarshal([]byte(`{"`+field.Property+`":"\ud83d\ude00"}`), &h); err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{"7", "true", "[]", "{}"} {
			if err := json.Unmarshal([]byte(`{"`+field.Property+`":`+token+`}`), &h); err == nil {
				t.Fatal("standard type error suppressed")
			}
		}
		if err := json.Unmarshal([]byte(`{"`+field.Property+`":null}`), &h); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSiteCodeAuditAllLevelsRawCaseAndNoop(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	if err := s.SetCurrentUser("HeaderTester"); err != nil {
		t.Fatal(err)
	}
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("%s-audit%d", field.Name, strength)}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			second := "NA"
			if field.Maximum == 8 {
				second = "at"
			}
			count := 0
			for phase, value := range []*string{vprotest.QualityString("AT"), &second, &second, nil} {
				setSiteCode(&h, field.Name, value)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if (phase == 0 && strength >= 2) || (phase == 1 && strength >= 1) || (phase == 3 && strength == 3) {
					count++
				}
				if vprotest.AuditCount(t, db, h.PlotNumber) != count {
					t.Fatalf("%s strength%d phase%d audit threshold mismatch", field.Name, strength, phase)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Table != "_Env" || entry.EditField != field.Name || entry.ID != nil || entry.User != "HeaderTester" {
					t.Fatalf("wrong code audit identity: %#v", entry)
				}
				if entry.BeforeEdit != nil && entry.AfterEdit != nil && (*entry.BeforeEdit != "AT" || *entry.AfterEdit != second) {
					t.Fatal("raw case-only audit was lost/normalized")
				}
			}
		}
	}
	for strength := 0; strength <= 3; strength++ {
		s.SetAuditStrength(strength)
		h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("CODECREATE%d", strength)}
		for _, field := range vprocore.SiteCodeHeaderFields(h) {
			setSiteCode(&h, field.Name, vprotest.QualityString("AT"))
		}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		want := 0
		if strength >= 2 {
			want = 5
		}
		if vprotest.AuditCount(t, db, h.PlotNumber) != want {
			t.Fatal("create code audit threshold mismatch")
		}
	}
}

func TestSiteCodeSchemaIdentityRollbackRetryAndPartnerPreservation(t *testing.T) {
	for _, field := range vprocore.SiteCodeHeaderFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + vprocore.QuoteHeaderIdentifier(field.Name)); err != nil {
				t.Fatal(err)
			}
			h := vprocore.FS882Header{PlotNumber: "CODESCHEMA"}
			setSiteCode(&h, field.Name, vprotest.QualityString("AT"))
			if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("missing code capability invented: %v", err)
			}
			setSiteCode(&h, field.Name, nil)
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
		})
	}
	s, db := vprotest.HeaderFixture(t)
	h := fullHeader("CODEROLLBACK")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	vprotest.SeedHeight(t, db, h.PlotNumber, 0)
	before := vprotest.SubstrateSnapshot(t, db)
	if err := s.CreatePlot(h); err == nil {
		t.Fatal("create collision overwrote existing identity")
	}
	missing := h
	missing.PlotNumber = "CODEMISSING"
	if err := s.UpdatePlot(missing); err == nil {
		t.Fatal("missing update recreated plot")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	next := h
	for _, field := range vprocore.SiteCodeHeaderFields(next) {
		setSiteCode(&next, field.Name, vprotest.QualityString("NA"))
	}
	next.OfficeNotes, next.FieldNotes = vprotest.QualityString("Admin rollback"), vprotest.QualityString("Env rollback")
	for _, trigger := range []string{
		`CREATE TRIGGER codes_reject BEFORE UPDATE OF Exposure2 ON Sample_Env BEGIN SELECT RAISE(ABORT,'Env data failure'); END`,
		`CREATE TRIGGER codes_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'Admin data failure'); END`,
		`CREATE TRIGGER codes_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Exposure2' BEGIN SELECT RAISE(ABORT,'audit failure'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := update(next); err == nil {
				t.Fatal("real data/audit failure silently succeeded")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
		if strings.Contains(trigger, "Sample_Audit") {
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(missing); err == nil {
					t.Fatal("create audit failure silently succeeded")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
		if _, err := db.Exec(`DROP TRIGGER codes_reject`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER codes_create_reject BEFORE INSERT ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'Admin create failure'); END`); err != nil {
		t.Fatal(err)
	}
	for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
		if err := create(missing); err == nil {
			t.Fatal("create data failure silently succeeded")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
	if _, err := db.Exec(`DROP TRIGGER codes_create_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	after := vprotest.SubstrateSnapshot(t, db)
	for _, table := range []string{"Veg", "Humus", "Mineral", "Other", "Metadata"} {
		if before[table] != after[table] {
			t.Fatalf("code edit changed %s", table)
		}
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) {
		t.Fatal("retry changed partners or true Boolean values")
	}
	if _, err := db.Exec(`CREATE TRIGGER codes_valid_no_rewrite BEFORE UPDATE OF
 SiteDisturbance1,SiteDisturbance2,SiteDisturbance3,Exposure1,Exposure2 ON Sample_Env
 BEGIN SELECT RAISE(ABORT,'unchanged valid code rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	count := vprotest.AuditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(next); err != nil || vprotest.AuditCount(t, db, h.PlotNumber) != count {
		t.Fatalf("noop code rewritten/audited: %v", err)
	}
}
