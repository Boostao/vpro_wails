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

func summaryCommonFirstFixture(t *testing.T, x, y ProjectMetadataCell, guide bool) (
	raw, su, stage, master, personal, layers ProjectMetadataTable, plan summaryVegetationReferencePlan) {
	t.Helper()
	_, su, stage, _, _, _ = summaryLayerFixture()
	stage.Rows = nil
	for i, value := range []struct {
		plot, species string
		cover         float64
	}{{"P1", "X", 14}, {"P2", "X", 28}, {"P2", "Y", 4}, {"P3", "X", 5}, {"P3", "Y", 6}} {
		stage.Rows = append(stage.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1),
			Cells: []ProjectMetadataCell{metadataText(value.plot), metadataText("01"), metadataText(value.species), strataReal(value.cover)}})
	}
	master = lifeformReferenceFixture(lifeformReferenceRow("X", "Alpha", metadataInteger("1"), x, metadataText("U")))
	master.Columns = append(master.Columns, ProjectMetadataColumn{Name: "IsAttribute"}, ProjectMetadataColumn{Name: "CombinedEnglishName"})
	master.Rows[0].Cells = append(master.Rows[0].Cells, metadataInteger("0"), cloneSiteUnitCell(x))
	personal = lifeformReferenceFixture(lifeformReferenceRow("Y", "Alpha", metadataInteger("1"), y, metadataText("U")))
	layers = summaryGroupedLayers()
	layers.Columns = append(layers.Columns, ProjectMetadataColumn{Name: "LayerText"})
	for i := range layers.Rows {
		layers.Rows[i].Cells = append(layers.Rows[i].Cells, cloneSiteUnitCell(layers.Rows[i].Cells[0]))
	}
	raw = ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}, Rows: []ProjectMetadataRow{}}
	for _, name := range longVegetationCoverColumns() {
		raw.Columns = append(raw.Columns, ProjectMetadataColumn{Name: name})
	}
	for _, row := range stage.Rows {
		cells := []ProjectMetadataCell{cloneSiteUnitCell(row.Cells[0]), cloneSiteUnitCell(row.Cells[2]), cloneSiteUnitCell(row.Cells[3])}
		for len(cells) < len(raw.Columns) {
			cells = append(cells, ProjectMetadataCell{Storage: "null"})
		}
		raw.Rows = append(raw.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	cells := make([]ProjectMetadataCell, len(raw.Rows[1].Cells))
	for i, cell := range raw.Rows[1].Cells {
		cells[i] = cloneSiteUnitCell(cell)
	}
	cells[2] = strataReal(1)
	raw.Rows = append(raw.Rows, ProjectMetadataRow{RowID: "6", Cells: cells})
	plan = summaryCommonFirstPlan(t, master, personal, guide)
	return
}

func summaryCommonFirstPlan(t *testing.T, master, personal ProjectMetadataTable, guide bool) summaryVegetationReferencePlan {
	t.Helper()
	descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}},
		Rows: []ProjectMetadataRow{}}
	plan, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions,
		SummaryVegetationReferenceOptions{ShowCommon: !guide, UseGuideNames: guide})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestSummaryCommonFirstOriginalProjectedQueries(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	for _, test := range []struct {
		name         string
		x, y, common ProjectMetadataCell
		ambiguous    bool
	}{
		{"uniform-text", metadataText("Common"), metadataText("Common"), metadataText("Common"), false},
		{"uniform-null", null, null, null, false},
		{"uniform-empty", metadataText(""), metadataText(""), null, false},
		{"mixed-text", metadataText("Common X"), metadataText("Common Y"), null, true},
		{"null-empty", null, metadataText(""), null, true},
		{"unit-separated", metadataText("Common X"), metadataText("Common Y"), null, true},
	} {
		for _, grouping := range []int{13, 14} {
			for _, average := range []int{1, 2} {
				t.Run(test.name+"/"+strconv.Itoa(grouping)+"/"+strconv.Itoa(average), func(t *testing.T) {
					raw, su, stage, master, personal, layers, plan := summaryCommonFirstFixture(t, test.x, test.y, false)
					if test.name == "unit-separated" {
						stage.Rows = []ProjectMetadataRow{stage.Rows[0], stage.Rows[1], stage.Rows[4]}
						raw.Rows = []ProjectMetadataRow{raw.Rows[0], raw.Rows[1], raw.Rows[4], raw.Rows[5]}
					}
					inputs := []any{raw, su, stage, master, personal, layers, plan}
					before, err := json.Marshal(inputs)
					if err != nil {
						t.Fatal(err)
					}
					for _, call := range []func() (summaryVegetationGroupedReport, error){
						func() (summaryVegetationGroupedReport, error) {
							return planSummaryVegetationUnanimousCommonStage(t.Context(), raw, su, stage, master, personal, layers,
								average, grouping, &plan)
						},
						func() (summaryVegetationGroupedReport, error) {
							return prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
								raw, su, master, personal, layers, average, grouping, &plan)
						},
					} {
						got, err := call()
						if test.ambiguous {
							if err == nil || !strings.Contains(err.Error(), "ambiguous") || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
								t.Fatal("ambiguous First chose/split a name or leaked output", got, err)
							}
							continue
						}
						if err != nil {
							t.Fatal(err)
						}
						field, group := summaryVegetationGroupField{Name: "LifeForm", DAOType: 10, Size: 3}, metadataText("01")
						if grouping == 14 {
							field, group = summaryVegetationGroupField{Name: "Group", DAOType: 9}, null
						}
						b := "11.00"
						if average == 2 {
							b = "5.50"
						}
						assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]ProjectMetadataCell{
							{group, metadataText("Alpha"), strataReal(15), metadataText("15.00"), metadataText(b)},
						})
						if got.GroupField != field || got.OrderField != (summaryVegetationGroupField{Name: "Order", DAOType: 7, Size: 8}) ||
							got.Report.Rows[0].CommonName == nil || !reflect.DeepEqual(*got.Report.Rows[0].CommonName, test.common) ||
							got.Report.Units[0].NumPlots != 4 || got.Report.Units[1].NumPlots != 1 {
							t.Fatal("original DAO shape/common First/fixed denominators differ", got)
						}
						if len(got.Report.Rows[0].Witnesses) != 6 {
							t.Fatal("physical membership/reference witnesses lost", got.Report.Rows[0].Witnesses)
						}
					}
					after, err := json.Marshal(inputs)
					if err != nil || string(before) != string(after) {
						t.Fatal("new preparation mutated typed inputs", err)
					}
				})
			}
		}
	}
}

