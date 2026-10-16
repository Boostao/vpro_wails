package main

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
		original, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
		if err != nil {
			t.Fatal(err)
		}
		written, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, original.Plot, original,
			nil, []siviParentActionEdit{charsSpeciesAction(t, original, &option)}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
		if err != nil || written == nil || written.ChangedCells != 1 || written.HistoryID == "" {
			t.Fatal("historical NULL could not receive an explicit source choice", written, err)
		}
		current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, original.Plot)
		want := "-1"
		if option == 2 {
			want = "0"
		}
		if err != nil || !metadataCellEqual(siviParentCell(t, current, "SpeciesListComplete"), metadataInteger(want)) {
			t.Fatal("source BOOLEAN storage changed", err)
		}
		beforeNoOp := databaseBytes(t, service.projects.sqlite.attachments)
		unchanged, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, original.Plot, current,
			nil, []siviParentActionEdit{charsSpeciesAction(t, current, &option)}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
		if err != nil || unchanged == nil || unchanged.ChangedCells != 0 || unchanged.HistoryID != "" {
			t.Fatal("display/same option manufactured a mutation receipt", unchanged, err)
		}
		assertProfileSUFiles(t, service, beforeNoOp)
		var histories int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_SIVICHARSParentEntryHistory"`).Scan(&histories); err != nil || histories != 1 {
			t.Fatal("same source choice manufactured history", histories, err)
		}
		restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, original.Plot, written.HistoryID, AuditRestorePrune)
		if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
			t.Fatal("source action could not restore original historical NULL", restored, err)
		}
		final, err := service.readSIVICHARSParent(context.Background(), state.ContextID, original.Plot)
		if err != nil || !sourceParentProjectionStorageEqual(original, final) {
			t.Fatal("source action restoration lost historical NULL or changed a companion", err)
		}
	}
}

func TestSIVICHARSSpeciesActionApprovalCancellationRollsBackAndRetries(t *testing.T) {
	service, state, _, original := charsWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	option := 1
	action := charsSpeciesAction(t, original, &option)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	written, err := service.writeSIVICHARSParentProposal(ctx, state.ContextID, original.Plot, original,
		[]siviParentScalarEdit{charsEntryEdit(t, original, "SV_WaterTableCM", siviReal(.1))}, []siviParentActionEdit{action},
		func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
			if err := charsWriteApprovePhysical(tx, observed, assignments); err != nil {
				return err
			}
			cancel()
			return nil
		}, sourceParentEntryLifecycle{})
	if !errors.Is(err, context.Canceled) || written != nil {
		t.Fatal("cancellation during mixed source approval committed", written, err)
	}
	assertProfileSUFiles(t, service, before)
	written, err = service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, original.Plot, original,
		nil, []siviParentActionEdit{action}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("cancelled source action retained a stale lease/transaction", written, err)
	}
}

func TestSIVICHARSSpeciesActionHistoryRejectsReinterpretedFullPlans(t *testing.T) {
	service, state, db, original := charsWriteFixture(t, false, 3)
	option := 1
	written, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, original.Plot, original,
		nil, []siviParentActionEdit{charsSpeciesAction(t, original, &option)}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
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
			var forged siviParentHistory
			if err := json.Unmarshal(raw, &forged); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "NULL":
				forged.EntryPlan[0].Value = ProjectMetadataCell{Storage: "null"}
			case "invalid":
				forged.EntryPlan[0].Value = metadataInteger("7")
			case "extra-storage":
				forged.EntryPlan[0].Value.Text = metadataText("ignored").Text
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
