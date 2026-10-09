const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { editor, fixture, owner, requestId, deletion, review, restorationReview, restoration, cell } = require('./pictureLifecycleTestHelpers.cjs');
const plain = value => JSON.parse(JSON.stringify(value));

test('deletion emits strict Go keys and detaches actual client-runtime proxy historical cells', () => {
  const source = client.proxy(review());
  const request = editor.pictureDeletionRequest(source, owner, '-7', requestId);
  assert.deepEqual(plain(request), fixture.deletion.request);
  source.records.rows[0].cells[2].text = 'changed';
  assert.equal(request.original.cells[2].text, '');
  assert.equal(request.original.cells[1].storage, 'null');
  assert.equal(request.original.cells[4].blobHex, 'ff00');
});

test('restoration preserves historical context and detaches actual proxy nested deletion provenance', () => {
  const current = { ...owner, contextId: 'later-owned' };
  const source = client.proxy(restorationReview(current));
  const request = editor.pictureRestorationRequest(source, current, fixture.restorationRequestId);
  assert.equal(request.contextId, 'later-owned');
  assert.equal(request.deletion.contextId, 'owned');
  assert.deepEqual(Object.keys(request).sort(), ['contextId', 'deletion', 'historyId', 'peers', 'requestId']);
  source.deletion.request.original.cells[2].text = 'changed';
  assert.equal(request.deletion.original.cells[2].text, '');
});

test('all signed32 source identities preserve exact signed64 rowid; NULL/noninteger do not infer zero', () => {
  for (const id of ['-2147483648', '-1', '0', '2147483647']) {
    const source = review();
    source.records.rows[0].cells[0] = cell('integer', id);
    assert.equal(editor.pictureDeletionRequest(source, owner, '-7', requestId).original.cells[0].integer, id);
  }
  for (const value of [cell(), cell('text', '0'), cell('real', 0), cell('integer', '2147483648')]) {
    const source = review(); source.records.rows[0].cells[0] = value;
    assert.throws(() => editor.pictureDeletionRequest(source, owner, '-7', requestId));
  }
  assert.throws(() => editor.pictureDeletionRequest(review(), owner, 'missing', requestId));
  assert.throws(() => editor.pictureDeletionRequest(review(), { ...owner, plotNumber: '00337 ' }, '-7', requestId));
});

test('strict shared Go deletion receipt validates complete original schema/provenance without repairing values', () => {
  assert.deepEqual(plain(editor.pictureDeletionReceiptFromWire(deletion(), fixture.deletion.request)), fixture.deletion);
  for (const key of ['id', 'historyId', 'source', 'requestedSource', 'actor', 'auditStrength', 'deletedWhen', 'didCommit', 'replayed']) {
    const changed = deletion(); changed[key] = null;
    assert.throws(() => editor.pictureDeletionReceiptFromWire(changed, fixture.deletion.request), key);
  }
  for (let index = 0; index < 5; index++) {
    const changed = deletion(); changed.request.original.cells[index] = cell('text', 'changed');
    assert.throws(() => editor.pictureDeletionReceiptFromWire(changed, fixture.deletion.request));
  }
  const changed = deletion(); changed.request.columns[0] = { name: 'ID', type: '' };
  assert.throws(() => editor.pictureDeletionReceiptFromWire(changed, fixture.deletion.request));
});

test('restoration receipt validates original deletion, current request, peers and independent actor/policy', () => {
  const current = { ...owner, contextId: 'later-owned' };
  const request = editor.pictureRestorationRequest(restorationReview(current), current, fixture.restorationRequestId);
  const receipt = restoration(request);
  assert.deepEqual(plain(editor.pictureRestorationReceiptFromWire(receipt, request)), plain(receipt));
  for (const key of ['historyId', 'contextId', 'requestId']) {
    const changed = structuredClone(receipt); changed.request[key] = 'other';
    assert.throws(() => editor.pictureRestorationReceiptFromWire(changed, request));
  }
  const changed = structuredClone(receipt); changed.deletion.request.original.cells[2].text = 'replacement';
  assert.throws(() => editor.pictureRestorationReceiptFromWire(changed, request));
  assert.throws(() => editor.pictureRestorationRequest(restorationReview(), owner, requestId));
});

test('history keeps explicit nullable restoration and refuses duplicates/foreign scope/consumed review corruption', () => {
  const history = { ...owner, events: [{ deletion: deletion(), restoration: null }] };
  assert.deepEqual(plain(editor.pictureLifecycleHistoryFromWire(history, owner)), history);
  assert.throws(() => editor.pictureLifecycleHistoryFromWire({ ...history, events: [...history.events, ...history.events] }, owner));
  assert.throws(() => editor.pictureLifecycleHistoryFromWire({ ...history, events: [{ deletion: deletion() }] }, owner));
  assert.throws(() => editor.pictureLifecycleHistoryFromWire({ ...history, plotNumber: 'foreign' }, owner));
  const restored = { ...owner, events: [{ deletion: deletion(), restoration: restoration() }] };
  assert.deepEqual(plain(editor.pictureLifecycleHistoryFromWire(restored, owner)), plain(restored));
  const occupied = restorationReview(); occupied.peers = [deletion().request.original];
  assert.throws(() => editor.pictureRestorationReviewFromWire(occupied, owner, '1'));
});
