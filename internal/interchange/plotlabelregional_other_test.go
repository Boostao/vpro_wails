//go:build !windows

package interchange

import (
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestPlotLabelNativeDateCrossPlatformPreviewAndCalendarBounds(t *testing.T) {
	contexts, state := vprotest.ContextServiceFixture(t)

	vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["project"], `
		INSERT INTO Sample_Env(PlotNumber,Zone,SubZone,SiteSeries,PlotRepresenting,ProjectID)
		VALUES('LBX01','CWH','vm1','01','  cross-platform plot  ','PID-LNX');
		INSERT INTO Sample_Admin(Plot) VALUES('LBX01');
	`)

	service, err := NewPlotLabelRegionalService(contexts)
	if err != nil {
		t.Fatalf("NewPlotLabelRegionalService: %v", err)
	}
	// Do NOT stub service.format — exercise real non-Windows formatPlotLabelNativeDate!
	service.now = func() time.Time { return time.Date(2026, time.November, 12, 14, 30, 0, 0, time.Local) }

	preview, err := service.GetPreview(t.Context(), state.ContextID, "LBX01")
	if err != nil || preview == nil {
		t.Fatalf("GetPreview failed on non-Windows platform: preview=%+v err=%v", preview, err)
	}
	if preview.LabelDate != "Thursday, November 12, 2026" ||
		preview.ISODate != "2026-11-12" ||
		preview.Locale != "en-US" ||
		preview.Pattern != "dddd, MMMM d, yyyy" ||
		preview.Source != "windows-user-long-date" {
		t.Fatalf("unexpected non-Windows PlotLabelRegionalPreview: %+v", preview)
	}

	// Verify plotLabelPDFFields accepts the native non-Windows preview cleanly!
	fields, parsedDate, err := plotLabelPDFFields(*preview)
	if err != nil || len(fields) != 6 || parsedDate.Format("2006-01-02") != "2026-11-12" {
		t.Fatalf("plotLabelPDFFields rejected non-Windows preview: fields=%+v err=%v", fields, err)
	}

	// Validate Windows SYSTEMTIME calendar bounds 1601..9999 on non-Windows as well.
	for _, validYear := range []int{1601, 2026, 9999} {
		got, err := formatPlotLabelNativeDate(time.Date(validYear, time.January, 2, 0, 0, 0, 0, time.UTC))
		if err != nil || got.text == "" || got.locale != "en-US" || got.pattern != "dddd, MMMM d, yyyy" {
			t.Fatalf("year %d should succeed, got %+v (%v)", validYear, got, err)
		}
	}
	for _, invalidYear := range []int{0, 1600, 10000, -50} {
		if got, err := formatPlotLabelNativeDate(time.Date(invalidYear, time.January, 2, 0, 0, 0, 0, time.UTC)); err == nil || got != (plotLabelNativeDate{}) {
			t.Fatalf("out-of-range year %d should be rejected, got %+v", invalidYear, got)
		}
	}
}
