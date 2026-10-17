package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryReferenceText(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "text", Text: &value}
}

func summaryReferenceInteger(value int64) ProjectMetadataCell {
	text := strconv.FormatInt(value, 10)
	return ProjectMetadataCell{Storage: "integer", Integer: &text}
}

func TestSummaryEffectiveReferenceEnvironmentalMasterBoundary(t *testing.T) {
	master, _, _ := summaryReferenceFixture()
	filtered, err := summaryEffectiveReferenceEnvironmentMaster(t.Context(), master)
	if err != nil || len(filtered.Rows) != 6 {
		t.Fatal("effective environmental master did not remove only old physical attributes", filtered, err)
	}
	for _, row := range filtered.Rows {
		if row.RowID == "3" {
			t.Fatal("removed attribute remained in environmental input")
		}
	}
	for _, ctx := range []context.Context{nil, func() context.Context {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		return ctx
	}()} {
		result, err := summaryEffectiveReferenceEnvironmentMaster(ctx, master)
		if err == nil || !reflect.DeepEqual(result, ProjectMetadataTable{}) {
			t.Fatal("unavailable environmental source context returned partial success", result, err)
		}
	}
	for _, edit := range []func(*ProjectMetadataTable){
		func(table *ProjectMetadataTable) { table.Columns[5].Name = "Missing" },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells = nil },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[5] = summaryReferenceInteger(1) },
	} {
		invalid, _, _ := summaryReferenceFixture()
		edit(&invalid)
		result, err := summaryEffectiveReferenceEnvironmentMaster(t.Context(), invalid)
		if err == nil || !reflect.DeepEqual(result, ProjectMetadataTable{}) {
			t.Fatal("invalid environmental attribute authority returned partial success", result, err)
		}
	}
	if value, err := summaryVegetationMasterLongInteger(summaryReferenceInteger(math.MaxInt32)); err != nil || value == nil || *value != math.MaxInt32 {
		t.Fatal("explicit environmental LONG policy narrowed signed32", value, err)
	}
	if value, err := summaryVegetationMasterLongInteger(summaryReferenceInteger(math.MaxInt32 + 1)); err == nil || value != nil {
		t.Fatal("environmental LONG overflow returned a value", value, err)
	}
	quick, physical, su := summaryEnvironmentalLifeformFixture(t)
	unit := "mixed"
	if result, err := prepareSummaryEnvironmentalLifeformWithMasterInteger(t.Context(), "1", &unit, quick, physical, su, nil); err == nil || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
		t.Fatal("missing explicit environmental source policy returned a value", result, err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := prepareSummaryEnvironmentalLifeformWithMasterInteger(cancelled, "1", &unit, quick, physical, su, summaryVegetationMasterLongInteger); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, ProjectMetadataCell{}) {
		t.Fatal("cancelled environmental source policy returned a value", result, err)
	}
}

func summaryReferenceFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	null := ProjectMetadataCell{Storage: "null"}
	text, integer := summaryReferenceText, summaryReferenceInteger
	master := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Code"}, {Name: "ScientificName"}, {Name: "Lifeform"}, {Name: "EnglishName"},
		{Name: "Codetype"}, {Name: "IsAttribute"}, {Name: "CombinedEnglishName"},
	}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text("A1"), text("Scientific A"), integer(1), text("Common"), text("U"), integer(0), text("Guide")}},
		{RowID: "2", Cells: []ProjectMetadataCell{text(strings.Repeat("W", 255)), null, integer(math.MaxInt32), text(""), text(strings.Repeat("T", 255)), null, null}},
		{RowID: "3", Cells: []ProjectMetadataCell{text("OLD"), text("Removed"), integer(2), text("Old attribute"), text("U"), integer(-1), text("Old guide")}},
		{RowID: "4", Cells: []ProjectMetadataCell{text("SUB"), null, null, null, text("S"), integer(0), null}},
		{RowID: "5", Cells: []ProjectMetadataCell{text("NULLTYPE"), null, null, null, null, integer(0), null}},
		{RowID: "6", Cells: []ProjectMetadataCell{text("ATTR"), null, null, null, text("U"), integer(0), null}},
		{RowID: "7", Cells: []ProjectMetadataCell{text("A1"), text("Scientific A"), integer(1), text("Common"), text("U"), integer(0), text("Guide")}},
	}}
	personal := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Code"}, {Name: "ScientificName"}, {Name: "LifeForm"}, {Name: "EnglishName"}, {Name: "Codetype"},
	}, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{text("A1"), text("Scientific A"), integer(1), text("Common"), text("U")}},
		{RowID: "2", Cells: []ProjectMetadataCell{text("PB"), null, integer(math.MinInt16), text(""), text("U")}},
	}}
	descriptions := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "ShortName"}, {Name: "Definition"}, {Name: "IsAttribute"}, {Name: "Code"},
	}, Rows: []ProjectMetadataRow{
		{RowID: "11", Cells: []ProjectMetadataCell{text("A1"), text("Common"), integer(0), text("Ignored")}},
		{RowID: "12", Cells: []ProjectMetadataCell{text("ATTR"), null, integer(-1), null}},
		{RowID: "13", Cells: []ProjectMetadataCell{text(""), text(""), null, text("Not a fallback")}},
		{RowID: "14", Cells: []ProjectMetadataCell{null, text("Not admitted"), integer(-1), text("No fallback")}},
		{RowID: "15", Cells: []ProjectMetadataCell{text("A1"), text("Common"), integer(-1), null}},
	}}
	return master, personal, descriptions
}

