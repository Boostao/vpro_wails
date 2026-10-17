const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const runtime = require('svelte/internal/client');
const compiler = require('svelte/compiler');
const ts = require('typescript');
const vm = require('node:vm');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const fields = loadTypeScript('summaryVegetationFieldList.ts');
const vegetation = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform });
const numeric = loadTypeScript('summaryVegetationEnvironmentalNumeric.ts', { './summaryVegetationFieldList': fields });
const environmental = loadTypeScript('summaryVegetationEnvironmental.ts', {
  './lifeformSummary': lifeform, './summaryVegetationFieldList': fields,
  './summaryVegetation': vegetation, './summaryVegetationEnvironmentalNumeric': numeric,
});
const publication = loadTypeScript('publicationSession.ts');
const environmentWorkbook = loadTypeScript('environmentWorkbook.ts', {
  './lifeformSummary': lifeform, './longEnvironmentReport': {}, './publicationSession': publication,
});
const workbook = loadTypeScript('lifeformWorkbook.ts', {
  './lifeformSummary': lifeform, './speciesAttributeSummary': {},
  './publicationSession': publication, './environmentWorkbook': environmentWorkbook,
});
const client = loadTypeScript('summaryVegetationWorkbook.ts', {
  './lifeformSummary': lifeform, './lifeformWorkbook': workbook,
  './summaryVegetation': vegetation, './summaryVegetationEnvironmental': environmental,
  './publicationSession': publication, './environmentWorkbook': environmentWorkbook,
});
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookWire.json'), 'utf8'));
const ancillaryWire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookAncillaryWire.json'), 'utf8'));
const referenceWire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookReferencesWire.json'), 'utf8'));
const groupedWire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookGroupedWire.json'), 'utf8'));
const strataReferenceWire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookStrataReferencesWire.json'), 'utf8'));
const groupedAttributeWire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWorkbookGroupedAttributesWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath']
  .map(key => [key, wire.vegetation.preview[key]]));
const yieldNow = () => Promise.resolve();
const plain = value => JSON.parse(JSON.stringify(value));
const deferred = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};
const destination = 'C:\\output\\ exact workbook .xlsx';
function sessionFor(value) {
  const session = new client.SummaryVegetationWorkbookSession(owner);
  session.edit('title', value.options.title);
  session.includeEnvironment(value.options.includeEnv);
  if (value.options.ancillary) {
    session.includeAncillary(true);
    session.includeSummary(value.options.ancillary.createSummary);
  }
  if (value.options.references) {
    session.selectReferences('include', true);
    session.selectReferences('common', value.options.references.showCommon);
    session.selectReferences('guide', value.options.references.useGuideNames);
  }
  return session;
}
function outcome(value, status = 'published') {
  return { status, requestedDestination: destination, path: 'C:\\output\\resolved.xlsx',
    sha256: value.workbookSHA256, errorMessage: status === 'published' ? '' : 'Committed, source changed; do not replay.' };
}

test('strict production Go wire validates both frames and explicit nonlegacy environmental calculation without widening old entrypoint', async () => {
  for (const value of Object.values(wire)) {
    const result = await client.validateSummaryVegetationWorkbookReview(value, owner, value.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(Object.isFrozen(result), true);
    assert.equal(Object.isFrozen(result.options), true);
    assert.equal(Object.isFrozen(result.preview.rows[0].covers), true);
  }
  const value = wire.environment;
  assert.equal(wire.vegetation.options.showClass, 1);
  assert.equal(wire.vegetation.preview.showClass, undefined);
  assert.deepEqual([value.options.average, value.options.display, value.options.showClass], [2, 4, 20]);
  assert.equal(value.environment.rows.length, 30);
  assert.equal(value.firstVegetationRow, 36);
  await assert.rejects(environmental.validateSummaryEnvironmentalLifeform(value.environment, owner, yieldNow));
  assert.ok(await environmental.validateSummaryEnvironmentalLifeformOptions(
    value.environment, owner, value.options, yieldNow));
});

test('actual Svelte proxy is detached, frozen and preserves literal title without clone-at-proxy failures', async () => {
  const source = proxy(structuredClone(wire.environment));
  assert.throws(() => structuredClone(source));
  const result = await client.validateSummaryVegetationWorkbookReview(source, owner, source.options, yieldNow);
  source.options.title = 'caller';
  source.preview.rows[0].species.text = 'caller';
  source.environment.rows[0].values[0].cell.text = 'caller';
  assert.deepEqual(plain(result), wire.environment);
  assert.equal(result.options.title, '=literal 001');
});

test('review refuses malformed scope, frame, owner, approval, requested options, hidden mismatched preview and XML text', async () => {
  const changes = [
    value => { value.extra = 1; },
    value => { delete value.environment; },
    value => { value.options.title += ' '; },
    value => { value.options.includeEnv = null; },
    value => { value.options.showClass = '20'; },
    value => { value.options.extra = true; },
    value => { value.scope = 'original Report'; },
    value => { value.sheet = 'Different'; },
    value => { value.approvalHash = 'A'.repeat(64); },
    value => { value.workbookSHA256 = ''; },
    value => { value.bytes = 0; },
    value => { value.bytes = '123'; },
    value => { value.firstVegetationRow++; },
    value => { value.preview.contextId = 'foreign'; },
    value => { value.environment = null; },
    value => { value.environment.vegetation.rows[0].covers[0].text = 'changed'; },
    value => { value.environment.masterSourcePath = ''; },
    value => { value.environment.sourcePath = 'bad\u0001path'; },
    value => { value.preview.columns.reverse(); },
    value => { value.preview.rows = []; },
  ];
  for (const [index, change] of changes.entries()) {
    const source = structuredClone(wire.environment); change(source);
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(source, owner,
      wire.environment.options, yieldNow), `case ${index}`);
  }
  const source = structuredClone(wire.vegetation);
  source.environment = wire.environment.environment;
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(source, owner, source.options, yieldNow));
  for (const title of ['\ud800', 'bad\u0001title', 'a'.repeat(32768)]) {
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(wire.vegetation, owner,
      { ...wire.vegetation.options, title }, yieldNow));
  }
});

test('per-owner drafts and correction/Undo survive remount while validation remains an independent close barrier', () => {
  const session = client.summaryVegetationWorkbookSession(owner);
  session.edit('title', '\ud800');
  assert.equal(session.held(), true);
  const remount = client.summaryVegetationWorkbookSession(owner);
  assert.equal(remount, session);
  assert.equal(remount.view().title, '\ud800');
  assert.match(remount.view().draftError, /Title/);
  remount.edit('title', '=literal 001');
  assert.equal(remount.held(), false);
  remount.edit('destination', 'C:\\bad.csv');
  assert.match(remount.view().draftError, /xlsx/);
  remount.undoDraft();
  assert.equal(remount.view().title, '');
  assert.equal(remount.view().draftError, '');
  assert.equal(remount.held(), false);
  assert.throws(() => client.summaryVegetationWorkbookSession({ ...owner, suPath: 'foreign' }), /different owned paths/);
  assert.notEqual(client.summaryVegetationWorkbookSession({ ...owner, contextId: 'another' }), session);
});

test('held asynchronous validation cancellation keeps ownership through remount until settlement and never populates stale result', async () => {
  const value = wire.environment, session = sessionFor(value);
  const entered = deferred(), release = deferred();
  let first = true, cancelled = 0;
  const read = Promise.resolve(value);
  read.cancel = () => { cancelled++; };
  const showing = session.review(value.options, {
    review: () => read,
    validationYield: async () => {
      if (first) { first = false; entered.resolve(); await release.promise; }
    },
  });
  await entered.promise;
  session.cancel();
  assert.equal(session.held(), true);
  assert.match(session.view().error, /cancelled/);
  await assert.rejects(session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow }), /prior operation/);
  release.resolve();
  assert.equal(await showing, false);
  assert.equal(cancelled, 1);
  assert.equal(session.view().review, null);
  assert.equal(session.held(), false);
  assert.equal(await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow }), true);
  assert.equal(session.view().error, '');
});

