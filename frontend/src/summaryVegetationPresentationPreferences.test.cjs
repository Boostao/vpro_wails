const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const frontend = path.dirname(__dirname);
const req = createRequire(path.join(frontend, 'package.json'));
const { proxy } = req('svelte/internal/client');
const { loadTypeScript, serverComponent, componentFunctions } = req('./src/svelteTestHelpers.cjs');
const { render } = req('svelte/server');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', { './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations });
const shared = loadTypeScript('reportTitlePreferences.ts', { './qualityEditor': quality, './googleEarthReview': earth });
const lifeform = loadTypeScript('lifeformSummary.ts');
const file = path.relative(path.join(frontend, 'src'), path.join(__dirname, 'summaryVegetationPresentationPreferences.ts'));
const prefs = loadTypeScript(file, { './googleEarthReview': earth, './qualityEditor': quality,
  './lifeformSummary': lifeform, './reportTitlePreferences': shared });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationPresentationPreferencesWire.json'), 'utf8'));
const { values: ignoredValues, source: ignoredSource, ...scope } = wire.review;
const clone = value => structuredClone(value);
const reviewed = values => ({ ...clone(wire.review), values });
const ready = (values = wire.review.values) => {
  const session = new prefs.SummaryVegetationPresentationPreferencesSession(scope);
  session.apply(reviewed(values));
  return session;
};
const outcome = (committed, errorMessage = '') => ({ contextId: scope.contextId, changed: committed, committed, errorMessage });

test('actual Go review detaches real Svelte proxies; Save and retained receipt carry the exact strict plan', async () => {
  const input = proxy(clone(wire.review));
  assert.throws(() => structuredClone(input));
  const detached = prefs.validateSummaryVegetationPresentationPreferences(input, scope);
  assert.deepEqual(clone(detached), wire.review);
  input.values.title = 'other';
  assert.equal(detached.values.title, wire.review.values.title);
  const session = ready();
  session.editTitle(wire.request.proposed.title);
  session.editFlag('constancy', true);
  await session.save(request => {
    assert.deepEqual(clone(request), wire.request);
    return Promise.resolve(wire.outcome);
  });
  const receipt = session.view().receipts[0];
  assert.deepEqual(clone(receipt.request), wire.request);
  assert.deepEqual(receipt.outcome, wire.outcome);
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  assert.equal(session.view().receipts[0].request.constancyApplied, true);
  receipt.request.proposed.title = 'untrusted copy';
  assert.equal(session.view().receipts[0].request.proposed.title, wire.request.proposed.title);
});

test('all 128 original Boolean combinations retain historical inconsistent storage as an owned no-op', async () => {
  const keys = ['showCommon', 'showGuide', 'hierarchyHeader', 'includeEnv', 'createSummary', 'combineVariants', 'constancy'];
  for (let mask = 0; mask < 128; mask++) {
    const values = { ...wire.review.values, title: ' historical\0title ', groupBy: 4, display: 6, valueLimit: 7.25 };
    keys.forEach((key, index) => values[key] = !!(mask & (1 << index)));
    const session = ready(values);
    await session.save(request => {
      assert.deepEqual(clone(request), { expected: values, proposed: values, constancyApplied: false });
      return Promise.resolve(outcome(false));
    });
    session.acknowledge();
    session.editFlag('showGuide', !values.showGuide);
    await session.save(request => {
      assert.deepEqual(request.expected, values);
      assert.equal(request.proposed.showCommon, values.showCommon);
      assert.equal(request.proposed.groupBy, 4);
      assert.equal(request.proposed.display, 6);
      assert.equal(request.proposed.valueLimit, 7.25);
      assert.equal(request.constancyApplied, false);
      return Promise.resolve(outcome(true));
    });
  }
});

