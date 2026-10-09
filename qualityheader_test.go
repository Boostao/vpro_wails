package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func qualitySet(h *FS882Header, member string, value *string) {
	reflect.ValueOf(h).Elem().FieldByName(member).Set(reflect.ValueOf(value))
}

func TestQualityHeaderAllFieldsCreateUpdateBoundaries(t *testing.T) {
	service, db := headerFixture(t)
	for _, member := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
		t.Run(member, func(t *testing.T) {
			for i, value := range []string{"unknown", "O'Neil", "lower", "LOWER", strings.Repeat("x", 15),
				strings.Repeat("x", 13) + "\U0001F600", "\uFFFD"} {
				h := FS882Header{PlotNumber: member + string(rune('A'+i))}
				qualitySet(&h, member, &value)
				if err := service.CreatePlot(h); err != nil {
					t.Fatal(err)
				}
				got, err := service.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(*got, h) {
					t.Fatalf("valid raw create changed: %#v %v", got, err)
				}
				next := h
				qualitySet(&next, member, qualityString("manual"))
				if err := service.UpdatePlot(next); err != nil {
					t.Fatal(err)
				}
				qualitySet(&next, member, nil)
				if err := service.UpdatePlot(next); err != nil {
					t.Fatal(err)
				}
				got, err = service.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(*got, next) {
					t.Fatalf("NULL clear changed partner: %#v %v", got, err)
				}
			}
			h := FS882Header{PlotNumber: member + "-bad"}
			if err := service.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			before := auditCount(t, db, h.PlotNumber)
			for _, bad := range []string{"", strings.Repeat("x", 16), strings.Repeat("x", 14) + "\U0001F600", string([]byte{0xFF})} {
				next := h
				qualitySet(&next, member, &bad)
				for _, save := range []func(FS882Header) error{service.UpdatePlot, service.SavePlot} {
					if err := save(next); err == nil || !strings.Contains(err.Error(), member) {
						t.Fatalf("invalid update %q accepted: %v", bad, err)
					}
				}
				next.PlotNumber += "-new"
				if err := service.CreatePlot(next); err == nil {
					t.Fatal("invalid create accepted")
				}
			}
			if auditCount(t, db, h.PlotNumber) != before {
				t.Fatal("invalid input wrote audit")
			}
			got, err := service.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(*got, h) {
				t.Fatal("invalid input changed row")
			}
		})
	}
}

func TestQualityHeaderHistoricalInvalidOmittedAndCleared(t *testing.T) {
	service, db := headerFixture(t)
	h := FS882Header{PlotNumber: "QUALHIST"}
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Admin SET SitePlotQuality=?,VegPlotQuality='',SoilPlotQuality=? WHERE Plot=?`,
		strings.Repeat("s", 16), strings.Repeat("t", 16), h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	old, err := service.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER quality_no_rewrite BEFORE UPDATE OF SitePlotQuality,VegPlotQuality,SoilPlotQuality
 ON Sample_Admin BEGIN SELECT RAISE(ABORT,'unchanged quality must be omitted'); END`); err != nil {
		t.Fatal(err)
	}
	next := *old
	next.OfficeNotes, next.FieldNotes = qualityString("admin partner"), qualityString("Env partner")
	if err := service.UpdatePlot(next); err != nil {
		t.Fatalf("historical unchanged columns were rewritten: %v", err)
	}
	got, err := service.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) {
		t.Fatal("historical data normalized")
	}
	next.SitePlotQuality = qualityString(strings.Repeat("n", 16))
	if err := service.UpdatePlot(next); err == nil {
		t.Fatal("changed invalid historical value accepted")
	}
	if _, err := db.Exec(`DROP TRIGGER quality_no_rewrite`); err != nil {
		t.Fatal(err)
	}
	next = *got
	next.SitePlotQuality, next.VegPlotQuality, next.SoilPlotQuality = nil, nil, nil
	if err := service.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	// A historical payload is not grandfathered when the actual stored row changed.
	if err := service.UpdatePlot(*old); err == nil {
		t.Fatal("stale historical invalid payload was reintroduced")
	}
	if err := service.CreatePlot(*old); err == nil {
		t.Fatal("Create grandfathered historical invalid input")
	}
}

