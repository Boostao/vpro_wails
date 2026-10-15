package main

import (
	"context"
	"strings"
	"testing"
)

func TestSourceParentBooleanNormalHistoricalNullAndLegacyReceipt(t *testing.T) {
	for _, action := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "action"}[action], func(t *testing.T) {
			service, state, db, _, _ := siviParentWriteFixture(t, false, 3)
			column := "SV_FloodPlain"
			history := siviParentDirectHistory
			if action {
				column, history = "SpeciesListComplete", siviParentActionHistory
			}
			setPhysical := func(value any) {
				t.Helper()
				if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(column)+`=? WHERE PlotNumber='108050'`, value); err != nil {
					t.Fatal(err)
				}
			}
			read := func() *siviParentProjection {
				t.Helper()
				parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				return parent
			}
			actionEdits := func(parent *siviParentProjection, option *int) []siviParentActionEdit {
				return []siviParentActionEdit{{state.ContextID, "form:frmSIVIsite/optSpeciesListComplete", parent.EnvTable,
					parent.Rows[0].Env.RowID, siviParentCell(t, parent, column), option}}
			}
			directEdits := func(parent *siviParentProjection, option *int) siviParentDirectEdits {
				value := ProjectMetadataCell{Storage: "null"}
				if option != nil {
					value = metadataInteger("0")
					if *option == 1 {
						value = metadataInteger("-1")
					}
				}
				return siviParentDirectEdits{Scalars: []siviParentScalarEdit{siviParentEdit(t, parent, column, value)}}
			}
			write := func(parent *siviParentProjection, option *int) (*siviParentWriteResult, error) {
				if action {
					result, err := service.writeSIVIParentActions(context.Background(), state.ContextID, parent.Plot, parent, actionEdits(parent, option))
					if result == nil {
						return nil, err
					}
					return &result.siviParentWriteResult, err
				}
				return service.writeSIVIParentDirect(context.Background(), state.ContextID, parent.Plot, parent, directEdits(parent, option))
			}
			restore := func(id string) (*AuditRestoreResult, error) {
				if action {
					return service.restoreSIVIParentActions(context.Background(), state.ContextID, "108050", id, AuditRestorePrune)
				}
				return service.restoreSIVIParentDirect(context.Background(), state.ContextID, "108050", id, AuditRestorePrune)
			}
			setPhysical(nil)
			parent := read()
			before := databaseBytes(t, service.projects.sqlite.attachments)
			result, err := write(parent, nil)
			if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
				t.Fatal("historical NULL generated a new assignment/history", result, err)
			}
			assertProfileSUFiles(t, service, before)
			for _, choice := range []struct {
				option  int
				integer string
			}{{1, "-1"}, {2, "0"}} {
				result, err = write(read(), &choice.option)
				if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
					t.Fatal("explicit source Yes/No failed", choice, result, err)
				}
				current := siviParentCell(t, read(), column)
				if current.Storage != "integer" || current.Integer == nil || *current.Integer != choice.integer {
					t.Fatal("source Yes/No lost -1/0 physical storage", current)
				}
				restored, err := restore(result.HistoryID)
				if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
					siviParentCell(t, read(), column).Storage != "null" {
					t.Fatal("original historical NULL could not be restored", restored, err)
				}
			}
			setPhysical(-1)
			parent = read()
			// Seed an old NULL-after receipt without using the corrected new-write entry point.
			legacy, err := service.writeSIVIParentPlanned(context.Background(), state.ContextID, parent.Plot, parent, history,
				func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
					if action {
						return planSIVIParentActionEdits(context.Background(), observed, actionEdits(observed, nil))
					}
					return planSIVIParentDirectEdits(context.Background(), observed, directEdits(observed, nil))
				})
			if err != nil || legacy == nil || legacy.ChangedCells != 1 {
				t.Fatal("legacy NULL-after fixture could not be constructed", legacy, err)
			}
			restored, err := restore(legacy.HistoryID)
			if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
				!sourceParentProjectionStorageEqual(parent, read()) {
				t.Fatal("new-write guard invalidated old NULL-after history", restored, err)
			}
		})
	}
}

func TestSourceParentBooleanOrdinaryHistoricalOmissionAndMixedRefusal(t *testing.T) {
	service, db := headerFixture(t)
	if err := service.SetAuditStrength(0); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePlot(FS882Header{PlotNumber: "FLAG-HISTORY"}); err != nil {
		t.Fatal(err)
	}
	if err := service.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=NULL WHERE PlotNumber='FLAG-HISTORY';
		CREATE TRIGGER historical_null_omit BEFORE UPDATE OF SpeciesListComplete ON Sample_Env
		BEGIN SELECT RAISE(ABORT,'historical NULL assigned'); END`); err != nil {
		t.Fatal(err)
	}
	header, err := service.GetPlot("FLAG-HISTORY")
	if err != nil || header == nil || header.SpeciesListComplete != nil {
		t.Fatal("historical NULL read was repaired", header, err)
	}
	header.OfficeNotes = qualityString("unrelated historical-NULL edit")
	if err := service.UpdatePlot(*header); err != nil {
		t.Fatal("historical NULL was assigned during an unrelated edit", err)
	}
	if _, err := db.Exec(`DROP TRIGGER historical_null_omit`); err != nil {
		t.Fatal(err)
	}
	flag := false
	header.SpeciesListComplete = &flag
	if err := service.UpdatePlot(*header); err != nil {
		t.Fatal("explicit No correction failed", err)
	}
	header.SpeciesListComplete = nil
	header.OfficeNotes = qualityString("must not commit")
	before := substrateSnapshot(t, db)
	if err := service.UpdatePlot(*header); err == nil || !strings.Contains(err.Error(), "select No explicitly") {
		t.Fatal("mixed ordinary Species NULL was accepted", err)
	}
	assertSubstrateSnapshot(t, db, before)
	header.SpeciesListComplete = &flag
	if err := service.UpdatePlot(*header); err != nil {
		t.Fatal("valid correction could not retry the rejected ordinary draft", err)
	}
}
