package integration

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/cloudsync"
	"github.com/boostao/vpro-wails/internal/datasheet"
	"github.com/boostao/vpro-wails/internal/diagnostics"
	"github.com/boostao/vpro-wails/internal/interchange"
	"github.com/boostao/vpro-wails/internal/lifecycle"
	"github.com/boostao/vpro-wails/internal/lumping"
	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/rsidecar"
	"github.com/boostao/vpro-wails/internal/sivi"
	"github.com/boostao/vpro-wails/internal/spatial"
	"github.com/boostao/vpro-wails/internal/twopage"
	"github.com/boostao/vpro-wails/internal/validation"
	"github.com/boostao/vpro-wails/internal/vegreport"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type strictDTOTestCase struct {
	name      string
	newTarget func() any
	validJSON string
	stringKey string // optional JSON string property name on this DTO for string-value surrogate/UTF-8 probes
}

func wave2To5StrictDTOTestCases() []strictDTOTestCase {
	return []strictDTOTestCase{
		// internal/datasheet
		{
			name:      "datasheet.DatasheetPageRequest",
			newTarget: func() any { return &datasheet.DatasheetPageRequest{} },
			validJSON: `{"tableKey":"Env","offset":0,"limit":50,"sort":[{"column":"PlotNumber","direction":"asc"}],"filters":[{"column":"Zone","op":"eq","value":"🌲"}],"globalSearch":"\\ud800","scopeToActiveSU":true}`,
			stringKey: "tableKey",
		},
		{
			name:      "datasheet.DatasheetCellEdit",
			newTarget: func() any { return &datasheet.DatasheetCellEdit{} },
			validJSON: `{"rowId":"1","column":"Zone","before":{"storage":"null","text":null,"integer":null,"real":null,"blobHex":null},"after":{"storage":"text","text":"CWH","integer":null,"real":null,"blobHex":null}}`,
			stringKey: "column",
		},
		{
			name:      "datasheet.DatasheetEditRequest",
			newTarget: func() any { return &datasheet.DatasheetEditRequest{} },
			validJSON: `{"tableKey":"Env","edits":[{"rowId":"1","column":"Zone","before":{"storage":"null","text":null,"integer":null,"real":null,"blobHex":null},"after":{"storage":"text","text":"CWH","integer":null,"real":null,"blobHex":null}}]}`,
			stringKey: "tableKey",
		},
		{
			name:      "datasheet.DatasheetFindReplaceRequest",
			newTarget: func() any { return &datasheet.DatasheetFindReplaceRequest{} },
			validJSON: `{"tableKey":"Env","column":"Zone","findText":"IDF","replaceText":"BG","matchCase":true,"exactCell":false,"filters":[],"globalSearch":"","scopeToActiveSU":false,"maxPreview":25}`,
			stringKey: "findText",
		},

		// internal/projectops
		{
			name:      "projectops.NewProjectRequest",
			newTarget: func() any { return &projectops.NewProjectRequest{} },
			validJSON: `{"projectName":"DemoProj","projectId":"DP1","projectTitle":"🌲 Survey","presetId":"LMH25_2023_FS882","destinationPath":"/tmp/DemoProj.db","activateOnSuccess":false}`,
			stringKey: "projectName",
		},
		{
			name:      "projectops.SaveProjectAsRequest",
			newTarget: func() any { return &projectops.SaveProjectAsRequest{} },
			validJSON: `{"newProjectName":"CopyProj","destinationPath":"/tmp/CopyProj.db","activateOnSuccess":false}`,
			stringKey: "newProjectName",
		},
		{
			name:      "projectops.ExportSplinterRequest",
			newTarget: func() any { return &projectops.ExportSplinterRequest{} },
			validJSON: `{"splinterProject":"SubProj","destinationPath":"/tmp/SubProj.db","useActiveSu":true,"siteUnits":["01"],"plotNumbers":["00337"],"activateOnSuccess":false}`,
			stringKey: "splinterProject",
		},
		{
			name:      "projectops.CompareProjectsRequest",
			newTarget: func() any { return &projectops.CompareProjectsRequest{} },
			validJSON: `{"sourcePath":"/tmp/A.db","sourceProject":"A","targetPath":"/tmp/B.db","targetProject":"B"}`,
			stringKey: "targetPath",
		},
		{
			name:      "projectops.OptimizeVegOptions",
			newTarget: func() any { return &projectops.OptimizeVegOptions{} },
			validJSON: `{"plotNumbers":["00337","108050"]}`,
		},
		{
			name:      "projectops.ApplyOptimizeVegRequest",
			newTarget: func() any { return &projectops.ApplyOptimizeVegRequest{} },
			validJSON: `{"plotNumbers":["00337"],"expectedDigest":"deadbeef"}`,
			stringKey: "expectedDigest",
		},
		{
			name:      "projectops.ApplyProjectMetadataAutomationRequest",
			newTarget: func() any { return &projectops.ApplyProjectMetadataAutomationRequest{} },
			validJSON: `{"projectId":"SAMPLE","applyDerived":true,"presetId":"LMH25_2023_FS882"}`,
			stringKey: "projectId",
		},
		{
			name:      "projectops.ThreeWayMergePreviewRequest",
			newTarget: func() any { return &projectops.ThreeWayMergePreviewRequest{} },
			validJSON: `{"sourcePath":"/tmp/Src.db","sourceProject":"Src","targetPath":"/tmp/Tgt.db","targetProject":"Tgt","basePath":"/tmp/Base.db","baseProject":"Base"}`,
			stringKey: "sourcePath",
		},
		{
			name:      "projectops.ThreeWayMergeExecuteRequest",
			newTarget: func() any { return &projectops.ThreeWayMergeExecuteRequest{} },
			validJSON: `{"sourcePath":"/tmp/Src.db","expectedPreviewDigest":"abc","defaultConflictResolution":"keep_target","plotResolutions":{"A1":"keep_target","a1":"accept_source"},"fieldResolutions":{"A1::Env::Zone":"keep_target","a1::Env::Zone":"accept_source"}}`,
			stringKey: "sourcePath",
		},

		// internal/classification
		{
			name:      "classification.CreateSUTableRequest",
			newTarget: func() any { return &classification.CreateSUTableRequest{} },
			validJSON: `{"name":"Draft2026","destination":"project","filePath":"","populateAllProjectPlots":true,"seedFromUserSiteUnit":false,"activate":true}`,
			stringKey: "name",
		},
		{
			name:      "classification.SaveSUTableAsRequest",
			newTarget: func() any { return &classification.SaveSUTableAsRequest{} },
			validJSON: `{"sourceSu":"Sample","newName":"CopySU","destination":"project","filePath":"","activate":false}`,
			stringKey: "newName",
		},
		{
			name:      "classification.BECUnitPickerRequest",
			newTarget: func() any { return &classification.BECUnitPickerRequest{} },
			validJSON: `{"query":"CWH","zone":"CWH","subZone":"vm1","limit":20}`,
			stringKey: "query",
		},
		{
			name:      "classification.AssignPlotsToSiteUnitRequest",
			newTarget: func() any { return &classification.AssignPlotsToSiteUnitRequest{} },
			validJSON: `{"plotNumbers":["00337"],"targetSiteUnit":"CWHvm1/01","removeFromSuWhenUnassigned":false}`,
			stringKey: "targetSiteUnit",
		},
		{
			name:      "classification.CreateSUFromEnvFieldsRequest",
			newTarget: func() any { return &classification.CreateSUFromEnvFieldsRequest{} },
			validJSON: `{"fields":["Zone","SubZone","SiteSeries"],"separator":"/","useBecSlashFormat":true,"mode":"overwrite","targetSuName":"EnvSU","destination":"project","filePath":"","scopeToExistingSuRowsOnly":false,"skipIncompleteRows":true,"previewOnly":true,"activate":false}`,
			stringKey: "separator",
		},
		{
			name:      "classification.CreateHierarchyTableRequest",
			newTarget: func() any { return &classification.CreateHierarchyTableRequest{} },
			validJSON: `{"name":"Hier2026","destination":"project","filePath":"","activate":true}`,
			stringKey: "name",
		},
		{
			name:      "classification.HierarchyAddNodeRequest",
			newTarget: func() any { return &classification.HierarchyAddNodeRequest{} },
			validJSON: `{"name":"TsugaOrder","parentId":null,"level":4,"tag":"","isBreakpoint":false}`,
			stringKey: "name",
		},
		{
			name:      "classification.HierarchyRenameNodeRequest",
			newTarget: func() any { return &classification.HierarchyRenameNodeRequest{} },
			validJSON: `{"nodeId":1,"newName":"RenamedOrder"}`,
			stringKey: "newName",
		},
		{
			name:      "classification.HierarchyReparentNodeRequest",
			newTarget: func() any { return &classification.HierarchyReparentNodeRequest{} },
			validJSON: `{"nodeId":2,"newParentId":1,"newLevel":5}`,
		},
		{
			name:      "classification.HierarchyDeleteNodeRequest",
			newTarget: func() any { return &classification.HierarchyDeleteNodeRequest{} },
			validJSON: `{"nodeId":2,"cascadeChildren":true}`,
		},
		{
			name:      "classification.SaveFlatHierarchyRequest",
			newTarget: func() any { return &classification.SaveFlatHierarchyRequest{} },
			validJSON: `{"rows":[{"leafId":1,"class":"C1","order":"O1","subOrder":"","alliance":"A1","subAlliance":"","association":"Assoc1","subAssociation":"","facies":"","workingUnit":"","siteSeries":"CWHvm1/01"}]}`,
		},

		// internal/lumping
		{
			name:      "lumping.LumpRowInput",
			newTarget: func() any { return &lumping.LumpRowInput{} },
			validJSON: `{"lumpCode":"CAREX","sppCode":"CARESIT","use":true,"allowUnlisted":false}`,
			stringKey: "lumpCode",
		},
		{
			name:      "lumping.LumpSpec",
			newTarget: func() any { return &lumping.LumpSpec{} },
			validJSON: `{"preset":"BEC_Provincial_Correlation_2026","collapseInfraspecific":true,"collapseSeedlingAndDeadSuffixes":true,"genusRollups":["CARE*"],"nonVascularLifeforms":[9,10],"baseLumpTable":"","baseLumpDbPath":"","customRules":[{"lumpCode":"CAREX","sppCode":"CARESIT","use":true}],"scopeToProjectSpecies":true}`,
			stringKey: "preset",
		},

		// internal/sivi & internal/twopage
		{
			name:      "sivi.CreateSIVIPlotRequest",
			newTarget: func() any { return &sivi.CreateSIVIPlotRequest{} },
			validJSON: `{"plotNumber":"SV001","plotType":"Ground","projectId":"P1","fieldNumber":"F1","date":"2026-07-01","siteSurveyor":"BM","svPolygonNumber":"12","zone":"CWH","subZone":"vm1","siteSeries":"01","userSiteUnit":"","addToActiveSu":true}`,
			stringKey: "plotNumber",
		},
		{
			name:      "sivi.UndoCreateSIVIPlotRequest",
			newTarget: func() any { return &sivi.UndoCreateSIVIPlotRequest{} },
			validJSON: `{"creationRequestId":"550e8400-e29b-41d4-a716-446655440000","plotNumber":"SV001"}`,
			stringKey: "plotNumber",
		},
		{
			name:      "twopage.DeleteTwoPageSoilHorizonRequest",
			newTarget: func() any { return &twopage.DeleteTwoPageSoilHorizonRequest{} },
			validJSON: `{"plotNumber":"108050","form":"SubSoilHumus2p","id":10,"rowId":"1"}`,
			stringKey: "plotNumber",
		},
		{
			name:      "twopage.RestoreTwoPageSoilHorizonRequest",
			newTarget: func() any { return &twopage.RestoreTwoPageSoilHorizonRequest{} },
			validJSON: `{"plotNumber":"108050","form":"SubSoilHumus2p","deleteRequestId":"550e8400-e29b-41d4-a716-446655440000"}`,
			stringKey: "plotNumber",
		},

		// internal/validation
		{
			name:      "validation.ComplianceOptions",
			newTarget: func() any { return &validation.ComplianceOptions{} },
			validJSON: `{"useActiveSu":true}`,
		},
		{
			name:      "validation.QuarantineOrphanRequest",
			newTarget: func() any { return &validation.QuarantineOrphanRequest{} },
			validJSON: `{"tables":["Veg","Humus"]}`,
		},

		// internal/interchange
		{
			name:      "interchange.AccessProjectImportRequest",
			newTarget: func() any { return &interchange.AccessProjectImportRequest{} },
			validJSON: `{"sourcePath":"/tmp/Legacy.accdb","sourceProject":"Legacy","targetProject":"Imported","destinationPath":"/tmp/Imported.db","activateOnSuccess":false}`,
			stringKey: "sourcePath",
		},
		{
			name:      "interchange.VenusImportRequest",
			newTarget: func() any { return &interchange.VenusImportRequest{} },
			validJSON: `{"sourcePath":"/tmp/Venus.xml","sourceProject":"V5","targetProject":"V5Imp","destinationPath":"/tmp/V5Imp.db","activateOnSuccess":false}`,
			stringKey: "sourcePath",
		},
		{
			name:      "interchange.TurboVegExportRequest",
			newTarget: func() any { return &interchange.TurboVegExportRequest{} },
			validJSON: `{"format":"xml","destinationPath":"/tmp/tv.xml","lumpTable":"","lumpDbPath":""}`,
			stringKey: "destinationPath",
		},
		{
			name:      "interchange.TurboVegImportRequest",
			newTarget: func() any { return &interchange.TurboVegImportRequest{} },
			validJSON: `{"sourcePath":"/tmp/tv.xml","format":"xml","targetProject":"TVImp","destinationPath":"/tmp/TVImp.db","activateOnSuccess":false}`,
			stringKey: "sourcePath",
		},
		{
			name:      "interchange.UserSpeciesExportRequest",
			newTarget: func() any { return &interchange.UserSpeciesExportRequest{} },
			validJSON: `{"format":"json","destinationPath":"/tmp/userspp.json"}`,
			stringKey: "format",
		},
		{
			name:      "interchange.UserSpeciesImportRequest",
			newTarget: func() any { return &interchange.UserSpeciesImportRequest{} },
			validJSON: `{"sourcePath":"/tmp/userspp.json","format":"json","content":"","collisionPolicy":"skip"}`,
			stringKey: "sourcePath",
		},
		{
			name:      "interchange.LongTidyCSVRequest",
			newTarget: func() any { return &interchange.LongTidyCSVRequest{} },
			validJSON: `{"destinationPath":"/tmp/long.csv","lumpTable":"","lumpDbPath":""}`,
			stringKey: "destinationPath",
		},
		{
			name:      "interchange.WideMatrixCSVRequest",
			newTarget: func() any { return &interchange.WideMatrixCSVRequest{} },
			validJSON: `{"destinationPath":"/tmp/wide.csv","layerMode":"rollup_all","zeroFill":true,"includeSiteUnit":true,"lumpTable":"","lumpDbPath":""}`,
			stringKey: "layerMode",
		},
		{
			name:      "interchange.RDSExportRequest",
			newTarget: func() any { return &interchange.RDSExportRequest{} },
			validJSON: `{"destinationPath":"/tmp/project.rds","lumpTable":"","lumpDbPath":""}`,
			stringKey: "destinationPath",
		},
		{
			name:      "interchange.GeoPackageExportRequest",
			newTarget: func() any { return &interchange.GeoPackageExportRequest{} },
			validJSON: `{"destinationPath":"/tmp/plots.gpkg","crs":"EPSG:3005","includeAttributeTables":true,"includeUnlocatedPlots":false,"lumpTable":"","lumpDbPath":""}`,
			stringKey: "crs",
		},

		// internal/spatial
		{
			name:      "spatial.PlotGeodesyInput",
			newTarget: func() any { return &spatial.PlotGeodesyInput{} },
			validJSON: `{"latitude":54.5,"longitude":-126.5,"utmZone":"09","utmEasting":661000,"utmNorthing":6042000,"locationAccuracy":25}`,
			stringKey: "utmZone",
		},
		{
			name:      "spatial.SpatialAOISelectionRequest",
			newTarget: func() any { return &spatial.SpatialAOISelectionRequest{} },
			validJSON: `{"polygon":[[-127,54],[-126,54],[-126,55],[-127,54]],"rings":[],"zone":"CWH"}`,
			stringKey: "zone",
		},
		{
			name:      "spatial.AssignSpatialAOIToSURequest",
			newTarget: func() any { return &spatial.AssignSpatialAOIToSURequest{} },
			validJSON: `{"mode":"update_active","suName":"Sample","filePath":"","siteUnit":"CWHvm1/01","plotNumbers":["108050"],"polygon":[],"rings":[],"includeNonAoiPlotsAsUnassigned":false,"activate":false}`,
			stringKey: "siteUnit",
		},
		{
			name:      "spatial.ApplySpatialAttributesRequest",
			newTarget: func() any { return &spatial.ApplySpatialAttributesRequest{} },
			validJSON: `{"plotNumbers":["108050"],"allScopedPlots":false,"stampGisBgc":true,"stampAdminFields":true,"onlyWhenBlank":true,"normalizeLongitudeSign":true,"syncUtmFromLatLon":false,"syncLatLonFromUtm":false}`,
		},

		// internal/diagnostics & internal/rsidecar
		{
			name:      "diagnostics.SynthesisOptions",
			newTarget: func() any { return &diagnostics.SynthesisOptions{} },
			validJSON: `{"unitSource":"auto","lumpTable":"","lumpDbPath":"","excludeStandardModifiers":true,"excludePatterns":[".poor"],"significanceBasis":"all_plots","unitOrder":[],"minMaxPresencePct":0,"pairwise":{"minConstancy":60,"minPlots":3}}`,
			stringKey: "unitSource",
		},
		{
			name:      "diagnostics.ReassignSynthesisPlotRequest",
			newTarget: func() any { return &diagnostics.ReassignSynthesisPlotRequest{} },
			validJSON: `{"plotNumber":"108050","targetSiteUnit":"01","options":{"unitSource":"auto","excludeStandardModifiers":true}}`,
			stringKey: "plotNumber",
		},
		{
			name:      "diagnostics.UndoSynthesisReassignmentRequest",
			newTarget: func() any { return &diagnostics.UndoSynthesisReassignmentRequest{} },
			validJSON: `{"historyId":1,"options":{"unitSource":"auto","excludeStandardModifiers":true}}`,
		},
		{
			name:      "diagnostics.ExportDiagnosticWorkbookRequest",
			newTarget: func() any { return &diagnostics.ExportDiagnosticWorkbookRequest{} },
			validJSON: `{"outputPath":"/tmp/diag.xlsx","reportTitle":"BEC Synthesis","includeCommonNames":true,"options":{"unitSource":"auto","excludeStandardModifiers":true}}`,
			stringKey: "outputPath",
		},
		{
			name:      "rsidecar.ClimateEdatopicRequest",
			newTarget: func() any { return &rsidecar.ClimateEdatopicRequest{} },
			validJSON: `{"refPeriod":"1961_1990","groupBy":"site_unit","bgcFilter":"","siteUnitFilter":"","useActiveSu":true,"plotNumbers":["108050"],"forceRefresh":false,"forceOfflineModel":true}`,
			stringKey: "refPeriod",
		},
		{
			name:      "rsidecar.ClusteringRequest",
			newTarget: func() any { return &rsidecar.ClusteringRequest{} },
			validJSON: `{"method":"NC","distance":"bray","linkage":"ward.D2","transform":"sqrt","k":3,"fuzziness":1.25,"dnoise":0.75,"cutHeight":0,"applyLump":false,"lumpName":"","useActiveSu":true,"bgcFilter":"","plotNumbers":["108050"],"unitPrefix":"CL","candidatePrefix":"Cand","materializeCandidate":false}`,
			stringKey: "method",
		},
		{
			name:      "rsidecar.PromoteCandidateSURequest",
			newTarget: func() any { return &rsidecar.PromoteCandidateSURequest{} },
			validJSON: `{"candidateSuName":"Sample_ClusterCandidate","targetSuName":"Rev2026","excludeNoise":true,"confirmOverwrite":false,"activate":false}`,
			stringKey: "targetSuName",
		},
		{
			name:      "rsidecar.QuartoGuideRequest",
			newTarget: func() any { return &rsidecar.QuartoGuideRequest{} },
			validJSON: `{"guideType":"single_unit","title":"Guide","targetUnit":"01","bgcFilter":"CWHvm1","conceptNotes":"🌲 Notes","refPeriod":"1961_1990","minConstancyPct":20,"useActiveSu":true,"applyLump":false,"includePdf":false,"destinationPath":"/tmp/guide.html"}`,
			stringKey: "title",
		},

		// internal/cloudsync
		{
			name:      "cloudsync.SessionConfigRequest",
			newTarget: func() any { return &cloudsync.SessionConfigRequest{} },
			validJSON: `{"roleMode":"contributor","clearOverrides":false,"host":"localhost","port":5432,"database":"becmaster","sslMode":"require","user":"u","password":"p","adminUser":"a","adminPassword":"ap"}`,
			stringKey: "roleMode",
		},
		{
			name:      "cloudsync.PublishStandardsRequest",
			newTarget: func() any { return &cloudsync.PublishStandardsRequest{} },
			validJSON: `{"releaseId":"R2026","becVersion":"BECdb_v13_2026","taxonomyVersion":"BCPlantTaxa_v19_2025","notes":"🌲 Baseline"}`,
			stringKey: "releaseId",
		},
		{
			name:      "cloudsync.SeedBaselineRequest",
			newTarget: func() any { return &cloudsync.SeedBaselineRequest{} },
			validJSON: `{"sourceDbPath":"/tmp/BECMaster.db","sourceProject":"BECMaster_fixing","snapshotTag":"BECMaster_v13_Baseline","replaceExisting":true,"skipAudit":false,"notes":"Initial"}`,
			stringKey: "sourceDbPath",
		},
		{
			name:      "cloudsync.SubsetFilter",
			newTarget: func() any { return &cloudsync.SubsetFilter{} },
			validJSON: `{"zones":["CWH"],"subzones":["vm1"],"projectIds":["P1"],"plotNumbers":["00337"],"bbox":[-128.5,48.5,-123.0,51.0],"limit":100,"allowFullCheckout":false}`,
		},
		{
			name:      "cloudsync.PullSubsetRequest",
			newTarget: func() any { return &cloudsync.PullSubsetRequest{} },
			validJSON: `{"filter":{"zones":["CWH"],"plotNumbers":["00337"]},"targetProject":"CWH_2026","destinationPath":"/tmp/CWH_2026.db","activateOnSuccess":true}`,
			stringKey: "targetProject",
		},
		{
			name:      "cloudsync.SubmitStagingRequest",
			newTarget: func() any { return &cloudsync.SubmitStagingRequest{} },
			validJSON: `{"useActiveSu":false,"submitterNotes":"🌲 Field season","submittedBy":"Ecologist","baseSnapshotTag":"BECMaster_v13_Baseline","custodianOverride":false}`,
			stringKey: "submitterNotes",
		},
		{
			name:      "cloudsync.ListSubmissionsFilter",
			newTarget: func() any { return &cloudsync.ListSubmissionsFilter{} },
			validJSON: `{"status":"pending","submittedBy":"Ecologist","limit":50}`,
			stringKey: "status",
		},
		{
			name:      "cloudsync.StagingMergePreviewRequest",
			newTarget: func() any { return &cloudsync.StagingMergePreviewRequest{} },
			validJSON: `{"submissionId":"550e8400-e29b-41d4-a716-446655440000","baseDbPath":"/tmp/Base.db","baseProject":"Base"}`,
			stringKey: "submissionId",
		},
		{
			name:      "cloudsync.StagingMergeExecuteRequest",
			newTarget: func() any { return &cloudsync.StagingMergeExecuteRequest{} },
			validJSON: `{"submissionId":"550e8400-e29b-41d4-a716-446655440000","expectedPreviewDigest":"abc","defaultConflictResolution":"keep_target","plotResolutions":{"00337":"accept_source"},"fieldResolutions":{"00337::Env::Zone":"accept_source"},"reviewerNotes":"Approved","actor":"Will MacKenzie"}`,
			stringKey: "submissionId",
		},
		{
			name:      "cloudsync.RejectSubmissionRequest",
			newTarget: func() any { return &cloudsync.RejectSubmissionRequest{} },
			validJSON: `{"submissionId":"550e8400-e29b-41d4-a716-446655440000","reviewerNotes":"Missing horizon depths","actor":"Will MacKenzie"}`,
			stringKey: "reviewerNotes",
		},
		{
			name:      "cloudsync.CreateSnapshotRequest",
			newTarget: func() any { return &cloudsync.CreateSnapshotRequest{} },
			validJSON: `{"snapshotTag":"BECMaster_2026_11","notes":"Approved cut","exportFormat":"rds","destinationPath":"/tmp/snap.rds","zoneFilter":"CWH"}`,
			stringKey: "snapshotTag",
		},

		// internal/vegreport
		{
			name:      "vegreport.HierarchyBreakpointRequest",
			newTarget: func() any { return &vegreport.HierarchyBreakpointRequest{} },
			validJSON: `{"rules":[{"pattern":".poor","matchMode":"suffix","enabled":true}],"useDefaultPresets":true,"options":{"breakpointPolicy":"lowest","unbrokenUnitPolicy":"keep_original","stripTildePrefix":true}}`,
		},
		{
			name:      "vegreport.SiteUnitExclusionPreviewRequest",
			newTarget: func() any { return &vegreport.SiteUnitExclusionPreviewRequest{} },
			validJSON: `{"rules":[{"pattern":".nudum","matchMode":"auto","enabled":true}],"useDefaultPresets":true,"useHierarchyBreakpoints":false,"breakpointOptions":{"breakpointPolicy":"lowest","unbrokenUnitPolicy":"keep_original","stripTildePrefix":false}}`,
		},
		{
			name:      "vegreport.SummaryVegetationExclusionRequest",
			newTarget: func() any { return &vegreport.SummaryVegetationExclusionRequest{} },
			validJSON: `{"average":1,"display":1,"showClass":0,"groupAndOrder":1,"rules":[],"useDefaultPresets":true,"useHierarchyBreakpoints":false,"breakpointOptions":{"breakpointPolicy":"lowest","unbrokenUnitPolicy":"keep_original","stripTildePrefix":false}}`,
		},
		{
			name:      "vegreport.LongVegetationExclusionRequest",
			newTarget: func() any { return &vegreport.LongVegetationExclusionRequest{} },
			validJSON: `{"rules":[],"useDefaultPresets":true,"useHierarchyBreakpoints":true,"breakpointOptions":{"breakpointPolicy":"highest","unbrokenUnitPolicy":"exclude_unbroken","stripTildePrefix":true}}`,
		},

		// Core/lifecycle Wails input structs
		{
			name:      "vprocore.ContextSelection",
			newTarget: func() any { return &vprocore.ContextSelection{} },
			validJSON: `{"project":"Sample","projectPath":"/tmp/Sample.db","su":"Sample","suPath":"/tmp/Sample.db","hierarchy":"Sample","hierarchyPath":"/tmp/Sample.db"}`,
			stringKey: "project",
		},
		{
			name:      "vprocore.CoordinateParts",
			newTarget: func() any { return &vprocore.CoordinateParts{} },
			validJSON: `{"degrees":54,"minutes":30,"seconds":15.5,"negative":true}`,
		},
		{
			name:      "lifecycle.StartupRecoveryRequest",
			newTarget: func() any { return &lifecycle.StartupRecoveryRequest{} },
			validJSON: `{"projectPath":"/tmp/Sample.db","projectName":"Sample","suPath":"","clearSu":true,"hierarchyPath":"","clearHierarchy":true}`,
			stringKey: "projectName",
		},
		{
			name:      "classification.ProjectPlotProfileRunRequest",
			newTarget: func() any { return &classification.ProjectPlotProfileRunRequest{} },
			validJSON: `{"originalRules":{"columns":[],"rows":[]},"projectLump":null,"subvarieties":true}`,
		},
		{
			name:      "vprocore.ProjectMetadataCell",
			newTarget: func() any { return &vprocore.ProjectMetadataCell{} },
			validJSON: `{"storage":"text","text":"🌲","integer":null,"real":null,"blobHex":null}`,
			stringKey: "storage",
		},
	}
}

