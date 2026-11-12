package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func reportNameCell(text *string) vprocore.ProjectMetadataCell {
	if text == nil {
		return vprocore.ProjectMetadataCell{Storage: "null"}
	}
	return vprocore.ProjectMetadataCell{Storage: "text", Text: text}
}

func TestReportUnitNamesPreservesPhysicalCandidatesAndAmbiguity(t *testing.T) {
	empty, name, other, integer := "", "  Unit name  ", "Other", "4"
	for _, test := range []struct {
		label  string
		cells  []vprocore.ProjectMetadataCell
		status string
		name   *string
	}{
		{"absent", nil, "missing", nil},
		{"null", []vprocore.ProjectMetadataCell{reportNameCell(nil)}, "missing", nil},
		{"empty", []vprocore.ProjectMetadataCell{reportNameCell(&empty)}, "unique", &empty},
		{"repeated and NULL", []vprocore.ProjectMetadataCell{reportNameCell(&name), reportNameCell(nil), reportNameCell(&name)}, "unique", &name},
		{"conflicting", []vprocore.ProjectMetadataCell{reportNameCell(&name), reportNameCell(&other)}, "conflicting", nil},
		{"empty is not NULL", []vprocore.ProjectMetadataCell{reportNameCell(&empty), reportNameCell(&name)}, "conflicting", nil},
		{"unsupported", []vprocore.ProjectMetadataCell{reportNameCell(&name), {Storage: "integer", Integer: &integer}}, "unsupported_storage", nil},
	} {
		t.Run(test.label, func(t *testing.T) {
			rows := []vprocore.ProjectMetadataRow{}
			for i, cell := range test.cells {
				rows = append(rows, vprocore.ProjectMetadataRow{RowID: []string{"3", "1", "2"}[i], Cells: []vprocore.ProjectMetadataCell{cell}})
			}
			names, err := reportcore.ResolveReportUnitNames(context.Background(), rows, 0, "Report")
			if err != nil || names.Status != test.status || !reflect.DeepEqual(names.LongName, test.name) ||
				len(names.Candidates) != len(rows) || names.Candidates == nil {
				t.Fatal("reference meaning changed", names, err)
			}
			for i := 1; i < len(names.Candidates); i++ {
				if names.Candidates[i-1].RowID >= names.Candidates[i].RowID {
					t.Fatal("physical candidates not deterministic")
				}
			}
			for _, candidate := range names.Candidates {
				for _, row := range rows {
					if candidate.RowID == row.RowID && !reflect.DeepEqual(candidate.Value, row.Cells[0]) {
						t.Fatal("candidate storage/identity lost")
					}
				}
			}
		})
	}
}

func TestReportUnitNamesCancellationErrorsAndNoAliases(t *testing.T) {
	name := "Original"
	rows := []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{reportNameCell(&name)}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{reportNameCell(&name)}}}
	names, err := reportcore.ResolveReportUnitNames(context.Background(), rows, 0, "Report")
	if err != nil {
		t.Fatal(err)
	}
	*names.Candidates[0].Value.Text = "Changed"
	*names.LongName = "Other"
	if name != "Original" || *names.Candidates[1].Value.Text != "Original" {
		t.Fatal("name/candidate aliases original or adjacent metadata")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := reportcore.ResolveReportUnitNames(ctx, rows, 0, "Report"); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(value, reportcore.ReportUnitNames{}) {
		t.Fatal("cancelled names returned partial data", value, err)
	}
	for _, test := range []struct {
		rows   []vprocore.ProjectMetadataRow
		column int
	}{{rows, -1}, {rows, 1}, {[]vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{{Storage: "text"}}}}, 0}} {
		if value, err := reportcore.ResolveReportUnitNames(context.Background(), test.rows, test.column, "Report"); err == nil ||
			!reflect.DeepEqual(value, reportcore.ReportUnitNames{}) {
			t.Fatal("malformed metadata accepted", value, err)
		}
	}
}

func TestLongVegetationUnitNamesLiteralNullEmptyAndDiagnosticScope(t *testing.T) {
	empty, code, name := "", "U", "Name"
	master := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{reportNameCell(&code), reportNameCell(&name)}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{reportNameCell(&code), reportNameCell(nil)}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{reportNameCell(&empty), reportNameCell(&empty)}},
			{RowID: "4", Cells: []vprocore.ProjectMetadataCell{reportNameCell(nil), reportNameCell(&name)}},
		},
	}
	missing := "missing"
	report := vegetationLayerReport{Units: []vegetationLayerUnit{
		{Code: reportNameCell(&code), NumPlots: 1, MembershipIDs: []string{"1"}},
		{Code: reportNameCell(&empty), NumPlots: 2, MembershipIDs: []string{"2", "3"}},
		{Code: reportNameCell(nil), NumPlots: 1, MembershipIDs: []string{"4"}},
		{Code: reportNameCell(&missing), NumPlots: 1, MembershipIDs: []string{"5"}},
	}, Diagnostics: []vegetationLayerDiagnostic{}}
	if err := addLongVegetationUnitNames(context.Background(), &report, master); err != nil {
		t.Fatal(err)
	}
	if report.Units[0].NameStatus != "unique" || *report.Units[0].LongName != name || len(report.Units[0].NameCandidates) != 2 ||
		report.Units[1].NameStatus != "unique" || *report.Units[1].LongName != "" ||
		report.Units[2].NameStatus != "unassigned" || *report.Units[2].LongName != "" || len(report.Units[2].NameCandidates) != 0 ||
		report.Units[3].NameStatus != "missing" || report.Units[3].LongName != nil ||
		!reflect.DeepEqual(report.Diagnostics, []vegetationLayerDiagnostic{{"unit_name_missing", "text:missing", 0}}) {
		t.Fatal("unit join/default/NULL/empty meaning differs", report)
	}
	if report.Units[0].NumPlots != 1 || len(report.Units[0].MembershipIDs) != 1 {
		t.Fatal("reference multiplicity changed the plot denominator")
	}
	*report.Units[0].NameCandidates[0].Value.Text = "Changed"
	if name != "Name" || *report.Units[0].LongName != "Name" {
		t.Fatal("unit names alias source/reference candidates")
	}
}
