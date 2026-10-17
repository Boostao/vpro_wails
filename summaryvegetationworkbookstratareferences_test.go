package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryStrataReferenceWorkbookFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryReferenceWorkbookFixture(t, external)
	service.groupedEnabled = true
	qualifySummaryGroupedDesktopFixture(t, service, state)
	return service, state
}

type summaryStrataReferenceWorkbookCase struct {
	name                   string
	average, display, show int
	env                    bool
	references             SummaryVegetationReferenceOptions
}

func summaryStrataReferenceWorkbookCases() []summaryStrataReferenceWorkbookCase {
	return []summaryStrataReferenceWorkbookCase{
		{"attributes", 1, 6, 1, false, SummaryVegetationReferenceOptions{}},
		{"common", 1, 6, 1, false, SummaryVegetationReferenceOptions{ShowCommon: true}},
		{"guide", 1, 6, 1, false, SummaryVegetationReferenceOptions{UseGuideNames: true}},
		{"guideCommon", 2, 6, 1, false, SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}},
		{"environment", 1, 6, 1, true, SummaryVegetationReferenceOptions{ShowCommon: true}},
		{"commonClass", 2, 4, 20, true, SummaryVegetationReferenceOptions{ShowCommon: true}},
		{"guideClass", 1, 5, 10, false, SummaryVegetationReferenceOptions{UseGuideNames: true}},
		{"distribution", 2, 3, 1, true, SummaryVegetationReferenceOptions{ShowCommon: true}},
	}
}

func TestSummaryStrataReferenceWorkbookOwnedTypedOutputAndZeroWrites(t *testing.T) {
	count := 0
	for _, external := range []bool{false, true} {
		service, state := summaryStrataReferenceWorkbookFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range summaryStrataReferenceWorkbookCases() {
			group := 12
			options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
				ShowClass: item.show, IncludeEnv: item.env, GroupAndOrder: &group, References: &item.references}
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || review == nil || review.Scope != summaryVegetationStrataReferencesScope ||
				!reflect.DeepEqual(review.Options, options) || review.Preview.GroupAndOrder != 12 ||
				review.Preview.Source != "literal-normal-su-effective-references" ||
				review.Preview.References == nil || (review.Environment != nil) != item.env {
				t.Fatal("owned Strata effective-reference review differs", external, item.name, review, err)
			}
			common := item.references.ShowCommon || item.references.UseGuideNames
			if review.Preview.References.ShowCommon != common || review.Preview.References.UseGuideNames != item.references.UseGuideNames {
				t.Fatal("effective reference request/metadata differ", item.name)
			}
			source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
			material, err := prepareSummaryWorkbookSource(t.Context(), source, options)
			if err != nil {
				t.Fatal(err)
			}
			book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: material.Bytes})
			expectedSource, err := json.Marshal(struct {
				Source  summaryVegetationWorkbookSource
				Options SummaryVegetationWorkbookOptions
				Scope   string
			}{source, options, summaryVegetationStrataReferencesScope})
			if err != nil || !bytes.Equal(expectedSource, summaryEnvironmentalWorkbookSource(t, book)) {
				t.Fatal("complete Strata source/options/scope hidden provenance differs", item.name, err)
			}
			if review.FirstVegetationRow != material.FirstVegetationRow || review.Sheet != material.Sheet ||
				len(material.Bytes) != review.Bytes {
				t.Fatal("reviewed output frame differs")
			}
			found := 0
			for i, row := range review.Preview.Rows {
				cells := []ProjectMetadataCell{row.Layer, row.Species}
				if common {
					if row.CommonName == nil {
						t.Fatal("common-name group absent")
					}
					cells = append(cells, *row.CommonName)
				} else if row.CommonName != nil {
					t.Fatal("common-name group invented")
				}
				cells = append(cells, row.Order)
				cells = append(cells, row.Covers...)
				for j, cell := range cells {
					address, err := excelize.CoordinatesToCellName(j+1, review.FirstVegetationRow+i)
					if err != nil {
						t.Fatal(err)
					}
					value, valueErr := book.GetCellValue(review.Sheet, address)
					formula, formulaErr := book.GetCellFormula(review.Sheet, address)
					want := ""
					if cell.Text != nil {
						want = *cell.Text
					} else if cell.Real != nil {
						want = fmt.Sprint(*cell.Real)
					}
					if valueErr != nil || formulaErr != nil || formula != "" || value != want {
						t.Fatal("independently reopened literal body cell differs", item.name, address, value, cell, valueErr, formulaErr)
					}
					if cell.Storage == "real" {
						kind, err := book.GetCellType(review.Sheet, address)
						if err != nil || kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
							t.Fatal("shifted DOUBLE Order became text", address, kind, err)
						}
					}
				}
				if item.average == 1 && item.display == 6 && row.Species.Text != nil && *row.Species.Text == "Lifeform 1" {
					found++
					want := "1.56"
					if !common {
						want = "3.13"
					}
					if row.Layer.Text == nil || *row.Layer.Text != "A" || row.Covers[0].Text == nil ||
						*row.Covers[0].Text != want || row.Order.Real == nil || fmt.Sprint(*row.Order.Real) != want {
						t.Fatal("independent Strata reference join weights differ", item.name, row, want)
					}
				}
			}
			if item.average == 1 && item.display == 6 {
				want := 1
				if common {
					want = 2
				}
				if found != want {
					t.Fatal("required independent physical reference groups absent", item.name, found, want)
				}
			}
			count++
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("Strata reference review wrote preferences", err)
		}
	}
	if count != 16 {
		t.Fatal("managed/external composition coverage differs", count)
	}
}

