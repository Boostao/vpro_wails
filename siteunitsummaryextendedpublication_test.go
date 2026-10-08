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

func summaryExtendedJSON(t *testing.T, request SiteUnitSummaryExtendedWorkbookOptions) string {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func summaryExtendedExportJSON(t *testing.T, request SiteUnitSummaryExtendedWorkbookOptions, approval, destination string) string {
	t.Helper()
	data, err := json.Marshal(siteUnitSummaryExtendedExportRequest{request.Method, request.OrderBy, request.IncludeSpecies,
		request.CoverCalculation, request.AndOr, request.PresenceGreaterThan, request.CoverGreaterThan, approval, destination})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSummaryExtendedPublicationOwnedModesDeterminismCollisionAndZeroDataWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := reportServiceFixture(t, external)
		service.siteUnitSummarySpeciesEnabled, service.siteUnitSummaryLifeformsEnabled = true, true
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			for _, mode := range [][2]int{{2, 0}, {1, 1}, {2, 1}} {
				options := SiteUnitSummaryExtendedWorkbookOptions{method, mode[0], mode[1], 1, 1, 0, 0}
				request := summaryExtendedJSON(t, options)
				review, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, request)
				if err != nil || review == nil || review.Options != options || review.Environment.Report.Method != method ||
					(review.Species != nil) != (mode[1] == 1) || len(review.Environment.Report.Fields) != map[int]int{1: 39, 2: 49}[mode[0]] {
					t.Fatal("owned extended review differs", review, err)
				}
				again, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, request)
				if err != nil || !reflect.DeepEqual(again, review) {
					t.Fatal("unchanged review is nondeterministic", err)
				}
				destination := filepath.Join(t.TempDir(), "extended.xlsx")
				export := summaryExtendedExportJSON(t, options, review.ApprovalHash, destination)
				outcome, err := exportSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, export, siteUnitSummaryWorkbookHooks{})
				if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
					t.Fatal("exact reviewed extended workbook not published", outcome, err)
				}
				data, err := os.ReadFile(destination)
				if err != nil || len(data) != review.Bytes {
					t.Fatal("published byte count differs", err)
				}
				hash := sha256.Sum256(data)
				if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 {
					t.Fatal("published bytes differ from reviewed SHA256")
				}
				collision, err := exportSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, export, siteUnitSummaryWorkbookHooks{})
				if err != nil || collision == nil || collision.Status != "not-published" || collision.ErrorMessage == "" {
					t.Fatal("collision hid refusal/replaced output", collision, err)
				}
				after, err := os.ReadFile(destination)
				if err != nil || !bytes.Equal(data, after) {
					t.Fatal("collision changed existing output", err)
				}
			}
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("extended workbook changed configuration", err)
		}
	}
}

func TestSummaryExtendedPublicationStrictTransportIndependentGatesAndScope(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	request := SiteUnitSummaryExtendedWorkbookOptions{1, 2, 1, 1, 1, 0, 0}
	raw := summaryExtendedJSON(t, request)
	for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}} {
		service.siteUnitSummarySpeciesEnabled, service.siteUnitSummaryLifeformsEnabled = flags[0], flags[1]
		if value, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, raw); err == nil || value != nil {
			t.Fatal("independent feature denial bypassed", flags, err)
		}
	}
	service.siteUnitSummarySpeciesEnabled, service.siteUnitSummaryLifeformsEnabled = true, true
	for _, malformed := range []string{
		`{}`, strings.Replace(raw, `"method":1`, `"method":null`, 1),
		strings.Replace(raw, `"method":1`, `"method":1.5`, 1),
		strings.Replace(raw, `"orderBy":2`, `"orderBy":3`, 1),
		strings.Replace(raw, `"includeSpecies":1`, `"includeSpecies":2`, 1),
		strings.Replace(raw, `"method":1`, `"method":1,"method":1`, 1),
		strings.Replace(raw, `"method":1`, `"method":1,"extra":"\ud800"`, 1),
		strings.Replace(raw, `"coverGreaterThan":0`, `"coverGreaterThan":32768`, 1),
	} {
		if value, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, malformed); err == nil || value != nil {
			t.Fatal("malformed extended transport accepted", malformed, err)
		}
	}
	if value, err := reviewSiteUnitSummaryExtendedWorkbook(nil, service, state.ContextID, raw); err == nil || value != nil {
		t.Fatal("nil context accepted", err)
	}
	if err := service.projects.preferences.update("ReportOptions", map[string]any{"SESuType": 2}); err != nil {
		t.Fatal(err)
	}
	if value, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, raw); err == nil || value != nil {
		t.Fatal("unavailable hierarchy scope was overridden", err)
	}
}

