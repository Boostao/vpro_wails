package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func charsWriteFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, *siviParentProjection) {
	t.Helper()
	service, state, db, _, _ := siviParentWriteFixture(t, external, strength)
	parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db, parent
}

func charsWriteApprovePhysical(tx *sql.Tx, original *siviParentProjection, _ []siviParentScalarAssignment) error {
	if tx == nil || original == nil || original.Form != "frmSIVIsite-CHARS" || len(original.Rows) != 1 {
		return errors.New("missing exact owned CHARS transaction/source")
	}
	return nil
}

func charsWrittenHistory(t *testing.T, db *sql.DB, id string) siviParentHistory {
	t.Helper()
	var raw string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICHARSParentEntryHistory" WHERE ID=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var event siviParentHistory
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestSIVICHARSBooleanNewNullRefusesBeforeApprovalAndMutation(t *testing.T) {
	for _, initial := range []string{"0", "-1"} {
		t.Run(initial, func(t *testing.T) {
			service, state, db, _ := charsWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET SV_FloodPlain=? WHERE PlotNumber='108050'`, initial); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			var beforeAudit, afterAudit int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&beforeAudit); err != nil {
				t.Fatal(err)
			}
			approved := false
			result, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, parent,
				[]siviParentScalarEdit{charsEntryEdit(t, parent, "RootZoneParticleSize", metadataText("FINE")),
					charsEntryEdit(t, parent, "SV_FloodPlain", ProjectMetadataCell{Storage: "null"})},
				func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error { approved = true; return nil })
			if err == nil || result != nil || approved {
				t.Fatal("new Boolean NULL reached approval or mutation", result, err, approved)
			}
			current, readErr := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
			if readErr != nil || !sourceParentProjectionStorageEqual(parent, current) {
				t.Fatal("rejected mixed Boolean NULL changed physical storage", readErr)
			}
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit`).Scan(&afterAudit); err != nil || beforeAudit != afterAudit {
				t.Fatal("rejected Boolean NULL changed audits", err, beforeAudit, afterAudit)
			}
			var histories int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='__VPRO_SIVICHARSParentEntryHistory'`).Scan(&histories); err != nil || histories != 0 {
				t.Fatal("rejected Boolean NULL created history", histories, err)
			}
		})
	}
}

func TestSIVICHARSBooleanHistoricalNullOmissionYesNoAndRestoration(t *testing.T) {
	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(t *testing.T) {
			service, state, db, _ := charsWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET SV_FloodPlain=NULL WHERE PlotNumber='108050'`); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			unchanged, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, parent,
				[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_FloodPlain", ProjectMetadataCell{Storage: "null"})},
				charsWriteApprovePhysical)
			if err != nil || unchanged == nil || unchanged.ChangedCells != 0 || unchanged.HistoryID != "" {
				t.Fatal("unchanged historical NULL was reinterpreted", unchanged, err)
			}
			written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, parent,
				[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_FloodPlain", metadataInteger(value))}, charsWriteApprovePhysical)
			if err != nil || written == nil || written.ChangedCells != 1 || written.HistoryID == "" {
				t.Fatal("explicit Yes/No was not writable", written, err)
			}
			restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestorePrune)
			if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
				t.Fatal("historical NULL restoration was treated as a new write", restored, err)
			}
			current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
			if err != nil || !sourceParentProjectionStorageEqual(parent, current) {
				t.Fatal("Yes/No restoration lost original NULL", err)
			}
		})
	}
}

