const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile, compileModule} = require('svelte/compiler');
const {loadTypeScript, componentFunctions, serverComponent} = require('./svelteTestHelpers.cjs');
const {render} = require('svelte/server');
const ts = require('typescript');
const runtime = require('svelte/internal/client');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor': loadTypeScript('becEditor.ts')});
const defaults = JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'),'utf8'));
const mapping = JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json'),'utf8'));
const editor = loadTypeScript('projectMetadataEditor.ts', {'./qualityEditor': quality,'../../resources/project-metadata-standard.json':defaults,'../../resources/project-metadata-template.json':mapping});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor': quality, './projectMetadataEditor': editor});
const layout = JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-layout.json'),'utf8'));
const presentation = loadTypeScript('projectMetadataPresentation.ts', {'../../resources/project-metadata-layout.json': layout});
const json = value => JSON.parse(JSON.stringify(value));
const cell = (storage, value) => ({storage,text:storage==='text'?value:null,integer:storage==='integer'?value:null,real:null,blobHex:null});
const columns = ['ID','ProjectID','ProjectTitle','StartDate','CollectedSite','DataQualityVeg','EcosysCollectionStandard','Notes'].map(name=>({name,declaredType:''}));
const row = {rowId:'-200',cells:[cell('integer','-200'),cell('text','Project'),cell('text','Historical'),cell('integer','2000'),cell('integer','-1'),cell('text','OLD'),cell('text','OLD'),cell('null')]};
const review = {project:'Sample',plotNumber:'P1',projectId:'Project',projectRecords:{columns,rows:[row,{...row,rowId:'-201',cells:[cell('integer','-201'),...row.cells.slice(1)]}]},masterTemplates:{columns:[],rows:[]}};
const field = (name,kind='text',maximum=255,extra={})=>({name,kind,maximum,collection:false,referenceList:'',referenceColumn:'',limitToList:false,options:[],...extra});

function restorationReview() {
  const current=json(row), restored=json(row);
  current.cells[2]=cell('text','');
  const audit={rowId:'9007199254740993',project:'Sample',user:'Tester',plotNumber:'P1',table:'_Metadata',
    editField:'ProjectTitle',editWhen:'2026-10-02 01:02:03',beforeEdit:'Historical',afterEdit:'',restore:false,flag:false,id:-200};
  return {contextId:'original-context',historyId:'9007199254740994',project:'Sample',plotNumber:'P1',projectId:'Project',id:-200,
    columns,current,restored,audits:[audit]};
}
test('actual metadata opener and both existing buttons share the exact flag/parent/child/header guard',()=>{
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.equal((form.match(/metadataDisabled=\{!canOpenProjectMetadata\(\)\}/g) ?? []).length, 2);
  for (const enabled of [false, true]) for (const parent of [false, true])
    for (const child of [false, true]) for (const invalid of [false, true]) {
      const context = componentFunctions('FS882Form.svelte', ['canOpenProjectMetadata', 'openProjectMetadata'], {
        metadataEnabled: enabled, childParentDisabled: parent, childUnsaved: child,
        headerValidation: invalid ? { FieldNumber: 'invalid raw draft' } : {},
        metadataOpen: false, error: 'old error', successMsg: 'old notice',
      });
      const available = enabled && !parent && !child && !invalid;
      assert.equal(context.actions.canOpenProjectMetadata(), available);
      context.actions.openProjectMetadata();
      assert.equal(context.metadataOpen, available);
      assert.equal(context.successMsg, available ? null : 'old notice');
      if (available) assert.equal(context.error, null);
      else assert.match(context.error, /Finish or Undo existing drafts/);
    }
});

