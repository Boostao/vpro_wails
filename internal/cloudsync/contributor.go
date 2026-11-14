package cloudsync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PullStandardsResult reports whether local VLists.db was already current or atomically updated.
type PullStandardsResult struct {
	Updated         bool                   `json:"updated"`
	AlreadyCurrent  bool                   `json:"alreadyCurrent"`
	ReleaseID       string                 `json:"releaseId"`
	BECVersion      string                 `json:"becVersion"`
	TaxonomyVersion string                 `json:"taxonomyVersion"`
	LocalSHA256     string                 `json:"localSha256"`
	CloudSHA256     string                 `json:"cloudSha256"`
	PublishedAt     string                 `json:"publishedAt,omitempty"`
	State           *vprocore.ProjectState `json:"state,omitempty"`
}

// SubsetFilter specifies criteria for querying/pulling a regional BECMaster subset.
type SubsetFilter struct {
	Zones             []string  `json:"zones,omitempty"`
	SubZones          []string  `json:"subzones,omitempty"`
	ProjectIDs        []string  `json:"projectIds,omitempty"`
	PlotNumbers       []string  `json:"plotNumbers,omitempty"`
	BBox              []float64 `json:"bbox,omitempty"` // [minLon, minLat, maxLon, maxLat]
	Limit             int       `json:"limit,omitempty"`
	AllowFullCheckout bool      `json:"allowFullCheckout,omitempty"`
}

