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
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const prefs = loadTypeScript('longVegetationQualityNullPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationQualityNullWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values;
const fields = ['siteIncludeNulls', 'vegIncludeNulls', 'soilIncludeNulls'];
const review = (values = wire.review.values, owner = scope) => ({ ...owner, values });
const outcome = (owner = scope, committed = true, errorMessage = '') =>
  ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
const ready = (owner = scope) => {
  const session = new prefs.LongVegetationQualityNullPreferencesSession(owner);
  session.apply(review(undefined, owner)); return session;
};
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const child = () => serverComponent(source('LongVegetationQualityNullPreferences.svelte'), 'LongVegetationQualityNullPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationQualityNullPreferences': prefs,
});

test('strict-Go quality wire transports three booleans detached from actual Svelte proxies', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const validated = prefs.validateLongVegetationQualityNullPreferences(reactive, scope);
  assert.deepEqual(structuredClone(validated), wire.review);
  reactive.values.siteIncludeNulls = false; assert.equal(validated.values.siteIncludeNulls, true);
  const session = ready(); for (const field of fields) session.editNullFlag(field, wire.request.proposed[field]);
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all eight combinations preserve independent boolean values without coercion', async () => {
  for (let mask = 0; mask < 8; mask++) {
    const values = { siteIncludeNulls: !!(mask & 1), vegIncludeNulls: !!(mask & 2), soilIncludeNulls: !!(mask & 4) };
    const session = ready(); for (const field of fields) session.editNullFlag(field, values[field]);
    let calls = 0; const changed = mask !== 7;
    await session.save(request => {
      calls++; assert.deepEqual(request, { expected: wire.review.values, proposed: values });
      return Promise.resolve(outcome(scope, changed));
    });
    assert.equal(calls, 1); session.acknowledge(); assert.deepEqual(session.view().saved, values);
  }
});
test('unloaded missing malformed and foreign reviews never install source defaults', async () => {
  const session = new prefs.LongVegetationQualityNullPreferencesSession(scope);
  assert.equal(session.view().draft, null); assert.equal(session.view().saved, null);
  assert.throws(() => session.editNullFlag('siteIncludeNulls', true), /Load saved/);
  await assert.rejects(session.save(async () => outcome()), /Load saved/);
  for (const value of [null, {}, review(null),
    ...fields.flatMap(field => [null, '', 'True', -1, 0, undefined].map(value => review({ ...wire.review.values, [field]: value }))),
    ...fields.map(field => { const values = { ...wire.review.values }; delete values[field]; return review(values); }),
    ...Object.keys(scope).map(key => review(undefined, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationQualityNullPreferences(value, scope), /no defaults applied/);
  }
});
test('raw runtime errors survive retention and prevent Save while correction and Undo remain live', async () => {
  const owner = { ...scope, contextId: 'retained-quality-nulls' };
  const session = prefs.longVegetationQualityNullPreferencesSession(owner); session.apply(review(undefined, owner));
  let calls = 0;
  for (const field of fields) for (const value of [null, '', 'False', 0, -1, undefined]) {
    session.editNullFlag(field, value);
    assert.ok(prefs.longVegetationQualityNullPreferencesSession(owner).view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(owner); }), /three boolean/);
    session.editNullFlag(field, false); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, wire.review.values);
  }
  assert.equal(calls, 0); assert.throws(() => session.editNullFlag('foreign', true), /Unsupported/);
  for (const key of ['project', 'projectPath', 'su', 'suPath']) {
    assert.throws(() => prefs.longVegetationQualityNullPreferencesSession({ ...owner, [key]: 'foreign' }));
  }
  assert.equal(prefs.longVegetationQualityNullPreferencesSession({ ...scope, contextId: 'another-context' }).view().draft, null);
});
test('held Save is noncancellable and committed-warning receipts persist after acknowledgement', async () => {
  const session = ready(); session.editNullFlag('siteIncludeNulls', false);
  let release, cancellations = 0;
  const pending = session.save(() => {
    const value = new Promise(done => { release = done; }); value.cancel = () => { cancellations++; }; return value;
  });
  assert.throws(() => session.editNullFlag('vegIncludeNulls', false)); assert.throws(() => session.beginLoad());
  assert.throws(() => session.undo()); assert.throws(() => session.acknowledge());
  release(outcome(scope, true, 'committed; do not replay')); await pending; session.acknowledge();
  assert.equal(cancellations, 0); assert.equal(session.view().saved.siteIncludeNulls, false);
  assert.equal(session.view().receipts[0].outcome.committed, true);
});
test('unknown outcomes lose authority and require observation without receipt erasure or replay', async () => {
  const session = ready(); session.editNullFlag('soilIncludeNulls', false);
  await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
  await assert.rejects(session.save(async () => outcome()), /Load saved/);
  const receipt = session.view().receipts[0];
  session.applyLoaded(review(), session.beginLoad());
  assert.deepEqual(session.view().receipts[0], receipt); assert.equal(session.view().authorityUnknown, false);
});
test('actual panel handlers cancel stale reads and enforce outside holds before dispatch', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} } });
  let release, cancelled = 0, saves = 0; const held = [];
  const context = componentFunctions('LongVegetationQualityNullPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    contextId: scope.contextId, preferences: session, view: session.view(), busy: false, error: '',
    operationHeld: false, mutationHeld: false, generation: 0, reads: new ReadRequests(),
    onReadBusyChange(value) { held.push(value); },
    LongVegetationQualityNullPreferencesService: {
      GetLongVegetationQualityNullPreferences() {
        const value = new Promise(done => { release = done; }); value.cancel = () => { cancelled++; }; return value;
      },
      SaveLongVegetationQualityNullPreferences(_id, raw) {
        saves++; assert.equal(JSON.parse(raw).proposed.siteIncludeNulls, false); return Promise.resolve(outcome());
      },
    },
  });
  const pending = context.actions.load(); assert.equal(held.at(-1), true);
  context.actions.cancel(); context.actions.edit('siteIncludeNulls', false);
  release(review()); await pending; assert.equal(cancelled, 1); assert.equal(held.at(-1), false);
  assert.equal(session.view().draft.siteIncludeNulls, false);
  context.operationHeld = true; await context.actions.save(); assert.equal(saves, 0);
  context.mutationHeld = true; context.actions.edit('vegIncludeNulls', false); assert.equal(session.view().draft.vegIncludeNulls, true);
  context.operationHeld = false; await context.actions.save(); assert.equal(saves, 1);
});
test('source group labels leaf controls and unloaded distinction render once with correction and Undo', () => {
  const controls = ['optSiteQualityNull', 'optVegQualityNull', 'optSoilQualityNull'];
  for (const initialized of [false, true]) {
    const session = initialized ? ready() : new prefs.LongVegetationQualityNullPreferencesSession(scope);
    if (initialized) session.editNullFlag('siteIncludeNulls', null);
    const { body } = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } });
    assert.match(body, /Data Quality - Include Nulls/);
    for (let index = 0; index < 3; index++) {
      assert.equal((body.match(new RegExp(`data-source-control="${controls[index]}"`, 'g')) ?? []).length, 1);
      assert.ok(body.includes(`for="long-vegetation-quality-null-${fields[index]}"`));
    }
    assert.match(body, /data-long-vegetation-quality-null-save[^>]*disabled/);
    assert.match(body, /does not enable it or change minimum criteria/);
    if (initialized) assert.doesNotMatch(body, /data-long-vegetation-quality-null-undo[^>]*disabled/);
    else assert.match(body, /unloaded disabled checkboxes do not represent saved values/);
  }
});
test('actual report retains errors across remount and propagates independent default-off barriers', () => {
  const reportSource = source('LongVegetationReport.svelte');
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `quality-null-composition-${enabled}` };
    const session = prefs.longVegetationQualityNullPreferencesSession(owner); session.apply(review(undefined, owner));
    session.editNullFlag('vegIncludeNulls', null);
    const component = serverComponent(reportSource.replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const qualityNullsEnabled = false;', `const qualityNullsEnabled = ${enabled};`), 'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {},
        './longVegetationStatisticsPreferences': {}, './longVegetationSwitchPreferences': {},
        './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
        './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './longVegetationQualityNullPreferences': prefs, './LongVegetationQualityNullPreferences.svelte': { default: child() },
        './LongVegetationSwitchPreferences.svelte': { default() {} }, './longEnvironmentReport': {}, './longVegetationReport': {},
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
          view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
        } },
      });
    for (let mount = 0; mount < 2; mount++) {
      const barriers = []; const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
      assert.equal(barriers.at(-1), enabled);
      if (enabled) assert.match(body, /data-long-vegetation-quality-nulls/);
      else assert.doesNotMatch(body, /data-long-vegetation-quality-nulls/);
    }
  }
  assert.match(reportSource, /qualityNullView = qualityNulls.view\(\); preview = null; workbookReview = null/);
  assert.match(reportSource, /groupBarrier \|\| qualityNullBarrier/);
  assert.match(reportSource, /unsubscribeQualityNulls\?\.\(\)/);
  assert.match(source('LongVegetationQualityNullPreferences.svelte'), /const checked = event.currentTarget.checked; edit\(field.key, checked\)/);
  assert.doesNotMatch(source('LongVegetationQualityNullPreferences.svelte'), /reads\.track\(LongVegetationQualityNullPreferencesService\.Save/);
});
