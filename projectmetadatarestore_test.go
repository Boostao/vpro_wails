package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func metadataRestorationFixture(t *testing.T) (*ContextService, ProjectState, ProjectMetadataEdit, ProjectMetadataRestoreReview) {
	t.Helper()
	service, state, request := metadataEditFixture(t)
	request.Changes = []ProjectMetadataChange{{"Notes", metadataText("")}, {"StartDate", metadataInteger("2026")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, request.PlotNumber, "1")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, request, review
}

func TestProjectMetadataRestorationAll70FieldsAndListedExactHistory(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	db := metadataHistoryDatabase(t, service)
	if _, err := db.Exec(`UPDATE Sample_Metadata SET CollectedSite=2 WHERE ID=-200`); err != nil {
		t.Fatal(err)
	}
	request = reloadMetadataEdit(t, service, state, request)
	empty, err := service.ListProjectMetadataRestoreHistory(t.Context(), state.ContextID, "META1")
	if err != nil || empty.HistoryPresent || len(empty.Events) != 0 {
		t.Fatal("absent typed history was not distinguished", empty, err)
	}
	request.Changes = nil
	for column, field := range projectMetadataFields {
		cell := metadataText("  Literal \U0001f600 restored by typed provenance  ")
		if field.kind == "integer" {
			cell = metadataInteger("7")
			if field.options {
				cell = metadataInteger("3")
			}
		}
		if field.limitToList {
			cell = ProjectMetadataCell{Storage: "null"}
		}
		request.Changes = append(request.Changes, ProjectMetadataChange{column, cell})
	}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	list, err := service.ListProjectMetadataRestoreHistory(t.Context(), state.ContextID, "META1")
	if err != nil || !list.HistoryPresent || len(list.Events) != 1 ||
		list.Events[0].HistoryID != "1" || list.Events[0].RowID != "-200" || list.Events[0].ID != -200 {
		t.Fatal("typed history omitted exact technical/metadata identities", list, err)
	}
	review, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", "1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
		ProjectMetadataRestore{review, AuditRestorePrune, true}); err != nil {
		t.Fatal(err)
	}
	actual := reloadMetadataEdit(t, service, state, request)
	if !reflect.DeepEqual(actual.Original, request.Original) {
		t.Fatal("batched fields/stamps did not restore their original typed cells")
	}
}

