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
const selection = loadTypeScript('longVegetationSelectedUnitSelection.ts', {
  './googleEarthReview':earth,'./qualityEditor':quality,'./longVegetationReport':ordinary
});
const project = loadTypeScript('longVegetationProjectReport.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./qualityEditor':quality
});
const scopeClient = loadTypeScript('longVegetationProjectScope.ts', {
  './longVegetationProjectReport':project,'./longVegetationSelectedUnitSelection':selection,'./qualityEditor':quality
});
const publication = loadTypeScript('publicationSession.ts');
const lifeform = loadTypeScript('lifeformSummary.ts');
const workbook = loadTypeScript('vegetationWorkbook.ts', {
  './longVegetationReport':ordinary,'./projectMetadataRestore':cells,'./lifeformSummary':lifeform,'./publicationSession':publication
});
const dependencies = {'./longVegetationProjectReport':project,'./longVegetationProjectScope':scopeClient,
  './lifeformSummary':lifeform,'./vegetationWorkbook':workbook,'./publicationSession':publication};
const filename = path.join(__dirname,'longVegetationProjectWorkbook.svelte.ts');
const stripped = ts.transpileModule(readFileSync(filename,'utf8'), {
  compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ESNext}
}).outputText;
const compiled = compileModule(stripped,{filename,generate:'client'});
assert.equal(compiled.warnings.length,0);
const moduleValue = {exports:{}};
vm.runInNewContext(ts.transpileModule(compiled.js.code, {
  compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}
}).outputText,{
  structuredClone,module:moduleValue,exports:moduleValue.exports,
  require(name) {
    if(name==='svelte/internal/client') return client;
    if(name in dependencies) return dependencies[name];
    throw new Error(`Unexpected project workbook dependency ${name}`);
  }
});
const projectWorkbook = moduleValue.exports;
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectWorkbookWire.json'),'utf8'));
const fixture = (index=0) => structuredClone(wire[index]);
const owner = scope => Object.fromEntries(['contextId','project','projectPath','su','suPath'].map(key=>[key,scope[key]]));
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});
const validate = (review,expected=review.scope) => projectWorkbook.validateProjectWorkbookReview(review,expected,owner(expected),true);
const deferred = () => {let resolve,reject; const promise = new Promise((yes,no)=>{resolve=yes;reject=no;});return {promise,resolve,reject};};

test('production-Go workbook wire retains complete scope and independently qualified summary on actual runtime proxies',()=>{
  assert.equal(wire.length,3);
  for(const [index,value] of wire.entries()) {
    for(const reactive of [false,true]) {
      const input=reactive?client.proxy(fixture(index).review):fixture(index).review;
      if(reactive) assert.throws(()=>structuredClone(input));
      const expected=reactive?client.proxy(fixture(index).review.scope):fixture(index).review.scope;
      const result=validate(input,expected);
      assert.equal(Object.isFrozen(result),true);
      assert.equal(Object.isFrozen(result.scope.build.projectTuples[0].plotNumber),true);
      input.scope.fields.push('later');
      input.preview.settings.title='later';
      input.summary.projectTuples[0].siteUnit.text='later';
      assert.equal(result.preview.settings.title,'Project units');
      assert.equal(result.scope.fields.includes('later'),false);
      assert.notEqual(result.summary.projectTuples[0].siteUnit.text,'later');
      assert.equal(result.scope.build.projectTuples.length,[4,2,2][index]);
      assert.equal(result.summary.sourcePlotRows,[4,2,1][index]);
      assert.equal(result.summary.environmentRows,2);
      assert.deepEqual(JSON.parse(JSON.stringify(projectWorkbook.projectWorkbookReadRequest(result.scope,owner(result.scope),true))),value.readRequest);
      if(index===2) {
        assert.equal(result.preview.report.quality.occurrences.length,8);
        assert.equal(result.preview.report.units[0].numPlots,1);
        assert.equal(result.preview.report.projectUnits.length,1);
      }
      assert.throws(()=>workbook.validateVegetationWorkbookReview(value.review,owner(value.review.scope),true));
    }
  }
});

test('every project workbook client entry is independently default-off with literal owned path identity',()=>{
  const {review}=fixture(),physicalOwner=owner(review.scope);
  assert.throws(()=>projectWorkbook.validateProjectWorkbookReview(review,review.scope,physicalOwner),/disabled/);
  assert.throws(()=>projectWorkbook.validateProjectWorkbookReview(review,review.scope,physicalOwner,1),/disabled/);
  assert.throws(()=>projectWorkbook.projectWorkbookReadRequest(review.scope,physicalOwner),/disabled/);
  assert.throws(()=>new projectWorkbook.ProjectWorkbookPublicationSession(physicalOwner),/disabled/);
  assert.throws(()=>projectWorkbook.projectWorkbookPublicationSession(physicalOwner),/disabled/);
  for(const changed of [{...physicalOwner,contextId:''},{...physicalOwner,suPath:'phantom'},
    {...physicalOwner,su:'Selected',suPath:''},{...physicalOwner,projectPath:'C:\\different.db'}]) {
    assert.throws(()=>projectWorkbook.validateProjectWorkbookReview(review,review.scope,changed,true));
  }
});

