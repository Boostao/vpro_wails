package cloudsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CloudConnectionReport summarizes live server health, role capabilities, and table counts.
// Zero passwords, DSNs, or host secrets are included.
type CloudConnectionReport struct {
	Connected              bool   `json:"connected"`
	ActiveRoleMode         string `json:"activeRoleMode"`
	ConfigSource           string `json:"configSource"`
	LatencyMs              int64  `json:"latencyMs"`
	ServerVersion          string `json:"serverVersion"`
	PostGISVersion         string `json:"postgisVersion,omitempty"`
	Database               string `json:"database"`
	SSLMode                string `json:"sslMode"`
	SchemasBootstrapped    bool   `json:"schemasBootstrapped"`
	CanReadMaster          bool   `json:"canReadMaster"`
	CanSubmitStaging       bool   `json:"canSubmitStaging"`
	CanMergeMaster         bool   `json:"canMergeMaster"`
	CanBootstrapSchema     bool   `json:"canBootstrapSchema"`
	BECMasterPlotCount     int64  `json:"becmasterPlotCount"`
	BECMasterVegCount      int64  `json:"becmasterVegCount"`
	PendingSubmissionCount int64  `json:"pendingSubmissionCount"`
	SnapshotCount          int64  `json:"snapshotCount"`
	LatestSnapshotTag      string `json:"latestSnapshotTag,omitempty"`
	ActiveStandardsRelease string `json:"activeStandardsRelease,omitempty"`
	ActiveBECVersion       string `json:"activeBecVersion,omitempty"`
	ActiveTaxonomyVersion  string `json:"activeTaxonomyVersion,omitempty"`
	ActiveVListsSHA256     string `json:"activeVListsSha256,omitempty"`
	CheckedAt              string `json:"checkedAt"`
}

func poolIdentityFingerprint(creds *ResolvedCredentials, roleMode string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(roleMode + "|" + creds.Host + "|" + fmt.Sprint(creds.Port) + "|" + creds.Database + "|" + creds.SSLMode + "|"))
	if roleMode == RoleModeCustodian {
		_, _ = h.Write([]byte(creds.AdminURL + "|" + creds.AdminUser + "|" + creds.AdminPassword))
	} else {
		_, _ = h.Write([]byte(creds.UserURL + "|" + creds.User + "|" + creds.UserPassword))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// AcquirePool returns a bounded pgxpool.Pool for the requested roleMode ("contributor" | "custodian").
func (m *Manager) AcquirePool(ctx context.Context, roleMode string) (*pgxpool.Pool, *ResolvedCredentials, error) {
	creds, err := m.ResolveCredentials()
	if err != nil {
		return nil, nil, err
	}
	if roleMode == "" {
		roleMode = creds.ActiveRoleMode
	}
	fp := poolIdentityFingerprint(creds, roleMode)

	m.mu.RLock()
	if roleMode == RoleModeCustodian && m.adminPool != nil && m.adminKey == fp {
		pool := m.adminPool
		m.mu.RUnlock()
		return pool, creds, nil
	}
	if roleMode != RoleModeCustodian && m.userPool != nil && m.userKey == fp {
		pool := m.userPool
		m.mu.RUnlock()
		return pool, creds, nil
	}
	m.mu.RUnlock()

	connURL, err := buildConnURLStringForRole(creds, roleMode)
	if err != nil {
		return nil, creds, err
	}
	poolCfg, err := pgxpool.ParseConfig(connURL)
	if err != nil {
		return nil, creds, wrapRedactedError("configure PostgreSQL pool", err, creds)
	}
	poolCfg.MaxConns = 4
	poolCfg.MinConns = 0
	poolCfg.MaxConnIdleTime = 2 * time.Minute
	poolCfg.ConnConfig.ConnectTimeout = 15 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, creds, wrapRedactedError("initialize PostgreSQL pool", err, creds)
	}

	m.mu.Lock()
	// Re-check under write lock so concurrent callers never close an active matching pool!
	if roleMode == RoleModeCustodian && m.adminPool != nil && m.adminKey == fp {
		existing := m.adminPool
		m.mu.Unlock()
		pool.Close()
		return existing, creds, nil
	}
	if roleMode != RoleModeCustodian && m.userPool != nil && m.userKey == fp {
		existing := m.userPool
		m.mu.Unlock()
		pool.Close()
		return existing, creds, nil
	}
	var oldPool *pgxpool.Pool
	if roleMode == RoleModeCustodian {
		oldPool = m.adminPool
		m.adminPool = pool
		m.adminKey = fp
	} else {
		oldPool = m.userPool
		m.userPool = pool
		m.userKey = fp
	}
	m.mu.Unlock()
	if oldPool != nil {
		oldPool.Close()
	}
	return pool, creds, nil
}

