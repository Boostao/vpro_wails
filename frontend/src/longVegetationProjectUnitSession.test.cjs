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
const cells=loadTypeScript('projectMetadataRestore.ts',{'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names=loadTypeScript('reportUnitNames.ts',{'./projectMetadataRestore':cells});
const environment=loadTypeScript('longEnvironmentReport.ts',{'./projectMetadataRestore':cells,'./reportUnitNames':names});
const ordinary=loadTypeScript('longVegetationReport.ts',{
  './projectMetadataRestore':cells,'./longEnvironmentReport':environment,'./qualityEditor':quality,'./reportUnitNames':names
});
const locations=loadTypeScript('plotLocationReview.ts',{'./projectMetadataRestore':cells});
const earth=loadTypeScript('googleEarthReview.ts',{'./projectMetadataRestore':cells,'./qualityEditor':quality,'./plotLocationReview':locations});
const selection=loadTypeScript('longVegetationSelectedUnitSelection.ts',{
  './googleEarthReview':earth,'./qualityEditor':quality,'./longVegetationReport':ordinary
});
const report=loadTypeScript('longVegetationProjectReport.ts',{
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./qualityEditor':quality
});
const guards=loadTypeScript('longVegetationProjectScope.ts',{
  './longVegetationProjectReport':report,'./longVegetationSelectedUnitSelection':selection,'./qualityEditor':quality
});
const preferences=loadTypeScript('reportTitlePreferences.ts',{'./googleEarthReview':earth,'./qualityEditor':quality});
const ordinaryUnits=loadTypeScript('longVegetationUnitPreferences.ts',{'./googleEarthReview':earth,'./reportTitlePreferences':preferences});
const moduleUnderTest=loadTypeScript('longVegetationProjectUnitSession.ts',{
  './longVegetationProjectReport':report,'./longVegetationProjectScope':guards,
  './longVegetationUnitPreferences':ordinaryUnits,'./reportTitlePreferences':preferences
});
const wire=JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectScopeWire.json'),'utf8'));
const requests=JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectUnitRequestWire.json'),'utf8'));
const owner={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'None',suPath:''};
const deferred=()=>{let resolve;const promise=new Promise(yes=>resolve=yes);return {promise,resolve}};
const ready=async(index=0)=>{
  const session=new moduleUnderTest.ProjectUnitPreferencesSession(owner,true);
  session.setApproval(wire[index].scope);
  await session.load(async request=>{
    assert.deepEqual(structuredClone(request),{revision:'3',approvalHash:wire[index].scope.build.approvalHash});
    return structuredClone(wire[index].unitPreferences);
  },async()=>{});
  return session;
};

test('project Unit session/registry are default-off, freeze actual None ownership and require scope-owned Save',async()=>{
  assert.throws(()=>new moduleUnderTest.ProjectUnitPreferencesSession(owner),/disabled/);
  assert.throws(()=>moduleUnderTest.createProjectUnitPreferencesRegistry(),/disabled/);
  const registry=moduleUnderTest.createProjectUnitPreferencesRegistry(true);
  assert.throws(()=>registry({...owner,suPath:'invented'}),/physical paths/);
  const session=registry(owner);
  assert.equal(registry({...owner}),session);
  assert.throws(()=>registry({...owner,projectPath:'different'}),/changed physical/);
  await assert.rejects(()=>session.save(),/full current scope/);
  await assert.rejects(()=>session.saveProject(async()=>wire[0].outcome),/Load and correct/);
  assert.equal(session.projectView().closeBlocked,false);
});

test('actual saved1 is not silently normalized; full production-Go requests drive source2/source3 Save',async()=>{
  for(const index of [0,1]) {
    const session=await ready(index);
    assert.equal(session.projectView().saved.unitGroups,1);
    assert.equal(session.projectView().draft.unitGroups,'1');
    assert.equal(session.projectView().saveReady,false);
    assert.equal(session.projectView().closeBlocked,false);
    session.editUnitGroups(String(index+2));
    assert.equal(session.projectView().saveReady,true);
    await session.saveProject(async request=>{
      assert.deepEqual(request,requests[index]);
      return proxy(structuredClone(wire[index].outcome));
    });
    const view=session.projectView();
    assert.equal(view.saved.unitGroups,index+2);
    assert.equal(view.blocked,true);assert.equal(view.closeBlocked,true);
    assert.equal(view.projectReceipts.length,1);
    assert.deepEqual(view.projectReceipts[0].request,requests[index]);
    session.acknowledge();
    assert.equal(session.projectView().projectReceipts.length,1);
    assert.equal(session.projectView().closeBlocked,false);
  }
});

test('dirty Unit drafts and errors survive retained remount and explicit Load; correction/Undo stay live',async()=>{
  const registry=moduleUnderTest.createProjectUnitPreferencesRegistry(true),session=registry(owner);
  session.setApproval(wire[0].scope);
  await session.load(async()=>structuredClone(wire[0].unitPreferences),async()=>{});
  session.editUnitGroups('2 ');
  assert.match(session.projectView().draftError,/supported/);
  assert.equal(registry({...owner}),session);
  const changed=structuredClone(wire[0].unitPreferences);changed.preferences.values.unitGroups=2;
  await session.load(async()=>changed,async()=>{});
  assert.equal(session.projectView().saved.unitGroups,2);
  assert.equal(session.projectView().draft.unitGroups,'2 ');
  assert.match(session.projectView().draftError,/supported/);
  assert.equal(session.projectView().closeBlocked,true);
  session.editUnitGroups('2');assert.equal(session.projectView().draftError,'');
  session.undo();assert.equal(session.projectView().draft.unitGroups,'2');
  assert.equal(session.projectView().closeBlocked,false);
});

test('scope changes preserve independent Unit draft and revalidate availability without deleting receipts',async()=>{
  const session=await ready();session.editUnitGroups('2');
  session.setApproval(wire[1].scope);
  assert.equal(session.projectView().draft.unitGroups,'2');
  assert.equal(session.projectView().readReady,false);
  await session.load(async()=>structuredClone(wire[1].unitPreferences),async()=>{});
  assert.equal(session.projectView().draft.unitGroups,'2');
  assert.match(session.projectView().draftError,/different report scope/);
  assert.equal(session.projectView().saveReady,false);
  session.undo();assert.equal(session.projectView().draft.unitGroups,'1');
  session.editUnitGroups('3');assert.equal(session.projectView().draftError,'');
  assert.equal(session.projectView().saveReady,true);
});

test('Unit Save cannot be cancelled or scope-switched while pending; committed cancellation and no-op receipts persist',async()=>{
  const session=await ready(),held=deferred();
  session.editUnitGroups('2');
  const pending=session.saveProject(async()=>held.promise);
  session.cancelRead();
  assert.equal(session.projectView().busy,true);
  assert.equal(session.projectView().closeBlocked,true);
  assert.throws(()=>session.setApproval(wire[1].scope),/acknowledge/);
  assert.throws(()=>session.acknowledge(),/cannot be cancelled/);
  const outcome=structuredClone(wire[0].outcome);outcome.outcome.errorMessage='context canceled';
  held.resolve(outcome);await pending;
  assert.equal(session.projectView().saved.unitGroups,2);
  assert.equal(session.projectView().projectReceipts[0].outcome.outcome.committed,true);
  session.acknowledge();
  const noop=structuredClone(wire[0].outcome);noop.outcome.changed=false;noop.outcome.committed=false;
  await session.saveProject(async request=>{
    assert.deepEqual(request.expected,{unitGroups:2});assert.deepEqual(request.proposed,{unitGroups:2});return noop;
  });
  session.acknowledge();
  assert.equal(session.projectView().projectReceipts.length,2);
  assert.equal(session.projectView().receipts.length,2);
  session.setApproval(wire[1].scope);
  assert.equal(session.projectView().projectReceipts[0].request.approvalHash,'a'.repeat(64));
  assert.equal(session.projectView().projectReceipts[0].outcome.outcome.committed,true);
});

test('unknown Unit Save outcome blocks replay until acknowledgement and actual owned Load, retaining full request',async()=>{
  const session=await ready();session.editUnitGroups('2');
  let calls=0;
  await session.saveProject(async()=>{calls++;throw new Error('transport lost')});
  const view=session.projectView();
  assert.equal(view.saved,null);assert.equal(view.authorityUnknown,true);assert.equal(view.blocked,true);
  assert.equal(view.projectReceipts[0].kind,'unknown');
  assert.deepEqual(view.projectReceipts[0].request,requests[0]);
  await assert.rejects(()=>session.saveProject(async()=>{calls++;return wire[0].outcome}),/Load and correct/);
  session.acknowledge();
  assert.equal(session.projectView().closeBlocked,true);
  const actual=structuredClone(wire[0].unitPreferences);actual.preferences.values.unitGroups=2;
  await session.load(async()=>actual,async()=>{});
  assert.equal(session.projectView().saved.unitGroups,2);
  assert.equal(session.projectView().authorityUnknown,false);
  assert.equal(session.projectView().projectReceipts[0].kind,'unknown');
  assert.equal(calls,1);
});

test('uncommitted unknown Save recovery retains the proposed Unit draft against independently recovered actual1',async()=>{
  const session=await ready();session.editUnitGroups('2');
  let calls=0;
  await session.saveProject(async()=>{calls++;throw new Error('transport failed before dispatch')});
  session.acknowledge();
  await session.load(async()=>structuredClone(wire[0].unitPreferences),async()=>{});
  const view=session.projectView();
  assert.equal(view.saved.unitGroups,1);
  assert.equal(view.draft.unitGroups,'2');
  assert.equal(view.authorityUnknown,false);
  assert.equal(view.closeBlocked,true);
  assert.equal(view.saveReady,true);
  assert.equal(view.projectReceipts[0].kind,'unknown');
  assert.deepEqual(view.projectReceipts[0].request,requests[0]);
  assert.equal(calls,1);
  session.undo();
  assert.equal(session.projectView().draft.unitGroups,'1');
  assert.equal(session.projectView().closeBlocked,false);
});

test('unknown Unit draft survives recovery under a changed explicit scope and is revalidated instead of reset',async()=>{
  const session=await ready();session.editUnitGroups('2');
  await session.saveProject(async()=>{throw new Error('transport failed before dispatch')});
  session.acknowledge();
  session.setApproval(wire[1].scope);
  await session.load(async()=>structuredClone(wire[1].unitPreferences),async()=>{});
  const view=session.projectView();
  assert.equal(view.saved.unitGroups,1);
  assert.equal(view.draft.unitGroups,'2');
  assert.match(view.draftError,/different report scope/);
  assert.equal(view.closeBlocked,true);
  assert.equal(view.saveReady,false);
  assert.equal(view.projectReceipts[0].request.approvalHash,'a'.repeat(64));
  assert.equal(view.projectReceipts[0].kind,'unknown');
  session.editUnitGroups('3');
  assert.equal(session.projectView().draftError,'');
  assert.equal(session.projectView().saveReady,true);
});

test('held Unit read validation is generation-safe, detached from actual proxies and not a mutation receipt',async()=>{
  const session=await ready(),read=proxy(structuredClone(wire[0].unitPreferences));
  const entered=deferred(),release=deferred(),notifications=[];
  session.subscribe(()=>notifications.push(session.projectView().readBusy));
  const pending=session.load(async()=>read,async()=>{entered.resolve();await release.promise});
  await entered.promise;
  read.preferences.values.unitGroups=3;read.scope.build.projectTuples[0].siteUnit.text='changed';
  release.resolve();await pending;
  assert.equal(session.projectView().saved.unitGroups,1);
  assert.equal(session.projectView().scope.build.projectTuples[0].siteUnit.text,'Alpha');
  assert.equal(session.projectView().projectReceipts.length,0);
  assert.equal(notifications.includes(true),true);assert.equal(notifications.at(-1),false);
  const staleEntered=deferred(),staleRelease=deferred();
  const old=session.load(async()=>structuredClone(wire[0].unitPreferences),async()=>{staleEntered.resolve();await staleRelease.promise});
  await staleEntered.promise;
  session.setApproval(wire[1].scope);
  staleRelease.resolve();await old;
  assert.equal(session.projectView().scope.sourceKind,'project-all-plots');
  assert.equal(session.projectView().readReady,false);
  assert.equal(session.projectView().saved.unitGroups,1);
});

test('initial Unit read failures do not invent saved defaults or close uncertainty; known refusals remain exact receipts',async()=>{
  const session=new moduleUnderTest.ProjectUnitPreferencesSession(owner,true);
  session.setApproval(wire[0].scope);
  await session.load(async()=>{throw new Error('read failed')},async()=>{});
  assert.equal(session.projectView().saved,null);
  assert.equal(session.projectView().authorityUnknown,false);
  assert.equal(session.projectView().closeBlocked,false);
  assert.match(session.projectView().readError,/no defaults/);
  await session.load(async()=>structuredClone(wire[0].unitPreferences),async()=>{});
  session.editUnitGroups('2');
  await session.saveProject(async()=>({sourceKind:'',revision:'',outcome:{contextId:'owned',changed:false,committed:false,errorMessage:'collision'}}));
  assert.equal(session.projectView().saved.unitGroups,1);
  assert.equal(session.projectView().projectReceipts[0].kind,'known');
  assert.equal(session.projectView().projectReceipts[0].outcome.outcome.committed,false);
  assert.equal(session.projectView().blocked,true);
});
