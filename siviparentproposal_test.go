package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func siviProposalEdits(parent *siviParentProjection) ([]siviParentScalarEdit, []siviParentScalarEdit) {
	scalars, options := []siviParentScalarEdit{}, []siviParentScalarEdit{}
	for _, binding := range parent.Bindings {
		_, scalar := siviParentScalarOwners[binding.Binding]
		option := binding.Binding == "SV_StandAgeEstMeas" || binding.Binding == "SV_StandHeightEstMeas"
		if !scalar && !option {
			continue
		}
		row := parent.Rows[0].Env
		if binding.Table == parent.AdminTable {
			row = parent.Rows[0].Admin
		}
		value := siviReal(123.25)
		if binding.Binding == "SV_FloodPlain" {
			value = metadataInteger("-1")
		}
		if option {
			value = metadataText("2")
		}
		before := row.Cells[binding.Column]
		if reflect.DeepEqual(value, before) {
			value = ProjectMetadataCell{Storage: "null"}
		}
		edit := siviParentScalarEdit{
			ContextID: parent.ContextID, Table: binding.Table, RowID: row.RowID,
			Column: binding.Binding, Expected: cloneSiteUnitCell(before), Value: value,
		}
		if scalar {
			scalars = append(scalars, edit)
		} else {
			options = append(options, edit)
		}
	}
	return scalars, options
}

func assertSIVIProposalAssignments(t *testing.T, parent *siviParentProjection, assignments []siviParentScalarAssignment, optionDomain bool) {
	t.Helper()
	fields := []struct {
		column, suffix string
		after          ProjectMetadataCell
		execution      any
	}{
		{"SV_StandHeight", "Env", siviReal(123.25), float64(123.25)},
		{"SV_AhorizonDepth", "Env", siviReal(123.25), float64(123.25)},
		{"SV_GleyingMottlingCM", "Env", siviReal(123.25), float64(123.25)},
		{"SV_PercentCoarseFrags", "Env", siviReal(123.25), float64(123.25)},
		{"SV_SoilDepth", "Env", siviReal(123.25), float64(123.25)},
		{"StrataCoverTotal", "Admin", siviReal(123.25), float64(123.25)},
		{"SV_FloodPlain", "Env", metadataInteger("-1"), int64(-1)},
		{"SV_StandAgeEstMeas", "Env", metadataText("2"), "2"},
		{"SV_StandHeightEstMeas", "Env", metadataText("2"), "2"},
	}
	if optionDomain {
		fields = fields[7:]
	} else {
		fields = fields[:7]
	}
	expected := map[string]siviParentScalarAssignment{}
	for _, field := range fields {
		columns, row := parent.EnvColumns, parent.Rows[0].Env
		if field.suffix == "Admin" {
			columns, row = parent.AdminColumns, parent.Rows[0].Admin
		}
		found := false
		for index, column := range columns {
			if column.Name != field.column {
				continue
			}
			found = true
			before := row.Cells[index]
			after, execution := field.after, field.execution
			if reflect.DeepEqual(before, after) {
				after, execution = ProjectMetadataCell{Storage: "null"}, nil
			}
			expected[field.column] = siviParentScalarAssignment{
				ContextID: parent.ContextID, Table: parent.Project + "_" + field.suffix,
				RowID: row.RowID, Column: field.column, Before: before, After: after, Value: execution,
			}
		}
		if !found {
			t.Fatal("literal acceptance field missing", field.column)
		}
	}
	if len(assignments) != len(expected) {
		t.Fatal("literal assignment count changed", assignments, expected)
	}
	for _, assignment := range assignments {
		want, found := expected[assignment.Column]
		if !found || !reflect.DeepEqual(assignment, want) {
			t.Fatal("literal owned assignment payload changed", assignment, want)
		}
		delete(expected, assignment.Column)
	}
	if len(expected) != 0 {
		t.Fatal("literal source assignments were omitted", expected)
	}
}

