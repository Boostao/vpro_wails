package main

import (
	"context"
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

func newDefaultPictureEnvLookup(base func(string) (string, bool), defaultVPicsPath, defaultImagesDir string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		if base != nil {
			if val, ok := base(key); ok {
				return val, true
			}
		}
		switch key {
		case "VPRO_PICTURE_LIBRARY_PATH":
			if defaultVPicsPath != "" {
				return defaultVPicsPath, true
			}
		case "VPRO_PICTURE_CHILD_DIRECTORY", "VPRO_PICTURE_MANAGER_DIRECTORY":
			if defaultImagesDir != "" {
				return defaultImagesDir, true
			}
		}
		return "", false
	}
}

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dataDir, err := userDataDir()
	if err != nil {
		log.Fatal(err)
	}
	configDir, err := userConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	rawDebugPort := os.Getenv("VPRO_WEBVIEW_DEBUG_PORT")
	if rawDebugPort != "" && lifecyclepkg.CurrentBuildInfo().BuildMode != lifecyclepkg.BuildModeDev {
		log.Printf("Ignoring VPRO_WEBVIEW_DEBUG_PORT in non-dev build (%s)", lifecyclepkg.CurrentBuildInfo().BuildMode)
	}
	windows, err := nativeWindowsOptions(rawDebugPort, dataDir, configDir)
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
	siteCodesInitErr := err
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
	parentCodesInitErr := err
	if err != nil {
		log.Printf("Warning: failed to initialize ParentCodeService: %v", err)
	}

	var app *application.App
	closeService, err := NewCloseService(
		func(nonce string) bool { return app.Event.Emit(closeRequestEvent, nonce) },
		func() { app.Quit() },
	)
	if err != nil {
		log.Fatal(err)
	}

	startupService := &StartupService{
		DataDir: dataDir,
		State: StartupState{
			ConfigPath:     filepath.Join(configDir, "config.yml"),
			ReleaseChannel: ReleaseChannelStable,
		},
	}
	cloudSyncManager := NewCloudSyncManager(configDir)
	lifecycleService := NewLifecycleService(startupService, closeService, nil)
	contextService := &ContextService{cloud: cloudSyncManager}
	plotEntryService := &PlotEntryService{}
	datasheetService := &DatasheetService{}
	classificationService := &ClassificationService{bec: becService}
	validationService := &ValidationService{}
	importExportService := &ImportExportService{}
	vegetationReportService := &VegetationReportService{}
	environmentReportService := &EnvironmentReportService{dataDir: dataDir}

	spatialExtraDirs := resolveSpatialExtraDirs(os.LookupEnv)
	initialAttributor := NewPMTilesAttributor(dataDir, spatialExtraDirs...)
	spatialService := &SpatialService{
		dataDir:    dataDir,
		extraDirs:  spatialExtraDirs,
		attributor: initialAttributor,
	}
	pictures := &PictureService{}
	helpService := &HelpService{}

	var (
		shutdownMu       sync.Mutex
		activeProjects   *ProjectService
		activeWorking    *WorkingUnitService
		activeAttributor = initialAttributor
		activeExtraDirs  = spatialExtraDirs
	)

	activateRuntime := func() (StartupState, error) {
		preferences, err := openDesktopConfig(dataDir, configDir)
		if err != nil {
			return StartupState{}, err
		}
		releaseChannel, err := preferences.ReleaseChannel()
		if err != nil {
			return StartupState{}, err
		}
		projects, err := newSQLiteProjectService(dataDir, configDir, preferences)
		if err != nil {
			return StartupState{}, err
		}
		if err := projects.EnsureActiveProjectWAL(context.Background()); err != nil {
			log.Printf("Warning: failed to initialize active project WAL mode: %v", err)
		}
		configuredSpatialDir := ""
		if snap, snapErr := preferences.Snapshot(); snapErr == nil {
			if spSec, secErr := ConfigSection(snap, "Spatial"); secErr == nil {
				if rawDir, ok := spSec["AssetsDir"].(string); ok {
					configuredSpatialDir = rawDir
				}
			}
		}

		plots, err := newPlotServiceWithPreferences(projects)
		if err != nil {
			_ = projects.CloseSQLiteContext()
			return StartupState{}, err
		}
		plots.SiteCodes, plots.SiteCodesError = siteCodes, siteCodesInitErr
		plots.ParentCodes, plots.ParentCodesError = parentCodes, parentCodesInitErr

		builtContext, err := NewContextService(projects, plots)
		if err != nil {
			_ = projects.CloseSQLiteContext()
			return StartupState{}, err
		}
		builtContext.cloud = cloudSyncManager
		*contextService = *builtContext

		abort := func(cause error) (StartupState, error) {
			contextService.core = nil
			_ = projects.CloseSQLiteContext()
			return StartupState{}, cause
		}

		plotLabelRegional, err := NewPlotLabelRegionalService(contextService)
		if err != nil {
			return abort(err)
		}
		siviParentShared, err := NewSIVIParentSharedService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageParentExtra, err := NewTwoPageParentExtraService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageParentCommon, err := NewTwoPageParentCommonService(contextService)
		if err != nil {
			return abort(err)
		}
		siviCovers, err := NewSIVICoverService(contextService)
		if err != nil {
			return abort(err)
		}
		siviCombined, err := NewSIVICombinedService(contextService)
		if err != nil {
			return abort(err)
		}
		siviCollected, err := NewSIVICollectedService(contextService)
		if err != nil {
			return abort(err)
		}
		siviSpecies, err := NewSIVISpeciesService(contextService)
		if err != nil {
			return abort(err)
		}
		siviIdentity, err := NewSIVIIdentityService(contextService)
		if err != nil {
			return abort(err)
		}
		siviCreation, err := NewSIVICreationService(contextService)
		if err != nil {
			return abort(err)
		}
		siviCreationUndo, err := NewSIVICreationUndoService(contextService)
		if err != nil {
			return abort(err)
		}
		siviDeletion, err := NewSIVIDeletionService(contextService)
		if err != nil {
			return abort(err)
		}
		defaultVPicsPath, defaultImagesDir, err := vprocorepkg.EnsureDefaultPictureLibrary(dataDir)
		if err != nil {
			return abort(err)
		}
		pictureLookup := newDefaultPictureEnvLookup(os.LookupEnv, defaultVPicsPath, defaultImagesDir)
		builtPictures, err := NewPictureService(contextService, pictureLookup)
		if err != nil {
			return abort(err)
		}
		pictureMetadata, err := NewPictureMetadataService(builtPictures)
		if err != nil {
			return abort(err)
		}
		pictureCreation, err := NewPictureCreationService(builtPictures)
		if err != nil {
			return abort(err)
		}
		pictureDeletion, err := NewPictureDeletionService(builtPictures)
		if err != nil {
			return abort(err)
		}
		builtPictures.Metadata = pictureMetadata
		builtPictures.Creation = pictureCreation
		builtPictures.Deletion = pictureDeletion

		summaryVegetation, err := NewSummaryVegetationService(contextService)
		if err != nil {
			return abort(err)
		}
		summaryEnvironment, err := NewSummaryVegetationEnvironmentalService(summaryVegetation)
		if err != nil {
			return abort(err)
		}
		summaryEnvironmentLifeform, err := NewSummaryVegetationEnvironmentalLifeformService(summaryEnvironment)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook, err := NewSummaryVegetationWorkbookService(summaryEnvironmentLifeform)
		if err != nil {
			return abort(err)
		}
		lifeformSummary, err := NewLifeformSummaryService(contextService)
		if err != nil {
			return abort(err)
		}
		speciesAttributeSummary, err := NewSpeciesAttributeSummaryService(contextService)
		if err != nil {
			return abort(err)
		}
		lifeformWorkbook, err := NewLifeformWorkbookService(contextService)
		if err != nil {
			return abort(err)
		}
		subzoneMatrix, err := NewSubzoneMatrixService(contextService)
		if err != nil {
			return abort(err)
		}
		siteUnitSummaryWorkbook, err := NewSiteUnitSummaryWorkbookService(contextService)
		if err != nil {
			return abort(err)
		}
		siteUnitSummaryExtendedWorkbook, err := NewSiteUnitSummaryExtendedWorkbookService(contextService)
		if err != nil {
			return abort(err)
		}
		environmentWorkbook, err := NewEnvironmentWorkbookService(contextService)
		if err != nil {
			return abort(err)
		}
		vegetationWorkbook, err := NewVegetationWorkbookService(contextService)
		if err != nil {
			return abort(err)
		}
		coordinates, err := newCoordinateService(configDir, preferences)
		if err != nil {
			return abort(err)
		}
		siviParentShared.References = siviParentSharedReferenceReaders{
			Site: siteCodes, Region: regionCodes, Parent: parentCodes,
			Geology: geologyCodes, Bec: becService,
		}
		entryReaders := twoPageEntryReferenceReaders{}
		if siteCodes != nil {
			entryReaders.Shared.Site = siteCodes
		}
		if regionCodes != nil {
			entryReaders.Shared.Region = regionCodes
			entryReaders.Ecosection = regionCodes
		}
		if parentCodes != nil {
			entryReaders.Shared.Parent = parentCodes
		}
		if geologyCodes != nil {
			entryReaders.Shared.Geology = geologyCodes
		}
		if becService != nil {
			entryReaders.Shared.Bec = becService
			entryReaders.Series = becService
		}
		if qualityService != nil {
			entryReaders.Quality = qualityService
		}
		if soilCodes != nil {
			entryReaders.Soil = soilCodes
		}
		twoPageParentEntry, err := NewTwoPageParentEntryService(contextService, entryReaders)
		if err != nil {
			return abort(err)
		}
		twoPageSoil, err := NewTwoPageSoilService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageSourceAudit, err := NewTwoPageSourceAuditService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageGoogleEarth, err := NewTwoPageGoogleEarthService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageVegetationTable, err := NewTwoPageVegetationTableService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageEnvironmentTable, err := NewTwoPageEnvironmentTableService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageProjectMetadata, err := NewTwoPageProjectMetadataService(contextService)
		if err != nil {
			return abort(err)
		}
		userLog, err := NewUserLogService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageProfiling, err := NewTwoPageProfilingService(contextService)
		if err != nil {
			return abort(err)
		}
		profileSpeciesPresence, err := NewProjectProfileSpeciesPresenceService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageVegetation, err := NewTwoPageVegetationService(contextService)
		if err != nil {
			return abort(err)
		}
		twoPageVegetationSpeciesList, err := NewTwoPageVegetationSpeciesListService(contextService)
		if err != nil {
			return abort(err)
		}
		workingUnits, err := NewWorkingUnitService(projects, configDir)
		if err != nil {
			log.Printf("Warning: failed to initialize WorkingUnitService: %v", err)
		}
		builtHelp, err := NewHelpService(preferences)
		if err != nil {
			if workingUnits != nil {
				_ = workingUnits.Close()
			}
			return abort(err)
		}
		builtHelp.setContext(contextService)
		builtHelp.setDataDir(dataDir)

		lifecycleService.userLog = userLog
		*plotEntryService = PlotEntryService{
			contexts:                     contextService,
			plots:                        plots,
			siviParentShared:             siviParentShared,
			siviCovers:                   siviCovers,
			siviCombined:                 siviCombined,
			siviCollected:                siviCollected,
			siviSpecies:                  siviSpecies,
			siviIdentity:                 siviIdentity,
			siviCreation:                 siviCreation,
			siviCreationUndo:             siviCreationUndo,
			siviDeletion:                 siviDeletion,
			twoPageParentExtra:           twoPageParentExtra,
			twoPageParentCommon:          twoPageParentCommon,
			twoPageParentEntry:           twoPageParentEntry,
			twoPageSoil:                  twoPageSoil,
			twoPageVegetation:            twoPageVegetation,
			twoPageVegetationSpeciesList: twoPageVegetationSpeciesList,
			twoPageSourceAudit:           twoPageSourceAudit,
			twoPageProjectMetadata:       twoPageProjectMetadata,
			references:                   refService,
			siteCodes:                    siteCodes,
			soilCodes:                    soilCodes,
			parentCodes:                  parentCodes,
			geologyCodes:                 geologyCodes,
			regionCodes:                  regionCodes,
			quality:                      qualityService,
		}
		*datasheetService = *NewDatasheetService(twoPageEnvironmentTable, twoPageVegetationTable)
		*classificationService = *NewClassificationService(
			contextService,
			becService,
			workingUnits,
			NewExternalSUEnvironmentService(contextService),
			twoPageProfiling,
			profileSpeciesPresence,
		)
		*validationService = *NewValidationService(contextService)
		*importExportService = *NewImportExportService(
			contextService,
			NewTableCSVArchiveService(contextService),
			plotLabelRegional,
			NewPlotLabelPDFExportService(contextService),
		)
		*vegetationReportService = VegetationReportService{
			contexts:                       contextService,
			summary:                        summaryVegetation,
			summaryEnvironment:             summaryEnvironment,
			summaryEnvironmentLifeform:     summaryEnvironmentLifeform,
			summaryWorkbook:                summaryVegetationWorkbook,
			summaryPreferences:             NewSummaryVegetationPreferencesService(contextService),
			summaryQualityPreferences:      NewSummaryVegetationQualityPreferencesService(contextService),
			summaryPresentationPreferences: NewSummaryVegetationPresentationPreferencesService(contextService),
			summaryFieldList:               NewSummaryVegetationFieldListService(contextService),
			longPreferences:                NewLongVegetationPreferencesService(contextService),
			longStatisticsPreferences:      NewLongVegetationStatisticsPreferencesService(contextService),
			longSwitchPreferences:          NewLongVegetationSwitchPreferencesService(contextService),
			longThresholdPreferences:       NewLongVegetationThresholdPreferencesService(contextService),
			longGroupPreferences:           NewLongVegetationGroupPreferencesService(contextService),
			longQualityNullPreferences:     NewLongVegetationQualityNullPreferencesService(contextService),
			longQualityMinimumPreferences:  NewLongVegetationQualityMinimumPreferencesService(contextService),
			longNamingPreferences:          NewLongVegetationNamingPreferencesService(contextService),
			longSelectedFieldPreferences:   NewLongVegetationSelectedFieldPreferencesService(contextService),
			longEnforcementPreferences:     NewLongVegetationEnforcementPreferencesService(contextService),
			longUnitPreferences:            NewLongVegetationUnitPreferencesService(contextService),
			longSelectedUnitSelection:      NewLongVegetationSelectedUnitSelectionService(contextService),
			longReportScope:                NewLongVegetationReportScopeService(contextService),
			longProjectWorkbook:            NewLongVegetationProjectWorkbookService(contextService),
			vegetationWorkbook:             vegetationWorkbook,
			lifeformSummary:                lifeformSummary,
			lifeformWorkbook:               lifeformWorkbook,
			speciesAttributeSummary:        speciesAttributeSummary,
		}
		*environmentReportService = EnvironmentReportService{
			contexts:                        contextService,
			environmentWorkbook:             environmentWorkbook,
			longEnvironmentPreferences:      NewLongEnvironmentPreferencesService(contextService),
			siteUnitSummaryPreferences:      NewSiteUnitSummaryPreferencesService(contextService),
			siteUnitSummaryWorkbook:         siteUnitSummaryWorkbook,
			siteUnitSummaryExtendedWorkbook: siteUnitSummaryExtendedWorkbook,
			subzoneMatrix:                   subzoneMatrix,
			dataDir:                         dataDir,
		}
		effectiveExtraDirs := resolveSpatialExtraDirs(os.LookupEnv, configuredSpatialDir)
		nextAttributor := NewPMTilesAttributor(dataDir, effectiveExtraDirs...)
		*spatialService = SpatialService{
			contexts:               contextService,
			coordinates:            coordinates,
			googleEarthReview:      NewGoogleEarthReviewService(contextService),
			googleEarthKML:         NewGoogleEarthKMLService(contextService),
			googleEarthKMLExport:   NewGoogleEarthKMLExportService(contextService),
			googleEarthPreferences: NewGoogleEarthPreferencesService(contextService),
			twoPageGoogleEarth:     twoPageGoogleEarth,
			dataDir:                dataDir,
			extraDirs:              effectiveExtraDirs,
			attributor:             nextAttributor,
		}
		*pictures = *builtPictures
		*helpService = *builtHelp

		shutdownMu.Lock()
		prevProjects := activeProjects
		prevWorking := activeWorking
		prevAttributor := activeAttributor
		activeProjects = projects
		activeWorking = workingUnits
		activeAttributor = nextAttributor
		activeExtraDirs = append([]string(nil), effectiveExtraDirs...)
		shutdownMu.Unlock()
		if prevAttributor != nil && prevAttributor != nextAttributor {
			_ = prevAttributor.Close()
		}
		if prevWorking != nil && prevWorking != workingUnits {
			_ = prevWorking.Close()
		}
		if prevProjects != nil && prevProjects != projects {
			_ = prevProjects.CloseSQLiteContext()
		}

		nextState := StartupState{
			Ready:          true,
			ConfigPath:     preferences.Path,
			ReleaseChannel: releaseChannel,
		}
		PopulateStartupRecoveryHints(&nextState)
		return nextState, nil
	}

	startupService.Activate = activateRuntime
	if initialState, initErr := activateRuntime(); initErr != nil {
		log.Printf("Saved context remains unavailable (entering startup recovery): %v", initErr)
		recoveryState := StartupState{
			Ready:          false,
			Message:        initErr.Error(),
			ConfigPath:     filepath.Join(configDir, "config.yml"),
			ReleaseChannel: ReleaseChannelStable,
		}
		PopulateStartupRecoveryHints(&recoveryState)
		startupService.SetState(recoveryState)
	} else {
		startupService.SetState(initialState)
	}

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
		OnShutdown: func() {
			shutdownMu.Lock()
			projToClose := activeProjects
			wuToClose := activeWorking
			attrToClose := activeAttributor
			activeProjects = nil
			activeWorking = nil
			activeAttributor = nil
			shutdownMu.Unlock()

			if wuToClose != nil {
				if err := wuToClose.Close(); err != nil {
					log.Printf("Warning: failed to close WorkingUnitService: %v", err)
				}
			}
			if projToClose != nil {
				if err := projToClose.CloseSQLiteContext(); err != nil {
					log.Printf("Warning: failed to close project context: %v", err)
				}
			}
			if refService != nil {
				if err := refService.Close(); err != nil {
					log.Printf("Warning: failed to close ReferenceService: %v", err)
				}
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
			if regionCodes != nil {
				if err := regionCodes.Close(); err != nil {
					log.Printf("Warning: failed to close RegionCodeService: %v", err)
				}
			}
			if siteCodes != nil {
				if err := siteCodes.Close(); err != nil {
					log.Printf("Warning: failed to close SiteCodeService: %v", err)
				}
			}
			if becService != nil {
				if err := becService.Close(); err != nil {
					log.Printf("Warning: failed to close BECService: %v", err)
				}
			}
			if attrToClose != nil {
				if err := attrToClose.Close(); err != nil {
					log.Printf("Warning: failed to close PMTilesAttributor: %v", err)
				}
			}
			if cloudSyncManager != nil {
				cloudSyncManager.Close()
			}
		},
		Assets: application.AssetOptions{
			Handler: NewDynamicPMTilesAssetHandler(application.AssetFileServerFS(assets), dataDir, func() []string {
				shutdownMu.Lock()
				defer shutdownMu.Unlock()
				return append([]string(nil), activeExtraDirs...)
			}),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "VPro",
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
			window.SetMinSize(dimensions.MinWidth, dimensions.MinHeight)
			window.SetSize(dimensions.Width, dimensions.Height)
			window.Center()
		})
	})
	window.RegisterHook(events.Common.WindowClosing, closeService.HandleWindowClosing)
	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
