package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/sivi"
	"github.com/boostao/vpro-wails/internal/spatial"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
	"github.com/boostao/vpro-wails/resources"
	"gopkg.in/yaml.v3"
)

type goListPackage struct {
	ImportPath   string   `json:"ImportPath"`
	Imports      []string `json:"Imports"`
	TestImports  []string `json:"TestImports"`
	XTestImports []string `json:"XTestImports"`
}

func TestFourLayerAcyclicArchitectureZeroLayer2SiblingImports(t *testing.T) {
	const modulePrefix = "github.com/boostao/vpro-wails/"

	// All Layer-2 domain packages (covering the 12 domain service packages plus sub-domain Layer-2 packages).
	// None of these packages may import any sibling Layer-2 package across Imports, TestImports, or XTestImports.
	layer2Packages := map[string]bool{
		modulePrefix + "internal/classification": true,
		modulePrefix + "internal/cloudsync":      true,
		modulePrefix + "internal/datasheet":      true,
		modulePrefix + "internal/diagnostics":    true,
		modulePrefix + "internal/envreport":      true,
		modulePrefix + "internal/interchange":    true,
		modulePrefix + "internal/lifecycle":      true,
		modulePrefix + "internal/lmh25rules":     true,
		modulePrefix + "internal/lumping":        true,
		modulePrefix + "internal/pictures":       true,
		modulePrefix + "internal/plotentry":      true,
		modulePrefix + "internal/projectops":     true,
		modulePrefix + "internal/rsidecar":       true,
		modulePrefix + "internal/sivi":           true,
		modulePrefix + "internal/spatial":        true,
		modulePrefix + "internal/twopage":        true,
		modulePrefix + "internal/validation":     true,
		modulePrefix + "internal/vegreport":      true,
	}

	cmd := exec.Command("go", "list", "-tags", "sqliteonly", "-json", "./internal/...")
	cmd.Dir = resources.RepoRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list ./internal/... failed: %v\n%s", err, string(out))
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	seenLayer2 := map[string]bool{}
	var violations []string

	for {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list JSON: %v", err)
		}
		if !layer2Packages[pkg.ImportPath] {
			continue
		}
		seenLayer2[pkg.ImportPath] = true

		checkEdgeList := func(kind string, imports []string) {
			for _, imp := range imports {
				if imp == pkg.ImportPath {
					// XTestImports (package foo_test) legitimately imports its own package foo.
					continue
				}
				if layer2Packages[imp] || imp == modulePrefix+"internal/integration" {
					violations = append(violations,
						strings.TrimPrefix(pkg.ImportPath, modulePrefix)+" ("+kind+") -> "+strings.TrimPrefix(imp, modulePrefix))
				}
			}
		}

		checkEdgeList("Imports", pkg.Imports)
		checkEdgeList("TestImports", pkg.TestImports)
		checkEdgeList("XTestImports", pkg.XTestImports)
	}

	if len(seenLayer2) < 12 {
		t.Fatalf("expected at least 12 Layer-2 domain packages from go list, saw %d", len(seenLayer2))
	}
	if len(violations) > 0 {
		t.Fatalf("Layer-2 sibling import violations detected:\n  %s", strings.Join(violations, "\n  "))
	}
}

func TestCrossDomainFeatureGateAndPreferenceIsolation(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// 1. Enabling TableCSVReview must NOT enable SIVI parent review.
	service.TableCSVReviewEnabled = true
	if result, err := sivi.GetSIVIParentOriginal(service, ctx, state.ContextID, "108050"); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("table CSV opt-in enabled SIVI review: result=%+v err=%v", result, err)
	}

	// 2. Enabling PlotLocationReview must NOT enable TableCSVReview or SIVI parent review.
	service.TableCSVReviewEnabled = false
	service.PlotLocationReviewEnabled = true
	if got, err := interchange.GetProjectTableCSVReview(service, ctx, state.ContextID, "Sample_Env"); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("plot location opt-in enabled table CSV review: got=%+v err=%v", got, err)
	}
	if got, err := sivi.GetSIVIParentOriginal(service, ctx, state.ContextID, "108050"); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("plot location opt-in enabled SIVI review: got=%+v err=%v", got, err)
	}

	// 3. Shared report string preference CAS reads and validates all typed keys (e.g. GoogleEarthDescField) before CAS mismatch.
	data, config := vprotest.ConfigFixture(t)
	store, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}
	values, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	values["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = nil
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(raw); err != nil {
		t.Fatal(err)
	}
	result, err := spatial.CompareAndSetGoogleEarthPreferences(store, ctx,
		spatial.GoogleEarthPreferences{PlaceName: "stale", DescriptionField: "expected"},
		spatial.GoogleEarthPreferences{PlaceName: "new", DescriptionField: "Zone"},
	)
	if err == nil || !strings.Contains(err.Error(), "GoogleEarthDescField") || result.Committed {
		t.Fatalf("shared extraction changed strict decode/CAS error precedence: result=%+v err=%v", result, err)
	}
}
