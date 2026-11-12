package lumping_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/lumping"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestLumpCRUDValidationSaveAsAndImpactPreview(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed controlled vegetation rows in Sample_Veg to test 99% cover cap and multi-species folding
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		DELETE FROM Sample_Veg WHERE PlotNumber='108050';
		INSERT INTO Sample_Veg (PlotNumber, ID, Species, Cover4, Cover6) VALUES
			('108050', 9001, 'SALIBEB', 62.5, NULL),
			('108050', 9002, 'SALISCO', 55.0, NULL),
			('108050', 9003, 'PSEUMEN1', NULL, 30.0),
			('108050', 9004, 'PSEUMEN2', NULL, 25.0);
	`)

	created, err := lumping.CreateLumpTable(service, ctx, state.ContextID, "TestCorr")
	if err != nil {
		t.Fatalf("CreateLumpTable failed: %v", err)
	}
	if created.TableName != "TestCorr_Lump" || created.MetadataVersion != "VP05" {
		t.Fatalf("unexpected created lump summary: %+v", created)
	}

	// Verify _table_metadata has 'VP05'
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var metaDesc string
	if err := db.QueryRow(`SELECT description FROM _table_metadata WHERE table_name='TestCorr_Lump'`).Scan(&metaDesc); err != nil {
		db.Close()
		t.Fatalf("missing _table_metadata entry: %v", err)
	}
	db.Close()
	if metaDesc != "VP05" {
		t.Fatalf("expected metadata VP05, got %q", metaDesc)
	}

	// Reject unknown species when AllowUnlisted is false
	if _, err := lumping.UpsertLumpRow(service, ctx, state.ContextID, "TestCorr", lumping.LumpRowInput{
		LumpCode: "SALIX",
		SppCode:  "ZZZZZZZ",
		Use:      true,
	}); err == nil {
		t.Fatal("expected unknown species code ZZZZZZZ to be rejected")
	}

	// Reject UTF-16 length overflow (SppCode > 8, LumpCode > 50)
	if _, err := lumping.UpsertLumpRow(service, ctx, state.ContextID, "TestCorr", lumping.LumpRowInput{
		LumpCode: "SALIX",
		SppCode:  "ABCDEFGHI",
		Use:      true,
	}); err == nil {
		t.Fatal("expected 9-char SppCode to be rejected")
	}

	// Add valid rows and verify Access BOOLEAN -1 / 0 storage
	r1, err := lumping.UpsertLumpRow(service, ctx, state.ContextID, "TestCorr", lumping.LumpRowInput{
		LumpCode: "SALIX",
		SppCode:  "SALIBEB",
		Use:      true,
	})
	if err != nil {
		t.Fatalf("UpsertLumpRow SALIBEB: %v", err)
	}
	if !r1.Use || r1.SourceCatalog != "VLists" {
		t.Fatalf("unexpected row metadata: %+v", r1)
	}

	if _, err := lumping.UpsertLumpRow(service, ctx, state.ContextID, "TestCorr", lumping.LumpRowInput{
		LumpCode: "SALIX",
		SppCode:  "SALISCO",
		Use:      true,
	}); err != nil {
		t.Fatalf("UpsertLumpRow SALISCO: %v", err)
	}

	// Verify physical SQLite storage uses -1 for Use=true
	dbCheck, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	var rawUse int
	if err := dbCheck.QueryRow(`SELECT CAST("Use" AS INTEGER) FROM TestCorr_Lump WHERE SppCode='SALIBEB'`).Scan(&rawUse); err != nil {
		dbCheck.Close()
		t.Fatal(err)
	}
	dbCheck.Close()
	if rawUse != -1 {
		t.Fatalf("expected Access BOOLEAN -1 in SQLite, got %d", rawUse)
	}

	// Preview impact: SALIBEB (62.5) + SALISCO (55.0) = 117.5 -> capped at 99.0!
	impact, err := lumping.PreviewLumpImpact(service, ctx, state.ContextID, "TestCorr_Lump", "")
	if err != nil {
		t.Fatalf("PreviewLumpImpact: %v", err)
	}
	if impact.CappedCells < 1 || impact.MergedCells < 1 {
		t.Fatalf("expected merged and 99%%-capped cells in impact preview, got %+v", impact)
	}
	foundSalix := false
	for _, g := range impact.Groups {
		if g.LumpCode == "SALIX" {
			foundSalix = true
			if g.MaxFoldedCover != 99.0 || g.CappedCells < 1 {
				t.Fatalf("expected SALIX MaxFoldedCover=99.0 and CappedCells>=1, got %+v", g)
			}
		}
	}
	if !foundSalix {
		t.Fatalf("expected SALIX group in impact preview, got %+v", impact.Groups)
	}

	// Toggle SALISCO off (Use = 0) and verify physical storage is 0 and cap no longer triggers
	toggled, err := lumping.ToggleLumpRow(service, ctx, state.ContextID, "TestCorr", "SALISCO", false)
	if err != nil {
		t.Fatalf("ToggleLumpRow: %v", err)
	}
	if toggled.Use {
		t.Fatalf("expected Use=false after toggle")
	}
	dbZero, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := dbZero.QueryRow(`SELECT CAST("Use" AS INTEGER) FROM TestCorr_Lump WHERE SppCode='SALISCO'`).Scan(&rawUse); err != nil {
		dbZero.Close()
		t.Fatal(err)
	}
	dbZero.Close()
	if rawUse != 0 {
		t.Fatalf("expected Access BOOLEAN 0 after toggle false, got %d", rawUse)
	}

	// SaveAs external & local discovery
	extPath := filepath.Join(t.TempDir(), "ExternalLumps.db")
	extDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(extPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extDB.Exec(`
		CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT);
		INSERT INTO "_table_metadata" VALUES ('Regional_Lump', 'VP05');
		CREATE TABLE "Regional_Lump" ("LumpCode" VARCHAR(50), "SppCode" VARCHAR(8) UNIQUE, "Use" BOOLEAN);
		INSERT INTO "Regional_Lump" VALUES ('PSEUMEN', 'PSEUMEN1', -1), ('PSEUMEN', 'PSEUMEN2', -1);
	`); err != nil {
		extDB.Close()
		t.Fatal(err)
	}
	extDB.Close()

	discovered, err := lumping.DiscoverLumpTables(service, ctx, state.ContextID, []string{extPath})
	if err != nil {
		t.Fatalf("DiscoverLumpTables: %v", err)
	}
	hasLocal, hasExternal := false, false
	for _, d := range discovered {
		if d.TableName == "TestCorr_Lump" && d.IsActiveProject {
			hasLocal = true
		}
		if d.TableName == "Regional_Lump" && !d.IsActiveProject {
			hasExternal = true
		}
	}
	if !hasLocal || !hasExternal {
		t.Fatalf("expected both local TestCorr_Lump and external Regional_Lump, got %+v", discovered)
	}

	copied, err := lumping.SaveAsLumpTable(service, ctx, state.ContextID, "Regional_Lump", extPath, "CopiedReg")
	if err != nil {
		t.Fatalf("SaveAsLumpTable: %v", err)
	}
	if copied.RowCount != 2 || copied.ActiveCount != 2 || copied.TableName != "CopiedReg_Lump" {
		t.Fatalf("unexpected SaveAs summary: %+v", copied)
	}

	// Update and Delete row
	if _, err := lumping.UpdateLumpRow(service, ctx, state.ContextID, "CopiedReg", "PSEUMEN2", lumping.LumpRowInput{
		LumpCode: "PSEUMEN",
		SppCode:  "SALIBEB",
		Use:      true,
	}); err != nil {
		t.Fatalf("UpdateLumpRow: %v", err)
	}
	if err := lumping.DeleteLumpRow(service, ctx, state.ContextID, "CopiedReg", "SALIBEB"); err != nil {
		t.Fatalf("DeleteLumpRow: %v", err)
	}
	rowsAfter, err := lumping.GetLumpTableRows(service, ctx, state.ContextID, "CopiedReg", "")
	if err != nil || len(rowsAfter) != 1 {
		t.Fatalf("expected 1 row after delete, got %d (err=%v)", len(rowsAfter), err)
	}
}

func TestLumpSpecCompilerPresetAndMaterialize(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	// Seed infraspecific (ABIEMAG1), Codetype='D' seedling/dead (ABIEAMAD, ABIEAMAX), Carex, Sphagnum, and liverwort (lifeform 10)
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		DELETE FROM Sample_Veg WHERE PlotNumber='108050';
		INSERT INTO Sample_Veg (PlotNumber, ID, Species, Cover2, Cover6, Cover7) VALUES
			('108050', 9101, 'ABIEMAG1', 20.0, NULL, NULL),
			('108050', 9102, 'ABIEAMAD', NULL, NULL, 5.0),
			('108050', 9103, 'ABIEAMAX', 15.0, NULL, NULL),
			('108050', 9104, 'CARESIT', NULL, 40.0, NULL),
			('108050', 9105, 'CAREAQU', NULL, 35.0, NULL),
			('108050', 9106, 'SPHACAP', NULL, NULL, 60.0),
			('108050', 9107, 'PTILCIL', NULL, NULL, 12.0);
	`)

	presets := lumping.ListLumpPresets()
	if len(presets) == 0 || presets[0].ID != lumping.PresetBECProvincialCorrelation2026 {
		t.Fatalf("expected BEC_Provincial_Correlation_2026 preset, got %+v", presets)
	}

	preview, err := lumping.PreviewLumpSpec(service, ctx, state.ContextID, lumping.LumpSpec{
		Preset:                lumping.PresetBECProvincialCorrelation2026,
		NonVascularLifeforms:  []int{10},
		ScopeToProjectSpecies: true,
	})
	if err != nil {
		t.Fatalf("PreviewLumpSpec failed: %v", err)
	}

	bySpp := map[string]lumping.LumpRow{}
	for _, r := range preview.Rows {
		bySpp[r.SppCode] = r
	}

	expectMap := map[string]string{
		"ABIEMAG1": "ABIEMAG",
		"ABIEAMAD": "ABIEAMA",
		"ABIEAMAX": "ABIEAMA",
		"CARESIT":  "CAREX",
		"CAREAQU":  "CAREX",
		"SPHACAP":  "SPHAGNUM",
	}
	for spp, expectedLump := range expectMap {
		row, ok := bySpp[spp]
		if !ok {
			t.Fatalf("expected %s in compiled LumpSpec rows, got %+v", spp, preview.Rows)
		}
		if row.LumpCode != expectedLump {
			t.Fatalf("for %s expected LumpCode=%s, got %s (origin=%s)", spp, expectedLump, row.LumpCode, row.RuleOrigin)
		}
	}
	// Check liverwort PTILCIL (Ptilidium ciliare, Lifeform 10) rolled up to genus
	if r, ok := bySpp["PTILCIL"]; !ok || !strings.Contains(r.RuleOrigin, "lifeform_10_genus") {
		t.Fatalf("expected PTILCIL lifeform 10 genus rollup, got %+v", bySpp["PTILCIL"])
	}

	// Materialize into Provincial2026_Lump
	mat, err := lumping.MaterializeLumpSpec(service, ctx, state.ContextID, "Provincial2026", lumping.LumpSpec{
		Preset:                lumping.PresetBECProvincialCorrelation2026,
		ScopeToProjectSpecies: true,
	}, false)
	if err != nil {
		t.Fatalf("MaterializeLumpSpec: %v", err)
	}
	if mat.Table.TableName != "Provincial2026_Lump" || mat.RowsWritten < 6 {
		t.Fatalf("unexpected materialization result: %+v", mat)
	}
}
