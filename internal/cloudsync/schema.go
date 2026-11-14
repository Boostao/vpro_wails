package cloudsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const CurrentSchemaVersion = 1

// SchemaBootstrapResult summarizes idempotent schema migration and role grant execution.
type SchemaBootstrapResult struct {
	SchemaVersion      int      `json:"schemaVersion"`
	AppliedMigrations  []string `json:"appliedMigrations"`
	SchemasVerified    []string `json:"schemasVerified"`
	PostGISEnabled     bool     `json:"postgisEnabled"`
	PGCryptoEnabled    bool     `json:"pgcryptoEnabled"`
	RoleGrantsApplied  bool     `json:"roleGrantsApplied"`
	ContributorRoleSet bool     `json:"contributorRoleSet"`
	BootstrappedAt     string   `json:"bootstrappedAt"`
}

func buildMasterTableDDL(table string, cols []ColumnSpec, pkCols []string, extraConstraints ...string) string {
	defs := make([]string, 0, len(cols)+len(extraConstraints)+1)
	for _, c := range cols {
		defs = append(defs, "  "+columnDDL(c))
	}
	if len(pkCols) > 0 {
		quotedPK := make([]string, len(pkCols))
		for i, pk := range pkCols {
			quotedPK[i] = quotePGIdent(pk)
		}
		defs = append(defs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(quotedPK, ", ")))
	}
	for _, extra := range extraConstraints {
		defs = append(defs, "  "+extra)
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS becmaster.%s (\n%s\n);", quotePGIdent(table), strings.Join(defs, ",\n"))
}

func buildStagingTableDDL(table string, cols []ColumnSpec, pkCols []string, fkToEnv bool) string {
	defs := make([]string, 0, len(cols)+3)
	if fkToEnv {
		defs = append(defs, `  submission_id UUID NOT NULL`)
	} else {
		defs = append(defs, `  submission_id UUID NOT NULL REFERENCES staging.submissions(submission_id) ON DELETE CASCADE`)
	}
	for _, c := range cols {
		defs = append(defs, "  "+columnDDL(c))
	}
	if len(pkCols) > 0 {
		quotedPK := make([]string, 0, len(pkCols)+1)
		quotedPK = append(quotedPK, "submission_id")
		for _, pk := range pkCols {
			quotedPK = append(quotedPK, quotePGIdent(pk))
		}
		defs = append(defs, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(quotedPK, ", ")))
	}
	if fkToEnv {
		plotCol := "PlotNumber"
		if table == "admin" {
			plotCol = "Plot"
		}
		defs = append(defs, fmt.Sprintf(
			`  FOREIGN KEY (submission_id, %s) REFERENCES staging.env(submission_id, "PlotNumber") ON DELETE CASCADE`,
			quotePGIdent(plotCol),
		))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS staging.%s (\n%s\n);", quotePGIdent(table), strings.Join(defs, ",\n"))
}

func buildStandardsTableDDL(table string, cols []ColumnSpec) string {
	defs := make([]string, 0, len(cols)+1)
	defs = append(defs, "  row_seq BIGSERIAL PRIMARY KEY")
	for _, c := range cols {
		defs = append(defs, "  "+columnDDL(c))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS standards.%s (\n%s\n);", quotePGIdent(table), strings.Join(defs, ",\n"))
}

func ensureIsolatedSchema(ctx context.Context, conn *pgx.Conn, schemaName string) error {
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schemaName).Scan(&exists); err != nil {
		return err
	}
	if exists {
		var canCreate bool
		if err := conn.QueryRow(ctx, `SELECT has_schema_privilege(current_user, $1, 'CREATE')`, schemaName).Scan(&canCreate); err != nil {
			return err
		}
		if !canCreate {
			return fmt.Errorf("custodian role lacks CREATE privilege on existing schema %q", schemaName)
		}
		return nil
	}
	var dbCreate bool
	if err := conn.QueryRow(ctx, `SELECT has_database_privilege(current_user, current_database(), 'CREATE')`).Scan(&dbCreate); err != nil {
		return err
	}
	if !dbCreate {
		return fmt.Errorf("custodian role lacks CREATE privilege on database to initialize schema %q; ask a PostgreSQL DBA/superuser to run GRANT CREATE ON DATABASE <db> TO <admin_role>", schemaName)
	}
	_, err := conn.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+quotePGIdent(schemaName))
	return err
}

