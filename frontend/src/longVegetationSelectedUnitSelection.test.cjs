const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const {proxy} = require('svelte/internal/client');
const {render} = require('svelte/server');
const {loadTypeScript,serverComponent} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const cells = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names = loadTypeScript('reportUnitNames.ts', {'./projectMetadataRestore':cells});
const env = loadTypeScript('longEnvironmentReport.ts', {'./projectMetadataRestore':cells,'./reportUnitNames':names});
const report = loadTypeScript('longVegetationReport.ts', {
  './projectMetadataRestore':cells,'./longEnvironmentReport':env,'./qualityEditor':quality,'./reportUnitNames':names
});
const locations = loadTypeScript('plotLocationReview.ts', {'./projectMetadataRestore':cells});
const earth = loadTypeScript('googleEarthReview.ts', {'./projectMetadataRestore':cells,'./qualityEditor':quality,'./plotLocationReview':locations});
const selection = loadTypeScript('longVegetationSelectedUnitSelection.ts', {
  './googleEarthReview':earth,'./qualityEditor':quality,'./longVegetationReport':report
});
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationSelectedUnitsWire.json'),'utf8'));
const selectorWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationSelectedUnitSelectionWire.json'),'utf8'));
const owner={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'Selected',suPath:'C:\\external.db'};
const field=(name,type='TEXT',available=true,reason='')=>({table:'Env',name,declaredType:type,available,reason});
const review=(revision='0',fields=[],built=false)=>({...owner,revision,fields,
  definitions:[field('Zone'),field('SubZone'),field('Date','REAL',false,'uncalibrated type')],
  build:built?{approvalHash:'a'.repeat(64),occurrences:structuredClone(wire.report.selectedUnits)}:null,
  buildCurrent:built
});
const ready=async()=>{const session=new selection.SelectedUnitSelectionSession(owner);await session.load(async()=>review());return session};
test('strict selector boundary accepts the shared production-Go Build/DTO wire, never metadata type aliases',()=>{
  const actual=selection.validateSelectedUnitSelection(proxy(structuredClone(selectorWire)),owner);
  assert.deepEqual(structuredClone(actual),selectorWire);
  assert.deepEqual(actual.build.occurrences.map(row=>structuredClone(row)),wire.report.selectedUnits);
  const invalid=structuredClone(selectorWire);
  invalid.definitions[0].type=invalid.definitions[0].declaredType;delete invalid.definitions[0].declaredType;
  assert.throws(()=>selection.validateSelectedUnitSelection(invalid,owner));
});
test('selector strict review retains detached actual proxies, unavailable definitions and original nullable joins',()=>{
  const input=review('2',['Zone'],true),reactive=proxy(structuredClone(input));
  assert.throws(()=>structuredClone(reactive));
  const value=selection.validateSelectedUnitSelection(reactive,owner);
  assert.deepEqual(structuredClone(value),input);
  reactive.fields[0]='changed';reactive.build.occurrences[0].siteUnit.text='changed';
  assert.equal(value.fields[0],'Zone');assert.equal(value.build.occurrences[0].siteUnit.text,'Alpha');
  for(const mutate of [
    x=>x.contextId='foreign',x=>x.projectPath='foreign',x=>x.revision='01',x=>x.revision=-1,
    x=>x.revision='9223372036854775808',x=>x.fields=null,x=>x.fields=['\ud800'],
    x=>x.definitions=null,x=>x.definitions[0].available='true',x=>x.definitions[0].declaredType='REAL',
    x=>x.definitions[0].reason='invented',x=>x.definitions.push(field('zone')),
    x=>x.buildCurrent=true,x=>x.extra=true
  ]) {const invalid=review();mutate(invalid);assert.throws(()=>selection.validateSelectedUnitSelection(invalid,owner),undefined,String(mutate));}
  for(const mutate of [
    x=>x.build.approvalHash='A'.repeat(64),x=>x.build.occurrences.pop(),
    x=>x.build.occurrences[0].adminRowId=null,x=>x.fields=['Date'],x=>x.fields=[],
    x=>delete x.build.occurrences[0].membershipId,x=>x.build.occurrences[0].siteUnit.text=''
  ]) {const invalid=review('2',['Zone'],true);mutate(invalid);assert.throws(()=>selection.validateSelectedUnitSelection(invalid,owner));}
  const drift=review('2',['removed'],true);drift.buildCurrent=false;
  assert.equal(selection.validateSelectedUnitSelection(drift,owner).buildCurrent,false);
});
test('failed/cancelled readonly initial Load does not invent uncertain selection or trap native close; prior unknown Build stays held',async()=>{
  const session=new selection.SelectedUnitSelectionSession(owner);
  await session.load(async()=>{throw new Error('selector backend disabled')});
  assert.equal(session.view().authorityUnknown,false);assert.equal(session.view().dirty,false);
  assert.equal(session.view().review,null);assert.match(session.view().error,/disabled/);
  let release;
  const pending=session.load(()=>new Promise(done=>{release=done}));
  session.cancel();release(review());await pending;
  assert.equal(session.view().authorityUnknown,false);assert.equal(session.view().review,null);
  await session.load(async()=>review());session.edit(['Zone']);
  await session.build(async()=>{throw new Error('lost Set acknowledgement')},async()=>review('2',['Zone'],true));
  assert.equal(session.view().authorityUnknown,true);
  await session.load(async()=>{throw new Error('read failed')});
  assert.equal(session.view().authorityUnknown,true);
  const retry=session.load(()=>new Promise(done=>{release=done}));
  session.cancel();release(review());await retry;
  assert.equal(session.view().authorityUnknown,true);
  await session.load(async()=>review('1',['Zone']));
  assert.equal(session.view().authorityUnknown,false);
});
test('ordered drafts survive remount, block malformed Build and keep correction/Undo live',async()=>{
  const registry=selection.createSelectedUnitSelectionRegistry(),session=registry(owner);
  await session.load(async()=>review());
  assert.equal(session.view().fields.length,0);
  for(const fields of [[],['Zone','zone'],['Date'],['absent'],['bad-name'],['\ud800']]) {
    session.edit(fields);assert.ok(session.view().draftError);assert.equal(registry(owner),session);
    let calls=0;await assert.rejects(session.build(async()=>{calls++},async()=>{calls++}));
    assert.equal(calls,0);session.edit(['SubZone','Zone']);assert.equal(session.view().draftError,'');
    session.undo();assert.equal(session.view().dirty,false);assert.equal(session.view().fields.length,0);
  }
  assert.throws(()=>registry({...owner,projectPath:'foreign'}));
});
test('explicit readonly draft/Build sequence validates exact revisions/ordered fields and retains immutable receipt',async()=>{
  const session=await ready();session.edit(['Zone']);let sent,revision;
  await session.build(async request=>{sent=request;return review('1',['Zone'])},
    async expected=>{revision=expected;return review('2',['Zone'],true)});
  assert.deepEqual(structuredClone(sent),{revision:'0',fields:['Zone']});assert.equal(revision,'1');
  assert.equal(session.view().review.buildCurrent,true);assert.equal(session.view().dirty,false);
  const view=session.view();view.review.build.occurrences[0].siteUnit.text='changed';
  assert.equal(session.view().review.build.occurrences[0].siteUnit.text,'Alpha');
  session.edit(['SubZone','Zone']);assert.equal(session.view().dirty,true);
  assert.equal(session.view().review.build.approvalHash,'a'.repeat(64));
  session.undo();assert.deepEqual(Array.from(session.view().fields),['Zone']);
});
test('cancelled late draft/Build reads cannot overwrite a remounted correction and require owned Load',async()=>{
  const registry=selection.createSelectedUnitSelectionRegistry(),session=registry(owner);
  await session.load(async()=>review());session.edit(['Zone']);let release;
  const pending=session.build(()=>new Promise(done=>{release=done}),async()=>review('2',['Zone'],true));
  assert.equal(session.view().busy,true);session.cancel();session.edit(['SubZone']);
  release(review('1',['Zone']));await pending;
  assert.equal(registry(owner),session);assert.deepEqual(Array.from(session.view().fields),['SubZone']);
  assert.equal(session.view().authorityUnknown,true);
  await assert.rejects(session.build(async()=>review('1',['SubZone']),async()=>review('2',['SubZone'],true)));
  await session.load(async()=>review('1',['Zone']));
  assert.deepEqual(Array.from(session.view().fields),['SubZone']);assert.equal(session.view().authorityUnknown,false);
  let approved,entered;
  const started=new Promise(done=>{entered=done});
  const late=session.build(async()=>review('2',['SubZone']),()=>new Promise(done=>{approved=done;entered()}));
  await started;
  session.cancel();session.edit(['Zone']);approved(review('3',['SubZone'],true));await late;
  assert.deepEqual(Array.from(session.view().fields),['Zone']);assert.equal(session.view().review.build,null);
});
test('failed or mismatched session acknowledgements require Load, not blind retry or repaired defaults',async()=>{
  for(const value of [null,review('0',['Zone']),review('1',['SubZone']),review('1',['Zone'],true)]) {
    const session=await ready();session.edit(['Zone']);
    await session.build(async()=>value,async()=>review('2',['Zone'],true));
    assert.ok(session.view().error);assert.equal(session.view().authorityUnknown,true);
    await assert.rejects(session.build(async()=>review('1',['Zone']),async()=>review('2',['Zone'],true)));
    await session.load(async()=>review());assert.equal(session.view().authorityUnknown,false);
  }
});
test('selector source grouping uses responsive Field List/Selected List and explicit disabled scalar diagnostics',async()=>{
  const session=await ready();session.edit(['Zone']);
  const source=readFileSync(path.join(__dirname,'LongVegetationSelectedUnitSelection.svelte'),'utf8');
  const component=serverComponent(source,'LongVegetationSelectedUnitSelection.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{},'./longVegetationSelectedUnitSelection':selection
  });
  const body=render(component,{props:{contextId:owner.contextId,selection:session,onChange(){}}}).body;
  assert.match(body,/Field List/);assert.match(body,/Selected List/);assert.match(body,/Build SU List/);
  assert.match(body,/uncalibrated type/);assert.match(body,/data-selected-unit-add="2" disabled/);
  assert.match(source,/minmax\(0, 1fr\)/);assert.match(source,/cancelAll\(\)/);
  assert.match(body,/Build is read-only/);assert.match(body,/does not save Unit Groups/);
});
test('selector changes invalidate held actual parent options/preview/workbook responses without replaying output',async()=>{
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  const actions=source.slice(source.indexOf('  function selectedUnitChanged()'),source.indexOf('  async function publishWorkbook()'));
  for(const method of ['options','show','reviewWorkbook']) {
    let release,cancelled=0,validated=0;
    const pending=new Promise(done=>{release=done});
    const context=vm.createContext({
      generation:0,busy:false,operationHeld:false,workbookBarrier:false,preferenceBarrier:false,
      settings:{},error:'',preview:null,workbookReview:null,selectedUnitView:null,
      contextId:owner.contextId,project:owner.project,projectPath:owner.projectPath,su:owner.su,suPath:owner.suPath,
      workbookEnabled:true,selectedUnitsEnabled:true,lifeformEnabled:false,strataEnabled:false,codeEnabled:false,
      projectActive:false,projectScopeKnown:true,projectWorkspace:null,
      selectedFieldEnabled:false,noneUnitsEnabled:false,preferences:null,statistics:null,
      reads:{track:value=>value,cancelAll(){cancelled++}},reportBusy(){},
      selectedUnitSelection:{view:()=>({dirty:true})},
      ContextService:{GetLongVegetationOptions:()=>pending,PreviewLongVegetation:()=>pending},
      VegetationWorkbookService:{GetReview:()=>pending},
      validateLongVegetationOptions(){validated++;return {settings:{}}},
      validateLongVegetationPreview(){validated++;return {settings:{}}},
      requireLongVegetationGrouping(){},
      workbookSession:{prepare(){validated++;return {preview:{settings:{}}}}},
    });
    vm.runInContext(ts.transpileModule(actions+'\nthis.actions={selectedUnitChanged,options,show,reviewWorkbook};',
      {compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText,context);
    const request=context.actions[method]();
    assert.equal(context.busy,true,method);
    context.actions.selectedUnitChanged();
    release({});await request;
    assert.equal(context.busy,false,method);
    assert.equal(context.settings,null,method);assert.equal(context.preview,null,method);assert.equal(context.workbookReview,null,method);
    assert.equal(validated,0,method);assert.equal(cancelled,2,method);assert.equal(context.generation,2,method);
  }
});
test('actual report module retains selector draft safety across remount with independent editor/consumer gates',async()=>{
    const registry=selection.createSelectedUnitSelectionRegistry(),session=registry(owner);
    await session.load(async()=>review());session.edit(['absent']);
    const child=serverComponent(readFileSync(path.join(__dirname,'LongVegetationSelectedUnitSelection.svelte'),'utf8'),
      'LongVegetationSelectedUnitSelection.svelte',{'../bindings/github.com/boostao/vpro-wails':{}});
    class Publication {view(){return {busy:false,blocked:false,error:'',outcome:null,requestedDestination:''}} subscribe(){return ()=>{}}}
    let created=0;
    const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
    const component=serverComponent(source
      .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_SELECTED_UNIT_SELECTION === 'true'/g,'true')
      .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g,'false'),'LongVegetationReport.svelte',{
      '../bindings/github.com/boostao/vpro-wails':{},
      './longVegetationSelectedUnitSelection':{createSelectedUnitSelectionRegistry(){created++;return registry}},
      './LongVegetationSelectedUnitSelection.svelte':{default:child},
      './longVegetationUnitPreferences':{},'./LongVegetationUnitPreferences.svelte':{default(){}},
      './longVegetationGroupPreferences':{},'./LongVegetationGroupPreferences.svelte':{default(){}},
      './longVegetationNamingPreferences':{},'./LongVegetationNamingPreferences.svelte':{default(){}},
      './longVegetationSelectedFieldPreferences':{},'./LongVegetationSelectedFieldPreferences.svelte':{default(){}},
      './longVegetationEnforcementPreferences':{},'./LongVegetationEnforcementPreferences.svelte':{default(){}},
      './longVegetationQualityNullPreferences':{},'./LongVegetationQualityNullPreferences.svelte':{default(){}},
      './longVegetationQualityMinimumPreferences':{},'./LongVegetationQualityMinimumPreferences.svelte':{default(){}},
      './longVegetationPreferences':{},'./longVegetationStatisticsPreferences':{},
      './longVegetationSwitchPreferences':{},'./LongVegetationSwitchPreferences.svelte':{default(){}},
      './longVegetationThresholdPreferences':{},'./LongVegetationThresholdPreferences.svelte':{default(){}},
      './longVegetationReport':{},'./longEnvironmentReport':{},
      './vegetationWorkbook':{VegetationWorkbookPublicationSession:Publication}
    });
    for(let mount=0;mount<2;mount++) {
      const body=render(component,{props:{...owner,onBusyChange(){}}}).body;
      assert.match(body,/Selected field absent is absent/);
      assert.match(body,/data-selected-unit-build="" disabled/);
      assert.match(body,/data-source-control="btnViewReport" disabled/);
      assert.deepEqual(Array.from(registry(owner).view().fields),['absent']);
    }
    assert.equal(created,1);
    assert.match(source,/const ordinaryUnitBarrier = \$derived\(selectedUnitBarrier/);
    assert.match(source,/const unitBarrier = \$derived\(ordinaryUnitBarrier \|\| projectBarrier\)/);
    assert.match(source,/const ordinaryUnitMutationHeld = \$derived\(selectedUnitMutationHeld/);
    assert.match(source,/const unitMutationHeld = \$derived\(ordinaryUnitMutationHeld \|\| projectMutationHeld\)/);
    assert.match(source,/onBusyChange\(busy \|\| workbookBarrier \|\| sharedPreferenceBarrier \|\| !!projectView\?\.closeBlocked\)/);
    assert.match(source,/operationHeld=\{projectBarrier \|\| selectedUnitBarrier/);
    assert.match(source,/settings\.unitGrouping === 'selected-fields' \? 2 : 1/g);
});
