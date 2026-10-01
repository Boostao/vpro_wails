package main

import (
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func restoreAuditFixture(t *testing.T, db *sql.DB, plot, table, field string, id *int64, before, after any) string {
	t.Helper()
	result, err := db.Exec(`INSERT INTO Sample_Audit
		(Project,User,PlotNumber,"Table",EditField,EditWhen,BeforeEdit,AfterEdit,Restore,Flag,ID)
		VALUES ('Sample','RestoreTester',?,?,?,'2026-09-30 01:00:00',?,?,0,0,?)`,
		plot, table, field, headerAuditValue(before), headerAuditValue(after), id)
	if err != nil {
		t.Fatal(err)
	}
	rowID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return strconv.FormatInt(rowID, 10)
}

func restoreFixture(t *testing.T) (*PlotService, *sql.DB) {
	t.Helper()
	s, db := childFixture(t)
	if _, err := db.Exec(`DELETE FROM Sample_Audit`); err != nil {
		t.Fatal(err)
	}
	return s, db
}

func assertRestoreField(t *testing.T, db *sql.DB, table, field, plot string, id *int64, want any) {
	t.Helper()
	key, args := `"PlotNumber" = ?`, []any{plot}
	if table == "Admin" {
		key = `"Plot" = ?`
	} else if id != nil {
		key += ` AND "ID" = ?`
		args = append(args, *id)
	}
	var got *string
	if err := db.QueryRow(`SELECT CAST(`+quoteHeaderIdentifier(field)+` AS TEXT) FROM Sample_`+table+` WHERE `+key, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	var actual any
	if got != nil {
		actual = *got
		switch want := want.(type) {
		case float64:
			number, err := strconv.ParseFloat(*got, 64)
			if err != nil || number != want {
				t.Fatalf("%s.%s = %v, want %v", table, field, actual, want)
			}
			return
		case int:
			number, err := strconv.ParseInt(*got, 10, 64)
			if err != nil || number != int64(want) {
				t.Fatalf("%s.%s = %v, want %v", table, field, actual, want)
			}
			return
		}
	}
	if !reflect.DeepEqual(actual, headerAuditValue(want)) {
		t.Fatalf("%s.%s = %v, want %v", table, field, actual, headerAuditValue(want))
	}
}

func TestPlotRestore_SelectionExactIDsCancelAndEmpty(t *testing.T) {
	s, db := restoreFixture(t)
	if _, err := db.Exec(`UPDATE Sample_Env SET Elevation = 2 WHERE PlotNumber = 'CHILD1'`); err != nil {
		t.Fatal(err)
	}
	row := restoreAuditFixture(t, db, "CHILD1", "_Env", "Elevation", nil, 1, 2)
	if _, err := db.Exec(`UPDATE Sample_Audit SET rowid = 9007199254740993, Restore = -1, Flag = -1 WHERE rowid = ?`, row); err != nil {
		t.Fatal(err)
	}
	row = "9007199254740993"
	entries, err := s.ListAuditEntries("CHILD1")
	if err != nil || len(entries) != 1 || entries[0].RowID != row || !entries[0].Restore || !entries[0].Flag {
		t.Fatalf("exact identity/Access flags: %+v %v", entries, err)
	}
	foreign := restoreAuditFixture(t, db, "CHILD2", "_Env", "Elevation", nil, 1, 2)
	for _, ids := range [][]string{{row, row}, {foreign}, {"9223372036854775808"}, {"1.0"}, {"+1"}, {"01"}, {" 1"}, {"999999"}} {
		if err := s.SetAuditRestoreSelection("CHILD1", ids); err == nil {
			t.Fatalf("invalid selection accepted: %v", ids)
		}
		entries, err := s.ListAuditEntries("CHILD1")
		if err != nil || !entries[0].Restore {
			t.Fatal("invalid selection changed flags")
		}
	}
	if err := s.SetAuditRestoreSelection("CHILD1", nil); err != nil {
		t.Fatal(err)
	}
	entries, _ = s.ListAuditEntries("CHILD1")
	if entries[0].Restore {
		t.Fatal("empty selection did not clear flags")
	}
	if err := s.SetAuditRestoreSelection("CHILD1", []string{row}); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{row}, AuditRestoreCancel)
	if err != nil || !result.Cancelled || result.RestoredRows != 0 {
		t.Fatalf("cancel: %+v %v", result, err)
	}
	result, err = s.RestoreSelectedAuditRecords("CHILD1", nil, AuditRestorePrune)
	if err != nil || *result != (AuditRestoreResult{}) {
		t.Fatalf("empty explicit selection: %+v %v", result, err)
	}
	assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 2)
	if auditCount(t, db, "CHILD1") != 1 {
		t.Fatal("cancel/empty selection pruned history")
	}
	if _, err := s.RestoreSelectedAuditRecords("CHILD1", nil, "unexpected"); err == nil {
		t.Fatal("invalid action accepted")
	}
	if err := s.RestoreAuditRecords("CHILD1", true); err != nil {
		t.Fatal(err)
	}
	assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 1)
	if auditCount(t, db, "CHILD1") != 0 || auditCount(t, db, "CHILD2") != 1 {
		t.Fatal("marked restore did not prune exact owned rows")
	}
	if err := s.RestoreAuditRecords("CHILD1", true); err != nil {
		t.Fatalf("no marked rows must be a no-op: %v", err)
	}
}