test('constancy-on forces source fields; off does not restore and allows a subsequent common-name edit', async () => {
  const values = { ...wire.review.values, groupBy: 4, display: 6, showCommon: true, valueLimit: 7.25 };
  const session = ready(values);
  session.editFlag('constancy', true);
  assert.equal(session.view().draft.groupBy, 3);
  assert.equal(session.view().draft.display, 1);
  assert.equal(session.view().draft.showCommon, false);
  assert.equal(session.view().draft.valueLimit, 7.25);
  assert.equal(session.view().draft.constancyApplied, true);
  assert.throws(() => session.editFlag('showCommon', true), /disabled/);
  session.editFlag('constancy', false);
  session.editFlag('showCommon', true);
  await session.save(request => {
    assert.deepEqual(clone(request), { expected: values, proposed: { ...values, groupBy: 3, display: 1 }, constancyApplied: true });
    return Promise.resolve(outcome(true));
  });
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  assert.equal(session.view().receipts[0].request.constancyApplied, true);
  session.editTitle(' next');
  await session.save(request => {
    assert.equal(request.constancyApplied, false);
    assert.equal(request.proposed.groupBy, 3);
    return Promise.resolve(outcome(true));
  });
  assert.deepEqual(clone(session.view().receipts.map(receipt => receipt.request.constancyApplied)), [true, false]);
});

test('Undo clears staged callback provenance but keeps irreversible source-plan receipts', async () => {
  const session = ready({ ...wire.review.values, groupBy: 4, display: 6, showCommon: true });
  const original = session.view().saved;
  session.editFlag('constancy', true);
  session.editFlag('constancy', false);
  session.undo();
  assert.deepEqual(session.view().draft, { ...original, constancyApplied: false });
  session.editFlag('constancy', true);
  await session.save(() => Promise.resolve(outcome(true)));
  session.acknowledge();
  session.undo();
  assert.equal(session.view().draft.groupBy, 3);
  assert.equal(session.view().draft.constancy, true);
  assert.equal(session.view().draft.constancyApplied, false);
  assert.equal(session.view().receipts.length, 1);
  assert.equal(session.view().receipts[0].request.constancyApplied, true);
});

test('repeated acknowledgement cannot erase a newly staged constancy callback plan', async () => {
  const session = ready();
  session.editTitle('first successful title');
  await session.save(() => Promise.resolve(outcome(true)));
  session.acknowledge();
  session.editFlag('constancy', true);
  assert.equal(session.view().draft.constancyApplied, true);
  session.acknowledge();
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, true);
  assert.equal(session.view().draftError, '');
  await session.save(request => {
    assert.equal(request.constancyApplied, true);
    assert.equal(request.proposed.groupBy, 3);
    return Promise.resolve(outcome(true));
  });
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  assert.deepEqual(clone(session.view().receipts.map(receipt => receipt.request.constancyApplied)), [false, true]);
});

