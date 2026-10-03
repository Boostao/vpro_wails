package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func environmentReportFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	env, admin := ProjectMetadataTable{}, ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Plot"}}}
	for _, field := range longEnvironmentFields() {
		if !field.Heading {
			table := &env
			if field.Source == "Admin" {
				table = &admin
			}
			table.Columns = append(table.Columns, ProjectMetadataColumn{Name: field.Key})
		}
	}
	for _, plot := range []string{"P1", "P2", "P4"} {
		for _, table := range []*ProjectMetadataTable{&env, &admin} {
			if table == &admin && plot == "P2" {
				continue
			}
			row := ProjectMetadataRow{RowID: fmt.Sprint(len(table.Rows) + 1), Cells: make([]ProjectMetadataCell, len(table.Columns))}
			for i, column := range table.Columns {
				row.Cells[i] = ProjectMetadataCell{Storage: "null"}
				switch column.Name {
				case "PlotNumber", "Plot":
					row.Cells[i] = metadataText(plot)
				case "Location":
					row.Cells[i] = metadataText("")
				case "Elevation":
					row.Cells[i] = metadataInteger("0")
				case "UserSiteUnit":
					row.Cells[i] = metadataText("OTHER")
				case "SitePlotQuality":
					row.Cells[i] = metadataText("Poor")
				}
			}
			table.Rows = append(table.Rows, row)
		}
	}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("U")}},
		{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("U")}},
		{RowID: "3", Cells: []ProjectMetadataCell{metadataText("P3"), metadataText("V")}},
		{RowID: "4", Cells: []ProjectMetadataCell{metadataText(""), metadataText("")}},
		{RowID: "5", Cells: []ProjectMetadataCell{{Storage: "null"}, metadataText("W")}},
		{RowID: "6", Cells: []ProjectMetadataCell{metadataText("P4"), {Storage: "null"}}},
	}}
	master := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{metadataText("U"), metadataText("Unit U")}},
		{RowID: "2", Cells: []ProjectMetadataCell{metadataText("U"), {Storage: "null"}}},
		{RowID: "3", Cells: []ProjectMetadataCell{metadataText("U"), metadataText("Unit U")}},
		{RowID: "4", Cells: []ProjectMetadataCell{metadataText("V"), metadataText("Alpha")}},
		{RowID: "5", Cells: []ProjectMetadataCell{metadataText("V"), metadataText("Beta")}},
	}}
	return env, admin, su, master
}

func TestLongEnvironmentExactRegistryScopeTypedValuesAndOrphans(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "  User's title  ", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "long-environment-fields.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines, headings := []string{}, 0
	for _, field := range report.Fields {
		if field.Heading {
			lines = append(lines, "# "+field.Label)
			headings++
			if field.Source != "" || field.Key != "" {
				t.Fatal("heading became an observation", field)
			}
		} else {
			lines = append(lines, field.Source+"."+field.Key+"="+field.Label)
		}
	}
	if len(lines) != 72 || headings != 5 || strings.Join(lines, "\n")+"\n" != strings.ReplaceAll(string(expected), "\r\n", "\n") {
		t.Fatal("exact 67 fields/five headings differ from source fixture", lines)
	}
	if report.Title != "  User's title  " || len(report.Units) != 3 || report.Units[0].Code != "" ||
		report.Units[1].Code != "U" || report.Units[2].Code != "V" {
		t.Fatal("raw title/unit identity or eligible scope lost", report)
	}
	u, v := report.Units[1], report.Units[2]
	if u.NameStatus != "unique" || u.LongName == nil || *u.LongName != "Unit U" || len(u.NameCandidates) != 3 ||
		u.NameCandidates[1].Value.Storage != "null" || v.NameStatus != "conflicting" || v.LongName != nil {
		t.Fatal("NULL/duplicate/conflicting definitions lost", u, v)
	}
	if len(u.Plots) != 2 || u.Plots[0].PlotNumber != "P1" || u.Plots[0].Status != "complete" ||
		u.Plots[1].Status != "missing_admin" || len(v.Plots) != 1 || v.Plots[0].Status != "missing_env_and_admin" {
		t.Fatal("physical membership/orphan projection differs", u, v)
	}
	values := u.Plots[0].Values
	for i, field := range report.Fields {
		switch field.Key {
		case "Elevation":
			if values[i].Storage != "integer" || *values[i].Integer != "0" {
				t.Fatal("numeric zero repaired")
			}
		case "Location":
			if values[i].Storage != "text" || *values[i].Text != "" {
				t.Fatal("empty text lost")
			}
		case "Longitude":
			if values[i].Storage != "null" {
				t.Fatal("NULL coordinate imputed")
			}
		case "UserSiteUnit":
			if *values[i].Text != "OTHER" {
				t.Fatal("assigned unit replaced with report grouping")
			}
		}
	}
	for _, unit := range report.Units {
		for _, plot := range unit.Plots {
			for i, value := range plot.Values {
				if (report.Fields[i].Heading || plot.Status != "complete") && value.Storage != "null" {
					t.Fatal("heading/orphan contains invented observation")
				}
			}
		}
	}
}

