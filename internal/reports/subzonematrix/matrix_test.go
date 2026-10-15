package subzonematrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func ptr[T any](value T) *T { return &value }

func TestControlledACE1033SourceMatrices(t *testing.T) {
	data, err := os.ReadFile("testdata/source.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Collation int
		Cases     []struct {
			Name      string
			Hierarchy []HierarchyRow
			Labels    []*string
			Columns   []string
			Rows      [][]*string
			Refusal   string
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Collation != 1033 || len(fixture.Cases) != 11 {
		t.Fatal("missing controlled native expectations")
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			before, err := json.Marshal(test)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Plan(context.Background(), test.Hierarchy, test.Labels)
			if test.Refusal != "" {
				if err == nil || !strings.Contains(err.Error(), test.Refusal) || !reflect.DeepEqual(result, Matrix{}) {
					t.Fatal("ambiguous source First was guessed", result, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				columns := append([]string{"Name"}, result.Columns...)
				rows := [][]*string{}
				for _, row := range result.Rows {
					rows = append(rows, append([]*string{row.Name}, row.Values...))
				}
				if !reflect.DeepEqual(columns, test.Columns) || !reflect.DeepEqual(rows, test.Rows) {
					t.Fatal("controlled native matrix differs", columns, rows, test.Columns, test.Rows)
				}
			}
			after, marshalErr := json.Marshal(test)
			if marshalErr != nil || string(before) != string(after) {
				t.Fatal("source input was mutated", marshalErr)
			}
		})
	}
}

func TestControlledACE1033ComparisonPairs(t *testing.T) {
	data, err := os.ReadFile("testdata/comparison.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Collation int
		Probes    []struct {
			ID       int
			Position int
			Text     string
		}
		EqualPairs [][2]int
		Scope      string
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Collation != 1033 || len(fixture.Probes) != 996 || len(fixture.EqualPairs) != 1658 {
		t.Fatal("missing independently read ACE equality pairs")
	}
	native := map[[2]int]bool{}
	for _, pair := range fixture.EqualPairs {
		native[pair] = true
	}
	keys := map[int]string{}
	for _, probe := range fixture.Probes {
		keys[probe.ID], err = key(probe.Text)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range fixture.Probes {
		for _, b := range fixture.Probes {
			if a.Position == b.Position && (keys[a.ID] == keys[b.ID]) != native[[2]int{a.ID, b.ID}] {
				t.Fatalf("native comparison differs: %q / %q", a.Text, b.Text)
			}
		}
	}
}

func ordinary() ([]HierarchyRow, []*string) {
	return []HierarchyRow{
		{1, ptr("Association"), nil, ptr[int64](10)},
		{2, ptr("AAAAAAAAA101"), ptr[int64](1), ptr[int64](11)},
	}, []*string{ptr("AAAAAAAAA1")}
}

func TestRefusalsReturnNoPartialMatrix(t *testing.T) {
	for _, test := range []struct {
		name      string
		hierarchy []HierarchyRow
		labels    []*string
		want      string
	}{
		{"duplicate-ID", []HierarchyRow{{ID: 1}, {ID: 1}}, nil, "ID"},
		{"malformed-Unicode", []HierarchyRow{{ID: 1, Name: ptr("\xff")}}, nil, "malformed Unicode"},
		{"unqualified-prefix", []HierarchyRow{{1, ptr("éBCDEFGHIJ01"), nil, ptr[int64](11)}}, nil, "ASCII"},
		{"unqualified-label", nil, []*string{ptr("éBCDEFGHIJ")}, "ASCII"},
		{"NUL-label", nil, []*string{ptr("A\x00")}, "ASCII"},
		{"control-label", nil, []*string{ptr("A\t")}, "ASCII"},
		{"pivot-case-alias", nil, []*string{ptr("AAAAAAAAA1"), ptr("aaaaaaaaa1")}, "spelling"},
		{"pivot-space-alias", nil, []*string{ptr("A"), ptr("A ")}, "spelling"},
		{"pivot-whitespace", nil, []*string{ptr("   ")}, "whitespace"},
		{"source-Name-column", nil, []*string{ptr("Name")}, "Name column"},
		{"null-caption-collision", nil, []*string{nil, ptr("<>")}, "same displayed"},
		{"parent-case-alias", []HierarchyRow{
			{1, ptr("Association"), nil, ptr[int64](10)}, {2, ptr("association"), nil, ptr[int64](10)},
			{3, ptr("AAAAAAAAA101"), ptr[int64](1), ptr[int64](11)},
			{4, ptr("AAAAAAAAA101"), ptr[int64](2), ptr[int64](11)},
		}, []*string{ptr("AAAAAAAAA1")}, "spelling"},
		{"unqualified-parent", []HierarchyRow{
			{1, ptr("Associación"), nil, ptr[int64](10)},
			{2, ptr("AAAAAAAAA101"), ptr[int64](1), ptr[int64](11)},
		}, []*string{ptr("AAAAAAAAA1")}, "ASCII"},
		{"NULL-versus-text-First", []HierarchyRow{
			{1, ptr("AAAAAAAAA101"), nil, ptr[int64](11)},
			{2, ptr("AAAAAAAAA202"), ptr[int64](999), ptr[int64](11)},
		}, []*string{ptr("AAAAAAAAA1"), ptr("AAAAAAAAA2")}, "First(Value)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Plan(context.Background(), test.hierarchy, test.labels)
			if err == nil || !strings.Contains(err.Error(), test.want) || !reflect.DeepEqual(result, Matrix{}) {
				t.Fatal("invalid/ambiguous input returned success/partial data", result, err)
			}
		})
	}
}

func TestOpaqueSuffixAndOutputOwnership(t *testing.T) {
	hierarchy, labels := ordinary()
	hierarchy[1].Name = ptr("AAAAAAAAA1é🌲")
	result, err := Plan(context.Background(), hierarchy, labels)
	if err != nil || *result.Rows[0].Values[0] != "é🌲" {
		t.Fatal("opaque suffix was folded/repaired", result, err)
	}
	*result.Rows[0].Name = "changed"
	*result.Rows[0].Values[0] = "changed"
	result.Columns[0] = "changed"
	if *hierarchy[0].Name != "Association" || *hierarchy[1].Name != "AAAAAAAAA1é🌲" || *labels[0] != "AAAAAAAAA1" {
		t.Fatal("output aliases source pointers/slices")
	}
	hierarchy[1].Name = ptr("AAAAAAAAA1")
	result, err = Plan(context.Background(), hierarchy, labels)
	if err != nil || result.Rows[0].Values[0] == nil || *result.Rows[0].Values[0] != "" {
		t.Fatal("empty Mid suffix became NULL", result, err)
	}
}

func TestExactPhysicalAndPivotBounds(t *testing.T) {
	hierarchy := make([]HierarchyRow, MaxInputRows)
	for i := range hierarchy {
		hierarchy[i].ID = int64(i)
	}
	labels := make([]*string, MaxInputRows)
	result, err := Plan(context.Background(), hierarchy, labels)
	if err != nil || len(result.Columns) != 1 || len(result.Rows) != 1 {
		t.Fatal("exact physical bound unavailable", result, err)
	}
	for _, test := range []struct {
		hierarchy []HierarchyRow
		labels    []*string
	}{
		{append(hierarchy, HierarchyRow{ID: MaxInputRows}), nil},
		{nil, append(labels, nil)},
	} {
		if result, err := Plan(context.Background(), test.hierarchy, test.labels); err == nil || !reflect.DeepEqual(result, Matrix{}) {
			t.Fatal("max+1 physical input truncated", result, err)
		}
	}
	labels = nil
	for i := range MaxColumns {
		labels = append(labels, ptr(fmt.Sprintf("U%08d/", i)))
	}
	result, err = Plan(context.Background(), nil, labels)
	if err != nil || len(result.Columns) != MaxColumns || len(result.Rows) != 1 {
		t.Fatal("exact pivot bound unavailable", result, err)
	}
	if result, err := Plan(context.Background(), nil, append(labels, ptr("U99999999/"))); err == nil || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("max+1 pivot truncated", result, err)
	}
}

func TestExactCellBound(t *testing.T) {
	labels := []*string{}
	for i := range 250 {
		labels = append(labels, ptr(fmt.Sprintf("U%08d/", i)))
	}
	hierarchy := []HierarchyRow{}
	for i := range 3999 {
		id := int64(2*i + 1)
		hierarchy = append(hierarchy,
			HierarchyRow{id, ptr(fmt.Sprintf("Group%05d", i)), nil, ptr[int64](10)},
			HierarchyRow{id + 1, ptr(fmt.Sprintf("U00000000/%04d", i)), ptr(id), ptr[int64](11)})
	}
	result, err := Plan(context.Background(), hierarchy, labels)
	if err != nil || len(result.Rows) != 4000 || len(result.Columns) != 250 ||
		len(result.Rows)*len(result.Columns) != MaxCells {
		t.Fatal("exact cell bound unavailable", len(result.Rows), len(result.Columns), err)
	}
	hierarchy = append(hierarchy, HierarchyRow{9000, ptr("Extra"), nil, ptr[int64](10)},
		HierarchyRow{9001, ptr("U00000000/extra"), ptr[int64](9000), ptr[int64](11)})
	if result, err := Plan(context.Background(), hierarchy, labels); err == nil || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("overbound cells returned a partial matrix", err)
	}
}

func TestExactSourceTextByteBound(t *testing.T) {
	text := strings.Repeat("x", MaxInputBytes)
	hierarchy := []HierarchyRow{{ID: 1, Name: &text}}
	result, err := Plan(context.Background(), hierarchy, nil)
	if err != nil || len(result.Rows) != 0 || result.Columns == nil {
		t.Fatal("exact source text byte bound unavailable", err)
	}
	if result, err := Plan(context.Background(), hierarchy, []*string{ptr("x")}); err == nil ||
		!strings.Contains(err.Error(), "64MiB") || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("aggregate text max+1 returned partial data", err)
	}
	hierarchy[0].Name = nil
	if result, err := Plan(context.Background(), hierarchy, []*string{ptr(text + "x")}); err == nil ||
		!strings.Contains(err.Error(), "64MiB") || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("single-field text max+1 accepted", err)
	}
}

