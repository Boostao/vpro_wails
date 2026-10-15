const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const vm = require('node:vm');
const client = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { TwoPageVegetationSession, wire, cell } = require('./twoPageVegetationTestHelpers.cjs');
const fixtures = require('./twoPageVegetationReceipts.json');
const outcomes = require('./twoPageVegetationOutcomes.json');
const sample = () => structuredClone(fixtures[3]);
const request = sample().history.Request;
const { contextId, project, plot, form } = request.original;
const owner = { contextId, project, plot, form };
const enabled = { VITE_TWO_PAGE_VEGETATION_REVIEW: 'true', VITE_TWO_PAGE_VEGETATION_WRITING: 'true' };
function registry(environment = {}, overrides = {}) {
  const calls = [], dependencies = {
    '../bindings/github.com/boostao/vpro-wails': { TwoPageVegetationService: {
      GetOriginal: (...args) => { calls.push(['read', ...args]); return Promise.resolve(client.proxy(sample().history.Request.original)); },
      Save: (...args) => { calls.push(['save', ...args]); return Promise.resolve(client.proxy(sample())); },
      RecoverOutcome: (...args) => { calls.push(['outcome', ...args]); return Promise.resolve(client.proxy(structuredClone(outcomes[3]))); },
      GetSpeciesReferences: (...args) => {
        calls.push(['references', ...args]);
        const wire = structuredClone(require('./twoPageVegetationReferenceWire.json'));
        Object.assign(wire, owner);
        return Promise.resolve(client.proxy(wire));
      },
      ...overrides,
    }, TwoPageVegetationSpeciesListService: {
      Save: (...args) => { calls.push(['species-list-save', ...args]); return Promise.reject(new Error('Held independent Species transport failure')); },
    } },
    './twoPageVegetationSession': { TwoPageVegetationSession },
    './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
  };
  const module = { exports: {} };
  const source = readFileSync(path.join(__dirname, 'twoPageVegetationSessions.ts'), 'utf8').replaceAll('import.meta.env', '__environment');
  vm.runInNewContext(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, { module, exports: module.exports, __environment: environment, require: name => {
    if (!(name in dependencies)) throw new Error(`Unexpected vegetation registry dependency ${name}`);
    return dependencies[name];
  } });
  return { ...module.exports, calls };
}
function stage(session) {
  session.stage('-7', 'Cover1', '5', false);
  session.stage('-7', 'Cover2', '', true);
  session.stage('-7', 'Collected', 'C', false);
}

test('every review/writing flag combination stays independent with actual default-off RPC dispatch', async () => {
  for (const review of ['true', 'false', undefined]) {
    for (const writing of ['true', 'false', undefined]) {
      const api = registry({ VITE_TWO_PAGE_VEGETATION_REVIEW: review, VITE_TWO_PAGE_VEGETATION_WRITING: writing });
      assert.equal(api.twoPageVegetationReviewEnabled, review === 'true');
      assert.equal(api.twoPageVegetationWritingEnabled, review === 'true' && writing === 'true');
      if (review !== 'true') {
        assert.throws(() => api.twoPageVegetationSession(owner), /disabled/);
        assert.equal(api.calls.length, 0);
      } else {
        const session = api.twoPageVegetationSession(owner);
        assert.equal(await session.load(), true);
        if (writing !== 'true') {
          assert.throws(() => stage(session), /disabled/);
          await assert.rejects(session.save(request.requestId), /disabled/);
          assert.equal(api.releaseTwoPageVegetationSession(owner), true);
          assert.equal(api.calls.length, 1);
        }
      }
    }
  }
});

test('literal tuple ownership preserves case whitespace delimiters contexts and every source form', () => {
  const api = registry(enabled), first = api.twoPageVegetationSession(client.proxy({ ...owner }));
  assert.equal(api.twoPageVegetationSession({ ...owner }), first);
  for (const changed of [
    { ...owner, contextId: contextId + ' ' }, { ...owner, project: 'sample' },
    { ...owner, plot: plot + ' ' }, { ...owner, form: 'SubVegAXL_BC' },
    { ...owner, form: 'SubVegCXL' }, { ...owner, form: 'SubVegDXL' },
    { ...owner, contextId: 'a|b', project: 'c' }, { ...owner, contextId: 'a', project: 'b|c' },
  ]) assert.notEqual(api.twoPageVegetationSession(changed), first);
  for (const invalid of [
    { ...owner, contextId: '' }, { ...owner, project: '\ud800' }, { ...owner, plot: null },
    { ...owner, plot: 'P\0' }, { ...owner, form: 'USysVegA' },
  ]) assert.throws(() => api.twoPageVegetationSession(invalid), /literal/);
  assert.equal(api.calls.length, 0);
});

