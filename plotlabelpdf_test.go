package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func plotLabelPDFTestPreview() PlotLabelRegionalPreview {
	zone, subzone, site, representing, project := "A", "B", "", "  represented plot 1  ", "ID-1"
	return PlotLabelRegionalPreview{
		ContextID: "owned-context", PlotNumber: "P01",
		Header:    SavedPlotLabelHeader{"P01", &zone, &subzone, &site, &representing, &project},
		LabelDate: "Monday, October 12, 2026", ISODate: "2026-10-12",
		Locale: "en-US", Pattern: "dddd, MMMM d, yyyy", Source: "windows-user-long-date",
	}
}

func TestPlotLabelPDFSourceProjection(t *testing.T) {
	preview := plotLabelPDFTestPreview()
	before, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	fields, date, err := plotLabelPDFFields(preview)
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, field := range fields {
		actual = append(actual, field.text)
	}
	expected := []string{"P01", "represented plot 1", "A   B/", "Label Date: Monday, October 12, 2026", "ID-1", "(VPro)"}
	if !reflect.DeepEqual(actual, expected) || date.Format("2006-01-02") != "2026-10-12" {
		t.Fatalf("unexpected projection: %v, %v", actual, date)
	}
	after, err := json.Marshal(preview)
	if err != nil || string(before) != string(after) {
		t.Fatalf("source changed: %s -> %s, %v", before, after, err)
	}
}

func TestPlotLabelPDFNullEmptyAndLiteralSpaces(t *testing.T) {
	for _, empty := range []bool{false, true} {
		preview := plotLabelPDFTestPreview()
		preview.Header.PlotRepresenting, preview.Header.ProjectID = nil, nil
		if empty {
			value := ""
			preview.Header.PlotRepresenting, preview.Header.ProjectID = &value, &value
		}
		fields, _, err := plotLabelPDFFields(preview)
		if err != nil || fields[1].text != "" || fields[4].text != "" {
			t.Fatalf("blank metadata: %v, %v", fields, err)
		}
		if (preview.Header.ProjectID == nil) == empty {
			t.Fatal("NULL/empty source distinction lost")
		}
	}
	preview := plotLabelPDFTestPreview()
	preview.PlotNumber, preview.Header.PlotNumber = " P01 ", " P01 "
	project := " ID-1 "
	preview.Header.ProjectID = &project
	fields, _, err := plotLabelPDFFields(preview)
	if err != nil || fields[0].text != "P01" || fields[4].text != " ID-1 " {
		t.Fatalf("literal display: %v, %v", fields, err)
	}
}

func TestPlotLabelPDFSourceDateCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "plot-label-regional-date.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct{ ISODate, Actual string }
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 12 {
		t.Fatalf("source cases: %d", len(fixture.Cases))
	}
	for _, source := range fixture.Cases {
		preview := plotLabelPDFTestPreview()
		preview.ISODate, preview.LabelDate = source.ISODate, strings.TrimPrefix(source.Actual, "Label Date: ")
		fields, _, err := plotLabelPDFFields(preview)
		if err != nil || fields[3].text != source.Actual {
			t.Fatalf("%s: %v, %v", source.ISODate, fields, err)
		}
	}
}

func TestPlotLabelPDFRefusesUnsupportedProjection(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*PlotLabelRegionalPreview)
	}{
		{"owner", "matching saved-plot owner", func(p *PlotLabelRegionalPreview) { p.ContextID = "" }},
		{"identity", "matching saved-plot owner", func(p *PlotLabelRegionalPreview) { p.Header.PlotNumber = "P02" }},
		{"null-zone", "non-NULL Zone", func(p *PlotLabelRegionalPreview) { p.Header.Zone = nil }},
		{"long-zone", "UTF-16 bound", func(p *PlotLabelRegionalPreview) { s := "ABCDE"; p.Header.Zone = &s }},
		{"long-project", "UTF-16 bound", func(p *PlotLabelRegionalPreview) { s := strings.Repeat("x", 51); p.Header.ProjectID = &s }},
		{"combined-zone", "Zone1", func(p *PlotLabelRegionalPreview) { s := strings.Repeat("x", 46); p.Header.SiteSeries = &s }},
		{"invalid-utf8", "repairing source text", func(p *PlotLabelRegionalPreview) { s := string([]byte{0xff}); p.Header.ProjectID = &s }},
		{"nul", "repairing source text", func(p *PlotLabelRegionalPreview) { s := "a\x00"; p.Header.PlotRepresenting = &s }},
		{"tab", "printable ASCII", func(p *PlotLabelRegionalPreview) { s := "a\t"; p.Header.PlotRepresenting = &s }},
		{"newline", "printable ASCII", func(p *PlotLabelRegionalPreview) { s := "a\n"; p.Header.PlotRepresenting = &s }},
		{"unicode", "printable ASCII", func(p *PlotLabelRegionalPreview) { s := "\u00e9"; p.Header.ProjectID = &s }},
		{"astral", "printable ASCII", func(p *PlotLabelRegionalPreview) { s := "\U0001f332"; p.Header.ProjectID = &s }},
		{"date", "ISO date", func(p *PlotLabelRegionalPreview) { p.ISODate = "2026-02-30" }},
		{"early-date", "ISO date", func(p *PlotLabelRegionalPreview) { p.ISODate = "1600-01-01" }},
		{"date-mismatch", "calibrated en-US", func(p *PlotLabelRegionalPreview) { p.LabelDate = "Tuesday, October 12, 2026" }},
		{"locale", "calibrated en-US", func(p *PlotLabelRegionalPreview) { p.Locale = "fr-CA" }},
		{"pattern", "calibrated en-US", func(p *PlotLabelRegionalPreview) { p.Pattern = "MMMM d, yyyy" }},
		{"source", "calibrated en-US", func(p *PlotLabelRegionalPreview) { p.Source = "browser" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			preview := plotLabelPDFTestPreview()
			test.change(&preview)
			output, err := renderPlotLabelPDF(context.Background(), preview, plotLabelPDFFonts{})
			if err == nil || !strings.Contains(err.Error(), test.want) || output != nil {
				t.Fatalf("expected %q and zero output, got %q, %v", test.want, output, err)
			}
		})
	}
}

func TestPlotLabelPDFContextAndMissingFonts(t *testing.T) {
	preview := plotLabelPDFTestPreview()
	if output, err := renderPlotLabelPDF(nil, preview, plotLabelPDFFonts{}); err == nil || output != nil {
		t.Fatalf("nil context: %q, %v", output, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if output, err := renderPlotLabelPDF(ctx, preview, plotLabelPDFFonts{}); !errors.Is(err, context.Canceled) || output != nil {
		t.Fatalf("cancellation: %q, %v", output, err)
	}
	if output, err := renderPlotLabelPDF(context.Background(), preview, plotLabelPDFFonts{}); err == nil ||
		!strings.Contains(err.Error(), "font parse") || output != nil {
		t.Fatalf("missing fonts: %q, %v", output, err)
	}
}