func TestLongParentFanoutWorkBound(t *testing.T) {
	name := strings.Repeat("A", 32<<20)
	parent := int64(0)
	level := int64(11)
	child := "AAAAAAAAA101"
	hierarchy := make([]HierarchyRow, MaxInputRows)
	hierarchy[0] = HierarchyRow{ID: parent, Name: &name}
	for i := 1; i < len(hierarchy); i++ {
		hierarchy[i] = HierarchyRow{ID: int64(i), Name: &child, Parent: &parent, Level: &level}
	}
	started := time.Now()
	result, err := Plan(context.Background(), hierarchy, []*string{ptr("AAAAAAAAA1")})
	if err != nil || len(result.Rows) != 1 || len(result.Columns) != 1 ||
		*result.Rows[0].Name != name || *result.Rows[0].Values[0] != "01" {
		t.Fatal("equivalent repeated children changed the cell", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("allowed long-parent fanout exceeded five-second work budget: %s", elapsed)
	}
}

type cancellingContext struct {
	context.Context
	cancel context.CancelFunc
	calls  int
}

func (ctx *cancellingContext) Err() error {
	ctx.calls++
	if ctx.calls == 5 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestCancellationAndMissingContext(t *testing.T) {
	hierarchy, labels := ordinary()
	if result, err := Plan(nil, hierarchy, labels); err == nil || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("missing context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := Plan(ctx, hierarchy, labels); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, Matrix{}) {
		t.Fatal("cancelled request accepted", result, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	held := &cancellingContext{Context: ctx, cancel: cancel}
	if result, err := Plan(held, hierarchy, labels); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, Matrix{}) ||
		held.calls != 5 {
		t.Fatal("in-flight cancellation returned partial data", result, err, held.calls)
	}
	if _, err := Plan(context.Background(), hierarchy, labels); err != nil {
		t.Fatal("fresh retry failed", err)
	}
}
