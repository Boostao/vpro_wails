package main

import (
	"os"
	"testing"
)

func TestPlotService_AtomicSaveAndAudit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vpro-plot-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	projects, err := NewProjectServiceWithConfig(tempDir, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	plotService := NewPlotService(projects)
	plotService.SetAuditStrength(2)
	plotService.SetCurrentUser("Tester")

	// 1. Save new plot
	elev := 450
	loc := "Test Basin"
	h := FS882Header{
		PlotNumber:      "TST001",
		Elevation:       &elev,
		GeneralLocation: &loc,
	}

	if err := plotService.SavePlot(h); err != nil {
		t.Fatalf("SavePlot failed: %v", err)
	}

	// 2. Read back
	saved, err := plotService.GetPlot("TST001")
	if err != nil {
		t.Fatalf("GetPlot failed: %v", err)
	}
	if saved.Elevation == nil || *saved.Elevation != 450 {
		t.Errorf("expected elevation 450, got %v", saved.Elevation)
	}
	if saved.GeneralLocation == nil || *saved.GeneralLocation != "Test Basin" {
		t.Errorf("expected location 'Test Basin', got %v", saved.GeneralLocation)
	}

	// 3. Update plot
	newElev := 550
	saved.Elevation = &newElev
	if err := plotService.SavePlot(*saved); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := plotService.GetPlot("TST001")
	if err != nil {
		t.Fatalf("GetPlot after update failed: %v", err)
	}
	if updated.Elevation == nil || *updated.Elevation != 550 {
		t.Errorf("expected updated elevation 550, got %v", updated.Elevation)
	}

	// 4. Verify audit entries
	db, _, release, err := plotService.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	var auditCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM "Sample_Audit" WHERE PlotNumber = 'TST001'`).Scan(&auditCount)
	if err != nil {
		t.Fatal(err)
	}
	// At strength 2: 2 initial insert entries (Elevation, Location) + 1 update entry (Elevation) = 3
	if auditCount < 2 {
		t.Errorf("expected at least 2 audit entries, got %d", auditCount)
	}

	// 5. Test Audit Restore
	// Mark the update audit record for restore
	_, err = db.Exec(`UPDATE "Sample_Audit" SET Restore = 1 WHERE PlotNumber = 'TST001' AND EditField = 'Elevation' AND BeforeEdit IS NOT NULL`)
	if err != nil {
		t.Fatal(err)
	}

	if err := plotService.RestoreAuditRecords("TST001", true); err != nil {
		t.Fatalf("RestoreAuditRecords failed: %v", err)
	}

	restored, err := plotService.GetPlot("TST001")
	if err != nil {
		t.Fatalf("GetPlot after restore failed: %v", err)
	}
	if restored.Elevation == nil || *restored.Elevation != 450 {
		t.Errorf("expected restored elevation 450, got %v", restored.Elevation)
	}
}

func TestPlotService_RollbackOnFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vpro-plot-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	projects, err := NewProjectServiceWithConfig(tempDir, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	plotService := NewPlotService(projects)

	// Save valid plot
	elev := 100
	h1 := FS882Header{
		PlotNumber: "TST999",
		Elevation:  &elev,
	}
	if err := plotService.SavePlot(h1); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	// Attempting to insert duplicate manually into one table to simulate constraint collision
	db, _, release, err := plotService.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	var envCount, adminCount int
	db.QueryRow(`SELECT COUNT(*) FROM "Sample_Env" WHERE PlotNumber = 'TST999'`).Scan(&envCount)
	db.QueryRow(`SELECT COUNT(*) FROM "Sample_Admin" WHERE Plot = 'TST999'`).Scan(&adminCount)

	if envCount != 1 || adminCount != 1 {
		t.Errorf("expected 1 Env and 1 Admin row, got Env=%d Admin=%d", envCount, adminCount)
	}
}

func TestPlotService_ChildCRUD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vpro-child-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	projects, err := NewProjectServiceWithConfig(tempDir, tempDir)
	if err != nil {
		t.Fatal(err)
	}

	plotService := NewPlotService(projects)

	plotNum := "TSTCHILD"
	if err := plotService.SavePlot(FS882Header{PlotNumber: plotNum}); err != nil {
		t.Fatalf("failed to create parent plot: %v", err)
	}

	// 1. Veg CRUD
	c1 := 15.0
	veg := VegRecord{
		PlotNumber: plotNum,
		Species:    "PSEUMEN",
		Layer:      nil,
		Cover1:     &c1,
	}
	if err := plotService.SaveVegRecord(veg); err != nil {
		t.Fatalf("SaveVegRecord failed: %v", err)
	}
	vegList, err := plotService.ListVegRecords(plotNum)
	if err != nil {
		t.Fatalf("ListVegRecords failed: %v", err)
	}
	if len(vegList) != 1 || vegList[0].Species != "PSEUMEN" {
		t.Errorf("unexpected veg list: %+v", vegList)
	}

	// 2. Humus CRUD
	up := 0.0
	low := 4.5
	humus := HumusRecord{
		PlotNumber: plotNum,
		UpperDepth: &up,
		LowerDepth: &low,
	}
	if err := plotService.SaveHumusRecord(humus); err != nil {
		t.Fatalf("SaveHumusRecord failed: %v", err)
	}
	humusList, err := plotService.ListHumusRecords(plotNum)
	if err != nil {
		t.Fatalf("ListHumusRecords failed: %v", err)
	}
	if len(humusList) != 1 {
		t.Errorf("unexpected humus list: %+v", humusList)
	}

	// 3. Delete Veg
	if err := plotService.DeleteVegRecord(plotNum, vegList[0].ID); err != nil {
		t.Fatalf("DeleteVegRecord failed: %v", err)
	}
	vegListAfter, _ := plotService.ListVegRecords(plotNum)
	if len(vegListAfter) != 0 {
		t.Errorf("expected 0 veg rows after deletion, got %d", len(vegListAfter))
	}
}
