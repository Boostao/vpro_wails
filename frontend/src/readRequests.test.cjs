const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { pathToFileURL } = require('node:url');
const vm = require('node:vm');
const ts = require('typescript');
const compiled = ts.transpileModule(readFileSync(path.join(__dirname, 'readRequests.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText;
function load(sdk, RuntimeError) {
  const output = {};
  vm.runInNewContext(compiled, { exports: output, require: () => ({ ...sdk, Call: { RuntimeError } }) });
  return output;
}
const runtime = () => import(pathToFileURL(path.join(__dirname, '..', 'node_modules',
  '@wailsio', 'runtime', 'dist', 'cancellable.js')).href);

test('read ownership cancels actual Wails promises once and permits a fresh generation', async () => {
  const { CancellablePromise, CancelError } = await runtime();
  const output = load(await runtime());
  const requests = new output.ReadRequests();
  let cancelled = 0;
  const first = new CancellablePromise(() => {}, () => { cancelled++; });
  const second = new CancellablePromise(() => {}, () => { cancelled++; });
  const a = requests.track(first);
  const b = requests.track(second);
  const outcomes = Promise.allSettled([a, b]);
  await Promise.resolve();
  requests.cancelAll();
  requests.cancelAll();
  for (const outcome of await outcomes) {
    assert.equal(outcome.status, 'rejected');
    assert.ok(outcome.reason instanceof CancelError);
  }
  assert.equal(cancelled, 2);
  assert.equal(await requests.track(CancellablePromise.resolve('fresh')), 'fresh');
});

test('settled reads leave the ownership set and operational failures are not swallowed', async () => {
  const { CancellablePromise } = await runtime();
  const output = load(await runtime());
  const requests = new output.ReadRequests();
  const completed = CancellablePromise.resolve('done');
  let cancelled = 0;
  completed.cancel = () => { cancelled++; };
  assert.equal(await requests.track(completed), 'done');
  await assert.rejects(requests.track(CancellablePromise.reject(new Error('read failure'))), /read failure/);
  requests.cancelAll();
  assert.equal(cancelled, 0);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /loadRequest\+\+;[\s\S]*?reads\.cancelAll\(\)/);
  assert.match(form, /const request = \+\+loadRequest;\s+reads\.cancelAll\(\)/);
  for (const name of ['GetPlot', 'GetHeaderCapabilities', 'GetChildCapabilities', 'ListVegRecords',
    'ListHumusRecords', 'ListMineralRecords', 'ListOtherRecords', 'ListAuditEntries']) {
    assert.match(form, new RegExp(`reads\\.track\\(PlotService\\.${name}\\(`));
  }
  assert.doesNotMatch(form, /reads\.track\(PlotService\.(?:Update|Save|Create|Delete|Restore|Set)/);
});

test('only owned cancelled read acknowledgements are handled, including after editor disposal', async () => {
  const sdk = await runtime();
  const { RuntimeError } = await import(pathToFileURL(path.join(__dirname, '..', 'node_modules',
    '@wailsio', 'runtime', 'dist', 'runtime.js')).href);
  const output = load(sdk, RuntimeError);
  const requests = new output.ReadRequests();
  const owned = new sdk.CancellablePromise(() => {});
  const outcome = requests.track(owned).catch(() => {});
  requests.cancelAll();
  await outcome;
  const error = cause => new sdk.CancelledRejectionError(owned, cause);
  assert.equal(output.expectedReadCancellation(error(new RuntimeError('context canceled'))), true);
  assert.equal(output.expectedReadCancellation(error(new RuntimeError('database corrupt'))), false);
  assert.equal(output.expectedReadCancellation(error(new Error('context canceled'))), false);
  assert.equal(output.expectedReadCancellation(new RuntimeError('context canceled')), false);
  const unowned = new sdk.CancellablePromise(() => {});
  assert.equal(output.expectedReadCancellation(new sdk.CancelledRejectionError(unowned,
    new RuntimeError('context canceled'))), false);
});

test('browse, hierarchy, reference and species consumers own only their read generations', () => {
  const root = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(root, /plotReads\.track\(ProjectService\.ListPlots/);
  assert.match(root, /hierarchyReads\.track\(ProjectService\.GetHierarchyNodes/);
  assert.match(root, /if \(current === hierarchyRequest\) hierarchyNodes/);
  assert.match(root, /request\+\+;\s+hierarchyRequest\+\+;\s+plotReads\.cancelAll\(\);\s+hierarchyReads\.cancelAll\(\)/);
  assert.match(form, /referenceReads\.track\(ReferenceService\.GetListItems/);
  assert.match(form, /speciesReads\.track\(ReferenceService\.SearchSpecies/);
  assert.match(form, /if \(request !== speciesRequest\) return/);
  assert.match(form, /if \(request === speciesRequest\) searchingSpecies = false/);
  assert.doesNotMatch(root, /Reads\.track\(ContextService\.SwitchContext/);
});
