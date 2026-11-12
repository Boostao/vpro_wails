package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
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

func twoPageEntryProjectChoice(t *testing.T, original *sourceform.SiviParentProjection, source int, value vprocore.ProjectMetadataCell) (sourceform.SIVIProjectChoices, sourceform.SiviProjectSelection) {
	t.Helper()
	choices := sourceform.SIVIProjectChoices{ContextID: original.ContextID, Project: original.Project,
		SourceOption: source, Source: "Env", Alias: "project", Table: original.Project + "_Metadata",
		Choices: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "ProjectID", DeclaredType: "TEXT"}, {Name: "ProjectTitle", DeclaredType: "TEXT"}},
			Rows: []vprocore.ProjectMetadataRow{
				{RowID: "-9223372036854775808", Cells: []vprocore.ProjectMetadataCell{vprocore.CloneSiteUnitCell(value), vprotest.MetadataText("")}},
				{RowID: "9007199254740993", Cells: []vprocore.ProjectMetadataCell{vprocore.CloneSiteUnitCell(value), {Storage: "null"}}},
			}}}
	if source == 2 {
		choices.Source, choices.Alias, choices.Table = "Master", "VMetaData", "ProjectMetaData"
	}
	target := sftest.SiviParentEdit(t, original, "ProjectID", value)
	selection := sourceform.SiviProjectSelection{ContextID: original.ContextID, ControlID: "form:" + original.Form + "/ProjectID",
		Table: target.Table, RowID: target.RowID, Expected: target.Expected,
		SourceOption: source, MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns:  append([]vprocore.ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: choices.Choices.Rows[1]}
	return choices, selection
}

func TestTwoPageEntryProjectAssignmentExactSourcePhysicalChoiceAndBounds(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		_, _, _, original, _ := twoPageWriteFixture(t, form, false)
		for _, source := range []int{1, 2} {
			for _, value := range []string{"  Literal MiXeD  ", strings.Repeat("\U0001f331", 15), strings.Repeat("x", 30)} {
				choices, selection := twoPageEntryProjectChoice(t, original, source, vprotest.MetadataText(value))
				plan, err := twopage.PlanTwoPageEntryProjectAssignment(context.Background(), original, choices, selection)
				expected := []sourceform.SiviParentScalarAssignment{{ContextID: original.ContextID, Table: original.EnvTable, RowID: original.Rows[0].Env.RowID,
					Column: "ProjectID", Before: sftest.SiviParentCell(t, original, "ProjectID"), After: vprotest.MetadataText(value), Value: value}}
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
				plan, err = twopage.PlanTwoPageEntryProjectAssignment(context.Background(), original, choices, selection)
				if err != nil || plan.MetadataOriginal.RowID != "-9223372036854775808" ||
					!reflect.DeepEqual(plan.MetadataOriginal.Cells[1], vprotest.MetadataText("")) {
					t.Fatal("minimum signed physical identity or distinct empty title changed", plan, err)
				}
			}
		}
	}
}

func TestTwoPageEntryProjectAssignmentOmitsUnchangedHistoricalInvalidValues(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		_, _, _, original, _ := twoPageWriteFixture(t, form, false)
		projectParent, err := twopage.TwoPageParentProjector(form)
		if err != nil {
			t.Fatal(err)
		}
		for _, historical := range []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText(""), vprotest.MetadataText(strings.Repeat("x", 31))} {
			env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{
				{RowID: original.Rows[0].Env.RowID, Cells: append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Env.Cells...)},
			}}
			for index, column := range env.Columns {
				if column.Name == "ProjectID" {
					env.Rows[0].Cells[index] = vprocore.CloneSiteUnitCell(historical)
				}
			}
			observed, err := projectParent(context.Background(), original.ContextID, original.Project, original.Plot, env,
				vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
			if err != nil {
				t.Fatal(err)
			}
			for _, source := range []int{1, 2} {
				choices, selection := twoPageEntryProjectChoice(t, observed, source, historical)
				plan, err := twopage.PlanTwoPageEntryProjectAssignment(context.Background(), observed, choices, selection)
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
		vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET ProjectID='old' WHERE PlotNumber='108050'`)
		original, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		for _, failure := range []string{"SIVI", "other variant", "other control", "source", "context", "alias", "table",
			"title", "column", "row", "physical parent", "original binding", "empty", "null", "unlisted", "overlength", "unicode"} {
			choices, selection := twoPageEntryProjectChoice(t, original, 1, vprotest.MetadataText("new"))
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
				selection.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{vprotest.MetadataText("new"), vprotest.MetadataText("changed title")}
			case "column":
				selection.MetadataColumns[0].DeclaredType = "VARCHAR"
			case "row":
				selection.MetadataOriginal.RowID = "09007199254740993"
			case "physical parent":
				selection.RowID = "-999"
			case "original binding":
				copy.EnvTable = "other_Env"
			case "empty":
				choices.Choices.Rows[1].Cells[0] = vprotest.MetadataText("")
				selection.MetadataOriginal = choices.Choices.Rows[1]
			case "null":
				choices.Choices.Rows[1].Cells[0] = vprocore.ProjectMetadataCell{Storage: "null"}
				selection.MetadataOriginal = choices.Choices.Rows[1]
				selection.Expected = vprotest.MetadataText("old")
			case "unlisted":
				selection.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{vprotest.MetadataText("unlisted"), {Storage: "null"}}
			case "overlength":
				choices.Choices.Rows[1].Cells[0] = vprotest.MetadataText(strings.Repeat("x", 31))
				selection.MetadataOriginal = choices.Choices.Rows[1]
			case "unicode":
				choices.Choices.Rows[1].Cells[0] = vprotest.MetadataText(string([]byte{0xff}))
				selection.MetadataOriginal = choices.Choices.Rows[1]
			}
			if plan, err := twopage.PlanTwoPageEntryProjectAssignment(context.Background(), &copy, choices, selection); err == nil || plan != nil {
				t.Fatal("source-bound selection drift produced a partial plan", form, failure, plan, err)
			}
		}
	}
	if plan, err := sourceform.PlanSourceProjectAssignment(context.Background(), "", "", "", "", vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataTable{},
		sourceform.SIVIProjectChoices{}, sourceform.SiviProjectSelection{}); err == nil || plan != nil {
		t.Fatal("generic shared planner accepted absent expected source control")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if plan, err := twopage.PlanTwoPageEntryProjectAssignment(cancelled, nil, sourceform.SIVIProjectChoices{}, sourceform.SiviProjectSelection{}); !errors.Is(err, context.Canceled) || plan != nil {
		t.Fatal("cancellation became absent-original failure", plan, err)
	}
}
