package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReviewedVegetationDeletionHiddenValuesRollbackAndReservation(t *testing.T) {
	s, db := childFixture(t)
	s.SetAuditStrength(3)
	if _, err := db.Exec(`ALTER TABLE Sample_Veg ADD COLUMN HiddenText TEXT;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,HiddenText,Flag)
		VALUES ('CHILD1','RAW',0,0,'hidden',1),('CHILD1','RAW',-9,0,'',0)`); err != nil {
		t.Fatal(err)
	}
	review := func(id int64) VegetationDeletionReview {
		t.Helper()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		snapshot, err := inspectVegetationDeletion(context.Background(), tx, `"Sample_Veg"`, "CHILD1", "SubVegAXL_BC", id)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.review.Species == nil || *snapshot.review.Species != "RAW" || len(snapshot.review.Expected) != 64 {
			t.Fatal("incomplete review:", snapshot.review)
		}
		return snapshot.review
	}
	initial := review(0)
	if _, err := db.Exec(`UPDATE Sample_Veg SET HiddenText='changed' WHERE ID=0`); err != nil {
		t.Fatal(err)
	}
	before := substrateSnapshot(t, db)
	request := VegetationDeletionRequest{0, initial.Form, initial.Expected}
	if err := s.DeleteReviewedVegetation("CHILD1", request); err == nil {
		t.Fatal("hidden field change escaped reviewed deletion")
	}
	assertSubstrateSnapshot(t, db, before)
	current := review(0)
	if current.Expected == initial.Expected {
		t.Fatal("hidden data was omitted from snapshot")
	}
	for _, bad := range []VegetationDeletionRequest{
		{0, "SubVegCXL", current.Expected}, {0, current.Form, strings.Repeat("0", 64)},
		{2147483648, current.Form, current.Expected}, {-9, current.Form, current.Expected},
		{0, current.Form, strings.ToUpper(current.Expected)},
	} {
		if err := s.DeleteReviewedVegetation("CHILD1", bad); err == nil {
			t.Fatal("altered source/identity/snapshot accepted:", bad)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	request.Expected = current.Expected
	if _, err := db.Exec(`CREATE TRIGGER fail_delete_extra BEFORE INSERT ON Sample_Audit
		WHEN NEW.EditField='HiddenText' BEGIN SELECT RAISE(ABORT,'injected hidden deletion audit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteReviewedVegetation("CHILD1", request); err == nil {
		t.Fatal("extra-field audit failure committed deletion")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec(`DROP TRIGGER fail_delete_extra`); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteReviewedVegetation("CHILD1", request); err != nil {
		t.Fatal(err)
	}
	if auditCount(t, db, "CHILD1") != 4 {
		t.Fatal("missing mapped/hidden/boolean deletion audits")
	}
	var flag string
	if err := db.QueryRow(`SELECT BeforeEdit FROM Sample_Audit WHERE EditField='Flag'`).Scan(&flag); err != nil || flag != "-1" {
		t.Fatal("Access BOOLEAN deletion audit was not normalized:", flag, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=0`).Scan(&count); err != nil || count != 1 {
		t.Fatal("deleted identity was not retained:", count, err)
	}
	before = substrateSnapshot(t, db)
	if err := s.DeleteReviewedVegetation("CHILD1", request); err == nil {
		t.Fatal("completed deletion replay was accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}

func TestVegetationDeletionFingerprintRawTextAndExplicitTransport(t *testing.T) {
	s, db := childFixture(t)
	if _, err := db.Exec(`ALTER TABLE Sample_Veg ADD COLUMN HiddenText TEXT;
		ALTER TABLE Sample_Veg ADD COLUMN Unsupported BLOB;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,HiddenText)
		VALUES ('CHILD1','RAW',-9,0,CAST(x'00ff' AS TEXT))`); err != nil {
		t.Fatal(err)
	}
	snapshot := func(ctx context.Context) (vegetationDeletionSnapshot, error) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		return inspectVegetationDeletion(ctx, tx, `"Sample_Veg"`, "CHILD1", "SubVegAXL_BC", -9)
	}
	first, err := snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET HiddenText=CAST(x'00fe' AS TEXT) WHERE ID=-9`); err != nil {
		t.Fatal(err)
	}
	second, err := snapshot(context.Background())
	if err != nil || first.review.Expected == second.review.Expected {
		t.Fatal("NUL/invalid Unicode hidden values collapsed:", err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Unsupported=x'01' WHERE ID=-9`); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot(context.Background()); err == nil {
		t.Fatal("unsupported BLOB deletion review succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled review was not propagated:", err)
	}
	for _, payload := range []string{
		`{"form":"SubVegAXL_BC","expected":"raw"}`,
		`{"id":null,"form":"SubVegAXL_BC","expected":"raw"}`,
		`{"id":0,"form":null,"expected":"raw"}`,
		`{"id":0,"form":"SubVegAXL_BC","expected":null}`,
		`{"id":0,"form":"\ud800","expected":"raw"}`,
	} {
		if err := json.Unmarshal([]byte(payload), &VegetationDeletionRequest{}); err == nil {
			t.Fatal("implicit/malformed deletion transport accepted:", payload)
		}
	}
	before := substrateSnapshot(t, db)
	if err := s.DeleteReviewedVegetation("CHILD1", VegetationDeletionRequest{-9, "SubVegAXL_BC", ""}); err == nil {
		t.Fatal("missing snapshot accepted")
	}
	assertSubstrateSnapshot(t, db, before)
}
