package rsidecar

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

var candidatePrefixPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,13}$`)

// ClusterIndicatorSpecies describes one diagnostic/indicator species for a cluster unit.
type ClusterIndicatorSpecies struct {
	Species        string  `json:"species"`
	ScientificName string  `json:"scientificName"`
	EnglishName    string  `json:"englishName"`
	Lifeform       int     `json:"lifeform"`
	ConstancyPct   float64 `json:"constancyPct"`
	MeanCover      float64 `json:"meanCover"`
	IndVal         float64 `json:"indVal"`
}

// ClusterUnitSummary summarizes one cluster produced by BEC_R vegclust or hierarchical clustering.
type ClusterUnitSummary struct {
	ClusterUnit    string                    `json:"clusterUnit"`
	IsNoise        bool                      `json:"isNoise"`
	PlotCount      int                       `json:"plotCount"`
	MeanRichness   float64                   `json:"meanRichness"`
	MeanSilhouette float64                   `json:"meanSilhouette"`
	Indicators     []ClusterIndicatorSpecies `json:"indicators"`
}

// ClusterPlotMembership gives the cluster assignment, fuzzy membership, and silhouette width for a plot.
type ClusterPlotMembership struct {
	PlotNumber      string  `json:"plotNumber"`
	ClusterUnit     string  `json:"clusterUnit"`
	Membership      float64 `json:"membership"`
	IsNoise         bool    `json:"isNoise"`
	Silhouette      float64 `json:"silhouette"`
	SpeciesRichness int     `json:"speciesRichness"`
}

// DendrogramSegment represents one orthogonal line segment in dendrogram coordinate space.
type DendrogramSegment struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

// DendrogramLeaf represents one ordered plot leaf along the dendrogram axis.
type DendrogramLeaf struct {
	PlotNumber  string  `json:"plotNumber"`
	X           float64 `json:"x"`
	Order       int     `json:"order"`
	ClusterUnit string  `json:"clusterUnit"`
	IsNoise     bool    `json:"isNoise"`
}

// DendrogramGeometry holds full SVG tree layout coordinates plus Newick string.
type DendrogramGeometry struct {
	MaxHeight float64             `json:"maxHeight"`
	CutHeight float64             `json:"cutHeight"`
	Newick    string              `json:"newick"`
	Segments  []DendrogramSegment `json:"segments"`
	Leaves    []DendrogramLeaf    `json:"leaves"`
}

// ClusteringRequest configures out-of-process BEC_R vegan / vegclust multivariate clustering.
type ClusteringRequest struct {
	Method               string   `json:"method"`          // "NC" (vegclust Noise Clustering, default) | "KM" | "FCM" | "HCLUST"
	Distance             string   `json:"distance"`        // "bray" (default) | "chord" | "jaccard"
	Linkage              string   `json:"linkage"`         // "ward.D2" (default) | "average" | "complete"
	Transform            string   `json:"transform"`       // "sqrt" (default) | "log1p" | "chord" | "hellinger" | "raw"
	K                    int      `json:"k"`               // Target cluster count (default 3)
	Fuzziness            float64  `json:"fuzziness"`       // vegclust m > 1.0 (default 1.25)
	DNoise               float64  `json:"dnoise"`          // vegclust noise distance (default 0.75)
	CutHeight            float64  `json:"cutHeight"`       // Optional hierarchical cut height
	ApplyLump            bool     `json:"applyLump"`       // Fold <Name>_Lump (Use != 0) species codes
	LumpName             string   `json:"lumpName"`        // Optional explicit Lump prefix (defaults to active project)
	UseActiveSU          bool     `json:"useActiveSu"`
	BGCFilter            string   `json:"bgcFilter"`
	PlotNumbers          []string `json:"plotNumbers,omitempty"`
	UnitPrefix           string   `json:"unitPrefix"`           // Cluster unit label prefix (default "CL")
	CandidatePrefix      string   `json:"candidatePrefix"`      // Prefix <Name> for <Name>_ClusterCandidate_SU
	MaterializeCandidate bool     `json:"materializeCandidate"` // Write <Name>_ClusterCandidate_SU inside <Project>.db
}

// ClusteringResult returns cluster memberships, indicator species, dendrogram coordinates, and candidate *_SU metadata.
type ClusteringResult struct {
	ContextID             string                  `json:"contextId"`
	Project               string                  `json:"project"`
	ActiveSU              string                  `json:"activeSu"`
	Method                string                  `json:"method"`
	Distance              string                  `json:"distance"`
	Linkage               string                  `json:"linkage"`
	Transform             string                  `json:"transform"`
	K                     int                     `json:"k"`
	PlotCount             int                     `json:"plotCount"`
	SpeciesCount          int                     `json:"speciesCount"`
	LumpAppliedRows       int                     `json:"lumpAppliedRows"`
	CopheneticCorrelation float64                 `json:"copheneticCorrelation"`
	MeanSilhouette        float64                 `json:"meanSilhouette"`
	NoiseCount            int                     `json:"noiseCount"`
	CandidateSUName       string                  `json:"candidateSuName,omitempty"`
	CandidateTableName    string                  `json:"candidateTableName,omitempty"`
	MaterializedRows      int                     `json:"materializedRows"`
	Clusters              []ClusterUnitSummary    `json:"clusters"`
	Memberships           []ClusterPlotMembership `json:"memberships"`
	Dendrogram            DendrogramGeometry      `json:"dendrogram"`
}

// PromoteCandidateSURequest promotes a reviewed <Name>_ClusterCandidate_SU table into a target *_SU table.
type PromoteCandidateSURequest struct {
	CandidateSUName  string `json:"candidateSuName"`  // e.g. "Sample_ClusterCandidate" (with or without "_SU" suffix)
	TargetSUName     string `json:"targetSuName"`     // Target SU prefix (without "_SU"), e.g. "BGCRev2026"
	ExcludeNoise     bool   `json:"excludeNoise"`     // Omit *_Noise assignments during promotion
	ConfirmOverwrite bool   `json:"confirmOverwrite"` // Required true if TargetSUName_SU already exists!
	Activate         bool   `json:"activate"`         // Switch active SU to TargetSUName after commit
}

// PromoteCandidateSUResult reports the outcome of promoting a cluster candidate table to a *_SU table.
type PromoteCandidateSUResult struct {
	ContextID          string                 `json:"contextId"`
	CandidateTableName string                 `json:"candidateTableName"`
	TargetSUName       string                 `json:"targetSuName"`
	TargetTableName    string                 `json:"targetTableName"`
	OverwroteExisting  bool                   `json:"overwroteExisting"`
	PromotedRows       int                    `json:"promotedRows"`
	SkippedNoiseRows   int                    `json:"skippedNoiseRows"`
	Activated          bool                   `json:"activated"`
	NewState           *vprocore.ProjectState `json:"newState,omitempty"`
}

func resolveCandidateSUName(rawPrefix, projectName string) (string, error) {
	base := strings.TrimSpace(rawPrefix)
	base = strings.TrimSuffix(base, "_SU")
	base = strings.TrimSuffix(base, "_ClusterCandidate")
	if base == "" {
		base = projectName
	}
	// Replace any "master" substring so candidate SU name never collides with master table guards
	lower := strings.ToLower(base)
	if strings.Contains(lower, "master") {
		base = "Proj"
	}
	if len(base) > 14 {
		base = base[:14]
	}
	if !utf8.ValidString(base) || !candidatePrefixPattern.MatchString(base) {
		return "", fmt.Errorf("candidate prefix %q must start with an ASCII letter and contain only letters, digits, or underscores (max 14 chars)", rawPrefix)
	}
	suName := base + "_ClusterCandidate"
	if !vprocore.ProjectNamePattern.MatchString(suName) {
		return "", fmt.Errorf("resolved candidate SU name %q is invalid", suName)
	}
	return suName, nil
}

func validateTargetPromotionSUName(raw string) (string, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(raw), "_SU")
	if !utf8.ValidString(trimmed) || !vprocore.ProjectNamePattern.MatchString(trimmed) {
		return "", fmt.Errorf("target SU name %q must start with a letter and contain only ASCII letters, digits, or underscores (max 31 chars)", raw)
	}
	lower := strings.ToLower(trimmed)
	if lower == "none" || lower == "usyssutabledynamic" || strings.Contains(lower, "master") {
		return "", fmt.Errorf("target SU name %q is reserved or protected and cannot be overwritten", trimmed)
	}
	if strings.HasSuffix(lower, "_clustercandidate") {
		return "", fmt.Errorf("target SU name %q cannot be another _ClusterCandidate staging table", trimmed)
	}
	return trimmed, nil
}

type rClusteringScriptResponse struct {
	OK                    bool                    `json:"ok"`
	Method                string                  `json:"method"`
	Distance              string                  `json:"distance"`
	Linkage               string                  `json:"linkage"`
	Transform             string                  `json:"transform"`
	K                     int                     `json:"k"`
	PlotCount             int                     `json:"plotCount"`
	SpeciesCount          int                     `json:"speciesCount"`
	LumpAppliedRows       int                     `json:"lumpAppliedRows"`
	CopheneticCorrelation float64                 `json:"copheneticCorrelation"`
	MeanSilhouette        float64                 `json:"meanSilhouette"`
	NoiseCount            int                     `json:"noiseCount"`
	Clusters              []ClusterUnitSummary    `json:"clusters"`
	Memberships           []ClusterPlotMembership `json:"memberships"`
	Dendrogram            DendrogramGeometry      `json:"dendrogram"`
}

// RunMultivariateClustering executes real R vegan/vegclust clustering over <Project>_Veg,
// returns indicator species and SVG dendrogram coordinates, and optionally materializes <Name>_ClusterCandidate_SU
// inside <Project>.db without ever modifying the active *_SU table.
func RunMultivariateClustering(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req ClusteringRequest,
	opts ...RuntimeOptions,
) (*ClusteringResult, error) {
	if ctx == nil {
		return nil, errors.New("RunMultivariateClustering requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("context service is unavailable")
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "NC"
	}
	switch method {
	case "NC", "KM", "FCM", "HCLUST":
	default:
		return nil, fmt.Errorf("unsupported clustering method %q (expected NC, KM, FCM, or HCLUST)", req.Method)
	}

	distance := strings.ToLower(strings.TrimSpace(req.Distance))
	if distance == "" {
		distance = "bray"
	}
	switch distance {
	case "bray", "chord", "jaccard", "kulczynski", "euclidean", "manhattan":
	default:
		return nil, fmt.Errorf("unsupported ecological distance %q", req.Distance)
	}

	linkage := strings.TrimSpace(req.Linkage)
	if linkage == "" {
		linkage = "ward.D2"
	}
	transform := strings.ToLower(strings.TrimSpace(req.Transform))
	if transform == "" {
		transform = "sqrt"
	}

	k := req.K
	if k <= 0 {
		k = 3
	}
	if k < 2 || k > 50 {
		return nil, fmt.Errorf("cluster count k (%d) must be between 2 and 50", k)
	}
	fuzziness := req.Fuzziness
	if fuzziness <= 0 {
		fuzziness = 1.25
	}
	if fuzziness <= 1.0 || fuzziness > 5.0 {
		return nil, fmt.Errorf("vegclust fuzziness exponent m (%.2f) must be in (1.0, 5.0]", fuzziness)
	}
	dnoise := req.DNoise
	if dnoise <= 0 {
		dnoise = 0.75
	}
	if dnoise < 0.05 || dnoise > 5.0 {
		return nil, fmt.Errorf("vegclust noise distance dnoise (%.2f) must be in [0.05, 5.0]", dnoise)
	}

	unitPrefix := strings.TrimSpace(req.UnitPrefix)
	if unitPrefix == "" {
		unitPrefix = "CL"
	}
	if !utf8.ValidString(unitPrefix) || len(utf16.Encode([]rune(unitPrefix))) > 24 || strings.ContainsAny(unitPrefix, "\x00\r\n") {
		return nil, errors.New("unitPrefix must be valid UTF-8 up to 24 UTF-16 code units")
	}

	cfg := effectiveOptions(opts...)

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ClusteringResult, error) {
		owner := plots.Projects.Sqlite
		var candidateSUName string
		if req.MaterializeCandidate {
			var cErr error
			candidateSUName, cErr = resolveCandidateSUName(req.CandidatePrefix, owner.Selection.Project)
			if cErr != nil {
				return nil, cErr
			}
		}

		// Resolve scoped plot numbers if UseActiveSU, BGCFilter, or explicit PlotNumbers were requested
		var scopedPlots []string
		if req.UseActiveSU || strings.TrimSpace(req.BGCFilter) != "" || len(req.PlotNumbers) > 0 {
			envRows, err := readScopedEnvPlots(ctx, owner, req.UseActiveSU, req.BGCFilter, "", req.PlotNumbers)
			if err != nil {
				return nil, err
			}
			scopedPlots = make([]string, 0, len(envRows))
			for _, er := range envRows {
				scopedPlots = append(scopedPlots, er.PlotNumber)
			}
			if len(scopedPlots) < 3 {
				return nil, fmt.Errorf("clustering requires at least 3 plots in scope (matched %d)", len(scopedPlots))
			}
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		vlistsPath := owner.Attachments["VLists"]
		projectPath := owner.Selection.ProjectPath
		projectName := owner.Selection.Project
		activeSU := owner.Selection.SU
		existingProjDB := owner.ProjectDB
		owner.Mu.Unlock()

		if existingProjDB != nil {
			if err := CheckpointProjectWAL(ctx, existingProjDB); err != nil {
				return nil, err
			}
		}

		lumpTable := ""
		if req.ApplyLump {
			lp := strings.TrimSuffix(strings.TrimSpace(req.LumpName), "_Lump")
			if lp == "" {
				lp = projectName
			}
			lumpTable = lp + "_Lump"
		}

		rPayload := map[string]any{
			"projectPath": projectPath,
			"projectName": projectName,
			"vlistsPath":  vlistsPath,
			"method":      method,
			"distance":    distance,
			"linkage":     linkage,
			"transform":   transform,
			"k":           k,
			"fuzziness":   fuzziness,
			"dnoise":      dnoise,
			"cutHeight":   req.CutHeight,
			"applyLump":   req.ApplyLump,
			"lumpTable":   lumpTable,
			"unitPrefix":  unitPrefix,
		}
		if len(scopedPlots) > 0 {
			rPayload["plotNumbers"] = scopedPlots
		}

		var rResp rClusteringScriptResponse
		err := executeRScriptJSON(
			ctx,
			cfg,
			"",
			"run_clustering.R",
			nil,
			rPayload,
			&rResp,
		)
		if err != nil {
			return nil, err
		}

		materializedRows := 0
		candidateTableName := ""
		if req.MaterializeCandidate {
			if strings.EqualFold(candidateSUName, activeSU) && vprocore.SameDesktopPath(projectPath, owner.Selection.SUPath) {
				return nil, fmt.Errorf("candidate SU %q is currently active; switch to another SU before overwriting", candidateSUName)
			}
			projDB, err := owner.ProjectDatabase(ctx)
			if err != nil {
				return nil, err
			}
			candidateTableName = candidateSUName + "_SU"
			user := strings.TrimSpace(plots.CurrentUser)
			if user == "" {
				user = "VProUser"
			}
			if err := materializeClusterCandidateAtomic(ctx, projDB, projectName, user, candidateSUName, rResp.Memberships, rPayload); err != nil {
				return nil, err
			}
			materializedRows = len(rResp.Memberships)
		}

		return &ClusteringResult{
			ContextID:             contextID,
			Project:               projectName,
			ActiveSU:              activeSU,
			Method:                rResp.Method,
			Distance:              rResp.Distance,
			Linkage:               rResp.Linkage,
			Transform:             rResp.Transform,
			K:                     rResp.K,
			PlotCount:             rResp.PlotCount,
			SpeciesCount:          rResp.SpeciesCount,
			LumpAppliedRows:       rResp.LumpAppliedRows,
			CopheneticCorrelation: rResp.CopheneticCorrelation,
			MeanSilhouette:        rResp.MeanSilhouette,
			NoiseCount:            rResp.NoiseCount,
			CandidateSUName:       candidateSUName,
			CandidateTableName:    candidateTableName,
			MaterializedRows:      materializedRows,
			Clusters:              rResp.Clusters,
			Memberships:           rResp.Memberships,
			Dendrogram:            rResp.Dendrogram,
		}, nil
	})
}

func materializeClusterCandidateAtomic(
	ctx context.Context,
	db *sql.DB,
	projectName string,
	userName string,
	candidateSUName string,
	memberships []ClusterPlotMembership,
	params map[string]any,
) error {
	// Safety invariant: candidate table MUST end with _ClusterCandidate_SU so active *_SU is never touched
	if !strings.HasSuffix(candidateSUName, "_ClusterCandidate") {
		return fmt.Errorf("refusing to materialize non-candidate table %q", candidateSUName)
	}
	tableName := candidateSUName + "_SU"
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	qUniq := vprocore.QuoteHeaderIdentifier("uidx_" + tableName + "_PlotNumber")
	qIdx := vprocore.QuoteHeaderIdentifier("idx_" + tableName + "_SiteUnit")

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin immediate transaction for %s: %w", tableName, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	// Replace only the staging *_ClusterCandidate_SU table
	if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS main.`+qTable); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE main.`+qTable+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR)`); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `CREATE UNIQUE INDEX main.`+qUniq+` ON `+qTable+` ("PlotNumber")`); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `CREATE INDEX main.`+qIdx+` ON `+qTable+` ("SiteUnit")`); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main._table_metadata (table_name TEXT PRIMARY KEY, description TEXT)`); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `INSERT OR REPLACE INTO main._table_metadata (table_name, description) VALUES (?, 'VP04')`, tableName); err != nil {
		return err
	}

	for _, m := range memberships {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, m.PlotNumber, m.ClusterUnit); err != nil {
			return err
		}
	}

	if err := insertProjectAuditEntryOnConn(ctx, conn, projectName, userName, "", tableName, "MaterializeClusterCandidate", "", fmt.Sprintf("%d plots", len(memberships))); err != nil {
		return err
	}
	if err := appendRSidecarHistoryOnConn(ctx, conn, "MaterializeClusterCandidate", map[string]any{
		"tableName": tableName,
		"rows":      len(memberships),
		"params":    params,
	}); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

// PromoteClusterCandidateToSU promotes a reviewed <Name>_ClusterCandidate_SU table into a target *_SU table
// inside <Project>.db, strictly enforcing ConfirmOverwrite when the target *_SU table already exists.
func PromoteClusterCandidateToSU(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req PromoteCandidateSURequest,
) (*PromoteCandidateSUResult, error) {
	if ctx == nil {
		return nil, errors.New("PromoteClusterCandidateToSU requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("context service is unavailable")
	}

	candTrim := strings.TrimSuffix(strings.TrimSpace(req.CandidateSUName), "_SU")
	if candTrim == "" || !strings.HasSuffix(candTrim, "_ClusterCandidate") || !vprocore.ProjectNamePattern.MatchString(candTrim) {
		return nil, fmt.Errorf("candidateSuName %q must identify a valid *_ClusterCandidate table", req.CandidateSUName)
	}
	targetSUName, err := validateTargetPromotionSUName(req.TargetSUName)
	if err != nil {
		return nil, err
	}

	var result *PromoteCandidateSUResult
	var currentSel vprocore.DesktopSelection

	_, err = vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		currentSel = owner.Selection
		projDB, err := owner.ProjectDatabase(ctx)
		if err != nil {
			return struct{}{}, err
		}

		user := strings.TrimSpace(plots.CurrentUser)
		if user == "" {
			user = "VProUser"
		}

		candTable := candTrim + "_SU"
		targetTable := targetSUName + "_SU"

		conn, err := projDB.Conn(ctx)
		if err != nil {
			return struct{}{}, err
		}
		defer conn.Close()

		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return struct{}{}, fmt.Errorf("begin immediate transaction for promotion to %s: %w", targetTable, err)
		}
		committed := false
		defer func() {
			if !committed {
				_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			}
		}()

		var candExists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ?`, candTable).Scan(&candExists); err != nil {
			return struct{}{}, err
		}
		if candExists == 0 {
			return struct{}{}, fmt.Errorf("candidate table %s does not exist in %s.db", candTable, owner.Selection.Project)
		}

		var targetExists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, targetTable).Scan(&targetExists); err != nil {
			return struct{}{}, err
		}
		overwrote := targetExists > 0
		if overwrote && !req.ConfirmOverwrite {
			return struct{}{}, fmt.Errorf("target SU table %s already exists; set confirmOverwrite=true to confirm replacing it", targetTable)
		}

		// Read rows from candidate table preserving literal PlotNumber text
		qCand := vprocore.QuoteHeaderIdentifier(candTable)
		rows, err := conn.QueryContext(ctx, `SELECT CAST("PlotNumber" AS TEXT), COALESCE(CAST("SiteUnit" AS TEXT), '') FROM main.`+qCand+` WHERE typeof("PlotNumber")='text' ORDER BY "PlotNumber" ASC`)
		if err != nil {
			return struct{}{}, err
		}
		type suPair struct {
			plot string
			unit string
		}
		var pairs []suPair
		skippedNoise := 0
		for rows.Next() {
			var p, u string
			if err := rows.Scan(&p, &u); err != nil {
				rows.Close()
				return struct{}{}, err
			}
			if req.ExcludeNoise && strings.HasSuffix(u, "_Noise") {
				skippedNoise++
				continue
			}
			pairs = append(pairs, suPair{plot: p, unit: u})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return struct{}{}, err
		}
		rows.Close()

		if len(pairs) == 0 {
			return struct{}{}, errors.New("no non-noise cluster assignments remain to promote")
		}

		qTarget := vprocore.QuoteHeaderIdentifier(targetTable)
		qUniq := vprocore.QuoteHeaderIdentifier("uidx_" + targetTable + "_PlotNumber")
		qIdx := vprocore.QuoteHeaderIdentifier("idx_" + targetTable + "_SiteUnit")

		if overwrote {
			if _, err := conn.ExecContext(ctx, `DELETE FROM main.`+qTarget); err != nil {
				return struct{}{}, err
			}
		} else {
			if _, err := conn.ExecContext(ctx, `CREATE TABLE main.`+qTarget+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR)`); err != nil {
				return struct{}{}, err
			}
			if _, err := conn.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS main.`+qUniq+` ON `+qTarget+` ("PlotNumber")`); err != nil {
				return struct{}{}, err
			}
			if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS main.`+qIdx+` ON `+qTarget+` ("SiteUnit")`); err != nil {
				return struct{}{}, err
			}
			if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main._table_metadata (table_name TEXT PRIMARY KEY, description TEXT)`); err != nil {
				return struct{}{}, err
			}
			if _, err := conn.ExecContext(ctx, `INSERT OR REPLACE INTO main._table_metadata (table_name, description) VALUES (?, 'VP04')`, targetTable); err != nil {
				return struct{}{}, err
			}
		}

		for _, pr := range pairs {
			if err := ctx.Err(); err != nil {
				return struct{}{}, err
			}
			if _, err := conn.ExecContext(ctx, `INSERT INTO main.`+qTarget+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, pr.plot, pr.unit); err != nil {
				return struct{}{}, err
			}
		}

		beforeDesc := "created"
		if overwrote {
			beforeDesc = "overwritten (confirmed)"
		}
		if err := insertProjectAuditEntryOnConn(ctx, conn, owner.Selection.Project, user, "", targetTable, "PromoteClusterCandidate", beforeDesc, fmt.Sprintf("%s -> %d plots", candTable, len(pairs))); err != nil {
			return struct{}{}, err
		}
		if err := appendRSidecarHistoryOnConn(ctx, conn, "PromoteClusterCandidate", map[string]any{
			"candidateTable": candTable,
			"targetTable":    targetTable,
			"overwrote":      overwrote,
			"promotedRows":   len(pairs),
			"skippedNoise":   skippedNoise,
		}); err != nil {
			return struct{}{}, err
		}

		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return struct{}{}, err
		}
		committed = true

		result = &PromoteCandidateSUResult{
			ContextID:          contextID,
			CandidateTableName: candTable,
			TargetSUName:       targetSUName,
			TargetTableName:    targetTable,
			OverwroteExisting:  overwrote,
			PromotedRows:       len(pairs),
			SkippedNoiseRows:   skippedNoise,
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	if req.Activate && result != nil {
		nextSel := vprocore.ContextSelection{
			Project:       currentSel.Project,
			ProjectPath:   currentSel.ProjectPath,
			SU:            targetSUName,
			SUPath:        currentSel.ProjectPath,
			Hierarchy:     currentSel.Hierarchy,
			HierarchyPath: currentSel.HierarchyPath,
		}
		newState, swErr := contexts.SwitchContext(contextID, nextSel)
		if swErr != nil {
			return nil, fmt.Errorf("promoted %s successfully, failed to activate context: %w", result.TargetTableName, swErr)
		}
		result.Activated = true
		result.ContextID = newState.ContextID
		result.NewState = &newState
	}

	return result, nil
}

func insertProjectAuditEntryOnConn(
	ctx context.Context,
	conn *sql.Conn,
	projectName, userName, plotNumber, tableName, editField, beforeVal, afterVal string,
) error {
	auditTable := projectName + "_Audit"
	var exists int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, auditTable).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	qAudit := vprocore.QuoteHeaderIdentifier(auditTable)
	var plotParam any
	if strings.TrimSpace(plotNumber) != "" {
		plotParam = plotNumber
	}

	// Restore = 0 (Access FALSE); PlotNumber = NULL for project-level SU actions so _Env FK passes cleanly
	_, err := conn.ExecContext(
		ctx,
		`INSERT INTO main.`+qAudit+` ("Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0)`,
		projectName,
		userName,
		plotParam,
		tableName,
		editField,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
		beforeVal,
		afterVal,
	)
	return err
}

func appendRSidecarHistoryOnConn(ctx context.Context, conn *sql.Conn, action string, details any) error {
	if _, err := conn.ExecContext(ctx, rsidecarHistoryDDL); err != nil {
		return err
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(
		ctx,
		`INSERT INTO main."__VPRO_RSidecarHistory" ("CreatedAt", "Action", "DetailsJSON") VALUES (?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339),
		action,
		string(payload),
	)
	return err
}
