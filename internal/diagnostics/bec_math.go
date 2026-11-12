package diagnostics

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
)

var crosstabCodePattern = regexp.MustCompile(`^[1-5] - (\+|[1-9])$`)

// NarrowSingleCover narrows a float64 cover input to IEEE-754 SINGLE (float32)
// prior to evaluating Access's `<100 Or Is Null` cover rule (§4 invariant).
func NarrowSingleCover(value float64) (float32, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	narrowed := float32(value)
	if math.IsNaN(float64(narrowed)) || math.IsInf(float64(narrowed), 0) || narrowed < 0 || narrowed >= 100 {
		return narrowed, false
	}
	return narrowed, true
}

// FormatACE15 formats a float64 via Access/DAO 15-significant-digit half-away rounding.
func FormatACE15(value float64, decimals int) string {
	return reportcore.SiteUnitFixedNumber(value, decimals, 1)
}

// RoundACE15 returns the numeric float64 value after ACE 15-significant-digit half-away rounding.
func RoundACE15(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	parsed, err := strconv.ParseFloat(FormatACE15(value, decimals), 64)
	if err != nil {
		return value
	}
	return parsed
}

// DiagnosticPresenceClass maps a unit presence ratio in [0, 1] to (1..5, "I".."V")
// matching V7mdlDiagnostic.DiagnosticPresenceClass & V7mdlValueConversions.Presence2Class.
func DiagnosticPresenceClass(ratio float64) (int, string) {
	if ratio <= 0.2 {
		return 1, "I"
	}
	if ratio <= 0.4 {
		return 2, "II"
	}
	if ratio <= 0.6 {
		return 3, "III"
	}
	if ratio <= 0.8 {
		return 4, "IV"
	}
	return 5, "V"
}

// DominKrajinaSignifClass maps a mean cover percentage to Domin-Krajina Significance Class
// ("+", "1".."9") and its numeric Diagnostic weight (0 for "+", 1..9 otherwise),
// matching V7mdlValueConversions.SignifClass.
func DominKrajinaSignifClass(signif float64) (string, int) {
	switch {
	case signif <= 0.3:
		return "+", 0
	case signif <= 1.0:
		return "1", 1
	case signif <= 2.2:
		return "2", 2
	case signif <= 5.0:
		return "3", 3
	case signif <= 10.0:
		return "4", 4
	case signif <= 20.0:
		return "5", 5
	case signif <= 33.0:
		return "6", 6
	case signif <= 50.0:
		return "7", 7
	case signif <= 75.0:
		return "8", 8
	default:
		return "9", 9
	}
}

// FormatUnitCrosstabCode formats the Access MyRS5 crosstab string ("1 - +" .. "5 - 9").
func FormatUnitCrosstabCode(presenceClass int, signifClass string) string {
	if presenceClass < 1 || presenceClass > 5 || signifClass == "" {
		return ""
	}
	return fmt.Sprintf("%d - %s", presenceClass, signifClass)
}

