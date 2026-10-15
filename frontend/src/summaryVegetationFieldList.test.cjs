const assert = require('node:assert/strict');
const { test } = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { randomUUID } = require('node:crypto');
const req = createRequire(path.join(__dirname, '..', 'package.json'));
const ts = req('typescript');
const { proxy } = req('svelte/internal/client');
const clone = value => structuredClone(value);
function load(name, dependencies = {}, environment = {}) {
  const source = fs.readFileSync(path.join(__dirname, name), 'utf8').replaceAll('import.meta.env', '__environment');
  const code = ts.transpileModule(source, { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS,
  } }).outputText;
  const module = { exports: {} };
  vm.runInNewContext(code, { module, exports: module.exports, __environment: environment,
    crypto: { randomUUID }, require: id => {
      if (!(id in dependencies)) throw new Error(`Uninjected dependency ${id}`);
      return dependencies[id];
    } }, { filename: name });
  return module.exports;
}
const client = load('summaryVegetationFieldList.ts');
const registries = load('summaryVegetationFieldListSessions.ts', { './summaryVegetationFieldList': client });
const wire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldListWire.json'), 'utf8'));
const { physical, source, ...owner } = wire.review;
const plain = value => clone(value);
const reports = table => Array.from(table.rows, r => r.cells[4].integer);
const ready = async (read = () => Promise.resolve(clone(wire.review)), save = () => Promise.reject(new Error('no delivery')),
  ids = () => randomUUID(), validationYield) => {
  const session = new client.SummaryVegetationFieldListSession(proxy(clone(owner)), { read, save, validationYield }, true, ids);
  assert.equal(await session.load(), true);
  return session;
};
function commitFor(request, changes = {}) {
  return { ...clone(wire.outcome), contextId: request.review.contextId, requestId: request.requestId, ...changes };
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  let cancelled = 0;
  promise.cancel = () => { cancelled++; };
  return { promise, resolve, reject, get cancelled() { return cancelled; } };
}
const tick = () => new Promise(resolve => setImmediate(resolve));

