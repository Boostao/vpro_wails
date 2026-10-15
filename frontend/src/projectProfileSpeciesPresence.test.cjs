const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { validateSpeciesPresencePreview } = loadTypeScript('projectProfileSpeciesPresence.ts');

const scope = { contextId: 'owned-context', project: 'Sample', table: 'Sample_Profile', su: 'None' };
const shared = JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'testdata', 'profiling-presence-wire.json'), 'utf8'));
const wire = () => structuredClone(shared.response);
const plain = value => JSON.parse(JSON.stringify(value));

test('species-only seven-field wire remains separate from ordered execution and literal input ownership', async () => {
  assert.equal(shared.request.originalRules.columns[1].declaredType, 'TEXT');
  assert.equal(Object.hasOwn(shared.request.originalRules.columns[1], 'type'), false);
  assert.equal(shared.request.projectLump, null);
  assert.deepEqual(plain(await validateSpeciesPresencePreview(wire(), scope)), wire());
  for (const plotNumbers of [[], null]) {
    assert.deepEqual(plain(await validateSpeciesPresencePreview({ ...wire(), plotNumbers }, scope)),
      { ...wire(), plotNumbers: [] });
  }
  const value = wire();
  const result = await validateSpeciesPresencePreview(value, scope);
  value.plotNumbers[0] = 'changed';
  assert.deepEqual(plain(result.plotNumbers), ['108050']);
});

test('species-only validation rejects unknown steps/actions, missing fields and mismatched current scope', async () => {
  for (const input of [null, [], {}, { ...wire(), steps: [] }, { ...wire(), contextId: 'retired' },
    { ...wire(), project: 'Other' }, { ...wire(), table: 'Sample_Profile ' }, { ...wire(), su: 'Other' },
    { ...wire(), mode: 'ordered' }, { ...wire(), totalPlots: -1 }, { ...wire(), totalPlots: 1.5 },
    { ...wire(), totalPlots: Number.MAX_SAFE_INTEGER + 1 }, { ...wire(), totalPlots: Infinity },
    { ...wire(), plotNumbers: {} }, { ...wire(), totalPlots: 0 }]) {
    await assert.rejects(validateSpeciesPresencePreview(input, scope));
  }
  for (const key of Object.keys(wire())) {
    const input = wire();
    delete input[key];
    await assert.rejects(validateSpeciesPresencePreview(input, scope));
  }
  await assert.rejects(validateSpeciesPresencePreview(wire(), { ...scope, contextId: '' }));
});

test('species-only matching identities are literal bounded ASCII, distinct and sorted without repair', async () => {
  for (const plotNumbers of [[''], ['12345678'], ['\0'], ['é'], ['\ud800'], [1],
    ['108050', '108050'], ['8229723', '108050'], ['A', 'a'], ['A', 'A ']]) {
    await assert.rejects(validateSpeciesPresencePreview({ ...wire(), plotNumbers }, scope));
  }
  const plotNumbers = ['0000001', '0000002', '108050'];
  assert.deepEqual(plain((await validateSpeciesPresencePreview({ ...wire(), plotNumbers }, scope)).plotNumbers), plotNumbers);
});

test('species-only validation yields at the exact threshold and owns result bytes across an awaited yield', async () => {
  let yielded = 0;
  const plots = count => Array.from({ length: count }, (_, i) => String(i).padStart(7, '0'));
  await validateSpeciesPresencePreview({ ...wire(), totalPlots: 127, plotNumbers: plots(127) }, scope,
    async () => { yielded++; });
  assert.equal(yielded, 0);
  await validateSpeciesPresencePreview({ ...wire(), totalPlots: 128, plotNumbers: plots(128) }, scope,
    async () => { yielded++; });
  assert.equal(yielded, 1);
  let release;
  const held = new Promise(resolve => { release = resolve; });
  const expected = { ...scope };
  const value = { ...wire(), totalPlots: 129, plotNumbers: plots(129) };
  const pending = validateSpeciesPresencePreview(value, expected, () => held);
  value.plotNumbers[0] = 'changed';
  value.plotNumbers[128] = 'changed';
  value.totalPlots = -1;
  expected.contextId = 'retired';
  release();
  assert.deepEqual(plain(await pending), { ...scope, mode: 'saved-species-only-readonly',
    totalPlots: 129, plotNumbers: plots(129) });
});