// ClassifySpeciesDiagnostic evaluates BEC differential & diagnostic categories across ordered units,
// matching V7mdlDiagnostic.Diagnostic and vpro_diagnostic_classify(unit_codes) exactly.
func ClassifySpeciesDiagnostic(orderedUnits []OrderedUnitCode) (DiagnosticClassification, error) {
	if len(orderedUnits) == 0 {
		return DiagnosticClassification{}, errors.New("ordered unit codes must be non-empty")
	}
	seenUnits := make(map[string]struct{}, len(orderedUnits))
	type presentEntry struct {
		unit         string
		presence     int
		significance int
	}
	present := make([]presentEntry, 0, len(orderedUnits))

	for _, item := range orderedUnits {
		if item.Unit == "" {
			return DiagnosticClassification{}, errors.New("ordered unit codes must have non-empty unit names")
		}
		if _, dup := seenUnits[item.Unit]; dup {
			return DiagnosticClassification{}, fmt.Errorf("duplicate unit %q in ordered unit codes", item.Unit)
		}
		seenUnits[item.Unit] = struct{}{}
		if item.Code == nil {
			continue
		}
		raw := *item.Code
		if !crosstabCodePattern.MatchString(raw) {
			return DiagnosticClassification{}, fmt.Errorf("invalid unit crosstab code %q for unit %q; expected '1 - +' through '5 - 9'", raw, item.Unit)
		}
		pres := int(raw[0] - '0')
		sigStr := raw[4:]
		sig := 0
		if sigStr != "+" {
			sig = int(sigStr[0] - '0')
		}
		present = append(present, presentEntry{
			unit:         item.Unit,
			presence:     pres,
			significance: sig,
		})
	}

	if len(present) == 0 {
		return DiagnosticClassification{}, nil
	}

	maximum := 0
	selected := 0
	for i, p := range present {
		if p.presence > maximum {
			maximum = p.presence
			selected = i
		}
	}
	selectedSignificance := present[selected].significance

	countPresence := 0
	countFive := 0
	countSignificance := 0
	for _, p := range present {
		if p.presence > maximum-2 {
			countPresence++
		}
		if p.presence == 5 {
			countFive++
		}
		if p.significance > selectedSignificance-2 {
			countSignificance++
		}
	}

	differential := countPresence == 1 && maximum >= 3
	dominantDifferential := countSignificance == 1 && !differential && selectedSignificance >= 5
	constantDominant := maximum == 5 && countPresence == 1 && selectedSignificance >= 5
	constant := maximum == 5 && countFive == 1 && selectedSignificance < 5
	importantCompanion := !differential && !dominantDifferential && !constantDominant && !constant && maximum >= 2 && countPresence == 1

	var codes []string
	if differential {
		codes = append(codes, "d")
	}
	if dominantDifferential {
		codes = append(codes, "dd")
	}
	if constantDominant {
		codes = append(codes, "cd")
	}
	if constant {
		codes = append(codes, "c")
	}
	if importantCompanion {
		codes = append(codes, "ic")
	}
	if len(codes) == 0 {
		return DiagnosticClassification{}, nil
	}
	return DiagnosticClassification{
		Unit:      present[selected].unit,
		Diagnosis: strings.Join(codes, ", "),
		Codes:     codes,
	}, nil
}

func normalizePairwiseParams(in PairwiseParams) PairwiseParams {
	out := in
	if out.MinConstancy <= 0 {
		out.MinConstancy = 50.0
	}
	if out.NoiseConstancy <= 0 {
		out.NoiseConstancy = 10.0
	}
	if out.MinPlots <= 0 {
		out.MinPlots = 1
	}
	if out.MinDiff <= 0 {
		out.MinDiff = 0.1
	}
	if out.MinDD <= 0 {
		out.MinDD = 0.35
	}
	if out.MinorCover <= 0 {
		out.MinorCover = 1.0
	}
	if out.DominantCover <= 0 {
		out.DominantCover = 7.0
	}
	return out
}

// DiagnosticPotentialScores captures BEC _calc_diagnostic_potential.R components.
type DiagnosticPotentialScores struct {
	DPotential          float64
	DDPotential         float64
	DiagnosticPotential float64
}

