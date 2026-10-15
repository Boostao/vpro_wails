const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const {compileModule} = require('svelte/compiler');
const client = require('svelte/internal/client');
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
const selected = loadTypeScript('longVegetationSelectedUnitSelection.ts', {
  './googleEarthReview':earth,'./qualityEditor':quality,'./longVegetationReport':ordinary
});
const report = loadTypeScript('longVegetationProjectReport.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./qualityEditor':quality
});
const scope = loadTypeScript('longVegetationProjectScope.ts', {
  './longVegetationProjectReport':report,'./longVegetationSelectedUnitSelection':selected,'./qualityEditor':quality
});
const selection = loadTypeScript('longVegetationProjectScopeSession.ts', {
  './longVegetationProjectReport':report,'./longVegetationProjectScope':scope,'./longVegetationSelectedUnitSelection':selected
},{setTimeout});
const preferences = loadTypeScript('reportTitlePreferences.ts', {'./googleEarthReview':earth,'./qualityEditor':quality});
const ordinaryUnits = loadTypeScript('longVegetationUnitPreferences.ts', {'./googleEarthReview':earth,'./reportTitlePreferences':preferences});
const units = loadTypeScript('longVegetationProjectUnitSession.ts', {
  './longVegetationProjectReport':report,'./longVegetationProjectScope':scope,
  './longVegetationUnitPreferences':ordinaryUnits,'./reportTitlePreferences':preferences
},{setTimeout});
const publication = loadTypeScript('publicationSession.ts');
const lifeform = loadTypeScript('lifeformSummary.ts');
const ordinaryWorkbook = loadTypeScript('vegetationWorkbook.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./lifeformSummary':lifeform,'./publicationSession':publication
});
function compiledModule(filename,dependencies) {
  const source=ts.transpileModule(readFileSync(path.join(__dirname,filename),'utf8'), {
    compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}
  }).outputText;
  const compiled=compileModule(source,{filename:path.join(__dirname,filename),generate:'client'});
  assert.equal(compiled.warnings.length,0);
  const module={exports:{}};
  vm.runInNewContext(ts.transpileModule(compiled.js.code,{
    compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}
  }).outputText,{module,exports:module.exports,structuredClone,setTimeout,require(name) {
    if(name==='svelte/internal/client') return client;
    if(name in dependencies) return dependencies[name];
    throw new Error(`Unexpected workspace dependency ${name}`);
  }});
  return module.exports;
}
const workbook = compiledModule('longVegetationProjectWorkbook.svelte.ts', {
  './longVegetationProjectReport':report,'./longVegetationProjectScope':scope,'./lifeformSummary':lifeform,
  './vegetationWorkbook':ordinaryWorkbook,'./publicationSession':publication
});
const workspaceModule = compiledModule('longVegetationProjectWorkspace.svelte.ts', {
  './longVegetationProjectReport':report,'./longVegetationProjectScope':scope,
  './longVegetationProjectScopeSession':selection,'./longVegetationProjectUnitSession':units,
  './longVegetationProjectWorkbook.svelte':workbook,'./longVegetationReport':ordinary
});
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectWorkbookWire.json'),'utf8'));
const scopeWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectScopeWire.json'),'utf8'));
const physicalOwner = scope => Object.fromEntries(['contextId','project','projectPath','su','suPath'].map(key=>[key,scope[key]]));
const capabilities = {enabled:true,workbook:true,lifeform:false,strata:false,code:false,selectedField:false};
const deferred = () => {let resolve,reject; const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return {promise,resolve,reject};};
function fixture(index=0,saved=index===1?3:2) {
  const input=structuredClone(wire[index]);
  let state=structuredClone(input.review.scope),actual=saved;
  const calls=[],cancels=[];
  const clone=value=>client.proxy(structuredClone(value));
  const currentReview=()=>({...structuredClone(input.review),scope:structuredClone(state)});
  const ports={
    scope:{
      scope:async()=>{calls.push(['scope']);return clone(state);},
      catalogue:async()=>({...structuredClone(scopeWire[index].catalogue),sourceKind:state.sourceKind,
        revision:state.revision}),
      switchScope:async request=>{
        calls.push(['switch',structuredClone(request)]);
        state={...state,sourceKind:request.sourceKind,revision:String(BigInt(state.revision)+1n),build:null,buildCurrent:false};
        return clone(state);
      },
      fields:async request=>{
        calls.push(['fields',structuredClone(request)]);
        state={...state,fields:[...request.fields],revision:String(BigInt(state.revision)+1n),build:null,buildCurrent:false};
        return clone(state);
      },
      build:async revision=>{
        calls.push(['build',revision]);
        state={...state,revision:String(BigInt(state.revision)+1n),build:structuredClone(input.review.scope.build),buildCurrent:true};
        return clone(state);
      },
      validationYield:async()=>{},
    },
    options:async request=>{calls.push(['options',structuredClone(request)]);return clone({scope:state,
      options:{...physicalOwner(state),settings:input.review.preview.settings}});},
    preview:async request=>{calls.push(['preview',structuredClone(request)]);return clone({scope:state,preview:input.review.preview});},
    unitPreferences:async request=>{calls.push(['units',structuredClone(request)]);return clone({scope:state,
      preferences:{...physicalOwner(state),values:{unitGroups:actual},availability:{
        siteUnitTable:false,selectedFields:state.sourceKind==='project-selected-fields',none:state.sourceKind==='project-all-plots'}}});},
    saveUnits:async request=>{calls.push(['save',structuredClone(request)]);actual=request.proposed.unitGroups;return {
      sourceKind:state.sourceKind,revision:state.revision,outcome:{contextId:state.contextId,changed:true,committed:true,errorMessage:''}};},
    workbookReview:async request=>{calls.push(['review',structuredClone(request)]);return clone(currentReview());},
    publishWorkbook:async request=>{calls.push(['publish',structuredClone(request)]);return clone(input.outcome);},
    cancelScopeReads:()=>cancels.push('scope'),cancelUnitReads:()=>cancels.push('unit'),cancelReportReads:()=>cancels.push('report'),
    validationYield:async()=>{},
  };
  const workspace=new workspaceModule.ProjectReportWorkspace(physicalOwner(state),ports,capabilities);
  return {workspace,ports,calls,cancels,input,currentReview,state:()=>structuredClone(state)};
}
async function ready(value) {
  await value.workspace.selection.load();
  await value.workspace.units.load(value.workspace.fetchUnitPreferences,async()=>{});
  return value;
}

