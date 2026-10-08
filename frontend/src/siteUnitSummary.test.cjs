const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
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
const validate = value => summary.validateSiteUnitSummary(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db',1);
test('summary retains empty units, physical duplicate weights, exclusions and source field order',()=>{
  assert.doesNotThrow(()=>validate(fixture()));
  const value=fixture(); value.report.units[0].values[0]='CWH(2)  (Null 1)';
  assert.equal(validate(value).report.units[0].values[0],'CWH(2)  (Null 1)');
});
test('summary rejects mismatched ownership, method, fields, names and physical provenance',()=>{
  for (const mutate of [
    v=>v.contextId='stale',v=>v.projectPath='elsewhere',v=>v.report.method=2,v=>v.report.querySource='unknown',
    v=>v.report.fields.pop(),v=>v.report.fields[8].key='SiteDisturbance1',v=>v.report.fields[29].source='Env',
    v=>v.report.units[0].values.pop(),v=>v.report.units[0].values[0]='\ud800',
    v=>v.report.units.push(v.report.units[0]),v=>v.report.units[0].longName='invented',
    v=>v.report.units[0].plots[1].adminRowId='1',v=>v.report.units[0].plots[0].envRowId='01',
    v=>v.report.units[0].plots[0].plotNumber='P2',v=>v.report.memberships[0].joinedRows=1,
    v=>v.report.memberships[1].status='joined',v=>v.report.memberships.push(v.report.memberships[0]),
    v=>v.report.memberships[0].siteUnit=cell(null),v=>v.report.units[0].plots=[]
  ]) { const value=fixture();mutate(value);assert.throws(()=>validate(value)); }
});
test('summary component is independently gated, cancellable and enters shared draft-safe navigation',()=>{
  const read=file=>readFileSync(path.join(__dirname,file),'utf8');
  for(const file of ['SiteUnitSummary.svelte','Navigation.svelte','App.svelte']) {
    assert.equal(compile(read(file),{filename:file,generate:'client'}).warnings.length,0);
  }
  assert.match(read('Navigation.svelte'),/VITE_SITE_UNIT_SUMMARY === 'true' \? 'summary-environment' : undefined/);
  assert.match(read('App.svelte'),/view === 'summary-environment' && import.meta.env.VITE_SITE_UNIT_SUMMARY === 'true'/);
  const panel=read('SiteUnitSummary.svelte');
  assert.match(panel,/onDestroy\(\(\) => \{ unsubscribe\?\.\(\); cancel\(\); \}\)/);assert.match(panel,/request !== generation/);
  assert.match(panel,/onBusyChange\(busy \|\| preferenceBarrier \|\| workbookBusy\)/);
  assert.match(panel,/checked=\{method === 1\}/);assert.match(panel,/validateSiteUnitSummary/);
  assert.match(panel,/if \(!preferencesEnabled\) void options\(\)/);
  assert.match(panel,/GetSiteUnitSummaryOptions/);
  assert.match(panel,/disabled=\{busy \|\| workbookBusy \|\| preferenceBarrier \|\| !ready \|\| su === 'None'\}/);
  assert.match(panel,/ready = siteUnitType === 1/);
  assert.match(panel,/VITE_SITE_UNIT_SUMMARY_PREFERENCES === 'true'/);
  assert.doesNotMatch(panel,/<button[^>]*>Export|localStorage/);
});
test('saved summary options retain quartile initialization and unavailable source scopes',()=>{
  for (const siteUnitType of [1,2,3]) {
    const value={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
      su:'Selected',suPath:'C:\\external.db',method:2,siteUnitType};
    assert.deepEqual(summary.validateSiteUnitSummaryOptions(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db'),value);
  }
});
test('saved summary options reject stale, missing, malformed or guessed values',()=>{
  const value={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',method:2,siteUnitType:1};
  const check=v=>summary.validateSiteUnitSummaryOptions(v,'owned','Sample','C:\\project.db','Selected','C:\\external.db');
  assert.throws(()=>check(null));
  for(const mutate of [
    v=>v.contextId='stale',v=>v.project='Else',v=>v.projectPath='elsewhere',
    v=>v.su='None',v=>v.suPath='elsewhere',v=>delete v.method,v=>v.method=null,
    v=>v.method='2',v=>v.method=3,v=>v.method=1.5,v=>delete v.siteUnitType,
    v=>v.siteUnitType=null,v=>v.siteUnitType='1',v=>v.siteUnitType=4
  ]) { const invalid=structuredClone(value);mutate(invalid);assert.throws(()=>check(invalid)); }
});