// CalcDiagnosticPotential ports _calc_diagnostic_potential.R from MoF BEC_R_working.
func CalcDiagnosticPotential(constancy, meanCov float64, params PairwiseParams) DiagnosticPotentialScores {
	p := normalizePairwiseParams(params)
	const constExp = 0.5
	const coverExp = 0.333
	const maxPotential = 10.0

	dPot := 0.0
	if constancy >= p.MinConstancy && 100.0 > p.MinConstancy {
		dPot = 4.0 * math.Pow((constancy-p.MinConstancy)/(100.0-p.MinConstancy), constExp)
	}
	ddPot := 0.0
	if constancy >= p.MinConstancy && meanCov >= p.DominantCover && 100.0 > p.DominantCover {
		ddPot = 4.0 * math.Pow((meanCov-p.DominantCover)/(100.0-p.DominantCover), coverExp)
	}
	coverWeight := 1.0
	if meanCov < p.MinorCover {
		if meanCov <= 0 {
			coverWeight = 0
		} else {
			coverWeight = math.Sqrt(meanCov / p.MinorCover)
		}
	}
	dPotAdj := dPot * coverWeight
	bonus := 0.0
	if dPotAdj > 0 && meanCov >= p.DominantCover {
		ratio := dPotAdj / 4.0
		bonus = 2.0 * ratio * ratio
	}
	diagPot := math.Min(dPotAdj+ddPot+bonus, maxPotential)
	if diagPot <= p.MinDiff {
		diagPot = 0
	}
	return DiagnosticPotentialScores{
		DPotential:          dPot,
		DDPotential:         ddPot,
		DiagnosticPotential: diagPot,
	}
}

type unitSpeciesStat struct {
	presentPlots int
	totalCover   float64
	meanCoverAll float64
	meanCoverPr  float64
	constancyPct float64
	diagPot      float64
}

type classifiedPlotRecord struct {
	plotNumber     string
	fieldNumber    string
	siteUnit       string
	zone           string
	subZone        string
	siteSeries     string
	userSiteUnit   string
	moistureRegime string
	nutrientRegime string
	speciesCovers  map[string]float64
	totalCover     float64
}

// SorensenSimilarity computes the BEC Diagnostic10.xla / Sorensen / Bray-Curtis quantitative similarity
// 2 * sum(min(u1, u2)) / sum(u1 + u2) in [0, 1].
func SorensenSimilarity(u1, u2 map[string]float64) float64 {
	sumMin := 0.0
	sumTot := 0.0
	for sp, v1 := range u1 {
		if v1 > 0 {
			sumTot += v1
			if v2 := u2[sp]; v2 > 0 {
				if v1 < v2 {
					sumMin += v1
				} else {
					sumMin += v2
				}
			}
		}
	}
	for _, v2 := range u2 {
		if v2 > 0 {
			sumTot += v2
		}
	}
	if sumTot <= 0 {
		return 0
	}
	return (2.0 * sumMin) / sumTot
}

func computePairwiseDPoints(constSelf, constOther, meanSelf float64, p PairwiseParams) float64 {
	constDiff := constSelf - constOther
	if constDiff < 20.0 || constSelf < p.MinConstancy {
		return 0
	}
	constScore := math.Cbrt(constDiff) * (constDiff / 100.0)
	pts := 0.0
	if constSelf >= p.MinConstancy+10.0 {
		pts = constScore
	} else {
		weight := math.Max(0, math.Min(1.0, (constSelf-p.MinConstancy)/10.0))
		pts = constScore * weight
	}
	if meanSelf <= p.MinorCover {
		pts *= meanSelf / p.MinorCover
	}
	if constDiff < 40.0 {
		pts *= (constDiff - 20.0) / 20.0
	}
	if pts < p.MinDiff {
		return 0
	}
	return pts
}

func computePairwiseDDPoints(meanSelf, meanOther float64, p PairwiseParams) float64 {
	if meanSelf <= meanOther || meanSelf <= 5.0 {
		return 0
	}
	ddiff := math.Abs(math.Cbrt(meanSelf) - math.Cbrt(meanOther))
	pts := 0.0
	if meanSelf >= 10.0 {
		pts = ddiff
	} else {
		pts = ddiff * ((meanSelf - 5.0) / 5.0)
	}
	if pts <= p.MinDD {
		return 0
	}
	return pts
}

