const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { editor, TwoPageSoilSession, fixture, original, cell, outcome, wire } = require('./twoPageSoilTestHelpers.cjs');
const { metadata, transport, restoration } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const soilReferences = loadTypeScript('twoPageSoilReferences.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration, './qualityEditor': quality,
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
});
const soil = loadTypeScript('soilChildEditor.ts', {
  './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
  './numericEditor': loadTypeScript('numericEditor.ts'),
});
const presentation = loadTypeScript('twoPageSoilPresentation.ts', {
  './projectMetadataEditor': metadata, './twoPageSoilEditor': editor, './soilChildEditor': soil,
});
const navigation = loadTypeScript('enterNavigation.ts');
const read = name => readFileSync(path.join(__dirname, name), 'utf8');
const sourceChoices = serverComponent(read('SoilReferenceChoices.svelte'), 'SoilReferenceChoices.svelte',
  { './projectMetadataEditor': metadata });
const bindings = {
  Humus: ['Horizon', 'UpperDepth', 'LowerDepth', 'HumusStructureDegree', 'HumusStructureKind',
    'MycelAbundance', 'FecalAbundance', 'RootsAbundance', 'RootsSize', 'vonPost', 'HumusFormpH', 'Comment'],
  Mineral: ['Horizon', 'UpperDepth', 'LowerDepth', 'PitDepthLimit', 'Texture', 'Colour', 'ASP',
    'PercentCoarseFragsGravel', 'PercentCoarseFragsCobbles', 'PercentCoarseFragsStones', 'PercentCoarseFragsTotal',
    'PercentCoarseFragsShape', 'RootsAbundance', 'RootsSize', 'MineralStructureClass', 'MineralStructureKind',
    'MineralFormpH', 'Comments'],
};
function complete(kind = 'Humus') {
  const source = original();
  source.form = `Soil${kind}XL`; source.table = `Sample_${kind}`; source.recordSource = `Usys${kind}`;
  for (const name of bindings[kind]) {
    if (source.records.columns.some(column => column.name === name)) continue;
    source.records.columns.push({ name, declaredType: '' });
    for (const row of source.records.rows) row.cells.push(cell());
  }
  return source;
}
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
function compiledSoilProfile(environment) {
  function declarations(source, names) {
    const ast = ts.createSourceFile('profile.ts', source, ts.ScriptTarget.Latest, true);
    const selected = ast.statements.filter(node => ts.isVariableStatement(node) &&
      node.declarationList.declarations.some(declaration => names.includes(declaration.name.getText(ast))));
    assert.equal(selected.length, names.length);
    return selected.map(node => node.getText(ast).replace(/^export /, '')).join('\n');
  }
  const root = read('FS882Form.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const clean = {
    soilEditingEnabled: true, soilReferenceReady: true, soilReferenceBusy: false, childParentDisabled: false,
    siviUnsaved: false, siviCoverUnsaved: false, siviCombinedUnsaved: false, siviCollectedUnsaved: false,
    siviSpeciesUnsaved: false, siviIdentityUnsaved: false, heightUnsaved: false, otherUnsaved: false,
    attributeUnsaved: false, collectedUnsaved: false, speciesUnsaved: false,
  };
  const script = `<script lang="ts">
    let { inspect } = $props();
    ${Object.entries(clean).map(([key, value]) => `const ${key} = ${value};`).join('\n')}
    ${declarations(read('twoPageSoilSessions.ts'), ['twoPageSoilReviewEnabled'])}
    ${declarations(root, ['twoPageReviewEnabled', 'twoPageEntryReviewEnabled', 'ordinarySoilUnavailable', 'soilEditingDisabled'])}
    function inspectProfile() { inspect({ ordinarySoilUnavailable, soilEditingDisabled }); }
    inspectProfile();
  </script>`;
  let profile;
  const html = render(serverComponent(script, 'SoilProfileTest.svelte', {}, environment), {
    props: { inspect: value => { profile = value; } },
  }).body;
  assert.equal(typeof html, 'string');
  assert.ok(profile);
  return profile;
}
function owned(source = complete(), port = {}) {
  return new TwoPageSoilSession({ ...fixture.owner, form: source.form }, {
    read: () => operation(source), save: () => { throw new Error('Unexpected Save'); },
    outcome: () => { throw new Error('Unexpected recovery'); }, ...port,
  });
}
const dependencies = session => ({
  '../bindings/github.com/boostao/vpro-wails': { TwoPageSoilService: {
    GetReferences() { throw new Error('Unexpected source reference read'); },
  } },
  './twoPageSoilReferences': soilReferences,
  './SoilReferenceChoices.svelte': { default: sourceChoices },
  './twoPageSoilSessions': { twoPageSoilSession: () => session, twoPageSoilWritingEnabled: true },
  './twoPageSoilEditor': editor, './projectMetadataEditor': metadata, './soilChildEditor': soil,
  './twoPageSoilPresentation': presentation,
  './enterNavigation': navigation,
});
const props = { ...fixture.owner, parentBlocked: false };

