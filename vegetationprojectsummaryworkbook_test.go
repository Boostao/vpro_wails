package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func projectSummaryWorkbookFixture(t *testing.T, kind string, qualified bool) (vegetationLayerReport, LongVegetationReportScope, LongVegetationSettings, vegetationProjectWorkbookSummary) {
	t.Helper()
	source, scope, settings := projectWorkbookKernelFixture(t, kind, qualified)
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
	if qualified {
		env.Rows[1] = ProjectMetadataRow{RowID: "13", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Gamma")}}
		admin.Rows[1] = ProjectMetadataRow{RowID: "23", Cells: []ProjectMetadataCell{metadataText("P2")}}
	}
	summary, err := prepareVegetationProjectWorkbookSummary(context.Background(), "2026-10-10", env, admin,
		projectSummaryDescription(metadataText("literal version")), scope, source)
	if err != nil {
		t.Fatal(err)
	}
	return source, scope, settings, summary
}

func readProjectWorkbookHidden(t *testing.T, book *excelize.File) []byte {
	t.Helper()
	var encoded strings.Builder
	for row := 1; ; row++ {
		value, err := book.GetCellValue("_VPRO_Source", "A"+strconv.Itoa(row))
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		encoded.WriteString(value)
	}
	raw, err := hex.DecodeString(encoded.String())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestProjectSummaryWorkbookReopenedCountsLabelsAndDistinctFullProvenance(t *testing.T) {
	for _, kind := range []string{vegetationProjectFieldsScope, vegetationProjectAllScope} {
		for _, qualified := range []bool{false, true} {
			if kind == vegetationProjectAllScope && qualified {
				continue
			}
			source, scope, settings, summary := projectSummaryWorkbookFixture(t, kind, qualified)
			layout := vegetationWorkbookLayout{ReportSummary: true}
			got, err := prepareLongVegetationProjectWorkbookWithSummary(context.Background(), source, scope, settings, layout, &summary)
			if err != nil {
				t.Fatal(err)
			}
			book, err := excelize.OpenReader(bytes.NewReader(got.Workbook.Bytes))
			if err != nil {
				t.Fatal(err)
			}
			count, label, completeTuples := "4", "Alpha", 4
			if kind == vegetationProjectAllScope {
				count, label, completeTuples = "2", "All plots", 2
			}
			if qualified {
				count, completeTuples = "1", 2
			}
			for address, expected := range map[string]string{
				"A1": "Project units", "A2": "Date created: 2026-10-10", "A4": "Vegetation table:", "B4": "Sample_Env",
				"A5": "Site unit table:", "B5": "USysSuTableDynamic_SU", "B6": "None_Lump", "B9": "-1", "B10": "-100",
				"A11": "Number of plots in database", "B11": "2", "A12": "Number of plots in site unit table", "B12": count,
				"B13": "literal version", "D3": "Original project report tuples (desktop adaptation)",
				"D4": "Plot Number", "E4": "Site Unit", "D5": "P1", "E5": label,
			} {
				actual, err := book.GetCellValue("ReportSummary", address, excelize.Options{RawCellValue: true})
				if err != nil || actual != expected {
					t.Fatal("project summary changed independent expected cell", address, actual, expected, err)
				}
			}
			list := book.GetSheetList()
			if list[len(list)-2] != "ReportSummary" || list[len(list)-1] != "_VPRO_Source" {
				t.Fatal("project summary order changed", list)
			}
			var hidden struct {
				Preview LongVegetationPreview
				Layout  vegetationWorkbookLayout
				Project vegetationProjectWorkbookEvidence
			}
			decoder := json.NewDecoder(bytes.NewReader(readProjectWorkbookHidden(t, book)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&hidden); err != nil {
				t.Fatal(err)
			}
			if hidden.Layout.Summary != nil || hidden.Project.Summary == nil || len(hidden.Project.Scope.Build.ProjectTuples) != completeTuples ||
				hidden.Project.Scope.SUPath != scope.SUPath || hidden.Project.Summary.SourcePlotRows != summary.SourcePlotRows ||
				!reflect.DeepEqual(hidden.Project.Summary.ProjectTuples, summary.ProjectTuples) ||
				hidden.Project.Summary.Header.VersionDefinitions.Rows[0].Cells[1].Storage != "text" {
				t.Fatal("project hidden provenance invented physical memberships or lost original tuples/metadata", hidden)
			}
			if qualified && (len(hidden.Preview.Report.Quality.Occurrences) != 8 || len(hidden.Preview.Report.ProjectUnits) != 1) {
				t.Fatal("quality provenance weights were conflated")
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			again, err := prepareLongVegetationProjectWorkbookWithSummary(context.Background(), source, scope, settings, layout, &summary)
			if err != nil || !bytes.Equal(again.Workbook.Bytes, got.Workbook.Bytes) {
				t.Fatal("project summary workbook bytes are not deterministic", err)
			}
		}
	}
}

func TestProjectSummaryWorkbookMissingMixedOrChangedEvidenceRefusesZeroBytes(t *testing.T) {
	for _, mutate := range []func(*vegetationProjectWorkbookSummary, *vegetationWorkbookLayout){
		func(_ *vegetationProjectWorkbookSummary, l *vegetationWorkbookLayout) { l.ReportSummary = false },
		func(_ *vegetationProjectWorkbookSummary, l *vegetationWorkbookLayout) {
			l.Summary = &vegetationWorkbookSummary{}
		},
		func(s *vegetationProjectWorkbookSummary, _ *vegetationWorkbookLayout) {
			s.SourceKind = vegetationProjectAllScope
		},
		func(s *vegetationProjectWorkbookSummary, _ *vegetationWorkbookLayout) {
			s.ProjectTuples[0].EnvRowID = "different"
		},
		func(s *vegetationProjectWorkbookSummary, _ *vegetationWorkbookLayout) { s.SourcePlotRows = 3 },
		func(s *vegetationProjectWorkbookSummary, _ *vegetationWorkbookLayout) { s.Header.EnvironmentRows = -1 },
		func(s *vegetationProjectWorkbookSummary, _ *vegetationWorkbookLayout) {
			s.Header.CreatedDate = "not a date"
		},
	} {
		source, scope, settings, summary := projectSummaryWorkbookFixture(t, vegetationProjectFieldsScope, false)
		layout := vegetationWorkbookLayout{ReportSummary: true}
		mutate(&summary, &layout)
		got, err := prepareLongVegetationProjectWorkbookWithSummary(context.Background(), source, scope, settings, layout, &summary)
		if err == nil || !reflect.DeepEqual(got, vegetationProjectWorkbook{}) {
			t.Fatal("invalid/mixed project summary produced success-shaped output", err)
		}
	}
	source, scope, settings, _ := projectSummaryWorkbookFixture(t, vegetationProjectFieldsScope, false)
	got, err := prepareLongVegetationProjectWorkbook(context.Background(), source, scope, settings, vegetationWorkbookLayout{ReportSummary: true})
	if err == nil || len(got.Workbook.Bytes) != 0 {
		t.Fatal("requested summary was silently omitted", err)
	}
}

func TestOrdinaryWorkbookHiddenMetadataDoesNotGainProjectEvidence(t *testing.T) {
	preview := vegetationWorkbookFixture(t)
	got, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(got.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(readProjectWorkbookHidden(t, book), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["Preview"] == nil || fields["Layout"] == nil {
		t.Fatal("ordinary workbook hidden wire changed", fields)
	}
	if err := book.Close(); err != nil {
		t.Fatal(err)
	}
}
