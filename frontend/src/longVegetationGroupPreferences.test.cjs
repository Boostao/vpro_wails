const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', { './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations });
const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const prefs = loadTypeScript('longVegetationGroupPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationGroupWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values; delete scope.availability;
const all = { layer: true, strata: true, lifeform: true, none: true };
const client = { strata: true, lifeform: true };
const review = (groupBy = 1, availability = all, owner = scope) => ({ ...owner, values: { groupBy }, availability });
const outcome = (committed = true, errorMessage = '') => ({ contextId: scope.contextId, changed: committed, committed, errorMessage });
const ready = (groupBy = 1, availability = all, frontend = client) => {
  const session = new prefs.LongVegetationGroupPreferencesSession(scope, frontend); session.apply(review(groupBy, availability)); return session;
};
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const child = () => serverComponent(source('LongVegetationGroupPreferences.svelte'), 'LongVegetationGroupPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': prefs,
});

test('strict-Go grouping wire detaches actual reactive proxies before transport', async () => {
  const reactive = proxy(structuredClone(wire.review)); assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationGroupPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.groupBy = 4; reactive.availability.strata = false;
  assert.equal(detached.values.groupBy, 1); assert.equal(detached.availability.strata, true);
  const session = ready(); session.editGroup('2');
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all grouping transitions distinguish saved validity, backend and client availability', async () => {
  for (let backend = 0; backend < 8; backend++) for (let frontend = 0; frontend < 4; frontend++) {
    const available = { layer: true, strata: !!(backend & 1), lifeform: !!(backend & 2), none: !!(backend & 4) };
    const flags = { strata: !!(frontend & 1), lifeform: !!(frontend & 2) };
    for (let old = 1; old <= 4; old++) for (let proposed = 1; proposed <= 4; proposed++) {
      const session = ready(old, available, flags); session.editGroup(String(proposed)); let dispatches = 0;
      const permitted = proposed === old || proposed === 1 ||
        proposed === 2 && !!(backend & 1) && !!(frontend & 1) ||
        proposed === 3 && !!(backend & 2) && !!(frontend & 2) || proposed === 4 && !!(backend & 4);
      assert.equal(session.view().draftError === '', permitted);
      const save = () => session.save(request => {
        dispatches++; assert.deepEqual(request, { expected: { groupBy: old }, proposed: { groupBy: proposed } });
        return Promise.resolve(outcome(old !== proposed));
      });
      if (permitted) { await save(); assert.equal(dispatches, 1); } else { await assert.rejects(save()); assert.equal(dispatches, 0); }
      session.acknowledge(); session.undo(); assert.equal(session.view().draftError, '');
    }
  }
});
test('missing malformed or foreign saved grouping/capabilities do not infer defaults', async () => {
  const session = new prefs.LongVegetationGroupPreferencesSession(scope, client);
  assert.deepEqual(session.view().draft, { groupBy: '' }); assert.equal(session.availability(), null);
  assert.throws(() => session.editGroup('1'), /Load/); await assert.rejects(session.save(async () => outcome()), /Load/);
  for (const value of [null, {}, review(null), ...[0, 5, '1', true, NaN, Infinity].map(value => review(value)),
    review(1, null), ...Object.keys(all).map(key => review(1, { ...all, [key]: 'true' })),
    review(1, { ...all, layer: false }), ...Object.keys(scope).map(key => review(1, all, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationGroupPreferences(value, scope), /no defaults applied/);
  }
});
test('raw invalid grouping drafts persist and block dispatch while correction and Undo stay live', async () => {
  const session = ready(); let calls = 0;
  for (const raw of ['', ' ', '01', '1.0', '1e0', '0', '5', 'Layer', '\ud800', null, 1]) {
    session.editGroup(raw); assert.equal(session.view().draft.groupBy, raw); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }));
    session.editGroup('4'); assert.equal(session.view().draftError, ''); session.undo();
    assert.equal(session.view().draft.groupBy, '1');
  }
  assert.equal(calls, 0);
});
test('module-lifetime grouping registry retains exact-owner drafts and immutable client capabilities', () => {
  const flags = { ...client }, registry = prefs.createLongVegetationGroupPreferencesRegistry(flags);
  const owner = { ...scope, contextId: 'group-registry' }, session = registry(owner);
  session.apply(review(1, all, owner)); flags.strata = false; session.editGroup('');
  assert.equal(registry(owner), session); assert.ok(registry(owner).view().draftError);
  session.editGroup('2'); assert.equal(session.view().draftError, '');
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => registry({ ...owner, [key]: 'foreign' }));
  const available = session.availability(); available.strata = false; assert.equal(session.availability().strata, true);
});
test('held Save, stale reads and irreversible known/unknown receipts preserve authority', async () => {
  const session = ready(); session.editGroup('2'); let release, cancelled = 0;
  const save = session.save(() => { const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise; });
  for (const action of [() => session.editGroup('3'), () => session.undo(), () => session.beginLoad(), () => session.acknowledge()]) assert.throws(action);
  release(outcome(true, 'committed; do not replay')); await save; session.acknowledge();
  assert.equal(cancelled, 0); assert.equal(session.view().saved.groupBy, 2);
  session.editGroup('3'); await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); await assert.rejects(session.save(async () => outcome()), /Load/);
  const token = session.beginLoad(); session.cancelLoad();
  assert.equal(session.applyLoaded(review(4, { ...all, strata: false }), token), false);
  assert.equal(session.availability().strata, true);
  session.applyLoaded(review(3), session.beginLoad()); assert.equal(session.view().receipts.length, 2);
  assert.equal(session.view().authorityUnknown, false);
});
test('actual grouping handlers cancel stale reads and refuse mutation under external holds', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} } });
  let release, cancelled = 0, saves = 0;
  const context = componentFunctions('LongVegetationGroupPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    preferences: session, view: session.view(), busy: false, error: '', generation: 0, operationHeld: false, mutationHeld: false,
    contextId: scope.contextId, reads: new ReadRequests(), onReadBusyChange() {},
    LongVegetationGroupPreferencesService: {
      GetLongVegetationGroupPreferences() { const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise; },
      SaveLongVegetationGroupPreferences() { saves++; return Promise.resolve(outcome()); },
    },
  });
  const pending = context.load(); context.cancel(); release(review(4)); await pending;
  assert.equal(cancelled, 1); assert.equal(session.view().saved.groupBy, 1);
  context.operationHeld = true; await context.save(); assert.equal(saves, 0);
  context.mutationHeld = true; context.edit('2'); assert.equal(session.view().draft.groupBy, '1');
  context.mutationHeld = false; context.edit('bad'); assert.ok(session.view().draftError);
});
test('actual grouping panel preserves source labels, unavailable saved choices and responsive actions', () => {
  const session = ready(4, { layer: true, strata: false, lifeform: false, none: false });
  const rendered = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } }).body;
  for (const label of ['Group by', 'Layer', 'Strata', 'Lifeform', 'None', 'Load saved report grouping', 'Save report grouping', 'Undo report grouping draft']) assert.ok(rendered.includes(label));
  assert.match(rendered, /data-source-control="rptLvGroupBy"/); assert.match(rendered, /value="2" disabled/);
  assert.doesNotMatch(rendered, /value="4" disabled/);
  const parent = source('LongVegetationReport.svelte');
  assert.match(parent, /<script module lang="ts">/); assert.match(parent, /const groupingSession/);
  assert.match(parent, /titleBarrier \|\| statisticsBarrier \|\| switchBarrier \|\| thresholdBarrier \|\| groupBarrier/);
  assert.match(parent, /unsubscribeGroups\?\.\(\)/); assert.match(source('LongVegetationGroupPreferences.svelte'), /minmax\(0, 1fr\)/);
});
test('actual parent module retains grouping validation across remount without recreating its registry', () => {
  let created = 0;
  const registry = prefs.createLongVegetationGroupPreferencesRegistry(client);
  const session = registry(scope); session.apply(review()); session.editGroup('');
  class Publication {
    view() { return { busy: false, blocked: false, error: '', outcome: null, requestedDestination: '' }; }
    subscribe() { return () => {}; }
  }
  const component = serverComponent(source('LongVegetationReport.svelte')
    .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_GROUP_PREFERENCES === 'true'/g, 'true')
    .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false'),
    'LongVegetationReport.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {},
      './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': { createLongVegetationGroupPreferencesRegistry() { created++; return registry; } },
      './LongVegetationGroupPreferences.svelte': { default: child() },
      './longVegetationPreferences': {}, './longVegetationStatisticsPreferences': {},
      './longVegetationSwitchPreferences': {}, './LongVegetationSwitchPreferences.svelte': { default() {} },
      './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
      './longVegetationReport': {}, './longEnvironmentReport': {},
      './vegetationWorkbook': { VegetationWorkbookPublicationSession: Publication },
    });
  for (let mount = 0; mount < 2; mount++) {
    const result = render(component, { props: { ...scope, onBusyChange() {} } }).body;
    assert.match(result, /Choose a supported Group by value/);
    assert.match(result, /data-long-vegetation-group-save="" disabled/);
  }
  assert.equal(created, 1); assert.equal(registry(scope).view().draft.groupBy, '');
});
