import { accessCaption, type PaperControl } from './paperLayout';

type Group = { title: string; fields: readonly string[] };
const groups: Record<string, readonly Group[]> = {
  Site: [
    { title: 'Identity and survey', fields: ['PlotNumber', 'Date', 'SiteSurveyor', 'FieldNumber'] },
    { title: 'Project', fields: ['ProjectID', 'StartDate', 'Option477', 'Option479', 'btnLoadMetadata'] },
    { title: 'BEC Master and Working Unit', fields: ['BECSiteUnit', 'UserSiteUnit', 'Option455', 'Option457', 'Option459', 'btnCoptToWorkingUnit'] },
    { title: 'Location', fields: ['Location', 'FSRegionDistrict', 'NtsMapSheet', 'UTMZone', 'UTMEasting', 'UTMNorthing', 'Combo467', 'AirPhotoNum', 'XCoord', 'YCoord', 'Ecosection'] },
    { title: 'Coordinate display', fields: ['Check369', 'Check371', 'Check373', 'Latitude', 'Longitude', 'LatD2', 'LatMD', 'LonD2', 'LonMD', 'LatD', 'LatM', 'LatS', 'LonD', 'LonM', 'LonS'] },
    { title: 'Site classification', fields: ['PlotRepresenting', 'Zone', 'SubZone', 'SiteSeries', 'RealmClass', 'TransDistrib', 'MapUnit'] },
    { title: 'Site conditions and stand', fields: ['MoistureRegime', 'NutrientRegime', 'SuccessionalStatus', 'StructuralStage', 'StandAge'] },
    { title: 'Topography', fields: ['Elevation', 'SlopeGradient', 'Aspect', 'MesoSlopePosition', 'SurfaceShape', 'SurfaceTopographyType', 'SurfaceTopographySize'] },
    { title: 'Data quality', fields: ['SitePlotQuality', 'VegPlotQuality', 'SoilPlotQuality'] },
    { title: 'Substrate (%)', fields: ['SubstrateOrganicMatter', 'SubstrateRocks', 'SubstrateDecWood', 'SubstrateMineralSoil', 'SubstrateBedRock', 'SubstrateWater'] },
    { title: 'Disturbance and exposure', fields: ['SiteDisturbance1', 'SiteDisturbance2', 'SiteDisturbance3', 'Exposure1', 'Exposure2'] },
    { title: 'Site diagram and pictures', fields: ['frmVPics', 'btnManagePictures', 'Photo'] },
    { title: 'Entry and source actions', fields: ['EnteredBy', 'UpdatedFromCards', 'Toggle408', 'Toggle409'] },
    { title: 'Field and office notes', fields: ['SiteNotes', 'OfficeNotes'] }
  ],
  'Soil/Terrain': [
    { title: 'Survey', fields: ['MensPlotNumber', 'SoilSurveyor'] },
    { title: 'Geology', fields: ['BedrockGeology1', 'BedrockGeology2', 'BedrockGeology3', 'CoarseFragLith1', 'CoarseFragLith2', 'CoarseFragLith3'] },
    { title: 'Surface terrain', fields: ['TerrainTextureSurf', 'SurficialMaterialSurf', 'SurfaceExpSurf', 'GeoMorProSurf'] },
    { title: 'Subsurface terrain', fields: ['TerrainTextureSubSurf', 'SurficialMaterialSubSurf', 'SurfaceExpSubSurf', 'GeoMorProSubSurf'] },
    { title: 'Soil classification', fields: ['SoilClassSubGroup', 'SoilClassGroup'] },
    { title: 'Humus form', fields: ['HumusForm', 'HumusFormPhase', 'HumusThickness'] },
    { title: 'Hydrogeology', fields: ['HydroGeoSystem', 'HydroGeoSubSystem'] },
    { title: 'Rooting and restricting layer', fields: ['RootingDepth', 'RootZoneParticleSize', 'RootRestrictingType', 'RootRestrictingDepth'] },
    { title: 'Water and drainage', fields: ['WaterSource', 'SoilDrainage', 'SeepageDepth', 'FloodingRegimeFreq', 'FloodingRegimeDur'] },
    { title: 'Organic horizons / layers', fields: ['SoilHumus'] },
    { title: 'Mineral horizons / layers', fields: ['SoilMineral'] },
    { title: 'Soil notes', fields: ['SoilNotes'] }
  ],
  Vegetation: [
    { title: 'Vegetation survey', fields: ['VegPlotNumber', 'VegSurveyor', 'SpeciesListComplete'] },
    { title: 'Stratum cover (%)', fields: ['StrataCoverTree', 'StrataCoverShrub', 'StrataCoverHerb', 'StrataCoverMoss'] },
    { title: 'Vegetation actions', fields: ['btnCoverAndHeight', 'btnCheckSppCodes', 'AddSpp', 'btnAllowSmallEntry', 'btnFindPlot'] },
    { title: 'Tree and shrub strata (A / B)', fields: ['SubVegA', 'SubVegAht'] },
    { title: 'Herb stratum (C)', fields: ['SubVegC', 'SubVegCht'] },
    { title: 'Moss and lichen stratum (D)', fields: ['SubVegD'] },
    { title: 'Vegetation notes', fields: ['VegNotes'] }
  ],
  'Veg Other': [
    { title: 'Plot', fields: ['Text445'] },
    { title: 'Vegetation characteristics', fields: ['USysVegOther'] }
  ],
  Other: [
    { title: 'Plot', fields: ['Text287'] },
    { title: 'User defined data', fields: ['SubOther'] }
  ]
};
const labels: Record<string, string> = {
  BECSiteUnit: 'BEC Master', UserSiteUnit: 'Working Unit', Zone: 'BEC zone', SubZone: 'BEC subzone',
  TransDistrib: 'Transition / distribution', MoistureRegime: 'Moisture regime', NutrientRegime: 'Nutrient regime',
  MesoSlopePosition: 'Meso slope position', SurfaceShape: 'Surface shape',
  SitePlotQuality: 'Site data quality', VegPlotQuality: 'Vegetation data quality', SoilPlotQuality: 'Soil data quality',
  SubstrateOrganicMatter: 'Organic matter (%)', SubstrateRocks: 'Rocks (%)', SubstrateDecWood: 'Decaying wood (%)',
  SubstrateMineralSoil: 'Mineral soil (%)', SubstrateBedRock: 'Bedrock (%)', SubstrateWater: 'Water (%)',
  SiteDisturbance1: 'Site disturbance 1', SiteDisturbance2: 'Site disturbance 2', SiteDisturbance3: 'Site disturbance 3',
  Exposure1: 'Exposure 1', Exposure2: 'Exposure 2', SiteNotes: 'Field notes', OfficeNotes: 'Office notes',
  SoilClassGroup: 'Soil great group', SoilClassSubGroup: 'Soil subgroup',
  BedrockGeology1: 'Bedrock type 1', BedrockGeology2: 'Bedrock type 2', BedrockGeology3: 'Bedrock type 3',
  CoarseFragLith1: 'Coarse fragment lithology 1', CoarseFragLith2: 'Coarse fragment lithology 2', CoarseFragLith3: 'Coarse fragment lithology 3',
  TerrainTextureSurf: 'Surface texture', SurficialMaterialSurf: 'Surface material', SurfaceExpSurf: 'Surface expression', GeoMorProSurf: 'Surface geomorphic process',
  TerrainTextureSubSurf: 'Subsurface texture', SurficialMaterialSubSurf: 'Subsurface material', SurfaceExpSubSurf: 'Subsurface expression', GeoMorProSubSurf: 'Subsurface geomorphic process',
  HumusFormPhase: 'Humus phase', HumusThickness: 'Humus thickness (cm)', HydroGeoSystem: 'Hydrogeologic system', HydroGeoSubSystem: 'Hydrogeologic subsystem',
  WaterSource: 'Water source', FloodingRegimeFreq: 'Flooding frequency', FloodingRegimeDur: 'Flooding duration',
  VegNotes: 'Vegetation notes', SoilNotes: 'Soil notes', Photo: 'Photo', SpeciesListComplete: 'Species list complete?',
  Check369: 'Decimal degrees', Check371: 'Degrees and decimal minutes', Check373: 'Degrees, minutes and seconds',
  Option455: 'Env choices', Option457: 'Master choices', Option459: 'SU table choices',
  Option477: 'Project from Env (unavailable)', Option479: 'Project from Master (unavailable)',
  LatD: 'Latitude degrees', LatM: 'Latitude minutes', LatS: 'Latitude seconds',
  LonD: 'Longitude degrees', LonM: 'Longitude minutes', LonS: 'Longitude seconds',
  LatD2: 'Latitude degrees', LatMD: 'Latitude decimal minutes', LonD2: 'Longitude degrees', LonMD: 'Longitude decimal minutes',
  Cover1: 'A1 cover (%)', Cover2: 'A2 cover (%)', Cover3: 'A3 cover (%)', TotalA: 'Total A cover (%)',
  Cover4: 'B1 cover (%)', Cover5: 'B2 cover (%)', TotalB: 'Total B cover (%)',
  Height1: 'A1 height', Height2: 'A2 height', Height3: 'A3 height', Height4: 'B1 height', Height5: 'B2 height',
  UpperDepth: 'Upper depth', LowerDepth: 'Lower depth', HumusFormpH: 'Humus pH', MineralFormpH: 'Mineral pH',
  HumusStructureKind: 'Humus structure kind', HumusStructureDegree: 'Humus structure degree',
  MineralStructureKind: 'Mineral structure kind', MineralStructureClass: 'Mineral structure class',
  PercentCoarseFragsGravel: 'Coarse fragments: gravel (%)', PercentCoarseFragsCobbles: 'Coarse fragments: cobbles (%)',
  PercentCoarseFragsStones: 'Coarse fragments: stones (%)', PercentCoarseFragsTotal: 'Coarse fragments: total (%)',
  PercentCoarseFragsShape: 'Coarse fragments: shape', RootsAbundance: 'Roots abundance', RootsSize: 'Roots size'
};
export function controlLabel(control: PaperControl): string {
  const token = control.controlName ?? control.column ?? '';
  return labels[token] ?? labels[control.column ?? ''] ??
    (accessCaption(control.caption).replace(/\s+/g, ' ').trim() ||
      (control.column ?? token).replace(/([a-z])([A-Z])/g, '$1 $2').replace(/([a-zA-Z])(\d)/g, '$1 $2') || 'Source field');
}
export function presentationGroups(name: string, controls: PaperControl[]) {
  const remaining = new Set(controls.filter(control => !['Label', 'Rectangle', 'OptionGroup'].includes(control.type)));
  const result = (groups[name] ?? []).map(group => {
    const members = group.fields.flatMap(token => [...remaining].filter(control => control.controlName === token || control.column === token));
    const unique = [...new Set(members)];
    unique.forEach(control => remaining.delete(control));
    return { title: group.title, controls: unique };
  }).filter(group => group.controls.length);
  if (remaining.size) result.push({ title: 'Additional source fields and actions', controls: [...remaining].sort((a, b) => a.tabOrder - b.tabOrder) });
  return result;
}
export function wideControl(control: PaperControl): boolean {
  return control.type === 'Subform' || ['Location', 'PlotRepresenting', 'SiteNotes', 'OfficeNotes', 'SoilNotes', 'VegNotes'].includes(control.column ?? '');
}
