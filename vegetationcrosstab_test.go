package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type vegetationPresenceFixtureRow struct {
	Layer     *string    `json:"layer"`
	Species   string     `json:"species"`
	Presence  *float64   `json:"presence"`
	MeanCover *float64   `json:"meanCover"`
	Plots     []*float64 `json:"plots"`
}

type vegetationPresenceFixture struct {
	Memberships  []string `json:"memberships"`
	Observations []struct {
		Plot, Layer, Species string
		Cover                *float64
	} `json:"observations"`
	Cases []struct {
		Grouped bool                           `json:"grouped"`
		Average string                         `json:"average"`
		Rows    []vegetationPresenceFixtureRow `json:"rows"`
	} `json:"cases"`
}

func readVegetationPresenceFixture(t *testing.T) vegetationPresenceFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "long-vegetation-presence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture vegetationPresenceFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func vegetationPresenceInputs(fixture vegetationPresenceFixture, grouped bool) ([]vegetationCrosstabInput, []vegetationCrosstabKey) {
	inputs, keys := []vegetationCrosstabInput{}, []vegetationCrosstabKey{}
	seen := map[string]bool{}
	for _, observation := range fixture.Observations {
		group := ProjectMetadataCell{Storage: "null"}
		if grouped {
			group = metadataText(observation.Layer)
		}
		key := vegetationCrosstabKey{group, metadataText(observation.Species)}
		tag := observation.Species
		if grouped {
			tag = observation.Layer + ":" + tag
		}
		if !seen[tag] {
			keys = append(keys, key)
			seen[tag] = true
		}
		cover := ProjectMetadataCell{Storage: "null"}
		if observation.Cover != nil {
			cover = ProjectMetadataCell{Storage: "real", Real: observation.Cover}
		}
		for _, plot := range fixture.Memberships {
			if plot == observation.Plot {
				inputs = append(inputs, vegetationCrosstabInput{key, plot, cover})
			}
		}
	}
	return inputs, keys
}

func normalizeVegetationCrosstab(rows []vegetationCrosstabRow) []vegetationPresenceFixtureRow {
	result := []vegetationPresenceFixtureRow{}
	for _, row := range rows {
		value := vegetationPresenceFixtureRow{Layer: row.Key.Group.Text, Species: *row.Key.Species.Text,
			Presence: row.Presence, MeanCover: row.MeanCover, Plots: []*float64{}}
		for _, plot := range row.Plots {
			value.Plots = append(value.Plots, plot.Cover)
		}
		result = append(result, value)
	}
	return result
}

func TestVegetationCrosstabMatchesAccessACEPresenceAndMeans(t *testing.T) {
	fixture := readVegetationPresenceFixture(t)
	for _, test := range fixture.Cases {
		inputs, keys := vegetationPresenceInputs(fixture, test.Grouped)
		options := vegetationCrosstabOptions{Average: test.Average, Ungrouped: !test.Grouped, ConstantSpeciesList: true,
			PresenceGreaterThan: 100, MeanCoverGreaterThan: 100}
		result, err := calculateVegetationCrosstab(context.Background(), inputs, len(fixture.Memberships), options, keys)
		if err != nil || !reflect.DeepEqual(normalizeVegetationCrosstab(result), test.Rows) {
			t.Fatal("native crosstab shape/values changed", test.Grouped, test.Average, normalizeVegetationCrosstab(result), test.Rows, err)
		}
		for _, row := range result {
			if len(row.Plots) != 2 || row.Plots[0].PlotNumber != "P1" || row.Plots[1].PlotNumber != "P2" {
				t.Fatal("orphan denominator plot became an invented pivot", row)
			}
		}
	}
}

