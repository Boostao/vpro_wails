const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { render } = require('svelte/server');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', { './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations });
const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const lifeform = loadTypeScript('lifeformSummary.ts');
const summary = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform });
const prefs = loadTypeScript('summaryVegetationPreferences.ts', {
  './googleEarthReview': earth, './reportTitlePreferences': shared, './lifeformSummary': lifeform, './summaryVegetation': summary,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationPreferencesWire.json'), 'utf8'));
const { values: unused, previewEnabled: unusedPreview, selectionAvailable: unusedSelection, ...scope } = wire.review;
const values = (groupBy = 1, average = 1, display = 1, showClass = 1, valueLimit = 0) =>
  ({ groupBy, average, display, showClass, valueLimit });
const review = (saved = values(), previewEnabled = true, owner = scope) =>
  ({ ...owner, values: saved, previewEnabled, selectionAvailable: !([2, 3].includes(saved.groupBy) && saved.display === 2) });
const ready = (saved = values(), backend = true, client = true) => {
  const session = new prefs.SummaryVegetationPreferencesSession(scope, client);
  session.apply(review(saved, backend));
  return session;
};
const outcome = (committed, errorMessage = '') => ({ contextId: scope.contextId, changed: committed, committed, errorMessage });

test('production Go wire validates actual reactive proxies and maps source group separately', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateSummaryVegetationPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.groupBy = 2;
  assert.equal(detached.values.groupBy, 1);
  const session = ready();
  session.editOption('groupBy', '4'); session.editOption('display', '6'); session.editOption('showClass', '20');
  let sent;
  await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all original option combinations and independent flags retain historical noops without enabling source failures', async () => {
  for (const backend of [false, true]) for (const client of [false, true])
    for (const group of [1, 2, 3, 4]) for (const average of [1, 2])
      for (const display of [1, 2, 3, 4, 5, 6]) for (const show of [1, 10, 20]) {
        const saved = values(group, average, display, show, 3.5);
        const session = ready(saved, backend, client);
        assert.equal(session.previewEnabled(), backend && client);
        assert.equal(session.view().draftError, '');
        await session.save(request => {
          assert.deepEqual(request, { expected: saved, proposed: saved });
          return Promise.resolve(outcome(false));
        });
        session.acknowledge(); session.editOption('groupBy', '4'); session.editOption('display', '6');
        session.editOption('average', average === 1 ? '2' : '1');
        const permitted = backend && client;
        assert.equal(session.view().draftError === '', permitted);
        let dispatch = 0;
        const save = () => session.save(request => {
          dispatch++; assert.equal(request.proposed.valueLimit, display === 6 ? 3.5 : 0);
          return Promise.resolve(outcome(true));
        });
        if (permitted) { await save(); assert.equal(dispatch, 1); session.acknowledge(); }
        else { await assert.rejects(save()); assert.equal(dispatch, 0); }
        session.undo(); assert.equal(session.view().draftError, '');
      }
});
test('coupled display callback clears only changed display2/4/5/6 and preserves away/back provenance', async () => {
  for (const old of [1, 2, 3, 4, 5, 6]) for (const next of [1, 2, 3, 4, 5, 6]) {
    const saved = values(4, 1, old, 1, 7.25), session = ready(saved);
    session.editOption('display', String(next));
    const expected = next !== old && ![1, 3].includes(next) ? 0 : 7.25;
    assert.equal(session.view().draft.valueLimit, expected);
    await session.save(request => {
      assert.equal(request.proposed.valueLimit, expected);
      return Promise.resolve(outcome(old !== next));
    });
  }
  const session = ready(values(1, 1, 1, 1, 7.25));
  session.editOption('display', '4'); session.editOption('display', '1');
  assert.equal(session.view().draft.valueLimit, 0);
  await session.save(request => {
    assert.equal(request.expected.display, request.proposed.display);
    assert.equal(request.expected.valueLimit, 7.25); assert.equal(request.proposed.valueLimit, 0);
    return Promise.resolve(outcome(true));
  });
});
test('unrelated valid average and Show edits never clear dormant historical limits', async () => {
  for (const display of [4, 5, 6]) for (const field of ['average', 'showClass']) {
    const saved = values(4, 1, display, 1, 7.25), session = ready(saved);
    session.editOption(field, field === 'average' ? '2' : '20');
    assert.equal(session.view().draft.valueLimit, 7.25);
    await session.save(request => {
      assert.equal(request.proposed.valueLimit, 7.25);
      return Promise.resolve(outcome(true));
    });
  }
});
test('strict missing foreign malformed or inconsistent wires never invent defaults', () => {
  const baseline = review();
  for (const invalid of [null, {}, [], { ...baseline, unexpected: true },
    { ...baseline, selectionAvailable: false }, { ...baseline, previewEnabled: 'true' },
    { ...baseline, values: { ...baseline.values, unknown: 1 } },
    ...Object.keys(scope).map(key => ({ ...baseline, [key]: 'foreign' })),
    ...['groupBy', 'average', 'display', 'showClass'].flatMap(key =>
      [null, '1', true, NaN, Infinity, 0, -1, 100].map(value => ({ ...baseline, values: { ...baseline.values, [key]: value } }))),
    ...[null, '0', true, NaN, Infinity].map(valueLimit => ({ ...baseline, values: { ...baseline.values, valueLimit } })),
  ]) assert.throws(() => prefs.validateSummaryVegetationPreferences(invalid, scope), /no defaults applied/);
  assert.throws(() => ready(values(), true, 'true'), /immutable/);
});
test('raw invalid drafts remain correctable and retained without Save dispatch', async () => {
  const session = ready();
  for (const field of ['groupBy', 'average', 'display', 'showClass']) {
    for (const raw of ['', ' ', '01', '1.0', '1e0', '0', 'bad', '\ud800', null, 1]) {
      session.editOption(field, raw); assert.equal(session.view().draft[field], raw);
      assert.ok(session.view().draftError);
      let calls = 0;
      await assert.rejects(session.save(async () => { calls++; return outcome(true); }));
      assert.equal(calls, 0);
      session.editOption(field, '1'); assert.equal(session.view().draftError, '');
      session.undo();
    }
  }
  session.editOption('groupBy', '2'); session.editOption('display', '2');
  assert.match(session.view().draftError, /criteria type mismatch/);
  session.editOption('groupBy', '4'); assert.equal(session.view().draftError, '');
  assert.throws(() => session.editOption('valueLimit', '3'), /editable/);
});
test('registry remount retains owner drafts, immutable flags and callback provenance', () => {
  const registry = prefs.createSummaryVegetationPreferencesRegistry(true);
  const session = registry(scope); session.apply(review(values(1, 1, 1, 1, 7.25)));
  session.editOption('display', '4'); session.editOption('display', '1'); session.editOption('average', '');
  assert.equal(registry(scope), session);
  assert.equal(registry(scope).view().draft.valueLimit, 0);
  assert.ok(registry(scope).view().draftError);
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() => registry({ ...scope, [key]: 'other' }), /different owned/);
  session.editOption('average', '2'); assert.equal(session.view().draftError, '');
  session.undo(); assert.equal(session.view().draft.valueLimit, 7.25);
});
test('held saves, late reads and known unknown receipts remain separate across remount', async () => {
  const session = ready(); session.editOption('groupBy', '4');
  const oldRead = session.beginLoad(); let release, cancelled = 0;
  const pending = session.save(() => {
    const request = new Promise(resolve => { release = resolve; });
    request.cancel = () => { cancelled++; }; return request;
  });
  for (const action of [() => session.editOption('groupBy', '3'), () => session.undo(), () => session.beginLoad(), () => session.acknowledge()]) assert.throws(action);
  assert.equal(session.applyLoaded(review(), oldRead), false);
  release(outcome(true, 'Committed, do not replay')); await pending; session.acknowledge();
  assert.equal(cancelled, 0); assert.equal(session.view().saved.groupBy, 4);
  assert.equal(session.view().receipts.length, 1);
  session.editOption('average', '2'); await session.save(async () => null); session.acknowledge();
  assert.equal(session.view().saved, null); assert.equal(session.view().authorityUnknown, true);
  assert.equal(session.view().receipts.length, 2);
  await assert.rejects(session.save(async () => outcome(true)), /Load/);
  const token = session.beginLoad(); session.applyLoaded(review(values(4)), token, true);
  assert.equal(session.view().authorityUnknown, false); assert.equal(session.view().draft.average, '2');
  session.undo(); assert.equal(session.view().receipts.length, 2);
});

