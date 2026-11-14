const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { receipt, TwoPageSoilSession, fixture, original, wire, cell, outcome, outcomes } = require('./twoPageSoilTestHelpers.cjs');
const owner = fixture.owner;
const requestId = fixture.request.requestId;
function deferred() {
  let resolve, reject, cancelled = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancelled++; };
  return { promise, resolve, reject, cancelled: () => cancelled };
}
function setup(overrides = {}) {
  const calls = { saves: [], receipts: [] };
  const port = {
    read: () => Promise.resolve(original()),
    save: json => { calls.saves.push(json); return Promise.resolve(wire()); },
    outcome: json => { calls.receipts.push(json); return Promise.resolve(outcome(json)); },
    ...overrides,
  };
  return { session: new TwoPageSoilSession(owner, port), port, calls };
}
function stage(session) {
  session.stage('-7', 'Horizon', 'LF', false);
  session.stage('-7', 'UpperDepth', '5', false);
  session.stage('-7', 'Comment', '', true);
}

test('actual Go receipts preserve all four audit policies and detach live Svelte proxy history', () => {
  for (const strength of [0, 1, 2, 3]) {
    const result = receipt.twoPageSoilReceiptFromWire(client.proxy(wire(strength)), client.proxy(fixture.request), owner);
    assert.deepEqual(structuredClone(result), wire(strength));
    assert.equal(result.history.Audits.length, strength);
    assert.equal(result.history.Committed.rows[1].cells[0].storage, 'null');
    assert.doesNotThrow(() => structuredClone(result));
    assert.equal(receipt.twoPageSoilReceiptFromWire({ ...wire(strength), didCommit: false, replayed: true },
      fixture.request, owner, true).replayed, true);
    assert.throws(() => receipt.twoPageSoilReceiptFromWire(wire(strength), fixture.request, owner, true));
  }
});

test('receipt validator refuses altered complete source plans, exact requests and audit provenance', () => {
  for (const mutate of [
    value => value.didCommit = false,
    value => value.replayed = true,
    value => value.extra = null,
    value => value.history.Request.extra = true,
    value => value.history.Request.requestId = 'f4b1aa00-1234-4123-8123-000000000002',
    value => value.history.Request.original.contextId = 'another context',
    value => value.history.Request.edits[0].expected = cell('text', 'LF'),
    value => value.history.Request.edits[0].value = cell('text', 'changed'),
    value => value.history.Request.edits[0].column = 'ID',
    value => value.history.Request.edits.pop(),
    value => value.history.Committed.columns[1].declaredType = '',
    value => value.history.Committed.rows[0].cells[3].real = 6,
    value => value.history.Committed.rows[1].cells[0] = cell('integer', '0'),
    value => value.history.Committed.rows[0].rowId = '-8',
    value => value.history.Committed.rows.reverse(),
    value => value.history.Actor = '\ud800',
    value => value.history.Actor = 'x'.repeat(101),
    value => value.history.AuditStrength = 4,
    value => value.history.EditWhen = '2026-02-31 12:34:56',
    value => value.history.Audits.pop(),
    value => value.history.Audits[1].rowId = value.history.Audits[0].rowId,
    value => value.history.Audits[1].rowId = '9223372036854775808',
    value => value.history.Audits[1].id = 0,
    value => value.history.Audits[1].beforeEdit = '-0.250',
    value => value.history.Audits[1].afterEdit = '05',
    value => value.history.Audits[1].table = '_Mineral',
    value => value.history.Audits[1].restore = true,
    value => value.history.Audits[1].flag = true,
    value => value.history.Audits[1].user = 'another actor',
    value => value.history.Audits[1].extra = true,
  ]) {
    const result = wire(3); mutate(result);
    assert.throws(() => receipt.twoPageSoilReceiptFromWire(result, fixture.request, owner));
  }
});

test('historical BLOB edits reuse exact Go byte-slice audits without repairing unrelated metadata', () => {
  const result = wire(1);
  const request = result.history.Request;
  request.original.records.rows[0].cells[2] = cell('blob', 'ff00');
  request.edits[0].expected = cell('blob', 'ff00');
  result.history.Audits.unshift({ ...result.history.Audits[0], rowId: '-1', editField: 'Horizon',
    beforeEdit: '[255 0]', afterEdit: 'LF' });
  assert.equal(receipt.twoPageSoilReceiptFromWire(result, request, owner).history.Audits[0].beforeEdit, '[255 0]');
  result.history.Audits[0].beforeEdit = 'ff00';
  assert.throws(() => receipt.twoPageSoilReceiptFromWire(result, request, owner));
});

