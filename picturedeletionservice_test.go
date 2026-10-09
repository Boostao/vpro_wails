package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func pictureDeletionServiceFixture(t *testing.T) (*PictureDeletionService, pictureDeletionRequest) {
	t.Helper()
	contexts, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
	pictures := &PictureService{contexts: contexts, source: source, enabled: true}
	service, err := NewPictureDeletionService(pictures, pictureServiceLookup(map[string]string{
		pictureDeletionWritingEnvironment: "true", pictureDeletionRestoringEnvironment: "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return service, request
}

func pictureLifecycleTestWire(t *testing.T, request any) string {
	t.Helper()
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(wire)
}

func TestPictureDeletionServiceIndependentGates(t *testing.T) {
	for _, environment := range []string{pictureDeletionWritingEnvironment, pictureDeletionRestoringEnvironment} {
		for _, value := range []string{"", "false", "true", "TRUE", " true", "1"} {
			t.Run(environment+"/"+value, func(t *testing.T) {
				contexts, source, request := pictureDeletionFixture(t, false, pictureDeletionTestSchema)
				pictures := &PictureService{contexts: contexts, enabled: true, source: source}
				values := map[string]string{}
				if value != "" {
					values[environment] = value
				}
				service, err := NewPictureDeletionService(pictures, pictureServiceLookup(values))
				if value != "" && value != "true" && value != "false" {
					if err == nil || service != nil {
						t.Fatal("malformed gate silently defaulted", service, err)
					}
					return
				}
				if err != nil || service.writing != (environment == pictureDeletionWritingEnvironment && value == "true") ||
					service.restoring != (environment == pictureDeletionRestoringEnvironment && value == "true") {
					t.Fatal("independent default-off gate changed", service, err)
				}
				if !service.writing {
					if result, err := service.Delete(t.Context(), request.ContextID, pictureLifecycleTestWire(t, request)); err == nil || result != nil {
						t.Fatal("disabled deletion wrote", result, err)
					}
				}
				if !service.restoring {
					if history, err := service.GetHistory(t.Context(), request.ContextID, request.PlotNumber); err == nil || history != nil {
						t.Fatal("disabled historical restoration borrowed authority", history, err)
					}
				}
				pictures.enabled, pictures.source = false, nil
				enabled, err := NewPictureDeletionService(pictures, pictureServiceLookup(map[string]string{environment: "true"}))
				if err == nil || enabled != nil {
					t.Fatal("lifecycle gate inferred picture read ownership", enabled, err)
				}
			})
		}
	}
}

func TestPictureDeletionServiceHistoryReviewAndHistoricalRestore(t *testing.T) {
	service, request := pictureDeletionServiceFixture(t)
	source, contexts := service.pictures.source, service.pictures.contexts
	initialRows := pictureCreationRawRows(t, source)
	history, err := service.GetHistory(t.Context(), request.ContextID, request.PlotNumber)
	if err != nil || history == nil || history.Events == nil || len(history.Events) != 0 {
		t.Fatal("empty history invented events or lost explicit array", history, err)
	}
	wire := pictureLifecycleTestWire(t, request)
	receipt, err := service.Delete(t.Context(), request.ContextID, wire)
	if err != nil || receipt == nil || !receipt.DidCommit || receipt.Replayed {
		t.Fatal("facade deletion failed", receipt, err)
	}
	selection := contexts.projects.sqlite.selection
	next, err := contexts.SwitchContext(request.ContextID, ContextSelection{Project: selection.Project, ProjectPath: selection.ProjectPath,
		SU: selection.SU, SUPath: selection.SUPath, Hierarchy: selection.Hierarchy, HierarchyPath: selection.HierarchyPath})
	if err != nil {
		t.Fatal(err)
	}
	history, err = service.GetHistory(t.Context(), next.ContextID, request.PlotNumber)
	if err != nil || history == nil || history.ContextID != next.ContextID || len(history.Events) != 1 ||
		history.Events[0].Restoration != nil || !reflect.DeepEqual(history.Events[0].Deletion, *receipt) {
		t.Fatal("history discovery rewrote or hid an old context", history, err)
	}
	review, err := service.ReviewRestoration(t.Context(), next.ContextID, request.PlotNumber, receipt.HistoryID)
	if err != nil || review == nil || review.ContextID != next.ContextID || !reflect.DeepEqual(review.Deletion, *receipt) {
		t.Fatal("current restoration review lost historical origin", review, err)
	}
	restore := pictureRestorationRequest{RequestID: pictureCreationTestUUID2, ContextID: review.ContextID,
		HistoryID: review.HistoryID, Deletion: review.Deletion.Request, Peers: review.Peers}
	restoreWire := pictureLifecycleTestWire(t, restore)
	unknown, err := service.LookupRestorationReceipt(t.Context(), next.ContextID, restoreWire)
	if err != nil || unknown != nil {
		t.Fatal("unperformed restoration fabricated success", unknown, err)
	}
	restored, err := service.Restore(t.Context(), next.ContextID, restoreWire)
	if err != nil || restored == nil || !restored.DidCommit || !reflect.DeepEqual(initialRows, pictureCreationRawRows(t, source)) {
		t.Fatal("reviewed facade restoration changed historical values", restored, err)
	}
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	replayed, err := service.Restore(t.Context(), next.ContextID, restoreWire)
	recovered, recoveryErr := service.LookupRestorationReceipt(t.Context(), next.ContextID, restoreWire)
	restored.Replayed = true
	if err != nil || recoveryErr != nil || replayed == nil || recovered == nil ||
		!reflect.DeepEqual(*replayed, *restored) || !reflect.DeepEqual(*recovered, *restored) ||
		!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("facade replay/readonly recovery repeated restoration", replayed, recovered, err, recoveryErr)
	}
	history, err = service.GetHistory(t.Context(), next.ContextID, request.PlotNumber)
	if err != nil || history.Events[0].Restoration == nil || history.Events[0].Restoration.Replayed {
		t.Fatal("historical list lost immutable acknowledgement", history, err)
	}
	if review, err := service.ReviewRestoration(t.Context(), next.ContextID, request.PlotNumber, receipt.HistoryID); err == nil || review != nil {
		t.Fatal("already consumed history offered another restoration", review, err)
	}
}

func TestPictureDeletionServiceReceiptObservationCannotRaceMutation(t *testing.T) {
	for _, restoration := range []bool{false, true} {
		t.Run(fmt.Sprint(restoration), func(t *testing.T) {
			service, request := pictureDeletionServiceFixture(t)
			wire, requestID := pictureLifecycleTestWire(t, request), request.RequestID
			var restore pictureRestorationRequest
			if restoration {
				deleted, err := service.Delete(t.Context(), request.ContextID, wire)
				if err != nil {
					t.Fatal(err)
				}
				restore = pictureRestorationFixture(t, service.pictures.contexts, service.pictures.source, *deleted)
				wire, requestID = pictureLifecycleTestWire(t, restore), restore.RequestID
			}
			read, release := make(chan struct{}), make(chan struct{})
			releaseRead := sync.OnceFunc(func() { close(release) })
			defer releaseRead()
			service.afterRead = func() error {
				close(read)
				<-release
				return nil
			}
			result := make(chan error, 1)
			go func() {
				var err error
				if restoration {
					_, err = service.LookupRestorationReceipt(t.Context(), request.ContextID, wire)
				} else {
					_, err = service.LookupReceipt(t.Context(), request.ContextID, wire)
				}
				result <- err
			}()
			<-read
			if restoration {
				if receipt, err := service.Restore(t.Context(), request.ContextID, wire); err != nil || receipt == nil || !receipt.DidCommit {
					t.Fatal("competing actual restoration did not finish", receipt, err)
				}
			} else {
				if receipt, err := service.Delete(t.Context(), request.ContextID, wire); err != nil || receipt == nil || !receipt.DidCommit {
					t.Fatal("competing actual deletion did not finish", receipt, err)
				}
			}
			releaseRead()
			if err := <-result; err == nil || !strings.Contains(err.Error(), "started during receipt lookup") {
				t.Fatal("completed mutation disguised an early null receipt", err)
			}
			service.afterRead = nil
			finish, err := service.beginMutation(requestID)
			if err != nil {
				t.Fatal(err)
			}
			if restoration {
				if receipt, err := service.LookupRestorationReceipt(t.Context(), request.ContextID, wire); err == nil || receipt != nil {
					t.Fatal("active restoration reported a completed receipt", receipt, err)
				}
			} else if receipt, err := service.LookupReceipt(t.Context(), request.ContextID, wire); err == nil || receipt != nil {
				t.Fatal("active deletion reported a completed receipt", receipt, err)
			}
			finish()
		})
	}
}

func TestPictureDeletionServiceCancelledCompletedReadIsNotSuccess(t *testing.T) {
	service, request := pictureDeletionServiceFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	service.afterRead = func() error {
		cancel()
		return nil
	}
	receipt, err := service.LookupReceipt(ctx, request.ContextID, pictureLifecycleTestWire(t, request))
	if err == nil || receipt != nil {
		t.Fatal("cancelled read reported a valid no-commit observation", receipt, err)
	}
}

func TestPictureDeletionServiceStrictWireAndHistoryScope(t *testing.T) {
	for _, reason := range []string{"duplicate", "escaped-duplicate", "unicode", "null", "unknown", "foreign-context", "foreign-parent", "review-alias", "review-rowid"} {
		t.Run(reason, func(t *testing.T) {
			service, request := pictureDeletionServiceFixture(t)
			source := service.pictures.source
			wire := pictureLifecycleTestWire(t, request)
			if reason == "review-alias" || reason == "review-rowid" {
				deleted, err := service.Delete(t.Context(), request.ContextID, wire)
				if err != nil {
					t.Fatal(err)
				}
				if reason == "review-alias" {
					pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(ID,PlotNumber) VALUES('01','other')`)
				} else {
					pictureMetadataFixtureExec(t, source, `INSERT INTO tblVPics(rowid,ID,PlotNumber) VALUES(1,9,'other')`)
				}
				before := databaseBytes(t, map[string]string{"pictures": source.path})
				if review, err := service.ReviewRestoration(t.Context(), request.ContextID, request.PlotNumber, deleted.HistoryID); err == nil || review != nil ||
					!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
					t.Fatal("review granted replacement or wrote", review, err)
				}
				return
			}
			contextID := request.ContextID
			switch reason {
			case "duplicate":
				wire = `{"requestId":"` + request.RequestID + `",` + wire[1:]
			case "escaped-duplicate":
				wire = `{"\u0072equestId":"` + request.RequestID + `",` + wire[1:]
			case "unicode":
				wire = `{"contextId":"\ud800",` + wire[1:]
			case "null":
				wire = strings.Replace(wire, `"original":{`, `"original":null,"unexpected":{`, 1)
			case "unknown":
				wire = `{"unexpected":true,` + wire[1:]
			case "foreign-context":
				contextID += "-other"
			case "foreign-parent":
				request.PlotNumber = "108050x"
				wire = pictureLifecycleTestWire(t, request)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			if receipt, err := service.Delete(t.Context(), contextID, wire); err == nil || receipt != nil ||
				!reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("strict facade boundary accepted or wrote", receipt, err)
			}
			if history, err := service.GetHistory(t.Context(), request.ContextID, "NOTOWN"); err == nil || history != nil {
				t.Fatal("history discovery exposed an unowned parent", history, err)
			}
		})
	}
}
