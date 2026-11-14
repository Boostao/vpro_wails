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

	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
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
	preferences, err := vprocorepkg.OpenDesktopConfig(dataDir, configDir)
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
	if err != nil || channel != vprocorepkg.ReleaseChannelStable {
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
	if about.ProductName != "VPro" || about.SchemaVersion != "VP08" || about.ReleaseChannel != vprocorepkg.ReleaseChannelExperimental ||
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
	closeSvc, err := lifecyclepkg.NewCloseService(
		func(nonce string) bool { emitted = nonce; return false },
		func() { quitCalled++ },
	)
	if err != nil {
		t.Fatal(err)
	}
	startupSvc := &lifecyclepkg.StartupService{State: StartupState{
		Ready:          true,
		ConfigPath:     `C:\config\config.yml`,
		ReleaseChannel: vprocorepkg.ReleaseChannelStable,
	}}
	lifecycle := NewLifecycleService(startupSvc, closeSvc, nil)
	state := lifecycle.GetStartupState()
	if !state.Ready || state.ReleaseChannel != vprocorepkg.ReleaseChannelStable {
		t.Fatalf("unexpected startup state: %+v", state)
	}
	if closeSvc.ShouldQuit() {
		t.Fatal("close should remain blocked until pending nonce is confirmed")
	}
	pending := lifecycle.GetPendingCloseRequest()
	if pending == "" || pending != emitted {
		t.Fatalf("expected emitted nonce %q to match pending %q", emitted, pending)
	}
	if err := lifecycle.CancelClose("forged"); !errors.Is(err, lifecyclepkg.ErrInvalidCloseRequest) {
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

func TestImportAccessProjectRenamedAuditAndChildLoadingViaPlotEntryService(t *testing.T) {
	candidates := []struct {
		paths         []string
		sourceProject string
		targetProject string
	}{
		{
			paths:         []string{`/home/bruno/Work/VPRO_ACCESS_1/VPro64/Haida_Gwaii_2019.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\Haida_Gwaii_2019.accdb`},
			sourceProject: "Haida_Gwaii_VPro19_2019",
			targetProject: "Haida2019",
		},
		{
			paths:         []string{`/home/bruno/Work/VPRO_ACCESS/VPro64/VPro64.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\VPro64.accdb`},
			sourceProject: "Sample",
			targetProject: "SampleRenamed",
		},
	}

	var chosenPath, sourceProject, targetProject string
	for _, cand := range candidates {
		for _, p := range cand.paths {
			if _, err := os.Stat(p); err == nil {
				chosenPath = p
				sourceProject = cand.sourceProject
				targetProject = cand.targetProject
				break
			}
		}
		if chosenPath != "" {
			break
		}
	}
	if chosenPath == "" {
		t.Skip("no sibling Access .accdb fixture available on this host")
	}

	dataDir := t.TempDir()
	configDir := t.TempDir()
	prefs, err := vprocorepkg.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := vprocorepkg.NewSQLiteProjectService(dataDir, configDir, prefs)
	if err != nil {
		t.Fatal(err)
	}
	defer projects.CloseSQLiteContext()
	plots, err := vprocorepkg.NewPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	contextSvc, err := NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	importExportSvc := NewImportExportService(contextSvc, nil, nil, nil)
	plotEntrySvc := &PlotEntryService{
		contexts: contextSvc,
		plots:    plots,
	}

	ctx := context.Background()
	initState, err := contextSvc.GetState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(dataDir, "projects", targetProject+".db")
	impRes, err := importExportSvc.ImportAccessProject(ctx, initState.ContextID, AccessProjectImportRequest{
		SourcePath:        chosenPath,
		SourceProject:     sourceProject,
		TargetProject:     targetProject,
		DestinationPath:   destPath,
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatalf("ImportAccessProject(%s -> %s) failed: %v", sourceProject, targetProject, err)
	}
	if !impRes.Activated || impRes.State == nil || impRes.State.ActiveProject != targetProject {
		t.Fatalf("expected activated %s state, got %+v", targetProject, impRes)
	}
	activeCtxID := impRes.State.ContextID

	page, err := contextSvc.ListPlots(ctx, 0, 50)
	if err != nil || len(page.Plots) == 0 {
		t.Fatalf("ListPlots on renamed project failed: page=%+v err=%v", page, err)
	}

	totalAuditRows := 0
	for _, summary := range page.Plots {
		pNum := summary.PlotNumber
		if _, err := plotEntrySvc.GetPlot(pNum); err != nil {
			t.Fatalf("PlotEntryService.GetPlot(%s) failed: %v", pNum, err)
		}
		if _, err := plotEntrySvc.ListVegRecords(pNum); err != nil {
			t.Fatalf("PlotEntryService.ListVegRecords(%s) failed: %v", pNum, err)
		}
		if _, err := plotEntrySvc.ListHumusRecords(pNum); err != nil {
			t.Fatalf("PlotEntryService.ListHumusRecords(%s) failed: %v", pNum, err)
		}
		if _, err := plotEntrySvc.ListMineralRecords(pNum); err != nil {
			t.Fatalf("PlotEntryService.ListMineralRecords(%s) failed: %v", pNum, err)
		}
		if _, err := plotEntrySvc.ListOtherRecords(pNum); err != nil {
			t.Fatalf("PlotEntryService.ListOtherRecords(%s) failed: %v", pNum, err)
		}
		auditEntries, err := plotEntrySvc.ListAuditReadEntries(pNum)
		if err != nil {
			t.Fatalf("PlotEntryService.ListAuditReadEntries(%s) failed: %v", pNum, err)
		}
		for _, entry := range auditEntries {
			if entry.Project != targetProject {
				t.Fatalf("audit row %s for plot %s retained foreign Project=%q instead of %q", entry.RowID, pNum, entry.Project, targetProject)
			}
		}
		trail, err := plotEntrySvc.ListAuditEntries(pNum)
		if err != nil {
			t.Fatalf("PlotEntryService.ListAuditEntries(%s) failed: %v", pNum, err)
		}
		totalAuditRows += len(trail)
	}
	if totalAuditRows == 0 {
		t.Fatalf("expected non-zero audit trail rows across %s plots to exercise project ownership normalization", targetProject)
	}

	// Verify editing and saving an imported plot under renamed TargetProject succeeds and records new audit rows
	firstPlot := page.Plots[0].PlotNumber
	hdr, err := plotEntrySvc.GetPlot(firstPlot)
	if err != nil {
		t.Fatal(err)
	}
	note := "Verified Wave 6.1 renamed import edit"
	hdr.OfficeNotes = &note
	if err := contextSvc.UpdatePlot(activeCtxID, *hdr); err != nil {
		t.Fatalf("UpdatePlot on renamed imported plot %s failed: %v", firstPlot, err)
	}
	afterAudits, err := contextSvc.ListAuditReadEntries(ctx, activeCtxID, firstPlot)
	if err != nil || len(afterAudits) == 0 {
		t.Fatalf("expected audit entries after UpdatePlot on %s, got len=%d err=%v", firstPlot, len(afterAudits), err)
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
	preferences, err := vprocorepkg.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := vprocorepkg.NewSQLiteProjectService(dataDir, configDir, preferences)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if projects.Sqlite != nil {
			_ = projects.Sqlite.Close()
		}
	})
	plots, err := vprocorepkg.NewPlotServiceWithPreferences(projects)
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
	picMeta, err := NewPictureMetadataService(picSvc)
	if err != nil {
		t.Fatalf("NewPictureMetadataService failed on cold boot: %v", err)
	}
	picCreate, err := NewPictureCreationService(picSvc)
	if err != nil {
		t.Fatalf("NewPictureCreationService failed on cold boot: %v", err)
	}
	picDelete, err := NewPictureDeletionService(picSvc)
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

	// Explicit directory/path env override must take precedence over defaults.
	missingLibrary := filepath.Join(t.TempDir(), "missing-vpics.db")
	overrideLookup := newDefaultPictureEnvLookup(func(key string) (string, bool) {
		if key == "VPRO_PICTURE_LIBRARY_PATH" {
			return missingLibrary, true
		}
		return "", false
	}, defaultVPicsPath, defaultImagesDir)
	if _, err := NewPictureService(contextSvc, overrideLookup); err == nil {
		t.Fatal("expected explicit VPRO_PICTURE_LIBRARY_PATH env override to take precedence over default")
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