func TestPlotRestore_AllVerifiedFieldsTypedRoundTrip(t *testing.T) {
	for _, kind := range []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := restoreFixture(t)
			if err := s.SetCurrentUser("Will MacKenzie"); err != nil {
				t.Fatal(err)
			}
			var fields []childField
			var record reflect.Value
			var id *int64
			if kind == "Env" || kind == "Admin" {
				record = reflect.ValueOf(fullHeader("CHILD1"))
				for _, field := range headerFields {
					if field.table == kind && field.property != "plotNumber" {
						fields = append(fields, childField{field.member, field.column})
					}
				}
			} else {
				zero := int64(0)
				id = &zero
				record = childRecord(kind, "CHILD1", zero, true)
				fields = childFields[kind]
				insert := `INSERT INTO Sample_` + kind + ` (PlotNumber,ID) VALUES ('CHILD1',0)`
				if kind == "Veg" {
					insert = `INSERT INTO Sample_Veg (PlotNumber,ID,Species,Cover1) VALUES ('CHILD1',0,'CURRENT',0)`
				}
				if _, err := db.Exec(insert); err != nil {
					t.Fatal(err)
				}
			}
			key := `"PlotNumber" = 'CHILD1'`
			if kind == "Admin" {
				key = `"Plot" = 'CHILD1'`
			} else if id != nil {
				key += ` AND ID = 0`
			}
			var selected []string
			for _, field := range fields {
				if kind == "Veg" && strings.HasPrefix(field.column, "Cover") {
					continue
				}
				member := record.FieldByName(field.member)
				before := childValue(record, field)
				if member.Kind() == reflect.Pointer {
					member = member.Elem()
				}
				var after any
				switch member.Kind() {
				case reflect.String:
					after = "after-'quoted'"
				case reflect.Float64:
					after = -1.125
				case reflect.Int:
					after = 0
				case reflect.Bool:
					after = !member.Bool()
				}
				if _, err := db.Exec(`UPDATE Sample_`+kind+` SET `+quoteHeaderIdentifier(field.column)+` = ? WHERE `+key,
					headerStorageValue(after)); err != nil {
					t.Fatal(err)
				}
				selected = append(selected, restoreAuditFixture(t, db, "CHILD1", "Sample_"+kind, field.column, id, before, after))
			}
			for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
				if action == AuditRestorePrune {
					// Reverse audit direction to exercise nullable clearing and
					// false booleans independently of the full non-null fixture.
					for _, field := range fields {
						if kind == "Veg" && strings.HasPrefix(field.column, "Cover") {
							continue
						}
						if _, err := db.Exec(`UPDATE Sample_Audit SET AfterEdit = BeforeEdit,
							BeforeEdit = CASE WHEN EditField = 'Species' THEN 'NULLTEST' ELSE NULL END
							WHERE PlotNumber = 'CHILD1' AND EditField = ?`, field.column); err != nil {
							t.Fatal(err)
						}
					}
				}
				result, err := s.RestoreSelectedAuditRecords("CHILD1", selected, action)
				if err != nil || result.RestoredRows != len(selected) || result.CleanedVegRows != 0 {
					t.Fatalf("%s restore: %+v %v", action, result, err)
				}
				for _, field := range fields {
					if kind == "Veg" && strings.HasPrefix(field.column, "Cover") {
						continue
					}
					want := childValue(record, field)
					if action == AuditRestorePrune {
						want = nil
						if field.column == "Species" {
							want = "NULLTEST"
						}
					}
					assertRestoreField(t, db, kind, field.column, "CHILD1", id, want)
				}
				count := len(selected)
				if action == AuditRestorePrune {
					count = 0
				}
				if auditCount(t, db, "CHILD1") != count {
					t.Fatalf("%s changed the wrong audit rows", action)
				}
			}
		})
	}
}

