package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestGeologyPhysicalRestoreBoundariesAndRoutes(t *testing.T) {
	for _, field := range geologyHeaderFields(FS882Header{}) {
		for i, invalid := range []string{"", "12345", "abc\U0001F600", string([]byte{0xff})} {
			for _, route := range []string{"selection", "retain", "prune", "marked-retain", "marked-prune"} {
				t.Run(field.name+"/invalid/"+strconv.Itoa(i)+"/"+route, func(t *testing.T) {
					s, db, plot, id := siteCodeRestoreFixture(t, field.name, &invalid, qualityString("AB"))
					if strings.HasPrefix(route, "marked") {
						if _, err := db.Exec(`UPDATE Sample_Audit SET Restore=-1 WHERE rowid=?`, id); err != nil {
							t.Fatal(err)
						}
					}
					before := substrateSnapshot(t, db)
					var err error
					switch route {
					case "selection":
						err = s.SetAuditRestoreSelection(plot, []string{id})
					case "retain", "prune":
						action := AuditRestoreRetain
						if route == "prune" {
							action = AuditRestorePrune
						}
						_, err = s.RestoreSelectedAuditRecords(plot, []string{id}, action)
					default:
						err = s.RestoreAuditRecords(plot, route == "marked-prune")
					}
					if err == nil || !strings.Contains(err.Error(), field.name) {
						t.Fatal("invalid restore accepted", err)
					}
					assertSubstrateSnapshot(t, db, before)
				})
			}
		}
		for i, target := range []*string{nil, qualityString("q'X4"), qualityString("aB"), qualityString(" "),
			qualityString("\nX"), qualityString("e\u0301"), qualityString("a\U0001F600b")} {
			t.Run(field.name+"/valid/"+strconv.Itoa(i), func(t *testing.T) {
				s, db, plot, id := siteCodeRestoreFixture(t, field.name, target, qualityString("historical-too-long"))
				count := auditCount(t, db, plot)
				if err := s.SetAuditRestoreSelection(plot, []string{id}); err != nil {
					t.Fatal(err)
				}
				action, pruned := AuditRestoreRetain, 0
				if i%2 == 0 {
					action, pruned = AuditRestorePrune, 1
				}
				result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, action)
				if err != nil || result.RestoredRows != 1 || result.PrunedAuditRows != pruned {
					t.Fatal("valid restore failed", result, err)
				}
				got, err := s.GetPlot(plot)
				if err != nil || !reflect.DeepEqual(reflect.ValueOf(*got).FieldByName(field.name).Interface(), target) ||
					auditCount(t, db, plot) != count-pruned {
					t.Fatal("restore repaired raw text/changed history")
				}
			})
		}
		if err := validateGeologyRestoreValue("Env", field.name, 7); err == nil {
			t.Fatal("non-string restore accepted")
		}
	}
}

