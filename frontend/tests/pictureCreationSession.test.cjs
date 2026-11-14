const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { editor, picture, owner, selection, receipt, requestId } = require('./pictureCreationTestHelpers.cjs');
const reads = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const { PictureCreationSession } = loadTypeScript('pictureCreationSession.ts', {
  './readRequests': reads, './pictureRead': picture, './pictureCreationEditor': editor,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
function operation(value) {
  return Object.assign(Promise.resolve(value), { cancel() {} });
}
function held() {
  let resolve, reject, cancellations = 0;
  const promise = Object.assign(new Promise((yes, no) => { resolve = yes; reject = no; }),
    { cancel() { cancellations++; } });
  return { promise, resolve, reject, cancellations: () => cancellations };
}
function fixture(overrides = {}) {
  const calls = [];
  const request = editor.pictureCreationRequest(selection(), owner, requestId);
  const port = {
    choose() { calls.push('choose'); return operation(selection()); },
    recoverSelection() { calls.push('selection'); return operation(selection()); },
    create(token, json) { calls.push(['create', token, JSON.parse(json)]); return operation(receipt(request)); },
    receipt(json) { calls.push(['receipt', JSON.parse(json)]); return operation(receipt(request)); },
    image() { calls.push('image'); return operation(null); },
    release(token) { calls.push(['release', token]); return operation(); },
    acknowledge(token) { calls.push(['acknowledge', token]); return operation(); },
    ...overrides,
  };
  return { session: new PictureCreationSession(owner, port), calls, port, request };
}

test('Add automatically creates exactly once, blocks leaving and preserves committed receipts through acknowledgement', async () => {
  const { session, calls } = fixture();
  assert.equal(await session.add(requestId), true);
  assert.equal(calls.filter(call => Array.isArray(call) && call[0] === 'create').length, 1);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.view().receipt.id, 1);
  await assert.rejects(session.create(), /instead of replaying/);
  assert.equal(await session.acknowledge(), true);
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().receipt.didCommit, true);
  assert.equal(session.view().receipts.length, 1);
  session.view().receipt.committed.cells[1].text = 'detached';
  assert.equal(session.view().receipt.committed.cells[1].text, 'Q:\\No image grant\\');
});

test('native Cancel has no Create, while promise cancellation cannot dismiss an open modal or clear its guard', async () => {
  const choose = held(), { session, calls } = fixture({ choose: () => choose.promise });
  const pending = session.add(requestId);
  assert.equal(session.view().phase, 'choosing');
  assert.throws(() => session.cancel(), /promise cancellation cannot dismiss/);
  assert.equal(choose.cancellations(), 0);
  assert.equal(session.closeState().busy, true);
  choose.resolve(null);
  assert.equal(await pending, false);
  assert.equal(session.closeState().blocked, false);
  assert.equal(calls.length, 0);
});

test('lost chooser delivery recovers the same selection without opening another modal or inventing a request UUID', async () => {
  const { session, calls } = fixture({ choose: () => operation(Promise.reject(new Error('lost response'))) });
  assert.equal(await session.add(requestId), false);
  assert.equal(session.view().request, null);
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.recoverSelection(), true);
  assert.equal(session.view().request.requestId, requestId);
  assert.equal(await session.create(), true);
  assert.equal(calls.filter(call => call === 'choose').length, 0);
  assert.equal(calls.filter(call => Array.isArray(call) && call[0] === 'create').length, 1);
});

test('cancelled Create delivery retains exact pending identity, ignores late success and resolves only readonly history', async () => {
  const write = held(), { session, calls, request } = fixture({ create: () => write.promise });
  const pending = session.add(requestId);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(session.view().phase, 'creating');
  session.cancel();
  assert.equal(write.cancellations(), 1);
  assert.equal(session.closeState().blocked, true);
  write.resolve(receipt(request));
  assert.equal(await pending, false);
  assert.equal(session.view().receipt, null);
  await assert.rejects(session.create(), /instead of replaying/);
  assert.equal(await session.recoverReceipt(), true);
  assert.equal(session.view().receipt.requestId, requestId);
  assert.equal(calls.filter(call => Array.isArray(call) && call[0] === 'receipt').length, 1);
});

