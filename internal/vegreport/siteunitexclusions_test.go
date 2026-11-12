package vegreport

import (
	"context"
	"slices"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestMatchSiteUnitExclusionModesAndDefaults(t *testing.T) {
	rules, err := NormalizeExclusionRules(nil, true)
	if err != nil || len(rules) != 3 {
		t.Fatalf("expected 3 default exclusion presets, got %+v (%v)", rules, err)
	}

	cases := []struct {
		unit    string
		exclude bool
		pattern string
	}{
		{"CWHvm1/01.support", true, ".support"},
		{"CWHvm1/01.SUPPORT", true, ".support"},
		{"CWHvm1/01.support-2", true, ".support"},
		{"SBSmk1/05.poor", true, ".poor"},
		{"IDFdk1/03.Nudum", true, ".nudum"},
		{"CWHvm1/01", false, ""},
		{"SupportCreek/01", false, ""}, // no leading dot before 'support' -> must NOT false-positive!
	}
	for _, tc := range cases {
		got, pat := MatchSiteUnitExclusion(tc.unit, rules)
		if got != tc.exclude || pat != tc.pattern {
			t.Fatalf("MatchSiteUnitExclusion(%q) = (%v, %q), want (%v, %q)", tc.unit, got, pat, tc.exclude, tc.pattern)
		}
	}

	// Wildcard and explicit phrase modes
	custom := []SiteUnitExclusionRule{
		{Pattern: "CWH*/99*", MatchMode: "wildcard", Enabled: true},
		{Pattern: "unassigned", MatchMode: "contains", Enabled: true},
		{Pattern: ".transitional", MatchMode: "suffix", Enabled: false},
	}
	if ok, _ := MatchSiteUnitExclusion("CWHvm1/99a", custom); !ok {
		t.Fatal("expected wildcard CWH*/99* to match CWHvm1/99a")
	}
	if ok, _ := MatchSiteUnitExclusion("SBSmk1/UNASSIGNED_PLOT", custom); !ok {
		t.Fatal("expected case-insensitive contains 'unassigned' to match")
	}
	if ok, _ := MatchSiteUnitExclusion("SBSmk1/01.transitional", custom); ok {
		t.Fatal("disabled rule must not exclude")
	}
}

func TestSummaryAndLongVegetationExclusionsAndHierarchyBreakpoints(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// Pick 5 real plots from Sample_Veg so every unit has real vegetation observations.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		CREATE TABLE Wave2B_SU (PlotNumber VARCHAR, SiteUnit VARCHAR);
		CREATE UNIQUE INDEX uidx_Wave2B_SU_PlotNumber ON Wave2B_SU(PlotNumber);
		INSERT INTO _table_metadata (table_name, description) VALUES ('Wave2B_SU', 'VP04');

		INSERT INTO Wave2B_SU (PlotNumber, SiteUnit) VALUES
			('108050',  'MHmm1/01'),
			('108050x', 'MHmm1/05'),
			('202050',  'MHmm1/01.support'),
			('579279',  'MHmm1/05.poor'),
			('HLB056',  'SBSdk/01.nudum'),
			('Cb13011', 'SBSdk/01');

		CREATE TABLE Wave2B_Hierarchy (
			ID INTEGER PRIMARY KEY,
			Name VARCHAR NOT NULL,
			Parent INTEGER,
			Level SMALLINT
		);
		INSERT INTO _table_metadata (table_name, description) VALUES ('Wave2B_Hierarchy', 'VP04');
		INSERT INTO Wave2B_Hierarchy (ID, Name, Parent, Level) VALUES
			(1, 'Vaccinio-Piceetea', NULL, 2),
			(2, '~Tsugo-Mertensietalia', 1, 4),
			(3, 'Mertensienion', 2, 6),
			(4, 'MHmm1/01', 3, 11),
			(5, 'MHmm1/05', 3, 11),
			(6, 'Piceetalia', 1, 4),
			(7, 'SBSdk/01', 6, 11);
	`)

	sel := vprotest.ContextSelection(state)
	sel.SU = "Wave2B"
	sel.SUPath = state.ProjectPath
	sel.Hierarchy = "Wave2B"
	sel.HierarchyPath = state.ProjectPath
	state, err := service.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext failed: %v", err)
	}

	// 1. PreviewSiteUnitExclusions with default presets (.support, .poor, .nudum)
	excPreview, err := PreviewSiteUnitExclusions(ctx, service, state.ContextID, SiteUnitExclusionPreviewRequest{
		UseDefaultPresets:       true,
		UseHierarchyBreakpoints: true,
	})
	if err != nil {
		t.Fatalf("PreviewSiteUnitExclusions failed: %v", err)
	}
	if excPreview.Exclusions.TotalPlotsBefore != 6 ||
		excPreview.Exclusions.ExcludedPlots != 3 ||
		excPreview.Exclusions.RetainedPlots != 3 ||
		len(excPreview.Exclusions.ExcludedUnits) != 3 {
		t.Fatalf("unexpected exclusion summary: %+v", excPreview.Exclusions)
	}
	if excPreview.Breakpoints == nil || excPreview.Breakpoints.TotalUnitsBefore != 3 || excPreview.Breakpoints.TotalUnitsAfter != 2 {
		t.Fatalf("expected 3 retained units (MHmm1/01, MHmm1/05, SBSdk/01) to roll up to 2 (~Tsugo-Mertensietalia, SBSdk/01), got %+v", excPreview.Breakpoints)
	}

	// 2. PreviewSummaryVegetationWithExclusions (exclusions ONLY, without breakpoints)
	sumFiltered, err := PreviewSummaryVegetationWithExclusions(ctx, service, state.ContextID, SummaryVegetationExclusionRequest{
		Average:           1,
		Display:           6,
		GroupAndOrder:     11,
		UseDefaultPresets: true,
	})
	if err != nil {
		t.Fatalf("PreviewSummaryVegetationWithExclusions failed: %v", err)
	}
	gotUnits := make([]string, 0, len(sumFiltered.Preview.Units))
	for _, u := range sumFiltered.Preview.Units {
		gotUnits = append(gotUnits, u.Code)
	}
	wantUnits := []string{"MHmm1/01", "MHmm1/05", "SBSdk/01"}
	if !slices.Equal(gotUnits, wantUnits) {
		t.Fatalf("Summary Vegetation units after exclusions = %v, want %v", gotUnits, wantUnits)
	}

	// 3. PreviewSummaryVegetationWithExclusions WITH dynamic '~' Hierarchy Breakpoint rollup!
	// MHmm1/01 + MHmm1/05 roll up into '~Tsugo-Mertensietalia' with NumPlots = 2, while .support/.poor/.nudum stay excluded!
	sumRolled, err := PreviewSummaryVegetationWithExclusions(ctx, service, state.ContextID, SummaryVegetationExclusionRequest{
		Average:                 1,
		Display:                 6,
		GroupAndOrder:           11,
		UseDefaultPresets:       true,
		UseHierarchyBreakpoints: true,
	})
	if err != nil {
		t.Fatalf("PreviewSummaryVegetationWithExclusions with breakpoints failed: %v", err)
	}
	if len(sumRolled.Preview.Units) != 2 {
		t.Fatalf("expected 2 summary units after ~ breakpoint rollup, got %+v", sumRolled.Preview.Units)
	}
	unitPlots := map[string]int{}
	for _, u := range sumRolled.Preview.Units {
		unitPlots[u.Code] = u.NumPlots
	}
	if unitPlots["~Tsugo-Mertensietalia"] != 2 || unitPlots["SBSdk/01"] != 1 {
		t.Fatalf("expected ~Tsugo-Mertensietalia=2 plots and SBSdk/01=1 plot, got %+v", unitPlots)
	}

	// 4. PreviewLongVegetationWithExclusions: excludes .support, .poor, .nudum cleanly!
	longFiltered, err := PreviewLongVegetationWithExclusions(ctx, service, state.ContextID, LongVegetationExclusionRequest{
		UseDefaultPresets: true,
	})
	if err != nil {
		t.Fatalf("PreviewLongVegetationWithExclusions failed: %v", err)
	}
	longUnits := make([]string, 0, len(longFiltered.Preview.Report.Units))
	for _, u := range longFiltered.Preview.Report.Units {
		if u.Code.Text != nil {
			longUnits = append(longUnits, *u.Code.Text)
		}
	}
	if !slices.Equal(longUnits, wantUnits) {
		t.Fatalf("Long Vegetation units after exclusions = %v, want %v", longUnits, wantUnits)
	}

	// Verify physical Wave2B_SU was NEVER mutated by dynamic exclusion/breakpoint previews!
	recheck, err := ResolveHierarchyBreakpointUnits(ctx, service, state.ContextID, HierarchyBreakpointRequest{
		UseDefaultPresets: false,
	})
	if err != nil || recheck.TotalUnitsBefore != 6 {
		t.Fatalf("physical Wave2B_SU must retain all 6 original units, got %+v (%v)", recheck, err)
	}
}
