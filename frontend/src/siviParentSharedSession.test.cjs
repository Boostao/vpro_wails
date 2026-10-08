const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { editor, writeSession, transport, source, original, cell, setCell, metadata, restoration, assignmentField } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const shared = loadTypeScript('siviParentSharedSession.ts', {
  '../../resources/fs1333-sivi-layout.json': source, './ordinaryEditor': ordinary,
  './projectMetadataEditor': metadata, './siviParentEditor': editor,
  './siviParentWriteSession': writeSession, './siviParentTransport': transport,
});
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
function deferred() {
  let resolve, reject;
  const promise = new Promise((a, b) => { resolve = a; reject = b; });
  return { promise, resolve, reject };
}
function fixture(overrides = {}) {
  let current = original();
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, cancel: 0 };
  let last;
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    save: async request => {
      calls.save++; last = structuredClone(request);
      for (const edit of request.edits) setCell(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '9007199254740993' };
    },
    restore: async (_, action) => {
      calls.restore++; current = original();
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => calls.refresh++,
    ...overrides,
  };
  return { session: new shared.SIVIParentSharedSession(owner, port, () => {}), calls, port, last: () => last };
}
test('eight exact source bindings reuse ordinary policies without callbacks or whole-header writes', async () => {
  assert.equal(shared.siviParentSharedFields.length, 8);
  for (const field of shared.siviParentSharedFields) {
    const exported = source.forms[0].fields.find(node => node.binding === field.column);
    assert.equal(field.controlId, exported.controlId);
    assert.equal(field.label, exported.caption || exported.controlName);
    assert.equal(field.policy, ordinary.ordinaryField(field.column));
  }
  const { session, calls, last } = fixture();
  await session.load();
  for (const column of shared.siviParentSharedColumns) {
    session.stage(column, { kind: 'text', raw: ['AirPhotoNum', 'VegNotes'].includes(column) ? '  Literal  ' : '3.25' });
  }
  assert.equal(await session.save(), true);
  assert.equal(last().edits.length, 8);
  assert.equal(Object.keys(last()).join(','), 'original,edits');
  for (const edit of last().edits) {
    assert.equal(edit.contextId, owner.contextId);
    assert.equal(edit.table, 'Project_Env');
    assert.equal(edit.rowId, '9007199254740993');
    assert.equal(edit.expected.storage, 'null');
  }
  assert.equal(last().edits.find(edit => edit.column === 'AirPhotoNum').value.text, '  Literal  ');
  for (const column of ['PlotType', 'SpeciesListComplete', 'ProjectID', 'VegSurveyor', 'SV_StandHeight']) {
    assert.throws(() => session.stage(column, { kind: 'clear' }), /unavailable/);
  }
  assert.equal(calls.save, 1);
});
test('raw errors survive snapshots/remount and block Save/Lock/close until correction or Undo', async () => {
  const { session, calls } = fixture();
  await session.load();
  session.stage('AirPhotoNum', { kind: 'text', raw: '😀'.repeat(11) });
  const remounted = session.view();
  assert.equal(remounted.drafts.AirPhotoNum.input.raw.length, 22);
  remounted.drafts.AirPhotoNum.input.raw = 'changed elsewhere';
  assert.equal(session.view().drafts.AirPhotoNum.input.raw.length, 22);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.closeState().canSave, false);
  assert.equal(session.closeState().unsaved, true);
  await assert.rejects(session.save(), /UTF-16/);
  await assert.rejects(session.load(), /Save or Undo/);
  assert.equal(calls.save, 0);
  session.stage('AirPhotoNum', { kind: 'text', raw: '\ud800' });
  assert.match(session.view().error, /Unicode/);
  session.stage('AirPhotoNum', { kind: 'text', raw: 'literal' });
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().blocked, false);
  session.stage('XCoord', { kind: 'text', raw: '1e39' });
  assert.match(session.view().error, /Single/);
  assert.equal(await session.undo(), true);
  assert.equal(session.view().error, null);
  assert.equal(shared.siviParentSharedDirty(session.view().drafts), false);
});
test('unchanged historical storage stays raw and omitted; NULL and empty are distinguishable', async () => {
  const r = original();
  setCell(r, 'AirPhotoNum', cell('text', 'x'.repeat(45)));
  setCell(r, 'XCoord', cell('text', 'historic-invalid'));
  setCell(r, 'YCoord', cell('integer', '9007199254740993'));
  setCell(r, 'VegNotes', cell('text', ''));
  const { session, calls } = fixture({ read: async () => r });
  await session.load();
  for (const column of ['AirPhotoNum', 'XCoord', 'YCoord', 'VegNotes']) {
    session.stage(column, { kind: 'text', raw: metadata.metadataCellText(shared.siviParentSharedCell(r, column)) });
  }
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'YCoord').integer, '9007199254740993');
  session.stage('VegNotes', { kind: 'clear' });
  assert.equal(shared.siviParentSharedRequest(r, session.view().drafts).edits[0].value.storage, 'null');
  session.stage('VegNotes', { kind: 'original' });
  assert.equal(session.closeState().unsaved, false);
  setCell(r, 'AirPhotoNum', cell('blob', '00ff'));
  await session.load();
  session.stage('AirPhotoNum', { kind: 'clear' });
  assert.match(session.view().error, /BLOB/);
  session.stage('AirPhotoNum', { kind: 'original' });
  assert.equal(session.view().error, null);
});
test('foreign drafts/original ownership cannot manufacture a shared-field request', async () => {
  const { session } = fixture();
  await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
  const view = session.view();
  view.drafts.XCoord.rowId = '123';
  assert.throws(() => shared.siviParentSharedRequest(view.original, view.drafts), /another original/);
  view.drafts = { ProjectID: { column: 'ProjectID' } };
  assert.throws(() => shared.siviParentSharedRequest(view.original, view.drafts), /exclude callbacks/);
  const other = fixture({ read: async () => ({ ...original(), ContextID: 'stale' }) });
  assert.equal(await other.session.load(), false);
  assert.match(other.session.view().error, /owner|ownership|context/i);
});
test('rejected or malformed acknowledgements conservatively block replay and recover explicitly', async () => {
  for (const result of [null, {}, { ChangedCells: 2, HistoryID: '1' }, { ChangedCells: 1, HistoryID: '01' }, 'reject']) {
    let saves = 0;
    const { session } = fixture({ save: async () => { saves++; if (result === 'reject') throw new Error('connection lost'); return result; } });
    await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
    assert.equal(await session.save(), false);
    assert.equal(session.closeState().blocked, true);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    await assert.rejects(session.save(), /do not replay/);
    assert.equal(saves, 1);
    assert.equal(await session.undo(), true);
    assert.equal(session.closeState().blocked, false);
    assert.equal(saves, 1);
  }
});
test('failed refresh never replays committed writes; retain/prune restoration use isolated history', async () => {
  let fail = true;
  const { session, calls } = fixture({ refreshParent: async () => { if (fail) throw new Error('refresh failed'); } });
  await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
  assert.equal(await session.save(), false);
  assert.equal(session.view().historyId, '9007199254740993');
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.undo(), false);
  fail = false;
  assert.equal(await session.undo(), true);
  assert.equal(calls.save, 1);
  assert.equal(await session.restore('retain'), true);
  assert.equal(calls.restore, 1);
  for (const action of ['retain', 'prune']) {
    const f = fixture();
    await f.session.load(); f.session.stage('XCoord', { kind: 'text', raw: '2' }); await f.session.save();
    assert.equal(await f.session.restore(action), true);
    await assert.rejects(f.session.restore(action), /verified parent history/);
  }
  const unknown = fixture({ restore: async () => { throw new Error('lost acknowledgement'); } });
  await unknown.session.load(); unknown.session.stage('XCoord', { kind: 'text', raw: '2' }); await unknown.session.save();
  assert.equal(await unknown.session.restore('retain'), false);
  assert.equal(unknown.session.closeState().blocked, true);
  assert.equal(unknown.session.view().historyId, null);
});
test('read cancellation rejects late originals; active commits cannot be cancelled or disposed', async () => {
  const read = deferred();
  const f = fixture({ read: () => read.promise });
  const load = f.session.load();
  f.session.cancel(); read.resolve(original());
  assert.equal(await load, false);
  assert.equal(f.session.view().original, null);
  assert.equal(f.calls.cancel, 1);
  const save = deferred();
  const active = fixture({ save: () => save.promise });
  await active.session.load(); active.session.stage('XCoord', { kind: 'text', raw: '2' });
  const pending = active.session.save();
  assert.throws(() => active.session.cancel(), /cannot be cancelled/);
  assert.throws(() => active.session.dispose(), /cannot be cancelled/);
  save.resolve({ ChangedCells: 1, HistoryID: '' });
  assert.equal(await pending, true);
});
const component = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentSharedFields.svelte'), 'utf8'),
  'SIVIParentSharedFields.svelte', { './siviParentSharedSession': shared, './projectMetadataEditor': metadata });