test('generation change at final validation boundary returns null rather than publishing a stale plan', async () => {
  let calls = 0;
  await client.validateSummaryVegetationWorkbookReview(wire.vegetation, owner, wire.vegetation.options,
    async () => { calls++; });
  assert.ok(calls > 1);
  let current = true, observed = 0;
  const result = await client.validateSummaryVegetationWorkbookReview(wire.vegetation, owner,
    wire.vegetation.options, async () => { if (++observed === calls) current = false; }, () => current);
  assert.equal(result, null);
  assert.equal(observed, calls);
});

test('publication uses literal immutable reviewed request and preserves irreversible receipt across acknowledgement', async () => {
  const value = wire.environment, session = sessionFor(value);
  await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
  session.edit('destination', destination);
  const release = deferred();
  let request;
  const publishing = session.publish(value.options, sent => { request = sent; return release.promise; });
  assert.equal(session.held(), true);
  assert.equal(session.view().publication.busy, true);
  assert.equal(session.view().review, null);
  assert.deepEqual(plain(request), { options: value.options, approvalHash: value.approvalHash, destination });
  assert.throws(() => session.acknowledge(), /cannot be cancelled or replayed/);
  assert.throws(() => session.edit('destination', 'C:\\other.xlsx'), /acknowledge/);
  release.resolve(outcome(value, 'published-with-errors'));
  await publishing;
  assert.equal(session.view().publication.blocked, true);
  assert.equal(session.view().publication.outcome.status, 'published-with-errors');
  await assert.rejects(session.publish(value.options, () => { throw new Error('replay'); }), /prior outcome/);
  const receipt = plain(session.view().publication.outcome);
  session.acknowledge();
  assert.equal(session.held(), false);
  assert.deepEqual(plain(session.view().publication.outcome), receipt);
  assert.equal(session.view().publication.requestedDestination, destination);
  await assert.rejects(session.publish(value.options, () => { throw new Error('replay'); }), /Review the current/);
});

test('unknown transport or inconsistent acknowledgement blocks replay; correction cannot erase receipt', async () => {
  for (const returned of [null, { ...outcome(wire.vegetation), sha256: 'f'.repeat(64) }]) {
    const value = wire.vegetation, session = sessionFor(value);
    await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
    session.edit('destination', destination);
    await session.publish(value.options, () => Promise.resolve(returned));
    assert.match(session.view().publication.error, /outcome unknown.*Do not repeat/);
    assert.equal(session.held(), true);
    assert.throws(() => session.undoDraft(), /acknowledge/);
    session.acknowledge();
    session.edit('title', 'corrected');
    assert.match(session.view().publication.error, /outcome unknown/);
    assert.equal(session.view().publication.requestedDestination, destination);
  }
});

test('changing Calculate/Display invalidates review at publication and no destination is dispatched', async () => {
  const value = wire.environment, session = sessionFor(value);
  await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
  session.edit('destination', destination);
  let dispatched = false;
  await assert.rejects(session.publish({ ...value.options, average: 1 },
    () => { dispatched = true; return Promise.resolve(outcome(value)); }), /current explicit/);
  assert.equal(dispatched, false);
});

function mountPanel(session, services, held = false, calculation = wire.environment.options, ancillaryEnabled = false,
  referencesEnabled = false, groupedEnabled = false) {
  const filename = 'SummaryVegetationWorkbookPanel.svelte';
  const source = readFileSync(path.join(__dirname, filename), 'utf8');
  assert.deepEqual(compiler.compile(source, { filename, generate: 'client' }).warnings, []);
  let destroy;
  const notifications = [];
  let script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1].replace('$props()', '__props');
  script += '\nexport const handlers = {review, publish, edit, undo, acknowledge, selectAncillary, selectReferences, view: () => viewState};';
  const stripped = ts.transpileModule(script, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
  }).outputText;
  const compiled = compiler.compileModule(stripped, { filename: 'summary-workbook-panel.svelte.js', generate: 'client' });
  const module = { exports: {} };
  const code = ts.transpileModule(compiled.js.code.replaceAll('import.meta.env', '__environment'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText;
  const dependencies = {
    'svelte/internal/client': runtime,
    svelte: { onDestroy: callback => { destroy = callback; }, untrack: runtime.untrack },
    '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: services },
    './summaryVegetation': vegetation,
    './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
    './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => session },
  };
  vm.runInNewContext(code, { module, exports: module.exports, structuredClone, setTimeout,
    __props: { owner, calculation, disabled: held, onBusyChange: value => notifications.push(value) },
    __environment: { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
      VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK: ancillaryEnabled ? 'true' : 'false',
      VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: referencesEnabled ? 'true' : 'false',
      VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: groupedEnabled ? 'true' : 'false' },
    require(name) { assert.ok(name in dependencies, name); return dependencies[name]; },
  });
  return { ...module.exports.handlers, notifications, destroy: () => destroy() };
}

test('actual compiled Svelte panel accepts client proxy, corrects invalid DOM value and retains receipt on remount', async () => {
  const value = wire.environment, session = sessionFor(value);
  let reviews = 0, publications = 0;
  const services = {
    GetReview: (contextId, json) => {
      reviews++; assert.equal(contextId, owner.contextId);
      assert.deepEqual(JSON.parse(json), value.options);
      return Promise.resolve(proxy(structuredClone(value)));
    },
    ExportReviewed: (contextId, json) => {
      publications++; assert.equal(contextId, owner.contextId);
      assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash, destination });
      return Promise.resolve(outcome(value));
    },
  };
  const pane = mountPanel(session, services);
  pane.edit('title', '\ud800');
  assert.match(pane.view().draftError, /Title/);
  await pane.review(); assert.equal(reviews, 0);
  pane.edit('title', value.options.title);
  assert.equal(pane.view().draftError, '');
  await pane.review();
  assert.equal(pane.view().review.environment.rows.length, 30);
  assert.equal(Object.isFrozen(pane.view().review), true);
  pane.edit('destination', destination);
  await pane.publish();
  assert.equal(publications, 1);
  assert.equal(pane.notifications.at(-1), true);
  pane.destroy();
  const remounted = mountPanel(session, services);
  assert.equal(remounted.view().publication.blocked, true);
  assert.equal(remounted.view().publication.outcome.sha256, value.workbookSHA256);
  await remounted.publish(); assert.equal(publications, 1);
  remounted.acknowledge();
  assert.equal(remounted.notifications.at(-1), false);
  assert.equal(remounted.view().publication.outcome.sha256, value.workbookSHA256);
  remounted.destroy();
});

test('actual compiled panel destroy cancels read while persistent owner remains held until it returns', async () => {
  const value = wire.environment, session = sessionFor(value);
  const release = deferred();
  let cancelled = 0;
  release.promise.cancel = () => { cancelled++; };
  const services = { GetReview: () => release.promise };
  const pane = mountPanel(session, services);
  const showing = pane.review();
  assert.equal(session.held(), true);
  pane.destroy();
  const remounted = mountPanel(session, services);
  assert.equal(remounted.notifications.at(-1), true);
  release.resolve(value);
  await showing;
  assert.equal(cancelled, 1);
  assert.equal(remounted.view().review, null);
  assert.match(remounted.view().error, /cancelled/);
  assert.equal(remounted.notifications.at(-1), false);
  remounted.destroy();
});

