const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const vm = require('node:vm');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { TwoPageSoilSession, fixture, original, wire, outcome } = require('./twoPageSoilTestHelpers.cjs');
function registry(environment = {}, overrides = {}) {
  const calls = [];
  const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
  const dependencies = {
    '../bindings/github.com/boostao/vpro-wails': { TwoPageSoilService: {
      GetOriginal: (...args) => { calls.push(['read', ...args]); return Promise.resolve(original()); },
      Save: (...args) => { calls.push(['save', ...args]); return Promise.resolve(wire()); },
      RecoverOutcome: (...args) => { calls.push(['receipt', ...args]); return Promise.resolve(outcome(args[1])); },
      ...overrides,
    } },
    './twoPageSoilSession': { TwoPageSoilSession }, './qualityEditor': quality,
  };
  const module = { exports: {} };
  const source = readFileSync(path.join(__dirname, 'twoPageSoilSessions.ts'), 'utf8').replaceAll('import.meta.env', '__environment');
  vm.runInNewContext(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, { module, exports: module.exports, __environment: environment, require: name => {
    if (!(name in dependencies)) throw new Error(`Unexpected registry dependency ${name}`);
    return dependencies[name];
  } });
  return { ...module.exports, calls };
}
const enabled = { VITE_TWO_PAGE_SOIL_REVIEW: 'true', VITE_TWO_PAGE_SOIL_WRITING: 'true' };
const owner = fixture.owner;
function stage(session) {
  session.stage('-7', 'Horizon', 'LF', false);
  session.stage('-7', 'UpperDepth', '5', false);
  session.stage('-7', 'Comment', '', true);
}

test('default and writing-only cannot grant review; read-only cannot stage or dispatch mutation', async () => {
  for (const environment of [{}, { VITE_TWO_PAGE_SOIL_REVIEW: 'false' }, { VITE_TWO_PAGE_SOIL_WRITING: 'true' }]) {
    const api = registry(environment);
    assert.equal(api.twoPageSoilReviewEnabled, false);
    assert.equal(api.twoPageSoilWritingEnabled, false);
    assert.throws(() => api.twoPageSoilSession(owner), /disabled/);
    assert.equal(api.calls.length, 0);
  }
  const api = registry({ VITE_TWO_PAGE_SOIL_REVIEW: 'true' });
  const session = api.twoPageSoilSession(owner);
  assert.equal(await session.load(), true);
  assert.throws(() => session.stage('-7', 'Horizon', 'LF', false), /disabled/);
  await assert.rejects(session.save(fixture.request.requestId), /disabled/);
  assert.equal(session.view().blocked, false);
  assert.equal(api.calls.length, 1);
  assert.equal(api.releaseTwoPageSoilSession(owner), true);
});

test('literal owner keys distinguish contexts/projects/parents/forms without trimming or delimiter collisions', () => {
  const api = registry(enabled);
  const first = api.twoPageSoilSession(owner);
  assert.equal(api.twoPageSoilSession({ ...owner }), first);
  for (const changed of [
    { ...owner, contextId: owner.contextId + ' ' }, { ...owner, project: 'Sample|suffix' },
    { ...owner, plot: owner.plot + ' ' }, { ...owner, form: 'SoilMineralXL' },
    { ...owner, contextId: 'a|b', project: 'c' }, { ...owner, contextId: 'a', project: 'b|c' },
  ]) assert.notEqual(api.twoPageSoilSession(changed), first);
  for (const invalid of [
    { ...owner, contextId: '' }, { ...owner, plot: null }, { ...owner, project: '\ud800' },
    { ...owner, plot: 'P\0' }, { ...owner, form: 'USysHumus' },
  ]) assert.throws(() => api.twoPageSoilSession(invalid), /literal/);
  assert.equal(api.calls.length, 0);
});

