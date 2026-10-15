package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type projectWorkbookWireCase struct {
	Review        LongVegetationProjectWorkbookReview    `json:"review"`
	ReadRequest   vegetationProjectWorkbookReadRequest   `json:"readRequest"`
	ExportRequest vegetationProjectWorkbookExportRequest `json:"exportRequest"`
	Outcome       VegetationWorkbookOutcome              `json:"outcome"`
}

func TestProjectWorkbookStrictClientWireMatchesOriginalKernelAndSummary(t *testing.T) {
	cases := []projectWorkbookWireCase{}
	for i, mode := range []struct {
		kind      string
		qualified bool
	}{{vegetationProjectFieldsScope, false}, {vegetationProjectAllScope, false}, {vegetationProjectFieldsScope, true}} {
		source, scope, settings, summary := projectSummaryWorkbookFixture(t, mode.kind, mode.qualified)
		layout := vegetationWorkbookLayout{ReportSummary: true, SpaceBetweenGroups: true}
		prepared, err := prepareLongVegetationProjectWorkbookWithSummary(context.Background(), source, scope, settings, layout, &summary)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(prepared.Workbook.Bytes)
		review := LongVegetationProjectWorkbookReview{
			Scope: prepared.Preview.Scope, Preview: prepared.Preview.Preview,
			Options: VegetationWorkbookOptions{false, true, true}, ReportScope: "unlumped", CreatedDate: "2026-10-10",
			Summary: &VegetationProjectWorkbookSummary{summary.Header.CreatedDate, summary.Header.EnvironmentRows,
				summary.SourcePlotRows, summary.SourceKind, summary.ProjectTuples, summary.Header.SpeciesVersion,
				summary.Header.VersionStatus, summary.Header.VersionDefinitions},
			// The literal approval tests transport; complete owned approval authority has its own physical-source tests.
			ApprovalHash: strings.Repeat(string(rune('b'+i)), 64), WorkbookSHA256: hex.EncodeToString(digest[:]),
			Bytes: len(prepared.Workbook.Bytes), Sheets: []VegetationWorkbookSheet{}, SkippedUnits: []VegetationWorkbookSkippedUnit{},
		}
		for _, sheet := range prepared.Workbook.Sheets {
			review.Sheets = append(review.Sheets, VegetationWorkbookSheet{sheet.Unit, sheet.Name})
		}
		for _, skipped := range prepared.Workbook.SkippedUnits {
			review.SkippedUnits = append(review.SkippedUnits, VegetationWorkbookSkippedUnit{skipped.Unit, skipped.Reason})
		}
		expectedTuples, expectedPlots := []int{4, 2, 2}[i], []int{4, 2, 1}[i]
		if len(review.Scope.Build.ProjectTuples) != expectedTuples || review.Summary.SourcePlotRows != expectedPlots ||
			review.Summary.EnvironmentRows != 2 || len(review.Preview.Report.ProjectUnits) != expectedPlots ||
			i == 2 && len(review.Preview.Report.Quality.Occurrences) != 8 {
			t.Fatal("project wire lost independent original Build/count/qualified expectations", i, review)
		}
		destination := `C:\output\literal # 'é'.xlsx`
		cases = append(cases, projectWorkbookWireCase{
			Review: review, ReadRequest: vegetationProjectWorkbookReadRequest{scope.Revision, scope.Build.ApprovalHash, "unlumped"},
			ExportRequest: vegetationProjectWorkbookExportRequest{scope.Revision, scope.Build.ApprovalHash, "unlumped",
				review.CreatedDate, review.ApprovalHash, destination},
			Outcome: VegetationWorkbookOutcome{Status: "published", RequestedDestination: destination, Path: destination, SHA256: review.WorkbookSHA256},
		})
	}
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("frontend", "src", "longVegetationProjectWorkbookWire.json")
	if os.Getenv("VPRO_UPDATE_PROJECT_WORKBOOK_WIRE") == "true" {
		if err := os.WriteFile(fixture, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var expected []projectWorkbookWireCase
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil || !reflect.DeepEqual(cases, expected) {
		t.Fatal("project workbook client fixture differs from strict production DTO/kernel/summary", err)
	}
	var reopened []projectWorkbookWireCase
	decoder = json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(reopened, expected) {
		t.Fatal("production roundtrip changed project workbook approval/request/outcome wire", err)
	}
}
