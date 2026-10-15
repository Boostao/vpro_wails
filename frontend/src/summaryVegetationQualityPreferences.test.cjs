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
const transport = loadTypeScript('siviParentTransport.ts', {
  './projectMetadataRestore': cells, './siviParentEditor': {}, './siviParentSession': {},
});
const privateFile = name => path.relative(path.join(frontend, 'src'), path.join(__dirname, name));
const references = loadTypeScript(privateFile('qualityMinimumReference.ts'), {
  './lifeformSummary': lifeform, './projectMetadataRestore': cells, './siviParentTransport': transport,
});
const prefs = loadTypeScript(privateFile('summaryVegetationQualityPreferences.ts'), {
  './googleEarthReview': earth, './lifeformSummary': lifeform, './qualityEditor': quality,
  './qualityMinimumReference': references, './reportTitlePreferences': shared,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationQualityPreferencesWire.json'), 'utf8'));
const { values: ignoredValues, source: ignoredSource, qualityDefinitions: ignoredQuality, becDefinitions: ignoredBEC, ...scope } = wire.review;
const reviewed = values => ({ ...structuredClone(wire.review), values });
const ready = (values = wire.review.values) => {
  const session = new prefs.SummaryVegetationQualityPreferencesSession(scope);
  session.apply(reviewed(values));
  return session;
};
const outcome = (committed, errorMessage = '') => ({ contextId: scope.contextId, changed: committed, committed, errorMessage });
const clone = value => structuredClone(value);

test('actual production Go wire detaches genuine Svelte proxies and sends the exact strict request', async () => {
  const input = proxy(clone(wire.review));
  assert.throws(() => structuredClone(input));
  const detached = prefs.validateSummaryVegetationQualityPreferences(input, scope);
  assert.deepEqual(clone(detached), wire.review);
  input.values.siteMinimum = 'different';
  assert.equal(detached.values.siteMinimum, wire.review.values.siteMinimum);
  const session = ready();
  session.editMinimum('siteMinimum', wire.request.proposed.siteMinimum);
  session.editMinimum('becMinimum', wire.request.proposed.becMinimum);
  session.editNullFlag('becIncludeNulls', wire.request.proposed.becIncludeNulls);
  await session.save(request => {
    assert.deepEqual(request, wire.request);
    return Promise.resolve(wire.outcome);
  });
  assert.deepEqual(session.view().outcome, wire.outcome);
  assert.deepEqual(clone(session.reference()), {
    source: wire.review.source, qualityDefinitions: wire.review.qualityDefinitions, becDefinitions: wire.review.becDefinitions,
  });
});
test('all original NULL combinations and unchanged enforcement state remain literal owned noops', async () => {
  for (const enforce of [false, true]) for (let mask = 0; mask < 16; mask++) {
    const values = { ...wire.review.values, enforce,
      siteIncludeNulls: !!(mask & 1), vegIncludeNulls: !!(mask & 2), soilIncludeNulls: !!(mask & 4), becIncludeNulls: !!(mask & 8) };
    const session = ready(values);
    await session.save(request => {
      assert.deepEqual(request, { expected: values, proposed: values });
      return Promise.resolve(outcome(false));
    });
    session.acknowledge();
    session.editNullFlag('becIncludeNulls', !values.becIncludeNulls);
    await session.save(request => {
      assert.equal(request.proposed.enforce, enforce);
      assert.equal(request.proposed.becIncludeNulls, !values.becIncludeNulls);
      return Promise.resolve(outcome(true));
    });
  }
});
test('four text drafts preserve empty, whitespace, case, nonmembers and historical NUL omission', async () => {
  for (const field of ['siteMinimum', 'vegMinimum', 'soilMinimum', 'becMinimum']) {
    for (const raw of ['', '  Poor ', 'poor', 'literal nonmember longer than fifteen units', 'old\0historical']) {
      const values = { ...wire.review.values, [field]: raw };
      const session = ready(values);
      session.editNullFlag('becIncludeNulls', !values.becIncludeNulls);
      await session.save(request => {
        assert.equal(request.expected[field], raw); assert.equal(request.proposed[field], raw);
        return Promise.resolve(outcome(true));
      });
      session.acknowledge();
      session.editMinimum(field, 'bad\0new');
      assert.ok(session.view().draftError);
      await assert.rejects(() => session.save(() => { throw new Error('invalid draft dispatched'); }));
      session.editMinimum(field, '\ud800');
      assert.ok(session.view().draftError);
      session.undo(); assert.equal(session.view().draftError, '');
      session.editMinimum(field, 'corrected');
      assert.equal(session.view().draftError, '');
    }
  }
});
test('source enforcement stays readonly; invalid edits do not alter reference authority', async () => {
  const session = ready();
  assert.throws(() => session.editNullFlag('enforce', true));
  session.editDraft({ ...session.view().draft, enforce: !wire.review.values.enforce });
  assert.match(session.view().draftError, /Form_Load/);
  await assert.rejects(() => session.save(() => { throw new Error('mutating Load invented'); }));
  session.undo();
  const original = session.reference();
  const invalid = clone(wire.review);
  invalid.becDefinitions.columns[0].declaredType = 'NUMERIC';
  assert.throws(() => session.apply(invalid));
  assert.deepEqual(session.reference(), original);
  assert.deepEqual(session.view().saved, wire.review.values);
});
test('owned exact references reject wrong scopes, duplicate identities, partial metadata and sparse arrays', () => {
  const mutations = [
    value => { value.contextId = 'foreign'; },
    value => { value.source += ' '; },
    value => { value.values.extra = true; },
    value => { value.values.becIncludeNulls = 1; },
    value => { value.values.becMinimum = '\ud800'; },
    value => { value.qualityDefinitions.columns[0].type = 'TEXT'; },
    value => { value.becDefinitions.columns.length = 11; },
    value => { value.qualityDefinitions.rows.push(clone(value.qualityDefinitions.rows[0])); },
    value => { value.qualityDefinitions.rows[0].cells.length = 9; },
    value => { delete value.qualityDefinitions.rows[0].cells[3]; },
    value => { value.qualityDefinitions.rows[0].cells[0].text = 'DataQuality'; },
    value => { value.qualityDefinitions.rows[0].rowId = '01'; },
  ];
  for (const mutate of mutations) {
    const value = clone(wire.review); mutate(value);
    assert.throws(() => prefs.validateSummaryVegetationQualityPreferences(value, scope));
  }
  const value = clone(wire.review);
  value.becDefinitions.rows = [];
  assert.equal(prefs.validateSummaryVegetationQualityPreferences(value, scope).becDefinitions.rows.length, 0);
});
test('failed reload blocks Save through editing, Undo and registry remount until successful explicit Load', async () => {
  const registry = prefs.createSummaryVegetationQualityPreferencesRegistry();
  const session = registry(scope);
  session.apply(wire.review);
  session.rejectRead(new Error('owned reload failed'));
  session.editMinimum('siteMinimum', 'corrected'); session.undo();
  assert.ok(registry(scope).view().readError);
  await assert.rejects(() => session.save(() => { throw new Error('failed read erased by Save'); }));
  session.cancelLoad();
  assert.ok(session.view().readError);
  session.apply(wire.review);
  assert.equal(session.view().readError, '');
  assert.equal(session.view().draftError, '');
});
test('pending and unknown receipts persist across remount; acknowledgement never restores lost authority', async () => {
  const registry = prefs.createSummaryVegetationQualityPreferencesRegistry();
  const session = registry(scope); session.apply(wire.review);
  session.editMinimum('becMinimum', '10');
  let release;
  const save = session.save(() => new Promise(resolve => { release = resolve; }));
  assert.equal(registry(scope).view().busy, true);
  assert.throws(() => session.undo()); assert.throws(() => session.acknowledge());
  release({ ...outcome(true), contextId: 'foreign' });
  await save;
  assert.equal(session.view().authorityUnknown, true);
  assert.equal(session.view().receipts[0].kind, 'unknown');
  session.acknowledge();
  assert.equal(session.view().saved, null);
  await assert.rejects(() => session.save(() => Promise.resolve(outcome(true))));
  session.apply(reviewed({ ...wire.review.values, becMinimum: '10' }));
  assert.equal(session.view().authorityUnknown, false);
  assert.equal(session.view().receipts.length, 1);
});
test('cancelled generation cannot repopulate a retained new draft or overwrite recovery', () => {
  const session = ready();
  const generation = session.beginLoad();
  session.cancelLoad();
  session.editMinimum('becMinimum', 'new draft');
  assert.equal(session.applyLoaded(wire.review, generation), false);
  assert.equal(session.view().draft.becMinimum, 'new draft');
  const next = session.beginLoad();
  assert.equal(session.applyLoaded(wire.review, next, true), true);
  assert.equal(session.view().draft.becMinimum, 'new draft');
});
test('successful recovery notifications expose new references and cleared read error together', () => {
  const session = ready();
  session.rejectRead('failed');
  const next = clone(wire.review);
  next.becDefinitions.rows = [];
  const observed = [];
  const release = session.subscribe(() => observed.push({
    readError: session.view().readError, becRows: session.reference().becDefinitions.rows.length,
  }));
  observed.length = 0;
  session.apply(next);
  release();
  assert.ok(observed.length > 0);
  assert.ok(observed.every(value => value.readError === '' && value.becRows === 0));
});
test('compiled private panel has one labeled control per original quality field and readonly enforcement', () => {
  const filename = privateFile('SummaryVegetationQualityPreferences.svelte');
  const Panel = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationQualityPreferences.svelte'), 'utf8'),
    filename, { './summaryVegetationQualityPreferences': prefs });
  const result = render(Panel, { props: {
    contextId: scope.contextId, preferences: ready(), port: {
      read() { throw new Error('SSR must not launch reads'); }, save() { throw new Error('SSR must not Save'); },
    }, onReadBusyChange() {},
  } });
  const html = result.body;
  for (const field of [...prefs.summaryVegetationQualityMinimumFields, ...prefs.summaryVegetationQualityNullFields]) {
    assert.equal(html.split(`data-source-control="${field.control}"`).length - 1, 1);
    assert.ok(html.includes(`for="summary-quality-${field.key}"`));
    assert.ok(html.includes(field.label.replace('>=', '&gt;=')) || html.includes(field.label));
  }
  assert.equal(html.split('data-source-control="optEnforceQC"').length - 1, 1);
  assert.match(html, /disabled[^>]*data-source-control="optEnforceQC"/);
  assert.match(html, /does not enable QC/);
});
test('actual private panel handlers preserve failed reload guards and cancel late reads without replay', async () => {
  const session = ready();
  let release;
  const held = new Promise(resolve => { release = resolve; });
  held.cancel = () => {};
  let dispatched = 0;
  const context = componentFunctions(privateFile('SummaryVegetationQualityPreferences.svelte'),
    ['cancel', 'load', 'save', 'editMinimum', 'editNull'], {
      contextId: scope.contextId, preferences: session, busy: false, view: session.view(), operationHeld: false,
      mutationHeld: false, generation: 0, error: '', onReadBusyChange() {},
      reads: { cancelAll() {}, track(value) { return value; } },
      port: { read() { return held; }, save() { dispatched++; return Promise.resolve(wire.outcome); } },
    });
  const unsubscribe = session.subscribe(() => { context.view = session.view(); });
  const load = context.actions.load();
  context.actions.cancel();
  release(wire.review);
  await load;
  assert.equal(context.busy, false);
  session.rejectRead('owned reload failure');
  context.actions.editMinimum('siteMinimum', 'corrected');
  await context.actions.save();
  assert.equal(dispatched, 0);
  assert.ok(session.view().readError);
  session.undo();
  context.port.read = () => Promise.resolve(wire.review);
  await context.actions.load();
  assert.equal(session.view().readError, '');
  context.actions.editMinimum('siteMinimum', wire.request.proposed.siteMinimum);
  context.actions.editMinimum('becMinimum', wire.request.proposed.becMinimum);
  context.actions.editNull('becIncludeNulls', wire.request.proposed.becIncludeNulls);
  await context.actions.save();
  assert.equal(dispatched, 1);
  assert.equal(session.view().blocked, true);
  unsubscribe();
});
test('production root effect combines both retained preference holds without one clearing the other', async () => {
  const ts = req('typescript'), vm = require('node:vm');
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('App.ts', source, ts.ScriptTarget.Latest, true);
  const effect = ast.statements.find(node => ts.isExpressionStatement(node) && node.getText(ast).includes('const views = [preferences?.view(), quality?.view(), presentation?.view()]'));
  assert.ok(effect);
  const rootOwner = { ...scope, su: 'NormalSU', suPath: scope.projectPath + '.su.sqlite' };
  const qualitySession = new prefs.SummaryVegetationQualityPreferencesSession(rootOwner);
  qualitySession.apply({ ...wire.review, ...rootOwner });
  const optionsSession = new shared.ReportTitlePreferencesSession(rootOwner, 'test sibling');
  optionsSession.apply({ ...rootOwner, values: { title: 'saved' } });
  let cleanup;
  const context = vm.createContext({
    summaryPreferenceBusy: false, view: 'home',
    $projectState: { contextId: rootOwner.contextId, activeProject: rootOwner.project, projectPath: rootOwner.projectPath,
      activeSU: rootOwner.su, suPath: rootOwner.suPath },
    summaryVegetationPreferenceSession: () => optionsSession,
    summaryVegetationQualityPreferenceSession: () => qualitySession,
    summaryVegetationPresentationPreferenceSession: null,
    $effect(callback) { cleanup = callback(); },
  });
  vm.runInContext(ts.transpileModule(effect.getText(ast), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText, context);
  qualitySession.editMinimum('siteMinimum', 'bad\0new');
  assert.equal(context.summaryPreferenceBusy, true);
  assert.equal(context.view, 'summary-vegetation');
  optionsSession.undo();
  assert.equal(context.summaryPreferenceBusy, true);
  qualitySession.undo();
  assert.equal(context.summaryPreferenceBusy, false);
  optionsSession.edit('changed');
  let release;
  const save = optionsSession.save(() => new Promise(resolve => { release = resolve; }));
  assert.equal(context.summaryPreferenceBusy, true);
  qualitySession.undo();
  assert.equal(context.summaryPreferenceBusy, true);
  release(outcome(true));
  await save;
  assert.equal(context.summaryPreferenceBusy, true);
  optionsSession.acknowledge();
  assert.equal(context.summaryPreferenceBusy, false);
  cleanup();
});
test('production Summary pane refuses unqualified enforced preview and holds sibling saves during quality mutation', () => {
  const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform });
  const owner = { ...scope, su: 'NormalSU' };
  const qualitySession = new prefs.SummaryVegetationQualityPreferencesSession(owner);
  qualitySession.apply({ ...wire.review, ...owner, values: { ...wire.review.values, enforce: true } });
  const Panel = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8'),
    'SummaryVegetationReport.svelte', {
      './summaryVegetation': report,
      './summaryVegetationQualityPreferenceSessions': { summaryVegetationQualityPreferenceSession: () => qualitySession },
      '../bindings/github.com/boostao/vpro-wails': {},
    }, { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_QUALITY_PREFERENCES: 'true' });
  const html = render(Panel, { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(html.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
  assert.match(html, /no unfiltered report is returned/);
  assert.equal(html.split('data-source-control="BEC_Use"').length - 1, 1);
  const unloaded = new prefs.SummaryVegetationQualityPreferencesSession(owner);
  const actions = componentFunctions('SummaryVegetationReport.svelte', ['loadPreferences', 'savePreferences'], {
    preferences: new shared.ReportTitlePreferencesSession(owner, 'test sibling'), qualityMutationHeld: true,
    viewState: { busy: false }, preferenceHeld: false, preferenceInvalid: false, operationHeld: false,
    qualityInvalid: false, quality: unloaded,
    SummaryVegetationPreferencesService: { GetSummaryVegetationPreferences() { throw new Error('held peer read dispatched'); },
      SaveSummaryVegetationPreferences() { throw new Error('held peer Save dispatched'); } },
  });
  return Promise.all([actions.actions.loadPreferences(), actions.actions.savePreferences()]);
});
