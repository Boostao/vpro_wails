const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const report = loadTypeScript('breakReportReview.ts', { './lifeformSummary': loadTypeScript('lifeformSummary.ts') });
const { ReadRequests } = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'breakReportWire.json'), 'utf8'));
const validate = value => report.breakReportFromWire(value, 'break-context', value.offset, value.limit);

test('actual-Go break-code wires preserve nullable DISTINCT pairs, paging, literals and visible column order', () => {
  const wires = fixture(), original = JSON.stringify(wires);
  for (const [name, total, rows] of [['first', 7, 7], ['page', 7, 1], ['end', 7, 0], ['empty', 0, 0]]) {
    const view = validate(wires[name]);
    assert.equal(view.total, total); assert.equal(view.rows.length, rows);
    assert.doesNotThrow(() => structuredClone(view));
    if (view.rows.length) view.rows[0].reference = 'caller only';
  }
  assert.equal(JSON.stringify(wires), original);
  assert.deepEqual(Array.from(validate(wires.first).rows, row => [row.breakCode, row.reference]),
    [[null, null], [null, ''], ['', null], ['', ''], ['001', '=1+1'], ['A', 'R1'], ['B', 'R2']]);
  assert.deepEqual(Array.from(report.breakReportFields, field => [field.key, field.caption]),
    [['breakCode', 'PC-ORD Break Code'], ['reference', 'VPro Reference']]);
  const literal = wires.page;
  literal.rows[0] = { breakCode: ' 001 ', reference: 'Z'.repeat(255) };
  assert.equal(validate(literal).rows[0].reference.length, 255);
  assert.equal(validate(literal).rows[0].breakCode, ' 001 ');
});

for (const [name, mutate] of [
  ['foreign context', value => value.contextId = 'foreign'],
  ['missing context', value => delete value.contextId],
  ['extra table', value => value.original = {}],
  ['empty source path', value => value.sourcePath = ''],
  ['NUL source path', value => value.sourcePath += '\0'],
  ['malformed source path', value => value.sourcePath = '\ud800'],
  ['short hash', value => value.sourceSHA256 = 'a'.repeat(63)],
  ['uppercase hash', value => value.sourceSHA256 = 'A'.repeat(64)],
  ['trailing-newline hash', value => value.sourceSHA256 += '\n'],
  ['wrong offset', value => value.offset = 1],
  ['wrong limit', value => value.limit = 25],
  ['negative total', value => value.total = -1],
  ['fractional total', value => value.total = 7.5],
  ['excessive total', value => value.total = 10001],
  ['missing pair', value => value.rows.pop()],
  ['sparse pair', value => delete value.rows[0]],
  ['duplicate pair', value => value.rows[1] = value.rows[0]],
  ['case-equivalent duplicate', value => value.rows[6] = { reference: 'r1', breakCode: 'a' }],
  ['space-equivalent duplicate', value => value.rows[6] = { reference: 'R1 ', breakCode: 'A ' }],
  ['wrong pair order', value => value.rows.reverse()],
  ['missing nullable reference', value => delete value.rows[0].reference],
  ['missing nullable code', value => delete value.rows[0].breakCode],
  ['extra row field', value => value.rows[0].plotNumber = 'ignored'],
]) test(`strict break-code transport rejects ${name}`, () => {
  const value = fixture().first; mutate(value);
  assert.throws(() => report.breakReportFromWire(value, 'break-context', 0, 50));
});
for (const key of ['reference', 'breakCode']) {
  for (const [name, invalid] of [
    ['numeric coercion', 1], ['overlength text', 'A'.repeat(256)], ['non-ASCII', '\u00e9'],
    ['surrogate', '\ud800'], ['NUL', 'A\0'], ['tab', 'A\t'], ['trailing newline', 'A\n'],
    ['CRLF', 'A\r\n'], ['DEL', 'A\x7f'],
  ]) test(`strict break-code ${key} rejects ${name} without repair`, () => {
    const value = fixture().page; value.rows[0][key] = invalid;
    assert.throws(() => validate(value));
  });
}
test('request ownership/bounds never silently default and complete-source witnesses are independent', () => {
  for (const [id, offset, limit] of [
    ['', 0, 50], ['bad\ud800', 0, 50], ['break-context', -1, 50], ['break-context', 0.5, 50],
    ['break-context', Infinity, 50], ['break-context', 0, 0], ['break-context', 0, 101],
  ]) assert.throws(() => report.breakReportFromWire(fixture().first, id, offset, limit));
  const last = fixture().end;
  last.total = 10000; last.offset = 10000;
  assert.equal(validate(last).rows.length, 0);
  const first = validate(fixture().first), next = validate(fixture().page);
  assert.equal(report.sameBreakReportSource(first, next), true);
  for (const [key, different] of [
    ['contextId', 'foreign'], ['sourcePath', 'C:\\foreign\\VPro64.db'],
    ['sourceSHA256', 'a'.repeat(64)], ['total', 8],
  ]) assert.equal(report.sameBreakReportSource(first, { ...next, [key]: different }), false);
});
test('actual Svelte client-runtime proxy is validated into detached nonproxied DTOs', async () => {
  const { proxy } = await import('svelte/internal/client');
  const reactive = proxy(fixture().first);
  assert.throws(() => structuredClone(reactive));
  const detached = validate(reactive);
  assert.doesNotThrow(() => structuredClone(detached));
  reactive.rows[4].reference = 'changed reactive';
  assert.equal(detached.rows[4].reference, '=1+1');
});