func TestProjectMetadataRestorationCancellationAndHistoricalInvalidOriginal(t *testing.T) {
	service, state, _, review := metadataRestorationFixture(t)
	before := metadataFileBytes(t, service)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.RestoreProjectMetadata(ctx, state.ContextID,
		ProjectMetadataRestore{review, AuditRestorePrune, true}); err == nil {
		t.Fatal("cancelled restoration mutated data")
	}
	assertMetadataFileBytes(t, service, before, "")
	service.projects.sqlite.mu.Lock()
	ctx, cancel = context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := service.RestoreProjectMetadata(ctx, state.ContextID, ProjectMetadataRestore{review, AuditRestorePrune, true})
		done <- err
	}()
	cancel()
	err := <-done
	service.projects.sqlite.mu.Unlock()
	if err == nil {
		t.Fatal("busy coordinator ignored cancellation")
	}
	assertMetadataFileBytes(t, service, before, "")
	service, state, request := metadataEditFixture(t)
	db := metadataHistoryDatabase(t, service)
	if _, err := db.Exec(`UPDATE Sample_Metadata SET ProjectTitle=? WHERE ID=-200`, strings.Repeat("x", 256)); err != nil {
		t.Fatal(err)
	}
	request = reloadMetadataEdit(t, service, state, request)
	request.Changes = []ProjectMetadataChange{{"ProjectTitle", metadataText("New valid title")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	before = metadataFileBytes(t, service)
	if _, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", "1"); err == nil {
		t.Fatal("restoration accepted a new overlength assignment from historical invalid storage")
	}
	assertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataRestorationReferenceMembershipAndRetainedRetry(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	db := metadataHistoryDatabase(t, service)
	ref, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer ref.Close()
	if _, err := ref.Exec(`INSERT INTO USysTableOfLists(ListName,Item,Note,ItemOrder)
		VALUES('PlotQualitySite','OLD_RESTORE','Old literal quality',20000),
		('PlotQualitySite','NEW_RESTORE','New literal quality',20001)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET DataQualityVeg='Old literal quality' WHERE ID=-200`); err != nil {
		t.Fatal(err)
	}
	request = reloadMetadataEdit(t, service, state, request)
	request.Changes = []ProjectMetadataChange{{"DataQualityVeg", metadataText("New literal quality")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", "1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ref.Exec(`DELETE FROM USysTableOfLists WHERE Item='OLD_RESTORE'`); err != nil {
		t.Fatal(err)
	}
	before := metadataFileBytes(t, service)
	if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
		ProjectMetadataRestore{review, AuditRestorePrune, true}); err == nil {
		t.Fatal("missing reference membership was silently accepted")
	}
	assertMetadataFileBytes(t, service, before, "")
	if _, err := ref.Exec(`INSERT INTO USysTableOfLists(ListName,Item,Note,ItemOrder)
		VALUES('PlotQualitySite','OLD_RESTORE','Old literal quality',20000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
		ProjectMetadataRestore{review, AuditRestorePrune, true}); err != nil {
		t.Fatal("restored independent definition did not permit retained retry:", err)
	}
}
func TestProjectMetadataRestorationRetainPruneCancelAndIndependentIdentity(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		t.Run(string(action), func(t *testing.T) {
			service, state, request, review := metadataRestorationFixture(t)
			db := metadataHistoryDatabase(t, service)
			before := metadataFileBytes(t, service)
			cancelled, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
				ProjectMetadataRestore{review, AuditRestoreCancel, false})
			if err != nil || !cancelled.Cancelled {
				t.Fatal("explicit Cancel failed", cancelled, err)
			}
			assertMetadataFileBytes(t, service, before, "")
			result, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
				ProjectMetadataRestore{review, action, true})
			if err != nil || result.RestoredRows != len(review.Audits) || result.CleanedVegRows != 0 {
				t.Fatal("typed metadata restoration failed", result, err)
			}
			wantPruned := 0
			if action == AuditRestorePrune {
				wantPruned = len(review.Audits)
			}
			if result.PrunedAuditRows != wantPruned {
				t.Fatal("prune count did not match only the proven event")
			}
			actual := reloadMetadataEdit(t, service, state, request)
			if !reflect.DeepEqual(actual.Original, request.Original) {
				t.Fatal("typed restoration did not preserve original NULL/text/integer and row identity")
			}
			assertMetadataFileBytes(t, service, before, "project")
			var events int
			if err := db.QueryRow(`SELECT COUNT(*) FROM __VPRO_MetadataEditHistory`).Scan(&events); err != nil || events != 1 {
				t.Fatal("restoration overwrote/appended its retained technical provenance", events, err)
			}
			list, err := service.ListProjectMetadataRestoreHistory(t.Context(), state.ContextID, request.PlotNumber)
			if err != nil || len(list.Events) != 1 || !list.Events[0].Restored {
				t.Fatal("restoration receipt did not explicitly retire its original event", list, err)
			}
			var receipt string
			if err := db.QueryRow(`SELECT Restored FROM __VPRO_MetadataEditHistory WHERE ID=1`).Scan(&receipt); err != nil {
				t.Fatal(err)
			}
			var sealed struct {
				Review ProjectMetadataRestoreReview
				Action AuditRestoreAction
			}
			if err := json.Unmarshal([]byte(receipt), &sealed); err != nil || !reflect.DeepEqual(sealed.Review, review) || sealed.Action != action {
				t.Fatal("retained receipt does not seal the exact independently confirmed restoration", err)
			}
			after := metadataFileBytes(t, service)
			if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
				ProjectMetadataRestore{review, action, true}); err == nil {
				t.Fatal("completed restoration was replayed")
			}
			assertMetadataFileBytes(t, service, after, "")
		})
	}
}

func TestProjectMetadataRestorationAuditStrengthLeavesUnauditedChanges(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	if err := service.plots.SetAuditStrength(1); err != nil {
		t.Fatal(err)
	}
	request.Changes = []ProjectMetadataChange{{"Notes", metadataText("Explicit existing-value edit")},
		{"FieldLeader", metadataText("Unaudited addition remains")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, request.PlotNumber, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Audits) != 1 || review.Audits[0].EditField != "Notes" {
		t.Fatal("restoration inherited phantom option/stamp/addition audits")
	}
	if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
		ProjectMetadataRestore{review, AuditRestorePrune, true}); err != nil {
		t.Fatal(err)
	}
	actual := reloadMetadataEdit(t, service, state, request)
	for i, column := range actual.Columns {
		if column.Name == "Notes" && !reflect.DeepEqual(actual.Original.Cells[i], request.Original.Cells[i]) {
			t.Fatal("audited Notes did not restore")
		}
		if column.Name == "FieldLeader" && !reflect.DeepEqual(actual.Original.Cells[i], metadataText("Unaudited addition remains")) {
			t.Fatal("restoration changed an unaudited addition")
		}
	}
}

func TestProjectMetadataRestorationAliasesDriftRollbackAndRetry(t *testing.T) {
	for name, change := range map[string]string{
		"parent":     `UPDATE Sample_Env SET ProjectID='Independent parent' WHERE PlotNumber='META1'`,
		"metadata":   `UPDATE Sample_Metadata SET FieldLeader='Independent metadata' WHERE ID=-200`,
		"audit":      `UPDATE Sample_Audit SET AfterEdit='Independent audit' WHERE "Table"='_Metadata'`,
		"provenance": `UPDATE __VPRO_MetadataEditHistory SET Proposal='{}'`,
		"duplicate": `ALTER TABLE Sample_Metadata RENAME TO MetadataBefore;
			CREATE TABLE Sample_Metadata AS SELECT * FROM MetadataBefore;
			INSERT INTO Sample_Metadata(ID,ProjectID) VALUES(-200,'Foreign identity'); DROP TABLE MetadataBefore`,
		"trigger": `CREATE TRIGGER restoration_abort BEFORE UPDATE ON Sample_Metadata BEGIN SELECT RAISE(ABORT,'independent rejection'); END`,
		"late-prune-target": `CREATE TRIGGER restoration_late_target AFTER DELETE ON Sample_Audit
			WHEN OLD."Table"='_Metadata' BEGIN UPDATE Sample_Metadata SET Notes='Unreviewed late prune' WHERE ID=-200; END`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, request, review := metadataRestorationFixture(t)
			db := metadataHistoryDatabase(t, service)
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			before := metadataFileBytes(t, service)
			if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
				ProjectMetadataRestore{review, AuditRestorePrune, true}); err == nil {
				t.Fatal("independent drift/rejection was accepted")
			}
			assertMetadataFileBytes(t, service, before, "")
			if name == "trigger" || name == "late-prune-target" {
				trigger := "restoration_abort"
				if name == "late-prune-target" {
					trigger = "restoration_late_target"
				}
				if _, err := db.Exec(`DROP TRIGGER ` + trigger); err != nil {
					t.Fatal(err)
				}
				if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
					ProjectMetadataRestore{review, AuditRestorePrune, true}); err != nil {
					t.Fatal("retained proposal could not retry:", err)
				}
				actual := reloadMetadataEdit(t, service, state, request)
				if !reflect.DeepEqual(actual.Original, request.Original) {
					t.Fatal("retry did not restore exactly one original row")
				}
			}
		})
	}
	for _, alias := range []string{"_metadata", "Sample_Metadata", "sAMPLE_mETADATA"} {
		t.Run(alias, func(t *testing.T) {
			service, state, _, _ := metadataRestorationFixture(t)
			db := metadataHistoryDatabase(t, service)
			if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE "Table"='_Metadata'`, alias); err != nil {
				t.Fatal(err)
			}
			review, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", "1")
			if err != nil {
				t.Fatal("explicit restoration alias rejected:", err)
			}
			if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID,
				ProjectMetadataRestore{review, AuditRestorePrune, true}); err != nil {
				t.Fatal("explicit restoration alias did not restore:", err)
			}
		})
	}
}

