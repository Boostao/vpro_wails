const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { cell, metadata, restoration, transport } = require('./siviParentTestHelpers.cjs');
const defaults = require('../../resources/sivi-new-row-defaults.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('siviDeletionEditor.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults, './qualityEditor': quality,
  './siviParentTransport': transport, './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const receipts = loadTypeScript('siviDeletionReceipt.ts', {
  './projectMetadataEditor': metadata, './projectMetadataRestore': restoration,
  './qualityEditor': quality, './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
  './siviDeletionEditor': editor,
});
const sessions = loadTypeScript('siviDeletionSession.ts', {
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
  './siviDeletionEditor': editor, './siviDeletionReceipt': receipts,
});
const { owner, token, original, receipt } = require('./siviDeletionTestHelpers.cjs');
const copy = value => structuredClone(value);
function request() { return editor.siviDeletionRequest(original(), owner, token, true); }
function resolved(value) {
  const promise = Promise.resolve(value); promise.cancel = () => {}; return promise;
}
function held() {
  let release, cancelled = 0;
  const promise = new Promise(resolve => { release = resolve; });
  promise.cancel = () => { cancelled++; };
  return { promise, release, cancelled: () => cancelled };
}
async function fixture(overrides = {}) {
  const calls = { original: 0, delete: 0, receipt: 0, requests: [] };
  const port = {
    targets: () => { const value = original(); return resolved({ ...owner, columns: value.columns, rows: [value.original] }); },
    original: () => { calls.original++; return resolved(original()); },
    delete: json => { calls.delete++; calls.requests.push(json); return resolved(receipt(JSON.parse(json))); },
    receipt: json => { calls.receipt++; calls.requests.push(json); return resolved(receipt(JSON.parse(json), true)); },
    ...overrides,
  };
  const current = new sessions.SIVIDeletionSession(owner, port);
  assert.equal(await current.review('SubVegA-SIVI_BC', original().original.rowId), true);
  return { current, port, calls };
}

test('genuine deletion DTO has17 properties, UUID history, literal local seconds and no replacement row', () => {
  const plan = request(), wire = receipt(plan);
  assert.equal(Object.keys(wire).length, 17);
  const accepted = receipts.siviDeletionReceiptFromWire(wire, plan, owner);
  assert.equal(accepted.id, -2147483648);
  assert.equal(accepted.original.cells.length, 44);
  assert.equal(accepted.audits.length, 3);
  assert.equal(accepted.audits.find(audit => audit.editField === 'Flag').beforeEdit, '-1');
  assert.equal(accepted.original.cells[42].integer, '1');
  assert.equal(Object.hasOwn(accepted, 'committed'), false);
  for (const strength of [0, 1, 2]) {
    assert.equal(receipts.siviDeletionReceiptFromWire(receipt(plan, false, strength), plan, owner).audits.length, 0);
  }
  const other = { ...owner, contextId: 'reopened' };
  const lookup = receipt(plan, true); lookup.contextId = other.contextId;
  const result = receipts.siviDeletionReceiptFromWire(lookup, plan, other, 'lookup');
  assert.equal(result.contextId, other.contextId);
  assert.equal(result.request.contextId, owner.contextId);
  result.original.cells[1].text = 'changed';
  assert.equal(plan.original.cells[1].text, '  historical overlength  ');
});

test('receipt refuses malformed, incomplete, foreign or normalized authority without inferring success', () => {
  const plan = request();
  for (const mutate of [
    value => value.historyId = '1', value => value.requestId = token.replace('aaaa', 'bbbb'),
    value => value.contextId = 'foreign', value => value.id = 0, value => value.rowId = '1',
    value => value.form = 'SubVegC-SIVI', value => value.actor = '\ud800',
    value => value.editWhen = '2026-10-07T18:30:53Z', value => value.editWhen = '2026-02-30 18:30:53',
    value => value.editWhen += ' ', value => value.committed = {},
    value => value.request.foreign = true, value => value.request.contextId = 'foreign',
    value => value.columns.reverse(), value => value.original.cells[42].integer = '-1',
    value => value.original.foreign = true, value => value.audits = null,
    value => value.audits.pop(), value => value.audits.push(copy(value.audits[0])),
    value => value.audits[0].rowId = '9223372036854775808',
    value => value.audits[0].afterEdit = '', value => value.audits[0].restore = true,
    value => value.audits[0].beforeEdit = 'repaired', value => value.audits[0].table = 'Sample_Veg',
    value => value.audits[0].editField = 'ID', value => value.audits[0].extra = true,
    value => value.replayed = true,
  ]) {
    const wire = receipt(plan); mutate(wire);
    assert.throws(() => receipts.siviDeletionReceiptFromWire(wire, plan, owner));
  }
  assert.throws(() => receipts.siviDeletionReceiptFromWire(receipt(plan), plan, owner, 'lookup'));
});

