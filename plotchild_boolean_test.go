package main

import "testing"

func TestPlotChild_AccessBooleanRoundtrip(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if err := s.SaveOtherRecord(OtherRecord{PlotNumber: "CHILD1"}); err != nil {
		t.Fatal(err)
	}
	id := childID(t, db, "Other", "CHILD1")
	if _, err := db.Exec(`UPDATE Sample_Other SET UserFlag1=-1,UserFlag2=0,UserFlag3=NULL WHERE PlotNumber='CHILD1' AND ID=?`, id); err != nil {
		t.Fatal(err)
	}
	clearChildAudit(t, db)
	rows, err := s.ListOtherRecords("CHILD1")
	if err != nil || len(rows) != 1 {
		t.Fatalf("list Access flags: %v %v", rows, err)
	}
	row := rows[0]
	if row.UserFlag1 == nil || !*row.UserFlag1 || row.UserFlag2 == nil || *row.UserFlag2 || row.UserFlag3 != nil {
		t.Fatalf("Access flags lost: %+v", row)
	}
	if err := s.UpdateOtherRecord(row); err != nil {
		t.Fatal(err)
	}
	var stored int
	if err := db.QueryRow(`SELECT CAST(UserFlag1 AS INTEGER) FROM Sample_Other WHERE ID=? AND PlotNumber='CHILD1'`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != -1 {
		t.Fatalf("Access true rewritten as %d", stored)
	}
	checkChildAudits(t, s, "Other", id, nil, nil)
	flag := false
	row.UserFlag1 = &flag
	if err := s.UpdateOtherRecord(row); err != nil {
		t.Fatal(err)
	}
	checkChildAudits(t, s, "Other", id, map[string]any{"UserFlag1": true}, map[string]any{"UserFlag1": false})
}
