const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { cell, metadata, restoration, transport } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric,
});
const cover = loadTypeScript('siviCoverEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric, './siviHeightEditor': height,
});
const decisions = loadTypeScript('vegetationSpeciesEditor.ts', { './qualityEditor': quality });
const species = loadTypeScript('siviSpeciesEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './vegetationSpeciesEditor': decisions, './siviHeightEditor': height,
});
const editor = loadTypeScript('siviCreationEditor.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviCoverEditor': cover,
  './siviSpeciesEditor': species, './vegetationSpeciesEditor': decisions,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const defaults = require('../../resources/sivi-new-row-defaults.json');
const receipts = loadTypeScript('siviCreationReceipt.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults,
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviParentTransport': transport,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
});
const reads = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const sessions = loadTypeScript('siviCreationSession.ts', {
  './readRequests': reads, './siviCreationEditor': editor,
  './siviSpeciesEditor': species, './siviCreationReceipt': receipts,
});
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const token = '00000000-0000-4000-8000-000000000001';
const copy = value => structuredClone(value);
const references = () => copy(require('./siviSpeciesReferenceFixture.json'));
function validDraft(form = 'SubVegA-SIVI') {
  let draft = editor.stageSIVICreationSpecies(editor.beginSIVICreation(form, owner),
    form === 'SubVegD-SIVI' ? 'MOSS' : 'TREE');
  return editor.stageSIVICreationCover(draft, form === 'SubVegD-SIVI' ? 'Cover7' : 'Cover1', '0', false);
}
function request() { return editor.siviCreationRequest(validDraft(), references(), owner, token); }
function receipt(plan, lookup = false) {
  const columns = ['ID', ...defaults.initialNullColumns, 'Flag'].map(name => ({ name, declaredType: name === 'Species' ? 'TEXT' : 'INTEGER' }));
  const original = { rowId: '', cells: columns.map(column => column.name === 'Flag' ? cell('integer', '0') : cell()) };
  const committed = { rowId: '9223372036854775806', cells: columns.map(column => {
    if (column.name === 'ID') return cell('integer', '2147483647');
    if (column.name === 'PlotNumber') return cell('text', plan.plot);
    if (column.name === 'Species') return cell('text', plan.species);
    if (column.name === 'Flag') return cell('integer', '0');
    return copy(plan.covers.find(cover => cover.column === column.name)?.value ?? cell());
  }) };
  return { requestId: plan.requestId, ...owner, form: plan.form, rowId: committed.rowId, historyId: plan.requestId,
    id: 2147483647, actor: ' literal user ', auditStrength: 3, editWhen: '2026-10-07 18:30:53',
    columns, original, committed, covers: copy(plan.covers), request: copy(plan), didCommit: !lookup, replayed: lookup };
}
function resolved(value) {
  const promise = Promise.resolve(value);
  promise.cancel = () => {};
  return promise;
}
function held() {
  let release;
  const promise = new Promise(resolve => { release = resolve; });
  promise.cancel = () => {};
  return { promise, release };
}
async function fixture(overrides = {}) {
  const calls = { references: 0, create: 0, receipt: 0, requests: [] };
  const port = {
    references: () => { calls.references++; return resolved(references()); },
    create: json => { calls.create++; calls.requests.push(json); return resolved(receipt(JSON.parse(json))); },
    receipt: json => { calls.receipt++; calls.requests.push(json); return resolved(receipt(JSON.parse(json), true)); },
    ...overrides,
  };
  const current = new sessions.SIVICreationSession(owner, port);
  assert.equal(await current.loadReferences(), true);
  current.start('SubVegA-SIVI');
  current.updateDraft(validDraft());
  return { current, calls, port };
}

test('creation request needs a stable UUID and detaches plan cells/explicit decisions without assigning identities', () => {
  const draft = validDraft();
  const plan = editor.siviCreationRequest(draft, references(), owner, token);
  assert.equal(plan.requestId, token);
  assert.equal(plan.covers[0].value.real, 0);
  plan.covers[0].value.real = 5;
  assert.equal(draft.cells.Cover1.value.real, 0);
  assert.equal(Object.hasOwn(plan, 'id'), false);
  assert.equal(Object.hasOwn(plan, 'decision'), false);
  for (const id of ['', 'unstable', token.toUpperCase().replace('000000000001', 'ABCDEABCDEAB')]) {
    assert.throws(() => editor.siviCreationRequest(draft, references(), owner, id), /stable request/);
  }
});

