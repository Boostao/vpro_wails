package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type summaryGroupedAttributeWorkbookCase struct {
	name                          string
	group, average, display, show int
	env                           bool
}

func summaryGroupedAttributeWorkbookCases() []summaryGroupedAttributeWorkbookCase {
	return []summaryGroupedAttributeWorkbookCase{
		{"lifeform", 13, 1, 6, 1, false}, {"lifeformEnvironment", 13, 2, 6, 1, true},
		{"lifeformClass", 13, 1, 4, 20, false}, {"lifeformDistribution", 13, 2, 3, 1, true},
		{"none", 14, 1, 6, 1, false}, {"nonePresence", 14, 2, 2, 1, true},
		{"noneClass", 14, 1, 5, 10, false}, {"noneDistribution", 14, 2, 1, 1, true},
	}
}

func summaryGroupedAttributeWorkbookOptions(item summaryGroupedAttributeWorkbookCase) SummaryVegetationWorkbookOptions {
	group := item.group
	return SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
		ShowClass: item.show, IncludeEnv: item.env, GroupAndOrder: &group,
		References: &SummaryVegetationReferenceOptions{}}
}

func TestSummaryGroupedAttributeWorkbookOwnedOutputAndZeroWrites(t *testing.T) {
	count := 0
	for _, external := range []bool{false, true} {
		service, state := summaryStrataReferenceWorkbookFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range summaryGroupedAttributeWorkbookCases() {
			options := summaryGroupedAttributeWorkbookOptions(item)
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || review == nil || review.Scope != summaryVegetationGroupedAttributesScope ||
				!reflect.DeepEqual(review.Options, options) || review.Preview.GroupAndOrder != item.group ||
				review.Preview.Source != "literal-normal-su-effective-references" || len(review.Preview.Rows) == 0 ||
				review.Preview.References == nil || review.Preview.References.ShowCommon || review.Preview.References.UseGuideNames ||
				(review.Environment != nil) != item.env {
				t.Fatal("owned grouped attribute review differs", external, item.name, review, err)
			}
			derived, nullGroup := false, false
			for _, row := range review.Preview.Rows {
				if row.CommonName != nil {
					t.Fatal("unselected common name leaked")
				}
				nullGroup = nullGroup || row.Layer.Storage == "null"
				for _, witness := range row.Witnesses {
					for _, origin := range witness.ReferenceOrigins {
						derived = derived || origin.Table == "USysSppAttributeDescription"
					}
				}
			}
			if !derived || !nullGroup {
				t.Fatal("derived attribute or source NULL group lost", item.name, derived, nullGroup)
			}
			source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
			material, err := prepareSummaryWorkbookSource(t.Context(), source, options)
			if err != nil {
				t.Fatal(err)
			}
			book := assertSummaryGroupedWorkbookBody(t, source.Input, material)
			expected, err := json.Marshal(struct {
				Source  summaryVegetationWorkbookSource
				Options SummaryVegetationWorkbookOptions
				Scope   string
			}{source, options, summaryVegetationGroupedAttributesScope})
			if err != nil || !bytes.Equal(expected, summaryEnvironmentalWorkbookSource(t, book)) {
				t.Fatal("complete grouped attribute hidden source/options/scope differ", item.name, err)
			}
			if review.FirstVegetationRow != material.FirstVegetationRow || review.Sheet != material.Sheet ||
				review.Bytes != len(material.Bytes) {
				t.Fatal("reviewed grouped attribute frame differs", item.name)
			}
			count++
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(after, config) {
			t.Fatal("grouped attribute review changed preferences", err)
		}
	}
	if count != 16 {
		t.Fatal("managed/external grouped attribute cases missing", count)
	}
}

func TestSummaryGroupedAttributeWorkbookIndependentGatesAndUnavailableNames(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	service.ancillaryEnabled = true
	for _, grouping := range []int{13, 14} {
		options := summaryGroupedAttributeWorkbookOptions(summaryGroupedAttributeWorkbookCase{group: grouping, average: 1, display: 6})
		for _, grouped := range []bool{false, true} {
			for _, references := range []bool{false, true} {
				service.groupedEnabled, service.referencesEnabled = grouped, references
				review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
				if grouped && references {
					if err != nil || review == nil {
						t.Fatal("joint authority refused grouped attributes", grouping, err)
					}
				} else if err == nil || review != nil || !strings.Contains(err.Error(), "disabled") {
					t.Fatal("independent grouped attribute gate bypassed", grouping, grouped, references, review, err)
				}
			}
		}
		for _, common := range []bool{false, true} {
			for _, guide := range []bool{false, true} {
				if !common && !guide {
					continue
				}
				changed := options
				changed.References = &SummaryVegetationReferenceOptions{ShowCommon: common, UseGuideNames: guide}
				if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, changed)); err == nil || review != nil {
					t.Fatal("unqualified First(common name) granted", grouping, common, guide, review, err)
				}
			}
		}
		service.ancillaryEnabled = false
		for _, summary := range []bool{false, true} {
			changed := options
			changed.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: summary}
			if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, changed)); err == nil ||
				review != nil || !strings.Contains(err.Error(), "ancillary workbook is disabled") {
				t.Fatal("disabled grouped ancillary authority inferred", grouping, summary, err)
			}
		}
		service.ancillaryEnabled = true
	}
}

