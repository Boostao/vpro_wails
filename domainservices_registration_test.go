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

	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
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
	startupSvc := &StartupService{State: StartupState{
		Ready:          true,
		ConfigPath:     `C:\config\config.yml`,
		ReleaseChannel: ReleaseChannelStable,
	}}
	lifecycle := NewLifecycleService(startupSvc, closeSvc, nil)
	state := lifecycle.GetStartupState()
	if !state.Ready || state.ReleaseChannel != ReleaseChannelStable {
		t.Fatalf("unexpected startup state: %+v", state)
	}
	if closeSvc.ShouldQuit() {
		t.Fatal("close should remain blocked until pending nonce is confirmed")
	}
	pending := lifecycle.GetPendingCloseRequest()
	if pending == "" || pending != emitted {
		t.Fatalf("expected emitted nonce %q to match pending %q", emitted, pending)
	}
	if err := lifecycle.CancelClose("forged"); !errors.Is(err, ErrInvalidCloseRequest) {
		t.Fatalf("forged nonce must be rejected, got %v", err)
	}
	if err := lifecycle.CancelClose(pending); err != nil || lifecycle.GetPendingCloseRequest() != "" {
		t.Fatalf("CancelClose failed: %v", err)
	}
	if closeSvc.ShouldQuit() {
		t.Fatal("close should mint a fresh nonce after cancellation")
	}
	next := lifecycle.GetPendingCloseRequest()
	if next == "" || next == pending {
		t.Fatalf("expected fresh nonce after cancel, got %q vs %q", next, pending)
	}
	if err := lifecycle.ConfirmClose(next); err != nil || quitCalled != 1 || !closeSvc.ShouldQuit() {
		t.Fatalf("ConfirmClose did not approve native quit: err=%v quitCalled=%d", err, quitCalled)
	}
}

func TestColdBootPictureLibraryWiringAndNilPictureServiceGuards(t *testing.T) {
	ctx := context.Background()

	// 1. Nil receiver guards on *PictureService must not panic!
	var nilPic *PictureService
	if _, err := nilPic.GetMetadata(ctx, "cid", "00337"); err == nil {
		t.Fatal("expected nil PictureService.GetMetadata to return error without panic")
	}
	if _, err := nilPic.GetImage(ctx, "cid", "00337", "{}"); err == nil {
		t.Fatal("expected nil PictureService.GetImage to return error without panic")
	}

	// 2. Cold boot EnsureDefaultPictureLibrary + newDefaultPictureEnvLookup enables all 4 picture services out-of-the-box!
	dataDir := t.TempDir()
	configDir := t.TempDir()
	preferences, err := openDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := newSQLiteProjectService(dataDir, configDir, preferences)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if projects.Sqlite != nil {
			_ = projects.Sqlite.Close()
		}
	})
	plots, err := newPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	contextSvc, err := NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	state, err := projects.GetState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defaultVPicsPath, defaultImagesDir, err := vprocorepkg.EnsureDefaultPictureLibrary(dataDir)
	if err != nil {
		t.Fatalf("EnsureDefaultPictureLibrary failed: %v", err)
	}
	emptyEnv := func(string) (string, bool) { return "", false }
	picLookup := newDefaultPictureEnvLookup(emptyEnv, defaultVPicsPath, defaultImagesDir)

	picSvc, err := NewPictureService(contextSvc, picLookup)
	if err != nil {
		t.Fatalf("NewPictureService failed on cold boot: %v", err)
	}
	picMeta, err := NewPictureMetadataService(picSvc, picLookup)
	if err != nil {
		t.Fatalf("NewPictureMetadataService failed on cold boot: %v", err)
	}
	picCreate, err := NewPictureCreationService(picSvc, picLookup)
	if err != nil {
		t.Fatalf("NewPictureCreationService failed on cold boot: %v", err)
	}
	picDelete, err := NewPictureDeletionService(picSvc, picLookup)
	if err != nil {
		t.Fatalf("NewPictureDeletionService failed on cold boot: %v", err)
	}
	picSvc.Metadata = picMeta
	picSvc.Creation = picCreate
	picSvc.Deletion = picDelete

	review, err := picSvc.GetMetadata(ctx, state.ContextID, "00337")
	if err != nil {
		t.Fatalf("GetMetadata on cold-boot seeded VPics.db failed: %v", err)
	}
	if review == nil || len(review.Records.Columns) != 5 || len(review.Records.Rows) != 0 {
		t.Fatalf("unexpected initial VPics metadata review: %+v", review)
	}

	// Explicit env override must take precedence over defaults.
	overrideLookup := newDefaultPictureEnvLookup(func(key string) (string, bool) {
		if key == "VPRO_PICTURE_READING" {
			return "false", true
		}
		return "", false
	}, defaultVPicsPath, defaultImagesDir)
	disabledPicSvc, err := NewPictureService(contextSvc, overrideLookup)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := disabledPicSvc.GetMetadata(ctx, state.ContextID, "00337"); err == nil {
		t.Fatal("expected explicit VPRO_PICTURE_READING=false env override to disable reading")
	}
}

