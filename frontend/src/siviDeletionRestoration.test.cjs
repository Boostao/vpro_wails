const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { cell, metadata, restoration, transport } = require('./siviParentTestHelpers.cjs');
const { owner, token, original, receipt: deletionReceipt } = require('./siviDeletionTestHelpers.cjs');
const defaults = require('../../resources/sivi-new-row-defaults.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ids = loadTypeScript('siviRequestId.ts'), dates = loadTypeScript('siviDateTimestamp.ts');
const editor = loadTypeScript('siviDeletionEditor.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults, './qualityEditor': quality,
  './siviParentTransport': transport, './siviRequestId': ids,
});
const receipts = loadTypeScript('siviDeletionReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration,
  './qualityEditor': quality, './siviDateTimestamp': dates, './siviDeletionEditor': editor,
});
const restore = loadTypeScript('siviDeletionRestoration.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviDateTimestamp': dates,
  './siviParentTransport': transport, './siviDeletionEditor': editor,
  './siviDeletionReceipt': receipts, './siviRequestId': ids,
});
const histories = loadTypeScript('siviDeletionHistory.ts', {
  './projectMetadataRestore': restoration, './qualityEditor': quality,
  './siviDateTimestamp': dates, './siviRequestId': ids,
});
const sessions = loadTypeScript('siviDeletionRestorationSession.ts', {
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
  './siviDeletionRestoration': restore,
  './siviDeletionHistory': histories,
});
const restoreId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
const copy = value => structuredClone(value);
function review(strength = 3) {
  const plan = editor.siviDeletionRequest(original(), owner, token, true);
  const deletion = deletionReceipt(plan, true, strength);
  const auditColumns = ['Project', 'User', 'PlotNumber', 'Table', 'EditField', 'EditWhen',
    'BeforeEdit', 'AfterEdit', 'Restore', 'Flag', 'ID'].map(name => ({ name, declaredType: 'TEXT' }));
  return { ...owner, historyId: token, expected: 'a'.repeat(64), form: plan.form,
    rowId: plan.original.rowId, id: deletion.id, columns: copy(plan.columns), original: copy(plan.original),
    deletion, auditColumns, auditsBefore: deletion.audits.map(audit => ({
      rowId: audit.rowId, cells: auditColumns.map(column => column.name === 'ID' ? cell('integer', String(audit.id)) :
        ['Restore', 'Flag'].includes(column.name) ? cell('integer', '0') : column.name === 'AfterEdit' ? cell() :
        cell('text', column.name === 'Project' ? audit.project : column.name === 'User' ? audit.user :
          column.name === 'PlotNumber' ? audit.plotNumber : column.name === 'Table' ? 'sAmPlE_vEg' :
          column.name === 'EditField' ? audit.editField : column.name === 'EditWhen' ? audit.editWhen : audit.beforeEdit)),
    })) };
}
function result(plan, reviewed, lookup = false) {
  const after = plan.action === 'prune' ? [] : copy(reviewed.auditsBefore);
  const index = reviewed.auditColumns.findIndex(column => column.name === 'Restore');
  for (const row of after) row.cells[index] = cell('integer', '-1');
  return { ...copy(reviewed), requestId: plan.requestId, restorationId: plan.requestId, action: plan.action,
    actor: ' restoring user ', auditStrength: 2, editWhen: '2026-10-07 19:31:54',
    restored: copy(reviewed.original), request: copy(plan), auditsAfter: after, cancelled: false,
    restoredRows: 1, prunedAuditRows: plan.action === 'prune' ? after.length + reviewed.auditsBefore.length : 0,
    didCommit: !lookup, replayed: lookup };
}
function resolved(value) { const promise = Promise.resolve(value); promise.cancel = () => {}; return promise; }
function history() {
  return { ...owner, historyPresent: true, events: [{
    historyId: token, form: 'SubVegA-SIVI_BC', rowId: original().original.rowId, id: -2147483648,
    species: '  historical overlength  ', actor: ' literal user ', editWhen: '2026-10-07 18:30:53',
    restored: false, consumed: false,
  }] };
}
function held() {
  let release;
  const promise = new Promise(resolve => { release = resolve; }); promise.cancel = () => {};
  return { promise, release };
}
async function fixture() {
  const calls = { restore: 0, receipt: 0, requests: [] }, reviewed = review();
  const port = {
    history: () => resolved(history()),
    review: () => resolved(reviewed),
    restore: json => { calls.restore++; calls.requests.push(json); return resolved(result(JSON.parse(json), reviewed)); },
    receipt: json => { calls.receipt++; calls.requests.push(json); return resolved(result(JSON.parse(json), reviewed, true)); },
  };
  const current = new sessions.SIVIDeletionRestorationSession(owner, port);
  assert.equal(await current.loadHistory(), true);
  assert.equal(await current.review(token), true);
  return { current, port, calls, reviewed };
}

