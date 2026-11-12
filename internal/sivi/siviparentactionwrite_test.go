package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func siviParentActionWriteFixture(t *testing.T, external bool, strength int) (*vprocore.ContextService, vprocore.ProjectState, *sourceform.SiviParentProjection, []sourceform.SiviParentActionEdit) {
	t.Helper()
	service, state, db, _, _ := sftest.SiviParentWriteFixture(t, external, strength)
	if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050';
		UPDATE Sample_Admin SET PlotType='legacy' WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, parent, sftest.SiviActionProposalEdits(parent)
}

func TestSIVIParentActionWriterSourceChoicesTypedRestorationAndIsolation(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(strconv.FormatBool(external), func(t *testing.T) {
			service, state, original, actions := siviParentActionWriteFixture(t, external, 3)
			for plotOption := 1; plotOption <= 5; plotOption++ {
				for speciesOption := 0; speciesOption <= 2; speciesOption++ {
					t.Run(strconv.Itoa(plotOption)+"/"+strconv.Itoa(speciesOption), func(t *testing.T) {
						edits := append([]sourceform.SiviParentActionEdit{}, actions...)
						option := plotOption
						edits[0].Option = &option
						edits[1].Option = nil
						if speciesOption != 0 {
							value := speciesOption
							edits[1].Option = &value
						}
						prior, err := service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
						if err != nil {
							t.Fatal(err)
						}
						filesBefore := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
						result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, original, edits)
						if speciesOption == 0 {
							if err == nil || result != nil || !strings.Contains(err.Error(), "select No explicitly") {
								t.Fatal("new source species NULL action was accepted", result, err)
							}
							vprotest.AssertProfileSUFiles(t, service, filesBefore)
							return
						}
						if err != nil || result == nil || result.ChangedCells != 2 || result.HistoryID == "" || !result.SourceRefreshRequired {
							t.Fatal("source action pair failed", result, err)
						}
						fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, original.Plot)
						if err != nil {
							t.Fatal(err)
						}
						gotActions := sftest.SiviActionProposalEdits(fresh)
						wantPlot := []string{"Ground", "Visual", "Note", "FS882", "Other"}[plotOption-1]
						wantSpecies := vprocore.ProjectMetadataCell{Storage: "null"}
						if speciesOption != 0 {
							truth := "-1"
							if speciesOption == 2 {
								truth = "0"
							}
							wantSpecies = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &truth}
						}
						if !reflect.DeepEqual(gotActions[0].Expected, vprocore.ProjectMetadataCell{Storage: "text", Text: &wantPlot}) ||
							!reflect.DeepEqual(gotActions[1].Expected, wantSpecies) {
							t.Fatal("source action storage mapping changed", gotActions)
						}
						audits, err := service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
						if err != nil || len(audits) != len(prior)+2 {
							t.Fatal("action audits changed inherited evidence", audits, err)
						}
						files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
						if result, err := sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune); err == nil || result != nil {
							t.Fatal("direct history endpoint restored source actions", result, err)
						}
						vprotest.AssertProfileSUFiles(t, service, files)
						restored, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune)
						if err != nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 || restored.CleanedVegRows != 0 {
							t.Fatal("action audited-cell restoration failed", restored, err)
						}
						after, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, original.Plot)
						if err != nil || !reflect.DeepEqual(after, original) {
							t.Fatal("historical action originals were normalized/lost", after, err)
						}
						audits, err = service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
						if err != nil || !reflect.DeepEqual(audits, prior) {
							t.Fatal("action prune changed historical audits", audits, err)
						}
						files = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
						if _, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestoreRetain); err == nil {
							t.Fatal("restored action history replay accepted")
						}
						vprotest.AssertProfileSUFiles(t, service, files)
					})
				}
			}
		})
	}
}