func TestGeologyPhysicalRestoreOwnershipCancelAndSchema(t *testing.T) {
	for _, mutate := range []string{
		`UPDATE Sample_Audit SET PlotNumber='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET Project='foreign' WHERE rowid=?`,
		`UPDATE Sample_Audit SET ID=0 WHERE rowid=?`,
		`UPDATE Sample_Audit SET AfterEdit='stale' WHERE rowid=?`,
		`ALTER TABLE Sample_Env DROP COLUMN BedrockGeology1`,
	} {
		t.Run(mutate, func(t *testing.T) {
			s, db, plot, id := siteCodeRestoreFixture(t, "BedrockGeology1", qualityString("ZZZZ"), qualityString("AB"))
			if err := s.CreatePlot(FS882Header{PlotNumber: "foreign"}); err != nil {
				t.Fatal(err)
			}
			args := []any{id}
			if strings.HasPrefix(mutate, "ALTER") {
				args = nil
			}
			if _, err := db.Exec(mutate, args...); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestorePrune); err == nil {
				t.Fatal("foreign/child/stale/unsupported restore accepted")
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
	s, db, plot, id := siteCodeRestoreFixture(t, "BedrockGeology1", qualityString("historical-too-long"), qualityString("AB"))
	before := substrateSnapshot(t, db)
	result, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreCancel)
	if err != nil || !result.Cancelled {
		t.Fatal("cancel changed/validated history")
	}
	result, err = s.RestoreSelectedAuditRecords(plot, nil, AuditRestorePrune)
	if err != nil || *result != (AuditRestoreResult{}) {
		t.Fatal("empty selection changed history")
	}
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id, id}, AuditRestorePrune); err == nil {
		t.Fatal("duplicate selection accepted")
	}
	if _, err := s.RestoreSelectedAuditRecords(plot, []string{id}, AuditRestoreRetain); err == nil {
		t.Fatal("historical invalid target grandfathered")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestGeologyPhysicalThreeRowRestoreAtomicAndDeterministic(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER geo_restore_reject BEFORE UPDATE OF BedrockGeology1 ON Sample_Env BEGIN SELECT RAISE(ABORT,'later restore rejected'); END`,
		`CREATE TRIGGER geo_restore_reject BEFORE DELETE ON Sample_Audit BEGIN SELECT RAISE(ABORT,'prune rejected'); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			s, db, plot, first := siteCodeRestoreFixture(t, "BedrockGeology1", qualityString("aB"), qualityString("AB"))
			ids := []string{first}
			for i, member := range []string{"BedrockGeology2", "BedrockGeology3"} {
				if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(member)+`='CD' WHERE PlotNumber=?`, plot); err != nil {
					t.Fatal(err)
				}
				inserted, err := db.Exec(`INSERT INTO Sample_Audit
				 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
				 VALUES ('Sample','RestoreTester',?,'_Env',?,?,'q''X4','CD',0,0)`,
					plot, member, "2026-09-30 12:00:0"+strconv.Itoa(i+1))
				if err != nil {
					t.Fatal(err)
				}
				row, err := inserted.LastInsertId()
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, strconv.FormatInt(row, 10))
			}
			if err := s.SetAuditRestoreSelection(plot, ids); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE TABLE GeoRestoreTrace(Field TEXT); CREATE TRIGGER geo_restore_trace AFTER UPDATE OF BedrockGeology1,BedrockGeology2,BedrockGeology3 ON Sample_Env BEGIN
			 INSERT INTO GeoRestoreTrace VALUES(CASE WHEN OLD.BedrockGeology1 IS NOT NEW.BedrockGeology1 THEN 'BedrockGeology1' WHEN OLD.BedrockGeology2 IS NOT NEW.BedrockGeology2 THEN 'BedrockGeology2' ELSE 'BedrockGeology3' END); END;` + trigger); err != nil {
				t.Fatal(err)
			}
			before := substrateSnapshot(t, db)
			if _, err := s.RestoreSelectedAuditRecords(plot, ids, AuditRestorePrune); err == nil {
				t.Fatal("failed three-row transaction accepted")
			}
			assertSubstrateSnapshot(t, db, before)
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM GeoRestoreTrace`).Scan(&count); err != nil || count != 0 {
				t.Fatal("earlier restore escaped rollback")
			}
			if _, err := db.Exec(`DROP TRIGGER geo_restore_reject`); err != nil {
				t.Fatal(err)
			}
			result, err := s.RestoreSelectedAuditRecords(plot, []string{ids[1], ids[0], ids[2]}, AuditRestorePrune)
			if err != nil || result.RestoredRows != 3 || result.PrunedAuditRows != 3 {
				t.Fatal("restore retry failed", result, err)
			}
			var order string
			if err := db.QueryRow(`SELECT group_concat(Field,',') FROM (SELECT Field FROM GeoRestoreTrace ORDER BY rowid)`).Scan(&order); err != nil ||
				order != "BedrockGeology3,BedrockGeology2,BedrockGeology1" {
				t.Fatal("reverse edit order changed", order, err)
			}
			got, err := s.GetPlot(plot)
			if err != nil || !sameSiteCode(got.BedrockGeology1, qualityString("aB")) ||
				!sameSiteCode(got.BedrockGeology2, qualityString("q'X4")) || !sameSiteCode(got.BedrockGeology3, qualityString("q'X4")) {
				t.Fatal("independent raw restore targets changed", err)
			}
		})
	}
}
