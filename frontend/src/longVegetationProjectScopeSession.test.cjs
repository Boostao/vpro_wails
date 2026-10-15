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
const report = loadTypeScript('longVegetationProjectReport.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./qualityEditor':quality
});
const guards = loadTypeScript('longVegetationProjectScope.ts', {
  './longVegetationProjectReport':report,'./longVegetationSelectedUnitSelection':selection,'./qualityEditor':quality
});
const moduleUnderTest = loadTypeScript('longVegetationProjectScopeSession.ts', {
  './longVegetationProjectReport':report,'./longVegetationProjectScope':guards,'./longVegetationSelectedUnitSelection':selection
});
const wire=JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectScopeWire.json'),'utf8'));
const owner={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'None',suPath:''};
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no});return {promise,resolve,reject}};
function fixture(initial={...structuredClone(wire[0].scope),sourceKind:'selected-list',revision:'0',fields:[],build:null,buildCurrent:false}) {
  let state=structuredClone(initial);
  const calls=[],invalidations=[];
  const port={
    scope:async()=>{calls.push(['scope']);return structuredClone(state)},
    catalogue:async()=>{calls.push(['catalogue']);return {...structuredClone(wire[0].catalogue),sourceKind:state.sourceKind,revision:state.revision}},
    switchScope:async request=>{
      calls.push(['switch',structuredClone(request)]);
      assert.equal(request.revision,state.revision);
      state={...state,sourceKind:request.sourceKind,revision:String(BigInt(state.revision)+1n),build:null,buildCurrent:false};
      return structuredClone(state);
    },
    fields:async request=>{
      calls.push(['fields',structuredClone(request)]);
      assert.equal(request.revision,state.revision);
      state={...state,fields:[...request.fields],revision:String(BigInt(state.revision)+1n),build:null,buildCurrent:false};
      return structuredClone(state);
    },
    build:async revision=>{
      calls.push(['build',revision]);assert.equal(revision,state.revision);
      state={...state,revision:String(BigInt(state.revision)+1n),build:structuredClone(wire[state.sourceKind==='project-all-plots'?1:0].scope.build),buildCurrent:true};
      return structuredClone(state);
    },
    validationYield:async()=>{}
  };
  const session=new moduleUnderTest.ProjectScopeSelectionSession(owner,port,true,()=>invalidations.push(calls.length));
  return {session,port,calls,invalidations,state:()=>structuredClone(state)};
}

test('scope session and registry stay default-off and freeze owner before constructing a port',()=>{
  assert.throws(()=>new moduleUnderTest.ProjectScopeSelectionSession(owner,{}),/disabled/);
  assert.throws(()=>moduleUnderTest.createProjectScopeSelectionRegistry(()=>({})),/disabled/);
  let calls=0;
  const registry=moduleUnderTest.createProjectScopeSelectionRegistry(()=>{calls++;return {}},true);
  assert.throws(()=>registry({...owner,suPath:'invented'}),/physical paths/);
  assert.equal(calls,0);
  const session=registry(owner);
  assert.equal(registry({...owner}),session);
  assert.throws(()=>registry({...owner,projectPath:'different'}),/changed physical/);
  assert.equal(calls,1);
});

test('initial failed/cancelled scope Load does not invent mutation uncertainty or trap native close',async()=>{
  const f=fixture();f.port.scope=async()=>{throw new Error('owned read failed')};
  await f.session.load();
  assert.equal(f.session.view().scope,null);
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.session.view().closeBlocked,false);
  assert.match(f.session.view().error,/no defaults/);
  const held=deferred();f.port.scope=()=>held.promise;
  const pending=f.session.load();f.session.cancel();
  held.resolve(f.state());await pending;
  assert.equal(f.session.view().scope,null);
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.session.view().closeBlocked,false);
});

test('ordered project field drafts survive retained registry/remount with live correction and Undo',async()=>{
  const f=fixture(),registry=moduleUnderTest.createProjectScopeSelectionRegistry(()=>f.port,true);
  const session=registry(owner);await session.load();
  await session.switchTo('project-selected-fields');
  session.edit(['MissingField']);
  const remounted=registry({...owner});
  assert.equal(remounted,session);
  assert.equal(remounted.view().dirty,true);
  assert.match(remounted.view().draftError,/unavailable/);
  assert.equal(remounted.view().closeBlocked,true);
  await remounted.load();
  assert.deepEqual([...remounted.view().fields],['MissingField']);
  assert.match(remounted.view().draftError,/unavailable/);
  await assert.rejects(()=>remounted.build(),/correct/);
  remounted.edit(['Zone']);assert.equal(remounted.view().draftError,'');
  remounted.undo();
  assert.deepEqual([...remounted.view().fields],[]);
  assert.equal(remounted.view().dirty,false);
});

