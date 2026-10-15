const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const targets = require('./twoPageEnvironmentTargets.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const vegetation = loadTypeScript('twoPageVegetationTable.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const table = loadTypeScript('twoPageEnvironmentTable.ts', {
  './twoPageEnvironmentTargets.json': targets, './siviParentTransport': transport,
  './qualityEditor': quality, './twoPageVegetationTable': vegetation,
});
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageEnvironmentTableWire.json'), 'utf8'));
const ownerOf = value => Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath', 'plot'].map(key => [key, value[key]]));
const owner = ownerOf(fixture().default);
const validate = value => table.twoPageEnvironmentTableFromWire(value, ownerOf(value), value.offset, value.limit);

test('actual-Go four targets/page/end/None/50+20 wires retain complete immutable typed pair witnesses', () => {
  const wire = fixture(), before = JSON.stringify(wire), targetBefore = JSON.stringify(targets);
  for (const [name, rows, total, form] of [
    ['default', 13, 13, 'FS882-6x4'], ['xl', 13, 13, 'FS882-6x4XL'],
    ['normal', 13, 13, 'FS882-8x6XL'], ['chars', 13, 13, 'FS882-8x6XL-CHARS'],
    ['page', 2, 13, 'FS882-6x4'], ['empty', 0, 13, 'FS882-6x4'],
    ['none', 50, 70, 'FS882-6x4'], ['pagingFirst', 50, 70, 'FS882-6x4'], ['pagingNext', 20, 70, 'FS882-6x4'],
  ]) {
    const view = validate(wire[name]);
    assert.equal(view.rows.length, rows); assert.equal(view.totalRows, total); assert.equal(view.target.form, form);
    assert.doesNotThrow(() => structuredClone(view));
    view.target.fields[0].caption = 'caller mutation';
    if (rows) {
      view.envOriginal.rows[0].cells[0].text = 'caller mutation';
      view.rows[0].envRowId = 'caller mutation';
    }
  }
  assert.equal(JSON.stringify(wire), before); assert.equal(JSON.stringify(targets), targetBefore);
  assert.deepEqual(Array.from(validate(wire.default).rows, row => row.envRowId),
    ['5', '2', '2', '2', '3', '3', '3', '6', '7', '-1', '4', '4', '4']);
  assert.deepEqual(Array.from(validate(wire.normal).rows, row => row.envRowId),
    ['6', '-1', '7', '2', '2', '2', '3', '3', '3', '4', '4', '4', '5']);
  const admin = validate(wire.default).adminOriginal, row = admin.rows.find(row => row.rowId === '-2');
  assert.equal(row.cells[admin.columns.findIndex(column => column.name === 'ID')].storage, 'null');
  assert.equal(row.cells[admin.columns.findIndex(column => column.name === 'Flag')].integer, '-1');
  assert.equal(row.cells[admin.columns.findIndex(column => column.name === 'Unmapped')].blobHex, 'ff00');
  assert.equal(table.twoPageEnvironmentCellLabel(cell()), '(NULL)');
  assert.equal(table.twoPageEnvironmentCellLabel(cell('text', '')), '(empty text)');
});
for (const [name, mutate] of [
  ['context', value => value.contextId = 'foreign'], ['project', value => value.project = 'foreign'],
  ['project path', value => value.projectPath += '.foreign'], ['SU', value => value.su = 'None'],
  ['SU path', value => value.suPath += '.foreign'], ['opening plot', value => value.plot = '0108050'],
  ['dynamic target', value => value.targetForm = 'unknown'], ['Env table', value => value.envTable += '.foreign'],
  ['Admin table', value => value.adminTable += '.foreign'], ['offset', value => value.offset++],
  ['limit', value => value.limit = 201], ['negative total', value => value.totalRows = -1],
  ['fractional total', value => value.totalRows = 1.1], ['excessive total', value => value.totalRows = 10000000001],
  ['locale claim', value => value.generalLocaleVerified = true], ['policy', value => value.orderingPolicy = 'literal'],
  ['short page', value => value.rows.pop()], ['sparse page', value => delete value.rows[3]],
  ['missing Env witness', value => value.rows[0].envRowId = '999'],
  ['missing Admin witness', value => value.rows[0].adminRowId = '999'],
  ['repeated pair', value => value.rows[1] = structuredClone(value.rows[0])],
  ['duplicate original', value => value.envOriginal.rows.push(structuredClone(value.envOriginal.rows[0]))],
  ['wrong schema wire', value => { value.envOriginal.columns[0].type = 'TEXT'; delete value.envOriginal.columns[0].declaredType; }],
  ['duplicate column', value => value.envOriginal.columns[1].name = value.envOriginal.columns[0].name],
  ['malformed schema Unicode', value => value.envOriginal.columns[0].name = '\ud800'],
  ['malformed original Unicode', value => value.envOriginal.rows[0].cells[0] = cell('text', '\ud800')],
  ['missing join key', value => value.adminOriginal.columns[0].name = 'Unrelated'],
  ['numeric join storage', value => value.envOriginal.rows[0].cells[0] = cell('integer', '1')],
  ['missing depth order', value => value.envOriginal.columns.find(column => column.name === 'RootingDepth').name = 'Other'],
  ['unsupported depth storage', value => {
    const index = value.envOriginal.columns.findIndex(column => column.name === 'RootingDepth');
    value.envOriginal.rows[0].cells[index] = cell('text', '20');
  }],
  ['unrelated original', value => { const row = structuredClone(value.adminOriginal.rows[0]); row.rowId = '999'; value.adminOriginal.rows.push(row); }],
]) test(`strict environment client rejects ${name}`, () => {
  const value = fixture().default; mutate(value);
  assert.throws(() => table.twoPageEnvironmentTableFromWire(value, owner, 0, 50));
});
test('strict environment client refuses invalid owner/page and end-page inventions', () => {
  for (const invalid of [{ ...owner, contextId: '' }, { ...owner, projectPath: '' }, { ...owner, suPath: '' }, { ...owner, plot: '\ud800' }]) {
    assert.throws(() => table.twoPageEnvironmentTableFromWire(fixture().default, invalid, 0, 50));
  }
  for (const [offset, limit] of [[-1, 50], [0.1, 50], [10000000001, 50], [0, 0], [0, 201], [0, 1.1]]) {
    assert.throws(() => table.twoPageEnvironmentTableFromWire(fixture().default, owner, offset, limit));
  }
  const end = fixture().empty;
  end.envOriginal.rows.push(fixture().default.envOriginal.rows[0]);
  assert.throws(() => validate(end));
});
test('source target metadata retains all446 direct instances, target-specific grouping and physical associations', () => {
  assert.deepEqual(targets.map(target => [target.form, target.fields.length, new Set(target.fields.map(field => field.column)).size, target.pages.length]), [
    ['FS882-6x4', 100, 96, 6], ['FS882-6x4XL', 103, 98, 6],
    ['FS882-8x6XL', 121, 118, 2], ['FS882-8x6XL-CHARS', 122, 120, 2],
  ]);
  for (const name of ['default', 'xl', 'normal', 'chars']) {
    const view = validate(fixture()[name]);
    const groups = table.twoPageEnvironmentGroups(view, view.rows[0]), fields = groups.flatMap(group => group.fields);
    assert.equal(fields.length, view.target.fields.length);
    assert.equal(new Set(fields.map(field => field.controlId)).size, fields.length);
    assert.ok(fields.every(field => field.label && field.cell && ['Env', 'Admin'].includes(field.role)));
    assert.match(view.target.sourceSHA256, /^[a-f0-9]{64}$/);
  }
  const view = validate(fixture().default), before = view.target.fields.length;
  view.envOriginal.columns[view.envOriginal.columns.findIndex(column => column.name === 'FieldNumber')].name = 'Missing';
  const fields = table.twoPageEnvironmentGroups(view, view.rows[0]).flatMap(group => group.fields);
  assert.equal(fields.length, before);
  assert.ok(fields.some(field => field.column === 'FieldNumber' && field.cell === null));
});
test('environment panel compiles and labels readonly target adaptation without invoking native datasheet behaviors', () => {
  const source = readFileSync(path.join(__dirname, 'TwoPageEnvironmentTablePanel.svelte'), 'utf8');
  assert.equal(compile(source, { generate: 'client' }).warnings.length, 0);
  assert.match(source, /saved Current.DataFormName selects the target/);
  assert.match(source, /not independently verified datasheet headers/);
  assert.match(source, /Each page reads a fresh snapshot and saved target/);
});
async function mountPanel(service) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageEnvironmentTablePanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  const ast = ts.createSourceFile('environment-table.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n').replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageEnvironmentTableService', 'twoPageEnvironmentTableFromWire',
    code + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack, reads.ReadRequests, service, table.twoPageEnvironmentTableFromWire);
  let actions; const activity = [];
  const dispose = runtime.effect_root(() => mount(null, { ...owner, form: 'FS882-8x6XL-CHARS', onBusyChange: value => activity.push(value), inspect: value => actions = value }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}
test('actual Svelte runtime reads only explicitly and preserves cloneable actual-Go50/20 pagination', async () => {
  const calls = [], wire = fixture();
  const panel = await mountPanel({ GetReview: (...args) => { calls.push(args); return Promise.resolve(args[2] === 0 ? wire.pagingFirst : wire.pagingNext); } });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().review.rows.length, 50); assert.equal(panel.actions().review.target.form, 'FS882-6x4');
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
    await panel.actions().load(50); panel.flush();
    assert.equal(panel.actions().review.rows.length, 20); assert.equal(panel.actions().review.offset, 50);
    assert.deepEqual(calls.map(args => args.slice(2)), [[0, 50], [50, 50]]);
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().busy, false);
  } finally { panel.dispose(); }
});
test('actual Svelte runtime retains explicit server/wire errors and permits correction/retry', async () => {
  let mode = 'server';
  const panel = await mountPanel({ GetReview: () => {
    if (mode === 'server') return Promise.reject(new Error('saved target unavailable'));
    const value = fixture().default;
    if (mode === 'wire') value.projectPath = 'foreign';
    return Promise.resolve(value);
  } });
  try {
    await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /saved target unavailable/);
    assert.equal(panel.actions().review, null);
    mode = 'wire'; await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /foreign ownership/);
    assert.equal(panel.actions().review, null);
    mode = 'valid'; await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().review.rows.length, 13);
  } finally { panel.dispose(); }
});
for (const retire of ['cancel', 'destroy']) test(`actual Svelte ${retire}/remount rejects held response without replay`, async () => {
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
test('mapped environment opener refuses all scope/draft/peer/child errors and Return holds pending read', () => {
  const create = input => componentFunctions('TwoPageEntryEditor.svelte', ['openEnvironmentTable', 'returnFromEnvironmentTable'], {
    environmentTableReaderAvailable: true, vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
    sourceBlocked: false, childBlocked: false, environmentTableOpen: false, environmentTableBusy: false, error: null, onchange() {},
    getDraftState: () => ({ unsaved: false, busy: false, blocked: false, error: null, ...input.draft }),
    sessions: new Map([[0, { view: () => input.peer ?? {} }]]), ...input,
  });
  for (const input of [
    { environmentTableReaderAvailable: false }, { vegetationScope: { projectPath: '', su: 'None', suPath: '' } },
    { vegetationScope: { projectPath: 'owned', su: 'Report', suPath: '' } }, { sourceBlocked: true }, { childBlocked: true },
    { draft: { unsaved: true } }, { draft: { busy: true } }, { draft: { blocked: true } }, { draft: { error: 'invalid' } },
    { peer: { error: 'invalid' } }, { peer: { referenceError: 'invalid' } },
  ]) {
    const panel = create(input); panel.actions.openEnvironmentTable();
    assert.equal(panel.environmentTableOpen, false); assert.match(panel.error, /Resolve/);
  }
  const panel = create({ environmentTableBusy: true }); panel.actions.openEnvironmentTable(); assert.equal(panel.environmentTableOpen, true);
  panel.actions.returnFromEnvironmentTable(); assert.equal(panel.environmentTableOpen, true); assert.match(panel.error, /cancel/);
});
test('open environment table suspends real parent/variant/close and every peer source callback', async () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const panel = componentFunctions('TwoPageEntryEditor.svelte',
    ['getDraftState', 'stage', 'selectProject', 'acknowledge', 'operation', 'chooseForm', 'closeReview', 'openAudit', 'openGoogleEarth', 'openVegetationTable', 'openExternalTransfer'], {
      environmentTableOpen: true, environmentTableBusy: false, vegetationTableOpen: false, vegetationTableBusy: false,
      externalOpen: false, auditOpen: false, auditBusy: false, googleEarthOpen: false, googleEarthBusy: false,
      sourceRefreshing: false, childBlocked: false, sourceBlocked: false, error: null, close: clean,
      view: { original: {}, referenceError: null }, auditReaderAvailable: true, googleEarthReaderAvailable: true, googleEarthPath: 'owned',
      vegetationTableReaderAvailable: true, vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
      externalSession: {}, sourceScope: { ...owner }, owner, form: 'FS882-8x6XL',
      sessions: new Map([[0, { closeState: () => clean, view: () => ({}) }]]), onchange() {},
      onclosed: () => { throw new Error('Source close is prohibited'); },
    });
  assert.equal(panel.actions.getDraftState().unsaved, true); assert.equal(panel.actions.getDraftState().blocked, true);
  panel.actions.stage('Latitude', { kind: 'text', raw: '54' }); assert.match(panel.error, /no field was staged/);
  panel.actions.selectProject('1'); assert.match(panel.error, /no ProjectID was staged/);
  panel.actions.acknowledge('Latitude'); assert.match(panel.error, /no literal draft was acknowledged/);
  await panel.actions.operation('save'); assert.match(panel.error, /environment table review/);
  await panel.actions.chooseForm('FS882-8x6XL-CHARS'); assert.equal(panel.form, 'FS882-8x6XL');
  panel.actions.closeReview(); assert.match(panel.error, /Save or Undo/);
  panel.actions.openAudit(); assert.equal(panel.auditOpen, false);
  panel.actions.openGoogleEarth(); assert.equal(panel.googleEarthOpen, false);
  panel.actions.openVegetationTable(); assert.equal(panel.vegetationTableOpen, false);
  panel.actions.openExternalTransfer(); assert.equal(panel.externalOpen, false);
});