func TestSummaryExtendedPublicationRawSevenTableDriftAndOptions(t *testing.T) {
	for _, source := range []struct{ role, table string }{
		{"project", "Sample_Env"}, {"project", "Sample_Admin"}, {"su", "Selected_SU"},
		{"VLists", "MasterSiteUnitList"}, {"project", "Sample_Veg"},
		{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"},
	} {
		t.Run(source.table, func(t *testing.T) {
			service, state := reportServiceFixture(t, true)
			service.siteUnitSummarySpeciesEnabled, service.siteUnitSummaryLifeformsEnabled = true, true
			options := SiteUnitSummaryExtendedWorkbookOptions{1, 2, 1, 1, 1, 0, 0}
			request := summaryExtendedJSON(t, options)
			review, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, request)
			if err != nil {
				t.Fatal(err)
			}
			table := source.table
			if source.role == "project" {
				table = strings.Replace(table, "Sample_", state.ActiveProject+"_", 1)
			}
			if source.role == "su" {
				table = state.ActiveSU + "_SU"
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments[source.role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`ALTER TABLE ` + quoteHeaderIdentifier(table) + ` ADD COLUMN UnrenderedWitness TEXT`)
			if err = errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			fresh, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, request)
			if err != nil || fresh.WorkbookSHA256 != review.WorkbookSHA256 || fresh.ApprovalHash == review.ApprovalHash {
				t.Fatal("unrendered physical source drift changed output or escaped approval", err)
			}
			destination := filepath.Join(t.TempDir(), "drift.xlsx")
			if value, err := exportSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID,
				summaryExtendedExportJSON(t, options, review.ApprovalHash, destination), siteUnitSummaryWorkbookHooks{}); err == nil || value != nil {
				t.Fatal("raw seven-table drift accepted", err)
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused drift left destination", err)
			}
		})
	}
}

func TestSummaryExtendedPublicationCancellationAndIrreversibleReceipts(t *testing.T) {
	for _, phase := range []string{"first-commit", "first-cleanup", "prelink-cleanup", "published-cleanup", "prelink-cancel", "published-cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := reportServiceFixture(t, false)
			service.siteUnitSummarySpeciesEnabled, service.siteUnitSummaryLifeformsEnabled = true, true
			options := SiteUnitSummaryExtendedWorkbookOptions{1, 2, 1, 1, 1, 0, 0}
			review, err := reviewSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID, summaryExtendedJSON(t, options))
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "extended.xlsx")
			request := summaryExtendedExportJSON(t, options, review.ApprovalHash, destination)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads := 0
			sentinel := errors.New("extended read completion/cleanup failure")
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
			outcome, err := exportSiteUnitSummaryExtendedWorkbook(ctx, service, state.ContextID, request, hooks)
			switch phase {
			case "first-commit", "first-cleanup":
				if !errors.Is(err, sentinel) || outcome != nil {
					t.Fatal("failed initial capture invented receipt", outcome, err)
				}
			case "prelink-cleanup", "prelink-cancel":
				if err != nil || outcome == nil || outcome.Status != "not-published" || outcome.ErrorMessage == "" {
					t.Fatal("known no-file failure hidden", outcome, err)
				}
			default:
				if err != nil || outcome == nil || outcome.Status != "published-with-errors" ||
					outcome.SHA256 != review.WorkbookSHA256 || !strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("committed output became replayable", outcome, err)
				}
			}
			if strings.HasPrefix(phase, "published-") {
				if data, err := os.ReadFile(destination); err != nil || len(data) != review.Bytes {
					t.Fatal("committed output disappeared", err)
				}
			} else {
				if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("known failure left output", err)
				}
				if retry, err := exportSiteUnitSummaryExtendedWorkbook(context.Background(), service, state.ContextID,
					request, siteUnitSummaryWorkbookHooks{}); err != nil || retry == nil || retry.Status != "published" {
					t.Fatal("known refusal prevented safe retry", retry, err)
				}
			}
		})
	}
}
