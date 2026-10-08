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

func lifeformWorkbookServiceFixture(t *testing.T, external bool) (*LifeformWorkbookService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	service, err := NewLifeformWorkbookService(contexts, func(name string) (string, bool) {
		if name != lifeformWorkbookFeatureEnvironment {
			t.Fatal("workbook borrowed another workflow gate", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func lifeformWorkbookReviewJSON(t *testing.T, details [6]bool) string {
	t.Helper()
	return workbookRequest(t, map[string]any{"details": details})
}

func lifeformWorkbookExportJSON(t *testing.T, details [6]bool, approval, destination string) string {
	t.Helper()
	return workbookRequest(t, map[string]any{"details": details, "approvalHash": approval, "destination": destination})
}

func TestLifeformWorkbookServiceOwnedReviewPublicationDeterminismAndCollision(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := lifeformWorkbookServiceFixture(t, external)
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		details := [6]bool{true, false, true, false, true, false}
		request := lifeformWorkbookReviewJSON(t, details)
		review, err := service.GetReview(context.Background(), state.ContextID, request)
		if err != nil || review == nil || review.Lifeform.ContextID != state.ContextID ||
			review.Attributes.ProjectPath != state.ProjectPath || review.Attributes.SUPath != state.SUPath ||
			len(review.Sheets) != len(review.Lifeform.Report.Units) || len(review.Details) != 6 ||
			len(review.Sheets) == 0 || review.Bytes <= 0 {
			t.Fatal("combined owned review differs", review, err)
		}
		again, err := service.GetReview(context.Background(), state.ContextID, request)
		if err != nil || !reflect.DeepEqual(review, again) {
			t.Fatal("unchanged exact review is nondeterministic", err)
		}
		destination := filepath.Join(t.TempDir(), "summary.xlsx")
		export := lifeformWorkbookExportJSON(t, details, review.ApprovalHash, destination)
		outcome, err := service.ExportReviewed(context.Background(), state.ContextID, export)
		if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
			t.Fatal("reviewed source workbook not published", outcome, err)
		}
		data, err := os.ReadFile(destination)
		if err != nil || len(data) != review.Bytes {
			t.Fatal("published bytes missing", err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
			t.Fatal("published bytes differ from reviewed exact workbook")
		}
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, export); err != nil ||
			outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
			t.Fatal("existing workbook replaced or rejection hidden", outcome, err)
		}
		collision, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(data, collision) {
			t.Fatal("collision changed published file", err)
		}
		assertProfileSUFiles(t, service.contexts, before)
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("workbook workflow changed config", err)
		}
	}
}

func TestLifeformWorkbookServiceIndependentGateAndRawShapeRefusal(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewLifeformWorkbookService(nil, func(string) (string, bool) { return gate, gate != "" })
		if gate == "" || gate == "false" || gate == "true" {
			if err != nil || service.enabled != (gate == "true") {
				t.Fatal("independent literal gate differs", gate, err)
			}
			if _, err := service.GetReview(context.Background(), "owned", `{}`); err == nil {
				t.Fatal("unavailable service returned a review")
			}
		} else if err == nil {
			t.Fatal("malformed gate accepted", gate)
		}
	}
	service, state := lifeformWorkbookServiceFixture(t, false)
	valid := `[false,false,false,false,false,false]`
	for _, request := range []string{
		`{}`, `{"details":null}`, `{"details":[]}`, `{"details":[false]}`,
		`{"details":[false,false,false,false,false,false,false]}`,
		`{"details":[null,false,false,false,false,false]}`,
		`{"details":[0,false,false,false,false,false]}`,
		`{"details":["false",false,false,false,false,false]}`,
		`{"details":` + valid + `,"details":` + valid + `}`,
		`{"Details":` + valid + `}`,
		`{"details":` + valid + `,"unknown":"\ud800"}`,
	} {
		if review, err := service.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("raw options silently repaired/defaulted", request, review, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID, `{"details":`+valid+`}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []string{
		lifeformWorkbookExportJSON(t, [6]bool{}, "BAD", filepath.Join(t.TempDir(), "bad.xlsx")),
		lifeformWorkbookExportJSON(t, [6]bool{}, review.ApprovalHash, filepath.Join(t.TempDir(), "bad.db")),
		lifeformWorkbookExportJSON(t, [6]bool{true}, review.ApprovalHash, filepath.Join(t.TempDir(), "changed.xlsx")),
		`{"details":` + valid + `,"approvalHash":"` + review.ApprovalHash + `","destination":"\ud800.xlsx"}`,
		`{"details":` + valid + `,"approvalHash":"` + review.ApprovalHash + `","destination":"a.xlsx","destination":"b.xlsx"}`,
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err == nil || outcome != nil {
			t.Fatal("invalid raw approval/options/destination published", outcome, err)
		}
	}
	if _, err := service.GetReview(nil, state.ContextID, `{"details":`+valid+`}`); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetReview(ctx, state.ContextID, `{"details":`+valid+`}`); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled review published", err)
	}
}

func TestLifeformWorkbookServiceReadFailureCancellationAndKnownRetry(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := lifeformWorkbookServiceFixture(t, false)
			review, err := service.GetReview(context.Background(), state.ContextID, lifeformWorkbookReviewJSON(t, [6]bool{}))
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			destination := filepath.Join(t.TempDir(), "summary.xlsx")
			request := lifeformWorkbookExportJSON(t, [6]bool{}, review.ApprovalHash, destination)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rejected, reads := errors.New("combined workbook snapshot refused"), 0
			hooks := lifeformWorkbookHooks{
				snapshot: publicationReadSnapshotHooks{
					commitRead: func(*sql.Tx) error {
						if phase == "first-commit" {
							return rejected
						}
						return nil
					},
					rollbackRead: func(*sql.Tx) error {
						reads++
						if phase == "first-cleanup" && reads == 1 || phase == "prelink-cleanup" && reads == 2 ||
							phase == "published-cleanup" && reads == 3 {
							return rejected
						}
						return nil
					},
				},
				publication: artifactPublicationHooks{observe: func(current, _ string) error {
					if phase == "prelink-cancel" && current == "prelink" || phase == "published-cancel" && current == "published" {
						cancel()
					}
					return nil
				}},
			}
			outcome, err := service.exportReviewed(ctx, state.ContextID, request, hooks)
			switch phase {
			case "first-commit", "first-cleanup":
				if !errors.Is(err, rejected) || outcome != nil {
					t.Fatal("failed initial snapshot invented publication acknowledgement", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("known no-file outcome hidden", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("irreversible publication became unknown or replayable", outcome, err)
				}
			}
			if strings.HasPrefix(phase, "published-") {
				data, err := os.ReadFile(destination)
				if err != nil || len(data) != review.Bytes {
					t.Fatal("committed workbook absent", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed publication left output", err)
				}
				retry, err := service.ExportReviewed(context.Background(), state.ContextID, request)
				if err != nil || retry == nil || retry.Status != "published" || retry.SHA256 != review.WorkbookSHA256 {
					t.Fatal("known failure leaked ownership or prevented safe retry", retry, err)
				}
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

func TestLifeformWorkbookServiceCompleteRawSourceDriftAndStaleOwner(t *testing.T) {
	service, state := lifeformWorkbookServiceFixture(t, true)
	review, err := service.GetReview(context.Background(), state.ContextID, lifeformWorkbookReviewJSON(t, [6]bool{}))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE USysAllSpecs SET EnglishName='unused source drift'`)
	if err = errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "summary.xlsx")
	outcome, err := service.ExportReviewed(context.Background(), state.ContextID,
		lifeformWorkbookExportJSON(t, [6]bool{}, review.ApprovalHash, destination))
	if err == nil || outcome != nil || !strings.Contains(err.Error(), "physical rows") {
		t.Fatal("unrendered raw source drift accepted", outcome, err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("drift rejection left a workbook", err)
	}
	next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetReview(context.Background(), state.ContextID, lifeformWorkbookReviewJSON(t, [6]bool{})); err == nil {
		t.Fatal("stale owner returned a review")
	}
	if _, err := service.GetReview(context.Background(), next.ContextID, lifeformWorkbookReviewJSON(t, [6]bool{})); err != nil {
		t.Fatal("fresh owner could not recover", err)
	}
}
