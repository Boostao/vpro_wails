package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func twoPageEntryProjectChoice(t *testing.T, original *siviParentProjection, source int, value ProjectMetadataCell) (SIVIProjectChoices, siviProjectSelection) {
	t.Helper()
	choices := SIVIProjectChoices{ContextID: original.ContextID, Project: original.Project,
		SourceOption: source, Source: "Env", Alias: "project", Table: original.Project + "_Metadata",
		Choices: ProjectMetadataTable{Columns: []ProjectMetadataColumn{{"ProjectID", "TEXT"}, {"ProjectTitle", "TEXT"}},
			Rows: []ProjectMetadataRow{
				{"-9223372036854775808", []ProjectMetadataCell{cloneSiteUnitCell(value), metadataText("")}},
				{"9007199254740993", []ProjectMetadataCell{cloneSiteUnitCell(value), {Storage: "null"}}},
			}}}
	if source == 2 {
		choices.Source, choices.Alias, choices.Table = "Master", "VMetaData", "ProjectMetaData"
	}
	target := siviParentEdit(t, original, "ProjectID", value)
	selection := siviProjectSelection{ContextID: original.ContextID, ControlID: "form:" + original.Form + "/ProjectID",
		Table: target.Table, RowID: target.RowID, Expected: target.Expected,
		SourceOption: source, MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns:  append([]ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: choices.Choices.Rows[1]}
	return choices, selection
}

func TestTwoPageEntryProjectAssignmentExactSourcePhysicalChoiceAndBounds(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		_, _, _, original, _ := twoPageWriteFixture(t, form, false)
		for _, source := range []int{1, 2} {
			for _, value := range []string{"  Literal MiXeD  ", strings.Repeat("\U0001f331", 15), strings.Repeat("x", 30)} {
				choices, selection := twoPageEntryProjectChoice(t, original, source, metadataText(value))
				plan, err := planTwoPageEntryProjectAssignment(context.Background(), original, choices, selection)
				expected := []siviParentScalarAssignment{{original.ContextID, original.EnvTable, original.Rows[0].Env.RowID,
					"ProjectID", siviParentCell(t, original, "ProjectID"), metadataText(value), value}}
				if err != nil || plan == nil || !reflect.DeepEqual(plan.Assignments, expected) ||
					plan.SourceOption != source || plan.MetadataOriginal.RowID != "9007199254740993" ||
					plan.MetadataOriginal.Cells[1].Storage != "null" || plan.MetadataTable != choices.Table {
					t.Fatal("exact source/choice/title/literal/30 UTF16 assignment changed", form, source, plan, err)
				}
				*plan.Assignments[0].After.Text = "caller-owned change"
				*plan.MetadataOriginal.Cells[0].Text = "caller-owned change"
				if *choices.Choices.Rows[0].Cells[0].Text != value || *selection.MetadataOriginal.Cells[0].Text != value {
					t.Fatal("plan aliased original/duplicate metadata ownership")
				}
				selection.MetadataOriginal = choices.Choices.Rows[0]
				plan, err = planTwoPageEntryProjectAssignment(context.Background(), original, choices, selection)
				if err != nil || plan.MetadataOriginal.RowID != "-9223372036854775808" ||
					!reflect.DeepEqual(plan.MetadataOriginal.Cells[1], metadataText("")) {
					t.Fatal("minimum signed physical identity or distinct empty title changed", plan, err)
				}
			}
		}
	}
}

