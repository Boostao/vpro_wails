const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createHash, webcrypto } = require('node:crypto');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const csv = loadTypeScript('projectTableCSV.ts', { './qualityEditor': quality, './projectMetadataRestore': cells },
  { crypto: webcrypto, TextEncoder, Uint8Array });
function storage() {
  const data = new Map();
  return { data, getItem: key => data.get(key) ?? null, setItem: (key, value) => data.set(key, value) };
}
function moduleFor(store = storage(), picker = async () => '') {
  return loadTypeScript('tableCSVArchiveExport.ts', {
    './qualityEditor': quality, './projectTableCSV': csv, '@wailsio/runtime': { Dialogs: { SaveFile: picker } },
  }, { localStorage: store });
}
const archive = moduleFor();
const owner = { contextId: 'archive-owner', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db' };
const table = 'Sample_Env';
const destination = 'C:\\FIXTUR~1\\ Literal\r\narchive.zip ';
const archiveSHA256 = 'b'.repeat(64);
const checksum = async text => createHash('sha256').update(text).digest('hex');
function review(scope = owner) {
  const data = 'Value\n7\n';
  return { review: { ...scope, descriptionMetadataPresent: false, csv: data,
    manifest: { version: 1, table, columns: [{ name: 'Value', declaredType: 'INTEGER' }], rowIds: ['1'],
      storage: [['integer']], descriptions: [], sha256: createHash('sha256').update(data).digest('hex') } },
    approvalHash: 'a'.repeat(64), archiveSHA256, byteCount: 412, format: 'vpro-owned-table-csv', version: 1 };
}
function outcome(status = 'published', requestedDestination = destination) {
  return { status, requestedDestination, path: status === 'not-published' ? '' : 'C:\\fixture\\ Literal\r\narchive.zip ',
    sha256: status === 'not-published' ? '' : archiveSHA256, errorMessage: status === 'published' ? '' : 'Explicit backend error' };
}
const deferred = () => { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; });
  return { promise, resolve, reject }; };

test('archive review checks owned physical CSV and artifact shape without claiming independent source/ZIP verification', async () => {
  const result = await archive.validateTableCSVArchiveReview(review(), owner, table, checksum);
  assert.equal(result.approvalHash, 'a'.repeat(64)); assert.equal(result.archiveSHA256, archiveSHA256);
  for (const mutate of [v => v.approvalHash = 'A'.repeat(64), v => v.approvalHash = '', v => v.archiveSHA256 = null,
    v => v.archiveSHA256 = 'B'.repeat(64), v => v.byteCount = 0, v => v.byteCount = -1,
    v => v.byteCount = 1.5, v => v.byteCount = Number.MAX_SAFE_INTEGER + 1, v => v.format = 'csv', v => v.version = 2,
    v => v.review.contextId = 'foreign', v => v.review.project = 'sample',
    v => v.review.projectPath = 'C:\\FIXTUR~1\\Sample.db', v => v.review.manifest.table = 'Sample_Veg',
    v => v.review.manifest.sha256 = '0'.repeat(64), v => v.review.csv += '8\n']) {
    const value = review(); mutate(value);
    await assert.rejects(archive.validateTableCSVArchiveReview(value, owner, table, checksum));
  }
  await assert.rejects(archive.validateTableCSVArchiveReview(null, owner, table, checksum));
  await assert.rejects(archive.validateTableCSVArchiveReview(review(), owner, 'Sample_Unsupported', checksum));
});

test('outer artifact and owner aliases detach before held asynchronous CSV verification', async () => {
  const input = review(), scope = { ...owner }, gate = deferred();
  const pending = archive.validateTableCSVArchiveReview(input, scope, table, () => gate.promise);
  input.approvalHash = 'changed'; input.archiveSHA256 = 'changed'; input.byteCount = 0;
  input.review.csv = 'changed'; input.review.manifest.columns[0].name = 'changed'; scope.contextId = 'changed';
  gate.resolve(await checksum('Value\n7\n'));
  const result = await pending;
  assert.equal(result.approvalHash, 'a'.repeat(64)); assert.equal(result.archiveSHA256, archiveSHA256);
  assert.equal(result.byteCount, 412); assert.equal(result.review.csv, 'Value\n7\n');
});

