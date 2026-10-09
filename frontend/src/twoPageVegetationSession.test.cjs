const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { wire, cell, TwoPageVegetationSession } = require('./twoPageVegetationTestHelpers.cjs');
const fixtures = require('./twoPageVegetationReceipts.json');
const outcomeFixtures = require('./twoPageVegetationOutcomes.json');
const saved = (index = 3) => structuredClone(fixtures[index]);
const seed = saved().history.Request;
const requestId = seed.requestId;
const nextID = 'b0cbfa00-1234-4123-8123-000000000002';
const ownerOf = request => {
  const { contextId, project, plot, form } = request.original;
  return { contextId, project, plot, form };
};
const owner = ownerOf(seed);
function deferred() {
  let resolve, reject, cancellations = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancellations++; };
  return { promise, resolve, reject, cancellations: () => cancellations };
}
function ready(value) {
  const promise = Promise.resolve(value);
  promise.cancel = () => {};
  return promise;
}
function setup(overrides = {}, writing = true, index = 3) {
  const source = saved(index), scoped = ownerOf(source.history.Request), calls = { reads: 0, saves: [], outcomes: [] };
  const port = {
    read: () => { calls.reads++; return ready(client.proxy(structuredClone(source.history.Request.original))); },
    save: json => { calls.saves.push(json); return ready(client.proxy(structuredClone(source))); },
    outcome: json => { calls.outcomes.push(json); return ready(structuredClone(outcomeFixtures[3])); },
    ...overrides,
  };
  return { session: new TwoPageVegetationSession(scoped, port, writing), port, calls, scoped };
}
function stage(session) {
  session.stage('-7', 'Cover1', '5', false);
  session.stage('-7', 'Cover2', '', true);
  session.stage('-7', 'Collected', 'C', false);
}
function outcome(status, json) {
  const result = structuredClone(outcomeFixtures.find(value => value.status === status));
  if (json !== undefined) {
    const request = JSON.parse(json);
    result.requestJSON = json; result.requestId = request.requestId; result.contextId = request.original.contextId;
  }
  return result;
}

test('physical selection survives reload and subscriptions without silently replacing a missing selected row', async () => {
  let original = structuredClone(seed.original);
  const { session } = setup({ read: () => ready(client.proxy(structuredClone(original))) });
  await session.load();
  assert.equal(session.view().selectedRowId, '-7');
  session.select('3');
  const stop = session.subscribe(() => {});
  stop();
  await session.load();
  assert.equal(session.view().selectedRowId, '3');
  original.records.rows = original.records.rows.filter(row => row.rowId !== '3');
  original.sourceRowIds = original.sourceRowIds.filter(rowId => rowId !== '3');
  assert.equal(await session.load(), true);
  assert.equal(session.view().selectedRowId, '3');
  assert.throws(() => session.select('3'), /observed physical/);
  assert.equal(session.view().selectedRowId, '3');
  session.select('8');
  assert.equal(session.view().selectedRowId, '8');
});

test('selection refuses active operations but unknown-outcome inspection never replaces the retained request', async () => {
  const read = deferred(), save = deferred();
  const requests = [];
  const { session } = setup({
    read: () => read.promise,
    save: json => { requests.push(json); return save.promise; },
    outcome: json => { assert.equal(json, requests[0]); return ready(outcome('running', json)); },
  });
  const loading = session.load();
  assert.throws(() => session.select('-7'), /Wait for or cancel/);
  read.resolve(structuredClone(seed.original));
  assert.equal(await loading, true);
  stage(session);
  const saving = session.save(requestId);
  assert.throws(() => session.select('3'), /Wait for or cancel/);
  save.reject(new Error('unknown delivery'));
  assert.equal(await saving, false);
  const retained = session.view();
  session.select('3');
  assert.equal(session.view().selectedRowId, '3');
  assert.deepEqual(session.view().original, retained.original);
  assert.deepEqual(session.view().drafts, retained.drafts);
  assert.equal(session.closeState().blocked, true);
  assert.throws(() => session.stage('3', 'Cover1', '5', false), /unknown/);
  assert.equal(await session.resolve(), false);
  assert.equal(requests.length, 1);
  assert.equal(session.view().selectedRowId, '3');
  assert.equal(session.closeState().blocked, true);
});

test('vegetation sessions copy immutable owners and default to read-only without hiding physical historical peers', async () => {
  const mutable = client.proxy({ ...owner });
  const session = new TwoPageVegetationSession(mutable, { read: () => ready(client.proxy(structuredClone(seed.original))) });
  mutable.contextId = 'changed caller'; mutable.form = 'SubVegCXL';
  assert.equal(await session.load(), true);
  assert.equal(session.view().original.contextId, owner.contextId);
  assert.equal(session.view().original.records.rows[2].cells[0].storage, 'null');
  assert.match(session.closeState().saveReason, /disabled/);
  assert.throws(() => session.stage('-7', 'Cover1', '5', false), /disabled/);
  await assert.rejects(session.save(requestId), /disabled/);
  session.dispose();
  assert.throws(() => session.subscribe(() => {}), /disposed/);
});

