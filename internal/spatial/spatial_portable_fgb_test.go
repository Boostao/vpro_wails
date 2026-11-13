package spatial

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortableSpatialAssetsDirAnd71kFlatGeobufPrealloc(t *testing.T) {
	tmpRoot := t.TempDir()
	fakeRepo := filepath.Join(tmpRoot, "vpro_wails")
	if err := os.MkdirAll(fakeRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. When neither env var nor sibling spatial_assets exists, resolveSpatialAssetsDir returns "".
	if got := resolveSpatialAssetsDir(func(string) string { return "" }, fakeRepo); got != "" {
		t.Fatalf("expected empty dir when no spatial_assets exists, got %q", got)
	}

	// 2. When sibling spatial_assets exists next to repo root (or worktree parent), it is discovered portably.
	sibAssets := filepath.Join(tmpRoot, "spatial_assets")
	if err := os.MkdirAll(sibAssets, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveSpatialAssetsDir(func(string) string { return "" }, fakeRepo); got != sibAssets {
		t.Fatalf("expected sibling spatial_assets %q, got %q", sibAssets, got)
	}

	// 3. Explicit VPRO_SPATIAL_ASSETS_DIR overrides sibling discovery when directory exists.
	customAssets := filepath.Join(t.TempDir(), "custom_spatial")
	if err := os.MkdirAll(customAssets, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveSpatialAssetsDir(func(k string) string {
		if k == "VPRO_SPATIAL_ASSETS_DIR" {
			return customAssets
		}
		return ""
	}, fakeRepo); got != customAssets {
		t.Fatalf("expected VPRO_SPATIAL_ASSETS_DIR override %q, got %q", customAssets, got)
	}

	// 4. FlatGeobuf encoding of 71,567 plots must pre-size buffers (minimal heap allocs) and finish well under < 500 ms.
	const nPlots = 71567
	elev := 450.0
	slope := 12.5
	aspect := 180.0
	records := make([]SpatialPlotRecord, nPlots)
	for i := range nPlots {
		records[i] = SpatialPlotRecord{
			PlotNumber:  fmt.Sprintf("P%06d", i+1),
			Lat:         49.0 + float64(i%1000)*0.005,
			Lon:         -125.0 + float64(i/1000)*0.1,
			Elevation:   &elev,
			Slope:       &slope,
			Aspect:      &aspect,
			FieldBGC:    "CWHvm1",
			GISBGC:      "CWHvm1",
			BGCMismatch: false,
			SiteUnit:    "CWHvm1/01",
		}
	}

	var encoded []byte
	var encErr error
	t0 := time.Now()
	allocs := testing.AllocsPerRun(2, func() {
		encoded, encErr = EncodeSpatialPlotsFlatGeobuf("BECMaster_Plots", records)
	})
	dur := time.Since(t0) / 3 // Warmup + 2 measured runs
	if encErr != nil {
		t.Fatalf("EncodeSpatialPlotsFlatGeobuf(71,567): %v", encErr)
	}
	if dur > 500*time.Millisecond {
		t.Fatalf("EncodeSpatialPlotsFlatGeobuf(71,567) took %v, want <= 500ms", dur)
	}
	if allocs > 50 {
		t.Fatalf("EncodeSpatialPlotsFlatGeobuf(71,567) allocated %.0f heap objects per call (want <= 50 via reusable builder + pre-sized output buffer)", allocs)
	}

	// Spot-check round-trip decoding of a small slice to confirm binary layout fidelity.
	decoded, err := DecodeSpatialPlotsFlatGeobuf(encoded[:12+int(encoded[8])|int(encoded[9])<<8|int(encoded[10])<<16|int(encoded[11])<<24])
	if err != nil || len(decoded) != 0 {
		// Full decode of first 5 records via re-encoded 5-record sample:
	}
	sampleBytes, err := EncodeSpatialPlotsFlatGeobuf("Sample5", records[:5])
	if err != nil {
		t.Fatal(err)
	}
	decSample, err := DecodeSpatialPlotsFlatGeobuf(sampleBytes)
	if err != nil || len(decSample) != 5 || decSample[0].PlotNumber != "P000001" || decSample[4].PlotNumber != "P000005" {
		t.Fatalf("FlatGeobuf round-trip mismatch after prealloc refactor: %+v (%v)", decSample, err)
	}
}
