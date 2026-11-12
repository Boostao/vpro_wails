package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestPlotLocationsSourceEightFieldsSignNullsScopeAndDetachedValues(t *testing.T) {
	env, admin, su := vprotest.LocationTestTables()
	report, err := vprocore.PlanPlotLocations(context.Background(), "Project", "None", env, admin, su)
	if err != nil || len(report.Rows) != 2 || len(report.Fields) != 8 {
		t.Fatal("source project-only scope", report, err)
	}
	labels := []string{}
	for _, field := range report.Fields {
		labels = append(labels, field.Label)
	}
	if !reflect.DeepEqual(labels, []string{"Plot Number", "Zone", "Subzone", "Site Series", "Accuracy", "Latitude", "Longitude", "Elevation"}) {
		t.Fatal("source worksheet order/labels changed", labels)
	}
	first, second := report.Rows[0], report.Rows[1]
	if first.EnvRowID != "-9007199254740993" || first.AdminRowID != "1" || len(first.MembershipRowIDs) != 0 ||
		*first.Values[0].Text != "000001" || *first.Values[1].Text != "  Zone  " || *first.Values[2].Text != "" ||
		*first.Values[3].Text != "01" || first.Values[4].Storage != "null" || *first.Values[7].Integer != "9007199254740993" ||
		*first.StoredLongitude.Real != 123.75 || *first.Values[6].Real != -123.75 ||
		*second.StoredLongitude.Real != -123.75 || *second.Values[6].Real != 123.75 || !math.Signbit(*second.Values[5].Real) {
		t.Fatal("source longitude negation, leading zeros, NULL/empty or physical precision changed", report)
	}
	*report.Rows[0].Values[1].Text = "caller"
	*report.Rows[0].StoredLongitude.Real = 1
	report.Fields[0].Label = "caller"
	if *env.Rows[0].Cells[1].Text != "  Zone  " || *env.Rows[0].Cells[6].Real != 123.75 {
		t.Fatal("report shares original cell pointers")
	}
	selected, err := vprocore.PlanPlotLocations(context.Background(), "Project", "Subset", env, admin, su)
	if err != nil || len(selected.Rows) != 1 ||
		!reflect.DeepEqual(selected.Rows[0].MembershipRowIDs, []string{"1", "2", "3"}) {
		t.Fatal("selected membership duplicates/classification NULL/empty altered one report row", selected, err)
	}
}

func TestPlotLocationsNumericNegationPreservesHistoricalValuesAndRejectsCoercion(t *testing.T) {
	for _, test := range []struct {
		cell vprocore.ProjectMetadataCell
		want string
	}{
		{vprotest.SiviReal(0), "-0"}, {vprotest.SiviReal(math.Copysign(0, -1)), "0"},
		{vprotest.SiviReal(-200), "200"}, {vprotest.MetadataInteger("9007199254740993"), "-9007199254740993"},
		{vprotest.MetadataInteger("-9223372036854775807"), "9223372036854775807"},
	} {
		got, err := vprocore.NegateLocationLongitude(test.cell)
		if err != nil {
			t.Fatal(err)
		}
		actual := ""
		if got.Storage == "integer" {
			actual = *got.Integer
		} else {
			actual = strconv.FormatFloat(*got.Real, 'g', -1, 64)
		}
		if actual != test.want {
			t.Fatal("numeric value repaired/rounded", test, got)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("123"), {Storage: "blob", BlobHex: vprotest.QualityString("00")},
		vprotest.MetadataInteger("-9223372036854775808"), vprotest.SiviReal(math.NaN()), vprotest.SiviReal(math.Inf(1))} {
		if _, err := vprocore.NegateLocationLongitude(cell); err == nil {
			t.Fatal("unsupported or overflowing coordinate coerced", cell)
		}
	}
	env, admin, su := vprotest.LocationTestTables()
	env.Rows[1].Cells[5] = vprotest.SiviReal(100)
	env.Rows[1].Cells[6] = vprotest.SiviReal(-200)
	got, err := vprocore.PlanPlotLocations(context.Background(), "Project", "None", env, admin, su)
	if err != nil || *got.Rows[1].Values[5].Real != 100 || *got.Rows[1].Values[6].Real != 200 {
		t.Fatal("historical ranges silently clamped/dropped", got, err)
	}
	zero, err := vprocore.NegateLocationLongitude(vprotest.SiviReal(0))
	if err != nil {
		t.Fatal(err)
	}
	if !math.Signbit(*zero.Real) {
		t.Fatal("source zero sign was repaired")
	}
}

func TestPlotLocationsMalformedAmbiguousAndCancelledInputYieldsNoPartialReport(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable)
	}{
		{"missing field", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Columns[7].Name = "not Elevation" }},
		{"partial row", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[1].Cells = env.Rows[1].Cells[:7] }},
		{"duplicate row ID", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[1].RowID = env.Rows[0].RowID }},
		{"duplicate parent", func(_, admin, _ *vprocore.ProjectMetadataTable) {
			admin.Rows = append(admin.Rows, vprocore.ProjectMetadataRow{RowID: "99", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("000001")}})
		}},
		{"duplicate env", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[1].Cells[0] = vprotest.MetadataText("000001") }},
		{"numeric identity", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[0].Cells[0] = vprotest.MetadataInteger("1") }},
		{"late text longitude", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[1].Cells[6] = vprotest.MetadataText("123") }},
		{"late text latitude", func(env, _, _ *vprocore.ProjectMetadataTable) { env.Rows[1].Cells[5] = vprotest.MetadataText("49") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			env, admin, su := vprotest.LocationTestTables()
			test.mutate(&env, &admin, &su)
			got, err := vprocore.PlanPlotLocations(context.Background(), "Project", "None", env, admin, su)
			if got != nil || err == nil {
				t.Fatal("unsupported inputs returned partial locations", got, err)
			}
		})
	}
	env, admin, su := vprotest.LocationTestTables()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := vprocore.PlanPlotLocations(ctx, "Project", "None", env, admin, su); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled projection returned rows", got, err)
	}
	if got, err := vprocore.PlanPlotLocations(context.Background(), "bad name", "None", env, admin, su); got != nil || err == nil || !strings.Contains(err.Error(), "literal") {
		t.Fatal("implicit family identity accepted", got, err)
	}
}
