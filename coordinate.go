package main

import (
	"fmt"
	"math"
)

type CoordinateMode string

const (
	CoordinateModeDD  CoordinateMode = "dd"
	CoordinateModeDM  CoordinateMode = "dm"
	CoordinateModeDMS CoordinateMode = "dms"
)

type CoordinateParts struct {
	Degrees  *float64 `json:"degrees"`
	Minutes  *float64 `json:"minutes"`
	Seconds  *float64 `json:"seconds"`
	Negative bool     `json:"negative"`
}

func validateCoordinateMode(mode CoordinateMode) error {
	switch mode {
	case CoordinateModeDD, CoordinateModeDM, CoordinateModeDMS:
		return nil
	default:
		return fmt.Errorf("unsupported coordinate mode %q; expected dd, dm or dms", mode)
	}
}

func coordinateLimit(axis string) (float64, error) {
	switch axis {
	case "latitude":
		return 90, nil
	case "longitude":
		return 180, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate axis %q; expected latitude or longitude", axis)
	}
}

func finiteCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// ConvertCoordinate converts one axis only. Geographic limits are an intentional
// safety adaptation; source Single/Nz/sign loss and phantom edits are not copied.
// Negative is a raw sign, never an inferred east/west hemisphere convention.
func ConvertCoordinate(axis string, mode CoordinateMode, parts CoordinateParts) (*float64, error) {
	limit, err := coordinateLimit(axis)
	if err != nil {
		return nil, err
	}
	if err := validateCoordinateMode(mode); err != nil {
		return nil, err
	}
	for _, component := range []struct {
		name  string
		value *float64
	}{{"degrees", parts.Degrees}, {"minutes", parts.Minutes}, {"seconds", parts.Seconds}} {
		if component.value != nil && !finiteCoordinate(*component.value) {
			return nil, fmt.Errorf("coordinate %s must be finite", component.name)
		}
	}
	if mode == CoordinateModeDD {
		if parts.Minutes != nil || parts.Seconds != nil || parts.Negative {
			return nil, fmt.Errorf("dd requires signed degrees only; minutes/seconds must be absent and negative must be false")
		}
		if parts.Degrees == nil {
			return nil, nil
		}
		degrees := *parts.Degrees
		if math.Abs(degrees) > limit {
			return nil, fmt.Errorf("%s magnitude must not exceed %g degrees", axis, limit)
		}
		return &degrees, nil
	}
	if mode == CoordinateModeDM && parts.Seconds != nil {
		return nil, fmt.Errorf("dm seconds must be absent")
	}
	if parts.Degrees == nil && parts.Minutes == nil && parts.Seconds == nil {
		return nil, nil
	}
	if parts.Degrees == nil || parts.Minutes == nil || (mode == CoordinateModeDMS && parts.Seconds == nil) {
		return nil, fmt.Errorf("%s requires all degrees/minutes%s components; partial coordinates cannot be converted", mode,
			map[bool]string{true: "/seconds", false: ""}[mode == CoordinateModeDMS])
	}
	degrees, minutes := *parts.Degrees, *parts.Minutes
	if degrees < 0 || degrees != math.Trunc(degrees) {
		return nil, fmt.Errorf("%s degrees must be an unsigned integer magnitude; use negative for the whole coordinate", mode)
	}
	if degrees > limit {
		return nil, fmt.Errorf("%s degree magnitude must not exceed %g", axis, limit)
	}
	if minutes < 0 || minutes >= 60 {
		return nil, fmt.Errorf("coordinate minutes must satisfy 0 <= minutes < 60")
	}
	var seconds float64
	if mode == CoordinateModeDMS {
		if minutes != math.Trunc(minutes) {
			return nil, fmt.Errorf("dms minutes must be an integer")
		}
		seconds = *parts.Seconds
		if seconds < 0 || seconds >= 60 {
			return nil, fmt.Errorf("coordinate seconds must satisfy 0 <= seconds < 60")
		}
	}
	if degrees == limit && (minutes != 0 || seconds != 0) {
		return nil, fmt.Errorf("%s endpoint %g requires zero minutes and seconds", axis, limit)
	}
	magnitude := degrees + minutes/60 + seconds/3600
	if magnitude > limit {
		return nil, fmt.Errorf("%s magnitude must not exceed %g degrees", axis, limit)
	}
	if parts.Negative {
		magnitude = math.Copysign(magnitude, -1)
	}
	return &magnitude, nil
}

// DecomposeCoordinate preserves raw sign (including negative zero) and NULL.
// It has no axis argument: latitude-specific bounds remain Convert's responsibility.
// DM/DMS decomposition and recomposition can differ by float64 arithmetic ULPs,
// never by a deliberate decimal display rounding or a float32 conversion.
func DecomposeCoordinate(value *float64, mode CoordinateMode) (CoordinateParts, error) {
	if err := validateCoordinateMode(mode); err != nil {
		return CoordinateParts{}, err
	}
	if value == nil {
		return CoordinateParts{}, nil
	}
	if !finiteCoordinate(*value) || math.Abs(*value) > 180 {
		return CoordinateParts{}, fmt.Errorf("coordinate must be finite with magnitude no greater than 180")
	}
	if mode == CoordinateModeDD {
		degrees := *value
		return CoordinateParts{Degrees: &degrees}, nil
	}
	magnitude := math.Abs(*value)
	degrees := math.Floor(magnitude)
	minutes := (magnitude - degrees) * 60
	// Carry only an arithmetic boundary, without rounding valid components.
	if minutes >= 60 {
		degrees++
		minutes = 0
	}
	result := CoordinateParts{Degrees: &degrees, Minutes: &minutes, Negative: math.Signbit(*value)}
	if mode == CoordinateModeDMS {
		wholeMinutes := math.Floor(minutes)
		seconds := (minutes - wholeMinutes) * 60
		if seconds >= 60 {
			wholeMinutes++
			seconds = 0
		}
		if wholeMinutes >= 60 {
			degrees++
			wholeMinutes = 0
		}
		result.Minutes = &wholeMinutes
		result.Seconds = &seconds
	}
	return result, nil
}
