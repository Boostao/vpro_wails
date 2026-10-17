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
	"time"

	"github.com/xuri/excelize/v2"
)

const summaryAncillaryReviewedTime = "2026-10-17T02:00:00Z"

func summaryAncillaryDesktopFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryDraftWorkbookFixture(t, external)
	service.ancillaryEnabled = true
	service.now = func() time.Time { return time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC) }
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		DROP TABLE IF EXISTS MasterSiteUnitList;
		CREATE TABLE MasterSiteUnitList(SiteSeries TEXT, SiteSeriesLongName TEXT);
		INSERT INTO MasterSiteUnitList VALUES('A','=Alpha_x0041_'),('A','=Alpha_x0041_'),('B',NULL),('unused','Retain');
		DROP TABLE IF EXISTS _table_metadata;
		CREATE TABLE _table_metadata(table_name TEXT, description TEXT);
		INSERT INTO _table_metadata VALUES('USysAllSpecs','fixture version');`)
	return service, state
}

func TestSummaryAncillaryDesktopStrictOptionalRequests(t *testing.T) {
	base := `{"title":"literal","average":1,"display":6,"showClass":0,"includeEnv":false`
	for _, suffix := range []string{
		`,"ancillary":null}`, `,"ancillary":{}}`, `,"ancillary":{"createSummary":null}}`,
		`,"ancillary":{"createSummary":"false"}}`, `,"ancillary":{"createSummary":false,"extra":0}}`,
		`,"ancillary":{"CreateSummary":false}}`,
		`,"ancillary":{"createSummary":true,"\u0063reateSummary":false}}`,
		`,"ancillary":{"createSummary":false},"\u0061ncillary":{"createSummary":true}}`,
		`,"Ancillary":{"createSummary":false}}`, `,"title":"duplicate"}`,
		`,"ancillary":{"createSummary":false}} {}`,
	} {
		var options SummaryVegetationWorkbookOptions
		if err := json.Unmarshal([]byte(base+suffix), &options); err == nil {
			t.Fatal("ambiguous optional authority accepted", suffix)
		}
	}
	for _, suffix := range []string{`}`, `,"ancillary":{"createSummary":false}}`, `,"ancillary":{"createSummary":true}}`} {
		var options SummaryVegetationWorkbookOptions
		if err := json.Unmarshal([]byte(base+suffix), &options); err != nil {
			t.Fatal("explicit legacy/ancillary mode refused", suffix, err)
		}
		encoded, err := json.Marshal(options)
		if err != nil || string(encoded) != base+suffix {
			t.Fatal("optional authority changed wire bytes", string(encoded), err)
		}
	}
	if err := json.Unmarshal([]byte(strings.Replace(base, "literal", `\ud800`, 1)+`}`),
		new(SummaryVegetationWorkbookOptions)); err == nil {
		t.Fatal("raw invalid Unicode repaired before validation")
	}
	options := base + `,"ancillary":{"createSummary":false}}`
	publication := `{"options":` + options + `,"approvalHash":"` + strings.Repeat("0", 64) + `","destination":"out.xlsx"`
	for _, suffix := range []string{
		`}`, `,"createdAt":null}`, `,"createdAt":""}`, `,"createdAt":"2026-10-17T02:00:00+00:00"}`,
		`,"createdAt":"2026-10-17T02:00:00.000Z"}`, `,"createdAt":"0099-10-17T02:00:00Z"}`,
		`,"createdAt":"2026-02-30T02:00:00Z"}`, `,"CreatedAt":"2026-10-17T02:00:00Z"}`,
		`,"createdAt":"2026-10-17T02:00:00Z","\u0063reatedAt":"2026-10-18T02:00:00Z"}`,
	} {
		if err := json.Unmarshal([]byte(publication+suffix), new(summaryVegetationWorkbookExportRequest)); err == nil {
			t.Fatal("missing/ambiguous/unreviewable timestamp accepted", suffix)
		}
	}
	valid := publication + `,"createdAt":"` + summaryAncillaryReviewedTime + `"}`
	var request summaryVegetationWorkbookExportRequest
	if err := json.Unmarshal([]byte(valid), &request); err != nil || request.CreatedAt != summaryAncillaryReviewedTime {
		t.Fatal("reviewed time refused", request, err)
	}
	legacyWithTime := strings.Replace(valid, options, base+`}`, 1)
	if err := json.Unmarshal([]byte(legacyWithTime), &request); err == nil {
		t.Fatal("legacy publication silently ignored an ancillary timestamp")
	}
}

func TestSummaryAncillaryDesktopModesReviewedTimeReopenAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryAncillaryDesktopFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, includeEnv := range []bool{false, true} {
			for _, summary := range []bool{false, true} {
				for _, average := range []int{1, 2} {
					options := SummaryVegetationWorkbookOptions{Title: "=literal title", Average: average, Display: 6,
						ShowClass: 20, IncludeEnv: includeEnv,
						Ancillary: &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: summary}}
					review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
					if err != nil || review == nil || review.Ancillary == nil {
						t.Fatal("owned ancillary review unavailable", external, includeEnv, summary, average, err)
					}
					ancillary := review.Ancillary
					if review.Scope != summaryVegetationAncillaryWorkbookScope ||
						ancillary.CreatedAt != summaryAncillaryReviewedTime || ancillary.MembershipRows != 8 ||
						(ancillary.Summary != nil) != summary {
						t.Fatal("conditional reviewed authority changed", review)
					}
					if summary {
						value := ancillary.Summary
						if value.EnvironmentRows != 3 || !value.EnvironmentCountAvailable || value.SelectedPlotRows != 7 ||
							value.Version != "fixture version" || value.VersionStatus != "literal" ||
							len(value.Units) != 2 || value.Units[0].Code != "A" || value.Units[0].NumPlots != 8 ||
							value.Units[0].LongName == nil || *value.Units[0].LongName != "=Alpha_x0041_" ||
							value.Units[1].Code != "B" || value.Units[1].NumPlots != 1 || value.Units[1].LongName != nil {
							t.Fatal("source join/count/NULL review differs", value)
						}
					}
					clock := service.now
					service.now = func() time.Time { t.Fatal("publication regenerated its reviewed time"); return time.Time{} }
					destination := filepath.Join(t.TempDir(), "owned inspection.xlsx")
					result, err := service.ExportReviewed(t.Context(), state.ContextID, summaryDraftWorkbookExport(t, review, destination))
					service.now = clock
					if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
						t.Fatal("reviewed bytes/time did not publish", result, err)
					}
					data, err := os.ReadFile(destination)
					digest := sha256.Sum256(data)
					if err != nil || len(data) != review.Bytes || hex.EncodeToString(digest[:]) != review.WorkbookSHA256 {
						t.Fatal("reopened destination differs from review", err)
					}
					book, err := excelize.OpenReader(bytes.NewReader(data))
					if err != nil {
						t.Fatal(err)
					}
					sheets := append(append([]string{}, ancillary.Sheets...), "_VPRO_Source")
					if !reflect.DeepEqual(book.GetSheetList(), sheets) {
						t.Fatal("published sheet list differs from visible review", book.GetSheetList(), sheets)
					}
					checks := map[string]string{"A1": "Plotnumber", "A2": "P1", "A7": "", "B7": "A", "A9": "P6", "B9": "Orphan"}
					for address, expected := range checks {
						got, err := book.GetCellValue("SU Table", address, excelize.Options{RawCellValue: true})
						if err != nil || got != expected {
							t.Fatal("physical membership lost at desktop publication", address, got, expected, err)
						}
					}
					if summary {
						for address, expected := range map[string]string{"A2": "Date created: " + summaryAncillaryReviewedTime,
							"B10": "3", "B11": "7", "D5": "A", "E5": "8", "F5": "=Alpha_x0041_", "F6": ""} {
							got, err := book.GetCellValue("ReportSummary", address, excelize.Options{RawCellValue: true})
							if err != nil || got != expected {
								t.Fatal("summary differs from the owned review", address, got, expected, err)
							}
						}
					}
					if err := book.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		afterConfig, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, afterConfig) ||
			!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
			t.Fatal("inspection changed source/config/audits", err)
		}
	}
}

func TestSummaryAncillaryDesktopDefaultOffAndSummaryOnlyInputs(t *testing.T) {
	service, state := summaryAncillaryDesktopFixture(t, false)
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6}
	service.ancillaryEnabled = false
	legacy, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil || legacy.Ancillary != nil || legacy.Scope != summaryVegetationWorkbookDraftScope {
		t.Fatal("independent ancillary gate changed legacy availability", legacy, err)
	}
	options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{}
	if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err == nil || result != nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("disabled ancillary mode fell back to legacy", result, err)
	}
	service.ancillaryEnabled = true
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `DROP TABLE MasterSiteUnitList`)
	if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err != nil || result == nil || result.Ancillary.Summary != nil {
		t.Fatal("SU-only mode read summary-only inputs", result, err)
	}
	options.Ancillary.CreateSummary = true
	if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err == nil || result != nil {
		t.Fatal("missing physical summary source became empty/default success", result, err)
	}
	mutateContextFixture(t, owner.attachments["VLists"], `CREATE TABLE MasterSiteUnitList(SiteSeries TEXT, SiteSeriesLongName TEXT);
		INSERT INTO MasterSiteUnitList VALUES('unused',CAST(X'FF' AS TEXT))`)
	if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err == nil || result != nil {
		t.Fatal("malformed unused summary peer escaped full-source validation", result, err)
	}
	mutateContextFixture(t, owner.attachments["VLists"], `UPDATE MasterSiteUnitList SET SiteSeriesLongName='Corrected'`)
	if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err != nil || result == nil {
		t.Fatal("read correction/retry unavailable", result, err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := service.GetReview(cancelled, state.ContextID, workbookRequest(t, options)); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled ancillary review returned a plan", result, err)
	}
}

func TestSummaryAncillaryDesktopAdditionalPeerDriftRetainsPublicationResult(t *testing.T) {
	for _, phase := range []string{"reviewed", "staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryAncillaryDesktopFixture(t, true)
			options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true,
				Ancillary: &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}}
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil {
				t.Fatal(err)
			}
			owner := service.source.environment.report.contexts.projects.sqlite
			drift := func() {
				mutateContextFixture(t, owner.attachments["VLists"],
					`UPDATE MasterSiteUnitList SET SiteSeriesLongName='Changed unused peer' WHERE SiteSeries='unused'`)
			}
			if phase == "reviewed" {
				drift()
			}
			destination := filepath.Join(t.TempDir(), "ancillary.xlsx")
			result, err := service.exportReviewed(t.Context(), state.ContextID, summaryDraftWorkbookExport(t, review, destination),
				summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
					if actual == phase {
						drift()
					}
					return nil
				}}})
			if phase == "reviewed" {
				if err == nil || result != nil || !strings.Contains(err.Error(), "reload review") {
					t.Fatal("unrendered additional peer escaped stale approval", result, err)
				}
			} else {
				expected := "not-published"
				if phase == "published" {
					expected = "published-with-errors"
				}
				if err != nil || result == nil || result.Status != expected || result.ErrorMessage == "" {
					t.Fatal("additional-source drift lost publication state", result, err)
				}
				if phase == "published" && (result.SHA256 != review.WorkbookSHA256 ||
					!strings.Contains(result.ErrorMessage, "do not replay")) {
					t.Fatal("committed ancillary result became replayable", result)
				}
			}
			if _, err := os.Stat(destination); phase != "published" && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused ancillary publication left output", err)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || again.ApprovalHash == review.ApprovalHash || again.WorkbookSHA256 == review.WorkbookSHA256 {
				t.Fatal("complete additional source was not bound into approval and provenance", err)
			}
		})
	}
}

func TestSummaryAncillaryDesktopTimestampModeGateAndCollisionBindApproval(t *testing.T) {
	service, state := summaryAncillaryDesktopFixture(t, false)
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6,
		Ancillary: &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "literal # ' workbook.xlsx")
	request := summaryVegetationWorkbookExportRequest{Options: options, ApprovalHash: review.ApprovalHash,
		Destination: destination, CreatedAt: review.Ancillary.CreatedAt}
	for _, mutation := range []string{"timestamp", "summary", "legacy", "gate"} {
		changed := request
		switch mutation {
		case "timestamp":
			changed.CreatedAt = "2026-10-18T02:00:00Z"
		case "summary":
			changed.Options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{}
		case "legacy":
			changed.Options.Ancillary = nil
			changed.CreatedAt = ""
		case "gate":
			service.ancillaryEnabled = false
		}
		result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, changed))
		service.ancillaryEnabled = true
		if err == nil || result != nil {
			t.Fatal("changed creation/mode/availability reused approval", mutation, result, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("rejected authority created output", mutation, err)
		}
	}
	result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "published" {
		t.Fatal("original reviewed authority did not recover", result, err)
	}
	before, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "not-published" || result.ErrorMessage == "" {
		t.Fatal("existing ancillary destination was replaced or refusal hidden", result, err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("collision changed reviewed output", err)
	}
}

func TestSummaryAncillaryDesktopReviewPreservesDescriptionAndEmptyCountStates(t *testing.T) {
	base := summaryAncillaryWorkbookFixture(t, false)
	for _, state := range []string{"metadata-unavailable", "missing-definition", "null", "literal"} {
		input := cloneSummaryAncillaryWorkbook(t, base)
		input.Options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}
		input.Tables.Env.Rows = []ProjectMetadataRow{}
		switch state {
		case "metadata-unavailable":
			input.Tables.Descriptions = ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
		case "missing-definition":
			input.Tables.Descriptions.Rows = []ProjectMetadataRow{}
		case "null":
			input.Tables.Descriptions.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "null"}
		case "literal":
			input.Tables.Descriptions.Rows[0].Cells[1] = metadataText("")
		}
		input.Source.Ancillary = &summaryVegetationWorkbookAncillarySource{
			CreatedAt: input.CreatedAt, Tables: input.Tables,
		}
		workbook, err := prepareSummaryWorkbookSource(t.Context(), input.Source, input.Options)
		if err != nil {
			t.Fatal(state, err)
		}
		review, err := summaryWorkbookAncillaryReview(input.Source, workbook, input.Options)
		wantVersion := "Unknown"
		if state == "literal" {
			wantVersion = ""
		}
		if err != nil || review.Summary.EnvironmentRows != 0 || review.Summary.EnvironmentCountAvailable ||
			review.Summary.VersionStatus != state || review.Summary.Version != wantVersion {
			t.Fatal("review repaired NULL/empty/missing metadata or unavailable count", state, review, err)
		}
	}
}

func readSummaryAncillaryDesktopSource(t *testing.T, service *SummaryVegetationWorkbookService, state ProjectState,
	options SummaryVegetationWorkbookOptions, createdAt string) summaryVegetationWorkbookSource {
	t.Helper()
	contexts := service.source.environment.report.contexts
	result, err := withContextPlotRequest(t.Context(), contexts, state.ContextID, func(plots *PlotService) (summaryVegetationWorkbookSource, error) {
		return withOwnedPreferenceSnapshot(t.Context(), plots, publicationReadSnapshotHooks{},
			func(owner *sqliteContext, tx *sql.Tx) (summaryVegetationWorkbookSource, error) {
				return service.readSourceAt(t.Context(), plots, owner, tx, state.ContextID, options, createdAt)
			})
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
