package main

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestSourceParentBooleanNullQualification(t *testing.T) {
	for _, column := range []string{"SV_FloodPlain", "SpeciesListComplete"} {
		for _, test := range []struct{ originalNull, nextNull, refuse bool }{
			{false, false, false}, {false, true, true}, {true, false, false}, {true, true, false},
		} {
			err := validateSourceParentBooleanNull(column, test.originalNull, test.nextNull)
			if (err != nil) != test.refuse || err != nil && !strings.Contains(err.Error(), "select No explicitly") {
				t.Fatal("qualified new-write Boolean NULL policy", column, test, err)
			}
		}
	}
	for _, column := range []string{"UpdatedFromCards", "SiteNotes"} {
		if err := validateSourceParentBooleanNull(column, false, true); err != nil {
			t.Fatal("an unqualified field inherited the source Boolean policy", column, err)
		}
	}
}

func TestSourceParentBooleanTwoPageWriteBoundariesAndLegacyRestoration(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		for _, complete := range []bool{false, true} {
			columns := []string{"SV_FloodPlain"}
			if complete {
				columns = append(columns, "SpeciesListComplete")
			}
			for _, column := range columns {
				t.Run(form+map[bool]string{false: "-common-", true: "-entry-"}[complete]+column, func(t *testing.T) {
					service, state, db, _, _ := twoPageWriteFixture(t, form, false)
					setPhysical := func(value any) {
						t.Helper()
						if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(column)+`=? WHERE PlotNumber='108050'`, value); err != nil {
							t.Fatal(err)
						}
					}
					read := func() *siviParentProjection {
						t.Helper()
						parent, err := service.readTwoPageParent(context.Background(), state.ContextID, "108050", form)
						if err != nil {
							t.Fatal(err)
						}
						return parent
					}
					approvals := 0
					write := func(parent *siviParentProjection, edits []siviParentScalarEdit) (*siviParentWriteResult, error) {
						if complete {
							return service.writeTwoPageEntry(context.Background(), state.ContextID, parent.Plot, form, parent, edits, false,
								func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error { approvals++; return nil })
						}
						return service.writeTwoPageParentCommon(context.Background(), state.ContextID, parent.Plot, form, parent, edits)
					}
					restore := func(id string) (*AuditRestoreResult, error) {
						if complete {
							return service.restoreTwoPageEntry(context.Background(), state.ContextID, "108050", form, id, AuditRestorePrune, false)
						}
						return service.restoreTwoPageParentCommon(context.Background(), state.ContextID, "108050", form, id, AuditRestorePrune)
					}
					setPhysical(0)
					parent := read()
					before := twoPageWriteTables(t, db)
					files := databaseBytes(t, service.projects.sqlite.attachments)
					edits := []siviParentScalarEdit{
						siviParentEdit(t, parent, "SV_StandHeight", siviReal(5.25)),
						siviParentEdit(t, parent, column, ProjectMetadataCell{Storage: "null"}),
					}
					result, err := write(parent, edits)
					if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") || approvals != 0 {
						t.Fatal("changed NULL reached approval or mutation", result, err, approvals)
					}
					if !reflect.DeepEqual(before, twoPageWriteTables(t, db)) {
						t.Fatal("mixed NULL refusal changed source, audit or history")
					}
					assertProfileSUFiles(t, service, files)

					setPhysical(nil)
					parent = read()
					before = twoPageWriteTables(t, db)
					result, err = write(parent, []siviParentScalarEdit{siviParentEdit(t, parent, column, ProjectMetadataCell{Storage: "null"})})
					if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" ||
						!reflect.DeepEqual(before, twoPageWriteTables(t, db)) {
						t.Fatal("historical NULL was not omitted", result, err)
					}
					for _, integer := range []string{"-1", "0"} {
						parent = read()
						result, err = write(parent, []siviParentScalarEdit{siviParentEdit(t, parent, column, metadataInteger(integer))})
						if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
							t.Fatal("explicit Yes/No failed", integer, result, err)
						}
						current := siviParentCell(t, read(), column)
						if current.Storage != "integer" || current.Integer == nil || *current.Integer != integer {
							t.Fatal("explicit Yes/No lost Access storage", current)
						}
						restored, err := restore(result.HistoryID)
						if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
							siviParentCell(t, read(), column).Storage != "null" {
							t.Fatal("historical NULL could not be restored", restored, err)
						}
					}

					setPhysical(-1)
					parent = read()
					edits = []siviParentScalarEdit{siviParentEdit(t, parent, column, ProjectMetadataCell{Storage: "null"})}
					history, err := twoPageParentCommonHistory(form)
					if complete {
						history, err = twoPageEntryHistory(form, false)
					}
					if err != nil {
						t.Fatal(err)
					}
					reader, err := twoPageParentWriteReader(form)
					if err != nil {
						t.Fatal(err)
					}
					// Construct a pre-correction receipt through the unchanged historical planner.
					legacy, err := service.writeSIVIParentPlannedWithHooks(context.Background(), state.ContextID, parent.Plot, parent, history,
						func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
							if complete {
								return planTwoPageEntryProjection(context.Background(), observed, edits, false)
							}
							return planTwoPageParentCommonProjection(context.Background(), observed, edits)
						}, siviParentWriteHooks{read: reader, decorate: func(event *siviParentHistory) {
							if complete {
								event.EntryPlan = edits
							}
						}})
					if err != nil || legacy == nil || legacy.ChangedCells != 1 {
						t.Fatal("legacy NULL-after receipt could not be prepared", legacy, err)
					}
					restored, err := restore(legacy.HistoryID)
					if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
						!sourceParentProjectionStorageEqual(parent, read()) {
						t.Fatal("new-write guard reinterpreted old NULL-after history", restored, err)
					}
				})
			}
		}
	}
}
func TestSourceParentBooleanNormalWritersRefuseMixedNullAtomically(t *testing.T) {
	for _, action := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "source-action"}[action], func(t *testing.T) {
			service, state, db, _, _ := siviParentWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=0 WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			before := twoPageWriteTables(t, db)
			if action {
				option := 1
				edits := []siviParentActionEdit{
					{state.ContextID, "form:frmSIVIsite/optPlotType", parent.AdminTable, parent.Rows[0].Admin.RowID,
						siviParentCell(t, parent, "PlotType"), &option},
					{state.ContextID, "form:frmSIVIsite/optSpeciesListComplete", parent.EnvTable, parent.Rows[0].Env.RowID,
						siviParentCell(t, parent, "SpeciesListComplete"), nil},
				}
				result, err := service.writeSIVIParentActions(context.Background(), state.ContextID, parent.Plot, parent, edits)
				if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
					t.Fatal("new source-action NULL did not refuse", result, err)
				}
			} else {
				height := 5.0
				edits := siviParentDirectEdits{Scalars: []siviParentScalarEdit{
					siviParentEdit(t, parent, "SV_StandHeight", ProjectMetadataCell{Storage: "real", Real: &height}),
					siviParentEdit(t, parent, "SV_FloodPlain", ProjectMetadataCell{Storage: "null"}),
				}}
				result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, parent.Plot, parent, edits)
				if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
					t.Fatal("new direct Boolean NULL did not refuse", result, err)
				}
			}
			if !reflect.DeepEqual(twoPageWriteTables(t, db), before) {
				t.Fatal("rejected mixed Boolean NULL changed source/audit tables")
			}
		})
	}
}
