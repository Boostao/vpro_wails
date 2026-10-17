package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

const summaryGroupedAncillaryTestScope = "Explicit read-only normal-SU Strata/Lifeform/None inspection with optional effective references, SU Table and optional ReportSummary; typed XLSX, no Lifeform/None common names, original Report callbacks or formatting/printing parity"

type summaryGroupedAncillaryCase struct {
	name, label                   string
	group, average, display, show int
	env, summary                  bool
	references                    *SummaryVegetationReferenceOptions
}

func summaryGroupedAncillaryCases() []summaryGroupedAncillaryCase {
	return []summaryGroupedAncillaryCase{
		{"strataSummary", "Strata", 12, 1, 6, 1, false, true, nil},
		{"strataCommonEnvironment", "Strata", 12, 2, 6, 1, true, true, &SummaryVegetationReferenceOptions{ShowCommon: true}},
		{"strataGuideMemberships", "Strata", 12, 1, 3, 1, false, false, &SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}},
		{"lifeformAttributeEnvironment", "Lifeform", 13, 1, 6, 1, true, true, &SummaryVegetationReferenceOptions{}},
		{"lifeformClassMemberships", "Lifeform", 13, 2, 5, 10, false, false, nil},
		{"noneFrequencyEnvironment", "None", 14, 2, 2, 1, true, true, nil},
		{"noneAttributeMemberships", "None", 14, 1, 6, 1, false, false, &SummaryVegetationReferenceOptions{}},
		{"noneAttributeDistribution", "None", 14, 2, 1, 1, true, true, &SummaryVegetationReferenceOptions{}},
	}
}

func summaryGroupedAncillaryOptions(item summaryGroupedAncillaryCase) SummaryVegetationWorkbookOptions {
	group := item.group
	return SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
		ShowClass: item.show, IncludeEnv: item.env, GroupAndOrder: &group, References: item.references,
		Ancillary: &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: item.summary}}
}

func TestSummaryGroupedAncillaryDesktopClientWire(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range summaryGroupedAncillaryCases() {
		wire[item.name] = portableSummaryWorkbookReview(t, service, state, summaryGroupedAncillaryOptions(item))
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookGroupedAncillaryWire.json",
		"VPRO_UPDATE_SUMMARY_GROUPED_ANCILLARY_WORKBOOK_WIRE", wire)
}

