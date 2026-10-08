package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func vegetationQualityFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows: []ProjectMetadataRow{{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P"), metadataText("  Unit  ")}}}}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []ProjectMetadataRow{{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P")}}}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot"}, {Name: "SitePlotQuality"},
		{Name: "VegPlotQuality"}, {Name: "SoilPlotQuality"}},
		Rows: []ProjectMetadataRow{{RowID: "31", Cells: []ProjectMetadataCell{
			metadataText("P"), metadataText("Good"), metadataText("Good"), metadataText("Good")}}}}
	lists := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Item"}, {Name: "ListName"}, {Name: "ItemOrder"}}}
	for i, code := range []string{"Poor", "Fair", "Good", "Excellent"} {
		lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []ProjectMetadataCell{
			metadataText(code), metadataText("DataQuality"), metadataInteger(strconv.Itoa(i + 1))}})
	}
	for i, code := range []string{"NA", "Excellent", "Good", "Fair", "Poor"} {
		lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 5), Cells: []ProjectMetadataCell{
			metadataText(code), metadataText("PlotQualitySite"), metadataInteger(strconv.Itoa(i + 1))}})
	}
	return su, env, admin, lists
}

func vegetationQualityCriteria(minimum string, mask int) [3]vegetationQualityCriterion {
	var criteria [3]vegetationQualityCriterion
	for i := range criteria {
		criteria[i] = vegetationQualityCriterion{Minimum: minimum, IncludeNull: mask&(1<<i) != 0}
	}
	return criteria
}

func TestLongVegetationQualityThresholdsAndAllNullToggleCombinations(t *testing.T) {
	for _, test := range []struct {
		label string
		cell  ProjectMetadataCell
		pass  bool
		null  bool
	}{
		{"above", metadataText("Excellent"), true, false},
		{"equal", metadataText("Good"), true, false},
		{"below", metadataText("Fair"), false, false},
		{"ASCII case-insensitive", metadataText("gOoD"), true, false},
		{"NULL", ProjectMetadataCell{Storage: "null"}, false, true},
		{"unmatched", metadataText("unknown"), false, true},
		{"empty", metadataText(""), false, true},
		{"other-list-only NA", metadataText("NA"), false, false},
	} {
		for mask := 0; mask < 8; mask++ {
			for field := 0; field < 3; field++ {
				t.Run(test.label+"/"+strconv.Itoa(mask)+"/"+strconv.Itoa(field), func(t *testing.T) {
					su, env, admin, lists := vegetationQualityFixture()
					admin.Rows[0].Cells[field+1] = test.cell
					result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Good", mask))
					if err != nil {
						t.Fatal(err)
					}
					pass := test.pass || test.null && mask&(1<<field) != 0
					if pass && (len(result.Occurrences) != 1 || len(result.ExcludedMembershipIDs) != 0) ||
						!pass && (len(result.Occurrences) != 0 || !reflect.DeepEqual(result.ExcludedMembershipIDs, []string{"11"})) {
						t.Fatal("threshold/domain/NULL semantics changed", result)
					}
					if !reflect.DeepEqual(result.ThresholdRowIDs, [3][]string{{"3"}, {"3"}, {"3"}}) {
						t.Fatal("editor's reverse rank or arbitrary reference selected", result.ThresholdRowIDs)
					}
					if pass && test.null && result.Occurrences[0].ListRowIDs[field] != nil {
						t.Fatal("missing LEFT JOIN invented a reference")
					}
				})
			}
		}
	}
}

