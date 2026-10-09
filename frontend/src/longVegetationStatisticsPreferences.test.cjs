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
const prefs = loadTypeScript('longVegetationStatisticsPreferences.ts', { './reportTitlePreferences': shared, './googleEarthReview': earth });
const titles = loadTypeScript('longVegetationPreferences.ts', { './reportTitlePreferences': shared });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'longVegetationStatisticsWire.json'), 'utf8'));
const scope = { ...wire.review }; delete scope.values;
const review = (values = wire.review.values, owner = scope) => ({ ...owner, values });
const outcome = (owner = scope, committed = true, errorMessage = '') =>
  ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
const ready = (owner = scope) => {
  const session = new prefs.LongVegetationStatisticsPreferencesSession(owner);
  session.apply(review(undefined, owner)); return session;
};
test('strict-Go statistics round trip detaches real Svelte proxies and uses exact numeric request fields', async () => {
  const reactive = proxy(structuredClone(wire.review));
  assert.throws(() => structuredClone(reactive));
  const detached = prefs.validateLongVegetationStatisticsPreferences(reactive, scope);
  assert.deepEqual(structuredClone(detached), wire.review);
  reactive.values.avgType = 20; assert.equal(detached.values.avgType, 10);
  assert.doesNotThrow(() => structuredClone(detached));
  const session = ready(); session.editAverage('20'); session.editOrder('10');
  let sent; await session.save(request => { sent = request; return Promise.resolve(wire.outcome); });
  assert.deepEqual(sent, wire.request); assert.deepEqual(session.view().outcome, wire.outcome);
});
test('all four Average/Order combinations preserve independently expected numeric payloads', async () => {
  for (const avgType of [10, 20]) for (const orderBy of [10, 20]) {
    const session = ready(); session.editAverage(String(avgType)); session.editOrder(String(orderBy));
    let calls = 0;
    await session.save(request => {
      calls++; assert.deepEqual(request, { expected: { avgType: 10, orderBy: 20 }, proposed: { avgType, orderBy } });
      return Promise.resolve(outcome(scope, avgType !== 10 || orderBy !== 20));
    });
    assert.equal(calls, 1); session.acknowledge(); assert.deepEqual(session.view().saved, { avgType, orderBy });
  }
});
test('missing, unsupported, wrongly typed and foreign choices never become defaults', () => {
  for (const value of [null, {}, review(null), review({ avgType: 10 }), review({ avgType: '10', orderBy: 20 }),
    review({ avgType: 15, orderBy: 20 }), review({ avgType: 10, orderBy: 0 }),
    ...Object.keys(scope).map(key => review(undefined, { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongVegetationStatisticsPreferences(value, scope), /no defaults applied/);
  }
});
test('raw empty, malformed and coerced select values retain validation, correction and Undo without dispatch', async () => {
  const session = ready(); let calls = 0;
  for (const edit of ['editAverage', 'editOrder']) for (const value of ['', '0', '15', ' 10', '10.0', '1e1', '\ud800']) {
    session[edit](value); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => { calls++; return outcome(); }), /supported Average/);
    session[edit]('10'); assert.equal(session.view().draftError, '');
    session.undo(); assert.deepEqual(session.view().draft, { avgType: '10', orderBy: '20' });
  }
  assert.equal(calls, 0);
});
test('exact-owner statistics drafts/receipts survive remount without crossing title or Environment registries', () => {
  const owner = { ...scope, contextId: 'statistics-retained' };
  const session = prefs.longVegetationStatisticsPreferencesSession(owner);
  const title = titles.longVegetationPreferencesSession(owner);
  title.apply({ ...owner, values: { title: 'unchanged' } }); session.apply(review(undefined, owner));
  session.editOrder('bad');
  assert.equal(prefs.longVegetationStatisticsPreferencesSession(owner).view().draftError, session.view().draftError);
  assert.equal(title.view().draft.title, 'unchanged'); assert.equal(title.view().draftError, '');
  for (const key of ['project', 'projectPath', 'su', 'suPath']) {
    assert.throws(() => prefs.longVegetationStatisticsPreferencesSession({ ...owner, [key]: 'foreign' }));
  }
});
test('held statistics Save is irreversible and retains committed-error receipt after acknowledgement', async () => {
  const session = ready(); session.editAverage('20');
  let release, cancellations = 0;
  const pending = session.save(() => {
    const request = new Promise(done => { release = done; }); request.cancel = () => { cancellations++; }; return request;
  });
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.throws(() => session.editAverage('10')); assert.throws(() => session.editOrder('10'));
  assert.throws(() => session.undo()); assert.throws(() => session.beginLoad()); assert.throws(() => session.acknowledge());
  release(outcome(scope, true, 'committed; do not replay')); await pending; session.acknowledge();
  assert.equal(cancellations, 0); assert.equal(session.view().saved.avgType, 20);
  assert.equal(session.view().receipts[0].outcome.committed, true);
});
test('unknown or contradictory responses lose authority, require explicit observation and retain receipts', async () => {
  for (const response of [null, { ...wire.outcome, contextId: 'foreign' },
    { ...wire.outcome, changed: false }, { ...wire.outcome, errorMessage: '\ud800' },
    outcome(scope, false)]) {
    const session = ready(); session.editOrder('10');
    await session.save(async () => response); session.acknowledge();
    assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
    await assert.rejects(session.save(async () => outcome()), /Load saved Average\/Order/);
    const receipt = session.view().receipts[0]; session.applyLoaded(review(), session.beginLoad());
    assert.equal(session.view().authorityUnknown, false); assert.deepEqual(session.view().receipts[0], receipt);
  }
});
test('stale/cancelled loads cannot overwrite edits and rejected saves retain known originals for retry', async () => {
  const session = ready(); const token = session.beginLoad(); session.editOrder('10');
  assert.equal(session.applyLoaded(review({ avgType: 20, orderBy: 20 }), token), false);
  await session.save(async () => outcome(scope, false, 'changed; reload'));
  session.acknowledge(); assert.deepEqual(session.view().saved, wire.review.values);
  assert.deepEqual(session.view().draft, { avgType: '10', orderBy: '10' });
  await session.save(async () => outcome()); assert.equal(session.view().saved.orderBy, 10);
});
test('actual statistics handlers cancel borrowed reads, suppress stale loads and dispatch strict JSON once', async () => {
  const session = ready();
  const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {
    Call: {}, CancelledRejectionError: class extends Error {},
  } });
  let release, cancellations = 0, saves = 0;
  const context = componentFunctions('LongVegetationReport.svelte',
    ['cancel', 'loadSavedStatistics', 'saveStatistics', 'editAverage', 'editOrder'], {
      preferences: null, statistics: session, statisticsView: session.view(), busy: false, operationHeld: false,
      workbookBarrier: false, preferenceBarrier: false, titleBarrier: false, switchBarrier: false, thresholdBarrier: false, groupBarrier: false, qualityNullBarrier: false, generation: 0, error: '',
      reads: new ReadRequests(), contextId: scope.contextId, reportBusy() {},
      LongVegetationStatisticsPreferencesService: {
        GetLongVegetationStatisticsPreferences() {
          const read = new Promise(done => { release = done; }); read.cancel = () => { cancellations++; }; return read;
        },
        SaveLongVegetationStatisticsPreferences(_id, raw) {
          saves++; assert.deepEqual(JSON.parse(raw), { expected: { avgType: 10, orderBy: 20 }, proposed: { avgType: 20, orderBy: 10 } });
          return Promise.resolve(outcome());
        },
      },
    });
  const pending = context.actions.loadSavedStatistics(); context.actions.editAverage('20'); context.actions.editOrder('10');
  release(review()); await pending; assert.equal(cancellations, 1);
  context.operationHeld = true; await context.actions.saveStatistics(); assert.equal(saves, 0);
  context.operationHeld = false; await context.actions.saveStatistics(); assert.equal(saves, 1);
});
test('independent default/title/statistics gates render live labelled choices and correction while blocking root work', () => {
  const source = readFileSync(path.join(__dirname, 'LongVegetationReport.svelte'), 'utf8');
  for (const enabled of [false, true]) for (const titleEnabled of [false, true]) {
    const owner = { ...scope, contextId: `statistics-render-${enabled}-${titleEnabled}` };
    const session = prefs.longVegetationStatisticsPreferencesSession(owner); session.apply(review(undefined, owner)); session.editAverage('');
    const component = serverComponent(source.replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'false')
      .replace('const statisticsEnabled = false;', `const statisticsEnabled = ${enabled};`)
      .replace('const preferencesEnabled = false;', `const preferencesEnabled = ${titleEnabled};`), 'LongVegetationReport.svelte', {
        '../bindings/github.com/boostao/vpro-wails': {}, './longVegetationPreferences': titles,
        './longVegetationStatisticsPreferences': prefs, './longEnvironmentReport': {}, './longVegetationReport': {},
        './longVegetationSwitchPreferences': {},
        './longVegetationThresholdPreferences': {}, './LongVegetationThresholdPreferences.svelte': { default() {} },
        './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences': {}, './LongVegetationGroupPreferences.svelte': { default() {} },
        './LongVegetationSwitchPreferences.svelte': { default() {} },
        './vegetationWorkbook': { VegetationWorkbookPublicationSession: class {
          view() { return { busy: false, blocked: false }; } subscribe() { return () => {}; }
        } },
      });
    const barriers = []; const { body } = render(component, { props: { ...owner, onBusyChange: value => barriers.push(value) } });
    assert.equal(barriers.at(-1), enabled);
    if (enabled) {
      assert.equal((body.match(/id="long-vegetation-average"/g) ?? []).length, 1);
      assert.match(body, /for="long-vegetation-average">Calculate/); assert.match(body, /for="long-vegetation-order">Order by/);
      assert.match(body, /value="10">By n Plots/); assert.match(body, /value="20">Characteristic/);
      assert.match(body, /value="10"[^>]*>Species/); assert.match(body, /value="20"[^>]*>Presence/);
      assert.match(body, /data-long-vegetation-statistics-save[^>]*disabled/);
      assert.doesNotMatch(body, /data-long-vegetation-statistics-undo[^>]*disabled/);
    } else assert.doesNotMatch(body, /id="long-vegetation-average"/);
  }
  assert.match(source, /const value = event.currentTarget.value; editAverage\(value\)/);
  assert.match(source, /const value = event.currentTarget.value; editOrder\(value\)/);
  assert.doesNotMatch(source, /reads\.track\(LongVegetationStatisticsPreferencesService\.Save/);
  assert.match(source, /settings\.average !==/); assert.match(source, /settings\.order !==/);
});
