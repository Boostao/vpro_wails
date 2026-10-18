const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { editor, receipt, TwoPageSoilSession } = require('./twoPageSoilTestHelpers.cjs');
const cases = require('./twoPageSoilCreationWire.json');
const fresh = index => structuredClone(cases[index]);
const ownerOf = request => {
  const { contextId, project, plot, form } = request.original;
  return { contextId, project, plot, form };
};
const deferred = () => {
  let resolve;
  const promise = new Promise(yes => { resolve = yes; });
  promise.cancel = () => {};
  return { promise, resolve };
};
function setup(index = 0, overrides = {}) {
  const value = fresh(index);
  const owner = ownerOf(value.request);
  const calls = { saves: 0, creates: [], recoveries: [] };
  const port = {
    read: () => Promise.resolve(value.request.original),
    save: () => { calls.saves++; throw new Error('ordinary Save must not create'); },
    create: json => { calls.creates.push(json); return Promise.resolve(value.receipt); },
    outcome: json => {
      calls.recoveries.push(json);
      return Promise.resolve({ contextId: owner.contextId, requestId: value.request.requestId, requestJSON: json,
        status: 'committed', reason: 'Exact source creation history.',
        receipt: { ...value.receipt, didCommit: false, replayed: true } });
    },
    ...overrides,
  };
  return { value, owner, calls, session: new TwoPageSoilSession(owner, port, true, true) };
}

test('eight actual-Go creation receipts preserve complete defaults and NULL-addition audits in live client proxies', () => {
  assert.equal(cases.length, 8);
  for (const entry of cases) {
    const owner = ownerOf(entry.request);
    const checked = receipt.twoPageSoilReceiptFromWire(client.proxy(structuredClone(entry.receipt)),
      client.proxy(structuredClone(entry.request)), owner);
    assert.deepEqual(structuredClone(checked), entry.receipt);
    assert.doesNotThrow(() => structuredClone(checked));
    assert.equal(checked.history.Audits.length, checked.history.AuditStrength >= 2 ? 1 : 0);
    const created = checked.history.Committed.rows.find(row => row.rowId === checked.history.CreatedRowID);
    for (const [index, column] of checked.history.Committed.columns.entries()) {
      const cell = created.cells[index];
      if (!['ID', 'Flag', 'Horizon', 'PlotNumber'].includes(column.name)) assert.equal(cell.storage, 'null');
      if (column.name === 'Flag') assert.equal(cell.integer, '0');
      if (column.name === 'Horizon') assert.equal(cell.text, ' Oh ');
    }
  }
});

test('creation receipt refuses altered original/peer/default/schema/identity/audit and legacy authority', () => {
  for (const mutate of [
    value => value.extra = true,
    value => value.history.Request.extra = true,
    value => value.history.Request.horizon = 'changed',
    value => value.history.Request.edits.push({}),
    value => value.history.Request.original.contextId = 'foreign',
    value => value.history.CreatedRowID = value.history.Request.original.records.rows[0].rowId,
    value => value.history.CreatedRowID = '01',
    value => value.history.Committed.rows.pop(),
    value => value.history.Committed.rows[0].cells[1] = { storage: 'null', text: null, integer: null, real: null, blobHex: null },
    value => value.history.Committed.columns[0].declaredType = 'NUMERIC',
    value => {
      const index = value.history.Committed.columns.findIndex(column => column.name === 'Flag');
      value.history.Committed.rows.at(-1).cells[index].integer = '-1';
    },
    value => {
      const index = value.history.Committed.columns.findIndex(column => column.name === 'Horizon');
      value.history.Committed.rows.at(-1).cells[index].text = 'Oh';
    },
    value => value.history.Audits[0].EditField = 'ID',
    value => value.history.Audits[0].AfterEdit = 'Oh',
    value => delete value.history.CreatedRowID,
  ]) {
    const value = fresh(3);
    mutate(value.receipt);
    assert.throws(() => receipt.twoPageSoilReceiptFromWire(value.receipt, value.request, ownerOf(value.request)));
  }
});

