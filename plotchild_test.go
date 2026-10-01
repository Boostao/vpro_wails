package main

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

func childFixture(t *testing.T) (*PlotService, *sql.DB) {
	t.Helper()
	root, err := os.MkdirTemp(".", ".child-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	projects, err := NewProjectServiceWithConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := NewPlotService(projects)
	service.SetCurrentUser("ChildTester")
	db, _, err := service.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	for _, plot := range []string{"CHILD1", "CHILD2"} {
		if err := service.CreatePlot(FS882Header{PlotNumber: plot}); err != nil {
			t.Fatal(err)
		}
	}
	return service, db
}

func childRecord(kind, plot string, id int64, full bool) reflect.Value {
	var record any
	switch kind {
	case "Veg":
		record = &VegRecord{Species: "PSEUMEN"}
	case "Humus":
		record = &HumusRecord{}
	case "Mineral":
		record = &MineralRecord{}
	case "Other":
		record = &OtherRecord{}
	}
	value := reflect.ValueOf(record).Elem()
	value.FieldByName("PlotNumber").SetString(plot)
	value.FieldByName("ID").SetInt(id)
	if full {
		present := map[string]bool{}
		for i, field := range childFields[kind] {
			member := value.FieldByName(field.member)
			if member.Kind() != reflect.Pointer {
				continue
			}
			data := reflect.New(member.Type().Elem())
			switch data.Elem().Kind() {
			case reflect.Float64:
				data.Elem().SetFloat(float64(i) + 0.25)
			case reflect.Int:
				data.Elem().SetInt(int64(i + 1))
			case reflect.Bool:
				data.Elem().SetBool(i%2 == 0)
			default:
				data.Elem().SetString("old-'quoted'")
			}
			member.Set(data)
			present[strings.ToLower(childJSONKey(value, field))] = true
		}
		setChildPresence(value.Addr().Interface(), present)
	}
	return value
}

func normalizeChildStored(kind string, field childField, stored any) any {
	if integer, ok := stored.(int64); ok {
		member := childRecord(kind, "", 0, false).FieldByName(field.member)
		if member.Kind() == reflect.Pointer && member.Type().Elem().Kind() == reflect.Int {
			return int(integer)
		}
		if member.Kind() == reflect.Pointer && member.Type().Elem().Kind() == reflect.Bool {
			return integer != 0
		}
	}

	return stored
}

func childStoredColumn(kind string, field childField) string {
	column := quoteHeaderIdentifier(field.column)
	member := childRecord(kind, "", 0, false).FieldByName(field.member)
	if member.Kind() == reflect.Pointer && member.Type().Elem().Kind() == reflect.Bool {
		return "CAST(" + column + " AS INTEGER)"
	}
	return column
}

func editChildMember(member reflect.Value) {
	if member.Kind() == reflect.Pointer {
		member = member.Elem()
	}
	switch member.Kind() {
	case reflect.Float64:
		member.SetFloat(member.Float() + 0.5)
	case reflect.Int:
		member.SetInt(member.Int() + 1)
	case reflect.Bool:
		member.SetBool(!member.Bool())
	default:
		member.SetString("new-'quoted'")
	}
}

func saveChildRecord(s *PlotService, value reflect.Value) error {
	switch r := value.Interface().(type) {
	case VegRecord:
		return s.SaveVegRecord(r)
	case HumusRecord:
		return s.SaveHumusRecord(r)
	case MineralRecord:
		return s.SaveMineralRecord(r)
	case OtherRecord:
		return s.SaveOtherRecord(r)
	}
	panic("unknown child")
}

func updateChildRecord(s *PlotService, value reflect.Value) error {
	switch r := value.Interface().(type) {
	case VegRecord:
		return s.UpdateVegRecord(r)
	case HumusRecord:
		return s.UpdateHumusRecord(r)
	case MineralRecord:
		return s.UpdateMineralRecord(r)
	case OtherRecord:
		return s.UpdateOtherRecord(r)
	}
	panic("unknown child")
}

func deleteChildRecord(s *PlotService, kind, plot string, id int64) error {
	switch kind {
	case "Veg":
		return s.DeleteVegRecord(plot, id)
	case "Humus":
		return s.DeleteHumusRecord(plot, id)
	case "Mineral":
		return s.DeleteMineralRecord(plot, id)
	case "Other":
		return s.DeleteOtherRecord(plot, id)
	}
	panic("unknown child")
}

func childCount(t *testing.T, db *sql.DB, kind, plot string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Sample_`+kind+`" WHERE PlotNumber = ?`, plot).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func childID(t *testing.T, db *sql.DB, kind, plot string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT ID FROM "Sample_`+kind+`" WHERE PlotNumber = ?`, plot).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func checkChildAudits(t *testing.T, s *PlotService, kind string, id int64, before, after map[string]any) {
	t.Helper()
	audits, err := s.ListAuditEntries("CHILD1")
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != len(after) {
		t.Fatalf("audit count = %d, want %d", len(audits), len(after))
	}
	seen := map[string]bool{}
	for _, audit := range audits {
		expectedAfter, ok := after[audit.EditField]
		if !ok || seen[audit.EditField] {
			t.Fatalf("unexpected/duplicate audit field: %s", audit.EditField)
		}
		seen[audit.EditField] = true
		if audit.ID == nil || *audit.ID != id || audit.Table != "_"+kind ||
			audit.Project != "Sample" || audit.User != "ChildTester" || audit.PlotNumber != "CHILD1" ||
			audit.Restore || audit.Flag || audit.EditWhen == "" {
			t.Fatalf("audit metadata = %+v", audit)
		}
		if !reflect.DeepEqual(headerAuditValue(before[audit.EditField]), derefAudit(audit.BeforeEdit)) ||
			!reflect.DeepEqual(headerAuditValue(expectedAfter), derefAudit(audit.AfterEdit)) {
			t.Fatalf("audit values for %s = %v -> %v, want %v -> %v", audit.EditField,
				derefAudit(audit.BeforeEdit), derefAudit(audit.AfterEdit), before[audit.EditField], expectedAfter)
		}
	}
}

func derefAudit(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func childValues(kind string, record reflect.Value) map[string]any {
	result := map[string]any{}
	for _, field := range childFields[kind] {
		if value := childValue(record, field); value != nil {
			result[field.column] = value
		}
	}
	return result
}

func clearChildAudit(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DELETE FROM Sample_Audit WHERE PlotNumber = 'CHILD1'`); err != nil {
		t.Fatal(err)
	}
}

func TestPlotChild_FieldAuditAndNullThresholds(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		for strength := 0; strength <= 3; strength++ {
			t.Run(fmt.Sprintf("%s/strength%d", kind, strength), func(t *testing.T) {
				s, db := childFixture(t)
				s.SetAuditStrength(strength)
				record := childRecord(kind, "CHILD1", 0, true)
				original := childValues(kind, record)
				if err := saveChildRecord(s, record); err != nil {
					t.Fatal(err)
				}
				id := childID(t, db, kind, "CHILD1")
				record.FieldByName("ID").SetInt(id)
				expected := map[string]any{}
				if strength >= 2 {
					expected = original
				}
				checkChildAudits(t, s, kind, id, nil, expected)
				clearChildAudit(t, db)
				if err := saveChildRecord(s, record); err != nil {
					t.Fatal(err)
				}
				checkChildAudits(t, s, kind, id, nil, nil)
				for _, field := range childFields[kind] {
					editChildMember(record.FieldByName(field.member))
				}
				edited := childValues(kind, record)
				if err := saveChildRecord(s, record); err != nil {
					t.Fatal(err)
				}
				expected = nil
				if strength >= 1 {
					expected = edited
				}
				checkChildAudits(t, s, kind, id, original, expected)
				// Verify every field persisted, including formerly omitted Veg fields.
				for _, field := range childFields[kind] {
					var stored any
					if err := db.QueryRow(`SELECT `+childStoredColumn(kind, field)+` FROM "Sample_`+kind+`" WHERE PlotNumber = 'CHILD1' AND ID = ?`, id).Scan(&stored); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(normalizeChildStored(kind, field, stored), edited[field.column]) {
						t.Fatalf("%s not persisted: %v != %v", field.column, stored, edited[field.column])
					}
				}
				clearChildAudit(t, db)
				nulls := map[string]any{}
				for _, field := range childFields[kind] {
					member := record.FieldByName(field.member)
					if member.Kind() == reflect.Pointer {
						member.SetZero()
						nulls[field.column] = nil
					}
				}
				if err := saveChildRecord(s, record); err != nil {
					t.Fatal(err)
				}
				expected = nil
				if strength == 3 {
					expected = nulls
				}
				checkChildAudits(t, s, kind, id, edited, expected)
				for field := range nulls {
					var stored any
					if err := db.QueryRow(`SELECT `+quoteHeaderIdentifier(field)+` FROM "Sample_`+kind+`" WHERE PlotNumber = 'CHILD1' AND ID = ?`, id).Scan(&stored); err != nil || stored != nil {
						t.Fatalf("nullable clearing %s: %v %v", field, stored, err)
					}
				}
				clearChildAudit(t, db)
				readded := childRecord(kind, "CHILD1", id, true)
				if kind == "Veg" {
					readded.FieldByName("Species").SetString("new-'quoted'")
				}
				if err := saveChildRecord(s, readded); err != nil {
					t.Fatal(err)
				}
				expected = map[string]any{}
				if strength >= 2 {
					for field := range nulls {
						expected[field] = childValues(kind, readded)[field]
					}
				}
				checkChildAudits(t, s, kind, id, nil, expected)
				clearChildAudit(t, db)
				if err := deleteChildRecord(s, kind, "CHILD1", id); err != nil {
					t.Fatal(err)
				}
				expected = map[string]any{}
				if strength == 3 {
					for field := range childValues(kind, readded) {
						expected[field] = nil
					}
				}
				checkChildAudits(t, s, kind, id, childValues(kind, readded), expected)
				if childCount(t, db, kind, "CHILD1") != 0 {
					t.Fatal("delete did not remove row")
				}
			})
		}
	}
}

func TestPlotChild_IdentityAllocationAndParents(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := childFixture(t)
			s.SetAuditStrength(3)
			// Preserve imported negative and extreme IDs; occupy allocator's
			// first candidate and leave a gap before the maximum signed ID.
			for _, id := range []int64{-17, 1, math.MaxInt64} {
				query := `INSERT INTO "Sample_` + kind + `" (PlotNumber,ID) VALUES ('CHILD2',?)`
				if kind == "Veg" {
					query = `INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES ('CHILD2',?,'IMPORTED')`
				}
				if _, err := db.Exec(query, id); err != nil {
					t.Fatal(err)
				}
			}
			importedRows := 3
			if kind == "Veg" {
				if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES ('CHILD2',1,'DUPID')`); err != nil {
					t.Fatal(err)
				}
				importedRows++
			}
			record := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			if id := childID(t, db, kind, "CHILD1"); id != 2 {
				t.Fatalf("allocation = %d, want unused gap 2", id)
			}
			for _, id := range []int64{-17, math.MaxInt64} {
				imported := childRecord(kind, "CHILD2", id, true)
				if err := saveChildRecord(s, imported); err != nil {
					t.Fatalf("signed imported identity: %v", err)
				}
			}
			initialAudit := auditCount(t, db, "CHILD1")
			for _, id := range []int64{-17, 999} {
				record.FieldByName("ID").SetInt(id)
				if err := saveChildRecord(s, record); err == nil {
					t.Fatal("cross-plot/stale update accepted")
				}
				if err := deleteChildRecord(s, kind, "CHILD1", id); err == nil {
					t.Fatal("cross-plot/stale delete accepted")
				}
			}
			for _, missing := range []string{"ABSENT", "ENVONLY", "ADMONLY", " "} {
				if missing == "ENVONLY" {
					if _, err := db.Exec(`INSERT INTO Sample_Env (PlotNumber) VALUES ('ENVONLY')`); err != nil {
						t.Fatal(err)
					}
				}
				if missing == "ADMONLY" {
					if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`INSERT INTO Sample_Admin (Plot) VALUES ('ADMONLY')`); err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
						t.Fatal(err)
					}
				}
				if err := saveChildRecord(s, childRecord(kind, missing, 0, true)); err == nil {
					t.Fatalf("missing parent %s create accepted", missing)
				}
				if err := saveChildRecord(s, childRecord(kind, missing, 2, true)); err == nil {
					t.Fatalf("missing parent %s update accepted", missing)
				}
				if err := deleteChildRecord(s, kind, missing, 2); err == nil {
					t.Fatalf("missing parent %s delete accepted", missing)
				}
			}
			if auditCount(t, db, "CHILD1") != initialAudit || childCount(t, db, kind, "CHILD1") != 1 ||
				childCount(t, db, kind, "CHILD2") != importedRows {
				t.Fatal("rejected operations changed data/audit")
			}

			if err := deleteChildRecord(s, kind, "CHILD1", 2); err != nil {
				t.Fatal(err)
			}
			record.FieldByName("ID").SetInt(2)
			if err := saveChildRecord(s, record); err == nil {
				t.Fatal("stale deleted identity recreated")
			}
			if err := saveChildRecord(s, childRecord(kind, "CHILD1", 0, true)); err != nil {
				t.Fatal(err)
			}
			if id := childID(t, db, kind, "CHILD1"); id != 3 {
				t.Fatalf("deleted identity reused: allocated %d, want 3", id)
			}
			initialAudit = auditCount(t, db, "CHILD1")
			if err := saveChildRecord(s, record); err == nil {
				t.Fatal("stale deleted identity overwrote newly allocated row")
			}
			if auditCount(t, db, "CHILD1") != initialAudit {
				t.Fatal("stale identity changed audit")
			}
		})
	}
}

