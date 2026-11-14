package cloudsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVP08AndStandardsColumnCountsAndCalibration(t *testing.T) {
	if len(VP08EnvColumns) != 112 {
		t.Fatalf("expected 112 VP08 Env columns (PlotNumber + 111), got %d", len(VP08EnvColumns))
	}
	if len(VP08AdminColumns) != 20 {
		t.Fatalf("expected 20 VP08 Admin columns (Plot + 19), got %d", len(VP08AdminColumns))
	}
	if len(VP08VegColumns) != 44 {
		t.Fatalf("expected 44 VP08 Veg columns, got %d", len(VP08VegColumns))
	}
	if len(VP08HumusColumns) != 18 {
		t.Fatalf("expected 18 VP08 Humus columns, got %d", len(VP08HumusColumns))
	}
	if len(VP08MineralColumns) != 28 {
		t.Fatalf("expected 28 VP08 Mineral columns, got %d", len(VP08MineralColumns))
	}
	if len(VP08OtherColumns) != 11 {
		t.Fatalf("expected 11 VP08 Other columns, got %d", len(VP08OtherColumns))
	}
	if len(VP08MetadataColumns) != 75 {
		t.Fatalf("expected 75 VP08 Metadata columns (ProjectID + 74), got %d", len(VP08MetadataColumns))
	}
	if len(VP08AuditColumns) != 11 {
		t.Fatalf("expected 11 VP08 Audit columns, got %d", len(VP08AuditColumns))
	}

	// IEEE-754 float32 < 100 calibration
	if _, ok := NarrowSingleCover(99.999996); !ok {
		t.Fatal("expected 99.999996 to narrow to < 100 in float32 and pass")
	}
	if _, ok := NarrowSingleCover(99.999999); ok {
		t.Fatal("expected 99.999999 to narrow to 100.0 in float32 and reject")
	}

	// Access BOOLEAN canonicalization (-1 / 0)
	boolCol := bCol("Flag")
	if v, err := CanonicalizeSQLiteToPG(boolCol, int64(1)); err != nil || v != int16(-1) {
		t.Fatalf("expected SQLite 1 -> PG int16(-1), got %v (%v)", v, err)
	}
	if v, err := CanonicalizeSQLiteToPG(boolCol, int64(-1)); err != nil || v != int16(-1) {
		t.Fatalf("expected SQLite -1 -> PG int16(-1), got %v (%v)", v, err)
	}
	if v, err := CanonicalizeSQLiteToPG(boolCol, int64(0)); err != nil || v != int16(0) {
		t.Fatalf("expected SQLite 0 -> PG int16(0), got %v (%v)", v, err)
	}
	if back := CanonicalizePGToSQLite(boolCol, int16(-1)); back != int64(-1) {
		t.Fatalf("expected PG -1 -> SQLite int64(-1), got %v", back)
	}

	// LMH25 soil depth directional signs
	u6, l0, l10 := 6.0, 0.0, 10.0
	if err := ValidateLMH25HorizonDepth("Humus", "Fm", &u6, &l0); err != nil {
		t.Fatalf("upland Fm 6..0 should pass: %v", err)
	}
	if err := ValidateLMH25HorizonDepth("Humus", "Fm", &l0, &u6); err == nil {
		t.Fatal("upland Fm 0..6 should fail LMH25 sign check")
	}
	if err := ValidateLMH25HorizonDepth("Humus", "Of1", &l0, &l10); err != nil {
		t.Fatalf("wetland Of1 0..10 should pass: %v", err)
	}
	if err := ValidateLMH25HorizonDepth("Mineral", "Bm1", &l10, &u6); err == nil {
		t.Fatal("mineral Bm1 10..6 should fail LMH25 sign check")
	}
}

