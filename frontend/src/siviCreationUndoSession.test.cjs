const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { owner, historyId, requestId, review, history, result, read, deferred, copy, sessions } = require('./siviCreationUndoTestHelpers.cjs');
function fixture(overrides = {}) {
  const calls = { history: 0, reviews: [], undos: [], lookups: [] };
  const port = {
    history() { calls.history++; return read(history()); },
    review(id) { calls.reviews.push(id); return read(review()); },
    undo(json) { calls.undos.push(json); return read(result(review(), JSON.parse(json))); },
    receipt(json) { calls.lookups.push(json); return read(result(review(), JSON.parse(json), true)); },
    ...overrides,
  };
  return { session: new sessions.SIVICreationUndoSession(owner, port), calls, port };
}
async function staged(f, action = 'retain') {
  assert.equal(await f.session.loadHistory(), true);
  assert.equal(await f.session.review(historyId), true);
  f.session.choose(action); f.session.confirm(true);
}
test('explicit loaded unconsumed selection only; no first event or remembered UUID inference', async () => {
  const f = fixture();
  await assert.rejects(f.session.review(historyId), /explicit unconsumed/);
  assert.equal(await f.session.loadHistory(), true);
  assert.equal(f.session.view().review, null);
  await assert.rejects(f.session.review(requestId), /explicit unconsumed/);
  assert.deepEqual(f.calls.reviews, []);
  assert.equal(await f.session.review(historyId), true);
  assert.equal(Object.keys(f.session.view().review).length, 13);
  await assert.rejects(f.session.review(historyId), /discard/);
  await assert.rejects(f.session.loadHistory(), /discard/);
});
for (const present of [false, true]) test(`missing and existing empty history are distinct: ${present}`, async () => {
  const f = fixture({ history: () => read({ ...owner, historyPresent: present, events: [] }) });
  assert.equal(await f.session.loadHistory(), true);
  assert.equal(f.session.view().history.historyPresent, present);
  await assert.rejects(f.session.review(historyId), /explicit unconsumed/);
});
test('consumed creation is visible but never reviewed', async () => {
  const f = fixture({ history: () => read(history(true)) });
  await f.session.loadHistory();
  assert.equal(f.session.view().history.events[0].consumed, true);
  await assert.rejects(f.session.review(historyId), /explicit unconsumed/);
  assert.deepEqual(f.calls.reviews, []);
});
test('coherent non-UUID history stays visible alongside available UUID choices without any legacy review RPC', async () => {
  const mixed = history();
  mixed.events.push({ ...mixed.events[0], historyId: 'legacy creation identity', reviewAvailable: false,
    unavailableReason: 'Legacy creation identity cannot be submitted through the UUID Undo protocol.' });
  const f = fixture({ history: () => read(mixed) });
  assert.equal(await f.session.loadHistory(), true);
  const loaded = f.session.view().history;
  assert.equal(loaded.events.length, 2);
  assert.equal(Object.keys(loaded.events[0]).length, 11);
  assert.equal(loaded.events[1].reviewAvailable, false);
  assert.equal(loaded.events[1].unavailableReason, mixed.events[1].unavailableReason);
  await assert.rejects(f.session.review('legacy creation identity'), /review-available/);
  assert.deepEqual(f.calls.reviews, []);
  assert.equal(await f.session.review(historyId), true);
  assert.deepEqual(f.calls.reviews, [historyId]);
});
test('history review availability does not bypass a fresh physical eligibility review', async () => {
  const f = fixture({ review(id) { f.calls.reviews.push(id); return read(null); } });
  assert.equal(await f.session.loadHistory(), true);
  assert.equal(f.session.view().history.events[0].reviewAvailable, true);
  assert.equal(await f.session.review(historyId), false);
  assert.deepEqual(f.calls.reviews, [historyId]);
  assert.equal(f.session.view().review, null);
  assert.equal(f.session.closeState().canSave, false);
  assert.equal(f.calls.undos.length, 0);
});
test('fresh review must match independently loaded historical identity', async () => {
  const wrong = history(); wrong.events[0].rowId = '2';
  const f = fixture({ history: () => read(wrong) });
  await f.session.loadHistory();
  assert.equal(await f.session.review(historyId), false);
  assert.match(f.session.view().error, /differs/);
  assert.equal(f.session.view().review, null);
});
test('separate audit action and confirmation; change resets confirmation; discard clears known errors', async () => {
  const f = fixture(); await f.session.loadHistory(); await f.session.review(historyId);
  assert.throws(() => f.session.confirm(true), /explicit audit action/);
  await assert.rejects(f.session.undo(requestId), /Choose/);
  f.session.choose('retain');
  await assert.rejects(f.session.undo(requestId), /Confirm/);
  f.session.confirm(true); assert.equal(f.session.closeState().canSave, true);
  f.session.choose('prune'); assert.equal(f.session.view().confirmed, false);
  f.session.discard(); assert.equal(f.session.closeState().unsaved, false);
  assert.equal(f.session.view().error, null); assert.equal(f.calls.undos.length, 0);
});
for (const action of ['retain', 'prune']) test(`exact seven-field request and 26-field verified ${action} receipt`, async () => {
  const f = fixture(); await staged(f, action);
  assert.equal(await f.session.undo(requestId), true);
  const request = JSON.parse(f.calls.undos[0]);
  assert.deepEqual(Object.keys(request).sort(), ['action', 'contextId', 'expected', 'historyId', 'plot', 'project', 'requestId']);
  assert.equal(request.expected, review().expected); assert.equal(request.action, action);
  const view = f.session.view();
  assert.equal(Object.keys(view.receipt).length, 26);
  assert.equal(view.receipt.removedRows, 1); assert.equal(view.receipt.prunedAuditRows, action === 'prune' ? 2 : 0);
  assert.equal(view.history, null); assert.equal(view.review, null); assert.equal(view.revision, 1);
  assert.equal(view.requestId, null); assert.equal(f.session.closeState().unsaved, false);
});
test('invalid/separate request UUID never submits or blocks a staged review', async () => {
  const f = fixture(); await staged(f);
  for (const id of ['bad', historyId]) await assert.rejects(f.session.undo(id), /UUIDv4/);
  assert.equal(f.calls.undos.length, 0); assert.equal(f.session.view().blocked, false);
  assert.equal(f.session.view().confirmed, true);
});
test('lost acknowledgement retains exact authority and can only lookup, never replay', async () => {
  const f = fixture({ undo(json) { f.calls.undos.push(json); return Object.assign(Promise.reject(new Error('lost')), { cancel() {} }); } });
  await staged(f);
  const before = f.session.view().review;
  assert.equal(await f.session.undo(requestId), false);
  assert.equal(f.session.closeState().blocked, true);
  assert.equal(f.session.closeState().unsaved, true);
  assert.deepEqual(f.session.view().review, before);
  await assert.rejects(f.session.undo(requestId), /Resolve/);
  await assert.rejects(f.session.review(historyId), /Resolve/);
  await assert.rejects(f.session.loadHistory(), /Resolve/);
  for (const operation of [() => f.session.discard(), () => f.session.choose('prune'), () => f.session.confirm(false)]) {
    assert.throws(operation, /Resolve/);
  }
  assert.equal(await f.session.resolve(), true);
  assert.equal(f.calls.lookups[0], f.calls.undos[0]);
  assert.equal(f.calls.undos.length, 1);
  assert.equal(f.session.view().receipt.replayed, true);
});
test('synchronous transport failure is unknown, not permission to discard or retry', async () => {
  const f = fixture({ undo(json) { f.calls.undos.push(json); throw new Error('dispatch failed'); } });
  await staged(f);
  assert.equal(await f.session.undo(requestId), false);
  assert.equal(f.session.view().blocked, true); assert.equal(f.session.view().busy, false);
  assert.throws(() => f.session.discard(), /Resolve/);
  f.port.receipt = () => { throw new Error('lookup dispatch failed'); };
  assert.equal(await f.session.resolve(), false);
  assert.equal(f.session.view().blocked, true); assert.equal(f.session.view().busy, false);
  f.port.receipt = json => { f.calls.lookups.push(json); return read(result(review(), JSON.parse(json), true)); };
  assert.equal(await f.session.resolve(), true);
  assert.equal(f.calls.lookups[0], f.calls.undos[0]);
});
for (const malformed of ['null', 'extra', 'cancelled', 'owner', 'row', 'audit', 'request', 'commit']) {
  test(`malformed ${malformed} acknowledgement remains unknown; malformed/missing lookup cannot release it`, async () => {
    const f = fixture({ undo(json) {
      f.calls.undos.push(json);
      const wire = result(review(), JSON.parse(json));
      if (malformed === 'null') return read(null);
      if (malformed === 'extra') wire.extra = true;
      if (malformed === 'cancelled') wire.cancelled = true;
      if (malformed === 'owner') wire.project = 'Other';
      if (malformed === 'row') wire.rowId = '2';
      if (malformed === 'audit') wire.auditsAfter = [];
      if (malformed === 'request') wire.request.expected = 'b'.repeat(64);
      if (malformed === 'commit') wire.didCommit = false;
      return read(wire);
    } });
    await staged(f);
    const before = f.session.view().review;
    assert.equal(await f.session.undo(requestId), false);
    for (const bad of [null, {}, result(review(), JSON.parse(f.calls.undos[0]))]) {
      f.port.receipt = json => { f.calls.lookups.push(json); return read(bad); };
      assert.equal(await f.session.resolve(), false);
      assert.equal(f.session.view().blocked, true);
      assert.deepEqual(f.session.view().review, before);
      assert.equal(f.session.view().requestId, requestId);
    }
    f.port.receipt = json => { f.calls.lookups.push(json); return read(result(review(), JSON.parse(json), true)); };
    assert.equal(await f.session.resolve(), true);
    assert.ok(f.calls.lookups.every(json => json === f.calls.undos[0]));
  });
}
test('cancel completed mutation delivery is not proof of SQL cancellation; late receipt ignored', async () => {
  const delivery = deferred();
  const f = fixture({ undo(json) { f.calls.undos.push(json); return delivery.promise; } });
  await staged(f);
  const inFlight = f.session.undo(requestId);
  delivery.resolve(result(review(), JSON.parse(f.calls.undos[0])));
  f.session.cancel();
  assert.equal(delivery.cancelled, 1);
  assert.equal(await inFlight, false);
  assert.equal(f.session.view().receipt, null); assert.equal(f.session.view().blocked, true);
  assert.match(f.session.view().error, /not proof of SQL cancellation/);
  assert.equal(await f.session.resolve(), true);
  assert.equal(f.calls.undos.length, 1);
});
test('mutation cannot be duplicated while busy; lookup cannot be requested before an unknown outcome', async () => {
  const delivery = deferred(), f = fixture({ undo: () => delivery.promise });
  await staged(f);
  await assert.rejects(f.session.resolve(), /No unknown/);
  const pending = f.session.undo(requestId);
  await assert.rejects(f.session.undo(requestId), /Wait/);
  await assert.rejects(f.session.resolve(), /Wait/);
  assert.throws(() => f.session.discard(), /Wait/);
  assert.equal(f.session.closeState().busy, true);
  f.session.cancel(); delivery.reject(new Error('late rejection'));
  assert.equal(await pending, false); assert.equal(f.session.view().blocked, true);
});
test('cancelled read cannot overwrite newer history; busy prevents duplicate operations', async () => {
  const delivery = deferred(), f = fixture({ history: () => delivery.promise });
  const first = f.session.loadHistory();
  await assert.rejects(f.session.loadHistory(), /Wait/);
  f.session.cancel();
  f.port.history = () => read(history(true));
  assert.equal(await f.session.loadHistory(), true);
  delivery.resolve(history(false)); assert.equal(await first, false);
  assert.equal(f.session.view().history.events[0].consumed, true);
  assert.equal(delivery.cancelled, 1);
});
test('cancelled review and lookup deliveries never replace retained authority', async () => {
  const reviewDelivery = deferred(), f = fixture({ review: () => reviewDelivery.promise });
  await f.session.loadHistory();
  const reading = f.session.review(historyId); f.session.cancel();
  reviewDelivery.resolve(review()); assert.equal(await reading, false);
  assert.equal(f.session.view().review, null);
  f.port.review = () => read(review()); await f.session.review(historyId);
  f.session.choose('retain'); f.session.confirm(true);
  f.port.undo = json => { f.calls.undos.push(json); return read(null); };
  await f.session.undo(requestId);
  const lookup = deferred(); f.port.receipt = () => lookup.promise;
  const resolving = f.session.resolve(); f.session.cancel();
  lookup.resolve(result(review(), JSON.parse(f.calls.undos[0]), true));
  assert.equal(await resolving, false); assert.equal(f.session.view().blocked, true);
  assert.equal(f.session.view().requestId, requestId);
});
test('errors and snapshots survive unsubscribe/remount; known discard clears read error', async () => {
  const f = fixture({ history: () => read(null) });
  let notifications = 0, detach = f.session.subscribe(() => { notifications++; f.session.view(); });
  await f.session.loadHistory();
  const error = f.session.view().error; assert.ok(error);
  detach(); const old = notifications;
  const view = f.session.view(); view.error = null;
  assert.equal(f.session.view().error, error);
  const remount = f.session.subscribe(() => { assert.equal(f.session.view().error, error); });
  remount(); f.session.discard();
  assert.equal(f.session.view().error, null); assert.equal(notifications, old);
});
test('owned snapshots cannot mutate retained historical44 review or source audits', async () => {
  const f = fixture(); await staged(f);
  const snapshot = f.session.view(), exact = f.session.view().review;
  snapshot.history.events[0].consumed = true;
  snapshot.review.committed.cells[0].integer = '999';
  snapshot.review.auditsBefore.length = 0;
  assert.deepEqual(f.session.view().review, exact);
  assert.equal(f.session.view().history.events[0].consumed, false);
  assert.equal(await f.session.undo(requestId), true);
});
test('owner cache uses actual four generated SDK methods, survives remount and isolates original plots', async () => {
  const calls = [], delivery = deferred();
  const sdk = {
    GetHistory(...args) { calls.push(['GetHistory', ...args]); return read(history()); },
    Review(...args) { calls.push(['Review', ...args]); return read(review()); },
    Undo(...args) { calls.push(['Undo', ...args]); return delivery.promise; },
    LookupReceipt(...args) { calls.push(['LookupReceipt', ...args]); return read(result(review(), JSON.parse(args[1]), true)); },
  };
  const cache = loadTypeScript('siviCreationUndoSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { SIVICreationUndoService: sdk },
    './siviCreationUndoSession': sessions,
  }).siviCreationUndoSession;
  const mutable = copy(owner), session = cache(mutable); mutable.plot = 'Changed';
  assert.equal(cache(copy(owner)), session);
  for (const key of ['contextId', 'project', 'plot']) assert.notEqual(cache({ ...owner, [key]: 'Other' }), session);
  await staged({ session });
  const detach = session.subscribe(() => session.view());
  const submitting = session.undo(requestId); session.cancel(); detach();
  const remounted = cache(owner);
  assert.equal(remounted, session); assert.equal(remounted.view().blocked, true);
  assert.ok(remounted.view().error);
  assert.equal(await remounted.resolve(), true);
  delivery.reject(new Error('late cancelled delivery')); assert.equal(await submitting, false);
  assert.deepEqual(calls[0], ['GetHistory', 'C', 'P']);
  assert.deepEqual(calls[1], ['Review', 'C', 'P', historyId]);
  assert.equal(calls[2][0], 'Undo'); assert.equal(calls[3][0], 'LookupReceipt');
  assert.equal(calls[2][1], 'C'); assert.equal(calls[3][2], calls[2][2]);
  assert.equal(JSON.parse(calls[2][2]).plot, 'P'); assert.equal(session.view().revision, 1);
});