test('complete creation receipt verifies43 preallocation NULLs/Flag0 and exact44-cell committed defaults and plan', () => {
  const plan = request(), wire = receipt(plan);
  const approved = receipts.siviCreationReceiptFromWire(wire, plan, owner);
  assert.equal(approved.columns.length, 44);
  assert.equal(approved.original.cells.filter(cell => cell.storage === 'null').length, 43);
  assert.equal(approved.original.rowId, '');
  assert.equal(approved.committed.rowId, '9223372036854775806');
  assert.equal(approved.id, 2147483647);
  assert.equal(approved.actor, ' literal user ');
  assert.equal(approved.didCommit, true);
  wire.committed.cells[1].text = 'foreign';
  plan.covers[0].value.real = 50;
  assert.equal(approved.committed.cells[1].text, 'P');
  assert.equal(approved.covers[0].value.real, 0);
});

test('creation producer uses its exact request UUID history key and calendar-valid local audit seconds', () => {
  const plan = request();
  for (const lookup of [false, true]) {
    const wire = receipt(plan, lookup);
    const approved = receipts.siviCreationReceiptFromWire(wire, plan, owner, lookup ? 'lookup' : 'create');
    assert.equal(approved.historyId, plan.requestId);
    assert.equal(approved.editWhen, '2026-10-07 18:30:53');
  }
  for (const historyId of ['1', '9223372036854775807', '00000000-0000-4000-8000-000000000002', token + '\n']) {
    assert.throws(() => receipts.siviCreationReceiptFromWire({ ...receipt(plan), historyId }, plan, owner));
  }
  for (const editWhen of ['2026-10-07T18:30:53Z', '2026-10-07 18:30:53.1', '2026-10-07 18:30:53\n',
    '2026-02-29 18:30:53', '2026-04-31 18:30:53', '2026-10-07 24:30:53', '2026-10-07 18:60:53']) {
    assert.throws(() => receipts.siviCreationReceiptFromWire({ ...receipt(plan), editWhen }, plan, owner));
  }
});

test('receipt follows physical schema order without recasing or inventing source defaults', () => {
  const plan = request(), wire = receipt(plan);
  wire.columns.reverse(); wire.original.cells.reverse(); wire.committed.cells.reverse();
  assert.equal(receipts.siviCreationReceiptFromWire(wire, plan, owner).columns[0].name, 'Flag');
  for (const changed of [
    value => value.columns[0].name = 'flag',
    value => value.columns[1].name = 'ID',
    value => value.columns.pop(),
    value => value.columns[0].declaredType = '\ud800',
    value => value.original.rowId = value.rowId,
    value => value.original.cells[43] = cell(),
    value => value.original.cells[0] = cell('integer', String(value.id)),
  ]) {
    const bad = receipt(plan); changed(bad);
    assert.throws(() => receipts.siviCreationReceiptFromWire(bad, plan, owner));
  }
});

test('every original/committed creation cell is authoritative, including hidden fields, identity and historical flags', () => {
  const plan = request();
  for (let index = 0; index < 44; index++) {
    for (const row of ['original', 'committed']) {
      const bad = receipt(plan);
      bad[row].cells[index] = cell('text', 'unplanned');
      assert.throws(() => receipts.siviCreationReceiptFromWire(bad, plan, owner), `${row} ${index}`);
    }
  }
});

test('incomplete, foreign, malformed or contradictory creation receipts never prove success', () => {
  const plan = request();
  for (const changed of [
    value => value.requestId = 'different', value => value.contextId = 'foreign',
    value => value.project = 'Second', value => value.plot = 'other', value => value.form = 'SubVegD-SIVI',
    value => value.rowId = '1', value => value.committed.rowId = '9223372036854775808',
    value => value.historyId = '0', value => value.historyId = '9223372036854775808',
    value => value.id = 0, value => value.id = -1, value => value.id = 2147483648,
    value => value.actor = '', value => value.actor = 'x'.repeat(101), value => value.actor = '\ud800',
    value => value.actor = 'x\0y', value => value.auditStrength = 4, value => value.auditStrength = 1.5,
    value => value.editWhen = '', value => value.editWhen = '2026-99-07T20:30:00Z',
    value => value.didCommit = false, value => value.replayed = true,
    value => value.columns = null, value => value.original = null, value => value.original.cells = null,
    value => value.committed.cells.pop(), value => value.committed.cells[1].integer = '1',
    value => value.covers = [], value => value.covers[0].value = cell('real', 1),
    value => value.request = null, value => value.request.contextId = 'foreign',
    value => value.request.decision = null, value => value.request.ID = 1,
    value => value.request.covers[0].column = 'Height1',
  ]) {
    const bad = receipt(plan); changed(bad);
    assert.throws(() => receipts.siviCreationReceiptFromWire(bad, plan, owner));
  }
});

