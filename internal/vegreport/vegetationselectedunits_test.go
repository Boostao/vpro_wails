package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func selectedUnitFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"},
		{Name: "SubZone", DeclaredType: "TEXT"}, {Name: "SiteSeries", DeclaredType: "TEXT"},
		{Name: "Num", DeclaredType: "INTEGER"}, {Name: "Flag", DeclaredType: "BOOLEAN"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("A"), vprotest.MetadataText("bc"), vprotest.MetadataText("01"), vprotest.MetadataInteger("7"), vprotest.MetadataInteger("-1")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Q"), {Storage: "null"}, {Storage: "null"}, {Storage: "null"}, vprotest.MetadataInteger("-2"), vprotest.MetadataInteger("0")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("R"), vprotest.MetadataText("Ab"), vprotest.MetadataText("c"), vprotest.MetadataText("01"), {Storage: "null"}, {Storage: "null"}}},
			{RowID: "4", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("S"), vprotest.MetadataText(""), vprotest.MetadataText(""), vprotest.MetadataText(""), vprotest.MetadataInteger("0"), vprotest.MetadataInteger("-1")}},
		}}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}, {Name: "Comment", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText(" first ")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("second")}},
			{RowID: "13", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Q"), {Storage: "null"}}},
			{RowID: "14", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("R"), vprotest.MetadataText("third")}},
			{RowID: "15", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("S"), vprotest.MetadataText("")}},
		}}
	su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "SiteUnit", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("old")}},
			{RowID: "22", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), {Storage: "null"}}},
			{RowID: "23", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Q"), vprotest.MetadataText("old")}},
			{RowID: "24", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("absent"), vprotest.MetadataText("orphan")}},
			{RowID: "25", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("null plot")}},
			{RowID: "26", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("R"), vprotest.MetadataText("old")}},
			{RowID: "27", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("S"), vprotest.MetadataText("old")}},
		}}
	return env, admin, su
}

func TestSelectedUnitSourceOrderedConcatenationLeftJoins(t *testing.T) {
	env, admin, su := selectedUnitFixture()
	got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "SubZone", "SiteSeries"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	type witness struct {
		member, env, admin string
		label              vprocore.ProjectMetadataCell
	}
	want := []witness{
		{"21", "1", "11", vprotest.MetadataText("Abc01")}, {"21", "1", "12", vprotest.MetadataText("Abc01")},
		{"22", "1", "11", vprotest.MetadataText("Abc01")}, {"22", "1", "12", vprotest.MetadataText("Abc01")},
		{"23", "2", "13", vprocore.ProjectMetadataCell{Storage: "null"}},
		{"24", "", "", vprocore.ProjectMetadataCell{Storage: "null"}}, {"25", "", "", vprocore.ProjectMetadataCell{Storage: "null"}},
		{"26", "3", "14", vprotest.MetadataText("Abc01")}, {"27", "4", "15", vprotest.MetadataText("")},
	}
	if len(got) != len(want) {
		t.Fatal("source LEFT JOIN multiplicity/NULL/orphans", got)
	}
	for i, expected := range want {
		row := got[i]
		if row.MembershipRowID != expected.member || !reflect.DeepEqual(row.SiteUnit, expected.label) ||
			(expected.env == "") != (row.EnvRowID == nil) || (expected.admin == "") != (row.AdminRowID == nil) ||
			row.EnvRowID != nil && *row.EnvRowID != expected.env || row.AdminRowID != nil && *row.AdminRowID != expected.admin {
			t.Fatal("independent original witnesses/labels differ", i, row, expected)
		}
	}
	*got[0].PlotNumber.Text = "changed"
	*got[0].SiteUnit.Text = "changed"
	*got[0].EnvRowID = "changed"
	if *su.Rows[0].Cells[0].Text != "P" || *env.Rows[0].Cells[1].Text != "A" ||
		*got[1].PlotNumber.Text != "P" || *got[1].SiteUnit.Text != "Abc01" || *got[1].EnvRowID != "1" {
		t.Fatal("derived result aliases original/peer")
	}
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"SiteSeries", "Zone", "SubZone"}, nil)
	if err != nil || *got[0].SiteUnit.Text != "01Abc" {
		t.Fatal("explicit selected-field order was changed", got, err)
	}
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "Comment"}, nil)
	if err != nil || *got[0].SiteUnit.Text != "A first " || *got[1].SiteUnit.Text != "Asecond" ||
		got[4].SiteUnit.Storage != "null" || *got[8].SiteUnit.Text != "" {
		t.Fatal("admin fields/whitespace/NULL-empty distinctions", got, err)
	}
}