test('actual root subscription restores unmounted draft/receipt authority and combines both Summary publication families', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('App.svelte.ts', script, ts.ScriptTarget.Latest, true);
  const effect = ast.statements.filter(node => ts.isExpressionStatement(node) && ts.isCallExpression(node.expression) &&
    node.expression.expression.getText(ast) === '$effect' &&
    node.getText(ast).includes('summaryVegetationWorkbookSession'));
  assert.equal(effect.length, 1);
  for (const enabled of [true, false]) for (const held of [true, false]) {
    let subscribed = 0, unsubscribed = 0;
    const context = vm.createContext({ summaryVegetationPublicationBusy: false, view: 'home',
      $projectState: { contextId: 'owned', activeProject: 'Sample', projectPath: 'C:\\project.db',
        activeSU: 'Summary', suPath: 'C:\\external.db' },
      summaryVegetationWorkbookSession(value) {
        assert.equal(value.contextId, 'owned');
        assert.equal(value.suPath, 'C:\\external.db');
        return { held: () => held, subscribe(listener) {
          subscribed++; listener(); return () => { unsubscribed++; };
        } };
      },
    });
    context.$effect = run => { context.cleanup = run(); };
    const code = effect[0].getText(ast).replaceAll("import.meta.env.VITE_SUMMARY_VEGETATION_WORKBOOK",
      JSON.stringify(enabled ? 'true' : 'false')).replaceAll("import.meta.env.VITE_SUMMARY_VEGETATION", "'true'");
    vm.runInContext(ts.transpileModule(code, {
      compilerOptions: { target: ts.ScriptTarget.ES2022 },
    }).outputText, context);
    assert.equal(subscribed, enabled ? 1 : 0);
    assert.equal(context.summaryVegetationPublicationBusy, enabled && held);
    assert.equal(context.view, enabled && held ? 'summary-vegetation' : 'home');
    if (enabled) { context.cleanup(); assert.equal(unsubscribed, 1); }
  }
  assert.match(source, /summaryPublicationBusy = \$derived\(summaryEnvironmentPublicationBusy \|\| summaryVegetationPublicationBusy\)/);
});

test('strict Go ancillary wire validates all conditional frames, exact source metadata and detached actual client proxies', async () => {
  for (const value of Object.values(ancillaryWire)) {
    const source = proxy(structuredClone(value));
    assert.throws(() => structuredClone(source));
    const result = await client.validateSummaryVegetationWorkbookReview(source, owner, source.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(Object.isFrozen(result.ancillary), true);
    assert.equal(Object.isFrozen(result.ancillary.sheets), true);
    if (result.ancillary.summary) {
      assert.equal(Object.isFrozen(result.ancillary.summary.units[0]), true);
      source.ancillary.summary.units[0].longName = 'caller';
      assert.equal(result.ancillary.summary.units[0].longName, '=Alpha_x0041_');
    }
    source.options.ancillary.createSummary = !source.options.ancillary.createSummary;
    assert.deepEqual(plain(result), value);
  }
});

test('ancillary validator rejects widened optional keys, timestamps, counts, version states, sheet collisions and NULL/empty repairs', async () => {
  const changes = [
    value => { delete value.ancillary; },
    value => { value.ancillary = null; },
    value => { value.options.ancillary = null; },
    value => { value.options.ancillary.createSummary = 'true'; },
    value => { value.options.ancillary.extra = false; },
    value => { value.ancillary.extra = false; },
    value => { value.ancillary.createdAt = '2026-10-17T02:00:00+00:00'; },
    value => { value.ancillary.createdAt = '2026-10-17T02:00:00.000Z'; },
    value => { value.ancillary.createdAt = '2026-02-30T02:00:00Z'; },
    value => { value.ancillary.createdAt = '0099-10-17T02:00:00Z'; },
    value => { value.ancillary.membershipRows++; },
    value => { value.ancillary.sheets = null; },
    value => { value.ancillary.sheets.reverse(); },
    value => { value.ancillary.sheets.push('_VPRO_Source'); },
    value => { value.ancillary.summary = null; },
    value => { value.ancillary.summary.units = null; },
    value => { value.ancillary.summary.units.push(value.ancillary.summary.units[0]); },
    value => { value.ancillary.summary.units[0].numPlots = Number.MAX_SAFE_INTEGER + 1; },
    value => { value.ancillary.summary.units[0].numPlots = 0; },
    value => { value.ancillary.summary.units[0].longName = '\ud800'; },
    value => { value.ancillary.summary.selectedPlotRows++; },
    value => { value.ancillary.summary.environmentRows = -1; },
    value => { value.ancillary.summary.environmentRows = '3'; },
    value => { value.ancillary.summary.environmentCountAvailable = false; },
    value => { value.ancillary.summary.versionStatus = 'unknown'; },
    value => { value.ancillary.summary.versionStatus = 'null'; },
    value => { value.scope = client.summaryVegetationWorkbookScope; },
  ];
  for (const [index, change] of changes.entries()) {
    const value = structuredClone(ancillaryWire.summaryEnvironment);
    change(value);
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(value, owner,
      ancillaryWire.summaryEnvironment.options, yieldNow), `ancillary case ${index}`);
  }
  const suOnly = structuredClone(ancillaryWire.memberships);
  suOnly.ancillary.summary = ancillaryWire.summary.ancillary.summary;
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(suOnly, owner, suOnly.options, yieldNow));
  const legacy = structuredClone(wire.vegetation);
  legacy.ancillary = undefined;
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(legacy, owner, legacy.options, yieldNow));
  for (const [version, versionStatus] of [['', 'literal'], ['Unknown', 'null'], ['Unknown', 'missing-definition'],
    ['Unknown', 'metadata-unavailable']]) {
    const value = structuredClone(ancillaryWire.summary);
    Object.assign(value.ancillary.summary, { version, versionStatus });
    value.ancillary.summary.units[0].longName = '';
    const result = await client.validateSummaryVegetationWorkbookReview(value, owner, value.options, yieldNow);
    assert.equal(result.ancillary.summary.version, version);
    assert.equal(result.ancillary.summary.units[0].longName, '');
    assert.equal(result.ancillary.summary.units[1].longName, null);
  }
  const empty = structuredClone(ancillaryWire.summary);
  Object.assign(empty.ancillary.summary, { environmentRows: 0, environmentCountAvailable: false });
  assert.ok(await client.validateSummaryVegetationWorkbookReview(empty, owner, empty.options, yieldNow));
});

test('ancillary drafts preserve independent errors and omission semantics, and Undo does not erase an irreversible receipt', async () => {
  const value = ancillaryWire.summaryEnvironment, session = sessionFor(value);
  session.edit('destination', 'invalid.txt');
  session.includeSummary(false);
  session.includeAncillary(false);
  assert.match(session.view().draftError, /Destination/);
  assert.equal(session.held(), true);
  assert.equal(Object.hasOwn(session.options(value.options), 'ancillary'), false);
  assert.throws(() => session.includeSummary(true), /Select source worksheets/);
  session.includeAncillary(true);
  session.includeSummary(true);
  session.edit('destination', destination);
  await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
  let calls = 0;
  await session.publish(value.options, request => {
    calls++;
    assert.deepEqual(plain(request), { options: value.options, approvalHash: value.approvalHash,
      destination, createdAt: value.ancillary.createdAt });
    return Promise.resolve(outcome(value));
  });
  assert.equal(calls, 1);
  assert.throws(() => session.includeAncillary(false), /acknowledge/);
  session.acknowledge();
  session.undoDraft();
  assert.equal(session.view().includeAncillary, false);
  assert.equal(session.view().createSummary, false);
  assert.equal(session.view().publication.outcome.sha256, value.workbookSHA256);
  assert.equal(Object.hasOwn(session.options(value.options), 'ancillary'), false);
});