test('actual Go strict wire, declaredType, immutable complete review and real Svelte proxy boundaries', async () => {
  assert.equal(physical.columns[0].declaredType, 'INTEGER');
  assert.equal(wire.review.physical.rows.length, 5);
  const reactive = proxy(clone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const review = client.validateFieldListReview(reactive, proxy(clone(owner)));
  assert.deepEqual(plain(review), wire.review);
  reactive.physical.rows[0].cells[2].text = 'external mutation';
  assert.equal(review.physical.rows[0].cells[2].text, 'duplicate');
  assert.ok(Object.isFrozen(review.physical.rows[0].cells[2]));
  const request = client.fieldListRequest(proxy(clone(wire.request)), owner);
  assert.deepEqual(JSON.parse(client.fieldListJSON(request)), wire.request);
  assert.deepEqual(plain(client.validateFieldListOutcome(proxy(clone(wire.outcome)), request)), wire.outcome);
  let sent;
  const session = await ready(() => Promise.resolve(proxy(clone(wire.review))), json => {
    sent = JSON.parse(json);
    return Promise.resolve(proxy(commitFor(sent)));
  });
  session.stageAll();
  assert.equal(await session.save(), true);
  assert.deepEqual(sent.actions, wire.request.actions);
  assert.deepEqual(plain(session.view().review), wire.after);
  assert.deepEqual(plain(session.view().receipts[0].outcome.plan), wire.outcome.plan);
});

test('independent source ordering, duplicate logical rows, nullable checkbox and hidden global bulk', () => {
  const index = client.fieldListIndex(physical);
  assert.deepEqual(plain(index), { visible: ['1', '2', '5'], selected: ['4', '3'], error: '' });
  const expected = [-1, -1, -1, -1, -1].map(String);
  const all = client.planFieldList(physical, [{ kind: 'all', rowId: '', selected: true }]);
  assert.deepEqual(reports(all.proposed), expected);
  assert.deepEqual(Array.from(all.assignments, a => a.before.rowId), ['1', '2', '5']);
  const none = client.planFieldList(physical, [{ kind: 'none', rowId: '', selected: false }]);
  assert.deepEqual(reports(none.proposed), ['0', '0', '0', '0', '0']);
  assert.deepEqual(Array.from(none.assignments, a => a.before.rowId), ['3', '4', '5']);
  const ordered = client.planFieldList(physical, [
    { kind: 'all', rowId: '', selected: true }, { kind: 'row', rowId: '2', selected: false },
  ]);
  assert.deepEqual(reports(ordered.proposed), ['-1', '0', '-1', '-1', '-1']);
  for (let i = 0; i < 5; i++) for (const col of [0, 1, 2, 3, 5, 6])
    assert.deepEqual(plain(ordered.proposed.rows[i].cells[col]), physical.rows[i].cells[col]);
  for (const rowId of ['3', '4', '5', '6', '01']) assert.throws(() =>
    client.planFieldList(physical, [{ kind: 'row', rowId, selected: true }]));
  assert.throws(() => client.planFieldList(physical, [
    { kind: 'all', rowId: '', selected: true }, { kind: 'row', rowId: '5', selected: false },
  ]), /original/);
});

test('historical NULL/empty/NUL/overlength and all tagged payloads stay complete without label edits', () => {
  const t = clone(physical);
  const tagged = (storage, key, value) => ({ storage, text: null, integer: null, real: null, blobHex: null, [key]: value });
  t.rows[0].cells[2] = tagged('text', 'text', '\0' + '𐐀'.repeat(256));
  t.rows[1].cells[2] = tagged('text', 'text', '');
  t.rows[2].cells[2] = tagged('null', 'text', null);
  t.rows[3].cells[3] = tagged('real', 'real', -0);
  t.rows[4].cells[3] = tagged('blob', 'blobHex', '00ff');
  const reviewed = client.validateFieldListReview({ ...clone(wire.review), physical: proxy(t) }, owner);
  const plan = client.planFieldList(reviewed.physical, [{ kind: 'none', rowId: '', selected: false }]);
  assert.deepEqual(plain(plan.original), t);
  assert.ok(Object.is(plan.proposed.rows[3].cells[3].real, -0));
  assert.ok(Object.is(client.fromWire(client.fieldListJSON(plan.proposed)).rows[3].cells[3].real, -0));
  assert.equal(plan.proposed.rows[0].cells[2].text, t.rows[0].cells[2].text);
});

test('physical availability is separate from invalid source index; bulk may correct Report, not Order', async () => {
  const invalidReport = clone(wire.review);
  invalidReport.physical.rows[4].cells[4] = { storage: 'text', text: 'True', integer: null, real: null, blobHex: null };
  const session = await ready(() => Promise.resolve(invalidReport));
  assert.equal(session.view().readError, '');
  assert.match(session.view().sourceIndex.error, /Report/);
  assert.equal(session.stageAll(), true);
  assert.equal(session.view().proposedIndex.error, '');
  const invalidOrder = clone(wire.review);
  invalidOrder.physical.rows[4].cells[0].integer = '32768';
  const second = await ready(() => Promise.resolve(invalidOrder));
  assert.match(second.view().sourceIndex.error, /Order/);
  assert.equal(second.stageAll(), true);
  assert.match(second.view().proposedIndex.error, /Order/);
  assert.equal(second.stageRow('5', true), false);
  assert.equal(second.closeState().canSave, false);
  second.undo();
  assert.equal(second.view().draftError, '');
});

test('recursive exact physical tags, keys, Unicode, canonical rowids and full columns refuse', () => {
  const mutators = [
    v => { delete v.physical.columns[0].declaredType; },
    v => { v.physical.columns[0].type = 'INTEGER'; },
    v => { v.physical.columns.reverse(); },
    v => { v.physical.rows[1].rowId = v.physical.rows[0].rowId; },
    v => { v.physical.rows.reverse(); },
    v => { v.physical.rows[0].rowId = '+1'; },
    v => { v.physical.rows[0].rowId = '9223372036854775808'; },
    v => { v.physical.rows[0].cells.pop(); },
    v => { v.physical.rows[0].cells[0].integer = '01'; },
    v => { v.physical.rows[0].cells[0].text = '1'; },
    v => { v.physical.rows[0].cells[1].text = '\ud800'; },
    v => { v.physical.rows[0].cells[1].storage = 'TEXT'; },
    v => { v.physical.rows[0].cells[1].extra = null; },
    v => { v.physical.rows[0].cells[1] = { storage: 'real', text: null, integer: null, real: Infinity, blobHex: null }; },
    v => { v.physical.rows[0].cells[1] = { storage: 'blob', text: null, integer: null, real: null, blobHex: 'FF' }; },
    v => { delete v.physical.rows[0]; },
    v => { v.physical.rows.extra = true; },
    v => { v.extra = true; },
  ];
  for (const mutate of mutators) {
    const v = clone(wire.review); mutate(v);
    assert.throws(() => client.validateFieldListReview(v, owner), mutate.toString());
  }
  for (const key of [...Object.keys(owner), 'source']) {
    const v = clone(wire.review); v[key] += 'foreign';
    assert.throws(() => client.validateFieldListReview(v, owner));
  }
  assert.throws(() => client.fieldListOwner({ ...owner, su: 'Chosen', suPath: '' }));
});

test('raw JSON duplicate decoded keys and unpaired surrogates never repair authority', () => {
  const json = JSON.stringify(wire.review);
  for (const raw of [
    json.replace('"contextId":', '"contextId":"foreign","contextId":'),
    json.replace('"contextId":', '"\\u0063ontextId":"foreign","contextId":'),
    json.replace('"storage":"integer"', '"storage":"null","storage":"integer"'),
    json.replace('"declaredType":"INTEGER"', '"declaredType":"INTEGER","\\u0064eclaredType":"INTEGER"'),
    json.replace('"duplicate"', '"\\ud800"'),
    json.replace('"duplicate"', '"\udc00"'),
    json + '{}',
  ]) assert.throws(() => client.validateFieldListReview(raw, owner));
  assert.deepEqual(plain(client.validateFieldListReview(json, owner)), wire.review);
  for (const id of ['00000000-0000-0000-0000-000000000000', wire.request.requestId.toUpperCase(), 'not-a-uuid'])
    assert.throws(() => client.fieldListRequest({ ...clone(wire.request), requestId: id }, owner));
});

test('strict ordered action keys/booleans/identity semantics refuse and cannot edit labels', () => {
  const bad = [
    null, [{ kind: 'all', rowId: '', selected: false }],
    [{ kind: 'none', rowId: '1', selected: false }],
    [{ kind: 'row', rowId: '1', selected: -1 }],
    [{ kind: 'row', rowId: '1' }],
    [{ kind: 'label', rowId: '1', selected: true }],
    [{ kind: 'row', rowId: '1', selected: true, Label: 'new' }],
  ];
  for (const actions of bad) assert.throws(() => client.fieldListRequest({ ...clone(wire.request), actions }, owner));
  assert.throws(() => client.fieldListRequest({ ...clone(wire.request), extra: true }, owner));
});

test('complete independent outcome plan rejects reduced peers, unrelated assignments, flags and owners', () => {
  const request = client.fieldListRequest(wire.request, owner);
  const mutations = [
    v => { v.plan.original.rows.pop(); },
    v => { v.plan.proposed.rows[2].cells[2].text = 'unrelated Label'; },
    v => { v.plan.proposed.rows[4].cells[4] = clone(physical.rows[4].cells[4]); },
    v => { v.plan.assignments.pop(); },
    v => { v.plan.assignments[0].before.cells[2].text = 'reused peer'; },
    v => { v.plan.assignments.reverse(); },
    v => { v.plan.assignments.push(clone(v.plan.assignments[0])); },
    v => { v.contextId = 'foreign'; },
    v => { v.requestId = randomUUID(); },
    v => { v.committed = false; },
    v => { v.plan = null; },
    v => { v.errorMessage = '\ud800'; },
    v => { v.extra = true; },
  ];
  for (const mutate of mutations) {
    const outcome = clone(wire.outcome); mutate(outcome);
    assert.throws(() => client.validateFieldListOutcome(outcome, request), mutate.toString());
  }
  const drift = { ...clone(wire.outcome), replayed: true, verified: false, errorMessage: 'source drift' };
  assert.equal(client.validateFieldListOutcome(drift, request).committed, true);
  assert.equal(client.validateFieldListOutcome({ ...clone(wire.outcome), verified: false }, request).committed, true);
});

test('explicit Save noop retains a known local attempt but not a committed server receipt', async () => {
  let attempts = 0;
  const session = await ready(undefined, json => {
    attempts++;
    const request = JSON.parse(json);
    return Promise.resolve({ contextId: owner.contextId, requestId: request.requestId, changed: false, committed: false,
      replayed: false, verified: true, errorMessage: '',
      plan: { original: clone(physical), proposed: clone(physical), assignments: [] } });
  });
  session.stageRow('1', false);
  assert.equal(await session.save(), true);
  assert.equal(attempts, 1);
  assert.equal(session.view().receipts[0].outcome.committed, false);
  assert.equal(session.view().actions.length, 0);
  assert.equal(session.closeState().blocked, true);
  session.acknowledge();
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().receipts.length, 1);
});

