package classification

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func strPtrTest(s string) *string { return &s }
func int64PtrTest(v int64) *int64 { return &v }

func TestSUExplorerCreateTreeAssignUndoAndInspector(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	bec := vprotest.BecFixture(t)

	// Reserved names 'Sample' and 'None' must be rejected.
	if _, err := CreateSUTable(ctx, service, state.ContextID, CreateSUTableRequest{Name: "Sample"}); err == nil {
		t.Fatal("expected reserved name 'Sample' to be rejected")
	}
	if _, err := CreateSUTable(ctx, service, state.ContextID, CreateSUTableRequest{Name: "None"}); err == nil {
		t.Fatal("expected reserved name 'None' to be rejected")
	}

	// Create a standalone SU table in project .db and activate it.
	created, err := CreateSUTable(ctx, service, state.ContextID, CreateSUTableRequest{
		Name:                    "Working2026",
		Destination:             "project",
		PopulateAllProjectPlots: false,
		Activate:                true,
	})
	if err != nil {
		t.Fatalf("CreateSUTable failed: %v", err)
	}
	if !created.Activated || created.TableName != "Working2026_SU" {
		t.Fatalf("unexpected CreateSUTable result: %+v", created)
	}
	ctxID := created.ContextID

	// Initially Working2026_SU is empty -> all project plots must appear in UnassignedPlots!
	tree0, err := GetSUPlotTree(ctx, service, ctxID, "")
	if err != nil {
		t.Fatalf("GetSUPlotTree initial failed: %v", err)
	}
	if tree0.TotalProject == 0 || tree0.TotalUnassigned != tree0.TotalProject || len(tree0.Units) != 0 {
		t.Fatalf("expected all %d plots unassigned initially, got %+v", tree0.TotalProject, tree0)
	}

	// Assign 2 plots (including leading-zero '00337') to 'CWHvm1/01'
	plotA := "00337"
	plotB := tree0.UnassignedPlots[len(tree0.UnassignedPlots)-1].PlotNumber
	assignRes, err := AssignPlotsToSiteUnit(ctx, service, ctxID, AssignPlotsToSiteUnitRequest{
		PlotNumbers:    []string{plotA, plotB},
		TargetSiteUnit: strPtrTest("CWHvm1/01"),
	})
	if err != nil {
		t.Fatalf("AssignPlotsToSiteUnit failed: %v", err)
	}
	if assignRes.ChangedPlots != 2 || assignRes.HistoryID <= 0 {
		t.Fatalf("expected 2 changed plots and positive historyId, got %+v", assignRes)
	}

	tree1, err := GetSUPlotTree(ctx, service, ctxID, "")
	if err != nil {
		t.Fatalf("GetSUPlotTree after assign failed: %v", err)
	}
	if tree1.TotalAssigned != 2 || len(tree1.Units) != 1 || tree1.Units[0].SiteUnit != "CWHvm1/01" || !tree1.Units[0].MaskValid {
		t.Fatalf("expected 1 CWHvm1/01 bucket with 2 plots, got %+v", tree1.Units)
	}

	// Verify Plot Inspector Summary for leading-zero '00337'
	summary, err := GetPlotInspectorSummary(ctx, service, ctxID, plotA)
	if err != nil {
		t.Fatalf("GetPlotInspectorSummary('00337') failed: %v", err)
	}
	if summary.PlotNumber != "00337" || summary.CurrentSUUnit != "CWHvm1/01" {
		t.Fatalf("unexpected PlotInspectorSummary: %+v", summary)
	}

	// Verify BECdb v13 Unit Picker returns mask-validated units
	choices, err := SearchBECUnitPicker(ctx, service, bec, ctxID, BECUnitPickerRequest{
		Query: "01",
		Limit: 25,
	})
	if err != nil || len(choices) == 0 {
		t.Fatalf("SearchBECUnitPicker failed or empty: %v (len=%d)", err, len(choices))
	}

	// Undo last SU assignment and verify both plots return to UnassignedPlots!
	undoRes, err := UndoSUAssignment(ctx, service, ctxID, assignRes.HistoryID)
	if err != nil || undoRes.ChangedPlots != 2 {
		t.Fatalf("UndoSUAssignment failed: %+v %v", undoRes, err)
	}
	treeAfterUndo, err := GetSUPlotTree(ctx, service, ctxID, "")
	if err != nil || treeAfterUndo.TotalAssigned != 0 || treeAfterUndo.TotalUnassigned != tree0.TotalProject {
		t.Fatalf("expected 0 assigned plots after undo, got %+v %v", treeAfterUndo, err)
	}

	// SaveSUTableAs into an external new .db file
	extFile := filepath.Join(t.TempDir(), "ExportedSU.db")
	savedAs, err := SaveSUTableAs(ctx, service, ctxID, SaveSUTableAsRequest{
		NewName:     "CopiedSU",
		Destination: "new_file",
		FilePath:    extFile,
		Activate:    false,
	})
	if err != nil || savedAs.TableName != "CopiedSU_SU" {
		t.Fatalf("SaveSUTableAs failed: %+v %v", savedAs, err)
	}
}

