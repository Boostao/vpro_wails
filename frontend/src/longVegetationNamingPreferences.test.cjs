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
const prefs = loadTypeScript('longVegetationNamingPreferences.ts', {
  './qualityEditor': quality, './reportTitlePreferences': shared, './googleEarthReview': earth,
});
const source = file => readFileSync(path.join(__dirname, file), 'utf8');
const wire = JSON.parse(source('longVegetationNamingWire.json'));
const scope = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.review[key]]));
const available = { none: true, english: true, code: true, selectedField: false };
const review = (mode = 1, selectedField = wire.review.values.selectedField, code = true, owner = scope) =>
  ({ ...owner, values: { mode, selectedField }, availability: { ...available, code } });
const outcome = (changed = true, errorMessage = '', owner = scope) =>
  ({ contextId: owner.contextId, changed, committed: changed, errorMessage });
const ready = (mode = 1, field = wire.review.values.selectedField, code = true, clientCode = true) => {
  const session = new prefs.LongVegetationNamingPreferencesSession(scope, { code: clientCode });
  session.apply(review(mode, field, code)); return session;
};
const child = () => serverComponent(source('LongVegetationNamingPreferences.svelte'), 'LongVegetationNamingPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationNamingPreferences': prefs,
});

test('strict-Go naming wire detaches actual reactive proxies and transports primitive paired values', async () => {
  const reactive = proxy(structuredClone(wire.review)); assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationNamingPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.mode = 4; reactive.values.selectedField = 'foreign'; reactive.availability.code = false;
  assert.deepEqual(structuredClone(detached), wire.review);
  const session = ready(); session.editMode('2');
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all source modes and backend/client Code flags distinguish historical no-op from new selection', async () => {
  for (const backend of [false, true]) for (const frontend of [false, true]) {
    for (const old of [0, 1, 2, 4]) for (const field of ['', ' old\0field ']) {
      const untouched = ready(old, field, backend, frontend);
      await untouched.save(request => {
        assert.deepEqual(request, { expected: { mode: old, selectedField: field }, proposed: { mode: old, selectedField: field } });
        return Promise.resolve(outcome(false));
      });
      for (const next of [0, 1, 2, 4]) {
        const session = ready(old, field, backend, frontend); session.editMode(String(next));
        const same = old === next && (next === 4 || field === '');
        const permitted = same || next !== 4 && (next !== 2 || backend && frontend);
        assert.equal(session.view().draftError === '', permitted, JSON.stringify({ old, field, next, backend, frontend }));
        let calls = 0;
        const save = () => session.save(request => {
          calls++; assert.equal(request.proposed.mode, next);
          assert.equal(request.proposed.selectedField, next === 4 ? field : '');
          return Promise.resolve(outcome(!same));
        });
        if (permitted) { await save(); assert.equal(calls, 1); }
        else { await assert.rejects(save()); assert.equal(calls, 0); }
      }
    }
  }
});
test('staged away/back mode remembers source field clear; untouched Save and Undo retain literal bytes', async () => {
  for (const field of ['', ' RedBlueList ', 'old\0field', '界'.repeat(40)]) {
    const session = ready(1, field); session.editMode('0'); session.editMode('1');
    assert.deepEqual(session.view().draft, { mode: '1', selectedField: '' });
    session.undo(); assert.deepEqual(session.view().draft, { mode: '1', selectedField: field });
    session.editMode('2'); session.editMode('1');
    await session.save(request => {
      assert.deepEqual(request, { expected: { mode: 1, selectedField: field }, proposed: { mode: 1, selectedField: '' } });
      return Promise.resolve(outcome(field !== ''));
    });
  }
});
test('malformed missing foreign reviews never infer Show values or availability', async () => {
  const session = new prefs.LongVegetationNamingPreferencesSession(scope, { code: false });
  assert.deepEqual(session.view().draft, { mode: '', selectedField: '' }); assert.equal(session.availability(), null);
  assert.throws(() => session.editMode('1'), /Load/); await assert.rejects(session.save(async () => outcome()), /Load/);
  for (const value of [null, {}, review(null), ...[-1, 3, 5, '1', true, NaN, Infinity].map(mode => review(mode)),
    review(1, null), review(1, '\ud800'), ...Object.keys(scope).map(key => review(1, '', true, { ...scope, [key]: 'foreign' })),
    ...Object.keys(available).map(key => ({ ...review(), availability: { ...available, [key]: 'true' } })),
    { ...review(), availability: { ...available, english: false } },
    { ...review(), availability: { ...available, selectedField: true } }]) {
    assert.throws(() => prefs.validateLongVegetationNamingPreferences(value, scope), /no defaults applied/);
  }
  assert.throws(() => new prefs.LongVegetationNamingPreferencesSession(scope, {}), /Explicit/);
});
test('raw invalid drafts survive exact-owner remount and keep correction/Undo live without Save dispatch', async () => {
  const flags = { code: true }, registry = prefs.createLongVegetationNamingPreferencesRegistry(flags);
  const session = registry(scope); session.apply(review()); flags.code = false;
  let calls = 0;
  for (const raw of ['', ' ', '01', '1.0', '1e0', '3', '5', 'English', '\ud800', null, 1]) {
    session.editMode(raw); assert.equal(registry(scope), session);
    assert.equal(session.view().draft.mode, raw); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }));
    session.editMode('2'); assert.equal(session.view().draftError, ''); session.undo();
    assert.equal(session.view().draft.selectedField, wire.review.values.selectedField);
  }
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => registry({ ...scope, [key]: 'foreign' }));
  const copy = session.availability(); copy.code = false; assert.equal(session.availability().code, true);
  assert.equal(calls, 0);
});
test('held naming Save cannot be cancelled and warning/unknown receipts preserve authority and clear provenance', async () => {
  const session = ready(); session.editMode('0'); session.editMode('1');
  let release, cancelled = 0;
  const save = session.save(() => {
    const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise;
  });
  for (const action of [() => session.editMode('2'), () => session.undo(), () => session.beginLoad(), () => session.acknowledge()]) assert.throws(action);
  release(outcome(true, 'committed; do not replay')); await save; session.acknowledge();
  assert.equal(cancelled, 0); assert.equal(session.view().saved.selectedField, '');
  session.editMode('2'); await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); await assert.rejects(session.save(async () => outcome()), /Load/);
  const token = session.beginLoad(); session.cancelLoad();
  assert.equal(session.applyLoaded(review(0, '', false), token), false); assert.equal(session.availability().code, true);
  session.applyLoaded(review(2, ''), session.beginLoad()); assert.equal(session.view().receipts.length, 2);
  assert.equal(session.view().authorityUnknown, false);
});
test('actual naming handlers cancel stale reads and separately gate Save versus correction', async () => {
  const session = ready();
  let resolveRead, reads = 0, saves = 0, cancellations = 0;
  const context = componentFunctions('LongVegetationNamingPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    contextId: scope.contextId, preferences: session, view: session.view(), busy: false, operationHeld: false, mutationHeld: false,
    generation: 0, error: '', onReadBusyChange() {},
    reads: { cancelAll() { cancellations++; }, track(value) { return value; } },
    LongVegetationNamingPreferencesService: {
      GetLongVegetationNamingPreferences() { reads++; return new Promise(done => { resolveRead = done; }); },
      SaveLongVegetationNamingPreferences(id, raw) {
        saves++; assert.equal(id, scope.contextId); assert.deepEqual(JSON.parse(raw), wire.request); return Promise.resolve(wire.outcome);
      },
    },
  });
  const pending = context.actions.load(); context.actions.cancel(); resolveRead(review(4, 'foreign')); await pending;
  assert.equal(reads, 1); assert.equal(session.view().saved.mode, 1); assert.ok(cancellations);
  context.operationHeld = true; context.actions.edit('2'); assert.equal(session.view().draft.mode, '2');
  await context.actions.save(); assert.equal(saves, 0);
  context.mutationHeld = true; context.actions.edit('0'); assert.equal(session.view().draft.mode, '2');
  context.operationHeld = false; await context.actions.save(); assert.equal(saves, 1);
});
test('actual naming panel retains exact source captions, read-only field and historical unavailable identity', () => {
  for (const mode of [0, 1, 2, 4]) {
    const session = ready(mode, ' old\0field ', false, false);
    const { body } = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } });
    for (const caption of ['Show english name', 'Show 8 char. code', 'Show none']) assert.ok(body.includes(caption));
    assert.equal((body.match(/data-source-control="rptLvShowCommon"/g) ?? []).length, 1);
    assert.match(body, /for="long-vegetation-naming"/); assert.match(body, /Saved selected field \(read-only\)/);
    assert.match(body, /data-source-control="cmbShowField"/);
    if (mode === 4) assert.match(body, /Saved selected-field mode \(unavailable\)/);
    session.editMode('');
    const invalid = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } }).body;
    assert.match(invalid, /data-long-vegetation-naming-save[^>]*disabled/);
    assert.doesNotMatch(invalid, /data-long-vegetation-naming-undo[^>]*disabled/);
  }
});
test('actual report remount retains naming draft errors and root close barriers behind the independent flag', () => {
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `naming-composition-${enabled}` };
    const registry = prefs.createLongVegetationNamingPreferencesRegistry({ code: true });
    const session = registry(owner); session.apply(review(1, 'old field', true, owner)); session.editMode('');
    const reportSource = source('LongVegetationReport.svelte')
      .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const namingSession = false', `const namingSession = ${enabled}`);
    const component = serverComponent(reportSource, 'LongVegetationReport.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {},
      './longVegetationStatisticsPreferences': {}, './longVegetationSwitchPreferences': {},
      './LongVegetationSwitchPreferences.svelte': { default() {} }, './longVegetationThresholdPreferences': {},
      './LongVegetationThresholdPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {},
      './LongVegetationGroupPreferences.svelte': { default() {} }, './longVegetationQualityNullPreferences': {},
      './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationQualityMinimumPreferences': {},
      './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
      './longVegetationNamingPreferences': { createLongVegetationNamingPreferencesRegistry: () => registry },
      './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
      './LongVegetationNamingPreferences.svelte': { default: child() }, './longEnvironmentReport': {}, './longVegetationReport': {},
      './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
        view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
      } },
    });
    for (let mount = 0; mount < 2; mount++) {
      const barriers = [];
      const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
      assert.equal(barriers.at(-1), enabled);
      assert.equal(body.includes('data-long-vegetation-naming'), enabled);
      if (enabled) assert.match(body, /data-long-vegetation-naming-save[^>]*disabled/);
    }
    session.undo();
  }
  const report = source('LongVegetationReport.svelte');
  assert.match(report, /namingView = naming.view\(\); invalidateReports\(\)/);
  assert.match(report, /function invalidateReports\(\) \{\s*preview = null; workbookReview = null;\s*projectWorkspace\?\.invalidate\(\)/);
  assert.match(report, /unsubscribeNaming\?\.\(\)/);
  assert.match(report, /qualityMinimumBarrier \|\| namingBarrier/);
  assert.match(report, /qualityMinimumMutationHeld \|\| namingMutationHeld/);
});
