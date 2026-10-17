package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func summaryGroupedDesktopFixture(t *testing.T, external bool) (*SummaryVegetationWorkbookService, ProjectState) {
	t.Helper()
	service, state := summaryDraftWorkbookFixture(t, external)
	service.groupedEnabled = true
	qualifySummaryGroupedDesktopFixture(t, service, state)
	return service, state
}

func qualifySummaryGroupedDesktopFixture(t *testing.T, service *SummaryVegetationWorkbookService, state ProjectState) {
	t.Helper()
	owner := service.source.environment.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"],
		`UPDATE LayerCode SET Strata='A',LayerCode='01' WHERE Layer1234567='1'`)
	mutateContextFixture(t, owner.attachments["project"],
		`UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+` SET Cover1=0.5 WHERE PlotNumber='P3' AND Species='LF1'`)
}

func TestSummaryGroupedDesktopStrictAuthorityAndIndependentGate(t *testing.T) {
	prefix := `{"title":"literal","average":1,"display":6,"showClass":0,"includeEnv":false`
	for _, suffix := range []string{
		`,"groupAndOrder":null}`, `,"groupAndOrder":"12"}`, `,"groupAndOrder":12.0}`,
		`,"groupAndOrder":true}`, `,"groupAndOrder":[]}`, `,"groupAndOrder":{}}`,
		`,"GroupAndOrder":12}`, `,"groupAndOrder":12,"groupAndOrder":13}`,
		`,"groupAndOrder":12,"\u0067roupAndOrder":14}`,
	} {
		if err := json.Unmarshal([]byte(prefix+suffix), new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("ambiguous grouped authority accepted", suffix)
		}
	}
	service, state := summaryGroupedDesktopFixture(t, false)
	for _, grouping := range []int{0, 11, 12, 13, 14, 15} {
		options := SummaryVegetationWorkbookOptions{Title: "literal", Average: 1, Display: 6, GroupAndOrder: &grouping}
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if grouping < 12 || grouping > 14 {
			if err == nil || review != nil {
				t.Fatal("unsupported explicit grouped mode accepted", grouping, err)
			}
		} else if err != nil || review == nil || review.Scope != summaryVegetationGroupedWorkbookScope {
			source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
			p := source.Input.Vegetation
			t.Logf("grouped source: group=%d source=%q ordering=%q rows=%d units=%d columns=%v memberships=%d fields=%+v/%+v",
				p.GroupAndOrder, p.Source, p.Ordering, len(p.Rows), len(p.Units), p.Columns, len(p.Memberships), p.GroupField, p.OrderField)
			t.Fatal("supported explicit grouped mode unavailable", grouping, err)
		}
	}
	grouping := 12
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, GroupAndOrder: &grouping}
	service.groupedEnabled = false
	if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err == nil || review != nil || !strings.Contains(err.Error(), "grouped workbook is disabled") {
		t.Fatal("independent backend gate bypassed", review, err)
	}
	ordinary := options
	ordinary.GroupAndOrder = nil
	if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, ordinary)); err != nil || review == nil || review.Preview.GroupAndOrder != 0 || review.Scope != summaryVegetationWorkbookDraftScope {
		t.Fatal("group gate changed ordinary availability", review, err)
	}
	service.groupedEnabled, service.referencesEnabled, service.ancillaryEnabled = true, true, true
	for _, mode := range []string{"references", "memberships", "summary", "display2"} {
		service.ancillaryEnabled = mode != "memberships" && mode != "summary"
		changed := options
		switch mode {
		case "references":
			group := 13
			changed.GroupAndOrder = &group
			changed.References = &SummaryVegetationReferenceOptions{ShowCommon: true}
		case "memberships":
			changed.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{}
		case "summary":
			changed.Ancillary = &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}
		case "display2":
			changed.Display = 2
		}
		if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, changed)); err == nil || review != nil {
			t.Fatal("unqualified grouped combination accepted", mode, review, err)
		}
	}
}

func TestSummaryGroupedDesktopOwnedModesTypedOutputAndZeroWrites(t *testing.T) {
	count := 0
	for _, external := range []bool{false, true} {
		service, state := summaryGroupedDesktopFixture(t, external)
		contexts := service.source.environment.report.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, grouping := range []int{12, 13, 14} {
			for _, average := range []int{1, 2} {
				for _, display := range []int{1, 2, 3, 4, 5, 6} {
					if grouping != 14 && display == 2 {
						continue
					}
					shows := []int{1}
					if display == 4 || display == 5 {
						shows = []int{1, 10, 20}
					}
					for _, show := range shows {
						for _, includeEnv := range []bool{false, true} {
							options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: average,
								Display: display, ShowClass: show, IncludeEnv: includeEnv, GroupAndOrder: &grouping}
							review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
							if err != nil || review == nil || review.Scope != summaryVegetationGroupedWorkbookScope ||
								!reflect.DeepEqual(review.Options, options) || review.Preview.GroupAndOrder != grouping ||
								review.Preview.Source != "literal-normal-su-grouped" || (review.Environment != nil) != includeEnv {
								t.Fatal("owned grouped review differs", external, grouping, average, display, show, includeEnv, err)
							}
							source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
							material, err := prepareSummaryWorkbookSource(t.Context(), source, options)
							if err != nil {
								t.Fatal(err)
							}
							hash := sha256.Sum256(material.Bytes)
							if hex.EncodeToString(hash[:]) != review.WorkbookSHA256 || len(material.Bytes) != review.Bytes {
								t.Fatal("review hash/bytes do not match the complete grouped source")
							}
							book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: material.Bytes})
							if !includeEnv {
								group, err := book.GetCellValue(review.Sheet, "A5")
								if err != nil || group != review.Preview.GroupField.Name {
									t.Fatal("original group label differs", group, err)
								}
							} else if review.FirstVegetationRow != 36 || len(review.Environment.Rows) != 30 {
								t.Fatal("grouping changed complete environmental placement", review.FirstVegetationRow)
							}
							found := false
							for i, row := range review.Preview.Rows {
								if display == 6 && row.Order.Storage != "real" ||
									display != 6 && row.Order.Storage != "text" || grouping == 14 && row.Layer.Storage != "null" {
									t.Fatal("source typed output lost", grouping, display, row)
								}
								if average == 1 && display == 6 && *row.Species.Text == "Lifeform 1" {
									found = true
									if *row.Covers[0].Text != "1.56" || *row.Order.Real != 1.56 {
										t.Fatal("independent duplicated-membership mean differs", grouping, row)
									}
									value, err := book.GetCellValue(review.Sheet, fmt.Sprintf("C%d", review.FirstVegetationRow+i))
									if err != nil || value != "1.56" {
										t.Fatal("grouped real Order not independently reopened", value, err)
									}
								}
							}
							if average == 1 && display == 6 && !found {
								t.Fatal("required independent numeric group absent")
							}
							encoded, err := json.Marshal(struct {
								Source  summaryVegetationWorkbookSource
								Options SummaryVegetationWorkbookOptions
								Scope   string
							}{source, options, summaryVegetationGroupedWorkbookScope})
							if err != nil || !bytes.Equal(encoded, summaryEnvironmentalWorkbookSource(t, book)) {
								t.Fatal("full grouped source/options/scope provenance differs", err)
							}
							count++
						}
					}
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("grouped review wrote preferences", err)
		}
	}
	if count != 224 {
		t.Fatal("grouped desktop matrix coverage differs", count)
	}
}

func TestSummaryGroupedDesktopMissingPivotColumnRefusesAndRecovers(t *testing.T) {
	service, state := summaryGroupedDesktopFixture(t, false)
	owner := service.source.environment.report.contexts.projects.sqlite
	statement := `UPDATE ` + quoteHeaderIdentifier(state.ActiveProject+"_Veg") + ` SET Cover1=`
	mutateContextFixture(t, owner.attachments["project"], statement+`0 WHERE PlotNumber='P3' AND Species='LF1'`)
	grouping := 12
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, GroupAndOrder: &grouping}
	source := readSummaryAncillaryDesktopSource(t, service, state, options, "")
	if len(source.Input.Vegetation.Units) != 2 || !reflect.DeepEqual(source.Input.Vegetation.Columns, []string{"A"}) {
		t.Fatal("independent missing-pivot witness lost")
	}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err == nil || review != nil || !strings.Contains(err.Error(), "aligned units") {
		t.Fatal("missing source pivot column silently filled or dropped", review, err)
	}
	mutateContextFixture(t, owner.attachments["project"], statement+`0.5 WHERE PlotNumber='P3' AND Species='LF1'`)
	if review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options)); err != nil || review == nil {
		t.Fatal("corrected complete pivot did not recover", err)
	}
}

func TestSummaryGroupedDesktopPublicationAuthorityCollisionAndRetry(t *testing.T) {
	service, state := summaryGroupedDesktopFixture(t, true)
	grouping := 12
	options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true, GroupAndOrder: &grouping}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "group # ' literal.xlsx")
	request := summaryVegetationWorkbookExportRequest{Options: options, ApprovalHash: review.ApprovalHash, Destination: destination}
	for _, mode := range []string{"ordinary", "group", "gate", "cancel", "owner"} {
		changed := request
		ctx, contextID := t.Context(), state.ContextID
		switch mode {
		case "ordinary":
			changed.Options.GroupAndOrder = nil
		case "group":
			other := 13
			changed.Options.GroupAndOrder = &other
		case "gate":
			service.groupedEnabled = false
		case "cancel":
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			ctx = cancelled
		case "owner":
			contextID = "foreign"
		}
		result, err := service.ExportReviewed(ctx, contextID, workbookRequest(t, changed))
		service.groupedEnabled = true
		if err == nil || result != nil {
			t.Fatal("changed grouped authority returned outcome", mode, result, err)
		}
		if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refused grouped request left a destination", mode, err)
		}
	}
	result, err := service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "published" || result.SHA256 != review.WorkbookSHA256 {
		t.Fatal("grouped publication did not recover", result, err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.ExportReviewed(t.Context(), state.ContextID, workbookRequest(t, request))
	if err != nil || result == nil || result.Status != "not-published" || result.ErrorMessage == "" {
		t.Fatal("grouped collision was overwritten or hidden", result, err)
	}
	after, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("grouped collision changed the publication", err)
	}
}

func TestSummaryGroupedDesktopPhysicalPeerDriftAndCancellation(t *testing.T) {
	for _, phase := range []string{"staged", "published", "cancel"} {
		t.Run(phase, func(t *testing.T) {
			service, state := summaryGroupedDesktopFixture(t, false)
			grouping := 13
			options := SummaryVegetationWorkbookOptions{Average: 1, Display: 6, GroupAndOrder: &grouping}
			review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil {
				t.Fatal(err)
			}
			owner := service.source.environment.report.contexts.projects.sqlite
			destination := filepath.Join(t.TempDir(), "grouped.xlsx")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result, err := service.exportReviewed(ctx, state.ContextID, summaryDraftWorkbookExport(t, review, destination),
				summaryVegetationWorkbookHooks{publication: artifactPublicationHooks{observe: func(actual, _ string) error {
					if phase == "cancel" && actual == "staged" {
						cancel()
					} else if actual == phase {
						mutateContextFixture(t, owner.attachments["VLists"],
							`UPDATE USysAllSpecs SET EnglishName='Unrendered peer changed' WHERE Code='LF2'`)
					}
					return nil
				}}})
			expected := "not-published"
			if phase == "published" {
				expected = "published-with-errors"
			}
			if err != nil || result == nil || result.Status != expected || result.ErrorMessage == "" {
				t.Fatal("grouped peer drift/cancel lost publication state", result, err)
			}
			if phase == "published" && (result.SHA256 != review.WorkbookSHA256 || !strings.Contains(result.ErrorMessage, "do not replay")) {
				t.Fatal("committed grouped receipt became replayable", result)
			}
			if _, err := os.Stat(destination); phase != "published" && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused grouped publication left a destination", err)
			}
			again, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
			if err != nil || again == nil {
				t.Fatal("grouped read did not recover", err)
			}
			if phase != "cancel" && (again.ApprovalHash == review.ApprovalHash || again.WorkbookSHA256 == review.WorkbookSHA256) {
				t.Fatal("unrendered peer was not bound to grouped provenance")
			}
		})
	}
}
