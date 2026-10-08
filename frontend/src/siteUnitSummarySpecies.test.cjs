const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { quality, restoration, summary, cell, fixture } = require('./siteUnitSummaryTestFixtures.cjs');
const location = loadTypeScript('plotLocationReview.ts', {'./projectMetadataRestore':restoration});
const earth = loadTypeScript('googleEarthReview.ts', {'./qualityEditor':quality,'./projectMetadataRestore':restoration,'./plotLocationReview':location});
const species = loadTypeScript('siteUnitSummarySpecies.ts', {
  './projectMetadataRestore':restoration,'./googleEarthReview':earth,'./siteUnitSummary':summary
});
const owner = {contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'Selected',suPath:'C:\\external.db'};
const request = {method:1,orderBy:1,coverCalculation:1,andOr:1,presenceGreaterThan:0,coverGreaterThan:0};
function speciesFixture(order=1) {
  const environment=fixture();
  if(order===2) {
    environment.report.querySource='selected-su-filtered-env-admin-quickveg-lifeform';
    environment.report.fields.splice(23,4,...summary.summaryLifeformCaptions.map((label,index)=>({
      source:'Lifeform',key:`Lifeform${index}`,label,section:'VEGETATION',kind:'lifeform-cover'
    })));
    environment.report.units[0].values.splice(23,4,...summary.summaryLifeformCaptions.map(()=>''));
  }
  return {environment,options:{...request,orderBy:order},units:[{code:'',nPlots:2,groups:[{
    index:1,caption:order===1?'Layer 1':'Coniferous Tree',rows:[{
      species:'A',scientificName:cell('Scientific'),englishName:cell(null),codeType:order===1?null:cell('U'),
      cover:'1.01',presence:'100.0',physicalValues:2,included:true,referenceRowIds:['1']
    }]
  }]}]};
}
test('species projection validates both exact field modes, nullable definitions and physical denominator',()=>{
  for(const order of [1,2]) {
    const value=speciesFixture(order);
    assert.equal(species.validateSummarySpecies(value,owner,{...request,orderBy:order}).environment.report.fields.length,order===1?39:49);
  }
  const value=speciesFixture();
  value.units[0].groups[0].rows[0].presence='200.0';value.units[0].groups[0].rows[0].physicalValues=4;
  assert.equal(species.validateSummarySpecies(value,owner,request).units[0].groups[0].rows[0].presence,'200.0');
  for(const [plots,count,presence] of [
    [3,1,'33.3'],[3,2,'66.7'],[3,7,'233.3'],[8,1,'12.5'],[16,1,'6.3'],
    [2000,1,'0.1'],[2000,3,'0.2'],[2000,19,'1.0'],[4000,1,'0.0'],[4000,3,'0.1'],
    [2001,1,'0.0'],[1999,1,'0.1']
  ]) {
    const value=speciesFixture();
    value.environment.report.memberships[0].joinedRows=plots;
    value.environment.report.units[0].plots=Array.from({length:plots},(_,index)=>({
      plotNumber:'P1',suRowId:'1',envRowId:'1',adminRowId:String(index+1)
    }));
    value.units[0].nPlots=plots;
    Object.assign(value.units[0].groups[0].rows[0],{
      physicalValues:count,presence,included:Number(presence)>0
    });
    assert.equal(species.validateSummarySpecies(value,owner,request).units[0].groups[0].rows[0].presence,presence);
  }
});
test('species transport refuses mismatched scope/options/grouping/format/weights/identity and threshold decisions',()=>{
  for(const mutate of [
    v=>v.environment.contextId='stale',v=>v.options.coverCalculation=2,v=>v.units=[],
    v=>v.units[0].code='foreign',v=>v.units[0].nPlots=1,v=>v.units[0].groups=null,
    v=>v.units[0].groups[0].index=8,v=>v.units[0].groups[0].caption='unknown',
    v=>v.units[0].groups[0].rows=[],v=>v.units[0].groups[0].rows[0].cover='1.0',
    v=>v.units[0].groups[0].rows[0].presence='100.00',v=>v.units[0].groups[0].rows[0].cover='-0.00',
    v=>v.units[0].groups[0].rows[0].physicalValues=3,v=>v.units[0].groups[0].rows[0].included=false,
    v=>v.units[0].groups[0].rows[0].scientificName=cell('\ud800'),
    v=>v.units[0].groups[0].rows[0].referenceRowIds=['01'],
    v=>v.units[0].groups[0].rows[0].referenceRowIds=['1','1'],
    v=>v.units[0].groups[0].rows[0].codeType=cell('U'),
    v=>v.units[0].groups[0].rows.push(v.units[0].groups[0].rows[0])
  ]) { const value=speciesFixture();mutate(value);assert.throws(()=>species.validateSummarySpecies(value,owner,request)); }
  const value=speciesFixture(2);value.units[0].groups[0].rows[0].codeType=cell('s');
  assert.throws(()=>species.validateSummarySpecies(value,owner,{...request,orderBy:2}));
});
test('explicit criteria preserve raw text/errors/remount ownership and do not hide invalid drafts',()=>{
  const scope={...owner,contextId:'criteria'};
  const session=species.summarySpeciesCriteria(scope);
  session.setEnabled(true);session.editText('coverGreaterThan',' 1 ');
  assert.match(session.view().error,/complete source INTEGER/);
  assert.throws(()=>session.setEnabled(false));
  assert.equal(species.summarySpeciesCriteria(scope).view().coverGreaterThan,' 1 ');
  assert.throws(()=>species.summarySpeciesCriteria({...scope,suPath:'foreign'}));
  session.editText('coverGreaterThan','+001');
  assert.equal(session.view().coverGreaterThan,'+001');assert.equal(session.view().error,'');
  assert.equal(session.request(2,'lifeform').coverGreaterThan,1);
  for(const text of ['', '1.5','1e2','32768','-32769','\ud800','\ufffd']) {
    session.editText('presenceGreaterThan',text);assert.notEqual(session.view().error,'');
  }
  session.reset();assert.equal(session.view().error,'');assert.equal(session.view().enabled,true);
  for(const text of ['-32768','32767','-0','000']) assert.doesNotThrow(()=>species.summarySpeciesCriterion(text,'Threshold'));
});
test('actual species dispatch, late cancellation and workbook/preference/error barriers retain criteria',async()=>{
  let calls=0, resolve;
  const criteria=new species.SummarySpeciesCriteria();criteria.setEnabled(true);
  const context=componentFunctions('SiteUnitSummary.svelte',['show','editMode','editSpeciesEnabled','editSpeciesText','resetSpeciesCriteria','savePreferences'],{
    busy:false,workbookBusy:false,preferenceBarrier:false,speciesBarrier:false,speciesView:criteria.view(),criteria,
    ready:true,su:'Selected',method:1,mode:'layer',lifeformsEnabled:true,generation:0,error:'',preview:null,speciesPreview:null,
    ...owner,preferences:{save(){throw Error('Unexpected preference write');}},
    cancel(){context.generation++;context.busy=false;},reportBusy(){},
    reads:{track(value){return value;}},
    ContextService:{PreviewSiteUnitSummarySpecies(){calls++;return new Promise(done=>resolve=done);}},
    validateSummarySpecies:species.validateSummarySpecies,validateSiteUnitSummary:summary.validateSiteUnitSummary
  });
  const pending=context.actions.show();assert.equal(calls,1);
  context.generation++;resolve(speciesFixture());await pending;
  assert.equal(context.preview,null);assert.equal(context.speciesPreview,null);
  context.busy=false;
  for(const barrier of ['workbookBusy','preferenceBarrier','speciesBarrier']) {
    context[barrier]=true;context.actions.editMode('lifeform');context.actions.editSpeciesEnabled(false);
    assert.equal(context.mode,'layer');assert.equal(criteria.view().enabled,true);
    await context.actions.show();assert.equal(calls,1);context[barrier]=false;
  }
  context.speciesBarrier=true;
  await context.actions.savePreferences();
  context.actions.editSpeciesText('coverGreaterThan','1x');
  assert.notEqual(criteria.view().error,'');
  context.actions.resetSpeciesCriteria();assert.equal(criteria.view().error,'');
  context.ready=false;
  context.actions.editSpeciesEnabled(false);
  assert.equal(criteria.view().enabled,true);
  context.actions.editSpeciesText('coverGreaterThan','1x');
  assert.notEqual(criteria.view().error,'');
  context.actions.editSpeciesText('coverGreaterThan','1');
  assert.equal(criteria.view().error,'');
  context.actions.editSpeciesText('coverGreaterThan','1x');
  context.actions.resetSpeciesCriteria();assert.equal(criteria.view().error,'');
  const panel=readFileSync(path.join(__dirname,'SiteUnitSummary.svelte'),'utf8');
  assert.equal(compile(panel,{filename:'SiteUnitSummary.svelte',generate:'client'}).warnings.length,0);
  assert.match(panel,/VITE_SITE_UNIT_SUMMARY_SPECIES === 'true'/);
  assert.match(panel,/mode === 'layer' && !speciesView\?\.enabled/);
  assert.match(panel,/onBusyChange\(busy \|\| preferenceBarrier \|\| workbookBusy \|\| speciesBarrier\)/);
  assert.match(panel,/<fieldset disabled=\{busy \|\| workbookBusy \|\| preferenceBarrier\}>\s*<legend>Species preview criteria/);
  assert.match(panel,/<fieldset disabled=\{busy \|\| workbookBusy \|\| speciesBarrier \|\| preferenceView/);
});