test('retained workspace loads actual saved units before original project options preview and workbook using strict full requests',async()=>{
  for(const index of [0,1,2]) {
    const value=fixture(index),workspace=value.workspace;
    await workspace.selection.load();
    await assert.rejects(workspace.loadOptions(),/actually saved/);
    await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
    assert.equal(workspace.view().outputReady,true);
    await workspace.loadOptions();
    assert.equal(workspace.view().settings.title,'Project units');
    await workspace.showPreview();
    assert.equal(workspace.view().preview.report.projectUnits.length,[4,2,1][index]);
    await workspace.reviewWorkbook();
    assert.equal(workspace.view().workbookReview.scope.build.projectTuples.length,[4,2,2][index]);
    for(const [kind,request] of value.calls.filter(([kind])=>['options','preview','units'].includes(kind))) {
      assert.deepEqual(request,{revision:'3',approvalHash:value.state().build.approvalHash},kind);
    }
    assert.deepEqual(value.calls.find(([kind])=>kind==='review')[1],value.input.readRequest);
    const detached=workspace.view();
    detached.workbookReview.scope.fields.push('not authoritative');
    assert.equal(workspace.view().workbookReview.scope.fields.includes('not authoritative'),false);
  }
});

test('saved mode mismatch requires one full scoped CAS Save and acknowledgement before output; dirty drafts keep scope switch unavailable',async()=>{
  const value=await ready(fixture(0,1)),workspace=value.workspace;
  assert.equal(workspace.view().outputReady,false);
  await assert.rejects(workspace.showPreview(),/actually saved/);
  workspace.units.editUnitGroups('2');
  assert.equal(workspace.view().scopeMutationHeld,true);
  await workspace.units.saveProject(workspace.saveUnitPreferences);
  assert.deepEqual(value.calls.find(([kind])=>kind==='save')[1],{
    revision:'3',approvalHash:value.state().build.approvalHash,expected:{unitGroups:1},proposed:{unitGroups:2}});
  assert.equal(workspace.view().closeBlocked,true);
  await assert.rejects(workspace.showPreview(),/actually saved/);
  workspace.units.acknowledge();
  assert.equal(workspace.view().outputReady,true);
  await workspace.showPreview();
  assert.equal(workspace.view().preview.report.units[0].numPlots,2);
});

