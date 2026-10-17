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

func portableSummaryWorkbookReview(t *testing.T, service *SummaryVegetationWorkbookService, state ProjectState,
	options SummaryVegetationWorkbookOptions) *SummaryVegetationWorkbookReview {
	t.Helper()
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
	review.Preview.ProjectPath, review.Preview.SUPath = `C:\fixture\Sample.db`, `C:\fixture\Sample.db`
	review.ApprovalHash = strings.Repeat("0", 64)
	if environment := review.Environment; environment != nil {
		environment.Vegetation = review.Preview
		environment.SourcePath, environment.EnvSourcePath, environment.MasterSourcePath =
			`C:\fixture\VPro64.db`, `C:\fixture\Sample.db`, `C:\fixture\VLists.db`
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
	return review
}

func assertSummaryWorkbookClientWire(t *testing.T, name, updateFlag string, wire map[string]*SummaryVegetationWorkbookReview) {
	t.Helper()
	raw, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	var decoded map[string]*SummaryVegetationWorkbookReview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(decoded, wire) {
		t.Fatal("strict production workbook review DTO round trip differs", name, err)
	}
	file := filepath.Join("frontend", "src", name)
	if os.Getenv(updateFlag) == "true" {
		if err := os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(expected, raw) {
		t.Fatal("portable strict-Go workbook wire differs", name, err)
	}
}

func TestSummaryGroupedDesktopClientWire(t *testing.T) {
	service, state := summaryGroupedDesktopFixture(t, false)
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range []struct {
		name                          string
		group, average, display, show int
		env                           bool
	}{
		{"strata", 12, 1, 6, 1, false}, {"strataEnvironment", 12, 1, 6, 1, true},
		{"lifeform", 13, 2, 6, 1, false}, {"lifeformEnvironment", 13, 2, 6, 1, true},
		{"none", 14, 1, 6, 1, false}, {"noneEnvironment", 14, 1, 6, 1, true},
		{"strataClass", 12, 2, 4, 20, true}, {"nonePresence", 14, 1, 2, 1, true},
	} {
		options := SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: item.average, Display: item.display,
			ShowClass: item.show, IncludeEnv: item.env, GroupAndOrder: &item.group}
		wire[item.name] = portableSummaryWorkbookReview(t, service, state, options)
	}
	assertSummaryWorkbookClientWire(t, "summaryVegetationWorkbookGroupedWire.json",
		"VPRO_UPDATE_SUMMARY_GROUPED_WORKBOOK_WIRE", wire)
}