test('13-property review and27-property receipt bind full44 raw cells, SHA256 CAS and explicit source audit aliases', () => {
  for (const strength of [0, 1, 2, 3]) for (const action of ['retain', 'prune']) {
    const wire = review(strength);
    assert.equal(Object.keys(wire).length, 13);
    const accepted = restore.siviDeletionRestorationReviewFromWire(wire, owner, token);
    const plan = restore.siviDeletionRestorationRequest(accepted, owner, restoreId, action, true);
    assert.equal(Object.keys(plan).length, 7);
    const receipt = result(plan, accepted);
    assert.equal(Object.keys(receipt).length, 27);
    const verified = restore.siviDeletionRestorationReceiptFromWire(receipt, accepted, plan, owner);
    assert.equal(verified.restored.cells.length, 44);
    assert.equal(verified.restored.cells[42].integer, '1');
    assert.equal(verified.prunedAuditRows, action === 'prune' ? accepted.auditsBefore.length : 0);
    assert.equal(verified.auditsAfter.length, action === 'prune' ? 0 : accepted.auditsBefore.length);
  }
});

test('review and confirmation refuse incomplete originals, wrong owners, foreign audits and malformed review tokens', () => {
  for (const mutate of [
    value => value.contextId = 'foreign', value => value.historyId = restoreId,
    value => value.expected = 'A'.repeat(64), value => value.expected = 'a'.repeat(63),
    value => value.expected += '\n', value => value.original.cells[42].integer = '-1',
    value => value.columns.reverse(), value => value.deletion.historyId = '1',
    value => value.auditsBefore.pop(), value => value.auditsBefore[0].cells[3].text = 'foreign_Veg',
    value => value.auditsBefore[0].cells[1].text = 'foreign user', value => value.auditsBefore[0].cells[8].integer = '-1',
    value => value.auditsBefore[0].rowId = '1', value => value.auditColumns[0].name = 'project',
    value => value.foreign = true,
  ]) {
    const wire = review(); mutate(wire);
    assert.throws(() => restore.siviDeletionRestorationReviewFromWire(wire, owner, token));
  }
  const accepted = restore.siviDeletionRestorationReviewFromWire(review(), owner, token);
  for (const [id, action, confirmed] of [[token, 'retain', true], [restoreId, 'cancel', true],
    [restoreId, 'retain', false], [restoreId.toUpperCase(), 'retain', true]]) {
    assert.throws(() => restore.siviDeletionRestorationRequest(accepted, owner, id, action, confirmed));
  }
});

test('receipt refuses identity changes, repaired cells, mismatched action/token and partial audit restoration', () => {
  const accepted = restore.siviDeletionRestorationReviewFromWire(review(), owner, token);
  const plan = restore.siviDeletionRestorationRequest(accepted, owner, restoreId, 'retain', true);
  for (const mutate of [
    value => value.restorationId = token, value => value.expected = 'b'.repeat(64),
    value => value.request.expected = 'b'.repeat(64), value => value.restored.rowId = '1',
    value => value.restored.cells[42].integer = '-1', value => value.action = 'prune',
    value => value.cancelled = true, value => value.restoredRows = 44,
    value => value.prunedAuditRows = 1, value => value.auditsAfter.pop(),
    value => value.auditsAfter[0].cells[8].integer = '1', value => value.auditsAfter[0].cells[0].text = 'foreign',
    value => value.actor = '\ud800', value => value.editWhen = '2026-02-30 19:31:54',
    value => value.editWhen = '2026-10-07T19:31:54Z', value => value.replayed = true,
    value => value.request.foreign = true,
  ]) {
    const wire = result(plan, accepted); mutate(wire);
    assert.throws(() => restore.siviDeletionRestorationReceiptFromWire(wire, accepted, plan, owner));
  }
  assert.throws(() => restore.siviDeletionRestorationReceiptFromWire(result(plan, accepted), accepted, plan, owner, 'lookup'));
});

