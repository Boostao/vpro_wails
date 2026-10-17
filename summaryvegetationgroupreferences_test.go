package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func summaryStrataReferenceFixture(t *testing.T, options SummaryVegetationReferenceOptions) (
	veg, su, master, personal, layers ProjectMetadataTable, plan summaryVegetationReferencePlan) {
	t.Helper()
	veg, su, master, personal, layers = summaryGroupedRawTables(t)
	master.Columns = append(master.Columns, ProjectMetadataColumn{Name: "IsAttribute"},
		ProjectMetadataColumn{Name: "CombinedEnglishName"})
	for i := range master.Rows {
		master.Rows[i].Cells = append(master.Rows[i].Cells, metadataInteger("0"), metadataText("Guide"))
	}
	personal = lifeformReferenceFixture(lifeformReferenceRow("X", "Alpha", metadataInteger("1"),
		metadataText("Personal"), metadataText("U")))
	descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "ShortName"}, {Name: "Definition"},
	}, Rows: []ProjectMetadataRow{{RowID: "11", Cells: []ProjectMetadataCell{metadataText("X"), metadataText("Definition")}}}}
	var err error
	plan, err = prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions, options)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestSummaryStrataEffectiveReferenceIndependentValues(t *testing.T) {
	for _, common := range []bool{false, true} {
		for _, guide := range []bool{false, true} {
			for _, average := range []int{1, 2} {
				veg, su, master, personal, layers, plan := summaryStrataReferenceFixture(t,
					SummaryVegetationReferenceOptions{ShowCommon: common, UseGuideNames: guide})
				got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, average, 12, 6, 0, &plan)
				if err != nil {
					t.Fatal(common, guide, average, err)
				}
				a, b, strataB, derivedA, derivedB, derivedStrataB := "12.00", "5.00", "2.00", "12.00", "5.00", "2.00"
				if average == 2 {
					a, strataB, derivedA, derivedStrataB = "16.00", "4.00", "16.00", "4.00"
				}
				if !common && !guide && average == 1 {
					a, b, strataB = "24.00", "10.00", "4.00"
				}
				type expectedRow struct {
					layer, species, common, a, b string
					order                        float64
				}
				order, orderB, derivedOrder, derivedOrderB := 12.0, 2.0, 12.0, 2.0
				if average == 2 {
					order, orderB, derivedOrder, derivedOrderB = 16, 4, 16, 4
				} else if !common && !guide {
					order, orderB = 24, 4
				}
				expected := []expectedRow{
					{"A", "Alpha", "", a, b, order}, {"A", "X", "", derivedA, derivedB, derivedOrder},
					{"B", "Alpha", "", strataB, "", orderB}, {"B", "X", "", derivedStrataB, "", derivedOrderB},
				}
				if common || guide {
					masterName, derivedName := "x", "Definition"
					if guide {
						masterName, derivedName = "Guide", ""
					}
					first, second := "Personal", masterName
					if guide {
						first, second = masterName, "Personal"
					}
					expected = []expectedRow{
						{"A", "Alpha", first, a, b, order}, {"A", "Alpha", second, a, b, order},
						{"A", "X", derivedName, derivedA, derivedB, derivedOrder},
						{"B", "Alpha", first, strataB, "", orderB}, {"B", "Alpha", second, strataB, "", orderB},
						{"B", "X", derivedName, derivedStrataB, "", derivedOrderB},
					}
				}
				if got.GroupField != (summaryVegetationGroupField{Name: "Layer", DAOType: 10, Size: 1}) ||
					got.OrderField != (summaryVegetationGroupField{Name: "Order", DAOType: 7, Size: 8}) ||
					!reflect.DeepEqual(got.Report.Columns, []string{"A", "B"}) || len(got.Report.Rows) != len(expected) ||
					got.Report.Units[0].NumPlots != 4 || got.Report.Units[1].NumPlots != 1 {
					t.Fatal("independent Strata shape/denominators differ", got)
				}
				sawDerived, sawRawDuplicate := false, false
				for i, want := range expected {
					row := got.Report.Rows[i]
					coverB := ProjectMetadataCell{Storage: "null"}
					if want.b != "" {
						coverB = metadataText(want.b)
					}
					if !reflect.DeepEqual(row.Layer, metadataText(want.layer)) ||
						!reflect.DeepEqual(row.Species, metadataText(want.species)) ||
						!reflect.DeepEqual(row.Order, strataReal(want.order)) ||
						!reflect.DeepEqual(row.Covers, []ProjectMetadataCell{metadataText(want.a), coverB}) {
						t.Fatal("independent formatted covers/DOUBLE Order differ", common, guide, average, i, row, want)
					}
					if common || guide {
						value := metadataText(want.common)
						if guide && want.species == "X" {
							value = ProjectMetadataCell{Storage: "null"}
						}
						if row.CommonName == nil || !reflect.DeepEqual(*row.CommonName, value) {
							t.Fatal("effective common/guide identity differs", row.CommonName, value)
						}
					} else if row.CommonName != nil {
						t.Fatal("unrequested common name leaked")
					}
					for _, witness := range row.Witnesses {
						if witness.ConversionLayerID != "" || len(witness.RawVegRowIDs) == 0 {
							t.Fatal("invented conversion/lost raw provenance", witness)
						}
						sawRawDuplicate = sawRawDuplicate || reflect.DeepEqual(witness.RawVegRowIDs, []string{"2", "4"})
						for _, origin := range witness.ReferenceOrigins {
							sawDerived = sawDerived || origin == (vegetationLifeformReferenceOrigin{"USysSppAttributeDescription", "11"})
						}
					}
				}
				if !sawDerived || !sawRawDuplicate {
					t.Fatal("derived physical metadata or lower MAX duplicate lost")
				}
			}
		}
	}
}

