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
function loadSessions() {
  const historyProtocol = loadTypeScript('siviCreationUndoHistory.ts', {
    './siviRequestId': ids, './qualityEditor': quality,
    './siviDeletionHistory': loadTypeScript('siviDeletionHistory.ts', {
      './projectMetadataRestore': restoration, './qualityEditor': quality,
      './siviDateTimestamp': dates, './siviRequestId': ids,
    }),
  });
  return loadTypeScript('siviCreationUndoSession.ts', {
    './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
    './siviCreationUndo': undo, './siviCreationUndoHistory': historyProtocol,
  });
}
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const historyId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', requestId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
const copy = value => structuredClone(value);
function review() {
  const columns = ['ID', ...defaults.initialNullColumns, 'Flag'].map(name => ({ name, declaredType: 'REAL' }));
  const original = { rowId: '', cells: columns.map(column => column.name === 'Flag' ? cell('integer', '0') : cell()) };
  const committed = { rowId: '9223372036854775806', cells: columns.map(column =>
    column.name === 'ID' ? cell('integer', '1') : column.name === 'Flag' ? cell('integer', '0') :
      column.name === 'Species' ? cell('text', 'TREE') : column.name === 'PlotNumber' ? cell('text', owner.plot) :
        column.name === 'Cover1' ? cell('real', 0) : cell()) };
  const request = { ...owner, requestId: historyId, form: 'SubVegA-SIVI', species: 'TREE',
    covers: [{ column: 'Cover1', value: cell('real', 0) }] };
  const creation = { ...owner, requestId: historyId, historyId, form: request.form, rowId: committed.rowId, id: 1,
    actor: ' literal creator ', auditStrength: 3, editWhen: '2026-10-08 10:11:12',
    columns, original, committed, covers: copy(request.covers), request, didCommit: false, replayed: true };
  const auditColumns = ['Project', 'User', 'PlotNumber', 'Table', 'EditField', 'EditWhen',
    'BeforeEdit', 'AfterEdit', 'Restore', 'Flag', 'ID'].map(name => ({ name, declaredType: 'TEXT' }));
  const auditsBefore = [['Species', 'TREE'], ['Cover1', '0']].map(([field, after], i) => ({
    rowId: String(i + 1), cells: auditColumns.map(column => {
      const values = { Project: owner.project, User: creation.actor, PlotNumber: owner.plot, Table: 'sAmPlE_vEg',
        EditField: field, EditWhen: creation.editWhen, AfterEdit: after };
      return column.name === 'ID' ? cell('integer', '1') : ['Restore', 'Flag'].includes(column.name) ? cell('integer', '0') :
        column.name === 'BeforeEdit' ? cell() : cell('text', values[column.name]);
    }),
  }));
  return { ...owner, historyId, expected: 'a'.repeat(64), form: request.form, rowId: committed.rowId, id: 1,
    columns: copy(columns), committed: copy(committed), creation, auditColumns, auditsBefore };
}
function history(consumed = false) {
  const reviewed = review();
  return { ...owner, historyPresent: true, events: [{ historyId, form: reviewed.form, rowId: reviewed.rowId,
    id: reviewed.id, species: 'TREE', actor: reviewed.creation.actor, editWhen: reviewed.creation.editWhen,
    undone: consumed, consumed, reviewAvailable: !consumed,
    unavailableReason: consumed ? 'This creation history has already been consumed.' : null }] };
}
function result(reviewed, request, lookup = false) {
  const after = request.action === 'prune' ? [] : copy(reviewed.auditsBefore);
  for (const row of after) row.cells[reviewed.auditColumns.findIndex(column => column.name === 'Restore')] = cell('integer', '-1');
  return { ...copy(reviewed), requestId: request.requestId, undoId: request.requestId, action: request.action,
    actor: ' undo actor ', auditStrength: 1, editWhen: '2026-10-08 11:12:13', request: copy(request),
    auditsAfter: after, cancelled: false, removedRows: 1, prunedAuditRows: request.action === 'prune' ? reviewed.auditsBefore.length : 0,
    didCommit: !lookup, replayed: lookup };
}
const read = value => Object.assign(Promise.resolve(value), { cancel() {} });
function deferred() {
  let resolve, reject, cancelled = 0;
  const promise = Object.assign(new Promise((yes, no) => { resolve = yes; reject = no; }), { cancel() { cancelled++; } });
  return { promise, resolve, reject, get cancelled() { return cancelled; } };
}
module.exports = { cell, metadata, owner, historyId, requestId, review, history, result, read, deferred, copy,
  get sessions() { return loadSessions(); } };