test('audit real text follows producer g-format thresholds rather than JavaScript number formatting', () => {
  for (const { real, text } of require('../../resources/sivi-audit-number-format.json')) {
    const plan = request(), index = plan.columns.findIndex(column => column.name === 'Cover5a');
    plan.original.cells[index] = cell('real', real);
    const wire = receipt(plan);
    wire.audits.find(audit => audit.editField === 'Cover5a').beforeEdit = text;
    assert.equal(receipts.siviDeletionReceiptFromWire(wire, plan, owner).audits[1].beforeEdit, text);
  }
});

test('review requires explicit confirmation and Undo before replacing the exact physical target', async () => {
  const { current, calls } = await fixture();
  assert.equal(current.closeState().canSave, false);
  assert.equal(current.closeState().unsaved, true);
  await assert.rejects(current.save(token), /Confirm deletion/);
  await assert.rejects(current.review('SubVegA-SIVI_BC', '1'), /Undo/);
  current.confirm(true);
  const remounted = current.view(); remounted.original.original.cells[1].text = 'foreign';
  assert.equal(current.view().original.original.cells[1].text, '  historical overlength  ');
  assert.equal(await current.save(token), true);
  assert.equal(calls.delete, 1);
  assert.equal(calls.receipt, 0);
  assert.equal(current.view().revision, 1);
  assert.equal(current.closeState().unsaved, false);
  assert.equal(current.view().confirmed, false);
  assert.equal(Object.hasOwn(JSON.parse(calls.requests[0]), 'confirmed'), false);
});

test('lost receipt retains immutable original/request and resolves read-only without another Delete', async () => {
  const { current, calls, port } = await fixture();
  const perform = port.delete; port.delete = json => { perform(json); return resolved({}); };
  current.confirm(true);
  assert.equal(await current.save(token), false);
  assert.equal(current.view().requestId, token);
  assert.equal(current.closeState().blocked, true);
  assert.throws(() => current.discard(), /Resolve/);
  assert.throws(() => current.confirm(false), /Resolve/);
  await assert.rejects(current.save(token), /Resolve/);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(null); };
  assert.equal(await current.resolve(), false);
  assert.match(current.view().error, /Missing history is unresolved/);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(receipt(JSON.parse(json), true)); };
  assert.equal(await current.resolve(), true);
  assert.equal(calls.delete, 1);
  assert.equal(calls.receipt, 2);
  assert.equal(new Set(calls.requests).size, 1);
});

test('cancelled acknowledgement cannot clear unknown authority through a late successful receipt', async () => {
  const pending = held();
  const { current, calls, port } = await fixture();
  port.delete = json => { calls.delete++; calls.requests.push(json); return pending.promise; };
  current.confirm(true);
  const save = current.save(token);
  assert.equal(current.closeState().busy, true);
  current.cancel();
  assert.equal(pending.cancelled(), 1);
  pending.release(receipt(JSON.parse(calls.requests[0])));
  assert.equal(await save, false);
  assert.equal(current.view().revision, 0);
  assert.equal(current.closeState().blocked, true);
  assert.equal(await current.resolve(), true);
  assert.equal(calls.delete, 1);
  assert.equal(current.view().revision, 1);
});

test('cancelled original read cannot infer a draft, confirmation or mutation from late delivery', async () => {
  const pending = held();
  const current = new sessions.SIVIDeletionSession(owner, {
    original: () => pending.promise, delete: () => { throw new Error('unexpected Delete'); },
    receipt: () => { throw new Error('unexpected lookup'); },
  });
  const review = current.review('SubVegA-SIVI_BC', original().original.rowId);
  current.cancel(); pending.release(original());
  assert.equal(await review, false);
  assert.equal(current.view().original, null);
  assert.equal(current.view().confirmed, false);
  assert.equal(current.closeState().canSave, false);
  assert.match(current.view().error, /cancelled/);
});

