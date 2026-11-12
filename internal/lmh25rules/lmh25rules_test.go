package lmh25rules_test

import (
	"math"
	"testing"

	"github.com/boostao/vpro-wails/internal/lmh25rules"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func strPtr(s string) *string     { return &s }
func fltPtr(f float64) *float64   { return &f }
func intPtr(i int) *int           { return &i }

func hasIssue(issues []lmh25rules.DiagnosticIssue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

func TestUplandHumusThicknessSubstratesAndContiguity(t *testing.T) {
	header := vprocore.FS882Header{
		PlotNumber:             "00337",
		HumusThickness:         fltPtr(8.0), // diverges from 12 - 0 = 12.0 cm!
		SubstrateDecWood:       fltPtr(15.0),
		SubstrateBedRock:       fltPtr(0.0),
		SubstrateRocks:         fltPtr(5.0),
		SubstrateMineralSoil:   fltPtr(10.0),
		SubstrateOrganicMatter: fltPtr(65.0), // sum = 95.0 != 100.0
		MesoSlopePos:           strPtr("MD"),
		Slope:                  fltPtr(18.0),
		Aspect:                 intPtr(90),
		MoistureRegime:         strPtr("4"),
		NutrientRegime:         strPtr("C"),
	}
	humus := []vprocore.HumusRecord{
		{ID: 1, PlotNumber: "00337", Horizon: strPtr("L"), UpperDepth: fltPtr(12.0), LowerDepth: fltPtr(9.0)},
		{ID: 2, PlotNumber: "00337", Horizon: strPtr("Fm"), UpperDepth: fltPtr(8.0), LowerDepth: fltPtr(2.0)}, // 1 cm gap 9->8!
		{ID: 3, PlotNumber: "00337", Horizon: strPtr("Hh"), UpperDepth: fltPtr(2.0), LowerDepth: fltPtr(0.0)},
	}
	mineral := []vprocore.MineralRecord{
		{ID: 10, PlotNumber: "00337", Horizon: strPtr("Bm1"), UpperDepth: fltPtr(0.0), LowerDepth: fltPtr(25.0), Texture: strPtr("SL"), Colour: strPtr("10YR 4/4"), PercentCoarseFragsTotal: intPtr(25)},
		{ID: 11, PlotNumber: "00337", Horizon: strPtr("BC"), UpperDepth: fltPtr(25.0), LowerDepth: fltPtr(65.0), Texture: strPtr("SL"), PercentCoarseFragsTotal: intPtr(30)},
	}

	diag := lmh25rules.EvaluatePlotDiagnostics(header, humus, mineral)
	if diag.DerivedHumusThickness == nil || math.Abs(*diag.DerivedHumusThickness-12.0) > 1e-6 {
		t.Fatalf("expected DerivedHumusThickness=12.0, got %v", diag.DerivedHumusThickness)
	}
	if !hasIssue(diag.Issues, "HUMUS_THICKNESS_MISMATCH") {
		t.Fatalf("expected HUMUS_THICKNESS_MISMATCH, got %+v", diag.Issues)
	}
	if diag.SubstrateValid || diag.SubstrateSum == nil || math.Abs(*diag.SubstrateSum-95.0) > 1e-6 {
		t.Fatalf("expected SubstrateValid=false and sum=95.0, got valid=%t sum=%v", diag.SubstrateValid, diag.SubstrateSum)
	}
	if !hasIssue(diag.Issues, "SUBSTRATE_SUM_NOT_100") {
		t.Fatalf("expected SUBSTRATE_SUM_NOT_100, got %+v", diag.Issues)
	}
	if !hasIssue(diag.Issues, "HORIZON_CONTIGUITY_GAP") {
		t.Fatalf("expected HORIZON_CONTIGUITY_GAP for 9.0 -> 8.0 cm, got %+v", diag.Issues)
	}
	if diag.KeyedSMRClass == nil || *diag.KeyedSMRClass != 4 || diag.SMRDivergent {
		t.Fatalf("expected mesic rSMR=4 with zero divergence, got %s (%v)", diag.KeyedSMR, diag.KeyedSMRClass)
	}
	if diag.KeyedSNRClass == nil || *diag.KeyedSNRClass != 2 || diag.SNRDivergent {
		t.Fatalf("expected medium rSNR=C (class 2), got %s (%v)", diag.KeyedSNR, diag.KeyedSNRClass)
	}
}

func TestHorizonDirectionalSignAndVonPostTable223(t *testing.T) {
	// Test inverted upland LFH (0 -> 5 instead of 5 -> 0), inverted mineral (30 -> 10), and invalid vonPost on Of/Om/Oh
	header := vprocore.FS882Header{
		PlotNumber:     "WET01",
		SoilClassGroup: strPtr("F"), // Fibrisol (Organic Order)
		MesoSlopePos:   strPtr("DP"),
		Slope:          fltPtr(1.0),
		SoilDrainage:   strPtr("v"),
		MoistureRegime: strPtr("2"), // Deliberately wrong field SMR=2 on very poorly drained bog (Keyed=8 -> divergence 6!)
		NutrientRegime: strPtr("E"), // Deliberately wrong field SNR=E on fibric Of bog (Keyed=A-B -> divergence >= 3!)
	}
	humus := []vprocore.HumusRecord{
		{ID: 1, PlotNumber: "WET01", Horizon: strPtr("L"), UpperDepth: fltPtr(0.0), LowerDepth: fltPtr(5.0)},                     // wrong sign for L
		{ID: 2, PlotNumber: "WET01", Horizon: strPtr("Of1"), UpperDepth: fltPtr(0.0), LowerDepth: fltPtr(30.0), VonPost: intPtr(6)}, // Of must be 1..4!
		{ID: 3, PlotNumber: "WET01", Horizon: strPtr("Om1"), UpperDepth: fltPtr(30.0), LowerDepth: fltPtr(60.0), VonPost: intPtr(2)}, // Om must be 5..6!
		{ID: 4, PlotNumber: "WET01", Horizon: strPtr("Oh1"), UpperDepth: fltPtr(80.0), LowerDepth: fltPtr(50.0), VonPost: intPtr(5)}, // wrong sign + Oh must be 7..10!
	}
	mineral := []vprocore.MineralRecord{
		{ID: 5, PlotNumber: "WET01", Horizon: strPtr("Cg"), UpperDepth: fltPtr(100.0), LowerDepth: fltPtr(80.0)}, // wrong sign!
	}

	diag := lmh25rules.EvaluatePlotDiagnostics(header, humus, mineral)
	for _, expectedCode := range []string{
		"HORIZON_SIGN_UPLAND_LFH",
		"HORIZON_SIGN_WETLAND_O",
		"HORIZON_SIGN_MINERAL",
		"VONPOST_OUT_OF_RANGE_OF",
		"VONPOST_OUT_OF_RANGE_OM",
		"VONPOST_OUT_OF_RANGE_OH",
		"SMR_KEY_DIVERGENCE",
		"SNR_KEY_DIVERGENCE",
	} {
		if !hasIssue(diag.Issues, expectedCode) {
			t.Errorf("expected diagnostic issue %s, got issues: %+v", expectedCode, diag.Issues)
		}
	}
	if !diag.SMRDivergent || diag.KeyedSMRClass == nil || *diag.KeyedSMRClass != 8 {
		t.Fatalf("expected KeyedSMR=8 and SMRDivergent=true, got %s (%v) div=%t", diag.KeyedSMR, diag.KeyedSMRClass, diag.SMRDivergent)
	}
	if !diag.SNRDivergent || diag.KeyedSNR != "A-B" {
		t.Fatalf("expected KeyedSNR=A-B and SNRDivergent=true, got %s div=%t", diag.KeyedSNR, diag.SNRDivergent)
	}
}

func TestKey94And96CrestXericVsRichMullToeSlope(t *testing.T) {
	// Case A: Very xeric / oligotrophic rocky ridge crest (rSMR = 0, rSNR = A)
	crest := lmh25rules.EvaluatePlotDiagnostics(
		vprocore.FS882Header{
			PlotNumber:           "CREST1",
			MesoSlopePos:         strPtr("CR"),
			Slope:                fltPtr(45.0),
			Aspect:               intPtr(210), // insolation SW
			SubstrateBedRock:     fltPtr(25.0),
			SubstrateRocks:       fltPtr(75.0),
			RootRestrictingDepth: intPtr(15),
			RootRestrictingType:  strPtr("L"),
			RootZoneParticleSize: strPtr("S"),
			HumusForm:            strPtr("R"), // Mor
			MoistureRegime:       strPtr("0"),
			NutrientRegime:       strPtr("A"),
		},
		[]vprocore.HumusRecord{
			{ID: 1, PlotNumber: "CREST1", Horizon: strPtr("L"), UpperDepth: fltPtr(2.0), LowerDepth: fltPtr(0.0)},
		},
		[]vprocore.MineralRecord{
			{ID: 2, PlotNumber: "CREST1", Horizon: strPtr("Ae"), UpperDepth: fltPtr(0.0), LowerDepth: fltPtr(4.0), Texture: strPtr("S"), PercentCoarseFragsTotal: intPtr(65)},
			{ID: 3, PlotNumber: "CREST1", Horizon: strPtr("Bm"), UpperDepth: fltPtr(4.0), LowerDepth: fltPtr(15.0), Texture: strPtr("S"), Colour: strPtr("10YR 7/2"), PercentCoarseFragsTotal: intPtr(70)},
			{ID: 4, PlotNumber: "CREST1", Horizon: strPtr("R"), UpperDepth: fltPtr(15.0), LowerDepth: fltPtr(20.0)},
		},
	)
	if crest.KeyedSMRClass == nil || *crest.KeyedSMRClass != 0 {
		t.Fatalf("expected crest KeyedSMRClass=0, got %v (%s)", crest.KeyedSMRClass, crest.KeyedSMR)
	}
	if crest.KeyedSNRClass == nil || *crest.KeyedSNRClass != 0 || crest.KeyedSNR != "A" {
		t.Fatalf("expected crest KeyedSNR=A, got %s", crest.KeyedSNR)
	}

	// Case B: Hygric / Very rich seepage toe-slope with Mull + thick Ah + dark B (rSMR = 6, rSNR = E)
	toe := lmh25rules.EvaluatePlotDiagnostics(
		vprocore.FS882Header{
			PlotNumber:           "TOE1",
			MesoSlopePos:         strPtr("TO"),
			Slope:                fltPtr(12.0),
			Aspect:               intPtr(45),
			SeepageDepth:         intPtr(40),
			WaterSource:          strPtr("G"),
			SoilDrainage:         strPtr("i"),
			HumusForm:            strPtr("VL"), // Vermimull
			RootZoneParticleSize: strPtr("FL"),
			MoistureRegime:       strPtr("6"),
			NutrientRegime:       strPtr("E"),
		},
		nil,
		[]vprocore.MineralRecord{
			{ID: 1, PlotNumber: "TOE1", Horizon: strPtr("Ah"), UpperDepth: fltPtr(0.0), LowerDepth: fltPtr(14.0), Texture: strPtr("SiL"), Colour: strPtr("10YR 2/1"), PercentCoarseFragsTotal: intPtr(10)},
			{ID: 2, PlotNumber: "TOE1", Horizon: strPtr("Bm"), UpperDepth: fltPtr(14.0), LowerDepth: fltPtr(38.0), Texture: strPtr("SiL"), Colour: strPtr("10YR 3/2"), PercentCoarseFragsTotal: intPtr(12)},
			{ID: 3, PlotNumber: "TOE1", Horizon: strPtr("Bg"), UpperDepth: fltPtr(38.0), LowerDepth: fltPtr(75.0), Texture: strPtr("SiL"), Colour: strPtr("10YR 3/2"), PercentCoarseFragsTotal: intPtr(15)},
		},
	)
	if toe.KeyedSMRClass == nil || *toe.KeyedSMRClass != 6 {
		t.Fatalf("expected toe KeyedSMRClass=6, got %v (%s)", toe.KeyedSMRClass, toe.KeyedSMR)
	}
	if toe.KeyedSNRClass == nil || *toe.KeyedSNRClass != 4 || toe.KeyedSNR != "E" {
		t.Fatalf("expected toe KeyedSNR=E, got %s", toe.KeyedSNR)
	}
}