func TestQualityHeaderAuditsLevelsCaseOnlyAndAtomicRollback(t *testing.T) {
	service, db := headerFixture(t)
	for _, member := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
		for strength := 0; strength <= 3; strength++ {
			service.SetAuditStrength(strength)
			h := FS882Header{PlotNumber: member + string(rune('0'+strength))}
			if err := service.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			count := 0
			for i, value := range []*string{qualityString("lower"), qualityString("LOWER"), qualityString("LOWER"), nil} {
				qualitySet(&h, member, value)
				if err := service.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if (i == 0 && strength >= 2) || (i == 1 && strength >= 1) || (i == 3 && strength == 3) {
					count++
				}
				if auditCount(t, db, h.PlotNumber) != count {
					t.Fatalf("raw case/NULL/unchanged threshold mismatch: %s/%d/%d", member, strength, i)
				}
			}
			entries, err := service.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Table != "_Admin" || entry.EditField != member || entry.ID != nil {
					t.Fatalf("quality audit physical identity: %#v", entry)
				}
			}
		}
	}
	service.SetAuditStrength(3)
	h := FS882Header{PlotNumber: "QUALROLL", SitePlotQuality: qualityString("Good")}
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := auditCount(t, db, h.PlotNumber)
	next := h
	next.SitePlotQuality, next.VegPlotQuality, next.SoilPlotQuality = qualityString("Fair"), qualityString("Poor"), qualityString("NA")
	next.FieldNotes = qualityString("Env must rollback")
	for _, trigger := range []string{
		`CREATE TRIGGER quality_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'quality data rejection'); END`,
		`CREATE TRIGGER quality_reject BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'quality audit rejection'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		if err := service.UpdatePlot(next); err == nil {
			t.Fatal("injected failure silently succeeded")
		}
		got, err := service.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) || auditCount(t, db, h.PlotNumber) != before {
			t.Fatal("Env/Admin/audit did not rollback atomically")
		}
		if _, err := db.Exec(`DROP TRIGGER quality_reject`); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
}

func TestQualityHeaderJSONUnicodeTokens(t *testing.T) {
	for _, property := range []string{"sitePlotQuality", "vegPlotQuality", "soilPlotQuality", "SITEPLOTQUALITY"} {
		for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\ud800\u0041"`,
			`"\ud800\\udc00"`, `"\ud800\ud800"`, `"\ud800\udc0"`, `"\ud"`, "\"\xff\""} {
			var h FS882Header
			if err := json.Unmarshal([]byte(`{"`+property+`":`+raw+`}`), &h); err == nil {
				t.Fatalf("%s repaired malformed Unicode %s", property, raw)
			}
		}
	}
	for _, raw := range []string{`"\ud83d\ude00"`, `"\\ud800"`, `"\\\\ud800"`, `"\ufffd"`, `null`, `"O'Neil"`} {
		var h FS882Header
		if err := json.Unmarshal([]byte(`{"sitePlotQuality":`+raw+`}`), &h); err != nil {
			t.Fatalf("valid quality token %s: %v", raw, err)
		}
	}
	var h FS882Header
	if err := json.Unmarshal([]byte(`{"sitePlotQuality":"\\ud800"}`), &h); err != nil || *h.SitePlotQuality != `\ud800` {
		t.Fatal("literal escaped-backslash text rejected/repaired")
	}
	for _, raw := range []string{
		`{"sitePlotQuality":"\ud800","sitePlotQuality":"Good"}`,
		`{"sitePlotQuality":7}`, `{"sitePlotQuality":true}`, `{"sitePlotQuality":[]}`, `{"plotNumber":7}`,
		`{"sitePlotQuality":"Good"} trailing`,
	} {
		if err := json.Unmarshal([]byte(raw), &h); err == nil {
			t.Fatalf("standard JSON/type error or malformed duplicate suppressed: %s", raw)
		}
	}
	if err := json.Unmarshal([]byte(`{"fieldNotes":"\ud800"}`), &h); err != nil || *h.FieldNotes != "\uFFFD" {
		t.Fatal("quality guard broadened another field's Unicode policy")
	}
}

