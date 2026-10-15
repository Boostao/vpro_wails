package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectSummaryFixture(t *testing.T, none bool) (vegetationLayerReport, LongVegetationReportScope, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	source := projectTransportFixture(t, none)
	_, _, _, env, admin := projectEntryFixture(t)
	kind := vegetationProjectFieldsScope
	if none {
		kind = vegetationProjectAllScope
	}
	report, err := longVegetationProjectTransport(context.Background(), source, kind)
	if err != nil {
		t.Fatal(err)
	}
	scope := LongVegetationReportScope{ContextID: "owned", Project: "Project", ProjectPath: `C:\project.db`, SU: "None",
		SourceKind: kind, Revision: "3", Fields: []string{"Unit"}, BuildCurrent: true,
		Build: &LongVegetationProjectBuild{ApprovalHash: strings.Repeat("a", 64), ProjectTuples: report.ProjectUnits}}
	return source, scope, env, admin
}

func projectSummaryDescription(value ProjectMetadataCell) ProjectMetadataTable {
	return ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "table_name", DeclaredType: "TEXT"}, {Name: "description", DeclaredType: "TEXT"}},
		Rows:    []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("USysAllSpecs"), value}}},
	}
}

func TestProjectSummaryOriginalEnvCountsCartesianTuplesNullEmptyAndDescription(t *testing.T) {
	for _, none := range []bool{false, true} {
		source, scope, env, admin := projectSummaryFixture(t, none)
		descriptions := projectSummaryDescription(metadataText(""))
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin, descriptions, scope, source)
		if err != nil {
			t.Fatal(err)
		}
		wantRows, wantCount := 7, 7
		if none {
			wantRows, wantCount = 6, 5
		}
		if got.Header.EnvironmentRows != 6 || len(got.ProjectTuples) != wantRows || got.SourcePlotRows != wantCount ||
			got.Header.CreatedDate != "2026-10-10" || got.Header.VersionStatus != "literal" ||
			got.Header.SpeciesVersion.Storage != "text" || *got.Header.SpeciesVersion.Text != "" {
			t.Fatal("original Env/derived tuple counts or literal empty Description changed", got)
		}
		if none {
			if got.ProjectTuples[5].PlotNumber.Storage != "null" || got.ProjectTuples[4].PlotNumber.Storage != "text" ||
				*got.ProjectTuples[4].PlotNumber.Text != "" {
				t.Fatal("NULL/empty original Env rows were conflated", got.ProjectTuples)
			}
			for _, row := range got.ProjectTuples {
				if row.AdminRowID != nil || *row.SiteUnit.Text != "All plots" {
					t.Fatal("source3 invented Admin/SU identity")
				}
			}
		} else if got.ProjectTuples[0].EnvRowID != "101" || *got.ProjectTuples[0].AdminRowID != "201" ||
			got.ProjectTuples[1].EnvRowID != "101" || *got.ProjectTuples[1].AdminRowID != "202" {
			t.Fatal("source2 original Cartesian tuple order changed", got.ProjectTuples)
		}
		descriptions.Rows[0].Cells[1] = metadataText("changed")
		descriptions.Columns[1].Name = "changed"
		scope.Build.ProjectTuples[0].SiteUnit = metadataText("changed")
		if *got.Header.VersionDefinitions.Rows[0].Cells[1].Text != "" || got.Header.VersionDefinitions.Columns[1].Name != "description" ||
			*got.ProjectTuples[0].SiteUnit.Text == "changed" {
			t.Fatal("summary evidence aliases original metadata or Build")
		}
	}
}

