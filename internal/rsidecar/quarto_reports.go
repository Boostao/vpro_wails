package rsidecar

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// QuartoVegSummaryRow represents one formatted species row in a published BEC unit/BGC guide.
type QuartoVegSummaryRow struct {
	Layer          string  `json:"layer"`
	Species        string  `json:"species"`
	ScientificName string  `json:"scientificName"`
	EnglishName    string  `json:"englishName"`
	ConstancyPct   float64 `json:"constancyPct"`
	ConstancyClass string  `json:"constancyClass"`
	MeanCover      float64 `json:"meanCover"`
}

// AspectRoseSector holds plot count for one of the 8 cardinal/intercardinal compass octants.
type AspectRoseSector struct {
	Sector string `json:"sector"`
	Count  int    `json:"count"`
}

// FiveNumberBoxSummary holds five-number summary statistics for SVG boxplot rendering.
type FiveNumberBoxSummary struct {
	N      int     `json:"n"`
	Min    float64 `json:"min"`
	Q1     float64 `json:"q1"`
	Median float64 `json:"median"`
	Q3     float64 `json:"q3"`
	Max    float64 `json:"max"`
}

// QuartoGuideRequest configures single-command Quarto BEC unit or BGC subzone HTML/PDF guide publication.
type QuartoGuideRequest struct {
	GuideType       string  `json:"guideType"`       // "single_unit" (default) | "bgc_unit"
	Title           string  `json:"title"`           // Optional custom guide title
	TargetUnit      string  `json:"targetUnit"`      // Optional SiteUnit filter (for single_unit)
	BGCFilter       string  `json:"bgcFilter"`       // Optional BGC filter (for bgc_unit or single_unit)
	ConceptNotes    string  `json:"conceptNotes"`    // Authored ecological concept markdown/notes
	RefPeriod       string  `json:"refPeriod"`       // Climate reference period ("1961_1990" | "1991_2020")
	MinConstancyPct float64 `json:"minConstancyPct"` // Minimum species constancy % threshold (default 20%)
	UseActiveSU     bool    `json:"useActiveSu"`
	ApplyLump       bool    `json:"applyLump"`
	IncludePDF      bool    `json:"includePdf"`
	DestinationPath string  `json:"destinationPath"` // Target .html path (optional; defaults to <ProjectDir>/exports/<slug>.html)
}

// QuartoGuideResult reports the atomically published HTML/PDF guide artifacts and summary preview.
type QuartoGuideResult struct {
	ContextID       string                `json:"contextId"`
	GuideType       string                `json:"guideType"`
	Title           string                `json:"title"`
	TargetUnit      string                `json:"targetUnit"`
	BGCLabel        string                `json:"bgcLabel"`
	PlotCount       int                   `json:"plotCount"`
	SpeciesRowCount int                   `json:"speciesRowCount"`
	RendererUsed    string                `json:"rendererUsed"` // "quarto" | "rmarkdown_fallback"
	HTMLPath        string                `json:"htmlPath"`
	HTMLSizeBytes   int64                 `json:"htmlSizeBytes"`
	HTMLSHA256      string                `json:"htmlSha256"`
	PDFPath         string                `json:"pdfPath,omitempty"`
	PDFStatus       string                `json:"pdfStatus,omitempty"`
	HTMLPreview     string                `json:"htmlPreview"`
	VegetationTable []QuartoVegSummaryRow `json:"vegetationTable"`
}

func constancyRomanClass(pct float64) string {
	switch {
	case pct > 80.0:
		return "V"
	case pct > 60.0:
		return "IV"
	case pct > 40.0:
		return "III"
	case pct > 20.0:
		return "II"
	default:
		return "I"
	}
}

func computeFiveNumberSummary(vals []float64) *FiveNumberBoxSummary {
	clean := make([]float64, 0, len(vals))
	for _, v := range vals {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			clean = append(clean, v)
		}
	}
	if len(clean) == 0 {
		return &FiveNumberBoxSummary{N: 0}
	}
	sort.Float64s(clean)
	quantile := func(p float64) float64 {
		if len(clean) == 1 {
			return clean[0]
		}
		pos := p * float64(len(clean)-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if lo == hi {
			return clean[lo]
		}
		frac := pos - float64(lo)
		return clean[lo]*(1.0-frac) + clean[hi]*frac
	}
	round1 := func(x float64) float64 { return math.Round(x*10.0) / 10.0 }
	return &FiveNumberBoxSummary{
		N:      len(clean),
		Min:    round1(clean[0]),
		Q1:     round1(quantile(0.25)),
		Median: round1(quantile(0.50)),
		Q3:     round1(quantile(0.75)),
		Max:    round1(clean[len(clean)-1]),
	}
}

