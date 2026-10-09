const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const client = require('svelte/internal/client');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { TwoPageVegetationSession, wire, editor, cell } = require('./twoPageVegetationTestHelpers.cjs');
const { metadata } = require('./siviParentTestHelpers.cjs');
const fixtures = require('./twoPageVegetationReceipts.json');
const read = name => readFileSync(path.join(__dirname, name), 'utf8');
const presentation = loadTypeScript('twoPageVegetationPresentation.ts', {
  './twoPageVegetationWire': wire, './twoPageVegetationEditor': editor, './projectMetadataEditor': metadata,
});
const collected = loadTypeScript('collectedEditor.ts');
const saved = (index = 3) => structuredClone(fixtures[index]);
const request = saved().history.Request;
const { contextId, project, plot, form } = request.original;
const props = { contextId, project, plot, form, parentBlocked: false };
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
const bindings = {
  SubVegAXL_BC: ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB', 'Collected'],
  SubVegAXL: ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB', 'Collected'],
  SubVegCXL: ['Cover6', 'Collected'],
  SubVegDXL: ['Cover7', 'Cover8', 'Cover9', 'Collected'],
};
function source(sourceForm = form) {
  const original = structuredClone(request.original);
  original.form = sourceForm;
  original.recordSource = wire.twoPageVegetationSource(sourceForm).source;
  return original;
}
function owned(original = source(), ports = {}) {
  const { contextId, project, plot, form } = original;
  return new TwoPageVegetationSession({ contextId, project, plot, form }, {
    read: () => operation(client.proxy(original)),
    save: () => { throw new Error('Unexpected Save'); }, outcome: () => { throw new Error('Unexpected recovery'); }, ...ports,
  }, true);
}
const dependencies = session => ({
  './twoPageVegetationSessions': { twoPageVegetationSession: () => session, twoPageVegetationWritingEnabled: true },
  './twoPageVegetationWire': wire, './projectMetadataEditor': metadata,
  './collectedEditor': collected, './twoPageVegetationPresentation': presentation,
  './VegetationSpeciesReferences.svelte': { default: serverComponent(read('VegetationSpeciesReferences.svelte'), 'VegetationSpeciesReferences.svelte', {}) },
});

test('all four source presentations retain every25 field exactly once with labels NULL and responsive groups', async () => {
  for (const sourceForm of Object.keys(bindings)) {
    const session = owned(source(sourceForm)); await session.load();
    const fields = presentation.twoPageVegetationGroups(sourceForm).flatMap(group => group.fields.map(field => field[0]));
    assert.deepEqual(Array.from(fields).sort(), bindings[sourceForm].slice().sort());
    const html = render(serverComponent(read('TwoPageVegetationPanel.svelte'), 'TwoPageVegetationPanel.svelte',
      dependencies(session)), { props: { ...props, form: sourceForm } }).body;
    for (const column of bindings[sourceForm]) assert.equal((html.match(new RegExp(`data-vegetation-column="${column}"`, 'g')) ?? []).length, 1, column);
    assert.match(html, /Species \(read only\)/);
    assert.match(html, /NULL \(not empty text\)/);
    assert.match(html, /grid-cols-1/);
    assert.match(html, /sm:grid-cols-2/);
    assert.match(html, /Save is explicit/);
    assert.match(html, /not source Species sorting/);
    assert.match(html, /calibrated SINGLE conversion/);
    assert.match(html, /Review Species definitions \(read only\)/);
    assert.match(html, /creation, deletion and restoration remain unavailable/);
  }
});

test('physical aliases NULL IDs and missing source controls remain distinct without inventing zero or writable rows', async () => {
  for (const mutate of [
    original => { original.records.rows[0].cells[0] = cell(); },
    original => {
      const index = wire.vegetationColumnIndex(original, 'Collected');
      original.records.columns.splice(index, 1);
      for (const row of original.records.rows) row.cells.splice(index, 1);
    },
  ]) {
    const original = source(); mutate(original);
    const session = owned(original); await session.load();
    const html = render(serverComponent(read('TwoPageVegetationPanel.svelte'), 'TwoPageVegetationPanel.svelte',
      dependencies(session)), { props }).body;
    assert.match(html, /Read-only physical row/);
    assert.ok(!/<input[^>]*type="text"(?![^>]*disabled)[^>]*>/.test(html));
  }
  const original = source();
  original.records.columns[2].name = '\u017fpecies';
  const session = owned(original); await session.load();
  const html = render(serverComponent(read('TwoPageVegetationPanel.svelte'), 'TwoPageVegetationPanel.svelte',
    dependencies(session)), { props }).body;
  assert.match(html, /Species \(read only\): TEST/);
  assert.doesNotMatch(html, /source column unavailable/);
});