func firstTopLevelMember(validJSON string) (key string, rawVal string) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(validJSON), &m); err != nil {
		panic(err)
	}
	// Preserve deterministic first key in validJSON order using Decoder.
	dec := json.NewDecoder(strings.NewReader(validJSON))
	_, _ = dec.Token() // '{'
	tok, err := dec.Token()
	if err != nil {
		panic(err)
	}
	k := tok.(string)
	return k, string(m[k])
}

func unicodeFoldVariant(key string) (string, bool) {
	if idx := strings.IndexByte(key, 's'); idx >= 0 {
		return key[:idx] + "\u017f" + key[idx+1:], true
	}
	if idx := strings.IndexByte(key, 'S'); idx >= 0 {
		return key[:idx] + "\u017f" + key[idx+1:], true
	}
	if idx := strings.IndexByte(key, 'k'); idx >= 0 {
		return key[:idx] + "\u212a" + key[idx+1:], true
	}
	if idx := strings.IndexByte(key, 'K'); idx >= 0 {
		return key[:idx] + "\u212a" + key[idx+1:], true
	}
	return "", false
}

func TestWave2To5DTOsEnforceStrictJSONWireAndDecoding(t *testing.T) {
	for _, tc := range wave2To5StrictDTOTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Valid wire unmarshals cleanly.
			okTarget := tc.newTarget()
			if err := json.Unmarshal([]byte(tc.validJSON), okTarget); err != nil {
				t.Fatalf("valid JSON rejected for %s: %v", tc.name, err)
			}

			firstKey, firstRawVal := firstTopLevelMember(tc.validJSON)
			body := strings.TrimSuffix(strings.TrimSpace(tc.validJSON), "}")

			// Build rejection vectors required across every DTO.
			badVectors := []struct {
				label string
				wire  []byte
			}{
				{"unknown_field", []byte(body + `,"__unknownStrictProbe":1}`)},
				{"trailing_json_braces", []byte(tc.validJSON + `{}`)},
				{"exact_duplicate_key", []byte(body + `,"` + firstKey + `":` + firstRawVal + `}`)},
				{"ascii_case_duplicate_key", []byte(body + `,"` + strings.ToUpper(firstKey) + `":` + firstRawVal + `}`)},
				{"surrogate_key", []byte(body + `,"\ud800":1}`)},
				{"raw_0xff_wire", append(append([]byte(body+`,"`), 0xff), []byte(`":1}`)...)},
			}
			if folded, ok := unicodeFoldVariant(firstKey); ok {
				badVectors = append(badVectors, struct {
					label string
					wire  []byte
				}{"unicode_equalfold_duplicate_key", []byte(body + `,"` + folded + `":` + firstRawVal + `}`)})
			}
			if tc.stringKey != "" {
				badVectors = append(badVectors,
					struct {
						label string
						wire  []byte
					}{"lone_high_surrogate_ud800", []byte(`{"` + tc.stringKey + `":"\ud800"}`)},
					struct {
						label string
						wire  []byte
					}{"lone_low_surrogate_udfff", []byte(`{"` + tc.stringKey + `":"\udfff"}`)},
					struct {
						label string
						wire  []byte
					}{"mismatched_surrogate_ud800_u0041", []byte(`{"` + tc.stringKey + `":"\ud800\u0041"}`)},
					struct {
						label string
						wire  []byte
					}{"raw_0xff_in_string_field", append(append([]byte(`{"`+tc.stringKey+`":"`), 0xff), '"', '}')},
				)
			}

			for _, bv := range badVectors {
				dst := tc.newTarget()
				zeroSnapshot := reflect.New(reflect.TypeOf(dst).Elem()).Elem().Interface()
				if err := json.Unmarshal(bv.wire, dst); err == nil {
					t.Fatalf("%s accepted %s (%q)", tc.name, bv.label, bv.wire)
				}
				if !reflect.DeepEqual(reflect.ValueOf(dst).Elem().Interface(), zeroSnapshot) {
					t.Fatalf("%s mutated target on %s error", tc.name, bv.label)
				}
			}
		})
	}
}