test('typed storage transitions do not reconstruct raw input or omit legitimate INTEGER-to-REAL writes', async () => {
  const result = wire();
  result.history.Request.original.records.columns[3].declaredType = '';
  result.history.Request.original.records.rows[0].cells[3] = cell('integer', '5');
  result.history.Request.edits[1].expected = cell('integer', '5');
  result.history.Committed.columns[3].declaredType = '';
  result.history.Audits[0].beforeEdit = '5';
  const { session } = setup({
    read: () => Promise.resolve(result.history.Request.original),
    save: () => Promise.resolve(result),
  });
  await session.load();
  session.stage('-7', 'Horizon', 'LF', false);
  session.stage('-7', 'UpperDepth', '5.0', false);
  session.stage('-7', 'Comment', '', true);
  assert.equal(await session.save(requestId), true);
  const retained = session.view().receipts[0];
  assert.equal(retained.history.Request.edits[1].expected.storage, 'integer');
  assert.equal(retained.history.Request.edits[1].value.storage, 'real');
  assert.equal(retained.history.Audits[0].beforeEdit, '5');
  assert.equal(retained.history.Audits[0].afterEdit, '5');
});

test('actual Go outcomes strictly echo retained bytes, scope, status and nullable committed receipt', () => {
  for (const expected of outcomes) {
    const decoded = receipt.twoPageSoilOutcomeFromWire(client.proxy(structuredClone(expected)),
      fixture.request, expected.requestJSON, owner);
    assert.equal(decoded.status, expected.status);
    assert.doesNotThrow(() => structuredClone(decoded));
  }
  for (const mutate of [
    value => value.extra = true,
    value => delete value.receipt,
    value => value.contextId += '-stale',
    value => value.requestId = 'f4b1aa00-1234-4123-8123-000000000002',
    value => value.requestJSON += ' ',
    value => value.status = 'success',
    value => value.reason = '',
    value => value.reason = '\ud800',
    value => value.receipt = wire(),
  ]) {
    const result = structuredClone(outcomes[2]); mutate(result);
    assert.throws(() => receipt.twoPageSoilOutcomeFromWire(result, fixture.request, outcomes[2].requestJSON, owner));
  }
  const oldRequest = structuredClone(fixture.request); oldRequest.original.contextId += '-old';
  assert.throws(() => receipt.twoPageSoilOutcomeFromWire(outcomes[2], oldRequest, outcomes[2].requestJSON, owner));
});

test('only server-sealed no-commit certification releases an unknown attempt while keeping correction/Undo and retiring its UUID', async () => {
  const { session, port, calls } = setup({ save: json => { calls.saves.push(json); return Promise.reject(new Error('writer refused')); } });
  await session.load(); stage(session); await session.save(requestId);
  port.outcome = json => { calls.receipts.push(json); return Promise.resolve(outcome(json, 'notCommitted')); };
  assert.equal(await session.resolve(), true);
  assert.equal(session.view().blocked, false);
  assert.equal(session.view().noCommitCertificate.requestId, requestId);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(Object.keys(session.view().drafts).length, 3);
  assert.equal(calls.saves.length, 1);
  assert.equal(calls.receipts[0], calls.saves[0]);
  await assert.rejects(session.save(requestId), /new UUID/);
  session.stage('-7', 'UpperDepth', '5.0', false);
  const nextID = 'f4b1aa00-1234-4123-8123-000000000002';
  port.save = json => { calls.saves.push(json); const result = wire(); result.history.Request = JSON.parse(json); return Promise.resolve(result); };
  assert.equal(await session.save(nextID), true);
  assert.equal(calls.saves.length, 2);
  assert.equal(session.view().receipts.length, 1);
  session.acknowledge(); session.undo();
  assert.equal(session.closeState().unsaved, false);
});

test('running and unknown outcomes retain the exact pending request without automatic dispatch or inferred disposal', async () => {
  const { session, port, calls } = setup({ save: () => Promise.reject(new Error('unknown delivery')) });
  await session.load(); stage(session); await session.save(requestId);
  for (const status of ['running', 'unknown']) {
    port.outcome = json => { calls.receipts.push(json); return Promise.resolve(outcome(json, status)); };
    assert.equal(await session.resolve(), false);
    assert.equal(session.view().blocked, true);
    assert.throws(() => session.undo(), /unknown/);
  }
  assert.equal(calls.receipts[0], calls.receipts[1]);
  assert.equal(session.view().noCommitCertificate, null);
});

