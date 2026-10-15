const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { render } = require('svelte/server');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const source = file => readFileSync(path.join(__dirname, file), 'utf8');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const transport = loadTypeScript('siviParentTransport.ts', {
  './projectMetadataRestore': cells, './siviParentEditor': {}, './siviParentSession': {},
});
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const prefs = loadTypeScript('longVegetationQualityMinimumPreferences.ts', {
  './qualityEditor': quality, './siviParentTransport': transport, './projectMetadataRestore': cells,
  './reportTitlePreferences': shared, './googleEarthReview': earth,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationQualityMinimumWire.json'), 'utf8'));
const scope = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.review[key]]));
const fields = ['siteMinimum', 'vegMinimum', 'soilMinimum'];
const review = (values = wire.review.values, owner = scope) => ({ ...wire.review, ...owner, values });
const outcome = (owner = scope, changed = true, errorMessage = '') =>
  ({ contextId: owner.contextId, changed, committed: changed, errorMessage });
const ready = (owner = scope) => {
  const session = new prefs.LongVegetationQualityMinimumPreferencesSession(owner);
  session.apply(review(undefined, owner)); return session;
};
const cell = (storage, value = null) => ({
  storage, text: storage === 'text' ? value : null, integer: storage === 'integer' ? value : null,
  real: storage === 'real' ? value : null, blobHex: storage === 'blob' ? value : null,
});
const row = (rowId, item, description = null) => ({ rowId, cells: [
  cell('text', 'PlotQualitySite'), cell('null'), cell('real', 1), cell(item === null ? 'null' : 'text', item),
  cell(description === null ? 'null' : 'text', description), cell('null'), cell('null'),
  cell('integer', '-1'), cell('null'), cell('integer', '0'),
] });
const child = () => serverComponent(source('LongVegetationQualityMinimumPreferences.svelte'), 'LongVegetationQualityMinimumPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationQualityMinimumPreferences': prefs,
});