test('regression independently reproduces the reviewed stale-ack bug with the exact previous guard', async () => {
  const fs = require('node:fs'), vm = require('node:vm');
  const ts = req('typescript');
  const source = fs.readFileSync(path.join(__dirname, 'summaryVegetationPresentationPreferences.ts'), 'utf8');
  const before = 'if (view.blocked && view.outcome &&';
  assert.equal(source.split(before).length, 2);
  const previous = source.replace(before, 'if (view.outcome &&');
  const module = { exports: {} };
  const dependencies = { './googleEarthReview': earth, './qualityEditor': quality,
    './lifeformSummary': lifeform, './reportTitlePreferences': shared };
  vm.runInNewContext(ts.transpileModule(previous, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, { module, exports: module.exports, structuredClone,
    require(name) { if (!(name in dependencies)) throw new Error('Unexpected regression dependency ' + name); return dependencies[name]; } });
  const session = new module.exports.SummaryVegetationPresentationPreferencesSession(scope);
  session.apply(wire.review);
  session.editTitle('first successful title');
  await session.save(() => Promise.resolve(outcome(true)));
  session.acknowledge();
  session.editFlag('constancy', true);
  assert.equal(session.view().draft.constancyApplied, true);
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  assert.match(session.view().draftError, /coupled callback plan/);
  await assert.rejects(() => session.save(() => { throw new Error('invalid old plan must not dispatch'); }), /coupled callback plan/);
});

test('title literals have no invented bounds; new NUL/malformed UTF16 survives remount and blocks Save until correction/Undo', async () => {
  for (const title of ['', '  literal  ', "O'Brien", '日本語😀', 'x'.repeat(10000), 'historical\0unchanged']) {
    const session = ready({ ...wire.review.values, title });
    session.editFlag('includeEnv', true);
    await session.save(request => {
      assert.equal(request.proposed.title, title);
      return Promise.resolve(outcome(true));
    });
    session.acknowledge();
    for (const invalid of ['new\0title', '\ud800']) {
      session.editTitle(invalid);
      assert.ok(session.view().draftError);
      session.apply(reviewed(session.view().saved), true);
      assert.ok(session.view().draftError);
      await assert.rejects(() => session.save(() => { throw new Error('invalid title dispatched'); }));
      session.undo();
      assert.equal(session.view().draftError, '');
    }
    session.editTitle('corrected');
    assert.equal(session.view().draftError, '');
  }
  const session = ready();
  for (const key of ['groupBy', 'display', 'valueLimit', 'other']) assert.throws(() => session.editFlag(key, true));
  assert.throws(() => session.editFlag('showGuide', 'true'));
  session.editDraft({ ...session.view().draft, valueLimit: 4 });
  assert.ok(session.view().draftError);
  await assert.rejects(() => session.save(() => { throw new Error('limit edit dispatched'); }));
  session.undo();
  session.editDraft({ ...session.view().draft, groupBy: 4 });
  assert.ok(session.view().draftError);
});

test('held successful/refused/unknown Save protects live Undo and retains exact provenance without replay', async () => {
  const session = ready();
  session.editFlag('constancy', true);
  let release, calls = 0;
  const saving = session.save(request => { calls++; assert.equal(request.constancyApplied, true); return new Promise(resolve => release = resolve); });
  assert.equal(session.view().busy, true);
  assert.equal(session.view().blocked, true);
  assert.throws(() => session.editTitle('later'));
  assert.throws(() => session.undo());
  assert.throws(() => session.acknowledge());
  await assert.rejects(() => session.save(() => { calls++; throw new Error('second Save dispatched'); }));
  release(outcome(true, 'committed warning; do not replay'));
  await saving;
  assert.equal(calls, 1);
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().receipts[0].request.constancyApplied, true);
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  session.editFlag('constancy', false);
  session.editFlag('constancy', true);
  await session.save(() => Promise.resolve(outcome(false, 'replacement denied')));
  assert.equal(session.view().outcome.committed, false);
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, true);
  session.editTitle('unknown committed title');
  await session.save(() => Promise.resolve({ ...outcome(true), contextId: 'foreign' }));
  assert.equal(session.view().authorityUnknown, true);
  assert.equal(session.view().saved, null);
  const unknown = session.view().receipts[2];
  assert.equal(unknown.kind, 'unknown');
  assert.equal(unknown.request.constancyApplied, true);
  session.acknowledge();
  await assert.rejects(() => session.save(() => { throw new Error('unknown replay'); }));
  assert.throws(() => session.undo());
  session.apply(reviewed(unknown.request.proposed));
  assert.equal(session.view().authorityUnknown, false);
  assert.equal(session.view().draft.constancyApplied, false);
  assert.equal(session.view().receipts.length, 3);
});

test('failed read and late generations survive correction/Undo/remount; successful Load alone recovers Save', async () => {
  const session = ready();
  const generation = session.beginLoad();
  session.cancelLoad();
  assert.equal(session.applyLoaded(wire.review, generation), false);
  session.rejectRead(new Error('read denied'));
  session.editTitle('valid correction');
  session.undo();
  assert.match(session.view().readError, /read denied/);
  await assert.rejects(() => session.save(() => { throw new Error('failed read bypassed'); }));
  const malformed = clone(wire.review);
  malformed.values.title = '\ud800';
  assert.throws(() => session.apply(malformed));
  assert.match(session.view().readError, /read denied/);
  const held = session.beginLoad();
  session.editFlag('constancy', true);
  assert.equal(session.applyLoaded(wire.review, held), false);
  const current = session.beginLoad();
  assert.equal(session.applyLoaded(wire.review, current, true), true);
  assert.equal(session.view().readError, '');
  assert.equal(session.view().draft.constancyApplied, true);
  await session.save(request => {
    assert.equal(request.constancyApplied, true);
    return Promise.resolve(outcome(true));
  });
});

