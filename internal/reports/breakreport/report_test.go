package breakreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func pointer(value string) *string { return &value }

func canonical(t *testing.T, pairs []Pair) []string {
	t.Helper()
	result := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		raw, err := json.Marshal(pair)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, string(raw))
	}
	sort.Strings(result)
	return result
}

func TestOriginalQueryFixtures(t *testing.T) {
	raw, err := os.ReadFile(`testdata/source.json`)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name        string
			SourceRows  []Pair
			ActualPairs [][]*string
			Refuse      bool
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 13 {
		t.Fatal("all thirteen independent ACE query cases are required")
	}
	for _, item := range fixture.Cases {
		t.Run(item.Name, func(t *testing.T) {
			actual, err := Plan(context.Background(), item.SourceRows)
			if item.Refuse {
				if err == nil || actual != nil {
					t.Fatalf("unqualified representative/comparison succeeded: %+v, %v", actual, err)
				}
				return
			}
			if err != nil || actual == nil {
				t.Fatalf("source query case unavailable: %+v, %v", actual, err)
			}
			expected := make([]Pair, 0, len(item.ActualPairs))
			for _, pair := range item.ActualPairs {
				if len(pair) != 2 {
					t.Fatal("original two-field query projection required")
				}
				expected = append(expected, Pair{Reference: pair[0], BreakCode: pair[1]})
			}
			if !reflect.DeepEqual(canonical(t, actual), canonical(t, expected)) {
				t.Fatalf("literal DISTINCT pairs differ: got %+v, want %+v", actual, expected)
			}
		})
	}
}

func TestTechnicalDisplayOrderAndDetachment(t *testing.T) {
	empty, reference, code := "", "R", "A"
	input := []Pair{
		{Reference: &reference, BreakCode: pointer("B")},
		{Reference: &reference, BreakCode: &code},
		{Reference: pointer(" Q"), BreakCode: &code},
		{Reference: &empty, BreakCode: &empty},
		{Reference: nil, BreakCode: &empty},
		{Reference: &empty, BreakCode: nil},
		{},
		{Reference: &reference, BreakCode: &code},
	}
	actual, err := Plan(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	expected := []Pair{{}, {Reference: pointer("")}, {BreakCode: pointer("")},
		{Reference: pointer(""), BreakCode: pointer("")},
		{Reference: pointer(" Q"), BreakCode: pointer("A")},
		{Reference: pointer("R"), BreakCode: pointer("A")},
		{Reference: pointer("R"), BreakCode: pointer("B")}}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("technical NULL-first code/reference order differs: %+v", actual)
	}
	reference, code, empty = "changed", "changed", "changed"
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("input mutation changed detached report rows")
	}
	*actual[5].Reference = "output changed"
	if *actual[6].Reference != "R" {
		t.Fatal("output rows share mutable text ownership")
	}
}

func TestRawLiteralIsNotComparisonKey(t *testing.T) {
	input := []Pair{{Reference: pointer(" R "), BreakCode: pointer("A  ")}}
	actual, err := Plan(context.Background(), input)
	if err != nil || !reflect.DeepEqual(actual, input) {
		t.Fatalf("single literal source pair was folded/trimmed: %+v, %v", actual, err)
	}
	for _, input := range [][]Pair{
		{{Reference: pointer("R"), BreakCode: pointer("A")}, {Reference: pointer("r"), BreakCode: pointer("A")}},
		{{Reference: pointer("R"), BreakCode: pointer("A")}, {Reference: pointer("R "), BreakCode: pointer("A")}},
		{{Reference: pointer("R"), BreakCode: pointer("A")}, {Reference: pointer("R"), BreakCode: pointer("a")}},
		{{Reference: pointer("R"), BreakCode: pointer("A")}, {Reference: pointer("R"), BreakCode: pointer("A ")}},
		{{Reference: pointer(""), BreakCode: nil}, {Reference: pointer(" "), BreakCode: nil}},
	} {
		actual, err := Plan(context.Background(), input)
		if err == nil || actual != nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("raw representative guessed: %+v, %v", actual, err)
		}
	}
	distinct := []Pair{{Reference: pointer("R"), BreakCode: pointer("A")},
		{Reference: pointer("r"), BreakCode: pointer("B")}}
	if result, err := Plan(context.Background(), distinct); err != nil || len(result) != 2 {
		t.Fatalf("different pair keys were merged/refused: %+v, %v", result, err)
	}
}

func TestInputAndTextBounds(t *testing.T) {
	input := make([]Pair, MaxRows)
	for i := range input {
		input[i] = Pair{Reference: pointer("R"), BreakCode: pointer(fmt.Sprintf("%05d", i))}
	}
	if result, err := Plan(context.Background(), input); err != nil || len(result) != MaxRows {
		t.Fatalf("exact physical bound refused: rows=%d err=%v", len(result), err)
	}
	if result, err := Plan(context.Background(), append(input, Pair{})); err == nil || result != nil {
		t.Fatalf("physical overflow succeeded: %+v, %v", result, err)
	}
	for _, field := range []string{"Reference", "BreakCode"} {
		t.Run(field, func(t *testing.T) {
			for _, size := range []int{MaxTextUnits, MaxTextUnits + 1} {
				value := strings.Repeat("A", size)
				pair := Pair{Reference: &value}
				if field == "BreakCode" {
					pair = Pair{BreakCode: &value}
				}
				result, err := Plan(context.Background(), []Pair{pair})
				if size == MaxTextUnits {
					if err != nil || len(result) != 1 {
						t.Fatalf("exact TEXT255 bound refused: %+v, %v", result, err)
					}
				} else if err == nil || result != nil || !strings.Contains(err.Error(), field) {
					t.Fatalf("TEXT256 succeeded or lost field identity: %+v, %v", result, err)
				}
			}
		})
	}
}

func TestUnsupportedTextInBothFields(t *testing.T) {
	for _, value := range []string{"\x00", "\t", "\n", "\x7f", "\u00c9", "\u00df", "\xff", "A\xff"} {
		for _, field := range []string{"Reference", "BreakCode"} {
			pair := Pair{Reference: &value}
			if field == "BreakCode" {
				pair = Pair{BreakCode: &value}
			}
			result, err := Plan(context.Background(), []Pair{pair})
			if err == nil || result != nil || !strings.Contains(err.Error(), field) {
				t.Fatalf("%s %q: invalid source text succeeded: %+v, %v", field, value, result, err)
			}
		}
	}
}

type cancellingContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
}

func (c *cancellingContext) Err() error {
	c.calls++
	if c.calls == 50 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCancellation(t *testing.T) {
	if result, err := Plan(nil, nil); err == nil || result != nil {
		t.Fatalf("missing context succeeded: %+v, %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := Plan(ctx, nil); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatalf("pre-cancel succeeded: %+v, %v", result, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	during := &cancellingContext{Context: ctx, cancel: cancel}
	if result, err := Plan(during, make([]Pair, 100)); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatalf("in-flight cancel succeeded: %+v, %v", result, err)
	}
	if result, err := Plan(context.Background(), nil); err != nil || result == nil || len(result) != 0 {
		t.Fatalf("fresh retry/empty result failed: %+v, %v", result, err)
	}
}
