package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectSummaryFixture(t *testing.T, none bool) (vegetationLayerReport, LongVegetationReportScope, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	t.Helper()
	source := projectTransportFixture(t, none)
	_, _, _, env, admin := projectEntryFixture(t)
	kind := vprocore.VegetationProjectFieldsScope
	if none {
		kind = vprocore.VegetationProjectAllScope
	}
	report, err := longVegetationProjectTransport(context.Background(), source, kind)
	if err != nil {
		t.Fatal(err)
	}
	scope := LongVegetationReportScope{ContextID: "owned", Project: "Project", ProjectPath: `C:\project.db`, SU: "None",
		SourceKind: kind, Revision: "3", Fields: []string{"Unit"}, BuildCurrent: true,
		Build: &vprocore.LongVegetationProjectBuild{ApprovalHash: strings.Repeat("a", 64), ProjectTuples: report.ProjectUnits}}
	return source, scope, env, admin
}

func projectSummaryDescription(value vprocore.ProjectMetadataCell) vprocore.ProjectMetadataTable {
	return vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "table_name", DeclaredType: "TEXT"}, {Name: "description", DeclaredType: "TEXT"}},
		Rows:    []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), value}}},
	}
}

func TestProjectSummaryOriginalEnvCountsCartesianTuplesNullEmptyAndDescription(t *testing.T) {
	for _, none := range []bool{false, true} {
		source, scope, env, admin := projectSummaryFixture(t, none)
		descriptions := projectSummaryDescription(vprotest.MetadataText(""))
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
		descriptions.Rows[0].Cells[1] = vprotest.MetadataText("changed")
		descriptions.Columns[1].Name = "changed"
		scope.Build.ProjectTuples[0].SiteUnit = vprotest.MetadataText("changed")
		if *got.Header.VersionDefinitions.Rows[0].Cells[1].Text != "" || got.Header.VersionDefinitions.Columns[1].Name != "description" ||
			*got.ProjectTuples[0].SiteUnit.Text == "changed" {
			t.Fatal("summary evidence aliases original metadata or Build")
		}
	}
}

func TestProjectSummaryVersionStatesRemainDistinctAndCorrectionRetry(t *testing.T) {
	source, scope, env, admin := projectSummaryFixture(t, false)
	for _, item := range []struct {
		table  vprocore.ProjectMetadataTable
		status string
		cell   vprocore.ProjectMetadataCell
	}{
		{vprocore.ProjectMetadataTable{}, "metadata-unavailable", vprotest.MetadataText("Unknown")},
		{vprocore.ProjectMetadataTable{Columns: projectSummaryDescription(vprotest.MetadataText("")).Columns, Rows: []vprocore.ProjectMetadataRow{}}, "missing-definition", vprotest.MetadataText("Unknown")},
		{projectSummaryDescription(vprocore.ProjectMetadataCell{Storage: "null"}), "null", vprocore.ProjectMetadataCell{Storage: "null"}},
		{projectSummaryDescription(vprotest.MetadataText("")), "literal", vprotest.MetadataText("")},
		{projectSummaryDescription(vprotest.MetadataText("a version")), "literal", vprotest.MetadataText("a version")},
	} {
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin, item.table, scope, source)
		if err != nil || got.Header.VersionStatus != item.status || !reflect.DeepEqual(got.Header.SpeciesVersion, item.cell) {
			t.Fatal("Description provenance states conflated", got.Header, err)
		}
	}
	for _, table := range []vprocore.ProjectMetadataTable{
		projectSummaryDescription(vprotest.MetadataInteger("1")),
		projectSummaryDescription(vprotest.MetadataText(string([]byte{0xff}))),
		{Columns: projectSummaryDescription(vprotest.MetadataText("")).Columns, Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), vprotest.MetadataText("one")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), vprotest.MetadataText("two")}},
		}},
	} {
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin, table, scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) {
			t.Fatal("invalid native Description produced summary evidence", err)
		}
	}
	if _, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
		projectSummaryDescription(vprotest.MetadataText("corrected")), scope, source); err != nil {
		t.Fatal("Description correction cannot retry", err)
	}
}

func TestProjectSummaryOriginalSUQualityExistenceDoesNotWeightDerivedCounts(t *testing.T) {
	source, scope, _ := projectWorkbookKernelFixture(t, vprocore.VegetationProjectFieldsScope, true)
	env := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Alpha")}},
			{RowID: "13", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2"), vprotest.MetadataText("Gamma")}},
		}}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "23", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2")}},
		}}
	got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
		projectSummaryDescription(vprotest.MetadataText("version")), scope, source)
	if err != nil || got.Header.EnvironmentRows != 2 || got.SourcePlotRows != 1 || len(got.ProjectTuples) != 1 ||
		got.ProjectTuples[0].EnvRowID != "11" || len(source.Quality.Occurrences) != 8 || len(scope.Build.ProjectTuples) != 2 {
		t.Fatal("quality multiplicity weighted original derived summary or lost full Build", got, err)
	}
}

func TestProjectSummaryChangedPhysicalWitnessScopeFieldAndCancellationRefuseZeroEvidence(t *testing.T) {
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable, *LongVegetationReportScope){
		func(e *vprocore.ProjectMetadataTable, _ *vprocore.ProjectMetadataTable, _ *LongVegetationReportScope) {
			e.Rows[0].Cells[1] = vprotest.MetadataText("changed")
		},
		func(_ *vprocore.ProjectMetadataTable, a *vprocore.ProjectMetadataTable, _ *LongVegetationReportScope) {
			a.Rows[0].RowID = "different"
		},
		func(_ *vprocore.ProjectMetadataTable, _ *vprocore.ProjectMetadataTable, s *LongVegetationReportScope) {
			s.BuildCurrent = false
		},
		func(_ *vprocore.ProjectMetadataTable, _ *vprocore.ProjectMetadataTable, s *LongVegetationReportScope) {
			s.SourceKind = vprocore.VegetationSelectedListScope
		},
		func(_ *vprocore.ProjectMetadataTable, _ *vprocore.ProjectMetadataTable, s *LongVegetationReportScope) {
			s.Fields = []string{"Absent"}
		},
	} {
		source, scope, env, admin := projectSummaryFixture(t, false)
		mutate(&env, &admin, &scope)
		got, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
			projectSummaryDescription(vprotest.MetadataText("version")), scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) {
			t.Fatal("changed source produced success-shaped summary", err)
		}
	}
	source, scope, env, admin := projectSummaryFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, request := range []context.Context{nil, ctx} {
		got, err := prepareVegetationProjectWorkbookSummary(request, "2026-10-10", env, admin, vprocore.ProjectMetadataTable{}, scope, source)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbookSummary{}) || request != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled summary produced evidence", err)
		}
	}
}