test('ancillary validation yields through all metadata rows and cancellation cannot install the old result', { timeout: 10000 }, async () => {
  const value = structuredClone(ancillaryWire.summary);
  value.ancillary.summary.units = Array.from({ length: 600 }, (_, i) => ({
    code: `Unit${i}`, numPlots: 1, longName: i % 2 ? '' : null,
  }));
  let metadataStarted = false, heldAtMetadata = false;
  Object.defineProperty(value.ancillary.summary.units[0], 'code', {
    enumerable: true, get() { metadataStarted = true; return 'Unit0'; },
  });
  const session = sessionFor(value), held = deferred();
  let yields = 0;
  const showing = session.review(value.options, { review: () => Promise.resolve(value),
    validationYield: () => {
      yields++;
      if (metadataStarted && !heldAtMetadata) { heldAtMetadata = true; return held.promise; }
      return Promise.resolve();
    } });
  while (!heldAtMetadata) await new Promise(resolve => setTimeout(resolve, 0));
  session.cancel();
  assert.equal(session.held(), true);
  held.resolve();
  assert.equal(await showing, false);
  assert.equal(session.view().review, null);
  assert.equal(session.held(), false);
  let count = 0;
  const result = await client.validateSummaryVegetationWorkbookReview(value, owner, value.options,
    () => { count++; return Promise.resolve(); });
  assert.equal(result.ancillary.summary.units.length, 600);
  assert.ok(count >= 6, `complete validation must yield, observed ${count}`);
});

test('compiled ancillary panel is separately gated, captures checked DOM values and retains exact timestamp publication', async () => {
  const value = ancillaryWire.summaryEnvironment;
  for (const enabled of [false, true]) {
    const session = sessionFor(wire.environment);
    let reads = 0, writes = 0;
    const pane = mountPanel(session, {
      GetReview(contextId, json) {
        reads++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), enabled ? value.options : wire.environment.options);
        return Promise.resolve(enabled ? value : wire.environment);
      },
      ExportReviewed(contextId, json) {
        writes++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash,
          destination, createdAt: value.ancillary.createdAt });
        return Promise.resolve(outcome(value));
      },
    }, false, value.options, enabled);
    pane.selectAncillary('sheets', { currentTarget: { checked: true } });
    assert.equal(session.view().includeAncillary, enabled);
    if (enabled) {
      pane.selectAncillary('summary', { currentTarget: { checked: true } });
      assert.equal(session.view().createSummary, true);
    }
    await pane.review();
    assert.equal(reads, 1);
    if (enabled) {
      pane.edit('destination', destination);
      await pane.publish();
      assert.equal(writes, 1);
      assert.equal(pane.notifications.at(-1), true);
      pane.destroy();
      const remounted = mountPanel(session, {}, false, value.options, enabled);
      assert.equal(remounted.view().publication.blocked, true);
      assert.equal(remounted.view().publication.outcome.sha256, value.workbookSHA256);
      remounted.acknowledge();
      assert.equal(remounted.view().publication.outcome.sha256, value.workbookSHA256);
      remounted.destroy();
    } else pane.destroy();
  }
});

test('actual consumed server rendering bounds summary DOM to fifty units and distinguishes empty from NULL names', async () => {
  const value = structuredClone(ancillaryWire.summary);
  value.ancillary.summary.units = Array.from({ length: 101 }, (_, i) => ({
    code: `Unit${String(i).padStart(3, '0')}`, numPlots: 1, longName: i % 2 ? '' : null,
  }));
  const session = sessionFor(value);
  await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
  let subscribed = 0;
  const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
    'SummaryVegetationWorkbookPanel.svelte', {
      svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
      '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
      './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
      './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => {
        subscribed++; return session;
      } },
      './summaryVegetation': vegetation,
    }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true', VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK: 'true' });
  const html = require('svelte/server').render(component, { props: {
    owner, calculation: value.options, disabled: false, onBusyChange() {},
  } }).body;
  assert.equal(subscribed, 1);
  assert.match(html, /101 units; page 1 of 3/);
  assert.match(html, /Unit049/);
  assert.doesNotMatch(html, /Unit050/);
  assert.equal((html.match(/<th\b[^>]*\bscope="row"[^>]*>/g) || []).length, 50);
  assert.match(html, /\(NULL\)/);
  assert.match(html, /\(empty\)/);
  assert.match(html, /Date|Reviewed creation time/);
});

test('strict Go effective-reference wire accepts all modes and actual proxies without widening ordinary previews', async () => {
  for (const value of Object.values(referenceWire)) {
    const source = proxy(structuredClone(value));
    assert.throws(() => structuredClone(source));
    const result = await client.validateSummaryVegetationWorkbookReview(source, owner, value.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(Object.isFrozen(result.preview.references), true);
    assert.equal(Object.isFrozen(result.options.references), true);
    assert.equal(Object.isFrozen(result.preview.rows[0].witnesses[0].referenceOrigins), true);
    source.preview.references.masterRows++;
    assert.deepEqual(plain(result), value);
    assert.throws(() => vegetation.validateSummaryVegetation(value.preview, owner, 1, 6));
    if (value.environment) {
      const { average, display, showClass } = value.options;
      await assert.rejects(environmental.validateSummaryEnvironmentalLifeformOptions(
        value.environment, owner, { average, display, showClass }, yieldNow));
    }
  }
  const guideAttribute = referenceWire.guide.preview.rows.find(row => row.species.text === 'LF1');
  assert.equal(guideAttribute.commonName.storage, 'null');
  assert.deepEqual(guideAttribute.witnesses[0].referenceOrigins,
    [{ table: 'USysSppAttributeDescription', rowId: '1' }, { table: 'USysSppAttributeDescription', rowId: '2' }]);
  const personal = referenceWire.guide.preview.rows.find(row => row.commonName.text === 'Personal');
  assert.equal(personal.witnesses[0].referenceOrigins[0].table, 'USysUserSpp');
});

test('reference validator rejects missing/extra authority, malformed counts, foreign mode, common cells and origins', async () => {
  const changes = [
    value => { delete value.options.references; },
    value => { value.options.references = null; },
    value => { value.options.references.extra = true; },
    value => { value.options.references.showCommon = 1; },
    value => { value.options.references.useGuideNames = null; },
    value => { value.scope = client.summaryVegetationWorkbookScope; },
    value => { delete value.preview.references; },
    value => { value.preview.references = null; },
    value => { value.preview.references.extra = 0; },
    value => { value.preview.references.masterRows = -1; },
    value => { value.preview.references.personalRows = Number.MAX_SAFE_INTEGER + 1; },
    value => { value.preview.references.descriptionRows = '4'; },
    value => { value.preview.references.removedMasterRows = 15; },
    value => { value.preview.references.derivedRows = 5; },
    value => { value.preview.references.unionRows = 18; },
    value => { value.preview.references.showCommon = false; },
    value => { value.preview.references.useGuideNames = false; },
    value => { delete value.preview.rows[0].commonName; },
    value => { value.preview.rows[0].commonName = null; },
    value => { value.preview.rows[0].commonName.text = '\ud800'; },
    value => { value.preview.rows[0].commonName.text = 'x'.repeat(256); },
    value => { value.preview.ordering = 'literal-null-first-layer-species'; },
    value => { value.preview.source = 'literal-normal-su-layer'; },
    value => { value.preview.rows[0].witnesses[0].referenceOrigins[0].table = 'InventedMaster'; },
    value => { value.environment.vegetation.references.unionRows--; },
  ];
  for (const [index, change] of changes.entries()) {
    const value = structuredClone(referenceWire.guideSummary);
    change(value);
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(
      value, owner, referenceWire.guideSummary.options, yieldNow), `reference case ${index}`);
  }
  const attributes = structuredClone(referenceWire.attributes);
  attributes.preview.rows[0].commonName = { storage: 'null', text: null };
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(attributes, owner, attributes.options, yieldNow));
});