// ComputePairwiseAnalysis builds both the symmetric U x U Sorensen similarity matrix
// and the BEC _do_pairwise.R differential comparison list.
func ComputePairwiseAnalysis(
	units []string,
	plotCounts map[string]int,
	speciesMeta map[string]SpeciesTaxonMeta,
	unitSpecies map[string]map[string]unitSpeciesStat,
	rawParams PairwiseParams,
) ([]PairwiseMatrixRow, []PairwiseUnitComparison) {
	p := normalizePairwiseParams(rawParams)
	nUnits := len(units)

	meanVecs := make(map[string]map[string]float64, nUnits)
	for _, u := range units {
		vec := make(map[string]float64, len(unitSpecies[u]))
		for sp, st := range unitSpecies[u] {
			if st.meanCoverAll > 0 {
				vec[sp] = st.meanCoverAll
			}
		}
		meanVecs[u] = vec
	}

	// Filter species attaining >= p.MinConstancy in at least one eligible unit for do_pairwise
	eligiblePairwiseSpecies := map[string]bool{}
	for _, u := range units {
		if plotCounts[u] < p.MinPlots {
			continue
		}
		for sp, st := range unitSpecies[u] {
			if st.constancyPct >= p.MinConstancy {
				eligiblePairwiseSpecies[sp] = true
			}
		}
	}

	sorensenMat := make([][]float64, nUnits)
	becSimMat := make([][]float64, nUnits)
	for i := range nUnits {
		sorensenMat[i] = make([]float64, nUnits)
		becSimMat[i] = make([]float64, nUnits)
		sorensenMat[i][i] = 1.0
		becSimMat[i][i] = 1.0
	}

	comparisons := make([]PairwiseUnitComparison, 0, nUnits*(nUnits-1)/2)

	for i := 0; i < nUnits; i++ {
		u1 := units[i]
		for j := i + 1; j < nUnits; j++ {
			u2 := units[j]
			sor := SorensenSimilarity(meanVecs[u1], meanVecs[u2])
			sorensenMat[i][j] = sor
			sorensenMat[j][i] = sor

			// Evaluate BEC do_pairwise differential comparison between u1 and u2
			unionSpp := map[string]struct{}{}
			if plotCounts[u1] >= p.MinPlots {
				for sp, st := range unitSpecies[u1] {
					if eligiblePairwiseSpecies[sp] && st.constancyPct > p.NoiseConstancy {
						unionSpp[sp] = struct{}{}
					}
				}
			}
			if plotCounts[u2] >= p.MinPlots {
				for sp, st := range unitSpecies[u2] {
					if eligiblePairwiseSpecies[sp] && st.constancyPct > p.NoiseConstancy {
						unionSpp[sp] = struct{}{}
					}
				}
			}

			sharedDiag := 0.0
			diffTot1 := 0.0
			diffTot2 := 0.0
			diffTaxa := make([]PairwiseDifferentialTaxon, 0)

			for sp := range unionSpp {
				var st1, st2 unitSpeciesStat
				if plotCounts[u1] >= p.MinPlots {
					if cand, ok := unitSpecies[u1][sp]; ok && cand.constancyPct > p.NoiseConstancy {
						st1 = cand
					}
				}
				if plotCounts[u2] >= p.MinPlots {
					if cand, ok := unitSpecies[u2][sp]; ok && cand.constancyPct > p.NoiseConstancy {
						st2 = cand
					}
				}
				if st1.diagPot <= 0 && st2.diagPot <= 0 {
					continue
				}
				sharedDiag += math.Min(st1.diagPot, st2.diagPot)

				d1 := computePairwiseDPoints(st1.constancyPct, st2.constancyPct, st1.meanCoverAll, p)
				dd1 := computePairwiseDDPoints(st1.meanCoverAll, st2.meanCoverAll, p)
				pts1 := d1 + dd1
				if d1 > p.MinDiff && dd1 > p.MinDiff {
					pts1 *= 1.25
				}
				pts1 = math.Min(pts1, 10.0)

				d2 := computePairwiseDPoints(st2.constancyPct, st1.constancyPct, st2.meanCoverAll, p)
				dd2 := computePairwiseDDPoints(st2.meanCoverAll, st1.meanCoverAll, p)
				pts2 := d2 + dd2
				if d2 > p.MinDiff && dd2 > p.MinDiff {
					pts2 *= 1.25
				}
				pts2 = math.Min(pts2, 10.0)

				diffTot1 += pts1
				diffTot2 += pts2

				if pts1 > 0 || pts2 > 0 {
					favours := u1
					if pts2 > pts1 {
						favours = u2
					}
					diffTaxa = append(diffTaxa, PairwiseDifferentialTaxon{
						Species:        sp,
						ScientificName: speciesMeta[sp].ScientificName,
						ConstancyUnit1: RoundACE15(st1.constancyPct, 1),
						MeanCoverUnit1: RoundACE15(st1.meanCoverAll, 2),
						ConstancyUnit2: RoundACE15(st2.constancyPct, 1),
						MeanCoverUnit2: RoundACE15(st2.meanCoverAll, 2),
						DiffPtsUnit1:   RoundACE15(pts1, 2),
						DiffPtsUnit2:   RoundACE15(pts2, 2),
						FavoursUnit:    favours,
					})
				}
			}

			slices.SortFunc(diffTaxa, func(a, b PairwiseDifferentialTaxon) int {
				maxA := math.Max(a.DiffPtsUnit1, a.DiffPtsUnit2)
				maxB := math.Max(b.DiffPtsUnit1, b.DiffPtsUnit2)
				if maxA > maxB {
					return -1
				}
				if maxA < maxB {
					return 1
				}
				return strings.Compare(a.Species, b.Species)
			})

			diffTot := diffTot1 + diffTot2
			diffRatio := 0.0
			if denom := diffTot + 2.0*sharedDiag; denom > 0 {
				diffRatio = (2.0 * sharedDiag) / denom
			}
			ratio1 := 0.0
			if denom1 := diffTot1 + sharedDiag; denom1 > 0 {
				ratio1 = sharedDiag / denom1
			}
			ratio2 := 0.0
			if denom2 := diffTot2 + sharedDiag; denom2 > 0 {
				ratio2 = sharedDiag / denom2
			}
			becMean := (ratio1 + ratio2) / 2.0
			becMin := math.Min(ratio1, ratio2)
			becMax := math.Max(ratio1, ratio2)

			becSimMat[i][j] = becMean
			becSimMat[j][i] = becMean

			comparisons = append(comparisons, PairwiseUnitComparison{
				Unit1:               u1,
				Unit2:               u2,
				PlotsUnit1:          plotCounts[u1],
				PlotsUnit2:          plotCounts[u2],
				SorensenSimilarity:  RoundACE15(sor, 4),
				SorensenPct:         RoundACE15(sor*100.0, 1),
				SharedDiagPotential: RoundACE15(sharedDiag, 2),
				DiffTotalUnit1:      RoundACE15(diffTot1, 2),
				DiffTotalUnit2:      RoundACE15(diffTot2, 2),
				DiffRatio:           RoundACE15(diffRatio, 4),
				BECSimMin:           RoundACE15(becMin, 4),
				BECSimMean:          RoundACE15(becMean, 4),
				BECSimMax:           RoundACE15(becMax, 4),
				DifferentialTaxa:    diffTaxa,
			})
		}
	}

	matrixRows := make([]PairwiseMatrixRow, nUnits)
	for i, uRow := range units {
		cells := make([]PairwiseMatrixCell, nUnits)
		for j, uCol := range units {
			sor := sorensenMat[i][j]
			sorPct := sor * 100.0
			bec := becSimMat[i][j]
			cells[j] = PairwiseMatrixCell{
				Unit:              uCol,
				Similarity:        RoundACE15(sor, 4),
				SimilarityPct:     RoundACE15(sorPct, 1),
				SimilarityPctText: FormatACE15(sorPct, 1),
				BECSimMean:        RoundACE15(bec, 4),
				BECSimMeanPct:     RoundACE15(bec*100.0, 1),
			}
		}
		matrixRows[i] = PairwiseMatrixRow{
			Unit:  uRow,
			Cells: cells,
		}
	}

	return matrixRows, comparisons
}

