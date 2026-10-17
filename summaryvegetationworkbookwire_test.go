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

func TestSummaryDraftWorkbookClientWire(t *testing.T) {
	source, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	service, err := NewSummaryVegetationWorkbookService(source, func(name string) (string, bool) {
		return "true", name == summaryVegetationWorkbookFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	wire := map[string]*SummaryVegetationWorkbookReview{}
	for _, item := range []struct {
		name    string
		options SummaryVegetationWorkbookOptions
	}{
		{"vegetation", SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: 1, Display: 6, ShowClass: 1}},
		{"environment", SummaryVegetationWorkbookOptions{Title: "=literal 001", Average: 2, Display: 4, ShowClass: 20, IncludeEnv: true}},
	} {
		request, err := json.Marshal(item.options)
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(t.Context(), state.ContextID, string(request))
		if err != nil {
			t.Fatal(err)
		}
		// Portable transport evidence, not a usable approval of a real project.
		got.Preview.ContextID = "summary-workbook-wire"
		got.Preview.ProjectPath = `C:\fixture\Sample.db`
		got.Preview.SUPath = `C:\fixture\Sample.db`
		got.ApprovalHash = strings.Repeat("0", 64)
		input := summaryVegetationWorkbookInput{Title: item.options.Title, Vegetation: got.Preview}
		if environment := got.Environment; environment != nil {
			environment.Vegetation = got.Preview
			environment.SourcePath = `C:\fixture\VPro64.db`
			environment.EnvSourcePath = `C:\fixture\Sample.db`
			environment.MasterSourcePath = `C:\fixture\VLists.db`
			input.Environment = &summaryEnvironmentalWorkbookInput{FirstRow: 5,
				FieldList: environment.FieldList, Units: summaryEnvironmentalPreviewUnits(got.Preview), Rows: environment.Rows}
		}
		workbook, err := prepareSummaryVegetationWorkbook(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(workbook.Bytes)
		got.WorkbookSHA256, got.Bytes = hex.EncodeToString(digest[:]), len(workbook.Bytes)
		wire[item.name] = got
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
		t.Fatal("strict production review DTO round trip differs", err)
	}
	path := filepath.Join("frontend", "src", "summaryVegetationWorkbookWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_WORKBOOK_WIRE") == "true" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(expected, raw) {
		t.Fatal("portable strict-Go Summary workbook wire differs", err)
	}
}
