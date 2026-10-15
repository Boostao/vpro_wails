const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const table = loadTypeScript('twoPageVegetationTable.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageVegetationTableWire.json'), 'utf8'));
const ownerOf = value => Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath', 'plot', 'form'].map(key => [key, value[key]]));
const owner = ownerOf(fixture().normal);
const validate = value => table.twoPageVegetationTableFromWire(value, ownerOf(value), value.offset, value.limit);

test('strict actual-Go normal/CHARS/page/empty/None wires preserve complete typed witnesses and independent counts', () => {
  const wire = fixture(), before = JSON.stringify(wire);
  for (const [name, count, fields, total] of [
    ['normal', 10, 17, 10], ['chars', 10, 14, 10], ['page', 2, 17, 10],
    ['empty', 0, 17, 10], ['none', 8, 17, 8], ['pagingFirst', 50, 17, 70], ['pagingNext', 20, 17, 70],
  ]) {
    const review = validate(wire[name]);
    assert.equal(review.rows.length, count); assert.equal(review.columns.length, fields); assert.equal(review.totalRows, total);
    assert.equal(review.selectedOriginal === null, name === 'none');
    if (count) {
      assert.doesNotThrow(() => structuredClone(review));
      review.rows[0].cells[0].text = 'caller';
      review.original.rows[0].cells[0].text = 'caller';
    }
  }
  assert.equal(JSON.stringify(wire), before);
  assert.deepEqual(Array.from(validate(wire.normal).rows, row => row.vegRowId), ['-2', '2', '2', '4', '4', '1', '1', '3', '3', '5']);
  assert.deepEqual(Array.from(validate(wire.chars).rows, row => row.vegRowId), ['-2', '2', '2', '1', '1', '4', '4', '3', '3', '5']);
  const original = validate(wire.normal).original;
  const row = original.rows.find(row => row.rowId === '-2');
  assert.equal(row.cells[original.columns.findIndex(column => column.name === 'ID')].storage, 'null');
  assert.equal(row.cells[original.columns.findIndex(column => column.name === 'Flag')].integer, '-1');
  assert.equal(row.cells[original.columns.findIndex(column => column.name === 'Unmapped')].blobHex, 'ff00');
  assert.equal(table.twoPageVegetationTableCellLabel(cell()), '(NULL)');
  assert.equal(table.twoPageVegetationTableCellLabel(cell('text', '')), '(empty text)');
});
for (const [name, mutate] of [
  ['context', value => value.contextId = 'other'], ['project', value => value.project = 'other'],
  ['project path', value => value.projectPath += '.other'], ['SU', value => value.su = 'None'],
  ['SU path', value => value.suPath += '.other'], ['opening plot', value => value.plot = '0108050'],
  ['source form', value => value.form += '-other'], ['Veg table', value => value.table += '-other'],
  ['SU table', value => value.suTable += '-other'], ['offset', value => value.offset++],
  ['limit', value => value.limit = 201], ['negative total', value => value.totalRows = -1],
  ['fractional total', value => value.totalRows = 1.1], ['above bounded total', value => value.totalRows = 10000000001],
  ['unqualified general locale', value => value.generalLocaleVerified = true],
  ['silent policy replacement', value => value.orderingPolicy = 'literal'],
  ['projected caption', value => value.columns[2].caption = 'Cover1'],
  ['projected source', value => value.columns[2].name = 'Cover8'],
  ['short projected columns', value => value.columns.pop()],
  ['truncated joined page', value => value.rows.pop()],
  ['sparse page', value => delete value.rows[0]],
  ['missing Veg witness', value => value.rows[0].vegRowId = '999'],
  ['missing selected witness', value => value.rows[0].suRowId = '999'],
  ['invented None witness', value => value.rows[0].suRowId = null],
  ['missing projected cells', value => value.rows[0].cells.pop()],
  ['altered projected storage', value => value.rows[0].cells[2] = cell('integer', '0')],
  ['repeated join pair', value => value.rows[1] = structuredClone(value.rows[0])],
  ['duplicate physical Veg rows', value => value.original.rows.push(structuredClone(value.original.rows[0]))],
  ['wrong original column wire', value => { value.original.columns[0].type = 'TEXT'; delete value.original.columns[0].declaredType; }],
  ['duplicate schema', value => value.original.columns[1].name = value.original.columns[0].name],
  ['malformed schema Unicode', value => value.original.columns[0].name = '\ud800'],
  ['malformed original text', value => value.original.rows[0].cells[0] = cell('text', '\ud800')],
  ['missing selected PlotNumber', value => value.selectedOriginal.columns[0].name = 'Other'],
  ['unrelated original', value => { const row = structuredClone(value.original.rows[0]); row.rowId = '999'; value.original.rows.push(row); }],
]) test(`strict client rejects ${name} without partial output`, () => {
  const value = fixture().normal; mutate(value);
  assert.throws(() => table.twoPageVegetationTableFromWire(value, owner, 0, 50));
});
test('strict client rejects impossible ownership/pages and None inventions while preserving nullable project plots', () => {
  for (const invalid of [
    { ...owner, contextId: '' }, { ...owner, projectPath: '' }, { ...owner, suPath: '' },
    { ...owner, plot: '\ud800' }, { ...owner, form: 'unknown' },
  ]) assert.throws(() => table.twoPageVegetationTableFromWire(fixture().normal, invalid, 0, 50));
  for (const [offset, limit] of [[-1, 50], [0.1, 50], [0, 0], [0, 201], [0, 1.1]]) {
    assert.throws(() => table.twoPageVegetationTableFromWire(fixture().normal, owner, offset, limit));
  }
  const value = fixture().none, opening = ownerOf(value);
  value.selectedOriginal = fixture().normal.selectedOriginal;
  assert.throws(() => table.twoPageVegetationTableFromWire(value, opening, 0, 50));
  const end = fixture().empty;
  end.original.rows.push(fixture().normal.original.rows[0]);
  assert.throws(() => validate(end));
});
test('panel compiles without presentation warnings and exposes source aliases with visible read-only adaptation labels', () => {
  const source = readFileSync(path.join(__dirname, 'TwoPageVegetationTablePanel.svelte'), 'utf8');
  assert.equal(compile(source, { generate: 'client' }).warnings.length, 0);
  assert.match(source, /scope="col"/); assert.match(source, /General ACE locale parity is unverified/);
  assert.match(source, /opening plot[\s\S]*?not a collection filter/);
  assert.match(source, /Each page is a new owned snapshot/);
});
async function mountPanel(service) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageVegetationTablePanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  const ast = ts.createSourceFile('vegetation-table.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n').replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageVegetationTableService', 'twoPageVegetationTableFromWire',
    code + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack, reads.ReadRequests, service, table.twoPageVegetationTableFromWire);
  let actions; const activity = [];
  const dispose = runtime.effect_root(() => mount(null, { ...owner, onBusyChange: value => activity.push(value), inspect: value => actions = value }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}
test('actual Svelte runtime reads only explicitly and paginates actual Go 50/20 wires without cloned proxy failures', async () => {
  const calls = [], wire = fixture();
  const panel = await mountPanel({ GetReview: (...args) => { calls.push(args); return Promise.resolve(args[3] === 0 ? wire.pagingFirst : wire.pagingNext); } });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().review.rows.length, 50);
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
    await panel.actions().load(50); panel.flush();
    assert.equal(panel.actions().review.offset, 50); assert.equal(panel.actions().review.rows.length, 20);
    assert.deepEqual(calls.map(args => args.slice(3)), [[0, 50], [50, 50]]);
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().busy, false);
  } finally { panel.dispose(); }
});
test('actual Svelte runtime surfaces server and invalid-wire errors then permits explicit correction/retry', async () => {
  let mode = 'server';
  const panel = await mountPanel({ GetReview: () => {
    if (mode === 'server') return Promise.reject(new Error('physical source unavailable'));
    const value = fixture().normal;
    if (mode === 'wire') value.projectPath = 'foreign';
    return Promise.resolve(value);
  } });
  try {
    await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /physical source unavailable/);
    assert.equal(panel.actions().review, null); assert.equal(panel.actions().busy, false);
    mode = 'wire'; await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /foreign ownership/);
    assert.equal(panel.actions().review, null);
    mode = 'valid'; await panel.actions().load(); panel.flush(); assert.equal(panel.actions().error, '');
    assert.equal(panel.actions().review.rows.length, 10);
  } finally { panel.dispose(); }
});
for (const retire of ['cancel', 'destroy']) test(`actual Svelte ${retire}/remount rejects held successful response without replay`, async () => {
  let release, cancellations = 0, calls = 0;
  const held = new Promise(resolve => release = resolve); held.cancel = () => cancellations++;
  const service = { GetReview: () => { calls++; return held; } };
  const panel = await mountPanel(service), pending = panel.actions().load(); panel.flush();
  const old = panel.actions();
  if (retire === 'cancel') old.cancel(); else panel.dispose();
  const fresh = await mountPanel(service);
  try {
    release(fixture().normal); await pending; panel.flush(); fresh.flush();
    assert.equal(old.current(), null); assert.equal(fresh.actions().review, null);
    assert.equal(calls, 1); assert.equal(cancellations, 1);
  } finally { fresh.dispose(); if (retire === 'cancel') panel.dispose(); }
});
test('mapped source opener refuses independent off/ownership/draft/peer/child errors and Return holds pending reads', () => {
  const create = input => componentFunctions('TwoPageEntryEditor.svelte', ['openVegetationTable', 'returnFromVegetationTable'], {
    vegetationTableReaderAvailable: true, vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
    sourceBlocked: false, childBlocked: false, vegetationTableOpen: false, vegetationTableBusy: false, error: null, onchange() {},
    getDraftState: () => ({ unsaved: false, busy: false, blocked: false, error: null, ...input.draft }),
    sessions: new Map([[0, { view: () => input.peer ?? {} }]]), ...input,
  });
  for (const input of [
    { vegetationTableReaderAvailable: false }, { vegetationScope: { projectPath: '', su: 'None', suPath: '' } },
    { vegetationScope: { projectPath: 'owned', su: 'Report', suPath: '' } }, { sourceBlocked: true }, { childBlocked: true },
    { draft: { unsaved: true } }, { draft: { busy: true } }, { draft: { blocked: true } }, { draft: { error: 'invalid' } },
    { peer: { error: 'invalid' } }, { peer: { referenceError: 'invalid' } },
  ]) {
    const panel = create(input); panel.actions.openVegetationTable();
    assert.equal(panel.vegetationTableOpen, false); assert.match(panel.error, /Resolve/);
  }
  const panel = create({ vegetationTableBusy: true }); panel.actions.openVegetationTable(); assert.equal(panel.vegetationTableOpen, true);
  panel.actions.returnFromVegetationTable(); assert.equal(panel.vegetationTableOpen, true); assert.match(panel.error, /cancel/);
});
test('open vegetation table suspends real parent/variant/close and all peer source callbacks', async () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const panel = componentFunctions('TwoPageEntryEditor.svelte',
    ['getDraftState', 'stage', 'selectProject', 'acknowledge', 'operation', 'chooseForm', 'closeReview', 'openAudit', 'openGoogleEarth', 'openExternalTransfer'], {
      vegetationTableOpen: true, vegetationTableBusy: false, externalOpen: false, auditOpen: false, auditBusy: false, googleEarthOpen: false,
      googleEarthBusy: false, sourceRefreshing: false, childBlocked: false, sourceBlocked: false, error: null, close: clean,
      view: { original: {}, referenceError: null }, auditReaderAvailable: true, googleEarthReaderAvailable: true, googleEarthPath: 'owned',
      externalSession: {}, sourceScope: { ...owner }, owner, form: owner.form,
      sessions: new Map([[0, { closeState: () => clean, view: () => ({}) }]]), onchange() {},
      onclosed: () => { throw new Error('Source close is prohibited'); },
    });
  assert.equal(panel.actions.getDraftState().unsaved, true); assert.equal(panel.actions.getDraftState().blocked, true);
  panel.actions.stage('Latitude', { kind: 'text', raw: '54' }); assert.match(panel.error, /no field was staged/);
  panel.actions.selectProject('1'); assert.match(panel.error, /no ProjectID was staged/);
  panel.actions.acknowledge('Latitude'); assert.match(panel.error, /no literal draft was acknowledged/);
  await panel.actions.operation('save'); assert.match(panel.error, /vegetation table review/);
  await panel.actions.chooseForm('FS882-8x6XL-CHARS'); assert.equal(panel.form, owner.form);
  panel.actions.closeReview(); assert.match(panel.error, /Save or Undo/);
  panel.actions.openAudit(); assert.equal(panel.auditOpen, false);
  panel.actions.openGoogleEarth(); assert.equal(panel.googleEarthOpen, false);
  panel.actions.openExternalTransfer(); assert.equal(panel.externalOpen, false);
});