func TestLongEnvironmentPermutationRepeatAndIndependentOutput(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	before, _ := json.Marshal([]ProjectMetadataTable{env, admin, su, master})
	expected, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(882))
	for n := 0; n < 30; n++ {
		for _, table := range []*ProjectMetadataTable{&env, &admin, &su, &master} {
			rng.Shuffle(len(table.Rows), func(i, j int) { table.Rows[i], table.Rows[j] = table.Rows[j], table.Rows[i] })
		}
		report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
		if err != nil || !reflect.DeepEqual(report, expected) {
			t.Fatal("row permutation changes report", n, report, err)
		}
	}
	env, admin, su, master = environmentReportFixture()
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range report.Units[1].Plots[0].Values {
		if cell.Text != nil {
			*cell.Text = "Caller changed output"
		}
		if cell.Integer != nil {
			*cell.Integer = "123"
		}
	}
	*report.Units[1].NameCandidates[0].Value.Text = "Changed name"
	after, _ := json.Marshal([]ProjectMetadataTable{env, admin, su, master})
	if string(before) != string(after) {
		t.Fatal("planner/output aliases original typed snapshots")
	}
	report.Fields[0].Label = "Changed label"
	if longEnvironmentFields()[0].Label != "Plot" {
		t.Fatal("caller corrupted shared report registry")
	}
}

func TestLongEnvironmentDuplicateMembershipEmptyNameAndRawQuotedCodes(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	duplicate := su.Rows[0]
	duplicate.RowID = "7"
	su.Rows = append(su.Rows, duplicate)
	su.Rows[0].Cells[1] = metadataText("  U 'quoted'  ")
	su.Rows[6].Cells = append([]ProjectMetadataCell(nil), su.Rows[0].Cells...)
	master.Rows = append(master.Rows, ProjectMetadataRow{RowID: "6", Cells: []ProjectMetadataCell{metadataText("  U 'quoted'  "), metadataText("")}})
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		if unit.Code == "  U 'quoted'  " && (unit.LongName == nil || *unit.LongName != "" || unit.NameStatus != "unique" || len(unit.Plots) != 1) {
			t.Fatal("raw quoted code/empty name or membership collapse differs", unit)
		}
	}
	found := false
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "duplicate_membership" {
			found = diagnostic.Count == 1 && *diagnostic.PlotNumber == "P1" && *diagnostic.Unit == "  U 'quoted'  "
		}
	}
	if !found {
		t.Fatal("duplicate membership not independently reported", report.Diagnostics)
	}
}

