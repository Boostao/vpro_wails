const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {proxy} = require('svelte/internal/client');
const {render} = require('svelte/server');
const {loadTypeScript,serverComponent} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const cells = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':{}});
const locations = loadTypeScript('plotLocationReview.ts', {'./projectMetadataRestore':cells});
const earth = loadTypeScript('googleEarthReview.ts', {'./projectMetadataRestore':cells,'./qualityEditor':quality,'./plotLocationReview':locations});
const shared = loadTypeScript('reportTitlePreferences.ts', {'./qualityEditor':quality,'./googleEarthReview':earth});
const prefs = loadTypeScript('longVegetationUnitPreferences.ts', {'./reportTitlePreferences':shared,'./googleEarthReview':earth});
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationUnitWire.json'),'utf8'));
const scope = {...wire.review}; delete scope.values; delete scope.availability;
const review = (unitGroups=1, none=true, owner=scope) => ({
  ...owner, values:{unitGroups}, availability:{siteUnitTable:true,selectedFields:false,none}
});
const outcome = (committed=true,errorMessage='') => ({contextId:scope.contextId,changed:committed,committed,errorMessage});
const ready = (unitGroups=1,backend=true,client=true) => {
  const session = new prefs.LongVegetationUnitPreferencesSession(scope,client);
  session.apply(review(unitGroups,backend)); return session;
};
const source = name => readFileSync(path.join(__dirname,name),'utf8');
const child = () => serverComponent(source('LongVegetationUnitPreferences.svelte'),'LongVegetationUnitPreferences.svelte',{
  '../bindings/github.com/boostao/vpro-wails':{}, './longVegetationUnitPreferences':prefs
});