test('compiled Metadata menu requires current availability and callback, without enabling other unavailable actions',()=>{
  const source = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  const icons = new Proxy({}, { get: () => () => {} });
  const component = serverComponent(source, 'Navigation.svelte', { '@lucide/svelte': icons });
  for (const available of [false, true]) for (const callback of [undefined, () => {}]) {
    const html = render(component, { props: {
      view: 'fs882', onnavigate() {}, metadataAvailable: available, onopenmetadata: callback,
    } }).body;
    for (const label of ['Metadata', 'Herbarium', 'Colour-theme', 'User setup', 'Enter/Edit SIVI Data']) {
      const match = html.match(new RegExp(`<button\\b[^>]*>[\\s\\S]*?<span>${label}<\\/span>\\s*<\\/button>`));
      assert.ok(match, `actual compiled ${label} button must exist`);
      const button = match[0].slice(match[0].lastIndexOf('<button'));
      assert.equal(/\bdisabled(?:\s|=|>)/.test(button), label !== 'Metadata' || !(available && callback));
      if (label === 'Metadata') assert.match(button, /clean, unlocked editor/);
    }
  }
});

test('actual metadata menu dispatch closes disclosure once and never dispatches unavailable or absent callbacks',()=>{
  const calls = [];
  const context = componentFunctions('Navigation.svelte', ['itemEnabled', 'activate', 'closeMenu', 'navigate'], {
    metadataAvailable: false, onopenmetadata: undefined, openMenu: 'Forms',
    navigation: { querySelector: () => ({ querySelector: () => ({ focus: () => calls.push('focus') }) }) },
    onnavigate: view => calls.push(view),
  });
  context.actions.activate({ action: 'metadata' });
  context.metadataAvailable = true;
  context.actions.activate({ action: 'metadata' });
  assert.deepEqual(calls, []);
  context.onopenmetadata = () => { assert.equal(context.openMenu, null); calls.push('metadata'); };
  context.actions.activate({ action: 'metadata' });
  assert.deepEqual(calls, ['focus', 'metadata']);
  context.metadataAvailable = false;
  context.actions.activate({ action: 'metadata' });
  assert.deepEqual(calls, ['focus', 'metadata']);
  context.actions.activate({ view: 'plots' });
  assert.deepEqual(calls, ['focus', 'metadata', 'plots']);
});

test('actual root metadata entry rejects unavailable and stale owners without opening or replacing anything',async()=>{
  for (const unavailable of ['route', 'editor', 'context', 'guard']) {
    let opened = 0, transitions = 0;
    const owner = { canOpenProjectMetadata: () => unavailable !== 'guard', openProjectMetadata: () => opened++ };
    const context = componentFunctions('App.svelte', ['openProjectMetadata'], {
      view: unavailable === 'route' ? 'home' : 'fs882', editor: unavailable === 'editor' ? undefined : owner,
      $projectState: unavailable === 'context' ? null : { contextId: 'owned' }, error: '',
      requestTransition: async run => { transitions++; await run('owned'); },
    });
    await context.actions.openProjectMetadata();
    assert.equal(opened, 0); assert.equal(transitions, 0);
    assert.match(context.error, /clean, unlocked open plot/);
  }
  for (const stale of ['route', 'editor', 'context', 'callbackContext', 'guard', 'none']) {
    let pending, opened = 0, allowed = true;
    const owner = { canOpenProjectMetadata: () => allowed, openProjectMetadata: () => opened++ };
    const context = componentFunctions('App.svelte', ['openProjectMetadata'], {
      view: 'fs882', editor: owner, $projectState: { contextId: 'owned' }, error: '',
      requestTransition: async run => { pending = run; },
    });
    await context.actions.openProjectMetadata();
    assert.equal(opened, 0);
    if (stale === 'route') context.view = 'home';
    if (stale === 'editor') context.editor = { ...owner };
    if (stale === 'context') context.$projectState = { contextId: 'new-context' };
    if (stale === 'guard') allowed = false;
    if (stale === 'none') { await pending('owned'); assert.equal(opened, 1); }
    else { await assert.rejects(pending(stale === 'callbackContext' ? 'new-context' : 'owned'), /owner or availability changed/); assert.equal(opened, 0); }
    assert.equal(context.editor === owner, stale !== 'editor');
  }
});