test('every read result discarded after held validation Cancel or sibling invalidation cannot populate a remounted workspace',async()=>{
  for(const kind of ['loadOptions','showPreview','reviewWorkbook']) {
    const value=await ready(fixture()),workspace=value.workspace;
    const entered=deferred(),release=deferred();
    value.ports.validationYield=()=>{entered.resolve();return release.promise;};
    const operation=workspace[kind]();
    await entered.promise;
    assert.equal(workspace.view().readBusy,true);assert.equal(workspace.view().closeBlocked,true);
    workspace.cancelReportRead();
    assert.equal(workspace.view().readBusy,false);
    workspace.invalidate();
    release.resolve();
    await operation;
    assert.equal(workspace.view().settings,null);
    assert.equal(workspace.view().preview,null);
    assert.equal(workspace.view().workbookReview,null);
    assert.equal(workspace.view().error,'');
    assert.ok(value.cancels.includes('report'));
  }
});

test('scope Build holds Unit reads and invalidates output and approval without erasing saved drafts',async()=>{
  const value=await ready(fixture()),workspace=value.workspace;
  await workspace.showPreview();
  const pending=deferred(),original=value.ports.scope.build;
  value.ports.scope.build=async revision=>{const result=await original(revision);await pending.promise;return result;};
  const operation=workspace.selection.build();
  await new Promise(resolve=>setTimeout(resolve,0));
  assert.equal(workspace.view().unitOperationHeld,true);
  assert.equal(workspace.view().preview,null);
  const calls=value.calls.filter(([kind])=>kind==='units').length;
  await assert.rejects(workspace.units.load(workspace.fetchUnitPreferences,async()=>{}),/Finish project scope/);
  assert.equal(value.calls.filter(([kind])=>kind==='units').length,calls);
  pending.resolve();
  await operation;
  assert.equal(workspace.view().units.saved.unitGroups,2);
  assert.equal(workspace.view().units.readReady,false);
  assert.equal(workspace.view().scope.scope.revision,'5');
  await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
  assert.equal(workspace.view().outputReady,true);
});

test('pending Unit Save prevents scope observation mutation and workbook reads without cancelling or replaying it',async()=>{
  const value=await ready(fixture(0,1)),workspace=value.workspace;
  workspace.units.editUnitGroups('2');
  const pending=deferred(),original=value.ports.saveUnits;
  value.ports.saveUnits=async request=>{const result=await original(request);await pending.promise;return result;};
  const operation=workspace.units.saveProject(workspace.saveUnitPreferences);
  assert.equal(workspace.view().scopeOperationHeld,true);
  assert.equal(workspace.view().scopeMutationHeld,true);
  assert.equal(workspace.view().closeBlocked,true);
  const reads=value.calls.filter(([kind])=>kind==='scope').length;
  await assert.rejects(workspace.selection.load(),/Finish or acknowledge/);
  assert.equal(value.calls.filter(([kind])=>kind==='scope').length,reads);
  await assert.rejects(workspace.reviewWorkbook(),/actually saved/);
  workspace.cancelReportRead();
  assert.equal(workspace.view().units.busy,true);
  pending.resolve();await operation;
  assert.equal(value.calls.filter(([kind])=>kind==='save').length,1);
  assert.equal(workspace.view().units.projectReceipts.length,1);
  workspace.units.acknowledge();
  await workspace.selection.load();
  await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
  assert.equal(workspace.view().outputReady,true);
});