func TestSIVIParentActionWriterDomainsStrengthsAndSourceRefresh(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		for mask := 1; mask < 4; mask++ {
			t.Run(strconv.Itoa(strength)+"/"+strconv.Itoa(mask), func(t *testing.T) {
				service, state, original, actions := siviParentActionWriteFixture(t, false, strength)
				edits := []sourceform.SiviParentActionEdit{}
				for index, action := range actions {
					if mask&(1<<index) != 0 {
						edits = append(edits, action)
					}
				}
				before, err := service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
				if err != nil {
					t.Fatal(err)
				}
				result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, original, edits)
				if err != nil || result == nil || result.ChangedCells != len(edits) || result.SourceRefreshRequired != (mask&1 != 0) {
					t.Fatal("action subset/refresh changed", result, err)
				}
				audits, err := service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
				if err != nil {
					t.Fatal(err)
				}
				if strength == 0 {
					if result.HistoryID != "" || !reflect.DeepEqual(audits, before) {
						t.Fatal("strength0 manufactured action history/audits", result, audits)
					}
					return
				}
				if len(audits) != len(before)+len(edits) || result.HistoryID == "" {
					t.Fatal("action strengths lost original-value audits", result, audits)
				}
				restored, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestoreRetain)
				if err != nil || restored.RestoredRows != len(edits) || restored.PrunedAuditRows != 0 {
					t.Fatal("action retain failed", restored, err)
				}
				retained, err := service.ListAuditEntries(context.Background(), state.ContextID, original.Plot)
				if err != nil {
					t.Fatal(err)
				}
				priorIDs := map[string]bool{}
				for _, audit := range before {
					priorIDs[audit.RowID] = true
				}
				for _, audit := range retained {
					if !priorIDs[audit.RowID] && (!audit.Restore || audit.ID != nil) {
						t.Fatal("action retain lost true/NULL-child identity", audit)
					}
				}
			})
		}
	}
}

func TestSIVIParentActionWriterRejectionsAndNoop(t *testing.T) {
	service, state, original, actions := siviParentActionWriteFixture(t, false, 3)
	for name, mutate := range map[string]func([]sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit{
		"none": func([]sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit { return nil },
		"plot-null": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			edits[0].Option = nil
			return edits
		},
		"invalid-tail": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			v := 3
			edits[1].Option = &v
			return edits
		},
		"repeated": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			return append(edits, edits[0])
		},
		"direct-control": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			edits[1].ControlID = "form:frmSIVIsite/SV_FloodPlain"
			return edits
		},
		"foreign-table": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			edits[0].Table = original.EnvTable
			return edits
		},
		"stale-cell": func(edits []sourceform.SiviParentActionEdit) []sourceform.SiviParentActionEdit {
			edits[1].Expected = vprocore.ProjectMetadataCell{Storage: "null"}
			return edits
		},
	} {
		t.Run(name, func(t *testing.T) {
			files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			edits := mutate(append([]sourceform.SiviParentActionEdit{}, actions...))
			if result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, original, edits); err == nil || result != nil {
				t.Fatal("unverified source action accepted", result, err)
			}
			vprotest.AssertProfileSUFiles(t, service, files)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := WriteSIVIParentActions(service, ctx, state.ContextID, original.Plot, original, actions); err == nil || result != nil {
		t.Fatal("cancelled source action accepted", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
	result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, original, actions)
	if err != nil {
		t.Fatal(err)
	}
	current, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, original.Plot)
	if err != nil {
		t.Fatal(err)
	}
	files = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	noop, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, current.Plot, current, sftest.SiviActionProposalEdits(current))
	if err != nil || noop.ChangedCells != 0 || noop.HistoryID != "" || !noop.SourceRefreshRequired {
		t.Fatal("PlotType source no-op lost refresh directive or manufactured history", noop, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
	if _, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, current.Plot, result.HistoryID, vprocore.AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
}

func TestSIVIParentActionHistoryRejectsDirectTargetAndWrongSourceValue(t *testing.T) {
	service, state, original, actions := siviParentActionWriteFixture(t, false, 3)
	result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, original, actions)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, original.Plot)
	if err != nil {
		t.Fatal(err)
	}
	event := sourceform.SiviParentHistory{Original: original, Committed: committed, Changes: []sourceform.SiviParentHistoryChange{}}
	planned, err := planSIVIParentActionEdits(context.Background(), original, actions)
	if err != nil {
		t.Fatal(err)
	}
	for _, assignment := range planned {
		event.Changes = append(event.Changes, sourceform.SiviParentHistoryChange{Table: assignment.Table, RowID: assignment.RowID,
			Column: assignment.Column, Before: assignment.Before, After: assignment.After})
	}
	for name, mutate := range map[string]func(*sourceform.SiviParentHistory){
		"direct-target": func(event *sourceform.SiviParentHistory) { event.Changes[0].Column = "SV_StandHeight" },
		"plot-null": func(event *sourceform.SiviParentHistory) {
			event.Changes[0].After = vprocore.ProjectMetadataCell{Storage: "null"}
		},
		"species-one": func(event *sourceform.SiviParentHistory) {
			value := "1"
			event.Changes[1].After = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
		},
		"repeated": func(event *sourceform.SiviParentHistory) { event.Changes = append(event.Changes, event.Changes[0]) },
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			var candidate sourceform.SiviParentHistory
			if err := json.Unmarshal(data, &candidate); err != nil {
				t.Fatal(err)
			}
			mutate(&candidate)
			if _, err := siviParentActionHistoryAssignments(context.Background(), candidate, original.Project, original.Plot); err == nil {
				t.Fatal("malformed action history gained source authorization")
			}
		})
	}
	if _, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune); err != nil {
		t.Fatal(err)
	}
}