func (r *SubsetFilter) UnmarshalJSON(data []byte) error {
	type plain SubsetFilter
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// SubsetPreview summarizes matching plot counts before downloading a local splinter .db.
type SubsetPreview struct {
	MatchingPlots int            `json:"matchingPlots"`
	MatchingVeg   int64          `json:"matchingVeg"`
	SamplePlots   []string       `json:"samplePlots"`
	ZoneBreakdown map[string]int `json:"zoneBreakdown"`
}

// PullSubsetRequest downloads a filtered BECMaster subset into a fresh local VP08 SQLite .db.
type PullSubsetRequest struct {
	Filter            SubsetFilter `json:"filter"`
	TargetProject     string       `json:"targetProject"`
	DestinationPath   string       `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool         `json:"activateOnSuccess"`
}

func (r *PullSubsetRequest) UnmarshalJSON(data []byte) error {
	type plain PullSubsetRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// PullSubsetResult summarizes a completed regional subset download and download_log receipt.
type PullSubsetResult struct {
	TargetProject   string                 `json:"targetProject"`
	DestinationPath string                 `json:"destinationPath"`
	DestinationSHA  string                 `json:"destinationSha256"`
	PlotCount       int                    `json:"plotCount"`
	VegCount        int                    `json:"vegCount"`
	HumusCount      int                    `json:"humusCount"`
	MineralCount    int                    `json:"mineralCount"`
	OtherCount      int                    `json:"otherCount"`
	MetadataCount   int                    `json:"metadataCount"`
	DownloadLogID   int64                  `json:"downloadLogId"`
	Activated       bool                   `json:"activated"`
	State           *vprocore.ProjectState `json:"state,omitempty"`
}

// CloudUserSpeciesRecord mirrors VUser.USysUserSpp entries packaged with staging uploads.
type CloudUserSpeciesRecord struct {
	Code           string   `json:"code"`
	ScientificName *string  `json:"scientificName"`
	LifeForm       *int     `json:"lifeForm"`
	EnglishName    *string  `json:"englishName"`
	Report         *float64 `json:"report,omitempty"`
	SppNumber      *int     `json:"sppNumber,omitempty"`
	Codetype       *string  `json:"codetype,omitempty"`
}

// CloudUserSpeciesManifest packages personal species referenced by a submitted project.
type CloudUserSpeciesManifest struct {
	SchemaVersion string                   `json:"schemaVersion"`
	ExportedAt    string                   `json:"exportedAt"`
	SourceProject string                   `json:"sourceProject"`
	RowCount      int                      `json:"rowCount"`
	PayloadSHA256 string                   `json:"payloadSha256"`
	Species       []CloudUserSpeciesRecord `json:"species"`
}

// ComplianceGateResult decouples internal/cloudsync (Layer 2) from internal/validation (Layer 2).
type ComplianceGateResult struct {
	Passed        bool            `json:"passed"`
	BlockingCount int             `json:"blockingCount"`
	Summary       map[string]int  `json:"summary"`
	ReportJSON    json.RawMessage `json:"report"`
}

// ComplianceValidatorFunc runs compliance pre-flight validation against the active SQLite context.
type ComplianceValidatorFunc func(ctx context.Context, s *vprocore.ContextService, contextID string, useActiveSU bool) (ComplianceGateResult, error)

// StagingLumpRow captures an active <Project>_Lump row packaged in staging.submissions.
type StagingLumpRow struct {
	LumpCode string `json:"lumpCode"`
	SppCode  string `json:"sppCode"`
	Use      bool   `json:"use"`
}

// StagingSURow captures an active *_SU assignment packaged in staging.submissions.
type StagingSURow struct {
	PlotNumber string  `json:"plotNumber"`
	SiteUnit   *string `json:"siteUnit"`
}

// StagingPreflightPreview summarizes compliance, personal species, Lump, and SU counts prior to upload.
type StagingPreflightPreview struct {
	Project             string                   `json:"project"`
	ProjectID           string                   `json:"projectId"`
	PlotCount           int                      `json:"plotCount"`
	VegRowCount         int                      `json:"vegRowCount"`
	SourceDBSHA256      string                   `json:"sourceDbSha256"`
	CompliancePassed    bool                     `json:"compliancePassed"`
	BlockingErrorCount  int                      `json:"blockingErrorCount"`
	ComplianceSummary   map[string]int           `json:"complianceSummary"`
	ComplianceReport    json.RawMessage          `json:"complianceReport"`
	UserSpeciesManifest CloudUserSpeciesManifest `json:"userSpeciesManifest"`
	LumpRowCount        int                      `json:"lumpRowCount"`
	SURowCount          int                      `json:"suRowCount"`
}

// SubmitStagingRequest uploads the active local VP08 project to staging.submissions.
type SubmitStagingRequest struct {
	UseActiveSU       bool   `json:"useActiveSu,omitempty"`
	SubmitterNotes    string `json:"submitterNotes,omitempty"`
	SubmittedBy       string `json:"submittedBy,omitempty"`
	BaseSnapshotTag   string `json:"baseSnapshotTag,omitempty"`
	CustodianOverride bool   `json:"custodianOverride,omitempty"`
}

func (r *SubmitStagingRequest) UnmarshalJSON(data []byte) error {
	type plain SubmitStagingRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// SubmitStagingResult confirms atomic staging upload into PostgreSQL.
type SubmitStagingResult struct {
	SubmissionID      string `json:"submissionId"`
	ProjectName       string `json:"projectName"`
	ProjectID         string `json:"projectId"`
	SubmittedBy       string `json:"submittedBy"`
	DBRole            string `json:"dbRole"`
	Status            string `json:"status"`
	PlotCount         int    `json:"plotCount"`
	VegRowCount       int    `json:"vegRowCount"`
	UserSpeciesCount  int    `json:"userSpeciesCount"`
	SourceDBSHA256    string `json:"sourceDbSha256"`
	CompliancePassed  bool   `json:"compliancePassed"`
	SubmittedAt       string `json:"submittedAt"`
}

func validateSplinterProjectName(raw string) (string, error) {
	if !utf8.ValidString(raw) || strings.ContainsRune(raw, 0) {
		return "", errors.New("target project name must be valid UTF-8 without NUL")
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed != raw {
		return "", errors.New("target project name must not be empty or contain leading/trailing whitespace")
	}
	if !vprocore.ProjectNamePattern.MatchString(trimmed) {
		return "", errors.New("target project name must start with a letter and use only ASCII letters, digits, or underscores (max 31 chars)")
	}
	lower := strings.ToLower(trimmed)
	if lower == "none" || lower == "sample" || strings.Contains(lower, "master") {
		return "", fmt.Errorf("target project name %q is reserved (None, Sample, and master names are not permitted)", trimmed)
	}
	return trimmed, nil
}

// PullCloudStandards compares standards.library_releases SHA-256 against local VLists.db
// and atomically swaps + reloads local VLists.db when a newer cloud release is active.
func (m *Manager) PullCloudStandards(ctx context.Context, s *vprocore.ContextService, contextID string) (PullStandardsResult, error) {
	if s == nil {
		return PullStandardsResult{}, errors.New("project context service is unavailable")
	}
	var localVListsPath string
	if _, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		localVListsPath = plots.Projects.SupportPaths["VLists"]
		if localVListsPath == "" && plots.Projects.Sqlite != nil {
			localVListsPath = plots.Projects.Sqlite.Attachments["VLists"]
		}
		return struct{}{}, nil
	}); err != nil {
		return PullStandardsResult{}, err
	}
	if strings.TrimSpace(localVListsPath) == "" {
		return PullStandardsResult{}, errors.New("active context has no attached VLists.db path")
	}

	localBytes, err := os.ReadFile(localVListsPath)
	if err != nil {
		return PullStandardsResult{}, fmt.Errorf("read local VLists.db: %w", err)
	}
	localSum := sha256.Sum256(localBytes)
	localSHA := hex.EncodeToString(localSum[:])

	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return PullStandardsResult{}, err
	}

	var releaseID, becVer, taxVer, cloudSHA string
	var pubAt time.Time
	var gzBytes []byte
	err = pool.QueryRow(ctx, `
		SELECT release_id, bec_version, taxonomy_version, vlists_sha256, published_at, vlists_gzip
		FROM standards.library_releases
		WHERE is_current = TRUE
		ORDER BY published_at DESC
		LIMIT 1
	`).Scan(&releaseID, &becVer, &taxVer, &cloudSHA, &pubAt, &gzBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PullStandardsResult{}, errors.New("no active provincial standards release has been published in standards.library_releases")
		}
		return PullStandardsResult{}, wrapRedactedError("query cloud standards release", err, creds)
	}

	if strings.EqualFold(localSHA, cloudSHA) {
		return PullStandardsResult{
			Updated:         false,
			AlreadyCurrent:  true,
			ReleaseID:       releaseID,
			BECVersion:      becVer,
			TaxonomyVersion: taxVer,
			LocalSHA256:     localSHA,
			CloudSHA256:     cloudSHA,
			PublishedAt:     pubAt.UTC().Format(time.RFC3339),
		}, nil
	}

	var stagedBytes []byte
	if len(gzBytes) > 0 {
		decompressed, dErr := decompressGzip(gzBytes)
		if dErr != nil {
			return PullStandardsResult{}, fmt.Errorf("decompress cloud VLists snapshot: %w", dErr)
		}
		gotSum := sha256.Sum256(decompressed)
		gotSHA := hex.EncodeToString(gotSum[:])
		if !strings.EqualFold(gotSHA, cloudSHA) {
			return PullStandardsResult{}, fmt.Errorf("cloud VLists SHA-256 mismatch (expected %s, got %s)", cloudSHA, gotSHA)
		}
		stagedBytes = decompressed
	} else {
		rebuilt, rErr := materializeVListsFromRelationalStandards(ctx, pool, localBytes, becVer, taxVer)
		if rErr != nil {
			return PullStandardsResult{}, wrapRedactedError("rebuild VLists.db from cloud standards tables", rErr, creds)
		}
		stagedBytes = rebuilt
	}

	newState, err := vprocore.ReplaceActiveVListsCatalogue(ctx, s, contextID, stagedBytes)
	if err != nil {
		return PullStandardsResult{}, err
	}
	finalSum := sha256.Sum256(stagedBytes)
	finalSHA := hex.EncodeToString(finalSum[:])

	return PullStandardsResult{
		Updated:         true,
		AlreadyCurrent:  false,
		ReleaseID:       releaseID,
		BECVersion:      becVer,
		TaxonomyVersion: taxVer,
		LocalSHA256:     finalSHA,
		CloudSHA256:     cloudSHA,
		PublishedAt:     pubAt.UTC().Format(time.RFC3339),
		State:           &newState,
	}, nil
}

func materializeVListsFromRelationalStandards(ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, baseVListsBytes []byte, becVer, taxVer string) ([]byte, error) {
	tmpFile, err := os.CreateTemp("", "vpro-vlists-rebuild-*.db")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(baseVListsBytes); err != nil {
		_ = tmpFile.Close()
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	syncTable := func(pgTable, sqliteTable string, cols []ColumnSpec) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+quotePGIdent(sqliteTable)); err != nil {
			return err
		}
		pRows, qErr := pool.Query(ctx, fmt.Sprintf("SELECT %s FROM standards.%s ORDER BY row_seq", quotedColumnCSV(cols), quotePGIdent(pgTable)))
		if qErr != nil {
			return qErr
		}
		defer pRows.Close()

		marks := make([]string, len(cols))
		for i := range marks {
			marks[i] = "?"
		}
		stmt, sErr := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			quotePGIdent(sqliteTable), quotedColumnCSV(cols), strings.Join(marks, ", ")))
		if sErr != nil {
			return sErr
		}
		defer stmt.Close()

		for pRows.Next() {
			vals, vErr := pRows.Values()
			if vErr != nil {
				return vErr
			}
			args := make([]any, len(cols))
			for i, col := range cols {
				args[i] = CanonicalizePGToSQLite(col, vals[i])
			}
			if _, eErr := stmt.ExecContext(ctx, args...); eErr != nil {
				return eErr
			}
		}
		return pRows.Err()
	}

	for _, item := range []struct {
		pgTbl     string
		sqliteTbl string
		cols      []ColumnSpec
	}{
		{"usys_all_specs", "USysAllSpecs", StandardsUSysAllSpecsColumns},
		{"usys_table_of_lists", "USysTableOfLists", StandardsUSysTableOfListsColumns},
		{"master_site_unit_list", "MasterSiteUnitList", StandardsMasterSiteUnitListColumns},
		{"usys_zone_list", "USysZoneList", StandardsUSysZoneListColumns},
	} {
		if err := syncTable(item.pgTbl, item.sqliteTbl, item.cols); err != nil {
			_ = tx.Rollback()
			_ = db.Close()
			return nil, err
		}
	}
	_, _ = tx.ExecContext(ctx, `UPDATE "_table_metadata" SET "description" = ? WHERE "table_name" = 'USysAllSpecs'`, taxVer)
	_, _ = tx.ExecContext(ctx, `UPDATE "_table_metadata" SET "description" = ? WHERE "table_name" = 'USysTableOfLists'`, becVer)
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmpPath)
}

func cleanTokens(items []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range items {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; !dup {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

func buildSubsetWhereClause(filter SubsetFilter, hasPostGISGeom bool) (whereSQL string, args []any, err error) {
	zones := cleanTokens(filter.Zones)
	subzones := cleanTokens(filter.SubZones)
	projectIDs := cleanTokens(filter.ProjectIDs)
	plots := cleanTokens(filter.PlotNumbers)

	if len(filter.BBox) != 0 && len(filter.BBox) != 4 {
		return "", nil, errors.New("bbox filter must contain exactly 4 numbers [minLon, minLat, maxLon, maxLat]")
	}
	var clauses []string
	nextArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if len(zones) > 0 {
		clauses = append(clauses, fmt.Sprintf(`"Zone" = ANY(%s)`, nextArg(zones)))
	}
	if len(subzones) > 0 {
		clauses = append(clauses, fmt.Sprintf(`"SubZone" = ANY(%s)`, nextArg(subzones)))
	}
	if len(projectIDs) > 0 {
		clauses = append(clauses, fmt.Sprintf(`"ProjectID" = ANY(%s)`, nextArg(projectIDs)))
	}
	if len(plots) > 0 {
		// Byte-exact TEXT array match in PostgreSQL preserves leading-zero PlotNumbers ('00337' != '337')
		clauses = append(clauses, fmt.Sprintf(`"PlotNumber" = ANY(%s)`, nextArg(plots)))
	}
	if len(filter.BBox) == 4 {
		minLon, minLat, maxLon, maxLat := filter.BBox[0], filter.BBox[1], filter.BBox[2], filter.BBox[3]
		for _, f := range filter.BBox {
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return "", nil, errors.New("bbox coordinates must be finite numbers")
			}
		}
		if minLon > maxLon || minLat > maxLat {
			return "", nil, errors.New("bbox requires minLon <= maxLon and minLat <= maxLat")
		}
		pMinLon := nextArg(minLon)
		pMinLat := nextArg(minLat)
		pMaxLon := nextArg(maxLon)
		pMaxLat := nextArg(maxLat)
		if hasPostGISGeom {
			clauses = append(clauses, fmt.Sprintf(
				`(geom && public.ST_MakeEnvelope(%s, %s, %s, %s, 4326) AND "Longitude" BETWEEN %s AND %s AND "Latitude" BETWEEN %s AND %s)`,
				pMinLon, pMinLat, pMaxLon, pMaxLat, pMinLon, pMaxLon, pMinLat, pMaxLat,
			))
		} else {
			clauses = append(clauses, fmt.Sprintf(
				`("Longitude" BETWEEN %s AND %s AND "Latitude" BETWEEN %s AND %s)`,
				pMinLon, pMaxLon, pMinLat, pMaxLat,
			))
		}
	}

	if len(clauses) == 0 && !filter.AllowFullCheckout {
		return "", nil, errors.New("at least one subset filter (Zone, SubZone, ProjectID, PlotNumber, or BBox) is required")
	}
	if filter.Limit < 0 {
		return "", nil, errors.New("subset limit must be >= 0")
	}
	if len(clauses) == 0 {
		return "", args, nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, nil
}

func hasEnvGeomColumn(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) bool {
	var ok bool
	_ = q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'becmaster' AND table_name = 'env' AND column_name = 'geom'
		)
	`).Scan(&ok)
	return ok
}