func TestVegetationCrosstabStrictThresholdsConstantNullAndLiteralKeys(t *testing.T) {
	fixture := readVegetationPresenceFixture(t)
	inputs, keys := vegetationPresenceInputs(fixture, true)
	for _, test := range []struct {
		presence, mean float64
		want           int
	}{{0, 0, 2}, {25, 0, 1}, {50, 0, 0}, {0, 1, 1}, {0, 1.5, 0}, {0, -1, 3}} {
		result, err := calculateVegetationCrosstab(context.Background(), inputs, 4,
			vegetationCrosstabOptions{Average: "all-plots", PresenceGreaterThan: test.presence, MeanCoverGreaterThan: test.mean}, nil)
		if err != nil || len(result) != test.want {
			t.Fatal("strict threshold boundary changed", test, result, err)
		}
	}
	keys = append(keys, vegetationCrosstabKey{metadataText("1"), metadataText("absent")})
	result, err := calculateVegetationCrosstab(context.Background(), inputs, 4,
		vegetationCrosstabOptions{Average: "all-plots", ConstantSpeciesList: true, PresenceGreaterThan: 100, MeanCoverGreaterThan: 999}, keys)
	if err != nil || len(result) != 5 {
		t.Fatal("constant list did not bypass both thresholds", result, err)
	}
	found := false
	for _, row := range result {
		if *row.Key.Species.Text == "absent" {
			found = true
			if row.Presence != nil || row.MeanCover != nil || len(row.Plots) != 2 || row.Plots[0].Cover != nil || row.Plots[1].Cover != nil {
				t.Fatal("missing unit species imputed as zero", row)
			}
		}
	}
	if !found {
		t.Fatal("whole-scope missing species lost")
	}
	inputs = []vegetationCrosstabInput{}
	keys = []vegetationCrosstabKey{}
	for _, group := range []ProjectMetadataCell{{Storage: "null"}, metadataText("")} {
		for _, species := range []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataText("A"), metadataText("a")} {
			key := vegetationCrosstabKey{group, species}
			keys = append(keys, key)
			inputs = append(inputs, vegetationCrosstabInput{key, "", metadataInteger("0")})
		}
	}
	result, err = calculateVegetationCrosstab(context.Background(), inputs, 1,
		vegetationCrosstabOptions{Average: "all-plots", ConstantSpeciesList: true}, keys)
	if err != nil || len(result) != 8 {
		t.Fatal("NULL/empty/case-distinct group keys collapsed", result, err)
	}
	for _, row := range result {
		if len(row.Plots) != 1 || row.Plots[0].PlotNumber != "" {
			t.Fatal("zero/empty pivot identity lost", row)
		}
		if row.Key.Group.Storage == "null" || row.Key.Species.Storage == "null" {
			if row.Presence != nil || row.MeanCover != nil || row.Plots[0].Cover != nil {
				t.Fatal("SQL NULL constant-list equality join unexpectedly matched", row)
			}
		} else if *row.Presence != 1 || *row.MeanCover != 0 || *row.Plots[0].Cover != 0 {
			t.Fatal("empty text or zero treated as NULL join", row)
		}
	}
	result, err = calculateVegetationCrosstab(context.Background(), inputs, 1,
		vegetationCrosstabOptions{Average: "all-plots", PresenceGreaterThan: -1, MeanCoverGreaterThan: -1}, nil)
	if err != nil || len(result) != 8 {
		t.Fatal("ordinary crosstab NULL grouping collapsed", result, err)
	}
	for _, row := range result {
		if *row.Presence != 1 || *row.MeanCover != 0 {
			t.Fatal("ordinary NULL grouping incorrectly inherited constant-list NULL join", row)
		}
	}
}

func TestVegetationCrosstabDeterminismExactCancellationAndIndependentOutput(t *testing.T) {
	fixture := readVegetationPresenceFixture(t)
	inputs, keys := vegetationPresenceInputs(fixture, true)
	options := vegetationCrosstabOptions{Average: "observations", ConstantSpeciesList: true}
	expected, err := calculateVegetationCrosstab(context.Background(), inputs, 4, options, keys)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(882))
	for n := 0; n < 30; n++ {
		rng.Shuffle(len(inputs), func(i, j int) { inputs[i], inputs[j] = inputs[j], inputs[i] })
		rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		result, err := calculateVegetationCrosstab(context.Background(), inputs, 4, options, keys)
		if err != nil || !reflect.DeepEqual(result, expected) {
			t.Fatal("permutation changes numeric result", n, err)
		}
	}
	before, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	*expected[0].Key.Species.Text = "changed"
	*expected[0].Key.Group.Text = "changed"
	*expected[0].Presence = 999
	*expected[0].MeanCover = 999
	*expected[0].Plots[0].Cover = 999
	after, err := json.Marshal(inputs)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("output aliases inputs", err)
	}
	key := vegetationCrosstabKey{metadataText("1"), metadataText("A")}
	positive := 9007199254740992.0
	result, err := calculateVegetationCrosstab(context.Background(), []vegetationCrosstabInput{
		{key, "P1", metadataInteger("9007199254740993")},
		{key, "P1", ProjectMetadataCell{Storage: "real", Real: &positive}},
		{key, "P1", metadataInteger("-9007199254740992")},
		{key, "P1", metadataInteger("-9007199254740992")},
	}, 1, vegetationCrosstabOptions{Average: "all-plots"}, nil)
	if err != nil || len(result) != 1 || *result[0].MeanCover != 1 || *result[0].Plots[0].Cover != 1 || *result[0].Presence != 1 {
		t.Fatal("exact signed64/real cancellation lost", result, err)
	}
}