test('cancelled scope mutation remains held until actual settlement and new owned Load; no options dispatch on stale authority',async()=>{
  const value=await ready(fixture()),workspace=value.workspace;
  const pending=deferred(),original=value.ports.scope.switchScope;
  value.ports.scope.switchScope=async request=>{const result=await original(request);await pending.promise;return result;};
  const operation=workspace.selection.switchTo('selected-list');
  workspace.cancelScopeReads();workspace.selection.cancel();
  assert.equal(workspace.view().scope.authorityUnknown,true);assert.equal(workspace.view().closeBlocked,true);
  await workspace.selection.load();
  assert.equal(workspace.view().scope.authorityUnknown,true);
  await assert.rejects(workspace.loadOptions(),/actually saved/);
  pending.resolve();await operation;
  await workspace.selection.load();
  assert.equal(workspace.view().projectActive,false);
  assert.equal(workspace.view().scope.authorityUnknown,false);
  assert.equal(value.calls.filter(([kind])=>kind==='switch').length,1);
  assert.equal(value.calls.filter(([kind])=>kind==='options').length,0);
});

test('publication retains complete reviewed intent and committed warning after Cancel while holding scope Unit and context disposal',async()=>{
  const value=await ready(fixture()),workspace=value.workspace;
  await workspace.reviewWorkbook();
  const pending=deferred();
  value.ports.publishWorkbook=request=>{value.calls.push(['publish',structuredClone(request)]);return pending.promise;};
  const operation=workspace.publishWorkbook(value.input.exportRequest.destination);
  assert.equal(workspace.view().publication.busy,true);
  assert.equal(workspace.view().scopeOperationHeld,true);assert.equal(workspace.view().unitMutationHeld,true);
  workspace.cancelReportRead();
  assert.equal(workspace.view().publication.busy,true);
  assert.throws(()=>workspace.workbook.acknowledge(),/cannot be cancelled/);
  pending.resolve({...value.input.outcome,status:'published-with-errors',errorMessage:'linked then cleanup failed'});
  await operation;
  assert.deepEqual(value.calls.find(([kind])=>kind==='publish')[1],value.input.exportRequest);
  assert.equal(workspace.view().workbookReview,null);
  assert.equal(workspace.view().publication.outcome.sha256,value.input.review.workbookSHA256);
  assert.equal(workspace.view().closeBlocked,true);
  workspace.workbook.acknowledge();
  assert.equal(workspace.view().publication.outcome.status,'published-with-errors');
  await assert.rejects(workspace.publishWorkbook(value.input.exportRequest.destination),/explicit original/);
  assert.equal(value.calls.filter(([kind])=>kind==='publish').length,1);
});

test('invalid wire and disabled grouping/workbook capabilities never silently populate output or invoke publication',async()=>{
  const value=await ready(fixture()),workspace=value.workspace;
  value.ports.options=async()=>null;
  await workspace.loadOptions();assert.match(workspace.view().error,/unavailable/);assert.equal(workspace.view().settings,null);
  value.ports.preview=async()=>({...value.input.review,scope:value.state(),preview:{...value.input.review.preview,
    report:{...value.input.review.preview.report,sourceKind:'selected-list'}}});
  await workspace.showPreview();assert.match(workspace.view().error,/unavailable/);assert.equal(workspace.view().preview,null);
  for(const mutate of [x=>x.grouping='lifeform',x=>x.grouping='strata',x=>x.showSpeciesCode=true,
    x=>x.selectedField='Species']) {
    const settings=structuredClone(value.input.review.preview.settings);mutate(settings);
    value.ports.options=async()=>({scope:value.state(),options:{...physicalOwner(value.state()),settings}});
    await workspace.loadOptions();
    assert.match(workspace.view().error,/unavailable/);
    assert.equal(workspace.view().settings,null);
  }
  const ports=fixture().ports;
  const unavailable=new workspaceModule.ProjectReportWorkspace(physicalOwner(wire[0].review.scope),ports,{...capabilities,workbook:false});
  await unavailable.selection.load();await unavailable.units.load(unavailable.fetchUnitPreferences,async()=>{});
  await assert.rejects(unavailable.reviewWorkbook(),/independently disabled/);
  assert.equal(unavailable.view().workbookReview,null);
});