test('typed receipt preserves literal request and allows canonical observed path but refuses incoherent results', () => {
  for (const status of ['published', 'published-with-errors', 'not-published']) {
    assert.equal(archive.validateTableCSVArchiveOutcome(outcome(status), destination, archiveSHA256).status, status);
  }
  for (const mutate of [v => v.requestedDestination = destination.trim(), v => v.requestedDestination = v.path,
    v => v.status = 'unknown', v => v.path = '', v => v.path = '\ud800', v => v.path = 'bad\0path',
    v => v.sha256 = '0'.repeat(64), v => v.sha256 = '', v => v.sha256 = 'B'.repeat(64),
    v => v.errorMessage = 'hidden error', v => v.errorMessage = '\ud800']) {
    const value = outcome(); mutate(value);
    assert.throws(() => archive.validateTableCSVArchiveOutcome(value, destination, archiveSHA256));
  }
  for (const value of [null, { ...outcome('not-published'), path: 'somewhere' },
    { ...outcome('not-published'), sha256: archiveSHA256 }, { ...outcome('not-published'), errorMessage: '' },
    { ...outcome('published-with-errors'), errorMessage: '' }]) {
    assert.throws(() => archive.validateTableCSVArchiveOutcome(value, destination, archiveSHA256));
  }
});

test('destination validation never repairs whitespace, line breaks or literal spelling', () => {
  for (const value of [destination, ' ', '\r\n', 'C:\\MixedCase\\withoutExtension', '🌲.zip']) {
    assert.equal(archive.validTableCSVArchiveDestination(value), true);
  }
  for (const value of ['', '\0', '\ud800', '\udc00', null, 4]) assert.equal(archive.validTableCSVArchiveDestination(value), false);
});

test('held write is not cancellable; busy/acknowledgement barrier and detached request survive remount', async () => {
  const mod = moduleFor(), first = mod.tableCSVArchivePublicationSession(owner), gate = deferred(), started = deferred();
  let calls = 0, cancels = 0, captured, busy;
  const unsubscribe = first.subscribe(() => { const view = first.view(); busy = view.busy || view.blocked; });
  const value = review();
  const pending = first.publish(value, table, destination, request => {
    captured = request; calls++; started.resolve();
    return { then: (...args) => gate.promise.then(...args), cancel() { cancels++; } };
  });
  value.approvalHash = 'changed'; value.review.manifest.table = 'changed';
  await started.promise;
  assert.equal(busy, true); assert.equal(first.view().receipts[0].phase, 'pending');
  assert.equal(captured.table, table); assert.equal(captured.approvalHash, 'a'.repeat(64));
  assert.equal(captured.destination, destination);
  assert.throws(() => first.acknowledge());
  await assert.rejects(first.publish(review(), table, destination, () => { calls++; }));
  unsubscribe();
  const remount = mod.tableCSVArchivePublicationSession(owner);
  assert.equal(remount, first);
  let remountedBusy; remount.subscribe(() => { const view = remount.view(); remountedBusy = view.busy || view.blocked; });
  gate.resolve(outcome()); await pending;
  assert.equal(calls, 1); assert.equal(cancels, 0);
  assert.equal(remountedBusy, true); assert.equal(remount.view().busy, false);
  remount.acknowledge(); assert.equal(remountedBusy, false);
  assert.equal(remount.view().receipts[0].outcome.status, 'published');
  assert.throws(() => mod.tableCSVArchivePublicationSession({ ...owner, projectPath: 'C:\\different.db' }));
  assert.throws(() => mod.tableCSVArchivePublicationSession({ ...owner, project: 'sample' }));
});

test('all known/unknown receipts persist after ack/reload/new attempts; unknown has no fabricated commit flags', async () => {
  const store = storage();
  let session = new archive.TableCSVArchivePublicationSession(owner, store);
  const ports = [async () => outcome(), async () => { throw new Error('IPC disconnected'); },
    async () => outcome('published-with-errors'), async () => null, async () => outcome('not-published'),
    async () => ({ ...outcome(), sha256: '0'.repeat(64) })];
  for (let index = 0; index < ports.length; index++) {
    await session.publish(review(), table, destination, ports[index]);
    const view = session.view(), receipt = view.receipts[index];
    assert.equal(view.blocked, true); assert.equal(view.receipts.length, index + 1);
    if (index % 2) {
      assert.equal(receipt.phase, 'unknown'); assert.equal(receipt.outcome, null);
      assert.match(receipt.error, /outcome unknown/);
      assert.equal('changed' in receipt, false); assert.equal('committed' in receipt, false);
    } else assert.equal(receipt.phase, 'known');
    const detached = session.view(); detached.receipts[0].requestedDestination = 'caller mutation';
    assert.equal(session.view().receipts[0].requestedDestination, destination);
    session.acknowledge();
    session = new archive.TableCSVArchivePublicationSession(owner, store);
    assert.equal(session.view().blocked, false); assert.equal(session.view().receipts.length, index + 1);
    assert.equal(session.view().receipts[index].acknowledged, true);
    assert.equal(session.view().receipts[0].outcome.status, 'published');
  }
  assert.equal(session.view().receipts.filter(receipt => receipt.phase === 'unknown').length, 3);
});

