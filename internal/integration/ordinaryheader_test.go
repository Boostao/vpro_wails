package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func setOrdinaryTestValue(h *vprocore.FS882Header, member string, value any) {
	for _, mapping := range vprocore.HeaderFields {
		if mapping.Column == member || mapping.Member == member {
			member = mapping.Member
			break
		}
	}
	field := reflect.ValueOf(h).Elem().FieldByName(member)
	if value == nil {
		field.SetZero()
	} else {
		field.Set(reflect.ValueOf(value))
	}
}

func TestOrdinaryTextBoundsUnicodeMemoAndHistoricalPreservation(t *testing.T) {
	for _, field := range vprocore.OrdinaryTextFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "O-TEXT"}
			valid := "  raw q'X4\r\n\U0001f600"
			if field.Maximum == 0 {
				valid = strings.Repeat(valid, 10000)
			} else if field.Maximum < 12 {
				valid = strings.Repeat("S", field.Maximum)
			}
			setOrdinaryTestValue(&h, field.Name, &valid)
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(got, &h) {
				t.Fatal("text was trimmed, normalized or truncated", err)
			}
			before := vprotest.SubstrateSnapshot(t, db)
			invalid := []string{"", string([]byte{0xff})}
			if field.Maximum > 0 {
				invalid = append(invalid, strings.Repeat("x", field.Maximum+1),
					strings.Repeat("x", field.Maximum-1)+"\U0001f600")
			}
			for _, text := range invalid {
				next := h
				setOrdinaryTestValue(&next, field.Name, &text)
				if err := s.UpdatePlot(next); err == nil {
					t.Fatal("invalid new text accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			if field.Maximum > 0 {
				legacy := strings.Repeat("x", field.Maximum+1)
				key := "PlotNumber"
				if field.Table == "Admin" {
					key = "Plot"
				}
				if _, err := db.Exec(`UPDATE Sample_`+field.Table+` SET `+vprocore.QuoteHeaderIdentifier(field.Name)+`=? WHERE `+vprocore.QuoteHeaderIdentifier(key)+`=?`, legacy, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				stored, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`CREATE TRIGGER ordinary_unchanged BEFORE UPDATE OF ` + vprocore.QuoteHeaderIdentifier(field.Name) + ` ON Sample_` + field.Table + ` BEGIN SELECT RAISE(ABORT,'unchanged historical value assigned'); END`); err != nil {
					t.Fatal(err)
				}
				stored.OfficeNotes = vprotest.QualityString("unrelated")
				if err := s.UpdatePlot(*stored); err != nil {
					t.Fatal("unchanged historical invalid text was assigned", err)
				}
			}
			for _, raw := range []string{`"\ud800"`, `"\udfff"`, `"\ud800\u0041"`, "\"\xff\""} {
				h := vprocore.FS882Header{PlotNumber: "unchanged"}
				input := `{"` + strings.ToUpper(field.Property) + `":` + raw + `,"` + field.Property + `":"safe"}`
				if err := json.Unmarshal([]byte(input), &h); err == nil || h.PlotNumber != "unchanged" {
					t.Fatal("raw malformed Unicode was repaired or hidden")
				}
			}
		})
	}
}

