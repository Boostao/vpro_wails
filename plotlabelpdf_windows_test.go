package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func plotLabelPDFWindowsTestFonts(t *testing.T) plotLabelPDFFonts {
	t.Helper()
	directory, err := windows.GetWindowsDirectory()
	if err != nil {
		t.Fatal(err)
	}
	regular, err := os.ReadFile(filepath.Join(directory, "Fonts", "arial.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	bold, err := os.ReadFile(filepath.Join(directory, "Fonts", "arialbd.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	return plotLabelPDFFonts{regular, bold}
}

func TestPlotLabelPDFFontIdentityAndPermissions(t *testing.T) {
	fonts := plotLabelPDFWindowsTestFonts(t)
	for _, font := range []struct {
		data []byte
		name string
	}{{fonts.regular, "ArialMT"}, {fonts.bold, "Arial-BoldMT"}} {
		if err := validatePlotLabelPDFFont(font.data, font.name); err != nil {
			t.Fatal(err)
		}
		if err := validatePlotLabelPDFFont(font.data, "Helvetica"); err == nil {
			t.Fatal("font substitution accepted")
		}
		for _, flags := range []uint16{0, 4, 8, 2, 0x100, 0x200, 0xc, 0xffff} {
			data := bytes.Clone(font.data)
			found := false
			for i := 0; i < int(binary.BigEndian.Uint16(data[4:6])); i++ {
				table := data[12+i*16 : 12+(i+1)*16]
				if string(table[:4]) == "OS/2" {
					offset := int(binary.BigEndian.Uint32(table[8:12]))
					binary.BigEndian.PutUint16(data[offset+8:offset+10], flags)
					found = true
				}
			}
			if !found {
				t.Fatal("actual font lacks OS/2")
			}
			err := validatePlotLabelPDFFont(data, font.name)
			if (err == nil) != (flags == 0 || flags == 8) {
				t.Fatalf("permissions 0x%x: %v", flags, err)
			}
		}
	}
}

func TestPlotLabelPDFActualArialPreparation(t *testing.T) {
	fonts := plotLabelPDFWindowsTestFonts(t)
	regularBefore, boldBefore := bytes.Clone(fonts.regular), bytes.Clone(fonts.bold)
	preview := plotLabelPDFTestPreview()
	before, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}

	output, err := renderPlotLabelPDF(context.Background(), preview, fonts)
	if err != nil {
		t.Fatal(err)
	}
	again, err := renderPlotLabelPDF(context.Background(), preview, fonts)
	if err != nil || !bytes.Equal(output, again) {
		t.Fatalf("non-deterministic preparation: %v", err)
	}
	after, err := json.Marshal(preview)
	if err != nil || !bytes.Equal(before, after) ||
		!bytes.Equal(regularBefore, fonts.regular) || !bytes.Equal(boldBefore, fonts.bold) {
		t.Fatalf("source/font mutation: %v", err)
	}
	for _, field := range []string{"representing", "project", "zone"} {
		p := plotLabelPDFTestPreview()
		switch field {
		case "representing":
			value := strings.Repeat("W", 255)
			p.Header.PlotRepresenting = &value
		case "project":
			value := strings.Repeat("W", 50)
			p.Header.ProjectID = &value
		case "zone":
			value := strings.Repeat("W", 40)
			p.Header.SiteSeries = &value
		}
		if out, err := renderPlotLabelPDF(context.Background(), p, fonts); err == nil ||
			!strings.Contains(err.Error(), "clipping/wrapping") || out != nil {
			t.Fatalf("%s overflow: %q, %v", field, out, err)
		}
	}
	if evidence := os.Getenv("VPRO_PRIVATE_LABEL_PDF_EVIDENCE"); evidence != "" {
		for _, blank := range []bool{false, true} {
			p := plotLabelPDFTestPreview()
			name := "full.pdf"
			if blank {
				p.Header.ProjectID, p.Header.PlotRepresenting = nil, nil
				name = "null.pdf"
			}
			data, err := renderPlotLabelPDF(context.Background(), p, fonts)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(filepath.Join(evidence, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			n, writeErr := file.Write(data)
			closeErr := file.Close()
			if n != len(data) || writeErr != nil || closeErr != nil {
				t.Fatalf("evidence write: %d/%d, %v, %v", n, len(data), writeErr, closeErr)
			}
		}
	}
}
