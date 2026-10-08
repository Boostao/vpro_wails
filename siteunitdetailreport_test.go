package main

import (
	"context"
	"reflect"
	"testing"
)

func summaryReportFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	env, admin, su := siteUnitDetailScopeFixture()
	for source, table := range map[string]*ProjectMetadataTable{"Env": &env, "Admin": &admin} {
		existing := map[string]bool{}
		for _, column := range table.Columns {
			existing[column.Name] = true
		}
		for _, field := range siteUnitSummaryFields() {
			if field.Source == source && !existing[field.Key] {
				existing[field.Key] = true
				table.Columns = append(table.Columns, ProjectMetadataColumn{Name: field.Key})
				for i := range table.Rows {
					table.Rows[i].Cells = append(table.Rows[i].Cells, ProjectMetadataCell{Storage: "null"})
				}
			}
		}
	}
	for i := range env.Rows {
		for j, column := range env.Columns {
			switch column.Name {
			case "Zone":
				env.Rows[i].Cells[j] = metadataText("CWH")
			case "SubZone":
				env.Rows[i].Cells[j] = metadataText("vm")
			case "SiteDisturbance2":
				env.Rows[i].Cells[j] = metadataText("Fire")
			case "Elevation":
				value := "100"
				if i == 1 {
					value = "300"
				}
				env.Rows[i].Cells[j] = ProjectMetadataCell{Storage: "integer", Integer: &value}
			}
		}
	}
	master := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}},
		Rows: []ProjectMetadataRow{
			{"1", []ProjectMetadataCell{metadataText("U"), metadataText("")}},
			{"2", []ProjectMetadataCell{metadataText("U"), {Storage: "null"}}},
		}}
	return env, admin, su, master
}

func TestSiteUnitSummaryCompleteRegistryPhysicalWeightsAndNames(t *testing.T) {
	env, admin, su, master := summaryReportFixture()
	report, err := planSiteUnitSummary(context.Background(), "Sample", "Selected", 1, 9, env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Fields) != 39 || len(report.Units) != 2 || len(report.Memberships) != 7 ||
		report.Units[0].Code != "U" || len(report.Units[0].Plots) != 8 || report.Units[1].Code != "" ||
		report.Units[0].Values[0] != "CWH(8)  " || report.Units[0].Values[1] != "CWHvm(8)  " ||
		report.Units[0].Values[2] != "100---200---300" ||
		report.Units[0].Values[8] != "Fire(8)  " || report.Units[0].Values[9] != "Fire(8)  " ||
		report.Units[0].NameStatus != "unique" || report.Units[0].LongName == nil || *report.Units[0].LongName != "" ||
		len(report.Units[0].NameCandidates) != 2 {
		t.Fatal("source order, duplicated disturbance, weighted values or typed names differ", report)
	}
	for i, expected := range []struct {
		section string
		count   int
	}{{"SITE", 21}, {"VEGETATION", 6}, {"SOILS", 12}} {
		count := 0
		for _, field := range report.Fields {
			if field.Section == expected.section {
				count++
			}
		}
		if count != expected.count {
			t.Fatal("section grouping differs", i, count)
		}
	}
	report.Units[0].NameCandidates[0].Value.Text = nil
	report.Units[0].Plots[0].PlotNumber = "changed"
	again, err := planSiteUnitSummary(context.Background(), "Sample", "Selected", 2, 9, env, admin, su, master)
	if err != nil || again.Units[0].Values[2] != "100---200---300" ||
		again.Units[0].NameCandidates[0].Value.Text == nil || again.Units[0].Plots[0].PlotNumber != "P1" {
		t.Fatal("quartile/ownership differs", again, err)
	}
}

func TestSiteUnitSummaryCategoryNullEmptyBGCAndAspectBoundaries(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	cases := []struct {
		cells, zones []ProjectMetadataCell
		kind, want   string
	}{
		{[]ProjectMetadataCell{metadataText(""), null, metadataText("a"), metadataText("a")}, nil, "category", "(1)  a(2)  (Null 1)"},
		{[]ProjectMetadataCell{metadataText("0"), metadataText("4"), metadataText("9"), metadataText(""), null}, nil, "moisture", "(1)  VX(1)  M(1)  (1)  (Null 1)"},
		{[]ProjectMetadataCell{null, null, metadataText("X"), metadataText("X")}, []ProjectMetadataCell{metadataText("X"), null, null, metadataText("")}, "bgc-unit", "X(3)  (Null 1)"},
	}
	for _, test := range cases {
		got, err := summarizeSiteUnitCategories(test.cells, test.zones, test.kind)
		if err != nil || got != test.want {
			t.Fatal(test.kind, got, err)
		}
	}
	for _, test := range []struct {
		degrees float32
		want    string
	}{
		{0, "N"}, {11.25, "NNE"}, {33.75, "NNE"}, {33.750004, "NE"}, {348.75, "NNW"}, {360, "N"}, {999, "Level"}, {-1, "???"}, {361, "???"},
	} {
		if got := siteUnitSummaryAspect(test.degrees); got != test.want {
			t.Fatal(test, got)
		}
	}
	if _, err := summarizeSiteUnitCategories([]ProjectMetadataCell{metadataText("0")}, nil, "aspect"); err == nil {
		t.Fatal("aspect text coerced")
	}
}

func TestSiteUnitSummaryFailureReturnsNoPartialReport(t *testing.T) {
	env, admin, su, master := summaryReportFixture()
	for _, test := range []struct{ method, budget int }{{0, 9}, {3, 9}, {1, 8}} {
		result, err := planSiteUnitSummary(context.Background(), "Sample", "Selected", test.method, test.budget, env, admin, su, master)
		if err == nil || !reflect.DeepEqual(result, SiteUnitSummaryReport{}) {
			t.Fatal("unsupported/truncated report returned", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := planSiteUnitSummary(ctx, "Sample", "Selected", 1, 9, env, admin, su, master); err == nil ||
		!reflect.DeepEqual(result, SiteUnitSummaryReport{}) {
		t.Fatal("cancelled report returned", result, err)
	}
}
