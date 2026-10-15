const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { wire, editor, receipt, references, speciesList, cell, TwoPageVegetationSession } = require('./twoPageVegetationTestHelpers.cjs');
const fixtures = require('./twoPageVegetationSpeciesListReceipts.json');
const fresh = (index = 7) => structuredClone(fixtures[index]);
const ownerOf = request => {
  const { contextId, project, plot, form } = request.original;
  return { contextId, project, plot, form };
};
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
const ready = () => Promise.resolve();
function held() {
  let release;
  const promise = new Promise(resolve => { release = resolve; });
  return { promise, release };
}
function setup(extra = {}, selection = true) {
  const source = fresh(), request = source.history.Request, owner = ownerOf(request);
  const calls = { legacy: 0, selected: [], recovered: [] };
  const session = new TwoPageVegetationSession(owner, {
    read: () => operation(client.proxy(structuredClone(request.original))),
    references: () => operation(client.proxy(structuredClone(request.speciesList))),
    save: () => { calls.legacy++; throw new Error('Unexpected legacy Species dispatch'); },
    saveSpeciesList: json => { calls.selected.push(json); return operation(client.proxy(structuredClone(source))); },
    outcome: json => {
      calls.recovered.push(json);
      return operation({ contextId: owner.contextId, requestId: request.requestId, requestJSON: json,
        status: 'committed', reason: 'Exact durable source plan.', receipt: { ...source, didCommit: false, replayed: true } });
    },
    validationYield: ready, ...extra,
  }, true, selection);
  return { source, request, owner, calls, session };
}
function stage(session) {
  session.stage('-7', 'Species', 'A', false);
  session.stage('-7', 'Cover1', '5', false);
  session.stage('-7', 'Collected', 'C', false);
}

test('qualified literal options preserve actual physical duplicates NULL/empty role identity and detach real proxies', async () => {
  const source = fresh().history.Request, owner = ownerOf(source);
  const proxy = client.proxy(structuredClone(source.speciesList));
  const refs = await references.vegetationReferencesFromWire(proxy, owner, ready);
  const options = speciesList.vegetationSpeciesListOptions(client.proxy(refs), owner);
  assert.deepEqual(Array.from(options, option => [option.role, option.rowId, option.code]),
    [['master', '-9', 'A'], ['master', '-8', 'A'], ['personal', '1', 'A']]);
  assert.equal(options[0].cells[1].storage, 'null');
  assert.equal(options[1].cells[1].text, '');
  options[0].cells[0].text = 'changed';
  assert.equal(proxy.master.rows[0].cells[0].text, 'A');
  assert.doesNotThrow(() => structuredClone(options));
  for (const value of ['a', 'A ', ' A', 'OLD', 'UNKNOWN', '', '\ud800', 'A\0']) {
    const draft = editor.stageTwoPageVegetation(source.original, '-7', 'Species', value, false, refs);
    assert.ok(draft.error, `unexpected literal acceptance: ${JSON.stringify(value)}`);
  }
  assert.throws(() => editor.stageTwoPageVegetation(source.original, '-7', 'Species', 'A', false), /unavailable/);
  assert.ok(editor.stageTwoPageVegetation(source.original, '-7', 'Species', '', true, refs).error);
  for (const form of ['SubVegCXL', 'SubVegDXL']) {
    assert.throws(() => speciesList.vegetationSpeciesListOptions({ ...refs, form }, { ...owner, form }), /A\/CHARS/);
  }
  const unicode = '\ud83d\ude00'.repeat(4);
  refs.master.rows[0].cells[0] = cell('text', unicode);
  assert.equal(editor.stageTwoPageVegetation(source.original, '-7', 'Species', unicode, false, refs).error, null);
  refs.master.rows[0].cells[0] = cell('text', unicode + 'A');
  assert.ok(editor.stageTwoPageVegetation(source.original, '-7', 'Species', unicode + 'A', false, refs).error);
});

test('unchanged historical Species is omitted and never normalizes or grants a selection envelope', () => {
  const request = fresh().history.Request;
  const owner = ownerOf(request);
  const index = wire.vegetationColumnIndex(request.original, 'Species');
  request.original.records.rows[0].cells[index] = cell('text', 'historical overlength');
  const unchanged = editor.stageTwoPageVegetation(request.original, '-7', 'Species', 'historical overlength', false, request.speciesList);
  const cover = editor.stageTwoPageVegetation(request.original, '-7', 'Cover1', '5', false);
  const result = editor.twoPageVegetationRequest(request.original, owner,
    { species: unchanged, cover }, request.requestId, request.speciesList);
  assert.equal(Object.hasOwn(result, 'speciesList'), false);
  assert.deepEqual(Array.from(result.edits, edit => edit.column), ['Cover1']);
  assert.equal(wire.vegetationSourceCell(result.original, '-7', 'Species').text, 'historical overlength');
});

