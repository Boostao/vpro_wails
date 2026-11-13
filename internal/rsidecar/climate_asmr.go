package rsidecar

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	PlotClimateTableName = "__VPRO_PlotClimate"
	plotClimateDDL       = `CREATE TABLE IF NOT EXISTS main."__VPRO_PlotClimate" (
		"PlotNumber" TEXT PRIMARY KEY,
		"CoordElevHash" TEXT NOT NULL,
		"RefPeriod" TEXT NOT NULL,
		"Source" TEXT NOT NULL,
		"MAT" REAL,
		"MWMT" REAL,
		"MCMT" REAL,
		"MAP" REAL,
		"MSP" REAL,
		"PAS" REAL,
		"CMD" REAL,
		"PPT_05" REAL,
		"PPT_06" REAL,
		"PPT_07" REAL,
		"PPT_08" REAL,
		"PPT_09" REAL,
		"PPT_at" REAL,
		"PPT_wt" REAL,
		"DD5" REAL,
		"NFFD" REAL,
		"ComputedAt" TEXT NOT NULL
	)`
)

// PlotClimateRow represents one cached plot climate normal record in __VPRO_PlotClimate
// enriched with the plot's site/edatopic attributes and computed Ministry CMD -> aSMR output.
type PlotClimateRow struct {
	PlotNumber     string   `json:"plotNumber"`
	CoordElevHash  string   `json:"coordElevHash"`
	RefPeriod      string   `json:"refPeriod"`
	Source         string   `json:"source"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	Elevation      float64  `json:"elevation"`
	BGC            string   `json:"bgc"`
	SiteUnit       string   `json:"siteUnit"`
	SlopeGradient  *float64 `json:"slopeGradient,omitempty"`
	Aspect         *float64 `json:"aspect,omitempty"`
	RSMR           string   `json:"rsmr"`
	RSNR           string   `json:"rsnr"`
	MAT            float64  `json:"mat"`
	MWMT           float64  `json:"mwmt"`
	MCMT           float64  `json:"mcmt"`
	MAP            float64  `json:"map"`
	MSP            float64  `json:"msp"`
	PAS            float64  `json:"pas"`
	CMD            float64  `json:"cmd"`
	PPT05          float64  `json:"ppt05"`
	PPT06          float64  `json:"ppt06"`
	PPT07          float64  `json:"ppt07"`
	PPT08          float64  `json:"ppt08"`
	PPT09          float64  `json:"ppt09"`
	PPTAt          float64  `json:"pptAt"`
	PPTWt          float64  `json:"pptWt"`
	DD5            float64  `json:"dd5"`
	NFFD           float64  `json:"nffd"`
	MoistMonths    int      `json:"moistMonths"`
	DormantDeficit float64  `json:"dormantDeficit"`
	AspectFactor   float64  `json:"aspectFactor"`
	CMDTopo        float64  `json:"cmdTopo"`
	CMDSite        *float64 `json:"cmdSite,omitempty"`
	ASMR           *float64 `json:"asmr,omitempty"`
	ComputedAt     string   `json:"computedAt"`
}

// EdatopicGridCell represents one (moisture x nutrient) cell in either a Relative (rSMR) or Actual (aSMR) edatopic grid.
type EdatopicGridCell struct {
	Moisture    string   `json:"moisture"`
	Nutrient    string   `json:"nutrient"`
	Count       int      `json:"count"`
	Frequency   float64  `json:"frequency"`
	MeanCMD     *float64 `json:"meanCmd,omitempty"`
	MeanCMDSite *float64 `json:"meanCmdSite,omitempty"`
	PlotNumbers []string `json:"plotNumbers"`
}

// EdatopicGridMatrix holds a full 2D edatopic grid summary (rSMR x rSNR or aSMR x rSNR).
type EdatopicGridMatrix struct {
	Kind         string             `json:"kind"` // "relative" | "actual"
	MoistureAxis []string           `json:"moistureAxis"`
	NutrientAxis []string           `json:"nutrientAxis"`
	TotalPlots   int                `json:"totalPlots"`
	ModalCell    string             `json:"modalCell"`
	Cells        []EdatopicGridCell `json:"cells"`
}

// DualUnitEdatopicSummary pairs Relative (rSMR x rSNR) and Actual (aSMR x rSNR) grids for a BGC or SiteUnit.
type DualUnitEdatopicSummary struct {
	GroupKey      string             `json:"groupKey"`
	BGC           string             `json:"bgc"`
	SiteUnit      string             `json:"siteUnit"`
	PlotCount     int                `json:"plotCount"`
	MeanMAT       float64            `json:"meanMat"`
	MeanMAP       float64            `json:"meanMap"`
	MeanCMD       float64            `json:"meanCmd"`
	MeanASMR      *float64           `json:"meanAsmr,omitempty"`
	MeanShift     *float64           `json:"meanShift,omitempty"` // mean(aSMR - rSMR)
	RelativeGrid  EdatopicGridMatrix `json:"relativeGrid"`
	ActualGrid    EdatopicGridMatrix `json:"actualGrid"`
}

// ClimateEdatopicRequest configures batch __VPRO_PlotClimate caching and CMD -> aSMR dual edatopic computation.
type ClimateEdatopicRequest struct {
	RefPeriod         string   `json:"refPeriod"`   // "1961_1990" (default) | "1991_2020"
	GroupBy           string   `json:"groupBy"`     // "bgc" | "site_unit" (default "site_unit" when SU active else "bgc")
	BGCFilter         string   `json:"bgcFilter"`   // Optional BGC filter
	SiteUnitFilter    string   `json:"siteUnitFilter"`
	UseActiveSU       bool     `json:"useActiveSu"`
	PlotNumbers       []string `json:"plotNumbers,omitempty"`
	ForceRefresh      bool     `json:"forceRefresh"`
	ForceOfflineModel bool     `json:"forceOfflineModel"`
}

// ClimateEdatopicResult returns cached/newly-computed plot climate rows and side-by-side Relative vs Actual edatopic grids.
type ClimateEdatopicResult struct {
	ContextID          string                    `json:"contextId"`
	Project            string                    `json:"project"`
	ActiveSU           string                    `json:"activeSu"`
	RefPeriod          string                    `json:"refPeriod"`
	TotalScopedPlots   int                       `json:"totalScopedPlots"`
	CoordinatizedPlots int                       `json:"coordinatizedPlots"`
	SkippedNoCoords    int                       `json:"skippedNoCoords"`
	CacheHits          int                       `json:"cacheHits"`
	CacheMisses        int                       `json:"cacheMisses"`
	SourceSummary      map[string]int            `json:"sourceSummary"`
	OverallRelative    EdatopicGridMatrix        `json:"overallRelative"`
	OverallActual      EdatopicGridMatrix        `json:"overallActual"`
	UnitSummaries      []DualUnitEdatopicSummary `json:"unitSummaries"`
	Plots              []PlotClimateRow          `json:"plots"`
}

type asmrRuleRow struct {
	SMRLevel int
	CMDMax   float64
	ASMR     float64
}

var (
	asmrRulesOnce sync.Once
	asmrRulesData []asmrRuleRow
	asmrRulesErr  error
)

func loadEmbeddedASMRRules() ([]asmrRuleRow, error) {
	asmrRulesOnce.Do(func() {
		raw, err := embeddedAssets.ReadFile("assets/aSMR_Rules_HalfStep.csv")
		if err != nil {
			asmrRulesErr = err
			return
		}
		raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
		reader := csv.NewReader(bytes.NewReader(raw))
		records, err := reader.ReadAll()
		if err != nil {
			asmrRulesErr = err
			return
		}
		for i, rec := range records {
			if i == 0 || len(rec) < 3 {
				continue
			}
			lvl, err1 := strconv.Atoi(strings.TrimSpace(rec[0]))
			cmdMax, err2 := strconv.ParseFloat(strings.TrimSpace(rec[1]), 64)
			asmrVal, err3 := strconv.ParseFloat(strings.TrimSpace(rec[2]), 64)
			if err1 != nil || err2 != nil || err3 != nil {
				continue
			}
			asmrRulesData = append(asmrRulesData, asmrRuleRow{
				SMRLevel: lvl,
				CMDMax:   cmdMax,
				ASMR:     asmrVal,
			})
		}
	})
	return asmrRulesData, asmrRulesErr
}

// NormalizeBCWesternLongitude converts legacy positive BC western longitudes (e.g. 123.629 -> -123.629)
// while leaving already-negative longitudes intact.
func NormalizeBCWesternLongitude(lon float64) float64 {
	if lon > 0 && lon >= 110.0 && lon <= 145.0 {
		return -lon
	}
	return lon
}

// NormalizeRefPeriod normalizes climate reference period identifiers.
func NormalizeRefPeriod(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "1961_1990", nil
	}
	normalized := strings.ReplaceAll(trimmed, "-", "_")
	switch normalized {
	case "1961_1990", "1991_2020":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported climate reference period %q (expected 1961_1990 or 1991_2020)", raw)
	}
}

// ComputeCoordElevHash computes a deterministic 128-bit hex key over (Round(Lat,5), Round(Lon,5), Round(Elev,1), RefPeriod).
func ComputeCoordElevHash(lat, lon, elev float64, refPeriod string) string {
	normLon := NormalizeBCWesternLongitude(lon)
	lat5 := math.Round(lat*1e5) / 1e5
	lon5 := math.Round(normLon*1e5) / 1e5
	elev1 := math.Round(elev*10.0) / 10.0
	canonical := fmt.Sprintf("%.5f|%.5f|%.1f|%s", lat5, lon5, elev1, refPeriod)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:16])
}

func parseRSMRDigit(raw string) (int, bool) {
	s := strings.TrimSpace(raw)
	for _, ch := range s {
		if ch >= '0' && ch <= '8' {
			return int(ch - '0'), true
		}
	}
	return 0, false
}

func normalizeRSNRCode(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	for _, ch := range s {
		if ch >= 'A' && ch <= 'F' {
			if ch == 'F' {
				// Map hyper-eutrophic F into E+ / E column when displaying standard 5-column A..E BEC grid, or preserve A..E
				return "E"
			}
			return string(ch)
		}
	}
	return ""
}

// ASMRTransferOutput holds intermediate and final outputs of the Ministry CMD -> aSMR transfer function.
type ASMRTransferOutput struct {
	MoistMonths    int
	DormantPrecip  float64
	DormantDeficit float64
	AspectFactor   float64
	CMDTopo        float64
	CMDSite        *float64
	ASMR           *float64
}

// EvaluateMinistryASMR implements the exact Ministry CMD -> aSMR transfer function from
// BEC_R_working/_functions/_calculate_aSMR.R + aSMR_Rules_HalfStep.csv.
func EvaluateMinistryASMR(
	cmdAnnual float64,
	ppt05, ppt06, ppt07, ppt08, ppt09 float64,
	pptAt, pptWt float64,
	slope, aspect *float64,
	rsmrRaw string,
) (ASMRTransferOutput, error) {
	rules, err := loadEmbeddedASMRRules()
	if err != nil {
		return ASMRTransferOutput{}, err
	}

	moistCnt := 0
	for _, p := range []float64{ppt05, ppt06, ppt07, ppt08, ppt09} {
		if !math.IsNaN(p) && p >= 90.0 {
			moistCnt++
		}
	}
	moistFlag := 0.0
	if moistCnt > 4 {
		moistFlag = 1.0
	}
	multiplier := 1.0 + (moistFlag / 5.0)
	cmdGS := cmdAnnual / multiplier

	pptDorm := pptAt + pptWt
	defVal := math.Max(0.0, 350.0-pptDorm)
	cmdTotal := cmdGS + defVal

	aspectFactor := 1.0
	if slope != nil && aspect != nil && !math.IsNaN(*slope) && !math.IsNaN(*aspect) && *slope > 35.0 {
		if *aspect > 180.0 && *aspect < 270.0 {
			aspectFactor = 1.25
		} else if *aspect > 315.0 || (*aspect >= 0.0 && *aspect < 45.0) {
			aspectFactor = 0.85
		}
	}
	cmdTopo := math.Round(cmdTotal*aspectFactor*10.0) / 10.0

	out := ASMRTransferOutput{
		MoistMonths:    moistCnt,
		DormantPrecip:  math.Round(pptDorm*10.0) / 10.0,
		DormantDeficit: math.Round(defVal*10.0) / 10.0,
		AspectFactor:   aspectFactor,
		CMDTopo:        cmdTopo,
	}

	rInt, ok := parseRSMRDigit(rsmrRaw)
	if !ok {
		return out, nil
	}
	if rInt == 8 {
		zero := 0.0
		eight := 8.0
		out.CMDSite = &zero
		out.ASMR = &eight
		return out, nil
	}

	var cmdSite float64
	switch {
	case rInt < 4:
		cmdSite = cmdTopo + float64(4-rInt)*125.0
	case rInt == 4:
		cmdSite = cmdTopo
	case rInt == 5:
		cmdSite = cmdTopo / 2.0
	case rInt == 6:
		cmdSite = cmdTopo / 4.0
	default: // rInt == 7
		cmdSite = cmdTopo / 8.0
	}
	cmdSite = math.Round(math.Max(0.0, cmdSite)*10.0) / 10.0
	out.CMDSite = &cmdSite

	smrLevel := 0
	if rInt >= 5 {
		smrLevel = rInt
	}
	bestASMR := 0.0
	matched := false
	for _, r := range rules {
		if r.SMRLevel == smrLevel && cmdSite <= r.CMDMax {
			if !matched || r.ASMR > bestASMR {
				bestASMR = r.ASMR
				matched = true
			}
		}
	}
	out.ASMR = &bestASMR
	return out, nil
}

func formatASMRLabel(v float64) string {
	rounded := math.Round(v*2.0) / 2.0
	if math.Abs(rounded-math.Round(rounded)) < 1e-6 {
		return strconv.Itoa(int(math.Round(rounded)))
	}
	return fmt.Sprintf("%.1f", rounded)
}

func buildEdatopicMatrices(rows []PlotClimateRow) (EdatopicGridMatrix, EdatopicGridMatrix) {
	nutrients := []string{"A", "B", "C", "D", "E"}
	relRows := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}
	actRows := []string{"0", "0.5", "1", "1.5", "2", "2.5", "3", "3.5", "4", "4.5", "5", "5.5", "6", "6.5", "7", "7.5", "8"}

	type accum struct {
		plots      []string
		cmdSum     float64
		cmdCnt     int
		cmdSiteSum float64
		cmdSiteCnt int
	}
	relMap := make(map[string]*accum)
	actMap := make(map[string]*accum)
	relTotal := 0
	actTotal := 0

	for _, r := range rows {
		snr := normalizeRSNRCode(r.RSNR)
		if snr == "" {
			continue
		}
		if rDigit, ok := parseRSMRDigit(r.RSMR); ok {
			key := fmt.Sprintf("%d|%s", rDigit, snr)
			a := relMap[key]
			if a == nil {
				a = &accum{}
				relMap[key] = a
			}
			a.plots = append(a.plots, r.PlotNumber)
			a.cmdSum += r.CMD
			a.cmdCnt++
			if r.CMDSite != nil {
				a.cmdSiteSum += *r.CMDSite
				a.cmdSiteCnt++
			}
			relTotal++
		}
		if r.ASMR != nil {
			aLbl := formatASMRLabel(*r.ASMR)
			key := fmt.Sprintf("%s|%s", aLbl, snr)
			a := actMap[key]
			if a == nil {
				a = &accum{}
				actMap[key] = a
			}
			a.plots = append(a.plots, r.PlotNumber)
			a.cmdSum += r.CMD
			a.cmdCnt++
			if r.CMDSite != nil {
				a.cmdSiteSum += *r.CMDSite
				a.cmdSiteCnt++
			}
			actTotal++
		}
	}

	materialize := func(kind string, mAxis []string, total int, mp map[string]*accum) EdatopicGridMatrix {
		cells := make([]EdatopicGridCell, 0, len(mAxis)*len(nutrients))
		modal := ""
		modalCnt := 0
		for _, m := range mAxis {
			for _, n := range nutrients {
				a := mp[m+"|"+n]
				if a == nil || len(a.plots) == 0 {
					continue
				}
				cnt := len(a.plots)
				if cnt > modalCnt {
					modalCnt = cnt
					modal = m + "/" + n
				}
				var meanCmd, meanCmdSite *float64
				if a.cmdCnt > 0 {
					v := math.Round((a.cmdSum/float64(a.cmdCnt))*10.0) / 10.0
					meanCmd = &v
				}
				if a.cmdSiteCnt > 0 {
					v := math.Round((a.cmdSiteSum/float64(a.cmdSiteCnt))*10.0) / 10.0
					meanCmdSite = &v
				}
				freq := 0.0
				if total > 0 {
					freq = math.Round((float64(cnt)/float64(total))*1000.0) / 10.0
				}
				plotsCopy := make([]string, len(a.plots))
				copy(plotsCopy, a.plots)
				sort.Strings(plotsCopy)
				cells = append(cells, EdatopicGridCell{
					Moisture:    m,
					Nutrient:    n,
					Count:       cnt,
					Frequency:   freq,
					MeanCMD:     meanCmd,
					MeanCMDSite: meanCmdSite,
					PlotNumbers: plotsCopy,
				})
			}
		}
		return EdatopicGridMatrix{
			Kind:         kind,
			MoistureAxis: mAxis,
			NutrientAxis: nutrients,
			TotalPlots:   total,
			ModalCell:    modal,
			Cells:        cells,
		}
	}

	return materialize("relative", relRows, relTotal, relMap), materialize("actual", actRows, actTotal, actMap)
}

type scopedEnvPlot struct {
	PlotNumber string
	Zone       string
	SubZone    string
	BGC        string
	SiteUnit   string
	Latitude   *float64
	Longitude  *float64
	Elevation  *float64
	Slope      *float64
	Aspect     *float64
	RSMR       string
	RSNR       string
}

func readScopedEnvPlots(
	ctx context.Context,
	owner *vprocore.SqliteContext,
	useActiveSU bool,
	bgcFilter string,
	siteUnitFilter string,
	explicitPlots []string,
) ([]scopedEnvPlot, error) {
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return nil, err
	}
	defer owner.Mu.Unlock()
	if owner.Conn == nil {
		return nil, errors.New("project context is closed")
	}

	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	envTable := vprocore.QuoteHeaderIdentifier(owner.Selection.Project + "_Env")
	suActive := owner.Selection.SU != "" && owner.Selection.SU != "None"

	suMap := make(map[string]string)
	if suActive {
		suSchema := "main"
		if _, ok := owner.Attachments["su"]; ok {
			suSchema = "su"
		} else if owner.Selection.SUPath != "" && !vprocore.SameDesktopPath(owner.Selection.SUPath, owner.Selection.ProjectPath) {
			suSchema = "su"
		}
		suTable := vprocore.QuoteHeaderIdentifier(owner.Selection.SU + "_SU")
		// Check whether SU table exists in suSchema or main
		var suExists int
		_ = tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, suSchema), owner.Selection.SU+"_SU").Scan(&suExists)
		if suExists == 0 && suSchema != "main" {
			_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, owner.Selection.SU+"_SU").Scan(&suExists)
			if suExists > 0 {
				suSchema = "main"
			}
		}
		if suExists > 0 {
			sRows, qErr := tx.QueryContext(ctx, fmt.Sprintf(
				`SELECT CAST("PlotNumber" AS TEXT), TRIM(COALESCE(CAST("SiteUnit" AS TEXT), '')) FROM %s.%s WHERE typeof("PlotNumber")='text'`,
				suSchema, suTable,
			))
			if qErr == nil {
				for sRows.Next() {
					var p, u string
					if err := sRows.Scan(&p, &u); err == nil {
						suMap[p] = u
					}
				}
				sRows.Close()
			}
		}
	}

	// Check whether Admin table has GIS_BGC for plots missing field Zone/SubZone
	gisMap := make(map[string]string)
	adminTableName := owner.Selection.Project + "_Admin"
	var adminExists int
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, adminTableName).Scan(&adminExists)
	envSchema := "project"
	if adminExists == 0 {
		var mainEnvExists int
		_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, owner.Selection.Project+"_Env").Scan(&mainEnvExists)
		if mainEnvExists > 0 {
			envSchema = "main"
			_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, adminTableName).Scan(&adminExists)
		}
	}
	if adminExists > 0 {
		aRows, aErr := tx.QueryContext(ctx, fmt.Sprintf(
			`SELECT CAST("Plot" AS TEXT), TRIM(COALESCE(CAST("GIS_BGC" AS TEXT), '')) FROM %s.%s WHERE typeof("Plot")='text'`,
			envSchema, vprocore.QuoteHeaderIdentifier(adminTableName),
		))
		if aErr == nil {
			for aRows.Next() {
				var p, g string
				if err := aRows.Scan(&p, &g); err == nil && g != "" {
					gisMap[p] = strings.ReplaceAll(g, " ", "")
				}
			}
			aRows.Close()
		}
	}

	allowedSet := make(map[string]bool, len(explicitPlots))
	for _, p := range explicitPlots {
		if p != "" {
			allowedSet[p] = true
		}
	}
	bgcFilterClean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(bgcFilter), " ", ""))
	unitFilterClean := strings.ToUpper(strings.TrimSpace(siteUnitFilter))

	query := fmt.Sprintf(
		`SELECT CAST("PlotNumber" AS TEXT),
		        TRIM(COALESCE(CAST("Zone" AS TEXT), '')),
		        TRIM(COALESCE(CAST("SubZone" AS TEXT), '')),
		        TRIM(COALESCE(CAST("SiteSeries" AS TEXT), '')),
		        CAST("Latitude" AS REAL),
		        CAST("Longitude" AS REAL),
		        CAST("Elevation" AS REAL),
		        CAST("SlopeGradient" AS REAL),
		        CAST("Aspect" AS REAL),
		        TRIM(COALESCE(CAST("MoistureRegime" AS TEXT), '')),
		        TRIM(COALESCE(CAST("NutrientRegime" AS TEXT), ''))
		   FROM %s.%s
		  WHERE typeof("PlotNumber")='text'
		  ORDER BY "PlotNumber" ASC`,
		envSchema, envTable,
	)
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []scopedEnvPlot
	for rows.Next() {
		var plotNum, zone, subZone, siteSeries, rsmr, rsnr string
		var lat, lon, elev, slope, aspect sql.NullFloat64
		if err := rows.Scan(&plotNum, &zone, &subZone, &siteSeries, &lat, &lon, &elev, &slope, &aspect, &rsmr, &rsnr); err != nil {
			return nil, err
		}
		if len(allowedSet) > 0 && !allowedSet[plotNum] {
			continue
		}
		suUnit, inSU := suMap[plotNum]
		if useActiveSU && suActive && !inSU {
			continue
		}

		bgc := strings.ReplaceAll(zone+subZone, " ", "")
		if bgc == "" {
			bgc = gisMap[plotNum]
		}
		effectiveUnit := suUnit
		if effectiveUnit == "" && bgc != "" && siteSeries != "" {
			effectiveUnit = bgc + "/" + siteSeries
		} else if effectiveUnit == "" && bgc != "" {
			effectiveUnit = bgc
		}

		if bgcFilterClean != "" {
			bgcUpper := strings.ToUpper(bgc)
			if bgcUpper != bgcFilterClean && !strings.HasPrefix(bgcUpper, bgcFilterClean) {
				continue
			}
		}
		if unitFilterClean != "" && strings.ToUpper(strings.TrimSpace(effectiveUnit)) != unitFilterClean {
			continue
		}

		item := scopedEnvPlot{
			PlotNumber: plotNum,
			Zone:       zone,
			SubZone:    subZone,
			BGC:        bgc,
			SiteUnit:   effectiveUnit,
			RSMR:       rsmr,
			RSNR:       rsnr,
		}
		if lat.Valid && !math.IsNaN(lat.Float64) && math.Abs(lat.Float64) <= 90.0 && lat.Float64 != 0 {
			v := lat.Float64
			item.Latitude = &v
		}
		if lon.Valid && !math.IsNaN(lon.Float64) && math.Abs(lon.Float64) <= 180.0 && lon.Float64 != 0 {
			v := NormalizeBCWesternLongitude(lon.Float64)
			item.Longitude = &v
		}
		if elev.Valid && !math.IsNaN(elev.Float64) && elev.Float64 >= -100.0 && elev.Float64 <= 6000.0 {
			v := elev.Float64
			item.Elevation = &v
		}
		if slope.Valid && !math.IsNaN(slope.Float64) {
			v := slope.Float64
			item.Slope = &v
		}
		if aspect.Valid && !math.IsNaN(aspect.Float64) {
			v := aspect.Float64
			item.Aspect = &v
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}

type cachedNormalRecord struct {
	PlotNumber    string
	CoordElevHash string
	RefPeriod     string
	Source        string
	MAT           float64
	MWMT          float64
	MCMT          float64
	MAP           float64
	MSP           float64
	PAS           float64
	CMD           float64
	PPT05         float64
	PPT06         float64
	PPT07         float64
	PPT08         float64
	PPT09         float64
	PPTAt         float64
	PPTWt         float64
	DD5           float64
	NFFD          float64
	ComputedAt    string
}

func readExistingPlotClimateCache(ctx context.Context, db *sql.DB) (map[string]cachedNormalRecord, error) {
	out := make(map[string]cachedNormalRecord)
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name = ?`, PlotClimateTableName).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT CAST("PlotNumber" AS TEXT), "CoordElevHash", "RefPeriod", "Source",
		       COALESCE("MAT",0), COALESCE("MWMT",0), COALESCE("MCMT",0), COALESCE("MAP",0),
		       COALESCE("MSP",0), COALESCE("PAS",0), COALESCE("CMD",0),
		       COALESCE("PPT_05",0), COALESCE("PPT_06",0), COALESCE("PPT_07",0), COALESCE("PPT_08",0), COALESCE("PPT_09",0),
		       COALESCE("PPT_at",0), COALESCE("PPT_wt",0), COALESCE("DD5",0), COALESCE("NFFD",0), "ComputedAt"
		  FROM main."__VPRO_PlotClimate"
		 WHERE typeof("PlotNumber")='text'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r cachedNormalRecord
		if err := rows.Scan(
			&r.PlotNumber, &r.CoordElevHash, &r.RefPeriod, &r.Source,
			&r.MAT, &r.MWMT, &r.MCMT, &r.MAP, &r.MSP, &r.PAS, &r.CMD,
			&r.PPT05, &r.PPT06, &r.PPT07, &r.PPT08, &r.PPT09,
			&r.PPTAt, &r.PPTWt, &r.DD5, &r.NFFD, &r.ComputedAt,
		); err != nil {
			return nil, err
		}
		out[r.PlotNumber] = r
	}
	return out, rows.Err()
}