// ComputeOutlierQueue evaluates every classified plot against its Leave-One-Out (LOO) assigned SiteUnit
// centroid and its nearest rival SiteUnit centroid, sorting descending by RivalSimilarity - OwnSimilarity.
func ComputeOutlierQueue(
	units []string,
	plots []classifiedPlotRecord,
	unitCoverSums map[string]map[string]float64,
	unitTotalSums map[string]float64,
	unitPlotCounts map[string]int,
) []OutlierPlotCandidate {
	if len(plots) == 0 {
		return []OutlierPlotCandidate{}
	}

	out := make([]OutlierPlotCandidate, 0, len(plots))
	for _, p := range plots {
		uAssigned := p.siteUnit
		nAssigned := unitPlotCounts[uAssigned]
		singleton := nAssigned <= 1

		ownSim := 0.0
		if !singleton {
			nLOO := float64(nAssigned - 1)
			looTotal := (unitTotalSums[uAssigned] - p.totalCover) / nLOO
			if looTotal < 0 {
				looTotal = 0
			}
			denom := p.totalCover + looTotal
			if denom > 0 {
				uSums := unitCoverSums[uAssigned]
				sumMin := 0.0
				for sp, cov := range p.speciesCovers {
					if cov <= 0 {
						continue
					}
					looMean := (uSums[sp] - cov) / nLOO
					if looMean > 0 {
						if cov < looMean {
							sumMin += cov
						} else {
							sumMin += looMean
						}
					}
				}
				ownSim = (2.0 * sumMin) / denom
			}
		}

		rivalUnit := ""
		rivalSim := 0.0
		for _, candUnit := range units {
			if candUnit == uAssigned {
				continue
			}
			nCand := unitPlotCounts[candUnit]
			if nCand <= 0 {
				continue
			}
			candMeanTotal := unitTotalSums[candUnit] / float64(nCand)
			denom := p.totalCover + candMeanTotal
			sim := 0.0
			if denom > 0 {
				cSums := unitCoverSums[candUnit]
				sumMin := 0.0
				for sp, cov := range p.speciesCovers {
					if cov <= 0 {
						continue
					}
					candMean := cSums[sp] / float64(nCand)
					if candMean > 0 {
						if cov < candMean {
							sumMin += cov
						} else {
							sumMin += candMean
						}
					}
				}
				sim = (2.0 * sumMin) / denom
			}
			if rivalUnit == "" || sim > rivalSim || (sim == rivalSim && candUnit < rivalUnit) {
				rivalUnit = candUnit
				rivalSim = sim
			}
		}

		delta := rivalSim - ownSim
		out = append(out, OutlierPlotCandidate{
			PlotNumber:         p.plotNumber,
			FieldNumber:        p.fieldNumber,
			AssignedUnit:       uAssigned,
			AssignedUnitPlots:  nAssigned,
			SingletonUnit:      singleton,
			OwnSimilarity:      RoundACE15(ownSim, 4),
			OwnSimilarityPct:   RoundACE15(ownSim*100.0, 1),
			NearestRivalUnit:   rivalUnit,
			RivalSimilarity:    RoundACE15(rivalSim, 4),
			RivalSimilarityPct: RoundACE15(rivalSim*100.0, 1),
			DeltaSimilarity:    RoundACE15(delta, 4),
			DeltaSimilarityPct: RoundACE15(delta*100.0, 1),
			Misplaced:          rivalUnit != "" && delta > 0,
			MoistureRegime:     p.moistureRegime,
			NutrientRegime:     p.nutrientRegime,
			Zone:               p.zone,
			SubZone:            p.subZone,
			SiteSeries:         p.siteSeries,
		})
	}

	slices.SortFunc(out, func(a, b OutlierPlotCandidate) int {
		if a.DeltaSimilarity > b.DeltaSimilarity {
			return -1
		}
		if a.DeltaSimilarity < b.DeltaSimilarity {
			return 1
		}
		if a.OwnSimilarity < b.OwnSimilarity {
			return -1
		}
		if a.OwnSimilarity > b.OwnSimilarity {
			return 1
		}
		return strings.Compare(a.PlotNumber, b.PlotNumber)
	})

	top5Cutoff := max(1, int(math.Ceil(float64(len(out))*0.05)))
	for i := range out {
		out[i].Rank = i + 1
		out[i].Top5PercentAnomaly = (i + 1) <= top5Cutoff
	}
	return out
}

