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
	"time"

	"github.com/xuri/excelize/v2"
)

func projectWorkbookOwnedFixture(t *testing.T, external, none, all bool) (*LongVegetationProjectWorkbookService, ProjectState, *LongVegetationReportScope) {
	t.Helper()
	scope, state, built := projectPreviewFixture(t, external, none, all)
	service := NewLongVegetationProjectWorkbookService(scope.contexts, true, true)
	service.now = func() time.Time { return time.Date(2026, 10, 10, 23, 59, 59, 0, time.Local) }
	return service, state, built
}

func projectWorkbookReadJSON(t *testing.T, scope *LongVegetationReportScope) string {
	t.Helper()
	return workbookRequest(t, vegetationProjectWorkbookReadRequest{scope.Revision, scope.Build.ApprovalHash, "unlumped"})
}

func projectWorkbookExportJSON(t *testing.T, review *LongVegetationProjectWorkbookReview, destination string) string {
	t.Helper()
	return workbookRequest(t, vegetationProjectWorkbookExportRequest{review.Scope.Revision, review.Scope.Build.ApprovalHash,
		review.ReportScope, review.CreatedDate, review.ApprovalHash, destination})
}

func TestProjectWorkbookOwnedReviewPublicationActualPathsSummaryAndOnceOnlyBytes(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, mode := range []struct{ none, all bool }{{false, false}, {true, false}, {true, true}} {
			service, state, scope := projectWorkbookOwnedFixture(t, external, mode.none, mode.all)
			before := databaseBytes(t, service.scope.contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(service.scope.contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			review, err := service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
			if err != nil {
				t.Fatal(err)
			}
			count := 5
			if mode.all {
				count = 4
			}
			if !reflect.DeepEqual(review.Scope, *scope) || review.Preview.SUPath != state.SUPath || review.ReportScope != "unlumped" ||
				review.CreatedDate != "2026-10-10" || review.Summary == nil || review.Summary.EnvironmentRows != 5 ||
				review.Summary.SourcePlotRows != count || len(review.Scope.Build.ProjectTuples) != 5 || review.Bytes <= 0 {
				t.Fatal("owned project review lost original paths/counts/date/summary", review)
			}
			service.now = func() time.Time { return time.Date(2026, 10, 11, 0, 0, 1, 0, time.Local) }
			destination := filepath.Join(t.TempDir(), "project # 'é'.xlsx")
			request := projectWorkbookExportJSON(t, review, destination)
			outcome, err := service.ExportReviewed(context.Background(), state.ContextID, request)
			if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 || outcome.ErrorMessage != "" {
				t.Fatal("owned project publication failed or changed reviewed date", outcome, err)
			}
			raw, err := os.ReadFile(destination)
			if err != nil || len(raw) != review.Bytes {
				t.Fatal("published bytes differ from review", len(raw), err)
			}
			hash := sha256.Sum256(raw)
			if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
				t.Fatal("independently reopened SHA differs")
			}
			book, err := excelize.OpenReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			for address, expected := range map[string]string{
				"B11": "5", "B12": map[bool]string{true: "4", false: "5"}[mode.all],
				"D3": "Original project report tuples (desktop adaptation)", "A2": "Date created: 2026-10-10",
			} {
				actual, err := book.GetCellValue("ReportSummary", address)
				if err != nil || actual != expected {
					t.Fatal("owned XLSX lost original summary witness", address, actual, err)
				}
			}
			if err := book.Close(); err != nil {
				t.Fatal(err)
			}
			second, err := service.ExportReviewed(context.Background(), state.ContextID, request)
			if err != nil || second == nil || second.Status != "not-published" || second.Path != "" || second.SHA256 != "" || second.ErrorMessage == "" {
				t.Fatal("publication replay overwrote existing file", second, err)
			}
			after, err := os.ReadFile(service.scope.contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("workbook changed owned configuration", err)
			}
			assertProfileSUFiles(t, service.scope.contexts, before)
		}
	}
}

