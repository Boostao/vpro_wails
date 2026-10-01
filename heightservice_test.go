package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func heightFloat(value float64) *float64 { return &value }

func seedHeight(t *testing.T, db *sql.DB, plot string, id any) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO Sample_Veg
		(PlotNumber,ID,Species,Cover1,Height1,Cover2,Height2,TotalA,TotalB,Collected)
		VALUES (?,?,'PSEUMEN',20,1.25,30,4.75,45,55,'C')`, plot, id); err != nil {
		t.Fatal(err)
	}
}

func heightUpdate(id int, field string, before, after *float64) HeightRecordUpdate {
	return HeightRecordUpdate{ID: id, Values: map[string]*float64{field: after}, Expected: map[string]*float64{field: before}}
}

func storedHeight(t *testing.T, db *sql.DB, plot string, id int, field string) *float64 {
	t.Helper()
	var value sql.NullFloat64
	if err := db.QueryRow(`SELECT `+quoteHeaderIdentifier(field)+` FROM Sample_Veg WHERE PlotNumber=? AND ID=?`, plot, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func heightTableSnapshot(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.Query(`SELECT * FROM ` + quoteHeaderIdentifier(table) + ` ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var records [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatal(err)
		}
		for i, value := range values {
			if data, ok := value.([]byte); ok {
				values[i] = append([]byte(nil), data...)
			}
		}
		records = append(records, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestHeight_AllFieldsPrecisionAndUnrelatedPreservation(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	seedHeight(t, db, "CHILD1", -7)
	header, err := s.GetPlot("CHILD1")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := s.ListVegRecords("CHILD1")
	if err != nil {
		t.Fatal(err)
	}
	unchangedTables := map[string]string{}
	for _, table := range []string{"Sample_Env", "Sample_Admin", "Sample_Metadata"} {
		unchangedTables[table] = heightTableSnapshot(t, db, table)
	}
	values, expected := map[string]*float64{}, map[string]*float64{}
	for property := range heightProperties {
		column := strings.ToUpper(property[:1]) + property[1:]
		expected[property] = storedHeight(t, db, "CHILD1", -7, column)
		values[property] = heightFloat(1.234567890123)
	}
	values["cover1"] = heightFloat(99.99)
	values["height2"] = heightFloat(-2.75)
	values["height3"] = heightFloat(0)
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{{ID: -7, Values: values, Expected: expected}}); err != nil {
		t.Fatal(err)
	}
	for property, want := range values {
		got := storedHeight(t, db, "CHILD1", -7, strings.ToUpper(property[:1])+property[1:])
		if !sameHeightValue(got, want) {
			t.Fatalf("%s precision changed: %v != %v", property, got, want)
		}
	}
	gotHeader, err := s.GetPlot("CHILD1")
	if err != nil || !reflect.DeepEqual(header, gotHeader) {
		t.Fatalf("header/SpeciesListComplete changed: %v", err)
	}
	for table, before := range unchangedTables {
		if before != heightTableSnapshot(t, db, table) {
			t.Fatalf("unrelated table %s changed", table)
		}
	}
	got, err := s.ListVegRecords("CHILD1")
	if err != nil || len(got) != 1 {
		t.Fatal(err)
	}
	if got[0].Species != initial[0].Species || !reflect.DeepEqual(got[0].Collected, initial[0].Collected) {
		t.Fatal("unmapped species/Collected changed")
	}
	var ledger int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='__VPRO_ChildIdentity'`).Scan(&ledger); err != nil || ledger != 0 {
		t.Fatalf("height update allocated/reserved identities: %d %v", ledger, err)
	}
	if got := auditCount(t, db, "CHILD1"); got != len(values) {
		t.Fatalf("all-field audits: %d", got)
	}
	var badAudit int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber='CHILD1' AND ("Table"<>'_Veg' OR ID IS NULL OR ID<>-7)`).Scan(&badAudit); err != nil || badAudit != 0 {
		t.Fatalf("incorrect child audit identity: %d %v", badAudit, err)
	}
	var auditValue string
	if err := db.QueryRow(`SELECT AfterEdit FROM Sample_Audit WHERE PlotNumber='CHILD1' AND EditField='Height1'`).Scan(&auditValue); err != nil || auditValue != "1.234567890123" {
		t.Fatalf("audit precision lost: %q %v", auditValue, err)
	}
}

