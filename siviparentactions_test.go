package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func siviActionFixture(t *testing.T, column string, before ProjectMetadataCell) (ProjectMetadataTable, ProjectMetadataTable, siviParentActionEdit) {
	t.Helper()
	env, admin := siviParentTables(t)
	table, name, control := &env, "Sample_Env", "form:frmSIVIsite/optSpeciesListComplete"
	if column == "PlotType" {
		table, name, control = &admin, "Sample_Admin", "form:frmSIVIsite/optPlotType"
	}
	found := false
	for index, field := range table.Columns {
		if field.Name == column {
			table.Rows[0].Cells[index] = before
			found = true
		}
	}
	if !found {
		t.Fatal("action fixture missing physical target", column)
	}
	return env, admin, siviParentActionEdit{
		ContextID: "owned", ControlID: control, Table: name, RowID: table.Rows[0].RowID,
		Expected: before,
	}
}

func TestSIVIParentActionSourceIdentitiesAndLiteralMappings(t *testing.T) {
	sources, err := siviParentActionSources()
	wantSources := map[string]siviParentActionSource{
		"form:frmSIVIsite/optPlotType":            {"optPlotType", "PlotType", "Admin", false},
		"form:frmSIVIsite/optSpeciesListComplete": {"optSpeciesListComplete", "SpeciesListComplete", "Env", true},
	}
	if err != nil || !reflect.DeepEqual(sources, wantSources) {
		t.Fatal("normal source group/target identities changed", sources, wantSources, err)
	}
	for _, action := range []struct {
		option int
		text   string
	}{{1, "Ground"}, {2, "Visual"}, {3, "Note"}, {4, "FS882"}, {5, "Other"}} {
		env, admin, edit := siviActionFixture(t, "PlotType", metadataText("historical"))
		edit.Option = &action.option
		got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit})
		want := []siviParentScalarAssignment{{"owned", "Sample_Admin", edit.RowID, "PlotType", metadataText("historical"), metadataText(action.text), action.text}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("selected option/caption replaced literal PlotType storage", action.option, got, want, err)
		}
	}
	env, admin, clear := siviActionFixture(t, "SpeciesListComplete", metadataInteger("2"))
	got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{clear})
	want := []siviParentScalarAssignment{{"owned", "Sample_Env", clear.RowID, "SpeciesListComplete", metadataInteger("2"), ProjectMetadataCell{Storage: "null"}, nil}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("explicit nullable implicit target changed", got, want, err)
	}
	for _, action := range []struct {
		option int
		stored string
		exec   int64
	}{{1, "-1", -1}, {2, "0", 0}} {
		env, admin, edit := siviActionFixture(t, "SpeciesListComplete", metadataInteger("2"))
		edit.Option = &action.option
		got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit})
		want := []siviParentScalarAssignment{{"owned", "Sample_Env", edit.RowID, "SpeciesListComplete", metadataInteger("2"), metadataInteger(action.stored), action.exec}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("source BOOLEAN was not exact Access-1/0", got, want, err)
		}
	}
}

func TestSIVIParentActionsUnchangedAndHistoricalCorrectionClones(t *testing.T) {
	for _, column := range []string{"PlotType", "SpeciesListComplete"} {
		option := 1
		valid := metadataText("Ground")
		if column == "SpeciesListComplete" {
			valid = metadataInteger("-1")
		}
		env, admin, edit := siviActionFixture(t, column, valid)
		edit.Option = &option
		got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged source option created an assignment/audit candidate", column, got, err)
		}
		for _, history := range []ProjectMetadataCell{
			metadataText(""), metadataText("Grnd"), metadataInteger("2"), siviReal(99), {Storage: "null"},
		} {
			env, admin, edit = siviActionFixture(t, column, history)
			edit.Option = &option
			got, err = planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit})
			var execution any = "Ground"
			if column == "SpeciesListComplete" {
				execution = int64(-1)
			}
			want := []siviParentScalarAssignment{{"owned", edit.Table, edit.RowID, column, history, valid, execution}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("selected correction changed literal historical payload", got, want, err)
			}
			if got[0].Before.Text != nil {
				*got[0].Before.Text = "caller"
				if *history.Text == "caller" || *edit.Expected.Text == "caller" {
					t.Fatal("action history shares caller ownership")
				}
			}
			if got[0].After.Text != nil {
				*got[0].After.Text = "caller"
				if *valid.Text != "Ground" {
					t.Fatal("action target shares source literal ownership")
				}
			}
			if got[0].After.Integer != nil {
				*got[0].After.Integer = "2"
				if *valid.Integer != "-1" {
					t.Fatal("action BOOLEAN shares source value ownership")
				}
			}
		}
	}
}

