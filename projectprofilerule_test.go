package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestProfileRuleDraftPhysicalBoundsAndSourceDependentAssignments(t *testing.T) {
	_, _, request := profileRunFixture(t)
	original := request.OriginalRules
	text := func(value string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: &value} }
	number := func(value string) ProjectMetadataCell {
		return ProjectMetadataCell{Storage: "integer", Integer: &value}
	}
	draft := func(changes ...ProjectMetadataChange) []profileRuleDraft {
		return []profileRuleDraft{{RowID: "3", Changes: changes}}
	}
	for _, change := range []ProjectMetadataChange{
		{Column: "Order", Value: number("-32768")}, {Column: "Order", Value: number("32767")},
		{Column: "Criteria", Value: text(strings.Repeat("a", 255))},
		{Column: "Criteria", Value: text(strings.Repeat("\U0001f600", 127) + "a")},
		{Column: "Criteria", Value: text("  O'Brien  ")},
		{Column: "Operator", Value: text("Not Like")},
		{Column: "Criteria", Value: ProjectMetadataCell{Storage: "null"}},
	} {
		_, assignments, err := prepareProfileRuleDrafts(original, draft(change))
		if err != nil || len(assignments) != 1 {
			t.Fatal("valid physical/literal assignment rejected or altered", change, assignments, err)
		}
	}
	for _, change := range []ProjectMetadataChange{
		{Column: "Order", Value: number("32768")}, {Column: "Order", Value: number("-32769")},
		{Column: "Order", Value: text("1")}, {Column: "Order", Value: number("1.0")},
		{Column: "Criteria", Value: text(strings.Repeat("a", 256))},
		{Column: "Criteria", Value: text(strings.Repeat("\U0001f600", 128))},
		{Column: "Criteria", Value: text(string([]byte{0xff}))},
		{Column: "Criteria", Value: number("3")},
		{Column: "Operator", Value: text("Like ")}, {Column: "Operator", Value: text(">=")},
		{Column: "Layer", Value: text("Any")},
		{Column: "PlotCount", Value: number("0")}, {Column: "Unavailable", Value: text("x")},
		{Column: "Table", Value: text("Veg")},
	} {
		if _, _, err := prepareProfileRuleDrafts(original, draft(change)); err == nil {
			t.Fatal("malformed/new overlength/unavailable/dependent assignment accepted", change)
		}
	}
	changes := []ProjectMetadataChange{{Column: "Table", Value: text("Veg")},
		{Column: "Field", Value: text("Species")}, {Column: "Layer", Value: text("SumB")},
		{Column: "Species", Value: text("ABIELAS")}}
	planned, assignments, err := prepareProfileRuleDrafts(original, draft(changes...))
	if err != nil || len(assignments) != 4 || reflect.DeepEqual(planned, original) {
		t.Fatal("explicit source Table/Field proposal not preserved", assignments, err)
	}
	if _, _, err := prepareProfileRuleDrafts(original, draft(changes[0], changes[0])); err == nil {
		t.Fatal("repeated assignment accepted")
	}
	if _, _, err := prepareProfileRuleDrafts(original, []profileRuleDraft{{RowID: "99999"}}); err == nil {
		t.Fatal("invented physical identity accepted")
	}
}

func TestProfileRuleDraftOmissionPreservesHistoricalInvalidCells(t *testing.T) {
	_, _, request := profileRunFixture(t)
	original := request.OriginalRules
	bad := strings.Repeat("historical", 40)
	original.Rows[0].Cells[6] = ProjectMetadataCell{Storage: "text", Text: &bad}
	original.Columns = append(original.Columns, ProjectMetadataColumn{Name: "HistoricalExtra", DeclaredType: ""})
	for index := range original.Rows {
		hex := "00ff"
		original.Rows[index].Cells = append(original.Rows[index].Cells, ProjectMetadataCell{Storage: "blob", BlobHex: &hex})
	}
	planned, assignments, err := prepareProfileRuleDrafts(original, []profileRuleDraft{{
		RowID: original.Rows[0].RowID, Changes: []ProjectMetadataChange{{Column: "Criteria", Value: original.Rows[0].Cells[6]}},
	}})
	if err != nil || len(assignments) != 0 || !reflect.DeepEqual(original, planned) {
		t.Fatal("unchanged historical invalid values were reassigned/rejected", planned, assignments, err)
	}
	value := "changed"
	if _, _, err := prepareProfileRuleDrafts(original, []profileRuleDraft{{
		RowID: original.Rows[0].RowID, Changes: []ProjectMetadataChange{{Column: "HistoricalExtra", Value: ProjectMetadataCell{Storage: "text", Text: &value}}},
	}}); err == nil {
		t.Fatal("historical unmapped column became editable")
	}
}

func TestProfileRuleSourceTableCaseDispatchPreservesLiteralStorage(t *testing.T) {
	_, _, request := profileRunFixture(t)
	text := func(value string) ProjectMetadataCell { return ProjectMetadataCell{Storage: "text", Text: &value} }
	for _, table := range []string{"Veg", "veg", "vEg", "Lump", "lump", "LUMP"} {
		field := "Species"
		if strings.EqualFold(table, "Lump") {
			field = "LumpCode"
		}
		draft := []profileRuleDraft{{RowID: "3", Changes: []ProjectMetadataChange{
			{Column: "Table", Value: text(table)}, {Column: "Field", Value: text(field)},
			{Column: "Layer", Value: text("Any")},
		}}}
		planned, _, err := prepareProfileRuleDrafts(request.OriginalRules, draft)
		if err != nil {
			t.Fatal("source Option Compare Database keyword branch unavailable", table, err)
		}
		if !reflect.DeepEqual(planned.Rows[0].Cells[1], text(table)) {
			t.Fatal("keyword dispatch rewrote literal stored Table", table, planned.Rows[0].Cells[1])
		}
		draft[0].Changes = draft[0].Changes[:1]
		if _, _, err := prepareProfileRuleDrafts(request.OriginalRules, draft); err == nil {
			t.Fatal("case variant bypassed explicit dependent Field assignment", table)
		}
	}
}