test('reload during invoked write restores unknown unacknowledged evidence, never automatic replay', async () => {
  const store = storage(), session = new archive.TableCSVArchivePublicationSession(owner, store), gate = deferred(), started = deferred();
  let calls = 0;
  const pending = session.publish(review(), table, destination, () => { calls++; started.resolve(); return gate.promise; });
  await started.promise;
  const reloaded = new archive.TableCSVArchivePublicationSession(owner, store);
  assert.equal(calls, 1); assert.equal(reloaded.view().busy, false); assert.equal(reloaded.view().blocked, true);
  assert.equal(reloaded.view().receipts[0].phase, 'unknown'); assert.equal(reloaded.view().receipts[0].outcome, null);
  await assert.rejects(reloaded.publish(review(), table, destination, () => { calls++; }));
  reloaded.acknowledge(); assert.equal(reloaded.view().receipts.length, 1);
  gate.resolve(outcome()); await pending;
});

test('malformed reviews never invoke writes; persistence failures refuse writes and retain previous evidence', async () => {
  const store = storage(), session = new archive.TableCSVArchivePublicationSession(owner, store);
  let calls = 0;
  await assert.rejects(session.publish({ ...review(), approvalHash: '' }, table, destination, () => { calls++; }));
  await assert.rejects(session.publish(review(), 'Sample_Veg', destination, () => { calls++; }));
  await assert.rejects(session.publish(review(), table, '\ud800', () => { calls++; }));
  assert.equal(calls, 0); assert.equal(session.view().receipts.length, 0); assert.equal(session.view().busy, false);
  await session.publish(review(), table, destination, async () => outcome()); session.acknowledge();
  const previous = JSON.stringify(session.view().receipts);
  store.setItem = () => { throw new Error('storage full'); };
  await assert.rejects(session.publish(review(), table, destination, () => { calls++; }));
  assert.equal(calls, 0); assert.equal(JSON.stringify(session.view().receipts), previous);
  assert.equal(session.view().blocked, true); assert.match(session.view().storageError, /no publication invoked/);
  assert.throws(() => session.acknowledge());
});

test('missing/corrupt/foreign persistent history fails closed instead of erasing or overwriting it', () => {
  for (const raw of ['not-json', JSON.stringify({ version: 1, owner: { ...owner, project: 'foreign' }, receipts: [] }),
    JSON.stringify({ version: 1, owner, receipts: [{ phase: 'known' }] })]) {
    const store = storage(); store.setItem(`vpro-owned-table-csv-receipts-v1:${owner.contextId}`, raw);
    const session = new archive.TableCSVArchivePublicationSession(owner, store);
    assert.equal(session.view().blocked, true); assert.match(session.view().storageError, /history unavailable/);
    assert.equal([...store.data.values()][0], raw);
  }
  const session = new archive.TableCSVArchivePublicationSession(owner, { getItem() { throw new Error('denied'); }, setItem() {} });
  assert.equal(session.view().blocked, true);
});

test('optional native picker cancellation/errors preserve destination and hold busy without invoking publication', async () => {
  const session = new archive.TableCSVArchivePublicationSession(owner, storage()), gate = deferred();
  let busy; session.subscribe(() => { const view = session.view(); busy = view.busy || view.blocked; });
  const pending = session.chooseDestination(destination, () => gate.promise);
  assert.equal(busy, true); assert.throws(() => session.acknowledge());
  await assert.rejects(session.publish(review(), table, destination, async () => outcome()));
  gate.resolve(''); assert.equal(await pending, destination); assert.equal(busy, false);
  await assert.rejects(session.chooseDestination(destination, async () => { throw new Error('native failure'); }));
  await assert.rejects(session.chooseDestination(destination, async () => '\ud800'));
  assert.equal(session.view().receipts.length, 0); assert.equal(busy, false);
  const canonical = 'C:\\fixture\\Selected.zip';
  assert.equal(await session.chooseDestination(destination, async () => canonical), canonical);
  let options;
  const mod = moduleFor(storage(), async value => { options = value; return ''; });
  assert.equal(await mod.chooseTableCSVArchiveFile(), '');
  assert.equal(options.Title, 'Select a new migration archive destination');
  assert.equal(options.Filename, 'migration.zip'); assert.equal(options.Filters[0].Pattern, '*.zip');
});

