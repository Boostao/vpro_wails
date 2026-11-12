package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMainRegistersStrictlyTwelveCohesiveDomainServices(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var registered []string
	var appendCalls int
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
			if ident, ok := assign.Lhs[0].(*ast.Ident); ok && ident.Name == "services" {
				if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
					if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "append" {
						appendCalls++
					}
				}
				if comp, ok := assign.Rhs[0].(*ast.CompositeLit); ok {
					for _, elt := range comp.Elts {
						call, ok := elt.(*ast.CallExpr)
						if !ok || len(call.Args) != 1 {
							t.Fatalf("unexpected service registration element: %T", elt)
						}
						arg, ok := call.Args[0].(*ast.Ident)
						if !ok {
							t.Fatalf("service registration argument must be named domain service variable, got %T", call.Args[0])
						}
						registered = append(registered, arg.Name)
					}
				}
			}
		}
		return true
	})
	want := []string{
		"lifecycleService",
		"contextService",
		"plotEntryService",
		"datasheetService",
		"classificationService",
		"validationService",
		"importExportService",
		"vegetationReportService",
		"environmentReportService",
		"spatialService",
		"pictures",
		"helpService",
	}
	if appendCalls != 0 || len(registered) != 12 || !reflect.DeepEqual(registered, want) {
		t.Fatalf("main.go must register strictly 12 cohesive domain services; got len=%d appendCalls=%d registered=%v", len(registered), appendCalls, registered)
	}
}

func TestHelpServiceProvides2026LibraryStampsAboutAndReleaseChannel(t *testing.T) {
	dataDir := t.TempDir()
	configDir := t.TempDir()
	preferences, err := openDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	help, err := NewHelpService(preferences)
	if err != nil {
		t.Fatal(err)
	}
	stamps, err := help.GetLibraryStamps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stamps.SchemaVersion != 1 ||
		!strings.HasPrefix(stamps.BECVersion, "BECdb_v13_2026") ||
		len(stamps.BECArchiveSHA256) != 64 ||
		len(stamps.BECExtractedSHA256) != 64 ||
		!strings.HasPrefix(stamps.TaxonomyVersion, "BCPlantTaxa_v19_2025") ||
		len(stamps.TaxonomyArchiveSHA256) != 64 ||
		len(stamps.TaxonomyExtractedSHA256) != 64 ||
		stamps.LMH25Edition != "LMH25 (2023)" ||
		len(stamps.LMH25PDFSHA256) != 64 ||
		len(stamps.Artifacts) != 7 {
		t.Fatalf("unexpected 2026 library stamps: %+v", stamps)
	}
	stamps.Sources[0].Version = "mutated"
	stamps.Artifacts[0].Counts["MasterSiteUnitList"] = -1
	fresh, err := help.GetLibraryStamps(context.Background())
	if err != nil || fresh.Sources[0].Version == "mutated" || fresh.Artifacts[0].Counts["MasterSiteUnitList"] != 6067 {
		t.Fatalf("GetLibraryStamps aliased internal slice/map state: %+v %v", fresh, err)
	}

	channel, err := help.GetReleaseChannel(context.Background())
	if err != nil || channel != ReleaseChannelStable {
		t.Fatalf("expected default stable release channel, got %q (%v)", channel, err)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(raw), "release_channel: stable", "release_channel: experimental", 1)
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	about, err := help.GetAboutInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if about.ProductName != "VPro" || about.SchemaVersion != "VP08" || about.ReleaseChannel != ReleaseChannelExperimental ||
		!strings.Contains(about.Organization, "Ministry of Forests") {
		t.Fatalf("unexpected AboutInfo: %+v", about)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := help.GetLibraryStamps(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on GetLibraryStamps, got %v", err)
	}
	if _, err := help.GetReleaseChannel(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on GetReleaseChannel, got %v", err)
	}
}

func TestLifecycleServiceConsolidatesStartupAndCloseHandshake(t *testing.T) {
	var emitted string
	var quitCalled int
	closeSvc, err := NewCloseService(
		func(nonce string) bool { emitted = nonce; return false },
		func() { quitCalled++ },
	)
	if err != nil {
		t.Fatal(err)
	}
	startupSvc := &StartupService{state: StartupState{
		Ready:          true,
		ConfigPath:     `C:\config\config.yml`,
		ReleaseChannel: ReleaseChannelStable,
	}}
	lifecycle := NewLifecycleService(startupSvc, closeSvc, nil)
	state := lifecycle.GetStartupState()
	if !state.Ready || state.ReleaseChannel != ReleaseChannelStable {
		t.Fatalf("unexpected startup state: %+v", state)
	}
	if closeSvc.shouldQuit() {
		t.Fatal("close should remain blocked until pending nonce is confirmed")
	}
	pending := lifecycle.GetPendingCloseRequest()
	if pending == "" || pending != emitted {
		t.Fatalf("expected emitted nonce %q to match pending %q", emitted, pending)
	}
	if err := lifecycle.CancelClose("forged"); !errors.Is(err, errInvalidCloseRequest) {
		t.Fatalf("forged nonce must be rejected, got %v", err)
	}
	if err := lifecycle.CancelClose(pending); err != nil || lifecycle.GetPendingCloseRequest() != "" {
		t.Fatalf("CancelClose failed: %v", err)
	}
	if closeSvc.shouldQuit() {
		t.Fatal("close should mint a fresh nonce after cancellation")
	}
	next := lifecycle.GetPendingCloseRequest()
	if next == "" || next == pending {
		t.Fatalf("expected fresh nonce after cancel, got %q vs %q", next, pending)
	}
	if err := lifecycle.ConfirmClose(next); err != nil || quitCalled != 1 || !closeSvc.shouldQuit() {
		t.Fatalf("ConfirmClose did not approve native quit: err=%v quitCalled=%d", err, quitCalled)
	}
}
