package rsidecar_test

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/boostao/vpro-wails/internal/rsidecar"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestRSidecarReadOnlyCallsDoNotMutateProjectDatabase(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	beforeBytes, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}

	assertUntouched := func(step string, expected []byte) {
		t.Helper()
		afterBytes, err := os.ReadFile(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(beforeBytes, afterBytes) && expected == nil {
			t.Fatalf("%s mutated untouched project .db bytes on disk", step)
		}
		if expected != nil && !bytes.Equal(expected, afterBytes) {
			t.Fatalf("%s mutated DELETE-mode project .db bytes on disk", step)
		}
		if _, walErr := os.Stat(state.ProjectPath + "-wal"); !os.IsNotExist(walErr) {
			t.Fatalf("%s created unexpected WAL file %s-wal", step, state.ProjectPath)
		}
	}

	// 1. Cold read-only GetCachedPlotClimate on untouched project .db MUST NOT open mode=rw+WAL!
	rows, err := rsidecar.GetCachedPlotClimate(ctx, svc, state.ContextID, "1961_1990")
	if err != nil {
		t.Fatalf("GetCachedPlotClimate failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 cached rows on cold project, got %d", len(rows))
	}
	assertUntouched("GetCachedPlotClimate (cold)", beforeBytes)

	// 2. Seed __VPRO_PlotClimate for plot '00337' in DELETE journal mode, then verify
	// GetCachedPlotClimate AND 100%-cache-hit ComputePlotClimateAndDualEdatopic AND
	// non-materializing RunMultivariateClustering leave the .db file 100% byte-identical!
	seedDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw")+"&_journal_mode=DELETE")
	if err != nil {
		t.Fatal(err)
	}
	hash := rsidecar.ComputeCoordElevHash(54.7804, 127.1743, 520, "1961_1990")
	_, err = seedDB.Exec(`
		UPDATE "Sample_Env"
		   SET "Zone"='SBS', "SubZone"='dk', "Latitude"=54.7804, "Longitude"=127.1743,
		       "Elevation"=520, "SlopeGradient"=10, "Aspect"=180, "MoistureRegime"='4', "NutrientRegime"='C'
		 WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST('00337' AS BLOB);
		CREATE TABLE "__VPRO_PlotClimate" (
			"PlotNumber" TEXT PRIMARY KEY,
			"CoordElevHash" TEXT NOT NULL,
			"RefPeriod" TEXT NOT NULL,
			"Source" TEXT NOT NULL,
			"MAT" REAL, "MWMT" REAL, "MCMT" REAL, "MAP" REAL, "MSP" REAL, "PAS" REAL, "CMD" REAL,
			"PPT_05" REAL, "PPT_06" REAL, "PPT_07" REAL, "PPT_08" REAL, "PPT_09" REAL,
			"PPT_at" REAL, "PPT_wt" REAL, "DD5" REAL, "NFFD" REAL,
			"ComputedAt" TEXT NOT NULL
		);
		INSERT INTO "__VPRO_PlotClimate" VALUES (
			'00337', ?, '1961_1990', 'bgc_lapse_model',
			3.2, 14.5, -9.8, 520, 240, 180, 210,
			40, 55, 50, 45, 50, 130, 150, 1100, 115, '2026-06-01T00:00:00Z'
		);
	`, hash)
	if err != nil {
		seedDB.Close()
		t.Fatal(err)
	}
	seedDB.Close()

	seededBytes, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}

	cachedRows, err := rsidecar.GetCachedPlotClimate(ctx, svc, state.ContextID, "1961_1990")
	if err != nil || len(cachedRows) != 1 || cachedRows[0].PlotNumber != "00337" {
		t.Fatalf("GetCachedPlotClimate seeded mismatch: rows=%+v err=%v", cachedRows, err)
	}
	assertUntouched("GetCachedPlotClimate (seeded)", seededBytes)

	hitRes, err := rsidecar.ComputePlotClimateAndDualEdatopic(ctx, svc, state.ContextID, rsidecar.ClimateEdatopicRequest{
		RefPeriod:   "1961_1990",
		PlotNumbers: []string{"00337"},
	}, rsidecar.RuntimeOptions{RscriptPath: "disabled"})
	if err != nil || hitRes.CacheHits != 1 || hitRes.CacheMisses != 0 {
		t.Fatalf("ComputePlotClimateAndDualEdatopic cache-hit failed: res=%+v err=%v", hitRes, err)
	}
	assertUntouched("ComputePlotClimateAndDualEdatopic (100% cache hit)", seededBytes)

	clustRes, err := rsidecar.RunMultivariateClustering(ctx, svc, state.ContextID, rsidecar.ClusteringRequest{
		Method:               "HCLUST",
		Distance:             "chord",
		Linkage:              "average",
		Transform:            "hellinger",
		K:                    3,
		MaterializeCandidate: false,
	})
	if err != nil || len(clustRes.Clusters) != 3 {
		t.Fatalf("RunMultivariateClustering (non-materializing) failed: res=%+v err=%v", clustRes, err)
	}
	assertUntouched("RunMultivariateClustering (MaterializeCandidate=false)", seededBytes)
}
