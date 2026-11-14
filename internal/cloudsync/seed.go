package cloudsync

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
	"github.com/jackc/pgx/v5"
)

// PublishStandardsRequest configures custodian publication of 2026 VLists + BECdb + VMetaData standards.
type PublishStandardsRequest struct {
	ReleaseID       string `json:"releaseId,omitempty"`
	BECVersion      string `json:"becVersion,omitempty"`
	TaxonomyVersion string `json:"taxonomyVersion,omitempty"`
	VListsPath      string `json:"vlistsPath,omitempty"`
	BECDBPath       string `json:"becDbPath,omitempty"`
	VMetaDataPath   string `json:"vmetaDataPath,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

func (r *PublishStandardsRequest) UnmarshalJSON(data []byte) error {
	type plain PublishStandardsRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// PublishStandardsResult summarizes published reference library counts and digests.
type PublishStandardsResult struct {
	ReleaseID         string         `json:"releaseId"`
	BECVersion        string         `json:"becVersion"`
	TaxonomyVersion   string         `json:"taxonomyVersion"`
	VListsSHA256      string         `json:"vlistsSha256"`
	BECDBSHA256       string         `json:"becDbSha256"`
	VMetaDataSHA256   string         `json:"vmetaDataSha256"`
	TableCounts       map[string]int `json:"tableCounts"`
	ElapsedMs         int64          `json:"elapsedMs"`
	PublishedAt       string         `json:"publishedAt"`
}

// SeedBaselineRequest configures streaming VP08 SQLite -> PostgreSQL becmaster.* baseline seeding.
type SeedBaselineRequest struct {
	SourceDBPath    string `json:"sourceDbPath"`
	SourceProject   string `json:"sourceProject,omitempty"`
	SnapshotTag     string `json:"snapshotTag,omitempty"`
	ReplaceExisting bool   `json:"replaceExisting"`
	SkipAudit       bool   `json:"skipAudit,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

func (r *SeedBaselineRequest) UnmarshalJSON(data []byte) error {
	type plain SeedBaselineRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// SeedBaselineResult summarizes row counts and snapshot provenance from SeedBECMasterBaseline.
type SeedBaselineResult struct {
	SourceProject     string           `json:"sourceProject"`
	SourceSHA256      string           `json:"sourceSha256"`
	SnapshotTag       string           `json:"snapshotTag"`
	PlotCount         int              `json:"plotCount"`
	AdminCount        int              `json:"adminCount"`
	VegCount          int64            `json:"vegCount"`
	HumusCount        int64            `json:"humusCount"`
	MineralCount      int64            `json:"mineralCount"`
	OtherCount        int64            `json:"otherCount"`
	MetadataCount     int              `json:"metadataCount"`
	AuditCount        int64            `json:"auditCount"`
	ChildIdentityRows int64            `json:"childIdentityRows"`
	TailAuditHash     string           `json:"tailAuditHash"`
	ElapsedMs         int64            `json:"elapsedMs"`
	SeededAt          string           `json:"seededAt"`
	TableCounts       map[string]int64 `json:"tableCounts"`
}

func readBundledStandardsVersions() (becVer, taxVer string) {
	becVer = "BECdb_v13_2026"
	taxVer = "BCPlantTaxa_v19_2025"
	var parsed struct {
		Sources []struct {
			Version string `json:"version"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(resources.Standards2026ProvenanceJSON, &parsed); err == nil {
		for _, src := range parsed.Sources {
			switch {
			case strings.HasPrefix(src.Version, "BECdb_v13"):
				becVer = src.Version
			case strings.HasPrefix(src.Version, "BCPlantTaxa_v19"):
				taxVer = src.Version
			}
		}
	}
	return becVer, taxVer
}

func compressGzipFast(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := gw.Write(raw); err != nil {
		_ = gw.Close()
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressGzip(compressed []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

func streamFileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// createConsistentSQLiteSnapshot uses SQLite VACUUM INTO to capture an atomic, standalone
// WAL-merged snapshot of an active SQLite database so hashing and streaming observe identical rows.
func createConsistentSQLiteSnapshot(ctx context.Context, srcPath string) (snapPath string, cleanup func(), err error) {
	cleanup = func() {}
	tmpDir, err := os.MkdirTemp("", "vpro-sqlsnap-*")
	if err != nil {
		return "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }
	snapPath = filepath.Join(tmpDir, "snapshot.db")

	db, err := sqlutil.OpenReadOnlyContext(ctx, srcPath)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	defer db.Close()

	escaped := strings.ReplaceAll(snapPath, "'", "''")
	if _, err := db.ExecContext(ctx, fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return snapPath, cleanup, nil
}

func materializeTempSQLiteIfNeeded(ctx context.Context, explicitPath string, fallbackBytes []byte, label string) (path string, rawBytes []byte, shaHex string, cleanup func(), err error) {
	cleanup = func() {}
	if trimmed := strings.TrimSpace(explicitPath); trimmed != "" {
		resolved, rErr := vprocore.ExistingDatabasePath(trimmed)
		if rErr != nil {
			return "", nil, "", cleanup, fmt.Errorf("%s database: %w", label, rErr)
		}
		targetPath := resolved
		if walInfo, wErr := os.Stat(resolved + "-wal"); wErr == nil && walInfo.Size() > 0 {
			snapPath, snapClean, sErr := createConsistentSQLiteSnapshot(ctx, resolved)
			if sErr != nil {
				return "", nil, "", cleanup, fmt.Errorf("snapshot WAL-backed %s database: %w", label, sErr)
			}
			targetPath = snapPath
			cleanup = snapClean
		}
		data, rErr := os.ReadFile(targetPath)
		if rErr != nil {
			cleanup()
			return "", nil, "", func() {}, rErr
		}
		sum := sha256.Sum256(data)
		return targetPath, data, hex.EncodeToString(sum[:]), cleanup, nil
	}
	if len(fallbackBytes) == 0 {
		return "", nil, "", cleanup, fmt.Errorf("missing %s database bytes", label)
	}
	tmp, tErr := os.CreateTemp("", "vpro-std-*.db")
	if tErr != nil {
		return "", nil, "", cleanup, tErr
	}
	tmpName := tmp.Name()
	cleanup = func() { _ = os.Remove(tmpName) }
	if _, wErr := tmp.Write(fallbackBytes); wErr != nil {
		_ = tmp.Close()
		cleanup()
		return "", nil, "", func() {}, wErr
	}
	if cErr := tmp.Close(); cErr != nil {
		cleanup()
		return "", nil, "", func() {}, cErr
	}
	sum := sha256.Sum256(fallbackBytes)
	return tmpName, fallbackBytes, hex.EncodeToString(sum[:]), cleanup, nil
}

func copySQLiteTableToPG(
	ctx context.Context,
	tx pgx.Tx,
	sqliteDB *sql.DB,
	sqliteTable string,
	pgSchema string,
	pgTable string,
	cols []ColumnSpec,
) (int64, error) {
	selectCols := sqliteSelectColumnsCSV(cols)
	query := fmt.Sprintf("SELECT %s FROM %s", selectCols, quotePGIdent(sqliteTable))
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query SQLite %s: %w", sqliteTable, err)
	}
	defer rows.Close()

	rawVals := make([]any, len(cols))
	scanDest := make([]any, len(cols))
	for i := range rawVals {
		scanDest[i] = &rawVals[i]
	}

	var streamErr error
	copied, copyErr := tx.CopyFrom(
		ctx,
		pgx.Identifier{pgSchema, pgTable},
		columnNames(cols),
		pgx.CopyFromFunc(func() ([]any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !rows.Next() {
				return nil, rows.Err()
			}
			if err := rows.Scan(scanDest...); err != nil {
				return nil, err
			}
			out := make([]any, len(cols))
			for i, col := range cols {
				conv, cErr := CanonicalizeSQLiteToPG(col, rawVals[i])
				if cErr != nil {
					streamErr = fmt.Errorf("%s.%s: %w", sqliteTable, col.Name, cErr)
					return nil, streamErr
				}
				out[i] = conv
			}
			return out, nil
		}),
	)
	if streamErr != nil {
		return 0, streamErr
	}
	if copyErr != nil {
		return 0, copyErr
	}
	return copied, rows.Close()
}

// PublishStandardsToCloud streams VLists.db + bec.db + VMetaData.db into standards.* via pgx.CopyFrom.
func (m *Manager) PublishStandardsToCloud(ctx context.Context, s *vprocore.ContextService, contextID string, req PublishStandardsRequest) (PublishStandardsResult, error) {
	t0 := time.Now()
	defaultBECVer, defaultTaxVer := readBundledStandardsVersions()
	becVer := strings.TrimSpace(req.BECVersion)
	if becVer == "" {
		becVer = defaultBECVer
	}
	taxVer := strings.TrimSpace(req.TaxonomyVersion)
	if taxVer == "" {
		taxVer = defaultTaxVer
	}

	vlistsExplicit := strings.TrimSpace(req.VListsPath)
	vmetaExplicit := strings.TrimSpace(req.VMetaDataPath)
	actor := "Custodian"
	if s != nil && contextID != "" {
		_, _ = vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			if vlistsExplicit == "" {
				vlistsExplicit = plots.Projects.SupportPaths["VLists"]
			}
			if vmetaExplicit == "" {
				vmetaExplicit = plots.Projects.SupportPaths["VMetaData"]
			}
			if u := strings.TrimSpace(plots.CurrentUser); u != "" {
				actor = u
			}
			return struct{}{}, nil
		})
	}

	var bundledVLists, bundledVMeta []byte
	if vlistsExplicit == "" {
		bundledVLists, _ = resources.DatabaseFamilyFS.ReadFile("resources/database-family/VLists.db")
	}
	if vmetaExplicit == "" {
		bundledVMeta, _ = resources.DatabaseFamilyFS.ReadFile("resources/database-family/VMetaData.db")
	}

	vlistsPath, vlistsBytes, vlistsSHA, cleanVLists, err := materializeTempSQLiteIfNeeded(ctx, vlistsExplicit, bundledVLists, "VLists")
	if err != nil {
		return PublishStandardsResult{}, err
	}
	defer cleanVLists()

	becPath, _, becSHA, cleanBEC, err := materializeTempSQLiteIfNeeded(ctx, req.BECDBPath, resources.BECDB, "BEC")
	if err != nil {
		return PublishStandardsResult{}, err
	}
	defer cleanBEC()

	vmetaPath, _, vmetaSHA, cleanVMeta, err := materializeTempSQLiteIfNeeded(ctx, vmetaExplicit, bundledVMeta, "VMetaData")
	if err != nil {
		return PublishStandardsResult{}, err
	}
	defer cleanVMeta()

	vlistsGzip, err := compressGzipFast(vlistsBytes)
	if err != nil {
		return PublishStandardsResult{}, fmt.Errorf("compress VLists.db snapshot: %w", err)
	}

	releaseID := strings.TrimSpace(req.ReleaseID)
	if releaseID == "" {
		releaseID = fmt.Sprintf("%s__%s__%s", becVer, taxVer, vlistsSHA[:12])
	}

	vlistsSQLite, err := sqlutil.OpenReadOnlyContext(ctx, vlistsPath)
	if err != nil {
		return PublishStandardsResult{}, fmt.Errorf("open VLists.db: %w", err)
	}
	defer vlistsSQLite.Close()

	becSQLite, err := sqlutil.OpenReadOnlyContext(ctx, becPath)
	if err != nil {
		return PublishStandardsResult{}, fmt.Errorf("open bec.db: %w", err)
	}
	defer becSQLite.Close()

	vmetaSQLite, err := sqlutil.OpenReadOnlyContext(ctx, vmetaPath)
	if err != nil {
		return PublishStandardsResult{}, fmt.Errorf("open VMetaData.db: %w", err)
	}
	defer vmetaSQLite.Close()

	conn, creds, err := m.OpenDedicatedConn(ctx, RoleModeCustodian)
	if err != nil {
		return PublishStandardsResult{}, err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return PublishStandardsResult{}, wrapRedactedError("begin standards publish transaction", err, creds)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		TRUNCATE TABLE
			standards.usys_all_specs,
			standards.usys_table_of_lists,
			standards.bec_site_series,
			standards.master_site_unit_list,
			standards.usys_zone_list,
			standards.project_metadata
		RESTART IDENTITY
	`); err != nil {
		return PublishStandardsResult{}, wrapRedactedError("truncate standards tables", err, creds)
	}

	counts := make(map[string]int, 6)
	type copyJob struct {
		db        *sql.DB
		srcTable  string
		pgTable   string
		cols      []ColumnSpec
	}
	jobs := []copyJob{
		{vlistsSQLite, "USysAllSpecs", "usys_all_specs", StandardsUSysAllSpecsColumns},
		{vlistsSQLite, "USysTableOfLists", "usys_table_of_lists", StandardsUSysTableOfListsColumns},
		{becSQLite, "BECSiteSeries", "bec_site_series", StandardsBECSiteSeriesColumns},
		{vlistsSQLite, "MasterSiteUnitList", "master_site_unit_list", StandardsMasterSiteUnitListColumns},
		{vlistsSQLite, "USysZoneList", "usys_zone_list", StandardsUSysZoneListColumns},
		{vmetaSQLite, "ProjectMetadata", "project_metadata", StandardsProjectMetadataColumns},
	}
	for _, job := range jobs {
		n, cErr := copySQLiteTableToPG(ctx, tx, job.db, job.srcTable, "standards", job.pgTable, job.cols)
		if cErr != nil {
			return PublishStandardsResult{}, wrapRedactedError("copy "+job.pgTable, cErr, creds)
		}
		counts[job.pgTable] = int(n)
	}

	nowUTC := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE standards.library_releases SET is_current = FALSE WHERE is_current = TRUE`); err != nil {
		return PublishStandardsResult{}, wrapRedactedError("retire prior current release", err, creds)
	}
	var notesPtr *string
	if n := strings.TrimSpace(req.Notes); n != "" {
		notesPtr = &n
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO standards.library_releases (
			release_id, published_at, published_by, bec_version, taxonomy_version,
			vlists_sha256, bec_db_sha256, vmetadata_sha256, notes, is_current, vlists_gzip
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
		ON CONFLICT (release_id) DO UPDATE SET
			published_at = EXCLUDED.published_at,
			published_by = EXCLUDED.published_by,
			bec_version = EXCLUDED.bec_version,
			taxonomy_version = EXCLUDED.taxonomy_version,
			vlists_sha256 = EXCLUDED.vlists_sha256,
			bec_db_sha256 = EXCLUDED.bec_db_sha256,
			vmetadata_sha256 = EXCLUDED.vmetadata_sha256,
			notes = EXCLUDED.notes,
			is_current = TRUE,
			vlists_gzip = EXCLUDED.vlists_gzip
	`, releaseID, nowUTC, actor, becVer, taxVer, vlistsSHA, becSHA, vmetaSHA, notesPtr, vlistsGzip); err != nil {
		return PublishStandardsResult{}, wrapRedactedError("record standards release", err, creds)
	}

	if err := tx.Commit(ctx); err != nil {
		return PublishStandardsResult{}, wrapRedactedError("commit standards release", err, creds)
	}

	return PublishStandardsResult{
		ReleaseID:       releaseID,
		BECVersion:      becVer,
		TaxonomyVersion: taxVer,
		VListsSHA256:    vlistsSHA,
		BECDBSHA256:     becSHA,
		VMetaDataSHA256: vmetaSHA,
		TableCounts:     counts,
		ElapsedMs:       time.Since(t0).Milliseconds(),
		PublishedAt:     nowUTC.Format(time.RFC3339),
	}, nil
}

func resolveProjectInSQLiteFile(ctx context.Context, dbPath, requestedProject string) (string, string, error) {
	resolved, err := vprocore.ExistingDatabasePath(dbPath)
	if err != nil {
		return "", "", err
	}
	projects, err := vprocore.InspectFileContext(ctx, resolved)
	if err != nil {
		return "", "", err
	}
	reqProj := strings.TrimSpace(requestedProject)
	if reqProj != "" {
		for _, p := range projects {
			if p.Name == reqProj && p.Compatible {
				return resolved, p.Name, nil
			}
		}
		return "", "", fmt.Errorf("compatible VP08 project %q not found in %s", reqProj, resolved)
	}
	var compatible []string
	for _, p := range projects {
		if p.Compatible {
			compatible = append(compatible, p.Name)
		}
	}
	if len(compatible) == 0 {
		return "", "", fmt.Errorf("no compatible VP08 project found in %s", resolved)
	}
	if len(compatible) > 1 {
		return "", "", fmt.Errorf("multiple compatible projects (%s) found in %s; specify sourceProject explicitly", strings.Join(compatible, ", "), resolved)
	}
	return resolved, compatible[0], nil
}

// SeedBECMasterBaseline streams a VP08 SQLite project (e.g. 71,567-plot BECMaster_fixing.db)
// into becmaster.* via pgx.CopyFrom without OOM, seeding child_identity, hash-chained audit, and baseline snapshot.
func (m *Manager) SeedBECMasterBaseline(ctx context.Context, req SeedBaselineRequest) (SeedBaselineResult, error) {
	t0 := time.Now()
	resolvedPath, project, err := resolveProjectInSQLiteFile(ctx, req.SourceDBPath, req.SourceProject)
	if err != nil {
		return SeedBaselineResult{}, err
	}
	srcSHA, err := streamFileSHA256Hex(resolvedPath)
	if err != nil {
		return SeedBaselineResult{}, fmt.Errorf("hash source SQLite database: %w", err)
	}

	snapshotTag := strings.TrimSpace(req.SnapshotTag)
	if snapshotTag == "" {
		snapshotTag = "BECMaster_v13_Baseline"
	}

	sqliteDB, err := sqlutil.OpenReadOnlyContext(ctx, resolvedPath)
	if err != nil {
		return SeedBaselineResult{}, fmt.Errorf("open source SQLite database: %w", err)
	}
	defer sqliteDB.Close()

	conn, creds, err := m.OpenDedicatedConn(ctx, RoleModeCustodian)
	if err != nil {
		return SeedBaselineResult{}, err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("begin BECMaster seed transaction", err, creds)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if req.ReplaceExisting {
		// Preserve becmaster.child_identity and child_identity_watermark so previously retired child IDs are never reused!
		if _, err := tx.Exec(ctx, `
			TRUNCATE TABLE
				becmaster.env,
				becmaster.admin,
				becmaster.veg,
				becmaster.humus,
				becmaster.mineral,
				becmaster.other,
				becmaster.metadata,
				becmaster.audit
			RESTART IDENTITY CASCADE
		`); err != nil {
			return SeedBaselineResult{}, wrapRedactedError("truncate existing becmaster baseline", err, creds)
		}
	}

	// 1. Stream <Project>_Env ("PlotNumber" byte-exact text identity)
	plotSet := make(map[string]struct{}, 72000)
	envCount, err := streamKeyedParentToPG(ctx, tx, sqliteDB, project+"_Env", "becmaster", "env", VP08EnvColumns, "PlotNumber", plotSet, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.env", err, creds)
	}

	// 2. Stream <Project>_Admin (filtered to valid plots in becmaster.env)
	seenAdmin := make(map[string]struct{}, len(plotSet))
	adminCount, err := streamKeyedParentToPG(ctx, tx, sqliteDB, project+"_Admin", "becmaster", "admin", VP08AdminColumns, "Plot", seenAdmin, plotSet)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.admin", err, creds)
	}

	// 3. Stream child tables (<Project>_Veg, _Humus, _Mineral, _Other) row-by-row with zero full-table slice in RAM
	vegCount, err := streamChildTableToPG(ctx, tx, sqliteDB, project+"_Veg", "becmaster", "veg", VP08VegColumns, plotSet, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.veg", err, creds)
	}
	humusCount, err := streamChildTableToPG(ctx, tx, sqliteDB, project+"_Humus", "becmaster", "humus", VP08HumusColumns, plotSet, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.humus", err, creds)
	}
	mineralCount, err := streamChildTableToPG(ctx, tx, sqliteDB, project+"_Mineral", "becmaster", "mineral", VP08MineralColumns, plotSet, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.mineral", err, creds)
	}
	otherCount, err := streamChildTableToPG(ctx, tx, sqliteDB, project+"_Other", "becmaster", "other", VP08OtherColumns, plotSet, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.other", err, creds)
	}

	// 4. Populate becmaster.child_identity and O(1) child_identity_watermark server-side
	cmdTag, err := tx.Exec(ctx, `
		INSERT INTO becmaster.child_identity (child_table, id)
		SELECT 'veg', "ID" FROM becmaster.veg WHERE "ID" IS NOT NULL
		UNION ALL
		SELECT 'humus', "ID" FROM becmaster.humus WHERE "ID" IS NOT NULL
		UNION ALL
		SELECT 'mineral', "ID" FROM becmaster.mineral WHERE "ID" IS NOT NULL
		UNION ALL
		SELECT 'other', "ID" FROM becmaster.other WHERE "ID" IS NOT NULL
		ON CONFLICT (child_table, id) DO NOTHING
	`)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.child_identity", err, creds)
	}
	childIdentityRows := cmdTag.RowsAffected()

	// Also merge any __VPRO_ChildIdentity retired reservations from the source SQLite database
	if err := copySQLiteRetiredChildIdentities(ctx, tx, sqliteDB, project); err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed retired child identities", err, creds)
	}

	for _, childTbl := range []string{"veg", "humus", "mineral", "other"} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO becmaster.child_identity_watermark (child_table, max_id)
			VALUES ($1, COALESCE((SELECT GREATEST(MAX(id), 0) FROM becmaster.child_identity WHERE child_table = $1), 0))
			ON CONFLICT (child_table) DO UPDATE SET
				max_id = GREATEST(becmaster.child_identity_watermark.max_id, EXCLUDED.max_id)
		`, childTbl); err != nil {
			return SeedBaselineResult{}, wrapRedactedError("seed child_identity_watermark", err, creds)
		}
	}

	// 5. Stream <Project>_Metadata
	seenMeta := make(map[string]struct{}, 128)
	metaCount, err := streamKeyedParentToPG(ctx, tx, sqliteDB, project+"_Metadata", "becmaster", "metadata", VP08MetadataColumns, "ProjectID", seenMeta, nil)
	if err != nil {
		return SeedBaselineResult{}, wrapRedactedError("seed becmaster.metadata", err, creds)
	}

	// 6. Stream <Project>_Audit with SHA-256 tamper-evident hash chain
	tailHash := GenesisAuditHash
	_ = tx.QueryRow(ctx, `SELECT row_hash FROM becmaster.audit ORDER BY audit_seq DESC LIMIT 1`).Scan(&tailHash)
	var auditCount int64
	if !req.SkipAudit {
		auditCount, tailHash, err = streamMasterAuditToPG(ctx, tx, sqliteDB, project+"_Audit", tailHash)
		if err != nil {
			return SeedBaselineResult{}, wrapRedactedError("seed becmaster.audit", err, creds)
		}
	}

	// 7. Record baseline snapshot in becmaster.snapshots
	becVer, taxVer := readBundledStandardsVersions()
	_ = tx.QueryRow(ctx, `
		SELECT bec_version, taxonomy_version FROM standards.library_releases
		WHERE is_current = TRUE ORDER BY published_at DESC LIMIT 1
	`).Scan(&becVer, &taxVer)

	nowUTC := time.Now().UTC()
	var notesPtr *string
	if n := strings.TrimSpace(req.Notes); n != "" {
		notesPtr = &n
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO becmaster.snapshots (
			snapshot_tag, created_at, created_by, plot_count, veg_count,
			bec_version, taxonomy_version, manifest_sha256, notes
		) VALUES ($1, $2, current_user, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (snapshot_tag) DO UPDATE SET
			created_at = EXCLUDED.created_at,
			created_by = EXCLUDED.created_by,
			plot_count = EXCLUDED.plot_count,
			veg_count = EXCLUDED.veg_count,
			bec_version = EXCLUDED.bec_version,
			taxonomy_version = EXCLUDED.taxonomy_version,
			manifest_sha256 = EXCLUDED.manifest_sha256,
			notes = EXCLUDED.notes
	`, snapshotTag, nowUTC, int(envCount), vegCount, becVer, taxVer, srcSHA, notesPtr); err != nil {
		return SeedBaselineResult{}, wrapRedactedError("record baseline snapshot", err, creds)
	}

	if err := tx.Commit(ctx); err != nil {
		return SeedBaselineResult{}, wrapRedactedError("commit BECMaster baseline", err, creds)
	}

	// Refresh planner statistics outside transaction for < 500ms merge previews & subset queries
	_, _ = conn.Exec(ctx, `ANALYZE becmaster.env, becmaster.admin, becmaster.veg, becmaster.humus, becmaster.mineral, becmaster.other`)

	return SeedBaselineResult{
		SourceProject:     project,
		SourceSHA256:      srcSHA,
		SnapshotTag:       snapshotTag,
		PlotCount:         int(envCount),
		AdminCount:        int(adminCount),
		VegCount:          vegCount,
		HumusCount:        humusCount,
		MineralCount:      mineralCount,
		OtherCount:        otherCount,
		MetadataCount:     int(metaCount),
		AuditCount:        auditCount,
		ChildIdentityRows: childIdentityRows,
		TailAuditHash:     tailHash,
		ElapsedMs:         time.Since(t0).Milliseconds(),
		SeededAt:          nowUTC.Format(time.RFC3339),
		TableCounts: map[string]int64{
			"env":      envCount,
			"admin":    adminCount,
			"veg":      vegCount,
			"humus":    humusCount,
			"mineral":  mineralCount,
			"other":    otherCount,
			"metadata": metaCount,
			"audit":    auditCount,
		},
	}, nil
}