func TestSummaryEffectiveReferenceProjectionAndIndependentOrigins(t *testing.T) {
	for _, guide := range []bool{false, true} {
		master, personal, descriptions := summaryReferenceFixture()
		before, err := json.Marshal([]ProjectMetadataTable{master, personal, descriptions})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions,
			SummaryVegetationReferenceOptions{UseGuideNames: guide})
		if err != nil {
			t.Fatal(err)
		}
		wantRows := 6
		if guide {
			wantRows = 7
		}
		expected := SummaryVegetationReferenceReview{
			MasterRows: 7, PersonalRows: 2, DescriptionRows: 5, RemovedMasterRows: 1,
			DerivedRows: 4, UnionRows: wantRows, ShowCommon: guide, UseGuideNames: guide,
		}
		if !reflect.DeepEqual(plan.Review, expected) || len(plan.References.Table.Rows) != wantRows {
			t.Fatal("independent source counts differ", plan.Review, expected)
		}
		find := func(code string, scientific *string) []ProjectMetadataRow {
			t.Helper()
			var rows []ProjectMetadataRow
			for _, row := range plan.References.Table.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == code && reflect.DeepEqual(row.Cells[1].Text, scientific) {
					rows = append(rows, row)
				}
			}
			return rows
		}
		for _, removed := range []string{"OLD", "SUB", "NULLTYPE", "Ignored", "No fallback"} {
			for _, row := range plan.References.Table.Rows {
				if row.Cells[0].Text != nil && *row.Cells[0].Text == removed {
					t.Fatal("excluded row or metadata fallback invented", removed)
				}
			}
		}
		scientific := "Scientific A"
		ordinary := find("A1", &scientific)
		if len(ordinary) != 1 && !guide || len(ordinary) != 2 && guide {
			t.Fatal("guide selection did not precede whole-row UNION", ordinary)
		}
		if !guide {
			origins := plan.References.Origins[ordinary[0].RowID]
			want := []vegetationLifeformReferenceOrigin{{"USysAllSpecs", "1"}, {"USysAllSpecs", "7"}, {"USysUserSpp", "1"}}
			if !reflect.DeepEqual(origins, want) {
				t.Fatal("physical duplicate ownership lost", origins)
			}
		} else if *ordinary[0].Cells[3].Text != "Guide" || *ordinary[1].Cells[3].Text != "Common" {
			t.Fatal("guide did not affect only master EnglishName", ordinary)
		}
		derived := find("A1", nil)
		if len(derived) != 1 || derived[0].Cells[2].Storage != "null" ||
			!reflect.DeepEqual(plan.References.Origins[derived[0].RowID],
				[]vegetationLifeformReferenceOrigin{{"USysSppAttributeDescription", "11"}, {"USysSppAttributeDescription", "15"}}) {
			t.Fatal("derived attributes invented physical master ownership or Lifeform", derived)
		}
		if guide && derived[0].Cells[3].Storage != "null" ||
			!guide && (derived[0].Cells[3].Text == nil || *derived[0].Cells[3].Text != "Common") {
			t.Fatal("guide attribute default CombinedEnglishName was replaced with Definition")
		}
		attribute := find("ATTR", nil)
		if len(attribute) != 1 || !reflect.DeepEqual(plan.References.Origins[attribute[0].RowID],
			[]vegetationLifeformReferenceOrigin{{"USysAllSpecs", "6"}, {"USysSppAttributeDescription", "12"}}) {
			t.Fatal("physical/derived equal-row UNION lost distinguishable origins", attribute)
		}
		empty := find("", nil)
		if len(empty) != 1 || !guide && (empty[0].Cells[3].Text == nil || *empty[0].Cells[3].Text != "") {
			t.Fatal("empty metadata identity/value changed into NULL or omitted", empty)
		}
		after, err := json.Marshal([]ProjectMetadataTable{master, personal, descriptions})
		if err != nil || string(before) != string(after) {
			t.Fatal("projection changed complete physical inputs", err)
		}
		*plan.References.Table.Rows[0].Cells[0].Text = "changed output"
		if *master.Rows[0].Cells[0].Text != "A1" {
			t.Fatal("projected cells alias physical source")
		}
	}
}