func TestPlotChild_UnmappedVegFieldsPreserved(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,HeightA)
		VALUES ('CHILD1',-91,'IMPORTED',123.75)`); err != nil {
		t.Fatal(err)
	}
	record := childRecord("Veg", "CHILD1", -91, true)
	if err := saveChildRecord(s, record); err != nil {
		t.Fatal(err)
	}
	var height float64
	if err := db.QueryRow(`SELECT HeightA FROM Sample_Veg WHERE PlotNumber = 'CHILD1' AND ID = -91`).Scan(&height); err != nil || height != 123.75 {
		t.Fatalf("unmapped HeightA changed: %v %v", height, err)
	}
	for _, species := range []string{"", " \t"} {
		if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD1", ID: -91, Species: species}); err == nil {
			t.Fatal("required species accepted empty value")
		}
	}
	audits, err := s.ListAuditEntries("CHILD1")
	if err != nil {
		t.Fatal(err)
	}
	for _, audit := range audits {
		if audit.EditField == "HeightA" || audit.EditField == "ID" || audit.EditField == "PlotNumber" {
			t.Fatalf("unmapped/identity field audited: %s", audit.EditField)
		}
	}
}

func TestPlotChild_VegDuplicateIdentity(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species) VALUES
		('CHILD1',-55,'ONE'),('CHILD1',-55,'TWO'),('CHILD2',-55,'OTHER'),
		('CHILD1',0,'ZERO'),('CHILD1',NULL,'NULLID')`); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD1", ID: -55, Species: "NO"}); err == nil {
		t.Fatal("ambiguous duplicate update accepted")
	}
	if err := s.DeleteVegRecord("CHILD1", -55); err == nil {
		t.Fatal("ambiguous duplicate delete accepted")
	}
	if auditCount(t, db, "CHILD1") != 0 || childCount(t, db, "Veg", "CHILD1") != 4 {
		t.Fatal("ambiguous identity changed data/audit")
	}
	if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD2", ID: -55, Species: "UPDATED"}); err != nil {
		t.Fatalf("IDs need not be globally unique for Veg: %v", err)
	}
	if err := s.DeleteVegRecord("CHILD2", -55); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveVegRecord(VegRecord{PlotNumber: "CHILD1", Species: "NEW"}); err != nil {
		t.Fatal(err)
	}
	if childCount(t, db, "Veg", "CHILD1") != 5 {
		t.Fatal("create overwrote imported zero/NULL identity")
	}
}