func TestTwoPageEntryProjectAssignmentOmitsUnchangedHistoricalInvalidValues(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		_, _, _, original, _ := twoPageWriteFixture(t, form, false)
		projectParent, err := twoPageParentProjector(form)
		if err != nil {
			t.Fatal(err)
		}
		for _, historical := range []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataText(strings.Repeat("x", 31))} {
			env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{
				{RowID: original.Rows[0].Env.RowID, Cells: append([]ProjectMetadataCell{}, original.Rows[0].Env.Cells...)},
			}}
			for index, column := range env.Columns {
				if column.Name == "ProjectID" {
					env.Rows[0].Cells[index] = cloneSiteUnitCell(historical)
				}
			}
			observed, err := projectParent(context.Background(), original.ContextID, original.Project, original.Plot, env,
				ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}})
			if err != nil {
				t.Fatal(err)
			}
			for _, source := range []int{1, 2} {
				choices, selection := twoPageEntryProjectChoice(t, observed, source, historical)
				plan, err := planTwoPageEntryProjectAssignment(context.Background(), observed, choices, selection)
				if err != nil || plan == nil || len(plan.Assignments) != 0 {
					t.Fatal("unchanged historical value was rejected or assigned", form, source, historical, plan, err)
				}
			}
		}
	}
}

func TestTwoPageEntryProjectAssignmentRejectsSourceIdentityAndMetadataDrift(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, _, original, _ := twoPageWriteFixture(t, form, false)
		mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET ProjectID='old' WHERE PlotNumber='108050'`)
		original, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		for _, failure := range []string{"SIVI", "other variant", "other control", "source", "context", "alias", "table",
			"title", "column", "row", "physical parent", "original binding", "empty", "null", "unlisted", "overlength", "unicode"} {
			choices, selection := twoPageEntryProjectChoice(t, original, 1, metadataText("new"))
			copy := *original
			switch failure {
			case "SIVI":
				selection.ControlID = "form:frmSIVIsite/ProjectID"
			case "other variant":
				other := "FS882-8x6XL"
				if form == other {
					other = "FS882-8x6XL-CHARS"
				}
				selection.ControlID = "form:" + other + "/ProjectID"
			case "other control":
				selection.ControlID = "form:" + form + "/optProjectID"
			case "source":
				selection.SourceOption = 2
			case "context":
				choices.ContextID = "foreign"
			case "alias":
				selection.MetadataAlias = "main"
			case "table":
				selection.MetadataTable = "other_Metadata"
			case "title":
				selection.MetadataOriginal.Cells = []ProjectMetadataCell{metadataText("new"), metadataText("changed title")}
			case "column":
				selection.MetadataColumns[0].DeclaredType = "VARCHAR"
			case "row":
				selection.MetadataOriginal.RowID = "09007199254740993"
			case "physical parent":
				selection.RowID = "-999"
			case "original binding":
				copy.EnvTable = "other_Env"
			case "empty":
				choices.Choices.Rows[1].Cells[0] = metadataText("")
				selection.MetadataOriginal = choices.Choices.Rows[1]
			case "null":
				choices.Choices.Rows[1].Cells[0] = ProjectMetadataCell{Storage: "null"}
				selection.MetadataOriginal = choices.Choices.Rows[1]
				selection.Expected = metadataText("old")
			case "unlisted":
				selection.MetadataOriginal.Cells = []ProjectMetadataCell{metadataText("unlisted"), {Storage: "null"}}
			case "overlength":
				choices.Choices.Rows[1].Cells[0] = metadataText(strings.Repeat("x", 31))
				selection.MetadataOriginal = choices.Choices.Rows[1]
			case "unicode":
				choices.Choices.Rows[1].Cells[0] = metadataText(string([]byte{0xff}))
				selection.MetadataOriginal = choices.Choices.Rows[1]
			}
			if plan, err := planTwoPageEntryProjectAssignment(context.Background(), &copy, choices, selection); err == nil || plan != nil {
				t.Fatal("source-bound selection drift produced a partial plan", form, failure, plan, err)
			}
		}
	}
	if plan, err := planSourceProjectAssignment(context.Background(), "", "", "", "", ProjectMetadataTable{}, ProjectMetadataTable{},
		SIVIProjectChoices{}, siviProjectSelection{}); err == nil || plan != nil {
		t.Fatal("generic shared planner accepted absent expected source control")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if plan, err := planTwoPageEntryProjectAssignment(cancelled, nil, SIVIProjectChoices{}, siviProjectSelection{}); !errors.Is(err, context.Canceled) || plan != nil {
		t.Fatal("cancellation became absent-original failure", plan, err)
	}
}