test('raw errors survive remounted subscriptions and block Save/reload/disposal until correction or Undo', async () => {
  const { session, calls } = setup();
  await session.load();
  session.stage('-7', 'Cover1', '100', false);
  session.stage('-7', 'Cover2', '1e', false);
  session.stage('-7', 'Collected', '\ud800', false);
  let retained;
  const stop = session.subscribe(() => { retained = session.view(); }); stop();
  assert.equal(Object.values(retained.drafts)[0].raw, '100');
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(requestId), /below100/);
  await assert.rejects(session.load(), /Correct or Undo/);
  assert.throws(() => session.dispose(), /Correct or Undo/);
  session.stage('-7', 'Cover1', '5', false);
  assert.match(session.view().error, /finite number/);
  session.stage('-7', 'Cover2', '5', false);
  assert.match(session.view().error, /Unicode/);
  session.stage('-7', 'Collected', 'C', false);
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().canSave, true);
  assert.equal(calls.saves.length, 0);
  session.undo();
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.view().error, null);
});

test('one exact Save survives acknowledgement remounts detached view mutation and independent reload without replay', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  assert.equal(await session.save(requestId), true);
  assert.equal(calls.saves[0], wire.serializeTwoPageVegetationRequest(seed));
  assert.equal(session.view().revision, 1);
  assert.equal(session.closeState().blocked, true);
  assert.throws(() => session.undo(), /Acknowledge/);
  const detached = session.view();
  detached.original.records.rows[0].cells[2] = cell('text', 'intruder');
  detached.receipts[0].history.Actor = 'intruder';
  assert.notEqual(session.view().receipts[0].history.Actor, 'intruder');
  assert.notEqual(session.view().original.records.rows[0].cells[2].text, 'intruder');
  session.acknowledge();
  const later = session.view().original; later.records.rows[0].cells[2] = cell('text', 'later observation');
  port.read = () => ready(client.proxy(later));
  await session.load();
  assert.equal(session.view().original.records.rows[0].cells[2].text, 'later observation');
  assert.equal(session.view().receipts[0].history.Committed.rows[0].cells[2].text, 'TEST');
  session.stage('-7', 'Cover1', '6', false);
  await assert.rejects(session.save(requestId), /already committed/);
  session.undo(); session.dispose();
  assert.equal(session.view().receipts.length, 1);
  assert.equal(calls.saves.length, 1);
});

test('last-cover Save recomputes membership while retaining the physical row and preventing another hidden-row edit', async () => {
  const { session, calls } = setup({}, true, 4);
  await session.load();
  session.stage('-7', 'Cover6', '', true); session.stage('-7', 'Collected', 'C', false);
  assert.equal(await session.save(requestId), true);
  assert.equal(session.view().original.sourceRowIds.includes('-7'), false);
  assert.equal(session.view().original.records.rows[0].rowId, '-7');
  session.acknowledge();
  assert.throws(() => session.stage('-7', 'Cover6', '5', false), /outside/);
  assert.equal(calls.saves.length, 1);
});

test('attempted failure or missing history cannot permit replay correction Undo reload or disposal', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); throw new Error('dispatch delivery unknown'); };
  port.outcome = json => { calls.outcomes.push(json); return ready(null); };
  assert.equal(await session.save(requestId), false);
  assert.equal(session.view().blocked, true);
  assert.throws(() => session.undo(), /unknown/);
  assert.throws(() => session.dispose(), /unknown/);
  assert.throws(() => session.stage('-7', 'Cover1', '6', false), /unknown/);
  await assert.rejects(session.load(), /unknown/);
  await assert.rejects(session.save(nextID), /unknown/);
  assert.equal(await session.resolve(), false);
  assert.match(session.view().error, /Missing history/);
  assert.equal(calls.outcomes[0], calls.saves[0]);
  assert.equal(Object.keys(session.view().drafts).length, 3);
});

test('only sealed absence releases failed UUID for correction with a new request while irreversible receipts remain separate', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return Promise.reject(new Error('writer refused')); };
  await session.save(requestId);
  port.outcome = json => { calls.outcomes.push(json); return ready(outcome('notCommitted', json)); };
  assert.equal(await session.resolve(), true);
  assert.equal(session.view().noCommitCertificate.requestId, requestId);
  assert.equal(session.view().blocked, false);
  assert.equal(Object.keys(session.view().drafts).length, 3);
  await assert.rejects(session.save(requestId), /new UUID/);
  session.stage('-7', 'Cover1', '5.0', false);
  port.save = json => {
    calls.saves.push(json); const result = saved(); result.history.Request = JSON.parse(json); return ready(result);
  };
  assert.equal(await session.save(nextID), true);
  assert.equal(session.view().noCommitCertificate, null);
  session.acknowledge(); session.undo();
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(calls.saves.length, 2);
  assert.equal(calls.outcomes[0], calls.saves[0]);
});