func TestSummaryCommonFirstGroupingGuideAndUnusedDifferences(t *testing.T) {
	for _, grouping := range []int{13, 14} {
		for _, scenario := range []string{"scientific-groups", "lifeform-groups", "unused", "guide"} {
			t.Run(strconv.Itoa(grouping)+"/"+scenario, func(t *testing.T) {
				raw, su, _, master, personal, layers, _ := summaryCommonFirstFixture(t, metadataText("Common X"), metadataText("Common Y"), false)
				guide, rows := false, 2
				switch scenario {
				case "scientific-groups":
					personal.Rows[0].Cells[1] = metadataText("Beta")
				case "lifeform-groups":
					personal.Rows[0].Cells[2] = metadataInteger("2")
				case "unused":
					personal.Rows[0].Cells[3] = metadataText("Common X")
					personal.Rows = append(personal.Rows, ProjectMetadataRow{RowID: "2",
						Cells: lifeformReferenceRow("Z", "Alpha", metadataInteger("1"), metadataText("Unused"), metadataText("U"))})
					rows = 1
				case "guide":
					guide, rows = true, 1
					master.Rows[0].Cells[6] = metadataText("Common Y")
				}
				plan := summaryCommonFirstPlan(t, master, personal, guide)
				got, err := prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
					raw, su, master, personal, layers, 1, grouping, &plan)
				if scenario == "lifeform-groups" && grouping == 14 {
					if err == nil || !strings.Contains(err.Error(), "ambiguous") || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
						t.Fatal("None incorrectly used Lifeform as a group", got, err)
					}
					return
				}
				if err != nil || len(got.Report.Rows) != rows {
					t.Fatal("original grouping, effective guide or unused difference refused", got, err)
				}
				for _, row := range got.Report.Rows {
					if row.CommonName == nil || row.CommonName.Text == nil {
						t.Fatal("accepted nonempty common name lost", row)
					}
					if scenario == "guide" && *row.CommonName.Text != "Common Y" {
						t.Fatal("guide used master EnglishName instead of effective CombinedEnglishName", row)
					}
				}
				if summaryVegetationReferenceGroupingAvailable(grouping, true) {
					t.Fatal("private preparation enabled public grouped names")
				}
				legacy, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
					raw, su, master, personal, layers, 1, grouping, 6, 0, &plan)
				if err == nil || !reflect.DeepEqual(legacy, summaryVegetationGroupedReport{}) {
					t.Fatal("ordinary caller silently gained First selection", legacy, err)
				}
			})
		}
	}
}

