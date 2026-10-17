package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryGroupedAttributeReferenceIndependentValues(t *testing.T) {
	for _, grouping := range []int{13, 14} {
		for _, average := range []int{1, 2} {
			veg, su, master, personal, layers, plan := summaryStrataReferenceFixture(t, SummaryVegetationReferenceOptions{})
			before, err := json.Marshal([]any{veg, su, master, personal, layers, plan})
			if err != nil {
				t.Fatal(err)
			}
			got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
				veg, su, master, personal, layers, average, grouping, 6, 0, &plan)
			if err != nil {
				t.Fatal(grouping, average, err)
			}
			group, a, b, derivedA, derivedB, order, derivedOrder := metadataText("01"), "56.00", "20.00", "28.00", "10.00", 56.0, 28.0
			if grouping == 14 {
				group, a, b, derivedA, derivedB, order, derivedOrder =
					ProjectMetadataCell{Storage: "null"}, "28.00", "10.00", "14.00", "5.00", 28, 14
			}
			if average == 2 {
				a, b, derivedA, derivedB, order, derivedOrder = "37.33", "10.00", "37.33", "10.00", 37.33, 37.33
				if grouping == 14 {
					a, b, derivedA, derivedB, order, derivedOrder = "18.67", "5.00", "18.67", "5.00", 18.67, 18.67
				}
			}
			expected := [][]ProjectMetadataCell{
				{group, metadataText("Alpha"), strataReal(order), metadataText(a), metadataText(b)},
				{group, metadataText("X"), strataReal(derivedOrder), metadataText(derivedA), metadataText(derivedB)},
			}
			if grouping == 13 {
				null := ProjectMetadataCell{Storage: "null"}
				unknownA, unknownB, unknownDerivedA, unknownDerivedB, unknownOrder, unknownDerivedOrder :=
					"28.00", "10.00", "14.00", "5.00", 28.0, 14.0
				if average == 2 {
					unknownA, unknownB, unknownDerivedA, unknownDerivedB, unknownOrder, unknownDerivedOrder =
						"18.67", "5.00", "18.67", "5.00", 18.67, 18.67
				}
				expected = append([][]ProjectMetadataCell{
					{null, metadataText("Alpha"), strataReal(unknownOrder), metadataText(unknownA), metadataText(unknownB)},
					{null, metadataText("X"), strataReal(unknownDerivedOrder), metadataText(unknownDerivedA), metadataText(unknownDerivedB)},
				}, expected...)
			}
			assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, expected)
			if got.Report.Units[0].NumPlots != 4 || got.Report.Units[1].NumPlots != 1 {
				t.Fatal("effective attribute reference fanout changed fixed denominators")
			}
			derived, duplicate := false, false
			for _, row := range got.Report.Rows {
				if row.CommonName != nil {
					t.Fatal("unselected First(common name) was invented", row)
				}
				for _, witness := range row.Witnesses {
					duplicate = duplicate || reflect.DeepEqual(witness.RawVegRowIDs, []string{"2", "4"})
					for _, origin := range witness.ReferenceOrigins {
						derived = derived || origin.Table == "USysSppAttributeDescription" && origin.RowID == "11"
					}
				}
			}
			if !derived || !duplicate {
				t.Fatal("derived physical origin or raw MAX peer provenance lost", grouping, derived, duplicate)
			}
			got.Report.Rows[0].Witnesses[0].RawVegRowIDs[0] = "detached"
			got.Report.Rows[0].Witnesses[0].ReferenceOrigins[0].RowID = "detached"
			after, err := json.Marshal([]any{veg, su, master, personal, layers, plan})
			if err != nil || string(before) != string(after) {
				t.Fatal("grouped attribute output aliases or changes source authority", err)
			}
		}
	}
}

func TestSummaryGroupedAttributeReferencePolicyAndQualifiedModes(t *testing.T) {
	allowed := map[int][2]bool{11: {true, true}, 12: {true, true}, 13: {true, false}, 14: {true, false}}
	for grouping := 0; grouping <= 15; grouping++ {
		for index, common := range []bool{false, true} {
			if summaryVegetationReferenceGroupingAvailable(grouping, common) != allowed[grouping][index] {
				t.Fatal("reference grouping policy differs", grouping, common)
			}
		}
	}
	for _, grouping := range []int{13, 14} {
		veg, su, master, _, layers, _ := summaryStrataReferenceFixture(t, SummaryVegetationReferenceOptions{})
		personal := lifeformReferenceFixture()
		descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}}}
		plan, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions, SummaryVegetationReferenceOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, average := range []int{1, 2} {
			for _, mode := range [][2]int{{1, 0}, {2, 0}, {3, 0}, {4, 1}, {4, 10}, {4, 20}, {5, 1}, {5, 10}, {5, 20}, {6, 0}} {
				got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, average, grouping, mode[0], mode[1], &plan)
				legacy, legacyErr := prepareSummaryVegetationGroupedOptions(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, average, grouping, mode[0], mode[1])
				if grouping == 13 && mode[0] == 2 {
					if err == nil || legacyErr == nil || !strings.Contains(err.Error(), "CDbl composite criteria type mismatch") ||
						!reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
						t.Fatal("original Lifeform Display2 failure changed", got, err, legacyErr)
					}
				} else if err != nil || legacyErr != nil || !reflect.DeepEqual(got, legacy) {
					t.Fatal("qualified calculation, group types or provenance changed", grouping, average, mode, err, legacyErr)
				}
			}
		}
		for _, common := range []bool{false, true} {
			for _, guide := range []bool{false, true} {
				if !common && !guide {
					continue
				}
				plan.Review.ShowCommon, plan.Review.UseGuideNames = common, guide
				got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, 1, grouping, 6, 0, &plan)
				if err == nil || !strings.Contains(err.Error(), "First(common name)") ||
					!reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
					t.Fatal("unqualified common/guide selection inferred", grouping, common, guide, got, err)
				}
			}
		}
		plan.Review.ShowCommon, plan.Review.UseGuideNames = false, false
		probe := &projectUnitCancellationContext{Context: t.Context()}
		if _, err := prepareSummaryVegetationGroupedReferences(probe, "Sample", "Selected",
			veg, su, master, personal, layers, 1, grouping, 6, 0, &plan); err != nil {
			t.Fatal(err)
		}
		for _, point := range []int{1, probe.calls / 2, probe.calls} {
			ctx := &projectUnitCancellationContext{Context: t.Context(), cancel: point}
			got, err := prepareSummaryVegetationGroupedReferences(ctx, "Sample", "Selected",
				veg, su, master, personal, layers, 1, grouping, 6, 0, &plan)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
				t.Fatal("cancelled grouped attribute preparation returned partial output", grouping, point, got, err)
			}
		}
	}
}