func TestOfflineConfigResolutionAndSecretRedaction(t *testing.T) {
	emptyDir := t.TempDir()
	mgr := NewManager(emptyDir, func(string) (string, bool) { return "", false })
	defer mgr.Close()

	st, err := mgr.GetConfigStatus()
	if err != nil {
		t.Fatal(err)
	}
	if st.Configured || st.ConfigSource != "unconfigured" || st.EnvFileDetected {
		t.Fatalf("expected unconfigured offline status, got %+v", st)
	}

	envContent := strings.Join([]string{
		"# Sample test env (not real secrets)",
		"VPRO_PG_HOST=db.example.internal",
		"VPRO_PG_PORT=5433",
		"VPRO_PG_DBNAME=becmaster",
		"VPRO_PG_SSLMODE=require",
		"VPRO_PG_USER=test_contrib",
		`VPRO_PG_PASSWORD="My$ecretUserPass!99"`,
		"VPRO_PG_ADMIN_USER=test_custodian",
		`VPRO_PG_ADMIN_PASSWORD='My$ecretAdminPass!88'`,
	}, "\n")
	if err := os.WriteFile(filepath.Join(emptyDir, "becmaster.env"), []byte(envContent), 0o600); err != nil {
		t.Fatal(err)
	}

	st2, err := mgr.GetConfigStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st2.Configured || st2.ConfigSource != "becmaster.env" || !st2.HasContributorCreds || !st2.HasCustodianCreds || st2.Port != 5433 {
		t.Fatalf("unexpected becmaster.env status: %+v", st2)
	}

	creds, err := mgr.ResolveCredentials()
	if err != nil {
		t.Fatal(err)
	}
	verifyFullCreds := *creds
	verifyFullCreds.SSLMode = "verify-full"
	vfCfg, err := buildConnConfigForRole(&verifyFullCreds, RoleModeContributor)
	if err != nil {
		t.Fatalf("buildConnConfigForRole(verify-full): %v", err)
	}
	if vfCfg.TLSConfig == nil || vfCfg.TLSConfig.ServerName != "db.example.internal" {
		t.Fatalf("expected TLSConfig.ServerName == db.example.internal, got %+v", vfCfg.TLSConfig)
	}

	rawErr := "dial tcp 203.0.113.42:5433 (db.example.internal): auth failed for postgres://test_contrib:My$ecretUserPass!99@db.example.internal:5433/becmaster password=My$ecretAdminPass!88"
	scrubbed := RedactSecrets(rawErr, creds)
	for _, forbidden := range []string{"My$ecretUserPass!99", "My$ecretAdminPass!88", "db.example.internal", "203.0.113.42"} {
		if strings.Contains(scrubbed, forbidden) {
			t.Fatalf("RedactSecrets leaked %q in %q", forbidden, scrubbed)
		}
	}
}

func requireLivePostgresOrSkip(t *testing.T) *Manager {
	t.Helper()
	mgr := NewManager("", os.LookupEnv)
	st, err := mgr.GetConfigStatus()
	if err != nil || !st.Configured || !st.HasCustodianCreds || !st.HasContributorCreds {
		mgr.Close()
		t.Skip("live PostgreSQL credentials (VPRO_PG_* or becmaster.env) not configured; skipping live PG test")
	}
	return mgr
}

