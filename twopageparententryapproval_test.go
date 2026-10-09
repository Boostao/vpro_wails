package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"
)

func twoPageEntryApprovalDraft(t *testing.T, snapshot *twoPageEntryReferenceSnapshot) ([]siviParentScalarEdit, []twoPageEntryCodeAcknowledgement) {
	t.Helper()
	original := snapshot.Original
	edits := []siviParentScalarEdit{
		siviParentEdit(t, original, "FieldNumber", metadataText("  Literal field  ")),
		siviParentEdit(t, original, "Zone", metadataText("BG")),
		siviParentEdit(t, original, "SubZone", metadataText("xh1")),
	}
	acknowledgements := []twoPageEntryCodeAcknowledgement{}
	for _, reference := range snapshot.Fields {
		if reference.Required {
			found := false
			for _, choice := range reference.Choices {
				if choice.Selectable && choice.Code != nil && *choice.Code != "" {
					edits = append(edits, siviParentEdit(t, original, reference.Column, metadataText(*choice.Code)))
					found = true
					break
				}
			}
			if !found {
				t.Fatal("required actual fixture lacks a listed code", reference.Column)
			}
		}
		if reference.Column == "RealmClass" || reference.Column == "SiteSeries" {
			edit := siviParentEdit(t, original, reference.Column, metadataText("ZZ"))
			edits = append(edits, edit)
			acknowledgements = append(acknowledgements, twoPageEntryCodeAcknowledgement{
				ContextID: original.ContextID, Project: original.Project, Plot: original.Plot, Form: original.Form,
				Table: edit.Table, RowID: edit.RowID, Column: edit.Column, Expected: edit.Expected, Value: edit.Value, Reference: reference,
			})
		}
	}
	return edits, acknowledgements
}

func assertTwoPageEntryRestoredPhysicalTables(t *testing.T, form string, before, after map[string]ProjectMetadataTable) {
	t.Helper()
	history, err := twoPageEntryHistory(form, false)
	if err != nil {
		t.Fatal(err)
	}
	columns, err := siteUnitTransferColumns(after[history.table], "Restored")
	if err != nil || len(after[history.table].Rows) != 1 {
		t.Fatal("typed restoration lost its retained historical event", err)
	}
	restored := after[history.table].Rows[0].Cells[columns["Restored"]]
	if restored.Storage != "text" || restored.Text == nil || *restored.Text == "" {
		t.Fatal("typed restoration failed to mark its retained event", restored)
	}
	delete(after, history.table)
	schema := after["sqlite_master"]
	schemaColumns, err := siteUnitTransferColumns(schema, "name", "type")
	if err != nil {
		t.Fatal(err)
	}
	schemaRows := []ProjectMetadataRow{}
	found := 0
	for _, row := range schema.Rows {
		if reflect.DeepEqual(row.Cells[schemaColumns["name"]], metadataText(history.table)) {
			if !reflect.DeepEqual(row.Cells[schemaColumns["type"]], metadataText("table")) {
				t.Fatal("retained event schema is not a physical history table")
			}
			found++
		} else {
			schemaRows = append(schemaRows, row)
		}
	}
	if found != 1 {
		t.Fatal("retained history schema identity changed", found)
	}
	schema.Rows = schemaRows
	after["sqlite_master"] = schema
	if !reflect.DeepEqual(before, after) {
		t.Fatal("typed restoration changed original physical tables/schema")
	}
}

