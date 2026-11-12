package projectops_test

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestCreateNewProjectAndSaveAsAndSplinterAndCompare(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// 1. Naming validation: reserve None, Sample, master, invalid identifiers.
	for _, invalid := range []string{"", "Sample", "sample", "None", "BECMaster", "coast_master_2", "1BadStart", "Bad Space", strings.Repeat("A", 32)} {
		_, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
			ProjectName: invalid,
		})
		if err == nil {
			t.Fatalf("expected invalid project name %q to be rejected", invalid)
		}
	}

	// 2. CreateNewProject with LMH25_2023_FS882 preset without immediate switch.
	created, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:       "Kootenay2026",
		ProjectID:         "KOOT26",
		ProjectTitle:      "Kootenay Subzone Revision",
		PresetID:          "LMH25_2023_FS882",
		ActivateOnSuccess: false,
	})
	if err != nil {
		t.Fatalf("CreateNewProject failed: %v", err)
	}
	if created.Activated || created.DestinationSHA == "" {
		t.Fatalf("unexpected CreateNewProject result: %+v", created)
	}

	// Atomic no-overwrite check: attempting to create at the same path must fail without mutating.
	if _, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:     "Kootenay2026",
		DestinationPath: created.DestinationPath,
	}); err == nil {
		t.Fatalf("expected no-overwrite failure when destination already exists")
	}

	// Inspect created DB for VP08 tags, Metadata row ID=1, __VPRO_ChildIdentity, Datum='2', and Audit rows.
	newDB, err := sqlutil.OpenReadOnly(created.DestinationPath)
	if err != nil {
		t.Fatal(err)
	}
	defer newDB.Close()

	var vp08Count int
	if err := newDB.QueryRow(`SELECT COUNT(*) FROM "_table_metadata" WHERE description = 'VP08'`).Scan(&vp08Count); err != nil {
		t.Fatal(err)
	}
	if vp08Count != 8 {
		t.Fatalf("expected 8 VP08 _table_metadata rows, got %d", vp08Count)
	}

	var datum, std, coverC string
	if err := newDB.QueryRow(`SELECT "Datum", "EcosysCollectionStandard", "CoverCDescription" FROM "Kootenay2026_Metadata" WHERE "ID" = 1 AND "ProjectID" = 'KOOT26'`).
		Scan(&datum, &std, &coverC); err != nil {
		t.Fatalf("read initial metadata row: %v", err)
	}
	if datum != "2" || std != "LMH25" || coverC == "" {
		t.Fatalf("unexpected preset values datum=%q std=%q coverC=%q", datum, std, coverC)
	}

	var auditCount int
	if err := newDB.QueryRow(`SELECT COUNT(*) FROM "Kootenay2026_Audit" WHERE "Table" IN ('NewProject','USysAllSpecs','USysTableOfLists')`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("expected 3 initial NewProject audit rows, got %d", auditCount)
	}

	// 3. SaveProjectAs from active Sample.db -> SampleBranch.db preserving companion _SU, _Hierarchy, _Lump, _Profile.
	savedAs, err := projectops.SaveProjectAs(svc, ctx, state.ContextID, projectops.SaveProjectAsRequest{
		NewProjectName:    "SampleBranch",
		ActivateOnSuccess: false,
	})
	if err != nil {
		t.Fatalf("SaveProjectAs failed: %v", err)
	}
	for _, wantComp := range []string{"SampleBranch_SU", "SampleBranch_Hierarchy", "SampleBranch_Lump", "SampleBranch_Profile"} {
		found := false
		for _, got := range savedAs.CompanionTables {
			if got == wantComp {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected companion table %s in %v", wantComp, savedAs.CompanionTables)
		}
	}

	// Compare Sample (active) vs SampleBranch -> all plots must be Identical!
	cmpIdentical, err := projectops.CompareProjects(svc, ctx, state.ContextID, projectops.CompareProjectsRequest{
		TargetPath:    savedAs.DestinationPath,
		TargetProject: "SampleBranch",
	})
	if err != nil {
		t.Fatalf("CompareProjects identical failed: %v", err)
	}
	if cmpIdentical.TotalPlots == 0 || cmpIdentical.IdenticalCount != cmpIdentical.TotalPlots || len(cmpIdentical.FieldDiffs) != 0 {
		t.Fatalf("expected 100%% identical plots after SaveProjectAs, got %+v", cmpIdentical)
	}

	// 4. ExportSplinterProject with explicit subset of plots (including leading-zero plot inserted first).
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO Sample_Env (PlotNumber, ProjectID, Zone, SubZone, Elevation) VALUES ('00997', 'SMP', 'CWH', 'vm1', 350);
		INSERT INTO Sample_Admin (Plot, PlotType, UserSiteUnit) VALUES ('00997', 'FS882', 'CWHvm1/01');
	`)
	splinter, err := projectops.ExportSplinterProject(svc, ctx, state.ContextID, projectops.ExportSplinterRequest{
		SplinterProject:   "CoastSplinter",
		PlotNumbers:       []string{"00997"},
		SiteUnits:         []string{"CWHvm1/01"},
		ActivateOnSuccess: false,
	})
	if err != nil {
		t.Fatalf("ExportSplinterProject failed: %v", err)
	}
	if splinter.ExportedPlotCount != 1 || !splinter.CreatedSUTable {
		t.Fatalf("expected 1 exported splinter plot with SU table, got %+v", splinter)
	}

	// Compare active Sample (Source) vs CoastSplinter (Target) -> 1 Identical ('00997') + remaining Added in Source!
	cmpSplinter, err := projectops.CompareProjects(svc, ctx, state.ContextID, projectops.CompareProjectsRequest{
		TargetPath:    splinter.DestinationPath,
		TargetProject: "CoastSplinter",
	})
	if err != nil {
		t.Fatalf("CompareProjects splinter failed: %v", err)
	}
	if cmpSplinter.IdenticalCount != 1 || cmpSplinter.AddedCount != splinter.SourcePlotCount-1 {
		t.Fatalf("unexpected CompareProjects counts: %+v", cmpSplinter)
	}

	// Mutate CoastSplinter's Zone on '00997' and verify Modified FieldDiff is detected!
	vprotest.MutateContextFixture(t, splinter.DestinationPath, `UPDATE CoastSplinter_Env SET Zone = 'MH' WHERE PlotNumber = '00997'`)
	cmpMod, err := projectops.CompareProjects(svc, ctx, state.ContextID, projectops.CompareProjectsRequest{
		TargetPath:    splinter.DestinationPath,
		TargetProject: "CoastSplinter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmpMod.ModifiedCount != 1 {
		t.Fatalf("expected 1 Modified plot, got %+v", cmpMod)
	}
	foundZoneDiff := false
	for _, fd := range cmpMod.FieldDiffs {
		if fd.PlotNumber == "00997" && fd.Table == "Env" && fd.Field == "Zone" &&
			fd.SourceVal != nil && *fd.SourceVal == "CWH" && fd.TargetVal != nil && *fd.TargetVal == "MH" {
			foundZoneDiff = true
		}
	}
	if !foundZoneDiff {
		t.Fatalf("expected Zone CWH vs MH FieldDiff, got %+v", cmpMod.FieldDiffs)
	}

	// Stale contextID must be rejected when switching or creating.
	activatedRes, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:       "SwitchedProj",
		ActivateOnSuccess: true,
	})
	if err != nil || !activatedRes.Activated || activatedRes.State == nil {
		t.Fatalf("CreateNewProject with switch failed: %v", err)
	}
	if _, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName: "ShouldRejectStale",
	}); err == nil {
		t.Fatalf("expected stale contextID to be rejected after context switch")
	}
}

func TestOptimizeVegMaxHeightCap99AndPermanentChildIDNonReuse(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Create clean isolated project and activate it.
	proj, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:       "OptVegProj",
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cid := proj.State.ContextID

	// Seed 1 plot and 3 split rows for PSEUMEN + 2 overlapping rows for TSUGHET exceeding 100% cover.
	vprotest.MutateContextFixture(t, proj.DestinationPath, `
		INSERT INTO OptVegProj_Env (PlotNumber, ProjectID, Zone) VALUES ('P001', 'OV1', 'CWH');
		INSERT INTO OptVegProj_Admin (Plot, PlotType) VALUES ('P001', 'FS882');
		INSERT INTO OptVegProj_Veg (PlotNumber, Species, ID, Cover2, Height2, Cover6, Height6, Flag)
			VALUES ('P001', 'PSEUMEN', 10, 45.0, 28.5, 12.0, 0.4, 0);
		INSERT INTO OptVegProj_Veg (PlotNumber, Species, ID, Cover5, Height5, Cover6, Height6, Collected, Flag)
			VALUES ('P001', 'PSEUMEN', 25, 18.0, 3.2, 8.0, 0.65, 'V', -1);
		INSERT INTO OptVegProj_Veg (PlotNumber, Species, ID, Cover7, Flag)
			VALUES ('P001', 'PSEUMEN', 42, 2.5, 0);
		INSERT INTO OptVegProj_Veg (PlotNumber, Species, ID, Cover4, Height4, HeightB, Flag)
			VALUES ('P001', 'TSUGHET', 50, 65.0, 6.0, '5.5', 0);
		INSERT INTO OptVegProj_Veg (PlotNumber, Species, ID, Cover4, Height4, HeightB, Flag)
			VALUES ('P001', 'TSUGHET', 55, 55.0, 8.2, '8.2', 0);
	`)

	preview, err := projectops.PreviewOptimizeVeg(svc, ctx, cid, projectops.OptimizeVegOptions{})
	if err != nil {
		t.Fatalf("PreviewOptimizeVeg failed: %v", err)
	}
	if preview.BeforeRowCount != 5 || preview.AfterRowCount != 2 || preview.DuplicateSpeciesCount != 2 {
		t.Fatalf("unexpected OptimizeVeg preview counts: %+v", preview)
	}

	// Verify stale digest is rejected without mutating rows.
	if _, err := projectops.ApplyOptimizeVeg(svc, ctx, cid, projectops.ApplyOptimizeVegRequest{
		ExpectedDigest: "stale-digest",
	}); err == nil {
		t.Fatalf("expected stale ExpectedDigest to fail")
	}

	// Cancelled context must roll back cleanly.
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := projectops.ApplyOptimizeVeg(svc, cancelCtx, cid, projectops.ApplyOptimizeVegRequest{
		ExpectedDigest: preview.PreviewDigest,
	}); err == nil {
		t.Fatalf("expected cancelled context to abort ApplyOptimizeVeg")
	}

	applied, err := projectops.ApplyOptimizeVeg(svc, ctx, cid, projectops.ApplyOptimizeVegRequest{
		ExpectedDigest: preview.PreviewDigest,
	})
	if err != nil {
		t.Fatalf("ApplyOptimizeVeg failed: %v", err)
	}
	if applied.ConsolidatedGroups != 2 || applied.DeletedRows != 3 || applied.AfterRowCount != 2 {
		t.Fatalf("unexpected ApplyOptimizeVeg result: %+v", applied)
	}

	// Check PSEUMEN Height6 == MAX(0.4, 0.65) == 0.65 (NOT 1.05 sum!), Cover6 == 20.0, Flag == -1, Collected == 'V'.
	// Check TSUGHET Cover4 capped at 99.0 (65 + 55 = 120 -> 99.0), Height4 == MAX(6.0, 8.2) == 8.2, HeightB == '8.2'.
	vegRows, err := svc.ListVegRecords(ctx, cid, "P001")
	if err != nil {
		t.Fatal(err)
	}
	if len(vegRows) != 2 {
		t.Fatalf("expected 2 consolidated veg rows, got %d", len(vegRows))
	}
	for _, r := range vegRows {
		switch r.Species {
		case "PSEUMEN":
			if r.Height6 == nil || math.Abs(*r.Height6-0.65) > 1e-5 {
				t.Fatalf("expected PSEUMEN Height6 MAX=0.65 (not sum 1.05), got %v", r.Height6)
			}
			if r.Cover6 == nil || math.Abs(*r.Cover6-20.0) > 1e-5 {
				t.Fatalf("expected PSEUMEN Cover6=20.0, got %v", r.Cover6)
			}
			if r.Cover2 == nil || *r.Cover2 != 45.0 || r.Cover5 == nil || *r.Cover5 != 18.0 || r.Cover7 == nil || *r.Cover7 != 2.5 {
				t.Fatalf("unexpected PSEUMEN disjoint covers: %+v", r)
			}
		case "TSUGHET":
			if r.Cover4 == nil || math.Abs(*r.Cover4-99.0) > 1e-5 {
				t.Fatalf("expected TSUGHET Cover4 capped at 99.0, got %v", r.Cover4)
			}
			if r.Height4 == nil || math.Abs(*r.Height4-8.2) > 1e-5 {
				t.Fatalf("expected TSUGHET Height4 MAX=8.2, got %v", r.Height4)
			}
		}
	}

	// Verify permanent child ID non-reuse: retired IDs (25, 42, 55) must be in __VPRO_ChildIdentity
	// and saving a new VegRecord must NOT reuse 25, 42, or 55!
	db, err := sqlutil.OpenReadOnly(proj.DestinationPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, retired := range []int64{25, 42, 55} {
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ID" = ?`, retired).Scan(&cnt); err != nil {
			t.Fatal(err)
		}
		if cnt == 0 {
			t.Fatalf("expected retired ID %d to be reserved in __VPRO_ChildIdentity", retired)
		}
	}
}

