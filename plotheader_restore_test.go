package main

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestPlotHeader_RestoreAllMappedFields(t *testing.T) {
	for _, qualified := range []bool{false, true} {
		t.Run(map[bool]string{false: "suffix-table", true: "project-table"}[qualified], func(t *testing.T) {
			s, db := headerFixture(t)
			h := fullHeader("HDRUNDO")
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if err := s.SavePlot(FS882Header{PlotNumber: h.PlotNumber}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Audit SET Restore = 1 WHERE PlotNumber = ? AND BeforeEdit IS NOT NULL AND AfterEdit IS NULL`, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			if qualified {
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table" = 'Sample' || "Table" WHERE PlotNumber = ? AND Restore = 1`, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
			}
			before := auditCount(t, db, h.PlotNumber)
			if err := s.RestoreAuditRecords(h.PlotNumber, true); err != nil {
				t.Fatal(err)
			}
			got, err := s.GetPlot(h.PlotNumber)
			if err != nil || !reflect.DeepEqual(*got, h) {
				t.Fatalf("header restore roundtrip: %#v %v", got, err)
			}
			if remaining := auditCount(t, db, h.PlotNumber); remaining != before-(len(headerFields)-2) {
				t.Fatalf("unselected audits removed: before %d remaining %d", before, remaining)
			}
		})
	}
}

func TestPlotHeader_RestoreNullAndRetainAudit(t *testing.T) {
	s, db := headerFixture(t)
	h := fullHeader("HDRNULL")
	if err := s.SavePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Audit SET Restore = 1 WHERE PlotNumber = ?`, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreAuditRecords(h.PlotNumber, false); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, FS882Header{PlotNumber: h.PlotNumber}) {
		t.Fatalf("initial audit NULL restoration: %#v %v", got, err)
	}
	if count := auditCount(t, db, h.PlotNumber); count != len(headerFields)-2 {
		t.Fatalf("audit rows not retained: %d", count)
	}
}

func TestPlotHeader_RestoreRollbackAndAllowlist(t *testing.T) {
	for _, failure := range []string{"env-trigger", "identity-field", "unverified-field", "missing-admin"} {
		t.Run(failure, func(t *testing.T) {
			s, db := headerFixture(t)
			h := fullHeader("HDRREST")
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if err := s.SavePlot(FS882Header{PlotNumber: h.PlotNumber}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE Sample_Audit SET Restore = 1 WHERE PlotNumber = ? AND BeforeEdit IS NOT NULL AND AfterEdit IS NULL`, h.PlotNumber); err != nil {
				t.Fatal(err)
			}
			query := ""
			switch failure {
			case "env-trigger":
				query = `CREATE TRIGGER fail_restore BEFORE UPDATE ON Sample_Env BEGIN SELECT RAISE(ABORT, 'restore Env failure'); END`
			case "identity-field":
				query = `UPDATE Sample_Audit SET EditField = 'PlotNumber' WHERE PlotNumber = 'HDRREST' AND Restore = 1 AND EditField = 'SiteNotes'`
			case "unverified-field":
				query = `UPDATE Sample_Audit SET EditField = 'Location"; SELECT 1; --' WHERE PlotNumber = 'HDRREST' AND Restore = 1 AND EditField = 'SiteNotes'`
			case "missing-admin":
				query = `DELETE FROM Sample_Admin WHERE Plot = 'HDRREST'`
			}
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			before := auditCount(t, db, h.PlotNumber)
			if err := s.RestoreAuditRecords(h.PlotNumber, true); err == nil {
				t.Fatal("expected restore failure")
			}
			if count := auditCount(t, db, h.PlotNumber); count != before {
				t.Fatal("failed restore deleted audits")
			}
			var location, office sql.NullString
			if err := db.QueryRow(`SELECT Location FROM Sample_Env WHERE PlotNumber = ?`, h.PlotNumber).Scan(&location); err != nil {
				t.Fatal(err)
			}
			if location.Valid {
				t.Fatal("failed restore left Env changed")
			}
			if failure != "missing-admin" {
				if err := db.QueryRow(`SELECT OfficeNotes FROM Sample_Admin WHERE Plot = ?`, h.PlotNumber).Scan(&office); err != nil {
					t.Fatal(err)
				}
				if office.Valid {
					t.Fatal("failed restore left Admin changed")
				}
			}
		})
	}
}
