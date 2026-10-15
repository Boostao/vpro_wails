const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {proxy} = require('svelte/internal/client');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const cells = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names = loadTypeScript('reportUnitNames.ts', {'./projectMetadataRestore':cells});
const environment = loadTypeScript('longEnvironmentReport.ts', {'./projectMetadataRestore':cells,'./reportUnitNames':names});
const ordinary = loadTypeScript('longVegetationReport.ts', {
  './projectMetadataRestore':cells,'./longEnvironmentReport':environment,'./qualityEditor':quality,'./reportUnitNames':names
});
const locations = loadTypeScript('plotLocationReview.ts', {'./projectMetadataRestore':cells});
const earth = loadTypeScript('googleEarthReview.ts', {'./projectMetadataRestore':cells,'./qualityEditor':quality,'./plotLocationReview':locations});
const selection = loadTypeScript('longVegetationSelectedUnitSelection.ts', {
  './googleEarthReview':earth,'./qualityEditor':quality,'./longVegetationReport':ordinary
});
const project = loadTypeScript('longVegetationProjectReport.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./qualityEditor':quality
});
const scopeClient = loadTypeScript('longVegetationProjectScope.ts', {
  './longVegetationProjectReport':project,'./longVegetationSelectedUnitSelection':selection,'./qualityEditor':quality
});
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectScopeWire.json'),'utf8'));
const previews = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectUnitsWire.json'),'utf8'));
const owner = scope => Object.fromEntries(['contextId','project','projectPath','su','suPath'].map(key=>[key,scope[key]]));
const options = (value,index) => ({scope:structuredClone(value.scope),options:{...owner(value.scope),settings:structuredClone(previews[index].settings)}});
const preview = (value,index) => ({scope:structuredClone(value.scope),preview:structuredClone(previews[index])});

test('strict production-Go scope wrappers preserve source2/source3/qualified ownership on actual client proxies',()=>{
  assert.equal(wire.length,3);
  for(const [index,value] of wire.entries()) {
    const physicalOwner=owner(value.scope);
    for(const reactive of [false,true]) {
      const wrap=input=>reactive?proxy(structuredClone(input)):structuredClone(input);
      const scope=wrap(value.scope);
      if(reactive) assert.throws(()=>structuredClone(scope));
      assert.equal(scopeClient.validateReportScope(scope,physicalOwner,true),scope);
      assert.equal(scopeClient.currentProjectScope(scope,physicalOwner,true),scope);
      const catalogue=wrap(value.catalogue),prefs=wrap(value.unitPreferences),receipt=wrap(value.outcome);
      assert.equal(scopeClient.validateProjectFieldCatalogue(catalogue,scope,physicalOwner,true),catalogue);
      assert.equal(scopeClient.validateProjectUnitPreferences(prefs,scope,physicalOwner,true),prefs);
      assert.equal(prefs.preferences.values.unitGroups,1);
      assert.equal(scopeClient.validateProjectUnitOutcome(receipt,scope,physicalOwner,true,true),receipt);
      const review=wrap(options(value,index)),report=wrap(preview(value,index));
      assert.equal(scopeClient.validateProjectOptions(review,scope,physicalOwner,true),review);
      assert.equal(scopeClient.validateProjectPreview(report,scope,physicalOwner,true),report);
      assert.equal(report.preview.report.projectUnits.length,index===0?4:index===1?2:1);
      assert.equal(scope.build.projectTuples.length,index===0?4:2);
    }
  }
  assert.deepEqual(wire[1].scope.fields,['MissingField']);
  assert.equal(wire[1].scope.suPath,'');
  assert.equal(previews[2].report.quality.occurrences.length,8);
  assert.equal(previews[2].report.units[0].numPlots,1);
});