test('source2 Build serializes exact draft/revision acknowledgements and invalidates sibling output before requests',async()=>{
  const f=fixture();await f.session.load();await f.session.switchTo('project-selected-fields');
  f.session.edit(['Zone']);await f.session.build();
  const view=f.session.view();
  assert.equal(view.scope.sourceKind,'project-selected-fields');
  assert.equal(view.scope.revision,'3');
  assert.equal(view.scope.build.projectTuples.length,4);
  assert.equal(view.outputReady,true);
  assert.equal(view.dirty,false);assert.equal(view.draftError,'');
  assert.deepEqual(f.calls.filter(call=>['switch','fields','build'].includes(call[0])),[
    ['switch',{revision:'0',sourceKind:'project-selected-fields'}],
    ['fields',{revision:'1',fields:['Zone']}],['build','2']
  ]);
  assert.deepEqual(f.invalidations,[0,2,4,4]);
  view.scope.fields[0]='mutated';view.scope.build.projectTuples[0].siteUnit.text='mutated';view.definitions[0].name='mutated';
  assert.equal(f.session.view().scope.fields[0],'Zone');
  assert.equal(f.session.view().scope.build.projectTuples[0].siteUnit.text,'Alpha');
  assert.equal(f.session.view().definitions[0].name,'PlotNumber');
});

test('source3 keeps inactive invalid source2 correction draft without using it as output or overwriting it',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();f.session.edit(['MissingField']);
  await f.session.switchTo('project-all-plots');await f.session.build();
  const view=f.session.view();
  assert.equal(view.scope.sourceKind,'project-all-plots');
  assert.deepEqual([...view.scope.fields],['Zone']);
  assert.deepEqual([...view.fields],['MissingField']);
  assert.match(view.draftError,/unavailable/);
  assert.equal(view.dirty,true);assert.equal(view.closeBlocked,true);assert.equal(view.outputReady,true);
  assert.equal(f.calls.some(call=>call[0]==='fields'),false);
  await f.session.switchTo('project-selected-fields');
  assert.deepEqual([...f.session.view().fields],['MissingField']);
  assert.equal(f.session.view().outputReady,false);
  f.session.undo();assert.deepEqual([...f.session.view().fields],['Zone']);assert.equal(f.session.view().draftError,'');
});

test('actual reactive DTOs detach before held validation and cannot mutate a published reviewed scope or catalogue',async()=>{
  const f=fixture(wire[0].scope),scope=proxy(structuredClone(wire[0].scope)),catalogue=proxy(structuredClone(wire[0].catalogue));
  assert.throws(()=>structuredClone(scope));
  f.port.scope=async()=>scope;f.port.catalogue=async()=>catalogue;
  const entered=deferred(),release=deferred();
  f.port.validationYield=async()=>{entered.resolve();await release.promise};
  const pending=f.session.load();await entered.promise;
  scope.fields[0]='changed';scope.build.projectTuples[0].siteUnit.text='changed';catalogue.definitions[0].name='changed';
  release.resolve();await pending;
  assert.equal(f.session.view().scope.fields[0],'Zone');
  assert.equal(f.session.view().scope.build.projectTuples[0].siteUnit.text,'Alpha');
  assert.equal(f.session.view().definitions[0].name,'PlotNumber');
});

test('cancel/remount/correction during awaited validation cannot publish an old successful Load',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();
  const entered=deferred(),release=deferred();
  f.port.validationYield=async()=>{entered.resolve();await release.promise};
  const pending=f.session.load();await entered.promise;
  f.session.cancel();f.session.edit(['MissingField']);
  release.resolve();await pending;
  assert.deepEqual([...f.session.view().fields],['MissingField']);
  assert.equal(f.session.view().dirty,true);
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.session.view().outputReady,false);
});

test('late successful session Build after Cancel retains uncertainty until owned Load without mutation replay',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();
  const original=f.port.build,held=deferred(),entered=deferred();
  f.port.build=async revision=>{const result=await original(revision);entered.resolve();await held.promise;return result};
  const pending=f.session.build();await entered.promise;
  f.session.cancel();f.session.edit(['MissingField']);
  held.resolve();await pending;
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.session.view().closeBlocked,true);
  assert.equal(f.session.view().scope.revision,'3');
  assert.deepEqual([...f.session.view().fields],['MissingField']);
  await assert.rejects(()=>f.session.build(),/correct/);
  await f.session.load();
  assert.equal(f.session.view().scope.revision,'5');
  assert.equal(f.session.view().authorityUnknown,false);
  assert.deepEqual([...f.session.view().fields],['MissingField']);
  assert.equal(f.calls.filter(call=>call[0]==='fields').length,1);
  assert.equal(f.calls.filter(call=>call[0]==='build').length,1);
});

