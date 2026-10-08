const { readFileSync } = require('node:fs');
const path = require('node:path');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names = loadTypeScript('reportUnitNames.ts', {'./projectMetadataRestore':restoration});
const summary = loadTypeScript('siteUnitSummary.ts', {'./projectMetadataRestore':restoration,'./reportUnitNames':names});
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});
function fixture() {
  const keys = 'Zone SubZone Elevation Aspect SlopeGradient MesoSlopePosition MoistureRegime NutrientRegime SiteDisturbance2 SiteDisturbance2 Exposure1 SurfaceTopographyType SubstrateOrganicMatter SubstrateRocks SubstrateDecWood SubstrateMineralSoil SubstrateBedRock SubstrateWater HydroGeoSystem HydroGeoSubSystem StandAge SuccessionalStatus StructuralStage StrataCoverTree StrataCoverShrub StrataCoverHerb StrataCoverMoss SoilClassGroup HumusForm HumusThickness SoilDrainage SeepageDepth SurficialMaterialSurf RootZoneParticleSize RootingDepth RootRestrictingType BedrockGeology1 BedrockGeology2 BedrockGeology3'.split(' ');
  return {contextId:'owned',projectPath:'C:\\project.db',suPath:'C:\\external.db',report:{
    project:'Sample',su:'Selected',method:1,querySource:'selected-su-filtered-env-admin',
    fields:keys.map((key,i)=>({source:i===29?'Admin':'Env',key,label:key,section:i<21?'SITE':i<27?'VEGETATION':'SOILS',kind:'category'})),
    memberships:[{rowId:'1',plotNumber:cell('P1'),siteUnit:cell(''),joinedRows:2,status:'joined'},
      {rowId:'2',plotNumber:cell(null),siteUnit:cell('U'),joinedRows:0,status:'null-plot'}],
    units:[{code:'',longName:null,nameStatus:'missing',nameCandidates:[],values:keys.map(()=>''),plots:[
      {plotNumber:'P1',suRowId:'1',envRowId:'1',adminRowId:'1'},
      {plotNumber:'P1',suRowId:'1',envRowId:'1',adminRowId:'2'}]}]
  }};
}
module.exports = { quality, restoration, summary, cell, fixture };
