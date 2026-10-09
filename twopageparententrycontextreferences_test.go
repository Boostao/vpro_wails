package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageEntryContextReferenceProjectSourcesAndRawMetadata(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, _, _, _ := twoPageWriteFixture(t, form, false)
		mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Metadata;
			INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle) VALUES
			(-7,'PROJECT','  literal title  '),(2,'PROJECT',NULL),(3,NULL,''),(4,'',NULL)`)
		mutateContextFixture(t, contexts.projects.sqlite.attachments["VMetaData"], `DELETE FROM ProjectMetadata;
			INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES
			('MASTER','  literal title  '),('MASTER2',NULL),(NULL,''),('',NULL)`)
		provider, err := newTwoPageEntryReferenceProvider(form, twoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		for _, source := range []int{1, 2} {
			_, err := withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
				return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
					ref, err := readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, form,
						twoPageEntryReferenceSelection{source, 1}, twoPageEntryReferenceAliases{"project", "VLists", "su"}, provider.fields["ProjectID"])
					if err != nil {
						return false, err
					}
					value, second := "PROJECT", "PROJECT"
					if source == 2 {
						value, second = "MASTER", "MASTER2"
					}
					if !ref.Available || ref.Required || len(ref.Choices) != 4 || len(ref.Definitions.Rows) != 4 ||
						!ref.Choices[0].Selectable || !ref.Choices[1].Selectable || ref.Choices[2].Selectable || ref.Choices[3].Selectable ||
						ref.Choices[0].Code == nil || *ref.Choices[0].Code != value || *ref.Choices[1].Code != second ||
						!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[1], metadataText("  literal title  ")) ||
						ref.Definitions.Rows[1].Cells[1].Storage != "null" ||
						!reflect.DeepEqual(ref.Definitions.Rows[2].Cells[1], metadataText("")) ||
						!reflect.DeepEqual(ref.Definitions.Rows[3].Cells[0], metadataText("")) {
						t.Fatal("source selection/duplicate/NULL/empty metadata changed", source, ref)
					}
					if source == 1 && ref.Choices[0].RowID != "-7" {
						t.Fatal("signed physical row identity repaired", ref.Choices[0])
					}
					return true, nil
				})
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestTwoPageEntryContextReferenceUnitsFromOwnedCurrentSources(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state, _, _, _ := twoPageWriteFixture(t, "FS882-8x6XL", external)
		mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `DELETE FROM MasterSiteUnitList;
			INSERT INTO MasterSiteUnitList(ID,SiteSeries,SiteSeriesLongName,SiteSeriesScientificName,Level) VALUES
			(1,'  Master literal  ',NULL,'scientific',11),(2,'  Master literal  ','','scientific',11),
			(3,NULL,NULL,NULL,11),(4,'',NULL,NULL,11),(5,'HIDDEN','hidden',NULL,12)`)
		mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Admin SET UserSiteUnit='  Env literal  ';
			INSERT INTO Sample_Admin(Plot,UserSiteUnit) VALUES('orphan','ORPHAN')`)
		mutateContextFixture(t, state.SUPath, `DELETE FROM Report_SU;
			INSERT INTO Report_SU(PlotNumber,SiteUnit) VALUES
			('108050','  SU literal  '),('108051','  SU literal  '),('108052',NULL),('108053','')`)
		provider, err := newTwoPageEntryReferenceProvider("FS882-8x6XL", twoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		for _, name := range []string{"BECSiteUnit", "UserSiteUnit"} {
			for _, source := range []int{1, 2, 3} {
				_, err := withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
					return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
						ref, err := readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
							twoPageEntryReferenceSelection{1, source}, twoPageEntryReferenceAliases{"project", "VLists", "su"}, provider.fields[name])
						if err != nil {
							return false, err
						}
						if !ref.Available {
							t.Fatal("owned source incorrectly unavailable", ref)
						}
						if name == "BECSiteUnit" || source == 2 {
							listed, invalid := 0, 0
							for _, choice := range ref.Choices {
								if choice.Selectable {
									listed++
								} else {
									invalid++
								}
							}
							if len(ref.Choices) != 4 || len(ref.Definitions.Columns) != 5 || listed != 2 || invalid != 2 {
								t.Fatal("master Level11 definitions/invalid choices changed", ref)
							}
							byID := map[string]ProjectMetadataRow{}
							for _, row := range ref.Definitions.Rows {
								byID[row.RowID] = row
							}
							if len(byID) != 4 || !reflect.DeepEqual(byID["1"].Cells[1], metadataText("  Master literal  ")) ||
								byID["1"].Cells[2].Storage != "null" || !reflect.DeepEqual(byID["2"].Cells[2], metadataText("")) {
								t.Fatal("master identity/duplicate/null/empty values changed", byID)
							}
						} else if source == 1 {
							if len(ref.Choices) != 1 || *ref.Choices[0].Code != "  Env literal  " ||
								!ref.Choices[0].Selectable || !strings.Contains(ref.Source, "distinct literal") {
								t.Fatal("Env orphan/duplicates included or literal changed", ref)
							}
						} else if len(ref.Choices) != 2 || ref.Choices[0].Code == nil || *ref.Choices[0].Code != "" ||
							ref.Choices[0].Selectable || *ref.Choices[1].Code != "  SU literal  " || !ref.Choices[1].Selectable {
							t.Fatal("owned selected SU group/source literal changed", external, ref)
						}
						return true, nil
					})
				})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestTwoPageEntryContextReferenceGuardsCancellationNoSUAndRetry(t *testing.T) {
	contexts, state, _, _, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	provider, err := newTwoPageEntryReferenceProvider("FS882-8x6XL", twoPageEntryReferenceReaders{})
	if err != nil {
		t.Fatal(err)
	}
	values, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	selection, err := twoPageEntryReferenceSelectionFromConfig(values)
	if err != nil || selection.projectSource != 2 || selection.workingSource != 1 {
		t.Fatal("source preferences not read", selection, err)
	}
	for _, invalid := range []any{0, 4, "1"} {
		copy := configValues{"Current": map[string]any{"ProjectIdSource": 1, "AssignedSuSource": invalid}}
		if _, err := twoPageEntryReferenceSelectionFromConfig(copy); err == nil {
			t.Fatal("invalid source mode repaired", invalid)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readTwoPageEntryContextReference(cancelled, nil, nil, "", "", selection, twoPageEntryReferenceAliases{},
		provider.fields["ProjectID"]); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation repaired", err)
	}
	_, err = withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
			for _, aliases := range []twoPageEntryReferenceAliases{{"foreign", "VLists", "su"}, {"project", "foreign", "su"}} {
				if _, err := readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
					selection, aliases, provider.fields["ProjectID"]); err == nil {
					t.Fatal("foreign internal alias accepted")
				}
			}
			field := provider.fields["ProjectID"]
			field.maximum++
			if _, err := readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				selection, twoPageEntryReferenceAliases{"project", "VLists", "su"}, field); err == nil {
				t.Fatal("foreign policy accepted")
			}
			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				return false, err
			}
			defer db.Close()
			foreign, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				return false, err
			}
			defer foreign.Rollback()
			if _, err := readTwoPageEntryContextReference(context.Background(), foreign, owner, state.ContextID, "FS882-8x6XL",
				selection, twoPageEntryReferenceAliases{"main", "VLists", "su"}, provider.fields["ProjectID"]); err == nil {
				t.Fatal("unowned transaction accepted")
			}
			_, err = readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				selection, twoPageEntryReferenceAliases{"project", "VLists", "su"}, provider.fields["ProjectID"])
			return err == nil, err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := contextSelection(state)
	next.SU, next.SUPath = "None", ""
	state, err = contexts.SwitchContext(state.ContextID, next)
	if err != nil {
		t.Fatal(err)
	}
	_, err = withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
		return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				twoPageEntryReferenceSelection{1, 3}, twoPageEntryReferenceAliases{"project", "VLists", ""}, provider.fields["UserSiteUnit"])
			if err != nil || ref.Available || ref.Diagnostic == "" || len(ref.Choices) != 0 {
				t.Fatal("known unselected SU conflated with lookup failure/available source", ref, err)
			}
			return true, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTwoPageEntryContextReferenceMalformedWorkingUnitTextStopsAndRetries(t *testing.T) {
	for _, source := range []int{1, 3} {
		contexts, state, _, _, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
		provider, err := newTwoPageEntryReferenceProvider("FS882-8x6XL", twoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		path, broken, repaired := state.ProjectPath,
			`UPDATE Sample_Admin SET UserSiteUnit=CAST(X'FF' AS TEXT) WHERE Plot='108050'`,
			`UPDATE Sample_Admin SET UserSiteUnit='  Repaired literal  ' WHERE Plot='108050'`
		if source == 3 {
			path, broken, repaired = state.SUPath,
				`UPDATE Report_SU SET SiteUnit=CAST(X'FF' AS TEXT) WHERE PlotNumber='108050'`,
				`UPDATE Report_SU SET SiteUnit='  Repaired literal  ' WHERE PlotNumber='108050'`
		}
		mutateContextFixture(t, path, broken)
		read := func() (SIVIParentSharedReference, error) {
			return withContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *PlotService) (SIVIParentSharedReference, error) {
				return withOwnedSIVISnapshot(context.Background(), plots, func(owner *sqliteContext, tx *sql.Tx) (SIVIParentSharedReference, error) {
					return readTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
						twoPageEntryReferenceSelection{1, source}, twoPageEntryReferenceAliases{"project", "VLists", "su"}, provider.fields["UserSiteUnit"])
				})
			})
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		ref, err := read()
		if err == nil || ref.Available || !strings.Contains(err.Error(), "malformed UTF-8") {
			t.Fatal("malformed TEXT became success-shaped reference metadata", source, ref, err)
		}
		assertProfileSUFiles(t, contexts, before)
		mutateContextFixture(t, path, repaired)
		ref, err = read()
		if err != nil || !ref.Available {
			t.Fatal("typed error leaked snapshot/lease and prevented retry", source, ref, err)
		}
		found := false
		for _, choice := range ref.Choices {
			found = found || (choice.Code != nil && *choice.Code == "  Repaired literal  " && choice.Selectable)
		}
		if !found {
			t.Fatal("valid correction lost literal source value", ref)
		}
	}
}

