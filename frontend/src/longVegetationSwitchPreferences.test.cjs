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
const prefs = loadTypeScript('longVegetationSwitchPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const titles = loadTypeScript('longVegetationPreferences.ts', { './reportTitlePreferences': shared });
const statistics = loadTypeScript('longVegetationStatisticsPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationSwitchWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values;
const fields = ['quickReport', 'spaceBetweenGroups', 'reportSummary', 'constantSpeciesList'];
const review = (values = wire.review.values, owner = scope) => ({ ...owner, values });
const outcome = (owner = scope, committed = true, errorMessage = '') =>
  ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
const ready = (owner = scope) => {
  const session = new prefs.LongVegetationSwitchPreferencesSession(owner);
  session.apply(review(undefined, owner)); return session;
};
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const child = () => serverComponent(source('LongVegetationSwitchPreferences.svelte'), 'LongVegetationSwitchPreferences.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationSwitchPreferences': prefs,
});

test('strict-Go boolean wire detaches actual Svelte proxies and transports exactly four scalar choices', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const validated = prefs.validateLongVegetationSwitchPreferences(reactive, scope);
  assert.deepEqual(structuredClone(validated), wire.review);
  reactive.values.quickReport = false; assert.equal(validated.values.quickReport, true);
  const session = ready(); for (const field of fields) session.editSwitch(field, wire.request.proposed[field]);
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all sixteen combinations use independently expected booleans without numeric or truthy coercion', async () => {
  for (let combination = 0; combination < 16; combination++) {
    const values = { quickReport: !!(combination & 1), spaceBetweenGroups: !!(combination & 2),
      reportSummary: !!(combination & 4), constantSpeciesList: !!(combination & 8) };
    const session = ready(); for (const field of fields) session.editSwitch(field, values[field]);
    let calls = 0; const changed = fields.some(field => values[field] !== wire.review.values[field]);
    await session.save(request => {
      calls++; assert.deepEqual(request, { expected: wire.review.values, proposed: values });
      return Promise.resolve(outcome(scope, changed));
    });
    assert.equal(calls, 1); session.acknowledge(); assert.deepEqual(session.view().saved, values);
  }
});
test('missing or foreign saved switches are unavailable and unloaded state never becomes a saved default', async () => {
  const session = new prefs.LongVegetationSwitchPreferencesSession(scope);
  assert.equal(session.view().draft, null); assert.equal(session.view().saved, null);
  assert.throws(() => session.editSwitch('quickReport', true), /Load saved/);
  await assert.rejects(session.save(async () => outcome()), /Load saved/);
  for (const value of [null, {}, review(null),
    ...fields.map(field => review({ ...wire.review.values, [field]: null })),
    ...fields.map(field => { const values = { ...wire.review.values }; delete values[field]; return review(values); }),
    ...Object.keys(scope).map(key => review(undefined, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationSwitchPreferences(value, scope), /no defaults applied/);
  }
});
test('malformed runtime boolean drafts retain errors while correction and Undo prevent any Save dispatch', async () => {
  const session = ready(); let calls = 0;
  for (const field of fields) for (const value of [null, '', 'false', 0, -1, undefined]) {
    session.editSwitch(field, value); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }), /four boolean/);
    session.editSwitch(field, true); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, wire.review.values);
  }
  assert.equal(calls, 0); assert.throws(() => session.editSwitch('foreign', true), /Unsupported/);
});
test('switch registries retain exact owned drafts without crossing title or statistics authority', () => {
  const owner = { ...scope, contextId: 'retained-switches' };
  const session = prefs.longVegetationSwitchPreferencesSession(owner); session.apply(review(undefined, owner));
  const title = titles.longVegetationPreferencesSession(owner); title.apply({ ...owner, values: { title: 'separate' } });
  const stats = statistics.longVegetationStatisticsPreferencesSession(owner); stats.apply({ ...owner, values: { avgType: 10, orderBy: 20 } });
  session.editSwitch('reportSummary', null);
  assert.ok(prefs.longVegetationSwitchPreferencesSession(owner).view().draftError);
  assert.equal(title.view().draftError, ''); assert.equal(stats.view().draftError, '');
  for (const key of ['project', 'projectPath', 'su', 'suPath']) {
    assert.throws(() => prefs.longVegetationSwitchPreferencesSession({ ...owner, [key]: 'foreign' }));
  }
});
test('held switch Save cannot cancel and committed-warning receipts survive acknowledgement', async () => {
  const session = ready(); session.editSwitch('reportSummary', false);
  let release, cancellations = 0;
  const pending = session.save(() => {
    const value = new Promise(done => { release = done; }); value.cancel = () => { cancellations++; }; return value;
  });
  assert.throws(() => session.editSwitch('quickReport', false)); assert.throws(() => session.beginLoad());
  assert.throws(() => session.undo()); assert.throws(() => session.acknowledge());
  release(outcome(scope, true, 'committed; do not replay')); await pending; session.acknowledge();
  assert.equal(cancellations, 0); assert.equal(session.view().saved.reportSummary, false);
  assert.equal(session.view().receipts[0].outcome.committed, true);
});
test('unknown responses lose authority and require explicit observation without erasing receipts', async () => {
  const session = ready(); session.editSwitch('quickReport', false);
  await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
  await assert.rejects(session.save(async () => outcome()), /Load saved report switches/);
  const receipt = session.view().receipts[0];
  session.applyLoaded(review(), session.beginLoad());
  assert.deepEqual(session.view().receipts[0], receipt); assert.equal(session.view().authorityUnknown, false);
});
test('actual switch component handlers cancel stale reads and refuse external held work before dispatch', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} } });
  let release, cancelled = 0, saves = 0; const held = [];
  const context = componentFunctions('LongVegetationSwitchPreferences.svelte', ['cancel', 'load', 'save', 'edit'], {
    contextId: scope.contextId, preferences: session, view: session.view(), busy: false, error: '',
    operationHeld: false, mutationHeld: false, generation: 0, reads: new ReadRequests(),
    onReadBusyChange(value) { held.push(value); },
    LongVegetationSwitchPreferencesService: {
      GetLongVegetationSwitchPreferences() {
        const value = new Promise(done => { release = done; }); value.cancel = () => { cancelled++; }; return value;
      },
      SaveLongVegetationSwitchPreferences(_id, raw) {
        saves++; assert.equal(JSON.parse(raw).proposed.reportSummary, false); return Promise.resolve(outcome());
      },
    },
  });
  const pending = context.actions.load(); assert.equal(held.at(-1), true);
  context.actions.cancel(); context.actions.edit('reportSummary', false);
  release(review()); await pending; assert.equal(cancelled, 1); assert.equal(held.at(-1), false);
  assert.equal(session.view().draft.reportSummary, false);
  context.operationHeld = true; await context.actions.save(); assert.equal(saves, 0);
  context.operationHeld = false; await context.actions.save(); assert.equal(saves, 1);
});
test('source captions and bindings render once with live correction/Undo and no loaded-value inference', () => {
  const captions = ['Screen report (skip some print page formatting - faster)', 'Space between groups',
    'Create report summary', 'Use constant species list'];
  const controls = ['optScreenReport', 'rptLvSpaceBetweenGroups', 'optCreateSummary', 'optConstantSppList'];
  assert.equal(prefs.longVegetationSwitchFields.length, 4);
  for (const initialized of [false, true]) {
    const session = initialized ? ready() : new prefs.LongVegetationSwitchPreferencesSession(scope);
    if (initialized) session.editSwitch('reportSummary', null);
    const { body } = render(child(), { props: { contextId: scope.contextId, preferences: session, onReadBusyChange() {} } });
    for (let index = 0; index < 4; index++) {
      assert.ok(body.includes(captions[index]));
      assert.equal((body.match(new RegExp(`data-source-control="${controls[index]}"`, 'g')) ?? []).length, 1);
      assert.ok(body.includes(`for="long-vegetation-switch-${fields[index]}"`));
    }
    assert.match(body, /data-long-vegetation-switch-save[^>]*disabled/);
    if (initialized) assert.doesNotMatch(body, /data-long-vegetation-switch-undo[^>]*disabled/);
    else assert.match(body, /unloaded boxes do not represent saved values/);
  }
});
test('actual report composition preserves default-off switches and propagates retained draft barriers to root', () => {
  const reportSource = source('LongVegetationReport.svelte');
  for (const enabled of [false, true]) {
    const owner = { ...scope, contextId: `switch-composition-${enabled}` };
    const session = prefs.longVegetationSwitchPreferencesSession(owner); session.apply(review(undefined, owner));
    session.editSwitch('reportSummary', null);
    const component = serverComponent(reportSource.replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const switchesEnabled = false;', `const switchesEnabled = ${enabled};`), 'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': {},
        './longVegetationStatisticsPreferences': {}, './longVegetationSwitchPreferences': prefs,
        './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
        './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
        './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
        './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
        './LongVegetationSwitchPreferences.svelte': { default: child() }, './longEnvironmentReport': {}, './longVegetationReport': {},
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
          view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
        } },
      });
    const barriers = []; const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
    assert.equal(barriers.at(-1), enabled);
    if (enabled) assert.match(body, /data-long-vegetation-switches/); else assert.doesNotMatch(body, /data-long-vegetation-switches/);
  }
  const panel = source('LongVegetationSwitchPreferences.svelte');
  assert.match(panel, /const value = event.currentTarget.checked; edit\(field.key, value\)/);
  assert.doesNotMatch(panel, /reads\.track\(LongVegetationSwitchPreferencesService\.Save/);
  assert.match(reportSource, /switchView = switches.view\(\); invalidateReports\(\)/);
  assert.match(reportSource, /function invalidateReports\(\) \{\s*preview = null; workbookReview = null;\s*projectWorkspace\?\.invalidate\(\)/);
  assert.ok(reportSource.indexOf('data-vegetation-workbook-unknown') < reportSource.indexOf('<LongVegetationSwitchPreferences'));
});
