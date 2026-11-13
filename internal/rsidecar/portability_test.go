package rsidecar

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestRSidecarPortableWorkRootSpaceInUsernameAndCaseInsensitivePath(t *testing.T) {
	// 1. Empty baseDir resolves under UserCacheDir/vpro or UserDataDir (never os.Getwd())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	runDir, tmpDir, cleanup, err := prepareIsolatedRunDir("")
	if err != nil {
		t.Fatalf("prepareIsolatedRunDir(\"\") failed: %v", err)
	}
	defer cleanup()
	if strings.HasPrefix(runDir, filepath.Join(cwd, rsidecarWorkSubdir)) {
		t.Fatalf("prepareIsolatedRunDir(\"\") defaulted to os.Getwd() %q: got %q", cwd, runDir)
	}
	if strings.ContainsAny(tmpDir, " \t") {
		t.Fatalf("expected space-free R TMPDIR, got %q", tmpDir)
	}

	// 2. When both WorkRootDir AND UserCacheDir contain spaces (e.g. 'Jane Doe'), R TMPDIR remains space-free
	spacedHome := filepath.Join(t.TempDir(), "Jane Doe")
	spacedCache := filepath.Join(spacedHome, "Cache Dir")
	spacedWork := filepath.Join(spacedHome, "Work Root")
	if err := os.MkdirAll(spacedCache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(spacedWork, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", spacedCache)
	runSpaced, tmpSpaced, cleanupSpaced, err := prepareIsolatedRunDir(spacedWork)
	if err != nil {
		t.Fatalf("prepareIsolatedRunDir(spacedWork) failed: %v", err)
	}
	if !strings.Contains(runSpaced, "Jane Doe") {
		t.Fatalf("expected runDir under spacedWork, got %q", runSpaced)
	}
	if strings.ContainsAny(tmpSpaced, " \t") {
		cleanupSpaced()
		t.Fatalf("R TMPDIR contained whitespace despite space-in-username cache dir: %q", tmpSpaced)
	}
	cleanupSpaced()
	if _, err := os.Stat(tmpSpaced); !os.IsNotExist(err) {
		t.Fatalf("expected space-free temp dir %q to be cleaned up", tmpSpaced)
	}

	// 3. Read-only ProjectDir fallback to DataDir/exports in resolveDefaultGuideExportPath & RenderQuartoGuide
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		roProjDir := filepath.Join(t.TempDir(), "readonly-share")
		if err := os.MkdirAll(roProjDir, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(roProjDir, 0o700) })

		writableDataDir := filepath.Join(t.TempDir(), "user-data")
		gotExport := resolveDefaultGuideExportPath(roProjDir, writableDataDir, "Sample_guide.html")
		wantExport := filepath.Join(writableDataDir, "exports", "Sample_guide.html")
		if gotExport != wantExport {
			t.Fatalf("resolveDefaultGuideExportPath on read-only ProjectDir = %q, want %q", gotExport, wantExport)
		}

		// Also verify end-to-end RenderQuartoGuide succeeds when ProjectDir/exports is read-only
		svc, state := vprotest.ContextServiceFixture(t)
		roExports := filepath.Join(filepath.Dir(state.ProjectPath), "exports")
		if err := os.MkdirAll(roExports, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(roExports, 0o700) })

		guideRes, err := RenderQuartoGuide(context.Background(), svc, state.ContextID, QuartoGuideRequest{
			GuideType: "bgc_unit",
		}, RuntimeOptions{DataDir: writableDataDir})
		if err != nil {
			t.Fatalf("RenderQuartoGuide with read-only ProjectDir/exports failed: %v", err)
		}
		if !strings.HasPrefix(guideRes.HTMLPath, filepath.Join(writableDataDir, "exports")) {
			t.Fatalf("expected HTMLPath under writableDataDir/exports, got %q", guideRes.HTMLPath)
		}
	}
}
