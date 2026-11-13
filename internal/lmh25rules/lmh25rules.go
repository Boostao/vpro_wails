package lmh25rules

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

type DiagnosticSeverity string

const (
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityError   DiagnosticSeverity = "error"
)

type DiagnosticIssue struct {
	Code       string             `json:"code"`
	Severity   DiagnosticSeverity `json:"severity"`
	Category   string             `json:"category"`
	Field      string             `json:"field,omitempty"`
	HorizonID  *int64             `json:"horizonId,omitempty"`
	Horizon    string             `json:"horizon,omitempty"`
	Message    string             `json:"message"`
	Suggested  string             `json:"suggested,omitempty"`
	LMH25Ref   string             `json:"lmh25Ref"`
}

type KeyTraceStep struct {
	Key       string `json:"key"`
	Step      string `json:"step"`
	Condition string `json:"condition"`
	Outcome   string `json:"outcome"`
}

type PlotDiagnosticsResult struct {
	PlotNumber             string             `json:"plotNumber"`
	DerivedHumusThickness  *float64           `json:"derivedHumusThickness"`
	HeaderHumusThickness   *float64           `json:"headerHumusThickness"`
	HumusThicknessDiff     *float64           `json:"humusThicknessDiff"`
	SubstratePopulated     int                `json:"substratePopulated"`
	SubstrateSum           *float64           `json:"substrateSum"`
	SubstrateValid         bool               `json:"substrateValid"`
	IsOrganicSoilOrder     bool               `json:"isOrganicSoilOrder"`
	KeyedSMR               string             `json:"keyedSmr"`
	KeyedSMRClass          *int               `json:"keyedSmrClass"`
	FieldSMR               string             `json:"fieldSmr"`
	FieldSMRClass          *int               `json:"fieldSmrClass"`
	SMRDivergence          *int               `json:"smrDivergence"`
	SMRDivergent           bool               `json:"smrDivergent"`
	SMRTrace               []KeyTraceStep     `json:"smrTrace"`
	KeyedSNR               string             `json:"keyedSnr"`
	KeyedSNRClass          *int               `json:"keyedSnrClass"`
	FieldSNR               string             `json:"fieldSnr"`
	FieldSNRClass          *int               `json:"fieldSnrClass"`
	SNRDivergence          *int               `json:"snrDivergence"`
	SNRDivergent           bool               `json:"snrDivergent"`
	SNRTrace               []KeyTraceStep     `json:"snrTrace"`
	Issues                 []DiagnosticIssue  `json:"issues"`
}

func roundACE2(val float64) float64 {
	// ACE 15-significant-digit decimal calibration before half-away rounding to 2 decimals
	formatted := vprocore.FormatCalibratedDecimal(val, 2)
	parsed, err := strconv.ParseFloat(formatted, 64)
	if err != nil {
		return val
	}
	return parsed
}

func trimUpper(s *string) string {
	if s == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*s))
}

func isOrganicOrderGroup(code string) bool {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "F", "M", "H", "FO":
		return true
	default:
		return false
	}
}

type classifiedHumus struct {
	rec       vprocore.HumusRecord
	code      string
	isWetland bool
	isUpland  bool
	isSurface bool
}

func classifyHumusHorizon(h string, organicSoilOrder bool, soilClassGroup string) (isUpland, isWetland, isSurface bool) {
	u := strings.ToUpper(strings.TrimSpace(h))
	if u == "" {
		return false, false, false
	}
	if strings.HasPrefix(u, "OF") || strings.HasPrefix(u, "OM") || strings.HasPrefix(u, "OH") ||
		strings.HasPrefix(u, "LIMNO") || strings.HasPrefix(u, "CUMULO") || strings.HasPrefix(u, "HYDRIC") {
		return false, true, false
	}
	if u == "S" {
		return false, false, true
	}
	if strings.HasPrefix(u, "O") {
		return false, true, false
	}
	if strings.HasPrefix(u, "L") || strings.HasPrefix(u, "F") || strings.HasPrefix(u, "H") {
		// Folisols (FO) are upland LFH organic soils; F/M/H great groups measure downward only for O horizons
		if organicSoilOrder && strings.ToUpper(strings.TrimSpace(soilClassGroup)) != "FO" && strings.HasPrefix(u, "O") {
			return false, true, false
		}
		return true, false, false
	}
	return false, false, false
}

var munsellValuePattern = regexp.MustCompile(`(?i)(?:[0-9.]+(?:YR|Y|R|GY|G|BG|B|PB)|N)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:/|$)`)

func parseMunsellValue(colour string) (float64, bool) {
	trimmed := strings.TrimSpace(colour)
	if trimmed == "" {
		return 0, false
	}
	if matches := munsellValuePattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		if v, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return v, true
		}
	}
	// Fallback: format like "3/2" without hue
	if slash := strings.IndexByte(trimmed, '/'); slash > 0 {
		fields := strings.Fields(trimmed[:slash])
		if len(fields) > 0 {
			if v, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

func parseFieldSMRClass(raw string) (*int, bool) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return nil, false
	}
	for i := 0; i < len(t); i++ {
		if t[i] >= '0' && t[i] <= '8' {
			v := int(t[i] - '0')
			return &v, true
		}
	}
	return nil, false
}

func parseFieldSNRClass(raw string) (*int, bool) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	if t == "" {
		return nil, false
	}
	for i := 0; i < len(t); i++ {
		if t[i] >= 'A' && t[i] <= 'F' {
			v := int(t[i] - 'A')
			return &v, true
		}
	}
	return nil, false
}

func snrClassLetter(c int) string {
	if c < 0 {
		c = 0
	}
	if c > 5 {
		c = 5
	}
	return string(rune('A' + c))
}