func TestVegetationCrosstabInvalidInputsOverflowAndCancellation(t *testing.T) {
	fixture := readVegetationPresenceFixture(t)
	inputs, keys := vegetationPresenceInputs(fixture, true)
	for _, options := range []vegetationCrosstabOptions{
		{Average: ""}, {Average: "unknown"},
		{Average: "all-plots", ConstantSpeciesList: true, Unfiltered: true},
		{Average: "all-plots", PresenceGreaterThan: math.NaN()},
		{Average: "all-plots", MeanCoverGreaterThan: math.Inf(1)},
	} {
		if result, err := calculateVegetationCrosstab(context.Background(), inputs, 4, options, nil); err == nil || result != nil {
			t.Fatal("invalid options returned success", result, err)
		}
	}
	for _, count := range []int{0, -1} {
		if result, err := calculateVegetationCrosstab(context.Background(), inputs, count,
			vegetationCrosstabOptions{Average: "all-plots"}, nil); err == nil || result != nil {
			t.Fatal("invalid denominator returned success", result, err)
		}
	}
	options := vegetationCrosstabOptions{Average: "all-plots", ConstantSpeciesList: true}
	if result, err := calculateVegetationCrosstab(context.Background(), inputs, 4,
		vegetationCrosstabOptions{Average: "all-plots", Ungrouped: true}, nil); err == nil || result != nil {
		t.Fatal("ungrouped calculation silently discarded group metadata", result, err)
	}
	repeated := append(append([]vegetationCrosstabKey{}, keys...), keys[0])
	if result, err := calculateVegetationCrosstab(context.Background(), inputs, 4, options, repeated); err == nil || result != nil {
		t.Fatal("duplicate constant keys accepted", result, err)
	}
	if result, err := calculateVegetationCrosstab(context.Background(), inputs, 4,
		vegetationCrosstabOptions{Average: "all-plots"}, keys); err == nil || result != nil {
		t.Fatal("inactive constant list accepted", result, err)
	}
	key := vegetationCrosstabKey{metadataText("1"), metadataText("A")}
	for _, bad := range []vegetationCrosstabInput{
		{key, "P1", metadataText("1")},
		{vegetationCrosstabKey{metadataInteger("1"), metadataText("A")}, "P1", metadataInteger("1")},
		{key, "\xff", metadataInteger("1")},
	} {
		if result, err := calculateVegetationCrosstab(context.Background(), []vegetationCrosstabInput{bad}, 4,
			vegetationCrosstabOptions{Average: "all-plots"}, nil); err == nil || result != nil {
			t.Fatal("invalid storage repaired", result, err)
		}
	}
	large := math.MaxFloat64
	overflow := []vegetationCrosstabInput{{key, "P1", ProjectMetadataCell{Storage: "real", Real: &large}},
		{key, "P1", ProjectMetadataCell{Storage: "real", Real: &large}}}
	if result, err := calculateVegetationCrosstab(context.Background(), overflow, 1,
		vegetationCrosstabOptions{Average: "observations"}, nil); err == nil || result != nil {
		t.Fatal("pivot overflow returned partial success", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{ctx, &vegetationCancelContext{context.Background(), 5}} {
		result, err := calculateVegetationCrosstab(ctx, inputs, 4, options, keys)
		if !errors.Is(err, context.Canceled) || result != nil {
			t.Fatal("cancellation returned partial success", result, err)
		}
	}
	result, err := calculateVegetationCrosstab(context.Background(), nil, 4, options, keys)
	if err != nil || len(result) != len(keys) {
		t.Fatal("empty unit constant-list LEFT JOIN lost", result, err)
	}
	for _, row := range result {
		if row.Presence != nil || row.MeanCover != nil || len(row.Plots) != 0 {
			t.Fatal("empty unit inferred observations", row)
		}
	}
}
