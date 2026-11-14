package vegreport

import (
	"context"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestSummaryVegetation2026BCPlantTaxonomyCodetypeXX(t *testing.T) {
	ctx := context.Background()

	// 1. Direct layer preparation with 2-character Codetype = "XX" in USysAllSpecs.
	raw, su, master, personal, layers := summaryRawLayerFixture()
	master.Rows = append(master.Rows, vprocore.ProjectMetadataRow{
		RowID: "3",
		Cells: vprotest.LifeformReferenceRow("ANEMRIC", "Anemonastrum richardsonii", vprotest.MetadataInteger("5"), vprotest.MetadataText("yellow anemone"), vprotest.MetadataText("XX")),
	})
	if _, err := prepareSummaryVegetationLayer(ctx, "Project", "Selected", raw, su, master, personal, layers, 1); err != nil {
		t.Fatalf("prepareSummaryVegetationLayer rejected 2-char Codetype 'XX': %v", err)
	}

	// Overlength master Codetype (> 255 UTF-16 units) must still be rejected.
	overMaster := master
	overMaster.Rows = append([]vprocore.ProjectMetadataRow{}, master.Rows...)
	overMaster.Rows[len(overMaster.Rows)-1] = vprocore.ProjectMetadataRow{
		RowID: "4",
		Cells: vprotest.LifeformReferenceRow("ANEMRIC", "Anemonastrum richardsonii", vprotest.MetadataInteger("5"), vprotest.MetadataText("yellow anemone"), vprotest.MetadataText(strings.Repeat("X", 256))),
	}
	if _, err := prepareSummaryVegetationLayer(ctx, "Project", "Selected", raw, su, overMaster, personal, layers, 1); err == nil {
		t.Fatal("expected overlength >255 UTF-16 Codetype to be rejected")
	}

	// 2. End-to-end SummaryVegetationService.Preview against unmodified bundled 2026 VLists.db
	// (BCPlantTaxonomy v19 containing 43 rows with Codetype = 'XX' plus 9-char non-synonym codes).
	contexts, state := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		CREATE TABLE BundledVLists_SU (PlotNumber VARCHAR, SiteUnit VARCHAR);
		CREATE UNIQUE INDEX uidx_BundledVLists_SU_PlotNumber ON BundledVLists_SU(PlotNumber);
		INSERT INTO _table_metadata (table_name, description) VALUES ('BundledVLists_SU', 'VP04');
		INSERT INTO BundledVLists_SU (PlotNumber, SiteUnit) VALUES
			('108050',  'MHmm1/01'),
			('108050x', 'MHmm1/01'),
			('Cb13011', 'SBSdk/01');
	`)
	sel := vprotest.ContextSelection(state)
	sel.SU = "BundledVLists"
	sel.SUPath = state.ProjectPath
	state, err := contexts.SwitchContext(state.ContextID, sel)
	if err != nil {
		t.Fatalf("SwitchContext: %v", err)
	}

	service, err := NewSummaryVegetationService(contexts)
	if err != nil {
		t.Fatalf("NewSummaryVegetationService: %v", err)
	}

	preview, err := service.Preview(ctx, state.ContextID, 1, 6)
	if err != nil {
		t.Fatalf("SummaryVegetationService.Preview failed against bundled 2026 VLists.db: %v", err)
	}
	if len(preview.Units) != 2 || len(preview.Rows) == 0 {
		t.Fatalf("expected 2 units and non-empty rows against bundled 2026 VLists.db, got units=%d rows=%d", len(preview.Units), len(preview.Rows))
	}

	grouped, err := service.PreviewGrouped(ctx, state.ContextID, 1, 13)
	if err != nil {
		t.Fatalf("SummaryVegetationService.PreviewGrouped failed against bundled 2026 VLists.db: %v", err)
	}
	if len(grouped.Units) != 2 || len(grouped.Rows) == 0 {
		t.Fatalf("expected 2 grouped units and non-empty rows, got units=%d rows=%d", len(grouped.Units), len(grouped.Rows))
	}
}