test('Load cannot reconcile a cancelled source switch before the held backend mutation actually settles',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();
  const original=f.port.switchScope,entered=deferred(),release=deferred();
  f.port.switchScope=async request=>{entered.resolve();await release.promise;return original(request)};
  const pending=f.session.switchTo('project-all-plots');await entered.promise;
  f.session.cancel();await f.session.load();
  assert.equal(f.session.view().scope.sourceKind,'project-selected-fields');
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.session.view().outputReady,false);
  assert.equal(f.session.view().closeBlocked,true);
  assert.match(f.session.view().error,/still pending/);
  await assert.rejects(()=>f.session.switchTo('selected-list'),/unknown/);
  release.resolve();await pending;
  assert.equal(f.state().sourceKind,'project-all-plots');
  assert.equal(f.session.view().scope.sourceKind,'project-selected-fields');
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.session.view().outputReady,false);
  assert.equal(f.session.view().closeBlocked,true);
  await f.session.load();
  assert.equal(f.session.view().scope.sourceKind,'project-all-plots');
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.session.view().closeBlocked,false);
  assert.equal(f.calls.filter(call=>call[0]==='switch').length,1);
});

test('Load during a cancelled held field mutation cannot restore Build readiness or replay its following Build',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();
  const original=f.port.fields,entered=deferred(),release=deferred();
  f.port.fields=async request=>{entered.resolve();await release.promise;return original(request)};
  const pending=f.session.build();await entered.promise;
  f.session.cancel();await f.session.load();
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.session.view().outputReady,false);
  assert.equal(f.session.view().closeBlocked,true);
  release.resolve();await pending;
  assert.equal(f.state().revision,'4');
  assert.equal(f.session.view().scope.revision,'3');
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.calls.filter(call=>call[0]==='build').length,0);
  await f.session.load();
  assert.equal(f.session.view().scope.revision,'4');
  assert.equal(f.session.view().scope.build,null);
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.session.view().outputReady,false);
  assert.equal(f.calls.filter(call=>call[0]==='fields').length,1);
});

test('settlement during a held Load validation cannot turn its earlier scope observation into post-settlement reconciliation',async()=>{
  const f=fixture(wire[0].scope);await f.session.load();
  const original=f.port.switchScope,mutationEntered=deferred(),mutationRelease=deferred();
  f.port.switchScope=async request=>{mutationEntered.resolve();await mutationRelease.promise;return original(request)};
  const mutation=f.session.switchTo('project-all-plots');await mutationEntered.promise;
  f.session.cancel();
  const validationEntered=deferred(),validationRelease=deferred();
  f.port.validationYield=async()=>{validationEntered.resolve();await validationRelease.promise};
  const loading=f.session.load();await validationEntered.promise;
  mutationRelease.resolve();await mutation;
  assert.equal(f.state().sourceKind,'project-all-plots');
  validationRelease.resolve();await loading;
  assert.equal(f.session.view().scope.sourceKind,'project-selected-fields');
  assert.equal(f.session.view().authorityUnknown,true);
  assert.equal(f.session.view().outputReady,false);
  assert.equal(f.session.view().closeBlocked,true);
  assert.match(f.session.view().error,/settled during/);
  f.port.validationYield=async()=>{};
  await f.session.load();
  assert.equal(f.session.view().scope.sourceKind,'project-all-plots');
  assert.equal(f.session.view().authorityUnknown,false);
  assert.equal(f.calls.filter(call=>call[0]==='switch').length,1);
});

test('wrong session acknowledgements and catalogue errors require owned Load and never blindly continue Build',async()=>{
  for(const failure of ['fields','switch','catalogue']) {
    const f=fixture(wire[0].scope);await f.session.load();
    if(failure==='fields') {
      const original=f.port.fields;f.port.fields=async request=>({...await original(request),revision:'99'});
      await f.session.build();
      assert.equal(f.calls.filter(call=>call[0]==='build').length,0);
    } else if(failure==='switch') {
      const original=f.port.switchScope;f.port.switchScope=async request=>({...await original(request),fields:['changed']});
      await f.session.switchTo('project-all-plots');
    } else {
      f.port.catalogue=async()=>{throw new Error('catalogue cleanup failed')};
      await f.session.build();
    }
    assert.equal(f.session.view().authorityUnknown,true);
    assert.equal(f.session.view().outputReady,false);
    assert.equal(f.session.view().closeBlocked,true);
    assert.match(f.session.view().error,/unknown/);
  }
});

test('scope and Build revision exhaustion prevents partial field mutation; explicit None owner is frozen',async()=>{
  for(const revision of ['9223372036854775807','9223372036854775806']) {
    const f=fixture({...structuredClone(wire[0].scope),revision});await f.session.load();
    await assert.rejects(()=>f.session.build(),/exhausted/);
    assert.equal(f.calls.some(call=>call[0]==='fields'||call[0]==='build'),false);
    if(revision==='9223372036854775807') await assert.rejects(()=>f.session.switchTo('project-all-plots'),/exhausted/);
  }
  const f=fixture(wire[2].scope),physicalOwner={...owner,su:'Selected',suPath:'C:\\external.db'};
  f.port.catalogue=async()=>structuredClone(wire[2].catalogue);
  const session=new moduleUnderTest.ProjectScopeSelectionSession(physicalOwner,f.port,true);
  physicalOwner.su='None';physicalOwner.suPath='';
  await session.load();
  assert.equal(session.view().scope.su,'Selected');
  await assert.rejects(()=>session.switchTo('project-all-plots'),/None SU/);
});
