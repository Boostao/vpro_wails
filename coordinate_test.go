package main

import (
	"math"
	"reflect"
	"testing"
)

func coordinateNumber(value float64) *float64 { return &value }

func TestCoordinate_ConversionSignNullAndFractions(t *testing.T) {
	for _, test := range []struct {
		name, axis string
		mode       CoordinateMode
		parts      CoordinateParts
		want       *float64
	}{
		{"dd signed latitude", "latitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(-49.1234567890123)}, coordinateNumber(-49.1234567890123)},
		{"dd raw positive longitude", "longitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(123.987654321098)}, coordinateNumber(123.987654321098)},
		{"dm decimal minutes", "latitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(49), Minutes: coordinateNumber(7.407407340738)}, coordinateNumber(49 + 7.407407340738/60)},
		{"dm whole negative magnitude", "longitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(123), Minutes: coordinateNumber(30.25), Negative: true}, coordinateNumber(-(123 + 30.25/60))},
		{"dms fractional seconds", "longitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(123), Minutes: coordinateNumber(59), Seconds: coordinateNumber(12.34567890123), Negative: true}, coordinateNumber(-(123 + 59.0/60 + 12.34567890123/3600))},
		{"dd NULL", "latitude", CoordinateModeDD, CoordinateParts{}, nil},
		{"dm signed NULL", "longitude", CoordinateModeDM, CoordinateParts{Negative: true}, nil},
		{"dms NULL", "latitude", CoordinateModeDMS, CoordinateParts{}, nil},
		{"latitude endpoint", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(90), Minutes: coordinateNumber(0), Seconds: coordinateNumber(0)}, coordinateNumber(90)},
		{"negative longitude endpoint", "longitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(180), Minutes: coordinateNumber(0), Negative: true}, coordinateNumber(-180)},
		{"dd negative zero", "longitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(math.Copysign(0, -1))}, coordinateNumber(math.Copysign(0, -1))},
		{"dms negative zero", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(0), Minutes: coordinateNumber(0), Seconds: coordinateNumber(0), Negative: true}, coordinateNumber(math.Copysign(0, -1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := cloneCoordinateParts(test.parts)
			got, err := ConvertCoordinate(test.axis, test.mode, test.parts)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (test.want == nil) {
				t.Fatalf("NULL conversion = %v, want %v", got, test.want)
			}
			if got != nil && math.Float64bits(*got) != math.Float64bits(*test.want) {
				t.Fatalf("conversion %.17g, want %.17g", *got, *test.want)
			}
			if !reflect.DeepEqual(before, test.parts) {
				t.Fatal("conversion mutated input components")
			}
		})
	}
}

func cloneCoordinateParts(parts CoordinateParts) CoordinateParts {
	copy := parts
	if parts.Degrees != nil {
		copy.Degrees = coordinateNumber(*parts.Degrees)
	}
	if parts.Minutes != nil {
		copy.Minutes = coordinateNumber(*parts.Minutes)
	}
	if parts.Seconds != nil {
		copy.Seconds = coordinateNumber(*parts.Seconds)
	}
	return copy
}

func TestCoordinate_RejectsInvalidModesAxesAndComponents(t *testing.T) {
	base := CoordinateParts{Degrees: coordinateNumber(49), Minutes: coordinateNumber(30), Seconds: coordinateNumber(15)}
	for _, test := range []struct {
		name, axis string
		mode       CoordinateMode
		parts      CoordinateParts
	}{
		{"unknown axis", "lat", CoordinateModeDD, CoordinateParts{}},
		{"empty axis", "", CoordinateModeDD, CoordinateParts{}},
		{"uppercase axis", "Latitude", CoordinateModeDD, CoordinateParts{}},
		{"unknown mode", "latitude", "unknown", CoordinateParts{}},
		{"empty mode", "latitude", "", CoordinateParts{}},
		{"uppercase mode", "latitude", "DD", CoordinateParts{}},
		{"dd minutes zero is supplied", "latitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(49), Minutes: coordinateNumber(0)}},
		{"dd seconds zero is supplied", "latitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(49), Seconds: coordinateNumber(0)}},
		{"dd sign flag", "latitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(49), Negative: true}},
		{"dd NULL sign flag", "latitude", CoordinateModeDD, CoordinateParts{Negative: true}},
		{"dd partial", "latitude", CoordinateModeDD, CoordinateParts{Minutes: coordinateNumber(30)}},
		{"dm seconds", "latitude", CoordinateModeDM, base},
		{"dm missing minutes", "latitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(49)}},
		{"dm missing degrees", "latitude", CoordinateModeDM, CoordinateParts{Minutes: coordinateNumber(30)}},
		{"dms missing seconds", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(49), Minutes: coordinateNumber(30)}},
		{"dms missing minutes", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(49), Seconds: coordinateNumber(15)}},
		{"dms missing degrees", "latitude", CoordinateModeDMS, CoordinateParts{Minutes: coordinateNumber(30), Seconds: coordinateNumber(15)}},
		{"fractional degrees", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(49.5), Minutes: base.Minutes, Seconds: base.Seconds}},
		{"negative degrees", "latitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(-49), Minutes: base.Minutes}},
		{"negative minutes", "latitude", CoordinateModeDM, CoordinateParts{Degrees: base.Degrees, Minutes: coordinateNumber(-1)}},
		{"minute 60", "latitude", CoordinateModeDM, CoordinateParts{Degrees: base.Degrees, Minutes: coordinateNumber(60)}},
		{"fractional dms minutes", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: base.Degrees, Minutes: coordinateNumber(30.5), Seconds: base.Seconds}},
		{"negative seconds", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: base.Degrees, Minutes: base.Minutes, Seconds: coordinateNumber(-1)}},
		{"seconds 60", "latitude", CoordinateModeDMS, CoordinateParts{Degrees: base.Degrees, Minutes: base.Minutes, Seconds: coordinateNumber(60)}},
		{"latitude dd range", "latitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(-90.000001)}},
		{"longitude dd range", "longitude", CoordinateModeDD, CoordinateParts{Degrees: coordinateNumber(180.000001)}},
		{"latitude dm range", "latitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(91), Minutes: coordinateNumber(0)}},
		{"longitude dms range", "longitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(181), Minutes: coordinateNumber(0), Seconds: coordinateNumber(0)}},
		{"latitude endpoint minutes", "latitude", CoordinateModeDM, CoordinateParts{Degrees: coordinateNumber(90), Minutes: coordinateNumber(0.000000000001)}},
		{"longitude endpoint seconds", "longitude", CoordinateModeDMS, CoordinateParts{Degrees: coordinateNumber(180), Minutes: coordinateNumber(0), Seconds: coordinateNumber(0.000000000001)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if result, err := ConvertCoordinate(test.axis, test.mode, test.parts); err == nil || result != nil {
				t.Fatalf("invalid conversion succeeded: %v %v", result, err)
			}
		})
	}
	for _, number := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, component := range []string{"degrees", "minutes", "seconds"} {
			parts := cloneCoordinateParts(base)
			switch component {
			case "degrees":
				parts.Degrees = coordinateNumber(number)
			case "minutes":
				parts.Minutes = coordinateNumber(number)
			case "seconds":
				parts.Seconds = coordinateNumber(number)
			}
			if value, err := ConvertCoordinate("latitude", CoordinateModeDMS, parts); err == nil || value != nil {
				t.Fatalf("nonfinite %s accepted: %v %v", component, value, err)
			}
		}
	}
}

func TestCoordinate_DecompositionRoundtripAndPrecision(t *testing.T) {
	values := []float64{
		0, math.Copysign(0, -1), math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		49.1234567890123, -123.987654321098, 0.000000000123456789,
		90, -90, 180, -180, math.Nextafter(1, 0), math.Nextafter(90, 0),
		math.Nextafter(180, 0), math.Nextafter(1.0/60, 0), math.Nextafter(1.0/3600, 0),
	}
	for _, mode := range []CoordinateMode{CoordinateModeDD, CoordinateModeDM, CoordinateModeDMS} {
		if parts, err := DecomposeCoordinate(nil, mode); err != nil || !reflect.DeepEqual(parts, CoordinateParts{}) {
			t.Fatalf("NULL decomposition %s: %+v %v", mode, parts, err)
		}
		for _, value := range values {
			parts, err := DecomposeCoordinate(&value, mode)
			if err != nil {
				t.Fatal(err)
			}
			if mode != CoordinateModeDD && parts.Negative != math.Signbit(value) {
				t.Fatal("decomposition lost raw sign")
			}
			converted, err := ConvertCoordinate("longitude", mode, parts)
			if err != nil || converted == nil {
				t.Fatalf("decomposed coordinate rejected: %.17g %s %+v %v", value, mode, parts, err)
			}
			if math.Signbit(*converted) != math.Signbit(value) {
				t.Fatal("roundtrip lost sign, including negative zero")
			}
			ulp := math.Nextafter(math.Abs(value), math.Inf(1)) - math.Abs(value)
			if math.Abs(*converted-value) > 2*ulp {
				t.Fatalf("roundtrip precision lost: %.17g -> %.17g (%s)", value, *converted, mode)
			}
		}
	}
	for _, mode := range []CoordinateMode{"", "unknown", "DD"} {
		if _, err := DecomposeCoordinate(nil, mode); err == nil {
			t.Fatal("NULL bypassed mode validation")
		}
	}
	for _, number := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 180.1, -180.1} {
		if _, err := DecomposeCoordinate(&number, CoordinateModeDMS); err == nil {
			t.Fatal("invalid decomposition value accepted")
		}
	}
	value := -123.987654321098
	for _, mode := range []CoordinateMode{CoordinateModeDD, CoordinateModeDM, CoordinateModeDMS} {
		parts, err := DecomposeCoordinate(&value, mode)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ConvertCoordinate("longitude", mode, parts)
		if err != nil {
			t.Fatal(err)
		}
		if *got == float64(float32(value)) || *got == math.Round(value*1e6)/1e6 {
			t.Fatalf("%s used source Single or six-decimal display rounding", mode)
		}
	}
}

func TestCoordinate_ConversionDoesNotTouchPartnerOrPreferences(t *testing.T) {
	// The service wrapper is pure even without a configured settings path.
	service := &CoordinateService{}
	partner := coordinateNumber(-123.987654321098)
	latitude := coordinateNumber(-49.1234567890123)
	partnerBits, latitudeBits := math.Float64bits(*partner), math.Float64bits(*latitude)
	got, err := service.ConvertCoordinate("latitude", CoordinateModeDD, CoordinateParts{Degrees: latitude})
	if err != nil {
		t.Fatal(err)
	}
	*got = 1
	if math.Float64bits(*latitude) != latitudeBits || math.Float64bits(*partner) != partnerBits {
		t.Fatal("conversion aliased input or changed the untouched partner")
	}
	if null, err := service.ConvertCoordinate("longitude", CoordinateModeDM, CoordinateParts{}); err != nil || null != nil {
		t.Fatal("untouched/absent coordinate was normalized to zero")
	}
}
