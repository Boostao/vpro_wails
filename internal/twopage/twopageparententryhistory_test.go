package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func twoPageEntryProjectHistoryFixture(t *testing.T, form string, source int) sourceform.SiviParentHistory {
	t.Helper()
	service, state, _, _, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, source, 3, "old")
	original, err := ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	selection.ControlID = "form:" + form + "/ProjectID"
	var evidence *sourceform.SiviProjectAssignmentHistory
	hooks := sourceform.SiviProjectAssignmentWriteHooks(service, context.Background(), state.ContextID, "108050", selection)
	err = sourceform.WithSIVIParentWriteTransaction(context.Background(), service.Projects.Sqlite, "source-history fixture read",
		func(tx *sql.Tx, _ string) error {
			var err error
			evidence, err = sourceform.ReadSIVIProjectAssignmentEvidence(context.Background(), service.Projects.Sqlite, tx, state.ContextID, selection)
			return err
		}, sourceform.SiviParentWriteHooks{Prepare: hooks.Prepare})
	if err != nil {
		t.Fatal(err)
	}
	edits := []sourceform.SiviParentScalarEdit{
		sftest.SiviParentEdit(t, original, "ProjectID", selection.MetadataOriginal.Cells[0]),
		sftest.SiviParentEdit(t, original, "FieldNumber", vprotest.MetadataText("  Literal field  ")),
		sftest.SiviParentEdit(t, original, "SV_PolygonNumber", vprotest.MetadataText("  Literal polygon  ")),
		sftest.SiviParentEdit(t, original, "GIS_BGC", vprotest.MetadataText("  Literal GIS  ")),
	}
	assignments, err := PlanTwoPageEntryProjection(context.Background(), original, edits, false)
	if err != nil || len(assignments) != 4 {
		t.Fatal("independent mixed source-history fixture did not plan four actual cells", assignments, err)
	}
	committed := *original
	committed.Rows = append([]sourceform.SiviParentRow{}, original.Rows...)
	committed.Rows[0].Env.Cells = append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Env.Cells...)
	committed.Rows[0].Admin.Cells = append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Admin.Cells...)
	event := sourceform.SiviParentHistory{Original: original, Committed: &committed, ProjectAssignment: evidence, EntryPlan: edits}
	for _, assignment := range assignments {
		row, columns := &committed.Rows[0].Env, committed.EnvColumns
		if assignment.Table == original.AdminTable {
			row, columns = &committed.Rows[0].Admin, committed.AdminColumns
		}
		found := false
		for index, column := range columns {
			if column.Name == assignment.Column {
				row.Cells[index] = vprocore.CloneSiteUnitCell(assignment.After)
				found = true
			}
		}
		if !found {
			t.Fatal("independent fixture lost a physical changed column", assignment.Column)
		}
		event.Changes = append(event.Changes, sourceform.SiviParentHistoryChange{Table: assignment.Table, RowID: assignment.RowID,
			Column: assignment.Column, Before: assignment.Before, After: assignment.After})
	}
	return event
}

func TestTwoPageEntryMixedProjectHistoryExactSourceAndDomainIsolation(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, source := range []int{1, 2} {
			event := twoPageEntryProjectHistoryFixture(t, form, source)
			history, err := TwoPageEntryHistory(form, false)
			if err != nil {
				t.Fatal(err)
			}
			planned, err := history.Assignments(context.Background(), event, event.Original.Project, event.Original.Plot)
			if err != nil || len(planned) != 4 || !sourceform.TwoPageEntryProjectHistoryTable(history.Table) {
				t.Fatal("mixed exact-form/source history rejected physical provenance", form, source, planned, err)
			}
			for _, assignment := range planned {
				if !reflect.DeepEqual(assignment.After, sftest.SiviParentCell(t, event.Committed, assignment.Column)) {
					t.Fatal("mixed history lost independently committed literal cells", assignment.Column)
				}
			}
			if _, err := sourceform.SiviProjectAssignmentHistoryAssignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil {
				t.Fatal("mixed entry provenance leaked into standalone SIVI history")
			}
			for _, other := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				extra, err := TwoPageParentExtraHistory(other)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := extra.Assignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil ||
					sourceform.TwoPageEntryProjectHistoryTable(extra.Table) {
					t.Fatal("mixed ProjectID provenance leaked into additional-field source history", other, err)
				}
				if other != form {
					foreign, err := TwoPageEntryHistory(other, false)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := foreign.Assignments(context.Background(), event, event.Original.Project, event.Original.Plot); err == nil {
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
				var changed sourceform.SiviParentHistory
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
					changed.ProjectAssignment.Selection.MetadataOriginal.Cells[1] = vprotest.MetadataText("foreign title")
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
							changed.Committed.Rows[0].Env.Cells[index] = vprotest.SiviReal(7)
							found = true
						}
					}
					if !found {
						t.Fatal("independent unrelated committed-cell fixture is absent")
					}
				case "before drift":
					changed.ProjectAssignment.Selection.Expected = vprotest.MetadataText("different original")
				}
				if planned, err := history.Assignments(context.Background(), changed, event.Original.Project, event.Original.Plot); err == nil || planned != nil {
					t.Fatal("malformed mixed source provenance produced a partial history plan", form, source, failure, planned, err)
				}
			}
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if planned, err := sourceform.ValidateSourceProjectAssignmentHistory(cancelled, nil, sourceform.SiviParentHistoryChange{}, nil, ""); !errors.Is(err, context.Canceled) || planned != nil {
		t.Fatal("source history evidence ignored cancellation", planned, err)
	}
}