test('actual root metadata entry retains busy/native-close/transition barriers and unsaved decisions',async()=>{
  const { closeDisposition } = loadTypeScript('closeLifecycle.ts');
  for (const hold of ['busy', 'editorBusy', 'transitionWorking', 'archivePublicationBusy', 'vegetationPublicationBusy',
    'lifeformPublicationBusy', 'summaryPublicationBusy', 'pendingTransition', 'closeRequest', 'editorBlocked', 'dirty', 'none']) {
    let opened = 0;
    const state = { unsaved: hold === 'dirty', busy: false, blocked: hold === 'editorBlocked' };
    const owner = { canOpenProjectMetadata: () => true, getCloseState: () => state, openProjectMetadata: () => opened++ };
    const values = {
      view: 'fs882', editor: owner, $projectState: { contextId: 'owned' }, error: '', transitionError: '',
      busy: false, editorBusy: false, transitionWorking: false, archivePublicationBusy: false,
      vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
      pendingTransition: null, closeRequest: '', closeDisposition, document: { activeElement: null },
      HTMLElement: class {}, tick: async () => {},
    };
    if (hold === 'pendingTransition') values.pendingTransition = { contextId: 'owned', run() {} };
    else if (hold === 'closeRequest') values.closeRequest = 'specific-owned-close-request';
    else if (hold in values) values[hold] = true;
    const context = componentFunctions('App.svelte', ['openProjectMetadata', 'requestTransition'], values);
    await context.actions.openProjectMetadata();
    assert.equal(opened, hold === 'none' ? 1 : 0);
    if (hold === 'dirty') assert.equal(typeof context.pendingTransition.run, 'function');
    else if (hold !== 'none') assert.match(context.error, /current operation or decision/);
  }
});

test('actual client-runtime metadata availability reacts across the exported editor method and root derived state',()=>{
  function execute(script, dependencies = {}) {
    const compiled = compileModule(script, { filename: 'metadata-menu.svelte.js', generate: 'client' });
    assert.equal(compiled.warnings.length, 0);
    const module = { exports: {} };
    new Function('module', 'exports', 'require', ts.transpileModule(compiled.js.code, { compilerOptions: {
      module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022,
    } }).outputText)(module, module.exports, id => {
      if (id === 'svelte/internal/client') return runtime;
      assert.ok(id in dependencies, `Unexpected client dependency ${id}`); return dependencies[id];
    });
    return module.exports;
  }
  const source = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  const ast = ts.createSourceFile('metadata.ts', source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1],
    ts.ScriptTarget.Latest, true);
  const predicate = ast.statements.find(node => ts.isFunctionDeclaration(node) && node.name.text === 'canOpenProjectMetadata');
  assert.ok(predicate);
  const child = execute(`
    let metadataEnabled = $state(false), childParentDisabled = $state(false), childUnsaved = $state(false),
      headerValidation = $state({});
    ${predicate.getText(ast)}
    export function update(flag, parent, peer, invalid) {
      metadataEnabled = flag; childParentDisabled = parent; childUnsaved = peer;
      if (invalid) headerValidation.raw = 'retained error'; else delete headerValidation.raw;
    }
  `);
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const expression = app.match(/metadataAvailable=\{([^}]+)\}/)?.[1];
  assert.ok(expression); assert.doesNotMatch(expression, /import\.meta/);
  const parent = execute(`
    import * as child from 'owned-editor';
    let editor = $state.raw(child), view = $state('fs882'), projectState = $state({ contextId: 'owned' });
    const available = $derived(${expression.replaceAll('$projectState', 'projectState')});
    export function enabled() { return available; }
    export function route(next) { view = next; }
    export function context(next) { projectState.contextId = next; }
    export function unmount() { editor = undefined; }
  `, { 'owned-editor': child });
  assert.equal(parent.enabled(), false);
  child.update(true, false, false, false); assert.equal(parent.enabled(), true);
  for (const changes of [[true, true, false, false], [true, false, true, false], [true, false, false, true], [false, false, false, false]]) {
    child.update(...changes); assert.equal(parent.enabled(), false);
    child.update(true, false, false, false); assert.equal(parent.enabled(), true);
  }
  parent.route('home'); assert.equal(parent.enabled(), false);
  parent.route('fs882'); assert.equal(parent.enabled(), true);
  parent.context(''); assert.equal(parent.enabled(), false);
  parent.context('owned'); assert.equal(parent.enabled(), true);
  parent.unmount(); assert.equal(parent.enabled(), false);
});