func TestHeight_AuditThresholdsAndDefault(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		for _, category := range []string{"edit", "addition", "clear", "unchanged", "null-unchanged"} {
			t.Run(string(rune('0'+strength))+"/"+category, func(t *testing.T) {
				s, db := childFixture(t)
				s.SetAuditStrength(strength)
				seedHeight(t, db, "CHILD1", 0)
				before, after := heightFloat(1.25), heightFloat(2.75)
				if category == "addition" || category == "null-unchanged" {
					before = nil
					if _, err := db.Exec(`UPDATE Sample_Veg SET Height1=NULL`); err != nil {
						t.Fatal(err)
					}
				}
				if category == "clear" || category == "null-unchanged" {
					after = nil
				}
				if category == "unchanged" {
					after = heightFloat(1.25)
				}
				if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(0, "height1", before, after)}); err != nil {
					t.Fatal(err)
				}
				want := 0
				if (category == "edit" && strength >= 1) || (category == "addition" && strength >= 2) ||
					(category == "clear" && strength == 3) {
					want = 1
				}
				if got := auditCount(t, db, "CHILD1"); got != want {
					t.Fatalf("audits %d, expected %d", got, want)
				}
				var nullID int
				if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber='CHILD1' AND (ID IS NULL OR ID<>0)`).Scan(&nullID); err != nil || nullID != 0 {
					t.Fatalf("zero child audit identity lost: %v", err)
				}
			})
		}
	}
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 1)
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, "height1", heightFloat(1.25), heightFloat(2))}); err != nil || auditCount(t, db, "CHILD1") != 1 {
		t.Fatalf("default strength1 edit: %v", err)
	}
}

func TestHeight_ValidationAndHistoricalValues(t *testing.T) {
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 1)
	for _, field := range []string{"cover1", "totalA", "height1"} {
		for _, number := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.5e38, -3.5e38} {
			before := heightFloat(1.25)
			if field == "cover1" {
				before = heightFloat(20)
			} else if field == "totalA" {
				before = heightFloat(45)
			}
			if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, field, before, heightFloat(number))}); err == nil {
				t.Fatalf("accepted %s value %v", field, number)
			}
			if math.IsNaN(number) || math.IsInf(number, 0) {
				if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, field, heightFloat(number), nil)}); err == nil {
					t.Fatalf("accepted nonfinite expectation %v", number)
				}
			}
		}
	}
	for _, field := range []string{"cover1", "totalA", "totalB"} {
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, field, heightFloat(20), heightFloat(100))}); err == nil {
			t.Fatal("accepted cover/total100")
		}
	}
	for _, value := range []float64{-99.75, 0, 99.99} {
		before := storedHeight(t, db, "CHILD1", 1, "Cover1")
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, "cover1", before, heightFloat(value))}); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []float64{-math.MaxFloat32, math.MaxFloat32, math.Copysign(0, -1)} {
		before := storedHeight(t, db, "CHILD1", 1, "Height1")
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, "height1", before, heightFloat(value))}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=125,Height1=1e40`); err != nil {
		t.Fatal(err)
	}
	for field, column := range map[string]string{"cover1": "Cover1", "height1": "Height1"} {
		before := storedHeight(t, db, "CHILD1", 1, column)
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, field, before, before)}); err != nil {
			t.Fatalf("cannot preserve historical out-of-range %s: %v", field, err)
		}
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, field, before, nil)}); err != nil {
			t.Fatalf("cannot clear historical out-of-range %s: %v", field, err)
		}
	}
}