func TestSIVIParentProposalOwnedDomainsSnapshotAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(t.TempDir(), "external # proposal.db")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				state, err = service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			scalars, options := siviProposalEdits(parent)
			if len(scalars) != 7 || len(options) != 2 {
				t.Fatal("proposal fixture lost source fields", scalars, options)
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, domain := range []string{"both", "scalar", "option", "unchanged"} {
				scalarInput, optionInput := append([]siviParentScalarEdit(nil), scalars...), append([]siviParentScalarEdit(nil), options...)
				switch domain {
				case "scalar":
					optionInput = nil
				case "option":
					scalarInput = nil
				case "unchanged":
					for i := range scalarInput {
						scalarInput[i].Value = scalarInput[i].Expected
					}
					for i := range optionInput {
						optionInput[i].Value = optionInput[i].Expected
					}
				}
				got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalarInput, optionInput)
				if err != nil || got == nil || !reflect.DeepEqual(got.Original, parent) || got.Scalars == nil || got.Options == nil {
					t.Fatal("owned original/domain proposal changed", domain, got, err)
				}
				expectedScalars, expectedOptions := len(scalarInput), len(optionInput)
				if domain == "unchanged" {
					expectedScalars, expectedOptions = 0, 0
				}
				if len(got.Scalars) != expectedScalars || len(got.Options) != expectedOptions {
					t.Fatal("source domains or unchanged history changed", domain, got)
				}
				if domain == "both" || domain == "scalar" {
					assertSIVIProposalAssignments(t, parent, got.Scalars, false)
				}
				if domain == "both" || domain == "option" {
					assertSIVIProposalAssignments(t, parent, got.Options, true)
				}
				if len(got.Options) > 0 {
					*got.Options[0].After.Text = "caller"
					if *options[0].Value.Text != "2" {
						t.Fatal("proposal shares input values")
					}
				}
				*got.Original.Rows[0].Env.Cells[0].Text = "caller"
				got.Original.EnvColumns[0].Name = "caller"
				again, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(again, parent) {
					t.Fatal("caller changed source or cached projection", again, err)
				}
			}
			assertProfileSUFiles(t, service, files)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(after, config) {
				t.Fatal("proposal changed YAML", err)
			}
		})
	}
}

func TestSIVIParentProposalRejectsInvalidTailAndStaleOwnership(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, kind := range []string{"empty", "option-tail", "scalar-tail", "wrong-domain", "duplicate", "context", "row", "expected", "missing-pair"} {
		t.Run(kind, func(t *testing.T) {
			a, b := append([]siviParentScalarEdit(nil), scalars...), append([]siviParentScalarEdit(nil), options...)
			id, plot := state.ContextID, "108050"
			switch kind {
			case "empty":
				a, b = nil, nil
			case "option-tail":
				b[len(b)-1].Value = metadataText(" 2")
			case "scalar-tail":
				a[len(a)-1].Value = metadataText("123")
			case "wrong-domain":
				a, b = b, a
			case "duplicate":
				b = append(b, b[0])
			case "context":
				id = "stale"
			case "row":
				b[len(b)-1].RowID = "-999"
			case "expected":
				b[len(b)-1].Expected = metadataText("stale")
			case "missing-pair":
				plot = "missing"
			}
			if got, err := service.prepareSIVIParentProposal(context.Background(), id, plot, a, b); err == nil || got != nil {
				t.Fatal("invalid request returned a partial or success-shaped proposal", kind, got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.prepareSIVIParentProposal(ctx, state.ContextID, "108050", scalars, options); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled proposal returned a snapshot", got, err)
	}
	got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options)
	if err != nil || got == nil || len(got.Scalars) != 7 || len(got.Options) != 2 {
		t.Fatal("rejected proposal discarded attachments", got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentProposalBlockedCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got, err := service.prepareSIVIParentProposal(ctx, state.ContextID, "108050", scalars, options); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked proposal returned partial data", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options)
	if err != nil || got == nil || !reflect.DeepEqual(got.Original, parent) {
		t.Fatal("cancelled proposal lost pinned attachments", got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentProposalChangedSourceHistoricalOmissionAndDuplicatePairs(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Env SET SV_StandHeight='historical',SV_FloodPlain=2,
		SV_StandAgeEstMeas='??',SV_StandHeightEstMeas='' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options); err == nil || got != nil {
		t.Fatal("stale observed values returned a proposal", got, err)
	}
	current, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options = siviProposalEdits(current)
	for i := range scalars {
		scalars[i].Value = scalars[i].Expected
	}
	for i := range options {
		options[i].Value = options[i].Expected
	}
	got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options)
	if err != nil || got == nil || len(got.Scalars) != 0 || len(got.Options) != 0 || !reflect.DeepEqual(got.Original, current) {
		t.Fatal("unchanged invalid history was lost or assigned", got, err)
	}
	assertProfileSUFiles(t, service, files)
	if _, err := db.Exec(`DROP INDEX uidx_Sample_Admin_PlotNumber;
		INSERT INTO Sample_Admin SELECT * FROM Sample_Admin WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	files = databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options); err == nil || got != nil {
		t.Fatal("multiple physical pairs authorized a proposal", got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentReadProjectionFailureAndLateCancellationKeepAttachments(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	sentinel := errors.New("rejected owned projection")
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		got, err := withContextPlotRequest(ctx, service, state.ContextID, func(plots *PlotService) (*siviParentProjection, error) {
			return withOwnedSIVIParentRead(ctx, plots, state.ContextID, "108050", func(original *siviParentProjection) (*siviParentProjection, error) {
				if cancelled {
					cancel()
					return original, nil
				}
				return original, sentinel
			})
		})
		cancel()
		expected := sentinel
		if cancelled {
			expected = context.Canceled
		}
		if got != nil || !errors.Is(err, expected) {
			t.Fatal("late failure returned a success-shaped snapshot", got, err)
		}
		retry, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
		if err != nil || !reflect.DeepEqual(retry, parent) {
			t.Fatal("projection rollback discarded pinned attachments", retry, err)
		}
	}
	assertProfileSUFiles(t, service, files)
}
