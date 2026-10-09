const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { wire, editor, cell, receipt } = require('./twoPageVegetationTestHelpers.cjs');
const fixtures = require('./twoPageVegetationReceipts.json');
const outcomes = require('./twoPageVegetationOutcomes.json');
const sample = (index = 3) => structuredClone(fixtures[index]);
const owner = request => {
  const { contextId, project, plot, form } = request.original;
  return { contextId, project, plot, form };
};
const index = (request, column) => wire.vegetationColumnIndex(request.original, column);

test('actual Go vegetation receipts preserve every audit policy and detach real Svelte proxy history', () => {
  for (const [strength, source] of fixtures.slice(0, 4).entries()) {
    const request = source.history.Request;
    const result = receipt.twoPageVegetationReceiptFromWire(client.proxy(structuredClone(source)),
      client.proxy(structuredClone(request)), owner(request));
    assert.deepEqual(structuredClone(result), source);
    assert.equal(result.history.Audits.length, strength);
    assert.equal(result.history.Committed.rows[1].cells[0].integer, '42');
    assert.equal(result.history.Committed.rows[2].cells[0].storage, 'null');
    assert.doesNotThrow(() => structuredClone(result));
    assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, request, owner(request), true));
    assert.equal(receipt.twoPageVegetationReceiptFromWire({ ...source, didCommit: false, replayed: true },
      request, owner(request), true).replayed, true);
  }
});

test('legacy binary64 receipts recover exactly without narrowing previously committed assignments', () => {
  for (const legacy of [0.1, 99.999999, 1e-46, -1e-46]) {
    const source = sample(0), request = source.history.Request;
    request.edits[0].value = cell('real', legacy);
    source.history.Committed.rows[0].cells[index(request, request.edits[0].column)] = cell('real', legacy);
    source.didCommit = false;
    source.replayed = true;
    const accepted = receipt.twoPageVegetationReceiptFromWire(client.proxy(source),
      client.proxy(request), owner(request), true);
    assert.equal(Object.is(accepted.history.Request.edits[0].value.real, legacy), true);
    assert.equal(Object.is(accepted.history.Committed.rows[0].cells[index(request, request.edits[0].column)].real, legacy), true);
    assert.notEqual(Math.fround(legacy), legacy);
  }
});

test('last Cover6 removal updates source membership without deleting its physical row or Collected change', () => {
  const source = sample(4), request = source.history.Request;
  const accepted = receipt.twoPageVegetationReceiptFromWire(source, request, owner(request));
  const committed = receipt.twoPageVegetationCommittedOriginal(accepted, owner(request));
  assert.equal(request.original.sourceRowIds.includes('-7'), true);
  assert.equal(committed.sourceRowIds.includes('-7'), false);
  assert.equal(committed.records.rows.length, request.original.records.rows.length);
  assert.equal(wire.vegetationSourceCell(committed, '-7', 'Collected').text, 'C');
});

test('vegetation receipt rejects altered source plans complete peers schemas flags dates and audit provenance', () => {
  for (const mutate of [
    value => value.didCommit = false,
    value => value.replayed = true,
    value => value.extra = null,
    value => value.history.Request.extra = null,
    value => value.history.Request.requestId = 'b0cbfa00-1234-4123-8123-000000000002',
    value => value.history.Request.original.contextId = 'other',
    value => value.history.Request.edits[0].value = cell('real', 6),
    value => value.history.Request.edits[0].expected = cell('real', 1),
    value => value.history.Request.edits.reverse(),
    value => value.history.Request.edits.pop(),
    value => value.history.Committed.columns[0].declaredType = 'DIFFERENT_SOURCE_TYPE',
    value => value.history.Committed.rows[1].cells[2] = cell('text', 'replacement'),
    value => value.history.Committed.rows[2].cells[0] = cell('integer', '0'),
    value => value.history.Committed.rows[0].rowId = '-8',
    value => value.history.Committed.rows.reverse(),
    value => value.history.Committed.rows.pop(),
    value => value.history.Actor = '',
    value => value.history.Actor = '\ud800',
    value => value.history.Actor = 'x'.repeat(101),
    value => value.history.AuditStrength = 4,
    value => value.history.EditWhen = '2026-02-31 12:34:56',
    value => value.history.Audits.pop(),
    value => value.history.Audits[1].rowId = value.history.Audits[0].rowId,
    value => value.history.Audits[1].rowId = '9223372036854775808',
    value => value.history.Audits[0].id = 0,
    value => value.history.Audits[0].beforeEdit = '00',
    value => value.history.Audits[0].afterEdit = '05',
    value => value.history.Audits[0].editField = 'Species',
    value => value.history.Audits[0].table = '_Humus',
    value => value.history.Audits[0].restore = true,
    value => value.history.Audits[0].flag = true,
    value => value.history.Audits[0].user = 'other',
    value => value.history.Audits[0].extra = null,
  ]) {
    const source = sample(), retained = structuredClone(source.history.Request);
    mutate(source);
    assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, retained, owner(retained)), mutate.toString());
  }
});

