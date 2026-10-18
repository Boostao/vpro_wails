package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryConstancyReferencePlan(t *testing.T, master, personal ProjectMetadataTable) summaryVegetationReferencePlan {
	t.Helper()
	physical := ProjectMetadataTable{
		Columns: append(append([]ProjectMetadataColumn{}, master.Columns...), ProjectMetadataColumn{Name: "IsAttribute"}),
		Rows:    []ProjectMetadataRow{},
	}
	for _, row := range master.Rows {
		cells := append(append([]ProjectMetadataCell{}, row.Cells...), metadataInteger("0"))
		physical.Rows = append(physical.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}},
		Rows: []ProjectMetadataRow{}}
	plan, err := prepareSummaryVegetationReferences(t.Context(), physical, personal, descriptions, SummaryVegetationReferenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestSummaryConstancyOriginalProjectedQueries(t *testing.T) {
	for _, scenario := range []string{"ordinary", "null-lifeform", "same-display-identical-values", "same-display-distinct-values", "numeric-policies"} {
		for _, average := range []int{1, 2} {
			t.Run(scenario+"/average"+strconv.Itoa(average), func(t *testing.T) {
				raw, su, stage, master, personal, _ := summaryLayerFixture()
				stage.Rows[0].Cells[3], stage.Rows[1].Cells[3] = strataReal(14), strataReal(28)
				for i := range stage.Rows {
					stage.Rows[i].Cells[1] = metadataText("01")
				}
				stage.Rows[3].Cells[1] = metadataText("?")
				appendStage := func(plot, layer, species string, cover float64) {
					stage.Rows = append(stage.Rows, ProjectMetadataRow{RowID: strconv.Itoa(len(stage.Rows) + 1),
						Cells: []ProjectMetadataCell{metadataText(plot), metadataText(layer), metadataText(species), strataReal(cover)}})
				}
				appendMaster := func(code, name, form string) {
					master.Rows = append(master.Rows, ProjectMetadataRow{RowID: strconv.Itoa(len(master.Rows) + 1),
						Cells: lifeformReferenceRow(code, name, metadataInteger(form), metadataText(code), metadataText("U"))})
				}
				switch scenario {
				case "null-lifeform":
					master.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "null"}
				case "same-display-identical-values":
					appendMaster("Y", "Alpha", "1")
					appendStage("P1", "01", "Y", 14)
					appendStage("P2", "01", "Y", 28)
					appendStage("P3", "01", "Y", 5)
				case "same-display-distinct-values":
					appendMaster("Y", "Alpha", "1")
					appendStage("P2", "01", "Y", 3)
				case "numeric-policies":
					master.Rows[0].Cells[2] = metadataInteger("128")
					appendMaster("Z", "Unselected wide master", "2147483647")
					personal = lifeformReferenceFixture(lifeformReferenceRow("U", "Personal negative",
						metadataInteger("-1"), metadataText("u"), metadataText("U")))
					for i := 0; i < 3; i++ {
						stage.Rows[i].Cells[1] = metadataText("128")
					}
					appendStage("P1", "-01", "U", 6)
				}
				plan := summaryConstancyReferencePlan(t, master, personal)
				before, err := json.Marshal([]any{raw, su, stage, master, personal, plan})
				if err != nil {
					t.Fatal(err)
				}
				got, err := planSummaryVegetationGroupedStageQuery(t.Context(), raw, su, stage, master, personal,
					summaryGroupedLayers(), average, 13, 1, 0, &plan, summaryVegetationGroupedConstancy)
				if err != nil {
					t.Fatal(err)
				}
				if got.GroupField != (summaryVegetationGroupField{Name: "Lifeform", DAOType: 4, Size: 4}) ||
					got.OrderField != (summaryVegetationGroupField{Name: "Order", DAOType: 10}) {
					t.Fatal("native Constancy DAO field metadata differs", got.GroupField, got.OrderField)
				}
				null := ProjectMetadataCell{Storage: "null"}
				alpha := []ProjectMetadataCell{metadataInteger("1"), metadataText("Alpha"), metadataText("1.00"),
					metadataText("0.75 - 14.00"), metadataText("1.00 - 5.00")}
				missing := []ProjectMetadataCell{null, metadataText("S"), metadataText("0.25"), metadataText("0.25 - 0.50"), null}
				if average == 2 {
					alpha[3], missing[3] = metadataText("0.75 - 18.67"), metadataText("0.25 - 2.00")
				}
				expected := [][]ProjectMetadataCell{missing, alpha}
				rawCodes := []string{"S", "X"}
				switch scenario {
				case "null-lifeform":
					alpha[0] = null
					expected, rawCodes = [][]ProjectMetadataCell{alpha, missing}, []string{"X", "S"}
				case "same-display-identical-values":
					expected, rawCodes = [][]ProjectMetadataCell{missing, alpha, alpha}, []string{"S", "X", "Y"}
				case "same-display-distinct-values":
					value := "0.25 - 0.75"
					if average == 2 {
						value = "0.25 - 3.00"
					}
					expected = append(expected, []ProjectMetadataCell{metadataInteger("1"), metadataText("Alpha"),
						metadataText("0.25"), metadataText(value), null})
					rawCodes = append(rawCodes, "Y")
				case "numeric-policies":
					alpha[0] = metadataInteger("128")
					value := "0.50 - 3.00"
					if average == 2 {
						value = "0.50 - 6.00"
					}
					expected = [][]ProjectMetadataCell{missing, {metadataInteger("-1"), metadataText("Personal negative"),
						metadataText("0.50"), metadataText(value), null}, alpha}
					rawCodes = []string{"S", "U", "X"}
				}
				assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, expected)
				for i, row := range got.Report.Rows {
					if row.RawSpecies == nil || !reflect.DeepEqual(*row.RawSpecies, metadataText(rawCodes[i])) ||
						row.CommonName != nil || len(row.Witnesses) == 0 {
						t.Fatal("raw grouping identity, no-common policy or physical witnesses lost", row)
					}
				}
				first, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				for attempt := 0; attempt < 10; attempt++ {
					repeated, err := planSummaryVegetationGroupedStageQuery(t.Context(), raw, su, stage, master, personal,
						summaryGroupedLayers(), average, 13, 1, 0, &plan, summaryVegetationGroupedConstancy)
					encoded, marshalErr := json.Marshal(repeated)
					if err != nil || marshalErr != nil || string(encoded) != string(first) {
						t.Fatal("explicit raw-code tie adaptation is nondeterministic", err, marshalErr)
					}
				}
				*got.Report.Rows[0].RawSpecies.Text = "detached"
				if got.Report.Rows[len(got.Report.Rows)-1].Layer.Integer != nil {
					*got.Report.Rows[len(got.Report.Rows)-1].Layer.Integer = "999"
				}
				after, err := json.Marshal([]any{raw, su, stage, master, personal, plan})
				if err != nil || string(after) != string(before) {
					t.Fatal("Constancy output aliases or mutates physical inputs", err)
				}
			})
		}
	}
}

