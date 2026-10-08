package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func siviDeletionServiceLookup(flags map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name == siviDeletionWritingFeatureEnvironment || name == siviDeletionRestoringFeatureEnvironment {
			value, present := flags[name]
			return value, present
		}
		return "true", true
	}
}

func siviDeletionServiceFixture(t *testing.T, contexts *ContextService, writing, restoring bool) *SIVIDeletionService {
	t.Helper()
	service, err := NewSIVIDeletionService(contexts, siviDeletionServiceLookup(map[string]string{
		siviDeletionWritingFeatureEnvironment: fmt.Sprint(writing), siviDeletionRestoringFeatureEnvironment: fmt.Sprint(restoring),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func siviDeletionServiceOperations(service *SIVIDeletionService, ctx context.Context, contextID, plot, form, rowID, deletionJSON, restorationJSON, historyID string) []func() error {
	return []func() error{
		func() error { _, err := service.GetOriginal(ctx, contextID, plot, form, rowID); return err },
		func() error { _, err := service.Delete(ctx, contextID, deletionJSON); return err },
		func() error { _, err := service.LookupReceipt(ctx, contextID, deletionJSON); return err },
		func() error { _, err := service.ReviewRestoration(ctx, contextID, plot, historyID); return err },
		func() error { _, err := service.Restore(ctx, contextID, restorationJSON); return err },
		func() error { _, err := service.LookupRestorationReceipt(ctx, contextID, restorationJSON); return err },
		func() error { _, err := service.GetHistory(ctx, contextID, plot); return err },
		func() error { _, err := service.GetTargets(ctx, contextID, plot); return err },
	}
}

func siviDeletionServiceJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSIVIDeletionServiceIndependentDefaultOffStrictFlagsAndGuards(t *testing.T) {
	if _, err := NewSIVIDeletionService(nil, siviDeletionServiceLookup(nil)); err == nil {
		t.Fatal("nil context ownership accepted")
	}
	if _, err := NewSIVIDeletionService(&ContextService{}, nil); err == nil {
		t.Fatal("nil feature lookup accepted")
	}
	for _, feature := range []string{siviDeletionWritingFeatureEnvironment, siviDeletionRestoringFeatureEnvironment} {
		for _, value := range []string{"", "unknown", " true ", "TRUE", "False", "1", "0", " true", "false\n"} {
			if _, err := NewSIVIDeletionService(&ContextService{}, siviDeletionServiceLookup(map[string]string{feature: value})); err == nil ||
				!strings.Contains(err.Error(), feature) {
				t.Fatal("malformed feature was silently defaulted/coerced", feature, value, err)
			}
		}
	}
	for _, flags := range []map[string]string{
		nil,
		{siviDeletionWritingFeatureEnvironment: "false", siviDeletionRestoringFeatureEnvironment: "false"},
		{siviDeletionWritingFeatureEnvironment: "true"},
		{siviDeletionRestoringFeatureEnvironment: "true"},
		{siviDeletionWritingFeatureEnvironment: "true", siviDeletionRestoringFeatureEnvironment: "true"},
	} {
		service, err := NewSIVIDeletionService(&ContextService{}, siviDeletionServiceLookup(flags))
		if err != nil || service.writing != (flags[siviDeletionWritingFeatureEnvironment] == "true") ||
			service.restoring != (flags[siviDeletionRestoringFeatureEnvironment] == "true") {
			t.Fatal("flags not independent/default-off", service, err)
		}
		for i, operation := range siviDeletionServiceOperations(service, context.Background(), "C", "P", "SubVegA-SIVI", "1", "{}", "{}", "H") {
			enabled := service.writing
			if i >= 3 && i < 7 {
				enabled = service.restoring
			}
			if !enabled {
				if err := operation(); err == nil || !strings.Contains(err.Error(), "disabled") {
					t.Fatal("disabled operation reached storage/transport", i, err)
				}
			}
		}
		for _, operation := range siviDeletionServiceOperations(service, nil, "C", "P", "F", "1", "{}", "{}", "H") {
			if err := operation(); err == nil || !strings.Contains(err.Error(), "context") {
				t.Fatal("nil request context accepted", err)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, operation := range siviDeletionServiceOperations(service, ctx, "C", "P", "F", "1", "{}", "{}", "H") {
			if err := operation(); !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation did not precede gating/JSON/storage", err)
			}
		}
	}
	var missing *SIVIDeletionService
	for _, service := range []*SIVIDeletionService{missing, {}, {writing: true, restoring: true}} {
		for _, operation := range siviDeletionServiceOperations(service, context.Background(), "C", "P", "F", "1", "{}", "{}", "H") {
			if err := operation(); err == nil || !strings.Contains(err.Error(), "ownership") {
				t.Fatal("nil service/context ownership accepted", err)
			}
		}
	}
}

func TestSIVIDeletionServiceMalformedRawRequestsDoNotWrite(t *testing.T) {
	contexts, state, _, deletion := siviDeletionFixture(t, false, 3)
	service := siviDeletionServiceFixture(t, contexts, true, true)
	base := siviDeletionServiceJSON(t, deletion)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, raw := range []string{
		``, `{}`, `null`, `[]`,
		strings.Replace(base, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(base, `"requestId":`, `"requestId":"duplicate","requestId":`, 1),
		strings.Replace(base, `"columns":`, `"confirmed":true,"columns":`, 1),
		strings.Replace(base, `"RAW"`, `"\ud800"`, 1),
		strings.Replace(base, `"RAW"`, "\"\xff\"", 1),
		strings.Replace(base, `"storage":"text"`, `"storage":"text","storage":"text"`, 1),
		strings.Replace(base, `"name":"PlotNumber"`, `"name":"PlotNumber","unexpected":true`, 1),
		base + `{}`,
	} {
		for _, operation := range []func() error{
			func() error { _, err := service.Delete(context.Background(), state.ContextID, raw); return err },
			func() error { _, err := service.LookupReceipt(context.Background(), state.ContextID, raw); return err },
		} {
			if err := operation(); err == nil {
				t.Fatal("malformed deletion authority accepted", raw)
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	result, err := service.Delete(context.Background(), state.ContextID, base)
	if err != nil || result == nil {
		t.Fatal("malformed refusals blocked valid deletion", result, err)
	}
	review, err := service.ReviewRestoration(context.Background(), state.ContextID, deletion.Plot, deletion.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	request := siviDeletionRestorationRequest{RequestID: "00000000-0000-4000-8000-000000000011",
		ContextID: state.ContextID, Project: deletion.Project, Plot: deletion.Plot, HistoryID: deletion.RequestID,
		Action: AuditRestoreRetain, Expected: review.Expected}
	base = siviDeletionServiceJSON(t, request)
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, raw := range []string{
		``, `{}`, `null`, `[]`,
		strings.Replace(base, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(base, `"contextId":`, `"contextId":"duplicate","contextId":`, 1),
		strings.Replace(base, `"project":`, `"original":{},"project":`, 1),
		strings.Replace(base, `"Sample"`, `"\ud800"`, 1),
		strings.Replace(base, `"Sample"`, "\"\xff\"", 1),
		strings.Replace(base, `,"expected":"`+review.Expected+`"`, "", 1),
		strings.Replace(base, `"expected":`, `"Expected":`, 1),
		strings.Replace(base, review.Expected, strings.ToUpper(review.Expected), 1),
		strings.Replace(base, `"expected":"`+review.Expected+`"`, `"expected":null`, 1),
		strings.Replace(base, `"action":"retain"`, `"action":"Retain"`, 1),
		base + `{}`,
	} {
		for _, operation := range []func() error{
			func() error { _, err := service.Restore(context.Background(), state.ContextID, raw); return err },
			func() error {
				_, err := service.LookupRestorationReceipt(context.Background(), state.ContextID, raw)
				return err
			},
		} {
			if err := operation(); err == nil {
				t.Fatal("malformed restoration/CAS authority accepted", raw)
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
}

func TestSIVIDeletionServiceDisposableDeletionIndependentRestorationAndLostReceipts(t *testing.T) {
	for _, strength := range []int{0, 3} {
		for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
			t.Run(fmt.Sprintf("%d/%s", strength, action), func(t *testing.T) {
				contexts, state, db, deletion := siviDeletionFixture(t, true, strength)
				writer := siviDeletionServiceFixture(t, contexts, true, false)
				restorer := siviDeletionServiceFixture(t, contexts, false, true)
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				var original *SIVIDeletionOriginal
				var err error
				original, err = writer.GetOriginal(context.Background(), state.ContextID, deletion.Plot, deletion.Form, deletion.Original.RowID)
				if err != nil || original == nil || original.ContextID != state.ContextID || len(original.Columns) != 44 ||
					!reflect.DeepEqual(original.Columns, deletion.Columns) || !reflect.DeepEqual(original.Original, deletion.Original) {
					t.Fatal("exported original alias lost full44 kernel evidence", original, err)
				}
				deletionJSON := siviDeletionServiceJSON(t, deletion)
				if receipt, err := writer.LookupReceipt(context.Background(), state.ContextID, deletionJSON); receipt != nil || err != nil {
					t.Fatal("uncommitted deletion lookup not unresolved", receipt, err)
				}
				assertProfileSUFiles(t, contexts, before)
				var deleted *SIVIDeletionResult
				deleted, err = writer.Delete(context.Background(), state.ContextID, deletionJSON)
				if err != nil || deleted == nil || !deleted.DidCommit || deleted.Replayed ||
					!reflect.DeepEqual(deleted.Original, original.Original) {
					t.Fatal("exported deletion facade did not invoke exact private kernel", deleted, err)
				}
				before = databaseBytes(t, contexts.projects.sqlite.attachments)
				expectedDeletion := *deleted
				expectedDeletion.DidCommit, expectedDeletion.Replayed = false, true
				for i := 0; i < 2; i++ {
					receipt, err := writer.LookupReceipt(context.Background(), state.ContextID, deletionJSON)
					if err != nil || !reflect.DeepEqual(receipt, &expectedDeletion) {
						t.Fatal("lost deletion receipt not recovered read-only", receipt, err)
					}
					assertProfileSUFiles(t, contexts, before)
				}
				for _, operation := range siviDeletionServiceOperations(restorer, context.Background(), state.ContextID, deletion.Plot,
					deletion.Form, deletion.Original.RowID, deletionJSON, "{}", deletion.RequestID)[:3] {
					if err := operation(); err == nil || !strings.Contains(err.Error(), "disabled") {
						t.Fatal("restoration feature implicitly enabled deletion", err)
					}
				}
				for _, operation := range siviDeletionServiceOperations(writer, context.Background(), state.ContextID, deletion.Plot,
					deletion.Form, deletion.Original.RowID, deletionJSON, "{}", deletion.RequestID)[3:7] {
					if err := operation(); err == nil || !strings.Contains(err.Error(), "disabled") {
						t.Fatal("deletion feature implicitly enabled restoration", err)
					}
				}
				var review *SIVIDeletionRestorationReview
				review, err = restorer.ReviewRestoration(context.Background(), state.ContextID, deletion.Plot, deletion.RequestID)
				if err != nil || review == nil || len(review.Expected) != 64 || !reflect.DeepEqual(review.Original, deleted.Original) {
					t.Fatal("independent exported restoration review/CAS failed", review, err)
				}
				request := siviDeletionRestorationRequest{RequestID: "00000000-0000-4000-8000-000000000011",
					ContextID: state.ContextID, Project: deletion.Project, Plot: deletion.Plot, HistoryID: deletion.RequestID,
					Action: action, Expected: review.Expected}
				cancel := request
				cancel.Action = AuditRestoreCancel
				if result, err := restorer.Restore(context.Background(), state.ContextID, siviDeletionServiceJSON(t, cancel)); result == nil ||
					err != nil || !result.Cancelled || result.DidCommit || result.Restored != nil {
					t.Fatal("facade cancel mutated or fabricated a restored row", result, err)
				}
				requestJSON := siviDeletionServiceJSON(t, request)
				if receipt, err := restorer.LookupRestorationReceipt(context.Background(), state.ContextID, requestJSON); receipt != nil || err != nil {
					t.Fatal("missing restoration receipt not unresolved", receipt, err)
				}
				assertProfileSUFiles(t, contexts, before)
				var restored *SIVIDeletionRestorationResult
				restored, err = restorer.Restore(context.Background(), state.ContextID, requestJSON)
				if err != nil || restored == nil || !restored.DidCommit || restored.Replayed || restored.Expected != review.Expected ||
					!reflect.DeepEqual(restored.Restored, &deleted.Original) {
					t.Fatal("exported restoration facade lost full typed receipt", restored, err)
				}
				expected := *restored
				expected.DidCommit, expected.Replayed = false, true
				before = databaseBytes(t, contexts.projects.sqlite.attachments)
				for i := 0; i < 2; i++ {
					receipt, err := restorer.LookupRestorationReceipt(context.Background(), state.ContextID, requestJSON)
					if err != nil || !reflect.DeepEqual(receipt, &expected) {
						t.Fatal("lost restoration receipt not recovered read-only", receipt, err)
					}
					replay, err := restorer.Restore(context.Background(), state.ContextID, requestJSON)
					if err != nil || !reflect.DeepEqual(replay, &expected) {
						t.Fatal("facade repeated restoration/pruning instead of durable replay", replay, err)
					}
					assertProfileSUFiles(t, contexts, before)
				}
				var reserved int
				if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID=?`, deleted.ID).Scan(&reserved); err != nil || reserved != 1 {
					t.Fatal("facade restoration released permanent reservation", reserved, err)
				}
				reopened, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
				if err != nil {
					t.Fatal(err)
				}
				before = databaseBytes(t, contexts.projects.sqlite.attachments)
				receipt, err := restorer.LookupRestorationReceipt(context.Background(), reopened.ContextID, requestJSON)
				if err != nil || receipt == nil || receipt.ContextID != reopened.ContextID || receipt.Request.ContextID != state.ContextID ||
					receipt.DidCommit || !receipt.Replayed {
					t.Fatal("facade did not preserve durable context/current caller on reopen", receipt, err)
				}
				assertProfileSUFiles(t, contexts, before)
			})
		}
	}
}

func TestSIVIDeletionServiceNilCancelledAndForeignContextNeverWrites(t *testing.T) {
	contexts, state, _, request, _ := siviDeletionRestorationFixture(t, true, 3)
	service := siviDeletionServiceFixture(t, contexts, true, true)
	restorationJSON := siviDeletionServiceJSON(t, request)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, requestContext := range []context.Context{nil, ctx} {
		for _, operation := range siviDeletionServiceOperations(service, requestContext, state.ContextID, request.Plot,
			"SubVegA-SIVI", "1", "{}", restorationJSON, request.HistoryID) {
			if err := operation(); err == nil || requestContext != nil && !errors.Is(err, context.Canceled) {
				t.Fatal("nil/cancelled owned-context call reached storage", err)
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	for _, operation := range []func() error{
		func() error {
			_, err := service.GetOriginal(context.Background(), "foreign", request.Plot, "SubVegA-SIVI", "1")
			return err
		},
		func() error {
			_, err := service.ReviewRestoration(context.Background(), "foreign", request.Plot, request.HistoryID)
			return err
		},
		func() error { _, err := service.Restore(context.Background(), "foreign", restorationJSON); return err },
		func() error {
			_, err := service.LookupRestorationReceipt(context.Background(), "foreign", restorationJSON)
			return err
		},
		func() error {
			_, err := service.GetHistory(context.Background(), "foreign", request.Plot)
			return err
		},
		func() error {
			_, err := service.GetTargets(context.Background(), "foreign", request.Plot)
			return err
		},
	} {
		if err := operation(); err == nil {
			t.Fatal("foreign current context accepted")
		}
		assertProfileSUFiles(t, contexts, before)
	}
}
