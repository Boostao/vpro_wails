package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryWorkbookServiceFixture(t *testing.T, external bool) (*SiteUnitSummaryWorkbookService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	service, err := NewSiteUnitSummaryWorkbookService(contexts, func(name string) (string, bool) {
		if name != siteUnitSummaryWorkbookFeatureEnvironment {
			t.Fatal("summary workbook borrowed another feature gate", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func summaryWorkbookExportJSON(t *testing.T, method int, approval, destination string) string {
	t.Helper()
	return workbookRequest(t, map[string]any{"method": method, "approvalHash": approval, "destination": destination})
}

func TestSiteUnitSummaryWorkbookServiceOwnedReviewPublishCollisionAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryWorkbookServiceFixture(t, external)
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			request := workbookRequest(t, map[string]any{"method": method})
			review, err := service.GetReview(context.Background(), state.ContextID, request)
			if err != nil || review == nil || review.Preview.Report.Method != method ||
				review.Preview.ContextID != state.ContextID || len(review.Sheets) != 1 || review.Bytes <= 0 ||
				review.Scope.SiteUnitType != 1 || review.Scope.OrderBy != 1 || review.Scope.IncludeSpecies != 0 {
				t.Fatal("owned review scope/output differs", review, err)
			}
			again, err := service.GetReview(context.Background(), state.ContextID, request)
			if err != nil || !reflect.DeepEqual(again, review) {
				t.Fatal("exact unchanged approval not deterministic", err)
			}
			destination := filepath.Join(t.TempDir(), "summary.xlsx")
			export := summaryWorkbookExportJSON(t, method, review.ApprovalHash, destination)
			outcome, err := service.ExportReviewed(context.Background(), state.ContextID, export)
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
				t.Fatal("reviewed workbook not published", outcome, err)
			}
			data, err := os.ReadFile(destination)
			if err != nil || len(data) != review.Bytes {
				t.Fatal("actual workbook missing/wrong size", err)
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
				t.Fatal("actual bytes differ from approved workbook")
			}
			collision, err := service.ExportReviewed(context.Background(), state.ContextID, export)
			if err != nil || collision == nil || collision.Status != "not-published" || collision.ErrorMessage == "" {
				t.Fatal("collision replaced output or hid refusal", collision, err)
			}
			after, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("collision changed original bytes", err)
			}
		}
		assertProfileSUFiles(t, service.contexts, before)
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("workbook wrote configuration", err)
		}
	}
}

