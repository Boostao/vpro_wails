package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryEnvironmentalCategoryOriginalFixtures(t *testing.T) {
	path := os.Getenv("VPRO_TEST_SUMMARY_ENVIRONMENT_CATEGORY_SOURCE")
	if path == "" {
		path = filepath.Join("testdata", "summary_environment_category_source.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Failure      json.RawMessage `json:"failure"`
		Observations []struct {
			Function string   `json:"function"`
			Unit     *string  `json:"unit"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if string(receipt.Failure) != "null" || len(receipt.Observations) != 40 {
		t.Fatal("accepted complete original source observations required")
	}
	seen := make(map[string]bool)
	for _, observation := range receipt.Observations {
		unit := "<NULL>"
		var categories []summaryVegetationEnvironmentalCategory
		var nullCount int64
		if observation.Unit != nil {
			unit = *observation.Unit
			switch unit {
			case "mixed", "MIXED":
				categories = []summaryVegetationEnvironmentalCategory{{"", 1}, {"A", 1}, {"B", 3}}
				nullCount = 1
			case "allnull":
				nullCount = 2
			case "":
				categories = []summaryVegetationEnvironmentalCategory{{"Z", 1}}
			case "plain":
				categories = []summaryVegetationEnvironmentalCategory{{"C", 1}}
			case "missing", "noenv":
			default:
				t.Fatalf("unknown original fixture unit %q", unit)
			}
		}
		key := observation.Function + "/" + unit
		if seen[key] {
			t.Fatalf("duplicate fixture %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			cell, err := formatSummaryVegetationEnvironmentalCategories(context.Background(),
				observation.Function, observation.Unit, categories, nullCount)
			if observation.Unit == nil {
				if err == nil || !strings.Contains(err.Error(), "error94") || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
					t.Fatalf("NULL argument result %#v %v", cell, err)
				}
				return
			}
			if len(observation.Actual) != 3 || observation.Actual[0] != "ok" || observation.Actual[1] != "String" {
				t.Fatal("original typed String result required")
			}
			want := observation.Actual[2]
			if err != nil || !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &want}) {
				t.Fatalf("got %#v %v; want %q", cell, err, want)
			}
		})
	}
}

func TestSummaryEnvironmentalCategoryLiteralPreparedOutput(t *testing.T) {
	unit := ""
	for _, tc := range []struct {
		name       string
		categories []summaryVegetationEnvironmentalCategory
		nulls      int64
		want       string
	}{
		{"empty", nil, 0, ""},
		{"nulls", nil, 2, "(Null 2)"},
		{"empty label", []summaryVegetationEnvironmentalCategory{{"", 1}}, 0, "(1)  "},
		{"ordered literal", []summaryVegetationEnvironmentalCategory{{"b", 2}, {" A ", 1}, {"b", 3}}, 0, "b(2)   A (1)  b(3)  "},
		{"unicode", []summaryVegetationEnvironmentalCategory{{"\u00e9", 1}}, 1, "\u00e9(1)  (Null 1)"},
		{"punctuation", []summaryVegetationEnvironmentalCategory{{"()", 1}}, 0, "()(1)  "},
		{"long count", []summaryVegetationEnvironmentalCategory{{"A", 2147483647}}, 2147483647, "A(2147483647)  (Null 2147483647)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cell, err := formatSummaryVegetationEnvironmentalCategories(context.Background(), "SVSnr", &unit, tc.categories, tc.nulls)
			if err != nil || !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "text", Text: &tc.want}) {
				t.Fatalf("got %#v %v; want %q", cell, err, tc.want)
			}
			wire, err := json.Marshal(cell)
			if err != nil {
				t.Fatal(err)
			}
			var object map[string]any
			if err := json.Unmarshal(wire, &object); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"storage": "text", "text": tc.want, "integer": nil, "real": nil, "blobHex": nil}
			if !reflect.DeepEqual(object, want) {
				t.Fatalf("full wire got %#v", object)
			}
			if len(tc.categories) > 0 {
				tc.categories[0].label = "changed"
				if *cell.Text != tc.want {
					t.Fatal("returned text aliases prepared input")
				}
			}
		})
	}
}

func TestSummaryEnvironmentalCategoryExplicitRefusalsAndRetry(t *testing.T) {
	validUnit := "unit"
	badUnit := string([]byte{0xff})
	for _, tc := range []struct {
		name       string
		function   string
		unit       *string
		categories []summaryVegetationEnvironmentalCategory
		nulls      int64
		errorText  string
	}{
		{"unsupported", "SVHumusDepth", &validUnit, nil, 0, "no calibrated"},
		{"NULL unit", "SVSnr", nil, nil, 0, "error94"},
		{"malformed unit", "SVSnr", &badUnit, nil, 0, "Site Unit"},
		{"malformed label", "SVSnr", &validUnit, []summaryVegetationEnvironmentalCategory{{string([]byte{0xff}), 1}}, 0, "label"},
		{"NUL label", "SVSnr", &validUnit, []summaryVegetationEnvironmentalCategory{{"a\x00", 1}}, 0, "label"},
		{"zero category", "SVSnr", &validUnit, []summaryVegetationEnvironmentalCategory{{"A", 0}}, 0, "positive"},
		{"negative category", "SVSnr", &validUnit, []summaryVegetationEnvironmentalCategory{{"A", -1}}, 0, "positive"},
		{"overflow category", "SVSnr", &validUnit, []summaryVegetationEnvironmentalCategory{{"A", 2147483648}}, 0, "positive"},
		{"negative NULL count", "SVSnr", &validUnit, nil, -1, "nonnegative"},
		{"overflow NULL count", "SVSnr", &validUnit, nil, 2147483648, "nonnegative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cell, err := formatSummaryVegetationEnvironmentalCategories(context.Background(), tc.function, tc.unit, tc.categories, tc.nulls)
			if err == nil || !strings.Contains(err.Error(), tc.errorText) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
				t.Fatalf("got %#v %v", cell, err)
			}
			cell, err = formatSummaryVegetationEnvironmentalCategories(context.Background(), "SVSnr", &validUnit, nil, 0)
			if err != nil || cell.Text == nil || *cell.Text != "" {
				t.Fatalf("corrected retry %#v %v", cell, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cell, err := formatSummaryVegetationEnvironmentalCategories(ctx, "SVSnr", &validUnit, nil, 0)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
		t.Fatalf("cancelled %#v %v", cell, err)
	}
}

type summaryEnvironmentalCategoryCancelContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
	at     int
}

func (ctx *summaryEnvironmentalCategoryCancelContext) Err() error {
	ctx.calls++
	if ctx.calls == ctx.at {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestSummaryEnvironmentalCategoryInFlightCancellation(t *testing.T) {
	unit := "unit"
	for _, at := range []int{3, 4} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &summaryEnvironmentalCategoryCancelContext{Context: base, cancel: cancel, at: at}
		cell, err := formatSummaryVegetationEnvironmentalCategories(ctx, "SVSnr", &unit,
			[]summaryVegetationEnvironmentalCategory{{"A", 1}, {"B", 1}}, 1)
		cancel()
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(cell, ProjectMetadataCell{}) {
			t.Fatalf("cancelled during call %d got %#v %v", at, cell, err)
		}
	}
}
