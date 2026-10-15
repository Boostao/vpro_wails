package main

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

func sourceDoubleDecimalString(value float64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'g', 15, 64)
}

type profileSQLNumber struct {
	value   float64
	decimal string
}

func profileEnvNumberCriterion(value float64) (profileSQLNumber, error) {
	literal := sourceDoubleDecimalString(value)
	number, err := strconv.ParseFloat(literal, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return profileSQLNumber{}, errors.New("source fifteen-digit numeric criterion is not finite")
	}
	return profileSQLNumberCriterion(literal)
}

func profileSQLNumberCriterion(literal string) (profileSQLNumber, error) {
	if len(literal) > 255 || !profileDecimal.MatchString(literal) {
		return profileSQLNumber{}, errors.New("source numeric criterion requires a finite decimal within TEXT255")
	}
	exponent := 0
	if position := strings.IndexAny(literal, "eE"); position >= 0 {
		digits := literal[position+1:]
		negative := digits[0] == '-'
		if negative || digits[0] == '+' {
			digits = digits[1:]
		}
		// ACE refuses the next digit after an accumulated exponent greater than 107374181.
		for _, digit := range digits {
			if exponent > 107374181 {
				return profileSQLNumber{}, errors.New("source numeric criterion exponent exceeds the ACE SQL token bound")
			}
			exponent = exponent*10 + int(digit-'0')
		}
		if negative {
			exponent = -exponent
		}
	}
	number, err := strconv.ParseFloat(literal, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return profileSQLNumber{}, errors.New("source numeric criterion is not finite")
	}
	result := profileSQLNumber{value: number}
	if number == 0 {
		if exponent > 28 {
			return profileSQLNumber{}, errors.New("source zero criterion with positive exponent above28 remains unavailable")
		}
		return result, nil
	}
	decimal, valid := new(big.Rat).SetString(literal)
	if !valid {
		return profileSQLNumber{}, errors.New("source numeric criterion decimal is invalid")
	}
	if sourceSQLDecimalLiteral(decimal) {
		result.decimal = literal
	}
	return result, nil
}

func sourceSQLDecimalLiteral(decimal *big.Rat) bool {
	if decimal.IsInt() && decimal.Num().IsInt64() {
		value := decimal.Num().Int64()
		if value >= -1<<31 && value <= 1<<31-1 {
			return false
		}
	}
	scaled := new(big.Rat).Set(decimal)
	for scale := 0; scale <= 28; scale++ {
		if scaled.IsInt() {
			return scaled.Num().BitLen() <= 96
		}
		scaled.Mul(scaled, big.NewRat(10, 1))
	}
	return false
}

func profileEnvDecimalCompare(raw any, literal string) (any, error) {
	return sourceProfileDecimalCompare(raw, literal, 15, 32)
}

func profileCoverDecimalCompare(raw any, literal string) (any, error) {
	return sourceProfileDecimalCompare(raw, literal, 15, 255)
}

func profileCoverSingleDecimalCompare(raw any, literal string) (any, error) {
	return sourceProfileDecimalCompare(raw, literal, 7, 255)
}

func sourceProfileDecimalCompare(raw any, literal string, precision, maximumLength int) (any, error) {
	if len(literal) > maximumLength || !profileDecimal.MatchString(literal) {
		return nil, errors.New("source DECIMAL comparison requires a finite decimal literal")
	}
	number, err := strconv.ParseFloat(literal, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, errors.New("source DECIMAL comparison requires a finite decimal literal")
	}
	if number == 0 {
		return nil, errors.New("source DECIMAL comparison requires a representable SQL DECIMAL criterion")
	}
	criterion, valid := new(big.Rat).SetString(literal)
	if !valid || !sourceSQLDecimalLiteral(criterion) {
		return nil, errors.New("source DECIMAL comparison requires a representable SQL DECIMAL criterion")
	}
	value, err := profileNumeric(raw)
	if err != nil || profileDriverNull(value) {
		return nil, err
	}
	number = value.(float64)
	if precision == 7 {
		number = float64(float32(number))
		if math.IsInf(number, 0) {
			return nil, errors.New("source SINGLE DECIMAL comparison overflow")
		}
	}
	magnitude := math.Abs(number)
	if magnitude >= math.Exp2(96) {
		if number < 0 {
			return int64(-1), nil
		}
		return int64(1), nil
	}
	decimal := new(big.Rat)
	if magnitude != 0 {
		// ACE uses the expression's runtime precision, not its DAO field descriptor.
		// Binary scaling/half-even rounding preserves adjacency lost by string formatting.
		scale := min(28, precision-1-int(math.Floor(math.Log10(magnitude))))
		scaled := magnitude
		if scale >= 0 {
			scaled *= math.Pow10(scale)
		} else {
			scaled /= math.Pow10(-scale)
		}
		rounded := int64(math.RoundToEven(scaled))
		if number < 0 {
			rounded = -rounded
		}
		decimal.SetInt64(rounded)
		exponent := scale
		if exponent < 0 {
			exponent = -exponent
		}
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil)
		if scale >= 0 {
			decimal.Quo(decimal, new(big.Rat).SetInt(power))
		} else {
			decimal.Mul(decimal, new(big.Rat).SetInt(power))
		}
	}
	return int64(decimal.Cmp(criterion)), nil
}