// OpenDedicatedConn opens a single *pgx.Conn for bulk CopyFrom or SERIALIZABLE transactions.
func (m *Manager) OpenDedicatedConn(ctx context.Context, roleMode string) (*pgx.Conn, *ResolvedCredentials, error) {
	creds, err := m.ResolveCredentials()
	if err != nil {
		return nil, nil, err
	}
	if roleMode == "" {
		roleMode = creds.ActiveRoleMode
	}
	cfg, err := buildConnConfigForRole(creds, roleMode)
	if err != nil {
		return nil, creds, err
	}
	cfg.ConnectTimeout = 20 * time.Second
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, creds, wrapRedactedError("connect to PostgreSQL", err, creds)
	}
	return conn, creds, nil
}

// TestCloudConnection probes PostgreSQL connectivity, latency, privileges, and counts for the active role.
func (m *Manager) TestCloudConnection(ctx context.Context) (CloudConnectionReport, error) {
	creds, err := m.ResolveCredentials()
	if err != nil {
		return CloudConnectionReport{}, err
	}
	roleMode := creds.ActiveRoleMode
	if roleMode == "" {
		roleMode = RoleModeContributor
	}

	t0 := time.Now()
	pool, _, err := m.AcquirePool(ctx, roleMode)
	if err != nil {
		return CloudConnectionReport{}, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return CloudConnectionReport{}, wrapRedactedError("connect to PostgreSQL", err, creds)
	}
	defer conn.Release()

	var serverVer string
	if err := conn.QueryRow(ctx, `SHOW server_version`).Scan(&serverVer); err != nil {
		return CloudConnectionReport{}, wrapRedactedError("probe PostgreSQL version", err, creds)
	}
	latency := time.Since(t0).Milliseconds()
	if latency < 1 {
		latency = 1
	}

	report := CloudConnectionReport{
		Connected:      true,
		ActiveRoleMode: roleMode,
		ConfigSource:   creds.Source,
		LatencyMs:      latency,
		ServerVersion:  strings.TrimSpace(serverVer),
		Database:       creds.Database,
		SSLMode:        creds.SSLMode,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}

	_ = conn.QueryRow(ctx, `SELECT COALESCE(extversion, '') FROM pg_extension WHERE extname = 'postgis'`).Scan(&report.PostGISVersion)
	_ = conn.QueryRow(ctx, `SELECT has_database_privilege(current_user, current_database(), 'CREATE')`).Scan(&report.CanBootstrapSchema)

	var schemaCount int
	if err := conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.schemata
		WHERE schema_name IN ('becmaster', 'staging', 'standards')
	`).Scan(&schemaCount); err != nil {
		return report, wrapRedactedError("inspect PostgreSQL schemas", err, creds)
	}
	report.SchemasBootstrapped = schemaCount == 3

	if !report.SchemasBootstrapped {
		return report, nil
	}

	var hasEnvTable bool
	_ = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = 'becmaster' AND c.relname = 'env'
		)
	`).Scan(&hasEnvTable)
	if !hasEnvTable {
		report.SchemasBootstrapped = false
		return report, nil
	}

	_ = conn.QueryRow(ctx, `
		SELECT has_table_privilege(current_user, 'becmaster.env', 'SELECT'),
		       has_table_privilege(current_user, 'staging.submissions', 'INSERT'),
		       has_table_privilege(current_user, 'becmaster.env', 'INSERT') AND has_table_privilege(current_user, 'becmaster.env', 'UPDATE')
	`).Scan(&report.CanReadMaster, &report.CanSubmitStaging, &report.CanMergeMaster)
	if report.CanMergeMaster {
		report.CanBootstrapSchema = true
	}

	if report.CanReadMaster {
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM becmaster.env`).Scan(&report.BECMasterPlotCount)
		// Fast exact count when modest or reltuples fallback if huge? COUNT(*) on 1.65M PK rows in PG 15 index-only scan takes ~60ms!
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM becmaster.veg`).Scan(&report.BECMasterVegCount)
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM becmaster.snapshots`).Scan(&report.SnapshotCount)
		_ = conn.QueryRow(ctx, `SELECT snapshot_tag FROM becmaster.snapshots ORDER BY created_at DESC LIMIT 1`).Scan(&report.LatestSnapshotTag)
		_ = conn.QueryRow(ctx, `
			SELECT release_id, bec_version, taxonomy_version, vlists_sha256
			FROM standards.library_releases
			WHERE is_current = TRUE
			ORDER BY published_at DESC
			LIMIT 1
		`).Scan(&report.ActiveStandardsRelease, &report.ActiveBECVersion, &report.ActiveTaxonomyVersion, &report.ActiveVListsSHA256)
	}

	var canReadStaging bool
	_ = conn.QueryRow(ctx, `SELECT has_table_privilege(current_user, 'staging.submissions', 'SELECT')`).Scan(&canReadStaging)
	if canReadStaging {
		_ = conn.QueryRow(ctx, `SELECT COUNT(*) FROM staging.submissions WHERE status = 'pending'`).Scan(&report.PendingSubmissionCount)
	}

	return report, nil
}
