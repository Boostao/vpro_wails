package main

import (
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func setSubstrate(h *FS882Header, column string, value *float64) {
	reflect.ValueOf(h).Elem().FieldByName(column).Set(reflect.ValueOf(value))
}

func substrateSnapshot(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, table := range []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other", "Metadata", "Audit"} {
		snapshot[table] = heightTableSnapshot(t, db, "Sample_"+table)
	}
	return snapshot
}

func assertSubstrateSnapshot(t *testing.T, db *sql.DB, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, substrateSnapshot(t, db)) {
		t.Fatal("refused mutation changed parent/children/metadata/history")
	}
}

func TestSubstrateAllFieldsWritePathsRoundtrip(t *testing.T) {
	s, _ := headerFixture(t)
	values := []*float64{nil, heightFloat(0), heightFloat(-3.75), heightFloat(.125), heightFloat(99),
		heightFloat(100), heightFloat(101), heightFloat(1.234567890123),
		heightFloat(math.MaxFloat32), heightFloat(-math.MaxFloat32)}
	for _, field := range substrateHeaderFields(FS882Header{}) {
		t.Run(field.property, func(t *testing.T) {
			for i, value := range values {
				for route, save := range map[string]func(FS882Header) error{"create": s.CreatePlot, "compat": s.SavePlot} {
					h := FS882Header{PlotNumber: fmt.Sprintf("%s-%s-%d", field.column, route, i)}
					setSubstrate(&h, field.column, value)
					if err := save(h); err != nil {
						t.Fatalf("%s valid create %v: %v", route, value, err)
					}
					got, err := s.GetPlot(h.PlotNumber)
					if err != nil || !reflect.DeepEqual(*got, h) {
						t.Fatalf("raw float64/NULL/partner roundtrip: %#v %v", got, err)
					}
					for _, update := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
						setSubstrate(&h, field.column, heightFloat(7.5))
						if err := update(h); err != nil {
							t.Fatal(err)
						}
						setSubstrate(&h, field.column, value)
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
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "SUBINVALID"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, field := range substrateHeaderFields(h) {
		for _, value := range []float64{3.5e38, -3.5e38, math.NaN(), math.Inf(1), math.Inf(-1)} {
			next := h
			setSubstrate(&next, field.column, &value)
			for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(next); err == nil || !strings.Contains(err.Error(), field.property) {
					t.Fatalf("invalid update %s=%v: %v", field.property, value, err)
				}
			}
			next.PlotNumber = "SUBNEW"
			for _, save := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := save(next); err == nil || !strings.Contains(err.Error(), field.property) {
					t.Fatalf("invalid create %s=%v: %v", field.property, value, err)
				}
			}
		}
	}
	assertSubstrateSnapshot(t, db, before)
	if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("create collision: %v", err)
	}
	h.PlotNumber = "SUBMISSING"
	if err := s.UpdatePlot(h); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("missing update: %v", err)
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestSubstrateUnsupportedColumnsNullAndSupplied(t *testing.T) {
	for _, field := range substrateHeaderFields(FS882Header{}) {
		t.Run(field.property, func(t *testing.T) {
			s, db := headerFixture(t)
			if _, err := db.Exec(`ALTER TABLE Sample_Env DROP COLUMN ` + quoteHeaderIdentifier(field.column)); err != nil {
				t.Fatal(err)
			}
			h := FS882Header{PlotNumber: "SUBSCHEMA"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatalf("unsupported nil should be omitted: %v", err)
			}
			for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(h); err != nil {
					t.Fatalf("unsupported nil update should be omitted: %v", err)
				}
			}
			before := substrateSnapshot(t, db)
			setSubstrate(&h, field.column, heightFloat(0))
			for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
				if err := save(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
					t.Fatalf("supplied unsupported value accepted: %v", err)
				}
			}
			h.PlotNumber = "SUBSCHEMANEW"
			if err := s.CreatePlot(h); err == nil || !strings.Contains(err.Error(), "unsupported") {
				t.Fatalf("create supplied unsupported value: %v", err)
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
}

func TestSubstrateHistoricalOverflowOmittedClearedAndActualRead(t *testing.T) {
	s, db := headerFixture(t)
	h := FS882Header{PlotNumber: "SUBHIST"}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	var assignments []string
	for i, field := range substrateHeaderFields(h) {
		value := "1e40"
		if i%2 != 0 {
			value = "-1e40"
		}
		assignments = append(assignments, quoteHeaderIdentifier(field.column)+"="+value)
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
	next.FieldNotes = qualityString("unrelated Env")
	next.OfficeNotes = qualityString("unrelated Admin")
	for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
		if err := save(next); err != nil {
			t.Fatalf("unchanged historical overflow rewritten: %v", err)
		}
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, next) {
		t.Fatal("historical value/partners changed")
	}
	for _, field := range substrateHeaderFields(next) {
		bad := next
		setSubstrate(&bad, field.column, heightFloat(-*field.value))
		if err := s.UpdatePlot(bad); err == nil {
			t.Fatal("new historical overflow accepted")
		}
	}
	if _, err := db.Exec(`DROP TRIGGER substrate_unchanged`); err != nil {
		t.Fatal(err)
	}
	// Preflight permits imported values, but the transaction's actual old read
	// must reject the same payload after another writer clears those fields.
	if err := validateSubstrateHeaderBeforeTransaction(db, "Sample", next, headerUpdate); err != nil {
		t.Fatal(err)
	}
	for _, field := range substrateHeaderFields(next) {
		setSubstrate(&next, field.column, nil)
	}
	if err := s.UpdatePlot(next); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot, s.CreatePlot} {
		if err := save(*old); err == nil {
			t.Fatal("stale payload reintroduced historical overflow")
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	caps, err := headerCapabilities(tx, "Sample")
	if err != nil {
		t.Fatal(err)
	}
	current, err := readHeader(tx, "Sample", h.PlotNumber, caps)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSubstrateHeaderValues(*old, current); err == nil {
		t.Fatal("transactional old read grandfathered changed historical value")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestSubstrateSumsNoBalancingAndUnrelatedPreservation(t *testing.T) {
	s, db := headerFixture(t)
	h := fullHeader("SUBSUMS")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	seedHeight(t, db, h.PlotNumber, 0)
	before := substrateSnapshot(t, db)
	for _, values := range [][]float64{{10, 10, 10, 10, 10, 10}, {10, 20, 20, 20, 20, 10}, {20, 20, 20, 20, 20, 20}} {
		for i, field := range substrateHeaderFields(h) {
			setSubstrate(&h, field.column, heightFloat(values[i]))
		}
		if err := s.UpdatePlot(h); err != nil {
			t.Fatalf("sum restriction invented: %v", err)
		}
		got, err := s.GetPlot(h.PlotNumber)
		if err != nil || !reflect.DeepEqual(*got, h) {
			t.Fatal("substrate edits balanced or changed unrelated header values")
		}
		after := substrateSnapshot(t, db)
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
	count := auditCount(t, db, h.PlotNumber)
	if err := s.SavePlot(h); err != nil || auditCount(t, db, h.PlotNumber) != count {
		t.Fatalf("no-op rewrote/audited substrate: %v", err)
	}
}

func TestSubstrateAuditLevelsRawValuesAndAtomicRollback(t *testing.T) {
	s, db := headerFixture(t)
	if err := s.SetCurrentUser("HeaderTester"); err != nil {
		t.Fatal(err)
	}
	for _, field := range substrateHeaderFields(FS882Header{}) {
		for strength := 0; strength <= 3; strength++ {
			s.SetAuditStrength(strength)
			h := FS882Header{PlotNumber: fmt.Sprintf("%s-%d", field.column, strength)}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			count := 0
			for i, value := range []*float64{heightFloat(1.25), heightFloat(1.234567890123), heightFloat(1.234567890123), nil} {
				setSubstrate(&h, field.column, value)
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				if (i == 0 && strength >= 2) || (i == 1 && strength >= 1) || (i == 3 && strength == 3) {
					count++
				}
				if auditCount(t, db, h.PlotNumber) != count {
					t.Fatalf("%s strength%d phase%d", field.property, strength, i)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Table != "_Env" || entry.EditField != field.column || entry.ID != nil || entry.User != "HeaderTester" {
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
	seedHeight(t, db, h.PlotNumber, 0)
	before := substrateSnapshot(t, db)
	next := h
	for _, field := range substrateHeaderFields(next) {
		setSubstrate(&next, field.column, heightFloat(1.234567890123))
	}
	next.FieldNotes, next.OfficeNotes = qualityString("Env rollback"), qualityString("Admin rollback")
	for _, trigger := range []string{
		`CREATE TRIGGER substrate_reject BEFORE UPDATE OF SubstrateWater ON Sample_Env BEGIN SELECT RAISE(ABORT,'Env data failure'); END`,
		`CREATE TRIGGER substrate_reject BEFORE UPDATE ON Sample_Admin BEGIN SELECT RAISE(ABORT,'data failure'); END`,
		`CREATE TRIGGER substrate_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='SubstrateWater'
 BEGIN SELECT RAISE(ABORT,'audit failure'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		for _, save := range []func(FS882Header) error{s.UpdatePlot, s.SavePlot} {
			if err := save(next); err == nil {
				t.Fatal("trigger failure silently succeeded")
			}
			assertSubstrateSnapshot(t, db, before)
		}
		newPlot := next
		newPlot.PlotNumber = "SUBROLLNEW"
		if strings.Contains(trigger, "Sample_Audit") {
			for _, save := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
				if err := save(newPlot); err == nil {
					t.Fatal("create audit failure silently succeeded")
				}
				assertSubstrateSnapshot(t, db, before)
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
	s, db := headerFixture(t)
	for strength := 0; strength <= 3; strength++ {
		s.SetAuditStrength(strength)
		h := FS882Header{PlotNumber: fmt.Sprintf("SUBCREATE%d", strength)}
		for _, field := range substrateHeaderFields(h) {
			setSubstrate(&h, field.column, heightFloat(1.234567890123))
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
	before := substrateSnapshot(t, db)
	h := FS882Header{PlotNumber: "SUBINSERTFAIL"}
	for _, field := range substrateHeaderFields(h) {
		setSubstrate(&h, field.column, heightFloat(101))
	}
	for _, save := range []func(FS882Header) error{s.CreatePlot, s.SavePlot} {
		if err := save(h); err == nil {
			t.Fatal("create data failure silently succeeded")
		}
		assertSubstrateSnapshot(t, db, before)
	}
}

func TestSubstrateSharedSingleGuardAndHeightCoverPolicy(t *testing.T) {
	for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := validateSingleRangeChange("substrateWater", &invalid, &invalid); err == nil {
			t.Fatal("unchanged nonfinite grandfathered")
		}
	}
	historical := heightFloat(1e40)
	if err := validateSingleRangeChange("substrateWater", historical, historical); err != nil {
		t.Fatal(err)
	}
	if err := validateHeightNumericChange("height1", historical, historical); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{100, 101, math.MaxFloat32} {
		if err := validateSingleRangeChange("substrateWater", nil, &value); err != nil {
			t.Fatal(err)
		}
		if err := validateHeightNumericChange("cover1", nil, &value); err == nil {
			t.Fatal("shared helper removed verified cover<100 policy")
		}
	}
	for _, value := range []float64{3.5e38, -3.5e38} {
		if err := validateHeightNumericChange("height1", nil, &value); err == nil {
			t.Fatal("shared helper removed height SINGLE bounds")
		}
	}
}