func TestQualityHeaderJSONCaseFoldRawUTF8AndLiteralPreservation(t *testing.T) {
	for _, field := range []struct {
		member, pascal, mixed string
	}{
		{"SitePlotQuality", "SitePlotQuality", "sItEpLoTqUaLiTy"},
		{"VegPlotQuality", "VegPlotQuality", "vEgPlOtQuAlItY"},
		{"SoilPlotQuality", "SoilPlotQuality", "sOiLpLoTqUaLiTy"},
	} {
		for _, key := range []string{field.pascal, field.mixed} {
			for _, raw := range [][]byte{
				[]byte(`"\ud800"`),
				[]byte(`"\udfff"`),
				{'"', 0xFF, '"'},
				{'"', 0xED, 0xA0, 0x80, '"'},
			} {
				h := FS882Header{PlotNumber: "unchanged", SitePlotQuality: qualityString("Good")}
				before := h
				data := append([]byte(`{"`+key+`":`), raw...)
				data = append(data, '}')
				if err := json.Unmarshal(data, &h); err == nil {
					t.Fatalf("case-folded key %q repaired malformed raw token %q", key, raw)
				}
				if !reflect.DeepEqual(h, before) {
					t.Fatal("Unicode refusal mutated destination before validation")
				}
			}
			for _, value := range []string{`\ud800`, "\uFFFD", "\U0001F600"} {
				token, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				data := append([]byte(`{"`+key+`":`), token...)
				data = append(data, '}')
				var h FS882Header
				if err := json.Unmarshal(data, &h); err != nil {
					t.Fatalf("case-folded valid literal %q rejected: %v", value, err)
				}
				got := reflect.ValueOf(h).FieldByName(field.member).Interface().(*string)
				if got == nil || *got != value {
					t.Fatalf("valid literal normalized: %q -> %#v", value, got)
				}
			}
		}
	}
	for _, data := range []string{
		`{"SitePlotQuality":"\ud800","sitePlotQuality":"Good"}`,
		`{"sitePlotQuality":"Good","sItEpLoTqUaLiTy":"\ud800"}`,
		`{"s\u0069tePlotQuality":"\ud800"}`,
	} {
		var h FS882Header
		if err := json.Unmarshal([]byte(data), &h); err == nil {
			t.Fatalf("duplicate/escaped case-folded key bypassed raw guard: %s", data)
		}
	}
}

func TestQualityHeaderUnsupportedIdentityCreateRollbackAndPartnerPreservation(t *testing.T) {
	for _, member := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
		t.Run(member+"-schema", func(t *testing.T) {
			service, db := headerFixture(t)
			if _, err := db.Exec(`ALTER TABLE Sample_Admin DROP COLUMN ` + quoteHeaderIdentifier(member)); err != nil {
				t.Fatal(err)
			}
			h := FS882Header{PlotNumber: "QUALSCHEMA"}
			qualitySet(&h, member, qualityString("Good"))
			if err := service.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("missing quality capability invented: %v", err)
			}
			if auditCount(t, db, h.PlotNumber) != 0 {
				t.Fatal("unsupported create wrote audit")
			}
		})
	}
	service, db := headerFixture(t)
	h := FS882Header{PlotNumber: "QUALPART", SitePlotQuality: qualityString("Good"),
		VegPlotQuality: qualityString("Fair"), SoilPlotQuality: qualityString("Poor")}
	if err := service.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := auditCount(t, db, h.PlotNumber)
	if err := service.CreatePlot(h); err == nil {
		t.Fatal("duplicate plot create overwrote identity")
	}
	missing := h
	missing.PlotNumber = "QUALMISSING"
	if err := service.UpdatePlot(missing); err == nil {
		t.Fatal("stale update recreated row")
	}
	if _, err := db.Exec(`CREATE TRIGGER quality_create_reject BEFORE INSERT ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'Admin insert failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePlot(missing); err == nil {
		t.Fatal("failed Admin create silently succeeded")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Env WHERE PlotNumber=?`, missing.PlotNumber).Scan(&count); err != nil || count != 0 ||
		auditCount(t, db, missing.PlotNumber) != 0 {
		t.Fatal("failed create left Env/audits")
	}
	if _, err := db.Exec(`DROP TRIGGER quality_create_reject`); err != nil {
		t.Fatal(err)
	}
	// Reject any actual change to unrelated workflows while the full header saves quality.
	for _, table := range []string{"Metadata", "Veg", "Humus", "Mineral", "Other"} {
		if _, err := db.Exec(`CREATE TRIGGER ` + quoteHeaderIdentifier("quality_no_"+table) +
			` BEFORE UPDATE ON ` + quoteHeaderIdentifier("Sample_"+table) +
			` BEGIN SELECT RAISE(ABORT,'unrelated workflow touched'); END`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER quality_no_species_change BEFORE UPDATE ON Sample_Env
 WHEN NEW.SpeciesListComplete IS NOT OLD.SpeciesListComplete BEGIN SELECT RAISE(ABORT,'SpeciesListComplete changed'); END`); err != nil {
		t.Fatal(err)
	}
	loaded, err := service.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	next := *loaded
	next.SitePlotQuality = qualityString("manual")
	if err := service.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) || auditCount(t, db, h.PlotNumber) != before+1 {
		t.Fatal("quality save changed partners or non-quality audit")
	}
}
