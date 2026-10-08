package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func workbookServiceFixture(t *testing.T, external bool) (*EnvironmentWorkbookService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	service, err := NewEnvironmentWorkbookService(contexts, func(name string) (string, bool) {
		if name != environmentWorkbookFeatureEnvironment {
			t.Fatal("workbook gate borrowed another workflow", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func workbookRequest(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLongEnvironmentWorkbookServiceOwnedReviewExportAndCollision(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := workbookServiceFixture(t, external)
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		title := "  Literal workbook 😀  "
		review, err := service.GetReview(context.Background(), state.ContextID, workbookRequest(t, LongEnvironmentRequest{title}))
		if err != nil || review == nil || review.Preview.ContextID != state.ContextID ||
			review.Preview.ProjectPath != state.ProjectPath || review.Preview.SUPath != state.SUPath ||
			len(review.Preview.Report.Fields) != 72 || len(review.Sheets) != 2 {
			t.Fatal("owned source review failed", review, err)
		}
		destination := filepath.Join(t.TempDir(), "report.xlsx")
		request := workbookRequest(t, EnvironmentWorkbookExportRequest{title, review.ApprovalHash, destination})
		outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request)
		if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
			t.Fatal("reviewed workbook publication failed", outcome, err)
		}
		data, err := os.ReadFile(destination)
		if err != nil || len(data) != review.Bytes {
			t.Fatal("published length differs", len(data), err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
			t.Fatal("published bytes differ from reviewed workbook")
		}
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request); err != nil ||
			outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
			t.Fatal("existing workbook replaced/replayed", outcome, err)
		}
		afterFile, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(data, afterFile) {
			t.Fatal("collision changed existing workbook", err)
		}
		assertProfileSUFiles(t, service.contexts, before)
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("workbook changed configuration", err)
		}
	}
}

func TestLongEnvironmentWorkbookServiceGateRawUnicodeAndScopeRefusal(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewEnvironmentWorkbookService(nil, func(string) (string, bool) { return gate, gate != "" })
		if gate == "" || gate == "false" || gate == "true" {
			if err != nil || service.enabled != (gate == "true") {
				t.Fatal(gate, service, err)
			}
			if _, err := service.GetReview(context.Background(), "owned", `{"title":""}`); err == nil {
				t.Fatal("unavailable workflow published review")
			}
		} else if err == nil {
			t.Fatal("nonliteral gate accepted", gate)
		}
	}
	service, state := workbookServiceFixture(t, false)
	for _, request := range []string{`{"title":"\ud800"}`, `{"title":"\udfff"}`, `{"title":"","extra":true}`, `{}`} {
		if review, err := service.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("raw Unicode/shape repaired", review, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID, `{"title":""}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []EnvironmentWorkbookExportRequest{
		{"", "BAD", filepath.Join(t.TempDir(), "report.xlsx")},
		{"", review.ApprovalHash, filepath.Join(t.TempDir(), "report.db")},
		{"changed", review.ApprovalHash, filepath.Join(t.TempDir(), "report.xlsx")},
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, workbookRequest(t, request)); err == nil || outcome != nil {
			t.Fatal("invalid approval/title/destination published", outcome, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetReview(ctx, state.ContextID, `{"title":""}`); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request published review", err)
	}
	if _, err := service.GetReview(nil, state.ContextID, `{"title":""}`); err == nil {
		t.Fatal("nil context published")
	}
	owner := service.contexts.projects.sqlite
	owner.selection.SU = "USysSuTableDynamic"
	if _, err := service.GetReview(context.Background(), state.ContextID, `{"title":""}`); err == nil {
		t.Fatal("unimplemented hierarchy scope published")
	}
}

func TestLongEnvironmentWorkbookServiceReadCleanupCancellationAndRetry(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := workbookServiceFixture(t, false)
			review, err := service.GetReview(context.Background(), state.ContextID, `{"title":""}`)
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			destination := filepath.Join(t.TempDir(), "report.xlsx")
			request := workbookRequest(t, EnvironmentWorkbookExportRequest{"", review.ApprovalHash, destination})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rejected, reads := errors.New("owned workbook snapshot cleanup refused"), 0
			hooks := environmentWorkbookHooks{
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
					t.Fatal("failed initial read invented publication acknowledgement", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("prelink failure lost known no-file outcome", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("irreversible outcome became missing/unknown", outcome, err)
				}
			}
			if strings.HasPrefix(phase, "published-") {
				data, err := os.ReadFile(destination)
				if err != nil || len(data) != review.Bytes {
					t.Fatal("committed workbook missing", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("failed publication left workbook", err)
				}
				retry, err := service.ExportReviewed(context.Background(), state.ContextID, request)
				if err != nil || retry == nil || retry.Status != "published" || retry.SHA256 != review.WorkbookSHA256 {
					t.Fatal("known rejected attempt prevented explicit retry", retry, err)
				}
			}
			assertProfileSUFiles(t, service.contexts, before)
		})
	}
}

func TestLongEnvironmentWorkbookServiceStaleOwnerRefusesReviewAndPublication(t *testing.T) {
	service, state := workbookServiceFixture(t, true)
	review, err := service.GetReview(context.Background(), state.ContextID, `{"title":""}`)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "report.xlsx")
	request := workbookRequest(t, EnvironmentWorkbookExportRequest{"", review.ApprovalHash, destination})
	for _, contextID := range []string{"", state.ContextID + "-stale"} {
		if review, err := service.GetReview(context.Background(), contextID, `{"title":""}`); err == nil || review != nil {
			t.Fatal("stale owner received review", review, err)
		}
		if outcome, err := service.ExportReviewed(context.Background(), contextID, request); err == nil || outcome != nil {
			t.Fatal("stale owner received publication", outcome, err)
		}
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale owner left workbook", err)
	}
}
func TestLongEnvironmentWorkbookServicePhysicalPrelinkDriftAndPublishedWarning(t *testing.T) {
	for _, phase := range []string{"prelink", "published"} {
		service, state := workbookServiceFixture(t, false)
		review, err := service.GetReview(context.Background(), state.ContextID, `{"title":""}`)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "report.xlsx")
		request := workbookRequest(t, EnvironmentWorkbookExportRequest{"", review.ApprovalHash, destination})
		before := review.Preview.Report
		hooks := environmentWorkbookHooks{publication: artifactPublicationHooks{observe: func(current, _ string) error {
			if current != phase {
				return nil
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				return err
			}
			_, err = db.Exec(`UPDATE Sample_Env SET SiteNotes='unreported physical drift' WHERE PlotNumber='00337'`)
			return errors.Join(err, db.Close())
		}}}
		outcome, err := service.exportReviewed(context.Background(), state.ContextID, request, hooks)
		if phase == "prelink" {
			if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
				t.Fatal("raw source drift published", outcome, err)
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("rejected drift left artifact", err)
			}
		} else if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
			!strings.Contains(outcome.ErrorMessage, "do not replay") {
			t.Fatal("postcommit drift lost known published outcome", outcome, err)
		}
		actual, err := service.contexts.PreviewLongEnvironment(context.Background(), state.ContextID, LongEnvironmentRequest{""})
		if err != nil || !reflect.DeepEqual(actual.Report, before) {
			t.Fatal("drift seam did not isolate physical provenance from workbook report", err)
		}
	}
}
