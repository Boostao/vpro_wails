package spatial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	spatialAOISUHistoryTable = "__VPRO_SpatialAOISUHistory"
	spatialAOISUHistorySQL   = `CREATE TABLE "__VPRO_SpatialAOISUHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
)

// SpatialAOISelectionRequest specifies a lasso/polygon AOI in EPSG:4326 [lon, lat] coordinates.
type SpatialAOISelectionRequest struct {
	Polygon [][2]float64   `json:"polygon"` // Single exterior ring of [lon, lat] vertices
	Rings   [][][2]float64 `json:"rings"`   // Optional multi-ring / polygon-with-holes [lon, lat]
	Zone    string         `json:"zone"`    // Optional BEC Zone filter
}

func (r *SpatialAOISelectionRequest) UnmarshalJSON(data []byte) error {
	type plain SpatialAOISelectionRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// SpatialAOIMatchedPlot describes a plot inside the selected AOI polygon.
type SpatialAOIMatchedPlot struct {
	PlotNumber string  `json:"plotNumber"`
	Lon        float64 `json:"lon"`
	Lat        float64 `json:"lat"`
	FieldBGC   string  `json:"fieldBgc"`
	GISBGC     string  `json:"gisBgc"`
	SiteUnit   string  `json:"siteUnit"`
}

// SpatialAOISelectionResult returns matched plots and AOI geometry summary.
type SpatialAOISelectionResult struct {
	ContextID      string                  `json:"contextId"`
	Project        string                  `json:"project"`
	SU             string                  `json:"su"`
	TotalEvaluated int                     `json:"totalEvaluated"`
	MatchedCount   int                     `json:"matchedCount"`
	PlotNumbers    []string                `json:"plotNumbers"`
	Plots          []SpatialAOIMatchedPlot `json:"plots"`
	BoundingBox    [4]float64              `json:"boundingBox"`
	ApproxAreaKm2  float64                 `json:"approxAreaKm2"`
}

// AssignSpatialAOIToSURequest configures transactional assignment of AOI-selected plots into a *_SU table.
type AssignSpatialAOIToSURequest struct {
	Mode                           string         `json:"mode"` // "update_active" | "create_project_su" | "create_su_file"
	SUName                         string         `json:"suName"`
	FilePath                       string         `json:"filePath"`
	SiteUnit                       string         `json:"siteUnit"`
	PlotNumbers                    []string       `json:"plotNumbers"`
	Polygon                        [][2]float64   `json:"polygon"`
	Rings                          [][][2]float64 `json:"rings"`
	IncludeNonAOIPlotsAsUnassigned bool           `json:"includeNonAoiPlotsAsUnassigned"`
	Activate                       bool           `json:"activate"`
}

func (r *AssignSpatialAOIToSURequest) UnmarshalJSON(data []byte) error {
	type plain AssignSpatialAOIToSURequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// AssignSpatialAOIToSUResult reports the committed *_SU table write outcome.
type AssignSpatialAOIToSUResult struct {
	ContextID    string                 `json:"contextId"`
	SUName       string                 `json:"suName"`
	TableName    string                 `json:"tableName"`
	FilePath     string                 `json:"filePath"`
	SiteUnit     string                 `json:"siteUnit"`
	MatchedPlots int                    `json:"matchedPlots"`
	ChangedRows  int                    `json:"changedRows"`
	Activated    bool                   `json:"activated"`
	NewState     *vprocore.ProjectState `json:"newState,omitempty"`
}

func normalizeAOIRings(single [][2]float64, rings [][][2]float64) ([][][2]float64, [4]float64, error) {
	effective := rings
	if len(effective) == 0 && len(single) > 0 {
		effective = [][][2]float64{single}
	}
	if len(effective) == 0 {
		return nil, [4]float64{}, errors.New("AOI polygon requires at least 3 [lon, lat] vertices")
	}

	bbox := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	cleaned := make([][][2]float64, 0, len(effective))
	for _, ring := range effective {
		pts := make([][2]float64, 0, len(ring))
		for _, pt := range ring {
			if math.IsNaN(pt[0]) || math.IsNaN(pt[1]) || math.IsInf(pt[0], 0) || math.IsInf(pt[1], 0) {
				return nil, [4]float64{}, errors.New("AOI polygon coordinates must be finite")
			}
			if pt[0] < -180.0 || pt[0] > 180.0 || pt[1] < -90.0 || pt[1] > 90.0 {
				return nil, [4]float64{}, fmt.Errorf("AOI vertex [%.5f, %.5f] is outside EPSG:4326 bounds", pt[0], pt[1])
			}
			if pt[0] < bbox[0] {
				bbox[0] = pt[0]
			}
			if pt[1] < bbox[1] {
				bbox[1] = pt[1]
			}
			if pt[0] > bbox[2] {
				bbox[2] = pt[0]
			}
			if pt[1] > bbox[3] {
				bbox[3] = pt[1]
			}
			pts = append(pts, pt)
		}
		if len(pts) >= 2 && pts[len(pts)-1] == pts[0] {
			pts = pts[:len(pts)-1]
		}
		if len(pts) < 3 {
			return nil, [4]float64{}, errors.New("each AOI polygon ring must contain at least 3 distinct vertices")
		}
		cleaned = append(cleaned, pts)
	}
	return cleaned, bbox, nil
}

// PointInPolygonRingsLonLat tests whether (lon, lat) lies inside an EPSG:4326 polygon/lasso using even-odd ray-casting.
func PointInPolygonRingsLonLat(lon, lat float64, rings [][][2]float64) bool {
	inside := false
	for _, ring := range rings {
		n := len(ring)
		if n < 3 {
			continue
		}
		j := n - 1
		for i := 0; i < n; i++ {
			xi, yi := ring[i][0], ring[i][1]
			xj, yj := ring[j][0], ring[j][1]
			if ((yi > lat) != (yj > lat)) && (lon < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
				inside = !inside
			}
			j = i
		}
	}
	return inside
}

func approximatePolygonAreaKm2(ring [][2]float64) float64 {
	if len(ring) < 3 {
		return 0
	}
	var meanLat float64
	for _, pt := range ring {
		meanLat += pt[1]
	}
	meanLat /= float64(len(ring))
	cosLat := math.Cos(meanLat * math.Pi / 180.0)
	kmPerDegLat := 111.132
	kmPerDegLon := 111.320 * cosLat

	var area2 float64
	j := len(ring) - 1
	for i := 0; i < len(ring); i++ {
		xi, yi := ring[i][0]*kmPerDegLon, ring[i][1]*kmPerDegLat
		xj, yj := ring[j][0]*kmPerDegLon, ring[j][1]*kmPerDegLat
		area2 += xj*yi - xi*yj
		j = i
	}
	return math.Abs(area2) * 0.5
}

// SelectPlotsInPolygonAOI filters all coordinatized plots in the active context against an EPSG:4326 polygon/lasso.
func SelectPlotsInPolygonAOI(ctx context.Context, contexts *vprocore.ContextService, contextID string, req SpatialAOISelectionRequest, attributors ...*PMTilesAttributor) (*SpatialAOISelectionResult, error) {
	if ctx == nil {
		return nil, errors.New("AOI selection requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("spatial context service is unavailable")
	}
	rings, bbox, err := normalizeAOIRings(req.Polygon, req.Rings)
	if err != nil {
		return nil, err
	}
	zoneFilter := strings.TrimSpace(req.Zone)
	attr := getDefaultPMTilesAttributor()
	if len(attributors) > 0 && attributors[0] != nil {
		attr = attributors[0]
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*SpatialAOISelectionResult, error) {
		owner := plots.Projects.Sqlite
		records, _, err := extractSpatialPlotRecords(ctx, owner, attr)
		if err != nil {
			return nil, err
		}
		res := &SpatialAOISelectionResult{
			ContextID:      contextID,
			Project:        owner.Selection.Project,
			SU:             owner.Selection.SU,
			TotalEvaluated: len(records),
			PlotNumbers:    []string{},
			Plots:          []SpatialAOIMatchedPlot{},
			BoundingBox:    bbox,
			ApproxAreaKm2:  approximatePolygonAreaKm2(rings[0]),
		}
		for _, rec := range records {
			if rec.Lon < bbox[0] || rec.Lon > bbox[2] || rec.Lat < bbox[1] || rec.Lat > bbox[3] {
				continue
			}
			if zoneFilter != "" && !strings.EqualFold(rec.Zone, zoneFilter) && !strings.HasPrefix(strings.ToUpper(rec.GISBGC), strings.ToUpper(zoneFilter)) {
				continue
			}
			if PointInPolygonRingsLonLat(rec.Lon, rec.Lat, rings) {
				res.PlotNumbers = append(res.PlotNumbers, rec.PlotNumber)
				res.Plots = append(res.Plots, SpatialAOIMatchedPlot{
					PlotNumber: rec.PlotNumber,
					Lon:        rec.Lon,
					Lat:        rec.Lat,
					FieldBGC:   rec.FieldBGC,
					GISBGC:     rec.GISBGC,
					SiteUnit:   rec.SiteUnit,
				})
			}
		}
		res.MatchedCount = len(res.PlotNumbers)
		return res, nil
	})
}

func validateSpatialSUName(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !utf8.ValidString(trimmed) || !vprocore.ProjectNamePattern.MatchString(trimmed) {
		return "", fmt.Errorf("SU name %q is invalid; start with a letter and use ASCII letters, digits, or underscores (max 31 chars)", raw)
	}
	lower := strings.ToLower(trimmed)
	if lower == "none" || lower == "sample" || lower == "usyssutabledynamic" || strings.Contains(lower, "master") {
		return "", fmt.Errorf("SU name %q is reserved and cannot be created", trimmed)
	}
	return trimmed, nil
}

func createSpatialSUTableInTx(ctx context.Context, tx *sql.Tx, suName string) error {
	tableName := suName + "_SU"
	var collision int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE name = ? COLLATE NOCASE`, tableName).Scan(&collision); err != nil {
		return err
	}
	if collision != 0 {
		return fmt.Errorf("table %s already exists in target database", tableName)
	}
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	qUniq := vprocore.QuoteHeaderIdentifier("uidx_" + tableName + "_PlotNumber")
	qIdx := vprocore.QuoteHeaderIdentifier("idx_" + tableName + "_SiteUnit")

	if _, err := tx.ExecContext(ctx, `CREATE TABLE main.`+qTable+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX main.`+qUniq+` ON `+qTable+` ("PlotNumber")`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX main.`+qIdx+` ON `+qTable+` ("SiteUnit")`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main._table_metadata (table_name TEXT PRIMARY KEY, description TEXT)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO main._table_metadata (table_name, description) VALUES (?, 'VP04')`, tableName); err != nil {
		return err
	}
	return nil
}

func upsertActiveSURowsInTx(ctx context.Context, tx *sql.Tx, tableName, siteUnit string, plots []string) (int, error) {
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	changed := 0
	for _, plot := range plots {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		var existingCnt int
		var prevUnit sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*), MAX(CAST("SiteUnit" AS TEXT)) FROM main.`+qTable+
				` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
			plot).Scan(&existingCnt, &prevUnit)
		if err != nil {
			return 0, err
		}
		if existingCnt > 0 {
			if prevUnit.Valid && prevUnit.String == siteUnit {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE main.`+qTable+` SET "SiteUnit" = ? WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
				siteUnit, plot); err != nil {
				return 0, err
			}
			changed++
		} else {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`,
				plot, siteUnit); err != nil {
				return 0, err
			}
			changed++
		}
	}
	return changed, nil
}

