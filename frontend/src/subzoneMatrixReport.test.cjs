const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const publication = loadTypeScript('publicationSession.ts');
const environment = loadTypeScript('environmentWorkbook.ts', {
  './lifeformSummary': lifeform, './longEnvironmentReport': {}, './publicationSession': publication,
});
const matrix = loadTypeScript('subzoneMatrixReport.ts', {
  './lifeformSummary': lifeform, './qualityEditor': quality, './environmentWorkbook': environment,
  './publicationSession': publication,
});
const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'subzoneMatrixWire.json'), 'utf8'));
const owner = { contextId: 'matrix-context', hierarchy: 'Sample', hierarchyPath: 'C:\\fixture\\hierarchy.db' };
const destination = 'C:\\fixture\\matrix.xlsx';

test('shared actual-Go paged wire preserves literal values, NULLs, no-label arrays and owned scope', () => {
  const wires = fixture(), before = JSON.stringify(wires);
  for (const name of ['first', 'page', 'end', 'empty']) {
    const input = wires[name], page = matrix.matrixPageFromWire(input, owner, input.offset, input.limit);
    assert.equal(page.total, name === 'empty' ? 0 : 3);
    assert.doesNotThrow(() => structuredClone(page));
    if (page.rows.length) page.rows[0].name = 'only caller';
  }
  assert.equal(JSON.stringify(wires), before);
  const review = matrix.matrixWorkbookFromWire(wires.review, owner);
  assert.equal(review.preview.rows[0].name, null);
  assert.equal(review.preview.rows[1].values[0], '01');
  assert.equal(matrix.matrixCellText(null), '(NULL)');
  assert.equal(matrix.matrixCellText(''), '(empty text)');
  assert.equal(matrix.matrixCellText('=1+1'), '=1+1');
  for (const name of ['published', 'notPublished', 'publishedWithErrors']) {
    assert.doesNotThrow(() => environment.validateEnvironmentWorkbookOutcome(wires[name], destination, review.workbookSHA256));
  }
});

for (const [name, mutate] of [
  ['context', value => value.contextId = 'foreign'],
  ['hierarchy', value => value.hierarchy = 'Other'],
  ['hierarchy path', value => value.hierarchyPath += 'x'],
  ['empty labels path', value => value.labelsPath = ''],
  ['malformed labels path', value => value.labelsPath = '\ud800'],
  ['source hash', value => value.sourceSHA256 = 'A'.repeat(64)],
  ['offset', value => value.offset = 1],
  ['limit', value => value.limit = 100],
  ['fractional total', value => value.total = 3.5],
  ['excess total', value => value.total = 100001],
  ['missing row', value => value.rows.pop()],
  ['sparse row', value => delete value.rows[0]],
  ['sparse column', value => delete value.columns[0]],
  ['duplicate pivot', value => value.columns[1] = value.columns[0]],
  ['unknown pivot type', value => value.columns[0] = null],
  ['malformed pivot', value => value.columns[0] = '\ud800'],
  ['missing nullable name', value => delete value.rows[0].name],
  ['numeric name', value => value.rows[0].name = 0],
  ['malformed name', value => value.rows[1].name = '\ud800'],
  ['short values', value => value.rows[1].values.pop()],
  ['sparse values', value => delete value.rows[1].values[0]],
  ['numeric value', value => value.rows[1].values[0] = 1],
  ['malformed value', value => value.rows[1].values[0] = '\ud800'],
  ['extra original table', value => value.hierarchyOriginal = {}],
]) test(`strict matrix transport rejects ${name}`, () => {
  const value = fixture().first; mutate(value);
  assert.throws(() => matrix.matrixPageFromWire(value, owner, 0, 25));
});

test('literal opaque suffixes stay unchanged; request/approval/output bounds never default', () => {
  const value = fixture().first;
  value.rows[1].values[0] = '=1+1\r\né🌲\0';
  assert.equal(matrix.matrixPageFromWire(value, owner, 0, 25).rows[1].values[0], value.rows[1].values[0]);
  for (const [offset, limit] of [[-1, 25], [0.5, 25], [0, 0], [0, 101], [Infinity, 25]]) {
    assert.throws(() => matrix.matrixPageFromWire(value, owner, offset, limit));
  }
  for (const change of [
    value => value.approvalHash = '',
    value => value.workbookSHA256 = 'x'.repeat(64),
    value => value.bytes = 0,
    value => value.bytes = 0.5,
    value => value.preview = fixture().empty,
    value => value.preview.offset = 1,
    value => value.preview.limit = 50,
  ]) {
    const review = fixture().review; change(review);
    assert.throws(() => matrix.matrixWorkbookFromWire(review, owner));
  }
});

