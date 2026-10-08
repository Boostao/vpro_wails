const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': [],
  '../../resources/project-metadata-template.json': [],
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const editor = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const session = loadTypeScript('siviHeightSession.ts', {
  './siviHeightEditor': editor, './projectMetadataRestore': restoration,
});
const id = '9007199254740993';
const { review: sourceReview } = require('./siviSourceFixture.cjs');
function review(extended = false) {
  return sourceReview({ extended });
}
function fixture(overrides = {}) {
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, notify: 0 };
  const port = {
    read: async extended => { calls.read++; return review(extended); },
    save: async (extended, original, edits) => {
      calls.save++;
      assert.equal(original[0].Form, extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC');
      assert.equal(edits[0].rowId, id);
      return { ChangedCells: edits.length, HistoryID: '1' };
    },
    restore: async (historyId, action) => {
      calls.restore++;
      assert.equal(historyId, '1');
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  const owner = new session.SIVIHeightSession('P', port, () => calls.notify++);
  return { owner, port, calls };
}
test('SIVI wire bridge validates actual nullable generated models without repairing missing cells', () => {
  const source = review(), result = session.siviProjectionFromWire(source, 'P', false);
  result[0].Rows[0].cells[2].text = 'changed';
  assert.equal(source[0].Rows[0].cells[2].text, 'RAW');
  for (const bad of [null, [], [{ ...source[0], Columns: null }], [{ ...source[0], Rows: null }],
    [{ ...source[0], Rows: [{ rowId: id, cells: null }] }]]) {
    assert.throws(() => session.siviProjectionFromWire(bad, 'P', false), /SIVI/);
  }
});
test('SIVI parent owner retains raw errors across presentation/remount snapshots and blocks Save/close', async () => {
  const { owner, calls } = fixture();
  assert.equal(await owner.load(), true);
  owner.stage(id, 'HeightB', 'x'.repeat(256), false);
  owner.presentation(true);
  const remount = owner.view();
  assert.equal(remount.review[0].Form, 'SubVegA-SIVI');
  assert.equal(remount.drafts[id].HeightB.raw.length, 256);
  assert.equal(owner.closeState().unsaved, true);
  assert.equal(owner.closeState().canSave, false);
  await assert.rejects(owner.save(), /invalid SIVI/);
  await assert.rejects(owner.load(), /Finish or Undo/);
  assert.equal(calls.save, 0);
  remount.drafts[id].HeightB.raw = 'foreign mutation';
  assert.equal(owner.view().drafts[id].HeightB.raw.length, 256);
  owner.stage(id, 'HeightB', '', true);
  assert.equal(owner.closeState().canSave, true);
  assert.equal(owner.view().error, null);
  assert.equal(owner.view().drafts[id].HeightB.value.storage, 'null');
  assert.equal(await owner.save(), true);
  assert.equal(owner.closeState().unsaved, false);
  assert.equal(owner.view().historyId, '1');
  assert.equal(calls.save, 1);
});
test('SIVI rejected Save retains drafts and allows a corrected/retried single owned Save', async () => {
  let rejected = true, saves = 0;
  const { owner } = fixture({ save: async (_, __, edits) => {
    saves++;
    if (rejected) throw new Error('stale source or cancelled');
    return { ChangedCells: edits.length, HistoryID: '' };
  } });
  await owner.load();
  owner.stage(id, 'HeightA', '3', false);
  assert.equal(await owner.save(), false);
  assert.equal(owner.view().drafts[id].HeightA.raw, '3');
  assert.equal(owner.closeState().canSave, true);
  assert.equal(owner.view().blocked, false);
  rejected = false;
  assert.equal(await owner.save(), true);
  assert.equal(saves, 2);
  assert.equal(owner.view().historyId, null);
});
test('SIVI incomplete Save acknowledgement and committed cleanup failures block retry until explicit Undo', async () => {
  for (const response of [null, { ChangedCells: 0, HistoryID: '1' },
    { ChangedCells: 1, HistoryID: '01' }, { ChangedCells: 1, HistoryID: '-1' },
    new Error('SIVI height edit committed but cleanup failed; reload before retrying: closed')]) {
    let saves = 0;
    const { owner, calls } = fixture({ save: async () => {
      saves++;
      if (response instanceof Error) throw response;
      return response;
    } });
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    assert.equal(await owner.save(), false);
    assert.equal(owner.view().blocked, true);
    assert.equal(owner.closeState().canSave, false);
    assert.equal(owner.view().historyId, null);
    await assert.rejects(owner.save(), /do not replay/);
    await assert.rejects(owner.load(), /Finish or Undo/);
    assert.throws(() => owner.stage(id, 'HeightA', '4', false), /Undo\/reload/);
    assert.equal(saves, 1);
    assert.equal(await owner.undo(), true);
    assert.equal(owner.closeState().unsaved, false);
    assert.equal(calls.refresh, 1);
  }
});
test('SIVI committed Save refresh failure remains an unsaved safety block, never a replayable draft', async () => {
  let fail = true;
  const { owner, calls } = fixture({ refreshParent: async () => {
    if (fail) throw new Error('parent refresh failed');
  } });
  await owner.load();
  owner.stage(id, 'HeightA', '3', false);
  assert.equal(await owner.save(), false);
  assert.equal(owner.view().blocked, true);
  assert.equal(editor.siviHeightDirty(owner.view().drafts), false);
  assert.equal(owner.closeState().unsaved, true);
  assert.equal(owner.view().historyId, '1');
  await assert.rejects(owner.save(), /do not replay/);
  fail = false;
  assert.equal(await owner.undo(), true);
  assert.equal(calls.save, 1);
  assert.equal(owner.closeState().unsaved, false);
});
test('SIVI retain/prune use exact generated acknowledgement fields and prevent completed replay', async () => {
  for (const action of ['retain', 'prune']) {
    const { owner, calls } = fixture();
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    await owner.save();
    assert.equal(await owner.restore(action), true);
    assert.equal(owner.view().historyId, null);
    assert.equal(owner.closeState().unsaved, false);
    await assert.rejects(owner.restore(action), /verified height history/);
    assert.equal(calls.restore, 1);
    await assert.rejects(owner.restore('cancel'), /explicit retain or prune/);
  }
});
test('SIVI rejected restoration retains history; unknown/committed results block replay', async () => {
  for (const response of [null, { cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 1 },
    { cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 },
    { restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 },
    new Error('tampered audit'),
    new Error('SIVI restoration committed but cleanup failed; do not replay: closed')]) {
    const { owner } = fixture({ restore: async () => {
      if (response instanceof Error) throw response;
      return response;
    } });
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    await owner.save();
    assert.equal(await owner.restore('retain'), false);
    const rejected = response instanceof Error && response.message === 'tampered audit';
    assert.equal(owner.view().blocked, !rejected);
    assert.equal(owner.view().historyId, rejected ? '1' : null);
  }
});
test('SIVI operation ownership rejects concurrent actions and disposed read results', async () => {
  let resolve;
  const { owner, calls } = fixture({ read: () => new Promise(done => resolve = done) });
  const pending = owner.load();
  assert.equal(owner.closeState().busy, true);
  await assert.rejects(owner.undo(), /Wait/);
  assert.throws(() => owner.presentation(true), /Wait/);
  owner.dispose();
  resolve(review());
  assert.equal(await pending, false);
  assert.equal(owner.view().review, null);
  assert.equal(calls.notify, 1);
  await assert.rejects(owner.load(), /ownership ended/);
});

test('SIVI post-commit source/restore refresh failures retain a no-replay block through failed Undo', async () => {
  for (const operation of ['save', 'restore']) {
    let failRead = false;
    const { owner, calls } = fixture({ read: async extended => {
      if (failRead) throw new Error('source read failed');
      return review(extended);
    } });
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    if (operation === 'restore') await owner.save();
    failRead = true;
    assert.equal(await (operation === 'save' ? owner.save() : owner.restore('prune')), false);
    assert.equal(owner.view().blocked, true);
    assert.equal(await owner.undo(), false);
    assert.equal(owner.view().blocked, true);
    assert.equal(owner.closeState().unsaved, true);
    failRead = false;
    assert.equal(await owner.undo(), true);
    assert.equal(owner.view().blocked, false);
    assert.equal(calls.save, 1);
    assert.equal(calls.restore, operation === 'restore' ? 1 : 0);
  }
});

test('SIVI clean historical/no-op drafts never call Save or fabricate history', async () => {
  const { owner, calls } = fixture();
  await owner.load();
  owner.stage(id, 'HeightA', '2', false);
  assert.equal(await owner.save(), true);
  assert.equal(calls.save, 0);
  assert.equal(owner.closeState().unsaved, false);
  assert.equal(owner.view().historyId, null);
});

test('SIVI ownership ending during a committed operation never refreshes a remounted parent', async () => {
  for (const operation of ['save', 'restore']) {
    let resolve;
    const { owner, calls, port } = fixture();
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    if (operation === 'restore') await owner.save();
    port[operation] = () => new Promise(done => resolve = done);
    const beforeRefresh = calls.refresh;
    const pending = operation === 'save' ? owner.save() : owner.restore('retain');
    owner.dispose();
    resolve(operation === 'save' ? { ChangedCells: 1, HistoryID: '1' }
      : { cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 });
    assert.equal(await pending, false);
    assert.equal(calls.refresh, beforeRefresh);
    assert.equal(owner.view().blocked, true);
  }
});

test('SIVI ownership ending during parent refresh never schedules another source read', async () => {
  for (const operation of ['save', 'restore']) {
    let resolve, refreshStarted;
    const started = new Promise(done => refreshStarted = done);
    const { owner, calls, port } = fixture();
    await owner.load();
    owner.stage(id, 'HeightA', '3', false);
    if (operation === 'restore') await owner.save();
    port.refreshParent = () => {
      refreshStarted();
      return new Promise(done => resolve = done);
    };
    const reads = calls.read;
    const pending = operation === 'save' ? owner.save() : owner.restore('retain');
    await started;
    owner.dispose();
    resolve();
    assert.equal(await pending, false);
    assert.equal(calls.read, reads);
    assert.equal(owner.view().blocked, true);
  }
});
