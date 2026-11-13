package sivi_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/boostao/vpro-wails/internal/sivi"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestCreateSIVIPlotAndUndoCreateSIVIPlot(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// 1. Validation guards: PlotNumber <= 7 UTF-16 units & PlotType in Ground/Visual/Note/Other.
	for _, bad := range []sivi.CreateSIVIPlotRequest{
		{PlotNumber: "", PlotType: "Ground"},
		{PlotNumber: "12345678", PlotType: "Ground"},
		{PlotNumber: "\U0001D538\U0001D538\U0001D538\U0001D538", PlotType: "Ground"}, // 4 astral chars = 8 UTF-16 code units (> 7!)
		{PlotNumber: "SV001", PlotType: "FS882"},     // Invalid SIVI PlotType
	} {
		if _, err := sivi.CreateSIVIPlot(svc, ctx, state.ContextID, bad); err == nil {
			t.Fatalf("expected invalid SIVI plot request %+v to fail", bad)
		}
	}

	// 2. Create valid Ground SIVI plot with leading-zero literal PlotNumber '00771'.
	created, err := sivi.CreateSIVIPlot(svc, ctx, state.ContextID, sivi.CreateSIVIPlotRequest{
		RequestID:       "22222222-3333-4444-8555-666666666661",
		PlotNumber:      "00771",
		PlotType:        "Ground",
		ProjectID:       "SIVI26",
		FieldNumber:     "POLY-14-G1",
		Date:            "2026-07-19",
		SiteSurveyor:    "W. MacKenzie",
		SVPolygonNumber: "POLY-14",
		Zone:            "SBS",
		SubZone:         "mk1",
		SiteSeries:      "01",
	})
	if err != nil {
		t.Fatalf("CreateSIVIPlot failed: %v", err)
	}
	if created.PlotNumber != "00771" || created.PlotType != "Ground" || created.AuditRowCount == 0 {
		t.Fatalf("unexpected CreateSIVIPlot result: %+v", created)
	}

	// Duplicate plot number with new UUID must be rejected!
	if _, err := sivi.CreateSIVIPlot(svc, ctx, state.ContextID, sivi.CreateSIVIPlotRequest{
		RequestID:  "22222222-3333-4444-8555-666666666662",
		PlotNumber: "00771",
		PlotType:   "Visual",
	}); err == nil {
		t.Fatalf("expected duplicate PlotNumber '00771' to fail")
	}

	// Verify ListCreatedSIVIPlots shows CanUndo=true.
	list1, err := sivi.ListCreatedSIVIPlots(svc, ctx, state.ContextID)
	if err != nil || len(list1) != 1 || !list1[0].CanUndo || list1[0].Undone {
		t.Fatalf("unexpected ListCreatedSIVIPlots before child add: %+v (err=%v)", list1, err)
	}

	// 3. Adding a child Veg record must block UndoCreateSIVIPlot until removed!
	cover := 15.0
	if err := svc.SaveVegRecord(state.ContextID, vprocore.VegRecord{
		PlotNumber: "00771",
		Species:    "PINUCON",
		Cover2:     &cover,
	}); err != nil {
		t.Fatalf("SaveVegRecord on SIVI plot failed: %v", err)
	}
	if _, err := sivi.UndoCreateSIVIPlot(svc, ctx, state.ContextID, sivi.UndoCreateSIVIPlotRequest{
		CreationRequestID: created.RequestID,
		PlotNumber:        "00771",
	}); err == nil {
		t.Fatalf("expected UndoCreateSIVIPlot to refuse deleting a plot that has child Veg records")
	}

	// Delete the child Veg record, then UndoCreateSIVIPlot must succeed cleanly!
	vegRows, err := svc.ListVegRecords(ctx, state.ContextID, "00771")
	if err != nil || len(vegRows) != 1 {
		t.Fatal(err)
	}
	if err := svc.DeleteVegRecord(state.ContextID, "00771", vegRows[0].ID); err != nil {
		t.Fatal(err)
	}

	undone, err := sivi.UndoCreateSIVIPlot(svc, ctx, state.ContextID, sivi.UndoCreateSIVIPlotRequest{
		RequestID:         "22222222-3333-4444-8555-666666666663",
		CreationRequestID: created.RequestID,
		PlotNumber:        "00771",
	})
	if err != nil {
		t.Fatalf("UndoCreateSIVIPlot failed: %v", err)
	}
	if undone.PlotNumber != "00771" || undone.Replayed {
		t.Fatalf("unexpected UndoCreateSIVIPlot result: %+v", undone)
	}

	// Verify _Env and _Admin rows are removed, __VPRO_SIVICreationPlotHistory.Undone == -1, and <P>_Audit has UndoCreateSIVIPlot receipt!
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var envCnt, adminCnt int
	var histUndone int64
	var auditReceiptCnt int
	if err := db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM "Sample_Env" WHERE "PlotNumber" = '00771'),
		(SELECT COUNT(*) FROM "Sample_Admin" WHERE "Plot" = '00771'),
		(SELECT "Undone" FROM "__VPRO_SIVICreationPlotHistory" WHERE "RequestID" = ?),
		(SELECT COUNT(*) FROM "Sample_Audit" WHERE "EditField" = 'UndoCreateSIVIPlot' AND "BeforeEdit" = '00771')`,
		created.RequestID).Scan(&envCnt, &adminCnt, &histUndone, &auditReceiptCnt); err != nil {
		t.Fatal(err)
	}
	if envCnt != 0 || adminCnt != 0 || histUndone != -1 || auditReceiptCnt != 1 {
		t.Fatalf("unexpected post-undo DB state: env=%d admin=%d histUndone=%d auditReceipt=%d",
			envCnt, adminCnt, histUndone, auditReceiptCnt)
	}
}

func TestUndoCreateSIVIPlotToleratesForwardFieldsInStoredHistory(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Create Sample_SU and switch active SU to "Sample" so AddToActiveSU writes into Sample_SU.
	rw, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`CREATE TABLE IF NOT EXISTS "Sample_SU"("PlotNumber" VARCHAR, "SiteUnit" VARCHAR)`); err != nil {
		rw.Close()
		t.Fatal(err)
	}
	rw.Close()

	suState, err := svc.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project:       state.ActiveProject,
		ProjectPath:   state.ProjectPath,
		SU:            "Sample",
		SUPath:        state.ProjectPath,
		Hierarchy:     state.ActiveHierarchy,
		HierarchyPath: state.HierarchyPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := sivi.CreateSIVIPlot(svc, ctx, suState.ContextID, sivi.CreateSIVIPlotRequest{
		RequestID:     "33333333-3333-4444-8555-666666666661",
		PlotNumber:    "00882",
		PlotType:      "Ground",
		UserSiteUnit:  "CWHvm1/01",
		AddToActiveSU: true,
	})
	if err != nil || !created.AddedToSU {
		t.Fatalf("CreateSIVIPlot with AddToActiveSU failed: %+v, err=%v", created, err)
	}

	// Simulate a forward/historical unknown field inside stored __VPRO_SIVICreationPlotHistory.Proposal.request!
	rw2, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw2.Exec(`UPDATE "__VPRO_SIVICreationPlotHistory"
		SET "Proposal" = replace("Proposal", '"plotNumber":"00882"', '"plotNumber":"00882","legacyForwardProp":true')
		WHERE "RequestID" = ?`, created.RequestID); err != nil {
		rw2.Close()
		t.Fatal(err)
	}
	rw2.Close()

	entries, err := sivi.ListCreatedSIVIPlots(svc, ctx, suState.ContextID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListCreatedSIVIPlots failed on forward history row: %v", err)
	}

	undone, err := sivi.UndoCreateSIVIPlot(svc, ctx, suState.ContextID, sivi.UndoCreateSIVIPlotRequest{
		RequestID:         "33333333-3333-4444-8555-666666666662",
		CreationRequestID: created.RequestID,
		PlotNumber:        "00882",
	})
	if err != nil {
		t.Fatalf("UndoCreateSIVIPlot failed on forward history row: %v", err)
	}
	if !undone.RemovedFromSU {
		t.Fatalf("UndoCreateSIVIPlot lost AddedToSU / failed to remove SU row due to strict request decoder on history row: %+v", undone)
	}
}