func TestSummaryConstancyRawPreparationAndSourcePolicies(t *testing.T) {
	for _, fanout := range []bool{false, true} {
		for _, average := range []int{1, 2} {
			veg, su, master, personal, layers := summaryGroupedRawTables(t)
			if fanout {
				personal = lifeformReferenceFixture(lifeformReferenceRow("X", "Alpha", metadataInteger("1"),
					metadataText("alternate"), metadataText("U")))
			}
			master.Rows = append(master.Rows, ProjectMetadataRow{RowID: "99",
				Cells: lifeformReferenceRow("Z", "Unselected wide master", metadataInteger("2147483647"),
					metadataText("z"), metadataText("U"))})
			plan := summaryConstancyReferencePlan(t, master, personal)
			got, err := prepareSummaryVegetationConstancyReferences(t.Context(), "Sample", "Selected",
				veg, su, master, personal, layers, average, &plan)
			if err != nil {
				t.Fatal(fanout, average, err)
			}
			prefix, a, b := "1.00", "0.75 - 14.00", "1.00 - 5.00"
			if average == 2 {
				a = "0.75 - 18.67"
			}
			if fanout {
				prefix, a, b = "2.00", "1.50 - 56.00", "2.00 - 20.00"
				if average == 2 {
					a, b = "1.50 - 37.33", "2.00 - 10.00"
				}
			}
			assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]ProjectMetadataCell{
				{metadataInteger("1"), metadataText("Alpha"), metadataText(prefix), metadataText(a), metadataText(b)},
			})
			if got.Report.Units[0].NumPlots != 4 || got.Report.Units[1].NumPlots != 1 {
				t.Fatal("Constancy changed fixed denominators")
			}
			duplicate := false
			for _, witness := range got.Report.Rows[0].Witnesses {
				if witness.ConversionLayerID != "" || len(witness.RawVegRowIDs) == 0 || len(witness.ReferenceOrigins) == 0 {
					t.Fatal("Constancy lost raw/reference origins or invented conversion", witness)
				}
				duplicate = duplicate || reflect.DeepEqual(witness.RawVegRowIDs, []string{"2", "4"})
			}
			if !duplicate {
				t.Fatal("raw MAX peer provenance was reduced")
			}
			if _, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
				veg, su, master, personal, layers, average, 13, 1, 0, &plan); err == nil {
				t.Fatal("new LONG reader silently widened the legacy grouped consumer")
			}
			master.Rows[0].Cells[2] = metadataInteger("2147483647")
			wide := summaryConstancyReferencePlan(t, master, personal)
			failed, err := prepareSummaryVegetationConstancyReferences(t.Context(), "Sample", "Selected",
				veg, su, master, personal, layers, average, &wide)
			if err == nil || !strings.Contains(err.Error(), "EntryDat_Veg.Layer") ||
				!reflect.DeepEqual(failed, summaryVegetationGroupedReport{}) {
				t.Fatal("selected out-of-width numeric group bypassed source TEXT3 staging", failed, err)
			}
		}
	}
}

