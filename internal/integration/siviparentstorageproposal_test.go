package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSIVIParentStorageProposalElevenFieldsAndLegacyCompatibility(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := vprotest.ContextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # storage proposal.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := vprotest.ContextSelection(state)
				selection.ProjectPath = path
				state, err = service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`UPDATE Sample_Env SET SV_PolygonNumber='original polygon',
				SV_CanopyComposition='original canopy' WHERE PlotNumber='108050'`)
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			scalars, options := sftest.SiviProposalEdits(parent)
			text := sftest.SiviStorageTextEdits(parent)
			if len(text) != 2 {
				t.Fatal("ordinary storage fixture lost source fields", text)
			}
			files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, domain := range []string{"all", "text", "scalar-text", "option-text", "unchanged"} {
				a, b, c := scalars, options, append([]sourceform.SiviParentScalarEdit(nil), text...)
				switch domain {
				case "text":
					a, b = nil, nil
				case "scalar-text":
					b = nil
				case "option-text":
					a = nil
				case "unchanged":
					a, b = nil, nil
					for i := range c {
						c[i].Value = c[i].Expected
					}
				}
				got, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), state.ContextID, "108050", a, b, c)
				if err != nil || got == nil || !reflect.DeepEqual(got.Original, parent) ||
					got.Scalars == nil || got.Options == nil || got.Text == nil {
					t.Fatal("complete original/domain proposal changed", domain, got, err)
				}
				if len(a) > 0 {
					assertSIVIProposalAssignments(t, parent, got.Scalars, false)
				} else if len(got.Scalars) != 0 {
					t.Fatal("absent scalar domain invented assignments", got)
				}
				if len(b) > 0 {
					assertSIVIProposalAssignments(t, parent, got.Options, true)
				} else if len(got.Options) != 0 {
					t.Fatal("absent option domain invented assignments", got)
				}
				want := []sourceform.SiviParentScalarAssignment{
					{ContextID: state.ContextID, Table: "Sample_Env", RowID: parent.Rows[0].Env.RowID, Column: "SV_PolygonNumber", Before: vprotest.MetadataText("original polygon"), After: vprotest.MetadataText("  New Polygon  "), Value: "  New Polygon  "},
					{ContextID: state.ContextID, Table: "Sample_Env", RowID: parent.Rows[0].Env.RowID, Column: "SV_CanopyComposition", Before: vprotest.MetadataText("original canopy"), After: vprotest.MetadataText("Literal canopy"), Value: "Literal canopy"},
				}
				if domain == "unchanged" {
					want = []sourceform.SiviParentScalarAssignment{}
				}
				if !reflect.DeepEqual(got.Text, want) {
					t.Fatal("literal two-field storage payload changed", domain, got.Text, want)
				}
				if len(got.Text) > 0 {
					*got.Text[0].Before.Text = "caller"
					*got.Text[0].After.Text = "caller"
					if *text[0].Expected.Text != "original polygon" || *text[0].Value.Text != "  New Polygon  " {
						t.Fatal("owned TEXT plan shares original/proposal ownership")
					}
				}
				*got.Original.Rows[0].Env.Cells[0].Text = "caller"
				again, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(again, parent) {
					t.Fatal("proposal caller changed source", again, err)
				}
			}
			legacy, err := sourceform.PrepareSIVIParentProposal(service, context.Background(), state.ContextID, "108050", scalars, options)
			if err != nil {
				t.Fatal(err)
			}
			compatible, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), state.ContextID, "108050", scalars, options, nil)
			if err != nil || compatible == nil || !reflect.DeepEqual(legacy, &compatible.SiviParentProposal) || len(compatible.Text) != 0 {
				t.Fatal("accepted nine-field signature/behavior changed", compatible, legacy, err)
			}
			vprotest.AssertProfileSUFiles(t, service, files)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("storage planning changed YAML", err)
			}
		})
	}
}

func TestSIVIParentStorageProposalThirdDomainFailuresReturnNoPartialResult(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := sftest.SiviProposalEdits(parent)
	text := sftest.SiviStorageTextEdits(parent)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	for _, kind := range []string{"empty", "empty-text", "domain", "duplicate", "expected", "row", "context", "missing-pair", "stale-context"} {
		t.Run(kind, func(t *testing.T) {
			a, b, c := scalars, options, append([]sourceform.SiviParentScalarEdit(nil), text...)
			id, plot := state.ContextID, "108050"
			switch kind {
			case "empty":
				a, b, c = nil, nil, nil
			case "empty-text":
				c[len(c)-1].Value = vprotest.MetadataText("")
			case "domain":
				c[len(c)-1] = options[0]
			case "duplicate":
				c = append(c, c[0])
			case "expected":
				c[len(c)-1].Expected = vprotest.MetadataText("changed source")
			case "row":
				c[len(c)-1].RowID = "-999"
			case "context":
				c[len(c)-1].ContextID = "stale"
			case "missing-pair":
				plot = "missing"
			case "stale-context":
				id = "stale"
			}
			if got, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), id, plot, a, b, c); err == nil || got != nil {
				t.Fatal("failed third domain returned earlier assignments", kind, got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.PrepareSIVIParentStorageProposal(service, ctx, state.ContextID, "108050", scalars, options, text); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled storage proposal returned assignments", got, err)
	}
	got, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), state.ContextID, "108050", scalars, options, text)
	if err != nil || got == nil || len(got.Scalars) != 7 || len(got.Options) != 2 || len(got.Text) != 2 {
		t.Fatal("rejected storage proposal discarded attachments", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
}

func TestSIVIParentStorageProposalHistoricalTextAndStaleOriginal(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	old, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	stale := sftest.SiviStorageTextEdits(old)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Env SET SV_PolygonNumber='',SV_CanopyComposition=X'00FF' WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), state.ContextID, "108050", nil, nil, stale); err == nil || got != nil {
		t.Fatal("stale storage source was silently adopted", got, err)
	}
	current, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	text := sftest.SiviStorageTextEdits(current)
	for i := range text {
		text[i].Value = text[i].Expected
	}
	got, err := sourceform.PrepareSIVIParentStorageProposal(service, context.Background(), state.ContextID, "108050", nil, nil, text)
	if err != nil || got == nil || got.Text == nil || len(got.Text) != 0 ||
		!reflect.DeepEqual(got.Original, current) {
		t.Fatal("owned historical TEXT aliases were repaired or assigned", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
}
