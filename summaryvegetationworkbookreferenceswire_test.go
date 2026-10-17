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

func TestSummaryEffectiveReferenceWorkbookClientWire(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range []struct {
		name       string
		references SummaryVegetationReferenceOptions
		env        bool
		ancillary  *SummaryVegetationWorkbookAncillaryOptions
	}{
		{"attributes", SummaryVegetationReferenceOptions{}, false, nil},
		{"common", SummaryVegetationReferenceOptions{ShowCommon: true}, false, nil},
		{"guide", SummaryVegetationReferenceOptions{UseGuideNames: true}, false, nil},
		{"guideCommon", SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}, false, nil},
		{"environment", SummaryVegetationReferenceOptions{ShowCommon: true}, true, nil},
		{"memberships", SummaryVegetationReferenceOptions{ShowCommon: true}, false, &SummaryVegetationWorkbookAncillaryOptions{}},
		{"summary", SummaryVegetationReferenceOptions{ShowCommon: true}, true, &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}},
		{"guideSummary", SummaryVegetationReferenceOptions{ShowCommon: true, UseGuideNames: true}, true, &SummaryVegetationWorkbookAncillaryOptions{CreateSummary: true}},
	} {
		options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: 1, Display: 6, ShowClass: 1,
			IncludeEnv: item.env, References: &item.references, Ancillary: item.ancillary}
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if err != nil {
			t.Fatal(err)
		}
		created := ""
		if review.Ancillary != nil {
			created = review.Ancillary.CreatedAt
		}
		source := readSummaryAncillaryDesktopSource(t, service, state, options, created)
		// Portable transport evidence cannot authorize publication of a real project.
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
		material, err := prepareSummaryWorkbookSource(t.Context(), source, options)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(material.Bytes)
		review.WorkbookSHA256, review.Bytes = hex.EncodeToString(digest[:]), len(material.Bytes)
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
		t.Fatal("strict production reference review DTO round trip differs", err)
	}
	path := filepath.Join("frontend", "src", "summaryVegetationWorkbookReferencesWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_REFERENCE_WORKBOOK_WIRE") == "true" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(expected, raw) {
		t.Fatal("portable strict-Go reference workbook wire differs", err)
	}
}