func TestSummaryGroupedAncillaryIndependentGatesAndRetainedAuthority(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	options := summaryGroupedAncillaryOptions(summaryGroupedAncillaryCases()[1])
	for _, grouped := range []bool{false, true} {
		for _, ancillary := range []bool{false, true} {
			for _, references := range []bool{false, true} {
				service.groupedEnabled, service.ancillaryEnabled, service.referencesEnabled = grouped, ancillary, references
				review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
				if grouped && ancillary && references {
					if err != nil || review == nil {
						t.Fatal("fully authorized composition refused", err)
					}
				} else if err == nil || review != nil {
					t.Fatal("independent grouped ancillary/reference gate bypassed", grouped, ancillary, references)
				}
			}
		}
	}
	guideOnly := options
	guideOnly.References = &SummaryVegetationReferenceOptions{UseGuideNames: true}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, guideOnly))
	if err != nil || review == nil || !reflect.DeepEqual(review.Options, guideOnly) ||
		review.Preview.References == nil || !review.Preview.References.ShowCommon || !review.Preview.References.UseGuideNames {
		t.Fatal("guide-only transport authority was rewritten or lost its effective common projection", err)
	}
	source := readSummaryAncillaryDesktopSource(t, service, state, options, "2026-10-17T02:00:00Z")
	input := summaryVegetationAncillaryWorkbookInput{
		Source: source, Options: options, Tables: source.Ancillary.Tables,
		CreatedAt: source.Ancillary.CreatedAt, CreateSummary: true,
	}
	input.Source.Ancillary = nil
	for _, item := range []struct {
		name   string
		mutate func(*summaryVegetationAncillaryWorkbookInput)
	}{
		{"implicitGroup", func(value *summaryVegetationAncillaryWorkbookInput) { value.Options.GroupAndOrder = nil }},
		{"wrongGroup", func(value *summaryVegetationAncillaryWorkbookInput) { *value.Options.GroupAndOrder = 13 }},
		{"missingReferences", func(value *summaryVegetationAncillaryWorkbookInput) { value.Options.References = nil }},
		{"changedCommon", func(value *summaryVegetationAncillaryWorkbookInput) { value.Options.References.ShowCommon = false }},
		{"changedGuide", func(value *summaryVegetationAncillaryWorkbookInput) { value.Options.References.UseGuideNames = true }},
		{"groupCaption", func(value *summaryVegetationAncillaryWorkbookInput) {
			value.Source.Input.Vegetation.GroupField.Name = "Strata"
		}},
		{"groupType", func(value *summaryVegetationAncillaryWorkbookInput) {
			value.Source.Input.Vegetation.GroupField.DAOType = 7
		}},
		{"orderType", func(value *summaryVegetationAncillaryWorkbookInput) {
			value.Source.Input.Vegetation.OrderField.DAOType = 10
		}},
		{"summarySelection", func(value *summaryVegetationAncillaryWorkbookInput) { value.Options.Ancillary.CreateSummary = false }},
	} {
		t.Run(item.name, func(t *testing.T) {
			changed := cloneSummaryAncillaryWorkbook(t, input)
			item.mutate(&changed)
			result, err := prepareSummaryVegetationAncillaryWorkbook(t.Context(), changed)
			if err == nil || len(result.Bytes) != 0 {
				t.Fatal("changed retained authority produced workbook bytes", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err := prepareSummaryVegetationAncillaryWorkbook(ctx, input)
	if !errors.Is(err, context.Canceled) || len(result.Bytes) != 0 {
		t.Fatal("cancelled composition produced workbook bytes", err)
	}
}

func TestSummaryGroupedAncillaryOwnedCompositionAndMetadata(t *testing.T) {
	count := 0
	for _, external := range []bool{false, true} {
		service, state := summaryStrataReferenceWorkbookFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}

		for _, item := range summaryGroupedAncillaryCases() {
			options := summaryGroupedAncillaryOptions(item)
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || review == nil || review.Scope != summaryGroupedAncillaryTestScope ||
				!reflect.DeepEqual(review.Options, options) || review.Preview.GroupAndOrder != item.group ||
				review.Ancillary == nil || (review.Ancillary.Summary != nil) != item.summary {
				t.Fatal("source-qualified grouped ancillary composition unavailable or altered", external, item.name, review, err)
			}
			source := readSummaryAncillaryDesktopSource(t, service, state, options, review.Ancillary.CreatedAt)
			material, err := prepareSummaryWorkbookSource(t.Context(), source, options)
			if err != nil {
				t.Fatal(err)
			}
			book := assertSummaryGroupedWorkbookBody(t, source.Input, material)
			expectedSheets := []string{review.Sheet, "SU Table"}
			if item.summary {
				expectedSheets = append(expectedSheets, "ReportSummary")
			}
			if !reflect.DeepEqual(review.Ancillary.Sheets, expectedSheets) ||
				!reflect.DeepEqual(book.GetSheetList(), append(expectedSheets, "_VPRO_Source")) {
				t.Fatal("grouped ancillary sheet identity/order differs", item.name, review.Ancillary.Sheets, book.GetSheetList())
			}
			for _, sheet := range expectedSheets[1:] {
				for _, axis := range []string{"A1", "B1", "A2", "B2"} {
					if formula, err := book.GetCellFormula(sheet, axis); err != nil || formula != "" {
						t.Fatal("literal ancillary field became a formula", item.name, sheet, axis, err)
					}
				}
			}
			if item.summary {
				for axis, expected := range map[string]string{"B4": review.Preview.Project, "B5": review.Preview.SU + "_SU",
					"B6": "None_Lump", "B7": item.label, "B8": "Species", "A7": "Data grouped by:"} {
					actual, err := book.GetCellValue("ReportSummary", axis, excelize.Options{RawCellValue: true})
					if err != nil || actual != expected {
						t.Fatal("original grouping/order/selected-table metadata differs", item.name, axis, actual, expected, err)
					}
				}
			}
			additional := source.Ancillary
			source.Ancillary = nil
			expected, err := json.Marshal(struct {
				Scope string
				Input summaryVegetationAncillaryWorkbookInput
			}{summaryGroupedAncillaryTestScope, summaryVegetationAncillaryWorkbookInput{
				Source: source, Options: options, Tables: additional.Tables,
				CreatedAt: additional.CreatedAt, CreateSummary: item.summary,
			}})
			if err != nil || !bytes.Equal(expected, summaryEnvironmentalWorkbookSource(t, book)) {
				t.Fatal("complete grouped ancillary source/options/scope provenance differs", item.name, err)
			}
			if review.FirstVegetationRow != material.FirstVegetationRow || review.Bytes != len(material.Bytes) ||
				review.Ancillary.MembershipRows != len(source.Raw.SU.Rows) {
				t.Fatal("reviewed frame/membership count differs", item.name)
			}
			count++
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("grouped ancillary inspection changed preferences", err)
		}
	}
	if count != 16 {
		t.Fatal("managed/external grouped ancillary cases missing", count)
	}
}