func TestSummaryStrataReferenceWorkbookIndependentGatesAndUnsupportedModes(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	group := 12
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, GroupAndOrder: &group,
		References: &SummaryVegetationReferenceOptions{ShowCommon: true}}
	for _, grouped := range []bool{false, true} {
		for _, references := range []bool{false, true} {
			service.groupedEnabled, service.referencesEnabled = grouped, references
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if grouped && references {
				if err != nil || review == nil {
					t.Fatal("joint existing authority did not enable Strata inspection", err)
				}
			} else if err == nil || review != nil || !strings.Contains(err.Error(), "disabled") {
				t.Fatal("independent authority bypassed", grouped, references, review, err)
			}
		}
	}
	service.ancillaryEnabled = true
	for _, mode := range []string{"lifeform", "none", "memberships", "summary", "display2"} {
		changed := options
		switch mode {
		case "lifeform", "none":
			other := 13
			if mode == "none" {
				other = 14
			}
			changed.GroupAndOrder = &other
		case "memberships":
			changed.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{}
		case "summary":
			changed.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}
		case "display2":
			changed.Display = 2
		}
		if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, changed)); err == nil || review != nil {
			t.Fatal("unqualified combination returned success", mode, review, err)
		}
	}
}

func TestSummaryStrataReferenceWorkbookPublicationAuthorityAndRecovery(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, true)
	group := 12
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, GroupAndOrder: &group,
		References: &SummaryVegetationReferenceOptions{ShowCommon: true}}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "Strata # ' literal.xlsx")
	request := summaryVegetationWorkbookExportRequest{Options: options, ApprovalHash: review.ApprovalHash, Destination: destination}
	for _, mode := range []string{"layer", "plain", "common", "guide", "group-gate", "reference-gate", "cancel", "owner", "unrendered-peer"} {
		changed := request
		ctx, contextID := t.Context(), state.ContextID
		owner := service.source.environment.report.contexts.projects.sqlite
		switch mode {
		case "layer":
			changed.Options.GroupAndOrder = nil
		case "plain":
			changed.Options.References = nil
		case "common":
			changed.Options.References = &SummaryVegetationReferenceOptions{}
		case "guide":
			changed.Options.References = &SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}
		case "group-gate":
			service.groupedEnabled = false
		case "reference-gate":
			service.referencesEnabled = false
		case "cancel":
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			ctx = cancelled
		case "owner":
			contextID = "foreign"
		case "unrendered-peer":
			mutateContextFixture(t, owner.attachments["VLists"],
				`UPDATE USysSppAttributeDescription SET Definition='changed unrendered' WHERE ShortName IS NULL`)
		}
		result, err := service.ExportReviewed(ctx, contextID, workbookRequest(t, changed))
		service.groupedEnabled, service.referencesEnabled = true, true
		if mode == "unrendered-peer" {
			mutateContextFixture(t, owner.attachments["VLists"],
				`UPDATE USysSppAttributeDescription SET Definition='Not admitted' WHERE ShortName IS NULL`)
		}
		if err == nil || result != nil {
			t.Fatal("changed composite authority returned an outcome", mode, result, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refused composite authority created output", mode, err)
		}
	}
	result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
		t.Fatal("exact source recovery did not publish reviewed composition", result, err)
	}
	before, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "not-published" || result.ErrorMessage == "" {
		t.Fatal("publication collision was not an explicit retained refusal", result, err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("collision changed completed composite publication", err)
	}
}

func TestSummaryStrataReferenceWorkbookClientWire(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range summaryStrataReferenceWorkbookCases() {
		group := 12
		options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
			ShowClass: item.show, IncludeEnv: item.env, GroupAndOrder: &group, References: &item.references}
		wire[item.name] = portableSummaryWorkbookReview(t, service, state, options)
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookStrataReferencesWire.json",
		"VPRO_UPDATE_SUMMARY_STRATA_REFERENCE_WORKBOOK_WIRE", wire)
}
