package twopage_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestTwoPageSoilLegacyIDZeroDeleteRestoreAndLMH25DepthSigns(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed two plots ('SOIL00' and 'SOIL01') that BOTH contain legacy imported ID=0 soil rows!
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO Sample_Env (PlotNumber, ProjectID, Zone) VALUES ('SOIL00', 'SMP', 'CWH'), ('SOIL01', 'SMP', 'CWH');
		INSERT INTO Sample_Admin (Plot, PlotType) VALUES ('SOIL00', 'FS882'), ('SOIL01', 'FS882');
		INSERT INTO Sample_Humus (PlotNumber, Horizon, UpperDepth, LowerDepth, HumusFormpH, Comment, Flag, ID) VALUES
			('SOIL00', 'Lv', 6.0, 2.0, 4.1, 'legacy zero id', 0, 0),
			('SOIL00', 'Fq', 2.0, 9.0, 4.0, 'historical inverted untouched', 0, 501),
			('SOIL01', 'Fm', 5.0, 0.0, 3.8, 'other plot row', 0, 502);
		INSERT INTO Sample_Mineral (PlotNumber, Horizon, UpperDepth, LowerDepth, Texture, Colour, Flag, ID) VALUES
			('SOIL00', 'Bm1', 0.0, 30.0, 'SL', '10YR 4/4', 0, 0);
	`)

	soilSvc, err := twopage.NewTwoPageSoilService(svc)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Delete legacy ID=0 Humus row on 'SOIL00' and verify ID=0 is reserved in __VPRO_ChildIdentity!
	delZero, err := twopage.DeleteTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.DeleteTwoPageSoilHorizonRequest{
		RequestID:  "22222222-3333-4444-8555-666666666601",
		PlotNumber: "SOIL00",
		Form:       "SoilHumusXL",
		ID:         0,
	})
	if err != nil {
		t.Fatalf("DeleteTwoPageSoilHorizon(ID=0) failed: %v", err)
	}
	if delZero.DeletedID != 0 || !delZero.ReservedInLedger || delZero.Horizon != "Lv" {
		t.Fatalf("unexpected ID=0 delete result: %+v", delZero)
	}

	roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer roDB.Close()

	var reservedZero int
	if err := roDB.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ChildTable" = '"Sample_Humus"' AND "ID" = 0`).Scan(&reservedZero); err != nil || reservedZero != 1 {
		t.Fatalf("expected ('\"Sample_Humus\"', 0) reserved in __VPRO_ChildIdentity, got %d (err=%v)", reservedZero, err)
	}

	// Restore legacy ID=0 Humus row on 'SOIL00' even though 'SOIL01' also has ID=0!
	resZero, err := twopage.RestoreTwoPageSoilHorizon(svc, ctx, state.ContextID, twopage.RestoreTwoPageSoilHorizonRequest{
		RequestID:       "22222222-3333-4444-8555-666666666602",
		PlotNumber:      "SOIL00",
		Form:            "SoilHumusXL",
		DeleteRequestID: delZero.RequestID,
	})
	if err != nil || resZero.RestoredID != 0 || resZero.Horizon != "Lv" {
		t.Fatalf("RestoreTwoPageSoilHorizon(ID=0) failed when sibling plot had ID=0: %+v (%v)", resZero, err)
	}

	// 2. Historical tolerance: editing Comment ONLY on row ID=501 (which has historical inverted Fq 2..9) MUST succeed!
	origHumus, err := soilSvc.GetOriginal(ctx, state.ContextID, "SOIL00", "SoilHumusXL")
	if err != nil {
		t.Fatal(err)
	}
	findCol := func(orig *twopage.TwoPageSoilOriginal, name string) int {
		for i, c := range orig.Records.Columns {
			if c.Name == name {
				return i
			}
		}
		t.Fatalf("column %s not found", name)
		return -1
	}
	idCol := findCol(origHumus, "ID")
	commentCol := findCol(origHumus, "Comment")
	var fqRowIdx, lvRowIdx int = -1, -1
	for i, r := range origHumus.Records.Rows {
		if r.Cells[idCol].Integer != nil && *r.Cells[idCol].Integer == "501" {
			fqRowIdx = i
		}
		if r.Cells[idCol].Integer != nil && *r.Cells[idCol].Integer == "0" {
			lvRowIdx = i
		}
	}
	if fqRowIdx < 0 || lvRowIdx < 0 {
		t.Fatalf("missing Fq or Lv row in original: %+v", origHumus.Records.Rows)
	}

	txtCell := func(s string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &s}
	}
	fltCell := func(v float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &v}
	}

	histTolPayload, _ := json.Marshal(map[string]any{
		"requestId": "22222222-3333-4444-8555-666666666603",
		"original":  origHumus,
		"edits": []map[string]any{{
			"contextId": state.ContextID,
			"form":      "SoilHumusXL",
			"rowId":     origHumus.Records.Rows[fqRowIdx].RowID,
			"column":    "Comment",
			"expected":  origHumus.Records.Rows[fqRowIdx].Cells[commentCol],
			"value":     txtCell("updated comment only"),
		}},
	})
	if _, err := soilSvc.Save(ctx, state.ContextID, string(histTolPayload)); err != nil {
		t.Fatalf("historical tolerance broken on untouched inverted Fq depth when editing Comment: %v", err)
	}

	// 3. LMH25 depth sign enforcement on edited Humus & Mineral rows:
	// 3a. Upland Lv (6..2) edited to UpperDepth=1.0 (1.0 <= 2.0) MUST be rejected!
	origHumus2, err := soilSvc.GetOriginal(ctx, state.ContextID, "SOIL00", "SoilHumusXL")
	if err != nil {
		t.Fatal(err)
	}
	upperHCol := findCol(origHumus2, "UpperDepth")
	lowerHCol := findCol(origHumus2, "LowerDepth")
	hzHCol := findCol(origHumus2, "Horizon")
	for i, r := range origHumus2.Records.Rows {
		if r.Cells[idCol].Integer != nil && *r.Cells[idCol].Integer == "0" {
			lvRowIdx = i
		}
	}
	badUplandJSON, _ := json.Marshal(map[string]any{
		"requestId": "22222222-3333-4444-8555-666666666604",
		"original":  origHumus2,
		"edits": []map[string]any{{
			"contextId": state.ContextID,
			"form":      "SoilHumusXL",
			"rowId":     origHumus2.Records.Rows[lvRowIdx].RowID,
			"column":    "UpperDepth",
			"expected":  origHumus2.Records.Rows[lvRowIdx].Cells[upperHCol],
			"value":     fltCell(1.0),
		}},
	})
	if _, err := soilSvc.Save(ctx, state.ContextID, string(badUplandJSON)); err == nil {
		t.Fatal("expected TwoPage SoilHumusXL to reject upland Lv with UpperDepth=1.0 <= LowerDepth=2.0")
	}

	// 3b. Wetland Of with UpperDepth=15.0 >= LowerDepth=5.0 MUST be rejected, while UpperDepth=0.0 < LowerDepth=15.0 succeeds!
	badWetlandJSON, _ := json.Marshal(map[string]any{
		"requestId": "22222222-3333-4444-8555-666666666605",
		"original":  origHumus2,
		"edits": []map[string]any{
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "Horizon",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[hzHCol],
				"value":    txtCell("Of"),
			},
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "UpperDepth",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[upperHCol],
				"value":    fltCell(15.0),
			},
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "LowerDepth",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[lowerHCol],
				"value":    fltCell(5.0),
			},
		},
	})
	if _, err := soilSvc.Save(ctx, state.ContextID, string(badWetlandJSON)); err == nil {
		t.Fatal("expected TwoPage SoilHumusXL to reject wetland Of with UpperDepth=15.0 >= LowerDepth=5.0")
	}
	goodWetlandJSON, _ := json.Marshal(map[string]any{
		"requestId": "22222222-3333-4444-8555-666666666607",
		"original":  origHumus2,
		"edits": []map[string]any{
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "Horizon",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[hzHCol],
				"value":    txtCell("Of"),
			},
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "UpperDepth",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[upperHCol],
				"value":    fltCell(0.0),
			},
			{
				"contextId": state.ContextID, "form": "SoilHumusXL",
				"rowId": origHumus2.Records.Rows[lvRowIdx].RowID, "column": "LowerDepth",
				"expected": origHumus2.Records.Rows[lvRowIdx].Cells[lowerHCol],
				"value":    fltCell(15.0),
			},
		},
	})
	if _, err := soilSvc.Save(ctx, state.ContextID, string(goodWetlandJSON)); err != nil {
		t.Fatalf("expected valid wetland Of (0..15) to succeed, got %v", err)
	}

	// 3c. Mineral Bm1 (0..30) edited to UpperDepth=35.0 >= LowerDepth=30.0 MUST be rejected!
	origMin, err := soilSvc.GetOriginal(ctx, state.ContextID, "SOIL00", "SoilMineralXL")
	if err != nil {
		t.Fatal(err)
	}
	upperMCol := findCol(origMin, "UpperDepth")
	badMinJSON, _ := json.Marshal(map[string]any{
		"requestId": "22222222-3333-4444-8555-666666666606",
		"original":  origMin,
		"edits": []map[string]any{{
			"contextId": state.ContextID,
			"form":      "SoilMineralXL",
			"rowId":     origMin.Records.Rows[0].RowID,
			"column":    "UpperDepth",
			"expected":  origMin.Records.Rows[0].Cells[upperMCol],
			"value":     fltCell(35.0),
		}},
	})
	if _, err := soilSvc.Save(ctx, state.ContextID, string(badMinJSON)); err == nil {
		t.Fatal("expected TwoPage SoilMineralXL to reject mineral Bm1 with UpperDepth=35.0 >= LowerDepth=30.0")
	}
}