test('strict owned source and registry isolation reject malformed shapes and retain per-context plans', () => {
  for (const change of [
    review => review.values.extra = true,
    review => delete review.values.createSummary,
    review => review.values.showGuide = 1,
    review => review.values.groupBy = 0,
    review => review.values.display = 7,
    review => review.values.valueLimit = Infinity,
    review => review.source = 'other',
    review => review.projectPath += 'changed',
    review => review.extra = 'field',
  ]) {
    const input = clone(wire.review);
    change(input);
    assert.throws(() => prefs.validateSummaryVegetationPresentationPreferences(input, scope));
  }
  const registry = prefs.createSummaryVegetationPresentationPreferencesRegistry();
  const a = registry(scope);
  a.apply(wire.review);
  a.editFlag('constancy', true);
  a.editTitle('invalid\0draft');
  const other = { ...scope, contextId: 'other' };
  const b = registry(other);
  b.apply({ ...clone(wire.review), ...other });
  assert.notEqual(a, b);
  assert.equal(registry(scope), a);
  assert.ok(a.view().draftError);
  assert.equal(a.view().draft.constancyApplied, true);
  assert.equal(b.view().draft.constancyApplied, false);
});

const panelFile = path.relative(path.join(frontend, 'src'), path.join(__dirname, 'SummaryVegetationPresentationPreferences.svelte'));
const panelSource = readFileSync(path.join(__dirname, 'SummaryVegetationPresentationPreferences.svelte'), 'utf8');
test('actual compiled pane preserves unique associated original labels and unavailable hierarchy scope', () => {
  const Panel = serverComponent(panelSource, panelFile, {});
  const session = ready({ ...wire.review.values, valueLimit: 7.25 });
  session.editFlag('constancy', true);
  const html = render(Panel, { props: { contextId: scope.contextId, preferences: session,
    port: { read() { throw new Error('SSR must not Load'); }, save() { throw new Error('SSR must not Save'); } },
    onReadBusyChange() {},
  } }).body;
  for (const [key, source, label] of [
    ['title', 'rptSvTitle', 'Report Title'], ['showCommon', 'rptSvShowCommon', 'Show english name'],
    ['showGuide', 'optUseShortFieldGuideNames', 'Use short field guide names'],
    ['includeEnv', 'optIncludeEnvFields', 'Include Environmental Fields?'],
    ['createSummary', 'optCreateSummary', 'Create report summary'],
    ['combineVariants', 'optCombineVariants', 'Treat subspecies/varieties as species?'],
    ['constancy', 'optConstancyFormat', 'Use Constancy Format'],
    ['hierarchyHeader', 'optIncludeHierarchyHeader', 'Include hierarchy header'],
  ]) {
    assert.equal(html.split(`data-source-control="${source}"`).length - 1, 1);
    assert.equal(html.split(`id="summary-presentation-${key}"`).length - 1, 1);
    assert.ok(html.includes(`for="summary-presentation-${key}"`));
    assert.ok(html.includes(label));
  }
  assert.match(html, /<input[^>]*id="summary-presentation-showCommon"[^>]*disabled/);
  assert.match(html, /<input[^>]*id="summary-presentation-hierarchyHeader"[^>]*disabled/);
  assert.match(html, /7\.25/);
  assert.match(html, /Configuration-only drafts, not report-output formatting parity/);
  assert.match(panelSource, /const raw = event\.currentTarget\.value; editTitle\(raw\)/);
  assert.match(panelSource, /const raw = event\.currentTarget\.checked; editFlag\(field\.key, raw\)/);
  assert.match(panelSource, /\$state\.raw/);
  assert.match(panelSource, /minmax\(0, 1fr\)/);
  const unloaded = new prefs.SummaryVegetationPresentationPreferencesSession(scope);
  const blank = render(Panel, { props: { contextId: scope.contextId, preferences: unloaded,
    port: { read() { throw new Error('SSR must not Load'); }, save() { throw new Error('SSR must not Save'); } },
    onReadBusyChange() {},
  } }).body;
  assert.match(blank, /<input[^>]*id="summary-presentation-title"[^>]*disabled/);
  assert.match(blank, /<button[^>]*data-summary-presentation-save[^>]*disabled/);
});