func TestSummaryCommonFirstExactIdentityAndRetry(t *testing.T) {
	for _, value := range []ProjectMetadataCell{metadataText("common"), metadataText("Common "), metadataText(""), {Storage: "null"}} {
		raw, su, _, master, personal, layers, _ := summaryCommonFirstFixture(t, metadataText("Common"), value, false)
		plan := summaryCommonFirstPlan(t, master, personal, false)
		got, err := prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
			raw, su, master, personal, layers, 1, 13, &plan)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("literal/typed name mismatch was folded or defaulted", got, err)
		}
		personal.Rows[0].Cells[3] = metadataText("Common")
		plan = summaryCommonFirstPlan(t, master, personal, false)
		got, err = prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
			raw, su, master, personal, layers, 1, 13, &plan)
		if err != nil || len(got.Report.Rows) != 1 {
			t.Fatal("corrected common input did not retry", got, err)
		}
	}
}

func TestSummaryCommonFirstUnionProvenanceAndDetachment(t *testing.T) {
	raw, su, _, master, personal, layers, _ := summaryCommonFirstFixture(t, metadataText("Common"), metadataText("Common"), false)
	master.Rows = append(master.Rows, ProjectMetadataRow{RowID: "2", Cells: append([]ProjectMetadataCell{}, master.Rows[0].Cells...)})
	personal.Rows = append(personal.Rows, ProjectMetadataRow{RowID: "2",
		Cells: lifeformReferenceRow("X", "Alpha", metadataInteger("1"), metadataText("Common"), metadataText("U"))})
	plan := summaryCommonFirstPlan(t, master, personal, false)
	inputs := []any{raw, su, master, personal, layers, plan}
	before, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
		raw, su, master, personal, layers, 1, 13, &plan)
	if err != nil || len(got.Report.Rows) != 1 || plan.Review.UnionRows != 2 {
		t.Fatal("whole-row UNION duplicates multiplied or lost output", got, err)
	}
	assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]ProjectMetadataCell{
		{metadataText("01"), metadataText("Alpha"), strataReal(15), metadataText("15.00"), metadataText("11.00")},
	})
	sawUnion, sawMax := false, false
	for _, witness := range got.Report.Rows[0].Witnesses {
		if reflect.DeepEqual(witness.ReferenceOrigins, []vegetationLifeformReferenceOrigin{
			{"USysAllSpecs", "1"}, {"USysAllSpecs", "2"}, {"USysUserSpp", "2"},
		}) {
			sawUnion = true
		}
		sawMax = sawMax || reflect.DeepEqual(witness.RawVegRowIDs, []string{"2", "6"})
	}
	if !sawUnion || !sawMax {
		t.Fatal("complete UNION or lower MAX peer origins lost", got)
	}
	*got.Report.Rows[0].CommonName.Text = "changed"
	*got.Report.Rows[0].Layer.Text = "changed"
	*got.Report.Rows[0].Covers[0].Text = "changed"
	got.Report.Rows[0].Witnesses[0].ReferenceOrigins[0].RowID = "changed"
	got.Report.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
	after, err := json.Marshal(inputs)
	if err != nil || string(before) != string(after) {
		t.Fatal("result aliases physical sources/plan", err)
	}
	retry, err := prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
		raw, su, master, personal, layers, 1, 13, &plan)
	if err != nil || retry.Report.Rows[0].CommonName.Text == nil || *retry.Report.Rows[0].CommonName.Text != "Common" {
		t.Fatal("detached-result retry changed", retry, err)
	}
}

