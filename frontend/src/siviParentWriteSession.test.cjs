const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { writeSession, editor, transport, original, cell, setCell, source, metadata } = require('./siviParentTestHelpers.cjs');
const { SIVIParentWriteSession, siviParentDirectColumns, siviParentDirectRequest } = writeSession;
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
function deferred() { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; }
function fixture(overrides = {}) {
  let current = original();
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, cancel: 0 };
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    save: async request => {
      calls.save++;
      assert.equal(request.original.ContextID, owner.contextId);
      const edits = [request.scalars, request.options, request.text, request.categorical].flat();
      for (const edit of edits) setCell(current, edit.column, structuredClone(edit.value));
      return { ChangedCells: edits.length, HistoryID: '9007199254740993' };
    },
    restore: async (_, action) => {
      calls.restore++;
      current = original();
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => calls.refresh++,
    ...overrides,
  };
  return { session: new SIVIParentWriteSession(owner, port, () => {}), port, calls };
}
test('direct parent request retains all14 raw targets and excludes callback actions', () => {
  const r = original(); let drafts = {};
  const input = column => column === 'SV_FloodPlain' ? { kind: 'boolean', value: true }
    : column.endsWith('EstMeas') ? { kind: 'option', option: 2 }
    : column === 'SnowCoverregime' ? { kind: 'empty' }
    : ['SV_PolygonNumber', 'SV_CanopyComposition', 'SV_RootZoneTexture', 'SV_AhorizonType'].includes(column)
      ? { kind: 'text', raw: 'new' } : { kind: 'text', raw: '3.25' };
  for (const column of siviParentDirectColumns) drafts = editor.stageSIVIParent(r, drafts, column, input(column));
  const request = siviParentDirectRequest(r, drafts);
  assert.deepEqual(Object.keys(request).sort(), ['categorical', 'options', 'original', 'scalars', 'text']);
  assert.deepEqual([request.scalars.length, request.options.length, request.text.length, request.categorical.length], [7, 2, 2, 3]);
  for (const edit of [request.scalars, request.options, request.text, request.categorical].flat()) {
    assert.deepEqual(Object.keys(edit).sort(), ['column', 'contextId', 'expected', 'rowId', 'table', 'value']);
    assert.equal(edit.contextId, owner.contextId);
    assert.equal(edit.rowId, edit.table.endsWith('_Admin') ? '-9223372036854775808' : '9007199254740993');
    assert.equal(edit.expected.storage, 'null');
  }
  drafts = editor.stageSIVIParent(r, drafts, 'PlotType', { kind: 'option', option: 1 });
  assert.throws(() => siviParentDirectRequest(r, drafts), /exclude callback/);
});
test('direct parent drafts survive remount and invalid raw errors block Save/close until correction or Undo', async () => {
  const { session, calls } = fixture();
  await session.load();
  session.stage('SV_PolygonNumber', { kind: 'text', raw: 'x'.repeat(26) });
  const snapshot = session.view();
  assert.equal(snapshot.drafts.SV_PolygonNumber.input.raw.length, 26);
  snapshot.drafts.SV_PolygonNumber.input.raw = 'foreign';
  assert.equal(session.view().drafts.SV_PolygonNumber.input.raw.length, 26);
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(), /UTF-16/);
  await assert.rejects(session.load(), /Save or Undo/);
  assert.equal(calls.save, 0);
  session.stage('SV_PolygonNumber', { kind: 'text', raw: '  Literal  ' });
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().error, null);
  assert.equal(await session.save(), true);
  assert.equal(session.view().original.Rows[0].Env.cells.find(c => c.text === '  Literal  ').text, '  Literal  ');
  assert.equal(session.closeState().unsaved, false);
  assert.equal(calls.save, 1);
  session.stage('SV_StandHeight', { kind: 'text', raw: 'broken' });
  assert.equal(await session.undo(), true);
  assert.equal(session.view().error, null);
  assert.equal(Object.keys(session.view().drafts).length, 0);
});
test('public requests preserve historical raw no-ops and categorical NULL versus explicit empty', async () => {
  const r = original(); setCell(r, 'SV_PolygonNumber', cell('text', 'x'.repeat(26)));
  const { session, calls } = fixture({ read: async () => r });
  await session.load();
  session.stage('SV_PolygonNumber', { kind: 'original' });
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  session.stage('SnowCoverregime', { kind: 'empty' });
  assert.equal(session.view().drafts.SnowCoverregime.value.storage, 'text');
  assert.equal(session.view().drafts.SnowCoverregime.value.text, '');
  session.stage('SnowCoverregime', { kind: 'clear' });
  assert.equal(session.view().drafts.SnowCoverregime.value.storage, 'null');
  assert.throws(() => session.stage('PlotType', { kind: 'option', option: 1 }), /callback actions/);
  assert.throws(() => session.stage('ProjectID', { kind: 'text', raw: 'unsafe' }), /callback actions/);
});
test('historical BLOB correction is explicitly unavailable but retaining it clears the writer blocker', async () => {
  const r = original(); setCell(r, 'SV_RootZoneTexture', cell('blob', '00ff'));
  const { session, calls } = fixture({ read: async () => r });
  await session.load();
  session.stage('SV_RootZoneTexture', { kind: 'text', raw: 'new' });
  assert.equal(session.closeState().blocked, true);
  assert.match(session.view().error, /lossless source audit/);
  await assert.rejects(session.save(), /BLOB replacement/);
  assert.equal(calls.save, 0);
  session.stage('SV_RootZoneTexture', { kind: 'original' });
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().error, null);
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
});
test('rejected saves retain proposals but resolved malformed acknowledgements retire proposals and block replay', async () => {
  for (const result of [null, {}, { ChangedCells: 99, HistoryID: '1' }, { ChangedCells: 1, HistoryID: '01' }]) {
    let rejected = true, saves = 0;
    const { session } = fixture({ save: async () => { saves++; if (rejected) throw new Error('stale original'); return result; } });
    await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' });
    assert.equal(await session.save(), false);
    assert.equal(session.view().drafts.SV_StandHeight.input.raw, '3');
    assert.equal(session.closeState().canSave, true);
    rejected = false;
    assert.equal(await session.save(), false);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    assert.equal(session.closeState().blocked, true);
    assert.equal(session.view().historyId, null);
    await assert.rejects(session.save(), /do not replay/);
    assert.equal(saves, 2);
    assert.equal(await session.undo(), true);
    assert.equal(session.closeState().blocked, false);
  }
});
test('known commits retire proposals before failed refresh and survive failed recovery without replay', async () => {
  let refreshFailure = true;
  const { session, calls } = fixture({ refreshParent: async () => { if (refreshFailure) throw new Error('refresh failed'); } });
  await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' });
  assert.equal(await session.save(), false);
  assert.equal(Object.keys(session.view().drafts).length, 0);
  assert.equal(session.view().historyId, '9007199254740993');
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.undo(), false);
  assert.equal(session.closeState().blocked, true);
  refreshFailure = false;
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
  assert.equal(calls.save, 1);
});
test('known committed cleanup errors retire drafts/history and require explicit successful observation', async () => {
  const { session } = fixture({ save: async () => { throw new Error('SIVI parent edit committed but cleanup failed; reopen'); } });
  await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' });
  assert.equal(await session.save(), false);
  assert.equal(Object.keys(session.view().drafts).length, 0);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.view().historyId, null);
  assert.equal(await session.undo(), true);
});
test('write commit and recovery prevent disposal/cancel/concurrent stage; read cancellation ignores late peers', async () => {
  const pending = deferred();
  const { session } = fixture({ save: () => pending.promise });
  await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' });
  const save = session.save();
  assert.equal(session.closeState().busy, true);
  assert.throws(() => session.dispose(), /cannot be cancelled/);
  assert.throws(() => session.cancel(), /cannot be cancelled/);
  assert.throws(() => session.stage('SV_StandHeight', { kind: 'clear' }), /Wait/);
  pending.resolve({ ChangedCells: 1, HistoryID: '' }); assert.equal(await save, true);
  const old = deferred(), current = deferred(); let reads = 0;
  const retry = fixture({ read: () => ++reads === 1 ? old.promise : current.promise });
  const cancelled = retry.session.load(); retry.session.cancel();
  assert.match(retry.session.view().error, /cancelled/);
  const fresh = retry.session.load(); current.resolve(original()); assert.equal(await fresh, true);
  old.resolve(null); assert.equal(await cancelled, false);
  assert.equal(retry.session.view().original.Plot, 'P');
  const recovery = deferred();
  const locked = fixture({ refreshParent: () => recovery.promise });
  await locked.session.load();
  const undo = locked.session.undo();
  assert.throws(() => locked.session.cancel(), /cannot be cancelled/);
  assert.throws(() => locked.session.dispose(), /cannot be cancelled/);
  recovery.resolve(); assert.equal(await undo, true);
});
test('restoration retires history before malformed replies or refresh failure and never replays', async () => {
  for (const result of [null, { cancelled: false, restoredRows: 2, prunedAuditRows: 2, cleanedVegRows: 0 }]) {
    const { session, calls } = fixture({ restore: async () => { calls.restore++; return result; } });
    await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' }); await session.save();
    assert.equal(await session.restore('prune'), false);
    assert.equal(session.view().historyId, null);
    assert.equal(session.closeState().blocked, true);
    await assert.rejects(session.restore('prune'), /verified parent history/);
    await session.undo();
    assert.equal(calls.restore, 1);
  }
  const { session } = fixture();
  await session.load(); session.stage('SV_StandHeight', { kind: 'text', raw: '3' }); await session.save();
  assert.equal(await session.restore('retain'), true);
  assert.equal(session.view().historyId, null);
});
const DirectField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'), 'SIVIParentDirectField.svelte', {
  './projectMetadataEditor': metadata,
});
const ActionField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'), 'SIVIParentActionField.svelte', {
  './projectMetadataEditor': metadata,
});
const Panel = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8'), 'SIVIParentReadPanel.svelte', {
  '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
  './siviParentTransport': transport, './siviParentWriteSession': writeSession, './siviParentEditor': editor,
  './SIVIParentDirectField.svelte': { default: DirectField },
  './SIVIParentActionField.svelte': { default: ActionField },
  './siviProjectAssignmentSession': require('./siviParentTestHelpers.cjs').assignmentSession,
  './SIVIProjectAssignmentField.svelte': { default: require('./siviParentTestHelpers.cjs').assignmentField },
});
test('mounted parent retains77 fields and one visible source-labelled live control per directly bound field only', async () => {
  const { session } = fixture(); await session.load();
  session.stage('SV_PolygonNumber', { kind: 'text', raw: 'x'.repeat(26) });
  const markup = render(Panel, { props: { view: { original: original(), busy: false, error: null },
    writeView: session.view(), writeDisabled: false, writeCanSave: false, reloadDisabled: true, onreload() {}, oncancel() {} } }).body;
  assert.equal((markup.match(/data-sivi-parent-field=/g) || []).length, 77);
  assert.equal((markup.match(/data-sivi-direct-field=/g) || []).length, 14);
  for (const column of siviParentDirectColumns) {
    assert.equal((markup.match(new RegExp(`id="sivi-direct-${column}"`, 'g')) || []).length, 1);
    assert.match(markup, new RegExp(`<label for="sivi-direct-${column}">`));
  }
  assert.doesNotMatch(markup, /id="sivi-direct-(?:PlotType|SpeciesListComplete|ProjectID)"/);
  assert.ok(markup.indexOf('exceeds25 UTF-16') < markup.indexOf('data-sivi-parent-field='));
  assert.match(markup, /data-sivi-direct-save[^>]*disabled/);
  assert.match(markup, /data-sivi-direct-load[^>]*disabled/);
});
test('parent draft owner joins global lifecycle and permits remount without implicit refresh/source writes', () => {
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /VITE_SIVI_PARENT_EDITING === 'true'/);
  assert.match(parent, /childUnsaved = \$derived\(nonParentChildUnsaved \|\| siviParentWriteUnsaved \|\| siviParentActionUnsaved \|\| siviProjectAssignmentUnsaved \|\| siviParentSharedUnsaved \|\| twoPageOpen\)/);
  assert.match(parent, /blocked: \(siviClose\?\.blocked \?\? false\) \|\| \(siviCoverClose\?\.blocked \?\? false\) \|\| \(siviCombinedClose\?\.blocked \?\? false\) \|\| \(siviParentWriteClose\?\.blocked \?\? false\)/);
  assert.match(parent, /if \(siviParentWriteUnsaved\) \{ await siviParentWriteOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviParentWriteUnsaved\) \{ void siviParentWriteOperation\('undo'\); return; \}/);
  assert.match(parent, /Save or explicitly Undo SIVI parent drafts\/recovery before changing the plot lock/);
  assert.match(parent, /dirty \|\| nonParentChildUnsaved \|\| !siviParentSession/);
  assert.match(parent, /if \(!siviParentWriteUnsaved && !siviParentActionUnsaved && !siviProjectAssignmentUnsaved && !siviParentSharedUnsaved &&\s+siviParentSourceSession/);
  assert.match(parent, /!siviParentSourceView\?\.error &&\s+!siviParentSourceView\?\.authorityUnknown\) await siviParentSourceSession\.load\(\)/);
  assert.match(parent, /siviParentWriteSession\.closeState\(\)\.unsaved/);
});
