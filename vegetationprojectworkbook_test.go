package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func projectWorkbookKernelFixture(t *testing.T, kind string, qualified bool) (vegetationLayerReport, LongVegetationReportScope, LongVegetationSettings) {
	t.Helper()
	ctx := context.Background()
	veg := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}},
		Rows: []ProjectMetadataRow{{RowID: "51", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("S")}}}}
	for _, cover := range longVegetationCoverColumns() {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: cover})
		cell := ProjectMetadataCell{Storage: "null"}
		if cover == "Cover1" {
			cell = metadataInteger("2")
		}
		veg.Rows[0].Cells = append(veg.Rows[0].Cells, cell)
	}
	env := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Beta")}},
		}}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "22", Cells: []ProjectMetadataCell{metadataText("P1")}},
		}}
	layers := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}, {Name: "LayerText"}},
		Rows: []ProjectMetadataRow{{RowID: "31", Cells: []ProjectMetadataCell{metadataText("1"), metadataText("Tree"), metadataText("1")}}}}
	master := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}},
		Rows: []ProjectMetadataRow{{RowID: "41", Cells: []ProjectMetadataCell{metadataText("S"), metadataText("SP"), {Storage: "null"}, {Storage: "null"}}}}}
	options := longVegetationOptions{Title: "Project units", Average: "all-plots", Order: "species",
		PresenceGreaterThan: -100, MeanCoverGreaterThan: -100}
	scope := LongVegetationReportScope{ContextID: "owned", Project: "Sample", ProjectPath: `C:\project.db`,
		SU: "None", SourceKind: kind, Revision: "3", Fields: []string{"Zone"}, BuildCurrent: true,
		Build: &LongVegetationProjectBuild{ApprovalHash: strings.Repeat("a", 64)}}
	su := ProjectMetadataTable{}
	var quality *vegetationQualitySelection
	if qualified {
		scope.SU, scope.SUPath = "Selected", `C:\external.db`
		env.Rows[1] = ProjectMetadataRow{RowID: "13", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Gamma")}}
		admin.Rows[1] = ProjectMetadataRow{RowID: "23", Cells: []ProjectMetadataCell{metadataText("P2")}}
		su = ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
			Rows: []ProjectMetadataRow{
				{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Original")}},
				{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Original")}},
			}}
		for _, name := range []string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"} {
			admin.Columns = append(admin.Columns, ProjectMetadataColumn{Name: name})
			admin.Rows[0].Cells = append(admin.Rows[0].Cells, metadataText("Good"))
			admin.Rows[1].Cells = append(admin.Rows[1].Cells, metadataText("Poor"))
		}
		lists := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Item"}, {Name: "ListName"}, {Name: "ItemOrder"}},
			Rows: []ProjectMetadataRow{
				{RowID: "90", Cells: []ProjectMetadataCell{metadataText("Poor"), metadataText("DataQuality"), metadataInteger("1")}},
				{RowID: "100", Cells: []ProjectMetadataCell{metadataText("Good"), metadataText("DataQuality"), metadataInteger("3")}},
				{RowID: "101", Cells: []ProjectMetadataCell{metadataText("Good"), metadataText("DataQuality"), metadataInteger("3")}},
			}}
		criteria := [3]vegetationQualityCriterion{{Minimum: "Good"}, {Minimum: "Good"}, {Minimum: "Good"}}
		planned, err := planLongVegetationQuality(ctx, su, env, admin, lists, criteria)
		if err != nil {
			t.Fatal(err)
		}
		quality, options.Quality = &planned, &criteria
	}
	var project vegetationProjectPreparation
	var err error
	if kind == vegetationProjectAllScope {
		project, err = prepareLongVegetationProjectNoneUnits(ctx, "Sample", veg, layers, env)
	} else {
		project, err = prepareLongVegetationProjectSelectedUnits(ctx, "Sample", scope.SU, veg, layers, env, admin, su, []string{"Zone"}, quality)
	}
	if err != nil {
		t.Fatal(err)
	}
	// The Build captures all original tuples, before the original-SU quality subset.
	admin21, admin22, admin23 := "21", "22", "23"
	if kind == vegetationProjectAllScope {
		scope.Build.ProjectTuples = []LongVegetationProjectUnit{
			{"11", nil, metadataText("P1"), metadataText("All plots")},
			{"12", nil, metadataText("P1"), metadataText("All plots")},
		}
		scope.Fields = []string{"RetainedInactive"}
	} else if qualified {
		scope.Build.ProjectTuples = []LongVegetationProjectUnit{
			{"11", &admin21, metadataText("P1"), metadataText("Alpha")},
			{"13", &admin23, metadataText("P2"), metadataText("Gamma")},
		}
	} else {
		scope.Build.ProjectTuples = []LongVegetationProjectUnit{
			{"11", &admin21, metadataText("P1"), metadataText("Alpha")},
			{"11", &admin22, metadataText("P1"), metadataText("Alpha")},
			{"12", &admin21, metadataText("P1"), metadataText("Beta")},
			{"12", &admin22, metadataText("P1"), metadataText("Beta")},
		}
	}
	prepared, err := projectReportPreparation(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := planLongVegetationLayers(ctx, prepared, master, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	if quality != nil {
		report.Quality = quality
		if err := addLongVegetationQualityMultiplicity(ctx, &report, *quality); err != nil {
			t.Fatal(err)
		}
	}
	if err := addLongVegetationUnitNames(ctx, &report, ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []ProjectMetadataRow{}}); err != nil {
		t.Fatal(err)
	}
	settings := vegetationSettings(options)
	if kind == vegetationProjectAllScope {
		settings.UnitGrouping = "all-plots"
	} else {
		settings.UnitGrouping, settings.UnitFields = "selected-fields", []string{"Zone"}
	}
	return report, scope, settings
}

