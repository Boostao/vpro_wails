const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const {compile} = require('svelte/compiler');
const {render} = require('svelte/server');
const {loadTypeScript,serverComponent,componentFunctions} = require('./svelteTestHelpers.cjs');
const wire=JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectWorkbookWire.json'),'utf8'));
const scopeWire=JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectScopeWire.json'),'utf8'));
const source=filename=>readFileSync(path.join(__dirname,filename),'utf8');
const owner=scope=>Object.fromEntries(['contextId','project','projectPath','su','suPath'].map(key=>[key,scope[key]]));
const deferred=()=>{let resolve;const promise=new Promise(yes=>resolve=yes);return {promise,resolve};};

function workspaceFixture(index=0) {
  const input=structuredClone(wire[index]),physicalOwner=owner(input.review.scope),calls=[],listeners=new Set();
  const units={saved:{unitGroups:index===1?3:2},draft:{unitGroups:index===1?'3':'2'},initialized:true,busy:false,blocked:false,
    authorityUnknown:false,error:'',draftError:'',readError:'',readBusy:false,readReady:true,saveReady:true,closeBlocked:false,
    projectReceipts:[],scope:input.review.scope};
  const scope={scope:input.review.scope,definitions:scopeWire[index].catalogue.definitions,fields:input.review.scope.fields,
    dirty:false,draftError:'',error:'',busy:false,authorityUnknown:false,outputReady:true,closeBlocked:false};
  const state={scope,units,publication:{busy:false,blocked:false,error:'',outcome:null,requestedDestination:''},
    readBusy:false,error:'',settings:input.review.preview.settings,preview:input.review.preview,workbookReview:input.review,
    projectActive:true,outputReady:true,scopeOperationHeld:false,scopeMutationHeld:false,
    unitOperationHeld:false,unitMutationHeld:false,closeBlocked:false,sourceRevalidationRequired:false};
  const subscription=listener=>{listeners.add(listener);listener();return ()=>listeners.delete(listener);};
  const workspace={
    view:()=>structuredClone(state),subscribe:subscription,
    selection:{view:()=>structuredClone(scope),subscribe:subscription,cancel:()=>calls.push('cancel-scope-session')},
    units:{projectView:()=>structuredClone(units),subscribe:subscription,cancelRead:()=>calls.push('cancel-unit-session')},
    workbook:{acknowledge:()=>calls.push('acknowledge')},
    cancelScopeReads:()=>calls.push('cancel-scope-getters'),cancelUnitReads:()=>calls.push('cancel-unit-getters'),
    cancelReportRead:()=>calls.push('cancel-report-getters'),invalidate:()=>calls.push('invalidate'),
    fetchUnitPreferences:()=>{throw new Error('SSR must not read preferences');},
    saveUnitPreferences:()=>{throw new Error('SSR must not save preferences');},
    loadOptions:async()=>calls.push('options'),showPreview:async()=>calls.push('preview'),
    reviewWorkbook:async()=>calls.push('review'),publishWorkbook:async destination=>calls.push(['publish',destination]),
  };
  return {input,physicalOwner,state,workspace,calls,notify:()=>{for(const listener of listeners)listener();}};
}
function renderReport(value,environment={},overrides={}) {
  class Publication {
    view(){return {busy:false,blocked:false,error:'',outcome:null,requestedDestination:''};}
    subscribe(listener){listener();return ()=>{};}
  }
  const dependencies={
    '../bindings/github.com/boostao/vpro-wails':{},
    './longVegetationProjectWorkspaces':{projectReportWorkspace:physicalOwner=>{
      assert.deepEqual(JSON.parse(JSON.stringify(physicalOwner)),value.physicalOwner);
      return value.workspace;
    }},
    './longVegetationSelectedUnitSelection':{},'./LongVegetationSelectedUnitSelection.svelte':{default(){}},
    './longVegetationGroupPreferences':{},'./LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationNamingPreferences':{},'./LongVegetationNamingPreferences.svelte':{default(){}},
    './longVegetationSelectedFieldPreferences':{},'./LongVegetationSelectedFieldPreferences.svelte':{default(){}},
    './longVegetationEnforcementPreferences':{},'./LongVegetationEnforcementPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences':{},'./LongVegetationQualityNullPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences':{},'./LongVegetationQualityMinimumPreferences.svelte':{default(){}},
    './longVegetationUnitPreferences':{},'./LongVegetationUnitPreferences.svelte':{default(){}},
    './longVegetationPreferences':{},'./longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{},'./LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{},'./LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationReport':{},
    './longEnvironmentReport':{reportCellText:cell=>cell.storage==='null'?'NULL':cell.text??cell.integer??cell.real??cell.blobHex},
    './vegetationWorkbook':{VegetationWorkbookPublicationSession:Publication,vegetationWorkbookPublicationSession:()=>new Publication()},
    ...overrides,
  };
  const component=serverComponent(source('LongVegetationReport.svelte'),'LongVegetationReport.svelte',dependencies,
    {VITE_LONG_VEGETATION_PROJECT_SCOPE:'true',VITE_LONG_VEGETATION_WORKBOOK:'true',...environment});
  const busy=[];
  const body=render(component,{props:{...value.physicalOwner,onBusyChange:value=>busy.push(value)}}).body;
  return {body,busy};
}