// streamKeyedParentToPG streams Env, Admin, or Metadata rows into becmaster.<tbl> or staging.<tbl>.
// If submissionID != nil, prepends submission_id as column 0 for staging tables.
func streamKeyedParentToPG(
	ctx context.Context,
	tx pgx.Tx,
	sqliteDB *sql.DB,
	sqliteTable string,
	pgSchema string,
	pgTable string,
	cols []ColumnSpec,
	keyCol string,
	seenKeys map[string]struct{},
	allowedKeys map[string]struct{},
	submissionID ...string,
) (int64, error) {
	qKey := quotePGIdent(keyCol)
	// Enforce literal text key extraction via CAST(col AS BLOB) when typeof='text', falling back to CAST AS TEXT for Metadata if numeric
	whereClause := fmt.Sprintf("WHERE %s IS NOT NULL", qKey)
	if keyCol == "PlotNumber" || keyCol == "Plot" {
		whereClause = fmt.Sprintf("WHERE typeof(%s) = 'text'", qKey)
	}
	query := fmt.Sprintf("SELECT %s FROM %s %s", sqliteSelectColumnsCSV(cols), quotePGIdent(sqliteTable), whereClause)
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", sqliteTable, err)
	}
	defer rows.Close()

	keyIdx := -1
	for i, c := range cols {
		if c.Name == keyCol {
			keyIdx = i
			break
		}
	}
	if keyIdx < 0 {
		return 0, fmt.Errorf("key column %s missing in %s spec", keyCol, pgTable)
	}

	hasSubID := len(submissionID) > 0 && submissionID[0] != ""
	copyCols := columnNames(cols)
	if hasSubID {
		copyCols = append([]string{"submission_id"}, copyCols...)
	}

	rawVals := make([]any, len(cols))
	scanDest := make([]any, len(cols))
	for i := range rawVals {
		scanDest[i] = &rawVals[i]
	}

	var streamErr error
	copied, copyErr := tx.CopyFrom(
		ctx,
		pgx.Identifier{pgSchema, pgTable},
		copyCols,
		pgx.CopyFromFunc(func() ([]any, error) {
			for {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !rows.Next() {
					return nil, rows.Err()
				}
				if err := rows.Scan(scanDest...); err != nil {
					return nil, err
				}
				keyVal, kErr := CanonicalizeSQLiteToPG(cols[keyIdx], rawVals[keyIdx])
				if kErr != nil || keyVal == nil {
					continue
				}
				keyStr, ok := keyVal.(string)
				if !ok || keyStr == "" {
					continue
				}
				if allowedKeys != nil {
					if _, exists := allowedKeys[keyStr]; !exists {
						continue
					}
				}
				if seenKeys != nil {
					if _, dup := seenKeys[keyStr]; dup {
						continue
					}
					seenKeys[keyStr] = struct{}{}
				}

				offset := 0
				out := make([]any, len(copyCols))
				if hasSubID {
					out[0] = submissionID[0]
					offset = 1
				}
				skipRow := false
				for i, col := range cols {
					conv, cErr := CanonicalizeSQLiteToPG(col, rawVals[i])
					if cErr != nil {
						streamErr = fmt.Errorf("%s.%s (key=%q): %w", sqliteTable, col.Name, keyStr, cErr)
						return nil, streamErr
					}
					out[offset+i] = conv
				}
				if skipRow {
					continue
				}
				return out, nil
			}
		}),
	)
	if streamErr != nil {
		return 0, streamErr
	}
	if copyErr != nil {
		return 0, copyErr
	}
	return copied, rows.Close()
}

