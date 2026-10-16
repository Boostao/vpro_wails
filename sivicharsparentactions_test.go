package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func charsSpeciesAction(t *testing.T, original *siviParentProjection, option *int) siviParentActionEdit {
	t.Helper()
	target := charsEntryEdit(t, original, "SpeciesListComplete", ProjectMetadataCell{Storage: "null"})
	return siviParentActionEdit{target.ContextID, "form:frmSIVIsite-CHARS/optSpeciesListComplete",
		target.Table, target.RowID, target.Expected, option}
}

func TestSIVICHARSSpeciesActionSourceOptionsAndScope(t *testing.T) {
	original := charsEntryOriginal(t)
	for _, option := range []int{1, 2} {
		action := charsSpeciesAction(t, original, &option)
		plan, err := planSIVICHARSParentProposal(context.Background(), original, nil, []siviParentActionEdit{action})
		want := "-1"
		if option == 2 {
			want = "0"
		}
		if err != nil || len(plan) != 1 || plan[0].Column != "SpeciesListComplete" ||
			plan[0].Table != "Sample_Env" || !metadataCellEqual(plan[0].After, metadataInteger(want)) {
			t.Fatal("source1/2 lost exact implicit BOOLEAN target", plan, err)
		}
	}
	for _, kind := range []string{"NULL", "zero", "three", "normal", "PlotType", "table", "row", "context", "expected", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			option := 1
			action := charsSpeciesAction(t, original, &option)
			switch kind {
			case "NULL":
				action.Option = nil
			case "zero":
				option = 0
			case "three":
				option = 3
			case "normal":
				action.ControlID = "form:frmSIVIsite/optSpeciesListComplete"
			case "PlotType":
				action.ControlID = "form:frmSIVIsite-CHARS/optPlotType"
			case "table":
				action.Table = original.AdminTable
			case "row":
				action.RowID = "foreign"
			case "context":
				action.ContextID = "foreign"
			case "expected":
				action.Expected = metadataInteger("0")
			}
			actions := []siviParentActionEdit{action}
			if kind == "duplicate" {
				actions = append(actions, action)
			}
			if plan, err := planSIVICHARSParentProposal(context.Background(), original, nil, actions); err == nil || plan != nil {
				t.Fatal("unsupported/foreign action planned", plan, err)
			}
		})
	}
	for _, column := range []string{"SpeciesListComplete", "PlotType"} {
		if plan, err := planSIVICHARSParentProposal(context.Background(), original,
			[]siviParentScalarEdit{charsEntryEdit(t, original, column, metadataInteger("-1"))}, nil); err == nil || plan != nil {
			t.Fatal("implicit source action acquired a direct writer", column, plan, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if plan, err := planSIVICHARSParentProposal(ctx, original, nil, nil); !errors.Is(err, context.Canceled) || plan != nil {
		t.Fatal("cancelled action plan returned success", plan, err)
	}
}

func TestSIVICHARSSpeciesActionMixedAtomicHistoryAndAuditedSubset(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, strength := range []int{1, 2, 3} {
			for _, option := range []int{1, 2} {
				t.Run(fmt.Sprintf("external%t/strength%d/option%d", external, strength, option), func(t *testing.T) {
					service, state, db, _ := charsWriteFixture(t, external, strength)
					initial, next := "-1", "0"
					if option == 1 {
						initial, next = "0", "-1"
					}
					if _, err := db.Exec(`UPDATE Sample_Env SET SpeciesListComplete=?,SV_WaterTableCM=NULL WHERE PlotNumber='108050'`, initial); err != nil {
						t.Fatal(err)
					}
					parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
					if err != nil {
						t.Fatal(err)
					}
					written, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, parent.Plot, parent,
						[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))},
						[]siviParentActionEdit{charsSpeciesAction(t, parent, &option)}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
					if err != nil || written == nil || written.ChangedCells != 2 || written.HistoryID == "" {
						t.Fatal("mixed direct/action Save failed", written, err)
					}
					event := charsWrittenHistory(t, db, written.HistoryID)
					audited := 2
					if strength == 1 {
						audited = 1
					}
					if len(event.EntryPlan) != 2 || len(event.Changes) != audited ||
						!metadataCellEqual(siviParentCell(t, event.Committed, "SpeciesListComplete"), metadataInteger(next)) {
						t.Fatal("complete plan or audited subset changed", event)
					}
					restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestorePrune)
					if err != nil || restored == nil || restored.RestoredRows != audited || restored.PrunedAuditRows != audited {
						t.Fatal("audited source action restoration failed", restored, err)
					}
					current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
					wantWater := ProjectMetadataCell{Storage: "null"}
					if strength == 1 {
						wantWater = siviReal(0.10000000149011612)
					}
					if err != nil || !metadataCellEqual(siviParentCell(t, current, "SpeciesListComplete"), metadataInteger(initial)) ||
						!metadataCellEqual(siviParentCell(t, current, "SV_WaterTableCM"), wantWater) {
						t.Fatal("restoration changed unaudited addition or source original", err)
					}
				})
			}
		}
	}
}