func TestLivePostgresIdempotentBootstrapAndRoleIsolation(t *testing.T) {
	mgr := requireLivePostgresOrSkip(t)
	defer mgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Snapshot public schema table count before bootstrap to prove zero non-VPro mutation
	adminConn, creds, err := mgr.OpenDedicatedConn(ctx, RoleModeCustodian)
	if err != nil {
		t.Fatalf("custodian connect failed: %v", err)
	}
	var publicCountBefore int
	if err := adminConn.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m')
	`).Scan(&publicCountBefore); err != nil {
		adminConn.Close(ctx)
		t.Fatalf("count public relations: %v", err)
	}
	adminConn.Close(ctx)

	// Run bootstrap twice to prove 100% idempotency
	res1, err := mgr.BootstrapSchemas(ctx)
	if err != nil {
		t.Fatalf("first BootstrapSchemas failed: %v", err)
	}
	if !res1.RoleGrantsApplied || !res1.ContributorRoleSet {
		t.Fatalf("expected role grants applied on first bootstrap: %+v", res1)
	}

	res2, err := mgr.BootstrapSchemas(ctx)
	if err != nil {
		t.Fatalf("second idempotent BootstrapSchemas failed: %v", err)
	}
	if len(res2.AppliedMigrations) != 0 {
		t.Fatalf("expected 0 newly applied migrations on idempotent rerun, got %v", res2.AppliedMigrations)
	}

	// Verify public schema untouched
	adminConn2, _, err := mgr.OpenDedicatedConn(ctx, RoleModeCustodian)
	if err != nil {
		t.Fatal(err)
	}
	defer adminConn2.Close(ctx)
	var publicCountAfter int
	if err := adminConn2.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m')
	`).Scan(&publicCountAfter); err != nil {
		t.Fatal(err)
	}
	if publicCountAfter != publicCountBefore {
		t.Fatalf("public schema relation count changed (%d -> %d)", publicCountBefore, publicCountAfter)
	}

	// Verify Contributor (VPRO_PG_USER) RBAC & RLS enforcement at the PostgreSQL database engine level!
	userConn, _, err := mgr.OpenDedicatedConn(ctx, RoleModeContributor)
	if err != nil {
		t.Fatalf("contributor connect failed: %v", err)
	}
	defer userConn.Close(ctx)

	// 1. Contributor MUST be rejected when attempting UPDATE or DELETE on becmaster.env or standards.usys_all_specs
	for _, forbiddenSQL := range []string{
		`DELETE FROM becmaster.env WHERE "PlotNumber" = '__NEVER__'`,
		`UPDATE becmaster.env SET "Zone" = 'IDF' WHERE "PlotNumber" = '__NEVER__'`,
		`DELETE FROM standards.usys_all_specs WHERE "Code" = '__NEVER__'`,
		`UPDATE staging.submissions SET status = 'approved' WHERE submission_id = '00000000-0000-0000-0000-000000000000'`,
	} {
		if _, err := userConn.Exec(ctx, forbiddenSQL); err == nil {
			t.Fatalf("SECURITY FAILURE: contributor role was permitted to execute %s", forbiddenSQL)
		}
	}

	// 2. Contributor MUST be rejected when attempting to insert non-pending or non-compliant staging submission
	tx, err := userConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, badStatusErr := tx.Exec(ctx, `
		INSERT INTO staging.submissions (
			submission_id, project_name, submitted_by, db_role, status,
			source_db_sha256, compliance_passed, compliance_report, user_species_manifest
		) VALUES (
			'00000000-0000-0000-0000-000000000001', 'BadProj', 'tester', $1, 'merged',
			'abc', TRUE, '{}'::jsonb, '{}'::jsonb
		)
	`, creds.User)
	_ = tx.Rollback(ctx)
	if badStatusErr == nil {
		t.Fatal("SECURITY FAILURE: contributor inserted status='merged' into staging.submissions")
	}

	tx2, err := userConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, badCompErr := tx2.Exec(ctx, `
		INSERT INTO staging.submissions (
			submission_id, project_name, submitted_by, db_role, status,
			source_db_sha256, compliance_passed, compliance_report, user_species_manifest
		) VALUES (
			'00000000-0000-0000-0000-000000000002', 'BadProj', 'tester', $1, 'pending',
			'abc', FALSE, '{}'::jsonb, '{}'::jsonb
		)
	`, creds.User)
	_ = tx2.Rollback(ctx)
	if badCompErr == nil {
		t.Fatal("SECURITY FAILURE: contributor inserted compliance_passed=FALSE into staging.submissions")
	}

	// 2b. Contributor MUST be rejected by RLS when attempting to insert into staging.env for a non-owned/non-pending submission_id
	tx3, err := userConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, badChildErr := tx3.Exec(ctx, `
		INSERT INTO staging.env (submission_id, "PlotNumber")
		VALUES ('00000000-0000-0000-0000-000000000099', '9999999')
	`)
	_ = tx3.Rollback(ctx)
	if badChildErr == nil {
		t.Fatal("SECURITY FAILURE: contributor inserted row into staging.env for unauthorized submission_id")
	}

	// 3. TestCloudConnection works for both roles without leaking secrets
	if _, err := mgr.ConfigureSession(SessionConfigRequest{RoleMode: RoleModeContributor}); err != nil {
		t.Fatal(err)
	}
	uRep, err := mgr.TestCloudConnection(ctx)
	if err != nil {
		t.Fatalf("TestCloudConnection(contributor) failed: %v", err)
	}
	if !uRep.Connected || !uRep.SchemasBootstrapped || !uRep.CanReadMaster || !uRep.CanSubmitStaging || uRep.CanMergeMaster {
		t.Fatalf("unexpected contributor capabilities: %+v", uRep)
	}

	if _, err := mgr.ConfigureSession(SessionConfigRequest{RoleMode: RoleModeCustodian}); err != nil {
		t.Fatal(err)
	}
	aRep, err := mgr.TestCloudConnection(ctx)
	if err != nil {
		t.Fatalf("TestCloudConnection(custodian) failed: %v", err)
	}
	if !aRep.Connected || !aRep.SchemasBootstrapped || !aRep.CanReadMaster || !aRep.CanMergeMaster {
		t.Fatalf("unexpected custodian capabilities: %+v", aRep)
	}
}
