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
const prefs = loadTypeScript('longVegetationSelectedFieldPreferences.ts', {
  './qualityEditor': quality, './reportTitlePreferences': shared, './googleEarthReview': earth,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationSelectedFieldWire.json'), 'utf8'));
const scope = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.review[key]]));
const review = (mode = 1, field = wire.review.values.selectedField) =>
  ({ ...structuredClone(wire.review), values: { mode, selectedField: field } });
const outcome = (changed = true, errorMessage = '') =>
  ({ contextId: scope.contextId, changed, committed: changed, errorMessage });
const ready = (mode = 1, field = wire.review.values.selectedField) => {
  const session = new prefs.LongVegetationSelectedFieldPreferencesSession(scope);
  session.apply(review(mode, field)); return session;
};
const source = file => readFileSync(path.join(__dirname, file), 'utf8');

test('strict-Go field wire detaches actual client proxies and retains declaredType metadata', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationSelectedFieldPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.selectedField = 'foreign'; reactive.definitions[1].name = 'repaired';
  assert.deepEqual(structuredClone(detached), wire.review);
  const session = ready(); session.editField('Native');
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
  const columns = session.definitions(); columns[1].declaredType = 'REAL';
  assert.equal(session.definitions()[1].declaredType, 'TEXT');
});
test('all source modes retain untouched pairs and literal field Change has nonempty/empty provenance', async () => {
  for (const old of [0, 1, 2, 4]) for (const field of ['', ' historical\0field ', 'Native']) {
    const untouched = ready(old, field);
    await untouched.save(request => {
      assert.deepEqual(request, { expected: { mode: old, selectedField: field }, proposed: { mode: old, selectedField: field } });
      return Promise.resolve(outcome(false));
    });
    for (const raw of ['', 'RedBlueList', 'Wetland_Ind', 'WeedStatus', ' nonmember ', '界'.repeat(300), 'SQL ]; DROP TABLE x;--']) {
      const session = ready(old, field); session.editField(raw);
      const next = raw === '' ? old : 4;
      assert.deepEqual(session.view().draft, { mode: String(next), selectedField: raw });
      assert.equal(session.view().draftError, '');
      await session.save(request => {
        assert.deepEqual(request, { expected: { mode: old, selectedField: field }, proposed: { mode: next, selectedField: raw } });
        return Promise.resolve(outcome(next !== old || raw !== field));
      });
    }
    const changedBack = ready(old, field); changedBack.editField('intermediate'); changedBack.editField('');
    assert.deepEqual(changedBack.view().draft, { mode: '4', selectedField: '' });
    changedBack.undo();
    assert.deepEqual(changedBack.view().draft, { mode: String(old), selectedField: field });
  }
  assert.deepEqual([...prefs.longVegetationSelectedFieldSuggestions], ['RedBlueList', 'Wetland_Ind', 'WeedStatus']);
});
test('malformed owner/value/source/physical wires refuse without inferred values or type repairs', async () => {
  const session = new prefs.LongVegetationSelectedFieldPreferencesSession(scope);
  assert.deepEqual(session.view().draft, { mode: '', selectedField: '' }); assert.equal(session.definitions(), null);
  assert.throws(() => session.editField('Native'), /Load/);
  await assert.rejects(session.save(async () => outcome()), /Load/);
  for (const value of [null, {}, review(3), review('1'), review(1, null), review(1, '\ud800'),
    ...Object.keys(scope).map(key => ({ ...review(), [key]: 'foreign' })),
    { ...review(), source: 'assumed Access schema' }, { ...review(), definitions: null }]) {
    assert.throws(() => prefs.validateLongVegetationSelectedFieldPreferences(value, scope), /no defaults applied/);
  }
  for (const column of [null, {}, { name: '', declaredType: 'TEXT' }, { name: '\ud800', declaredType: 'TEXT' },
    { name: 'x\0', declaredType: 'TEXT' }, { name: 'Native', type: 'TEXT' },
    { name: 'Native', declaredType: null }, { name: 'Native', declaredType: '\ud800' }]) {
    assert.throws(() => prefs.validateLongVegetationSelectedFieldPreferences({ ...review(), definitions: [column] }, scope), /metadata is malformed/);
  }
});
test('invalid drafts survive exact-owner remount, block Save and keep correction and Undo live', async () => {
  const registry = prefs.createLongVegetationSelectedFieldPreferencesRegistry(), session = registry(scope);
  session.apply(review()); let calls = 0;
  for (const raw of ['new\0field', '\ud800', '\udc00']) {
    session.editField(raw); assert.equal(registry(scope), session);
    assert.equal(session.view().draft.selectedField, raw); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }));
    session.editField(' correction '); assert.equal(session.view().draftError, '');
    session.undo(); assert.equal(session.view().draft.selectedField, wire.review.values.selectedField);
  }
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => registry({ ...scope, [key]: 'foreign' }));
  assert.equal(calls, 0);
});
test('field-only clear receipts and unknown mutation authority remain across acknowledgement and remount', async () => {
  const registry = prefs.createLongVegetationSelectedFieldPreferencesRegistry(), session = registry(scope);
  session.apply(review(1, '')); session.editField('Native'); session.editField('');
  let release, cancelled = 0;
  const save = session.save(() => {
    const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise;
  });
  for (const action of [() => session.editField('Other'), () => session.undo(), () => session.beginLoad(), () => session.acknowledge()]) assert.throws(action);
  release(outcome(true, 'committed; do not replay')); await save;
  assert.equal(cancelled, 0); assert.equal(session.view().blocked, true);
  session.acknowledge(); assert.equal(registry(scope), session); assert.equal(session.view().receipts.length, 1);
  session.editField('Native'); await session.save(async () => { throw new Error('wire failed after dispatch'); });
  assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().receipts.length, 2);
  session.acknowledge(); await assert.rejects(session.save(async () => outcome()));
  session.apply(review(4, 'Native'));
  assert.equal(session.view().authorityUnknown, false); assert.equal(session.view().receipts.length, 2);
});
test('actual field handlers cancel stale reads and distinguish operation holds from mutation holds', async () => {
  const session = ready(); let resolveRead, reads = 0, saves = 0, cancellations = 0;
  const context = componentFunctions('LongVegetationSelectedFieldPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    contextId: scope.contextId, preferences: session, view: session.view(), busy: false, operationHeld: false, mutationHeld: false,
    generation: 0, error: '', onReadBusyChange() {},
    reads: { cancelAll() { cancellations++; }, track(value) { return value; } },
    LongVegetationSelectedFieldPreferencesService: {
      GetLongVegetationSelectedFieldPreferences() { reads++; return new Promise(done => { resolveRead = done; }); },
      SaveLongVegetationSelectedFieldPreferences(id, raw) {
        saves++; assert.equal(id, scope.contextId); assert.deepEqual(JSON.parse(raw), wire.request);
        return Promise.resolve(wire.outcome);
      },
    },
  });
  const pending = context.actions.load(); context.actions.cancel(); resolveRead(review(4, 'foreign')); await pending;
  assert.equal(reads, 1); assert.equal(session.view().saved.mode, 1); assert.ok(cancellations);
  context.operationHeld = true; context.actions.edit('Native'); assert.equal(session.view().draft.mode, '4');
  await context.actions.save(); assert.equal(saves, 0);
  context.operationHeld = false; context.mutationHeld = true; context.actions.edit('blocked');
  assert.equal(session.view().draft.selectedField, 'Native');
  context.mutationHeld = false; context.view = session.view(); await context.actions.save(); assert.equal(saves, 1);
  const text = source('LongVegetationSelectedFieldPreferences.svelte');
  assert.match(text, /oninput=\{event => \{ const raw = event\.currentTarget\.value; edit\(raw\); \}\}/);
});
test('actual field panel associates one live source control with its label and literal suggestions/physical types', () => {
  const component = serverComponent(source('LongVegetationSelectedFieldPreferences.svelte'), 'LongVegetationSelectedFieldPreferences.svelte', {
    '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationSelectedFieldPreferences': prefs,
  });
  const html = render(component, { props: { contextId: scope.contextId, preferences: ready(), onReadBusyChange() {} } }).body;
  assert.match(html, /for="long-vegetation-selected-field"/);
  assert.match(html, /Show selected field/);
  assert.equal((html.match(/data-source-control="cmbShowField"/g) ?? []).length, 1);
  for (const name of ['RedBlueList', 'Wetland_Ind', 'WeedStatus', 'Native', 'IsAttribute', 'BOOLEAN', 'INTEGER']) assert.ok(html.includes(name), name);
  assert.match(html, /suggestions are not mandatory membership/);
  assert.match(html, /no trimming, case change or column creation/);
  assert.match(html, /saving preferences does not enable them/);
});
test('actual root remount retains field errors and independently gates every report operation', () => {
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `selected-field-root-${enabled}` };
    const registry = prefs.createLongVegetationSelectedFieldPreferencesRegistry(), session = registry(owner);
    session.apply({ ...review(), ...owner }); session.editField('new\0field');
    const reportSource = source('LongVegetationReport.svelte').replace(/import\.meta\.env\.(\w+) === 'true'/g,
      (_, flag) => String(enabled && flag === 'VITE_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES'));
    const child = serverComponent(source('LongVegetationSelectedFieldPreferences.svelte'), 'LongVegetationSelectedFieldPreferences.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationSelectedFieldPreferences': prefs,
    });
    const component = serverComponent(reportSource, 'LongVegetationReport.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {},
      './longVegetationStatisticsPreferences': {}, './longVegetationSwitchPreferences': {},
      './LongVegetationSwitchPreferences.svelte': { default() {} }, './longVegetationThresholdPreferences': {},
      './LongVegetationThresholdPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {},
      './LongVegetationGroupPreferences.svelte': { default() {} }, './longVegetationQualityNullPreferences': {},
      './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationQualityMinimumPreferences': {},
      './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
      './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
      './longVegetationSelectedFieldPreferences': { createLongVegetationSelectedFieldPreferencesRegistry: () => registry },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
      './LongVegetationSelectedFieldPreferences.svelte': { default: child }, './longEnvironmentReport': {}, './longVegetationReport': {},
      './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
        view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
      } },
    });
    for (let mount = 0; mount < 2; mount++) {
      const barriers = [];
      const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
      assert.equal(barriers.at(-1), enabled);
      assert.equal(body.includes('data-long-vegetation-selected-field'), enabled);
      if (enabled) {
        assert.match(body, /data-long-vegetation-selected-field-save[^>]*disabled/);
        assert.doesNotMatch(body, /data-long-vegetation-selected-field-undo[^>]*disabled/);
      }
    }
    session.undo();
  }
  const report = source('LongVegetationReport.svelte');
  assert.match(report, /selectedFieldView = selectedField.view\(\); invalidateReports\(\)/);
  assert.match(report, /function invalidateReports\(\) \{\s*preview = null; workbookReview = null;\s*projectWorkspace\?\.invalidate\(\)/);
  assert.match(report, /unsubscribeSelectedField\?\.\(\)/);
  assert.match(report, /namingBarrier \|\| selectedFieldBarrier/);
  assert.match(report, /namingMutationHeld \|\| selectedFieldMutationHeld/);
});