func TestSIVIParentActionWriterRollbackAndRetry(t *testing.T) {
	for _, failure := range []string{"second-audit", "second-blob", "stale-unrelated-cell"} {
		t.Run(failure, func(t *testing.T) {
			service, state, db, _, _ := sftest.SiviParentWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050';
				UPDATE Sample_Admin SET PlotType='legacy' WHERE Plot='108050'`); err != nil {
				t.Fatal(err)
			}
			if failure == "second-audit" {
				if _, err := db.Exec(`CREATE TRIGGER action_audit_abort BEFORE INSERT ON Sample_Audit
					WHEN NEW.EditField='SpeciesListComplete' BEGIN SELECT RAISE(ABORT,'action audit abort'); END`); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "second-blob" {
				if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=X'DEAD' WHERE PlotNumber='108050'`); err != nil {
					t.Fatal(err)
				}
			}
			parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			actions := sftest.SiviActionProposalEdits(parent)
			if failure == "stale-unrelated-cell" {
				if _, err := db.Exec(`UPDATE Sample_Env SET FieldNumber='changed after original' WHERE PlotNumber='108050'`); err != nil {
					t.Fatal(err)
				}
			}
			files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, parent.Plot, parent, actions); err == nil || result != nil {
				t.Fatal("action failure committed a staged first action", result, err)
			}
			vprotest.AssertProfileSUFiles(t, service, files)
			if failure == "second-audit" {
				if _, err := db.Exec("DROP TRIGGER action_audit_abort"); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "second-blob" {
				if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=2 WHERE PlotNumber='108050'`); err != nil {
					t.Fatal(err)
				}
			}
			parent, err = sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, parent.Plot)
			if err != nil {
				t.Fatal(err)
			}
			result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, parent.Plot, parent, sftest.SiviActionProposalEdits(parent))
			if err != nil || result.ChangedCells != 2 {
				t.Fatal("corrected action batch could not retry", result, err)
			}
			if _, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, parent.Plot, result.HistoryID, vprocore.AuditRestorePrune); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSIVIParentActionLegacyRestorationPreservesUnauditedAssignments(t *testing.T) {
	for _, strength := range []int{1, 2} {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, original, actions := siviParentActionWriteFixture(t, false, strength)
			actions[1].Option = nil
			// Old NULL-after receipts retain their original audited subset, not new-write authority.
			result, err := sourceform.WriteSIVIParentPlanned(service, context.Background(), state.ContextID, original.Plot, original,
				siviParentActionHistory, func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
					return planSIVIParentActionEdits(context.Background(), observed, actions)
				})
			if err != nil || result.ChangedCells != 2 || result.HistoryID == "" {
				t.Fatal("mixed audited/unaudited action pair failed", result, err)
			}
			restored, err := CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, original.Plot, result.HistoryID, vprocore.AuditRestorePrune)
			if err != nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
				t.Fatal("unaudited action deletion became restorable", restored, err)
			}
			fresh, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, original.Plot)
			if err != nil {
				t.Fatal(err)
			}
			fields := sftest.SiviActionProposalEdits(fresh)
			if !reflect.DeepEqual(fields[0].Expected, actions[0].Expected) || fields[1].Expected.Storage != "null" {
				t.Fatal("action restore recovered unaudited original storage", fields)
			}
		})
	}
}
