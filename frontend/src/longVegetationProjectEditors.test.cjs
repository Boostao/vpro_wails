const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {render} = require('svelte/server');
const {serverComponent,componentFunctions} = require('./svelteTestHelpers.cjs');
const source = name => readFileSync(path.join(__dirname,name),'utf8');
const scopeFile = 'LongVegetationProjectScopeSelection.svelte';
const unitFile = 'LongVegetationProjectUnitPreferences.svelte';
const wire = JSON.parse(source('longVegetationProjectScopeWire.json'));
const component = name => serverComponent(source(name),name,{});
const scopeView = () => ({
  scope:{sourceKind:'project-selected-fields',build:null},definitions:[
    {table:'Env',name:'Zone',declaredType:'TEXT',available:true,reason:''},
    {table:'Admin',name:'Date',declaredType:'REAL',available:false,reason:'uncalibrated type'}
  ],fields:['Zone','SubZone'],dirty:false,draftError:'',error:'',busy:false,authorityUnknown:false,outputReady:false
});
const unitView = () => ({
  saved:{unitGroups:1},draft:{unitGroups:'1'},scope:{sourceKind:'project-selected-fields'},
  readBusy:false,readReady:true,busy:false,blocked:false,authorityUnknown:false,draftError:'',error:'',readError:'',
  projectReceipts:[],saveReady:false
});
const session = (view,method) => ({
  [method]:()=>view,subscribe:listener=>{listener();return ()=>{}},cancel(){},cancelRead(){}
});
const scopeHTML = (view=scopeView(),props={}) => render(component(scopeFile),{props:{
  selection:session(view,'view'),allPlotsAvailable:false,cancelReads(){},onChange(){},...props
}}).body;
const unitHTML = (view=unitView(),props={}) => render(component(unitFile),{props:{
  preferences:session(view,'projectView'),fetchPreferences(){},savePreferences(){},cancelReads(){},onChange(){},...props
}}).body;
const buttonDisabled = (html,selector) => {
  const button = html.match(new RegExp(`<button[^>]*${selector}(?:="[^"]*")?[^>]*>`));
  assert.ok(button,`missing planned button ${selector}`);
  return /\sdisabled(?:[=>\s])/.test(button[0]);
};
const optionDisabled = (html,value) => {
  const option = html.match(new RegExp(`<option value="${value}"[^>]*>`));
  assert.ok(option,`missing planned option ${value}`);
  return /\sdisabled(?:[=>\s])/.test(option[0]);
};

test('project scope presentation keeps explicit source intent, original field order and unavailable diagnostics',()=>{
  const html=scopeHTML();
  assert.match(html,/<label for="project-report-source"[^>]*>/);
  assert.match(html,/Current selected site-unit list/);
  assert.match(html,/Whole project: Selected fields/);
  assert.match(html,/Whole project: All plots \(None SU\)/);
  assert.equal(optionDisabled(html,'project-all-plots'),true);
  assert.equal(optionDisabled(scopeHTML(scopeView(),{allPlotsAvailable:true}),'project-all-plots'),false);
  assert.match(html,/Field List/);assert.match(html,/Selected List/);
  assert.ok(html.indexOf('Env.Zone')<html.indexOf('Admin.Date'));
  assert.match(html,/uncalibrated type/);
  assert.equal(buttonDisabled(html,'data-project-field-add="1"'),true);
  assert.equal(buttonDisabled(html,'data-project-field-up="0"'),true);
  assert.equal(buttonDisabled(html,'data-project-field-down="1"'),true);
  assert.match(source(scopeFile),/grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/);
  assert.match(source(scopeFile),/@media \(max-width: 640px\)/);
});