test('draft/remount and aggregate lifecycle activity retain exact owner, raw errors and clean-session release', async () => {
  const api = registry(enabled);
  let activity;
  const stop = api.subscribeTwoPageSoilActivity(value => { activity = value; });
  const session = api.twoPageSoilSession(owner);
  await session.load();
  assert.deepEqual(api.calls[0], ['read', owner.contextId, owner.plot, owner.form]);
  session.stage('-7', 'Horizon', '123456789', false);
  assert.equal(activity.blocked, true);
  assert.deepEqual({ ...activity.owner }, owner);
  assert.equal(api.twoPageSoilSession(owner), session);
  assert.equal(Object.values(api.twoPageSoilSession(owner).view().drafts)[0].raw, '123456789');
  assert.throws(() => api.releaseTwoPageSoilSession(owner), /Resolve/);
  session.undo();
  assert.equal(activity.blocked, false);
  assert.equal(api.releaseTwoPageSoilSession(owner), true);
  assert.notEqual(api.twoPageSoilSession(owner), session);
  stop();
});

test('actual Save and RecoverOutcome retain exact JSON/argument order; acknowledgement cannot erase receipt-owning registry entry', async () => {
  const api = registry(enabled, { Save: () => Promise.reject(new Error('unknown delivery')) });
  const session = api.twoPageSoilSession(owner);
  await session.load(); stage(session);
  assert.equal(await session.save(fixture.request.requestId), false);
  assert.equal(api.twoPageSoilActivity().blocked, true);
  assert.throws(() => api.releaseTwoPageSoilSession(owner), /Resolve/);
  assert.equal(await session.resolve(), true);
  const recovery = api.calls.find(call => call[0] === 'receipt');
  assert.equal(recovery[1], owner.contextId);
  assert.deepEqual(JSON.parse(recovery[2]), fixture.request);
  assert.equal(api.twoPageSoilActivity().blocked, true);
  session.acknowledge();
  assert.equal(api.twoPageSoilActivity().blocked, false);
  assert.equal(api.releaseTwoPageSoilSession(owner), false);
  assert.equal(api.twoPageSoilSession(owner), session);
  assert.equal(session.view().receipts.length, 1);
});

test('multiple owners aggregate independently and resubscription observes a still-busy retained owner', async () => {
  let release;
  const held = new Promise(resolve => { release = resolve; }); held.cancel = () => {};
  const otherOwner = { ...owner, plot: '0108051' };
  const otherOriginal = original();
  otherOriginal.plot = otherOwner.plot;
  for (const row of otherOriginal.records.rows) row.cells[1].text = otherOwner.plot;
  const api = registry(enabled, { GetOriginal: (_context, plot) =>
    plot === otherOwner.plot ? held : Promise.resolve(original()) });
  const first = api.twoPageSoilSession(owner), other = api.twoPageSoilSession(otherOwner);
  await first.load();
  first.stage('-7', 'Horizon', '123456789', false);
  const loading = other.load();
  let oldNotifications = 0;
  const stop = api.subscribeTwoPageSoilActivity(() => { oldNotifications++; });
  stop();
  let current;
  const remount = api.subscribeTwoPageSoilActivity(value => { current = value; });
  assert.equal(current.busy, true);
  assert.equal(current.blocked, true);
  assert.deepEqual({ ...current.owner }, owner);
  first.undo();
  assert.equal(current.busy, true);
  assert.equal(current.blocked, false);
  assert.deepEqual({ ...current.owner }, otherOwner);
  assert.equal(oldNotifications, 1);
  release(otherOriginal);
  assert.equal(await loading, true);
  assert.equal(current.busy, false);
  assert.equal(current.owner, null);
  assert.equal(api.twoPageSoilSession(otherOwner), other);
  remount();
});

test('enabled registry invokes production Save with the same retained complete strict-Go wire', async () => {
  const api = registry(enabled);
  const session = api.twoPageSoilSession(owner);
  await session.load(); stage(session);
  assert.equal(await session.save(fixture.request.requestId), true);
  const dispatched = api.calls.find(call => call[0] === 'save');
  assert.equal(dispatched[1], owner.contextId);
  assert.deepEqual(JSON.parse(dispatched[2]), fixture.request);
  assert.equal(api.calls.filter(call => call[0] === 'save').length, 1);
});