async function mountScript(session, parent = false, dom = {}, sourceProps = props) {
  let script = read('TwoPageVegetationPanel.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, parentBlocked }', 'form, parentBlocked, inspect }');
  script += '\nfunction bindTestPanel(value) { panel = value; }\n$effect(() => inspect({ operation, stage, selectRow, cycleCollected, bindTestPanel, row, error, view, close, fieldsDisabled }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('panel.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const mount = new Function('$', 'untrack', 'twoPageVegetationSession', 'twoPageVegetationWritingEnabled',
    'vegetationColumnIndex', 'vegetationSourceCell', 'metadataCellText', 'collectedNext',
    'twoPageVegetationGroups', 'twoPageVegetationStoredText', 'twoPageVegetationRowDiagnostic', 'twoPageVegetationHasColumn',
    'HTMLButtonElement', 'crypto', code + '\nreturn _unknown_;')(
    client, client.untrack, typeof session === 'function' ? session : () => session, true, wire.vegetationColumnIndex, wire.vegetationSourceCell,
    metadata.metadataCellText, collected.collectedNext, presentation.twoPageVegetationGroups,
    presentation.twoPageVegetationStoredText, presentation.twoPageVegetationRowDiagnostic, presentation.twoPageVegetationHasColumn,
    dom.HTMLButtonElement, { randomUUID: () => request.requestId });
  const blocked = client.state(parent);
  let actions;
  const dispose = client.effect_root(() => mount(null, {
    ...sourceProps, get parentBlocked() { return client.get(blocked); }, inspect: value => { actions = value; },
    get contextId() { return sourceProps.contextId; }, get project() { return sourceProps.project; },
    get plot() { return sourceProps.plot; }, get form() { return sourceProps.form; },
  }));
  client.flush();
  return { actions: () => actions, dispose, blocked };
}

test('actual client scripts retain invalid drafts through remount and keep parent blocks from disabling correction Undo', async () => {
  const session = owned(); await session.load();
  let mounted = await mountScript(session);
  try {
    mounted.actions().stage('Cover1', '100', false); client.flush();
    assert.equal(mounted.actions().close.blocked, true);
    mounted.dispose(); mounted = await mountScript(session);
    assert.equal(Object.values(mounted.actions().view.drafts)[0].raw, '100');
    client.set(mounted.blocked, true); client.flush();
    mounted.actions().stage('Cover1', '5', false); client.flush();
    assert.equal(Object.values(session.view().drafts)[0].raw, '100');
    assert.match(mounted.actions().error, /parent/);
    await mounted.actions().operation('undo'); client.flush();
    assert.equal(Object.keys(session.view().drafts).length, 0);
    client.set(mounted.blocked, false); client.flush();
    mounted.actions().stage('Cover1', '5', false); client.flush();
    assert.equal(mounted.actions().close.canSave, true);
  } finally { mounted.dispose(); }
});

test('actual reactive owner switch retains the newer panel error when an older reference read finishes', async () => {
  for (const completion of ['resolve', 'reject']) {
    let release, reject;
    const wait = Object.assign(new Promise((yes, no) => { release = yes; reject = no; }), { cancel() {} });
    const first = owned(source(), { references: () => wait });
    const nextSource = source(); nextSource.contextId = 'new owner';
    const next = owned(nextSource, {
      references: () => operation(null),
    });
    const context = client.state(contextId);
    const mounted = await mountScript(value => value.contextId === contextId ? first : next, false, {}, {
      ...props, get contextId() { return client.get(context); },
    });
    try {
      const old = mounted.actions().operation('references');
      client.set(context, 'new owner'); client.flush();
      await mounted.actions().operation('references'); client.flush();
      const error = mounted.actions().error;
      assert.match(error, /definitions unavailable/);
      if (completion === 'resolve') {
        const data = structuredClone(require('./twoPageVegetationReferenceWire.json'));
        Object.assign(data, { contextId, project, plot, form });
        release(client.proxy(data));
      } else reject(new Error('old owner source failure'));
      await old; client.flush();
      assert.equal(mounted.actions().error, error);
      assert.equal(mounted.actions().view.references, null);
      assert.equal(mounted.actions().view.referenceError, next.view().referenceError);
    } finally { mounted.dispose(); }
  }
});

test('actual client last-cover Save/remount pins a physical row outside the source query without selecting its peer', async () => {
  const result = saved(4), original = result.history.Request.original;
  const session = owned(original, { save: () => operation(client.proxy(result)) }); await session.load();
  let mounted = await mountScript(session, false, {}, { ...props, form: 'SubVegCXL' });
  try {
    mounted.actions().stage('Cover6', '', true);
    mounted.actions().stage('Collected', 'C', false);
    await mounted.actions().operation('save'); client.flush();
    assert.equal(mounted.actions().row.rowId, '-7');
    assert.equal(session.view().original.sourceRowIds.includes('-7'), false);
    assert.equal(mounted.actions().fieldsDisabled, true);
    await mounted.actions().operation('acknowledge'); client.flush();
    assert.equal(mounted.actions().row.rowId, '-7');
    mounted.dispose(); mounted = await mountScript(session, false, {}, { ...props, form: 'SubVegCXL' });
    assert.equal(session.view().receipts.length, 1);
    assert.equal(mounted.actions().row.rowId, '-7');
    assert.equal(mounted.actions().fieldsDisabled, true);
  } finally { mounted.dispose(); }
});

