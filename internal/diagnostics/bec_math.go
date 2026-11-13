package diagnostics

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
)

var pow10Uint64 = [20]uint64{
	1, 10, 100, 1000, 10000,
	100000, 1000000, 10000000, 100000000, 1000000000,
	10000000000, 100000000000, 1000000000000, 10000000000000, 100000000000000,
	1000000000000000, 10000000000000000, 100000000000000000, 1000000000000000000, 10000000000000000000,
}

var crosstabCodeTable = [6][10]string{
	{},
	{"1 - +", "1 - 1", "1 - 2", "1 - 3", "1 - 4", "1 - 5", "1 - 6", "1 - 7", "1 - 8", "1 - 9"},
	{"2 - +", "2 - 1", "2 - 2", "2 - 3", "2 - 4", "2 - 5", "2 - 6", "2 - 7", "2 - 8", "2 - 9"},
	{"3 - +", "3 - 1", "3 - 2", "3 - 3", "3 - 4", "3 - 5", "3 - 6", "3 - 7", "3 - 8", "3 - 9"},
	{"4 - +", "4 - 1", "4 - 2", "4 - 3", "4 - 4", "4 - 5", "4 - 6", "4 - 7", "4 - 8", "4 - 9"},
	{"5 - +", "5 - 1", "5 - 2", "5 - 3", "5 - 4", "5 - 5", "5 - 6", "5 - 7", "5 - 8", "5 - 9"},
}

// ace15ScaledUint64 computes round_half_away(15_sig_digits(|value|) * 10^decimals) on the stack
// without allocating math/big objects when the scaled integer fits comfortably in uint64.
func ace15ScaledUint64(value float64, decimals int) (scaled uint64, negative bool, ok bool) {
	if decimals < 0 || decimals > 9 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false, false
	}
	if value == 0 {
		return 0, false, true
	}
	negative = value < 0
	absVal := math.Abs(value)

	var sci [32]byte
	b := strconv.AppendFloat(sci[:0], absVal, 'e', 14, 64)
	// Format is d.ddddddddddddddE±dd (e.g. "1.00500000000000e+00", len >= 20)
	eIdx := -1
	for i := 0; i < len(b); i++ {
		if b[i] == 'e' || b[i] == 'E' {
			eIdx = i
			break
		}
	}
	if eIdx != 16 || b[1] != '.' {
		return 0, false, false
	}
	mant := uint64(b[0] - '0')
	for i := 2; i < 16; i++ {
		mant = mant*10 + uint64(b[i]-'0')
	}
	expSign := 1
	expPos := eIdx + 1
	if expPos >= len(b) {
		return 0, false, false
	}
	if b[expPos] == '-' {
		expSign = -1
		expPos++
	} else if b[expPos] == '+' {
		expPos++
	}
	expVal := 0
	for ; expPos < len(b); expPos++ {
		if b[expPos] < '0' || b[expPos] > '9' {
			return 0, false, false
		}
		expVal = expVal*10 + int(b[expPos]-'0')
	}
	exp := expSign * expVal

	k := exp - 14 + decimals
	if k >= 0 {
		if k > 4 {
			return 0, false, false
		}
		return mant * pow10Uint64[k], negative, true
	}
	negK := -k
	if negK > 16 {
		return 0, false, true
	}
	div := pow10Uint64[negK]
	q := mant / div
	rem := mant % div
	if rem >= (div+1)/2 && (div%2 == 0 && rem*2 >= div || div%2 != 0) {
		q++
	}
	if q == 0 {
		negative = false
	}
	return q, negative, true
}

func formatScaledDecimalBytes(dst []byte, scaled uint64, negative bool, decimals int) []byte {
	if negative && scaled != 0 {
		dst = append(dst, '-')
	}
	if decimals == 0 {
		return strconv.AppendUint(dst, scaled, 10)
	}
	var digits [24]byte
	raw := strconv.AppendUint(digits[:0], scaled, 10)
	if len(raw) <= decimals {
		dst = append(dst, '0', '.')
		for i := 0; i < decimals-len(raw); i++ {
			dst = append(dst, '0')
		}
		dst = append(dst, raw...)
		return dst
	}
	split := len(raw) - decimals
	dst = append(dst, raw[:split]...)
	dst = append(dst, '.')
	dst = append(dst, raw[split:]...)
	return dst
}

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
	if scaled, neg, ok := ace15ScaledUint64(value, decimals); ok {
		var buf [32]byte
		return string(formatScaledDecimalBytes(buf[:0], scaled, neg, decimals))
	}
	return reportcore.SiteUnitFixedNumber(value, decimals, 1)
}