test('reference drafts preserve independent validation and receipts, guide forces common, and omitted mode stays omitted', async () => {
  const value = referenceWire.guideSummary, session = sessionFor(value);
  session.edit('title', '\ud800');
  session.selectReferences('include', false);
  assert.match(session.view().draftError, /Title/);
  assert.equal(session.held(), true);
  assert.throws(() => session.options(value.options), /inconsistent options/);
  session.edit('title', value.options.title);
  assert.equal(Object.hasOwn(session.options({ ...value.options, title: 'ignored' }), 'references'), false);
  assert.throws(() => session.selectReferences('guide', false), /Select effective/);
  session.selectReferences('include', true);
  assert.throws(() => session.selectReferences('common', false), /clear guide/);
  session.selectReferences('guide', false);
  session.selectReferences('common', false);
  session.selectReferences('guide', true);
  assert.equal(session.view().showCommon, true);
  session.edit('title', value.options.title);
  session.edit('destination', destination);
  assert.equal(await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow }), true);
  let calls = 0;
  await session.publish(value.options, request => {
    calls++;
    assert.deepEqual(plain(request), { options: value.options, approvalHash: value.approvalHash,
      destination, createdAt: value.ancillary.createdAt });
    return Promise.resolve(outcome(value));
  });
  assert.equal(calls, 1);
  assert.throws(() => session.selectReferences('include', false), /acknowledge/);
  session.acknowledge();
  session.undoDraft();
  assert.equal(session.view().includeReferences, false);
  assert.equal(session.view().publication.outcome.sha256, value.workbookSHA256);
});

test('effective-reference validation yields, cancellation retains the barrier and cannot install a stale result', { timeout: 10000 }, async () => {
  const value = referenceWire.guideSummary, session = sessionFor(value);
  const entered = deferred(), release = deferred();
  const reviewing = session.review(value.options, {
    review: () => Promise.resolve(proxy(structuredClone(value))),
    validationYield: async () => { entered.resolve(); await release.promise; },
  });
  await entered.promise;
  session.cancel();
  assert.equal(session.held(), true);
  assert.equal(session.view().review, null);
  release.resolve();
  assert.equal(await reviewing, false);
  assert.equal(session.held(), false);
  assert.equal(session.view().review, null);
  assert.match(session.view().error, /cancelled/);
});

test('compiled reference controls obey their independent gate and retain actual publication on remount', async () => {
  const value = referenceWire.guideCommon;
  for (const enabled of [false, true]) {
    const session = sessionFor(wire.vegetation);
    let reads = 0, writes = 0;
    const pane = mountPanel(session, {
      GetReview(contextId, json) {
        reads++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), enabled ? value.options : wire.vegetation.options);
        return Promise.resolve(proxy(structuredClone(enabled ? value : wire.vegetation)));
      },
      ExportReviewed(contextId, json) {
        writes++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash, destination });
        return Promise.resolve(outcome(value));
      },
    }, false, wire.vegetation.options, false, enabled);
    pane.selectReferences('include', { currentTarget: { checked: true } });
    pane.selectReferences('guide', { currentTarget: { checked: true } });
    assert.equal(session.view().includeReferences, enabled);
    assert.equal(session.view().showCommon, enabled);
    assert.equal(session.view().useGuideNames, enabled);
    await pane.review();
    assert.equal(reads, 1);
    if (enabled) {
      pane.edit('destination', destination);
      await pane.publish();
      assert.equal(writes, 1);
      pane.destroy();
      const remount = mountPanel(session, {}, false, value.options, false, enabled);
      assert.equal(remount.view().publication.blocked, true);
      remount.acknowledge();
      remount.undo();
      assert.equal(remount.view().publication.outcome.sha256, value.workbookSHA256);
      remount.destroy();
    } else pane.destroy();
  }
});

test('consumed reference rendering bounds name groups to fifty with labelled controls and visible NULL/empty distinction', async () => {
  const value = structuredClone(referenceWire.common), seed = value.preview.rows[0];
  value.preview.rows = Array.from({ length: 101 }, (_, i) => ({
    ...structuredClone(seed), species: { ...seed.species, text: `Name${String(i).padStart(3, '0')}` },
    commonName: { ...seed.commonName, storage: i % 2 ? 'text' : 'null', text: i % 2 ? '' : null },
  }));
  const session = sessionFor(value);
  assert.equal(await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow }), true);
  let subscribed = 0;
  const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
    'SummaryVegetationWorkbookPanel.svelte', {
      svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
      '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
      './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
      './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => { subscribed++; return session; } },
      './summaryVegetation': vegetation,
    }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true', VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: 'true' });
  const html = require('svelte/server').render(component, { props: {
    owner, calculation: value.options, disabled: false, onBusyChange() {},
  } }).body;
  assert.equal(subscribed, 1);
  assert.match(html, /101 name groups; page 1 of\s+3/);
  assert.match(html, /Name049/);
  assert.doesNotMatch(html, /Name050/);
  assert.equal((html.match(/<th\b[^>]*\bscope="row"[^>]*>/g) || []).length, 50);
  assert.match(html, /\(NULL\)/);
  assert.match(html, /\(empty\)/);
  assert.match(html, /Project effective source references and attribute descriptions/);
  assert.match(html, /Include common names in grouping and the workbook/);
  assert.match(html, /Use master CombinedEnglishName/);
});

test('eight strict Go grouped wires detach actual proxies and preserve typed metadata without widening ordinary consumers', async () => {
  assert.equal(Object.keys(groupedWire).length, 8);
  for (const value of Object.values(groupedWire)) {
    const source = proxy(structuredClone(value));
    assert.throws(() => structuredClone(source));
    const result = await client.validateSummaryVegetationWorkbookReview(source, owner, value.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(Object.isFrozen(result.preview.groupField), true);
    assert.equal(Object.isFrozen(result.preview.rows[0].order), true);
    source.preview.groupField.name = 'caller';
    assert.deepEqual(plain(result), value);
    assert.throws(() => vegetation.validateSummaryVegetation(value.preview, owner,
      value.options.average, value.options.display, value.options.showClass));
    if (value.environment) {
      const { groupAndOrder, ...ordinary } = value.options;
      await assert.rejects(environmental.validateSummaryEnvironmentalLifeformOptions(
        value.environment, owner, ordinary, yieldNow));
    }
  }
  assert.deepEqual(groupedWire.lifeform.preview.groupField, { name: 'LifeForm', daoType: 10, size: 3 });
  assert.equal(groupedWire.lifeform.preview.rows[0].layer.storage, 'null');
  assert.equal(groupedWire.lifeform.preview.rows.find(row => row.species.text === 'Lifeform 1').layer.text, '01');
  assert.equal(groupedWire.none.preview.rows[0].layer.storage, 'null');
  assert.deepEqual(groupedWire.strata.preview.orderField, { name: 'Order', daoType: 7, size: 8 });
  assert.equal(groupedWire.strata.preview.rows.find(row => row.species.text === 'Lifeform 1').order.real, 1.56);
  assert.deepEqual(groupedWire.strataClass.preview.orderField, { name: 'Order', daoType: 10, size: 0 });
});

test('grouped authority rejects omitted, null, legacy, mistyped and cross-mode metadata without repairing it', async () => {
  const changes = [
    value => { delete value.options.groupAndOrder; },
    value => { value.options.groupAndOrder = null; },
    value => { value.options.groupAndOrder = 11; },
    value => { value.options.groupAndOrder = '12'; },
    value => { value.options.groupAndOrder = 13; },
    value => { value.options.GroupAndOrder = 12; },
    value => { value.options.references = { showCommon: false, useGuideNames: false }; },
    value => { value.options.ancillary = { createSummary: false }; },
    value => { value.scope = client.summaryVegetationWorkbookScope; },
    value => { delete value.preview.groupAndOrder; },
    value => { value.preview.groupField.name = 'Strata'; },
    value => { value.preview.groupField.size = 3; },
    value => { value.preview.orderField.daoType = 10; },
    value => { value.preview.rows[0].order.real = '1.56'; },
    value => { value.preview.rows[0].order.real = Infinity; },
    value => { value.preview.ordering = 'literal-null-first-layer-species'; },
    value => { value.environment.vegetation.groupAndOrder = 14; },
    value => { value.preview.columns.pop(); },
  ];
  for (const [index, change] of changes.entries()) {
    const value = structuredClone(groupedWire.strataEnvironment);
    change(value);
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(
      value, owner, value.options, yieldNow), `grouped case ${index}`);
  }
  const value = structuredClone(groupedWire.none);
  value.preview.rows[0].layer = { storage: 'text', text: '' };
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(value, owner, value.options, yieldNow));
  await assert.rejects(client.validateSummaryVegetationWorkbookReview(groupedWire.lifeform,
    owner, groupedWire.strata.options, yieldNow));
});