test('source3 retains inactive invalid draft and safe Build while error and Undo stay visible above guidance',()=>{
  const view=scopeView();
  view.scope={sourceKind:'project-all-plots',build:{approvalHash:'a'.repeat(64),projectTuples:[{},{}]}};
  view.fields=['Removed'];view.dirty=true;view.draftError='Removed field unavailable';view.outputReady=true;
  const html=scopeHTML(view,{allPlotsAvailable:true});
  assert.match(html,/<details data-project-inactive-fields(?:="")?[^>]*>/);
  assert.match(html,/Retained inactive Selected List draft \(unsaved\)/);
  assert.match(html,/Removed/);
  assert.doesNotMatch(html,/data-project-report-field-lists/);
  assert.equal(buttonDisabled(html,'data-project-scope-build'),false);
  assert.equal(buttonDisabled(html,'data-project-field-undo'),false);
  assert.ok(html.indexOf('Removed field unavailable')<html.indexOf('<label'));
  assert.match(html,/2 original project tuples/);
  assert.match(html,/Source approval is current/);
});

test('scope busy, unknown and parent barriers block mutations but retain recovery and correction paths',()=>{
  for(const key of ['busy','authorityUnknown']) {
    const view=scopeView();view[key]=true;
    const html=scopeHTML(view);
    assert.equal(buttonDisabled(html,'data-project-scope-build'),true);
    if(key==='busy') assert.match(html,/data-project-scope-cancel/);
    else {
      assert.equal(buttonDisabled(html,'data-project-scope-load'),false);
      assert.equal(buttonDisabled(html,'data-project-field-undo'),false);
      assert.match(html,/Wait for the pending mutation to settle/);
    }
  }
  for(const key of ['operationHeld','mutationHeld']) {
    const html=scopeHTML(scopeView(),{[key]:true});
    assert.equal(buttonDisabled(html,'data-project-scope-build'),true);
    assert.equal(buttonDisabled(html,'data-project-field-undo'),key==='mutationHeld');
  }
  const html=scopeHTML({...scopeView(),scope:null,definitions:[],fields:[]});
  assert.equal(buttonDisabled(html,'data-project-scope-build'),true);
  assert.equal(buttonDisabled(html,'data-project-scope-load'),false);
});

test('project Unit editor displays actual saved mode and never enables another source implicitly',()=>{
  for(const kind of ['project-selected-fields','project-all-plots'])for(const saved of [1,2,3]) {
    const view=unitView();view.saved.unitGroups=saved;view.draft.unitGroups=String(saved);view.scope.sourceKind=kind;
    const html=unitHTML(view);
    assert.match(html,/<label for="project-unit-groups"[^>]*>/);
    assert.match(html,new RegExp(`Actual saved Unit Groups: ${saved}`));
    assert.equal(optionDisabled(html,'1'),saved!==1);
    assert.equal(optionDisabled(html,'2'),kind!=='project-selected-fields'&&saved!==2);
    assert.equal(optionDisabled(html,'3'),kind!=='project-all-plots'&&saved!==3);
    assert.equal(buttonDisabled(html,'data-project-unit-save'),true);
  }
  const view=unitView();view.draft.unitGroups='2';view.saveReady=true;
  assert.equal(buttonDisabled(unitHTML(view),'data-project-unit-save'),false);
});

test('full scoped receipts stay above controls and acknowledgement does not imply resolved unknown authority',()=>{
  const view=unitView();
  view.authorityUnknown=true;view.blocked=true;
  const request={revision:'3',approvalHash:'a'.repeat(64),expected:{unitGroups:1},proposed:{unitGroups:2}};
  const outcome=structuredClone(wire[0].outcome);
  outcome.outcome.errorMessage='Postcommit cleanup failed';
  view.projectReceipts=[
    {kind:'known',request,outcome},
    {kind:'unknown',request,errorMessage:'Transport lost'}
  ];
  const html=unitHTML(view);
  assert.equal((html.match(/data-project-unit-receipt/g)||[]).length,2);
  assert.ok(html.indexOf('Postcommit cleanup failed')<html.indexOf('<label'));
  assert.match(html,/Changed true; committed true/);
  assert.match(html,/Changed unknown; committed unknown/);
  assert.match(html,/expected 1; proposed 2/);
  assert.match(html,/Acknowledge the receipt, then Load actual saved values/);
  assert.equal(buttonDisabled(html,'data-project-unit-acknowledge'),false);
  assert.equal(buttonDisabled(html,'data-project-unit-save'),true);
  assert.equal(buttonDisabled(html,'data-project-unit-load'),true);
});