test('strict project workbook refuses malformed envelopes, legacy summaries, stale Builds and unavailable report variants',()=>{
  for(const mutate of [
    x=>x.extra=true,x=>x.scope=null,x=>x.scope.revision='4',x=>x.scope.buildCurrent=false,
    x=>x.scope.build.approvalHash='e'.repeat(64),x=>x.scope.fields=['Other'],x=>x.scope.build.projectTuples.pop(),
    x=>x.preview.report.projectUnits.pop(),x=>x.preview.report.sourceKind='selected-list',
    x=>x.reportScope='lumped',x=>x.reportScope='source4',x=>x.createdDate='2026-02-30',
    x=>x.createdDate='0099-12-31',x=>x.createdDate='2026-10-10T00:00:00Z',
    x=>x.approvalHash='A'.repeat(64),x=>x.workbookSHA256='',x=>x.bytes=0,x=>x.bytes=0.5,
    x=>x.options=null,x=>x.options.quickReport=-1,x=>x.options.reportSummary='true',x=>x.options.extra=true,
    x=>x.sheets=null,x=>x.skippedUnits=null,x=>x.sheets=[],
    x=>x.sheets[0].name='Changed',x=>x.sheets[0].unit=cell('Other'),x=>x.sheets[1].name=x.sheets[0].name,
    x=>x.summary=null,x=>x.summary.extra=true,x=>x.summary.createdDate='2026-10-11',
    x=>x.summary.sourceKind='project-all-plots',x=>x.summary.sourcePlotRows=2,
    x=>x.summary.environmentRows=1,x=>x.summary.environmentRows=-1,
    x=>x.summary.projectTuples=null,x=>x.summary.projectTuples.pop(),x=>x.summary.projectTuples.reverse(),
    x=>x.summary.projectTuples[0].envRowId='99',x=>x.summary.projectTuples[0].adminRowId=null,
    x=>x.summary.projectTuples[0].extra=true,x=>x.summary.projectTuples[0].plotNumber.text='P2',
    x=>x.summary.selectedPlotRows=x.summary.sourcePlotRows,
    x=>x.skippedUnits=[{unit:x.sheets[0].unit,reason:'source NumPlots < 1'}],
    x=>x.skippedUnits=[{unit:cell('Other'),reason:'invented'}],
  ]) {
    const {review}=fixture(),expected=structuredClone(review.scope);
    mutate(review);
    assert.throws(()=>validate(review,expected),undefined,String(mutate));
  }
  const {review}=fixture(2),expected=structuredClone(review.scope);
  review.summary.sourcePlotRows=8;
  assert.throws(()=>validate(review,expected),/summary/);
  review.summary.sourcePlotRows=1;review.summary.projectTuples=structuredClone(review.scope.build.projectTuples);
  assert.throws(()=>validate(review,expected),/summary/);
  const absent=fixture().review;absent.options.reportSummary=false;absent.summary=null;
  assert.doesNotThrow(()=>validate(absent));
  absent.summary=fixture().review.summary;
  assert.throws(()=>validate(absent),/Unrequested/);
});

test('project Description evidence preserves NULL empty literal missing and unavailable states without repair',()=>{
  for(const version of [cell(null),cell(''),cell(' literal é '),cell('x'.repeat(300))]) {
    const {review}=fixture();
    review.summary.versionDefinitions.rows[0].cells[1]=version;
    review.summary.speciesVersion=structuredClone(version);
    review.summary.versionStatus=version.storage==='null'?'null':'literal';
    assert.equal(validate(review).summary.speciesVersion.text,version.text);
  }
  for(const columns of [null,[]]) {
    const {review}=fixture();
    review.summary.versionDefinitions={columns,rows:null};
    review.summary.speciesVersion=cell('Unknown');review.summary.versionStatus='metadata-unavailable';
    assert.doesNotThrow(()=>validate(review));
  }
  const missing=fixture().review;
  missing.summary.versionDefinitions.rows=[];missing.summary.speciesVersion=cell('Unknown');missing.summary.versionStatus='missing-definition';
  assert.doesNotThrow(()=>validate(missing));
  for(const mutate of [
    x=>x.speciesVersion=cell('invented'),x=>x.versionStatus='null',
    x=>x.versionDefinitions.rows.push(structuredClone(x.versionDefinitions.rows[0])),
    x=>x.versionDefinitions.columns[0].type=x.versionDefinitions.columns[0].declaredType,
    x=>x.versionDefinitions.rows[0].cells[0]=cell('OtherTable'),
    x=>x.versionDefinitions.rows[0].cells[1]=cell('\ud800'),
    x=>x.versionDefinitions.rows[0].rowId='01',
  ]) {
    const {review}=fixture();mutate(review.summary);assert.throws(()=>validate(review),undefined,String(mutate));
  }
});