test('per-context publication keeps held operation and irreversible receipts through remount/acknowledgement', async () => {
  const wires = fixture();
  const session = matrix.matrixPublicationSession(owner);
  let resolve, calls = 0;
  const pending = session.publish(wires.review, destination, request => {
    calls++; assert.equal(request.approvalHash, wires.review.approvalHash);
    return new Promise(yes => { resolve = yes; });
  });
  const remount = matrix.matrixPublicationSession(owner);
  assert.equal(remount, session); assert.equal(remount.view().busy, true);
  assert.throws(() => remount.acknowledge(), /cannot be cancelled/);
  await assert.rejects(remount.publish(wires.review, destination, () => { calls++; }), /Acknowledge/);
  assert.throws(() => matrix.matrixPublicationSession({ ...owner, hierarchyPath: 'foreign' }), /different selected hierarchy/);
  resolve(wires.publishedWithErrors); await pending;
  assert.equal(calls, 1);
  assert.equal(remount.view().blocked, true);
  assert.equal(remount.view().outcome.status, 'published-with-errors');
  remount.acknowledge();
  assert.equal(remount.view().blocked, false);
  assert.equal(remount.view().outcome.status, 'published-with-errors');
});

test('unknown publication blocks replay; invalid literal destination never invokes the port', async () => {
  const session = new matrix.MatrixPublicationSession(owner), review = fixture().review;
  let calls = 0;
  await assert.rejects(session.publish(review, 'wrong.db', () => { calls++; }));
  assert.equal(calls, 0);
  await session.publish(review, destination, () => { calls++; throw new Error('lost response'); });
  assert.equal(session.view().blocked, true);
  assert.match(session.view().error, /outcome unknown.*Do not repeat/);
  session.acknowledge();
  assert.match(session.view().error, /lost response/);
});

function deferred() {
  let resolve, reject, cancels = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancels++; };
  return { promise, resolve, reject, cancelled: () => cancels };
}
function driver(port) {
  const activity = [];
  const values = {
    ...owner, owner, session: new matrix.MatrixPublicationSession(owner), destination,
    busy: false, barrier: false, available: true, preview: null, workbook: null, error: '',
    generation: 0, reads: new ReadRequests(), limit: 25, SubzoneMatrixService: port,
    onBusyChange: value => activity.push(value),
    matrixPageFromWire: matrix.matrixPageFromWire, matrixWorkbookFromWire: matrix.matrixWorkbookFromWire,
    sameMatrixSource: matrix.sameMatrixSource,
  };
  const context = componentFunctions('SubzoneMatrixReport.svelte',
    ['reportBusy', 'cancel', 'show', 'reviewWorkbook', 'publishWorkbook', 'acknowledge'], values);
  return { context, activity, actions: context.actions };
}
test('actual component paging never requests XLSX and refuses changed-source continuation', async () => {
  const wires = fixture(), calls = [];
  const d = driver({ GetPreview: (...args) => {
    calls.push(args); return Object.assign(Promise.resolve(wires.first), { cancel() {} });
  }, GetReview() { throw new Error('paging must not serialize XLSX'); } });
  await d.actions.show();
  assert.deepEqual(calls, [[owner.contextId, '{"offset":0,"limit":25}']]);
  assert.equal(d.context.preview.total, 3); assert.equal(d.context.workbook, null);
  d.context.SubzoneMatrixService.GetPreview = () => {
    const changed = fixture().end; changed.sourceSHA256 = 'a'.repeat(64);
    return Object.assign(Promise.resolve(changed), { cancel() {} });
  };
  await d.actions.show(3);
  assert.equal(d.context.preview, null); assert.match(d.context.error, /changed between pages/);
});
test('actual Cancel and remount ignore late read/review results without clearing a newer held operation', async () => {
  const old = deferred(), fresh = deferred();
  const d = driver({ GetReview: () => old.promise, GetPreview: () => fresh.promise });
  const pendingOld = d.actions.reviewWorkbook();
  d.actions.cancel(); assert.equal(old.cancelled(), 1);
  const pendingFresh = d.actions.show();
  old.resolve(fixture().review); await pendingOld;
  assert.equal(d.context.busy, true); assert.equal(d.context.workbook, null);
  fresh.resolve(fixture().first); await pendingFresh;
  assert.equal(d.context.preview.total, 3); assert.equal(d.context.busy, false);
  const held = deferred(), prior = driver({ GetPreview: () => held.promise });
  const pending = prior.actions.show();
  prior.actions.cancel();
  const remount = driver({ GetPreview: () => Object.assign(Promise.resolve(fixture().first), { cancel() {} }) });
  held.resolve(fixture().first); await pending;
  assert.equal(prior.context.preview, null); assert.equal(remount.context.preview, null);
  await remount.actions.show(); assert.equal(remount.context.preview.total, 3);
});
test('actual component honors publication barrier and surfaces backend read refusal', async () => {
  const d = driver({ GetPreview() { throw new Error('selected hierarchy refused'); } });
  d.context.barrier = true;
  await d.actions.show(); await d.actions.reviewWorkbook(); await d.actions.publishWorkbook();
  assert.equal(d.context.preview, null); assert.equal(d.context.error, '');
  d.context.barrier = false;
  await d.actions.show();
  assert.match(d.context.error, /selected hierarchy refused/); assert.equal(d.context.busy, false);
});

