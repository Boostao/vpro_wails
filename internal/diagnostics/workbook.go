package diagnostics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// ResolveDiagnosticWorkbookSheets validates Excel worksheet names (31 UTF-16 units, XML safety)
// and enforces case-insensitive uniqueness via strings.EqualFold matching excelize/v2 (§4.3).
func ResolveDiagnosticWorkbookSheets(rawNames []string) ([]string, error) {
	if len(rawNames) == 0 {
		return nil, errors.New("workbook requires at least one worksheet")
	}
	resolved := make([]string, 0, len(rawNames))
	for i, raw := range rawNames {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, fmt.Errorf("worksheet #%d name must not be empty", i+1)
		}
		clean, err := reportcore.EnvironmentWorkbookSheetName(trimmed, i+1)
		if err != nil {
			return nil, fmt.Errorf("worksheet %q: %w", raw, err)
		}
		for _, prev := range resolved {
			if strings.EqualFold(prev, clean) {
				return nil, fmt.Errorf("worksheet name collision between %q and %q (case-insensitive Excel match)", prev, clean)
			}
		}
		resolved = append(resolved, clean)
	}
	return resolved, nil
}

func primaryDiagFillKey(codes []string) string {
	if slices.Contains(codes, "d") && (slices.Contains(codes, "cd") || slices.Contains(codes, "dd")) {
		return "d_dom"
	}
	if slices.Contains(codes, "dd") {
		return "dd"
	}
	if slices.Contains(codes, "d") {
		return "d"
	}
	if slices.Contains(codes, "cd") {
		return "cd"
	}
	if slices.Contains(codes, "c") {
		return "c"
	}
	if slices.Contains(codes, "ic") {
		return "ic"
	}
	return ""
}

func setCellChecked(f *excelize.File, sheet string, col, row int, val any) error {
	coord, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	if str, ok := val.(string); ok {
		if err := reportcore.EnvironmentWorkbookText(str); err != nil {
			return err
		}
	}
	return f.SetCellValue(sheet, coord, val)
}

func styleRangeChecked(f *excelize.File, sheet string, c1, r1, c2, r2, styleID int) error {
	start, err := excelize.CoordinatesToCellName(c1, r1)
	if err != nil {
		return err
	}
	end, err := excelize.CoordinatesToCellName(c2, r2)
	if err != nil {
		return err
	}
	return f.SetCellStyle(sheet, start, end, styleID)
}

