package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryCommonFirstWorkbookOptions(grouping, average int) SummaryVegetationWorkbookOptions {
	return SummaryVegetationWorkbookOptions{Title: "=Unanimous", Average: average, Display: 6, ShowClass: 20,
		GroupAndOrder: &grouping, References: &SummaryVegetationReferenceOptions{ShowCommon: true}, UnanimousCommon: true}
}

func summaryCommonFirstDesktopFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryStrataReferenceWorkbookFixture(t, external)
	service.ancillaryEnabled = true
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype,IsAttribute,CombinedEnglishName)
		VALUES('FIRSTX','Alpha',1,'Common','U',0,'Guide common'),
		('FIRSTY','Alpha',1,'Common','U',0,'Guide common'),
		('FIRSTE','Empty name',1,'','U',0,''),
		('FIRSTN','Null name',1,NULL,'U',0,NULL);`)
	mutateContextFixture(t, owner.attachments["su"], `DELETE FROM `+quoteHeaderIdentifier(state.ActiveSU+"_SU")+`;
		INSERT INTO `+quoteHeaderIdentifier(state.ActiveSU+"_SU")+`(PlotNumber,SiteUnit)
		VALUES('P1','A'),('P1','A'),('P2','A'),('P5','A'),('P3','B'),(NULL,'A'),('P9',NULL),('P6','Orphan');`)
	mutateContextFixture(t, owner.attachments["project"], `DELETE FROM `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+`;
		INSERT INTO `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+`(PlotNumber,Species,Cover1)
		VALUES('P1','FIRSTX',14),('P2','FIRSTX',28),('P2','FIRSTY',4),('P3','FIRSTX',5),
		('P3','FIRSTY',6),('P2','FIRSTX',1),('P1','FIRSTE',3),('P1','FIRSTN',2);`)
	return service, state
}

func TestSummaryCommonFirstWorkbookStrictRequests(t *testing.T) {
	const base = `{"title":"=Unanimous","average":1,"display":6,"showClass":20,"includeEnv":false,"references":{"showCommon":true,"useGuideNames":false},"groupAndOrder":13`
	for _, suffix := range []string{
		`,"unanimousCommon":false}`, `,"unanimousCommon":null}`, `,"unanimousCommon":1}`, `,"unanimousCommon":"true"}`,
		`,"UnanimousCommon":true}`, `,"unanimousCommon":true,"unanimousCommon":true}`,
		`,"unanimousCommon":true,"\u0075nanimousCommon":true}`, `,"unanimousCommon":true,"constancy":true}`,
		`,"unanimousCommon":true} {}`,
	} {
		if err := json.Unmarshal([]byte(base+suffix), new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("ambiguous unanimous request accepted", suffix)
		}
	}
	valid := base + `,"unanimousCommon":true}`
	for _, mutation := range [][2]string{
		{`"groupAndOrder":13`, `"groupAndOrder":12`}, {`,"groupAndOrder":13`, ""},
		{`"display":6`, `"display":1`}, {`"showCommon":true`, `"showCommon":false`},
		{`,"references":{"showCommon":true,"useGuideNames":false}`, ""},
	} {
		if err := json.Unmarshal([]byte(strings.Replace(valid, mutation[0], mutation[1], 1)),
			new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("incompatible unanimous request accepted", mutation)
		}
	}
	for _, request := range []string{base + "}", valid, strings.Replace(valid, `"groupAndOrder":13`, `"groupAndOrder":14`, 1),
		strings.Replace(strings.Replace(valid, `"showCommon":true`, `"showCommon":false`, 1), `"useGuideNames":false`, `"useGuideNames":true`, 1)} {
		var options SummaryVegetationWorkbookOptions
		if err := json.Unmarshal([]byte(request), &options); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(options)
		if err != nil || string(data) != request {
			t.Fatal("explicit marker or legacy omission changed", string(data), request, err)
		}
	}
}

func TestSummaryCommonFirstWorkbookOwnedOutputAndClientWire(t *testing.T) {
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, external := range []bool{false, true} {
		service, state := summaryCommonFirstDesktopFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, grouping := range []int{13, 14} {
			for _, average := range []int{1, 2} {
				for _, mode := range []string{"common", "guide", "environment", "combined"} {
					options := summaryCommonFirstWorkbookOptions(grouping, average)
					options.IncludeEnv = mode == "environment" || mode == "combined"
					if mode == "guide" {
						options.References.UseGuideNames = true
					}
					if mode == "combined" {
						options.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}
					}
					review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
					if err != nil {
						t.Fatal(grouping, average, mode, err)
					}
					if review.Scope != summaryVegetationUnanimousCommonScope ||
						review.Preview.Source != summaryVegetationUnanimousCommonSource ||
						review.Preview.Ordering != summaryVegetationUnanimousCommonOrdering ||
						review.Options.ShowClass != 20 || review.Preview.ShowClass != 0 || len(review.Preview.Rows) != 3 {
						t.Fatal("unanimous authority/frame drift", workbookRequest(t, review))
					}
					wantGroup, wantCommon, wantB := "01", "Common", "11.00"
					if grouping == 14 {
						wantGroup = ""
					}
					if mode == "guide" {
						wantCommon = "Guide common"
					}
					if average == 2 {
						wantB = "5.50"
					}
					alpha := review.Preview.Rows[0]
					if alpha.Species.Text == nil || *alpha.Species.Text != "Alpha" ||
						alpha.CommonName == nil || alpha.CommonName.Text == nil || *alpha.CommonName.Text != wantCommon ||
						alpha.Order.Real == nil || *alpha.Order.Real != 15 || len(alpha.Covers) != 2 ||
						alpha.Covers[0].Text == nil || *alpha.Covers[0].Text != "15.00" ||
						alpha.Covers[1].Text == nil || *alpha.Covers[1].Text != wantB {
						t.Fatal("independent original arithmetic/name differs", workbookRequest(t, alpha))
					}
					for _, row := range review.Preview.Rows[1:] {
						if row.CommonName == nil || row.CommonName.Storage != "null" {
							t.Fatal("source First NULL/empty output distinction misprojected", workbookRequest(t, row))
						}
					}
					destination := filepath.Join(t.TempDir(), "unanimous # ' space.xlsx")
					request := summaryDraftWorkbookExport(t, review, destination)
					outcome, err := service.ExportReviewed(t.Context(), state.ContextID, request)
					if err != nil || outcome == nil || outcome.Status != "published" || outcome.SHA256 != review.WorkbookSHA256 {
						t.Fatal("unanimous publication failed", outcome, err)
					}
					data, err := os.ReadFile(destination)
					if err != nil || len(data) != review.Bytes {
						t.Fatal("reviewed bytes unavailable", err)
					}
					book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: data})
					for column, want := range []string{wantGroup, "Alpha", wantCommon, "15", "15.00", wantB} {
						address, err := excelize.CoordinatesToCellName(column+1, review.FirstVegetationRow)
						if err != nil {
							t.Fatal(err)
						}
						actual, err := book.GetCellValue(review.Sheet, address, excelize.Options{RawCellValue: true})
						formula, formulaErr := book.GetCellFormula(review.Sheet, address)
						if err != nil || formulaErr != nil || actual != want || formula != "" {
							t.Fatal("independent reopened typed body differs", address, actual, want, err, formulaErr)
						}
						if column == 3 {
							kind, err := book.GetCellType(review.Sheet, address)
							if err != nil || kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
								t.Fatal("DOUBLE Order became text", kind, err)
							}
						}
					}
					collision, err := service.ExportReviewed(t.Context(), state.ContextID, request)
					after, readErr := os.ReadFile(destination)
					if err != nil || collision == nil || collision.Status != "not-published" ||
						collision.ErrorMessage == "" || readErr != nil || !bytes.Equal(data, after) {
						t.Fatal("collision hid refusal or overwrote bytes", collision, err, readErr)
					}
					if !external {
						wire[fmt.Sprintf("group%dAverage%d%s", grouping, average, mode)] =
							portableSummaryWorkbookReview(t, service, state, options)
					}
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("unanimous inspection persisted preferences", err)
		}
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookUnanimousCommonWire.json",
		"VPRO_UPDATE_SUMMARY_UNANIMOUS_COMMON_WORKBOOK_WIRE", wire)
}

func TestSummaryCommonFirstWorkbookAuthorityAmbiguityAndRetry(t *testing.T) {
	service, state := summaryCommonFirstDesktopFixture(t, false)
	options := summaryCommonFirstWorkbookOptions(13, 1)
	source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
	for _, mode := range []string{"marker", "raw-source", "body-source", "average", "title", "environment", "showClass", "guide"} {
		t.Run(mode, func(t *testing.T) {
			candidate, requested := source, options
			requested.References = &SummaryVegetationReferenceOptions{ShowCommon: true}
			switch mode {
			case "marker":
				requested.UnanimousCommon = false
			case "raw-source":
				candidate.Raw.Preview.Source = "literal-normal-su-effective-references"
			case "body-source":
				candidate.Input.Vegetation.Source = "literal-normal-su-effective-references"
			case "average":
				requested.Average = 2
			case "title":
				requested.Title = "Changed"
			case "environment":
				requested.IncludeEnv = true
			case "showClass":
				requested.ShowClass = 99
			case "guide":
				requested.References.UseGuideNames = true
			}
			result, err := prepareSummaryWorkbookSource(t.Context(), candidate, requested)
			if err == nil || len(result.Bytes) != 0 {
				t.Fatal("mismatched source/options returned bytes", mode, err)
			}
		})
	}
	for _, mode := range []string{"duplicate-key", "empty-name", "ordering", "metadata", "raw-species", "layer-mode-downgrade"} {
		t.Run(mode, func(t *testing.T) {
			var input summaryVegetationWorkbookInput
			if err := json.Unmarshal([]byte(workbookRequest(t, source.Input)), &input); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "duplicate-key":
				row := input.Vegetation.Rows[0]
				common := metadataText("Different")
				row.CommonName = &common
				input.Vegetation.Rows = append(input.Vegetation.Rows[:1], append([]SummaryVegetationRow{row}, input.Vegetation.Rows[1:]...)...)
			case "empty-name":
				empty := metadataText("")
				input.Vegetation.Rows[0].CommonName = &empty
			case "ordering":
				input.Vegetation.Ordering = "literal-null-first-layer-species-common"
			case "metadata":
				input.Vegetation.GroupField.DAOType = 4
			case "raw-species":
				raw := metadataText("FIRSTX")
				input.Vegetation.Rows[0].RawSpecies = &raw
			case "layer-mode-downgrade":
				input.Vegetation.GroupAndOrder = 11
				input.Vegetation.GroupField, input.Vegetation.OrderField = nil, nil
				for i := range input.Vegetation.Rows {
					input.Vegetation.Rows[i].Order = metadataText("15.00")
				}
			}
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err == nil || len(result.Bytes) != 0 {
				t.Fatal("invalid original key/output accepted", mode, err)
			}
		})
	}
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `UPDATE USysAllSpecs SET EnglishName='Different' WHERE Code='FIRSTY';`)
	before := databaseBytes(t, owner.attachments)
	for _, grouping := range []int{13, 14} {
		requested := summaryCommonFirstWorkbookOptions(grouping, 1)
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, requested))
		if err == nil || review != nil || !strings.Contains(err.Error(), "First(common name) is ambiguous") {
			t.Fatal("mixed source names silently chose/split a group", review, err)
		}
		requested.References.UseGuideNames = true
		review, err = service.GetReview(t.Context(), state.ContextID, workbookRequest(t, requested))
		if err != nil || review == nil || review.Preview.Rows[0].CommonName.Text == nil ||
			*review.Preview.Rows[0].CommonName.Text != "Guide common" {
			t.Fatal("valid guide correction could not retry", review, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if result, err := service.GetReview(ctx, state.ContextID, workbookRequest(t, requested)); err == nil || result != nil {
			t.Fatal("cancelled request succeeded", result, err)
		}
		requested.UnanimousCommon = false
		if result, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, requested)); err == nil || result != nil {
			t.Fatal("ordinary wrapper was implicitly widened", result, err)
		}
	}
	if !reflect.DeepEqual(before, databaseBytes(t, owner.attachments)) {
		t.Fatal("ambiguity/correction/cancellation mutated source")
	}
}

func TestSummaryCommonFirstWorkbookIndependentGates(t *testing.T) {
	service, state := summaryCommonFirstDesktopFixture(t, false)
	for mask := 0; mask < 8; mask++ {
		service.groupedEnabled, service.referencesEnabled, service.presentationEnabled = mask&1 != 0, mask&2 != 0, mask&4 != 0
		options := summaryCommonFirstWorkbookOptions(13, 1)
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if mask&3 == 3 {
			if err != nil || review == nil {
				t.Fatal("unanimous mode borrowed the Constancy presentation gate", mask, err)
			}
		} else if err == nil || review != nil {
			t.Fatal("independent grouped/reference gate bypassed", mask, err)
		}
		ordinary := SummaryVegetationWorkbookOptions{Title: "ordinary", Average: 1, Display: 6}
		if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, ordinary)); err != nil || review == nil {
			t.Fatal("unanimous gates disabled ordinary workbook", mask, err)
		}
	}
}

func TestSummaryCommonFirstWorkbookApprovalAndPhysicalDrift(t *testing.T) {
	for _, phase := range []string{"reviewed", "staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryCommonFirstDesktopFixture(t, true)
			options := summaryCommonFirstWorkbookOptions(14, 2)
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil {
				t.Fatal(err)
			}
			other := options
			other.UnanimousCommon, other.References = false, &SummaryVegetationReferenceOptions{}
			destination := filepath.Join(t.TempDir(), "foreign-approval.xlsx")
			request := summaryVegetationWorkbookExportRequest{Options: other, ApprovalHash: review.ApprovalHash, Destination: destination}
			outcome, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
			if err == nil || outcome != nil {
				t.Fatal("unanimous approval authorized ordinary attributes", outcome, err)
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("cross-scope refusal left output", err)
			}
			ordinary, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, other))
			if err != nil {
				t.Fatal(err)
			}
			request.Options, request.ApprovalHash = options, ordinary.ApprovalHash
			outcome, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
			if err == nil || outcome != nil {
				t.Fatal("ordinary approval authorized unanimous output", outcome, err)
			}
			owner := service.source.environment.report.contexts.projects.sqlite
			drift := func() {
				mutateContextFixture(t, owner.attachments["VLists"],
					`UPDATE USysAllSpecs SET EnglishName='Unused physical drift' WHERE Code='LF2';`)
			}
			if phase == "reviewed" {
				drift()
			}
			outcome, err = service.exportReviewed(t.Context(), state.ContextID,
				summaryDraftWorkbookExport(t, review, destination),
				summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
					if actual == phase {
						drift()
					}
					return nil
				}}})
			if phase == "reviewed" {
				if err == nil || outcome != nil || !strings.Contains(err.Error(), "reload review") {
					t.Fatal("unused physical reference failed to invalidate approval", outcome, err)
				}
			} else {
				want := "not-published"
				if phase == "published" {
					want = "published-with-errors"
				}
				if err != nil || outcome == nil || outcome.Status != want || outcome.ErrorMessage == "" {
					t.Fatal("physical drift lost publication state", outcome, err)
				}
			}
			data, readErr := os.ReadFile(destination)
			if phase == "published" {
				if readErr != nil || len(data) != review.Bytes || outcome.SHA256 != review.WorkbookSHA256 ||
					!strings.Contains(outcome.ErrorMessage, "do not replay") {
					t.Fatal("committed output became replayable or lost reviewed identity", outcome, readErr)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatal("refused publication left output", readErr)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || again.ApprovalHash == review.ApprovalHash ||
				!reflect.DeepEqual(again.Preview, review.Preview) {
				t.Fatal("physical provenance failed to distinguish identical visible rows", err)
			}
		})
	}
}