// PreviewBECMasterSubset previews matching plot and vegetation row counts for a regional checkout filter.
func (m *Manager) PreviewBECMasterSubset(ctx context.Context, filter SubsetFilter) (SubsetPreview, error) {
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return SubsetPreview{}, err
	}
	hasGeom := hasEnvGeomColumn(ctx, pool)
	whereSQL, args, err := buildSubsetWhereClause(filter, hasGeom)
	if err != nil {
		return SubsetPreview{}, err
	}

	limitClause := ""
	if filter.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	cte := fmt.Sprintf(`WITH matched AS (SELECT "PlotNumber", COALESCE("Zone", '(Unassigned)') AS z FROM becmaster.env %s ORDER BY "PlotNumber"%s)`, whereSQL, limitClause)

	var plotCount int
	var vegCount int64
	if err := pool.QueryRow(ctx, cte+`
		SELECT
			(SELECT COUNT(*) FROM matched),
			(SELECT COUNT(*) FROM becmaster.veg v WHERE v."PlotNumber" IN (SELECT "PlotNumber" FROM matched))
	`, args...).Scan(&plotCount, &vegCount); err != nil {
		return SubsetPreview{}, wrapRedactedError("preview BECMaster subset", err, creds)
	}

	zones := map[string]int{}
	zRows, err := pool.Query(ctx, cte+` SELECT z, COUNT(*) FROM matched GROUP BY z ORDER BY z`, args...)
	if err != nil {
		return SubsetPreview{}, wrapRedactedError("preview BECMaster zone breakdown", err, creds)
	}
	for zRows.Next() {
		var z string
		var c int
		if sErr := zRows.Scan(&z, &c); sErr == nil {
			zones[z] = c
		}
	}
	zRows.Close()

	samplePlots := []string{}
	sRows, err := pool.Query(ctx, cte+` SELECT "PlotNumber" FROM matched ORDER BY "PlotNumber" LIMIT 20`, args...)
	if err == nil {
		for sRows.Next() {
			var p string
			if sRows.Scan(&p) == nil {
				samplePlots = append(samplePlots, p)
			}
		}
		sRows.Close()
	}

	return SubsetPreview{
		MatchingPlots: plotCount,
		MatchingVeg:   vegCount,
		SamplePlots:   samplePlots,
		ZoneBreakdown: zones,
	}, nil
}