test('strict-Go wire and full definitions detach from real Svelte proxies without cloning a proxy', async () => {
  const reactive = proxy(structuredClone(wire.review));
  reactive.definitions.rows = [row('-4', 'Same', 'first'), row('-3', 'Same'), row('-2', null), row('-1', '')];
  assert.throws(() => structuredClone(reactive));
  const validated = prefs.validateLongVegetationQualityMinimumPreferences(reactive, scope);
  const copied = structuredClone(validated);
  reactive.values.siteMinimum = 'changed';
  reactive.definitions.rows[0].cells[3].text = 'changed';
  assert.equal(copied.values.siteMinimum, 'Poor');
  assert.equal(copied.definitions.rows[0].cells[3].text, 'Same');
  assert.equal(copied.definitions.rows.length, 4);
  assert.equal(copied.definitions.rows[2].cells[3].storage, 'null');
  assert.equal(copied.definitions.rows[3].cells[3].text, '');
  const session = ready();
  for (const key of fields) session.editMinimum(key, wire.request.proposed[key]);
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request);
  assert.deepEqual(session.view().outcome, wire.outcome);
});
test('empty suggestions never impose membership, bounds, trimming or case changes', async () => {
  for (const value of ['', ' poor ', 'CUSTOM', '界'.repeat(32), 'Poor']) {
    const session = ready(); session.editMinimum('siteMinimum', value);
    const changed = value !== 'Poor';
    await session.save(request => {
      assert.equal(request.proposed.siteMinimum, value);
      assert.equal(request.proposed.vegMinimum, 'Poor');
      return Promise.resolve(outcome(scope, changed));
    });
    assert.equal(session.reference().definitions.rows.length, 0);
    assert.equal(session.view().saved.siteMinimum, value);
  }
});
test('missing foreign malformed and incomplete physical reviews refuse without installing defaults', () => {
  const session = new prefs.LongVegetationQualityMinimumPreferencesSession(scope);
  assert.equal(session.view().draft, null); assert.equal(session.reference(), null);
  assert.throws(() => session.editMinimum('siteMinimum', 'Fair'), /Load saved/);
  for (const value of [null, {}, review(null),
    ...fields.flatMap(key => [null, 1, undefined, '\ud800'].map(value => review({ ...wire.review.values, [key]: value }))),
    ...Object.keys(scope).map(key => review(undefined, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationQualityMinimumPreferences(value, scope));
  }
  for (const corrupt of [
    value => { value.source = 'copied catalogue'; },
    value => { value.definitions.columns[0].type = 'TEXT'; delete value.definitions.columns[0].declaredType; },
    value => { value.definitions.columns = new Array(10); },
    value => { value.definitions.rows = new Array(1); },
    value => { value.definitions.rows = [row('-1', 'A'), row('-1', 'B')]; },
    value => { value.definitions.rows = [row('9223372036854775808', 'A')]; },
    value => { value.definitions.rows = [row('-1', 'A')]; value.definitions.rows[0].cells[0].text = 'DifferentList'; },
    value => { value.definitions.rows = [row('-1', 'A')]; value.definitions.rows[0].cells[3].text = '\ud800'; },
    value => { value.definitions.rows = [row('-1', 'A')]; value.definitions.rows[0].cells[3].real = 1; },
  ]) {
    const value = structuredClone(wire.review); corrupt(value);
    assert.throws(() => prefs.validateLongVegetationQualityMinimumPreferences(value, scope));
  }
});
test('retained invalid drafts block Save and preserve valid correction, Undo and unchanged historical NUL', async () => {
  const owner = { ...scope, contextId: 'retained-quality-minimum' };
  const session = prefs.longVegetationQualityMinimumPreferencesSession(owner);
  session.apply(review(undefined, owner));
  let calls = 0;
  for (const key of fields) for (const value of ['\ud800', '\udc00', 'new\0value', null, undefined, 1]) {
    session.editMinimum(key, value);
    assert.ok(prefs.longVegetationQualityMinimumPreferencesSession(owner).view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(owner); }), /complete Unicode/);
    session.editMinimum(key, 'Fair'); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, wire.review.values);
  }
  assert.equal(calls, 0);
  const historical = ready(); historical.apply(review({ ...wire.review.values, soilMinimum: 'old\0value' }));
  historical.editMinimum('siteMinimum', 'Fair');
  await historical.save(request => {
    assert.equal(request.proposed.soilMinimum, 'old\0value');
    return Promise.resolve(outcome());
  });
  assert.throws(() => session.editMinimum('foreign', 'Fair'), /Unsupported/);
});
test('held and committed-warning outcomes retain receipts and prevent read, edit, Undo or cancellation', async () => {
  const session = ready(); session.editMinimum('siteMinimum', 'Fair');
  let finish;
  const pending = session.save(() => new Promise(resolve => { finish = resolve; }));
  assert.throws(() => session.editMinimum('vegMinimum', 'Good'));
  assert.throws(() => session.beginLoad()); assert.throws(() => session.undo()); assert.throws(() => session.acknowledge());
  finish(outcome(scope, true, 'committed; do not replay')); await pending; session.acknowledge();
  assert.equal(session.view().saved.siteMinimum, 'Fair');
  assert.equal(session.view().receipts[0].outcome.committed, true);
  assert.equal(session.reference().definitions.rows.length, 0);
});
test('cancelled reads cannot overwrite retained references, while unknown outcomes require owned observation', async () => {
  const session = ready(), token = session.beginLoad();
  session.cancelLoad();
  const stale = review({ ...wire.review.values, siteMinimum: 'Old' });
  stale.definitions = { ...stale.definitions, rows: [row('-1', 'Stale')] };
  assert.equal(session.applyLoaded(stale, token), false);
  assert.equal(session.reference().definitions.rows.length, 0);
  session.editMinimum('siteMinimum', 'Fair'); await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true);
  await assert.rejects(session.save(async () => outcome()), /Load saved/);
  const receipt = session.view().receipts[0];
  session.applyLoaded(review(), session.beginLoad());
  assert.deepEqual(session.view().receipts[0], receipt);
  assert.equal(session.view().authorityUnknown, false);
});
test('actual panel handlers cancel stale references and enforce sibling holds before dispatch', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} },
  });
  let release, cancelled = 0, saves = 0;
  const holds = [];
  const context = componentFunctions('LongVegetationQualityMinimumPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    contextId: scope.contextId, preferences: session, view: session.view(), busy: false, error: '',
    operationHeld: false, mutationHeld: false, generation: 0, reads: new ReadRequests(),
    onReadBusyChange(value) { holds.push(value); },
    LongVegetationQualityMinimumPreferencesService: {
      GetLongVegetationQualityMinimumPreferences() {
        const pending = new Promise(resolve => { release = resolve; });
        pending.cancel = () => { cancelled++; };
        return pending;
      },
      SaveLongVegetationQualityMinimumPreferences(contextId, raw) {
        assert.equal(contextId, scope.contextId);
        assert.equal(JSON.parse(raw).proposed.siteMinimum, ' literal nonmember ');
        saves++; return Promise.resolve(outcome());
      },
    },
  });
  const pending = context.actions.load();
  assert.equal(holds.at(-1), true);
  context.actions.cancel();
  context.actions.edit('siteMinimum', ' literal nonmember ');
  const stale = review(); stale.definitions = { ...stale.definitions, rows: [row('-1', 'Stale')] };
  release(stale); await pending;
  assert.equal(cancelled, 1); assert.equal(holds.at(-1), false);
  assert.equal(session.reference().definitions.rows.length, 0);
  assert.equal(session.view().draft.siteMinimum, ' literal nonmember ');
  context.operationHeld = true; await context.actions.save(); assert.equal(saves, 0);
  context.mutationHeld = true; context.actions.edit('vegMinimum', 'Good');
  assert.equal(session.view().draft.vegMinimum, 'Poor');
  context.operationHeld = false; await context.actions.save(); assert.equal(saves, 1);
});
test('actual panel preserves labels, one unrestricted control per field and distinct physical suggestions', () => {
  const controls = ['SitePlotQuality', 'VegPlotQuality', 'SoilPlotQuality'];
  for (const initialized of [false, true]) {
    const session = initialized ? ready() : new prefs.LongVegetationQualityMinimumPreferencesSession(scope);
    if (initialized) {
      const value = review();
      value.definitions = { ...value.definitions, rows: [
        row('-4', 'Same', 'first'), row('-3', 'Same', 'second'), row('-2', null), row('-1', ''),
      ] };
      session.apply(value); session.editMinimum('siteMinimum', '\ud800');
    }
    const { body } = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } });
    assert.match(body, /Data Quality - Minimum/);
    for (let index = 0; index < fields.length; index++) {
      assert.equal((body.match(new RegExp(`data-source-control="${controls[index]}"`, 'g')) ?? []).length, 1);
      assert.ok(body.includes(`for="long-vegetation-quality-minimum-${fields[index]}"`));
    }
    for (const label of ['Site >=', 'Veg >=', 'Soil >=']) assert.ok(body.includes(label));
    assert.doesNotMatch(body, /maxlength=|pattern=|required=/);
    assert.match(body, /data-long-vegetation-quality-minimum-save[^>]*disabled/);
    if (initialized) {
      assert.doesNotMatch(body, /data-long-vegetation-quality-minimum-undo[^>]*disabled/);
      assert.equal((body.match(/<option value="Same"/g) ?? []).length, 2);
      assert.match(body, /first/); assert.match(body, /second/);
      assert.match(body, /Item null null/);
      assert.match(body, /Item text ""/);
    }
  }
});
test('actual report retains minimum validation on remount and independently gates root and sibling actions', () => {
  const reportSource = source('LongVegetationReport.svelte');
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `quality-minimum-composition-${enabled}` };
    const session = prefs.longVegetationQualityMinimumPreferencesSession(owner);
    session.apply(review(undefined, owner)); session.editMinimum('vegMinimum', '\ud800');
    const component = serverComponent(reportSource.replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const qualityMinimaEnabled = false;', `const qualityMinimaEnabled = ${enabled};`), 'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {},
        './longVegetationStatisticsPreferences': {}, './longVegetationSwitchPreferences': {},
        './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
        './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} },
        './longVegetationQualityMinimumPreferences': prefs, './LongVegetationQualityMinimumPreferences.svelte': { default: child() },
        './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
        './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
        './LongVegetationSwitchPreferences.svelte': { default() {} }, './longEnvironmentReport': {}, './longVegetationReport': {},
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
          view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
        } },
      });
    for (let mount = 0; mount < 2; mount++) {
      const barriers = [];
      const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
      assert.equal(barriers.at(-1), enabled);
      if (enabled) {
        assert.match(body, /data-long-vegetation-quality-minima/);
        assert.match(body, /data-long-vegetation-quality-minimum-save[^>]*disabled/);
        assert.doesNotMatch(body, /data-long-vegetation-quality-minimum-undo[^>]*disabled/);
      } else assert.doesNotMatch(body, /data-long-vegetation-quality-minima/);
    }
    session.undo();
  }
  assert.match(reportSource, /qualityMinimumView = qualityMinima.view\(\); invalidateReports\(\)/);
  assert.match(reportSource, /function invalidateReports\(\) \{\s*preview = null; workbookReview = null;\s*projectWorkspace\?\.invalidate\(\)/);
  assert.match(reportSource, /qualityNullBarrier \|\| qualityMinimumBarrier/);
  assert.match(reportSource, /unsubscribeQualityMinima\?\.\(\)/);
  const panel = source('LongVegetationQualityMinimumPreferences.svelte');
  assert.match(panel, /const value = event.currentTarget.value; edit\(field.key, value\)/);
  assert.doesNotMatch(panel, /reads\.track\(LongVegetationQualityMinimumPreferencesService\.Save/);
});