func TestSummaryConstancyAttributeOriginsCombineStageLayers(t *testing.T) {
	for _, average := range []int{1, 2} {
		veg, su, master, personal, layers, plan := summaryStrataReferenceFixture(t, SummaryVegetationReferenceOptions{})
		before, err := json.Marshal([]any{veg, su, master, personal, layers, plan})
		if err != nil {
			t.Fatal(err)
		}
		got, err := prepareSummaryVegetationConstancyReferences(t.Context(), "Sample", "Selected",
			veg, su, master, personal, layers, average, &plan)
		if err != nil {
			t.Fatal(err)
		}
		derivedA, derivedB, alphaA, alphaB := "1.50 - 42.00", "2.00 - 15.00", "3.00 - 84.00", "4.00 - 30.00"
		if average == 2 {
			derivedA, derivedB, alphaA, alphaB = "1.50 - 28.00", "2.00 - 7.50", "3.00 - 28.00", "4.00 - 7.50"
		}
		assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]ProjectMetadataCell{
			{{Storage: "null"}, metadataText("X"), metadataText("2.00"), metadataText(derivedA), metadataText(derivedB)},
			{metadataInteger("1"), metadataText("Alpha"), metadataText("4.00"), metadataText(alphaA), metadataText(alphaB)},
		})
		for _, row := range got.Report.Rows {
			if row.RawSpecies == nil || !reflect.DeepEqual(*row.RawSpecies, metadataText("X")) {
				t.Fatal("derived display replaced raw code identity", row)
			}
			mapped, unmapped, derived := false, false, false
			for _, witness := range row.Witnesses {
				mapped, unmapped = mapped || witness.LayerMapID != "", unmapped || witness.LayerMapID == ""
				for _, origin := range witness.ReferenceOrigins {
					derived = derived || origin.Table == "USysSppAttributeDescription" && origin.RowID == "11"
				}
			}
			if !mapped || !unmapped || derived != (row.Layer.Storage == "null") {
				t.Fatal("numeric grouping did not combine original stage layers with exact origins", row)
			}
		}
		got.Report.Rows[0].Witnesses[0].RawVegRowIDs[0] = "detached"
		got.Report.Rows[0].Witnesses[0].ReferenceOrigins[0].RowID = "detached"
		after, err := json.Marshal([]any{veg, su, master, personal, layers, plan})
		if err != nil || string(before) != string(after) {
			t.Fatal("Constancy physical origins alias source", err)
		}
	}
}