// PullBECMasterSubset streams a filtered BECMaster subset into a fresh local VP08 SQLite .db
// (WAL mode, _table_metadata 'VP08', __VPRO_ChildIdentity seeded) and logs to becmaster.download_log.
func (m *Manager) PullBECMasterSubset(ctx context.Context, s *vprocore.ContextService, contextID string, req PullSubsetRequest) (PullSubsetResult, error) {
	if s == nil {
		return PullSubsetResult{}, errors.New("project context service is unavailable")
	}
	targetProject, err := validateSplinterProjectName(req.TargetProject)
	if err != nil {
		return PullSubsetResult{}, err
	}

	var projectsDir string
	actor := "VProUser"
	if _, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		projectsDir = filepath.Join(plots.Projects.Root, "projects")
		if u := strings.TrimSpace(plots.CurrentUser); u != "" {
			actor = u
		}
		return struct{}{}, nil
	}); err != nil {
		return PullSubsetResult{}, err
	}

	destReq := strings.TrimSpace(req.DestinationPath)
	if destReq == "" {
		destReq = filepath.Join(projectsDir, targetProject+".db")
	} else if !strings.EqualFold(filepath.Ext(destReq), ".db") {
		destReq += ".db"
	}
	resolvedDest, parentInfo, err := vprocore.FreshSQLiteDestination(destReq, "Project")
	if err != nil {
		return PullSubsetResult{}, err
	}

	review, err := s.ReadNewProjectTemplates(ctx, contextID)
	if err != nil {
		return PullSubsetResult{}, err
	}
	ddlPlan, err := vprocore.PlanNewProjectDDL(ctx, targetProject, review)
	if err != nil {
		return PullSubsetResult{}, err
	}

	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return PullSubsetResult{}, err
	}
	hasGeom := hasEnvGeomColumn(ctx, pool)
	whereSQL, args, err := buildSubsetWhereClause(req.Filter, hasGeom)
	if err != nil {
		return PullSubsetResult{}, err
	}
	limitClause := ""
	if req.Filter.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT %d", req.Filter.Limit)
	}

	// Fetch matched plot list inside a REPEATABLE READ PostgreSQL read transaction so parent & child tables are snapshot-consistent.
	pgTx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PullSubsetResult{}, wrapRedactedError("begin subset snapshot", err, creds)
	}
	defer func() { _ = pgTx.Rollback(ctx) }()

	pRows, err := pgTx.Query(ctx, fmt.Sprintf(`SELECT "PlotNumber" FROM becmaster.env %s ORDER BY "PlotNumber"%s`, whereSQL, limitClause), args...)
	if err != nil {
		return PullSubsetResult{}, wrapRedactedError("query matching BECMaster plots", err, creds)
	}
	var plotList []string
	for pRows.Next() {
		var p string
		if err := pRows.Scan(&p); err != nil {
			pRows.Close()
			return PullSubsetResult{}, wrapRedactedError("scan matching BECMaster plot", err, creds)
		}
		plotList = append(plotList, p)
	}
	pRows.Close()
	if err := pRows.Err(); err != nil {
		return PullSubsetResult{}, wrapRedactedError("read matching BECMaster plots", err, creds)
	}
	if len(plotList) == 0 {
		return PullSubsetResult{}, errors.New("subset filter matched 0 plots in BECMaster")
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-pgsubset-*.db")
	if err != nil {
		return PullSubsetResult{}, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return PullSubsetResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		return PullSubsetResult{}, err
	}

	sqlDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return PullSubsetResult{}, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqTx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		_ = sqlDB.Close()
		return PullSubsetResult{}, err
	}

	var vegN, humusN, mineralN, otherN, metaN int
	populate := func() error {
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					if _, err := sqTx.ExecContext(ctx, *mapping.SQL); err != nil {
						return fmt.Errorf("create SQLite DDL for %s: %w", tblPlan.TargetTable, err)
					}
				}
			}
		}
		if _, err := sqTx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range vprocore.CoreTables {
			if _, err := sqTx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, targetProject+"_"+suffix); err != nil {
				return err
			}
		}
		if _, err := sqTx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" ("ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable", "ID"))`); err != nil {
			return err
		}

		streamPGToSQLite := func(pgTbl, plotCol, sqliteSuffix string, cols []ColumnSpec) (int, error) {
			sqlQuery := fmt.Sprintf(`SELECT %s FROM becmaster.%s WHERE %s = ANY($1)`, quotedColumnCSV(cols), quotePGIdent(pgTbl), quotePGIdent(plotCol))
			rows, qErr := pgTx.Query(ctx, sqlQuery, plotList)
			if qErr != nil {
				return 0, qErr
			}
			defer rows.Close()

			marks := make([]string, len(cols))
			idColIdx, sppColIdx := -1, -1
			for i, col := range cols {
				marks[i] = "?"
				if col.Name == "ID" {
					idColIdx = i
				} else if sqliteSuffix == "Veg" && col.Name == "Species" {
					sppColIdx = i
				}
			}
			enforceUniqueID := sqliteSuffix == "Humus" || sqliteSuffix == "Mineral" || sqliteSuffix == "Other" || sqliteSuffix == "Veg"
			usedIDs := map[int64]bool{}
			nextSyntheticID := int64(100000000)

			stmt, sErr := sqTx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				quotePGIdent(targetProject+"_"+sqliteSuffix), quotedColumnCSV(cols), strings.Join(marks, ", ")))
			if sErr != nil {
				return 0, sErr
			}
			defer stmt.Close()

			count := 0
			for rows.Next() {
				vals, vErr := rows.Values()
				if vErr != nil {
					return 0, vErr
				}
				rowArgs := make([]any, len(cols))
				for i, col := range cols {
					rowArgs[i] = CanonicalizePGToSQLite(col, vals[i])
				}
				if sppColIdx >= 0 && rowArgs[sppColIdx] == nil {
					rowArgs[sppColIdx] = ""
				}
				if enforceUniqueID && idColIdx >= 0 {
					var idVal int64
					hasID := false
					if iv, ok := rowArgs[idColIdx].(int64); ok {
						idVal = iv
						hasID = true
					}
					if !hasID || usedIDs[idVal] {
						for usedIDs[nextSyntheticID] {
							nextSyntheticID++
						}
						idVal = nextSyntheticID
						nextSyntheticID++
						rowArgs[idColIdx] = idVal
					}
					usedIDs[idVal] = true
				}
				if _, eErr := stmt.ExecContext(ctx, rowArgs...); eErr != nil {
					return 0, eErr
				}
				count++
			}
			return count, rows.Err()
		}

		if _, err := streamPGToSQLite("env", "PlotNumber", "Env", VP08EnvColumns); err != nil {
			return fmt.Errorf("stream env: %w", err)
		}
		if _, err := streamPGToSQLite("admin", "Plot", "Admin", VP08AdminColumns); err != nil {
			return fmt.Errorf("stream admin: %w", err)
		}
		if vegN, err = streamPGToSQLite("veg", "PlotNumber", "Veg", VP08VegColumns); err != nil {
			return fmt.Errorf("stream veg: %w", err)
		}
		if humusN, err = streamPGToSQLite("humus", "PlotNumber", "Humus", VP08HumusColumns); err != nil {
			return fmt.Errorf("stream humus: %w", err)
		}
		if mineralN, err = streamPGToSQLite("mineral", "PlotNumber", "Mineral", VP08MineralColumns); err != nil {
			return fmt.Errorf("stream mineral: %w", err)
		}
		if otherN, err = streamPGToSQLite("other", "PlotNumber", "Other", VP08OtherColumns); err != nil {
			return fmt.Errorf("stream other: %w", err)
		}
		if _, err := streamPGToSQLite("audit", "PlotNumber", "Audit", VP08AuditColumns); err != nil {
			return fmt.Errorf("stream audit: %w", err)
		}

		// Normalize imported Audit Project column to targetProject so PlotEntryService audit ownership checks pass cleanly!
		if _, err := sqTx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET "Project" = ?`, quotePGIdent(targetProject+"_Audit")), targetProject); err != nil {
			return err
		}

		// Stream matching Metadata rows by ProjectIDs referenced in <Target>_Env
		projIDRows, pErr := sqTx.QueryContext(ctx, fmt.Sprintf(`SELECT DISTINCT "ProjectID" FROM %s WHERE typeof("ProjectID") = 'text' AND "ProjectID" <> ''`, quotePGIdent(targetProject+"_Env")))
		if pErr != nil {
			return pErr
		}
		var refProjIDs []string
		for projIDRows.Next() {
			var pid string
			if projIDRows.Scan(&pid) == nil {
				refProjIDs = append(refProjIDs, pid)
			}
		}
		projIDRows.Close()

		if len(refProjIDs) > 0 {
			mRows, mErr := pgTx.Query(ctx, fmt.Sprintf(`SELECT %s FROM becmaster.metadata WHERE "ProjectID" = ANY($1)`, quotedColumnCSV(VP08MetadataColumns)), refProjIDs)
			if mErr != nil {
				return mErr
			}
			defer mRows.Close()
			marks := make([]string, len(VP08MetadataColumns))
			metaIDIdx := -1
			for i, col := range VP08MetadataColumns {
				marks[i] = "?"
				if col.Name == "ID" {
					metaIDIdx = i
				}
			}
			usedMetaIDs := map[int64]bool{}
			nextMetaID := int64(1)
			mStmt, sErr := sqTx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
				quotePGIdent(targetProject+"_Metadata"), quotedColumnCSV(VP08MetadataColumns), strings.Join(marks, ", ")))
			if sErr != nil {
				return sErr
			}
			defer mStmt.Close()
			for mRows.Next() {
				vals, vErr := mRows.Values()
				if vErr != nil {
					return vErr
				}
				rowArgs := make([]any, len(VP08MetadataColumns))
				for i, col := range VP08MetadataColumns {
					rowArgs[i] = CanonicalizePGToSQLite(col, vals[i])
				}
				if metaIDIdx >= 0 {
					var mid int64
					hasMID := false
					if iv, ok := rowArgs[metaIDIdx].(int64); ok && iv > 0 {
						mid = iv
						hasMID = true
					}
					if !hasMID || usedMetaIDs[mid] {
						for usedMetaIDs[nextMetaID] {
							nextMetaID++
						}
						mid = nextMetaID
						nextMetaID++
						rowArgs[metaIDIdx] = mid
					}
					usedMetaIDs[mid] = true
				}
				if _, eErr := mStmt.ExecContext(ctx, rowArgs...); eErr != nil {
					return eErr
				}
				metaN++
			}
			if err := mRows.Err(); err != nil {
				return err
			}
		}
		if metaN == 0 {
			// Seed default metadata row so VP08 project metadata panels always have >=1 row
			if _, err := sqTx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s ("ProjectID", "ProjectTitle", "ID") VALUES (?, ?, 1)`, quotePGIdent(targetProject+"_Metadata")), targetProject, targetProject+" BECMaster Subset"); err != nil {
				return err
			}
			metaN = 1
		}

		// Seed __VPRO_ChildIdentity for all child tables in local splinter .db
		for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other", "Metadata", "Audit"} {
			fullTbl := targetProject + "_" + suffix
			qFull := quotePGIdent(fullTbl)
			if _, err := sqTx.ExecContext(ctx, fmt.Sprintf(
				`INSERT OR IGNORE INTO "__VPRO_ChildIdentity" ("ChildTable", "ID") SELECT ?, "ID" FROM %s WHERE typeof("ID") = 'integer' AND "ID" >= 0`,
				qFull,
			), qFull); err != nil {
				return err
			}
		}
		return nil
	}

	if err := populate(); err != nil {
		_ = sqTx.Rollback()
		_ = sqlDB.Close()
		return PullSubsetResult{}, wrapRedactedError("populate local subset SQLite database", err, creds)
	}
	if err := sqTx.Commit(); err != nil {
		_ = sqlDB.Close()
		return PullSubsetResult{}, err
	}
	if err := sqlDB.Close(); err != nil {
		return PullSubsetResult{}, err
	}
	_ = pgTx.Rollback(ctx) // Close read-only snapshot cleanly

	inspected, err := vprocore.InspectFileContext(ctx, tmpPath)
	if err != nil {
		return PullSubsetResult{}, fmt.Errorf("inspect staged subset SQLite file: %w", err)
	}
	compatible := false
	for _, p := range inspected {
		if p.Name == targetProject && p.Compatible {
			compatible = true
			break
		}
	}
	if !compatible {
		return PullSubsetResult{}, errors.New("staged subset SQLite file did not pass VP08 compatibility verification")
	}

	curParent, err := os.Stat(filepath.Dir(resolvedDest))
	if err != nil || !os.SameFile(parentInfo, curParent) {
		return PullSubsetResult{}, errors.New("destination parent directory changed during subset pull")
	}
	if err := sqlutil.LinkOrCreateExclusive(tmpPath, resolvedDest); err != nil {
		return PullSubsetResult{}, err
	}
	published = true
	_ = os.Remove(tmpPath)

	if err := vprocore.InitializeSQLiteWAL(ctx, resolvedDest); err != nil {
		_ = os.Remove(resolvedDest + "-wal")
		_ = os.Remove(resolvedDest + "-shm")
		_ = os.Remove(resolvedDest)
		return PullSubsetResult{}, err
	}

	destSHA, err := streamFileSHA256Hex(resolvedDest)
	if err != nil {
		return PullSubsetResult{}, err
	}

	// Record download activity in becmaster.download_log (Schedule A 1.v)
	filterRaw, _ := json.Marshal(req.Filter)
	roleTier := creds.ActiveRoleMode
	if roleTier == "" {
		roleTier = RoleModeContributor
	}
	var logID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO becmaster.download_log (actor, role_tier, filter_json, format, plot_count)
		VALUES ($1, $2, $3::jsonb, 'VP08_SQLite', $4)
		RETURNING id
	`, actor, roleTier, string(filterRaw), len(plotList)).Scan(&logID); err != nil {
		return PullSubsetResult{}, wrapRedactedError("record download_log entry", err, creds)
	}

	var activated bool
	var newState *vprocore.ProjectState
	if req.ActivateOnSuccess {
		st, swErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:     targetProject,
			ProjectPath: resolvedDest,
			SU:          "None",
			Hierarchy:   "None",
		})
		if swErr == nil {
			_ = s.Projects.EnsureActiveProjectWAL(ctx)
			activated = true
			newState = &st
		}
	}

	return PullSubsetResult{
		TargetProject:   targetProject,
		DestinationPath: resolvedDest,
		DestinationSHA:  destSHA,
		PlotCount:       len(plotList),
		VegCount:        vegN,
		HumusCount:      humusN,
		MineralCount:    mineralN,
		OtherCount:      otherN,
		MetadataCount:   metaN,
		DownloadLogID:   logID,
		Activated:       activated,
		State:           newState,
	}, nil
}

func extractActiveProjectSubmissionArtifacts(
	ctx context.Context,
	s *vprocore.ContextService,
	contextID string,
	useActiveSU bool,
) (projName, projPath, projID, actor, activeSU, activeSUPath string, suScopePlots map[string]struct{}, plotCount, vegCount int, userManifest CloudUserSpeciesManifest, lumpRows []StagingLumpRow, suRows []StagingSURow, err error) {
	_, err = vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		if lErr := sqlutil.AcquireMutexLease(ctx, &owner.Mu); lErr != nil {
			return struct{}{}, lErr
		}
		defer owner.Mu.Unlock()
		if owner.Conn == nil {
			return struct{}{}, errors.New("project context is closed")
		}
		projName = owner.Selection.Project
		projPath = owner.Selection.ProjectPath
		activeSU = owner.Selection.SU
		activeSUPath = owner.Selection.SUPath
		actor = strings.TrimSpace(plots.CurrentUser)
		if actor == "" {
			actor = "VProUser"
		}
		if useActiveSU && (activeSU == "" || activeSU == "None") {
			return struct{}{}, errors.New("useActiveSu is enabled, but no active Site Unit (*_SU) table is selected")
		}

		tx, bErr := owner.BeginReadSnapshot(ctx)
		if bErr != nil {
			return struct{}{}, bErr
		}
		defer func() { _ = tx.Rollback() }()

		if useActiveSU {
			suScopePlots = map[string]struct{}{}
			suFilterRows, sfErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS BLOB) FROM "su".%s WHERE typeof("PlotNumber") = 'text'`, quotePGIdent(activeSU+"_SU")))
			if sfErr != nil {
				return struct{}{}, fmt.Errorf("read active SU %s: %w", activeSU, sfErr)
			}
			for suFilterRows.Next() {
				var raw []byte
				if suFilterRows.Scan(&raw) == nil && len(raw) > 0 {
					suScopePlots[string(raw)] = struct{}{}
				}
			}
			suFilterRows.Close()
			if len(suScopePlots) == 0 {
				return struct{}{}, fmt.Errorf("active Site Unit table %q contains 0 text-keyed plots", activeSU)
			}
		}

		// Count plots & veg rows (strictly honoring suScopePlots when useActiveSU is enabled!)
		referencedSpeciesUpper := map[string]struct{}{}
		eRows, eErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS BLOB) FROM "project".%s WHERE typeof("PlotNumber") = 'text'`, quotePGIdent(projName+"_Env")))
		if eErr != nil {
			return struct{}{}, eErr
		}
		validEnvPlots := map[string]struct{}{}
		for eRows.Next() {
			var raw []byte
			if eRows.Scan(&raw) == nil {
				p := string(raw)
				if suScopePlots != nil {
					if _, ok := suScopePlots[p]; !ok {
						continue
					}
				}
				validEnvPlots[p] = struct{}{}
				plotCount++
			}
		}
		eRows.Close()

		vRows, vErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS BLOB), COALESCE(CAST("Species" AS TEXT), '') FROM "project".%s WHERE typeof("PlotNumber") = 'text'`, quotePGIdent(projName+"_Veg")))
		if vErr == nil {
			for vRows.Next() {
				var raw []byte
				var spp string
				if vRows.Scan(&raw, &spp) == nil {
					p := string(raw)
					if _, ok := validEnvPlots[p]; !ok {
						continue
					}
					vegCount++
					if trimmedSpp := strings.TrimSpace(spp); trimmedSpp != "" {
						referencedSpeciesUpper[strings.ToUpper(trimmedSpp)] = struct{}{}
					}
				}
			}
			vRows.Close()
		}

		_ = tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COALESCE(CAST("ProjectID" AS TEXT), '') FROM "project".%s LIMIT 1`, quotePGIdent(projName+"_Metadata"))).Scan(&projID)
		if strings.TrimSpace(projID) == "" {
			_ = tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COALESCE(CAST("ProjectID" AS TEXT), '') FROM "project".%s WHERE "ProjectID" IS NOT NULL AND "ProjectID" <> '' LIMIT 1`, quotePGIdent(projName+"_Env"))).Scan(&projID)
		}

		// Extract personal species from VUser.USysUserSpp referenced by the scoped <Project>_Veg rows
		uRows, uErr := tx.QueryContext(ctx, `
			SELECT u."Code", u."ScientificName", u."LifeForm", u."EnglishName", u."Report", u."SppNumber", u."Codetype"
			FROM "VUser"."USysUserSpp" u
			ORDER BY u."Code" COLLATE NOCASE, u."Code"
		`)
		var species []CloudUserSpeciesRecord
		if uErr == nil {
			for uRows.Next() {
				var code string
				var sci, eng, ctype sql.NullString
				var lf, sppNum sql.NullInt64
				var rep sql.NullFloat64
				if sErr := uRows.Scan(&code, &sci, &lf, &eng, &rep, &sppNum, &ctype); sErr == nil {
					if _, used := referencedSpeciesUpper[strings.ToUpper(strings.TrimSpace(code))]; !used {
						continue
					}
					rec := CloudUserSpeciesRecord{Code: code}
					if sci.Valid {
						v := sci.String
						rec.ScientificName = &v
					}
					if lf.Valid {
						v := int(lf.Int64)
						rec.LifeForm = &v
					}
					if eng.Valid {
						v := eng.String
						rec.EnglishName = &v
					}
					if rep.Valid {
						v := rep.Float64
						rec.Report = &v
					}
					if sppNum.Valid {
						v := int(sppNum.Int64)
						rec.SppNumber = &v
					}
					if ctype.Valid {
						v := ctype.String
						rec.Codetype = &v
					}
					species = append(species, rec)
				}
			}
			uRows.Close()
		}
		if species == nil {
			species = []CloudUserSpeciesRecord{}
		}
		canonicalBytes, _ := json.Marshal(species)
		pSum := sha256.Sum256(canonicalBytes)
		userManifest = CloudUserSpeciesManifest{
			SchemaVersion: "vpro.userspecies.v1",
			ExportedAt:    time.Now().UTC().Format(time.RFC3339),
			SourceProject: projName,
			RowCount:      len(species),
			PayloadSHA256: hex.EncodeToString(pSum[:]),
			Species:       species,
		}

		// Extract <Project>_Lump rows if present
		lumpRows = []StagingLumpRow{}
		var hasLump bool
		_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM "project".sqlite_master WHERE type='table' AND name=?)`, projName+"_Lump").Scan(&hasLump)
		if hasLump {
			lRows, lErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT COALESCE(CAST("LumpCode" AS TEXT),''), COALESCE(CAST("SppCode" AS TEXT),''), COALESCE("Use", 0) FROM "project".%s ORDER BY "LumpCode", "SppCode"`, quotePGIdent(projName+"_Lump")))
			if lErr == nil {
				for lRows.Next() {
					var lc, sc string
					var useInt int64
					if lRows.Scan(&lc, &sc, &useInt) == nil && lc != "" && sc != "" {
						lumpRows = append(lumpRows, StagingLumpRow{LumpCode: lc, SppCode: sc, Use: useInt != 0})
					}
				}
				lRows.Close()
			}
		}

		// Extract active *_SU or <Project>_SU rows if present
		suRows = []StagingSURow{}
		if activeSU != "" && activeSU != "None" {
			sRows, sErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS TEXT), "SiteUnit" FROM "su".%s WHERE typeof("PlotNumber") = 'text' ORDER BY CAST("PlotNumber" AS BLOB)`, quotePGIdent(activeSU+"_SU")))
			if sErr == nil {
				for sRows.Next() {
					var p string
					var su sql.NullString
					if sRows.Scan(&p, &su) == nil {
						var ptr *string
						if su.Valid {
							v := su.String
							ptr = &v
						}
						suRows = append(suRows, StagingSURow{PlotNumber: p, SiteUnit: ptr})
					}
				}
				sRows.Close()
			}
		} else {
			var hasProjSU bool
			_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM "project".sqlite_master WHERE type='table' AND name=?)`, projName+"_SU").Scan(&hasProjSU)
			if hasProjSU {
				sRows, sErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS TEXT), "SiteUnit" FROM "project".%s WHERE typeof("PlotNumber") = 'text' ORDER BY CAST("PlotNumber" AS BLOB)`, quotePGIdent(projName+"_SU")))
				if sErr == nil {
					for sRows.Next() {
						var p string
						var su sql.NullString
						if sRows.Scan(&p, &su) == nil {
							var ptr *string
							if su.Valid {
								v := su.String
								ptr = &v
							}
							suRows = append(suRows, StagingSURow{PlotNumber: p, SiteUnit: ptr})
						}
					}
					sRows.Close()
				}
			}
		}
		return struct{}{}, ctx.Err()
	})
	return
}

// runLayer1ComplianceFallback executes critical compliance checks directly against SQLiteContext
// when no external Layer-2 ComplianceValidatorFunc is injected (such as direct internal/cloudsync tests).
func runLayer1ComplianceFallback(ctx context.Context, s *vprocore.ContextService, contextID string) (ComplianceGateResult, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (ComplianceGateResult, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return ComplianceGateResult{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return ComplianceGateResult{}, err
		}
		defer func() { _ = tx.Rollback() }()

		proj := owner.Selection.Project
		var envAdminMismatch, orphans, boundsErrs, unlistedSpp int

		// 1. Env <-> Admin 1:1 completeness
		_ = tx.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT (
				SELECT COUNT(*) FROM "project".%s e
				WHERE typeof(e."PlotNumber")='text' AND NOT EXISTS (
					SELECT 1 FROM "project".%s a WHERE typeof(a."Plot")='text' AND CAST(a."Plot" AS BLOB)=CAST(e."PlotNumber" AS BLOB)
				)
			) + (
				SELECT COUNT(*) FROM "project".%s a
				WHERE typeof(a."Plot")='text' AND NOT EXISTS (
					SELECT 1 FROM "project".%s e WHERE typeof(e."PlotNumber")='text' AND CAST(e."PlotNumber" AS BLOB)=CAST(a."Plot" AS BLOB)
				)
			)
		`, quotePGIdent(proj+"_Env"), quotePGIdent(proj+"_Admin"), quotePGIdent(proj+"_Admin"), quotePGIdent(proj+"_Env"))).Scan(&envAdminMismatch)

		// 2. Orphan child rows
		for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other"} {
			var cnt int
			_ = tx.QueryRowContext(ctx, fmt.Sprintf(`
				SELECT COUNT(*) FROM "project".%s c
				WHERE typeof(c."PlotNumber")<>'text' OR NOT EXISTS (
					SELECT 1 FROM "project".%s e WHERE typeof(e."PlotNumber")='text' AND CAST(e."PlotNumber" AS BLOB)=CAST(c."PlotNumber" AS BLOB)
				)
			`, quotePGIdent(proj+"_"+suffix), quotePGIdent(proj+"_Env"))).Scan(&cnt)
			orphans += cnt
		}

		// 3. PlotNumber UTF-16 <= 7 & Veg Species UTF-16 <= 8 & float32 cover < 100
		pRows, pErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS TEXT) FROM "project".%s WHERE typeof("PlotNumber")='text'`, quotePGIdent(proj+"_Env")))
		if pErr == nil {
			for pRows.Next() {
				var p string
				if pRows.Scan(&p) == nil && ValidateUTF16Bound("PlotNumber", p, 7) != nil {
					boundsErrs++
				}
			}
			pRows.Close()
		}

		vRows, vErr := tx.QueryContext(ctx, fmt.Sprintf(`SELECT COALESCE(CAST("Species" AS TEXT),''), "Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover6", "Cover7" FROM "project".%s`, quotePGIdent(proj+"_Veg")))
		if vErr == nil {
			for vRows.Next() {
				var spp string
				var c1, c2, c3, ta, c4, c5, tb, c6, c7 sql.NullFloat64
				if vRows.Scan(&spp, &c1, &c2, &c3, &ta, &c4, &c5, &tb, &c6, &c7) == nil {
					if ValidateUTF16Bound("Species", spp, 8) != nil {
						boundsErrs++
					}
					for _, cv := range []sql.NullFloat64{c1, c2, c3, ta, c4, c5, tb, c6, c7} {
						if cv.Valid {
							if _, ok := NarrowSingleCover(cv.Float64); !ok {
								boundsErrs++
							}
						}
					}
				}
			}
			vRows.Close()
		}

		// 4. Unlisted species check against VLists.USysAllSpecs + VUser.USysUserSpp
		_ = tx.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM "project".%s v
			WHERE v."Species" IS NOT NULL AND v."Species" <> ''
			  AND NOT EXISTS (SELECT 1 FROM "VLists"."USysAllSpecs" s WHERE UPPER(s."Code") = UPPER(CAST(v."Species" AS TEXT)))
			  AND NOT EXISTS (SELECT 1 FROM "VUser"."USysUserSpp" u WHERE UPPER(u."Code") = UPPER(CAST(v."Species" AS TEXT)))
		`, quotePGIdent(proj+"_Veg"))).Scan(&unlistedSpp)

		blocking := envAdminMismatch + orphans + boundsErrs + unlistedSpp
		summary := map[string]int{
			"envAdminMismatches": envAdminMismatch,
			"orphanChildRows":    orphans,
			"boundsErrors":       boundsErrs,
			"unlistedSpecies":    unlistedSpp,
		}
		passed := blocking == 0
		repJSON, _ := json.Marshal(map[string]any{
			"project": proj,
			"passed":  passed,
			"summary": summary,
		})
		return ComplianceGateResult{
			Passed:        passed,
			BlockingCount: blocking,
			Summary:       summary,
			ReportJSON:    repJSON,
		}, nil
	})
}