func EvaluatePlotDiagnostics(header vprocore.FS882Header, humus []vprocore.HumusRecord, mineral []vprocore.MineralRecord) PlotDiagnosticsResult {
	res := PlotDiagnosticsResult{
		PlotNumber:           header.PlotNumber,
		HeaderHumusThickness: header.HumusThickness,
		SubstrateValid:       true,
		FieldSMR:             trimUpper(header.MoistureRegime),
		FieldSNR:             trimUpper(header.NutrientRegime),
		SMRTrace:             []KeyTraceStep{},
		SNRTrace:             []KeyTraceStep{},
		Issues:               []DiagnosticIssue{},
	}

	soilGroup := trimUpper(header.SoilClassGroup)
	res.IsOrganicSoilOrder = isOrganicOrderGroup(soilGroup)

	// 1. 6-Field Substrate Sum Check (LMH25 2023 pp. 40-41)
	substrates := []struct {
		name string
		val  *float64
	}{
		{"SubstrateDecWood", header.SubstrateDecWood},
		{"SubstrateBedRock", header.SubstrateBedRock},
		{"SubstrateRocks", header.SubstrateRocks},
		{"SubstrateMineralSoil", header.SubstrateMineralSoil},
		{"SubstrateOrganicMatter", header.SubstrateOrganicMatter},
		{"SubstrateWater", header.SubstrateWater},
	}
	var subSum float64
	for _, sub := range substrates {
		if sub.val != nil {
			res.SubstratePopulated++
			v := *sub.val
			subSum += v
			if v < 0 || v > 100 {
				res.SubstrateValid = false
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:     "SUBSTRATE_VALUE_OUT_OF_BOUNDS",
					Severity: SeverityError,
					Category: "substrate",
					Field:    sub.name,
					Message:  fmt.Sprintf("%s (%.2f%%) must be between 0%% and 100%%.", sub.name, v),
					LMH25Ref: "LMH25 (2023) Sec. 1 Item 36 pp. 40-41",
				})
			}
		}
	}
	if res.SubstratePopulated > 0 {
		roundedSum := roundACE2(subSum)
		res.SubstrateSum = &roundedSum
		if math.Abs(subSum-100.0) > 0.01 {
			res.SubstrateValid = false
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "SUBSTRATE_SUM_NOT_100",
				Severity:  SeverityWarning,
				Category:  "substrate",
				Field:     "SubstrateSum",
				Message:   fmt.Sprintf("Surface substrates sum to %.2f%% across %d populated field(s); LMH25 requires 100%% total.", roundedSum, res.SubstratePopulated),
				Suggested: "100",
				LMH25Ref:  "LMH25 (2023) Sec. 1 Item 36 pp. 40-41",
			})
		}
	}

	// 2. Humus Horizons: DerivedHumusThickness, Directional Sign, Contiguity, and Table 2.23 von Post
	var uplandHorizons []vprocore.HumusRecord
	var wetlandHorizons []vprocore.HumusRecord
	var maxUplandUpper float64
	var minUplandLower float64
	hasUplandDepths := false
	var uplandHThickness, uplandTotalThickness float64
	var peatTotalThickness, peatOfUpper40, peatOmUpper40, peatUpper40Total float64
	hasPeatyHorizons := false

	for _, hr := range humus {
		hCode := trimUpper(hr.Horizon)
		idCopy := hr.ID
		isUpland, isWetland, _ := classifyHumusHorizon(hCode, res.IsOrganicSoilOrder, soilGroup)

		// Table 2.23 von Post check (LMH25 p. 73)
		if hr.VonPost != nil {
			vp := *hr.VonPost
			if vp < 1 || vp > 10 {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "VONPOST_INVALID_SCALE",
					Severity:  SeverityError,
					Category:  "soil_vonpost",
					Field:     "vonPost",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Horizon %s has von Post = %d; valid von Post scale is 1..10.", hCode, vp),
					LMH25Ref:  "LMH25 (2023) Table 2.23 p. 73",
				})
			} else if strings.HasPrefix(hCode, "OF") && (vp < 1 || vp > 4) {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "VONPOST_OUT_OF_RANGE_OF",
					Severity:  SeverityWarning,
					Category:  "soil_vonpost",
					Field:     "vonPost",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Fibric horizon %s has von Post = %d; Table 2.23 specifies 1..4 for Of.", hCode, vp),
					Suggested: "1..4",
					LMH25Ref:  "LMH25 (2023) Table 2.23 p. 73",
				})
			} else if strings.HasPrefix(hCode, "OM") && (vp < 5 || vp > 6) {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "VONPOST_OUT_OF_RANGE_OM",
					Severity:  SeverityWarning,
					Category:  "soil_vonpost",
					Field:     "vonPost",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Mesic peat horizon %s has von Post = %d; Table 2.23 specifies 5..6 for Om.", hCode, vp),
					Suggested: "5..6",
					LMH25Ref:  "LMH25 (2023) Table 2.23 p. 73",
				})
			} else if strings.HasPrefix(hCode, "OH") && (vp < 7 || vp > 10) {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "VONPOST_OUT_OF_RANGE_OH",
					Severity:  SeverityWarning,
					Category:  "soil_vonpost",
					Field:     "vonPost",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Humic peat horizon %s has von Post = %d; Table 2.23 specifies 7..10 for Oh.", hCode, vp),
					Suggested: "7..10",
					LMH25Ref:  "LMH25 (2023) Table 2.23 p. 73",
				})
			}
		}

		if hr.UpperDepth == nil || hr.LowerDepth == nil {
			continue
		}
		up, lo := *hr.UpperDepth, *hr.LowerDepth

		if isUpland {
			if up <= lo || lo < 0 {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "HORIZON_SIGN_UPLAND_LFH",
					Severity:  SeverityError,
					Category:  "soil_horizon",
					Field:     "UpperDepth",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Upland humus horizon %s (%.1f to %.1f cm) must measure upward from 0 cm with UpperDepth > LowerDepth >= 0.", hCode, up, lo),
					LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
				})
			} else {
				uplandHorizons = append(uplandHorizons, hr)
				if !hasUplandDepths {
					maxUplandUpper = up
					minUplandLower = lo
					hasUplandDepths = true
				} else {
					if up > maxUplandUpper {
						maxUplandUpper = up
					}
					if lo < minUplandLower {
						minUplandLower = lo
					}
				}
				thick := up - lo
				uplandTotalThickness += thick
				if strings.HasPrefix(hCode, "H") {
					uplandHThickness += thick
				}
			}
		} else if isWetland {
			hasPeatyHorizons = true
			if up >= lo || up < 0 {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "HORIZON_SIGN_WETLAND_O",
					Severity:  SeverityError,
					Category:  "soil_horizon",
					Field:     "UpperDepth",
					HorizonID: &idCopy,
					Horizon:   hCode,
					Message:   fmt.Sprintf("Wetland organic horizon %s (%.1f to %.1f cm) must measure downward from 0 cm with 0 <= UpperDepth < LowerDepth.", hCode, up, lo),
					LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
				})
			} else {
				wetlandHorizons = append(wetlandHorizons, hr)
				peatTotalThickness += (lo - up)
				// Compute overlap with upper 40 cm tier (0..40 cm)
				tStart := math.Max(0, up)
				tEnd := math.Min(40, lo)
				if tEnd > tStart {
					span := tEnd - tStart
					peatUpper40Total += span
					if strings.HasPrefix(hCode, "OF") {
						peatOfUpper40 += span
					} else if strings.HasPrefix(hCode, "OM") {
						peatOmUpper40 += span
					}
				}
			}
		}
	}

	if hasUplandDepths {
		derived := roundACE2(maxUplandUpper - minUplandLower)
		res.DerivedHumusThickness = &derived
		if header.HumusThickness == nil {
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "HUMUS_THICKNESS_SUGGESTED",
				Severity:  SeverityInfo,
				Category:  "humus_thickness",
				Field:     "HumusThickness",
				Message:   fmt.Sprintf("Derived upland L/F/H HumusThickness is %.1f cm (%.1f to %.1f cm); header HumusThickness is currently empty.", derived, maxUplandUpper, minUplandLower),
				Suggested: strconv.FormatFloat(derived, 'f', -1, 64),
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
			})
		} else {
			diff := roundACE2(*header.HumusThickness - derived)
			res.HumusThicknessDiff = &diff
			if math.Abs(diff) > 0.05 {
				res.Issues = append(res.Issues, DiagnosticIssue{
					Code:      "HUMUS_THICKNESS_MISMATCH",
					Severity:  SeverityWarning,
					Category:  "humus_thickness",
					Field:     "HumusThickness",
					Message:   fmt.Sprintf("Header HumusThickness (%.1f cm) differs from upland L/F/H horizon span (%.1f cm) by %+.1f cm.", *header.HumusThickness, derived, diff),
					Suggested: strconv.FormatFloat(derived, 'f', -1, 64),
					LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
				})
			}
		}
	}

	// Upland contiguity check (ordered UpperDepth DESC)
	slices.SortFunc(uplandHorizons, func(a, b vprocore.HumusRecord) int {
		if *a.UpperDepth > *b.UpperDepth {
			return -1
		}
		if *a.UpperDepth < *b.UpperDepth {
			return 1
		}
		return int(a.ID - b.ID)
	})
	for i := 0; i+1 < len(uplandHorizons); i++ {
		curr, next := uplandHorizons[i], uplandHorizons[i+1]
		diff := *curr.LowerDepth - *next.UpperDepth
		if math.Abs(diff) > 0.01 {
			nextID := next.ID
			code := "HORIZON_CONTIGUITY_GAP"
			desc := "gap"
			if diff < 0 {
				code = "HORIZON_CONTIGUITY_OVERLAP"
				desc = "overlap"
			}
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      code,
				Severity:  SeverityWarning,
				Category:  "soil_contiguity",
				Field:     "UpperDepth",
				HorizonID: &nextID,
				Horizon:   trimUpper(next.Horizon),
				Message:   fmt.Sprintf("Upland humus contiguity %s between %s (LowerDepth %.1f cm) and %s (UpperDepth %.1f cm).", desc, trimUpper(curr.Horizon), *curr.LowerDepth, trimUpper(next.Horizon), *next.UpperDepth),
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
			})
		}
	}
	if len(uplandHorizons) > 0 && len(mineral) > 0 {
		last := uplandHorizons[len(uplandHorizons)-1]
		if *last.LowerDepth > 0.01 {
			lastID := last.ID
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "HORIZON_CONTIGUITY_GAP",
				Severity:  SeverityWarning,
				Category:  "soil_contiguity",
				Field:     "LowerDepth",
				HorizonID: &lastID,
				Horizon:   trimUpper(last.Horizon),
				Message:   fmt.Sprintf("Lowest upland humus horizon %s ends at %.1f cm instead of 0 cm mineral interface.", trimUpper(last.Horizon), *last.LowerDepth),
				Suggested: "0",
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
			})
		}
	}

	// Wetland O contiguity check (ordered UpperDepth ASC)
	slices.SortFunc(wetlandHorizons, func(a, b vprocore.HumusRecord) int {
		if *a.UpperDepth < *b.UpperDepth {
			return -1
		}
		if *a.UpperDepth > *b.UpperDepth {
			return 1
		}
		return int(a.ID - b.ID)
	})
	for i := 0; i+1 < len(wetlandHorizons); i++ {
		curr, next := wetlandHorizons[i], wetlandHorizons[i+1]
		diff := *next.UpperDepth - *curr.LowerDepth
		if math.Abs(diff) > 0.01 {
			nextID := next.ID
			code := "HORIZON_CONTIGUITY_GAP"
			desc := "gap"
			if diff < 0 {
				code = "HORIZON_CONTIGUITY_OVERLAP"
				desc = "overlap"
			}
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      code,
				Severity:  SeverityWarning,
				Category:  "soil_contiguity",
				Field:     "UpperDepth",
				HorizonID: &nextID,
				Horizon:   trimUpper(next.Horizon),
				Message:   fmt.Sprintf("Wetland organic horizon %s between %s (LowerDepth %.1f cm) and %s (UpperDepth %.1f cm).", desc, trimUpper(curr.Horizon), *curr.LowerDepth, trimUpper(next.Horizon), *next.UpperDepth),
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 17 p. 71",
			})
		}
	}

	// 3. Mineral Horizons: Directional Sign, Contiguity, Texture, Coarse Frags, Ah/Ae, Gleying & Munsell Value
	var validMinerals []vprocore.MineralRecord
	var ahDepth, aeDepth float64
	var bDark, bLight bool
	var hasGleying bool
	var minGleyDepth float64 = -1
	var lithicDepth float64 = -1
	var maxMineralLower float64
	var weightedCF, cfWeight float64
	var dominantTexture string
	var maxThickForTex float64
	var hasTurbation, hasSalineHorizon bool

	for _, mr := range mineral {
		mCode := trimUpper(mr.Horizon)
		idCopy := mr.ID
		if strings.Contains(strings.ToLower(strings.TrimSpace(derefStr(mr.Horizon))), "g") || strings.HasPrefix(mCode, "W") {
			hasGleying = true
			if mr.UpperDepth != nil && (minGleyDepth < 0 || *mr.UpperDepth < minGleyDepth) {
				minGleyDepth = *mr.UpperDepth
			}
		}
		rawLower := strings.ToLower(strings.TrimSpace(derefStr(mr.Horizon)))
		if strings.Contains(rawLower, "u") || strings.Contains(rawLower, "b") && strings.HasPrefix(mCode, "A") {
			hasTurbation = true
		}
		if strings.Contains(rawLower, "sa") || (strings.HasPrefix(mCode, "B") && strings.Contains(rawLower, "n")) {
			hasSalineHorizon = true
		}
		if mr.MineralFormpH != nil && *mr.MineralFormpH > 8.2 {
			hasSalineHorizon = true
		}
		if strings.HasPrefix(mCode, "R") && mr.UpperDepth != nil {
			if lithicDepth < 0 || *mr.UpperDepth < lithicDepth {
				lithicDepth = *mr.UpperDepth
			}
		}
		if strings.HasPrefix(mCode, "B") && mr.Colour != nil {
			if val, ok := parseMunsellValue(*mr.Colour); ok {
				if val <= 3.0 {
					bDark = true
				}
				if val >= 7.0 {
					bLight = true
				}
			}
		}

		if mr.UpperDepth == nil || mr.LowerDepth == nil {
			continue
		}
		up, lo := *mr.UpperDepth, *mr.LowerDepth
		if up >= lo || up < 0 {
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "HORIZON_SIGN_MINERAL",
				Severity:  SeverityError,
				Category:  "soil_horizon",
				Field:     "UpperDepth",
				HorizonID: &idCopy,
				Horizon:   mCode,
				Message:   fmt.Sprintf("Mineral horizon %s (%.1f to %.1f cm) must measure downward from 0 cm with 0 <= UpperDepth < LowerDepth.", mCode, up, lo),
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 25 p. 79",
			})
			continue
		}
		validMinerals = append(validMinerals, mr)
		thick := lo - up
		if lo > maxMineralLower {
			maxMineralLower = lo
		}
		if strings.HasPrefix(mCode, "AH") || strings.HasPrefix(mCode, "AP") {
			ahDepth += thick
		}
		if strings.HasPrefix(mCode, "AE") {
			aeDepth += thick
		}
		if mr.PercentCoarseFragsTotal != nil && up < 100 {
			weightedCF += float64(*mr.PercentCoarseFragsTotal) * thick
			cfWeight += thick
		}
		if mr.Texture != nil && strings.TrimSpace(*mr.Texture) != "" && thick > maxThickForTex {
			maxThickForTex = thick
			dominantTexture = strings.ToUpper(strings.TrimSpace(*mr.Texture))
		}
	}

	slices.SortFunc(validMinerals, func(a, b vprocore.MineralRecord) int {
		if *a.UpperDepth < *b.UpperDepth {
			return -1
		}
		if *a.UpperDepth > *b.UpperDepth {
			return 1
		}
		return int(a.ID - b.ID)
	})
	for i := 0; i+1 < len(validMinerals); i++ {
		curr, next := validMinerals[i], validMinerals[i+1]
		diff := *next.UpperDepth - *curr.LowerDepth
		if math.Abs(diff) > 0.01 {
			nextID := next.ID
			code := "HORIZON_CONTIGUITY_GAP"
			desc := "gap"
			if diff < 0 {
				code = "HORIZON_CONTIGUITY_OVERLAP"
				desc = "overlap"
			}
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      code,
				Severity:  SeverityWarning,
				Category:  "soil_contiguity",
				Field:     "UpperDepth",
				HorizonID: &nextID,
				Horizon:   trimUpper(next.Horizon),
				Message:   fmt.Sprintf("Mineral horizon contiguity %s between %s (LowerDepth %.1f cm) and %s (UpperDepth %.1f cm).", desc, trimUpper(curr.Horizon), *curr.LowerDepth, trimUpper(next.Horizon), *next.UpperDepth),
				LMH25Ref:  "LMH25 (2023) Sec. 2 Item 25 p. 79",
			})
		}
	}

	// 4. LMH25 (2023) Keys 9.3, 9.4 (rSMR 0..8) & 9.6 / 9.7 (rSNR A..F) Inference
	inferSMRAndSNR(&res, header, soilGroup, hasPeatyHorizons, peatTotalThickness, peatUpper40Total, peatOfUpper40, peatOmUpper40,
		uplandTotalThickness, uplandHThickness, ahDepth, aeDepth, bDark, bLight, hasGleying, minGleyDepth, lithicDepth,
		maxMineralLower, weightedCF, cfWeight, dominantTexture, hasTurbation, hasSalineHorizon)

	return res
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func inferSMRAndSNR(
	res *PlotDiagnosticsResult,
	header vprocore.FS882Header,
	soilGroup string,
	hasPeatyHorizons bool,
	peatTotalThickness, peatUpper40Total, peatOfUpper40, peatOmUpper40 float64,
	uplandTotalThickness, uplandHThickness float64,
	ahDepth, aeDepth float64,
	bDark, bLight, hasGleying bool,
	minGleyDepth, lithicDepth, maxMineralLower, weightedCF, cfWeight float64,
	dominantTexture string,
	hasTurbation, hasSalineHorizon bool,
) {
	slope := 0.0
	if header.Slope != nil {
		slope = *header.Slope
	}
	aspect := -1
	if header.Aspect != nil {
		aspect = *header.Aspect
	}
	meso := trimUpper(header.MesoSlopePos)
	drainage := strings.ToLower(strings.TrimSpace(derefStr(header.SoilDrainage)))
	waterSrc := trimUpper(header.WaterSource)
	rzps := trimUpper(header.RootZoneParticleSize)
	rrType := trimUpper(header.RootRestrictingType)
	humusForm := trimUpper(header.HumusForm)
	floodFreq := trimUpper(header.FloodingRegimeFreq)
	surfMat := trimUpper(header.SurficialMaterialSurf)

	// Insolation vs Cold slope (Key 9.3 & Key 9.4 note c)
	insolation := slope > 35.0 && aspect >= 157 && aspect <= 285
	coldSlope := slope > 35.0 && aspect >= 0 && aspect != 999 && (aspect >= 337 || aspect <= 105)

	// Soil depth (cm) to restricting layer
	soilDepth := 120.0
	hasExplicitSoilDepth := false
	if header.RootRestrictingDepth != nil && rrType != "N" {
		soilDepth = float64(*header.RootRestrictingDepth)
		hasExplicitSoilDepth = true
	}
	if lithicDepth >= 0 && (!hasExplicitSoilDepth || lithicDepth < soilDepth) {
		soilDepth = lithicDepth
		hasExplicitSoilDepth = true
	}
	if !hasExplicitSoilDepth && maxMineralLower > 0 && maxMineralLower < 40 && rrType == "L" {
		soilDepth = maxMineralLower
		hasExplicitSoilDepth = true
	}

	avgCF := 20.0
	if cfWeight > 0 {
		avgCF = weightedCF / cfWeight
	}

	// Particle size Coarse vs Fine (Key 9.3 definitions)
	isSandy := dominantTexture == "S" || dominantTexture == "LS" || rzps == "S" || rzps == "SS"
	isLoamy := dominantTexture == "SL" || dominantTexture == "L" || dominantTexture == "SCL" || rzps == "CL" || rzps == "L" || rzps == "FL"
	isSilty := dominantTexture == "SIL" || dominantTexture == "SI" || rzps == "CSI" || rzps == "FSI"
	isClayey := dominantTexture == "SICL" || dominantTexture == "CL" || dominantTexture == "SC" || dominantTexture == "SIC" || dominantTexture == "C" || rzps == "C" || rzps == "FC"

	isFragmental := rzps == "F" || avgCF > 85.0
	isCoarse := isFragmental || rzps == "S" || rzps == "SS" || rzps == "CI" || rzps == "CLS" || rzps == "AS" ||
		(isSandy && (cfWeight == 0 || avgCF > 35.0)) ||
		(isLoamy && avgCF > 70.0) || avgCF > 70.0
	isFine := !isCoarse && (rzps == "C" || rzps == "FC" || rzps == "CSI" || rzps == "FSI" ||
		((isSilty || isClayey) && (cfWeight == 0 || avgCF <= 20.0)))

	// Wet Soil Indicators (WSI - Key 9.3)
	wsi := false
	wsiDepth := 999.0
	if header.SeepageDepth != nil && *header.SeepageDepth >= 0 {
		wsi = true
		if float64(*header.SeepageDepth) < wsiDepth {
			wsiDepth = float64(*header.SeepageDepth)
		}
	}
	if hasGleying {
		wsi = true
		if minGleyDepth >= 0 && minGleyDepth < wsiDepth {
			wsiDepth = minGleyDepth
		}
	}
	if hasPeatyHorizons || (res.IsOrganicSoilOrder && soilGroup != "FO") {
		wsi = true
		if peatTotalThickness >= 20 && wsiDepth > 15 {
			wsiDepth = 10
		}
	}
	switch drainage {
	case "v", "aq", "pa":
		wsi = true
		wsiDepth = 0
	case "p", "ac":
		wsi = true
		if wsiDepth > 25 {
			wsiDepth = 20
		}
	case "i", "sa":
		wsi = true
		if wsiDepth > 45 {
			wsiDepth = 45
		}
	}
	if waterSrc == "G" || waterSrc == "S" || waterSrc == "M" || waterSrc == "T" || waterSrc == "E" {
		wsi = true
		if wsiDepth > 50 {
			wsiDepth = 45
		}
	}
	waterTableAtSurface := drainage == "v" || drainage == "aq" || drainage == "pa" ||
		(header.SeepageDepth != nil && *header.SeepageDepth == 0) ||
		(header.SubstrateWater != nil && *header.SubstrateWater >= 25.0)

	activeFluvial := waterSrc == "F" || floodFreq == "A" || floodFreq == "F" || strings.HasPrefix(surfMat, "FA")
	bedrockPct := 0.0
	if header.SubstrateBedRock != nil {
		bedrockPct = *header.SubstrateBedRock
	}

	// Slope group
	slopeGroup := "MID"
	switch meso {
	case "CR", "UP":
		slopeGroup = "UPPER"
	case "MD":
		slopeGroup = "MID"
	case "LW", "TO", "GU":
		slopeGroup = "LOWER"
	case "LV", "DP":
		slopeGroup = "FLAT"
	default:
		if header.Slope != nil && slope < 5.0 {
			slopeGroup = "FLAT"
		}
	}

	res.SMRTrace = append(res.SMRTrace, KeyTraceStep{
		Key:       "9.3",
		Step:      "Site & Soil Indicators",
		Condition: fmt.Sprintf("MesoSlope=%s (%s), Slope=%.1f%%, Aspect=%d, SoilDepth=%.0fcm, Coarse=%t, Fine=%t, WSI=%t (depth=%.0fcm)", meso, slopeGroup, slope, aspect, soilDepth, isCoarse, isFine, wsi, wsiDepth),
		Outcome:   slopeGroup,
	})

	// Key 9.4 Decision Tree
	var smrClass int
	var smrLabel string

	switch slopeGroup {
	case "UPPER":
		if wsi {
			if wsiDepth < 30 {
				smrClass, smrLabel = 6, "6"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper + WSI", "WSI < 30 cm from surface", "6"})
			} else {
				smrClass, smrLabel = 5, "(4)-5"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper + WSI", "WSI >= 30 cm from surface", "(4)-5"})
			}
		} else if hasExplicitSoilDepth && soilDepth < 40 {
			if bedrockPct > 10 || soilDepth < 20 {
				smrClass, smrLabel = 0, "0(-1)"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper shallow (<40 cm)", "Bedrock > 10% or soil depth < 20 cm", "0(-1)"})
			} else {
				smrClass, smrLabel = 1, "1"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper shallow (<40 cm)", "Soil depth 20-40 cm, bedrock <= 10%", "1"})
			}
		} else if isCoarse {
			if insolation {
				smrClass, smrLabel = 1, "1(-2)"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper deep coarse", "Insolation slope (>35%, SSE-W)", "1(-2)"})
			} else {
				smrClass, smrLabel = 2, "(1-)2"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper deep coarse", "Non-insolation slope", "(1-)2"})
				if coldSlope {
					smrClass, smrLabel = 3, "2-3 (cold aspect)"
				}
			}
		} else {
			if insolation {
				smrClass, smrLabel = 2, "2(-3)"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper deep medium/fine", "Insolation slope (>35%, SSE-W)", "2(-3)"})
			} else {
				smrClass, smrLabel = 3, "(2-)3"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Ridge/Upper deep medium/fine", "Non-insolation slope", "(2-)3"})
				if coldSlope {
					smrClass, smrLabel = 4, "3-4 (cold aspect)"
				}
			}
		}

	case "MID":
		if wsi {
			if wsiDepth < 30 {
				smrClass, smrLabel = 6, "6"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope + WSI", "WSI < 30 cm from surface", "6"})
			} else {
				smrClass, smrLabel = 5, "(4-)5"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope + WSI", "WSI >= 30 cm from surface", "(4-)5"})
			}
		} else {
			shallow := hasExplicitSoilDepth && soilDepth < 50
			if isCoarse {
				if shallow || insolation {
					if shallow && insolation {
						smrClass, smrLabel = 1, "1-(2)"
						res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope coarse", "Both soil depth < 50 cm and insolation", "1-(2)"})
					} else {
						smrClass, smrLabel = 2, "2"
						res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope coarse", "Either soil depth < 50 cm or insolation", "2"})
					}
				} else {
					smrClass, smrLabel = 3, "3"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope coarse", "Soil depth >= 50 cm and non-insolation", "3"})
				}
			} else {
				if shallow || insolation {
					if shallow && insolation {
						smrClass, smrLabel = 2, "2"
						res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope medium/fine", "Both soil depth < 50 cm and insolation", "2"})
					} else {
						smrClass, smrLabel = 3, "3"
						res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope medium/fine", "Either soil depth < 50 cm or insolation", "3"})
						if coldSlope {
							smrClass, smrLabel = 4, "3-4 (cold aspect)"
						}
					}
				} else {
					smrClass, smrLabel = 4, "4"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Mid slope medium/fine", "Deep (>= 50 cm) non-insolation mesic reference", "4"})
				}
			}
		}

	case "LOWER":
		if wsi {
			if wsiDepth < 30 {
				if waterTableAtSurface {
					smrClass, smrLabel = 8, "8"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Lower/Toe/Gully + WSI", "Water table at or above surface", "8"})
				} else {
					smrClass, smrLabel = 7, "7"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Lower/Toe/Gully + WSI", "WSI < 30 cm from surface", "7"})
				}
			} else {
				smrClass, smrLabel = 6, "6"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Lower/Toe/Gully + WSI", "WSI >= 30 cm from surface", "6"})
			}
		} else {
			if isCoarse {
				smrClass, smrLabel = 4, "4(-5)"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Lower/Toe/Gully without WSI", "Coarse particle size", "4(-5)"})
			} else {
				smrClass, smrLabel = 5, "5(-6)"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Lower/Toe/Gully without WSI", "Medium/fine particle size", "5(-6)"})
			}
		}

	case "FLAT":
		if (wsi && wsiDepth < 60) || activeFluvial {
			if wsi && wsiDepth < 30 {
				if waterTableAtSurface {
					smrClass, smrLabel = 8, "8"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression wet", "WSI < 30 cm and water table at/above surface", "8"})
				} else {
					smrClass, smrLabel = 7, "7"
					res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression wet", "WSI < 30 cm, water table below surface", "7"})
				}
			} else if isFine {
				smrClass, smrLabel = 6, "6"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression moist", "WSI 30-60 cm or fluvial + fine particle size", "6"})
			} else {
				smrClass, smrLabel = 5, "5"
				res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression moist", "WSI 30-60 cm or fluvial + non-fine particle size", "5"})
			}
		} else if isCoarse {
			smrClass, smrLabel = 2, "2-3"
			res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression dry", "No WSI < 60 cm + coarse particle size", "2-3"})
		} else if isFine {
			smrClass, smrLabel = 5, "5"
			res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression dry", "No WSI < 60 cm + fine particle size", "5"})
		} else {
			smrClass, smrLabel = 4, "4"
			res.SMRTrace = append(res.SMRTrace, KeyTraceStep{"9.4", "Flat/Depression dry", "No WSI < 60 cm + medium particle size", "4"})
		}
	}

	res.KeyedSMR = smrLabel
	res.KeyedSMRClass = &smrClass
	if fieldClass, ok := parseFieldSMRClass(res.FieldSMR); ok {
		res.FieldSMRClass = fieldClass
		diff := *fieldClass - smrClass
		if diff < 0 {
			diff = -diff
		}
		res.SMRDivergence = &diff
		if diff >= 2 {
			res.SMRDivergent = true
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "SMR_KEY_DIVERGENCE",
				Severity:  SeverityWarning,
				Category:  "edatopic_smr",
				Field:     "MoistureRegime",
				Message:   fmt.Sprintf("Field MoistureRegime (%s) diverges by %d classes from LMH25 Key 9.4 inference (%s).", res.FieldSMR, diff, smrLabel),
				Suggested: strconv.Itoa(smrClass),
				LMH25Ref:  "LMH25 (2023) Key 9.4 p. 226",
			})
		}
	}

	// Key 9.6 & 9.7 Decision Tree for rSNR (A..F -> 0..5)
	restrictedFlow := slope < 5.0 && (drainage == "p" || drainage == "v" || drainage == "aq" || drainage == "pa" || isFine)
	humusClass := classifyHumusFormFamily(humusForm)
	isSaline := waterSrc == "E" || rrType == "X" || hasSalineHorizon

	var snrClass int
	var snrLabel string

	if isSaline {
		snrClass, snrLabel = 5, "F"
		res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Excess Saline / Tidal / Chemical", "Saline water source, chemical restriction, or pH > 8.2", "F"})
	} else if wsi || activeFluvial {
		if peatTotalThickness > 20 || (res.IsOrganicSoilOrder && soilGroup != "FO") {
			ofDom := (peatUpper40Total > 0 && peatOfUpper40/peatUpper40Total >= 0.5) || soilGroup == "F"
			ofOmDom := (peatUpper40Total > 0 && (peatOfUpper40+peatOmUpper40)/peatUpper40Total >= 0.5) || soilGroup == "M"
			if ofDom {
				snrClass, snrLabel = 0, "A-B"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Wet/Peaty > 20 cm", "Of (fibric) dominant in upper 40 cm", "A-B"})
			} else if ofOmDom {
				snrClass, snrLabel = 1, "B-C"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Wet/Peaty > 20 cm", "Of+Om dominant in upper 40 cm", "B-C"})
			} else if restrictedFlow {
				snrClass, snrLabel = 2, "C-D"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Wet/Peaty > 20 cm", "Oh dominant + restricted water flow", "C-D"})
			} else {
				snrClass, snrLabel = 3, "D-E"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Wet/Peaty > 20 cm", "Oh dominant + moving water", "D-E"})
			}
		} else if wsi && wsiDepth < 30 {
			if restrictedFlow {
				if ahDepth > 5 || humusClass == "MULL" {
					snrClass, snrLabel = 2, "C"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI < 30 cm + restricted flow", "Ah > 5 cm or Mull humus", "C"})
				} else {
					snrClass, snrLabel = 1, "B"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI < 30 cm + restricted flow", "Ah <= 5 cm", "B"})
				}
			} else {
				if ahDepth > 5 || activeFluvial || humusClass == "MULL" {
					snrClass, snrLabel = 3, "D"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI < 30 cm + moving water", "Ah > 5 cm, Mull, or active fluvial", "D"})
				} else {
					snrClass, snrLabel = 2, "C"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI < 30 cm + moving water", "Ah <= 5 cm, non-fluvial", "C"})
				}
			}
		} else {
			if restrictedFlow {
				if ahDepth > 5 || humusClass == "MULL" {
					snrClass, snrLabel = 3, "D"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI >= 30 cm + restricted flow", "Ah > 5 cm or Mull humus", "D"})
				} else {
					snrClass, snrLabel = 2, "C"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI >= 30 cm + restricted flow", "Ah <= 5 cm", "C"})
				}
			} else {
				if ahDepth > 5 || activeFluvial || humusClass == "MULL" {
					snrClass, snrLabel = 4, "E"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI >= 30 cm / Active Fluvial", "Ah > 5 cm, Mull, or active fluvial seepage", "E"})
				} else {
					snrClass, snrLabel = 3, "D"
					res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "WSI >= 30 cm + moving water", "Ah <= 5 cm", "D"})
				}
			}
		}
	} else if (hasExplicitSoilDepth && soilDepth < 30) || isFragmental || soilGroup == "FO" {
		if soilGroup == "FO" {
			if (uplandTotalThickness + soilDepth) < 30 {
				snrClass, snrLabel = 0, "A"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Folisol", "LFH + Mineral < 30 cm", "A"})
			} else if uplandTotalThickness > 0 && uplandHThickness/uplandTotalThickness >= 0.5 {
				snrClass, snrLabel = 2, "C"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Folisol", "H horizons dominant", "C"})
			} else {
				snrClass, snrLabel = 1, "B"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Folisol", "L/F horizons dominant", "B"})
			}
		} else if ahDepth > 5 || bDark || humusClass == "MULL" {
			snrClass, snrLabel = 2, "C"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Mineral < 30 cm or Fragmental", "Ah > 5 cm, dark B (value <= 3), or Mull", "C"})
		} else if bLight || aeDepth > 2 || (hasExplicitSoilDepth && soilDepth < 10) || (humusClass == "MOR" && isCoarse) {
			snrClass, snrLabel = 0, "A"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Mineral < 30 cm or Fragmental", "Light soil (value >= 7), Ae > 2 cm, depth < 10 cm, or coarse Mor", "A"})
		} else {
			snrClass, snrLabel = 1, "B"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Mineral < 30 cm or Fragmental", "Moderate mineral colour, no thick Ae", "B"})
		}
	} else if isCoarse || (hasExplicitSoilDepth && soilDepth < 50) {
		if ahDepth > 2 || bDark || humusClass == "MULL" {
			if hasTurbation || humusClass == "MULL" {
				snrClass, snrLabel = 3, "D"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Coarse or depth < 50 cm", "Ah > 2 cm / dark B + Mull or soil turbation", "D"})
			} else {
				snrClass, snrLabel = 2, "C"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Coarse or depth < 50 cm", "Ah > 2 cm or dark B without turbation", "C"})
			}
		} else if bLight || aeDepth > 2 {
			snrClass, snrLabel = 0, "A"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Coarse or depth < 50 cm", "Light B (value >= 7) or Ae > 2 cm", "A"})
		} else if humusClass == "MODER" {
			snrClass, snrLabel = 2, "C"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.7", "Coarse or depth < 50 cm + Moder", "Moder humus form on >= 30 cm soil", "C"})
		} else {
			snrClass, snrLabel = 1, "B"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Coarse or depth < 50 cm", "Mor/unspecified humus without dark B or thick Ae", "B"})
		}
	} else {
		// Deep medium/fine upland soil
		if ahDepth > 2 || bDark || humusClass == "MULL" {
			if isFine && ahDepth > 5 {
				snrClass, snrLabel = 4, "E"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Deep medium/fine upland", "Fine textured + Ah > 5 cm", "E"})
			} else {
				snrClass, snrLabel = 3, "D"
				res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Deep medium/fine upland", "Ah > 2 cm, dark B (value <= 3), or Mull humus", "D"})
			}
		} else if bLight || aeDepth > 2 || (humusClass == "MOR" && coldSlope && uplandTotalThickness > 10) {
			snrClass, snrLabel = 1, "B"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Deep medium/fine upland", "Light B (value >= 7), Ae > 2 cm, or thick Mor on cool aspect", "B"})
		} else if humusClass == "MODER" && slopeGroup == "LOWER" {
			snrClass, snrLabel = 3, "D"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.7", "Deep medium/fine + Moder on lower/toe", "Moder humus on receiving slope", "D"})
		} else {
			snrClass, snrLabel = 2, "C"
			res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6", "Deep medium/fine upland", "Mesotrophic reference soil", "C"})
		}
	}

	// Permafrost adjustment (* footnote in Key 9.6: lower SNR by 1 class when permafrost present)
	if (waterSrc == "Z" || rrType == "Z" || (header.ActiveLayerDepth != nil && *header.ActiveLayerDepth > 0)) && snrClass > 0 && snrClass < 5 {
		snrClass--
		snrLabel = snrClassLetter(snrClass)
		res.SNRTrace = append(res.SNRTrace, KeyTraceStep{"9.6*", "Permafrost adjustment", "Permafrost / active layer present -> lower SNR by 1 class", snrLabel})
	}

	res.KeyedSNR = snrLabel
	res.KeyedSNRClass = &snrClass
	if fieldClass, ok := parseFieldSNRClass(res.FieldSNR); ok {
		res.FieldSNRClass = fieldClass
		diff := *fieldClass - snrClass
		if diff < 0 {
			diff = -diff
		}
		res.SNRDivergence = &diff
		if diff >= 2 {
			res.SNRDivergent = true
			res.Issues = append(res.Issues, DiagnosticIssue{
				Code:      "SNR_KEY_DIVERGENCE",
				Severity:  SeverityWarning,
				Category:  "edatopic_snr",
				Field:     "NutrientRegime",
				Message:   fmt.Sprintf("Field NutrientRegime (%s) diverges by %d classes from LMH25 Key 9.6 inference (%s).", res.FieldSNR, diff, snrLabel),
				Suggested: snrClassLetter(snrClass),
				LMH25Ref:  "LMH25 (2023) Key 9.6 p. 228",
			})
		}
	}
}

func classifyHumusFormFamily(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "L", "VL", "YL":
		return "MULL"
	case "D", "LD", "MD", "RD", "SD", "TD", "YD":
		return "MODER"
	case "R", "FR", "HR", "LR", "MR", "RR", "UR", "YR":
		return "MOR"
	default:
		return ""
	}
}

func EvaluateStoredPlotDiagnostics(s *vprocore.ContextService, ctx context.Context, contextID, plotNumber string) (PlotDiagnosticsResult, error) {
	header, err := s.GetPlot(ctx, contextID, plotNumber)
	if err != nil {
		return PlotDiagnosticsResult{}, err
	}
	humus, err := s.ListHumusRecords(ctx, contextID, plotNumber)
	if err != nil {
		return PlotDiagnosticsResult{}, err
	}
	mineral, err := s.ListMineralRecords(ctx, contextID, plotNumber)
	if err != nil {
		return PlotDiagnosticsResult{}, err
	}
	return EvaluatePlotDiagnostics(*header, humus, mineral), nil
}
