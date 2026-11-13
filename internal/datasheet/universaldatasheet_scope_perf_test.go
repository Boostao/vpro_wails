package datasheet

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestDatasheetScopeToActiveSUTautologyAndVegGlobalSearchPerf(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// 1. Seed '337' alongside existing '00337' and '108050', then scope to a 2-plot SU ('00337', '108050').
	// Before the fix, unqualified "PlotNumber" inside EXISTS bound to __su."PlotNumber", making ScopeToActiveSU=true
	// a WHERE TRUE tautology that returned ALL unscoped rows!
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Env WHERE PlotNumber = '337';
		DELETE FROM Sample_Admin WHERE Plot = '337';
		INSERT INTO Sample_Env (PlotNumber, Zone, SubZone, SiteSeries, Flag) VALUES ('337', 'IDF', 'dk1', '01', 0);
		INSERT INTO Sample_Admin (Plot, PlotType, UpdatedFromCards) VALUES ('337', 'FS882', 0);
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Flag) VALUES ('337', 'PINUPON', 99001, 35.0, 0);

		CREATE TABLE TwoPlot_SU (PlotNumber VARCHAR, SiteUnit VARCHAR);
		CREATE UNIQUE INDEX uidx_TwoPlot_SU_PlotNumber ON TwoPlot_SU(PlotNumber);
		INSERT INTO _table_metadata (table_name, description) VALUES ('TwoPlot_SU', 'VP04');
		INSERT INTO TwoPlot_SU (PlotNumber, SiteUnit) VALUES
			('00337',  'ICHvk2/01'),
			('108050', 'MHmm1/01');
	`)

	sel := vprotest.ContextSelection(state)
	sel.SU = "TwoPlot"
	sel.SUPath = state.ProjectPath
	state, err := service.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext TwoPlot: %v", err)
	}

	envScoped, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey:        "Env",
		ScopeToActiveSU: true,
		Limit:           100,
	})
	if err != nil {
		t.Fatalf("GetDatasheetPage(Env, ScopeToActiveSU=true): %v", err)
	}
	if envScoped.TotalRows != 2 || len(envScoped.Rows) != 2 {
		t.Fatalf("ScopeToActiveSU=true tautology bug: expected 2 scoped Env rows ('00337', '108050'), got totalRows=%d", envScoped.TotalRows)
	}

	adminScoped, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey:        "Admin",
		ScopeToActiveSU: true,
		Limit:           100,
	})
	if err != nil || adminScoped.TotalRows != 2 {
		t.Fatalf("expected 2 scoped Admin rows, got totalRows=%d err=%v", adminScoped.TotalRows, err)
	}

	vegUnscoped, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey:        "Veg",
		ScopeToActiveSU: false,
		Limit:           10,
	})
	if err != nil {
		t.Fatal(err)
	}
	vegScoped, err := GetDatasheetPage(ctx, service, state.ContextID, DatasheetPageRequest{
		TableKey:        "Veg",
		ScopeToActiveSU: true,
		Limit:           100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if vegScoped.TotalRows <= 0 || vegScoped.TotalRows >= vegUnscoped.TotalRows {
		t.Fatalf("expected Veg ScopeToActiveSU=true (%d) to be strictly smaller than unscoped (%d)", vegScoped.TotalRows, vegUnscoped.TotalRows)
	}
	pColIdx := findColIdx(vegScoped.Columns, "PlotNumber")
	for _, r := range vegScoped.Rows {
		p := *r.Cells[pColIdx].Text
		if p != "00337" && p != "108050" {
			t.Fatalf("unscoped plot %q leaked into ScopeToActiveSU=true Veg page", p)
		}
	}

	// Verify integer 337 in external INT_TEXT SU does NOT match text '337' in Env/Veg.
	extSUPath := filepath.Join(t.TempDir(), "DatasheetIntSU.db")
	extDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(extSUPath, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extDB.Exec(`
		CREATE TABLE _table_metadata (table_name TEXT PRIMARY KEY, description TEXT);
		INSERT INTO _table_metadata (table_name, description) VALUES ('IntOnly_SU', 'VP04');
		CREATE TABLE IntOnly_SU (PlotNumber INT_TEXT, SiteUnit VARCHAR);
		INSERT INTO IntOnly_SU (PlotNumber, SiteUnit) VALUES (337, 'BAD_INT');
	`); err != nil {
		extDB.Close()
		t.Fatal(err)
	}
	extDB.Close()

	selInt := vprotest.ContextSelection(state)
	selInt.SU = "IntOnly"
	selInt.SUPath = extSUPath
	stateInt, err := service.SwitchContext(state.ContextID, selInt)
	if err != nil {
		t.Fatal(err)
	}
	intScoped, err := GetDatasheetPage(ctx, service, stateInt.ContextID, DatasheetPageRequest{
		TableKey:        "Env",
		ScopeToActiveSU: true,
		Limit:           50,
	})
	if err != nil || intScoped.TotalRows != 0 {
		t.Fatalf("expected 0 Env rows when SU only contains integer 337, got totalRows=%d err=%v", intScoped.TotalRows, err)
	}

	// 2. GlobalSearch='PSEUMEN' performance benchmark over 1,000,000 _Veg rows (< 400 ms).
	perfSvc, perfState := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, perfState.ProjectPath, `
		PRAGMA foreign_keys = OFF;
		DELETE FROM Sample_Veg;
		WITH RECURSIVE seq(n) AS (
			SELECT 1
			UNION ALL
			SELECT n + 1 FROM seq WHERE n < 1000000
		)
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Cover4, Cover6, Flag)
		SELECT printf('P%05d', ((n - 1) % 50000) + 1),
		       CASE WHEN n % 250 = 0 THEN 'PSEUMEN' ELSE 'TSUGHET' END,
		       n, 25.0, 10.0, 5.0, 0
		FROM seq;
	`)

	t0 := time.Now()
	searchPage, err := GetDatasheetPage(ctx, perfSvc, perfState.ContextID, DatasheetPageRequest{
		TableKey:     "Veg",
		GlobalSearch: "PSEUMEN",
		Offset:       0,
		Limit:        100,
	})
	dur := time.Since(t0)
	if err != nil {
		t.Fatalf("GetDatasheetPage(Veg, GlobalSearch='PSEUMEN') failed: %v", err)
	}
	if searchPage.TotalRows != 4000 || len(searchPage.Rows) != 100 {
		t.Fatalf("expected 4000 PSEUMEN matches and 100 window rows, got total=%d rows=%d", searchPage.TotalRows, len(searchPage.Rows))
	}
	if dur > 400*time.Millisecond {
		t.Fatalf("GetDatasheetPage(Veg, GlobalSearch='PSEUMEN') over 1M rows took %v, want <= 400ms", dur)
	}

	// Verify numeric GlobalSearch ('25') still matches numeric Cover1 cells!
	numPage, err := GetDatasheetPage(ctx, perfSvc, perfState.ContextID, DatasheetPageRequest{
		TableKey:     "Veg",
		GlobalSearch: "25",
		Filters:      []DatasheetFilterSpec{{Column: "PlotNumber", Op: "eq", Value: "P00001"}},
		Offset:       0,
		Limit:        50,
	})
	if err != nil || numPage.TotalRows == 0 {
		t.Fatalf("numeric GlobalSearch='25' failed to match numeric Cover1=25.0: total=%d err=%v", numPage.TotalRows, err)
	}
}
