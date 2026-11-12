package integration

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"

	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLongEnvironmentExactRegistryScopeTypedValuesAndOrphans(t *testing.T) {
	env, admin, su, master := sftest.EnvironmentReportFixture()
	report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "  User's title  ", env, admin, su, master)
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
	env, admin, su, master := sftest.EnvironmentReportFixture()
	before, _ := json.Marshal([]vprocore.ProjectMetadataTable{env, admin, su, master})
	expected, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(882))
	for n := 0; n < 30; n++ {
		for _, table := range []*vprocore.ProjectMetadataTable{&env, &admin, &su, &master} {
			rng.Shuffle(len(table.Rows), func(i, j int) { table.Rows[i], table.Rows[j] = table.Rows[j], table.Rows[i] })
		}
		report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
		if err != nil || !reflect.DeepEqual(report, expected) {
			t.Fatal("row permutation changes report", n, report, err)
		}
	}
	env, admin, su, master = sftest.EnvironmentReportFixture()
	report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
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
	after, _ := json.Marshal([]vprocore.ProjectMetadataTable{env, admin, su, master})
	if string(before) != string(after) {
		t.Fatal("planner/output aliases original typed snapshots")
	}
	report.Fields[0].Label = "Changed label"
	if reportcore.LongEnvironmentFields()[0].Label != "Plot" {
		t.Fatal("caller corrupted shared report registry")
	}
}

func TestLongEnvironmentDuplicateMembershipEmptyNameAndRawQuotedCodes(t *testing.T) {
	env, admin, su, master := sftest.EnvironmentReportFixture()
	duplicate := su.Rows[0]
	duplicate.RowID = "7"
	su.Rows = append(su.Rows, duplicate)
	su.Rows[0].Cells[1] = vprotest.MetadataText("  U 'quoted'  ")
	su.Rows[6].Cells = append([]vprocore.ProjectMetadataCell(nil), su.Rows[0].Cells...)
	master.Rows = append(master.Rows, vprocore.ProjectMetadataRow{RowID: "6", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("  U 'quoted'  "), vprotest.MetadataText("")}})
	report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
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
			env, admin, su, master := sftest.EnvironmentReportFixture()
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
				su.Rows = append(su.Rows, vprocore.ProjectMetadataRow{RowID: "99", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Different")}})
			case "invalid-identity":
				su.Rows[0].Cells[0] = vprotest.MetadataInteger("1")
			case "malformed-cell":
				env.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "text"}
			case "malformed-text":
				env.Rows[0].Cells[1] = vprotest.MetadataText(string([]byte{0xff}))
			case "duplicate-rowid":
				env.Rows[1].RowID = env.Rows[0].RowID
			}
			report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
			if err == nil || !reflect.DeepEqual(report, reportcore.EnvironmentReport{}) {
				t.Fatal("invalid input publishes partial/success-shaped report", report, err)
			}
		})
	}
}

func TestLongEnvironmentEmptyScopeCancellationAndTypedNameDiagnostic(t *testing.T) {
	env, admin, su, master := sftest.EnvironmentReportFixture()
	su.Rows = nil
	report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
	if err != nil || report.Units == nil || len(report.Units) != 0 || len(report.Fields) != 72 {
		t.Fatal("empty eligible scope not represented", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err = reportcore.PlanLongEnvironment(ctx, "Project", "Selected", "", env, admin, su, master)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, reportcore.EnvironmentReport{}) {
		t.Fatal("cancellation publishes output", report, err)
	}
	for _, identities := range [][3]string{{"", "Selected", ""}, {"Project", "None", ""}, {"Project", "", ""}, {"Project", "Selected", string([]byte{0xff})}, {string([]byte{0xc2}), string([]byte{0xa9}), ""}} {
		if _, err := reportcore.PlanLongEnvironment(context.Background(), identities[0], identities[1], identities[2], env, admin, su, master); err == nil {
			t.Fatal("invalid identity/title accepted", identities)
		}
	}
	env, admin, su, master = sftest.EnvironmentReportFixture()
	master.Rows[0].Cells[1] = vprotest.MetadataInteger("0")
	report, err = reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
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
	env, admin, su, master := sftest.EnvironmentReportFixture()
	ctx := &environmentReportCancellation{Context: context.Background()}
	report, err := reportcore.PlanLongEnvironment(ctx, "Project", "Selected", "", env, admin, su, master)
	if !errors.Is(err, context.Canceled) || ctx.calls < 10 || !reflect.DeepEqual(report, reportcore.EnvironmentReport{}) {
		t.Fatal("mid-plan cancellation publishes partial output", report, err)
	}
}

func TestLongEnvironmentRealBlobAndSigned64StorageWithoutAliases(t *testing.T) {
	env, admin, su, master := sftest.EnvironmentReportFixture()
	real, blob := 0.5, "00ff"
	for i, column := range env.Columns {
		switch column.Name {
		case "Elevation":
			env.Rows[0].Cells[i] = vprocore.ProjectMetadataCell{Storage: "real", Real: &real}
		case "Location":
			env.Rows[0].Cells[i] = vprocore.ProjectMetadataCell{Storage: "blob", BlobHex: &blob}
		case "FieldNumber":
			env.Rows[0].Cells[i] = vprotest.MetadataInteger("-9223372036854775808")
		}
	}
	before, _ := json.Marshal(env)
	report, err := reportcore.PlanLongEnvironment(context.Background(), "Project", "Selected", "", env, admin, su, master)
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