func TestCreateSUFromEnvFieldsExistingAndNewTableModes(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// 1. Create an existing SU table and manually assign '108050' (which has MH / un / 02 in Sample_Env) to 'CUSTOM/99'
	suRes, err := CreateSUTable(ctx, service, state.ContextID, CreateSUTableRequest{
		Name:        "EnvConcatSU",
		Destination: "project",
		Activate:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctxID := suRes.ContextID

	if _, err := AssignPlotsToSiteUnit(ctx, service, ctxID, AssignPlotsToSiteUnitRequest{
		PlotNumbers:    []string{"108050"},
		TargetSiteUnit: strPtrTest("CUSTOM/99"),
	}); err != nil {
		t.Fatal(err)
	}

	// 2. Dry-run preview of fill_unassigned_only (Zone + SubZone + "/" + SiteSeries)
	preview, err := CreateSUFromEnvFields(ctx, service, ctxID, CreateSUFromEnvFieldsRequest{
		Fields:            []string{"Zone", "SubZone", "SiteSeries"},
		Separator:         "/",
		UseBECSlashFormat: true,
		Mode:              "fill_unassigned_only",
		PreviewOnly:       true,
	})
	if err != nil {
		t.Fatalf("CreateSUFromEnvFields preview failed: %v", err)
	}
	if !preview.PreviewOnly || preview.PreservedCount != 1 {
		t.Fatalf("expected PreservedCount=1 for '108050' in fill_unassigned_only preview, got %+v", preview)
	}

	// 3. Commit fill_unassigned_only into existing EnvConcatSU_SU: '108050' MUST keep 'CUSTOM/99'!
	filled, err := CreateSUFromEnvFields(ctx, service, ctxID, CreateSUFromEnvFieldsRequest{
		Fields:            []string{"Zone", "SubZone", "SiteSeries"},
		Separator:         "/",
		UseBECSlashFormat: true,
		Mode:              "fill_unassigned_only",
	})
	if err != nil {
		t.Fatalf("CreateSUFromEnvFields fill_unassigned_only failed: %v", err)
	}
	if filled.PreservedCount != 1 || filled.InsertedCount == 0 {
		t.Fatalf("unexpected fill_unassigned_only counts: %+v", filled)
	}
	sum108050, err := GetPlotInspectorSummary(ctx, service, ctxID, "108050")
	if err != nil || sum108050.CurrentSUUnit != "CUSTOM/99" {
		t.Fatalf("fill_unassigned_only overwrote existing unit on 108050: got %q (%v)", sum108050.CurrentSUUnit, err)
	}

	// 4. Commit overwrite mode into existing EnvConcatSU_SU: '108050' is now updated to 'MHun/02'!
	overwritten, err := CreateSUFromEnvFields(ctx, service, ctxID, CreateSUFromEnvFieldsRequest{
		Fields:            []string{"Zone", "SubZone", "SiteSeries"},
		Separator:         "/",
		UseBECSlashFormat: true,
		Mode:              "overwrite",
	})
	if err != nil || overwritten.UpdatedCount != 1 {
		t.Fatalf("CreateSUFromEnvFields overwrite failed: %+v %v", overwritten, err)
	}
	sumAfterOverwrite, err := GetPlotInspectorSummary(ctx, service, ctxID, "108050")
	if err != nil || sumAfterOverwrite.CurrentSUUnit != "MHun/02" {
		t.Fatalf("overwrite mode did not update 108050 to MHun/02: got %q (%v)", sumAfterOverwrite.CurrentSUUnit, err)
	}

	// 5. Mode 'new_table' creates a brand new BrandNewEnv_SU table and populates it
	newTblRes, err := CreateSUFromEnvFields(ctx, service, ctxID, CreateSUFromEnvFieldsRequest{
		Fields:          []string{"Zone", "SubZone", "MoistureRegime"},
		FieldSeparators: []string{"", "-"},
		Mode:            "new_table",
		TargetSUName:    "BrandNewEnv",
		Destination:     "project",
		Activate:        true,
	})
	if err != nil || !newTblRes.Activated || newTblRes.InsertedCount == 0 {
		t.Fatalf("CreateSUFromEnvFields new_table failed: %+v %v", newTblRes, err)
	}
}

func TestHierarchyExplorerHybridTreeFlatTableCyclesAndBreakpoints(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// Create and activate a fresh hierarchy table
	created, err := CreateHierarchyTable(ctx, service, state.ContextID, CreateHierarchyTableRequest{
		Name:        "Phylo2026",
		Destination: "project",
		Activate:    true,
	})
	if err != nil || !created.Activated {
		t.Fatalf("CreateHierarchyTable failed: %+v %v", created, err)
	}
	ctxID := created.ContextID

	// Add Class -> Order -> Alliance (with ~ breakpoint) -> SiteSeries
	nClass, err := AddHierarchyNode(ctx, service, ctxID, HierarchyAddNodeRequest{
		Name:  "Vaccinio-Piceetea",
		Level: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	nOrder, err := AddHierarchyNode(ctx, service, ctxID, HierarchyAddNodeRequest{
		Name:     "Piceetalia",
		ParentID: int64PtrTest(nClass.NodeID),
		Level:    4,
	})
	if err != nil {
		t.Fatal(err)
	}
	nAlliance, err := AddHierarchyNode(ctx, service, ctxID, HierarchyAddNodeRequest{
		Name:         "Tsugion heterophyllae",
		ParentID:     int64PtrTest(nOrder.NodeID),
		Level:        6,
		IsBreakpoint: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if nAlliance.NodeName != "~Tsugion heterophyllae" {
		t.Fatalf("expected ~ prefix on breakpoint node, got %q", nAlliance.NodeName)
	}
	nSeries, err := AddHierarchyNode(ctx, service, ctxID, HierarchyAddNodeRequest{
		Name:     "CWHvm1/01",
		ParentID: int64PtrTest(nAlliance.NodeID),
		Level:    11,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Cycle detection: self-parenting and reparenting Class under its own descendant SiteSeries MUST reject!
	if _, err := ReparentHierarchyNode(ctx, service, ctxID, HierarchyReparentNodeRequest{
		NodeID:      nClass.NodeID,
		NewParentID: int64PtrTest(nClass.NodeID),
	}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected self-parent cycle rejection, got %v", err)
	}
	if _, err := ReparentHierarchyNode(ctx, service, ctxID, HierarchyReparentNodeRequest{
		NodeID:      nClass.NodeID,
		NewParentID: int64PtrTest(nSeries.NodeID),
	}); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected ancestor->descendant cycle rejection, got %v", err)
	}

	// Toggle ~ Breakpoint off and back on
	toggledOff, err := ToggleHierarchyBreakpoint(ctx, service, ctxID, nAlliance.NodeID)
	if err != nil || toggledOff.NodeName != "Tsugion heterophyllae" {
		t.Fatalf("ToggleHierarchyBreakpoint off failed: %+v %v", toggledOff, err)
	}
	toggledOn, err := ToggleHierarchyBreakpoint(ctx, service, ctxID, nAlliance.NodeID)
	if err != nil || toggledOn.NodeName != "~Tsugion heterophyllae" {
		t.Fatalf("ToggleHierarchyBreakpoint on failed: %+v %v", toggledOn, err)
	}

	// Verify Hybrid Tree + Wide Flat Phytosociological Rank projection
	view, err := GetHierarchyExplorer(ctx, service, ctxID)
	if err != nil {
		t.Fatalf("GetHierarchyExplorer failed: %v", err)
	}
	if view.Diagnostics.TotalNodes != 4 || view.Diagnostics.BreakpointNodes != 1 || len(view.FlatRankRows) != 1 {
		t.Fatalf("unexpected HierarchyExplorerView: %+v", view)
	}
	row := view.FlatRankRows[0]
	if row.Class != "Vaccinio-Piceetea" || row.Order != "Piceetalia" ||
		row.Alliance != "~Tsugion heterophyllae" || row.SiteSeries != "CWHvm1/01" {
		t.Fatalf("unexpected FlatHierarchyRow: %+v", row)
	}

	// SaveFlatHierarchyRows: add a second SiteSeries + WorkingUnit + Association via wide flat table!
	updatedFlat := []FlatHierarchyRow{
		row,
		{
			Class:       "Vaccinio-Piceetea",
			Order:       "Piceetalia",
			Alliance:    "~Tsugion heterophyllae",
			Association: "Tsugo-Abietetum",
			WorkingUnit: "WU-Moist",
			SiteSeries:  "CWHvm1/05",
		},
	}
	saveFlatRes, err := SaveFlatHierarchyRows(ctx, service, ctxID, SaveFlatHierarchyRequest{Rows: updatedFlat})
	if err != nil {
		t.Fatalf("SaveFlatHierarchyRows failed: %v", err)
	}
	if saveFlatRes.AffectedRows != 7 {
		t.Fatalf("expected 7 distinct hierarchy nodes after flat sync, got %d", saveFlatRes.AffectedRows)
	}

	// Conflicting parent in flat table must reject cleanly without corrupting tree!
	if _, err := SaveFlatHierarchyRows(ctx, service, ctxID, SaveFlatHierarchyRequest{
		Rows: []FlatHierarchyRow{
			{Class: "ClassA", Order: "SharedOrder", SiteSeries: "U1"},
			{Class: "ClassB", Order: "SharedOrder", SiteSeries: "U2"},
		},
	}); err == nil || !strings.Contains(err.Error(), "conflicting parents") {
		t.Fatalf("expected conflicting parents error in SaveFlatHierarchyRows, got %v", err)
	}

	// Delete node and verify monotonic __VPRO_HierarchyIdentity prevents deleted ID reuse!
	delID := nSeries.NodeID
	if _, err := DeleteHierarchyNode(ctx, service, ctxID, HierarchyDeleteNodeRequest{
		NodeID:          delID,
		CascadeChildren: true,
	}); err != nil {
		t.Fatalf("DeleteHierarchyNode failed: %v", err)
	}
	addedAfterDel, err := AddHierarchyNode(ctx, service, ctxID, HierarchyAddNodeRequest{
		Name:     "CWHvm1/07",
		ParentID: int64PtrTest(nAlliance.NodeID),
		Level:    11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if addedAfterDel.NodeID <= delID {
		t.Fatalf("deleted hierarchy ID %d was reused or not monotonic (got %d)", delID, addedAfterDel.NodeID)
	}
}
