package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviProjectSelectionFixture(t *testing.T, source int, before, value ProjectMetadataCell) (ProjectMetadataTable, ProjectMetadataTable, SIVIProjectChoices, siviProjectSelection) {
	t.Helper()
	env, admin := siviParentTables(t)
	for index, field := range env.Columns {
		if field.Name == "ProjectID" {
			env.Rows[0].Cells[index] = cloneSiteUnitCell(before)
		}
	}
	choices := SIVIProjectChoices{ContextID: "owned", Project: "Sample", SourceOption: source,
		Source: "Env", Alias: "project", Table: "Sample_Metadata", Choices: ProjectMetadataTable{
			Columns: []ProjectMetadataColumn{{"ProjectID", "TEXT"}, {"ProjectTitle", "TEXT"}},
			Rows: []ProjectMetadataRow{
				{"-9223372036854775808", []ProjectMetadataCell{cloneSiteUnitCell(value), metadataText("")}},
				{"9007199254740993", []ProjectMetadataCell{cloneSiteUnitCell(value), {Storage: "null"}}},
			},
		}}
	if source == 2 {
		choices.Source, choices.Alias, choices.Table = "Master", "VMetaData", "ProjectMetaData"
	}
	selection := siviProjectSelection{ContextID: "owned", ControlID: "form:frmSIVIsite/ProjectID",
		Table: "Sample_Env", RowID: env.Rows[0].RowID, Expected: cloneSiteUnitCell(before),
		SourceOption: source, MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns:  append([]ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: choices.Choices.Rows[1]}
	return env, admin, choices, selection
}

func TestSIVIProjectAssignmentPlansExplicitPhysicalDuplicateWithoutNormalization(t *testing.T) {
	for _, source := range []int{1, 2} {
		value := "  Literal 🌱  "
		env, admin, choices, selection := siviProjectSelectionFixture(t, source, metadataText("old"), metadataText(value))
		plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection)
		want := []siviParentScalarAssignment{{"owned", "Sample_Env", env.Rows[0].RowID, "ProjectID", metadataText("old"), metadataText(value), value}}
		if err != nil || !reflect.DeepEqual(plan.Assignments, want) || plan.MetadataOriginal.RowID != "9007199254740993" ||
			plan.MetadataOriginal.Cells[1].Storage != "null" || plan.MetadataTable != choices.Table || plan.SourceOption != source {
			t.Fatal("physical duplicate/title provenance or literal assignment changed", plan, err)
		}
		*plan.Assignments[0].After.Text = "changed"
		*plan.MetadataOriginal.Cells[0].Text = "changed"
		plan.MetadataColumns[0].DeclaredType = "caller"
		if *selection.MetadataOriginal.Cells[0].Text != value || *choices.Choices.Rows[0].Cells[0].Text != value {
			t.Fatal("caller changed source metadata or another duplicate")
		}
		if selection.MetadataColumns[0].DeclaredType != "TEXT" || choices.Choices.Columns[0].DeclaredType != "TEXT" {
			t.Fatal("caller changed expected/source column provenance")
		}
	}
}

func TestSIVIProjectAssignmentExactUTF16BoundaryAndHistoricalNoops(t *testing.T) {
	for _, value := range []string{strings.Repeat("🌱", 15), strings.Repeat("x", 30)} {
		env, admin, choices, selection := siviProjectSelectionFixture(t, 1, metadataText("old"), metadataText(value))
		if plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection); err != nil || len(plan.Assignments) != 1 {
			t.Fatal("exact30-unit ProjectID rejected", plan, err)
		}
		choices.Choices.Rows[1].Cells[0] = metadataText(value + "x")
		selection.MetadataOriginal = choices.Choices.Rows[1]
		if plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection); err == nil || plan != nil {
			t.Fatal("31-unit ProjectID produced a partial plan", plan, err)
		}
	}
	for _, value := range []ProjectMetadataCell{metadataText(strings.Repeat("x", 31)), metadataText(""), {Storage: "null"}, metadataInteger("2")} {
		env, admin, choices, selection := siviProjectSelectionFixture(t, 1, value, value)
		if plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection); err != nil || len(plan.Assignments) != 0 {
			t.Fatal("unchanged historical assignment was not omitted", plan, err)
		}
	}
}

func TestSIVIProjectAssignmentRejectsForeignDriftAndUnavailableCompletion(t *testing.T) {
	for _, mutation := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *SIVIProjectChoices, *siviProjectSelection){
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataColumns = nil
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataColumns[0].DeclaredType = "VARCHAR"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataColumns[0].Name = "projectid"
		},
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) {
			c.Choices.Columns[1].DeclaredType = "VARCHAR"
		},
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) {
			c.ContextID = "foreign"
		},
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) { c.SourceOption = 2 },
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) { c.Source = "Master" },
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.ControlID = "form:frmSIVIsite/optProjectID"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataTable = "Other_Metadata"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataAlias = "VMetaData"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataOriginal.RowID = "1"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.MetadataOriginal.Cells = []ProjectMetadataCell{metadataText("new"), metadataText("")}
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) { s.RowID = "1" },
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.Table = "Sample_Admin"
		},
		func(_, _ *ProjectMetadataTable, _ *SIVIProjectChoices, s *siviProjectSelection) {
			s.Expected = metadataText("stale")
		},
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) {
			c.Choices.Rows = append(c.Choices.Rows, c.Choices.Rows[0])
		},
		func(_, _ *ProjectMetadataTable, c *SIVIProjectChoices, _ *siviProjectSelection) {
			c.Choices.Rows[0].RowID = "+1"
		},
		func(_, a *ProjectMetadataTable, _ *SIVIProjectChoices, _ *siviProjectSelection) {
			a.Rows = append(a.Rows, a.Rows[0])
		},
	} {
		env, admin, choices, selection := siviProjectSelectionFixture(t, 1, metadataText("old"), metadataText("new"))
		mutation(&env, &admin, &choices, &selection)
		if plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection); err == nil || plan != nil {
			t.Fatal("foreign/changed metadata or parent produced a plan", plan, err)
		}
	}
	for _, value := range []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataInteger("2"), metadataText(string([]byte{0xff}))} {
		env, admin, choices, selection := siviProjectSelectionFixture(t, 1, metadataText("old"), value)
		if plan, err := planSIVIProjectAssignment(context.Background(), "owned", "Sample", "108050", env, admin, choices, selection); err == nil || plan != nil {
			t.Fatal("new NULL/empty/non-TEXT/malformed choice was completed implicitly", plan, err)
		}
	}
	env, admin, choices, selection := siviProjectSelectionFixture(t, 1, metadataText("old"), metadataText("new"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if plan, err := planSIVIProjectAssignment(ctx, "owned", "Sample", "108050", env, admin, choices, selection); !errors.Is(err, context.Canceled) || plan != nil {
		t.Fatal("pre-cancelled selection returned a plan", plan, err)
	}
}
