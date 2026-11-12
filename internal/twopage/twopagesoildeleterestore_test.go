package twopage_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestTwoPageSoilDeleteRestoreAndPermanentIDNonReuse(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed a test plot with Humus and Mineral horizons.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO Sample_Env (PlotNumber, ProjectID, Zone) VALUES ('SOIL99', 'SMP', 'CWH');
		INSERT INTO Sample_Admin (Plot, PlotType) VALUES ('SOIL99', 'FS882');
		INSERT INTO Sample_Humus (PlotNumber, Horizon, UpperDepth, LowerDepth, HumusFormpH, Flag, ID)
			VALUES ('SOIL99', 'Lv', 8.0, 5.0, 4.2, 0, 9001),
			       ('SOIL99', 'Fm', 5.0, 0.0, 3.9, 0, 9002);
		INSERT INTO Sample_Mineral (PlotNumber, Horizon, UpperDepth, LowerDepth, Texture, Colour, Flag, ID)
			VALUES ('SOIL99', 'Bf', 0.0, 35.0, 'SL', '7.5YR 4/6', 0, 9101);
	`)

	soilSvc, err := twopage.NewTwoPageSoilService(svc, func(string) (string, bool) {
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Delete Humus horizon ID=9002.
	delHumus, err := twopage.DeleteTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.DeleteTwoPageSoilHorizonRequest{
		RequestID:  "11111111-2222-4333-8444-555555555551",
		PlotNumber: "SOIL99",
		Form:       "SoilHumusXL",
		ID:         9002,
	})
	if err != nil {
		t.Fatalf("DeleteTwoPageSoilHorizon Humus failed: %v", err)
	}
	if delHumus.DeletedID != 9002 || delHumus.Horizon != "Fm" || !delHumus.ReservedInLedger || delHumus.AuditRowCount == 0 {
		t.Fatalf("unexpected DeleteTwoPageSoilHorizon result: %+v", delHumus)
	}

	// Idempotent replay with same UUID returns Replayed=true without failing.
	replayDel, err := twopage.DeleteTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.DeleteTwoPageSoilHorizonRequest{
		RequestID:  "11111111-2222-4333-8444-555555555551",
		PlotNumber: "SOIL99",
		Form:       "SoilHumusXL",
		ID:         9002,
	})
	if err != nil || !replayDel.Replayed {
		t.Fatalf("expected idempotent delete replay, got %+v (err=%v)", replayDel, err)
	}

	// 2. Verify permanent deleted child ID non-reuse when creating a brand new Humus horizon via TwoPageSoilService.CreateHorizon!
	// Also proves TwoPageSoilService.CreateHorizon / readTwoPageSoilReceipt coexists cleanly with delete entries in __VPRO_TwoPageSoilHistory!
	origAfterDelete, err := soilSvc.GetOriginal(ctx, state.ContextID, "SOIL99", "SoilHumusXL")
	if err != nil {
		t.Fatalf("GetOriginal after delete failed: %v", err)
	}
	if len(origAfterDelete.Records.Rows) != 1 {
		t.Fatalf("expected 1 remaining Humus row after delete, got %d", len(origAfterDelete.Records.Rows))
	}

	createReqJSON, err := json.Marshal(map[string]any{
		"requestId": "11111111-2222-4333-8444-555555555552",
		"original":  origAfterDelete,
		"edits":     []any{},
		"horizon":   "Hh",
	})
	if err != nil {
		t.Fatal(err)
	}
	createdRes, err := soilSvc.CreateHorizon(ctx, state.ContextID, string(createReqJSON))
	if err != nil {
		t.Fatalf("CreateHorizon after delete failed: %v", err)
	}
	if !createdRes.DidCommit {
		t.Fatalf("expected CreateHorizon to commit")
	}

	humusList, err := svc.ListHumusRecords(ctx, state.ContextID, "SOIL99")
	if err != nil {
		t.Fatal(err)
	}
	for _, hr := range humusList {
		if hr.ID == 9002 {
			t.Fatalf("CRITICAL INVARIANT VIOLATION: newly created Humus horizon reused deleted ID 9002!")
		}
	}

	// 3. ListTwoPageSoilDeletedHorizons must list 9002 with Restored=false.
	deletedEntries, err := twopage.ListTwoPageSoilDeletedHorizons(svc, ctx, state.ContextID, "SOIL99", "SoilHumusXL")
	if err != nil {
		t.Fatalf("ListTwoPageSoilDeletedHorizons failed: %v", err)
	}
	if len(deletedEntries) != 1 || deletedEntries[0].DeletedID != 9002 || deletedEntries[0].Restored {
		t.Fatalf("unexpected deleted horizons list: %+v", deletedEntries)
	}

	// 4. RestoreTwoPageSoilHorizon restores ID=9002 with its exact original fields (Fm, 5.0..0.0, pH 3.9).
	restoredHumus, err := twopage.RestoreTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.RestoreTwoPageSoilHorizonRequest{
		RequestID:       "11111111-2222-4333-8444-555555555553",
		PlotNumber:      "SOIL99",
		Form:            "SoilHumusXL",
		DeleteRequestID: delHumus.RequestID,
	})
	if err != nil {
		t.Fatalf("RestoreTwoPageSoilHorizon failed: %v", err)
	}
	if restoredHumus.RestoredID != 9002 || restoredHumus.Horizon != "Fm" {
		t.Fatalf("unexpected RestoreTwoPageSoilHorizon result: %+v", restoredHumus)
	}

	// Attempting to restore the same deletion receipt a second time with a new UUID must be rejected!
	if _, err := twopage.RestoreTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.RestoreTwoPageSoilHorizonRequest{
		RequestID:       "11111111-2222-4333-8444-555555555554",
		PlotNumber:      "SOIL99",
		Form:            "SoilHumusXL",
		DeleteRequestID: delHumus.RequestID,
	}); err == nil {
		t.Fatalf("expected double-restore of same deletion receipt to fail")
	}

	// 5. Also verify Mineral horizon delete & restore and Audit Restore = -1 flag!
	delMin, err := soilSvc.DeleteHorizon(ctx, state.ContextID, twopage.DeleteTwoPageSoilHorizonRequest{
		RequestID:  "11111111-2222-4333-8444-555555555555",
		PlotNumber: "SOIL99",
		Form:       "SoilMineralXL",
		ID:         9101,
	})
	if err != nil {
		t.Fatalf("DeleteHorizon Mineral failed: %v", err)
	}
	resMin, err := soilSvc.RestoreHorizon(ctx, state.ContextID, twopage.RestoreTwoPageSoilHorizonRequest{
		RequestID:       "11111111-2222-4333-8444-555555555556",
		PlotNumber:      "SOIL99",
		Form:            "SoilMineralXL",
		DeleteRequestID: delMin.RequestID,
	})
	if err != nil || resMin.RestoredID != 9101 {
		t.Fatalf("RestoreHorizon Mineral failed: %+v (err=%v)", resMin, err)
	}

	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var markedRestored int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "Sample_Audit" WHERE "PlotNumber" = 'SOIL99' AND "Restore" = -1`).Scan(&markedRestored); err != nil || markedRestored == 0 {
		t.Fatalf("expected deletion audit rows to be marked Restore = -1 after restoration, got %d (err=%v)", markedRestored, err)
	}
}