func TestProjectMetadataRestorationStrictTransportStaleContextAndLegacyHistory(t *testing.T) {
	service, state, _, review := metadataRestorationFixture(t)
	before := metadataFileBytes(t, service)
	raw, err := json.Marshal(ProjectMetadataRestore{review, AuditRestoreRetain, true})
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{`{}`, `{"review":null,"action":"retain","confirmed":true}`,
		strings.TrimSuffix(string(raw), "}") + `,"extra":true}`,
		strings.Replace(string(raw), `"META1"`, `"\ud800"`, 1)} {
		var request ProjectMetadataRestore
		if err := json.Unmarshal([]byte(payload), &request); err == nil {
			t.Fatal("malformed/unknown restoration transport accepted")
		}
	}
	if _, err := service.RestoreProjectMetadata(t.Context(), "stale",
		ProjectMetadataRestore{review, AuditRestoreRetain, true}); err == nil {
		t.Fatal("stale restoration context accepted")
	}
	for _, historyID := range []string{"01", "+1", "1.0", "9223372036854775808", "999999"} {
		if _, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", historyID); err == nil {
			t.Fatal("nonexact or nonexistent technical identity accepted")
		}
	}
	if _, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META2", "1"); err == nil {
		t.Fatal("another parent consumed this event")
	}
	for _, mutate := range []func(*ProjectMetadataRestore){
		func(r *ProjectMetadataRestore) { r.Confirmed = false },
		func(r *ProjectMetadataRestore) { r.Action = "unknown" },
		func(r *ProjectMetadataRestore) { r.Review.ID = 1 },
		func(r *ProjectMetadataRestore) { r.Review.ContextID = "old" },
		func(r *ProjectMetadataRestore) { r.Review.Restored.RowID = "1" },
	} {
		request := ProjectMetadataRestore{review, AuditRestoreRetain, true}
		mutate(&request)
		if _, err := service.RestoreProjectMetadata(t.Context(), state.ContextID, request); err == nil {
			t.Fatal("unconfirmed/foreign/tampered review accepted")
		}
	}
	assertMetadataFileBytes(t, service, before, "")
	db := metadataHistoryDatabase(t, service)
	if _, err := db.Exec(`DROP TABLE __VPRO_MetadataEditHistory`); err != nil {
		t.Fatal(err)
	}
	before = metadataFileBytes(t, service)
	if _, err := service.ReviewProjectMetadataRestoration(t.Context(), state.ContextID, "META1", "1"); err == nil {
		t.Fatal("legacy plaintext audit was guessed without typed provenance")
	}
	assertMetadataFileBytes(t, service, before, "")
}