// ParseRelativeSMR extracts a canonical BEC rSMR class ("0".."8") from an Env MoistureRegime field.
func ParseRelativeSMR(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	ch := trimmed[0]
	if ch >= '0' && ch <= '8' {
		// Reject multi-digit integers like "10" while allowing modifiers like "4w" or "3-4"
		if len(trimmed) > 1 && trimmed[1] >= '0' && trimmed[1] <= '9' {
			return "", false
		}
		return string(ch), true
	}
	return "", false
}

// ParseRelativeSNR extracts a canonical BEC rSNR class ("A".."F") from an Env NutrientRegime field.
func ParseRelativeSNR(raw string) (string, bool) {
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", false
	}
	ch := trimmed[0]
	if ch >= 'A' && ch <= 'F' {
		if len(trimmed) > 1 && trimmed[1] >= 'A' && trimmed[1] <= 'Z' {
			return "", false
		}
		return string(ch), true
	}
	return "", false
}

func buildSingleEdatopicGrid(siteUnit, label string, plots []classifiedPlotRecord) EdatopicUnitGrid {
	type cellBucket struct {
		plots []string
	}
	grid := make([][]cellBucket, len(SMRRows))
	for r := range SMRRows {
		grid[r] = make([]cellBucket, len(SNRCols))
	}

	plotted := 0
	for _, p := range plots {
		smr, okM := ParseRelativeSMR(p.moistureRegime)
		snr, okN := ParseRelativeSNR(p.nutrientRegime)
		if !okM || !okN {
			continue
		}
		rIdx := int(smr[0] - '0')
		cIdx := int(snr[0] - 'A')
		grid[rIdx][cIdx].plots = append(grid[rIdx][cIdx].plots, p.plotNumber)
		plotted++
	}

	total := len(plots)
	cells := make([]EdatopicCell, 0, len(SMRRows)*len(SNRCols))
	maxCount := 0
	maxProm := 0.0

	for rIdx, smr := range SMRRows {
		for cIdx, snr := range SNRCols {
			pList := grid[rIdx][cIdx].plots
			if pList == nil {
				pList = []string{}
			} else {
				slices.Sort(pList)
			}
			cnt := len(pList)
			unitPct := 0.0
			if total > 0 {
				unitPct = RoundACE15((float64(cnt)/float64(total))*100.0, 1)
			}
			prom := 0.0
			if plotted > 0 {
				prom = RoundACE15((float64(cnt)/float64(plotted))*100.0, 1)
			}
			if cnt > maxCount {
				maxCount = cnt
			}
			if prom > maxProm {
				maxProm = prom
			}
			cells = append(cells, EdatopicCell{
				SMR:         smr,
				SNR:         snr,
				PlotCount:   cnt,
				UnitPct:     unitPct,
				Prominence:  prom,
				PlotNumbers: pList,
			})
		}
	}

	return EdatopicUnitGrid{
		SiteUnit:             siteUnit,
		Label:                label,
		TotalPlots:           total,
		PlottedEdatopicCount: plotted,
		MissingEdatopicCount: total - plotted,
		MaxCellCount:         maxCount,
		MaxProminence:        maxProm,
		Cells:                cells,
	}
}

