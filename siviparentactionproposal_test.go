package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func siviActionProposalEdits(parent *siviParentProjection) []siviParentActionEdit {
	edits := []siviParentActionEdit{}
	for _, source := range []struct {
		control, column, table string
		row                    ProjectMetadataRow
		columns                []ProjectMetadataColumn
	}{
		{"form:frmSIVIsite/optPlotType", "PlotType", parent.AdminTable, parent.Rows[0].Admin, parent.AdminColumns},
		{"form:frmSIVIsite/optSpeciesListComplete", "SpeciesListComplete", parent.EnvTable, parent.Rows[0].Env, parent.EnvColumns},
	} {
		for index, column := range source.columns {
			if column.Name == source.column {
				option := 1
				edits = append(edits, siviParentActionEdit{
					ContextID: parent.ContextID, ControlID: source.control, Table: source.table,
					RowID: source.row.RowID, Expected: cloneSiteUnitCell(source.row.Cells[index]), Option: &option,
				})
			}
		}
	}
	return edits
}

func TestSIVIParentActionProposalSixteenFieldsAndLegacyCompatibility(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # action proposal.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
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
			db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`UPDATE Sample_Env SET SV_PolygonNumber='original polygon',
				SV_CanopyComposition='original canopy',SnowCoverregime='S',
				SV_RootZoneTexture='original texture',SV_AhorizonType='Ah',SpeciesListComplete=2
				WHERE PlotNumber='108050'; UPDATE Sample_Admin SET PlotType='Other' WHERE Plot='108050'`)
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			scalars, options := siviProposalEdits(parent)
			text, categorical, actions := siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent), siviActionProposalEdits(parent)
			if len(text) != 2 || len(categorical) != 3 || len(actions) != 2 {
				t.Fatal("owned source action fixture lost targets")
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for mask := 1; mask < 32; mask++ {
				var a, b, c, d []siviParentScalarEdit
				var e []siviParentActionEdit
				if mask&1 != 0 {
					a = scalars
				}
				if mask&2 != 0 {
					b = options
				}
				if mask&4 != 0 {
					c = text
				}
				if mask&8 != 0 {
					d = categorical
				}
				if mask&16 != 0 {
					e = actions
				}
				got, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", a, b, c, d, e)
				if err != nil || got == nil || !reflect.DeepEqual(got.Original, parent) ||
					got.Scalars == nil || got.Options == nil || got.Text == nil || got.Categorical == nil || got.Actions == nil {
					t.Fatal("complete original/action proposal changed", mask, got, err)
				}
				if len(a) > 0 {
					assertSIVIProposalAssignments(t, parent, got.Scalars, false)
				} else if len(got.Scalars) != 0 {
					t.Fatal("absent scalar domain invented assignments", mask, got)
				}
				if len(b) > 0 {
					assertSIVIProposalAssignments(t, parent, got.Options, true)
				} else if len(got.Options) != 0 {
					t.Fatal("absent option domain invented assignments", mask, got)
				}
				wantText, wantCategorical, wantActions := []siviParentScalarAssignment{}, []siviParentScalarAssignment{}, []siviParentScalarAssignment{}
				if len(c) > 0 {
					wantText = []siviParentScalarAssignment{
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_PolygonNumber", metadataText("original polygon"), metadataText("  New Polygon  "), "  New Polygon  "},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_CanopyComposition", metadataText("original canopy"), metadataText("Literal canopy"), "Literal canopy"},
					}
				}
				if len(d) > 0 {
					wantCategorical = []siviParentScalarAssignment{
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SnowCoverregime", metadataText("S"), metadataText("Z"), "Z"},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_RootZoneTexture", metadataText("original texture"), metadataText("  MiXeD \t "), "  MiXeD \t "},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_AhorizonType", metadataText("Ah"), metadataText(""), ""},
					}
				}
				if len(e) > 0 {
					wantActions = []siviParentScalarAssignment{
						{state.ContextID, "Sample_Admin", parent.Rows[0].Admin.RowID, "PlotType", metadataText("Other"), metadataText("Ground"), "Ground"},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SpeciesListComplete", metadataInteger("2"), metadataInteger("-1"), int64(-1)},
					}
				}
				if !reflect.DeepEqual(got.Text, wantText) || !reflect.DeepEqual(got.Categorical, wantCategorical) || !reflect.DeepEqual(got.Actions, wantActions) {
					t.Fatal("literal sixteen-field storage payload changed", mask, got, wantText, wantCategorical, wantActions)
				}
				if len(got.Actions) > 0 {
					*got.Actions[0].Before.Text = "caller"
					*got.Actions[0].After.Text = "caller"
					*got.Actions[1].After.Integer = "2"
					if *actions[0].Expected.Text != "Other" || *actions[0].Option != 1 || *actions[1].Expected.Integer != "2" {
						t.Fatal("owned action assignment shares original/proposal ownership")
					}
				}
				*got.Original.Rows[0].Env.Cells[0].Text = "caller"
				again, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(again, parent) {
					t.Fatal("action proposal caller changed source", again, err)
				}
			}
			legacy, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical)
			if err != nil {
				t.Fatal(err)
			}
			compatible, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical, nil)
			if err != nil || compatible == nil || !reflect.DeepEqual(legacy, &compatible.siviParentCategoricalProposal) || len(compatible.Actions) != 0 {
				t.Fatal("accepted fourteen-field signature/behavior changed", compatible, legacy, err)
			}
			assertProfileSUFiles(t, service, files)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("action proposal changed YAML", err)
			}
		})
	}
}