test('every project scope/wrapper client entry remains default-off',()=>{
  const value=wire[0],physicalOwner=owner(value.scope);
  for(const run of [
    ()=>scopeClient.validateReportScope(value.scope,physicalOwner),
    ()=>scopeClient.currentProjectScope(value.scope,physicalOwner),
    ()=>scopeClient.validateProjectFieldCatalogue(value.catalogue,value.scope,physicalOwner),
    ()=>scopeClient.validateProjectOptions(options(value,0),value.scope,physicalOwner),
    ()=>scopeClient.validateProjectPreview(preview(value,0),value.scope,physicalOwner),
    ()=>scopeClient.validateProjectUnitPreferences(value.unitPreferences,value.scope,physicalOwner),
    ()=>scopeClient.validateProjectUnitOutcome(value.outcome,value.scope,physicalOwner,true)
  ]) assert.throws(run,/disabled/);
  assert.throws(()=>scopeClient.validateReportScope(value.scope,physicalOwner,1),/disabled/);
});

test('explicit scope refuses phantom current Builds, invalid revisions and mixed physical provenance',()=>{
  const value=wire[0],physicalOwner=owner(value.scope);
  for(const mutate of [
    x=>x.contextId='stale',x=>x.suPath='invented',x=>x.sourceKind='project-all-plots',
    x=>x.revision='01',x=>x.revision='9223372036854775808',x=>x.fields=null,x=>x.fields=['Zone','zone'],
    x=>x.fields=['bad-name'],x=>x.build=null,x=>x.build.projectTuples.pop(),
    x=>x.build.approvalHash=['a'.repeat(64)],x=>x.build.approvalHash='A'.repeat(64),x=>x.extra=true,
    x=>x.sourceKind='selected-list'
  ]) {
    const changed=structuredClone(value.scope);mutate(changed);
    assert.throws(()=>scopeClient.validateReportScope(changed,physicalOwner,true));
  }
  const selected={...structuredClone(value.scope),sourceKind:'selected-list',build:null,buildCurrent:false,fields:['MissingField']};
  assert.equal(scopeClient.validateReportScope(selected,physicalOwner,true),selected);
  assert.throws(()=>scopeClient.currentProjectScope(selected,physicalOwner,true),/current/);
  const stale={...structuredClone(value.scope),buildCurrent:false};
  assert.equal(scopeClient.validateReportScope(stale,physicalOwner,true),stale);
  assert.throws(()=>scopeClient.currentProjectScope(stale,physicalOwner,true),/current/);
  assert.throws(()=>scopeClient.validateReportScope(wire[1].scope,{...owner(wire[1].scope),su:'Selected',suPath:'C:\\external.db'},true));
});

test('field catalogue reuses literal duplicate/type guards and retains unavailable correction drafts',()=>{
  const value=wire[0],physicalOwner=owner(value.scope);
  for(const mutate of [
    x=>x.revision='4',x=>x.sourceKind='selected-list',x=>x.projectPath='different',x=>x.definitions=null,
    x=>{x.definitions[0].type=x.definitions[0].declaredType;delete x.definitions[0].declaredType},
    x=>x.definitions[1].declaredType='REAL',x=>x.definitions[1].available=false,
    x=>x.definitions.push({...x.definitions[1],table:'Admin',name:'zone'})
  ]) {
    const changed=structuredClone(value.catalogue);mutate(changed);
    assert.throws(()=>scopeClient.validateProjectFieldCatalogue(changed,value.scope,physicalOwner,true));
  }
  const catalogue=structuredClone(value.catalogue),scope={...structuredClone(value.scope),fields:['MissingField'],build:null,buildCurrent:false};
  catalogue.definitions.push({table:'Admin',name:'Uncalibrated','declaredType':'REAL',available:false,reason:'uncalibrated physical type'});
  assert.equal(scopeClient.validateProjectFieldCatalogue(catalogue,scope,physicalOwner,true),catalogue);
  assert.match(selection.selectedUnitFieldError(scope.fields,catalogue.definitions),/unavailable/);
});

