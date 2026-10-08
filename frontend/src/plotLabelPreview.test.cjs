const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const lookup = loadTypeScript('scopedPlotLookup.ts', { './qualityEditor': quality });
const labels = loadTypeScript('plotLabelPreview.ts', { './qualityEditor': quality, './scopedPlotLookup': lookup });
function header() {
  return { plotNumber: '00337', zone: 'CWH', subZone: 'dm', siteSeries: '01',
    plotRepresenting: '  Original representing  ', projectId: ' literal ID ' };
}
function host(operation = () => Promise.resolve(header())) {
  const calls = [], changes = [];
  const context = componentFunctions('PlotLabelPreview.svelte', ['cancel', 'load'], {
    contextId: 'owned', project: 'Sample', ownerKey: 'owned-Sample', plot: '00337',
    preview: null, labelDate: '', error: '', busy: false, generation: 0,
    plotLookupInputError: lookup.plotLookupInputError, plotLabelFromHeader: labels.plotLabelFromHeader,
    Date, onBusyChange(value) { changes.push(value); },
    reads: { track(value) { return value; }, cancelAll() { calls.push('cancel'); } },
    ContextService: { GetPlot(...args) { calls.push(['read', ...args]); return operation(...args); } },
  });
  return { context, calls, changes };
}

test('label preparation preserves exact assignments and applies only source display Trim', () => {
  const source = header(); source.plotNumber = ' 00337 ';
  const before = JSON.stringify(source);
  const result = labels.plotLabelFromHeader(source, source.plotNumber);
  assert.equal(result.plotNumber1, ' 00337 ');
  assert.equal(result.displayPlotNumber, '00337');
  assert.equal(result.zone1, 'CWH dm/01');
  assert.equal(result.representing1, '  Original representing  ');
  assert.equal(result.displayRepresenting, 'Original representing');
  assert.equal(result.projectId1, ' literal ID ');
  assert.equal(JSON.stringify(source), before);
  source.plotRepresenting = '\t text \t';
  assert.equal(labels.plotLabelFromHeader(source, source.plotNumber).displayRepresenting, '\t text \t');
  for (const ending of ['\n', '\r', '\r\n', '\u2028', '\u2029']) {
    source.plotNumber = ' A ' + ending; source.plotRepresenting = ' A ' + ending;
    const exact = labels.plotLabelFromHeader(source, source.plotNumber);
    assert.equal(exact.displayPlotNumber, 'A ' + ending);
    assert.equal(exact.displayRepresenting, 'A ' + ending);
  }
});

test('label NULL/empty metadata remains distinguishable and concatenation is explicit', () => {
  const source = header();
  source.zone = ''; source.subZone = null; source.siteSeries = null;
  source.plotRepresenting = null; source.projectId = '';
  const result = labels.plotLabelFromHeader(source, source.plotNumber);
  assert.equal(result.zone1, '    /');
  assert.equal(result.representing1, null);
  assert.equal(result.displayRepresenting, null);
  assert.equal(result.projectId1, '');
  source.zone = null;
  assert.throws(() => labels.plotLabelFromHeader(source, source.plotNumber), /non-NULL.*Len\/Space/);
});

test('label source text bounds are exact UTF-16 units without repair or truncation', () => {
  const source = header();
  source.zone = '😀😀'; source.subZone = 'x'.repeat(45); source.siteSeries = '';
  source.plotNumber = '😀😀abc'; source.projectId = '😀'.repeat(25); source.plotRepresenting = '😀'.repeat(127) + 'x';
  assert.equal(labels.plotLabelFromHeader(source, source.plotNumber).zone1.length, 50);
  for (const [field, value] of [
    ['zone', '😀😀x'], ['subZone', 'x'.repeat(46)], ['plotNumber', '😀😀abcd'],
    ['projectId', '😀'.repeat(25) + 'x'], ['plotRepresenting', '😀'.repeat(128)],
  ]) {
    const changed = { ...source, [field]: value };
    assert.throws(() => labels.plotLabelFromHeader(changed, changed.plotNumber), /UTF-16 bound/);
  }
});