test('group changes invalidate reviewed publication while receipts and invalid drafts keep their independent barriers', async () => {
  const value = groupedWire.strata, session = sessionFor(value);
  assert.equal(client.sameSummaryWorkbookOptions(value.options, { ...value.options, groupAndOrder: 13 }), false);
  assert.equal(client.sameSummaryWorkbookOptions(value.options, wire.vegetation.options), false);
  await session.review(value.options, { review: () => Promise.resolve(value), validationYield: yieldNow });
  session.edit('destination', destination);
  let writes = 0;
  await assert.rejects(session.publish({ ...value.options, groupAndOrder: 14 }, () => { writes++; }), /current explicit/);
  assert.equal(writes, 0);
  await session.publish(value.options, request => {
    writes++;
    assert.deepEqual(plain(request), { options: value.options, approvalHash: value.approvalHash, destination });
    return Promise.resolve(outcome(value));
  });
  assert.equal(writes, 1);
  session.acknowledge();
  session.edit('title', '\ud800');
  assert.equal(session.held(), true);
  assert.match(session.view().draftError, /Title/);
  session.undoDraft();
  assert.equal(session.held(), false);
  assert.equal(session.view().publication.outcome.sha256, value.workbookSHA256);
});

test('compiled grouped panel gate refuses dispatch, preserves incompatible selections and publishes only explicit authority', async () => {
  const value = groupedWire.lifeform;
  for (const enabled of [false, true]) {
    const session = sessionFor(value);
    let reads = 0, writes = 0;
    const services = {
      GetReview(contextId, json) {
        reads++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), value.options);
        return Promise.resolve(proxy(structuredClone(value)));
      },
      ExportReviewed(contextId, json) {
        writes++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash, destination });
        return Promise.resolve(outcome(value));
      },
    };
    const pane = mountPanel(session, services, false, value.options, true, true, enabled);
    await pane.review();
    assert.equal(reads, enabled ? 1 : 0);
    if (enabled) {
      pane.selectAncillary('sheets', { currentTarget: { checked: true } });
      await pane.review();
      assert.equal(reads, 1);
      assert.equal(pane.view().includeAncillary, true);
      pane.selectAncillary('sheets', { currentTarget: { checked: false } });
      pane.selectReferences('include', { currentTarget: { checked: true } });
      pane.selectReferences('common', { currentTarget: { checked: true } });
      await pane.review();
      assert.equal(reads, 1);
      assert.equal(pane.view().includeReferences, true);
      pane.selectReferences('include', { currentTarget: { checked: false } });
      await pane.review();
      pane.edit('destination', destination);
      await pane.publish();
      assert.equal(writes, 1);
    }
    pane.destroy();
    const remount = mountPanel(session, services, false, value.options, true, true, enabled);
    await remount.publish();
    assert.equal(writes, enabled ? 1 : 0);
    if (enabled) assert.equal(remount.view().publication.outcome.sha256, value.workbookSHA256);
    remount.destroy();
  }
});

test('grouped validation cancellation waits for yield and cannot populate a remounted panel', { timeout: 10000 }, async () => {
  const value = groupedWire.lifeformEnvironment, session = sessionFor(value);
  const entered = deferred(), release = deferred();
  const pending = session.review(value.options, {
    review: () => Promise.resolve(proxy(structuredClone(value))),
    validationYield: async () => { entered.resolve(); await release.promise; },
  });
  await entered.promise;
  session.cancel();
  const remount = mountPanel(session, {}, false, value.options, false, false, true);
  assert.equal(session.held(), true);
  assert.equal(remount.view().review, null);
  release.resolve();
  assert.equal(await pending, false);
  assert.equal(session.held(), false);
  assert.equal(remount.view().review, null);
  remount.destroy();
});

test('consumed outer Summary rendering forwards only flag-authorized Group By selections to the existing workbook panel', () => {
  const reportSession = vegetation.summaryVegetationSession(owner);
  for (const enabled of [false, true]) for (const group of [11, 12, 13, 14]) {
    reportSession.edit(1, 6, 1, group);
    const calls = [];
    const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8'),
      'SummaryVegetationReport.svelte', {
        svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
        './summaryVegetation': vegetation,
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: {} },
        './SummaryVegetationWorkbookPanel.svelte': { default: (_payload, props) => { calls.push(props); } },
      }, { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
        VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: enabled ? 'true' : 'false' });
    const html = require('svelte/server').render(component, { props: { ...owner, onBusyChange() {} } }).body;
    assert.match(html, /Group by/);
    assert.equal(calls.length, 1);
    assert.deepEqual(plain(calls[0].calculation), { average: 1, display: 6, showClass: 1,
      ...(enabled && group !== 11 ? { groupAndOrder: group } : {}) });
    assert.equal(calls[0].disabled, !enabled && group !== 11);
  }
});

test('consumed grouped panel renders accurate grouping and explicit unavailable feedback without clearing retained modes', () => {
  for (const [enabled, mode] of [[false, null], [true, null], [true, 'references'], [true, 'ancillary']]) {
    const value = groupedWire.lifeform, session = sessionFor(value);
    if (mode === 'references') {
      session.selectReferences('include', true);
      session.selectReferences('common', true);
    }
    if (mode === 'ancillary') session.includeAncillary(true);
    let subscriptions = 0;
    const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
      'SummaryVegetationWorkbookPanel.svelte', {
        svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
        './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
        './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => {
          subscriptions++; return session;
        } },
        './summaryVegetation': vegetation,
      }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
        VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: enabled ? 'true' : 'false',
        VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: 'true', VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK: 'true' });
    const html = require('svelte/server').render(component, { props: {
      owner, calculation: value.options, disabled: false, onBusyChange() {},
    } }).body;
    assert.equal(subscriptions, 1);
    assert.match(html, /Workbook grouping: Lifeform/);
    assert.equal(html.includes('No options were changed automatically.'), !enabled || mode !== null);
    const button = html.match(/<button[^>]*data-summary-workbook-review[^>]*>/);
    assert.ok(button);
    assert.equal(/\bdisabled\b/.test(button[0]), !enabled || mode !== null);
    assert.equal(session.view().includeReferences, mode === 'references');
    assert.equal(session.view().includeAncillary, mode === 'ancillary');
  }
});

