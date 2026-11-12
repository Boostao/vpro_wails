package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func vegetationSummaryFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}}},
		},
	}
	su := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("=P1"), vprotest.MetadataText("U")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("=P1"), vprotest.MetadataText("V")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("")}},
			{RowID: "4", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}}},
		},
	}
	metadata := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "table_name"}, {Name: "description"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), vprotest.MetadataText("=version")}},
		},
	}
	return env, su, metadata
}

func TestLongVegetationWorkbookSummaryMetadataCountsAndRefusals(t *testing.T) {
	env, su, metadata := vegetationSummaryFixture()
	for _, tc := range []struct {
		name, status, display string
		metadata              vprocore.ProjectMetadataTable
	}{
		{"literal", "literal", "=version", metadata},
		{"absent", "metadata-unavailable", "Unknown", vprocore.ProjectMetadataTable{}},
		{"missing", "missing-definition", "Unknown", vprocore.ProjectMetadataTable{Columns: metadata.Columns}},
		{"null", "null", "", vprocore.ProjectMetadataTable{Columns: metadata.Columns,
			Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), {Storage: "null"}}}}}},
		{"empty", "literal", "", vprocore.ProjectMetadataTable{Columns: metadata.Columns,
			Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), vprotest.MetadataText("")}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-06", env, su, tc.metadata)
			if err != nil || summary.EnvironmentRows != 3 || summary.SelectedPlotRows != 3 ||
				len(summary.Memberships) != 4 || summary.VersionStatus != tc.status ||
				!reflect.DeepEqual(summary.VersionDefinitions, tc.metadata) {
				t.Fatal("source row counts/metadata state changed", summary, err)
			}
			value, err := vprocore.VegetationReportIdentity(summary.SpeciesVersion)
			if err != nil || value != nil && *value != tc.display || value == nil && tc.name != "null" {
				t.Fatal("literal/NULL/empty Description changed", value, err)
			}
		})
	}
	for _, date := range []string{"", "2026-2-03", "2026-02-29", "0099-01-01", "2026-10-06T00:00:00Z"} {
		if _, err := prepareVegetationWorkbookSummary(context.Background(), date, env, su, metadata); err == nil {
			t.Fatal("unreviewed/malformed date accepted", date)
		}
	}
	if _, err := prepareVegetationWorkbookSummary(context.Background(), "2024-02-29", env, su, metadata); err != nil {
		t.Fatal("valid leap calendar date refused", err)
	}
	duplicate := metadata
	duplicate.Rows = append(append([]vprocore.ProjectMetadataRow{}, metadata.Rows...), vprocore.ProjectMetadataRow{RowID: "2", Cells: metadata.Rows[0].Cells})
	malformedSU := su
	malformedSU.Rows = []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}}}
	numericSU := su
	numericSU.Rows = []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.MetadataText("U")}}}
	badDescription := vprocore.ProjectMetadataTable{Columns: metadata.Columns,
		Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("USysAllSpecs"), vprotest.MetadataInteger("7")}}}}
	foreignDescription := vprocore.ProjectMetadataTable{Columns: metadata.Columns,
		Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Other"), vprotest.MetadataText("version")}}}}
	for _, tc := range []struct{ env, su, metadata vprocore.ProjectMetadataTable }{
		{env, su, duplicate}, {env, malformedSU, metadata}, {env, numericSU, metadata},
		{env, su, badDescription}, {env, su, foreignDescription},
		{env, su, vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "wrong"}}}},
	} {
		summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-06", tc.env, tc.su, tc.metadata)
		if err == nil || !reflect.DeepEqual(summary, vegetationWorkbookSummary{}) {
			t.Fatal("invalid source returned partial summary", summary, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareVegetationWorkbookSummary(ctx, "2026-10-06", env, su, metadata); !errors.Is(err, context.Canceled) {
		t.Fatal("summary lost cancellation", err)
	}
	if _, err := prepareVegetationWorkbookSummary(nil, "2026-10-06", env, su, metadata); err == nil {
		t.Fatal("nil summary context accepted")
	}
}

func TestLongVegetationWorkbookSummarySourceCellsLegendAndProvenance(t *testing.T) {
	env, su, metadata := vegetationSummaryFixture()
	summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-06", env, su, metadata)
	if err != nil {
		t.Fatal(err)
	}
	preview := vegetationWorkbookFixture(t)
	preview.Settings.Grouping = "lifeform"
	for unit := range preview.Report.Units {
		for row := range preview.Report.Units[unit].Rows {
			preview.Report.Units[unit].Rows[row].Layer = vprotest.MetadataInteger("2")
		}
	}
	preview.Settings.Order = "species"
	preview.Settings.PresenceGreaterThan = 50
	layout := vegetationWorkbookLayout{QuickReport: true, ReportSummary: true, Summary: &summary}
	first, err := prepareLongVegetationWorkbook(context.Background(), preview, layout)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareLongVegetationWorkbook(context.Background(), preview, layout)
	if err != nil || !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatal("reviewed summary date produces nondeterministic output", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(first.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for address, want := range map[string]string{
		"A1": preview.Report.Title, "A2": "Date created: 2026-10-06",
		"A4": "Vegetation table:", "B4": preview.Report.Project + "_Env",
		"A5": "Site unit table:", "B5": preview.Report.SU + "_SU",
		"A6": "Lumping table:", "B6": "None_Lump",
		"A7": "Data grouped by:", "B7": "Lifeform", "A8": "Data ordered by:", "B8": "Species",
		"A9": "Presence > than:", "A10": "Mean cover > than:",
		"A11": "Number of plots in database", "B11": "3",
		"A12": "Number of plots in site unit table", "B12": "3", "A13": "AllSpecs version:", "B13": "=version",
		"D4": "Plot Number", "E4": "Site Unit", "D5": "=P1", "D6": "=P1", "D7": "", "D8": "",
		"E5": "U", "E6": "V", "G4": "Code", "H4": "Description", "G5": "0", "G18": "13",
		"H5": "genus-level and mixed", "H18": "macro alga",
	} {
		got, err := book.GetCellValue("ReportSummary", address, excelize.Options{RawCellValue: true})
		if err != nil || got != want {
			t.Fatal("source Summary cell or non-overlapping legend changed", address, got, want, err)
		}
	}
	for _, address := range []string{"B13", "D5"} {
		formula, err := book.GetCellFormula("ReportSummary", address)
		if err != nil || formula != "" {
			t.Fatal("summary literal became formula", address, formula, err)
		}
	}
	for address, want := range map[string]string{
		"B9":  "0.5",
		"B10": strconv.FormatFloat(preview.Settings.MeanCoverGreaterThan, 'f', -1, 64),
	} {
		got, err := book.GetCellValue("ReportSummary", address, excelize.Options{RawCellValue: true})
		kind, typeErr := book.GetCellType("ReportSummary", address)
		if err != nil || typeErr != nil || got != want || kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
			t.Fatal("summary threshold is not original typed numeric", address, got, kind, err, typeErr)
		}
	}
	sheets := book.GetSheetList()
	if sheets[len(sheets)-2] != "ReportSummary" || sheets[len(sheets)-1] != "_VPRO_Source" {
		t.Fatal("source summary is not after unit sheets", sheets)
	}
	var chunks strings.Builder
	for row := 1; ; row++ {
		value, err := book.GetCellValue("_VPRO_Source", "A"+strconv.Itoa(row))
		if err != nil {
			t.Fatal(err)
		}
		if value == "" {
			break
		}
		chunks.WriteString(value)
	}
	data, err := hex.DecodeString(chunks.String())
	if err != nil {
		t.Fatal(err)
	}
	var source struct{ Layout vegetationWorkbookLayout }
	if err := json.Unmarshal(data, &source); err != nil || !reflect.DeepEqual(source.Layout, layout) {
		t.Fatal("reviewed summary date/NULL/empty/raw Description/membership provenance lost", err)
	}
	for _, name := range []string{"REPORTSUMMARY", "Report\u017fummary"} {
		preview.Report.Units[0].Code = vprotest.MetadataText(name)
		result, err := prepareLongVegetationWorkbook(context.Background(), preview, layout)
		if err == nil || !reflect.DeepEqual(result, vegetationWorkbook{}) {
			t.Fatal("summary/unit sheet collision overwrote output", name, err)
		}
	}
}

func TestLongVegetationWorkbookUnicodeFoldSheetCollisions(t *testing.T) {
	preview := vegetationWorkbookFixture(t)
	if len(preview.Report.Units) < 2 {
		t.Fatal("fixture requires two physical units")
	}
	preview.Report.Units[0].Code = vprotest.MetadataText("S")
	preview.Report.Units[1].Code = vprotest.MetadataText("\u017f")
	result, err := prepareLongVegetationWorkbook(context.Background(), preview, vegetationWorkbookLayout{})
	if err == nil || !reflect.DeepEqual(result, vegetationWorkbook{}) {
		t.Fatal("Excelize-equivalent Unicode names silently overwrote unit data", err)
	}
}