const componentSource = readFileSync(path.join(__dirname, 'ProjectTableCSVReview.svelte'), 'utf8');
function componentHarness({ enabled = true, service = {}, mod = moduleFor(), csvModule = csv, ownerScope = owner } = {}) {
  const script = componentSource.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1]
    .replaceAll("import.meta.env.VITE_TABLE_CSV_ARCHIVE_EXPORT === 'true'", String(enabled));
  const destroy = [], changes = [], reads = { tracked: 0, cancelled: 0 };
  const dependencies = {
    svelte: { onDestroy: callback => destroy.push(callback) },
    '../bindings/github.com/boostao/vpro-wails': { ContextService: service, TableCSVArchiveService: service },
    './readRequests': { ReadRequests: class { track(value) { reads.tracked++; return value; } cancelAll() { reads.cancelled++; } } },
    './projectTableCSV': csvModule, './longEnvironmentReport': { reportCellText: String }, './tableCSVArchiveExport': mod,
  };
  const module = { exports: {} }, state = value => value; state.snapshot = structuredClone;
  const exposed = `\nmodule.exports = { show, cancel, changeTable, chooseDestination, publish, acknowledge,
    setSuffix(value) { suffix = value; changeTable(); }, setDestination(value) { destination = value; },
    view() { return { busy, error, review, approval, publication, destination }; } };`;
  const code = ts.transpileModule(script + exposed, { compilerOptions: { module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022 } }).outputText;
  vm.runInNewContext(code, { module, exports: module.exports, structuredClone,
    $props: () => ({ ...ownerScope, onBusyChange: value => changes.push(value) }), $state: state,
    require(name) { if (name in dependencies) return dependencies[name]; throw new Error(`Unexpected ${name}`); } });
  return { ...module.exports, reads, changes, destroy: () => destroy.forEach(callback => callback()) };
}

test('component generation cancellation drops late archive verification and changing table invalidates approval', async () => {
  const gate = deferred(), started = deferred(), mod = { ...moduleFor(), validateTableCSVArchiveReview: async (...args) => {
    const value = await archive.validateTableCSVArchiveReview(...args); started.resolve(); await gate.promise; return value;
  } };
  const harness = componentHarness({ mod, service: { GetTableCSVArchiveReview: async () => review() } });
  const pending = harness.show(); await started.promise; harness.setSuffix('Veg'); gate.resolve(); await pending;
  assert.equal(harness.view().review, null); assert.equal(harness.view().approval, null);
  assert.equal(harness.changes.at(-1), false); assert.ok(harness.reads.cancelled >= 2);
  harness.destroy();
});

test('late read transport/cancel and picker errors cannot install approval, publish or release unknown receipt barrier', async () => {
  const readGate = deferred();
  let writes = 0;
  const harness = componentHarness({ service: {
    GetTableCSVArchiveReview: () => readGate.promise,
    ExportReviewedTableCSVArchive: async () => { writes++; },
  } });
  const reading = harness.show(); harness.cancel(); readGate.resolve(review()); await reading;
  assert.equal(harness.view().approval, null); assert.equal(harness.view().review, null);
  const failingPicker = componentHarness({ mod: moduleFor(storage(), async () => { throw new Error('native chooser error'); }) });
  failingPicker.setDestination(destination); await failingPicker.chooseDestination();
  assert.equal(failingPicker.view().destination, destination);
  assert.match(failingPicker.view().error, /chooser unavailable.*destination unchanged, no publication invoked/);
  assert.equal(failingPicker.changes.at(-1), false);
  await harness.publish(); assert.equal(writes, 0); harness.destroy(); failingPicker.destroy();
});