func TestProjectSummaryVersionStatesRemainDistinctAndCorrectionRetry(t *testing.T) {
	source, scope, env, admin := projectSummaryFixture(t, false)
	for _, item := range []struct {
		table  ProjectMetadataTable
		status string
		cell   ProjectMetadataCell
	}{
		{ProjectMetadataTable{}, "metadata-unavailable", metadataText("Unknown")},
		{ProjectMetadataTable{Columns: projectSummaryDescription(metadataText("")).Columns, Rows: []ProjectMetadataRow{}}, "missing-definition", metadataText("Unknown")},
		{projectSummaryDescription(ProjectMetadataCell{Storage: "null"}), "null", ProjectMetadataCell{Storage: "null"}},
		{projectSummaryDescription(metadataText("")), "literal", metadataText("")},
		{projectSummaryDescription(metadataText("a version")), "literal", metadataText("a version")},
	} {
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin, item.table, scope, source)
		if err != nil || got.Header.VersionStatus != item.status || !reflect.DeepEqual(got.Header.SpeciesVersion, item.cell) {
			t.Fatal("Description provenance states conflated", got.Header, err)
		}
	}
	for _, table := range []ProjectMetadataTable{
		projectSummaryDescription(metadataInteger("1")),
		projectSummaryDescription(metadataText(string([]byte{0xff}))),
		{Columns: projectSummaryDescription(metadataText("")).Columns, Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("USysAllSpecs"), metadataText("one")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("USysAllSpecs"), metadataText("two")}},
		}},
	} {
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin, table, scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) {
			t.Fatal("invalid native Description produced summary evidence", err)
		}
	}
	if _, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
		projectSummaryDescription(metadataText("corrected")), scope, source); err != nil {
		t.Fatal("Description correction cannot retry", err)
	}
}

func TestProjectSummaryOriginalSUQualityExistenceDoesNotWeightDerivedCounts(t *testing.T) {
	source, scope, _ := projectWorkbookKernelFixture(t, vegetationProjectFieldsScope, true)
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "13", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Gamma")}},
		}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "23", Cells: []ProjectMetadataCell{metadataText("P2")}},
		}}
	got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
		projectSummaryDescription(metadataText("version")), scope, source)
	if err != nil || got.Header.EnvironmentRows != 2 || got.SourcePlotRows != 1 || len(got.ProjectTuples) != 1 ||
		got.ProjectTuples[0].EnvRowID != "11" || len(source.Quality.Occurrences) != 8 || len(scope.Build.ProjectTuples) != 2 {
		t.Fatal("quality multiplicity weighted original derived summary or lost full Build", got, err)
	}
}

func TestProjectSummaryChangedPhysicalWitnessScopeFieldAndCancellationRefuseZeroEvidence(t *testing.T) {
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *LongVegetationReportScope){
		func(e *ProjectMetadataTable, _ *ProjectMetadataTable, _ *LongVegetationReportScope) {
			e.Rows[0].Cells[1] = metadataText("changed")
		},
		func(_ *ProjectMetadataTable, a *ProjectMetadataTable, _ *LongVegetationReportScope) {
			a.Rows[0].RowID = "different"
		},
		func(_ *ProjectMetadataTable, _ *ProjectMetadataTable, s *LongVegetationReportScope) {
			s.BuildCurrent = false
		},
		func(_ *ProjectMetadataTable, _ *ProjectMetadataTable, s *LongVegetationReportScope) {
			s.SourceKind = vegetationSelectedListScope
		},
		func(_ *ProjectMetadataTable, _ *ProjectMetadataTable, s *LongVegetationReportScope) {
			s.Fields = []string{"Absent"}
		},
	} {
		source, scope, env, admin := projectSummaryFixture(t, false)
		mutate(&env, &admin, &scope)
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
			projectSummaryDescription(metadataText("version")), scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) {
			t.Fatal("changed source produced success-shaped summary", err)
		}
	}
	source, scope, env, admin := projectSummaryFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, request := range []context.Context{nil, ctx} {
		got, err := prepareVegetationProjectWorkbookSummary(request, "2026-10-10", env, admin, ProjectMetadataTable{}, scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) || request != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled summary produced evidence", err)
		}
	}
}