func TestSIVICHARSBooleanLegacyNullAfterHistoryRemainsRestorable(t *testing.T) {
	service, state, db, _ := charsWriteFixture(t, false, 3)
	if _, err := db.Exec(`UPDATE Sample_Env SET SV_FloodPlain=-1 WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []siviParentScalarEdit{charsEntryEdit(t, parent, "SV_FloodPlain", ProjectMetadataCell{Storage: "null"})}
	legacy, err := service.writeSIVIParentPlannedWithHooks(context.Background(), state.ContextID, parent.Plot, parent,
		siviCHARSParentEntryHistory, nil, siviParentWriteHooks{
			read: readSIVICHARSParentForWrite,
			plan: func(_ *sql.Tx, observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
				return planSIVICHARSParentWritableEntry(context.Background(), observed, edits)
			},
			decorate: func(event *siviParentHistory) { event.EntryPlan = edits },
		})
	if err != nil || legacy == nil || legacy.ChangedCells != 1 {
		t.Fatal("legacy NULL-after fixture could not be constructed", legacy, err)
	}
	restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, legacy.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 {
		t.Fatal("new-write guard reinterpreted the legacy NULL-after receipt", restored, err)
	}
	current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
	if err != nil || !sourceParentProjectionStorageEqual(parent, current) {
		t.Fatal("legacy NULL-after receipt lost original Yes", err)
	}
}

func TestSIVICHARSWriteAllQualifiedDomainsPhysicalHistoryAndRestoration(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state, db, parent := charsWriteFixture(t, external, 3)
			edits := []siviParentScalarEdit{}
			expected := map[string]ProjectMetadataCell{}
			for _, field := range charsEntrySourceFields(t) {
				if field.Column == "PlotNumber" || field.Column == "ProjectID" || field.Column == "PlotType" ||
					field.Column == "SpeciesListComplete" || field.Column == "SubZone" {
					continue
				}
				value := ProjectMetadataCell{}
				switch field.Type {
				case 1:
					value = metadataInteger("-1")
				case 3:
					value = metadataInteger("-2")
					if field.Column == "StartDate" {
						value = metadataInteger("2020")
					}
				case 6:
					value = siviReal(.1)
					expected[field.Column] = siviReal(0.10000000149011612)
				case 7:
					value = siviReal(.1)
				case 8:
					value = metadataText("2026-10-15 12:00:00")
				case 10, 12:
					value = metadataText("Z")
					if field.Column == "SV_StandAgeEstMeas" || field.Column == "SV_StandHeightEstMeas" {
						value = metadataText("2")
					}
				default:
					t.Fatal("unqualified source field type", field)
				}
				if _, present := expected[field.Column]; !present {
					expected[field.Column] = value
				}
				edits = append(edits, charsEntryEdit(t, parent, field.Column, value))
			}
			if len(edits) != 67 {
				t.Fatal("physical source-domain cohort changed", len(edits))
			}
			written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, parent, edits, charsWriteApprovePhysical)
			if err != nil || written == nil || written.ChangedCells != 67 || written.HistoryID == "" {
				t.Fatal("mixed CHARS domains failed atomic publication", written, err)
			}
			current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
			if err != nil {
				t.Fatal(err)
			}
			for column, cell := range expected {
				if !metadataCellEqual(charsEntryEdit(t, current, column, ProjectMetadataCell{Storage: "null"}).Expected, cell) {
					t.Fatal("physical source conversion/value changed", column)
				}
			}
			event := charsWrittenHistory(t, db, written.HistoryID)
			if len(event.EntryPlan) != 67 || len(event.Changes) != 67 || !sourceParentProjectionStorageEqual(event.Committed, current) {
				t.Fatal("prepared full plan/history did not match physical source")
			}
			for _, change := range event.Changes {
				alias := "_eNv"
				if change.Table == parent.AdminTable {
					alias = "sAmPlE_aDmIn"
				}
				if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, change.Audit.RowID); err != nil {
					t.Fatal(err)
				}
			}
			cancelled, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestoreCancel)
			if err != nil || cancelled == nil || !cancelled.Cancelled {
				t.Fatal("restoration cancel failed", cancelled, err)
			}
			action := AuditRestorePrune
			if external {
				action = AuditRestoreRetain
			}
			restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, action)
			if err != nil || restored == nil || restored.RestoredRows != 67 {
				t.Fatal("CHARS typed restoration failed", restored, err)
			}
			after, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
			if err != nil || !sourceParentProjectionStorageEqual(after, parent) {
				t.Fatal("restoration changed unrelated physical source", err)
			}
			if replay, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, action); err == nil || replay != nil {
				t.Fatal("completed restoration was replayable", replay, err)
			}
			var normalHistory int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='__VPRO_SIVIParentHistory'`).Scan(&normalHistory); err != nil || normalHistory != 0 {
				t.Fatal("CHARS published normal-form provenance", normalHistory, err)
			}
		})
	}
}