func TestSummaryGroupedAttributeWorkbookPublicationAuthorityAndRecovery(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, true)
	for _, grouping := range []int{13, 14} {
		options := summaryGroupedAttributeWorkbookOptions(summaryGroupedAttributeWorkbookCase{group: grouping, average: 1, display: 6})
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(t.TempDir(), "Attributes # ' literal.xlsx")
		request := summaryVegetationWorkbookExportRequest{Options: options, ApprovalHash: review.ApprovalHash, Destination: destination}
		for _, mode := range []string{"layer", "plain", "other-group", "common", "guide", "group-gate", "reference-gate", "cancel", "owner", "unrendered-peer"} {
			changed := request
			ctx, contextID := t.Context(), state.ContextID
			owner := service.source.environment.report.contexts.projects.sqlite
			switch mode {
			case "layer":
				changed.Options.GroupAndOrder = nil
			case "plain":
				changed.Options.References = nil
			case "other-group":
				other := 14
				if grouping == 14 {
					other = 13
				}
				changed.Options.GroupAndOrder = &other
			case "common":
				changed.Options.References = &SummaryVegetationReferenceOptions{ShowCommon: true}
			case "guide":
				changed.Options.References = &SummaryVegetationReferenceOptions{UseGuideNames: true}
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
				t.Fatal("changed grouped attribute authority returned an outcome", grouping, mode, result, err)
			}
			if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused grouped attribute authority created output", grouping, mode, err)
			}
		}
		result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
		if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
			t.Fatal("exact source recovery did not publish reviewed grouped attributes", grouping, result, err)
		}
		before, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		result, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
		after, readErr := os.ReadFile(destination)
		if err != nil || result == nil || result.Status != "not-published" || result.ErrorMessage == "" ||
			readErr != nil || !bytes.Equal(before, after) {
			t.Fatal("publication collision changed grouped attribute output", grouping, result, err, readErr)
		}
	}
}

func TestSummaryGroupedAttributeWorkbookClientWire(t *testing.T) {
	service, state := summaryStrataReferenceWorkbookFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range summaryGroupedAttributeWorkbookCases() {
		wire[item.name] = portableSummaryWorkbookReview(t, service, state, summaryGroupedAttributeWorkbookOptions(item))
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookGroupedAttributesWire.json",
		"VPRO_UPDATE_SUMMARY_GROUPED_ATTRIBUTE_WORKBOOK_WIRE", wire)
}
