package main

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestPlotChild_ExplicitZeroIDUpdate(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := childFixture(t)
			s.SetAuditStrength(3)
			query := `INSERT INTO "Sample_` + kind + `" (PlotNumber,ID) VALUES ('CHILD1',0)`
			if kind == "Veg" {
				query = `INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES ('CHILD1',0,'ZERO')`
			}
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			if kind == "Veg" {
				loaded, err := s.ListVegRecords("CHILD1")
				if err != nil || len(loaded) != 1 || loaded[0].ID != 0 {
					t.Fatalf("load zero identity: %+v %v", loaded, err)
				}
			}
			record := childRecord(kind, "CHILD1", 0, true)
			if err := updateChildRecord(s, record); err != nil {
				t.Fatalf("explicit zero-ID update: %v", err)
			}
			if childCount(t, db, kind, "CHILD1") != 1 || childID(t, db, kind, "CHILD1") != 0 {
				t.Fatal("explicit zero-ID update created another row")
			}
			before := map[string]any{}
			if kind == "Veg" {
				before["Species"] = "ZERO"
			}
			checkChildAudits(t, s, kind, 0, before, childValues(kind, record))
			clearChildAudit(t, db)
			if err := updateChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			checkChildAudits(t, s, kind, 0, nil, nil)
			if err := updateChildRecord(s, childRecord(kind, "CHILD2", 0, true)); err == nil {
				t.Fatal("cross-plot/missing explicit zero-ID update accepted")
			}
			if err := updateChildRecord(s, childRecord(kind, "CHILD1", -99, true)); err == nil {
				t.Fatal("stale explicit update recreated missing row")
			}
			if kind == "Veg" {
				invalid := record.Interface().(VegRecord)
				invalid.Species = ""
				if err := s.UpdateVegRecord(invalid); err == nil {
					t.Fatal("explicit update bypassed species validation")
				}
			}
			// Audit failure after the data UPDATE must roll back ID=0 edits too.
			if _, err := db.Exec(`CREATE TRIGGER zero_update_failure BEFORE INSERT ON Sample_Audit
				BEGIN SELECT RAISE(ABORT,'zero update audit failure'); END`); err != nil {
				t.Fatal(err)
			}
			field := childFields[kind][len(childFields[kind])-1]
			want := childValue(record, field)
			editChildMember(record.FieldByName(field.member))
			if err := updateChildRecord(s, record); err == nil {
				t.Fatal("zero-ID update escaped failing audit trigger")
			}
			var stored any
			if err := db.QueryRow(`SELECT ` + quoteHeaderIdentifier(field.column) + ` FROM "Sample_` + kind + `" WHERE PlotNumber = 'CHILD1' AND ID = 0`).Scan(&stored); err != nil || !reflect.DeepEqual(normalizeChildStored(kind, field, stored), want) {
				t.Fatalf("zero-ID update not rolled back: %v %v", stored, err)
			}
			checkChildAudits(t, s, kind, 0, nil, nil)
			if _, err := db.Exec(`DROP TRIGGER zero_update_failure`); err != nil {
				t.Fatal(err)
			}
			// Compatibility Save(ID=0) still creates; loaded rows must use Update.
			if err := saveChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			if childCount(t, db, kind, "CHILD1") != 2 {
				t.Fatal("compatibility Save zero-ID create behavior changed")
			}
		})
	}
}

func TestPlotChild_VegZeroDuplicateAndNullDiagnostics(t *testing.T) {
	s, db := childFixture(t)
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES
		('CHILD1',0,'ONE'),('CHILD1',0,'TWO'),('CHILD2',1,'AAA'),('CHILD2',NULL,'ZZZ')`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateVegRecord(VegRecord{PlotNumber: "CHILD1", ID: 0, Species: "NO"}); err == nil {
		t.Fatal("ambiguous zero-ID update accepted")
	}
	if childCount(t, db, "Veg", "CHILD1") != 2 || auditCount(t, db, "CHILD1") != 0 {
		t.Fatal("ambiguous zero-ID update changed rows/audit")
	}
	rows, err := s.ListVegRecords("CHILD2")
	if err == nil || !strings.Contains(err.Error(), "unsupported NULL ID") ||
		!strings.Contains(err.Error(), "CHILD2") || !strings.Contains(err.Error(), "ZZZ") || rows != nil {
		t.Fatalf("NULL identity must return diagnostic, not partial/default-zero rows: %+v %v", rows, err)
	}
}

func TestPlotChild_AllocationBounds(t *testing.T) {
	s, db := childFixture(t)
	// Positive IDs outside signed32, including JS-unsafe imported IDs, must
	// not push generated IDs outside the documented Access LONG range.
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES
		('CHILD1',-2147483648,'NEGMIN'),('CHILD1',2147483647,'POSMAX'),
		('CHILD1',9007199254740993,'BIG')`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{math.MinInt32, math.MaxInt32} {
		if err := s.UpdateVegRecord(VegRecord{PlotNumber: "CHILD1", ID: id, Species: "UPDATED"}); err != nil {
			t.Fatalf("valid imported signed32 ID rejected: %v", err)
		}
	}
	if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD2", Species: "NEW"}); err != nil {
		t.Fatal(err)
	}
	id := childID(t, db, "Veg", "CHILD2")
	if id < 1 || id > math.MaxInt32 {
		t.Fatalf("allocated ID %d outside positive signed32 range", id)
	}
	// Exercise exhaustion at a small limit without inserting billions of rows.
	for _, test := range []struct {
		sql   string
		want  int64
		error bool
	}{
		{`SELECT 1 UNION SELECT 2 UNION SELECT 3 ORDER BY 1`, 0, true},
		{`SELECT 1 UNION SELECT 3 UNION SELECT 9007199254740993 ORDER BY 1`, 2, false},
		{`SELECT 1 UNION SELECT 2 UNION SELECT 9007199254740993 ORDER BY 1`, 3, false},
	} {
		rows, err := db.Query(test.sql)
		if err != nil {
			t.Fatal(err)
		}
		got, err := availableChildID(rows, 3)
		rows.Close()
		if got != test.want || (err != nil) != test.error {
			t.Fatalf("bounded allocation = %d %v, want %d error=%v", got, err, test.want, test.error)
		}
	}
}
