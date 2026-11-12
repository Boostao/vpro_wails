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
	if _, err := siviFeature(breakReportReviewEnvironment, os.LookupEnv); err != nil {
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
	siviCHARSParentEntryEnabled, err := siviFeature("VPRO_SIVI_CHARS_PARENT_ENTRY", os.LookupEnv)
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
	plotLabelRegional, err := NewPlotLabelRegionalService(contextService)
	if err != nil {
		log.Fatal(err)
	}
	contextService.siviParentReviewEnabled = siviParentReviewEnabled
	contextService.siviParentEditingEnabled = siviParentEditingEnabled
	contextService.siviParentActionEditingEnabled = siviParentActionEditingEnabled
	contextService.siviProjectAssignmentEnabled = siviProjectAssignmentEnabled
	contextService.tableCSVReviewEnabled = tableCSVReviewEnabled
	contextService.plotLocationReviewEnabled = plotLocationReviewEnabled
	contextService.siteUnitSummaryEnabled = siteUnitSummaryEnabled
	contextService.siteUnitSummaryLifeformsEnabled = siteUnitSummaryLifeformsEnabled
	contextService.siteUnitSummarySpeciesEnabled = siteUnitSummarySpeciesEnabled
	contextService.longVegetationNoneEnabled = longVegetationNoneEnabled
	contextService.longVegetationLifeformEnabled = longVegetationLifeformEnabled
	contextService.longVegetationStrataEnabled = longVegetationStrataEnabled
	contextService.longVegetationCodeEnabled = longVegetationCodeEnabled
	contextService.longVegetationSelectedFieldEnabled = longVegetationSelectedFieldEnabled
	contextService.longVegetationNoneUnitsEnabled = longVegetationNoneUnitsEnabled
	contextService.longVegetationSelectedUnitsEnabled = longVegetationSelectedUnitsEnabled
	siviParentShared, err := NewSIVIParentSharedService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageParentExtra, err := NewTwoPageParentExtraService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageParentCommon, err := NewTwoPageParentCommonService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviCovers, err := NewSIVICoverService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviCombined, err := NewSIVICombinedService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviCollected, err := NewSIVICollectedService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviSpecies, err := NewSIVISpeciesService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviIdentity, err := NewSIVIIdentityService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviCreation, err := NewSIVICreationService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviCreationUndo, err := NewSIVICreationUndoService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siviDeletion, err := NewSIVIDeletionService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	pictures, err := NewPictureService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	pictureMetadata, err := NewPictureMetadataService(pictures, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	pictureCreation, err := NewPictureCreationService(pictures, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	pictureDeletion, err := NewPictureDeletionService(pictures, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	pictures.metadata = pictureMetadata
	pictures.creation = pictureCreation
	pictures.deletion = pictureDeletion
	summaryVegetation, err := NewSummaryVegetationService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryEnvironment, err := NewSummaryVegetationEnvironmentalService(summaryVegetation, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryEnvironmentLifeform, err := NewSummaryVegetationEnvironmentalLifeformService(summaryEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationWorkbook, err := NewSummaryVegetationWorkbookService(summaryEnvironmentLifeform, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationWorkbook.ancillaryEnabled, err = siviFeature(summaryVegetationAncillaryWorkbookFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationWorkbook.referencesEnabled, err = siviFeature(summaryVegetationEffectiveReferencesFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationWorkbook.groupedEnabled, err = siviFeature(summaryVegetationGroupedWorkbookFeatureEnvironment, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationPreferencesEnabled, err := summaryVegetationPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationQualityEnabled, err := summaryVegetationQualityPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetation.qualityPreferences = summaryVegetationQualityEnabled
	summaryVegetationPresentationEnabled, err := summaryVegetationPresentationPreferencesFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	summaryVegetationWorkbook.presentationEnabled = summaryVegetationPresentationEnabled
	summaryVegetationFieldListEnabled, err := summaryVegetationFieldListFeature(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	lifeformSummary, err := NewLifeformSummaryService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	speciesAttributeSummary, err := NewSpeciesAttributeSummaryService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	lifeformWorkbook, err := NewLifeformWorkbookService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	subzoneMatrix, err := NewSubzoneMatrixService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siteUnitSummaryWorkbook, err := NewSiteUnitSummaryWorkbookService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	siteUnitSummaryExtendedWorkbook, err := NewSiteUnitSummaryExtendedWorkbookService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	environmentWorkbook, err := NewEnvironmentWorkbookService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	vegetationWorkbook, err := NewVegetationWorkbookService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
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
	siviParentShared.references = siviParentSharedReferenceReaders{
		site: siteCodes, region: regionCodes, parent: parentCodes,
		geology: geologyCodes, bec: becService,
	}
	siviCHARSParentEntry, err := newSIVICHARSParentEntryService(contextService, siviParentShared.references, siviCHARSParentEntryEnabled)
	if err != nil {
		log.Fatal(err)
	}
	entryReaders := twoPageEntryReferenceReaders{}
	if siteCodes != nil {
		entryReaders.shared.site = siteCodes
	}
	if regionCodes != nil {
		entryReaders.shared.region = regionCodes
		entryReaders.ecosection = regionCodes
	}
	if parentCodes != nil {
		entryReaders.shared.parent = parentCodes
	}
	if geologyCodes != nil {
		entryReaders.shared.geology = geologyCodes
	}
	if becService != nil {
		entryReaders.shared.bec = becService
		entryReaders.series = becService
	}
	if qualityService != nil {
		entryReaders.quality = qualityService
	}
	if soilCodes != nil {
		entryReaders.soil = soilCodes
	}
	twoPageParentEntry, err := NewTwoPageParentEntryService(contextService, entryReaders, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageSoil, err := NewTwoPageSoilService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageSourceAudit, err := NewTwoPageSourceAuditService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageGoogleEarth, err := NewTwoPageGoogleEarthService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageVegetationTable, err := NewTwoPageVegetationTableService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageEnvironmentTable, err := NewTwoPageEnvironmentTableService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageProjectMetadata, err := NewTwoPageProjectMetadataService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	userLog, err := NewUserLogService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageProfiling, err := NewTwoPageProfilingService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	profileSpeciesPresence, err := NewProjectProfileSpeciesPresenceService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageVegetation, err := NewTwoPageVegetationService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	twoPageVegetationSpeciesList, err := NewTwoPageVegetationSpeciesListService(contextService, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	workingUnits, err := NewWorkingUnitService(projects, configDir)
	if err != nil {
		log.Printf("Warning: failed to initialize WorkingUnitService: %v", err)
	}
	releaseChannel, err := preferences.releaseChannel()
	if err != nil {
		runStartupRecovery(windows, configDir, err)
		return
	}
	helpService, err := NewHelpService(preferences)
	if err != nil {
		log.Fatal(err)
	}

	var app *application.App
	closeService, err := NewCloseService(
		func(nonce string) bool { return app.Event.Emit(closeRequestEvent, nonce) },
		func() { app.Quit() },
	)
	if err != nil {
		log.Fatal(err)
	}
	startupService := &StartupService{state: StartupState{
		Ready:          true,
		ConfigPath:     preferences.path,
		ReleaseChannel: releaseChannel,
	}}
	lifecycleService := NewLifecycleService(startupService, closeService, userLog)
	plotEntryService := &PlotEntryService{
		contexts:                     contextService,
		plots:                        plots,
		siviParentShared:             siviParentShared,
		siviCHARSParentEntry:         siviCHARSParentEntry,
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
	datasheetService := NewDatasheetService(twoPageEnvironmentTable, twoPageVegetationTable)
	classificationService := NewClassificationService(
		contextService,
		becService,
		workingUnits,
		NewExternalSUEnvironmentService(contextService, externalSUEnvironmentEditingEnabled),
		twoPageProfiling,
		profileSpeciesPresence,
	)
	validationService := NewValidationService(contextService)
	importExportService := NewImportExportService(
		contextService,
		NewTableCSVArchiveService(contextService, tableCSVArchiveEnabled),
		plotLabelRegional,
		NewPlotLabelPDFExportService(contextService, plotLabelPDFExportEnabled),
	)
	vegetationReportService := &VegetationReportService{
		contexts:                       contextService,
		summary:                        summaryVegetation,
		summaryEnvironment:             summaryEnvironment,
		summaryEnvironmentLifeform:     summaryEnvironmentLifeform,
		summaryWorkbook:                summaryVegetationWorkbook,
		summaryPreferences:             NewSummaryVegetationPreferencesService(contextService, summaryVegetationPreferencesEnabled, summaryVegetation.enabled),
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
		longProjectWorkbook:            NewLongVegetationProjectWorkbookService(contextService, longVegetationProjectScopeEnabled, vegetationWorkbook.private.enabled),
		vegetationWorkbook:             vegetationWorkbook,
		lifeformSummary:                lifeformSummary,
		lifeformWorkbook:               lifeformWorkbook,
		speciesAttributeSummary:        speciesAttributeSummary,
	}
	environmentReportService := &EnvironmentReportService{
		contexts:                        contextService,
		environmentWorkbook:             environmentWorkbook,
		longEnvironmentPreferences:      NewLongEnvironmentPreferencesService(contextService, longEnvironmentPreferencesEnabled),
		siteUnitSummaryPreferences:      NewSiteUnitSummaryPreferencesService(contextService, siteUnitSummaryPreferencesEnabled),
		siteUnitSummaryWorkbook:         siteUnitSummaryWorkbook,
		siteUnitSummaryExtendedWorkbook: siteUnitSummaryExtendedWorkbook,
		subzoneMatrix:                   subzoneMatrix,
	}
	spatialService := &SpatialService{
		contexts:               contextService,
		coordinates:            coordinates,
		googleEarthReview:      NewGoogleEarthReviewService(contextService, googleEarthReviewEnabled),
		googleEarthKML:         NewGoogleEarthKMLService(contextService, googleEarthKMLEnabled),
		googleEarthKMLExport:   NewGoogleEarthKMLExportService(contextService, googleEarthKMLExportEnabled),
		googleEarthPreferences: NewGoogleEarthPreferencesService(contextService, googleEarthPreferencesEnabled),
		twoPageGoogleEarth:     twoPageGoogleEarth,
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