func TestPlotRestore_OrderingAndStaleChain(t *testing.T) {
	for _, sameTime := range []bool{false, true} {
		t.Run(fmt.Sprint(sameTime), func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`UPDATE Sample_Env SET Elevation = 3 WHERE PlotNumber = 'CHILD1'`); err != nil {
				t.Fatal(err)
			}
			first := restoreAuditFixture(t, db, "CHILD1", "_Env", "Elevation", nil, 1, 2)
			second := restoreAuditFixture(t, db, "CHILD1", "Sample_Env", "Elevation", nil, 2, 3)
			if !sameTime {
				if _, err := db.Exec(`UPDATE Sample_Audit SET EditWhen = '2026-09-30 01:00:01' WHERE rowid = ?`, second); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{first}, AuditRestorePrune); err == nil {
				t.Fatal("older noncontiguous selection bypassed stale guard")
			}
			if err := s.SetAuditRestoreSelection("CHILD1", []string{first}); err == nil {
				t.Fatal("stale selection bypassed history-chain validation")
			}
			assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 3)
			result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{first, second}, AuditRestoreRetain)
			if err != nil || result.RestoredRows != 2 {
				t.Fatalf("reverse ordered chain: %+v %v", result, err)
			}
			assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 1)
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{first, second}, AuditRestorePrune); err == nil {
				t.Fatal("retained history restored twice without stale error")
			}
			if auditCount(t, db, "CHILD1") != 2 {
				t.Fatal("stale retained restore removed history")
			}
		})
	}
	// A later stale row must roll back earlier successful field restoration.
	s, db := restoreFixture(t)
	if _, err := db.Exec(`UPDATE Sample_Env SET Elevation = 3, Aspect = 99 WHERE PlotNumber = 'CHILD1'`); err != nil {
		t.Fatal(err)
	}
	second := restoreAuditFixture(t, db, "CHILD1", "_Env", "Aspect", nil, 1, 2)
	first := restoreAuditFixture(t, db, "CHILD1", "_Env", "Elevation", nil, 1, 3)
	if _, err := s.RestoreSelectedAuditRecords("CHILD1", []string{first, second}, AuditRestorePrune); err == nil {
		t.Fatal("stale mixed selection accepted")
	}
	assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 3)
	assertRestoreField(t, db, "Env", "Aspect", "CHILD1", nil, 99)
	if auditCount(t, db, "CHILD1") != 2 {
		t.Fatal("stale failure pruned audit")
	}
}