func TestPlotChild_NonfiniteNumbers(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral"} {
		t.Run(kind, func(t *testing.T) {
			s, db := childFixture(t)
			seed := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, seed); err != nil {
				t.Fatal(err)
			}
			id := childID(t, db, kind, "CHILD1")
			initialAudit := auditCount(t, db, "CHILD1")
			for _, field := range childFields[kind] {
				member := seed.FieldByName(field.member)
				if member.Kind() != reflect.Pointer || member.Type().Elem().Kind() != reflect.Float64 {
					continue
				}
				for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
					for _, identity := range []int64{0, id} {
						record := childRecord(kind, "CHILD1", identity, true)
						record.FieldByName(field.member).Elem().SetFloat(invalid)
						if err := saveChildRecord(s, record); err == nil {
							t.Fatalf("%s invalid number accepted on identity %d", field.column, identity)
						}
					}
				}
			}
			if childCount(t, db, kind, "CHILD1") != 1 || auditCount(t, db, "CHILD1") != initialAudit {
				t.Fatal("invalid numeric mutation changed data/audit")
			}
		})
	}
}

func TestPlotChild_TriggerRollback(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		for _, operation := range []string{"create", "update", "delete"} {
			for _, failing := range []string{"audit", "data"} {
				t.Run(kind+"/"+operation+"/"+failing, func(t *testing.T) {
					s, db := childFixture(t)
					s.SetAuditStrength(3)
					record := childRecord(kind, "CHILD1", 0, true)
					if operation != "create" {
						if err := saveChildRecord(s, record); err != nil {
							t.Fatal(err)
						}

						record.FieldByName("ID").SetInt(childID(t, db, kind, "CHILD1"))
					}
					clearChildAudit(t, db)
					original := childValues(kind, record)
					for _, field := range childFields[kind] {
						editChildMember(record.FieldByName(field.member))
					}
					var trigger string
					if failing == "audit" {
						fields := childFields[kind]
						trigger = `CREATE TRIGGER child_failure BEFORE INSERT ON Sample_Audit WHEN NEW.EditField = '` +
							fields[len(fields)-1].column + `' BEGIN SELECT RAISE(ABORT,'audit failure'); END`
					} else {
						event := map[string]string{"create": "INSERT", "update": "UPDATE", "delete": "DELETE"}[operation]
						trigger = `CREATE TRIGGER child_failure AFTER ` + event + ` ON "Sample_` + kind +
							`" BEGIN SELECT RAISE(ABORT,'data failure'); END`
					}
					if _, err := db.Exec(trigger); err != nil {
						t.Fatal(err)
					}
					var err error
					if operation == "delete" {
						err = deleteChildRecord(s, kind, "CHILD1", record.FieldByName("ID").Int())
					} else {
						err = saveChildRecord(s, record)
					}
					if err == nil {
						t.Fatal("trigger did not reject mutation")
					}
					expectedRows := 1
					if operation == "create" {
						expectedRows = 0
					}
					if childCount(t, db, kind, "CHILD1") != expectedRows || auditCount(t, db, "CHILD1") != 0 {
						t.Fatal("data or partial audit escaped rollback")
					}
					if operation == "create" {
						var tables int
						if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = '__VPRO_ChildIdentity'`).Scan(&tables); err != nil || tables != 0 {
							t.Fatalf("identity reservation escaped rollback: %d %v", tables, err)
						}
					}
					if operation != "create" {
						for _, field := range childFields[kind] {
							want := original[field.column]
							var stored any
							if err := db.QueryRow(`SELECT ` + childStoredColumn(kind, field) + ` FROM "Sample_` + kind + `" WHERE PlotNumber = 'CHILD1'`).Scan(&stored); err != nil ||
								!reflect.DeepEqual(normalizeChildStored(kind, field, stored), want) {
								t.Fatalf("rollback field %s = %v, want %v; %v", field.column, stored, want, err)
							}
						}
					}
				})
			}
		}
	}
}
