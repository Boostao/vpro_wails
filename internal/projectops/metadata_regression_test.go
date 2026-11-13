package projectops_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestMetadataAutomationForeignKeyRegionPathAndProjectIDFallback(t *testing.T) {
	t.Run("BugA_ForeignKeyNullPlotNumberAndBugB_RegionCodesRootPath", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		projectsDir := filepath.Dir(state.ProjectPath)
		dataRoot := filepath.Dir(projectsDir)

		// Bug B check: DeriveProjectMetadataFromPlots must write region-codes.db in dataRoot, NOT projectsDir.
		derived, err := projectops.DeriveProjectMetadataFromPlots(svc, ctx, state.ContextID, "BEC")
		if err != nil {
			t.Fatalf("DeriveProjectMetadataFromPlots failed: %v", err)
		}
		if derived.TotalPlotsInScope == 0 {
			t.Fatalf("expected BEC plots in Sample.db, got 0")
		}
		strayRegionDB := filepath.Join(projectsDir, "region-codes.db")
		if _, statErr := os.Stat(strayRegionDB); !os.IsNotExist(statErr) {
			t.Fatalf("expected no stray %s in projects directory, got statErr=%v", strayRegionDB, statErr)
		}
		expectedRegionDB := filepath.Join(dataRoot, "region-codes.db")
		if _, statErr := os.Stat(expectedRegionDB); statErr != nil {
			t.Fatalf("expected region-codes.db at data root %s: %v", expectedRegionDB, statErr)
		}

		// Bug A check: Sample_Audit has FOREIGN KEY ("PlotNumber") REFERENCES "Sample_Env" ("PlotNumber").
		// Updating an existing _Metadata row AND creating a new _Metadata row must insert NULL PlotNumber in _Audit.
		updated, err := projectops.ApplyProjectMetadataAutomation(svc, ctx, state.ContextID, projectops.ApplyProjectMetadataAutomationRequest{
			ProjectID:    "BEC",
			ApplyDerived: true,
			PresetID:     "LMH25_2023_FS882",
		})
		if err != nil {
			t.Fatalf("ApplyProjectMetadataAutomation (existing row BEC) failed with FK error: %v", err)
		}
		if updated.CreatedRow || updated.AuditRowCount == 0 {
			t.Fatalf("expected existing BEC row update with audits, got %+v", updated)
		}

		created, err := projectops.ApplyProjectMetadataAutomation(svc, ctx, state.ContextID, projectops.ApplyProjectMetadataAutomationRequest{
			ProjectID:    "NEWPID26",
			ApplyDerived: false,
			PresetID:     "DEIF_1980",
		})
		if err != nil {
			t.Fatalf("ApplyProjectMetadataAutomation (new row NEWPID26) failed with FK error: %v", err)
		}
		if !created.CreatedRow || created.AuditRowCount == 0 {
			t.Fatalf("expected new NEWPID26 row creation with audits, got %+v", created)
		}

		roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		defer roDB.Close()

		var nonNullPlotAuditCount int
		if err := roDB.QueryRow(`SELECT COUNT(*) FROM "Sample_Audit" WHERE "Table" = '_Metadata' AND "PlotNumber" IS NOT NULL`).Scan(&nonNullPlotAuditCount); err != nil {
			t.Fatal(err)
		}
		if nonNullPlotAuditCount != 0 {
			t.Fatalf("expected 0 _Metadata audit rows with non-NULL PlotNumber, got %d", nonNullPlotAuditCount)
		}
	})

	t.Run("BugC_SingleExistingProjectIDResolutionWhenUIRequestsProjectName", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		// Trim Sample_Metadata and Sample_Env to a single ProjectID ('BEC') while active project is 'Sample'.
		vprotest.MutateContextFixture(t, state.ProjectPath, `
			DELETE FROM "Sample_Metadata" WHERE "ProjectID" <> 'BEC';
			UPDATE "Sample_Env" SET "ProjectID" = 'BEC';
		`)

		// UI passes projectID == snap.Project ("Sample"), which matches 0 rows literally in _Env/_Metadata,
		// while _Metadata has 1 row with ProjectID = "BEC".
		derived, err := projectops.DeriveProjectMetadataFromPlots(svc, ctx, state.ContextID, "Sample")
		if err != nil {
			t.Fatalf("DeriveProjectMetadataFromPlots('Sample') failed: %v", err)
		}
		if derived.ProjectID != "BEC" || derived.TotalPlotsInScope == 0 {
			t.Fatalf("expected resolution to single existing ProjectID 'BEC' with >0 plots, got ProjectID=%q TotalPlotsInScope=%d",
				derived.ProjectID, derived.TotalPlotsInScope)
		}

		applied, err := projectops.ApplyProjectMetadataAutomation(svc, ctx, state.ContextID, projectops.ApplyProjectMetadataAutomationRequest{
			ProjectID:    "Sample",
			ApplyDerived: true,
			PresetID:     "LMH25_2023_FS882",
		})
		if err != nil {
			t.Fatalf("ApplyProjectMetadataAutomation('Sample') failed: %v", err)
		}
		if applied.CreatedRow || applied.ProjectID != "BEC" {
			t.Fatalf("expected update to existing 'BEC' row (CreatedRow=false), got %+v", applied)
		}

		roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		defer roDB.Close()

		var metaCount int
		if err := roDB.QueryRow(`SELECT COUNT(*) FROM "Sample_Metadata"`).Scan(&metaCount); err != nil {
			t.Fatal(err)
		}
		if metaCount != 1 {
			t.Fatalf("expected exactly 1 row in Sample_Metadata (no duplicate 'Sample' row), got %d", metaCount)
		}
	})
}