test('source-bound soil presentation contains each of the12/18 editable bindings exactly once', async () => {
  for (const kind of ['Humus', 'Mineral']) {
    const groups = presentation.twoPageSoilGroups[kind];
    assert.deepEqual(Array.from(groups.flatMap(group => Array.from(group.fields, field => field[0]))).sort(), bindings[kind].slice().sort());
    const session = owned(complete(kind));
    assert.equal(await session.load(), true);
    const component = serverComponent(read('TwoPageSoilPanel.svelte'), 'TwoPageSoilPanel.svelte', dependencies(session));
    const html = render(component, { props: { ...props, form: `Soil${kind}XL` } }).body;
    for (const column of bindings[kind]) assert.equal((html.match(new RegExp(`data-soil-column="${column}"`, 'g')) ?? []).length, 1, column);
    assert.match(html, /physical row 9223372036854775807/);
    assert.match(html, /NULL \(not empty text\)/);
    assert.match(html, /Numeric\/NULL depth direction follows the source form/);
    assert.match(html, /Source choices use current configured definitions and stage through ordinary soil validation/);
    assert.match(html, /creation, deletion and restoration remain unavailable/);
  }
});

test('NULL ID, absent Horizon and missing mapped fields stay explicit readonly physical rows', async () => {
  for (const change of [
    source => { source.records.rows[0].cells[0] = cell(); },
    source => {
      const index = source.records.columns.findIndex(column => column.name === 'Horizon');
      source.records.columns.splice(index, 1);
      for (const row of source.records.rows) row.cells.splice(index, 1);
    },
  ]) {
    const source = complete(); change(source);
    const session = owned(source); await session.load();
    const html = render(serverComponent(read('TwoPageSoilPanel.svelte'), 'TwoPageSoilPanel.svelte',
      dependencies(session)), { props }).body;
    assert.match(html, /Read-only physical row/);
    assert.ok(!/<input[^>]*type="text"(?![^>]*disabled)[^>]*>/.test(html));
    if (!source.records.columns.some(column => column.name === 'Horizon')) {
      assert.match(html, /source column unavailable/);
      assert.match(html, /Source field Horizon unavailable; row is read only/);
    } else assert.match(html, /Application ID NULL/);
  }
});

async function mountScript(client, session, parent = false, dom = {}) {
  let script = read('TwoPageSoilPanel.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, parentBlocked }', 'form, parentBlocked, inspect }');
  script += '\nfunction bindTestPanel(value) { panel = value; }\n$effect(() => inspect({ operation, stage, selectRow, enterRecord, bindTestPanel, row, error, view, close, loadReferences, chooseReference, referenceView, fieldsDisabled }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('panel.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const mount = new Function('$', 'tick', 'untrack', 'twoPageSoilSession', 'twoPageSoilWritingEnabled',
    'twoPageSoilKind', 'metadataCellText', 'equalCell', 'soilFields', 'twoPageSoilGroups', 'twoPageSoilCell',
    'twoPageSoilStoredText', 'twoPageSoilRowDiagnostic', 'twoPageSoilDepthOrder', 'enterNavigationMode',
    'document', 'HTMLInputElement', 'HTMLTextAreaElement', 'crypto', 'SoilReferenceLookup', 'TwoPageSoilService',
    code + '\nreturn _unknown_;')(
    client, async () => client.flush(), client.untrack, () => session, true, editor.twoPageSoilKind,
    metadata.metadataCellText, metadata.equalCell, soil.soilFields, presentation.twoPageSoilGroups, presentation.twoPageSoilCell,
    presentation.twoPageSoilStoredText, presentation.twoPageSoilRowDiagnostic, presentation.twoPageSoilDepthOrder,
    navigation.enterNavigationMode, dom.document, dom.HTMLInputElement, dom.HTMLTextAreaElement,
    { randomUUID: () => fixture.request.requestId }, soilReferences.SoilReferenceLookup, {
      GetReferences: (...args) => {
        if (!dom.referenceRead) throw new Error('Unexpected source reference read');
        return dom.referenceRead(...args);
      },
    });
  const parentBlocked = client.state(parent);
  let actions;
  const dispose = client.effect_root(() => mount(null, {
    ...props, ...dom.owner, get parentBlocked() { return client.get(parentBlocked); },
    inspect: value => { actions = value; },
  }));
  client.flush();
  return { actions: () => actions, dispose, parentBlocked };
}

