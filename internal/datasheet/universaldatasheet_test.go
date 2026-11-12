package datasheet

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func strPtr(s string) *string    { return &s }
func realPtr(f float64) *float64 { return &f }

func textCell(s string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &s}
}

func intCell(s string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &s}
}

func realCell(f float64) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "real", Real: &f}
}

func nullCell() vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "null"}
}

func findColIdx(cols []DatasheetColumnMeta, name string) int {
	for i, c := range cols {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

func TestUniversalDatasheetAllNineTablesAndLiteralPlotLookup(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// Activate Sample SU and Sample Hierarchy so all 9 canonical tables are active.
	sel := vprotest.ContextSelection(state)
	sel.SU = "Sample"
	sel.SUPath = state.ProjectPath
	sel.Hierarchy = "Sample"
	sel.HierarchyPath = state.ProjectPath
	state, err := service.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext failed: %v", err)
	}

	// Sample.db already contains leading-zero plot '00337'; also insert '00987' to test literal BLOB lookup.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO Sample_Env (PlotNumber, Zone, SubZone, SiteSeries, Elevation, Flag) VALUES ('00987', 'CWH', 'vm1', '01', 420, 0);
		INSERT INTO Sample_Admin (Plot, PlotType, UserSiteUnit, UpdatedFromCards) VALUES ('00987', 'FS882', 'CWHvm1/01', 0);
		INSERT INTO Sample_SU (PlotNumber, SiteUnit) VALUES ('00987', 'CWHvm1/01');
	`)

	tables, err := ListDatasheetTables(ctx, service, state.ContextID)
	if err != nil {
		t.Fatalf("ListDatasheetTables failed: %v", err)
	}
	if len(tables) != 9 {
		t.Fatalf("expected 9 canonical datasheet tables, got %d", len(tables))
	}
	for _, tbl := range tables {
		if !tbl.Available {
			t.Fatalf("expected table %s (%s) to be available, reason=%q", tbl.Key, tbl.TableName, tbl.UnavailableReason)
		}
		page, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
			TableKey: tbl.Key,
			Offset:   0,
			Limit:    10,
		})
		if err != nil {
			t.Fatalf("GetDatasheetPage(%s) failed: %v", tbl.Key, err)
		}
		if page.TotalRows != tbl.RowCount {
			t.Fatalf("table %s totalRows=%d != listed rowCount=%d", tbl.Key, page.TotalRows, tbl.RowCount)
		}
	}

	// Verify literal leading-zero lookup on '00337' vs '337'
	eq00337, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Env",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "00337"}},
	})
	if err != nil {
		t.Fatalf("GetDatasheetPage literal 00337 failed: %v", err)
	}
	if eq00337.TotalRows != 1 || len(eq00337.Rows) != 1 {
		t.Fatalf("expected exactly 1 row for literal '00337', got %d", eq00337.TotalRows)
	}

	eq337, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Env",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "337"}},
	})
	if err != nil {
		t.Fatalf("GetDatasheetPage literal 337 failed: %v", err)
	}
	if eq337.TotalRows != 0 {
		t.Fatalf("expected 0 rows for '337' (must not coerce '00337'), got %d", eq337.TotalRows)
	}

	// Verify multi-column deterministic sort + pagination
	sorted1, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Env",
		Offset:   0,
		Limit:    5,
		Sort: []DatasheetSortSpec{
			{Column: "Zone", Direction: "asc"},
			{Column: "Elevation", Direction: "desc"},
		},
	})
	if err != nil || len(sorted1.Rows) != 5 {
		t.Fatalf("multi-column sort page 1 failed: %v (rows=%d)", err, len(sorted1.Rows))
	}
}

func TestApplyDatasheetEditsValidationAuditAndAtomicity(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	service.Plots.SetCurrentUser("AnalystJane")

	// Seed a plot with an out-of-rule historical column (Zone = 'TOO_LONG_LEGACY_ZONE' > 4 chars)
	// plus child Veg, Humus, Mineral rows to test historical tolerance, float32 cover narrowing,
	// UTF-16 widths, Access BOOLEAN -1/0, BECSiteUnit lock, LMH25 soil depth signs, and atomic rollback.
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		INSERT INTO Sample_Env (PlotNumber, Zone, SubZone, SiteSeries, MoistureRegime, SpeciesListComplete, Flag)
		VALUES ('DS001', 'LEGACY_OVER_4_CHARS', 'vm1', '01', '4', 0, 0);
		INSERT INTO Sample_Admin (Plot, PlotType, UserSiteUnit, BECSiteUnit, UpdatedFromCards)
		VALUES ('DS001', 'FS882', 'CWHvm1/01', 'CWHvm1/01', 0);
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Cover6, Flag)
		VALUES ('DS001', 'TSUGHET', 9001, 25.0, 10.5, 0);
		INSERT INTO Sample_Humus (ID, PlotNumber, Horizon, UpperDepth, LowerDepth, Flag)
		VALUES (9101, 'DS001', 'Lv', 6.0, 2.0, 0);
		INSERT INTO Sample_Mineral (ID, PlotNumber, Horizon, UpperDepth, LowerDepth, Texture, Colour, Flag)
		VALUES (9201, 'DS001', 'Bf', 0.0, 25.0, 'SL', '10YR 4/4', 0);
	`)

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	countAudits := func() int {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber = 'DS001'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// 1. Historical field tolerance + Access BOOLEAN (-1/0) on _Env:
	// Zone has 'LEGACY_OVER_4_CHARS', which we do NOT touch; editing MoistureRegime & SpeciesListComplete must succeed!
	envPage, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Env",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "DS001"}},
	})
	if err != nil || len(envPage.Rows) != 1 {
		t.Fatalf("load DS001 Env row failed: %v", err)
	}
	envRow := envPage.Rows[0]
	mrIdx := findColIdx(envPage.Columns, "MoistureRegime")
	slcIdx := findColIdx(envPage.Columns, "SpeciesListComplete")

	res, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Env",
		Edits: []DatasheetCellEdit{
			{RowID: envRow.RowID, Column: "MoistureRegime", Before: envRow.Cells[mrIdx], After: textCell("5")},
			{RowID: envRow.RowID, Column: "SpeciesListComplete", Before: envRow.Cells[slcIdx], After: textCell("true")},
		},
	})
	if err != nil {
		t.Fatalf("ApplyDatasheetEdits on Env failed despite historical tolerance: %v", err)
	}
	if res.UpdatedRows != 1 || res.UpdatedCells != 2 || res.AuditRows != 2 {
		t.Fatalf("expected 1 row, 2 cells, 2 audit rows; got %+v", res)
	}

	var storedBool int
	var storedZone string
	if err := db.QueryRow(`SELECT CAST(SpeciesListComplete AS INTEGER), Zone FROM Sample_Env WHERE PlotNumber = 'DS001'`).Scan(&storedBool, &storedZone); err != nil {
		t.Fatal(err)
	}
	if storedBool != -1 {
		t.Fatalf("Access BOOLEAN true must store -1 in SQLite, got %d", storedBool)
	}
	if storedZone != "LEGACY_OVER_4_CHARS" {
		t.Fatalf("untouched historical Zone was altered: %q", storedZone)
	}
	var auditAfter string
	if err := db.QueryRow(`SELECT AfterEdit FROM Sample_Audit WHERE PlotNumber = 'DS001' AND EditField = 'SpeciesListComplete'`).Scan(&auditAfter); err != nil {
		t.Fatal(err)
	}
	if auditAfter != "-1" {
		t.Fatalf("expected Access BOOLEAN audit AfterEdit='-1', got %q", auditAfter)
	}

	// 2. IEEE-754 float32 cover narrowing on _Veg:
	// 99.999996 narrows to float32 99.99999237060547 (< 100) -> PASSES!
	// 99.999999 narrows to float32 100 (>= 100) -> REJECTS!
	vegPage, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Veg",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "DS001"}},
	})
	if err != nil || len(vegPage.Rows) != 1 {
		t.Fatalf("load DS001 Veg row failed: %v", err)
	}
	vegRow := vegPage.Rows[0]
	c1Idx := findColIdx(vegPage.Columns, "Cover1")
	sppIdx := findColIdx(vegPage.Columns, "Species")

	for _, badCover := range []float64{99.999999, math.Nextafter(100, math.Inf(-1)), 100.0, 105.0, -0.1} {
		_, err = ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
			TableKey: "Veg",
			Edits: []DatasheetCellEdit{
				{RowID: vegRow.RowID, Column: "Cover1", Before: vegRow.Cells[c1Idx], After: realCell(badCover)},
			},
		})
		if err == nil {
			t.Fatalf("expected cover %v to be rejected by float32 <100 narrowing", badCover)
		}
	}

	// Valid narrowed cover 99.999996 must pass and store float64(float32(99.999996)) = 99.99999237060547!
	res, err = ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Veg",
		Edits: []DatasheetCellEdit{
			{RowID: vegRow.RowID, Column: "Cover1", Before: vegRow.Cells[c1Idx], After: realCell(99.999996)},
		},
	})
	if err != nil {
		t.Fatalf("expected cover 99.999996 to pass float32 narrowing (<100), got error: %v", err)
	}
	var storedCover float64
	if err := db.QueryRow(`SELECT Cover1 FROM Sample_Veg WHERE PlotNumber = 'DS001' AND ID = 9001`).Scan(&storedCover); err != nil {
		t.Fatal(err)
	}
	if storedCover != float64(float32(99.999996)) {
		t.Fatalf("expected stored narrowed float32 99.99999237060547, got %.17g", storedCover)
	}

	// 3. UTF-16 code unit bounds (Species <= 8, Horizon <= 8, Texture <= 4, Colour <= 14) + atomic batch rollback!
	priorAuditCount := countAudits()
	vegPage, _ = GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Veg",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "DS001"}},
	})
	vegRow = vegPage.Rows[0]
	c6Idx := findColIdx(vegPage.Columns, "Cover6")
	// Batch with 1 valid edit (Cover6 = 42.0) and 1 invalid edit (Species = 9 chars > 8 UTF-16 units) MUST roll back completely!
	_, err = ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Veg",
		Edits: []DatasheetCellEdit{
			{RowID: vegRow.RowID, Column: "Cover6", Before: vegRow.Cells[c6Idx], After: realCell(42.0)},
			{RowID: vegRow.RowID, Column: "Species", Before: vegRow.Cells[sppIdx], After: textCell("ABCDEFGHI")},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "UTF-16") {
		t.Fatalf("expected UTF-16 Species > 8 error, got %v", err)
	}
	var cover6AfterRollback float64
	if err := db.QueryRow(`SELECT Cover6 FROM Sample_Veg WHERE PlotNumber = 'DS001' AND ID = 9001`).Scan(&cover6AfterRollback); err != nil {
		t.Fatal(err)
	}
	if cover6AfterRollback != 10.5 || countAudits() != priorAuditCount {
		t.Fatalf("atomic BEGIN IMMEDIATE rollback failed: Cover6=%v audits=%d (expected 10.5, %d)", cover6AfterRollback, countAudits(), priorAuditCount)
	}

	// Mineral Texture <= 4 and Colour <= 14 UTF-16 checks (including astral rune = 2 UTF-16 code units!)
	minPage, _ := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Mineral",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "DS001"}},
	})
	minRow := minPage.Rows[0]
	texIdx := findColIdx(minPage.Columns, "Texture")
	colIdx := findColIdx(minPage.Columns, "Colour")
	// "AB\U0001D306D" is 4 runes, 5 UTF-16 code units -> must reject Texture <= 4!
	if _, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Mineral",
		Edits:    []DatasheetCellEdit{{RowID: minRow.RowID, Column: "Texture", Before: minRow.Cells[texIdx], After: textCell("AB\U0001D306D")}},
	}); err == nil {
		t.Fatal("expected astral character exceeding 4 UTF-16 code units on Texture to reject")
	}
	if _, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Mineral",
		Edits:    []DatasheetCellEdit{{RowID: minRow.RowID, Column: "Colour", Before: minRow.Cells[colIdx], After: textCell("123456789012345")}},
	}); err == nil {
		t.Fatal("expected 15-unit Colour (>14) to reject")
	}

	// 4. Soil Horizon LMH25 depth sign check: Upland Lv requires UpperDepth > LowerDepth!
	humPage, _ := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Humus",
		Filters:  []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "DS001"}},
	})
	humRow := humPage.Rows[0]
	uIdx := findColIdx(humPage.Columns, "UpperDepth")
	if _, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Humus",
		Edits:    []DatasheetCellEdit{{RowID: humRow.RowID, Column: "UpperDepth", Before: humRow.Cells[uIdx], After: realCell(1.0)}},
	}); err == nil || !strings.Contains(err.Error(), "UpperDepth > LowerDepth") {
		t.Fatalf("expected upland Lv horizon depth inversion error, got %v", err)
	}

	// 5. Protected BECSiteUnit: rejected for AnalystJane, allowed for Will MacKenzie!
	admPage, _ := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey: "Admin",
		Filters:  []DatasheetFilterSpec{{Column: "Plot", Op: "eq", Value: "DS001"}},
	})
	admRow := admPage.Rows[0]
	becIdx := findColIdx(admPage.Columns, "BECSiteUnit")
	if _, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Admin",
		Edits:    []DatasheetCellEdit{{RowID: admRow.RowID, Column: "BECSiteUnit", Before: admRow.Cells[becIdx], After: textCell("CWHvm1/05")}},
	}); err == nil {
		t.Fatal("expected BECSiteUnit edit to be blocked for non-custodian user")
	}

	service.Plots.SetCurrentUser("Will MacKenzie")
	if _, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Admin",
		Edits:    []DatasheetCellEdit{{RowID: admRow.RowID, Column: "BECSiteUnit", Before: admRow.Cells[becIdx], After: textCell("CWHvm1/05")}},
	}); err != nil {
		t.Fatalf("expected BECSiteUnit edit to succeed for Will MacKenzie, got %v", err)
	}

	// 6. Column-scoped Find & Replace preview + apply round-trip!
	preview, err := PreviewDatasheetFindReplace(ctx, service, state.ContextID, DatasheetFindReplaceRequest{
		TableKey:    "Admin",
		Column:      "UserSiteUnit",
		FindText:    "CWHvm1/01",
		ReplaceText: "CWHvm1/06",
		MatchCase:   false,
		ExactCell:   false,
		Filters:     []DatasheetFilterSpec{{Column: "Plot", Op: "eq", Value: "DS001"}},
	})
	if err != nil {
		t.Fatalf("PreviewDatasheetFindReplace failed: %v", err)
	}
	if preview.MatchedRows != 1 || preview.ValidCount != 1 || preview.InvalidCount != 0 || len(preview.Items) != 1 {
		t.Fatalf("unexpected Find/Replace preview: %+v", preview)
	}
	frRes, err := ApplyDatasheetEdits(ctx, service, state.ContextID, DatasheetEditRequest{
		TableKey: "Admin",
		Edits:    []DatasheetCellEdit{preview.Items[0].Edit},
	})
	if err != nil || frRes.UpdatedCells != 1 || frRes.AuditRows != 1 {
		t.Fatalf("ApplyDatasheetEdits from Find/Replace preview failed: %+v %v", frRes, err)
	}
}