func TestLongVegetationQualityPhysicalFanoutNullDomainAndNoAliases(t *testing.T) {
	su, env, admin, lists := vegetationQualityFixture()
	lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: "10", Cells: []ProjectMetadataCell{
		metadataText("Good"), ProjectMetadataCell{Storage: "null"}, metadataInteger("4")}})
	su.Rows = append(su.Rows, ProjectMetadataRow{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P"), {Storage: "null"}}})
	result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Good", 0))
	if err != nil || len(result.Occurrences) != 16 {
		t.Fatal("two physical memberships times2^3 qualifying references not preserved", result, err)
	}
	tuples := map[string]bool{}
	for _, occurrence := range result.Occurrences {
		key := occurrence.Membership.RowID
		for _, id := range occurrence.ListRowIDs {
			if id == nil {
				t.Fatal("NULL domain with numeric order was incorrectly treated as missing reference")
			}
			key += "/" + *id
		}
		tuples[key] = true
		if occurrence.EnvRowID != "21" || occurrence.AdminRowID != "31" {
			t.Fatal("physical parent links changed", occurrence)
		}
	}
	if len(tuples) != 16 || len(result.ExcludedMembershipIDs) != 0 {
		t.Fatal("physical Cartesian provenance collapsed", tuples)
	}
	*result.Occurrences[0].Membership.SiteUnit.Text = "Changed"
	*result.Occurrences[0].Membership.PlotNumber.Text = "Other"
	*result.Occurrences[0].ListRowIDs[0] = "Altered"
	if *su.Rows[0].Cells[0].Text != "P" || *su.Rows[0].Cells[1].Text != "  Unit  " ||
		*result.Occurrences[1].Membership.SiteUnit.Text != "  Unit  " ||
		*result.Occurrences[1].ListRowIDs[0] != "3" || lists.Rows[2].RowID != "3" {
		t.Fatal("qualification aliases source/adjacent occurrences")
	}
}

func TestLongVegetationQualityNullOrderReferenceVersusNullJoin(t *testing.T) {
	for _, include := range []bool{false, true} {
		su, env, admin, lists := vegetationQualityFixture()
		admin.Rows[0].Cells[1] = metadataText("ordered NULL")
		lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: "10", Cells: []ProjectMetadataCell{
			metadataText("ordered NULL"), metadataText("DataQuality"), {Storage: "null"}}})
		criteria := vegetationQualityCriteria("Good", 0)
		criteria[0].IncludeNull = include
		result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, criteria)
		if err != nil || include && (len(result.Occurrences) != 1 || result.Occurrences[0].ListRowIDs[0] == nil ||
			*result.Occurrences[0].ListRowIDs[0] != "10") || !include && len(result.Occurrences) != 0 {
			t.Fatal("physical NULL ItemOrder reference confused with missing LEFT JOIN", result, err)
		}
	}
}

func TestLongVegetationQualityRequiresBothPhysicalParentsAndDoesNotJoinNullKeys(t *testing.T) {
	for _, missing := range []string{"env", "admin", "null", "orphan"} {
		su, env, admin, lists := vegetationQualityFixture()
		switch missing {
		case "env":
			env.Rows = nil
		case "admin":
			admin.Rows = nil
		case "null":
			su.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"}
			env.Rows[0].Cells[0], admin.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"}
		case "orphan":
			su.Rows[0].Cells[0] = metadataText("other")
		}
		result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Poor", 7))
		if err != nil || len(result.Occurrences) != 0 || !reflect.DeepEqual(result.ExcludedMembershipIDs, []string{"11"}) {
			t.Fatal("NULL/orphan was invented as an all-NULL quality plot", result, err)
		}
	}
}