func inspectExtensions(ctx context.Context, conn *pgx.Conn) (hasPostGIS bool, hasPGCrypto bool, err error) {
	rows, err := conn.Query(ctx, `SELECT extname FROM pg_extension WHERE extname IN ('postgis', 'pgcrypto')`)
	if err != nil {
		return false, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, false, err
		}
		switch name {
		case "postgis":
			hasPostGIS = true
		case "pgcrypto":
			hasPGCrypto = true
		}
	}
	return hasPostGIS, hasPGCrypto, rows.Err()
}

// BootstrapSchemas runs idempotent versioned DDL migrations for becmaster, staging, and standards
// and applies contributor vs custodian role grants without touching public or any non-VPro schema.
func (m *Manager) BootstrapSchemas(ctx context.Context) (SchemaBootstrapResult, error) {
	creds, err := m.ResolveCredentials()
	if err != nil {
		return SchemaBootstrapResult{}, err
	}
	cfg, err := buildConnConfigForRole(creds, RoleModeCustodian)
	if err != nil {
		return SchemaBootstrapResult{}, err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return SchemaBootstrapResult{}, wrapRedactedError("connect as custodian for schema bootstrap", err, creds)
	}
	defer conn.Close(ctx)

	res, err := bootstrapSchemasOnConn(ctx, conn, creds.User, creds.AdminUser)
	if err != nil {
		return SchemaBootstrapResult{}, wrapRedactedError("bootstrap PostgreSQL schemas", err, creds)
	}
	return res, nil
}