// ComputeEdatopicGrids computes the Overall and per-SiteUnit 9 x 6 (rSMR 0..8 x rSNR A..F) heatmaps.
func ComputeEdatopicGrids(units []string, plots []classifiedPlotRecord) (EdatopicUnitGrid, []EdatopicUnitGrid) {
	overall := buildSingleEdatopicGrid("__OVERALL__", "All Classified Plots", plots)
	byUnit := make(map[string][]classifiedPlotRecord, len(units))
	for _, p := range plots {
		byUnit[p.siteUnit] = append(byUnit[p.siteUnit], p)
	}
	unitGrids := make([]EdatopicUnitGrid, 0, len(units))
	for _, u := range units {
		unitGrids = append(unitGrids, buildSingleEdatopicGrid(u, u, byUnit[u]))
	}
	return overall, unitGrids
}

// LifeformLabel maps a VPro numeric Lifeform code (1..13) to its BEC display name and 4-stratum group.
func LifeformLabel(code *int) (string, string) {
	if code == nil {
		return "Unclassified", "Unclassified"
	}
	switch *code {
	case 1:
		return "Conifer Tree", "Tree"
	case 2:
		return "Deciduous Tree", "Tree"
	case 3:
		return "Evergreen Shrub", "Shrub"
	case 4:
		return "Deciduous Shrub", "Shrub"
	case 5:
		return "Fern", "Herb"
	case 6:
		return "Graminoid", "Herb"
	case 7:
		return "Forb", "Herb"
	case 8:
		return "Parasitic", "Herb"
	case 9:
		return "Moss", "Moss"
	case 10:
		return "Liverwort", "Moss"
	case 11:
		return "Lichen", "Moss"
	case 12:
		return "Dwarf Woody Plant", "Shrub"
	case 13:
		return "Macro Algae", "Moss"
	default:
		return fmt.Sprintf("Lifeform %d", *code), "Unclassified"
	}
}