func TestLongVegetationQualityThresholdAmbiguityAndMalformedInput(t *testing.T) {
	nan := math.NaN()
	for _, test := range []struct {
		label string
		cell  ProjectMetadataCell
	}{
		{"conflicting", metadataInteger("4")},
		{"NULL threshold", ProjectMetadataCell{Storage: "null"}},
		{"fractional threshold", func() ProjectMetadataCell { value := 2.5; return ProjectMetadataCell{Storage: "real", Real: &value} }()},
		{"out-of-Integer-range", metadataInteger("32768")},
		{"text order", metadataText("3")},
		{"NaN", ProjectMetadataCell{Storage: "real", Real: &nan}},
	} {
		su, env, admin, lists := vegetationQualityFixture()
		lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: "10", Cells: []ProjectMetadataCell{
			metadataText("Good"), metadataText("DataQuality"), test.cell}})
		result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Good", 7))
		if err == nil || !reflect.DeepEqual(result, vegetationQualitySelection{}) {
			t.Fatal(test.label, "accepted or returned partial qualification", result, err)
		}
	}
	for _, malformed := range []string{"missing threshold", "duplicate row ID", "short row", "missing column", "ambiguous env", "ambiguous admin", "bad quality", "bad list domain"} {
		su, env, admin, lists := vegetationQualityFixture()
		criteria := vegetationQualityCriteria("Good", 7)
		switch malformed {
		case "missing threshold":
			criteria[0].Minimum = "NA"
		case "duplicate row ID":
			lists.Rows[1].RowID = "1"
		case "short row":
			su.Rows[0].Cells = nil
		case "missing column":
			admin.Columns[1].Name = "different"
		case "ambiguous env":
			env.Rows = append(env.Rows, ProjectMetadataRow{RowID: "22", Cells: env.Rows[0].Cells})
		case "ambiguous admin":
			admin.Rows = append(admin.Rows, ProjectMetadataRow{RowID: "32", Cells: admin.Rows[0].Cells})
		case "bad quality":
			admin.Rows[0].Cells[1] = metadataInteger("3")
		case "bad list domain":
			lists.Rows[2].Cells[1] = metadataInteger("3")
		}
		result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, criteria)
		if err == nil || !reflect.DeepEqual(result, vegetationQualitySelection{}) {
			t.Fatal(malformed, "accepted or returned partial qualification", result, err)
		}
	}
	su, env, admin, lists := vegetationQualityFixture()
	lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: "10", Cells: lists.Rows[2].Cells})
	result, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("gOoD", 0))
	if err != nil || len(result.Occurrences) != 8 || !reflect.DeepEqual(result.ThresholdRowIDs[0], []string{"3", "10"}) {
		t.Fatal("identical physical threshold definitions lost or chosen arbitrarily", result, err)
	}
}

func TestLongVegetationQualityCancellationAndStrictActiveCriteria(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	su, env, admin, lists := vegetationQualityFixture()
	result, err := planLongVegetationQuality(ctx, su, env, admin, lists, vegetationQualityCriteria("Poor", 7))
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, vegetationQualitySelection{}) {
		t.Fatal("cancelled qualifier returned a partial selection", result, err)
	}

	values := longVegetationOptionsFixture(t)
	values["ReportOptions"].(map[string]any)["DataQualityFilterEnforceLV"] = true
	values["ReportOptions"].(map[string]any)["DataQualityFilterSiteLV"] = nil
	if _, err := decodeLongVegetationOptions(values); err == nil || !strings.Contains(err.Error(), "DataQualityFilterSiteLV") {
		t.Fatal("enforced quality silently accepted malformed criteria", err)
	}
}