test('read-only resolution requires earlier-commit flags and preserves original durable context after explicit reopening', () => {
  const plan = request(), reopened = { ...owner, contextId: 'reopened' };
  const wire = receipt(plan, true); wire.contextId = reopened.contextId;
  const approved = receipts.siviCreationReceiptFromWire(wire, plan, reopened, 'lookup');
  assert.equal(approved.contextId, 'reopened');
  assert.equal(approved.request.contextId, 'C');
  assert.equal(approved.didCommit, false);
  assert.equal(approved.replayed, true);
  assert.throws(() => receipts.siviCreationReceiptFromWire(receipt(plan), plan, owner, 'lookup'));
  assert.throws(() => receipts.siviCreationReceiptFromWire(wire, plan, { ...reopened, project: 'Second' }, 'lookup'));
});

test('complete receipts preserve and independently compare every explicit Species decision property', () => {
  for (const [entry, kind, selected] of [['old', 'keep'], ['old', 'replace', 'TREE'], ['mine', 'user', 'MINE']]) {
    let draft = editor.stageSIVICreationSpecies(validDraft(), entry);
    draft = editor.chooseSIVICreationSpecies(draft, references(), owner, kind, selected);
    const plan = editor.siviCreationRequest(draft, references(), owner, token);
    const wire = receipt(plan);
    assert.equal(receipts.siviCreationReceiptFromWire(wire, plan, owner).request.decision.kind, kind);
    for (const changed of [
      value => delete value.request.decision,
      value => value.request.decision.kind = 'foreign',
      value => value.request.decision.entered = 'foreign',
      value => value.request.decision.selected = 'foreign',
      value => value.request.decision.extra = true,
    ]) {
      const bad = receipt(plan); changed(bad);
      assert.throws(() => receipts.siviCreationReceiptFromWire(bad, plan, owner));
    }
  }
});

test('accepted creation clears only its draft, emits one revision and keeps detached source-bound receipt', async () => {
  const { current, calls } = await fixture();
  assert.equal(current.closeState().unsaved, true);
  assert.equal(current.closeState().canSave, true);
  assert.equal(await current.save(token), true);
  assert.equal(calls.create, 1);
  assert.equal(current.view().revision, 1);
  assert.equal(current.view().draft, null);
  assert.equal(current.closeState().unsaved, false);
  const view = current.view();
  view.receipt.request.species = 'foreign';
  assert.equal(current.view().receipt.request.species, 'TREE');
  await assert.rejects(current.save(token), /Start an explicit/);
  current.start('SubVegD-SIVI');
  assert.equal(current.view().receipt, null);
  assert.equal(current.view().revision, 1);
});

test('invalid raw input survives unrelated corrections and remount subscriptions, and explicit Undo clears the correct draft', async () => {
  const { current, calls } = await fixture();
  current.updateDraft(editor.stageSIVICreationCover(current.view().draft, 'Cover1', '100', false));
  current.updateDraft(editor.stageSIVICreationCover(current.view().draft, 'Cover2', '5', false));
  const notify = () => {}; current.subscribe(notify)(); current.subscribe(notify)();
  assert.equal(current.view().draft.cells.Cover1.raw, '100');
  assert.equal(current.closeState().blocked, true);
  assert.equal(current.closeState().canSave, false);
  await assert.rejects(current.save(token), /100/);
  assert.equal(calls.create, 0);
  current.updateDraft(editor.stageSIVICreationCover(current.view().draft, 'Cover1', '0', false));
  assert.equal(current.view().error, null);
  assert.equal(current.closeState().blocked, false);
  current.discard();
  assert.equal(current.view().draft, null);
  assert.equal(current.closeState().unsaved, false);
});