test('actual pane handlers cancel held reads, retain failed-read guards and keep correction/Undo live', async () => {
  const session = ready();
  let release, dispatched = 0;
  const held = new Promise(resolve => release = resolve);
  const context = componentFunctions(panelFile, ['cancel', 'load', 'save', 'editTitle', 'editFlag'], {
    contextId: scope.contextId, preferences: session, busy: false, view: session.view(), operationHeld: false,
    mutationHeld: false, generation: 0, error: '', onReadBusyChange() {},
    reads: { cancelAll() {}, track(value) { return value; } },
    port: { read() { return held; }, save() { dispatched++; return Promise.resolve(wire.outcome); } },
  });
  const unsubscribe = session.subscribe(() => { context.view = session.view(); });
  const loading = context.actions.load();
  context.actions.cancel();
  release(wire.review);
  await loading;
  assert.equal(context.busy, false);
  session.rejectRead('owned reload failure');
  context.actions.editTitle('valid correction');
  await context.actions.save();
  assert.equal(dispatched, 0);
  assert.match(session.view().readError, /owned reload failure/);
  context.actions.cancel(); session.undo();
  assert.match(session.view().readError, /owned reload failure/);
  context.port.read = () => Promise.resolve(wire.review);
  await context.actions.load();
  assert.equal(session.view().readError, '');
  context.actions.editTitle('invalid\0draft');
  assert.ok(session.view().draftError);
  context.operationHeld = true;
  context.actions.editTitle('corrected despite root invalid-draft hold');
  assert.equal(session.view().draftError, '');
  await context.actions.save();
  assert.equal(dispatched, 0);
  context.operationHeld = false;
  context.mutationHeld = true;
  context.actions.editTitle('must not replace draft while peer saves');
  assert.equal(session.view().draft.title, 'corrected despite root invalid-draft hold');
  context.mutationHeld = false;
  unsubscribe();
});

test('actual pane Save sends exact callback wire once and surfaces disabled-common errors without enabling hierarchy', async () => {
  const session = ready();
  const requests = [];
  let release;
  const context = componentFunctions(panelFile, ['cancel', 'load', 'save', 'editTitle', 'editFlag'], {
    contextId: scope.contextId, preferences: session, busy: false, view: session.view(), operationHeld: false,
    mutationHeld: false, generation: 0, error: '', onReadBusyChange() {},
    reads: { cancelAll() {}, track(value) { return value; } },
    port: { read() { return Promise.resolve(wire.review); }, save(id, raw) {
      assert.equal(id, scope.contextId);
      requests.push(JSON.parse(raw)); return new Promise(resolve => release = resolve);
    } },
  });
  const unsubscribe = session.subscribe(() => { context.view = session.view(); });
  context.actions.editTitle(wire.request.proposed.title);
  context.actions.editFlag('constancy', true);
  context.actions.editFlag('showCommon', true);
  assert.match(context.error, /disabled while constancy/);
  assert.equal(session.view().draft.showCommon, false);
  const saving = context.actions.save();
  assert.equal(session.view().busy, true);
  await context.actions.save();
  assert.deepEqual(requests, [wire.request]);
  release(wire.outcome);
  await saving;
  assert.equal(session.view().blocked, true);
  assert.equal(session.view().receipts[0].request.constancyApplied, true);
  session.acknowledge();
  assert.equal(session.view().draft.constancyApplied, false);
  unsubscribe();
});