test('all27 Species review/writing/selection combinations keep the new dispatch independently unavailable by default', async () => {
  for (const review of ['true', 'false', undefined]) {
    for (const writing of ['true', 'false', undefined]) {
      for (const selection of ['true', 'false', undefined]) {
        const api = registry({ VITE_TWO_PAGE_VEGETATION_REVIEW: review, VITE_TWO_PAGE_VEGETATION_WRITING: writing,
          VITE_TWO_PAGE_VEGETATION_SPECIES_LIST: selection }, {
          GetSpeciesReferences: () => {
            const refs = structuredClone(require('./twoPageVegetationSpeciesListReceipts.json')[7].history.Request.speciesList);
            Object.assign(refs, owner);
            return Promise.resolve(client.proxy(refs));
          },
        });
        const enabled = review === 'true' && writing === 'true' && selection === 'true';
        assert.equal(api.twoPageVegetationSpeciesListEnabled, enabled);
        if (review !== 'true') {
          assert.throws(() => api.twoPageVegetationSession(owner), /disabled/);
          assert.equal(api.calls.length, 0);
          continue;
        }
        const session = api.twoPageVegetationSession(owner);
        await session.load(); await session.loadReferences();
        if (!enabled) {
          assert.throws(() => session.stage('-7', 'Species', 'A', false), /disabled|independent gate/);
          assert.equal(api.calls.filter(call => call[0].includes('save')).length, 0);
          continue;
        }
        session.stage('-7', 'Species', 'A', false);
        assert.equal(await session.save(request.requestId), false);
        assert.equal(api.calls.filter(call => call[0] === 'save').length, 0);
        const dispatched = api.calls.filter(call => call[0] === 'species-list-save');
        assert.equal(dispatched.length, 1);
        assert.equal(dispatched[0][1], contextId);
        assert.ok(JSON.parse(dispatched[0][2]).speciesList);
        assert.equal(session.closeState().blocked, true);
      }
    }
  }
});

test('readonly Species facade uses exact retained owner and aggregate operation/remount barriers independently of writing', async () => {
  let release;
  const api = registry({ VITE_TWO_PAGE_VEGETATION_REVIEW: 'true' }, {
    GetSpeciesReferences: (...args) => {
      assert.deepEqual(args, [contextId, plot, form]);
      return new Promise(resolve => { release = resolve; });
    },
  });
  const retained = api.twoPageVegetationSession(owner);
  const pending = retained.loadReferences();
  assert.equal(api.twoPageVegetationActivity().busy, true);
  assert.equal(api.twoPageVegetationActivity(owner).busy, false);
  assert.throws(() => api.releaseTwoPageVegetationSession(owner), /operations/);
  assert.equal(api.twoPageVegetationSession({ ...owner }), retained);
  const wire = structuredClone(require('./twoPageVegetationReferenceWire.json'));
  Object.assign(wire, owner);
  release(client.proxy(wire));
  assert.equal(await pending, true);
  assert.equal(api.twoPageVegetationActivity().busy, false);
  assert.equal(api.twoPageVegetationActivity().blocked, false);
  assert.equal(retained.view().references.form, form);
  assert.equal(api.releaseTwoPageVegetationSession(owner), true);
});

test('retained raw invalid drafts survive subscription remount and aggregate guards protect their exact owner', async () => {
  const api = registry(enabled);
  let observed;
  const stop = api.subscribeTwoPageVegetationActivity(value => { observed = value; });
  const mutableOwner = client.proxy({ ...owner });
  const session = api.twoPageVegetationSession(mutableOwner);
  mutableOwner.contextId = 'intruder';
  await session.load();
  assert.deepEqual(api.calls[0], ['read', contextId, plot, form]);
  session.stage('-7', 'Cover1', '100', false);
  assert.equal(observed.blocked, true);
  assert.deepEqual({ ...observed.owner }, owner);
  observed.owner.contextId = 'external observer mutation';
  assert.equal(api.twoPageVegetationActivity().owner.contextId, contextId);
  assert.equal(api.twoPageVegetationSession(owner), session);
  assert.equal(Object.values(session.view().drafts)[0].raw, '100');
  assert.throws(() => api.releaseTwoPageVegetationSession(owner), /Resolve/);
  session.undo();
  assert.equal(observed.blocked, false);
  assert.equal(api.releaseTwoPageVegetationSession(owner), true);
  assert.equal(api.releaseTwoPageVegetationSession(owner), false);
  assert.notEqual(api.twoPageVegetationSession(owner), session);
  stop();
});