func TestPlotRestore_CoverGuardWithoutCleanup(t *testing.T) {
	for _, failure := range []string{"", "cover", "missing-unrelated-column", "delete-trigger"} {
		t.Run("case-"+failure, func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,Cover1,Cover5a,Height1) VALUES
					('CHILD1',1,'CURRENT',0,NULL,2),
					('CHILD1',2,'EMPTY',NULL,NULL,8),
					('CHILD1',3,'SOURCE5A',NULL,50,8),
					('CHILD2',4,'FOREIGN',NULL,NULL,8)`); err != nil {
				t.Fatal(err)
			}
			id := int64(1)
			field := "Height1"
			var before, after any = 1.25, 2.0
			if failure == "cover" {
				field, before, after = "Cover1", 10.25, 0.0
			}
			row := restoreAuditFixture(t, db, "CHILD1", "_Veg", field, &id, before, after)
			if failure == "missing-unrelated-column" {
				if _, err := db.Exec(`ALTER TABLE Sample_Veg RENAME COLUMN Cover10 TO UnsupportedCover`); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "delete-trigger" {
				if _, err := db.Exec(`CREATE TRIGGER fail_clean BEFORE DELETE ON Sample_Veg
						BEGIN SELECT RAISE(ABORT,'clean failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{row}, AuditRestorePrune)
			if failure == "cover" {
				if err == nil {
					t.Fatal("unsupported cover restore accepted")
				}
				assertRestoreField(t, db, "Veg", "Height1", "CHILD1", &id, 2.0)
				if childCount(t, db, "Veg", "CHILD1") != 3 || auditCount(t, db, "CHILD1") != 1 {
					t.Fatal("failed cover restore left partial changes")
				}
				return
			}
			if err != nil || result.RestoredRows != 1 || result.CleanedVegRows != 0 || result.PrunedAuditRows != 1 {
				t.Fatalf("field-only restore: %+v %v", result, err)
			}
			assertRestoreField(t, db, "Veg", "Height1", "CHILD1", &id, 1.25)
			if childCount(t, db, "Veg", "CHILD1") != 3 || childCount(t, db, "Veg", "CHILD2") != 1 {
				t.Fatal("field restoration removed unrelated vegetation")
			}
		})
	}
}

