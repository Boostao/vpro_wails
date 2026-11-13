package projectops_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestLifecyclePrefixRewriteAuditOwnershipAndWALMergeDigest(t *testing.T) {
	t.Run("BugA_SaveProjectAsPrefixExtensionAndBugB_AuditOwnershipAndZeroIdentity", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		// Find an existing plot in Sample_Env and ensure it has an Audit entry + ID=0 ChildIdentity reservation.
		var samplePlot string
		roInit, err := sqlutil.OpenReadOnly(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := roInit.QueryRow(`SELECT "PlotNumber" FROM "Sample_Env" ORDER BY "PlotNumber" LIMIT 1`).Scan(&samplePlot); err != nil {
			roInit.Close()
			t.Fatal(err)
		}
		roInit.Close()

		vprotest.MutateContextFixture(t, state.ProjectPath, `
			CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
				"ChildTable" TEXT NOT NULL,
				"ID" INTEGER NOT NULL,
				PRIMARY KEY ("ChildTable", "ID")
			);
			INSERT OR IGNORE INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") VALUES ('"Sample_Veg"', 0);
			INSERT INTO "Sample_Audit" ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID")
			VALUES ('Sample','Tester','`+samplePlot+`','_Env','Elevation','2026-06-01 12:00:00','500','550',0,0,NULL);
		`)

		// Bug A: Save "Sample" as "Sample_Copy" (where "Sample_" is a prefix of "Sample_Copy_").
		savedAs, err := projectops.SaveProjectAs(svc, ctx, state.ContextID, projectops.SaveProjectAsRequest{
			NewProjectName:    "Sample_Copy",
			ActivateOnSuccess: false,
		})
		if err != nil {
			t.Fatalf("SaveProjectAs('Sample' -> 'Sample_Copy') failed: %v", err)
		}

		copyDB, err := sqlutil.OpenReadOnly(savedAs.DestinationPath)
		if err != nil {
			t.Fatal(err)
		}
		defer copyDB.Close()

		ddlRows, err := copyDB.Query(`SELECT name, sql FROM sqlite_master WHERE sql IS NOT NULL`)
		if err != nil {
			t.Fatal(err)
		}
		for ddlRows.Next() {
			var name, ddl string
			if err := ddlRows.Scan(&name, &ddl); err != nil {
				ddlRows.Close()
				t.Fatal(err)
			}
			if strings.Contains(name, "Sample_Copy_Copy") || strings.Contains(ddl, "Sample_Copy_Copy") {
				ddlRows.Close()
				t.Fatalf("found double-rewritten identifier in %s: %s", name, ddl)
			}
		}
		ddlRows.Close()

		// Bug B (part 1): ExportSplinterProject from Sample -> Sample_Splinter, preserving ID=0 reservation and rewriting Audit.Project.
		splinter, err := projectops.ExportSplinterProject(svc, ctx, state.ContextID, projectops.ExportSplinterRequest{
			SplinterProject:   "Sample_Splinter",
			PlotNumbers:       []string{samplePlot},
			ActivateOnSuccess: false,
		})
		if err != nil {
			t.Fatalf("ExportSplinterProject failed: %v", err)
		}

		splDB, err := sqlutil.OpenReadOnly(splinter.DestinationPath)
		if err != nil {
			t.Fatal(err)
		}
		defer splDB.Close()

		var zeroIdentityCount int
		if err := splDB.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ChildTable" = '"Sample_Splinter_Veg"' AND "ID" = 0`).Scan(&zeroIdentityCount); err != nil {
			t.Fatal(err)
		}
		if zeroIdentityCount != 1 {
			t.Fatalf("expected ID=0 reservation copied into Sample_Splinter __VPRO_ChildIdentity, got %d", zeroIdentityCount)
		}

		// Switch context to Sample_Copy and verify ListAuditEntries + ListAuditReadEntries succeed without foreign ownership error.
		switchedCopy, err := svc.SwitchContext(state.ContextID, vprocore.ContextSelection{
			Project:     "Sample_Copy",
			ProjectPath: savedAs.DestinationPath,
			SU:          "None",
			Hierarchy:   "None",
		})
		if err != nil {
			t.Fatalf("SwitchContext to Sample_Copy failed: %v", err)
		}
		if entries, err := svc.ListAuditEntries(ctx, switchedCopy.ContextID, samplePlot); err != nil || len(entries) == 0 {
			t.Fatalf("ListAuditEntries on Sample_Copy failed: entries=%d err=%v", len(entries), err)
		}
		if readEntries, err := svc.ListAuditReadEntries(ctx, switchedCopy.ContextID, samplePlot); err != nil || len(readEntries) == 0 {
			t.Fatalf("ListAuditReadEntries on Sample_Copy failed: entries=%d err=%v", len(readEntries), err)
		}

		// Switch context to Sample_Splinter and verify ListAuditEntries + ListAuditReadEntries succeed without foreign ownership error.
		switchedSpl, err := svc.SwitchContext(switchedCopy.ContextID, vprocore.ContextSelection{
			Project:     "Sample_Splinter",
			ProjectPath: splinter.DestinationPath,
			SU:          "None",
			Hierarchy:   "None",
		})
		if err != nil {
			t.Fatalf("SwitchContext to Sample_Splinter failed: %v", err)
		}
		if entries, err := svc.ListAuditEntries(ctx, switchedSpl.ContextID, samplePlot); err != nil || len(entries) == 0 {
			t.Fatalf("ListAuditEntries on Sample_Splinter failed: entries=%d err=%v", len(entries), err)
		}
		if readEntries, err := svc.ListAuditReadEntries(ctx, switchedSpl.ContextID, samplePlot); err != nil || len(readEntries) == 0 {
			t.Fatalf("ListAuditReadEntries on Sample_Splinter failed: entries=%d err=%v", len(readEntries), err)
		}
	})

	t.Run("BugC_ThreeWayMergeDigestStableAcrossWALCheckpoint", func(t *testing.T) {
		svc, state := vprotest.ContextServiceFixture(t)
		ctx := context.Background()

		baseRes, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
			ProjectName: "WALBase",
			ProjectID:   "WAL26",
		})
		if err != nil {
			t.Fatal(err)
		}
		vprotest.MutateContextFixture(t, baseRes.DestinationPath, `
			INSERT INTO "WALBase_Env" ("PlotNumber","ProjectID","Elevation","Zone") VALUES ('0001','WAL26',600,'ICH');
			INSERT INTO "WALBase_Admin" ("Plot","PlotType") VALUES ('0001','FS882');
		`)

		switchedBase, err := svc.SwitchContext(state.ContextID, vprocore.ContextSelection{
			Project:     "WALBase",
			ProjectPath: baseRes.DestinationPath,
			SU:          "None",
			Hierarchy:   "None",
		})
		if err != nil {
			t.Fatal(err)
		}
		tgtRes, err := projectops.SaveProjectAs(svc, ctx, switchedBase.ContextID, projectops.SaveProjectAsRequest{
			NewProjectName: "WALTarget",
		})
		if err != nil {
			t.Fatal(err)
		}
		stgRes, err := projectops.SaveProjectAs(svc, ctx, switchedBase.ContextID, projectops.SaveProjectAsRequest{
			NewProjectName: "WALStage",
		})
		if err != nil {
			t.Fatal(err)
		}

		switchedTgt, err := svc.SwitchContext(switchedBase.ContextID, vprocore.ContextSelection{
			Project:     "WALTarget",
			ProjectPath: tgtRes.DestinationPath,
			SU:          "None",
			Hierarchy:   "None",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Open WALStage in WAL mode, disable auto-checkpoint so frames sit in WALStage.db-wal during preview!
		walDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(stgRes.DestinationPath, "rw")+"&_journal_mode=WAL")
		if err != nil {
			t.Fatal(err)
		}
		defer walDB.Close()
		if _, err := walDB.Exec(`PRAGMA wal_autocheckpoint = 0; UPDATE "WALStage_Env" SET "Elevation" = 725 WHERE "PlotNumber" = '0001';`); err != nil {
			t.Fatal(err)
		}

		preview, err := projectops.PreviewThreeWayMerge(svc, ctx, switchedTgt.ContextID, projectops.ThreeWayMergePreviewRequest{
			SourcePath:    stgRes.DestinationPath,
			SourceProject: "WALStage",
			BasePath:      baseRes.DestinationPath,
			BaseProject:   "WALBase",
		})
		if err != nil {
			t.Fatalf("PreviewThreeWayMerge failed: %v", err)
		}

		// Now checkpoint WALStage.db-wal into WALStage.db (changing raw file + WAL bytes without changing logical content!).
		if _, err := walDB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			t.Fatal(err)
		}
		_ = walDB.Close()

		mergeRes, err := projectops.ExecuteThreeWayMerge(svc, ctx, switchedTgt.ContextID, projectops.ThreeWayMergeExecuteRequest{
			SourcePath:            stgRes.DestinationPath,
			SourceProject:         "WALStage",
			BasePath:              baseRes.DestinationPath,
			BaseProject:           "WALBase",
			ExpectedPreviewDigest: preview.PreviewDigest,
		})
		if err != nil {
			t.Fatalf("ExecuteThreeWayMerge failed after WAL checkpoint: %v", err)
		}
		if mergeRes.UpdatedPlots != 1 {
			t.Fatalf("expected 1 updated plot, got %+v", mergeRes)
		}

		// Verify genuine logical change after preview IS still rejected!
		vprotest.MutateContextFixture(t, stgRes.DestinationPath, `UPDATE "WALStage_Env" SET "Elevation" = 999 WHERE "PlotNumber" = '0001';`)
		if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, switchedTgt.ContextID, projectops.ThreeWayMergeExecuteRequest{
			SourcePath:            stgRes.DestinationPath,
			SourceProject:         "WALStage",
			BasePath:              baseRes.DestinationPath,
			BaseProject:           "WALBase",
			ExpectedPreviewDigest: preview.PreviewDigest,
		}); err == nil || !strings.Contains(err.Error(), "changed since preview") {
			t.Fatalf("expected stale preview digest error after logical edit, got %v", err)
		}
	})
}
