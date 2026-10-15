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
const prefs = loadTypeScript('longVegetationEnforcementPreferences.ts', {
  './qualityEditor': quality, './reportTitlePreferences': shared, './googleEarthReview': earth,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationEnforcementWire.json'), 'utf8'));
const scope = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.review[key]]));
const review = (enforce = false, unitGroups = 2) => ({ ...structuredClone(wire.review), values: { enforce, unitGroups } });
const outcome = (changed = true, errorMessage = '') => ({ ...structuredClone(wire.outcome), changed, committed: changed, errorMessage });
const source = file => readFileSync(path.join(__dirname, file), 'utf8');
const ready = (enforce = false, unitGroups = 2) => {
  const session = new prefs.LongVegetationEnforcementPreferencesSession(scope);
  session.apply(review(enforce, unitGroups)); return session;
};
const panel = () => serverComponent(source('LongVegetationEnforcementPreferences.svelte'), 'LongVegetationEnforcementPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {},
});

test('strict production-Go enforcement wire detaches actual client proxy pairs and printers', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const session = new prefs.LongVegetationEnforcementPreferencesSession(scope);
  session.apply(reactive); reactive.values.unitGroups = 3; reactive.printers.names[0] = 'changed';
  assert.deepEqual(session.view().saved, wire.review.values);
  assert.deepEqual(session.printerView().value, wire.review.printers);
  session.editEnforce(true);
  let sent; await session.saveWithPrinters(request => { sent = request; return Promise.resolve(proxy(structuredClone(wire.outcome))); });
  assert.deepEqual(sent, wire.request);
  assert.deepEqual(session.view().outcome, { contextId: scope.contextId, changed: true, committed: true, errorMessage: '' });
  assert.deepEqual(session.printerView().value, wire.outcome.printers);
  const detached = session.printerView(); detached.value.names.push('inferred');
  assert.equal(session.printerView().value.names.length, 0);
});
test('source enforcement pairs retain untouched group1/2/3 and changed-away-back reset provenance', async () => {
  for (const enforce of [false, true]) for (const unitGroups of [1, 2, 3]) {
    const untouched = ready(enforce, unitGroups);
    await untouched.saveWithPrinters(request => {
      assert.deepEqual(request, { expected: { enforce, unitGroups }, proposed: { enforce, unitGroups } });
      return Promise.resolve(outcome(false));
    });
    const session = ready(enforce, unitGroups);
    session.editEnforce(!enforce); session.editEnforce(enforce);
    assert.deepEqual(session.view().draft, { enforce, unitGroups: 1 });
    await session.saveWithPrinters(request => {
      assert.deepEqual(request, { expected: { enforce, unitGroups }, proposed: { enforce, unitGroups: 1 } });
      return Promise.resolve(outcome(unitGroups !== 1));
    });
    session.acknowledge(); session.undo();
    assert.deepEqual(session.view().draft, { enforce, unitGroups: 1 });
    const undone = ready(enforce, unitGroups); undone.editEnforce(!enforce); undone.undo();
    assert.deepEqual(undone.view().draft, { enforce, unitGroups });
  }
});
test('incomplete owner/pair/printer wires refuse without inferred host defaults or repaired names', () => {
  for (const invalid of [null, {}, review(null), review(-1), review(false, 0), review(false, 4), review(false, '1'),
    ...Object.keys(scope).map(key => ({ ...review(), [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationEnforcementPreferences(invalid, scope), /no defaults applied/);
  }
  for (const printers of [null, {}, { ...wire.review.printers, source: 'assumed atomic' },
    { ...wire.review.printers, names: null }, { ...wire.review.printers, names: [''] },
    { ...wire.review.printers, names: ['\ud800'] }, { ...wire.review.printers, names: ['x\0'] },
    { ...wire.review.printers, default: '' }, { ...wire.review.printers, default: undefined }]) {
    assert.throws(() => prefs.validateLongVegetationEnforcementPreferences({ ...review(), printers }, scope), /no printer names or default were inferred/);
  }
  const session = new prefs.LongVegetationEnforcementPreferencesSession(scope);
  assert.deepEqual(session.view().draft, { enforce: null, unitGroups: null });
  assert.equal(session.printerView().value, null); assert.throws(() => session.editEnforce(true), /Load/);
});
test('retained invalid enforcement drafts block Save but allow correction/Undo without remount reset', async () => {
  const registry = prefs.createLongVegetationEnforcementPreferencesRegistry(), session = registry(scope);
  session.apply(review());
  for (const bad of [null, 'true', -1]) {
    session.editEnforce(bad); assert.equal(registry(scope), session);
    assert.ok(session.view().draftError);
    await assert.rejects(session.saveWithPrinters(async () => outcome()));
    session.editEnforce(true); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, { enforce: false, unitGroups: 2 });
  }
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => registry({ ...scope, [key]: 'foreign' }));
});
test('printer failures do not erase committed YAML receipts, acknowledge or retry authority', async () => {
  for (const printers of [null, { ...wire.outcome.printers, names: null }]) {
    const session = ready(); session.editEnforce(true);
    await session.saveWithPrinters(async () => ({ ...outcome(true, 'committed; refresh failed; do not replay'), printers }));
    assert.equal(session.view().outcome.committed, true); assert.equal(session.view().authorityUnknown, false);
    assert.equal(session.printerView().value, null); assert.match(session.printerView().error, /do not replay/);
    session.acknowledge(); assert.equal(session.view().receipts.length, 1);
    session.apply(review(true, 1)); assert.equal(session.printerView().error, '');
    await session.saveWithPrinters(async () => outcome(false));
    assert.equal(session.view().receipts.length, 2); assert.equal(session.view().outcome.committed, false);
  }
  const session = ready(); session.editEnforce(true);
  await session.saveWithPrinters(async () => { throw new Error('lost response'); });
  assert.equal(session.view().authorityUnknown, true);
  assert.equal(session.printerView().value, null);
  assert.match(session.printerView().error, /not been confirmed/);
  session.acknowledge(); await assert.rejects(session.saveWithPrinters(async () => outcome()));
  session.apply(review(true, 1)); assert.equal(session.view().receipts.length, 1);
});
test('held enforcement Save is noncancellable and prevents reads/edits/Undo/acknowledgement', async () => {
  const session = ready(); session.editEnforce(true);
  let release, cancelled = 0;
  const save = session.saveWithPrinters(() => {
    const promise = new Promise(done => { release = done; }); promise.cancel = () => { cancelled++; }; return promise;
  });
  for (const action of [() => session.editEnforce(false), () => session.undo(), () => session.beginLoad(), () => session.acknowledge()]) assert.throws(action);
  release(outcome()); await save;
  assert.equal(cancelled, 0); assert.equal(session.view().blocked, true);
});
test('actual enforcement handlers drop cancelled reads and capture DOM Boolean with correction live', async () => {
  const session = ready();
  let resolveRead, cancelled = 0;
  const request = new Promise(done => { resolveRead = done; });
  const values = { preferences: session, view: session.view(), busy: false, error: '', generation: 0,
    operationHeld: false, mutationHeld: false, contextId: scope.contextId, onReadBusyChange() {},
    reads: { track: value => value, cancelAll: () => { cancelled++; } },
    LongVegetationEnforcementPreferencesService: { GetLongVegetationEnforcementPreferences: () => request } };
  const ctx = componentFunctions('LongVegetationEnforcementPreferences.svelte', ['cancel', 'load', 'edit'], values);
  const load = ctx.actions.load(); ctx.actions.cancel(); resolveRead(review(true, 3)); await load;
  assert.deepEqual(session.view().saved, { enforce: false, unitGroups: 2 });
  ctx.mutationHeld = true; ctx.actions.edit(true); assert.equal(session.view().draft.enforce, false);
  ctx.mutationHeld = false; ctx.operationHeld = true; ctx.actions.edit(true);
  assert.equal(session.view().draft.enforce, true); assert.ok(cancelled > 0);
  assert.match(source('LongVegetationEnforcementPreferences.svelte'), /const raw = event.currentTarget.checked; edit\(raw\)/);
});
test('actual enforcement panel preserves source label and read-only host observations', () => {
  const html = render(panel(), { props: { contextId: scope.contextId, preferences: ready(), onReadBusyChange() {} } }).body;
  assert.match(html, /for="long-vegetation-enforcement"/); assert.match(html, /Enforce rules/);
  assert.equal((html.match(/data-source-control="optEnforceQC"/g) ?? []).length, 1);
  for (const expected of ['literal printer', 'Other printer', 'Selected fields (report unavailable)',
    'No printer selection, default change or printing', 'including a toggle away and back']) assert.ok(html.includes(expected), expected);
  assert.match(html, /data-long-vegetation-enforcement-save/);
});
test('actual report retains enforcement draft barriers across remounts behind its independent flag', () => {
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `enforcement-root-${enabled}` };
    const registry = prefs.createLongVegetationEnforcementPreferencesRegistry(), session = registry(owner);
    session.apply({ ...review(), ...owner }); session.editEnforce(null);
    const reportSource = source('LongVegetationReport.svelte').replace(/import\.meta\.env\.(\w+) === 'true'/g,
      (_, flag) => String(enabled && flag === 'VITE_LONG_VEGETATION_ENFORCEMENT_PREFERENCES'));
    const dependencies = {
      '../bindings/github.com/boostao/vpro-wails': {}, './longEnvironmentReport': {}, './longVegetationReport': {},
      './longVegetationEnforcementPreferences': { createLongVegetationEnforcementPreferencesRegistry: () => registry },
      './LongVegetationEnforcementPreferences.svelte': { default: panel() },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
      './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
        view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
      } },
    };
    for (const name of ['Preferences', 'StatisticsPreferences', 'SwitchPreferences', 'ThresholdPreferences', 'GroupPreferences',
      'QualityNullPreferences', 'QualityMinimumPreferences', 'NamingPreferences', 'SelectedFieldPreferences']) {
      dependencies[`./longVegetation${name}`] = {};
      dependencies[`./LongVegetation${name}.svelte`] = { default() {} };
    }
    const report = serverComponent(reportSource, 'LongVegetationReport.svelte', dependencies);
    for (let mount = 0; mount < 2; mount++) {
      const holds = [];
      const html = render(report, { props: { ...owner, onBusyChange: busy => holds.push(busy) } }).body;
      assert.equal(holds.at(-1), enabled);
      assert.equal(html.includes('data-long-vegetation-enforcement'), enabled);
      if (enabled) {
        assert.match(html, /data-long-vegetation-enforcement-save[^>]*disabled/);
        assert.doesNotMatch(html, /data-long-vegetation-enforcement-undo[^>]*disabled/);
      }
    }
    session.undo();
  }
  const report = source('LongVegetationReport.svelte');
  assert.match(report, /enforcementView = enforcement.view\(\); invalidateReports\(\)/);
  assert.match(report, /function invalidateReports\(\) \{\s*preview = null; workbookReview = null;\s*projectWorkspace\?\.invalidate\(\)/);
  assert.match(report, /unsubscribeEnforcement\?\.\(\)/);
  assert.match(report, /selectedFieldBarrier \|\| enforcementBarrier/);
  assert.match(report, /selectedFieldMutationHeld \|\| enforcementMutationHeld/);
});