function deferred() {
  let resolve, reject, cancels = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancels++; };
  return { promise, resolve, reject, cancelled: () => cancels };
}
function driver(getReview) {
  const activity = [], values = {
    generation: 0, reads: new ReadRequests(), busy: false, error: '', review: null,
    contextId: 'break-context', limit: 50, operationHeld: false,
    onBusyChange: value => activity.push(value), BreakReportService: { GetReview: getReview },
    breakReportFromWire: report.breakReportFromWire, sameBreakReportSource: report.sameBreakReportSource,
  };
  const context = componentFunctions('BreakReportReview.svelte', ['cancel', 'show'], values);
  return { context, activity, actions: context.actions };
}
test('actual component reads source first/tail pages and refuses full-original source drift', async () => {
  const calls = [], d = driver((...args) => { calls.push(args); return Promise.resolve(fixture().first); });
  await d.actions.show();
  assert.deepEqual(calls, [['break-context', 0, 50]]);
  assert.equal(d.context.review.total, 7); assert.equal(d.context.busy, false);
  assert.deepEqual(d.activity, [false, true, false]);
  d.context.BreakReportService.GetReview = () => Promise.resolve(fixture().end);
  await d.actions.show(100);
  assert.equal(d.context.review.rows.length, 0);
  d.context.BreakReportService.GetReview = () => {
    const changed = fixture().end; changed.sourceSHA256 = 'a'.repeat(64);
    return Promise.resolve(changed);
  };
  await d.actions.show(100);
  assert.equal(d.context.review, null); assert.match(d.context.error, /complete physical break-code source changed/);
  await d.actions.show(100);
  assert.match(d.context.error, /first page/);
  d.context.BreakReportService.GetReview = () => Promise.resolve(fixture().first);
  await d.actions.show();
  assert.equal(d.context.review.total, 7); assert.equal(d.context.error, '');
});
test('Cancel/remount rejects held late success/error and never clears a newer busy operation', async () => {
  const old = deferred(), fresh = deferred(); let calls = 0;
  const d = driver(() => (++calls === 1 ? old.promise : fresh.promise));
  const prior = d.actions.show();
  d.actions.cancel(); assert.equal(old.cancelled(), 1);
  const next = d.actions.show();
  old.resolve(fixture().first); await prior;
  assert.equal(d.context.review, null); assert.equal(d.context.busy, true);
  fresh.resolve(fixture().first); await next;
  assert.equal(d.context.review.total, 7); assert.equal(d.context.busy, false);
  const held = deferred(), beforeUnmount = driver(() => held.promise);
  const pending = beforeUnmount.actions.show(); beforeUnmount.actions.cancel();
  const remount = driver(() => Promise.resolve(fixture().first));
  held.reject(new Error('old error')); await pending;
  assert.equal(beforeUnmount.context.error, ''); assert.equal(remount.context.review, null);
  await remount.actions.show(); assert.equal(remount.context.review.total, 7);
});
test('held operations, context drift, backend refusals and post-validation cancellation cannot populate results', async () => {
  let calls = 0;
  const blocked = driver(() => { calls++; throw new Error('source read failed'); });
  blocked.context.operationHeld = true; await blocked.actions.show();
  assert.equal(calls, 0); assert.match(blocked.context.error, /current operation/);
  blocked.context.operationHeld = false; await blocked.actions.show();
  assert.equal(blocked.context.review, null); assert.match(blocked.context.error, /source read failed/);
  const held = deferred(), changed = driver(() => held.promise);
  const pending = changed.actions.show(); changed.context.contextId = 'foreign';
  held.resolve(fixture().first); await pending;
  assert.equal(changed.context.review, null);
  const failed = deferred(), foreignError = driver(() => failed.promise);
  const failing = foreignError.actions.show(); foreignError.context.contextId = 'foreign';
  failed.reject(new Error('foreign old failure')); await failing;
  assert.equal(foreignError.context.review, null); assert.equal(foreignError.context.error, '');
  const validating = driver(() => Promise.resolve(fixture().first));
  validating.context.breakReportFromWire = (...args) => {
    const value = report.breakReportFromWire(...args); validating.actions.cancel(); return value;
  };
  await validating.actions.show();
  assert.equal(validating.context.review, null); assert.equal(validating.context.busy, false);
});
test('root refresh/list/hierarchy retain break-report read activity barriers', async () => {
  for (const name of ['refresh', 'loadPlots', 'loadHierarchy']) {
    const context = Object.fromEntries([
      'projectReportBusy', 'twoPageVegetationBusy', 'twoPageSoilBusy', 'pictureCreationBusy',
      'pictureLifecycleBusy', 'vegetationPublicationBusy', 'lifeformPublicationBusy', 'summaryPublicationBusy',
      'summaryPreferenceBusy', 'externalSUEnvironmentBusy',
    ].map(key => [key, false]));
    Object.assign(context, { view: 'break-report', editorBusy: true });
    await componentFunctions('App.svelte', [name], context)[name](0);
  }
});
test('compiled SSR executes cleanup, code/reference columns, NULL/empty labels and explicit limitations', async () => {
  let source = readFileSync(path.join(__dirname, 'BreakReportReview.svelte'), 'utf8');
  const marker = 'let review = $state.raw<BreakReportPage | null>(null);';
  assert.equal(source.split(marker).length, 2);
  source = source.replace(marker, `let review = $state.raw<BreakReportPage | null>(${JSON.stringify(fixture().first)});`);
  const component = serverComponent(source, 'BreakReportReview.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { BreakReportService: {} },
    './breakReportReview': report,
  });
  const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
  let callbacks = 0;
  const body = render(component, { props: { contextId: 'break-context', onBusyChange: () => callbacks++ } }).body;
  assert.ok(callbacks > 0, 'lazy SSR must actually execute cleanup callbacks');
  assert.ok(body.indexOf('>PC-ORD Break Code</th>') < body.indexOf('>VPro Reference</th>'));
  for (const caption of ['PC-ORD Break Code', 'VPro Reference']) assert.ok(body.includes(`>${caption}</th>`));
  assert.equal((body.match(/aria-label="Break-code pair /g) ?? []).length, 7);
  for (const text of ['VPro Break Report', '(NULL)', '(empty text)', '=1+1', '001',
    'Complete original source SHA256', 'report printing and export generation are not implemented']) assert.ok(body.includes(text));
  assert.equal(compile(source, { filename: 'BreakReportReview.svelte', generate: 'client' }).warnings.length, 0);
  assert.doesNotMatch(source, /\b(?:Save|Delete|Publish|Export)\s*\(/);
});
for (const flag of [undefined, false, true, 'false', 'TRUE', '1', 'true']) {
  test(`compiled PC-ORD menu requires its exact independent literal ${String(flag)}`, async () => {
    const component = serverComponent(readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8'), 'Navigation.svelte', {
      '@lucide/svelte': new Proxy({}, { get: () => () => {} }),
    }, { VITE_BREAK_REPORT_REVIEW: flag, VITE_SUBZONE_MATRIX_WORKBOOK: 'true', VITE_USER_LOG_REVIEW: 'true' });
    const { render } = await import(pathToFileURL(require.resolve('svelte/server')).href);
    const body = render(component, { props: { view: 'home', onnavigate: () => {} } }).body;
    const button = [...body.matchAll(/<button\b([^>]*)>([\s\S]*?)<\/button>/g)].find(match => match[2].includes('PC-ORD Break Report'));
    assert.ok(button, 'planned PC-ORD menu selector must resolve');
    assert.equal(/\bdisabled(?:=|\s|$)/.test(button[1]), flag !== 'true');
  });
}
