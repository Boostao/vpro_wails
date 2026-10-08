const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { render } = require('svelte/server');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const prefs = loadTypeScript('longEnvironmentPreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const scope = { contextId: 'owned-title', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', su: 'None', suPath: '' };
const review = (title = 'Long Evironment Report', owner = scope) => ({ ...owner, values: { title } });
const outcome = (committed = true, errorMessage = '', owner = scope) => ({ contextId: owner.contextId, changed: committed, committed, errorMessage });
function ready() {
  const session = new prefs.LongEnvironmentPreferencesSession(scope);
  session.apply(review()); session.edit('Changed'); return session;
}
test('owned review is exact, preserves historical NUL and empty title, never defaults malformed data', () => {
  for (const title of ['', ' Literal <&\x01🌲\r\n ', 'old\0title']) {
    assert.equal(prefs.validateLongEnvironmentPreferences(review(title), scope).values.title, title);
  }
  for (const value of [null, {}, { ...scope, values: null }, review(null), review(42), review('\ud800'),
    ...['contextId','project','projectPath','su','suPath'].map(key => review('Title', { ...scope, [key]: 'foreign' }))]) {
    assert.throws(() => prefs.validateLongEnvironmentPreferences(value, scope));
  }
});
test('uninitialized or failed Load has no inferred title or Undo authority; deliberate drafts initialize preview only', async () => {
  const session = new prefs.LongEnvironmentPreferencesSession(scope);
  let mounted;
  const unsubscribe = session.subscribe(() => { mounted = session.view(); });
  assert.equal(mounted.initialized, false); assert.equal(mounted.draft.title, ''); assert.equal(mounted.saved, null);
  assert.throws(() => session.undo(), /known owned saved title/);
  for (const value of [null, {}, { ...scope, values: null }, review(null), review(42), review('\ud800')]) {
    const token = session.beginLoad();
    assert.throws(() => session.applyLoaded(value, token));
    assert.equal(mounted.initialized, false); assert.equal(mounted.draft.title, ''); assert.equal(mounted.saved, null);
    assert.throws(() => session.undo());
    await assert.rejects(session.save(async () => outcome()), /Load saved title/);
  }
  session.edit('Intentional preview');
  assert.equal(mounted.initialized, true); assert.equal(mounted.draft.title, 'Intentional preview');
  assert.equal(mounted.saved, null); assert.throws(() => session.undo());
  await assert.rejects(session.save(async () => outcome()));
  session.edit(''); assert.equal(mounted.initialized, true); assert.equal(mounted.draft.title, '');
  session.apply(review('owned original')); session.edit('different'); session.undo();
  assert.equal(mounted.draft.title, 'owned original'); assert.equal(mounted.initialized, true);
  unsubscribe();
  const owner = { ...scope, contextId: 'uninitialized-remount' };
  const persistent = prefs.longEnvironmentPreferencesSession(owner);
  assert.equal(prefs.longEnvironmentPreferencesSession(owner).view().initialized, false);
  persistent.edit('deliberate');
  assert.equal(prefs.longEnvironmentPreferencesSession(owner).view().initialized, true);
  assert.equal(prefs.longEnvironmentPreferencesSession(owner).view().saved, null);
});
test('draft validation clears on literal correction or Undo and invalid originals remain unchanged', async () => {
  const session = ready();
  for (const title of ['new\0title', '\ud800']) {
    session.edit(title); assert.ok(session.view().draftError);
    await assert.rejects(session.save(async () => outcome()));
    session.edit(''); assert.equal(session.view().draftError, '');
  }
  session.edit('bad\0title'); session.undo();
  assert.equal(session.view().draft.title, 'Long Evironment Report'); assert.equal(session.view().draftError, '');
  session.apply(review('old\0title')); session.edit('old\0title');
  await session.save(async () => outcome(false)); session.acknowledge();
  assert.equal(session.view().saved.title, 'old\0title');
  session.edit('corrected'); await session.save(async () => outcome());
  assert.equal(session.view().saved.title, 'corrected');
});
test('cancelled older reads cannot overwrite drafts, Undo, newer Load or Save', async () => {
  const session = ready();
  const token = session.beginLoad(); session.edit('Newer');
  assert.equal(session.applyLoaded(review(), token), false);
  const cancelled = session.beginLoad(); session.cancelLoad();
  assert.equal(session.applyLoaded(review(), cancelled), false);
  const older = session.beginLoad(), latest = session.beginLoad();
  assert.equal(session.applyLoaded(review('old'), older), false);
  assert.equal(session.applyLoaded(review('latest'), latest), true);
  const undo = session.beginLoad(); session.undo();
  assert.equal(session.applyLoaded(review('old'), undo), false);
  const preSave = session.beginLoad(); session.edit('Written');
  await session.save(async () => outcome());
  assert.equal(session.applyLoaded(review(), preSave), false);
});
test('per-context registry retains invalid drafts and irreversible acknowledged receipts across remount', async () => {
  const owner = { ...scope, contextId: 'remount-title' };
  const session = prefs.longEnvironmentPreferencesSession(owner);
  session.apply(review('original', owner)); session.edit('bad\0title');
  assert.equal(prefs.longEnvironmentPreferencesSession(owner).view().draftError, session.view().draftError);
  session.edit('Written');
  let resolve, captured, cancellations = 0;
  const pending = session.save(request => {
    captured = request;
    return { then(ok, fail) { return new Promise(done => { resolve = done; }).then(ok, fail); }, cancel() { cancellations++; } };
  });
  await Promise.resolve();
  assert.equal(captured.expected.title, 'original'); assert.equal(captured.proposed.title, 'Written');
  const remount = prefs.longEnvironmentPreferencesSession(owner);
  assert.equal(remount.view().busy, true); assert.equal(remount.view().blocked, true);
  assert.throws(() => remount.acknowledge()); assert.throws(() => remount.edit('other'));
  assert.throws(() => remount.undo()); assert.throws(() => remount.beginLoad());
  resolve(outcome(true, 'Committed; do not replay', owner)); await pending;
  assert.equal(cancellations, 0); assert.equal(remount.view().blocked, true);
  remount.acknowledge(); remount.edit('Written again');
  await remount.save(async () => outcome(true, '', owner)); remount.acknowledge();
  assert.equal(prefs.longEnvironmentPreferencesSession(owner).view().receipts.length, 2);
  assert.equal(remount.view().receipts[0].kind, 'known');
  assert.equal(remount.view().receipts[0].outcome.committed, true);
  assert.equal(remount.view().receipts[0].request.expected.title, 'original');
  assert.throws(() => prefs.longEnvironmentPreferencesSession({ ...owner, suPath: 'other' }));
});
test('typed failures retain saved authority and drafts; unknown or contradictory receipts require explicit Load', async () => {
  const rejected = ready();
  await rejected.save(async () => outcome(false, 'CAS stale; reload'));
  assert.equal(rejected.view().saved.title, 'Long Evironment Report');
  assert.equal(rejected.view().draft.title, 'Changed'); assert.equal(rejected.view().blocked, true);
  rejected.acknowledge(); assert.equal(rejected.view().outcome.committed, false);
  for (const result of [null, {}, outcome(true, '', { ...scope, contextId: 'foreign' }),
    { ...outcome(), changed: false }, { ...outcome(), errorMessage: '\ud800' },
    { ...outcome(), errorMessage: 'bad\0receipt' }, outcome(false)]) {
    const session = ready();
    await session.save(async () => result);
    assert.equal(session.view().saved, null); assert.equal(session.view().authorityUnknown, true);
    assert.equal(session.view().blocked, true); session.acknowledge();
    await assert.rejects(session.save(async () => outcome()));
    assert.throws(() => session.undo()); assert.equal(session.view().authorityUnknown, true);
    session.apply(review('actual')); assert.equal(session.view().authorityUnknown, false);
  }
  const transport = ready();
  await transport.save(async () => { throw new Error('transport lost'); });
  assert.match(transport.view().error, /unknown/); assert.equal(transport.view().saved, null);
  const noOp = ready(); noOp.undo();
  await noOp.save(async () => outcome(true, 'contradictory no-op commit'));
  assert.equal(noOp.view().authorityUnknown, true);
});
test('tagged unknown receipt survives acknowledgement, explicit reload, later save and remount without invented commit bits', async () => {
  const owner = { ...scope, contextId: 'unknown-remount' };
  const session = prefs.longEnvironmentPreferencesSession(owner);
  session.apply(review('original', owner)); session.edit('attempted title');
  await session.save(async () => { throw new Error('response lost after request'); });
  const evidence = session.view().receipts[0];
  assert.equal(evidence.kind, 'unknown'); assert.equal(evidence.contextId, owner.contextId);
  assert.deepEqual(evidence.request, { expected: { title: 'original' }, proposed: { title: 'attempted title' } });
  assert.match(evidence.errorMessage, /response lost after request/);
  assert.equal(Object.hasOwn(evidence, 'changed'), false); assert.equal(Object.hasOwn(evidence, 'committed'), false);
  assert.equal(Object.hasOwn(evidence, 'outcome'), false);
  const remount = prefs.longEnvironmentPreferencesSession(owner);
  remount.acknowledge(); assert.deepEqual(remount.view().receipts[0], evidence);
  remount.applyLoaded(review('observed saved title', owner), remount.beginLoad());
  assert.equal(remount.view().authorityUnknown, false); assert.equal(remount.view().error, '');
  assert.deepEqual(remount.view().receipts[0], evidence);
  remount.edit('subsequent title');
  await remount.save(async () => outcome(true, '', owner)); remount.acknowledge();
  const after = prefs.longEnvironmentPreferencesSession(owner).view();
  assert.deepEqual(after.receipts[0], evidence);
  assert.equal(after.receipts[1].kind, 'known'); assert.equal(after.receipts[1].outcome.committed, true);
});
test('saved CRLF no-op preserves raw literals; deliberate multiline LF drafts survive remount and Save', async () => {
  const owner = { ...scope, contextId: 'multiline-remount' };
  const session = prefs.longEnvironmentPreferencesSession(owner);
  const saved = ' First line\r\nSecond line\r\n ';
  session.apply(review(saved, owner));
  let captured;
  await session.save(async request => { captured = request; return outcome(false, '', owner); });
  assert.equal(captured.expected.title, saved); assert.equal(captured.proposed.title, saved);
  session.acknowledge();
  const edited = ' First line\nChanged second line\n ';
  session.edit(edited);
  const remount = prefs.longEnvironmentPreferencesSession(owner);
  assert.equal(remount.view().draft.title, edited); assert.equal(remount.view().saved.title, saved);
  assert.equal(remount.view().draftError, '');
  await remount.save(async request => { captured = request; return outcome(true, '', owner); });
  assert.equal(captured.expected.title, saved); assert.equal(captured.proposed.title, edited);
  assert.equal(remount.view().saved.title, edited);
});
test('rendered title has one labelled conditional control: enabled two-row textarea, default-off original input', () => {
  const source = readFileSync(path.join(__dirname, 'LongEnvironmentReport.svelte'), 'utf8');
  const session = new prefs.LongEnvironmentPreferencesSession(scope);
  const saved = 'First line\r\nSecond line';
  session.apply(review(saved));
  for (const enabled of [true, false]) {
    const component = serverComponent(source.replace(
      "import.meta.env.VITE_LONG_ENVIRONMENT_PREFERENCES === 'true'", String(enabled))
      .replace("import.meta.env.VITE_LONG_ENVIRONMENT_WORKBOOK === 'true'", 'false'),
    'LongEnvironmentReport.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { ContextService: {}, LongEnvironmentPreferencesService: {} },
      './longEnvironmentPreferences': { longEnvironmentPreferencesSession: () => session },
      './longEnvironmentReport': { reportTitleError: () => null },
      './environmentWorkbook': { environmentWorkbookPublicationSession: () => ({
        view: () => ({ busy: false, blocked: false, outcome: null, error: '', requestedDestination: '' }),
        subscribe: () => () => {},
      }) },
    });
    const { body } = render(component, { props: { ...scope, onBusyChange: () => {} } });
    const controls = body.match(/<(?:input|textarea)\b[^>]*id="long-environment-title"[^>]*>/g) ?? [];
    assert.equal(controls.length, 1);
    assert.match(body, /<label[^>]*for="long-environment-title"[^>]*>Title<\/label>/);
    if (enabled) {
      assert.match(controls[0], /^<textarea/); assert.match(controls[0], /rows="2"/);
      assert.match(body, /First line\r\nSecond line<\/textarea>/);
      assert.doesNotMatch(controls[0], /maxlength/);
      assert.equal(session.view().draft.title, saved);
    } else {
      assert.match(controls[0], /^<input/); assert.match(controls[0], /disabled/);
      assert.doesNotMatch(body, /<textarea/);
    }
  }
  assert.match(source, /\{#if preferencesEnabled\}\s*<textarea[\s\S]*?\{:else\}\s*<input[^>]*disabled=\{busy \|\| !ready \|\| workbookBarrier\}/);
  assert.equal((source.match(/const value = event.currentTarget.value; editTitle\(value\)/g) ?? []).length, 2);
  assert.match(source, /input, textarea \{[^}]*width: 100%/);
});
test('default-off panel has one live title and explicit cancellable Load/noncancellable Save barriers', () => {
  const panel = readFileSync(path.join(__dirname, 'LongEnvironmentReport.svelte'), 'utf8');
  assert.equal(compile(panel, { filename: 'LongEnvironmentReport.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(panel, /VITE_LONG_ENVIRONMENT_PREFERENCES === 'true'/);
  assert.equal((panel.match(/id="long-environment-title"/g) ?? []).length, 2);
  assert.match(panel, /if \(!preferencesEnabled\) void options\(\)/);
  assert.match(panel, /if \(!preferencesEnabled\) title = value.title/);
  assert.match(panel, /ready = preferenceView.initialized/);
  assert.match(panel, /data-long-environment-preferences-undo disabled=\{[^}]+!preferenceView\?\.saved/);
  assert.match(panel, /receipt.kind === 'known'/);
  assert.match(panel, /Preference receipt: outcome unknown; changed unknown; committed unknown/);
  assert.doesNotMatch(readFileSync(path.join(__dirname, 'longEnvironmentPreferences.ts'), 'utf8'), /Long Evironment Report/);
  assert.match(panel, /const value = event.currentTarget.value; editTitle\(value\)/);
  assert.match(panel, /preferences\.save\(request => LongEnvironmentPreferencesService\.SaveLongEnvironmentPreferences/);
  assert.doesNotMatch(panel, /reads\.track\(LongEnvironmentPreferencesService\.Save/);
  assert.match(panel, /preferenceView\.authorityUnknown \|\| !!preferenceView\.draftError/);
  assert.match(panel, /This report preparation made no data\/configuration writes/);
  for (const selector of ['load','save','undo','acknowledge','receipt']) assert.match(panel, new RegExp(`data-long-environment-preferences-${selector}`));
});
