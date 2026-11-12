package integration

import (
	"github.com/boostao/vpro-wails/internal/sivi"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
			err := vprocore.ValidateSourceParentBooleanNull(column, test.originalNull, test.nextNull)
			if (err != nil) != test.refuse || err != nil && !strings.Contains(err.Error(), "select No explicitly") {
				t.Fatal("qualified new-write Boolean NULL policy", column, test, err)
			}
		}
	}
	for _, column := range []string{"UpdatedFromCards", "SiteNotes"} {
		if err := vprocore.ValidateSourceParentBooleanNull(column, false, true); err != nil {
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
						if _, err := db.Exec(`UPDATE Sample_Env SET `+vprocore.QuoteHeaderIdentifier(column)+`=? WHERE PlotNumber='108050'`, value); err != nil {
							t.Fatal(err)
						}
					}
					read := func() *sourceform.SiviParentProjection {
						t.Helper()
						parent, err := twopage.ReadTwoPageParent(service, context.Background(), state.ContextID, "108050", form)
						if err != nil {
							t.Fatal(err)
						}
						return parent
					}
					approvals := 0
					write := func(parent *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) (*sourceform.SiviParentWriteResult, error) {
						if complete {
							return twopage.WriteTwoPageEntry(service, context.Background(), state.ContextID, parent.Plot, form, parent, edits, false,
								func(*sql.Tx, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarAssignment) error {
									approvals++
									return nil
								})
						}
						return twopage.WriteTwoPageParentCommon(service, context.Background(), state.ContextID, parent.Plot, form, parent, edits)
					}
					restore := func(id string) (*vprocore.AuditRestoreResult, error) {
						if complete {
							return twopage.RestoreTwoPageEntry(service, context.Background(), state.ContextID, "108050", form, id, vprocore.AuditRestorePrune, false)
						}
						return twopage.RestoreTwoPageParentCommon(service, context.Background(), state.ContextID, "108050", form, id, vprocore.AuditRestorePrune)
					}
					setPhysical(0)
					parent := read()
					before := vprotest.TwoPageWriteTables(t, db)
					files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
					edits := []sourceform.SiviParentScalarEdit{
						sftest.SiviParentEdit(t, parent, "SV_StandHeight", vprotest.SiviReal(5.25)),
						sftest.SiviParentEdit(t, parent, column, vprocore.ProjectMetadataCell{Storage: "null"}),
					}
					result, err := write(parent, edits)
					if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") || approvals != 0 {
						t.Fatal("changed NULL reached approval or mutation", result, err, approvals)
					}
					if !reflect.DeepEqual(before, vprotest.TwoPageWriteTables(t, db)) {
						t.Fatal("mixed NULL refusal changed source, audit or history")
					}
					vprotest.AssertProfileSUFiles(t, service, files)

					setPhysical(nil)
					parent = read()
					before = vprotest.TwoPageWriteTables(t, db)
					result, err = write(parent, []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, column, vprocore.ProjectMetadataCell{Storage: "null"})})
					if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" ||
						!reflect.DeepEqual(before, vprotest.TwoPageWriteTables(t, db)) {
						t.Fatal("historical NULL was not omitted", result, err)
					}
					for _, integer := range []string{"-1", "0"} {
						parent = read()
						result, err = write(parent, []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, column, vprotest.MetadataInteger(integer))})
						if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
							t.Fatal("explicit Yes/No failed", integer, result, err)
						}
						current := sftest.SiviParentCell(t, read(), column)
						if current.Storage != "integer" || current.Integer == nil || *current.Integer != integer {
							t.Fatal("explicit Yes/No lost Access storage", current)
						}
						restored, err := restore(result.HistoryID)
						if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
							sftest.SiviParentCell(t, read(), column).Storage != "null" {
							t.Fatal("historical NULL could not be restored", restored, err)
						}
					}

					setPhysical(-1)
					parent = read()
					edits = []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, column, vprocore.ProjectMetadataCell{Storage: "null"})}
					history, err := twopage.TwoPageParentCommonHistory(form)
					if complete {
						history, err = twopage.TwoPageEntryHistory(form, false)
					}
					if err != nil {
						t.Fatal(err)
					}
					reader, err := twopage.TwoPageParentWriteReader(form)
					if err != nil {
						t.Fatal(err)
					}
					// Construct a pre-correction receipt through the unchanged historical planner.
					legacy, err := sourceform.WriteSIVIParentPlannedWithHooks(service, context.Background(), state.ContextID, parent.Plot, parent, history,
						func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
							if complete {
								return twopage.PlanTwoPageEntryProjection(context.Background(), observed, edits, false)
							}
							return twopage.PlanTwoPageParentCommonProjection(context.Background(), observed, edits)
						}, sourceform.SiviParentWriteHooks{Read: reader, Decorate: func(event *sourceform.SiviParentHistory) {
							if complete {
								event.EntryPlan = edits
							}
						}})
					if err != nil || legacy == nil || legacy.ChangedCells != 1 {
						t.Fatal("legacy NULL-after receipt could not be prepared", legacy, err)
					}
					restored, err := restore(legacy.HistoryID)
					if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
						!sourceform.SourceParentProjectionStorageEqual(parent, read()) {
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
			service, state, db, _, _ := sftest.SiviParentWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=0 WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			before := vprotest.TwoPageWriteTables(t, db)
			if action {
				option := 1
				edits := []sourceform.SiviParentActionEdit{
					{ContextID: state.ContextID, ControlID: "form:frmSIVIsite/optPlotType", Table: parent.AdminTable, RowID: parent.Rows[0].Admin.RowID,
						Expected: sftest.SiviParentCell(t, parent, "PlotType"), Option: &option},
					{ContextID: state.ContextID, ControlID: "form:frmSIVIsite/optSpeciesListComplete", Table: parent.EnvTable, RowID: parent.Rows[0].Env.RowID,
						Expected: sftest.SiviParentCell(t, parent, "SpeciesListComplete"), Option: nil},
				}
				result, err := sivi.WriteSIVIParentActions(service, context.Background(), state.ContextID, parent.Plot, parent, edits)
				if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
					t.Fatal("new source-action NULL did not refuse", result, err)
				}
			} else {
				height := 5.0
				edits := sourceform.SiviParentDirectEdits{Scalars: []sourceform.SiviParentScalarEdit{
					sftest.SiviParentEdit(t, parent, "SV_StandHeight", vprocore.ProjectMetadataCell{Storage: "real", Real: &height}),
					sftest.SiviParentEdit(t, parent, "SV_FloodPlain", vprocore.ProjectMetadataCell{Storage: "null"}),
				}}
				result, err := sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, parent.Plot, parent, edits)
				if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
					t.Fatal("new direct Boolean NULL did not refuse", result, err)
				}
			}
			if !reflect.DeepEqual(vprotest.TwoPageWriteTables(t, db), before) {
				t.Fatal("rejected mixed Boolean NULL changed source/audit tables")
			}
		})
	}
}
