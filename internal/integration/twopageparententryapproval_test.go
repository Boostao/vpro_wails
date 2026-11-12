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
	"os"
	"reflect"
	"testing"
)

func TestTwoPageEntryFixedApprovalAtomicWriterRollbackRetryAndRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		snapshot, err := twopage.ReadTwoPageEntryReferences(contexts, context.Background(), state.ContextID, "108050", form,
			vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"), provider.Readers)
		if err != nil {
			t.Fatal(err)
		}
		edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "ro"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		originalTables := vprotest.TwoPageWriteTables(t, db)
		var owner *vprocore.SqliteContext
		lifecycle := twopage.TwoPageEntryLifecycle{
			Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
				owner = plots.Projects.Sqlite
				_, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS two_page_entry_refs`,
					sqlutil.SqliteFileURI(owner.Attachments["VLists"], "ro"))
				if err != nil {
					return nil, err
				}
				return func() {}, nil
			},
			Verify: func(_ *vprocore.PlotService, _ *sql.Tx) error { return vprocore.ProfileOwnedFiles(owner) },
		}
		write := func(acks []twopage.TwoPageEntryCodeAcknowledgementInternal) (*sourceform.SiviParentWriteResult, error) {
			return twopage.WriteTwoPageEntryWithLifecycle(contexts, context.Background(), state.ContextID, "108050", form,
				snapshot.Original, edits, false,
				func(tx *sql.Tx, original *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
					return twopage.ApproveTwoPageEntryReferences(context.Background(), tx, owner, provider, state.ContextID, original,
						assignments, acks, twopage.TwoPageEntryReferenceSelection{ProjectSource: 2, WorkingSource: 1}, twopage.TwoPageEntryReferenceAliases{Project: "main", Lists: "two_page_entry_refs", Su: ""})
				}, lifecycle)
		}
		if result, err := write(nil); err == nil || result != nil {
			t.Fatal("missing explicit optional approval leaked a mixed commit", result, err)
		}
		vprotest.AssertProfileSUFiles(t, contexts, before)
		result, err := write(acknowledgements)
		if err != nil || result == nil || result.HistoryID == "" || result.ChangedCells != len(edits) {
			t.Fatal("fixed concrete approval could not retry one atomic mixed writer transaction", form, result, err)
		}
		observed, err := twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		for _, edit := range edits {
			if !reflect.DeepEqual(sftest.SiviParentCell(t, observed, edit.Column), edit.Value) {
				t.Fatal("fixed approval changed literal source assignment", edit.Column)
			}
		}
		restored, err := twopage.RestoreTwoPageEntry(contexts, context.Background(), state.ContextID, "108050", form, result.HistoryID,
			vprocore.AuditRestorePrune, false)
		if err != nil || restored == nil || restored.RestoredRows != len(edits) {
			t.Fatal("fixed reference approval could not restore literal mixed history", restored, err)
		}
		observed, err = twopage.ReadTwoPageParent(contexts, context.Background(), state.ContextID, "108050", form)
		if err != nil || !reflect.DeepEqual(observed, snapshot.Original) {
			t.Fatal("fixed reference restoration changed physical source original", observed, err)
		}
		after := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		restoredTables := vprotest.TwoPageWriteTables(t, db)
		assertTwoPageEntryRestoredPhysicalTables(t, form, originalTables, restoredTables)
		for role, bytes := range before {
			if !os.SameFile(owner.AttachmentInfo[role], owner.AttachmentInfo["project"]) && !reflect.DeepEqual(after[role], bytes) {
				t.Fatal("fixed approval writer/restoration changed an owned reference family", role)
			}
		}
	}
}

func TestTwoPageEntryFixedApprovalCurrentPlannedFiltersMembershipAndAcknowledgement(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
		fresh, err := twopage.ReadTwoPageEntryReferences(contexts, context.Background(), state.ContextID, "108050", form,
			vprotest.MetadataText("BG"), vprotest.MetadataText("xh1"), provider.Readers)
		if err != nil {
			t.Fatal(err)
		}

		edits, acknowledgements := twoPageEntryApprovalDraft(t, fresh)
		assignments, err := twopage.PlanTwoPageEntryProjection(context.Background(), fresh.Original, edits, false)
		if err != nil {
			t.Fatal(err)
		}
		var currentSeries sourceform.SIVIParentSharedReference
		for _, acknowledgement := range acknowledgements {
			if acknowledgement.Column == "SiteSeries" {
				currentSeries = acknowledgement.Reference
			}
		}
		if len(currentSeries.Definitions.Rows) == 0 {
			t.Fatal("independent BG/xh1 catalogue fixture lacks filtered definitions")
		}
		_, err = vprocore.WithContextPlotRequest(context.Background(), contexts, state.ContextID, func(plots *vprocore.PlotService) (bool, error) {
			return vprocore.WithOwnedSIVISnapshot(context.Background(), plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (bool, error) {
				for _, projectSource := range []int{1, 2} {
					for _, workingSource := range []int{1, 2, 3} {
						if err := twopage.ApproveTwoPageEntryReferences(context.Background(), tx, owner, provider, state.ContextID,
							fresh.Original, assignments, acknowledgements, twopage.TwoPageEntryReferenceSelection{ProjectSource: projectSource, WorkingSource: workingSource},
							twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}); err != nil {
							t.Fatal("fixed mandatory/optional draft-filter composition failed", form, projectSource, workingSource, err)
						}
					}
				}
				stale, err := provider.Read(context.Background(), tx, "VLists", provider.Fields["SiteSeries"],
					vprocore.ProjectMetadataCell{Storage: "null"}, vprocore.ProjectMetadataCell{Storage: "null"})
				if err != nil || reflect.DeepEqual(stale, currentSeries) {
					t.Fatal("old original filter fixture is not independently distinct", err)
				}
				for _, failure := range []string{"missing acknowledgement", "old BEC pair", "mandatory nonmember", "forged assignment",
					"foreign context", "foreign form", "foreign aliases", "bad source", "unused acknowledgement", "project mutation", "cancelled"} {
					current := append([]sourceform.SiviParentScalarAssignment{}, assignments...)
					acks := append([]twopage.TwoPageEntryCodeAcknowledgementInternal{}, acknowledgements...)
					original := *fresh.Original
					contextID := state.ContextID
					aliases := twopage.TwoPageEntryReferenceAliases{Project: "project", Lists: "VLists", Su: "su"}
					selection := twopage.TwoPageEntryReferenceSelection{ProjectSource: 2, WorkingSource: 1}
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
						bad := append([]sourceform.SiviParentScalarEdit{}, edits...)
						for index := range bad {
							if bad[index].Column == "MesoSlopePosition" {
								bad[index].Value = vprotest.MetadataText("ZZ")
							}
						}
						current, err = twopage.PlanTwoPageEntryProjection(ctx, &original, bad, false)
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
						aliases.Lists = "project"
					case "bad source":
						selection.WorkingSource = 4
					case "unused acknowledgement":
						current = nil
					case "project mutation":
						projectEdit := sftest.SiviParentEdit(t, &original, "ProjectID", vprotest.MetadataText("new"))
						current = append(current, sourceform.SiviParentScalarAssignment{ContextID: projectEdit.ContextID, Table: projectEdit.Table,
							RowID: projectEdit.RowID, Column: "ProjectID", Before: projectEdit.Expected, After: projectEdit.Value, Value: "new"})
					case "cancelled":
						cancelled, cancel := context.WithCancel(ctx)
						cancel()
						ctx = cancelled
					}
					err := twopage.ApproveTwoPageEntryReferences(ctx, tx, owner, provider, contextID, &original, current, acks, selection, aliases)
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
		vprotest.AssertProfileSUFiles(t, contexts, before)
	}
}