test('a stale no-commit validation yield cannot release an unknown owner before current-generation recovery', async () => {
  const { session, port } = setup({ save: () => Promise.reject(new Error('unknown delivery')) });
  await session.load(); stage(session); await session.save(requestId);
  port.outcome = json => Promise.resolve(outcome(json, 'notCommitted'));
  const entered = deferred(), release = deferred();
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const resolving = session.resolve();
  await entered.promise;
  session.cancel(); release.resolve();
  assert.equal(await resolving, false);
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().noCommitCertificate, null);
  port.validationYield = undefined;
  assert.equal(await session.resolve(), true);
  session.undo();
  assert.equal(session.closeState().unsaved, false);
});

test('raw invalid drafts survive subscription remounts, block Save/close and clear only on correction or Undo', async () => {
  const { session } = setup();
  assert.equal(await session.load(), true);
  session.stage('-7', 'Horizon', '123456789', false);
  session.stage('-7', 'UpperDepth', '1e', false);
  let remounted;
  const stop = session.subscribe(() => { remounted = session.view(); }); stop();
  assert.equal(Object.values(remounted.drafts)[0].raw, '123456789');
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(requestId), /UTF-16|exceeds/);
  session.stage('-7', 'Horizon', 'LF', false);
  assert.match(session.view().error, /UpperDepth/);
  session.stage('-7', 'UpperDepth', '5', false);
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().canSave, true);
  session.undo();
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.view().error, null);
});

test('one Save commits exact shared wire; acknowledgement, refreshed source and detached views retain irreversible receipts', async () => {
  const { session, port, calls } = setup({ read: () => Promise.resolve(client.proxy(original())) });
  await session.load(); stage(session);
  assert.equal(await session.save(requestId), true);
  assert.deepEqual(JSON.parse(calls.saves[0]), fixture.request);
  assert.equal(session.view().revision, 1);
  assert.equal(session.closeState().blocked, true);
  assert.throws(() => session.undo(), /Acknowledge/);
  const detached = session.view();
  detached.receipts[0].history.Actor = 'untrusted view mutation';
  detached.original.records.rows[0].cells[2].text = 'untrusted original mutation';
  assert.equal(session.view().receipts[0].history.Actor, 'Fixture actor');
  session.acknowledge();
  assert.equal(session.closeState().unsaved, false);
  const later = original(); later.records.rows[0].cells[4] = cell('text', 'later independent edit');
  port.read = () => Promise.resolve(later);
  await session.load();
  assert.equal(session.view().original.records.rows[0].cells[4].text, 'later independent edit');
  assert.equal(session.view().receipts[0].history.Committed.rows[0].cells[4].storage, 'null');
  assert.equal(calls.saves.length, 1);
  session.dispose();
  assert.equal(session.view().receipts.length, 1);
});

test('attempted failure and missing history never infer no commit or grant replay/Undo/disposal', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return Promise.reject(new Error('transport failed')); };
  port.outcome = json => { calls.receipts.push(json); return Promise.resolve(null); };
  assert.equal(await session.save(requestId), false);
  assert.equal(session.view().blocked, true);
  assert.throws(() => session.undo(), /unknown/);
  assert.throws(() => session.dispose(), /unknown/);
  assert.throws(() => session.stage('-7', 'Horizon', 'F', false), /unknown/);
  await assert.rejects(session.load(), /unknown/);
  await assert.rejects(session.save(requestId), /unknown/);
  assert.equal(await session.resolve(), false);
  assert.match(session.view().error, /Missing history/);
  assert.equal(session.view().blocked, true);
  assert.equal(calls.receipts[0], calls.saves[0]);
  assert.equal(Object.keys(session.view().drafts).length, 3);
});