func TestLongEnvironmentFailsClosedWithoutPartialOutput(t *testing.T) {
	for _, mode := range []string{"missing-env-column", "missing-admin-column", "missing-su-column", "missing-master-column",
		"duplicate-env", "duplicate-admin", "conflicting-su", "invalid-identity", "malformed-cell", "malformed-text", "duplicate-rowid"} {
		t.Run(mode, func(t *testing.T) {
			env, admin, su, master := environmentReportFixture()
			switch mode {
			case "missing-env-column":
				env.Columns[1].Name = "Absent"
			case "missing-admin-column":
				admin.Columns[1].Name = "Absent"
			case "missing-su-column":
				su.Columns[1].Name = "Absent"
			case "missing-master-column":
				master.Columns[1].Name = "Absent"
			case "duplicate-env":
				duplicate := env.Rows[0]
				duplicate.RowID = "99"
				env.Rows = append(env.Rows, duplicate)
			case "duplicate-admin":
				duplicate := admin.Rows[0]
				duplicate.RowID = "99"
				admin.Rows = append(admin.Rows, duplicate)
			case "conflicting-su":
				su.Rows = append(su.Rows, ProjectMetadataRow{RowID: "99", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Different")}})
			case "invalid-identity":
				su.Rows[0].Cells[0] = metadataInteger("1")
			case "malformed-cell":
				env.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "text"}
			case "malformed-text":
				env.Rows[0].Cells[1] = metadataText(string([]byte{0xff}))
			case "duplicate-rowid":
				env.Rows[1].RowID = env.Rows[0].RowID
			}
			report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
			if err == nil || !reflect.DeepEqual(report, EnvironmentReport{}) {
				t.Fatal("invalid input publishes partial/success-shaped report", report, err)
			}
		})
	}
}

func TestLongEnvironmentEmptyScopeCancellationAndTypedNameDiagnostic(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	su.Rows = nil
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil || report.Units == nil || len(report.Units) != 0 || len(report.Fields) != 72 {
		t.Fatal("empty eligible scope not represented", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err = planLongEnvironment(ctx, "Project", "Selected", "", env, admin, su, master)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, EnvironmentReport{}) {
		t.Fatal("cancellation publishes output", report, err)
	}
	for _, identities := range [][3]string{{"", "Selected", ""}, {"Project", "None", ""}, {"Project", "", ""}, {"Project", "Selected", string([]byte{0xff})}, {string([]byte{0xc2}), string([]byte{0xa9}), ""}} {
		if _, err := planLongEnvironment(context.Background(), identities[0], identities[1], identities[2], env, admin, su, master); err == nil {
			t.Fatal("invalid identity/title accepted", identities)
		}
	}
	env, admin, su, master = environmentReportFixture()
	master.Rows[0].Cells[1] = metadataInteger("0")
	report, err = planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil || report.Units[1].NameStatus != "unsupported_storage" || report.Units[1].LongName != nil ||
		report.Units[1].NameCandidates[0].Value.Storage != "integer" {
		t.Fatal("historical unsupported name silently converted or discarded", report, err)
	}
}

type environmentReportCancellation struct {
	context.Context
	calls int
}

func (c *environmentReportCancellation) Err() error {
	c.calls++
	if c.calls >= 10 {
		return context.Canceled
	}
	return nil
}

func TestLongEnvironmentMidPlanCancellationPublishesNothing(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	ctx := &environmentReportCancellation{Context: context.Background()}
	report, err := planLongEnvironment(ctx, "Project", "Selected", "", env, admin, su, master)
	if !errors.Is(err, context.Canceled) || ctx.calls < 10 || !reflect.DeepEqual(report, EnvironmentReport{}) {
		t.Fatal("mid-plan cancellation publishes partial output", report, err)
	}
}

func TestLongEnvironmentRealBlobAndSigned64StorageWithoutAliases(t *testing.T) {
	env, admin, su, master := environmentReportFixture()
	real, blob := 0.5, "00ff"
	for i, column := range env.Columns {
		switch column.Name {
		case "Elevation":
			env.Rows[0].Cells[i] = ProjectMetadataCell{Storage: "real", Real: &real}
		case "Location":
			env.Rows[0].Cells[i] = ProjectMetadataCell{Storage: "blob", BlobHex: &blob}
		case "FieldNumber":
			env.Rows[0].Cells[i] = metadataInteger("-9223372036854775808")
		}
	}
	before, _ := json.Marshal(env)
	report, err := planLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	values := report.Units[1].Plots[0].Values
	for i, field := range report.Fields {
		switch field.Key {
		case "Elevation":
			if values[i].Storage != "real" || *values[i].Real != real {
				t.Fatal("historical real storage lost")
			}
			*values[i].Real = 123
		case "Location":
			if values[i].Storage != "blob" || *values[i].BlobHex != blob {
				t.Fatal("historical blob storage lost")
			}
			*values[i].BlobHex = "ffff"
		case "FieldNumber":
			if values[i].Storage != "integer" || *values[i].Integer != "-9223372036854775808" {
				t.Fatal("exact signed64 value lost")
			}
			*values[i].Integer = "0"
		}
	}
	after, _ := json.Marshal(env)
	if string(before) != string(after) {
		t.Fatal("typed report output aliases source numeric/blob pointers")
	}
}