test('once-only publication sends strict-Go full scope request and retains immutable committed receipts across acknowledgement',async()=>{
  const value=fixture(),physicalOwner=owner(value.review.scope);
  const session=new projectWorkbook.ProjectWorkbookPublicationSession(physicalOwner,true);
  physicalOwner.projectPath='mutated';
  const ready=session.prepare(client.proxy(value.review),client.proxy(value.review.scope));
  let calls=0;
  await session.publish(ready,ready.scope,value.exportRequest.destination,request=>{
    calls++;assert.deepEqual(JSON.parse(JSON.stringify(request)),value.exportRequest);
    return value.outcome;
  });
  assert.equal(calls,1);assert.equal(session.view().outcome.status,'published');
  assert.equal(session.view().blocked,true);
  assert.throws(()=>session.prepare(ready,ready.scope),/Acknowledge/);
  session.acknowledge();
  assert.equal(session.view().outcome.status,'published');
  await assert.rejects(session.publish(ready,ready.scope,value.exportRequest.destination,()=>{calls++;}),/fresh explicit/);
  assert.equal(calls,1);
  session.prepare(ready,ready.scope);
  await session.publish(ready,ready.scope,value.exportRequest.destination,()=>({...value.outcome,status:'not-published',
    path:'',sha256:'',errorMessage:'destination already exists'}));
  assert.equal(session.view().outcome.status,'not-published');
  assert.equal(session.view().error,'destination already exists');
});

test('pending unknown and committed-warning outcomes block replay and remount disposal without losing receipts',async()=>{
  for(const result of [
    null,
    {status:'published',requestedDestination:'wrong',path:'wrong',sha256:'b'.repeat(64),errorMessage:''},
    new Error('wire lost'),
    {...fixture().outcome,status:'published-with-errors',errorMessage:'committed then cleanup failed'},
  ]) {
    const value=fixture(),session=new projectWorkbook.ProjectWorkbookPublicationSession(owner(value.review.scope),true);
    session.prepare(value.review,value.review.scope);
    const pending=deferred();
    let calls=0;
    const operation=session.publish(client.proxy(value.review),value.review.scope,value.exportRequest.destination,()=>{calls++;return pending.promise;});
    assert.equal(session.view().busy,true);assert.equal(session.view().blocked,true);
    assert.throws(()=>session.acknowledge(),/cannot be cancelled/);
    await assert.rejects(session.publish(value.review,value.review.scope,value.exportRequest.destination,()=>{calls++;}),/Acknowledge/);
    if(result instanceof Error) pending.reject(result);else pending.resolve(result);
    await operation;
    assert.equal(session.view().busy,false);assert.equal(session.view().blocked,true);
    assert.equal(calls,1);
    if(result?.status==='published-with-errors') {
      assert.equal(session.view().outcome.sha256,value.review.workbookSHA256);
      assert.equal(session.view().error,'committed then cleanup failed');
    } else {
      assert.equal(session.view().outcome,null);assert.match(session.view().error,/unknown.*Do not repeat/);
    }
    const retained=session.view();
    session.acknowledge();
    assert.equal(session.view().error,retained.error);
    assert.deepEqual(session.view().outcome,retained.outcome);
    await assert.rejects(session.publish(value.review,value.review.scope,value.exportRequest.destination,()=>{calls++;}),/fresh explicit/);
    assert.equal(calls,1);
  }
});

test('stale independently current scope and malformed destinations never dispatch or consume approval',async()=>{
  const value=fixture(),session=new projectWorkbook.ProjectWorkbookPublicationSession(owner(value.review.scope),true);
  session.prepare(value.review,value.review.scope);
  let calls=0;
  for(const destination of ['', 'C:\\out.csv','C:\\bad\0.xlsx','C:\\bad\ud800.xlsx']) {
    await assert.rejects(session.publish(value.review,value.review.scope,destination,()=>{calls++;}),/literal/);
  }
  const stale=structuredClone(value.review.scope);stale.revision='4';
  await assert.rejects(session.publish(value.review,stale,value.exportRequest.destination,()=>{calls++;}),/independently/);
  assert.equal(calls,0);
  await session.publish(value.review,value.review.scope,value.exportRequest.destination,()=>{calls++;return value.outcome;});
  assert.equal(calls,1);
});

test('per-context publication registry preserves receipts and refuses physical or capability identity changes',()=>{
  const value=fixture(),physicalOwner={...owner(value.review.scope),contextId:'project-workbook-registry'};
  const session=projectWorkbook.projectWorkbookPublicationSession(physicalOwner,true);
  assert.equal(projectWorkbook.projectWorkbookPublicationSession({...physicalOwner},true),session);
  assert.throws(()=>projectWorkbook.projectWorkbookPublicationSession({...physicalOwner,projectPath:'other'},true),/different/);
  assert.throws(()=>projectWorkbook.projectWorkbookPublicationSession(physicalOwner,false),/different/);
});
