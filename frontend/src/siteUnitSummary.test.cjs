const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { componentFunctions } = require('./svelteTestHelpers.cjs');
const { summary, cell, fixture } = require('./siteUnitSummaryTestFixtures.cjs');
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
function lifeformFixture() {
  const value = fixture();
  value.report.querySource = 'selected-su-filtered-env-admin-quickveg-lifeform';
  value.report.fields.splice(23, 4, ...summary.summaryLifeformCaptions.map((label, form) =>
    ({source:'Lifeform', key:`Lifeform${form}`, label, section:'VEGETATION', kind:'lifeform-cover'})));
  value.report.units[0].values.splice(23, 4, ...summary.summaryLifeformCaptions.map(() => '0---0.3---1.1'));
  return value;
}
const validateLifeforms = value => summary.validateSiteUnitSummary(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db',1,'lifeform');
test('Summary lifeform mode verifies all49 fields without relaxing original layer/provenance validation',()=>{
  const value = lifeformFixture();
  assert.equal(validateLifeforms(value).report.fields.length, 49);
  assert.throws(()=>validate(value));
  assert.throws(()=>validateLifeforms(fixture()));
  for (const mutate of [
    v=>v.report.fields[23].label='unknown',v=>v.report.fields[36].key='Lifeform12',
    v=>v.report.fields[23].source='Env',v=>v.report.fields[23].kind='numeric',
    v=>v.report.fields[39].source='Env',v=>v.report.fields[37].section='VEGETATION',
    v=>v.report.units[0].values.pop(),v=>v.report.units[0].values[23]='\ud800',
    v=>v.report.units[0].plots[0].plotNumber='foreign'
  ]) { const invalid=lifeformFixture(); mutate(invalid); assert.throws(()=>validateLifeforms(invalid)); }
});
test('actual mode dispatch shares cancellation/owner checks and cannot change workbook authority',async()=>{
  let layerCalls=0, lifeformCalls=0, cancellations=0;
  const context=componentFunctions('SiteUnitSummary.svelte',['show','editMode'],{
    busy:false,workbookBusy:false,preferenceBarrier:false,speciesBarrier:false,speciesView:null,criteria:null,speciesPreview:null,ready:true,su:'Selected',method:1,
    mode:'layer',lifeformsEnabled:true,generation:0,error:'',preview:null,
    contextId:'owned',project:'Sample',projectPath:'C:\\project.db',suPath:'C:\\external.db',
    cancel(){context.generation++;cancellations++;},reportBusy(){},
    reads:{track(value){return value;}},
    ContextService:{
      PreviewSiteUnitSummary(){layerCalls++;return fixture();},
      PreviewSiteUnitSummaryLifeforms(){lifeformCalls++;return lifeformFixture();}
    },validateSiteUnitSummary:summary.validateSiteUnitSummary,
  });
  await context.actions.show();
  assert.equal(layerCalls,1);
  context.actions.editMode('lifeform');
  await context.actions.show();
  assert.equal(lifeformCalls,1);
  assert.equal(context.preview.report.fields.length,49);
  for(const flag of ['workbookBusy','preferenceBarrier','busy']) {
    context[flag]=true; context.actions.editMode('layer'); await context.actions.show();
    assert.equal(context.mode,'lifeform'); assert.equal(layerCalls,1); assert.equal(lifeformCalls,1);
    context[flag]=false;
  }
  context.lifeformsEnabled=false; context.mode='layer';
  context.actions.editMode('lifeform');
  assert.equal(context.mode,'layer');
  assert.ok(cancellations>=3);
  const panel=readFileSync(path.join(__dirname,'SiteUnitSummary.svelte'),'utf8');
  assert.match(panel,/VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true' && mode === 'layer'/);
});
test('summary component is independently gated, cancellable and enters shared draft-safe navigation',()=>{
  const read=file=>readFileSync(path.join(__dirname,file),'utf8');
  for(const file of ['SiteUnitSummary.svelte','Navigation.svelte','App.svelte']) {
    assert.equal(compile(read(file),{filename:file,generate:'client'}).warnings.length,0);
  }
  assert.match(read('Navigation.svelte'),/VITE_SITE_UNIT_SUMMARY === 'true' \? 'summary-environment' : undefined/);
  assert.match(read('App.svelte'),/view === 'summary-environment' && import.meta.env.VITE_SITE_UNIT_SUMMARY === 'true'/);
  const panel=read('SiteUnitSummary.svelte');
  assert.match(panel,/onDestroy\(\(\) => \{ unsubscribe\?\.\(\); unsubscribeSpecies\?\.\(\); cancel\(\); \}\)/);assert.match(panel,/request !== generation/);
  assert.match(panel,/onBusyChange\(busy \|\| preferenceBarrier \|\| workbookBusy \|\| speciesBarrier\)/);
  assert.match(panel,/checked=\{method === 1\}/);assert.match(panel,/validateSiteUnitSummary/);
  assert.match(panel,/if \(!preferencesEnabled\) void options\(\)/);
  assert.match(panel,/GetSiteUnitSummaryOptions/);
  assert.match(panel,/disabled=\{busy \|\| workbookBusy \|\| preferenceBarrier \|\| speciesBarrier \|\| !ready \|\| su === 'None'\}/);
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
