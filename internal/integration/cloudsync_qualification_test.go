package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/cloudsync"
	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/validation"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func locateBECMasterForPGQualification(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/tmp/vpro-pg-qa/BECMaster_fixing.accdb",
	}
	if explicit := strings.TrimSpace(os.Getenv("VPRO_BECMASTER_ACCDB")); explicit != "" {
		candidates = append(candidates, explicit)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "Work", "VPRO_ACCESS_1", "VPro64", "BECMaster_fixing.accdb"))
	}
	for _, src := range candidates {
		if st, err := os.Stat(src); err == nil && st.Mode().IsRegular() && st.Size() > 100*1024*1024 {
			if strings.HasPrefix(src, "/tmp/vpro-pg-qa/") {
				return src
			}
			dst := filepath.Join(t.TempDir(), "BECMaster_fixing_disposable.accdb")
			inF, err := os.Open(src)
			if err != nil {
				t.Fatalf("open BECMaster source: %v", err)
			}
			outF, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				inF.Close()
				t.Fatalf("create disposable BECMaster copy: %v", err)
			}
			if _, err := io.Copy(outF, inF); err != nil {
				inF.Close()
				outF.Close()
				t.Fatalf("copy BECMaster: %v", err)
			}
			_ = inF.Close()
			_ = outF.Close()
			return dst
		}
	}
	t.Skip("BECMaster_fixing.accdb not available; skipping 71,567-plot live PostgreSQL qualification")
	return ""
}

func makeLayer3ComplianceValidator() cloudsync.ComplianceValidatorFunc {
	return func(ctx context.Context, core *vprocore.ContextService, cid string, useActiveSU bool) (cloudsync.ComplianceGateResult, error) {
		rep, err := validation.RunComplianceValidation(core, ctx, cid, validation.ComplianceOptions{UseActiveSU: useActiveSU})
		if err != nil {
			return cloudsync.ComplianceGateResult{}, err
		}
		raw, _ := json.Marshal(rep)
		blocking := rep.Summary.EnvCodeErrors +
			rep.Summary.BoundsErrors +
			rep.Summary.EnvAdminMismatches +
			rep.Summary.OrphanChildRows +
			rep.Summary.DuplicateSpeciesSplits +
			rep.Summary.UnlistedSpecies +
			rep.Summary.SynonymSpecies
		return cloudsync.ComplianceGateResult{
			Passed:        rep.Passed,
			BlockingCount: blocking,
			Summary: map[string]int{
				"envCodeErrors":          rep.Summary.EnvCodeErrors,
				"boundsErrors":           rep.Summary.BoundsErrors,
				"envAdminMismatches":     rep.Summary.EnvAdminMismatches,
				"orphanChildRows":        rep.Summary.OrphanChildRows,
				"duplicateSpeciesSplits": rep.Summary.DuplicateSpeciesSplits,
				"unlistedSpecies":        rep.Summary.UnlistedSpecies,
				"synonymSpecies":         rep.Summary.SynonymSpecies,
				"userSpecies":            rep.Summary.UserSpecies,
			},
			ReportJSON: raw,
		}, nil
	}
}