test('options, preview and preference wrappers must match independently requested complete Build',()=>{
  const value=wire[0],physicalOwner=owner(value.scope);
  for(const [make,validate] of [
    [()=>options(value,0),scopeClient.validateProjectOptions],
    [()=>preview(value,0),scopeClient.validateProjectPreview],
    [()=>structuredClone(value.unitPreferences),scopeClient.validateProjectUnitPreferences]
  ]) {
    for(const mutate of [
      x=>x.scope.revision='4',x=>x.scope.build.approvalHash='d'.repeat(64),x=>x.scope.fields=['Plot'],
      x=>x.scope.build.projectTuples[0].siteUnit.text='changed',x=>x.scope.buildCurrent=false,x=>x.extra=true
    ]) {
      const changed=make();mutate(changed);
      assert.throws(()=>validate(changed,value.scope,physicalOwner,true));
    }
  }
  for(const mutate of [
    x=>x.preferences.values.unitGroups=0,x=>x.preferences.availability.siteUnitTable=true,
    x=>x.preferences.availability.none=true,x=>x.preferences.availability.selectedFields=false,
    x=>x.preferences.suPath='different',x=>x.preferences.values.extra=true
  ]) {
    const changed=structuredClone(value.unitPreferences);mutate(changed);
    assert.throws(()=>scopeClient.validateProjectUnitPreferences(changed,value.scope,physicalOwner,true));
  }
  const changed=options(value,0);changed.options.settings.unitFields=['PlotNumber'];
  assert.throws(()=>scopeClient.validateProjectOptions(changed,value.scope,physicalOwner,true),/settings/);
});

test('qualified output retains exact existing Build tuples rather than merely matching counts or plot names',()=>{
  const value=wire[2],physicalOwner=owner(value.scope);
  const changed=preview(value,2);
  changed.preview.report.projectUnits[0].siteUnit.text='replacement';
  changed.preview.report.units[0].code.text='replacement';
  changed.preview.report.diagnostics[1].identity='text:replacement';
  assert.doesNotThrow(()=>project.validateLongVegetationProjectPreview(changed.preview,physicalOwner,'project-selected-fields',['Zone'],true));
  assert.throws(()=>scopeClient.validateProjectPreview(changed,value.scope,physicalOwner,true),/complete original Build/);
  const missing=preview(wire[1],1);
  missing.preview.report.projectUnits.pop();missing.preview.report.units[0].projectUnitIndexes=[0];
  missing.preview.report.units[0].numPlots=1;
  missing.preview.report.diagnostics=missing.preview.report.diagnostics.filter(row=>row.code!=='project_unit_join_multiplicity');
  assert.doesNotThrow(()=>project.validateLongVegetationProjectPreview(missing.preview,owner(wire[1].scope),'project-all-plots',[],true));
  assert.throws(()=>scopeClient.validateProjectPreview(missing,wire[1].scope,owner(wire[1].scope),true),/complete original Build/);
});

test('project unit receipts preserve committed cancellation and exact no-op/refusal shapes without acknowledging away writes',()=>{
  const value=wire[0],physicalOwner=owner(value.scope);
  const cancelled=structuredClone(value.outcome);cancelled.outcome.errorMessage='context canceled';
  assert.equal(scopeClient.validateProjectUnitOutcome(cancelled,value.scope,physicalOwner,true,true),cancelled);
  const noop=structuredClone(value.outcome);noop.outcome.changed=false;noop.outcome.committed=false;
  assert.equal(scopeClient.validateProjectUnitOutcome(noop,value.scope,physicalOwner,false,true),noop);
  const failed={sourceKind:'',revision:'',outcome:{contextId:'owned',changed:false,committed:false,errorMessage:'disabled'}};
  assert.equal(scopeClient.validateProjectUnitOutcome(failed,value.scope,physicalOwner,true,true),failed);
  for(const mutate of [
    x=>x.sourceKind='project-all-plots',x=>x.revision='4',x=>x.outcome.contextId='different',
    x=>x.outcome.changed=false,x=>x.outcome.errorMessage=null,x=>x.outcome.extra=true
  ]) {
    const changed=structuredClone(value.outcome);mutate(changed);
    assert.throws(()=>scopeClient.validateProjectUnitOutcome(changed,value.scope,physicalOwner,true,true));
  }
  assert.throws(()=>scopeClient.validateProjectUnitOutcome(value.outcome,value.scope,physicalOwner,false,true));
  assert.throws(()=>scopeClient.validateProjectUnitOutcome(failed,value.scope,physicalOwner,'true',true));
});