func bootstrapSchemasOnConn(ctx context.Context, conn *pgx.Conn, contributorRole, custodianRole string) (SchemaBootstrapResult, error) {
	targetSchemas := []string{"becmaster", "staging", "standards"}
	for _, sName := range targetSchemas {
		if err := ensureIsolatedSchema(ctx, conn, sName); err != nil {
			return SchemaBootstrapResult{}, err
		}
	}

	hasPostGIS, hasPGCrypto, err := inspectExtensions(ctx, conn)
	if err != nil {
		return SchemaBootstrapResult{}, err
	}

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS becmaster.schema_migrations (
			version INT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`); err != nil {
		return SchemaBootstrapResult{}, err
	}

	applied := []string{}
	var v1Applied bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM becmaster.schema_migrations WHERE version = 1)`).Scan(&v1Applied); err != nil {
		return SchemaBootstrapResult{}, err
	}

	// Execute idempotent DDL statements inside a single atomic transaction so re-runs also self-heal missing objects.
	tx, err := conn.Begin(ctx)
	if err != nil {
		return SchemaBootstrapResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	stmts := buildV1MigrationStatements(hasPostGIS)
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return SchemaBootstrapResult{}, err
		}
	}

	if !v1Applied {
		if _, err := tx.Exec(ctx, `INSERT INTO becmaster.schema_migrations (version, name) VALUES (1, 'v1_becmaster_staging_standards') ON CONFLICT (version) DO NOTHING`); err != nil {
			return SchemaBootstrapResult{}, err
		}
		applied = append(applied, "1:v1_becmaster_staging_standards")
	}

	if err := tx.Commit(ctx); err != nil {
		return SchemaBootstrapResult{}, err
	}

	contributorApplied, err := ApplyRoleGrants(ctx, conn, contributorRole, custodianRole)
	if err != nil {
		return SchemaBootstrapResult{}, err
	}

	return SchemaBootstrapResult{
		SchemaVersion:      CurrentSchemaVersion,
		AppliedMigrations:  applied,
		SchemasVerified:    targetSchemas,
		PostGISEnabled:     hasPostGIS,
		PGCryptoEnabled:    hasPGCrypto,
		RoleGrantsApplied:  true,
		ContributorRoleSet: contributorApplied,
		BootstrappedAt:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func buildV1MigrationStatements(hasPostGIS bool) []string {
	var stmts []string

	// 1. becmaster canonical VP08 production master tables
	stmts = append(stmts,
		buildMasterTableDDL("env", VP08EnvColumns, []string{"PlotNumber"}),
		`CREATE INDEX IF NOT EXISTS idx_becmaster_env_zone ON becmaster.env ("Zone");`,
		`CREATE INDEX IF NOT EXISTS idx_becmaster_env_subzone ON becmaster.env ("SubZone");`,
		`CREATE INDEX IF NOT EXISTS idx_becmaster_env_projectid ON becmaster.env ("ProjectID");`,
		`CREATE INDEX IF NOT EXISTS idx_becmaster_env_coords ON becmaster.env ("Longitude", "Latitude");`,
	)
	if hasPostGIS {
		stmts = append(stmts,
			`ALTER TABLE becmaster.env ADD COLUMN IF NOT EXISTS geom public.geometry(Point, 4326) GENERATED ALWAYS AS (
				CASE
					WHEN "Longitude" IS NOT NULL AND "Latitude" IS NOT NULL
					     AND "Longitude" >= -180.0 AND "Longitude" <= 180.0
					     AND "Latitude" >= -90.0 AND "Latitude" <= 90.0
					THEN public.ST_SetSRID(public.ST_MakePoint("Longitude", "Latitude"), 4326)
					ELSE NULL
				END
			) STORED;`,
			`CREATE INDEX IF NOT EXISTS idx_becmaster_env_geom ON becmaster.env USING GIST (geom);`,
		)
	}

	stmts = append(stmts,
		buildMasterTableDDL("admin", VP08AdminColumns, []string{"Plot"}, `FOREIGN KEY ("Plot") REFERENCES becmaster.env("PlotNumber") ON DELETE CASCADE`),
		buildMasterTableDDL("veg", VP08VegColumns, []string{"PlotNumber", "ID"}, `FOREIGN KEY ("PlotNumber") REFERENCES becmaster.env("PlotNumber") ON DELETE CASCADE`),
		`CREATE INDEX IF NOT EXISTS idx_becmaster_veg_species ON becmaster.veg ("Species");`,
		buildMasterTableDDL("humus", VP08HumusColumns, []string{"PlotNumber", "ID"}, `FOREIGN KEY ("PlotNumber") REFERENCES becmaster.env("PlotNumber") ON DELETE CASCADE`),
		buildMasterTableDDL("mineral", VP08MineralColumns, []string{"PlotNumber", "ID"}, `FOREIGN KEY ("PlotNumber") REFERENCES becmaster.env("PlotNumber") ON DELETE CASCADE`),
		buildMasterTableDDL("other", VP08OtherColumns, []string{"PlotNumber", "ID"}, `FOREIGN KEY ("PlotNumber") REFERENCES becmaster.env("PlotNumber") ON DELETE CASCADE`),
		`CREATE TABLE IF NOT EXISTS becmaster.child_identity (
			child_table TEXT NOT NULL,
			id BIGINT NOT NULL,
			PRIMARY KEY (child_table, id)
		);`,
		`CREATE TABLE IF NOT EXISTS becmaster.child_identity_watermark (
			child_table TEXT PRIMARY KEY,
			max_id BIGINT NOT NULL
		);`,
		buildMasterTableDDL("metadata", VP08MetadataColumns, []string{"ProjectID"}),
	)

	// becmaster.audit (11 VP08 columns + merge_tx_id, prev_hash, row_hash)
	auditDefs := []string{"  audit_seq BIGSERIAL PRIMARY KEY"}
	for _, c := range VP08AuditColumns {
		auditDefs = append(auditDefs, "  "+columnDDL(c))
	}
	auditDefs = append(auditDefs,
		"  merge_tx_id UUID",
		"  prev_hash TEXT NOT NULL",
		"  row_hash TEXT NOT NULL",
	)
	stmts = append(stmts,
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS becmaster.audit (\n%s\n);", strings.Join(auditDefs, ",\n")),
		`CREATE INDEX IF NOT EXISTS idx_becmaster_audit_plot ON becmaster.audit ("PlotNumber");`,
		`CREATE INDEX IF NOT EXISTS idx_becmaster_audit_mergetx ON becmaster.audit (merge_tx_id);`,
		`CREATE TABLE IF NOT EXISTS becmaster.merge_ledger (
			merge_tx_id UUID PRIMARY KEY,
			submission_id UUID,
			actor TEXT NOT NULL,
			merged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			source_sha256 TEXT NOT NULL,
			added_plots INT NOT NULL,
			updated_plots INT NOT NULL,
			summary_json JSONB NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS becmaster.snapshots (
			snapshot_tag TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL,
			plot_count INT NOT NULL,
			veg_count BIGINT NOT NULL,
			bec_version TEXT NOT NULL,
			taxonomy_version TEXT NOT NULL,
			manifest_sha256 TEXT NOT NULL,
			notes TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS becmaster.download_log (
			id BIGSERIAL PRIMARY KEY,
			downloaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			actor TEXT NOT NULL,
			role_tier TEXT NOT NULL,
			filter_json JSONB NOT NULL,
			format TEXT NOT NULL,
			plot_count INT NOT NULL
		);`,
		`CREATE OR REPLACE FUNCTION becmaster.enforce_download_log_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			NEW.downloaded_at := COALESCE(NEW.downloaded_at, now());
			IF NEW.actor IS NULL OR btrim(NEW.actor) = '' THEN
				NEW.actor := current_user;
			END IF;
			RETURN NEW;
		END;
		$$;`,
		`DROP TRIGGER IF EXISTS trg_becmaster_download_log_insert ON becmaster.download_log;`,
		`CREATE TRIGGER trg_becmaster_download_log_insert
			BEFORE INSERT ON becmaster.download_log
			FOR EACH ROW EXECUTE FUNCTION becmaster.enforce_download_log_insert();`,
	)

	// 2. staging contributor upload queue & keyed VP08 tables
	stmts = append(stmts,
		`CREATE TABLE IF NOT EXISTS staging.submissions (
			submission_id UUID PRIMARY KEY,
			project_name TEXT NOT NULL,
			project_id TEXT,
			submitted_by TEXT NOT NULL,
			db_role TEXT NOT NULL,
			submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'merged')),
			plot_count INT NOT NULL DEFAULT 0,
			veg_row_count INT NOT NULL DEFAULT 0,
			source_db_sha256 TEXT NOT NULL,
			compliance_passed BOOLEAN NOT NULL,
			compliance_report JSONB NOT NULL,
			user_species_manifest JSONB NOT NULL,
			lump_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
			su_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
			submitter_notes TEXT,
			base_snapshot_tag TEXT,
			reviewer_notes TEXT,
			reviewed_by TEXT,
			reviewed_at TIMESTAMPTZ,
			merged_tx_id UUID
		);`,
		`CREATE INDEX IF NOT EXISTS idx_staging_submissions_status ON staging.submissions (status, submitted_at DESC);`,
		buildStagingTableDDL("env", VP08EnvColumns, []string{"PlotNumber"}, false),
		buildStagingTableDDL("admin", VP08AdminColumns, []string{"Plot"}, true),
		buildStagingTableDDL("veg", VP08VegColumns, []string{"PlotNumber", "ID"}, true),
		buildStagingTableDDL("humus", VP08HumusColumns, []string{"PlotNumber", "ID"}, true),
		buildStagingTableDDL("mineral", VP08MineralColumns, []string{"PlotNumber", "ID"}, true),
		buildStagingTableDDL("other", VP08OtherColumns, []string{"PlotNumber", "ID"}, true),
		buildStagingTableDDL("metadata", VP08MetadataColumns, []string{"ProjectID"}, false),
	)

	stgAuditDefs := []string{
		"  audit_seq BIGSERIAL PRIMARY KEY",
		"  submission_id UUID NOT NULL REFERENCES staging.submissions(submission_id) ON DELETE CASCADE",
	}
	for _, c := range VP08AuditColumns {
		stgAuditDefs = append(stgAuditDefs, "  "+columnDDL(c))
	}
	stmts = append(stmts,
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS staging.audit (\n%s\n);", strings.Join(stgAuditDefs, ",\n")),
		`CREATE INDEX IF NOT EXISTS idx_staging_audit_sub ON staging.audit (submission_id);`,
		`CREATE OR REPLACE FUNCTION staging.enforce_submission_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		DECLARE
			is_table_owner BOOLEAN;
		BEGIN
			NEW.db_role := current_user;
			NEW.submitted_at := COALESCE(NEW.submitted_at, now());
			SELECT pg_has_role(current_user, c.relowner, 'USAGE')
			INTO is_table_owner
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'staging' AND c.relname = 'submissions';

			IF NOT COALESCE(is_table_owner, FALSE) THEN
				IF NEW.status <> 'pending' THEN
					RAISE EXCEPTION 'contributor submissions must start with status = pending' USING ERRCODE = '42501';
				END IF;
				IF NEW.compliance_passed IS NOT TRUE THEN
					RAISE EXCEPTION 'contributor submissions require compliance_passed = TRUE' USING ERRCODE = '42501';
				END IF;
				NEW.reviewed_by := NULL;
				NEW.reviewed_at := NULL;
				NEW.merged_tx_id := NULL;
			END IF;
			RETURN NEW;
		END;
		$$;`,
		`DROP TRIGGER IF EXISTS trg_staging_submission_insert ON staging.submissions;`,
		`CREATE TRIGGER trg_staging_submission_insert
			BEFORE INSERT ON staging.submissions
			FOR EACH ROW EXECUTE FUNCTION staging.enforce_submission_insert();`,
		`CREATE OR REPLACE FUNCTION staging.enforce_staging_child_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		DECLARE
			is_table_owner BOOLEAN;
			sub_ok BOOLEAN;
			cache_key TEXT;
		BEGIN
			cache_key := NEW.submission_id::text || ':' || current_user;
			IF COALESCE(current_setting('vpro.verified_sub_id', true), '') = cache_key THEN
				RETURN NEW;
			END IF;
			SELECT pg_has_role(current_user, c.relowner, 'USAGE')
			INTO is_table_owner
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'staging' AND c.relname = 'submissions';

			IF NOT COALESCE(is_table_owner, FALSE) THEN
				SELECT EXISTS (
					SELECT 1 FROM staging.submissions s
					WHERE s.submission_id = NEW.submission_id
					  AND s.status = 'pending'
					  AND s.db_role = current_user
				) INTO sub_ok;
				IF NOT COALESCE(sub_ok, FALSE) THEN
					RAISE EXCEPTION 'contributor child row insert requires owned pending submission_id' USING ERRCODE = '42501';
				END IF;
			END IF;
			PERFORM set_config('vpro.verified_sub_id', cache_key, true);
			RETURN NEW;
		END;
		$$;`,
	)
	for _, childTbl := range []string{"env", "admin", "veg", "humus", "mineral", "other", "metadata", "audit"} {
		qTbl := quotePGIdent(childTbl)
		trgName := quotePGIdent("trg_staging_" + childTbl + "_insert")
		stmts = append(stmts,
			fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON staging.%s;", trgName, qTbl),
			fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON staging.%s FOR EACH ROW EXECUTE FUNCTION staging.enforce_staging_child_insert();", trgName, qTbl),
		)
	}

	// 3. standards versioned reference libraries (Schedule A 3.g)
	stmts = append(stmts,
		`CREATE TABLE IF NOT EXISTS standards.library_releases (
			release_id TEXT PRIMARY KEY,
			published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			published_by TEXT NOT NULL,
			bec_version TEXT NOT NULL,
			taxonomy_version TEXT NOT NULL,
			vlists_sha256 TEXT NOT NULL,
			bec_db_sha256 TEXT,
			vmetadata_sha256 TEXT,
			notes TEXT,
			is_current BOOLEAN NOT NULL DEFAULT FALSE,
			vlists_gzip BYTEA
		);`,
		buildStandardsTableDDL("usys_all_specs", StandardsUSysAllSpecsColumns),
		`CREATE INDEX IF NOT EXISTS idx_standards_specs_code ON standards.usys_all_specs ("Code");`,
		`CREATE INDEX IF NOT EXISTS idx_standards_specs_oldcode ON standards.usys_all_specs ("OldCode");`,
		buildStandardsTableDDL("usys_table_of_lists", StandardsUSysTableOfListsColumns),
		`CREATE INDEX IF NOT EXISTS idx_standards_lists_name ON standards.usys_table_of_lists ("ListName");`,
		buildStandardsTableDDL("bec_site_series", StandardsBECSiteSeriesColumns),
		`CREATE INDEX IF NOT EXISTS idx_standards_bec_zone ON standards.bec_site_series ("Zone", "SubZone");`,
		buildStandardsTableDDL("master_site_unit_list", StandardsMasterSiteUnitListColumns),
		buildStandardsTableDDL("usys_zone_list", StandardsUSysZoneListColumns),
		buildStandardsTableDDL("project_metadata", StandardsProjectMetadataColumns),
	)

	return stmts
}

// ApplyRoleGrants enforces PostgreSQL least-privilege grants and RLS policies for
// VPRO_PG_USER (Contributor) vs VPRO_PG_ADMIN_USER (Custodian).
func ApplyRoleGrants(ctx context.Context, conn *pgx.Conn, contributorRole, custodianRole string) (bool, error) {
	roleExists := func(role string) (bool, error) {
		if strings.TrimSpace(role) == "" {
			return false, nil
		}
		var ok bool
		err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, role).Scan(&ok)
		return ok, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock down PUBLIC on all 3 VPro schemas so unrelated DB roles on shared servers have zero access.
	for _, schema := range []string{"becmaster", "staging", "standards"} {
		qSchema := quotePGIdent(schema)
		if _, err := tx.Exec(ctx, fmt.Sprintf("REVOKE ALL ON SCHEMA %s FROM PUBLIC", qSchema)); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("REVOKE ALL ON ALL TABLES IN SCHEMA %s FROM PUBLIC", qSchema)); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf("REVOKE ALL ON ALL SEQUENCES IN SCHEMA %s FROM PUBLIC", qSchema)); err != nil {
			return false, err
		}
	}

	if hasAdmin, err := roleExists(custodianRole); err != nil {
		return false, err
	} else if hasAdmin {
		qAdmin := quotePGIdent(custodianRole)
		for _, schema := range []string{"becmaster", "staging", "standards"} {
			qSchema := quotePGIdent(schema)
			for _, sqlStmt := range []string{
				fmt.Sprintf("GRANT ALL PRIVILEGES ON SCHEMA %s TO %s", qSchema, qAdmin),
				fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA %s TO %s", qSchema, qAdmin),
				fmt.Sprintf("GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA %s TO %s", qSchema, qAdmin),
			} {
				if _, err := tx.Exec(ctx, sqlStmt); err != nil {
					return false, err
				}
			}
		}
	}

	if strings.TrimSpace(contributorRole) != "" && strings.TrimSpace(custodianRole) != "" && contributorRole == custodianRole {
		return false, errors.New("VPRO_PG_USER and VPRO_PG_ADMIN_USER must be distinct PostgreSQL roles to enforce contributor/custodian isolation")
	}
	hasContributor, err := roleExists(contributorRole)
	if err != nil {
		return false, err
	}
	if hasContributor && contributorRole != custodianRole {
		qUser := quotePGIdent(contributorRole)
		grantStmts := []string{
			// Clean slate for contributor role on VPro schemas
			fmt.Sprintf("REVOKE ALL ON ALL TABLES IN SCHEMA becmaster FROM %s", qUser),
			fmt.Sprintf("REVOKE ALL ON ALL SEQUENCES IN SCHEMA becmaster FROM %s", qUser),
			fmt.Sprintf("REVOKE ALL ON ALL TABLES IN SCHEMA staging FROM %s", qUser),
			fmt.Sprintf("REVOKE ALL ON ALL SEQUENCES IN SCHEMA staging FROM %s", qUser),
			fmt.Sprintf("REVOKE ALL ON ALL TABLES IN SCHEMA standards FROM %s", qUser),
			fmt.Sprintf("REVOKE ALL ON ALL SEQUENCES IN SCHEMA standards FROM %s", qUser),

			// Schema USAGE on all 3 VPro schemas (never CREATE)
			fmt.Sprintf("GRANT USAGE ON SCHEMA becmaster, staging, standards TO %s", qUser),

			// Read-only SELECT on becmaster.* and standards.*
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA becmaster TO %s", qUser),
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA standards TO %s", qUser),

			// Download activity log INSERT + sequence
			fmt.Sprintf("GRANT INSERT ON TABLE becmaster.download_log TO %s", qUser),
			fmt.Sprintf("GRANT USAGE, SELECT ON SEQUENCE becmaster.download_log_id_seq TO %s", qUser),

			// Staging queue SELECT + INSERT only (zero UPDATE/DELETE/TRUNCATE)
			fmt.Sprintf("GRANT SELECT, INSERT ON ALL TABLES IN SCHEMA staging TO %s", qUser),
			fmt.Sprintf("GRANT USAGE, SELECT ON SEQUENCE staging.audit_audit_seq_seq TO %s", qUser),

			// Row-Level Security on staging.submissions
			`ALTER TABLE staging.submissions ENABLE ROW LEVEL SECURITY`,
			`DROP POLICY IF EXISTS submissions_contributor_select ON staging.submissions`,
			fmt.Sprintf(`CREATE POLICY submissions_contributor_select ON staging.submissions FOR SELECT TO %s USING (true)`, qUser),
			`DROP POLICY IF EXISTS submissions_contributor_insert ON staging.submissions`,
			fmt.Sprintf(`CREATE POLICY submissions_contributor_insert ON staging.submissions FOR INSERT TO %s WITH CHECK (
				status = 'pending'
				AND compliance_passed = TRUE
				AND db_role = current_user
				AND reviewed_by IS NULL
				AND merged_tx_id IS NULL
			)`, qUser),
		}
		// Disable RLS on child tables so PostgreSQL COPY FROM STDIN (pgx.CopyFrom) succeeds;
		// per-submission ownership & pending-status isolation on COPY/INSERT is enforced by trg_staging_*_insert!
		for _, stgTbl := range []string{"env", "admin", "veg", "humus", "mineral", "other", "metadata", "audit"} {
			qTbl := quotePGIdent(stgTbl)
			selPol := quotePGIdent(stgTbl + "_contributor_select")
			insPol := quotePGIdent(stgTbl + "_contributor_insert")
			grantStmts = append(grantStmts,
				fmt.Sprintf(`DROP POLICY IF EXISTS %s ON staging.%s`, selPol, qTbl),
				fmt.Sprintf(`DROP POLICY IF EXISTS %s ON staging.%s`, insPol, qTbl),
				fmt.Sprintf(`ALTER TABLE staging.%s DISABLE ROW LEVEL SECURITY`, qTbl),
			)
		}
		for _, stmt := range grantStmts {
			if _, err := tx.Exec(ctx, stmt); err != nil {
				return false, err
			}
		}
	} else if contributorRole != "" && !hasContributor {
		return false, errors.New("configured VPRO_PG_USER role does not exist in PostgreSQL pg_roles")
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return hasContributor, nil
}