func TestSIVICHARSWriteRefusalsRollbackAndRetry(t *testing.T) {
	for _, kind := range []string{"nil-approval", "approval-error", "cancel-in-flight", "invalid-tail", "stale",
		"foreign-form", "SubZone", "blank-horizon", "negative-zero", "StartDate", "ProjectID", "PlotType", "SpeciesListComplete"} {
		t.Run(kind, func(t *testing.T) {
			service, state, _, parent := charsWriteFixture(t, false, 3)
			edits := []siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))}
			approve := siviCHARSParentEntryApproval(charsWriteApprovePhysical)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("independent CHARS approval refusal")
			contextID := state.ContextID
			switch kind {
			case "nil-approval":
				approve = nil
			case "approval-error":
				approve = func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error { return failure }
			case "cancel-in-flight":
				approve = func(*sql.Tx, *siviParentProjection, []siviParentScalarAssignment) error { cancel(); return ctx.Err() }
			case "invalid-tail":
				edits = append(edits, charsEntryEdit(t, parent, "RootZoneParticleSize", metadataText("TOO LONG")))
			case "stale":
				contextID = "stale"
			case "foreign-form":
				parent.Form = "frmSIVIsite"
			case "SubZone":
				edits = append(edits, charsEntryEdit(t, parent, "SubZone", metadataText("Z")))
			case "blank-horizon":
				edits = append(edits, charsEntryEdit(t, parent, "SV_AhorizonType", metadataText("")))
			case "negative-zero":
				edits[0].Value = siviReal(math.Copysign(0, -1))
			case "StartDate":
				edits = append(edits, charsEntryEdit(t, parent, "StartDate", metadataInteger("1899")))
			default:
				edits = append(edits, charsEntryEdit(t, parent, kind, metadataText("Z")))
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			written, err := service.writeSIVICHARSParentEntry(ctx, contextID, "108050", parent, edits, approve)
			if err == nil || written != nil || kind == "approval-error" && !errors.Is(err, failure) ||
				kind == "cancel-in-flight" && !errors.Is(err, context.Canceled) {
				t.Fatal("rejected CHARS mutation became success", written, err)
			}
			assertProfileSUFiles(t, service, before)
			parent, err = service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			if retry, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, "108050", parent,
				[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))}, charsWriteApprovePhysical); err != nil || retry == nil || retry.ChangedCells != 1 {
				t.Fatal("rejected mutation cannot retry", retry, err)
			}
		})
	}
}

func TestSIVICHARSWriteHistoricalOmissionPreparedNoOpAndBitCAS(t *testing.T) {
	service, state, db, _ := charsWriteFixture(t, false, 3)
	if _, err := db.Exec(`UPDATE Sample_Env SET SV_WaterTableCM=100,SubZone='too long historical',SV_AhorizonType='',Latitude=0;
		UPDATE Sample_Admin SET StartDate=1800`); err != nil {
		t.Fatal(err)
	}
	parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	edits := []siviParentScalarEdit{
		charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(99.999999)),
		charsEntryEdit(t, parent, "SubZone", siviParentCell(t, parent, "SubZone")),
		charsEntryEdit(t, parent, "SV_AhorizonType", metadataText("")),
		charsEntryEdit(t, parent, "StartDate", metadataInteger("1800")),
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, "108050", parent, edits, charsWriteApprovePhysical)
	if err != nil || written == nil || written.ChangedCells != 0 || written.HistoryID != "" {
		t.Fatal("omitted/prepared no-op invented a write", written, err)
	}
	assertProfileSUFiles(t, service, before)
	for _, binding := range parent.Bindings {
		if binding.Binding == "Latitude" {
			parent.Rows[0].Env.Cells[binding.Column] = siviReal(math.Copysign(0, -1))
		}
	}
	if written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, "108050", parent,
		[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))}, charsWriteApprovePhysical); err == nil || written != nil {
		t.Fatal("whole-parent forged zero bits bypassed review CAS", written, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICHARSWriteStrengthOneRestoresOnlyAuditedSubset(t *testing.T) {
	service, state, db, parent := charsWriteFixture(t, false, 1)
	edits := []siviParentScalarEdit{
		charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1)),
		charsEntryEdit(t, parent, "SnowCoverregime", metadataText("Z")),
	}
	written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, "108050", parent, edits, charsWriteApprovePhysical)
	if err != nil || written == nil || written.ChangedCells != 2 || written.HistoryID == "" {
		t.Fatal("strength1 mixed NULL plan failed", written, err)
	}
	event := charsWrittenHistory(t, db, written.HistoryID)
	if len(event.EntryPlan) != 2 || len(event.Changes) != 1 || event.Changes[0].Column != "SnowCoverregime" {
		t.Fatal("full source plan was confused with audited subset", event)
	}
	restored, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestorePrune)
	if err != nil || restored == nil || restored.RestoredRows != 1 {
		t.Fatal("audited-only restoration failed", restored, err)
	}
	current, err := service.readSIVICHARSParent(context.Background(), state.ContextID, parent.Plot)
	if err != nil || !metadataCellEqual(siviParentCell(t, current, "SV_WaterTableCM"), siviReal(0.10000000149011612)) ||
		!metadataCellEqual(siviParentCell(t, current, "SnowCoverregime"), metadataText("S")) {
		t.Fatal("restoration reverted unaudited NULL addition", err)
	}
}

