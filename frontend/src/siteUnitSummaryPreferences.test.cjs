const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const names = loadTypeScript('reportUnitNames.ts', { './projectMetadataRestore': cells });
const summary = loadTypeScript('siteUnitSummary.ts', { './projectMetadataRestore': cells, './reportUnitNames': names });
const prefs = loadTypeScript('siteUnitSummaryPreferences.ts', {
  './googleEarthReview': earth, './qualityEditor': quality, './siteUnitSummary': summary,
});
const scope = { contextId: 'summary-preferences', project: 'Sample', projectPath: 'C:\\Sample.db', su: 'Selected', suPath: 'C:\\external.db' };
const review = (method = 2, siteUnitType = 1) => ({ ...scope, method, siteUnitType });
const outcome = (committed = true, errorMessage = '') => ({ contextId: scope.contextId, committed, changed: committed, errorMessage });
function ready() {
  const session = new prefs.SiteUnitSummaryPreferencesSession(scope);
  session.applyLoaded(review(), session.beginLoad()); session.editMethod(1); return session;
}
test('summary preferences start without defaults and preserve saved/unavailable values through explicit loading', async () => {
  const session = new prefs.SiteUnitSummaryPreferencesSession(scope);
  assert.equal(session.view().draft, null);
  assert.throws(() => session.undo()); assert.throws(() => session.editMethod(1));
  await assert.rejects(session.save(async () => outcome()));
  for (const unitType of [1, 2, 3]) {
    session.applyLoaded(review(2, unitType), session.beginLoad());
    assert.deepEqual(session.view().draft, { method: 2, siteUnitType: unitType });
    assert.equal(session.view().receipts.length, 0);
  }
  session.selectNormalSU(); assert.equal(session.view().draft.siteUnitType, 1);
  session.undo(); assert.equal(session.view().draft.siteUnitType, 3);
  await assert.rejects(session.save(async () => outcome()), /normal-SU/);
});
test('summary loads cannot overwrite later edits, Undo or cancellation', () => {
  const session = ready();
  const token = session.beginLoad(); session.editMethod(2);
  assert.equal(session.applyLoaded(review(1), token), false);
  const cancelled = session.beginLoad(); session.cancelLoad();
  assert.equal(session.applyLoaded(review(1), cancelled), false);
  const undo = session.beginLoad(); session.undo();
  assert.equal(session.applyLoaded(review(1), undo), false);
  assert.equal(session.view().draft.method, 2);
  assert.throws(() => session.applyLoaded({ ...review(), contextId: 'stale' }, session.beginLoad()));
});
test('summary preference writes freeze requests, enforce acknowledgement and retain committed warnings', async () => {
  const session = ready(); let release, captured;
  const saved = session.save(request => { captured = request; return new Promise(resolve => { release = resolve; }); });
  assert.equal(session.view().busy, true); assert.throws(() => session.acknowledge()); assert.throws(() => session.editMethod(2));
  captured.proposed.method = 2;
  release(outcome(true, 'Committed; do not replay.'));
  await saved;
  assert.equal(session.view().saved.method, 1); assert.equal(session.view().receipts[0].request.proposed.method, 1);
  assert.equal(session.view().blocked, true); assert.throws(() => session.undo());
  session.acknowledge(); session.undo(); assert.equal(session.view().draft.method, 1);
});
test('summary no-op and rejected writes remain distinct without invented commits', async () => {
  const session = ready();
  await session.save(async () => outcome(false, 'Collision; reload.'));
  assert.equal(session.view().saved.method, 2); assert.equal(session.view().draft.method, 1);
  session.acknowledge(); session.undo();
  await session.save(async () => outcome(false));
  assert.equal(session.view().receipts.at(-1).outcome.committed, false);
  session.acknowledge();
});
test('summary unknown and contradictory receipts require explicit owned reload and never replay', async () => {
  for (const result of [null, { ...outcome(), contextId: 'foreign' }, { ...outcome(), changed: false },
    { ...outcome(), errorMessage: '\ud800' }, outcome(false)]) {
    const session = ready();
    await session.save(async () => result);
    assert.equal(session.view().authorityUnknown, true); assert.equal(session.view().saved, null);
    assert.equal(session.view().receipts[0].kind, 'unknown');
    assert.throws(() => session.undo()); await assert.rejects(session.save(async () => outcome()));
    session.acknowledge(); await assert.rejects(session.save(async () => outcome()));
    session.applyLoaded(review(1), session.beginLoad());
    assert.equal(session.view().authorityUnknown, false); assert.equal(session.view().receipts[0].kind, 'unknown');
  }
});
test('summary draft errors, ownership and receipts survive remounts without resetting drafts', async () => {
  const first = prefs.siteUnitSummaryPreferencesSession(scope);
  first.applyLoaded(review(), first.beginLoad()); first.editMethod(99);
  const remount = prefs.siteUnitSummaryPreferencesSession({ ...scope });
  assert.equal(first, remount); assert.notEqual(remount.view().draftError, '');
  assert.throws(() => prefs.siteUnitSummaryPreferencesSession({ ...scope, suPath: 'foreign' }));
  remount.editMethod(1); assert.equal(remount.view().draftError, '');
  await remount.save(async () => { throw new Error('lost acknowledgement'); });
  assert.equal(prefs.siteUnitSummaryPreferencesSession(scope).view().authorityUnknown, true);
  remount.acknowledge(); remount.applyLoaded(review(1), remount.beginLoad());
  assert.equal(remount.view().receipts[0].kind, 'unknown');
});
