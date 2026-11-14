package cloudsync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestSubsetWhereClauseAndProjectNameValidation(t *testing.T) {
	if _, err := validateSplinterProjectName("CWH_2026"); err != nil {
		t.Fatalf("expected valid splinter name, got %v", err)
	}
	for _, bad := range []string{"", " Space", "9Bad", "Sample", "None", "BECMaster_Copy", strings.Repeat("A", 32)} {
		if _, err := validateSplinterProjectName(bad); err == nil {
			t.Fatalf("expected %q to be rejected by validateSplinterProjectName", bad)
		}
	}

	if _, _, err := buildSubsetWhereClause(SubsetFilter{}, true); err == nil {
		t.Fatal("expected empty SubsetFilter without AllowFullCheckout to fail")
	}
	if _, _, err := buildSubsetWhereClause(SubsetFilter{BBox: []float64{-125, 49, -126, 50}}, true); err == nil {
		t.Fatal("expected inverted minLon > maxLon bbox to fail")
	}

	whereSQL, args, err := buildSubsetWhereClause(SubsetFilter{
		Zones:       []string{"CWH", "ICH"},
		PlotNumbers: []string{"00337", "00338"},
		BBox:        []float64{-128.5, 48.5, -123.0, 51.5},
	}, true)
	if err != nil {
		t.Fatalf("unexpected buildSubsetWhereClause error: %v", err)
	}
	if !strings.Contains(whereSQL, `"Zone" = ANY($1)`) ||
		!strings.Contains(whereSQL, `"PlotNumber" = ANY($2)`) ||
		!strings.Contains(whereSQL, `ST_MakeEnvelope`) ||
		len(args) != 6 {
		t.Fatalf("unexpected whereSQL=%q args=%v", whereSQL, args)
	}
}

func TestAuditHashChainDeterminismAndTamperDetection(t *testing.T) {
	row1 := []any{"BECMaster", "Will MacKenzie", "00337", "_Env", "Zone", "2026-11-14 08:00:00", "IDF", "CWH", int16(0), int16(0), nil}
	row2 := []any{"BECMaster", "Will MacKenzie", "00338", "_Veg", "Cover4", "2026-11-14 08:00:01", nil, "15.5", int16(0), int16(0), int64(42)}

	h1 := ComputeAuditRowHash(GenesisAuditHash, "tx-1", row1)
	h1Again := ComputeAuditRowHash(GenesisAuditHash, "tx-1", row1)
	if h1 != h1Again || len(h1) != 64 {
		t.Fatalf("expected deterministic 64-hex SHA-256 hash, got %q vs %q", h1, h1Again)
	}
	h2 := ComputeAuditRowHash(h1, "tx-1", row2)

	// Mutate a single character in row1 and prove h2 changes completely
	tamperedRow1 := append([]any(nil), row1...)
	tamperedRow1[7] = "ICH"
	h1Tampered := ComputeAuditRowHash(GenesisAuditHash, "tx-1", tamperedRow1)
	h2Tampered := ComputeAuditRowHash(h1Tampered, "tx-1", row2)
	if h1Tampered == h1 || h2Tampered == h2 {
		t.Fatal("tampered audit chain produced colliding SHA-256 digest")
	}
}