test('actual client soil choices reject stale remount delivery, detach proxies and stage through retained validation without Save', async () => {
  const client = await import('svelte/internal/client');
  let release, cancelled = 0, saves = 0;
  const first = Object.assign(new Promise(resolve => { release = resolve; }), { cancel() { cancelled++; } });
  const referenceWire = () => {
    const result = structuredClone(require('./twoPageSoilReferenceWire.json')[0].wire);
    Object.assign(result, fixture.owner);
    const historic = structuredClone(result.fields[0].definitions.rows[0]);
    historic.rowId = '-9'; historic.cells[3] = cell('text', 'overlength');
    result.fields[0].definitions.rows.push(historic);
    return result;
  };
  const session = owned(complete(), { save() { saves++; throw new Error('Choice must not Save'); } });
  await session.load();
  let mounted = await mountScript(client, session, false, { referenceRead: () => first });
  try {
    const pending = mounted.actions().loadReferences();
    client.flush();
    assert.equal(mounted.actions().referenceView.busy, true);
    assert.equal(mounted.actions().fieldsDisabled, true);
    mounted.actions().stage('Horizon', 'LF', false);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    mounted.dispose();
    mounted = await mountScript(client, session, false, {
      referenceRead: () => operation(client.proxy(referenceWire())),
    });
    release(referenceWire()); await pending; client.flush();
    assert.equal(cancelled, 1);
    assert.equal(mounted.actions().referenceView.data, null);
    await mounted.actions().loadReferences(); client.flush();
    const data = mounted.actions().referenceView.data;
    assert.ok(data);
    assert.doesNotThrow(() => structuredClone(data));
    mounted.actions().bindTestPanel({ dataset: { soilCurrentRow: '-7', twoPageSoilPanel: 'SoilHumusXL' } });
    const definition = data.fields[0].definitions.rows.find(row => row.cells[3].text === 'L');
    const horizon = definition.cells[3];
    mounted.actions().chooseReference('Horizon', 'wrong physical row', definition.rowId, horizon); client.flush();
    assert.equal(Object.keys(session.view().drafts).length, 0);
    mounted.actions().chooseReference('Horizon', '-7', definition.rowId, cell('text', 'H')); client.flush();
    assert.equal(Object.keys(session.view().drafts).length, 0);
    assert.match(mounted.actions().error, /definition identity\/value differs/);
    mounted.actions().chooseReference('Horizon', '-7', definition.rowId, horizon); client.flush();
    assert.equal(session.view().drafts['["-7","Horizon"]'].raw, 'L');
    assert.equal(saves, 0);
    mounted.actions().chooseReference('Horizon', '-7', '-9', cell('text', 'overlength')); client.flush();
    assert.ok(session.view().drafts['["-7","Horizon"]'].error);
    assert.equal(session.closeState().canSave, false);
    mounted.dispose();
    mounted = await mountScript(client, session);
    assert.ok(mounted.actions().view.drafts['["-7","Horizon"]'].error);
    assert.equal(mounted.actions().referenceView.data, null);
    await mounted.actions().operation('undo'); client.flush();
    assert.equal(session.closeState().unsaved, false);
    assert.equal(saves, 0);
  } finally { mounted.dispose(); }
});

test('actual source chooser preserves numeric TEXT parsing, NULL versus empty and wrong-form refusal on both soil forms', async () => {
  const client = await import('svelte/internal/client');
  for (const [index, kind, column, fieldIndex] of [[0, 'Humus', 'vonPost', 3], [1, 'Mineral', 'ASP', 0]]) {
    const source = complete(kind), owner = { ...fixture.owner, form: source.form };
    const references = structuredClone(require('./twoPageSoilReferenceWire.json')[index].wire);
    Object.assign(references, owner);
    const textField = references.fields[kind === 'Humus' ? 0 : 1];
    source.records.rows[0].cells[source.records.columns.findIndex(field => field.name === column)] = cell('integer', '2');
    source.records.rows[0].cells[source.records.columns.findIndex(field => field.name === textField.column)] = cell('text', kind === 'Humus' ? 'F' : 'SL');
    const empty = structuredClone(textField.definitions.rows[0]);
    empty.rowId = '-900'; empty.cells[3] = cell('text', '');
    textField.definitions.rows.push(empty);
    let saves = 0;
    const session = owned(source, { save() { saves++; throw new Error('Source chooser must not Save'); } });
    await session.load();
    const mounted = await mountScript(client, session, false, { owner, referenceRead: () => operation(references) });
    try {
      await mounted.actions().loadReferences(); client.flush();
      const field = mounted.actions().referenceView.data.fields[fieldIndex];
      const ten = field.definitions.rows.find(row => row.cells[3].text === '10');
      await mounted.actions().selectRow('-7'); client.flush();
      class ObservedPanel {
        dataset = { soilCurrentRow: '-7', twoPageSoilPanel: kind === 'Humus' ? 'SoilMineralXL' : 'SoilHumusXL' };
      }
      const observedPanel = new ObservedPanel();
      mounted.actions().bindTestPanel(observedPanel);
      mounted.actions().chooseReference(column, '-7', ten.rowId, ten.cells[3]); client.flush();
      assert.equal(Object.keys(session.view().drafts).length, 0);
      observedPanel.dataset.twoPageSoilPanel = source.form;
      mounted.actions().chooseReference(column, '-7', ten.rowId, ten.cells[3]); client.flush();
      const numeric = session.view().drafts[JSON.stringify(['-7', column])];
      assert.ok(numeric, mounted.actions().error);
      assert.equal(numeric.raw, '10'); assert.equal(numeric.value.storage, 'integer'); assert.equal(numeric.value.integer, '10');
      const nullable = field.definitions.rows.find(row => row.cells[3].storage === 'null');
      mounted.actions().chooseReference(column, '-7', nullable.rowId, nullable.cells[3]); client.flush();
      assert.equal(session.view().drafts[JSON.stringify(['-7', column])].nullValue, true);
      mounted.actions().chooseReference(textField.column, '-7', empty.rowId, empty.cells[3]); client.flush();
      const blank = session.view().drafts[JSON.stringify(['-7', textField.column])];
      assert.equal(blank.nullValue, false); assert.equal(blank.value.storage, 'text'); assert.equal(blank.value.text, '');
      const nullText = textField.definitions.rows.find(row => row.cells[3].storage === 'null');
      mounted.actions().chooseReference(textField.column, '-7', nullText.rowId, nullText.cells[3]); client.flush();
      assert.equal(session.view().drafts[JSON.stringify(['-7', textField.column])].nullValue, true);
      assert.equal(saves, 0);
      await mounted.actions().operation('undo'); client.flush();
      assert.equal(session.closeState().unsaved, false);
    } finally { mounted.dispose(); }
  }
});