test('committed cleanup warnings and historical replay drift retain complete commit before explicit correction', async () => {
  for (const flags of [
    { replayed: false, verified: true, errorMessage: 'coordinator close warning' },
    { replayed: true, verified: false, errorMessage: 'later physical source drift' },
    { replayed: false, verified: false, errorMessage: '' },
  ]) {
    let readValue = clone(wire.review), attempts = 0;
    const session = await ready(() => Promise.resolve(readValue), json => {
      attempts++; return Promise.resolve(commitFor(JSON.parse(json), flags));
    });
    session.stageAll();
    assert.equal(await session.save(), true);
    assert.equal(session.view().receipts[0].outcome.committed, true);
    assert.deepEqual(plain(session.view().receipts[0].outcome.plan), wire.outcome.plan);
    assert.throws(() => session.acknowledge(), /correction/);
    await assert.rejects(session.save(), /receipt/);
    session.undo();
    assert.equal(session.closeState().blocked, true);
    readValue = clone(wire.after);
    readValue.physical.rows[2].cells[2].text = 'independently observed later drift';
    assert.equal(await session.load(), true);
    assert.throws(() => session.acknowledge(), /correction/);
    session.correctReceipt(); session.acknowledge();
    assert.equal(session.closeState().blocked, false);
    assert.equal(attempts, 1);
    assert.deepEqual(plain(session.view().receipts[0].reconciliation), readValue);
    assert.deepEqual(plain(session.view().receipts[0].outcome.plan), wire.outcome.plan);
  }
});