test('all eight actual strict-Go receipts detach client proxies and retain complete selection/audit/history provenance', async () => {
  for (const [index, original] of fixtures.entries()) {
    const source = structuredClone(original), request = source.history.Request, owner = ownerOf(request);
    assert.throws(() => receipt.twoPageVegetationReceiptFromWire(source, request, owner));
    const confirmed = await receipt.twoPageVegetationSpeciesListReceiptFromWire(client.proxy(source), client.proxy(request), owner, false, ready);
    assert.deepEqual(JSON.parse(JSON.stringify(confirmed)), original);
    assert.equal(confirmed.history.Audits.length, [0, 2, 3, 3][index % 4]);
    assert.doesNotThrow(() => structuredClone(confirmed));
    const recovery = await receipt.twoPageVegetationSpeciesListReceiptFromWire(
      { ...source, didCommit: false, replayed: true }, request, owner, true, ready);
    assert.equal(recovery.replayed, true);
  }
});

test('full selection receipt rejects peer reference drift malformed production keys and changed plan authority', async () => {
  for (const mutate of [
    source => { source.history.Request.speciesList.master.rows[1].cells[1] = cell('text', 'other definition'); },
    source => { source.history.Request.speciesList.personal.rows[0].cells[0] = cell('text', 'OTHER'); },
    source => { source.history.Request.speciesList.master.columns[0].type = 'TEXT'; },
    source => { source.history.Request.speciesList.master.rows[0].cells[2] = cell('integer', '2147483648'); },
    source => { source.history.Request.speciesList.personal.rows[0].cells[2] = cell('integer', '32768'); },
    source => { source.history.Request.speciesList.plot += ' '; },
    source => { source.history.Request.edits[0].value = cell('text', 'a'); },
    source => { source.history.Committed.rows[1].cells[2] = cell('text', 'merged peer'); },
    source => { source.history.Request.extra = true; },
  ]) {
    const source = fresh(), retained = structuredClone(source.history.Request);
    mutate(source);
    await assert.rejects(receipt.twoPageVegetationSpeciesListReceiptFromWire(source, retained, ownerOf(retained), false, ready));
  }
});

test('selection validation persists across subscriptions and correction Undo and seven-cover transitions stay live', async () => {
  const { session } = setup();
  await session.load(); await session.loadReferences();
  session.stage('-7', 'Species', 'A ', false);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.closeState().canSave, false);
  const before = session.view();
  const unsubscribe = session.subscribe(() => {}); unsubscribe();
  assert.deepEqual(session.view().drafts, before.drafts);
  session.stage('-7', 'Species', 'A', false);
  assert.equal(session.closeState().blocked, false);
  await assert.rejects(session.loadReferences(), /reference snapshot/);
  for (const column of ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB']) session.stage('-7', column, '', true);
  assert.equal(session.closeState().blocked, true);
  assert.match(session.view().drafts[JSON.stringify(['-7', 'Species'])].error, /seven common covers NULL/);
  session.stage('-7', 'Cover3', '0', false);
  assert.equal(session.view().drafts[JSON.stringify(['-7', 'Species'])].error, null);
  assert.equal(session.closeState().canSave, true);
  session.undo();
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.view().error, null);
  const disabled = setup({}, false).session;
  await disabled.load(); await disabled.loadReferences();
  assert.throws(() => disabled.stage('-7', 'Species', 'A', false), /independent gate/);
});

test('selection Save dispatches once through its own service and retains immutable receipt across acknowledgement/remount', async () => {
  const { session, request, calls } = setup();
  await session.load(); await session.loadReferences(); stage(session);
  assert.equal(await session.save(request.requestId), true);
  assert.equal(calls.legacy, 0); assert.equal(calls.selected.length, 1);
  assert.ok(JSON.parse(calls.selected[0]).speciesList);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.closeState().blocked, true);
  session.acknowledge();
  const unsubscribe = session.subscribe(() => {}); unsubscribe();
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(wire.vegetationSourceCell(session.view().original, '-7', 'Species').text, 'A');
});

test('Cancel during awaited full-reference validation rejects the old success and recovers without replay after remount', { timeout: 5000 }, async () => {
  const validation = held(), entered = held();
  let holding = false;
  const { session, request, calls } = setup({ validationYield: () => {
    if (!holding) return Promise.resolve();
    entered.release(); return validation.promise;
  } });
  await session.load(); await session.loadReferences(); stage(session);
  holding = true;
  const saving = session.save(request.requestId);
  await entered.promise;
  session.cancel();
  const unsubscribe = session.subscribe(() => {}); unsubscribe();
  holding = false; validation.release();
  assert.equal(await saving, false);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(wire.vegetationSourceCell(session.view().original, '-7', 'Species').text, 'TEST');
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.resolve(), true);
  assert.equal(calls.selected.length, 1);
  assert.equal(calls.recovered.length, 1);
  assert.equal(calls.recovered[0], calls.selected[0]);
  assert.equal(session.view().receipts.length, 1);
});
