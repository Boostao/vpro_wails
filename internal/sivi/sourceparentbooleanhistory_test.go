package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"strings"
	"testing"
)

func TestSourceParentBooleanNormalHistoricalNullAndLegacyReceipt(t *testing.T) {
	for _, action := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "action"}[action], func(t *testing.T) {
			service, state, db, _, _ := sftest.SiviParentWriteFixture(t, false, 3)
			column := "SV_FloodPlain"
			history := sourceform.SiviParentDirectHistory
			if action {
				column, history = "SpeciesListComplete", siviParentActionHistory
			}
			setPhysical := func(value any) {
				t.Helper()
				if _, err := db.Exec(`UPDATE Sample_Env SET `+vprocore.QuoteHeaderIdentifier(column)+`=? WHERE PlotNumber='108050'`, value); err != nil {
					t.Fatal(err)
				}
			}
			read := func() *sourceform.SiviParentProjection {
				t.Helper()
				parent, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				return parent
			}
			actionEdits := func(parent *sourceform.SiviParentProjection, option *int) []sourceform.SiviParentActionEdit {
				return []sourceform.SiviParentActionEdit{{ContextID: state.ContextID, ControlID: "form:frmSIVIsite/optSpeciesListComplete", Table: parent.EnvTable,
					RowID: parent.Rows[0].Env.RowID, Expected: sftest.SiviParentCell(t, parent, column), Option: option}}
			}
			directEdits := func(parent *sourceform.SiviParentProjection, option *int) sourceform.SiviParentDirectEdits {
				value := vprocore.ProjectMetadataCell{Storage: "null"}
				if option != nil {
					value = vprotest.MetadataInteger("0")
					if *option == 1 {
						value = vprotest.MetadataInteger("-1")
					}
				}
				return sourceform.SiviParentDirectEdits{Scalars: []sourceform.SiviParentScalarEdit{sftest.SiviParentEdit(t, parent, column, value)}}
			}
			write := func(parent *sourceform.SiviParentProjection, option *int) (*sourceform.SiviParentWriteResult, error) {
				if action {
					result, err := WriteSIVIParentActions(service, context.Background(), state.ContextID, parent.Plot, parent, actionEdits(parent, option))
					if result == nil {
						return nil, err
					}
					return &result.SiviParentWriteResult, err
				}
				return sourceform.WriteSIVIParentDirect(service, context.Background(), state.ContextID, parent.Plot, parent, directEdits(parent, option))
			}
			restore := func(id string) (*vprocore.AuditRestoreResult, error) {
				if action {
					return CtxRestoreSIVIParentActions(service, context.Background(), state.ContextID, "108050", id, vprocore.AuditRestorePrune)
				}
				return sourceform.CtxRestoreSIVIParentDirect(service, context.Background(), state.ContextID, "108050", id, vprocore.AuditRestorePrune)
			}
			setPhysical(nil)
			parent := read()
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			result, err := write(parent, nil)
			if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
				t.Fatal("historical NULL generated a new assignment/history", result, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			for _, choice := range []struct {
				option  int
				integer string
			}{{1, "-1"}, {2, "0"}} {
				result, err = write(read(), &choice.option)
				if err != nil || result == nil || result.ChangedCells != 1 || result.HistoryID == "" {
					t.Fatal("explicit source Yes/No failed", choice, result, err)
				}
				current := sftest.SiviParentCell(t, read(), column)
				if current.Storage != "integer" || current.Integer == nil || *current.Integer != choice.integer {
					t.Fatal("source Yes/No lost -1/0 physical storage", current)
				}
				restored, err := restore(result.HistoryID)
				if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
					sftest.SiviParentCell(t, read(), column).Storage != "null" {
					t.Fatal("original historical NULL could not be restored", restored, err)
				}
			}
			setPhysical(-1)
			parent = read()
			// Seed an old NULL-after receipt without using the corrected new-write entry point.
			legacy, err := sourceform.WriteSIVIParentPlanned(service, context.Background(), state.ContextID, parent.Plot, parent, history,
				func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
					if action {
						return planSIVIParentActionEdits(context.Background(), observed, actionEdits(observed, nil))
					}
					return sourceform.PlanSIVIParentDirectEdits(context.Background(), observed, directEdits(observed, nil))
				})
			if err != nil || legacy == nil || legacy.ChangedCells != 1 {
				t.Fatal("legacy NULL-after fixture could not be constructed", legacy, err)
			}
			restored, err := restore(legacy.HistoryID)
			if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 ||
				!sourceform.SourceParentProjectionStorageEqual(parent, read()) {
				t.Fatal("new-write guard invalidated old NULL-after history", restored, err)
			}
		})
	}
}

func TestSourceParentBooleanOrdinaryHistoricalOmissionAndMixedRefusal(t *testing.T) {
	service, db := vprotest.HeaderFixture(t)
	if err := service.SetAuditStrength(0); err != nil {
		t.Fatal(err)
	}
	if err := service.CreatePlot(vprocore.FS882Header{PlotNumber: "FLAG-HISTORY"}); err != nil {
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
	header.OfficeNotes = vprotest.QualityString("unrelated historical-NULL edit")
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
	header.OfficeNotes = vprotest.QualityString("must not commit")
	before := vprotest.SubstrateSnapshot(t, db)
	if err := service.UpdatePlot(*header); err == nil || !strings.Contains(err.Error(), "select No explicitly") {
		t.Fatal("mixed ordinary Species NULL was accepted", err)
	}
	vprotest.AssertSubstrateSnapshot(t, db, before)
	header.SpeciesListComplete = &flag
	if err := service.UpdatePlot(*header); err != nil {
		t.Fatal("valid correction could not retry the rejected ordinary draft", err)
	}
}
