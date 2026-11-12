package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
		vprotest.MutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Metadata;
			INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle) VALUES
			(-7,'PROJECT','  literal title  '),(2,'PROJECT',NULL),(3,NULL,''),(4,'',NULL)`)
		vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["VMetaData"], `DELETE FROM ProjectMetadata;
			INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES
			('MASTER','  literal title  '),('MASTER2',NULL),(NULL,''),('',NULL)`)
		provider, err := twopage.NewTwoPageEntryReferenceProvider(form, twopage.TwoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		for _, source := range []int{1, 2} {
			_, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
				return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
					ref, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, form,
						twopage.TwoPageEntryReferenceSelection{ProjectSource: source, WorkingSource: 1}, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}, provider.Fields["ProjectID"])
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
						!reflect.DeepEqual(ref.Definitions.Rows[0].Cells[1], vprotest.MetadataText("  literal title  ")) ||
						ref.Definitions.Rows[1].Cells[1].Storage != "null" ||
						!reflect.DeepEqual(ref.Definitions.Rows[2].Cells[1], vprotest.MetadataText("")) ||
						!reflect.DeepEqual(ref.Definitions.Rows[3].Cells[0], vprotest.MetadataText("")) {
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
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
}

func TestTwoPageEntryContextReferenceUnitsFromOwnedCurrentSources(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state, _, _, _ := twoPageWriteFixture(t, "FS882-8x6XL", external)
		vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["VLists"], `DELETE FROM MasterSiteUnitList;
			INSERT INTO MasterSiteUnitList(ID,SiteSeries,SiteSeriesLongName,SiteSeriesScientificName,Level) VALUES
			(1,'  Master literal  ',NULL,'scientific',11),(2,'  Master literal  ','','scientific',11),
			(3,NULL,NULL,NULL,11),(4,'',NULL,NULL,11),(5,'HIDDEN','hidden',NULL,12)`)
		vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Admin SET UserSiteUnit='  Env literal  ';
			INSERT INTO Sample_Admin(Plot,UserSiteUnit) VALUES('orphan','ORPHAN')`)
		vprotest.MutateContextFixture(t, state.SUPath, `DELETE FROM Report_SU;
			INSERT INTO Report_SU(PlotNumber,SiteUnit) VALUES
			('108050','  SU literal  '),('108051','  SU literal  '),('108052',NULL),('108053','')`)
		provider, err := twopage.NewTwoPageEntryReferenceProvider("FS882-8x6XL", twopage.TwoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		for _, name := range []string{"BECSiteUnit", "UserSiteUnit"} {
			for _, source := range []int{1, 2, 3} {
				_, err := vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
					return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
						ref, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
							twopage.TwoPageEntryReferenceSelection{ProjectSource: 1, WorkingSource: source}, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}, provider.Fields[name])
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
							byID := map[string]vprocore.ProjectMetadataRow{}
							for _, row := range ref.Definitions.Rows {
								byID[row.RowID] = row
							}
							if len(byID) != 4 || !reflect.DeepEqual(byID["1"].Cells[1], vprotest.MetadataText("  Master literal  ")) ||
								byID["1"].Cells[2].Storage != "null" || !reflect.DeepEqual(byID["2"].Cells[2], vprotest.MetadataText("")) {
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
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
}

func TestTwoPageEntryContextReferenceGuardsCancellationNoSUAndRetry(t *testing.T) {
	contexts, state, _, _, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	provider, err := twopage.NewTwoPageEntryReferenceProvider("FS882-8x6XL", twopage.TwoPageEntryReferenceReaders{})
	if err != nil {
		t.Fatal(err)
	}
	values, err := contexts.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	selection, err := twopage.TwoPageEntryReferenceSelectionFromConfig(values)
	if err != nil || selection.ProjectSource != 2 || selection.WorkingSource != 1 {
		t.Fatal("source preferences not read", selection, err)
	}
	for _, invalid := range []any{0, 4, "1"} {
		copy := vprocore.ConfigValues{"Current": map[string]any{"ProjectIdSource": 1, "AssignedSuSource": invalid}}
		if _, err := twopage.TwoPageEntryReferenceSelectionFromConfig(copy); err == nil {
			t.Fatal("invalid source mode repaired", invalid)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := twopage.ReadTwoPageEntryContextReference(cancelled, nil, nil, "", "", selection, twopage.TwoPageEntryReferenceAliases{},
		provider.Fields["ProjectID"]); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation repaired", err)
	}
	_, err = vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
			for _, aliases := range []twopage.TwoPageEntryReferenceAliases{{Project: "foreign", Lists: "VLists", Su: "su"}, {Project: "project", Lists: "foreign", Su: "su"}} {
				if _, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
					selection, aliases, provider.Fields["ProjectID"]); err == nil {
					t.Fatal("foreign internal alias accepted")
				}
			}
			field := provider.Fields["ProjectID"]
			field.Maximum++
			if _, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				selection, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}, field); err == nil {
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
			if _, err := twopage.ReadTwoPageEntryContextReference(context.Background(), foreign, owner, state.ContextID, "FS882-8x6XL",
				selection, twopage.TwoPageEntryReferenceAliases{Project: "main", Lists: "VLists", Su: "su"}, provider.Fields["ProjectID"]); err == nil {
				t.Fatal("unowned transaction accepted")
			}
			_, err = twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				selection, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}, provider.Fields["ProjectID"])
			return err == nil, err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := vprotest.ContextSelection(state)
	next.SU, next.SUPath = "None", ""
	state, err = contexts.SwitchContext(state.ContextID, next)
	if err != nil {
		t.Fatal(err)
	}
	_, err = vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
		return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
			ref, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
				twopage.TwoPageEntryReferenceSelection{ProjectSource: 1, WorkingSource: 3}, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: ""}, provider.Fields["UserSiteUnit"])
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
		provider, err := twopage.NewTwoPageEntryReferenceProvider("FS882-8x6XL", twopage.TwoPageEntryReferenceReaders{})
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
		vprotest.MutateContextFixture(t, path, broken)
		read := func() (sourceform.SIVIParentSharedReference, error) {
			return vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (sourceform.SIVIParentSharedReference, error) {
				return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (sourceform.SIVIParentSharedReference, error) {
					return twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, state.ContextID, "FS882-8x6XL",
						twopage.TwoPageEntryReferenceSelection{ProjectSource: 1, WorkingSource: source}, twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}, provider.Fields["UserSiteUnit"])
				})
			})
		}
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		ref, err := read()
		if err == nil || ref.Available || !strings.Contains(err.Error(), "malformed UTF-8") {
			t.Fatal("malformed TEXT became success-shaped reference metadata", source, ref, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
		vprotest.MutateContextFixture(t, path, repaired)
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
		provider, err := twopage.NewTwoPageEntryReferenceProvider(original.Form, twopage.TwoPageEntryReferenceReaders{})
		if err != nil {
			t.Fatal(err)
		}
		var owner *vprocore.SqliteContext
		lifecycle := twopage.TwoPageEntryLifecycle{
			Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
				owner = plots.Projects.Sqlite
				for _, attachment := range []struct{ role, alias string }{{"VLists", "two_page_entry_refs"}, {"VMetaData", "VMetaData"}} {
					if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS `+vprocore.QuoteHeaderIdentifier(attachment.alias),
						sqlutil.SqliteFileURI(owner.Attachments[attachment.role], "ro")); err != nil {
						return nil, err
					}
				}
				return func() {}, nil
			},
		}
		approve := func(tx *sql.Tx, observed *sourceform.SiviParentProjection, _ []sourceform.SiviParentScalarAssignment) error {
			alias := "main"
			if external {
				alias = "sivi_su"
			}
			for _, name := range []string{"ProjectID", "BECSiteUnit", "UserSiteUnit"} {
				for _, source := range []int{1, 2, 3} {
					_, err := twopage.ReadTwoPageEntryContextReference(context.Background(), tx, owner, observed.ContextID, observed.Form,
						twopage.TwoPageEntryReferenceSelection{ProjectSource: 2, WorkingSource: source}, twopage.TwoPageEntryReferenceAliases{Project: "main", Lists: "two_page_entry_refs", Su: alias}, provider.Fields[name])
					if err != nil {
						return err
					}
				}
			}
			return nil
		}
		tables := vprotest.TwoPageWriteTables(t, db)
		references := vprotest.DatabaseBytes(t, map[string]string{
			"VLists": contexts.Projects.Sqlite.Attachments["VLists"], "VMetaData": contexts.Projects.Sqlite.Attachments["VMetaData"],
		})
		result, err := twopage.WriteTwoPageEntryWithLifecycle(contexts, context.Background(), state.ContextID, original.Plot, original.Form,
			original, []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, original, "FieldNumber", vprotest.MetadataText("  reference reviewed  "))}, false, approve, lifecycle)
		if err != nil || result == nil || result.ChangedCells != 1 {
			t.Fatal("owned writer aliases could not read context sources", external, result, err)
		}
		if restored, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, original.Plot, original.Form,
			result.HistoryID, vprocore.AuditRestorePrune, false); err != nil || restored == nil {
			t.Fatal("unrelated typed entry restoration regressed", restored, err)
		}
		after := vprotest.TwoPageWriteTables(t, db)
		assertTwoPageEntryRestoredPhysicalTables(t, original.Form, tables, after)
		if !reflect.DeepEqual(references, vprotest.DatabaseBytes(t, map[string]string{
			"VLists": contexts.Projects.Sqlite.Attachments["VLists"], "VMetaData": contexts.Projects.Sqlite.Attachments["VMetaData"],
		})) {
			t.Fatal("owned reference reads wrote support families")
		}
	}
}
