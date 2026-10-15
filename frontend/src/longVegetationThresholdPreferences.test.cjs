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
const prefs = loadTypeScript('longVegetationThresholdPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const switches = loadTypeScript('longVegetationSwitchPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationThresholdWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values;
const fields = ['presenceGreaterThan', 'meanCoverGreaterThan'];
const review = (values = wire.review.values, owner = scope) => ({ ...owner, values });
const outcome = (owner = scope, committed = true, errorMessage = '') => ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
const ready = (owner = scope) => { const session = new prefs.LongVegetationThresholdPreferencesSession(owner); session.apply(review(undefined, owner)); return session; };
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const child = () => serverComponent(source('LongVegetationThresholdPreferences.svelte'), 'LongVegetationThresholdPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationThresholdPreferences': prefs,
});

test('strict-Go threshold wire detaches actual Svelte proxies before transport', async () => {
  const reactive = proxy(structuredClone(wire.review)); assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationThresholdPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.presenceGreaterThan = 101; assert.equal(detached.values.presenceGreaterThan, 0);
  const session = ready(); for (const field of fields) session.editThreshold(field, String(wire.request.proposed[field]));
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('raw finite DOUBLE grammar retains signed and unbounded values without trimming or underflow repair', () => {
  for (const [raw, expected] of [['-1', -1], ['101.5', 101.5], ['.5', .5], ['+1.', 1], ['1e2', 100],
    ['5e-324', Number.MIN_VALUE], ['-5e-324', -Number.MIN_VALUE], ['1.7976931348623157e308', Number.MAX_VALUE],
    ['-0', -0], ['0e999', 0], ['0.0e-999', 0]]) {
    assert.ok(Object.is(prefs.longVegetationThresholdValue(raw, 'threshold'), expected));
  }
  for (const raw of ['', ' ', ' 1', '1 ', '1,5', '0x10', 'NaN', 'Infinity', '1e400', '1e-400', '-1e-400',
    '1.2.3', '1_000', '\ud800', null, 0, true]) {
    assert.throws(() => prefs.longVegetationThresholdValue(raw, 'threshold'), /finite|DOUBLE/);
  }
});
test('unloaded or malformed saved thresholds are unavailable, not zero defaults', async () => {
  const session = new prefs.LongVegetationThresholdPreferencesSession(scope);
  assert.deepEqual(session.view().draft, { presenceGreaterThan: '', meanCoverGreaterThan: '' });
  assert.throws(() => session.editThreshold('presenceGreaterThan', '0'), /Load saved/);
  await assert.rejects(session.save(async () => outcome()), /Load/);
  for (const value of [null, {}, review(null), ...fields.flatMap(field =>
    [null, undefined, '0', false, NaN, Infinity].map(value => review({ ...wire.review.values, [field]: value }))),
    ...Object.keys(scope).map(key => review(undefined, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationThresholdPreferences(value, scope), /no defaults applied/);
  }
});
test('invalid raw drafts persist without Save dispatch, with live correction and Undo', async () => {
  const session = ready(); let calls = 0;
  for (const field of fields) for (const raw of ['', '1,5', '1e-400', ' 1', '\ud800']) {
    session.editThreshold(field, raw); assert.equal(session.view().draft[field], raw); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }));
    session.editThreshold(field, '101'); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, { presenceGreaterThan: '0', meanCoverGreaterThan: '0' });
  }
  assert.equal(calls, 0); assert.throws(() => session.editThreshold('foreign', '1'), /Unsupported/);
});
test('threshold registry retains exact-owner validation independently from switch drafts', () => {
  const owner = { ...scope, contextId: 'threshold-registry' };
  const session = prefs.longVegetationThresholdPreferencesSession(owner); session.apply(review(undefined, owner));
  session.editThreshold('meanCoverGreaterThan', '');
  assert.ok(prefs.longVegetationThresholdPreferencesSession(owner).view().draftError);
  assert.equal(switches.longVegetationSwitchPreferencesSession(owner).view().draftError, '');
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => prefs.longVegetationThresholdPreferencesSession({ ...owner, [key]: 'foreign' }));
});
test('held Save is noncancellable and known/unknown receipts retain authority rules', async () => {
  const session = ready(); session.editThreshold('presenceGreaterThan', '-1');
  let release, cancellations = 0;
  const pending = session.save(() => { const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancellations++; }; return promise; });
  assert.throws(() => session.editThreshold('presenceGreaterThan', '2')); assert.throws(() => session.undo());
  assert.throws(() => session.beginLoad()); assert.throws(() => session.acknowledge());
  release(outcome(scope, true, 'committed; do not replay')); await pending; session.acknowledge();
  assert.equal(cancellations, 0); assert.equal(session.view().saved.presenceGreaterThan, -1);
  session.editThreshold('meanCoverGreaterThan', '1.5'); await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
  await assert.rejects(session.save(async () => outcome()), /Load/);
  session.applyLoaded(review(), session.beginLoad()); assert.equal(session.view().receipts.length, 2);
  assert.equal(session.view().authorityUnknown, false);
});
test('actual threshold handlers cancel stale reads and respect external holds before dispatch', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} } });
  let release, cancelled = 0, saves = 0, reads = 0;
  const context = componentFunctions('LongVegetationThresholdPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    preferences: session, view: session.view(), busy: false, error: '', generation: 0, operationHeld: false, mutationHeld: false,
    contextId: scope.contextId, reads: new ReadRequests(), onReadBusyChange() {},
    LongVegetationThresholdPreferencesService: {
      GetLongVegetationThresholdPreferences() { reads++; const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise; },
      SaveLongVegetationThresholdPreferences(_id, raw) { saves++; assert.equal(JSON.parse(raw).proposed.presenceGreaterThan, -1); return Promise.resolve(outcome()); },
    },
  });
  const pending = context.actions.load(); context.actions.cancel(); context.actions.edit('presenceGreaterThan', '-1');
  release(review()); await pending; assert.equal(cancelled, 1); assert.equal(session.view().draft.presenceGreaterThan, '-1');
  context.operationHeld = true; await context.actions.load(); await context.actions.save(); assert.equal(reads, 1); assert.equal(saves, 0);
  context.operationHeld = false; await context.actions.save(); assert.equal(saves, 1);
});
test('source labels and percent suffixes preserve one live field with retained safety above guidance', () => {
  const session = ready(); session.editThreshold('meanCoverGreaterThan', '');
  const { body } = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } });
  for (const [field, control, caption] of [
    ['presenceGreaterThan', 'rptLvShowSppPres', 'Show species with presence > than'],
    ['meanCoverGreaterThan', 'rptLvShowSppMC', 'Show species with mean cover > than'],
  ]) {
    assert.equal((body.match(new RegExp(`id="long-vegetation-threshold-${field}"`, 'g')) || []).length, 1);
    assert.match(body, new RegExp(`data-source-control="${control}"`)); assert.ok(body.includes(caption));
  }
  assert.match(body, /data-long-vegetation-threshold-save[^>]*disabled/);
  assert.doesNotMatch(body, /data-long-vegetation-threshold-undo[^>]*disabled/);
  assert.ok(body.indexOf('role="alert"') < body.indexOf('Load saved thresholds before editing'));
  assert.match(source('LongVegetationThresholdPreferences.svelte'), /const raw = event.currentTarget.value; edit\(field.key, raw\)/);
  assert.doesNotMatch(source('LongVegetationThresholdPreferences.svelte'), /reads\.track\(LongVegetationThresholdPreferencesService\.Save/);
});
test('actual parent/child composition keeps threshold gate independent and propagates retained barriers', () => {
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `threshold-composition-${enabled}` };
    const session = prefs.longVegetationThresholdPreferencesSession(owner); session.apply(review(undefined, owner));
    session.editThreshold('presenceGreaterThan', '');
    const component = serverComponent(source('LongVegetationReport.svelte')
      .replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const thresholdsEnabled = false;', `const thresholdsEnabled = ${enabled};`), 'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {}, './longVegetationStatisticsPreferences': {},
        './longVegetationSwitchPreferences': {}, './LongVegetationSwitchPreferences.svelte': { default() {} },
        './longVegetationThresholdPreferences': prefs, './LongVegetationThresholdPreferences.svelte': { default: child() },
        './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
        './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
        './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
        './longEnvironmentReport': {}, './longVegetationReport': {},
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class { view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; } } },
      });
    const barriers = []; const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
    assert.equal(barriers.at(-1), enabled);
    if (enabled) assert.match(body, /data-long-vegetation-thresholds/); else assert.doesNotMatch(body, /data-long-vegetation-thresholds/);
  }
});
