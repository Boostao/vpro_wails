package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSummaryConstancyWorkbookOwnedPublicationAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryConstancyDesktopFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, average := range []int{1, 2} {
			for _, combined := range []bool{false, true} {
				options := summaryConstancyWorkbookOptions()
				options.Average, options.IncludeEnv = average, combined
				if combined {
					options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}
				}
				review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
				if err != nil {
					t.Fatal(err)
				}
				destination := filepath.Join(t.TempDir(), "constancy # ' space.xlsx")
				request := summaryDraftWorkbookExport(t, review, destination)
				outcome, err := service.ExportReviewed(t.Context(), state.ContextID, request)
				if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
					t.Fatal("owned Constancy publication failed", outcome, err)
				}
				data, err := os.ReadFile(destination)
				if err != nil || len(data) != review.Bytes {
					t.Fatal("published reviewed bytes unavailable", err)
				}
				digest := sha256.Sum256(data)
				if hex.EncodeToString(digest[:]) != review.WorkbookSHA256 {
					t.Fatal("published Constancy hash differs")
				}
				book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: data})
				caption := "Vegetation Table - Constancy Format"
				if combined {
					caption = "Site Unit Summary Table"
					actual, err := book.GetCellValue("ReportSummary", "B7")
					if err != nil || actual != "Lifeform" {
						t.Fatal("Constancy ancillary grouping caption drifted", actual, err)
					}
				}
				actual, err := book.GetCellValue(review.Sheet, "A2")
				if err != nil || actual != caption {
					t.Fatal("owned publication caption precedence drifted", actual, err)
				}
				for i, row := range review.Preview.Rows {
					if row.Species.Text == nil || *row.Species.Text != "Lifeform 2" {
						continue
					}
					want := "0.75 - 1.56"
					if average == 2 {
						want = "0.75 - 2.08"
					}
					for _, cell := range []struct{ column, value string }{{"A", "2"}, {"B", "Lifeform 2"}, {"C", "0.75"}, {"D", want}, {"E", ""}, {"F", ""}} {
						address := fmt.Sprintf("%s%d", cell.column, review.FirstVegetationRow+i)
						value, err := book.GetCellValue(review.Sheet, address, excelize.Options{RawCellValue: true})
						if err != nil || value != cell.value {
							t.Fatal("independent reopened Constancy body differs", address, value, cell.value, err)
						}
					}
				}
				collision, err := service.ExportReviewed(t.Context(), state.ContextID, request)
				after, readErr := os.ReadFile(destination)
				if err != nil || collision == nil || collision.Status != "not-published" ||
					collision.ErrorMessage == "" || readErr != nil || !bytes.Equal(after, data) {
					t.Fatal("Constancy collision overwrote output or hid refusal", collision, err, readErr)
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(after, config) {
			t.Fatal("Constancy publication persisted preferences", err)
		}
	}
}

func TestSummaryConstancyWorkbookSourceAuthorityAndLegacyReaderPolicies(t *testing.T) {
	service, state := summaryConstancyDesktopFixture(t, false)
	options := summaryConstancyWorkbookOptions()
	ordinary := options
	ordinary.Constancy = false
	if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, ordinary)); err == nil || review != nil {
		t.Fatal("Constancy implicitly widened the legacy signed16 reference reader", review, err)
	}
	source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
	for _, mode := range []string{"marker", "raw-source", "body-source", "raw-identity", "average"} {
		t.Run(mode, func(t *testing.T) {
			candidate, requested := source, options
			switch mode {
			case "marker":
				requested.Constancy = false
			case "raw-source":
				candidate.Raw.Preview.Source = "literal-normal-su-effective-references"
			case "body-source":
				candidate.Input.Vegetation.Source = "literal-normal-su-effective-references"
			case "raw-identity":
				candidate.Input.Vegetation.Rows = append([]SummaryVegetationRow{}, source.Input.Vegetation.Rows...)
				raw := metadataText("Changed")
				candidate.Input.Vegetation.Rows[0].RawSpecies = &raw
			case "average":
				requested.Average = 2
			}
			result, err := prepareSummaryWorkbookSource(t.Context(), candidate, requested)
			if err == nil || len(result.Bytes) != 0 {
				t.Fatal("mismatched Constancy source/options returned bytes", mode, err)
			}
		})
	}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil || !reflect.DeepEqual(review.Preview.GroupField, &SummaryVegetationField{"Lifeform", 4, 4}) {
		t.Fatal("explicit Constancy could not retry after legacy refusal", err)
	}
	destination := filepath.Join(t.TempDir(), "stale-scope.xlsx")
	request := summaryVegetationWorkbookExportRequest{Options: ordinary, ApprovalHash: review.ApprovalHash, Destination: destination}
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err == nil || outcome != nil || !strings.Contains(err.Error(), "lifeform") && !strings.Contains(err.Error(), "Lifeform") {
		t.Fatal("Constancy approval authorized a different legacy reader", outcome, err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("refused cross-scope publication left output", err)
	}
}
