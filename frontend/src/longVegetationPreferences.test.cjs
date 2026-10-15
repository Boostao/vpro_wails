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
const prefs = loadTypeScript('longVegetationPreferences.ts', { './reportTitlePreferences': shared });
const environment = loadTypeScript('longEnvironmentPreferences.ts', { './reportTitlePreferences': shared });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationPreferenceWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values;
const review = (title, owner = scope) => ({ ...owner, values: { title } });
const outcome = (owner = scope, committed = true, errorMessage = '') =>
  ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
const ready = (owner = scope) => {
  const session = new prefs.LongVegetationPreferencesSession(owner);
  session.apply(review(wire.request.expected.title, owner)); return session;
};

test('strict-Go wire has the exact title request/review/outcome shape and no inferred values', async () => {
  const session = ready();
  assert.deepEqual(JSON.parse(JSON.stringify(prefs.validateLongVegetationPreferences(wire.review, scope))), wire.review);
  session.edit(wire.request.proposed.title);
  let sent;
  await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request);
  assert.deepEqual(session.view().outcome, wire.outcome);
});
test('actual Svelte client-runtime proxies detach through validated scalars before any clone', () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationPreferences(reactive, scope);
  assert.doesNotThrow(() => structuredClone(detached));
  assert.equal(detached.values.title, wire.review.values.title);
  reactive.values.title = 'changed after validation';
  assert.equal(detached.values.title, wire.review.values.title);
});
test('literal historical/empty/long titles and exact paths stay distinct from unavailable reviews', () => {
  for (const title of ['', 'old\0title', ' Literal\r\n🌲 ', 'x'.repeat(1000)]) {
    assert.equal(prefs.validateLongVegetationPreferences(review(title), scope).values.title, title);
  }
  for (const value of [null, {}, review(null), review(42), review('\ud800'),
    ...Object.keys(scope).map(key => review('title', { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationPreferences(value, scope), /Saved Long Vegetation/);
  }
});
test('equivalent title behavior uses independent Environment and Vegetation registries', () => {
  const owner = { ...scope, contextId: 'separate-title-owners' };
  const vegetation = prefs.longVegetationPreferencesSession(owner);
  const other = environment.longEnvironmentPreferencesSession(owner);
  vegetation.apply(review('vegetation', owner)); other.apply(review('environment', owner));
  vegetation.edit('invalid\0draft');
  assert.equal(other.view().draft.title, 'environment');
  assert.equal(other.view().draftError, '');
  assert.equal(prefs.longVegetationPreferencesSession(owner).view().draftError, vegetation.view().draftError);
  assert.throws(() => prefs.longVegetationPreferencesSession({ ...owner, projectPath: 'foreign' }));
});
test('retained title validation blocks Save without losing correction, Undo or historical no-op', async () => {
  const session = ready(); let calls = 0;
  for (const title of ['new\0title', '\ud800']) {
    session.edit(title);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }), /complete Unicode/);
    session.edit(''); assert.equal(session.view().draftError, '');
  }
  assert.equal(calls, 0);
  session.undo(); assert.equal(session.view().draft.title, wire.request.expected.title);
  session.apply(review('old\0title')); session.edit('old\0title');
  await session.save(async () => outcome(scope, false)); session.acknowledge();
  assert.equal(session.view().saved.title, 'old\0title');
});
test('held Save cannot cancel, edit or reload; committed-error receipt survives acknowledgement/remount', async () => {
  const owner = { ...scope, contextId: 'held-title' };
  const session = prefs.longVegetationPreferencesSession(owner);
  session.apply(review('old', owner)); session.edit('new');
  let release, cancellations = 0, calls = 0;
  const pending = session.save(() => {
    calls++;
    const request = new Promise(done => { release = done; });
    request.cancel = () => { cancellations++; }; return request;
  });
  assert.equal(session.view().busy, true);
  assert.throws(() => session.edit('replacement'));
  assert.throws(() => session.undo()); assert.throws(() => session.beginLoad());
  assert.throws(() => session.acknowledge());
  release(outcome(owner, true, 'committed; do not replay')); await pending;
  assert.equal(calls, 1); assert.equal(cancellations, 0);
  session.acknowledge();
  const retained = prefs.longVegetationPreferencesSession(owner).view();
  assert.equal(retained.saved.title, 'new');
  assert.equal(retained.receipts.length, 1); assert.equal(retained.receipts[0].outcome.committed, true);
});
test('unknown response requires explicit owned observation and never erases irreversible receipt evidence', async () => {
  const session = ready(); session.edit('attempted');
  await session.save(async () => { throw new Error('lost response'); });
  session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
  await assert.rejects(session.save(async () => outcome()), /Load saved title/);
  const receipt = session.view().receipts[0];
  session.applyLoaded(review('observed'), session.beginLoad());
  assert.equal(session.view().authorityUnknown, false);
  assert.deepEqual(session.view().receipts[0], receipt);
});
test('actual title handlers cancel reads and refuse stale publication without cancelling mutations', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {
    Call: {}, CancelledRejectionError: class extends Error {},
  } });
  let release, cancellations = 0, saves = 0;
  const context = componentFunctions('LongVegetationReport.svelte',
    ['cancel', 'loadSavedTitle', 'saveTitle', 'editTitle'], {
      preferences: session, preferenceView: session.view(), busy: false, operationHeld: false, projectWorkspace: null,
      workbookBarrier: false, preferenceBarrier: false, statistics: null, statisticsBarrier: false, switchBarrier: false, thresholdBarrier: false, groupBarrier: false, qualityNullBarrier: false, qualityMinimumBarrier: false, namingBarrier: false, selectedFieldBarrier: false, enforcementBarrier: false, unitBarrier: false, unitMutationHeld: false, generation: 0, error: '',
      reads: new ReadRequests(), contextId: scope.contextId, reportBusy() {},
      LongVegetationPreferencesService: {
        GetLongVegetationPreferences() {
          const read = new Promise(done => { release = done; }); read.cancel = () => { cancellations++; };
          return read;
        },
        SaveLongVegetationPreferences(_id, raw) {
          saves++; assert.deepEqual(JSON.parse(raw), { expected: { title: wire.request.expected.title }, proposed: { title: 'newer' } });
          return Promise.resolve(outcome());
        },
      },
    });
  const pending = context.actions.loadSavedTitle();
  context.actions.editTitle('newer');
  release(review('obsolete')); await pending;
  assert.equal(cancellations, 1); assert.equal(session.view().draft.title, 'newer');
  await context.actions.saveTitle(); assert.equal(saves, 1);
});
test('enabled title presentation has one visible labelled control and retained safety; default has no new editor', () => {
  const source = readFileSync(path.join(__dirname, 'LongVegetationReport.svelte'), 'utf8');
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `render-title-${enabled}` };
    const session = prefs.longVegetationPreferencesSession(owner);
    session.apply(review('First\r\nSecond', owner)); session.edit('invalid\0draft');
    const component = serverComponent(source.replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const preferencesEnabled = false;', `const preferencesEnabled = ${enabled};`),
      'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {},
        './longVegetationPreferences': prefs,
        './longVegetationStatisticsPreferences': {},
        './longVegetationSwitchPreferences': {},
        './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
        './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
        './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
        './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
        './LongVegetationSwitchPreferences.svelte': { default() {} },
        './longEnvironmentReport': {},
        './longVegetationReport': {},
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
          view() { return { busy: false, blocked: false }; }
          subscribe() { return () => {}; }
        } },
      });
    const barriers = [];
    const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
    if (enabled) {
      assert.equal((body.match(/id="long-vegetation-title"/g) ?? []).length, 1);
      assert.match(body, /for="long-vegetation-title">Report Title/);
      assert.match(body, /aria-invalid="true"/);
      assert.match(body, /correct it or Undo/); assert.equal(barriers.at(-1), true);
    } else {
      assert.doesNotMatch(body, /id="long-vegetation-title"/); assert.equal(barriers.at(-1), false);
    }
  }
  assert.match(source, /const value = event.currentTarget.value; editTitle\(value\)/);
  assert.doesNotMatch(source, /reads\.track\(LongVegetationPreferencesService\.Save/);
  assert.match(source, /workbookBarrier \|\| preferenceBarrier \|\| !workbookReview/);
});
