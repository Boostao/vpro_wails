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

func siviCategoricalProposalEdits(parent *siviParentProjection) []siviParentScalarEdit {
	edits := []siviParentScalarEdit{}
	for _, field := range []struct{ column, value string }{
		{"SnowCoverregime", "Z"}, {"SV_RootZoneTexture", "  MiXeD \t "}, {"SV_AhorizonType", ""},
	} {
		for index, column := range parent.EnvColumns {
			if column.Name == field.column {
				edits = append(edits, siviParentScalarEdit{
					ContextID: parent.ContextID, Table: parent.EnvTable, RowID: parent.Rows[0].Env.RowID,
					Column: field.column, Expected: cloneSiteUnitCell(parent.Rows[0].Env.Cells[index]), Value: metadataText(field.value),
				})
			}
		}
	}
	return edits
}

func TestSIVIParentCategoricalProposalFourteenFieldsAndLegacyCompatibility(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # categorical proposal.db")
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
				SV_RootZoneTexture='original texture',SV_AhorizonType='Ah' WHERE PlotNumber='108050'`)
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			scalars, options := siviProposalEdits(parent)
			text, categorical := siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent)
			if len(text) != 2 || len(categorical) != 3 {
				t.Fatal("categorical storage fixture lost source fields")
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for mask := 1; mask < 16; mask++ {
				var a, b, c, d []siviParentScalarEdit
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
				got, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", a, b, c, d)
				if err != nil || got == nil || !reflect.DeepEqual(got.Original, parent) ||
					got.Scalars == nil || got.Options == nil || got.Text == nil || got.Categorical == nil {
					t.Fatal("complete original/domain proposal changed", mask, got, err)
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
				wantText := []siviParentScalarAssignment{}
				if len(c) > 0 {
					wantText = []siviParentScalarAssignment{
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_PolygonNumber", metadataText("original polygon"), metadataText("  New Polygon  "), "  New Polygon  "},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_CanopyComposition", metadataText("original canopy"), metadataText("Literal canopy"), "Literal canopy"},
					}
				}
				wantCategorical := []siviParentScalarAssignment{}
				if len(d) > 0 {
					wantCategorical = []siviParentScalarAssignment{
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SnowCoverregime", metadataText("S"), metadataText("Z"), "Z"},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_RootZoneTexture", metadataText("original texture"), metadataText("  MiXeD \t "), "  MiXeD \t "},
						{state.ContextID, "Sample_Env", parent.Rows[0].Env.RowID, "SV_AhorizonType", metadataText("Ah"), metadataText(""), ""},
					}
				}
				if !reflect.DeepEqual(got.Text, wantText) || !reflect.DeepEqual(got.Categorical, wantCategorical) {
					t.Fatal("literal TEXT/categorical payload changed", mask, got, wantText, wantCategorical)
				}
				if len(got.Categorical) > 0 {
					*got.Categorical[0].Before.Text = "caller"
					*got.Categorical[0].After.Text = "caller"
					if *categorical[0].Expected.Text != "S" || *categorical[0].Value.Text != "Z" {
						t.Fatal("owned categorical assignment shares original/proposal ownership")
					}
				}
				*got.Original.Rows[0].Env.Cells[0].Text = "caller"
				again, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(again, parent) {
					t.Fatal("proposal caller changed source", again, err)
				}
			}
			legacy, err := service.prepareSIVIParentStorageProposal(context.Background(), state.ContextID, "108050", scalars, options, text)
			if err != nil {
				t.Fatal(err)
			}
			compatible, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", scalars, options, text, nil)
			if err != nil || compatible == nil || !reflect.DeepEqual(legacy, &compatible.siviParentStorageProposal) || len(compatible.Categorical) != 0 {
				t.Fatal("accepted eleven-field signature/behavior changed", compatible, legacy, err)
			}
			nine, err := service.prepareSIVIParentProposal(context.Background(), state.ContextID, "108050", scalars, options)
			if err != nil {
				t.Fatal(err)
			}
			compatible, err = service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", scalars, options, nil, nil)
			if err != nil || compatible == nil || !reflect.DeepEqual(nine, &compatible.siviParentProposal) ||
				len(compatible.Text) != 0 || len(compatible.Categorical) != 0 {
				t.Fatal("accepted nine-field signature/behavior changed", compatible, nine, err)
			}
			assertProfileSUFiles(t, service, files)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("categorical proposal changed YAML", err)
			}
		})
	}
}

func TestSIVIParentCategoricalProposalFourthDomainFailureAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	text, categorical := siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, kind := range []string{"empty", "overlength", "domain", "implicit", "duplicate", "expected", "row", "context", "table", "missing-pair", "stale-context"} {
		t.Run(kind, func(t *testing.T) {
			a, b, c, d := scalars, options, text, append([]siviParentScalarEdit(nil), categorical...)
			id, plot := state.ContextID, "108050"
			switch kind {
			case "empty":
				a, b, c, d = nil, nil, nil, nil
			case "overlength":
				d[len(d)-1].Value = metadataText("123456")
			case "domain":
				d[len(d)-1] = text[0]
			case "implicit":
				d[len(d)-1].Column = "SpeciesListComplete"
			case "duplicate":
				d = append(d, d[0])
			case "expected":
				d[len(d)-1].Expected = metadataText("changed source")
			case "row":
				d[len(d)-1].RowID = "-999"
			case "context":
				d[len(d)-1].ContextID = "stale"
			case "table":
				d[len(d)-1].Table = "Sample_Admin"
			case "missing-pair":
				plot = "missing"
			case "stale-context":
				id = "stale"
			}
			if got, err := service.prepareSIVIParentCategoricalProposal(context.Background(), id, plot, a, b, c, d); err == nil || got != nil {
				t.Fatal("failed fourth domain returned earlier assignments", kind, got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.prepareSIVIParentCategoricalProposal(ctx, state.ContextID, "108050", scalars, options, text, categorical); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled categorical proposal returned assignments", got, err)
	}
	got, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical)
	if err != nil || got == nil || len(got.Scalars) != 7 || len(got.Options) != 2 || len(got.Text) != 2 || len(got.Categorical) != 3 {
		t.Fatal("rejected categorical proposal discarded attachments", got, err)
	}
	late, cancelLate := context.WithCancel(context.Background())
	defer cancelLate()
	rejected, err := withContextPlotRequest(late, service, state.ContextID, func(plots *PlotService) (*siviParentCategoricalProposal, error) {
		return withOwnedSIVIParentRead(late, plots, state.ContextID, "108050", func(original *siviParentProjection) (*siviParentCategoricalProposal, error) {
			if !reflect.DeepEqual(original, got.Original) {
				t.Fatal("late-cancellation snapshot differs from completed proposal")
			}
			cancelLate()
			return got, nil
		})
	})
	if !errors.Is(err, context.Canceled) || rejected != nil {
		t.Fatal("late cancellation retained a complete fourteen-field proposal", rejected, err)
	}
	retry, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", scalars, options, text, categorical)
	if err != nil || !reflect.DeepEqual(retry, got) {
		t.Fatal("late categorical cancellation discarded attachments or changed payload", retry, got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentCategoricalProposalHistoricalStorageAndStaleOriginal(t *testing.T) {
	service, state := contextServiceFixture(t)
	old, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	stale := siviCategoricalProposalEdits(old)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Env SET SnowCoverregime='',SV_RootZoneTexture=X'00FF',SV_AhorizonType=NULL WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, stale); err == nil || got != nil {
		t.Fatal("stale categorical original returned assignments", got, err)
	}
	current, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := siviCategoricalProposalEdits(current)
	for index := range edits {
		edits[index].Value = cloneSiteUnitCell(edits[index].Expected)
	}
	got, err := service.prepareSIVIParentCategoricalProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, edits)
	if err != nil || got == nil || len(got.Categorical) != 0 || !reflect.DeepEqual(got.Original, current) {
		t.Fatal("historical categorical storage was changed", got, err)
	}
	if edits[0].Expected.Storage != "text" || *edits[0].Expected.Text != "" ||
		edits[1].Expected.Storage != "blob" || *edits[1].Expected.BlobHex != "00ff" || edits[2].Expected.Storage != "null" {
		t.Fatal("physical empty/BLOB/NULL history collapsed", edits)
	}
	*got.Original.Rows[0].Env.Cells[0].Text = "caller"
	again, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(again, current) {
		t.Fatal("historical proposal shares source ownership", again, err)
	}
	assertProfileSUFiles(t, service, files)
}