test('real Svelte controls carry original labels, one input per field, nullable actions and safety feedback', async () => {
  const { session } = fixture();
  await session.load(); session.stage('XCoord', { kind: 'text', raw: 'broken' });
  const html = render(component, { props: { view: session.view(), disabled: false, canSave: false,
    onstage() {}, onoperation() {}, oncancel() {} } }).body;
  for (const field of shared.siviParentSharedFields) {
    assert.equal(html.split(`id="sivi-shared-${field.column}"`).length - 1, 1);
    assert.equal(html.split(`for="sivi-shared-${field.column}"`).length - 1, 1);
    assert.ok(html.includes(field.label));
    assert.ok(html.includes(field.controlId));
  }
  assert.equal((html.match(/data-sivi-shared-field=/g) || []).length, 8);
  assert.equal((html.match(/Clear to NULL/g) || []).length, 8);
  assert.match(html, /aria-invalid="true"/);
  assert.match(html, /role="alert"/);
  assert.ok(html.indexOf('role="alert"') < html.indexOf('desktop safety adaptations'));
  assert.match(html, /sm:grid-cols-2/);
});
test('read panel optional shared slot suppresses only supplied live fields and blocks competing operations', () => {
  const direct = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'),
    'SIVIParentDirectField.svelte', { './siviParentEditor': editor, './projectMetadataEditor': metadata });
  const actions = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'),
    'SIVIParentActionField.svelte', { './siviParentEditor': editor, './projectMetadataEditor': metadata });
  const panel = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8'),
    'SIVIParentReadPanel.svelte', {
      '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
      './siviParentTransport': transport, './siviParentEditor': editor, './siviParentWriteSession': writeSession,
      './SIVIParentDirectField.svelte': { default: direct }, './SIVIParentActionField.svelte': { default: actions },
      './SIVIProjectAssignmentField.svelte': { default: assignmentField },
      './siviProjectAssignmentSession': require('./siviParentTestHelpers.cjs').assignmentSession,
    });
  const props = { view: { original: original(), busy: false, error: null }, onreload() {}, oncancel() {}, reloadDisabled: false };
  const baseline = render(panel, { props }).body;
  assert.equal((baseline.match(/data-sivi-parent-field=/g) || []).length, 77);
  const content = () => {};
  const mounted = render(panel, { props: { ...props,
    sharedEditor: { liveColumns: shared.siviParentSharedColumns, content, busy: false, unsaved: true } } }).body;
  assert.equal((mounted.match(/data-sivi-parent-field=/g) || []).length, 69);
  for (const column of shared.siviParentSharedColumns) assert.ok(!mounted.includes(`data-sivi-parent-field="${column}"`));
  assert.match(mounted, /<fieldset[^>]*disabled/);
});
