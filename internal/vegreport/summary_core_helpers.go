package vegreport

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

// Covers must already have source SINGLE staging and all source join weights.
// The shared binary unit certifies exact DOUBLE partial sums in every order.
func aggregateSummaryVegetationCovers(ctx context.Context, covers []vprocore.ProjectMetadataCell, numPlots int64) (summaryVegetationCellInput, error) {
	fail := func(err error) (summaryVegetationCellInput, error) { return summaryVegetationCellInput{}, err }
	if ctx == nil {
		return fail(errors.New("summary vegetation aggregation requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if numPlots < 1 || numPlots > math.MaxInt32 {
		return fail(errors.New("summary vegetation aggregation requires positive signed32 original plot count"))
	}
	count := int64(0)
	sum, absolute := new(big.Rat), new(big.Rat)
	minExponent, haveExponent := 0, false
	for i, cell := range covers {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		number, err := vprocore.VegetationReportNumber(cell)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation joined cover %d: %w", i+1, err))
		}
		if number == nil {
			continue
		}
		value, _ := number.Float64()
		single := float64(float32(value))
		if math.IsInf(single, 0) || new(big.Rat).SetFloat64(single).Cmp(number) != 0 {
			return fail(fmt.Errorf("summary vegetation joined cover %d lacks exact original SINGLE staging", i+1))
		}
		if count == math.MaxInt32 {
			return fail(errors.New("summary vegetation non-NULL cover count exceeds original signed32 range"))
		}
		count++
		sum.Add(sum, number)
		absolute.Add(absolute, new(big.Rat).Abs(number))
		if number.Sign() != 0 {
			exponent := int(number.Num().TrailingZeroBits()) - (number.Denom().BitLen() - 1)
			if !haveExponent || exponent < minExponent {
				minExponent, haveExponent = exponent, true
			}
		}
	}
	unit := new(big.Rat).SetInt64(1)
	if haveExponent {
		magnitude := minExponent
		if magnitude < 0 {
			magnitude = -magnitude
		}
		power := new(big.Int).Lsh(big.NewInt(1), uint(magnitude))
		if minExponent >= 0 {
			unit.SetInt(power)
		} else {
			unit.SetFrac(big.NewInt(1), power)
		}
	}
	bound := new(big.Rat).Quo(absolute, unit)
	limit := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 53))
	if bound.Cmp(limit) > 0 {
		return fail(errors.New("summary vegetation DOUBLE sum is not certified order-independent; source arithmetic remains unavailable"))
	}
	result := summaryVegetationCellInput{NumPlots: numPlots, Count: count}
	if count > 0 {
		total, exact := sum.Float64()
		if !exact || math.IsInf(total, 0) || math.IsNaN(total) {
			return fail(errors.New("summary vegetation certified sum lost exact finite DOUBLE output"))
		}
		mean := total / float64(count)
		result.Sum, result.Average = &total, &mean
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

type summaryVegetationCellInput struct {
	Count    int64
	NumPlots int64
	Sum      *float64
	Average  *float64
}

// The caller owns the joined aggregates. This preserves the source Variant
// before the crosstab infers/coerces its column type.
func calculateSummaryVegetationCover(ctx context.Context, input summaryVegetationCellInput, average int) (vprocore.ProjectMetadataCell, error) {
	fail := func(err error) (vprocore.ProjectMetadataCell, error) { return vprocore.ProjectMetadataCell{}, err }
	if err := validateSummaryVegetationCellInput(ctx, input, average); err != nil {
		return fail(err)
	}
	if input.Count == 0 {
		return vprocore.ProjectMetadataCell{Storage: "null"}, nil
	}
	if reportcore.SiteUnitFixedNumber(float64(input.Count)/float64(input.NumPlots), 4, 1) == "0.0000" {
		return fail(errors.New("summary vegetation formatted cover count is empty; original numeric criteria comparison is unavailable"))
	}
	return formatSummaryVegetationCover(ctx, input, average)
}

func validateSummaryVegetationCellInput(ctx context.Context, input summaryVegetationCellInput, average int) error {
	if ctx == nil {
		return errors.New("summary vegetation cover calculation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if average != 1 && average != 2 {
		return errors.New("summary vegetation cover requires original Average1 or2")
	}
	if input.NumPlots < 1 || input.NumPlots > math.MaxInt32 || input.Count < 0 || input.Count > math.MaxInt32 {
		return errors.New("summary vegetation cover requires positive original plot count and nonnegative signed32 cover count")
	}
	for _, value := range []*float64{input.Sum, input.Average} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return errors.New("summary vegetation aggregates require finite DOUBLE values")
		}
	}
	if input.Count == 0 {
		if input.Sum != nil || input.Average != nil {
			return errors.New("summary vegetation zero cover count requires NULL aggregates")
		}
	} else if input.Sum == nil || average == 2 && input.Average == nil {
		return errors.New("summary vegetation nonzero cover count lost its required aggregate")
	}
	return ctx.Err()
}

