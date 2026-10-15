package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

type summaryClassExpected struct {
	ID            int      `json:"id"`
	Display       int      `json:"display"`
	ShowClass     int      `json:"showClass"`
	MeanCoverBits string   `json:"meanCoverBits"`
	PresenceBits  string   `json:"presenceBits"`
	Sum           *float64 `json:"sum"`
	Count         int64    `json:"count"`
	NumPlots      int64    `json:"numPlots"`
	VariantType   int      `json:"variantType"`
	Value         *float64 `json:"value"`
	SingleBits    *string  `json:"singleBits"`
	Text          *string  `json:"text"`
}

func TestSummaryVegetationClassSealedSourceFixtures(t *testing.T) {
	path := os.Getenv("VPRO_SUMMARY_CLASS_FIXTURE")
	if path == "" {
		path = filepath.Join("testdata", "summaryVegetationClassSource.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SingleText []struct {
			Source     string `json:"source"`
			ID         int    `json:"id"`
			SingleBits string `json:"singleBits"`
			Text       string `json:"text"`
		} `json:"singleText"`
		DirectHelpers []summaryClassExpected `json:"directHelpers"`
		Wrappers      []summaryClassExpected `json:"wrappers"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.SingleText) != 281 || len(fixture.DirectHelpers) != 192 || len(fixture.Wrappers) != 72 {
		t.Fatal("source fixture coverage lost")
	}
	bits := func(t *testing.T, text string, size int) []byte {
		t.Helper()
		data, err := hex.DecodeString(text)
		if err != nil || len(data) != size {
			t.Fatal("source fixture bit encoding differs", text, err)
		}
		return data
	}
	assert := func(t *testing.T, got ProjectMetadataCell, expected summaryClassExpected) {
		t.Helper()
		switch expected.VariantType {
		case 1:
			if expected.Value != nil || expected.Text != nil || !reflect.DeepEqual(got, ProjectMetadataCell{Storage: "null"}) {
				t.Fatal("source NULL Variant differs", got, expected)
			}
		case 2:
			if expected.Value == nil || got.Storage != "integer" || got.Integer == nil ||
				*got.Integer != strconv.Itoa(int(*expected.Value)) || expected.Text == nil || *expected.Text != *got.Integer {
				t.Fatal("source INTEGER Variant or inferred TEXT differs", got, expected)
			}
		case 4:
			if expected.Value == nil || got.Storage != "real" || got.Real == nil ||
				math.Float64bits(*got.Real) != math.Float64bits(*expected.Value) {
				t.Fatal("source SINGLE Variant differs", got, expected)
			}
			if expected.SingleBits != nil &&
				math.Float32bits(float32(*got.Real)) != binary.LittleEndian.Uint32(bits(t, *expected.SingleBits, 4)) {
				t.Fatal("source assigned SINGLE bits differ", got, expected)
			}
			text, err := formatSummaryVegetationSingle(*got.Real)
			if err != nil || expected.Text == nil || text != *expected.Text {
				t.Fatal("source inferred TEXT differs", text, err, expected)
			}
		default:
			t.Fatal("unexpected source Variant", expected)
		}
	}
	for _, expected := range fixture.SingleText {
		t.Run(fmt.Sprintf("text/%s/%d", expected.Source, expected.ID), func(t *testing.T) {
			value := float64(math.Float32frombits(binary.LittleEndian.Uint32(bits(t, expected.SingleBits, 4))))
			got, err := formatSummaryVegetationSingle(value)
			if err != nil || got != expected.Text {
				t.Fatal(value, got, expected.Text, err)
			}
		})
	}
	for _, expected := range fixture.DirectHelpers {
		t.Run(fmt.Sprintf("helper/%d/%d/%d", expected.Display, expected.ShowClass, expected.ID), func(t *testing.T) {
			mean := math.Float64frombits(binary.LittleEndian.Uint64(bits(t, expected.MeanCoverBits, 8)))
			presence := math.Float64frombits(binary.LittleEndian.Uint64(bits(t, expected.PresenceBits, 8)))
			got, err := calculateSummaryVegetationClass(context.Background(), mean, presence, expected.Display, expected.ShowClass)
			if err != nil {
				t.Fatal(err)
			}
			assert(t, got, expected)
		})
	}
	for _, expected := range fixture.Wrappers {
		t.Run(fmt.Sprintf("formula/%d/%d/%d", expected.Display, expected.ShowClass, expected.ID), func(t *testing.T) {
			got, err := calculateSummaryVegetationClassFormula(context.Background(), expected.Sum, expected.Count,
				expected.NumPlots, expected.Display, expected.ShowClass)
			if err != nil {
				t.Fatal(err)
			}
			assert(t, got, expected)
		})
	}
}

func TestSummaryVegetationClassErrorsCancellationAndNegativeZero(t *testing.T) {
	for _, test := range []struct {
		name          string
		mean, present float64
		display, show int
	}{
		{"negative-prominence-sqrt", 1, -1, 4, 20},
		{"negative-goldstream-sqrt", -1, 1, 5, 20},
		{"nan-mean", math.NaN(), 1, 4, 20},
		{"infinite-presence", 1, math.Inf(1), 5, 20},
		{"double-overflow", math.MaxFloat64, 1, 4, 20},
		{"single-overflow", math.MaxFloat32, 1, 4, 20},
		{"unknown-display", 1, 1, 6, 20},
		{"unknown-show", 1, 1, 4, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := calculateSummaryVegetationClass(context.Background(), test.mean, test.present, test.display, test.show)
			if err == nil || !reflect.DeepEqual(got, ProjectMetadataCell{}) {
				t.Fatal("invalid input produced a partial result", got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, request := range []context.Context{nil, ctx} {
		got, err := calculateSummaryVegetationClass(request, 1, 1, 4, 20)
		if err == nil || !reflect.DeepEqual(got, ProjectMetadataCell{}) || request != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(got, err)
		}
		sum := 1.0
		got, err = calculateSummaryVegetationClassFormula(request, &sum, 1, 1, 4, 20)
		if err == nil || !reflect.DeepEqual(got, ProjectMetadataCell{}) || request != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(got, err)
		}
	}
	zero := math.Copysign(0, -1)
	for _, display := range []int{4, 5} {
		mean, presence := zero, 1.0
		if display == 5 {
			mean, presence = 1, zero
		}
		got, err := calculateSummaryVegetationClass(context.Background(), mean, presence, display, 20)
		if err != nil || got.Real == nil || !math.Signbit(*got.Real) {
			t.Fatal("SINGLE assignment lost signed zero", got, err)
		}
		text, err := formatSummaryVegetationSingle(*got.Real)
		if err != nil || text != "-0" {
			t.Fatal(text, err)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), 0.1, math.MaxFloat64} {
		text, err := formatSummaryVegetationSingle(value)
		if err == nil || text != "" {
			t.Fatal("unassigned/nonfinite SINGLE text accepted", value, text, err)
		}
	}
	for _, test := range []struct {
		sum   float64
		count int64
		plots int64
	}{
		{math.NaN(), 1, 1}, {math.Inf(-1), 1, 1},
		{1, -1, 1}, {1, math.MaxInt32 + 1, 1}, {1, 1, -1}, {1, 1, math.MaxInt32 + 1},
	} {
		got, err := calculateSummaryVegetationClassFormula(context.Background(), &test.sum, test.count, test.plots, 4, 20)
		if err == nil || !reflect.DeepEqual(got, ProjectMetadataCell{}) {
			t.Fatal("invalid formula produced a partial result", got, err)
		}
	}
}