test('Typed restoration preserves NULL, empty and historical unchanged cells; payloads are independent exact snapshots',()=>{
  const source=restorationReview();
  const request=restoration.metadataRestorationRequest(source,'retain','original-context','P1');
  assert.equal(request.confirmed,true);
  assert.equal(request.review.historyId,'9007199254740994');
  assert.equal(request.review.current.cells[2].text,'');
  assert.equal(request.review.restored.cells[2].text,'Historical');
  assert.equal(request.review.restored.cells[4].integer,'-1');
  source.restored.cells[2].text='Later mutation';
  assert.equal(request.review.restored.cells[2].text,'Historical');
  assert.throws(()=>restoration.metadataRestorationRequest(source,'cancel','original-context','P1'),/Explicit retain/);
  for(const [property,value] of [['contextId','stale'],['plotNumber','P2'],['id',-201],['historyId','01']]) {
    assert.throws(()=>restoration.validateMetadataRestorationReview({...restorationReview(),[property]:value},'original-context','P1'));
  }
  for(const mutate of [
    r=>r.restored.rowId='-201', r=>r.restored.cells[0]=cell('integer','-201'),
    r=>r.restored.cells[7]=cell('text','Unaudited change'), r=>r.audits.push({...r.audits[0]}),
    r=>r.audits[0].editField='ID',r=>r.audits[0].table='_Env',
    r=>r.restored.cells[2].text='\ud800',r=>r.restored.cells[2].integer='7',
    r=>r.restored.cells[1]=cell('text','project')
  ]) {
    const bad=restorationReview();mutate(bad);
    assert.throws(()=>restoration.validateMetadataRestorationReview(bad,'original-context','P1'));
  }
});
test('Typed history distinguishes absence, exact events and retirement; committed result shape prevents replay',()=>{
  const history={historyPresent:true,events:[{historyId:'9007199254740994',created:'2026-10-02T01:02:03Z',rowId:'-200',id:-200,fields:['ProjectTitle'],restored:false}]};
  assert.equal(restoration.validateMetadataRestorationHistory(history),history);
  assert.doesNotThrow(()=>restoration.validateMetadataRestorationHistory({historyPresent:false,events:[]}));
  for(const bad of [{...history,historyPresent:false},{...history,events:null},
    {...history,events:[history.events[0],history.events[0]]},
    {...history,events:[{...history.events[0],historyId:'+1'}]},
    {...history,events:[{...history.events[0],restored:undefined}]}]) {
    assert.throws(()=>restoration.validateMetadataRestorationHistory(bad));
  }
  for(const action of ['retain','prune']) {
    const request=restoration.metadataRestorationRequest(restorationReview(),action,'original-context','P1');
    const result={cancelled:false,restoredRows:1,prunedAuditRows:action==='prune'?1:0,cleanedVegRows:0};
    assert.doesNotThrow(()=>restoration.validateMetadataRestorationResult(result,request));
    for(const bad of [{...result,cancelled:undefined},{...result,restoredRows:0},{...result,prunedAuditRows:2},{...result,cleanedVegRows:1}]) {
      assert.throws(()=>restoration.validateMetadataRestorationResult(bad,request),/committed.*reload/);
    }
  }
});
test('Metadata restoration is independently default-off and preserves explicit lifecycle and commit boundaries',()=>{
  const component=readFileSync(path.join(__dirname,'ProjectMetadataRestore.svelte'),'utf8');
  const parent=readFileSync(path.join(__dirname,'ProjectMetadataEditor.svelte'),'utf8');
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.equal(compile(component,{filename:'ProjectMetadataRestore.svelte',generate:'client'}).warnings.length,0);
  assert.match(form,/VITE_PROJECT_METADATA_RESTORE === 'true'/);
  assert.match(parent,/allowRestoration = false/);
  assert.match(parent,/if \(restorationOpen\) return restorationEditor\?\.getCloseState/);
  assert.match(component,/committed = true;[\s\S]*?proposal = null;[\s\S]*?validateMetadataRestorationResult/);
  assert.match(component,/metadataRestorationRequest\(\$state\.snapshot\(proposal\)/);
  assert.match(component,/committedFailure && committedRow !== null/);
  assert.match(component,/proposal retained for retry/);
  assert.match(component,/disabled=\{event\.restored\}/);
  assert.match(component,/Older plaintext audit rows/);
});

test('Metadata selection is explicit, literal and physical; duplicate candidates never imply first-row choice',()=>{
  const draft = editor.beginMetadataDraft(review,'-201');
  assert.equal(draft.id,-201);
  assert.equal(draft.original.rowId,'-201');
  assert.throws(()=>editor.beginMetadataDraft(review,''),/explicit physical/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectId:'project'},'-200'),/literal supported/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectRecords:{columns,rows:[row,row]}},'-200'),/explicit physical/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectId:null},'-200'),/literal parent/);
});
test('Blank metadata proposal captures empty schema and existing bounded parent identity without inventing a physical ID',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  const schema=['ID','ProjectID','AllSpecs','TableOfLists','DateLastEdited',...names].map(name=>({name,declaredType:'TEXT'}));
  const empty={...review,projectRecords:{columns:schema,rows:[]}};
  const request=editor.metadataBlankRequest(empty);
  assert.equal(request.projectId,'Project');
  assert.equal(request.plotNumber,'P1');
  assert.equal(request.original,empty.projectRecords);
  assert.equal('id' in request,false);
  for(const invalid of [review,{...empty,projectId:null},{...empty,projectId:''},{...empty,projectId:'x'.repeat(21)},
    {...empty,projectId:'\ud800'},{...empty,projectRecords:{columns:schema,rows:null}}]) {
    assert.throws(()=>editor.metadataBlankRequest(invalid));
  }
});
test('Template proposal preserves physical selection and source33 mapping without timestamp/code coercion or historical-invalid inheritance',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  const schema=['ID','ProjectID','AllSpecs','TableOfLists','DateLastEdited',...names].map(name=>({name,declaredType:'TEXT'}));
  const sourceNames=['ProjectID',...editor.metadataTemplateFields];
  const masterRow={rowId:'12',cells:sourceNames.map(name=>name==='ProjectID'?cell('text','Project'):
    name==='ProjectTitle'?cell('text','  Literal title  '):name==='StartDate'?cell('text','2000-01-02 03:04:05'):
    name==='GeoRefMethod'?cell('integer','7'):name==='CollectedSite'?cell('integer','-1'):
    name==='Notes'?cell('text',''):cell('null'))};
  const empty={...review,projectRecords:{columns:schema,rows:[]},
    masterTemplates:{columns:sourceNames.map(name=>({name,declaredType:'TEXT'})),rows:[masterRow,{...masterRow,rowId:'13'}]}};
  const fields=editor.metadataTemplateFields.map(name=>field(name,
    ['StartDate','EndDate','NumberOfFS882Plots','NumberOfSiteVisits',...editor.metadataTemplateFields.filter(name=>name.startsWith('Collected'))].includes(name)?'integer':'text',
    name==='NumberOfFS882Plots'?32:name==='Notes'?0:['StartDate','EndDate','NumberOfSiteVisits'].includes(name)?16:255,
    {collection:name.startsWith('Collected')}));
  let draft=editor.beginMetadataTemplate(empty,'13',fields);
  assert.equal(draft.original.rowId,'13');
  assert.equal(Object.keys(draft.cells).length,32);
  assert.equal(draft.cells.StartDate.raw,'2000-01-02 03:04:05');
  assert.match(draft.cells.StartDate.error,/explicit/);
  assert.match(draft.cells.GeoRefMethod.error,/explicit literal text/);
  assert.match(draft.cells.CollectedSite.error,/not a BOOLEAN/);
  assert.throws(()=>editor.metadataTemplateRequest(json(draft)));
  for(const [name,raw] of [['StartDate','2025'],['GeoRefMethod','Chosen literal'],['CollectedSite','1']]) {
    draft=editor.stageMetadataTemplate(json(draft),fields.find(field=>field.name===name),raw,false);
  }
  let request=editor.metadataTemplateRequest(draft);
  assert.equal(request.rowId,'13');
  assert.equal(request.values.length,32);
  assert.equal(request.values.find(change=>change.column==='ProjectTitle').value.text,'  Literal title  ');
  assert.equal(request.values.find(change=>change.column==='Notes').value.text,'');
  assert.equal(request.values.find(change=>change.column==='EndDate').value.storage,'null');
  assert.equal(request.values.some(change=>['ID','ProjectID','DateLastEdited'].includes(change.column)),false);
  draft=editor.stageMetadataTemplate(draft,fields.find(field=>field.name==='StartDate'),'32768',false);
  assert.throws(()=>editor.metadataTemplateRequest(json(draft)));
  draft=editor.stageMetadataTemplate(draft,fields.find(field=>field.name==='StartDate'),draft.cells.StartDate.raw,true);
  assert.equal(editor.metadataTemplateRequest(draft).values.find(change=>change.column==='StartDate').value.storage,'null');
  for(const id of ['', '01','-0','99']) assert.throws(()=>editor.beginMetadataTemplate(empty,id,fields));
  assert.throws(()=>editor.beginMetadataTemplate(review,'12',fields));
  assert.throws(()=>editor.beginMetadataTemplate(empty,'12',fields.slice(1)),/missing or ambiguous/);
  assert.throws(()=>editor.stageMetadataTemplate(draft,field('ID'),'1',false),/identity\/stamps/);
});
test('Metadata raw numeric errors survive serialization/remount, gate requests and clear on valid correction or Undo',()=>{
  const start = field('StartDate','integer',16);
  let draft = editor.beginMetadataDraft(review,'-200');
  for (const raw of ['1e2','32768','1.5','+1',' 1','-0','']) {
    draft = editor.stageMetadata(json(draft),start,raw,false);
    assert.equal(draft.cells.StartDate.raw,raw);
    assert.throws(()=>editor.metadataEditRequest(json(draft)));
  }
  draft = editor.stageMetadata(draft,start,'2024',false);
  assert.equal(editor.metadataErrors(draft).length,0);
  assert.deepEqual(json(editor.metadataEditRequest(draft).changes),[{column:'StartDate',value:cell('integer','2024')}]);
  assert.equal(editor.metadataDirty(editor.beginMetadataDraft(review,'-200')),false);
});
test('Metadata preserves NULL/empty/literal text and enforces exact UTF-16 bounds without truncation',()=>{
  const notes = field('Notes','text',0), title = field('ProjectTitle','text',255);
  let draft = editor.beginMetadataDraft(review,'-200');
  draft = editor.stageMetadata(draft,notes,'',false);
  assert.deepEqual(json(editor.metadataEditRequest(draft).changes[0].value),cell('text',''));
  draft = editor.stageMetadata(draft,notes,'',true);
  assert.equal(editor.metadataDirty(draft),false);
  for (const raw of ['\ud800','\udc00','🌲'.repeat(128)]) {
    draft = editor.stageMetadata(draft,title,raw,false);
    assert.equal(draft.cells.ProjectTitle.raw,raw);
    assert.throws(()=>editor.metadataEditRequest(json(draft)));
  }
  draft = editor.stageMetadata(draft,title,'🌲'.repeat(127)+'x',false);
  assert.equal(editor.metadataErrors(draft).length,0);
  draft = editor.stageMetadata(draft,title,'  Mixed CASE  ',false);
  assert.equal(editor.metadataEditRequest(draft).changes[0].value.text,'  Mixed CASE  ');
});
test('Metadata source collections are three-state options; invalid historical values are omitted unchanged',()=>{
  const collected = field('CollectedSite','integer',16,{collection:true});
  let draft = editor.beginMetadataDraft(review,'-200');
  draft = editor.stageMetadata(draft,collected,'-1',false);
  assert.equal(editor.metadataDirty(draft),false);
  draft = editor.stageMetadata(draft,collected,'0',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/not a BOOLEAN/);
  draft = editor.stageMetadata(draft,collected,'3',false);
  assert.equal(editor.metadataEditRequest(draft).changes[0].value.integer,'3');
});
test('Metadata quality uses literal registered Notes; source-standard population requires an explicit separate decision',()=>{
  const qualityField=field('DataQualityVeg','text',255,{limitToList:true,referenceList:'PlotQualitySite',referenceColumn:'Note',options:[{rowId:'1',value:'Good Note',description:null}]});
  let draft=editor.stageMetadata(editor.beginMetadataDraft(review,'-200'),qualityField,'good note',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/literal registered Note/);
  draft=editor.stageMetadata(draft,qualityField,'Good Note',false);
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DEIF 2024',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/Explicitly keep/);
  draft.standardPopulation='keep';
  assert.equal(editor.metadataEditRequest(draft).standardPopulation,'keep');
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DTE changed',false);
  assert.equal(draft.standardPopulation,'');
});
test('Metadata source labels/grouping cover each ordinary field once and retain all74 binding relationships',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  assert.equal(names.length,70);
  assert.equal(new Set(names).size,70);
  for(const name of names) assert.ok(presentation.metadataLabel(name));
  assert.equal(layout.forms[0].fields.filter(field=>field.column).length,74);
  assert.equal(presentation.metadataLabel('CoverB2aDescription'),'B3 layer description');
  assert.equal(presentation.metadataLabel('GeoRefMethodOther'),'Georeference Method — other');
});
test('Confirmed source-standard population stages exactly24 literals, preserves other raw errors and permits deliberate later field edits',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  const complete={...review,projectRecords:{
    columns:[...columns.slice(0,2),...names.map(name=>({name,declaredType:'TEXT'}))],
    rows:[{rowId:'-200',cells:[cell('integer','-200'),cell('text','Project'),...names.map(()=>cell('null'))]}],
  }};
  const fields=names.map(name=>field(name));
  let draft=editor.stageMetadata(editor.beginMetadataDraft(complete,'-200'),field('EcosysCollectionStandard'),'DEIF 2024',false);
  draft=editor.stageMetadata(draft,field('StartDate','integer',16),'1e',false);
  const original=json(draft);
  assert.throws(()=>editor.populateMetadataStandard(draft,fields.filter(f=>f.name!=='Datum')),/unavailable/);
  assert.deepEqual(json(draft),original,'failed population cannot partially replace raw drafts');
  draft=editor.populateMetadataStandard(draft,fields);
  assert.equal(Object.keys(defaults).length,24);
  assert.equal(draft.standardPopulation,'populate');
  for(const [name,value] of Object.entries(defaults)) assert.equal(draft.cells[name].raw,value);
  assert.equal(draft.cells.StartDate.raw,'1e');
  assert.throws(()=>editor.metadataEditRequest(draft),/exact integer/);
  draft=editor.stageMetadata(draft,field('StartDate','integer',16),'2024',false);
  draft=editor.stageMetadata(draft,field('CoordinatingAgency'),'Deliberate override',false);
  assert.equal(editor.metadataEditRequest(draft).standardPopulation,'populate');
  assert.equal(editor.metadataEditRequest(draft).changes.length,26);
  assert.equal(draft.cells.CoordinatingAgency.value.text,'Deliberate override');
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DTE later',false);
  assert.equal(draft.standardPopulation,'');
  assert.throws(()=>editor.metadataEditRequest(draft),/confirm/);
  assert.throws(()=>editor.populateMetadataStandard(editor.beginMetadataDraft(complete,'-200'),fields),/changed recognized/);
});
test('Metadata component stays outside remounted tabs and gates ordinary Save/Lock/native-close/context while retaining failed writes',()=>{
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  const component=readFileSync(path.join(__dirname,'ProjectMetadataEditor.svelte'),'utf8');
  assert.match(form,/VITE_PROJECT_METADATA_EDITING === 'true'/);
  assert.match(form,/siviCreationPeerUnsaved = \$derived\([^;]*metadataOpen/);
  assert.match(form,/childParentDisabled = \$derived\([^;]*metadataOpen/);
  assert.match(form,/if \(metadataOpen\) return metadataEditor\?\.getCloseState/);
  assert.match(form,/if \(metadataOpen\) \{ metadataEditor\?\.undo\(\); return; \}/);
  assert.match(form,/ordinary plot Save never applies it/);
  assert.match(form,/Finish or close metadata review before changing the plot lock/);
  assert.match(component,/await client\.SaveProjectMetadata\(request\)/);
  assert.match(component,/metadata edit committed, but/);
  assert.match(component,/completed writes must not be replayed/);
  assert.match(component,/Save failed; raw drafts retained/);
  assert.match(form,/VITE_PROJECT_METADATA_CREATION === 'true'/);
  assert.match(form,/VITE_PROJECT_METADATA_TEMPLATE_CREATION === 'true'/);
  assert.match(component,/template = beginMetadataTemplate\(review, rowId, available\)/);
  assert.match(component,/await client\.CreateProjectMetadataFromTemplate\(metadataTemplateRequest\(template\)\)/);
  assert.match(component,/committed = true; template = null/);
  assert.match(component,/blank = metadataBlankRequest\(review\)/);
  assert.match(component,/await client\.CreateBlankProjectMetadata\(blank\)/);
  assert.match(component,/blank = null; error = null; success = 'Blank creation proposal undone/);
  assert.match(component,/reviewed proposal retained for retry/);
  assert.match(component,/committed = true; blank = null/);
  assert.equal(compile(component,{filename:'ProjectMetadataEditor.svelte',generate:'client'}).warnings.length,0);
});
test('Metadata reload invalidates editing readiness until both current records and references load successfully',()=>{
  const component=readFileSync(path.join(__dirname,'ProjectMetadataEditor.svelte'),'utf8');
  assert.match(component,/reading = true; ready = false/);
  assert.match(component,/await Promise\.all\(\[\s*reads\.track\(client\.ReviewProjectMetadata\(plot\)\), reads\.track\(client\.ListProjectMetadataFields\(\)\)/);
  assert.match(component,/review = next; fields = definitions; draft = null; committedFailure = false; ready = true/);
  for(const guard of [
    '!ready || !review || busy || dirty || committedFailure',
    '!ready || !draft || busy || committedFailure',
    'disabled={!ready || busy || dirty || committedFailure}',
    'disabled={!ready || busy || committedFailure || cell.nullValue}',
    'disabled={!ready || busy || committedFailure}',
    'disabled={!ready || busy || !draft || !dirty',
  ]) assert.ok(component.includes(guard),guard);
  assert.match(component,/export function undo\(\) \{[\s\S]*?if \(busy\)/);
});