func computeDBWithWALSHA256(path string) (string, error) {
	h := sha256.New()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	_, _ = h.Write(raw)
	if wal, wErr := os.ReadFile(path + "-wal"); wErr == nil && len(wal) > 0 {
		_, _ = h.Write([]byte("|WAL|"))
		_, _ = h.Write(wal)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PreviewStagingSubmission runs mandatory compliance validation and packages UserSpeciesManifest for UI review.
func (m *Manager) PreviewStagingSubmission(
	ctx context.Context,
	s *vprocore.ContextService,
	contextID string,
	useActiveSU bool,
	validator ComplianceValidatorFunc,
) (StagingPreflightPreview, error) {
	if s == nil {
		return StagingPreflightPreview{}, errors.New("project context service is unavailable")
	}
	if validator == nil {
		validator = func(ctx context.Context, s *vprocore.ContextService, cid string, _ bool) (ComplianceGateResult, error) {
			return runLayer1ComplianceFallback(ctx, s, cid)
		}
	}
	comp, err := validator(ctx, s, contextID, useActiveSU)
	if err != nil {
		return StagingPreflightPreview{}, err
	}
	projName, projPath, projID, _, _, _, _, plotCount, vegCount, userManifest, lumpRows, suRows, err := extractActiveProjectSubmissionArtifacts(ctx, s, contextID, useActiveSU)
	if err != nil {
		return StagingPreflightPreview{}, err
	}
	dbSHA, err := computeDBWithWALSHA256(projPath)
	if err != nil {
		return StagingPreflightPreview{}, err
	}
	return StagingPreflightPreview{
		Project:             projName,
		ProjectID:           projID,
		PlotCount:           plotCount,
		VegRowCount:         vegCount,
		SourceDBSHA256:      dbSHA,
		CompliancePassed:    comp.Passed,
		BlockingErrorCount:  comp.BlockingCount,
		ComplianceSummary:   comp.Summary,
		ComplianceReport:    comp.ReportJSON,
		UserSpeciesManifest: userManifest,
		LumpRowCount:        len(lumpRows),
		SURowCount:          len(suRows),
	}, nil
}

// SubmitProjectToStaging enforces compliance validation, packages personal species from VUser.USysUserSpp,
// and streams all 8 VP08 tables into staging.* inside a single atomic PostgreSQL transaction.
func (m *Manager) SubmitProjectToStaging(
	ctx context.Context,
	s *vprocore.ContextService,
	contextID string,
	req SubmitStagingRequest,
	validator ComplianceValidatorFunc,
) (SubmitStagingResult, error) {
	if s == nil {
		return SubmitStagingResult{}, errors.New("project context service is unavailable")
	}
	if validator == nil {
		validator = func(ctx context.Context, s *vprocore.ContextService, cid string, _ bool) (ComplianceGateResult, error) {
			return runLayer1ComplianceFallback(ctx, s, cid)
		}
	}
	comp, err := validator(ctx, s, contextID, req.UseActiveSU)
	if err != nil {
		return SubmitStagingResult{}, err
	}

	creds, err := m.ResolveCredentials()
	if err != nil {
		return SubmitStagingResult{}, err
	}
	activeRole := creds.ActiveRoleMode
	if activeRole == "" {
		activeRole = RoleModeContributor
	}

	if !comp.Passed {
		if !(req.CustodianOverride && activeRole == RoleModeCustodian) {
			return SubmitStagingResult{}, fmt.Errorf(
				"staging upload blocked by pre-flight compliance gate (%d blocking violation(s)); fix errors in Compliance Validator before submitting",
				comp.BlockingCount,
			)
		}
	}

	projName, projPath, projID, actor, _, _, suScopePlots, prePlotCount, preVegCount, userManifest, lumpRows, suRows, err := extractActiveProjectSubmissionArtifacts(ctx, s, contextID, req.UseActiveSU)
	if err != nil {
		return SubmitStagingResult{}, err
	}
	if strings.EqualFold(projName, "Sample") {
		return SubmitStagingResult{}, errors.New("built-in Sample project cannot be submitted to provincial BECMaster staging")
	}
	if subBy := strings.TrimSpace(req.SubmittedBy); subBy != "" {
		actor = subBy
	}

	// Capture an atomic WAL-merged standalone SQLite snapshot so source_db_sha256 and streamed rows are 100% snapshot-consistent!
	snapPath, cleanSnap, err := createConsistentSQLiteSnapshot(ctx, projPath)
	if err != nil {
		return SubmitStagingResult{}, fmt.Errorf("snapshot project SQLite database for staging upload: %w", err)
	}
	defer cleanSnap()

	dbSHA, err := streamFileSHA256Hex(snapPath)
	if err != nil {
		return SubmitStagingResult{}, err
	}

	sqliteDB, err := sqlutil.OpenReadOnlyContext(ctx, snapPath)
	if err != nil {
		return SubmitStagingResult{}, fmt.Errorf("open project SQLite snapshot for staging upload: %w", err)
	}
	defer sqliteDB.Close()

	conn, _, err := m.OpenDedicatedConn(ctx, activeRole)
	if err != nil {
		return SubmitStagingResult{}, err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return SubmitStagingResult{}, wrapRedactedError("begin staging upload transaction", err, creds)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentDBRole string
	if err := tx.QueryRow(ctx, `SELECT current_user`).Scan(&currentDBRole); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("read current PostgreSQL role", err, creds)
	}

	subID := uuid.NewString()
	nowUTC := time.Now().UTC()
	manifestJSON, _ := json.Marshal(userManifest)
	lumpJSON, _ := json.Marshal(lumpRows)
	suJSON, _ := json.Marshal(suRows)

	var notesPtr, baseSnapPtr, projIDPtr *string
	if n := strings.TrimSpace(req.SubmitterNotes); n != "" {
		notesPtr = &n
	}
	if b := strings.TrimSpace(req.BaseSnapshotTag); b != "" {
		baseSnapPtr = &b
	}
	if p := strings.TrimSpace(projID); p != "" {
		projIDPtr = &p
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO staging.submissions (
			submission_id, project_name, project_id, submitted_by, db_role, submitted_at,
			status, plot_count, veg_row_count, source_db_sha256, compliance_passed,
			compliance_report, user_species_manifest, lump_rows, su_rows,
			submitter_notes, base_snapshot_tag
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			'pending', $7, $8, $9, $10,
			$11::jsonb, $12::jsonb, $13::jsonb, $14::jsonb,
			$15, $16
		)
	`, subID, projName, projIDPtr, actor, currentDBRole, nowUTC,
		prePlotCount, preVegCount, dbSHA, comp.Passed,
		string(comp.ReportJSON), string(manifestJSON), string(lumpJSON), string(suJSON),
		notesPtr, baseSnapPtr,
	); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("insert staging.submissions header", err, creds)
	}

	plotSet := map[string]struct{}{}
	envCopied, err := streamKeyedParentToPG(ctx, tx, sqliteDB, projName+"_Env", "staging", "env", VP08EnvColumns, "PlotNumber", plotSet, suScopePlots, subID)
	if err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.env", err, creds)
	}
	if envCopied == 0 {
		return SubmitStagingResult{}, errors.New("project contains 0 valid text-keyed plots to submit")
	}

	seenAdmin := map[string]struct{}{}
	if _, err := streamKeyedParentToPG(ctx, tx, sqliteDB, projName+"_Admin", "staging", "admin", VP08AdminColumns, "Plot", seenAdmin, plotSet, subID); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.admin", err, creds)
	}

	vegCopied, err := streamChildTableToPG(ctx, tx, sqliteDB, projName+"_Veg", "staging", "veg", VP08VegColumns, plotSet, &subID)
	if err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.veg", err, creds)
	}
	if _, err := streamChildTableToPG(ctx, tx, sqliteDB, projName+"_Humus", "staging", "humus", VP08HumusColumns, plotSet, &subID); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.humus", err, creds)
	}
	if _, err := streamChildTableToPG(ctx, tx, sqliteDB, projName+"_Mineral", "staging", "mineral", VP08MineralColumns, plotSet, &subID); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.mineral", err, creds)
	}
	if _, err := streamChildTableToPG(ctx, tx, sqliteDB, projName+"_Other", "staging", "other", VP08OtherColumns, plotSet, &subID); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.other", err, creds)
	}

	seenMeta := map[string]struct{}{}
	if _, err := streamKeyedParentToPG(ctx, tx, sqliteDB, projName+"_Metadata", "staging", "metadata", VP08MetadataColumns, "ProjectID", seenMeta, nil, subID); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.metadata", err, creds)
	}

	if err := streamStagingAuditToPG(ctx, tx, sqliteDB, projName+"_Audit", subID, plotSet); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("stream staging.audit", err, creds)
	}

	// Update plot_count and veg_row_count if custodian/table owner; for contributors without UPDATE privilege, inspect before insert!
	// Wait: contributors do NOT have UPDATE on staging.submissions! Let's check: we ALREADY know plotCount & vegCount before INSERT!
	// Wait — let's make sure the initial INSERT into staging.submissions ALREADY sets exact plotCount and vegCount!
	_ = slices.Compact([]string{}) // keep slices used if needed

	if err := tx.Commit(ctx); err != nil {
		return SubmitStagingResult{}, wrapRedactedError("commit staging upload", err, creds)
	}

	return SubmitStagingResult{
		SubmissionID:     subID,
		ProjectName:      projName,
		ProjectID:        projID,
		SubmittedBy:      actor,
		DBRole:           currentDBRole,
		Status:           "pending",
		PlotCount:        int(envCopied),
		VegRowCount:      int(vegCopied),
		UserSpeciesCount: userManifest.RowCount,
		SourceDBSHA256:   dbSHA,
		CompliancePassed: comp.Passed,
		SubmittedAt:      nowUTC.Format(time.RFC3339),
	}, nil
}

func streamStagingAuditToPG(ctx context.Context, tx pgx.Tx, sqliteDB *sql.DB, sqliteAuditTable, subID string, allowedPlots map[string]struct{}) error {
	var hasAudit bool
	if err := sqliteDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, sqliteAuditTable).Scan(&hasAudit); err != nil || !hasAudit {
		return nil
	}
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY rowid", sqliteSelectColumnsCSV(VP08AuditColumns), quotePGIdent(sqliteAuditTable))
	rows, err := sqliteDB.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	copyCols := append([]string{"submission_id"}, columnNames(VP08AuditColumns)...)
	rawVals := make([]any, len(VP08AuditColumns))
	scanDest := make([]any, len(VP08AuditColumns))
	for i := range rawVals {
		scanDest[i] = &rawVals[i]
	}

	_, copyErr := tx.CopyFrom(
		ctx,
		pgx.Identifier{"staging", "audit"},
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
				out := make([]any, len(copyCols))
				out[0] = subID
				for i, col := range VP08AuditColumns {
					conv, cErr := CanonicalizeSQLiteToPG(col, rawVals[i])
					if cErr != nil {
						return nil, cErr
					}
					out[1+i] = conv
				}
				if pStr, ok := out[3].(string); ok && pStr != "" && allowedPlots != nil {
					if _, exists := allowedPlots[pStr]; !exists {
						continue
					}
				}
				return out, nil
			}
		}),
	)
	if copyErr != nil {
		return copyErr
	}
	return rows.Close()
}