func TestHeight_ExpectedConflictKeysAndIdentity(t *testing.T) {
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 1)
	seedHeight(t, db, "CHILD1", 0)
	seedHeight(t, db, "CHILD2", 1)
	for _, update := range []HeightRecordUpdate{
		{ID: 1},
		{ID: 1, Values: map[string]*float64{"height1": nil}},
		{ID: 1, Values: map[string]*float64{"height1": nil}, Expected: map[string]*float64{"cover1": nil}},
		heightUpdate(1, "Height1", nil, nil),
		heightUpdate(1, `height1"=0;--`, nil, nil),
		heightUpdate(1, "species", nil, nil),
		heightUpdate(1, "height1", nil, nil),
		heightUpdate(1, "height1", heightFloat(math.Nextafter(1.25, 2)), nil),
		heightUpdate(1, "cover1", heightFloat(100), heightFloat(100)),
		heightUpdate(999, "height1", heightFloat(1.25), nil),
	} {
		if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{update}); err == nil {
			t.Fatalf("invalid/conflicting update accepted: %+v", update)
		}
	}
	if err := s.UpdateHeightRecords("CHILD1", nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	update := heightUpdate(1, "height1", heightFloat(1.25), heightFloat(2))
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{update, update}); err == nil {
		t.Fatal("duplicate batch ID accepted")
	}
	if err := s.UpdateHeightRecords("missing", []HeightRecordUpdate{update}); err == nil {
		t.Fatal("missing parents accepted")
	}
	if err := s.UpdateHeightRecords("CHILD2", []HeightRecordUpdate{heightUpdate(0, "height1", heightFloat(1.25), nil)}); err == nil {
		t.Fatal("wrong plot accepted")
	}
	seedHeight(t, db, "CHILD1", 1)
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{update}); err == nil {
		t.Fatal("duplicate stored ID accepted")
	}
	seedHeight(t, db, "CHILD2", nil)
	if err := s.UpdateHeightRecords("CHILD2", []HeightRecordUpdate{heightUpdate(0, "height1", nil, nil)}); err == nil {
		t.Fatal("NULL identity treated as0")
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Height1=0 WHERE ID=0`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(0, "height1", heightFloat(math.Copysign(0, -1)), nil)}); err == nil {
		t.Fatal("bit-inexact expected signed zero accepted")
	}
	if auditCount(t, db, "CHILD1") != 0 {
		t.Fatal("rejected updates left audits")
	}
}

func TestHeight_MultirowRollbackAndNoComputedPartners(t *testing.T) {
	for _, failure := range []string{"field", "audit", "conflict", "trigger-conflict", "schema"} {
		t.Run(failure, func(t *testing.T) {
			s, db := childFixture(t)
			s.SetAuditStrength(3)
			seedHeight(t, db, "CHILD1", 1)
			seedHeight(t, db, "CHILD1", 2)
			updates := []HeightRecordUpdate{
				heightUpdate(1, "height1", heightFloat(1.25), heightFloat(2.75)),
				heightUpdate(2, "height1", heightFloat(1.25), heightFloat(3.75)),
			}
			var setupErr error
			switch failure {
			case "field":
				_, setupErr = db.Exec(`CREATE TRIGGER reject_height BEFORE UPDATE ON Sample_Veg WHEN NEW.ID=2 BEGIN SELECT RAISE(ABORT,'field rejection'); END`)
			case "audit":
				_, setupErr = db.Exec(`CREATE TRIGGER reject_height_audit BEFORE INSERT ON Sample_Audit WHEN NEW.ID=2 BEGIN SELECT RAISE(ABORT,'audit rejection'); END`)
			case "conflict":
				updates[1].Expected["height1"] = heightFloat(99)
			case "trigger-conflict":
				_, setupErr = db.Exec(`CREATE TRIGGER move_second AFTER UPDATE ON Sample_Veg WHEN NEW.ID=1 BEGIN UPDATE Sample_Veg SET Height1=99 WHERE ID=2; END`)
			case "schema":
				_, setupErr = db.Exec(`ALTER TABLE Sample_Veg DROP COLUMN Height6`)
				updates[1] = heightUpdate(2, "height6", nil, nil)
			}
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			if err := s.UpdateHeightRecords("CHILD1", updates); err == nil {
				t.Fatalf("%s failed to reject batch", failure)
			}
			for _, id := range []int{1, 2} {
				if value := storedHeight(t, db, "CHILD1", id, "Height1"); value == nil || *value != 1.25 {
					t.Fatalf("row%d not rolled back", id)
				}
			}
			if auditCount(t, db, "CHILD1") != 0 {
				t.Fatal("partial audits survived")
			}
		})
	}
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 1)
	seedHeight(t, db, "CHILD1", 2)
	seedHeight(t, db, "CHILD2", 1)
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{
		heightUpdate(1, "cover1", heightFloat(20), nil),
		heightUpdate(2, "height1", heightFloat(1.25), heightFloat(-2.75)),
	}); err != nil {
		t.Fatal(err)
	}
	for field, value := range map[string]float64{"Height1": 1.25, "Cover2": 30, "Height2": 4.75, "TotalA": 45, "TotalB": 55} {
		if got := storedHeight(t, db, "CHILD1", 1, field); got == nil || *got != value {
			t.Fatalf("partner %s changed", field)
		}
	}
	if got := storedHeight(t, db, "CHILD2", 1, "Height1"); got == nil || *got != 1.25 {
		t.Fatal("same ID in partner plot changed")
	}
}

func TestHeight_JSONShapeNormalizationAndConcurrentExpected(t *testing.T) {
	var update HeightRecordUpdate
	if err := json.Unmarshal([]byte(`{"id":0,"values":{"height1":null},"expected":{"height1":1.25}}`), &update); err != nil ||
		update.ID != 0 || update.Values["height1"] != nil || *update.Expected["height1"] != 1.25 {
		t.Fatalf("nullable DTO shape: %+v %v", update, err)
	}
	for _, input := range []string{
		`{"id":null,"values":{"height1":null},"expected":{"height1":null}}`,
		`{"values":{"height1":null},"expected":{"height1":null}}`,
		`{"id":0.5,"values":{"height1":null},"expected":{"height1":null}}`,
	} {
		if err := json.Unmarshal([]byte(input), &update); err == nil {
			t.Fatalf("invalid JSON identity accepted: %s", input)
		}
	}
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 1)
	if _, err := db.Exec(`UPDATE Sample_Veg SET Height1=2`); err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, "height1", heightFloat(2), heightFloat(1.25))}); err != nil {
		t.Fatalf("SQLite integer normalized read: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, number := range []float64{2.75, 3.75} {
		wg.Add(1)
		go func(number float64) {
			defer wg.Done()
			results <- s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(1, "height1", heightFloat(1.25), heightFloat(number))})
		}(number)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent expectation must allow exactly one writer, got%d", success)
	}
}

func TestHeight_DeletedParentAndReservationCollision(t *testing.T) {
	s, db := childFixture(t)
	seedHeight(t, db, "CHILD1", 0)
	seedHeight(t, db, "CHILD1", -9)
	seedHeight(t, db, "CHILD2", -9)
	if _, err := db.Exec(`CREATE TABLE "__VPRO_ChildIdentity" (ChildTable TEXT,ID INTEGER);
		INSERT INTO "__VPRO_ChildIdentity" VALUES ('"Sample_Veg"',0),('"Sample_Veg"',-9)`); err != nil {
		t.Fatal(err)
	}
	ledger := heightTableSnapshot(t, db, "__VPRO_ChildIdentity")
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{
		heightUpdate(0, "height1", heightFloat(1.25), heightFloat(2.75)),
		heightUpdate(-9, "height1", heightFloat(1.25), nil),
	}); err != nil {
		t.Fatal(err)
	}
	if ledger != heightTableSnapshot(t, db, "__VPRO_ChildIdentity") {
		t.Fatal("reserved/colliding imported identities were altered")
	}
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE PlotNumber='CHILD1' AND ID=0`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(0, "height1", heightFloat(2.75), nil)}); err == nil {
		t.Fatal("deleted row recreated")
	}
	if _, err := db.Exec(`DELETE FROM Sample_Admin WHERE Plot='CHILD1'`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateHeightRecords("CHILD1", []HeightRecordUpdate{heightUpdate(-9, "height1", nil, heightFloat(2))}); err == nil {
		t.Fatal("missing Admin parent accepted")
	}
	if got := storedHeight(t, db, "CHILD1", -9, "Height1"); got != nil {
		t.Fatal("missing-parent update changed data")
	}
}
