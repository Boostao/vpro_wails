package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryAncillaryDesktopClientWire(t *testing.T) {
	service, state := summaryAncillaryDesktopFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range []struct {
		name       string
		includeEnv bool
		summary    bool
		average    int
		display    int
	}{
		{"memberships", false, false, 1, 6},
		{"membershipsEnvironment", true, false, 2, 4},
		{"summary", false, true, 1, 6},
		{"summaryEnvironment", true, true, 2, 4},
	} {
		options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
			ShowClass: 20, IncludeEnv: item.includeEnv,
			Ancillary: &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: item.summary}}
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if err != nil {
			t.Fatal(err)
		}
		source := readSummaryAncillaryDesktopSource(t, service, state, options, review.Ancillary.CreatedAt)
		// Portable transport evidence, not a usable approval of a real project.
		review.Preview.ContextID = "summary-workbook-wire"
		review.Preview.ProjectPath = `C:\fixture\Sample.db`
		review.Preview.SUPath = `C:\fixture\Sample.db`
		review.ApprovalHash = strings.Repeat("0", 64)
		if environment := review.Environment; environment != nil {
			environment.Vegetation = review.Preview
			environment.SourcePath = `C:\fixture\VPro64.db`
			environment.EnvSourcePath = `C:\fixture\Sample.db`
			environment.MasterSourcePath = `C:\fixture\VLists.db`
		}
		source.Input.Vegetation, source.Raw.Preview = review.Preview, review.Preview
		for i, path := range source.Paths {
			name := path.Role
			if name == "project" || name == "su" {
				name = "Sample"
			}
			source.Paths[i].Path = `C:\fixture\` + name + ".db"
		}
		workbook, err := prepareSummaryWorkbookSource(t.Context(), source, options)
		if err != nil {
			t.Fatal(err)
		}
		metadata, err := summaryWorkbookAncillaryReview(source, workbook, options)
		if err != nil || !reflect.DeepEqual(metadata, review.Ancillary) {
			t.Fatal("portable fixture changed actual production review metadata", err)
		}
		digest := sha256.Sum256(workbook.Bytes)
		review.WorkbookSHA256, review.Bytes = hex.EncodeToString(digest[:]), len(workbook.Bytes)
		wire[item.name] = review
	}
	raw, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	var decoded map[string]*SummaryVegetationWorkbookReview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(decoded, wire) {
		t.Fatal("strict production ancillary review DTO round trip differs", err)
	}
	path := filepath.Join("frontend", "src", "summaryVegetationWorkbookAncillaryWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_ANCILLARY_WORKBOOK_WIRE") == "true" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(expected, raw) {
		t.Fatal("portable strict-Go ancillary workbook wire differs", err)
	}
}