func TestTwoPageEntryFixedApprovalAtomicWriterRollbackRetryAndRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		snapshot, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", form,
			metadataText("BG"), metadataText("xh1"), provider.readers)
		if err != nil {
			t.Fatal(err)
		}
		edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "ro"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		originalTables := twoPageWriteTables(t, db)
		var owner *sqliteContext
		lifecycle := twoPageEntryLifecycle{
			prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
				owner = plots.projects.sqlite
				_, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS two_page_entry_refs`,
					sqliteFileURI(owner.attachments["VLists"], "ro"))
				if err != nil {
					return nil, err
				}
				return func() {}, nil
			},
			verify: func(_ *PlotService, _ *sql.Tx) error { return profileOwnedFiles(owner) },
		}
		write := func(acks []twoPageEntryCodeAcknowledgement) (*siviParentWriteResult, error) {
			return contexts.writeTwoPageEntryWithLifecycle(context.Background(), state.ContextID, "108050", form,
				snapshot.Original, edits, false,
				func(tx *sql.Tx, original *siviParentProjection, assignments []siviParentScalarAssignment) error {
					return approveTwoPageEntryReferences(context.Background(), tx, owner, provider, state.ContextID, original,
						assignments, acks, twoPageEntryReferenceSelection{2, 1}, twoPageEntryReferenceAliases{"main", "two_page_entry_refs", ""})
				}, lifecycle)
		}
		if result, err := write(nil); err == nil || result != nil {
			t.Fatal("missing explicit optional approval leaked a mixed commit", result, err)
		}
		assertProfileSUFiles(t, contexts, before)
		result, err := write(acknowledgements)
		if err != nil || result == nil || result.HistoryID == "" || result.ChangedCells != len(edits) {
			t.Fatal("fixed concrete approval could not retry one atomic mixed writer transaction", form, result, err)
		}
		observed, err := contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		for _, edit := range edits {
			if !reflect.DeepEqual(siviParentCell(t, observed, edit.Column), edit.Value) {
				t.Fatal("fixed approval changed literal source assignment", edit.Column)
			}
		}
		restored, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, "108050", form, result.HistoryID,
			AuditRestorePrune, false)
		if err != nil || restored == nil || restored.RestoredRows != len(edits) {
			t.Fatal("fixed reference approval could not restore literal mixed history", restored, err)
		}
		observed, err = contexts.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
		if err != nil || !reflect.DeepEqual(observed, snapshot.Original) {
			t.Fatal("fixed reference restoration changed physical source original", observed, err)
		}
		after := databaseBytes(t, contexts.projects.sqlite.attachments)
		restoredTables := twoPageWriteTables(t, db)
		assertTwoPageEntryRestoredPhysicalTables(t, form, originalTables, restoredTables)
		for role, bytes := range before {
			if !os.SameFile(owner.attachmentInfo[role], owner.attachmentInfo["project"]) && !reflect.DeepEqual(after[role], bytes) {
				t.Fatal("fixed approval writer/restoration changed an owned reference family", role)
			}
		}
	}
}

func TestTwoPageEntryFixedApprovalCurrentPlannedFiltersMembershipAndAcknowledgement(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		fresh, err := contexts.readTwoPageEntryReferences(context.Background(), state.ContextID, "108050", form,
			metadataText("BG"), metadataText("xh1"), provider.readers)
		if err != nil {
			t.Fatal(err)
		}

		edits, acknowledgements := twoPageEntryApprovalDraft(t, fresh)
		assignments, err := planTwoPageEntryProjection(context.Background(), fresh.Original, edits, false)
		if err != nil {
			t.Fatal(err)
		}
		var currentSeries SIVIParentSharedReference
		for _, acknowledgement := range acknowledgements {
			if acknowledgement.Column == "SiteSeries" {
				currentSeries = acknowledgement.Reference
			}
		}
		if len(currentSeries.Definitions.Rows) == 0 {
			t.Fatal("independent BG/xh1 catalogue fixture lacks filtered definitions")
		}
		_, err = withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
			return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
				for _, projectSource := range []int{1, 2} {
					for _, workingSource := range []int{1, 2, 3} {
						if err := approveTwoPageEntryReferences(context.Background(), tx, owner, provider, state.ContextID,
							fresh.Original, assignments, acknowledgements, twoPageEntryReferenceSelection{projectSource, workingSource},
							twoPageEntryReferenceAliases{"project", "VLists", "su"}); err != nil {
							t.Fatal("fixed mandatory/optional draft-filter composition failed", form, projectSource, workingSource, err)
						}
					}
				}
				stale, err := provider.read(context.Background(), tx, "VLists", provider.fields["SiteSeries"],
					ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"})
				if err != nil || reflect.DeepEqual(stale, currentSeries) {
					t.Fatal("old original filter fixture is not independently distinct", err)
				}
				for _, failure := range []string{"missing acknowledgement", "old BEC pair", "mandatory nonmember", "forged assignment",
					"foreign context", "foreign form", "foreign aliases", "bad source", "unused acknowledgement", "project mutation", "cancelled"} {
					current := append([]siviParentScalarAssignment{}, assignments...)
					acks := append([]twoPageEntryCodeAcknowledgement{}, acknowledgements...)
					original := *fresh.Original
					contextID := state.ContextID
					aliases := twoPageEntryReferenceAliases{"project", "VLists", "su"}
					selection := twoPageEntryReferenceSelection{2, 1}
					ctx := context.Background()
					switch failure {
					case "missing acknowledgement":
						acks = nil
					case "old BEC pair":
						for index := range acks {
							if acks[index].Column == "SiteSeries" {
								acks[index].Reference = stale
							}
						}
					case "mandatory nonmember":
						bad := append([]siviParentScalarEdit{}, edits...)
						for index := range bad {
							if bad[index].Column == "MesoSlopePosition" {
								bad[index].Value = metadataText("ZZ")
							}
						}
						current, err = planTwoPageEntryProjection(ctx, &original, bad, false)
						if err != nil {
							t.Fatal(err)
						}
					case "forged assignment":
						current[0].Value = "forged"
					case "foreign context":
						contextID = "foreign"
					case "foreign form":
						original.Form = "frmSIVIsite"
					case "foreign aliases":
						aliases.lists = "project"
					case "bad source":
						selection.workingSource = 4
					case "unused acknowledgement":
						current = nil
					case "project mutation":
						projectEdit := siviParentEdit(t, &original, "ProjectID", metadataText("new"))
						current = append(current, siviParentScalarAssignment{ContextID: projectEdit.ContextID, Table: projectEdit.Table,
							RowID: projectEdit.RowID, Column: "ProjectID", Before: projectEdit.Expected, After: projectEdit.Value, Value: "new"})
					case "cancelled":
						cancelled, cancel := context.WithCancel(ctx)
						cancel()
						ctx = cancelled
					}
					err := approveTwoPageEntryReferences(ctx, tx, owner, provider, contextID, &original, current, acks, selection, aliases)
					if err == nil || (failure == "cancelled" && !errors.Is(err, context.Canceled)) {
						t.Fatal("fixed approval accepted source/value/reference drift", failure, err)
					}
				}
				return true, nil
			})
		})
		if err != nil {
			t.Fatal(err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}
