package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func setSubstrate(h *vprocore.FS882Header, column string, value *float64) {
	reflect.ValueOf(h).Elem().FieldByName(column).Set(reflect.ValueOf(value))
}

func TestSubstrateAllFieldsWritePathsRoundtrip(t *testing.T) {
	s, _ := vprotest.HeaderFixture(t)
	values := []*float64{nil, vprotest.HeightFloat(0), vprotest.HeightFloat(-3.75), vprotest.HeightFloat(.125), vprotest.HeightFloat(99),
		vprotest.HeightFloat(100), vprotest.HeightFloat(101), vprotest.HeightFloat(1.234567890123),
		vprotest.HeightFloat(math.MaxFloat32), vprotest.HeightFloat(-math.MaxFloat32)}
	for _, field := range vprocore.SubstrateHeaderFields(vprocore.FS882Header{}) {
		t.Run(field.Property, func(t *testing.T) {
			for i, value := range values {
				for route, save := range map[string]func(vprocore.FS882Header) error{"create": s.CreatePlot, "compat": s.SavePlot} {
					h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("%s-%s-%d", field.Column, route, i)}
					setSubstrate(&h, field.Column, value)
					if err := save(h); err != nil {
						t.Fatalf("%s valid create %v: %v", route, value, err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(*got, h) {
						t.Fatalf("raw float64/NULL/partner roundtrip: %#v %v", got, err)
					}
					for _, update := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSubstrate(&h, field.Column, vprotest.HeightFloat(7.5))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSubstrate(&h, field.Column, value)
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						got, err = s.GetPlot(h.PlotNumber)
						if err != nil || !reflect.DeepEqual(*got, h) {
							t.Fatal("update changed exact double or partner")
						}
					}
				}
			}
		})
	}
}

func TestSubstrateAllFieldsRejectInvalidEveryWritePath(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SUBINVALID"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, field := range vprocore.SubstrateHeaderFields(h) {
		for _, value := range []float64{3.5e38, -3.5e38, math.NaN(), math.Inf(1), math.Inf(-1)} {
			next := h
			setSubstrate(&next, field.Column, &value)
			for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(next); err == nil || !strings.Contains(err.Error(), field.Property) {
					t.Fatalf("invalid update %s=%v: %v", field.Property, value, err)
				}
			}
			next.PlotNumber = "SUBNEW"
			for _, save := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := save(next); err == nil || !strings.Contains(err.Error(), field.Property) {
					t.Fatalf("invalid create %s=%v: %v", field.Property, value, err)
				}
			}
		}
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("create collision: %v", err)
	}
	h.PlotNumber = "SUBMISSING"
	if err := s.UpdatePlot(h); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("missing update: %v", err)
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestSubstrateUnsupportedColumnsNullAndSupplied(t *testing.T) {
	for _, field := range vprocore.SubstrateHeaderFields(vprocore.FS882Header{}) {
		t.Run(field.Property, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + vprocore.QuoteHeaderIdentifier(field.Column)); err != nil {
				t.Fatal(err)
			}
			h := vprocore.FS882Header{PlotNumber: "SUBSCHEMA"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatalf("unsupported nil should be omitted: %v", err)
			}
			for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(h); err != nil {
					t.Fatalf("unsupported nil update should be omitted: %v", err)
				}
			}
			before := vprotest.SubstrateSnapshot(t, db)
			setSubstrate(&h, field.Column, vprotest.HeightFloat(0))
			for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
					t.Fatalf("supplied unsupported value accepted: %v", err)
				}
			}
			h.PlotNumber = "SUBSCHEMANEW"
			if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("create supplied unsupported value: %v", err)
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		})
	}
}