test('source Collected cycle checks independently observed row/control/value and only stages without Save', async () => {
  let saves = 0;
  const session = owned(source(), { save: () => { saves++; throw new Error('Unexpected Save'); } }); await session.load();
  class Button { disabled = false; checkVisibility() { return true; } }
  const button = new Button(), input = { disabled: false, checkVisibility: () => true, value: '' };
  class Panel {
    dataset = { twoPageVegetationPanel: form, vegetationCurrentRow: '-7' };
    contains(value) { return value === button; }
    querySelector() { return input; }
  }
  const panel = new Panel();
  const mounted = await mountScript(session, false, { HTMLButtonElement: Button });
  try {
    mounted.actions().bindTestPanel(panel); client.flush();
    for (const [expected, displayed] of [['C', 'C'], ['V', 'V'], [null, '']]) {
      mounted.actions().cycleCollected('-7', { currentTarget: button }); client.flush();
      const draft = Object.values(session.view().drafts).find(value => value.column === 'Collected');
      assert.equal(draft?.value.text ?? null, expected);
      input.value = displayed;
    }
    assert.equal(Object.keys(session.view().drafts).length, 0);
    input.value = 'unobserved input';
    mounted.actions().cycleCollected('-7', { currentTarget: button }); client.flush();
    assert.match(mounted.actions().error, /display differs/);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    input.value = ''; panel.dataset.vegetationCurrentRow = '3';
    mounted.actions().cycleCollected('-7', { currentTarget: button }); client.flush();
    assert.match(mounted.actions().error, /physical row differs/);
    assert.equal(saves, 0);
  } finally { mounted.dispose(); }
});

test('parent and root barriers block hidden retained vegetation drafts while preserving ordinary default behavior', async () => {
  const parent = componentFunctions('FS882Form.svelte', ['getCloseState'], {
    twoPageVegetationBlocked: true, twoPageVegetationBusy: false, error: null,
  });
  assert.equal(parent.actions.getCloseState().blocked, true);
  assert.match(parent.actions.getCloseState().saveReason, /linked-vegetation/);
  let calls = 0;
  const root = componentFunctions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy'], {
    twoPageVegetationBusy: true, pictureCreationBusy: false, pictureLifecycleBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    ProjectService: { ListPlots: () => { calls++; } },
  });
  await root.actions.loadPlots(0); await root.actions.refresh(); await root.actions.loadHierarchy();
  assert.equal(calls, 0);
  const edit = componentFunctions('FS882Form.svelte', ['editSourceChild'], {
    ordinaryVegetationUnavailable: 'Physical source review enabled', error: null,
  });
  await edit.actions.editSourceChild('SubVegAXL', 0, 'Collected', 'C');
  assert.equal(edit.error, 'Physical source review enabled');
});

