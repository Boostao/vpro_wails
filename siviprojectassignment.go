package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type siviProjectSelection struct {
	ContextID, ControlID, Table, RowID string
	Expected                           ProjectMetadataCell
	SourceOption                       int
	MetadataAlias, MetadataTable       string
	MetadataColumns                    []ProjectMetadataColumn
	MetadataOriginal                   ProjectMetadataRow
}

type siviProjectAssignmentPlan struct {
	Assignments      []siviParentScalarAssignment
	SourceOption     int
	MetadataAlias    string
	MetadataTable    string
	MetadataColumns  []ProjectMetadataColumn
	MetadataOriginal ProjectMetadataRow
}

func planSIVIProjectAssignment(ctx context.Context, contextID, project, plot string, env, admin ProjectMetadataTable,
	choices SIVIProjectChoices, selection siviProjectSelection) (*siviProjectAssignmentPlan, error) {
	return planSourceProjectAssignment(ctx, "form:frmSIVIsite/ProjectID", contextID, project, plot, env, admin, choices, selection)
}

func planSourceProjectAssignment(ctx context.Context, controlID, contextID, project, plot string, env, admin ProjectMetadataTable,
	choices SIVIProjectChoices, selection siviProjectSelection) (*siviProjectAssignmentPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if controlID == "" || choices.ContextID != contextID || choices.Project != project || selection.ContextID != contextID ||
		selection.ControlID != controlID || selection.SourceOption != choices.SourceOption ||
		selection.MetadataAlias != choices.Alias || selection.MetadataTable != choices.Table {
		return nil, errors.New("SIVI ProjectID selection changed its owned source, preference or control identity")
	}
	switch choices.SourceOption {
	case 1:
		if choices.Source != "Env" || choices.Alias != "project" || choices.Table != project+"_Metadata" {
			return nil, errors.New("SIVI ProjectID Env selection requires the exact project metadata source")
		}
	case 2:
		if choices.Source != "Master" || choices.Alias != "VMetaData" || !strings.EqualFold(choices.Table, "ProjectMetadata") {
			return nil, errors.New("SIVI ProjectID Master selection requires its observed physical metadata table")
		}
	default:
		return nil, errors.New("SIVI ProjectID selection requires source Env (1) or Master (2)")
	}
	if len(choices.Choices.Columns) != 2 {
		return nil, errors.New("SIVI ProjectID selection requires the two original projected choice columns")
	}
	if !reflect.DeepEqual(selection.MetadataColumns, choices.Choices.Columns) {
		return nil, errors.New("SIVI ProjectID expected metadata column names or declared types changed; reload before planning")
	}
	projected, err := projectSIVIProjectChoices(ctx, choices.Choices)
	if err != nil {
		return nil, err
	}
	var selected *ProjectMetadataRow
	for index := range projected.Rows {
		row := &projected.Rows[index]
		id, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != row.RowID {
			return nil, errors.New("SIVI ProjectID choices require exact signed physical metadata identities")
		}
		if row.RowID == selection.MetadataOriginal.RowID {
			selected = row
		}
	}
	if selected == nil || !reflect.DeepEqual(*selected, selection.MetadataOriginal) {
		return nil, errors.New("SIVI ProjectID selected physical definition or title changed; reload before planning")
	}
	value := selected.Cells[0]
	assignments, err := planSIVIParentCells(ctx, contextID, project, plot, env, admin, []siviParentScalarEdit{{
		ContextID: selection.ContextID, Table: selection.Table, RowID: selection.RowID,
		Column: "ProjectID", Expected: selection.Expected, Value: value,
	}}, map[string]string{"ProjectID": "Env"}, func(column string, cell ProjectMetadataCell) error {
		if cell.Storage != "text" || cell.Text == nil || *cell.Text == "" {
			return fmt.Errorf("SIVI %s new assignment requires an existing nonempty TEXT choice; NULL/empty completion paths remain unverified", column)
		}
		return validateSiteCodeText(column, cell.Text, 30)
	})
	if err != nil {
		return nil, err
	}
	return &siviProjectAssignmentPlan{
		Assignments: assignments, SourceOption: choices.SourceOption,
		MetadataAlias: choices.Alias, MetadataTable: choices.Table,
		MetadataColumns: append([]ProjectMetadataColumn{}, choices.Choices.Columns...),
		MetadataOriginal: ProjectMetadataRow{RowID: selected.RowID, Cells: []ProjectMetadataCell{
			cloneSiteUnitCell(selected.Cells[0]), cloneSiteUnitCell(selected.Cells[1]),
		}},
	}, nil
}