func TestSummaryStrataEffectiveReferenceNullEmptyAndUnionOrigins(t *testing.T) {
	veg, su, master, personal, layers, _ := summaryStrataReferenceFixture(t, SummaryVegetationReferenceOptions{})
	master.Rows[0].Cells[3] = ProjectMetadataCell{Storage: "null"}
	master.Rows = append(master.Rows, ProjectMetadataRow{RowID: "3",
		Cells: append([]ProjectMetadataCell{}, master.Rows[0].Cells...)})
	personal.Rows[0].Cells[3] = metadataText("")
	descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}}}
	plan, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions,
		SummaryVegetationReferenceOptions{ShowCommon: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
		veg, su, master, personal, layers, 1, 12, 6, 0, &plan)
	if err != nil || len(got.Report.Rows) != 4 {
		t.Fatal("NULL/empty common groups or literal UNION changed", got, err)
	}
	null := ProjectMetadataCell{Storage: "null"}
	for i, wantCommon := range []ProjectMetadataCell{null, metadataText(""), null, metadataText("")} {
		row := got.Report.Rows[i]
		group, covers, order := "A", []ProjectMetadataCell{metadataText("12.00"), metadataText("5.00")}, 12.0
		if i >= 2 {
			group, covers, order = "B", []ProjectMetadataCell{metadataText("2.00"), null}, 2
		}
		if row.CommonName == nil || !reflect.DeepEqual(*row.CommonName, wantCommon) ||
			!reflect.DeepEqual(row.Layer, metadataText(group)) || !reflect.DeepEqual(row.Species, metadataText("Alpha")) ||
			!reflect.DeepEqual(row.Covers, covers) || !reflect.DeepEqual(row.Order, strataReal(order)) {
			t.Fatal("NULL-first distinct name group differs", i, row)
		}
		if i%2 == 0 {
			for _, witness := range row.Witnesses {
				if !reflect.DeepEqual(witness.ReferenceOrigins, []vegetationLifeformReferenceOrigin{
					{"USysAllSpecs", "1"}, {"USysAllSpecs", "3"},
				}) {
					t.Fatal("physical duplicates lost or multiplied after whole-row UNION", witness)
				}
			}
		}
	}
	plan.References.Table.Rows[0].Cells[3] = metadataText("\xff")
	if result, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
		veg, su, master, personal, layers, 1, 12, 6, 0, &plan); err == nil ||
		!reflect.DeepEqual(result, summaryVegetationGroupedReport{}) {
		t.Fatal("malformed effective name returned repaired/partial output", result, err)
	}
	plan.References.Table.Rows[0].Cells[3] = null
	if result, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
		veg, su, master, personal, layers, 1, 12, 6, 0, &plan); err != nil || !reflect.DeepEqual(result, got) {
		t.Fatal("corrected effective name did not retry exactly", result, err)
	}
}

