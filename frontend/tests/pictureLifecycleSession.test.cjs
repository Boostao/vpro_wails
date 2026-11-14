const assert = require('node:assert/strict');
const { test } = require('node:test');
const { editor, fixture, owner, requestId, deletion, review, restorationReview, restoration, loadTypeScript } = require('./pictureLifecycleTestHelpers.cjs');
const { picture } = require('./pictureCreationTestHelpers.cjs');
const { PictureLifecycleSession } = loadTypeScript('pictureLifecycleSession.ts', {
  './pictureLifecycleEditor': editor, './pictureRead': picture, './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
});
function deferred() {
  let resolve, reject, cancelled = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancelled++; };
  return { promise, resolve, reject, cancelled: () => cancelled };
}
function setup(overrides = {}) {
  const calls = { deletes: [], restores: [], receipts: [], restorationReceipts: [] };
  const port = {
    read: () => Promise.resolve(review()),
    history: () => Promise.resolve({ ...owner, events: [{ deletion: deletion(), restoration: null }] }),
    review: () => Promise.resolve(restorationReview()),
    delete: json => { calls.deletes.push(json); return Promise.resolve({ ...deletion(), request: JSON.parse(json) }); },
    deletionReceipt: json => { calls.receipts.push(json); return Promise.resolve({ ...deletion(), request: JSON.parse(json), replayed: true }); },
    restore: json => { calls.restores.push(json); return Promise.resolve(restoration(JSON.parse(json))); },
    restorationReceipt: json => { calls.restorationReceipts.push(json); return Promise.resolve({ ...restoration(JSON.parse(json)), replayed: true }); },
    ...overrides,
  };
  return { session: new PictureLifecycleSession(owner, port), calls };
}

test('review/confirmation/discard do not mutate; committed receipt blocks close until acknowledgement and survives it', async () => {
  const { session, calls } = setup();
  assert.equal(await session.reviewDeletion('-7'), true);
  assert.equal(session.closeState().unsaved, true);
  assert.equal(session.closeState().canSave, false);
  session.confirm(true);
  assert.equal(session.closeState().canSave, true);
  assert.equal(await session.save(requestId), true);
  assert.equal(calls.deletes.length, 1);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.view().acknowledgementRequired, true);
  assert.equal(session.closeState().unsaved, true);
  session.acknowledge();
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.closeState().unsaved, false);
  await session.reviewDeletion('-7');
  session.discard();
  assert.equal(calls.deletes.length, 1);
  assert.equal(session.view().receipts.length, 1);
});

test('unknown mutation retains exact request through cancellation/remount-style subscriptions and readonly recovery without replay', async () => {
  const { session, calls } = setup();
  await session.reviewDeletion('-7'); session.confirm(true);
  const held = deferred();
  let dispatched;
  session.port.delete = json => { calls.deletes.push(json); dispatched = json; return held.promise; };
  const save = session.save(requestId);
  session.cancel();
  assert.equal(session.view().blocked, true);
  let remounted;
  const stop = session.subscribe(() => { remounted = session.view(); });
  stop();
  assert.equal(remounted.original.records.rows[0].rowId, '-7');
  held.resolve({ ...deletion(), request: JSON.parse(dispatched) });
  assert.equal(await save, false);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(await session.resolve(), true);
  assert.deepEqual(calls.receipts, [dispatched]);
  assert.equal(calls.deletes.length, 1);
  assert.equal(session.view().receipts[0].receipt.replayed, true);
});

test('missing or malformed recovery remains blocked and cannot discard or replace unknown original', async () => {
  for (const wire of [null, undefined, {}, { ...deletion(), id: 9 }]) {
    const { session, calls } = setup({
      delete: () => Promise.reject(new Error('lost delivery')),
      deletionReceipt: () => Promise.resolve(wire),
    });
    await session.reviewDeletion('-7'); session.confirm(true);
    assert.equal(await session.save(requestId), false);
    assert.equal(await session.resolve(), false);
    assert.equal(session.closeState().blocked, true);
    assert.throws(() => session.discard());
    await assert.rejects(() => session.reviewDeletion('-7'));
    assert.equal(session.view().receipts.length, 0);
    assert.equal(calls.restores.length, 0);
  }
});

test('synchronous subscriber cancellation before mutation dispatch leaves no phantom unknown operation', async () => {
  const { session, calls } = setup();
  await session.reviewDeletion('-7'); session.confirm(true);
  const stop = session.subscribe(() => {
    if (session.view().busy && session.view().confirmed) session.cancel();
  });
  assert.equal(await session.save(requestId), false);
  stop();
  assert.equal(calls.deletes.length, 0);
  assert.equal(session.view().blocked, false);
  assert.equal(session.view().confirmed, false);
  assert.equal(session.view().original.records.rows[0].rowId, '-7');
  await assert.rejects(() => session.resolve());
});

test('late cancelled original/history reads cannot populate the retained panel', async () => {
  for (const history of [false, true]) {
    const held = deferred(), { session } = setup(history ? { history: () => held.promise } : { read: () => held.promise });
    const pending = history ? session.loadHistory() : session.reviewDeletion('-7');
    session.cancel();
    held.resolve(history ? { ...owner, events: [{ deletion: deletion(), restoration: null }] } : review());
    assert.equal(await pending, false);
    assert.equal(session.view().original, null);
    assert.equal(session.view().history, null);
    assert.equal(session.view().busy, false);
    assert.equal(held.cancelled(), 1);
  }
});

test('restoration requires complete current review and explicit confirmation while retaining independent actor and immutable deletion', async () => {
  const { session, calls } = setup();
  assert.equal(await session.loadHistory(), true);
  assert.equal(await session.reviewRestoration('1'), true);
  assert.equal(session.closeState().canSave, false);
  session.confirm(true);
  assert.equal(await session.save(fixture.restorationRequestId), true);
  assert.equal(calls.restores.length, 1);
  assert.equal(calls.deletes.length, 0);
  const receipt = session.view().receipts[0];
  assert.equal(receipt.kind, 'restore');
  assert.equal(receipt.receipt.actor, 'later actor');
  assert.equal(receipt.receipt.deletion.actor, 'original actor');
  session.acknowledge();
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.view().revision, 1);
});

test('history and review errors preserve prior evidence explicitly without fabricating source rows', async () => {
  const { session } = setup();
  await session.loadHistory();
  const before = session.view().history;
  session.port.history = () => Promise.reject(new Error('read failed'));
  assert.equal(await session.loadHistory(), false);
  assert.deepEqual(session.view().history, before);
  assert.match(session.view().error, /read failed/);
  session.port.read = () => Promise.resolve({ ...review(), project: 'foreign' });
  assert.equal(await session.reviewDeletion('-7'), false);
  assert.equal(session.view().original, null);
  assert.match(session.view().error, /another context/);
});