test('typed INTEGER-to-REAL change is retained without recreating unavailable raw input', () => {
  const source = sample(1), request = source.history.Request;
  request.original.records.rows[0].cells[index(request, 'Cover1')] = cell('integer', '5');
  request.edits[0].expected = cell('integer', '5');
  source.history.Audits[0].beforeEdit = '5';
  const result = receipt.twoPageVegetationReceiptFromWire(source, request, owner(request));
  assert.equal(result.history.Request.edits[0].expected.storage, 'integer');
  assert.equal(result.history.Request.edits[0].value.storage, 'real');
  const blob = sample(1);
  blob.history.Request.original.records.rows[0].cells[index(blob.history.Request, 'Cover1')] = cell('blob', 'ff00');
  blob.history.Request.edits[0].expected = cell('blob', 'ff00');
  blob.history.Audits[0].beforeEdit = '[255 0]';
  assert.equal(receipt.twoPageVegetationReceiptFromWire(blob, blob.history.Request, owner(blob.history.Request))
    .history.Audits[0].beforeEdit, '[255 0]');
  blob.history.Audits[0].beforeEdit = 'ff00';
  assert.throws(() => receipt.twoPageVegetationReceiptFromWire(blob, blob.history.Request, owner(blob.history.Request)));
});

test('REAL negative-zero peer witnesses and canonical request bytes cannot be repaired by receipt validation', () => {
  const source = sample(1), request = source.history.Request, column = index(request, 'Cover3');
  request.original.records.rows[1].cells[column] = cell('real', -0);
  source.history.Committed.rows[1].cells[column] = cell('real', -0);
  const result = receipt.twoPageVegetationReceiptFromWire(source, request, owner(request));
  assert.equal(Object.is(result.history.Committed.rows[1].cells[column].real, -0), true);
  assert.notEqual(wire.serializeTwoPageVegetationRequest(request), JSON.stringify(request));
  source.history.Committed.rows[1].cells[column] = cell('real', 0);
  assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, request, owner(request)));
  source.history.Committed.rows[1].cells[column] = cell('real', -0);
  source.history.Request = JSON.parse(JSON.stringify(request));
  assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, request, owner(request)));
});

test('negative-to-positive zero remains a typed change but has no source audit at every strength', () => {
  for (const strength of [0, 1, 2, 3]) {
    const source = sample(strength), request = source.history.Request, column = index(request, 'Cover1');
    request.original.records.rows[0].cells[column] = cell('real', -0);
    const draft = editor.stageTwoPageVegetation(request.original, '-7', 'Cover1', '+0', false);
    const retained = editor.twoPageVegetationRequest(request.original, owner(request), { zero: draft }, request.requestId);
    source.history.Request = retained;
    source.history.Committed = structuredClone(retained.original.records);
    source.history.Committed.rows[0].cells[column] = cell('real', 0);
    source.history.Audits = [];
    const accepted = receipt.twoPageVegetationReceiptFromWire(source, retained, owner(retained));
    assert.equal(Object.is(accepted.history.Request.edits[0].expected.real, -0), true);
    assert.equal(Object.is(accepted.history.Committed.rows[0].cells[column].real, 0), true);
    assert.equal(accepted.history.Audits.length, 0);
    assert.doesNotThrow(() => receipt.twoPageVegetationReceiptFromWire({ ...source, didCommit: false, replayed: true },
      retained, owner(retained), true));
    source.history.Committed.rows[0].cells[column] = cell('real', -0);
    assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, retained, owner(retained)));
  }
});

test('outcomes require exact attempted bytes sealed original-context absence and readonly committed receipts', () => {
  const source = sample(), request = source.history.Request, scoped = owner(request);
  const requestJSON = wire.serializeTwoPageVegetationRequest(request);
  const outcome = (status = 'notCommitted') => structuredClone(outcomes.find(value => value.status === status));
  for (const status of ['running', 'unknown', 'notCommitted', 'committed']) {
    assert.equal(receipt.twoPageVegetationOutcomeFromWire(client.proxy(outcome(status)), request, requestJSON, scoped).status, status);
  }
  const rotated = { ...scoped, contextId: 'rotated' };
  assert.equal(receipt.twoPageVegetationOutcomeFromWire({ ...outcome('unknown'), contextId: rotated.contextId },
    request, requestJSON, rotated).status, 'unknown');
  assert.throws(() => receipt.twoPageVegetationOutcomeFromWire({ ...outcome(), contextId: rotated.contextId },
    request, requestJSON, rotated));
  for (const mutate of [
    value => value.requestJSON += ' ',
    value => value.requestId = 'other',
    value => value.contextId = 'other',
    value => value.status = 'absent',
    value => value.reason = '',
    value => value.reason = '\ud800',
    value => value.receipt = sample(),
    value => value.extra = null,
  ]) {
    const value = outcome(); mutate(value);
    assert.throws(() => receipt.twoPageVegetationOutcomeFromWire(value, request, requestJSON, scoped));
  }
  assert.throws(() => receipt.twoPageVegetationOutcomeFromWire({ ...outcome('committed'), receipt: sample() },
    request, requestJSON, scoped));
  assert.throws(() => receipt.twoPageVegetationOutcomeFromWire(outcome(), request, requestJSON + ' ', scoped));
});
