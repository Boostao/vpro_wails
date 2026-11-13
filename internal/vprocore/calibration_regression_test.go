package vprocore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func f64Ptr(v float64) *float64 { return &v }
func strPtr(v string) *string   { return &v }
func intPtr(v int) *int         { return &v }

func TestVProCoreCalibrationGuards(t *testing.T) {
	t.Run("CreatePlotPlotNumberBoundsAndHistoricalUpdateTolerance", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		// Reject empty, invalid UTF-8, NUL, >7 ASCII, and 8 UTF-16 code units ("123456🌲").
		for _, badPlot := range []string{
			"",
			"   ",
			"BAD\xff",
			"P01\x00",
			"12345678",
			"123456\U0001F332", // 6 BMP + 1 supplementary = 8 UTF-16 code units
		} {
			if err := svc.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: badPlot}); err == nil {
				t.Fatalf("expected CreatePlot(%q) to be rejected", badPlot)
			}
		}

		// Reject unpaired surrogate in raw JSON for plotNumber before Go JSON repair.
		var rawHdr vprocore.FS882Header
		if err := json.Unmarshal([]byte(`{"plotNumber":"\ud800"}`), &rawHdr); err == nil {
			t.Fatalf("expected unpaired surrogate in plotNumber JSON to be rejected")
		}

		// Accept exact 7 UTF-16 code units ("1234567" and "12345🌲" = 5 + 2 = 7 UTF-16 units).
		for _, okPlot := range []string{"1234567", "12345\U0001F332"} {
			if err := svc.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: okPlot}); err != nil {
				t.Fatalf("expected CreatePlot(%q) (7 UTF-16 units) to succeed, got: %v", okPlot, err)
			}
		}

		// Historical >7 PlotNumber read + update tolerance on existing rows.
		vprotest.MutateContextFixture(t, state.ProjectPath, `
			INSERT INTO "Sample_Env" ("PlotNumber", "Elevation") VALUES ('HISTLONG99', 400);
			INSERT INTO "Sample_Admin" ("Plot") VALUES ('HISTLONG99');
		`)
		histHdr, err := svc.GetPlot(ctx, state.ContextID, "HISTLONG99")
		if err != nil {
			t.Fatalf("GetPlot on historical long PlotNumber failed: %v", err)
		}
		histHdr.Elevation = intPtr(450)
		if err := svc.UpdatePlot(state.ContextID, *histHdr); err != nil {
			t.Fatalf("UpdatePlot on historical long PlotNumber failed: %v", err)
		}
	})

	t.Run("SingleCoverFloat32NarrowingVegAndStrataCover", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		if err := svc.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: "COV001"}); err != nil {
			t.Fatal(err)
		}

		// Veg cover: 99.999996 narrows to float32 99.99999237 (< 100) -> PASS.
		if err := svc.SaveVegRecord(state.ContextID, vprocore.VegRecord{
			PlotNumber: "COV001",
			Species:    "PSEUMEN",
			Cover1:     f64Ptr(99.999996),
			Cover5a:    f64Ptr(0.0),
			TotalA:     f64Ptr(99.999996),
		}); err != nil {
			t.Fatalf("expected 99.999996 Veg cover to pass float32 narrowing, got: %v", err)
		}

		// Veg cover: 99.999999 narrows to float32 100 -> REJECT; -0.0001 -> REJECT.
		for _, badCov := range []float64{99.999999, 100.0, -0.0001} {
			if err := svc.SaveVegRecord(state.ContextID, vprocore.VegRecord{
				PlotNumber: "COV001",
				Species:    "TSUGHET",
				Cover1:     f64Ptr(badCov),
			}); err == nil {
				t.Fatalf("expected Veg Cover1=%v to be rejected", badCov)
			}
			if err := svc.SaveVegRecord(state.ContextID, vprocore.VegRecord{
				PlotNumber: "COV001",
				Species:    "TSUGHET",
				Cover5a:    f64Ptr(badCov),
			}); err == nil {
				t.Fatalf("expected Veg Cover5a=%v to be rejected", badCov)
			}
		}

		// StrataCoverTree/Shrub/Herb/Moss: 99.999996 passes, 99.999999 and -0.0001 reject!
		hdr, err := svc.GetPlot(ctx, state.ContextID, "COV001")
		if err != nil {
			t.Fatal(err)
		}
		hdr.StrataCoverTree = f64Ptr(99.999996)
		hdr.StrataCoverShrub = f64Ptr(0.0)
		if err := svc.UpdatePlot(state.ContextID, *hdr); err != nil {
			t.Fatalf("expected StrataCoverTree=99.999996 to pass, got: %v", err)
		}

		for _, badStrata := range []float64{99.999999, 100.0, -0.0001} {
			badHdr := *hdr
			badHdr.StrataCoverHerb = f64Ptr(badStrata)
			if err := svc.UpdatePlot(state.ContextID, badHdr); err == nil {
				t.Fatalf("expected StrataCoverHerb=%v to be rejected", badStrata)
			}
		}

		// Historical untouched StrataCoverMoss = 125.0 must survive unrelated UpdatePlot!
		vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE "Sample_Env" SET "StrataCoverMoss" = 125.0 WHERE "PlotNumber" = 'COV001'`)
		histHdr, err := svc.GetPlot(ctx, state.ContextID, "COV001")
		if err != nil {
			t.Fatal(err)
		}
		histHdr.Elevation = intPtr(610)
		if err := svc.UpdatePlot(state.ContextID, *histHdr); err != nil {
			t.Fatalf("expected unchanged historical StrataCoverMoss=125.0 to pass unrelated UpdatePlot, got: %v", err)
		}
	})

	t.Run("SoilHorizonLMH25DepthSignEnforcementAndHistoricalTolerance", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		if err := svc.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: "SOIL01"}); err != nil {
			t.Fatal(err)
		}

		// Upland L/F/H requires UpperDepth > LowerDepth.
		if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Fm"),
			UpperDepth: f64Ptr(6.0),
			LowerDepth: f64Ptr(1.0),
		}); err != nil {
			t.Fatalf("expected valid upland Fm (6 -> 1) to succeed, got: %v", err)
		}
		if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Fm"),
			UpperDepth: f64Ptr(1.0),
			LowerDepth: f64Ptr(6.0),
		}); err == nil {
			t.Fatalf("expected inverted upland Fm (1 -> 6) to be rejected")
		}

		// Wetland Of/Om/Oh requires UpperDepth < LowerDepth.
		if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Om"),
			UpperDepth: f64Ptr(0.0),
			LowerDepth: f64Ptr(20.0),
		}); err != nil {
			t.Fatalf("expected valid wetland Om (0 -> 20) to succeed, got: %v", err)
		}
		if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Of1"),
			UpperDepth: f64Ptr(20.0),
			LowerDepth: f64Ptr(0.0),
		}); err == nil {
			t.Fatalf("expected inverted wetland Of1 (20 -> 0) to be rejected")
		}

		// Mineral A/B/C/R/W requires UpperDepth < LowerDepth.
		if err := svc.SaveMineralRecord(state.ContextID, vprocore.MineralRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Bf"),
			UpperDepth: f64Ptr(5.0),
			LowerDepth: f64Ptr(30.0),
		}); err != nil {
			t.Fatalf("expected valid mineral Bf (5 -> 30) to succeed, got: %v", err)
		}
		if err := svc.SaveMineralRecord(state.ContextID, vprocore.MineralRecord{
			PlotNumber: "SOIL01",
			Horizon:    strPtr("Bf"),
			UpperDepth: f64Ptr(30.0),
			LowerDepth: f64Ptr(5.0),
		}); err == nil {
			t.Fatalf("expected inverted mineral Bf (30 -> 5) to be rejected")
		}

		// Lithologic discontinuity digits & Roman numerals ('2Bm', '3C', 'IIBm', 'IVCg', '2Of', 'IIFm') must strip prefix before checking LMH25 depth sign!
		for _, minHz := range []string{"2Bm", "3C", "IIBm", "IVCg"} {
			if err := svc.SaveMineralRecord(state.ContextID, vprocore.MineralRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(minHz),
				UpperDepth: f64Ptr(40.0),
				LowerDepth: f64Ptr(15.0),
			}); err == nil {
				t.Fatalf("expected inverted lithologic discontinuity mineral horizon %q (40 -> 15) to be rejected", minHz)
			}
			if err := svc.SaveMineralRecord(state.ContextID, vprocore.MineralRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(minHz),
				UpperDepth: f64Ptr(15.0),
				LowerDepth: f64Ptr(40.0),
			}); err != nil {
				t.Fatalf("expected valid lithologic discontinuity mineral horizon %q (15 -> 40) to succeed, got: %v", minHz, err)
			}
		}
		for _, wetHz := range []string{"2Of", "IIOm"} {
			if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(wetHz),
				UpperDepth: f64Ptr(25.0),
				LowerDepth: f64Ptr(10.0),
			}); err == nil {
				t.Fatalf("expected inverted lithologic discontinuity wetland humus horizon %q (25 -> 10) to be rejected", wetHz)
			}
			if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(wetHz),
				UpperDepth: f64Ptr(10.0),
				LowerDepth: f64Ptr(25.0),
			}); err != nil {
				t.Fatalf("expected valid lithologic discontinuity wetland humus horizon %q (10 -> 25) to succeed, got: %v", wetHz, err)
			}
		}
		for _, upHz := range []string{"2Fm", "IILv"} {
			if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(upHz),
				UpperDepth: f64Ptr(1.0),
				LowerDepth: f64Ptr(5.0),
			}); err == nil {
				t.Fatalf("expected inverted lithologic discontinuity upland humus horizon %q (1 -> 5) to be rejected", upHz)
			}
			if err := svc.SaveHumusRecord(state.ContextID, vprocore.HumusRecord{
				PlotNumber: "SOIL01",
				Horizon:    strPtr(upHz),
				UpperDepth: f64Ptr(5.0),
				LowerDepth: f64Ptr(1.0),
			}); err != nil {
				t.Fatalf("expected valid lithologic discontinuity upland humus horizon %q (5 -> 1) to succeed, got: %v", upHz, err)
			}
		}

		// Historical untouched depth anomaly in DB must survive unrelated Comment update!
		vprotest.MutateContextFixture(t, state.ProjectPath, `
			INSERT INTO "Sample_Humus" ("PlotNumber", "ID", "Horizon", "UpperDepth", "LowerDepth")
			VALUES ('SOIL01', 900, 'Lv', 1.0, 5.0);
		`)
		humusRows, err := svc.ListHumusRecords(ctx, state.ContextID, "SOIL01")
		if err != nil {
			t.Fatal(err)
		}
		var histRow *vprocore.HumusRecord
		for i := range humusRows {
			if humusRows[i].ID == 900 {
				histRow = &humusRows[i]
				break
			}
		}
		if histRow == nil {
			t.Fatal("missing seeded historical humus row 900")
		}
		histRow.Comment = strPtr("untouched depths")
		if err := svc.UpdateHumusRecord(state.ContextID, *histRow); err != nil {
			t.Fatalf("expected historical untouched depth row Comment update to succeed, got: %v", err)
		}
	})

	t.Run("AllocateChildIDUnionsPositiveAuditIDs", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		if err := svc.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: "AUDID1"}); err != nil {
			t.Fatal(err)
		}

		// Clear positive IDs 1 & 2 from Sample_Veg (if any) and record them only in Sample_Audit!
		vprotest.MutateContextFixture(t, state.ProjectPath, `
			DELETE FROM "Sample_Veg" WHERE "ID" IN (1, 2);
			INSERT INTO "Sample_Audit" ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID")
			VALUES
				('Sample','Importer','AUDID1','_Veg','Species','2025-01-01 00:00:00','PSEUMEN',NULL,0,0,1),
				('Sample','Importer','AUDID1','sample_veg','Species','2025-01-01 00:00:00','TSUGHET',NULL,0,0,2);
		`)

		if err := svc.SaveVegRecord(state.ContextID, vprocore.VegRecord{
			PlotNumber: "AUDID1",
			Species:    "ABIEAMA",
		}); err != nil {
			t.Fatalf("SaveVegRecord failed: %v", err)
		}

		rows, err := svc.ListVegRecords(ctx, state.ContextID, "AUDID1")
		if err != nil || len(rows) != 1 {
			t.Fatalf("ListVegRecords failed: rows=%v err=%v", rows, err)
		}
		if rows[0].ID == 1 || rows[0].ID == 2 || rows[0].ID < 3 {
			t.Fatalf("AllocateChildID reused deleted Audit ID: got ID=%d, expected >= 3", rows[0].ID)
		}
	})
}