test('successful historical replay is accepted as historical commit, never a new mutation claim', async () => {
  const session = await ready(undefined, json => Promise.resolve(commitFor(JSON.parse(json), { replayed: true })));
  session.stageAll(); assert.equal(await session.save(), true);
  assert.equal(session.view().receipts[0].outcome.replayed, true);
  assert.deepEqual(plain(session.view().review), wire.after);
  session.acknowledge();
});

test('unknown delivery requires owned Load plus explicit correction; receipts never become invented commits', async () => {
  for (const failure of [() => Promise.reject(new Error('response lost')), () => Promise.resolve(null),
    () => { throw new Error('dispatch threw'); },
    () => Promise.resolve({ ...clone(wire.outcome), contextId: 'foreign' })]) {
    let attempts = 0, readValue = clone(wire.review);
    const session = await ready(() => Promise.resolve(readValue), () => { attempts++; return failure(); });
    session.stageAll(); assert.equal(await session.save(), false);
    assert.equal(session.view().receipts[0].kind, 'unknown');
    assert.throws(() => session.acknowledge());
    await assert.rejects(session.save());
    session.undo();
    readValue = clone(wire.after);
    assert.equal(await session.load(), true);
    assert.equal(session.view().receipts[0].kind, 'unknown');
    assert.throws(() => session.acknowledge());
    session.correctReceipt(); session.acknowledge();
    assert.equal(session.view().receipts[0].outcome, null);
    assert.equal(session.view().receipts[0].acknowledged, true);
    assert.equal(attempts, 1);
  }
});

test('fresh lowercase UUIDv4 belongs to actual Save attempts; used identities cannot be resent', async () => {
  const id = randomUUID();
  let deliveries = 0, generated = 0;
  const session = await ready(undefined, json => {
    deliveries++; return Promise.resolve(commitFor(JSON.parse(json)));
  }, () => { generated++; return id; });
  await assert.rejects(session.save(), /No staged/);
  assert.equal(generated, 0);
  session.stageAll(); await session.save(); session.acknowledge();
  session.stageNone();
  await assert.rejects(session.save(), /already used/);
  assert.equal(deliveries, 1);
  assert.equal(generated, 2);
  assert.match(session.view().receipts[0].request.requestId, /^[0-9a-f-]{36}$/);
});

