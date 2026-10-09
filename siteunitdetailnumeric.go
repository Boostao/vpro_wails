package main

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// summarizeSiteUnitNumeric consumes already joined, duplicate-weighted cells.
// Historical text/blob conversion is deliberately outside this numeric boundary.
func summarizeSiteUnitNumeric(cells []ProjectMetadataCell, field string, method int) (string, error) {
	ground := false
	switch field {
	case "Elevation", "StandAge", "SlopeGradient":
	case "SubstrateOrganicMatter", "SubstrateRocks", "SubstrateDecWood",
		"SubstrateMineralSoil", "SubstrateBedRock", "SubstrateWater",
		"StrataCoverTree", "StrataCoverShrub", "StrataCoverHerb", "StrataCoverMoss",
		"HumusThickness", "SeepageDepth", "RootingDepth":
		ground = true
	default:
		return "", fmt.Errorf("site unit numeric field %q is unavailable", field)
	}
	if method != 1 && method != 2 {
		return "", fmt.Errorf("site unit numeric method %d is unavailable", method)
	}

	values := make([]float64, 0, len(cells))
	nulls := 0
	sum := new(big.Rat)
	for i, cell := range cells {
		value, err := metadataCellValue(cell)
		if err != nil {
			return "", fmt.Errorf("site unit %s cell %d: %w", field, i, err)
		}
		var number float64
		switch value := value.(type) {
		case nil:
			nulls++
			continue
		case int64:
			number = float64(value)
		case float64:
			number = value
		default:
			return "", fmt.Errorf("site unit %s cell %d: historical %s storage is unsupported; numeric/NULL storage is required", field, i, cell.Storage)
		}
		values = append(values, number)
		sum.Add(sum, new(big.Rat).SetFloat64(number))
	}

	if len(values) == 0 {
		// Grouped Elevation/StandAge minima jump to the NULL suffix. Ungrouped
		// Slope/GroundCover minima reach an empty grouped average and exit early.
		if method == 1 && !ground && field != "SlopeGradient" && nulls > 0 {
			return fmt.Sprintf("  (Null %d)", nulls), nil
		}
		return "", nil
	}
	sort.Float64s(values)
	var parts [3]string
	if method == 2 {
		for i := range parts {
			parts[i] = siteUnitNumericString(siteUnitInclusiveQuartile(values, i+1))
		}
	} else {
		minimum, maximum := values[0], values[len(values)-1]
		if ground {
			minimum, maximum = math.Inf(1), math.Inf(-1)
			for _, value := range values {
				formatted := siteUnitGroundExtremum(value)
				minimum = math.Min(minimum, formatted)
				maximum = math.Max(maximum, formatted)
			}
			// The maximum query alone includes NULL: Format(NULL) is empty,
			// and Val of that empty formatted string contributes zero.
			if nulls > 0 {
				maximum = math.Max(maximum, 0)
			}
		}
		average, _ := new(big.Rat).Quo(sum, new(big.Rat).SetInt64(int64(len(values)))).Float64()
		parts[0], parts[2] = siteUnitNumericString(minimum), siteUnitNumericString(maximum)
		switch {
		case ground:
			parts[1] = siteUnitFixedNumber(average, 1, 1)
		case field == "SlopeGradient":
			parts[1] = siteUnitFixedNumber(average, 0, 1)
		default:
			parts[1] = siteUnitFixedNumber(average, 0, 2)
		}
	}
	result := strings.Join(parts[:], "---")
	if nulls > 0 {
		result += fmt.Sprintf("  (Null %d)", nulls)
	}
	return result, nil
}

func siteUnitInclusiveQuartile(sorted []float64, quartile int) float64 {
	index, remainder := (len(sorted)-1)*quartile/4, (len(sorted)-1)*quartile%4
	if remainder == 0 {
		return sorted[index]
	}
	// Rational interpolation avoids overflow when finite endpoints span the
	// full float64 range; Excel's inclusive rank is 1 + (n-1)*p.
	left := new(big.Rat).Mul(new(big.Rat).SetFloat64(sorted[index]), big.NewRat(int64(4-remainder), 4))
	right := new(big.Rat).Mul(new(big.Rat).SetFloat64(sorted[index+1]), big.NewRat(int64(remainder), 4))
	value, _ := left.Add(left, right).Float64()
	return value
}

func siteUnitNumericString(value float64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func siteUnitFixedNumber(value float64, decimals, width int) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return strconv.FormatFloat(value, 'f', decimals, 64)
	}
	// DAO first converts DOUBLE to fifteen significant decimal digits.
	// Round that decimal half-away, without binary multiplication error.
	decimal, _ := new(big.Rat).SetString(strconv.FormatFloat(value, 'g', 15, 64))
	negative := decimal.Sign() < 0
	decimal.Abs(decimal)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	decimal.Mul(decimal, new(big.Rat).SetInt(scale))
	remainder := new(big.Int)
	rounded := new(big.Int)
	rounded.QuoRem(decimal.Num(), decimal.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(decimal.Denom()) >= 0 {
		rounded.Add(rounded, big.NewInt(1))
	}
	text := rounded.String()
	if decimals > 0 {
		if len(text) <= decimals {
			text = strings.Repeat("0", decimals+1-len(text)) + text
		}
		text = text[:len(text)-decimals] + "." + text[len(text)-decimals:]
	}
	sign := ""
	if negative && rounded.Sign() != 0 {
		sign = "-"
	}
	integerWidth := len(strings.SplitN(text, ".", 2)[0])
	if integerWidth < width {
		text = strings.Repeat("0", width-integerWidth) + text
	}
	return sign + text
}

func siteUnitGroundExtremum(value float64) float64 {
	text := siteUnitFixedNumber(value, 2, 1)
	sign := 1.0
	if strings.HasPrefix(text, "-") {
		sign, text = -1, text[1:]
	}
	integer := strings.SplitN(text, ".", 2)[0]
	if len(integer) > 3 {
		// The source composes grouped Format with period-only Val; Val stops
		// at the first thousands comma rather than parsing the whole number.
		digits := len(integer) % 3
		if digits == 0 {
			digits = 3
		}
		text = integer[:digits]
	}
	number, _ := strconv.ParseFloat(text, 64)
	return sign * number
}