func TestProjectWorkbookStrictRequestsGatesStaleScopeAndSavedModeRefuse(t *testing.T) {
	service, state, scope := projectWorkbookOwnedFixture(t, false, false, false)
	request := projectWorkbookReadJSON(t, scope)
	for _, gates := range [][2]bool{{false, false}, {true, false}, {false, true}} {
		disabled := NewLongVegetationProjectWorkbookService(service.scope.contexts, gates[0], gates[1])
		if review, err := disabled.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
			t.Fatal("project workbook borrowed another capability", gates, err)
		}
	}
	for _, invalid := range []string{
		`{}`, strings.Replace(request, `"unlumped"`, `"current"`, 1),
		strings.Replace(request, `"revision":"3"`, `"revision":"03"`, 1),
		request[:len(request)-1] + `,"scope":"unlumped"}`,
		request[:len(request)-1] + `,"\u0073cope":"unlumped"}`,
		request[:len(request)-1] + `,"unknown":1}`,
	} {
		if review, err := service.GetReview(context.Background(), state.ContextID, invalid); err == nil || review != nil {
			t.Fatal("invalid strict project review succeeded", invalid, err)
		}
	}
	review, err := service.GetReview(context.Background(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	export := projectWorkbookExportJSON(t, review, filepath.Join(t.TempDir(), "never.xlsx"))
	for _, invalid := range []string{
		export[:len(export)-1] + `,"destination":"replacement.xlsx"}`,
		strings.Replace(export, `"createdDate":"2026-10-10"`, `"createdDate":"not-date"`, 1),
		strings.Replace(export, review.ApprovalHash, strings.Repeat("0", 64), 1),
		strings.Replace(export, `"scope":"unlumped"`, `"scope":"current"`, 1),
	} {
		if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, invalid); err == nil || outcome != nil {
			t.Fatal("invalid strict project export succeeded", outcome, err)
		}
	}
	if err := service.scope.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": 1}); err != nil {
		t.Fatal(err)
	}
	if review, err := service.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
		t.Fatal("project workbook ignored actual saved mode", err)
	}
	if err := service.scope.contexts.projects.preferences.update("ReportOptions", map[string]any{"LVUnitGroups": 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.scope.SetLongVegetationReportScope(context.Background(), state.ContextID,
		projectScopeRequestJSON(t, scope.Revision, vegetationSelectedListScope)); err != nil {
		t.Fatal(err)
	}
	if review, err := service.GetReview(context.Background(), state.ContextID, request); err == nil || review != nil {
		t.Fatal("stale project intent became selected-list workbook", err)
	}
}

func TestProjectWorkbookReadCleanupCancellationAndRetryHaveNoSuccessOutput(t *testing.T) {
	service, state, scope := projectWorkbookOwnedFixture(t, false, true, true)
	request := projectWorkbookReadJSON(t, scope)
	sentinel := errors.New("owned project read cleanup")
	service.snapshot.commitRead = func(*sql.Tx) error { return sentinel }
	if review, err := service.GetReview(context.Background(), state.ContextID, request); review != nil || !errors.Is(err, sentinel) {
		t.Fatal("failed read cleanup returned project approval", review, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if review, err := service.GetReview(ctx, state.ContextID, request); review != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read returned project approval", review, err)
	}
	if review, err := service.GetReview(context.Background(), state.ContextID, request); err != nil || review == nil {
		t.Fatal("read failure lost owned coordinator or retry", err)
	}
}

func TestProjectWorkbookPublicationPrelinkAndPostcommitDriftRetainCorrectReceipts(t *testing.T) {
	for _, phase := range []string{"prelink", "published"} {
		service, state, scope := projectWorkbookOwnedFixture(t, false, true, true)
		review, err := service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "owned.xlsx")
		mutated := false
		outcome, err := service.exportReviewed(context.Background(), state.ContextID,
			projectWorkbookExportJSON(t, review, destination), vegetationWorkbookHooks{publication: artifactPublicationHooks{
				observe: func(current, _ string) error {
					if current != phase || mutated {
						return nil
					}
					db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
					if err != nil {
						return err
					}
					_, err = db.Exec(`UPDATE Sample_Env SET PlotNumber='drifted' WHERE rowid=1`)
					mutated = true
					return errors.Join(err, db.Close())
				},
			}})
		if err != nil || outcome == nil || !mutated || outcome.ErrorMessage == "" {
			t.Fatal("physical source drift lost explicit receipt", phase, outcome, err)
		}
		if phase == "prelink" {
			if outcome.Status != "not-published" {
				t.Fatal("prelink drift published stale workbook", outcome)
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("prelink refusal left destination", err)
			}
		} else {
			if outcome.Status != "published-with-errors" || outcome.Path == "" || outcome.SHA256 != review.WorkbookSHA256 ||
				!strings.Contains(outcome.ErrorMessage, "do not replay") {
				t.Fatal("postcommit drift erased committed publication", outcome)
			}
			raw, err := os.ReadFile(destination)
			if err != nil || len(raw) != review.Bytes {
				t.Fatal("postcommit receipt lacks physical workbook", err)
			}
		}
	}
}

func TestProjectWorkbookPublicationCleanupFailuresCancelAndStagedCorruption(t *testing.T) {
	for _, scenario := range []string{"first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel", "staged-corruption"} {
		service, state, scope := projectWorkbookOwnedFixture(t, false, true, true)
		review, err := service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
		if err != nil {
			t.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		destination := filepath.Join(t.TempDir(), "failure.xlsx")
		reads := 0
		sentinel := errors.New("snapshot cleanup")
		outcome, err := service.exportReviewed(ctx, state.ContextID, projectWorkbookExportJSON(t, review, destination), vegetationWorkbookHooks{
			snapshot: publicationReadSnapshotHooks{rollbackRead: func(*sql.Tx) error {
				reads++
				if scenario == "first-cleanup" && reads == 1 || scenario == "prelink-cleanup" && reads == 2 ||
					scenario == "published-cleanup" && reads == 3 {
					return sentinel
				}
				return nil
			}},
			publication: artifactPublicationHooks{observe: func(phase, temporary string) error {
				if scenario == "prelink-cancel" && phase == "prelink" || scenario == "published-cancel" && phase == "published" {
					cancel()
				}
				if scenario == "staged-corruption" && phase == "prelink" {
					return os.WriteFile(temporary, []byte("corrupt"), 0600)
				}
				return nil
			}},
		})
		cancel()
		if scenario == "first-cleanup" {
			if outcome != nil || !errors.Is(err, sentinel) {
				t.Fatal("failed initial read produced publication receipt", outcome, err)
			}
		} else if strings.HasPrefix(scenario, "published-") {
			if err != nil || outcome == nil || outcome.Status != "published-with-errors" || outcome.SHA256 != review.WorkbookSHA256 || outcome.ErrorMessage == "" {
				t.Fatal("committed cleanup/cancel erased result", scenario, outcome, err)
			}
		} else if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
			t.Fatal("precommit failure produced committed output", scenario, outcome, err)
		}
		if scenario == "first-cleanup" || !strings.HasPrefix(scenario, "published-") {
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed export left destination", scenario, err)
			}
			if retry, err := service.ExportReviewed(context.Background(), state.ContextID, projectWorkbookExportJSON(t, review, destination)); err != nil ||
				retry == nil || retry.Status != "published" {
				t.Fatal("clean retry unavailable", retry, err)
			}
		}
	}
}