func TestContextServiceOmitsFuncParametersAndForwardsAllVProCoreRPCs(t *testing.T) {
	mainType := reflect.TypeOf(&ContextService{})
	coreType := reflect.TypeOf(&vprocorepkg.ContextService{})

	// 1. main.ContextService MUST NOT expose Edit or any method with reflect.Func parameters/returns!
	if _, hasEdit := mainType.MethodByName("Edit"); hasEdit {
		t.Fatal("main.ContextService must not expose Edit(string, func(*PlotService) error) error to Wails bindings")
	}
	for i := 0; i < mainType.NumMethod(); i++ {
		m := mainType.Method(i)
		mt := m.Type
		for p := 1; p < mt.NumIn(); p++ {
			if mt.In(p).Kind() == reflect.Func {
				t.Fatalf("main.ContextService.%s parameter %d is a func (%s), which triggers Wails JSON binding warning", m.Name, p, mt.In(p))
			}
		}
		for r := 0; r < mt.NumOut(); r++ {
			if mt.Out(r).Kind() == reflect.Func {
				t.Fatalf("main.ContextService.%s return %d is a func (%s)", m.Name, r, mt.Out(r))
			}
		}
	}

	// 2. Every exported method on *vprocore.ContextService EXCEPT Edit MUST exist on *main.ContextService!
	for i := 0; i < coreType.NumMethod(); i++ {
		cm := coreType.Method(i)
		if cm.Name == "Edit" {
			continue
		}
		mm, ok := mainType.MethodByName(cm.Name)
		if !ok {
			t.Fatalf("main.ContextService is missing forwarded RPC method %s from vprocore.ContextService", cm.Name)
		}
		if mm.Type.NumIn() != cm.Type.NumIn() || mm.Type.NumOut() != cm.Type.NumOut() {
			t.Fatalf("main.ContextService.%s arity (%d in, %d out) differs from vprocore.ContextService.%s (%d in, %d out)",
				cm.Name, mm.Type.NumIn(), mm.Type.NumOut(), cm.Name, cm.Type.NumIn(), cm.Type.NumOut())
		}
	}
}

func TestSpatialExtraDirsNoHardcodedHomePath(t *testing.T) {
	rawMain, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawMain), "/home/bruno") {
		t.Fatal("main.go must not contain hardcoded developer path /home/bruno")
	}

	existingDir := t.TempDir()
	missingDir := filepath.Join(t.TempDir(), "does_not_exist")

	dirs := resolveSpatialExtraDirs(func(key string) (string, bool) {
		if key == "VPRO_SPATIAL_ASSETS_DIR" {
			return existingDir, true
		}
		return "", false
	})
	found := false
	for _, d := range dirs {
		if d == existingDir {
			found = true
		}
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Fatalf("resolveSpatialExtraDirs included non-existent directory %q: %v", d, err)
		}
	}
	if !found {
		t.Fatalf("expected existing VPRO_SPATIAL_ASSETS_DIR %q in %v", existingDir, dirs)
	}

	dirsMissing := resolveSpatialExtraDirs(func(key string) (string, bool) {
		if key == "VPRO_SPATIAL_ASSETS_DIR" {
			return missingDir, true
		}
		return "", false
	})
	for _, d := range dirsMissing {
		if d == missingDir {
			t.Fatalf("resolveSpatialExtraDirs must omit non-existent directory %q", missingDir)
		}
	}
}