func TestThreeWayMergeExecuteRequestCaseDistinctMapKeysVsStructDuplicateKeys(t *testing.T) {
	var req projectops.ThreeWayMergeExecuteRequest
	valid := []byte(`{
		"sourcePath": "/tmp/Source.db",
		"plotResolutions": {"A1": "keep_target", "a1": "accept_source", "rootsSize": "keep_target", "rootſSize": "accept_source"},
		"fieldResolutions": {"A1::Env::Zone": "keep_target", "a1::Env::Zone": "accept_source"}
	}`)
	if err := json.Unmarshal(valid, &req); err != nil {
		t.Fatalf("ThreeWayMergeExecuteRequest rejected valid case-distinct map keys: %v", err)
	}
	if req.PlotResolutions["A1"] != "keep_target" || req.PlotResolutions["a1"] != "accept_source" ||
		req.PlotResolutions["rootsSize"] != "keep_target" || req.PlotResolutions["rootſSize"] != "accept_source" {
		t.Fatalf("unexpected PlotResolutions: %#v", req.PlotResolutions)
	}
	if req.FieldResolutions["A1::Env::Zone"] != "keep_target" || req.FieldResolutions["a1::Env::Zone"] != "accept_source" {
		t.Fatalf("unexpected FieldResolutions: %#v", req.FieldResolutions)
	}

	for _, bad := range []struct {
		name string
		wire string
	}{
		{"exact_dup_plot_map_key", `{"sourcePath":"/tmp/S.db","plotResolutions":{"A1":"keep_target","A1":"accept_source"}}`},
		{"escaped_exact_dup_plot_map_key", `{"sourcePath":"/tmp/S.db","plotResolutions":{"A1":"keep_target","\u00411":"accept_source"}}`},
		{"exact_dup_field_map_key", `{"sourcePath":"/tmp/S.db","fieldResolutions":{"A1::Env::Zone":"keep_target","A1::Env::Zone":"accept_source"}}`},
		{"struct_case_dup_key", `{"sourcePath":"/tmp/S.db","SourcePath":"/tmp/Other.db"}`},
		{"struct_unicode_fold_dup_key", `{"sourcePath":"/tmp/S.db","plotResolutions":{},"plotReſolutions":{}}`},
		{"surrogate_in_map_val", `{"sourcePath":"/tmp/S.db","plotResolutions":{"A1":"\ud800"}}`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			var out projectops.ThreeWayMergeExecuteRequest
			if err := json.Unmarshal([]byte(bad.wire), &out); err == nil {
				t.Fatalf("expected ThreeWayMergeExecuteRequest to reject %s (%s)", bad.name, bad.wire)
			}
		})
	}
}