test('explicitly allowed alias changes before Restore do not replace historical literals or schema', () => {
  const accepted = restore.siviDeletionRestorationReviewFromWire(review(), owner, token);
  const plan = restore.siviDeletionRestorationRequest(accepted, owner, restoreId, 'retain', true);
  const wire = result(plan, accepted);
  for (const row of [...wire.auditsBefore, ...wire.auditsAfter]) row.cells[3].text = '_vEg';
  assert.equal(restore.siviDeletionRestorationReceiptFromWire(wire, accepted, plan, owner).auditsAfter[0].cells[3].text, '_vEg');
  assert.equal(accepted.auditsBefore[0].cells[3].text, 'sAmPlE_vEg');
});

test('retained restoration review requires separate action/confirmation and survives cloned remount state', async () => {
  const { current, calls } = await fixture();
  await assert.rejects(current.save(restoreId), /Choose/);
  current.choose('retain'); current.confirm(true);
  current.choose('prune');
  assert.equal(current.view().confirmed, false);
  current.confirm(true);
  const remount = current.view(); remount.review.original.cells[1].text = 'changed';
  assert.equal(current.view().review.original.cells[1].text, '  historical overlength  ');
  assert.equal(await current.save(restoreId), true);
  assert.equal(calls.restore, 1);
  assert.equal(current.view().revision, 1);
  assert.equal(current.closeState().unsaved, false);
  assert.equal(current.view().receipt.restored.cells[42].integer, '1');
});

test('lost restoration acknowledgement and missing history retain exact review/CAS/UUID without replay', async () => {
  const { current, calls, port, reviewed } = await fixture();
  const perform = port.restore; port.restore = json => { perform(json); return resolved({}); };
  current.choose('retain'); current.confirm(true);
  assert.equal(await current.save(restoreId), false);
  assert.throws(() => current.discard(), /Resolve/);
  assert.throws(() => current.choose('prune'), /Resolve/);
  await assert.rejects(current.save(restoreId), /Resolve/);
  port.receipt = () => resolved(null);
  assert.equal(await current.resolve(), false);
  assert.match(current.view().error, /Missing restoration history is unresolved/);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(result(JSON.parse(json), reviewed, true)); };
  assert.equal(await current.resolve(), true);
  assert.equal(calls.restore, 1);
  assert.equal(new Set(calls.requests).size, 1);
});

test('cancelled acknowledgement ignores late restored-row success and retains unknown authority for readonly lookup', async () => {
  const { current, calls, port, reviewed } = await fixture(), pending = held();
  port.restore = json => { calls.restore++; calls.requests.push(json); return pending.promise; };
  current.choose('retain'); current.confirm(true);
  const save = current.save(restoreId);
  current.cancel(); pending.release(result(JSON.parse(calls.requests[0]), reviewed));
  assert.equal(await save, false);
  assert.equal(current.view().revision, 0);
  assert.equal(current.closeState().blocked, true);
  assert.equal(await current.resolve(), true);
  assert.equal(calls.restore, 1);
});

test('historical discovery distinguishes missing/empty stores, raw text and consumed facts rather than current row eligibility', () => {
  for (const present of [false, true]) {
    const wire = { ...owner, historyPresent: present, events: [] };
    assert.equal(histories.siviDeletionHistoryFromWire(wire, owner).historyPresent, present);
  }
  const wire = history();
  wire.events[0].restored = true; wire.events[0].consumed = true;
  const accepted = histories.siviDeletionHistoryFromWire(wire, owner);
  assert.equal(accepted.events[0].species, '  historical overlength  ');
  assert.equal(accepted.events[0].consumed, true);
  for (const mutate of [
    value => value.contextId = 'foreign', value => value.events = null, value => value.historyPresent = false,
    value => value.events.push(copy(value.events[0])), value => value.events[0].consumed = false,
    value => value.events[0].species = '\ud800', value => value.events[0].historyId = 'remembered',
    value => value.events[0].editWhen = '2026-02-30 18:30:53', value => value.events[0].foreign = true,
  ]) {
    const invalid = copy(wire); mutate(invalid);
    assert.throws(() => histories.siviDeletionHistoryFromWire(invalid, owner));
  }
});

test('restoration discovery forbids remembered/unloaded/consumed UUIDs and retains owned choices after cancelled read', async () => {
  const { current, port } = await fixture();
  current.discard();
  await assert.rejects(current.review(restoreId), /loaded owned/);
  const consumed = history(); consumed.events[0].consumed = consumed.events[0].restored = true;
  port.history = () => resolved(consumed);
  assert.equal(await current.loadHistory(), true);
  await assert.rejects(current.review(token), /unconsumed/);
  const pending = held(); port.history = () => pending.promise;
  const load = current.loadHistory(); current.cancel(); pending.release(history());
  assert.equal(await load, false);
  assert.equal(current.view().history.events[0].consumed, true);
  assert.match(current.view().error, /cancelled/);
});
