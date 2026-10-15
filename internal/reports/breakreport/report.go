package breakreport

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/boostao/vpro-wails/internal/accesscompare"
)

const MaxRows = 10000
const MaxTextUnits = 255

type Pair struct {
	Reference *string `json:"reference"`
	BreakCode *string `json:"breakCode"`
}

type nullableKey struct {
	present bool
	text    string
}

type pairKey struct {
	reference nullableKey
	code      nullableKey
}

func textKey(text *string) (nullableKey, error) {
	if text == nil {
		return nullableKey{}, nil
	}
	key, ok := accesscompare.PrintableASCIIKey(*text)
	if !ok {
		return nullableKey{}, errors.New("requires calibrated printable ASCII; no text was repaired")
	}
	if len(*text) > MaxTextUnits {
		return nullableKey{}, errors.New("exceeds the source TEXT255 bound; no text was truncated")
	}
	return nullableKey{present: true, text: key}, nil
}

func same(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func clone(text *string) *string {
	if text == nil {
		return nil
	}
	value := *text
	return &value
}

func compare(left, right nullableKey) int {
	if left.present != right.present {
		if left.present {
			return 1
		}
		return -1
	}
	if left.text < right.text {
		return -1
	}
	if left.text > right.text {
		return 1
	}
	return 0
}

// Plan preserves DISTINCT pairs, refusing unqualified raw representatives.
// Output uses technical NULL-first code/reference order, not source report order.
func Plan(ctx context.Context, input []Pair) ([]Pair, error) {
	if ctx == nil {
		return nil, errors.New("PC-ORD Break Report requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(input) > MaxRows {
		return nil, errors.New("PC-ORD Break Report exceeds the 10000-row input bound; no rows were truncated")
	}
	groups := make(map[pairKey]Pair, len(input))
	for i, pair := range input {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		reference, err := textKey(pair.Reference)
		if err != nil {
			return nil, fmt.Errorf("PC-ORD Break Report row %d Reference: %w", i+1, err)
		}
		code, err := textKey(pair.BreakCode)
		if err != nil {
			return nil, fmt.Errorf("PC-ORD Break Report row %d BreakCode: %w", i+1, err)
		}
		key := pairKey{reference, code}
		if previous, exists := groups[key]; exists {
			if !same(previous.Reference, pair.Reference) || !same(previous.BreakCode, pair.BreakCode) {
				return nil, errors.New("PC-ORD Break Report has ambiguous raw DISTINCT representatives; no case or trailing-space value was chosen")
			}
			continue
		}
		groups[key] = Pair{Reference: clone(pair.Reference), BreakCode: clone(pair.BreakCode)}
	}
	keys := make([]pairKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if order := compare(keys[i].code, keys[j].code); order != 0 {
			return order < 0
		}
		return compare(keys[i].reference, keys[j].reference) < 0
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output := make([]Pair, 0, len(keys))
	for _, key := range keys {
		output = append(output, groups[key])
	}
	return output, nil
}