func TestSubstrateHistoricalOverflowOmittedClearedAndActualRead(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "SUBHIST"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	var assignments []string
	for i, field := range vprocore.SubstrateHeaderFields(h) {
		value := "1e40"
		if i%2 != 0 {
			value = "-1e40"
		}
		assignments = append(assignments, vprocore.QuoteHeaderIdentifier(field.Column)+"="+value)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET ` + strings.Join(assignments, ",") + ` WHERE PlotNumber='SUBHIST'`); err != nil {
		t.Fatal(err)
	}
	old, err := s.GetPlot(h.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER substrate_unchanged BEFORE UPDATE OF
 SubstrateBedRock,SubstrateDecWood,SubstrateMineralSoil,SubstrateOrganicMatter,SubstrateRocks,SubstrateWater
 ON Sample_Env BEGIN SELECT RAISE(ABORT,'unchanged substrate rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	next := *old
	next.FieldNotes = vprotest.QualityString("unrelated Env")
	next.OfficeNotes = vprotest.QualityString("unrelated Admin")
	for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
		if err := save(next); err != nil {
			t.Fatalf("unchanged historical overflow rewritten: %v", err)
		}
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) {
		t.Fatal("historical value/partners changed")
	}
	for _, field := range vprocore.SubstrateHeaderFields(next) {
		bad := next
		setSubstrate(&bad, field.Column, vprotest.HeightFloat(-*field.Value))
		if err := s.UpdatePlot(bad); err == nil {
			t.Fatal("new historical overflow accepted")
		}
	}
	if _, err := db.Exec(`DROP TRIGGER substrate_unchanged`); err != nil {
		t.Fatal(err)
	}
	// Preflight permits imported values, but the transaction's actual old read
	// must reject the same payload after another writer clears those fields.
	if err := vprocore.ValidateSubstrateHeaderBeforeTransaction(db, "Sample", next, vprocore.HeaderUpdate); err != nil {
		t.Fatal(err)
	}
	for _, field := range vprocore.SubstrateHeaderFields(next) {
		setSubstrate(&next, field.Column, nil)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot, s.CreatePlot} {
		if err := save(*old); err == nil {
			t.Fatal("stale payload reintroduced historical overflow")
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	caps, err := vprocore.HeaderCapabilities(tx, "Sample")
	if err != nil {
		t.Fatal(err)
	}
	current, err := vprocore.ReadHeader(tx, "Sample", h.PlotNumber, caps)
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.ValidateSubstrateHeaderValues(*old, current); err == nil {
		t.Fatal("transactional old read grandfathered changed historical value")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
}

func TestSubstrateSumsNoBalancingAndUnrelatedPreservation(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := fullHeader("SUBSUMS")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	vprotest.SeedHeight(t, db, h.PlotNumber, 0)
	before := vprotest.SubstrateSnapshot(t, db)
	for _, values := range [][]float64{{10, 10, 10, 10, 10, 10}, {10, 20, 20, 20, 20, 10}, {20, 20, 20, 20, 20, 20}} {
		for i, field := range vprocore.SubstrateHeaderFields(h) {
			setSubstrate(&h, field.Column, vprotest.HeightFloat(values[i]))
		}
		if err := s.UpdatePlot(h); err != nil {
			t.Fatalf("sum restriction invented: %v", err)
		}
		got, err := s.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) {
			t.Fatal("substrate edits balanced or changed unrelated header values")
		}
		after := vprotest.SubstrateSnapshot(t, db)
		for _, table := range []string{"Admin", "Veg", "Humus", "Mineral", "Other", "Metadata"} {
			if before[table] != after[table] {
				t.Fatalf("substrate changed unrelated %s", table)
			}
		}
	}
	// Unchanged supported columns are omitted too, not just historical invalids.
	if _, err := db.Exec(`CREATE TRIGGER substrate_valid_no_rewrite BEFORE UPDATE OF
 SubstrateBedRock,SubstrateDecWood,SubstrateMineralSoil,SubstrateOrganicMatter,SubstrateRocks,SubstrateWater
 ON Sample_Env BEGIN SELECT RAISE(ABORT,'unchanged valid substrate rewritten'); END`); err != nil {
		t.Fatal(err)
	}
	count := vprotest.AuditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(h); err != nil || vprotest.AuditCount(t, db, h.PlotNumber) != count {
		t.Fatalf("no-op rewrote/audited substrate: %v", err)
	}
}

func TestSubstrateAuditLevelsRawValuesAndAtomicRollback(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	if err := s.SetCurrentUser("HeaderTester"); err != nil {
		t.Fatal(err)
	}
	for _, field := range vprocore.SubstrateHeaderFields(vprocore.FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("%s-%d", field.Column, strength)}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			count := 0
			for i, value := range []*float64{vprotest.HeightFloat(1.25), vprotest.HeightFloat(1.234567890123), vprotest.HeightFloat(1.234567890123), nil} {
				setSubstrate(&h, field.Column, value)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if (i == 0 && strength >= 2) || (i == 1 && strength >= 1) || (i == 3 && strength == 3) {
					count++
				}
				if vprotest.AuditCount(t, db, h.PlotNumber) != count {
					t.Fatalf("%s strength%d phase%d", field.Property, strength, i)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Table != "_Env" || entry.EditField != field.Column || entry.ID != nil || entry.User != "HeaderTester" {
					t.Fatalf("incorrect substrate audit identity: %#v", entry)
				}
				if entry.AfterEdit != nil && *entry.AfterEdit != "1.25" && *entry.AfterEdit != "1.234567890123" {
					t.Fatal("substrate audit quantized to source six decimals")
				}
			}
		}
	}
	s.SetAuditStrength(3)
	if err := s.SetCurrentUser("Will MacKenzie"); err != nil {
		t.Fatal(err)
	}
	h := fullHeader("SUBROLL")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	vprotest.SeedHeight(t, db, h.PlotNumber, 0)
	before := vprotest.SubstrateSnapshot(t, db)
	next := h
	for _, field := range vprocore.SubstrateHeaderFields(next) {
		setSubstrate(&next, field.Column, vprotest.HeightFloat(1.234567890123))
	}
	next.FieldNotes, next.OfficeNotes = vprotest.QualityString("Env rollback"), vprotest.QualityString("Admin rollback")
	for _, trigger := range []string{
		`CREATE TRIGGER substrate_reject BEFORE UPDATE OF SubstrateWater ON Sample_Env BEGIN SELECT RAISE(ABORT,'Env data failure'); END`,
		`CREATE TRIGGER substrate_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'data failure'); END`,
		`CREATE TRIGGER substrate_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='SubstrateWater'
 BEGIN SELECT RAISE(ABORT,'audit failure'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		for _, save := range []func(vprocore.FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := save(next); err == nil {
				t.Fatal("trigger failure silently succeeded")
			}
			vprotest.AssertSubstrateSnapshot(t, db, before)
		}
		newPlot := next
		newPlot.PlotNumber = "SUBROLLNEW"
		if strings.Contains(trigger, "Sample_Audit") {
			for _, save := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := save(newPlot); err == nil {
					t.Fatal("create audit failure silently succeeded")
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
			}
		}
		if _, err := db.Exec(`DROP TRIGGER substrate_reject`); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
}

func TestSubstrateCreateAuditLevelsAndDataRollback(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	for strength := 0; strength <= 3; strength++ {
		s.SetAuditStrength(strength)
		h := vprocore.FS882Header{PlotNumber: fmt.Sprintf("SUBCREATE%d", strength)}
		for _, field := range vprocore.SubstrateHeaderFields(h) {
			setSubstrate(&h, field.Column, vprotest.HeightFloat(1.234567890123))
		}
		if err := s.CreatePlot(h); err != nil {
			t.Fatal(err)
		}
		want := 0
		if strength >= 2 {
			want = 6
		}
		entries, err := s.ListAuditEntries(h.PlotNumber)
		if err != nil || len(entries) != want {
			t.Fatalf("create strength%d: %d entries %v", strength, len(entries), err)
		}
		for _, entry := range entries {
			if entry.Table != "_Env" || entry.BeforeEdit != nil || entry.AfterEdit == nil ||
				*entry.AfterEdit != "1.234567890123" || entry.ID != nil {
				t.Fatalf("create raw substrate audit: %#v", entry)
			}
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER substrate_create_reject BEFORE INSERT ON Sample_Admin
 BEGIN SELECT RAISE(ABORT,'Admin create failure'); END`); err != nil {
		t.Fatal(err)
	}
	before := vprotest.SubstrateSnapshot(t, db)
	h := vprocore.FS882Header{PlotNumber: "SUBINSERTFAIL"}
	for _, field := range vprocore.SubstrateHeaderFields(h) {
		setSubstrate(&h, field.Column, vprotest.HeightFloat(101))
	}
	for _, save := range []func(vprocore.FS882Header) error{s.CreatePlot, s.SavePlot} {
		if err := save(h); err == nil {
			t.Fatal("create data failure silently succeeded")
		}
		vprotest.AssertSubstrateSnapshot(t, db, before)
	}
}

func TestSubstrateSharedSingleGuardAndHeightCoverPolicy(t *testing.T) {
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := vprocore.ValidateSingleRangeChange("substrateWater", &invalid, &invalid); err == nil {
			t.Fatal("unchanged nonfinite grandfathered")
		}
	}
	historical := vprotest.HeightFloat(1e40)
	if err := vprocore.ValidateSingleRangeChange("substrateWater", historical, historical); err != nil {
		t.Fatal(err)
	}
	if err := vprocore.ValidateHeightNumericChange("height1", historical, historical); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{100, 101, math.MaxFloat32} {
		if err := vprocore.ValidateSingleRangeChange("substrateWater", nil, &value); err != nil {
			t.Fatal(err)
		}
		if err := vprocore.ValidateHeightNumericChange("cover1", nil, &value); err == nil {
			t.Fatal("shared helper removed verified cover<100 policy")
		}
	}
	for _, value := range []float64{3.5e38, -3.5e38} {
		if err := vprocore.ValidateHeightNumericChange("height1", nil, &value); err == nil {
			t.Fatal("shared helper removed height SINGLE bounds")
		}
	}
}