const component = readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
function panel(session, environment = { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_PREFERENCES: 'true' }) {
  return serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': summary,
    './summaryVegetationPreferenceSessions': { summaryVegetationPreferenceSession: environment.VITE_SUMMARY_VEGETATION_PREFERENCES === 'true' ? () => session : null },
    '../bindings/github.com/boostao/vpro-wails': {},
  }, environment);
}
test('compiled saved-option panel has one live source control per field and no render-time mutation', async () => {
  const owner = { ...scope, contextId: 'normal-panel', su: 'NormalSU' };
  const session = new prefs.SummaryVegetationPreferencesSession(owner, true);
  session.apply(review(values(4, 2, 6, 20, 3.5), true, owner));
  const holds = [];
  let body = render(panel(session), { props: { ...owner, onBusyChange: flag => holds.push(flag) } }).body;
  for (const control of ['rptSvGroupBy', 'rptSvValueEq', 'OptShowClassValue', 'rptSvAvgType']) {
    assert.equal(body.split(`data-source-control="${control}"`).length - 1, 1);
  }
  assert.match(body, /data-summary-preferences-save/);
  assert.match(body, /<option value="14" selected/);
  assert.doesNotMatch(body.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
  session.editOption('average', '');
  body = render(panel(session), { props: { ...owner, onBusyChange: flag => holds.push(flag) } }).body;
  assert.match(body.match(/<button[^>]*data-summary-preferences-save[^>]*>/)[0], /\bdisabled\b/);
  assert.match(body.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
  assert.doesNotMatch(body.match(/<select[^>]*data-summary-average[^>]*>/)[0], /\bdisabled\b/);
  assert.doesNotMatch(body.match(/<button[^>]*data-summary-preferences-undo[^>]*>/)[0], /\bdisabled\b/);
  assert.ok(holds.includes(true)); session.undo();
  await session.save(async () => ({ contextId: owner.contextId, changed: false, committed: false, errorMessage: '' })); session.acknowledge();
  body = render(panel(session), { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(body, /data-summary-preference-receipt/);
  const disabled = render(panel(session, { VITE_SUMMARY_VEGETATION: 'true' }), { props: { ...owner, onBusyChange() {} } }).body;
  assert.doesNotMatch(disabled, /data-summary-preferences/);
});
test('actual control handlers capture DOM values before preference validation and refuse duplicate field editors', () => {
  const session = ready(values(1, 1, 1, 1, 7.25));
  const actions = componentFunctions('SummaryVegetationReport.svelte',
    ['changeAverage', 'changeDisplay', 'changeShowClass', 'changeGroup', 'editPreference'], {
      preferences: session, editHeld: false, cancelPreferenceRead() {}, preferenceReadError: '',
    }).actions;
  actions.changeGroup({ currentTarget: { value: '14' } });
  actions.changeDisplay({ currentTarget: { value: '4' } });
  actions.changeDisplay({ currentTarget: { value: '1' } });
  actions.changeAverage({ currentTarget: { value: '2' } });
  actions.changeShowClass({ currentTarget: { value: '20' } });
  assert.deepEqual(session.view().draft, { groupBy: '4', average: '2', display: '1', showClass: '20', valueLimit: 0 });
});
test('saved reads cancel by generation and cannot populate remounted Summary controls', async () => {
  const session = ready(); let release, cancelled = 0, reports = 0;
  const context = componentFunctions('SummaryVegetationReport.svelte', ['loadPreferences', 'cancelPreferenceRead'], {
    preferences: session, viewState: { busy: false }, operationHeld: false, preferenceHeld: false,
    preferenceGeneration: 0, preferenceReadBusy: false, preferenceReadError: '', contextId: scope.contextId,
    reportBusy() { reports++; },
    preferenceReads: { track: request => request, cancelAll() { cancelled++; } },
    SummaryVegetationPreferencesService: { GetSummaryVegetationPreferences() { return new Promise(resolve => { release = resolve; }); } },
  });
  const pending = context.actions.loadPreferences();
  assert.equal(context.preferenceReadBusy, true);
  context.actions.cancelPreferenceRead();
  release(review(values(4))); await pending;
  assert.equal(session.view().saved.groupBy, 1);
  assert.equal(context.preferenceReadBusy, false); assert.ok(cancelled >= 2); assert.ok(reports >= 2);
});
test('saved preference mutation barrier remains rooted independently of active report tab', async () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  assert.match(source, /editorBusy = \$derived\([^;]*summaryPreferenceBusy/);
  assert.match(source, /summaryPreferenceBusy = views\.some\(current => !!current && \(current\.busy \|\| current\.blocked \|\| !!current\.draftError\)\)/);
  assert.match(source, /const views = \[preferences\?\.view\(\), quality\?\.view\(\), presentation\?\.view\(\)\]/);
  const context = componentFunctions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy'], {
    summaryPreferenceBusy: true, pictureCreationBusy: false, pictureLifecycleBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
  });
  await context.actions.loadPlots(0); await context.actions.refresh(); await context.actions.loadHierarchy();
});

test('failed owned reload blocks Save and Preview across remount until a successful explicit Load', async () => {
  const owner = { ...scope, contextId: 'reload-failure-panel', su: 'NormalSU' };
  const registry = prefs.createSummaryVegetationPreferencesRegistry(true);
  const session = registry(owner); session.apply(review(values(4, 1, 6), true, owner));
  const context = componentFunctions('SummaryVegetationReport.svelte', ['loadPreferences', 'cancelPreferenceRead'], {
    preferences: session, viewState: { busy: false }, operationHeld: false, preferenceHeld: false,
    preferenceGeneration: 0, preferenceReadBusy: false, preferenceReadError: '', contextId: owner.contextId,
    reportBusy() {}, preferenceReads: { track: request => request, cancelAll() {} },
    SummaryVegetationPreferencesService: {
      GetSummaryVegetationPreferences() { return Promise.reject(new Error('Owned reload unavailable')); },
    },
  });
  await context.actions.loadPreferences();
  assert.match(session.view().readError, /Owned reload unavailable/);
  let calls = 0;
  await assert.rejects(session.save(async () => { calls++; return outcome(false); }), /last read failed/);
  assert.equal(calls, 0);
  session.editOption('average', '2'); session.undo();
  assert.match(session.view().readError, /Owned reload unavailable/);
  const remounted = registry(owner);
  let body = render(panel(remounted), { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(body, /Owned reload unavailable/);
  for (const selector of ['data-summary-preferences-save', 'data-summary-preview']) {
    assert.match(body.match(new RegExp(`<button[^>]*${selector}[^>]*>`))[0], /\bdisabled\b/);
  }
  assert.doesNotMatch(body.match(/<button[^>]*data-summary-preferences-load[^>]*>/)[0], /\bdisabled\b/);
  context.SummaryVegetationPreferencesService.GetSummaryVegetationPreferences = async () => review(values(4, 1, 6), true, owner);
  await context.actions.loadPreferences();
  assert.equal(session.view().readError, '');
  body = render(panel(remounted), { props: { ...owner, onBusyChange() {} } }).body;
  for (const selector of ['data-summary-preferences-save', 'data-summary-preview']) {
    assert.doesNotMatch(body.match(new RegExp(`<button[^>]*${selector}[^>]*>`))[0], /\bdisabled\b/);
  }
});

test('unchanged historical source failure is preview-unavailable rather than a perpetual draft close hold', async () => {
  for (const groupBy of [2, 3]) {
    const owner = { ...scope, contextId: `historical-panel-${groupBy}`, su: 'NormalSU' };
    const session = new prefs.SummaryVegetationPreferencesSession(owner, true);
    session.apply(review(values(groupBy, 1, 2), true, owner));
    const holds = [];
    const body = render(panel(session), { props: { ...owner, onBusyChange: flag => holds.push(flag) } }).body;
    assert.match(body.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
    assert.equal(session.view().draftError, '');
    assert.ok(holds.length > 0); assert.ok(holds.every(flag => flag === false));
    session.undo();
    await session.save(async request => {
      assert.deepEqual(request.expected, request.proposed);
      return { contextId: owner.contextId, changed: false, committed: false, errorMessage: '' };
    });
    session.acknowledge();
    const after = [];
    const retainedBody = render(panel(session), { props: { ...owner, onBusyChange: flag => after.push(flag) } }).body;
    assert.match(retainedBody, /data-summary-preference-receipt/);
    assert.ok(after.length > 0); assert.ok(after.every(flag => flag === false));
    session.editOption('average', '2');
    assert.ok(session.view().draftError);
    const changed = [];
    const changedBody = render(panel(session), { props: { ...owner, onBusyChange: flag => changed.push(flag) } }).body;
    assert.match(changedBody.match(/<button[^>]*data-summary-preferences-save[^>]*>/)[0], /\bdisabled\b/);
    assert.ok(changed.includes(true));
    session.undo(); assert.equal(session.view().draftError, '');
  }
});