// AssignSpatialAOIToSU assigns AOI-selected plots to a SiteUnit in the active *_SU table, a new project *_SU table, or a new .db file.
func AssignSpatialAOIToSU(ctx context.Context, contexts *vprocore.ContextService, contextID string, req AssignSpatialAOIToSURequest, attributors ...*PMTilesAttributor) (*AssignSpatialAOIToSUResult, error) {
	if ctx == nil {
		return nil, errors.New("AOI SU assignment requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("spatial context service is unavailable")
	}

	siteUnit := strings.TrimSpace(req.SiteUnit)
	if siteUnit == "" || !utf8.ValidString(siteUnit) || strings.ContainsRune(siteUnit, 0) {
		return nil, errors.New("siteUnit must be a non-empty UTF-8 string")
	}
	if len(utf16.Encode([]rune(siteUnit))) > 255 {
		return nil, errors.New("siteUnit exceeds 255 UTF-16 code units")
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		if strings.TrimSpace(req.FilePath) != "" {
			mode = "create_su_file"
		} else if strings.TrimSpace(req.SUName) != "" {
			mode = "create_project_su"
		} else {
			mode = "update_active"
		}
	}
	if mode != "update_active" && mode != "create_project_su" && mode != "create_su_file" {
		return nil, fmt.Errorf("unsupported AOI SU assignment mode %q", req.Mode)
	}

	attr := getDefaultPMTilesAttributor()
	if len(attributors) > 0 && attributors[0] != nil {
		attr = attributors[0]
	}

	var out *AssignSpatialAOIToSUResult
	var currentSelection vprocore.DesktopSelection

	_, err := vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		currentSelection = owner.Selection

		records, _, err := extractSpatialPlotRecords(ctx, owner, attr)
		if err != nil {
			return struct{}{}, err
		}

		// Determine matched plots either from explicit PlotNumbers or by ray-casting Polygon/Rings
		plotOrder := make([]string, 0)
		seen := make(map[string]bool)
		if len(req.PlotNumbers) > 0 {
			validPlots := make(map[string]bool, len(records))
			rawRows, qErr := queryScopedSpatialPlotRows(ctx, owner)
			if qErr != nil {
				return struct{}{}, qErr
			}
			for _, r := range rawRows {
				validPlots[r.plotNumber] = true
			}
			for _, p := range req.PlotNumbers {
				if p == "" || !utf8.ValidString(p) || strings.ContainsRune(p, 0) || len(utf16.Encode([]rune(p))) > 7 {
					return struct{}{}, fmt.Errorf("invalid plot number %q", p)
				}
				if !validPlots[p] {
					return struct{}{}, fmt.Errorf("plot %q is not present in the active project/SU scope", p)
				}
				if !seen[p] {
					seen[p] = true
					plotOrder = append(plotOrder, p)
				}
			}
		} else {
			rings, bbox, rErr := normalizeAOIRings(req.Polygon, req.Rings)
			if rErr != nil {
				return struct{}{}, rErr
			}
			for _, rec := range records {
				if rec.Lon < bbox[0] || rec.Lon > bbox[2] || rec.Lat < bbox[1] || rec.Lat > bbox[3] {
					continue
				}
				if len(utf16.Encode([]rune(rec.PlotNumber))) > 7 {
					continue
				}
				if PointInPolygonRingsLonLat(rec.Lon, rec.Lat, rings) && !seen[rec.PlotNumber] {
					seen[rec.PlotNumber] = true
					plotOrder = append(plotOrder, rec.PlotNumber)
				}
			}
		}
		if len(plotOrder) == 0 {
			return struct{}{}, errors.New("no plots matched the AOI selection")
		}

		var allProjectPlots []string
		if req.IncludeNonAOIPlotsAsUnassigned && mode != "update_active" {
			rawRows, qErr := queryScopedSpatialPlotRows(ctx, owner)
			if qErr != nil {
				return struct{}{}, qErr
			}
			for _, r := range rawRows {
				if len(utf16.Encode([]rune(r.plotNumber))) <= 7 {
					allProjectPlots = append(allProjectPlots, r.plotNumber)
				}
			}
		}

		proposalJSON, jErr := json.Marshal(struct {
			Mode      string   `json:"mode"`
			SUName    string   `json:"suName"`
			SiteUnit  string   `json:"siteUnit"`
			PlotCount int      `json:"plotCount"`
			Plots     []string `json:"plots"`
		}{Mode: mode, SUName: req.SUName, SiteUnit: siteUnit, PlotCount: len(plotOrder), Plots: plotOrder})
		if jErr != nil {
			return struct{}{}, jErr
		}

		switch mode {
		case "update_active":
			suName := owner.Selection.SU
			if suName == "" || suName == "None" {
				return struct{}{}, errors.New("no active SU table is selected; choose create_project_su or create_su_file")
			}
			if strings.Contains(strings.ToLower(suName), "master") {
				return struct{}{}, fmt.Errorf("active SU %q is a protected master table and cannot be modified", suName)
			}
			tableName := suName + "_SU"
			targetPath := owner.Selection.SUPath
			for _, supRole := range []string{"VPro64", "VLists", "VUser", "VMetaData", "VMessageBoard", "VPics"} {
				if supPath := owner.Attachments[supRole]; supPath != "" && vprocore.SameDesktopPath(supPath, targetPath) {
					return struct{}{}, fmt.Errorf("active SU file aliases protected support database %s", supRole)
				}
			}
			role := "su"
			if owner.Selection.SUPath == owner.Selection.ProjectPath {
				role = "project"
			}
			var changed int
			wErr := owner.WithSelectedFileWriter(ctx, role, owner.ValidateMetadataWriterFiles, func(conn *sql.Conn) (txErr error) {
				tx, bErr := conn.BeginTx(ctx, nil)
				if bErr != nil {
					return bErr
				}
				defer func() {
					if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
						txErr = errors.Join(txErr, rb)
					}
				}()
				c, uErr := upsertActiveSURowsInTx(ctx, tx, tableName, siteUnit, plotOrder)
				if uErr != nil {
					return uErr
				}
				changed = c
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, spatialAOISUHistoryTable, spatialAOISUHistorySQL, string(proposalJSON), "Spatial AOI SU update"); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return tx.Commit()
			})
			if wErr != nil {
				return struct{}{}, wErr
			}
			out = &AssignSpatialAOIToSUResult{
				ContextID:    contextID,
				SUName:       suName,
				TableName:    tableName,
				FilePath:     targetPath,
				SiteUnit:     siteUnit,
				MatchedPlots: len(plotOrder),
				ChangedRows:  changed,
			}

		case "create_project_su":
			suName, vErr := validateSpatialSUName(req.SUName)
			if vErr != nil {
				return struct{}{}, vErr
			}
			tableName := suName + "_SU"
			targetPath := owner.Selection.ProjectPath
			var inserted int
			wErr := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
				tx, bErr := conn.BeginTx(ctx, nil)
				if bErr != nil {
					return bErr
				}
				defer func() {
					if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
						txErr = errors.Join(txErr, rb)
					}
				}()
				if err := createSpatialSUTableInTx(ctx, tx, suName); err != nil {
					return err
				}
				qTable := vprocore.QuoteHeaderIdentifier(tableName)
				for _, p := range plotOrder {
					if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, p, siteUnit); err != nil {
						return err
					}
					inserted++
				}
				for _, p := range allProjectPlots {
					if seen[p] {
						continue
					}
					if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, NULL)`, p); err != nil {
						return err
					}
					inserted++
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, spatialAOISUHistoryTable, spatialAOISUHistorySQL, string(proposalJSON), "Spatial AOI project SU creation"); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return tx.Commit()
			})
			if wErr != nil {
				return struct{}{}, wErr
			}
			out = &AssignSpatialAOIToSUResult{
				ContextID:    contextID,
				SUName:       suName,
				TableName:    tableName,
				FilePath:     targetPath,
				SiteUnit:     siteUnit,
				MatchedPlots: len(plotOrder),
				ChangedRows:  inserted,
			}

		case "create_su_file":
			suName, vErr := validateSpatialSUName(req.SUName)
			if vErr != nil {
				return struct{}{}, vErr
			}
			freshPath, _, fErr := vprocore.FreshSQLiteDestination(req.FilePath, "SU")
			if fErr != nil {
				return struct{}{}, fErr
			}
			tmpFile, tErr := os.CreateTemp(filepath.Dir(freshPath), ".vpro-spatial-su-*.db")
			if tErr != nil {
				return struct{}{}, tErr
			}
			tmpPath := tmpFile.Name()
			if err := tmpFile.Close(); err != nil {
				_ = os.Remove(tmpPath)
				return struct{}{}, err
			}
			defer func() {
				_ = os.Remove(tmpPath)
				for _, suf := range []string{"-journal", "-wal", "-shm"} {
					_ = os.Remove(tmpPath + suf)
				}
			}()
			tableName := suName + "_SU"
			db, oErr := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_foreign_keys=on&_busy_timeout=5000&_journal_mode=DELETE&_synchronous=FULL&_txlock=immediate")
			if oErr != nil {
				return struct{}{}, oErr
			}
			db.SetMaxOpenConns(1)
			tx, bErr := db.BeginTx(ctx, nil)
			if bErr != nil {
				_ = db.Close()
				return struct{}{}, bErr
			}
			if err := createSpatialSUTableInTx(ctx, tx, suName); err != nil {
				_ = tx.Rollback()
				_ = db.Close()
				return struct{}{}, err
			}
			qTable := vprocore.QuoteHeaderIdentifier(tableName)
			inserted := 0
			for _, p := range plotOrder {
				if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, p, siteUnit); err != nil {
					_ = tx.Rollback()
					_ = db.Close()
					return struct{}{}, err
				}
				inserted++
			}
			for _, p := range allProjectPlots {
				if seen[p] {
					continue
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber", "SiteUnit") VALUES (?, NULL)`, p); err != nil {
					_ = tx.Rollback()
					_ = db.Close()
					return struct{}{}, err
				}
				inserted++
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, spatialAOISUHistoryTable, spatialAOISUHistorySQL, string(proposalJSON), "Spatial AOI SU file creation"); err != nil {
				_ = tx.Rollback()
				_ = db.Close()
				return struct{}{}, err
			}
			if err := tx.Commit(); err != nil {
				_ = db.Close()
				return struct{}{}, err
			}
			if err := db.Close(); err != nil {
				return struct{}{}, err
			}
			if err := sqlutil.LinkOrCreateExclusive(tmpPath, freshPath); err != nil {
				return struct{}{}, fmt.Errorf("SU publication requires atomic no-replace linking; destination unchanged: %w", err)
			}
			out = &AssignSpatialAOIToSUResult{
				ContextID:    contextID,
				SUName:       suName,
				TableName:    tableName,
				FilePath:     freshPath,
				SiteUnit:     siteUnit,
				MatchedPlots: len(plotOrder),
				ChangedRows:  inserted,
			}
		}

		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	if req.Activate && out != nil && mode != "update_active" {
		nextState, swErr := contexts.SwitchContext(contextID, vprocore.ContextSelection{
			Project:       currentSelection.Project,
			ProjectPath:   currentSelection.ProjectPath,
			SU:            out.SUName,
			SUPath:        out.FilePath,
			Hierarchy:     currentSelection.Hierarchy,
			HierarchyPath: currentSelection.HierarchyPath,
		})
		if swErr == nil {
			out.Activated = true
			out.ContextID = nextState.ContextID
			out.NewState = &nextState
		}
	}
	return out, nil
}
