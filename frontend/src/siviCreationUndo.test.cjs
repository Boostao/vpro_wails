const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { cell, metadata, restoration, transport } = require('./siviParentTestHelpers.cjs');
const defaults = require('../../resources/sivi-new-row-defaults.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ids = loadTypeScript('siviRequestId.ts'), dates = loadTypeScript('siviDateTimestamp.ts');
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const covers = loadTypeScript('siviCoverEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'), './siviHeightEditor': height,
});
const creation = loadTypeScript('siviCreationReceipt.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './siviParentTransport': transport, './siviDateTimestamp': dates,
});
const deletionEditor = loadTypeScript('siviDeletionEditor.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults, './qualityEditor': quality,
  './siviParentTransport': transport, './siviRequestId': ids,
});
const deletionReceipt = loadTypeScript('siviDeletionReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration,
  './qualityEditor': quality, './siviDateTimestamp': dates, './siviDeletionEditor': deletionEditor,
});
const historical = loadTypeScript('siviDeletionRestoration.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviDateTimestamp': dates,
  './siviParentTransport': transport, './siviDeletionEditor': deletionEditor,
  './siviDeletionReceipt': deletionReceipt, './siviRequestId': ids,
});
const undo = loadTypeScript('siviCreationUndo.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviDateTimestamp': dates,
  './siviParentTransport': transport, './siviCreationReceipt': creation, './siviCoverEditor': covers,
  './siviDeletionReceipt': deletionReceipt, './siviDeletionRestoration': historical, './siviRequestId': ids,
});
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const historyId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', undoId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
const copy = value => structuredClone(value);
function review(strength = 3, value = 0, empty = false) {
  const request = { ...owner, requestId: historyId, form: 'SubVegA-SIVI', species: 'TREE',
    covers: empty ? [] : [{ column: 'Cover1', value: cell('real', value) }] };
  const columns = ['ID', ...defaults.initialNullColumns, 'Flag'].map(name => ({ name, declaredType: name === 'Species' ? 'TEXT' : 'REAL' }));
  const original = { rowId: '', cells: columns.map(column => column.name === 'Flag' ? cell('integer', '0') : cell()) };
  const committed = { rowId: '9223372036854775806', cells: columns.map(column =>
    column.name === 'ID' ? cell('integer', '1') : column.name === 'Flag' ? cell('integer', '0') :
      column.name === 'Species' ? cell('text', request.species) : column.name === 'PlotNumber' ? cell('text', owner.plot) :
        copy(request.covers.find(cover => cover.column === column.name)?.value ?? cell())) };
  const created = { ...owner, requestId: historyId, historyId, form: request.form, rowId: committed.rowId, id: 1,
    actor: ' literal creator ', auditStrength: strength, editWhen: '2026-10-08 10:11:12',
    columns, original, committed, covers: copy(request.covers), request, didCommit: false, replayed: true };
  const auditColumns = ['Project', 'User', 'PlotNumber', 'Table', 'EditField', 'EditWhen',
    'BeforeEdit', 'AfterEdit', 'Restore', 'Flag', 'ID'].map(name => ({ name, declaredType: 'TEXT' }));
  const additions = strength < 2 ? [] : [
    ['Species', 'TREE'], ...request.covers.map(cover => ['Cover1', deletionReceipt.siviSourceAuditText(columns[0], cover.value)]),
  ];
  const auditsBefore = additions.map(([field, after], i) => ({ rowId: String(i + 1), cells: auditColumns.map(column => {
    const values = { Project: owner.project, User: created.actor, PlotNumber: owner.plot, Table: 'sAmPlE_vEg',
      EditField: field, EditWhen: created.editWhen, AfterEdit: after };
    return column.name === 'ID' ? cell('integer', '1') : ['Restore', 'Flag'].includes(column.name) ? cell('integer', '0') :
      column.name === 'BeforeEdit' ? cell() : cell('text', values[column.name]);
  }) }));
  return { ...owner, historyId, expected: 'a'.repeat(64), form: request.form, rowId: committed.rowId, id: 1,
    columns: copy(columns), committed: copy(committed), creation: created, auditColumns, auditsBefore };
}
function result(reviewed, request, lookup = false) {
  const after = request.action === 'prune' ? [] : copy(reviewed.auditsBefore);
  for (const row of after) row.cells[reviewed.auditColumns.findIndex(column => column.name === 'Restore')] = cell('integer', '-1');
  return { ...copy(reviewed), requestId: request.requestId, undoId: request.requestId, action: request.action,
    actor: ' undo actor ', auditStrength: 1, editWhen: '2026-10-08 11:12:13', request: copy(request),
    auditsAfter: after, cancelled: false, removedRows: 1, prunedAuditRows: request.action === 'prune' ? reviewed.auditsBefore.length : 0,
    didCommit: !lookup, replayed: lookup };
}