test('unknown Unit Save keeps full retained draft receipt and owner holds through acknowledgement and corrected recovery Load',async()=>{
  const value=await ready(fixture(0,1)),workspace=value.workspace;
  workspace.units.editUnitGroups('2');
  value.ports.saveUnits=()=>Promise.reject(new Error('transport disappeared'));
  await workspace.units.saveProject(workspace.saveUnitPreferences);
  assert.equal(workspace.view().units.projectReceipts[0].kind,'unknown');
  assert.equal(workspace.view().closeBlocked,true);
  assert.equal(workspace.view().scopeMutationHeld,true);
  workspace.units.acknowledge();
  assert.equal(workspace.view().units.authorityUnknown,true);
  await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
  assert.equal(workspace.view().units.saved.unitGroups,1);
  assert.equal(workspace.view().units.draft.unitGroups,'2');
  assert.equal(workspace.view().units.authorityUnknown,false);
  assert.equal(workspace.view().units.projectReceipts.length,1);
  assert.equal(workspace.view().closeBlocked,true);
  assert.equal(value.calls.filter(([kind])=>kind==='save').length,0);
  workspace.units.undo();
  assert.equal(workspace.view().scopeMutationHeld,false);
  assert.equal(workspace.view().outputReady,false);
});

test('default-off immutable per-context registry never borrows another owner or changed capability through remount',async()=>{
  const value=fixture();
  assert.throws(()=>new workspaceModule.ProjectReportWorkspace(physicalOwner(value.state()),value.ports,{...capabilities,enabled:false}),/disabled/);
  assert.throws(()=>workspaceModule.createProjectReportWorkspaceRegistry(()=>value.ports,{...capabilities,enabled:false}),/disabled/);
  const config={...capabilities},registry=workspaceModule.createProjectReportWorkspaceRegistry(()=>value.ports,config);
  config.workbook=false;
  const owner=physicalOwner(value.state()),workspace=registry(owner);
  assert.ok(workspace.workbook);
  assert.equal(registry({...owner}),workspace);
  await workspace.selection.load();await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
  workspace.units.editUnitGroups('bad');
  assert.equal(registry({...owner}).view().units.draft.unitGroups,'bad');
  assert.equal(registry({...owner}).view().closeBlocked,true);
  assert.throws(()=>registry({...owner,projectPath:'different'}),/identity changed/);
  assert.throws(()=>registry({...owner,suPath:'phantom'}),/physical paths/);
});

function sdkRegistry(environment,bindings) {
  class RuntimeError extends Error {}
  class CancelledRejectionError extends Error {}
  const reads=loadTypeScript('readRequests.ts',{'@wailsio/runtime':{Call:{RuntimeError},CancelledRejectionError}});
  const module={exports:{}};
  const code=ts.transpileModule(readFileSync(path.join(__dirname,'longVegetationProjectWorkspaces.ts'),'utf8')
    .replaceAll('import.meta.env','__environment'),{
    compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}
  }).outputText;
  vm.runInNewContext(code,{module,exports:module.exports,__environment:environment,require(name) {
    if(name==='../bindings/github.com/boostao/vpro-wails') return bindings;
    if(name==='./readRequests') return reads;
    if(name==='./longVegetationProjectWorkspace.svelte') return workspaceModule;
    throw new Error(`Unexpected registry dependency ${name}`);
  }});
  return module.exports.projectReportWorkspace;
}