func encodeDiagnosticWorkbookBytes(
	ctx context.Context,
	snap *SynthesisSnapshot,
	req ExportDiagnosticWorkbookRequest,
	sheetNames []string,
) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", sheetNames[0]); err != nil {
		return nil, err
	}
	for _, name := range sheetNames[1:] {
		if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
	}

	// Shared BC Gov & BEC Diagnostic styles (#003366 Navy, #FCBA19 Gold,BEC differential fills)
	titleStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 13, Color: "#003366", Family: "Calibri"},
	})
	if err != nil {
		return nil, err
	}
	subtitleStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Italic: true, Size: 10, Color: "#334155", Family: "Calibri"},
	})
	if err != nil {
		return nil, err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "#FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#003366"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#FCBA19", Style: 2},
		},
	})
	if err != nil {
		return nil, err
	}
	headerLeftStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "#FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#003366"}},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "#FCBA19", Style: 2},
		},
	})
	if err != nil {
		return nil, err
	}
	sectionBannerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10, Color: "#003366", Family: "Calibri"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#E5EBF2"}},
		Border: []excelize.Border{
			{Type: "top", Color: "#94A3B8", Style: 1},
			{Type: "bottom", Color: "#94A3B8", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}
	centerCellStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	italicSciStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Italic: true, Size: 10, Family: "Calibri"},
	})
	if err != nil {
		return nil, err
	}

	makeDiagFill := func(hexColor string, bold bool) (int, error) {
		return f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: bold, Size: 10, Color: "#0F172A", Family: "Calibri"},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{hexColor}},
			Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
			Border: []excelize.Border{
				{Type: "left", Color: "#CBD5E1", Style: 1},
				{Type: "right", Color: "#CBD5E1", Style: 1},
				{Type: "top", Color: "#CBD5E1", Style: 1},
				{Type: "bottom", Color: "#CBD5E1", Style: 1},
			},
		})
	}
	diagStyles := map[string]int{}
	for k, spec := range map[string]struct {
		hex  string
		bold bool
	}{
		"d_dom": {"#FCBA19", true},  // Differential + Dominant / Constant Dominant (BC Gold)
		"dd":    {"#FDE68A", true},  // Dominant Differential
		"d":     {"#BBF7D0", true},  // Differential
		"cd":    {"#BFDBFE", true},  // Constant Dominant
		"c":     {"#E0F2FE", false}, // Constant
		"ic":    {"#F5EFE0", false}, // Important Companion
	} {
		stID, sErr := makeDiagFill(spec.hex, spec.bold)
		if sErr != nil {
			return nil, sErr
		}
		diagStyles[k] = stID
	}

	warnRowStyle, err := makeDiagFill("#FEF3C7", true)
	if err != nil {
		return nil, err
	}
	highSimStyle, err := makeDiagFill("#BFDBFE", true)
	if err != nil {
		return nil, err
	}
	medSimStyle, err := makeDiagFill("#E0F2FE", false)
	if err != nil {
		return nil, err
	}
	edatopicCoreStyle, err := makeDiagFill("#FCBA19", true)
	if err != nil {
		return nil, err
	}
	edatopicSubStyle, err := makeDiagFill("#FEF08A", false)
	if err != nil {
		return nil, err
	}

	// =========================================================================
	// SHEET 1: Formatted BEC Diagnostic Summary Sheet (Diagnostic10.xla compatible)
	// =========================================================================
	diagSheet := sheetNames[0]
	title := strings.TrimSpace(req.ReportTitle)
	if title == "" {
		suLabel := snap.SU
		if suLabel == "" || strings.EqualFold(suLabel, "None") {
			suLabel = "UserSiteUnit"
		}
		title = fmt.Sprintf("VPro BEC Diagnostic Summary Vegetation Report — %s (%s)", snap.Project, suLabel)
	}
	if err := setCellChecked(f, diagSheet, 1, 1, title); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, diagSheet, 1, 1, 1, 1, titleStyle)

	subMeta := fmt.Sprintf("Vegetation Table | Classified Plots: %d across %d Site Units | Significance Basis: %s",
		snap.ClassifiedPlotCount, len(snap.Units), snap.SignificanceBasis)
	if err := setCellChecked(f, diagSheet, 1, 2, subMeta); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, diagSheet, 1, 2, 1, 2, subtitleStyle)

	lumpComment := ""
	if snap.LumpTableUsed != "" {
		lumpComment = fmt.Sprintf("Vegetation was lumped using %s table (99%% cover cap)", snap.LumpTableUsed)
	}
	if err := setCellChecked(f, diagSheet, 1, 3, lumpComment); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, diagSheet, 1, 3, 1, 3, subtitleStyle)

	// Determine fixed columns matching V7mdlReportsShortVeg.DiagSpp + FormatDiagReport Row 5 header
	fixedHeaders := []string{"Lifeform", "Species", "ScientificName"}
	if req.IncludeCommonNames {
		fixedHeaders = append(fixedHeaders, "EnglishName")
	}
	fixedHeaders = append(fixedHeaders, "DiagnosticUnit", "Diagnostic")
	baseCols := len(fixedHeaders)

	// Row 4: nPlots per SiteUnit + BEC legend
	if err := setCellChecked(f, diagSheet, 2, 4, "nPlots"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, diagSheet, 1, 4, baseCols, 4, subtitleStyle)

	// Row 5: Column headers (3 columns per SiteUnit: Diagnostic10 Pres-Sig code, Presence I-V (%), Cover +..9 (%))
	for cIdx, h := range fixedHeaders {
		if err := setCellChecked(f, diagSheet, cIdx+1, 5, h); err != nil {
			return nil, err
		}
		st := headerLeftStyle
		if h == "Lifeform" || h == "Diagnostic" {
			st = headerStyle
		}
		_ = styleRangeChecked(f, diagSheet, cIdx+1, 5, cIdx+1, 5, st)
	}

	for uIdx, u := range snap.Units {
		colStart := baseCols + uIdx*3 + 1
		if err := setCellChecked(f, diagSheet, colStart, 4, u.PlotCount); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, diagSheet, colStart, 4, colStart+2, 4, centerCellStyle)

		if err := setCellChecked(f, diagSheet, colStart, 5, u.SiteUnit); err != nil {
			return nil, err
		}
		if err := setCellChecked(f, diagSheet, colStart+1, 5, u.SiteUnit+" Pres (I-V / %)"); err != nil {
			return nil, err
		}
		if err := setCellChecked(f, diagSheet, colStart+2, 5, u.SiteUnit+" Cov (+..9 / %)"); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, diagSheet, colStart, 5, colStart+2, 5, headerStyle)
	}
	totalDiagCols := max(baseCols+len(snap.Units)*3, baseCols)

	writeSpeciesRow := func(excelRow int, sp SynthesisSpeciesRow) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		lfVal := ""
		if sp.Lifeform != nil {
			lfVal = fmt.Sprintf("%d (%s)", *sp.Lifeform, sp.StratumGroup)
		}
		col := 1
		if err := setCellChecked(f, diagSheet, col, excelRow, lfVal); err != nil {
			return err
		}
		col++
		if err := setCellChecked(f, diagSheet, col, excelRow, sp.Species); err != nil {
			return err
		}
		col++
		sciCol := col
		if err := setCellChecked(f, diagSheet, col, excelRow, sp.ScientificName); err != nil {
			return err
		}
		_ = styleRangeChecked(f, diagSheet, sciCol, excelRow, sciCol, excelRow, italicSciStyle)
		col++
		if req.IncludeCommonNames {
			if err := setCellChecked(f, diagSheet, col, excelRow, sp.EnglishName); err != nil {
				return err
			}
			col++
		}
		if err := setCellChecked(f, diagSheet, col, excelRow, sp.DiagnosticUnit); err != nil {
			return err
		}
		_ = styleRangeChecked(f, diagSheet, col, excelRow, col, excelRow, centerCellStyle)
		col++
		if err := setCellChecked(f, diagSheet, col, excelRow, sp.Diagnosis); err != nil {
			return err
		}
		if fk := primaryDiagFillKey(sp.DiagnosticCodes); fk != "" {
			_ = styleRangeChecked(f, diagSheet, col, excelRow, col, excelRow, diagStyles[fk])
		} else {
			_ = styleRangeChecked(f, diagSheet, col, excelRow, col, excelRow, centerCellStyle)
		}

		for uIdx, cell := range sp.Cells {
			cStart := baseCols + uIdx*3 + 1
			if cell.PresentPlots > 0 {
				presDisplay := fmt.Sprintf("%s (%s%%)", cell.PresenceRoman, cell.PresencePctText)
				meanText := cell.MeanCoverAllText
				if snap.SignificanceBasis == "present_plots" {
					meanText = cell.MeanCoverPresentText
				}
				covDisplay := fmt.Sprintf("%s (%s%%)", cell.SignifClass, meanText)

				if err := setCellChecked(f, diagSheet, cStart, excelRow, cell.CrosstabCode); err != nil {
					return err
				}
				if err := setCellChecked(f, diagSheet, cStart+1, excelRow, presDisplay); err != nil {
					return err
				}
				if err := setCellChecked(f, diagSheet, cStart+2, excelRow, covDisplay); err != nil {
					return err
				}

				cellStyle := centerCellStyle
				if fk := primaryDiagFillKey(cell.CellDiagnosticCodes); fk != "" {
					cellStyle = diagStyles[fk]
				}
				_ = styleRangeChecked(f, diagSheet, cStart, excelRow, cStart+2, excelRow, cellStyle)
			}
		}
		return nil
	}

	curRow := 6
	diagByUnit := map[string][]SynthesisSpeciesRow{}
	var nonDiagSpecies []SynthesisSpeciesRow
	for _, sp := range snap.Species {
		if sp.DiagnosticUnit != "" {
			diagByUnit[sp.DiagnosticUnit] = append(diagByUnit[sp.DiagnosticUnit], sp)
		} else {
			nonDiagSpecies = append(nonDiagSpecies, sp)
		}
	}

	for _, u := range snap.Units {
		unitHeader := "~" + u.SiteUnit
		if u.LongName != "" {
			unitHeader = fmt.Sprintf("~%s — %s (n=%d)", u.SiteUnit, u.LongName, u.PlotCount)
		}
		if err := setCellChecked(f, diagSheet, 2, curRow, unitHeader); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, diagSheet, 1, curRow, totalDiagCols, curRow, sectionBannerStyle)
		curRow++

		for _, sp := range diagByUnit[u.SiteUnit] {
			if err := writeSpeciesRow(curRow, sp); err != nil {
				return nil, err
			}
			curRow++
		}
	}

	// Non-Diagnostic Species block (matching V7mdlReportsShortVeg.DiagSpp line 2239 & ShadeXlCells)
	if err := setCellChecked(f, diagSheet, 2, curRow, "Non-Diagnostic Species"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, diagSheet, 1, curRow, totalDiagCols, curRow, sectionBannerStyle)
	curRow++

	for _, sp := range nonDiagSpecies {
		if err := writeSpeciesRow(curRow, sp); err != nil {
			return nil, err
		}
		curRow++
	}

	_ = f.SetColWidth(diagSheet, "A", "B", 14)
	_ = f.SetColWidth(diagSheet, "C", "C", 26)
	if topLeft, err := excelize.CoordinatesToCellName(baseCols+1, 6); err == nil {
		_ = f.SetPanes(diagSheet, &excelize.Panes{
			Freeze:      true,
			XSplit:      baseCols,
			YSplit:      5,
			TopLeftCell: topLeft,
			ActivePane:  "bottomRight",
		})
	}

	// =========================================================================
	// SHEET 2: Pairwise Similarity Matrix Sheet
	// =========================================================================
	simSheet := sheetNames[1]
	if err := setCellChecked(f, simSheet, 1, 1, "Pairwise Unit Similarity Matrix (Sorensen / Bray-Curtis & BEC Differential Similarity)"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, simSheet, 1, 1, 1, 1, titleStyle)
	if err := setCellChecked(f, simSheet, 1, 2, "Formula: 2 * sum(min(MeanCover_u1, MeanCover_u2)) / sum(MeanCover_u1 + MeanCover_u2)"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, simSheet, 1, 2, 1, 2, subtitleStyle)

	if err := setCellChecked(f, simSheet, 1, 4, "SiteUnit"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, simSheet, 1, 4, 1, 4, headerLeftStyle)
	for j, u := range snap.Units {
		if err := setCellChecked(f, simSheet, j+2, 4, u.SiteUnit); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, simSheet, j+2, 4, j+2, 4, headerStyle)
	}

	for i, row := range snap.SimilarityMatrix {
		rIdx := 5 + i
		if err := setCellChecked(f, simSheet, 1, rIdx, row.Unit); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, simSheet, 1, rIdx, 1, rIdx, sectionBannerStyle)
		for j, cell := range row.Cells {
			if err := setCellChecked(f, simSheet, j+2, rIdx, cell.SimilarityPct); err != nil {
				return nil, err
			}
			st := centerCellStyle
			if i != j {
				if cell.SimilarityPct >= 70.0 {
					st = highSimStyle
				} else if cell.SimilarityPct >= 50.0 {
					st = medSimStyle
				}
			}
			_ = styleRangeChecked(f, simSheet, j+2, rIdx, j+2, rIdx, st)
		}
	}

	pairStartRow := 7 + len(snap.SimilarityMatrix)
	if err := setCellChecked(f, simSheet, 1, pairStartRow, "Pairwise BEC Differential Potential Summary (do_pairwise)"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, simSheet, 1, pairStartRow, 1, pairStartRow, titleStyle)

	pairHeaders := []string{
		"Unit1", "Unit2", "nPlots1", "nPlots2",
		"SorensenSim (%)", "BEC.sim.mean (%)", "BEC.sim.min (%)", "BEC.sim.max (%)",
		"SharedDiagPot", "DiffPts Unit1", "DiffPts Unit2", "Top Diff Taxa",
	}
	pairHeaderRow := pairStartRow + 1
	for cIdx, h := range pairHeaders {
		if err := setCellChecked(f, simSheet, cIdx+1, pairHeaderRow, h); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, simSheet, cIdx+1, pairHeaderRow, cIdx+1, pairHeaderRow, headerStyle)
	}
	for idx, comp := range snap.PairwiseComparisons {
		r := pairHeaderRow + 1 + idx
		topNames := make([]string, 0, min(5, len(comp.DifferentialTaxa)))
		for k := 0; k < len(comp.DifferentialTaxa) && k < 5; k++ {
			dt := comp.DifferentialTaxa[k]
			topNames = append(topNames, fmt.Sprintf("%s (%s)", dt.Species, dt.FavoursUnit))
		}
		vals := []any{
			comp.Unit1, comp.Unit2, comp.PlotsUnit1, comp.PlotsUnit2,
			comp.SorensenPct, RoundACE15(comp.BECSimMean*100.0, 1),
			RoundACE15(comp.BECSimMin*100.0, 1), RoundACE15(comp.BECSimMax*100.0, 1),
			comp.SharedDiagPotential, comp.DiffTotalUnit1, comp.DiffTotalUnit2,
			strings.Join(topNames, "; "),
		}
		for cIdx, v := range vals {
			if err := setCellChecked(f, simSheet, cIdx+1, r, v); err != nil {
				return nil, err
			}
		}
	}
	_ = f.SetColWidth(simSheet, "A", "L", 16)

	// =========================================================================
	// SHEET 3: Candidate Misplaced Plots (Outlier Queue) Sheet
	// =========================================================================
	outSheet := sheetNames[2]
	if err := setCellChecked(f, outSheet, 1, 1, "Candidate Misplaced Plots Queue (Leave-One-Out Centroid Similarity)"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, outSheet, 1, 1, 1, 1, titleStyle)
	if err := setCellChecked(f, outSheet, 1, 2, "Sorted by RivalSimilarity - OwnLOOSimilarity descending; top 5% anomalous and misplaced plots highlighted"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, outSheet, 1, 2, 1, 2, subtitleStyle)

	outHeaders := []string{
		"Rank", "PlotNumber", "AssignedUnit", "UnitPlots",
		"OwnLOOSim (%)", "NearestRivalUnit", "RivalSim (%)", "DeltaSim (%)",
		"Status", "rSMR", "rSNR", "Zone", "SubZone", "SiteSeries",
	}
	for cIdx, h := range outHeaders {
		if err := setCellChecked(f, outSheet, cIdx+1, 4, h); err != nil {
			return nil, err
		}
		_ = styleRangeChecked(f, outSheet, cIdx+1, 4, cIdx+1, 4, headerStyle)
	}

	for idx, cand := range snap.OutlierQueue {
		r := 5 + idx
		status := "Consistent"
		switch {
		case cand.Misplaced && cand.Top5PercentAnomaly:
			status = "Candidate Misplaced (Top 5%)"
		case cand.Misplaced:
			status = "Candidate Misplaced"
		case cand.SingletonUnit:
			status = "Singleton Unit"
		case cand.Top5PercentAnomaly:
			status = "Top 5% Anomaly"
		}
		vals := []any{
			cand.Rank, cand.PlotNumber, cand.AssignedUnit, cand.AssignedUnitPlots,
			cand.OwnSimilarityPct, cand.NearestRivalUnit, cand.RivalSimilarityPct, cand.DeltaSimilarityPct,
			status, cand.MoistureRegime, cand.NutrientRegime, cand.Zone, cand.SubZone, cand.SiteSeries,
		}
		for cIdx, v := range vals {
			if err := setCellChecked(f, outSheet, cIdx+1, r, v); err != nil {
				return nil, err
			}
		}
		if cand.Misplaced || cand.Top5PercentAnomaly {
			_ = styleRangeChecked(f, outSheet, 1, r, len(outHeaders), r, warnRowStyle)
		}
	}
	_ = f.SetColWidth(outSheet, "A", "N", 16)

	// =========================================================================
	// SHEET 4: Edatopic Grid (rSMR 0..8 x rSNR A..F) Counts & Prominence Sheet
	// =========================================================================
	edaSheet := sheetNames[3]
	if err := setCellChecked(f, edaSheet, 1, 1, "Relative Edatopic Grids (rSMR 0..8 x rSNR A..F) — Plot Counts & % Prominence"); err != nil {
		return nil, err
	}
	_ = styleRangeChecked(f, edaSheet, 1, 1, 1, 1, titleStyle)

	writeEdatopicBlock := func(targetSheet string, startRow int, grid EdatopicUnitGrid) (int, error) {
		hdr := fmt.Sprintf("%s (Plotted: %d / %d, Missing Edatopic: %d)",
			grid.Label, grid.PlottedEdatopicCount, grid.TotalPlots, grid.MissingEdatopicCount)
		if err := setCellChecked(f, targetSheet, 1, startRow, hdr); err != nil {
			return startRow, err
		}
		_ = styleRangeChecked(f, targetSheet, 1, startRow, 7, startRow, sectionBannerStyle)

		colHdrRow := startRow + 1
		if err := setCellChecked(f, targetSheet, 1, colHdrRow, "rSMR \\ rSNR"); err != nil {
			return startRow, err
		}
		_ = styleRangeChecked(f, targetSheet, 1, colHdrRow, 1, colHdrRow, headerStyle)
		for cIdx, snr := range SNRCols {
			if err := setCellChecked(f, targetSheet, cIdx+2, colHdrRow, snr); err != nil {
				return startRow, err
			}
			_ = styleRangeChecked(f, targetSheet, cIdx+2, colHdrRow, cIdx+2, colHdrRow, headerStyle)
		}

		for rIdx, smr := range SMRRows {
			rowNum := colHdrRow + 1 + rIdx
			if err := setCellChecked(f, targetSheet, 1, rowNum, smr); err != nil {
				return startRow, err
			}
			_ = styleRangeChecked(f, targetSheet, 1, rowNum, 1, rowNum, sectionBannerStyle)

			for cIdx := range SNRCols {
				cell := grid.Cells[rIdx*len(SNRCols)+cIdx]
				display := ""
				if cell.PlotCount > 0 {
					display = fmt.Sprintf("%d (%.0f%%)", cell.PlotCount, cell.Prominence)
				}
				if err := setCellChecked(f, targetSheet, cIdx+2, rowNum, display); err != nil {
					return startRow, err
				}
				st := centerCellStyle
				if cell.Prominence >= 25.0 {
					st = edatopicCoreStyle
				} else if cell.PlotCount > 0 {
					st = edatopicSubStyle
				}
				_ = styleRangeChecked(f, targetSheet, cIdx+2, rowNum, cIdx+2, rowNum, st)
			}
		}
		return colHdrRow + len(SMRRows) + 2, nil
	}

	nextEdaRow, err := writeEdatopicBlock(edaSheet, 3, snap.OverallEdatopicGrid)
	if err != nil {
		return nil, err
	}
	for uIdx, uGrid := range snap.UnitEdatopicGrids {
		nextEdaRow, err = writeEdatopicBlock(edaSheet, nextEdaRow, uGrid)
		if err != nil {
			return nil, err
		}
		if req.IncludeUnitEdatopicTabs && 4+uIdx < len(sheetNames) {
			uTab := sheetNames[4+uIdx]
			if _, err := writeEdatopicBlock(uTab, 2, uGrid); err != nil {
				return nil, err
			}
			_ = f.SetColWidth(uTab, "A", "G", 15)
		}
	}
	_ = f.SetColWidth(edaSheet, "A", "G", 15)

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validateEncodedDiagnosticWorkbook(ctx context.Context, data []byte, expectedSheets []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	opened, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open staged diagnostic workbook: %w", err)
	}
	defer func() { _ = opened.Close() }()

	actualSheets := opened.GetSheetList()
	if len(actualSheets) != len(expectedSheets) {
		return fmt.Errorf("expected %d sheets in diagnostic workbook, got %d", len(expectedSheets), len(actualSheets))
	}
	for i := range expectedSheets {
		if actualSheets[i] != expectedSheets[i] {
			return fmt.Errorf("sheet #%d mismatch: expected %q, got %q", i+1, expectedSheets[i], actualSheets[i])
		}
		for j := range i {
			if strings.EqualFold(actualSheets[j], actualSheets[i]) {
				return fmt.Errorf("case-folded worksheet collision detected in staged workbook: %q vs %q", actualSheets[j], actualSheets[i])
			}
		}
	}
	return nil
}