test('generated production service calls use exact captured owners and stable JSON without replay after recovery', async () => {
  const api = registry(enabled, { Save: (...args) => { api.calls.push(['save', ...args]); return Promise.reject(new Error('unknown')); } });
  const session = api.twoPageVegetationSession(owner);
  await session.load(); stage(session);
  assert.equal(await session.save(request.requestId), false);
  assert.equal(api.twoPageVegetationActivity().blocked, true);
  assert.throws(() => api.releaseTwoPageVegetationSession(owner), /Resolve/);
  assert.equal(await session.resolve(), true);
  assert.deepEqual(api.calls.find(value => value[0] === 'save'), ['save', contextId, wire.serializeTwoPageVegetationRequest(request)]);
  assert.deepEqual(api.calls.find(value => value[0] === 'outcome'), ['outcome', contextId, wire.serializeTwoPageVegetationRequest(request)]);
  assert.equal(api.twoPageVegetationActivity().blocked, true);
  session.acknowledge();
  assert.equal(api.twoPageVegetationActivity().blocked, false);
  assert.equal(api.releaseTwoPageVegetationSession(owner), false);
  assert.equal(api.twoPageVegetationSession(owner), session);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(api.calls.filter(value => value[0] === 'save').length, 1);
});

test('separate owners aggregate dirty and busy states across unmounted subscriptions without borrowing another original', async () => {
  let release;
  const held = new Promise(resolve => { release = resolve; }); held.cancel = () => {};
  const otherOwner = { ...owner, plot: '0108051' };
  const otherOriginal = sample().history.Request.original;
  otherOriginal.plot = otherOwner.plot;
  for (const row of otherOriginal.records.rows) row.cells[1].text = otherOwner.plot;
  const api = registry(enabled, { GetOriginal: (_context, value) =>
    value === otherOwner.plot ? held : Promise.resolve(sample().history.Request.original) });
  const first = api.twoPageVegetationSession(owner), other = api.twoPageVegetationSession(otherOwner);
  await first.load();
  first.stage('-7', 'Cover1', '100', false);
  const loading = other.load();
  let previous = 0;
  const unsubscribe = api.subscribeTwoPageVegetationActivity(() => { previous++; }); unsubscribe();
  let current;
  const remount = api.subscribeTwoPageVegetationActivity(value => { current = value; });
  assert.equal(current.busy, true);
  assert.equal(current.blocked, true);
  assert.deepEqual({ ...current.owner }, owner);
  first.undo();
  assert.equal(current.busy, true);
  assert.equal(current.blocked, false);
  assert.deepEqual({ ...current.owner }, otherOwner);
  assert.equal(previous, 1);
  assert.throws(() => api.releaseTwoPageVegetationSession(otherOwner), /Resolve/);
  release(client.proxy(otherOriginal));
  assert.equal(await loading, true);
  assert.equal(current.busy, false);
  assert.equal(current.owner, null);
  assert.equal(api.twoPageVegetationSession(otherOwner), other);
  remount();
});

test('owner-excluding activity keeps correction live without allowing another owner to edit an overlapping source', async () => {
  const api = registry(enabled), session = api.twoPageVegetationSession(owner);
  await session.load(); session.stage('-7', 'Cover1', '100', false);
  assert.equal(api.twoPageVegetationActivity().blocked, true);
  assert.equal(api.twoPageVegetationActivity(owner).blocked, false);
  assert.equal(api.twoPageVegetationActivity({ ...owner, form: 'SubVegCXL' }).blocked, true);
  assert.throws(() => api.twoPageVegetationActivity({ ...owner, contextId: '' }), /literal/);
  session.undo();
  assert.equal(api.twoPageVegetationActivity({ ...owner, form: 'SubVegCXL' }).blocked, false);
});

test('signed-zero physical peers reach the actual service port without normalizing request bytes', async () => {
  const original = sample().history.Request.original, column = wire.vegetationColumnIndex(original, 'Cover3');
  original.records.rows[1].cells[column] = cell('real', -0);
  const api = registry(enabled, {
    GetOriginal: () => Promise.resolve(client.proxy(original)),
    Save: (...args) => { api.calls.push(['save', ...args]); return Promise.reject(new Error('unknown')); },
    RecoverOutcome: (...args) => {
      api.calls.push(['outcome', ...args]);
      return Promise.resolve({ ...structuredClone(outcomes[2]), requestJSON: args[1] });
    },
  });
  const session = api.twoPageVegetationSession(owner);
  await session.load(); stage(session); await session.save(request.requestId);
  assert.equal(await session.resolve(), true);
  const dispatched = api.calls.find(value => value[0] === 'save');
  assert.equal(dispatched[1], contextId);
  assert.notEqual(dispatched[2], JSON.stringify(JSON.parse(dispatched[2])));
  assert.equal(api.calls.find(value => value[0] === 'outcome')[2], dispatched[2]);
  assert.equal(session.view().noCommitCertificate.requestId, request.requestId);
});