func TestSiteUnitSummaryWorkbookServiceGateStrictRequestsAndRetainedScope(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSiteUnitSummaryWorkbookService(nil, func(string) (string, bool) { return flag, flag != "" })
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal("literal independent gate differs", flag, err)
			}
			if _, err := service.GetReview(context.Background(), "owned", `{"method":1}`); err == nil {
				t.Fatal("unavailable gate/context produced review")
			}
		} else if err == nil {
			t.Fatal("malformed flag accepted", flag)
		}
	}
	service, state := summaryWorkbookServiceFixture(t, false)
	for _, request := range []string{`{}`, `{"method":null}`, `{"method":0}`, `{"method":3}`,
		`{"method":"1"}`, `{"method":1,"method":2}`, `{"Method":1}`,
		`{"method":1,"unknown":"\ud800"}`} {
		if review, err := service.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("malformed method/raw Unicode repaired", request, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID, `{"method":1}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []string{
		summaryWorkbookExportJSON(t, 1, "BAD", filepath.Join(t.TempDir(), "bad.xlsx")),
		summaryWorkbookExportJSON(t, 1, review.ApprovalHash, filepath.Join(t.TempDir(), "bad.db")),
		summaryWorkbookExportJSON(t, 2, review.ApprovalHash, filepath.Join(t.TempDir(), "changed.xlsx")),
		`{"method":1,"approvalHash":"` + review.ApprovalHash + `","destination":"\ud800.xlsx"}`,
		`{"method":1,"approvalHash":"` + review.ApprovalHash + `","destination":"a.xlsx","destination":"b.xlsx"}`,
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil || outcome != nil {
			t.Fatal("invalid approval/destination/method published", outcome, err)
		}
	}
	for _, changes := range []map[string]any{
		{"SESuType": 2}, {"SESuType": 3}, {"SEOrderBy": 2}, {"SEIncludeSppSummary": 1},
		{"SEOrderBy": "1"}, {"SEIncludeSppSummary": nil},
	} {
		scoped, scopeState := summaryWorkbookServiceFixture(t, false)
		if err := scoped.contexts.projects.preferences.update("ReportOptions", changes); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(scoped.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		if review, err := scoped.GetReview(context.Background(), scopeState.ContextID, `{"method":1}`); err == nil || review != nil {
			t.Fatal("unavailable retained scope silently repaired", changes, err)
		}
		after, err := os.ReadFile(scoped.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("refusal reset saved scope", changes, err)
		}
	}
	if _, err := service.GetReview(nil, state.ContextID, `{"method":1}`); err == nil {
		t.Fatal("nil context accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetReview(cancelled, state.ContextID, `{"method":1}`); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled review produced source", err)
	}
}

func TestSiteUnitSummaryWorkbookServiceRawSourceScopeAndOwnerDrift(t *testing.T) {
	for _, drift := range []string{"raw-table", "saved-scope"} {
		service, state := summaryWorkbookServiceFixture(t, true)
		review, err := service.GetReview(context.Background(), state.ContextID, `{"method":1}`)
		if err != nil {
			t.Fatal(err)
		}
		if drift == "raw-table" {
			db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["project"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`UPDATE ` + quoteHeaderIdentifier(state.ActiveProject+"_Env") + ` SET Location='unrendered source drift'`)
			if err = errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
		} else if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{"SEOrderBy": 3}); err != nil {
			t.Fatal(err)
		}
		fresh, err := service.GetReview(context.Background(), state.ContextID, `{"method":1}`)
		if err != nil || fresh.WorkbookSHA256 != review.WorkbookSHA256 || fresh.ApprovalHash == review.ApprovalHash {
			t.Fatal("unrendered raw/scope drift did not preserve workbook while changing complete approval", drift, err)
		}
		destination := filepath.Join(t.TempDir(), "drift.xlsx")
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID,
			summaryWorkbookExportJSON(t, 1, review.ApprovalHash, destination)); err == nil || outcome != nil {
			t.Fatal("unrendered raw/saved scope drift accepted", drift, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("drift left a workbook", err)
		}
		next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.GetReview(context.Background(), state.ContextID, `{"method":1}`); err == nil {
			t.Fatal("stale owner returned review")
		}
		if _, err := service.GetReview(context.Background(), next.ContextID, `{"method":1}`); err != nil {
			t.Fatal("fresh owner retry leaked lease", err)
		}
	}
}

func TestSiteUnitSummaryWorkbookServiceCancellationCompletionAndIrreversibleReceipts(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryWorkbookServiceFixture(t, false)
			review, err := service.GetReview(context.Background(), state.ContextID, `{"method":1}`)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "summary.xlsx")
			request := summaryWorkbookExportJSON(t, 1, review.ApprovalHash, destination)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads := 0
			sentinel := errors.New("owned summary completion/cleanup failure")
			hooks := siteUnitSummaryWorkbookHooks{snapshot: publicationReadSnapshotHooks{
				commitRead: func(*sql.Tx) error {
					reads++
					if phase == "first-commit" && reads == 1 {
						return sentinel
					}
					if phase == "prelink-cancel" && reads == 2 || phase == "published-cancel" && reads == 3 {
						cancel()
					}
					return nil
				},
				rollbackRead: func(*sql.Tx) error {
					if phase == "first-cleanup" && reads == 1 || phase == "prelink-cleanup" && reads == 2 || phase == "published-cleanup" && reads == 3 {
						return sentinel
					}
					return nil
				},
			}}
			outcome, err := service.exportReviewed(ctx, state.ContextID, request, hooks)
			switch phase {
			case "first-commit", "first-cleanup":
				if !errors.Is(err, sentinel) || outcome != nil {
					t.Fatal("initial read failure invented receipt", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("known no-file result hidden", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("committed result became replayable/unknown", outcome, err)
				}
			}
			if strings.HasPrefix(phase, "published-") {
				if data, err := os.ReadFile(destination); err != nil || len(data) != review.Bytes {
					t.Fatal("committed workbook absent", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed publication left output", err)
				}
				retry, err := service.ExportReviewed(context.Background(), state.ContextID, request)
				if err != nil || retry == nil || retry.Status != "published" {
					t.Fatal("known failure prevented safe retry", retry, err)
				}
			}
		})
	}
}
