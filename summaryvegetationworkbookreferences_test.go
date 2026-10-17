package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryReferenceWorkbookFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryAncillaryDesktopFixture(t, external)
	service.referencesEnabled = true
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		UPDATE USysAllSpecs SET CombinedEnglishName='Guide:'||EnglishName;
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype,IsAttribute,CombinedEnglishName)
			VALUES('LF1','Removed',1,'Old attribute','U',-1,'Old guide');
		DROP TABLE IF EXISTS USysSppAttributeDescription;
		CREATE TABLE USysSppAttributeDescription(ShortName TEXT, Definition TEXT, IsAttribute INTEGER, Code TEXT);
		INSERT INTO USysSppAttributeDescription VALUES
			('LF1','=Definition_x0041_',0,'Ignored'),('LF1','=Definition_x0041_',-1,NULL),
			(NULL,'Not admitted',-1,'No fallback'),('ATTR',NULL,0,NULL);`)
	mutateContextFixture(t, owner.attachments["VUser"], `
		INSERT INTO USysUserSpp(Code,ScientificName,LifeForm,EnglishName,Codetype)
			VALUES('LF1','Lifeform 1',1,'Personal','U');`)
	return service, state
}

func TestSummaryEffectiveReferenceWorkbookStrictRequestsAndIndependentGate(t *testing.T) {
	base := `{"title":"literal","average":1,"display":6,"showClass":0,"includeEnv":false`
	for _, suffix := range []string{
		`,"references":null}`, `,"references":{}}`, `,"references":{"showCommon":false}}`,
		`,"references":{"showCommon":false,"useGuideNames":null}}`,
		`,"references":{"showCommon":"false","useGuideNames":false}}`,
		`,"references":{"ShowCommon":false,"useGuideNames":false}}`,
		`,"references":{"showCommon":false,"useGuideNames":false,"extra":0}}`,
		`,"references":{"showCommon":true,"\u0073howCommon":false,"useGuideNames":false}}`,
		`,"references":{"showCommon":false,"useGuideNames":false},"\u0072eferences":{}}`,
		`,"references":{"showCommon":false,"useGuideNames":false}} {}`,
	} {
		if err := json.Unmarshal([]byte(base+suffix), new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("ambiguous effective reference authority accepted", suffix)
		}
	}
	service, state := summaryReferenceWorkbookFixture(t, false)
	service.referencesEnabled = false
	for _, selection := range []*SummaryVegetationReferenceOptions{
		nil, {}, {ShowCommon: true}, {UseGuideNames: true},
	} {
		options := SummaryVegetationWorkbookOptions{Title: "literal", Average: 1, Display: 6, References: selection}
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if selection == nil {
			if err != nil || review == nil || review.Preview.References != nil || review.Scope != summaryVegetationWorkbookDraftScope {
				t.Fatal("independent reference gate changed legacy availability", review, err)
			}
		} else if err == nil || review != nil || !strings.Contains(err.Error(), "effective references are disabled") {
			t.Fatal("backend gate bypassed", review, err)
		}
	}
}

func TestSummaryEffectiveReferenceWorkbookOwnedModesAndLiteralColumns(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryReferenceWorkbookFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, selection := range []SummaryVegetationReferenceOptions{
			{}, {ShowCommon: true}, {UseGuideNames: true}, {ShowCommon: true, UseGuideNames: true},
		} {
			for _, includeEnv := range []bool{false, true} {
				for _, ancillary := range []*SummaryVegetationWorkbookAncillaryOptions{nil, {}, {CreateSummary: true}} {
					options := SummaryVegetationWorkbookOptions{Title: "=literal", Average: 1, Display: 6,
						IncludeEnv: includeEnv, References: &selection, Ancillary: ancillary}
					review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
					if err != nil || review == nil {
						t.Fatal("owned effective reference review failed", options, err)
					}
					common := selection.ShowCommon || selection.UseGuideNames
					wantRows := 14
					if common {
						wantRows = 15
					}
					wantMetadata := SummaryVegetationReferenceReview{
						MasterRows: 14, PersonalRows: 1, DescriptionRows: 4, RemovedMasterRows: 1,
						DerivedRows: 3, UnionRows: 16, ShowCommon: common, UseGuideNames: selection.UseGuideNames,
					}
					if review.Scope != summaryWorkbookScope(options) || len(review.Preview.Rows) != wantRows ||
						!reflect.DeepEqual(review.Preview.References, &wantMetadata) {
						t.Fatal("independent projected row/count expectations differ", review.Preview.References, len(review.Preview.Rows), wantRows)
					}
					var material summaryVegetationWorkbook
					_, err = withContextPlotRequest(t.Context(), contexts, state.ContextID, func(plots *PlotService) (bool, error) {
						return withOwnedPreferenceSnapshot(t.Context(), plots, publicationReadSnapshotHooks{},
							func(owner *sqliteContext, tx *sql.Tx) (bool, error) {
								created := ""
								if ancillary != nil {
									created = summaryAncillaryReviewedTime
								}
								source, err := service.readSourceAt(t.Context(), plots, owner, tx, state.ContextID, options, created)
								if err != nil {
									return false, err
								}
								if source.Raw.Descriptions == nil || len(source.Raw.Master.Rows) != 14 {
									t.Fatal("complete physical metadata/master sources were replaced by projected rows")
								}
								material, err = prepareSummaryWorkbookSource(t.Context(), source, options)
								return err == nil, err
							})
					})
					if err != nil {
						t.Fatal(err)
					}
					book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: material.Bytes})
					for i, row := range review.Preview.Rows {
						if row.Species.Text != nil && *row.Species.Text == "Removed" {
							t.Fatal("old attribute survived effective deletion")
						}
						if common && row.CommonName == nil || !common && row.CommonName != nil {
							t.Fatal("common-name layout differs from reviewed selection")
						}
						column := 4
						if common {
							column = 5
							cell, _ := excelize.CoordinatesToCellName(3, review.FirstVegetationRow+i)
							value, err := book.GetCellValue(review.Sheet, cell)
							formula, formulaErr := book.GetCellFormula(review.Sheet, cell)
							want := ""
							if row.CommonName.Text != nil {
								want = *row.CommonName.Text
							}
							if err != nil || formulaErr != nil || formula != "" || value != want {
								t.Fatal("literal common-name column changed value or became a formula", value, want, err, formulaErr)
							}
							if *row.Species.Text == "LF1" && selection.UseGuideNames && row.CommonName.Storage != "null" {
								t.Fatal("guide attribute used Definition instead of default NULL CombinedEnglishName")
							}
						}
						if row.Species.Text != nil && *row.Species.Text == "Lifeform 1" {
							cell, _ := excelize.CoordinatesToCellName(column, review.FirstVegetationRow+i)
							value, err := book.GetCellValue(review.Sheet, cell)
							want := "3.13"
							if common {
								want = "1.56"
							}
							if err != nil || value != want {
								t.Fatal("common-name grouping did not change join weights before aggregation", value, want, err)
							}
						}
					}
					if review.Environment != nil {
						for _, row := range review.Environment.Rows {
							if row.Field.Cells[5].Text != nil && *row.Field.Cells[5].Text == "SVLifeformCover" &&
								row.Field.Cells[6].Text != nil && *row.Field.Cells[6].Text == "1" &&
								(row.Values[0].Cell == nil || row.Values[0].Cell.Text == nil || *row.Values[0].Cell.Text != "3.8 - 4.4 - 5.0") {
								t.Fatal("environment borrowed personal/derived/deleted attribute references", row.Values)
							}
						}
					}
				}
			}
		}
		actual, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, actual) || !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
			t.Fatal("effective reference inspection changed source data, settings or audits", err)
		}
	}
}

func TestSummaryEffectiveReferenceWorkbookMasterLongPolicyAndPhysicalDrift(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, false)
	contexts := service.source.environment.report.contexts
	owner := contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VLists"], `
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype,IsAttribute)
			VALUES('WIDE',NULL,40000,NULL,'U',0);`)
	options := SummaryVegetationWorkbookOptions{Title: "literal", Average: 1, Display: 6, IncludeEnv: true,
		References: &SummaryVegetationReferenceOptions{ShowCommon: true}}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil || review == nil || review.Preview.References.MasterRows != 15 {
		t.Fatal("owned body/header implicitly narrowed master source LONG", review, err)
	}
	if _, err := service.source.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0); err == nil {
		t.Fatal("ordinary accepted environmental consumer was implicitly widened")
	}
	destination := filepath.Join(t.TempDir(), "effective.xlsx")
	export := summaryDraftWorkbookExport(t, review, destination)
	mutateContextFixture(t, owner.attachments["VLists"], `UPDATE USysSppAttributeDescription SET Code='Changed unused peer' WHERE ShortName IS NULL`)
	outcome, err := service.ExportReviewed(t.Context(), state.ContextID, export)
	if err == nil || outcome != nil {
		t.Fatal("unrendered metadata peer drift published under stale authority", outcome, err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("stale source left publication output", err)
	}
}

func TestSummaryEffectiveReferenceWorkbookDisplayAverageMatrix(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, false)
	for _, selection := range []SummaryVegetationReferenceOptions{
		{}, {ShowCommon: true}, {UseGuideNames: true}, {ShowCommon: true, UseGuideNames: true},
	} {
		for _, average := range []int{1, 2} {
			for display := 1; display <= 6; display++ {
				classes := []int{0}
				if display == 4 || display == 5 {
					classes = []int{1, 10, 20}
				}
				for _, class := range classes {
					options := SummaryVegetationWorkbookOptions{Average: average, Display: display,
						ShowClass: class, References: &selection}
					review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
					if err != nil || review == nil {
						t.Fatal("effective reference calculation/layout combination refused", options, err)
					}
					common := selection.ShowCommon || selection.UseGuideNames
					ordinary := 0
					for _, row := range review.Preview.Rows {
						if row.Species.Text != nil && *row.Species.Text == "Lifeform 1" {
							ordinary++
							if display == 6 {
								want := "2.08"
								if average == 1 {
									want = "3.13"
									if common {
										want = "1.56"
									}
								}
								if row.Covers[0].Text == nil || *row.Covers[0].Text != want {
									t.Fatal("independent common/average join-weight expectation differs", options, row.Covers, want)
								}
							}
						}
					}
					want := 1
					if common {
						want = 2
					} else if display == 2 {
						// The literal duplicate join gives presence6/4; source presence classes omit values above1.
						want = 0
					}
					if ordinary != want {
						t.Fatal("display/class selection changed common-name group membership", options, ordinary, want)
					}
				}
			}
		}
	}
}

func TestSummaryEffectiveReferenceWorkbookPublicationAuthorityAndRecovery(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, true)
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true,
		References: &SummaryVegetationReferenceOptions{ShowCommon: true}}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "effective # ' workbook.xlsx")
	request := summaryVegetationWorkbookExportRequest{Options: options, ApprovalHash: review.ApprovalHash, Destination: destination}
	for _, mutation := range []string{"legacy", "common", "guide", "gate", "cancel"} {
		changed := request
		ctx := t.Context()
		switch mutation {
		case "legacy":
			changed.Options.References = nil
		case "common":
			changed.Options.References = &SummaryVegetationReferenceOptions{}
		case "guide":
			changed.Options.References = &SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}
		case "gate":
			service.referencesEnabled = false
		case "cancel":
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			ctx = cancelled
		}
		result, err := service.ExportReviewed(ctx, state.ContextID, workbookRequest(t, changed))
		service.referencesEnabled = true
		if err == nil || result != nil {
			t.Fatal("changed effective authority published or returned an outcome", mutation, result, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refused effective authority left a destination", mutation, err)
		}
	}
	result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
		t.Fatal("effective authority did not recover after rejected attempts", result, err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	})
	name, err := book.GetCellValue(review.Sheet, "C36")
	formula, formulaErr := book.GetCellFormula(review.Sheet, "C36")
	if err != nil || formulaErr != nil || name != "=Definition_x0041_" || formula != "" {
		t.Fatal("independently reopened effective common name differs", name, formula, err, formulaErr)
	}
	result, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "not-published" || result.ErrorMessage == "" {
		t.Fatal("effective publication replaced an existing destination", result, err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("effective collision changed the prior publication", err)
	}
}

func TestSummaryEffectiveReferenceWorkbookLatePhysicalPeerDrift(t *testing.T) {
	for _, phase := range []string{"staged", "published"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryReferenceWorkbookFixture(t, false)
			options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6,
				References: &SummaryVegetationReferenceOptions{UseGuideNames: true},
				Ancillary:  &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}}
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil {
				t.Fatal(err)
			}
			owner := service.source.environment.report.contexts.projects.sqlite
			destination := filepath.Join(t.TempDir(), "guide.xlsx")
			result, err := service.exportReviewed(t.Context(), state.ContextID, summaryDraftWorkbookExport(t, review, destination),
				summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
					if actual == phase {
						mutateContextFixture(t, owner.attachments["VLists"],
							`UPDATE USysAllSpecs SET EnglishName='Unrendered original name changed' WHERE Code='LF2'`)
					}
					return nil
				}}})
			expected := "not-published"
			if phase == "published" {
				expected = "published-with-errors"
			}
			if err != nil || result == nil || result.Status != expected || result.ErrorMessage == "" {
				t.Fatal("unrendered physical peer drift lost publication state", result, err)
			}
			if phase == "published" && (result.SHA256 != review.WorkbookSHA256 || !strings.Contains(result.ErrorMessage, "do not replay")) {
				t.Fatal("committed effective-reference receipt became replayable", result)
			}
			if _, err := os.Stat(destination); phase != "published" && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("staged reference drift left a destination", err)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || again.ApprovalHash == review.ApprovalHash || again.WorkbookSHA256 == review.WorkbookSHA256 {
				t.Fatal("guide projection erased unrendered physical source provenance", err)
			}
		})
	}
}
