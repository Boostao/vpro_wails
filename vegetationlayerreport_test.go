package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func vegetationLayerFixture(t *testing.T) (VegetationReportPreparation, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	veg, su, layers := vegetationReportFixture(t)
	layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "Layer1234567"}, ProjectMetadataColumn{Name: "Layer"})
	for i := range layers.Rows {
		value := layers.Rows[i].Cells[0]
		label := ProjectMetadataCell{Storage: "null"}
		if value.Text != nil {
			label = metadataText("L" + *value.Text)
		}
		layers.Rows[i].Cells = append(layers.Rows[i].Cells, cloneSiteUnitCell(value), label)
	}
	prepared, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	species := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("Scientific A"), metadataText("English A"), metadataText("U")}},
		}}
	return prepared, species, layers
}

func layerTestOptions() longVegetationOptions {
	return longVegetationOptions{Title: "  Exact title  ", Average: "all-plots", ConstantSpeciesList: true,
		ShowEnglishName: true, Order: "presence"}
}

func TestLongVegetationLayerPhysicalDenominatorsAndMasterGrouping(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
	if err != nil || report.Title != "  Exact title  " || len(report.Units) != 4 {
		t.Fatal("unit scope or title differs", report, err)
	}
	counts := map[string]int{"null": 1, "text:": 1, "text:U": 2, "text:V": 1}
	for _, unit := range report.Units {
		if unit.NumPlots != counts[vegetationTextKey(unit.Code)] || len(unit.Rows) != 11 {
			t.Fatal("physical duplicate/NULL denominator or constant-list scope lost", unit)
		}
		for _, row := range unit.Rows {
			if row.Species.Text != nil && *row.Species.Text == "Scientific A" && row.Layer.Text != nil && *row.Layer.Text == "L1" {
				switch vegetationTextKey(unit.Code) {
				case "text:U", "text:V":
					if *row.Presence != 0.5 && vegetationTextKey(unit.Code) == "text:U" ||
						*row.MeanCover != -2 || *row.Plots[0].Cover != float64(unit.NumPlots)*-2 ||
						*row.EnglishName.Text != "English A" {
						t.Fatal("master grouping or duplicate weighting changed", row)
					}
				case "text:":
					if row.Presence != nil || row.MeanCover != nil {
						t.Fatal("absent constant species imputed", row)
					}
				}
			}
			if row.Species.Storage == "null" && (row.Presence != nil || row.MeanCover != nil) {
				t.Fatal("NULL species constant equality matched", row)
			}
		}
	}
	codes := map[string]bool{}
	for _, issue := range report.Diagnostics {
		codes[issue.Code] = true
	}
	for _, code := range []string{"physical_membership_multiplicity", "missing_master_species", "excluded_unit_without_vegetation"} {
		if !codes[code] {
			t.Fatal("missing scope diagnostic", code, report.Diagnostics)
		}
	}
}

func TestLongVegetationLayerEnglishGroupingConstantFanoutAndSynonymExclusion(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	species.Rows = append(species.Rows,
		ProjectMetadataRow{RowID: "2", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("Scientific A"), metadataText("Other English"), metadataText("X")}},
		ProjectMetadataRow{RowID: "3", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("Excluded"), metadataText("Synonym"), metadataText("s")}})
	report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		if vegetationTextKey(unit.Code) != "text:U" {
			continue
		}
		var rows []vegetationLayerRow
		for _, row := range unit.Rows {
			if row.Layer.Text != nil && *row.Layer.Text == "L1" && row.Species.Text != nil && *row.Species.Text == "Scientific A" {
				rows = append(rows, row)
			}
			if row.Species.Text != nil && *row.Species.Text == "Excluded" {
				t.Fatal("source synonym filter not applied")
			}
		}
		if len(rows) != 4 {
			t.Fatal("English grouping/base-key-only constant join fanout collapsed", rows)
		}
		combinations := map[string]bool{}
		for _, row := range rows {
			combinations[*row.EnglishName.Text+"|"+*row.MatchedName.Text] = true
			if *row.MeanCover != -2 || *row.Presence != 0.5 {
				t.Fatal("name bucket incorrectly combined stats", row)
			}
		}
		if len(combinations) != 4 {
			t.Fatal("fanout dimensions lost", combinations)
		}
		*rows[0].MeanCover = 999
		if *rows[1].MeanCover == 999 {
			t.Fatal("fanout rows alias numeric cells")
		}
	}
	options := layerTestOptions()
	options.ShowEnglishName = false
	options.ConstantSpeciesList = false
	options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -999
	report, err = planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		if vegetationTextKey(unit.Code) == "text:U" {
			for _, row := range unit.Rows {
				if row.Species.Text != nil && *row.Species.Text == "Scientific A" && row.Layer.Text != nil && *row.Layer.Text == "L1" {
					if *row.MeanCover != -4 || row.EnglishName.Storage != "null" || *row.Presence != 0.5 {
						t.Fatal("hidden English dimension still grouped, duplicate refs not multiplied", row)
					}
				}
			}
		}
	}
}

