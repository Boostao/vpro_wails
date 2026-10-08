const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-template.json'))),
});
const restoration = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': editor });
const names = loadTypeScript('reportUnitNames.ts', { './projectMetadataRestore': restoration });
const environment = loadTypeScript('longEnvironmentReport.ts', { './projectMetadataRestore': restoration, './reportUnitNames': names });
const report = loadTypeScript('longVegetationReport.ts', {
  './projectMetadataRestore': restoration, './reportUnitNames': names, './qualityEditor': quality, './longEnvironmentReport': environment,
});
const workbook = loadTypeScript('vegetationWorkbook.ts', {
  './longVegetationReport': report, './projectMetadataRestore': restoration,
  './lifeformSummary': loadTypeScript('lifeformSummary.ts'), './publicationSession': loadTypeScript('publicationSession.ts'),
});
const owner = { contextId: 'vegetation-workbook', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
const destination = 'C:\\output\\ exact.xlsx';
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
function fixture() {
  const settings = { title: ' Title ', grouping: 'layer', average: 'all-plots', order: 'species', constantSpeciesList: false,
    presenceGreaterThan: 0, meanCoverGreaterThan: 0, showEnglishName: false, showSpeciesCode: false, quality: null };
  return { preview: { contextId: owner.contextId, projectPath: owner.projectPath, suPath: owner.suPath, settings,
    report: { project: owner.project, su: owner.su, title: settings.title, quality: null, diagnostics: [],
      units: [{ code: cell('U:1'), longName: null, nameStatus: 'missing', nameCandidates: [], numPlots: 1, membershipIds: ['1'],
        rows: [{ layer: cell('Tree'), species: cell('SP'), englishName: cell(null), matchedName: cell(null),
          presence: 1, meanCover: 0, plots: [{ plotNumber: 'P1', cover: 0 }] }] }] } },
    options: { quickReport: false, spaceBetweenGroups: true, reportSummary: true },
    summary: { createdDate: '2026-10-06', environmentRows: 15, selectedPlotRows: 12,
      speciesVersion: cell('Unknown'), versionStatus: 'metadata-unavailable', versionDefinitions: { columns: null, rows: null } },
    scope: 'unlumped', createdDate: '2026-10-06', sheets: [{ unit: cell('U:1'), name: 'U-1' }], skippedUnits: [],
    approvalHash: 'a'.repeat(64), workbookSHA256: 'b'.repeat(64), bytes: 12345 };
}
const receipt = (status = 'published') => ({ status, requestedDestination: destination,
  path: status === 'not-published' ? '' : destination, sha256: status === 'not-published' ? '' : 'b'.repeat(64),
  errorMessage: status === 'published' ? '' : 'explicit collision/publication error' });
const source = () => readFileSync(path.join(__dirname, 'LongVegetationReport.svelte'), 'utf8');

test('strict unlumped review retains detached owned preview and nullable absent metadata without defaulting malformed DTOs', () => {
  const input = fixture(), value = workbook.validateVegetationWorkbookReview(input, owner);
  input.preview.settings.title = 'changed'; input.sheets[0].name = 'changed';
  assert.equal(value.preview.settings.title, ' Title '); assert.equal(value.sheets[0].name, 'U-1');
  assert.equal(Object.isFrozen(value), true); assert.equal(Object.isFrozen(value.preview.settings), true);
  for (const mutate of [
    x => x.scope = '', x => x.scope = 'lumped', x => x.scope = 'source4',
    x => x.extra = true, x => x.preview = null, x => x.preview.contextId = 'stale',
    x => x.preview.suPath = 'different', x => x.sheets = null, x => x.skippedUnits = null,
    x => x.sheets = [], x => x.sheets[0] = null, x => x.sheets[0].unit = cell('other'),
    x => x.sheets[0].name = 'U:1', x => x.options = null, x => x.options.quickReport = -1,
    x => x.options.extra = true, x => x.summary = null, x => x.summary.createdDate = '2026-10-07',
    x => x.summary.environmentRows = -1, x => x.summary.selectedPlotRows = 1.5,
    x => x.summary.speciesVersion = cell(null), x => x.summary.versionStatus = 'literal',
    x => x.summary.versionDefinitions = null, x => x.summary.versionDefinitions.columns = {},
    x => x.summary.versionDefinitions.rows = [null], x => x.createdDate = '2026-02-30',
    x => x.createdDate = '0099-12-31', x => x.createdDate = '2026-1-01', x => x.createdDate = '2026-10-06T00:00:00Z',
    x => x.approvalHash = 'A'.repeat(64), x => x.workbookSHA256 = '', x => x.bytes = 0,
    x => x.preview.report.units[0].rows = [], x => x.preview.report.units[0].rows[0].plots = [],
    x => x.skippedUnits = [{ unit: cell('U:1'), reason: 'source NumPlots < 1' }],
    x => x.skippedUnits = [{ unit: cell('other'), reason: 'invented' }],
  ]) { const invalid = fixture(); mutate(invalid); assert.throws(() => workbook.validateVegetationWorkbookReview(invalid, owner), undefined, String(mutate)); }
  const noSummary = fixture(); noSummary.options.reportSummary = false; noSummary.summary = null;
  assert.doesNotThrow(() => workbook.validateVegetationWorkbookReview(noSummary, owner));
  assert.throws(() => workbook.validateVegetationWorkbookReview(fixture(), { ...owner, su: 'USysSuTableDynamic' }));
});

test('AllSpecs version states preserve NULL/empty/raw physical definitions and reject duplicate/foreign/invented values', () => {
  for (const literal of [null, '', ' Version ']) {
    const input = fixture(), summary = input.summary;
    summary.speciesVersion = cell(literal); summary.versionStatus = literal === null ? 'null' : 'literal';
    summary.versionDefinitions = { columns: [{ name: 'table_name', declaredType: 'TEXT' }, { name: 'description', declaredType: 'TEXT' }],
      rows: [{ rowId: '9', cells: [cell('USysAllSpecs'), cell(literal)] }] };
    assert.equal(workbook.validateVegetationWorkbookReview(input, owner).summary.speciesVersion.text, literal);
    for (const mutate of [
      x => x.summary.versionDefinitions.rows.push(x.summary.versionDefinitions.rows[0]),
      x => x.summary.versionDefinitions.rows[0].cells[0] = cell('OtherSpecs'),
      x => x.summary.versionDefinitions.rows[0].cells = null,
      x => x.summary.versionDefinitions.rows[0].rowId = '09',
      x => x.summary.speciesVersion = cell('guessed'), x => x.summary.versionStatus = 'missing-definition',
      x => x.summary.versionDefinitions.columns[1].name = 'other',
    ]) { const invalid = structuredClone(input); mutate(invalid); assert.throws(() => workbook.validateVegetationWorkbookReview(invalid, owner)); }
  }
  const missing = fixture(); missing.summary.versionStatus = 'missing-definition';
  missing.summary.versionDefinitions.columns = [{ name: 'table_name', declaredType: 'TEXT' }, { name: 'description', declaredType: 'TEXT' }];
  assert.doesNotThrow(() => workbook.validateVegetationWorkbookReview(missing, owner));
});

test('worksheet identities preserve NULL versus empty text, source names and collision refusal', () => {
  for (const [unit, name] of [[null, 'Unassigned'], ['', 'NoName0'], ['U:1', 'U-1'], ['😀'.repeat(15) + 'AB', '😀'.repeat(15) + 'A']]) {
    const input = fixture(); input.preview.report.units[0].code = cell(unit);
    if (unit === null) { input.preview.report.units[0].longName = ''; input.preview.report.units[0].nameStatus = 'unassigned'; }
    input.sheets[0] = { unit: cell(unit), name };
    assert.doesNotThrow(() => workbook.validateVegetationWorkbookReview(input, owner));
  }
  for (const name of ['ReportSummary', '_VPRO_Source', "'name", "name'", '😀'.repeat(16)]) {
    const input = fixture(); input.preview.report.units[0].code = cell(name); input.sheets[0] = { unit: cell(name), name };
    assert.throws(() => workbook.validateVegetationWorkbookReview(input, owner));
  }
});

test('strict publication receipts distinguish no-replace collision from committed errors and unknown acknowledgements', () => {
  for (const status of ['published', 'published-with-errors', 'not-published']) {
    assert.equal(workbook.validateVegetationWorkbookOutcome(receipt(status), destination, 'b'.repeat(64)).status, status);
  }
  for (const mutate of [
    x => x.status = 'success', x => x.sha256 = 'c'.repeat(64), x => x.path = '',
    x => x.requestedDestination = 'other', x => x.errorMessage = 'hidden warning', x => x.extra = true,
    x => x.path = '\ud800', x => x.errorMessage = null,
  ]) { const invalid = receipt(); mutate(invalid); assert.throws(() => workbook.validateVegetationWorkbookOutcome(invalid, destination, 'b'.repeat(64))); }
});

test('owned session freezes literal date/scope requests, holds duplicate/export/ack and requires new review after collision', async () => {
  const session = workbook.vegetationWorkbookPublicationSession(owner);
  const review = session.prepare(fixture());
  let resolve, captured, calls = 0;
  const pending = session.publish(review, destination, request => {
    captured = request; calls++; return new Promise(done => { resolve = done; });
  });
  review.createdDate = '2026-10-07'; review.approvalHash = 'changed';
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.throws(() => session.acknowledge(), /cannot be cancelled/);
  assert.throws(() => session.prepare(fixture()), /Acknowledge/);
  await assert.rejects(session.publish(fixture(), destination, async () => receipt()), /Acknowledge/);
  assert.equal(workbook.vegetationWorkbookPublicationSession({ ...owner }), session);
  assert.throws(() => workbook.vegetationWorkbookPublicationSession({ ...owner, suPath: 'other' }), /owned paths/);
  resolve(receipt('not-published')); await pending;
  assert.equal(calls, 1); assert.equal(captured.scope, 'unlumped'); assert.equal(captured.createdDate, '2026-10-06');
  assert.deepEqual(Object.keys(captured).sort(), ['approvalHash', 'createdDate', 'destination', 'scope']);
  session.acknowledge();
  await assert.rejects(session.publish(fixture(), destination, async () => receipt()), /fresh explicit/);
  await session.publish(session.prepare(fixture()), destination, async () => receipt());
  assert.equal(session.view().outcome.status, 'published'); session.acknowledge();
});

test('unknown RPC/nullable malformed outcomes persist through remount and acknowledgement with no blind replay', async () => {
  for (const response of [null, { status: 'success' }, new Error('lost acknowledgement')]) {
    const owned = { ...owner, contextId: `unknown-${String(response)}` };
    const session = workbook.vegetationWorkbookPublicationSession(owned), input = fixture();
    input.preview.contextId = owned.contextId;
    await session.publish(input, destination, async () => { if (response instanceof Error) throw response; return response; });
    assert.equal(session.view().outcome, null); assert.equal(session.view().blocked, true);
    assert.match(session.view().error, /outcome unknown/);
    const remounted = workbook.vegetationWorkbookPublicationSession(owned);
    assert.equal(remounted, session); remounted.acknowledge();
    assert.match(remounted.view().error, /Do not repeat/);
    await assert.rejects(remounted.publish(input, destination, async () => receipt()), /fresh explicit/);
  }
});

function routeHarness() {
  const code = source();
  const session = new workbook.VegetationWorkbookPublicationSession(owner);
  const reads = new (loadTypeScript('readRequests.ts', { '@wailsio/runtime': { Call: {}, CancelledRejectionError: class extends Error {} } }).ReadRequests)();
  let resolve, resolveExport, cancelled = 0, exportCancelled = 0, exports = 0;
  const context = vm.createContext({
    ...owner, workbookEnabled: true, lifeformEnabled: false, strataEnabled: false, codeEnabled: false,
    generation: 0, busy: false, operationHeld: false, error: '', preview: null, workbookReview: null, settings: fixture().preview.settings,
    reads, workbookSession: session, destination, reportBusy() {},
    requireLongVegetationGrouping: report.requireLongVegetationGrouping,
    VegetationWorkbookService: { GetReview(_context, request) {
      assert.equal(request, '{"scope":"unlumped"}');
      const pending = new Promise(done => { resolve = done; });
      pending.cancel = () => { cancelled++; };
      return pending;
    }, ExportReviewed(_context, request) {
      exports++;
      const planned = JSON.parse(request);
      assert.equal(planned.createdDate, '2026-10-06'); assert.equal(planned.scope, 'unlumped');
      const pending = new Promise(done => { resolveExport = done; });
      pending.cancel = () => { exportCancelled++; };
      return pending;
    } },
  });
  session.subscribe(() => { if (session.view().blocked) context.workbookReview = null; });
  Object.defineProperty(context, 'workbookBarrier', { get: () => session.view().busy || session.view().blocked });
  const cancel = code.slice(code.indexOf('  function cancel()'), code.indexOf('  async function options()'));
  const actions = code.slice(code.indexOf('  async function reviewWorkbook()'), code.indexOf('  const numberText'));
  vm.runInContext(cancel + actions + '\nthis.actions = { cancel, reviewWorkbook, publishWorkbook, acknowledgeWorkbook };', context);
  return { context, session, actions: context.actions, complete: value => resolve(value), cancelled: () => cancelled,
    completeExport: value => resolveExport(value), exportCancelled: () => exportCancelled, exports: () => exports };
}

test('actual review handler cancels borrowed reads, suppresses stale results and refuses overlapping root work', async () => {
  const harness = routeHarness(), { context, actions } = harness;
  context.operationHeld = true; await actions.reviewWorkbook(); assert.equal(context.busy, false);
  context.operationHeld = false;
  const pending = actions.reviewWorkbook(); assert.equal(context.busy, true);
  await actions.reviewWorkbook();
  actions.cancel(); assert.equal(harness.cancelled(), 1);
  harness.complete(fixture()); await pending;
  assert.equal(context.workbookReview, null); assert.equal(context.preview, null);
  const fresh = actions.reviewWorkbook(); harness.complete(fixture()); await fresh;
  assert.equal(context.workbookReview.createdDate, '2026-10-06');
  context.operationHeld = true; await actions.publishWorkbook(); assert.equal(harness.session.view().blocked, false);
});

test('actual export handler holds reads and duplicate publication without cancellation and clears prepared review on receipt', async () => {
  const harness = routeHarness(), { context, actions } = harness;
  const read = actions.reviewWorkbook(); harness.complete(fixture()); await read;
  const pending = actions.publishWorkbook();
  assert.equal(harness.session.view().busy, true); assert.equal(context.workbookReview, null);
  await actions.reviewWorkbook(); await actions.publishWorkbook();
  actions.cancel();
  assert.equal(harness.session.view().busy, true); assert.equal(harness.exportCancelled(), 0);
  assert.throws(() => actions.acknowledgeWorkbook(), /cannot be cancelled/);
  harness.completeExport(receipt('published-with-errors')); await pending;
  assert.equal(harness.exports(), 1); assert.equal(harness.session.view().blocked, true);
  actions.acknowledgeWorkbook(); assert.equal(context.workbookReview, null);
  await actions.publishWorkbook(); assert.equal(harness.exports(), 1);
  const retry = actions.reviewWorkbook(); harness.complete(fixture()); await retry;
  assert.ok(context.workbookReview);
});

test('actual root transition and both native-close paths reject unresolved publication even without a mounted route', async () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8'), ts = require('typescript');
  const close = app.slice(app.indexOf('  async function handleCloseRequest('), app.indexOf('  let activeHierarchyIndex'));
  const transition = app.slice(app.indexOf('  async function requestTransition('), app.indexOf('  async function respondToTransition('));
  let confirmed = 0, cancelled = 0, navigated = 0;
  const context = vm.createContext({
    closeWorking: false, closeRequest: '', closeError: '', error: '', view: 'home', editor: undefined,
    busy: false, editorBusy: false, transitionWorking: false, archivePublicationBusy: false, vegetationPublicationBusy: true,
    pendingTransition: null, transitionError: '', $projectState: { contextId: owner.contextId },
    document: { activeElement: null }, HTMLElement: class {}, tick: async () => {},
    closeDisposition: loadTypeScript('closeLifecycle.ts').closeDisposition,
    CloseService: { GetPendingCloseRequest: async () => 'request', CancelClose: async () => { cancelled++; },
      ConfirmClose: async () => { confirmed++; } },
  });
  vm.runInContext(ts.transpileModule(close + transition + '\nthis.actions = { handleCloseRequest, respondToClose, requestTransition };',
    { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
  await context.actions.handleCloseRequest('request'); assert.equal(cancelled, 1); assert.equal(confirmed, 0);
  context.closeRequest = 'request';
  await context.actions.respondToClose('discard'); assert.equal(confirmed, 0); assert.match(context.closeError, /Wait/);
  context.closeRequest = '';
  await context.actions.requestTransition(async () => { navigated++; });
  assert.equal(navigated, 0); assert.match(context.error, /Wait/);
  context.vegetationPublicationBusy = false;
  await context.actions.requestTransition(async () => { navigated++; });
  assert.equal(navigated, 1);
});

test('route and root wiring keep held navigation/native close, no cancellable export, raw approval, labels and wrapped safety above guidance', () => {
  const panel = source(), app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  for (const [filename, value] of [['LongVegetationReport.svelte', panel], ['App.svelte', app]]) {
    assert.equal(compile(value, { filename, generate: 'client' }).warnings.length, 0);
  }
  assert.match(panel, /VITE_LONG_VEGETATION_WORKBOOK === 'true'/);
  assert.match(panel, /workbookReview = \$state\.raw<ValidatedVegetationWorkbookReview/);
  assert.match(panel, /reads\.track\(VegetationWorkbookService\.GetReview/);
  assert.doesNotMatch(panel, /reads\.track\(VegetationWorkbookService\.ExportReviewed/);
  assert.match(panel, /if \(publication\.blocked\) workbookReview = null/);
  assert.match(panel, /onBusyChange\(busy \|\| workbookBarrier\)/);
  assert.match(panel, /label for="vegetation-workbook-destination"/);
  assert.match(panel, /textarea id="vegetation-workbook-destination"/);
  assert.match(panel, /grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(panel, /overflow-wrap: anywhere/);
  assert.ok(panel.indexOf('data-vegetation-workbook-unknown') < panel.indexOf('class="guidance"'));
  assert.equal((app.match(/closeDisposition\(state, busy \|\| editorBusy[^\n]*vegetationPublicationBusy/g) || []).length, 3);
  for (const name of ['loadPlots', 'refresh', 'loadHierarchy']) {
    assert.match(app, new RegExp(`async function ${name}\\([^)]*\\) \\{\\s*if \\(vegetationPublicationBusy`));
  }
  assert.match(app, /operationHeld=\{busy \|\| transitionWorking \|\| pendingTransition !== null \|\| !!closeRequest\}/);
  assert.match(app, /vegetationPublicationBusy = publication.busy \|\| publication.blocked/);
});

test('SSR remount shows persistent unknown alert and explicit acknowledgement without recreating preparation', async () => {
  const owned = { ...owner, contextId: 'ssr-unknown' }, input = fixture(); input.preview.contextId = owned.contextId;
  await workbook.vegetationWorkbookPublicationSession(owned).publish(input, destination, async () => null);
  const component = serverComponent(source().replace(/import\.meta\.env\.VITE_LONG_VEGETATION_\w+ === 'true'/g, 'true'),
    'LongVegetationReport.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { ContextService: {}, VegetationWorkbookService: {} },
      './longEnvironmentReport': environment, './longVegetationReport': report, './vegetationWorkbook': workbook,
    });
  const result = render(component, { props: { ...owned, onBusyChange() {} } });
  assert.match(result.body, /data-vegetation-workbook-unknown/);
  assert.match(result.body, /Acknowledge workbook outcome/);
  assert.match(result.body, /never replay blindly/);
  assert.doesNotMatch(result.body, /id="vegetation-workbook-destination"/);
});