func TestThreeWayProjectMergeAndMetadataAutomation(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Create Base.db, Target (active), and Staging.db.
	baseDir := t.TempDir()
	basePath := filepath.Join(baseDir, "BaseProj.db")
	_, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:     "BaseProj",
		DestinationPath: basePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	vprotest.MutateContextFixture(t, basePath, `
		INSERT INTO BaseProj_Env (PlotNumber, ProjectID, Date, FSRegionDistrict, Zone, SubZone, Elevation, SurficialMaterialSurf, SpeciesListComplete)
			VALUES ('0101', 'MRG26', '2024-07-15', 'RCB.DMH', 'SBS', 'dw1', 850, 'Morainal', -1),
			       ('0102', 'MRG26', '2025-08-02', 'RCB.DCC', 'SBS', 'dw2', 910, NULL, 0);
		INSERT INTO BaseProj_Admin (Plot, PlotType, UserSiteUnit, UpdatedFromCards)
			VALUES ('0101', 'FS882', 'SBSdw1/01', 0),
			       ('0102', 'Ground', 'SBSdw2/04', 0);
		INSERT INTO BaseProj_Veg (PlotNumber, Species, ID, Cover2, Flag)
			VALUES ('0101', 'PINUCON', 5, 35.0, 0);
	`)

	// Switch context to BaseProj first, SaveProjectAs TargetProj & StagingProj!
	switchedBase, err := svc.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project: "BaseProj", ProjectPath: basePath, SU: "None", Hierarchy: "None",
	})
	if err != nil {
		t.Fatal(err)
	}

	stagingRes, err := projectops.SaveProjectAs(svc, ctx, switchedBase.ContextID, projectops.SaveProjectAsRequest{
		NewProjectName:    "StagingProj",
		DestinationPath:   filepath.Join(baseDir, "StagingProj.db"),
		ActivateOnSuccess: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	targetRes, err := projectops.SaveProjectAs(svc, ctx, switchedBase.ContextID, projectops.SaveProjectAsRequest{
		NewProjectName:    "TargetProj",
		DestinationPath:   filepath.Join(baseDir, "TargetProj.db"),
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cid := targetRes.State.ContextID

	// Diverge StagingProj:
	// - Plot '0101': Fast-forward Elevation 850 -> 875 (Target unchanged = 850)
	// - Plot '0102': True conflict Zone='ICH' in Staging vs Zone='IDF' in Target (Base had 'SBS')
	// - Plot '0103': Added in Staging with Humus horizon
	vprotest.MutateContextFixture(t, stagingRes.DestinationPath, `
		UPDATE StagingProj_Env SET Elevation = 875 WHERE PlotNumber = '0101';
		UPDATE StagingProj_Env SET Zone = 'ICH', SubZone = 'mk1' WHERE PlotNumber = '0102';
		INSERT INTO StagingProj_Env (PlotNumber, ProjectID, Date, FSRegionDistrict, Zone, Elevation, SpeciesListComplete)
			VALUES ('0103', 'MRG26', '2026-06-20', 'RSK.DSS', 'CWH', 220, -1);
		INSERT INTO StagingProj_Admin (Plot, PlotType, UpdatedFromCards)
			VALUES ('0103', 'FS882', -1);
		INSERT INTO StagingProj_Humus (PlotNumber, Horizon, UpperDepth, LowerDepth, Flag, ID)
			VALUES ('0103', 'Lv', 6.0, 4.0, 0, 1);
	`)
	vprotest.MutateContextFixture(t, targetRes.DestinationPath, `
		UPDATE TargetProj_Env SET Zone = 'IDF' WHERE PlotNumber = '0102';
	`)

	preview, err := projectops.PreviewThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergePreviewRequest{
		SourcePath:    stagingRes.DestinationPath,
		SourceProject: "StagingProj",
		BasePath:      basePath,
		BaseProject:   "BaseProj",
	})
	if err != nil {
		t.Fatalf("PreviewThreeWayMerge failed: %v", err)
	}
	if preview.AddedCount != 1 || preview.FastForwardCount != 1 || preview.ConflictCount != 1 {
		t.Fatalf("expected Added=1, FastForward=1, Conflict=1, got %+v", preview)
	}

	// Resolve 0102::Env::Zone -> keep_target ('IDF'), while 0102::Env::SubZone is FastForward -> 'mk1'!
	mergeRes, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:                stagingRes.DestinationPath,
		SourceProject:             "StagingProj",
		BasePath:                  basePath,
		BaseProject:               "BaseProj",
		ExpectedPreviewDigest:     preview.PreviewDigest,
		DefaultConflictResolution: projectops.MergeResolutionKeepTarget,
		FieldResolutions: map[string]string{
			"0102::Env::Zone": projectops.MergeResolutionKeepTarget,
		},
	})
	if err != nil {
		t.Fatalf("ExecuteThreeWayMerge failed: %v", err)
	}
	if mergeRes.AddedPlots != 1 || mergeRes.UpdatedPlots != 2 || mergeRes.AuditRowsWritten == 0 {
		t.Fatalf("unexpected ExecuteThreeWayMerge result: %+v", mergeRes)
	}

	// Verify merged state in TargetProj.
	p101, err := svc.GetPlot(ctx, cid, "0101")
	if err != nil || p101.Elevation == nil || *p101.Elevation != 875 {
		t.Fatalf("expected fast-forwarded 0101 Elevation=875, got %+v (err=%v)", p101, err)
	}
	p102, err := svc.GetPlot(ctx, cid, "0102")
	if err != nil || p102.Zone == nil || *p102.Zone != "IDF" || p102.SubZone == nil || *p102.SubZone != "mk1" {
		t.Fatalf("expected 0102 Zone=IDF (keep_target) and SubZone=mk1 (fast-forward), got %+v (err=%v)", p102, err)
	}
	p103Humus, err := svc.ListHumusRecords(ctx, cid, "0103")
	if err != nil || len(p103Humus) != 1 || p103Humus[0].ID <= 0 {
		t.Fatalf("expected merged 0103 humus record with allocated ID, got %+v (err=%v)", p103Humus, err)
	}

	// Test Auto-6 DeriveProjectMetadataFromPlots & ApplyProjectMetadataAutomation on TargetProj ('MRG26')!
	presets := projectops.ListProjectMetadataPresets()
	if len(presets) != 4 {
		t.Fatalf("expected 4 built-in presets, got %d", len(presets))
	}
	for _, p := range presets {
		if p.Datum != "2" || len(p.CoverDescriptions) != 15 {
			t.Fatalf("preset %s missing Datum='2' or 15 Cover*Description fields: %+v", p.ID, p)
		}
	}

	derived, err := projectops.DeriveProjectMetadataFromPlots(svc, ctx, cid, "MRG26")
	if err != nil {
		t.Fatalf("DeriveProjectMetadataFromPlots failed: %v", err)
	}
	if derived.TotalPlotsInScope != 3 || derived.StartDate == nil || *derived.StartDate != 2024 ||
		derived.EndDate == nil || *derived.EndDate != 2026 ||
		derived.NumberOfFS882Plots != 2 || derived.NumberOfSiteVisits != 1 ||
		derived.CollectedSite != 1 || derived.CollectedVeg != 2 || derived.CollectedSoil != 2 || derived.CollectedTerrain != 2 ||
		!strings.Contains(derived.AllSpecs, "sha256:") || derived.VListsSHA256Short == "" {
		t.Fatalf("unexpected DerivedProjectMetadata: %+v", derived)
	}

	autoApplied, err := projectops.ApplyProjectMetadataAutomation(svc, ctx, cid, projectops.ApplyProjectMetadataAutomationRequest{
		ProjectID:    "MRG26",
		ApplyDerived: true,
		PresetID:     "CHARS_Arctic",
	})
	if err != nil {
		t.Fatalf("ApplyProjectMetadataAutomation failed: %v", err)
	}
	if autoApplied.MetadataID <= 0 || autoApplied.AuditRowCount == 0 {
		t.Fatalf("unexpected ApplyProjectMetadataAutomation result: %+v", autoApplied)
	}

	// Verify stored metadata values in TargetProj_Metadata.
	tgtDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(targetRes.DestinationPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer tgtDB.Close()
	var startYr, endYr, fs882Cnt, siviCnt int
	var std, datum, b2a string
	if err := tgtDB.QueryRow(`SELECT "StartDate","EndDate","NumberOfFS882Plots","NumberOfSiteVisits","EcosysCollectionStandard","Datum","CoverB2aDescription"
		FROM "TargetProj_Metadata" WHERE "ProjectID" = 'MRG26'`).
		Scan(&startYr, &endYr, &fs882Cnt, &siviCnt, &std, &datum, &b2a); err != nil {
		t.Fatal(err)
	}
	if startYr != 2024 || endYr != 2026 || fs882Cnt != 2 || siviCnt != 1 || std != "CHAR" || datum != "2" || !strings.Contains(b2a, "CHARS") {
		t.Fatalf("unexpected stored metadata row: start=%d end=%d fs882=%d sivi=%d std=%s datum=%s b2a=%s",
			startYr, endYr, fs882Cnt, siviCnt, std, datum, b2a)
	}
}