func TestSIVIParentActionProposalFifthDomainFailuresCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	text, categorical, actions := siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent), siviActionProposalEdits(parent)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, kind := range []string{"empty", "option", "source", "duplicate", "expected", "row", "context", "table", "missing-pair", "stale-context"} {
		t.Run(kind, func(t *testing.T) {
			a, b, c, d, e := scalars, options, text, categorical, append([]siviParentActionEdit(nil), actions...)
			id, plot := state.ContextID, "108050"
			switch kind {
			case "empty":
				a, b, c, d, e = nil, nil, nil, nil, nil
			case "option":
				bad := 3
				e[len(e)-1].Option = &bad
			case "source":
				e[len(e)-1].ControlID = "optSpeciesListComplete"
			case "duplicate":
				e = append(e, e[0])
			case "expected":
				e[len(e)-1].Expected = metadataText("changed source")
			case "row":
				e[len(e)-1].RowID = "-999"
			case "context":
				e[len(e)-1].ContextID = "stale"
			case "table":
				e[len(e)-1].Table = "Sample_Admin"
			case "missing-pair":
				plot = "missing"
			case "stale-context":
				id = "stale"
			}
			if got, err := service.prepareSIVIParentActionProposal(context.Background(), id, plot, a, b, c, d, e); err == nil || got != nil {
				t.Fatal("failed fifth domain retained earlier proposals", kind, got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.prepareSIVIParentActionProposal(ctx, state.ContextID, "108050", scalars, options, text, categorical, actions); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled action proposal returned assignments", got, err)
	}
	got, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical, actions)
	if err != nil || got == nil || got.Actions == nil {
		t.Fatal("rejected action proposal discarded attachments", got, err)
	}
	late, cancelLate := context.WithCancel(context.Background())
	defer cancelLate()
	rejected, err := withContextPlotRequest(late, service, state.ContextID, func(plots *PlotService) (*siviParentActionProposal, error) {
		return withOwnedSIVIParentRead(late, plots, state.ContextID, "108050", func(original *siviParentProjection) (*siviParentActionProposal, error) {
			if !reflect.DeepEqual(original, got.Original) {
				t.Fatal("late-cancellation action snapshot changed")
			}
			cancelLate()
			return got, nil
		})
	})
	if !errors.Is(err, context.Canceled) || rejected != nil {
		t.Fatal("late cancellation retained completed action assignments", rejected, err)
	}
	retry, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical, actions)
	if err != nil || !reflect.DeepEqual(retry, got) {
		t.Fatal("late cancellation discarded attachments/changed payload", retry, got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentActionProposalStaleHistoryAndExplicitNull(t *testing.T) {
	service, state := contextServiceFixture(t)
	old, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	stale := siviActionProposalEdits(old)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Admin SET PlotType='' WHERE Plot='108050';
		UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, nil, stale); err == nil || got != nil {
		t.Fatal("stale action original returned a proposal", got, err)
	}
	current, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := siviActionProposalEdits(current)
	edits[1].Option = nil
	got, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, nil, edits)
	want := []siviParentScalarAssignment{
		{state.ContextID, "Sample_Admin", current.Rows[0].Admin.RowID, "PlotType", metadataText(""), metadataText("Ground"), "Ground"},
		{state.ContextID, "Sample_Env", current.Rows[0].Env.RowID, "SpeciesListComplete", metadataInteger("2"), ProjectMetadataCell{Storage: "null"}, nil},
	}
	if err != nil || got == nil || !reflect.DeepEqual(got.Original, current) || !reflect.DeepEqual(got.Actions, want) {
		t.Fatal("owned historical correction/explicit NULL payload changed", got, want, err)
	}
	assertProfileSUFiles(t, service, files)
}