test('public SDK registry routes only distinct project APIs using production-Go full requests and one immutable client capability',async()=>{
  for(const flag of [undefined,'false','1',true]) {
    assert.equal(sdkRegistry({VITE_LONG_VEGETATION_PROJECT_SCOPE:flag},{}),null);
  }
  const input=structuredClone(wire[0]),owner=physicalOwner(input.review.scope),calls=[],cancelled=[];
  let saved=1;
  const cancellable=(operation,value)=>{
    const promise=Promise.resolve(value);
    promise.cancel=()=>cancelled.push(operation);
    return promise;
  };
  const scopeService={
    GetLongVegetationReportScope:context=>{assert.equal(context,owner.contextId);return cancellable('scope',input.review.scope);},
    GetLongVegetationProjectFieldDefinitions:()=>cancellable('catalogue',{...scopeWire[0].catalogue,revision:input.review.scope.revision}),
    GetLongVegetationProjectUnitPreferences:(context,request)=>{
      calls.push(['units',context,JSON.parse(request)]);
      return cancellable('units',{scope:input.review.scope,preferences:{...owner,values:{unitGroups:saved},
        availability:{siteUnitTable:false,selectedFields:true,none:false}}});
    },
    SaveLongVegetationProjectUnits:(context,request)=>{
      calls.push(['save',context,JSON.parse(request)]);saved=JSON.parse(request).proposed.unitGroups;
      return cancellable('save',{sourceKind:input.review.scope.sourceKind,revision:input.review.scope.revision,
        outcome:{contextId:context,changed:true,committed:true,errorMessage:''}});
    },
    GetLongVegetationProjectOptions:(context,request)=>{
      calls.push(['options',context,JSON.parse(request)]);
      return cancellable('options',{scope:input.review.scope,options:{...owner,settings:input.review.preview.settings}});
    },
    PreviewLongVegetationProject:(context,request)=>{
      calls.push(['preview',context,JSON.parse(request)]);
      return cancellable('preview',{scope:input.review.scope,preview:input.review.preview});
    },
  };
  const workbookService={
    GetReview:(context,request)=>{calls.push(['review',context,JSON.parse(request)]);return cancellable('review',input.review);},
    ExportReviewed:(context,request)=>{calls.push(['publish',context,JSON.parse(request)]);return cancellable('publish',input.outcome);},
  };
  const environment={VITE_LONG_VEGETATION_PROJECT_SCOPE:'true',VITE_LONG_VEGETATION_WORKBOOK:'true'};
  const registry=sdkRegistry(environment,{
    LongVegetationReportScopeService:scopeService,LongVegetationProjectWorkbookService:workbookService
  });
  environment.VITE_LONG_VEGETATION_PROJECT_SCOPE='false';environment.VITE_LONG_VEGETATION_WORKBOOK='false';
  const workspace=registry(owner);
  await workspace.selection.load();await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
  workspace.units.editUnitGroups('2');
  await workspace.units.saveProject(workspace.saveUnitPreferences);workspace.units.acknowledge();
  await workspace.loadOptions();await workspace.showPreview();await workspace.reviewWorkbook();
  await workspace.publishWorkbook(input.exportRequest.destination);
  assert.deepEqual(calls.map(([kind])=>kind),['units','save','options','preview','review','publish']);
  assert.ok(calls.every(([,context])=>context===owner.contextId));
  assert.deepEqual(calls.find(([kind])=>kind==='save')[2],{revision:'3',approvalHash:input.review.scope.build.approvalHash,
    expected:{unitGroups:1},proposed:{unitGroups:2}});
  assert.deepEqual(calls.find(([kind])=>kind==='review')[2],input.readRequest);
  assert.deepEqual(calls.find(([kind])=>kind==='publish')[2],input.exportRequest);
  assert.equal(cancelled.includes('save'),false);assert.equal(cancelled.includes('publish'),false);
});

