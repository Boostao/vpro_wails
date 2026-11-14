package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	lifecyclepkg "github.com/boostao/vpro-wails/internal/lifecycle"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func resolveSpatialExtraDirs(lookup func(string) (string, bool), configured ...string) []string {
	var candidates []string
	if lookup != nil {
		if envDir, ok := lookup("VPRO_SPATIAL_ASSETS_DIR"); ok && strings.TrimSpace(envDir) != "" {
			for _, item := range filepath.SplitList(envDir) {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					candidates = append(candidates, trimmed)
				}
			}
		}
	}
	for _, raw := range configured {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			candidates = append(candidates, trimmed)
		}
	}
	if repoRoot := resources.RepoRoot(); strings.TrimSpace(repoRoot) != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(repoRoot), "spatial_assets"))
	}
	if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
		resolvedExe := exePath
		if eval, evalErr := filepath.EvalSymlinks(exePath); evalErr == nil && eval != "" {
			resolvedExe = eval
		}
		exeDir := filepath.Dir(resolvedExe)
		candidates = append(candidates,
			filepath.Join(exeDir, "maps"),
			filepath.Join(exeDir, "..", "Resources", "maps"),
			filepath.Join(exeDir, "..", "share", "vpro", "maps"),
		)
	}
	seen := make(map[string]bool, len(candidates))
	var out []string
	for _, dir := range candidates {
		cleaned := filepath.Clean(dir)
		if cleaned == "" || cleaned == "." || seen[cleaned] {
			continue
		}
		if info, err := os.Stat(cleaned); err == nil && info.IsDir() {
			seen[cleaned] = true
			out = append(out, cleaned)
		}
	}
	return out
}

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dataDir, configDir, windows, err := resolveHostEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	refs := openReferenceCatalogBundle(dataDir)
	var app *application.App
	closeService, err := lifecyclepkg.NewCloseService(
		func(nonce string) bool { return app.Event.Emit(lifecyclepkg.CloseRequestEvent, nonce) },
		func() { app.Quit() },
	)
	if err != nil {
		log.Fatal(err)
	}

	cloudSyncManager := NewCloudSyncManager(configDir)
	lifecycleService := NewLifecycleService(nil, closeService, nil)
	contextService := &ContextService{cloud: cloudSyncManager}
	plotEntryService := &PlotEntryService{}
	datasheetService := &DatasheetService{}
	classificationService := &ClassificationService{bec: refs.bec}
	validationService := &ValidationService{}
	importExportService := &ImportExportService{}
	vegetationReportService := &VegetationReportService{}
	environmentReportService := &EnvironmentReportService{dataDir: dataDir}

	spatialExtraDirs := resolveSpatialExtraDirs(os.LookupEnv)
	initialAttributor := NewPMTilesAttributor(dataDir, spatialExtraDirs...)
	spatialService := &SpatialService{dataDir: dataDir, extraDirs: spatialExtraDirs, attributor: initialAttributor}
	pictures := &PictureService{}
	helpService := &HelpService{}
	runtimeTracker := &runtimeShutdownTracker{attributor: initialAttributor, extraDirs: spatialExtraDirs}

	activateRuntime := func() (StartupState, error) {
		active, err := openActiveProjectContext(dataDir, configDir, refs, cloudSyncManager)
		if err != nil {
			return StartupState{}, err
		}
		plots := active.plots
		*contextService = *active.contextSvc

		var workingUnits *vprocorepkg.WorkingUnitService
		abort := func(cause error) (StartupState, error) {
			if workingUnits != nil {
				_ = workingUnits.Close()
			}
			contextService.core = nil
			_ = active.projects.CloseSQLiteContext()
			return StartupState{}, cause
		}

		builtPlotEntry, err := buildPlotEntryService(contextService, plots, refs)
		if err != nil {
			return abort(err)
		}
		builtPictures, err := buildPictureService(contextService, dataDir, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		builtDatasheet, err := buildDatasheetService(contextService)
		if err != nil {
			return abort(err)
		}
		builtImportExport, err := buildImportExportService(contextService)
		if err != nil {
			return abort(err)
		}
		builtVegReport, err := buildVegetationReportService(contextService)
		if err != nil {
			return abort(err)
		}
		builtEnvReport, err := buildEnvironmentReportService(contextService, dataDir)
		if err != nil {
			return abort(err)
		}
		userLog, err := lifecyclepkg.NewUserLogService(unwrapContextService(contextService))
		if err != nil {
			return abort(err)
		}
		if workingUnits, err = vprocorepkg.NewWorkingUnitService(active.projects, configDir); err != nil {
			log.Printf("Warning: failed to initialize WorkingUnitService: %v", err)
		}
		builtClassification, err := buildClassificationService(contextService, refs.bec, workingUnits)
		if err != nil {
			return abort(err)
		}
		builtHelp, err := buildHelpService(active.preferences, contextService, dataDir)
		if err != nil {
			return abort(err)
		}
		effectiveExtraDirs := resolveSpatialExtraDirs(os.LookupEnv, active.configuredSpatialDir)
		nextAttributor := NewPMTilesAttributor(dataDir, effectiveExtraDirs...)
		builtSpatial, err := buildSpatialService(contextService, configDir, active.preferences, dataDir, effectiveExtraDirs, nextAttributor)
		if err != nil {
			if nextAttributor != nil {
				_ = nextAttributor.Close()
			}
			return abort(err)
		}

		lifecycleService.userLog = userLog
		*plotEntryService = builtPlotEntry
		*datasheetService = *builtDatasheet
		*classificationService = *builtClassification
		*validationService = *NewValidationService(contextService)
		*importExportService = *builtImportExport
		*vegetationReportService = builtVegReport
		*environmentReportService = builtEnvReport
		*spatialService = builtSpatial
		*pictures = *builtPictures
		*helpService = *builtHelp

		runtimeTracker.swap(active.projects, workingUnits, nextAttributor, effectiveExtraDirs)
		nextState := StartupState{Ready: true, ConfigPath: active.preferences.Path, ReleaseChannel: active.releaseChannel}
		lifecyclepkg.PopulateRecoveryHints(&nextState)
		return nextState, nil
	}

	lifecycleService.startup = newStartupServiceWithInitialActivation(dataDir, configDir, activateRuntime)

	services := []application.Service{
		application.NewService(lifecycleService),
		application.NewService(contextService),
		application.NewService(plotEntryService),
		application.NewService(datasheetService),
		application.NewService(classificationService),
		application.NewService(validationService),
		application.NewService(importExportService),
		application.NewService(vegetationReportService),
		application.NewService(environmentReportService),
		application.NewService(spatialService),
		application.NewService(pictures),
		application.NewService(helpService),
	}

	app = application.New(application.Options{
		Name:        "VPro",
		Description: "VENUS (Vegetation and Environment NexUS) Professional — B.C. Ministry of Forests, Research Branch (BEC Program)",
		Services:    services,
		Windows:     windows,
		ShouldQuit:  closeService.ShouldQuit,
		OnShutdown:  func() { runtimeTracker.shutdown(refs, cloudSyncManager) },
		Assets: application.AssetOptions{
			Handler: NewDynamicPMTilesAssetHandler(application.AssetFileServerFS(assets), dataDir, runtimeTracker.currentExtraDirs),
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "main", Title: "VPro", Width: 1400, Height: 900, MinWidth: 560, MinHeight: 520,
		BackgroundColour: application.NewRGB(247, 249, 247), URL: "/",
	})
	var layoutOnce sync.Once
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		layoutOnce.Do(func() {
			screen := app.Screen.GetPrimary()
			if screen == nil {
				log.Fatal("Cannot size the application: primary display is unavailable")
			}
			dimensions, err := vprocorepkg.InitialWindowDimensions(screen.WorkArea.Width, screen.WorkArea.Height)
			if err != nil {
				log.Fatal(err)
			}
			window.SetMinSize(dimensions.MinWidth, dimensions.MinHeight)
			window.SetSize(dimensions.Width, dimensions.Height)
			window.Center()
		})
	})
	window.RegisterHook(events.Common.WindowClosing, closeService.HandleWindowClosing)
	if err = app.Run(); err != nil {
		log.Fatal(err)
	}
}
