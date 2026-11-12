package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectUnitFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env, admin, _ := selectedUnitFixture()
	env.Rows = append(env.Rows,
		vprocore.ProjectMetadataRow{RowID: "5", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("A2"), vprotest.MetadataText("bc"), vprotest.MetadataText("01"), vprotest.MetadataInteger("7"), vprotest.MetadataInteger("-1")}},
		vprocore.ProjectMetadataRow{RowID: "6", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("orphan"), vprotest.MetadataText("unmatched"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
		vprocore.ProjectMetadataRow{RowID: "7", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("null plot"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
		vprocore.ProjectMetadataRow{RowID: "8", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("empty plot"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
	)
	admin.Rows = append(admin.Rows,
		vprocore.ProjectMetadataRow{RowID: "16", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("null admin")}},
		vprocore.ProjectMetadataRow{RowID: "17", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("empty admin")}},
		vprocore.ProjectMetadataRow{RowID: "18", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("admin only"), vprotest.MetadataText("unmatched")}},
	)
	return env, admin
}

func TestProjectSelectedUnitsOriginalInnerJoin(t *testing.T) {
	env, admin := projectUnitFixture()
	before, err := json.Marshal([]vprocore.ProjectMetadataTable{env, admin})
	if err != nil {
		t.Fatal(err)
	}
	got, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone", "Comment"})
	if err != nil {
		t.Fatal(err)
	}
	want := []vegetationProjectSelectedUnitOccurrence{
		{"1", "11", vprotest.MetadataText("P"), vprotest.MetadataText("A first ")},
		{"1", "12", vprotest.MetadataText("P"), vprotest.MetadataText("Asecond")},
		{"2", "13", vprotest.MetadataText("Q"), vprocore.ProjectMetadataCell{Storage: "null"}},
		{"3", "14", vprotest.MetadataText("R"), vprotest.MetadataText("Abthird")},
		{"4", "15", vprotest.MetadataText("S"), vprotest.MetadataText("")},
		{"5", "11", vprotest.MetadataText("P"), vprotest.MetadataText("A2 first ")},
		{"5", "12", vprotest.MetadataText("P"), vprotest.MetadataText("A2second")},
		{"8", "17", vprotest.MetadataText(""), vprotest.MetadataText("empty plotempty admin")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("original Env/Admin complete fanout, INNER JOIN NULL/unmatched and literal empty/space differ", got, want)
	}
	after, err := json.Marshal([]vprocore.ProjectMetadataTable{env, admin})
	if err != nil || string(before) != string(after) {
		t.Fatal("pure project projection changed inputs", err)
	}
	*got[0].PlotNumber.Text = "changed"
	*got[0].SiteUnit.Text = "changed"
	if *env.Rows[0].Cells[0].Text != "P" || *admin.Rows[0].Cells[1].Text != " first " ||
		*got[1].PlotNumber.Text != "P" || *got[1].SiteUnit.Text != "Asecond" {
		t.Fatal("project output aliases original or peer")
	}
	reordered, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Comment", "Zone"})
	if err != nil || *reordered[0].SiteUnit.Text != " first A" || *reordered[1].SiteUnit.Text != "secondA" {
		t.Fatal("explicit order or separator/space changed", reordered, err)
	}
}

func TestProjectNoneUnitsOriginalEnvOnly(t *testing.T) {
	env, _ := projectUnitFixture()
	got, err := planLongVegetationProjectNoneUnits(context.Background(), env)
	want := []vegetationProjectNoneUnitOccurrence{
		{"1", vprotest.MetadataText("P"), vprotest.MetadataText("All plots")},
		{"2", vprotest.MetadataText("Q"), vprotest.MetadataText("All plots")},
		{"3", vprotest.MetadataText("R"), vprotest.MetadataText("All plots")},
		{"4", vprotest.MetadataText("S"), vprotest.MetadataText("All plots")},
		{"5", vprotest.MetadataText("P"), vprotest.MetadataText("All plots")},
		{"6", vprotest.MetadataText("orphan"), vprotest.MetadataText("All plots")},
		{"7", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("All plots")},
		{"8", vprotest.MetadataText(""), vprotest.MetadataText("All plots")},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("None must preserve every original Env, including duplicate/NULL/unmatched/empty plots", got, err)
	}
	*got[0].PlotNumber.Text = "changed"
	*got[0].SiteUnit.Text = "changed"
	if *env.Rows[0].Cells[0].Text != "P" || *got[4].PlotNumber.Text != "P" || *got[1].SiteUnit.Text != "All plots" {
		t.Fatal("None output aliases original or peer")
	}
	env.Rows = nil
	got, err = planLongVegetationProjectNoneUnits(context.Background(), env)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty Env must yield explicit empty projection", got, err)
	}
}

func TestProjectSelectedUnitsScalarsAndLabelBounds(t *testing.T) {
	env, admin := projectUnitFixture()
	got, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Num", "Flag"})
	if err != nil || *got[0].SiteUnit.Text != "7-1" || *got[2].SiteUnit.Text != "-20" ||
		got[3].SiteUnit.Storage != "null" || *got[4].SiteUnit.Text != "0-1" {
		t.Fatal("original Long/Boolean -1/0 and NULL differ", got, err)
	}
	for _, test := range []struct {
		text  string
		valid bool
	}{
		{strings.Repeat("x", 255), true}, {strings.Repeat("x", 256), false},
		{strings.Repeat("\U0001f332", 127) + "x", true}, {strings.Repeat("\U0001f332", 128), false},
		{"", true}, {" Literal Case ", true}, {"bad\x00text", false}, {string([]byte{0xff}), false},
	} {
		env, admin := projectUnitFixture()
		env.Rows[0].Cells[1] = vprotest.MetadataText(test.text)
		rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone"})
		if test.valid {
			if err != nil || *rows[0].SiteUnit.Text != test.text {
				t.Fatal("valid original literal label changed", test.text, rows, err)
			}
		} else if err == nil || rows != nil {
			t.Fatal("invalid new UTF16/NUL/Unicode label accepted or partial plan leaked", len(test.text), rows, err)
		}
	}
	env.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 255))
	env.Rows[0].Cells[2] = vprotest.MetadataText("z")
	if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone", "SubZone"}); err == nil || rows != nil {
		t.Fatal("combined label bounds must be enforced", rows, err)
	}
}

