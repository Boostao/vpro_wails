package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestTwoPageEntryMixedProjectWriterRejectsForeignSourceSelectionAndUser(t *testing.T) {
	contexts, state, _, _, selection := siviProjectAssignmentWriteFixture(t, true, 2, 3, "old")
	original, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	selection.ControlID = "form:" + original.Form + "/ProjectID"
	provider := twoPageEntryCatalogueProviderFixture(t, original.Form)
	values, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := twoPageEntryReferenceSelectionFromConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	edits := []siviParentScalarEdit{
		siviParentEdit(t, original, "ProjectID", selection.MetadataOriginal.Cells[0]),
		siviParentEdit(t, original, "FieldNumber", metadataText("  Literal field  ")),
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, failure := range []string{"SIVI", "CHARS", "title", "source", "missing physical row", "different literal", "no audit user"} {
		current := selection
		currentEdits := append([]siviParentScalarEdit{}, edits...)
		user := contexts.plots.currentUser
		switch failure {
		case "SIVI":
			current.ControlID = "form:frmSIVIsite/ProjectID"
		case "CHARS":
			current.ControlID = "form:FS882-8x6XL-CHARS/ProjectID"
		case "title":
			current.MetadataOriginal.Cells = []ProjectMetadataCell{selection.MetadataOriginal.Cells[0], metadataText("different title")}
		case "source":
			current.SourceOption = 1
		case "missing physical row":
			current.MetadataOriginal.RowID = "-999"
		case "different literal":
			currentEdits[0].Value = metadataText("not the physical choice")
		case "no audit user":
			contexts.plots.currentUser = ""
		}
		result, err := contexts.writeTwoPageEntryWithOwnedReferencesAndProject(context.Background(), state.ContextID, "108050", original.Form,
			original, currentEdits, false, provider.readers, sources, nil, &current)
		contexts.plots.currentUser = user
		if err == nil || result != nil {
			t.Fatal("mixed source selection bypassed independent ownership/user proof", failure, result, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	for _, role := range []string{"project", "VMetaData", "VLists", "su"} {
		owner := contexts.projects.sqlite
		prior, attached := owner.attachmentInfo[role]
		if !attached {
			continue
		}
		replacement := owner.attachmentInfo["VLists"]
		if role == "VLists" {
			replacement = owner.attachmentInfo["project"]
		}
		owner.attachmentInfo[role] = replacement
		ordered := []siviParentScalarEdit{edits[1], edits[0]}
		result, err := contexts.writeTwoPageEntryWithOwnedReferencesAndProject(context.Background(), state.ContextID, "108050", original.Form,
			original, ordered, false, provider.readers, sources, nil, &selection)
		owner.attachmentInfo[role] = prior
		if err == nil || result != nil {
			t.Fatal("mixed ProjectID source ignored a foreign physical attachment", role, result, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	result, err := contexts.writeTwoPageEntryWithOwnedReferencesAndProject(context.Background(), state.ContextID, "108050", original.Form,
		original, edits, false, provider.readers, sources, nil, &selection)
	if err != nil || result == nil || result.ChangedCells != 2 || result.HistoryID == "" {
		t.Fatal("mixed ProjectID proof/ownership failure leaked lease or prevented valid retry", result, err)
	}
}

func TestTwoPageEntryMixedProjectWriterAllAuditStrengthsAndUnauditedRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, source := range []int{1, 2} {
			for strength := 0; strength <= 3; strength++ {
				for _, beforeProject := range []any{nil, "old"} {
					contexts, state, db, _, selection := siviProjectAssignmentWriteFixture(t, false, source, strength, beforeProject)
					if _, err := db.Exec(`UPDATE Sample_Env SET FieldNumber='old field',SV_PolygonNumber=NULL WHERE PlotNumber='108050';
						UPDATE Sample_Admin SET GIS_BGC=NULL WHERE Plot='108050'`); err != nil {
						t.Fatal(err)
					}
					original, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					selection.ControlID = "form:" + form + "/ProjectID"
					provider := twoPageEntryCatalogueProviderFixture(t, form)
					values := map[string]ProjectMetadataCell{
						"ProjectID":        selection.MetadataOriginal.Cells[0],
						"FieldNumber":      metadataText("  Literal field  "),
						"SV_PolygonNumber": metadataText("  Literal polygon  "),
						"GIS_BGC":          metadataText("  Literal GIS  "),
					}
					edits := []siviParentScalarEdit{}
					for _, column := range []string{"FieldNumber", "ProjectID", "SV_PolygonNumber", "GIS_BGC"} {
						edits = append(edits, siviParentEdit(t, original, column, values[column]))
					}
					observed, err := contexts.projects.preferences.snapshot()
					if err != nil {
						t.Fatal(err)
					}
					sources, err := twoPageEntryReferenceSelectionFromConfig(observed)
					if err != nil {
						t.Fatal(err)
					}
					before := databaseBytes(t, contexts.projects.sqlite.attachments)
					if result, err := contexts.writeTwoPageEntryWithOwnedReferences(context.Background(), state.ContextID, "108050", form,
						original, edits, false, provider.readers, sources, nil); err == nil || result != nil {
						t.Fatal("mixed ProjectID mutation escaped its strict physical selection proof", result, err)
					}
					assertProfileSUFiles(t, contexts, before)
					result, err := contexts.writeTwoPageEntryWithOwnedReferencesAndProject(context.Background(), state.ContextID, "108050", form,
						original, edits, false, provider.readers, sources, nil, &selection)
					if err != nil || result == nil || result.ChangedCells != 4 || (result.HistoryID == "") != (strength == 0) {
						t.Fatal("mixed ProjectID source/audit strength failed one atomic save", form, source, strength, beforeProject, result, err)
					}
					committed, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					for column, value := range values {
						if !reflect.DeepEqual(siviParentCell(t, committed, column), value) {
							t.Fatal("mixed ProjectID save changed literal values", column)
						}
					}
					if strength == 0 {
						continue
					}
					history, err := twoPageEntryHistory(form, false)
					if err != nil {
						t.Fatal(err)
					}
					var proposal string
					if err := db.QueryRow(`SELECT Proposal FROM `+quoteHeaderIdentifier(history.table)+` WHERE ID=?`, result.HistoryID).Scan(&proposal); err != nil {
						t.Fatal(err)
					}
					var event siviParentHistory
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
					if _, err := history.assignments(context.Background(), event, original.Project, original.Plot); err != nil {
						t.Fatal("mixed save wrote an unrestorable full source plan", err)
					}
					if strength == 1 && beforeProject == nil {
						if source == 1 {
							if _, err := db.Exec(`DELETE FROM Sample_Metadata`); err != nil {
								t.Fatal(err)
							}
						} else {
							mutateContextFixture(t, contexts.projects.supportPaths["VMetaData"], `DELETE FROM ProjectMetadata`)
						}
						if err := contexts.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 3 - source}); err != nil {
							t.Fatal(err)
						}
						before = databaseBytes(t, contexts.projects.sqlite.attachments)
					}
					restore, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, "108050", form,
						result.HistoryID, AuditRestorePrune, false)
					if err != nil || restore == nil || restore.RestoredRows != audited {
						t.Fatal("mixed typed ProjectID history could not restore audited subset", form, source, strength, beforeProject, restore, err)
					}
					restored, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
					if err != nil {
						t.Fatal(err)
					}
					for column, after := range values {
						expected := siviParentCell(t, original, column)
						if strength == 1 && expected.Storage == "null" {
							expected = after
						}
						if !reflect.DeepEqual(siviParentCell(t, restored, column), expected) {
							t.Fatal("restoration changed unaudited values or failed audited source restoration", column, strength)
						}
					}
					after := databaseBytes(t, contexts.projects.sqlite.attachments)
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