func TestPlotRestore_TriggerRollbackAndTableOrder(t *testing.T) {
	for _, failure := range []string{"", "data-update", "data-ignore", "audit-delete", "selection-update"} {
		t.Run("case-"+failure, func(t *testing.T) {
			s, db := restoreFixture(t)
			if _, err := db.Exec(`UPDATE Sample_Env SET Elevation = 2 WHERE PlotNumber = 'CHILD1';
					UPDATE Sample_Admin SET OfficeNotes = 'after' WHERE Plot = 'CHILD1';
					CREATE TABLE RestoreOrder (Target TEXT);
					CREATE TRIGGER order_admin AFTER UPDATE OF OfficeNotes ON Sample_Admin
					BEGIN INSERT INTO RestoreOrder VALUES ('Admin'); END;
					CREATE TRIGGER order_env AFTER UPDATE OF Elevation ON Sample_Env
					BEGIN INSERT INTO RestoreOrder VALUES ('Env'); END`); err != nil {
				t.Fatal(err)
			}
			env := restoreAuditFixture(t, db, "CHILD1", "_Env", "Elevation", nil, 1, 2)
			admin := restoreAuditFixture(t, db, "CHILD1", "_Admin", "OfficeNotes", nil, "before", "after")
			if err := s.SetAuditRestoreSelection("CHILD1", []string{admin}); err != nil {
				t.Fatal(err)
			}
			query := ""
			switch failure {
			case "data-update":
				query = `CREATE TRIGGER fail_restore BEFORE UPDATE ON Sample_Env
						BEGIN SELECT RAISE(ABORT,'restore failure'); END`
			case "data-ignore":
				query = `CREATE TRIGGER fail_restore BEFORE UPDATE ON Sample_Env
						BEGIN SELECT RAISE(IGNORE); END`
			case "audit-delete":
				query = `CREATE TRIGGER fail_prune BEFORE DELETE ON Sample_Audit
						WHEN OLD."Table" = '_Env' BEGIN SELECT RAISE(ABORT,'prune failure'); END`
			case "selection-update":
				query = `CREATE TRIGGER fail_selection BEFORE UPDATE ON Sample_Audit
						WHEN NEW.Restore = -1 AND OLD."Table" = '_Env'
						BEGIN SELECT RAISE(ABORT,'selection failure'); END`
			}
			if query != "" {
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "selection-update" {
				if err := s.SetAuditRestoreSelection("CHILD1", []string{env}); err == nil {
					t.Fatal("expected selection update failure")
				}
				entries, err := s.ListAuditEntries("CHILD1")
				if err != nil || len(entries) != 2 {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.Restore != (entry.RowID == admin) {
						t.Fatal("selection failure did not roll back flags")
					}
				}
				return
			}
			result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{env, admin}, AuditRestorePrune)
			if failure == "" {
				if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
					t.Fatalf("restore order: %+v %v", result, err)
				}
				var order string
				if err := db.QueryRow(`SELECT group_concat(Target, ',') FROM RestoreOrder`).Scan(&order); err != nil || order != "Admin,Env" {
					t.Fatalf("canonical table order: %q %v", order, err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected transactional failure")
			}
			assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 2)
			assertRestoreField(t, db, "Admin", "OfficeNotes", "CHILD1", nil, "after")
			if auditCount(t, db, "CHILD1") != 2 {
				t.Fatal("trigger failure pruned audit")
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM RestoreOrder`).Scan(&count); err != nil || count != 0 {
				t.Fatal("trigger failure left data/trigger side effects")
			}
		})
	}
}

func TestPlotRestore_BooleanLegacyEncodingAndSelectivePrune(t *testing.T) {
	s, db := restoreFixture(t)
	if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete = 1, Elevation = 2 WHERE PlotNumber = 'CHILD1';
			INSERT INTO Sample_Other (PlotNumber, ID, UserFlag1) VALUES ('CHILD1',-2147483648,-1)`); err != nil {
		t.Fatal(err)
	}
	row := restoreAuditFixture(t, db, "CHILD1", "_Env", "SpeciesListComplete", nil, false, true)
	id := int64(math.MinInt32)
	child := restoreAuditFixture(t, db, "CHILD1", "_Other", "UserFlag1", &id, nil, true)
	unselected := restoreAuditFixture(t, db, "CHILD1", "_Env", "Elevation", nil, 1, 2)
	if _, err := db.Exec(`UPDATE Sample_Audit SET Restore = -1 WHERE rowid = ?`, unselected); err != nil {
		t.Fatal(err)
	}
	result, err := s.RestoreSelectedAuditRecords("CHILD1", []string{row, child}, AuditRestorePrune)
	if err != nil || result.RestoredRows != 2 || result.PrunedAuditRows != 2 {
		t.Fatalf("boolean restore: %+v %v", result, err)
	}
	assertRestoreField(t, db, "Env", "SpeciesListComplete", "CHILD1", nil, false)
	assertRestoreField(t, db, "Other", "UserFlag1", "CHILD1", &id, nil)
	entries, err := s.ListAuditEntries("CHILD1")
	if err != nil || len(entries) != 1 || entries[0].RowID != unselected || !entries[0].Restore {
		t.Fatalf("explicit restore altered unselected marked history: %+v %v", entries, err)
	}
}
func TestPlotRestore_InvalidOwnershipSchemaAndValues(t *testing.T) {
	cases := []string{
		"foreign-plot", "foreign-project", "duplicate-selection", "duplicate-audit", "missing-audit",
		"missing-child", "foreign-child", "duplicate-child", "missing-child-id", "large-child-id",
		"parent-child-id", "identity-field", "unknown-table", "unknown-field", "injection",
		"wrong-project-table", "missing-column", "wrong-table-column", "missing-parent",
		"bad-before-int", "bad-after-int", "fractional-int", "nonfinite", "bad-bool",
		"invalid-stored-bool", "bad-time", "unchanged-audit", "missing-audit-column",
	}
	for _, failure := range cases {
		t.Run(failure, func(t *testing.T) {
			s, db := restoreFixture(t)
			id := int64(7)
			if _, err := db.Exec(`INSERT INTO Sample_Veg (PlotNumber,ID,Species,Cover1,Height1)
				VALUES ('CHILD1',7,'CURRENT',0,2);
				UPDATE Sample_Env SET Elevation = 2, SlopeGradient = 2, SpeciesListComplete = 0 WHERE PlotNumber = 'CHILD1'`); err != nil {
				t.Fatal(err)
			}
			row := restoreAuditFixture(t, db, "CHILD1", "_Veg", "Height1", &id, 1.25, 2.0)
			query := ""
			switch failure {
			case "foreign-plot":
				query = `UPDATE Sample_Audit SET PlotNumber = 'CHILD2'`
			case "foreign-project":
				query = `UPDATE Sample_Audit SET Project = 'OtherProject'`
			case "missing-child":
				query = `DELETE FROM Sample_Veg WHERE PlotNumber = 'CHILD1'`
			case "foreign-child":
				query = `UPDATE Sample_Veg SET PlotNumber = 'CHILD2' WHERE ID = 7`
			case "duplicate-child":
				query = `INSERT INTO Sample_Veg (PlotNumber,ID,Species,Cover1,Height1) VALUES ('CHILD1',7,'DUP',0,2)`
			case "missing-child-id":
				query = `UPDATE Sample_Audit SET ID = NULL`
			case "large-child-id":
				query = fmt.Sprintf(`UPDATE Sample_Audit SET ID = %d`, int64(math.MaxInt32)+1)
			case "parent-child-id":
				query = `UPDATE Sample_Audit SET "Table" = '_Env', EditField = 'Elevation'`
			case "identity-field":
				query = `UPDATE Sample_Audit SET EditField = 'ID'`
			case "unknown-table":
				query = `UPDATE Sample_Audit SET "Table" = '_Metadata'`
			case "unknown-field":
				query = `UPDATE Sample_Audit SET EditField = 'Unmapped'`
			case "injection":
				query = `UPDATE Sample_Audit SET "Table" = '_Veg"; DELETE FROM Sample_Env; --'`
			case "wrong-project-table":
				query = `UPDATE Sample_Audit SET "Table" = 'Other_Veg'`
			case "missing-column":
				query = `ALTER TABLE Sample_Veg RENAME COLUMN Height1 TO UnsupportedHeight`
			case "wrong-table-column":
				query = `UPDATE Sample_Audit SET EditField = 'HumusFormpH'`
			case "missing-parent":
				query = `DELETE FROM Sample_Admin WHERE Plot = 'CHILD1'`
			case "bad-before-int", "bad-after-int", "fractional-int":
				query = `UPDATE Sample_Audit SET "Table" = '_Env', EditField = 'Elevation', ID = NULL`
				column, value := "BeforeEdit", "not-int"
				if failure == "bad-after-int" {
					column = "AfterEdit"
				}
				if failure == "fractional-int" {
					value = "1.25"
				}
				query += `; UPDATE Sample_Audit SET ` + column + ` = '` + value + `'`
			case "nonfinite":
				query = `UPDATE Sample_Audit SET BeforeEdit = 'NaN'`
			case "bad-bool", "invalid-stored-bool":
				query = `UPDATE Sample_Audit SET "Table" = '_Env', EditField = 'SpeciesListComplete', ID = NULL, BeforeEdit = '-1', AfterEdit = '0'`
				if failure == "bad-bool" {
					query += `; UPDATE Sample_Audit SET BeforeEdit = '2'`
				} else {
					query += `; UPDATE Sample_Env SET SpeciesListComplete = 'invalid' WHERE PlotNumber = 'CHILD1'`
				}
			case "bad-time":
				query = `UPDATE Sample_Audit SET EditWhen = 'yesterday'`
			case "unchanged-audit":
				query = `UPDATE Sample_Audit SET BeforeEdit = AfterEdit`
			case "missing-audit-column":
				query = `ALTER TABLE Sample_Audit RENAME COLUMN BeforeEdit TO UnsupportedBefore`
			}
			if query != "" {
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			selected := []string{row}
			if failure == "duplicate-selection" {
				selected = append(selected, row)
			}
			if failure == "duplicate-audit" {
				selected = append(selected, restoreAuditFixture(t, db, "CHILD1", "_Veg", "Height1", &id, 1.25, 2.0))
			}
			if failure == "missing-audit" {
				selected = []string{"9223372036854775807"}
			}
			before := auditCount(t, db, "CHILD1") + auditCount(t, db, "CHILD2")
			if _, err := s.RestoreSelectedAuditRecords("CHILD1", selected, AuditRestorePrune); err == nil {
				t.Fatal("invalid restore accepted")
			}
			if failure == "foreign-project" {
				if _, err := s.ListAuditEntries("CHILD1"); err == nil {
					t.Fatal("listing silently hid foreign project audit ownership")
				}
				if _, err := db.Exec(`UPDATE Sample_Audit SET Restore = -1`); err != nil {
					t.Fatal(err)
				}
				if err := s.RestoreAuditRecords("CHILD1", true); err == nil {
					t.Fatal("marked restore ignored foreign project audit ownership")
				}
			}
			if auditCount(t, db, "CHILD1")+auditCount(t, db, "CHILD2") != before {
				t.Fatal("invalid restore pruned history")
			}
			assertRestoreField(t, db, "Env", "Elevation", "CHILD1", nil, 2)
		})
	}
}