// streamChildTableToPG streams Veg, Humus, Mineral, or Other ordered by PlotNumber so
// per-plot duplicate/NULL ID repair uses O(1) memory across 1.65M+ rows.
func streamChildTableToPG(
	ctx context.Context,
	tx pgx.Tx,
	sqliteDB *sql.DB,
	sqliteTable string,
	pgSchema string,
	pgTable string,
	cols []ColumnSpec,
	allowedPlots map[string]struct{},
	submissionID *string,
) (int64, error) {
	var maxExistingID sql.NullInt64
	_ = sqliteDB.QueryRowContext(ctx, fmt.Sprintf(`SELECT MAX("ID") FROM %s WHERE typeof("ID") = 'integer'`, quotePGIdent(sqliteTable))).Scan(&maxExistingID)
	nextSyntheticID := int64(100000000)
	if maxExistingID.Valid && maxExistingID.Int64 >= nextSyntheticID {
		nextSyntheticID = maxExistingID.Int64 + 1
	}

	query := fmt.Sprintf(`SELECT %s FROM %s WHERE typeof("PlotNumber") = 'text' ORDER BY CAST("PlotNumber" AS BLOB)`,
		sqliteSelectColumnsCSV(cols), quotePGIdent(sqliteTable))
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", sqliteTable, err)
	}
	defer rows.Close()

	plotIdx, idIdx := -1, -1
	for i, c := range cols {
		if c.Name == "PlotNumber" {
			plotIdx = i
		} else if c.Name == "ID" {
			idIdx = i
		}
	}

	hasSubID := submissionID != nil && *submissionID != ""
	copyCols := columnNames(cols)
	if hasSubID {
		copyCols = append([]string{"submission_id"}, copyCols...)
	}

	rawVals := make([]any, len(cols))
	scanDest := make([]any, len(cols))
	for i := range rawVals {
		scanDest[i] = &rawVals[i]
	}

	seenIDsTableWide := map[int64]struct{}{}
	var streamErr error

	copied, copyErr := tx.CopyFrom(
		ctx,
		pgx.Identifier{pgSchema, pgTable},
		copyCols,
		pgx.CopyFromFunc(func() ([]any, error) {
			for {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !rows.Next() {
					return nil, rows.Err()
				}
				if err := rows.Scan(scanDest...); err != nil {
					return nil, err
				}
				pVal, pErr := CanonicalizeSQLiteToPG(cols[plotIdx], rawVals[plotIdx])
				if pErr != nil || pVal == nil {
					continue
				}
				plotStr, ok := pVal.(string)
				if !ok || plotStr == "" {
					continue
				}
				if allowedPlots != nil {
					if _, exists := allowedPlots[plotStr]; !exists {
						continue
					}
				}

				offset := 0
				out := make([]any, len(copyCols))
				if hasSubID {
					out[0] = *submissionID
					offset = 1
				}
				for i, col := range cols {
					conv, cErr := CanonicalizeSQLiteToPG(col, rawVals[i])
					if cErr != nil {
						streamErr = fmt.Errorf("%s.%s (plot=%q): %w", sqliteTable, col.Name, plotStr, cErr)
						return nil, streamErr
					}
					out[offset+i] = conv
				}

				var rowID int64
				if out[offset+idIdx] == nil {
					rowID = nextSyntheticID
					nextSyntheticID++
				} else {
					rowID = out[offset+idIdx].(int64)
					if _, dup := seenIDsTableWide[rowID]; dup {
						rowID = nextSyntheticID
						nextSyntheticID++
					}
				}
				for {
					if _, dup := seenIDsTableWide[rowID]; !dup {
						break
					}
					rowID = nextSyntheticID
					nextSyntheticID++
				}
				seenIDsTableWide[rowID] = struct{}{}
				out[offset+idIdx] = rowID
				return out, nil
			}
		}),
	)
	if streamErr != nil {
		return 0, streamErr
	}
	if copyErr != nil {
		return 0, copyErr
	}
	return copied, rows.Close()
}

