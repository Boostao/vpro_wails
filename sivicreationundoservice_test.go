package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func siviCreationUndoServiceFixture(t *testing.T, contexts *ContextService) *SIVICreationUndoService {
	t.Helper()
	service, err := NewSIVICreationUndoService(contexts, func(name string) (string, bool) {
		return "true", name == siviCreationUndoFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func siviCreationUndoServiceOperations(service *SIVICreationUndoService, ctx context.Context) []func() error {
	return []func() error{
		func() error { _, err := service.GetHistory(ctx, "C", "P"); return err },
		func() error { _, err := service.Review(ctx, "C", "P", "H"); return err },
		func() error { _, err := service.Undo(ctx, "C", "{}"); return err },
		func() error { _, err := service.LookupReceipt(ctx, "C", "{}"); return err },
	}
}

func TestSIVICreationUndoServiceIndependentFlagsAndGuards(t *testing.T) {
	lookup := func(string) (string, bool) { return "true", true }
	if _, err := NewSIVICreationUndoService(nil, lookup); err == nil {
		t.Fatal("nil ownership accepted")
	}
	if _, err := NewSIVICreationUndoService(&ContextService{}, nil); err == nil {
		t.Fatal("nil feature lookup accepted")
	}
	for _, value := range []string{"", "TRUE", "1", " true ", "false\n"} {
		if _, err := NewSIVICreationUndoService(&ContextService{}, func(string) (string, bool) {
			return value, true
		}); err == nil || !strings.Contains(err.Error(), siviCreationUndoFeatureEnvironment) {
			t.Fatal("malformed flag accepted", value, err)
		}
	}
	for _, value := range []string{"absent", "false", "true"} {
		service, err := NewSIVICreationUndoService(&ContextService{}, func(name string) (string, bool) {
			if name != siviCreationUndoFeatureEnvironment {
				return "true", true
			}
			return value, value != "absent"
		})
		if err != nil || service.enabled != (value == "true") {
			t.Fatal("independent default-off flag failed", service, err)
		}
		if !service.enabled {
			for _, operation := range siviCreationUndoServiceOperations(service, context.Background()) {
				if err := operation(); err == nil || !strings.Contains(err.Error(), "disabled") {
					t.Fatal("disabled call reached transport/storage", err)
				}
			}
		}
		for _, operation := range siviCreationUndoServiceOperations(service, nil) {
			if err := operation(); err == nil || !strings.Contains(err.Error(), "context") {
				t.Fatal("nil context accepted", err)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, operation := range siviCreationUndoServiceOperations(service, ctx) {
			if err := operation(); !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost priority", err)
			}
		}
	}
	for _, service := range []*SIVICreationUndoService{nil, {}, {enabled: true}} {
		for _, operation := range siviCreationUndoServiceOperations(service, context.Background()) {
			if err := operation(); err == nil || !strings.Contains(err.Error(), "ownership") {
				t.Fatal("unowned service accepted", err)
			}
		}
	}
}

func TestSIVICreationUndoServiceRawRequestsRefuseWithoutWrites(t *testing.T) {
	contexts, state, _, request, _ := siviCreationUndoFixture(t, false, 3)
	service := siviCreationUndoServiceFixture(t, contexts)
	base := siviDeletionServiceJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, raw := range []string{
		"", "{}", "null", "[]", base + "{}",
		strings.Replace(base, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(base, `"requestId":`, `"requestId":"duplicate","requestId":`, 1),
		strings.Replace(base, `"project":`, `"confirmed":true,"project":`, 1),
		strings.Replace(base, `"project":"Sample"`, `"project":"\ud800"`, 1),
		strings.Replace(base, `"project":"Sample"`, "\"project\":\"\xff\"", 1),
	} {
		for _, operation := range []func() error{
			func() error { _, err := service.Undo(context.Background(), state.ContextID, raw); return err },
			func() error { _, err := service.LookupReceipt(context.Background(), state.ContextID, raw); return err },
		} {
			if err := operation(); err == nil {
				t.Fatal("malformed raw authority accepted", raw)
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	if receipt, err := service.LookupReceipt(context.Background(), state.ContextID, base); err != nil || receipt != nil {
		t.Fatal("missing Undo history was not unresolved", receipt, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVICreationUndoHistoryMissingEmptyAndUncommitted(t *testing.T) {
	contexts, state, db, creation := siviCreationFixture(t, true, 3)
	service := siviCreationUndoServiceFixture(t, contexts)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	list, err := service.GetHistory(context.Background(), state.ContextID, creation.Plot)
	if err != nil || list == nil || list.HistoryPresent || list.Events == nil || len(list.Events) != 0 {
		t.Fatal("missing history conflated with empty history", list, err)
	}
	assertProfileSUFiles(t, contexts, before)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE;
		CREATE TABLE "__VPRO_SIVICreationHistory" ( RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
		INSERT INTO "__VPRO_SIVICreationHistory" VALUES('pending','invalid','invalid')`); err != nil {
		t.Fatal(err)
	}
	pending, readErr := service.GetHistory(context.Background(), state.ContextID, creation.Plot)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || !reflect.DeepEqual(pending, list) {
		t.Fatal("uncommitted history leaked into discovery", pending, readErr)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`CREATE TABLE "__VPRO_SIVICreationHistory" ( RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	list, err = service.GetHistory(context.Background(), state.ContextID, creation.Plot)
	if err != nil || list == nil || !list.HistoryPresent || list.Events == nil || len(list.Events) != 0 {
		t.Fatal("existing empty history not explicit", list, err)
	}
	assertSIVICreationUndoWire(t, list, []string{"contextId", "project", "plot", "historyPresent", "events"})
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVICreationUndoHistoryReopenReviewConsumeAndReadOnlyReceipt(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		contexts, state, _, request, creation := siviCreationUndoFixture(t, true, 3)
		reopened, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
		if err != nil {
			t.Fatal(err)
		}
		service := siviCreationUndoServiceFixture(t, contexts)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		list, err := service.GetHistory(context.Background(), reopened.ContextID, request.Plot)
		if err != nil || list == nil || !list.HistoryPresent || len(list.Events) != 1 ||
			list.ContextID != reopened.ContextID || list.Project != request.Project || list.Plot != request.Plot {
			t.Fatal("reopened owner cannot discover persisted creation", list, err)
		}
		event := list.Events[0]
		if event.HistoryID != creation.HistoryID || event.RowID != creation.RowID || event.ID != creation.ID ||
			event.Form != creation.Form || event.Species != creation.Request.Species || event.Actor != creation.Actor ||
			event.EditWhen != creation.EditWhen || event.Undone || event.Consumed {
			t.Fatal("historical summary differs from durable creation", event)
		}
		if !event.ReviewAvailable || event.UnavailableReason != nil {
			t.Fatal("fresh UUID history could not request independent review", event)
		}
		assertSIVICreationUndoWire(t, event, []string{"historyId", "form", "rowId", "id", "species", "actor", "editWhen", "undone", "consumed",
			"reviewAvailable", "unavailableReason"})
		review, err := service.Review(context.Background(), reopened.ContextID, request.Plot, event.HistoryID)
		if err != nil || review == nil || !reflect.DeepEqual(review.Committed, creation.Committed) {
			t.Fatal("explicit selection failed to acquire full44 fresh review", review, err)
		}
		assertProfileSUFiles(t, contexts, before)
		request.ContextID, request.Expected, request.Action = reopened.ContextID, review.Expected, action
		result, err := service.Undo(context.Background(), reopened.ContextID, siviDeletionServiceJSON(t, request))
		if err != nil || result == nil || !result.DidCommit || result.Replayed || result.RemovedRows != 1 {
			t.Fatal("explicit reviewed Undo failed", result, err)
		}
		before = databaseBytes(t, contexts.projects.sqlite.attachments)
		list, err = service.GetHistory(context.Background(), reopened.ContextID, request.Plot)
		if err != nil || list == nil || len(list.Events) != 1 || !list.Events[0].Consumed || !list.Events[0].Undone {
			t.Fatal("consumed creation vanished or remained unconsumed", list, err)
		}
		if list.Events[0].ReviewAvailable || list.Events[0].UnavailableReason == nil {
			t.Fatal("consumed history offered another review without an unavailable diagnostic", list)
		}
		if review, err := service.Review(context.Background(), reopened.ContextID, request.Plot, event.HistoryID); err == nil || review != nil {
			t.Fatal("consumed history granted another Undo review", review, err)
		}

		receipt, err := service.LookupReceipt(context.Background(), reopened.ContextID, siviDeletionServiceJSON(t, request))
		expected := *result
		expected.DidCommit, expected.Replayed = false, true
		if err != nil || !reflect.DeepEqual(receipt, &expected) {
			t.Fatal("read-only lookup did not recover complete acknowledgement", receipt, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestSIVICreationUndoHistoryRejectsCorruptionBeforeOwnerFiltering(t *testing.T) {
	contexts, state, db, request, _ := siviCreationUndoFixture(t, false, 0)
	service := siviCreationUndoServiceFixture(t, contexts)
	if _, err := db.Exec(`INSERT INTO "__VPRO_SIVICreationHistory" VALUES('foreign','2001-01-01 00:00:00','{}')`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	if list, err := service.GetHistory(context.Background(), state.ContextID, request.Plot); err == nil || list != nil {
		t.Fatal("corrupt global history produced partial owner success", list, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVICreationUndoHistoryLegacyIdentityVisibleWithoutBlockingUUIDChoices(t *testing.T) {
	contexts, state, _, request, _ := siviCreationUndoFixture(t, false, 3)
	legacy := siviCreationRequest{RequestID: "legacy-creation", ContextID: state.ContextID,
		Project: request.Project, Plot: request.Plot, Form: "SubVegA-SIVI", Species: "A",
		Covers: []siviCreationCover{{"Cover1", siviReal(0)}}}
	if _, err := contexts.createSIVIVegetation(context.Background(), state.ContextID, legacy); err != nil {
		t.Fatal(err)
	}
	service := siviCreationUndoServiceFixture(t, contexts)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	list, err := service.GetHistory(context.Background(), state.ContextID, request.Plot)
	if err != nil || list == nil || len(list.Events) != 2 {
		t.Fatal("coherent legacy history blocked or vanished from owned discovery", list, err)
	}
	for _, event := range list.Events {
		if event.HistoryID == legacy.RequestID {
			if event.ReviewAvailable || event.UnavailableReason == nil || !strings.Contains(*event.UnavailableReason, "legacy") {
				t.Fatal("legacy history did not expose mutation-unavailable diagnostics", event)
			}
		} else if event.HistoryID != request.HistoryID || !event.ReviewAvailable || event.UnavailableReason != nil {
			t.Fatal("valid UUID historical choice was blocked by legacy evidence", event)
		}
	}
	if reviewed, err := service.Review(context.Background(), state.ContextID, request.Plot, legacy.RequestID); reviewed != nil || err == nil {
		t.Fatal("legacy identity acquired UUID Undo authority", reviewed, err)
	}
	if reviewed, err := service.Review(context.Background(), state.ContextID, request.Plot, request.HistoryID); reviewed == nil || err != nil {
		t.Fatal("valid UUID history could not be independently reviewed", reviewed, err)
	}
	assertProfileSUFiles(t, contexts, before)
}