func TestLongVegetationLayerNullLabelsThresholdsAndUnassignedScope(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	for i := range layers.Rows {
		if layers.Rows[i].Cells[0].Text != nil && *layers.Rows[i].Cells[0].Text == "10" {
			layers.Rows[i].Cells[2] = ProjectMetadataCell{Storage: "null"}
		}
	}
	options := layerTestOptions()
	report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		for _, row := range unit.Rows {
			if row.Layer.Storage == "null" && (row.Presence != nil || row.MeanCover != nil) {
				t.Fatal("NULL layer constant-list key joined", row)
			}
		}
	}
	options.ConstantSpeciesList = false
	report, err = planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	foundNull := false
	for _, unit := range report.Units {
		for _, row := range unit.Rows {
			if row.Layer.Storage == "null" {
				foundNull = true
				if *row.MeanCover != 10 {
					t.Fatal("ordinary NULL-label group lost", row)
				}
			}
			if *row.MeanCover <= 0 || *row.Presence <= 0 {
				t.Fatal("strict default species thresholds bypassed", row)
			}
		}
	}
	if !foundNull {
		t.Fatal("NULL layer metadata incorrectly excluded ordinary group")
	}
	prepared.Memberships = []VegetationReportMembership{
		{RowID: "1", PlotNumber: metadataText("P2"), SiteUnit: ProjectMetadataCell{Storage: "null"}},
		{RowID: "2", PlotNumber: ProjectMetadataCell{Storage: "null"}, SiteUnit: ProjectMetadataCell{Storage: "null"}},
	}
	report, err = planLongVegetationLayers(context.Background(), prepared, species, layers, options)
	if err != nil || len(report.Units) != 1 || report.Units[0].NumPlots != 1 || len(report.Units[0].MembershipIDs) != 2 {
		t.Fatal("unassigned DCount confused with physical row count", report, err)
	}
}

func TestLongVegetationLayerMatchesACEEnglishJoinFixture(t *testing.T) {
	fixture := readVegetationPresenceFixture(t)
	veg, _, _ := vegetationReportFixture(t)
	veg.Rows = nil
	for i, observation := range fixture.Observations {
		row := ProjectMetadataRow{RowID: fmt.Sprint(i + 1), Cells: []ProjectMetadataCell{
			metadataText(observation.Plot), metadataText(observation.Species)}}
		for _, column := range veg.Columns[2:] {
			cell := ProjectMetadataCell{Storage: "null"}
			if column.Name == "Cover"+observation.Layer && observation.Cover != nil {
				cell = ProjectMetadataCell{Storage: "real", Real: observation.Cover}
			}
			row.Cells = append(row.Cells, cell)
		}
		veg.Rows = append(veg.Rows, row)
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for i, plot := range fixture.Memberships {
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: fmt.Sprint(i + 1), Cells: []ProjectMetadataCell{metadataText(plot), metadataText("U")}})
	}
	layers := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "LayerText"}, {Name: "Layer1234567"}, {Name: "Layer"}},
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("1"), metadataText("1"), metadataText("A1")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("2"), metadataText("2"), metadataText("A2")}},
		}}
	species := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("Scientific A"), metadataText("English A"), metadataText("U")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("Scientific A"), metadataText("Other English"), metadataText("X")}},
			{RowID: "3", Cells: []ProjectMetadataCell{metadataText("B"), metadataText("Excluded"), metadataText("Synonym"), metadataText("S")}},
		}}
	prepared, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
	if err != nil || len(report.Units) != 1 || report.Units[0].NumPlots != 4 {
		t.Fatal(report, err)
	}
	type oracleRow struct {
		Layer, Spp, EnglishName, MatchedName string
		P, MC, P1, P2                        *float64
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "long-vegetation-layer-join.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected []oracleRow
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	actual := []oracleRow{}
	for _, row := range report.Units[0].Rows {
		if len(row.Plots) != 2 || row.Plots[0].PlotNumber != "P1" || row.Plots[1].PlotNumber != "P2" {
			t.Fatal("pivot shape", row)
		}
		actual = append(actual, oracleRow{*row.Layer.Text, *row.Species.Text, *row.EnglishName.Text, *row.MatchedName.Text,
			row.Presence, row.MeanCover, row.Plots[0].Cover, row.Plots[1].Cover})
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("saved ACE English join differs", actual, expected)
	}
}
func TestLongVegetationLayerDeterminismNoAliasesCancellationAndMalformedReferences(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	expected, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal([]any{prepared, species, layers})
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(882))
	for n := 0; n < 20; n++ {
		rng.Shuffle(len(prepared.Memberships), func(i, j int) {
			prepared.Memberships[i], prepared.Memberships[j] = prepared.Memberships[j], prepared.Memberships[i]
		})
		rng.Shuffle(len(prepared.Observations), func(i, j int) {
			prepared.Observations[i], prepared.Observations[j] = prepared.Observations[j], prepared.Observations[i]
		})
		rng.Shuffle(len(species.Rows), func(i, j int) { species.Rows[i], species.Rows[j] = species.Rows[j], species.Rows[i] })
		rng.Shuffle(len(layers.Rows), func(i, j int) { layers.Rows[i], layers.Rows[j] = layers.Rows[j], layers.Rows[i] })
		actual, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
		if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatal("permutation changes report", n, err)
		}
	}
	prepared, species, layers = vegetationLayerFixture(t)
	*expected.Units[1].Code.Text = "changed"
	for _, unit := range expected.Units {
		for _, row := range unit.Rows {
			for _, cell := range []ProjectMetadataCell{row.Layer, row.Species, row.EnglishName, row.MatchedName} {
				if cell.Text != nil {
					*cell.Text = "changed"
				}
			}
		}
	}
	after, err := json.Marshal([]any{prepared, species, layers})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("report aliases input snapshots", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{ctx, &vegetationCancelContext{context.Background(), 10}} {
		report, err := planLongVegetationLayers(ctx, prepared, species, layers, layerTestOptions())
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, vegetationLayerReport{}) {
			t.Fatal("cancellation returned partial report", report, err)
		}
	}
	species.Rows[0].Cells[1] = metadataInteger("5")
	report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions())
	if err == nil || !reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("malformed reference was coerced", report, err)
	}
}
