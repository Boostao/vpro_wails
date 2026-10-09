const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const prefs = loadTypeScript('googleEarthPreferences.ts', {
  './qualityEditor': quality, './googleEarthReview': earth,
  './googleEarthKMLReview': { googleEarthXMLText: value => quality.wellFormedUTF16(value) && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value) },
});
const scope = { contextId: 'owned-prefs', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', su: 'None', suPath: '' };
const saved = { title: ' Literal <&🌲\r\n ', descriptionField: 'Absent' };
const review = (values = saved, owner = scope) => ({ ...owner, values: structuredClone(values) });
const outcome = (committed = true, errorMessage = '') => ({ contextId: scope.contextId, changed: committed, committed, errorMessage });
function ready() {
  const session = new prefs.GoogleEarthPreferencesSession(scope);
  session.apply(review()); session.edit({ title: 'Changed', descriptionField: 'Absent' }); return session;
}

test('saved preference review keeps unavailable invalid literals without defaults or repairs', () => {
  const historical = { title: 'old\x01title', descriptionField: 'missing\0field' };
  assert.deepEqual(prefs.validateGoogleEarthPreferences(review(historical), scope).values, historical);
  for (const value of [null, {}, review(null), review({ title: null, descriptionField: 'Zone' }),
    review({ title: 'title', descriptionField: 42 }), review({ title: '\ud800', descriptionField: 'Zone' }),
    review(saved, { ...scope, projectPath: 'foreign' })]) {
    assert.throws(() => prefs.validateGoogleEarthPreferences(value, scope));
  }
});

test('explicit load applies once and owned session retains new drafts across remount', () => {
  const owner = { ...scope, contextId: 'draft-remount' };
  const session = prefs.googleEarthPreferencesSession(owner);
  session.apply(review(saved, owner));
  session.edit({ title: 'New unsaved title', descriptionField: 'Longitude' });
  const remount = prefs.googleEarthPreferencesSession(owner);
  assert.equal(session, remount);
  assert.deepEqual(remount.view().draft, { title: 'New unsaved title', descriptionField: 'Longitude' });
  assert.deepEqual(remount.view().saved, saved);
  assert.throws(() => prefs.googleEarthPreferencesSession({ ...owner, su: 'foreign' }));
});

test('held async loads reject stale generations after edits cancellation newer load or mutation', async () => {
  const session = ready();
  let resolve;
  const token = session.beginLoad();
  const pending = new Promise(done => { resolve = done; }).then(value => session.applyLoaded(value, token));
  session.edit({ title: 'Newer draft', descriptionField: 'Absent' });
  resolve(review()); assert.equal(await pending, false); assert.equal(session.view().draft.title, 'Newer draft');
  const cancelled = session.beginLoad(); session.cancelLoad();
  assert.equal(session.applyLoaded(review(), cancelled), false);
  const older = session.beginLoad(), latest = session.beginLoad();
  assert.equal(session.applyLoaded(review({ title: 'old', descriptionField: 'Zone' }), older), false);
  assert.equal(session.applyLoaded(review({ title: 'latest', descriptionField: 'Zone' }), latest), true);
  const preSave = session.beginLoad(); session.edit({ title: 'Written', descriptionField: 'Zone' });
  await session.save(['Zone'], async () => outcome());
  assert.equal(session.applyLoaded(review(), preSave), false);
  assert.equal(session.view().draft.title, 'Written'); assert.equal(session.view().blocked, true);
});

test('save uses literal expected/proposed snapshots with noncancellable acknowledgement barrier', async () => {
  const session = ready();
  let resolve, captured, calls = 0, cancellations = 0;
  const pending = session.save([], request => {
    captured = request; calls++;
    return { then(ok, fail) { return new Promise(done => { resolve = done; }).then(ok, fail); }, cancel() { cancellations++; } };
  });
  await Promise.resolve();
  assert.deepEqual(captured.expected, saved); assert.equal(captured.proposed.title, 'Changed');
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.throws(() => session.edit(saved)); assert.throws(() => session.apply(review())); assert.throws(() => session.acknowledge());
  await assert.rejects(session.save([], async () => outcome()));
  resolve(outcome()); await pending;
  assert.equal(calls, 1); assert.equal(cancellations, 0);
  assert.equal(session.view().outcome.committed, true);
  assert.equal(session.view().blocked, true); session.acknowledge();
  assert.equal(session.view().blocked, false); assert.equal(session.view().outcome.committed, true);
});

test('changed physical fields require current membership but unchanged unavailable historical fields survive title correction', async () => {
  const session = ready();
  await session.save([], async () => outcome());
  assert.equal(session.view().outcome.committed, true); session.acknowledge();
  session.edit({ title: 'Changed', descriptionField: 'Foreign' });
  await assert.rejects(session.save(['Zone'], async () => outcome()));
  session.edit({ title: 'Changed', descriptionField: 'Longitude' });
  await session.save(['Longitude'], async () => outcome());
  assert.equal(session.view().saved.descriptionField, 'Longitude');
  const historical = new prefs.GoogleEarthPreferencesSession(scope);
  historical.apply(review({ title: 'old\x01title', descriptionField: 'Absent' }));
  historical.edit({ title: 'old\x01title', descriptionField: 'Zone' });
  await historical.save(['Zone'], async () => outcome());
  historical.acknowledge(); historical.edit({ title: 'new\x01title', descriptionField: 'Zone' });
  await assert.rejects(historical.save(['Zone'], async () => outcome()));
});

test('typed committed errors and explicit rejection retain receipt and drafts across subscriptions', async () => {
  for (const committed of [true, false]) {
    const session = ready();
    await session.save([], async () => outcome(committed, committed ? 'Committed; do not replay' : 'CAS stale; reload'));
    let mounted;
    const unsubscribe = session.subscribe(() => { mounted = session.view(); }); unsubscribe();
    session.subscribe(() => { mounted = session.view(); });
    assert.equal(mounted.outcome.committed, committed); assert.equal(mounted.blocked, true);
    assert.equal(mounted.draft.title, 'Changed'); assert.ok(mounted.error);
    session.acknowledge(); assert.equal(session.view().outcome.committed, committed);
  }
});

test('owned receipt registry preserves a held save and committed error across actual session remount lookup', async () => {
  const owner = { ...scope, contextId: 'receipt-remount' };
  const session = prefs.googleEarthPreferencesSession(owner);
  session.apply(review(saved, owner)); session.edit({ title: 'Changed', descriptionField: 'Absent' });
  let resolve;
  const pending = session.save([], () => new Promise(done => { resolve = done; }));
  const remount = prefs.googleEarthPreferencesSession(owner);
  assert.equal(remount.view().busy, true); assert.equal(remount.view().blocked, true);
  resolve({ ...outcome(true, 'Committed with cancellation; do not replay'), contextId: owner.contextId });
  await pending;
  const receipt = prefs.googleEarthPreferencesSession(owner).view();
  assert.equal(receipt.outcome.committed, true); assert.equal(receipt.draft.title, 'Changed'); assert.equal(receipt.blocked, true);
});

test('transport failures and malformed successful defaults remain unknown and require explicit reload after acknowledgement', async () => {
  for (const port of [
    async () => { throw new Error('RPC disconnected after potential commit'); },
    async () => null, async () => ({}), async () => outcome(false),
    async () => ({ ...outcome(), changed: false }),
    async () => ({ ...outcome(), contextId: 'foreign' }),
    async () => ({ ...outcome(), errorMessage: '\ud800' }),
  ]) {
    const session = ready(); await session.save([], port);
    assert.equal(session.view().outcome, null); assert.match(session.view().error, /outcome unknown/);
    assert.equal(session.view().saved, null); assert.equal(session.view().blocked, true);
    session.acknowledge(); await assert.rejects(session.save([], async () => outcome()));
    session.apply(review({ title: 'Potential committed title', descriptionField: 'Absent' }));
    assert.equal(session.view().draft.title, 'Potential committed title');
  }
});

test('no-op successful acknowledgement is explicit and cannot masquerade as committed mutation', async () => {
  const session = new prefs.GoogleEarthPreferencesSession(scope); session.apply(review());
  await session.save([], async () => outcome(false));
  assert.equal(session.view().outcome.committed, false); assert.equal(session.view().error, '');
  session.acknowledge(); await session.save([], async () => outcome(true));
  assert.equal(session.view().outcome, null); assert.match(session.view().error, /outcome unknown/);
});

test('provider request mutation cannot rewrite retained live drafts or the expected saved snapshot', async () => {
  const session = ready();
  await session.save([], async request => {
    request.expected.title = 'provider expected mutation';
    request.proposed.title = 'provider proposed mutation';
    return outcome();
  });
  assert.equal(session.view().draft.title, 'Changed'); assert.equal(session.view().saved.title, 'Changed');
  assert.equal(session.view().outcome.committed, true);
});

test('new invalid preference drafts retain errors across remount and rejected Save without locking valid correction', async () => {
  const owner = { ...scope, contextId: 'invalid-draft-remount' };
  const session = prefs.googleEarthPreferencesSession(owner);
  session.apply(review(saved, owner));
  session.edit({ title: 'new\x01title', descriptionField: 'Absent' });
  assert.match(session.view().draftError, /Place Name/);
  assert.equal(session.view().blocked, false); assert.equal(session.view().busy, false);
  let calls = 0;
  await assert.rejects(session.save([], async () => { calls++; return outcome(); }));
  assert.equal(calls, 0);
  const remount = prefs.googleEarthPreferencesSession(owner);
  assert.match(remount.view().draftError, /Place Name/); assert.equal(remount.view().draft.title, 'new\x01title');
  remount.edit({ title: 'Corrected', descriptionField: 'Absent' });
  assert.equal(session.view().draftError, '');
  for (const field of ['', '\ud800', 'Zone\0']) {
    remount.edit({ title: 'Corrected', descriptionField: field });
    assert.match(session.view().draftError, /description field/);
    await assert.rejects(session.save(['Zone'], async () => { calls++; return outcome(); }));
  }
  assert.equal(calls, 0);
  remount.edit({ title: 'Corrected', descriptionField: 'Zone' });
  assert.equal(session.view().draftError, '');
  await session.save(['Zone'], async () => { calls++; return { ...outcome(), contextId: owner.contextId }; });
  assert.equal(calls, 1); assert.equal(session.view().outcome.committed, true); assert.equal(session.view().draftError, '');
});

test('Undo and explicit Load restore owned saved literals clear draft validation and preserve irreversible receipts', async () => {
  const session = ready();
  await session.save([], async () => outcome(true, 'Committed with warning; do not replay'));
  assert.throws(() => session.undo());
  session.acknowledge();
  session.edit({ title: 'bad\0title', descriptionField: 'Absent' });
  assert.ok(session.view().draftError);
  session.undo();
  assert.deepEqual(session.view().draft, { title: 'Changed', descriptionField: 'Absent' });
  assert.equal(session.view().draftError, '');
  assert.equal(session.view().outcome.committed, true); assert.equal(session.view().error, 'Committed with warning; do not replay');
  session.edit({ title: '\ud800', descriptionField: 'Absent' });
  assert.ok(session.view().draftError);
  session.apply(review({ title: 'Fresh saved', descriptionField: 'Absent' }));
  assert.equal(session.view().draftError, ''); assert.equal(session.view().draft.title, 'Fresh saved');
  const unloaded = new prefs.GoogleEarthPreferencesSession(scope);
  assert.throws(() => unloaded.undo());
});

test('unchanged historical XML-invalid title and NUL field are not new draft errors when correcting the other key', async () => {
  const session = new prefs.GoogleEarthPreferencesSession(scope);
  const historical = { title: 'old\x01title', descriptionField: 'absent\0field' };
  session.apply(review(historical)); session.setPhysicalFields(['Zone']);
  assert.equal(session.view().draftError, '');
  session.edit({ title: 'Corrected', descriptionField: historical.descriptionField });
  assert.equal(session.view().draftError, '');
  await session.save(['Zone'], async () => outcome()); session.acknowledge();
  session.apply(review(historical));
  session.edit({ title: historical.title, descriptionField: 'Zone' });
  assert.equal(session.view().draftError, '');
  await session.save(['Zone'], async () => outcome()); session.acknowledge();
  session.edit({ title: 'new\x01title', descriptionField: 'Zone' });
  assert.ok(session.view().draftError); session.undo();
  assert.deepEqual(session.view().draft, { title: historical.title, descriptionField: 'Zone' });
  assert.equal(session.view().draftError, '');
});

test('panel independently gates preferences shares one title and field and protects stale async loads', () => {
  const source = readFileSync(path.join(__dirname, 'GoogleEarthReview.svelte'), 'utf8');
  const compiled = compile(source, { filename: 'GoogleEarthReview.svelte', generate: 'client', dev: true });
  assert.equal(compiled.warnings.filter(w => w.code.startsWith('a11y')).length, 0);
  assert.match(source, /const preferencesEnabled = import\.meta\.env\.VITE_GOOGLE_EARTH_PREFERENCES === 'true'/);
  assert.match(source, /\{#if preferencesEnabled && preferences\}/);
  for (const action of ['load', 'save', 'outcome', 'acknowledge']) {
    assert.equal((source.match(new RegExp(`data-google-earth-preferences-${action}\\b`, 'g')) || []).length, 1);
  }
  assert.equal((source.match(/id="google-earth-place-name"/g) || []).length, 1);
  assert.equal((source.match(/id="google-earth-description"/g) || []).length, 1);
  assert.match(source, /<option value=\{selectedField\}>[\s\S]*saved literal; physical availability not established/);
  assert.match(source, /fields = null; if \(!preferencesEnabled\) selectedField = ''/);
  assert.match(source, /\{#if kmlEnabled \|\| kmlExportEnabled \|\| preferencesEnabled\}/);
  assert.match(source, /\{#if kmlEnabled \|\| kmlExportEnabled\}[\s\S]*data-google-earth-kml-prepare/);
  const load = source.slice(source.indexOf('async function loadPreferences'), source.indexOf('async function savePreferences'));
  assert.match(load, /reads\.track/); assert.match(load, /if \(request !== generation\) return;[\s\S]*preferenceSession\.applyLoaded/);
  assert.match(load, /review = null; kml = null; approval = null/);
  const save = source.slice(source.indexOf('async function savePreferences'), source.indexOf('async function loadFields'));
  assert.doesNotMatch(save, /reads\.track|\.cancel/);
  assert.match(source, /preferences\.busy \|\| preferences\.blocked/);
  const busy = source.slice(source.indexOf('function updateBusy'), source.indexOf('const unsubscribe ='));
  assert.match(busy, /preferences\.draftError/);
  const locks = source.slice(source.indexOf('const preferenceLocked'), source.indexOf('function updateBusy'));
  assert.doesNotMatch(locks, /draftError/);
  assert.match(source, /data-google-earth-preferences-save disabled=\{locked \|\| !preferences\.saved \|\| !!preferences\.draftError\}/);
  assert.match(source, /data-google-earth-preferences-undo disabled=\{locked \|\| !preferences\.saved\}/);
  assert.match(source, /title = \(event\.currentTarget as HTMLTextAreaElement\)\.value;[\s\S]*rememberDraft/);
  assert.match(source, /selectedField = \(event\.currentTarget as HTMLSelectElement\)\.value;[\s\S]*rememberDraft/);
  assert.ok(source.indexOf('data-google-earth-preferences-draft-error') < source.indexOf('id="google-earth-place-name"'));
  const undo = source.slice(source.indexOf('function undoPreferences'), source.indexOf('async function loadPreferences'));
  assert.match(undo, /preferenceSession\.undo/); assert.match(undo, /review = null; kml = null; approval = null/);
  assert.doesNotMatch(undo, /Service\./);
  assert.match(source, /Text preparation itself creates no file, saves no preference and launches no viewer/);
  assert.match(source, /onDestroy\(\(\) => unsubscribePreferences\?\.\(\)\)/);
  assert.doesNotMatch(source, /\$effect[\s\S]*loadPreferences/);
  const generated = readFileSync(path.join(__dirname, '..', 'bindings', 'github.com', 'boostao', 'vpro-wails', 'googleearthpreferencesservice.ts'), 'utf8');
  assert.match(generated, /export function SaveGoogleEarthPreferences/);
  assert.match(generated, /GoogleEarthPreferencesOutcome/);
});
