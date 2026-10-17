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

function mountPanel(session, services, held = false, calculation = wire.environment.options, ancillaryEnabled = false) {
  const filename = 'SummaryVegetationWorkbookPanel.svelte';
  const source = readFileSync(path.join(__dirname, filename), 'utf8');
  assert.deepEqual(compiler.compile(source, { filename, generate: 'client' }).warnings, []);
  let destroy;
  const notifications = [];
  let script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1].replace('$props()', '__props');
  script += '\nexport const handlers = {review, publish, edit, undo, acknowledge, selectAncillary, view: () => viewState};';
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
    './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalLifeformEnabled: true },
    './summaryVegetationWorkbook': { ...client, summaryVegetationWorkbookSession: () => session },
  };
  vm.runInNewContext(code, { module, exports: module.exports, structuredClone, setTimeout,
    __props: { owner, calculation, disabled: held, onBusyChange: value => notifications.push(value) },
    __environment: { VITE_SUMMARY_VEGETATION_WORKBOOK: 'true',
      VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK: ancillaryEnabled ? 'true' : 'false' },
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