func TestTwoPageEntryContextReferenceWriterAliasesAndHistoricalRestoration(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state, db, original, _ := twoPageWriteFixture(t, "FS882-8x6XL-CHARS", external)
		provider, err := newTwoPageEntryReferenceProvider(original.Form, twoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		var owner *sqliteContext
		lifecycle := twoPageEntryLifecycle{
			prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
				owner = plots.projects.sqlite
				for _, attachment := range []struct{ role, alias string }{{"VLists", "two_page_entry_refs"}, {"VMetaData", "VMetaData"}} {
					if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS `+quoteHeaderIdentifier(attachment.alias),
						sqliteFileURI(owner.attachments[attachment.role], "ro")); err != nil {
						return nil, err
					}
				}
				return func() {}, nil
			},
		}
		approve := func(tx *sql.Tx, observed *siviParentProjection, _ []siviParentScalarAssignment) error {
			alias := "main"
			if external {
				alias = "sivi_su"
			}
			for _, name := range []string{"ProjectID", "BECSiteUnit", "UserSiteUnit"} {
				for _, source := range []int{1, 2, 3} {
					_, err := readTwoPageEntryContextReference(context.Background(), tx, owner, observed.ContextID, observed.Form,
						twoPageEntryReferenceSelection{2, source}, twoPageEntryReferenceAliases{"main", "two_page_entry_refs", alias}, provider.fields[name])
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
		tables := twoPageWriteTables(t, db)
		references := databaseBytes(t, map[string]string{
			"VLists": contexts.projects.sqlite.attachments["VLists"], "VMetaData": contexts.projects.sqlite.attachments["VMetaData"],
		})
		result, err := contexts.writeTwoPageEntryWithLifecycle(context.Background(), state.ContextID, original.Plot, original.Form,
			original, []siviParentScalarEdit{siviParentEdit(t, original, "FieldNumber", metadataText("  reference reviewed  "))}, false, approve, lifecycle)
		if err != nil || result == nil || result.ChangedCells != 1 {
			t.Fatal("owned writer aliases could not read context sources", external, result, err)
		}
		if restored, err := contexts.restoreTwoPageEntry(context.Background(), state.ContextID, original.Plot, original.Form,
			result.HistoryID, AuditRestorePrune, false); err != nil || restored == nil {
			t.Fatal("unrelated typed entry restoration regressed", restored, err)
		}
		after := twoPageWriteTables(t, db)
		assertTwoPageEntryRestoredPhysicalTables(t, original.Form, tables, after)
		if !reflect.DeepEqual(references, databaseBytes(t, map[string]string{
			"VLists": contexts.projects.sqlite.attachments["VLists"], "VMetaData": contexts.projects.sqlite.attachments["VMetaData"],
		})) {
			t.Fatal("owned reference reads wrote support families")
		}
	}
}