test('actual public SDK read ownership cancels getter requests but never an issued scope mutation',async()=>{
  const input=structuredClone(wire[0]),owner=physicalOwner(input.review.scope),cancelled=[],mutation=deferred();
  let state=structuredClone(input.review.scope);
  const cancellable=(name,promise)=>Object.assign(promise,{cancel:()=>cancelled.push(name)});
  const service={
    GetLongVegetationReportScope:()=>cancellable('scope',Promise.resolve(state)),
    GetLongVegetationProjectFieldDefinitions:()=>cancellable('catalogue',Promise.resolve({
      ...scopeWire[0].catalogue,sourceKind:state.sourceKind,revision:state.revision})),
    SetLongVegetationReportScope:(context,raw)=>{
      const request=JSON.parse(raw);
      assert.equal(context,owner.contextId);
      assert.deepEqual(request,{revision:'3',sourceKind:'selected-list'});
      state={...state,sourceKind:'selected-list',revision:'4',build:null,buildCurrent:false};
      return cancellable('switch',mutation.promise);
    },
  };
  const registry=sdkRegistry({VITE_LONG_VEGETATION_PROJECT_SCOPE:'true'},{
    LongVegetationReportScopeService:service,LongVegetationProjectWorkbookService:{}
  });
  const workspace=registry(owner);
  await workspace.selection.load();
  const operation=workspace.selection.switchTo('selected-list');
  workspace.cancelScopeReads();workspace.selection.cancel();
  assert.equal(cancelled.includes('switch'),false);
  mutation.resolve(state);await operation;
  await workspace.selection.load();
  assert.equal(workspace.view().projectActive,false);
  assert.equal(workspace.view().closeBlocked,false);
  const getter=deferred();
  service.GetLongVegetationReportScope=()=>cancellable('scope',getter.promise);
  const reading=workspace.selection.load();
  workspace.cancelScopeReads();workspace.selection.cancel();
  assert.equal(cancelled.includes('scope'),true);
  getter.resolve(state);await reading;
  assert.equal(cancelled.includes('switch'),false);
});

test('sibling invalidation revokes held scope and Unit getter generations and requires fresh owned recovery',async()=>{
  for(const kind of ['scope','unit']) {
    const value=await ready(fixture()),workspace=value.workspace;
    const entered=deferred(),release=deferred(),hold=()=>{entered.resolve();return release.promise;};
    let operation;
    if(kind==='scope') {
      value.ports.scope.validationYield=hold;
      operation=workspace.selection.load();
    } else operation=workspace.units.load(workspace.fetchUnitPreferences,hold);
    await entered.promise;
    workspace.invalidate();
    assert.equal(workspace.view().sourceRevalidationRequired,true);
    assert.equal(workspace.view().outputReady,false);
    assert.ok(value.cancels.includes('scope'));assert.ok(value.cancels.includes('unit'));
    release.resolve();await operation;
    assert.equal(workspace.view().sourceRevalidationRequired,true);
    assert.equal(workspace.view().units.readReady,false);
    await assert.rejects(workspace.loadOptions(),/actually saved/);
    value.ports.scope.validationYield=async()=>{};
    await workspace.selection.load();
    assert.equal(workspace.view().sourceRevalidationRequired,false);
    assert.equal(workspace.view().outputReady,false);
    await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
    assert.equal(workspace.view().outputReady,true);
  }
});