test('13-property creation review, seven-field request and26-property Undo result preserve exact typed history', () => {
  for (const strength of [0, 1, 2, 3]) for (const action of ['retain', 'prune']) {
    const wire = review(strength);
    const accepted = undo.siviCreationUndoReviewFromWire(wire, owner, historyId);
    const request = undo.siviCreationUndoRequest(accepted, owner, undoId, action, true);
    assert.equal(Object.keys(accepted).length, 13);
    assert.equal(Object.keys(request).length, 7);
    assert.equal(request.confirmed, undefined);
    for (const lookup of [false, true]) {
      const receipt = result(accepted, request, lookup);
      assert.equal(Object.keys(receipt).length, 26);
      const parsed = undo.siviCreationUndoReceiptFromWire(receipt, accepted, request, owner, lookup ? 'lookup' : 'undo');
      assert.equal(parsed.committed.cells.length, 44);
      assert.equal(parsed.removedRows, 1);
      assert.equal(parsed.didCommit, !lookup);
      assert.equal(parsed.prunedAuditRows, action === 'prune' ? accepted.auditsBefore.length : 0);
    }
  }
});
test('source audit formatting shares Go numeric thresholds and signed zero; empty-cover history does not invent membership', () => {
  for (const value of [-0, -0.00001, 0.0001, 99.999]) {
    assert.doesNotThrow(() => undo.siviCreationUndoReviewFromWire(review(3, value), owner, historyId));
  }
  const wire = review(3, 0, true);
  const accepted = undo.siviCreationUndoReviewFromWire(wire, owner, historyId);
  assert.equal(accepted.creation.covers.length, 0);
  assert.equal(accepted.auditsBefore.length, 1);
});
test('creation reviews refuse foreign or malformed authority, extra nested fields and incomplete exact audit evidence', () => {
  for (const mutate of [
    value => value.contextId = 'foreign', value => value.project = 'sample', value => value.plot = 'P ',
    value => value.expected = 'A'.repeat(64), value => value.id = 2, value => value.rowId = '1',
    value => value.creation.didCommit = true, value => value.creation.request.requestId = undoId,
    value => value.creation.request.species = '\ud800', value => value.creation.request.covers.push(copy(value.creation.request.covers[0])),
    value => value.creation.request.covers[0].column = 'Height1',
    value => value.creation.original.extra = true, value => value.creation.columns[0].extra = true,
    value => value.committed.cells[0].extra = true, value => value.creation.covers[0].value.extra = true,
    value => value.columns.reverse(), value => value.committed.cells[0].integer = '2',
    value => value.auditsBefore.pop(), value => value.auditsBefore.push(copy(value.auditsBefore[0])),
    value => value.auditsBefore[0].cells[3].text = 'Other_Veg',
    value => value.auditsBefore[0].cells[6] = cell('text', ''),
    value => value.auditsBefore[0].cells[7].text = 'OTHER',
    value => value.auditsBefore[0].cells[8].integer = '-1',
    value => value.auditsBefore[0].cells[9].integer = '1',
  ]) {
    const wire = review();
    mutate(wire);
    assert.throws(() => undo.siviCreationUndoReviewFromWire(wire, owner, historyId));
  }
  const accepted = undo.siviCreationUndoReviewFromWire(review(), owner, historyId);
  for (const [id, action, confirmed] of [[historyId, 'retain', true], ['1', 'retain', true],
    [undoId, 'cancel', true], [undoId, 'retain', false]]) {
    assert.throws(() => undo.siviCreationUndoRequest(accepted, owner, id, action, confirmed));
  }
});
test('receipts reject changed reviewed creation evidence even with zero audits and never infer missing commit authority', () => {
  for (const strength of [0, 3]) for (const action of ['retain', 'prune']) {
    const reviewed = undo.siviCreationUndoReviewFromWire(review(strength), owner, historyId);
    const request = undo.siviCreationUndoRequest(reviewed, owner, undoId, action, true);
    for (const mutate of [
      value => value.undoId = historyId, value => value.expected = 'b'.repeat(64),
      value => value.request.confirmed = true, value => value.removedRows = 0, value => value.cancelled = true,
      value => value.didCommit = value.replayed = true, value => value.prunedAuditRows++,
      value => value.creation.actor = 'changed', value => value.creation.request.contextId = 'changed',
      value => value.creation.editWhen = '2026-10-08 10:11:13',
      value => value.creation.auditStrength = strength === 0 ? 1 : 2,
      value => value.auditColumns.reverse(),
      value => { if (action === 'prune') value.auditsAfter = reviewed.auditsBefore.length ? copy(reviewed.auditsBefore)
          : [{ rowId: '1', cells: value.auditColumns.map(() => cell()) }];
        else if (strength) value.auditsAfter[0].cells[8].integer = '1';
        else value.auditsAfter.push({ rowId: '1', cells: value.auditColumns.map(() => cell()) }); },
    ]) {
      const receipt = result(reviewed, request);
      mutate(receipt);
      assert.throws(() => undo.siviCreationUndoReceiptFromWire(receipt, reviewed, request, owner),
        `${strength}/${action}: ${String(mutate)}`);
    }
    assert.throws(() => undo.siviCreationUndoReceiptFromWire(result(reviewed, request), reviewed, request, owner, 'lookup'));
  }
});
