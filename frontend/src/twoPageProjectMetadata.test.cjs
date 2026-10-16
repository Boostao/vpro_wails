const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const definition = require('../../resources/project-metadata-layout.json');
const presentation = loadTypeScript('projectMetadataPresentation.ts', { '../../resources/project-metadata-layout.json': definition });
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const vegetation = loadTypeScript('twoPageVegetationTable.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const table = loadTypeScript('twoPageProjectMetadata.ts', {
  '../../resources/project-metadata-layout.json': definition, './siviParentTransport': transport,
  './projectMetadataPresentation': presentation, './qualityEditor': quality, './twoPageVegetationTable': vegetation,
});
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageProjectMetadataWire.json'), 'utf8'));
const ownerOf = value => Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath', 'plot'].map(name => [name, value[name]]));
const owner = ownerOf(fixture().default);
const validate = value => table.twoPageProjectMetadataFromWire(value, ownerOf(value), value.offset, value.limit);
const indexOf = (value, column) => value.columns.findIndex(item => item.name === column);

test('actual-Go default/page/beyond-end/empty-ID/None/50+20 wires retain immutable complete originals', () => {
  const wire = fixture(), before = JSON.stringify(wire);
  for (const [name, rows, total] of [
    ['default', 3, 3], ['page', 1, 3], ['end', 0, 3], ['emptyId', 1, 1],
    ['none', 50, 70], ['pagingFirst', 50, 70], ['pagingNext', 20, 70],
  ]) {
    const view = validate(wire[name]);
    assert.equal(view.metadataOriginal.rows.length, rows); assert.equal(view.totalRows, total);
    assert.equal(view.envOriginal.rows.length, 1); assert.equal(view.adminOriginal.rows.length, 1);
    assert.doesNotThrow(() => structuredClone(view));
    view.envOriginal.rows[0].cells[0].text = 'caller mutation';
    if (rows) view.metadataOriginal.rows[0].cells[0].text = 'caller mutation';
  }
  assert.equal(JSON.stringify(wire), before);
  assert.equal(validate(wire.emptyId).projectId, '');
  assert.equal(validate(wire.end).offset, 100000);
  assert.equal(table.twoPageProjectMetadataCellLabel(cell()), '(NULL)');
  assert.equal(table.twoPageProjectMetadataCellLabel(cell('text', '')), '(empty text)');
});
for (const [name, mutate] of [
  ['context', value => value.contextId = 'foreign'], ['project', value => value.project = 'foreign'],
  ['project path', value => value.projectPath += '.foreign'], ['SU', value => value.su = 'None'],
  ['SU path', value => value.suPath += '.foreign'], ['plot', value => value.plot = '0108050'],
  ['target', value => value.targetForm = 'ProjectMetadata'], ['table', value => value.metadataTable += '.foreign'],
  ['ProjectID', value => value.projectId = 'alpha'], ['NULL ProjectID', value => value.projectId = null],
  ['offset', value => value.offset++], ['limit', value => value.limit = 201],
  ['negative total', value => value.totalRows = -1], ['fractional total', value => value.totalRows = 1.1],
  ['excessive total', value => value.totalRows = 100001], ['locale claim', value => value.generalLocaleVerified = true],
  ['policy', value => value.matchingPolicy = 'literal'], ['short page', value => value.metadataOriginal.rows.pop()],
  ['sparse page', value => delete value.metadataOriginal.rows[1]],
  ['duplicate row', value => value.metadataOriginal.rows[1] = structuredClone(value.metadataOriginal.rows[0])],
  ['reversed physical order', value => value.metadataOriginal.rows.reverse()],
  ['missing parent', value => value.envOriginal.rows = []],
  ['duplicate parent', value => value.adminOriginal.rows.push(structuredClone(value.adminOriginal.rows[0]))],
  ['numeric plot', value => value.envOriginal.rows[0].cells[indexOf(value.envOriginal, 'PlotNumber')] = cell('integer', '108050')],
  ['foreign physical plot', value => value.adminOriginal.rows[0].cells[indexOf(value.adminOriginal, 'Plot')] = cell('text', 'foreign')],
  ['NULL parent ID', value => value.envOriginal.rows[0].cells[indexOf(value.envOriginal, 'ProjectID')] = cell()],
  ['unrelated row ID', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', 'Other')],
  ['NULL metadata ID', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell()],
  ['unsupported criteria', value => {
    value.projectId = "O'Brien"; value.envOriginal.rows[0].cells[indexOf(value.envOriginal, 'ProjectID')] = cell('text', value.projectId);
  }],
  ['unsupported row criteria', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', 'Alpha!')],
  ['newline criteria', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', 'Alpha\n')],
  ['carriage-return criteria', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', 'Alpha\r')],
  ['Unicode recasing', value => value.metadataOriginal.rows[0].cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', '\u017f')],
  ['leading-zero identity conversion', value => {
    value.projectId = '007'; value.envOriginal.rows[0].cells[indexOf(value.envOriginal, 'ProjectID')] = cell('text', '007');
    for (const row of value.metadataOriginal.rows) row.cells[indexOf(value.metadataOriginal, 'ProjectID')] = cell('text', '7');
  }],
  ['wrong schema wire', value => { value.metadataOriginal.columns[0].type = 'TEXT'; delete value.metadataOriginal.columns[0].declaredType; }],
  ['duplicate schema', value => value.metadataOriginal.columns[1].name = value.metadataOriginal.columns[0].name],
  ['ambiguous binding', value => value.metadataOriginal.columns[1].name = 'projectid'],
  ['sparse schema', value => delete value.metadataOriginal.columns[2]],
  ['sparse cells', value => delete value.metadataOriginal.rows[0].cells[2]],
  ['malformed schema Unicode', value => value.metadataOriginal.columns[0].name = '\ud800'],
  ['malformed source Unicode', value => value.metadataOriginal.rows[0].cells[1] = cell('text', '\ud800')],
  ['missing key schema', value => value.metadataOriginal.columns[0].name = 'Unrelated'],
]) test(`strict metadata transport rejects ${name}`, () => {
  const value = fixture().default; mutate(value);
  assert.throws(() => table.twoPageProjectMetadataFromWire(value, owner, 0, 50));
});
test('invalid owner/page and invented beyond-end originals refuse explicitly', () => {
  for (const invalid of [{ ...owner, contextId: '' }, { ...owner, projectPath: '' }, { ...owner, suPath: '' }, { ...owner, plot: '\ud800' }]) {
    assert.throws(() => table.twoPageProjectMetadataFromWire(fixture().default, invalid, 0, 50));
  }
  for (const [offset, limit] of [[-1, 50], [0.1, 50], [100001, 50], [0, 0], [0, 201], [0, 1.1]]) {
    assert.throws(() => table.twoPageProjectMetadataFromWire(fixture().default, owner, offset, limit));
  }
  const end = fixture().end;
  end.metadataOriginal.rows.push(fixture().default.metadataOriginal.rows[0]);
  assert.throws(() => validate(end));
});
test('all74 source bindings retain readable existing groups, historical storage and missing-field diagnostics', () => {
  const view = validate(fixture().default), row = view.metadataOriginal.rows[0];
  const fields = table.twoPageProjectMetadataGroups(view, row).flatMap(group => group.fields);
  assert.equal(fields.length, 74); assert.equal(new Set(fields.map(field => field.controlId)).size, 74);
  assert.ok(fields.every(field => field.label && field.cell));
  assert.deepEqual(Array.from(fields.slice(0, 4), field => field.name), ['ProjectID', 'DateLastEdited', 'AllSpecs', 'TableOfLists']);
  assert.ok(fields.some(field => field.name === 'CollectedVeg' && field.label.includes('Vegetation')));
  const id = indexOf(view.metadataOriginal, 'ID');
  assert.deepEqual(Array.from(view.metadataOriginal.rows, row => row.cells[id].storage), ['null', 'integer', 'text']);
  assert.deepEqual(Array.from(view.metadataOriginal.rows, row => row.rowId), ['-7', '2', '3']);
  assert.equal(row.cells[indexOf(view.metadataOriginal, 'Unmapped')].blobHex, 'ff00');
  view.metadataOriginal.columns[indexOf(view.metadataOriginal, 'Notes')].name = 'HistoricalExtra';
  const missing = table.twoPageProjectMetadataGroups(view, row).flatMap(group => group.fields);
  assert.equal(missing.length, 74); assert.equal(missing.find(field => field.name === 'Notes').cell, null);
  assert.throws(() => table.twoPageProjectMetadataGroups(view, structuredClone(row)));
});
async function mountPanel(service) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageProjectMetadataPanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  const ast = ts.createSourceFile('metadata-panel.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n').replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageProjectMetadataService', 'twoPageProjectMetadataFromWire',
    code + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack, reads.ReadRequests, service, table.twoPageProjectMetadataFromWire);
  let actions; const activity = [];
  const dispose = runtime.effect_root(() => mount(null, { ...owner, form: 'FS882-8x6XL-CHARS', onBusyChange: value => activity.push(value), inspect: value => actions = value }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}
test('actual Svelte runtime loads explicitly and keeps Go50/20 results cloneable without reactive proxies', async () => {
  const calls = [], wire = fixture();
  const panel = await mountPanel({ GetReview: (...args) => { calls.push(args); return Promise.resolve(args[2] === 0 ? wire.pagingFirst : wire.pagingNext); } });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().review.metadataOriginal.rows.length, 50);
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
    await panel.actions().load(50); panel.flush();
    assert.equal(panel.actions().review.metadataOriginal.rows.length, 20);
    assert.deepEqual(calls.map(args => args.slice(2)), [[0, 50], [50, 50]]);
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().busy, false);
  } finally { panel.dispose(); }
});
test('actual Svelte runtime retains server/wire errors and permits correction/retry', async () => {
  let mode = 'server';
  const panel = await mountPanel({ GetReview: () => {
    if (mode === 'server') return Promise.reject(new Error('parent ProjectID unavailable'));
    const value = fixture().default;
    if (mode === 'wire') value.projectPath = 'foreign';
    return Promise.resolve(value);
  } });
  try {
    await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /parent ProjectID unavailable/);
    assert.equal(panel.actions().review, null);
    mode = 'wire'; await panel.actions().load(); panel.flush(); assert.match(panel.actions().error, /foreign ownership/);
    mode = 'valid'; await panel.actions().load(); panel.flush();
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().review.metadataOriginal.rows.length, 3);
  } finally { panel.dispose(); }
});
for (const retire of ['cancel', 'destroy']) test(`actual Svelte ${retire}/remount rejects held metadata response without replay`, async () => {
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
test('mapped metadata opener refuses every scope/draft/peer/child error and pending Return', () => {
  const create = input => componentFunctions('TwoPageEntryEditor.svelte', ['openProjectMetadata', 'returnFromProjectMetadata'], {
    projectMetadataReaderAvailable: true, vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
    sourceBlocked: false, childBlocked: false, projectMetadataOpen: false, projectMetadataBusy: false, error: null, onchange() {},
    getDraftState: () => ({ unsaved: false, busy: false, blocked: false, error: null, ...input.draft }),
    sessions: new Map([[0, { view: () => input.peer ?? {} }]]), ...input,
  });
  for (const input of [
    { projectMetadataReaderAvailable: false }, { vegetationScope: { projectPath: '', su: 'None', suPath: '' } },
    { vegetationScope: { projectPath: 'owned', su: 'Report', suPath: '' } }, { sourceBlocked: true }, { childBlocked: true },
    { draft: { unsaved: true } }, { draft: { busy: true } }, { draft: { blocked: true } }, { draft: { error: 'invalid' } },
    { peer: { error: 'invalid' } }, { peer: { referenceError: 'invalid' } },
  ]) {
    const panel = create(input); panel.actions.openProjectMetadata();
    assert.equal(panel.projectMetadataOpen, false); assert.match(panel.error, /Resolve/);
  }
  const panel = create({ projectMetadataBusy: true }); panel.actions.openProjectMetadata();
  assert.equal(panel.projectMetadataOpen, 'read');
  panel.actions.returnFromProjectMetadata(); assert.equal(panel.projectMetadataOpen, 'read'); assert.match(panel.error, /cancel/);
  panel.projectMetadataBusy = false; panel.actions.returnFromProjectMetadata();
  assert.equal(panel.projectMetadataOpen, null); assert.equal(panel.error, null);
});
test('open metadata suspends actual parent/variant/close and every peer source callback', async () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const panel = componentFunctions('TwoPageEntryEditor.svelte',
    ['getCloseState', 'getDraftState', 'stage', 'selectProject', 'acknowledge', 'operation', 'chooseForm', 'closeReview',
      'openAudit', 'openGoogleEarth', 'openVegetationTable', 'openEnvironmentTable', 'openExternalTransfer'], {
      projectMetadataOpen: true, projectMetadataBusy: true, externalOpen: false, auditOpen: false, auditBusy: false,
      googleEarthOpen: false, googleEarthBusy: false, sourceRefreshing: false, childBlocked: false, sourceBlocked: false,
      error: null, close: clean, view: { original: {}, referenceError: null }, auditReaderAvailable: true,
      googleEarthReaderAvailable: true, googleEarthPath: 'owned', vegetationTableReaderAvailable: true,
      environmentTableReaderAvailable: true, vegetationScope: { projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath },
      externalSession: {}, sourceScope: { ...owner }, owner, form: 'FS882-8x6XL',
      sessions: new Map([[0, { closeState: () => clean, view: () => ({}) }]]), onchange() {},
      onclosed: () => { throw new Error('Source close is prohibited'); },
    });
  assert.equal(panel.actions.getCloseState().busy, true); assert.equal(panel.actions.getCloseState().canSave, false);
  assert.equal(panel.actions.getDraftState().unsaved, true); assert.equal(panel.actions.getDraftState().blocked, true);
  panel.actions.stage('Latitude', { kind: 'text', raw: '54' }); assert.match(panel.error, /no field was staged/);
  panel.actions.selectProject('1'); assert.match(panel.error, /no ProjectID was staged/);
  panel.actions.acknowledge('Latitude'); assert.match(panel.error, /no literal draft was acknowledged/);
  for (const action of ['load', 'save', 'undo', 'retain', 'prune', 'references']) {
    await panel.actions.operation(action); assert.match(panel.error, /project metadata review/);
  }
  await panel.actions.chooseForm('FS882-8x6XL-CHARS'); assert.equal(panel.form, 'FS882-8x6XL');
  panel.actions.closeReview(); assert.match(panel.error, /Save or Undo/);
  for (const action of ['openAudit', 'openGoogleEarth', 'openVegetationTable', 'openEnvironmentTable', 'openExternalTransfer']) {
    panel.actions[action](); assert.match(panel.error, /Resolve/);
  }
});