test('compiled report renders original source2 source3 and qualified tuples with distinct workbook counts and no synthetic memberships',()=>{
  for(const index of [0,1,2]) {
    const value=workspaceFixture(index),{body}=renderReport(value);
    assert.match(body,/Project report source and unit fields/);
    assert.match(body,/Actual saved project Unit Groups/);
    assert.match(body,/Original project tuple provenance/);
    assert.match(body,/Original project tuple denominator/);
    assert.match(body,/No synthetic SU row IDs or memberships are created/);
    assert.match(body,/explicit desktop adaptation/);
    assert.match(body,/literal version/);
    assert.doesNotMatch(body,/Physical membership row IDs/);
    assert.doesNotMatch(body,/This is not all-project scope/);
    assert.doesNotMatch(body,/Select an SU in Projects/);
    assert.match(body,new RegExp(`original project non-NULL plot tuples ${[4,2,1][index]}`));
    if(index===1) assert.match(body,/All plots from original Env rows/);
    if(index===2) assert.match(body,/Original project tuples are retained by qualified plot existence/);
  }
});

test('inactive invalid source2 drafts retain close holds but do not disable source3 View or Save controls',()=>{
  const value=workspaceFixture(1);
  value.state.scope.fields=['MissingField'];value.state.scope.dirty=true;
  value.state.scope.draftError='Correct the inactive field draft';
  value.state.scope.closeBlocked=true;value.state.closeBlocked=true;
  const {body,busy}=renderReport(value);
  assert.match(body,/Correct the inactive field draft/);
  const view=body.match(/<button\b[^>]*data-source-control="btnViewReport"[^>]*>/g)??[];
  const save=body.match(/<button\b[^>]*data-project-unit-save[^>]*>/g)??[];
  assert.equal(view.length,1);assert.equal(save.length,1);
  assert.doesNotMatch(view[0],/\bdisabled\b/);assert.doesNotMatch(save[0],/\bdisabled\b/);
  assert.ok(busy.includes(true));
});

test('remount presentation retains unknown and committed-warning publication safety before editor fields without replay',()=>{
  for(const status of [null,'published-with-errors']) {
    const value=workspaceFixture();
    value.state.publication={busy:false,blocked:true,requestedDestination:value.input.exportRequest.destination,
      error:status?'linked then cleanup failed':'outcome unknown; never repeat',outcome:status?{
        ...value.input.outcome,status,errorMessage:'linked then cleanup failed'}:null};
    value.state.closeBlocked=true;
    for(let mount=0;mount<2;mount++) {
      const {body,busy}=renderReport(value);
      assert.match(body,/Acknowledge original project workbook outcome/);
      assert.ok(body.indexOf(status?'data-project-workbook-receipt':'data-project-workbook-unknown')<body.indexOf('data-project-report-source'));
      if(status) assert.match(body,new RegExp(value.input.review.workbookSHA256));
      else assert.match(body,/never replay blindly/);
      assert.ok(busy.includes(true));
    }
    assert.equal(value.calls.some(call=>Array.isArray(call)&&call[0]==='publish'),false);
  }
});

