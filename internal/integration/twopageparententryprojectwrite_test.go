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
	"encoding/json"
	"reflect"
	"testing"
)

func TestTwoPageEntryMixedProjectWriterRejectsForeignSourceSelectionAndUser(t *testing.T) {
	contexts, state, _, _, selection := sftest.SiviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	original, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	selection.ControlID = "form:" + original.Form + "/ProjectID"
	provider := twoPageEntryCatalogueProviderFixture(t, original.Form)
	values, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := twopage.TwoPageEntryReferenceSelectionFromConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	edits := []sourceform.SiviParentScalarEdit{
		sftest.SiviParentEdit(t, original, "ProjectID", selection.MetadataOriginal.Cells[0]),
		sftest.SiviParentEdit(t, original, "FieldNumber", vprotest.MetadataText("  Literal field  ")),
	}
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	for _, failure := range []string{"SIVI", "CHARS", "title", "source", "missing physical row", "different literal", "no audit user"} {
		current := selection
		currentEdits := append([]sourceform.SiviParentScalarEdit{}, edits...)
		user := contexts.Plots.CurrentUser
		switch failure {
		case "SIVI":
			current.ControlID = "form:frmSIVIsite/ProjectID"
		case "CHARS":
			current.ControlID = "form:FS882-8x6XL-CHARS/ProjectID"
		case "title":
			current.MetadataOriginal.Cells = []vprocore.ProjectMetadataCell{selection.MetadataOriginal.Cells[0], vprotest.MetadataText("different title")}
		case "source":
			current.SourceOption = 1
		case "missing physical row":
			current.MetadataOriginal.RowID = "-999"
		case "different literal":
			currentEdits[0].Value = vprotest.MetadataText("not the physical choice")
		case "no audit user":
			contexts.Plots.CurrentUser = ""
		}
		result, err := twopage.WriteTwoPageEntryWithOwnedReferencesAndProject(contexts, context.Background(), state.ContextID, "108050", original.Form,
			original, currentEdits, false, provider.Readers, sources, nil, &current)
		contexts.Plots.CurrentUser = user
		if err == nil || result != nil {
			t.Fatal("mixed source selection bypassed independent ownership/user proof", failure, result, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	for _, role := range []string{"project", "VMetaData", "VLists", "su"} {
		owner := contexts.Projects.Sqlite
		prior, attached := owner.AttachmentInfo[role]
		if !attached {
			continue
		}
		replacement := owner.AttachmentInfo["VLists"]
		if role == "VLists" {
			replacement = owner.AttachmentInfo["project"]
		}
		owner.AttachmentInfo[role] = replacement
		ordered := []sourceform.SiviParentScalarEdit{edits[1], edits[0]}
		result, err := twopage.WriteTwoPageEntryWithOwnedReferencesAndProject(contexts, context.Background(), state.ContextID, "108050", original.Form,
			original, ordered, false, provider.Readers, sources, nil, &selection)
		owner.AttachmentInfo[role] = prior
		if err == nil || result != nil {
			t.Fatal("mixed ProjectID source ignored a foreign physical attachment", role, result, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
	result, err := twopage.WriteTwoPageEntryWithOwnedReferencesAndProject(contexts, context.Background(), state.ContextID, "108050", original.Form,
		original, edits, false, provider.Readers, sources, nil, &selection)
	if err != nil || result == nil || result.ChangedCells != 2 || result.HistoryID == "" {
		t.Fatal("mixed ProjectID proof/ownership failure leaked lease or prevented valid retry", result, err)
	}
}

func TestTwoPageEntryMixedProjectWriterAllAuditStrengthsAndUnauditedRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, source := range []int{1, 2} {
			for strength := 0; strength <= 3; strength++ {
				for _, beforeProject := range []any{nil, "old"} {
					contexts, state, db, _, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, source, strength, beforeProject)
					if _, err := db.Exec(`UPDATE Sample_Env SET FieldNumber='old field',SV_PolygonNumber=NULL WHERE PlotNumber='108050';
						UPDATE Sample_Admin SET GIS_BGC=NULL WHERE Plot='108050'`); err != nil {
						t.Fatal(err)
					}
					original, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					selection.ControlID = "form:" + form + "/ProjectID"
					provider := twoPageEntryCatalogueProviderFixture(t, form)
					values := map[string]vprocore.ProjectMetadataCell{
						"ProjectID":        selection.MetadataOriginal.Cells[0],
						"FieldNumber":      vprotest.MetadataText("  Literal field  "),
						"SV_PolygonNumber": vprotest.MetadataText("  Literal polygon  "),
						"GIS_BGC":          vprotest.MetadataText("  Literal GIS  "),
					}
					edits := []sourceform.SiviParentScalarEdit{}
					for _, column := range []string{"FieldNumber", "ProjectID", "SV_PolygonNumber", "GIS_BGC"} {
						edits = append(edits, sftest.SiviParentEdit(t, original, column, values[column]))
					}
					observed, err := contexts.Projects.Preferences.Snapshot()
					if err != nil {
						t.Fatal(err)
					}
					sources, err := twopage.TwoPageEntryReferenceSelectionFromConfig(observed)
					if err != nil {
						t.Fatal(err)
					}
					before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					if result, err := twopage.WriteTwoPageEntryWithOwnedReferences(contexts, context.Background(), state.ContextID, "108050", form,
						original, edits, false, provider.Readers, sources, nil); err == nil || result != nil {
						t.Fatal("mixed ProjectID mutation escaped its strict physical selection proof", result, err)
					}
					vprotest.AssertProfileSUFiles(t, contexts, before)
					result, err := twopage.WriteTwoPageEntryWithOwnedReferencesAndProject(contexts, context.Background(), state.ContextID, "108050", form,
						original, edits, false, provider.Readers, sources, nil, &selection)
					if err != nil || result == nil || result.ChangedCells != 4 || (result.HistoryID == "") != (strength == 0) {
						t.Fatal("mixed ProjectID source/audit strength failed one atomic save", form, source, strength, beforeProject, result, err)
					}
					committed, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					for column, value := range values {
						if !reflect.DeepEqual(sftest.SiviParentCell(t, committed, column), value) {
							t.Fatal("mixed ProjectID save changed literal values", column)
						}
					}
					if strength == 0 {
						continue
					}
					history, err := twopage.TwoPageEntryHistory(form, false)
					if err != nil {
						t.Fatal(err)
					}
					var proposal string
					if err := db.QueryRow(`SELECT Proposal FROM `+vprocore.QuoteHeaderIdentifier(history.Table)+` WHERE ID=?`, result.HistoryID).Scan(&proposal); err != nil {
						t.Fatal(err)
					}
					var event sourceform.SiviParentHistory
					if err := json.Unmarshal([]byte(proposal), &event); err != nil {
						t.Fatal(err)
					}
					audited := 4
					if strength == 1 {
						audited = 1
						if beforeProject != nil {
							audited++
						}
					}
					if len(event.EntryPlan) != 4 || len(event.Changes) != audited || event.ProjectAssignment == nil ||
						event.ProjectAssignment.Selection.ControlID != selection.ControlID ||
						event.ProjectAssignment.Selection.SourceOption != source ||
						event.ProjectAssignment.Selection.MetadataOriginal.Cells[1].Storage != "null" {
						t.Fatal("mixed typed history lost full plan/audited subset/exact source provenance", event)
					}
					if _, err := history.Assignments(context.Background(), event, original.Project, original.Plot); err != nil {
						t.Fatal("mixed save wrote an unrestorable full source plan", err)
					}
					if strength == 1 && beforeProject == nil {
						if source == 1 {
							if _, err := db.Exec(`DELETE FROM Sample_Metadata`); err != nil {
								t.Fatal(err)
							}
						} else {
							vprotest.MutateContextFixture(t, contexts.Projects.SupportPaths["VMetaData"], `DELETE FROM ProjectMetadata`)
						}
						if err := contexts.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": 3 - source}); err != nil {
							t.Fatal(err)
						}
						before = vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					}
					restore, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, "108050", form,
						result.HistoryID, vprocore.AuditRestorePrune, false)
					if err != nil || restore == nil || restore.RestoredRows != audited {
						t.Fatal("mixed typed ProjectID history could not restore audited subset", form, source, strength, beforeProject, restore, err)
					}
					restored, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					for column, after := range values {
						expected := sftest.SiviParentCell(t, original, column)
						if strength == 1 && expected.Storage == "null" {
							expected = after
						}
						if !reflect.DeepEqual(sftest.SiviParentCell(t, restored, column), expected) {
							t.Fatal("restoration changed unaudited values or failed audited source restoration", column, strength)
						}
					}
					after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
					for _, role := range []string{"VLists", "VMetaData"} {
						if !reflect.DeepEqual(after[role], before[role]) {
							t.Fatal("mixed ProjectID writer/restoration changed a source reference family", role)
						}
					}
				}
			}
		}
	}
}