func computeAspectRose(aspects []float64) []AspectRoseSector {
	labels := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	counts := make([]int, 8)
	for _, a := range aspects {
		if math.IsNaN(a) || math.IsInf(a, 0) || a < 0 || a > 360 || a == 999 {
			continue
		}
		norm := math.Mod(a+360.0, 360.0)
		idx := int(math.Floor((norm+22.5)/45.0)) % 8
		counts[idx]++
	}
	out := make([]AspectRoseSector, 8)
	for i, lbl := range labels {
		out[i] = AspectRoseSector{Sector: lbl, Count: counts[i]}
	}
	return out
}

func combineStratumCovers(vals ...float64) float64 {
	prod := 1.0
	anyPos := false
	for _, v := range vals {
		if math.IsNaN(v) || v <= 0 {
			continue
		}
		clamped := math.Min(99.9, v)
		prod *= (1.0 - clamped/100.0)
		anyPos = true
	}
	if !anyPos {
		return 0.0
	}
	return 100.0 * (1.0 - prod)
}

func buildQuartoVegSummary(
	ctx context.Context,
	owner *vprocore.SqliteContext,
	plotNumbers []string,
	applyLump bool,
	minConstancyPct float64,
) ([]QuartoVegSummaryRow, error) {
	if len(plotNumbers) == 0 {
		return []QuartoVegSummaryRow{}, nil
	}
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

	vegSchema := "project"
	var projVegExists int
	_ = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, owner.Selection.Project+"_Veg").Scan(&projVegExists)
	if projVegExists == 0 {
		vegSchema = "main"
	}

	// Optional lump map
	lumpMap := make(map[string]string)
	if applyLump {
		lumpTableName := owner.Selection.Project + "_Lump"
		var lumpExists int
		_ = tx.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.sqlite_master WHERE type='table' AND name = ? COLLATE NOCASE`, vegSchema), lumpTableName).Scan(&lumpExists)
		if lumpExists > 0 {
			lRows, lErr := tx.QueryContext(ctx, fmt.Sprintf(
				`SELECT UPPER(TRIM(CAST("SppCode" AS TEXT))), UPPER(TRIM(CAST("LumpCode" AS TEXT))), COALESCE("Use", 0) FROM %s.%s`,
				vegSchema, vprocore.QuoteHeaderIdentifier(lumpTableName),
			))
			if lErr == nil {
				for lRows.Next() {
					var spp, lump string
					var useFlag int
					if err := lRows.Scan(&spp, &lump, &useFlag); err == nil && useFlag != 0 && spp != "" && lump != "" {
						lumpMap[spp] = lump
					}
				}
				lRows.Close()
			}
		}
	}

	// Load VLists species names
	type sppNames struct {
		sci string
		eng string
	}
	nameMap := make(map[string]sppNames)
	sRows, sErr := tx.QueryContext(ctx, `
		SELECT UPPER(TRIM(CAST("Code" AS TEXT))),
		       COALESCE(CAST("ScientificName" AS TEXT), ''),
		       COALESCE(CAST("EnglishName" AS TEXT), '')
		  FROM VLists."USysAllSpecs"
		 WHERE COALESCE("Codetype", 'U') IN ('U', 'D')
	`)
	if sErr == nil {
		for sRows.Next() {
			var c, sci, eng string
			if err := sRows.Scan(&c, &sci, &eng); err == nil && c != "" {
				if _, exists := nameMap[c]; !exists {
					nameMap[c] = sppNames{sci: sci, eng: eng}
				}
			}
		}
		sRows.Close()
	}

	allowed := make(map[string]bool, len(plotNumbers))
	for _, p := range plotNumbers {
		allowed[p] = true
	}

	vegQuery := fmt.Sprintf(
		`SELECT CAST("PlotNumber" AS TEXT), UPPER(TRIM(CAST("Species" AS TEXT))),
		        COALESCE("Cover1",0), COALESCE("Cover2",0), COALESCE("Cover3",0), COALESCE("TotalA",0),
		        COALESCE("Cover4",0), COALESCE("Cover5",0), COALESCE("Cover5a",0), COALESCE("Cover5b",0), COALESCE("Cover5c",0), COALESCE("TotalB",0),
		        COALESCE("Cover6",0), COALESCE("Cover7",0), COALESCE("Cover8",0), COALESCE("Cover9",0), COALESCE("Cover10",0)
		   FROM %s.%s
		  WHERE typeof("PlotNumber")='text' AND TRIM(COALESCE("Species",'')) <> ''`,
		vegSchema, vprocore.QuoteHeaderIdentifier(owner.Selection.Project+"_Veg"),
	)
	vRows, err := tx.QueryContext(ctx, vegQuery)
	if err != nil {
		return nil, err
	}
	defer vRows.Close()

	// Aggregate cover per (LayerGroup, Species, PlotNumber)
	type cellKey struct {
		layerOrder int
		layerLabel string
		species    string
	}
	plotLayerCover := make(map[cellKey]map[string]float64)
	addCover := func(ord int, label, spp, plot string, cov float64) {
		if cov <= 0 {
			return
		}
		k := cellKey{layerOrder: ord, layerLabel: label, species: spp}
		mp := plotLayerCover[k]
		if mp == nil {
			mp = make(map[string]float64)
			plotLayerCover[k] = mp
		}
		mp[plot] = combineStratumCovers(mp[plot], cov)
	}

	for vRows.Next() {
		var plot, spp string
		var c1, c2, c3, ta, c4, c5, c5a, c5b, c5c, tb, c6, c7, c8, c9, c10 sql.NullFloat64
		if err := vRows.Scan(&plot, &spp, &c1, &c2, &c3, &ta, &c4, &c5, &c5a, &c5b, &c5c, &tb, &c6, &c7, &c8, &c9, &c10); err != nil {
			return nil, err
		}
		if !allowed[plot] || spp == "" {
			continue
		}
		if folded, ok := lumpMap[spp]; ok {
			spp = folded
		}
		treeCov := ta.Float64
		if treeCov <= 0 {
			treeCov = combineStratumCovers(c1.Float64, c2.Float64, c3.Float64)
		}
		shrubCov := tb.Float64
		if shrubCov <= 0 {
			shrubCov = combineStratumCovers(c4.Float64, c5.Float64, c5a.Float64, c5b.Float64, c5c.Float64)
		}
		herbCov := combineStratumCovers(c6.Float64, c8.Float64, c9.Float64, c10.Float64)
		mossCov := c7.Float64

		addCover(1, "Tree (A)", spp, plot, treeCov)
		addCover(2, "Shrub (B)", spp, plot, shrubCov)
		addCover(3, "Herb (C)", spp, plot, herbCov)
		addCover(4, "Moss/Lichen (D)", spp, plot, mossCov)
	}
	if err := vRows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	nTotal := float64(len(plotNumbers))
	type sortableRow struct {
		order int
		row   QuartoVegSummaryRow
	}
	var collected []sortableRow
	for k, mp := range plotLayerCover {
		presCnt := len(mp)
		constancy := (float64(presCnt) / nTotal) * 100.0
		if constancy < minConstancyPct {
			continue
		}
		covSum := 0.0
		for _, cv := range mp {
			covSum += cv
		}
		meanCov := covSum / float64(presCnt)
		nm := nameMap[k.species]
		sci := nm.sci
		if sci == "" {
			sci = k.species
		}
		collected = append(collected, sortableRow{
			order: k.layerOrder,
			row: QuartoVegSummaryRow{
				Layer:          k.layerLabel,
				Species:        k.species,
				ScientificName: sci,
				EnglishName:    nm.eng,
				ConstancyPct:   math.Round(constancy*10.0) / 10.0,
				ConstancyClass: constancyRomanClass(constancy),
				MeanCover:      math.Round(meanCov*100.0) / 100.0,
			},
		})
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].order != collected[j].order {
			return collected[i].order < collected[j].order
		}
		if collected[i].row.ConstancyPct != collected[j].row.ConstancyPct {
			return collected[i].row.ConstancyPct > collected[j].row.ConstancyPct
		}
		if collected[i].row.MeanCover != collected[j].row.MeanCover {
			return collected[i].row.MeanCover > collected[j].row.MeanCover
		}
		return collected[i].row.Species < collected[j].row.Species
	})

	out := make([]QuartoVegSummaryRow, len(collected))
	for i, item := range collected {
		out[i] = item.row
	}
	return out, nil
}

func sanitizeSlug(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		return "bec_guide"
	}
	return res
}

func publishBytesAtomically(destPath string, content []byte) (int64, string, error) {
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return 0, "", fmt.Errorf("resolve destination path %q: %w", destPath, err)
	}
	parentDir := filepath.Dir(absDest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return 0, "", fmt.Errorf("create destination directory %q: %w", parentDir, err)
	}

	var randTok [6]byte
	_, _ = rand.Read(randTok[:])
	stageFile := filepath.Join(parentDir, fmt.Sprintf(".%s.stage-%s", filepath.Base(absDest), hex.EncodeToString(randTok[:])))
	f, err := os.OpenFile(stageFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return 0, "", fmt.Errorf("open atomic staging file %q: %w", stageFile, err)
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = os.Remove(stageFile)
		}
	}()

	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return 0, "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return 0, "", err
	}
	if err := f.Close(); err != nil {
		return 0, "", err
	}
	if err := os.Rename(stageFile, absDest); err != nil {
		return 0, "", fmt.Errorf("atomic rename to %q: %w", absDest, err)
	}
	cleanupStage = false

	sum := sha256.Sum256(content)
	return int64(len(content)), hex.EncodeToString(sum[:]), nil
}

// RenderQuartoGuide compiles embedded parameterized Quarto templates (single_unit_guide.qmd or bgc_unit_guide.qmd)
// combining formatted Veg/Env/Climate tables, Dual (rSMR vs aSMR) Edatopic SVG heatmaps, Aspect Rose, and Boxplots,
// and publishes standalone HTML (plus optional PDF) atomically to disk.
func RenderQuartoGuide(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req QuartoGuideRequest,
	opts ...RuntimeOptions,
) (*QuartoGuideResult, error) {
	if ctx == nil {
		return nil, errors.New("RenderQuartoGuide requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("context service is unavailable")
	}

	guideType := strings.ToLower(strings.TrimSpace(req.GuideType))
	if guideType == "" {
		guideType = "single_unit"
	}
	if guideType != "single_unit" && guideType != "bgc_unit" {
		return nil, fmt.Errorf("unsupported guideType %q (expected single_unit or bgc_unit)", req.GuideType)
	}
	if !utf8.ValidString(req.Title) || !utf8.ValidString(req.ConceptNotes) || !utf8.ValidString(req.TargetUnit) || !utf8.ValidString(req.BGCFilter) {
		return nil, errors.New("guide text fields must be valid UTF-8")
	}
	if len(utf16.Encode([]rune(req.Title))) > 200 {
		return nil, errors.New("guide title exceeds 200 UTF-16 code units")
	}
	if len(utf16.Encode([]rune(req.ConceptNotes))) > 16000 {
		return nil, errors.New("concept notes exceed 16000 UTF-16 code units")
	}

	minConstancy := req.MinConstancyPct
	if minConstancy <= 0 {
		minConstancy = 20.0
	}
	if minConstancy > 100.0 {
		minConstancy = 100.0
	}

	cfg := effectiveOptions(opts...)
	// Verify Rscript availability upfront for informative error messaging
	rscriptBin, _, rErr := resolveRscriptBinary(cfg)
	if rErr != nil {
		return nil, fmt.Errorf("R runtime unavailable for Quarto guide generation: %w", rErr)
	}

	// Step 1: Compute/fetch cached __VPRO_PlotClimate & Dual Edatopic Grids for the requested scope
	climReq := ClimateEdatopicRequest{
		RefPeriod:      req.RefPeriod,
		GroupBy:        "site_unit",
		BGCFilter:      req.BGCFilter,
		SiteUnitFilter: "",
		UseActiveSU:    req.UseActiveSU,
	}
	if guideType == "single_unit" && strings.TrimSpace(req.TargetUnit) != "" {
		climReq.SiteUnitFilter = strings.TrimSpace(req.TargetUnit)
	}
	climRes, err := ComputePlotClimateAndDualEdatopic(ctx, contexts, contextID, climReq, cfg)
	if err != nil {
		return nil, err
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*QuartoGuideResult, error) {
		owner := plots.Projects.Sqlite
		unitFilter := ""
		if guideType == "single_unit" {
			unitFilter = strings.TrimSpace(req.TargetUnit)
		}
		envPlots, err := readScopedEnvPlots(ctx, owner, req.UseActiveSU, req.BGCFilter, unitFilter, nil)
		if err != nil {
			return nil, err
		}
		if len(envPlots) == 0 {
			return nil, errors.New("no plots matched the requested Quarto guide filter")
		}

		plotNums := make([]string, 0, len(envPlots))
		var elevs, slopes, aspects []float64
		bgcCounts := make(map[string]int)
		unitCounts := make(map[string]int)
		for _, ep := range envPlots {
			plotNums = append(plotNums, ep.PlotNumber)
			if ep.Elevation != nil {
				elevs = append(elevs, *ep.Elevation)
			}
			if ep.Slope != nil {
				slopes = append(slopes, *ep.Slope)
			}
			if ep.Aspect != nil {
				aspects = append(aspects, *ep.Aspect)
			}
			if ep.BGC != "" {
				bgcCounts[ep.BGC]++
			}
			if ep.SiteUnit != "" {
				unitCounts[ep.SiteUnit]++
			}
		}

		vegRows, err := buildQuartoVegSummary(ctx, owner, plotNums, req.ApplyLump, minConstancy)
		if err != nil {
			return nil, err
		}

		// Determine modal BGC & TargetUnit labels
		bgcLabel := strings.TrimSpace(req.BGCFilter)
		if bgcLabel == "" {
			bestCnt := 0
			for b, cnt := range bgcCounts {
				if cnt > bestCnt || (cnt == bestCnt && b < bgcLabel) {
					bestCnt = cnt
					bgcLabel = b
				}
			}
		}
		if bgcLabel == "" {
			bgcLabel = "All BGC Units"
		}

		targetUnitLabel := strings.TrimSpace(req.TargetUnit)
		if targetUnitLabel == "" {
			if guideType == "bgc_unit" {
				targetUnitLabel = bgcLabel
			} else {
				bestCnt := 0
				for u, cnt := range unitCounts {
					if cnt > bestCnt || (cnt == bestCnt && u < targetUnitLabel) {
						bestCnt = cnt
						targetUnitLabel = u
					}
				}
				if targetUnitLabel == "" {
					targetUnitLabel = owner.Selection.Project
				}
			}
		}

		title := strings.TrimSpace(req.Title)
		if title == "" {
			if guideType == "bgc_unit" {
				title = fmt.Sprintf("%s Biogeoclimatic Unit Synthesis", bgcLabel)
			} else {
				title = fmt.Sprintf("%s — Ecosystem Site Unit Guide", targetUnitLabel)
			}
		}
		conceptNotes := strings.TrimSpace(req.ConceptNotes)
		if conceptNotes == "" {
			conceptNotes = fmt.Sprintf(
				"Synthesized BEC ecosystem description for %s (%s) across %d sampled plots using %s climate normals and Ministry CMD -> aSMR edatopic calibration.",
				targetUnitLabel, bgcLabel, len(envPlots), climRes.RefPeriod,
			)
		}

		var climateSummary any
		if len(climRes.Plots) > 0 {
			var matS, mapS, mspS, pasS, cmdS float64
			for _, pr := range climRes.Plots {
				matS += pr.MAT
				mapS += pr.MAP
				mspS += pr.MSP
				pasS += pr.PAS
				cmdS += pr.CMD
			}
			n := float64(len(climRes.Plots))
			climateSummary = map[string]any{
				"meanMat": math.Round((matS/n)*10.0) / 10.0,
				"meanMap": math.Round(mapS / n),
				"meanMsp": math.Round(mspS / n),
				"meanPas": math.Round(pasS / n),
				"meanCmd": math.Round(cmdS / n),
			}
		}

		// Unit breakdown for bgc_unit guide
		var unitBreakdown []map[string]any
		if guideType == "bgc_unit" {
			elevByUnit := make(map[string][]float64)
			for _, ep := range envPlots {
				u := ep.SiteUnit
				if u == "" {
					u = "Unassigned"
				}
				if ep.Elevation != nil {
					elevByUnit[u] = append(elevByUnit[u], *ep.Elevation)
				}
			}
			cmdSiteByUnit := make(map[string][]float64)
			for _, pr := range climRes.Plots {
				u := pr.SiteUnit
				if u == "" {
					u = "Unassigned"
				}
				if pr.CMDSite != nil {
					cmdSiteByUnit[u] = append(cmdSiteByUnit[u], *pr.CMDSite)
				}
			}
			for _, us := range climRes.UnitSummaries {
				meanElevStr := "—"
				if evs := elevByUnit[us.GroupKey]; len(evs) > 0 {
					s := 0.0
					for _, v := range evs {
						s += v
					}
					meanElevStr = fmt.Sprintf("%.0f", s/float64(len(evs)))
				}
				meanAsmrStr := "—"
				if us.MeanASMR != nil {
					meanAsmrStr = fmt.Sprintf("%.2f", *us.MeanASMR)
				}
				meanCmdSiteStr := "—"
				if cs := cmdSiteByUnit[us.GroupKey]; len(cs) > 0 {
					s := 0.0
					for _, v := range cs {
						s += v
					}
					meanCmdSiteStr = fmt.Sprintf("%.0f mm", s/float64(len(cs)))
				}
				modalEda := us.RelativeGrid.ModalCell
				if modalEda == "" {
					modalEda = "—"
				}
				unitBreakdown = append(unitBreakdown, map[string]any{
					"siteUnit":      us.GroupKey,
					"plotCount":     us.PlotCount,
					"modalEdatope":  modalEda,
					"meanAsmr":      meanAsmrStr,
					"meanElevation": meanElevStr,
					"meanCmdSite":   meanCmdSiteStr,
				})
			}
		}

		payload := map[string]any{
			"guideType":       guideType,
			"title":           title,
			"project":         owner.Selection.Project,
			"targetUnit":      targetUnitLabel,
			"bgcLabel":        bgcLabel,
			"plotCount":       len(envPlots),
			"generatedAt":     time.Now().UTC().Format("2006-01-02 15:04 UTC"),
			"conceptNotes":    conceptNotes,
			"climateSummary":  climateSummary,
			"relativeGrid":    climRes.OverallRelative,
			"actualGrid":      climRes.OverallActual,
			"aspectRose":      computeAspectRose(aspects),
			"elevationStats":  computeFiveNumberSummary(elevs),
			"slopeStats":      computeFiveNumberSummary(slopes),
			"vegetationTable": vegRows,
			"unitBreakdown":   unitBreakdown,
		}

		projectDir := filepath.Dir(owner.Selection.ProjectPath)
		baseWork := strings.TrimSpace(cfg.WorkRootDir)
		if baseWork == "" {
			baseWork = projectDir
		}
		runDir, tmpDir, cleanup, err := prepareIsolatedRunDir(baseWork)
		if err != nil {
			return nil, err
		}
		defer cleanup()

		templateName := "single_unit_guide.qmd"
		if guideType == "bgc_unit" {
			templateName = "bgc_unit_guide.qmd"
		}
		qmdPath, err := stageEmbeddedAsset(runDir, templateName)
		if err != nil {
			return nil, err
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadPath := filepath.Join(runDir, "guide_payload.json")
		if err := os.WriteFile(payloadPath, payloadBytes, 0600); err != nil {
			return nil, err
		}

		renderedHTMLName := "rendered_guide.html"
		renderedHTMLPath := filepath.Join(runDir, renderedHTMLName)
		childEnv := buildIsolatedChildEnv(tmpDir)
		// Ensure Quarto finds the exact Rscript binary discovered by VPro
		rBinDir := filepath.Dir(rscriptBin)
		childEnv = append(childEnv, "QUARTO_R="+rscriptBin)
		for i, kv := range childEnv {
			if strings.HasPrefix(kv, "PATH=") {
				childEnv[i] = "PATH=" + rBinDir + string(os.PathListSeparator) + strings.TrimPrefix(kv, "PATH=")
				break
			}
		}

		quartoBin := resolveQuartoBinary(cfg)
		rendererUsed := ""

		if quartoBin != "" {
			qCmd := exec.CommandContext(
				ctx,
				quartoBin,
				"render",
				filepath.Base(qmdPath),
				"--to", "html",
				"-M", "embed-resources:true",
				"-P", "payload_json:guide_payload.json",
				"--output", renderedHTMLName,
			)
			qCmd.Dir = runDir
			qCmd.Env = childEnv
			qCmd.WaitDelay = 2 * time.Second
			var qOut bytes.Buffer
			qCmd.Stdout = &qOut
			qCmd.Stderr = &qOut
			if qErr := qCmd.Run(); qErr == nil {
				rendererUsed = "quarto"
			} else if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
		}

		if rendererUsed == "" {
			fallbackScript, err := stageEmbeddedAsset(runDir, "render_guide_fallback.R")
			if err != nil {
				return nil, err
			}
			rCmd := exec.CommandContext(ctx, rscriptBin, "--vanilla", fallbackScript, qmdPath, payloadPath, renderedHTMLPath)
			rCmd.Dir = runDir
			rCmd.Env = childEnv
			rCmd.WaitDelay = 2 * time.Second
			var rOut bytes.Buffer
			rCmd.Stdout = &rOut
			rCmd.Stderr = &rOut
			if err := rCmd.Run(); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return nil, ctxErr
				}
				msg := strings.TrimSpace(rOut.String())
				if len(msg) > 600 {
					msg = msg[:600] + "..."
				}
				return nil, fmt.Errorf("Quarto/RMarkdown guide rendering failed (%v): %s", err, msg)
			}
			rendererUsed = "rmarkdown_fallback"
		}

		htmlBytes, err := os.ReadFile(renderedHTMLPath)
		if err != nil || len(htmlBytes) == 0 {
			return nil, fmt.Errorf("rendered HTML guide was not produced: %w", err)
		}

		destHTML := strings.TrimSpace(req.DestinationPath)
		if destHTML == "" {
			slug := sanitizeSlug(owner.Selection.Project + "_" + guideType + "_" + targetUnitLabel)
			destHTML = filepath.Join(projectDir, "exports", slug+".html")
		} else if !strings.EqualFold(filepath.Ext(destHTML), ".html") {
			destHTML += ".html"
		}
		absHTML, err := filepath.Abs(destHTML)
		if err != nil {
			return nil, err
		}

		htmlSize, htmlSHA, err := publishBytesAtomically(absHTML, htmlBytes)
		if err != nil {
			return nil, err
		}

		pdfPath := ""
		pdfStatus := ""
		if req.IncludePDF {
			if quartoBin != "" {
				renderedPDFName := "rendered_guide.pdf"
				renderedPDFPath := filepath.Join(runDir, renderedPDFName)
				pCmd := exec.CommandContext(
					ctx,
					quartoBin,
					"render",
					filepath.Base(qmdPath),
					"--to", "pdf",
					"-P", "payload_json:guide_payload.json",
					"--output", renderedPDFName,
				)
				pCmd.Dir = runDir
				pCmd.Env = childEnv
				pCmd.WaitDelay = 2 * time.Second
				if pErr := pCmd.Run(); pErr == nil {
					if pdfBytes, readErr := os.ReadFile(renderedPDFPath); readErr == nil && len(pdfBytes) > 0 {
						targetPDF := strings.TrimSuffix(absHTML, filepath.Ext(absHTML)) + ".pdf"
						if _, _, pubErr := publishBytesAtomically(targetPDF, pdfBytes); pubErr == nil {
							pdfPath = targetPDF
							pdfStatus = "published"
						}
					}
				} else if ctxErr := ctx.Err(); ctxErr != nil {
					return nil, ctxErr
				}
			}
			if pdfPath == "" {
				pdfStatus = "tex_unavailable_print_html_ready"
			}
		}

		preview := string(htmlBytes)
		// Cap inline preview string to 250 KB if extraordinarily large while preserving full standalone HTML for normal guides
		if len(preview) > 256*1024 {
			preview = preview[:256*1024]
		}

		return &QuartoGuideResult{
			ContextID:       contextID,
			GuideType:       guideType,
			Title:           title,
			TargetUnit:      targetUnitLabel,
			BGCLabel:        bgcLabel,
			PlotCount:       len(envPlots),
			SpeciesRowCount: len(vegRows),
			RendererUsed:    rendererUsed,
			HTMLPath:        absHTML,
			HTMLSizeBytes:   htmlSize,
			HTMLSHA256:      htmlSHA,
			PDFPath:         pdfPath,
			PDFStatus:       pdfStatus,
			HTMLPreview:     preview,
			VegetationTable: vegRows,
		}, nil
	})
}