// ExportDiagnosticWorkbook atomically publishes a Diagnostic10.xla-compatible 4-sheet .xlsx workbook
// using vprocore.PublishArtifactChecked (staged temp file + os.Link no-overwrite) and strings.EqualFold
// worksheet collision guards.
func ExportDiagnosticWorkbook(
	ctx context.Context,
	contexts *vprocore.ContextService,
	contextID string,
	req ExportDiagnosticWorkbookRequest,
) (*ExportDiagnosticWorkbookResult, error) {
	started := time.Now()
	if ctx == nil {
		return nil, errors.New("diagnostic workbook export requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	outPath := strings.TrimSpace(req.OutputPath)
	if outPath == "" {
		return nil, errors.New("outputPath is required for diagnostic workbook export")
	}
	if !strings.EqualFold(filepathExt(outPath), ".xlsx") {
		return nil, errors.New("diagnostic workbook outputPath must end with .xlsx")
	}

	diagName := strings.TrimSpace(req.DiagnosticSheetName)
	if diagName == "" {
		diagName = "Diagnostic Summary"
	}
	simName := strings.TrimSpace(req.SimilaritySheetName)
	if simName == "" {
		simName = "Pairwise Similarity"
	}
	outName := strings.TrimSpace(req.OutlierSheetName)
	if outName == "" {
		outName = "Outlier Queue"
	}
	edaName := strings.TrimSpace(req.EdatopicSheetName)
	if edaName == "" {
		edaName = "Edatopic Grid"
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ExportDiagnosticWorkbookResult, error) {
		owner := plots.Projects.Sqlite
		snap, err := computeSnapshotForPlots(ctx, owner, contextID, req.Options)
		if err != nil {
			return nil, err
		}

		rawSheets := []string{diagName, simName, outName, edaName}
		if req.IncludeUnitEdatopicTabs {
			for _, u := range snap.Units {
				rawSheets = append(rawSheets, "Eda_"+u.SiteUnit)
			}
		}
		sheetNames, err := ResolveDiagnosticWorkbookSheets(rawSheets)
		if err != nil {
			return nil, err
		}

		pub, err := vprocore.PublishArtifactChecked(
			ctx,
			outPath,
			vprocore.ArtifactPublicationFormat{
				Label:        "diagnostic synthesis workbook",
				Artifact:     "workbook",
				CommitLabel:  "diagnostic synthesis workbook",
				StagePattern: ".vpro-diagnostic-*.xlsx.tmp",
				Encode: func(encodeCtx context.Context) ([]byte, error) {
					return encodeDiagnosticWorkbookBytes(encodeCtx, snap, req, sheetNames)
				},
				Validate: func(valCtx context.Context, data []byte) error {
					return validateEncodedDiagnosticWorkbook(valCtx, data, sheetNames)
				},
			},
			func() error {
				return vprocore.ProfileOwnedFiles(owner)
			},
			vprocore.ArtifactPublicationHooks{},
		)
		if err != nil {
			return nil, err
		}

		latencyMs := float64(time.Since(started).Microseconds()) / 1000.0
		return &ExportDiagnosticWorkbookResult{
			Path:        pub.Path,
			SHA256:      pub.SHA256,
			SheetNames:  sheetNames,
			UnitCount:   len(snap.Units),
			SpeciesRows: len(snap.Species),
			OutlierRows: len(snap.OutlierQueue),
			LatencyMs:   RoundACE15(latencyMs, 2),
		}, nil
	})
}

func filepathExt(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			break
		}
		if p[i] == '.' {
			return p[i:]
		}
	}
	return ""
}
