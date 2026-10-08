package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func twoPageEntryProjectHistoryFixture(t *testing.T, form string, source int) siviParentHistory {
	t.Helper()
	service, state, _, _, selection := siviProjectAssignmentWriteFixture(t, false, source, 3, "old")
	original, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	selection.ControlID = "form:" + form + "/ProjectID"
	var evidence *siviProjectAssignmentHistory
	hooks := service.siviProjectAssignmentWriteHooks(context.Background(), state.ContextID, "108050", selection)
	err = withSIVIParentWriteTransaction(context.Background(), service.projects.sqlite, "source-history fixture read",
		func(tx *sql.Tx, _ string) error {
			var err error
			evidence, err = readSIVIProjectAssignmentEvidence(context.Background(), service.projects.sqlite, tx, state.ContextID, selection)
			return err
		}, siviParentWriteHooks{prepare: hooks.prepare})
	if err != nil {
		t.Fatal(err)
	}
	edits := []siviParentScalarEdit{
		siviParentEdit(t, original, "ProjectID", selection.MetadataOriginal.Cells[0]),
		siviParentEdit(t, original, "FieldNumber", metadataText("  Literal field  ")),
		siviParentEdit(t, original, "SV_PolygonNumber", metadataText("  Literal polygon  ")),
		siviParentEdit(t, original, "GIS_BGC", metadataText("  Literal GIS  ")),
	}
	assignments, err := planTwoPageEntryProjection(context.Background(), original, edits, false)
	if err != nil || len(assignments) != 4 {
		t.Fatal("independent mixed source-history fixture did not plan four actual cells", assignments, err)
	}
	committed := *original
	committed.Rows = append([]siviParentRow{}, original.Rows...)
	committed.Rows[0].Env.Cells = append([]ProjectMetadataCell{}, original.Rows[0].Env.Cells...)
	committed.Rows[0].Admin.Cells = append([]ProjectMetadataCell{}, original.Rows[0].Admin.Cells...)
	event := siviParentHistory{Original: original, Committed: &committed, ProjectAssignment: evidence, EntryPlan: edits}
	for _, assignment := range assignments {
		row, columns := &committed.Rows[0].Env, committed.EnvColumns
		if assignment.Table == original.AdminTable {
			row, columns = &committed.Rows[0].Admin, committed.AdminColumns
		}
		found := false
		for index, column := range columns {
			if column.Name == assignment.Column {
				row.Cells[index] = cloneSiteUnitCell(assignment.After)
				found = true
			}
		}
		if !found {
			t.Fatal("independent fixture lost a physical changed column", assignment.Column)
		}
		event.Changes = append(event.Changes, siviParentHistoryChange{Table: assignment.Table, RowID: assignment.RowID,
			Column: assignment.Column, Before: assignment.Before, After: assignment.After})
	}
	return event
}

func TestTwoPageEntryMixedProjectHistoryExactSourceAndDomainIsolation(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, source := range []int{1, 2} {
			event := twoPageEntryProjectHistoryFixture(t, form, source)
			history, err := twoPageEntryHistory(form, false)
			if err != nil {
				t.Fatal(err)
			}
			planned, err := history.assignments(context.Background(), event, event.Original.Project, event.Original.Plot)
			if err != nil || len(planned) != 4 || !twoPageEntryProjectHistoryTable(history.table) {
				t.Fatal("mixed exact-form/source history rejected physical provenance", form, source, planned, err)
			}
			for _, assignment := range planned {
				if !reflect.DeepEqual(assignment.After, siviParentCell(t, event.Committed, assignment.Column)) {
					t.Fatal("mixed history lost independently committed literal cells", assignment.Column)
				}
			}
			if _, err := siviProjectAssignmentHistoryAssignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil {
				t.Fatal("mixed entry provenance leaked into standalone SIVI history")
			}
			for _, other := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				extra, err := twoPageParentExtraHistory(other)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := extra.assignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil ||
					twoPageEntryProjectHistoryTable(extra.table) {
					t.Fatal("mixed ProjectID provenance leaked into additional-field source history", other, err)
				}
				if other != form {
					foreign, err := twoPageEntryHistory(other, false)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := foreign.assignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil {
						t.Fatal("mixed ProjectID provenance leaked across normal/CHARS history")
					}
				}
			}
			for _, failure := range []string{"missing evidence", "missing full plan", "SIVI control", "foreign title", "changed schema", "source",
				"metadata alias", "repeated ProjectID", "unused evidence", "unrelated committed cell", "before drift"} {
				data, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				var changed siviParentHistory
				if err := json.Unmarshal(data, &changed); err != nil {
					t.Fatal(err)
				}
				switch failure {
				case "missing evidence":
					changed.ProjectAssignment = nil
				case "missing full plan":
					changed.EntryPlan = nil
				case "SIVI control":
					changed.ProjectAssignment.Selection.ControlID = "form:frmSIVIsite/ProjectID"
				case "foreign title":
					changed.ProjectAssignment.Selection.MetadataOriginal.Cells[1] = metadataText("foreign title")
				case "changed schema":
					changed.ProjectAssignment.SchemaSHA256 = "changed"
				case "source":
					changed.ProjectAssignment.Selection.SourceOption = 3 - source
				case "metadata alias":
					changed.ProjectAssignment.Selection.MetadataAlias = "main"
				case "repeated ProjectID":
					changed.Changes = append(changed.Changes, changed.Changes[0])
				case "unused evidence":
					for index, edit := range changed.EntryPlan {
						if edit.Column == "ProjectID" {
							changed.EntryPlan = append(changed.EntryPlan[:index], changed.EntryPlan[index+1:]...)
							break
						}
					}
					for index, change := range changed.Changes {
						if change.Column == "ProjectID" {
							changed.Changes = append(changed.Changes[:index], changed.Changes[index+1:]...)
							break
						}
					}
				case "unrelated committed cell":
					found := false
					for index, column := range changed.Committed.EnvColumns {
						if column.Name == "SV_StandHeight" {
							changed.Committed.Rows[0].Env.Cells[index] = siviReal(7)
							found = true
						}
					}
					if !found {
						t.Fatal("independent unrelated committed-cell fixture is absent")
					}
				case "before drift":
					changed.ProjectAssignment.Selection.Expected = metadataText("different original")
				}
				if planned, err := history.assignments(context.Background(), changed, event.Original.Project, event.Original.Plot); err == nil || planned != nil {
					t.Fatal("malformed mixed source provenance produced a partial history plan", form, source, failure, planned, err)
				}
			}
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if planned, err := validateSourceProjectAssignmentHistory(cancelled, nil, siviParentHistoryChange{}, nil, ""); !errors.Is(err, context.Canceled) || planned != nil {
		t.Fatal("source history evidence ignored cancellation", planned, err)
	}
}
