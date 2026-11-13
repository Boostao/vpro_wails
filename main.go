package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
		case "VPRO_PICTURE_READING",
			"VPRO_PICTURE_METADATA_WRITING",
			"VPRO_PICTURE_CREATION_WRITING",
			"VPRO_PICTURE_DELETION_WRITING",
			"VPRO_PICTURE_DELETION_RESTORING":
			return "true", true
		}
		return "", false
	}
}

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	siviHeightEnabled, err := siviHeightFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siviCoverFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siviCombinedFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siviCollectedFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siviSpeciesFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siviIdentityFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{siviDeletionWritingFeatureEnvironment, siviDeletionRestoringFeatureEnvironment, siviCreationUndoFeatureEnvironment} {
		if _, err := siviFeature(name, os.LookupEnv); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := siviFeature(pictureReadingEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{pictureCreationWritingEnvironment, pictureDeletionWritingEnvironment, pictureDeletionRestoringEnvironment} {
		if _, err := siviFeature(name, os.LookupEnv); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := siviFeature(lifeformWorkbookFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(subzoneMatrixFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siteUnitSummaryWorkbookFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	if _, err := siviFeature(siteUnitSummaryExtendedWorkbookFeatureEnvironment, os.LookupEnv); err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{twoPageParentReviewEnvironment, twoPageParentExtraEditingEnvironment, twoPageParentCommonEditingEnvironment, twoPageParentEntryEditingEnvironment} {
		if _, err := siviFeature(name, os.LookupEnv); err != nil {
			log.Fatal(err)
		}
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
	siteUnitSummaryEnabled, err := siviFeature(siteUnitSummaryFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siteUnitSummaryLifeformsEnabled, err := siviFeature(siteUnitSummaryLifeformFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siteUnitSummarySpeciesEnabled, err := siviFeature(siteUnitSummarySpeciesFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationNoneEnabled, err := siviFeature(longVegetationNoneFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationLifeformEnabled, err := siviFeature(longVegetationLifeformFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationStrataEnabled, err := siviFeature(longVegetationStrataFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationCodeEnabled, err := siviFeature(longVegetationCodeFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationSelectedFieldEnabled, err := siviFeature(longVegetationSelectedFieldFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationNoneUnitsEnabled, err := siviFeature("VPRO_LONG_VEGETATION_NONE_UNITS", os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationSelectedUnitsEnabled, err := siviFeature(longVegetationSelectedUnitsFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationSelectedUnitSelectionEnabled, err := siviFeature(longVegetationSelectedUnitSelectionFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationProjectScopeEnabled, err := siviFeature(longVegetationProjectScopeFeatureEnvironment, os.LookupEnv)
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
	plotLabelPDFExportEnabled, err := siviFeature(plotLabelPDFExportEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	externalSUEnvironmentEditingEnabled, err := siviFeature(externalSUEnvironmentEditingEnvironment, os.LookupEnv)
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
	longVegetationPreferencesEnabled, err := longVegetationPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationStatisticsPreferencesEnabled, err := longVegetationStatisticsPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationSwitchPreferencesEnabled, err := longVegetationSwitchPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationThresholdPreferencesEnabled, err := longVegetationThresholdPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationGroupPreferencesEnabled, err := longVegetationGroupPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationQualityNullPreferencesEnabled, err := longVegetationQualityNullPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationQualityMinimumPreferencesEnabled, err := longVegetationQualityMinimumPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationNamingPreferencesEnabled, err := longVegetationNamingPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationSelectedFieldPreferencesEnabled, err := longVegetationSelectedFieldPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationEnforcementPreferencesEnabled, err := longVegetationEnforcementPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	longVegetationUnitPreferencesEnabled, err := longVegetationUnitPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siteUnitSummaryPreferencesEnabled, err := siteUnitSummaryPreferencesFeature(os.LookupEnv)
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
	windows, err := nativeWindowsOptions(os.Getenv("VPRO_WEBVIEW_DEBUG_PORT"), dataDir, configDir)
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
	lifecycleService := NewLifecycleService(startupService, closeService, nil)
	contextService := &ContextService{}
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
		*contextService = *builtContext
		coreContext := unwrapContextService(contextService)
		coreContext.SiviHeightEnabled = siviHeightEnabled
		coreContext.SiviParentReviewEnabled = siviParentReviewEnabled
		coreContext.SiviParentEditingEnabled = siviParentEditingEnabled
		coreContext.SiviParentActionEditingEnabled = siviParentActionEditingEnabled
		coreContext.SiviProjectAssignmentEnabled = siviProjectAssignmentEnabled
		coreContext.TableCSVReviewEnabled = tableCSVReviewEnabled
		coreContext.PlotLocationReviewEnabled = plotLocationReviewEnabled
		coreContext.SiteUnitSummaryEnabled = siteUnitSummaryEnabled
		coreContext.SiteUnitSummaryLifeformsEnabled = siteUnitSummaryLifeformsEnabled
		coreContext.SiteUnitSummarySpeciesEnabled = siteUnitSummarySpeciesEnabled
		coreContext.LongVegetationNoneEnabled = longVegetationNoneEnabled
		coreContext.LongVegetationLifeformEnabled = longVegetationLifeformEnabled
		coreContext.LongVegetationStrataEnabled = longVegetationStrataEnabled
		coreContext.LongVegetationCodeEnabled = longVegetationCodeEnabled
		coreContext.LongVegetationSelectedFieldEnabled = longVegetationSelectedFieldEnabled
		coreContext.LongVegetationNoneUnitsEnabled = longVegetationNoneUnitsEnabled
		coreContext.LongVegetationSelectedUnitsEnabled = longVegetationSelectedUnitsEnabled

		abort := func(cause error) (StartupState, error) {
			contextService.core = nil
			_ = projects.CloseSQLiteContext()
			return StartupState{}, cause
		}

		plotLabelRegional, err := NewPlotLabelRegionalService(contextService)
		if err != nil {
			return abort(err)
		}
		siviParentShared, err := NewSIVIParentSharedService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageParentExtra, err := NewTwoPageParentExtraService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageParentCommon, err := NewTwoPageParentCommonService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviCovers, err := NewSIVICoverService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviCombined, err := NewSIVICombinedService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviCollected, err := NewSIVICollectedService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviSpecies, err := NewSIVISpeciesService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviIdentity, err := NewSIVIIdentityService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviCreation, err := NewSIVICreationService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviCreationUndo, err := NewSIVICreationUndoService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siviDeletion, err := NewSIVIDeletionService(contextService, os.LookupEnv)
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
		pictureMetadata, err := NewPictureMetadataService(builtPictures, pictureLookup)
		if err != nil {
			return abort(err)
		}
		pictureCreation, err := NewPictureCreationService(builtPictures, pictureLookup)
		if err != nil {
			return abort(err)
		}
		pictureDeletion, err := NewPictureDeletionService(builtPictures, pictureLookup)
		if err != nil {
			return abort(err)
		}
		builtPictures.Metadata = pictureMetadata
		builtPictures.Creation = pictureCreation
		builtPictures.Deletion = pictureDeletion

		summaryVegetation, err := NewSummaryVegetationService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryEnvironment, err := NewSummaryVegetationEnvironmentalService(summaryVegetation, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryEnvironmentLifeform, err := NewSummaryVegetationEnvironmentalLifeformService(summaryEnvironment, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook, err := NewSummaryVegetationWorkbookService(summaryEnvironmentLifeform, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook.AncillaryEnabled, err = siviFeature(summaryVegetationAncillaryWorkbookFeatureEnvironment, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook.ReferencesEnabled, err = siviFeature(summaryVegetationEffectiveReferencesFeatureEnvironment, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook.GroupedEnabled, err = siviFeature(summaryVegetationGroupedWorkbookFeatureEnvironment, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationPreferencesEnabled, err := summaryVegetationPreferencesFeature(os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationQualityEnabled, err := summaryVegetationQualityPreferencesFeature(os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetation.QualityPreferences = summaryVegetationQualityEnabled
		summaryVegetationPresentationEnabled, err := summaryVegetationPresentationPreferencesFeature(os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		summaryVegetationWorkbook.PresentationEnabled = summaryVegetationPresentationEnabled
		summaryVegetationFieldListEnabled, err := summaryVegetationFieldListFeature(os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		lifeformSummary, err := NewLifeformSummaryService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		speciesAttributeSummary, err := NewSpeciesAttributeSummaryService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		lifeformWorkbook, err := NewLifeformWorkbookService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		subzoneMatrix, err := NewSubzoneMatrixService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siteUnitSummaryWorkbook, err := NewSiteUnitSummaryWorkbookService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		siteUnitSummaryExtendedWorkbook, err := NewSiteUnitSummaryExtendedWorkbookService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		environmentWorkbook, err := NewEnvironmentWorkbookService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		vegetationWorkbook, err := NewVegetationWorkbookService(contextService, os.LookupEnv)
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
		twoPageParentEntry, err := NewTwoPageParentEntryService(contextService, entryReaders, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageSoil, err := NewTwoPageSoilService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageSourceAudit, err := NewTwoPageSourceAuditService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageGoogleEarth, err := NewTwoPageGoogleEarthService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageVegetationTable, err := NewTwoPageVegetationTableService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageEnvironmentTable, err := NewTwoPageEnvironmentTableService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageProjectMetadata, err := NewTwoPageProjectMetadataService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		userLog, err := NewUserLogService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageProfiling, err := NewTwoPageProfilingService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		profileSpeciesPresence, err := NewProjectProfileSpeciesPresenceService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageVegetation, err := NewTwoPageVegetationService(contextService, os.LookupEnv)
		if err != nil {
			return abort(err)
		}
		twoPageVegetationSpeciesList, err := NewTwoPageVegetationSpeciesListService(contextService, os.LookupEnv)
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
			NewExternalSUEnvironmentService(contextService, externalSUEnvironmentEditingEnabled),
			twoPageProfiling,
			profileSpeciesPresence,
		)
		*validationService = *NewValidationService(contextService)
		*importExportService = *NewImportExportService(
			contextService,
			NewTableCSVArchiveService(contextService, tableCSVArchiveEnabled),
			plotLabelRegional,
			NewPlotLabelPDFExportService(contextService, plotLabelPDFExportEnabled),
		)
		*vegetationReportService = VegetationReportService{
			contexts:                       contextService,
			summary:                        summaryVegetation,
			summaryEnvironment:             summaryEnvironment,
			summaryEnvironmentLifeform:     summaryEnvironmentLifeform,
			summaryWorkbook:                summaryVegetationWorkbook,
			summaryPreferences:             NewSummaryVegetationPreferencesService(contextService, summaryVegetationPreferencesEnabled, summaryVegetation.Enabled),
			summaryQualityPreferences:      NewSummaryVegetationQualityPreferencesService(contextService, summaryVegetationQualityEnabled),
			summaryPresentationPreferences: NewSummaryVegetationPresentationPreferencesService(contextService, summaryVegetationPresentationEnabled),
			summaryFieldList:               NewSummaryVegetationFieldListService(contextService, summaryVegetationFieldListEnabled),
			longPreferences:                NewLongVegetationPreferencesService(contextService, longVegetationPreferencesEnabled),
			longStatisticsPreferences:      NewLongVegetationStatisticsPreferencesService(contextService, longVegetationStatisticsPreferencesEnabled),
			longSwitchPreferences:          NewLongVegetationSwitchPreferencesService(contextService, longVegetationSwitchPreferencesEnabled),
			longThresholdPreferences:       NewLongVegetationThresholdPreferencesService(contextService, longVegetationThresholdPreferencesEnabled),
			longGroupPreferences:           NewLongVegetationGroupPreferencesService(contextService, longVegetationGroupPreferencesEnabled),
			longQualityNullPreferences:     NewLongVegetationQualityNullPreferencesService(contextService, longVegetationQualityNullPreferencesEnabled),
			longQualityMinimumPreferences:  NewLongVegetationQualityMinimumPreferencesService(contextService, longVegetationQualityMinimumPreferencesEnabled),
			longNamingPreferences:          NewLongVegetationNamingPreferencesService(contextService, longVegetationNamingPreferencesEnabled),
			longSelectedFieldPreferences:   NewLongVegetationSelectedFieldPreferencesService(contextService, longVegetationSelectedFieldPreferencesEnabled),
			longEnforcementPreferences:     NewLongVegetationEnforcementPreferencesService(contextService, longVegetationEnforcementPreferencesEnabled),
			longUnitPreferences:            NewLongVegetationUnitPreferencesService(contextService, longVegetationUnitPreferencesEnabled),
			longSelectedUnitSelection:      NewLongVegetationSelectedUnitSelectionService(contextService, longVegetationSelectedUnitSelectionEnabled),
			longReportScope:                NewLongVegetationReportScopeService(contextService, longVegetationProjectScopeEnabled),
			longProjectWorkbook:            NewLongVegetationProjectWorkbookService(contextService, longVegetationProjectScopeEnabled, vegetationWorkbook.Private.Enabled),
			vegetationWorkbook:             vegetationWorkbook,
			lifeformSummary:                lifeformSummary,
			lifeformWorkbook:               lifeformWorkbook,
			speciesAttributeSummary:        speciesAttributeSummary,
		}
		*environmentReportService = EnvironmentReportService{
			contexts:                        contextService,
			environmentWorkbook:             environmentWorkbook,
			longEnvironmentPreferences:      NewLongEnvironmentPreferencesService(contextService, longEnvironmentPreferencesEnabled),
			siteUnitSummaryPreferences:      NewSiteUnitSummaryPreferencesService(contextService, siteUnitSummaryPreferencesEnabled),
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
			googleEarthReview:      NewGoogleEarthReviewService(contextService, googleEarthReviewEnabled),
			googleEarthKML:         NewGoogleEarthKMLService(contextService, googleEarthKMLEnabled),
			googleEarthKMLExport:   NewGoogleEarthKMLExportService(contextService, googleEarthKMLExportEnabled),
			googleEarthPreferences: NewGoogleEarthPreferencesService(contextService, googleEarthPreferencesEnabled),
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