func TestSIVICHARSWriteRestorationRejectsForgedHistoryAndDrift(t *testing.T) {
	for _, kind := range []string{"missing-plan", "foreign-form", "unrelated-cell", "zero-bits", "ProjectID", "repeated-plan", "physical-drift"} {
		t.Run(kind, func(t *testing.T) {
			service, state, db, parent := charsWriteFixture(t, false, 3)
			if _, err := db.Exec(`UPDATE Sample_Env SET Latitude=0`); err != nil {
				t.Fatal(err)
			}
			parent, err := service.readSIVICHARSParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			written, err := service.writeSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, parent,
				[]siviParentScalarEdit{charsEntryEdit(t, parent, "SV_WaterTableCM", siviReal(.1))}, charsWriteApprovePhysical)
			if err != nil {
				t.Fatal(err)
			}
			event := charsWrittenHistory(t, db, written.HistoryID)
			switch kind {
			case "missing-plan":
				event.EntryPlan = nil
			case "foreign-form":
				event.Committed.Form = "frmSIVIsite"
			case "unrelated-cell", "zero-bits":
				cell := metadataText("unrelated")
				column := "FieldNumber"
				if kind == "zero-bits" {
					cell, column = siviReal(math.Copysign(0, -1)), "Latitude"
				}
				for _, binding := range event.Committed.Bindings {
					if binding.Binding == column {
						row := &event.Committed.Rows[0].Env
						if binding.Table == event.Committed.AdminTable {
							row = &event.Committed.Rows[0].Admin
						}
						row.Cells[binding.Column] = cell
					}
				}
			case "ProjectID":
				event.ProjectAssignment = &siviProjectAssignmentHistory{}
			case "repeated-plan":
				event.EntryPlan = append(event.EntryPlan, event.EntryPlan[0])
			case "physical-drift":
				if _, err := db.Exec(`UPDATE Sample_Env SET RootZoneParticleSize='drift' WHERE PlotNumber='108050'`); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE "__VPRO_SIVICHARSParentEntryHistory" SET Proposal=? WHERE ID=?`, string(raw), written.HistoryID); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.restoreSIVICHARSParentEntry(context.Background(), state.ContextID, parent.Plot, written.HistoryID, AuditRestorePrune); err == nil || got != nil {
				t.Fatal("forged history/drift restored or pruned source", got, err)
			}
			assertProfileSUFiles(t, service, before)
			if _, err := siviParentHistoryAssignments(context.Background(), event, parent.Project, parent.Plot); err == nil {
				t.Fatal("CHARS history crossed normal authority")
			}
		})
	}
}

func TestSIVICHARSProjectionStorageEqualityMatchesExactRealBits(t *testing.T) {
	original := charsEntryOriginal(t)
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var copy siviParentProjection
	if err := json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	if !sourceParentProjectionStorageEqual(original, &copy) || !sourceParentProjectionStorageEqual(nil, nil) {
		t.Fatal("equal sources were rejected")
	}
	copy.Rows[0].Env.RowID = "different"
	if sourceParentProjectionStorageEqual(original, &copy) || sourceParentProjectionStorageEqual(original, nil) || reflect.DeepEqual(original, &copy) {
		t.Fatal("different source owner compared equal")
	}
}