test('label header rejects foreign, incomplete and malformed values rather than inferring blanks', () => {
  const source = header();
  for (const value of [null, [], {}, { ...source, plotNumber: 'foreign' }]) {
    assert.throws(() => labels.plotLabelFromHeader(value, source.plotNumber));
  }
  for (const field of ['zone', 'subZone', 'siteSeries', 'plotRepresenting', 'projectId']) {
    const missing = { ...source }; delete missing[field];
    assert.throws(() => labels.plotLabelFromHeader(missing, source.plotNumber), /missing source label fields/);
    for (const invalid of [undefined, 1, false, '\ud800', '\udc00', '\0']) {
      assert.throws(() => labels.plotLabelFromHeader({ ...source, [field]: invalid }, source.plotNumber));
    }
  }
});

test('saved-label loader performs only one owned parent read and never stages or prints', async () => {
  const { context, calls, changes } = host();
  await context.actions.load();
  assert.deepEqual(calls.find(Array.isArray), ['read', 'owned', '00337']);
  assert.equal(context.preview.zone1, 'CWH dm/01');
  assert.notEqual(context.labelDate, '');
  assert.equal(context.busy, false);
  assert.equal(changes.at(-1), false);
  const source = readFileSync(path.join(__dirname, 'PlotLabelPreview.svelte'), 'utf8');
  assert.doesNotMatch(source, /UpdatePlot|CreatePlot|SetLabelRecord|window\.print|OpenReport|Save|WriteFile/);
});

test('saved-label malformed input never reaches a binding and reports its error', async () => {
  const { context, calls } = host();
  context.plot = '\ud800';
  await context.actions.load();
  assert.match(context.error, /complete literal plot/);
  assert.equal(calls.filter(Array.isArray).length, 0);
  assert.equal(context.preview, null);
});

test('saved-label completed-response cancellation discards late data and permits explicit retry', async () => {
  let release, held = true;
  const { context, calls } = host(() => held ? new Promise(resolve => release = resolve) : Promise.resolve(header()));
  const pending = context.actions.load();
  assert.equal(context.busy, true);
  context.actions.cancel();
  release(header());
  await pending;
  assert.equal(context.preview, null);
  assert.equal(context.labelDate, '');
  held = false;
  await context.actions.load();
  assert.equal(context.preview.plotNumber1, '00337');
  assert.equal(calls.filter(Array.isArray).length, 2);
});

test('saved-label owner changes reject late results without an implicit cross-context label', async () => {
  let release;
  const { context } = host(() => new Promise(resolve => release = resolve));
  const pending = context.actions.load();
  context.ownerKey = 'new-context';
  release(header());
  await pending;
  assert.equal(context.preview, null);
  assert.equal(context.labelDate, '');
  assert.equal(context.error, '');
});

test('saved-label failed reads or date rendering assign no partial card and retain explicit errors', async () => {
  const { context } = host(() => Promise.reject(new Error('parent missing')));
  await context.actions.load();
  assert.match(context.error, /parent missing/);
  assert.equal(context.preview, null);
  const second = host().context;
  second.Date = class { toLocaleDateString() { throw new Error('clock display unavailable'); } };
  await second.actions.load();
  assert.match(second.error, /clock display unavailable/);
  assert.equal(second.preview, null);
  assert.equal(second.labelDate, '');
});

test('saved-label server render and navigation remain independently default-off/read-only', () => {
  const source = readFileSync(path.join(__dirname, 'PlotLabelPreview.svelte'), 'utf8');
  const component = serverComponent(source, 'PlotLabelPreview.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { ContextService: {} },
    './scopedPlotLookup': lookup, './plotLabelPreview': labels,
  });
  const body = render(component, { props: { contextId: 'owned', project: 'Sample', onBusyChange() {} } }).body;
  assert.match(body, /VPro Plot Labels/);
  assert.match(body, /for="plot-label-number"/);
  assert.match(body, /six-position printing and Print All Plots remain unavailable/);
  const nav = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  assert.match(nav, /VITE_PLOT_LABEL_PREVIEW === 'true' \? 'plot-label-preview' : undefined/);
  assert.match(app, /view === 'plot-label-preview' && import\.meta\.env\.VITE_PLOT_LABEL_PREVIEW === 'true'/);
  assert.match(app, /<PlotLabelPreview[^>]*onBusyChange=\{\(value\) => \{ editorBusy = value; \}\}/s);
  assert.match(source, /onDestroy\(cancel\)/);
});