test('production unit wire projects actual reactive proxies and sends exact expected/proposed source integers',async()=>{
  const reactive=proxy(structuredClone(wire.review));assert.throws(()=>structuredClone(reactive));
  const detached=prefs.validateLongVegetationUnitPreferences(reactive,scope);
  assert.deepEqual(structuredClone(detached),wire.review);
  reactive.values.unitGroups=2;reactive.availability.none=false;
  assert.equal(detached.values.unitGroups,1);assert.equal(detached.availability.none,true);
  const session=ready();session.editUnitGroups('3');let sent;
  await session.save(request=>{sent=request;return Promise.resolve(wire.outcome)});
  assert.deepEqual(sent,wire.request);assert.deepEqual(session.view().outcome,wire.outcome);
  assert.deepEqual(Array.from(prefs.longVegetationUnitChoices,x=>[x.value,x.label]),[
    [1,'Site unit table'],[2,'Selected fields'],[3,'None']
  ]);
});
test('all unit choices distinguish valid historical no-op, unavailable Selected fields and independent client/server None gates',async()=>{
  for(const backend of [false,true])for(const client of [false,true])for(const old of [1,2,3])for(const proposed of [1,2,3]){
    const session=ready(old,backend,client);session.editUnitGroups(String(proposed));let calls=0;
    const allowed=old===proposed||proposed===1||proposed===3&&backend&&client;
    assert.equal(session.view().draftError==='',allowed);
    const save=()=>session.save(request=>{
      calls++;assert.deepEqual(request,{expected:{unitGroups:old},proposed:{unitGroups:proposed}});
      return Promise.resolve(outcome(old!==proposed));
    });
    if(allowed){await save();assert.equal(calls,1)}else{await assert.rejects(save());assert.equal(calls,0)}
    session.acknowledge();session.undo();assert.equal(session.view().draftError,'');
  }
});
test('unit preferences never infer missing saved values or malformed Selected fields availability',async()=>{
  const session=new prefs.LongVegetationUnitPreferencesSession(scope,true);
  assert.deepEqual(session.view().draft,{unitGroups:''});assert.equal(session.availability(),null);
  assert.throws(()=>session.editUnitGroups('1'),/Load/);await assert.rejects(session.save(async()=>outcome()),/Load/);
  for(const value of [null,{},review(null),...[0,4,'1',true,NaN,Infinity].map(x=>review(x)),
    {...review(),availability:null},{...review(),availability:{siteUnitTable:false,selectedFields:false,none:true}},
    {...review(),availability:{siteUnitTable:true,selectedFields:'true',none:true}},
    {...review(),availability:{siteUnitTable:true,none:true}},
    {...review(),availability:{siteUnitTable:true,selectedFields:false,none:'true'}},
    ...Object.keys(scope).map(key=>review(1,true,{...scope,[key]:'foreign'}))]){
    assert.throws(()=>prefs.validateLongVegetationUnitPreferences(value,scope),/no defaults applied/);
  }
  for(const value of [null,undefined,0,'true'])assert.throws(()=>new prefs.LongVegetationUnitPreferencesSession(scope,value));
  for(const value of [null,0,'true'])assert.throws(()=>new prefs.LongVegetationUnitPreferencesSession(scope,true,value));
});
test('selected-unit preference capability intersects independent immutable client and current reviewed backend gates',async()=>{
  for(const backend of [false,true])for(const client of [false,true])for(const old of [1,2,3]) {
    const input=review(old);input.availability.selectedFields=backend;
    const reactive=proxy(structuredClone(input));
    const session=new prefs.LongVegetationUnitPreferencesSession(scope,false,client);
    session.apply(reactive);
    reactive.availability.selectedFields=!backend;
    assert.equal(session.availability().selectedFields,backend&&client);
    session.editUnitGroups('2');
    const allowed=old===2||backend&&client;
    assert.equal(session.view().draftError==='',allowed);
    let calls=0;
    const save=()=>session.save(request=>{
      calls++;assert.deepEqual(request,{expected:{unitGroups:old},proposed:{unitGroups:2}});
      return Promise.resolve(outcome(old!==2));
    });
    if(allowed) {await save();assert.equal(calls,1)}
    else {await assert.rejects(save());assert.equal(calls,0)}
  }
  const defaultOff=new prefs.LongVegetationUnitPreferencesSession(scope,true);
  const input=review();input.availability.selectedFields=true;
  defaultOff.apply(input);defaultOff.editUnitGroups('2');
  assert.ok(defaultOff.view().draftError);
  const registry=prefs.createLongVegetationUnitPreferencesRegistry(false,true);
  const retained=registry(scope);retained.apply(input);retained.editUnitGroups('2');
  assert.equal(registry(scope),retained);assert.equal(retained.view().draftError,'');
});
test('raw invalid or unavailable Unit Groups drafts survive remount and block writes while correction and Undo remain live',async()=>{
  const registry=prefs.createLongVegetationUnitPreferencesRegistry(true);
  const session=registry(scope);session.apply(review());let calls=0;
  for(const value of ['', ' ', '01','1.0','1e0','0','4','None','\ud800',null,1,'2']){
    session.editUnitGroups(value);assert.equal(registry(scope),session);
    assert.equal(registry(scope).view().draft.unitGroups,value);assert.ok(registry(scope).view().draftError);
    await assert.rejects(session.save(async()=>{calls++;return outcome()}));
    session.editUnitGroups('3');assert.equal(session.view().draftError,'');session.undo();
    assert.equal(session.view().draft.unitGroups,'1');
  }
  assert.equal(calls,0);
  const available=session.availability();available.none=false;assert.equal(session.availability().none,true);
  for(const key of ['project','projectPath','su','suPath'])assert.throws(()=>registry({...scope,[key]:'foreign'}));
  const other={...scope,contextId:'other-unit-owner'};
  const sibling=registry(other);sibling.apply(review(3,true,other));sibling.editUnitGroups('1');
  assert.equal(registry(scope).view().draft.unitGroups,'1');
  assert.equal(registry(other).view().draft.unitGroups,'1');
  assert.notEqual(sibling,session);
});
test('held unit Save and warning/unknown mutation receipts block replay without losing retained authority',async()=>{
  const session=ready();session.editUnitGroups('3');let release,cancelled=0;
  const save=session.save(()=>{
    const promise=new Promise(done=>{release=done});promise.cancel=()=>{cancelled++};return promise;
  });
  for(const action of [()=>session.editUnitGroups('1'),()=>session.undo(),()=>session.beginLoad(),()=>session.acknowledge()])assert.throws(action);
  release(outcome(true,'committed; do not replay'));await save;session.acknowledge();
  assert.equal(cancelled,0);assert.equal(session.view().saved.unitGroups,3);
  session.editUnitGroups('1');await session.save(async()=>null);session.acknowledge();
  assert.equal(session.view().authorityUnknown,true);await assert.rejects(session.save(async()=>outcome()),/Load/);
  const token=session.beginLoad();session.cancelLoad();assert.equal(session.applyLoaded(review(1),token),false);
  assert.equal(session.view().saved,null);
  assert.equal(session.view().draft.unitGroups,'1');
  assert.equal(session.view().receipts[0].kind,'known');
  assert.equal(session.view().receipts[0].request.proposed.unitGroups,3);
  const retry=session.beginLoad();assert.equal(session.applyLoaded(review(1),retry),true);
  assert.equal(session.view().authorityUnknown,false);
});
test('source-labelled Unit Groups presentation preserves one responsive live control and unavailable historical choices',()=>{
  const component=child();
  for(const old of [1,2,3]){
    const session=ready(old,false,false);
    const body=render(component,{props:{contextId:scope.contextId,preferences:session,onReadBusyChange(){}}}).body;
    assert.match(body,/label for="long-vegetation-unit-groups"/);
    assert.match(body,/data-source-control="rptLvUnitGroups"/);
    assert.equal((body.match(/id="long-vegetation-unit-groups"/g)||[]).length,1);
    for(const caption of ['Unit Groups','Site unit table','Selected fields','None'])assert.ok(body.includes(caption));
    if(old!==2)assert.match(body,/value="2" disabled/);
    if(old!==3)assert.match(body,/value="3" disabled/);
    assert.match(body,/Selected fields require a current owned session Build/);
    assert.match(body,/all-project fallback remains unavailable/);
    assert.match(body,/resets the saved Unit Groups to Site unit table/);
  }
  assert.match(source('LongVegetationUnitPreferences.svelte'),/grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(source('LongVegetationUnitPreferences.svelte'),/const raw = event\.currentTarget\.value; edit\(raw\)/);
});
test('actual parent unit registry retains invalid drafts across remount and reaches aggregate report/native-close barriers',()=>{
  let created=0;
  const registry=prefs.createLongVegetationUnitPreferencesRegistry(true);
  const session=registry(scope);session.apply(review());session.editUnitGroups('2');
  class Publication{
    view(){return {busy:false,blocked:false,error:'',outcome:null,requestedDestination:''}}
    subscribe(){return ()=>{}}
  }
  const component=serverComponent(source('LongVegetationReport.svelte')
    .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_UNIT_PREFERENCES === 'true'/g,'true')
    .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g,'false'),
    'LongVegetationReport.svelte',{
      '../bindings/github.com/boostao/vpro-wails':{},
      './longVegetationUnitPreferences':{createLongVegetationUnitPreferencesRegistry(){created++;return registry}},
      './LongVegetationUnitPreferences.svelte':{default:child()},
      './longVegetationSelectedUnitSelection':{},'./LongVegetationSelectedUnitSelection.svelte':{default(){}},
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
      './vegetationWorkbook':{VegetationWorkbookPublicationSession:Publication},
    });
  for(let mount=0;mount<2;mount++){
    const body=render(component,{props:{...scope,onBusyChange(){}}}).body;
    assert.match(body,/This Unit Groups choice is unavailable/);
    assert.match(body,/data-long-vegetation-unit-save="" disabled/);
    assert.equal(registry(scope).view().draft.unitGroups,'2');
  }
  assert.equal(created,1);
  const parent=source('LongVegetationReport.svelte');
  assert.match(parent,/const sharedPreferenceBarrier = \$derived\([^\n]*ordinaryUnitBarrier\)/);
  assert.match(parent,/const preferenceBarrier = \$derived\(sharedPreferenceBarrier \|\| projectBarrier\)/);
  assert.match(parent,/onBusyChange\(busy \|\| workbookBarrier \|\| sharedPreferenceBarrier \|\| !!projectView\?\.closeBlocked\)/);
  assert.match(parent,/unsubscribeUnits\?\.\(\)/);
  for(const name of ['Switch','Threshold','Group','QualityNull','QualityMinimum','Naming','SelectedField','Enforcement']){
    const start=parent.indexOf(`<LongVegetation${name}Preferences `);
    assert.notEqual(start,-1,`missing ${name} sibling`);
    const end=parent.indexOf('/>',start);
    const props=parent.slice(start,end);
    assert.match(props,/operationHeld=\{unitBarrier \|\|/);
    assert.match(props,/mutationHeld=\{unitMutationHeld \|\|/);
  }
});