func TestSIVIParentActionsRejectUnknownOptionsAndForeignSources(t *testing.T) {
	for _, column := range []string{"PlotType", "SpeciesListComplete"} {
		env, admin, edit := siviActionFixture(t, column, ProjectMetadataCell{Storage: "null"})
		for _, option := range []int{-1, 0, 6, 99} {
			edit.Option = &option
			if got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit}); err == nil || got != nil {
				t.Fatal("unsupported action option returned a plan", column, option, got, err)
			}
		}
		if column == "PlotType" {
			edit.Option = nil
		} else {
			option := 3
			edit.Option = &option
		}
		if got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit}); err == nil || got != nil {
			t.Fatal("unverified PlotType clear/unsupported species Else path admitted", got, err)
		}
		option := 1
		edit.Option = &option
		for _, control := range []string{"", "optPlotType", "optSpeciesListComplete", "form:frmSIVIsiteCHARS/optPlotType", "form:frmSIVIsite/optProjectID"} {
			edit.ControlID = control
			if got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{edit}); err == nil || got != nil {
				t.Fatal("name/CHARS/preference group masqueraded as source action", got, err)
			}
		}
	}
}

func TestSIVIParentActionsCompletePayloadOwnershipAndCancellation(t *testing.T) {
	env, admin, plot := siviActionFixture(t, "PlotType", metadataText("old"))
	one, two := 1, 2
	plot.Option = &one
	species := siviParentActionEdit{"owned", "form:frmSIVIsite/optSpeciesListComplete", "Sample_Env", env.Rows[0].RowID, metadataInteger("2"), &two}
	for index, column := range env.Columns {
		if column.Name == "SpeciesListComplete" {
			env.Rows[0].Cells[index] = metadataInteger("2")
		}
	}
	edits := []siviParentActionEdit{plot, species}
	want := []siviParentScalarAssignment{
		{"owned", "Sample_Admin", plot.RowID, "PlotType", metadataText("old"), metadataText("Ground"), "Ground"},
		{"owned", "Sample_Env", species.RowID, "SpeciesListComplete", metadataInteger("2"), metadataInteger("0"), int64(0)},
	}
	got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("combined physical source action payload changed", got, want, err)
	}
	for _, kind := range []string{"context", "table", "row", "expected", "duplicate", "source", "option"} {
		t.Run(kind, func(t *testing.T) {
			tail := species
			switch kind {
			case "context":
				tail.ContextID = "stale"
			case "table":
				tail.Table = "Sample_Admin"
			case "row":
				tail.RowID = "-999"
			case "expected":
				tail.Expected = metadataInteger("-1")
			case "duplicate":
				tail = plot
			case "source":
				tail.ControlID = "optSpeciesListComplete"
			case "option":
				bad := 3
				tail.Option = &bad
			}
			if got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentActionEdit{plot, tail}); err == nil || got != nil {
				t.Fatal("invalid action tail returned partial output", kind, got, err)
			}
		})
	}
	direct := []siviParentScalarEdit{{"owned", "Sample_Env", species.RowID, "SpeciesListComplete", metadataInteger("2"), metadataInteger("0")}}
	owners := map[string]string{"SpeciesListComplete": "Env"}
	if got, err := planSIVIParentCells(context.Background(), "owned", "Sample", "108050", env, admin, direct, owners, func(string, ProjectMetadataCell) error { return nil }); err == nil || got != nil {
		t.Fatal("ordinary shared guard admitted an implicit action target", got, err)
	}
	for _, planner := range []func(context.Context, string, string, string, ProjectMetadataTable, ProjectMetadataTable, []siviParentScalarEdit) ([]siviParentScalarAssignment, error){
		planSIVIParentScalars, planSIVIParentOptions, planSIVIParentText, planSIVIParentCategorical,
	} {
		if got, err := planner(context.Background(), "owned", "Sample", "108050", env, admin, direct); err == nil || got != nil {
			t.Fatal("action extraction widened an ordinary domain", got, err)
		}
	}
	for _, empty := range []string{"context", "project", "edits"} {
		id, project, proposed := "owned", "Sample", edits
		switch empty {
		case "context":
			id = ""
		case "project":
			project = ""
		case "edits":
			proposed = nil
		}
		if got, err := planSIVIParentActions(context.Background(), id, project, "108050", env, admin, proposed); err == nil || got != nil {
			t.Fatal("anonymous/display initialization generated a source action", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSIVIParentActions(ctx, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled action planning returned output", got, err)
	}
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 10000}
	if _, err := planSIVIParentActions(probe, "owned", "Sample", "108050", env, admin, edits); err != nil {
		t.Fatal(err)
	}
	late := &vegetationCancelContext{Context: context.Background(), remaining: 10000 - probe.remaining - 1}
	if got, err := planSIVIParentActions(late, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight action planning returned partial output", got, err)
	}
	got, err = planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("source action retry changed exact payload", got, want, err)
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-8"
	if got, err := planSIVIParentActions(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
		t.Fatal("ambiguous source action pair returned assignments", got, err)
	}
}