func TestSelectedUnitScalarAndQualificationExistence(t *testing.T) {
	env, admin, su := selectedUnitFixture()
	got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Num", "Flag"}, nil)
	if err != nil || len(got) != 9 || *got[0].SiteUnit.Text != "7-1" || *got[4].SiteUnit.Text != "-20" ||
		got[7].SiteUnit.Storage != "null" || *got[8].SiteUnit.Text != "0-1" {
		t.Fatal("source Long/Boolean concatenation", got, err)
	}
	quality := vegetationQualitySelection{}
	for _, id := range []string{"11", "12", "11", "12"} {
		quality.Occurrences = append(quality.Occurrences, vegetationQualityOccurrence{
			Membership: reportcore.VegetationReportMembership{RowID: "21", PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText("old")},
			EnvRowID:   "1", AdminRowID: id,
		})
	}
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, &quality)
	if err != nil || len(got) != 4 || got[0].MembershipRowID != "21" || got[2].MembershipRowID != "22" {
		t.Fatal("qualified plot existence retains all prepared peers, not quality multiplicity/membership-only filter", got, err)
	}
	quality.Occurrences = nil
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, &quality)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty qualified scope must not fall back", got, err)
	}
	for _, mutate := range []func(*vegetationQualityOccurrence){
		func(q *vegetationQualityOccurrence) { q.Membership.RowID = "missing" },
		func(q *vegetationQualityOccurrence) { q.Membership.SiteUnit = vprotest.MetadataText("not original") },
		func(q *vegetationQualityOccurrence) { q.EnvRowID = "2" },
		func(q *vegetationQualityOccurrence) { q.AdminRowID = "13" },
	} {
		q := vegetationQualityOccurrence{Membership: reportcore.VegetationReportMembership{RowID: "21", PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText("old")},
			EnvRowID: "1", AdminRowID: "11"}
		mutate(&q)
		quality.Occurrences = []vegetationQualityOccurrence{q}
		if got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, &quality); err == nil || got != nil {
			t.Fatal("invalid original quality witness accepted", got, err)
		}
	}
}

func TestSelectedUnitBoundsAndRefusals(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{strings.Repeat("x", 255), true}, {strings.Repeat("x", 256), false},
		{strings.Repeat("\U0001f332", 127) + "x", true}, {strings.Repeat("\U0001f332", 128), false},
		{" literal Case ", true}, {"bad\x00value", false}, {string([]byte{0xff}), false},
	} {
		env, admin, su := selectedUnitFixture()
		env.Rows[0].Cells[1] = vprotest.MetadataText(test.value)
		got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, nil)
		if test.valid && (err != nil || *got[0].SiteUnit.Text != test.value) || !test.valid && (err == nil || got != nil) {
			t.Fatal("exact new label UTF16/malformed/NUL requirement", test.valid, len(test.value), got, err)
		}
	}
	for _, fields := range [][]string{nil, {"Zone", "zone"}, {"missing"}, {"[Zone]"}, {"Zone;SubZone"}} {
		env, admin, su := selectedUnitFixture()
		if got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, fields, nil); err == nil || got != nil {
			t.Fatal("invalid selected identifiers accepted", fields, got, err)
		}
	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable, *vprocore.ProjectMetadataTable){
		func(e, a, s *vprocore.ProjectMetadataTable) { e.Columns[1].DeclaredType = "REAL" },
		func(e, a, s *vprocore.ProjectMetadataTable) { e.Columns[1].DeclaredType = "TIMESTAMP" },
		func(e, a, s *vprocore.ProjectMetadataTable) { a.Columns[1].Name = "zone" },
		func(e, a, s *vprocore.ProjectMetadataTable) { s.Rows[1].RowID = s.Rows[0].RowID },
		func(e, a, s *vprocore.ProjectMetadataTable) { s.Rows[0].Cells = s.Rows[0].Cells[:1] },
		func(e, a, s *vprocore.ProjectMetadataTable) { s.Rows[0].Cells[0] = vprotest.MetadataInteger("1") },
		func(e, a, s *vprocore.ProjectMetadataTable) { s.Rows[0].Cells[0] = vprotest.MetadataText("p") },
		func(e, a, s *vprocore.ProjectMetadataTable) { s.Rows[0].Cells[0] = vprotest.MetadataText("\u00e9") },
	} {
		env, admin, su := selectedUnitFixture()
		mutate(&env, &admin, &su)
		if got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, nil); err == nil || got != nil {
			t.Fatal("unsupported schema/scope accepted", got, err)
		}
	}
	env, admin, su := selectedUnitFixture()
	for _, ctx := range []context.Context{nil, cancelledSelectedUnitContext()} {
		got, err := planLongVegetationSelectedUnits(ctx, env, admin, su, []string{"Zone"}, nil)
		if err == nil || got != nil || ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("missing/cancelled context", got, err)
		}
	}
}

func cancelledSelectedUnitContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestSelectedUnitParentMultiplicityAndCombinedBounds(t *testing.T) {
	env, admin, su := selectedUnitFixture()
	env.Rows = append(env.Rows, vprocore.ProjectMetadataRow{RowID: "5", Cells: []vprocore.ProjectMetadataCell{
		vprotest.MetadataText("P"), vprotest.MetadataText("a"), vprotest.MetadataText("bc"), vprotest.MetadataText("01"), vprotest.MetadataInteger("7"), vprotest.MetadataInteger("-1")}})
	got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "SubZone"}, nil)
	if err != nil || len(got) != 13 {
		t.Fatal("two SU*two Env*two Admin plus five other/orphan rows", got, err)
	}
	want := [][3]string{{"21", "1", "11"}, {"21", "1", "12"}, {"21", "5", "11"}, {"21", "5", "12"},
		{"22", "1", "11"}, {"22", "1", "12"}, {"22", "5", "11"}, {"22", "5", "12"}}
	for i, witness := range want {
		if got[i].MembershipRowID != witness[0] || *got[i].EnvRowID != witness[1] || *got[i].AdminRowID != witness[2] ||
			*got[i].SiteUnit.Text != []string{"Abc", "abc"}[(i%4)/2] {
			t.Fatal("complete original parent witnesses/case values", i, got[i])
		}
	}
	admin.Rows = admin.Rows[2:]
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, nil)
	if err != nil || len(got) != 7 || got[0].EnvRowID != nil || got[1].AdminRowID != nil || got[0].SiteUnit.Storage != "null" {
		t.Fatal("Env without Admin is not an UsysEnv row; outer SU survives", got, err)
	}
	env, admin, su = selectedUnitFixture()
	env.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 254))
	env.Rows[0].Cells[2] = vprotest.MetadataText("z")
	got, err = planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "SubZone"}, nil)
	if err != nil || *got[0].SiteUnit.Text != strings.Repeat("x", 254)+"z" {
		t.Fatal("exact concatenated255-unit label", got, err)
	}
	env.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 255))
	for _, quality := range []*vegetationQualitySelection{nil, {}, {Occurrences: []vegetationQualityOccurrence{
		{Membership: reportcore.VegetationReportMembership{RowID: "23", PlotNumber: vprotest.MetadataText("Q"), SiteUnit: vprotest.MetadataText("old")},
			EnvRowID: "2", AdminRowID: "13"},
	}}} {
		if got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "SubZone"}, quality); err == nil || got != nil {
			t.Fatal("build before prune must refuse256-unit label even on excluded plot", got, err)
		}
	}
	for _, test := range []struct {
		field string
		cell  vprocore.ProjectMetadataCell
	}{
		{"Num", vprotest.MetadataInteger("2147483648")}, {"Num", vprotest.MetadataText("7")},
		{"Flag", vprotest.MetadataInteger("1")}, {"Flag", vprotest.MetadataInteger("-2")}, {"Flag", vprotest.MetadataText("True")},
	} {
		env, admin, su = selectedUnitFixture()
		index := 4
		if test.field == "Flag" {
			index = 5
		}
		env.Rows[0].Cells[index] = test.cell
		if got, err := planLongVegetationSelectedUnits(context.Background(), env, admin, su, []string{test.field}, nil); err == nil || got != nil {
			t.Fatal("scalar affinity/Boolean/Long repaired", test.field, test.cell, got, err)
		}
	}
}

type selectedUnitCancellingContext struct {
	context.Context
	remaining int
}

func (c *selectedUnitCancellingContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestSelectedUnitCancellationDuringReadAndJoin(t *testing.T) {
	for _, steps := range []int{2, 7, 18, 23} {
		env, admin, su := selectedUnitFixture()
		ctx := &selectedUnitCancellingContext{Context: context.Background(), remaining: steps}
		got, err := planLongVegetationSelectedUnits(ctx, env, admin, su, []string{"Zone"}, nil)
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("mid-read/join cancellation leaked partial plan", steps, got, err)
		}
	}
}
