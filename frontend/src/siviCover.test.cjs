const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { cell, review } = require('./siviSourceFixture.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': [],
  '../../resources/project-metadata-template.json': [],
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric,
});
const heightSession = loadTypeScript('siviHeightSession.ts', {
  './siviHeightEditor': height, './projectMetadataRestore': restoration,
});
const editor = loadTypeScript('siviCoverEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric, './siviHeightEditor': height,
});
const childSession = loadTypeScript('siviChildWriteSession.ts', {
  './projectMetadataRestore': restoration,
  './siviHeightEditor': height, './siviHeightSession': heightSession,
});
const session = loadTypeScript('siviCoverSession.ts', {
  './siviCoverEditor': editor, './siviChildWriteSession': childSession,
});
const id = '9007199254740993';
const copy = value => JSON.parse(JSON.stringify(value));
const at = (source, index, column) => source[index].Rows[0].cells[source[index].Columns.indexOf(column)];
function set(source, index, column, value) {
  source[index].Rows[0].cells[source[index].Columns.indexOf(column)] = value;
}
function stage(raw, original = cell('null'), column = 'Cover2', nullValue = false) {
  const source = review({ extended: true });
  set(source, 0, column, original);
  const drafts = editor.stageSIVICover(source, {}, id, column, raw, nullValue);
  return { source, drafts, draft: drafts[id][column] };
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
function fixture(overrides = {}, extended = false) {
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, notify: 0, request: null };
  const port = {
    read: async extended => { calls.read++; return review({ extended }); },
    save: async (extended, requestJSON) => {
      calls.save++;
      const request = JSON.parse(requestJSON);
      calls.request = request;
      assert.equal(request.original[0].Form, extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC');
      assert.equal(request.edits[0].rowId, id);
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
    restore: async (historyId, action) => {
      calls.restore++;
      assert.equal(historyId, '1');
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  return { owner: new session.SIVICoverSession('P', port, () => calls.notify++, extended), calls, port };
}
async function saved(overrides = {}) {
  const fixtureValue = fixture(overrides);
  await fixtureValue.owner.load();
  fixtureValue.owner.stage(id, 'Cover2', '3', false);
  assert.equal(await fixtureValue.owner.save(), true);
  return fixtureValue;
}

test('every source cover/total obeys strict100, finite SINGLE, negative/no-lower-bound and NULL rules', () => {
  for (const column of editor.siviCoverColumns) {
    const source = review({ extended: true });
    const valid = [['', null], ['0', 0], ['-1', -1], ['99.99999999999999', 99.99999999999999],
      ['1e-100', 1e-100], ['-3.4028234663852886e38', -numeric.singleMaximum], ['+.125', .125]];
    for (const [raw, value] of valid) {
      const drafts = editor.stageSIVICover(source, {}, id, column, raw, false);
      const draft = drafts[id][column];
      assert.equal(draft.error, null, `${column}: ${raw}`);
      assert.equal(draft.raw, raw);
      assert.equal(draft.value.storage, value === null ? 'null' : 'real');
      assert.equal(draft.value.real, value);
      assert.doesNotThrow(() => editor.siviCoverEdits(source, drafts));
    }
    for (const raw of ['100', '100.00001', '1e2', '101', '3.4028234663852886e38',
      '-3.402823466385289e38', '1e309', '-Infinity', 'Infinity', 'NaN', '0x1', '1_0', '1,5',
      ' 5', '5 ', '\t5', '\n', ' ', '1\n2', 'five', '\u0000', '\ud800', '\udc00']) {
      const drafts = editor.stageSIVICover(source, {}, id, column, raw, false);
      assert.ok(drafts[id][column].error, `${column}: ${JSON.stringify(raw)}`);
      assert.equal(drafts[id][column].raw, raw);
      assert.throws(() => editor.siviCoverEdits(source, drafts));
    }
    const cleared = editor.stageSIVICover(source, {}, id, column, '125', true);
    assert.equal(cleared[id][column].value.storage, 'null');
    assert.equal(cleared[id][column].error, null);
  }
});
test('unchanged historical invalid typed cells are omitted exactly, never normalized or rounded', () => {
  for (const original of [cell('real', 125), cell('real', -1e100), cell('integer', '125'),
    cell('text', '125'), cell('text', ' 125 '), cell('text', 'NaN'), cell('text', ''),
    cell('blob', '00ab')]) {
    const raw = metadata.metadataCellText(original);
    const { source, drafts, draft } = stage(raw, original);
    assert.equal(draft.error, null);
    assert.deepEqual(copy(draft.expected), original);
    assert.deepEqual(copy(draft.value), original);
    assert.equal(editor.siviCoverDirty(drafts), false);
    assert.equal(editor.siviCoverEdits(source, drafts).length, 0);
  }
  const historical = stage('125', cell('integer', '125'));
  assert.equal(editor.siviCoverEdits(historical.source, historical.drafts).length, 0);
  const changed = stage('125.0', cell('integer', '125'));
  assert.match(changed.draft.error, /strictly less/);
  const converted = stage('12.0', cell('integer', '12'));
  assert.equal(editor.siviCoverEdits(converted.source, converted.drafts)[0].value.storage, 'real');
  assert.ok(stage('1', cell('blob', 'ff')).draft.error);
  assert.equal(stage('12.3456789012345').draft.value.real, 12.3456789012345);
});
test('source controls are scoped to their actual forms, with physical row+column keys across groups', () => {
  const source = review({ extended: true });
  let drafts = {};
  for (const column of editor.siviCoverColumns) drafts = editor.stageSIVICover(source, drafts, id, column, '-2', false);
  const edits = editor.siviCoverEdits(source, drafts);
  assert.equal(edits.length, 14);
  assert.equal(new Set(edits.map(edit => `${edit.rowId}:${edit.column}`)).size, 14);
  for (const edit of edits) {
    assert.equal(edit.form, edit.column === 'Cover6' ? 'SubVegC-SIVI' :
      ['Cover7', 'Cover8', 'Cover9'].includes(edit.column) ? 'SubVegD-SIVI' : 'SubVegA-SIVI');
    assert.equal(edit.rowId, id);
  }
  for (const forbidden of ['HeightA', 'HeightB', 'Height6', 'Species', 'Collected', 'ID', 'PlotNumber', 'cover1', 'Cover10']) {
    assert.throws(() => editor.stageSIVICover(source, {}, id, forbidden, '1', false), /Only source-scoped/);
    assert.throws(() => editor.siviCoverEdits(source, { [id]: { [forbidden]: drafts[id].Cover1 } }), /Only source-scoped/);
  }
  for (const column of ['Cover5a', 'Cover5b', 'Cover5c']) {
    assert.throws(() => editor.stageSIVICover(review(), {}, id, column, '1', false), /hidden/);
  }
  for (const badId of ['01', '-0', '9007199254740992', '9223372036854775808']) {
    assert.throws(() => editor.stageSIVICover(source, {}, badId, 'Cover1', '1', false));
  }
  const duplicate = structuredClone(source);
  duplicate[0].Rows.push(structuredClone(duplicate[0].Rows[0]));
  assert.throws(() => editor.stageSIVICover(duplicate, {}, id, 'Cover1', '1', false), /unavailable/);
  const repeatedColumn = structuredClone(source);
  repeatedColumn[0].Columns.push('Cover1');
  assert.throws(() => editor.stageSIVICover(repeatedColumn, {}, id, 'Cover1', '1', false));
  const wrongForm = structuredClone(source);
  wrongForm[0].Form = 'FS882';
  assert.throws(() => editor.stageSIVICover(wrongForm, {}, id, 'Cover1', '1', false));
});
test('raw requests have exact wire aliases, complete cloned original typed cells and no raw draft Unicode repair', () => {
  const { source, drafts } = stage('-3', cell('integer', '125'));
  const json = editor.siviCoverRequestJSON(source, drafts), request = JSON.parse(json);
  assert.deepEqual(Object.keys(request), ['original', 'edits']);
  assert.deepEqual(Object.keys(request.original[0]), ['Form', 'Query', 'Columns', 'Rows']);
  assert.deepEqual(Object.keys(request.original[0].Rows[0]), ['rowId', 'cells']);
  assert.deepEqual(Object.keys(request.edits[0]), ['rowId', 'form', 'column', 'expected', 'value']);
  assert.deepEqual(request.edits[0].expected, cell('integer', '125'));
  assert.deepEqual(request.edits[0].value, cell('real', -3));
  request.original[0].Rows[0].cells[2].text = 'mutated';
  request.edits[0].expected.integer = '9';
  assert.equal(at(source, 0, 'Species').text, 'RAW');
  assert.equal(drafts[id].Cover2.expected.integer, '125');
  for (const raw of ['\ud800', '1\udc00']) {
    const invalid = stage(raw);
    assert.throws(() => editor.siviCoverRequestJSON(invalid.source, invalid.drafts), /Unicode/);
  }
  const damaged = structuredClone(source);
  set(damaged, 0, 'Species', cell('text', '\ud800'));
  assert.throws(() => editor.siviCoverRequestJSON(damaged, drafts), /incomplete raw/);
  const forged = structuredClone(drafts);
  forged[id].Cover2.value.real = 4;
  assert.throws(() => editor.siviCoverRequestJSON(source, forged), /differs/);
});
test('original safety rejects missing cells, bad Unicode/identity, height-only membership; zero/negative covers stay visible', async () => {
  for (const bad of [null, [], [{ ...review()[0], Columns: null }], [{ ...review()[0], Rows: null }]]) {
    const { owner } = fixture({ read: async () => bad });
    assert.equal(await owner.load(), false);
    assert.equal(owner.view().review, null);
    assert.equal(owner.closeState().canSave, false);
  }
  for (const mutate of [
    source => { set(source, 0, 'Cover1', cell('null')); },
    source => { set(source, 0, 'Species', cell('text', '\udc00')); },
    source => { source[0].Rows[0].cells.pop(); },
    source => { set(source, 0, 'PlotNumber', cell('text', 'other')); },
    source => { set(source, 1, 'Species', cell('text', 'disagree')); },
    source => { source[0].Rows[0].rowId = '01'; },
  ]) {
    const source = review(); mutate(source);
    const { owner } = fixture({ read: async () => source });
    assert.equal(await owner.load(), false);
  }
  for (const value of [0, -5]) {
    const source = review(); set(source, 0, 'Cover1', cell('real', value));
    const { owner } = fixture({ read: async () => source });
    assert.equal(await owner.load(), true);
    assert.equal(owner.view().review[0].Rows.length, 1);
    source[0].Rows[0].cells[2].text = 'external mutation';
    assert.equal(owner.view().review[0].Rows[0].cells[2].text, 'RAW');
  }
});
test('invalid literal drafts/errors persist through snapshots and extended remounts, blocking Save/close/Lock until correction/Undo', async () => {
  for (const raw of ['100', ' 2 ', '\ud800', 'not a number']) {
    const { owner, calls } = fixture();
    await owner.load();
    owner.stage(id, 'Cover2', raw, false);
    owner.presentation(true); owner.presentation(false);
    const remount = owner.view();
    assert.equal(remount.drafts[id].Cover2.raw, raw);
    assert.equal(owner.closeState().blocked, true);
    assert.equal(owner.closeState().unsaved, true);
    assert.equal(owner.closeState().canSave, false);
    await assert.rejects(owner.save(), /invalid SIVI/);
    await assert.rejects(owner.load(), /Finish or Undo/);
    remount.drafts[id].Cover2.raw = 'foreign';
    assert.equal(owner.view().drafts[id].Cover2.raw, raw);
    owner.stage(id, 'Cover2', '-5', false);
    assert.equal(owner.view().error, null);
    assert.equal(owner.closeState().blocked, false);
    assert.equal(await owner.save(), true);
    assert.equal(calls.save, 1);
    assert.equal(Object.keys(owner.view().drafts).length, 0);
    owner.stage(id, 'Cover2', raw, false);
    assert.equal(await owner.undo(), true);
    assert.equal(owner.view().error, null);
    assert.equal(owner.closeState().unsaved, false);
  }
});
test('hidden extended drafts always block authority, including unchanged NULL drafts; showing retains history/error/value', async () => {
  for (const raw of ['4', '100', '']) {
    const { owner, calls } = fixture({}, true);
    await owner.load();
    owner.stage(id, 'Cover2', '3', false);
    await owner.save();
    owner.stage(id, 'Cover5a', raw, false);
    const before = owner.view().drafts[id].Cover5a;
    owner.presentation(false);
    assert.deepEqual(copy(owner.view().drafts[id].Cover5a), copy(before));
    assert.equal(owner.view().historyId, '1');
    assert.equal(owner.closeState().unsaved, true);
    assert.equal(owner.closeState().blocked, true);
    await assert.rejects(owner.save(), /Show extended/);
    await assert.rejects(owner.restore('retain'), /Finish or Undo/);
    assert.throws(() => owner.stage(id, 'Cover5a', '5', false), /hidden/);
    assert.equal(calls.save, 1);
    owner.presentation(true);
    assert.deepEqual(copy(owner.view().drafts[id].Cover5a), copy(before));
    owner.stage(id, 'Cover5a', '4', false);
    assert.equal(await owner.save(), true);
    assert.equal(calls.request.edits[0].form, 'SubVegA-SIVI');
    assert.equal(calls.save, 2);
  }
});
test('historical125/no-op never writes or refreshes, and changed Save refreshes the whole source with verified cover history', async () => {
  const source = review(); set(source, 0, 'TotalA', cell('real', 125));
  const { owner, calls } = fixture({ read: async extended => height.siviHeightPresentation(source, 'P', extended) });
  await owner.load();
  owner.stage(id, 'TotalA', '125', false);
  assert.equal(await owner.save(), true);
  assert.equal(calls.save, 0);
  assert.equal(calls.refresh, 0);
  owner.stage(id, 'TotalA', '99', false);
  owner.stage(id, 'Cover6', '-5', false);
  assert.equal(await owner.save(), true);
  assert.equal(calls.request.edits.length, 2);
  assert.equal(calls.request.original.length, 3);
  assert.equal(calls.refresh, 1);
  assert.equal(owner.view().historyId, '1');
  assert.equal(owner.closeState().unsaved, false);
});
test('rejected/cancelled/collision Save preserves drafts and existing history, permits correction and one retry', async () => {
  for (const reason of ['stale source collision', 'cancelled before write', 'audit ownership rejected']) {
    const { owner, port } = await saved();
    let saves = 0, reject = true;
    port.save = async (_, json) => {
      saves++;
      if (reject) throw new Error(reason);
      return { ChangedCells: JSON.parse(json).edits.length, HistoryID: '' };
    };
    owner.stage(id, 'Cover2', '4', false);
    assert.equal(await owner.save(), false);
    assert.equal(owner.view().historyId, '1');
    assert.equal(owner.view().drafts[id].Cover2.raw, '4');
    assert.equal(owner.view().blocked, false);
    owner.stage(id, 'Cover2', '5', false);
    reject = false;
    assert.equal(await owner.save(), true);
    assert.equal(saves, 2);
    assert.equal(owner.view().historyId, null);
  }
});
test('incomplete/alias/unknown save acknowledgements or committed-cleanup marker prevent replay until explicit recovery', async () => {
  for (const response of [null, {}, { changedCells: 1, historyId: '1' }, { ChangedCells: 0, HistoryID: '1' },
    { ChangedCells: '1', HistoryID: '1' }, { ChangedCells: 1.5, HistoryID: '1' },
    ...['01', '-1', '0', '9223372036854775808', 1, null, undefined].map(HistoryID => ({ ChangedCells: 1, HistoryID })),
    new Error('SIVI cover edit committed but cleanup failed; sealed failure')]) {
    let saves = 0;
    const { owner } = fixture({ save: async () => {
      saves++; if (response instanceof Error) throw response; return response;
    } });
    await owner.load(); owner.stage(id, 'Cover2', '3', false);
    assert.equal(await owner.save(), false);
    assert.equal(owner.view().blocked, true);
    assert.equal(owner.view().historyId, null);
    assert.equal(owner.view().review, null);
    assert.equal(owner.closeState().unsaved, true);
    await assert.rejects(owner.save(), /do not replay/);
    await assert.rejects(owner.load(), /Finish or Undo/);
    assert.throws(() => owner.stage(id, 'Cover2', '4', false), /Undo\/reload/);
    assert.equal(saves, 1);
    assert.equal(await owner.undo(), true);
    assert.equal(owner.closeState().unsaved, false);
    assert.equal(owner.closeState().canSave, true);
  }
  for (const HistoryID of ['', '9007199254740993', '9223372036854775807']) {
    const { owner } = fixture({ save: async () => ({ ChangedCells: 1, HistoryID }) });
    await owner.load(); owner.stage(id, 'Cover2', '3', false);
    assert.equal(await owner.save(), true);
    assert.equal(owner.view().historyId, HistoryID || null);
  }
});
test('committed Save parent/source refresh failures remove write authority, retain valid history, recover without replay', async () => {
  for (const failure of ['parent', 'source']) {
    let fail = false;
    const { owner, calls } = fixture({
      read: async extended => fail && failure === 'source' ? null : review({ extended }),
      refreshParent: async () => { if (fail && failure === 'parent') throw new Error('parent failed'); },
    });
    await owner.load(); owner.stage(id, 'Cover2', '3', false); fail = true;
    assert.equal(await owner.save(), false);
    assert.equal(owner.view().blocked, true);
    assert.equal(owner.view().historyId, '1');
    assert.equal(editor.siviCoverDirty(owner.view().drafts), false);
    assert.equal(owner.view().review, null);
    assert.equal(await owner.undo(), false);
    assert.equal(owner.closeState().blocked, true);
    fail = false;
    assert.equal(await owner.undo(), true);
    assert.equal(calls.save, 1);
    assert.equal(owner.view().historyId, '1');
    assert.equal(owner.closeState().unsaved, false);
  }
});
test('cover-only retain/prune restores consume history once, refresh whole source and reject unrelated actions/drafts', async () => {
  for (const action of ['retain', 'prune']) {
    const { owner, calls } = await saved();
    owner.presentation(true); owner.presentation(false);
    await assert.rejects(owner.restore('cancel'), /explicit retain or prune/);
    owner.stage(id, 'Cover2', '100', false);
    await assert.rejects(owner.restore(action), /Finish or Undo/);
    await owner.undo();
    assert.equal(owner.view().historyId, '1');
    assert.equal(await owner.restore(action), true);
    assert.equal(owner.view().historyId, null);
    assert.equal(calls.restore, 1);
    assert.equal(calls.refresh, 3);
    await assert.rejects(owner.restore(action), /verified cover history/);
  }
});
test('restore rejected/collision results retain history; incomplete/aliases/unknown/cleanup results block replay', async () => {
  for (const response of [null, {}, { Cancelled: false, RestoredRows: 1, PrunedAuditRows: 0, CleanedVegRows: 0 },
    { cancelled: true, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 },
    { cancelled: false, restoredRows: 0, prunedAuditRows: 0, cleanedVegRows: 0 },
    { cancelled: false, restoredRows: 1.5, prunedAuditRows: 0, cleanedVegRows: 0 },
    { cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 1 },
    { cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 },
    new Error('collision: current cover differs'),
    new Error('SIVI restoration committed but cleanup failed; closed'),
    new Error('SIVI cover restoration committed but cleanup failed; closed')]) {
    const { owner } = await saved({ restore: async () => {
      if (response instanceof Error) throw response; return response;
    } });
    assert.equal(await owner.restore('retain'), false);
    const rejected = response instanceof Error && response.message.startsWith('collision');
    assert.equal(owner.view().blocked, !rejected);
    assert.equal(owner.view().historyId, rejected ? '1' : null);
    if (!rejected) await assert.rejects(owner.restore('retain'), /Finish or Undo/);
    assert.equal(await owner.undo(), true);
  }
  const { owner, port } = await saved({ restore: async () => { throw new Error('cancelled before restore'); } });
  assert.equal(await owner.restore('retain'), false);
  assert.equal(owner.view().historyId, '1');
  port.restore = async () => ({ cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 });
  assert.equal(await owner.restore('prune'), true);
});
test('acknowledged restore refresh failure consumes history and blocks replay until successful explicit reload', async () => {
  for (const failure of ['parent', 'source']) {
    const { owner, port, calls } = await saved();
    const read = port.read;
    if (failure === 'parent') port.refreshParent = async () => { throw new Error('refresh failed'); };
    else port.read = async () => null;
    assert.equal(await owner.restore('prune'), false);
    assert.equal(owner.view().historyId, null);
    assert.equal(owner.view().blocked, true);
    await assert.rejects(owner.restore('prune'), /Finish or Undo/);
    port.read = read; port.refreshParent = async () => {};
    assert.equal(await owner.undo(), true);
    assert.equal(calls.restore, 1);
  }
});
test('load/save/restore/Undo share mutual-operation ownership and cloned snapshots', async () => {
  for (const operation of ['load', 'save', 'restore', 'undo']) {
    const { owner, port, calls } = operation === 'restore' || operation === 'undo' ? await saved() : fixture();
    const wait = deferred();
    if (operation === 'load' || operation === 'undo') port.read = () => wait.promise;
    if (operation === 'save') {
      await owner.load(); owner.stage(id, 'Cover2', '3', false);
      port.save = (_, json) => {
        calls.request = JSON.parse(json);
        calls.request.original[0].Rows[0].cells[2].text = 'foreign port mutation';
        return wait.promise;
      };
    }
    if (operation === 'restore') port.restore = () => wait.promise;
    const pending = operation === 'restore' ? owner.restore('retain') : owner[operation]();
    assert.equal(owner.view().busy, true);
    assert.equal(owner.closeState().canSave, false);
    assert.throws(() => owner.stage(id, 'Cover2', '4', false), /Wait/);
    assert.throws(() => owner.presentation(true), /Wait/);
    for (const other of ['load', 'save', 'undo']) await assert.rejects(owner[other](), /Wait/);
    await assert.rejects(owner.restore('retain'), /Wait/);
    if (operation === 'save') {
      assert.equal(owner.view().review[0].Rows[0].cells[2].text, 'RAW');
      wait.resolve({ ChangedCells: 1, HistoryID: '1' });
    } else if (operation === 'restore') {
      wait.resolve({ cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 });
    } else wait.resolve(review());
    assert.equal(await pending, true);
    assert.equal(owner.view().busy, false);
  }
});
test('dispose seals ownership before or during load/save/restore/refresh and stops callbacks/replay', async () => {
  const unopened = fixture(); unopened.owner.dispose();
  await assert.rejects(unopened.owner.load(), /ownership ended/);
  assert.throws(() => unopened.owner.presentation(true), /ownership ended/);
  for (const operation of ['load', 'save', 'restore', 'refresh']) {
    const { owner, port, calls } = operation === 'restore' ? await saved() : fixture();
    const wait = deferred();
    if (operation === 'load') port.read = () => wait.promise;
    if (operation === 'save' || operation === 'refresh') {
      await owner.load(); owner.stage(id, 'Cover2', '3', false);
      if (operation === 'save') port.save = () => wait.promise;
      else port.refreshParent = () => wait.promise;
    }
    if (operation === 'restore') port.restore = () => wait.promise;
    const pending = operation === 'load' ? owner.load() :
      operation === 'restore' ? owner.restore('retain') : owner.save();
    if (operation === 'refresh') await Promise.resolve();
    owner.dispose();
    const notifications = calls.notify, refreshes = calls.refresh;
    wait.resolve(operation === 'load' ? review() : operation === 'restore'
      ? { cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 }
      : operation === 'save' ? { ChangedCells: 1, HistoryID: '1' } : undefined);
    assert.equal(await pending, false);
    assert.equal(calls.notify, notifications);
    assert.equal(calls.refresh, refreshes);
    if (operation !== 'load') assert.equal(owner.view().blocked, true);
    await assert.rejects(owner.save(), /ownership ended/);
    await assert.rejects(owner.undo(), /ownership ended/);
  }
});
test('panel compiles without warnings and renders one visible associated cover control, grouped responsive labels and read-only context', async () => {
  const filename = 'SIVICoverPanel.svelte';
  const source = readFileSync(join(__dirname, filename), 'utf8');
  const component = serverComponent(source, filename, {
    '../bindings/github.com/boostao/vpro-wails': {
      AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
    },
    './siviCoverEditor': editor, './projectMetadataEditor': metadata,
  });
  const { owner } = fixture(); await owner.load();
  const html = () => render(component, { props: { view: owner.view(), disabled: false,
    canSave: owner.closeState().canSave, onstage() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {} } }).body;
  const normal = html();
  assert.equal((normal.match(/data-sivi-cover-column=/g) || []).length, 11);
  assert.equal((normal.match(/<input\b/g) || []).length, 11);
  assert.match(normal, /grid-cols-1.*sm:grid-cols-2.*lg:grid-cols-4/);
  assert.match(normal, /application ID 1/);
  assert.match(normal, /Collected \(\?\)/);
  assert.match(normal, /Tree\/Shrubs: RAW/);
  for (const column of editor.siviCoverColumns.filter(column => !['Cover5a', 'Cover5b', 'Cover5c'].includes(column))) {
    const controlId = `sivi-cover-${id}-${column}`;
    assert.equal((normal.match(new RegExp(`id="${controlId}"`, 'g')) || []).length, 1);
    assert.ok(normal.includes(`for="${controlId}"`));
  }
  for (const column of ['HeightA', 'HeightB', 'Height6', 'Species', 'Collected', 'ID', 'PlotNumber']) {
    assert.ok(!normal.includes(`data-sivi-cover-column="${column}"`));
  }
  owner.presentation(true);
  const extended = html();
  assert.equal((extended.match(/data-sivi-cover-column=/g) || []).length, 14);
  owner.stage(id, 'Cover5a', '100', false); owner.presentation(false);
  const blocked = html();
  assert.match(blocked, /Show extended controls/);
  assert.ok(blocked.indexOf('strictly less than 100') < blocked.indexOf('data-sivi-cover-group'));
  assert.match(blocked, /data-sivi-cover-save[^>]*disabled/);
  assert.ok(blocked.indexOf('No totals are calculated automatically') > blocked.indexOf('data-sivi-cover-group'));
});
