package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryVegetationEnvironmentalCountOriginalSourceCases(t *testing.T) {
	fixture := os.Getenv("VPRO_TEST_SUMMARY_ENVIRONMENT_COUNT_SOURCE")
	if fixture == "" {
		fixture = filepath.Join("testdata", "summary_environment_count_source.json")
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Failure      json.RawMessage `json:"failure"`
		Observations []struct {
			Unit                *string  `json:"unit"`
			Actual              []string `json:"actual"`
			ExpectedStatus      string   `json:"expectedStatus"`
			ExpectedTypeOrError string   `json:"expectedTypeOrError"`
			ExpectedCount       *int64   `json:"expectedCount"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if string(source.Failure) != "null" || len(source.Observations) != 8 {
		t.Fatal("complete passing eight-case original source fixture required")
	}
	for _, observation := range source.Observations {
		t.Run(strings.Join(observation.Actual, "|"), func(t *testing.T) {
			count := int64(0)
			if observation.ExpectedCount != nil {
				count = *observation.ExpectedCount
			} else if observation.ExpectedTypeOrError == "6" {
				count = 32768
			}
			cell, err := calculateSummaryVegetationEnvironmentalPlotCount(context.Background(), observation.Unit,
				observation.ExpectedTypeOrError != "3021", count)
			if observation.ExpectedStatus == "error" {
				if err == nil || !strings.Contains(err.Error(), observation.ExpectedTypeOrError) ||
					!reflect.DeepEqual(cell, ProjectMetadataCell{}) {
					t.Fatal("source failure lost", observation, cell, err)
				}
				return
			}
			if err != nil || observation.ExpectedTypeOrError != "Integer" ||
				cell.Storage != "integer" || cell.Integer == nil || *cell.Integer != observation.Actual[2] ||
				cell.Text != nil || cell.Real != nil || cell.BlobHex != nil {
				t.Fatal("original Variant Integer/complete cell shape differs", observation, cell, err)
			}
		})
	}
}

func TestSummaryVegetationEnvironmentalCountExactRangeAndImpossibleInput(t *testing.T) {
	unit := ""
	for _, count := range []int64{0, 1, 32766, 32767, 32768, 65536, math.MaxInt64, -1, math.MinInt64} {
		cell, err := calculateSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, true, count)
		if count < 0 || count > 32767 {
			if err == nil || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
				t.Fatal("invalid prepared count became successful output", count, cell, err)
			}
			continue
		}
		if err != nil || cell.Storage != "integer" || cell.Integer == nil {
			t.Fatal(count, cell, err)
		}
		if count == 0 && *cell.Integer != "0" || count == 1 && *cell.Integer != "1" ||
			count == 32766 && *cell.Integer != "32766" || count == 32767 && *cell.Integer != "32767" {
			t.Fatal("independent decimal count differs", count, cell)
		}
	}
}

func TestSummaryVegetationEnvironmentalCountCancellationRetryAndDetachedWire(t *testing.T) {
	unit := "literal"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cell, err := calculateSummaryVegetationEnvironmentalPlotCount(ctx, &unit, true, 3)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatal("cancelled calculation returned output", cell, err)
	}
	cell, err = calculateSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, true, 3)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cell)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shape, map[string]any{
		"storage": "integer", "text": nil, "integer": "3", "real": nil, "blobHex": nil,
	}) {
		t.Fatal("full tagged integer wire differs", string(raw))
	}
	*cell.Integer = "poison"
	next, err := calculateSummaryVegetationEnvironmentalPlotCount(context.Background(), &unit, true, 3)
	if err != nil || next.Integer == nil || *next.Integer != "3" || next.Integer == cell.Integer {
		t.Fatal("result pointers share mutable authority", next, err)
	}
	unit = "changed"
	if *next.Integer != "3" {
		t.Fatal("input-name mutation changed count output")
	}
}