func TestSummaryCommonFirstPhysicalFanout(t *testing.T) {
	for _, grouping := range []int{13, 14} {
		for _, average := range []int{1, 2} {
			raw, su, _, master, personal, layers, _ := summaryCommonFirstFixture(t, metadataText("Common"), metadataText("Common"), false)
			personal.Rows = append(personal.Rows, ProjectMetadataRow{RowID: "2",
				Cells: lifeformReferenceRow("X", "Alpha", metadataInteger("2"), metadataText("Common"), metadataText("U"))})
			plan := summaryCommonFirstPlan(t, master, personal, false)
			got, err := prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
				raw, su, master, personal, layers, average, grouping, &plan)
			if err != nil {
				t.Fatal(err)
			}
			a, b, secondA, secondB, order, secondOrder := "29.00", "16.00", "28.00", "10.00", 29.0, 28.0
			if average == 2 {
				a, b, secondA, secondB, order, secondOrder = "16.57", "5.33", "18.67", "5.00", 16.57, 18.67
			}
			group := metadataText("01")
			if grouping == 14 {
				group = ProjectMetadataCell{Storage: "null"}
			}
			expected := [][]ProjectMetadataCell{{group, metadataText("Alpha"), strataReal(order), metadataText(a), metadataText(b)}}
			if grouping == 13 {
				expected = append(expected, []ProjectMetadataCell{
					metadataText("02"), metadataText("Alpha"), strataReal(secondOrder), metadataText(secondA), metadataText(secondB),
				})
			}
			assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, expected)
			for i, row := range got.Report.Rows {
				witnesses := 10
				if i == 1 {
					witnesses = 8
				}
				if len(row.Witnesses) != witnesses || row.CommonName == nil ||
					!reflect.DeepEqual(*row.CommonName, metadataText("Common")) {
					t.Fatal("First collapsed physical source fanout/provenance", got)
				}
			}
		}
	}
}

func TestSummaryCommonFirstContributingRowsOnly(t *testing.T) {
	for _, grouping := range []int{13, 14} {
		raw, su, stage, master, personal, layers, plan := summaryCommonFirstFixture(t, metadataText("Common X"), metadataText("Common Y"), false)
		stage.Rows = []ProjectMetadataRow{stage.Rows[0], stage.Rows[1], stage.Rows[3], stage.Rows[4]}
		stage.Rows[3].Cells[0] = metadataText("P9")
		raw.Rows = []ProjectMetadataRow{raw.Rows[0], raw.Rows[1], raw.Rows[3], raw.Rows[4], raw.Rows[5]}
		raw.Rows[3].Cells[0] = metadataText("P9")
		for _, staged := range []bool{false, true} {
			var got summaryVegetationGroupedReport
			var err error
			if staged {
				got, err = planSummaryVegetationUnanimousCommonStage(t.Context(), raw, su, stage, master, personal, layers, 1, grouping, &plan)
			} else {
				got, err = prepareSummaryVegetationUnanimousCommonReferences(t.Context(), "Sample", "Selected",
					raw, su, master, personal, layers, 1, grouping, &plan)
			}
			if err != nil || len(got.Report.Rows) != 1 || got.Report.Rows[0].CommonName == nil ||
				!reflect.DeepEqual(*got.Report.Rows[0].CommonName, metadataText("Common X")) {
				t.Fatal("noncontributing physical difference blocked original group", grouping, staged, got, err)
			}
		}
		stage.Rows[3].Cells[0] = metadataText("P2")
		stage.Rows[3].Cells[3] = ProjectMetadataCell{Storage: "null"}
		got, err := planSummaryVegetationUnanimousCommonStage(t.Context(), raw, su, stage, master, personal, layers, 1, grouping, &plan)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("contributing NULL-cover First input was silently omitted", got, err)
		}
	}
}