func TestProjectUnitsPhysicalRefusals(t *testing.T) {
	for _, fields := range [][]string{nil, {"Zone", "zone"}, {"missing"}, {"[Zone]"}, {"Zone;SubZone"}, {"PlotNumber", "Plot"}} {
		env, admin := projectUnitFixture()
		if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, fields); fields != nil && reflect.DeepEqual(fields, []string{"PlotNumber", "Plot"}) {
			if err != nil || *rows[0].SiteUnit.Text != "PP" {
				t.Fatal("physical Env/Admin identity fields are valid explicit selections", rows, err)
			}
		} else if err == nil || rows != nil {
			t.Fatal("invalid field selection accepted", fields, rows, err)
		}
	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable){
		func(e *vprocore.ProjectMetadataTable) { e.Columns[0].Name = "missing" },
		func(e *vprocore.ProjectMetadataTable) { e.Columns[1].Name = "PlotNumber" },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].RowID = "01" },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[1].RowID = "1" },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].Cells = e.Rows[0].Cells[:1] },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].Cells[0] = vprotest.MetadataInteger("7") },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].Cells[0] = vprotest.MetadataText("p") },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].Cells[0] = vprotest.MetadataText("P\x00") },
		func(e *vprocore.ProjectMetadataTable) { e.Rows[0].Cells[0] = vprotest.MetadataText("\u00e9") },
		func(e *vprocore.ProjectMetadataTable) {
			e.Rows[0].Cells[0] = vprotest.MetadataText(string([]byte{0xff}))
		},
	} {
		env, admin := projectUnitFixture()
		mutate(&env)
		if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone"}); err == nil || rows != nil {
			t.Fatal("invalid complete project source2 rows/identity accepted", rows, err)
		}
		if rows, err := planLongVegetationProjectNoneUnits(context.Background(), env); err == nil || rows != nil {
			t.Fatal("invalid complete project source3 rows/identity accepted", rows, err)
		}
	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable){
		func(e, a *vprocore.ProjectMetadataTable) { a.Columns[0].Name = "missing" },
		func(e, a *vprocore.ProjectMetadataTable) { a.Rows[1].RowID = "11" },
		func(e, a *vprocore.ProjectMetadataTable) { a.Rows[0].Cells = nil },
		func(e, a *vprocore.ProjectMetadataTable) { a.Rows[0].Cells[0] = vprotest.MetadataText("p") },
		func(e, a *vprocore.ProjectMetadataTable) { a.Rows[7].Cells[0] = vprotest.MetadataInteger("7") },
		func(e, a *vprocore.ProjectMetadataTable) { e.Columns[1].DeclaredType = "REAL" },
		func(e, a *vprocore.ProjectMetadataTable) { a.Columns[1].Name = "Zone" },
		func(e, a *vprocore.ProjectMetadataTable) {
			e.Rows[5].Cells[1] = vprotest.MetadataText("bad\x00unmatched")
		},
		func(e, a *vprocore.ProjectMetadataTable) {
			a.Rows[7].Cells[1] = vprotest.MetadataText(string([]byte{0xff}))
		},
	} {
		env, admin := projectUnitFixture()
		mutate(&env, &admin)
		if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone", "Comment"}); err == nil || rows != nil {
			t.Fatal("invalid Admin/unmatched selected scalar/ambiguous field accepted", rows, err)
		}
	}
	for _, test := range []struct {
		column int
		field  string
		cell   vprocore.ProjectMetadataCell
	}{
		{4, "Num", vprotest.MetadataInteger("2147483648")}, {4, "Num", vprotest.MetadataInteger("-2147483649")},
		{4, "Num", vprotest.MetadataText("7")}, {5, "Flag", vprotest.MetadataInteger("1")},
		{5, "Flag", vprotest.MetadataInteger("-2")}, {5, "Flag", vprotest.MetadataText("True")},
	} {
		env, admin := projectUnitFixture()
		env.Rows[5].Cells[test.column] = test.cell
		if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{test.field}); err == nil || rows != nil {
			t.Fatal("unmatched scalar must be validated without affinity repair", test, rows, err)
		}
	}
	env, admin := projectUnitFixture()
	admin.Rows = nil
	if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone"}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("no Admin matches must not become Env-only fallback", rows, err)
	}
	env.Rows = nil
	if rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone"}); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty full source must be explicit empty", rows, err)
	}
}

