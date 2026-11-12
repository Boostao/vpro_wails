package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryConstancyReferencePlan(t *testing.T, master, personal vprocore.ProjectMetadataTable) summaryVegetationReferencePlan {
	t.Helper()
	physical := vprocore.ProjectMetadataTable{
		Columns: append(append([]vprocore.ProjectMetadataColumn{}, master.Columns...), vprocore.ProjectMetadataColumn{Name: "IsAttribute"}),
		Rows:    []vprocore.ProjectMetadataRow{},
	}
	for _, row := range master.Rows {
		cells := append(append([]vprocore.ProjectMetadataCell{}, row.Cells...), vprotest.MetadataInteger("0"))
		physical.Rows = append(physical.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	descriptions := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "ShortName"}, {Name: "Definition"}},
		Rows: []vprocore.ProjectMetadataRow{}}
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
				stage.Rows[0].Cells[3], stage.Rows[1].Cells[3] = vprotest.StrataReal(14), vprotest.StrataReal(28)
				for i := range stage.Rows {
					stage.Rows[i].Cells[1] = vprotest.MetadataText("01")
				}
				stage.Rows[3].Cells[1] = vprotest.MetadataText("?")
				appendStage := func(plot, layer, species string, cover float64) {
					stage.Rows = append(stage.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(len(stage.Rows) + 1),
						Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(plot), vprotest.MetadataText(layer), vprotest.MetadataText(species), vprotest.StrataReal(cover)}})
				}
				appendMaster := func(code, name, form string) {
					master.Rows = append(master.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(len(master.Rows) + 1),
						Cells: vprotest.LifeformReferenceRow(code, name, vprotest.MetadataInteger(form), vprotest.MetadataText(code), vprotest.MetadataText("U"))})
				}
				switch scenario {
				case "null-lifeform":
					master.Rows[0].Cells[2] = vprocore.ProjectMetadataCell{Storage: "null"}
				case "same-display-identical-values":
					appendMaster("Y", "Alpha", "1")
					appendStage("P1", "01", "Y", 14)
					appendStage("P2", "01", "Y", 28)
					appendStage("P3", "01", "Y", 5)
				case "same-display-distinct-values":
					appendMaster("Y", "Alpha", "1")
					appendStage("P2", "01", "Y", 3)
				case "numeric-policies":
					master.Rows[0].Cells[2] = vprotest.MetadataInteger("128")
					appendMaster("Z", "Unselected wide master", "2147483647")
					personal = sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("U", "Personal negative",
						vprotest.MetadataInteger("-1"), vprotest.MetadataText("u"), vprotest.MetadataText("U")))
					for i := 0; i < 3; i++ {
						stage.Rows[i].Cells[1] = vprotest.MetadataText("128")
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
				null := vprocore.ProjectMetadataCell{Storage: "null"}
				alpha := []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("1.00"),
					vprotest.MetadataText("0.75 - 14.00"), vprotest.MetadataText("1.00 - 5.00")}
				missing := []vprocore.ProjectMetadataCell{null, vprotest.MetadataText("S"), vprotest.MetadataText("0.25"), vprotest.MetadataText("0.25 - 0.50"), null}
				if average == 2 {
					alpha[3], missing[3] = vprotest.MetadataText("0.75 - 18.67"), vprotest.MetadataText("0.25 - 2.00")
				}
				expected := [][]vprocore.ProjectMetadataCell{missing, alpha}
				rawCodes := []string{"S", "X"}
				switch scenario {
				case "null-lifeform":
					alpha[0] = null
					expected, rawCodes = [][]vprocore.ProjectMetadataCell{alpha, missing}, []string{"X", "S"}
				case "same-display-identical-values":
					expected, rawCodes = [][]vprocore.ProjectMetadataCell{missing, alpha, alpha}, []string{"S", "X", "Y"}
				case "same-display-distinct-values":
					value := "0.25 - 0.75"
					if average == 2 {
						value = "0.25 - 3.00"
					}
					expected = append(expected, []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.MetadataText("Alpha"),
						vprotest.MetadataText("0.25"), vprotest.MetadataText(value), null})
					rawCodes = append(rawCodes, "Y")
				case "numeric-policies":
					alpha[0] = vprotest.MetadataInteger("128")
					value := "0.50 - 3.00"
					if average == 2 {
						value = "0.50 - 6.00"
					}
					expected = [][]vprocore.ProjectMetadataCell{missing, {vprotest.MetadataInteger("-1"), vprotest.MetadataText("Personal negative"),
						vprotest.MetadataText("0.50"), vprotest.MetadataText(value), null}, alpha}
					rawCodes = []string{"S", "U", "X"}
				}
				assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, expected)
				for i, row := range got.Report.Rows {
					if row.RawSpecies == nil || !reflect.DeepEqual(*row.RawSpecies, vprotest.MetadataText(rawCodes[i])) ||
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
				personal = sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("X", "Alpha", vprotest.MetadataInteger("1"),
					vprotest.MetadataText("alternate"), vprotest.MetadataText("U")))
			}
			master.Rows = append(master.Rows, vprocore.ProjectMetadataRow{RowID: "99",
				Cells: vprotest.LifeformReferenceRow("Z", "Unselected wide master", vprotest.MetadataInteger("2147483647"),
					vprotest.MetadataText("z"), vprotest.MetadataText("U"))})
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
			assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]vprocore.ProjectMetadataCell{
				{vprotest.MetadataInteger("1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText(prefix), vprotest.MetadataText(a), vprotest.MetadataText(b)},
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
			master.Rows[0].Cells[2] = vprotest.MetadataInteger("2147483647")
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
		assertSummaryLayerRows(t, got.Report, []string{"A", "B"}, [][]vprocore.ProjectMetadataCell{
			{{Storage: "null"}, vprotest.MetadataText("X"), vprotest.MetadataText("2.00"), vprotest.MetadataText(derivedA), vprotest.MetadataText(derivedB)},
			{vprotest.MetadataInteger("1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText("4.00"), vprotest.MetadataText(alphaA), vprotest.MetadataText(alphaB)},
		})
		for _, row := range got.Report.Rows {
			if row.RawSpecies == nil || !reflect.DeepEqual(*row.RawSpecies, vprotest.MetadataText("X")) {
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
		cell := vprotest.MetadataInteger(value)
		if _, err := vegetationLifeformInsertionLayer(cell); err == nil {
			t.Fatal("legacy insertion reader widened", value)
		}
		layer, err := vegetationLifeformInsertionLayerWithInteger(cell, summaryVegetationMasterLongInteger)
		if err != nil || layer.Text == nil || *layer.Text != value {
			t.Fatal("explicit master LONG formatting differs", value, layer, err)
		}
		union := sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("Z", "Wide", cell, vprotest.MetadataText("z"), vprotest.MetadataText("U")))
		if _, err := reportcore.VegetationLifeformReferenceSchema(t.Context(), union); err == nil {
			t.Fatal("legacy reference schema widened", value)
		}
		if _, err := reportcore.VegetationLifeformReferenceSourceSchemaWithInteger(t.Context(), union, "Lifeform",
			summaryVegetationMasterLongInteger); err != nil {
			t.Fatal("explicit master LONG schema rejected", value, err)
		}
	}
	_, prepared := summaryRawGroupFixture(t)
	prepared.ReducedRows = prepared.ReducedRows[:3]
	union := sftest.LifeformReferenceFixture(
		vprotest.LifeformReferenceRow("X", "Alpha", vprotest.MetadataInteger("1"), vprotest.MetadataText("x"), vprotest.MetadataText("U")),
		vprotest.LifeformReferenceRow("Z", "Unused", vprotest.MetadataInteger("2147483647"), vprotest.MetadataText("z"), vprotest.MetadataText("U")))
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
	for _, value := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.StrataReal(1), vprotest.MetadataInteger("2147483648")} {
		bad := plan
		bad.References.Table.Rows = append([]vprocore.ProjectMetadataRow{}, plan.References.Table.Rows...)
		bad.References.Table.Rows[0].Cells = append([]vprocore.ProjectMetadataCell{}, plan.References.Table.Rows[0].Cells...)
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