test('actual client-runtime proxy boundary is detached without structuredClone(proxy)', async () => {
  const { proxy } = await import('svelte/internal/client');
  const reactive = proxy(fixture().review);
  assert.throws(() => structuredClone(reactive));
  const validated = matrix.matrixWorkbookFromWire(reactive, owner);
  assert.doesNotThrow(() => structuredClone(validated));
  reactive.preview.rows[1].values[0] = 'changed reactive';
  assert.equal(validated.preview.rows[1].values[0], '01');
});

test('compiled SSR consumes callbacks, literal table/group captions and portable limitations', async () => {
  let source = readFileSync(path.join(__dirname, 'SubzoneMatrixReport.svelte'), 'utf8');
  const marker = 'let preview = $state.raw<MatrixPage | null>(null);';
  assert.equal(source.split(marker).length, 2);
  source = source.replace(marker, `let preview = $state.raw<MatrixPage | null>(${JSON.stringify(fixture().first)});`);
  source = source.replace('let workbook = $state.raw<MatrixWorkbookReview | null>(null);',
    `let workbook = $state.raw<MatrixWorkbookReview | null>(${JSON.stringify(fixture().review)});`);
  const component = serverComponent(source, 'SubzoneMatrixReport.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { SubzoneMatrixService: {} },
    './subzoneMatrixReport': { ...matrix, matrixPublicationSession: () => new matrix.MatrixPublicationSession(owner) },
  });
  const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
  let callbacks = 0;
  const body = render(component, { props: { ...owner, onBusyChange: () => callbacks++ } }).body;
  assert.ok(callbacks > 0, 'lazy SSR must execute subscription/cleanup callbacks');
  for (const caption of ['Name', 'AAAAAAAAA', 'BBBBBBBBB', 'ZZZZZZZZZ']) assert.ok(body.includes(`>${caption}</th>`));
  assert.match(body, /Association row 3/);
  assert.match(body, /New XLSX destination/); assert.match(body, /not the current project or site-unit table/);
  assert.match(body, /not native Excel AutoFit/); assert.match(body, /never overwrites/);
  const client = compile(source, { filename: 'SubzoneMatrixReport.svelte', generate: 'client' });
  assert.equal(client.warnings.length, 0);
  assert.doesNotMatch(client.js.code, /proxy\(validated\)/);
});
for (const flag of [undefined, false, true, 'false', 'TRUE', '1', 'true']) {
  test(`compiled matrix menu requires exact independent literal ${String(flag)}`, async () => {
    const component = serverComponent(readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8'), 'Navigation.svelte', {
      '@lucide/svelte': new Proxy({}, { get: () => () => {} }),
    }, { VITE_SUBZONE_MATRIX_WORKBOOK: flag });
    const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
    const body = render(component, { props: { view: 'home', onnavigate: () => {} } }).body;
    const button = [...body.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)].find(match => match[2].includes('Subzone Matrix'));
    assert.ok(button, 'matrix menu selector must resolve');
    assert.equal(/\bdisabled(?:=|\s|$)/.test(button[1]), flag !== 'true');
  });
}
test('actual application refresh/paging guards do not borrow plot reads during matrix work', async () => {
  for (const name of ['loadPlots', 'loadHierarchy', 'refresh']) {
    const context = Object.fromEntries([
      'projectReportBusy', 'twoPageVegetationBusy', 'twoPageSoilBusy', 'pictureCreationBusy',
      'pictureLifecycleBusy', 'vegetationPublicationBusy', 'lifeformPublicationBusy', 'summaryPublicationBusy',
      'summaryPreferenceBusy', 'externalSUEnvironmentBusy',
    ].map(key => [key, false]));
    Object.assign(context, { view: 'subzone-matrix', editorBusy: true });
    const functions = componentFunctions('App.svelte', [name], context);
    await functions[name](0);
  }
});
