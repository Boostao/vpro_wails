package main

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func setOrdinaryTestValue(h *FS882Header, member string, value any) {
	field := reflect.ValueOf(h).Elem().FieldByName(member)
	if value == nil {
		field.SetZero()
	} else {
		field.Set(reflect.ValueOf(value))
	}
}

func TestOrdinaryTextBoundsUnicodeMemoAndHistoricalPreservation(t *testing.T) {
	for _, field := range ordinaryTextFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "O-TEXT"}
			valid := "  raw q'X4\r\n\U0001f600"
			if field.maximum == 0 {
				valid = strings.Repeat(valid, 10000)
			}
			setOrdinaryTestValue(&h, field.name, &valid)
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(got, &h) {
				t.Fatal("text was trimmed, normalized or truncated", err)
			}
			before := substrateSnapshot(t, db)
			invalid := []string{"", string([]byte{0xff})}
			if field.maximum > 0 {
				invalid = append(invalid, strings.Repeat("x", field.maximum+1),
					strings.Repeat("x", field.maximum-1)+"\U0001f600")
			}
			for _, text := range invalid {
				next := h
				setOrdinaryTestValue(&next, field.name, &text)
				if err := s.UpdatePlot(next); err == nil {
					t.Fatal("invalid new text accepted")
				}
				assertSubstrateSnapshot(t, db, before)
			}
			if field.maximum > 0 {
				legacy := strings.Repeat("x", field.maximum+1)
				key := "PlotNumber"
				if field.table == "Admin" {
					key = "Plot"
				}
				if _, err := db.Exec(`UPDATE Sample_`+field.table+` SET `+quoteHeaderIdentifier(field.name)+`=? WHERE `+quoteHeaderIdentifier(key)+`=?`, legacy, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				stored, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER ordinary_unchanged BEFORE UPDATE OF ` + quoteHeaderIdentifier(field.name) + ` ON Sample_` + field.table + ` BEGIN SELECT RAISE(ABORT,'unchanged historical value assigned'); END`); err != nil {
					t.Fatal(err)
				}
				stored.OfficeNotes = qualityString("unrelated")
				if err := s.UpdatePlot(*stored); err != nil {
					t.Fatal("unchanged historical invalid text was assigned", err)
				}
			}
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`, "\"\xff\""} {
				h := FS882Header{PlotNumber: "unchanged"}
				input := `{"` + strings.ToUpper(field.property) + `":` + raw + `,"` + field.property + `":"safe"}`
				if err := json.Unmarshal([]byte(input), &h); err == nil || h.PlotNumber != "unchanged" {
					t.Fatal("raw malformed Unicode was repaired or hidden")
				}
			}
		})
	}
}

func TestOrdinaryNumericPhysicalDomainsAllSaveRoutes(t *testing.T) {
	for _, field := range ordinaryNumberFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "O-NUM"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			if field.isInteger {
				for _, value := range []int{-32768, -1, 0, 32767} {
					setOrdinaryTestValue(&h, field.name, &value)
					if err := s.UpdatePlot(h); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				for _, value := range []float64{-1, 0, 100, 101, 1.1234567890123, math.MaxFloat32} {
					setOrdinaryTestValue(&h, field.name, &value)
					if err := s.SavePlot(h); err != nil {
						t.Fatal(err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(got, &h) {
						t.Fatal("numeric value was rounded or balanced", err)
					}
				}
			}
			before := substrateSnapshot(t, db)
			next := h
			var invalid any
			if field.isInteger {
				value := 32768
				invalid = &value
			} else {
				value := math.MaxFloat64
				invalid = &value
			}
			setOrdinaryTestValue(&next, field.name, invalid)
			for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(next); err == nil {
					t.Fatal("numeric overflow accepted")
				}
				assertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "O-NEW"
			for _, create := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("numeric overflow created")
				}
				assertSubstrateSnapshot(t, db, before)
			}
			setOrdinaryTestValue(&h, field.name, nil)
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("NULL rejected", err)
			}
		})
	}
}

func TestOrdinaryAtomicAuditRollbackRetryAndRestoreGuards(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "O-AUDIT", AirPhotoNum: qualityString("old")}
	if err := s.SetAuditStrength(0); err != nil {
		t.Fatal(err)
	}

	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	h.AirPhotoNum, h.EnteredBy = qualityString("new"), qualityString(" Raw User ")
	if _, err := db.Exec(`CREATE TRIGGER ordinary_reject BEFORE INSERT ON Sample_Audit
 WHEN NEW.EditField='EnteredBy' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	if err := s.UpdatePlot(h); err == nil {
		t.Fatal("failed audit committed")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER ordinary_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal(err)
	}
	if count := auditCount(t, db, h.PlotNumber); count != 2 {
		t.Fatal("retry duplicated or omitted audits", count)
	}
	for _, field := range ordinaryTextFields(FS882Header{}) {
		if err := validateOrdinaryRestoreValue(field.table, field.name, 7); err == nil {
			t.Fatal("wrong restore type accepted", field.name)
		}
	}
	for _, field := range ordinaryNumberFields(FS882Header{}) {
		value := any(math.MaxFloat64)
		if field.isInteger {
			value = 32768
		}
		if err := validateOrdinaryRestoreValue(field.table, field.name, value); err == nil {
			t.Fatal("overflow restore accepted", field.name)
		}
	}
	for _, held := range []string{"becSiteUnit", "soilDrainage", "photo", "xCoord", "yCoord", "speciesListComplete", "updatedFromCards"} {
		if isOrdinaryProperty(held) {
			t.Fatal(fmt.Sprintf("unavailable distinct workflow enabled: %s", held))
		}
	}

}

func TestOrdinaryHistoricalNumericValuesOmitUnchangedAssignments(t *testing.T) {
	for _, field := range ordinaryNumberFields(FS882Header{}) {
		t.Run(field.name, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "O-LEGACY"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			value := any(3.5e38)
			if field.isInteger {
				value = 65535
			}
			key := "PlotNumber"
			if field.table == "Admin" {
				key = "Plot"
			}
			if _, err := db.Exec(`UPDATE Sample_`+field.table+` SET `+quoteHeaderIdentifier(field.name)+`=? WHERE `+key+`=?`, value, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TRIGGER ordinary_unchanged BEFORE UPDATE OF ` + quoteHeaderIdentifier(field.name) +
				` ON Sample_` + field.table + ` BEGIN SELECT RAISE(ABORT,'historical number assigned'); END`); err != nil {
				t.Fatal(err)
			}
			stored, err := s.GetPlot(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			stored.OfficeNotes = qualityString("unrelated edit")
			if err := s.UpdatePlot(*stored); err != nil {
				t.Fatal("unchanged historical number was assigned", err)
			}
		})
	}
}

func TestOrdinaryRestorationAliasesRejectOverflowWithoutWrites(t *testing.T) {
	for _, alias := range []string{"_Env", "Sample_Env"} {
		t.Run(alias, func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`UPDATE Sample_Env SET RootingDepth=12 WHERE PlotNumber='CHILD1'`); err != nil {
				t.Fatal(err)
			}
			row := restoreAuditFixture(t, db, "CHILD1", alias, "RootingDepth", nil, 32768, 12)
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{row}, AuditRestorePrune); err == nil {
				t.Fatal("restoration alias bypassed Integer bound")
			} else if !strings.Contains(err.Error(), "physical Access Integer range") {
				t.Fatal("restoration did not reach the physical Integer guard", err)
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
}