func TestProjectWorkbookOriginalKernelReopenedCellsDenominatorsAndNoFabricatedSU(t *testing.T) {
	for _, kind := range []string{vegetationProjectFieldsScope, vegetationProjectAllScope} {
		for _, qualified := range []bool{false, true} {
			if qualified && kind == vegetationProjectAllScope {
				continue
			}
			source, scope, settings := projectWorkbookKernelFixture(t, kind, qualified)
			got, err := prepareLongVegetationProjectWorkbook(context.Background(), source, scope, settings, vegetationWorkbookLayout{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Preview.Scope.SourceKind != kind || got.Preview.Preview.SUPath != scope.SUPath ||
				len(got.Preview.Preview.Report.SelectedUnits) != 0 || len(got.Preview.Preview.Report.NoneUnits) != 0 {
				t.Fatal("project workbook invented selected-list ownership", got.Preview)
			}
			wantSheets, wantPlots, presence, mean, cover := 2, 2, "0.5", "2", "4"
			if kind == vegetationProjectAllScope {
				wantSheets = 1
			}
			if qualified {
				wantSheets, wantPlots, presence, mean, cover = 1, 1, "1", "2", "2"
				if len(got.Preview.Preview.Report.Quality.Occurrences) != 8 || len(got.Preview.Scope.Build.ProjectTuples) != 2 ||
					len(got.Preview.Preview.Report.ProjectUnits) != 1 {
					t.Fatal("original quality multiplicity changed project denominator or lost complete Build")
				}
			}
			if len(got.Workbook.Sheets) != wantSheets {
				t.Fatal("original unit sheet count", len(got.Workbook.Sheets), wantSheets)
			}
			book, err := excelize.OpenReader(bytes.NewReader(got.Workbook.Bytes))
			if err != nil {
				t.Fatal(err)
			}
			for i, sheet := range got.Workbook.Sheets {
				if got.Preview.Preview.Report.Units[i].NumPlots != wantPlots {
					t.Fatal("original denominator changed")
				}
				for address, expected := range map[string]string{
					"A1": "Project units", "A4": "Layer", "B4": "Spp", "B5": "SP", "C5": presence, "D5": mean, "E4": "P1", "E5": cover,
				} {
					actual, err := book.GetCellValue(sheet.Name, address, excelize.Options{RawCellValue: true})
					if err != nil || actual != expected {
						t.Fatal("project XLSX changed independently expected value", address, actual, expected, err)
					}
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			again, err := prepareLongVegetationProjectWorkbook(context.Background(), source, scope, settings, vegetationWorkbookLayout{})
			if err != nil || !bytes.Equal(again.Workbook.Bytes, got.Workbook.Bytes) {
				t.Fatal("project rendering is not deterministic", err)
			}
			scope.Build.ProjectTuples[0].SiteUnit = metadataText("changed")
			if *got.Preview.Scope.Build.ProjectTuples[0].SiteUnit.Text == "changed" {
				t.Fatal("project receipt aliases source Build")
			}
			if legacy, err := prepareLongVegetationWorkbook(context.Background(), got.Preview.Preview, vegetationWorkbookLayout{}); err == nil || len(legacy.Bytes) != 0 {
				t.Fatal("legacy workbook consumed project provenance", err)
			}
			if settings.UnitFields != nil {
				settings.UnitFields[0] = "changed"
				if got.Preview.Preview.Settings.UnitFields[0] == "changed" {
					t.Fatal("project receipt aliases source field settings")
				}
			}
			if settings.Quality != nil {
				settings.Quality.Site.Minimum = "changed"
				if got.Preview.Preview.Settings.Quality.Site.Minimum == "changed" {
					t.Fatal("project receipt aliases source quality settings")
				}
			}
		}
	}
}

func TestProjectWorkbookOwnerScopeWitnessAndRequestedSummaryRefuseZeroOutput(t *testing.T) {
	for _, mutate := range []func(*vegetationLayerReport, *LongVegetationReportScope, *LongVegetationSettings, *vegetationWorkbookLayout){
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.SUPath = "invented"
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.BuildCurrent = false
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.Project = "foreign"
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.Revision = "03"
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.Build.ApprovalHash = strings.Repeat("A", 64)
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.Build.ProjectTuples = s.Build.ProjectTuples[:3]
		},
		func(_ *vegetationLayerReport, s *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.Build.ProjectTuples[0], s.Build.ProjectTuples[1] = s.Build.ProjectTuples[1], s.Build.ProjectTuples[0]
		},
		func(_ *vegetationLayerReport, _ *LongVegetationReportScope, s *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			s.UnitFields = []string{"Different"}
		},
		func(_ *vegetationLayerReport, _ *LongVegetationReportScope, _ *LongVegetationSettings, l *vegetationWorkbookLayout) {
			l.ReportSummary = true
		},
		func(r *vegetationLayerReport, _ *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			r.Units[0].ProjectUnitIndexes[0] = 3
		},
		func(r *vegetationLayerReport, _ *LongVegetationReportScope, _ *LongVegetationSettings, _ *vegetationWorkbookLayout) {
			r.Units[0].MembershipIDs = []string{"fabricated"}
		},
	} {
		source, scope, settings := projectWorkbookKernelFixture(t, vegetationProjectFieldsScope, false)
		layout := vegetationWorkbookLayout{}
		mutate(&source, &scope, &settings, &layout)
		got, err := prepareLongVegetationProjectWorkbook(context.Background(), source, scope, settings, layout)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbook{}) {
			t.Fatal("invalid project workbook produced success-shaped output", err)
		}
	}
	source, scope, settings := projectWorkbookKernelFixture(t, vegetationProjectFieldsScope, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		got, err := prepareLongVegetationProjectWorkbook(ctx, source, scope, settings, vegetationWorkbookLayout{})
		if err == nil || len(got.Workbook.Bytes) != 0 || ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("project workbook cancellation produced output", err)
		}
	}
}