test('component exact approved table, explicit export, picker and persistent acknowledgement drive close/navigation barrier', async () => {
  const gate = deferred(), pickerGate = deferred(), store = storage(), started = deferred();
  const mod = moduleFor(store, () => pickerGate.promise);
  let writes = 0, request;
  const service = { GetTableCSVArchiveReview: async (contextId, json) => {
    assert.equal(contextId, owner.contextId); assert.equal(JSON.parse(json).table, table); return review();
  }, ExportReviewedTableCSVArchive: (contextId, json) => {
    writes++; request = JSON.parse(json); assert.equal(contextId, owner.contextId); started.resolve(); return gate.promise;
  } };
  const harness = componentHarness({ mod, service });
  harness.setDestination(destination);
  const choosing = harness.chooseDestination(); assert.equal(harness.changes.at(-1), true);
  pickerGate.resolve(''); await choosing; assert.equal(harness.view().destination, destination);
  assert.equal(harness.changes.at(-1), false); assert.equal(writes, 0);
  await harness.show(); assert.equal(writes, 0);
  const pending = harness.publish(); await started.promise;
  assert.equal(harness.changes.at(-1), true); assert.equal(writes, 1);
  assert.equal(request.destination, destination); assert.equal(request.table, table);
  assert.equal(harness.reads.tracked, 1); assert.equal(harness.view().approval, null);
  gate.reject(new Error('RPC disconnect')); await pending;
  assert.equal(harness.changes.at(-1), true);
  assert.equal(harness.view().publication.receipts[0].phase, 'unknown');
  harness.destroy(); assert.equal(harness.changes.at(-1), true);
  const remount = componentHarness({ mod, service });
  assert.equal(remount.changes.at(-1), true);
  await remount.show(); assert.equal(remount.reads.tracked, 0);
  remount.acknowledge(); assert.equal(remount.changes.at(-1), false);
  assert.equal(remount.view().publication.receipts[0].phase, 'unknown');
  await remount.show(); assert.equal(remount.view().publication.receipts.length, 1);
  assert.equal(writes, 1);
  remount.setSuffix('Veg'); assert.equal(remount.view().approval, null);
  await remount.publish(); assert.equal(writes, 1); remount.destroy();
});

test('old default-off component remains read-only and uses existing cancellation/validation', async () => {
  let writes = 0;
  const harness = componentHarness({ enabled: false, service: {
    GetProjectTableCSVReview: async () => review().review,
    ExportReviewedTableCSVArchive: async () => { writes++; },
  } });
  await harness.show(); assert.equal(harness.view().review.manifest.table, table);
  await harness.chooseDestination(); await harness.publish();
  assert.equal(writes, 0); assert.equal(harness.view().publication, null); assert.equal(harness.view().approval, null);
  harness.destroy(); assert.equal(harness.changes.at(-1), false);
});

