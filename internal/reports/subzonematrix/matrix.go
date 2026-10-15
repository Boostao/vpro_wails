package subzonematrix

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	MaxInputRows  = 100000
	MaxInputBytes = 64 << 20
	MaxColumns    = 254
	MaxCells      = 1000000
)

type HierarchyRow struct {
	ID     int64
	Name   *string
	Parent *int64
	Level  *int64
}

type Row struct {
	Name   *string
	Values []*string
}

type Matrix struct {
	Columns []string
	Rows    []Row
}

type axis struct {
	key, caption string
}

type member struct {
	parent *int64
	value  string
}

// This is the bounded ACE1033 ASCII calibration, not a Unicode collation.
func key(text string) (string, error) {
	for _, char := range text {
		if char < 32 || char > 126 {
			return "", errors.New("matrix comparison requires calibrated printable ASCII; no text was repaired")
		}
	}
	return strings.ToLower(strings.TrimRight(text, " ")), nil
}

func first(text string, count int) string {
	if len(text) < count {
		return text
	}
	return text[:count]
}

func clone(text *string) *string {
	if text == nil {
		return nil
	}
	value := *text
	return &value
}

// Plan preserves source joins and refuses unordered First(Value) ambiguities.
// Axes/rows use technical ASCII-key order, not unqualified source locale order.
func Plan(ctx context.Context, hierarchy []HierarchyRow, labels []*string) (Matrix, error) {
	if ctx == nil {
		return Matrix{}, errors.New("matrix requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return Matrix{}, err
	}
	if len(hierarchy) > MaxInputRows || len(labels) > MaxInputRows {
		return Matrix{}, errors.New("matrix exceeds the 100000-row physical input bound; no truncation")
	}
	remaining := MaxInputBytes
	spend := func(value *string) error {
		if value != nil {
			if len(*value) > remaining {
				return errors.New("matrix exceeds the 64MiB source text bound; no truncation")
			}
			remaining -= len(*value)
		}
		return nil
	}
	parents := make(map[int64]*string, len(hierarchy))
	members := map[string][]member{}
	for _, row := range hierarchy {
		if err := ctx.Err(); err != nil {
			return Matrix{}, err
		}
		if _, duplicate := parents[row.ID]; duplicate {
			return Matrix{}, fmt.Errorf("matrix hierarchy ID %d is ambiguous", row.ID)
		}
		if err := spend(row.Name); err != nil {
			return Matrix{}, err
		}
		if row.Name != nil && !utf8.ValidString(*row.Name) {
			return Matrix{}, errors.New("matrix hierarchy contains malformed Unicode")
		}
		parents[row.ID] = row.Name
		if row.Level == nil || *row.Level != 11 || row.Name == nil {
			continue
		}
		unit := first(*row.Name, 10)
		unitKey, err := key(unit)
		if err != nil {
			return Matrix{}, err
		}
		members[unitKey] = append(members[unitKey], member{row.Parent, (*row.Name)[len(unit):]})
	}
	axes := map[string]axis{}
	captions := map[string]string{}
	type label struct {
		unit, axis string
		null       bool
	}
	prepared := make([]label, 0, len(labels))
	seen := map[label]bool{}
	for _, value := range labels {
		if err := ctx.Err(); err != nil {
			return Matrix{}, err
		}
		if err := spend(value); err != nil {
			return Matrix{}, err
		}
		item := label{axis: "\x00", null: value == nil}
		caption := "<>"
		if value != nil {
			var err error
			item.unit, err = key(*value)
			if err != nil {
				return Matrix{}, err
			}
			pivot := first(*value, 9)
			if pivot != "" {
				item.axis, err = key(pivot)
				if err != nil {
					return Matrix{}, err
				}
				if item.axis == "" {
					return Matrix{}, errors.New("matrix whitespace-only pivot representation is unverified")
				}
				caption = pivot
			}
		}
		if item.axis == "name" {
			return Matrix{}, errors.New("matrix pivot collides with the source Name column")
		}
		if existing, found := axes[item.axis]; found && existing.caption != caption {
			return Matrix{}, errors.New("matrix pivot has ambiguous source spelling; no representative was chosen")
		}
		if existing, found := captions[caption]; found && existing != item.axis {
			return Matrix{}, errors.New("matrix has distinct pivots with the same displayed column")
		}
		axes[item.axis], captions[caption] = axis{item.axis, caption}, item.axis
		if len(axes) > MaxColumns {
			return Matrix{}, errors.New("matrix exceeds the 254-column pivot bound; no truncation")
		}
		if !seen[item] {
			seen[item] = true
			prepared = append(prepared, item)
		}
	}
	ordered := make([]axis, 0, len(axes))
	for _, item := range axes {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	result := Matrix{Columns: []string{}, Rows: []Row{}}
	positions := map[string]int{}
	for i, item := range ordered {
		positions[item.key] = i
		result.Columns = append(result.Columns, item.caption)
	}
	type group struct {
		row      Row
		assigned []bool
	}
	groups := map[string]*group{}
	parentGroups := map[int64]*group{}
	var nullGroup *group
	contribute := func(parent *int64, value *string, column int) error {
		current := nullGroup
		if parent != nil {
			current = parentGroups[*parent]
		}
		if current == nil {
			var name *string
			if parent != nil {
				name = parents[*parent]
			}
			groupKey := "\x00"
			if name != nil {
				var err error
				groupKey, err = key(*name)
				if err != nil {
					return err
				}
			}
			current = groups[groupKey]
			if current == nil {
				if (len(groups)+1)*len(ordered) > MaxCells {
					return errors.New("matrix exceeds the 1000000-cell output bound; no partial matrix")
				}
				current = &group{Row{clone(name), make([]*string, len(ordered))}, make([]bool, len(ordered))}
				groups[groupKey] = current
			} else if name != nil && *current.row.Name != *name {
				return errors.New("matrix parent Name has ambiguous source spelling; no representative was chosen")
			}
			if parent == nil {
				nullGroup = current
			} else {
				parentGroups[*parent] = current
			}
		}
		previous := current.row.Values[column]
		if current.assigned[column] && (previous == nil) != (value == nil) ||
			current.assigned[column] && previous != nil && value != nil && *previous != *value {
			return errors.New("matrix has differing unordered First(Value) candidates; no value was guessed")
		}
		current.row.Values[column], current.assigned[column] = clone(value), true
		return nil
	}
	for _, item := range prepared {
		if err := ctx.Err(); err != nil {
			return Matrix{}, err
		}
		matches := members[item.unit]
		if item.null || len(matches) == 0 {
			if err := contribute(nil, nil, positions[item.axis]); err != nil {
				return Matrix{}, err
			}
			continue
		}
		for _, match := range matches {
			if err := ctx.Err(); err != nil {
				return Matrix{}, err
			}
			var value *string
			if match.parent != nil {
				value = &match.value
			}
			if err := contribute(match.parent, value, positions[item.axis]); err != nil {
				return Matrix{}, err
			}
		}
	}
	groupKeys := make([]string, 0, len(groups))
	for groupKey := range groups {
		groupKeys = append(groupKeys, groupKey)
	}
	sort.Strings(groupKeys)
	for _, groupKey := range groupKeys {
		result.Rows = append(result.Rows, groups[groupKey].row)
	}
	if err := ctx.Err(); err != nil {
		return Matrix{}, err
	}
	return result, nil
}