func formatSummaryVegetationCover(ctx context.Context, input summaryVegetationCellInput, average int) (vprocore.ProjectMetadataCell, error) {
	value := *input.Sum / float64(input.NumPlots)
	if average == 2 {
		value = *input.Average
	}
	var result vprocore.ProjectMetadataCell
	if value < 0.01 {
		floor := 0.01
		result = vprocore.ProjectMetadataCell{Storage: "real", Real: &floor}
	} else {
		text := reportcore.SiteUnitFixedNumber(value, 2, 1)
		result = vprocore.ProjectMetadataCell{Storage: "text", Text: &text}
	}
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataCell{}, err
	}
	return result, nil
}

type summaryVegetationUnit struct {
	Code          string
	NumPlots      int
	MembershipIDs []string
	RawVegRowIDs  []string
}

// Literal read-model identities are not an Access database-collation claim.
// Admission uses raw Veg; SV1 counts SU PlotNumber, not converted cover rows.
func prepareSummaryVegetationUnits(ctx context.Context, veg, su vprocore.ProjectMetadataTable) ([]summaryVegetationUnit, error) {
	if ctx == nil {
		return nil, errors.New("summary vegetation unit preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vc, err := vprocore.SiteUnitTransferColumns(veg, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("summary vegetation raw Veg schema: %w", err)
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("summary vegetation SU schema: %w", err)
	}
	raw := map[string][]string{}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[vc["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation raw Veg row %s PlotNumber: %w", row.RowID, err)
		}
		if plot != nil {
			raw[*plot] = append(raw[*plot], row.RowID)
		}
	}
	type unitState struct {
		unit summaryVegetationUnit
		raw  map[string]bool
	}
	units := map[string]*unitState{}
	groups := map[string]bool{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation SU row %s PlotNumber: %w", row.RowID, err)
		}
		code, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]])
		if err != nil {
			return nil, fmt.Errorf("summary vegetation SU row %s SiteUnit: %w", row.RowID, err)
		}
		groups[reportcore.VegetationTextKey(row.Cells[sc["SiteUnit"]])] = true
		if len(groups) > 250 {
			return nil, errors.New("summary vegetation has too many original SU groups (>250); limit units before output")
		}
		if code == nil {
			continue
		}
		state := units[*code]
		if state == nil {
			state = &unitState{unit: summaryVegetationUnit{Code: *code, MembershipIDs: []string{}, RawVegRowIDs: []string{}},
				raw: map[string]bool{}}
			units[*code] = state
		}
		state.unit.MembershipIDs = append(state.unit.MembershipIDs, row.RowID)
		if plot == nil {
			continue
		}
		state.unit.NumPlots++
		for _, id := range raw[*plot] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !state.raw[id] {
				state.raw[id] = true
				state.unit.RawVegRowIDs = append(state.unit.RawVegRowIDs, id)
			}
		}
	}
	result := []summaryVegetationUnit{}
	for _, state := range units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(state.unit.RawVegRowIDs) == 0 {
			continue
		}
		sort.Strings(state.unit.MembershipIDs)
		sort.Strings(state.unit.RawVegRowIDs)
		result = append(result, state.unit)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