func TestSummaryConstancyExplicitIntegerReaderCompatibility(t *testing.T) {
	for _, value := range []string{"-2147483648", "-32769", "32768", "2147483647"} {
		cell := metadataInteger(value)
		if _, err := vegetationLifeformInsertionLayer(cell); err == nil {
			t.Fatal("legacy insertion reader widened", value)
		}
		layer, err := vegetationLifeformInsertionLayerWithInteger(cell, summaryVegetationMasterLongInteger)
		if err != nil || layer.Text == nil || *layer.Text != value {
			t.Fatal("explicit master LONG formatting differs", value, layer, err)
		}
		union := lifeformReferenceFixture(lifeformReferenceRow("Z", "Wide", cell, metadataText("z"), metadataText("U")))
		if _, err := vegetationLifeformReferenceSchema(t.Context(), union); err == nil {
			t.Fatal("legacy reference schema widened", value)
		}
		if _, err := vegetationLifeformReferenceSourceSchemaWithInteger(t.Context(), union, "Lifeform",
			summaryVegetationMasterLongInteger); err != nil {
			t.Fatal("explicit master LONG schema rejected", value, err)
		}
	}
	_, prepared := summaryRawGroupFixture(t)
	prepared.ReducedRows = prepared.ReducedRows[:3]
	union := lifeformReferenceFixture(
		lifeformReferenceRow("X", "Alpha", metadataInteger("1"), metadataText("x"), metadataText("U")),
		lifeformReferenceRow("Z", "Unused", metadataInteger("2147483647"), metadataText("z"), metadataText("U")))
	if _, err := prepareSummaryVegetationGroupedEntries(t.Context(), prepared, union, 13); err == nil {
		t.Fatal("legacy staged-reference reader widened")
	}
	if _, err := prepareSummaryVegetationGroupedEntriesWithInteger(t.Context(), prepared, union, 13,
		summaryVegetationMasterLongInteger); err != nil {
		t.Fatal("new caller rejected unused valid master LONG", err)
	}
}

func TestSummaryConstancyGuardsCancellationAndRetry(t *testing.T) {
	veg, su, master, personal, layers := summaryGroupedRawTables(t)
	plan := summaryConstancyReferencePlan(t, master, personal)
	for _, mode := range [][3]int{{12, 1, 0}, {14, 1, 0}, {13, 3, 0}, {13, 6, 0}, {13, 1, 10}} {
		got, err := prepareSummaryVegetationGroupedQuery(t.Context(), "Sample", "Selected", veg, su, master, personal,
			layers, 1, mode[0], mode[1], mode[2], &plan, summaryVegetationGroupedConstancy)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("incompatible Constancy mode returned output", mode, got, err)
		}
	}
	for _, mode := range []string{"absent", "common", "guide"} {
		var references *summaryVegetationReferencePlan
		if mode != "absent" {
			proposed := plan
			proposed.Review.ShowCommon, proposed.Review.UseGuideNames = mode == "common", mode == "guide"
			references = &proposed
		}
		got, err := prepareSummaryVegetationConstancyReferences(t.Context(), "Sample", "Selected", veg, su,
			master, personal, layers, 1, references)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("absent or incompatible reference authority returned output", got, err)
		}
	}
	for _, value := range []ProjectMetadataCell{metadataText("1"), strataReal(1), metadataInteger("2147483648")} {
		bad := plan
		bad.References.Table.Rows = append([]ProjectMetadataRow{}, plan.References.Table.Rows...)
		bad.References.Table.Rows[0].Cells = append([]ProjectMetadataCell{}, plan.References.Table.Rows[0].Cells...)
		bad.References.Table.Rows[0].Cells[2] = value
		_, _, stage, _, _, _ := summaryLayerFixture()
		got, err := planSummaryVegetationGroupedStageQuery(t.Context(), veg, su, stage, master, personal, layers,
			1, 13, 1, 0, &bad, summaryVegetationGroupedConstancy)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("malformed numeric reference returned output", got, err)
		}
	}
	probe := &projectUnitCancellationContext{Context: t.Context()}
	if _, err := prepareSummaryVegetationConstancyReferences(probe, "Sample", "Selected",
		veg, su, master, personal, layers, 1, &plan); err != nil {
		t.Fatal(err)
	}
	for _, point := range []int{1, probe.calls / 2, probe.calls} {
		ctx := &projectUnitCancellationContext{Context: t.Context(), cancel: point}
		got, err := prepareSummaryVegetationConstancyReferences(ctx, "Sample", "Selected",
			veg, su, master, personal, layers, 1, &plan)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("cancelled Constancy returned partial output", point, got, err)
		}
	}
	if _, err := prepareSummaryVegetationConstancyReferences(t.Context(), "Sample", "Selected",
		veg, su, master, personal, layers, 1, &plan); err != nil {
		t.Fatal("cancelled preparation prevented retry", err)
	}
}