func TestOrdinaryNumericPhysicalDomainsAllSaveRoutes(t *testing.T) {
	for _, field := range vprocore.OrdinaryNumberFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "O-NUM"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			if field.IsInteger {
				for _, value := range []int{-32768, -1, 0, 32767} {
					setOrdinaryTestValue(&h, field.Name, &value)
					if err := s.UpdatePlot(h); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				for _, value := range []float64{-1, 0, 100, 101, 1.1234567890123, math.MaxFloat32} {
					setOrdinaryTestValue(&h, field.Name, &value)
					if err := s.SavePlot(h); err != nil {
						t.Fatal(err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(got, &h) {
						t.Fatal("numeric value was rounded or balanced", err)
					}
				}
			}
			before := vprotest.SubstrateSnapshot(t, db)
			next := h
			var invalid any
			if field.IsInteger {
				value := 32768
				invalid = &value
			} else {
				value := math.MaxFloat64
				invalid = &value
			}
			setOrdinaryTestValue(&next, field.Name, invalid)
			for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(next); err == nil {
					t.Fatal("numeric overflow accepted")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			next.PlotNumber = "O-NEW"
			for _, create := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := create(next); err == nil {
					t.Fatal("numeric overflow created")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
			setOrdinaryTestValue(&h, field.Name, nil)
			if err := s.UpdatePlot(h); err != nil {
				t.Fatal("NULL rejected", err)
			}
		})
	}
}

func TestOrdinaryAtomicAuditRollbackRetryAndRestoreGuards(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "O-AUDIT", AirPhotoNum: vprotest.QualityString("old")}
	if err := s.SetAuditStrength(0); err != nil {
		t.Fatal(err)
	}

	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	h.AirPhotoNum, h.EnteredBy = vprotest.QualityString("new"), vprotest.QualityString(" Raw User ")
	if _, err := db.Exec(`CREATE TRIGGER ordinary_reject BEFORE INSERT ON Sample_Audit
 WHEN NEW.EditField='EnteredBy' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	if err := s.UpdatePlot(h); err == nil {
		t.Fatal("failed audit committed")
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER ordinary_reject`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePlot(h); err != nil {
		t.Fatal(err)
	}
	if count := vprotest.AuditCount(t, db, h.PlotNumber); count != 2 {
		t.Fatal("retry duplicated or omitted audits", count)
	}
	for _, field := range vprocore.OrdinaryTextFields(vprocore.FS882Header{}) {
		if err := vprocore.ValidateOrdinaryRestoreValue(field.Table, field.Name, 7); err == nil {
			t.Fatal("wrong restore type accepted", field.Name)
		}
	}
	for _, field := range vprocore.OrdinaryNumberFields(vprocore.FS882Header{}) {
		value := any(math.MaxFloat64)
		if field.IsInteger {
			value = 32768
		}
		if err := vprocore.ValidateOrdinaryRestoreValue(field.Table, field.Name, value); err == nil {
			t.Fatal("overflow restore accepted", field.Name)
		}
	}
	for _, held := range []string{"becSiteUnit", "soilDrainage", "speciesListComplete", "updatedFromCards"} {
		if vprocore.IsOrdinaryProperty(held) {
			t.Fatal(fmt.Sprintf("unavailable distinct workflow enabled: %s", held))
		}
	}

}

func TestOrdinaryHistoricalNumericValuesOmitUnchangedAssignments(t *testing.T) {
	for _, field := range vprocore.OrdinaryNumberFields(vprocore.FS882Header{}) {
		t.Run(field.Name, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "O-LEGACY"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			value := any(3.5e38)
			if field.IsInteger {
				value = 65535
			}
			key := "PlotNumber"
			if field.Table == "Admin" {
				key = "Plot"
			}
			if _, err := db.Exec(`UPDATE Sample_`+field.Table+` SET `+vprocore.QuoteHeaderIdentifier(field.Name)+`=? WHERE `+key+`=?`, value, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TRIGGER ordinary_unchanged BEFORE UPDATE OF ` + vprocore.QuoteHeaderIdentifier(field.Name) +
				` ON Sample_` + field.Table + ` BEGIN SELECT RAISE(ABORT,'historical number assigned'); END`); err != nil {
				t.Fatal(err)
			}
			stored, err := s.GetPlot(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			stored.OfficeNotes = vprotest.QualityString("unrelated edit")
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
			before := vprotest.SubstrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{row}, vprocore.AuditRestorePrune); err == nil {
				t.Fatal("restoration alias bypassed Integer bound")
			} else if !strings.Contains(err.Error(), "physical Access Integer range") {
				t.Fatal("restoration did not reach the physical Integer guard", err)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		})
	}
}
