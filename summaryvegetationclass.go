package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

func calculateSummaryVegetationClass(ctx context.Context, meanCover, presence float64,
	display, showClass int) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) { return ProjectMetadataCell{}, err }
	if err := validateSummaryVegetationClassMode(ctx, display, showClass); err != nil {
		return fail(err)
	}
	if math.IsNaN(meanCover) || math.IsInf(meanCover, 0) || math.IsNaN(presence) || math.IsInf(presence, 0) {
		return fail(errors.New("summary vegetation class operands require finite DOUBLE values"))
	}
	var value float64
	if display == 4 {
		if presence < 0 {
			return fail(errors.New("summary vegetation Prominence negative presence requires unverified source sqrt error handling"))
		}
		value = (meanCover * 10) * math.Sqrt(presence)
	} else {
		if meanCover < 0 {
			return fail(errors.New("summary vegetation Goldstream negative mean cover requires unverified source sqrt error handling"))
		}
		value = (presence * 100) * math.Sqrt(meanCover)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fail(errors.New("summary vegetation class arithmetic cannot produce a finite DOUBLE"))
	}
	single, err := lifeformSummarySingle(new(big.Rat).SetFloat64(value))
	if err != nil {
		return fail(fmt.Errorf("summary vegetation class TempVal SINGLE: %w", err))
	}
	if single == 0 {
		single = math.Copysign(single, value)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if showClass == 20 {
		return ProjectMetadataCell{Storage: "real", Real: &single}, nil
	}
	if single < 0 {
		return ProjectMetadataCell{Storage: "null"}, nil
	}
	limits := []float64{15, 50, 100, 200}
	class := 1
	if display == 5 {
		limits, class = []float64{5, 25, 75, 150, 300, 500}, 0
	}
	for _, limit := range limits {
		if single <= limit {
			break
		}
		class++
	}
	text := strconv.Itoa(class)
	return ProjectMetadataCell{Storage: "integer", Integer: &text}, nil
}

func validateSummaryVegetationClassMode(ctx context.Context, display, showClass int) error {
	if ctx == nil {
		return errors.New("summary vegetation class calculation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if display != 4 && display != 5 {
		return errors.New("summary vegetation class calculation requires original display4 or5")
	}
	if showClass != 1 && showClass != 10 && showClass != 20 {
		return errors.New("summary vegetation class calculation requires original ShowClass1,10 or20")
	}
	return nil
}

// RkFormula3/4 guard only NULL Sum and positive nPlots; they neither use AvgCover
// nor require positive CountCover. Their two Format(..., "Fixed") calls precede math.
func calculateSummaryVegetationClassFormula(ctx context.Context, sum *float64, count, numPlots int64,
	display, showClass int) (ProjectMetadataCell, error) {
	fail := func(err error) (ProjectMetadataCell, error) { return ProjectMetadataCell{}, err }
	if err := validateSummaryVegetationClassMode(ctx, display, showClass); err != nil {
		return fail(err)
	}
	if count < 0 || count > math.MaxInt32 || numPlots < 0 || numPlots > math.MaxInt32 {
		return fail(errors.New("summary vegetation class formula requires original nonnegative signed32 counts"))
	}
	if sum != nil && (math.IsNaN(*sum) || math.IsInf(*sum, 0)) {
		return fail(errors.New("summary vegetation class formula requires a finite DOUBLE sum"))
	}
	if sum == nil || numPlots == 0 {
		return ProjectMetadataCell{Storage: "null"}, nil
	}
	meanText := siteUnitFixedNumber(*sum/float64(numPlots), 2, 1)
	presenceText := siteUnitFixedNumber(float64(count)/float64(numPlots), 2, 1)
	mean, err := strconv.ParseFloat(meanText, 64)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation formatted class mean: %w", err))
	}
	presence, err := strconv.ParseFloat(presenceText, 64)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation formatted class presence: %w", err))
	}
	return calculateSummaryVegetationClass(ctx, mean, presence, display, showClass)
}

func formatSummaryVegetationSingle(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || float64(float32(value)) != value {
		return "", errors.New("summary vegetation text requires an exactly assigned finite SINGLE")
	}
	sign := ""
	if math.Signbit(value) {
		sign = "-"
	}
	if value == 0 {
		return sign + "0", nil
	}
	exact := new(big.Rat).SetFloat64(math.Abs(value))
	power := func(exponent int) *big.Rat {
		magnitude := exponent
		if magnitude < 0 {
			magnitude = -magnitude
		}
		integer := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(magnitude)), nil)
		result := new(big.Rat).SetInt(integer)
		if exponent < 0 {
			result.Inv(result)
		}
		return result
	}
	exponent := int(math.Floor(math.Log10(math.Abs(value))))
	for exact.Cmp(power(exponent)) < 0 {
		exponent--
	}
	for exact.Cmp(power(exponent+1)) >= 0 {
		exponent++
	}
	scaled := new(big.Rat).Mul(exact, power(6-exponent))
	rounded, remainder := new(big.Int), new(big.Int)
	rounded.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(scaled.Denom()) >= 0 {
		rounded.Add(rounded, big.NewInt(1))
	}
	if rounded.Cmp(big.NewInt(10000000)) >= 0 {
		rounded.Quo(rounded, big.NewInt(10))
		exponent++
	}
	coefficient := strings.TrimRight(rounded.String(), "0")
	fractionalDigits := len(coefficient) - 1 - exponent
	if exponent <= 6 && fractionalDigits <= 7 {
		if exponent < 0 {
			return sign + "0." + strings.Repeat("0", -exponent-1) + coefficient, nil
		}
		if fractionalDigits <= 0 {
			return sign + coefficient + strings.Repeat("0", -fractionalDigits), nil
		}
		return sign + coefficient[:exponent+1] + "." + coefficient[exponent+1:], nil
	}
	mantissa := coefficient[:1]
	if len(coefficient) > 1 {
		mantissa += "." + coefficient[1:]
	}
	return sign + mantissa + "E" + fmt.Sprintf("%+03d", exponent), nil
}
