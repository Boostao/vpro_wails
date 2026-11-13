package projectops_test

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"testing"

	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestPreviewOptimizeVegMemoryScaleAndExactParity(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	proj, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:       "OptScaleProj",
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cid := proj.State.ContextID

	// Bulk-insert 25,000 non-duplicate _Veg rows plus 2 duplicate groups (including leading-zero plot '00337').
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(proj.DestinationPath, "rw")+"&_journal_mode=WAL&_synchronous=OFF")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	envStmt, err := tx.Prepare(`INSERT INTO "OptScaleProj_Env" ("PlotNumber", "ProjectID", "Zone") VALUES (?, 'SCALE', 'CWH')`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	vegStmt, err := tx.Prepare(`INSERT INTO "OptScaleProj_Veg" ("PlotNumber", "Species", "ID", "Cover2", "Height2", "Cover6", "Height6", "Flag") VALUES (?, ?, ?, ?, ?, ?, ?, 0)`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	nextID := int64(1)
	// Leading-zero plot '00337' with 3 duplicate PSEUMEN rows
	if _, err := envStmt.Exec("00337"); err != nil {
		t.Fatal(err)
	}
	for _, c2 := range []float64{20.0, 15.5, 5.0} {
		if _, err := vegStmt.Exec("00337", "PSEUMEN", nextID, c2, 25.0, 4.0, 0.5); err != nil {
			t.Fatal(err)
		}
		nextID++
	}

	// 500 plots x 50 unique species = 25,000 non-duplicate rows, plus 1 extra TSUGHET duplicate on P0250
	for p := 1; p <= 500; p++ {
		plot := fmt.Sprintf("P%04d", p)
		if _, err := envStmt.Exec(plot); err != nil {
			t.Fatal(err)
		}
		for s := 1; s <= 50; s++ {
			spp := fmt.Sprintf("SPP%04d", s)
			if _, err := vegStmt.Exec(plot, spp, nextID, 12.5, 10.0, 2.0, 0.3); err != nil {
				t.Fatal(err)
			}
			nextID++
		}
		if p == 250 {
			if _, err := vegStmt.Exec(plot, "TSUGHET", nextID, 60.0, 18.0, nil, nil); err != nil {
				t.Fatal(err)
			}
			nextID++
			if _, err := vegStmt.Exec(plot, "TSUGHET", nextID, 50.0, 22.0, nil, nil); err != nil {
				t.Fatal(err)
			}
			nextID++
		}
	}
	envStmt.Close()
	vegStmt.Close()
	if err := tx.Commit(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	const expectedBefore = 25005
	const expectedAfter = 25002

	runtime.GC()
	var mBefore, mAfter runtime.MemStats
	runtime.ReadMemStats(&mBefore)

	preview, err := projectops.PreviewOptimizeVeg(svc, ctx, cid, projectops.OptimizeVegOptions{})
	if err != nil {
		t.Fatalf("PreviewOptimizeVeg failed: %v", err)
	}

	runtime.ReadMemStats(&mAfter)
	allocDelta := mAfter.TotalAlloc - mBefore.TotalAlloc

	// Loading full 44-col maps for all 25,005 rows allocates ~140+ MiB; duplicate-only pushdown allocates < 2 MiB!
	const maxAllowedAllocBytes = 10 * 1024 * 1024 // 10 MiB
	if allocDelta > maxAllowedAllocBytes {
		t.Fatalf("PreviewOptimizeVeg allocated %.2f MiB scanning %d rows (must pre-filter duplicates in SQL; limit %.2f MiB)",
			float64(allocDelta)/(1024*1024), expectedBefore, float64(maxAllowedAllocBytes)/(1024*1024))
	}

	if preview.BeforeRowCount != expectedBefore || preview.AfterRowCount != expectedAfter ||
		preview.DuplicateSpeciesCount != 2 || preview.RowsToConsolidate != 3 || len(preview.Groups) != 2 {
		t.Fatalf("unexpected PreviewOptimizeVeg summary: before=%d after=%d dups=%d consolidate=%d groups=%d",
			preview.BeforeRowCount, preview.AfterRowCount, preview.DuplicateSpeciesCount, preview.RowsToConsolidate, len(preview.Groups))
	}
	if preview.Groups[0].PlotNumber != "00337" || preview.Groups[0].Species != "PSEUMEN" ||
		preview.Groups[1].PlotNumber != "P0250" || preview.Groups[1].Species != "TSUGHET" {
		t.Fatalf("unexpected duplicate groups: %+v", preview.Groups)
	}

	// Verify PlotNumbers filter option preserves BeforeRowCount and scopes duplicate consolidation to '00337'.
	filteredPreview, err := projectops.PreviewOptimizeVeg(svc, ctx, cid, projectops.OptimizeVegOptions{
		PlotNumbers: []string{"00337"},
	})
	if err != nil {
		t.Fatalf("filtered PreviewOptimizeVeg failed: %v", err)
	}
	if filteredPreview.BeforeRowCount != expectedBefore || filteredPreview.AfterRowCount != expectedBefore-2 || len(filteredPreview.Groups) != 1 {
		t.Fatalf("unexpected filtered PreviewOptimizeVeg: %+v", filteredPreview)
	}

	// Verify ApplyOptimizeVeg succeeds with preview.PreviewDigest and produces exact AfterRowCount.
	applied, err := projectops.ApplyOptimizeVeg(svc, ctx, cid, projectops.ApplyOptimizeVegRequest{
		ExpectedDigest: preview.PreviewDigest,
	})
	if err != nil {
		t.Fatalf("ApplyOptimizeVeg failed: %v", err)
	}
	if applied.BeforeRowCount != expectedBefore || applied.AfterRowCount != expectedAfter || applied.ConsolidatedGroups != 2 || applied.DeletedRows != 3 {
		t.Fatalf("unexpected ApplyOptimizeVeg result: %+v", applied)
	}
}