func TestLongVegetationQualityPreparationWeightsOriginalMembershipsAndStatistics(t *testing.T) {
	veg, _, _ := vegetationReportFixture(t)
	_, species, layers := vegetationLayerFixture(t)
	_, env, admin, lists := vegetationQualityFixture()
	veg.Rows, env.Rows, admin.Rows = nil, nil, nil
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for i, quality := range []string{"Good", "Fair", "Good", "NA"} {
		plot := "P" + strconv.Itoa(i+1)
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 11),
			Cells: []ProjectMetadataCell{metadataText(plot), metadataText("U")}})
		env.Rows = append(env.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 21),
			Cells: []ProjectMetadataCell{metadataText(plot)}})
		admin.Rows = append(admin.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 31),
			Cells: []ProjectMetadataCell{metadataText(plot), metadataText(quality), metadataText(quality), metadataText(quality)}})
		row := ProjectMetadataRow{RowID: strconv.Itoa(i + 41), Cells: []ProjectMetadataCell{metadataText(plot), metadataText("A")}}
		for range veg.Columns[2:] {
			row.Cells = append(row.Cells, ProjectMetadataCell{Storage: "null"})
		}
		if i != 2 {
			row.Cells[2] = metadataInteger([]string{"10", "30", "", "100"}[i])
		}
		veg.Rows = append(veg.Rows, row)
	}
	lists.Rows = append(lists.Rows, ProjectMetadataRow{RowID: "10", Cells: lists.Rows[2].Cells})
	prepared, selection, err := prepareQualityLongVegetation(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, lists, vegetationQualityCriteria("Poor", 7))
	if err != nil || len(selection.Occurrences) != 17 || len(prepared.Memberships) != 3 ||
		len(prepared.ReducedRows) != 3 || !reflect.DeepEqual(selection.ExcludedMembershipIDs, []string{"14"}) {
		t.Fatal("qualification repaired IDs, lost fanout or kept an excluded plot", prepared, selection, err)
	}
	if !reflect.DeepEqual(prepared.qualityMembershipCounts, map[string]int{"11": 8, "12": 1, "13": 8}) ||
		prepared.Memberships[0].RowID != "11" || prepared.Memberships[1].RowID != "12" || prepared.Memberships[2].RowID != "13" {
		t.Fatal("original physical SU identity or weights changed", prepared)
	}
	*selection.Occurrences[0].Membership.SiteUnit.Text = "Changed"
	*selection.Occurrences[0].ListRowIDs[0] = "Other"
	if *prepared.Memberships[0].SiteUnit.Text != "U" || *su.Rows[0].Cells[1].Text != "U" {
		t.Fatal("selection/preparation aliases original or adjacent metadata")
	}
	for _, average := range []string{"all-plots", "observations"} {
		options := layerTestOptions()
		options.ConstantSpeciesList, options.Average = false, average
		report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, options)
		if err != nil || len(report.Units) != 1 || report.Units[0].NumPlots != 17 ||
			!reflect.DeepEqual(report.Units[0].MembershipIDs, []string{"11", "12", "13"}) || len(report.Units[0].Rows) != 1 {
			t.Fatal("quality weights fabricated physical memberships or lost weighted denominator", report, err)
		}
		row := report.Units[0].Rows[0]
		mean := 110.0 / 17
		if average == "observations" {
			mean = 110.0 / 9
		}
		if row.Presence == nil || math.Abs(*row.Presence-2.0/17) > 1e-12 ||
			row.MeanCover == nil || math.Abs(*row.MeanCover-mean) > 1e-12 ||
			len(row.Plots) != 2 || *row.Plots[0].Cover != 80 || *row.Plots[1].Cover != 30 {
			t.Fatal("pivot-column presence, weighted sums or characteristic average changed", row)
		}
		qualityCount := 0
		for _, diagnostic := range report.Diagnostics {
			if diagnostic.Code == "physical_membership_multiplicity" {
				t.Fatal("reference fanout mislabeled as extra physical SU rows", diagnostic)
			}
			if diagnostic.Code == "quality_reference_multiplicity" {
				qualityCount++
				if diagnostic.Count != 8 || diagnostic.Identity != "11" && diagnostic.Identity != "13" {
					t.Fatal("quality provenance/count differs", diagnostic)
				}
			}
		}
		if qualityCount != 2 {
			t.Fatal("missing explicit quality multiplicity", report.Diagnostics)
		}
	}
	prepared.qualityMembershipCounts["11"] = 0
	if report, err := planLongVegetationLayers(context.Background(), prepared, species, layers, layerTestOptions()); err == nil ||
		!reflect.DeepEqual(report, vegetationLayerReport{}) {
		t.Fatal("invalid weighted preparation accepted or returned partial report", report, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, selection, err := prepareQualityLongVegetation(cancelled, "Project", "Selected", veg, su, layers, env, admin, lists,
		vegetationQualityCriteria("Poor", 7)); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, VegetationReportPreparation{}) || !reflect.DeepEqual(selection, vegetationQualitySelection{}) {
		t.Fatal("cancelled preparation retained partial selection", result, selection, err)
	}
}