test('production root aggregates three sessions without sibling Undo or acknowledgement clearing another hold', async () => {
  const ts = req('typescript'), vm = require('node:vm');
  const script = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('App.ts', script, ts.ScriptTarget.Latest, true);
  const effect = ast.statements.find(node => ts.isExpressionStatement(node) &&
    node.getText(ast).includes('const views = [preferences?.view(), quality?.view(), presentation?.view()]'));
  assert.ok(effect);
  const rootOwner = { ...scope, su: 'NormalSU', suPath: scope.projectPath + '.su.sqlite' };
  const presentation = new prefs.SummaryVegetationPresentationPreferencesSession(rootOwner);
  presentation.apply({ ...wire.review, ...rootOwner });
  const options = new shared.ReportTitlePreferencesSession(rootOwner, 'test option peer');
  const qualityPeer = new shared.ReportTitlePreferencesSession(rootOwner, 'test quality peer');
  options.apply({ ...rootOwner, values: { title: 'saved' } });
  qualityPeer.apply({ ...rootOwner, values: { title: 'saved' } });
  let cleanup;
  const context = vm.createContext({ summaryPreferenceBusy: false, view: 'home',
    $projectState: { contextId: rootOwner.contextId, activeProject: rootOwner.project, projectPath: rootOwner.projectPath,
      activeSU: rootOwner.su, suPath: rootOwner.suPath },
    summaryVegetationPreferenceSession: () => options,
    summaryVegetationQualityPreferenceSession: () => qualityPeer,
    summaryVegetationPresentationPreferenceSession: () => presentation,
    $effect(callback) { cleanup = callback(); },
  });
  vm.runInContext(ts.transpileModule(effect.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText, context);
  presentation.editTitle('bad\0new');
  assert.equal(context.summaryPreferenceBusy, true);
  assert.equal(context.view, 'summary-vegetation');
  options.undo(); qualityPeer.undo();
  assert.equal(context.summaryPreferenceBusy, true);
  presentation.undo();
  assert.equal(context.summaryPreferenceBusy, false);
  presentation.editFlag('constancy', true);
  let release;
  const saving = presentation.save(() => new Promise(resolve => release = resolve));
  options.undo(); qualityPeer.undo();
  assert.equal(context.summaryPreferenceBusy, true);
  release(outcome(true)); await saving;
  assert.equal(context.summaryPreferenceBusy, true);
  qualityPeer.edit('invalid\0peer');
  presentation.acknowledge();
  assert.equal(context.summaryPreferenceBusy, true);
  qualityPeer.undo();
  assert.equal(context.summaryPreferenceBusy, false);
  cleanup();
  presentation.editTitle('invalid\0after cleanup');
  assert.equal(context.summaryPreferenceBusy, false);
});

test('production report wires one presentation pane, peer mutation holds and explicitly separate raw preview', async () => {
  const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform });
  const owner = { ...scope, su: 'NormalSU' };
  const presentation = new prefs.SummaryVegetationPresentationPreferencesSession(owner);
  presentation.apply({ ...wire.review, ...owner });
  const Panel = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8'),
    'SummaryVegetationReport.svelte', {
      './summaryVegetation': report,
      './summaryVegetationPresentationPreferenceSessions': { summaryVegetationPresentationPreferenceSession: () => presentation },
      '../bindings/github.com/boostao/vpro-wails': {},
    }, { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES: 'true' });
  const html = render(Panel, { props: { ...owner, onBusyChange() {} } }).body;
  assert.equal(html.split('data-summary-presentation-preferences').length - 1, 1);
  assert.equal(html.split('data-source-control="rptSvTitle"').length - 1, 1);
  assert.match(html, /Stored presentation drafts are separate and are not consumed by this raw crosstab preview/);
  const actions = componentFunctions('SummaryVegetationReport.svelte', ['loadPreferences', 'savePreferences'], {
    preferences: new shared.ReportTitlePreferencesSession(owner, 'test sibling'), presentationMutationHeld: true,
    viewState: { busy: false }, preferenceHeld: false, preferenceInvalid: false, operationHeld: false,
    qualityMutationHeld: false, qualityInvalid: false, presentationInvalid: false,
    SummaryVegetationPreferencesService: {
      GetSummaryVegetationPreferences() { throw new Error('held peer read dispatched'); },
      SaveSummaryVegetationPreferences() { throw new Error('held peer Save dispatched'); },
    },
  });
  await Promise.all([actions.actions.loadPreferences(), actions.actions.savePreferences()]);
});