type projectUnitCancellationContext struct {
	context.Context
	calls  int
	cancel int
}

func (c *projectUnitCancellationContext) Err() error {
	c.calls++
	if c.cancel > 0 && c.calls >= c.cancel {
		return context.Canceled
	}
	return nil
}

func TestProjectUnitCancellationEveryObservation(t *testing.T) {
	for _, none := range []bool{false, true} {
		env, admin := projectUnitFixture()
		run := func(ctx context.Context) (bool, error) {
			if none {
				rows, err := planLongVegetationProjectNoneUnits(ctx, env)
				return rows == nil, err
			}
			rows, err := planLongVegetationProjectSelectedUnits(ctx, env, admin, []string{"Zone", "Comment"})
			return rows == nil, err
		}
		if empty, err := run(nil); err == nil || !empty {
			t.Fatal("nil context accepted", none, empty, err)
		}
		probe := &projectUnitCancellationContext{Context: context.Background()}
		if empty, err := run(probe); err != nil || empty || probe.calls < 2 {
			t.Fatal("successful probe did not exercise observations", none, probe, err)
		}
		for step := 1; step <= probe.calls; step++ {
			ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: step}
			if empty, err := run(ctx); !errors.Is(err, context.Canceled) || !empty {
				t.Fatal("cancellation exposed partial plan", none, step, empty, err)
			}
		}
	}
}

func TestSelectedUnitSharedIdentityAndLabelGuards(t *testing.T) {
	reader := selectedUnitIdentityReader()
	if value, err := reader(vprocore.ProjectMetadataCell{Storage: "null"}); err != nil || value != nil {
		t.Fatal(value, err)
	}
	if value, err := reader(vprotest.MetadataText("")); err != nil || value == nil || *value != "" {
		t.Fatal(value, err)
	}
	if _, err := reader(vprotest.MetadataText("P")); err != nil {
		t.Fatal(err)
	}
	if _, err := reader(vprotest.MetadataText("p")); err == nil {
		t.Fatal("case-equivalent original identities accepted")
	}
	if _, err := selectedUnitIdentityReader()(vprotest.MetadataText("p")); err != nil {
		t.Fatal("independent projections share literal scope state", err)
	}
	empty, literal := "", " literal "
	for _, test := range []struct {
		values []*string
		want   vprocore.ProjectMetadataCell
	}{
		{nil, vprocore.ProjectMetadataCell{Storage: "null"}},
		{[]*string{nil, nil}, vprocore.ProjectMetadataCell{Storage: "null"}},
		{[]*string{nil, &empty}, vprotest.MetadataText("")},
		{[]*string{&literal, &empty, &literal}, vprotest.MetadataText(" literal  literal ")},
	} {
		got, err := selectedUnitLabel(test.values)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatal("shared ordered NULL/empty/literal label guard", got, test.want, err)
		}
	}
	bad := strings.Repeat("x", 256)
	if _, err := selectedUnitLabel([]*string{&bad}); err == nil {
		t.Fatal("shared bound guard accepted overlength")
	}
}