test('operation preflight prevents scope and Unit transitions during a report read before notifications can erase holds',async()=>{
  const value=await ready(fixture()),workspace=value.workspace;
  const pending=deferred();
  value.ports.preview=()=>pending.promise;
  const reading=workspace.showPreview();
  const calls=value.calls.length,receipts=workspace.view().units.projectReceipts.length;
  assert.equal(workspace.view().readBusy,true);
  await assert.rejects(workspace.selection.load(),/Finish or acknowledge/);
  await assert.rejects(workspace.selection.build(),/finish or acknowledge/);
  await assert.rejects(workspace.selection.switchTo('selected-list'),/finish or acknowledge/);
  await assert.rejects(workspace.units.load(workspace.fetchUnitPreferences,async()=>{}),/Finish project scope/);
  await assert.rejects(workspace.units.saveProject(workspace.saveUnitPreferences),/Finish or correct/);
  assert.equal(value.calls.length,calls);
  assert.equal(workspace.view().readBusy,true);
  assert.equal(workspace.view().units.projectReceipts.length,receipts);
  assert.equal(workspace.view().scope.authorityUnknown,false);
  workspace.cancelReportRead();
  pending.resolve({scope:value.state(),preview:value.input.review.preview});await reading;
});

test('All plots Unit Save3 ignores inactive dirty invalid source2 fields while retaining their strict close hold',async()=>{
  const value=await ready(fixture(1,1)),workspace=value.workspace;
  workspace.selection.edit(['MissingField','Invalid-Field']);
  assert.equal(workspace.view().scope.dirty,true);assert.ok(workspace.view().scope.draftError);
  assert.equal(workspace.view().scope.outputReady,true);
  assert.equal(workspace.view().unitMutationHeld,false);
  assert.equal(workspace.view().closeBlocked,true);
  workspace.units.editUnitGroups('3');
  await workspace.units.saveProject(workspace.saveUnitPreferences);
  assert.equal(value.calls.filter(([kind])=>kind==='save').length,1);
  assert.equal(workspace.view().units.projectReceipts.length,1);
  assert.equal(workspace.view().units.projectReceipts[0].kind,'known');
  workspace.units.acknowledge();
  assert.equal(workspace.view().outputReady,true);
  assert.equal(workspace.view().closeBlocked,true);
  assert.deepEqual([...workspace.view().scope.fields],['MissingField','Invalid-Field']);
  await workspace.showPreview();
  assert.equal(workspace.view().preview.report.sourceKind,'project-all-plots');
});

test('external changes during a pending Save or its receipt revoke actual Unit freshness despite identical recovered scope',async()=>{
  for(const phase of ['pending','receipt']) {
    const value=await ready(fixture(0,1)),workspace=value.workspace;
    workspace.units.editUnitGroups('2');
    const release=deferred(),committed=deferred(),original=value.ports.saveUnits;
    value.ports.saveUnits=async request=>{
      const result=await original(request);
      committed.resolve();
      if(phase==='pending') await release.promise;
      return result;
    };
    const operation=workspace.units.saveProject(workspace.saveUnitPreferences);
    await committed.promise;
    if(phase==='receipt') await operation;
    value.ports.unitPreferences=async()=>({scope:value.state(),preferences:{
      ...physicalOwner(value.state()),values:{unitGroups:1},availability:{siteUnitTable:false,selectedFields:true,none:false}}});
    workspace.invalidate();
    assert.equal(workspace.view().units.readReady,false);
    if(phase==='pending') {assert.equal(workspace.view().units.busy,true);release.resolve();await operation;}
    assert.equal(workspace.view().units.projectReceipts.length,1);
    assert.equal(workspace.view().units.projectReceipts[0].kind,'known');
    workspace.units.acknowledge();
    await workspace.selection.load();
    assert.equal(workspace.view().sourceRevalidationRequired,false);
    assert.equal(workspace.view().units.scope.revision,'3');
    assert.equal(workspace.view().units.readReady,false);
    assert.equal(workspace.view().outputReady,false);
    await assert.rejects(workspace.showPreview(),/actually saved/);
    await workspace.units.load(workspace.fetchUnitPreferences,async()=>{});
    assert.equal(workspace.view().units.saved.unitGroups,1);
    assert.equal(workspace.view().units.draft.unitGroups,'1');
    assert.equal(workspace.view().outputReady,false);
    assert.equal(value.calls.filter(([kind])=>kind==='save').length,1);
  }
});