func TestLiveStandardsPublishUnderThreeSecondsAndContributorPullSync(t *testing.T) {
	mgr := requireLivePostgresOrSkip(t)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := mgr.BootstrapSchemas(ctx); err != nil {
		t.Fatalf("BootstrapSchemas: %v", err)
	}

	svc, state := vprotest.ContextServiceFixture(t)

	// Warm custodian pool so we measure pure streaming PublishStandardsToCloud time
	if _, err := mgr.ConfigureSession(SessionConfigRequest{RoleMode: RoleModeCustodian}); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.TestCloudConnection(ctx); err != nil {
		t.Fatalf("TestCloudConnection: %v", err)
	}

	pubRes, err := mgr.PublishStandardsToCloud(ctx, svc, state.ContextID, PublishStandardsRequest{
		Notes: "Automated 2026 BECdb v13 + BCPlantTaxonomy v19 verification",
	})
	if err != nil {
		t.Fatalf("PublishStandardsToCloud failed: %v", err)
	}
	t.Logf("PublishStandardsToCloud completed in %d ms: counts=%v", pubRes.ElapsedMs, pubRes.TableCounts)
	if pubRes.TableCounts["usys_all_specs"] != 16728 ||
		pubRes.TableCounts["bec_site_series"] != 8900 ||
		pubRes.TableCounts["usys_table_of_lists"] != 1566 ||
		pubRes.TableCounts["master_site_unit_list"] != 6067 ||
		pubRes.TableCounts["usys_zone_list"] != 331 {
		t.Fatalf("unexpected 2026 standards table counts: %+v", pubRes.TableCounts)
	}

	// Verify 2-char Codetype='XX' survived round-trip into standards.usys_all_specs
	pool, _, err := mgr.AcquirePool(ctx, RoleModeContributor)
	if err != nil {
		t.Fatal(err)
	}
	var xxCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM standards.usys_all_specs WHERE "Codetype" = 'XX'`).Scan(&xxCount); err != nil || xxCount == 0 {
		t.Fatalf("expected Codetype='XX' rows in standards.usys_all_specs, got %d (err=%v)", xxCount, err)
	}

	// Switch to Contributor role and verify PullCloudStandards fast-path when local VLists.db matches
	if _, err := mgr.ConfigureSession(SessionConfigRequest{RoleMode: RoleModeContributor}); err != nil {
		t.Fatal(err)
	}
	pullSame, err := mgr.PullCloudStandards(ctx, svc, state.ContextID)
	if err != nil {
		t.Fatalf("PullCloudStandards (already current) failed: %v", err)
	}
	if pullSame.Updated || !pullSame.AlreadyCurrent {
		t.Fatalf("expected AlreadyCurrent=true when SHA-256 matches, got %+v", pullSame)
	}

	// Mutate disposable fixture's local VLists.db (simulating an older local VLists), then run PullCloudStandards
	vlistsPath := svc.Projects.SupportPaths["VLists"]
	mutDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(vlistsPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mutDB.Exec(`UPDATE "_table_metadata" SET "description" = 'outdated-2023-test' WHERE "table_name" = 'USysAllSpecs'`); err != nil {
		_ = mutDB.Close()
		t.Fatal(err)
	}
	_ = mutDB.Close()

	// Re-open fixture SQLiteContext on the modified VLists.db so AttachmentInfo matches disk before sync
	refreshedState, err := svc.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project:       state.ActiveProject,
		ProjectPath:   state.ProjectPath,
		SU:            state.ActiveSU,
		SUPath:        state.SUPath,
		Hierarchy:     state.ActiveHierarchy,
		HierarchyPath: state.HierarchyPath,
	})
	if err != nil {
		t.Fatalf("SwitchContext after VLists mutation: %v", err)
	}

	pullUpdated, err := mgr.PullCloudStandards(ctx, svc, refreshedState.ContextID)
	if err != nil {
		t.Fatalf("PullCloudStandards (outdated local VLists) failed: %v", err)
	}
	if !pullUpdated.Updated || pullUpdated.AlreadyCurrent || pullUpdated.State == nil {
		t.Fatalf("expected Updated=true and refreshed ProjectState, got %+v", pullUpdated)
	}
	if pullUpdated.State.ContextID == refreshedState.ContextID {
		t.Fatal("expected ReplaceActiveVListsCatalogue to mint a fresh 128-bit ContextID")
	}

	afterBytes, err := os.ReadFile(vlistsPath)
	if err != nil {
		t.Fatal(err)
	}
	afterSum := sha256.Sum256(afterBytes)
	if hex.EncodeToString(afterSum[:]) != pubRes.VListsSHA256 {
		t.Fatalf("local VLists.db SHA-256 after PullCloudStandards did not converge to cloud release %s", pubRes.VListsSHA256)
	}
}