func TestSIVICHARSSpeciesActionRefusalBeforeApprovalAndRetry(t *testing.T) {
	service, state, _, parent := charsWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	option := 1
	action := charsSpeciesAction(t, parent, &option)
	action.ControlID = "form:frmSIVIsite-CHARS/optPlotType"
	approved := false
	if result, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, parent.Plot, parent,
		[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))}, []siviParentActionEdit{action},
		func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error { approved = true; return nil },
		sourceParentEntryLifecycle{}); err == nil || result != nil || approved {
		t.Fatal("unsupported action reached approval or committed direct peer", result, err)
	}
	assertProfileSUFiles(t, service, before)
	action = charsSpeciesAction(t, parent, &option)
	written, err := service.writeSIVICHARSParentProposal(context.Background(), state.ContextID, parent.Plot, parent,
		nil, []siviParentActionEdit{action}, charsWriteApprovePhysical, sourceParentEntryLifecycle{})
	if err != nil || written == nil || written.ChangedCells != 1 {
		t.Fatal("explicit source action retry failed", written, err)
	}
	restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("source original restoration failed", restored, err)
	}
	current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
	if err != nil || !sourceParentProjectionStorageEqual(parent, current) {
		t.Fatal("source action retry lost complete original", err)
	}
}

func TestSIVICHARSSpeciesActionStrictWire(t *testing.T) {
	original := charsEntryOriginal(t)
	option := 1
	action := charsSpeciesAction(t, original, &option)
	request := SIVICHARSParentEntryWrite{Original: original, Edits: []SIVIParentCellEdit{},
		Actions: []SIVIParentActionInput{{action.ContextID, action.ControlID, action.Table, action.RowID, action.Expected, &option}}}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SIVICHARSParentEntryWrite
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded.Actions) != 1 {
		t.Fatal("source action Go wire did not round trip", err)
	}
	valid := string(raw)
	for _, malformed := range []string{
		strings.Replace(valid, `"actions":`, `"actions":[],"actions":`, 1),
		strings.Replace(valid, `"actions":`, `"actions":[],"\u0061ctions":`, 1),
		strings.Replace(valid, `"option":1`, `"option":1,"option":2`, 1),
		strings.Replace(valid, `"option":1`, `"option":1,"\u006fption":2`, 1),
		strings.Replace(valid, `"option":1`, `"option":1,"Option":2`, 1),
		strings.Replace(valid, `"option":1`, `"option":null`, 1),
		strings.Replace(valid, `"option":1`, `"Option":1`, 1),
		strings.Replace(valid, `"option":1`, `"unknown":1`, 1),
		strings.Replace(valid, `"controlId":`, `"controlId":"ignored","controlId":`, 1),
		strings.Replace(valid, `"controlId":`, `"ControlID":"ignored","controlId":`, 1),
		strings.Replace(valid, `"actions":`, `"Actions":`, 1),
		strings.Replace(valid, `"controlId":"form:frmSIVIsite-CHARS/optSpeciesListComplete"`, `"controlId":"\ud800"`, 1),
	} {
		if err := json.Unmarshal([]byte(malformed), &decoded); err == nil {
			t.Fatal("duplicate/unknown/malformed source action wire accepted", malformed)
		}
	}
}

func TestSIVICHARSSpeciesActionOwnedServiceMixedRequestAndIndependentGate(t *testing.T) {
	service, state, review := charsEntryServiceFixture(t)
	option := 1
	if metadataCellEqual(siviParentCell(t, review.Original, "SpeciesListComplete"), metadataInteger("-1")) {
		option = 2
	}
	action := charsSpeciesAction(t, review.Original, &option)
	request := SIVICHARSParentEntryWrite{Original: review.Original,
		Edits: []SIVIParentCellEdit{charsMandatoryEdit(t, review, "Exposure1")},
		Actions: []SIVIParentActionInput{
			{action.ContextID, action.ControlID, action.Table, action.RowID, action.Expected, action.Option},
		}}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	disabled, err := newSIVICHARSParentEntryService(service.contexts, service.readers, false)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := disabled.Save(context.Background(), state.ContextID, review.Original.Plot, request); err == nil || result != nil {
		t.Fatal("new source action bypassed independent parent gate", result, err)
	}
	assertProfileSUFiles(t, service.contexts, before)
	written, err := service.Save(context.Background(), state.ContextID, review.Original.Plot, request)
	if err != nil || written == nil || written.ChangedCells != 2 || written.HistoryID == "" {
		t.Fatal("actual owned service failed mixed source action", written, err)
	}
	restored, err := service.Restore(context.Background(), state.ContextID, review.Original.Plot, written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 2 || restored.PrunedAuditRows != 2 {
		t.Fatal("owned mixed source restoration failed", restored, err)
	}
	current, err := service.Review(context.Background(), state.ContextID, review.Original.Plot)
	if err != nil || !sourceParentProjectionStorageEqual(review.Original, current.Original) {
		t.Fatal("owned source action changed original references/storage", err)
	}
}