function routeHarness(value) {
  const code=source('LongVegetationReport.svelte');
  const handlers=code.slice(code.indexOf('  function cancel()'),code.indexOf('  async function loadSavedTitle()'));
  const rejectLegacy=()=>{throw new Error('Project route called a selected-list service');};
  const context=vm.createContext({...value.physicalOwner,projectWorkspace:value.workspace,projectActive:true,
    projectView:value.workspace.view(),sharedPreferenceBarrier:false,preferenceBarrier:false,workbookBarrier:false,
    busy:false,operationHeld:false,workbookEnabled:true,error:'',settings:null,preview:null,workbookReview:null,
    preferences:null,statistics:null,generation:0,destination:value.input.exportRequest.destination,
    reportBusy(){},reads:{cancelAll(){}},
    ContextService:{GetLongVegetationOptions:rejectLegacy,PreviewLongVegetation:rejectLegacy},
    VegetationWorkbookService:{GetReview:rejectLegacy,ExportReviewed:rejectLegacy}});
  vm.runInContext(ts.transpileModule(handlers+'\nthis.actions={options,show,reviewWorkbook,publishWorkbook,cancel};',
    {compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText,context);
  return {context,actions:context.actions};
}

test('actual project route handlers bypass selected-list and None SU restrictions only for ready owned project APIs',async()=>{
  for(const index of [0,1,2]) {
    const value=workspaceFixture(index),{context,actions}=routeHarness(value);
    await actions.options();await actions.show();await actions.reviewWorkbook();await actions.publishWorkbook();
    assert.deepEqual(value.calls,['options','preview','review',['publish',value.input.exportRequest.destination]]);
    context.operationHeld=true;await actions.options();await actions.show();await actions.reviewWorkbook();await actions.publishWorkbook();
    assert.equal(value.calls.length,4);
    context.operationHeld=false;context.sharedPreferenceBarrier=true;
    await actions.options();await actions.publishWorkbook();assert.equal(value.calls.length,4);
    context.sharedPreferenceBarrier=false;context.projectView.outputReady=false;
    await actions.options();await actions.show();await actions.reviewWorkbook();assert.equal(value.calls.length,4);
  }
});

test('actual project report Cancel never invokes publication cancellation or replays an issued output',async()=>{
  const value=workspaceFixture(),pending=deferred();
  value.workspace.publishWorkbook=destination=>{value.calls.push(['publish',destination]);return pending.promise;};
  const {actions}=routeHarness(value);
  const operation=actions.publishWorkbook();
  actions.cancel();
  assert.deepEqual(value.calls,[['publish',value.input.exportRequest.destination],'cancel-report-getters']);
  pending.resolve();await operation;
  assert.equal(value.calls.filter(call=>Array.isArray(call)&&call[0]==='publish').length,1);
});

test('actual root owner watcher keeps unmounted None-context project work in aggregate close Lock and context barriers',async()=>{
  const value=workspaceFixture(1),app=source('App.svelte');
  const call=app.indexOf('    const workspace = projectReportWorkspace(');
  const effect=app.slice(app.lastIndexOf('  $effect(',call),app.indexOf('  $effect(',call));
  const close=app.slice(app.indexOf('  async function handleCloseRequest('),app.indexOf('  let activeHierarchyIndex'));
  const transition=app.slice(app.indexOf('  async function requestTransition('),app.indexOf('  async function respondToTransition('));
  const effects=[];
  let confirmed=0,cancelled=0,navigated=0;
  const context=vm.createContext({
    $effect:fn=>effects.push(fn),projectReportWorkspace:()=>value.workspace,projectReportBusy:false,
    $projectState:{contextId:value.physicalOwner.contextId,activeProject:value.physicalOwner.project,
      projectPath:value.physicalOwner.projectPath,activeSU:'None',suPath:''},
    view:'home',busy:false,transitionWorking:false,archivePublicationBusy:false,vegetationPublicationBusy:false,
    lifeformPublicationBusy:false,summaryPublicationBusy:false,pendingTransition:null,closeWorking:false,
    closeRequest:'',closeError:'',error:'',editor:undefined,transitionError:'',
    document:{activeElement:null},HTMLElement:class{},tick:async()=>{},
    closeDisposition:loadTypeScript('closeLifecycle.ts').closeDisposition,
    CloseService:{GetPendingCloseRequest:async()=>'close',CancelClose:async()=>{cancelled++;},ConfirmClose:async()=>{confirmed++;}},
  });
  Object.defineProperty(context,'editorBusy',{get:()=>context.projectReportBusy});
  vm.runInContext(ts.transpileModule(effect.replaceAll("import.meta.env.VITE_LONG_VEGETATION_REPORT !== 'true'",'false')+
    close+transition+'\nthis.actions={handleCloseRequest,respondToClose,requestTransition};',
    {compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText,context);
  assert.equal(effects.length,1);const unsubscribe=effects[0]();
  value.state.closeBlocked=true;value.notify();
  assert.equal(context.projectReportBusy,true);assert.equal(context.view,'long-vegetation');
  context.view='home';
  await context.actions.handleCloseRequest('close');
  assert.equal(cancelled,1);assert.equal(confirmed,0);
  context.closeRequest='close';await context.actions.respondToClose('discard');assert.equal(confirmed,0);
  context.closeRequest='';await context.actions.requestTransition(async()=>{navigated++;});assert.equal(navigated,0);
  value.state.closeBlocked=false;value.notify();
  await context.actions.requestTransition(async()=>{navigated++;});assert.equal(navigated,1);
  unsubscribe();
});

test('full project report and root compile with one central immutable factory and correction versus mutation holds',()=>{
  const panel=source('LongVegetationReport.svelte'),app=source('App.svelte');
  for(const [filename,input] of [['LongVegetationReport.svelte',panel],['App.svelte',app]]) {
    assert.equal(compile(input,{filename,generate:'client'}).warnings.length,0);
    assert.match(input,/import \{ projectReportWorkspace \} from '\.\/longVegetationProjectWorkspaces'/);
  }
  assert.match(panel,/settings = \$state\.raw/);assert.match(panel,/preview = \$state\.raw/);
  assert.match(panel,/mutationHeld=\{busy \|\| operationHeld \|\| workbookBarrier \|\| sharedMutationHeld \|\| projectView\.unitMutationHeld\}/);
  assert.match(panel,/if \(disposed\) return/);
  assert.match(app,/editorBusy = \$derived\([^\n]*projectReportBusy\)/);
  assert.match(panel,/projectWorkspace\?\.cancelScopeReads\(\); projectWorkspace\?\.selection\.cancel\(\)/);
  assert.match(panel,/projectWorkspace\?\.cancelUnitReads\(\); projectWorkspace\?\.units\.cancelRead\(\)/);
});

test('ordinary Unit and selected-field controls inherit pending project scope holds while retained unknown receipts remain acknowledgeable',()=>{
  const unitSource=source('LongVegetationUnitPreferences.svelte');
  const selectionSource=source('LongVegetationSelectedUnitSelection.svelte');
  for(const busy of [true,false]) {
    const value=workspaceFixture();
    value.state.projectActive=false;
    value.state.scope.busy=busy;
    value.state.scope.authorityUnknown=!busy;
    value.state.scope.scope.sourceKind='selected-list';
    const unitView={saved:{unitGroups:1},draft:{unitGroups:'1'},busy:false,blocked:!busy,
      authorityUnknown:!busy,error:'',draftError:'',receipts:busy?[]:[{
        kind:'unknown',errorMessage:'Retained ordinary Save outcome; never replay',
      }]};
    const units={view:()=>unitView,availability:()=>({siteUnitTable:true,selectedFields:true,none:true}),
      subscribe:listener=>{listener();return ()=>{};},cancelLoad(){}};
    const selectedView={review:{definitions:[{table:'Env',name:'PlotNumber',declaredType:'TEXT',available:true}]},
      fields:[],busy:false,dirty:false,authorityUnknown:false,draftError:'',error:''};
    const selection={view:()=>selectedView,subscribe:listener=>{listener();return ()=>{};},cancel(){}};
    const dependencies={
      './longVegetationUnitPreferences':{createLongVegetationUnitPreferencesRegistry:()=>()=>units,
        longVegetationUnitChoices:[{value:1,label:'Site unit table',capability:'siteUnitTable'}]},
      './longVegetationSelectedUnitSelection':{createSelectedUnitSelectionRegistry:()=>()=>selection},
    };
    dependencies['./LongVegetationUnitPreferences.svelte']={default:serverComponent(unitSource,
      'LongVegetationUnitPreferences.svelte',{'../bindings/github.com/boostao/vpro-wails':{},...dependencies})};
    dependencies['./LongVegetationSelectedUnitSelection.svelte']={default:serverComponent(selectionSource,
      'LongVegetationSelectedUnitSelection.svelte',{'../bindings/github.com/boostao/vpro-wails':{}})};
    const {body}=renderReport(value,{VITE_LONG_VEGETATION_UNIT_PREFERENCES:'true',
      VITE_LONG_VEGETATION_SELECTED_UNIT_SELECTION:'true'},dependencies);
    const button=attribute=>{
      const matches=body.match(new RegExp(`<button\\b[^>]*${attribute}[^>]*>`,'g'))??[];
      assert.equal(matches.length,1,attribute);return matches[0];
    };
    for(const attribute of ['data-long-vegetation-unit-load','data-long-vegetation-unit-save']) {
      assert.match(button(attribute),/\bdisabled\b/);
    }
    if(busy) {
      for(const attribute of ['data-long-vegetation-unit-undo','data-selected-unit-load',
        'data-selected-unit-build','data-selected-unit-add']) assert.match(button(attribute),/\bdisabled\b/);
    } else {
      assert.match(body,/Retained ordinary Save outcome; never replay/);
      assert.doesNotMatch(button('data-long-vegetation-unit-acknowledge'),/\bdisabled\b/);
    }
  }
});

test('retained project holds suppress all three actual root background readers on every unmounted route before cancellation or dispatch',async()=>{
  for(const view of ['home','plots','profiles','fs882']) {
    const dispatch=[];
    const reads={cancelAll:()=>dispatch.push('cancel'),track:request=>request};
    const context=componentFunctions('App.svelte',['loadPlots','refresh','loadHierarchy'],{
      projectReportBusy:true,externalSUEnvironmentBusy:false,twoPageVegetationBusy:false,twoPageSoilBusy:false,
      pictureCreationBusy:false,pictureLifecycleBusy:false,vegetationPublicationBusy:false,
      lifeformPublicationBusy:false,summaryPublicationBusy:false,view,editorBusy:false,
      request:0,stateRequest:0,hierarchyRequest:0,busy:false,error:'',profileNavigation:null,pageSize:25,
      plotReads:reads,stateReads:reads,hierarchyReads:reads,
      StartupService:{GetState:async()=>{dispatch.push('startup');return {ready:true};}},
      ProjectService:{ListPlots:async()=>dispatch.push('plots'),GetState:async()=>dispatch.push('state'),
        GetHierarchyNodes:async()=>dispatch.push('hierarchy')},
    });
    await context.actions.loadPlots(0);await context.actions.refresh();await context.actions.loadHierarchy();
    assert.deepEqual(dispatch,[],view);
    assert.equal(context.request,0);assert.equal(context.stateRequest,0);assert.equal(context.hierarchyRequest,0);
    context.projectReportBusy=false;
    await context.actions.loadPlots(0);await context.actions.loadHierarchy();
    assert.deepEqual(dispatch,['cancel','plots','cancel','hierarchy'],view);
  }
});

test('unobserved or refused project scope blocks output but not native close unless real work draft or mutation authority is retained',()=>{
  for(const held of [false,true]) {
    const value=workspaceFixture();
    value.state.projectActive=false;value.state.outputReady=false;
    value.state.scope.scope=null;value.state.scope.outputReady=false;
    value.state.scope.error='Long Vegetation project report scope is disabled in this session';
    value.state.scope.authorityUnknown=held;value.state.scope.closeBlocked=held;value.state.closeBlocked=held;
    const {body,busy}=renderReport(value);
    assert.match(body,/project report scope is disabled/);
    const controls=body.match(/<button\b[^>]*data-source-control="btnViewReport"[^>]*>/g)??[];
    assert.equal(controls.length,1);assert.match(controls[0],/\bdisabled\b/);
    assert.equal(busy.includes(true),held);
  }
});

test('project workspace report getters expose live Cancel while irreversible project publication never does',()=>{
  for(const publication of [false,true]) {
    const value=workspaceFixture();
    value.state.readBusy=!publication;value.state.outputReady=false;value.state.closeBlocked=true;
    value.state.publication.busy=publication;
    const {body}=renderReport(value);
    const cancels=body.match(/<button\b[^>]*>Cancel report read<\/button>/g)??[];
    assert.equal(cancels.length,publication?0:1);
    if(!publication) assert.doesNotMatch(cancels[0],/\bdisabled\b/);
  }
});