func TestProjectWorkbookOwnedQualityKeepsCompleteBuildOriginalSUAndSummarySeparate(t *testing.T) {
	service, state, scope := projectWorkbookOwnedFixture(t, true, false, false)
	owner := service.scope.contexts.projects.sqlite
	project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = project.Exec(`DELETE FROM Sample_Env WHERE rowid=2; DELETE FROM Sample_Admin WHERE rowid=2`)
	if err = errors.Join(err, project.Close()); err != nil {
		t.Fatal(err)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName='DataQuality';
				INSERT INTO USysTableOfLists(ListName,Item,ItemOrder)
				VALUES('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),('DataQuality','Good',3)`)
	if err = errors.Join(err, lists.Close()); err != nil {
		t.Fatal(err)
	}
	if err := service.scope.contexts.projects.preferences.update("ReportOptions", map[string]any{
		"DataQualityFilterEnforceLV": true, "DataQualityFilterSiteLV": "Good",
		"DataQualityFilterVegLV": "Good", "DataQualityFilterSoilLV": "Good",
	}); err != nil {
		t.Fatal(err)
	}
	scope, err = service.scope.BuildLongVegetationProjectScope(context.Background(), state.ContextID, scope.Revision)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, owner.attachments)
	review, err := service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
	if err != nil || review == nil || review.Preview.Report.Quality == nil || len(review.Preview.Report.Quality.Occurrences) != 8 ||
		len(review.Scope.Build.ProjectTuples) != 2 || len(review.Preview.Report.ProjectUnits) != 1 ||
		review.Preview.Report.Units[0].NumPlots != 1 || review.Summary == nil || review.Summary.SourcePlotRows != 1 ||
		review.Summary.EnvironmentRows != 4 || len(review.Summary.ProjectTuples) != 1 {
		t.Fatal("owned original quality/Build/summary provenance conflated", review, err)
	}
	destination := filepath.Join(t.TempDir(), "qualified.xlsx")
	outcome, err := service.ExportReviewed(context.Background(), state.ContextID, projectWorkbookExportJSON(t, review, destination))
	if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
		t.Fatal("owned qualified project workbook unavailable", outcome, err)
	}
	book, err := excelize.OpenFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := book.GetCellValue("ReportSummary", "B12"); err != nil || count != "1" {
		t.Fatal("QCLV occurrence multiplicity weighted published project summary", count, err)
	}
	if err := book.Close(); err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, service.scope.contexts, before)
}