func copySQLiteRetiredChildIdentities(ctx context.Context, tx pgx.Tx, sqliteDB *sql.DB, project string) error {
	var hasTable bool
	if err := sqliteDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='__VPRO_ChildIdentity')`).Scan(&hasTable); err != nil || !hasTable {
		return nil
	}

	// Select only retired/deleted IDs not already present in the live child tables, then bulk-stream via CopyFrom.
	type suffixSpec struct {
		suffix string
		pgName string
	}
	var pairs [][2]any
	for _, sp := range []suffixSpec{
		{"Veg", "veg"},
		{"Humus", "humus"},
		{"Mineral", "mineral"},
		{"Other", "other"},
	} {
		rawName := project + "_" + sp.suffix
		quotedName := quotePGIdent(rawName)
		q := fmt.Sprintf(`
			SELECT "ID" FROM "__VPRO_ChildIdentity"
			WHERE typeof("ID") = 'integer' AND ("ChildTable" = ? OR "ChildTable" = ?)
			EXCEPT
			SELECT "ID" FROM %s WHERE typeof("ID") = 'integer'
		`, quotedName)
		rows, err := sqliteDB.QueryContext(ctx, q, rawName, quotedName)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				pairs = append(pairs, [2]any{sp.pgName, id})
			}
		}
		rows.Close()
	}
	if len(pairs) == 0 {
		return nil
	}

	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE IF NOT EXISTS _vpro_retired_ids (child_table TEXT NOT NULL, id BIGINT NOT NULL) ON COMMIT DROP`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `TRUNCATE TABLE _vpro_retired_ids`); err != nil {
		return err
	}
	idx := 0
	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"_vpro_retired_ids"},
		[]string{"child_table", "id"},
		pgx.CopyFromFunc(func() ([]any, error) {
			if idx >= len(pairs) {
				return nil, nil
			}
			p := pairs[idx]
			idx++
			return []any{p[0], p[1]}, nil
		}),
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO becmaster.child_identity (child_table, id)
		SELECT child_table, id FROM _vpro_retired_ids
		ON CONFLICT (child_table, id) DO NOTHING
	`)
	return err
}

func streamMasterAuditToPG(ctx context.Context, tx pgx.Tx, sqliteDB *sql.DB, sqliteAuditTable, initialPrevHash string) (int64, string, error) {
	var hasAudit bool
	if err := sqliteDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, sqliteAuditTable).Scan(&hasAudit); err != nil || !hasAudit {
		return 0, initialPrevHash, err
	}
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY rowid", sqliteSelectColumnsCSV(VP08AuditColumns), quotePGIdent(sqliteAuditTable))
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return 0, initialPrevHash, err
	}
	defer rows.Close()

	copyCols := append(columnNames(VP08AuditColumns), "merge_tx_id", "prev_hash", "row_hash")
	rawVals := make([]any, len(VP08AuditColumns))
	scanDest := make([]any, len(VP08AuditColumns))
	for i := range rawVals {
		scanDest[i] = &rawVals[i]
	}

	prevHash := initialPrevHash
	var streamErr error
	copied, copyErr := tx.CopyFrom(
		ctx,
		pgx.Identifier{"becmaster", "audit"},
		copyCols,
		pgx.CopyFromFunc(func() ([]any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !rows.Next() {
				return nil, rows.Err()
			}
			if err := rows.Scan(scanDest...); err != nil {
				return nil, err
			}
			vp08Vals := make([]any, len(VP08AuditColumns))
			for i, col := range VP08AuditColumns {
				conv, cErr := CanonicalizeSQLiteToPG(col, rawVals[i])
				if cErr != nil {
					// Tolerate historical malformed text in legacy 2008 Audit memos by sanitizing invalid UTF-8 only on baseline audit import
					if s, ok := rawVals[i].(string); ok {
						conv = strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "\uFFFD")
					} else if b, ok := rawVals[i].([]byte); ok {
						conv = strings.ToValidUTF8(strings.ReplaceAll(string(b), "\x00", ""), "\uFFFD")
					} else {
						streamErr = cErr
						return nil, streamErr
					}
				}
				vp08Vals[i] = conv
			}
			rowHash := ComputeAuditRowHash(prevHash, "", vp08Vals)
			out := make([]any, len(copyCols))
			copy(out, vp08Vals)
			out[len(VP08AuditColumns)] = nil // merge_tx_id
			out[len(VP08AuditColumns)+1] = prevHash
			out[len(VP08AuditColumns)+2] = rowHash
			prevHash = rowHash
			return out, nil
		}),
	)
	if streamErr != nil {
		return 0, prevHash, streamErr
	}
	if copyErr != nil {
		return 0, prevHash, copyErr
	}
	return copied, prevHash, rows.Close()
}