test('eight strict Go Strata reference vectors detach actual proxies and require both explicit authorities', async () => {
  assert.equal(Object.keys(strataReferenceWire).length, 8);
  for (const value of Object.values(strataReferenceWire)) {
    const source = proxy(structuredClone(value));
    assert.throws(() => structuredClone(source));
    const result = await client.validateSummaryVegetationWorkbookReview(source, owner, value.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(Object.isFrozen(result.preview.references), true);
    assert.equal(Object.isFrozen(result.preview.groupField), true);
    assert.equal(Object.isFrozen(result.options.references), true);
    assert.equal(Object.isFrozen(result.preview.rows[0].witnesses[0].referenceOrigins), true);
    source.preview.references.masterRows++;
    source.preview.groupField.name = 'caller';
    assert.deepEqual(plain(result), value);
    assert.throws(() => vegetation.validateSummaryVegetation(value.preview, owner,
      value.options.average, value.options.display, value.options.showClass));
    for (const field of ['references', 'groupAndOrder']) {
      const options = { ...value.options };
      delete options[field];
      await assert.rejects(client.validateSummaryVegetationWorkbookReview(value, owner, options, yieldNow));
    }
    if (value.environment) {
      const { groupAndOrder, ...ordinary } = value.options;
      await assert.rejects(environmental.validateSummaryEnvironmentalLifeformOptions(
        value.environment, owner, ordinary, yieldNow));
    }
  }
  const common = strataReferenceWire.common.preview.rows.filter(row => row.species.text === 'Lifeform 1');
  assert.equal(common.length, 2);
  assert.ok(common.every(row => row.layer.text === 'A' && row.order.real === 1.56 && row.covers[0].text === '1.56'));
  assert.equal(strataReferenceWire.attributes.preview.rows.find(row => row.species.text === 'Lifeform 1').order.real, 3.13);
  const guideAttribute = strataReferenceWire.guide.preview.rows.find(row => row.species.text === 'LF1');
  assert.equal(guideAttribute.commonName.storage, 'null');
  assert.deepEqual(guideAttribute.witnesses[0].referenceOrigins,
    [{ table: 'USysSppAttributeDescription', rowId: '1' }, { table: 'USysSppAttributeDescription', rowId: '2' }]);
});

test('Strata reference validator rejects cross-mode authority, numeric/text confusion and conflicting owned frames', async () => {
  const changes = [
    value => { value.options.groupAndOrder = 13; },
    value => { value.options.groupAndOrder = 14; },
    value => { value.options.references = null; },
    value => { value.options.references.showCommon = false; },
    value => { value.options.ancillary = { createSummary: false }; },
    value => { value.options.display = 2; },
    value => { value.scope = client.summaryVegetationGroupedWorkbookScope; },
    value => { value.scope = client.summaryVegetationEffectiveReferencesScope; },
    value => { value.preview.source = 'literal-normal-su-grouped'; },
    value => { value.preview.ordering = 'literal-null-first-group-species'; },
    value => { delete value.preview.groupField; },
    value => { value.preview.groupField.name = 'Strata'; },
    value => { value.preview.groupField.size = 3; },
    value => { value.preview.orderField.daoType = 10; },
    value => { value.preview.references.useGuideNames = true; },
    value => { value.preview.references.removedMasterRows = -1; },
    value => { delete value.preview.rows[0].commonName; },
    value => { value.preview.rows[0].commonName.text = '\ud800'; },
    value => { value.preview.rows[0].order = { storage: 'text', text: '1.56' }; },
    value => { value.preview.rows[0].witnesses[0].referenceOrigins[0].table = 'Unowned'; },
    value => { value.environment.vegetation.references.unionRows--; },
    value => { value.environment.vegetation.groupAndOrder = 13; },
  ];
  for (const [index, change] of changes.entries()) {
    const value = structuredClone(strataReferenceWire.environment);
    change(value);
    await assert.rejects(client.validateSummaryVegetationWorkbookReview(
      value, owner, value.options, yieldNow), `Strata reference case ${index}`);
  }
});

test('compiled Strata reference panel requires both existing flags and retains incompatible grouping without dispatch', async () => {
  const value = strataReferenceWire.common;
  for (const grouped of [false, true]) for (const references of [false, true]) {
    const session = sessionFor(value);
    let reads = 0, writes = 0;
    const services = {
      GetReview(contextId, json) {
        reads++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), value.options);
        return Promise.resolve(proxy(structuredClone(value)));
      },
      ExportReviewed(contextId, json) {
        writes++;
        assert.equal(contextId, owner.contextId);
        assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash, destination });
        return Promise.resolve(outcome(value));
      },
    };
    const pane = mountPanel(session, services, false, value.options, true, references, grouped);
    await pane.review();
    assert.equal(reads, grouped && references ? 1 : 0);
    assert.equal(pane.view().includeReferences, true);
    if (grouped && references) {
      pane.selectAncillary('sheets', { currentTarget: { checked: true } });
      await pane.review();
      assert.equal(reads, 1);
      assert.equal(pane.view().includeAncillary, true);
      pane.selectAncillary('sheets', { currentTarget: { checked: false } });
    }
    pane.destroy();
    const incompatible = mountPanel(session, services, false, { ...value.options, groupAndOrder: 13 }, true, references, grouped);
    await incompatible.review();
    await incompatible.publish();
    assert.equal(reads, grouped && references ? 1 : 0);
    assert.equal(writes, 0);
    assert.equal(incompatible.view().includeReferences, true);
    incompatible.destroy();
    const returned = mountPanel(session, services, false, value.options, true, references, grouped);
    if (grouped && references) {
      await returned.review();
      returned.edit('destination', destination);
      await returned.publish();
      assert.equal(writes, 1);
      returned.acknowledge();
      assert.equal(returned.view().publication.outcome.sha256, value.workbookSHA256);
    } else {
      await returned.publish();
      assert.equal(writes, 0);
    }
    returned.destroy();
  }
});

test('Strata reference held validation remains owned through Cancel and cannot populate a remounted panel', { timeout: 10000 }, async () => {
  const value = strataReferenceWire.environment, session = sessionFor(value);
  const entered = deferred(), release = deferred();
  const pending = session.review(value.options, {
    review: () => Promise.resolve(proxy(structuredClone(value))),
    validationYield: async () => { entered.resolve(); await release.promise; },
  });
  await entered.promise;
  session.cancel();
  const remount = mountPanel(session, {}, false, value.options, false, true, true);
  assert.equal(session.held(), true);
  assert.equal(remount.view().review, null);
  release.resolve();
  assert.equal(await pending, false);
  assert.equal(session.held(), false);
  assert.equal(remount.view().review, null);
  assert.equal(remount.view().includeReferences, true);
  remount.destroy();
});

test('consumed reference tables retain original group captions and display typed numeric Order rather than NULL', async () => {
  for (const value of [referenceWire.common, strataReferenceWire.attributes, groupedAttributeWire.lifeform, groupedAttributeWire.none]) {
    const session = sessionFor(value);
    assert.equal(await session.review(value.options, { review: () => Promise.resolve(proxy(structuredClone(value))),
      validationYield: yieldNow }), true);
    let subscribed = 0;
    const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
      'SummaryVegetationWorkbookPanel.svelte', {
        svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
        './summaryVegetation': vegetation,
        './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
        './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => {
          subscribed++; return session;
        } },
      }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true', VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: 'true',
        VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: 'true' });
    const html = require('svelte/server').render(component, { props: {
      owner, calculation: value.options, disabled: false, onBusyChange() {},
    } }).body;
    assert.equal(subscribed, 1);
    const table = html.replace(/<!--[\s\S]*?-->/g, '').match(/<table\b[^>]*aria-label="Reviewed effective reference groups"[^>]*>([\s\S]*?)<\/table>/);
    assert.ok(table);
    assert.match(table[1], new RegExp(`<th\\b[^>]*scope="col"[^>]*>${value.preview.groupField?.name ?? 'Layer'}</th>`));
    const body = table[1].match(/<tbody\b[^>]*>([\s\S]*?)<\/tbody>/);
    assert.ok(body);
    const rows = [...body[1].matchAll(/<tr\b[^>]*>([\s\S]*?)<\/tr>/g)];
    const expected = value.preview.rows.slice(0, 50);
    assert.ok(expected.length > 0);
    assert.equal(rows.length, expected.length);
    for (const [index, row] of expected.entries()) {
      const cells = [...rows[index][1].matchAll(/<td\b[^>]*>([\s\S]*?)<\/td>/g)];
      assert.ok(cells.length >= 2);
      assert.equal(cells.at(-1)[1].trim(), row.order.storage === 'real' ? String(row.order.real) :
        row.order.text === null ? '(NULL)' : row.order.text === '' ? '(empty)' : row.order.text);
    }
  }
});