func TestSummaryCommonFirstCancellationAndGuards(t *testing.T) {
	raw, su, stage, master, personal, layers, plan := summaryCommonFirstFixture(t, metadataText("Common"), metadataText("Common"), false)
	for _, grouping := range []int{13, 14} {
		for _, staged := range []bool{false, true} {
			call := func(ctx context.Context) (summaryVegetationGroupedReport, error) {
				if staged {
					return planSummaryVegetationUnanimousCommonStage(ctx, raw, su, stage, master, personal, layers, 1, grouping, &plan)
				}
				return prepareSummaryVegetationUnanimousCommonReferences(ctx, "Sample", "Selected",
					raw, su, master, personal, layers, 1, grouping, &plan)
			}
			probe := &projectUnitCancellationContext{Context: context.Background()}
			if _, err := call(probe); err != nil {
				t.Fatal(err)
			}
			for point := 1; point <= probe.calls; point++ {
				got, err := call(&projectUnitCancellationContext{Context: context.Background(), cancel: point})
				if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
					t.Fatal("cancellation leaked First output", grouping, staged, point, got, err)
				}
			}
			if got, err := call(t.Context()); err != nil || len(got.Report.Rows) != 1 {
				t.Fatal("cancelled operation poisoned retry", got, err)
			}
			if got, err := call(nil); err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
				t.Fatal("nil context returned First output", got, err)
			}
		}
	}
	for _, test := range []struct {
		group, display, class int
		mode                  summaryVegetationGroupedMode
	}{{12, 6, 0, summaryVegetationGroupedUnanimousCommon}, {13, 1, 0, summaryVegetationGroupedUnanimousCommon},
		{14, 2, 0, summaryVegetationGroupedUnanimousCommon}, {14, 6, 10, summaryVegetationGroupedUnanimousCommon},
		{13, 6, 0, 255}} {
		got, err := prepareSummaryVegetationGroupedQuery(t.Context(), "Sample", "Selected", raw, su, master, personal, layers,
			1, test.group, test.display, test.class, &plan, test.mode)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("unqualified First mode accepted", test, got, err)
		}
	}
	bad := plan
	bad.Review.ShowCommon = false
	for _, reference := range []*summaryVegetationReferencePlan{nil, &bad} {
		got, err := planSummaryVegetationUnanimousCommonStage(t.Context(), raw, su, stage, master, personal, layers, 1, 13, reference)
		if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
			t.Fatal("missing effective common plan accepted", got, err)
		}
	}
	plan.References.Table.Rows[0].Cells[3] = metadataText("\xff")
	got, err := planSummaryVegetationUnanimousCommonStage(t.Context(), raw, su, stage, master, personal, layers, 1, 13, &plan)
	if err == nil || !reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
		t.Fatal("malformed source common name was repaired", got, err)
	}
	plan.References.Table.Rows[0].Cells[3] = metadataText("Common")
	for _, projection := range []summaryVegetationStageProjection{
		{unanimousCommon: true},
		{unanimousCommon: true, references: &plan},
		{unanimousCommon: true, references: &plan, numericOrder: true, constancy: true},
	} {
		result, err := planSummaryVegetationStage(t.Context(), raw, su, stage, master, personal, layers, 1, 6, 0, projection)
		if err == nil || !reflect.DeepEqual(result, summaryVegetationLayerReport{}) {
			t.Fatal("shared aggregation bypassed First mode invariants", result, err)
		}
	}
}
