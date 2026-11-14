package interchange

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestPlotLabelRegionalAndPDFCrossPlatformFallbackAndHardlinkUnsupported(t *testing.T) {
	contexts, state := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["project"],
		`INSERT INTO Sample_Env(PlotNumber,Zone,SubZone,SiteSeries,PlotRepresenting,ProjectID)
		 VALUES('PT001','SBS','mk1','01','Prince George Plot','BEC2026')`)
	vprotest.MutateContextFixture(t, contexts.Projects.Sqlite.Attachments["project"],
		`INSERT INTO Sample_Admin(Plot) VALUES('PT001')`)

	// 1. Regional preview works cross-platform without stubbing format() or requiring Windows
	regSvc, err := NewPlotLabelRegionalService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	regPreview, err := regSvc.GetPreview(context.Background(), state.ContextID, "PT001")
	if err != nil || regPreview == nil || regPreview.LabelDate == "" {
		t.Fatalf("cross-platform GetPlotLabelRegionalPreview failed: %+v (%v)", regPreview, err)
	}

	// 2. Plot Label PDF review + export works cross-platform with embedded LiberationSans fallback when Arial is absent
	pdfSvc := NewPlotLabelPDFExportService(contexts)
	revReq, _ := json.Marshal(PlotLabelPDFReviewRequest{PlotNumber: "PT001"})
	review, err := pdfSvc.GetReview(context.Background(), state.ContextID, string(revReq))
	if err != nil || review == nil || len(review.PDFSHA256) != 64 {
		t.Fatalf("cross-platform PlotLabelPDFExportService.GetReview failed: %+v (%v)", review, err)
	}

	destPDF := filepath.Join(t.TempDir(), "PT001-label.pdf")
	expReq, _ := json.Marshal(PlotLabelPDFExportRequest{
		PlotNumber:   "PT001",
		ApprovalHash: review.ApprovalHash,
		Destination:  destPDF,
		RequestID:    "11111111-2222-4333-8444-555555555555",
		SessionID:    review.SessionID,
	})
	outcome, err := pdfSvc.ExportReviewed(context.Background(), state.ContextID, string(expReq))
	if err != nil || outcome == nil || outcome.Status != "published" {
		t.Fatalf("cross-platform ExportReviewed failed: %+v (%v)", outcome, err)
	}
	pdfBytes, err := os.ReadFile(destPDF)
	if err != nil || !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("published PDF missing or invalid header: %v", err)
	}

	// 3. ArtifactPublicationChecked succeeds when os.Link fails with EOPNOTSUPP (exFAT/FAT32/SMB simulation)
	prep, err := ReadPlotLabelPDFPreparation(contexts, context.Background(), state.ContextID, "PT001", plotLabelPDFRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	exfatDest := filepath.Join(t.TempDir(), "PT001-exfat.pdf")
	pubRes, err := PublishOwnedPlotLabelPDF(contexts, context.Background(), state.ContextID, *prep, exfatDest, plotLabelPDFPublicationHooks{
		publication: vprocore.ArtifactPublicationHooks{
			Link: func(oldname, newname string) error {
				return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EOPNOTSUPP}
			},
		},
	})
	if err != nil || !pubRes.Published {
		t.Fatalf("PublishOwnedPlotLabelPDF failed on EOPNOTSUPP fallback: %+v (%v)", pubRes, err)
	}
	exfatBytes, err := os.ReadFile(exfatDest)
	if err != nil || !bytes.Equal(exfatBytes, pdfBytes) {
		t.Fatal("exFAT fallback PDF bytes mismatch")
	}
}