test('eight strict Go grouped attribute vectors detach proxies without synthesizing First common names', async () => {
  assert.equal(Object.keys(groupedAttributeWire).length, 8);
  for (const value of Object.values(groupedAttributeWire)) {
    const raw = proxy(structuredClone(value));
    const result = await client.validateSummaryVegetationWorkbookReview(raw, owner, raw.options, yieldNow);
    assert.deepEqual(plain(result), value);
    assert.equal(result.scope, client.summaryVegetationGroupedAttributesScope);
    assert.equal(result.preview.references.showCommon, false);
    assert.equal(result.preview.references.useGuideNames, false);
    assert.ok(result.preview.rows.length > 0);
    assert.ok(result.preview.rows.some(row => row.layer.storage === 'null'));
    assert.ok(result.preview.rows.every(row => !Object.hasOwn(row, 'commonName')));
    assert.equal(Object.isFrozen(result.options.references), true);
    assert.equal(Object.isFrozen(result.preview.rows[0].witnesses), true);
    raw.preview.rows[0].witnesses[0].rawVegRowIds[0] = 'foreign';
    assert.deepEqual(plain(result), value);
  }
  for (let group = 0; group <= 15; group++) {
    const allowed = { 11: [true, true], 12: [true, true], 13: [true, false], 14: [true, false] };
    for (const [index, common] of [false, true].entries())
      assert.equal(vegetation.summaryVegetationReferenceGroupingAvailable(group, common), allowed[group]?.[index] ?? false);
  }
});

test('grouped attribute reviews reject name selection, scope confusion and malformed grouping authority', async () => {
  for (const base of [groupedAttributeWire.lifeform, groupedAttributeWire.none]) {
    for (const mutate of [
      value => { value.options.references.showCommon = true; },
      value => { value.options.references.useGuideNames = true; },
      value => { value.preview.references.showCommon = true; },
      value => { value.preview.references.useGuideNames = true; },
      value => { value.options.ancillary = { createSummary: false }; },
      value => { delete value.options.references; },
      value => { delete value.preview.references; },
      value => { value.options.groupAndOrder = 12; },
      value => { value.scope = client.summaryVegetationStrataReferencesScope; },
      value => { value.scope = client.summaryVegetationGroupedWorkbookScope; },
      value => { value.preview.groupField.daoType = 7; },
      value => { value.preview.rows[0].commonName = { storage: 'null', text: null, integer: null, real: null, blobHex: null }; },
      value => { value.preview.rows[0].order = { storage: 'text', text: '1', integer: null, real: null, blobHex: null }; },
    ]) {
      const value = structuredClone(base);
      mutate(value);
      await assert.rejects(client.validateSummaryVegetationWorkbookReview(value, owner, value.options, yieldNow));
    }
  }
});

test('compiled grouped attribute panels require both gates and retain unsupported common and guide drafts across remounts', async () => {
  for (const value of [groupedAttributeWire.lifeform, groupedAttributeWire.none]) {
    for (const grouped of [false, true]) for (const references of [false, true]) {
      const session = sessionFor(value);
      let reads = 0, writes = 0;
      const services = {
        GetReview(contextId, json) {
          reads++;
          assert.equal(contextId, owner.contextId);
          assert.deepEqual(JSON.parse(json), value.options);
          return Promise.resolve(proxy(structuredClone(value)));
        },
        ExportReviewed(contextId, json) {
          writes++;
          assert.equal(contextId, owner.contextId);
          assert.deepEqual(JSON.parse(json), { options: value.options, approvalHash: value.approvalHash, destination });
          return Promise.resolve(outcome(value));
        },
      };
      const pane = mountPanel(session, services, false, value.options, true, references, grouped);
      await pane.review();
      assert.equal(reads, grouped && references ? 1 : 0);
      if (grouped && references) {
        pane.selectReferences('common', { currentTarget: { checked: true } });
        await pane.review();
        assert.equal(reads, 1);
      }
      pane.destroy();
      const remount = mountPanel(session, services, false, value.options, true, references, grouped);
      await remount.review();
      await remount.publish();
      assert.equal(reads, grouped && references ? 1 : 0);
      assert.equal(writes, 0);
      if (grouped && references) {
        assert.equal(remount.view().showCommon, true);
        remount.selectReferences('common', { currentTarget: { checked: false } });
        remount.selectReferences('guide', { currentTarget: { checked: true } });
        await remount.review();
        assert.equal(reads, 1);
        assert.equal(remount.view().useGuideNames, true);
        remount.selectReferences('guide', { currentTarget: { checked: false } });
        remount.selectReferences('common', { currentTarget: { checked: false } });
        await remount.review();
        assert.equal(reads, 2);
        remount.edit('destination', destination);
        await remount.publish();
        assert.equal(writes, 1);
        remount.acknowledge();
        remount.edit('title', '\ud800');
        assert.equal(session.held(), true);
        remount.undo();
        assert.equal(session.held(), false);
        assert.equal(session.view().publication.outcome.sha256, value.workbookSHA256);
      }
      remount.destroy();
    }
  }
});

test('consumed grouped attribute rendering distinguishes supported references from retained unavailable names', () => {
  for (const grouped of [false, true]) for (const references of [false, true]) {
    let selected, subscribed = 0;
    const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
      'SummaryVegetationWorkbookPanel.svelte', {
        svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
        './summaryVegetation': vegetation,
        './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
        './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => {
          subscribed++; return selected;
        } },
      }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
        VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: grouped ? 'true' : 'false',
        VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: references ? 'true' : 'false' });
    for (const value of [groupedAttributeWire.lifeform, groupedAttributeWire.none]) {
      for (const mode of ['attributes', 'common', 'guide']) {
        selected = sessionFor(value);
        if (mode !== 'attributes') selected.selectReferences(mode, true);
        const html = require('svelte/server').render(component, { props: {
          owner, calculation: value.options, disabled: false, onBusyChange() {},
        } }).body;
        const button = html.match(/<button[^>]*data-summary-workbook-review[^>]*>/);
        assert.ok(button);
        assert.equal(/\bdisabled\b/.test(button[0]), !grouped || !references || mode !== 'attributes');
        assert.equal(selected.view().includeReferences, true);
        if (mode !== 'attributes') assert.match(html, /Lifeform\/None support attribute references with both names off/);
      }
    }
    assert.equal(subscribed, 6);
  }
});

test('consumed Strata reference panel rendering preserves independent gates and retained scope intent', () => {
  for (const value of [referenceWire.common, strataReferenceWire.common]) {
  for (const grouped of [false, true]) for (const references of [false, true]) {
    const session = sessionFor(value);
    let subscriptions = 0;
    const component = serverComponent(readFileSync(path.join(__dirname, 'SummaryVegetationWorkbookPanel.svelte'), 'utf8'),
      'SummaryVegetationWorkbookPanel.svelte', {
        svelte: require(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-server.js')),
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationWorkbookService: {} },
        './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
        './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => {
          subscriptions++; return session;
        } },
        './summaryVegetation': vegetation,
      }, { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
        VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK: grouped ? 'true' : 'false',
        VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES: references ? 'true' : 'false' });
    const html = require('svelte/server').render(component, { props: {
      owner, calculation: value.options, disabled: false, onBusyChange() {},
    } }).body;
    assert.equal(subscriptions, 1);
    const strata = value.options.groupAndOrder === 12;
    assert.match(html, strata ? /Workbook grouping: Strata/ : /Workbook grouping: Layer/);
    const button = html.match(/<button[^>]*data-summary-workbook-review[^>]*>/);
    assert.ok(button);
    assert.equal(/\bdisabled\b/.test(button[0]), strata && !grouped || !references);
    assert.equal(html.includes('Effective reference projection is disabled'), !references);
    assert.equal(session.view().includeReferences, true);
  }
  }
});