test('actual client panel subscription retains drafts/errors and read-only unknown recovery across remount', async () => {
  const client = await import('svelte/internal/client');
  let saves = 0, recoveries = 0;
  const session = owned(complete(), {
    save: () => { saves++; return operation(Promise.reject(new Error('Lost Save delivery'))); },
    outcome: json => { recoveries++; return operation(outcome(json, 'notCommitted')); },
  });
  let mounted = await mountScript(client, session);
  try {
    await mounted.actions().operation('load'); client.flush();
    mounted.actions().stage('Horizon', 'overlength', false); client.flush();
    assert.ok(mounted.actions().view.drafts['["-7","Horizon"]'].error);
    assert.equal(mounted.actions().close.canSave, false);
    mounted.dispose();
    mounted = await mountScript(client, session);
    assert.equal(mounted.actions().view.drafts['["-7","Horizon"]'].raw, 'overlength');
    mounted.actions().stage('Horizon', 'LF', false); client.flush();
    assert.equal(mounted.actions().close.canSave, true);
    client.set(mounted.parentBlocked, true); client.flush();
    await mounted.actions().operation('save');
    assert.equal(saves, 0);
    mounted.actions().stage('Horizon', 'L', false);
    assert.equal(session.view().drafts['["-7","Horizon"]'].raw, 'LF');
    client.set(mounted.parentBlocked, false); client.flush();
    await mounted.actions().operation('save'); client.flush();
    assert.equal(saves, 1);
    assert.equal(mounted.actions().close.blocked, true);
    mounted.dispose();
    mounted = await mountScript(client, session, true);
    await mounted.actions().operation('recover'); client.flush();
    assert.equal(saves, 1);
    assert.equal(recoveries, 1);
    assert.equal(mounted.actions().view.blocked, false);
    assert.equal(mounted.actions().view.drafts['["-7","Horizon"]'].raw, 'LF');
    await mounted.actions().operation('undo'); client.flush();
    assert.equal(session.closeState().unsaved, false);
  } finally { mounted.dispose(); }
});

test('actual panel Save dispatches once, retains immutable receipt and permits acknowledgement while parent blocks staging', async () => {
  const client = await import('svelte/internal/client');
  let saves = 0;
  const session = owned(complete(), {
    save: json => {
      saves++;
      const request = JSON.parse(json), result = wire(0);
      result.history.Request = request;
      result.history.Committed = structuredClone(request.original.records);
      const index = request.original.records.columns.findIndex(column => column.name === 'Horizon');
      result.history.Committed.rows[0].cells[index] = cell('text', 'LF');
      return operation(result);
    },
  });
  const mounted = await mountScript(client, session);
  try {
    await mounted.actions().operation('load'); client.flush();
    mounted.actions().stage('Horizon', 'LF', false); client.flush();
    await mounted.actions().operation('save'); client.flush();
    assert.equal(saves, 1);
    assert.equal(session.view().acknowledgementRequired, true, session.view().error);
    assert.equal(session.view().receipts.length, 1);
    await mounted.actions().operation('save');
    assert.equal(saves, 1);
    client.set(mounted.parentBlocked, true); client.flush();
    await mounted.actions().operation('acknowledge'); client.flush();
    assert.equal(session.closeState().blocked, false);
    assert.equal(session.view().receipts.length, 1);
    mounted.actions().view.receipts[0].history.Actor = 'external';
    assert.notEqual(session.view().receipts[0].history.Actor, 'external');
  } finally { mounted.dispose(); }
});

test('parent actual activity is not the always-unsaved review wrapper and includes clean peer operations', () => {
  const clean = { unsaved: false, busy: false, blocked: false, canSave: false, error: null, saveReason: '' };
  const peer = { ...clean };
  const context = componentFunctions('TwoPageEntryEditor.svelte', ['getCloseState', 'getDraftState'], {
    close: clean, error: null, view: { original: {}, referenceError: null },
    sessions: new Map([['normal', { closeState: () => clean }], ['chars', { closeState: () => peer }]]),
  });
  assert.equal(context.actions.getCloseState().unsaved, true);
  assert.equal(context.actions.getDraftState().unsaved, false);
  assert.equal(context.actions.getDraftState().blocked, false);
  for (const key of ['unsaved', 'busy', 'blocked']) {
    peer[key] = true;
    assert.equal(context.actions.getDraftState()[key], true);
    peer[key] = false;
  }
  context.view.referenceError = 'failed reference read';
  assert.equal(context.actions.getDraftState().blocked, true);
});