test('cancelled Save preserves exact request; settlement barrier and late delivery cannot erase or replay it', async () => {
  const held = deferred();
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  port.save = json => { calls.saves.push(json); return held.promise; };
  const saving = session.save(requestId);
  session.cancel();
  assert.equal(held.cancelled(), 1);
  assert.equal(session.view().blocked, true);
  await assert.rejects(session.resolve(), /settle/);
  held.resolve(wire());
  assert.equal(await saving, false);
  assert.equal(session.view().receipts.length, 0);
  let remounted;
  const stop = session.subscribe(() => { remounted = session.view(); }); stop();
  assert.equal(Object.keys(remounted.drafts).length, 3);
  assert.equal(await session.resolve(), true);
  assert.equal(calls.receipts[0], calls.saves[0]);
  assert.equal(calls.saves.length, 1);
  assert.equal(session.view().receipts[0].replayed, true);
  session.acknowledge();
  assert.equal(session.view().receipts.length, 1);
});

test('synchronous subscriber cancellation before dispatch retains editable drafts without invented attempts', async () => {
  const { session, calls } = setup();
  await session.load(); stage(session);
  let armed = true;
  const stop = session.subscribe(() => {
    if (armed && session.view().busy) { armed = false; session.cancel(); }
  });
  assert.equal(await session.save(requestId), false);
  stop();
  assert.equal(calls.saves.length, 0);
  assert.equal(session.view().blocked, false);
  assert.equal(session.closeState().canSave, true);
  assert.equal(await session.save(requestId), true);
  assert.equal(calls.saves.length, 1);
});

test('post-validation cancellation checks generation before assigning a receipt and recovers once without replay', async () => {
  const { session, port, calls } = setup();
  await session.load(); stage(session);
  const entered = deferred(), release = deferred();
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const saving = session.save(requestId);
  await entered.promise;
  session.cancel();
  release.resolve();
  assert.equal(await saving, false);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(session.view().blocked, true);
  port.validationYield = undefined;
  assert.equal(await session.resolve(), true);
  assert.equal(calls.saves.length, 1);
  assert.equal(session.view().receipts.length, 1);
});

test('cancelled read or validation cannot populate a disposed owner with stale source rows', async () => {
  const { session, port } = setup();
  await session.load();
  const held = deferred();
  port.read = () => held.promise;
  const reading = session.load();
  session.cancel(); session.dispose();
  const changed = original(); changed.records.rows[0].cells[4].text = 'late source';
  held.resolve(changed);
  assert.equal(await reading, false);
  assert.equal(session.view().original.records.rows[0].cells[4].text, '');
  assert.throws(() => session.subscribe(() => {}), /disposed/);
});

test('load validation yield cannot populate an owner after cancellation and safe disposal', async () => {
  const entered = deferred(), release = deferred();
  const { session } = setup({ validationYield: () => { entered.resolve(); return release.promise; } });
  const loading = session.load();
  await entered.promise;
  session.cancel(); session.dispose(); release.resolve();
  assert.equal(await loading, false);
  assert.equal(session.view().original, null);
});

test('receipt recovery validation yield cannot acknowledge an old generation after remount-style cancellation', async () => {
  const { session, port } = setup({ save: () => Promise.reject(new Error('unknown delivery')) });
  await session.load(); stage(session); await session.save(requestId);
  const entered = deferred(), release = deferred();
  port.validationYield = () => { entered.resolve(); return release.promise; };
  const resolving = session.resolve();
  await entered.promise;
  session.cancel(); release.resolve();
  assert.equal(await resolving, false);
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(session.view().acknowledgementRequired, false);
  port.validationYield = undefined;
  assert.equal(await session.resolve(), true);
  assert.equal(session.view().receipts.length, 1);
});

test('cancelled receipt recovery retains blocked owner and rejects mismatched history before explicit correction/retry', async () => {
  const { session, port } = setup({ save: () => Promise.reject(new Error('unknown delivery')) });
  await session.load(); stage(session); await session.save(requestId);
  const held = deferred();
  port.outcome = () => held.promise;
  const recovering = session.resolve();
  session.cancel(); held.resolve(outcome(JSON.stringify(fixture.request)));
  assert.equal(await recovering, false);
  assert.equal(session.view().blocked, true);
  const corrupt = wire(); corrupt.history.Committed.rows[1].cells[0] = cell('integer', '0');
  port.outcome = json => Promise.resolve(outcome(json, 'committed', corrupt));
  assert.equal(await session.resolve(), false);
  assert.equal(session.view().blocked, true);
  port.outcome = json => Promise.resolve(outcome(json));
  assert.equal(await session.resolve(), true);
  assert.equal(session.view().receipts.length, 1);
});