test('subscriber cancellation at creating notification invalidates generation before any write is dispatched', async () => {
  const { session, calls, port } = fixture();
  session.subscribe(() => {
    if (session.view().phase === 'creating') session.cancel();
  });
  assert.equal(await session.add(requestId), false);
  assert.equal(calls.filter(call => Array.isArray(call) && call[0] === 'create').length, 0);
  assert.equal(session.view().receipt, null);
  assert.equal(session.view().request.requestId, requestId);
  assert.equal(session.closeState().blocked, true);
  port.receipt = () => operation(null);
  assert.equal(await session.recoverReceipt(), true);
  assert.equal(await session.release(), true);
});

test('subscriber cancellation at read notifications dispatches no stale recovery or preview', async () => {
  for (const phase of ['recovering-selection', 'recovering-receipt', 'previewing']) {
    const { session, calls } = fixture();
    await session.add(requestId);
    const before = calls.length;
    session.subscribe(() => { if (session.view().phase === phase) session.cancel(); });
    const result = phase === 'recovering-selection' ? await session.recoverSelection()
      : phase === 'recovering-receipt' ? await session.recoverReceipt() : await session.preview();
    assert.equal(result, false);
    assert.equal(calls.length, before);
    assert.equal(session.view().receipt.didCommit, true);
    assert.equal(session.view().receipts.length, 1);
    assert.equal(session.closeState().blocked, true);
  }
});

test('preview failure and acknowledgement failure never erase a commit or allow close', async () => {
  const { session, port } = fixture({
    image: () => operation(Promise.reject(new Error('unsupported or replaced'))),
    acknowledge: () => operation(Promise.reject(new Error('lost acknowledgement'))),
  });
  await session.add(requestId);
  assert.equal(await session.preview(), false);
  assert.match(session.view().error, /metadata commit and receipt retained/);
  assert.equal(await session.acknowledge(), false);
  assert.equal(session.view().receipt.didCommit, true);
  assert.equal(session.closeState().blocked, true);
  port.recoverSelection = () => operation(null);
  assert.equal(await session.recoverSelection(), true);
  assert.equal(await session.acknowledge(), true);
  assert.equal(session.view().receipts.length, 1);
});

test('verified no-commit can explicitly release, but unknown/read-error outcomes and changed capability cannot', async () => {
  const { session, port } = fixture({ create: () => operation(Promise.reject(new Error('failed save'))),
    receipt: () => operation(null) });
  await session.add(requestId);
  await assert.rejects(session.release(), /Resolve durable history/);
  assert.equal(await session.recoverReceipt(), true);
  port.receipt = () => operation(Promise.reject(new Error('source drift')));
  assert.equal(await session.recoverReceipt(), false);
  await assert.rejects(session.release(), /Resolve durable history/);
  port.recoverSelection = () => operation({ ...selection(), token: 'b'.repeat(64) });
  assert.equal(await session.recoverSelection(), false);
  assert.equal(session.view().selection.token, selection().token);
  port.receipt = () => operation(null);
  await session.recoverReceipt();
  assert.equal(await session.release(), true);
  assert.equal(session.closeState().blocked, false);
});

test('a later cancelled Add retains earlier irreversible receipts and subscribers can remount without changing identity', async () => {
  const { session, port } = fixture();
  await session.add(requestId);
  await session.acknowledge();
  let observations = 0;
  const unsubscribe = session.subscribe(() => observations++);
  unsubscribe();
  port.choose = () => operation(null);
  assert.equal(await session.add(requestId.replace('0001', '0002')), false);
  const remount = session.subscribe(() => observations++);
  assert.equal(session.view().receipts[0].requestId, requestId);
  assert.equal(observations, 2);
  remount();
});
