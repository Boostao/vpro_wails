package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummaryDraftWorkbookPublicationInvisibleDrift(t *testing.T) {
	for _, phase := range []string{"reviewed", "staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryDraftWorkbookFixture(t, true)
			owner := service.source.environment.report.contexts.projects.sqlite
			mutateContextFixture(t, owner.attachments["VUser"], `INSERT INTO USysUserSpp
				(Code,ScientificName,LifeForm,CodeType) VALUES('UNUSED','Private one',1,'U')`)
			options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true}
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil {
				t.Fatal(err)
			}
			drift := func() {
				mutateContextFixture(t, owner.attachments["VUser"],
					`UPDATE USysUserSpp SET ScientificName='Private two' WHERE Code='UNUSED'`)
			}
			if phase == "reviewed" {
				drift()
			}
			destination := filepath.Join(t.TempDir(), "draft.xlsx")
			outcome, err := service.exportReviewed(t.Context(), state.ContextID, summaryDraftWorkbookExport(t, review, destination),
				summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
					if actual == phase {
						drift()
					}
					return nil
				}}})
			if phase == "reviewed" {
				if err == nil || outcome != nil || !strings.Contains(err.Error(), "reload review") {
					t.Fatal("invisible stale review accepted", outcome, err)
				}
			} else {
				expected := "not-published"
				if phase == "published" {
					expected = "published-with-errors"
				}
				if err != nil || outcome == nil || outcome.Status != expected || outcome.ErrorMessage == "" {
					t.Fatal("invisible peer drift lost publication state", outcome, err)
				}
				if phase == "published" && (outcome.SHA256 != review.WorkbookSHA256 ||
					!strings.Contains(outcome.ErrorMessage, "do not replay")) {
					t.Fatal("committed bytes became replayable or unknown", outcome)
				}
			}
			actual, readErr := os.ReadFile(destination)
			if phase == "published" {
				if readErr != nil || len(actual) != review.Bytes {
					t.Fatal("published bytes lost after drift", readErr)
				}
			} else if !errors.Is(readErr, os.ErrNotExist) {
				t.Fatal("refused publication left a destination", readErr)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || again.ApprovalHash == review.ApprovalHash || again.WorkbookSHA256 != review.WorkbookSHA256 {
				t.Fatal("complete raw-source approval did not distinguish unchanged visible bytes", err)
			}
		})
	}
}

func TestSummaryDraftWorkbookPublicationCancellationCleanupAndRetry(t *testing.T) {
	for _, mode := range []string{"legacy", "memberships", "summary"} {
		for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
			t.Run(mode+"/"+phase, func(t *testing.T) {
				service, state := summaryAncillaryDesktopFixture(t, false)
				options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6}
				if mode != "legacy" {
					options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: mode == "summary"}
				}
				review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
				if err != nil {
					t.Fatal(err)
				}
				destination := filepath.Join(t.TempDir(), "draft.xlsx")
				request := summaryDraftWorkbookExport(t, review, destination)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				reads := 0
				failure := errors.New("draft source cleanup failure")
				hooks := summaryVegetationWorkbookHooks{snapshot: publicationReadSnapshotHooks{
					commitRead: func(*sql.Tx) error {
						reads++
						if phase == "first-commit" && reads == 1 {
							return failure
						}
						if phase == "prelink-cancel" && reads == 2 || phase == "published-cancel" && reads == 3 {
							cancel()
						}
						return nil
					},
					rollbackRead: func(*sql.Tx) error {
						if phase == "first-cleanup" && reads == 1 || phase == "prelink-cleanup" && reads == 2 ||
							phase == "published-cleanup" && reads == 3 {
							return failure
						}
						return nil
					},
				}}
				outcome, err := service.exportReviewed(ctx, state.ContextID, request, hooks)
				if strings.HasPrefix(phase, "first-") {
					if !errors.Is(err, failure) || outcome != nil {
						t.Fatal("initial failed read invented an outcome", outcome, err)
					}
				} else {
					expected := "not-published"
					if strings.HasPrefix(phase, "published-") {
						expected = "published-with-errors"
					}
					if err != nil || outcome == nil || outcome.Status != expected || outcome.ErrorMessage == "" {
						t.Fatal("read cleanup/cancellation hid publication state", outcome, err)
					}
				}
				if strings.HasPrefix(phase, "published-") {
					if outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
						t.Fatal("postcommit failure lost the irreversible receipt", outcome)
					}
					if data, err := os.ReadFile(destination); err != nil || len(data) != review.Bytes {
						t.Fatal("committed workbook missing", err)
					}
				} else {
					if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("known failed operation created a destination", err)
					}
					retry, err := service.ExportReviewed(t.Context(), state.ContextID, request)
					if err != nil || retry == nil || retry.Status != "published" {
						t.Fatal("known precommit failure prevented safe retry", retry, err)
					}
				}
			})
		}
	}
}

func TestSummaryDraftWorkbookPublicationSavedQualityDrift(t *testing.T) {
	for _, phase := range []string{"staged", "published"} {
		service, state := summaryDraftWorkbookFixture(t, false)
		preferences := service.source.environment.report.contexts.projects.preferences
		review, err := service.GetReview(t.Context(), state.ContextID,
			workbookRequest(t, SummaryVegetationWorkbookOptions{Average: 1, Display: 6}))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "draft.xlsx")
		outcome, err := service.exportReviewed(t.Context(), state.ContextID, summaryDraftWorkbookExport(t, review, destination),
			summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
				if actual == phase {
					return preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": true})
				}
				return nil
			}}})
		expected := "not-published"
		if phase == "published" {
			expected = "published-with-errors"
		}
		if err != nil || outcome == nil || outcome.Status != expected ||
			!strings.Contains(outcome.ErrorMessage, "quality enforcement") {
			t.Fatal("saved quality drift was ignored or lost output authority", phase, outcome, err)
		}
	}
}
