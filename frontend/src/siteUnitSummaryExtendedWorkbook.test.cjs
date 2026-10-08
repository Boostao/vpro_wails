const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { quality, restoration, summary, cell, fixture: environmentFixture } = require('./siteUnitSummaryTestFixtures.cjs');
const location = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': restoration });
const earth = loadTypeScript('googleEarthReview.ts', {
  './qualityEditor': quality, './projectMetadataRestore': restoration, './plotLocationReview': location,
});
const lifeform = loadTypeScript('lifeformSummary.ts');
const publication = loadTypeScript('publicationSession.ts');
const environment = loadTypeScript('environmentWorkbook.ts', {
  './lifeformSummary': lifeform, './publicationSession': publication, './longEnvironmentReport': {},
});
const shared = loadTypeScript('lifeformWorkbook.ts', {
  './lifeformSummary': lifeform, './speciesAttributeSummary': {},
  './environmentWorkbook': environment, './publicationSession': publication,
});
const workbook = loadTypeScript('siteUnitSummaryWorkbook.ts', {
  './siteUnitSummary': summary, './projectMetadataRestore': restoration, './lifeformSummary': lifeform,
  './lifeformWorkbook': shared, './publicationSession': publication, './environmentWorkbook': environment,
});
const species = loadTypeScript('siteUnitSummarySpecies.ts', {
  './projectMetadataRestore': restoration, './googleEarthReview': earth, './siteUnitSummary': summary,
});
const extended = loadTypeScript('siteUnitSummaryExtendedWorkbook.ts', {
  './googleEarthReview': earth, './lifeformSummary': lifeform, './lifeformWorkbook': shared,
  './siteUnitSummaryWorkbook': workbook, './siteUnitSummarySpecies': species,
  './publicationSession': publication, './environmentWorkbook': environment,
});
const owner = { contextId: 'owned', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
const destination = 'C:\\output\\ exact name .xlsx';
const options = (orderBy = 2, includeSpecies = 0) => ({ method: 1, orderBy, includeSpecies,
  coverCalculation: 1, andOr: 1, presenceGreaterThan: 0, coverGreaterThan: 0 });
function fixture(request = options(), owned = owner) {
  const environment = environmentFixture();
  Object.assign(environment, { contextId: owned.contextId, projectPath: owned.projectPath, suPath: owned.suPath });
  Object.assign(environment.report, { project: owned.project, su: owned.su, method: request.method,
    fields: structuredClone(workbook.siteUnitSummaryWorkbookFields) });
  if (request.orderBy === 2) {
    environment.report.querySource = 'selected-su-filtered-env-admin-quickveg-lifeform';
    environment.report.fields.splice(23, 4, ...summary.summaryLifeformCaptions.map((label, index) => ({
      source: 'Lifeform', key: `Lifeform${index}`, label, section: 'VEGETATION', kind: 'lifeform-cover',
    })));
    environment.report.units[0].values.splice(23, 4, ...summary.summaryLifeformCaptions.map(() => ''));
  }
  const { includeSpecies, ...criteria } = request;
  return { environment, options: { ...request }, scope: { savedMethod: 2, siteUnitType: 1, orderBy: 3, includeSpecies: 1 },
    species: includeSpecies === 0 ? null : { environment: structuredClone(environment), options: criteria,
      units: [{ code: '', nPlots: 2, groups: [{ index: 1,
        caption: request.orderBy === 1 ? 'Layer 1' : summary.summaryLifeformCaptions[1], rows: [{
          species: 'A', scientificName: cell('Scientific'), englishName: cell(null),
          codeType: request.orderBy === 1 ? null : cell('U'), cover: '1.01', presence: '100.0',
          physicalValues: 2, referenceRowIds: ['1'], included: request.andOr === 1
            ? 100 > request.presenceGreaterThan && 1.01 > request.coverGreaterThan
            : 100 > request.presenceGreaterThan || 1.01 > request.coverGreaterThan,
        }] }] }] },
    sheets: [{ unit: cell(''), name: 'NoName0' }], approvalHash: 'a'.repeat(64),
    workbookSHA256: 'b'.repeat(64), bytes: 1234 };
}
function receipt(status = 'published') {
  return { status, requestedDestination: destination, path: status === 'not-published' ? '' : destination,
    sha256: status === 'not-published' ? '' : 'b'.repeat(64), errorMessage: status === 'published' ? '' : 'Explicit diagnostic' };
}
test('extended review validates all three modes, both methods/calculations/combinations and exact 39/49 registries', () => {
  let cases = 0;
  for (const [order, include] of [[2, 0], [1, 1], [2, 1]]) for (const method of [1, 2]) {
    for (const calculation of include ? [1, 2] : [1]) for (const andOr of include ? [1, 2] : [1]) {
      const request = { ...options(order, include), method, coverCalculation: calculation, andOr };
      const value = fixture(request), result = extended.validateExtendedSummaryWorkbookReview(value, owner, request);
      assert.equal(result.environment.report.fields.length, order === 1 ? 39 : 49);
      assert.ok(Object.isFrozen(result.environment.report.units[0].plots));
      value.environment.report.units[0].values[0] = 'changed';
      assert.equal(result.environment.report.units[0].values[0], '');
      assert.equal(result.scope.orderBy, 3); assert.equal(result.scope.includeSpecies, 1);
      cases++;
    }
  }
  assert.equal(cases, 18);
  const request = { ...options(2, 1), presenceGreaterThan: 100, coverGreaterThan: 2 };
  const result = extended.validateExtendedSummaryWorkbookReview(fixture(request), owner, request);
  assert.equal(result.species.units[0].groups[0].rows[0].included, false);
  assert.equal(result.species.units[0].groups[0].caption, 'Coniferous Tree');
  const legacy = fixture(options(1, 1));
  assert.throws(() => workbook.validateSiteUnitSummaryWorkbookReview({ preview: legacy.environment,
    scope: legacy.scope, sheets: legacy.sheets, approvalHash: legacy.approvalHash,
    workbookSHA256: legacy.workbookSHA256, bytes: legacy.bytes }, owner, 1));
});
test('extended transport refuses extra/missing shapes, cross-projection differences and every explicit option mismatch', () => {
  const request = options(2, 1);
  const mutations = [
    v => v.extra = 1, v => delete v.species, v => v.species = null,
    v => v.options.extra = 1, v => delete v.options.andOr,
    ...Object.keys(request).map(key => v => v.options[key]++),
    v => v.scope.orderBy = 0, v => v.scope.siteUnitType = 2, v => v.scope.includeSpecies = 2,
    v => v.environment.contextId = 'foreign', v => v.environment.report.fields[23].label = 'invented',
    v => v.environment.report.units[0].values.pop(), v => v.sheets[0].name = 'invented',
    v => v.approvalHash = '', v => v.bytes = 0,
    v => v.species.extra = 1, v => v.species.options.extra = 1,
    v => v.species.environment.report.units[0].values[0] = 'different',
    v => v.species.units[0].extra = 1, v => v.species.units[0].groups[0].extra = 1,
    v => v.species.units[0].groups[0].rows[0].extra = 1,
    v => v.species.units[0].groups[0].rows[0].codeType.extra = 1,
    v => v.species.units[0].groups[0].rows[0].scientificName.extra = 1,
    v => v.species.units[0].groups[0].rows[0].englishName = cell('\ud800'),
    v => v.species.units[0].groups[0].rows[0].referenceRowIds = ['1'.repeat(32768)],
    v => v.species.units[0].groups[0].rows[0].presence = '50.0',
  ];
  for (const mutate of mutations) {
    const value = fixture(request); mutate(value);
    assert.throws(() => extended.validateExtendedSummaryWorkbookReview(value, owner, request));
  }
  for (const mutate of [v => v.orderBy = 1, v => v.coverCalculation = 2, v => v.andOr = 2,
    v => v.presenceGreaterThan = 1, v => v.coverGreaterThan = -1]) {
    const request = options(); mutate(request); assert.throws(() => extended.validateExtendedSummaryWorkbookOptions(request));
  }
  for (const threshold of [-32768, 32767]) {
    const request = { ...options(1, 1), presenceGreaterThan: threshold, coverGreaterThan: threshold };
    assert.doesNotThrow(() => extended.validateExtendedSummaryWorkbookReview(fixture(request), owner, request));
  }
  const absent = fixture(); absent.species = fixture(request).species;
  assert.throws(() => extended.validateExtendedSummaryWorkbookReview(absent, owner, options()));
});
test('persistent extension owns exact request, irreversible known/unknown receipts, retry and remount identities', async () => {
  for (const outcome of [receipt(), receipt('not-published'), receipt('published-with-errors'), null,
    { ...receipt(), sha256: 'c'.repeat(64) }, new Error('lost receipt')]) {
    const session = new extended.ExtendedSummaryWorkbookPublicationSession(owner);
    let resolve, calls = 0;
    const waiting = new Promise(done => resolve = done), request = options(2, 1);
    const publishing = session.publish(fixture(request), destination, async sent => {
      calls++; assert.deepEqual(sent, { ...request, approvalHash: 'a'.repeat(64), destination });
      await waiting; if (outcome instanceof Error) throw outcome; return outcome;
    }, request);
    assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
    assert.throws(() => session.acknowledge(), /Wait/);
    await assert.rejects(session.publish(fixture(), destination, async () => { calls++; }, options()), /Acknowledge/);
    assert.deepEqual(session.requestedOptions(), request);
    resolve(); await publishing;
    assert.equal(calls, 1); assert.equal(session.view().blocked, true);
    if (outcome && !(outcome instanceof Error) && outcome.sha256 !== 'c'.repeat(64)) {
      assert.equal(session.view().outcome.status, outcome.status);
    } else { assert.equal(session.view().outcome, null); assert.match(session.view().error, /outcome unknown/); }
    session.acknowledge(); assert.equal(session.view().blocked, false);
  }
  const owned = { ...owner, contextId: 'extended-remount' }, request = options();
  const session = extended.extendedSummaryWorkbookPublicationSession(owned);
  await session.publish(fixture(request, owned), destination, async () => null, request);
  const remount = extended.extendedSummaryWorkbookPublicationSession({ ...owned });
  assert.equal(remount, session); assert.equal(remount.view().blocked, true);
  assert.equal(remount.requestedOptions().orderBy, 2);
  assert.throws(() => extended.extendedSummaryWorkbookPublicationSession({ ...owned, suPath: 'foreign' }));
  const copy = remount.requestedOptions(); copy.orderBy = 1;
  assert.equal(remount.requestedOptions().orderBy, 2);
  remount.acknowledge();
});
test('stale criteria, unsafe destinations and invalid reviews never dispatch', async () => {
  const session = new extended.ExtendedSummaryWorkbookPublicationSession(owner), request = options(1, 1);
  let calls = 0;
  const port = async () => { calls++; return receipt(); };
  await assert.rejects(session.publish(fixture(request), destination, port, { ...request, coverGreaterThan: 2 }));
  for (const bad of ['', 'x.csv', 'x.xlsx ', '\ud800.xlsx', 'x\0.xlsx']) {
    await assert.rejects(session.publish(fixture(request), bad, port, request), /literal .xlsx/);
  }
  assert.equal(calls, 0); assert.equal(session.view().blocked, false);
});
test('actual extended panel handlers reject late canceled and changed-criteria reads and publish the exact reviewed request', async () => {
  const request = options(2, 1), requests = [], held = [];
  const session = new extended.ExtendedSummaryWorkbookPublicationSession(owner);
  const context = componentFunctions('SiteUnitSummaryWorkbookPanel.svelte',
    ['reportBusy', 'cancel', 'getReview', 'publish', 'acknowledge'], {
      enabled: true, held: false, normalOwner: true, validMethod: true, method: 1, extended: true,
      extendedOptions: request, owned: owner, generation: 0, disposed: false, busy: false, error: '', review: null,
      destination, currentReview: true, publication: session.view(), session, extendedSession: session, originalSession: null,
      onBusyChange: value => held.push(value), structuredClone,
      reads: { track: value => value, cancelAll: () => requests.forEach(value => value.cancel()) },
      sameExtendedSummaryWorkbookOptions: extended.sameExtendedSummaryWorkbookOptions,
      validateExtendedSummaryWorkbookReview: extended.validateExtendedSummaryWorkbookReview,
      SiteUnitSummaryExtendedWorkbookService: {
        GetReview(id, json) {
          let resolve;
          const value = new Promise(done => resolve = done);
          Object.assign(value, { resolve, id, json, canceled: false, cancel() { this.canceled = true; } });
          requests.push(value); return value;
        },
        ExportReviewed: async (id, json) => {
          assert.equal(id, owner.contextId);
          assert.deepEqual(JSON.parse(json), { ...request, approvalHash: 'a'.repeat(64), destination });
          return receipt();
        },
      },
    });
  session.subscribe(() => { context.publication = session.view(); if (context.publication.busy) context.review = null; });
  const canceled = context.actions.getReview(); context.actions.cancel();
  requests[0].resolve(fixture(request)); await canceled;
  assert.equal(requests[0].canceled, true); assert.equal(context.review, null);
  const stale = context.actions.getReview();
  context.extendedOptions = { ...request, coverGreaterThan: 2 };
  requests[1].resolve(fixture(request)); await stale; assert.equal(context.review, null);
  context.extendedOptions = request;
  const current = context.actions.getReview(); requests[2].resolve(fixture(request)); await current;
  assert.equal(context.review.options.orderBy, 2);
  await context.actions.publish(); assert.equal(session.view().blocked, true);
  context.actions.cancel(); assert.equal(session.view().blocked, true);
  context.actions.acknowledge(); assert.equal(session.view().blocked, false);
});
test('actual top-level effect aggregates independent old/extended authority even with both panels unmounted', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('App.svelte.ts', source, ts.ScriptTarget.Latest, true);
  const effect = ast.statements.find(node => ts.isExpressionStatement(node) &&
    node.getText(ast).startsWith('$effect(') && node.getText(ast).includes('extendedSummaryWorkbookPublicationSession'));
  assert.ok(effect);
  for (const oldGate of [false, true]) for (const newGate of [false, true]) {
    for (const oldHeld of [false, true]) for (const newHeld of [false, true]) {
      let cleanup, listeners = [], stopped = 0;
      const context = vm.createContext({
        summaryPublicationBusy: false, view: 'home',
        $projectState: { contextId: 'owned', activeProject: 'Sample', projectPath: 'C:\\project.db', activeSU: 'Selected', suPath: 'C:\\external.db' },
        $effect(run) { cleanup = run(); },
        siteUnitSummaryWorkbookPublicationSession: () => ({ view: () => ({ busy: oldHeld, blocked: false }),
          subscribe(run) { listeners.push(run); run(); return () => stopped++; } }),
        extendedSummaryWorkbookPublicationSession: () => ({ view: () => ({ busy: false, blocked: newHeld }),
          subscribe(run) { listeners.push(run); run(); return () => stopped++; } }),
      });
      const code = effect.getText(ast)
        .replaceAll('import.meta.env.VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK', JSON.stringify(newGate ? 'true' : 'false'))
        .replaceAll('import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK', JSON.stringify(oldGate ? 'true' : 'false'))
        .replaceAll('import.meta.env.VITE_SITE_UNIT_SUMMARY', "'true'");
      vm.runInContext(ts.transpileModule(code, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
      assert.equal(context.summaryPublicationBusy, oldGate && oldHeld || newGate && newHeld);
      assert.equal(context.view, newGate && newHeld ? 'summary-environment' : 'home');
      assert.equal(listeners.length, Number(oldGate) + Number(newGate));
      if (cleanup) cleanup();
      assert.equal(stopped, listeners.length);
    }
  }
});
test('extended UI keeps gates, visible labels and held authority while restoring the original lifeform mode on remount', () => {
  const panel = readFileSync(path.join(__dirname, 'SiteUnitSummaryWorkbookPanel.svelte'), 'utf8');
  const parent = readFileSync(path.join(__dirname, 'SiteUnitSummary.svelte'), 'utf8');
  for (const [name, source] of [['SiteUnitSummaryWorkbookPanel.svelte', panel], ['SiteUnitSummary.svelte', parent]]) {
    assert.equal(compile(source, { filename: name, generate: 'client' }).warnings.length, 0);
  }
  assert.match(parent, /retainedOptions\?\.orderBy === 2 \? 'lifeform'/);
  assert.match(parent, /extendedAuthority\?\.view\(\)\.blocked/);
  assert.match(parent, /\{extendedOptions\}/);
  assert.match(panel, /VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK === 'true'/);
  assert.match(panel, /sameExtendedSummaryWorkbookOptions\(requestedOptions, extendedOptions/);
  assert.match(panel, /extendedSession\.publish\(review, destination/);
  assert.doesNotMatch(panel, /reads\.track\(SiteUnitSummaryExtendedWorkbookService\.ExportReviewed/);
  for (const heading of ['Scientific Name', 'Common Name', '%Cover', '%Presence']) assert.ok(panel.includes(heading));
  assert.match(panel, /scope="row" aria-label=/);
});
