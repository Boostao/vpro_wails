package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSelectedUnitSummaryReopenedWorkbookAndSourceDrift(t *testing.T) {
	veg, su, layers, master, env, admin := selectedUnitReportFixture(t)
	prepared, joins, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	options := layerTestOptions()
	options.SelectedUnitFields = []string{"Zone"}
	source, err := planLongVegetationLayers(context.Background(), prepared, master, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	source.SelectedUnits = joins
	report, err := longVegetationTransport(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	base, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-10", env, su, ProjectMetadataTable{})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := vegetationWorkbookSelectedUnitSummary(context.Background(), base, env, admin, su, []string{"Zone"}, report)
	if err != nil || summary.SelectedPlotRows != 8 || len(summary.Memberships) != 8 || base.SelectedPlotRows != 2 {
		t.Fatal("derived2SU*2Env*2Admin summary counts/original preservation", summary, err)
	}
	preview := LongVegetationPreview{ContextID: "owned", ProjectPath: t.TempDir(), SUPath: t.TempDir(),
		Settings: vegetationSettings(options), Report: report}
	workbook, err := prepareLongVegetationWorkbook(context.Background(), preview,
		vegetationWorkbookLayout{QuickReport: true, ReportSummary: true, Summary: &summary})
	if err != nil || len(workbook.Bytes) == 0 || len(workbook.Sheets) != 2 {
		t.Fatal("derived source2 workbook was not built", len(workbook.Bytes), err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(workbook.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if !reflect.DeepEqual(sheets, []string{"Alpha", "Beta", "ReportSummary", "_VPRO_Source"}) {
		t.Fatal("expected two derived sheets, summary and existing provenance", sheets)
	}
	for cell, expected := range map[string]string{"B5": "USysSuTableDynamic_SU", "B12": "8",
		"D5": "P1", "E5": "Alpha", "D7": "P1", "E7": "Beta"} {
		value, err := book.GetCellValue("ReportSummary", cell)
		if err != nil || value != expected {
			t.Fatal("independently reopened source2 summary cell", cell, value, expected, err)
		}
	}
	before := env.Rows[0].Cells[1]
	env.Rows[0].Cells[1] = metadataText("Changed")
	if got, err := vegetationWorkbookSelectedUnitSummary(context.Background(), base, env, admin, su, []string{"Zone"}, report); err == nil || got.CreatedDate != "" {
		t.Fatal("physical selected field drift accepted stale witness", got, err)
	}
	env.Rows[0].Cells[1] = before
	retried, err := vegetationWorkbookSelectedUnitSummary(context.Background(), base, env, admin, su, []string{"Zone"}, report)
	if err != nil || !reflect.DeepEqual(retried, summary) {
		t.Fatal("exact source recovery did not permit explicit reviewed retry", retried, err)
	}
	for _, mutate := range []func(*LongVegetationReport){
		func(r *LongVegetationReport) { r.SelectedUnits = r.SelectedUnits[:1] },
		func(r *LongVegetationReport) { r.SelectedUnits[0].MembershipID = "missing" },
		func(r *LongVegetationReport) { r.SelectedUnits[0].EnvRowID = nil },
		func(r *LongVegetationReport) { r.SelectedUnits[0].AdminRowID = nil },
		func(r *LongVegetationReport) { r.SelectedUnits[0].SiteUnit = metadataText("wrong label") },
		func(r *LongVegetationReport) { r.SelectedUnits = append(r.SelectedUnits, r.SelectedUnits[0]) },
	} {
		copy, err := longVegetationTransport(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&copy)
		if got, err := vegetationWorkbookSelectedUnitSummary(context.Background(), base, env, admin, su, []string{"Zone"}, copy); err == nil || got.CreatedDate != "" {
			t.Fatal("incomplete/forged source2 witness accepted", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := vegetationWorkbookSelectedUnitSummary(ctx, base, env, admin, su, []string{"Zone"}, report); !errors.Is(err, context.Canceled) || got.CreatedDate != "" {
		t.Fatal("cancelled summary leaked result", got, err)
	}
	*summary.Memberships[0].SelectedUnit.EnvRowID = "mutated"
	*summary.Memberships[0].SelectedUnit.SiteUnit.Text = "mutated"
	if *report.SelectedUnits[0].EnvRowID != "11" || *report.SelectedUnits[0].SiteUnit.Text != "Alpha" ||
		*summary.Memberships[1].SelectedUnit.SiteUnit.Text != "Alpha" {
		t.Fatal("summary aliases source/derived peer")
	}
}