// RoundACE15 returns the numeric float64 value after ACE 15-significant-digit half-away rounding.
func RoundACE15(value float64, decimals int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if scaled, neg, ok := ace15ScaledUint64(value, decimals); ok {
		if scaled == 0 {
			return 0
		}
		if decimals == 0 {
			res := float64(scaled)
			if neg {
				return -res
			}
			return res
		}
		var buf [32]byte
		s := formatScaledDecimalBytes(buf[:0], scaled, neg, decimals)
		if parsed, err := strconv.ParseFloat(string(s), 64); err == nil {
			return parsed
		}
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
	if presenceClass < 1 || presenceClass > 5 || len(signifClass) != 1 {
		if presenceClass < 1 || presenceClass > 5 || signifClass == "" {
			return ""
		}
		return fmt.Sprintf("%d - %s", presenceClass, signifClass)
	}
	c := signifClass[0]
	if c == '+' {
		return crosstabCodeTable[presenceClass][0]
	}
	if c >= '1' && c <= '9' {
		return crosstabCodeTable[presenceClass][c-'0']
	}
	return fmt.Sprintf("%d - %s", presenceClass, signifClass)
}

func parseCrosstabCodeFast(raw string) (pres int, sig int, ok bool) {
	if len(raw) != 5 || raw[1] != ' ' || raw[2] != '-' || raw[3] != ' ' {
		return 0, 0, false
	}
	if raw[0] < '1' || raw[0] > '5' {
		return 0, 0, false
	}
	pres = int(raw[0] - '0')
	if raw[4] == '+' {
		return pres, 0, true
	}
	if raw[4] >= '1' && raw[4] <= '9' {
		return pres, int(raw[4] - '0'), true
	}
	return 0, 0, false
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
		pres, sig, valid := parseCrosstabCodeFast(raw)
		if !valid {
			return DiagnosticClassification{}, fmt.Errorf("invalid unit crosstab code %q for unit %q; expected '1 - +' through '5 - 9'", raw, item.Unit)
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

	// Assign dense species IDs 0..S-1 so pairwise U*(U-1)/2 comparisons run as zero-alloc sorted slice merges.
	spSet := map[string]struct{}{}
	for _, u := range units {
		for sp := range unitSpecies[u] {
			spSet[sp] = struct{}{}
		}
	}
	spNames := make([]string, 0, len(spSet))
	for sp := range spSet {
		spNames = append(spNames, sp)
	}
	slices.Sort(spNames)
	spIDByName := make(map[string]int32, len(spNames))
	spSciByID := make([]string, len(spNames))
	eligibleByID := make([]bool, len(spNames))
	for idx, sp := range spNames {
		spIDByName[sp] = int32(idx)
		spSciByID[idx] = speciesMeta[sp].ScientificName
	}

	for _, u := range units {
		if plotCounts[u] < p.MinPlots {
			continue
		}
		for sp, st := range unitSpecies[u] {
			if st.constancyPct >= p.MinConstancy {
				eligibleByID[spIDByName[sp]] = true
			}
		}
	}

	type indexedUnitSp struct {
		spID         int32
		meanCoverAll float64
		constancyPct float64
		diagPot      float64
	}
	unitVecs := make([][]indexedUnitSp, nUnits)
	unitSumMeans := make([]float64, nUnits)
	unitEligible := make([]bool, nUnits)
	for uIdx, u := range units {
		unitEligible[uIdx] = plotCounts[u] >= p.MinPlots
		m := unitSpecies[u]
		vec := make([]indexedUnitSp, 0, len(m))
		sumMean := 0.0
		for sp, st := range m {
			if st.meanCoverAll > 0 || st.constancyPct > 0 {
				vec = append(vec, indexedUnitSp{
					spID:         spIDByName[sp],
					meanCoverAll: st.meanCoverAll,
					constancyPct: st.constancyPct,
					diagPot:      st.diagPot,
				})
				if st.meanCoverAll > 0 {
					sumMean += st.meanCoverAll
				}
			}
		}
		slices.SortFunc(vec, func(a, b indexedUnitSp) int {
			return int(a.spID - b.spID)
		})
		unitVecs[uIdx] = vec
		unitSumMeans[uIdx] = sumMean
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
		v1 := unitVecs[i]
		elig1 := unitEligible[i]
		for j := i + 1; j < nUnits; j++ {
			u2 := units[j]
			v2 := unitVecs[j]
			elig2 := unitEligible[j]

			sumMin := 0.0
			sharedDiag := 0.0
			diffTot1 := 0.0
			diffTot2 := 0.0
			var diffTaxa []PairwiseDifferentialTaxon

			k1, k2 := 0, 0
			for k1 < len(v1) || k2 < len(v2) {
				var spID int32
				var s1, s2 indexedUnitSp
				var has1, has2 bool
				if k2 >= len(v2) || (k1 < len(v1) && v1[k1].spID < v2[k2].spID) {
					s1 = v1[k1]
					spID = s1.spID
					has1 = true
					k1++
				} else if k1 >= len(v1) || v2[k2].spID < v1[k1].spID {
					s2 = v2[k2]
					spID = s2.spID
					has2 = true
					k2++
				} else {
					s1 = v1[k1]
					s2 = v2[k2]
					spID = s1.spID
					has1, has2 = true, true
					k1++
					k2++
					if s1.meanCoverAll > 0 && s2.meanCoverAll > 0 {
						if s1.meanCoverAll < s2.meanCoverAll {
							sumMin += s1.meanCoverAll
						} else {
							sumMin += s2.meanCoverAll
						}
					}
				}

				if !eligibleByID[spID] {
					continue
				}
				var st1, st2 indexedUnitSp
				inUnion := false
				if has1 && elig1 && s1.constancyPct > p.NoiseConstancy {
					st1 = s1
					inUnion = true
				}
				if has2 && elig2 && s2.constancyPct > p.NoiseConstancy {
					st2 = s2
					inUnion = true
				}
				if !inUnion || (st1.diagPot <= 0 && st2.diagPot <= 0) {
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
						Species:        spNames[spID],
						ScientificName: spSciByID[spID],
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
			if diffTaxa == nil {
				diffTaxa = []PairwiseDifferentialTaxon{}
			}

			sor := 0.0
			if sumTot := unitSumMeans[i] + unitSumMeans[j]; sumTot > 0 {
				sor = (2.0 * sumMin) / sumTot
			}
			sorensenMat[i][j] = sor
			sorensenMat[j][i] = sor

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

	nUnits := len(units)
	unitIdxByName := make(map[string]int, nUnits)
	spSet := map[string]struct{}{}
	for i, u := range units {
		unitIdxByName[u] = i
		for sp := range unitCoverSums[u] {
			spSet[sp] = struct{}{}
		}
	}
	for i := range plots {
		for sp := range plots[i].speciesCovers {
			spSet[sp] = struct{}{}
		}
	}
	nSp := len(spSet)
	spIDByName := make(map[string]int32, nSp)
	nextSpID := int32(0)
	for sp := range spSet {
		spIDByName[sp] = nextSpID
		nextSpID++
	}

	unitSumsFlat := make([]float64, nUnits*nSp)
	unitCountsArr := make([]int, nUnits)
	unitTotalsArr := make([]float64, nUnits)
	unitInvCounts := make([]float64, nUnits)
	unitMeanTotals := make([]float64, nUnits)
	for uIdx, u := range units {
		cnt := unitPlotCounts[u]
		unitCountsArr[uIdx] = cnt
		tot := unitTotalSums[u]
		unitTotalsArr[uIdx] = tot
		if cnt > 0 {
			inv := 1.0 / float64(cnt)
			unitInvCounts[uIdx] = inv
			unitMeanTotals[uIdx] = tot * inv
		}
		base := uIdx * nSp
		for sp, s := range unitCoverSums[u] {
			unitSumsFlat[base+int(spIDByName[sp])] = s
		}
	}

	type spCovPair struct {
		spID int32
		cov  float64
	}
	var plotVec []spCovPair

	out := make([]OutlierPlotCandidate, 0, len(plots))
	for i := range plots {
		p := &plots[i]
		uAssigned := p.siteUnit
		uAssignedIdx, hasAssigned := unitIdxByName[uAssigned]
		nAssigned := 0
		if hasAssigned {
			nAssigned = unitCountsArr[uAssignedIdx]
		} else {
			nAssigned = unitPlotCounts[uAssigned]
		}
		singleton := nAssigned <= 1

		plotVec = plotVec[:0]
		for sp, cov := range p.speciesCovers {
			if cov > 0 {
				plotVec = append(plotVec, spCovPair{spID: spIDByName[sp], cov: cov})
			}
		}

		ownSim := 0.0
		if !singleton && hasAssigned {
			invLOO := 1.0 / float64(nAssigned-1)
			looTotal := (unitTotalsArr[uAssignedIdx] - p.totalCover) * invLOO
			if looTotal < 0 {
				looTotal = 0
			}
			denom := p.totalCover + looTotal
			if denom > 0 {
				uBase := uAssignedIdx * nSp
				sumMin := 0.0
				for _, pair := range plotVec {
					looMean := (unitSumsFlat[uBase+int(pair.spID)] - pair.cov) * invLOO
					if looMean > 0 {
						if pair.cov < looMean {
							sumMin += pair.cov
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
		for uIdx, candUnit := range units {
			if uIdx == uAssignedIdx && hasAssigned {
				continue
			}
			nCand := unitCountsArr[uIdx]
			if nCand <= 0 {
				continue
			}
			denom := p.totalCover + unitMeanTotals[uIdx]
			sim := 0.0
			if denom > 0 {
				invCand := unitInvCounts[uIdx]
				cBase := uIdx * nSp
				sumMin := 0.0
				for _, pair := range plotVec {
					candMean := unitSumsFlat[cBase+int(pair.spID)] * invCand
					if candMean > 0 {
						if pair.cov < candMean {
							sumMin += pair.cov
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