func stratumSortRank(stratum string) int {
	switch stratum {
	case "Tree":
		return 1
	case "Shrub":
		return 2
	case "Herb":
		return 3
	case "Moss":
		return 4
	default:
		return 5
	}
}

func wildcardMatchLower(pattern, target string) bool {
	p := []rune(pattern)
	s := []rune(target)
	pi, si := 0, 0
	starIdx, matchIdx := -1, 0
	for si < len(s) {
		if pi < len(p) && (p[pi] == '?' || p[pi] == s[si]) {
			pi++
			si++
		} else if pi < len(p) && p[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// MatchUnitExcluded checks whether a SiteUnit matches BEC standard exclusions (.support/.poor/.nudum)
// or custom exclusion patterns.
func MatchUnitExcluded(siteUnit string, excludeStandard bool, customPatterns []string) (bool, string) {
	trimmed := strings.TrimSpace(siteUnit)
	if trimmed == "" {
		return false, ""
	}
	lowerUnit := strings.ToLower(trimmed)

	var patterns []string
	if excludeStandard {
		patterns = append(patterns, DefaultExcludedUnitModifiers...)
	}
	patterns = append(patterns, customPatterns...)

	for _, rawPat := range patterns {
		pat := strings.TrimSpace(rawPat)
		if pat == "" {
			continue
		}
		lowerPat := strings.ToLower(pat)
		if strings.ContainsAny(lowerPat, "*?") {
			if wildcardMatchLower(lowerPat, lowerUnit) {
				return true, rawPat
			}
			continue
		}
		if strings.HasPrefix(lowerPat, ".") {
			if strings.HasSuffix(lowerUnit, lowerPat) ||
				strings.Contains(lowerUnit, lowerPat+".") ||
				strings.Contains(lowerUnit, lowerPat+"-") ||
				strings.Contains(lowerUnit, lowerPat+"_") ||
				strings.Contains(lowerUnit, lowerPat+" ") {
				return true, rawPat
			}
			continue
		}
		if strings.Contains(lowerUnit, lowerPat) {
			return true, rawPat
		}
	}
	return false, ""
}
