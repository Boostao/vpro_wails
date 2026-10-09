const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { editor, owner, selection, receipt, requestId, cell } = require('./pictureCreationTestHelpers.cjs');

test('owned selection Cancel is explicit null, never missing or an empty success', () => {
  assert.equal(editor.pictureSelectionFromWire(null, owner), null);
  for (const wire of [undefined, '', {}, { ...selection(), selectedPath: '' }]) {
    assert.throws(() => editor.pictureSelectionFromWire(wire, owner));
  }
});

test('creation emits only the five strict Go request keys and detaches actual Svelte proxy primitives', () => {
  const source = client.proxy(selection());
  const request = editor.pictureCreationRequest(source, owner, requestId);
  assert.deepEqual(Object.keys(request).sort(), ['contextId', 'plotNumber', 'project', 'requestId', 'selectedPath']);
  assert.equal(JSON.parse(JSON.stringify(request)).selectedPath, 'Q:\\No image grant\\  Case #100% 😀.JpG');
  source.selectedPath = 'Q:\\changed.jpg';
  assert.equal(request.selectedPath, 'Q:\\No image grant\\  Case #100% 😀.JpG');
  assert.equal(request.token, undefined);
  assert.equal(request.windowId, undefined);
});

test('selection scope, native identity, token and raw literal limits are validated without repair', () => {
  for (const key of ['contextId', 'project', 'plotNumber']) {
    assert.throws(() => editor.pictureSelectionFromWire({ ...selection(), [key]: 'foreign' }, owner));
    assert.throws(() => editor.pictureSelectionFromWire(selection(), { ...owner, [key]: '' }));
  }
  for (const token of ['', 'a'.repeat(63), 'A'.repeat(64), 'x'.repeat(64)]) {
    assert.throws(() => editor.pictureSelectionFromWire({ ...selection(), token }, owner));
  }
  for (const windowId of [-1, 1.5, Number.MAX_SAFE_INTEGER + 1, '0', null]) {
    assert.throws(() => editor.pictureSelectionFromWire({ ...selection(), windowId }, owner));
  }
  for (const selectedPath of ['relative.jpg', 'Q:/a.jpg', '\\\\?\\Q:\\a.jpg', 'Q:\\a\0.jpg', 'Q:\\\ud800.jpg',
    'Q:\\' + '😀'.repeat(128), 'Q:\\' + 'd'.repeat(253) + '\\a.jpg']) {
    assert.throws(() => editor.pictureSelectionFromWire({ ...selection(), selectedPath }, owner));
  }
  assert.equal(editor.pictureSelectionFromWire({ ...selection(), selectedPath: 'Q:\\' + '😀'.repeat(127) + 'x' }, owner).selectedPath.length, 258);
  assert.equal(editor.pictureSelectionFromWire({ ...selection(), selectedPath: '\\\\server\\share\\ Case.jpg' }, owner).selectedPath, '\\\\server\\share\\ Case.jpg');
  for (const id of ['', requestId.toUpperCase(), requestId.replace('-4a74-', '-3a74-')]) {
    assert.throws(() => editor.pictureCreationRequest(selection(), owner, id));
  }
});

test('creation receipt proves all five native cells including NULL comment and literal path, not merely allocated ID', () => {
  const request = editor.pictureCreationRequest(selection(), owner, requestId), wire = receipt(request);
  const result = editor.pictureCreationReceiptFromWire(wire, request, owner);
  assert.equal(result.committed.cells[1].text, 'Q:\\No image grant\\');
  assert.equal(result.committed.cells[2].text, '  Case #100% 😀.JpG');
  assert.equal(result.committed.cells[4].storage, 'null');
  assert.equal(result.rowId, '9223372036854775807');
  for (let index = 0; index < 5; index++) {
    const changed = structuredClone(wire);
    changed.committed.cells[index] = cell('text', 'changed');
    assert.throws(() => editor.pictureCreationReceiptFromWire(changed, request, owner));
  }
  for (const key of ['requestId', 'contextId', 'project', 'plotNumber', 'selectedPath', 'rowId']) {
    assert.throws(() => editor.pictureCreationReceiptFromWire({ ...wire, [key]: 'changed' }, request, owner));
  }
});

test('receipt provenance, allocator bound, typed columns and durable history remain explicit', () => {
  const request = editor.pictureCreationRequest(selection(), owner, requestId), wire = receipt(request);
  for (const change of [
    { didCommit: false }, { replayed: null }, { id: 0 }, { id: -1 }, { id: 2147483648 }, { id: 1.5 },
    { actor: '' }, { actor: '\ud800' }, { actor: 'a'.repeat(256) }, { requestedSource: '' }, { source: '' },
    { ownedFiles: {} }, { ownedFiles: { pictures: null } }, { ownedFiles: { pictures: '\ud800' } },
    { auditStrength: -1 }, { auditStrength: 4 }, { auditStrength: 1.5 }, { historyId: '0' },
    { historyId: '9223372036854775808' }, { createdWhen: 'not a date' },
  ]) {
    assert.throws(() => editor.pictureCreationReceiptFromWire({ ...wire, ...change }, request, owner));
  }
  const wrongColumns = wire.columns.map(({ name, declaredType }) => ({ name, type: declaredType }));
  assert.throws(() => editor.pictureCreationReceiptFromWire({ ...wire, columns: wrongColumns }, request, owner));
  const longActor = editor.pictureCreationReceiptFromWire({ ...wire, actor: 'a'.repeat(255), replayed: true,
    historyId: '9223372036854775807' }, request, owner);
  assert.equal(longActor.actor.length, 255);
  assert.equal(longActor.replayed, true);
});
