package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

func TestSIVICHARSSpeciesActionHistoricalNullNoOpAndRestoration(t *testing.T) {
	for _, option := range []int{1, 2} {
		service, state, db, _ := charsWriteFixture(t, false, 3)
		if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=NULL WHERE PlotNumber='108050'`); err != nil {
			t.Fatal(err)
		}
		original, err := ReadSIVICHARSParent(service, context.Background(), state.ContextID, "108050")
		if err != nil {
			t.Fatal(err)
		}
		written, err := WriteSIVICHARSParentProposal(service, context.Background(), state.ContextID, original.Plot, original,
			nil, []sourceform.SiviParentActionEdit{charsSpeciesAction(t, original, &option)}, charsWriteApprovePhysical, sourceform.SourceParentEntryLifecycle{})
		if err != nil || written == nil || written.ChangedCells != 1 || written.HistoryID == "" {
			t.Fatal("historical NULL could not receive an explicit source choice", written, err)
		}
		current, err := ReadSIVICHARSParent(service, context.Background(), state.ContextID, original.Plot)
		want := "-1"
		if option == 2 {
			want = "0"
		}
		if err != nil || !vprocore.MetadataCellEqual(sftest.SiviParentCell(t, current, "SpeciesListComplete"), vprotest.MetadataInteger(want)) {
			t.Fatal("source BOOLEAN storage changed", err)
		}
		beforeNoOp := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		unchanged, err := WriteSIVICHARSParentProposal(service, context.Background(), state.ContextID, original.Plot, current,
			nil, []sourceform.SiviParentActionEdit{charsSpeciesAction(t, current, &option)}, charsWriteApprovePhysical, sourceform.SourceParentEntryLifecycle{})
		if err != nil || unchanged == nil || unchanged.ChangedCells != 0 || unchanged.HistoryID != "" {
			t.Fatal("display/same option manufactured a mutation receipt", unchanged, err)
		}
		vprotest.AssertProfileSUFiles(t, service, beforeNoOp)
		var histories int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_SIVICHARSParentEntryHistory"`).Scan(&histories); err != nil || histories != 1 {
			t.Fatal("same source choice manufactured history", histories, err)
		}
		restored, err := RestoreSIVICHARSParentEntry(service, context.Background(), state.ContextID, original.Plot, written.HistoryID, vprocore.AuditRestorePrune)
		if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
			t.Fatal("source action could not restore original historical NULL", restored, err)
		}
		final, err := ReadSIVICHARSParent(service, context.Background(), state.ContextID, original.Plot)
		if err != nil || !sourceform.SourceParentProjectionStorageEqual(original, final) {
			t.Fatal("source action restoration lost historical NULL or changed a companion", err)
		}
	}
}

func TestSIVICHARSSpeciesActionApprovalCancellationRollsBackAndRetries(t *testing.T) {
	service, state, _, original := charsWriteFixture(t, false, 3)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	option := 1
	action := charsSpeciesAction(t, original, &option)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	written, err := WriteSIVICHARSParentProposal(service, ctx, state.ContextID, original.Plot, original,
		[]sourceform.SiviParentScalarEdit{charsEntryEdit(t, original, "SV_WaterTableCM", vprotest.SiviReal(.1))}, []sourceform.SiviParentActionEdit{action},
		func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			if err := charsWriteApprovePhysical(tx, observed, assignments); err != nil {
				return err
			}
			cancel()
			return nil
		}, sourceform.SourceParentEntryLifecycle{})
	if !errors.Is(err, context.Canceled) || written != nil {
		t.Fatal("cancellation during mixed source approval committed", written, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	written, err = WriteSIVICHARSParentProposal(service, context.Background(), state.ContextID, original.Plot, original,
		nil, []sourceform.SiviParentActionEdit{action}, charsWriteApprovePhysical, sourceform.SourceParentEntryLifecycle{})
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("cancelled source action retained a stale lease/transaction", written, err)
	}
}

func TestSIVICHARSSpeciesActionHistoryRejectsReinterpretedFullPlans(t *testing.T) {
	service, state, db, original := charsWriteFixture(t, false, 3)
	option := 1
	written, err := WriteSIVICHARSParentProposal(service, context.Background(), state.ContextID, original.Plot, original,
		nil, []sourceform.SiviParentActionEdit{charsSpeciesAction(t, original, &option)}, charsWriteApprovePhysical, sourceform.SourceParentEntryLifecycle{})
	if err != nil || written == nil || written.HistoryID == "" {
		t.Fatal("source action history fixture failed", written, err)
	}
	event := charsWrittenHistory(t, db, written.HistoryID)
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"NULL", "invalid", "extra-storage", "duplicate", "row", "context", "normal"} {
		t.Run(kind, func(t *testing.T) {
			var forged sourceform.SiviParentHistory
			if err := json.Unmarshal(raw, &forged); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "NULL":
				forged.EntryPlan[0].Value = vprocore.ProjectMetadataCell{Storage: "null"}
			case "invalid":
				forged.EntryPlan[0].Value = vprotest.MetadataInteger("7")
			case "extra-storage":
				forged.EntryPlan[0].Value.Text = vprotest.MetadataText("ignored").Text
			case "duplicate":
				forged.EntryPlan = append(forged.EntryPlan, forged.EntryPlan[0])
			case "row":
				forged.EntryPlan[0].RowID = "foreign"
			case "context":
				forged.EntryPlan[0].ContextID = "foreign"
			case "normal":
				forged.Original.Form = "frmSIVIsite"
			}
			if plan, err := siviCHARSParentEntryHistoryAssignments(context.Background(), forged, original.Project, original.Plot); err == nil || plan != nil {
				t.Fatal("reinterpreted/foreign source action full plan accepted", plan, err)
			}
		})
	}
}