test('Unit read/Save and parent barriers distinguish cancellable reads, correction, Undo and receipts',()=>{
  const view=unitView();view.readBusy=true;
  assert.match(unitHTML(view),/data-project-unit-cancel/);
  view.readBusy=false;view.busy=true;
  assert.doesNotMatch(unitHTML(view),/data-project-unit-cancel/);
  assert.match(unitHTML(view),/Save cannot be cancelled/);
  view.busy=false;view.draftError='Correct or Undo';view.draft.unitGroups='2 ';
  let html=unitHTML(view,{operationHeld:true});
  assert.equal(buttonDisabled(html,'data-project-unit-save'),true);
  assert.equal(buttonDisabled(html,'data-project-unit-undo'),false);
  assert.ok(html.indexOf('Correct or Undo')<html.indexOf('<label'));
  html=unitHTML(view,{mutationHeld:true});
  assert.equal(buttonDisabled(html,'data-project-unit-undo'),true);
});

test('scope handlers enforce gates, pass exact ordered draft and surface predispatch errors',async()=>{
  const calls=[],view=scopeView();
  const context=componentFunctions(scopeFile,['run','load','switchScope','build','edit','move','undo'],{
    generation:0,error:'',view,operationHeld:false,mutationHeld:false,selection:{
      load:async()=>calls.push('load'),switchTo:async value=>calls.push(value),
      build:async()=>{throw new Error('Revision exhausted')},edit:fields=>calls.push(Array.from(fields)),undo:()=>calls.push('undo')
    }
  });
  await context.actions.load();await context.actions.switchScope('project-all-plots');
  context.actions.move(0,1);context.actions.undo();
  assert.deepEqual(calls,['load','project-all-plots',['SubZone','Zone'],'undo']);
  await context.actions.build();assert.match(context.error,/Revision exhausted/);
  await context.actions.switchScope('invented');assert.match(context.error,/explicit report source/);
  context.operationHeld=true;await context.actions.load();await context.actions.switchScope('selected-list');
  context.mutationHeld=true;context.actions.edit(['Changed']);await context.actions.build();
  assert.equal(calls.length,4);
});

test('Unit ports stay scope-owned and unmount cancellation never cancels or replays irreversible Save',async()=>{
  const calls=[],view=unitView();view.saveReady=true;
  const fetch=request=>Promise.resolve(request),save=request=>Promise.resolve(request);
  const context=componentFunctions(unitFile,['cancel','load','save','edit','undo'],{
    generation:0,error:'',view,operationHeld:false,mutationHeld:false,fetchPreferences:fetch,savePreferences:save,
    cancelReads:()=>calls.push('cancel getter only'),preferences:{
      cancelRead:()=>calls.push('drop read generation'),load:async port=>assert.equal(port,fetch),
      saveProject:async port=>{assert.equal(port,save);calls.push('Save')},
      editUnitGroups:raw=>calls.push(raw),undo:()=>calls.push('undo')
    }
  });
  await context.actions.load();await context.actions.save();context.actions.edit('2');context.actions.undo();context.actions.cancel();
  assert.deepEqual(calls,['Save','2','undo','cancel getter only','drop read generation']);
  assert.doesNotMatch(source(unitFile),/savePreferences\.cancel|reads\.track|\.save\(/);
  assert.doesNotMatch(source(scopeFile),/reads\.track|Service\.|JSON\.stringify/);
  for(const name of [scopeFile,unitFile]) {
    assert.match(source(name),/\$state\.raw\(untrack/);
    assert.match(source(name),/onDestroy\(\(\) => \{ unsubscribe\(\); cancel\(\); \}\)/);
  }
});
