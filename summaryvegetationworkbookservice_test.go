package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryDraftWorkbookFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	source, state := summaryEnvironmentalLifeformServiceFixture(t, external)
	service, err := NewSummaryVegetationWorkbookService(source, func(key string) (string, bool) {
		if key != summaryVegetationWorkbookFeatureEnvironment {
			t.Fatal("draft workbook borrowed another feature gate", key)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func summaryDraftWorkbookExport(t *testing.T, review *SummaryVegetationWorkbookReview, destination string) string {
	t.Helper()
	return workbookRequest(t, summaryVegetationWorkbookExportRequest{review.Options, review.ApprovalHash, destination})
}

func TestSummaryDraftWorkbookServiceReviewPublishCollisionAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryDraftWorkbookFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, includeEnv := range []bool{false, true} {
			options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: 1, Display: 6, IncludeEnv: includeEnv}
			request := workbookRequest(t, options)
			review, err := service.GetReview(t.Context(), state.ContextID, request)
			if err != nil || review == nil || review.Options != options || review.Preview.ContextID != state.ContextID ||
				review.Scope != summaryVegetationWorkbookDraftScope || review.Sheet != state.ActiveProject ||
				review.Bytes <= 0 || (review.Environment != nil) != includeEnv {
				t.Fatal("owned explicit draft review differs", review, err)
			}
			first := 6
			if includeEnv {
				first = 36
				if len(review.Environment.Rows) != 30 || len(review.Environment.FieldList.Rows) != 31 {
					t.Fatal("complete environmental input lost", review.Environment)
				}
			}
			if review.FirstVegetationRow != first {
				t.Fatal("source-related body placement differs", review.FirstVegetationRow, first)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, request)
			if err != nil || !reflect.DeepEqual(review, again) {
				t.Fatal("unchanged review is not deterministic", err)
			}
			destination := filepath.Join(t.TempDir(), "draft # ' é.xlsx")
			export := summaryDraftWorkbookExport(t, review, destination)
			outcome, err := service.ExportReviewed(t.Context(), state.ContextID, export)
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.RequestedDestination != destination || outcome.Path == "" ||
				outcome.SHA256 != review.WorkbookSHA256 {
				t.Fatal("reviewed draft not published", outcome, err)
			}
			requestedInfo, requestedErr := os.Stat(destination)
			publishedInfo, publishedErr := os.Stat(outcome.Path)
			if requestedErr != nil || publishedErr != nil || !os.SameFile(requestedInfo, publishedInfo) {
				t.Fatal("resolved published path does not own requested file", requestedErr, publishedErr)
			}
			data, err := os.ReadFile(destination)
			if err != nil || len(data) != review.Bytes {
				t.Fatal("published bytes missing or wrong length", err)
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
				t.Fatal("published bytes differ from reviewed hash")
			}
			book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: data})
			title, err := book.GetCellValue(review.Sheet, "A1")
			formula, formulaErr := book.GetCellFormula(review.Sheet, "A1")
			if err != nil || formulaErr != nil || title != "=literal 001" || formula != "" {
				t.Fatal("published literal text became a formula", title, formula, err, formulaErr)
			}
			collision, err := service.ExportReviewed(t.Context(), state.ContextID, export)
			if err != nil || collision == nil || collision.Status != "not-published" || collision.ErrorMessage == "" {
				t.Fatal("existing destination was overwritten or refusal hidden", collision, err)
			}
			actual, err := os.ReadFile(destination)
			if err != nil || !bytes.Equal(data, actual) {
				t.Fatal("collision modified existing workbook", err)
			}
		}
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) || !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
			t.Fatal("draft publication changed physical data/audits or preferences", err)
		}
	}
}

func TestSummaryDraftWorkbookServiceGateStrictOptionsAndApproval(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSummaryVegetationWorkbookService(nil, func(string) (string, bool) { return flag, flag != "" })
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal("independent literal gate differs", flag, err)
			}
			if review, err := service.GetReview(t.Context(), "owned", `{}`); err == nil || review != nil {
				t.Fatal("unavailable service returned review", review, err)
			}
		} else if err == nil {
			t.Fatal("malformed flag accepted", flag)
		}
	}
	service, state := summaryDraftWorkbookFixture(t, false)
	valid := `{"title":"","average":1,"display":6,"showClass":0,"includeEnv":false}`
	for _, request := range []string{
		`{}`, `null`, strings.Replace(valid, `"showClass":0`, `"showClass":null`, 1),
		strings.Replace(valid, `"includeEnv":false`, `"includeEnv":null`, 1),
		strings.Replace(valid, `"average":1`, `"average":"1"`, 1),
		strings.Replace(valid, `"title":""`, `"Title":""`, 1),
		strings.Replace(valid, `"title":""`, `"title":"","title":"later"`, 1),
		strings.Replace(valid, `"title":""`, `"title":"","\u0074itle":"later"`, 1),
		strings.Replace(valid, `"title":""`, `"title":"\ud800"`, 1),
		strings.Replace(valid, `"display":6`, `"display":7`, 1),
		strings.Replace(valid, `"includeEnv":false`, `"includeEnv":false,"extra":1`, 1),
	} {
		if review, err := service.GetReview(t.Context(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("malformed options accepted or repaired", request, review, err)
		}
	}
	review, err := service.GetReview(t.Context(), state.ContextID, valid)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "draft.xlsx")
	export := summaryDraftWorkbookExport(t, review, destination)
	for _, request := range []string{
		`{}`, `{"options":null,"approvalHash":"","destination":"draft.xlsx"}`,
		strings.Replace(export, review.ApprovalHash, strings.ToUpper(review.ApprovalHash), 1),
		strings.Replace(export, `"average":1`, `"average":2`, 1),
		strings.Replace(export, `"destination":`, `"unknown":`, 1),
		strings.Replace(export, `"options":`, `"options":null,"options":`, 1),
		strings.Replace(export, `"showClass":0`, `"showClass":null`, 1),
		strings.Replace(export, `.xlsx`, `.db`, 1),
	} {
		if outcome, err := service.ExportReviewed(t.Context(), state.ContextID, request); err == nil || outcome != nil {
			t.Fatal("malformed/stale publication request accepted", request, outcome, err)
		}
	}
	if _, err := service.GetReview(nil, state.ContextID, valid); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := service.GetReview(context.Background(), "stale", valid); err == nil {
		t.Fatal("stale context accepted")
	}
}

func TestSummaryDraftWorkbookServiceSavedQualityCannotBeBypassed(t *testing.T) {
	service, state := summaryDraftWorkbookFixture(t, false)
	report := service.source.environment.report
	if report.qualityPreferences {
		t.Fatal("fixture requires legacy quality gate off to prove independent refusal")
	}
	request := workbookRequest(t, SummaryVegetationWorkbookOptions{Average: 1, Display: 6})
	for _, enforce := range []any{true, "False", nil} {
		if err := report.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": enforce}); err != nil {
			t.Fatal(err)
		}
		if review, err := service.GetReview(t.Context(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("saved unavailable/malformed quality policy was ignored", enforce, review, err)
		}
	}
	if err := report.contexts.projects.preferences.update("ReportOptions", map[string]any{"DataQualityFilterEnforce": false}); err != nil {
		t.Fatal(err)
	}
	if review, err := service.GetReview(t.Context(), state.ContextID, request); err != nil || review == nil {
		t.Fatal("quality correction did not permit explicit unfiltered draft", err)
	}
}