test('foreign draft/form replacement and hidden authority fail without replacing the retained draft', async () => {
  const { current } = await fixture();
  const original = current.view().draft;
  for (const changed of [
    draft => draft.contextId = 'foreign', draft => draft.project = 'Second',
    draft => draft.plot = 'other', draft => draft.form = 'SubVegD-SIVI',
    draft => draft.cells.ID = draft.cells.Cover1,
    draft => draft.cells.Cover1.expected = cell('real', 20),
  ]) {
    const bad = copy(original); changed(bad);
    assert.throws(() => current.updateDraft(bad));
    assert.deepEqual(current.view().draft, original);
  }
  assert.throws(() => current.start('SubVegD-SIVI'), /explicitly discard/);
});

test('lost acknowledgement retains stable request and readonly unresolved/conflicting resolution cannot replay or discard it', async () => {
  const { current, calls, port } = await fixture();
  port.create = json => { calls.create++; calls.requests.push(json); return resolved(null); };
  assert.equal(await current.save(token), false);
  const draft = current.view().draft;
  assert.equal(current.closeState().blocked, true);
  assert.equal(current.view().requestId, token);
  assert.throws(() => current.discard(), /unknown/);
  assert.throws(() => current.start('SubVegD-SIVI'), /unknown/);
  assert.throws(() => current.updateDraft(validDraft()), /unknown/);
  await assert.rejects(current.save(token), /unknown/);
  await assert.rejects(current.loadReferences(), /unknown/);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(null); };
  assert.equal(await current.resolve(), false);
  assert.match(current.view().error, /unresolved, not proof/);
  assert.deepEqual(current.view().draft, draft);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(receipt(JSON.parse(json))); };
  assert.equal(await current.resolve(), false);
  port.receipt = json => { calls.receipt++; calls.requests.push(json); return resolved(receipt(JSON.parse(json), true)); };
  assert.equal(await current.resolve(), true);
  assert.equal(calls.create, 1);
  assert.equal(calls.receipt, 3);
  assert.equal(new Set(calls.requests).size, 1);
  assert.equal(current.view().receipt.didCommit, false);
  assert.equal(current.view().revision, 1);
  assert.equal(current.closeState().unsaved, false);
});

test('cancelled creation and late successful acknowledgement retain unknown request until a separate readonly lookup', async () => {
  const delayed = held();
  const { current, calls, port } = await fixture();
  let saved;
  port.create = json => { calls.create++; saved = JSON.parse(json); return delayed.promise; };
  const saving = current.save(token);
  assert.equal(current.view().busy, true);
  assert.throws(() => current.discard(), /Wait for or cancel/);
  await assert.rejects(current.save(token), /Wait for or cancel/);
  current.cancel();
  assert.equal(current.view().blocked, true);
  delayed.release(receipt(saved));
  assert.equal(await saving, false);
  assert.equal(current.view().receipt, null);
  assert.equal(current.view().revision, 0);
  assert.equal(current.view().requestId, token);
  assert.equal(await current.resolve(), true);
  assert.equal(calls.create, 1);
  assert.equal(calls.receipt, 1);
});

test('cancelled readonly lookup cannot clear unknown authority through its late receipt', async () => {
  const { current, calls, port } = await fixture();
  port.create = json => { calls.create++; calls.requests.push(json); return resolved(null); };
  await current.save(token);
  const delayed = held();
  port.receipt = () => delayed.promise;
  const resolving = current.resolve();
  current.cancel();
  delayed.release(receipt(JSON.parse(calls.requests[0]), true));
  assert.equal(await resolving, false);
  assert.equal(current.view().blocked, true);
  assert.equal(current.view().receipt, null);
  assert.equal(current.view().requestId, token);
  assert.equal(calls.create, 1);
});

test('failed or cancelled definition reload leaves the same draft and previously approved definitions intact', async () => {
  const { current, port } = await fixture();
  const original = current.view();
  port.references = () => resolved({ ...references(), project: 'Second' });
  assert.equal(await current.loadReferences(), false);
  assert.deepEqual(current.view().draft, original.draft);
  assert.deepEqual(current.view().references, original.references);
  const delayed = held();
  port.references = () => delayed.promise;
  const loading = current.loadReferences();
  current.cancel();
  delayed.release(references());
  assert.equal(await loading, false);
  assert.deepEqual(current.view().draft, original.draft);
  assert.deepEqual(current.view().references, original.references);
  assert.match(current.view().error, /cancelled/);
});