test('parent child guards refuse new drafts/write/variant/close while retaining correction and Undo', async () => {
  const calls = [];
  const close = { unsaved: false, busy: false, blocked: false };
  const context = componentFunctions('TwoPageEntryEditor.svelte',
    ['stage', 'selectProject', 'chooseForm', 'closeReview', 'operation'], {
      close, childBlocked: true, reviewEnabled: true, editingEnabled: true, error: null,
      view: { drafts: {} }, form: 'normal', onclosed: () => calls.push('close'), onchange: () => calls.push('change'),
      session: { stage: column => calls.push(column), selectProject: () => calls.push('project'),
        view: () => ({ error: null }), save: () => calls.push('save'), undo: () => { calls.push('undo'); return true; } },
  });
  context.actions.stage('Comments', { kind: 'text', raw: 'new' });
  context.actions.selectProject('1');
  await context.actions.operation('save');
  await context.actions.chooseForm('chars');
  context.actions.closeReview();
  assert.deepEqual(calls, []);
  context.view.drafts.Comments = {};
  context.actions.stage('Comments', { kind: 'text', raw: 'correction' });
  await context.actions.operation('undo');
  assert.deepEqual(calls, ['Comments', 'undo']);
});

test('root and form lifecycle barriers retain soil ownership and default-off numeric child fallback', () => {
  const root = read('App.svelte'), form = read('FS882Form.svelte');
  assert.match(root, /editorBusy = \$derived\(localEditorBusy \|\| pictureCreationBusy \|\| pictureLifecycleBusy \|\| twoPageSoilBusy \|\| twoPageVegetationBusy\)/);
  assert.match(root, /subscribeTwoPageSoilActivity\(activity => \{ twoPageSoilBusy = activity\.busy \|\| activity\.blocked;/);
  assert.match(form, /twoPageSoilReviewEnabled && twoPageScope === 'entry'/);
  assert.match(form, /childBlocked=\{twoPageSoilBusy \|\| twoPageSoilBlocked \|\| twoPageVegetationBusy \|\| twoPageVegetationBlocked\}/);
  assert.match(form, /SourceChild name=\{child\.form\}[^>]*disabled=\{true\}/);
  assert.match(form, /otherHeaderWorkflowBusy = \$derived\(independentHeaderWorkflowBusy \|\| twoPageOpen \|\| twoPageBusy\)/);
  const context = componentFunctions('FS882Form.svelte', ['getCloseState'], {
    twoPageSoilBusy: false, twoPageSoilBlocked: true, error: null,
  });
  const state = context.actions.getCloseState();
  assert.equal(state.unsaved, true);
  assert.equal(state.blocked, true);
  assert.equal(state.canSave, false);
  context.twoPageSoilBusy = true;
  assert.equal(context.actions.getCloseState().busy, true);
});

test('actual root close, context transition and background reads remain blocked after soil reader disposal and certification', async () => {
  const cancelled = [], transitions = [];
  const context = componentFunctions('App.svelte', ['handleCloseRequest', 'requestTransition', 'loadPlots', 'refresh', 'loadHierarchy'], {
    view: 'fs882', localEditorBusy: false, pictureCreationBusy: false, pictureLifecycleBusy: false,
    busy: false, editor: { getCloseState: () => ({ busy: false, blocked: false, unsaved: false, canSave: false, error: null }) },
    transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    pendingTransition: null, closeWorking: false, closeRequest: '', closeError: '', error: '', transitionError: '',
    request: 0, stateRequest: 0, hierarchyRequest: 0, $projectState: { contextId: fixture.owner.contextId },
    document: { activeElement: null }, HTMLElement: class {},
    closeDisposition: loadTypeScript('closeLifecycle.ts').closeDisposition, tick: async () => {},
    CloseService: { CancelClose: id => { cancelled.push(id); }, GetPendingCloseRequest: () => operation('soil-close') },
  });
  const expression = read('App.svelte').match(/const editorBusy = \$derived\(([^;]+)\);/)[1];
  const aggregate = new Function('localEditorBusy', 'pictureCreationBusy', 'pictureLifecycleBusy', 'twoPageSoilBusy', 'twoPageVegetationBusy', `return ${expression}`);
  Object.defineProperty(context, 'editorBusy', {
    get: () => aggregate(context.localEditorBusy, context.pictureCreationBusy, context.pictureLifecycleBusy, context.twoPageSoilBusy, context.twoPageVegetationBusy),
  });
  const session = owned(complete(), {
    save: () => operation(Promise.reject(new Error('Lost dispatch'))),
    outcome: json => operation(outcome(json, 'notCommitted')),
  });
  const stop = session.subscribe(() => {
    const state = session.closeState();
    context.twoPageSoilBusy = state.busy || state.blocked || state.unsaved;
  });
  async function blocked() {
    assert.equal(context.editorBusy, true);
    await context.actions.handleCloseRequest('soil-close');
    assert.equal(cancelled.at(-1), 'soil-close');
    await context.actions.requestTransition(async () => transitions.push('context'));
    for (const name of ['loadPlots', 'refresh', 'loadHierarchy']) await context.actions[name](0);
    assert.deepEqual(transitions, []);
    assert.equal(context.request, 0); assert.equal(context.stateRequest, 0); assert.equal(context.hierarchyRequest, 0);
  }
  try {
    await session.load();
    session.stage('-7', 'Horizon', 'overlength', false); await blocked();
    session.stage('-7', 'Horizon', 'LF', false);
    await session.save(fixture.request.requestId); await blocked();
    await session.resolve(); await blocked();
    session.undo();
    assert.equal(context.editorBusy, false);
  } finally { stop(); }
});

test('source depth direction uses exact numeric/NULL storage and stable ties without rewriting original order', () => {
  const depths = [
    ['1', cell()], ['2', cell('integer', '9007199254740993')], ['3', cell('real', 9007199254740992)],
    ['4', cell('integer', '-2')], ['5', cell('real', -2)], ['6', cell('real', -0)], ['7', cell('integer', '0')],
    ['8', cell('integer', '9223372036854775807')], ['9', cell('integer', '-9223372036854775808')],
  ];
  for (const [form, expected] of [
    ['SoilHumusXL', ['8', '2', '3', '6', '7', '4', '5', '9', '1']],
    ['SoilMineralXL', ['1', '9', '4', '5', '6', '7', '3', '2', '8']],
  ]) {
    const source = complete();
    source.form = form;
    const index = source.records.columns.findIndex(column => column.name === 'UpperDepth');
    source.records.rows = depths.map(([rowId, depth]) => {
      const row = structuredClone(source.records.rows[0]); row.rowId = rowId; row.cells[index] = depth; return row;
    });
    const before = structuredClone(source);
    const result = presentation.twoPageSoilDepthOrder(source);
    assert.deepEqual(Array.from(result.rows, row => row.rowId), expected);
    assert.equal(result.diagnostic, null);
    assert.deepEqual(source, before);
  }
});

test('historical text/blob or absent depth explicitly retains physical order, including numeric-looking text', () => {
  for (const depth of [cell('text', '5'), cell('text', ''), cell('blob', 'ff00')]) {
    const source = complete();
    const index = source.records.columns.findIndex(column => column.name === 'UpperDepth');
    source.records.rows[0].cells[index] = depth;
    const result = presentation.twoPageSoilDepthOrder(source);
    assert.deepEqual(Array.from(result.rows, row => row.rowId), ['-7', '9223372036854775807']);
    assert.match(result.diagnostic, /historical .* UpperDepth.*without coercion/);
  }
  const source = complete();
  const index = source.records.columns.findIndex(column => column.name === 'UpperDepth');
  source.records.columns.splice(index, 1);
  for (const row of source.records.rows) row.cells.splice(index, 1);
  assert.match(presentation.twoPageSoilDepthOrder(source).diagnostic, /UpperDepth is missing/);
});

test('actual compiled soil Enter verifies current focus/value/owner, moves to Horizon and never dispatches Save', async () => {
  const client = await import('svelte/internal/client');
  const source = complete();
  const depth = source.records.columns.findIndex(column => column.name === 'UpperDepth');
  const horizon = source.records.columns.findIndex(column => column.name === 'Horizon');
  source.records.rows[0].cells[depth] = cell('real', 10);
  source.records.rows[1].cells[depth] = cell('real', 5);
  source.records.rows[1].cells[0] = cell('integer', '1');
  source.records.rows[1].cells[horizon] = cell('text', 'B');
  let saves = 0, focused = 0;
  const session = owned(source, { save: () => { saves++; throw new Error('Enter dispatched Save'); } });
  const document = { activeElement: null };
  class Input {
    constructor(value = 'H', column = 'Horizon') { this.value = value; this.column = column; this.disabled = false; }
    checkVisibility() { return true; }
    closest() { return { dataset: { soilColumn: this.column } }; }
    focus() { document.activeElement = this; focused++; }
  }
  class Textarea extends Input {}
  const mounted = await mountScript(client, session, false, { document, HTMLInputElement: Input, HTMLTextAreaElement: Textarea });
  const destination = new Input('B');
  const panel = {
    dataset: { twoPageSoilPanel: source.form, get soilCurrentRow() { return mounted.actions().row?.rowId; } },
    contains: () => true, querySelector: () => destination,
  };
  let prevented = 0;
  const target = new Input(source.records.rows[0].cells[horizon].text);
  const event = { key: 'Enter', currentTarget: target, target, preventDefault() { prevented++; } };
  try {
    await mounted.actions().operation('load'); client.flush();
    mounted.actions().bindTestPanel(panel); client.flush();
    document.activeElement = target;
    target.value = 'stale DOM';
    await mounted.actions().enterRecord(event, 'Horizon'); client.flush();
    assert.equal(mounted.actions().row.rowId, '-7'); assert.match(mounted.actions().error, /value differs/);
    target.value = source.records.rows[0].cells[horizon].text;
    await mounted.actions().enterRecord({ ...event, ctrlKey: true }, 'Horizon');
    assert.equal(prevented, 1);
    mounted.actions().stage('Horizon', 'overlength', false); client.flush();
    await mounted.actions().enterRecord(event, 'Horizon'); client.flush();
    assert.equal(mounted.actions().row.rowId, '-7'); assert.match(mounted.actions().error, /Correct retained validation/);
    mounted.actions().stage('Horizon', 'LF', false); client.flush(); target.value = 'LF';
    await mounted.actions().enterRecord(event, 'Horizon'); client.flush();
    assert.equal(mounted.actions().row.rowId, '9223372036854775807');
    assert.equal(document.activeElement, destination); assert.equal(focused, 1);
    assert.equal(session.view().drafts['["-7","Horizon"]'].raw, 'LF');
    await mounted.actions().enterRecord({ ...event, currentTarget: destination, target: destination }, 'Horizon'); client.flush();
    assert.match(mounted.actions().error, /No next existing soil record/);
    assert.equal(saves, 0); assert.equal(session.view().receipts.length, 0);
  } finally { mounted.dispose(); }
});

test('explicit NULL displays blank without erasing retained raw text and remains directly typable', async () => {
  const source = complete();
  source.records.rows[0].cells[source.records.columns.findIndex(column => column.name === 'Horizon')] = cell('text', 'H');
  const session = owned(source); await session.load();
  session.stage('-7', 'Horizon', 'overlength', true);
  const html = render(serverComponent(read('TwoPageSoilPanel.svelte'), 'TwoPageSoilPanel.svelte',
    dependencies(session)), { props }).body;
  const input = html.match(/<input[^>]*id="SoilHumusXL--7-Horizon"[^>]*>/)?.[0];
  assert.ok(input);
  assert.match(input, /value=""/); assert.match(input, /placeholder="NULL"/);
  assert.doesNotMatch(input, /disabled/);
  assert.equal(session.view().drafts['["-7","Horizon"]'].raw, 'overlength');
  session.stage('-7', 'Horizon', 'LF', false);
  assert.equal(session.view().drafts['["-7","Horizon"]'].nullValue, false);
  assert.equal(session.view().drafts['["-7","Horizon"]'].error, null);
});

for (const remount of [false, true]) test(`successful depth Save preserves selected physical identity after ${remount ? 'retained draft remount' : 'explicit Load'}`, async () => {
  const client = await import('svelte/internal/client');
  const source = complete();
  const depth = source.records.columns.findIndex(column => column.name === 'UpperDepth');
  source.records.rows[0].cells[depth] = cell('real', 10);
  source.records.rows[1].cells[depth] = cell('real', 5);
  const session = owned(source, {
    save: json => {
      const request = JSON.parse(json), result = wire(0);
      result.history.Request = request; result.history.Committed = structuredClone(request.original.records);
      result.history.Committed.rows[0].cells[depth] = cell('real', 0);
      return operation(result);
    },
  });
  let mounted = await mountScript(client, session);
  try {
    await mounted.actions().operation('load'); client.flush();
    assert.equal(mounted.actions().row.rowId, '-7');
    mounted.actions().stage('UpperDepth', '0', false); client.flush();
    if (remount) {
      mounted.dispose(); mounted = await mountScript(client, session);
      assert.equal(mounted.actions().row.rowId, '-7');
      assert.equal(mounted.actions().view.drafts['["-7","UpperDepth"]'].raw, '0');
    }
    await mounted.actions().operation('save'); client.flush();
    assert.equal(session.view().receipts.length, 1, session.view().error);
    assert.equal(presentation.twoPageSoilDepthOrder(session.view().original).rows[0].rowId, '9223372036854775807');
    assert.equal(mounted.actions().row.rowId, '-7');
  } finally { mounted.dispose(); }
});

test('compiled flag matrix preserves legacy parent reads unless the complete physical-soil profile is usable', async () => {
  for (let mask = 0; mask < 8; mask++) {
    const rawProfile = mask === 7;
    const profile = compiledSoilProfile({
      VITE_TWO_PAGE_PARENT_REVIEW: mask & 1 ? 'true' : 'false',
      VITE_TWO_PAGE_PARENT_ENTRY_REVIEW: mask & 2 ? 'true' : 'false',
      VITE_TWO_PAGE_SOIL_REVIEW: mask & 4 ? 'true' : 'false',
    });
    assert.equal(!!profile.ordinarySoilUnavailable, rawProfile, `mask ${mask}`);
    assert.equal(profile.soilEditingDisabled, rawProfile, `mask ${mask}`);
    const calls = [];
    const context = componentFunctions('FS882Form.svelte', ['loadChildData'], {
      loadRequest: 1, childRevision: 0, siviIdentityEnabled: false,
      ...profile,
      reads: { track: value => value }, PlotService: {
        ListVegRecords: () => { calls.push('veg'); return operation([]); },
        ListHumusRecords: () => { calls.push('humus'); if (rawProfile) throw new Error('Historical NULL ID'); return operation([]); },
        ListMineralRecords: () => { calls.push('mineral'); if (rawProfile) throw new Error('Historical BLOB/text depth'); return operation([]); },
        ListOtherRecords: () => { calls.push('other'); return operation([]); },
        ListAuditEntries: () => { calls.push('audit'); return operation([]); },
        GetChildCapabilities: kind => { calls.push(`cap:${kind}`); return operation({ horizon: true }); },
      },
    });
    await context.actions.loadChildData(fixture.owner.plot);
    assert.deepEqual(calls, rawProfile
      ? ['veg', 'other', 'audit', 'cap:Veg', 'cap:Humus', 'cap:Mineral', 'cap:Other']
      : ['veg', 'humus', 'mineral', 'other', 'audit', 'cap:Veg', 'cap:Humus', 'cap:Mineral', 'cap:Other']);
    assert.equal(context.childRevision, 1);
    assert.deepEqual(Array.from(context.humusList), []);
    assert.deepEqual(Array.from(context.mineralList), []);
  }
});

test('physical-soil profile propagates unrelated read failures without becoming an empty success', async () => {
  for (const failing of ['veg', 'other', 'audit', 'cap:Humus']) {
    const read = kind => kind === failing ? operation(Promise.reject(new Error(`physical ${kind} read failure`))) : operation([]);
    const context = componentFunctions('FS882Form.svelte', ['loadChildData'], {
      loadRequest: 1, childRevision: 0, siviIdentityEnabled: false, ordinarySoilUnavailable: 'Unavailable ordinary soil profile.',
      reads: { track: value => value }, PlotService: {
        ListVegRecords: () => read('veg'), ListOtherRecords: () => read('other'), ListAuditEntries: () => read('audit'),
        GetChildCapabilities: kind => kind === 'Humus' ? read('cap:Humus') : operation({ horizon: true }),
      },
    });
    await assert.rejects(context.actions.loadChildData(fixture.owner.plot), new RegExp(`physical ${failing} read failure`));
    assert.equal(context.childRevision, 0);
  }
});

test('ordinary soil controls, empty placeholders and layer counts are excluded only by the usable default-off raw profile', () => {
  const source = read('FS882Form.svelte');
  assert.match(source, /ordinarySoilUnavailable = twoPageSoilReviewEnabled && twoPageEntryReviewEnabled/);
  assert.match(source, /soilEditingDisabled = \$derived\(!!ordinarySoilUnavailable \|\| !soilEditingEnabled/);
  assert.match(source, /if \(ordinarySoilUnavailable \|\| !soilEditingEnabled \|\| soilReferenceBusy\) return/);
  assert.match(source, /historical rows are not absent or discarded/);
  const actions = source.match(/\{#if !ordinarySoilUnavailable\}\s*<details[\s\S]*?<\/details>\s*\{\/if\}/)?.[0];
  assert.ok(actions, 'Ordinary soil actions require the explicit profile availability guard');
  assert.match(actions, /Humus Layers \(\{humusList.length\}\)/);
  assert.match(actions, /Mineral Layers \(\{mineralList.length\}\)/);
  assert.match(source, /\{:else if ordinarySoilUnavailable && \(child\.form === 'SoilHumusXL'/);
  assert.match(source, /\{#if ordinarySoilUnavailable\}\s*<p role="status" data-soil-ordinary-unavailable=\{child\.form\}/);
});

test('profile blocks ordinary soil callbacks before transport, confirmations or reference reads', async () => {
  const reason = 'Ordinary soil rows unavailable in physical review profile.';
  for (const kind of ['humus', 'mineral']) {
    const context = componentFunctions('FS882Form.svelte',
      ['openNewChild', 'createChild', 'editSourceChild', 'deleteSourceChild', 'stageSoilCell', 'saveSoilDrafts', 'loadSoilSuggestions'], {
        ordinarySoilUnavailable: reason, soilEditingDisabled: true, soilUnsaved: false,
        childEditingDisabled: false, newChild: kind, error: null, childRevision: 0,
        soilEditingEnabled: true, soilReferenceBusy: false,
        PlotService: new Proxy({}, { get() { throw new Error('No ordinary soil transport permitted'); } }),
        window: { confirm() { throw new Error('No confirmation permitted'); } },
      });
    context.actions.openNewChild(kind);
    assert.match(context.error, /available child workflow/);
    await context.actions.createChild('H', 'text');
    assert.equal(context.error, reason);
    const form = kind === 'humus' ? 'SoilHumusXL' : 'SoilMineralXL';
    await context.actions.editSourceChild(form, 1, 'Horizon', 'H');
    assert.equal(context.error, reason);
    await context.actions.deleteSourceChild(form, 1);
    assert.match(context.error, /available child workflow/);
    context.actions.stageSoilCell(kind === 'humus' ? 'Humus' : 'Mineral', 1, 'Horizon', 'H');
    assert.match(context.error, /Soil draft failed.*unavailable/);
    await context.actions.saveSoilDrafts();
    assert.match(context.error, /available references/);
    await context.actions.loadSoilSuggestions();
    assert.equal(context.childRevision, 1);
  }
});