test('running unknown and malformed recovery outcomes keep the exact pending attempt', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return Promise.reject(new Error('unknown')); };
  await session.save(requestId);
  for (const status of ['running', 'unknown']) {
    port.outcome = json => { calls.outcomes.push(json); return ready(outcome(status, json)); };
    assert.equal(await session.resolve(), false);
    assert.equal(session.view().blocked, true);
    assert.throws(() => session.undo(), /unknown/);
  }
  port.outcome = json => ready({ ...outcome('notCommitted', json), requestJSON: json + ' ' });
  assert.equal(await session.resolve(), false);
  assert.equal(session.view().noCommitCertificate, null);
  assert.equal(calls.outcomes[0], calls.saves[0]);
  assert.equal(calls.outcomes[1], calls.saves[0]);
});

test('cancelled Save waits for transport settlement and recovers once without accepting late results or redispatch', async () => {
  const held = deferred(), { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return held.promise; };
  const saving = session.save(requestId);
  session.cancel();
  assert.equal(held.cancellations(), 1);
  assert.equal(session.view().blocked, true);
  await assert.rejects(session.resolve(), /settle/);
  held.resolve(client.proxy(saved()));
  assert.equal(await saving, false);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(await session.resolve(), true);
  assert.equal(calls.outcomes[0], calls.saves[0]);
  assert.equal(calls.saves.length, 1);
  assert.equal(session.view().receipts.length, 1);
  session.acknowledge();
});

test('generation barriers discard stale original validation without overwriting a new original', async () => {
  const entered = deferred(), release = deferred(), { session, port } = setup();
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const loading = session.load(); await entered.promise;
  session.cancel();
  const next = structuredClone(seed.original); next.records.rows[0].cells[2] = cell('text', 'fresh physical source');
  port.read = () => ready(client.proxy(next)); port.validationYield = undefined;
  assert.equal(await session.load(), true);
  release.resolve();
  assert.equal(await loading, false);
  assert.equal(session.view().original.records.rows[0].cells[2].text, 'fresh physical source');
});

test('cancelled committed validation stays unknown until exact readonly recovery validates in the current generation', async () => {
  const entered = deferred(), release = deferred(), { session, port, calls } = setup();
  await session.load(); stage(session);
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const saving = session.save(requestId); await entered.promise;
  session.cancel(); release.resolve();
  assert.equal(await saving, false);
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().receipts.length, 0);
  port.validationYield = undefined;
  assert.equal(await session.resolve(), true);
  assert.equal(calls.saves.length, 1);
  assert.equal(session.view().receipts.length, 1);
});

test('stale no-commit validation cannot retire or release a pending owner', async () => {
  const { session, port } = setup(), entered = deferred(), release = deferred();
  await session.load(); stage(session);
  port.save = () => Promise.reject(new Error('unknown')); await session.save(requestId);
  port.outcome = json => ready(outcome('notCommitted', json));
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const recovering = session.resolve(); await entered.promise;
  session.cancel(); release.resolve();
  assert.equal(await recovering, false);
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().noCommitCertificate, null);
  port.validationYield = undefined;
  assert.equal(await session.resolve(), true);
  session.undo(); session.dispose();
});

test('begin-time listener cancellation prevents dispatch and malformed reads never replace an earlier original', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  const stop = session.subscribe(() => { if (session.view().busy) session.cancel(); });
  assert.equal(await session.save(requestId), false);
  stop();
  assert.equal(calls.saves.length, 0);
  assert.equal(session.view().blocked, false);
  assert.equal(Object.keys(session.view().drafts).length, 3);
  session.undo();
  port.read = () => ready({ ...seed.original, contextId: 'unrelated' });
  assert.equal(await session.load(), false);
  assert.equal(session.view().original.contextId, owner.contextId);
  assert.match(session.view().error, /unavailable/);
});

test('lossless signed-zero request witnesses remain unchanged through failed dispatch certification and explicit retry', async () => {
  const { session, port, calls } = setup();
  const original = structuredClone(seed.original), column = wire.vegetationColumnIndex(original, 'Cover3');
  original.records.rows[1].cells[column] = cell('real', -0);
  port.read = () => ready(client.proxy(original));
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return Promise.reject(new Error('unknown')); };
  await session.save(requestId);
  assert.notEqual(calls.saves[0], JSON.stringify(JSON.parse(calls.saves[0])));
  port.outcome = json => { calls.outcomes.push(json); return ready(outcome('notCommitted', json)); };
  assert.equal(await session.resolve(), true);
  assert.equal(calls.outcomes[0], calls.saves[0]);
  port.save = json => {
    calls.saves.push(json);
    const result = saved(); result.history.Request = JSON.parse(json);
    result.history.Committed.rows[1].cells[column] = cell('real', -0);
    return ready(client.proxy(result));
  };
  assert.equal(await session.save(nextID), true);
  assert.equal(Object.is(session.view().original.records.rows[1].cells[column].real, -0), true);
  assert.equal(Object.is(session.view().receipts[0].history.Request.original.records.rows[1].cells[column].real, -0), true);
  session.acknowledge();
});
