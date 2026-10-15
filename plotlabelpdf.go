package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/sfnt"
)

// Private portable preparation, not Access printer geometry or an export API.
type plotLabelPDFFonts struct {
	regular, bold []byte
}

type plotLabelPDFField struct {
	name, text         string
	left, top, width   float64
	size               float64
	bold, rightAligned bool
}

func plotLabelPDFFields(preview PlotLabelRegionalPreview) ([]plotLabelPDFField, time.Time, error) {
	if preview.ContextID == "" || preview.PlotNumber == "" || preview.PlotNumber != preview.Header.PlotNumber {
		return nil, time.Time{}, errors.New("label PDF requires a complete matching saved-plot owner")
	}
	h := preview.Header
	for _, item := range []struct {
		name  string
		value *string
		limit int
	}{
		{"PlotNumber1", &h.PlotNumber, 7}, {"Zone", h.Zone, 4}, {"SubZone", h.SubZone, 50},
		{"SiteSeries", h.SiteSeries, 50}, {"Representing1", h.PlotRepresenting, 255}, {"ProjectID1", h.ProjectID, 50},
	} {
		if item.value != nil {
			if err := plotLabelText(*item.value, item.name, item.limit); err != nil {
				return nil, time.Time{}, err
			}
		}
	}
	if h.Zone == nil {
		return nil, time.Time{}, errors.New("label PDF requires non-NULL Zone for source Len/Space")
	}
	value := func(text *string) string {
		if text == nil {
			return ""
		}
		return *text
	}
	zone := *h.Zone + strings.Repeat(" ", 4-len(utf16.Encode([]rune(*h.Zone)))) + value(h.SubZone) + "/" + value(h.SiteSeries)
	if err := plotLabelText(zone, "Zone1", 50); err != nil {
		return nil, time.Time{}, err
	}
	date, err := time.Parse("2006-01-02", preview.ISODate)
	if err != nil || date.Year() < 1601 || date.Year() > 9999 {
		return nil, time.Time{}, errors.Join(err, errors.New("label PDF requires a supported complete ISO date"))
	}
	if preview.Source != "windows-user-long-date" || preview.Locale != "en-US" ||
		preview.Pattern != "dddd, MMMM d, yyyy" || preview.LabelDate != date.Format("Monday, January 2, 2006") {
		return nil, time.Time{}, errors.New("label PDF requires the calibrated en-US Windows Long Date; no locale fallback")
	}
	fields := []plotLabelPDFField{
		{"PlotNumber1", strings.Trim(h.PlotNumber, " "), 72, 60, 1213, 9, true, false},
		{"Representing1", strings.Trim(value(h.PlotRepresenting), " "), 72, 300, 4753, 6, false, false},
		{"Zone1", zone, 2448, 60, 2385, 9, true, true},
		{"Date", "Label Date: " + preview.LabelDate, 507, 660, 4320, 6, false, true},
		{"ProjectID1", value(h.ProjectID), 1296, 60, 1155, 8, true, false},
		{"RK", "(VPro)", 75, 660, 420, 6, false, false},
	}
	for _, field := range fields {
		for _, r := range field.text {
			if r < 32 || r > 126 {
				return nil, time.Time{}, fmt.Errorf("%s label PDF glyph/layout is unavailable outside printable ASCII", field.name)
			}
		}
	}
	return fields, date, nil
}

func validatePlotLabelPDFFont(data []byte, expectedName string) error {
	font, err := sfnt.Parse(data)
	if err != nil {
		return fmt.Errorf("label PDF font parse: %w", err)
	}
	name, err := font.Name(nil, sfnt.NameID(6))
	if err != nil || name != expectedName {
		return errors.Join(err, fmt.Errorf("label PDF requires original %s, not substituted font %q", expectedName, name))
	}
	if len(data) < 12 || binary.BigEndian.Uint32(data[:4]) != 0x00010000 {
		return errors.New("label PDF requires TrueType outlines")
	}
	count := int(binary.BigEndian.Uint16(data[4:6]))
	if count > (len(data)-12)/16 {
		return errors.New("label PDF font table directory is incomplete")
	}
	found := false
	for i := 0; i < count; i++ {
		table := data[12+i*16 : 12+(i+1)*16]
		if string(table[:4]) != "OS/2" {
			continue
		}
		if found {
			return errors.New("label PDF font has duplicate embedding metadata")
		}
		found = true
		offset, length := uint64(binary.BigEndian.Uint32(table[8:12])), uint64(binary.BigEndian.Uint32(table[12:16]))
		if length < 10 || offset+length > uint64(len(data)) {
			return errors.New("label PDF font embedding metadata is incomplete")
		}
		flags := binary.BigEndian.Uint16(data[offset+8 : offset+10])
		if flags != 0 && flags != 8 {
			return fmt.Errorf("label PDF font embedding/subsetting permissions are unsupported: 0x%04x", flags)
		}
	}
	if !found {
		return errors.New("label PDF font lacks explicit embedding metadata")
	}
	for r := rune(32); r <= 126; r++ {
		glyph, err := font.GlyphIndex(nil, r)
		if err != nil || glyph == 0 {
			return errors.Join(err, fmt.Errorf("label PDF font lacks required glyph U+%04X", r))
		}
	}
	return nil
}

func renderPlotLabelPDF(ctx context.Context, preview PlotLabelRegionalPreview, fonts plotLabelPDFFonts) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("label PDF requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fields, date, err := plotLabelPDFFields(preview)
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "pt", "Letter", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(date)
	pdf.SetModificationDate(date)
	for _, font := range []struct {
		data        []byte
		name, style string
	}{
		{fonts.regular, "ArialMT", ""}, {fonts.bold, "Arial-BoldMT", "B"},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data := bytes.Clone(font.data)
		if err := validatePlotLabelPDFFont(data, font.name); err != nil {
			return nil, err
		}
		pdf.AddUTF8FontFromBytes("VPROSystemArial", font.style, data)
		// Registration errors in this library become observable at selection.
		pdf.SetFont("VPROSystemArial", font.style, 6)
		if err := pdf.Error(); err != nil {
			return nil, fmt.Errorf("label PDF font registration: %w", err)
		}
	}
	pdf.AddPage()
	for _, field := range fields {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		style := ""
		if field.bold {
			style = "B"
		}
		pdf.SetFont("VPROSystemArial", style, field.size)
		width := pdf.GetStringWidth(field.text)
		if err := pdf.Error(); err != nil {
			return nil, err
		}
		if width > field.width/20 {
			return nil, fmt.Errorf("%s label PDF requires unqualified source clipping/wrapping; no partial output", field.name)
		}
		x := 36 + field.left/20
		if field.rightAligned {
			x += field.width/20 - width
		}
		pdf.Text(x, 36+field.top/20+field.size, field.text)
	}
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, fmt.Errorf("label PDF encoding: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
