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

func setSiteCode(h *FS882Header, member string, value *string) {
	reflect.ValueOf(h).Elem().FieldByName(member).Set(reflect.ValueOf(value))
}

func TestSiteCodeAllFieldsWritePathsAndBoundaries(t *testing.T) {
	s, _ := headerFixture(t)
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		values := []*string{nil, qualityString("AT"), qualityString("X")}
		if field.maximum == 8 {
			values = append(values, qualityString("zz"), qualityString("O'"), qualityString("rawCase"),
				qualityString(strings.Repeat("x", 8)), qualityString(strings.Repeat("x", 6)+"\U0001F600"),
				qualityString("\uFFFD"), qualityString(`\ud800`), qualityString("M.f"), qualityString("M.s"))
		}
		for i, value := range values {
			for route, create := range map[string]func(FS882Header) error{"create": s.CreatePlot, "compat": s.SavePlot} {
				h := FS882Header{PlotNumber: fmt.Sprintf("%s-%s-%d", field.name, route, i)}
				setSiteCode(&h, field.name, value)
				if err := create(h); err != nil {
					t.Fatalf("valid %s: %v", field.name, err)
				}
				for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
					setSiteCode(&h, field.name, qualityString("NA"))
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					setSiteCode(&h, field.name, value)
					if err := update(h); err != nil {
						t.Fatal(err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(*got, h) {
						t.Fatalf("raw/NULL value or partner changed: %#v %v", got, err)
					}
				}
				setSiteCode(&h, field.name, nil)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestSiteCodeInvalidEveryWritePathAndExposureStrictCanonical(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "CODEINVALID"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, field := range siteCodeHeaderFields(h) {
		invalids := []string{"", strings.Repeat("x", field.maximum+1),
			strings.Repeat("x", field.maximum-1) + "\U0001F600", string([]byte{0xFF})}
		if field.maximum == 2 {
			invalids = append(invalids, "at", "At", "a", "A", "zz", "O'", "Atmosphere-related effects", "\U0001F600", "\uFFFD")
		}
		for _, value := range invalids {
			next := h
			setSiteCode(&next, field.name, &value)
			for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err == nil || !strings.Contains(err.Error(), field.name) {
					t.Fatalf("%s invalid %q accepted: %v", field.name, value, err)
				}
				assertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "NEWINVALID"
			for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatalf("invalid %s create accepted", field.name)
				}
				assertSubstrateSnapshot(t, db, before)
			}
		}
	}
}

func TestSiteCodeHistoricalOmissionAndCatalogueFailureIsolation(t *testing.T) {
	for _, failure := range []string{"corrupt-default", "closed-injected", "startup-error"} {
		t.Run(failure, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "CODEHIST"}
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
				if err := os.WriteFile(filepath.Join(s.projects.root, "site-codes.db"), []byte("corrupt"), 0600); err != nil {
					t.Fatal(err)
				}
			case "closed-injected":
				s.siteCodes, err = NewSiteCodeService(s.projects.root)
				if err != nil || s.siteCodes.Close() != nil {
					t.Fatal("close fixture failed")
				}
			case "startup-error":
				s.siteCodesError = fmt.Errorf("catalogue initialization failure")
			}
			if _, err := db.Exec(`CREATE TRIGGER codes_no_rewrite BEFORE UPDATE OF
 SiteDisturbance1,SiteDisturbance2,SiteDisturbance3,Exposure1,Exposure2 ON Sample_Env
 BEGIN SELECT RAISE(ABORT,'unchanged code rewritten'); END`); err != nil {
				t.Fatal(err)
			}
			next := *loaded
			next.OfficeNotes = qualityString("unrelated Admin edit")
			next.FieldNotes = qualityString("unrelated Env edit")
			for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := update(next); err != nil {
					t.Fatalf("unrelated historical save required catalogue or rewrote code: %v", err)
				}
			}
			if _, err := db.Exec(`DROP TRIGGER codes_no_rewrite`); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			next.Exposure1 = qualityString("AT")
			if err := s.UpdatePlot(next); err == nil || !strings.Contains(err.Error(), "catalogue") {
				t.Fatalf("new Exposure bypassed unavailable catalogue: %v", err)
			}
			assertSubstrateSnapshot(t, db, before)
			next.Exposure1 = nil
			next.Exposure2 = nil
			next.SiteDisturbance1 = nil
			next.SiteDisturbance2 = nil
			if err := s.UpdatePlot(next); err != nil {
				t.Fatalf("NULL clear required catalogue: %v", err)
			}
			next.SiteDisturbance3 = qualityString("unknown")
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
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		keys := []string{field.property, field.name, strings.ToUpper(field.property),
			field.property[:1] + `\u` + fmt.Sprintf("%04x", field.property[1]) + field.property[2:]}
		for _, key := range keys {
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`,
				`"\ud800\\udc00"`, `"\ud800\ud800"`, `"\ud800\udc0"`, `"\ud"`, "\"\xff\"", "\"\xed\xa0\x80\""} {
				for _, data := range []string{
					`{"` + key + `":` + raw + `}`,
					`{"` + key + `":` + raw + `,"` + field.property + `":"AT"}`,
					`{"` + field.property + `":"AT","` + key + `":` + raw + `}`,
				} {
					h := FS882Header{PlotNumber: "unchanged"}
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
				var h FS882Header
				if err := json.Unmarshal([]byte(`{"`+key+`":`+string(token)+`}`), &h); err != nil {
					t.Fatal(err)
				}
				got := reflect.ValueOf(h).FieldByName(field.name).Interface().(*string)
				if got == nil || *got != text {
					t.Fatal("valid Unicode/literal backslash normalized")
				}
			}
		}
		var h FS882Header
		if err := json.Unmarshal([]byte(`{"`+field.property+`":"\ud83d\ude00"}`), &h); err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{"7", "true", "[]", "{}"} {
			if err := json.Unmarshal([]byte(`{"`+field.property+`":`+token+`}`), &h); err == nil {
				t.Fatal("standard type error suppressed")
			}
		}
		if err := json.Unmarshal([]byte(`{"`+field.property+`":null}`), &h); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSiteCodeAuditAllLevelsRawCaseAndNoop(t *testing.T) {
	s, db := headerFixture(t)
	if err := s.SetCurrentUser("HeaderTester"); err != nil {
		t.Fatal(err)
	}
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := FS882Header{PlotNumber: fmt.Sprintf("%s-audit%d", field.name, strength)}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			second := "NA"
			if field.maximum == 8 {
				second = "at"
			}
			count := 0
			for phase, value := range []*string{qualityString("AT"), &second, &second, nil} {
				setSiteCode(&h, field.name, value)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if (phase == 0 && strength >= 2) || (phase == 1 && strength >= 1) || (phase == 3 && strength == 3) {
					count++
				}
				if auditCount(t, db, h.PlotNumber) != count {
					t.Fatalf("%s strength%d phase%d audit threshold mismatch", field.name, strength, phase)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Table != "_Env" || entry.EditField != field.name || entry.ID != nil || entry.User != "HeaderTester" {
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
		h := FS882Header{PlotNumber: fmt.Sprintf("CODECREATE%d", strength)}
		for _, field := range siteCodeHeaderFields(h) {
			setSiteCode(&h, field.name, qualityString("AT"))
		}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		want := 0
		if strength >= 2 {
			want = 5
		}
		if auditCount(t, db, h.PlotNumber) != want {
			t.Fatal("create code audit threshold mismatch")
		}
	}
}

func TestSiteCodeSchemaIdentityRollbackRetryAndPartnerPreservation(t *testing.T) {
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + quoteHeaderIdentifier(field.name)); err != nil {
				t.Fatal(err)
			}
			h := FS882Header{PlotNumber: "CODESCHEMA"}
			setSiteCode(&h, field.name, qualityString("AT"))
			if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("missing code capability invented: %v", err)
			}
			setSiteCode(&h, field.name, nil)
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
		})
	}
	s, db := headerFixture(t)
	h := fullHeader("CODEROLLBACK")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	seedHeight(t, db, h.PlotNumber, 0)
	before := substrateSnapshot(t, db)
	if err := s.CreatePlot(h); err == nil {
		t.Fatal("create collision overwrote existing identity")
	}
	missing := h
	missing.PlotNumber = "CODEMISSING"
	if err := s.UpdatePlot(missing); err == nil {
		t.Fatal("missing update recreated plot")
	}
	assertSubstrateSnapshot(t, db, before)
	next := h
	for _, field := range siteCodeHeaderFields(next) {
		setSiteCode(&next, field.name, qualityString("NA"))
	}
	next.OfficeNotes, next.FieldNotes = qualityString("Admin rollback"), qualityString("Env rollback")
	for _, trigger := range []string{
		`CREATE TRIGGER codes_reject BEFORE UPDATE OF Exposure2 ON Sample_Env BEGIN SELECT RAISE(ABORT,'Env data failure'); END`,
		`CREATE TRIGGER codes_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'Admin data failure'); END`,
		`CREATE TRIGGER codes_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Exposure2' BEGIN SELECT RAISE(ABORT,'audit failure'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := update(next); err == nil {
				t.Fatal("real data/audit failure silently succeeded")
			}
			assertSubstrateSnapshot(t, db, before)
		}
		if strings.Contains(trigger, "Sample_Audit") {
			for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(missing); err == nil {
					t.Fatal("create audit failure silently succeeded")
				}
				assertSubstrateSnapshot(t, db, before)
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
	for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
		if err := create(missing); err == nil {
			t.Fatal("create data failure silently succeeded")
		}
		assertSubstrateSnapshot(t, db, before)
	}
	if _, err := db.Exec(`DROP TRIGGER codes_create_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	after := substrateSnapshot(t, db)
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
	count := auditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(next); err != nil || auditCount(t, db, h.PlotNumber) != count {
		t.Fatalf("noop code rewritten/audited: %v", err)
	}
}