func readExistingPlotClimateCacheReadOnly(ctx context.Context, owner *vprocore.SqliteContext) (map[string]cachedNormalRecord, error) {
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return nil, err
	}
	projectPath := owner.Selection.ProjectPath
	owner.Mu.Unlock()

	roDB, err := sqlutil.OpenReadOnlyContext(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	defer roDB.Close()
	return readExistingPlotClimateCache(ctx, roDB)
}

type rClimatePointInput struct {
	CoordElevHash string  `json:"coordElevHash"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Elev          float64 `json:"elev"`
	BGC           string  `json:"bgc"`
	Zone          string  `json:"zone"`
}

type rClimateNormalOutput struct {
	CoordElevHash string  `json:"coordElevHash"`
	Source        string  `json:"source"`
	MAT           float64 `json:"mat"`
	MWMT          float64 `json:"mwmt"`
	MCMT          float64 `json:"mcmt"`
	MAP           float64 `json:"map"`
	MSP           float64 `json:"msp"`
	PAS           float64 `json:"pas"`
	CMD           float64 `json:"cmd"`
	PPT05         float64 `json:"ppt05"`
	PPT06         float64 `json:"ppt06"`
	PPT07         float64 `json:"ppt07"`
	PPT08         float64 `json:"ppt08"`
	PPT09         float64 `json:"ppt09"`
	PPTAt         float64 `json:"pptAt"`
	PPTWt         float64 `json:"pptWt"`
	DD5           float64 `json:"dd5"`
	NFFD          float64 `json:"nffd"`
}

type rClimateScriptResponse struct {
	OK              bool                   `json:"ok"`
	RefPeriod       string                 `json:"refPeriod"`
	VProCompatible  bool                   `json:"vproCompatible"`
	ClimrUsed       bool                   `json:"climrUsed"`
	ComputedNormals []rClimateNormalOutput `json:"computedNormals"`
}

// GetCachedPlotClimate reads the currently cached __VPRO_PlotClimate rows enriched with plot site/edatopic metadata.
func GetCachedPlotClimate(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	refPeriod string,
) ([]PlotClimateRow, error) {
	if ctx == nil {
		return nil, errors.New("GetCachedPlotClimate requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("context service is unavailable")
	}
	period, err := NormalizeRefPeriod(refPeriod)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) ([]PlotClimateRow, error) {
		owner := plots.Projects.Sqlite
		envPlots, err := readScopedEnvPlots(ctx, owner, false, "", "", nil)
		if err != nil {
			return nil, err
		}
		cacheMap, err := readExistingPlotClimateCacheReadOnly(ctx, owner)
		if err != nil {
			return nil, err
		}

		out := make([]PlotClimateRow, 0, len(cacheMap))
		for _, ep := range envPlots {
			c, ok := cacheMap[ep.PlotNumber]
			if !ok || c.RefPeriod != period {
				continue
			}
			lat, lon, elev := 0.0, 0.0, 0.0
			if ep.Latitude != nil {
				lat = *ep.Latitude
			}
			if ep.Longitude != nil {
				lon = *ep.Longitude
			}
			if ep.Elevation != nil {
				elev = *ep.Elevation
			}
			tr, err := EvaluateMinistryASMR(c.CMD, c.PPT05, c.PPT06, c.PPT07, c.PPT08, c.PPT09, c.PPTAt, c.PPTWt, ep.Slope, ep.Aspect, ep.RSMR)
			if err != nil {
				return nil, err
			}
			out = append(out, PlotClimateRow{
				PlotNumber:     ep.PlotNumber,
				CoordElevHash:  c.CoordElevHash,
				RefPeriod:      c.RefPeriod,
				Source:         c.Source,
				Latitude:       lat,
				Longitude:      lon,
				Elevation:      elev,
				BGC:            ep.BGC,
				SiteUnit:       ep.SiteUnit,
				SlopeGradient:  ep.Slope,
				Aspect:         ep.Aspect,
				RSMR:           ep.RSMR,
				RSNR:           ep.RSNR,
				MAT:            c.MAT,
				MWMT:           c.MWMT,
				MCMT:           c.MCMT,
				MAP:            c.MAP,
				MSP:            c.MSP,
				PAS:            c.PAS,
				CMD:            c.CMD,
				PPT05:          c.PPT05,
				PPT06:          c.PPT06,
				PPT07:          c.PPT07,
				PPT08:          c.PPT08,
				PPT09:          c.PPT09,
				PPTAt:          c.PPTAt,
				PPTWt:          c.PPTWt,
				DD5:            c.DD5,
				NFFD:           c.NFFD,
				MoistMonths:    tr.MoistMonths,
				DormantDeficit: tr.DormantDeficit,
				AspectFactor:   tr.AspectFactor,
				CMDTopo:        tr.CMDTopo,
				CMDSite:        tr.CMDSite,
				ASMR:           tr.ASMR,
				ComputedAt:     c.ComputedAt,
			})
		}
		return out, nil
	})
}

// ComputePlotClimateAndDualEdatopic maintains __VPRO_PlotClimate keyed by (Round(Lat,5), Round(Lon,5), Elev, RefPeriod)
// and computes side-by-side Relative (rSMR x rSNR) and Actual (aSMR x rSNR) edatopic grid matrices.
func ComputePlotClimateAndDualEdatopic(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req ClimateEdatopicRequest,
	opts ...RuntimeOptions,
) (*ClimateEdatopicResult, error) {
	if ctx == nil {
		return nil, errors.New("ComputePlotClimateAndDualEdatopic requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("context service is unavailable")
	}
	if !utf8.ValidString(req.BGCFilter) || !utf8.ValidString(req.SiteUnitFilter) {
		return nil, errors.New("filter inputs must be valid UTF-8")
	}
	refPeriod, err := NormalizeRefPeriod(req.RefPeriod)
	if err != nil {
		return nil, err
	}
	cfg := effectiveOptions(opts...)

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ClimateEdatopicResult, error) {
		owner := plots.Projects.Sqlite
		envPlots, err := readScopedEnvPlots(ctx, owner, req.UseActiveSU, req.BGCFilter, req.SiteUnitFilter, req.PlotNumbers)
		if err != nil {
			return nil, err
		}

		cacheByPlot, err := readExistingPlotClimateCacheReadOnly(ctx, owner)
		if err != nil {
			return nil, err
		}

		// Also index already-cached normals by CoordElevHash so a new plot at identical coordinates reuses cached normals
		normByHash := make(map[string]cachedNormalRecord, len(cacheByPlot))
		if !req.ForceRefresh {
			for _, rec := range cacheByPlot {
				if rec.RefPeriod == refPeriod {
					normByHash[rec.CoordElevHash] = rec
				}
			}
		}

		uncachedByHash := make(map[string]rClimatePointInput)
		var uncachedList []rClimatePointInput
		cacheHits := 0
		cacheMisses := 0
		coordinatized := 0
		skippedNoCoords := 0

		evaluatedSpecs := make([]plotClimateSpec, 0, len(envPlots))

		for _, ep := range envPlots {
			if ep.Latitude == nil || ep.Longitude == nil || ep.Elevation == nil {
				skippedNoCoords++
				continue
			}
			coordinatized++
			h := ComputeCoordElevHash(*ep.Latitude, *ep.Longitude, *ep.Elevation, refPeriod)
			existingPlot, plotMatched := cacheByPlot[ep.PlotNumber]
			if !req.ForceRefresh && plotMatched && existingPlot.CoordElevHash == h && existingPlot.RefPeriod == refPeriod {
				cacheHits++
				evaluatedSpecs = append(evaluatedSpecs, plotClimateSpec{ep: ep, hash: h, hit: true})
				continue
			}
			cacheMisses++
			evaluatedSpecs = append(evaluatedSpecs, plotClimateSpec{ep: ep, hash: h, hit: false})
			if _, ok := normByHash[h]; !ok {
				if _, queued := uncachedByHash[h]; !queued {
					pt := rClimatePointInput{
						CoordElevHash: h,
						Lat:           math.Round((*ep.Latitude)*1e5) / 1e5,
						Lon:           math.Round((*ep.Longitude)*1e5) / 1e5,
						Elev:          math.Round((*ep.Elevation)*10.0) / 10.0,
						BGC:           ep.BGC,
						Zone:          ep.Zone,
					}
					uncachedByHash[h] = pt
					uncachedList = append(uncachedList, pt)
				}
			}
		}

		var projDB *sql.DB
		if len(uncachedList) > 0 || cacheMisses > 0 {
			projDB, err = owner.ProjectDatabase(ctx)
			if err != nil {
				return nil, err
			}
		}

		// Invoke out-of-process R sidecar only when there are uncached coordinate hashes to compute
		if len(uncachedList) > 0 {
			if err := CheckpointProjectWAL(ctx, projDB); err != nil {
				return nil, err
			}
			projectDir := ""
			if owner.Selection.ProjectPath != "" {
				projectDir = filepath.Dir(owner.Selection.ProjectPath)
			}
			var rResp rClimateScriptResponse
			rReq := map[string]any{
				"assetsDir":         ".",
				"projectPath":       owner.Selection.ProjectPath,
				"projectName":       owner.Selection.Project,
				"refPeriod":         refPeriod,
				"forceOfflineModel": req.ForceOfflineModel,
				"uncachedPoints":    uncachedList,
			}
			err := executeRScriptJSON(
				ctx,
				cfg,
				projectDir,
				"compute_climate_asmr.R",
				[]string{"aSMR_Rules_HalfStep.csv", "CMD_adjustments.csv", "bgc_climate_normals.csv"},
				rReq,
				&rResp,
			)
			if err != nil {
				return nil, err
			}
			nowISO := time.Now().UTC().Format(time.RFC3339)
			for _, cn := range rResp.ComputedNormals {
				normByHash[cn.CoordElevHash] = cachedNormalRecord{
					CoordElevHash: cn.CoordElevHash,
					RefPeriod:     refPeriod,
					Source:        cn.Source,
					MAT:           cn.MAT,
					MWMT:          cn.MWMT,
					MCMT:          cn.MCMT,
					MAP:           cn.MAP,
					MSP:           cn.MSP,
					PAS:           cn.PAS,
					CMD:           cn.CMD,
					PPT05:         cn.PPT05,
					PPT06:         cn.PPT06,
					PPT07:         cn.PPT07,
					PPT08:         cn.PPT08,
					PPT09:         cn.PPT09,
					PPTAt:         cn.PPTAt,
					PPTWt:         cn.PPTWt,
					DD5:           cn.DD5,
					NFFD:          cn.NFFD,
					ComputedAt:    nowISO,
				}
			}
		}

		// Persist any missed plot rows atomically inside BEGIN IMMEDIATE
		if cacheMisses > 0 {
			if err := upsertPlotClimateRowsAtomic(ctx, projDB, evaluatedSpecs, normByHash, refPeriod); err != nil {
				return nil, err
			}
		}

		// Assemble PlotClimateRow list & dual edatopic grids
		sourceSummary := make(map[string]int)
		plotRows := make([]PlotClimateRow, 0, len(evaluatedSpecs))
		for _, spec := range evaluatedSpecs {
			var norm cachedNormalRecord
			var ok bool
			if spec.hit {
				norm, ok = cacheByPlot[spec.ep.PlotNumber]
			} else {
				norm, ok = normByHash[spec.hash]
			}
			if !ok {
				continue
			}
			sourceSummary[norm.Source]++
			tr, err := EvaluateMinistryASMR(
				norm.CMD,
				norm.PPT05, norm.PPT06, norm.PPT07, norm.PPT08, norm.PPT09,
				norm.PPTAt, norm.PPTWt,
				spec.ep.Slope, spec.ep.Aspect,
				spec.ep.RSMR,
			)
			if err != nil {
				return nil, err
			}
			plotRows = append(plotRows, PlotClimateRow{
				PlotNumber:     spec.ep.PlotNumber,
				CoordElevHash:  spec.hash,
				RefPeriod:      refPeriod,
				Source:         norm.Source,
				Latitude:       *spec.ep.Latitude,
				Longitude:      *spec.ep.Longitude,
				Elevation:      *spec.ep.Elevation,
				BGC:            spec.ep.BGC,
				SiteUnit:       spec.ep.SiteUnit,
				SlopeGradient:  spec.ep.Slope,
				Aspect:         spec.ep.Aspect,
				RSMR:           spec.ep.RSMR,
				RSNR:           spec.ep.RSNR,
				MAT:            norm.MAT,
				MWMT:           norm.MWMT,
				MCMT:           norm.MCMT,
				MAP:            norm.MAP,
				MSP:            norm.MSP,
				PAS:            norm.PAS,
				CMD:            norm.CMD,
				PPT05:          norm.PPT05,
				PPT06:          norm.PPT06,
				PPT07:          norm.PPT07,
				PPT08:          norm.PPT08,
				PPT09:          norm.PPT09,
				PPTAt:          norm.PPTAt,
				PPTWt:          norm.PPTWt,
				DD5:            norm.DD5,
				NFFD:           norm.NFFD,
				MoistMonths:    tr.MoistMonths,
				DormantDeficit: tr.DormantDeficit,
				AspectFactor:   tr.AspectFactor,
				CMDTopo:        tr.CMDTopo,
				CMDSite:        tr.CMDSite,
				ASMR:           tr.ASMR,
				ComputedAt:     norm.ComputedAt,
			})
		}

		overallRel, overallAct := buildEdatopicMatrices(plotRows)

		groupMode := strings.ToLower(strings.TrimSpace(req.GroupBy))
		if groupMode == "" {
			if owner.Selection.SU != "" && owner.Selection.SU != "None" {
				groupMode = "site_unit"
			} else {
				groupMode = "bgc"
			}
		}

		grouped := make(map[string][]PlotClimateRow)
		for _, pr := range plotRows {
			key := pr.SiteUnit
			if groupMode == "bgc" {
				key = pr.BGC
			}
			if strings.TrimSpace(key) == "" {
				key = "Unassigned"
			}
			grouped[key] = append(grouped[key], pr)
		}

		keys := make([]string, 0, len(grouped))
		for k := range grouped {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		unitSummaries := make([]DualUnitEdatopicSummary, 0, len(keys))
		for _, k := range keys {
			sub := grouped[k]
			relG, actG := buildEdatopicMatrices(sub)
			var matSum, mapSum, cmdSum float64
			var asmrSum, shiftSum float64
			asmrCnt, shiftCnt := 0, 0
			bgcSample, suSample := "", ""
			for _, r := range sub {
				matSum += r.MAT
				mapSum += r.MAP
				cmdSum += r.CMD
				if bgcSample == "" && r.BGC != "" {
					bgcSample = r.BGC
				}
				if suSample == "" && r.SiteUnit != "" {
					suSample = r.SiteUnit
				}
				if r.ASMR != nil {
					asmrSum += *r.ASMR
					asmrCnt++
					if rDig, ok := parseRSMRDigit(r.RSMR); ok {
						shiftSum += (*r.ASMR - float64(rDig))
						shiftCnt++
					}
				}
			}
			n := float64(len(sub))
			var meanAsmr, meanShift *float64
			if asmrCnt > 0 {
				v := math.Round((asmrSum/float64(asmrCnt))*100.0) / 100.0
				meanAsmr = &v
			}
			if shiftCnt > 0 {
				v := math.Round((shiftSum/float64(shiftCnt))*100.0) / 100.0
				meanShift = &v
			}
			unitSummaries = append(unitSummaries, DualUnitEdatopicSummary{
				GroupKey:     k,
				BGC:          bgcSample,
				SiteUnit:     suSample,
				PlotCount:    len(sub),
				MeanMAT:      math.Round((matSum/n)*100.0) / 100.0,
				MeanMAP:      math.Round((mapSum/n)*10.0) / 10.0,
				MeanCMD:      math.Round((cmdSum/n)*10.0) / 10.0,
				MeanASMR:     meanAsmr,
				MeanShift:    meanShift,
				RelativeGrid: relG,
				ActualGrid:   actG,
			})
		}

		return &ClimateEdatopicResult{
			ContextID:          contextID,
			Project:            owner.Selection.Project,
			ActiveSU:           owner.Selection.SU,
			RefPeriod:          refPeriod,
			TotalScopedPlots:   len(envPlots),
			CoordinatizedPlots: coordinatized,
			SkippedNoCoords:    skippedNoCoords,
			CacheHits:          cacheHits,
			CacheMisses:        cacheMisses,
			SourceSummary:      sourceSummary,
			OverallRelative:    overallRel,
			OverallActual:      overallAct,
			UnitSummaries:      unitSummaries,
			Plots:              plotRows,
		}, nil
	})
}

type plotClimateSpec struct {
	ep   scopedEnvPlot
	hash string
	hit  bool
}

func upsertPlotClimateRowsAtomic(
	ctx context.Context,
	db *sql.DB,
	specs []plotClimateSpec,
	normByHash map[string]cachedNormalRecord,
	refPeriod string,
) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin immediate transaction for __VPRO_PlotClimate: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	if _, err := conn.ExecContext(ctx, plotClimateDDL); err != nil {
		return err
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)
	for _, s := range specs {
		if s.hit {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		norm, ok := normByHash[s.hash]
		if !ok {
			continue
		}
		compAt := norm.ComputedAt
		if compAt == "" {
			compAt = nowISO
		}

		// Literal PlotNumber check to prevent SQLite numeric affinity coercion on leading-zero plots
		var existing int
		if err := conn.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM main."__VPRO_PlotClimate" WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
			s.ep.PlotNumber,
		).Scan(&existing); err != nil {
			return err
		}
		if existing > 0 {
			_, err = conn.ExecContext(
				ctx,
				`UPDATE main."__VPRO_PlotClimate"
				    SET "CoordElevHash" = ?, "RefPeriod" = ?, "Source" = ?,
				        "MAT" = ?, "MWMT" = ?, "MCMT" = ?, "MAP" = ?, "MSP" = ?, "PAS" = ?, "CMD" = ?,
				        "PPT_05" = ?, "PPT_06" = ?, "PPT_07" = ?, "PPT_08" = ?, "PPT_09" = ?,
				        "PPT_at" = ?, "PPT_wt" = ?, "DD5" = ?, "NFFD" = ?, "ComputedAt" = ?
				  WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
				s.hash, refPeriod, norm.Source,
				norm.MAT, norm.MWMT, norm.MCMT, norm.MAP, norm.MSP, norm.PAS, norm.CMD,
				norm.PPT05, norm.PPT06, norm.PPT07, norm.PPT08, norm.PPT09,
				norm.PPTAt, norm.PPTWt, norm.DD5, norm.NFFD, compAt,
				s.ep.PlotNumber,
			)
		} else {
			_, err = conn.ExecContext(
				ctx,
				`INSERT INTO main."__VPRO_PlotClimate" (
					"PlotNumber", "CoordElevHash", "RefPeriod", "Source",
					"MAT", "MWMT", "MCMT", "MAP", "MSP", "PAS", "CMD",
					"PPT_05", "PPT_06", "PPT_07", "PPT_08", "PPT_09",
					"PPT_at", "PPT_wt", "DD5", "NFFD", "ComputedAt"
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				s.ep.PlotNumber, s.hash, refPeriod, norm.Source,
				norm.MAT, norm.MWMT, norm.MCMT, norm.MAP, norm.MSP, norm.PAS, norm.CMD,
				norm.PPT05, norm.PPT06, norm.PPT07, norm.PPT08, norm.PPT09,
				norm.PPTAt, norm.PPTWt, norm.DD5, norm.NFFD, compAt,
			)
		}
		if err != nil {
			return err
		}
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
