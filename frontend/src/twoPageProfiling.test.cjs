const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const definition = require('../../resources/two-page-plot-profiling-fields.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const vegetation = loadTypeScript('twoPageVegetationTable.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const api = loadTypeScript('twoPageProfiling.ts', {
  '../../resources/two-page-plot-profiling-fields.json': definition, './siviParentTransport': transport,
  './qualityEditor': quality, './twoPageVegetationTable': vegetation,
}, { setTimeout });
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageProfilingWire.json'), 'utf8'));
const ownerOf = value => ({
  ...Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath', 'plot'].map(name => [name, value[name]])),
  profileSource: { ...value.source },
});
const owner = ownerOf(fixture().default);
const validate = value => api.twoPageProfilingFromWire(value, ownerOf(value), value.offset, value.limit);
const indexOf = (value, column) => value.columns.findIndex(item => item.name === column);

test('actual-Go default/page/beyond-end/None/absent and50+20 wires preserve complete immutable originals', async () => {
  const wire = fixture(), before = JSON.stringify(wire);
  for (const [name, rows, total] of [
    ['default', 4, 4], ['page', 1, 4], ['end', 0, 4], ['none', 50, 70],
    ['absent', 50, 70], ['pagingFirst', 50, 70], ['pagingNext', 20, 70],
  ]) {
    const view = await validate(wire[name]);
    assert.equal(view.profileOriginal.rows.length, rows); assert.equal(view.totalRows, total);
    assert.equal(view.envOriginal.rows.length, 1); assert.equal(view.adminOriginal.rows.length, 1);
    assert.equal(view.descriptionsPresent, name !== 'absent');
    assert.equal(view.descriptionOriginal.rows.length, name === 'absent' ? 0 : 3);
    assert.doesNotThrow(() => structuredClone(view));
    view.envOriginal.rows[0].cells[0].text = 'caller mutation';
    if (rows) view.profileOriginal.rows[0].cells[0].text = 'caller mutation';
    view.profileSource.name = 'caller mutation';
  }
  assert.equal(JSON.stringify(wire), before);
});
for (const [name, mutate] of [
  ['context', v => v.contextId = 'foreign'], ['project', v => v.project = 'foreign'],
  ['project path', v => v.projectPath += '.foreign'], ['SU', v => v.su = 'None'],
  ['SU path', v => v.suPath += '.foreign'], ['plot', v => v.plot = '0108050'],
  ['source name', v => v.source.name = 'Other'], ['source path', v => v.source.path += '.other'],
  ['source None', v => v.source.name = 'None'], ['target', v => v.targetForm = 'Profile'],
  ['table', v => v.profileTable += '.other'], ['offset', v => v.offset++],
  ['limit', v => v.limit = 201], ['negative total', v => v.totalRows = -1],
  ['fractional total', v => v.totalRows = 1.1], ['excessive total', v => v.totalRows = 100001],
  ['source execution claim', v => v.orderingPolicy = 'Order'],
  ['short page', v => v.profileOriginal.rows.pop()], ['sparse page', v => delete v.profileOriginal.rows[1]],
  ['duplicate physical row', v => v.profileOriginal.rows[1] = structuredClone(v.profileOriginal.rows[0])],
  ['reversed row order', v => v.profileOriginal.rows.reverse()],
  ['missing parent', v => v.envOriginal.rows = []],
  ['duplicate parent', v => v.adminOriginal.rows.push(structuredClone(v.adminOriginal.rows[0]))],
  ['numeric plot', v => v.envOriginal.rows[0].cells[indexOf(v.envOriginal, 'PlotNumber')] = cell('integer', '108050')],
  ['foreign parent', v => v.adminOriginal.rows[0].cells[indexOf(v.adminOriginal, 'Plot')] = cell('text', 'foreign')],
  ['missing binding', v => v.profileOriginal.columns[indexOf(v.profileOriginal, 'Species')].name = 'Missing'],
  ['wrong schema wire', v => { v.profileOriginal.columns[0].type = ''; delete v.profileOriginal.columns[0].declaredType; }],
  ['duplicate schema', v => v.profileOriginal.columns[1].name = v.profileOriginal.columns[0].name],
  ['case ambiguous schema', v => v.profileOriginal.columns[1].name = 'order'],
  ['sparse schema', v => delete v.profileOriginal.columns[1]], ['sparse cells', v => delete v.profileOriginal.rows[0].cells[1]],
  ['malformed schema Unicode', v => v.profileOriginal.columns[0].name = '\ud800'],
  ['malformed source Unicode', v => v.profileOriginal.rows[0].cells[1] = cell('text', '\ud800')],
  ['invented absent metadata', v => v.descriptionsPresent = false],
  ['numeric Description table', v => v.descriptionOriginal.rows[0].cells[0] = cell('integer', '0')],
  ['foreign Description table', v => v.descriptionOriginal.rows[0].cells[0] = cell('text', 'sample_profile')],
  ['Description terminal space', v => v.descriptionOriginal.rows[0].cells[0] = cell('text', 'Sample_Profile ')],
  ['missing Description schema', v => v.descriptionOriginal.columns[1].name = 'Missing'],
  ['invalid Description presence', v => v.descriptionsPresent = null],
]) test(`strict profiling transport rejects ${name}`, async () => {
  const value = fixture().default; mutate(value);
  await assert.rejects(api.twoPageProfilingFromWire(value, owner, 0, 50));
});
test('invalid immutable owner/page and invented beyond-end rows are refused', async () => {
  for (const invalid of [
    { ...owner, contextId: '' }, { ...owner, projectPath: '' }, { ...owner, suPath: '' },
    { ...owner, plot: '\ud800' }, { ...owner, profileSource: { name: 'None', path: '' } },
    { ...owner, profileSource: { name: 'Sample', path: '' } },
  ]) await assert.rejects(api.twoPageProfilingFromWire(fixture().default, invalid, 0, 50));
  for (const [offset, limit] of [[-1, 50], [0.1, 50], [100001, 50], [0, 0], [0, 201], [0, 1.1]]) {
    await assert.rejects(api.twoPageProfilingFromWire(fixture().default, owner, offset, limit));
  }
  const end = fixture().end; end.profileOriginal.rows.push(fixture().default.profileOriginal.rows[0]);
  await assert.rejects(validate(end));
});
test('source nine labels retain logical order, duplicate/NULL/historical Order and stored counts', async () => {
  const view = await validate(fixture().default), rows = view.profileOriginal.rows;
  const fields = api.twoPageProfilingRowFields(view, rows[0]);
  assert.deepEqual(Array.from(fields, f => f.label), ['Order', 'Table', 'Field', 'Species', 'Layer', 'Operator', 'Criteria', 'Operation', 'Plot Count']);
  assert.equal(new Set(fields.map(f => f.controlId)).size, 9);
  assert.ok(fields.every(f => f.cell));
  const order = indexOf(view.profileOriginal, 'Order');
  assert.deepEqual(Array.from(rows, row => row.cells[order].storage), ['null', 'integer', 'integer', 'text']);
  assert.equal(rows[1].cells[order].integer, '10'); assert.equal(rows[2].cells[order].integer, '10');
  assert.equal(rows[3].cells[order].text, 'not integer');
  assert.equal(rows[0].cells[indexOf(view.profileOriginal, 'Criteria')].text.length, 256);
  assert.equal(rows[0].cells[indexOf(view.profileOriginal, 'Extra')].blobHex, 'ff00');
  assert.equal(rows[1].cells[indexOf(view.profileOriginal, 'PlotCount')].integer, '-1');
  assert.equal(rows[2].cells[indexOf(view.profileOriginal, 'PlotCount')].text, 'historical count');
  assert.throws(() => api.twoPageProfilingRowFields(view, structuredClone(rows[0])));
});
test('Description validation yields across chunks and refuses cross-chunk physical duplicates', async () => {
  const value = fixture().default;
  value.descriptionOriginal.rows = Array.from({ length: 201 }, (_, i) => ({
    rowId: String(i + 1), cells: structuredClone(value.descriptionOriginal.rows[0].cells),
  }));
  let yields = 0;
  const view = await api.twoPageProfilingFromWire(value, owner, 0, 50, async () => { yields++; });
  assert.equal(view.descriptionOriginal.rows.length, 201); assert.equal(yields, 5);
  value.descriptionOriginal.rows[200].rowId = '1';
  await assert.rejects(validate(value));
});

async function mountPanel(service, validator = api.twoPageProfilingFromWire) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageProfilingPanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  const ast = ts.createSourceFile('profiling-panel.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n').replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageProfilingService', 'twoPageProfilingFromWire',
    code + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack, reads.ReadRequests, service, validator);
  let actions; const activity = [];
  const dispose = runtime.effect_root(() => mount(null, { ...owner, form: 'FS882-8x6XL-CHARS', onBusyChange: v => activity.push(v), inspect: v => actions = v }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}
test('actual Svelte runtime explicit Load keeps50/20 Go responses cloneable and does not run/select profiles', async () => {
  const calls = [], wire = fixture();
  const panel = await mountPanel({ GetReview: (...args) => { calls.push(args); return Promise.resolve(args[2] === 0 ? wire.pagingFirst : wire.pagingNext); } });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().review.profileOriginal.rows.length, 50);
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
    await panel.actions().load(50); panel.flush();
    assert.equal(panel.actions().review.profileOriginal.rows.length, 20);
    assert.deepEqual(calls.map(args => args.slice(2)), [[0, 50], [50, 50]]);
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().busy, false);
  } finally { panel.dispose(); }
});
test('actual Svelte runtime retains server/wire errors and permits correction/retry', async () => {
  let mode = 'server';
  const panel = await mountPanel({ GetReview: () => {
    if (mode === 'server') return Promise.reject(new Error('explicit profile unavailable'));
    const value = fixture().default;
    if (mode === 'wire') value.source.path = 'foreign';
    return Promise.resolve(value);
  } });
  try {
    await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /explicit profile unavailable/);
    assert.equal(panel.actions().review, null);
    mode = 'wire'; await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /foreign owner/);
    mode = 'valid'; await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().review.profileOriginal.rows.length, 4);
  } finally { panel.dispose(); }
});
for (const retire of ['cancel', 'destroy']) test(`actual Svelte ${retire}/remount rejects held profile response without replay`, async () => {
  let release, cancellations = 0, calls = 0;
  const held = new Promise(resolve => release = resolve); held.cancel = () => cancellations++;
  const service = { GetReview: () => { calls++; return held; } };
  const panel = await mountPanel(service), pending = panel.actions().load(); panel.flush();
  const old = panel.actions();
  if (retire === 'cancel') old.cancel(); else panel.dispose();
  const fresh = await mountPanel(service);
  try {
    release(fixture().default); await pending; panel.flush(); fresh.flush();
    assert.equal(old.current(), null); assert.equal(fresh.actions().review, null);
    assert.equal(calls, 1); assert.equal(cancellations, 1);
  } finally { fresh.dispose(); if (retire === 'cancel') panel.dispose(); }
});
test('actual Svelte remount during held validation yield cannot receive the old completed result', async () => {
  let entered, release;
  const reached = new Promise(resolve => entered = resolve), held = new Promise(resolve => release = resolve);
  const validator = (value, owned, offset, limit, yieldRead) => api.twoPageProfilingFromWire(value, owned, offset, limit, async () => {
    entered(); await held; await yieldRead();
  });
  const service = { GetReview: () => Promise.resolve(fixture().default) };
  const panel = await mountPanel(service, validator), pending = panel.actions().load(); panel.flush();
  await reached; const old = panel.actions(); panel.dispose();
  const fresh = await mountPanel(service);
  try {
    release(); await pending; panel.flush(); fresh.flush();
    assert.equal(old.current(), null); assert.equal(fresh.actions().review, null);
  } finally { fresh.dispose(); }
});
test('mapped profiling opener refuses scope/draft/peer/child errors and pending Return', () => {
  const create = input => componentFunctions('TwoPageEntryEditor.svelte', ['openProfiling', 'returnFromProfiling'], {
    profilingReaderAvailable: true, profileSource: owner.profileSource,
    vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
    sourceBlocked: false, childBlocked: false, profilingOpen: false, profilingBusy: false, error: null, onchange() {},
    getDraftState: () => ({ unsaved: false, busy: false, blocked: false, error: null, ...input.draft }),
    sessions: new Map([[0, { view: () => input.peer ?? {} }]]), ...input,
  });
  for (const input of [
    { profilingReaderAvailable: false }, { profileSource: null }, { profileSource: { name: 'None', path: '' } },
    { profileSource: { name: 'Sample', path: '' } }, { vegetationScope: { projectPath: '', su: 'None', suPath: '' } },
    { vegetationScope: { projectPath: 'owned', su: 'Report', suPath: '' } }, { sourceBlocked: true }, { childBlocked: true },
    { draft: { unsaved: true } }, { draft: { busy: true } }, { draft: { blocked: true } }, { draft: { error: 'invalid' } },
    { peer: { error: 'invalid' } }, { peer: { referenceError: 'invalid' } },
  ]) {
    const panel = create(input); panel.actions.openProfiling();
    assert.equal(panel.profilingOpen, false); assert.match(panel.error, /Resolve/);
  }
  const panel = create({ profilingBusy: true }); panel.actions.openProfiling();
  assert.equal(panel.profilingOpen, true); panel.actions.returnFromProfiling();
  assert.equal(panel.profilingOpen, true); assert.match(panel.error, /cancel/);
  panel.profilingBusy = false; panel.actions.returnFromProfiling(); assert.equal(panel.profilingOpen, false);
});
test('profiling owns every coupled operation, staging, ProjectID, acknowledgement, variant and close path', async () => {
  let calls = 0;
  const panel = componentFunctions('TwoPageEntryEditor.svelte', [
    'operation', 'stage', 'selectProject', 'acknowledge', 'chooseForm', 'closeReview', 'getCloseState', 'getDraftState',
  ], {
    profilingOpen: true, profilingBusy: true, reviewEnabled: true, editingEnabled: true,
    sourceRefreshing: false, auditBusy: false, googleEarthBusy: false, externalOpen: false, auditOpen: false, googleEarthOpen: false,
    childBlocked: false, close: { unsaved: false, busy: false, blocked: false },
    view: { original: {}, referenceError: null }, error: null,
    sessions: new Map(), session: { stage() { calls++; }, selectProject() { calls++; }, acknowledge() { calls++; } },
    onclosed() { calls++; },
  });
  for (const action of ['load', 'save', 'undo', 'retain', 'prune', 'references']) await panel.actions.operation(action);
  panel.actions.stage('Zone', {}); panel.actions.selectProject('1'); panel.actions.acknowledge('Zone');
  await panel.actions.chooseForm('FS882-8x6XL-CHARS'); panel.actions.closeReview();
  assert.equal(calls, 0);
  assert.equal(panel.actions.getCloseState().busy, true);
  const state = panel.actions.getDraftState(); assert.equal(state.unsaved, true); assert.equal(state.busy, true); assert.equal(state.blocked, true);
});
test('production panel compiles warning-free with source-bound labels, complete originals and safety wording', () => {
  const source = readFileSync(path.join(__dirname, 'TwoPageProfilingPanel.svelte'), 'utf8');
  assert.equal(compile(source, { generate: 'client' }).warnings.length, 0);
  assert.match(source, /data-source-binding-control/); assert.match(source, /Complete physical originals/);
  assert.match(source, /Stored Plot Count is historical data/); assert.match(source, /not source Order execution/);
});
test('complete original disclosure renders only its local50-Description page while retaining all reviewed definitions', async () => {
  const { render } = require('svelte/server');
  const value = fixture().default;
  value.descriptionOriginal.rows = Array.from({ length: 201 }, (_, i) => ({
    rowId: String(i + 1), cells: structuredClone(value.descriptionOriginal.rows[0].cells),
  }));
  const view = await validate(value);
  const source = readFileSync(path.join(__dirname, 'TwoPageProfilingPanel.svelte'), 'utf8')
    .replace('form, onBusyChange }:', 'form, onBusyChange, initialReview, initialOpen, initialOffset }:')
    .replace('$state.raw<TwoPageProfilingView | null>(null)', '$state.raw<TwoPageProfilingView | null>(untrack(() => initialReview))')
    .replace('let originalsOpen = $state(false)', 'let originalsOpen = $state(untrack(() => initialOpen))')
    .replace('let descriptionOffset = $state(0)', 'let descriptionOffset = $state(untrack(() => initialOffset))');
  const Panel = serverComponent(source, 'TwoPageProfilingPanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { TwoPageProfilingService: {} },
    './twoPageProfiling': api,
  });
  const markup = (open, offset) => render(Panel, { props: { ...owner, form: 'FS882-8x6XL',
    onBusyChange() {}, initialReview: view, initialOpen: open, initialOffset: offset } }).body;
  assert.equal((markup(false, 0).match(/aria-label="Description physical row /g) ?? []).length, 0);
  const first = markup(true, 0), next = markup(true, 50), last = markup(true, 200);
  assert.equal((first.match(/aria-label="Description physical row /g) ?? []).length, 50);
  assert.equal((next.match(/aria-label="Description physical row /g) ?? []).length, 50);
  assert.equal((last.match(/aria-label="Description physical row /g) ?? []).length, 1);
  assert.match(first, /aria-label="Description physical row 1"/);
  assert.doesNotMatch(first, /aria-label="Description physical row 51"/);
  assert.match(next, /aria-label="Description physical row 51"/);
  assert.match(last, /aria-label="Description physical row 201"/);
  assert.equal(view.descriptionOriginal.rows.length, 201);
});