func TestCloudSyncBECMaster71kQualificationAndThreeWayCustodianMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live PostgreSQL 71,567-plot BECMaster qualification in -short mode")
	}

	mgr := cloudsync.NewManager("", os.LookupEnv)
	defer mgr.Close()
	cfgStatus, err := mgr.GetConfigStatus()
	if err != nil || !cfgStatus.Configured || !cfgStatus.HasContributorCreds || !cfgStatus.HasCustodianCreds {
		t.Skip("live PostgreSQL credentials (VPRO_PG_* or becmaster.env) not configured; skipping live qualification")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 1. Bootstrap schemas + role grants as Custodian
	if _, err := mgr.BootstrapSchemas(ctx); err != nil {
		t.Fatalf("BootstrapSchemas failed: %v", err)
	}

	svc, state := vprotest.ContextServiceFixture(t)
	activeCtxID := state.ContextID

	if _, err := mgr.ConfigureSession(cloudsync.SessionConfigRequest{RoleMode: cloudsync.RoleModeCustodian}); err != nil {
		t.Fatal(err)
	}
	connRep, err := mgr.TestCloudConnection(ctx)
	if err != nil {
		t.Fatalf("TestCloudConnection(custodian) failed: %v", err)
	}

	// Ensure 2026 standards are published
	if connRep.ActiveStandardsRelease == "" {
		if _, err := mgr.PublishStandardsToCloud(ctx, svc, activeCtxID, cloudsync.PublishStandardsRequest{
			Notes: "2026 BECdb v13 + BCPlantTaxonomy v19 baseline",
		}); err != nil {
			t.Fatalf("PublishStandardsToCloud failed: %v", err)
		}
	}

	// Seed 71,567-plot BECMaster_fixing baseline if not yet seeded (or if VPRO_FORCE_PG_RESEED=1)
	if connRep.BECMasterPlotCount < 71500 || os.Getenv("VPRO_FORCE_PG_RESEED") == "1" {
		accdbPath := locateBECMasterForPGQualification(t)
		sqliteStage := filepath.Join(t.TempDir(), "BECMaster_fixing.db")
		impRes, impErr := interchange.ImportAccessProject(svc, ctx, activeCtxID, interchange.AccessProjectImportRequest{
			SourcePath:        accdbPath,
			SourceProject:     "BECMaster_fixing",
			TargetProject:     "BECMaster_fixing",
			DestinationPath:   sqliteStage,
			ActivateOnSuccess: false,
		})
		if impErr != nil {
			t.Fatalf("ImportAccessProject(BECMaster_fixing.accdb) failed: %v", impErr)
		}
		t.Logf("[Qualification Step 1] Imported BECMaster_fixing.accdb -> SQLite (%d plots)", impRes.PlotCount)

		seedRes, sErr := mgr.SeedBECMasterBaseline(ctx, cloudsync.SeedBaselineRequest{
			SourceDBPath:    sqliteStage,
			SourceProject:   "BECMaster_fixing",
			SnapshotTag:     "BECMaster_v13_Baseline",
			ReplaceExisting: true,
			Notes:           "71,567-plot BECMaster_fixing production baseline",
		})
		if sErr != nil {
			t.Fatalf("SeedBECMasterBaseline failed: %v", sErr)
		}
		t.Logf("[Qualification Step 2] Seeded PostgreSQL BECMaster in %d ms: plots=%d veg=%d humus=%d mineral=%d audit=%d child_ids=%d",
			seedRes.ElapsedMs, seedRes.PlotCount, seedRes.VegCount, seedRes.HumusCount, seedRes.MineralCount, seedRes.AuditCount, seedRes.ChildIdentityRows)
	}

	// Verify exact baseline counts before running contributor/custodian test transactions
	adminPool, _, err := mgr.AcquirePool(ctx, cloudsync.RoleModeCustodian)
	if err != nil {
		t.Fatal(err)
	}
	var baselinePlotCount int
	var baselineVegCount, baselineMaxAuditSeq int64
	if err := adminPool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM becmaster.env),
			(SELECT COUNT(*) FROM becmaster.veg),
			COALESCE((SELECT MAX(audit_seq) FROM becmaster.audit), 0)
	`).Scan(&baselinePlotCount, &baselineVegCount, &baselineMaxAuditSeq); err != nil {
		t.Fatal(err)
	}
	t.Logf("[Qualification Baseline] becmaster.env=%d plots | becmaster.veg=%d rows | max_audit_seq=%d",
		baselinePlotCount, baselineVegCount, baselineMaxAuditSeq)
	if baselinePlotCount != 71567 {
		t.Fatalf("expected exact 71,567 plots in becmaster.env, got %d", baselinePlotCount)
	}
	if baselineVegCount < 1650000 {
		t.Fatalf("expected >= 1,650,000 rows in becmaster.veg, got %d", baselineVegCount)
	}
	var corruptedTimestampCount int
	if err := adminPool.QueryRow(ctx, `SELECT COUNT(*) FROM becmaster.env WHERE "Date" LIKE '%+0000 UTC%'`).Scan(&corruptedTimestampCount); err != nil {
		t.Fatal(err)
	}
	if corruptedTimestampCount != 0 {
		t.Fatalf("expected 0 go-sqlite3 '+0000 UTC' corrupted Date strings in becmaster.env, found %d", corruptedTimestampCount)
	}

	// Choose a guaranteed non-existent leading-zero PlotNumber (preferring '00338')
	leadingZeroPlot := "00338"
	var exists00338 bool
	_ = adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM becmaster.env WHERE "PlotNumber" = $1)`, leadingZeroPlot).Scan(&exists00338)
	if exists00338 {
		for _, cand := range []string{"00339", "00038", "00938", "00008"} {
			var ex bool
			_ = adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM becmaster.env WHERE "PlotNumber" = $1)`, cand).Scan(&ex)
			if !ex {
				leadingZeroPlot = cand
				break
			}
		}
	}

	// Find a clean CWH plot in becmaster that passes strict 2026 compliance so we can pull it + 4 neighbours
	var targetEditPlot string
	var origSiteNotes *string
	var origBECSiteUnit *string
	if err := adminPool.QueryRow(ctx, `
		SELECT e."PlotNumber", e."SiteNotes", a."BECSiteUnit"
		FROM becmaster.env e
		JOIN becmaster.admin a ON a."Plot" = e."PlotNumber"
		WHERE e."Zone" = 'CWH'
		  AND length(e."PlotNumber") <= 7
		  AND (a."StartDate" IS NULL OR (a."StartDate" BETWEEN 1900 AND 2500))
		  AND NOT EXISTS (
			SELECT 1 FROM becmaster.veg v
			WHERE v."PlotNumber" = e."PlotNumber"
			  AND NOT EXISTS (SELECT 1 FROM standards.usys_all_specs s WHERE UPPER(s."Code") = UPPER(v."Species"))
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM becmaster.veg v
			WHERE v."PlotNumber" = e."PlotNumber"
			GROUP BY UPPER(v."Species") HAVING COUNT(*) > 1
		  )
		ORDER BY e."PlotNumber"
		LIMIT 1
	`).Scan(&targetEditPlot, &origSiteNotes, &origBECSiteUnit); err != nil {
		t.Fatalf("find compliant CWH plot for test edit: %v", err)
	}

	var createdSubIDs []string
	var createdMergeTxID string
	var createdDownloadLogID int64
	var allocatedTestChildIDs []int64
	const testSnapshotTag = "QA_Wave4_Snapshot"
	const testProjectID = "QA_CWH_2026_SUB"

	// Strict cleanup ensuring production becmaster returns to the exact 71,567-plot baseline!
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanCancel()
		cConn, _, cErr := mgr.OpenDedicatedConn(cleanCtx, cloudsync.RoleModeCustodian)
		if cErr != nil {
			t.Errorf("cleanup connect failed: %v", cErr)
			return
		}
		defer cConn.Close(cleanCtx)

		_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.env WHERE "PlotNumber" = $1`, leadingZeroPlot)
		_, _ = cConn.Exec(cleanCtx, `UPDATE becmaster.env SET "SiteNotes" = $1 WHERE "PlotNumber" = $2`, origSiteNotes, targetEditPlot)
		_, _ = cConn.Exec(cleanCtx, `UPDATE becmaster.admin SET "BECSiteUnit" = $1 WHERE "Plot" = $2`, origBECSiteUnit, targetEditPlot)
		_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.veg WHERE "PlotNumber" = $1 AND "Species" = 'MYQASPP'`, targetEditPlot)
		_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.metadata WHERE "ProjectID" = $1`, testProjectID)
		if len(allocatedTestChildIDs) > 0 {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.child_identity WHERE id = ANY($1)`, allocatedTestChildIDs)
		}
		if baselineMaxAuditSeq > 0 {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.audit WHERE audit_seq > $1`, baselineMaxAuditSeq)
		} else if createdMergeTxID != "" {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.audit WHERE merge_tx_id = $1::uuid`, createdMergeTxID)
		}
		if createdMergeTxID != "" {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.merge_ledger WHERE merge_tx_id = $1::uuid`, createdMergeTxID)
		}
		for _, sid := range createdSubIDs {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM staging.submissions WHERE submission_id = $1::uuid`, sid)
		}
		_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.snapshots WHERE snapshot_tag = $1`, testSnapshotTag)
		if createdDownloadLogID > 0 {
			_, _ = cConn.Exec(cleanCtx, `DELETE FROM becmaster.download_log WHERE id >= $1`, createdDownloadLogID)
		}

		var finalPlots int
		var finalVeg int64
		if err := cConn.QueryRow(cleanCtx, `SELECT (SELECT COUNT(*) FROM becmaster.env), (SELECT COUNT(*) FROM becmaster.veg)`).Scan(&finalPlots, &finalVeg); err != nil {
			t.Errorf("verify final baseline counts: %v", err)
		} else if finalPlots != baselinePlotCount || finalVeg != baselineVegCount {
			t.Errorf("cleanup left drift in becmaster: got plots=%d veg=%d, want plots=%d veg=%d", finalPlots, finalVeg, baselinePlotCount, baselineVegCount)
		}
	})

	// 2. Connect as Contributor (VPRO_PG_USER): verify direct write rejection + pull CWH subset
	if _, err := mgr.ConfigureSession(cloudsync.SessionConfigRequest{RoleMode: cloudsync.RoleModeContributor}); err != nil {
		t.Fatal(err)
	}
	uConn, _, err := mgr.OpenDedicatedConn(ctx, cloudsync.RoleModeContributor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uConn.Exec(ctx, `UPDATE becmaster.env SET "SiteNotes" = 'hacked' WHERE "PlotNumber" = $1`, targetEditPlot); err == nil {
		uConn.Close(ctx)
		t.Fatal("SECURITY FAILURE: contributor was allowed to UPDATE becmaster.env directly")
	}
	if _, err := uConn.Exec(ctx, `DELETE FROM becmaster.env WHERE "PlotNumber" = $1`, targetEditPlot); err == nil {
		uConn.Close(ctx)
		t.Fatal("SECURITY FAILURE: contributor was allowed to DELETE FROM becmaster.env directly")
	}
	uConn.Close(ctx)

	cwhPrev, err := mgr.PreviewBECMasterSubset(ctx, cloudsync.SubsetFilter{Zones: []string{"CWH"}, Limit: 20})
	if err != nil || cwhPrev.MatchingPlots == 0 {
		t.Fatalf("PreviewBECMasterSubset(CWH) failed: prev=%+v err=%v", cwhPrev, err)
	}

	subsetDest := filepath.Join(t.TempDir(), "CWH_Subset_QA.db")
	pullRes, err := mgr.PullBECMasterSubset(ctx, svc, activeCtxID, cloudsync.PullSubsetRequest{
		Filter: cloudsync.SubsetFilter{
			Zones:       []string{"CWH"},
			PlotNumbers: []string{targetEditPlot},
		},
		TargetProject:     "CWH_Subset_QA",
		DestinationPath:   subsetDest,
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatalf("PullBECMasterSubset failed: %v", err)
	}
	createdDownloadLogID = pullRes.DownloadLogID
	if !pullRes.Activated || pullRes.State == nil || pullRes.PlotCount != 1 || pullRes.DownloadLogID <= 0 {
		t.Fatalf("unexpected PullBECMasterSubset result: %+v", pullRes)
	}
	activeCtxID = pullRes.State.ContextID

	// Register personal species 'MYQASPP' in VUser.USysUserSpp so UserSpeciesManifest packaging is exercised
	vuserPath := svc.Projects.SupportPaths["VUser"]
	vuserDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(vuserPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vuserDB.Exec(`INSERT OR REPLACE INTO "USysUserSpp" ("Code", "ScientificName", "LifeForm", "EnglishName", "Codetype") VALUES ('MYQASPP', 'Carex testiana', 6, 'QA sedge', 'U')`); err != nil {
		_ = vuserDB.Close()
		t.Fatal(err)
	}
	_ = vuserDB.Close()
	// Refresh context so attached VUser sees MYQASPP
	swState, err := svc.SwitchContext(activeCtxID, vprocore.ContextSelection{
		Project:     "CWH_Subset_QA",
		ProjectPath: subsetDest,
		SU:          "None",
		Hierarchy:   "None",
	})
	if err != nil {
		t.Fatal(err)
	}
	activeCtxID = swState.ContextID

	// Normalize pulled plot codes if historical CWH plot had any out-of-list legacy fields, edit SiteNotes + BECSiteUnit, and add leading-zero plot '00338'!
	subDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(subsetDest, "rw")+"&_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	// Clean any historical code fields on targetEditPlot except Zone='CWH' so strict 2026 USysTableOfLists compliance passes cleanly
	if _, err := subDB.Exec(`
		DELETE FROM "CWH_Subset_QA_Env";
		DELETE FROM "CWH_Subset_QA_Admin";
		DELETE FROM "CWH_Subset_QA_Veg";
		DELETE FROM "CWH_Subset_QA_Humus";
		DELETE FROM "CWH_Subset_QA_Mineral";
		DELETE FROM "CWH_Subset_QA_Other";
		DELETE FROM "CWH_Subset_QA_Metadata";
		INSERT INTO "CWH_Subset_QA_Metadata" ("ProjectID", "ProjectTitle", "ID") VALUES (?, 'CWH QA Subset', 1);
		INSERT INTO "CWH_Subset_QA_Env" ("PlotNumber", "ProjectID", "Date", "Zone", "SubZone", "SiteNotes", "Latitude", "Longitude", "SpeciesListComplete", "Flag")
		VALUES (?, ?, '2026-07-15', 'CWH', 'ds', 'Updated field notes from contributor CWH QA', 49.25, -125.15, -1, 0),
		       (?, ?, '2026-07-16', 'CWH', 'ds', 'New leading-zero plot from CWH QA', 49.28, -125.18, -1, 0);
		INSERT INTO "CWH_Subset_QA_Admin" ("Plot", "StartDate", "UpdatedFromCards", "UserSiteUnit", "BECSiteUnit")
		VALUES (?, 2026, 0, 'CWHvm1/01', 'CWHvm1/99_PROPOSED'),
		       (?, 2026, 0, 'CWHvm1/01', NULL);
		INSERT INTO "CWH_Subset_QA_Veg" ("PlotNumber", "Species", "Layer", "Cover6", "Flag", "ID")
		VALUES (?, 'MYQASPP', '6', 99.999999, 0, 500001),
		       (?, 'TSUGHET', '1', 45.0, 0, 500002);
		INSERT INTO "CWH_Subset_QA_Humus" ("PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "Flag", "ID")
		VALUES (?, 'Fm', 6.0, 0.0, 0, 600001);
		INSERT INTO "CWH_Subset_QA_Mineral" ("PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "Texture", "Flag", "ID")
		VALUES (?, 'Bm1', 0.0, 25.0, 'SL', 0, 700001);
	`, testProjectID,
		targetEditPlot, testProjectID,
		leadingZeroPlot, testProjectID,
		targetEditPlot,
		leadingZeroPlot,
		targetEditPlot,
		leadingZeroPlot,
		leadingZeroPlot,
		leadingZeroPlot,
	); err != nil {
		_ = subDB.Close()
		t.Fatal(err)
	}

	validator := makeLayer3ComplianceValidator()

	// Verify 99.999999 (which narrows to 100.0 in IEEE-754 float32) BLOCKS staging submission!
	if _, blockedErr := mgr.SubmitProjectToStaging(ctx, svc, activeCtxID, cloudsync.SubmitStagingRequest{
		SubmittedBy:    "Regional Ecologist",
		SubmitterNotes: "Should be blocked by Cover6=99.999999",
	}, validator); blockedErr == nil {
		_ = subDB.Close()
		t.Fatal("expected Cover6=99.999999 (float32 100.0) to be blocked by compliance gate")
	}

	// Narrow Cover6 to 99.999996 (which narrows to 99.999992 < 100 in float32) and pass compliance!
	if _, err := subDB.Exec(`UPDATE "CWH_Subset_QA_Veg" SET "Cover6" = 99.999996 WHERE "PlotNumber" = ? AND "Species" = 'MYQASPP'`, targetEditPlot); err != nil {
		_ = subDB.Close()
		t.Fatal(err)
	}
	_ = subDB.Close()

	preflight, err := mgr.PreviewStagingSubmission(ctx, svc, activeCtxID, false, validator)
	if err != nil {
		t.Fatalf("PreviewStagingSubmission failed: %v", err)
	}
	if !preflight.CompliancePassed || preflight.UserSpeciesManifest.RowCount != 1 || preflight.UserSpeciesManifest.Species[0].Code != "MYQASPP" {
		t.Fatalf("unexpected StagingPreflightPreview: %+v", preflight)
	}

	subRes, err := mgr.SubmitProjectToStaging(ctx, svc, activeCtxID, cloudsync.SubmitStagingRequest{
		SubmittedBy:     "Regional Ecologist",
		SubmitterNotes:  "CWH subset with leading-zero plot " + leadingZeroPlot,
		BaseSnapshotTag: "BECMaster_v13_Baseline",
	}, validator)
	if err != nil {
		t.Fatalf("SubmitProjectToStaging failed: %v", err)
	}
	createdSubIDs = append(createdSubIDs, subRes.SubmissionID)
	if subRes.Status != "pending" || subRes.PlotCount != 2 || subRes.UserSpeciesCount != 1 {
		t.Fatalf("unexpected SubmitStagingResult: %+v", subRes)
	}

	// Also submit a second pending submission to verify RejectStagingSubmission workflow
	subResReject, err := mgr.SubmitProjectToStaging(ctx, svc, activeCtxID, cloudsync.SubmitStagingRequest{
		SubmittedBy:    "Regional Ecologist",
		SubmitterNotes: "Duplicate draft to reject",
	}, validator)
	if err != nil {
		t.Fatalf("second SubmitProjectToStaging failed: %v", err)
	}
	createdSubIDs = append(createdSubIDs, subResReject.SubmissionID)

	// 3. Connect as Custodian (VPRO_PG_ADMIN_USER): review detail, <500ms 3-way diff vs 71,567-plot BECMaster, selective merge, snapshot & audit verification!
	if _, err := mgr.ConfigureSession(cloudsync.SessionConfigRequest{RoleMode: cloudsync.RoleModeCustodian}); err != nil {
		t.Fatal(err)
	}

	detail, err := mgr.GetStagingSubmissionDetail(ctx, subRes.SubmissionID)
	if err != nil {
		t.Fatalf("GetStagingSubmissionDetail failed: %v", err)
	}
	if len(detail.ProvisionalSpeciesDiffs) != 1 || detail.ProvisionalSpeciesDiffs[0].Status != "new_provisional" {
		t.Fatalf("expected MYQASPP flagged as new_provisional, got %+v", detail.ProvisionalSpeciesDiffs)
	}

	mergePrev, err := mgr.PreviewStagingToMasterMerge(ctx, cloudsync.StagingMergePreviewRequest{
		SubmissionID: subRes.SubmissionID,
	})
	if err != nil {
		t.Fatalf("PreviewStagingToMasterMerge failed: %v", err)
	}
	t.Logf("[Qualification Step 3 — 3-Way Diff vs 71,567-plot BECMaster] elapsed=%d ms added=%d conflict=%d protected_diffs=%d",
		mergePrev.ElapsedMs, mergePrev.AddedCount, mergePrev.ConflictCount, len(mergePrev.CustodianProtectedDiffs))
	if mergePrev.ElapsedMs >= 500 {
		t.Fatalf("expected PreviewStagingToMasterMerge < 500 ms against 71,567-plot BECMaster, took %d ms", mergePrev.ElapsedMs)
	}
	if mergePrev.AddedCount != 1 || mergePrev.ConflictCount != 1 || len(mergePrev.CustodianProtectedDiffs) == 0 {
		t.Fatalf("unexpected merge preview summary: added=%d conflict=%d protected=%d",
			mergePrev.AddedCount, mergePrev.ConflictCount, len(mergePrev.CustodianProtectedDiffs))
	}

	// Selectively accept Env.SiteNotes + added Veg row MYQASPP on targetEditPlot while KEEPING target on custodian-protected Admin.BECSiteUnit and other historical fields!
	fieldResolutions := map[string]string{
		targetEditPlot + "::Env::SiteNotes":        cloudsync.MergeResolutionAcceptSource,
		targetEditPlot + "::Veg::Species=MYQASPP|Layer=6": cloudsync.MergeResolutionAcceptSource,
		targetEditPlot + "::Admin::BECSiteUnit":    cloudsync.MergeResolutionKeepTarget,
	}

	mergeRes, err := mgr.ExecuteStagingToMasterMerge(ctx, cloudsync.StagingMergeExecuteRequest{
		SubmissionID:              subRes.SubmissionID,
		ExpectedPreviewDigest:     mergePrev.PreviewDigest,
		DefaultConflictResolution: cloudsync.MergeResolutionKeepTarget,
		FieldResolutions:          fieldResolutions,
		ReviewerNotes:             "Accepted SiteNotes + MYQASPP + new plot " + leadingZeroPlot + "; kept Master BECSiteUnit",
		Actor:                     "Will MacKenzie",
	})
	if err != nil {
		t.Fatalf("ExecuteStagingToMasterMerge failed: %v", err)
	}
	createdMergeTxID = mergeRes.MergeTxID
	allocatedTestChildIDs = append(allocatedTestChildIDs, mergeRes.AllocatedChildIDs...)
	if mergeRes.AddedPlots != 1 || mergeRes.UpdatedPlots != 1 || mergeRes.AuditRowsWritten == 0 || len(mergeRes.AllocatedChildIDs) == 0 {
		t.Fatalf("unexpected StagingMergeResult: %+v", mergeRes)
	}

	// Verify byte-exact leading-zero PlotNumber '00338' exists in becmaster.env and '338' was NOT conflated,
	// SiteNotes updated, and custodian-protected BECSiteUnit preserved!
	var mergedNotes string
	var mergedBEC *string
	if err := adminPool.QueryRow(ctx, `
		SELECT e."SiteNotes", a."BECSiteUnit"
		FROM becmaster.env e JOIN becmaster.admin a ON a."Plot" = e."PlotNumber"
		WHERE e."PlotNumber" = $1
	`, targetEditPlot).Scan(&mergedNotes, &mergedBEC); err != nil {
		t.Fatal(err)
	}
	if mergedNotes != "Updated field notes from contributor CWH QA" {
		t.Fatalf("expected updated SiteNotes on %s, got %q", targetEditPlot, mergedNotes)
	}
	if (mergedBEC == nil) != (origBECSiteUnit == nil) || (mergedBEC != nil && *mergedBEC != *origBECSiteUnit) {
		t.Fatalf("expected custodian-protected BECSiteUnit preserved (%v), got %v", origBECSiteUnit, mergedBEC)
	}

	var exactLeadingZeroExists bool
	if err := adminPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM becmaster.env WHERE "PlotNumber" = $1)`, leadingZeroPlot).Scan(&exactLeadingZeroExists); err != nil || !exactLeadingZeroExists {
		t.Fatalf("expected byte-exact leading-zero plot %q in becmaster.env", leadingZeroPlot)
	}

	// Verify cryptographic hash chain on becmaster.audit
	chainVer, err := mgr.VerifyBECMasterAuditChain(ctx, 200)
	if err != nil || !chainVer.Valid || chainVer.VerifiedRows == 0 {
		t.Fatalf("VerifyBECMasterAuditChain failed: res=%+v err=%v", chainVer, err)
	}

	// Reject the second submission with notes
	rejSummary, err := mgr.RejectStagingSubmission(ctx, cloudsync.RejectSubmissionRequest{
		SubmissionID:  subResReject.SubmissionID,
		ReviewerNotes: "Superseded by merged CWH submission",
		Actor:         "Will MacKenzie",
	})
	if err != nil || rejSummary.Status != "rejected" {
		t.Fatalf("RejectStagingSubmission failed: %+v err=%v", rejSummary, err)
	}

	// Cut a dated BECMaster snapshot + verify download_log
	snapRec, err := mgr.CreateBECMasterSnapshot(ctx, svc, activeCtxID, cloudsync.CreateSnapshotRequest{
		SnapshotTag: testSnapshotTag,
		Notes:       "Wave 4 live qualification snapshot",
	}, nil)
	if err != nil || snapRec.PlotCount != baselinePlotCount+1 {
		t.Fatalf("CreateBECMasterSnapshot failed: rec=%+v err=%v", snapRec, err)
	}

	dlLogs, err := mgr.ListBECMasterDownloadLog(ctx, 10)
	if err != nil || len(dlLogs) == 0 {
		t.Fatalf("expected becmaster.download_log entries, got len=%d err=%v", len(dlLogs), err)
	}
}