test('read errors and draft errors survive Undo and registry remount; correction never repairs silently', async () => {
  let failRead = false;
  const reg = registries.fieldListRegistry(() => ({ read: () => failRead
    ? Promise.reject(new Error('read fault')) : Promise.resolve(clone(wire.review)), save: () => Promise.reject('unused') }),
    { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' });
  const session = reg.session(owner);
  await session.load();
  assert.equal(session.stageRow('5', true), false);
  reg.detach(owner);
  assert.equal(reg.session(proxy(clone(owner))), session);
  assert.match(session.view().draftError, /checkbox/);
  assert.equal(reg.activity().blocked, true);
  assert.throws(() => reg.release(owner));
  failRead = true;
  assert.equal(await session.load(), false);
  const readError = session.view().readError;
  session.undo();
  assert.equal(session.view().readError, readError);
  assert.equal(session.closeState().canSave, false);
  assert.equal(session.closeState().blocked, true);
  failRead = false;
  await session.load();
  assert.equal(session.view().readError, '');
  session.stageRow('5', true);
  session.stageRow('1', true);
  assert.equal(session.view().draftError, '');
});

test('held Load validation result cannot cross cancel/remount generation, including later successful Load', async () => {
  const hold = deferred();
  let yieldNext = false;
  const session = await ready(undefined, undefined, undefined, () => yieldNext ? hold.promise : Promise.resolve());
  yieldNext = true;
  const pending = session.load(); await tick();
  session.detach();
  assert.match(session.view().readError, /cancelled/);
  yieldNext = false;
  assert.equal(await session.load(), true);
  const revision = session.view().revision;
  hold.resolve();
  assert.equal(await pending, false);
  assert.equal(session.view().revision, revision);
  assert.deepEqual(plain(session.view().review), wire.review);
});

test('held read failure after remount does not overwrite newer explicit Load', async () => {
  const first = deferred();
  let next = false;
  const session = await ready(() => next ? first.promise : Promise.resolve(clone(wire.review)));
  next = true;
  const pending = session.load();
  session.detach();
  assert.equal(first.cancelled, 1);
  next = false;
  await session.load();
  first.reject(new Error('stale read failure'));
  assert.equal(await pending, false);
  assert.equal(session.view().readError, '');
});

test('cancelled Save keeps root-close delivery hold until transport/result settle, never replays', async () => {
  const response = deferred();
  let attempts = 0;
  const session = await ready(undefined, () => { attempts++; return response.promise; });
  session.stageAll();
  const saving = session.save();
  session.detach();
  assert.equal(response.cancelled, 1);
  assert.equal(session.view().receipts[0].kind, 'unknown');
  assert.equal(session.closeState().busy, true);
  await assert.rejects(session.load(), /settlement/);
  await assert.rejects(session.save(), /settlement/);
  assert.throws(() => session.undo(), /settlement/);
  response.resolve(commitFor(session.view().receipts[0].request));
  assert.equal(await saving, false);
  assert.equal(session.closeState().busy, false);
  assert.equal(session.view().receipts[0].kind, 'unknown');
  await session.load(); session.correctReceipt(); session.acknowledge();
  assert.equal(attempts, 1);
});

test('held Save validation cannot assign known outcome into retired generation', async () => {
  const hold = deferred(); let yielding = false;
  const session = await ready(undefined, json => Promise.resolve(commitFor(JSON.parse(json))), undefined,
    () => yielding ? hold.promise : Promise.resolve());
  session.stageAll(); yielding = true;
  const saving = session.save(); await tick();
  session.detach();
  assert.equal(session.closeState().busy, true);
  hold.resolve(); assert.equal(await saving, false);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.view().receipts[0].kind, 'unknown');
  assert.deepEqual(plain(session.view().review), wire.review);
});

test('foreign/malformed reconciliation preserves previous authority and receipt, Undo keeps read error', async () => {
  let current = clone(wire.review);
  const session = await ready(() => Promise.resolve(current));
  session.stageAll(); await session.save();
  current = { ...clone(wire.after), sourcePath: 'foreign path' };
  assert.equal(await session.load(), false);
  session.undo();
  assert.ok(session.view().readError);
  assert.throws(() => session.correctReceipt());
  assert.throws(() => session.acknowledge());
  assert.deepEqual(plain(session.view().review), wire.review);
  assert.equal(session.view().receipts[0].kind, 'unknown');
});

test('registry literal flags, exact owner isolation, retained receipts and aggregate lifecycle holds', async () => {
  for (const environment of [
    {}, { VITE_SUMMARY_VEGETATION: 'true' }, { VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' },
    { VITE_SUMMARY_VEGETATION: true, VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' },
    { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_FIELD_LIST: 'TRUE' },
  ]) {
    const reg = registries.fieldListRegistry(() => { throw new Error('must not create port'); }, environment);
    assert.equal(reg.enabled, false); assert.throws(() => reg.session(owner), /disabled/);
  }
  assert.equal(registries.summaryVegetationFieldListEnabled, false);
  const reg = registries.fieldListRegistry(scope => ({
    read: () => Promise.resolve({ ...clone(wire.review), ...scope }),
    save: json => Promise.resolve(commitFor(JSON.parse(json))),
  }), { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' });
  const activities = [];
  const unsubscribe = reg.subscribe(value => activities.push(value));
  const first = reg.session(owner), alternateOwner = { ...owner, contextId: owner.contextId + '2' };
  const second = reg.session(alternateOwner);
  await first.load(); await second.load();
  first.stageAll();
  assert.equal(second.view().actions.length, 0);
  assert.deepEqual(plain(reg.activity().owner), owner);
  assert.throws(() => reg.release(owner));
  await first.save(); first.acknowledge();
  assert.equal(reg.release(owner), false);
  assert.equal(reg.session(owner), first);
  assert.equal(reg.release(alternateOwner), true);
  assert.equal(activities.at(-1).blocked, false);
  unsubscribe();
});

test('bounded contexts/actions/rows refuse without eviction, request or evidence reduction', async () => {
  const reg = registries.fieldListRegistry(() => ({ read: () => Promise.resolve(wire.review), save: () => Promise.resolve(null) }),
    { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' });
  for (let i = 0; i < 16; i++) reg.session({ ...owner, contextId: String(i) });
  assert.throws(() => reg.session({ ...owner, contextId: '17' }), /capacity/);
  assert.equal(reg.session({ ...owner, contextId: '0' }), reg.session({ ...owner, contextId: '0' }));
  const session = await ready();
  assert.equal(session.setActions(Array.from({ length: client.fieldListLimits.actions + 1 },
    () => ({ kind: 'all', rowId: '', selected: true }))), false);
  assert.equal(session.view().actions.length, 0);
  assert.equal(session.closeState().blocked, true);
  session.undo();
  const oversized = clone(physical);
  oversized.rows = Array.from({ length: client.fieldListLimits.rows + 1 }, () => clone(physical.rows[0]));
  assert.throws(() => client.fieldListTable(oversized), /oversized/);
  assert.throws(() => client.fromWire(' '.repeat(client.fieldListLimits.jsonUnits + 1)), /oversized/);
});

test('disabled direct controller never enables writes and has no generated binding dependencies', async () => {
  const session = new client.SummaryVegetationFieldListSession(owner, { read: () => Promise.resolve(wire.review),
    save: () => { throw new Error('disabled dispatch'); } });
  await session.load();
  assert.throws(() => session.stageAll(), /Enabled/);
  await assert.rejects(session.save(), /disabled/);
  const source = fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldList.ts'), 'utf8');
  assert.doesNotMatch(source, /structuredClone\s*\(|bindings\/|bindings\\|Service\./);
});

test('exact JSON request/outcome keys and duplicates are rejected before typed result acceptance', () => {
  const request = client.fieldListRequest(wire.request, owner);
  for (const json of [
    JSON.stringify(wire.request).replace('"kind":"all"', '"kind":"none","\\u006bind":"all"'),
    JSON.stringify(wire.request).replace('"requestId":', '"RequestId":'),
    JSON.stringify(wire.request).replace('"selected":true', '"selected":null'),
  ]) assert.throws(() => client.fieldListRequest(json, owner));
  for (const json of [
    JSON.stringify(wire.outcome).replace('"committed":true', '"committed":false,"\\u0063ommitted":true'),
    JSON.stringify(wire.outcome).replace('"proposed":', '"original":null,"proposed":'),
    JSON.stringify(wire.outcome).replace('"errorMessage":""', '"errorMessage":"\\udfff"'),
  ]) assert.throws(() => client.validateFieldListOutcome(json, request));
  assert.deepEqual(plain(client.validateFieldListOutcome(JSON.stringify(wire.outcome), request)), wire.outcome);
});

test('known rejected precommit outcome retains draft/attempt until explicit Load correction without commit invention', async () => {
  const session = await ready(undefined, json => {
    const request = JSON.parse(json);
    return Promise.resolve({ contextId: owner.contextId, requestId: request.requestId,
      changed: false, committed: false, replayed: false, verified: false, plan: null, errorMessage: 'witness changed' });
  });
  session.stageAll();
  assert.equal(await session.save(), true);
  assert.equal(session.view().receipts[0].kind, 'known');
  assert.equal(session.view().receipts[0].outcome.committed, false);
  assert.equal(session.view().actions.length, 1);
  assert.throws(() => session.acknowledge());
  session.undo();
  await session.load(); session.correctReceipt(); session.acknowledge();
  assert.equal(session.view().receipts[0].outcome.plan, null);
});

test('a failed later Load invalidates previously loaded correction authority without erasing receipts', async () => {
  let rejected = false;
  const session = await ready(() => rejected ? Promise.reject(new Error('new read failed')) : Promise.resolve(wire.review));
  session.stageAll(); await session.save();
  assert.equal(await session.load(), true);
  rejected = true; assert.equal(await session.load(), false);
  assert.throws(() => session.correctReceipt());
  assert.throws(() => session.acknowledge());
  assert.equal(session.view().receipts.length, 1);
  rejected = false; await session.load(); session.correctReceipt(); session.acknowledge();
});

test('retained attempt capacity blocks new Save without dropping acknowledged receipt evidence', async () => {
  let attempts = 0, generated = 0;
  const session = await ready(undefined, json => {
    attempts++;
    const request = JSON.parse(json);
    return Promise.resolve({ contextId: owner.contextId, requestId: request.requestId,
      changed: false, committed: false, replayed: false, verified: true, errorMessage: '',
      plan: { original: clone(physical), proposed: clone(physical), assignments: [] } });
  }, () => { generated++; return randomUUID(); });
  for (let i = 0; i < client.fieldListLimits.receipts; i++) {
    session.stageRow('1', false); assert.equal(await session.save(), true); session.acknowledge();
  }
  session.stageRow('1', false);
  await assert.rejects(session.save(), /capacity/);
  assert.equal(session.view().receipts.length, client.fieldListLimits.receipts);
  assert.equal(attempts, client.fieldListLimits.receipts);
  assert.equal(generated, attempts);
});

test('synchronous lifecycle cancellation before dispatch cannot launch stale Load or Save', async () => {
  let reads = 0, saves = 0;
  const session = await ready(() => { reads++; return Promise.resolve(wire.review); },
    () => { saves++; return Promise.resolve(wire.outcome); });
  let cancelNext = true;
  const unsubscribe = session.subscribe(() => {
    if (session.view().busy && cancelNext) { cancelNext = false; session.cancel(); }
  });
  assert.equal(await session.load(), false);
  assert.equal(reads, 1);
  await session.load();
  session.stageAll(); cancelNext = true;
  assert.equal(await session.save(), false);
  assert.equal(saves, 0);
  assert.equal(session.view().receipts.length, 0);
  assert.equal(session.closeState().busy, false);
  unsubscribe();
});

for (const [drift, change] of [
  ['removed row', review => { review.physical.rows.shift(); }],
  ['hidden Order', review => { review.physical.rows[0].cells[0].integer = '0'; }],
  ['NULL Report', review => {
    review.physical.rows[0].cells[4] = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  }],
  ['noncalibrated Report', review => { review.physical.rows[0].cells[4].integer = '1'; }],
]) for (const firstOutcome of ['unknown delivery', 'known rejection']) {
  test(`retained row draft stays attached through owned ${drift} Load and subscriber rendering (${firstOutcome})`, async () => {
    let current = clone(wire.review);
    const requests = [];
    const session = await ready(() => Promise.resolve(proxy(clone(current))), json => {
      const request = JSON.parse(json); requests.push(request);
      if (requests.length === 1) {
        if (firstOutcome === 'unknown delivery') return Promise.reject(new Error('response lost'));
        return Promise.resolve({ contextId: owner.contextId, requestId: request.requestId,
          changed: false, committed: false, replayed: false, verified: false, plan: null, errorMessage: 'witness changed' });
      }
      assert.deepEqual(request.review, current);
      assert.deepEqual(request.actions, [{ kind: 'row', rowId: '2', selected: true }]);
      const original = clone(current.physical), proposed = clone(current.physical);
      const before = original.rows.find(row => row.rowId === '2'), after = proposed.rows.find(row => row.rowId === '2');
      after.cells[4] = { storage: 'integer', text: null, integer: '-1', real: null, blobHex: null };
      return Promise.resolve({ contextId: owner.contextId, requestId: request.requestId,
        changed: true, committed: true, replayed: false, verified: true, errorMessage: '',
        plan: { original, proposed, assignments: [{ before, after }] } });
    });
    const notifications = [];
    const unsubscribe = session.subscribe(() => {
      notifications.push(plain(session.view()));
      session.closeState();
    });
    assert.equal(session.stageRow('1', true), true);
    assert.equal(await session.save(), firstOutcome === 'known rejection');
    const retained = plain(session.view().receipts[0]);
    const historicalProposed = plain(session.view().proposed);
    const changedAuthority = clone(wire.review); change(changedAuthority); current = changedAuthority;
    assert.equal(await session.load(), true);
    const pending = session.view();
    assert.deepEqual(plain(pending.review), wire.review);
    assert.deepEqual(plain(pending.proposed), historicalProposed);
    assert.deepEqual(plain(pending.actions), [{ kind: 'row', rowId: '1', selected: true }]);
    assert.deepEqual(plain(pending.reconciliationReview), changedAuthority);
    assert.ok(Object.isFrozen(pending.reconciliationReview.physical.rows[0].cells[4]));
    assert.deepEqual(plain(pending.receipts[0]), retained);
    assert.equal(pending.readError, '');
    assert.equal(session.closeState().blocked, true);
    assert.throws(() => session.acknowledge());
    await assert.rejects(session.save());
    const afterLoadNotification = notifications.at(-1);
    assert.deepEqual(afterLoadNotification.review, wire.review);
    assert.deepEqual(afterLoadNotification.reconciliationReview, changedAuthority);
    session.correctReceipt();
    const corrected = session.view();
    assert.deepEqual(plain(corrected.review), changedAuthority);
    assert.deepEqual(plain(corrected.proposed), changedAuthority.physical);
    assert.equal(corrected.reconciliationReview, null);
    assert.deepEqual(plain(corrected.actions), []);
    assert.deepEqual(plain(corrected.receipts[0].request), retained.request);
    assert.deepEqual(plain(corrected.receipts[0].outcome), retained.outcome);
    assert.deepEqual(plain(corrected.receipts[0].reconciliation), changedAuthority);
    session.acknowledge();
    assert.equal(session.closeState().blocked, false);
    assert.equal(session.stageRow('1', true), false);
    assert.equal(session.closeState().blocked, true);
    assert.equal(session.stageRow('2', true), true);
    assert.equal(await session.save(), true);
    session.acknowledge();
    assert.equal(requests.length, 2);
    assert.notEqual(requests[0].requestId, requests[1].requestId);
    assert.deepEqual(requests[0].review, wire.review);
    assert.deepEqual(requests[0].actions, [{ kind: 'row', rowId: '1', selected: true }]);
    assert.deepEqual(plain(session.view().receipts[0].request), retained.request);
    assert.equal(session.view().receipts[0].kind, firstOutcome === 'known rejection' ? 'known' : 'unknown');
    assert.equal(session.view().receipts[0].acknowledged, true);
    assert.equal(session.view().receipts[1].outcome.committed, true);
    assert.equal(session.closeState().blocked, false);
    unsubscribe();
  });
}

test('successful committed receipt cannot acknowledge past a separately loaded drift observation', async () => {
  let current = clone(wire.review);
  const session = await ready(() => Promise.resolve(current), json => Promise.resolve(commitFor(JSON.parse(json))));
  const views = [];
  const unsubscribe = session.subscribe(() => { views.push(plain(session.view())); });
  session.stageAll(); assert.equal(await session.save(), true);
  const retainedReceipt = plain(session.view().receipts[0]);
  current = clone(wire.after); current.physical.rows.shift();
  assert.equal(await session.load(), true);
  assert.deepEqual(plain(session.view().review), wire.after);
  assert.deepEqual(plain(session.view().reconciliationReview), current);
  assert.throws(() => session.acknowledge(), /reconciliation/);
  assert.equal(session.closeState().blocked, true);
  session.correctReceipt(); session.acknowledge();
  assert.deepEqual(plain(session.view().review), current);
  assert.equal(views.at(-1).reconciliationReview, null);
  assert.deepEqual(plain(session.view().receipts[0].outcome), retainedReceipt.outcome);
  assert.deepEqual(plain(session.view().receipts[0].request), retainedReceipt.request);
  assert.equal(session.view().receipts[0].outcome.committed, true);
  assert.equal(session.closeState().blocked, false);
  unsubscribe();
});
