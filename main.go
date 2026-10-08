package main

import (
	"embed"
	"log"
	"os"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	siviHeightEnabled, err := siviHeightFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviParentReviewEnabled, err := siviParentReviewFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviParentEditingEnabled, err := siviParentEditingFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviParentActionEditingEnabled, err := siviParentActionEditingFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviProjectAssignmentEnabled, err := siviProjectAssignmentFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	tableCSVReviewEnabled, err := tableCSVReviewFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	tableCSVArchiveEnabled, err := tableCSVArchiveFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	plotLocationReviewEnabled, err := plotLocationReviewFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	googleEarthReviewEnabled, err := googleEarthReviewFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	googleEarthKMLEnabled, err := googleEarthKMLFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	googleEarthKMLExportEnabled, err := googleEarthKMLExportFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	googleEarthPreferencesEnabled, err := googleEarthPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longEnvironmentPreferencesEnabled, err := longEnvironmentPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	dataDir, err := userDataDir()
	if err != nil {
		log.Fatal(err)
	}
	configDir, err := userConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	windows, err := nativeWindowsOptions(os.Getenv("VPRO_WEBVIEW_DEBUG_PORT"), configDir)
	if err != nil {
		log.Fatal(err)
	}
	preferences, err := openDesktopConfig(dataDir, configDir)
	if err != nil {
		runStartupRecovery(windows, configDir, err)
		return
	}
	projects, err := newSQLiteProjectService(dataDir, configDir, preferences)
	if err != nil {
		runStartupRecovery(windows, configDir, err)
		return
	}
	plots, err := newPlotServiceWithPreferences(projects)
	if err != nil {
		log.Fatal(err)
	}
	contextService, err := NewContextService(projects, plots)
	if err != nil {
		log.Fatal(err)
	}
	contextService.siviHeightEnabled = siviHeightEnabled
	contextService.siviParentReviewEnabled = siviParentReviewEnabled
	contextService.siviParentEditingEnabled = siviParentEditingEnabled
	contextService.siviParentActionEditingEnabled = siviParentActionEditingEnabled
	contextService.siviProjectAssignmentEnabled = siviProjectAssignmentEnabled
	contextService.tableCSVReviewEnabled = tableCSVReviewEnabled
	contextService.plotLocationReviewEnabled = plotLocationReviewEnabled
	coordinates, err := newCoordinateService(configDir, preferences)
	if err != nil {
		log.Fatal(err)
	}
	refService, err := NewReferenceService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize ReferenceService: %v", err)
	}
	becService, err := NewBECService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize BECService: %v", err)
	}
	qualityService, err := NewQualityService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize QualityService: %v", err)
	}
	siteCodes, err := NewSiteCodeService(dataDir)
	plots.siteCodes, plots.siteCodesError = siteCodes, err
	if err != nil {
		log.Printf("Warning: failed to initialize SiteCodeService: %v", err)
	}
	regionCodes, err := NewRegionCodeService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize RegionCodeService: %v", err)
	}
	soilCodes, err := NewSoilCodeService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize SoilCodeService: %v", err)
	}
	geologyCodes, err := NewGeologyCodeService(dataDir)
	if err != nil {
		log.Printf("Warning: failed to initialize GeologyCodeService: %v", err)
	}
	parentCodes, err := NewParentCodeService(dataDir)
	plots.parentCodes, plots.parentCodesError = parentCodes, err
	if err != nil {
		log.Printf("Warning: failed to initialize ParentCodeService: %v", err)
	}
	workingUnits, err := NewWorkingUnitService(projects, configDir)
	if err != nil {
		log.Printf("Warning: failed to initialize WorkingUnitService: %v", err)
	}

	services := []application.Service{
		application.NewService(&StartupService{state: StartupState{Ready: true, ConfigPath: preferences.path}}),
		application.NewService(projects),
		application.NewService(plots),
		application.NewService(contextService),
		application.NewService(NewGoogleEarthReviewService(contextService, googleEarthReviewEnabled)),
		application.NewService(NewGoogleEarthKMLService(contextService, googleEarthKMLEnabled)),
		application.NewService(NewGoogleEarthKMLExportService(contextService, googleEarthKMLExportEnabled)),
		application.NewService(NewGoogleEarthPreferencesService(contextService, googleEarthPreferencesEnabled)),
		application.NewService(NewLongEnvironmentPreferencesService(contextService, longEnvironmentPreferencesEnabled)),
		application.NewService(NewTableCSVArchiveService(contextService, tableCSVArchiveEnabled)),
		application.NewService(coordinates),
	}
	if refService != nil {
		services = append(services, application.NewService(refService))
	}
	if becService != nil {
		services = append(services, application.NewService(becService))
	}
	if qualityService != nil {
		services = append(services, application.NewService(qualityService))
	}
	if siteCodes != nil {
		services = append(services, application.NewService(siteCodes))
	}
	if regionCodes != nil {
		services = append(services, application.NewService(regionCodes))
	}
	if soilCodes != nil {
		services = append(services, application.NewService(soilCodes))
	}
	if geologyCodes != nil {
		services = append(services, application.NewService(geologyCodes))
	}
	if parentCodes != nil {
		services = append(services, application.NewService(parentCodes))
	}
	if workingUnits != nil {
		services = append(services, application.NewService(workingUnits))
	}

	var app *application.App
	closeService, err := NewCloseService(
		func(nonce string) bool { return app.Event.Emit(closeRequestEvent, nonce) },
		func() { app.Quit() },
	)
	if err != nil {
		log.Fatal(err)
	}
	services = append(services, application.NewService(closeService))

	app = application.New(application.Options{
		Name:        "VPRO",
		Description: "Vegetation Processor",
		Services:    services,
		Windows:     windows,
		ShouldQuit:  closeService.shouldQuit,
		OnShutdown: func() {
			if err := projects.closeSQLiteContext(); err != nil {
				log.Printf("Warning: failed to close project context: %v", err)
			}
			if parentCodes != nil {
				if err := parentCodes.Close(); err != nil {
					log.Printf("Warning: failed to close ParentCodeService: %v", err)
				}
			}
			if geologyCodes != nil {
				if err := geologyCodes.Close(); err != nil {
					log.Printf("Warning: failed to close GeologyCodeService: %v", err)
				}
			}
			if soilCodes != nil {
				if err := soilCodes.Close(); err != nil {
					log.Printf("Warning: failed to close SoilCodeService: %v", err)
				}
			}
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "VPRO",
		Width:            1400,
		Height:           900,
		MinWidth:         560,
		MinHeight:        520,
		BackgroundColour: application.NewRGB(247, 249, 247),
		URL:              "/",
	})
	var layoutOnce sync.Once
	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(_ *application.WindowEvent) {
		layoutOnce.Do(func() {
			screen := app.Screen.GetPrimary()
			if screen == nil {
				log.Fatal("Cannot size the application: primary display is unavailable")
			}
			dimensions, err := initialWindowDimensions(screen.WorkArea.Width, screen.WorkArea.Height)
			if err != nil {
				log.Fatal(err)
			}
			window.SetMinSize(dimensions.minWidth, dimensions.minHeight)
			window.SetSize(dimensions.width, dimensions.height)
			window.Center()
		})
	})
	window.RegisterHook(events.Common.WindowClosing, closeService.handleWindowClosing)
	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
