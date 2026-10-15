package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectUnitFixture() (ProjectMetadataTable, ProjectMetadataTable) {
	env, admin, _ := selectedUnitFixture()
	env.Rows = append(env.Rows,
		ProjectMetadataRow{RowID: "5", Cells: []ProjectMetadataCell{metadataText("P"), metadataText("A2"), metadataText("bc"), metadataText("01"), metadataInteger("7"), metadataInteger("-1")}},
		ProjectMetadataRow{RowID: "6", Cells: []ProjectMetadataCell{metadataText("orphan"), metadataText("unmatched"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
		ProjectMetadataRow{RowID: "7", Cells: []ProjectMetadataCell{{Storage: "null"}, metadataText("null plot"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
		ProjectMetadataRow{RowID: "8", Cells: []ProjectMetadataCell{metadataText(""), metadataText("empty plot"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, {Storage: "null"}}},
	)
	admin.Rows = append(admin.Rows,
		ProjectMetadataRow{RowID: "16", Cells: []ProjectMetadataCell{{Storage: "null"}, metadataText("null admin")}},
		ProjectMetadataRow{RowID: "17", Cells: []ProjectMetadataCell{metadataText(""), metadataText("empty admin")}},
		ProjectMetadataRow{RowID: "18", Cells: []ProjectMetadataCell{metadataText("admin only"), metadataText("unmatched")}},
	)
	return env, admin
}

func TestProjectSelectedUnitsOriginalInnerJoin(t *testing.T) {
	env, admin := projectUnitFixture()
	before, err := json.Marshal([]ProjectMetadataTable{env, admin})
	if err != nil {
		t.Fatal(err)
	}
	got, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone", "Comment"})
	if err != nil {
		t.Fatal(err)
	}
	want := []vegetationProjectSelectedUnitOccurrence{
		{"1", "11", metadataText("P"), metadataText("A first ")},
		{"1", "12", metadataText("P"), metadataText("Asecond")},
		{"2", "13", metadataText("Q"), ProjectMetadataCell{Storage: "null"}},
		{"3", "14", metadataText("R"), metadataText("Abthird")},
		{"4", "15", metadataText("S"), metadataText("")},
		{"5", "11", metadataText("P"), metadataText("A2 first ")},
		{"5", "12", metadataText("P"), metadataText("A2second")},
		{"8", "17", metadataText(""), metadataText("empty plotempty admin")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("original Env/Admin complete fanout, INNER JOIN NULL/unmatched and literal empty/space differ", got, want)
	}
	after, err := json.Marshal([]ProjectMetadataTable{env, admin})
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
		{"1", metadataText("P"), metadataText("All plots")},
		{"2", metadataText("Q"), metadataText("All plots")},
		{"3", metadataText("R"), metadataText("All plots")},
		{"4", metadataText("S"), metadataText("All plots")},
		{"5", metadataText("P"), metadataText("All plots")},
		{"6", metadataText("orphan"), metadataText("All plots")},
		{"7", ProjectMetadataCell{Storage: "null"}, metadataText("All plots")},
		{"8", metadataText(""), metadataText("All plots")},
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
		env.Rows[0].Cells[1] = metadataText(test.text)
		rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, []string{"Zone"})
		if test.valid {
			if err != nil || *rows[0].SiteUnit.Text != test.text {
				t.Fatal("valid original literal label changed", test.text, rows, err)
			}
		} else if err == nil || rows != nil {
			t.Fatal("invalid new UTF16/NUL/Unicode label accepted or partial plan leaked", len(test.text), rows, err)
		}
	}
	env.Rows[0].Cells[1] = metadataText(strings.Repeat("x", 255))
	env.Rows[0].Cells[2] = metadataText("z")
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
	for _, mutate := range []func(*ProjectMetadataTable){
		func(e *ProjectMetadataTable) { e.Columns[0].Name = "missing" },
		func(e *ProjectMetadataTable) { e.Columns[1].Name = "PlotNumber" },
		func(e *ProjectMetadataTable) { e.Rows[0].RowID = "01" },
		func(e *ProjectMetadataTable) { e.Rows[1].RowID = "1" },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells = e.Rows[0].Cells[:1] },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataInteger("7") },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataText("p") },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataText("P\x00") },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataText("\u00e9") },
		func(e *ProjectMetadataTable) { e.Rows[0].Cells[0] = metadataText(string([]byte{0xff})) },
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
	for _, mutate := range []func(*ProjectMetadataTable, *ProjectMetadataTable){
		func(e, a *ProjectMetadataTable) { a.Columns[0].Name = "missing" },
		func(e, a *ProjectMetadataTable) { a.Rows[1].RowID = "11" },
		func(e, a *ProjectMetadataTable) { a.Rows[0].Cells = nil },
		func(e, a *ProjectMetadataTable) { a.Rows[0].Cells[0] = metadataText("p") },
		func(e, a *ProjectMetadataTable) { a.Rows[7].Cells[0] = metadataInteger("7") },
		func(e, a *ProjectMetadataTable) { e.Columns[1].DeclaredType = "REAL" },
		func(e, a *ProjectMetadataTable) { a.Columns[1].Name = "Zone" },
		func(e, a *ProjectMetadataTable) { e.Rows[5].Cells[1] = metadataText("bad\x00unmatched") },
		func(e, a *ProjectMetadataTable) { a.Rows[7].Cells[1] = metadataText(string([]byte{0xff})) },
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
		cell   ProjectMetadataCell
	}{
		{4, "Num", metadataInteger("2147483648")}, {4, "Num", metadataInteger("-2147483649")},
		{4, "Num", metadataText("7")}, {5, "Flag", metadataInteger("1")},
		{5, "Flag", metadataInteger("-2")}, {5, "Flag", metadataText("True")},
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
	if value, err := reader(ProjectMetadataCell{Storage: "null"}); err != nil || value != nil {
		t.Fatal(value, err)
	}
	if value, err := reader(metadataText("")); err != nil || value == nil || *value != "" {
		t.Fatal(value, err)
	}
	if _, err := reader(metadataText("P")); err != nil {
		t.Fatal(err)
	}
	if _, err := reader(metadataText("p")); err == nil {
		t.Fatal("case-equivalent original identities accepted")
	}
	if _, err := selectedUnitIdentityReader()(metadataText("p")); err != nil {
		t.Fatal("independent projections share literal scope state", err)
	}
	empty, literal := "", " literal "
	for _, test := range []struct {
		values []*string
		want   ProjectMetadataCell
	}{
		{nil, ProjectMetadataCell{Storage: "null"}},
		{[]*string{nil, nil}, ProjectMetadataCell{Storage: "null"}},
		{[]*string{nil, &empty}, metadataText("")},
		{[]*string{&literal, &empty, &literal}, metadataText(" literal  literal ")},
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