test('ordinary vegetation read profile bypasses numeric readers only when both review and complete-entry flags are on', async () => {
  for (const review of [false, true]) for (const parent of [false, true]) for (const entry of [false, true]) {
    const root = read('FS882Form.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
    function declarations(source, names) {
      const ast = ts.createSourceFile('profile.ts', source, ts.ScriptTarget.Latest, true);
      const selected = ast.statements.filter(node => ts.isVariableStatement(node) &&
        node.declarationList.declarations.some(declaration => names.includes(declaration.name.getText(ast))));
      assert.equal(selected.length, names.length);
      return selected.map(node => node.getText(ast).replace(/^export /, '')).join('\n');
    }
    let profile;
    const script = `<script lang="ts">let { inspect } = $props();
      const vegList = [];
      ${declarations(read('twoPageVegetationSessions.ts'), ['twoPageVegetationReviewEnabled'])}
      ${declarations(root, ['twoPageReviewEnabled', 'twoPageEntryReviewEnabled', 'ordinaryVegetationUnavailable'])}
      const vegetationReadUnavailable = ordinaryVegetationUnavailable;
      ${declarations(root, ['totalTreeCover', 'totalShrubCover', 'totalHerbCover', 'totalMossCover'])}
      function inspectProfile() { inspect({ ordinaryVegetationUnavailable, totals: [totalTreeCover, totalShrubCover, totalHerbCover, totalMossCover] }); } inspectProfile();</script>`;
    const body = render(serverComponent(script, 'VegetationProfileTest.svelte', {}, {
      VITE_TWO_PAGE_VEGETATION_REVIEW: String(review), VITE_TWO_PAGE_PARENT_REVIEW: String(parent),
      VITE_TWO_PAGE_PARENT_ENTRY_REVIEW: String(entry),
    }), { props: { inspect: value => { profile = value; } } }).body;
    assert.equal(typeof body, 'string');
    assert.ok(profile);
    const unavailable = profile.ordinaryVegetationUnavailable;
    assert.equal(typeof unavailable === 'string', review && parent && entry);
    assert.deepEqual(Array.from(profile.totals), review && parent && entry ? [null, null, null, null] : [0, 0, 0, 0]);
    let ordinary = 0, availability = 0;
    const readPort = value => Promise.resolve(value);
    const context = componentFunctions('FS882Form.svelte', ['loadChildData'], {
      loadRequest: 1, ordinaryVegetationUnavailable: unavailable, siviIdentityEnabled: false, siviContextId: contextId,
      reads: { track: readPort }, childRevision: 0,
      PlotService: {
        ListVegRecords: () => { ordinary++; return readPort([{ id: 42 }]); },
        ListHumusRecords: () => readPort([]), ListMineralRecords: () => readPort([]), ListOtherRecords: () => readPort([]),
        ListAuditEntries: () => readPort([]), GetChildCapabilities: () => readPort({}),
      }, ContextService: { GetVegetationReadAvailability: () => { availability++; throw new Error('Unexpected fallback'); } },
    });
    await context.actions.loadChildData(plot);
    assert.equal(ordinary, review && parent && entry ? 0 : 1);
    assert.equal(availability, 0);
    assert.equal(context.vegetationReadUnavailable, unavailable);
    if (unavailable) assert.equal(context.vegList.length, 0);
  }
});

test('actual root native-close and context transition barriers survive unmounted failed vegetation Save and certified absence', async () => {
  const cancellations = [], transitions = [];
  const context = componentFunctions('App.svelte', ['handleCloseRequest', 'requestTransition', 'loadPlots', 'refresh', 'loadHierarchy'], {
    view: 'fs882', localEditorBusy: false, pictureCreationBusy: false, pictureLifecycleBusy: false,
    busy: false, editor: { getCloseState: () => ({ busy: false, blocked: false, unsaved: false, canSave: false, error: null }) },
    transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    pendingTransition: null, closeWorking: false, closeRequest: '', closeError: '', error: '', transitionError: '',
    request: 0, stateRequest: 0, hierarchyRequest: 0, $projectState: { contextId },
    document: { activeElement: null }, HTMLElement: class {},
    closeDisposition: loadTypeScript('closeLifecycle.ts').closeDisposition, tick: async () => {},
    CloseService: { CancelClose: id => { cancellations.push(id); }, GetPendingCloseRequest: () => operation('vegetation-close') },
  });
  const expression = read('App.svelte').match(/const editorBusy = \$derived\(([^;]+)\);/)[1];
  const aggregate = new Function('localEditorBusy', 'pictureCreationBusy', 'pictureLifecycleBusy', 'twoPageSoilBusy', 'twoPageVegetationBusy', `return ${expression}`);
  Object.defineProperty(context, 'editorBusy', {
    get: () => aggregate(context.localEditorBusy, context.pictureCreationBusy, context.pictureLifecycleBusy, context.twoPageSoilBusy, context.twoPageVegetationBusy),
  });
  const session = owned(source(), {
    save: () => Promise.reject(new Error('unknown delivery')),
    outcome: json => operation({ ...structuredClone(require('./twoPageVegetationOutcomes.json')[2]), requestJSON: json }),
  });
  const stop = session.subscribe(() => {
    const state = session.closeState();
    context.twoPageVegetationBusy = state.busy || state.blocked || state.unsaved;
  });
  async function blocked() {
    assert.equal(context.editorBusy, true);
    await context.actions.handleCloseRequest('vegetation-close');
    assert.equal(cancellations.at(-1), 'vegetation-close');
    await context.actions.requestTransition(async () => transitions.push('changed context'));
    await context.actions.loadPlots(0); await context.actions.refresh(); await context.actions.loadHierarchy();
    assert.equal(transitions.length, 0);
    assert.equal(context.request, 0); assert.equal(context.stateRequest, 0); assert.equal(context.hierarchyRequest, 0);
  }
  try {
    await session.load(); session.stage('-7', 'Cover1', '100', false); await blocked();
    session.stage('-7', 'Cover1', '5', false);
    await session.save(request.requestId); await blocked();
    assert.equal(await session.resolve(), true); await blocked();
    session.undo();
    assert.equal(context.editorBusy, false);
    await context.actions.requestTransition(async () => transitions.push('changed context'));
    assert.deepEqual(transitions, ['changed context']);
  } finally { stop(); }
});