func TestSummaryEffectiveReferenceSourcePoliciesAndFailureShape(t *testing.T) {
	for _, item := range []struct {
		name   string
		edit   func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable)
		guide  bool
		reject bool
	}{
		{"master-long-min", func(m, p, d *ProjectMetadataTable) { m.Rows[0].Cells[2] = summaryReferenceInteger(math.MinInt32) }, false, false},
		{"master-long-overflow", func(m, p, d *ProjectMetadataTable) { m.Rows[0].Cells[2] = summaryReferenceInteger(math.MaxInt32 + 1) }, false, true},
		{"master-long-underflow", func(m, p, d *ProjectMetadataTable) { m.Rows[0].Cells[2] = summaryReferenceInteger(math.MinInt32 - 1) }, false, true},
		{"personal-integer-max", func(m, p, d *ProjectMetadataTable) { p.Rows[0].Cells[2] = summaryReferenceInteger(math.MaxInt16) }, false, false},
		{"personal-overflow", func(m, p, d *ProjectMetadataTable) { p.Rows[0].Cells[2] = summaryReferenceInteger(math.MaxInt16 + 1) }, false, true},
		{"personal-underflow", func(m, p, d *ProjectMetadataTable) { p.Rows[0].Cells[2] = summaryReferenceInteger(math.MinInt16 - 1) }, false, true},
		{"master-wide-code", func(m, p, d *ProjectMetadataTable) {
			m.Rows[0].Cells[0] = summaryReferenceText(strings.Repeat("C", 255))
		}, false, false},
		{"master-overlength-code", func(m, p, d *ProjectMetadataTable) {
			m.Rows[0].Cells[0] = summaryReferenceText(strings.Repeat("C", 256))
		}, false, true},
		{"personal-wide-code", func(m, p, d *ProjectMetadataTable) { p.Rows[0].Cells[0] = summaryReferenceText("123456789") }, false, true},
		{"personal-null-code", func(m, p, d *ProjectMetadataTable) { p.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"} }, false, true},
		{"master-boolean-one", func(m, p, d *ProjectMetadataTable) { m.Rows[0].Cells[5] = summaryReferenceInteger(1) }, false, true},
		{"master-numeric-coercion", func(m, p, d *ProjectMetadataTable) { m.Rows[0].Cells[2] = summaryReferenceText("1") }, false, true},
		{"utf16-boundary", func(m, p, d *ProjectMetadataTable) {
			d.Rows[0].Cells[0] = summaryReferenceText(strings.Repeat("\U0001F332", 127) + "X")
		}, false, false},
		{"utf16-overlength", func(m, p, d *ProjectMetadataTable) {
			d.Rows[0].Cells[0] = summaryReferenceText(strings.Repeat("\U0001F332", 128))
		}, false, true},
		{"malformed-unused-master", func(m, p, d *ProjectMetadataTable) { m.Rows[4].Cells[3] = summaryReferenceText(string([]byte{0xff})) }, false, true},
		{"malformed-unadmitted-metadata", func(m, p, d *ProjectMetadataTable) { d.Rows[3].Cells[1] = summaryReferenceText(string([]byte{0xff})) }, false, true},
		{"missing-guide-unused", func(m, p, d *ProjectMetadataTable) { m.Columns[6].Name = "Unavailable" }, false, false},
		{"missing-guide-required", func(m, p, d *ProjectMetadataTable) { m.Columns[6].Name = "Unavailable" }, true, true},
		{"partially-renamed", func(m, p, d *ProjectMetadataTable) { m.Columns[6].Name = "JustEnglishName" }, false, true},
		{"ambiguous-personal-spelling", func(m, p, d *ProjectMetadataTable) {
			p.Columns = append(p.Columns, ProjectMetadataColumn{Name: "Lifeform"})
			for i := range p.Rows {
				p.Rows[i].Cells = append(p.Rows[i].Cells, summaryReferenceInteger(1))
			}
		}, false, true},
		{"missing-description-column", func(m, p, d *ProjectMetadataTable) { d.Columns[0].Name = "Unavailable" }, false, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			master, personal, descriptions := summaryReferenceFixture()
			item.edit(&master, &personal, &descriptions)
			got, err := prepareSummaryVegetationReferences(t.Context(), master, personal, descriptions,
				SummaryVegetationReferenceOptions{UseGuideNames: item.guide})
			if (err != nil) != item.reject || err != nil && !reflect.DeepEqual(got, summaryVegetationReferencePlan{}) {
				t.Fatal("source-policy admission or zero failure shape differs", err, got.Review)
			}
		})
	}
	master, personal, descriptions := summaryReferenceFixture()
	for _, ctx := range []context.Context{nil, func() context.Context {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		return ctx
	}()} {
		got, err := prepareSummaryVegetationReferences(ctx, master, personal, descriptions, SummaryVegetationReferenceOptions{})
		if err == nil || !reflect.DeepEqual(got, summaryVegetationReferencePlan{}) {
			t.Fatal("unavailable/cancelled context returned partial projection", err)
		}
	}
	if _, err := prepareVegetationLifeformReferences(t.Context(), master, personal); err == nil {
		t.Fatal("legacy reference reader was implicitly widened to accept master signed32")
	}
}