test('actual App close handlers and navigation transition refuse pending/unacknowledged archive and allow after ack', async () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const close = loadTypeScript('closeLifecycle.ts');
  const session = new archive.TableCSVArchivePublicationSession(owner, storage()), gate = deferred(), started = deferred();
  const context = { editorBusy: false, archivePublicationBusy: false, vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    busy: false, transitionWorking: false, pendingTransition: null,
    closeWorking: false, closeRequest: '', view: 'table-csv', editor: null, error: '', transitionError: '',
    closeError: '', $projectState: { contextId: owner.contextId }, calls: { cancelled: 0, confirmed: 0, navigated: 0 },
    document: { activeElement: null }, HTMLElement: class {}, tick: async () => {}, closeDisposition: close.closeDisposition };
  context.CloseService = { GetPendingCloseRequest: async () => 'owned-close',
    CancelClose: async () => { context.calls.cancelled++; }, ConfirmClose: async () => { context.calls.confirmed++; } };
  const nativeClose = app.slice(app.indexOf('  async function handleCloseRequest'), app.indexOf('  let activeHierarchyIndex'));
  const transition = app.slice(app.indexOf('  async function requestTransition'), app.indexOf('  async function respondToTransition'));
  vm.createContext(context);
  vm.runInContext(ts.transpileModule(nativeClose + transition, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
  session.subscribe(() => { const view = session.view(); context.archivePublicationBusy = view.busy || view.blocked; });
  const pending = session.publish(review(), table, destination, () => { started.resolve(); return gate.promise; });
  await started.promise;
  await context.handleCloseRequest('owned-close');
  await context.requestTransition(async () => { context.calls.navigated++; });
  assert.equal(context.calls.cancelled, 1); assert.equal(context.calls.confirmed, 0); assert.equal(context.calls.navigated, 0);
  context.closeRequest = 'owned-close';
  await context.respondToClose('discard');
  assert.equal(context.calls.confirmed, 0); context.closeRequest = '';
  gate.resolve(outcome()); await pending;
  await context.handleCloseRequest('owned-close');
  await context.requestTransition(async () => { context.calls.navigated++; });
  assert.equal(context.calls.cancelled, 2); assert.equal(context.calls.navigated, 0);
  session.acknowledge();
  await context.handleCloseRequest('owned-close');
  await context.requestTransition(async () => { context.calls.navigated++; });
  assert.equal(context.calls.confirmed, 1); assert.equal(context.calls.navigated, 1);
  context.vegetationPublicationBusy = true;
  await context.handleCloseRequest('owned-close');
  await context.requestTransition(async () => { context.calls.navigated++; });
  assert.equal(context.calls.cancelled, 3); assert.equal(context.calls.confirmed, 1); assert.equal(context.calls.navigated, 1);
  context.closeRequest = 'owned-close';
  await context.respondToClose('discard');
  assert.equal(context.calls.confirmed, 1); context.closeRequest = '';
  context.vegetationPublicationBusy = false;
  await context.handleCloseRequest('owned-close');
  await context.requestTransition(async () => { context.calls.navigated++; });
  assert.equal(context.calls.confirmed, 2); assert.equal(context.calls.navigated, 2);
});

test('UI literal opt-in, labels, receipts above fields and existing app native/navigation close guards remain integrated', async () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  assert.equal(compile(componentSource, { filename: 'ProjectTableCSVReview.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(componentSource, /VITE_TABLE_CSV_ARCHIVE_EXPORT === 'true'/);
  assert.match(componentSource, /busy \|\| !!publication\?\.busy \|\| !!publication\?\.blocked/);
  assert.ok(componentSource.indexOf('data-table-csv-archive-receipt') < componentSource.indexOf('id="table-csv-archive-destination"'));
  assert.match(componentSource, /<label for="table-csv-archive-destination">/);
  assert.match(componentSource, /<textarea id="table-csv-archive-destination"/);
  assert.match(componentSource, /data-table-csv-archive-publish/);
  assert.match(componentSource, /data-table-csv-archive-choose/);
  assert.match(componentSource, /data-table-csv-archive-receipt=\{receipt\.phase === 'known' \? '' : undefined\}/);
  assert.match(componentSource, /data-table-csv-archive-unknown-receipt=\{receipt\.phase === 'unknown' \? '' : undefined\}/);
  assert.doesNotMatch(componentSource, /\.trim\(|toLowerCase\(|toUpperCase\(/);
  assert.match(componentSource, /not independently verified by browser/);
  assert.match(componentSource, /not independently computed by browser/);
  assert.match(app, /view === 'table-csv' && import\.meta\.env\.VITE_TABLE_CSV_REVIEW === 'true'/);
  assert.match(app, /ProjectTableCSVReview[\s\S]*?onBusyChange=\{\(value\) => \{ editorBusy = value; \}\}/);
  assert.match(app, /function navigate[\s\S]*?requestTransition/);
  assert.match(app, /closeDisposition\(state, busy \|\| editorBusy \|\| transitionWorking/);
  assert.match(app, /archivePublicationBusy = publication\.busy \|\| publication\.blocked/);
  assert.match(app, /publication\.blocked && view === 'home'\) view = 'table-csv'/);
  const { render } = await import('svelte/server');
  for (const gate of [undefined, 'false', 'TRUE', '1', true, 'true']) {
    const enabled = gate === 'true';
    const source = componentSource.replaceAll("import.meta.env.VITE_TABLE_CSV_ARCHIVE_EXPORT === 'true'", String(enabled));
    const mod = moduleFor();
    const component = serverComponent(source, 'ProjectTableCSVReview.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {}, './projectTableCSV': csv,
      './longEnvironmentReport': { reportCellText: String }, './tableCSVArchiveExport': mod,
    });
    const html = render(component, { props: { ...owner, onBusyChange() {} } }).body;
    assert.equal(html.includes('data-table-csv-archive-export'), enabled);
    assert.equal(html.includes('File publication, import, RDS and TurboVeg remain unavailable'), !enabled);
  }
  const scripts = JSON.parse(readFileSync(path.join(__dirname, '..', 'package.json'))).scripts;
  assert.equal(scripts.test.split('src/tableCSVArchiveExport.test.cjs').length - 1, 1);
});