func TestProjectWorkbookOwnedMetadataDriftAndOwnerLeaseThroughIrreversibleLink(t *testing.T) {
	service, state, scope := projectWorkbookOwnedFixture(t, false, true, true)
	review, err := service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := sql.Open("sqlite3", sqliteFileURI(service.scope.contexts.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = metadata.Exec(`UPDATE _table_metadata SET description='changed literal Description' WHERE table_name='USysAllSpecs'`)
	if err = errors.Join(err, metadata.Close()); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "metadata.xlsx")
	if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, projectWorkbookExportJSON(t, review, destination)); err == nil || outcome != nil {
		t.Fatal("changed native Description passed old approval", outcome, err)
	}
	review, err = service.GetReview(context.Background(), state.ContextID, projectWorkbookReadJSON(t, scope))
	if err != nil || review.Summary == nil || review.Summary.SpeciesVersion.Text == nil || *review.Summary.SpeciesVersion.Text != "changed literal Description" {
		t.Fatal("native Description correction/review unavailable", review, err)
	}
	linkCalls := 0
	outcome, err := service.exportReviewed(context.Background(), state.ContextID, projectWorkbookExportJSON(t, review, destination),
		vegetationWorkbookHooks{publication: artifactPublicationHooks{link: func(from, to string) error {
			linkCalls++
			short, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if _, err := service.scope.SetLongVegetationReportScope(short, state.ContextID,
				projectScopeRequestJSON(t, scope.Revision, vegetationSelectedListScope)); !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("scope switch interleaved with irreversible file link: %v", err)
			}
			return os.Link(from, to)
		}}})
	if err != nil || outcome == nil || outcome.Status != "published" || linkCalls != 1 {
		t.Fatal("project publication lost owner lease or committed result", outcome, err)
	}
	if _, err := service.scope.contexts.SwitchContext(state.ContextID, contextSelection(state)); err != nil {
		t.Fatal("publication leaked owner/context lease", err)
	}
	if outcome, err := service.ExportReviewed(context.Background(), state.ContextID, projectWorkbookExportJSON(t, review, filepath.Join(t.TempDir(), "retired.xlsx"))); err == nil || outcome != nil {
		t.Fatal("retired owner retained project publication authority", outcome, err)
	}
}
