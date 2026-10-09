package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNullableParentFlagsPreserveNullFalseAndAccessTrue(t *testing.T) {
	for _, field := range []struct{ property, member, table, key string }{
		{"speciesListComplete", "SpeciesListComplete", "Env", "PlotNumber"},
		{"updatedFromCards", "UpdatedFromCards", "Admin", "Plot"},
	} {
		t.Run(field.member, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "FLAG"}
			if err := s.SetAuditStrength(0); err != nil {
				t.Fatal(err)
			}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			if err := s.SetAuditStrength(3); err != nil {
				t.Fatal(err)
			}
			for _, value := range []any{true, false, nil} {
				if value == nil {
					setOrdinaryTestValue(&h, field.member, nil)
				} else {
					flag := value.(bool)
					setOrdinaryTestValue(&h, field.member, &flag)
				}
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				var stored *int
				if err := db.QueryRow(`SELECT CAST(` + field.member + ` AS INTEGER) FROM Sample_` + field.table + ` WHERE ` + field.key + `='FLAG'`).Scan(&stored); err != nil {
					t.Fatal(err)
				}
				if value == nil && stored != nil || value != nil && (stored == nil || value == true && *stored != -1 || value == false && *stored != 0) {
					t.Fatal("nullable Boolean physical representation changed", value, stored)
				}
			}
			if auditCount(t, db, "FLAG") != 3 {
				t.Fatal("nullable flag changes produced phantom or omitted audits")
			}
			for _, raw := range []string{"-1", "0", "1", `"true"`, `"false"`} {
				if err := json.Unmarshal([]byte(`{"`+field.property+`":`+raw+`}`), &h); err == nil {
					t.Fatal("raw nonboolean JSON was accepted", raw)
				}
			}
			if _, err := db.Exec(`UPDATE Sample_` + field.table + ` SET ` + field.member + `=1 WHERE ` + field.key + `='FLAG'; CREATE TRIGGER flag_unchanged BEFORE UPDATE OF ` + field.member + ` ON Sample_` + field.table + ` BEGIN SELECT RAISE(ABORT,'historical flag assigned'); END`); err != nil {
				t.Fatal(err)
			}
			stored, err := s.GetPlot("FLAG")
			if err != nil {
				t.Fatal(err)
			}
			stored.OfficeNotes = qualityString("unrelated edit")
			if err := s.UpdatePlot(*stored); err != nil {
				t.Fatal("historical true reassigned", err)
			}
			if _, err := db.Exec(`DROP TRIGGER flag_unchanged; CREATE TRIGGER flag_audit_reject BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='` + field.member + `' BEGIN SELECT RAISE(ABORT,'flag audit rollback'); END`); err != nil {
				t.Fatal(err)
			}
			flag := false
			setOrdinaryTestValue(stored, field.member, &flag)
			before := substrateSnapshot(t, db)
			if err := s.UpdatePlot(*stored); err == nil || !strings.Contains(err.Error(), "flag audit rollback") {
				t.Fatal("flag audit rejection did not fail explicitly", err)
			}
			assertSubstrateSnapshot(t, db, before)
		})
	}
}