function presenceHandler(name, environment) {
  const component = readFileSync(path.join(__dirname, 'ProjectPlotProfileReview.svelte'), 'utf8');
  const declaration = component.match(new RegExp(`(?:async )?function ${name}\\(\\) \\{[\\s\\S]*?\\n  \\}`));
  assert.ok(declaration, `actual ${name} declaration must resolve`);
  // Construct snapshot roots in the Svelte runtime's own object realm.
  return Function('environment', `with (environment) { return (${declaration[0]}); }`)(environment);
}

test('actual species-only handler snapshots a real Svelte client proxy before transport and gates stored-only input', async () => {
  const runtime = await import('svelte/internal/client');
  const review = runtime.proxy({ project: scope.project, table: scope.table, rules: structuredClone(shared.request.originalRules) });
  assert.throws(() => structuredClone(review), /could not be cloned|DataCloneError/);
  let calls = 0;
  const environment = {
    allowPresence: true, busy: false, blocked: false, review, subvarieties: false,
    contextId: scope.contextId, activeSU: scope.su, generation: 0, lump: null,
    result: { ordered: true }, runInput: { ordered: true }, presenceResult: null,
    error: null, success: null, reading: false, running: false,
    $state: { snapshot: runtime.snapshot }, onbusy() {},
    reads: { track: promise => promise },
    client: { PreviewProjectProfileSpeciesPresence(input) {
      calls++;
      assert.deepEqual(structuredClone(input), shared.request);
      return Promise.resolve(wire());
    } },
    validateSpeciesPresencePreview,
  };
  const run = presenceHandler('runPresence', environment);
  await run();
  assert.equal(calls, 1);
  assert.equal(environment.error, null);
  assert.deepEqual(plain(environment.presenceResult), wire());
  assert.equal(environment.result, null);
  assert.equal(environment.runInput, null);
  assert.equal(environment.reading, false);
  for (const [key, value] of [['allowPresence', false], ['busy', true], ['blocked', true], ['subvarieties', true]]) {
    const original = environment[key];
    environment[key] = value;
    await run();
    assert.equal(calls, 1, `${key} must prevent a readonly RPC`);
    environment[key] = original;
  }
});

test('actual species-only handler rechecks generation after held validation and cannot populate a remounted operation', async () => {
  let release;
  let entered;
  const hold = new Promise(resolve => { release = resolve; });
  const started = new Promise(resolve => { entered = resolve; });
  const environment = {
    allowPresence: true, busy: false, blocked: false,
    review: { project: scope.project, table: scope.table, rules: shared.request.originalRules },
    subvarieties: false, contextId: scope.contextId, activeSU: scope.su, generation: 0, lump: null,
    result: null, runInput: null, presenceResult: null, error: null, success: null, reading: false, running: false,
    $state: { snapshot: structuredClone }, onbusy() {},
    reads: { track: promise => promise, cancelAll() {} },
    client: { PreviewProjectProfileSpeciesPresence: () => Promise.resolve({
      ...wire(), totalPlots: 128, plotNumbers: Array.from({ length: 128 }, (_, i) => String(i).padStart(7, '0')),
    }) },
    validateSpeciesPresencePreview: (value, expected) => validateSpeciesPresencePreview(value, expected, async () => {
      entered();
      await hold;
    }),
  };
  const pending = presenceHandler('runPresence', environment)();
  await started;
  assert.equal(environment.reading, true);
  presenceHandler('cancelRun', environment)();
  assert.equal(environment.reading, false);
  environment.generation++;
  environment.reading = true;
  environment.running = true;
  environment.presenceResult = 'new-panel-result';
  release();
  await pending;
  assert.equal(environment.presenceResult, 'new-panel-result');
  assert.equal(environment.reading, true);
  assert.equal(environment.running, true);
});