test('cancelled lookup retains the same unknown request despite late receipt delivery', async () => {
  const { current, port } = await fixture({ delete: () => resolved({}) });
  current.confirm(true);
  assert.equal(await current.save(token), false);
  const pending = held(); port.receipt = () => pending.promise;
  const lookup = current.resolve();
  current.cancel(); pending.release(receipt(request(), true));
  assert.equal(await lookup, false);
  assert.equal(current.view().requestId, token);
  assert.equal(current.view().revision, 0);
  assert.equal(current.closeState().blocked, true);
  assert.throws(() => current.discard(), /Resolve/);
});

test('subscription owner receives synchronous state changes and cleanup stops remount notifications', async () => {
  const { current } = await fixture();
  let notifications = 0;
  const release = current.subscribe(() => notifications++);
  assert.equal(notifications, 1);
  current.confirm(true);
  assert.equal(notifications, 2);
  release();
  current.discard();
  assert.equal(notifications, 2);
});

test('owned raw targets discover exact source paths without borrowing other workflow gates or repairing IDs', () => {
  const value = original(), wire = { ...owner, columns: value.columns, rows: [value.original] };
  const targets = editor.siviDeletionTargetsFromWire(wire, owner);
  assert.deepEqual(copy(targets.map(target => target.form)), ['SubVegA-SIVI_BC', 'SubVegA-SIVI']);
  assert.equal(targets[0].unavailable, null);
  assert.match(targets[0].label, /historical overlength/);
  value.original.cells[43] = cell();
  assert.match(editor.siviDeletionTargetsFromWire(wire, owner)[0].unavailable, /NULL/);
  assert.equal(value.original.cells[43].storage, 'null');
  const cover = value.columns.findIndex(column => column.name === 'Cover5a');
  value.original.cells[cover] = cell();
  assert.equal(editor.siviDeletionTargetsFromWire(wire, owner).length, 0);
});

test('target loading keeps unsubmitted review immutable and late cancelled rows never replace prior targets', async () => {
  const { current, port } = await fixture();
  await assert.rejects(current.loadTargets(), /Undo/);
  current.discard();
  assert.equal(await current.loadTargets(), true);
  assert.equal(current.view().targets.length, 2);
  const pending = held(); port.targets = () => pending.promise;
  const load = current.loadTargets(); current.cancel();
  pending.release({ ...owner, columns: [], rows: [] });
  assert.equal(await load, false);
  assert.equal(current.view().targets.length, 2);
  assert.match(current.view().error, /cancelled/);
});

test('empty target tables cannot turn malformed owner/schema or foreign physical parents into success', () => {
  const value = original();
  for (const mutate of [
    wire => wire.contextId = 'foreign', wire => wire.columns[0].declaredType = '\ud800',
    wire => wire.columns[0].foreign = true, wire => wire.foreign = true,
    wire => wire.columns.pop(), wire => wire.rows[0].cells[0].text = 'foreign',
  ]) {
    const wire = { ...owner, columns: copy(value.columns), rows: [copy(value.original)] };
    mutate(wire); assert.throws(() => editor.siviDeletionTargetsFromWire(wire, owner));
  }
  assert.throws(() => editor.siviDeletionTargetsFromWire({ ...owner, plot: '', columns: value.columns, rows: [] }, { ...owner, plot: '' }));
});

test('historical BOOLEAN storage errors remain blocked through remount until explicit Undo', async () => {
  const invalid = original(); invalid.original.cells[42] = cell('text', 'TRUE');
  const { current } = await fixture({ original: () => resolved(invalid) });
  assert.match(current.view().error, /BOOLEAN/);
  current.confirm(true);
  assert.equal(current.closeState().blocked, true);
  assert.equal(current.closeState().canSave, false);
  assert.match(current.view().error, /BOOLEAN/);
  current.discard();
  assert.equal(current.view().error, null);
  assert.equal(current.closeState().unsaved, false);
});