func TestSummaryStrataEffectiveReferenceReusesQualifiedDisplayAndClassModes(t *testing.T) {
	for _, common := range []bool{false, true} {
		veg, su, master, _, layers, _ := summaryStrataReferenceFixture(t, SummaryVegetationReferenceOptions{})
		personal := lifeformReferenceFixture()
		descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}}}
		plan, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions,
			SummaryVegetationReferenceOptions{ShowCommon: common})
		if err != nil {
			t.Fatal(err)
		}
		for _, average := range []int{1, 2} {
			for _, mode := range [][2]int{{1, 0}, {2, 0}, {3, 0}, {4, 1}, {4, 10}, {4, 20}, {5, 1}, {5, 10}, {5, 20}, {6, 0}} {
				legacy, legacyErr := prepareSummaryVegetationGroupedOptions(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, average, 12, mode[0], mode[1])
				got, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
					veg, su, master, personal, layers, average, 12, mode[0], mode[1], &plan)
				if mode[0] == 2 {
					if legacyErr == nil || err == nil || !strings.Contains(err.Error(), "CDbl composite criteria type mismatch") ||
						!reflect.DeepEqual(got, summaryVegetationGroupedReport{}) {
						t.Fatal("original Strata composite conversion refusal silently repaired", got, err)
					}
					continue
				}
				if err != nil || legacyErr != nil {
					t.Fatal(average, mode, err, legacyErr)
				}
				for i := range got.Report.Rows {
					name := got.Report.Rows[i].CommonName
					if common && (name == nil || !reflect.DeepEqual(*name, metadataText("x"))) ||
						!common && name != nil {
						t.Fatal("qualified display lost exact common-name selection", name)
					}
					got.Report.Rows[i].CommonName = nil
				}
				if !reflect.DeepEqual(got, legacy) {
					t.Fatal("reference composition changed existing calibrated display/class or provenance", average, mode, got, legacy)
				}
			}
		}
	}
}

func TestSummaryStrataEffectiveReferenceCancellationDetachmentAndRefusal(t *testing.T) {
	veg, su, master, personal, layers, plan := summaryStrataReferenceFixture(t,
		SummaryVegetationReferenceOptions{ShowCommon: true})
	inputs := []any{veg, su, master, personal, layers, plan}
	before, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	probe := &projectUnitCancellationContext{Context: t.Context()}
	got, err := prepareSummaryVegetationGroupedReferences(probe, "Sample", "Selected",
		veg, su, master, personal, layers, 1, 12, 6, 0, &plan)
	if err != nil {
		t.Fatal(err)
	}
	*got.Report.Rows[0].CommonName.Text = "changed"
	got.Report.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
	got.Report.Rows[0].Witnesses[0].ReferenceOrigins[0].RowID = "changed"
	after, err := json.Marshal(inputs)
	if err != nil || string(before) != string(after) {
		t.Fatal("private grouped reference output aliases source authority", err)
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: t.Context(), cancel: point}
		result, err := prepareSummaryVegetationGroupedReferences(ctx, "Sample", "Selected",
			veg, su, master, personal, layers, 1, 12, 6, 0, &plan)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, summaryVegetationGroupedReport{}) {
			t.Fatal("cancelled effective Strata preparation returned partial success", point, err)
		}
	}
	for _, group := range []int{13, 14} {
		result, err := prepareSummaryVegetationGroupedReferences(t.Context(), "Sample", "Selected",
			veg, su, master, personal, layers, 1, group, 6, 0, &plan)
		if err == nil || !strings.Contains(err.Error(), "First(common name)") ||
			!reflect.DeepEqual(result, summaryVegetationGroupedReport{}) {
			t.Fatal("unqualified First(common name) inferred", group, result, err)
		}
	}
	for _, ctx := range []context.Context{nil, func() context.Context {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		return ctx
	}()} {
		result, err := prepareSummaryVegetationGroupedReferences(ctx, "Sample", "Selected",
			veg, su, master, personal, layers, 1, 12, 6, 0, &plan)
		if err == nil || !reflect.DeepEqual(result, summaryVegetationGroupedReport{}) {
			t.Fatal("missing/cancelled context returned a source result", result, err)
		}
	}
}