test('new Horizon raw validation retains source widths, malformed Unicode, literal whitespace and caller ownership', () => {
  const { request } = fresh(0);
  const owner = ownerOf(request);
  assert.equal(editor.twoPageSoilCreationRequest(client.proxy(request.original), owner, ' Oh ', request.requestId).horizon, ' Oh ');
  for (const raw of ['', '123456789', '\ud800', '🌲🌲🌲🌲A']) {
    assert.throws(() => editor.twoPageSoilCreationRequest(request.original, owner, raw, request.requestId));
  }
  assert.doesNotThrow(() => editor.twoPageSoilCreationRequest(request.original, owner, '🌲🌲🌲🌲', request.requestId));
  const altered = structuredClone(request.original);
  altered.records.columns.push({ name: 'UnknownDefault', declaredType: '' });
  for (const row of altered.records.rows) row.cells.push({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
  assert.throws(() => editor.twoPageSoilCreationRequest(altered, owner, 'Oh', request.requestId));
});

test('retained invalid/valid creation blocks ordinary Save and disposal while correction/Undo remain live', async () => {
  const { session, value, calls } = setup();
  await session.load();
  session.beginCreation();
  assert.equal(session.closeState().blocked, true);
  assert.throws(() => session.dispose());
  session.stageCreation('123456789');
  assert.equal(session.view().creation.raw, '123456789');
  assert.equal(session.closeState().blocked, true);
  session.stageCreation(' Oh ');
  assert.equal(session.closeState().canSave, false);
  assert.equal(session.closeState().unsaved, true);
  await assert.rejects(session.save(value.request.requestId));
  assert.throws(() => session.stage(session.view().original.records.rows[0].rowId, 'Horizon', 'Oh', false));
  session.undo();
  assert.equal(session.view().creation, undefined);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(calls.saves, 0);
  assert.equal(calls.creates.length, 0);
});

test('explicit creation retains one immutable receipt and updates physical rows without ordinary Save', async () => {
  const { session, value, calls } = setup(3);
  await session.load(); session.beginCreation(); session.stageCreation(' Oh ');
  assert.equal(await session.create(value.request.requestId), true);
  assert.equal(calls.creates.length, 1);
  assert.equal(calls.saves, 0);
  assert.deepEqual(JSON.parse(calls.creates[0]), value.request);
  assert.deepEqual(session.view().original.records, value.receipt.history.Committed);
  assert.equal(session.view().creation, undefined);
  assert.equal(session.closeState().blocked, true);
  const retained = session.view().receipts;
  session.acknowledge();
  assert.deepEqual(session.view().receipts, retained);
  assert.equal(session.closeState().unsaved, false);
});

test('held creation cancellation recovers read-only once and never dispatches another creation', async () => {
  const held = deferred();
  const { session, value, calls } = setup(3, { create: json => { calls.creates.push(json); return held.promise; } });
  await session.load(); session.beginCreation(); session.stageCreation(' Oh ');
  const pending = session.create(value.request.requestId);
  session.cancel();
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().creation.raw, ' Oh ');
  await assert.rejects(session.resolve(), /transport to settle/);
  held.resolve(value.receipt);
  assert.equal(await pending, false);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(await session.resolve(), true);
  assert.equal(calls.creates.length, 1);
  assert.equal(calls.recoveries.length, 1);
  assert.equal(session.view().receipts[0].replayed, true);
});

test('creation has its own explicit client gate and ordinary sessions do not gain creation authority', async () => {
  const value = fresh(0);
  const session = new TwoPageSoilSession(ownerOf(value.request), {
    read: () => Promise.resolve(value.request.original), save: () => Promise.reject(new Error('unexpected')),
    outcome: () => Promise.reject(new Error('unexpected')),
  });
  await session.load();
  assert.throws(() => session.beginCreation(), /enabled creation/);
  assert.equal(session.closeState().unsaved, false);
});
